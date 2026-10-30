package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/auth"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollment"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollmentsecure"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/ids"
)

// This observer delegates every call to the concrete Python processor selected at
// startup. It retains only hashes and scalar result metadata, never input/vector
// bytes. It cannot change a result, create a job, or activate an enrollment.
type integrationRuntime struct {
	processor enrollment.Processor
	db        *sql.DB
	keys      stagingKeys
	traceID   string
	mu        sync.Mutex
	observed  map[string]processingObservation
	grants    map[string][32]byte
	trace     []integrationTrace
}

type processingObservation struct {
	ProcessedSHA256  string
	Calls            int
	FaceCount        int
	Dimension        int
	Finite           bool
	ManifestVerified bool
}

type integrationTrace struct {
	Stage            string `json:"stage"`
	TraceID          string `json:"trace_id"`
	SubmissionID     string `json:"submission_id,omitempty"`
	CanonicalSHA256  string `json:"canonical_sha256,omitempty"`
	ProcessedSHA256  string `json:"processed_sha256,omitempty"`
	ProcessingCalls  int    `json:"processing_calls,omitempty"`
	FaceCount        int    `json:"face_count,omitempty"`
	EmbeddingDim     int    `json:"embedding_dim,omitempty"`
	EmbeddingFinite  bool   `json:"embedding_finite"`
	ManifestVerified bool   `json:"manifest_verified"`
	Status           string `json:"status,omitempty"`
}

func newIntegrationRuntime(processor enrollment.Processor, db *sql.DB) *integrationRuntime {
	return &integrationRuntime{processor: processor, db: db, traceID: ids.New(), observed: make(map[string]processingObservation), grants: make(map[string][32]byte)}
}

func (r *integrationRuntime) Manifest() enrollment.Manifest { return r.processor.Manifest() }

func (r *integrationRuntime) Process(ctx context.Context, photo []byte) (enrollment.ProcessingResult, error) {
	sum := sha256.Sum256(photo)
	hash := hex.EncodeToString(sum[:])
	// The domain permits one RUNNING lease. Bind observations to that actual
	// job before invoking Python, rather than conflating repeated JPEG hashes
	// across separate submissions. A failed read never changes Process output.
	var submissionID string
	if r.db.QueryRowContext(ctx, `SELECT submission_id FROM enrollment_processing_jobs WHERE state='RUNNING' AND expected_photo_hash=?`, hash).Scan(&submissionID) != nil {
		submissionID = ""
	}
	result, err := r.processor.Process(ctx, photo)
	finite := len(result.Embedding) > 0
	for _, v := range result.Embedding {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			finite = false
		}
	}
	r.mu.Lock()
	observation := r.observed[submissionID]
	observation.ProcessedSHA256 = hash
	observation.Calls++
	observation.FaceCount = result.FaceCount
	observation.Dimension = len(result.Embedding)
	observation.Finite = finite
	observation.ManifestVerified = err == nil && result.Manifest == r.processor.Manifest()
	r.observed[submissionID] = observation
	r.trace = append(r.trace, integrationTrace{Stage: "python.process", TraceID: r.traceID, SubmissionID: submissionID, CanonicalSHA256: hash, ProcessedSHA256: hash, ProcessingCalls: observation.Calls, FaceCount: observation.FaceCount, EmbeddingDim: observation.Dimension, EmbeddingFinite: finite, ManifestVerified: observation.ManifestVerified})
	r.mu.Unlock()
	return result, err
}

func (s *staging) seedIntegrationAccounts(ctx context.Context) error {
	s.fixtures = map[string]adminFixture{
		"admin":   {UserID: s.admin.ID, Email: s.admin.Email},
		"student": {UserID: s.student.ID, Email: s.student.Email, PersonID: s.person.ID, IdentityReference: s.person.IdentityReference},
	}
	for _, account := range []struct {
		name string
		role auth.Role
	}{{"reviewer2", auth.RoleAdmin}, {"lecturer", auth.RoleDosen}} {
		if _, err := s.inviteFixture(ctx, account.name, account.role); err != nil {
			return err
		}
	}
	return nil
}

type integrationSnapshot struct {
	Schema                  int                `json:"schema"`
	JournalMode             string             `json:"journal_mode"`
	SubmissionID            string             `json:"submission_id"`
	Status                  string             `json:"status"`
	ReasonCode              string             `json:"reason_code"`
	PhotoRows               int                `json:"photo_rows"`
	TemplateRows            int                `json:"template_rows"`
	ActiveRows              int                `json:"active_rows"`
	CiphertextFiles         int                `json:"ciphertext_files"`
	DeletionIncomplete      bool               `json:"deletion_incomplete"`
	PhotoExpiresAt          string             `json:"photo_expires_at"`
	CanonicalSHA256         string             `json:"canonical_sha256"`
	ProcessedSHA256         string             `json:"processed_sha256"`
	ProcessingCalls         int                `json:"processing_calls"`
	FaceCount               int                `json:"face_count"`
	EmbeddingDim            int                `json:"embedding_dim"`
	EmbeddingFinite         bool               `json:"embedding_finite"`
	ManifestVerified        bool               `json:"manifest_verified"`
	TemplateEnvelopeVersion int                `json:"template_envelope_version"`
	TemplateBindingValid    bool               `json:"template_binding_valid"`
	GrantIdentityPreserved  bool               `json:"grant_identity_preserved"`
	Audit                   []integrationAudit `json:"audit"`
	Trace                   []integrationTrace `json:"trace"`
}

type integrationAudit struct {
	Action    string `json:"action"`
	Result    string `json:"result"`
	RequestID string `json:"request_id"`
}

func fixtureUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, c := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return value != "00000000-0000-0000-0000-000000000000"
}

func (s *staging) integrationSnapshot(ctx context.Context, id string) (integrationSnapshot, error) {
	out := integrationSnapshot{SubmissionID: id, Schema: int(s.migration.CurrentVersion), Audit: []integrationAudit{}}
	if !fixtureUUID(id) {
		return out, errors.New("invalid submission")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if err = tx.QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&out.JournalMode); err != nil {
		return out, err
	}
	var personID, consentID, subjectID string
	err = tx.QueryRowContext(ctx, `SELECT person_id,consent_id,subject_user_id,status,COALESCE(revoked_reason_code,processing_error_code,rejection_reason_code,''),photo_expires_at,canonical_sha256 FROM face_enrollment_submissions WHERE id=?`, id).Scan(&personID, &consentID, &subjectID, &out.Status, &out.ReasonCode, &out.PhotoExpiresAt, &out.CanonicalSHA256)
	if err != nil {
		return out, err
	}
	for _, query := range []struct {
		sql    string
		target *int
	}{
		{`SELECT COUNT(*) FROM face_enrollment_photos WHERE submission_id=?`, &out.PhotoRows},
		{`SELECT COUNT(*) FROM face_templates WHERE submission_id=?`, &out.TemplateRows},
		{`SELECT COUNT(*) FROM face_enrollments a JOIN face_enrollment_submissions s ON s.activated_enrollment_id=a.id WHERE s.id=? AND a.status='active'`, &out.ActiveRows},
	} {
		if err = tx.QueryRowContext(ctx, query.sql, id).Scan(query.target); err != nil {
			return out, err
		}
	}
	var incomplete int
	err = tx.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM biometric_deletion_jobs WHERE person_id=? AND state<>'COMPLETE')+(SELECT COUNT(*) FROM enrollment_file_deletions WHERE submission_id=?)`, personID, id).Scan(&incomplete)
	if err != nil {
		return out, err
	}
	out.DeletionIncomplete = incomplete > 0
	// Capture the immutable grant tuple while granted, then compare its digest
	// after withdrawal. The referenced grant ID is part of that tuple; status and
	// withdrawal fields deliberately are not. No identity fields leave this method.
	var grant [11]string
	var grantStatus string
	err = tx.QueryRowContext(ctx, `SELECT id,person_id,subject_user_id,purpose,notice_version,notice_digest,policy_version,method,granted_at,recorded_by,status FROM consent_records WHERE id=?`, consentID).Scan(&grant[0], &grant[1], &grant[2], &grant[3], &grant[4], &grant[5], &grant[6], &grant[7], &grant[8], &grant[9], &grantStatus)
	if err != nil {
		return out, err
	}
	grant[10] = id
	encoded, err := json.Marshal(grant)
	if err != nil {
		return out, err
	}
	grantDigest := sha256.Sum256(encoded)
	clear(encoded)
	r := s.integration
	r.mu.Lock()
	baseline, exists := r.grants[id]
	bound := grant[0] == consentID && grant[1] == personID && grant[2] == subjectID && grant[3] == "face_enrollment" && grant[7] == "digital_explicit" && grant[9] == subjectID
	if !exists && grantStatus == "granted" && bound {
		r.grants[id] = grantDigest
		baseline, exists = grantDigest, true
	}
	out.GrantIdentityPreserved = bound && exists && baseline == grantDigest
	r.mu.Unlock()
	if out.TemplateRows > 0 {
		out.TemplateBindingValid, out.TemplateEnvelopeVersion, err = s.verifyIntegrationTemplate(ctx, tx, id)
		if err != nil {
			return out, err
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT action,result,COALESCE(request_id,'') FROM audit_events WHERE entity_id IN (?,?,?) OR request_id=? ORDER BY rowid`, id, personID, consentID, r.traceID)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var event integrationAudit
		if err = rows.Scan(&event.Action, &event.Result, &event.RequestID); err != nil {
			rows.Close()
			return out, err
		}
		if !safeIntegrationAction(event.Action) {
			continue
		}
		if event.Result != "success" && event.Result != "failure" {
			continue
		}
		if !fixtureUUID(event.RequestID) {
			event.RequestID = ""
		}
		out.Audit = append(out.Audit, event)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if err = tx.Commit(); err != nil {
		return out, err
	}
	entries, err := os.ReadDir(filepath.Join(s.root, "photos"))
	if err != nil {
		return out, err
	}
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			return out, errors.New("unexpected photo storage entry")
		}
		out.CiphertextFiles++
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	observation := r.observed[id]
	out.ProcessingCalls, out.FaceCount, out.EmbeddingDim = observation.Calls, observation.FaceCount, observation.Dimension
	out.EmbeddingFinite, out.ManifestVerified = observation.Finite, observation.ManifestVerified
	out.ProcessedSHA256 = observation.ProcessedSHA256
	r.trace = append(r.trace, integrationTrace{Stage: "snapshot", TraceID: r.traceID, SubmissionID: id, CanonicalSHA256: out.CanonicalSHA256, Status: out.Status})
	out.Trace = make([]integrationTrace, 0, len(r.trace))
	for _, stage := range r.trace {
		if stage.SubmissionID != id {
			continue
		}
		out.Trace = append(out.Trace, stage)
	}
	return out, nil
}

// Reconstruct AAD from the submission/consent/activation join, not from stored
// raw AAD or a caller-supplied identity. Open validates v2, authenticates, then
// the plaintext is wiped immediately. No vector or envelope is ever serialized.
func (s *staging) verifyIntegrationTemplate(ctx context.Context, tx *sql.Tx, id string) (bool, int, error) {
	var binding enrollmentsecure.Binding
	var envelope enrollmentsecure.Envelope
	var manifest enrollment.Manifest
	var dimension int
	var dtype string
	err := tx.QueryRowContext(ctx, `SELECT t.id,s.id,s.person_id,c.id,t.envelope_version,t.algorithm,t.key_version,t.aad,t.nonce,t.ciphertext,t.embedding_dim,t.dtype,t.model_id,t.model_version,t.model_checksum,t.detector_id,t.detector_version,t.detector_checksum,t.pipeline_revision,t.synthetic_only
	 FROM face_enrollment_submissions s JOIN consent_records c ON c.id=s.consent_id AND c.person_id=s.person_id AND c.subject_user_id=s.subject_user_id
	 JOIN face_templates t ON t.submission_id=s.id AND t.person_id=s.person_id AND t.consent_id=c.id AND t.activation_enrollment_id=s.activated_enrollment_id
	 JOIN face_enrollments a ON a.id=s.activated_enrollment_id AND a.person_id=s.person_id AND a.model_name=t.model_id AND a.model_version=t.model_version AND a.model_checksum=t.model_checksum
	 WHERE s.id=? AND s.status='ACTIVE' AND c.status='granted' AND c.purpose='face_enrollment' AND a.status='active'`, id).Scan(&binding.RecordID, &binding.SubmissionID, &binding.PersonID, &binding.ConsentID, &envelope.Version, &envelope.Algorithm, &envelope.KeyVersion, &envelope.AAD, &envelope.Nonce, &envelope.Ciphertext, &dimension, &dtype, &manifest.ModelID, &manifest.ModelVersion, &manifest.ModelChecksum, &manifest.DetectorID, &manifest.DetectorVersion, &manifest.DetectorChecksum, &manifest.PipelineRevision, &manifest.SyntheticOnly)
	if errors.Is(err, sql.ErrNoRows) {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, err
	}
	if manifest != s.integration.Manifest() || dimension != 128 || dtype != "float32-le" {
		return false, envelope.Version, nil
	}
	binding.Purpose, binding.ModelID, binding.ModelVersion, binding.ModelFingerprint = enrollmentsecure.Template, manifest.ModelID, manifest.ModelVersion, manifest.Fingerprint()
	plain, err := enrollmentsecure.Open(ctx, s.integration.keys, binding, envelope)
	defer clear(plain)
	return err == nil && len(plain) == 512, envelope.Version, nil
}

func safeIntegrationAction(action string) bool {
	switch action {
	case "enrollment.queue_read", "enrollment.review_opened":
		return true
	case "consent.granted", "consent.withdrawn", "enrollment.submitted", "photo.preview_authorized", "enrollment.approved", "processing.queued", "processing.started", "processing.failed", "processing.retry_requested", "enrollment.activated", "enrollment.rejected", "enrollment.revoked", "enrollment.expired", "photo.deleted", "template.deleted", "deletion.requested", "deletion.completed", "deletion.failed":
		return true
	}
	return false
}

func (s *staging) readIntegrationControls(ctx context.Context, input io.Reader, output io.Writer, stop context.CancelFunc) {
	defer stop()
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 4096)
	encoder := json.NewEncoder(output)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}
		var command struct {
			Command      string `json:"command"`
			SubmissionID string `json:"submission_id,omitempty"`
			Seconds      int64  `json:"seconds,omitempty"`
			ChallengeID  string `json:"challenge_id,omitempty"`
		}
		decoder := json.NewDecoder(bytes.NewReader(scanner.Bytes()))
		decoder.DisallowUnknownFields()
		err := decoder.Decode(&command)
		if err == nil {
			var extra any
			if decoder.Decode(&extra) != io.EOF {
				err = errors.New("one object required")
			}
		}
		ack := map[string]any{"fixture_control": true, "trace_id": s.integration.traceID}
		if err == nil {
			s.control.workMu.Lock()
			if ctx.Err() != nil {
				s.control.workMu.Unlock()
				return
			}
			switch command.Command {
			case "otp.wait":
				err = s.waitOTPDelivery(ctx, command.ChallengeID)
			case "snapshot":
				var snapshot integrationSnapshot
				snapshot, err = s.integrationSnapshot(ctx, command.SubmissionID)
				if err == nil {
					ack["data"] = snapshot
				}
			case "worker.step":
				err = s.service.ProcessNext(ctx)
			case "reconcile":
				err = s.service.Reconcile(ctx)
			case "clock.advance":
				if command.Seconds <= 0 || command.Seconds > 604800 {
					err = errors.New("invalid advance")
				} else {
					s.control.mu.Lock()
					s.control.offset += time.Duration(command.Seconds) * time.Second
					s.control.mu.Unlock()
					err = s.service.Reconcile(ctx)
				}
			default:
				err = errors.New("unknown control")
			}
			s.control.workMu.Unlock()
		}
		// Never echo malformed caller data or SQL/processor errors to evidence.
		switch command.Command {
		case "otp.wait", "snapshot", "worker.step", "reconcile", "clock.advance":
			ack["command"] = command.Command
		}
		ack["ok"] = err == nil
		ack["server_now"] = s.clk.Now().UTC().Format(time.RFC3339Nano)
		ack["worker_paused"] = true
		if err != nil {
			ack["error"] = "fixture control rejected"
		}
		if encoder.Encode(ack) != nil {
			return
		}
	}
}

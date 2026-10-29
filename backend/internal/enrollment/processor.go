package enrollment

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollmentsecure"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/ids"
)

func validManifest(m Manifest) bool {
	if !m.SyntheticOnly {
		return false
	}
	for _, v := range []string{m.ModelID, m.ModelVersion, m.DetectorID, m.DetectorVersion, m.PipelineRevision} {
		if len(v) < 1 || len(v) > 128 {
			return false
		}
		for _, r := range v {
			if r < 0x21 || r > 0x7e {
				return false
			}
		}
	}
	for _, v := range []string{m.ModelChecksum, m.DetectorChecksum} {
		b, err := hex.DecodeString(v)
		if err != nil || len(b) != 32 {
			return false
		}
	}
	return true
}
func validEmbedding(result ProcessingResult, expected Manifest) ([]byte, string) {
	if result.Manifest != expected || !validManifest(result.Manifest) {
		return nil, "MODEL_REVISION_MISMATCH"
	}
	if result.FaceCount == 0 {
		return nil, "NO_FACE"
	}
	if result.FaceCount != 1 {
		return nil, "MULTIPLE_FACES"
	}
	if len(result.Embedding) != 128 {
		return nil, "INVALID_EMBEDDING"
	}
	var norm float64
	for _, v := range result.Embedding {
		f := float64(v)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return nil, "INVALID_EMBEDDING"
		}
		norm += f * f
	}
	if norm == 0 || math.Abs(math.Sqrt(norm)-1) > 1e-5 {
		return nil, "INVALID_EMBEDDING"
	}
	plain := make([]byte, 512)
	for i, v := range result.Embedding {
		binary.LittleEndian.PutUint32(plain[i*4:], math.Float32bits(v))
	}
	return plain, ""
}
func processingCode(err error) string {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "AI_ENGINE_TIMEOUT"
	}
	var e *ProcessingError
	if errors.As(err, &e) {
		switch e.Code {
		case "NO_FACE", "MULTIPLE_FACES", "EXTRACTION_FAILED", "MODEL_REVISION_MISMATCH", "MODEL_UNAVAILABLE", "AI_ENGINE_TIMEOUT", "LOW_QUALITY", "FACE_TOO_SMALL", "INVALID_IMAGE", "INFERENCE_FAILED":
			return e.Code
		}
	}
	return "EXTRACTION_FAILED"
}

// ProcessNext processes at most one durable job; no automatic retries or fallback model exist.
// A processor that violates cancellation retains the single slot until it actually returns.
func (s *Service) ProcessNext(ctx context.Context) error {
	if s == nil || s.processor == nil || s.keys == nil || s.files == nil {
		return failure("BIOMETRIC_UNAVAILABLE", 503)
	}
	manifest := s.processor.Manifest()
	if !validManifest(manifest) {
		return failure("BIOMETRIC_UNAVAILABLE", 503)
	}
	select {
	case s.worker <- struct{}{}:
		defer func() { <-s.worker }()
	default:
		return failure("PROCESSOR_BUSY", 409)
	}
	var row submission
	var attempt, lease, expectedHash string
	var expectedRevision, expectedEpoch int64
	var found bool
	err := s.transaction(ctx, func(c *sql.Conn) error {
		if err := s.maintain(ctx, c); err != nil {
			return err
		}
		var running int
		if err := c.QueryRowContext(ctx, `SELECT COUNT(*) FROM enrollment_processing_jobs WHERE state='RUNNING'`).Scan(&running); err != nil {
			return err
		}
		if running > 0 {
			return failure("PROCESSOR_BUSY", 409)
		}
		err := c.QueryRowContext(ctx, `SELECT submission_id,attempt_id,expected_revision,expected_epoch,expected_photo_hash FROM enrollment_processing_jobs WHERE state='QUEUED' ORDER BY rowid LIMIT 1`).Scan(&row.id, &attempt, &expectedRevision, &expectedEpoch, &expectedHash)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		row, err = s.submission(ctx, c, row.id)
		if err != nil {
			return err
		}
		if row.status != "APPROVED_PROCESSING" || row.revision != expectedRevision || row.epoch != expectedEpoch || row.photoHash != expectedHash {
			return failure("INVALID_TRANSITION", 409)
		}
		valid, err := s.subjectValid(ctx, c, row)
		if err != nil {
			return err
		}
		if !valid {
			return s.failProcessing(ctx, c, row, "SUBJECT_INELIGIBLE")
		}
		valid, err = s.reviewerValid(ctx, c, row)
		if err != nil {
			return err
		}
		if !valid {
			return s.failProcessing(ctx, c, row, "REVIEWER_INELIGIBLE")
		}
		var count int
		if err = c.QueryRowContext(ctx, `SELECT COUNT(*) FROM face_enrollment_photos WHERE submission_id=? AND storage_state='READY' AND expires_at>?`, row.id, nowText(s.clk.Now())).Scan(&count); err != nil {
			return err
		}
		if count != 1 {
			return s.failProcessing(ctx, c, row, "PHOTO_UNAVAILABLE")
		}
		lease = nowText(s.clk.Now().Add(30 * time.Second))
		if _, err = c.ExecContext(ctx, `UPDATE enrollment_processing_jobs SET state='RUNNING',lease_until=? WHERE submission_id=? AND attempt_id=? AND state='QUEUED'`, lease, row.id, attempt); err != nil {
			return err
		}
		found = true
		return s.record(ctx, c, Request{}, "processing.started", "face_enrollment_submission", row.id, "")
	})
	if err != nil {
		return safe(err)
	}
	if !found {
		return nil
	}
	fail := func(code string) error { return s.finishFailure(ctx, row, attempt, code) }
	s.ioMu.Lock()
	photo, err := s.readPhoto(ctx, row)
	s.ioMu.Unlock()
	if err != nil {
		return fail("BIOMETRIC_UNAVAILABLE")
	}
	defer wipe(photo)
	processCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	result, processErr := s.processor.Process(processCtx, photo)
	deadlineErr := processCtx.Err()
	cancel()
	defer func() {
		for i := range result.Embedding {
			result.Embedding[i] = 0
		}
	}()
	if deadlineErr != nil {
		return fail("AI_ENGINE_TIMEOUT")
	}
	if processErr != nil {
		return fail(processingCode(processErr))
	}
	plain, code := validEmbedding(result, manifest)
	if code != "" {
		return fail(code)
	}
	defer wipe(plain)
	templateID := ids.New()
	envelope, err := enrollmentsecure.Seal(ctx, s.keys, enrollmentsecure.Binding{Purpose: "template", RecordID: templateID, SubmissionID: row.id, PersonID: row.personID, ConsentID: row.consentID, ModelID: manifest.ModelID, ModelVersion: manifest.ModelVersion, ModelFingerprint: manifest.Fingerprint()}, plain)
	if err != nil {
		return fail("BIOMETRIC_UNAVAILABLE")
	}
	err = s.transaction(ctx, func(c *sql.Conn) error {
		if err := s.maintain(ctx, c); err != nil {
			return err
		}
		current, err := s.submission(ctx, c, row.id)
		if err != nil {
			return err
		}
		if current.status != "APPROVED_PROCESSING" || current.revision != expectedRevision || current.epoch != expectedEpoch {
			return nil
		}
		var state, currentAttempt, currentLease, currentHash string
		var revision, epoch int64
		if err = c.QueryRowContext(ctx, `SELECT state,attempt_id,COALESCE(lease_until,''),expected_revision,expected_epoch,expected_photo_hash FROM enrollment_processing_jobs WHERE submission_id=?`, row.id).Scan(&state, &currentAttempt, &currentLease, &revision, &epoch, &currentHash); err != nil {
			return err
		}
		if state != "RUNNING" || currentAttempt != attempt || currentLease != lease || revision != expectedRevision || epoch != expectedEpoch || currentHash != expectedHash || current.photoHash != expectedHash {
			return nil
		}
		until, err := time.Parse(time.RFC3339Nano, currentLease)
		if err != nil {
			return err
		}
		if !s.clk.Now().Before(until) {
			return s.failProcessing(ctx, c, current, "AI_ENGINE_TIMEOUT")
		}
		valid, err := s.subjectValid(ctx, c, current)
		if err != nil {
			return err
		}
		if !valid {
			return s.failProcessing(ctx, c, current, "SUBJECT_INELIGIBLE")
		}
		valid, err = s.reviewerValid(ctx, c, current)
		if err != nil {
			return err
		}
		if !valid {
			return s.failProcessing(ctx, c, current, "REVIEWER_INELIGIBLE")
		}
		// Recheck explicit synthetic marker at the irreversible write boundary.
		if !manifest.SyntheticOnly || s.processor.Manifest() != manifest {
			return s.failProcessing(ctx, c, current, "MODEL_REVISION_MISMATCH")
		}
		var count int
		if err = c.QueryRowContext(ctx, `SELECT COUNT(*) FROM face_enrollment_photos WHERE submission_id=? AND storage_state='READY' AND expires_at>?`, row.id, nowText(s.clk.Now())).Scan(&count); err != nil {
			return err
		}
		if count != 1 {
			return s.failProcessing(ctx, c, current, "PHOTO_UNAVAILABLE")
		}
		activation := ids.New()
		now := nowText(s.clk.Now())
		if _, err = c.ExecContext(ctx, `INSERT INTO face_enrollments(id,person_id,status,model_name,model_version,model_checksum,sample_count,enrolled_at,created_by,created_at,updated_at) VALUES(?,?,'active',?,?,?,1,?,?,?,?)`, activation, row.personID, manifest.ModelID, manifest.ModelVersion, manifest.ModelChecksum, now, row.reviewer, now, now); err != nil {
			return err
		}
		if _, err = c.ExecContext(ctx, `INSERT INTO face_templates(id,person_id,activation_enrollment_id,consent_id,submission_id,ciphertext,nonce,aad,envelope_version,algorithm,key_version,embedding_dim,dtype,model_id,model_version,model_checksum,detector_id,detector_version,detector_checksum,pipeline_revision,synthetic_only,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,128,'float32-le',?,?,?,?,?,?,?,1,?)`, templateID, row.personID, activation, row.consentID, row.id, envelope.Ciphertext, envelope.Nonce, envelope.AAD, envelope.Version, envelope.Algorithm, envelope.KeyVersion, manifest.ModelID, manifest.ModelVersion, strings.ToLower(manifest.ModelChecksum), manifest.DetectorID, manifest.DetectorVersion, strings.ToLower(manifest.DetectorChecksum), manifest.PipelineRevision, now); err != nil {
			return err
		}
		if _, err = c.ExecContext(ctx, `UPDATE face_enrollment_submissions SET status='ACTIVE',activated_enrollment_id=?,revision=revision+1,updated_at=? WHERE id=? AND revision=?`, activation, now, row.id, expectedRevision); err != nil {
			return err
		}
		if _, err = c.ExecContext(ctx, `UPDATE enrollment_processing_jobs SET state='FINISHED',lease_until=NULL,last_error_code=NULL WHERE submission_id=? AND attempt_id=?`, row.id, attempt); err != nil {
			return err
		}
		if err = s.purge(ctx, c, Request{}, row.personID, row.id, "ACTIVATED", false); err != nil {
			return err
		}
		return s.record(ctx, c, Request{}, "enrollment.activated", "face_enrollment_submission", row.id, "")
	})
	if err != nil {
		// Preserve transactional audit failures exactly: no unaudited ACTIVE or partial template survives.
		return safe(err)
	}
	return nil
}
func (s *Service) finishFailure(ctx context.Context, row submission, attempt, code string) error {
	// Request cancellation cannot strand a running durable lease; use a bounded cleanup context.
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return safe(s.transaction(cleanup, func(c *sql.Conn) error {
		current, err := s.submission(cleanup, c, row.id)
		if err != nil {
			return err
		}
		if current.status != "APPROVED_PROCESSING" || current.revision != row.revision {
			return nil
		}
		var currentAttempt, state string
		if err = c.QueryRowContext(cleanup, `SELECT attempt_id,state FROM enrollment_processing_jobs WHERE submission_id=?`, row.id).Scan(&currentAttempt, &state); err != nil {
			return err
		}
		if attempt != currentAttempt || state != "RUNNING" {
			return nil
		}
		return s.failProcessing(cleanup, c, current, code)
	}))
}

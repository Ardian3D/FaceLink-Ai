package enrollment

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollmentsecure"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/ids"
)

type submission struct {
	id, personID, userID, sessionID, consentID, status, submitted, expires, photoHash    string
	revision, epoch, sessionEpoch                                                        int64
	decision, reviewer, reviewed, method, ref, rejection, processing, activation, reason string
	reviewerEpoch                                                                        int64
}

const submissionColumns = `id,person_id,subject_user_id,subject_session_id,consent_id,status,submitted_at,photo_expires_at,canonical_sha256,revision,captured_epoch,subject_session_epoch,COALESCE(review_decision,''),COALESCE(reviewer_admin_id,''),COALESCE(reviewed_at,''),COALESCE(identity_check_method,''),COALESCE(campus_check_ref,''),COALESCE(rejection_reason_code,''),COALESCE(processing_error_code,''),COALESCE(activated_enrollment_id,''),COALESCE(revoked_reason_code,''),COALESCE(reviewer_epoch,0)`

func scanSubmission(row interface{ Scan(...any) error }) (submission, error) {
	v := submission{}
	err := row.Scan(&v.id, &v.personID, &v.userID, &v.sessionID, &v.consentID, &v.status, &v.submitted, &v.expires, &v.photoHash, &v.revision, &v.epoch, &v.sessionEpoch, &v.decision, &v.reviewer, &v.reviewed, &v.method, &v.ref, &v.rejection, &v.processing, &v.activation, &v.reason, &v.reviewerEpoch)
	return v, err
}
func (s *Service) submission(ctx context.Context, c *sql.Conn, id string) (submission, error) {
	v, err := scanSubmission(c.QueryRowContext(ctx, `SELECT `+submissionColumns+` FROM face_enrollment_submissions WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return v, failure("NOT_FOUND", 404)
	}
	return v, err
}
func currentNotice() NoticeView {
	sum := sha256.Sum256([]byte(NoticeText))
	return NoticeView{Version: NoticeVersion, Digest: hex.EncodeToString(sum[:]), Text: NoticeText}
}
func (s *Service) consent(ctx context.Context, c *sql.Conn, p person) (ConsentView, error) {
	v := ConsentView{State: "NOT_GIVEN", Notice: currentNotice(), Revision: p.revision}
	var state string
	var granted, withdrawn sql.NullString
	err := c.QueryRowContext(ctx, `SELECT id,status,granted_at,withdrawn_at FROM consent_records WHERE person_id=? AND purpose='face_enrollment' ORDER BY CASE WHEN status='granted' THEN 0 ELSE 1 END,created_at DESC,rowid DESC LIMIT 1`, p.id).Scan(&v.RecordID, &state, &granted, &withdrawn)
	if errors.Is(err, sql.ErrNoRows) {
		return v, nil
	}
	if err != nil {
		return v, err
	}
	if state == "granted" {
		v.State = "ACTIVE"
	} else {
		v.State = "WITHDRAWN"
	}
	v.GrantedAt = granted.String
	v.WithdrawnAt = withdrawn.String
	return v, nil
}
func (s *Service) validConsent(ctx context.Context, c *sql.Conn, p person, id string) error {
	var state, version, digest, subject, policy string
	err := c.QueryRowContext(ctx, `SELECT status,COALESCE(notice_version,''),COALESCE(notice_digest,''),COALESCE(subject_user_id,''),COALESCE(policy_version,'') FROM consent_records WHERE id=? AND person_id=? AND purpose='face_enrollment'`, id, p.id).Scan(&state, &version, &digest, &subject, &policy)
	if errors.Is(err, sql.ErrNoRows) {
		return failure("CONSENT_REQUIRED", 422)
	}
	if err != nil {
		return err
	}
	if state != "granted" {
		return failure("CONSENT_WITHDRAWN", 422)
	}
	if version != NoticeVersion || digest != currentNotice().Digest || subject != p.user || policy != "synthetic-v1" {
		return failure("CONSENT_VERSION_MISMATCH", 409)
	}
	return nil
}

// roster reads only the requested page/person set in the enclosing transaction.
// Both detail joins share the same slice; queue pages do not issue N roster queries.
func roster(ctx context.Context, c *sql.Conn, personIDs []string) (map[string][]ClassRef, error) {
	result := make(map[string][]ClassRef, len(personIDs))
	args := make([]any, 0, len(personIDs))
	for _, id := range personIDs {
		if _, exists := result[id]; !exists {
			result[id] = []ClassRef{}
			args = append(args, id)
		}
	}
	if len(args) == 0 {
		return result, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(args)), ",")
	rows, err := c.QueryContext(ctx, `SELECT m.person_id,k.id,k.code,k.title FROM class_memberships m JOIN classes k ON k.id=m.class_id WHERE m.person_id IN (`+placeholders+`) AND m.left_at IS NULL AND k.status='active' AND k.deleted_at IS NULL ORDER BY k.code,k.id,m.person_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var personID string
		var class ClassRef
		if err := rows.Scan(&personID, &class.ID, &class.Code, &class.Title); err != nil {
			return nil, err
		}
		result[personID] = append(result[personID], class)
	}
	return result, rows.Err()
}
func (s *Service) view(ctx context.Context, c *sql.Conn, p person, id string, admin bool) (EnrollmentView, error) {
	consent, err := s.consent(ctx, c, p)
	if err != nil {
		return EnrollmentView{}, err
	}
	now := nowText(s.clk.Now())
	v := EnrollmentView{ServerNow: now, EffectiveStatus: "NOT_ENROLLED", ConsentState: consent.State, IdentityState: "UNREVIEWED", DeletionState: "NONE", Revision: p.revision, AllowedActions: []string{}, SyntheticOnly: true}
	if admin {
		classes, err := roster(ctx, c, []string{p.id})
		if err != nil {
			return v, err
		}
		binding := &IdentityBindingView{AccountUserID: p.user, PersonID: p.id, ExternalCode: p.code, Eligible: p.eligible, Classes: classes[p.id]}
		err = c.QueryRowContext(ctx, `SELECT COALESCE(u.status,''),COALESCE(u.role,''),p.status,COALESCE(p.identity_reference,''),COALESCE(p.identity_verified_at,'') FROM people p LEFT JOIN users u ON u.id=p.account_user_id WHERE p.id=?`, p.id).Scan(&binding.AccountStatus, &binding.AccountRole, &binding.PersonStatus, &binding.IdentityReference, &binding.IdentityVerifiedAt)
		if err != nil {
			return v, err
		}
		v.Student = &StudentView{UserID: p.user, PersonID: p.id, DisplayName: p.name, ExternalCode: p.code, Classes: classes[p.id]}
		v.Consent = &consent
		v.IdentityBinding = binding
		v.ArtifactIntegrity = "UNAVAILABLE"
	}
	var state string
	err = c.QueryRowContext(ctx, `SELECT state FROM biometric_deletion_jobs WHERE person_id=?`, p.id).Scan(&state)
	if err == nil {
		v.DeletionState = state
	} else if !errors.Is(err, sql.ErrNoRows) {
		return v, err
	}
	var row submission
	if id != "" {
		row, err = s.submission(ctx, c, id)
	} else {
		row, err = scanSubmission(c.QueryRowContext(ctx, `SELECT `+submissionColumns+` FROM face_enrollment_submissions WHERE person_id=? ORDER BY submitted_at DESC,rowid DESC LIMIT 1`, p.id))
	}
	if errors.Is(err, sql.ErrNoRows) {
		if p.eligible {
			v.AllowedActions = append(v.AllowedActions, "GRANT_CONSENT")
			if consent.State == "ACTIVE" {
				v.AllowedActions = append(v.AllowedActions, "SUBMIT", "WITHDRAW")
			}
		}
		return v, nil
	}
	if err != nil {
		return v, err
	}
	v.SubmissionID = row.id
	v.SubmissionStatus = row.status
	v.EffectiveStatus = row.status
	v.Revision = row.revision
	v.SubmittedAt = row.submitted
	v.ReviewedAt = row.reviewed
	v.PhotoExpiresAt = row.expires
	if row.decision == "APPROVED" {
		v.IdentityState = "ADMIN_ATTESTED"
	} else if row.decision == "REJECTED" {
		v.IdentityState = "REJECTED"
	}
	v.ReasonCode = row.reason
	if v.ReasonCode == "" {
		v.ReasonCode = row.processing
	}
	if v.ReasonCode == "" {
		v.ReasonCode = row.rejection
	}
	if consent.State == "WITHDRAWN" {
		v.EffectiveStatus = "REVOKED"
	}
	var count int
	err = c.QueryRowContext(ctx, `SELECT COUNT(*) FROM face_enrollment_photos WHERE submission_id=? AND storage_state='READY' AND expires_at>?`, row.id, now).Scan(&count)
	if err != nil {
		return v, err
	}
	v.PhotoAvailable = count == 1 && p.eligible && p.epoch == row.epoch && s.validConsent(ctx, c, p, row.consentID) == nil
	if admin && v.PhotoAvailable {
		// Metadata presence is not byte/hash verification; only photo/approve do that.
		v.ArtifactIntegrity = "NOT_CHECKED"
	}
	if row.status == "APPROVED_PROCESSING" {
		var deadline sql.NullString
		err = c.QueryRowContext(ctx, `SELECT lease_until FROM enrollment_processing_jobs WHERE submission_id=?`, row.id).Scan(&deadline)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return v, err
		}
		v.ProcessingDeadline = deadline.String
	}
	if row.status == "ACTIVE" && p.eligible && row.epoch == p.epoch && s.validConsent(ctx, c, p, row.consentID) == nil {
		v.TemplateReady = s.templateReady(ctx, c, row)
	}
	if row.status == "ACTIVE" && !v.TemplateReady && v.ReasonCode == "" {
		v.ReasonCode = "BIOMETRIC_UNAVAILABLE"
	}
	if p.eligible && consent.State == "ACTIVE" {
		v.AllowedActions = append(v.AllowedActions, "WITHDRAW")
	}
	if (row.status == "REJECTED" || row.status == "REVOKED" || row.status == "EXPIRED" || row.status == "FAILED_PROCESSING") && v.DeletionState == "COMPLETE" && p.eligible {
		v.AllowedActions = append(v.AllowedActions, "GRANT_CONSENT")
	}
	if p.eligible && consent.State == "ACTIVE" && consent.RecordID != row.consentID && (v.DeletionState == "NONE" || v.DeletionState == "COMPLETE") {
		v.AllowedActions = append(v.AllowedActions, "SUBMIT")
	}
	if admin && row.status == "PENDING_APPROVAL" && v.PhotoAvailable {
		v.AllowedActions = append(v.AllowedActions, "APPROVE", "REJECT")
	}
	if admin && row.status == "FAILED_PROCESSING" && row.processing == "BIOMETRIC_UNAVAILABLE" && v.PhotoAvailable {
		valid, err := s.reviewerValid(ctx, c, row)
		if err != nil {
			return v, err
		}
		var attempts int
		if err = c.QueryRowContext(ctx, `SELECT attempts_started FROM enrollment_processing_jobs WHERE submission_id=?`, row.id).Scan(&attempts); err != nil {
			return v, err
		}
		if valid && attempts < 3 {
			v.AllowedActions = append(v.AllowedActions, "RETRY_PROCESSING")
		}
	}
	if admin && row.decision != "" {
		v.Review = &ReviewView{ReviewerAdminID: row.reviewer, Decision: row.decision, At: row.reviewed, IdentityCheckMethod: row.method, CampusCheckRef: row.ref}
	}
	if admin {
		rows, err := c.QueryContext(ctx, `SELECT id,action,COALESCE(actor_id,''),occurred_at,COALESCE(json_extract(metadata_json,'$.reason_code'),'') FROM audit_events WHERE (entity_type='face_enrollment_submission' AND entity_id=?) OR (entity_type IN ('person','consent_record') AND entity_id IN (?,?)) ORDER BY occurred_at DESC,id DESC LIMIT 50`, row.id, p.id, row.consentID)
		if err != nil {
			return v, err
		}
		defer rows.Close()
		for rows.Next() {
			h := HistoryView{}
			if err := rows.Scan(&h.AuditID, &h.Action, &h.ActorID, &h.Time, &h.ReasonCode); err != nil {
				return v, err
			}
			v.History = append(v.History, h)
		}
		if err := rows.Err(); err != nil {
			return v, err
		}
	}
	return v, nil
}
func (s *Service) templateReady(ctx context.Context, c *sql.Conn, row submission) bool {
	if s.keys == nil {
		return false
	}
	var e enrollmentsecure.Envelope
	var templateID, submissionID, personID, consentID string
	var manifest Manifest
	// Reconstruct AAD exclusively from joined database identity and pipeline
	// metadata. Stored envelope AAD is authenticated input, never identity authority.
	err := c.QueryRowContext(ctx, `SELECT t.id,s.id,p.id,g.id,
		t.envelope_version,t.algorithm,t.key_version,t.aad,t.nonce,t.ciphertext,
		t.synthetic_only,t.model_id,t.model_version,t.model_checksum,
		t.detector_id,t.detector_version,t.detector_checksum,t.pipeline_revision
		FROM face_templates t
		JOIN face_enrollment_submissions s ON s.id=t.submission_id AND s.person_id=t.person_id AND s.consent_id=t.consent_id
		JOIN people p ON p.id=s.person_id AND p.account_user_id=s.subject_user_id AND p.biometric_epoch=s.captured_epoch
		JOIN consent_records g ON g.id=s.consent_id AND g.person_id=p.id AND g.subject_user_id=s.subject_user_id
		JOIN face_enrollments a ON a.id=t.activation_enrollment_id AND a.id=s.activated_enrollment_id AND a.person_id=p.id
		WHERE s.id=? AND p.id=? AND g.id=? AND s.status='ACTIVE' AND s.review_decision='APPROVED'
		AND g.status='granted' AND g.purpose='face_enrollment' AND g.method='digital_explicit'
		AND a.status='active' AND a.model_name=t.model_id AND a.model_version=t.model_version
		AND lower(a.model_checksum)=lower(t.model_checksum) AND t.embedding_dim=128 AND t.dtype='float32-le' AND t.synthetic_only=1`,
		row.id, row.personID, row.consentID).Scan(&templateID, &submissionID, &personID, &consentID,
		&e.Version, &e.Algorithm, &e.KeyVersion, &e.AAD, &e.Nonce, &e.Ciphertext,
		&manifest.SyntheticOnly, &manifest.ModelID, &manifest.ModelVersion, &manifest.ModelChecksum,
		&manifest.DetectorID, &manifest.DetectorVersion, &manifest.DetectorChecksum, &manifest.PipelineRevision)
	if err != nil || !validManifest(manifest) {
		return false
	}
	plain, err := enrollmentsecure.Open(ctx, s.keys, enrollmentsecure.Binding{Purpose: "template", RecordID: templateID, SubmissionID: submissionID, PersonID: personID, ConsentID: consentID, ModelID: manifest.ModelID, ModelVersion: manifest.ModelVersion, ModelFingerprint: manifest.Fingerprint()}, e)
	defer wipe(plain)
	return err == nil && len(plain) == 512
}
func wipe(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
func (s *Service) detail(ctx context.Context, c *sql.Conn, r Request) (Result, error) {
	row, err := s.submission(ctx, c, r.TargetID)
	if err != nil {
		return Result{}, err
	}
	p, err := s.person(ctx, c, row.personID, false, false)
	if err != nil {
		return Result{}, err
	}
	if err = s.record(ctx, c, r, "enrollment.review_opened", "face_enrollment_submission", row.id, ""); err != nil {
		return Result{}, err
	}
	v, err := s.view(ctx, c, p, row.id, true)
	return Result{HTTPStatus: 200, Data: v, ETag: etag(row.revision)}, err
}
func (s *Service) list(ctx context.Context, c *sql.Conn, r Request) (Result, error) {
	limit := r.Limit
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 50 {
		return Result{}, failure("VALIDATION_FAILED", 400)
	}
	if r.FilterStatus != "" && !strings.Contains("|PENDING_APPROVAL|APPROVED_PROCESSING|ACTIVE|REJECTED|FAILED_PROCESSING|REVOKED|EXPIRED|", "|"+r.FilterStatus+"|") {
		return Result{}, failure("VALIDATION_FAILED", 400)
	}
	cursor := struct{ At, ID string }{}
	if r.Cursor != "" {
		data, err := base64.RawURLEncoding.DecodeString(r.Cursor)
		if err != nil || len(data) > 256 || json.Unmarshal(data, &cursor) != nil || !uuid(cursor.ID) {
			return Result{}, failure("VALIDATION_FAILED", 400)
		}
		if _, err = time.Parse(time.RFC3339Nano, cursor.At); err != nil {
			return Result{}, failure("VALIDATION_FAILED", 400)
		}
	}
	v := ListView{ServerNow: nowText(s.clk.Now()), Items: []ListItem{}, Page: Page{Limit: limit}}
	rows, err := c.QueryContext(ctx, `SELECT s.id,s.subject_user_id,s.person_id,p.display_name,p.external_code,s.status,s.submitted_at,s.photo_expires_at FROM face_enrollment_submissions s JOIN people p ON p.id=s.person_id WHERE (?='' OR s.status=?) AND (?='' OR s.submitted_at>? OR (s.submitted_at=? AND s.id>?)) ORDER BY s.submitted_at,s.id LIMIT ?`, r.FilterStatus, r.FilterStatus, cursor.At, cursor.At, cursor.At, cursor.ID, limit+1)
	if err != nil {
		return Result{}, err
	}
	for rows.Next() {
		item := ListItem{}
		if err = rows.Scan(&item.ID, &item.Student.UserID, &item.Student.PersonID, &item.Student.DisplayName, &item.Student.ExternalCode, &item.Status, &item.SubmittedAt, &item.PhotoExpiresAt); err != nil {
			rows.Close()
			return Result{}, err
		}
		v.Items = append(v.Items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return Result{}, err
	}
	if len(v.Items) > limit {
		v.Items = v.Items[:limit]
		v.Page.HasMore = true
		last := v.Items[len(v.Items)-1]
		encoded, _ := json.Marshal(struct{ At, ID string }{last.SubmittedAt, last.ID})
		v.Page.NextCursor = base64.RawURLEncoding.EncodeToString(encoded)
	}
	personIDs := make([]string, len(v.Items))
	for i := range v.Items {
		personIDs[i] = v.Items[i].Student.PersonID
	}
	classes, err := roster(ctx, c, personIDs)
	if err != nil {
		return Result{}, err
	}
	for i := range v.Items {
		v.Items[i].Student.Classes = classes[v.Items[i].Student.PersonID]
	}
	if err = c.QueryRowContext(ctx, `SELECT COUNT(*) FROM face_enrollment_submissions WHERE status='PENDING_APPROVAL'`).Scan(&v.Summary.PendingCount); err != nil {
		return Result{}, err
	}
	if err = s.record(ctx, c, r, "enrollment.queue_read", "enrollment_queue", "", ""); err != nil {
		return Result{}, err
	}
	return Result{HTTPStatus: 200, Data: v}, nil
}
func (s *Service) grant(ctx context.Context, c *sql.Conn, r Request, p person) (Result, error) {
	if r.IfMatch == "" {
		return Result{}, failure("PRECONDITION_REQUIRED", 428)
	}
	if r.IfMatch != etag(p.revision) {
		return Result{}, failure("PRECONDITION_FAILED", 412)
	}
	if !r.Accepted {
		return Result{}, failure("VALIDATION_FAILED", 422)
	}
	if r.NoticeVersion != NoticeVersion {
		return Result{}, failure("CONSENT_VERSION_MISMATCH", 409)
	}
	var live int
	if err := c.QueryRowContext(ctx, `SELECT COUNT(*) FROM face_enrollment_submissions WHERE person_id=? AND status IN ('PENDING_APPROVAL','APPROVED_PROCESSING','ACTIVE')`, p.id).Scan(&live); err != nil {
		return Result{}, err
	}
	if live > 0 {
		return Result{}, failure("ENROLLMENT_EXISTS", 409)
	}
	var pending int
	if err := c.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM face_enrollment_photos f JOIN face_enrollment_submissions x ON x.id=f.submission_id WHERE x.person_id=?)+(SELECT COUNT(*) FROM face_templates WHERE person_id=?)+(SELECT COUNT(*) FROM biometric_deletion_jobs WHERE person_id=? AND state<>'COMPLETE')`, p.id, p.id, p.id).Scan(&pending); err != nil {
		return Result{}, err
	}
	if pending > 0 {
		return Result{}, failure("DELETION_INCOMPLETE", 409)
	}
	old, err := s.consent(ctx, c, p)
	if err != nil {
		return Result{}, err
	}
	if old.State == "ACTIVE" {
		if r.ReplacesConsentID != old.RecordID {
			return Result{}, failure("CONSENT_CONFLICT", 409)
		}
		var submitted int
		if err = c.QueryRowContext(ctx, `SELECT COUNT(*) FROM face_enrollment_submissions WHERE consent_id=?`, old.RecordID).Scan(&submitted); err != nil {
			return Result{}, err
		}
		if submitted == 0 {
			return Result{}, failure("CONSENT_CONFLICT", 409)
		}
		if _, err = c.ExecContext(ctx, `UPDATE consent_records SET status='withdrawn',withdrawn_at=?,withdrawn_by=?,withdraw_reason_code='REPLACED_BY_NEW_GRANT' WHERE id=?`, nowText(s.clk.Now()), r.ActorID, old.RecordID); err != nil {
			return Result{}, err
		}
		if err = s.record(ctx, c, r, "consent.withdrawn", "consent_record", old.RecordID, "REPLACED_BY_NEW_GRANT"); err != nil {
			return Result{}, err
		}
	} else if r.ReplacesConsentID != "" && r.ReplacesConsentID != old.RecordID {
		return Result{}, failure("CONSENT_CONFLICT", 409)
	}
	// Failed processing is terminal in this conservative policy: its photo was already purged.
	if _, err = c.ExecContext(ctx, `UPDATE face_enrollment_submissions SET status='REVOKED',revoked_reason_code='REPLACED_BY_NEW_GRANT',revision=revision+1,updated_at=? WHERE person_id=? AND status='FAILED_PROCESSING'`, nowText(s.clk.Now()), p.id); err != nil {
		return Result{}, err
	}
	id := ids.New()
	notice := currentNotice()
	_, err = c.ExecContext(ctx, `INSERT INTO consent_records(id,person_id,purpose,status,granted_at,method,recorded_by,created_at,notice_version,notice_digest,subject_user_id,policy_version) VALUES(?,?,'face_enrollment','granted',?,'digital_explicit',?,?,?,?,?,'synthetic-v1')`, id, p.id, nowText(s.clk.Now()), r.ActorID, nowText(s.clk.Now()), notice.Version, notice.Digest, r.ActorID)
	if err != nil {
		return Result{}, err
	}
	if _, err = c.ExecContext(ctx, `UPDATE people SET consent_state='granted',consent_revision=consent_revision+1,biometric_epoch=biometric_epoch+1 WHERE id=?`, p.id); err != nil {
		return Result{}, err
	}
	p.revision++
	if err = s.record(ctx, c, r, "consent.granted", "consent_record", id, ""); err != nil {
		return Result{}, err
	}
	v, err := s.consent(ctx, c, p)
	return receipt(id, p.revision, v, 201), err
}

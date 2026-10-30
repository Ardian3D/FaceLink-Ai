package enrollment

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/ids"
)

func (s *Service) deletion(ctx context.Context, c *sql.Conn, r Request, personID, reason string) error {
	_, err := c.ExecContext(ctx, `INSERT INTO biometric_deletion_jobs(person_id,requested_at,reason_code,state) VALUES(?,?,?,'PENDING') ON CONFLICT(person_id) DO UPDATE SET requested_at=excluded.requested_at,reason_code=excluded.reason_code,state='PENDING',completed_at=NULL,last_error_code=NULL,revision=biometric_deletion_jobs.revision+1`, personID, nowText(s.clk.Now()), reason)
	if err != nil {
		return err
	}
	return s.record(ctx, c, r, "deletion.requested", "person", personID, reason)
}
func (s *Service) purge(ctx context.Context, c *sql.Conn, r Request, personID, submissionID, reason string, templates bool) error {
	filter := `submission_id=?`
	arg := submissionID
	if submissionID == "" {
		filter = `submission_id IN (SELECT id FROM face_enrollment_submissions WHERE person_id=?)`
		arg = personID
	}
	result, err := c.ExecContext(ctx, `DELETE FROM face_enrollment_photos WHERE `+filter, arg)
	if err != nil {
		return err
	}
	photos, err := result.RowsAffected()
	if err != nil {
		return err
	}
	var removed int64
	if templates {
		result, err = c.ExecContext(ctx, `DELETE FROM face_templates WHERE `+filter, arg)
		if err != nil {
			return err
		}
		removed, err = result.RowsAffected()
		if err != nil {
			return err
		}
		if submissionID == "" {
			_, err = c.ExecContext(ctx, `UPDATE face_enrollments SET status='revoked',revoked_at=?,revoked_reason=?,updated_at=? WHERE person_id=? AND status='active'`, nowText(s.clk.Now()), reason, nowText(s.clk.Now()), personID)
		} else {
			_, err = c.ExecContext(ctx, `UPDATE face_enrollments SET status='revoked',revoked_at=?,revoked_reason=?,updated_at=? WHERE id IN (SELECT activated_enrollment_id FROM face_enrollment_submissions WHERE id=?) AND status='active'`, nowText(s.clk.Now()), reason, nowText(s.clk.Now()), submissionID)
		}
		if err != nil {
			return err
		}
	}
	entity, id := "face_enrollment_submission", submissionID
	if id == "" {
		entity, id = "person", personID
	}
	if photos > 0 {
		if err = s.record(ctx, c, r, "photo.deleted", entity, id, reason); err != nil {
			return err
		}
	}
	if removed > 0 {
		if err = s.record(ctx, c, r, "template.deleted", entity, id, reason); err != nil {
			return err
		}
	}
	if photos+removed > 0 {
		return s.deletion(ctx, c, r, personID, reason)
	}
	return nil
}
func (s *Service) withdraw(ctx context.Context, c *sql.Conn, r Request, p person) (Result, error) {
	if !r.Confirmed || r.Scope != "ALL_FACE_ENROLLMENT" {
		return Result{}, failure("VALIDATION_FAILED", 422)
	}
	reason := "STUDENT_REQUEST"
	if r.Action == AssistedWithdraw {
		if r.RequestMethod != "CAMPUS_STUDENT_REQUEST" || !uuid(r.RequestRef) {
			return Result{}, failure("VALIDATION_FAILED", 422)
		}
		reason = "STUDENT_ASSISTED_REQUEST"
	}
	var count int
	if err := c.QueryRowContext(ctx, `SELECT COUNT(*) FROM consent_records WHERE person_id=? AND purpose='face_enrollment' AND status='granted'`, p.id).Scan(&count); err != nil {
		return Result{}, err
	}
	result, err := c.ExecContext(ctx, `UPDATE face_enrollment_submissions SET status='REVOKED',revoked_reason_code=?,revision=revision+1,updated_at=? WHERE person_id=? AND status IN ('PENDING_APPROVAL','APPROVED_PROCESSING','FAILED_PROCESSING','ACTIVE')`, reason, nowText(s.clk.Now()), p.id)
	if err != nil {
		return Result{}, err
	}
	revoked, _ := result.RowsAffected()
	if _, err = c.ExecContext(ctx, `UPDATE consent_records SET status='withdrawn',withdrawn_at=?,withdrawn_by=?,withdraw_reason_code=? WHERE person_id=? AND purpose='face_enrollment' AND status='granted'`, nowText(s.clk.Now()), r.ActorID, reason, p.id); err != nil {
		return Result{}, err
	}
	if _, err = c.ExecContext(ctx, `UPDATE enrollment_processing_jobs SET state='FINISHED',lease_until=NULL,last_error_code=? WHERE submission_id IN (SELECT id FROM face_enrollment_submissions WHERE person_id=?) AND state<>'FINISHED'`, reason, p.id); err != nil {
		return Result{}, err
	}
	if count > 0 || revoked > 0 {
		if _, err = c.ExecContext(ctx, `UPDATE people SET consent_state='withdrawn',consent_revision=consent_revision+1,biometric_epoch=biometric_epoch+1 WHERE id=?`, p.id); err != nil {
			return Result{}, err
		}
		p.revision++
		if count > 0 {
			if err = s.record(ctx, c, r, "consent.withdrawn", "person", p.id, reason); err != nil {
				return Result{}, err
			}
		}
		if revoked > 0 {
			if err = s.record(ctx, c, r, "enrollment.revoked", "person", p.id, reason); err != nil {
				return Result{}, err
			}
		}
	}
	if err = s.purge(ctx, c, r, p.id, "", reason, true); err != nil {
		return Result{}, err
	}
	p, err = s.person(ctx, c, p.id, false, false)
	if err != nil {
		return Result{}, err
	}
	v, err := s.view(ctx, c, p, "", r.Action == AssistedWithdraw)
	return receipt(p.id, p.revision, v, 200), err
}
func (s *Service) reviewerValid(ctx context.Context, c *sql.Conn, row submission) (bool, error) {
	var count int
	err := c.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id=? AND role='admin' AND status='active' AND enrollment_auth_epoch=?`, row.reviewer, row.reviewerEpoch).Scan(&count)
	return count == 1, err
}
func (s *Service) subjectValid(ctx context.Context, c *sql.Conn, row submission) (bool, error) {
	p, err := s.person(ctx, c, row.personID, false, false)
	if err != nil {
		return false, err
	}
	if !p.eligible || p.user != row.userID || p.epoch != row.epoch {
		return false, nil
	}
	if err = s.validConsent(ctx, c, p, row.consentID); err != nil {
		var e *Error
		if errors.As(err, &e) {
			return false, nil
		}
		return false, err
	}
	// Durable review is independent of the uploading access token's lifetime.
	// Revocation events are still fenced monotonically by the schema epoch hook.
	return true, nil
}
func (s *Service) failProcessing(ctx context.Context, c *sql.Conn, row submission, code string) error {
	if row.status != "APPROVED_PROCESSING" {
		return nil
	}
	if _, err := c.ExecContext(ctx, `UPDATE face_enrollment_submissions SET status='FAILED_PROCESSING',processing_error_code=?,revision=revision+1,updated_at=? WHERE id=? AND status='APPROVED_PROCESSING' AND revision=?`, code, nowText(s.clk.Now()), row.id, row.revision); err != nil {
		return err
	}
	if _, err := c.ExecContext(ctx, `UPDATE enrollment_processing_jobs SET state='FINISHED',lease_until=NULL,last_error_code=? WHERE submission_id=?`, code, row.id); err != nil {
		return err
	}
	if err := s.record(ctx, c, Request{}, "processing.failed", "face_enrollment_submission", row.id, code); err != nil {
		return err
	}
	// A missing key or an integrity incident is never an automatic disposal authorization.
	if code == "BIOMETRIC_UNAVAILABLE" {
		return nil
	}
	return s.purge(ctx, c, Request{}, row.personID, row.id, code, false)
}
func (s *Service) maintain(ctx context.Context, c *sql.Conn) error {
	rows, err := c.QueryContext(ctx, `SELECT `+submissionColumns+` FROM face_enrollment_submissions WHERE status IN ('PENDING_APPROVAL','APPROVED_PROCESSING','FAILED_PROCESSING')`)
	if err != nil {
		return err
	}
	var work []submission
	for rows.Next() {
		row, err := scanSubmission(rows)
		if err != nil {
			rows.Close()
			return err
		}
		work = append(work, row)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, row := range work {
		deadline, err := time.Parse(time.RFC3339Nano, row.expires)
		if err != nil {
			return err
		}
		reason, status := "", ""
		if !s.clk.Now().Before(deadline) {
			reason, status = "PHOTO_RETENTION_EXPIRED", "EXPIRED"
		} else if row.status == "APPROVED_PROCESSING" {
			valid, err := s.reviewerValid(ctx, c, row)
			if err != nil {
				return err
			}
			if !valid {
				if err = s.failProcessing(ctx, c, row, "REVIEWER_INELIGIBLE"); err != nil {
					return err
				}
				continue
			}
		}
		if reason == "" {
			valid, err := s.subjectValid(ctx, c, row)
			if err != nil {
				return err
			}
			if !valid {
				reason, status = "SUBJECT_INELIGIBLE", "REVOKED"
			}
		}
		if reason != "" {
			if _, err = c.ExecContext(ctx, `UPDATE face_enrollment_submissions SET status=?,revoked_reason_code=?,revision=revision+1,updated_at=? WHERE id=? AND revision=?`, status, reason, nowText(s.clk.Now()), row.id, row.revision); err != nil {
				return err
			}
			if _, err = c.ExecContext(ctx, `UPDATE enrollment_processing_jobs SET state='FINISHED',lease_until=NULL,last_error_code=? WHERE submission_id=?`, reason, row.id); err != nil {
				return err
			}
			if err = s.purge(ctx, c, Request{}, row.personID, row.id, reason, false); err != nil {
				return err
			}
			action := "enrollment.revoked"
			if status == "EXPIRED" {
				action = "enrollment.expired"
			}
			if err = s.record(ctx, c, Request{}, action, "face_enrollment_submission", row.id, reason); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *Service) decide(ctx context.Context, c *sql.Conn, r Request) (Result, error) {
	row, err := s.submission(ctx, c, r.TargetID)
	if err != nil {
		return Result{}, err
	}
	p, err := s.person(ctx, c, row.personID, false, true)
	if err != nil {
		return Result{}, err
	}
	if r.IfMatch == "" {
		return Result{}, failure("PRECONDITION_REQUIRED", 428)
	}
	if r.IfMatch != etag(row.revision) {
		return Result{}, failure("PRECONDITION_FAILED", 412)
	}
	if r.Action == Retry {
		return s.retry(ctx, c, r, p, row)
	}
	if row.status != "PENDING_APPROVAL" {
		return Result{}, failure("INVALID_TRANSITION", 409)
	}
	valid, err := s.subjectValid(ctx, c, row)
	if err != nil {
		return Result{}, err
	}
	if !valid {
		return Result{}, failure("SUBJECT_INELIGIBLE", 409)
	}
	if r.Action == Approve {
		if !r.Confirmed || r.IdentityCheckMethod != "CAMPUS_AUTHORIZED_CHECK" || r.AttestationVersion != "synthetic-v1" || !uuid(r.CampusCheckRef) {
			return Result{}, failure("VALIDATION_FAILED", 422)
		}
		if row.userID == r.ActorID {
			return Result{}, failure("SELF_REVIEW_FORBIDDEN", 403)
		}
		if s.keys == nil || s.files == nil || s.processor == nil || !validManifest(s.processor.Manifest()) {
			return Result{}, failure("BIOMETRIC_UNAVAILABLE", 503)
		}
		if r.verifiedPhotoHash != row.photoHash {
			return Result{}, failure("BIOMETRIC_UNAVAILABLE", 503)
		}
		var available int
		if err = c.QueryRowContext(ctx, `SELECT COUNT(*) FROM face_enrollment_photos WHERE submission_id=? AND storage_state='READY' AND expires_at>?`, row.id, nowText(s.clk.Now())).Scan(&available); err != nil {
			return Result{}, err
		}
		if available != 1 {
			return Result{}, failure("PHOTO_UNAVAILABLE", 409)
		}
		var queued int
		if err = c.QueryRowContext(ctx, `SELECT COUNT(*) FROM enrollment_processing_jobs WHERE state IN ('QUEUED','RUNNING')`).Scan(&queued); err != nil {
			return Result{}, err
		}
		if queued >= 10 {
			return Result{}, failure("QUEUE_FULL", 409)
		}
		var reviewerEpoch int64
		if err = c.QueryRowContext(ctx, `SELECT enrollment_auth_epoch FROM users WHERE id=?`, r.ActorID).Scan(&reviewerEpoch); err != nil {
			return Result{}, err
		}
		if _, err = c.ExecContext(ctx, `UPDATE face_enrollment_submissions SET status='APPROVED_PROCESSING',review_decision='APPROVED',reviewer_admin_id=?,reviewer_epoch=?,reviewed_at=?,identity_check_method=?,identity_attestation_version=?,campus_check_ref=?,revision=revision+1,updated_at=? WHERE id=? AND revision=? AND status='PENDING_APPROVAL'`, r.ActorID, reviewerEpoch, nowText(s.clk.Now()), r.IdentityCheckMethod, r.AttestationVersion, r.CampusCheckRef, nowText(s.clk.Now()), row.id, row.revision); err != nil {
			return Result{}, err
		}
		if _, err = c.ExecContext(ctx, `INSERT INTO enrollment_processing_jobs(submission_id,attempt_id,attempts_started,expected_revision,expected_epoch,expected_photo_hash,state) VALUES(?,?,1,?,?,?,'QUEUED')`, row.id, ids.New(), row.revision+1, row.epoch, row.photoHash); err != nil {
			return Result{}, err
		}
		if err = s.record(ctx, c, r, "enrollment.approved", "face_enrollment_submission", row.id, ""); err != nil {
			return Result{}, err
		}
		if err = s.record(ctx, c, r, "processing.queued", "face_enrollment_submission", row.id, ""); err != nil {
			return Result{}, err
		}
	} else {
		if r.ReasonCode != "IDENTITY_MISMATCH" && r.ReasonCode != "EVIDENCE_INSUFFICIENT" && r.ReasonCode != "UNSUITABLE_SUBMISSION" {
			return Result{}, failure("VALIDATION_FAILED", 422)
		}
		var reviewerEpoch int64
		if err = c.QueryRowContext(ctx, `SELECT enrollment_auth_epoch FROM users WHERE id=?`, r.ActorID).Scan(&reviewerEpoch); err != nil {
			return Result{}, err
		}
		if _, err = c.ExecContext(ctx, `UPDATE face_enrollment_submissions SET status='REJECTED',review_decision='REJECTED',reviewer_admin_id=?,reviewer_epoch=?,reviewed_at=?,rejection_reason_code=?,revision=revision+1,updated_at=? WHERE id=? AND revision=? AND status='PENDING_APPROVAL'`, r.ActorID, reviewerEpoch, nowText(s.clk.Now()), r.ReasonCode, nowText(s.clk.Now()), row.id, row.revision); err != nil {
			return Result{}, err
		}
		if err = s.record(ctx, c, r, "enrollment.rejected", "face_enrollment_submission", row.id, r.ReasonCode); err != nil {
			return Result{}, err
		}
		if err = s.purge(ctx, c, r, p.id, row.id, "REJECTED", false); err != nil {
			return Result{}, err
		}
	}
	v, err := s.view(ctx, c, p, row.id, true)
	status := 200
	if r.Action == Approve {
		status = 202
	}
	return receipt(row.id, row.revision+1, v, status), err
}

func (s *Service) retry(ctx context.Context, c *sql.Conn, r Request, p person, row submission) (Result, error) {
	if !r.Confirmed {
		return Result{}, failure("VALIDATION_FAILED", 422)
	}
	if row.status != "FAILED_PROCESSING" || row.decision != "APPROVED" || row.processing != "BIOMETRIC_UNAVAILABLE" {
		return Result{}, failure("RETRY_NOT_ALLOWED", 409)
	}
	valid, err := s.subjectValid(ctx, c, row)
	if err != nil {
		return Result{}, err
	}
	if !valid {
		return Result{}, failure("RETRY_NOT_ALLOWED", 409)
	}
	valid, err = s.reviewerValid(ctx, c, row)
	if err != nil {
		return Result{}, err
	}
	if !valid {
		return Result{}, failure("RETRY_NOT_ALLOWED", 409)
	}
	if s.keys == nil || s.files == nil || s.processor == nil || !validManifest(s.processor.Manifest()) || r.verifiedPhotoHash != row.photoHash {
		return Result{}, failure("BIOMETRIC_UNAVAILABLE", 503)
	}
	var attempts int
	if err = c.QueryRowContext(ctx, `SELECT attempts_started FROM enrollment_processing_jobs WHERE submission_id=?`, row.id).Scan(&attempts); err != nil {
		return Result{}, err
	}
	if attempts >= 3 {
		return Result{}, failure("RETRY_EXHAUSTED", 409)
	}
	var count int
	if err = c.QueryRowContext(ctx, `SELECT COUNT(*) FROM face_enrollment_photos WHERE submission_id=? AND storage_state='READY' AND expires_at>? AND canonical_sha256=?`, row.id, nowText(s.clk.Now()), row.photoHash).Scan(&count); err != nil {
		return Result{}, err
	}
	if count != 1 {
		return Result{}, failure("PHOTO_UNAVAILABLE", 409)
	}
	if err = c.QueryRowContext(ctx, `SELECT COUNT(*) FROM enrollment_processing_jobs WHERE state IN ('QUEUED','RUNNING')`).Scan(&count); err != nil {
		return Result{}, err
	}
	if count >= 10 {
		return Result{}, failure("QUEUE_FULL", 409)
	}
	if _, err = c.ExecContext(ctx, `UPDATE face_enrollment_submissions SET status='APPROVED_PROCESSING',processing_error_code=NULL,revision=revision+1,updated_at=? WHERE id=? AND revision=?`, nowText(s.clk.Now()), row.id, row.revision); err != nil {
		return Result{}, err
	}
	if _, err = c.ExecContext(ctx, `UPDATE enrollment_processing_jobs SET attempt_id=?,attempts_started=attempts_started+1,expected_revision=?,expected_epoch=?,expected_photo_hash=?,state='QUEUED',lease_until=NULL,last_error_code=NULL WHERE submission_id=?`, ids.New(), row.revision+1, row.epoch, row.photoHash, row.id); err != nil {
		return Result{}, err
	}
	if err = s.record(ctx, c, r, "processing.retry_requested", "face_enrollment_submission", row.id, ""); err != nil {
		return Result{}, err
	}
	if err = s.record(ctx, c, r, "processing.queued", "face_enrollment_submission", row.id, ""); err != nil {
		return Result{}, err
	}
	v, err := s.view(ctx, c, p, row.id, true)
	return receipt(row.id, row.revision+1, v, 202), err
}

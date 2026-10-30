package enrollment

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollmentsecure"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/ids"
)

func (s *Service) submit(ctx context.Context, r Request) (Result, error) {
	if len(r.JPEG) > enrollmentsecure.MaxImageBytes {
		return Result{}, failure("IMAGE_TOO_LARGE", 413)
	}
	if !uuid(r.ConsentID) {
		return Result{}, failure("VALIDATION_FAILED", 400)
	}
	var replay Result
	var replayed bool
	// Authorize, recheck eligibility, replay and charge durable quotas before expensive decoding.
	err := s.transaction(ctx, func(c *sql.Conn) error {
		if err := s.authorize(ctx, c, r); err != nil {
			return err
		}
		if err := s.maintain(ctx, c); err != nil {
			return err
		}
		p, err := s.person(ctx, c, r.ActorID, true, true)
		if err != nil {
			return err
		}
		replay, replayed, err = s.replay(ctx, c, r, p)
		if err != nil || replayed {
			return err
		}
		if err = s.validConsent(ctx, c, p, r.ConsentID); err != nil {
			return err
		}
		return s.rate(ctx, c, r)
	})
	if err != nil || replayed {
		return replay, safe(err)
	}
	if s.keys == nil || s.files == nil {
		return Result{}, failure("BIOMETRIC_UNAVAILABLE", 503)
	}
	select {
	case s.decode <- struct{}{}:
		defer func() { <-s.decode }()
	default:
		return Result{}, &Error{Code: "RATE_LIMITED", Status: 429, RetryAfter: 1}
	}
	canonical, width, height, err := enrollmentsecure.NormalizeJPEG(r.JPEG)
	if err != nil {
		return Result{}, failure("IMAGE_DECODE_FAILED", 422)
	}
	defer wipe(canonical)
	id := ids.New()
	var p person
	hash := sha256.Sum256(canonical)
	canonicalHash := hex.EncodeToString(hash[:])
	err = s.transaction(ctx, func(c *sql.Conn) error {
		if err := s.authorize(ctx, c, r); err != nil {
			return err
		}
		var err error
		p, err = s.person(ctx, c, r.ActorID, true, true)
		if err != nil {
			return err
		}
		return s.validConsent(ctx, c, p, r.ConsentID)
	})
	if err != nil {
		return Result{}, safe(err)
	}
	envelope, err := enrollmentsecure.Seal(ctx, s.keys, enrollmentsecure.Binding{Purpose: "pending-photo", RecordID: id, PersonID: p.id, UserID: p.user, ConsentID: r.ConsentID}, canonical)
	if err != nil {
		return Result{}, failure("BIOMETRIC_UNAVAILABLE", 503)
	}
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	if err = s.files.Stage(id, envelope); err != nil {
		return Result{}, failure("BIOMETRIC_UNAVAILABLE", 503)
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.files.Delete(id)
		}
	}()
	var result Result
	err = s.transaction(ctx, func(c *sql.Conn) error {
		if err := s.authorize(ctx, c, r); err != nil {
			return err
		}
		if err := s.maintain(ctx, c); err != nil {
			return err
		}
		current, err := s.person(ctx, c, r.ActorID, true, true)
		if err != nil {
			return err
		}
		if current.id != p.id || current.epoch != p.epoch {
			return failure("SUBJECT_INELIGIBLE", 409)
		}
		p = current
		old, ok, err := s.replay(ctx, c, r, p)
		if err != nil {
			return err
		}
		if ok {
			result = old
			return nil
		}
		if err = s.validConsent(ctx, c, p, r.ConsentID); err != nil {
			return err
		}
		var count int
		if err = c.QueryRowContext(ctx, `SELECT COUNT(*) FROM face_enrollment_submissions WHERE (person_id=? OR subject_user_id=?) AND status IN ('PENDING_APPROVAL','APPROVED_PROCESSING','FAILED_PROCESSING','ACTIVE')`, p.id, p.user).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			return failure("ENROLLMENT_EXISTS", 409)
		}
		if err = c.QueryRowContext(ctx, `SELECT COUNT(*) FROM face_enrollment_submissions WHERE consent_id=?`, r.ConsentID).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			return failure("CONSENT_RENEWAL_REQUIRED", 409)
		}
		if err = c.QueryRowContext(ctx, `SELECT COUNT(*) FROM biometric_deletion_jobs WHERE person_id=? AND state<>'COMPLETE'`, p.id).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			return failure("DELETION_INCOMPLETE", 409)
		}
		var size int64
		if err = c.QueryRowContext(ctx, `SELECT COALESCE(SUM(byte_length+16),0) FROM face_enrollment_photos`).Scan(&size); err != nil {
			return err
		}
		if size+int64(len(envelope.Ciphertext)) > 100<<20 {
			return &Error{Code: "STORAGE_QUOTA_EXCEEDED", Status: 429, RetryAfter: 60}
		}
		var sessionEpoch int64
		if err = c.QueryRowContext(ctx, `SELECT enrollment_epoch FROM session_tokens WHERE id=?`, r.SessionID).Scan(&sessionEpoch); err != nil {
			return err
		}
		now := nowText(s.clk.Now())
		expiry := nowText(s.clk.Now().Add(PhotoTTL))
		if _, err = c.ExecContext(ctx, `INSERT INTO face_enrollment_submissions(id,person_id,subject_user_id,subject_session_id,subject_session_epoch,consent_id,status,submitted_at,photo_expires_at,canonical_sha256,captured_epoch,updated_at) VALUES(?,?,?,?,?,?,'PENDING_APPROVAL',?,?,?,?,?)`, id, p.id, p.user, r.SessionID, sessionEpoch, r.ConsentID, now, expiry, canonicalHash, p.epoch, now); err != nil {
			return err
		}
		if _, err = c.ExecContext(ctx, `INSERT INTO face_enrollment_photos(submission_id,key_version,nonce,storage_state,width,height,byte_length,canonical_sha256,expires_at) VALUES(?,?,?,'STAGED',?,?,?,?,?)`, id, envelope.KeyVersion, envelope.Nonce, width, height, len(canonical), canonicalHash, expiry); err != nil {
			return err
		}
		if err = s.record(ctx, c, r, "enrollment.submitted", "face_enrollment_submission", id, ""); err != nil {
			return err
		}
		v, err := s.view(ctx, c, p, id, false)
		if err != nil {
			return err
		}
		result = receipt(id, 1, v, 201)
		return s.saveReceipt(ctx, c, r, result)
	})
	if err != nil {
		return Result{}, safe(err)
	}
	if result.Replay {
		return result, nil
	} // The newly staged file is unreferenced and removed by the deferred cleanup.
	committed = true
	if err = s.files.Commit(id); err != nil {
		return Result{}, failure("BIOMETRIC_UNAVAILABLE", 503)
	}
	if err = s.transaction(ctx, func(c *sql.Conn) error {
		_, err := c.ExecContext(ctx, `UPDATE face_enrollment_photos SET storage_state='READY' WHERE submission_id=? AND storage_state='STAGED'`, id)
		return err
	}); err != nil {
		return Result{}, safe(err)
	}
	return result, nil
}

func (s *Service) photoGuard(ctx context.Context, c *sql.Conn, r Request) (submission, person, error) {
	row, err := s.submission(ctx, c, r.TargetID)
	if err != nil {
		return row, person{}, err
	}
	p, err := s.person(ctx, c, row.personID, false, false)
	if err != nil {
		return row, p, err
	}
	var count int
	err = c.QueryRowContext(ctx, `SELECT COUNT(*) FROM face_enrollment_photos WHERE submission_id=? AND storage_state='READY' AND expires_at>?`, row.id, nowText(s.clk.Now())).Scan(&count)
	if err != nil {
		return row, p, err
	}
	valid, err := s.subjectValid(ctx, c, row)
	if err != nil {
		return row, p, err
	}
	if count != 1 || !valid || row.status == "REJECTED" || row.status == "REVOKED" || row.status == "EXPIRED" || row.status == "ACTIVE" {
		return row, p, failure("PHOTO_UNAVAILABLE", 410)
	}
	return row, p, nil
}
func (s *Service) readPhoto(ctx context.Context, row submission) ([]byte, error) {
	if s.keys == nil || s.files == nil {
		return nil, failure("BIOMETRIC_UNAVAILABLE", 503)
	}
	envelope, err := s.files.Read(row.id)
	if err != nil {
		return nil, failure("BIOMETRIC_UNAVAILABLE", 503)
	}
	var key, canonicalHash string
	var nonce []byte
	var length int
	err = s.db.QueryRowContext(ctx, `SELECT key_version,nonce,byte_length,canonical_sha256 FROM face_enrollment_photos WHERE submission_id=?`, row.id).Scan(&key, &nonce, &length, &canonicalHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, failure("PHOTO_UNAVAILABLE", 410)
	}
	if err != nil {
		return nil, err
	}
	if key != envelope.KeyVersion || !bytes.Equal(nonce, envelope.Nonce) {
		return nil, failure("BIOMETRIC_UNAVAILABLE", 503)
	}
	plain, err := enrollmentsecure.Open(ctx, s.keys, enrollmentsecure.Binding{Purpose: "pending-photo", RecordID: row.id, PersonID: row.personID, UserID: row.userID, ConsentID: row.consentID}, envelope)
	if err != nil {
		return nil, failure("BIOMETRIC_UNAVAILABLE", 503)
	}
	hash := sha256.Sum256(plain)
	if len(plain) != length || canonicalHash != row.photoHash || hex.EncodeToString(hash[:]) != row.photoHash {
		wipe(plain)
		return nil, failure("BIOMETRIC_UNAVAILABLE", 503)
	}
	return plain, nil
}
func (s *Service) preview(ctx context.Context, r Request) (Result, error) {
	var row submission
	err := s.transaction(ctx, func(c *sql.Conn) error {
		if err := s.authorize(ctx, c, r); err != nil {
			return err
		}
		if err := s.maintain(ctx, c); err != nil {
			return err
		}
		if err := s.rate(ctx, c, r); err != nil {
			return err
		}
		var err error
		row, _, err = s.photoGuard(ctx, c, r)
		return err
	})
	if err != nil {
		return Result{}, safe(err)
	}
	s.ioMu.Lock()
	plain, err := s.readPhoto(ctx, row)
	s.ioMu.Unlock()
	if err != nil {
		return Result{}, safe(err)
	}
	err = s.transaction(ctx, func(c *sql.Conn) error {
		if err := s.authorize(ctx, c, r); err != nil {
			return err
		}
		if err := s.maintain(ctx, c); err != nil {
			return err
		}
		current, _, err := s.photoGuard(ctx, c, r)
		if err != nil {
			return err
		}
		if current.revision != row.revision || current.epoch != row.epoch {
			return failure("PHOTO_UNAVAILABLE", 410)
		}
		return s.record(ctx, c, r, "photo.preview_authorized", "face_enrollment_submission", row.id, "")
	})
	if err != nil {
		wipe(plain)
		return Result{}, safe(err)
	}
	return Result{HTTPStatus: 200, Photo: plain}, nil
}

func (s *Service) verifyApproval(ctx context.Context, r Request) (Result, error) {
	var row submission
	var replay Result
	var replayed bool
	err := s.transaction(ctx, func(c *sql.Conn) error {
		if err := s.authorize(ctx, c, r); err != nil {
			return err
		}
		if err := s.maintain(ctx, c); err != nil {
			return err
		}
		var err error
		row, err = s.submission(ctx, c, r.TargetID)
		if err != nil {
			return err
		}
		// Read an already committed receipt before requiring current eligibility.
		// A later unlink/suspension must not hide a decision that already happened.
		replay, replayed, err = s.replay(ctx, c, r, person{})
		if err != nil || replayed {
			return err
		}
		if _, err = s.person(ctx, c, row.personID, false, true); err != nil {
			return err
		}
		if r.IfMatch == "" {
			return failure("PRECONDITION_REQUIRED", 428)
		}
		if r.IfMatch != etag(row.revision) {
			return failure("PRECONDITION_FAILED", 412)
		}
		if r.Action == Approve && row.status != "PENDING_APPROVAL" {
			return failure("INVALID_TRANSITION", 409)
		}
		if r.Action == Retry && (row.status != "FAILED_PROCESSING" || row.processing != "BIOMETRIC_UNAVAILABLE") {
			return failure("RETRY_NOT_ALLOWED", 409)
		}
		if err = s.rate(ctx, c, r); err != nil {
			return err
		}
		row, _, err = s.photoGuard(ctx, c, r)
		return err
	})
	if err != nil || replayed {
		return replay, safe(err)
	}
	s.ioMu.Lock()
	plain, err := s.readPhoto(ctx, row)
	s.ioMu.Unlock()
	if err != nil {
		return Result{}, safe(err)
	}
	wipe(plain)
	r.verifiedPhotoHash = row.photoHash
	r.rateCharged = true
	return s.Execute(ctx, r)
}

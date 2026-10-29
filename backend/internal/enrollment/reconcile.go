package enrollment

import (
	"context"
	"database/sql"
	"time"
)

// Reconcile repairs committed staging, fences abandoned jobs and removes only
// unreferenced ciphertext. It never decrypts or interprets key loss as permission to delete.
// Call at synthetic harness startup before accepting requests and periodically thereafter.
func (s *Service) Reconcile(ctx context.Context) error {
	if s == nil {
		return failure("BIOMETRIC_UNAVAILABLE", 503)
	}
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	err := s.transaction(ctx, func(c *sql.Conn) error {
		if err := s.maintain(ctx, c); err != nil {
			return err
		}
		rows, err := c.QueryContext(ctx, `SELECT s.id FROM enrollment_processing_jobs j JOIN face_enrollment_submissions s ON s.id=j.submission_id WHERE j.state='RUNNING' AND (? OR j.lease_until<=?)`, len(s.worker) == 0, nowText(s.clk.Now()))
		if err != nil {
			return err
		}
		var ids []string
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, id := range ids {
			row, err := s.submission(ctx, c, id)
			if err != nil {
				return err
			}
			if err = s.failProcessing(ctx, c, row, "PROCESSING_INTERRUPTED"); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return safe(err)
	}
	if s.files == nil {
		return failure("BIOMETRIC_UNAVAILABLE", 503)
	}
	type ref struct{ id, state string }
	var refs []ref
	rows, err := s.db.QueryContext(ctx, `SELECT submission_id,storage_state FROM face_enrollment_photos`)
	if err != nil {
		return safe(err)
	}
	for rows.Next() {
		v := ref{}
		if err = rows.Scan(&v.id, &v.state); err != nil {
			rows.Close()
			return safe(err)
		}
		refs = append(refs, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return safe(err)
	}
	var fileErr error
	for _, v := range refs {
		if v.state != "STAGED" {
			continue
		}
		// Crash after DB commit: finish Stage->Commit. Crash after rename: Read
		// succeeds and the same immutable reference becomes READY without overwrite.
		if _, err = s.files.Read(v.id); err != nil {
			if err = s.files.Commit(v.id); err != nil {
				fileErr = err
				continue
			}
		}
		if err = s.transaction(ctx, func(c *sql.Conn) error {
			result, err := c.ExecContext(ctx, `UPDATE face_enrollment_photos SET storage_state='READY' WHERE submission_id=? AND storage_state='STAGED'`, v.id)
			if err != nil {
				return err
			}
			changed, _ := result.RowsAffected()
			if changed == 0 {
				return nil
			}
			return s.record(ctx, c, Request{}, "photo.reconciled", "face_enrollment_submission", v.id, "")
		}); err != nil {
			return safe(err)
		}
	}
	all, err := s.files.IDs()
	if err != nil {
		return s.cleanupFailed(ctx, "FILE_CLEANUP_FAILED")
	}
	remaining := make(map[string]bool, len(all))
	for _, id := range all {
		remaining[id] = true
		if err = s.transaction(ctx, func(c *sql.Conn) error {
			var count int
			if err := c.QueryRowContext(ctx, `SELECT COUNT(*) FROM face_enrollment_photos WHERE submission_id=?`, id).Scan(&count); err != nil {
				return err
			}
			if count != 0 {
				return nil
			}
			if err := s.files.Delete(id); err != nil {
				return err
			}
			delete(remaining, id)
			return nil
		}); err != nil {
			fileErr = err
		}
	}
	if fileErr != nil {
		if err = s.cleanupFailed(ctx, "FILE_CLEANUP_FAILED"); err != nil {
			return err
		}
		return failure("BIOMETRIC_UNAVAILABLE", 503)
	}
	if err = s.transaction(ctx, func(c *sql.Conn) error {
		rows, err := c.QueryContext(ctx, `SELECT submission_id FROM enrollment_file_deletions`)
		if err != nil {
			return err
		}
		var absent []string
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			if !remaining[id] {
				absent = append(absent, id)
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, id := range absent {
			if _, err = c.ExecContext(ctx, `DELETE FROM enrollment_file_deletions WHERE submission_id=? AND NOT EXISTS(SELECT 1 FROM face_enrollment_photos WHERE submission_id=?)`, id, id); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return safe(err)
	}
	type cleanupJob struct {
		id       string
		revision int64
	}
	var jobs []cleanupJob
	rows, err = s.db.QueryContext(ctx, `SELECT person_id,revision FROM biometric_deletion_jobs WHERE state IN ('PENDING','FAILED')`)
	if err != nil {
		return safe(err)
	}
	for rows.Next() {
		v := cleanupJob{}
		if err = rows.Scan(&v.id, &v.revision); err != nil {
			rows.Close()
			return safe(err)
		}
		jobs = append(jobs, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return safe(err)
	}
	if len(jobs) == 0 {
		return nil
	}
	if err = s.db.Checkpoint(ctx); err != nil {
		return s.cleanupFailed(ctx, "MAINTENANCE_FAILED")
	}
	if _, err = s.db.ExecContext(ctx, `VACUUM`); err != nil {
		return s.cleanupFailed(ctx, "MAINTENANCE_FAILED")
	}
	if err = s.db.Checkpoint(ctx); err != nil {
		return s.cleanupFailed(ctx, "MAINTENANCE_FAILED")
	}
	return safe(s.transaction(ctx, func(c *sql.Conn) error {
		for _, job := range jobs {
			var outstanding int
			if err := c.QueryRowContext(ctx, `SELECT COUNT(*) FROM enrollment_file_deletions WHERE person_id=?`, job.id).Scan(&outstanding); err != nil {
				return err
			}
			if outstanding != 0 {
				continue
			}
			result, err := c.ExecContext(ctx, `UPDATE biometric_deletion_jobs SET state='COMPLETE',completed_at=?,last_error_code=NULL WHERE person_id=? AND revision=? AND state IN ('PENDING','FAILED')`, nowText(s.clk.Now()), job.id, job.revision)
			if err != nil {
				return err
			}
			changed, _ := result.RowsAffected()
			if changed > 0 {
				if err = s.record(ctx, c, Request{}, "deletion.completed", "person", job.id, "CONTROLLED_LOCAL_STORAGE"); err != nil {
					return err
				}
			}
		}
		return nil
	}))
}
func (s *Service) cleanupFailed(ctx context.Context, code string) error {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	err := s.transaction(cleanup, func(c *sql.Conn) error {
		if _, err := c.ExecContext(cleanup, `UPDATE biometric_deletion_jobs SET state='FAILED',last_error_code=? WHERE state IN ('PENDING','FAILED')`, code); err != nil {
			return err
		}
		return s.record(cleanup, c, Request{}, "deletion.failed", "enrollment_cleanup", "", code)
	})
	if err != nil {
		return safe(err)
	}
	return failure("BIOMETRIC_UNAVAILABLE", 503)
}

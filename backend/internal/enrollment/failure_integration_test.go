package enrollment

// FL-ENR05 disposable SQLite/filesystem failure integration. The processor is
// deterministic dependency injection; none of these cases proves model detection.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollmentsecure"
)

func reopenEnrollmentFailureFixture(t *testing.T, f *fixture) {
	t.Helper()
	if err := f.db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := database.Open(context.Background(), filepath.Join(f.root, "synthetic.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	files, err := enrollmentsecure.NewFileStore(filepath.Join(f.root, "ciphertext"))
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(db, f.clk, f.keys, files, &syntheticProcessor{})
	if err != nil {
		t.Fatal(err)
	}
	f.db, f.files, f.s = db, files, service
}

func TestEnrollmentRestartReopensSQLiteAndReconcilesTransitions(t *testing.T) {
	for _, phase := range []string{"pending", "queued", "running", "failed", "active", "withdrawn"} {
		t.Run(phase, func(t *testing.T) {
			f := setup(t)
			ctx := context.Background()
			_, submitted := f.submit(t, f.grant(t))
			id := submitted.Data.(MutationReceipt).ResourceID
			if phase != "pending" {
				f.approve(t, id)
			}
			switch phase {
			case "running":
				// Inject the persisted boundary left by a dead worker. No live
				// processor is claimed to have crashed in this domain test.
				if _, err := f.db.ExecContext(ctx, `UPDATE enrollment_processing_jobs SET state='RUNNING',lease_until=? WHERE submission_id=?`, nowText(f.clk.Now().Add(30*time.Second)), id); err != nil {
					t.Fatal(err)
				}
			case "failed":
				f.keys.setUnavailable(true)
				if err := f.s.ProcessNext(ctx); err != nil {
					t.Fatal(err)
				}
				f.keys.setUnavailable(false)
			case "active":
				if err := f.s.ProcessNext(ctx); err != nil {
					t.Fatal(err)
				}
			case "withdrawn":
				withdraw := f.request(Withdraw)
				withdraw.Confirmed, withdraw.Scope = true, "ALL_FACE_ENROLLMENT"
				run(t, f, withdraw)
			}
			var originalDeadline string
			if err := f.db.QueryRowContext(ctx, `SELECT photo_expires_at FROM face_enrollment_submissions WHERE id=?`, id).Scan(&originalDeadline); err != nil {
				t.Fatal(err)
			}
			reopenEnrollmentFailureFixture(t, f)
			if err := f.s.Reconcile(ctx); err != nil {
				t.Fatal(err)
			}
			status := run(t, f, f.request(Status)).Data.(EnrollmentView)
			want := map[string]string{"pending": "PENDING_APPROVAL", "queued": "APPROVED_PROCESSING", "running": "FAILED_PROCESSING", "failed": "FAILED_PROCESSING", "active": "ACTIVE", "withdrawn": "REVOKED"}[phase]
			if status.SubmissionStatus != want || status.TemplateReady != (phase == "active") {
				t.Fatalf("reopened %s status=%+v", phase, status)
			}
			if phase == "running" && status.ReasonCode != "PROCESSING_INTERRUPTED" {
				t.Fatal("abandoned worker did not become interrupted")
			}
			if phase == "failed" && status.ReasonCode != "BIOMETRIC_UNAVAILABLE" {
				t.Fatal("restart changed recoverable failure classification")
			}
			var reopenedDeadline string
			if err := f.db.QueryRowContext(ctx, `SELECT photo_expires_at FROM face_enrollment_submissions WHERE id=?`, id).Scan(&reopenedDeadline); err != nil {
				t.Fatal(err)
			}
			if originalDeadline != reopenedDeadline {
				t.Fatal("restart extended immutable retention deadline")
			}
			if phase == "queued" {
				if err := f.s.ProcessNext(ctx); err != nil {
					t.Fatal(err)
				}
				status = run(t, f, f.request(Status)).Data.(EnrollmentView)
				if status.SubmissionStatus != "ACTIVE" || !status.TemplateReady {
					t.Fatal("durable queued work was not resumed")
				}
			}
			wantTemplates := 0
			if phase == "active" || phase == "queued" {
				wantTemplates = 1
			}
			if count(t, f, `SELECT COUNT(*) FROM face_templates`) != wantTemplates || count(t, f, `SELECT COUNT(*) FROM audit_events WHERE action='enrollment.activated'`) != wantTemplates {
				t.Fatal("restart duplicated or fabricated activation")
			}
			if phase == "running" || phase == "withdrawn" {
				files, err := f.files.IDs()
				if err != nil || len(files) != 0 || count(t, f, `SELECT COUNT(*) FROM face_enrollment_photos`) != 0 {
					t.Fatal("restart failed to reconcile purged payload")
				}
			}
		})
	}
}

func TestEnrollmentCleanupFailurePersistsAndReconcilesAfterRestart(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	_, submitted := f.submit(t, f.grant(t))
	id := submitted.Data.(MutationReceipt).ResourceID
	before, err := f.files.Read(id)
	if err != nil {
		t.Fatal(err)
	}
	root, displaced := filepath.Join(f.root, "ciphertext"), filepath.Join(f.root, "temporarily-offline-ciphertext")
	// Portable injected storage outage: move the owned directory, preserving its
	// identity and ciphertext. No permission relaxation, symlinks, or key loss.
	if err := os.Rename(root, displaced); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Rename(displaced, root) })
	withdraw := f.request(Withdraw)
	withdraw.Confirmed, withdraw.Scope = true, "ALL_FACE_ENROLLMENT"
	result := run(t, f, withdraw)
	if result.HTTPStatus != 200 {
		t.Fatal("logical withdrawal depended on physical cleanup")
	}
	if err := f.s.Reconcile(ctx); err == nil {
		t.Fatal("offline file store reported cleanup success")
	}
	status := run(t, f, f.request(Status)).Data.(EnrollmentView)
	if status.SubmissionStatus != "REVOKED" || status.TemplateReady || status.DeletionState != "FAILED" {
		t.Fatalf("cleanup outage status=%+v", status)
	}
	if count(t, f, `SELECT COUNT(*) FROM face_enrollment_photos`) != 0 || count(t, f, `SELECT COUNT(*) FROM face_templates`) != 0 || count(t, f, `SELECT COUNT(*) FROM enrollment_file_deletions`) != 1 || count(t, f, `SELECT COUNT(*) FROM audit_events WHERE action='deletion.failed'`) != 1 || count(t, f, `SELECT COUNT(*) FROM audit_events WHERE action='deletion.completed'`) != 0 {
		t.Fatal("cleanup failure lost logical denial, retry marker, or honest audit")
	}
	if err := os.Rename(displaced, root); err != nil {
		t.Fatal(err)
	}
	after, err := f.files.Read(id)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before.Ciphertext, after.Ciphertext) || before.AAD != after.AAD || !bytes.Equal(before.Nonce, after.Nonce) {
		t.Fatal("failed cleanup modified retained ciphertext")
	}
	reopenEnrollmentFailureFixture(t, f)
	if err := f.s.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	status = run(t, f, f.request(Status)).Data.(EnrollmentView)
	if status.SubmissionStatus != "REVOKED" || status.TemplateReady || status.DeletionState != "COMPLETE" {
		t.Fatalf("recovered cleanup status=%+v", status)
	}
	files, err := f.files.IDs()
	if err != nil || len(files) != 0 || count(t, f, `SELECT COUNT(*) FROM enrollment_file_deletions`) != 0 || count(t, f, `SELECT COUNT(*) FROM audit_events WHERE action='deletion.completed'`) != 1 {
		t.Fatal("cleanup recovery retained payload or missed completion audit")
	}
	if err := f.s.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if count(t, f, `SELECT COUNT(*) FROM audit_events WHERE action='deletion.completed'`) != 1 {
		t.Fatal("cleanup recovery replay duplicated completion")
	}
}

func TestEnrollmentCorruptPendingEnvelopeNeverReachesProcessorOrAutoDeletes(t *testing.T) {
	for _, defect := range []string{"ciphertext", "aad"} {
		t.Run(defect, func(t *testing.T) {
			f := setup(t)
			ctx := context.Background()
			calls := 0
			f.s.processor = &syntheticProcessor{fn: func(context.Context, []byte) (ProcessingResult, error) {
				calls++
				t.Error("corrupt pending artifact reached inference")
				return ProcessingResult{}, &ProcessingError{Code: "INFERENCE_FAILED"}
			}}
			_, submitted := f.submit(t, f.grant(t))
			id := submitted.Data.(MutationReceipt).ResourceID
			f.approve(t, id)
			envelope, err := f.files.Read(id)
			if err != nil {
				t.Fatal(err)
			}
			if defect == "ciphertext" {
				envelope.Ciphertext[0] ^= 1
			} else {
				envelope.AAD += "untrusted-mismatch"
			}
			// Inject corruption in the real stored envelope, leaving the trusted
			// immutable submission/photo bindings untouched.
			if err := f.files.Delete(id); err != nil {
				t.Fatal(err)
			}
			if err := f.files.Stage(id, envelope); err != nil {
				t.Fatal(err)
			}
			if err := f.files.Commit(id); err != nil {
				t.Fatal(err)
			}
			if err := f.s.ProcessNext(ctx); err != nil {
				t.Fatal(err)
			}
			status := run(t, f, f.request(Status)).Data.(EnrollmentView)
			if status.SubmissionStatus != "FAILED_PROCESSING" || status.ReasonCode != "BIOMETRIC_UNAVAILABLE" || status.TemplateReady || calls != 0 {
				t.Fatalf("corrupted source failure=%+v processor calls=%d", status, calls)
			}
			if err := f.s.Reconcile(ctx); err != nil {
				t.Fatal(err)
			}
			retained, err := f.files.Read(id)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(envelope.Ciphertext, retained.Ciphertext) || envelope.AAD != retained.AAD || count(t, f, `SELECT COUNT(*) FROM face_enrollment_photos`) != 1 || count(t, f, `SELECT COUNT(*) FROM face_templates`) != 0 || count(t, f, `SELECT COUNT(*) FROM audit_events WHERE action='enrollment.activated'`) != 0 {
				t.Fatal("integrity failure activated or automatically disposed quarantined source")
			}
			withdraw := f.request(Withdraw)
			withdraw.Confirmed, withdraw.Scope = true, "ALL_FACE_ENROLLMENT"
			run(t, f, withdraw)
			if err := f.s.Reconcile(ctx); err != nil {
				t.Fatal(err)
			}
			files, err := f.files.IDs()
			if err != nil || len(files) != 0 || count(t, f, `SELECT COUNT(*) FROM face_enrollment_photos`) != 0 {
				t.Fatal("explicit withdrawal could not dispose corrupted ciphertext")
			}
		})
	}
}

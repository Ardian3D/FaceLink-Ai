package enrollment

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/ids"
)

func TestStructuredProcessorFailuresRemainClosedAndAudited(t *testing.T) {
	for _, failureCode := range []string{"NO_FACE", "MULTIPLE_FACES", "LOW_QUALITY", "FACE_TOO_SMALL", "INVALID_IMAGE", "MODEL_UNAVAILABLE", "INFERENCE_FAILED", "PRIVATE_UNKNOWN_FAILURE"} {
		t.Run(failureCode, func(t *testing.T) {
			f := setup(t)
			f.s.processor = &syntheticProcessor{fn: func(context.Context, []byte) (ProcessingResult, error) {
				return ProcessingResult{}, &ProcessingError{Code: failureCode}
			}}
			_, submitted := f.submit(t, f.grant(t))
			id := submitted.Data.(MutationReceipt).ResourceID
			f.approve(t, id)
			if err := f.s.ProcessNext(context.Background()); err != nil {
				t.Fatal(err)
			}
			want := failureCode
			if failureCode == "PRIVATE_UNKNOWN_FAILURE" {
				want = "EXTRACTION_FAILED"
			}
			var status, reason, state, jobReason string
			if err := f.db.QueryRowContext(context.Background(), `SELECT status,processing_error_code FROM face_enrollment_submissions WHERE id=?`, id).Scan(&status, &reason); err != nil {
				t.Fatal(err)
			}
			if err := f.db.QueryRowContext(context.Background(), `SELECT state,last_error_code FROM enrollment_processing_jobs WHERE submission_id=?`, id).Scan(&state, &jobReason); err != nil {
				t.Fatal(err)
			}
			if status != "FAILED_PROCESSING" || reason != want || state != "FINISHED" || jobReason != want {
				t.Fatalf("failure state=%s reason=%s job=%s reason=%s", status, reason, state, jobReason)
			}
			if count(t, f, `SELECT COUNT(*) FROM face_templates`) != 0 || count(t, f, `SELECT COUNT(*) FROM face_enrollments`) != 0 || count(t, f, `SELECT COUNT(*) FROM audit_events WHERE action='processing.failed'`) != 1 || count(t, f, `SELECT COUNT(*) FROM face_enrollment_photos`) != 0 {
				t.Fatal("failed inference activated, retained plaintext source, or missed audit")
			}
		})
	}
	if processingCode(errors.New("private subprocess stderr")) != "EXTRACTION_FAILED" {
		t.Fatal("unstructured errors escaped closed allowlist")
	}
}

func TestInferenceLeaseRevocationAndImmutableBindings(t *testing.T) {
	for _, mutation := range []string{"expired-lease", "lease-replaced", "attempt", "job-revision", "job-epoch", "artifact-hash", "reviewer-revoked-restored", "subject-revoked-restored"} {
		t.Run(mutation, func(t *testing.T) {
			f := setup(t)
			entered, release := make(chan struct{}), make(chan struct{})
			f.s.processor = &syntheticProcessor{fn: func(context.Context, []byte) (ProcessingResult, error) {
				close(entered)
				<-release
				vector := make([]float32, 128)
				vector[0] = 1
				return ProcessingResult{Embedding: vector, FaceCount: 1, Manifest: syntheticManifest()}, nil
			}}
			_, submitted := f.submit(t, f.grant(t))
			id := submitted.Data.(MutationReceipt).ResourceID
			f.approve(t, id)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- f.s.ProcessNext(ctx) }()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("processor did not start")
			}
			var err error
			switch mutation {
			case "expired-lease":
				f.clk.Advance(30 * time.Second)
			case "lease-replaced":
				_, err = f.db.ExecContext(ctx, `UPDATE enrollment_processing_jobs SET lease_until=? WHERE submission_id=?`, nowText(f.clk.Now().Add(time.Minute)), id)
			case "attempt":
				_, err = f.db.ExecContext(ctx, `UPDATE enrollment_processing_jobs SET attempt_id=? WHERE submission_id=?`, ids.New(), id)
			case "job-revision":
				_, err = f.db.ExecContext(ctx, `UPDATE enrollment_processing_jobs SET expected_revision=expected_revision+1 WHERE submission_id=?`, id)
			case "job-epoch":
				_, err = f.db.ExecContext(ctx, `UPDATE enrollment_processing_jobs SET expected_epoch=expected_epoch+1 WHERE submission_id=?`, id)
			case "artifact-hash":
				_, err = f.db.ExecContext(ctx, `UPDATE enrollment_processing_jobs SET expected_photo_hash=? WHERE submission_id=?`, strings.Repeat("0", 64), id)
			case "reviewer-revoked-restored":
				_, err = f.db.ExecContext(ctx, `UPDATE users SET role='dosen' WHERE id=?`, f.admin)
				if err == nil {
					_, err = f.db.ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=?`, f.admin)
				}
			case "subject-revoked-restored":
				_, err = f.db.ExecContext(ctx, `UPDATE users SET status='suspended' WHERE id=?`, f.user)
				if err == nil {
					_, err = f.db.ExecContext(ctx, `UPDATE users SET status='active' WHERE id=?`, f.user)
				}
			}
			close(release)
			immutable := mutation == "attempt" || mutation == "job-revision" || mutation == "job-epoch" || mutation == "artifact-hash"
			if immutable && err == nil {
				t.Fatal("database allowed mutation of an immutable processing binding")
			}
			if !immutable && err != nil {
				t.Fatal(err)
			}
			select {
			case err = <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("processor did not finish")
			}
			if immutable {
				// The forbidden write never took effect; legitimate in-flight work may finish.
				if count(t, f, `SELECT COUNT(*) FROM face_templates`) != 1 || count(t, f, `SELECT COUNT(*) FROM audit_events WHERE action='enrollment.activated'`) != 1 {
					t.Fatal("rejected binding mutation damaged legitimate activation")
				}
				return
			}
			if count(t, f, `SELECT COUNT(*) FROM face_templates`) != 0 || count(t, f, `SELECT COUNT(*) FROM face_enrollments`) != 0 || count(t, f, `SELECT COUNT(*) FROM face_enrollment_submissions WHERE status='ACTIVE'`) != 0 || count(t, f, `SELECT COUNT(*) FROM audit_events WHERE action='enrollment.activated'`) != 0 {
				t.Fatal("late/revoked/stale inference crossed activation boundary")
			}
			if mutation == "expired-lease" || mutation == "reviewer-revoked-restored" || mutation == "subject-revoked-restored" {
				var status string
				if err = f.db.QueryRowContext(ctx, `SELECT status FROM face_enrollment_submissions WHERE id=?`, id).Scan(&status); err != nil {
					t.Fatal(err)
				}
				if status == "APPROVED_PROCESSING" {
					t.Fatal("expired or revoked processing was left runnable")
				}
			}
		})
	}
}

// These are dependency-fault/domain tests, not evidence of real-model detection.
func TestTemplateEncryptionFailureRetainsRecoverablePhotoWithoutActivation(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	f.s.processor = &syntheticProcessor{fn: func(context.Context, []byte) (ProcessingResult, error) {
		// Photo decryption already succeeded; fail specifically at template sealing.
		f.keys.setUnavailable(true)
		vector := make([]float32, 128)
		vector[0] = 1
		return ProcessingResult{Embedding: vector, FaceCount: 1, Manifest: syntheticManifest()}, nil
	}}
	_, submitted := f.submit(t, f.grant(t))
	id := submitted.Data.(MutationReceipt).ResourceID
	f.approve(t, id)
	before, err := f.files.Read(id)
	if err != nil {
		t.Fatal(err)
	}
	var originalExpiry string
	if err = f.db.QueryRowContext(ctx, `SELECT expires_at FROM face_enrollment_photos WHERE submission_id=?`, id).Scan(&originalExpiry); err != nil {
		t.Fatal(err)
	}
	if err = f.s.ProcessNext(ctx); err != nil {
		t.Fatal(err)
	}
	status := run(t, f, f.request(Status)).Data.(EnrollmentView)
	if status.SubmissionStatus != "FAILED_PROCESSING" || status.ReasonCode != "BIOMETRIC_UNAVAILABLE" || status.TemplateReady {
		t.Fatalf("encryption failure status=%+v", status)
	}
	var state, reason string
	if err = f.db.QueryRowContext(ctx, `SELECT state,last_error_code FROM enrollment_processing_jobs WHERE submission_id=?`, id).Scan(&state, &reason); err != nil {
		t.Fatal(err)
	}
	if state != "FINISHED" || reason != "BIOMETRIC_UNAVAILABLE" {
		t.Fatalf("encryption failure job=%s reason=%s", state, reason)
	}
	after, err := f.files.Read(id)
	if err != nil {
		t.Fatal(err)
	}
	var retainedExpiry string
	if err = f.db.QueryRowContext(ctx, `SELECT expires_at FROM face_enrollment_photos WHERE submission_id=?`, id).Scan(&retainedExpiry); err != nil {
		t.Fatal(err)
	}
	if before.Version != after.Version || before.Algorithm != after.Algorithm || before.KeyVersion != after.KeyVersion || before.AAD != after.AAD || !bytes.Equal(before.Nonce, after.Nonce) || !bytes.Equal(before.Ciphertext, after.Ciphertext) || retainedExpiry != originalExpiry {
		t.Fatal("template encryption failure changed retained envelope or extended photo expiry")
	}
	if count(t, f, `SELECT COUNT(*) FROM face_templates`) != 0 || count(t, f, `SELECT COUNT(*) FROM face_enrollments`) != 0 || count(t, f, `SELECT COUNT(*) FROM audit_events WHERE action='enrollment.activated'`) != 0 || count(t, f, `SELECT COUNT(*) FROM audit_events WHERE action='processing.failed'`) != 1 {
		t.Fatal("template encryption failure activated or missed its failure audit")
	}
	f.keys.setUnavailable(false)
	f.s.processor = &syntheticProcessor{}
	detail := f.request(Detail)
	detail.TargetID = id
	current := run(t, f, detail).Data.(EnrollmentView)
	retry := f.request(Retry)
	retry.TargetID = id
	retry.IfMatch = etag(current.Revision)
	retry.Confirmed = true
	run(t, f, retry)
	if err = f.s.ProcessNext(ctx); err != nil {
		t.Fatal(err)
	}
	recovered := run(t, f, f.request(Status)).Data.(EnrollmentView)
	if recovered.SubmissionStatus != "ACTIVE" || !recovered.TemplateReady {
		t.Fatalf("recovered encryption dependency status=%+v", recovered)
	}
}

func TestConcurrentDuplicateProcessingActivatesExactlyOnce(t *testing.T) {
	f := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, release := make(chan struct{}, 1), make(chan struct{})
	var calls atomic.Int32
	processor := &syntheticProcessor{fn: func(ctx context.Context, _ []byte) (ProcessingResult, error) {
		call := calls.Add(1)
		select {
		case entered <- struct{}{}:
		default:
		}
		select {
		case <-release:
		case <-ctx.Done():
			return ProcessingResult{}, ctx.Err()
		}
		vector := make([]float32, 128)
		vector[call-1] = 1
		return ProcessingResult{Embedding: vector, FaceCount: 1, Manifest: syntheticManifest()}, nil
	}}
	f.s.processor = processor
	other, err := New(f.db, f.clk, f.keys, f.files, processor)
	if err != nil {
		t.Fatal(err)
	}
	_, submitted := f.submit(t, f.grant(t))
	f.approve(t, submitted.Data.(MutationReceipt).ResourceID)
	second := secondSubjectSEC01(t, f)
	_, secondSubmitted := second.submit(t, second.grant(t))
	secondID := secondSubmitted.Data.(MutationReceipt).ResourceID
	second.approve(t, secondID)
	done := make(chan error, 1)
	go func() { done <- f.s.ProcessNext(ctx) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("processor did not start")
	}
	// Exercise both the instance slot and the durable RUNNING lease.
	code(t, f.s.ProcessNext(ctx), "PROCESSOR_BUSY")
	code(t, other.ProcessNext(ctx), "PROCESSOR_BUSY")
	close(release)
	select {
	case err = <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("processor did not finish")
	}
	// The queued second subject may now run; subsequent polls must not replay either.
	if err = f.s.ProcessNext(ctx); err != nil {
		t.Fatal(err)
	}
	if err = other.ProcessNext(ctx); err != nil {
		t.Fatal(err)
	}
	status := run(t, f, f.request(Status)).Data.(EnrollmentView)
	if status.SubmissionStatus != "ACTIVE" || !status.TemplateReady || calls.Load() != 2 {
		t.Fatalf("duplicate processing status=%+v calls=%d", status, calls.Load())
	}
	secondStatus := run(t, second, second.request(Status)).Data.(EnrollmentView)
	if secondStatus.SubmissionID != secondID || !secondStatus.TemplateReady {
		t.Fatalf("concurrent queue mixed subject/template records: %+v", secondStatus)
	}
	assertTemplateVectorSEC01(t, f, status.SubmissionID, 0)
	assertTemplateVectorSEC01(t, second, secondID, 1)
	if count(t, f, `SELECT COUNT(*) FROM face_templates`) != 2 || count(t, f, `SELECT COUNT(*) FROM face_enrollments`) != 2 || count(t, f, `SELECT COUNT(*) FROM audit_events WHERE action='processing.started'`) != 2 || count(t, f, `SELECT COUNT(*) FROM audit_events WHERE action='enrollment.activated'`) != 2 || count(t, f, `SELECT COUNT(*) FROM enrollment_processing_jobs WHERE state='FINISHED' AND attempts_started=1`) != 2 {
		t.Fatal("duplicate processing repeated activation, audit, or durable attempt")
	}
}

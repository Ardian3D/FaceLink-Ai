package enrollment

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/clock"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollmentfixture"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollmentsecure"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/ids"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/migrations"
)

type syntheticKeys struct {
	mu          sync.Mutex
	unavailable bool
}

func (k *syntheticKeys) Active(_ context.Context, purpose string) (string, []byte, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.unavailable {
		return "", nil, enrollmentsecure.ErrUnavailable
	}
	value := byte(17)
	if purpose == "template" {
		value = 29
	}
	return "test-1", bytes.Repeat([]byte{value}, 32), nil
}
func (k *syntheticKeys) ByVersion(ctx context.Context, purpose, version string) ([]byte, error) {
	_, b, err := k.Active(ctx, purpose)
	if version != "test-1" {
		return nil, enrollmentsecure.ErrUnavailable
	}
	return b, err
}
func (k *syntheticKeys) setUnavailable(v bool) { k.mu.Lock(); k.unavailable = v; k.mu.Unlock() }
func syntheticManifest() Manifest {
	return Manifest{SyntheticOnly: true, ModelID: "synthetic-unit-model", ModelVersion: "1", ModelChecksum: strings.Repeat("a", 64), DetectorID: "synthetic-unit-detector", DetectorVersion: "1", DetectorChecksum: strings.Repeat("b", 64), PipelineRevision: "synthetic-unit-1"}
}

type syntheticProcessor struct {
	fn func(context.Context, []byte) (ProcessingResult, error)
}

func (p *syntheticProcessor) Manifest() Manifest { return syntheticManifest() }
func (p *syntheticProcessor) Process(ctx context.Context, b []byte) (ProcessingResult, error) {
	if p.fn != nil {
		return p.fn(ctx, b)
	}
	v := make([]float32, 128)
	v[0] = 1
	return ProcessingResult{Embedding: v, FaceCount: 1, Manifest: p.Manifest()}, nil
}

type fixture struct {
	s                                                    *Service
	db                                                   *database.DB
	clk                                                  *clock.Fake
	keys                                                 *syntheticKeys
	files                                                *enrollmentsecure.FileStore
	root, user, admin, person, userSession, adminSession string
}

func setup(t *testing.T) *fixture {
	t.Helper()
	return setupAtVersion(t, 8)
}

func setupAtVersion(t *testing.T, version int64) *fixture {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	db, err := database.Open(ctx, filepath.Join(root, "synthetic.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, source, _, _ := runtime.Caller(0)
	combined, err := enrollmentfixture.PrepareMigrations(filepath.Join(filepath.Dir(source), "..", "..", "migrations"), filepath.Join(root, "migration-files"))
	if err != nil {
		t.Fatal(err)
	}
	available, err := migrations.Load(combined)
	if err != nil {
		t.Fatal(err)
	}
	for _, migration := range available {
		if migration.Version > version {
			if err = os.Remove(migration.Path); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err = migrations.Run(ctx, db, combined); err != nil {
		t.Fatal(err)
	}
	f := &fixture{db: db, clk: clock.NewFake(time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)), keys: &syntheticKeys{}, root: root, user: ids.New(), admin: ids.New(), person: ids.New(), userSession: ids.New(), adminSession: ids.New()}
	f.files, err = enrollmentsecure.NewFileStore(filepath.Join(root, "ciphertext"))
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []struct{ id, role string }{{f.user, "mahasiswa"}, {f.admin, "admin"}} {
		if _, err = db.ExecContext(ctx, `INSERT INTO users(id,email,display_name,role,status) VALUES(?,?,? ,?,'active')`, u.id, u.id+"@example.invalid", "Synthetic", u.role); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO people(id,external_code,display_name,account_user_id,identity_reference,identity_verified_at) VALUES(?,?,'Synthetic person',?,'synthetic-roster',?)`, f.person, ids.New(), f.user, nowText(f.clk.Now())); err != nil {
		t.Fatal(err)
	}
	classID := ids.New()
	if _, err = db.ExecContext(ctx, `INSERT INTO classes(id,code,title) VALUES(?,?,'Synthetic class')`, classID, ids.New()); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO class_memberships(class_id,person_id,created_by) VALUES(?,?,?)`, classID, f.person, f.admin); err != nil {
		t.Fatal(err)
	}
	for _, u := range []struct{ id, session string }{{f.user, f.userSession}, {f.admin, f.adminSession}} {
		if _, err = db.ExecContext(ctx, `INSERT INTO session_tokens(id,user_id,access_hash,refresh_hash,access_expires_at,refresh_expires_at,absolute_expires_at) VALUES(?,?,?,?,?,?,?)`, u.session, u.id, ids.New(), ids.New(), nowText(f.clk.Now().Add(7*24*time.Hour)), nowText(f.clk.Now().Add(7*24*time.Hour)), nowText(f.clk.Now().Add(7*24*time.Hour))); err != nil {
			t.Fatal(err)
		}
	}
	f.s, err = New(db, f.clk, f.keys, f.files, &syntheticProcessor{})
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func (f *fixture) request(a Action) Request {
	r := Request{Action: a, ActorID: f.user, SessionID: f.userSession, RequestID: ids.New()}
	if adminAction(a) {
		r.ActorID = f.admin
		r.SessionID = f.adminSession
	}
	if mutation(a) {
		r.IdempotencyKey = ids.New()
	}
	return r
}
func run(t *testing.T, f *fixture, r Request) Result {
	t.Helper()
	result, err := f.s.Execute(context.Background(), r)
	if err != nil {
		t.Fatalf("%s: %v", r.Action, err)
	}
	return result
}
func code(t *testing.T, err error, want string) {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) || e.Code != want {
		t.Fatalf("error=%v want=%s", err, want)
	}
}
func (f *fixture) grant(t *testing.T) string {
	t.Helper()
	notice := run(t, f, f.request(Notice))
	r := f.request(Grant)
	r.IfMatch = notice.ETag
	r.NoticeVersion = NoticeVersion
	r.Accepted = true
	result := run(t, f, r)
	return result.Data.(MutationReceipt).ResourceID
}
func syntheticJPEG(t *testing.T) []byte {
	t.Helper()
	im := image.NewRGBA(image.Rect(0, 0, 112, 112))
	for y := range 112 {
		for x := range 112 {
			im.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 73, A: 255})
		}
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, im, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func (f *fixture) submit(t *testing.T, consent string) (Request, Result) {
	t.Helper()
	r := f.request(Submit)
	r.ConsentID = consent
	r.JPEG = syntheticJPEG(t)
	return r, run(t, f, r)
}
func (f *fixture) approve(t *testing.T, id string) Result {
	t.Helper()
	detail := f.request(Detail)
	detail.TargetID = id
	v := run(t, f, detail)
	r := f.request(Approve)
	r.TargetID = id
	r.IfMatch = v.ETag
	r.Confirmed = true
	r.IdentityCheckMethod = "CAMPUS_AUTHORIZED_CHECK"
	r.AttestationVersion = "synthetic-v1"
	r.CampusCheckRef = ids.New()
	return run(t, f, r)
}
func count(t *testing.T, f *fixture, q string) int {
	t.Helper()
	var n int
	if err := f.db.QueryRowContext(context.Background(), q).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSyntheticEnrollmentLifecycleAndEncryptedPayloads(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	initial := run(t, f, f.request(Status)).Data.(EnrollmentView)
	if initial.EffectiveStatus != "NOT_ENROLLED" || initial.TemplateReady || initial.ConsentState != "NOT_GIVEN" {
		t.Fatalf("initial=%+v", initial)
	}
	consent := f.grant(t)
	request, submitted := f.submit(t, consent)
	id := submitted.Data.(MutationReceipt).ResourceID
	if count(t, f, `SELECT COUNT(*) FROM face_templates`) != 0 || count(t, f, `SELECT COUNT(*) FROM enrollment_processing_jobs`) != 0 {
		t.Fatal("submission ran extraction")
	}
	replayed := run(t, f, request)
	if !replayed.Replay || replayed.HTTPStatus != 200 || replayed.Data.(MutationReceipt).ResourceID != id {
		t.Fatal("not replayed")
	}
	preview := f.request(Photo)
	preview.TargetID = id
	photo := run(t, f, preview).Photo
	canonical, _, _, err := enrollmentsecure.NormalizeJPEG(request.JPEG)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(photo, canonical) {
		t.Fatal("preview differs from canonical reviewed bytes")
	}
	wipe(photo)
	wipe(canonical)
	approved := f.approve(t, id)
	if approved.HTTPStatus != 202 || approved.Data.(MutationReceipt).Current.(EnrollmentView).TemplateReady {
		t.Fatal("approval falsely active")
	}
	if err = f.s.ProcessNext(ctx); err != nil {
		t.Fatal(err)
	}
	active := run(t, f, f.request(Status)).Data.(EnrollmentView)
	if active.EffectiveStatus != "ACTIVE" || !active.TemplateReady || !active.SyntheticOnly {
		t.Fatalf("active=%+v", active)
	}
	if count(t, f, `SELECT COUNT(*) FROM face_templates WHERE length(ciphertext)=528 AND synthetic_only=1`) != 1 || count(t, f, `SELECT COUNT(*) FROM face_enrollment_photos`) != 0 {
		t.Fatal("atomic activation payload invariant")
	}
	if err = f.s.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	idsOnDisk, err := f.files.IDs()
	if err != nil || len(idsOnDisk) != 0 {
		t.Fatalf("cipher files=%v %v", idsOnDisk, err)
	}
	withdraw := f.request(Withdraw)
	withdraw.Scope = "ALL_FACE_ENROLLMENT"
	withdraw.Confirmed = true
	run(t, f, withdraw)
	if count(t, f, `SELECT COUNT(*) FROM face_templates`) != 0 || count(t, f, `SELECT COUNT(*) FROM face_enrollments WHERE status='active'`) != 0 {
		t.Fatal("withdrawal left usable payload")
	}
	if err = f.s.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	status := run(t, f, f.request(Status)).Data.(EnrollmentView)
	if status.EffectiveStatus != "REVOKED" || status.TemplateReady || status.DeletionState != "COMPLETE" {
		t.Fatalf("withdrawn=%+v", status)
	}
	if count(t, f, `SELECT COUNT(*) FROM audit_events WHERE action='photo.preview_authorized'`) != 1 {
		t.Fatal("preview authorization not audited")
	}
}
func TestConsentCASIdempotencyAndFreshGrant(t *testing.T) {
	f := setup(t)
	r := f.request(Grant)
	r.NoticeVersion = "obsolete"
	r.Accepted = true
	r.IfMatch = `"0"`
	_, err := f.s.Execute(context.Background(), r)
	code(t, err, "CONSENT_VERSION_MISMATCH")
	consent := f.grant(t)
	upload, submitted := f.submit(t, consent)
	id := submitted.Data.(MutationReceipt).ResourceID
	changed := upload
	changed.JPEG = append(append([]byte{}, upload.JPEG...), 0)
	_, err = f.s.Execute(context.Background(), changed)
	code(t, err, "IDEMPOTENCY_CONFLICT")
	detail := f.request(Detail)
	detail.TargetID = id
	view := run(t, f, detail)
	reject := f.request(Reject)
	reject.TargetID = id
	reject.IfMatch = view.ETag
	reject.ReasonCode = "EVIDENCE_INSUFFICIENT"
	run(t, f, reject)
	if count(t, f, `SELECT COUNT(*) FROM face_enrollment_photos`) != 0 {
		t.Fatal("rejection retained photo")
	}
	if err = f.s.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	fresh := f.request(Grant)
	fresh.NoticeVersion = NoticeVersion
	fresh.Accepted = true
	fresh.ReplacesConsentID = consent
	fresh.IfMatch = `"0"`
	_, err = f.s.Execute(context.Background(), fresh)
	code(t, err, "PRECONDITION_FAILED")
	notice := run(t, f, f.request(Notice))
	fresh.IfMatch = notice.ETag
	f.clk.Advance(time.Millisecond)
	newConsent := run(t, f, fresh).Data.(MutationReceipt).ResourceID
	if newConsent == consent {
		t.Fatal("grant reused")
	}
	_, newSubmission := f.submit(t, newConsent)
	if newSubmission.Data.(MutationReceipt).ResourceID == id {
		t.Fatal("submission reused")
	}
}
func TestDecisionRaceIsSingleImmutableCAS(t *testing.T) {
	f := setup(t)
	_, submitted := f.submit(t, f.grant(t))
	id := submitted.Data.(MutationReceipt).ResourceID
	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		r := f.request(Reject)
		r.TargetID = id
		r.IfMatch = `"1"`
		r.ReasonCode = "UNSUITABLE_SUBMISSION"
		go func(r Request) { <-start; _, err := f.s.Execute(context.Background(), r); results <- err }(r)
	}
	close(start)
	a, b := <-results, <-results
	if (a == nil) == (b == nil) {
		t.Fatalf("race=%v/%v", a, b)
	}
	if a != nil {
		code(t, a, "PRECONDITION_FAILED")
	} else {
		code(t, b, "PRECONDITION_FAILED")
	}
	if count(t, f, `SELECT COUNT(*) FROM audit_events WHERE action='enrollment.rejected'`) != 1 {
		t.Fatal("duplicate decision audit")
	}
	if _, err := f.db.ExecContext(context.Background(), `UPDATE face_enrollment_submissions SET review_decision='APPROVED',revision=revision+1 WHERE id=?`, id); err == nil {
		t.Fatal("decision mutable")
	}
}
func TestConcurrentSubmitOnlyOneLiveAttempt(t *testing.T) {
	f := setup(t)
	consent := f.grant(t)
	jpeg := syntheticJPEG(t)
	start := make(chan struct{})
	errs := make(chan error, 2)
	for range 2 {
		r := f.request(Submit)
		r.ConsentID = consent
		r.JPEG = jpeg
		go func(r Request) { <-start; _, err := f.s.Execute(context.Background(), r); errs <- err }(r)
	}
	close(start)
	a, b := <-errs, <-errs
	if (a == nil) == (b == nil) {
		t.Fatalf("race=%v/%v", a, b)
	}
	if a != nil {
		code(t, a, "ENROLLMENT_EXISTS")
	} else {
		code(t, b, "ENROLLMENT_EXISTS")
	}
	if count(t, f, `SELECT COUNT(*) FROM face_enrollment_submissions`) != 1 {
		t.Fatal("duplicate submission")
	}
	if err := f.s.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	files, err := f.files.IDs()
	if err != nil || len(files) != 1 {
		t.Fatalf("orphan files=%v %v", files, err)
	}
}
func TestAuditFailuresRollBackGrantPreviewDecisionAndActivation(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if _, err := f.db.ExecContext(ctx, `CREATE TRIGGER synthetic_audit_failure BEFORE INSERT ON audit_events WHEN NEW.action='consent.granted' BEGIN SELECT RAISE(ABORT,'synthetic'); END`); err != nil {
		t.Fatal(err)
	}
	r := f.request(Grant)
	r.Accepted = true
	r.NoticeVersion = NoticeVersion
	r.IfMatch = `"0"`
	_, err := f.s.Execute(ctx, r)
	code(t, err, "AUDIT_UNAVAILABLE")
	if count(t, f, `SELECT COUNT(*) FROM consent_records`) != 0 || count(t, f, `SELECT COUNT(*) FROM idempotency_records WHERE scope LIKE 'enrollment:%'`) != 0 {
		t.Fatal("grant partially committed")
	}
	if _, err = f.db.ExecContext(ctx, `DROP TRIGGER synthetic_audit_failure`); err != nil {
		t.Fatal(err)
	}
	_, submitted := f.submit(t, f.grant(t))
	id := submitted.Data.(MutationReceipt).ResourceID
	if _, err = f.db.ExecContext(ctx, `CREATE TRIGGER synthetic_audit_failure BEFORE INSERT ON audit_events WHEN NEW.action IN ('photo.preview_authorized','enrollment.approved') BEGIN SELECT RAISE(ABORT,'synthetic'); END`); err != nil {
		t.Fatal(err)
	}
	photo := f.request(Photo)
	photo.TargetID = id
	response, err := f.s.Execute(ctx, photo)
	code(t, err, "AUDIT_UNAVAILABLE")
	if len(response.Photo) != 0 {
		t.Fatal("preview bytes escaped failed audit")
	}
	approve := f.request(Approve)
	approve.TargetID = id
	approve.IfMatch = `"1"`
	approve.Confirmed = true
	approve.IdentityCheckMethod = "CAMPUS_AUTHORIZED_CHECK"
	approve.AttestationVersion = "synthetic-v1"
	approve.CampusCheckRef = ids.New()
	_, err = f.s.Execute(ctx, approve)
	code(t, err, "AUDIT_UNAVAILABLE")
	if count(t, f, `SELECT COUNT(*) FROM enrollment_processing_jobs`) != 0 {
		t.Fatal("queue partially committed")
	}
	if _, err = f.db.ExecContext(ctx, `DROP TRIGGER synthetic_audit_failure`); err != nil {
		t.Fatal(err)
	}
	f.approve(t, id)
	if _, err = f.db.ExecContext(ctx, `CREATE TRIGGER synthetic_audit_failure BEFORE INSERT ON audit_events WHEN NEW.action='enrollment.activated' BEGIN SELECT RAISE(ABORT,'synthetic'); END`); err != nil {
		t.Fatal(err)
	}
	err = f.s.ProcessNext(ctx)
	code(t, err, "AUDIT_UNAVAILABLE")
	if count(t, f, `SELECT COUNT(*) FROM face_templates`) != 0 || count(t, f, `SELECT COUNT(*) FROM face_enrollments`) != 0 || count(t, f, `SELECT COUNT(*) FROM face_enrollment_photos`) != 1 {
		t.Fatal("activation partially committed")
	}
}
func TestWithdrawalDuringProcessorFencesLateResult(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	entered, release := make(chan struct{}), make(chan struct{})
	f.s.processor = &syntheticProcessor{fn: func(context.Context, []byte) (ProcessingResult, error) {
		close(entered)
		<-release
		v := make([]float32, 128)
		v[0] = 1
		return ProcessingResult{Embedding: v, FaceCount: 1, Manifest: syntheticManifest()}, nil
	}}
	_, submitted := f.submit(t, f.grant(t))
	f.approve(t, submitted.Data.(MutationReceipt).ResourceID)
	done := make(chan error, 1)
	go func() { done <- f.s.ProcessNext(ctx) }()
	<-entered
	if err := f.s.ProcessNext(ctx); err == nil {
		t.Fatal("parallel inference allowed")
	}
	withdraw := f.request(Withdraw)
	withdraw.Confirmed = true
	withdraw.Scope = "ALL_FACE_ENROLLMENT"
	run(t, f, withdraw)
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if count(t, f, `SELECT COUNT(*) FROM face_templates`) != 0 || count(t, f, `SELECT COUNT(*) FROM face_enrollment_submissions WHERE status='ACTIVE'`) != 0 {
		t.Fatal("late activation survived withdrawal")
	}
}
func TestExpiryIs72HoursAndNeverExtendedByReplay(t *testing.T) {
	f := setup(t)
	upload, submitted := f.submit(t, f.grant(t))
	id := submitted.Data.(MutationReceipt).ResourceID
	f.clk.Advance(23 * time.Hour)
	run(t, f, upload)
	f.clk.Advance(49 * time.Hour)
	v := run(t, f, f.request(Status)).Data.(EnrollmentView)
	if v.SubmissionStatus != "EXPIRED" || v.PhotoAvailable || v.ConsentState != "ACTIVE" {
		t.Fatalf("expiry=%+v", v)
	}
	photo := f.request(Photo)
	photo.TargetID = id
	_, err := f.s.Execute(context.Background(), photo)
	code(t, err, "PHOTO_UNAVAILABLE")
	if count(t, f, `SELECT COUNT(*) FROM face_enrollment_photos`) != 0 {
		t.Fatal("expired photo retained logically")
	}
}
func TestReviewerDemotionRestorationPermanentlyFencesApproval(t *testing.T) {
	f := setup(t)
	_, submitted := f.submit(t, f.grant(t))
	id := submitted.Data.(MutationReceipt).ResourceID
	f.approve(t, id)
	if _, err := f.db.ExecContext(context.Background(), `UPDATE users SET role='dosen' WHERE id=?`, f.admin); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.ExecContext(context.Background(), `UPDATE users SET role='admin' WHERE id=?`, f.admin); err != nil {
		t.Fatal(err)
	}
	if err := f.s.ProcessNext(context.Background()); err != nil {
		t.Fatal(err)
	}
	v := run(t, f, f.request(Status)).Data.(EnrollmentView)
	if v.SubmissionStatus != "FAILED_PROCESSING" || v.ReasonCode != "REVIEWER_INELIGIBLE" {
		t.Fatalf("reviewer=%+v", v)
	}
	r := f.request(Retry)
	r.TargetID = id
	r.IfMatch = etag(v.Revision)
	r.Confirmed = true
	_, err := f.s.Execute(context.Background(), r)
	code(t, err, "RETRY_NOT_ALLOWED")
}
func TestAccountRosterAndLinkRestorationDoNotResurrectWork(t *testing.T) {
	for _, change := range []string{"account", "roster", "link", "person", "session"} {
		t.Run(change, func(t *testing.T) {
			f := setup(t)
			_, submitted := f.submit(t, f.grant(t))
			id := submitted.Data.(MutationReceipt).ResourceID
			f.approve(t, id)
			ctx := context.Background()
			var err error
			exec := func(q string, args ...any) {
				if err == nil {
					_, err = f.db.ExecContext(ctx, q, args...)
				}
			}
			switch change {
			case "account":
				exec(`UPDATE users SET status='suspended' WHERE id=?`, f.user)
				exec(`UPDATE users SET status='active' WHERE id=?`, f.user)
			case "roster":
				exec(`UPDATE class_memberships SET left_at=? WHERE person_id=?`, nowText(f.clk.Now()), f.person)
				exec(`UPDATE class_memberships SET left_at=NULL WHERE person_id=?`, f.person)
			case "link":
				exec(`UPDATE people SET account_user_id=NULL WHERE id=?`, f.person)
				exec(`UPDATE people SET account_user_id=? WHERE id=?`, f.user, f.person)
			case "person":
				exec(`UPDATE people SET status='inactive' WHERE id=?`, f.person)
				exec(`UPDATE people SET status='active' WHERE id=?`, f.person)
			case "session":
				exec(`UPDATE session_tokens SET revoked_at=? WHERE id=?`, nowText(f.clk.Now()), f.userSession)
				exec(`UPDATE session_tokens SET revoked_at=NULL WHERE id=?`, f.userSession)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = f.s.ProcessNext(ctx); err != nil {
				t.Fatal(err)
			}
			if count(t, f, `SELECT COUNT(*) FROM face_templates`) != 0 {
				t.Fatal("restoration resurrected old approval")
			}
			v := run(t, f, f.request(Status)).Data.(EnrollmentView)
			if v.SubmissionStatus != "REVOKED" {
				t.Fatalf("lifecycle=%+v", v)
			}
		})
	}
}
func TestOriginalAccessExpiryDoesNotInvalidateDurableReview(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if _, err := f.db.ExecContext(ctx, `UPDATE session_tokens SET access_expires_at=? WHERE id=?`, nowText(f.clk.Now().Add(time.Hour)), f.userSession); err != nil {
		t.Fatal(err)
	}
	_, submitted := f.submit(t, f.grant(t))
	f.clk.Advance(2 * time.Hour)
	id := submitted.Data.(MutationReceipt).ResourceID
	f.approve(t, id)
	if err := f.s.ProcessNext(ctx); err != nil {
		t.Fatal(err)
	}
	detail := f.request(Detail)
	detail.TargetID = id
	v := run(t, f, detail).Data.(EnrollmentView)
	if !v.TemplateReady || v.SubmissionStatus != "ACTIVE" {
		t.Fatalf("expired uploader invalidated durable approval=%+v", v)
	}
}
func TestKeyFailurePreservesCiphertextAndWithdrawalNeedsNoKey(t *testing.T) {
	f := setup(t)
	_, submitted := f.submit(t, f.grant(t))
	id := submitted.Data.(MutationReceipt).ResourceID
	f.approve(t, id)
	f.keys.setUnavailable(true)
	if err := f.s.ProcessNext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if count(t, f, `SELECT COUNT(*) FROM face_enrollment_photos`) != 1 {
		t.Fatal("key failure deleted ciphertext")
	}
	withdraw := f.request(Withdraw)
	withdraw.Confirmed = true
	withdraw.Scope = "ALL_FACE_ENROLLMENT"
	run(t, f, withdraw)
	if count(t, f, `SELECT COUNT(*) FROM face_enrollment_photos`) != 0 {
		t.Fatal("withdrawal depended on key")
	}
}
func TestExtractionErrorsNeverActivateAndConservativelyPurge(t *testing.T) {
	for _, kind := range []string{"no_face", "multiple", "nan", "zero", "norm", "manifest"} {
		t.Run(kind, func(t *testing.T) {
			f := setup(t)
			f.s.processor = &syntheticProcessor{fn: func(context.Context, []byte) (ProcessingResult, error) {
				v := make([]float32, 128)
				v[0] = 1
				r := ProcessingResult{Embedding: v, FaceCount: 1, Manifest: syntheticManifest()}
				switch kind {
				case "no_face":
					r.FaceCount = 0
				case "multiple":
					r.FaceCount = 2
				case "nan":
					v[2] = float32(math.NaN())
				case "zero":
					v[0] = 0
				case "norm":
					v[0] = 2
				case "manifest":
					r.Manifest.PipelineRevision = "wrong"
				}
				return r, nil
			}}
			_, submitted := f.submit(t, f.grant(t))
			f.approve(t, submitted.Data.(MutationReceipt).ResourceID)
			if err := f.s.ProcessNext(context.Background()); err != nil {
				t.Fatal(err)
			}
			if count(t, f, `SELECT COUNT(*) FROM face_templates`) != 0 || count(t, f, `SELECT COUNT(*) FROM face_enrollment_photos`) != 0 {
				t.Fatal("invalid extraction retained/activated")
			}
			v := run(t, f, f.request(Status)).Data.(EnrollmentView)
			if v.SubmissionStatus != "FAILED_PROCESSING" {
				t.Fatalf("failure=%+v", v)
			}
		})
	}
}
func TestEncryptedFileCrashRecoveryAndOrphanCleanup(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	upload, submitted := f.submit(t, f.grant(t))
	id := submitted.Data.(MutationReceipt).ResourceID
	// Model crash after DB commit but before Stage->Commit with a fresh fixture's ciphertext.
	envelope, err := f.files.Read(id)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.files.Delete(id); err != nil {
		t.Fatal(err)
	}
	if err = f.files.Stage(id, envelope); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.ExecContext(ctx, `DROP TRIGGER enrollment_photo_immutable;UPDATE face_enrollment_photos SET storage_state='STAGED' WHERE submission_id=?`, id); err != nil {
		t.Fatal(err)
	}
	orphan := ids.New()
	if err = f.files.Stage(orphan, envelope); err != nil {
		t.Fatal(err)
	}
	if err = f.s.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	photo := f.request(Photo)
	photo.TargetID = id
	data := run(t, f, photo).Photo
	canonical, _, _, _ := enrollmentsecure.NormalizeJPEG(upload.JPEG)
	if !bytes.Equal(data, canonical) {
		t.Fatal("staging recovery changed bytes")
	}
	wipe(data)
	wipe(canonical)
	files, err := f.files.IDs()
	if err != nil || len(files) != 1 || files[0] != id {
		t.Fatalf("recovery files=%v %v", files, err)
	}
}
func TestAbandonedRunningJobFailsOnRestart(t *testing.T) {
	f := setup(t)
	_, submitted := f.submit(t, f.grant(t))
	id := submitted.Data.(MutationReceipt).ResourceID
	f.approve(t, id)
	if _, err := f.db.ExecContext(context.Background(), `UPDATE enrollment_processing_jobs SET state='RUNNING',lease_until=? WHERE submission_id=?`, nowText(f.clk.Now().Add(30*time.Second)), id); err != nil {
		t.Fatal(err)
	}
	restarted, err := New(f.db, f.clk, f.keys, f.files, &syntheticProcessor{})
	if err != nil {
		t.Fatal(err)
	}
	f.s = restarted
	if err = f.s.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	v := run(t, f, f.request(Status)).Data.(EnrollmentView)
	if v.SubmissionStatus != "FAILED_PROCESSING" || v.ReasonCode != "PROCESSING_INTERRUPTED" {
		t.Fatalf("restart=%+v", v)
	}
}
func TestRoleFencePreservesActiveTemplateButReadinessFalse(t *testing.T) {
	f := setup(t)
	_, submitted := f.submit(t, f.grant(t))
	id := submitted.Data.(MutationReceipt).ResourceID
	f.approve(t, id)
	if err := f.s.ProcessNext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.ExecContext(context.Background(), `UPDATE users SET role='dosen' WHERE id=?`, f.user); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.ExecContext(context.Background(), `UPDATE users SET role='mahasiswa' WHERE id=?`, f.user); err != nil {
		t.Fatal(err)
	}
	v := run(t, f, f.request(Status)).Data.(EnrollmentView)
	if v.TemplateReady || count(t, f, `SELECT COUNT(*) FROM face_templates`) != 1 {
		t.Fatal("role change deleted or resurrected template")
	}
}
func TestMissingDependenciesAndCrossRoleFailClosed(t *testing.T) {
	f := setup(t)
	f.s.keys = nil
	f.s.files = nil
	f.s.processor = nil
	consent := f.grant(t)
	r := f.request(Submit)
	r.ConsentID = consent
	r.JPEG = syntheticJPEG(t)
	_, err := f.s.Execute(context.Background(), r)
	code(t, err, "BIOMETRIC_UNAVAILABLE")
	photo := f.request(Photo)
	photo.ActorID = f.user
	photo.SessionID = f.userSession
	photo.TargetID = ids.New()
	_, err = f.s.Execute(context.Background(), photo)
	code(t, err, "FORBIDDEN_ROLE")
	withdraw := f.request(Withdraw)
	withdraw.Confirmed = true
	withdraw.Scope = "ALL_FACE_ENROLLMENT"
	run(t, f, withdraw)
}
func TestPersistentUploadRateLimitPrecedesImageDecode(t *testing.T) {
	f := setup(t)
	consent := f.grant(t)
	for range 3 {
		r := f.request(Submit)
		r.ConsentID = consent
		r.JPEG = []byte("invalid")
		_, err := f.s.Execute(context.Background(), r)
		code(t, err, "IMAGE_DECODE_FAILED")
	}
	r := f.request(Submit)
	r.ConsentID = consent
	r.JPEG = []byte("invalid")
	_, err := f.s.Execute(context.Background(), r)
	code(t, err, "RATE_LIMITED")
	if count(t, f, `SELECT count FROM enrollment_rate_buckets WHERE scope LIKE 'submit-hour:%'`) != 3 {
		t.Fatal("quota not durable")
	}
}
func TestCanonicalHashIsPersistedAndNotPublic(t *testing.T) {
	f := setup(t)
	r, result := f.submit(t, f.grant(t))
	canonical, _, _, _ := enrollmentsecure.NormalizeJPEG(r.JPEG)
	sum := sha256.Sum256(canonical)
	wipe(canonical)
	var hash string
	if err := f.db.QueryRowContext(context.Background(), `SELECT canonical_sha256 FROM face_enrollment_submissions WHERE id=?`, result.Data.(MutationReceipt).ResourceID).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if hash != hex.EncodeToString(sum[:]) {
		t.Fatal("canonical hash missing")
	}
	for _, name := range []string{"synthetic.db", "synthetic.db-wal"} {
		data, err := os.ReadFile(filepath.Join(f.root, name))
		if err == nil && bytes.Contains(data, r.JPEG) {
			t.Fatal("plaintext JPEG on disk")
		}
	}
}

func TestRecoveredRetainedPhotoRetryIsFencedAndBounded(t *testing.T) {
	f := setup(t)
	_, submitted := f.submit(t, f.grant(t))
	id := submitted.Data.(MutationReceipt).ResourceID
	f.approve(t, id)
	for attempt := range 3 {
		f.keys.setUnavailable(true)
		if err := f.s.ProcessNext(context.Background()); err != nil {
			t.Fatal(err)
		}
		f.keys.setUnavailable(false)
		detail := f.request(Detail)
		detail.TargetID = id
		current := run(t, f, detail).Data.(EnrollmentView)
		retry := f.request(Retry)
		retry.TargetID = id
		retry.IfMatch = etag(current.Revision)
		retry.Confirmed = true
		if attempt == 2 {
			_, err := f.s.Execute(context.Background(), retry)
			code(t, err, "RETRY_EXHAUSTED")
		} else {
			result := run(t, f, retry)
			if result.HTTPStatus != 202 {
				t.Fatal("retry not queued")
			}
		}
	}
	if count(t, f, `SELECT attempts_started FROM enrollment_processing_jobs`) != 3 || count(t, f, `SELECT COUNT(*) FROM face_enrollment_photos`) != 1 {
		t.Fatal("retry limit or protected photo invariant")
	}
}
func TestWithdrawalAuditFailureRollsBackAllPayloadDeletion(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	_, submitted := f.submit(t, f.grant(t))
	id := submitted.Data.(MutationReceipt).ResourceID
	f.approve(t, id)
	if err := f.s.ProcessNext(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.ExecContext(ctx, `CREATE TRIGGER synthetic_withdraw_audit_failure BEFORE INSERT ON audit_events WHEN NEW.action='template.deleted' BEGIN SELECT RAISE(ABORT,'synthetic'); END`); err != nil {
		t.Fatal(err)
	}
	r := f.request(Withdraw)
	r.Scope = "ALL_FACE_ENROLLMENT"
	r.Confirmed = true
	_, err := f.s.Execute(ctx, r)
	code(t, err, "AUDIT_UNAVAILABLE")
	if count(t, f, `SELECT COUNT(*) FROM face_templates`) != 1 || count(t, f, `SELECT COUNT(*) FROM consent_records WHERE purpose='face_enrollment' AND status='granted'`) != 1 || count(t, f, `SELECT COUNT(*) FROM face_enrollment_submissions WHERE status='ACTIVE'`) != 1 {
		t.Fatal("withdrawal partially committed despite failed audit")
	}
}
func TestPhotoExpiryDuringProcessorFencesLateActivation(t *testing.T) {
	f := setup(t)
	entered, release := make(chan struct{}), make(chan struct{})
	f.s.processor = &syntheticProcessor{fn: func(context.Context, []byte) (ProcessingResult, error) {
		close(entered)
		<-release
		v := make([]float32, 128)
		v[0] = 1
		return ProcessingResult{Embedding: v, FaceCount: 1, Manifest: syntheticManifest()}, nil
	}}
	_, submitted := f.submit(t, f.grant(t))
	f.approve(t, submitted.Data.(MutationReceipt).ResourceID)
	done := make(chan error, 1)
	go func() { done <- f.s.ProcessNext(context.Background()) }()
	<-entered
	f.clk.Advance(PhotoTTL)
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	status := run(t, f, f.request(Status)).Data.(EnrollmentView)
	if status.SubmissionStatus != "EXPIRED" || count(t, f, `SELECT COUNT(*) FROM face_templates`) != 0 {
		t.Fatalf("late expiry result=%+v", status)
	}
}
func TestAssistedWithdrawalWorksForSuspendedSubject(t *testing.T) {
	f := setup(t)
	f.submit(t, f.grant(t))
	if _, err := f.db.ExecContext(context.Background(), `UPDATE users SET status='suspended' WHERE id=?`, f.user); err != nil {
		t.Fatal(err)
	}
	r := f.request(AssistedWithdraw)
	r.TargetID = f.person
	r.Scope = "ALL_FACE_ENROLLMENT"
	r.Confirmed = true
	r.RequestMethod = "CAMPUS_STUDENT_REQUEST"
	r.RequestRef = ids.New()
	result := run(t, f, r)
	if result.Data.(MutationReceipt).Current.(EnrollmentView).ConsentState != "WITHDRAWN" || count(t, f, `SELECT COUNT(*) FROM face_enrollment_photos`) != 0 {
		t.Fatal("assisted withdrawal blocked")
	}
}
func TestWithdrawalReplayReportsNewCurrentConsentWithoutRepeatingEffect(t *testing.T) {
	f := setup(t)
	f.grant(t)
	withdraw := f.request(Withdraw)
	withdraw.Scope = "ALL_FACE_ENROLLMENT"
	withdraw.Confirmed = true
	run(t, f, withdraw)
	f.clk.Advance(time.Millisecond)
	newConsent := f.grant(t)
	result := run(t, f, withdraw)
	if !result.Replay || result.Data.(MutationReceipt).Current.(EnrollmentView).ConsentState != "ACTIVE" {
		t.Fatal("old receipt claimed current consent withdrawn")
	}
	var state string
	if err := f.db.QueryRowContext(context.Background(), `SELECT status FROM consent_records WHERE id=?`, newConsent).Scan(&state); err != nil || state != "granted" {
		t.Fatalf("replay revoked new consent=%s %v", state, err)
	}
}
func TestExpiredActorAndInvalidAttestationCannotCommit(t *testing.T) {
	f := setup(t)
	_, submitted := f.submit(t, f.grant(t))
	id := submitted.Data.(MutationReceipt).ResourceID
	r := f.request(Approve)
	r.TargetID = id
	r.IfMatch = `"1"`
	r.Confirmed = true
	r.IdentityCheckMethod = "CAMPUS_ROSTER_IN_PERSON"
	r.AttestationVersion = "production"
	r.CampusCheckRef = ids.New()
	_, err := f.s.Execute(context.Background(), r)
	code(t, err, "VALIDATION_FAILED")
	if _, err = f.db.ExecContext(context.Background(), `UPDATE session_tokens SET access_expires_at=? WHERE id=?`, nowText(f.clk.Now()), f.adminSession); err != nil {
		t.Fatal(err)
	}
	r.IdentityCheckMethod = "CAMPUS_AUTHORIZED_CHECK"
	r.AttestationVersion = "synthetic-v1"
	_, err = f.s.Execute(context.Background(), r)
	code(t, err, "UNAUTHORIZED")
	if count(t, f, `SELECT COUNT(*) FROM enrollment_processing_jobs`) != 0 {
		t.Fatal("expired actor committed decision")
	}
}

func TestCommittedDecisionReplaySurvivesSubjectIneligibility(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	_, submitted := f.submit(t, f.grant(t))
	id := submitted.Data.(MutationReceipt).ResourceID
	detail := f.request(Detail)
	detail.TargetID = id
	view := run(t, f, detail)
	approve := f.request(Approve)
	approve.TargetID = id
	approve.IfMatch = view.ETag
	approve.Confirmed = true
	approve.IdentityCheckMethod = "CAMPUS_AUTHORIZED_CHECK"
	approve.AttestationVersion = "synthetic-v1"
	approve.CampusCheckRef = ids.New()
	if result := run(t, f, approve); result.HTTPStatus != 202 {
		t.Fatalf("approve status=%d", result.HTTPStatus)
	}
	if _, err := f.db.ExecContext(ctx, `UPDATE users SET status='suspended' WHERE id=?`, f.user); err != nil {
		t.Fatal(err)
	}
	replay := run(t, f, approve)
	if !replay.Replay || replay.HTTPStatus != 200 || replay.Data.(MutationReceipt).ResourceID != id {
		t.Fatalf("committed receipt hidden after ineligibility: %+v", replay)
	}
	if count(t, f, `SELECT COUNT(*) FROM enrollment_processing_jobs`) != 1 {
		t.Fatal("replay duplicated the committed operation")
	}
}

func TestAdminDetailAllowsUnlinkedAccountWithoutFabricatedIdentity(t *testing.T) {
	f := setup(t)
	_, submitted := f.submit(t, f.grant(t))
	id := submitted.Data.(MutationReceipt).ResourceID
	if _, err := f.db.ExecContext(context.Background(), `UPDATE people SET account_user_id=NULL WHERE id=?`, f.person); err != nil {
		t.Fatal(err)
	}
	detail := f.request(Detail)
	detail.TargetID = id
	v := run(t, f, detail).Data.(EnrollmentView)
	if v.Student.UserID != "" || v.IdentityBinding.AccountUserID != "" || v.IdentityBinding.AccountStatus != "" || v.IdentityBinding.AccountRole != "" || v.IdentityBinding.PersonStatus != "active" || v.IdentityBinding.Eligible {
		t.Fatalf("unlinked binding fabricated identity: %+v", v.IdentityBinding)
	}
}

func TestAdminMetadataUsesLiveBindingRosterAndServiceClock(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	consentID := f.grant(t)
	_, submitted := f.submit(t, consentID)
	id := submitted.Data.(MutationReceipt).ResourceID
	if _, err := f.db.ExecContext(ctx, `UPDATE classes SET code='B-SYNTHETIC'`); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ code, status, deleted, left string }{
		{"A-SYNTHETIC", "active", "", ""},
		{"C-INACTIVE", "archived", "", ""},
		{"D-DELETED", "active", nowText(f.clk.Now()), ""},
		{"E-LEFT", "active", "", nowText(f.clk.Now())},
	} {
		classID := ids.New()
		if _, err := f.db.ExecContext(ctx, `INSERT INTO classes(id,code,title,status,deleted_at) VALUES(?,?,?, ?,NULLIF(?,''))`, classID, row.code, "Fixture "+row.code, row.status, row.deleted); err != nil {
			t.Fatal(err)
		}
		if _, err := f.db.ExecContext(ctx, `INSERT INTO class_memberships(class_id,person_id,created_by,left_at) VALUES(?,?,?,NULLIF(?,''))`, classID, f.person, f.admin, row.left); err != nil {
			t.Fatal(err)
		}
	}
	detail := f.request(Detail)
	detail.TargetID = id
	v := run(t, f, detail).Data.(EnrollmentView)
	if v.ServerNow != nowText(f.clk.Now()) || !v.SyntheticOnly || v.Consent == nil || v.Consent.RecordID != consentID || v.Consent.Notice != currentNotice() {
		t.Fatalf("clock/consent=%+v", v)
	}
	b := v.IdentityBinding
	if b == nil || b.AccountUserID != f.user || b.PersonID != f.person || b.ExternalCode != v.Student.ExternalCode || b.AccountStatus != "active" || b.AccountRole != "mahasiswa" || b.PersonStatus != "active" || b.IdentityReference != "synthetic-roster" || b.IdentityVerifiedAt != nowText(f.clk.Now()) || !b.Eligible {
		t.Fatalf("binding=%+v", b)
	}
	if len(b.Classes) != 2 || len(v.Student.Classes) != 2 || b.Classes[0].Code != "A-SYNTHETIC" || b.Classes[1].Code != "B-SYNTHETIC" {
		t.Fatalf("roster=%+v", b.Classes)
	}
	for i := range b.Classes {
		if b.Classes[i] != v.Student.Classes[i] || b.Classes[i].ID == "" || b.Classes[i].Title == "" {
			t.Fatal("student/binding roster diverged")
		}
	}
	queue := run(t, f, f.request(List)).Data.(ListView)
	if queue.ServerNow != v.ServerNow || len(queue.Items) != 1 || queue.Items[0].PhotoExpiresAt != v.PhotoExpiresAt || len(queue.Items[0].Student.Classes) != 2 {
		t.Fatalf("queue=%+v", queue)
	}
	for i := range b.Classes {
		if queue.Items[0].Student.Classes[i] != b.Classes[i] {
			t.Fatal("queue/detail roster diverged")
		}
	}
	own := run(t, f, f.request(Status)).Data.(EnrollmentView)
	if own.ServerNow != v.ServerNow || own.Student != nil || own.IdentityBinding != nil || own.Consent != nil || own.ArtifactIntegrity != "" {
		t.Fatal("student status exposed admin joins")
	}
	// Metadata reads must not claim they decrypted/hashed the artifact. A key
	// outage is invisible to metadata but the actual photo path must fail closed.
	f.keys.setUnavailable(true)
	v = run(t, f, detail).Data.(EnrollmentView)
	if !v.PhotoAvailable || v.ArtifactIntegrity != "NOT_CHECKED" {
		t.Fatalf("metadata falsely verified bytes=%+v", v)
	}
	photo := f.request(Photo)
	photo.TargetID = id
	_, err := f.s.Execute(ctx, photo)
	code(t, err, "BIOMETRIC_UNAVAILABLE")
	f.keys.setUnavailable(false)
	if _, err = f.db.ExecContext(ctx, `UPDATE users SET status='suspended' WHERE id=?`, f.user); err != nil {
		t.Fatal(err)
	}
	v = run(t, f, detail).Data.(EnrollmentView)
	if v.IdentityBinding.AccountStatus != "suspended" || v.IdentityBinding.Eligible || v.PhotoAvailable || v.ArtifactIntegrity != "UNAVAILABLE" {
		t.Fatalf("live account state not reflected=%+v", v)
	}
}

func TestAdminQueueAndDetailExpireAtExactServiceDeadline(t *testing.T) {
	f := setup(t)
	_, submitted := f.submit(t, f.grant(t))
	id := submitted.Data.(MutationReceipt).ResourceID
	deadline := nowText(f.clk.Now().Add(PhotoTTL))
	f.clk.Advance(PhotoTTL - time.Millisecond)
	list := f.request(List)
	list.FilterStatus = "PENDING_APPROVAL"
	before := run(t, f, list).Data.(ListView)
	if len(before.Items) != 1 || before.Items[0].PhotoExpiresAt != deadline || before.Summary.PendingCount != 1 || before.ServerNow != nowText(f.clk.Now()) {
		t.Fatalf("before deadline=%+v", before)
	}
	f.clk.Advance(time.Millisecond)
	at := run(t, f, list).Data.(ListView)
	if len(at.Items) != 0 || at.Summary.PendingCount != 0 || at.ServerNow != deadline {
		t.Fatalf("at deadline=%+v", at)
	}
	detail := f.request(Detail)
	detail.TargetID = id
	v := run(t, f, detail).Data.(EnrollmentView)
	if v.ServerNow != deadline || v.PhotoExpiresAt != deadline || v.SubmissionStatus != "EXPIRED" || v.PhotoAvailable || v.ArtifactIntegrity != "UNAVAILABLE" || v.Consent.State != "ACTIVE" {
		t.Fatalf("expired detail=%+v", v)
	}
	for _, action := range v.AllowedActions {
		if action == "APPROVE" || action == "REJECT" || action == "RETRY_PROCESSING" {
			t.Fatal("expired artifact remained actionable")
		}
	}
}

func TestAdminQueuePaginationAndOwnStatusRemainSubjectBound(t *testing.T) {
	f := setup(t)
	_, first := f.submit(t, f.grant(t))
	other := *f
	other.user, other.person, other.userSession = ids.New(), ids.New(), ids.New()
	ctx := context.Background()
	if _, err := f.db.ExecContext(ctx, `INSERT INTO users(id,email,display_name,role,status) VALUES(?,?,'Other synthetic','mahasiswa','active')`, other.user, other.user+"@example.invalid"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.ExecContext(ctx, `INSERT INTO people(id,external_code,display_name,account_user_id,identity_reference,identity_verified_at) VALUES(?,?,'Other synthetic person',?,'other-synthetic-roster',?)`, other.person, ids.New(), other.user, nowText(f.clk.Now())); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.ExecContext(ctx, `INSERT INTO class_memberships(class_id,person_id,created_by) SELECT id,?,? FROM classes`, other.person, f.admin); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.ExecContext(ctx, `INSERT INTO session_tokens(id,user_id,access_hash,refresh_hash,access_expires_at,refresh_expires_at,absolute_expires_at) VALUES(?,?,?,?,?,?,?)`, other.userSession, other.user, ids.New(), ids.New(), nowText(f.clk.Now().Add(7*24*time.Hour)), nowText(f.clk.Now().Add(7*24*time.Hour)), nowText(f.clk.Now().Add(7*24*time.Hour))); err != nil {
		t.Fatal(err)
	}
	// TargetID is not ownership authority on a self-service action.
	ownRequest := other.request(Status)
	ownRequest.TargetID = first.Data.(MutationReceipt).ResourceID
	own := run(t, &other, ownRequest).Data.(EnrollmentView)
	if own.SubmissionID != "" || own.EffectiveStatus != "NOT_ENROLLED" {
		t.Fatal("self status used caller target instead of authenticated binding")
	}
	_, second := other.submit(t, other.grant(t))
	want := map[string]string{first.Data.(MutationReceipt).ResourceID: f.person, second.Data.(MutationReceipt).ResourceID: other.person}
	list := f.request(List)
	list.Limit = 1
	page1 := run(t, f, list).Data.(ListView)
	if len(page1.Items) != 1 || !page1.Page.HasMore || page1.Page.NextCursor == "" || page1.Summary.PendingCount != 2 {
		t.Fatalf("first page=%+v", page1)
	}
	list.Cursor = page1.Page.NextCursor
	page2 := run(t, f, list).Data.(ListView)
	if len(page2.Items) != 1 || page2.Page.HasMore || page2.Page.NextCursor != "" || page2.Items[0].ID == page1.Items[0].ID {
		t.Fatalf("second page=%+v", page2)
	}
	for _, page := range []ListView{page1, page2} {
		item := page.Items[0]
		if item.Student.PersonID != want[item.ID] || len(item.Student.Classes) != 1 || item.PhotoExpiresAt == "" || page.ServerNow != nowText(f.clk.Now()) {
			t.Fatalf("page join crossed subjects=%+v", item)
		}
	}
}

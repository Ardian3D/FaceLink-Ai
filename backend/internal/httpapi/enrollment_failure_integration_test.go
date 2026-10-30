package httpapi

// FL-ENR05 failure-boundary integration tests. Requests use a real, certificate-
// verified TLS socket and the production router/SQLite/domain/crypto/file store.
// Authentication is the existing synthetic HTTP fixture; processing below is
// explicitly deterministic fault injection, NOT a real-model positive result.

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollment"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollmentsecure"
	"github.com/google/uuid"
)

type enrFailureProcessor struct {
	fn func(context.Context, []byte) (enrollment.ProcessingResult, error)
}

func (enrFailureProcessor) Manifest() enrollment.Manifest { return enrHTTPProcessor{}.Manifest() }
func (p enrFailureProcessor) Process(ctx context.Context, b []byte) (enrollment.ProcessingResult, error) {
	if p.fn != nil {
		return p.fn(ctx, b)
	}
	return enrHTTPProcessor{}.Process(ctx, b)
}

type enrFailureHTTPS struct {
	h                                *harnessAuth
	svc                              *enrollment.Service
	server                           *httptest.Server
	keys                             enrHTTPKeys
	files                            *enrollmentsecure.FileStore
	admin, student, lecturer, person string
}

func newEnrFailureHTTPS(t *testing.T, processor enrollment.Processor) *enrFailureHTTPS {
	t.Helper()
	h, _, admin, student, lecturer, person := enrHTTPSetup(t)
	keys := enrHTTPKeys{}
	for _, purpose := range []string{enrollmentsecure.PendingPhoto, enrollmentsecure.Template} {
		keys[purpose] = make([]byte, 32)
		if _, err := rand.Read(keys[purpose]); err != nil {
			t.Fatal(err)
		}
	}
	files, err := enrollmentsecure.NewFileStore(filepath.Join(t.TempDir(), "ciphertext"))
	if err != nil {
		t.Fatal(err)
	}
	service, err := enrollment.New(h.db, h.clk, keys, files, processor)
	if err != nil {
		t.Fatal(err)
	}
	h.srv.Enrollment = service
	h.router = NewRouter(h.srv)
	server := httptest.NewTLSServer(h.router)
	t.Cleanup(server.Close)
	server.Client().Timeout = 10 * time.Second
	return &enrFailureHTTPS{h: h, svc: service, server: server, keys: keys, files: files, admin: admin, student: student, lecturer: lecturer, person: person}
}

func (f *enrFailureHTTPS) send(method, route, token string, body []byte, headers map[string]string) responsUji {
	f.h.t.Helper()
	req, err := http.NewRequest(method, f.server.URL+route, bytes.NewReader(body))
	if err != nil {
		f.h.t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	// The client trusts only the httptest server certificate; no TLS bypass.
	response, err := f.server.Client().Do(req)
	if err != nil {
		f.h.t.Fatal(err)
	}
	defer response.Body.Close()
	if response.TLS == nil || len(response.TLS.VerifiedChains) == 0 {
		f.h.t.Fatal("request did not use certificate-verified TLS")
	}
	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		f.h.t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	for name, values := range response.Header {
		rec.Header()[name] = values
	}
	rec.WriteHeader(response.StatusCode)
	_, _ = rec.Write(bodyBytes)
	result := responsUji{Status: response.StatusCode, Rec: rec}
	if response.Header.Get("Content-Type") != "image/jpeg" && len(bodyBytes) != 0 {
		if err := json.Unmarshal(bodyBytes, &result.Body); err != nil {
			f.h.t.Fatal("invalid API JSON")
		}
	}
	return result
}

func (f *enrFailureHTTPS) json(route, token string, body any, tag, key string) responsUji {
	f.h.t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		f.h.t.Fatal(err)
	}
	headers := map[string]string{"Content-Type": "application/json", "Idempotency-Key": key}
	if tag != "" {
		headers["If-Match"] = tag
	}
	return f.send("POST", route, token, encoded, headers)
}

func (f *enrFailureHTTPS) grant() string {
	f.h.t.Helper()
	notice := f.send("GET", "/api/v1/me/consent", f.student, nil, nil)
	f.h.harapSukses(notice, 200)
	result := f.h.harapSukses(f.json("/api/v1/me/consent", f.student, map[string]any{"notice_version": enrollment.NoticeVersion, "accepted": true}, notice.Rec.Header().Get("ETag"), uuid.NewString()), 201)
	return result["resource_id"].(string)
}

func (f *enrFailureHTTPS) upload(consent, key string) responsUji {
	return f.send("POST", "/api/v1/me/face-enrollment-submissions", f.student, enrJPEG(f.h.t), map[string]string{"Content-Type": "image/jpeg", "X-Consent-Id": consent, "Idempotency-Key": key})
}

func (f *enrFailureHTTPS) submit() string {
	f.h.t.Helper()
	return f.h.harapSukses(f.upload(f.grant(), uuid.NewString()), 201)["resource_id"].(string)
}

func (f *enrFailureHTTPS) detail(id string) responsUji {
	return f.send("GET", "/api/v1/face-enrollment-submissions/"+id, f.admin, nil, nil)
}

func enrFailureAttestation() map[string]any {
	return map[string]any{"identity_check_method": "CAMPUS_AUTHORIZED_CHECK", "attestation_version": "synthetic-v1", "confirmed": true, "campus_check_ref": uuid.NewString()}
}

func (f *enrFailureHTTPS) approve(id string) {
	f.h.t.Helper()
	detail := f.detail(id)
	f.h.harapSukses(detail, 200)
	approved := f.h.harapSukses(f.json("/api/v1/face-enrollment-submissions/"+id+"/approve", f.admin, enrFailureAttestation(), detail.Rec.Header().Get("ETag"), uuid.NewString()), 202)
	if approved["current"].(map[string]any)["template_ready"] != false {
		f.h.t.Fatal("approval claimed an activated template")
	}
}

func (f *enrFailureHTTPS) status(want string) {
	f.h.t.Helper()
	data := f.h.harapSukses(f.send("GET", "/api/v1/me/face-enrollment", f.student, nil, nil), 200)
	if data["effective_status"] != want || data["template_ready"] != false {
		f.h.t.Fatalf("expected closed status %s, got %v", want, data)
	}
}

func (f *enrFailureHTTPS) renewFixtureSessions() {
	f.h.t.Helper()
	// Auth fixtures only: issue fresh real Store sessions after a 72h clock jump,
	// without changing an existing token's expiry or the production auth algorithm.
	for _, item := range []struct {
		email string
		token *string
	}{{"admin.enrollment@kampus.example", &f.admin}, {"student.enrollment@kampus.example", &f.student}} {
		var user string
		if err := f.h.db.QueryRowContext(context.Background(), "SELECT id FROM users WHERE email=?", item.email).Scan(&user); err != nil {
			f.h.t.Fatal(err)
		}
		pair, err := f.h.srv.Sessions.Issue(context.Background(), user, "synthetic-failure-test")
		if err != nil {
			f.h.t.Fatal(err)
		}
		*item.token = pair.AccessToken
	}
}

func (f *enrFailureHTTPS) noActivation() {
	f.h.t.Helper()
	for _, query := range []string{"SELECT count(*) FROM face_templates", "SELECT count(*) FROM face_enrollments WHERE status='active'", "SELECT count(*) FROM audit_events WHERE action='enrollment.activated'"} {
		if f.h.hitung(query) != 0 {
			f.h.t.Fatal("failure crossed activation boundary")
		}
	}
}

func TestEnrollmentFailureHTTPSConsentAndDecisionFences(t *testing.T) {
	t.Run("consent-not-given", func(t *testing.T) {
		f := newEnrFailureHTTPS(t, enrFailureProcessor{})
		response := f.upload(uuid.NewString(), uuid.NewString())
		f.h.harapGalat(response, 422, "CONSENT_REQUIRED")
		if f.h.hitung("SELECT count(*) FROM face_enrollment_submissions") != 0 || f.h.hitung("SELECT count(*) FROM face_enrollment_photos") != 0 {
			t.Fatal("unconsented upload persisted")
		}
		f.noActivation()
	})
	t.Run("withdraw-before-approval", func(t *testing.T) {
		f := newEnrFailureHTTPS(t, enrFailureProcessor{})
		id := f.submit()
		before := f.detail(id)
		f.h.harapSukses(before, 200)
		f.h.harapSukses(f.json("/api/v1/me/consent/withdraw", f.student, map[string]any{"scope": "ALL_FACE_ENROLLMENT", "confirmed": true}, "", uuid.NewString()), 200)
		response := f.json("/api/v1/face-enrollment-submissions/"+id+"/approve", f.admin, enrFailureAttestation(), before.Rec.Header().Get("ETag"), uuid.NewString())
		f.h.harapGalat(response, 412, "PRECONDITION_FAILED")
		f.status("REVOKED")
		f.noActivation()
		if f.h.hitung("SELECT count(*) FROM enrollment_processing_jobs WHERE state IN ('QUEUED','RUNNING')") != 0 {
			t.Fatal("withdrawn approval queued work")
		}
	})
	t.Run("invalid-roster-binding", func(t *testing.T) {
		f := newEnrFailureHTTPS(t, enrFailureProcessor{})
		consent := f.grant()
		f.h.execSQL("UPDATE class_memberships SET left_at=? WHERE person_id=?", f.h.clk.Now().UTC().Format("2006-01-02T15:04:05.000Z"), f.person)
		f.h.harapGalat(f.upload(consent, uuid.NewString()), 409, "SUBJECT_INELIGIBLE")
		if f.h.hitung("SELECT count(*) FROM face_enrollment_submissions") != 0 || f.h.hitung("SELECT count(*) FROM face_enrollment_photos") != 0 {
			t.Fatal("invalid roster binding persisted an artifact")
		}
		f.noActivation()
	})
	t.Run("attestation-and-etag", func(t *testing.T) {
		f := newEnrFailureHTTPS(t, enrFailureProcessor{})
		id := f.submit()
		detail := f.detail(id)
		f.h.harapSukses(detail, 200)
		route := "/api/v1/face-enrollment-submissions/" + id + "/approve"
		for _, patch := range []map[string]any{{"confirmed": false}, {"attestation_version": "unrecognized"}, {"identity_check_method": "UNVERIFIED"}, {"campus_check_ref": "not-a-uuid"}} {
			body := enrFailureAttestation()
			for key, value := range patch {
				body[key] = value
			}
			response := f.json(route, f.admin, body, detail.Rec.Header().Get("ETag"), uuid.NewString())
			if response.Status != 400 && response.Status != 422 {
				t.Fatalf("invalid attestation accepted: %d", response.Status)
			}
		}
		f.h.harapGalat(f.json(route, f.admin, enrFailureAttestation(), "\"999\"", uuid.NewString()), 412, "PRECONDITION_FAILED")
		f.status("PENDING_APPROVAL")
		f.noActivation()
	})
}

func TestEnrollmentFailureHTTPSExact72HourRetention(t *testing.T) {
	f := newEnrFailureHTTPS(t, enrFailureProcessor{})
	consent, key := f.grant(), uuid.NewString()
	id := f.h.harapSukses(f.upload(consent, key), 201)["resource_id"].(string)
	var expires string
	if err := f.h.db.QueryRowContext(context.Background(), "SELECT expires_at FROM face_enrollment_photos WHERE submission_id=?", id).Scan(&expires); err != nil {
		t.Fatal(err)
	}
	f.h.harapSukses(f.upload(consent, key), 200)
	f.h.clk.Advance(72*time.Hour - time.Millisecond)
	f.renewFixtureSessions()
	// Receipt retention is 24h, independent of the unchanged 72h photo TTL.
	f.h.harapGalat(f.upload(consent, key), 409, "ENROLLMENT_EXISTS")
	f.status("PENDING_APPROVAL")
	preview := f.send("GET", "/api/v1/face-enrollment-submissions/"+id+"/photo", f.admin, nil, nil)
	if preview.Status != 200 || preview.Rec.Header().Get("Content-Type") != "image/jpeg" || !bytes.HasPrefix(preview.Rec.Body.Bytes(), []byte{255, 216}) {
		t.Fatal("pending photo unavailable one millisecond before deadline")
	}
	var afterReplay string
	if err := f.h.db.QueryRowContext(context.Background(), "SELECT expires_at FROM face_enrollment_photos WHERE submission_id=?", id).Scan(&afterReplay); err != nil {
		t.Fatal(err)
	}
	if expires != afterReplay {
		t.Fatal("upload replay extended photo deadline")
	}
	f.h.clk.Advance(time.Millisecond)
	f.status("EXPIRED")
	f.h.harapGalat(f.send("GET", "/api/v1/face-enrollment-submissions/"+id+"/photo", f.admin, nil, nil), 410, "PHOTO_UNAVAILABLE")
	detail := f.detail(id)
	f.h.harapSukses(detail, 200)
	f.h.harapGalat(f.json("/api/v1/face-enrollment-submissions/"+id+"/approve", f.admin, enrFailureAttestation(), detail.Rec.Header().Get("ETag"), uuid.NewString()), 409, "INVALID_TRANSITION")
	if f.h.hitung("SELECT count(*) FROM face_enrollment_photos") != 0 {
		t.Fatal("deadline retained logical photo")
	}
	f.noActivation()
	if err := f.svc.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	files, err := f.files.IDs()
	if err != nil || len(files) != 0 {
		t.Fatal("expiry reconciliation retained ciphertext")
	}
}

func TestEnrollmentFailureHTTPSRetryKeepsOriginal72HourDeadline(t *testing.T) {
	f := newEnrFailureHTTPS(t, enrFailureProcessor{})
	id := f.submit()
	f.approve(id)
	var deadline string
	if err := f.h.db.QueryRowContext(context.Background(), "SELECT expires_at FROM face_enrollment_photos WHERE submission_id=?", id).Scan(&deadline); err != nil {
		t.Fatal(err)
	}
	photoKey := f.keys[enrollmentsecure.PendingPhoto]
	delete(f.keys, enrollmentsecure.PendingPhoto)
	if err := f.svc.ProcessNext(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.keys[enrollmentsecure.PendingPhoto] = photoKey
	f.status("FAILED_PROCESSING")
	f.h.clk.Advance(72*time.Hour - time.Second)
	f.renewFixtureSessions()
	detail := f.detail(id)
	f.h.harapSukses(detail, 200)
	f.h.harapSukses(f.json("/api/v1/face-enrollment-submissions/"+id+"/retry-processing", f.admin, map[string]any{"confirmed": true}, detail.Rec.Header().Get("ETag"), uuid.NewString()), 202)
	var retained string
	if err := f.h.db.QueryRowContext(context.Background(), "SELECT expires_at FROM face_enrollment_photos WHERE submission_id=?", id).Scan(&retained); err != nil {
		t.Fatal(err)
	}
	if retained != deadline {
		t.Fatal("processing retry extended original photo deadline")
	}
	f.h.clk.Advance(time.Second)
	if err := f.svc.ProcessNext(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.status("EXPIRED")
	f.noActivation()
	detail = f.detail(id)
	f.h.harapSukses(detail, 200)
	f.h.harapGalat(f.json("/api/v1/face-enrollment-submissions/"+id+"/retry-processing", f.admin, map[string]any{"confirmed": true}, detail.Rec.Header().Get("ETag"), uuid.NewString()), 409, "RETRY_NOT_ALLOWED")
}

func TestEnrollmentFailureHTTPSInFlightResultsAreFenced(t *testing.T) {
	for _, boundary := range []string{"consent-withdrawal", "reviewer-role-revoked-restored", "stale-lease"} {
		t.Run(boundary, func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			defer releaseOnce.Do(func() { close(release) })
			var calls atomic.Int32
			processor := enrFailureProcessor{fn: func(ctx context.Context, b []byte) (enrollment.ProcessingResult, error) {
				calls.Add(1)
				close(entered)
				select {
				case <-release:
					return enrHTTPProcessor{}.Process(ctx, b)
				case <-ctx.Done():
					return enrollment.ProcessingResult{}, ctx.Err()
				}
			}}
			f := newEnrFailureHTTPS(t, processor)
			id := f.submit()
			f.approve(id)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- f.svc.ProcessNext(ctx) }()
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("injected processor did not enter")
			}
			switch boundary {
			case "consent-withdrawal":
				f.h.harapSukses(f.json("/api/v1/me/consent/withdraw", f.student, map[string]any{"scope": "ALL_FACE_ENROLLMENT", "confirmed": true}, "", uuid.NewString()), 200)
			case "reviewer-role-revoked-restored":
				f.h.execSQL("UPDATE users SET role='dosen' WHERE email='admin.enrollment@kampus.example'")
				f.h.harapGalat(f.detail(id), 403, codeForbiddenRole)
				f.h.execSQL("UPDATE users SET role='admin' WHERE email='admin.enrollment@kampus.example'")
			case "stale-lease":
				f.h.clk.Advance(30 * time.Second)
			}
			releaseOnce.Do(func() { close(release) })
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal("injected processor did not finish")
			}
			if calls.Load() != 1 {
				t.Fatal("unexpected processing call count")
			}
			want := "FAILED_PROCESSING"
			if boundary == "consent-withdrawal" {
				want = "REVOKED"
			}
			f.status(want)
			f.noActivation()
			if boundary == "reviewer-role-revoked-restored" {
				data := f.h.harapSukses(f.detail(id), 200)
				if data["reason_code"] != "REVIEWER_INELIGIBLE" {
					t.Fatal("reviewer restoration revived prior authority")
				}
			}
			if f.h.hitung("SELECT count(*) FROM face_enrollment_photos") != 0 {
				t.Fatal("fenced work retained logical photo")
			}
			if err := f.svc.Reconcile(context.Background()); err != nil {
				t.Fatal(err)
			}
			files, err := f.files.IDs()
			if err != nil || len(files) != 0 {
				t.Fatal("fenced work retained ciphertext after reconciliation")
			}
		})
	}
}

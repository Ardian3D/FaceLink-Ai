package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollment"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollmentfixture"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollmentsecure"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/migrations"
	"github.com/google/uuid"
)

type enrHTTPKeys map[string][]byte

func (k enrHTTPKeys) Active(_ context.Context, p string) (string, []byte, error) {
	b, ok := k[p]
	if !ok {
		return "", nil, enrollmentsecure.ErrUnavailable
	}
	return "1", append([]byte(nil), b...), nil
}
func (k enrHTTPKeys) ByVersion(ctx context.Context, p, v string) ([]byte, error) {
	if v != "1" {
		return nil, enrollmentsecure.ErrUnavailable
	}
	_, b, e := k.Active(ctx, p)
	return b, e
}

type enrHTTPProcessor struct{}

func (enrHTTPProcessor) Manifest() enrollment.Manifest {
	h := sha256.Sum256([]byte("synthetic-only-test"))
	d := hex.EncodeToString(h[:])
	return enrollment.Manifest{SyntheticOnly: true, ModelID: "synthetic-sface", ModelVersion: "test-v1", ModelChecksum: d, DetectorID: "synthetic-yunet", DetectorVersion: "test-v1", DetectorChecksum: d, PipelineRevision: "synthetic-v1"}
}
func (p enrHTTPProcessor) Process(_ context.Context, _ []byte) (enrollment.ProcessingResult, error) {
	v := make([]float32, 128)
	v[0] = 1
	return enrollment.ProcessingResult{Embedding: v, FaceCount: 1, Manifest: p.Manifest()}, nil
}
func enrHTTPSetup(t *testing.T) (*harnessAuth, *enrollment.Service, string, string, string, string) {
	t.Helper()
	h := newHarnessAuth(t)
	enrApplySchema(t, h)
	_, admin := h.adminPertama("admin.enrollment@kampus.example")
	studentID, student := h.undangDanMasuk(admin.Access, "student.enrollment@kampus.example", "Mahasiswa Sintetis", "mahasiswa")
	_, lecturer := h.undangDanMasuk(admin.Access, "dosen.enrollment@kampus.example", "Dosen Sintetis", "dosen")
	person := corePerson(h, admin.Access, "ENR-SYN-001")
	class := coreClass(h, admin.Access, "ENR-SYN")
	h.harapSukses(h.kirim(http.MethodPut, "/api/v1/classes/"+class+"/members/"+person, nil, admin.Access), http.StatusCreated)
	h.harapSukses(h.kirim(http.MethodPut, "/api/v1/people/"+person+"/account-link", map[string]any{"user_id": studentID}, admin.Access), http.StatusOK)
	keys := enrHTTPKeys{}
	for _, p := range []string{"pending-photo", "template"} {
		keys[p] = make([]byte, 32)
		if _, e := rand.Read(keys[p]); e != nil {
			t.Fatal(e)
		}
	}
	files, e := enrollmentsecure.NewFileStore(filepath.Join(t.TempDir(), "ciphertext"))
	if e != nil {
		t.Fatal(e)
	}
	svc, e := enrollment.New(h.db, h.clk, keys, files, enrHTTPProcessor{})
	if e != nil {
		t.Fatal(e)
	}
	h.srv.Enrollment = svc
	h.router = NewRouter(h.srv)
	return h, svc, admin.Access, student.Access, lecturer.Access, person
}
func enrApplySchema(t *testing.T, h *harnessAuth) {
	t.Helper()
	dir, e := enrollmentfixture.PrepareMigrations(filepath.Join("..", "..", "migrations"), filepath.Join(t.TempDir(), "migrations"))
	if e != nil {
		t.Fatal(e)
	}
	status, e := migrations.Run(context.Background(), h.db, dir)
	if e != nil {
		t.Fatal(e)
	}
	h.srv.Migrations = status
}
func enrJPEG(t *testing.T) []byte {
	t.Helper()
	im := image.NewRGBA(image.Rect(0, 0, 128, 128))
	for y := range 128 {
		for x := range 128 {
			im.SetRGBA(x, y, color.RGBA{uint8(x), uint8(y), 99, 255})
		}
	}
	var b bytes.Buffer
	if e := jpeg.Encode(&b, im, &jpeg.Options{Quality: 85}); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func enrSend(h *harnessAuth, method, path, token string, body []byte, headers map[string]string) responsUji {
	h.t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	out := responsUji{Status: rec.Code, Rec: rec}
	if rec.Header().Get("Content-Type") != "image/jpeg" && rec.Body.Len() > 0 {
		if e := json.Unmarshal(rec.Body.Bytes(), &out.Body); e != nil {
			h.t.Fatalf("invalid API JSON: %v", e)
		}
	}
	return out
}
func enrJSON(h *harnessAuth, path, token string, body any, tag, key string) responsUji {
	h.t.Helper()
	b, e := json.Marshal(body)
	if e != nil {
		h.t.Fatal(e)
	}
	headers := map[string]string{"Content-Type": "application/json", "Idempotency-Key": key}
	if tag != "" {
		headers["If-Match"] = tag
	}
	return enrSend(h, http.MethodPost, path, token, b, headers)
}
func enrGrant(h *harnessAuth, token string) string {
	h.t.Helper()
	notice := h.kirim(http.MethodGet, "/api/v1/me/consent", nil, token)
	h.harapSukses(notice, 200)
	res := h.harapSukses(enrJSON(h, "/api/v1/me/consent", token, map[string]any{"notice_version": enrollment.NoticeVersion, "accepted": true}, notice.Rec.Header().Get("ETag"), uuid.NewString()), 201)
	return res["resource_id"].(string)
}
func enrSubmit(h *harnessAuth, student, consent, key string, jpg []byte) responsUji {
	return enrSend(h, "POST", "/api/v1/me/face-enrollment-submissions", student, jpg, map[string]string{"Content-Type": "image/jpeg", "X-Consent-Id": consent, "Idempotency-Key": key})
}

func TestEnrollmentHTTPGuardsPreviewApprovalAndWithdrawal(t *testing.T) {
	h, svc, admin, student, lecturer, _ := enrHTTPSetup(t)
	consent := enrGrant(h, student)
	jpg := enrJPEG(t)
	key := uuid.NewString()
	first := h.harapSukses(enrSubmit(h, student, consent, key, jpg), 201)
	id := first["resource_id"].(string)
	path := "/api/v1/face-enrollment-submissions/" + id
	replay := h.harapSukses(enrSubmit(h, student, consent, key, jpg), 200)
	if replay["resource_id"] != id {
		t.Fatal("retry created another submission")
	}
	for _, token := range []string{student, lecturer} {
		h.harapGalat(enrSend(h, "GET", path+"/photo", token, nil, nil), 403, codeForbiddenRole)
		h.harapGalat(enrJSON(h, path+"/approve", token, map[string]any{"confirmed": true}, "\"1\"", uuid.NewString()), 403, codeForbiddenRole)
	}
	h.harapGalat(enrSend(h, "GET", "/api/v1/me/face-enrollment?person_id="+uuid.NewString(), student, nil, nil), 400, codeValidationFailed)
	detail := h.kirim(http.MethodGet, path, nil, admin)
	h.harapSukses(detail, 200)
	preview := enrSend(h, "GET", path+"/photo", admin, nil, nil)
	if preview.Status != 200 || preview.Rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("preview %d", preview.Status)
	}
	if _, e := jpeg.Decode(bytes.NewReader(preview.Rec.Body.Bytes())); e != nil {
		t.Fatal(e)
	}
	if preview.Rec.Header().Get("X-Content-Type-Options") != "nosniff" || preview.Rec.Header().Get("Content-Length") != fmt.Sprint(preview.Rec.Body.Len()) {
		t.Fatal("unsafe preview response headers")
	}
	if ranged := enrSend(h, "GET", path+"/photo", admin, nil, map[string]string{"Range": "bytes=0-5"}); ranged.Status != 400 {
		t.Fatalf("range preview=%d", ranged.Status)
	}
	h.execSQL("CREATE TRIGGER fail_enrollment_preview BEFORE INSERT ON audit_events WHEN NEW.action='photo.preview_authorized' BEGIN SELECT RAISE(ABORT,'synthetic audit failure'); END")
	blocked := enrSend(h, "GET", path+"/photo", admin, nil, nil)
	if blocked.Status < 500 || blocked.Rec.Header().Get("Content-Type") == "image/jpeg" || bytes.HasPrefix(blocked.Rec.Body.Bytes(), []byte{255, 216}) {
		t.Fatal("preview leaked despite failed audit")
	}
	h.execSQL("DROP TRIGGER fail_enrollment_preview")
	approved := h.harapSukses(enrJSON(h, path+"/approve", admin, map[string]any{"identity_check_method": "CAMPUS_AUTHORIZED_CHECK", "attestation_version": "synthetic-v1", "confirmed": true, "campus_check_ref": uuid.NewString()}, detail.Rec.Header().Get("ETag"), uuid.NewString()), 202)
	if approved["current"].(map[string]any)["effective_status"] != "APPROVED_PROCESSING" {
		t.Fatal("approval activated enrollment")
	}
	if e := svc.ProcessNext(context.Background()); e != nil {
		t.Fatal(e)
	}
	active := h.harapSukses(h.kirim("GET", "/api/v1/me/face-enrollment", nil, student), 200)
	if active["effective_status"] != "ACTIVE" || active["template_ready"] != true {
		t.Fatalf("synthetic activation %v", active)
	}
	h.harapSukses(enrJSON(h, "/api/v1/me/consent/withdraw", student, map[string]any{"scope": "ALL_FACE_ENROLLMENT", "confirmed": true}, "", uuid.NewString()), 200)
	withdrawn := h.harapSukses(h.kirim("GET", "/api/v1/me/face-enrollment", nil, student), 200)
	if withdrawn["template_ready"] != false || withdrawn["effective_status"] != "REVOKED" {
		t.Fatal("withdraw left biometrics ready")
	}
	if n := h.hitung("SELECT count(*) FROM face_templates"); n != 0 {
		t.Fatalf("templates after withdraw=%d", n)
	}
	logs := h.logs.String()
	for _, secret := range []string{admin, student, lecturer, string(jpg), "campus_check_ref", "identity_check_method", "ciphertext", "embedding"} {
		if strings.Contains(logs, secret) {
			t.Fatal("enrollment log leaked protected input")
		}
	}
}

func TestEnrollmentHTTPRejectsForgedPayloadAndExpiresWithoutScheduler(t *testing.T) {
	h, _, admin, student, _, _ := enrHTTPSetup(t)
	notice := h.kirim("GET", "/api/v1/me/consent", nil, student)
	h.harapSukses(notice, 200)
	for _, body := range []string{`{"notice_version":"synthetic-v1","accepted":true,"person_id":"forged"}`, `{"notice_version":"synthetic-v1","accepted":false,"accepted":true}`} {
		h.harapGalat(enrSend(h, "POST", "/api/v1/me/consent", student, []byte(body), map[string]string{"Content-Type": "application/json", "If-Match": notice.Rec.Header().Get("ETag"), "Idempotency-Key": uuid.NewString()}), 400, codeValidationFailed)
	}
	consent := enrGrant(h, student)
	key := uuid.NewString()
	jpg := enrJPEG(t)
	h.harapGalat(enrSubmit(h, student, consent, uuid.NewString(), []byte("not-jpeg")), 422, "IMAGE_DECODE_FAILED")
	first := h.harapSukses(enrSubmit(h, student, consent, key, jpg), 201)
	id := first["resource_id"].(string)
	path := "/api/v1/face-enrollment-submissions/" + id
	// Refresh synthetic sessions without changing the production session algorithm.
	h.clk.Advance(72 * time.Hour)
	h.execSQL("UPDATE session_tokens SET refresh_expires_at=?, access_expires_at=?, absolute_expires_at=?", h.clk.Now().Add(4*24*time.Hour).Format("2006-01-02T15:04:05.000Z"), h.clk.Now().Add(time.Hour).Format("2006-01-02T15:04:05.000Z"), h.clk.Now().Add(7*24*time.Hour).Format("2006-01-02T15:04:05.000Z"))
	own := h.harapSukses(h.kirim("GET", "/api/v1/me/face-enrollment", nil, student), 200)
	if own["effective_status"] != "EXPIRED" {
		t.Fatalf("expiry=%v", own)
	}
	photo := enrSend(h, "GET", path+"/photo", admin, nil, nil)
	if photo.Status != 410 {
		t.Fatalf("expired preview=%d", photo.Status)
	}
	expired := h.kirim("GET", path, nil, admin)
	h.harapSukses(expired, 200)
	h.harapGalat(enrJSON(h, path+"/approve", admin, map[string]any{"identity_check_method": "CAMPUS_AUTHORIZED_CHECK", "attestation_version": "synthetic-v1", "confirmed": true, "campus_check_ref": uuid.NewString()}, expired.Rec.Header().Get("ETag"), uuid.NewString()), 409, "INVALID_TRANSITION")
}

// This is a real TLS socket exercise of the production router, not a response mock.
func TestEnrollmentSyntheticHTTPS(t *testing.T) {
	h, _, _, student, _, _ := enrHTTPSetup(t)
	srv := httptest.NewTLSServer(h.router)
	defer srv.Close()
	req, e := http.NewRequest("GET", srv.URL+"/api/v1/me/consent", nil)
	if e != nil {
		t.Fatal(e)
	}
	req.Header.Set("Authorization", "Bearer "+student)
	res, e := srv.Client().Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer res.Body.Close()
	b, e := io.ReadAll(res.Body)
	if e != nil {
		t.Fatal(e)
	}
	if res.StatusCode != 200 || res.TLS == nil || !bytes.Contains(b, []byte("NOT_GIVEN")) {
		t.Fatalf("HTTPS consent response %d", res.StatusCode)
	}
}

func TestEnrollmentHTTPRejectsMalformedQueriesHeadersAndImageEnvelope(t *testing.T) {
	h, _, admin, student, _, _ := enrHTTPSetup(t)
	for _, p := range []string{"/api/v1/me/face-enrollment?person_id=%ZZ", "/api/v1/me/consent?user_id=forged"} {
		h.harapGalat(enrSend(h, "GET", p, student, nil, nil), 400, codeValidationFailed)
	}
	h.harapGalat(enrSend(h, "GET", "/api/v1/face-enrollment-submissions?status=ACTIVE;cursor=x", admin, nil, nil), 400, codeValidationFailed)
	body := map[string]any{"notice_version": enrollment.NoticeVersion, "accepted": true}
	for _, tag := range []string{`W/"0"`, `*`, `"0","1"`, `"-1"`, `"00"`, `"+1"`} {
		h.harapGalat(enrJSON(h, "/api/v1/me/consent", student, body, tag, uuid.NewString()), 400, codeValidationFailed)
	}
	h.harapGalat(enrJSON(h, "/api/v1/me/consent", student, body, "", uuid.NewString()), 428, "PRECONDITION_REQUIRED")
	consent := enrGrant(h, student)
	jpg := enrJPEG(t)
	wrong := enrSend(h, "POST", "/api/v1/me/face-enrollment-submissions", student, jpg, map[string]string{"Content-Type": "image/png", "X-Consent-Id": consent, "Idempotency-Key": uuid.NewString()})
	h.harapGalat(wrong, 415, "UNSUPPORTED_MEDIA_TYPE")
	oversized := enrSubmit(h, student, consent, uuid.NewString(), make([]byte, enrollmentsecure.MaxImageBytes+1))
	if oversized.Status != 413 {
		t.Fatalf("oversized status=%d", oversized.Status)
	}
	for _, length := range []int64{-1, 0, int64(len(jpg) + 1)} {
		req := httptest.NewRequest("POST", "/api/v1/me/face-enrollment-submissions", bytes.NewReader(jpg))
		req.ContentLength = length
		req.Header.Set("Authorization", "Bearer "+student)
		req.Header.Set("Content-Type", "image/jpeg")
		req.Header.Set("Idempotency-Key", uuid.NewString())
		req.Header.Set("X-Consent-Id", consent)
		rec := httptest.NewRecorder()
		h.router.ServeHTTP(rec, req)
		if rec.Code != 400 {
			t.Fatalf("invalid length=%d status=%d", length, rec.Code)
		}
	}
	req := httptest.NewRequest("POST", "/api/v1/me/face-enrollment-submissions", bytes.NewReader(jpg))
	req.Header.Set("Authorization", "Bearer "+student)
	req.Header.Set("Content-Type", "image/jpeg")
	req.Header.Set("X-Consent-Id", consent)
	req.Header.Add("Idempotency-Key", uuid.NewString())
	req.Header.Add("Idempotency-Key", uuid.NewString())
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Fatalf("duplicate critical header=%d", rec.Code)
	}
}

func TestEnrollmentHTTPRequiresExplicitInjectionAndPreservesManualReadiness(t *testing.T) {
	h := newHarnessAuth(t)
	_, admin := h.adminPertama("admin.disabled-enrollment@kampus.example")
	absent := h.kirim("GET", "/api/v1/face-enrollment-submissions", nil, admin.Access)
	if absent.Status != 404 || !strings.Contains(absent.Rec.Body.String(), `"NOT_FOUND"`) {
		t.Fatalf("uninjected enrollment route status=%d body=%s", absent.Status, absent.Rec.Body.String())
	}
	h.harapSukses(h.kirim("GET", "/api/v1/me", nil, admin.Access), 200)
	if n := h.hitung("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='face_templates'"); n != 0 {
		t.Fatal("normal schema enabled synthetic enrollment")
	}
	enrApplySchema(t, h)
	// A configured domain with unavailable key/processor must not disable core.
	svc, e := enrollment.New(h.db, h.clk, nil, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	h.srv.Enrollment = svc
	h.router = NewRouter(h.srv)
	h.harapSuksesLarik(h.kirim("GET", "/api/v1/classes", nil, admin.Access), 200)
	if got := h.hitung("SELECT count(*) FROM face_templates"); got != 0 {
		t.Fatalf("key unavailable created template=%d", got)
	}
}

func TestEnrollmentAdminMetadataEnvelopeAndLiveRoleLoss(t *testing.T) {
	h, _, admin, student, lecturer, personID := enrHTTPSetup(t)
	submission := h.harapSukses(enrSubmit(h, student, enrGrant(h, student), uuid.NewString(), enrJPEG(t)), 201)
	id := submission["resource_id"].(string)
	path := "/api/v1/face-enrollment-submissions/" + id
	detail := h.harapSukses(h.kirim("GET", path, nil, admin), 200)
	if detail["server_now"] != h.clk.Now().UTC().Format("2006-01-02T15:04:05.000Z") || detail["synthetic_only"] != true || detail["artifact_integrity"] != "NOT_CHECKED" {
		t.Fatalf("admin metadata=%v", detail)
	}
	binding := detail["identity_binding"].(map[string]any)
	consent := detail["consent"].(map[string]any)
	joinedStudent := detail["student"].(map[string]any)
	if binding["person_id"] != personID || binding["account_user_id"] != joinedStudent["user_id"] || binding["account_role"] != "mahasiswa" || binding["eligible"] != true || consent["state"] != "ACTIVE" {
		t.Fatalf("admin binding/consent=%v/%v", binding, consent)
	}
	classes := binding["classes"].([]any)
	if len(classes) != 1 || classes[0].(map[string]any)["code"] != "ENR-SYN" || len(joinedStudent["classes"].([]any)) != 1 {
		t.Fatalf("admin roster=%v", binding)
	}
	queue := h.kirim("GET", "/api/v1/face-enrollment-submissions?status=PENDING_APPROVAL&limit=1", nil, admin)
	if queue.Status != 200 || queue.Body["server_now"] != detail["server_now"] || queue.Rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("queue envelope=%v", queue.Body)
	}
	items := queue.Body["data"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["photo_expires_at"] != detail["photo_expires_at"] {
		t.Fatalf("queue expiry=%v", items)
	}
	if len(items[0].(map[string]any)["student"].(map[string]any)["classes"].([]any)) != 1 {
		t.Fatal("queue lost active roster metadata")
	}
	own := h.harapSukses(h.kirim("GET", "/api/v1/me/face-enrollment", nil, student), 200)
	for _, field := range []string{"student", "consent", "identity_binding", "artifact_integrity"} {
		if _, present := own[field]; present {
			t.Fatalf("self status exposed admin field %s", field)
		}
	}
	if own["server_now"] != detail["server_now"] {
		t.Fatal("own/admin clocks diverged")
	}
	// RBAC precedes lookup, including absent IDs; neither student nor lecturer
	// can use the new metadata joins as an existence or identity oracle.
	unknown := "/api/v1/face-enrollment-submissions/" + uuid.NewString()
	for _, token := range []string{student, lecturer} {
		for _, route := range []string{"/api/v1/face-enrollment-submissions", path, unknown} {
			h.harapGalat(h.kirim("GET", route, nil, token), 403, codeForbiddenRole)
		}
		for _, action := range []string{"reject", "retry-processing"} {
			h.harapGalat(enrJSON(h, unknown+"/"+action, token, map[string]any{"confirmed": true}, "\"1\"", uuid.NewString()), 403, codeForbiddenRole)
		}
	}
	h.execSQL("UPDATE users SET role='dosen' WHERE email='admin.enrollment@kampus.example'")
	for _, route := range []string{"/api/v1/face-enrollment-submissions", path, path + "/photo"} {
		h.harapGalat(h.kirim("GET", route, nil, admin), 403, codeForbiddenRole)
	}
	if n := h.hitung("SELECT count(*) FROM face_enrollment_submissions WHERE status='PENDING_APPROVAL'"); n != 1 {
		t.Fatal("role loss unexpectedly made a review decision")
	}
}

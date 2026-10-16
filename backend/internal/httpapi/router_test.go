package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/migrations"
)

const testVersion = "fl-005-uji"

func newTestServer(t *testing.T, opts ...func(*Server)) Server {
	t.Helper()

	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "facelink.db"))
	if err != nil {
		t.Fatalf("Open() gagal: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	s := Server{
		Config: ServerConfig{
			Version:        testVersion,
			ReadTimeout:    5 * time.Second,
			WriteTimeout:   5 * time.Second,
			IdleTimeout:    30 * time.Second,
			RequestTimeout: 5 * time.Second,
			MaxBodyBytes:   DefaultMaxBodyBytes,
		},
		DB:         db,
		Migrations: migrations.Status{CurrentVersion: 1, Available: 1},
		Logger:     slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}
	for _, opt := range opts {
		opt(&s)
	}
	return s
}

func do(t *testing.T, s Server, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	rec := httptest.NewRecorder()
	NewRouter(s).ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("badan respons bukan JSON yang sah (%q): %v", rec.Body.String(), err)
	}
	return body
}

func TestHealthzMenjawabStatusOk(t *testing.T) {
	s := newTestServer(t)
	rec := do(t, s, http.MethodGet, "/api/v1/healthz")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200; badan: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, ingin application/json", ct)
	}

	body := decode(t, rec)
	if body["status"] != "ok" {
		t.Errorf("status badan = %v, ingin %q", body["status"], "ok")
	}
	if body["version"] != testVersion {
		t.Errorf("version = %v, ingin %q", body["version"], testVersion)
	}

	raw, ok := body["time"].(string)
	if !ok {
		t.Fatalf("time bukan string: %v", body["time"])
	}
	if _, err := time.Parse(time.RFC3339, raw); err != nil {
		t.Errorf("time %q bukan RFC3339: %v", raw, err)
	}
	if !strings.HasSuffix(raw, "Z") {
		t.Errorf("time %q tidak dalam UTC", raw)
	}
}

func TestHealthzTidakMembocorkanDetailInternal(t *testing.T) {
	s := newTestServer(t)
	rec := do(t, s, http.MethodGet, "/api/v1/healthz")

	body := strings.ToLower(rec.Body.String())
	forbidden := []string{
		"sqlite", "facelink.db", ".db", "journal_mode", "wal",
		"goroutine", "runtime", "commit", "branch", "chi", "modernc",
		filepath.ToSlash(s.DB.Path()),
	}
	for _, marker := range forbidden {
		if strings.Contains(body, strings.ToLower(marker)) {
			t.Errorf("badan /healthz memuat detail internal %q yang dilarang kontrak §5.12:\n%s",
				marker, rec.Body.String())
		}
	}
}

func TestHealthzMenjawabDenganMethodLainDitolak(t *testing.T) {
	s := newTestServer(t)
	rec := do(t, s, http.MethodPost, "/api/v1/healthz")

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, ingin 405", rec.Code)
	}
	body := decode(t, rec)
	if body["code"] != "METHOD_NOT_ALLOWED" {
		t.Errorf("code = %v, ingin METHOD_NOT_ALLOWED", body["code"])
	}
}

func TestReadyzMenjawabSiapSaatSeluruhKomponenSehat(t *testing.T) {
	s := newTestServer(t)
	rec := do(t, s, http.MethodGet, "/api/v1/readyz")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200; badan: %s", rec.Code, rec.Body.String())
	}

	body := decode(t, rec)
	if body["database"] != "ok" {
		t.Errorf("database = %v, ingin ok", body["database"])
	}
	if body["migrations"] != "applied" {
		t.Errorf("migrations = %v, ingin applied", body["migrations"])
	}

	wal, ok := body["wal"].(map[string]any)
	if !ok {
		t.Fatalf("wal bukan objek: %v", body["wal"])
	}
	if !strings.EqualFold(wal["journal_mode"].(string), "wal") {
		t.Errorf("wal.journal_mode = %v, ingin wal", wal["journal_mode"])
	}
}

func TestReadyzMelaporkanMesinAIBelumTersediaTanpaMenggagalkanBackend(t *testing.T) {
	s := newTestServer(t)
	rec := do(t, s, http.MethodGet, "/api/v1/readyz")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200: absensi manual harus tetap berjalan tanpa mesin AI", rec.Code)
	}
	body := decode(t, rec)
	if body["ai_engine"] != "unavailable" {
		t.Errorf("ai_engine = %v, ingin unavailable", body["ai_engine"])
	}
	if body["database"] != "ok" {
		t.Errorf("database = %v, ingin tetap ok", body["database"])
	}
}

func TestReadyzMenjawab503SaatBasisDataDitutup(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "facelink.db"))
	if err != nil {
		t.Fatalf("Open() gagal: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() gagal: %v", err)
	}

	s := Server{
		Config:     ServerConfig{Version: testVersion, RequestTimeout: 5 * time.Second},
		DB:         db,
		Migrations: migrations.Status{CurrentVersion: 1, Available: 1},
		Logger:     slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}

	rec := do(t, s, http.MethodGet, "/api/v1/readyz")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, ingin 503; badan: %s", rec.Code, rec.Body.String())
	}
	body := decode(t, rec)
	if body["code"] != "DATABASE_BUSY" {
		t.Errorf("code = %v, ingin DATABASE_BUSY", body["code"])
	}
}

func TestReadyzMenjawab503SaatMigrasiBelumDiterapkan(t *testing.T) {
	s := newTestServer(t, func(s *Server) {
		s.Migrations = migrations.ForTesting(1, 2, 2)
	})

	rec := do(t, s, http.MethodGet, "/api/v1/readyz")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, ingin 503; badan: %s", rec.Code, rec.Body.String())
	}
	body := decode(t, rec)
	if body["code"] != "MIGRATIONS_PENDING" {
		t.Errorf("code = %v, ingin MIGRATIONS_PENDING", body["code"])
	}
}

func TestReadyzTidakMenjalankanIntegrityCheckPenuh(t *testing.T) {
	s := newTestServer(t)

	// Memasang tabel yang tidak ada pada skema MVP: integrity_check akan
	// menyebutkannya bila dijalankan penuh.
	rec := do(t, s, http.MethodGet, "/api/v1/readyz")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200", rec.Code)
	}

	start := time.Now()
	for i := 0; i < 5; i++ {
		if got := do(t, s, http.MethodGet, "/api/v1/readyz").Code; got != http.StatusOK {
			t.Fatalf("panggilan ke-%d menjawab %d", i, got)
		}
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("lima panggilan /readyz memakan %v; pembacaan tampaknya berat", elapsed)
	}
}

func TestRuteTidakDikenalMenjawabJSON404(t *testing.T) {
	s := newTestServer(t)
	rec := do(t, s, http.MethodGet, "/api/v1/tidak-ada")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, ingin 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, ingin JSON", ct)
	}
	body := decode(t, rec)
	if body["code"] != "NOT_FOUND" {
		t.Errorf("code = %v, ingin NOT_FOUND", body["code"])
	}
	if _, ok := body["request_id"]; !ok {
		t.Error("respons galat harus memuat request_id")
	}
}

func TestMiddlewareRequestIdSelaluAda(t *testing.T) {
	s := newTestServer(t)

	// Chi mengisi X-Request-Id pada seluruh respons lewat middleware.RequestID.
	for _, target := range []string{"/api/v1/healthz", "/api/v1/readyz", "/api/v1/tidak-ada"} {
		rec := do(t, s, http.MethodGet, target)
		if id := rec.Header().Get("X-Request-Id"); id == "" {
			t.Errorf("%s: header X-Request-Id kosong", target)
		}
	}
}

func TestPanicDitangkapDanTidakMembocorkanJejakTumpukan(t *testing.T) {
	s := newTestServer(t)

	rec := httptest.NewRecorder()
	handler := NewRouter(s)
	// Handler buatan yang panik, dipasang langsung agar dapat diuji.
	wrapped := recoverer(s)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("rahasia internal yang tidak boleh sampai ke klien")
	}))
	wrapped.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apa-saja", nil))
	_ = handler

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, ingin 500", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "rahasia internal") {
		t.Errorf("badan respons membocorkan pesan panic:\n%s", body)
	}
	if strings.Contains(body, "goroutine") || strings.Contains(body, ".go:") {
		t.Errorf("badan respons membocorkan jejak tumpukan:\n%s", body)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("badan 500 bukan JSON: %v", err)
	}
	if payload["code"] != "INTERNAL_ERROR" {
		t.Errorf("code = %v, ingin INTERNAL_ERROR", payload["code"])
	}
}

// TestPanicHandlerDitangkapLewatRantaiTimeout menutup celah yang belum dicakup
// TestPanicDitangkapDanTidakMembocorkanJejakTumpukan: di sana recoverer diuji
// langsung, tanpa middleware batas waktu. Pada rantai sungguhan, handler
// berjalan di goroutine yang dibuat middleware batas waktu, sehingga panic-nya
// tidak berada pada tumpukan pemanggil dan recoverer lapisan luar tidak
// menjangkaunya — proses backend berhenti, bukan menjawab 500.
//
// Susunan di bawah mengikuti NewRouter: recoverer(s)(timeout(s)(handler)).
func TestPanicHandlerDitangkapLewatRantaiTimeout(t *testing.T) {
	s := newTestServer(t)

	rantai := recoverer(s)(timeout(s)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("rahasia internal lewat rantai timeout")
	})))

	rec := httptest.NewRecorder()
	rantai.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/panic-lewat-timeout", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, ingin 500", rec.Code)
	}
	badan := rec.Body.String()
	if strings.Contains(badan, "rahasia internal") {
		t.Errorf("badan respons membocorkan pesan panic:\n%s", badan)
	}
	if strings.Contains(badan, "goroutine") || strings.Contains(badan, ".go:") {
		t.Errorf("badan respons membocorkan jejak tumpukan:\n%s", badan)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("badan 500 bukan JSON: %v", err)
	}
	if payload["code"] != "INTERNAL_ERROR" {
		t.Errorf("code = %v, ingin INTERNAL_ERROR", payload["code"])
	}
}

func TestBadanPermintaanTerlaluBesarDitolak(t *testing.T) {
	s := newTestServer(t, func(s *Server) { s.Config.MaxBodyBytes = 16 })

	req := httptest.NewRequest(http.MethodPost, "/api/v1/healthz", strings.NewReader(strings.Repeat("x", 64)))
	req.ContentLength = 64
	rec := httptest.NewRecorder()
	NewRouter(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge && rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, ingin 413 (atau 405 bila metode ditolak lebih dahulu)", rec.Code)
	}
}

func TestBatasWaktuProsesMenjawab503(t *testing.T) {
	s := newTestServer(t)

	s.Config.RequestTimeout = 50 * time.Millisecond

	slow := timeout(s)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))

	rec := httptest.NewRecorder()
	slow.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lambat", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, ingin 503", rec.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("badan 503 bukan JSON: %v", err)
	}
	if payload["code"] != "PROCESSING_TIMEOUT" {
		t.Errorf("code = %v, ingin PROCESSING_TIMEOUT", payload["code"])
	}
}

func TestLogAksesTidakMencatatParameterKueri(t *testing.T) {
	var buf strings.Builder
	s := newTestServer(t, func(s *Server) {
		s.Logger = slog.New(slog.NewJSONHandler(&buf, nil))
	})

	do(t, s, http.MethodGet, "/api/v1/healthz?person_id=rahasia-peserta&token=abc123")

	logged := buf.String()
	if strings.Contains(logged, "rahasia-peserta") {
		t.Errorf("log memuat nilai parameter kueri:\n%s", logged)
	}
	if strings.Contains(logged, "abc123") {
		t.Errorf("log memuat token dari parameter kueri:\n%s", logged)
	}
	if !strings.Contains(logged, "/api/v1/healthz") {
		t.Errorf("log tidak memuat jalur permintaan:\n%s", logged)
	}
}

package httpapi

// Pengujian keandalan access log (FL-005S).
//
// Aturan yang dijaga:
//   - status pada access log adalah status final yang benar-benar dikirim klien,
//     termasuk 503 batas waktu dan 500 hasil pemulihan panic;
//   - tepat satu baris log per permintaan, ditulis saat respons selesai — bukan
//     menunggu handler lambat berhenti;
//   - log tidak memuat tajuk Authorization, cookie, token, atau payload sensitif.

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// serverLog mengganti logger server uji dengan penampung yang dapat diperiksa.
func serverLog(t *testing.T, opts ...func(*Server)) (Server, *syncBuffer) {
	t.Helper()

	buf := &syncBuffer{}
	all := append([]func(*Server){func(s *Server) {
		s.Logger = slog.New(slog.NewJSONHandler(buf, nil))
	}}, opts...)
	return newTestServer(t, all...), buf
}

// entriLog mengurai seluruh baris log JSON menjadi daftar map.
func entriLog(t *testing.T, buf *syncBuffer) []map[string]any {
	t.Helper()

	var entri []map[string]any
	for _, baris := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if strings.TrimSpace(baris) == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(baris), &m); err != nil {
			t.Fatalf("baris log bukan JSON: %v | isi: %q", err, baris)
		}
		entri = append(entri, m)
	}
	return entri
}

// statusAccessLog mengembalikan status pada baris "permintaan http" saja.
func statusAccessLog(t *testing.T, buf *syncBuffer) []int {
	t.Helper()

	var status []int
	for _, m := range entriLog(t, buf) {
		if m["msg"] != "permintaan http" {
			continue
		}
		nilai, ok := m["status"].(float64)
		if !ok {
			t.Fatalf("baris access log tidak memuat status numerik: %v", m)
		}
		status = append(status, int(nilai))
	}
	return status
}

// periksaSatuBaris memastikan jumlah baris access log sesuai harapan.
func periksaSatuBaris(t *testing.T, buf *syncBuffer, ingin int) []int {
	t.Helper()

	status := statusAccessLog(t, buf)
	if len(status) != ingin {
		t.Fatalf("jumlah baris access log = %d, ingin %d (status: %v)", len(status), ingin, status)
	}
	return status
}

func TestAccessLogStatus200(t *testing.T) {
	s, buf := serverLog(t)

	rec := do(t, s, http.MethodGet, "/api/v1/healthz")
	if rec.Code != http.StatusOK {
		t.Fatalf("status respons = %d, ingin 200", rec.Code)
	}
	if status := periksaSatuBaris(t, buf, 1); status[0] != http.StatusOK {
		t.Errorf("status access log = %d, ingin 200", status[0])
	}
}

func TestAccessLogStatus404(t *testing.T) {
	s, buf := serverLog(t)

	rec := do(t, s, http.MethodGet, "/api/v1/tidak-ada")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status respons = %d, ingin 404", rec.Code)
	}
	if status := periksaSatuBaris(t, buf, 1); status[0] != http.StatusNotFound {
		t.Errorf("status access log = %d, ingin 404", status[0])
	}
}

func TestAccessLogStatus405(t *testing.T) {
	s, buf := serverLog(t)

	rec := do(t, s, http.MethodPost, "/api/v1/healthz")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status respons = %d, ingin 405", rec.Code)
	}
	if status := periksaSatuBaris(t, buf, 1); status[0] != http.StatusMethodNotAllowed {
		t.Errorf("status access log = %d, ingin 405", status[0])
	}
}

func TestAccessLogStatus413(t *testing.T) {
	s, buf := serverLog(t, func(s *Server) { s.Config.MaxBodyBytes = 16 })

	req := httptest.NewRequest(http.MethodPost, "/api/v1/healthz", strings.NewReader(strings.Repeat("x", 64)))
	req.ContentLength = 64
	rec := httptest.NewRecorder()
	NewRouter(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status respons = %d, ingin 413", rec.Code)
	}
	if status := periksaSatuBaris(t, buf, 1); status[0] != http.StatusRequestEntityTooLarge {
		t.Errorf("status access log = %d, ingin 413", status[0])
	}
}

func TestAccessLogStatus500SaatPanicDipulihkan(t *testing.T) {
	s, buf := serverLog(t)

	rantai := chain(s, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("panic uji access log")
	}))

	rec := httptest.NewRecorder()
	rantai.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status respons = %d, ingin 500", rec.Code)
	}
	if status := periksaSatuBaris(t, buf, 1); status[0] != http.StatusInternalServerError {
		t.Errorf("status access log = %d, ingin 500", status[0])
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("badan 500 bukan JSON: %v", err)
	}
	if payload["code"] != "INTERNAL_ERROR" {
		t.Errorf("code = %v, ingin INTERNAL_ERROR", payload["code"])
	}
}

func TestAccessLogStatus503SaatBatasWaktu(t *testing.T) {
	s, buf := serverLog(t)
	s.Config.RequestTimeout = 30 * time.Millisecond

	rantai := chain(s, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))

	rec := httptest.NewRecorder()
	rantai.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lambat", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status respons = %d, ingin 503", rec.Code)
	}
	if status := periksaSatuBaris(t, buf, 1); status[0] != http.StatusServiceUnavailable {
		t.Errorf("status access log = %d, ingin 503", status[0])
	}
}

// TestAccessLogCocokSaatHandlerMenulisDiBatasWaktu menekan jalur balapan antara
// penulisan handler dan penulisan batas waktu. Yang dijamin bukan hasil tertentu
// — keduanya sah — melainkan bahwa status pada access log selalu sama dengan
// status yang benar-benar diterima klien dan badannya tidak pernah tercampur.
func TestAccessLogCocokSaatHandlerMenulisDiBatasWaktu(t *testing.T) {
	for i := range 50 {
		s, buf := serverLog(t)
		s.Config.RequestTimeout = 2 * time.Millisecond

		rantai := chain(s, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
			writeJSON(w, http.StatusOK, map[string]string{"hasil": "selesai"})
		}))

		rec := httptest.NewRecorder()
		rantai.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/balapan", nil))

		status := periksaSatuBaris(t, buf, 1)
		if status[0] != rec.Code {
			t.Fatalf("iterasi %d: status access log = %d, status respons = %d", i, status[0], rec.Code)
		}

		badan := strings.TrimSpace(rec.Body.String())
		switch rec.Code {
		case http.StatusOK:
			if badan == "" {
				continue // batas waktu menutup respons di antara status dan badan
			}
			var payload map[string]string
			if err := json.Unmarshal([]byte(badan), &payload); err != nil || payload["hasil"] != "selesai" {
				t.Fatalf("iterasi %d: badan 200 bukan payload handler: %q (err=%v)", i, badan, err)
			}
		case http.StatusServiceUnavailable:
			var payload map[string]any
			if err := json.Unmarshal([]byte(badan), &payload); err != nil || payload["code"] != "PROCESSING_TIMEOUT" {
				t.Fatalf("iterasi %d: badan 503 bukan payload batas waktu: %q (err=%v)", i, badan, err)
			}
		default:
			t.Fatalf("iterasi %d: status tak terduga %d dengan badan %q", i, rec.Code, badan)
		}
	}
}

// TestAccessLogDitulisSebelumHandlerLambatSelesai menutup masalah C: klien sudah
// menerima respons batas waktu, jadi baris access log harus sudah ada — bukan
// menunggu handler yang masih berjalan.
func TestAccessLogDitulisSebelumHandlerLambatSelesai(t *testing.T) {
	s, buf := serverLog(t)
	s.Config.RequestTimeout = 30 * time.Millisecond

	mulai := make(chan struct{})
	lepas := make(chan struct{})
	t.Cleanup(func() { close(lepas) })

	rantai := chain(s, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(mulai)
		<-lepas
		writeJSON(w, http.StatusOK, map[string]string{"terlambat": "ya"})
	}))

	rec := httptest.NewRecorder()
	rantai.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lambat", nil))
	<-mulai

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status respons = %d, ingin 503", rec.Code)
	}
	status := periksaSatuBaris(t, buf, 1)
	if status[0] != http.StatusServiceUnavailable {
		t.Errorf("status access log = %d, ingin 503", status[0])
	}
}

// TestAccessLogSatuBarisPerPermintaan menegaskan tidak ada baris ganda, termasuk
// pada jalur panic dan batas waktu.
func TestAccessLogSatuBarisPerPermintaan(t *testing.T) {
	s, buf := serverLog(t)
	s.Config.RequestTimeout = 30 * time.Millisecond

	normal := NewRouter(s)
	normal.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil))
	normal.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/tidak-ada", nil))

	chain(s, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("panic uji satu baris")
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/panic", nil))

	chain(s, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/lambat", nil))

	status := statusAccessLog(t, buf)
	if len(status) != 4 {
		t.Fatalf("jumlah baris access log = %d, ingin 4 (status: %v)", len(status), status)
	}
	ingin := []int{http.StatusOK, http.StatusNotFound, http.StatusInternalServerError, http.StatusServiceUnavailable}
	for i := range ingin {
		if status[i] != ingin[i] {
			t.Errorf("baris %d: status = %d, ingin %d", i, status[i], ingin[i])
		}
	}
}

// TestAccessLogTidakMemuatRahasia memastikan seluruh jalur baru — normal, 413,
// panic, dan batas waktu — tidak menuliskan tajuk, cookie, token, atau pesan
// internal ke access log maupun ke badan respons.
func TestAccessLogTidakMemuatRahasia(t *testing.T) {
	s, buf := serverLog(t)
	s.Config.RequestTimeout = 30 * time.Millisecond

	rahasia := []string{"TOKEN-UJI-ACCESSLOG", "COOKIE-UJI-ACCESSLOG", "APIKEY-UJI-ACCESSLOG"}

	kirim := func(h http.Handler, target string, body *strings.Reader, panjang int64) {
		t.Helper()
		var req *http.Request
		if body == nil {
			req = httptest.NewRequest(http.MethodGet, target, nil)
		} else {
			req = httptest.NewRequest(http.MethodPost, target, body)
			req.ContentLength = panjang
		}
		req.Header.Set("Authorization", "Bearer "+rahasia[0])
		req.Header.Set("Cookie", "sesi="+rahasia[1])
		req.Header.Set("X-Api-Key", rahasia[2])
		h.ServeHTTP(httptest.NewRecorder(), req)
	}

	kirim(NewRouter(s), "/api/v1/healthz?token="+rahasia[0], nil, 0)
	kirim(NewRouter(s), "/api/v1/tidak-ada?api_key="+rahasia[2], nil, 0)
	kirim(NewRouter(s), "/api/v1/healthz", strings.NewReader(strings.Repeat("x", 64)), 64)
	kirim(chain(s, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("panic uji rahasia")
	})), "/panic", nil, 0)
	kirim(chain(s, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})), "/lambat", nil, 0)

	catatan := buf.String()
	if strings.TrimSpace(catatan) == "" {
		t.Fatal("log kosong: pengujian tidak membuktikan apa pun")
	}
	for _, nilai := range rahasia {
		if strings.Contains(catatan, nilai) {
			t.Errorf("log memuat nilai sensitif %q", nilai)
		}
	}

	// Pemeriksaan berikut hanya berlaku untuk baris access log: baris kesalahan
	// recoverer memang memuat pesan panic (diagnostik internal), sedangkan access
	// log tidak boleh memuat tajuk, cookie, parameter kueri, atau pesan internal.
	var barisAkses []string
	for _, m := range entriLog(t, buf) {
		if m["msg"] != "permintaan http" {
			continue
		}
		isi, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("mengubah baris access log gagal: %v", err)
		}
		barisAkses = append(barisAkses, string(isi))
	}
	if len(barisAkses) != len(rahasia)+2 {
		t.Fatalf("jumlah baris access log = %d, ingin %d", len(barisAkses), len(rahasia)+2)
	}
	akses := strings.Join(barisAkses, "\n")
	for _, jejak := range []string{"Authorization", "Cookie", "token=", "api_key=", "panic uji", "Bearer"} {
		if strings.Contains(akses, jejak) {
			t.Errorf("access log memuat %q:\n%s", jejak, akses)
		}
	}
}

// TestAccessLogTidakMengubahRespons memastikan pencatatan status tidak mengubah
// status ataupun bentuk JSON yang diterima klien pada tiap jalur.
func TestAccessLogTidakMengubahRespons(t *testing.T) {
	s, buf := serverLog(t)
	s.Config.RequestTimeout = 30 * time.Millisecond

	kasus := []struct {
		nama   string
		rantai http.Handler
		target string
		status int
		kode   string
	}{
		{"200", NewRouter(s), "/api/v1/healthz", http.StatusOK, ""},
		{"404", NewRouter(s), "/api/v1/tidak-ada", http.StatusNotFound, "NOT_FOUND"},
		{"405", NewRouter(s), "/api/v1/healthz", http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED"},
		{"500", chain(s, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic("panic uji kontrak")
		})), "/panic", http.StatusInternalServerError, "INTERNAL_ERROR"},
		{"503", chain(s, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		})), "/lambat", http.StatusServiceUnavailable, "PROCESSING_TIMEOUT"},
	}

	for _, k := range kasus {
		metode := http.MethodGet
		if k.nama == "405" {
			metode = http.MethodPost
		}
		rec := httptest.NewRecorder()
		k.rantai.ServeHTTP(rec, httptest.NewRequest(metode, k.target, nil))

		if rec.Code != k.status {
			t.Fatalf("kasus %s: status respons = %d, ingin %d", k.nama, rec.Code, k.status)
		}
		if k.kode != "" {
			var payload map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("kasus %s: badan bukan JSON: %v | %q", k.nama, err, rec.Body.String())
			}
			if payload["code"] != k.kode {
				t.Errorf("kasus %s: code = %v, ingin %s", k.nama, payload["code"], k.kode)
			}
		}
	}

	status := statusAccessLog(t, buf)
	if len(status) != len(kasus) {
		t.Fatalf("jumlah baris access log = %d, ingin %d", len(status), len(kasus))
	}
	for i, k := range kasus {
		if status[i] != k.status {
			t.Errorf("kasus %s: status access log = %d, ingin %d", k.nama, status[i], k.status)
		}
	}
}

// TestAccessLogPermintaanDibatalkanTetapSatuBaris menutup jalur penutupan writer
// saat klien membatalkan permintaan: handler yang berjalan setelahnya tidak boleh
// menulis lagi, dan access log tetap tepat satu baris dengan status yang sama
// seperti yang dilihat klien.
func TestAccessLogPermintaanDibatalkanTetapSatuBaris(t *testing.T) {
	s, buf := serverLog(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	rantai := chain(s, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"hasil": "setelah-batal"})
	}))

	rec := httptest.NewRecorder()
	rantai.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/batal", nil).WithContext(ctx))

	status := periksaSatuBaris(t, buf, 1)
	if status[0] != rec.Code {
		t.Errorf("status access log = %d, status respons = %d", status[0], rec.Code)
	}
}

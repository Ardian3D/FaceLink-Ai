package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestResponsYangSudahDikirimTidakDitimpaBatasWaktu menutup celah yang belum
// dicakup TestBatasWaktuProsesMenjawab503: di sana handler tidak menulis apa pun
// sebelum batas waktu tercapai, sehingga jalur "handler sudah mengirim respons
// lalu batas waktu menyusul" tidak teruji.
//
// Kontrak yang dijaga: middleware timeout hanya menjawab 503 bila handler belum
// menulis apa pun (timeoutWriter.timeoutResponse memeriksanya di bawah kunci
// yang sama dengan penulisan). Respons yang sudah dikirim tidak boleh ditimpa,
// dan badan respons tidak boleh tercampur dua payload JSON (lihat komentar
// timeoutWriter pada payload.go).
func TestResponsYangSudahDikirimTidakDitimpaBatasWaktu(t *testing.T) {
	s := newTestServer(t)
	s.Config.RequestTimeout = 50 * time.Millisecond

	slow := timeout(s)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handler menyelesaikan pekerjaannya dan mengirim respons lebih dahulu,
		// baru kemudian melewati batas waktu.
		writeJSON(w, http.StatusOK, map[string]string{"hasil": "selesai"})
		<-r.Context().Done()
		time.Sleep(20 * time.Millisecond)
	}))

	rec := httptest.NewRecorder()
	slow.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/selesai-lalu-lambat", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200; respons yang sudah dikirim tidak boleh ditimpa", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "PROCESSING_TIMEOUT") {
		t.Fatalf("badan memuat payload timeout sehingga tercampur: %q", rec.Body.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("badan bukan satu objek JSON utuh: %v | isi: %q", err, rec.Body.String())
	}
	if payload["hasil"] != "selesai" {
		t.Errorf("badan = %q, ingin memuat hasil awal tanpa perubahan", rec.Body.String())
	}
}

// TestBatasWaktuTidakMencampurBadanRespons menekan titik balapan antara
// goroutine handler dan goroutine middleware batas waktu: handler menulis
// responsnya tepat ketika batas waktu tercapai, sehingga keduanya memakai
// ResponseWriter yang sama.
//
// Hasil akhir boleh berupa respons handler atau respons batas waktu, tergantung
// siapa yang menulis lebih dahulu. Yang tidak boleh terjadi adalah campuran
// keduanya — mis. badan berisi {"hasil":"selesai"} dan
// {"code":"PROCESSING_TIMEOUT"} sekaligus, atau status 200 dengan badan 503.
//
// Pengujian dijalankan berulang karena kedua urutan tersebut bergantung waktu;
// pengujian ini juga yang menjaga balapan data (temuan `go test -race`
// FL-005R-FIX) tetap tertutup.
func TestBatasWaktuTidakMencampurBadanRespons(t *testing.T) {
	for i := range 100 {
		s := newTestServer(t)
		s.Config.RequestTimeout = 2 * time.Millisecond

		slow := timeout(s)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Menulis kira-kira bersamaan dengan jatuhnya batas waktu.
			time.Sleep(2 * time.Millisecond)
			writeJSON(w, http.StatusOK, map[string]string{"hasil": "selesai"})
		}))

		rec := httptest.NewRecorder()
		slow.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/balapan-menulis", nil))

		badan := strings.TrimSpace(rec.Body.String())
		switch rec.Code {
		case http.StatusOK:
			if badan == "" {
				// Batas waktu menutup respons di antara status dan badan; ini
				// keadaan yang sah, bukan campuran payload.
				continue
			}
			var payload map[string]string
			if err := json.Unmarshal([]byte(badan), &payload); err != nil || payload["hasil"] != "selesai" {
				t.Fatalf("iterasi %d: status 200 dengan badan bukan payload handler: %q (err=%v)", i, badan, err)
			}
		case http.StatusServiceUnavailable:
			var payload map[string]any
			if err := json.Unmarshal([]byte(badan), &payload); err != nil || payload["code"] != "PROCESSING_TIMEOUT" {
				t.Fatalf("iterasi %d: status 503 dengan badan bukan payload batas waktu: %q (err=%v)", i, badan, err)
			}
		default:
			t.Fatalf("iterasi %d: status tak terduga %d dengan badan %q", i, rec.Code, badan)
		}
	}
}

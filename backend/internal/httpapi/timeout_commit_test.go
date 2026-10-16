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
// menulis apa pun (timeoutWriter.committed). Respons yang sudah dikirim tidak
// boleh ditimpa, dan badan respons tidak boleh tercampur dua payload JSON
// (lihat komentar timeoutWriter pada payload.go).
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

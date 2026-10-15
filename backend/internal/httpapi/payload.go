package httpapi

import (
	"encoding/json"
	"net/http"
)

// healthzPayload adalah badan /healthz. Field-nya dibatasi oleh kontrak:
// jalur berkas, versi pustaka internal, nama basis data, dan status perangkat
// tidak boleh muncul di sini (docs/API_CONTRACT.md §5.12).
type healthzPayload struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Time    string `json:"time"`
}

// readyzPayload adalah badan /readyz saat seluruh komponen siap.
type readyzPayload struct {
	Status     string     `json:"status"`
	Database   string     `json:"database"`
	AIEngine   string     `json:"ai_engine"`
	Migrations string     `json:"migrations"`
	WAL        walPayload `json:"wal"`
}

type walPayload struct {
	JournalMode string `json:"journal_mode"`
}

// readyzError adalah badan /readyz saat 503. Bentuknya mengikuti pola galat
// seragam docs/API_CONTRACT.md §3.
type readyzError struct {
	Code      string        `json:"code"`
	Message   string        `json:"message"`
	RequestID string        `json:"request_id"`
	Details   readyzPayload `json:"details"`
}

// timeoutWriter mencatat apakah handler sudah mulai menulis respons, sehingga
// middleware batas waktu tidak menimpa respons yang sudah terkirim. Bila
// penulisan ditolak karena respons sudah dikomit, writeJSON melewatinya supaya
// badan respons tidak tercampur.
type timeoutWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

// committed melaporkan apakah respons sudah dikirim, baik karena WriteHeader
// eksplisit maupun karena penulisan badan pertama.
func (t *timeoutWriter) committed() bool { return t.wrote }

func (t *timeoutWriter) WriteHeader(code int) {
	t.status = code
	if !t.wrote {
		t.wrote = true
		t.ResponseWriter.WriteHeader(code)
	}
}

// writeHeader hanya ada untuk memenuhi antarmuka yang dipakai writeJSON.
// Middleware batas waktu hanya menulis satu respons, sehingga tidak ada status
// tersembunyi yang perlu ditahan.
func (t *timeoutWriter) writeHeader(code int, final bool) bool {
	t.WriteHeader(code)
	return true
}

func (t *timeoutWriter) Write(b []byte) (int, error) {
	t.wrote = true
	return t.ResponseWriter.Write(b)
}

// errorPayload adalah galat seragam untuk respons non-kesiapan.
type errorPayload struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	h, ok := w.(interface{ writeHeader(int, bool) bool })
	if !ok {
		w.WriteHeader(status)
		writeJSONBody(w, body)
		return
	}
	// writeHeader melaporkan apakah status benar-benar terkirim; bila tidak
	// (misalnya respons sudah dikomit lebih dahulu), badan respons tidak ikut
	// ditulis agar tidak tercampur.
	if !h.writeHeader(status, true) {
		return
	}
	writeJSONBody(w, body)
}

func writeJSONBody(w http.ResponseWriter, body any) {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(body)
}

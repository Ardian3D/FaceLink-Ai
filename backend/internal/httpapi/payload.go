package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
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
	Status           string           `json:"status"`
	Database         string           `json:"database"`
	AIEngine         string           `json:"ai_engine"`
	Migrations       string           `json:"migrations"`
	WAL              walPayload       `json:"wal"`
	ManualAttendance serviceReadiness `json:"manual_attendance"`
	Biometrics       serviceReadiness `json:"biometrics"`
}

// serviceReadiness memisahkan readiness core dari jalur biometrik D-030.
type serviceReadiness struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
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
//
// Handler berjalan pada goroutine terpisah dari middleware batas waktu (lihat
// timeout pada router.go). Karena itu ResponseWriter yang mendasari hanya boleh
// disentuh satu goroutine pada satu waktu, dan seluruh penandanya dijaga satu
// mutex. Dua hal yang pernah terbukti gagal sebelum FL-005R-FIX:
//
//   - pembacaan penanda "sudah menulis" dari goroutine middleware tanpa urutan
//     yang ditentukan (balapan data yang dilaporkan `go test -race`);
//   - penulisan peta header ResponseWriter dari dua goroutine sekaligus melalui
//     Header() (berhenti dengan `fatal error: concurrent map writes`).
//
// Karena itu header disimpan pada peta pribadi milik handler dan baru disalin
// ke ResponseWriter yang mendasari ketika respons benar-benar dikirim.
type timeoutWriter struct {
	http.ResponseWriter
	header http.Header
	mu     sync.Mutex
	// wrote menandai handler sudah mengirim status atau badan respons.
	wrote bool
	// sealed menandai respons sudah diakhiri middleware batas waktu. Setelah
	// itu tulisan handler diabaikan agar tidak ada goroutine yang menyentuh
	// ResponseWriter sesudah middleware mengembalikan kendali ke net/http.
	sealed bool
}

// Header mengembalikan peta header pribadi handler. Peta itu hanya diakses
// goroutine handler sampai disalin ke ResponseWriter pada pengiriman pertama.
func (t *timeoutWriter) Header() http.Header {
	return t.header
}

// WriteHeader meneruskan status pertama yang dikirim handler; panggilan
// berikutnya diabaikan, mengikuti perilaku net/http.
func (t *timeoutWriter) WriteHeader(code int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.writeHeaderLocked(code)
}

// writeHeaderLocked melaporkan apakah status benar-benar terkirim. Harus
// dipanggil dengan mutex dipegang.
func (t *timeoutWriter) writeHeaderLocked(code int) bool {
	if t.wrote || t.sealed {
		return false
	}
	t.wrote = true
	t.copyHeaderLocked()
	t.ResponseWriter.WriteHeader(code)
	return true
}

// writeHeader hanya ada untuk memenuhi antarmuka yang dipakai writeJSON.
func (t *timeoutWriter) writeHeader(code int, final bool) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.writeHeaderLocked(code)
}

func (t *timeoutWriter) Write(b []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.sealed {
		return 0, http.ErrHandlerTimeout
	}
	if !t.wrote {
		// Menulis badan tanpa WriteHeader berarti status 200, dan header
		// handler harus ikut terkirim sebelum badannya.
		t.wrote = true
		t.copyHeaderLocked()
	}
	return t.ResponseWriter.Write(b)
}

// copyHeaderLocked menyalin header handler ke ResponseWriter yang mendasari
// tepat sebelum respons dikirim. Harus dipanggil dengan mutex dipegang.
func (t *timeoutWriter) copyHeaderLocked() {
	dst := t.ResponseWriter.Header()
	for k, values := range t.header {
		dst[k] = append([]string(nil), values...)
	}
}

// timeoutResponse menulis respons batas waktu bila handler belum mengirim apa
// pun, lalu menutup writer agar tulisan handler berikutnya diabaikan.
//
// Pemeriksaan "belum menulis" dan penulisan respons terjadi di bawah satu
// kunci. Memeriksa lebih dahulu lalu menulis di luar kunci membuka peluang
// respons batas waktu tercampur dengan respons handler — persis pola yang
// diperbaiki pada FL-005R-FIX.
//
// Penutupan dilakukan juga ketika respons handler sudah terkirim: setelah
// middleware batas waktu mengembalikan kendali, handler yang masih berjalan
// tidak boleh lagi menulis ke ResponseWriter.
func (t *timeoutWriter) timeoutResponse(status int, body any) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	alreadyWritten := t.wrote
	t.sealed = true
	if alreadyWritten {
		return false
	}

	t.wrote = true
	w := t.ResponseWriter
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	writeJSONBody(w, body)
	return true
}

// seal menutup writer tanpa menulis apa pun. Dipakai ketika permintaan
// dibatalkan klien: handler dapat masih berjalan sesaat, dan tulisan sesudahnya
// tidak boleh lagi menyentuh ResponseWriter.
func (t *timeoutWriter) seal() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sealed = true
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

func writeJSONBody(w io.Writer, body any) {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(body)
}

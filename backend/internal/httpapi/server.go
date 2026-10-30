// Package httpapi memasang router HTTP, middleware, dan endpoint kesehatan.
package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/audit"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/auth"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/authz"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/clock"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/emailotp"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollment"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/migrations"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/oidc"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/session"
)

// Server adalah dependensi yang dibutuhkan router.
type Server struct {
	Config     ServerConfig
	DB         *database.DB
	Migrations migrations.Status
	Logger     *slog.Logger

	// Komponen autentikasi dan otorisasi (FL-011). Boleh nil pada pengujian
	// yang hanya memeriksa routing atau kesehatan; dalam keadaan itu endpoint
	// autentikasi menjawab "layanan tidak tersedia" alih-alih panik.
	Accounts *auth.Store
	Sessions *session.Store
	Google   *oidc.Verifier
	EmailOTP *emailotp.Service
	Audit    *audit.Writer
	Scope    authz.Directory
	Clock    clock.Clock

	// Explicit injection only. The normal server never enables synthetic enrollment.
	Enrollment *enrollment.Service
}

// authSiap melaporkan apakah komponen autentikasi tersedia. Pemeriksaan ini
// membuat konfigurasi yang belum lengkap berperilaku jelas (menolak melayani)
// tanpa melumpuhkan endpoint lokal lain seperti /healthz (SECURITY_PRIVACY §5e).
func (s Server) authSiap() bool {
	return s.Accounts != nil && s.Sessions != nil && s.Scope != nil
}

func (s Server) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

// waktuSekarang mengembalikan waktu dari jam server yang disuntikkan. Seluruh
// perhitungan masa berlaku pada lapisan HTTP memakai sumber ini supaya kebijakan
// sesi dan respons memakai satu acuan waktu yang sama — termasuk pada pengujian
// dengan jam palsu.
func (s Server) waktuSekarang() time.Time {
	if s.Clock == nil {
		return time.Now().UTC()
	}
	return s.Clock.Now().UTC()
}

// ServerConfig memuat nilai yang dipakai lapisan HTTP tanpa menarik paket
// konfigurasi, agar pengujian router tidak perlu menyentuh lingkungan proses.
type ServerConfig struct {
	Version        string
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	AIEngineURL    string
	MaxBodyBytes   int64
	RequestTimeout time.Duration
}

// DefaultMaxBodyBytes membatasi ukuran badan permintaan. Kontrak API §1.1 butir
// 10 mewajibkan batas ukuran untuk setiap endpoint unggahan.
const DefaultMaxBodyBytes = 1 << 20

// DefaultRequestTimeout membatasi satu permintaan; kontrak API §1.1 butir 10
// mewajibkan batas waktu proses agar satu permintaan lambat tidak menahan
// koneksi.
const DefaultRequestTimeout = 15 * time.Second

var requestCounter atomic.Uint64

func nextRequestID() string {
	return "req-" + time.Now().UTC().Format("20060102T150405.000000000") + "-" + itoa(int(requestCounter.Add(1)))
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func normalizePath(p string) string {
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		return "/" + p
	}
	return p
}

// statusWriter mencatat kode status dan apakah respons sudah benar-benar
// dikomit ke klien, tanpa mengubah perilaku ResponseWriter.
type statusWriter struct {
	http.ResponseWriter
	code  int
	wrote bool
}

func (w *statusWriter) WriteHeader(code int) {
	w.code = code
	if !w.wrote {
		w.wrote = true
		w.ResponseWriter.WriteHeader(code)
	}
}

// writeHeader hanya ada untuk memenuhi antarmuka yang dipakai writeJSON.
func (w *statusWriter) writeHeader(code int, final bool) bool {
	w.WriteHeader(code)
	return true
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.code == 0 {
		w.code = http.StatusOK
	}
	w.wrote = true
	return w.ResponseWriter.Write(b)
}

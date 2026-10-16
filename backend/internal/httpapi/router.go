package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/logging"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter memasang rute MVP. Urutan middleware ditentukan satu kali di chain.
func NewRouter(s Server) http.Handler {
	r := chi.NewRouter()

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/healthz", s.handleHealthz)
		r.Get("/readyz", s.handleReadyz)

		// --- Autentikasi (FL-011) ------------------------------------------
		// Penerbitan nonce sekali pakai untuk satu percobaan Google Sign-In.
		// Endpoint ini publik dan tidak menerima rahasia apa pun; yang dibatasi
		// pada tahap berikutnya adalah laju permintaannya (§10 kontrak).
		r.Post("/auth/nonce", s.handleAuthNonce)
		// Pertukaran identitas Google bersifat publik dengan batas laju pada
		// tahap berikutnya; bukti identitasnya adalah ID token Google.
		r.Post("/auth/google", s.handleAuthGoogle)
		// Penyegaran diautentikasi oleh token penyegar itu sendiri.
		r.Post("/auth/refresh", s.handleAuthRefresh)

		// Logout idempoten: panggilan kedua dengan sesi yang sudah dicabut
		// tetap menjawab 204 (docs/API_CONTRACT.md §5.1).
		r.Group(func(r chi.Router) {
			r.Use(requireAuthIdempoten(s))
			r.Post("/auth/logout", s.handleAuthLogout)
		})

		r.Group(func(r chi.Router) {
			r.Use(requireAuth(s))
			r.Get("/me", s.handleMe)
			r.Get("/me/classes", s.handleMeClasses)

			// Penugasan dosen dapat dibaca admin dan dosen yang ditugaskan;
			// kebijakan cakupan diperiksa di handler.
			r.Get("/classes/{class_id}/dosen", s.handleListClassLecturers)

			// --- Administrasi akun, sesi, dan peserta: khusus admin --------
			r.Group(func(r chi.Router) {
				r.Use(DenganPeranAdmin(s))
				r.Get("/users", s.handleListUsers)
				r.Post("/users", s.handleInviteUser)
				r.Patch("/users/{user_id}", s.handlePatchUser)
				r.Get("/auth/sessions", s.handleListSessions)
				r.Delete("/auth/sessions/{session_id}", s.handleRevokeSession)
				r.Put("/people/{person_id}/account-link", s.handleLinkPersonAccount)
				r.Delete("/people/{person_id}/account-link", s.handleUnlinkPersonAccount)
				r.Put("/classes/{class_id}/dosen/{user_id}", s.handleAssignLecturer)
				r.Delete("/classes/{class_id}/dosen/{user_id}", s.handleRemoveLecturer)
			})
		})

		r.NotFound(notFoundJSON)
		r.MethodNotAllowed(methodNotAllowedJSON)
	})

	// Rute tidak dikenal tetap dijawab JSON, bukan halaman HTML bawaan Go.
	r.NotFound(notFoundJSON)

	return chain(s, r)
}

// chain memasang seluruh middleware pada handler. Fungsi ini adalah satu-satunya
// tempat urutan middleware ditentukan, sehingga urutan yang diuji pengujian
// selalu sama dengan urutan yang dijalankan produksi.
//
// Urutan dari luar ke dalam, beserta alasannya:
//
//  1. RequestID paling luar agar setiap catatan log dan audit memiliki penautan.
//  2. accessLog berada di luar timeout dan recoverer (FL-005S). Status yang
//     dicatat adalah status final yang benar-benar dikirim klien — termasuk 503
//     batas waktu dan 500 hasil pemulihan panic — dan barisnya ditulis saat
//     respons selesai, bukan setelah handler lambat berhenti. Sebelumnya
//     accessLog berada di dalam timeout sehingga respons 503 tercatat 200,
//     panic yang dipulihkan tidak tercatat sama sekali, dan baris log menunggu
//     handler selesai meskipun klien sudah menerima respons batas waktu.
//  3. recoverer berada di dalam accessLog agar panic yang ia pulihkan ditulis
//     lewat statusWriter yang dicatat accessLog. Panic pada handler dipulihkan
//     recoverer tambahan di dalam goroutine timeout (lihat timeout).
//  4. timeout, lalu bodyLimit.
func chain(s Server, next http.Handler) http.Handler {
	return middleware.RequestID(
		echoRequestID(
			accessLog(s)(
				recoverer(s)(
					timeout(s)(
						bodyLimit(s)(next))))))
}

// echoRequestID memastikan setiap respons memuat penautan permintaan, baik
// ketika klien sudah mengirim X-Request-Id maupun ketika Chi yang membuatnya
// (docs/API_CONTRACT.md §1.4 butir 1).
func echoRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := middleware.GetReqID(r.Context()); id != "" {
			w.Header().Set("X-Request-Id", id)
		}
		next.ServeHTTP(w, r)
	})
}

// recoverer menangkap panic di handler. Kontrak API §1.4 butir 3 mewajibkan
// panic tidak membocorkan jejak tumpukan kepada klien.
//
// Middleware ini dipasang di dalam accessLog (lihat chain) supaya respons 500
// yang dihasilkannya tetap tercatat dengan status yang benar, dan dipasang lagi
// di dalam goroutine timeout (lihat timeout) karena panic pada handler terjadi
// di goroutine yang berbeda dari pemanggil middleware.
//
// Isi panic tidak pernah masuk log apa adanya: nilai mentahnya dapat memuat
// token, surel, atau potongan badan permintaan (temuan FL-005S, D-036). Yang
// dicatat adalah jenis nilai, pesannya setelah disensor logging.PanicSummary,
// dan jejak tumpukan yang dipotong — cukup untuk diagnosis tanpa membocorkan
// rahasia.
func recoverer(s Server) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					s.logger().Error("panic pada handler",
						"request_id", middleware.GetReqID(r.Context()),
						"method", r.Method,
						"path", normalizePath(r.URL.Path),
						"panic", logging.PanicSummary(rec),
						"stack", logging.StackTrace(),
					)
					writeJSON(w, http.StatusInternalServerError, errorPayload{
						Code:      "INTERNAL_ERROR",
						Message:   "Terjadi kesalahan internal.",
						RequestID: middleware.GetReqID(r.Context()),
					})
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// timeout membatasi durasi satu permintaan. Bila batas tercapai, klien menerima
// 503 PROCESSING_TIMEOUT alih-alih menunggu tanpa batas.
//
// Handler berjalan pada goroutine tersendiri supaya middleware dapat menjawab
// batas waktu tanpa menunggu pekerjaan handler selesai. Konsekuensinya dua hal
// yang harus dijaga di sini:
//
//  1. Penulisan respons dari kedua goroutine diserialisasi oleh timeoutWriter
//     (lihat payload.go); middleware tidak pernah menulis langsung ke
//     ResponseWriter mentah.
//  2. Recoverer dipasang pada rantai di dalam goroutine, karena panic handler
//     tidak berada pada tumpukan pemanggil middleware sehingga recoverer di
//     lapisan luar tidak dapat menangkapnya (proses backend akan berhenti).
func timeout(s Server) func(http.Handler) http.Handler {
	d := s.Config.RequestTimeout
	if d <= 0 {
		d = DefaultRequestTimeout
	}
	return func(next http.Handler) http.Handler {
		guarded := recoverer(s)(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()

			done := make(chan struct{})
			tw := &timeoutWriter{ResponseWriter: w, header: http.Header{}}

			go func() {
				defer close(done)
				guarded.ServeHTTP(tw, r.WithContext(ctx))
			}()

			select {
			case <-done:
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					// Batas waktu hanya menjawab bila handler belum mengirim
					// respons apa pun; respons yang sudah dimulai tidak ditimpa.
					tw.timeoutResponse(http.StatusServiceUnavailable, errorPayload{
						Code:      "PROCESSING_TIMEOUT",
						Message:   "Permintaan melebihi batas waktu proses.",
						RequestID: middleware.GetReqID(r.Context()),
					})
				} else {
					// Klien membatalkan permintaan: tutup writer supaya handler
					// yang masih berjalan tidak menulis setelah middleware ini
					// selesai.
					tw.seal()
				}
			}
		})
	}
}

// bodyLimit menolak badan permintaan yang melebihi batas ukuran sebelum dibaca
// seluruhnya (docs/API_CONTRACT.md §1.1 butir 10).
func bodyLimit(s Server) func(http.Handler) http.Handler {
	limit := s.Config.MaxBodyBytes
	if limit <= 0 {
		limit = DefaultMaxBodyBytes
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}
			if r.ContentLength > limit {
				// Ditolak sebelum handler melihat badan permintaan: koneksi ditutup
				// agar sisa kiriman tidak menahan sumber daya
				// (docs/API_CONTRACT.md §1.1 butir 10).
				w.Header().Set("Connection", "close")
				writeJSON(w, http.StatusRequestEntityTooLarge, errorPayload{
					Code:      "PAYLOAD_TOO_LARGE",
					Message:   "Ukuran permintaan melebihi batas yang diizinkan.",
					RequestID: middleware.GetReqID(r.Context()),
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// accessLog mencatat satu baris ringkas per permintaan. Nilai yang dicatat
// dibatasi pada metode, jalur, status, durasi, dan request id; parameter kueri
// tidak dicatat agar data peserta tidak masuk log (AGENTS.md §3).
//
// Middleware ini dipasang paling luar sesudah RequestID (lihat chain) supaya
// status yang dicatat adalah status final yang benar-benar dikirim klien —
// termasuk 503 batas waktu dan 500 hasil pemulihan panic — dan supaya barisnya
// ditulis saat respons selesai, bukan setelah handler lambat berhenti
// (FL-005S).
func accessLog(s Server) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w}
			next.ServeHTTP(sw, r)

			if sw.code == 0 {
				sw.code = http.StatusOK
			}
			s.logger().Info("permintaan http",
				"request_id", middleware.GetReqID(r.Context()),
				"method", r.Method,
				"path", normalizePath(r.URL.Path),
				"status", sw.code,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}

// handleHealthz menjawab liveness. Kontrak API §5.12 menegaskan badan respons
// tidak boleh memuat jalur berkas, versi pustaka internal, nama basis data, atau
// status perangkat.
func (s Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, healthzPayload{
		Status:  "ok",
		Version: s.Config.Version,
		Time:    time.Now().UTC().Format(time.RFC3339),
	})
}

// handleReadyz menjawab kesiapan. Pembacaan harus ringan: PRAGMA
// integrity_check penuh tidak dijalankan di sini (docs/API_CONTRACT.md §5.12).
func (s Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	payload := readyzPayload{
		Database:   "unavailable",
		AIEngine:   "unavailable",
		Migrations: "unknown",
		WAL:        walPayload{JournalMode: "unknown"},
	}

	code := http.StatusServiceUnavailable
	var failure string

	if s.DB != nil {
		if err := s.DB.Ping(ctx); err != nil {
			failure = "DATABASE_BUSY"
		} else {
			payload.Database = "ok"
			mode, err := s.DB.JournalMode(ctx)
			if err != nil {
				failure = "DATABASE_BUSY"
			} else {
				payload.WAL.JournalMode = mode
				if mode != database.JournalModeWAL {
					failure = "DATABASE_BUSY"
					payload.Database = "degraded"
				}
			}
		}
	} else {
		failure = "DATABASE_BUSY"
	}

	if s.Migrations.Available > 0 || s.Migrations.CurrentVersion > 0 {
		if s.Migrations.AppliedPending() {
			payload.Migrations = "pending"
			if failure == "" {
				failure = "MIGRATIONS_PENDING"
			}
		} else {
			payload.Migrations = "applied"
		}
	}

	if s.Config.AIEngineURL == "" {
		payload.AIEngine = "unavailable"
	} else if err := probeAIEngine(ctx, s.Config.AIEngineURL); err != nil {
		payload.AIEngine = "unavailable"
		// Mesin AI yang belum siap tidak menggagalkan kesiapan backend:
		// absensi manual harus tetap berjalan (docs/SECURITY_PRIVACY.md §5e).
	} else {
		payload.AIEngine = "ok"
	}

	if payload.Database == "ok" && !s.Migrations.AppliedPending() {
		code = http.StatusOK
	}

	if code != http.StatusOK {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(code)
		writeJSONBody(w, readyzError{
			Code:      failure,
			Message:   readinessMessage(failure),
			RequestID: middleware.GetReqID(r.Context()),
			Details:   payload,
		})
		return
	}

	writeJSON(w, http.StatusOK, readyzPayload{
		Status:     "ready",
		Database:   payload.Database,
		AIEngine:   payload.AIEngine,
		Migrations: payload.Migrations,
		WAL:        payload.WAL,
	})
}

func readinessMessage(code string) string {
	switch code {
	case "DATABASE_BUSY":
		return "Basis data belum dapat melayani permintaan."
	case "AI_ENGINE_UNAVAILABLE":
		return "Mesin AI lokal belum tersedia; absensi manual tetap dapat dilakukan."
	case "MIGRATIONS_PENDING":
		return "Migrasi skema belum diterapkan."
	default:
		return "Layanan belum siap melayani permintaan."
	}
}

func probeAIEngine(ctx context.Context, baseURL string) error {
	url := baseURL
	if url[len(url)-1] != '/' {
		url += "/"
	}
	url += "healthz"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := (&http.Client{Timeout: 2 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("mesin AI menjawab status %d", resp.StatusCode)
	}
	return nil
}

func notFoundJSON(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotFound, errorPayload{
		Code:      "NOT_FOUND",
		Message:   "Sumber daya tidak ditemukan.",
		RequestID: middleware.GetReqID(r.Context()),
	})
}

func methodNotAllowedJSON(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusMethodNotAllowed, errorPayload{
		Code:      "METHOD_NOT_ALLOWED",
		Message:   "Metode HTTP tidak diizinkan untuk jalur ini.",
		RequestID: middleware.GetReqID(r.Context()),
	})
}

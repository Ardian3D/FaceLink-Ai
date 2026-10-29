package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/audit"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/auth"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/authz"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/logging"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/session"
	"github.com/go-chi/chi/v5/middleware"
)

// principalKey adalah kunci context untuk principal yang sudah terautentikasi.
type principalKey struct{}

// withPrincipal menaruh principal pada context permintaan.
func withPrincipal(ctx context.Context, p authz.Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom mengambil principal terautentikasi dari context. Ada hanya
// setelah middleware autentikasi berhasil.
func PrincipalFrom(ctx context.Context) (authz.Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(authz.Principal)
	return p, ok
}

// bearerToken mengambil token akses dari tajuk Authorization. Token tidak
// pernah diambil dari kueri atau badan permintaan (tidak boleh masuk URL/log).
func bearerToken(r *http.Request) (string, bool) {
	tajuk := strings.TrimSpace(r.Header.Get("Authorization"))
	if tajuk == "" {
		return "", false
	}
	bagian := strings.Fields(tajuk)
	if len(bagian) != 2 || !strings.EqualFold(bagian[0], "Bearer") {
		return "", false
	}
	token := strings.TrimSpace(bagian[1])
	if token == "" {
		return "", false
	}
	return token, true
}

// authenticate memvalidasi token akses lalu memuat keadaan akun **terbaru** dari
// database. Role dan status tidak pernah diambil dari token: perubahan role dan
// penonaktifan akun berlaku pada permintaan berikutnya (D-033, D-037).
func (s Server) authenticate(ctx context.Context, r *http.Request) (authz.Principal, session.Session, error) {
	token, ok := bearerToken(r)
	if !ok {
		return authz.Principal{}, session.Session{}, errAuth{http.StatusUnauthorized, codeUnauthenticated}
	}

	sess, err := s.Sessions.Authenticate(ctx, token)
	switch {
	case errors.Is(err, session.ErrAccessExpired):
		return authz.Principal{}, session.Session{}, errAuth{http.StatusUnauthorized, codeTokenExpired}
	case errors.Is(err, session.ErrSessionRevoked):
		return authz.Principal{}, session.Session{}, errAuth{http.StatusUnauthorized, codeSessionRevoked}
	case err != nil:
		return authz.Principal{}, session.Session{}, errAuth{http.StatusUnauthorized, codeUnauthenticated}
	}

	user, err := s.Accounts.UserByID(ctx, sess.UserID)
	if err != nil {
		// Sesi yang menunjuk akun tak dikenal diperlakukan sebagai tidak sah,
		// bukan sebagai kesalahan server.
		return authz.Principal{}, session.Session{}, errAuth{http.StatusUnauthorized, codeUnauthenticated}
	}
	if user.Status != auth.StatusActive {
		return authz.Principal{}, session.Session{}, errAuth{http.StatusUnauthorized, codeSessionRevoked}
	}

	principal := authz.Principal{UserID: user.ID, Role: user.Role}
	if user.Role == auth.RoleMahasiswa {
		if personID, ok, err := s.Scope.PersonForUser(ctx, user.ID); err == nil && ok {
			principal.PersonID = personID
		}
	}

	// Jejak terakhir dipakai tidak wajib berhasil; kegagalannya tidak boleh
	// menolak permintaan yang sudah sah.
	_ = s.Sessions.Touch(ctx, sess.ID)
	return principal, sess, nil
}

// requireAuth menolak permintaan tanpa token akses sah dan menaruh principal
// pada context untuk handler berikutnya.
func requireAuth(s Server) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !s.authSiap() {
				writeErrorCode(w, r, http.StatusServiceUnavailable, codeAuthUnavailable, "Layanan autentikasi belum dikonfigurasi pada backend ini.")
				return
			}
			principal, _, err := s.authenticate(r.Context(), r)
			if err != nil {
				s.catatPenolakan(r, kodePenolakan(err))
				writeAuthFailure(w, r, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(withPrincipal(r.Context(), principal)))
		})
	}
}

// requireRole membatasi endpoint pada peran tertentu. Deny-by-default: peran di
// luar daftar ditolak dengan 403 FORBIDDEN_ROLE, dan penolakan pada jalur
// administratif dicatat ke audit (D-036: `authz.denied`).
func requireRole(s Server, roles ...auth.Role) func(http.Handler) http.Handler {
	diizinkan := make(map[auth.Role]bool, len(roles))
	for _, peran := range roles {
		diizinkan[peran] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := PrincipalFrom(r.Context())
			if !ok {
				writeErrorCode(w, r, http.StatusUnauthorized, codeUnauthenticated, pesanGalatAuth(codeUnauthenticated))
				return
			}
			if !diizinkan[principal.Role] {
				s.catatAudit(r, audit.Event{
					ActorType: audit.ActorUser, ActorID: principal.UserID, Action: "authz.denied",
					Result: audit.ResultFailure,
					Reason: "Peran tidak berwenang pada jalur administratif",
					Metadata: map[string]string{
						"reason_code": "forbidden_role",
						"method":      r.Method,
						"path":        normalizePath(r.URL.Path),
						"role_before": string(principal.Role),
					},
				})
				writeErrorCode(w, r, http.StatusForbidden, codeForbiddenRole, pesanGalatAuth(codeForbiddenRole))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// DenganPeranAdmin adalah pintasan middleware untuk endpoint khusus admin.
func DenganPeranAdmin(s Server) func(http.Handler) http.Handler {
	return requireRole(s, auth.RoleAdmin)
}

// DenganPeranAdminDosen membatasi endpoint pada admin atau dosen. Cakupan
// dosen tetap diperiksa di handler (mis. kelas yang ditugaskan).
func DenganPeranAdminDosen(s Server) func(http.Handler) http.Handler {
	return requireRole(s, auth.RoleAdmin, auth.RoleDosen)
}

// catatPenolakan menulis audit penolakan otentikasi/otorisasi. Jalur ini tidak
// pernah memuat token, email, atau isi tajuk permintaan.
func (s Server) catatPenolakan(r *http.Request, kode string) {
	s.catatAudit(r, audit.Event{
		ActorType: audit.ActorSystem,
		Action:    "auth.login.denied",
		Result:    audit.ResultFailure,
		Reason:    "Permintaan ditolak sebelum handler dijalankan",
		Metadata: map[string]string{
			"reason_code": kode,
			"method":      r.Method,
			"path":        normalizePath(r.URL.Path),
		},
	})
}

// catatAudit menulis satu peristiwa audit dan melaporkan kegagalannya lewat log
// internal tanpa pernah menggagalkan permintaan yang sudah sah.
func (s Server) catatAudit(r *http.Request, e audit.Event) {
	if s.Audit == nil {
		return
	}
	if e.RequestID == "" {
		e.RequestID = middleware.GetReqID(r.Context())
	}
	if err := s.Audit.Record(r.Context(), e); err != nil {
		s.logger().Error("gagal menulis audit",
			"request_id", middleware.GetReqID(r.Context()),
			"action", e.Action,
			"error", logging.Redact(err.Error()),
		)
	}
}

// kodePenolakan mengembalikan kode galat auth atau kode umum bila galatnya
// bukan penolakan autentikasi.
func kodePenolakan(err error) string {
	var ea errAuth
	if errors.As(err, &ea) {
		return ea.code
	}
	return codeUnauthenticated
}

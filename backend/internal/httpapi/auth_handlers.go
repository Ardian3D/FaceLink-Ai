package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/audit"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/auth"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/logging"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/oidc"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/session"
	"github.com/go-chi/chi/v5/middleware"
)

// masaBerlakuNonce adalah masa berlaku satu nonce OIDC. Cukup panjang untuk
// menyelesaikan alur Google Sign-In, cukup pendek untuk tidak menumpuk baris
// (D-033: nonce sekali pakai).
const masaBerlakuNonce = 5 * time.Minute

// sesiResponse adalah bentuk respons login dan penyegaran (kontrak §5.1).
type sesiResponse struct {
	AccessToken  string       `json:"access_token"`
	ExpiresIn    int          `json:"expires_in"`
	RefreshToken string       `json:"refresh_token"`
	Session      sesiRingkas  `json:"session"`
	User         penggunaRing `json:"user"`
}

type sesiRingkas struct {
	ID                string `json:"id"`
	AccessExpiresAt   string `json:"access_expires_at"`
	AbsoluteExpiresAt string `json:"absolute_expires_at"`
}

// penggunaRing adalah identitas pemanggil yang aman dikirim ke klien. Tidak ada
// hash, identitas penyedia, atau token di dalamnya.
type penggunaRing struct {
	ID          string   `json:"id"`
	Email       string   `json:"email,omitempty"`
	DisplayName string   `json:"display_name"`
	Role        string   `json:"role"`
	Status      string   `json:"status"`
	PersonID    *string  `json:"person_id"`
	ClassIDs    []string `json:"class_ids"`
}

// handleAuthNonce menerbitkan nonce sekali pakai untuk satu percobaan Google
// Sign-In (D-033, kontrak §5.1).
//
// Aplikasi memanggil endpoint ini SEBELUM membuka alur Google, memakai nonce
// yang dikembalikan sebagai parameter `nonce` pada permintaan otorisasi Google,
// lalu mengirimkannya kembali bersama ID token ke `POST /auth/google`. Karena
// nonce hanya sah bila terdaftar di backend, token yang bocor tidak dapat
// dipakai ulang di luar percobaan login yang benar-benar dimulai aplikasi —
// itulah pengikatan yang dituntut kontrak, dan pengikatan itu tidak dapat
// dipenuhi bila klien mendaftarkan sendiri nonce-nya setelah login.
func (s Server) handleAuthNonce(w http.ResponseWriter, r *http.Request) {
	if !s.authSiap() {
		writeErrorCode(w, r, http.StatusServiceUnavailable, codeAuthUnavailable, pesanGalatAuth(codeAuthUnavailable))
		return
	}

	nonce, err := auth.NonceBaru()
	if err != nil {
		s.logger().Error("gagal membuat nonce", "request_id", reqID(r), "error", logging.Redact(err.Error()))
		writeErrorCode(w, r, http.StatusInternalServerError, codeInternalError, "Terjadi kesalahan internal.")
		return
	}
	if err := s.Accounts.StoreNonce(r.Context(), nonce); err != nil {
		s.logger().Error("gagal menyimpan nonce", "request_id", reqID(r), "error", logging.Redact(err.Error()))
		writeErrorCode(w, r, http.StatusInternalServerError, codeInternalError, "Terjadi kesalahan internal.")
		return
	}

	writeData(w, r, http.StatusOK, map[string]any{
		"nonce":      nonce,
		"expires_in": int(auth.TTLNonce.Seconds()),
	})
}

// handleAuthGoogle menukar ID token Google menjadi sesi aplikasi.
//
// Alur: verifikasi OIDC (kode/signature/iss/aud/exp/sub/email_verified) →
// pencocokan nonce pada token → pemakaian nonce sekali pakai → pencarian
// identitas Google → penautan akun undangan bila belum tertaut → penerbitan
// sesi. Role tidak pernah dibaca dari klien maupun dari klaim Google.
func (s Server) handleAuthGoogle(w http.ResponseWriter, r *http.Request) {
	if s.Google == nil {
		writeErrorCode(w, r, http.StatusServiceUnavailable, codeGoogleUnavailable, pesanGalatAuth(codeGoogleUnavailable))
		return
	}

	var req struct {
		IDToken string `json:"id_token"`
		Nonce   string `json:"nonce"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeErrorCode(w, r, http.StatusBadRequest, codeValidationFailed, "Badan permintaan tidak dapat dibaca.")
		return
	}
	if strings.TrimSpace(req.IDToken) == "" {
		writeErrorCode(w, r, http.StatusBadRequest, codeMissingField, "id_token wajib diisi.")
		return
	}
	if strings.TrimSpace(req.Nonce) == "" {
		// Nonce wajib: nilainya harus ikut dikirim ke Google oleh aplikasi
		// sehingga klaim `nonce` pada ID token terikat pada percobaan login ini
		// (D-033, docs/API_CONTRACT.md §5.1).
		writeErrorCode(w, r, http.StatusBadRequest, codeMissingField, "nonce wajib diisi dan harus dikirim juga ke Google.")
		return
	}

	claims, err := s.Google.Verify(r.Context(), req.IDToken)
	if err != nil {
		s.catatPenolakan(r, alasanVerifikasi(err))
		writeErrorCode(w, r, http.StatusUnauthorized, codeIdentityTokenInvalid, pesanGalatAuth(codeIdentityTokenInvalid))
		return
	}
	if err := claims.CheckNonce(req.Nonce); err != nil {
		s.catatPenolakan(r, "nonce_mismatch")
		writeErrorCode(w, r, http.StatusUnauthorized, codeIdentityTokenInvalid, pesanGalatAuth(codeIdentityTokenInvalid))
		return
	}

	// Nonce baru dikonsumsi setelah ID token terbukti sah dan klaim nonce-nya
	// cocok, tetapi tetap sebelum akun diselesaikan dan sesi diterbitkan. Dua
	// sifat yang dituntut kontrak §5.1 dijaga di sini:
	//
	//  1. Nonce harus terdaftar — dibuat lewat `POST /auth/nonce` oleh aplikasi
	//     yang akan memulai alur Google. Nonce yang tidak terdaftar, sudah
	//     dipakai, atau sudah kedaluwarsa dijawab `401` dan tidak pernah
	//     dihidupkan kembali. Tanpa aturan ini, ID token yang bocor dapat
	//     diputar ulang setelah TTL nonce lewat (temuan audit K-01).
	//  2. Pemakaian hanya sekali, ditentukan satu pernyataan DELETE bersyarat di
	//     dalam database, sehingga dua permintaan paralel dengan nonce yang sama
	//     tidak dapat sama-sama lolos (temuan audit K-02).
	//
	// Urutan ini juga berarti ID token yang tidak sah tidak "membakar" nonce
	// yang sah: percobaan ulang dengan token yang benar masih dapat memakai
	// nonce yang sama selama masa berlakunya.
	if err := s.Accounts.ConsumeNonce(r.Context(), req.Nonce); err != nil {
		if errors.Is(err, auth.ErrNonceNotFound) {
			s.catatPenolakan(r, "nonce_replay")
			writeErrorCode(w, r, http.StatusUnauthorized, codeIdentityTokenInvalid, pesanGalatAuth(codeIdentityTokenInvalid))
			return
		}
		if errors.Is(err, auth.ErrInvalidInput) {
			writeErrorCode(w, r, http.StatusBadRequest, codeMissingField, "nonce wajib diisi dan harus dikirim juga ke Google.")
			return
		}
		s.logger().Error("gagal memakai nonce", "request_id", reqID(r), "error", logging.Redact(err.Error()))
		writeErrorCode(w, r, http.StatusInternalServerError, codeInternalError, "Terjadi kesalahan internal.")
		return
	}

	user, baruTertaut, err := s.resolveAccount(r, claims.Subject, claims.Email)
	if err != nil {
		var ea errAuth
		if errors.As(err, &ea) {
			s.catatPenolakan(r, ea.code)
			writeErrorCode(w, r, ea.status, ea.code, pesanGalatAuth(ea.code))
			return
		}
		s.logger().Error("gagal menyelesaikan akun", "request_id", reqID(r), "error", logging.Redact(err.Error()))
		writeErrorCode(w, r, http.StatusInternalServerError, codeInternalError, "Terjadi kesalahan internal.")
		return
	}

	pair, err := s.Sessions.Issue(r.Context(), user.ID, ringkasUserAgent(r.UserAgent()))
	if err != nil {
		s.logger().Error("gagal menerbitkan sesi", "request_id", reqID(r), "error", logging.Redact(err.Error()))
		writeErrorCode(w, r, http.StatusInternalServerError, codeInternalError, "Terjadi kesalahan internal.")
		return
	}

	_ = s.Accounts.TouchLogin(r.Context(), user.ID)

	if baruTertaut {
		s.catatAudit(r, audit.Event{
			ActorType:  audit.ActorUser,
			ActorID:    user.ID,
			Action:     "auth.google.linked",
			EntityType: "user",
			EntityID:   user.ID,
			Result:     audit.ResultSuccess,
			Reason:     "Identitas Google ditautkan pada login pertama akun undangan",
			Metadata:   map[string]string{"provider": "google", "role_after": string(user.Role)},
		})
	}
	s.catatAudit(r, audit.Event{
		ActorType:  audit.ActorUser,
		ActorID:    user.ID,
		Action:     "auth.google.succeeded",
		EntityType: "session_token",
		EntityID:   pair.SessionID,
		Result:     audit.ResultSuccess,
		Metadata:   map[string]string{"provider": "google", "session_id": pair.SessionID},
	})

	s.writeSesiResponse(w, r, user, pair)
}

// resolveAccount menentukan akun internal untuk identitas Google yang sudah
// diverifikasi: memakai tautan yang ada, atau menautkan akun undangan.
func (s Server) resolveAccount(r *http.Request, subject, email string) (auth.User, bool, error) {
	user, ada, err := s.Accounts.UserBySubject(r.Context(), subject)
	if err != nil {
		return auth.User{}, false, err
	}
	if ada {
		switch user.Status {
		case auth.StatusActive:
			return user, false, nil
		case auth.StatusSuspended:
			s.catatAudit(r, audit.Event{
				ActorType: audit.ActorUser, ActorID: user.ID, Action: "auth.login.denied",
				EntityType: "user", EntityID: user.ID, Result: audit.ResultFailure,
				Reason:   "Akun dinonaktifkan",
				Metadata: map[string]string{"reason_code": "suspended"},
			})
			return auth.User{}, false, errAuth{http.StatusForbidden, codeIdentityNotLinked}
		default:
			// Status `invited` pada akun yang identitasnya sudah tertaut berarti
			// admin mengembalikan akun itu ke undangan (mis. setelah identitas
			// salah taut). Sesi baru tidak boleh diterbitkan: middleware akan
			// menolaknya pada permintaan pertama, sehingga menerbitkannya hanya
			// menghasilkan sesi yang tidak dapat dipakai (kontrak §5.1).
			s.catatAudit(r, audit.Event{
				ActorType: audit.ActorUser, ActorID: user.ID, Action: "auth.login.denied",
				EntityType: "user", EntityID: user.ID, Result: audit.ResultFailure,
				Reason:   "Akun belum aktif",
				Metadata: map[string]string{"reason_code": "not_active"},
			})
			return auth.User{}, false, errAuth{http.StatusForbidden, codeIdentityNotLinked}
		}
	}

	user, err = s.Accounts.LinkInvitedAccount(r.Context(), subject, email, true)
	switch {
	case errors.Is(err, auth.ErrNotInvited):
		s.catatDeniedAkun(r, "no_account")
		return auth.User{}, false, errAuth{http.StatusForbidden, codeIdentityNotLinked}
	case errors.Is(err, auth.ErrIdentityTaken), errors.Is(err, auth.ErrAlreadyLinked):
		s.catatDeniedAkun(r, "identity_taken")
		return auth.User{}, false, errAuth{http.StatusForbidden, codeIdentityNotLinked}
	case err != nil:
		return auth.User{}, false, err
	}
	return user, true, nil
}

// handleAuthRefresh memutar token penyegar. Pemakaian ulang token yang sudah
// dirotasi mencabut sesi dan dicatat, bukan hanya ditolak.
func (s Server) handleAuthRefresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeErrorCode(w, r, http.StatusBadRequest, codeValidationFailed, "Badan permintaan tidak dapat dibaca.")
		return
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
		writeErrorCode(w, r, http.StatusBadRequest, codeMissingField, "refresh_token wajib diisi.")
		return
	}

	pair, err := s.Sessions.Refresh(r.Context(), req.RefreshToken)
	switch {
	case errors.Is(err, session.ErrRefreshReused):
		s.catatAudit(r, audit.Event{
			ActorType: audit.ActorSystem, Action: "auth.session.reuse_detected",
			EntityType: "session_token", EntityID: pair.SessionID, Result: audit.ResultFailure,
			Reason:   "Token penyegar yang sudah dirotasi dipakai lagi; seluruh sesi pengguna dicabut",
			Metadata: map[string]string{"reason_code": "refresh_reuse"},
		})
		writeErrorCode(w, r, http.StatusUnauthorized, codeSessionRevoked, pesanGalatAuth(codeSessionRevoked))
		return
	case errors.Is(err, session.ErrRefreshInvalid), errors.Is(err, session.ErrNotFound), errors.Is(err, session.ErrUnauthenticated):
		writeErrorCode(w, r, http.StatusUnauthorized, codeUnauthenticated, pesanGalatAuth(codeUnauthenticated))
		return
	case err != nil:
		s.logger().Error("gagal memutar sesi", "request_id", reqID(r), "error", logging.Redact(err.Error()))
		writeErrorCode(w, r, http.StatusInternalServerError, codeInternalError, "Terjadi kesalahan internal.")
		return
	}

	// Sesi yang baru diputar dipakai untuk memastikan akun masih aktif sebelum
	// token dikirim kembali; role dan status tidak pernah diambil dari token
	// penyegar.
	sessBaru, err := s.Sessions.Authenticate(r.Context(), pair.AccessToken)
	if err != nil {
		writeErrorCode(w, r, http.StatusUnauthorized, codeUnauthenticated, pesanGalatAuth(codeUnauthenticated))
		return
	}
	user, err := s.Accounts.UserByID(r.Context(), sessBaru.UserID)
	if err != nil {
		writeErrorCode(w, r, http.StatusUnauthorized, codeUnauthenticated, pesanGalatAuth(codeUnauthenticated))
		return
	}
	if user.Status != auth.StatusActive {
		_, _ = s.Sessions.RevokeAllForUser(r.Context(), user.ID, "status_tidak_aktif")
		writeErrorCode(w, r, http.StatusUnauthorized, codeSessionRevoked, pesanGalatAuth(codeSessionRevoked))
		return
	}

	s.catatAudit(r, audit.Event{
		ActorType: audit.ActorUser, ActorID: user.ID, Action: "auth.session.refreshed",
		EntityType: "session_token", EntityID: pair.SessionID, Result: audit.ResultSuccess,
		Metadata: map[string]string{"session_id": pair.SessionID},
	})

	// Kontrak §5.1: respons penyegaran hanya memuat pasangan token baru —
	// bentuk `user` maupun objek `session` tidak dikirim ulang, karena aplikasi
	// memakai `GET /me` bila memerlukan identitas atau cakupannya. Identitas
	// sesi tidak berubah oleh rotasi, sehingga klien yang sudah memegangnya
	// tidak kehilangan apa pun.
	writeData(w, r, http.StatusOK, map[string]any{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"expires_in":    s.sisaAkses(pair),
	})
}

// sisaAkses menghitung sisa masa berlaku token akses memakai jam server, bukan
// jam dinding proses: abstraksi jam yang sama dipakai seluruh kebijakan sesi,
// sehingga respons dan penerbitan token tidak pernah berbeda acuan.
func (s Server) sisaAkses(pair session.Pair) int {
	sisa := int(pair.AccessExpiresAt.Sub(s.waktuSekarang()).Seconds())
	if sisa < 0 {
		return 0
	}
	return sisa
}

// handleAuthLogout mencabut sesi yang sedang dipakai. Panggilan kedua tetap
// 204: sesi yang sudah tercabut diperlakukan sebagai sudah logout (kontrak §5.1).
func (s Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if _, dicabut := r.Context().Value(sesiDicabutKey{}).(bool); dicabut {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	sess, ok := SesiFrom(r.Context())
	if !ok {
		writeErrorCode(w, r, http.StatusUnauthorized, codeUnauthenticated, pesanGalatAuth(codeUnauthenticated))
		return
	}
	// Sesi dicabut hanya bila memang milik pemanggil: pencabutan sesi orang lain
	// bukan wewenang endpoint ini (itu jalur admin), dan batas kepemilikan itu
	// ditegakkan di lapisan store, bukan hanya oleh token yang dikirim.
	if _, err := s.Sessions.RevokeOwned(r.Context(), sess.ID, sess.UserID, s.waktuSekarang(), "logout"); err != nil {
		s.logger().Error("gagal mencabut sesi", "request_id", reqID(r), "error", logging.Redact(err.Error()))
		writeErrorCode(w, r, http.StatusInternalServerError, codeInternalError, "Terjadi kesalahan internal.")
		return
	}
	principal, _ := PrincipalFrom(r.Context())
	s.catatAudit(r, audit.Event{
		ActorType: audit.ActorUser, ActorID: principal.UserID, Action: "auth.logout",
		EntityType: "session_token", EntityID: sess.ID, Result: audit.ResultSuccess,
		Metadata: map[string]string{"session_id": sess.ID},
	})
	w.WriteHeader(http.StatusNoContent)
}

// handleMe mengembalikan identitas pemanggil beserta cakupan kelasnya. Nilai
// role dan status dibaca dari database pada permintaan ini, bukan dari token.
func (s Server) handleMe(w http.ResponseWriter, r *http.Request) {
	principal, ok := PrincipalFrom(r.Context())
	if !ok {
		writeErrorCode(w, r, http.StatusUnauthorized, codeUnauthenticated, pesanGalatAuth(codeUnauthenticated))
		return
	}
	user, err := s.Accounts.UserByID(r.Context(), principal.UserID)
	if err != nil {
		writeErrorCode(w, r, http.StatusUnauthorized, codeUnauthenticated, pesanGalatAuth(codeUnauthenticated))
		return
	}
	ring, err := s.penggunaRingkasUntuk(r, user)
	if err != nil {
		s.logger().Error("gagal membaca cakupan pengguna", "request_id", reqID(r), "error", logging.Redact(err.Error()))
		writeErrorCode(w, r, http.StatusInternalServerError, codeInternalError, "Terjadi kesalahan internal.")
		return
	}
	writeData(w, r, http.StatusOK, ring)
}

// writeSesiResponse menulis pasangan token beserta identitas pemanggil.
func (s Server) writeSesiResponse(w http.ResponseWriter, r *http.Request, user auth.User, pair session.Pair) {
	ring, err := s.penggunaRingkasUntuk(r, user)
	if err != nil {
		s.logger().Error("gagal membaca cakupan pengguna", "request_id", reqID(r), "error", logging.Redact(err.Error()))
		writeErrorCode(w, r, http.StatusInternalServerError, codeInternalError, "Terjadi kesalahan internal.")
		return
	}
	expiresIn := s.sisaAkses(pair)
	writeData(w, r, http.StatusOK, sesiResponse{
		AccessToken:  pair.AccessToken,
		ExpiresIn:    expiresIn,
		RefreshToken: pair.RefreshToken,
		Session: sesiRingkas{
			ID:                pair.SessionID,
			AccessExpiresAt:   audit.FormatWaktu(pair.AccessExpiresAt),
			AbsoluteExpiresAt: audit.FormatWaktu(pair.AbsoluteExpiresAt),
		},
		User: ring,
	})
}

// penggunaRingkasUntuk menyusun identitas ringkas beserta cakupan kelas:
// dosen memakai penugasan kelas, mahasiswa memakai kelas yang diikuti.
func (s Server) penggunaRingkasUntuk(r *http.Request, user auth.User) (penggunaRing, error) {
	ring := penggunaRing{
		ID:          user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Role:        string(user.Role),
		Status:      string(user.Status),
		ClassIDs:    []string{},
	}

	switch user.Role {
	case auth.RoleDosen:
		kelas, err := s.Accounts.ClassesForLecturer(r.Context(), user.ID)
		if err != nil {
			return penggunaRing{}, err
		}
		for _, k := range kelas {
			ring.ClassIDs = append(ring.ClassIDs, k.ID)
		}
	case auth.RoleMahasiswa:
		personID, ok, err := s.Accounts.PersonForUser(r.Context(), user.ID)
		if err != nil {
			return penggunaRing{}, err
		}
		if ok {
			ring.PersonID = &personID
			kelas, err := s.Accounts.ClassesForPerson(r.Context(), personID)
			if err != nil {
				return penggunaRing{}, err
			}
			for _, k := range kelas {
				ring.ClassIDs = append(ring.ClassIDs, k.ID)
			}
		}
	}
	return ring, nil
}

// catatDeniedAkun mencatat penolakan login tanpa menyimpan email atau subjek.
func (s Server) catatDeniedAkun(r *http.Request, alasan string) {
	s.catatAudit(r, audit.Event{
		ActorType: audit.ActorSystem,
		Action:    "auth.login.denied",
		Result:    audit.ResultFailure,
		Reason:    "Identitas Google tidak tertaut ke akun yang diundang",
		Metadata:  map[string]string{"reason_code": alasan, "provider": "google"},
	})
}

// alasanVerifikasi memetakan galat verifikasi OIDC ke kode alasan audit. Kode
// ini hanya masuk audit internal, bukan ke respons klien.
func alasanVerifikasi(err error) string {
	switch {
	case errors.Is(err, oidc.ErrTokenExpired):
		return "token_expired"
	case errors.Is(err, oidc.ErrSignatureInvalid):
		return "signature_invalid"
	case errors.Is(err, oidc.ErrIssuerInvalid):
		return "issuer_invalid"
	case errors.Is(err, oidc.ErrAudienceInvalid):
		return "audience_invalid"
	case errors.Is(err, oidc.ErrUnsupportedAlgorithm):
		return "algorithm_not_allowed"
	case errors.Is(err, oidc.ErrEmailNotVerified):
		return "email_not_verified"
	case errors.Is(err, oidc.ErrNonceMismatch):
		return "nonce_mismatch"
	case errors.Is(err, oidc.ErrJWKSUnavailable):
		return "jwks_unavailable"
	default:
		return "token_invalid"
	}
}

// ringkasUserAgent memendekkan user agent agar dapat disimpan tanpa memuat data
// pribadi yang panjang.
func ringkasUserAgent(ua string) string {
	const batas = 120
	ua = strings.TrimSpace(ua)
	if len(ua) > batas {
		return ua[:batas]
	}
	return ua
}

// reqID mengembalikan penautan permintaan untuk log internal.
func reqID(r *http.Request) string { return middleware.GetReqID(r.Context()) }

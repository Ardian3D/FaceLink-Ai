package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/audit"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/auth"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/authz"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/logging"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/session"
	"github.com/go-chi/chi/v5"
)

// akunRing adalah bentuk aman baris `users` untuk respons administrasi. Tidak
// ada hash, identitas penyedia, atau token di dalamnya.
type akunRing struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	LastLoginAt string `json:"last_login_at,omitempty"`
}

func akunKeRing(u auth.User) akunRing {
	ring := akunRing{
		ID:          u.ID,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		Role:        string(u.Role),
		Status:      string(u.Status),
		CreatedAt:   audit.FormatWaktu(u.CreatedAt),
	}
	if u.LastLoginAt != nil {
		ring.LastLoginAt = audit.FormatWaktu(*u.LastLoginAt)
	}
	return ring
}

// handleListUsers mengembalikan daftar akun (khusus admin).
func (s Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	peran := auth.Role(strings.TrimSpace(r.URL.Query().Get("role")))
	status := auth.Status(strings.TrimSpace(r.URL.Query().Get("status")))
	for _, nilai := range []string{string(peran), string(status)} {
		if nilai != "" && !peran.Valid() && !status.Valid() {
			writeErrorCode(w, r, http.StatusBadRequest, codeValidationFailed, "Filter peran atau status tidak dikenal.")
			return
		}
	}
	if peran != "" && !peran.Valid() {
		writeErrorCode(w, r, http.StatusBadRequest, codeValidationFailed, "Filter peran tidak dikenal.")
		return
	}
	if status != "" && !status.Valid() {
		writeErrorCode(w, r, http.StatusBadRequest, codeValidationFailed, "Filter status tidak dikenal.")
		return
	}

	daftar, err := s.Accounts.ListUsers(r.Context(), peran, status, 200)
	if err != nil {
		s.tulisGalatDomain(w, r, err)
		return
	}
	hasil := make([]akunRing, 0, len(daftar))
	for _, u := range daftar {
		hasil = append(hasil, akunKeRing(u))
	}
	writeData(w, r, http.StatusOK, hasil)
}

// handleInviteUser membuat akun berstatus `invited`. Role hanya ditetapkan di
// sini — tidak pernah dari Google atau dari klien lain (D-032, D-037).
func (s Server) handleInviteUser(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFrom(r.Context())

	var req struct {
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeErrorCode(w, r, http.StatusBadRequest, codeValidationFailed, "Badan permintaan tidak dapat dibaca.")
		return
	}
	if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.DisplayName) == "" || strings.TrimSpace(req.Role) == "" {
		writeErrorCode(w, r, http.StatusBadRequest, codeMissingField, "email, display_name, dan role wajib diisi.")
		return
	}

	user, err := s.Accounts.InviteUser(r.Context(), req.Email, req.DisplayName, auth.Role(req.Role), principal.UserID)
	if err != nil {
		s.tulisGalatDomain(w, r, err)
		return
	}
	s.catatAudit(r, audit.Event{
		ActorType: audit.ActorUser, ActorID: principal.UserID, Action: "user.invited",
		EntityType: "user", EntityID: user.ID, Result: audit.ResultSuccess,
		Reason:   "Akun diundang oleh admin",
		Metadata: map[string]string{"role_after": string(user.Role), "status_after": string(user.Status)},
	})
	writeData(w, r, http.StatusCreated, akunKeRing(user))
}

// handlePatchUser mengubah role, status, atau nama tampilan akun. Setiap
// perubahan hak akses diaudit beserta nilai sebelum dan sesudahnya.
func (s Server) handlePatchUser(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFrom(r.Context())
	userID := chi.URLParam(r, "user_id")

	var req struct {
		Role        *string `json:"role"`
		Status      *string `json:"status"`
		DisplayName *string `json:"display_name"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeErrorCode(w, r, http.StatusBadRequest, codeValidationFailed, "Badan permintaan tidak dapat dibaca.")
		return
	}
	if req.Role == nil && req.Status == nil && req.DisplayName == nil {
		writeErrorCode(w, r, http.StatusBadRequest, codeMissingField, "Minimal satu dari role, status, atau display_name harus diisi.")
		return
	}

	sebelum, err := s.Accounts.UserByID(r.Context(), userID)
	if err != nil {
		s.tulisGalatDomain(w, r, err)
		return
	}

	user := sebelum
	if req.DisplayName != nil {
		user, err = s.Accounts.SetDisplayName(r.Context(), principal.UserID, userID, *req.DisplayName)
		if err != nil {
			s.tulisGalatDomain(w, r, err)
			return
		}
	}
	if req.Role != nil {
		user, err = s.Accounts.SetRole(r.Context(), principal.UserID, userID, auth.Role(*req.Role))
		if err != nil {
			s.tulisGalatDomain(w, r, err)
			return
		}
		s.catatAudit(r, audit.Event{
			ActorType: audit.ActorUser, ActorID: principal.UserID, Action: "user.role_changed",
			EntityType: "user", EntityID: userID, Result: audit.ResultSuccess,
			Metadata: map[string]string{"role_before": string(sebelum.Role), "role_after": string(user.Role)},
		})
	}
	if req.Status != nil {
		user, err = s.Accounts.SetStatus(r.Context(), principal.UserID, userID, auth.Status(*req.Status))
		if err != nil {
			s.tulisGalatDomain(w, r, err)
			return
		}
		s.catatAudit(r, audit.Event{
			ActorType: audit.ActorUser, ActorID: principal.UserID, Action: "user.status_changed",
			EntityType: "user", EntityID: userID, Result: audit.ResultSuccess,
			Metadata: map[string]string{"status_before": string(sebelum.Status), "status_after": string(user.Status)},
		})
		if user.Status != auth.StatusActive {
			// Penonaktifan akun mencabut seluruh sesinya seketika (D-033).
			if jumlah, err := s.Sessions.RevokeAllForUser(r.Context(), userID, "akun_tidak_aktif"); err == nil && jumlah > 0 {
				s.catatAudit(r, audit.Event{
					ActorType: audit.ActorUser, ActorID: principal.UserID, Action: "auth.session.revoked",
					EntityType: "session_token", Result: audit.ResultSuccess,
					Reason:   "Seluruh sesi akun dicabut karena status akun berubah",
					Metadata: map[string]string{"count": itoa(int(jumlah))},
				})
			}
		}
	}
	writeData(w, r, http.StatusOK, akunKeRing(user))
}

// handleListSessions menampilkan sesi milik satu akun (khusus admin).
func (s Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userID == "" {
		writeErrorCode(w, r, http.StatusBadRequest, codeMissingField, "Parameter user_id wajib diisi.")
		return
	}
	akun, err := s.Accounts.UserByID(r.Context(), userID)
	if err != nil {
		s.tulisGalatDomain(w, r, err)
		return
	}
	daftar, err := s.Sessions.ListActiveForUser(r.Context(), userID)
	if err != nil {
		s.tulisGalatDomain(w, r, err)
		return
	}
	hasil := make([]sesiAdminRing, 0, len(daftar))
	for _, sesi := range daftar {
		ring := sesiAdminRing{
			ID:                sesi.ID,
			UserID:            sesi.UserID,
			Role:              string(akun.Role),
			CreatedAt:         audit.FormatWaktu(sesi.CreatedAt),
			AbsoluteExpiresAt: audit.FormatWaktu(sesi.AbsoluteExpiresAt),
			UserAgentRingkas:  sesi.UserAgentRingkas,
		}
		if sesi.LastSeenAt != nil {
			ring.LastSeenAt = audit.FormatWaktu(*sesi.LastSeenAt)
		}
		hasil = append(hasil, ring)
	}
	writeData(w, r, http.StatusOK, hasil)
}

type sesiAdminRing struct {
	ID                string `json:"id"`
	UserID            string `json:"user_id"`
	Role              string `json:"role"`
	CreatedAt         string `json:"created_at"`
	LastSeenAt        string `json:"last_seen_at,omitempty"`
	AbsoluteExpiresAt string `json:"absolute_expires_at"`
	UserAgentRingkas  string `json:"user_agent_ringkas,omitempty"`
}

// handleRevokeSession mencabut sesi mana pun (khusus admin).
func (s Server) handleRevokeSession(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFrom(r.Context())
	sessionID := chi.URLParam(r, "session_id")

	if err := s.Sessions.Revoke(r.Context(), sessionID, "admin_revoked"); err != nil {
		s.tulisGalatDomain(w, r, err)
		return
	}
	s.catatAudit(r, audit.Event{
		ActorType: audit.ActorUser, ActorID: principal.UserID, Action: "auth.session.revoked",
		EntityType: "session_token", EntityID: sessionID, Result: audit.ResultSuccess,
		Reason:   "Sesi dicabut oleh admin",
		Metadata: map[string]string{"session_id": sessionID},
	})
	w.WriteHeader(http.StatusNoContent)
}

// handleLinkPersonAccount menautkan akun mahasiswa ke baris peserta.
func (s Server) handleLinkPersonAccount(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFrom(r.Context())
	personID := chi.URLParam(r, "person_id")

	var req struct {
		UserID string `json:"user_id"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeErrorCode(w, r, http.StatusBadRequest, codeValidationFailed, "Badan permintaan tidak dapat dibaca.")
		return
	}
	if strings.TrimSpace(req.UserID) == "" {
		writeErrorCode(w, r, http.StatusBadRequest, codeMissingField, "user_id wajib diisi.")
		return
	}
	if err := s.Accounts.LinkPersonAccount(r.Context(), principal.UserID, personID, req.UserID); err != nil {
		s.tulisGalatDomain(w, r, err)
		return
	}
	s.catatAudit(r, audit.Event{
		ActorType: audit.ActorUser, ActorID: principal.UserID, Action: "person.account_linked",
		EntityType: "person", EntityID: personID, Result: audit.ResultSuccess,
		Reason:   "Akun mahasiswa ditautkan ke baris peserta",
		Metadata: map[string]string{"count": "1"},
	})
	writeData(w, r, http.StatusOK, map[string]string{"person_id": personID, "user_id": req.UserID})
}

// handleUnlinkPersonAccount melepas tautan akun peserta tanpa menghapus data.
func (s Server) handleUnlinkPersonAccount(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFrom(r.Context())
	personID := chi.URLParam(r, "person_id")

	if err := s.Accounts.UnlinkPersonAccount(r.Context(), principal.UserID, personID); err != nil {
		s.tulisGalatDomain(w, r, err)
		return
	}
	s.catatAudit(r, audit.Event{
		ActorType: audit.ActorUser, ActorID: principal.UserID, Action: "person.account_unlinked",
		EntityType: "person", EntityID: personID, Result: audit.ResultSuccess,
		Reason: "Tautan akun peserta dilepas",
	})
	w.WriteHeader(http.StatusNoContent)
}

// handleListClassLecturers menampilkan penugasan dosen pada satu kelas.
func (s Server) handleListClassLecturers(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFrom(r.Context())
	classID := chi.URLParam(r, "class_id")

	if err := authz.CanViewClass(r.Context(), s.Scope, principal, classID); err != nil {
		s.tulisGalatKebijakan(w, r, err)
		return
	}
	daftar, err := s.Accounts.LecturersForClass(r.Context(), classID)
	if err != nil {
		s.tulisGalatDomain(w, r, err)
		return
	}
	hasil := make([]akunRing, 0, len(daftar))
	for _, u := range daftar {
		hasil = append(hasil, akunKeRing(u))
	}
	writeData(w, r, http.StatusOK, hasil)
}

// handleAssignLecturer menugaskan dosen pada kelas. Hanya admin, dan hanya
// akun ber-role dosen yang sah ditugaskan.
func (s Server) handleAssignLecturer(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFrom(r.Context())
	classID := chi.URLParam(r, "class_id")
	userID := chi.URLParam(r, "user_id")

	if err := s.Accounts.AssignLecturer(r.Context(), principal.UserID, classID, userID); err != nil {
		s.tulisGalatDomain(w, r, err)
		return
	}
	s.catatAudit(r, audit.Event{
		ActorType: audit.ActorUser, ActorID: principal.UserID, Action: "class.dosen_assigned",
		EntityType: "class", EntityID: classID, Result: audit.ResultSuccess,
		Reason:   "Penugasan dosen pada kelas",
		Metadata: map[string]string{"count": "1"},
	})
	writeData(w, r, http.StatusCreated, map[string]string{"class_id": classID, "user_id": userID})
}

// handleRemoveLecturer mencabut penugasan dosen pada kelas.
func (s Server) handleRemoveLecturer(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFrom(r.Context())
	classID := chi.URLParam(r, "class_id")
	userID := chi.URLParam(r, "user_id")

	if err := s.Accounts.RemoveLecturer(r.Context(), classID, userID); err != nil {
		s.tulisGalatDomain(w, r, err)
		return
	}
	s.catatAudit(r, audit.Event{
		ActorType: audit.ActorUser, ActorID: principal.UserID, Action: "class.dosen_removed",
		EntityType: "class", EntityID: classID, Result: audit.ResultSuccess,
		Reason:   "Penugasan dosen dicabut dari kelas",
		Metadata: map[string]string{"count": "1"},
	})
	w.WriteHeader(http.StatusNoContent)
}

// kelasRing adalah bentuk ringkas baris `classes`.
type kelasRing struct {
	ID    string `json:"id"`
	Code  string `json:"code"`
	Title string `json:"title"`
}

// handleMeClasses mengembalikan kelas milik pemanggil: kelas yang ditugaskan
// untuk dosen, kelas yang diikuti untuk mahasiswa.
func (s Server) handleMeClasses(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFrom(r.Context())

	var daftar []auth.Class
	var err error
	switch principal.Role {
	case auth.RoleDosen:
		daftar, err = s.Accounts.ClassesForLecturer(r.Context(), principal.UserID)
	case auth.RoleMahasiswa:
		if principal.PersonID == "" {
			writeErrorCode(w, r, http.StatusConflict, codeAccountNotLinked, "Akun belum ditautkan ke data peserta.")
			return
		}
		daftar, err = s.Accounts.ClassesForPerson(r.Context(), principal.PersonID)
	default:
		writeErrorCode(w, r, http.StatusForbidden, codeForbiddenRole, pesanGalatAuth(codeForbiddenRole))
		return
	}
	if err != nil {
		s.tulisGalatDomain(w, r, err)
		return
	}

	hasil := make([]kelasRing, 0, len(daftar))
	for _, k := range daftar {
		hasil = append(hasil, kelasRing{ID: k.ID, Code: k.Code, Title: k.Title})
	}
	writeData(w, r, http.StatusOK, hasil)
}

// tulisGalatDomain memetakan galat domain ke kode status kontrak. Pemetaan ini
// sengaja eksplisit: galat yang tidak dikenal menjadi 500, bukan 400, agar
// kesalahan program tidak tersamar sebagai kesalahan pemanggil.
func (s Server) tulisGalatDomain(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, auth.ErrNotFound), errors.Is(err, session.ErrNotFound):
		writeErrorCode(w, r, http.StatusNotFound, codeNotFound, "Sumber daya tidak ditemukan.")
	case errors.Is(err, auth.ErrLastAdminProtected):
		writeErrorCode(w, r, http.StatusConflict, codeLastAdminProtected, "Operasi akan menghilangkan admin aktif terakhir.")
	case errors.Is(err, auth.ErrSelfAccountProtected):
		writeErrorCode(w, r, http.StatusConflict, codeSelfAccountProtected, "Admin tidak dapat menonaktifkan atau menurunkan peran akunnya sendiri.")
	case errors.Is(err, auth.ErrConflict), errors.Is(err, auth.ErrAlreadyLinked):
		writeErrorCode(w, r, http.StatusConflict, codeConflict, "Operasi bertentangan dengan keadaan data saat ini.")
	case errors.Is(err, auth.ErrNotMahasiswa):
		writeErrorCode(w, r, http.StatusUnprocessableEntity, codeValidationFailed, "Akun yang ditautkan harus ber-role mahasiswa.")
	case errors.Is(err, auth.ErrInvalidInput), errors.Is(err, auth.ErrRoleInvalid), errors.Is(err, auth.ErrStatusInvalid):
		writeErrorCode(w, r, http.StatusBadRequest, codeValidationFailed, "Nilai yang dikirim tidak memenuhi aturan.")
	case errors.Is(err, auth.ErrIdentityTaken), errors.Is(err, auth.ErrNotInvited):
		writeErrorCode(w, r, http.StatusConflict, codeConflict, "Identitas atau akun tidak dapat dipakai untuk operasi ini.")
	default:
		s.logger().Error("galat domain tak terduga", "request_id", reqID(r), "error", logging.Redact(err.Error()))
		writeErrorCode(w, r, http.StatusInternalServerError, codeInternalError, "Terjadi kesalahan internal.")
	}
}

// tulisGalatKebijakan memetakan galat kebijakan: peran salah menjadi 403, dan
// objek di luar cakupan menjadi 404 agar keberadaannya tidak terbocor.
func (s Server) tulisGalatKebijakan(w http.ResponseWriter, r *http.Request, err error) {
	principal, _ := PrincipalFrom(r.Context())
	switch {
	case errors.Is(err, authz.ErrForbidden):
		s.catatAudit(r, audit.Event{
			ActorType: audit.ActorUser, ActorID: principal.UserID, Action: "authz.denied",
			Result: audit.ResultFailure, Reason: "Peran tidak berwenang atas operasi ini",
			Metadata: map[string]string{"reason_code": "forbidden_role", "method": r.Method, "path": normalizePath(r.URL.Path)},
		})
		writeErrorCode(w, r, http.StatusForbidden, codeForbiddenRole, pesanGalatAuth(codeForbiddenRole))
	case errors.Is(err, authz.ErrOutOfScope):
		writeErrorCode(w, r, http.StatusNotFound, codeNotFound, "Sumber daya tidak ditemukan.")
	default:
		s.logger().Error("galat kebijakan tak terduga", "request_id", reqID(r), "error", logging.Redact(err.Error()))
		writeErrorCode(w, r, http.StatusInternalServerError, codeInternalError, "Terjadi kesalahan internal.")
	}
}

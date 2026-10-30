package httpapi

import (
	"net/http"
	"strings"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/audit"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/auth"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/authz"
	"github.com/go-chi/chi/v5"
)

// personDaftarRing adalah bentuk peserta pada daftar (kontrak §5.2). Nilai
// consent/enrollment adalah proyeksi baca-saja; tidak ada payload biometrik.
type personDaftarRing struct {
	ID              string `json:"id"`
	ExternalCode    string `json:"external_code"`
	DisplayName     string `json:"display_name"`
	Status          string `json:"status"`
	ConsentState    string `json:"consent_state"`
	EnrollmentState string `json:"enrollment_state"`
}

// personDetailRing menambahkan field roster nonbiometrik untuk admin/dosen
// yang berhak. NIM tetap berupa external_code; reference bersifat opaque.
type personDetailRing struct {
	ID                 string  `json:"id"`
	ExternalCode       string  `json:"external_code"`
	DisplayName        string  `json:"display_name"`
	Status             string  `json:"status"`
	Note               string  `json:"note,omitempty"`
	IdentityReference  string  `json:"identity_reference,omitempty"`
	IdentityVerifiedAt string  `json:"identity_verified_at,omitempty"`
	AccountUserID      *string `json:"account_user_id"`
	ConsentState       string  `json:"consent_state"`
	EnrollmentState    string  `json:"enrollment_state"`
	CreatedAt          string  `json:"created_at"`
}

func personKeDaftar(p auth.Person) personDaftarRing {
	return personDaftarRing{
		ID:              p.ID,
		ExternalCode:    p.ExternalCode,
		DisplayName:     p.DisplayName,
		Status:          p.Status,
		ConsentState:    p.ConsentState,
		EnrollmentState: p.EnrollmentState,
	}
}

func personKeDetail(p auth.Person) personDetailRing {
	ring := personDetailRing{
		ID:                p.ID,
		ExternalCode:      p.ExternalCode,
		DisplayName:       p.DisplayName,
		Status:            p.Status,
		Note:              p.Note,
		IdentityReference: p.IdentityReference,
		ConsentState:      p.ConsentState,
		EnrollmentState:   p.EnrollmentState,
		CreatedAt:         audit.FormatWaktu(p.CreatedAt),
	}
	if p.IdentityVerifiedAt != nil {
		ring.IdentityVerifiedAt = audit.FormatWaktu(*p.IdentityVerifiedAt)
	}
	if p.AccountUserID != "" {
		id := p.AccountUserID
		ring.AccountUserID = &id
	}
	return ring
}

// handleListPeople mengembalikan peserta sesuai cakupan. Dosen hanya menerima
// peserta pada kelas yang ditugaskan; admin dapat menyaring dengan class_id.
func (s Server) handleListPeople(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFrom(r.Context())

	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status != "" && status != "active" && status != "inactive" {
		writeErrorCode(w, r, http.StatusBadRequest, codeValidationFailed, "Filter status peserta harus active atau inactive.")
		return
	}
	classID := strings.TrimSpace(r.URL.Query().Get("class_id"))

	if classID != "" {
		if principal.IsDosen() {
			if err := authz.CanViewClass(r.Context(), s.Scope, principal, classID); err != nil {
				s.tulisGalatKebijakan(w, r, err)
				return
			}
		} else if _, err := s.Accounts.ClassByID(r.Context(), classID); err != nil {
			s.tulisGalatDomain(w, r, err)
			return
		}
	}

	halaman, ok := bacaHalaman(w, r, jenisKursorPerson)
	if !ok {
		return
	}

	daftar, err := s.Accounts.ListPeople(r.Context(), auth.PersonQuery{
		IsAdmin:        principal.IsAdmin(),
		ScopeUserID:    principal.UserID,
		Status:         status,
		ClassID:        classID,
		Limit:          halaman.Limit + 1,
		AfterCreatedAt: halaman.AfterCreatedAt,
		AfterID:        halaman.AfterID,
	})
	if err != nil {
		s.tulisGalatDomain(w, r, err)
		return
	}

	adaLagi := len(daftar) > halaman.Limit
	if adaLagi {
		daftar = daftar[:halaman.Limit]
	}
	hasil := make([]personDaftarRing, 0, len(daftar))
	for _, p := range daftar {
		hasil = append(hasil, personKeDaftar(p))
	}

	berikut := ""
	if adaLagi && len(daftar) > 0 {
		terakhir := daftar[len(daftar)-1]
		berikut = kursorPerson(audit.FormatWaktu(terakhir.CreatedAt), terakhir.ID)
	}
	writePage(w, r, hasil, berikut, adaLagi, halaman.Limit)
}

// handleGetPerson menampilkan detail peserta. Dosen hanya untuk peserta pada
// kelas yang ditugaskan; objek di luar cakupan dijawab 404.
func (s Server) handleGetPerson(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFrom(r.Context())
	personID := chi.URLParam(r, "person_id")

	if err := authz.CanViewPerson(r.Context(), s.Scope, principal, personID); err != nil {
		s.tulisGalatKebijakan(w, r, err)
		return
	}
	p, err := s.Accounts.PersonByID(r.Context(), personID)
	if err != nil {
		s.tulisGalatDomain(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, personKeDetail(p))
}

// handleCreatePerson mencatat peserta dari roster kampus. Admin yang menulis
// baris ini adalah attestation roster; bukan verifikasi identitas otomatis.
func (s Server) handleCreatePerson(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ExternalCode      string `json:"external_code"`
		DisplayName       string `json:"display_name"`
		Note              string `json:"note"`
		IdentityReference string `json:"identity_reference"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeErrorCode(w, r, http.StatusBadRequest, codeValidationFailed, "Badan permintaan tidak dapat dibaca.")
		return
	}
	if strings.TrimSpace(req.ExternalCode) == "" || strings.TrimSpace(req.DisplayName) == "" {
		writeErrorCode(w, r, http.StatusBadRequest, codeMissingField, "external_code dan display_name wajib diisi.")
		return
	}

	p, err := s.Accounts.CreatePerson(r.Context(), principalID(r), req.ExternalCode, req.DisplayName, req.Note, req.IdentityReference)
	if err != nil {
		s.tulisGalatDomain(w, r, err)
		return
	}
	writeData(w, r, http.StatusCreated, personKeDetail(p))
}

// handlePatchPerson mengubah field roster nonbiometrik. NIM/external_code
// sengaja tidak dapat diubah lewat endpoint ini.
func (s Server) handlePatchPerson(w http.ResponseWriter, r *http.Request) {
	personID := chi.URLParam(r, "person_id")

	var req struct {
		DisplayName       *string `json:"display_name"`
		Status            *string `json:"status"`
		Note              *string `json:"note"`
		IdentityReference *string `json:"identity_reference"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeErrorCode(w, r, http.StatusBadRequest, codeValidationFailed, "Badan permintaan tidak dapat dibaca.")
		return
	}
	if req.DisplayName == nil && req.Status == nil && req.Note == nil && req.IdentityReference == nil {
		writeErrorCode(w, r, http.StatusBadRequest, codeMissingField, "Minimal satu field peserta harus diisi.")
		return
	}

	p, err := s.Accounts.UpdatePerson(r.Context(), principalID(r), personID, auth.PersonUpdate{
		DisplayName:       req.DisplayName,
		Status:            req.Status,
		Note:              req.Note,
		IdentityReference: req.IdentityReference,
	})
	if err != nil {
		s.tulisGalatDomain(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, personKeDetail(p))
}

// handleDeletePerson menghapus lunak peserta. Riwayat kehadiran yang sudah ada
// menahan operasi dengan 409 POLICY_CONFLICT.
func (s Server) handleDeletePerson(w http.ResponseWriter, r *http.Request) {
	personID := chi.URLParam(r, "person_id")
	if err := s.Accounts.DeletePerson(r.Context(), principalID(r), personID); err != nil {
		s.tulisGalatDomain(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleListClasses menampilkan kelas untuk admin (semua) atau dosen (yang
// ditugaskan). Paginasi dilakukan setelah pembacaan karena jumlah kelas MVP
// kecil; kursor tetap memakai jenis daftar kelas.
func (s Server) handleListClasses(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFrom(r.Context())
	halaman, ok := bacaHalaman(w, r, jenisKursorDaftarKelas)
	if !ok {
		return
	}

	var daftar []auth.Class
	var err error
	if principal.IsAdmin() {
		daftar, err = s.Accounts.AllClasses(r.Context())
	} else {
		daftar, err = s.Accounts.ClassesForLecturer(r.Context(), principal.UserID)
	}
	if err != nil {
		s.tulisGalatDomain(w, r, err)
		return
	}

	mulai := 0
	if halaman.Lanjut {
		for mulai < len(daftar) {
			k := daftar[mulai]
			if k.Code > halaman.AfterCreatedAt || (k.Code == halaman.AfterCreatedAt && k.ID > halaman.AfterID) {
				break
			}
			mulai++
		}
	}
	if mulai > len(daftar) {
		mulai = len(daftar)
	}
	sisa := daftar[mulai:]
	adaLagi := len(sisa) > halaman.Limit
	if adaLagi {
		sisa = sisa[:halaman.Limit]
	}

	hasil := make([]kelasRing, 0, len(sisa))
	for _, k := range sisa {
		hasil = append(hasil, kelasKeRing(k))
	}
	berikut := ""
	if adaLagi && len(sisa) > 0 {
		terakhir := sisa[len(sisa)-1]
		berikut = bikinKursor(jenisKursorDaftarKelas, terakhir.Code, terakhir.ID)
	}
	writePage(w, r, hasil, berikut, adaLagi, halaman.Limit)
}

// handleCreateClass membuat kelas baru (admin).
func (s Server) handleCreateClass(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code  string `json:"code"`
		Title string `json:"title"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeErrorCode(w, r, http.StatusBadRequest, codeValidationFailed, "Badan permintaan tidak dapat dibaca.")
		return
	}
	if strings.TrimSpace(req.Code) == "" || strings.TrimSpace(req.Title) == "" {
		writeErrorCode(w, r, http.StatusBadRequest, codeMissingField, "code dan title wajib diisi.")
		return
	}

	k, err := s.Accounts.CreateClass(r.Context(), principalID(r), req.Code, req.Title)
	if err != nil {
		s.tulisGalatDomain(w, r, err)
		return
	}
	writeData(w, r, http.StatusCreated, kelasKeRing(k))
}

// handleListClassMembers menampilkan peserta aktif pada satu kelas.
func (s Server) handleListClassMembers(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFrom(r.Context())
	classID := chi.URLParam(r, "class_id")

	if err := authz.CanViewClass(r.Context(), s.Scope, principal, classID); err != nil {
		s.tulisGalatKebijakan(w, r, err)
		return
	}
	daftar, err := s.Accounts.MembersForClass(r.Context(), classID)
	if err != nil {
		s.tulisGalatDomain(w, r, err)
		return
	}
	type memberRing struct {
		PersonID     string `json:"person_id"`
		ExternalCode string `json:"external_code"`
		DisplayName  string `json:"display_name"`
	}
	hasil := make([]memberRing, 0, len(daftar))
	for _, p := range daftar {
		if p.Status == "active" {
			hasil = append(hasil, memberRing{PersonID: p.ID, ExternalCode: p.ExternalCode, DisplayName: p.DisplayName})
		}
	}
	writeData(w, r, http.StatusOK, hasil)
}

// handleAddClassMember menambahkan peserta ke kelas (admin). Pengulangan
// menghasilkan 200 tanpa baris ganda; penambahan baru menghasilkan 201.
func (s Server) handleAddClassMember(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "class_id")
	personID := chi.URLParam(r, "person_id")

	dibuat, err := s.Accounts.AddClassMember(r.Context(), principalID(r), classID, personID)
	if err != nil {
		s.tulisGalatDomain(w, r, err)
		return
	}
	status := http.StatusOK
	if dibuat {
		status = http.StatusCreated
	}
	writeData(w, r, status, map[string]string{"class_id": classID, "person_id": personID})
}

// handleRemoveClassMember menghapus keanggotaan kelas (admin).
func (s Server) handleRemoveClassMember(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "class_id")
	personID := chi.URLParam(r, "person_id")

	if err := s.Accounts.RemoveClassMember(r.Context(), principalID(r), classID, personID); err != nil {
		s.tulisGalatDomain(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// principalID mengambil id aktor terautentikasi. Handler hanya dipanggil di
// balik middleware peran, sehingga nilai kosong berarti kesalahan konfigurasi
// router; store akan menolaknya sebagai masukan tidak sah.
func principalID(r *http.Request) string {
	p, _ := PrincipalFrom(r.Context())
	return p.UserID
}

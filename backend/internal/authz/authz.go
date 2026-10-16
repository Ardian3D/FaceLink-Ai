// Package authz menegakkan kebijakan peran dan cakupan sesuai
// docs/AUTHORIZATION.md: deny-by-default, least privilege, dan pemisahan tegas
// antara "peran tidak berwenang" (403) dan "objek di luar cakupan" (404).
//
// Kebijakan dipisahkan dari HTTP supaya dapat diuji tanpa server, dan setiap
// keputusan cakupan selalu diambil di backend — UI hanya menyembunyikan tombol.
package authz

import (
	"context"
	"errors"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/auth"
)

// Sentinel error kebijakan. Pemanggil HTTP memetakannya ke kode status kontrak:
// ErrForbidden → 403 FORBIDDEN_ROLE, ErrOutOfScope → 404 NOT_FOUND.
var (
	ErrForbidden  = errors.New("authz: peran tidak berwenang")
	ErrOutOfScope = errors.New("authz: objek di luar cakupan pemanggil")
)

// Principal adalah identitas pemanggil yang sudah terautentikasi.
type Principal struct {
	UserID   string
	Role     auth.Role
	PersonID string // terisi hanya bila akun mahasiswa tertaut ke baris peserta
}

// IsAdmin melaporkan apakah pemanggil administrator sistem.
func (p Principal) IsAdmin() bool { return p.Role == auth.RoleAdmin }

// IsDosen melaporkan apakah pemanggil pengajar.
func (p Principal) IsDosen() bool { return p.Role == auth.RoleDosen }

// IsMahasiswa melaporkan apakah pemanggil peserta.
func (p Principal) IsMahasiswa() bool { return p.Role == auth.RoleMahasiswa }

// Directory menyediakan fakta cakupan yang dibutuhkan kebijakan.
type Directory interface {
	// IsLecturerAssigned melaporkan apakah pengguna ditugaskan pada kelas.
	IsLecturerAssigned(ctx context.Context, userID, classID string) (bool, error)
	// IsClassMember melaporkan apakah peserta adalah anggota kelas.
	IsClassMember(ctx context.Context, personID, classID string) (bool, error)
	// IsPersonInLecturerScope melaporkan apakah peserta termasuk anggota salah
	// satu kelas yang ditugaskan kepada dosen tersebut.
	IsPersonInLecturerScope(ctx context.Context, userID, personID string) (bool, error)
	// PersonForUser mengembalikan baris peserta yang tertaut ke akun.
	PersonForUser(ctx context.Context, userID string) (string, bool, error)
}

// CanManageAccounts membatasi pengelolaan akun dan role pada admin (D-034).
func CanManageAccounts(p Principal) error {
	if !p.IsAdmin() {
		return ErrForbidden
	}
	return nil
}

// CanManageClasses membatasi pembuatan kelas dan penugasan dosen pada admin.
func CanManageClasses(p Principal) error {
	if !p.IsAdmin() {
		return ErrForbidden
	}
	return nil
}

// CanManageDevices membatasi pengelolaan perangkat IoT pada admin.
func CanManageDevices(p Principal) error {
	if !p.IsAdmin() {
		return ErrForbidden
	}
	return nil
}

// CanManageBiometrics membatasi enrollment dan persetujuan biometrik pada admin.
func CanManageBiometrics(p Principal) error {
	if !p.IsAdmin() {
		return ErrForbidden
	}
	return nil
}

// CanViewAuditEvents membatasi pembacaan audit pada admin.
func CanViewAuditEvents(p Principal) error {
	if !p.IsAdmin() {
		return ErrForbidden
	}
	return nil
}

// CanExportAttendance membatasi ekspor data administratif pada admin.
func CanExportAttendance(p Principal) error {
	if !p.IsAdmin() {
		return ErrForbidden
	}
	return nil
}

// CanViewClass mengizinkan admin pada kelas mana pun dan dosen hanya pada kelas
// yang ditugaskan. Mahasiswa tidak memakai jalur ini (mereka memakai /me/*).
func CanViewClass(ctx context.Context, d Directory, p Principal, classID string) error {
	switch {
	case p.IsAdmin():
		return nil
	case p.IsDosen():
		ok, err := d.IsLecturerAssigned(ctx, p.UserID, classID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrOutOfScope
		}
		return nil
	default:
		return ErrForbidden
	}
}

// CanOperateClass mengizinkan operasi kelas (sesi absensi, koreksi manual) bagi
// admin dan dosen yang ditugaskan. Admin tidak memerlukan role dosen kedua.
func CanOperateClass(ctx context.Context, d Directory, p Principal, classID string) error {
	return CanViewClass(ctx, d, p, classID)
}

// CanViewPerson mengizinkan admin, dosen untuk peserta pada kelas yang
// ditugaskannya, dan mahasiswa hanya untuk dirinya sendiri.
func CanViewPerson(ctx context.Context, d Directory, p Principal, personID string) error {
	switch {
	case p.IsAdmin():
		return nil
	case p.IsDosen():
		ok, err := d.IsPersonInLecturerScope(ctx, p.UserID, personID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrOutOfScope
		}
		return nil
	case p.IsMahasiswa():
		return CanReadOwnPerson(ctx, d, p, personID)
	default:
		return ErrForbidden
	}
}

// CanReadOwnPerson membatasi akses mahasiswa pada datanya sendiri. Akun yang
// belum tertaut peserta selalu ditolak di sini; pemanggil menerjemahkannya
// menjadi 409 ACCOUNT_NOT_LINKED sesuai kontrak.
func CanReadOwnPerson(ctx context.Context, d Directory, p Principal, personID string) error {
	if !p.IsMahasiswa() {
		return ErrForbidden
	}
	miliknya, ok, err := d.PersonForUser(ctx, p.UserID)
	if err != nil {
		return err
	}
	if !ok || miliknya != personID {
		return ErrOutOfScope
	}
	return nil
}

// CanViewClassAsStudent melaporkan apakah mahasiswa berhak melihat kelas yang
// diikutinya. Dipakai endpoint /me/classes.
func CanViewClassAsStudent(ctx context.Context, d Directory, p Principal, classID string) error {
	if !p.IsMahasiswa() {
		return ErrForbidden
	}
	personID, ok, err := d.PersonForUser(ctx, p.UserID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrOutOfScope
	}
	anggota, err := d.IsClassMember(ctx, personID, classID)
	if err != nil {
		return err
	}
	if !anggota {
		return ErrOutOfScope
	}
	return nil
}

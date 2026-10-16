package auth

import (
	"errors"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// Sentinel galat paket auth. Pemanggil memakai errors.Is; setiap galat
// dibungkus dengan fmt.Errorf("...: %w", err) agar konteks tetap terbaca pada
// log tanpa kehilangan kemampuan pencocokan.
var (
	// ErrNotFound dipakai bila baris yang diminta tidak ada. sql.ErrNoRows tidak
	// pernah bocor keluar paket ini.
	ErrNotFound = errors.New("auth: baris tidak ditemukan")
	// ErrConflict dipakai bila keadaan database menolak operasi: email ganda,
	// admin pertama yang sudah ada, peserta yang sudah tertaut akun lain.
	ErrConflict = errors.New("auth: konflik keadaan")
	// ErrInvalidInput dipakai bila masukan tidak lengkap atau tidak sah.
	ErrInvalidInput = errors.New("auth: masukan tidak sah")
	// ErrNotInvited dipakai bila tidak ada undangan yang cocok untuk login.
	ErrNotInvited = errors.New("auth: tidak ada undangan yang cocok")
	// ErrIdentityTaken dipakai bila subjek Google sudah tertaut akun lain.
	ErrIdentityTaken = errors.New("auth: identitas Google sudah tertaut akun lain")
	// ErrLastAdminProtected menjaga agar admin aktif terakhir tidak hilang.
	ErrLastAdminProtected = errors.New("auth: admin aktif terakhir dilindungi")
	// ErrSelfAccountProtected melarang aktor mengubah akunnya sendiri.
	ErrSelfAccountProtected = errors.New("auth: akun sendiri dilindungi")
	// ErrRoleInvalid dipakai bila role tidak termasuk tiga role manusia.
	ErrRoleInvalid = errors.New("auth: role tidak sah")
	// ErrStatusInvalid dipakai bila status tidak termasuk tiga status akun.
	ErrStatusInvalid = errors.New("auth: status tidak sah")
	// ErrNotMahasiswa dipakai bila pengaitan peserta diminta untuk akun yang
	// bukan mahasiswa.
	ErrNotMahasiswa = errors.New("auth: akun bukan mahasiswa")
	// ErrAlreadyLinked dipakai bila akun sudah memiliki identitas atau sudah
	// tertaut ke peserta lain.
	ErrAlreadyLinked = errors.New("auth: akun sudah tertaut")
)

// isUniqueViolation memeriksa galat batasan unik SQLite. Indeks unik adalah
// penjaga terakhir penautan (D-032): dua permintaan login bersamaan hanya boleh
// menghasilkan satu baris, dan pelanggaran dari database harus dipetakan ke
// sentinel yang sama dengan pemeriksaan di dalam transaksi.
func isUniqueViolation(err error) bool {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	switch sqliteErr.Code() {
	case sqlite3.SQLITE_CONSTRAINT_UNIQUE, sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY:
		return true
	default:
		return false
	}
}

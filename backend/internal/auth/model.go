package auth

import "time"

// providerGoogle adalah satu-satunya penyedia identitas yang diakui skema
// (`user_identities.provider` memiliki CHECK (provider = 'google')).
const providerGoogle = "google"

// Role adalah role manusia yang diakui tabel acuan `roles` setelah migrasi
// 00002 (D-031). Satu akun memiliki satu role utama; tidak ada perpindahan role
// diam-diam (D-037).
type Role string

// Role manusia yang sah.
const (
	RoleAdmin     Role = "admin"
	RoleDosen     Role = "dosen"
	RoleMahasiswa Role = "mahasiswa"
)

// Valid memastikan role termasuk tiga role manusia. Nilai di luar daftar ini
// ditolak sebelum menyentuh database supaya permintaan yang salah bentuk tidak
// pernah menghasilkan baris dengan role yang tidak dikenal.
func (r Role) Valid() bool {
	switch r {
	case RoleAdmin, RoleDosen, RoleMahasiswa:
		return true
	default:
		return false
	}
}

// Status adalah lifecycle akun D-032: akun lahir `invited`, menjadi `active`
// setelah identitas Google tertaut, dan `suspended` mencabut seluruh sesinya.
type Status string

// Status akun yang sah.
const (
	StatusInvited   Status = "invited"
	StatusActive    Status = "active"
	StatusSuspended Status = "suspended"
)

// Valid memastikan status termasuk tiga status lifecycle akun.
func (s Status) Valid() bool {
	switch s {
	case StatusInvited, StatusActive, StatusSuspended:
		return true
	default:
		return false
	}
}

// User adalah satu baris tabel `users`. Kata sandi tidak lagi disimpan (D-032):
// autentikasi sepenuhnya lewat identitas Google pada `user_identities`.
type User struct {
	ID          string
	Email       string
	DisplayName string
	Role        Role
	Status      Status
	// CreatedBy berisi id admin yang mengundang; kosong untuk akun yang dibuat
	// sistem, termasuk admin pertama hasil bootstrap.
	CreatedBy string
	// LastLoginAt kosong selama akun belum pernah menautkan identitas.
	LastLoginAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Identity adalah penautan identitas Google ke satu akun. `Subject` adalah klaim
// `sub` Google, satu-satunya kunci identitas yang stabil (email bukan kunci).
type Identity struct {
	ID              string
	UserID          string
	Provider        string
	Subject         string
	EmailAtProvider string
	LinkedAt        time.Time
	CreatedAt       time.Time
}

// Class adalah kelas yang dipakai untuk menampilkan cakupan seorang dosen
// (D-034). Hanya kolom yang dibutuhkan pemanggil yang dibawa.
//
// MemberCount dan ActiveSessionID adalah nilai turunan yang dihitung pada kueri
// yang sama (kontrak §5.14). ActiveSessionID kosong berarti tidak ada sesi
// absensi terbuka untuk kelas itu; kolomnya tidak membocorkan data kehadiran.
type Class struct {
	ID              string
	Code            string
	Title           string
	MemberCount     int
	ActiveSessionID string
}

// LecturerAssignment adalah satu penugasan dosen pada kelas beserta waktu
// penugasannya (kontrak §5.5).
type LecturerAssignment struct {
	UserID      string
	DisplayName string
	Email       string
	AssignedAt  time.Time
}

// Person adalah potret satu baris peserta seperlunya respons API (kontrak §5.2).
// AccountUserID kosong berarti peserta belum tertaut akun mana pun.
type Person struct {
	ID            string
	ExternalCode  string
	DisplayName   string
	Status        string
	AccountUserID string
}

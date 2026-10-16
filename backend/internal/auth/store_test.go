package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/clock"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/migrations"
)

// waktuUji adalah titik waktu tetap agar seluruh cap waktu pada pengujian dapat
// dibandingkan tanpa bergantung jam mesin.
var waktuUji = time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC)

// bukaStore menyiapkan database sementara dengan skema nyata yang sudah
// diterapkan. Pengujian memakai berkas migrasi yang benar-benar dikirim backend,
// bukan skema tiruan, supaya batasan unik dan foreign key yang menjadi penjaga
// terakhir benar-benar diuji.
func bukaStore(t *testing.T) (*Store, *database.DB, *clock.Fake) {
	t.Helper()
	ctx := context.Background()

	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "facelink.db"))
	if err != nil {
		t.Fatalf("database.Open() gagal: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	dir, err := filepath.Abs(filepath.Join("..", "..", "migrations"))
	if err != nil {
		t.Fatalf("resolusi direktori migrasi gagal: %v", err)
	}
	if _, err := migrations.Run(ctx, db, dir); err != nil {
		t.Fatalf("penerapan migrasi gagal: %v", err)
	}

	jam := clock.NewFake(waktuUji)
	return NewStore(db, jam), db, jam
}

// undangAdmin membuat admin pertama lalu mengembalikan barisnya.
func undangAdmin(t *testing.T, s *Store) User {
	t.Helper()
	admin, err := s.CreateFirstAdmin(context.Background(), "admin@example.com", "Admin Utama")
	if err != nil {
		t.Fatalf("CreateFirstAdmin() gagal: %v", err)
	}
	return admin
}

// aktifkanAdmin mengubah admin hasil bootstrap menjadi `active` agar aturan
// "admin aktif terakhir" dapat diuji.
func aktifkanAdmin(t *testing.T, s *Store, db *database.DB, adminID string) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(),
		`UPDATE users SET status = 'active' WHERE id = ?`, adminID); err != nil {
		t.Fatalf("aktivasi admin gagal: %v", err)
	}
}

// buatKelas menyisipkan baris kelas. Tabel `classes` dimiliki tugas lain; di
// sini ia hanya dipakai sebagai prasyarat penugasan dosen.
func buatKelas(t *testing.T, db *database.DB, id, code, title string) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO classes (id, code, title) VALUES (?, ?, ?)`, id, code, title); err != nil {
		t.Fatalf("penyisipan kelas %s gagal: %v", id, err)
	}
}

// buatPeserta menyisipkan baris peserta.
func buatPeserta(t *testing.T, db *database.DB, id, externalCode, nama string) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO people (id, external_code, display_name) VALUES (?, ?, ?)`, id, externalCode, nama); err != nil {
		t.Fatalf("penyisipan peserta %s gagal: %v", id, err)
	}
}

// hitung menjalankan COUNT(*) sederhana untuk memeriksa jumlah baris.
func hitung(t *testing.T, db *database.DB, kueri string, args ...any) int {
	t.Helper()
	var jumlah int
	if err := db.QueryRowContext(context.Background(), kueri, args...).Scan(&jumlah); err != nil {
		t.Fatalf("COUNT gagal (%s): %v", kueri, err)
	}
	return jumlah
}

func TestInviteUserBerhasilDanEmailDinormalkan(t *testing.T) {
	s, db, _ := bukaStore(t)
	ctx := context.Background()
	admin := undangAdmin(t, s)

	user, err := s.InviteUser(ctx, "  Dosen@Example.COM ", "  Budi  ", RoleDosen, admin.ID)
	if err != nil {
		t.Fatalf("InviteUser() gagal: %v", err)
	}
	if user.Email != "dosen@example.com" {
		t.Errorf("email = %q, ingin %q", user.Email, "dosen@example.com")
	}
	if user.DisplayName != "Budi" {
		t.Errorf("display_name = %q, ingin %q", user.DisplayName, "Budi")
	}
	if user.Role != RoleDosen {
		t.Errorf("role = %q, ingin %q", user.Role, RoleDosen)
	}
	if user.Status != StatusInvited {
		t.Errorf("status = %q, ingin %q", user.Status, StatusInvited)
	}
	if user.CreatedBy != admin.ID {
		t.Errorf("created_by = %q, ingin %q", user.CreatedBy, admin.ID)
	}
	if user.LastLoginAt != nil {
		t.Errorf("last_login_at = %v, ingin kosong", user.LastLoginAt)
	}
	if user.ID == "" {
		t.Error("id pengguna kosong")
	}

	// Email yang berbeda penulisannya menunjuk akun yang sama; email duplikat
	// harus ditolak tanpa membuat baris kedua.
	if _, err := s.InviteUser(ctx, "DOSEN@example.com", "Budi Lain", RoleMahasiswa, admin.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("InviteUser() email duplikat = %v, ingin ErrConflict", err)
	}
	if got := hitung(t, db, `SELECT COUNT(*) FROM users WHERE email = ?`, "dosen@example.com"); got != 1 {
		t.Errorf("jumlah baris email dosen@example.com = %d, ingin 1", got)
	}

	// Pembuat undangan boleh kosong untuk akun yang dibuat sistem.
	sistem, err := s.InviteUser(ctx, "sistem@example.com", "Akun Sistem", RoleMahasiswa, "")
	if err != nil {
		t.Fatalf("InviteUser() tanpa pembuat gagal: %v", err)
	}
	if sistem.CreatedBy != "" {
		t.Errorf("created_by = %q, ingin kosong", sistem.CreatedBy)
	}
	var createdBy sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT created_by FROM users WHERE id = ?`, sistem.ID).Scan(&createdBy); err != nil {
		t.Fatalf("pembacaan created_by gagal: %v", err)
	}
	if createdBy.Valid {
		t.Errorf("created_by di database = %q, ingin NULL", createdBy.String)
	}
}

func TestInviteUserMenolakMasukanTidakSah(t *testing.T) {
	s, db, _ := bukaStore(t)
	ctx := context.Background()
	admin := undangAdmin(t, s)

	kasus := []struct {
		nama  string
		email string
		disp  string
		role  Role
		ingin error
	}{
		{"email kosong", "   ", "Budi", RoleDosen, ErrInvalidInput},
		{"display_name kosong", "budi@example.com", "  ", RoleDosen, ErrInvalidInput},
		{"role kosong", "budi@example.com", "Budi", "", ErrRoleInvalid},
		{"role asing", "budi@example.com", "Budi", Role("operator"), ErrRoleInvalid},
		{"role device", "budi@example.com", "Budi", Role("device"), ErrRoleInvalid},
	}
	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			if _, err := s.InviteUser(ctx, k.email, k.disp, k.role, admin.ID); !errors.Is(err, k.ingin) {
				t.Fatalf("InviteUser() = %v, ingin %v", err, k.ingin)
			}
		})
	}

	if got := hitung(t, db, `SELECT COUNT(*) FROM users`); got != 1 {
		t.Errorf("jumlah pengguna = %d, ingin hanya admin pertama (1)", got)
	}
}

func TestCreateFirstAdminBootstrapSekali(t *testing.T) {
	s, db, _ := bukaStore(t)
	ctx := context.Background()

	admin, err := s.CreateFirstAdmin(ctx, "  ADMIN@Example.com ", " Admin ")
	if err != nil {
		t.Fatalf("CreateFirstAdmin() gagal: %v", err)
	}
	if admin.Email != "admin@example.com" {
		t.Errorf("email = %q, ingin dinormalkan", admin.Email)
	}
	if admin.Role != RoleAdmin || admin.Status != StatusInvited {
		t.Errorf("role/status = %q/%q, ingin admin/invited", admin.Role, admin.Status)
	}
	if admin.CreatedBy != "" {
		t.Errorf("created_by = %q, ingin kosong (dibuat sistem)", admin.CreatedBy)
	}

	// Percobaan kedua ditolak: pengguna Google pertama tidak boleh otomatis
	// menjadi admin dan bootstrap tidak boleh diulang.
	if _, err := s.CreateFirstAdmin(ctx, "admin2@example.com", "Admin Kedua"); !errors.Is(err, ErrConflict) {
		t.Fatalf("CreateFirstAdmin() kedua = %v, ingin ErrConflict", err)
	}
	if got := hitung(t, db, `SELECT COUNT(*) FROM users WHERE role = 'admin'`); got != 1 {
		t.Errorf("jumlah admin = %d, ingin 1", got)
	}

	// Admin `suspended` tidak lagi dihitung sebagai admin yang ada; bootstrap
	// baru tetap memerlukan penilaian operator, jadi baris lama tidak diubah.
	if _, err := db.ExecContext(ctx, `UPDATE users SET status = 'suspended' WHERE id = ?`, admin.ID); err != nil {
		t.Fatalf("penonaktifan admin gagal: %v", err)
	}
	if _, err := s.CreateFirstAdmin(ctx, "admin2@example.com", "Admin Kedua"); err != nil {
		t.Fatalf("CreateFirstAdmin() setelah admin disuspend gagal: %v", err)
	}
}

func TestSetRoleMenolakAkunSendiriDanAdminTerakhir(t *testing.T) {
	s, db, _ := bukaStore(t)
	ctx := context.Background()
	admin := undangAdmin(t, s)
	aktifkanAdmin(t, s, db, admin.ID)

	// Selama akun itu satu-satunya admin aktif, aturan "admin terakhir"
	// diperiksa lebih dahulu — termasuk ketika pelakunya akun itu sendiri
	// (docs/API_CONTRACT.md §5.2, skenario TA-21).
	if _, err := s.SetRole(ctx, admin.ID, admin.ID, RoleDosen); !errors.Is(err, ErrLastAdminProtected) {
		t.Fatalf("SetRole() akun sendiri saat admin terakhir = %v, ingin ErrLastAdminProtected", err)
	}

	sebelum := hitung(t, db, `SELECT COUNT(*) FROM users WHERE role = 'admin' AND status = 'active'`)
	_, err := s.SetRole(ctx, "admin-lain-tidak-ada", admin.ID, RoleDosen)
	if !errors.Is(err, ErrLastAdminProtected) {
		t.Fatalf("SetRole() admin aktif terakhir = %v, ingin ErrLastAdminProtected", err)
	}
	sesudah := hitung(t, db, `SELECT COUNT(*) FROM users WHERE role = 'admin' AND status = 'active'`)
	if sebelum != sesudah {
		t.Errorf("jumlah admin aktif berubah dari %d menjadi %d", sebelum, sesudah)
	}
	if sesudah != 1 {
		t.Errorf("jumlah admin aktif = %d, ingin tetap 1", sesudah)
	}

	if _, err := s.SetRole(ctx, admin.ID, "pengguna-tidak-ada", RoleDosen); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SetRole() pengguna tidak ada = %v, ingin ErrNotFound", err)
	}
	if _, err := s.SetRole(ctx, admin.ID, admin.ID, Role("operator")); !errors.Is(err, ErrRoleInvalid) {
		t.Fatalf("SetRole() role tidak sah = %v, ingin ErrRoleInvalid", err)
	}

	// Admin kedua membuka jalan menurunkan admin pertama; keduanya tetap
	// tersimpan sebagai dosen setelah perubahan.
	admin2 := undangAdminKedua(t, s, db, admin)
	if _, err := s.SetRole(ctx, admin2.ID, admin2.ID, RoleDosen); !errors.Is(err, ErrSelfAccountProtected) {
		t.Fatalf("SetRole() akun sendiri dengan admin lain tersedia = %v, ingin ErrSelfAccountProtected", err)
	}
	diperbarui, err := s.SetRole(ctx, admin2.ID, admin.ID, RoleDosen)
	if err != nil {
		t.Fatalf("SetRole() dengan admin lain tersedia gagal: %v", err)
	}
	if diperbarui.Role != RoleDosen {
		t.Errorf("role = %q, ingin %q", diperbarui.Role, RoleDosen)
	}
	if diperbarui.UpdatedAt.Before(waktuUji) {
		t.Errorf("updated_at = %v, ingin tidak mundur", diperbarui.UpdatedAt)
	}
}

// undangAdminKedua menambahkan admin kedua lewat undangan admin yang sudah ada
// (bootstrap lokal memang hanya sekali), lalu mengaktifkannya agar aturan
// "admin aktif terakhir" dapat diuji.
func undangAdminKedua(t *testing.T, s *Store, db *database.DB, admin User) User {
	t.Helper()
	ctx := context.Background()

	user, err := s.InviteUser(ctx, "admin2@example.com", "Admin Kedua", RoleAdmin, admin.ID)
	if err != nil {
		t.Fatalf("InviteUser() admin kedua gagal: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE users SET status = 'active' WHERE id = ?`, user.ID); err != nil {
		t.Fatalf("aktivasi admin kedua gagal: %v", err)
	}
	aktif, err := s.UserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("UserByID() admin kedua gagal: %v", err)
	}
	return aktif
}

func TestSetStatusMenolakAkunSendiriDanAdminTerakhir(t *testing.T) {
	s, db, _ := bukaStore(t)
	ctx := context.Background()
	admin := undangAdmin(t, s)
	aktifkanAdmin(t, s, db, admin.ID)

	// Aturan "admin terakhir" diperiksa lebih dahulu; karena ini akun admin
	// aktif satu-satunya, kode yang dijawab LAST_ADMIN (TA-21).
	if _, err := s.SetStatus(ctx, admin.ID, admin.ID, StatusSuspended); !errors.Is(err, ErrLastAdminProtected) {
		t.Fatalf("SetStatus() akun sendiri saat admin terakhir = %v, ingin ErrLastAdminProtected", err)
	}
	if _, err := s.SetStatus(ctx, admin.ID, admin.ID, Status("disabled")); !errors.Is(err, ErrStatusInvalid) {
		t.Fatalf("SetStatus() status tidak sah = %v, ingin ErrStatusInvalid", err)
	}

	if _, err := s.SetStatus(ctx, "aktor-lain", admin.ID, StatusSuspended); !errors.Is(err, ErrLastAdminProtected) {
		t.Fatalf("SetStatus() admin aktif terakhir = %v, ingin ErrLastAdminProtected", err)
	}
	if got := hitung(t, db, `SELECT COUNT(*) FROM users WHERE status = 'active'`); got != 1 {
		t.Errorf("jumlah pengguna aktif = %d, ingin tetap 1", got)
	}
	if _, err := s.SetStatus(ctx, "aktor-lain", "tidak-ada", StatusSuspended); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SetStatus() pengguna tidak ada = %v, ingin ErrNotFound", err)
	}

	admin2 := undangAdminKedua(t, s, db, admin)
	if _, err := s.SetStatus(ctx, admin2.ID, admin2.ID, StatusSuspended); !errors.Is(err, ErrSelfAccountProtected) {
		t.Fatalf("SetStatus() akun sendiri dengan admin lain tersedia = %v, ingin ErrSelfAccountProtected", err)
	}
	if _, err := s.SetStatus(ctx, admin2.ID, admin.ID, StatusSuspended); err != nil {
		t.Fatalf("SetStatus() dengan admin lain tersedia gagal: %v", err)
	}
}

func TestUserByIDDanListUsers(t *testing.T) {
	s, _, _ := bukaStore(t)
	ctx := context.Background()
	admin := undangAdmin(t, s)

	dosen, err := s.InviteUser(ctx, "dosen@example.com", "Dosen Satu", RoleDosen, admin.ID)
	if err != nil {
		t.Fatalf("InviteUser() gagal: %v", err)
	}
	mahasiswa, err := s.InviteUser(ctx, "mhs@example.com", "Mhs Satu", RoleMahasiswa, admin.ID)
	if err != nil {
		t.Fatalf("InviteUser() gagal: %v", err)
	}

	dibaca, err := s.UserByID(ctx, dosen.ID)
	if err != nil {
		t.Fatalf("UserByID() gagal: %v", err)
	}
	if dibaca.Email != "dosen@example.com" || dibaca.Role != RoleDosen {
		t.Errorf("UserByID() = %+v, ingin dosen@example.com/dosen", dibaca)
	}
	if _, err := s.UserByID(ctx, "tidak-ada"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UserByID() tidak ada = %v, ingin ErrNotFound", err)
	}

	semua, err := s.ListUsers(ctx, "", "", 0)
	if err != nil {
		t.Fatalf("ListUsers() gagal: %v", err)
	}
	if len(semua) != 3 {
		t.Errorf("ListUsers() = %d baris, ingin 3", len(semua))
	}

	hanyaDosen, err := s.ListUsers(ctx, RoleDosen, "", 10)
	if err != nil {
		t.Fatalf("ListUsers() role gagal: %v", err)
	}
	if len(hanyaDosen) != 1 || hanyaDosen[0].ID != dosen.ID {
		t.Errorf("ListUsers(dosen) = %+v, ingin hanya %s", hanyaDosen, dosen.ID)
	}

	undangan, err := s.ListUsers(ctx, "", StatusInvited, 10)
	if err != nil {
		t.Fatalf("ListUsers() status gagal: %v", err)
	}
	if len(undangan) != 3 {
		t.Errorf("ListUsers(invited) = %d baris, ingin 3", len(undangan))
	}

	if _, err := s.ListUsers(ctx, Role("operator"), "", 10); !errors.Is(err, ErrRoleInvalid) {
		t.Fatalf("ListUsers() role tidak sah = %v, ingin ErrRoleInvalid", err)
	}
	if _, err := s.ListUsers(ctx, "", Status("disabled"), 10); !errors.Is(err, ErrStatusInvalid) {
		t.Fatalf("ListUsers() status tidak sah = %v, ingin ErrStatusInvalid", err)
	}

	hanyaMahasiswa, err := s.ListUsers(ctx, RoleMahasiswa, StatusInvited, 10)
	if err != nil {
		t.Fatalf("ListUsers() gabungan gagal: %v", err)
	}
	if len(hanyaMahasiswa) != 1 || hanyaMahasiswa[0].ID != mahasiswa.ID {
		t.Errorf("ListUsers(mahasiswa, invited) = %+v, ingin hanya %s", hanyaMahasiswa, mahasiswa.ID)
	}

	// Batas limit dihormati.
	if terbatas, err := s.ListUsers(ctx, "", "", 1); err != nil || len(terbatas) != 1 {
		t.Errorf("ListUsers(limit=1) = (%d baris, %v), ingin 1 baris tanpa galat", len(terbatas), err)
	}
}

func TestLinkInvitedAccountBerhasil(t *testing.T) {
	s, db, _ := bukaStore(t)
	ctx := context.Background()
	admin := undangAdmin(t, s)

	undangan, err := s.InviteUser(ctx, "mahasiswa@example.com", "Mhs Satu", RoleMahasiswa, admin.ID)
	if err != nil {
		t.Fatalf("InviteUser() gagal: %v", err)
	}

	user, err := s.LinkInvitedAccount(ctx, "subjek-google-1", "  MAHASISWA@example.com ", true)
	if err != nil {
		t.Fatalf("LinkInvitedAccount() gagal: %v", err)
	}
	if user.ID != undangan.ID {
		t.Errorf("id pengguna = %q, ingin %q", user.ID, undangan.ID)
	}
	if user.Status != StatusActive {
		t.Errorf("status = %q, ingin %q", user.Status, StatusActive)
	}
	if user.LastLoginAt == nil {
		t.Fatal("last_login_at kosong setelah penautan")
	}
	if !user.LastLoginAt.Equal(waktuUji) {
		t.Errorf("last_login_at = %v, ingin %v", user.LastLoginAt, waktuUji)
	}

	ident, ada, err := s.IdentityForUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("IdentityForUser() gagal: %v", err)
	}
	if !ada {
		t.Fatal("identitas tidak ditemukan setelah penautan")
	}
	if ident.Subject != "subjek-google-1" || ident.Provider != providerGoogle {
		t.Errorf("identitas = %+v, ingin subjek google", ident)
	}
	if ident.EmailAtProvider != "mahasiswa@example.com" {
		t.Errorf("email_at_provider = %q, ingin dinormalkan", ident.EmailAtProvider)
	}
	if ident.UserID != user.ID {
		t.Errorf("user_id identitas = %q, ingin %q", ident.UserID, user.ID)
	}

	if got := hitung(t, db, `SELECT COUNT(*) FROM user_identities`); got != 1 {
		t.Errorf("jumlah identitas = %d, ingin 1", got)
	}

	// Akun tanpa identitas mengembalikan false tanpa galat.
	if _, ada, err := s.IdentityForUser(ctx, admin.ID); err != nil || ada {
		t.Errorf("IdentityForUser() akun tanpa identitas = (%v, %v), ingin (false, nil)", ada, err)
	}
}

func TestLinkInvitedAccountMenolakEmailTidakDikenal(t *testing.T) {
	s, db, _ := bukaStore(t)
	ctx := context.Background()
	undangAdmin(t, s)

	sebelum := hitung(t, db, `SELECT COUNT(*) FROM users`)

	_, err := s.LinkInvitedAccount(ctx, "subjek-asing", "orang-asing@example.com", true)
	if !errors.Is(err, ErrNotInvited) {
		t.Fatalf("LinkInvitedAccount() email tidak dikenal = %v, ingin ErrNotInvited", err)
	}
	if sesudah := hitung(t, db, `SELECT COUNT(*) FROM users`); sesudah != sebelum {
		t.Errorf("jumlah pengguna berubah dari %d menjadi %d; akun baru tidak boleh dibuat", sebelum, sesudah)
	}
	if got := hitung(t, db, `SELECT COUNT(*) FROM user_identities`); got != 0 {
		t.Errorf("jumlah identitas = %d, ingin 0", got)
	}
}

func TestLinkInvitedAccountMenolakEmailBelumDiverifikasi(t *testing.T) {
	s, db, _ := bukaStore(t)
	ctx := context.Background()
	admin := undangAdmin(t, s)

	undangan, err := s.InviteUser(ctx, "mahasiswa@example.com", "Mhs Satu", RoleMahasiswa, admin.ID)
	if err != nil {
		t.Fatalf("InviteUser() gagal: %v", err)
	}

	if _, err := s.LinkInvitedAccount(ctx, "subjek-1", "mahasiswa@example.com", false); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("LinkInvitedAccount() email_verified=false = %v, ingin ErrInvalidInput", err)
	}

	// Akun tetap `invited` dan tanpa identitas: penolakan tidak boleh menyisakan
	// perubahan separuh jalan.
	user, err := s.UserByID(ctx, undangan.ID)
	if err != nil {
		t.Fatalf("UserByID() gagal: %v", err)
	}
	if user.Status != StatusInvited {
		t.Errorf("status = %q, ingin tetap %q", user.Status, StatusInvited)
	}
	if got := hitung(t, db, `SELECT COUNT(*) FROM user_identities`); got != 0 {
		t.Errorf("jumlah identitas = %d, ingin 0", got)
	}
}

func TestLinkInvitedAccountMenolakSubjectMilikAkunLain(t *testing.T) {
	s, db, _ := bukaStore(t)
	ctx := context.Background()
	admin := undangAdmin(t, s)

	pertama, err := s.InviteUser(ctx, "satu@example.com", "Satu", RoleMahasiswa, admin.ID)
	if err != nil {
		t.Fatalf("InviteUser() gagal: %v", err)
	}
	kedua, err := s.InviteUser(ctx, "dua@example.com", "Dua", RoleMahasiswa, admin.ID)
	if err != nil {
		t.Fatalf("InviteUser() gagal: %v", err)
	}

	if _, err := s.LinkInvitedAccount(ctx, "subjek-bersama", "satu@example.com", true); err != nil {
		t.Fatalf("LinkInvitedAccount() pertama gagal: %v", err)
	}

	if _, err := s.LinkInvitedAccount(ctx, "subjek-bersama", "dua@example.com", true); !errors.Is(err, ErrIdentityTaken) {
		t.Fatalf("LinkInvitedAccount() subjek sama = %v, ingin ErrIdentityTaken", err)
	}

	// Subjek yang gagal tidak boleh menghasilkan baris, dan akun kedua tetap
	// `invited` tanpa identitas.
	if got := hitung(t, db, `SELECT COUNT(*) FROM user_identities`); got != 1 {
		t.Errorf("jumlah identitas = %d, ingin 1", got)
	}
	dua, err := s.UserByID(ctx, kedua.ID)
	if err != nil {
		t.Fatalf("UserByID() gagal: %v", err)
	}
	if dua.Status != StatusInvited {
		t.Errorf("status akun kedua = %q, ingin tetap %q", dua.Status, StatusInvited)
	}

	// Akun pertama tetap `active` dengan identitasnya utuh.
	pertamaSesudah, err := s.UserByID(ctx, pertama.ID)
	if err != nil {
		t.Fatalf("UserByID() akun pertama gagal: %v", err)
	}
	if pertamaSesudah.Status != StatusActive {
		t.Errorf("status akun pertama = %q, ingin %q", pertamaSesudah.Status, StatusActive)
	}
	if _, ada, err := s.IdentityForUser(ctx, pertama.ID); err != nil || !ada {
		t.Errorf("IdentityForUser() akun pertama = (%v, %v), ingin identitas ada", ada, err)
	}
}

func TestLinkInvitedAccountMenolakPenautanKedua(t *testing.T) {
	s, db, _ := bukaStore(t)
	ctx := context.Background()
	admin := undangAdmin(t, s)

	undangan, err := s.InviteUser(ctx, "mahasiswa@example.com", "Mhs Satu", RoleMahasiswa, admin.ID)
	if err != nil {
		t.Fatalf("InviteUser() gagal: %v", err)
	}
	if _, err := s.LinkInvitedAccount(ctx, "subjek-1", "mahasiswa@example.com", true); err != nil {
		t.Fatalf("LinkInvitedAccount() pertama gagal: %v", err)
	}

	// Subjek yang sama untuk akun yang sama tidak boleh menggandakan tautan.
	if _, err := s.LinkInvitedAccount(ctx, "subjek-1", "mahasiswa@example.com", true); !errors.Is(err, ErrAlreadyLinked) {
		t.Fatalf("LinkInvitedAccount() kedua (subjek sama) = %v, ingin ErrAlreadyLinked", err)
	}
	// Subjek berbeda untuk akun yang sudah tertaut juga ditolak: satu akun hanya
	// memiliki satu identitas per penyedia.
	if _, err := s.LinkInvitedAccount(ctx, "subjek-2", "mahasiswa@example.com", true); !errors.Is(err, ErrAlreadyLinked) {
		t.Fatalf("LinkInvitedAccount() kedua (subjek lain) = %v, ingin ErrAlreadyLinked", err)
	}
	if got := hitung(t, db, `SELECT COUNT(*) FROM user_identities WHERE user_id = ?`, undangan.ID); got != 1 {
		t.Errorf("jumlah identitas akun = %d, ingin 1", got)
	}
}

func TestLinkInvitedAccountBersamaan(t *testing.T) {
	// Jumlah goroutine dibuat lebih dari dua agar penautan benar-benar
	// berdesakan pada satu baris undangan; dengan dua goroutine, penjadwalan
	// yang berurutan dapat menyembunyikan balapan.
	const jumlahGoroutine = 8

	t.Run("subjek berbeda", func(t *testing.T) {
		s, db, _ := bukaStore(t)
		ctx := context.Background()
		admin := undangAdmin(t, s)

		undangan, err := s.InviteUser(ctx, "mahasiswa@example.com", "Mhs Satu", RoleMahasiswa, admin.ID)
		if err != nil {
			t.Fatalf("InviteUser() gagal: %v", err)
		}

		subjek := make([]string, jumlahGoroutine)
		for i := range subjek {
			subjek[i] = fmt.Sprintf("subjek-%d", i)
		}
		hasil := jalankanBersamaan(ctx, s, subjek, "mahasiswa@example.com")

		sukses := periksaHasilPenautan(t, hasil)
		if sukses != 1 {
			t.Fatalf("jumlah penautan berhasil = %d, ingin tepat 1", sukses)
		}
		if got := hitung(t, db, `SELECT COUNT(*) FROM user_identities WHERE user_id = ?`, undangan.ID); got != 1 {
			t.Errorf("jumlah identitas = %d, ingin 1", got)
		}
		if got := hitung(t, db, `SELECT COUNT(*) FROM users WHERE status = 'active'`); got != 1 {
			t.Errorf("jumlah akun aktif = %d, ingin 1", got)
		}
	})

	t.Run("subjek sama", func(t *testing.T) {
		s, db, _ := bukaStore(t)
		ctx := context.Background()
		admin := undangAdmin(t, s)

		undangan, err := s.InviteUser(ctx, "mahasiswa@example.com", "Mhs Satu", RoleMahasiswa, admin.ID)
		if err != nil {
			t.Fatalf("InviteUser() gagal: %v", err)
		}

		subjek := make([]string, jumlahGoroutine)
		for i := range subjek {
			subjek[i] = "subjek-sama"
		}
		hasil := jalankanBersamaan(ctx, s, subjek, "mahasiswa@example.com")

		sukses := periksaHasilPenautan(t, hasil)
		if sukses != 1 {
			t.Fatalf("jumlah penautan berhasil = %d, ingin tepat 1", sukses)
		}
		if got := hitung(t, db, `SELECT COUNT(*) FROM user_identities WHERE user_id = ? AND subject = ?`,
			undangan.ID, "subjek-sama"); got != 1 {
			t.Errorf("jumlah baris identitas untuk subjek sama = %d, ingin 1", got)
		}
		if got := hitung(t, db, `SELECT COUNT(*) FROM user_identities`); got != 1 {
			t.Errorf("jumlah identitas = %d, ingin 1", got)
		}
	})
}

// jalankanBersamaan menjalankan LinkInvitedAccount dari beberapa goroutine yang
// dilepas pada saat yang sama. Penyerentakan pelepasannya penting: tanpa itu,
// goroutine pertama dapat selesai sebelum yang lain mulai dan balapan tidak
// pernah benar-benar terjadi.
func jalankanBersamaan(ctx context.Context, s *Store, subjek []string, email string) []error {
	hasil := make([]error, len(subjek))
	var wg sync.WaitGroup
	mulai := make(chan struct{})
	for i := range subjek {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-mulai
			_, hasil[i] = s.LinkInvitedAccount(ctx, subjek[i], email, true)
		}(i)
	}
	close(mulai)
	wg.Wait()
	return hasil
}

// periksaHasilPenautan memastikan hanya satu goroutine yang berhasil dan
// sisanya gagal dengan sentinel penautan, bukan galat mentah database.
func periksaHasilPenautan(t *testing.T, hasil []error) int {
	t.Helper()
	sukses := 0
	for i, err := range hasil {
		if err == nil {
			sukses++
			continue
		}
		if !errors.Is(err, ErrAlreadyLinked) && !errors.Is(err, ErrIdentityTaken) {
			t.Errorf("galat goroutine %d = %v, ingin ErrAlreadyLinked atau ErrIdentityTaken", i, err)
		}
	}
	return sukses
}

func TestNonceSekaliPakaiDanKedaluwarsa(t *testing.T) {
	s, _, _ := bukaStore(t)
	ctx := context.Background()

	if err := s.StoreNonce(ctx, "nonce-1"); err != nil {
		t.Fatalf("StoreNonce() gagal: %v", err)
	}
	if err := s.StoreNonce(ctx, "   "); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("StoreNonce() nonce kosong = %v, ingin ErrInvalidInput", err)
	}

	// Pemakaian pertama berhasil, pemakaian kedua ditolak sebagai tidak
	// terdaftar (sudah dipakai dan barisnya terhapus).
	if err := s.ConsumeNonce(ctx, "nonce-1"); err != nil {
		t.Fatalf("ConsumeNonce() pertama = %v, ingin nil", err)
	}
	if err := s.ConsumeNonce(ctx, "nonce-1"); !errors.Is(err, ErrNonceNotFound) {
		t.Fatalf("ConsumeNonce() kedua = %v, ingin ErrNonceNotFound", err)
	}

	// Nonce yang tidak pernah didaftarkan ditolak.
	if err := s.ConsumeNonce(ctx, "nonce-tidak-ada"); !errors.Is(err, ErrNonceNotFound) {
		t.Errorf("ConsumeNonce() nonce asing = %v, ingin ErrNonceNotFound", err)
	}
	if err := s.ConsumeNonce(ctx, "  "); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("ConsumeNonce() nonce kosong = %v, ingin ErrInvalidInput", err)
	}
}

// TestNonceKedaluwarsaTidakDihidupkanKembali adalah inti perbaikan FL-011:
// nonce yang masa simpannya lewat tidak boleh diterima, dan nonce yang sudah
// dipakai tidak boleh didaftarkan ulang oleh permintaan berikutnya. Tanpa
// aturan ini, ID token yang bocor dapat diputar ulang setelah TTL nonce lewat
// (temuan audit keamanan K-01).
func TestNonceKedaluwarsaTidakDihidupkanKembali(t *testing.T) {
	s, db, jam := bukaStore(t)
	ctx := context.Background()

	if err := s.StoreNonce(ctx, "nonce-kedaluwarsa"); err != nil {
		t.Fatalf("StoreNonce() gagal: %v", err)
	}
	jam.Advance(masaSimpanNonce + time.Minute)

	if err := s.ConsumeNonce(ctx, "nonce-kedaluwarsa"); !errors.Is(err, ErrNonceNotFound) {
		t.Fatalf("ConsumeNonce() setelah masa simpan lewat = %v, ingin ErrNonceNotFound", err)
	}

	// Pendaftaran ulang nonce yang sama tidak menghidupkannya kembali: barisnya
	// masih ada (masa simpan lebih panjang daripada TTL pemakaian), sehingga
	// ON CONFLICT DO NOTHING membiarkannya kedaluwarsa.
	if err := s.StoreNonce(ctx, "nonce-kedaluwarsa"); err != nil {
		t.Fatalf("StoreNonce() ulang gagal: %v", err)
	}
	if err := s.ConsumeNonce(ctx, "nonce-kedaluwarsa"); !errors.Is(err, ErrNonceNotFound) {
		t.Fatalf("ConsumeNonce() setelah pendaftaran ulang = %v, ingin tetap ErrNonceNotFound", err)
	}

	// Nonce yang sudah dipakai tidak berlaku lagi: barisnya terhapus oleh
	// pemakaian, dan pendaftaran ulang hanya mungkin lewat endpoint nonce yang
	// selalu menerbitkan nilai acak baru — nilai nonce lama tidak pernah dapat
	// didaftarkan kembali oleh klien, sehingga jalur login tidak dapat
	// menghidupkan nonce yang sudah dipakai.
	if got := hitung(t, db, `SELECT COUNT(*) FROM auth_nonces WHERE nonce_hash = ?`, NonceHash("nonce-terpakai")); got != 0 {
		t.Errorf("baris nonce terpakai masih ada = %d, ingin 0", got)
	}

	// Tepi TTL: nonce yang tepat pada expires_at tidak boleh diterima.
	if err := s.StoreNonce(ctx, "nonce-tepi"); err != nil {
		t.Fatalf("StoreNonce() gagal: %v", err)
	}
	jam.Advance(TTLNonce)
	if err := s.ConsumeNonce(ctx, "nonce-tepi"); !errors.Is(err, ErrNonceNotFound) {
		t.Errorf("ConsumeNonce() tepat pada batas pemakaian = %v, ingin ErrNonceNotFound", err)
	}
}

// TestNonceTersimpanSebagaiHash menegaskan bentuk yang boleh menyentuh database:
// hash SHA-256, bukan nilai nonce.
func TestNonceTersimpanSebagaiHash(t *testing.T) {
	s, db, _ := bukaStore(t)
	ctx := context.Background()

	const rahasia = "nonce-rahasia-yang-tidak-boleh-tersimpan"
	if err := s.StoreNonce(ctx, rahasia); err != nil {
		t.Fatalf("StoreNonce() gagal: %v", err)
	}

	var tersimpan string
	if err := db.QueryRowContext(ctx, `SELECT nonce_hash FROM auth_nonces`).Scan(&tersimpan); err != nil {
		t.Fatalf("pembacaan nonce_hash gagal: %v", err)
	}
	if tersimpan == rahasia {
		t.Fatal("nilai nonce tersimpan mentah di database")
	}
	if tersimpan != NonceHash(rahasia) {
		t.Errorf("nonce_hash = %q, ingin SHA-256 dari nonce", tersimpan)
	}
}

func TestCleanupExpiredNonces(t *testing.T) {
	s, db, jam := bukaStore(t)
	ctx := context.Background()

	if err := s.StoreNonce(ctx, "nonce-pendek"); err != nil {
		t.Fatalf("StoreNonce() gagal: %v", err)
	}
	// Baris kedua didaftarkan setelah waktu maju, sehingga masa simpannya
	// berakhir lebih lambat daripada baris pertama.
	jam.Advance(masaSimpanNonce + time.Minute)
	if err := s.StoreNonce(ctx, "nonce-panjang"); err != nil {
		t.Fatalf("StoreNonce() gagal: %v", err)
	}

	jumlah, err := s.CleanupExpiredNonces(ctx)
	if err != nil {
		t.Fatalf("CleanupExpiredNonces() gagal: %v", err)
	}
	if jumlah != 1 {
		t.Errorf("jumlah nonce terhapus = %d, ingin 1", jumlah)
	}
	if got := hitung(t, db, `SELECT COUNT(*) FROM auth_nonces`); got != 1 {
		t.Errorf("jumlah nonce tersisa = %d, ingin 1", got)
	}

	// Nonce yang masih berlaku tetap dapat dipakai setelah pembersihan.
	if err := s.ConsumeNonce(ctx, "nonce-panjang"); err != nil {
		t.Errorf("ConsumeNonce() setelah pembersihan = %v, ingin nil", err)
	}
}

func TestAssignLecturerDanCakupan(t *testing.T) {
	s, db, _ := bukaStore(t)
	ctx := context.Background()
	admin := undangAdmin(t, s)

	dosen, err := s.InviteUser(ctx, "dosen@example.com", "Dosen Satu", RoleDosen, admin.ID)
	if err != nil {
		t.Fatalf("InviteUser() gagal: %v", err)
	}
	mahasiswa, err := s.InviteUser(ctx, "mhs@example.com", "Mhs Satu", RoleMahasiswa, admin.ID)
	if err != nil {
		t.Fatalf("InviteUser() gagal: %v", err)
	}
	buatKelas(t, db, "kelas-1", "IF101", "Pengantar Informatika")
	buatKelas(t, db, "kelas-2", "IF102", "Struktur Data")

	terassign, err := s.IsLecturerAssigned(ctx, dosen.ID, "kelas-1")
	if err != nil {
		t.Fatalf("IsLecturerAssigned() gagal: %v", err)
	}
	if terassign {
		t.Error("IsLecturerAssigned() sebelum penugasan = true, ingin false")
	}

	if err := s.AssignLecturer(ctx, admin.ID, "kelas-1", dosen.ID); err != nil {
		t.Fatalf("AssignLecturer() gagal: %v", err)
	}
	// Penugasan ganda idempoten.
	if err := s.AssignLecturer(ctx, admin.ID, "kelas-1", dosen.ID); err != nil {
		t.Fatalf("AssignLecturer() ulang gagal: %v", err)
	}
	if got := hitung(t, db, `SELECT COUNT(*) FROM class_lecturers WHERE class_id = 'kelas-1'`); got != 1 {
		t.Errorf("jumlah penugasan = %d, ingin 1", got)
	}
	if err := s.AssignLecturer(ctx, admin.ID, "kelas-2", dosen.ID); err != nil {
		t.Fatalf("AssignLecturer() kelas kedua gagal: %v", err)
	}

	terassign, err = s.IsLecturerAssigned(ctx, dosen.ID, "kelas-1")
	if err != nil {
		t.Fatalf("IsLecturerAssigned() gagal: %v", err)
	}
	if !terassign {
		t.Error("IsLecturerAssigned() setelah penugasan = false, ingin true")
	}
	if terassign, err := s.IsLecturerAssigned(ctx, dosen.ID, "kelas-tidak-ada"); err != nil || terassign {
		t.Errorf("IsLecturerAssigned() kelas asing = (%v, %v), ingin (false, nil)", terassign, err)
	}

	// Role selain dosen ditolak.
	if err := s.AssignLecturer(ctx, admin.ID, "kelas-1", mahasiswa.ID); !errors.Is(err, ErrRoleInvalid) {
		t.Fatalf("AssignLecturer() mahasiswa = %v, ingin ErrRoleInvalid", err)
	}
	if err := s.AssignLecturer(ctx, admin.ID, "kelas-1", admin.ID); !errors.Is(err, ErrRoleInvalid) {
		t.Fatalf("AssignLecturer() admin = %v, ingin ErrRoleInvalid", err)
	}
	// Kelas dan pengguna harus ada.
	if err := s.AssignLecturer(ctx, admin.ID, "kelas-tidak-ada", dosen.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("AssignLecturer() kelas tidak ada = %v, ingin ErrNotFound", err)
	}
	if err := s.AssignLecturer(ctx, admin.ID, "kelas-1", "dosen-tidak-ada"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("AssignLecturer() dosen tidak ada = %v, ingin ErrNotFound", err)
	}
	if err := s.AssignLecturer(ctx, "", "kelas-1", dosen.ID); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("AssignLecturer() aktor kosong = %v, ingin ErrInvalidInput", err)
	}

	daftar, err := s.LecturersForClass(ctx, "kelas-1")
	if err != nil {
		t.Fatalf("LecturersForClass() gagal: %v", err)
	}
	if len(daftar) != 1 || daftar[0].ID != dosen.ID {
		t.Fatalf("LecturersForClass() = %+v, ingin hanya %s", daftar, dosen.ID)
	}
	if daftar[0].Role != RoleDosen {
		t.Errorf("role dosen = %q, ingin %q", daftar[0].Role, RoleDosen)
	}

	kelas, err := s.ClassesForLecturer(ctx, dosen.ID)
	if err != nil {
		t.Fatalf("ClassesForLecturer() gagal: %v", err)
	}
	if len(kelas) != 2 {
		t.Fatalf("ClassesForLecturer() = %d kelas, ingin 2", len(kelas))
	}
	if kelas[0].ID != "kelas-1" || kelas[0].Code != "IF101" || kelas[0].Title != "Pengantar Informatika" {
		t.Errorf("kelas pertama = %+v, ingin kelas-1/IF101", kelas[0])
	}
	if kosong, err := s.ClassesForLecturer(ctx, mahasiswa.ID); err != nil || len(kosong) != 0 {
		t.Errorf("ClassesForLecturer() mahasiswa = (%v, %v), ingin kosong tanpa galat", kosong, err)
	}

	if err := s.RemoveLecturer(ctx, "kelas-2", dosen.ID); err != nil {
		t.Fatalf("RemoveLecturer() gagal: %v", err)
	}
	if err := s.RemoveLecturer(ctx, "kelas-2", dosen.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("RemoveLecturer() ulang = %v, ingin ErrNotFound", err)
	}
	if err := s.RemoveLecturer(ctx, "kelas-1", dosen.ID); err != nil {
		t.Fatalf("RemoveLecturer() gagal: %v", err)
	}
	if terassign, err := s.IsLecturerAssigned(ctx, dosen.ID, "kelas-1"); err != nil || terassign {
		t.Errorf("IsLecturerAssigned() setelah pencabutan = (%v, %v), ingin (false, nil)", terassign, err)
	}
}

func TestLinkPersonAccountHanyaMahasiswaDanTolakGanda(t *testing.T) {
	s, db, _ := bukaStore(t)
	ctx := context.Background()
	admin := undangAdmin(t, s)

	mahasiswa, err := s.InviteUser(ctx, "mhs@example.com", "Mhs Satu", RoleMahasiswa, admin.ID)
	if err != nil {
		t.Fatalf("InviteUser() gagal: %v", err)
	}
	mahasiswa2, err := s.InviteUser(ctx, "mhs2@example.com", "Mhs Dua", RoleMahasiswa, admin.ID)
	if err != nil {
		t.Fatalf("InviteUser() gagal: %v", err)
	}
	dosen, err := s.InviteUser(ctx, "dosen@example.com", "Dosen Satu", RoleDosen, admin.ID)
	if err != nil {
		t.Fatalf("InviteUser() gagal: %v", err)
	}
	buatPeserta(t, db, "orang-1", "2026001", "Peserta Satu")
	buatPeserta(t, db, "orang-2", "2026002", "Peserta Dua")

	if _, ada, err := s.PersonForUser(ctx, mahasiswa.ID); err != nil || ada {
		t.Fatalf("PersonForUser() sebelum pengaitan = (%v, %v), ingin (false, nil)", ada, err)
	}

	if err := s.LinkPersonAccount(ctx, admin.ID, "orang-1", mahasiswa.ID); err != nil {
		t.Fatalf("LinkPersonAccount() gagal: %v", err)
	}
	personID, ada, err := s.PersonForUser(ctx, mahasiswa.ID)
	if err != nil {
		t.Fatalf("PersonForUser() gagal: %v", err)
	}
	if !ada || personID != "orang-1" {
		t.Fatalf("PersonForUser() = (%q, %v), ingin (orang-1, true)", personID, ada)
	}
	// Pengaitan yang sama diulang tidak mengubah apa pun.
	if err := s.LinkPersonAccount(ctx, admin.ID, "orang-1", mahasiswa.ID); err != nil {
		t.Fatalf("LinkPersonAccount() ulang gagal: %v", err)
	}

	// Hanya mahasiswa yang boleh dikaitkan.
	if err := s.LinkPersonAccount(ctx, admin.ID, "orang-2", dosen.ID); !errors.Is(err, ErrNotMahasiswa) {
		t.Fatalf("LinkPersonAccount() dosen = %v, ingin ErrNotMahasiswa", err)
	}
	if err := s.LinkPersonAccount(ctx, admin.ID, "orang-2", admin.ID); !errors.Is(err, ErrNotMahasiswa) {
		t.Fatalf("LinkPersonAccount() admin = %v, ingin ErrNotMahasiswa", err)
	}

	// Satu peserta hanya untuk satu akun.
	if err := s.LinkPersonAccount(ctx, admin.ID, "orang-1", mahasiswa2.ID); !errors.Is(err, ErrAlreadyLinked) {
		t.Fatalf("LinkPersonAccount() peserta sudah tertaut = %v, ingin ErrAlreadyLinked", err)
	}
	// Satu akun hanya untuk satu peserta.
	if err := s.LinkPersonAccount(ctx, admin.ID, "orang-2", mahasiswa.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("LinkPersonAccount() akun sudah tertaut = %v, ingin ErrConflict", err)
	}
	// Peserta harus ada.
	if err := s.LinkPersonAccount(ctx, admin.ID, "orang-tidak-ada", mahasiswa2.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("LinkPersonAccount() peserta tidak ada = %v, ingin ErrNotFound", err)
	}
	if err := s.LinkPersonAccount(ctx, admin.ID, "orang-2", "akun-tidak-ada"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("LinkPersonAccount() akun tidak ada = %v, ingin ErrNotFound", err)
	}
	if err := s.LinkPersonAccount(ctx, "", "orang-2", mahasiswa2.ID); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("LinkPersonAccount() aktor kosong = %v, ingin ErrInvalidInput", err)
	}

	if got := hitung(t, db, `SELECT COUNT(*) FROM people WHERE account_user_id IS NOT NULL`); got != 1 {
		t.Errorf("jumlah peserta tertaut = %d, ingin 1", got)
	}

	if err := s.UnlinkPersonAccount(ctx, admin.ID, "orang-1"); err != nil {
		t.Fatalf("UnlinkPersonAccount() gagal: %v", err)
	}
	if _, ada, err := s.PersonForUser(ctx, mahasiswa.ID); err != nil || ada {
		t.Errorf("PersonForUser() setelah pelepasan = (%v, %v), ingin (false, nil)", ada, err)
	}
	if err := s.UnlinkPersonAccount(ctx, admin.ID, "orang-tidak-ada"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UnlinkPersonAccount() peserta tidak ada = %v, ingin ErrNotFound", err)
	}
	if err := s.UnlinkPersonAccount(ctx, "", "orang-1"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("UnlinkPersonAccount() aktor kosong = %v, ingin ErrInvalidInput", err)
	}
}

func TestTouchLogin(t *testing.T) {
	s, _, jam := bukaStore(t)
	ctx := context.Background()
	admin := undangAdmin(t, s)

	if err := s.TouchLogin(ctx, admin.ID); err != nil {
		t.Fatalf("TouchLogin() gagal: %v", err)
	}
	jam.Advance(3 * time.Hour)
	if err := s.TouchLogin(ctx, admin.ID); err != nil {
		t.Fatalf("TouchLogin() kedua gagal: %v", err)
	}

	user, err := s.UserByID(ctx, admin.ID)
	if err != nil {
		t.Fatalf("UserByID() gagal: %v", err)
	}
	if user.LastLoginAt == nil {
		t.Fatal("last_login_at kosong setelah TouchLogin")
	}
	if ingin := waktuUji.Add(3 * time.Hour); !user.LastLoginAt.Equal(ingin) {
		t.Errorf("last_login_at = %v, ingin %v", user.LastLoginAt, ingin)
	}

	if err := s.TouchLogin(ctx, "tidak-ada"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("TouchLogin() pengguna tidak ada = %v, ingin ErrNotFound", err)
	}
	if err := s.TouchLogin(ctx, ""); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("TouchLogin() id kosong = %v, ingin ErrInvalidInput", err)
	}
}

func TestRoleDanStatusValid(t *testing.T) {
	for _, r := range []Role{RoleAdmin, RoleDosen, RoleMahasiswa} {
		if !r.Valid() {
			t.Errorf("Role(%q).Valid() = false, ingin true", r)
		}
	}
	for _, r := range []Role{"", "operator", "device", "ADMIN", "superadmin"} {
		if Role(r).Valid() {
			t.Errorf("Role(%q).Valid() = true, ingin false", r)
		}
	}
	for _, st := range []Status{StatusInvited, StatusActive, StatusSuspended} {
		if !st.Valid() {
			t.Errorf("Status(%q).Valid() = false, ingin true", st)
		}
	}
	for _, st := range []Status{"", "disabled", "ACTIVE", "pending"} {
		if Status(st).Valid() {
			t.Errorf("Status(%q).Valid() = true, ingin false", st)
		}
	}
}

func TestUserBySubject(t *testing.T) {
	s, _, _ := bukaStore(t)
	ctx := context.Background()
	admin := undangAdmin(t, s)

	undangan, err := s.InviteUser(ctx, "mahasiswa@example.com", "Mhs Satu", RoleMahasiswa, admin.ID)
	if err != nil {
		t.Fatalf("InviteUser() gagal: %v", err)
	}

	// Subjek yang belum tertaut bukan keadaan galat.
	if _, ada, err := s.UserBySubject(ctx, "subjek-belum-ada"); err != nil || ada {
		t.Fatalf("UserBySubject() sebelum penautan = (%v, %v), ingin (false, nil)", ada, err)
	}
	if _, _, err := s.UserBySubject(ctx, "   "); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("UserBySubject() subjek kosong = %v, ingin ErrInvalidInput", err)
	}

	if _, err := s.LinkInvitedAccount(ctx, "subjek-1", "mahasiswa@example.com", true); err != nil {
		t.Fatalf("LinkInvitedAccount() gagal: %v", err)
	}

	user, ada, err := s.UserBySubject(ctx, "subjek-1")
	if err != nil {
		t.Fatalf("UserBySubject() gagal: %v", err)
	}
	if !ada {
		t.Fatal("UserBySubject() setelah penautan = false, ingin true")
	}
	if user.ID != undangan.ID {
		t.Errorf("id pengguna = %q, ingin %q", user.ID, undangan.ID)
	}
	if user.Status != StatusActive {
		t.Errorf("status = %q, ingin %q", user.Status, StatusActive)
	}
}

func TestSetDisplayName(t *testing.T) {
	s, _, _ := bukaStore(t)
	ctx := context.Background()
	admin := undangAdmin(t, s)

	// Perubahan nama sendiri diizinkan: nama bukan hak akses.
	diperbarui, err := s.SetDisplayName(ctx, admin.ID, admin.ID, "  Nama Baru  ")
	if err != nil {
		t.Fatalf("SetDisplayName() akun sendiri gagal: %v", err)
	}
	if diperbarui.DisplayName != "Nama Baru" {
		t.Errorf("display_name = %q, ingin %q", diperbarui.DisplayName, "Nama Baru")
	}
	if diperbarui.Role != RoleAdmin || diperbarui.Status != StatusInvited {
		t.Errorf("role/status berubah menjadi %q/%q, ingin tetap admin/invited", diperbarui.Role, diperbarui.Status)
	}

	if _, err := s.SetDisplayName(ctx, admin.ID, admin.ID, "   "); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("SetDisplayName() nama kosong = %v, ingin ErrInvalidInput", err)
	}

	// Batas 120 karakter: tepat batas diterima, lebih satu ditolak.
	if _, err := s.SetDisplayName(ctx, admin.ID, admin.ID, strings.Repeat("a", 120)); err != nil {
		t.Fatalf("SetDisplayName() tepat 120 karakter gagal: %v", err)
	}
	if _, err := s.SetDisplayName(ctx, admin.ID, admin.ID, strings.Repeat("a", 121)); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("SetDisplayName() 121 karakter = %v, ingin ErrInvalidInput", err)
	}
	if _, err := s.SetDisplayName(ctx, admin.ID, "tidak-ada", "Nama"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SetDisplayName() pengguna tidak ada = %v, ingin ErrNotFound", err)
	}
	if _, err := s.SetDisplayName(ctx, "", admin.ID, "Nama"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("SetDisplayName() aktor kosong = %v, ingin ErrInvalidInput", err)
	}
}

func TestClassesForPersonMengabaikanKeanggotaanBerakhir(t *testing.T) {
	s, db, _ := bukaStore(t)
	ctx := context.Background()
	admin := undangAdmin(t, s)

	mahasiswa, err := s.InviteUser(ctx, "mhs@example.com", "Mhs Satu", RoleMahasiswa, admin.ID)
	if err != nil {
		t.Fatalf("InviteUser() gagal: %v", err)
	}
	buatPeserta(t, db, "orang-1", "2026001", "Peserta Satu")
	buatPeserta(t, db, "orang-2", "2026002", "Peserta Dua")
	buatKelas(t, db, "kelas-1", "IF101", "Pengantar Informatika")
	buatKelas(t, db, "kelas-2", "IF102", "Struktur Data")
	buatKelas(t, db, "kelas-3", "IF103", "Basis Data")

	if err := s.LinkPersonAccount(ctx, admin.ID, "orang-1", mahasiswa.ID); err != nil {
		t.Fatalf("LinkPersonAccount() gagal: %v", err)
	}

	// Keanggotaan aktif, keanggotaan yang sudah berakhir, dan keanggotaan
	// peserta lain.
	keanggotaan := []struct {
		kelas  string
		orang  string
		leftAt any
	}{
		{"kelas-1", "orang-1", nil},
		{"kelas-2", "orang-1", "2026-10-15T00:00:00.000Z"},
		{"kelas-3", "orang-2", nil},
	}
	for _, k := range keanggotaan {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO class_memberships (class_id, person_id, left_at, created_by) VALUES (?, ?, ?, ?)`,
			k.kelas, k.orang, k.leftAt, admin.ID); err != nil {
			t.Fatalf("penyisipan keanggotaan %s/%s gagal: %v", k.kelas, k.orang, err)
		}
	}

	kelas, err := s.ClassesForPerson(ctx, "orang-1")
	if err != nil {
		t.Fatalf("ClassesForPerson() gagal: %v", err)
	}
	if len(kelas) != 1 {
		t.Fatalf("ClassesForPerson() = %+v, ingin hanya 1 kelas aktif", kelas)
	}
	if kelas[0].ID != "kelas-1" || kelas[0].Code != "IF101" || kelas[0].Title != "Pengantar Informatika" {
		t.Errorf("kelas = %+v, ingin kelas-1/IF101", kelas[0])
	}

	if kosong, err := s.ClassesForPerson(ctx, "orang-2"); err != nil || len(kosong) != 1 {
		t.Errorf("ClassesForPerson(orang-2) = (%+v, %v), ingin 1 kelas", kosong, err)
	}
	if kosong, err := s.ClassesForPerson(ctx, "orang-tidak-ada"); err != nil || len(kosong) != 0 {
		t.Errorf("ClassesForPerson(peserta asing) = (%+v, %v), ingin kosong tanpa galat", kosong, err)
	}
	if _, err := s.ClassesForPerson(ctx, "  "); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("ClassesForPerson() personID kosong = %v, ingin ErrInvalidInput", err)
	}
}

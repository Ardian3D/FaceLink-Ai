// Package auth menyimpan akun, identitas Google, nonce OIDC, penugasan dosen,
// dan pengaitan peserta sesuai D-032 (lifecycle akun), D-033 (nonce sekali
// pakai), D-034 (cakupan dosen), dan D-037 (satu role utama per akun).
//
// Paket ini tidak menyimpan kredensial apa pun: tidak ada kata sandi, tidak ada
// ID token Google, dan nonce hanya disimpan sebagai hash SHA-256. Seluruh
// keputusan otorisasi tetap di lapisan pemanggil; paket ini menegakkan aturan
// keadaan yang tidak boleh dilanggar oleh data (admin terakhir, akun sendiri,
// satu identitas per akun, satu peserta per akun).
package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/audit"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/clock"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
)

// Store adalah satu-satunya pintu masuk perubahan tabel akun. Seluruh penulisan
// berjalan pada koneksi tunggal yang disediakan internal/database sehingga
// SQLite tetap satu penulis (docs/DATA_MODEL.md §6.2).
type Store struct {
	db  *database.DB
	clk clock.Clock
}

// NewStore membuat penyimpanan autentikasi. Jam disuntikkan agar waktu
// kedaluwarsa nonce dan cap waktu baris dapat diuji tanpa tidur panjang.
func NewStore(db *database.DB, clk clock.Clock) *Store {
	if clk == nil {
		clk = clock.Real{}
	}
	return &Store{db: db, clk: clk}
}

// userColumns adalah daftar kolom tabel `users` yang dibaca paket ini; urutannya
// harus sama dengan urutan pemindaian pada scanUser.
const userColumns = `id, email, display_name, role, status, created_by, last_login_at, created_at, updated_at`

// rowScanner adalah bagian baris hasil kueri yang dibutuhkan pemindaian.
type rowScanner interface {
	Scan(dest ...any) error
}

// waktuSekarang mengembalikan waktu sekarang dalam UTC.
func (s *Store) waktuSekarang() time.Time { return s.clk.Now().UTC() }

// normalisasiEmail menyeragamkan penulisan email agar pencocokan undangan pada
// login pertama tidak bergantung pada huruf besar-kecil atau spasi tepi (D-032).
func normalisasiEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// teks mengembalikan teks tanpa spasi tepi.
func teks(nilai string) string { return strings.TrimSpace(nilai) }

// nullable mengubah teks kosong menjadi NULL. Kolom seperti `users.created_by`
// dan `user_identities.email_at_provider` memang boleh kosong; menyimpannya
// sebagai teks kosong akan membingungkan pembaca berikutnya.
func nullable(nilai string) any {
	if teks(nilai) == "" {
		return nil
	}
	return teks(nilai)
}

// scanUser memindai satu baris pengguna. Galat pemindaian dikembalikan apa
// adanya supaya pemanggil dapat mengenali sql.ErrNoRows.
func scanUser(sc rowScanner) (User, error) {
	var (
		u                    User
		role, status         string
		createdBy, lastLogin sql.NullString
		createdAt, updatedAt string
	)
	if err := sc.Scan(&u.ID, &u.Email, &u.DisplayName, &role, &status,
		&createdBy, &lastLogin, &createdAt, &updatedAt); err != nil {
		return User{}, err
	}

	u.Role = Role(role)
	u.Status = Status(status)
	u.CreatedBy = createdBy.String

	created, err := parseWaktu(createdAt)
	if err != nil {
		return User{}, fmt.Errorf("auth: created_at pengguna %s tidak dapat dibaca: %w", u.ID, err)
	}
	u.CreatedAt = created

	updated, err := parseWaktu(updatedAt)
	if err != nil {
		return User{}, fmt.Errorf("auth: updated_at pengguna %s tidak dapat dibaca: %w", u.ID, err)
	}
	u.UpdatedAt = updated

	if lastLogin.Valid && teks(lastLogin.String) != "" {
		login, err := parseWaktu(lastLogin.String)
		if err != nil {
			return User{}, fmt.Errorf("auth: last_login_at pengguna %s tidak dapat dibaca: %w", u.ID, err)
		}
		u.LastLoginAt = &login
	}

	return u, nil
}

// scanIdentity memindai satu baris identitas.
func scanIdentity(sc rowScanner) (Identity, error) {
	var (
		ident               Identity
		emailAtProvider     sql.NullString
		linkedAt, createdAt string
	)
	if err := sc.Scan(&ident.ID, &ident.UserID, &ident.Provider, &ident.Subject,
		&emailAtProvider, &linkedAt, &createdAt); err != nil {
		return Identity{}, err
	}
	ident.EmailAtProvider = emailAtProvider.String

	linked, err := parseWaktu(linkedAt)
	if err != nil {
		return Identity{}, fmt.Errorf("auth: linked_at identitas %s tidak dapat dibaca: %w", ident.ID, err)
	}
	ident.LinkedAt = linked

	created, err := parseWaktu(createdAt)
	if err != nil {
		return Identity{}, fmt.Errorf("auth: created_at identitas %s tidak dapat dibaca: %w", ident.ID, err)
	}
	ident.CreatedAt = created

	return ident, nil
}

// layoutWaktu adalah format kolom teks waktu pada skema (audit.FormatWaktu).
const layoutWaktu = "2006-01-02T15:04:05.000Z"

// parseWaktu membaca waktu ISO 8601 UTC yang disimpan skema. Beberapa varian
// diterima agar baris hasil DEFAULT strftime maupun tulisan audit.FormatWaktu
// sama-sama terbaca.
func parseWaktu(nilai string) (time.Time, error) {
	nilai = teks(nilai)
	if nilai == "" {
		return time.Time{}, errors.New("auth: waktu kosong")
	}
	layouts := []string{layoutWaktu, time.RFC3339Nano, "2006-01-02T15:04:05Z"}
	var lastErr error
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, nilai)
		if err == nil {
			return parsed.UTC(), nil
		}
		lastErr = err
	}
	return time.Time{}, lastErr
}

// formatWaktu menulis waktu dengan format kolom skema.
func formatWaktu(t time.Time) string { return audit.FormatWaktu(t) }

// immediateTx menjalankan fn di dalam transaksi tulis SQLite yang dibuka dengan
// BEGIN IMMEDIATE.
//
// Transaksi deferred akan menaikkan kunci tulis baru pada penulisan pertama,
// sehingga pemeriksaan "baca dahulu, tulis kemudian" dapat berlomba dengan
// penulis lain dan gagal dengan SQLITE_BUSY. BEGIN IMMEDIATE mengambil kunci
// tulis sejak awal, sehingga pemeriksaan keadaan (mis. admin aktif terakhir dan
// penautan identitas) benar-benar atomik terhadap permintaan lain.
func (s *Store) immediateTx(ctx context.Context, fn func(ctx context.Context, conn *sql.Conn) error) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("auth: gagal mengambil koneksi: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("auth: gagal membuka transaksi: %w", err)
	}

	if err := fn(ctx, conn); err != nil {
		if _, rbErr := conn.ExecContext(ctx, "ROLLBACK"); rbErr != nil {
			return fmt.Errorf("%w (pengurungan transaksi gagal: %v)", err, rbErr)
		}
		return err
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		if _, rbErr := conn.ExecContext(ctx, "ROLLBACK"); rbErr != nil {
			return fmt.Errorf("auth: gagal menyimpan transaksi: %w (pengurungan gagal: %v)", err, rbErr)
		}
		return fmt.Errorf("auth: gagal menyimpan transaksi: %w", err)
	}
	return nil
}

// userByIDTx membaca pengguna di dalam transaksi. Baris yang tidak ada dipetakan
// ke ErrNotFound sehingga sql.ErrNoRows tidak pernah keluar dari paket ini.
func userByIDTx(ctx context.Context, conn *sql.Conn, id string) (User, error) {
	u, err := scanUser(conn.QueryRowContext(ctx,
		`SELECT `+userColumns+` FROM users WHERE id = ?`, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, fmt.Errorf("%w: pengguna %s", ErrNotFound, id)
		}
		return User{}, fmt.Errorf("auth: gagal membaca pengguna %s: %w", id, err)
	}
	return u, nil
}

// userByEmailTx membaca pengguna berdasarkan email di dalam transaksi.
func userByEmailTx(ctx context.Context, conn *sql.Conn, email string) (User, error) {
	u, err := scanUser(conn.QueryRowContext(ctx,
		`SELECT `+userColumns+` FROM users WHERE email = ?`, email))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, fmt.Errorf("%w: pengguna dengan email tersebut", ErrNotFound)
		}
		return User{}, fmt.Errorf("auth: gagal membaca pengguna berdasarkan email: %w", err)
	}
	return u, nil
}

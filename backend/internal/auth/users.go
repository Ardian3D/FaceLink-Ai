package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/ids"
)

// InviteUser membuat akun berstatus `invited` (D-032). Akun hanya lahir dari
// undangan: tidak ada pendaftaran mandiri dan tidak ada akun otomatis dari login
// Google. Email dinormalkan menjadi huruf kecil tanpa spasi tepi supaya
// pencocokan pada login pertama tidak bergantung penulisan.
func (s *Store) InviteUser(ctx context.Context, email, displayName string, role Role, createdBy string) (User, error) {
	email = normalisasiEmail(email)
	createdBy = teks(createdBy)
	if email == "" {
		return User{}, fmt.Errorf("%w: email wajib diisi", ErrInvalidInput)
	}
	displayName, err := validasiDisplayName(displayName)
	if err != nil {
		return User{}, err
	}
	if !role.Valid() {
		return User{}, fmt.Errorf("%w: %q", ErrRoleInvalid, string(role))
	}

	var hasil User
	err = s.immediateTx(ctx, func(ctx context.Context, conn *sql.Conn) error {
		if createdBy != "" {
			// Pembuat undangan harus benar-benar ada; foreign key juga akan
			// menolaknya, tetapi pesan yang jelas lebih berguna bagi pemanggil.
			if _, err := userByIDTx(ctx, conn, createdBy); err != nil {
				return err
			}
		}

		id := ids.New()
		now := formatWaktu(s.waktuSekarang())
		if _, err := conn.ExecContext(ctx,
			`INSERT INTO users (id, email, display_name, role, status, created_by, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			id, email, displayName, string(role), string(StatusInvited), nullable(createdBy), now, now,
		); err != nil {
			if isUniqueViolation(err) {
				return fmt.Errorf("%w: email %s sudah terdaftar", ErrConflict, email)
			}
			return fmt.Errorf("auth: gagal menyimpan undangan: %w", err)
		}

		user, err := userByIDTx(ctx, conn, id)
		if err != nil {
			return err
		}
		hasil = user
		return nil
	})
	if err != nil {
		return User{}, err
	}
	return hasil, nil
}

// CreateFirstAdmin menulis admin pertama pada bootstrap lokal
// (`-bootstrap-admin`, D-032). Fungsi ini menolak berjalan bila sudah ada akun
// `admin` berstatus `invited` atau `active`: pengguna Google pertama tidak
// pernah otomatis menjadi admin, dan bootstrap ganda hanya menambah jalan masuk
// istimewa yang tidak terpantau. `created_by` dibiarkan NULL karena akun ini
// dibuat sistem, bukan oleh admin lain.
func (s *Store) CreateFirstAdmin(ctx context.Context, email, displayName string) (User, error) {
	email = normalisasiEmail(email)
	if email == "" {
		return User{}, fmt.Errorf("%w: email wajib diisi", ErrInvalidInput)
	}
	displayName, err := validasiDisplayName(displayName)
	if err != nil {
		return User{}, err
	}

	var hasil User
	err = s.immediateTx(ctx, func(ctx context.Context, conn *sql.Conn) error {
		var jumlah int
		if err := conn.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM users WHERE role = ? AND status IN (?, ?)`,
			string(RoleAdmin), string(StatusInvited), string(StatusActive),
		).Scan(&jumlah); err != nil {
			return fmt.Errorf("auth: gagal memeriksa admin yang ada: %w", err)
		}
		if jumlah > 0 {
			return fmt.Errorf("%w: sudah ada akun admin dengan status invited atau active", ErrConflict)
		}

		id := ids.New()
		now := formatWaktu(s.waktuSekarang())
		if _, err := conn.ExecContext(ctx,
			`INSERT INTO users (id, email, display_name, role, status, created_by, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, NULL, ?, ?)`,
			id, email, displayName, string(RoleAdmin), string(StatusInvited), now, now,
		); err != nil {
			if isUniqueViolation(err) {
				return fmt.Errorf("%w: email %s sudah terdaftar", ErrConflict, email)
			}
			return fmt.Errorf("auth: gagal menyimpan admin pertama: %w", err)
		}

		user, err := userByIDTx(ctx, conn, id)
		if err != nil {
			return err
		}
		hasil = user
		return nil
	})
	if err != nil {
		return User{}, err
	}
	return hasil, nil
}

// UserByID membaca satu akun. Baris yang tidak ada dikembalikan sebagai
// ErrNotFound, bukan sql.ErrNoRows.
func (s *Store) UserByID(ctx context.Context, id string) (User, error) {
	id = teks(id)
	if id == "" {
		return User{}, fmt.Errorf("%w: id pengguna wajib diisi", ErrInvalidInput)
	}

	user, err := scanUser(s.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id = ?`, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, fmt.Errorf("%w: pengguna %s", ErrNotFound, id)
		}
		return User{}, fmt.Errorf("auth: gagal membaca pengguna %s: %w", id, err)
	}
	return user, nil
}

// batasDaftarPengguna adalah batas jumlah baris yang dikembalikan ListUsers bila
// pemanggil meminta nilai yang tidak masuk akal. Daftar akun selalu dipaginasi
// di lapisan HTTP; batas ini menjaga kueri tetap terbatas.
const batasDaftarPengguna = 500

// ListUsers mengembalikan daftar akun dengan penyaringan opsional. Role atau
// status kosong berarti tanpa penyaringan; limit <= 0 memakai batas bawaan.
func (s *Store) ListUsers(ctx context.Context, role Role, status Status, limit int) ([]User, error) {
	if role != "" && !role.Valid() {
		return nil, fmt.Errorf("%w: %q", ErrRoleInvalid, string(role))
	}
	if status != "" && !status.Valid() {
		return nil, fmt.Errorf("%w: %q", ErrStatusInvalid, string(status))
	}
	if limit <= 0 || limit > batasDaftarPengguna {
		limit = batasDaftarPengguna
	}

	kueri := `SELECT ` + userColumns + ` FROM users WHERE 1 = 1`
	var args []any
	if role != "" {
		kueri += ` AND role = ?`
		args = append(args, string(role))
	}
	if status != "" {
		kueri += ` AND status = ?`
		args = append(args, string(status))
	}
	kueri += ` ORDER BY created_at, id LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, kueri, args...)
	if err != nil {
		return nil, fmt.Errorf("auth: gagal membaca daftar pengguna: %w", err)
	}
	defer rows.Close()

	daftar := make([]User, 0, limit)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("auth: gagal membaca baris pengguna: %w", err)
		}
		daftar = append(daftar, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("auth: penelusuran daftar pengguna gagal: %w", err)
	}
	return daftar, nil
}

// SetRole mengubah role utama satu akun (D-037). Dua penjaga berlaku sebelum
// perubahan apa pun: aktor tidak boleh mengubah akunnya sendiri, dan admin aktif
// terakhir tidak boleh diturunkan. Perhitungan admin aktif lain dilakukan di
// dalam transaksi yang sama supaya dua permintaan bersamaan tidak dapat
// menghabiskan seluruh admin.
func (s *Store) SetRole(ctx context.Context, actorID, userID string, role Role) (User, error) {
	actorID, userID = teks(actorID), teks(userID)
	if !role.Valid() {
		return User{}, fmt.Errorf("%w: %q", ErrRoleInvalid, string(role))
	}
	if actorID == "" || userID == "" {
		return User{}, fmt.Errorf("%w: actorID dan userID wajib diisi", ErrInvalidInput)
	}

	var hasil User
	err := s.immediateTx(ctx, func(ctx context.Context, conn *sql.Conn) error {
		user, err := userByIDTx(ctx, conn, userID)
		if err != nil {
			return err
		}

		if user.Role == role {
			hasil = user
			return nil
		}

		// Penjaga admin aktif terakhir diperiksa **sebelum** aturan akun
		// sendiri. Bila pelakunya adalah admin aktif terakhir, aturan yang
		// sesungguhnya dilanggar adalah yang pertama — dan itulah kode yang
		// dipakai kontrak (docs/API_CONTRACT.md §5.2, skenario TA-21).
		if user.Role == RoleAdmin && user.Status == StatusActive {
			lain, err := adminAktifLain(ctx, conn, userID)
			if err != nil {
				return err
			}
			if lain == 0 {
				return fmt.Errorf("%w: menurunkan role admin aktif terakhir", ErrLastAdminProtected)
			}
		}

		if actorID == userID {
			return fmt.Errorf("%w: role akun sendiri tidak dapat diubah", ErrSelfAccountProtected)
		}

		now := formatWaktu(s.waktuSekarang())
		if _, err := conn.ExecContext(ctx,
			`UPDATE users SET role = ?, updated_at = ? WHERE id = ?`,
			string(role), now, userID,
		); err != nil {
			return fmt.Errorf("auth: gagal mengubah role pengguna %s: %w", userID, err)
		}

		diperbarui, err := userByIDTx(ctx, conn, userID)
		if err != nil {
			return err
		}
		hasil = diperbarui
		return nil
	})
	if err != nil {
		return User{}, err
	}
	return hasil, nil
}

// SetStatus mengubah status satu akun. Penonaktifan admin aktif terakhir
// ditolak, dan akun sendiri tidak dapat diubah (D-037). Pencabutan sesi akibat
// `suspended` dilakukan pemanggil pada tabel `session_tokens` (D-033); paket ini
// hanya menegakkan keadaan akun.
func (s *Store) SetStatus(ctx context.Context, actorID, userID string, status Status) (User, error) {
	actorID, userID = teks(actorID), teks(userID)
	if !status.Valid() {
		return User{}, fmt.Errorf("%w: %q", ErrStatusInvalid, string(status))
	}
	if actorID == "" || userID == "" {
		return User{}, fmt.Errorf("%w: actorID dan userID wajib diisi", ErrInvalidInput)
	}

	var hasil User
	err := s.immediateTx(ctx, func(ctx context.Context, conn *sql.Conn) error {
		user, err := userByIDTx(ctx, conn, userID)
		if err != nil {
			return err
		}

		if user.Status == status {
			hasil = user
			return nil
		}

		// Menonaktifkan admin aktif terakhir akan menghilangkan seluruh akses
		// administratif; menaikkan status kembali ke `active` tidak dibatasi.
		// Urutannya diperiksa sebelum aturan akun sendiri agar kode yang
		// dijawab sesuai kontrak (D-037, TA-21).
		if user.Role == RoleAdmin && user.Status == StatusActive && status != StatusActive {
			lain, err := adminAktifLain(ctx, conn, userID)
			if err != nil {
				return err
			}
			if lain == 0 {
				return fmt.Errorf("%w: menonaktifkan admin aktif terakhir", ErrLastAdminProtected)
			}
		}

		if actorID == userID {
			return fmt.Errorf("%w: status akun sendiri tidak dapat diubah", ErrSelfAccountProtected)
		}

		now := formatWaktu(s.waktuSekarang())
		if _, err := conn.ExecContext(ctx,
			`UPDATE users SET status = ?, updated_at = ? WHERE id = ?`,
			string(status), now, userID,
		); err != nil {
			return fmt.Errorf("auth: gagal mengubah status pengguna %s: %w", userID, err)
		}

		diperbarui, err := userByIDTx(ctx, conn, userID)
		if err != nil {
			return err
		}
		hasil = diperbarui
		return nil
	})
	if err != nil {
		return User{}, err
	}
	return hasil, nil
}

// TouchLogin mencatat waktu login terakhir. Dipanggil pada setiap penyegaran
// identitas yang sah; baris yang tidak ada dikembalikan sebagai ErrNotFound.
func (s *Store) TouchLogin(ctx context.Context, userID string) error {
	userID = teks(userID)
	if userID == "" {
		return fmt.Errorf("%w: userID wajib diisi", ErrInvalidInput)
	}

	now := formatWaktu(s.waktuSekarang())
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET last_login_at = ?, updated_at = ? WHERE id = ?`,
		now, now, userID,
	)
	if err != nil {
		return fmt.Errorf("auth: gagal mencatat login pengguna %s: %w", userID, err)
	}
	terpengaruh, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("auth: hasil pembaruan login tidak dapat dibaca: %w", err)
	}
	if terpengaruh == 0 {
		return fmt.Errorf("%w: pengguna %s", ErrNotFound, userID)
	}
	return nil
}

// batasPanjangDisplayName adalah panjang maksimum `users.display_name` menurut
// docs/DATA_MODEL.md §4.1 (1–120 karakter).
const batasPanjangDisplayName = 120

// validasiDisplayName memastikan nama tampilan terisi dan tidak melebihi batas
// skema. Nama bukan data keamanan, tetapi baris yang melampaui batas akan
// merusak tampilan dan menyulitkan pemadanan dengan data akademik.
func validasiDisplayName(displayName string) (string, error) {
	displayName = teks(displayName)
	if displayName == "" {
		return "", fmt.Errorf("%w: display_name wajib diisi", ErrInvalidInput)
	}
	if len([]rune(displayName)) > batasPanjangDisplayName {
		return "", fmt.Errorf("%w: display_name melebihi %d karakter", ErrInvalidInput, batasPanjangDisplayName)
	}
	return displayName, nil
}

// SetDisplayName mengubah nama tampilan satu akun. Berbeda dari role dan status,
// perubahan nama tidak menyentuh hak akses, sehingga pengguna boleh mengubah
// namanya sendiri — karena itu tidak ada penjaga akun-sendiri di sini. Nama
// kosong atau lebih dari 120 karakter ditolak.
func (s *Store) SetDisplayName(ctx context.Context, actorID, userID, displayName string) (User, error) {
	actorID, userID = teks(actorID), teks(userID)
	displayName, err := validasiDisplayName(displayName)
	if err != nil {
		return User{}, err
	}
	if actorID == "" || userID == "" {
		return User{}, fmt.Errorf("%w: actorID dan userID wajib diisi", ErrInvalidInput)
	}

	var hasil User
	err = s.immediateTx(ctx, func(ctx context.Context, conn *sql.Conn) error {
		if _, err := userByIDTx(ctx, conn, userID); err != nil {
			return err
		}
		if _, err := userByIDTx(ctx, conn, actorID); err != nil {
			return err
		}

		now := formatWaktu(s.waktuSekarang())
		if _, err := conn.ExecContext(ctx,
			`UPDATE users SET display_name = ?, updated_at = ? WHERE id = ?`,
			displayName, now, userID,
		); err != nil {
			return fmt.Errorf("auth: gagal mengubah nama pengguna %s: %w", userID, err)
		}

		diperbarui, err := userByIDTx(ctx, conn, userID)
		if err != nil {
			return err
		}
		hasil = diperbarui
		return nil
	})
	if err != nil {
		return User{}, err
	}
	return hasil, nil
}

// adminAktifLain menghitung admin berstatus `active` selain userID.
func adminAktifLain(ctx context.Context, conn *sql.Conn, userID string) (int, error) {
	var jumlah int
	if err := conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE role = ? AND status = ? AND id <> ?`,
		string(RoleAdmin), string(StatusActive), userID,
	).Scan(&jumlah); err != nil {
		return 0, fmt.Errorf("auth: gagal menghitung admin aktif lain: %w", err)
	}
	return jumlah, nil
}

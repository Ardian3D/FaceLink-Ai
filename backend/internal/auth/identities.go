package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/ids"
)

// IdentityForUser mengembalikan identitas Google yang tertaut ke satu akun.
// Nilai kedua bernilai false bila akun belum memiliki identitas.
func (s *Store) IdentityForUser(ctx context.Context, userID string) (Identity, bool, error) {
	userID = teks(userID)
	if userID == "" {
		return Identity{}, false, fmt.Errorf("%w: userID wajib diisi", ErrInvalidInput)
	}

	ident, err := scanIdentity(s.db.QueryRowContext(ctx,
		`SELECT id, user_id, provider, subject, email_at_provider, linked_at, created_at
		   FROM user_identities WHERE user_id = ? AND provider = ?`,
		userID, providerGoogle))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Identity{}, false, nil
		}
		return Identity{}, false, fmt.Errorf("auth: gagal membaca identitas pengguna %s: %w", userID, err)
	}
	return ident, true, nil
}

// LinkInvitedAccount menautkan identitas Google ke akun `invited` yang emailnya
// sama, lalu mengaktifkannya (D-032). Fungsi ini adalah operasi **penautan
// pertama**: pemanggil yang melayani login berulang mencari identitas lewat
// IdentityForUser lebih dahulu, bukan memanggil fungsi ini lagi.
//
// Seluruh langkah berjalan dalam satu transaksi BEGIN IMMEDIATE. Bila tidak ada
// undangan yang cocok, fungsi ini **tidak** membuat akun baru: auto-provision
// akan menulis baris untuk orang asing dan membuka pengambilalihan akun
// (alternatif yang ditolak pada D-032).
//
// Batasan unik `(provider, subject)` dan `(user_id, provider)` adalah penjaga
// terakhir untuk permintaan bersamaan: dua login paralel dengan subjek berbeda
// hanya dapat menghasilkan satu tautan, dan pelanggaran dari database dipetakan
// ke sentinel yang sama dengan pemeriksaan di dalam transaksi.
func (s *Store) LinkInvitedAccount(ctx context.Context, subject, email string, emailVerified bool) (User, error) {
	subject = teks(subject)
	if subject == "" {
		return User{}, fmt.Errorf("%w: subject wajib diisi", ErrInvalidInput)
	}
	if !emailVerified {
		// Email yang belum diverifikasi Google tidak boleh menjadi dasar
		// pencocokan undangan; email bukan kunci identitas.
		return User{}, fmt.Errorf("%w: email_verified belum diverifikasi", ErrInvalidInput)
	}
	email = normalisasiEmail(email)
	if email == "" {
		return User{}, fmt.Errorf("%w: email wajib diisi", ErrInvalidInput)
	}

	var hasil User
	err := s.immediateTx(ctx, func(ctx context.Context, conn *sql.Conn) error {
		user, err := userByEmailTx(ctx, conn, email)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return fmt.Errorf("%w: tidak ada undangan untuk email tersebut", ErrNotInvited)
			}
			return err
		}

		pemilik, ada, err := pemilikSubjectTx(ctx, conn, subject)
		if err != nil {
			return err
		}
		if ada && pemilik != user.ID {
			return fmt.Errorf("%w: subjek sudah tertaut akun lain", ErrIdentityTaken)
		}
		if ada && pemilik == user.ID {
			return fmt.Errorf("%w: akun %s sudah memiliki identitas google", ErrAlreadyLinked, user.ID)
		}

		var sudahTertaut int
		if err := conn.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM user_identities WHERE user_id = ? AND provider = ?`,
			user.ID, providerGoogle).Scan(&sudahTertaut); err != nil {
			return fmt.Errorf("auth: gagal memeriksa identitas akun %s: %w", user.ID, err)
		}
		if sudahTertaut > 0 {
			return fmt.Errorf("%w: akun %s sudah memiliki identitas google", ErrAlreadyLinked, user.ID)
		}

		if user.Status != StatusInvited {
			return fmt.Errorf("%w: akun %s berstatus %s, bukan invited", ErrNotInvited, user.ID, string(user.Status))
		}

		now := s.waktuSekarang()
		if _, err := conn.ExecContext(ctx,
			`INSERT INTO user_identities (id, user_id, provider, subject, email_at_provider, linked_at, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			ids.New(), user.ID, providerGoogle, subject, nullable(email),
			formatWaktu(now), formatWaktu(now),
		); err != nil {
			if isUniqueViolation(err) {
				// Permintaan bersamaan menang lebih dahulu: subjek ini milik akun
				// lain, atau akun ini baru saja ditautkan.
				if pemilikLain, ada, cekErr := pemilikSubjectTx(ctx, conn, subject); cekErr == nil && ada && pemilikLain != user.ID {
					return fmt.Errorf("%w: subjek sudah tertaut akun lain", ErrIdentityTaken)
				}
				return fmt.Errorf("%w: akun %s sudah memiliki identitas google", ErrAlreadyLinked, user.ID)
			}
			return fmt.Errorf("auth: gagal menyimpan identitas pengguna %s: %w", user.ID, err)
		}

		if _, err := conn.ExecContext(ctx,
			`UPDATE users SET status = ?, last_login_at = ?, updated_at = ? WHERE id = ?`,
			string(StatusActive), formatWaktu(now), formatWaktu(now), user.ID,
		); err != nil {
			return fmt.Errorf("auth: gagal mengaktifkan pengguna %s: %w", user.ID, err)
		}

		diperbarui, err := userByIDTx(ctx, conn, user.ID)
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

// UserBySubject mencari akun yang identitas Google-nya sudah tertaut. Nilai
// kedua bernilai false bila belum ada identitas dengan subjek tersebut; subjek
// yang belum tertaut bukan keadaan galat, sehingga ErrNotFound tidak dipakai di
// sini. Provider tidak menjadi parameter karena kolomnya di-CHECK hanya berisi
// 'google'.
func (s *Store) UserBySubject(ctx context.Context, subject string) (User, bool, error) {
	subject = teks(subject)
	if subject == "" {
		return User{}, false, fmt.Errorf("%w: subject wajib diisi", ErrInvalidInput)
	}

	userID, ada, err := pemilikSubjectQ(ctx, s.db, subject)
	if err != nil {
		return User{}, false, err
	}
	if !ada {
		return User{}, false, nil
	}

	user, err := scanUser(s.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id = ?`, userID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Baris identitas selalu menunjuk pengguna (foreign key CASCADE),
			// sehingga keadaan ini hanya mungkin bila baris diubah di luar
			// aplikasi; dilaporkan sebagai tidak ditemukan, bukan panik.
			return User{}, false, fmt.Errorf("%w: pengguna %s", ErrNotFound, userID)
		}
		return User{}, false, fmt.Errorf("auth: gagal membaca pengguna %s: %w", userID, err)
	}
	return user, true, nil
}

// pemilikSubjectQ membaca pemilik subjek di luar transaksi.
func pemilikSubjectQ(ctx context.Context, db *database.DB, subject string) (string, bool, error) {
	var userID string
	err := db.QueryRowContext(ctx,
		`SELECT user_id FROM user_identities WHERE provider = ? AND subject = ?`,
		providerGoogle, subject).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("auth: gagal membaca identitas berdasarkan subjek: %w", err)
	}
	return userID, true, nil
}

// pemilikSubjectTx mengembalikan id akun yang memiliki subjek Google tertentu.
// Nilai kedua bernilai false bila subjek belum tertaut ke akun mana pun.
func pemilikSubjectTx(ctx context.Context, conn *sql.Conn, subject string) (string, bool, error) {
	var userID string
	err := conn.QueryRowContext(ctx,
		`SELECT user_id FROM user_identities WHERE provider = ? AND subject = ?`,
		providerGoogle, subject).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("auth: gagal membaca identitas berdasarkan subjek: %w", err)
	}
	return userID, true, nil
}

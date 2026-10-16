package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// AssignLecturer menugaskan seorang dosen ke satu kelas (D-034). Cakupan dosen
// dibatasi pada kelas yang secara resmi ditugaskan, sehingga baris di
// `class_lecturers` adalah dasar otorisasi — bukan sekadar tampilan.
//
// Hanya akun ber-role `dosen` yang boleh ditugaskan; role lain ditolak dengan
// ErrRoleInvalid. Penugasan yang sama diulang bersifat idempoten: baris yang
// sudah ada tidak diubah, termasuk `assigned_by` dan `assigned_at`-nya, agar
// riwayat penugasan awal tetap terbaca.
func (s *Store) AssignLecturer(ctx context.Context, actorID, classID, userID string) error {
	actorID, classID, userID = teks(actorID), teks(classID), teks(userID)
	if actorID == "" || classID == "" || userID == "" {
		return fmt.Errorf("%w: actorID, classID, dan userID wajib diisi", ErrInvalidInput)
	}

	return s.immediateTx(ctx, func(ctx context.Context, conn *sql.Conn) error {
		user, err := userByIDTx(ctx, conn, userID)
		if err != nil {
			return err
		}
		if user.Role != RoleDosen {
			return fmt.Errorf("%w: akun %s berrole %s, bukan dosen", ErrRoleInvalid, userID, string(user.Role))
		}
		if _, err := userByIDTx(ctx, conn, actorID); err != nil {
			return err
		}
		if err := pastikanKelasAda(ctx, conn, classID); err != nil {
			return err
		}

		if _, err := conn.ExecContext(ctx,
			`INSERT INTO class_lecturers (class_id, user_id, assigned_by, assigned_at)
			 VALUES (?, ?, ?, ?)
			 ON CONFLICT (class_id, user_id) DO NOTHING`,
			classID, userID, actorID, formatWaktu(s.waktuSekarang()),
		); err != nil {
			return fmt.Errorf("auth: gagal menugaskan dosen %s ke kelas %s: %w", userID, classID, err)
		}
		return nil
	})
}

// RemoveLecturer mencabut penugasan seorang dosen dari satu kelas. Penugasan
// yang tidak ada dikembalikan sebagai ErrNotFound supaya pemanggil dapat
// membedakan pencabutan yang berhasil dari id yang salah.
func (s *Store) RemoveLecturer(ctx context.Context, classID, userID string) error {
	classID, userID = teks(classID), teks(userID)
	if classID == "" || userID == "" {
		return fmt.Errorf("%w: classID dan userID wajib diisi", ErrInvalidInput)
	}

	res, err := s.db.ExecContext(ctx,
		`DELETE FROM class_lecturers WHERE class_id = ? AND user_id = ?`, classID, userID)
	if err != nil {
		return fmt.Errorf("auth: gagal mencabut penugasan dosen %s dari kelas %s: %w", userID, classID, err)
	}
	terhapus, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("auth: jumlah penugasan terhapus tidak dapat dibaca: %w", err)
	}
	if terhapus == 0 {
		return fmt.Errorf("%w: penugasan dosen %s pada kelas %s", ErrNotFound, userID, classID)
	}
	return nil
}

// LecturersForClass mengembalikan seluruh dosen yang ditugaskan pada satu kelas.
func (s *Store) LecturersForClass(ctx context.Context, classID string) ([]User, error) {
	classID = teks(classID)
	if classID == "" {
		return nil, fmt.Errorf("%w: classID wajib diisi", ErrInvalidInput)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT u.id, u.email, u.display_name, u.role, u.status, u.created_by,
		        u.last_login_at, u.created_at, u.updated_at
		   FROM class_lecturers cl
		   JOIN users u ON u.id = cl.user_id
		  WHERE cl.class_id = ?
		  ORDER BY u.display_name, u.id`, classID)
	if err != nil {
		return nil, fmt.Errorf("auth: gagal membaca dosen kelas %s: %w", classID, err)
	}
	defer rows.Close()

	daftar := make([]User, 0, 4)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("auth: gagal membaca baris dosen kelas %s: %w", classID, err)
		}
		daftar = append(daftar, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("auth: penelusuran dosen kelas %s gagal: %w", classID, err)
	}
	return daftar, nil
}

// ClassesForLecturer mengembalikan kelas yang ditugaskan kepada seorang dosen.
// Kelas yang sudah dihapus lunak tidak ikut ditampilkan.
func (s *Store) ClassesForLecturer(ctx context.Context, userID string) ([]Class, error) {
	userID = teks(userID)
	if userID == "" {
		return nil, fmt.Errorf("%w: userID wajib diisi", ErrInvalidInput)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT c.id, c.code, c.title
		   FROM class_lecturers cl
		   JOIN classes c ON c.id = cl.class_id
		  WHERE cl.user_id = ? AND c.deleted_at IS NULL
		  ORDER BY c.code, c.id`, userID)
	if err != nil {
		return nil, fmt.Errorf("auth: gagal membaca kelas dosen %s: %w", userID, err)
	}
	defer rows.Close()

	daftar := make([]Class, 0, 4)
	for rows.Next() {
		var k Class
		if err := rows.Scan(&k.ID, &k.Code, &k.Title); err != nil {
			return nil, fmt.Errorf("auth: gagal membaca baris kelas dosen %s: %w", userID, err)
		}
		daftar = append(daftar, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("auth: penelusuran kelas dosen %s gagal: %w", userID, err)
	}
	return daftar, nil
}

// ClassesForPerson mengembalikan kelas yang masih diikuti peserta. Keanggotaan
// yang sudah berakhir (`left_at` terisi) tidak ikut dihitung: hak akses peserta
// mengikuti keanggotaan aktif, bukan riwayat. Kelas yang dihapus lunak juga
// tidak ditampilkan.
func (s *Store) ClassesForPerson(ctx context.Context, personID string) ([]Class, error) {
	personID = teks(personID)
	if personID == "" {
		return nil, fmt.Errorf("%w: personID wajib diisi", ErrInvalidInput)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT c.id, c.code, c.title
		   FROM class_memberships cm
		   JOIN classes c ON c.id = cm.class_id
		  WHERE cm.person_id = ? AND cm.left_at IS NULL AND c.deleted_at IS NULL
		  ORDER BY c.code, c.id`, personID)
	if err != nil {
		return nil, fmt.Errorf("auth: gagal membaca kelas peserta %s: %w", personID, err)
	}
	defer rows.Close()

	daftar := make([]Class, 0, 4)
	for rows.Next() {
		var k Class
		if err := rows.Scan(&k.ID, &k.Code, &k.Title); err != nil {
			return nil, fmt.Errorf("auth: gagal membaca baris kelas peserta %s: %w", personID, err)
		}
		daftar = append(daftar, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("auth: penelusuran kelas peserta %s gagal: %w", personID, err)
	}
	return daftar, nil
}

// IsLecturerAssigned melaporkan apakah seorang dosen ditugaskan pada satu kelas.
// Fungsi ini dipakai jalur otorisasi sehingga hanya mengembalikan benar/salah,
// bukan galat baris tidak ditemukan.
func (s *Store) IsLecturerAssigned(ctx context.Context, userID, classID string) (bool, error) {
	userID, classID = teks(userID), teks(classID)
	if userID == "" || classID == "" {
		return false, fmt.Errorf("%w: userID dan classID wajib diisi", ErrInvalidInput)
	}

	var ada int
	if err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM class_lecturers WHERE class_id = ? AND user_id = ?)`,
		classID, userID).Scan(&ada); err != nil {
		return false, fmt.Errorf("auth: gagal memeriksa penugasan dosen %s pada kelas %s: %w", userID, classID, err)
	}
	return ada == 1, nil
}

// LinkPersonAccount mengaitkan akun mahasiswa dengan satu baris peserta
// (`people.account_user_id`). Pengaitan adalah dasar "data miliknya sendiri"
// pada matriks otorisasi, sehingga hanya akun ber-role `mahasiswa` yang boleh
// dikaitkan; akun lain ditolak dengan ErrNotMahasiswa.
//
// Dua sisi relasi dijaga: satu akun hanya boleh menunjuk satu peserta
// (ErrConflict bila akun sudah tertaut peserta lain), dan satu peserta hanya
// boleh dimiliki satu akun (ErrAlreadyLinked bila peserta sudah tertaut akun
// lain). Indeks unik parsial `uq_people_account_user` adalah penjaga terakhir
// untuk permintaan bersamaan, dan pelanggarannya dipetakan ke sentinel yang
// sama dengan pemeriksaan di dalam transaksi.
func (s *Store) LinkPersonAccount(ctx context.Context, actorID, personID, userID string) error {
	actorID, personID, userID = teks(actorID), teks(personID), teks(userID)
	if actorID == "" || personID == "" || userID == "" {
		return fmt.Errorf("%w: actorID, personID, dan userID wajib diisi", ErrInvalidInput)
	}

	return s.immediateTx(ctx, func(ctx context.Context, conn *sql.Conn) error {
		user, err := userByIDTx(ctx, conn, userID)
		if err != nil {
			return err
		}
		if user.Role != RoleMahasiswa {
			return fmt.Errorf("%w: akun %s berrole %s", ErrNotMahasiswa, userID, string(user.Role))
		}
		if _, err := userByIDTx(ctx, conn, actorID); err != nil {
			return err
		}

		// Baris peserta sekaligus membawa pengaitan yang sudah ada, sehingga
		// keberadaan peserta dan keadaan pengaitannya dibaca sekali saja.
		var akunPeserta sql.NullString
		if err := conn.QueryRowContext(ctx,
			`SELECT account_user_id FROM people WHERE id = ? AND deleted_at IS NULL`,
			personID).Scan(&akunPeserta); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w: peserta %s", ErrNotFound, personID)
			}
			return fmt.Errorf("auth: gagal membaca pengaitan peserta %s: %w", personID, err)
		}
		if akunPeserta.Valid && akunPeserta.String == userID {
			// Pengaitan yang sama diulang tidak mengubah apa pun.
			return nil
		}
		if akunPeserta.Valid {
			return fmt.Errorf("%w: peserta %s sudah tertaut akun %s", ErrAlreadyLinked, personID, akunPeserta.String)
		}

		var pesertaLain sql.NullString
		// Pemeriksaan ini tidak menyaring `deleted_at`, sama seperti indeks unik
		// `uq_people_account_user` yang juga tidak menyaringnya: baris peserta
		// yang dihapus lunak tetap memegang slot unik, sehingga pengaitan baru
		// untuk akun itu memang akan ditolak database.
		if err := conn.QueryRowContext(ctx,
			`SELECT id FROM people WHERE account_user_id = ?`, userID).Scan(&pesertaLain); err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("auth: gagal memeriksa pengaitan akun %s: %w", userID, err)
			}
		}
		if pesertaLain.Valid {
			return fmt.Errorf("%w: akun %s sudah tertaut peserta %s", ErrConflict, userID, pesertaLain.String)
		}

		if _, err := conn.ExecContext(ctx,
			`UPDATE people SET account_user_id = ?, updated_at = ? WHERE id = ?`,
			userID, formatWaktu(s.waktuSekarang()), personID,
		); err != nil {
			if isUniqueViolation(err) {
				return fmt.Errorf("%w: akun %s sudah tertaut peserta lain", ErrAlreadyLinked, userID)
			}
			return fmt.Errorf("auth: gagal mengaitkan peserta %s dengan akun %s: %w", personID, userID, err)
		}
		return nil
	})
}

// UnlinkPersonAccount melepas pengaitan akun dari satu peserta. Peserta yang
// tidak ada dikembalikan sebagai ErrNotFound; peserta yang memang belum tertaut
// tidak menghasilkan galat karena hasil akhirnya sama.
func (s *Store) UnlinkPersonAccount(ctx context.Context, actorID, personID string) error {
	actorID, personID = teks(actorID), teks(personID)
	if actorID == "" || personID == "" {
		return fmt.Errorf("%w: actorID dan personID wajib diisi", ErrInvalidInput)
	}

	return s.immediateTx(ctx, func(ctx context.Context, conn *sql.Conn) error {
		if _, err := userByIDTx(ctx, conn, actorID); err != nil {
			return err
		}
		if err := pastikanPesertaAda(ctx, conn, personID); err != nil {
			return err
		}
		if _, err := conn.ExecContext(ctx,
			`UPDATE people SET account_user_id = NULL, updated_at = ? WHERE id = ?`,
			formatWaktu(s.waktuSekarang()), personID,
		); err != nil {
			return fmt.Errorf("auth: gagal melepas pengaitan peserta %s: %w", personID, err)
		}
		return nil
	})
}

// PersonForUser mengembalikan id peserta yang tertaut ke satu akun. Nilai kedua
// bernilai false bila akun belum tertaut peserta mana pun.
func (s *Store) PersonForUser(ctx context.Context, userID string) (string, bool, error) {
	userID = teks(userID)
	if userID == "" {
		return "", false, fmt.Errorf("%w: userID wajib diisi", ErrInvalidInput)
	}

	var personID string
	// Tidak menyaring `deleted_at`: pengaitan adalah keadaan baris peserta, dan
	// slot unik tetap dipegang baris yang dihapus lunak. Pemanggil yang hanya
	// ingin peserta aktif menyaringnya sendiri lewat status peserta.
	if err := s.db.QueryRowContext(ctx,
		`SELECT id FROM people WHERE account_user_id = ?`, userID).Scan(&personID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("auth: gagal membaca peserta akun %s: %w", userID, err)
	}
	return personID, true, nil
}

// pastikanKelasAda memastikan kelas yang dituju ada dan belum dihapus lunak.
func pastikanKelasAda(ctx context.Context, conn *sql.Conn, classID string) error {
	var ada int
	if err := conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM classes WHERE id = ? AND deleted_at IS NULL`, classID).Scan(&ada); err != nil {
		return fmt.Errorf("auth: gagal memeriksa kelas %s: %w", classID, err)
	}
	if ada == 0 {
		return fmt.Errorf("%w: kelas %s", ErrNotFound, classID)
	}
	return nil
}

// pastikanPesertaAda memastikan baris peserta ada dan belum dihapus lunak.
func pastikanPesertaAda(ctx context.Context, conn *sql.Conn, personID string) error {
	var ada int
	if err := conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM people WHERE id = ? AND deleted_at IS NULL`, personID).Scan(&ada); err != nil {
		return fmt.Errorf("auth: gagal memeriksa peserta %s: %w", personID, err)
	}
	if ada == 0 {
		return fmt.Errorf("%w: peserta %s", ErrNotFound, personID)
	}
	return nil
}

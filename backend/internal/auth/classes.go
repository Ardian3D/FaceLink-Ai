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
// sudah ada tidak diubah. Nilai balik kedua bernilai true hanya bila baris
// penugasan benar-benar dibuat, sehingga lapisan HTTP dapat menjawab `201` atau
// `200` sesuai kontrak §5.5.
func (s *Store) AssignLecturer(ctx context.Context, actorID, classID, userID string) (bool, error) {
	actorID, classID, userID = teks(actorID), teks(classID), teks(userID)
	if actorID == "" || classID == "" || userID == "" {
		return false, fmt.Errorf("%w: actorID, classID, dan userID wajib diisi", ErrInvalidInput)
	}

	dibuat := false
	err := s.immediateTx(ctx, func(ctx context.Context, conn *sql.Conn) error {
		if err := requireActiveAdminTx(ctx, conn, actorID); err != nil {
			return err
		}
		user, err := userByIDTx(ctx, conn, userID)
		if err != nil {
			return err
		}
		if user.Role != RoleDosen {
			return fmt.Errorf("%w: akun %s berrole %s, bukan dosen", ErrRoleInvalid, userID, string(user.Role))
		}
		if err := pastikanKelasAda(ctx, conn, classID); err != nil {
			return err
		}

		hasil, err := conn.ExecContext(ctx,
			`INSERT INTO class_lecturers (class_id, user_id, assigned_by, assigned_at)
			 VALUES (?, ?, ?, ?)
			 ON CONFLICT (class_id, user_id) DO NOTHING`,
			classID, userID, actorID, formatWaktu(s.waktuSekarang()),
		)
		if err != nil {
			return fmt.Errorf("auth: gagal menugaskan dosen %s ke kelas %s: %w", userID, classID, err)
		}
		jumlah, err := hasil.RowsAffected()
		if err != nil {
			return fmt.Errorf("auth: jumlah penugasan dibuat tidak dapat dibaca: %w", err)
		}
		dibuat = jumlah == 1
		if !dibuat {
			return nil
		}
		return s.auditTx(ctx, conn, actorID, "class.dosen_assigned", "class", classID,
			"Dosen ditugaskan ke kelas", map[string]string{"count": "1"})
	})
	if err != nil {
		return false, err
	}
	return dibuat, nil
}

// RemoveLecturer mencabut penugasan seorang dosen dari satu kelas. Penugasan
// yang tidak ada dikembalikan sebagai ErrNotFound supaya pemanggil dapat
// membedakan pencabutan yang berhasil dari id yang salah.
func (s *Store) RemoveLecturer(ctx context.Context, actorID, classID, userID string) error {
	actorID, classID, userID = teks(actorID), teks(classID), teks(userID)
	if actorID == "" || classID == "" || userID == "" {
		return fmt.Errorf("%w: actorID, classID, dan userID wajib diisi", ErrInvalidInput)
	}

	return s.immediateTx(ctx, func(ctx context.Context, conn *sql.Conn) error {
		if err := requireActiveAdminTx(ctx, conn, actorID); err != nil {
			return err
		}
		if err := pastikanKelasAda(ctx, conn, classID); err != nil {
			return err
		}
		res, err := conn.ExecContext(ctx,
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
		return s.auditTx(ctx, conn, actorID, "class.dosen_removed", "class", classID,
			"Penugasan dosen dicabut dari kelas", map[string]string{"count": "1"})
	})
}

// LecturersForClass mengembalikan seluruh penugasan dosen pada satu kelas
// beserta waktu penugasannya (kontrak §5.5). Waktu itu hanya dapat dibaca dari
// tabel penugasan, sehingga kueri mengembalikan bentuk penugasan — bukan bentuk
// akun — supaya `assigned_at` tidak hilang di lapisan HTTP.
func (s *Store) LecturersForClass(ctx context.Context, classID string) ([]LecturerAssignment, error) {
	classID = teks(classID)
	if classID == "" {
		return nil, fmt.Errorf("%w: classID wajib diisi", ErrInvalidInput)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT u.id, u.display_name, u.email, cl.assigned_at
		   FROM class_lecturers cl
		   JOIN users u ON u.id = cl.user_id
		  WHERE cl.class_id = ?
		  ORDER BY u.display_name, u.id`, classID)
	if err != nil {
		return nil, fmt.Errorf("auth: gagal membaca dosen kelas %s: %w", classID, err)
	}
	defer rows.Close()

	daftar := make([]LecturerAssignment, 0, 4)
	for rows.Next() {
		var (
			p        LecturerAssignment
			assigned string
		)
		if err := rows.Scan(&p.UserID, &p.DisplayName, &p.Email, &assigned); err != nil {
			return nil, fmt.Errorf("auth: gagal membaca baris dosen kelas %s: %w", classID, err)
		}
		waktu, err := parseWaktu(assigned)
		if err != nil {
			return nil, fmt.Errorf("auth: waktu penugasan dosen kelas %s tidak terbaca: %w", classID, err)
		}
		p.AssignedAt = waktu
		daftar = append(daftar, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("auth: penelusuran dosen kelas %s gagal: %w", classID, err)
	}
	return daftar, nil
}

// kolomKelas adalah proyeksi kelas beserta nilai turunan yang diwajibkan
// kontrak §5.14: jumlah anggota aktif dan sesi absensi yang sedang terbuka.
// Keduanya dihitung dengan subkueri berkorelasi supaya satu kelas tetap satu
// baris tanpa GROUP BY dan tanpa menggandakan hasil.
const kolomKelas = `c.id, c.code, c.title,
	(SELECT COUNT(*) FROM class_memberships cm
	  JOIN people p ON p.id = cm.person_id
	  WHERE cm.class_id = c.id AND cm.left_at IS NULL
	    AND p.deleted_at IS NULL AND p.status = 'active') AS member_count,
	(SELECT s.id FROM attendance_sessions s
	  WHERE s.class_id = c.id AND s.status = 'open'
	  ORDER BY s.closes_at LIMIT 1) AS active_session_id`

// scanKelas memindai satu baris proyeksi kolomKelas.
func scanKelas(sc interface{ Scan(...any) error }) (Class, error) {
	var (
		k             Class
		activeSession sql.NullString
	)
	if err := sc.Scan(&k.ID, &k.Code, &k.Title, &k.MemberCount, &activeSession); err != nil {
		return Class{}, err
	}
	k.ActiveSessionID = activeSession.String
	return k, nil
}

// ClassesForLecturer mengembalikan kelas yang ditugaskan kepada seorang dosen.
// Kelas yang sudah dihapus lunak tidak ikut ditampilkan.
func (s *Store) ClassesForLecturer(ctx context.Context, userID string) ([]Class, error) {
	userID = teks(userID)
	if userID == "" {
		return nil, fmt.Errorf("%w: userID wajib diisi", ErrInvalidInput)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT `+kolomKelas+`
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
		k, err := scanKelas(rows)
		if err != nil {
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
		`SELECT `+kolomKelas+`
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
		k, err := scanKelas(rows)
		if err != nil {
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
func (s *Store) LinkPersonAccount(ctx context.Context, actorID, personID, userID string) (Person, error) {
	actorID, personID, userID = teks(actorID), teks(personID), teks(userID)
	if actorID == "" || personID == "" || userID == "" {
		return Person{}, fmt.Errorf("%w: actorID, personID, dan userID wajib diisi", ErrInvalidInput)
	}

	var hasil Person
	err := s.immediateTx(ctx, func(ctx context.Context, conn *sql.Conn) error {
		if err := requireActiveAdminTx(ctx, conn, actorID); err != nil {
			return err
		}
		user, err := userByIDTx(ctx, conn, userID)
		if err != nil {
			return err
		}
		if user.Role != RoleMahasiswa {
			return fmt.Errorf("%w: akun %s berrole %s", ErrNotMahasiswa, userID, string(user.Role))
		}
		if user.Status != StatusActive {
			return fmt.Errorf("%w: akun %s berstatus %s", ErrAccountInactive, userID, user.Status)
		}

		peserta, err := personByIDTx(ctx, conn, personID)
		if err != nil {
			return err
		}
		if peserta.Status != "active" {
			return fmt.Errorf("%w: peserta %s berstatus %s", ErrSubjectInactive, personID, peserta.Status)
		}
		if teks(peserta.IdentityReference) == "" || memuatKontrol(peserta.IdentityReference) || peserta.IdentityVerifiedAt == nil {
			return fmt.Errorf("%w: peserta %s", ErrIdentityNotVerified, personID)
		}
		if peserta.AccountUserID == userID {
			// Pengaitan yang sama diulang tidak mengubah apa pun.
			hasil = peserta
			return nil
		}
		if peserta.AccountUserID != "" {
			return fmt.Errorf("%w: peserta %s sudah tertaut akun %s", ErrAlreadyLinked, personID, peserta.AccountUserID)
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
				return fmt.Errorf("%w: akun %s sudah tertaut peserta lain", ErrConflict, userID)
			}
			return fmt.Errorf("auth: gagal mengaitkan peserta %s dengan akun %s: %w", personID, userID, err)
		}

		if err := s.auditTx(ctx, conn, actorID, "person.account_linked", "person", personID,
			"Akun mahasiswa ditautkan ke baris peserta",
			map[string]string{"count": "1"}); err != nil {
			return err
		}
		hasil, err = personByIDTx(ctx, conn, personID)
		return err
	})
	if err != nil {
		return Person{}, err
	}
	return hasil, nil
}

// UnlinkPersonAccount melepas pengaitan akun dari satu peserta. Peserta yang
// tidak ada menghasilkan ErrNotFound; peserta tanpa tautan menghasilkan
// ErrNotLinked sesuai kontrak §5.2.
func (s *Store) UnlinkPersonAccount(ctx context.Context, actorID, personID string) error {
	actorID, personID = teks(actorID), teks(personID)
	if actorID == "" || personID == "" {
		return fmt.Errorf("%w: actorID dan personID wajib diisi", ErrInvalidInput)
	}

	return s.immediateTx(ctx, func(ctx context.Context, conn *sql.Conn) error {
		if err := requireActiveAdminTx(ctx, conn, actorID); err != nil {
			return err
		}
		if err := pastikanPesertaAda(ctx, conn, personID); err != nil {
			return err
		}

		// Peserta yang memang belum tertaut tidak menghasilkan perubahan apa
		// pun; kontrak §5.2 menuntut jawaban `409`, bukan `204`, supaya klien
		// dapat membedakan "tautan dilepas" dari "tidak ada tautan" dan audit
		// tidak mencatat pelepasan yang tidak pernah terjadi.
		var akunSekarang sql.NullString
		if err := conn.QueryRowContext(ctx,
			`SELECT account_user_id FROM people WHERE id = ?`, personID).Scan(&akunSekarang); err != nil {
			return fmt.Errorf("auth: gagal membaca pengaitan peserta %s: %w", personID, err)
		}
		if !akunSekarang.Valid || akunSekarang.String == "" {
			return fmt.Errorf("%w: peserta %s", ErrNotLinked, personID)
		}

		if _, err := conn.ExecContext(ctx,
			`UPDATE people SET account_user_id = NULL, updated_at = ? WHERE id = ?`,
			formatWaktu(s.waktuSekarang()), personID,
		); err != nil {
			return fmt.Errorf("auth: gagal melepas pengaitan peserta %s: %w", personID, err)
		}
		return s.auditTx(ctx, conn, actorID, "person.account_unlinked", "person", personID,
			"Tautan akun peserta dilepas", nil)
	})
}

// PersonForUser mengembalikan id peserta aktif yang tertaut ke satu akun. Nilai
// kedua bernilai false bila akun belum tertaut peserta mana pun, peserta sudah
// dihapus, atau peserta berstatus inactive. Pemeriksaan status dilakukan di
// sini agar akun yang tertaut ke peserta nonaktif tidak memperoleh akses /me.
func (s *Store) PersonForUser(ctx context.Context, userID string) (string, bool, error) {
	userID = teks(userID)
	if userID == "" {
		return "", false, fmt.Errorf("%w: userID wajib diisi", ErrInvalidInput)
	}

	var personID string
	if err := s.db.QueryRowContext(ctx,
		`SELECT p.id FROM people p
		   JOIN users u ON u.id = p.account_user_id
		  WHERE p.account_user_id = ? AND p.deleted_at IS NULL AND p.status = 'active'
		    AND p.identity_reference IS NOT NULL AND trim(p.identity_reference) <> ''
		    AND p.identity_verified_at IS NOT NULL
		    AND u.role = 'mahasiswa' AND u.status = 'active'`,
		userID).Scan(&personID); err != nil {
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

package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/ids"
)

// CreateClass membuat kelas baru (admin). Kode kelas tidak boleh dipakai ulang,
// termasuk oleh kelas yang sudah dihapus lunak, agar audit dan riwayat kelas
// tetap merujuk kode yang sama.
func (s *Store) CreateClass(ctx context.Context, actorID, code, title string) (Class, error) {
	actorID, code, title = teks(actorID), teks(code), teks(title)
	if actorID == "" || code == "" || title == "" {
		return Class{}, fmt.Errorf("%w: actorID, code, dan title wajib diisi", ErrInvalidInput)
	}
	if !validKodeKelas(code) {
		return Class{}, fmt.Errorf("%w: code harus 1-32 huruf/angka/tanda hubung", ErrInvalidInput)
	}
	if !validJudulKelas(title) {
		return Class{}, fmt.Errorf("%w: title harus 1-120 karakter tanpa karakter kontrol", ErrInvalidInput)
	}

	var hasil Class
	err := s.immediateTx(ctx, func(ctx context.Context, conn *sql.Conn) error {
		if err := requireActiveAdminTx(ctx, conn, actorID); err != nil {
			return err
		}

		var ada string
		if err := conn.QueryRowContext(ctx,
			`SELECT id FROM classes WHERE code = ? LIMIT 1`, code).Scan(&ada); err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("auth: gagal memeriksa kode kelas: %w", err)
			}
		} else {
			return fmt.Errorf("%w: kode kelas %s sudah dipakai", ErrConflict, code)
		}

		classID := ids.New()
		if _, err := conn.ExecContext(ctx,
			`INSERT INTO classes (id, code, title) VALUES (?, ?, ?)`, classID, code, title); err != nil {
			if isUniqueViolation(err) {
				return fmt.Errorf("%w: kode kelas %s sudah dipakai", ErrConflict, code)
			}
			return fmt.Errorf("auth: gagal membuat kelas: %w", err)
		}
		if err := s.auditTx(ctx, conn, actorID, "class.created", "class", classID,
			"Kelas dibuat oleh admin", map[string]string{"count": "1"}); err != nil {
			return err
		}
		hasil = Class{ID: classID, Code: code, Title: title}
		return nil
	})
	if err != nil {
		return Class{}, err
	}
	return hasil, nil
}

// AllClasses mengembalikan seluruh kelas yang belum dihapus (admin).
func (s *Store) AllClasses(ctx context.Context) ([]Class, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+kolomKelas+`
		   FROM classes c
		  WHERE c.deleted_at IS NULL
		  ORDER BY c.code, c.id`)
	if err != nil {
		return nil, fmt.Errorf("auth: gagal membaca daftar kelas: %w", err)
	}
	defer rows.Close()

	return scanKelasList(rows)
}

// ClassByID membaca satu kelas yang belum dihapus.
func (s *Store) ClassByID(ctx context.Context, classID string) (Class, error) {
	classID = teks(classID)
	if classID == "" {
		return Class{}, fmt.Errorf("%w: classID wajib diisi", ErrInvalidInput)
	}
	k, err := scanKelas(s.db.QueryRowContext(ctx,
		`SELECT `+kolomKelas+` FROM classes c
		  WHERE c.id = ? AND c.deleted_at IS NULL`, classID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Class{}, fmt.Errorf("%w: kelas %s", ErrNotFound, classID)
		}
		return Class{}, fmt.Errorf("auth: gagal membaca kelas %s: %w", classID, err)
	}
	return k, nil
}

// MembersForClass mengembalikan peserta yang masih menjadi anggota kelas.
// Keanggotaan dengan `left_at` terisi tidak ditampilkan; peserta yang sudah
// dihapus lunak juga tidak ikut.
func (s *Store) MembersForClass(ctx context.Context, classID string) ([]Person, error) {
	classID = teks(classID)
	if classID == "" {
		return nil, fmt.Errorf("%w: classID wajib diisi", ErrInvalidInput)
	}
	if _, err := s.ClassByID(ctx, classID); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT `+personColumns+`
		   FROM class_memberships cm
		   JOIN people p ON p.id = cm.person_id
		  WHERE cm.class_id = ? AND cm.left_at IS NULL
		    AND p.deleted_at IS NULL AND p.status = 'active'
		  ORDER BY p.display_name, p.id`, classID)
	if err != nil {
		return nil, fmt.Errorf("auth: gagal membaca anggota kelas %s: %w", classID, err)
	}
	defer rows.Close()

	daftar := make([]Person, 0, 8)
	for rows.Next() {
		p, err := scanPerson(rows)
		if err != nil {
			return nil, fmt.Errorf("auth: gagal membaca baris anggota kelas %s: %w", classID, err)
		}
		daftar = append(daftar, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("auth: penelusuran anggota kelas %s gagal: %w", classID, err)
	}
	return daftar, nil
}

// AddClassMember menambahkan peserta ke kelas. Operasi idempoten: true hanya
// bila baris keanggotaan benar-benar dibuat atau diaktifkan kembali.
func (s *Store) AddClassMember(ctx context.Context, actorID, classID, personID string) (bool, error) {
	actorID, classID, personID = teks(actorID), teks(classID), teks(personID)
	if actorID == "" || classID == "" || personID == "" {
		return false, fmt.Errorf("%w: actorID, classID, dan personID wajib diisi", ErrInvalidInput)
	}

	dibuat := false
	err := s.immediateTx(ctx, func(ctx context.Context, conn *sql.Conn) error {
		if err := requireActiveAdminTx(ctx, conn, actorID); err != nil {
			return err
		}
		if err := pastikanKelasAda(ctx, conn, classID); err != nil {
			return err
		}
		peserta, err := personByIDTx(ctx, conn, personID)
		if err != nil {
			return err
		}
		if peserta.Status != "active" {
			return fmt.Errorf("%w: peserta %s berstatus %s", ErrSubjectInactive, personID, peserta.Status)
		}

		var leftAt sql.NullString
		sekarang := formatWaktu(s.waktuSekarang())
		if err := conn.QueryRowContext(ctx,
			`SELECT left_at FROM class_memberships WHERE class_id = ? AND person_id = ?`,
			classID, personID).Scan(&leftAt); err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("auth: gagal memeriksa keanggotaan kelas %s: %w", classID, err)
			}
			if _, err := conn.ExecContext(ctx,
				`INSERT INTO class_memberships (class_id, person_id, joined_at, created_by)
				 VALUES (?, ?, ?, ?)`, classID, personID, sekarang, actorID); err != nil {
				return fmt.Errorf("auth: gagal menambahkan peserta %s ke kelas %s: %w", personID, classID, err)
			}
			dibuat = true
		} else if !leftAt.Valid {
			// Sudah menjadi anggota aktif: tidak ada perubahan, tidak ada audit.
			return nil
		} else {
			// Baris lama yang pernah keluar diaktifkan kembali.
			if _, err := conn.ExecContext(ctx,
				`UPDATE class_memberships
				    SET left_at = NULL, joined_at = ?, created_by = ?
				  WHERE class_id = ? AND person_id = ?`,
				sekarang, actorID, classID, personID); err != nil {
				return fmt.Errorf("auth: gagal mengaktifkan kembali peserta %s di kelas %s: %w", personID, classID, err)
			}
			dibuat = true
		}

		if !dibuat {
			return nil
		}
		return s.auditTx(ctx, conn, actorID, "class.member.added", "class", classID,
			"Peserta ditambahkan ke kelas",
			map[string]string{"count": "1"})
	})
	if err != nil {
		return false, err
	}
	return dibuat, nil
}

// RemoveClassMember menghapus keanggotaan peserta dari kelas. Penghapusan
// ditolak bila peserta memiliki catatan kehadiran pada sesi kelas yang masih
// terbuka (kontrak §5.5), agar riwayat yang sedang berjalan tidak diputus.
func (s *Store) RemoveClassMember(ctx context.Context, actorID, classID, personID string) error {
	actorID, classID, personID = teks(actorID), teks(classID), teks(personID)
	if actorID == "" || classID == "" || personID == "" {
		return fmt.Errorf("%w: actorID, classID, dan personID wajib diisi", ErrInvalidInput)
	}

	return s.immediateTx(ctx, func(ctx context.Context, conn *sql.Conn) error {
		if err := requireActiveAdminTx(ctx, conn, actorID); err != nil {
			return err
		}
		if err := pastikanKelasAda(ctx, conn, classID); err != nil {
			return err
		}

		var kehadiran int
		if err := conn.QueryRowContext(ctx,
			`SELECT COUNT(*)
			   FROM attendance_records ar
			   JOIN attendance_sessions s ON s.id = ar.session_id
			  WHERE ar.person_id = ? AND s.class_id = ? AND s.status = 'open'`,
			personID, classID).Scan(&kehadiran); err != nil {
			return fmt.Errorf("auth: gagal memeriksa kehadiran peserta %s di kelas %s: %w", personID, classID, err)
		}
		if kehadiran > 0 {
			return fmt.Errorf("%w: peserta %s memiliki kehadiran pada sesi terbuka kelas %s", ErrPolicyConflict, personID, classID)
		}

		hasil, err := conn.ExecContext(ctx,
			`DELETE FROM class_memberships WHERE class_id = ? AND person_id = ?`, classID, personID)
		if err != nil {
			return fmt.Errorf("auth: gagal menghapus anggota kelas %s: %w", classID, err)
		}
		jumlah, err := hasil.RowsAffected()
		if err != nil {
			return fmt.Errorf("auth: jumlah anggota terhapus tidak dapat dibaca: %w", err)
		}
		if jumlah == 0 {
			return fmt.Errorf("%w: keanggotaan peserta %s di kelas %s", ErrNotFound, personID, classID)
		}
		return s.auditTx(ctx, conn, actorID, "class.member.removed", "class", classID,
			"Peserta dihapus dari kelas",
			map[string]string{"count": "1"})
	})
}

// scanKelasList memindai hasil kolomKelas.
func scanKelasList(rows *sql.Rows) ([]Class, error) {
	daftar := make([]Class, 0, 8)
	for rows.Next() {
		k, err := scanKelas(rows)
		if err != nil {
			return nil, fmt.Errorf("auth: gagal membaca baris kelas: %w", err)
		}
		daftar = append(daftar, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("auth: penelusuran kelas gagal: %w", err)
	}
	return daftar, nil
}

func validKodeKelas(nilai string) bool {
	if len(nilai) < 1 || len(nilai) > 32 {
		return false
	}
	for _, r := range nilai {
		if r > 127 {
			return false
		}
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') &&
			!(r >= '0' && r <= '9') && r != '-' {
			return false
		}
	}
	return true
}

func validJudulKelas(nilai string) bool {
	if nilai == "" || utf8.RuneCountInString(nilai) > 120 {
		return false
	}
	return !memuatKontrol(nilai)
}

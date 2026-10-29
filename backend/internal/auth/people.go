package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/audit"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/ids"
)

// personColumns adalah proyeksi peserta yang dipakai daftar dan detail.
//
// `consent_state` dan `enrollment_state` adalah nilai turunan baca-saja untuk
// kontrak §5.2. Keduanya TIDAK membuat consent/enrollment; tabel biometrik
// tetap tidak ditulis oleh paket ini. Pemetaan consent mengikuti kosakata
// kontrak (`active`/`none`/`withdrawn`) dari kolom skema
// (`granted`/`unknown`/`withdrawn`/`expired`).
const personColumns = `p.id, p.external_code, p.display_name, p.status,
	COALESCE(p.note, ''), COALESCE(p.identity_reference, ''), p.identity_verified_at,
	COALESCE(p.account_user_id, ''), p.created_at, p.consent_state,
	CASE WHEN EXISTS (
		SELECT 1 FROM face_enrollments fe
		 WHERE fe.person_id = p.id AND fe.status = 'active'
	) THEN 'enrolled' ELSE 'none' END`

// PersonQuery membatasi daftar peserta. IsAdmin=false wajib menyertakan
// ScopeUserID milik dosen; query lalu hanya mengembalikan peserta pada kelas
// yang ditugaskan kepadanya. ClassID, bila diisi, menyaring satu kelas.
type PersonQuery struct {
	IsAdmin        bool
	ScopeUserID    string
	Status         string
	ClassID        string
	Limit          int
	AfterCreatedAt string
	AfterID        string
}

// PersonUpdate membedakan "tidak diubah" (nil) dari "diubah menjadi kosong"
// (pointer ke string kosong). External code/NIM tidak dapat diubah.
type PersonUpdate struct {
	DisplayName       *string
	Status            *string
	Note              *string
	IdentityReference *string
}

// scanPerson memindai proyeksi personColumns.
func scanPerson(sc rowScanner) (Person, error) {
	var (
		p                  Person
		note, idRef        string
		verified, account  sql.NullString
		createdAt, consent string
		enrollmentState    string
	)
	if err := sc.Scan(&p.ID, &p.ExternalCode, &p.DisplayName, &p.Status,
		&note, &idRef, &verified, &account, &createdAt, &consent, &enrollmentState); err != nil {
		return Person{}, err
	}
	p.Note = note
	p.IdentityReference = idRef
	p.AccountUserID = account.String
	p.ConsentState = consentUntukKontrak(consent)
	p.EnrollmentState = enrollmentState
	if verified.Valid && teks(verified.String) != "" {
		waktu, err := parseWaktu(verified.String)
		if err != nil {
			return Person{}, fmt.Errorf("auth: identity_verified_at peserta %s tidak dapat dibaca: %w", p.ID, err)
		}
		p.IdentityVerifiedAt = &waktu
	}
	waktu, err := parseWaktu(createdAt)
	if err != nil {
		return Person{}, fmt.Errorf("auth: created_at peserta %s tidak dapat dibaca: %w", p.ID, err)
	}
	p.CreatedAt = waktu
	return p, nil
}

// consentUntukKontrak memetakan nilai kolom skema ke kosakata kontrak.
func consentUntukKontrak(nilai string) string {
	switch teks(nilai) {
	case "granted":
		return "active"
	case "withdrawn", "expired":
		return "withdrawn"
	default:
		return "none"
	}
}

// personByIDTx membaca satu peserta aktif (belum dihapus) di dalam transaksi.
func personByIDTx(ctx context.Context, conn *sql.Conn, personID string) (Person, error) {
	p, err := scanPerson(conn.QueryRowContext(ctx,
		`SELECT `+personColumns+` FROM people p
		  WHERE p.id = ? AND p.deleted_at IS NULL`, personID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Person{}, fmt.Errorf("%w: peserta %s", ErrNotFound, personID)
		}
		return Person{}, fmt.Errorf("auth: gagal membaca peserta %s: %w", personID, err)
	}
	return p, nil
}

// PersonByID membaca detail peserta yang belum dihapus.
func (s *Store) PersonByID(ctx context.Context, personID string) (Person, error) {
	personID = teks(personID)
	if personID == "" {
		return Person{}, fmt.Errorf("%w: personID wajib diisi", ErrInvalidInput)
	}
	p, err := scanPerson(s.db.QueryRowContext(ctx,
		`SELECT `+personColumns+` FROM people p
		  WHERE p.id = ? AND p.deleted_at IS NULL`, personID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Person{}, fmt.Errorf("%w: peserta %s", ErrNotFound, personID)
		}
		return Person{}, fmt.Errorf("auth: gagal membaca peserta %s: %w", personID, err)
	}
	return p, nil
}

// ListPeople mengembalikan peserta sesuai cakupan pemanggil. Pemanggil HTTP
// sudah memeriksa peran; query tetap menyaring cakupan dosen agar kesalahan
// handler tidak membocorkan peserta kelas lain.
func (s *Store) ListPeople(ctx context.Context, q PersonQuery) ([]Person, error) {
	if q.Limit <= 0 {
		return nil, fmt.Errorf("%w: limit wajib positif", ErrInvalidInput)
	}
	if !q.IsAdmin && teks(q.ScopeUserID) == "" {
		return nil, fmt.Errorf("%w: ScopeUserID wajib untuk dosen", ErrInvalidInput)
	}
	if q.Status != "" && q.Status != "active" && q.Status != "inactive" {
		return nil, fmt.Errorf("%w: status peserta tidak dikenal", ErrInvalidInput)
	}
	if !q.IsAdmin && teks(q.ClassID) != "" {
		assigned, err := s.IsLecturerAssigned(ctx, q.ScopeUserID, q.ClassID)
		if err != nil {
			return nil, err
		}
		if !assigned {
			return nil, fmt.Errorf("%w: kelas di luar penugasan", ErrNotFound)
		}
	}

	kueri := `SELECT ` + personColumns + ` FROM people p
	           WHERE p.deleted_at IS NULL`
	args := make([]any, 0, 6)

	if !q.IsAdmin {
		kueri += `
		   AND EXISTS (
		        SELECT 1
		          FROM class_memberships cm
		          JOIN class_lecturers cl ON cl.class_id = cm.class_id
		          JOIN classes c ON c.id = cm.class_id AND c.deleted_at IS NULL
		         WHERE cm.person_id = p.id
		           AND cm.left_at IS NULL
		           AND cl.user_id = ?
		   )`
		args = append(args, teks(q.ScopeUserID))
	}
	if q.Status != "" {
		kueri += ` AND p.status = ?`
		args = append(args, q.Status)
	}
	if teks(q.ClassID) != "" {
		kueri += `
		   AND EXISTS (
		        SELECT 1 FROM class_memberships cm2
		         WHERE cm2.person_id = p.id
		           AND cm2.class_id = ?
		           AND cm2.left_at IS NULL
		   )`
		args = append(args, teks(q.ClassID))
	}
	if teks(q.AfterID) != "" {
		if teks(q.AfterCreatedAt) == "" {
			return nil, fmt.Errorf("%w: kursor peserta tanpa waktu", ErrInvalidInput)
		}
		kueri += ` AND (p.created_at > ? OR (p.created_at = ? AND p.id > ?))`
		args = append(args, teks(q.AfterCreatedAt), teks(q.AfterCreatedAt), teks(q.AfterID))
	}
	kueri += ` ORDER BY p.created_at, p.id LIMIT ?`
	args = append(args, q.Limit)

	rows, err := s.db.QueryContext(ctx, kueri, args...)
	if err != nil {
		return nil, fmt.Errorf("auth: gagal membaca daftar peserta: %w", err)
	}
	defer rows.Close()

	daftar := make([]Person, 0, q.Limit)
	for rows.Next() {
		p, err := scanPerson(rows)
		if err != nil {
			return nil, fmt.Errorf("auth: gagal membaca baris peserta: %w", err)
		}
		daftar = append(daftar, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("auth: penelusuran peserta gagal: %w", err)
	}
	return daftar, nil
}

// CreatePerson mencatat peserta dari roster kampus oleh admin aktif.
// Referensi kampus nonkosong adalah attestation eksplisit admin; tanpa
// referensi, peserta tetap belum terverifikasi dan tidak dapat ditautkan.
func (s *Store) CreatePerson(ctx context.Context, actorID, externalCode, displayName, note, identityReference string) (Person, error) {
	actorID = teks(actorID)
	externalCode = teks(externalCode)
	displayName = teks(displayName)
	note = teks(note)
	identityReference = teks(identityReference)

	if actorID == "" || externalCode == "" || displayName == "" {
		return Person{}, fmt.Errorf("%w: actorID, external_code, dan display_name wajib diisi", ErrInvalidInput)
	}
	if !validExternalCode(externalCode) {
		return Person{}, fmt.Errorf("%w: external_code harus 1-32 huruf/angka/tanda hubung", ErrInvalidInput)
	}
	if !validNama(displayName) {
		return Person{}, fmt.Errorf("%w: display_name harus 1-120 karakter tanpa karakter kontrol", ErrInvalidInput)
	}
	if utf8.RuneCountInString(note) > 200 || memuatKontrol(note) {
		return Person{}, fmt.Errorf("%w: note maksimum 200 karakter tanpa karakter kontrol", ErrInvalidInput)
	}
	if utf8.RuneCountInString(identityReference) > 120 || memuatKontrol(identityReference) {
		return Person{}, fmt.Errorf("%w: identity_reference maksimum 120 karakter tanpa karakter kontrol", ErrInvalidInput)
	}

	var hasil Person
	err := s.immediateTx(ctx, func(ctx context.Context, conn *sql.Conn) error {
		if err := requireActiveAdminTx(ctx, conn, actorID); err != nil {
			return err
		}

		// Pemeriksaan di bawah kunci tulis memberi galat domain yang stabil;
		// indeks global tetap menjadi penjaga terakhir di database.
		var ada string
		if err := conn.QueryRowContext(ctx,
			`SELECT id FROM people WHERE external_code = ? LIMIT 1`, externalCode).Scan(&ada); err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("auth: gagal memeriksa kode peserta: %w", err)
			}
		} else {
			return fmt.Errorf("%w: %s", ErrDuplicateExternalCode, externalCode)
		}

		personID := ids.New()
		sekarang := s.waktuSekarang()
		var verified any
		if identityReference != "" {
			verified = formatWaktu(sekarang)
		}
		_, err := conn.ExecContext(ctx,
			`INSERT INTO people
			   (id, external_code, display_name, status, note, identity_reference, identity_verified_at)
			 VALUES (?, ?, ?, 'active', ?, ?, ?)`,
			personID, externalCode, displayName, nullable(note), nullable(identityReference), verified,
		)
		if err != nil {
			if isUniqueViolation(err) {
				return fmt.Errorf("%w: %s", ErrDuplicateExternalCode, externalCode)
			}
			return fmt.Errorf("auth: gagal membuat peserta: %w", err)
		}

		if err := s.auditTx(ctx, conn, actorID, "person.created", "person", personID,
			"Peserta dicatat dari roster oleh admin",
			map[string]string{"method": "admin_roster_entry", "status_after": "active"}); err != nil {
			return err
		}
		if verified != nil {
			if err := s.auditTx(ctx, conn, actorID, "person.identity_verified", "person", personID,
				"Identitas peserta ditegaskan admin berdasarkan referensi kampus",
				map[string]string{"method": "admin_roster_reference", "status_after": "verified"}); err != nil {
				return err
			}
		}

		p, err := personByIDTx(ctx, conn, personID)
		if err != nil {
			return err
		}
		hasil = p
		return nil
	})
	if err != nil {
		return Person{}, err
	}
	return hasil, nil
}

// UpdatePerson mengubah field nonbiometrik peserta. External code/NIM sengaja
// tidak dapat diubah agar identitas roster tidak berpindah baris diam-diam.
func (s *Store) UpdatePerson(ctx context.Context, actorID, personID string, update PersonUpdate) (Person, error) {
	actorID, personID = teks(actorID), teks(personID)
	if actorID == "" || personID == "" {
		return Person{}, fmt.Errorf("%w: actorID dan personID wajib diisi", ErrInvalidInput)
	}
	if update.DisplayName == nil && update.Status == nil && update.Note == nil && update.IdentityReference == nil {
		return Person{}, fmt.Errorf("%w: minimal satu field peserta wajib diisi", ErrInvalidInput)
	}

	var hasil Person
	err := s.immediateTx(ctx, func(ctx context.Context, conn *sql.Conn) error {
		if err := requireActiveAdminTx(ctx, conn, actorID); err != nil {
			return err
		}
		sebelum, err := personByIDTx(ctx, conn, personID)
		if err != nil {
			return err
		}

		baru := sebelum
		fields := make([]string, 0, 4)
		if update.DisplayName != nil {
			nilai := teks(*update.DisplayName)
			if !validNama(nilai) {
				return fmt.Errorf("%w: display_name harus 1-120 karakter tanpa karakter kontrol", ErrInvalidInput)
			}
			if nilai != sebelum.DisplayName {
				baru.DisplayName = nilai
				fields = append(fields, "display_name")
			}
		}
		if update.Status != nil {
			nilai := teks(*update.Status)
			if nilai != "active" && nilai != "inactive" {
				return fmt.Errorf("%w: status peserta harus active atau inactive", ErrInvalidInput)
			}
			if nilai != sebelum.Status {
				baru.Status = nilai
				fields = append(fields, "status")
			}
		}
		if update.Note != nil {
			nilai := teks(*update.Note)
			if utf8.RuneCountInString(nilai) > 200 || memuatKontrol(nilai) {
				return fmt.Errorf("%w: note maksimum 200 karakter tanpa karakter kontrol", ErrInvalidInput)
			}
			if nilai != sebelum.Note {
				baru.Note = nilai
				fields = append(fields, "note")
			}
		}
		identityChanged := false
		if update.IdentityReference != nil {
			nilai := teks(*update.IdentityReference)
			if utf8.RuneCountInString(nilai) > 120 || memuatKontrol(nilai) {
				return fmt.Errorf("%w: identity_reference maksimum 120 karakter tanpa karakter kontrol", ErrInvalidInput)
			}
			if nilai != sebelum.IdentityReference || (nilai != "" && sebelum.IdentityVerifiedAt == nil) {
				if sebelum.AccountUserID != "" {
					return fmt.Errorf("%w: lepas tautan akun sebelum mengubah referensi identitas", ErrConflict)
				}
				baru.IdentityReference = nilai
				identityChanged = true
				fields = append(fields, "identity_reference")
			}
		}

		if len(fields) == 0 {
			hasil = sebelum
			return nil
		}

		verified := any(nil)
		if identityChanged && baru.IdentityReference == "" {
			baru.IdentityVerifiedAt = nil
		} else if identityChanged {
			sekarang := s.waktuSekarang()
			baru.IdentityVerifiedAt = &sekarang
			verified = formatWaktu(*baru.IdentityVerifiedAt)
		} else if sebelum.IdentityVerifiedAt != nil {
			verified = formatWaktu(*sebelum.IdentityVerifiedAt)
		}
		if _, err := conn.ExecContext(ctx,
			`UPDATE people
			    SET display_name = ?, status = ?, note = ?, identity_reference = ?,
			        identity_verified_at = ?, updated_at = ?
			  WHERE id = ? AND deleted_at IS NULL`,
			baru.DisplayName, baru.Status, nullable(baru.Note), nullable(baru.IdentityReference),
			verified, formatWaktu(s.waktuSekarang()), personID,
		); err != nil {
			return fmt.Errorf("auth: gagal mengubah peserta %s: %w", personID, err)
		}

		meta := map[string]string{"fields": strings.Join(fields, ",")}
		if baru.Status != sebelum.Status {
			meta["status_before"] = sebelum.Status
			meta["status_after"] = baru.Status
		}
		if err := s.auditTx(ctx, conn, actorID, "person.updated", "person", personID,
			"Data peserta diubah oleh admin", meta); err != nil {
			return err
		}
		if identityChanged && baru.IdentityReference != "" {
			if err := s.auditTx(ctx, conn, actorID, "person.identity_verified", "person", personID,
				"Identitas peserta ditegaskan admin berdasarkan referensi kampus",
				map[string]string{"method": "admin_roster_reference", "status_after": "verified"}); err != nil {
				return err
			}
		}

		hasil, err = personByIDTx(ctx, conn, personID)
		return err
	})
	if err != nil {
		return Person{}, err
	}
	return hasil, nil
}

// DeletePerson menghapus lunak peserta, melepas tautan akun, dan menghapus
// keanggotaan kelas. Riwayat kehadiran tidak dihapus; bila sudah ada catatan
// kehadiran atau data enrollment/template, operasi ditolak ErrPolicyConflict:
// jalur nonbiometrik ini tidak menerapkan penghapusan biometrik D-015.
func (s *Store) DeletePerson(ctx context.Context, actorID, personID string) error {
	actorID, personID = teks(actorID), teks(personID)
	if actorID == "" || personID == "" {
		return fmt.Errorf("%w: actorID dan personID wajib diisi", ErrInvalidInput)
	}

	return s.immediateTx(ctx, func(ctx context.Context, conn *sql.Conn) error {
		if err := requireActiveAdminTx(ctx, conn, actorID); err != nil {
			return err
		}
		sebelum, err := personByIDTx(ctx, conn, personID)
		if err != nil {
			return err
		}

		var kehadiran int
		if err := conn.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM attendance_records WHERE person_id = ?`, personID).Scan(&kehadiran); err != nil {
			return fmt.Errorf("auth: gagal memeriksa riwayat kehadiran peserta %s: %w", personID, err)
		}
		if kehadiran > 0 {
			return fmt.Errorf("%w: peserta %s memiliki riwayat kehadiran", ErrPolicyConflict, personID)
		}
		if err := guardPersonBiometricDeletionTx(ctx, conn, personID); err != nil {
			return err
		}

		hasilHapus, err := conn.ExecContext(ctx,
			`DELETE FROM class_memberships WHERE person_id = ?`, personID)
		if err != nil {
			return fmt.Errorf("auth: gagal menghapus keanggotaan kelas peserta %s: %w", personID, err)
		}
		anggota, err := hasilHapus.RowsAffected()
		if err != nil {
			return fmt.Errorf("auth: jumlah keanggotaan terhapus tidak dapat dibaca: %w", err)
		}

		sekarang := formatWaktu(s.waktuSekarang())
		if _, err := conn.ExecContext(ctx,
			`UPDATE people
			    SET deleted_at = ?, account_user_id = NULL, updated_at = ?
			  WHERE id = ? AND deleted_at IS NULL`,
			sekarang, sekarang, personID,
		); err != nil {
			return fmt.Errorf("auth: gagal menghapus peserta %s: %w", personID, err)
		}

		return s.auditTx(ctx, conn, actorID, "person.deleted", "person", personID,
			"Peserta dihapus lunak oleh admin",
			map[string]string{
				"status_before": sebelum.Status,
				"count":         fmt.Sprintf("%d", anggota),
			})
	})
}

// requireActiveAdminTx membaca role/status di dalam transaksi mutasi, sehingga
// aktor yang dicabut haknya tidak dapat memakai hasil pemeriksaan lama.
func requireActiveAdminTx(ctx context.Context, conn *sql.Conn, actorID string) error {
	actor, err := userByIDTx(ctx, conn, actorID)
	if err != nil {
		return err
	}
	if actor.Role != RoleAdmin || actor.Status != StatusActive {
		return fmt.Errorf("%w: mutasi roster memerlukan admin aktif", ErrActorUnauthorized)
	}
	return nil
}

// guardPersonBiometricDeletionTx gagal-aman bila data enrollment/template
// sudah ada. Tabel template masa depan hanya dibaca; tidak dibuat/dihapus.
func guardPersonBiometricDeletionTx(ctx context.Context, conn *sql.Conn, personID string) error {
	for _, table := range []string{"face_enrollments", "face_templates"} {
		var exists int
		if err := conn.QueryRowContext(ctx,
			`SELECT EXISTS (SELECT 1 FROM sqlite_schema WHERE type = 'table' AND name = ?)`,
			table).Scan(&exists); err != nil {
			return fmt.Errorf("auth: gagal memeriksa tabel retensi %s: %w", table, err)
		}
		if exists == 0 {
			continue
		}
		var referenced int
		// Nama tabel berasal dari allowlist tetap di atas, bukan masukan klien.
		if err := conn.QueryRowContext(ctx,
			`SELECT EXISTS (SELECT 1 FROM `+table+` WHERE person_id = ?)`,
			personID).Scan(&referenced); err != nil {
			return fmt.Errorf("auth: gagal memeriksa retensi %s: %w", table, err)
		}
		if referenced != 0 {
			return fmt.Errorf("%w: peserta memiliki data %s; penghapusan biometrik belum tersedia", ErrPolicyConflict, table)
		}
	}
	return nil
}

// auditTx menulis audit pada transaksi yang sama. Kegagalan audit menggagalkan
// transaksi sehingga tidak ada perubahan tanpa jejak audit.
func (s *Store) auditTx(ctx context.Context, conn *sql.Conn, actorID, action, entityType, entityID, reason string, metadata map[string]string) error {
	if teks(actorID) == "" {
		return fmt.Errorf("%w: actorID audit wajib diisi", ErrInvalidInput)
	}
	if err := audit.NewWriter(conn, s.clk).Record(ctx, audit.Event{
		ActorType:  audit.ActorUser,
		ActorID:    actorID,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Result:     audit.ResultSuccess,
		Reason:     reason,
		Metadata:   metadata,
	}); err != nil {
		return fmt.Errorf("auth: gagal menulis audit %s: %w", action, err)
	}
	return nil
}

func validExternalCode(nilai string) bool {
	if len(nilai) < 1 || len(nilai) > 32 {
		return false
	}
	for _, r := range nilai {
		if r > unicode.MaxASCII {
			return false
		}
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') &&
			!(r >= '0' && r <= '9') && r != '-' {
			return false
		}
	}
	return true
}

func validNama(nilai string) bool {
	if nilai == "" || utf8.RuneCountInString(nilai) > 120 {
		return false
	}
	return !memuatKontrol(nilai)
}

func memuatKontrol(nilai string) bool {
	for _, r := range nilai {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

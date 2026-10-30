package authz

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// QueryRower adalah bagian minimal dari *sql.DB yang dibutuhkan pembaca
// cakupan. Menerima antarmuka ini membuat *database.DB dapat dipakai langsung
// tanpa konversi tipe di pemanggil.
type QueryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// SQLDirectory memenuhi Directory dengan membaca tabel cakupan secara langsung.
// Pendekatan ini menjaga logika cakupan tetap di satu tempat dan tidak
// bergantung pada bentuk internal paket lain; nilainya hanya fakta keanggotaan.
type SQLDirectory struct {
	db QueryRower
}

// NewSQLDirectory membuat pembaca cakupan di atas koneksi database yang ada.
func NewSQLDirectory(db QueryRower) *SQLDirectory {
	return &SQLDirectory{db: db}
}

// IsLecturerAssigned melaporkan apakah pengguna ditugaskan pada kelas.
func (d *SQLDirectory) IsLecturerAssigned(ctx context.Context, userID, classID string) (bool, error) {
	return d.ada(ctx,
		`SELECT 1 FROM class_lecturers WHERE user_id = ? AND class_id = ?`,
		userID, classID)
}

// IsClassMember melaporkan apakah peserta masih menjadi anggota kelas.
func (d *SQLDirectory) IsClassMember(ctx context.Context, personID, classID string) (bool, error) {
	return d.ada(ctx,
		`SELECT 1 FROM class_memberships
		  WHERE person_id = ? AND class_id = ? AND left_at IS NULL`,
		personID, classID)
}

// IsPersonInLecturerScope melaporkan apakah peserta adalah anggota salah satu
// kelas yang ditugaskan kepada dosen tersebut.
func (d *SQLDirectory) IsPersonInLecturerScope(ctx context.Context, userID, personID string) (bool, error) {
	return d.ada(ctx,
		`SELECT 1 FROM class_lecturers AS cl
		   JOIN class_memberships AS cm ON cm.class_id = cl.class_id
		  WHERE cl.user_id = ? AND cm.person_id = ? AND cm.left_at IS NULL`,
		userID, personID)
}

// PersonForUser mengembalikan baris peserta yang tertaut ke akun mahasiswa.
func (d *SQLDirectory) PersonForUser(ctx context.Context, userID string) (string, bool, error) {
	if d == nil || d.db == nil {
		return "", false, errors.New("authz: pembaca cakupan tidak siap")
	}
	var personID string
	err := d.db.QueryRowContext(ctx,
		`SELECT id FROM people
		  WHERE account_user_id = ? AND deleted_at IS NULL AND status = 'active'
		    AND identity_verified_at IS NOT NULL
		    AND length(trim(COALESCE(identity_reference, ''))) > 0`,
		userID).Scan(&personID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("authz: membaca pengaitan peserta gagal: %w", err)
	}
	return personID, true, nil
}

// ada menjalankan kueri keberadaan dan menerjemahkan hasilnya menjadi boolean.
func (d *SQLDirectory) ada(ctx context.Context, query string, args ...any) (bool, error) {
	if d == nil || d.db == nil {
		return false, errors.New("authz: pembaca cakupan tidak siap")
	}
	var satu int
	err := d.db.QueryRowContext(ctx, query, args...).Scan(&satu)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("authz: pemeriksaan cakupan gagal: %w", err)
	}
	return true, nil
}

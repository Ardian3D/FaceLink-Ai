package migrations

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
)

// schemaPath menunjuk berkas migrasi yang benar-benar dikirim bersama backend,
// bukan berkas buatan pengujian.
func schemaPath(t *testing.T) (string, string) {
	t.Helper()

	dir, err := filepath.Abs(filepath.Join("..", "..", "migrations"))
	if err != nil {
		t.Fatalf("resolusi jalur migrasi gagal: %v", err)
	}
	return dir, filepath.Join(dir, "00001_init_core_schema.sql")
}

func TestSkemaNyataMenerapkanSeluruhTabelNonbiometrik(t *testing.T) {
	dir, _ := schemaPath(t)
	db := openDB(t)
	ctx := context.Background()

	status, err := Run(ctx, db, dir)
	if err != nil {
		t.Fatalf("penerapan skema nyata gagal: %v", err)
	}
	if status.CurrentVersion != 1 {
		t.Errorf("CurrentVersion = %d, ingin 1", status.CurrentVersion)
	}

	want := []string{
		"users", "operator_tokens", "roles", "people", "classes",
		"class_memberships", "attendance_sessions", "attendance_records",
		"consent_records", "face_enrollments", "devices", "audit_events",
		"idempotency_records",
	}
	for _, name := range want {
		var found int
		if err := db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&found); err != nil {
			t.Fatalf("pembacaan sqlite_master untuk %s gagal: %v", name, err)
		}
		if found != 1 {
			t.Errorf("tabel %s tidak dibuat oleh 00001_init_core_schema.sql", name)
		}
	}
}

func TestSkemaNyataTidakMembuatTabelBiometrik(t *testing.T) {
	dir, _ := schemaPath(t)
	db := openDB(t)
	ctx := context.Background()

	if _, err := Run(ctx, db, dir); err != nil {
		t.Fatalf("penerapan skema nyata gagal: %v", err)
	}

	// Tabel ini bergantung pada rancangan manajemen kunci (D-030) dan sengaja
	// ditunda sampai tugas tersebut disetujui.
	for _, name := range []string{"face_templates", "face_enrollment_samples"} {
		var found int
		if err := db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&found); err != nil {
			t.Fatalf("pembacaan sqlite_master untuk %s gagal: %v", name, err)
		}
		if found != 0 {
			t.Errorf("tabel %s dibuat padahal menunggu rancangan D-030", name)
		}
	}
}

func TestSkemaNyataMenyemaiPeranSistem(t *testing.T) {
	dir, _ := schemaPath(t)
	db := openDB(t)
	ctx := context.Background()

	if _, err := Run(ctx, db, dir); err != nil {
		t.Fatalf("penerapan skema nyata gagal: %v", err)
	}

	rows, err := db.QueryContext(ctx, `SELECT name FROM roles ORDER BY name`)
	if err != nil {
		t.Fatalf("pembacaan tabel roles gagal: %v", err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("pembacaan baris roles gagal: %v", err)
		}
		got = append(got, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("penelusuran roles gagal: %v", err)
	}

	wantList := []string{"admin", "device", "operator"}
	if len(got) != len(wantList) {
		t.Fatalf("roles berisi %v, ingin %v", got, wantList)
	}
	for i := range wantList {
		if got[i] != wantList[i] {
			t.Errorf("roles[%d] = %q, ingin %q", i, got[i], wantList[i])
		}
	}
}

func TestSkemaNyataMenegakkanForeignKeys(t *testing.T) {
	dir, _ := schemaPath(t)
	db := openDB(t)
	ctx := context.Background()

	if _, err := Run(ctx, db, dir); err != nil {
		t.Fatalf("penerapan skema nyata gagal: %v", err)
	}

	// class_memberships menunjuk classes dan people; keduanya harus ada lebih
	// dahulu (docs/DATA_MODEL.md §4.6).
	if _, err := db.ExecContext(ctx,
		`INSERT INTO class_memberships (class_id, person_id) VALUES ('kelas-tidak-ada', 'orang-tidak-ada')`); err == nil {
		t.Error("penyisipan ke class_memberships dengan induk tidak ada harus ditolak")
	}
}

func TestSkemaNyataMenegakkanUnikSatuSesiTerbukaPerKelas(t *testing.T) {
	dir, _ := schemaPath(t)
	db := openDB(t)
	ctx := context.Background()

	if _, err := Run(ctx, db, dir); err != nil {
		t.Fatalf("penerapan skema nyata gagal: %v", err)
	}

	seed := []string{
		`INSERT INTO users (id, email, display_name, password_hash, role) VALUES ('u1','a@contoh.test','Operator','x','operator')`,
		`INSERT INTO classes (id, code, title) VALUES ('c1','KLS1','Kelas Satu')`,
		`INSERT INTO attendance_sessions (id, class_id, status, closes_at, created_by)
			VALUES ('s1','c1','open','2026-10-15T10:00:00Z','u1')`,
	}
	for _, stmt := range seed {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("penyiapan data uji gagal pada %q: %v", stmt, err)
		}
	}

	// Indeks unik parsial uq_sessions_one_open_per_class melarang sesi terbuka
	// kedua pada kelas yang sama (docs/DATA_MODEL.md §4.7).
	_, err := db.ExecContext(ctx,
		`INSERT INTO attendance_sessions (id, class_id, status, closes_at, created_by)
			VALUES ('s2','c1','open','2026-10-15T11:00:00Z','u1')`)
	if err == nil {
		t.Error("sesi terbuka kedua pada kelas yang sama harus ditolak oleh uq_sessions_one_open_per_class")
	}

	// Sesi tertutup kedua pada kelas yang sama tetap diizinkan.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO attendance_sessions (id, class_id, status, closes_at, created_by)
			VALUES ('s3','c1','closed','2026-10-15T09:00:00Z','u1')`); err != nil {
		t.Errorf("sesi tertutup kedua seharusnya diizinkan: %v", err)
	}
}

func TestSkemaNyataMenegakkanAturanAbsensiManual(t *testing.T) {
	dir, _ := schemaPath(t)
	db := openDB(t)
	ctx := context.Background()

	if _, err := Run(ctx, db, dir); err != nil {
		t.Fatalf("penerapan skema nyata gagal: %v", err)
	}

	seed := []string{
		`INSERT INTO users (id, email, display_name, password_hash, role) VALUES ('u1','a@contoh.test','Operator','x','operator')`,
		`INSERT INTO people (id, external_code, display_name) VALUES ('p1','P001','Peserta Satu')`,
		`INSERT INTO classes (id, code, title) VALUES ('c1','KLS1','Kelas Satu')`,
		`INSERT INTO attendance_sessions (id, class_id, status, closes_at, created_by)
			VALUES ('s1','c1','open','2026-10-15T10:00:00Z','u1')`,
	}
	for _, stmt := range seed {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("penyiapan data uji gagal pada %q: %v", stmt, err)
		}
	}

	// ck_attendance_manual_fields mewajibkan created_by dan reason terisi untuk
	// absensi manual (docs/DATA_MODEL.md §4.8).
	if _, err := db.ExecContext(ctx,
		`INSERT INTO attendance_records (id, session_id, person_id, status, source, checked_in_at)
			VALUES ('r1','s1','p1','present','manual','2026-10-15T08:00:00Z')`); err == nil {
		t.Error("absensi manual tanpa created_by dan reason harus ditolak")
	}

	if _, err := db.ExecContext(ctx,
		`INSERT INTO attendance_records (id, session_id, person_id, status, source, checked_in_at, created_by, reason)
			VALUES ('r2','s1','p1','present','manual','2026-10-15T08:00:00Z','u1','Kamera gagal membaca')`); err != nil {
		t.Errorf("absensi manual lengkap seharusnya diterima: %v", err)
	}

	// uq_attendance_session_person mencegah absensi ganda pada sesi yang sama.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO attendance_records (id, session_id, person_id, status, source, checked_in_at, created_by, reason)
			VALUES ('r3','s1','p1','present','manual','2026-10-15T09:00:00Z','u1','Duplikat')`); err == nil {
		t.Error("absensi ganda pada sesi dan peserta yang sama harus ditolak")
	}
}

func TestSkemaNyataDapatDiterapkanDuaKaliTanpaPerubahan(t *testing.T) {
	dir, _ := schemaPath(t)
	db := openDB(t)
	ctx := context.Background()

	first, err := Run(ctx, db, dir)
	if err != nil {
		t.Fatalf("penerapan pertama gagal: %v", err)
	}
	second, err := Run(ctx, db, dir)
	if err != nil {
		t.Fatalf("penerapan kedua gagal: %v", err)
	}

	if first.CurrentVersion != second.CurrentVersion {
		t.Errorf("versi berubah dari %d menjadi %d pada penerapan kedua",
			first.CurrentVersion, second.CurrentVersion)
	}
	if second.Applied != 0 {
		t.Errorf("Applied pada penerapan kedua = %d, ingin 0", second.Applied)
	}
}

func TestSkemaNyataTetapUtuhSetelahCheckpoint(t *testing.T) {
	dir, _ := schemaPath(t)
	db := openDB(t)
	ctx := context.Background()

	if _, err := Run(ctx, db, dir); err != nil {
		t.Fatalf("penerapan skema nyata gagal: %v", err)
	}
	if err := db.Checkpoint(ctx); err != nil {
		t.Fatalf("Checkpoint() gagal: %v", err)
	}
	if err := db.IntegrityCheck(ctx); err != nil {
		t.Fatalf("integrity_check melaporkan kerusakan setelah checkpoint: %v", err)
	}
}

var _ = database.JournalModeWAL

package migrations

import (
	"context"
	"os"
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
	if status.CurrentVersion != 3 {
		t.Errorf("CurrentVersion = %d, ingin 3", status.CurrentVersion)
	}

	want := []string{
		"users", "session_tokens", "roles", "people", "classes",
		"class_memberships", "attendance_sessions", "attendance_records",
		"consent_records", "face_enrollments", "devices", "audit_events",
		"idempotency_records", "user_identities", "auth_nonces", "class_lecturers",
		"session_token_history",
	}
	for _, name := range want {
		var found int
		if err := db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&found); err != nil {
			t.Fatalf("pembacaan sqlite_master untuk %s gagal: %v", name, err)
		}
		if found != 1 {
			t.Errorf("tabel %s tidak ada setelah seluruh migrasi diterapkan", name)
		}
	}

	// Tabel token kata sandi digantikan sesi opak pada migrasi 00002 (FL-011).
	var usang int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='operator_tokens'`).Scan(&usang); err != nil {
		t.Fatalf("pembacaan sqlite_master untuk operator_tokens gagal: %v", err)
	}
	if usang != 0 {
		t.Error("tabel operator_tokens masih ada padahal sudah digantikan session_tokens")
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

	wantList := []string{"admin", "dosen", "mahasiswa"}
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
		`INSERT INTO users (id, email, display_name, role) VALUES ('u1','a@contoh.test','Pengguna Uji','dosen')`,
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
		`INSERT INTO users (id, email, display_name, role) VALUES ('u1','a@contoh.test','Pengguna Uji','dosen')`,
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

// TestSkemaNyataMigrasiDatabaseLamaDenganData menutup syarat migrasi FL-011:
// database lama yang sudah berisi data domain harus tetap utuh setelah migrasi
// 00002 dan 00003 diterapkan, data sesi lama dibuang secara tercatat, dan
// seluruh foreign key tetap konsisten setelah tabel `users` dibangun ulang.
// Migrasi 00003 tidak mengubah data apa pun: ia hanya menambah tabel riwayat
// rotasi penyegar dan satu indeks.
func TestSkemaNyataMigrasiDatabaseLamaDenganData(t *testing.T) {
	dirPenuh, berkas00001 := schemaPath(t)
	ctx := context.Background()

	// 1. Siapkan database "lama": hanya migrasi 00001 yang diterapkan.
	dirLama := t.TempDir()
	isi, err := os.ReadFile(berkas00001)
	if err != nil {
		t.Fatalf("membaca %s gagal: %v", berkas00001, err)
	}
	if err := os.WriteFile(filepath.Join(dirLama, filepath.Base(berkas00001)), isi, 0o600); err != nil {
		t.Fatalf("menyiapkan direktori migrasi lama gagal: %v", err)
	}

	db := openDB(t)
	statusLama, err := Run(ctx, db, dirLama)
	if err != nil {
		t.Fatalf("penerapan skema lama gagal: %v", err)
	}
	if statusLama.CurrentVersion != 1 {
		t.Fatalf("versi skema lama = %d, ingin 1", statusLama.CurrentVersion)
	}

	lama := []string{
		`INSERT INTO users (id, email, display_name, password_hash, role, status)
			VALUES ('u1','  Lama@Contoh.Test ','Pengguna Lama','x','operator','active')`,
		`INSERT INTO users (id, email, display_name, password_hash, role, status)
			VALUES ('u2','nonaktif@contoh.test','Pengguna Nonaktif','x','admin','disabled')`,
		`INSERT INTO classes (id, code, title) VALUES ('c1','KLS1','Kelas Satu')`,
		`INSERT INTO attendance_sessions (id, class_id, status, closes_at, created_by)
			VALUES ('s1','c1','open','2026-10-15T10:00:00Z','u1')`,
		`INSERT INTO people (id, external_code, display_name) VALUES ('p1','P001','Peserta Satu')`,
		`INSERT INTO attendance_records (id, session_id, person_id, status, source, checked_in_at, created_by, reason)
			VALUES ('r1','s1','p1','present','manual','2026-10-15T08:00:00Z','u1','Kamera gagal membaca')`,
		`INSERT INTO operator_tokens (id, user_id, token_hash, expires_at)
			VALUES ('t1','u1','hash-sesi-lama','2026-10-16T00:00:00Z')`,
	}
	for _, stmt := range lama {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("penyiapan data lama gagal pada %q: %v", stmt, err)
		}
	}

	// 2. Terapkan migrasi baru di atas database berisi data.
	statusBaru, err := Run(ctx, db, dirPenuh)
	if err != nil {
		t.Fatalf("migrasi di atas database lama gagal: %v", err)
	}
	if statusBaru.CurrentVersion != 3 || statusBaru.Applied != 2 {
		t.Fatalf("status migrasi = %+v, ingin versi 3 dengan 2 migrasi diterapkan", statusBaru)
	}

	// 3. Data domain tetap ada.
	for tabel, ingin := range map[string]int{
		"users": 2, "classes": 1, "attendance_sessions": 1, "attendance_records": 1, "people": 1,
	} {
		var jumlah int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+tabel).Scan(&jumlah); err != nil {
			t.Fatalf("menghitung %s gagal: %v", tabel, err)
		}
		if jumlah != ingin {
			t.Errorf("jumlah baris %s = %d, ingin %d", tabel, jumlah, ingin)
		}
	}

	// 4. Pemetaan kolom yang berubah.
	var email, role, status string
	if err := db.QueryRowContext(ctx,
		`SELECT email, role, status FROM users WHERE id = 'u1'`).Scan(&email, &role, &status); err != nil {
		t.Fatalf("pembacaan pengguna lama gagal: %v", err)
	}
	if email != "lama@contoh.test" {
		t.Errorf("email = %q, ingin dinormalkan menjadi huruf kecil tanpa spasi tepi", email)
	}
	if role != "dosen" {
		t.Errorf("role = %q, ingin `operator` dipetakan menjadi `dosen`", role)
	}
	if status != "active" {
		t.Errorf("status = %q, ingin tetap `active`", status)
	}
	if err := db.QueryRowContext(ctx,
		`SELECT role, status FROM users WHERE id = 'u2'`).Scan(&role, &status); err != nil {
		t.Fatalf("pembacaan pengguna nonaktif gagal: %v", err)
	}
	if role != "admin" || status != "suspended" {
		t.Errorf("pengguna u2 = (%q,%q), ingin (admin,suspended)", role, status)
	}

	// 5. Sesi lama dibuang dan peristiwanya tercatat.
	var jumlahSesi int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM session_tokens`).Scan(&jumlahSesi); err != nil {
		t.Fatalf("penghitungan session_tokens gagal: %v", err)
	}
	if jumlahSesi != 0 {
		t.Errorf("session_tokens berisi %d baris, ingin 0 (sesi lama tidak dipetakan)", jumlahSesi)
	}
	var jumlahAudit int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM audit_events WHERE action = 'auth.session.reset_legacy'`).Scan(&jumlahAudit); err != nil {
		t.Fatalf("penghitungan audit reset sesi gagal: %v", err)
	}
	if jumlahAudit != 1 {
		t.Errorf("audit auth.session.reset_legacy = %d, ingin 1", jumlahAudit)
	}

	// 6. Foreign key konsisten setelah tabel users dibangun ulang.
	rows, err := db.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatalf("foreign_key_check gagal: %v", err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Error("foreign_key_check melaporkan pelanggaran setelah migrasi")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("penelusuran foreign_key_check gagal: %v", err)
	}

	// 7. Constraint baru benar-benar berlaku.
	harusGagal := map[string]string{
		"role di luar tiga role manusia": `INSERT INTO users (id, email, display_name, role)
			VALUES ('u9','x@contoh.test','Salah','operator')`,
		"status di luar lifecycle": `INSERT INTO users (id, email, display_name, role, status)
			VALUES ('u9','x@contoh.test','Salah','dosen','disabled')`,
		"identitas Google ganda": `INSERT INTO user_identities (id, user_id, provider, subject)
			VALUES ('i1','u1','google','sub-1'), ('i2','u2','google','sub-1')`,
		"batas absolut sesi": `INSERT INTO session_tokens (id, user_id, access_hash, access_expires_at, refresh_hash, refresh_expires_at, absolute_expires_at)
			VALUES ('sx','u1','a','2026-10-16T00:00:00Z','r','2026-11-20T00:00:00Z','2026-10-20T00:00:00Z')`,
		"penugasan dosen pada kelas yang tidak ada": `INSERT INTO class_lecturers (class_id, user_id, assigned_by)
			VALUES ('kelas-tidak-ada','u1','u2')`,
	}
	for nama, stmt := range harusGagal {
		if _, err := db.ExecContext(ctx, stmt); err == nil {
			t.Errorf("%s seharusnya ditolak skema", nama)
		}
	}

	// 8. Kolom pengaitan peserta dapat dipakai dan unik.
	if _, err := db.ExecContext(ctx,
		`UPDATE people SET account_user_id = 'u2' WHERE id = 'p1'`); err != nil {
		t.Fatalf("pengaitan akun peserta gagal: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO people (id, external_code, display_name, account_user_id) VALUES ('p2','P002','Peserta Dua','u2')`); err == nil {
		t.Error("dua peserta dengan akun yang sama seharusnya ditolak uq_people_account_user")
	}
}

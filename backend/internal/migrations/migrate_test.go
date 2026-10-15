package migrations

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
)

func writeMigration(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("penulisan migrasi %s gagal: %v", name, err)
	}
}

func openDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "facelink.db"))
	if err != nil {
		t.Fatalf("Open() gagal: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestLoadMengurutkanMigrasiBerdasarkanNomor(t *testing.T) {
	dir := t.TempDir()
	writeMigration(t, dir, "00003_ketiga.sql", "SELECT 3;")
	writeMigration(t, dir, "00001_pertama.sql", "SELECT 1;")
	writeMigration(t, dir, "00002_kedua.sql", "SELECT 2;")

	list, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() gagal: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("jumlah migrasi = %d, ingin 3", len(list))
	}
	for i, want := range []int64{1, 2, 3} {
		if list[i].Version != want {
			t.Errorf("migrasi ke-%d berversi %d, ingin %d", i, list[i].Version, want)
		}
	}
}

func TestLoadMenghitungChecksum(t *testing.T) {
	dir := t.TempDir()
	writeMigration(t, dir, "00001_a.sql", "CREATE TABLE a (id INTEGER);")

	list, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() gagal: %v", err)
	}
	if len(list[0].Checksum) != 64 {
		t.Errorf("checksum SHA-256 seharusnya 64 karakter heksadesimal, dapat %d", len(list[0].Checksum))
	}

	writeMigration(t, dir, "00001_a.sql", "CREATE TABLE a (id INTEGER, b TEXT);")
	changed, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() kedua gagal: %v", err)
	}
	if changed[0].Checksum == list[0].Checksum {
		t.Error("checksum tidak berubah setelah isi berkas migrasi berubah")
	}
}

func TestLoadMenolakVersiGanda(t *testing.T) {
	dir := t.TempDir()
	writeMigration(t, dir, "00001_a.sql", "SELECT 1;")
	writeMigration(t, dir, "00001_b.sql", "SELECT 2;")

	_, err := Load(dir)
	if err == nil {
		t.Fatal("dua migrasi dengan versi sama harus ditolak")
	}
	if !strings.Contains(err.Error(), "00001") {
		t.Errorf("galat tidak menyebut versi yang bertabrakan: %v", err)
	}
}

func TestLoadMengabaikanBerkasYangTidakSesuaiPola(t *testing.T) {
	dir := t.TempDir()
	writeMigration(t, dir, "00001_sah.sql", "SELECT 1;")
	writeMigration(t, dir, "catatan.txt", "bukan migrasi")
	writeMigration(t, dir, "1_pendek.sql", "SELECT 2;")
	writeMigration(t, dir, "00002_HURUF.sql", "SELECT 3;")

	list, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() gagal: %v", err)
	}
	if len(list) != 1 || list[0].Version != 1 {
		t.Fatalf("hanya 00001_sah.sql yang boleh dikenali, dapat %d berkas: %+v", len(list), list)
	}
}

func TestLoadMenolakDirektoriTidakAda(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "tidak-ada")); err == nil {
		t.Fatal("direktori migrasi yang tidak ada harus dilaporkan sebagai galat")
	}
}

func TestRunMenerapkanMigrasiPertamaKali(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	dir := t.TempDir()
	writeMigration(t, dir, "00001_buat.sql", `CREATE TABLE uji (id TEXT PRIMARY KEY, v TEXT NOT NULL);`)
	writeMigration(t, dir, "00002_tambah.sql", `INSERT INTO uji (id, v) VALUES ('x', 'y');`)

	status, err := Run(ctx, db, dir)
	if err != nil {
		t.Fatalf("Run() gagal: %v", err)
	}
	if status.Applied != 2 {
		t.Errorf("Applied = %d, ingin 2", status.Applied)
	}
	if status.CurrentVersion != 2 {
		t.Errorf("CurrentVersion = %d, ingin 2", status.CurrentVersion)
	}
	if status.Available != 2 {
		t.Errorf("Available = %d, ingin 2", status.Available)
	}
	if status.AppliedPending() {
		t.Error("AppliedPending() harus false setelah seluruh migrasi diterapkan")
	}

	var v string
	if err := db.QueryRowContext(ctx, `SELECT v FROM uji WHERE id = 'x'`).Scan(&v); err != nil {
		t.Fatalf("data hasil migrasi tidak ada: %v", err)
	}
	if v != "y" {
		t.Errorf("v = %q, ingin %q", v, "y")
	}
}

func TestRunTidakMengulangMigrasiYangSudahDiterapkan(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	dir := t.TempDir()
	writeMigration(t, dir, "00001_buat.sql", `CREATE TABLE uji (id INTEGER PRIMARY KEY);`)

	first, err := Run(ctx, db, dir)
	if err != nil {
		t.Fatalf("Run() pertama gagal: %v", err)
	}
	if first.Applied != 1 {
		t.Fatalf("Applied pada penerapan pertama = %d, ingin 1", first.Applied)
	}

	second, err := Run(ctx, db, dir)
	if err != nil {
		t.Fatalf("Run() kedua gagal: %v", err)
	}
	if second.Applied != 0 {
		t.Errorf("Applied pada penerapan kedua = %d, ingin 0 (tidak boleh mengulang)", second.Applied)
	}
	if second.CurrentVersion != 1 {
		t.Errorf("CurrentVersion pada penerapan kedua = %d, ingin 1", second.CurrentVersion)
	}
}

func TestRunMenerapkanMigrasiTambahanSaja(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	dir := t.TempDir()
	writeMigration(t, dir, "00001_buat.sql", `CREATE TABLE uji (id INTEGER PRIMARY KEY);`)

	if _, err := Run(ctx, db, dir); err != nil {
		t.Fatalf("Run() pertama gagal: %v", err)
	}

	writeMigration(t, dir, "00002_tambah_kolom.sql", `ALTER TABLE uji ADD COLUMN v TEXT;`)

	status, err := Run(ctx, db, dir)
	if err != nil {
		t.Fatalf("Run() kedua gagal: %v", err)
	}
	if status.Applied != 1 {
		t.Errorf("Applied = %d, ingin 1 (hanya migrasi baru)", status.Applied)
	}
	if status.CurrentVersion != 2 {
		t.Errorf("CurrentVersion = %d, ingin 2", status.CurrentVersion)
	}
}

func TestRunMenggagalkanSeluruhTransaksiBilaSatuPernyataanRusak(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	dir := t.TempDir()
	writeMigration(t, dir, "00001_setengah_rusak.sql", `
		CREATE TABLE pertama (id INTEGER PRIMARY KEY);
		CREATE TABLE pertama (id INTEGER PRIMARY KEY);
	`)

	if _, err := Run(ctx, db, dir); err == nil {
		t.Fatal("migrasi dengan pernyataan rusak harus gagal")
	}

	var applied int64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&applied); err != nil {
		t.Fatalf("pembacaan schema_migrations gagal: %v", err)
	}
	if applied != 0 {
		t.Errorf("versi tercatat %d padahal migrasi gagal; transaksi tidak di-rollback penuh", applied)
	}

	var tables int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='pertama'`).Scan(&tables); err != nil {
		t.Fatalf("pembacaan sqlite_master gagal: %v", err)
	}
	if tables != 0 {
		t.Error("tabel dari migrasi gagal masih ada; transaksi tidak di-rollback penuh")
	}
}

func TestRunMenolakBerkasMigrasiYangBerubahSetelahDiterapkan(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	dir := t.TempDir()
	writeMigration(t, dir, "00001_buat.sql", `CREATE TABLE uji (id INTEGER PRIMARY KEY);`)

	if _, err := Run(ctx, db, dir); err != nil {
		t.Fatalf("Run() pertama gagal: %v", err)
	}

	writeMigration(t, dir, "00001_buat.sql", `CREATE TABLE uji (id INTEGER PRIMARY KEY, v TEXT);`)

	_, err := Run(ctx, db, dir)
	if err == nil {
		t.Fatal("berkas migrasi yang berubah setelah diterapkan harus ditolak")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "checksum") {
		t.Errorf("galat tidak menyebut checksum: %v", err)
	}
}

func TestRunPadaDirektoriKosongTidakMengubahApaPun(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	dir := t.TempDir()

	status, err := Run(ctx, db, dir)
	if err != nil {
		t.Fatalf("Run() pada direktori kosong gagal: %v", err)
	}
	if status.Applied != 0 || status.Available != 0 {
		t.Errorf("status = %+v, ingin nol", status)
	}
	if status.AppliedPending() {
		t.Error("AppliedPending() pada direktori kosong harus false")
	}
}

func TestRunMembuatTabelPelacakMigrasi(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	dir := t.TempDir()
	writeMigration(t, dir, "00001_buat.sql", `CREATE TABLE uji (id INTEGER PRIMARY KEY);`)

	if _, err := Run(ctx, db, dir); err != nil {
		t.Fatalf("Run() gagal: %v", err)
	}

	for _, name := range []string{"schema_migrations", "migration_lock"} {
		var found int
		if err := db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&found); err != nil {
			t.Fatalf("pembacaan sqlite_master untuk %s gagal: %v", name, err)
		}
		if found != 1 {
			t.Errorf("tabel %s tidak ditemukan setelah Run()", name)
		}
	}
}

func TestRunMencatatVersiDanChecksum(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	dir := t.TempDir()
	writeMigration(t, dir, "00001_buat.sql", `CREATE TABLE uji (id INTEGER PRIMARY KEY);`)

	if _, err := Run(ctx, db, dir); err != nil {
		t.Fatalf("Run() gagal: %v", err)
	}

	var version int64
	var checksum string
	var appliedAt string
	if err := db.QueryRowContext(ctx,
		`SELECT version, checksum, applied_at FROM schema_migrations WHERE version = 1`).
		Scan(&version, &checksum, &appliedAt); err != nil {
		t.Fatalf("pembacaan catatan migrasi gagal: %v", err)
	}
	if version != 1 {
		t.Errorf("version = %d, ingin 1", version)
	}
	if len(checksum) != 64 {
		t.Errorf("checksum tercatat sepanjang %d, ingin 64", len(checksum))
	}
	if !strings.HasSuffix(appliedAt, "Z") {
		t.Errorf("applied_at = %q, ingin waktu UTC berakhiran Z", appliedAt)
	}
}

func TestRunDuaKaliBerurutanMenghasilkanStatusIdempoten(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	dir := t.TempDir()
	writeMigration(t, dir, "00001_a.sql", `CREATE TABLE a (id INTEGER PRIMARY KEY);`)
	writeMigration(t, dir, "00002_b.sql", `CREATE TABLE b (id INTEGER PRIMARY KEY);`)

	if _, err := Run(ctx, db, dir); err != nil {
		t.Fatalf("Run() pertama gagal: %v", err)
	}

	var rowsAfterFirst int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&rowsAfterFirst); err != nil {
		t.Fatalf("pembacaan jumlah catatan gagal: %v", err)
	}

	if _, err := Run(ctx, db, dir); err != nil {
		t.Fatalf("Run() kedua gagal: %v", err)
	}

	var rowsAfterSecond int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&rowsAfterSecond); err != nil {
		t.Fatalf("pembacaan jumlah catatan gagal: %v", err)
	}
	if rowsAfterFirst != rowsAfterSecond {
		t.Errorf("jumlah catatan migrasi bertambah dari %d menjadi %d pada penerapan kedua",
			rowsAfterFirst, rowsAfterSecond)
	}
}

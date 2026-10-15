package database

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func openTemp(t *testing.T) (*DB, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "facelink.db")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open(%q) gagal: %v", path, err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("Close() gagal: %v", err)
		}
	})
	return db, path
}

func TestOpenMembuatBerkasDanDirektori(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "nested", "dalam", "facelink.db")

	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open() harus membuat direktori induk: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("berkas basis data tidak ada setelah Open: %v", err)
	}
	if db.Path() != path {
		t.Errorf("Path() = %q, ingin %q", db.Path(), path)
	}
	if db.WALPath() != path+"-wal" {
		t.Errorf("WALPath() = %q", db.WALPath())
	}
	if db.SHMPath() != path+"-shm" {
		t.Errorf("SHMPath() = %q", db.SHMPath())
	}
}

func TestOpenMengaktifkanWAL(t *testing.T) {
	db, _ := openTemp(t)

	mode, err := db.JournalMode(context.Background())
	if err != nil {
		t.Fatalf("JournalMode() gagal: %v", err)
	}
	if mode != JournalModeWAL {
		t.Fatalf("journal_mode = %q, ingin %q (docs/DATA_MODEL.md §6.1)", mode, JournalModeWAL)
	}

	var pragma string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&pragma); err != nil {
		t.Fatalf("pembacaan PRAGMA journal_mode gagal: %v", err)
	}
	if !strings.EqualFold(pragma, JournalModeWAL) {
		t.Errorf("PRAGMA journal_mode = %q, ingin %q", pragma, JournalModeWAL)
	}
}

func TestOpenMengaktifkanForeignKey(t *testing.T) {
	db, _ := openTemp(t)

	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("pembacaan PRAGMA foreign_keys gagal: %v", err)
	}
	if fk != 1 {
		t.Fatalf("foreign_keys = %d, ingin 1 (docs/DATA_MODEL.md §6.1)", fk)
	}
}

func TestForeignKeyBenarBenarMenolakPelanggaran(t *testing.T) {
	db, _ := openTemp(t)
	ctx := context.Background()

	if _, err := db.ExecContext(ctx, `CREATE TABLE induk (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatalf("pembuatan tabel induk gagal: %v", err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE anak (
		id TEXT PRIMARY KEY,
		induk_id TEXT NOT NULL REFERENCES induk(id) ON DELETE RESTRICT
	)`); err != nil {
		t.Fatalf("pembuatan tabel anak gagal: %v", err)
	}

	if _, err := db.ExecContext(ctx, `INSERT INTO anak (id, induk_id) VALUES ('a1', 'tidak-ada')`); err == nil {
		t.Fatal("penyisipan dengan kunci asing tidak ada harus ditolak; foreign_keys tampaknya tidak aktif")
	}
}

func TestOpenMenetapkanBusyTimeout(t *testing.T) {
	db, _ := openTemp(t)

	var busy int
	if err := db.QueryRow("PRAGMA busy_timeout").Scan(&busy); err != nil {
		t.Fatalf("pembacaan PRAGMA busy_timeout gagal: %v", err)
	}
	if busy != BusyTimeoutMillis {
		t.Errorf("busy_timeout = %d, ingin %d", busy, BusyTimeoutMillis)
	}
}

func TestOpenMenetapkanSynchronousNormal(t *testing.T) {
	db, _ := openTemp(t)

	var sync int
	if err := db.QueryRow("PRAGMA synchronous").Scan(&sync); err != nil {
		t.Fatalf("pembacaan PRAGMA synchronous gagal: %v", err)
	}
	// Nilai numerik NORMAL adalah 1 pada SQLite.
	if sync != 1 {
		t.Errorf("synchronous = %d, ingin 1 (NORMAL)", sync)
	}
}

func TestOpenMenolakJalurKosong(t *testing.T) {
	if _, err := Open(context.Background(), "   "); err == nil {
		t.Fatal("jalur kosong harus ditolak")
	}
}

func TestOpenMembukaUlangBasisDataYangSudahAda(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facelink.db")

	first, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open pertama gagal: %v", err)
	}
	if _, err := first.Exec(`CREATE TABLE penanda (id INTEGER PRIMARY KEY, nilai TEXT NOT NULL)`); err != nil {
		t.Fatalf("pembuatan tabel penanda gagal: %v", err)
	}
	if _, err := first.Exec(`INSERT INTO penanda (nilai) VALUES ('utuh')`); err != nil {
		t.Fatalf("penyisipan gagal: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close pertama gagal: %v", err)
	}

	second, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open kedua gagal: %v", err)
	}
	defer second.Close()

	var nilai string
	if err := second.QueryRow(`SELECT nilai FROM penanda WHERE id = 1`).Scan(&nilai); err != nil {
		t.Fatalf("data hilang setelah dibuka ulang: %v", err)
	}
	if nilai != "utuh" {
		t.Errorf("nilai = %q, ingin %q", nilai, "utuh")
	}
}

func TestPingBerhasilPadaBasisDataHidup(t *testing.T) {
	db, _ := openTemp(t)
	if err := db.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() pada basis data hidup gagal: %v", err)
	}
}

func TestPingGagalSetelahBasisDataDitutup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facelink.db")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open() gagal: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() gagal: %v", err)
	}

	if err := db.Ping(context.Background()); err == nil {
		t.Fatal("Ping() pada basis data tertutup seharusnya melaporkan galat")
	}
}

func TestCheckpointMengosongkanWAL(t *testing.T) {
	db, _ := openTemp(t)
	ctx := context.Background()

	if _, err := db.ExecContext(ctx, `CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)`); err != nil {
		t.Fatalf("pembuatan tabel gagal: %v", err)
	}
	for i := 0; i < 20; i++ {
		if _, err := db.ExecContext(ctx, `INSERT INTO t (v) VALUES (?)`, "baris"); err != nil {
			t.Fatalf("penyisipan ke-%d gagal: %v", i, err)
		}
	}

	if err := db.Checkpoint(ctx); err != nil {
		t.Fatalf("Checkpoint() gagal: %v", err)
	}

	if info, err := os.Stat(db.WALPath()); err == nil && info.Size() > 0 {
		t.Errorf("berkas WAL masih berukuran %d byte setelah checkpoint TRUNCATE", info.Size())
	}
}

func TestIntegrityCheckMelaporkanBasisDataSehat(t *testing.T) {
	db, _ := openTemp(t)

	if err := db.IntegrityCheck(context.Background()); err != nil {
		t.Errorf("IntegrityCheck() pada basis data sehat melaporkan galat: %v", err)
	}
}

func TestCloseMenutupKoneksi(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facelink.db")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open() gagal: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() gagal: %v", err)
	}

	var one int
	if err := db.QueryRow("SELECT 1").Scan(&one); err == nil {
		t.Error("kueri setelah Close seharusnya gagal; koneksi tampaknya masih hidup")
	}

	// Close yang dipanggil dua kali tidak boleh panik.
	if err := db.Close(); err != nil {
		t.Errorf("Close() kedua mengembalikan galat: %v", err)
	}
}

func TestCopyFileMenyalinBasisDataDenganUtuh(t *testing.T) {
	db, path := openTemp(t)
	ctx := context.Background()

	if _, err := db.ExecContext(ctx, `CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT NOT NULL)`); err != nil {
		t.Fatalf("pembuatan tabel gagal: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO t (v) VALUES ('salin')`); err != nil {
		t.Fatalf("penyisipan gagal: %v", err)
	}
	if err := db.Checkpoint(ctx); err != nil {
		t.Fatalf("Checkpoint() gagal: %v", err)
	}

	dst := filepath.Join(t.TempDir(), "salinan.db")
	if err := CopyFile(dst, path); err != nil {
		t.Fatalf("CopyFile() gagal: %v", err)
	}

	copied, err := sql.Open("sqlite", "file:"+filepath.ToSlash(dst)+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("membuka salinan gagal: %v", err)
	}
	defer copied.Close()

	var v string
	if err := copied.QueryRow(`SELECT v FROM t WHERE id = 1`).Scan(&v); err != nil {
		t.Fatalf("pembacaan salinan gagal: %v", err)
	}
	if v != "salin" {
		t.Errorf("nilai salinan = %q, ingin %q", v, "salin")
	}
}

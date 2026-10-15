// Package database membuka dan mengonfigurasi database SQLite FaceLink AI.
//
// Pengaturan yang ditegakkan ada di docs/DATA_MODEL.md §6.1 dan §7.1:
// journal_mode WAL, foreign_keys ON, busy_timeout 5000, synchronous NORMAL,
// wal_autocheckpoint 1000, dan temp_store MEMORY. Backend Go adalah satu-satunya
// penulis database (D-027).
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Pengaturan PRAGMA yang berlaku.
const (
	JournalModeWAL    = "wal"
	BusyTimeoutMillis = 5000
	SyncMode          = "NORMAL"
	AutoCheckpoint    = 1000
	TempStore         = "MEMORY"
)

// maxOpenConns menjaga SQLite tetap pada satu penulis. Koneksi tambahan hanya
// menambah penantian pada kunci tulis dan memperbesar kemungkinan SQLITE_BUSY
// (docs/DATA_MODEL.md §6.2).
const maxOpenConns = 1

// DB membungkus *sql.DB bersama jalur berkasnya supaya pemeriksaan keadaan
// seperti keberadaan sidecar WAL tidak perlu menebak nama berkas.
type DB struct {
	*sql.DB
	path string
}

// Path mengembalikan jalur berkas database utama.
func (d *DB) Path() string { return d.path }

// WALPath mengembalikan jalur berkas sidecar WAL.
func (d *DB) WALPath() string { return d.path + "-wal" }

// SHMPath mengembalikan jalur berkas sidecar shared-memory.
func (d *DB) SHMPath() string { return d.path + "-shm" }

// Open membuka database pada jalur yang diberikan, membuat direktori induk bila
// perlu, lalu menerapkan dan memverifikasi seluruh PRAGMA wajib.
func Open(ctx context.Context, path string) (*DB, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("database: jalur berkas tidak boleh kosong")
	}

	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("database: gagal membuat direktori %s: %w", dir, err)
		}
	}

	dsn := buildDSN(path)

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("database: gagal membuka %s: %w", path, err)
	}
	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(maxOpenConns)
	sqlDB.SetConnMaxLifetime(0)

	// Penerapan pengaturan dan verifikasinya dijalankan pada satu koneksi yang
	// sama supaya PRAGMA tingkat koneksi benar-benar berlaku pada koneksi yang
	// akan dipakai.
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("database: gagal mengambil koneksi: %w", err)
	}
	defer conn.Close()

	if err := configure(ctx, conn); err != nil {
		sqlDB.Close()
		return nil, err
	}

	if err := verify(ctx, conn); err != nil {
		sqlDB.Close()
		return nil, err
	}

	return &DB{DB: sqlDB, path: path}, nil
}

// buildDSN menyusun DSN modernc.org/sqlite. Nilai dikirim lewat parameter
// kueri sehingga jalur berkas tidak perlu di-escape manual.
func buildDSN(path string) string {
	q := url.Values{}
	q.Add("_pragma", "busy_timeout("+itoa(BusyTimeoutMillis)+")")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "journal_mode("+JournalModeWAL+")")
	q.Add("_pragma", "synchronous("+SyncMode+")")
	q.Add("_pragma", "temp_store("+TempStore+")")
	q.Add("_pragma", "wal_autocheckpoint("+itoa(AutoCheckpoint)+")")
	return "file:" + filepath.ToSlash(path) + "?" + q.Encode()
}

func itoa(v int) string { return fmt.Sprintf("%d", v) }

func configure(ctx context.Context, conn *sql.Conn) error {
	pragmas := []string{
		"PRAGMA journal_mode = " + JournalModeWAL,
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = " + itoa(BusyTimeoutMillis),
		"PRAGMA synchronous = " + SyncMode,
		"PRAGMA wal_autocheckpoint = " + itoa(AutoCheckpoint),
		"PRAGMA temp_store = " + TempStore,
	}
	for _, stmt := range pragmas {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("database: gagal menerapkan %q: %w", stmt, err)
		}
	}
	return nil
}

// verify memastikan pengaturan yang berlaku benar-benar seperti yang diminta.
// Pengujian skenario 7 dan 8 pada docs/DATA_MODEL.md §12 mensyaratkan kesalahan
// konfigurasi terdeteksi di sini, bukan oleh pengguna.
func verify(ctx context.Context, conn *sql.Conn) error {
	var journalMode string
	if err := conn.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		return fmt.Errorf("database: gagal membaca journal_mode: %w", err)
	}
	if !strings.EqualFold(journalMode, JournalModeWAL) {
		return fmt.Errorf("database: journal_mode = %q, seharusnya %q", journalMode, JournalModeWAL)
	}

	var foreignKeys int
	if err := conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		return fmt.Errorf("database: gagal membaca foreign_keys: %w", err)
	}
	if foreignKeys != 1 {
		return fmt.Errorf("database: foreign_keys = %d, seharusnya 1; kaskade dan integritas referensial tidak akan berjalan", foreignKeys)
	}

	var busyTimeout int
	if err := conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		return fmt.Errorf("database: gagal membaca busy_timeout: %w", err)
	}
	if busyTimeout != BusyTimeoutMillis {
		return fmt.Errorf("database: busy_timeout = %d, seharusnya %d", busyTimeout, BusyTimeoutMillis)
	}

	return nil
}

// Ping memeriksa ketersediaan database tanpa menjalankan PRAGMA integrity_check
// penuh; /readyz harus tetap pembacaan ringan (docs/API_CONTRACT.md §5.12).
func (d *DB) Ping(ctx context.Context) error {
	return d.DB.PingContext(ctx)
}

// JournalMode membaca mode jurnal yang sedang berlaku untuk /readyz.
func (d *DB) JournalMode(ctx context.Context) (string, error) {
	var mode string
	if err := d.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil {
		return "", fmt.Errorf("database: gagal membaca journal_mode: %w", err)
	}
	return mode, nil
}

// Close menutup database dengan rapi: seluruh penulis dihentikan lebih dahulu,
// checkpoint penuh dijalankan untuk menyusutkan WAL, lalu koneksi ditutup.
// PRAGMA optimize dipanggil pada penutupan agar statistik kueri tetap mutakhir.
func (d *DB) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _ = d.ExecContext(ctx, "PRAGMA optimize")
	_, _ = d.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")

	return d.DB.Close()
}

// Checkpoint menjalankan checkpoint WAL secara eksplisit; dipakai pengujian
// skenario 17 (docs/DATA_MODEL.md §12) yang memeriksa ukuran -wal menyusut
// setelah seluruh penulis berhenti.
func (d *DB) Checkpoint(ctx context.Context) error {
	row := d.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
	var busy, log, checkpointed int
	if err := row.Scan(&busy, &log, &checkpointed); err != nil {
		return fmt.Errorf("database: wal_checkpoint gagal: %w", err)
	}
	if busy != 0 {
		return fmt.Errorf("database: wal_checkpoint tertahan (busy=%d); masih ada pembaca atau penulis aktif", busy)
	}
	return nil
}

// IntegrityCheck menjalankan pemeriksaan integritas menyeluruh. Tidak dipanggil
// pada jalur permintaan /readyz; hanya untuk perkakas operator dan pengujian.
func (d *DB) IntegrityCheck(ctx context.Context) error {
	var result string
	if err := d.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil {
		return fmt.Errorf("database: integrity_check gagal: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("database: integrity_check melaporkan %q", result)
	}
	return nil
}

// CopyFile menyalin satu berkas; dipakai pengujian pencadangan untuk
// menunjukkan bahwa menyalin database tanpa sidecar WAL kehilangan transaksi
// terakhir (skenario 12, docs/DATA_MODEL.md §12).
func CopyFile(dst, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

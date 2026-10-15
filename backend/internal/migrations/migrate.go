// Package migrations menjalankan migrasi skema SQL berversi dari sisi Go.
//
// Mekanisme ini adalah D-026 (docs/DATA_MODEL.md §9). Aturan yang ditegakkan:
// penerapan hanya dari satu proses yang dilindungi kunci, penerapan kedua tidak
// mengubah apa pun, dan kegagalan di tengah tidak menaikkan versi skema.
package migrations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
)

const (
	// advisoryLockKey adalah kunci tunggu bersama bagi seluruh proses yang
	// mencoba bermigrasi. Nilainya sembarang tetapi harus stabil antarproses.
	advisoryLockKey = 0x464C3030 // "FL00"

	// advisoryLockTimeoutMillis membatasi penantian kunci. Proses kedua harus
	// menunggu, bukan menjalankan migrasi bersamaan (§9.4 butir 2).
	advisoryLockTimeoutMillis = 10000
)

// filePattern menerima NNNNN_deskripsi.sql dengan lima digit berurutan (§9.3).
var filePattern = regexp.MustCompile(`^(\d{5})_([a-z0-9_]+)\.sql$`)

// Migration adalah satu berkas migrasi yang sudah dibaca dari disk.
type Migration struct {
	Version  int64
	Name     string
	Path     string
	Checksum string
	SQL      string
}

// Status melaporkan hasil penerapan migrasi untuk dipakai /readyz.
type Status struct {
	// CurrentVersion adalah versi skema yang berlaku setelah penerapan.
	CurrentVersion int64
	// Applied adalah jumlah migrasi yang benar-benar dijalankan pada pemanggilan
	// terakhir; nol berarti skema sudah mutakhir sebelumnya.
	Applied int
	// Available adalah jumlah berkas migrasi yang ditemukan.
	Available int

	latest int64
}

// AppliedPending melaporkan apakah masih ada migrasi yang belum diterapkan,
// yaitu keadaan yang membuat /readyz menjawab 503.
func (s Status) AppliedPending() bool {
	return s.latest > s.CurrentVersion
}

// ForTesting membangun Status dengan versi terbaru yang ditentukan, agar
// pengujian kesiapan dapat menyusun keadaan "migrasi tertunda" tanpa perlu
// menjalankan migrasi sungguhan.
func ForTesting(currentVersion, latest, available int64) Status {
	return Status{
		CurrentVersion: currentVersion,
		Applied:        0,
		Available:      int(available),
		latest:         latest,
	}
}

// runnerTables adalah tabel internal migrator. Tidak diekspos ke API klien dan
// bukan tabel domain (§9.4 butir 1).
const createTablesStmt = `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version     INTEGER PRIMARY KEY,
  name        TEXT    NOT NULL,
  checksum    TEXT    NOT NULL,
  applied_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE IF NOT EXISTS migration_lock (
  id          INTEGER PRIMARY KEY CHECK (id = 1),
  locked_at   TEXT,
  locked_by   TEXT
);
`

var errLockBusy = errors.New("migrasi: kunci migrasi sedang dipegang proses lain")

// Load membaca dan memvalidasi seluruh berkas migrasi pada direktori yang
// diberikan, terurut menaik menurut versi.
func Load(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("migrasi: gagal membaca direktori %s: %w", dir, err)
	}

	var list []Migration
	seen := make(map[int64]string)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		match := filePattern.FindStringSubmatch(entry.Name())
		if match == nil {
			// Berkas .sql yang tidak mengikuti pola diabaikan, bukan digagalkan:
			// catatan dan berkas bantu boleh berada di direktori yang sama.
			continue
		}

		version, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("migrasi: nomor versi pada %q tidak sah: %w", entry.Name(), err)
		}
		if prev, dup := seen[version]; dup {
			return nil, fmt.Errorf("migrasi: versi %d dipakai dua kali oleh %q dan %q; nomor tidak boleh dipakai ulang", version, prev, entry.Name())
		}
		seen[version] = entry.Name()

		path := filepath.Join(dir, entry.Name())
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("migrasi: gagal membaca %s: %w", path, err)
		}

		sum := sha256.Sum256(body)
		list = append(list, Migration{
			Version:  version,
			Name:     match[2],
			Path:     path,
			Checksum: hex.EncodeToString(sum[:]),
			SQL:      string(body),
		})
	}

	if len(list) == 0 {
		return nil, nil
	}

	sort.Slice(list, func(i, j int) bool { return list[i].Version < list[j].Version })
	return list, nil
}

// Run menerapkan seluruh migrasi yang belum dijalankan. Pemanggilan kedua tidak
// mengubah apa pun dan tidak gagal (§9.3, §12 skenario 13).
func Run(ctx context.Context, db *database.DB, dir string) (Status, error) {
	list, err := Load(dir)
	if err != nil {
		return Status{}, err
	}

	if len(list) == 0 {
		return Status{}, nil
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return Status{}, fmt.Errorf("migrasi: gagal mengambil koneksi: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, createTablesStmt); err != nil {
		return Status{}, fmt.Errorf("migrasi: gagal menyiapkan tabel migrator: %w", err)
	}

	acquired, err := acquireLock(ctx, conn)
	if err != nil {
		return Status{}, err
	}
	if !acquired {
		return Status{}, errLockBusy
	}
	defer releaseLock(ctx, conn)

	applied, err := appliedVersions(ctx, conn)
	if err != nil {
		return Status{}, err
	}

	current, err := currentVersion(ctx, conn)
	if err != nil {
		return Status{}, err
	}

	status := Status{CurrentVersion: current, Available: len(list), latest: list[len(list)-1].Version}

	for _, m := range list {
		if recorded, ok := applied[m.Version]; ok {
			// Berkas migrasi yang sudah diterapkan tidak boleh berubah isi:
			// perubahan diam-diam pada migrasi lama menghasilkan skema yang
			// berbeda antar mesin dengan nomor versi yang sama.
			if recorded != m.Checksum {
				return status, fmt.Errorf("migrasi: checksum %s (versi %d) berubah dari %s menjadi %s; buat migrasi baru alih-alih mengubah yang sudah diterapkan",
					m.Path, m.Version, recorded, m.Checksum)
			}
			continue
		}

		if err := applyOne(ctx, conn, m); err != nil {
			return status, err
		}
		status.Applied++
		status.CurrentVersion = m.Version
	}

	return status, nil
}

// applyOne menjalankan satu migrasi dalam satu transaksi bersama pencatatan
// versinya. Bila salah satu pernyataan gagal, seluruh transaksi diurungkan dan
// versi skema tidak naik (§12 skenario 14).
func applyOne(ctx context.Context, conn *sql.Conn, m Migration) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("migrasi: gagal membuka transaksi untuk versi %d: %w", m.Version, err)
	}
	defer tx.Rollback()

	body := strings.TrimSpace(m.SQL)
	if body == "" {
		return fmt.Errorf("migrasi: berkas %s kosong", m.Path)
	}

	if _, err := tx.ExecContext(ctx, body); err != nil {
		return fmt.Errorf("migrasi: versi %d (%s) gagal: %w", m.Version, m.Name, err)
	}

	if _, err := tx.ExecContext(ctx,
		"INSERT INTO schema_migrations (version, name, checksum) VALUES (?, ?, ?)",
		m.Version, m.Name, m.Checksum,
	); err != nil {
		return fmt.Errorf("migrasi: gagal mencatat versi %d: %w", m.Version, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migrasi: gagal menyimpan versi %d: %w", m.Version, err)
	}
	return nil
}

func appliedVersions(ctx context.Context, conn *sql.Conn) (map[int64]string, error) {
	rows, err := conn.QueryContext(ctx, "SELECT version, checksum FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("migrasi: gagal membaca riwayat: %w", err)
	}
	defer rows.Close()

	applied := make(map[int64]string)
	for rows.Next() {
		var version int64
		var checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			return nil, fmt.Errorf("migrasi: gagal membaca baris riwayat: %w", err)
		}
		applied[version] = checksum
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("migrasi: gagal menelusuri riwayat: %w", err)
	}
	return applied, nil
}

func currentVersion(ctx context.Context, conn *sql.Conn) (int64, error) {
	var version sql.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT MAX(version) FROM schema_migrations").Scan(&version); err != nil {
		return 0, fmt.Errorf("migrasi: gagal membaca versi berlaku: %w", err)
	}
	if !version.Valid {
		return 0, nil
	}
	return version.Int64, nil
}

// acquireLock memakai kunci berbasis baris pada tabel migration_lock. SQLite
// hanya mengizinkan satu penulis, sehingga transaksi IMMEDIATE yang singkat
// sudah cukup untuk menjamin satu penerap saja.
func acquireLock(ctx context.Context, conn *sql.Conn) (bool, error) {
	deadline := fmt.Sprintf("-%d milliseconds", advisoryLockTimeoutMillis)

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		// SQLITE_BUSY saat memulai transaksi berarti proses lain sedang
		// memegang kunci; tunggu sampai busy_timeout, lalu laporkan sibuk.
		if isBusy(err) {
			return false, nil
		}
		return false, fmt.Errorf("migrasi: gagal memulai transaksi kunci: %w", err)
	}

	var lockedAt sql.NullString
	err := conn.QueryRowContext(ctx, "SELECT locked_at FROM migration_lock WHERE id = 1").Scan(&lockedAt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		_, _ = conn.ExecContext(ctx, "ROLLBACK")
		return false, fmt.Errorf("migrasi: gagal membaca kunci: %w", err)
	}

	if lockedAt.Valid {
		// Kunci dipegang proses lain dan belum dilepas.
		var stale bool
		if err := conn.QueryRowContext(ctx,
			"SELECT CASE WHEN locked_at <= datetime('now', ?) THEN 1 ELSE 0 END FROM migration_lock WHERE id = 1",
			deadline,
		).Scan(&stale); err != nil {
			_, _ = conn.ExecContext(ctx, "ROLLBACK")
			return false, fmt.Errorf("migrasi: gagal memeriksa umur kunci: %w", err)
		}
		if !stale {
			_, _ = conn.ExecContext(ctx, "ROLLBACK")
			return false, nil
		}
	}

	if _, err := conn.ExecContext(ctx,
		"INSERT INTO migration_lock (id, locked_at, locked_by) VALUES (1, strftime('%Y-%m-%dT%H:%M:%fZ','now'), ?) "+
			"ON CONFLICT(id) DO UPDATE SET locked_at = strftime('%Y-%m-%dT%H:%M:%fZ','now'), locked_by = excluded.locked_by",
		advisoryLockKey,
	); err != nil {
		_, _ = conn.ExecContext(ctx, "ROLLBACK")
		return false, fmt.Errorf("migrasi: gagal mengambil kunci: %w", err)
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return false, fmt.Errorf("migrasi: gagal menyimpan kunci: %w", err)
	}
	return true, nil
}

func releaseLock(ctx context.Context, conn *sql.Conn) {
	_, _ = conn.ExecContext(ctx,
		"UPDATE migration_lock SET locked_at = NULL, locked_by = NULL WHERE id = 1")
}

func isBusy(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "sqlite_busy") ||
		strings.Contains(msg, "database table is locked")
}

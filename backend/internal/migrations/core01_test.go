package migrations

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCORE01UpgradePreservesLedgerAndLegacyIdentity(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		t.Run(map[bool]string{false: "upgrade", true: "duplicate_rolls_back"}[duplicate], func(t *testing.T) {
			ctx := context.Background()
			full, _ := schemaPath(t)
			old := t.TempDir()
			for _, name := range []string{"00001_init_core_schema.sql", "00002_auth_identity.sql", "00003_auth_session_history.sql", "00004_email_otp.sql"} {
				data, err := os.ReadFile(filepath.Join(full, name))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(old, name), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			db := openDB(t)
			if _, err := Run(ctx, db, old); err != nil {
				t.Fatal(err)
			}
			var before string
			const ledger = `SELECT group_concat(version || ':' || checksum, '|') FROM (SELECT version, checksum FROM schema_migrations ORDER BY version)`
			if err := db.QueryRowContext(ctx, ledger).Scan(&before); err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(ctx, `INSERT INTO people(id,external_code,display_name,deleted_at) VALUES('legacy','LEGACY-001','Peserta Sintetis',NULL)`); err != nil {
				t.Fatal(err)
			}
			if duplicate {
				if _, err := db.ExecContext(ctx, `INSERT INTO people(id,external_code,display_name,deleted_at) VALUES('archived','LEGACY-001','Peserta Arsip Sintetis','2026-01-01T00:00:00.000Z')`); err != nil {
					t.Fatal(err)
				}
			}
			_, err := Run(ctx, db, full)
			if duplicate {
				if err == nil {
					t.Fatal("duplicate archived NIM must reject migration")
				}
				var after string
				if err := db.QueryRowContext(ctx, ledger).Scan(&after); err != nil {
					t.Fatal(err)
				}
				if after != before {
					t.Fatalf("failed migration changed ledger: %q -> %q", before, after)
				}
				var columns int
				if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('people') WHERE name='identity_reference'`).Scan(&columns); err != nil {
					t.Fatal(err)
				}
				if columns != 0 {
					t.Fatal("failed migration leaked partial schema")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var existingLedger string
			if err := db.QueryRowContext(ctx, `SELECT group_concat(version || ':' || checksum, '|') FROM (SELECT version, checksum FROM schema_migrations WHERE version < 5 ORDER BY version)`).Scan(&existingLedger); err != nil {
				t.Fatal(err)
			}
			if existingLedger != before {
				t.Fatalf("existing checksums changed: %q -> %q", before, existingLedger)
			}
			var unverified int
			if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM people WHERE id='legacy' AND external_code='LEGACY-001' AND identity_reference IS NULL AND identity_verified_at IS NULL`).Scan(&unverified); err != nil {
				t.Fatal(err)
			}
			if unverified != 1 {
				t.Fatal("legacy person lost or incorrectly attested")
			}
			status, err := Run(ctx, db, full)
			if err != nil {
				t.Fatal(err)
			}
			if status.Applied != 0 {
				t.Fatalf("repeat migration wrote %d versions", status.Applied)
			}
		})
	}
}

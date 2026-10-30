package migrations

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
)

func core02MigrationSnapshot(t *testing.T, db *database.DB, query string) string {
	t.Helper()
	var result string
	if err := db.QueryRowContext(context.Background(), query).Scan(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestCORE02UpgradePreservesLedgerLegacyAttendanceAndForeignKeys(t *testing.T) {
	ctx := context.Background()
	source, _ := schemaPath(t)
	full, old := t.TempDir(), t.TempDir()
	list, err := Load(source)
	if err != nil {
		t.Fatal(err)
	}
	// This regression targets the 5→6 transition, independent of later migrations.
	for _, migration := range list {
		if migration.Version > 6 {
			continue
		}
		body, err := os.ReadFile(migration.Path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(full, filepath.Base(migration.Path)), body, 0600); err != nil {
			t.Fatal(err)
		}
		if migration.Version <= 5 {
			if err := os.WriteFile(filepath.Join(old, filepath.Base(migration.Path)), body, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	db := openDB(t)
	if status, err := Run(ctx, db, old); err != nil || status.CurrentVersion != 5 {
		t.Fatalf("legacy schema=%+v %v", status, err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO users(id,email,display_name,role,status) VALUES('legacy-admin','legacy@example.invalid','Admin Sintetis','admin','active');
		INSERT INTO people(id,external_code,display_name) VALUES('legacy-person','LEGACY-CORE02','Peserta Sintetis');
		INSERT INTO classes(id,code,title) VALUES('legacy-open-class','LEGACY-OPEN','Kelas lama terbuka'),('legacy-closed-class','LEGACY-CLOSED','Kelas lama tutup'),('legacy-cancel-class','LEGACY-CANCEL','Kelas lama batal');
		INSERT INTO devices(id,device_code) VALUES('legacy-device','LEGACY-DEVICE');
		INSERT INTO attendance_sessions(id,class_id,device_id,status,opens_at,closes_at,late_after,note,created_by,created_at,updated_at,closed_at,closed_by) VALUES
		('legacy-open','legacy-open-class','legacy-device','open','2026-01-01T01:00:00.000Z','2026-01-01T02:00:00.000Z','2026-01-01T01:10:00.000Z','Catatan lama open','legacy-admin','2026-01-01T00:59:00.000Z','2026-01-01T01:00:00.000Z',NULL,NULL),
		('legacy-closed','legacy-closed-class',NULL,'closed','2026-01-02T01:00:00.000Z','2026-01-02T02:00:00.000Z',NULL,'Catatan lama closed','legacy-admin','2026-01-02T00:59:00.000Z','2026-01-02T02:00:00.000Z','2026-01-02T02:00:00.000Z','legacy-admin'),
		('legacy-cancel','legacy-cancel-class',NULL,'cancelled',NULL,'2026-01-03T02:00:00.000Z',NULL,'Catatan lama cancelled','legacy-admin','2026-01-03T00:59:00.000Z','2026-01-03T01:00:00.000Z',NULL,NULL);
		INSERT INTO attendance_records(id,session_id,person_id,status,source,confidence,checked_in_at,device_id,created_by,reason,created_at,updated_at) VALUES
		('legacy-record-open','legacy-open','legacy-person','present','import',NULL,'2026-01-01T01:05:00.000Z','legacy-device',NULL,'Import sintetis lama','2026-01-01T01:05:00.000Z','2026-01-01T01:05:00.000Z'),
		('legacy-record-closed','legacy-closed','legacy-person','excused','manual',NULL,'2026-01-02T01:05:00.000Z',NULL,'legacy-admin','Alasan manual lama','2026-01-02T01:05:00.000Z','2026-01-02T01:05:00.000Z');
		INSERT INTO audit_events(id,actor_type,actor_id,action,entity_type,entity_id,result,reason) VALUES('legacy-audit','user','legacy-admin','attendance.manual_corrected','attendance_record','legacy-record-closed','success','Alasan lama');
		INSERT INTO idempotency_records(scope,key,request_hash,response_code,response_body,expires_at) VALUES('operator:legacy-admin','legacy-key','synthetic-hash',201,'{}','2026-01-02T00:00:00.000Z');
	`); err != nil {
		t.Fatal(err)
	}
	const sessions = `SELECT json_group_array(json_array(id,class_id,device_id,status,opens_at,closes_at,late_after,note,created_by,created_at,updated_at,closed_at,closed_by)) FROM (SELECT * FROM attendance_sessions ORDER BY id)`
	const records = `SELECT json_group_array(json_array(id,session_id,person_id,status,source,confidence,checked_in_at,device_id,created_by,reason,created_at,updated_at)) FROM (SELECT * FROM attendance_records ORDER BY id)`
	const ledger = `SELECT group_concat(version||':'||checksum,'|') FROM (SELECT version,checksum FROM schema_migrations WHERE version<=5 ORDER BY version)`
	const recordFKs = `SELECT json_group_array(json_array(id,seq,"table","from","to",on_update,on_delete,match)) FROM pragma_foreign_key_list('attendance_records')`
	const sessionFKs = `SELECT json_group_array(json_array(id,seq,"table","from","to",on_update,on_delete,match)) FROM pragma_foreign_key_list('attendance_sessions')`
	before := map[string]string{}
	for _, query := range []string{sessions, records, ledger, recordFKs, sessionFKs} {
		before[query] = core02MigrationSnapshot(t, db, query)
	}
	status, err := Run(ctx, db, full)
	if err != nil || status.CurrentVersion != 6 || status.Applied != 1 {
		t.Fatalf("upgrade=%+v %v", status, err)
	}
	for query, want := range before {
		if got := core02MigrationSnapshot(t, db, query); got != want {
			t.Fatalf("upgrade changed legacy values or FK\nquery=%s\nbefore=%s\nafter=%s", query, want, got)
		}
	}
	for _, query := range []string{
		`SELECT COUNT(*) FROM attendance_sessions WHERE id IN('legacy-open','legacy-closed') AND started_at=opens_at`,
		`SELECT COUNT(*) FROM attendance_sessions WHERE id='legacy-cancel' AND status='cancelled' AND started_at IS NULL`,
		`SELECT COUNT(*) FROM attendance_records`,
		`SELECT COUNT(*) FROM audit_events WHERE id='legacy-audit' AND reason='Alasan lama'`,
		`SELECT COUNT(*) FROM idempotency_records WHERE scope='operator:legacy-admin' AND key='legacy-key'`,
	} {
		var got int
		if err := db.QueryRowContext(ctx, query).Scan(&got); err != nil {
			t.Fatal(err)
		}
		want := 1
		if query == `SELECT COUNT(*) FROM attendance_records` || query == `SELECT COUNT(*) FROM attendance_sessions WHERE id IN('legacy-open','legacy-closed') AND started_at=opens_at` {
			want = 2
		}
		if got != want {
			t.Fatalf("preservation query=%s got=%d want=%d", query, got, want)
		}
	}
	var foreignKeys, violations int
	if err := db.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_foreign_key_check`).Scan(&violations); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 || violations != 0 {
		t.Fatalf("FK state=%d violations=%d", foreignKeys, violations)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO attendance_sessions(id,class_id,closes_at,created_by,title) VALUES('new-draft','legacy-closed-class','2026-10-25T02:00:00.000Z','legacy-admin','Draft sintetis')`); err != nil {
		t.Fatalf("draft default rejected=%v", err)
	}
	if got := core02MigrationSnapshot(t, db, `SELECT status FROM attendance_sessions WHERE id='new-draft'`); got != "draft" {
		t.Fatalf("default status=%s", got)
	}
	for _, query := range []string{
		`INSERT INTO attendance_sessions(id,class_id,status,closes_at,created_by) VALUES('duplicate-open','legacy-open-class','open','2026-10-25T02:00:00.000Z','legacy-admin')`,
		`INSERT INTO attendance_sessions(id,class_id,device_id,status,closes_at,created_by) VALUES('duplicate-device','legacy-closed-class','legacy-device','open','2026-10-25T02:00:00.000Z','legacy-admin')`,
		`INSERT INTO attendance_records(id,session_id,person_id,status,source,checked_in_at) VALUES('duplicate-record','legacy-open','legacy-person','present','import','2026-01-01T01:06:00.000Z')`,
		`INSERT INTO attendance_records(id,session_id,person_id,status,source,checked_in_at) VALUES('orphan','nonexistent','legacy-person','present','import','2026-01-01T01:06:00.000Z')`,
		`INSERT INTO attendance_sessions(id,class_id,status,opens_at,closes_at,created_by) VALUES('invalid-time','legacy-closed-class','draft','2026-01-01T02:00:00.000Z','2026-01-01T01:00:00.000Z','legacy-admin')`,
		`UPDATE attendance_sessions SET status='unrecognized' WHERE id='new-draft'`,
	} {
		if _, err := db.ExecContext(ctx, query); err == nil {
			t.Fatalf("preserved constraint was lost: %s", query)
		}
	}
	for _, name := range []string{"uq_sessions_one_open_per_class", "uq_sessions_one_open_per_device", "idx_sessions_class_status", "idx_sessions_closes_at", "uq_attendance_session_person", "idx_attendance_person", "idx_attendance_session_status"} {
		var got int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_schema WHERE type='index' AND name=?`, name).Scan(&got); err != nil || got != 1 {
			t.Fatalf("index %s missing=%d %v", name, got, err)
		}
	}
	repeat, err := Run(ctx, db, full)
	if err != nil || repeat.Applied != 0 || repeat.CurrentVersion != 6 {
		t.Fatalf("repeat upgrade=%+v %v", repeat, err)
	}
	if got := core02MigrationSnapshot(t, db, records); got != before[records] {
		t.Fatal("repeated upgrade changed old attendance")
	}
}

func TestCORE02FailedUpgradeRollsBackSchemaDataAndLedger(t *testing.T) {
	ctx := context.Background()
	full, _ := schemaPath(t)
	old := t.TempDir()
	list, err := Load(full)
	if err != nil {
		t.Fatal(err)
	}
	for _, migration := range list {
		if migration.Version <= 5 {
			body, err := os.ReadFile(migration.Path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(old, filepath.Base(migration.Path)), body, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	db := openDB(t)
	if _, err := Run(ctx, db, old); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO users(id,email,display_name,role,status) VALUES('synthetic-admin','migration@example.invalid','Admin Sintetis','admin','active');
		INSERT INTO classes(id,code,title) VALUES('synthetic-class','UPGRADE-FAIL','Kelas Sintetis');
		INSERT INTO attendance_sessions(id,class_id,status,closes_at,created_by,note) VALUES('synthetic-session','synthetic-class','cancelled','2026-01-01T02:00:00.000Z','synthetic-admin','Preserve old note');
		CREATE TRIGGER synthetic_migration_failure BEFORE UPDATE ON attendance_sessions BEGIN SELECT RAISE(ABORT,'synthetic migration failure'); END;
	`); err != nil {
		t.Fatal(err)
	}
	const ledger = `SELECT group_concat(version||':'||checksum,'|') FROM (SELECT version,checksum FROM schema_migrations ORDER BY version)`
	before := core02MigrationSnapshot(t, db, ledger)
	if _, err := Run(ctx, db, full); err == nil {
		t.Fatal("injected migration failure was accepted")
	}
	if got := core02MigrationSnapshot(t, db, ledger); got != before {
		t.Fatalf("failed upgrade changed ledger=%s", got)
	}
	var partial int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('attendance_sessions') WHERE name IN('status_legacy','title','started_at','close_reason')`).Scan(&partial); err != nil {
		t.Fatal(err)
	}
	if partial != 0 {
		t.Fatal("failed upgrade leaked partial columns")
	}
	if got := core02MigrationSnapshot(t, db, `SELECT status||':'||note FROM attendance_sessions WHERE id='synthetic-session'`); got != "cancelled:Preserve old note" {
		t.Fatalf("failed upgrade changed old data=%s", got)
	}
}

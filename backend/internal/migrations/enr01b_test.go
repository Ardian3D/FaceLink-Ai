package migrations

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollmentfixture"
)

func enr01bLegacy(t *testing.T) (*database.DB, string) {
	t.Helper()
	full, _ := schemaPath(t)
	old := t.TempDir()
	list, err := Load(full)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range list {
		if m.Version <= 6 {
			body, err := os.ReadFile(m.Path)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(old, filepath.Base(m.Path)), body, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	db := openDB(t)
	if status, err := Run(context.Background(), db, old); err != nil || status.CurrentVersion != 6 {
		t.Fatalf("legacy=%+v %v", status, err)
	}
	if _, err = db.ExecContext(context.Background(), `INSERT INTO users(id,email,display_name,role,status) VALUES('synthetic-admin','enr01b@example.invalid','Synthetic Admin','admin','active'),('synthetic-user','student-enr01b@example.invalid','Synthetic Student','mahasiswa','active');
	INSERT INTO people(id,external_code,display_name,account_user_id,identity_reference,identity_verified_at) VALUES('synthetic-person','SYNTHETIC-ENR01B','Synthetic Person','synthetic-user','synthetic-ref','2026-10-26T12:00:00.000Z');
	INSERT INTO consent_records(id,person_id,purpose,status,granted_at,method,recorded_by) VALUES('legacy-consent','synthetic-person','attendance_photo','granted','2026-10-26T12:00:00.000Z','written','synthetic-admin');
	INSERT INTO audit_events(id,actor_type,action,result) VALUES('legacy-enr01b-audit','system','synthetic.legacy','success');
	INSERT INTO idempotency_records(scope,key,request_hash,response_code,response_body,expires_at) VALUES('legacy-enr01b','legacy-key','safe-hash',200,'{}','2026-10-27T12:00:00.000Z');`); err != nil {
		t.Fatal(err)
	}
	combined, err := enrollmentfixture.PrepareMigrations(full, filepath.Join(t.TempDir(), "combined"))
	if err != nil {
		t.Fatal(err)
	}
	return db, combined
}
func enr01bSnapshot(t *testing.T, db *database.DB, q string) string {
	t.Helper()
	var v string
	if err := db.QueryRowContext(context.Background(), q).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}
func TestENR01BUpgradeIsAdditiveAndPreservesPriorChecksums(t *testing.T) {
	db, full := enr01bLegacy(t)
	ctx := context.Background()
	queries := []string{`SELECT group_concat(version||':'||checksum,'|') FROM (SELECT version,checksum FROM schema_migrations WHERE version<=6 ORDER BY version)`, `SELECT json_group_array(json_array(id,email,role,status)) FROM (SELECT * FROM users ORDER BY id)`, `SELECT json_group_array(json_array(id,external_code,account_user_id,identity_reference,identity_verified_at)) FROM people`, `SELECT json_group_array(json_array(id,person_id,purpose,status,granted_at,method,recorded_by)) FROM consent_records`, `SELECT json_group_array(json_array(id,action,result)) FROM audit_events`, `SELECT json_group_array(json_array(scope,key,request_hash,response_code,response_body)) FROM idempotency_records`, `SELECT json_group_array(json_array(id,seq,"table","from","to",on_delete)) FROM pragma_foreign_key_list('attendance_records')`}
	before := map[string]string{}
	for _, q := range queries {
		before[q] = enr01bSnapshot(t, db, q)
	}
	status, err := Run(ctx, db, full)
	if err != nil || status.CurrentVersion != 8 || status.Applied != 2 {
		t.Fatalf("upgrade=%+v %v", status, err)
	}
	for _, q := range queries {
		if got := enr01bSnapshot(t, db, q); got != before[q] {
			t.Fatalf("changed legacy data/checksum/FK: %s\nbefore=%s\nafter=%s", q, before[q], got)
		}
	}
	if got := enr01bSnapshot(t, db, `SELECT COUNT(*) FROM pragma_foreign_key_check`); got != "0" {
		t.Fatalf("FK=%s", got)
	}
	for _, table := range []string{"face_enrollment_submissions", "face_enrollment_photos", "face_templates", "enrollment_processing_jobs", "biometric_deletion_jobs", "enrollment_file_deletions", "enrollment_rate_buckets"} {
		if got := enr01bSnapshot(t, db, `SELECT COUNT(*) FROM sqlite_schema WHERE type='table' AND name='`+table+`'`); got != "1" {
			t.Fatalf("missing table %s", table)
		}
	}
	if got := enr01bSnapshot(t, db, `SELECT COUNT(*) FROM face_templates`); got != "0" {
		t.Fatal("migration generated template")
	}
	if repeat, err := Run(ctx, db, full); err != nil || repeat.Applied != 0 || repeat.CurrentVersion != 8 {
		t.Fatalf("repeat=%+v %v", repeat, err)
	}
}
func TestENR01BFailedUpgradeRollsBackSchemaDataAndLedger(t *testing.T) {
	db, full := enr01bLegacy(t)
	ctx := context.Background()
	before := enr01bSnapshot(t, db, `SELECT group_concat(version||':'||checksum,'|') FROM (SELECT version,checksum FROM schema_migrations ORDER BY version)`)
	if _, err := db.ExecContext(ctx, `CREATE TRIGGER synthetic_enr01b_upgrade_failure BEFORE UPDATE ON consent_records BEGIN SELECT RAISE(ABORT,'synthetic upgrade failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(ctx, db, full); err == nil {
		t.Fatal("upgrade fault did not fail")
	}
	if got := enr01bSnapshot(t, db, `SELECT group_concat(version||':'||checksum,'|') FROM (SELECT version,checksum FROM schema_migrations ORDER BY version)`); got != before {
		t.Fatal("failed migration changed ledger")
	}
	for _, q := range []string{`SELECT COUNT(*) FROM pragma_table_info('people') WHERE name='biometric_epoch'`, `SELECT COUNT(*) FROM pragma_table_info('users') WHERE name='enrollment_auth_epoch'`, `SELECT COUNT(*) FROM sqlite_schema WHERE name='face_templates'`, `SELECT COUNT(*) FROM pragma_table_info('consent_records') WHERE name='method_legacy'`} {
		if got := enr01bSnapshot(t, db, q); got != "0" {
			t.Fatalf("partial schema: %s = %s", q, got)
		}
	}
	if got := enr01bSnapshot(t, db, `SELECT method FROM consent_records WHERE id='legacy-consent'`); got != "written" {
		t.Fatal("legacy consent changed")
	}
}
func TestENR01BConstraintsAndMonotonicLifecycleHooks(t *testing.T) {
	db, full := enr01bLegacy(t)
	ctx := context.Background()
	if _, err := Run(ctx, db, full); err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("a", 64)
	if _, err := db.ExecContext(ctx, `INSERT INTO consent_records(id,person_id,purpose,status,granted_at,method,recorded_by,subject_user_id,notice_version,notice_digest,policy_version) VALUES('synthetic-consent','synthetic-person','face_enrollment','granted','2026-10-26T12:00:00.000Z','digital_explicit','synthetic-user','synthetic-user','synthetic-v1',?,'synthetic-v1');
	INSERT INTO face_enrollment_submissions(id,person_id,subject_user_id,subject_session_id,subject_session_epoch,consent_id,status,submitted_at,photo_expires_at,canonical_sha256,captured_epoch,updated_at) VALUES('synthetic-submission','synthetic-person','synthetic-user','synthetic-session',0,'synthetic-consent','PENDING_APPROVAL','2026-10-26T12:00:00.000Z','2026-10-29T12:00:00.000Z',?,0,'2026-10-26T12:00:00.000Z');
	INSERT INTO face_enrollment_photos(submission_id,key_version,nonce,storage_state,width,height,byte_length,canonical_sha256,expires_at) VALUES('synthetic-submission','1',zeroblob(12),'STAGED',112,112,400,?,'2026-10-29T12:00:00.000Z');`, hash, hash, hash); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`UPDATE consent_records SET notice_version='other' WHERE id='synthetic-consent'`,
		`UPDATE face_enrollment_photos SET byte_length=401 WHERE submission_id='synthetic-submission'`,
		`UPDATE face_enrollment_submissions SET canonical_sha256='` + strings.Repeat("b", 64) + `',revision=revision+1 WHERE id='synthetic-submission'`,
		`UPDATE face_enrollment_submissions SET status='APPROVED_PROCESSING',revision=revision+1 WHERE id='synthetic-submission'`,
		`INSERT INTO face_enrollment_submissions(id,person_id,subject_user_id,subject_session_id,subject_session_epoch,consent_id,status,submitted_at,photo_expires_at,canonical_sha256,captured_epoch,updated_at) VALUES('duplicate','synthetic-person','synthetic-user','synthetic-session',0,'synthetic-consent','PENDING_APPROVAL','2026-10-26T12:00:00.000Z','2026-10-29T12:00:00.000Z','` + hash + `',0,'2026-10-26T12:00:00.000Z')`,
	} {
		if _, err := db.ExecContext(ctx, q); err == nil {
			t.Fatalf("constraint accepted: %s", q)
		}
	}
	if _, err := db.ExecContext(ctx, `UPDATE face_enrollment_photos SET storage_state='READY' WHERE submission_id='synthetic-submission';UPDATE users SET role='dosen' WHERE id='synthetic-admin';UPDATE users SET role='admin' WHERE id='synthetic-admin';UPDATE people SET account_user_id=NULL WHERE id='synthetic-person';UPDATE people SET account_user_id='synthetic-user' WHERE id='synthetic-person'`); err != nil {
		t.Fatal(err)
	}
	if got := enr01bSnapshot(t, db, `SELECT enrollment_auth_epoch FROM users WHERE id='synthetic-admin'`); got != "2" {
		t.Fatalf("reviewer fence=%s", got)
	}
	if got := enr01bSnapshot(t, db, `SELECT biometric_epoch FROM people WHERE id='synthetic-person'`); got != "2" {
		t.Fatalf("restored link fence=%s", got)
	}
	if _, err := db.ExecContext(ctx, `UPDATE face_enrollment_submissions SET status='REJECTED',review_decision='REJECTED',reviewer_admin_id='synthetic-admin',reviewer_epoch=2,reviewed_at='2026-10-26T12:01:00.000Z',rejection_reason_code='EVIDENCE_INSUFFICIENT',revision=revision+1 WHERE id='synthetic-submission';DELETE FROM face_enrollment_photos WHERE submission_id='synthetic-submission'`); err != nil {
		t.Fatal(err)
	}
	if got := enr01bSnapshot(t, db, `SELECT COUNT(*) FROM enrollment_file_deletions WHERE submission_id='synthetic-submission'`); got != "1" {
		t.Fatal("file deletion not durable")
	}
	if _, err := db.ExecContext(ctx, `UPDATE face_enrollment_submissions SET rejection_reason_code='IDENTITY_MISMATCH',revision=revision+1 WHERE id='synthetic-submission'`); err == nil {
		t.Fatal("review decision changed")
	}
}

func TestENR01BDefaultDiscoveryRemainsSchemaSixWithoutBiometricTables(t *testing.T) {
	base, _ := schemaPath(t)
	db := openDB(t)
	status, err := Run(context.Background(), db, base)
	if err != nil || status.CurrentVersion != 6 {
		t.Fatalf("default migration discovery=%+v %v", status, err)
	}
	if got := enr01bSnapshot(t, db, `SELECT COUNT(*) FROM sqlite_schema WHERE name IN ('face_templates','face_enrollment_submissions','face_enrollment_photos')`); got != "0" {
		t.Fatal("default startup discovered synthetic biometric schema")
	}
	fresh := filepath.Join(t.TempDir(), "explicit")
	combined, err := enrollmentfixture.PrepareMigrations(base, fresh)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = enrollmentfixture.PrepareMigrations(base, fresh); err == nil {
		t.Fatal("composition reused existing destination")
	}
	list, err := Load(combined)
	if err != nil || len(list) != 8 || list[len(list)-1].Version != 8 {
		t.Fatalf("explicit synthetic composition=%d %v", len(list), err)
	}
}

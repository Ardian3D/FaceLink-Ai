package migrations

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSEC01UpgradePreservesLegacyTemplateBytesLedgerAndForeignKeys(t *testing.T) {
	db, full := enr01bLegacy(t)
	ctx := context.Background()
	list, err := Load(full)
	if err != nil {
		t.Fatal(err)
	}
	throughSeven := t.TempDir()
	for _, migration := range list {
		if migration.Version <= 7 {
			if err = os.WriteFile(filepath.Join(throughSeven, filepath.Base(migration.Path)), []byte(migration.SQL), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if status, err := Run(ctx, db, throughSeven); err != nil || status.CurrentVersion != 7 {
		t.Fatalf("v7 fixture=%+v %v", status, err)
	}
	hash := strings.Repeat("a", 64)
	if _, err = db.ExecContext(ctx, `INSERT INTO consent_records(id,person_id,purpose,status,granted_at,method,recorded_by,subject_user_id,notice_version,notice_digest,policy_version) VALUES('legacy-grant','synthetic-person','face_enrollment','granted','2026-10-26T12:00:00.000Z','digital_explicit','synthetic-user','synthetic-user','synthetic-v1',?,'synthetic-v1');
	INSERT INTO face_enrollment_submissions(id,person_id,subject_user_id,subject_session_id,subject_session_epoch,consent_id,status,submitted_at,photo_expires_at,canonical_sha256,captured_epoch,review_decision,reviewer_admin_id,reviewer_epoch,reviewed_at,identity_check_method,identity_attestation_version,campus_check_ref,updated_at) VALUES('legacy-submission','synthetic-person','synthetic-user','synthetic-session',0,'legacy-grant','APPROVED_PROCESSING','2026-10-26T12:00:00.000Z','2026-10-29T12:00:00.000Z',?,0,'APPROVED','synthetic-admin',0,'2026-10-26T12:01:00.000Z','CAMPUS_AUTHORIZED_CHECK','synthetic-v1','synthetic-ref','2026-10-26T12:01:00.000Z');
	INSERT INTO face_enrollments(id,person_id,status,model_name,model_version,model_checksum,sample_count,enrolled_at,created_by) VALUES('legacy-activation','synthetic-person','active','synthetic-model','1',?,1,'2026-10-26T12:01:00.000Z','synthetic-admin');
	INSERT INTO face_templates(id,person_id,activation_enrollment_id,consent_id,submission_id,ciphertext,nonce,aad,envelope_version,algorithm,key_version,embedding_dim,dtype,model_id,model_version,model_checksum,detector_id,detector_version,detector_checksum,pipeline_revision,synthetic_only,created_at) VALUES('legacy-template','synthetic-person','legacy-activation','legacy-grant','legacy-submission',randomblob(528),randomblob(12),'historical-v1-aad',1,'AES-256-GCM','legacy-key',128,'float32-le','synthetic-model','1',?,'synthetic-detector','1',?,'synthetic-pipeline',1,'2026-10-26T12:01:00.000Z');
	UPDATE face_enrollment_submissions SET status='ACTIVE',activated_enrollment_id='legacy-activation',revision=revision+1 WHERE id='legacy-submission';`, hash, hash, hash, hash, hash); err != nil {
		t.Fatal(err)
	}
	queries := []string{
		`SELECT json_group_array(json_array(id,person_id,activation_enrollment_id,consent_id,submission_id,hex(ciphertext),hex(nonce),aad,envelope_version,algorithm,key_version,embedding_dim,dtype,model_id,model_version,model_checksum,detector_id,detector_version,detector_checksum,pipeline_revision,synthetic_only,created_at)) FROM face_templates`,
		`SELECT group_concat(version||':'||checksum,'|') FROM (SELECT version,checksum FROM schema_migrations WHERE version<=7 ORDER BY version)`,
		`SELECT json_group_array(json_array(id,seq,"table","from","to",on_update,on_delete,"match")) FROM pragma_foreign_key_list('face_templates')`,
		`SELECT json_group_array(json_array(id,status,activated_enrollment_id,revision)) FROM face_enrollment_submissions`,
	}
	before := make(map[string]string)
	for _, query := range queries {
		before[query] = enr01bSnapshot(t, db, query)
	}
	if status, err := Run(ctx, db, full); err != nil || status.CurrentVersion != 8 || status.Applied != 1 {
		t.Fatalf("v8 upgrade=%+v %v", status, err)
	}
	for _, query := range queries {
		if got := enr01bSnapshot(t, db, query); got != before[query] {
			t.Fatalf("legacy rows, ledger or foreign keys changed: %s", query)
		}
	}
	if got := enr01bSnapshot(t, db, `SELECT count(*) FROM pragma_foreign_key_check`); got != "0" {
		t.Fatalf("foreign-key violations=%s", got)
	}
	if _, err = db.ExecContext(ctx, `UPDATE face_templates SET aad='changed' WHERE id='legacy-template'`); err == nil {
		t.Fatal("rebuild lost template immutability")
	}
	// Remove the quarantined row explicitly and restore its parent to the legal
	// insertion phase, isolating the version guard from uniqueness/lifecycle guards.
	if _, err = db.ExecContext(ctx, `DROP TRIGGER enrollment_submission_immutable;DELETE FROM face_templates;UPDATE face_enrollment_submissions SET status='APPROVED_PROCESSING',activated_enrollment_id=NULL WHERE id='legacy-submission'`); err != nil {
		t.Fatal(err)
	}
	insert := `INSERT INTO face_templates(id,person_id,activation_enrollment_id,consent_id,submission_id,ciphertext,nonce,aad,envelope_version,algorithm,key_version,embedding_dim,dtype,model_id,model_version,model_checksum,detector_id,detector_version,detector_checksum,pipeline_revision,synthetic_only,created_at) VALUES('new-template','synthetic-person','legacy-activation','legacy-grant','legacy-submission',zeroblob(528),zeroblob(12),'opaque',?,'AES-256-GCM','new-key',128,'float32-le','synthetic-model','1',?,'synthetic-detector','1',?,'synthetic-pipeline',1,'2026-10-26T12:02:00.000Z')`
	if _, err = db.ExecContext(ctx, insert, 1, hash, hash); err == nil {
		t.Fatal("v8 accepted a new v1 template")
	}
	if _, err = db.ExecContext(ctx, insert, 2, hash, hash); err != nil {
		t.Fatalf("version guard rejected a legally bound v2 row: %v", err)
	}
	if _, err = db.ExecContext(ctx, `DELETE FROM consent_records WHERE id='legacy-grant'`); err == nil {
		t.Fatal("rebuild lost consent foreign-key restriction")
	}
}

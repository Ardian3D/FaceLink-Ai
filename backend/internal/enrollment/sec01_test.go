package enrollment

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollmentfixture"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollmentsecure"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/ids"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/migrations"
)

func activateSEC01(t *testing.T, f *fixture) string {
	t.Helper()
	_, submitted := f.submit(t, f.grant(t))
	id := submitted.Data.(MutationReceipt).ResourceID
	f.approve(t, id)
	if err := f.s.ProcessNext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if view := run(t, f, f.request(Status)).Data.(EnrollmentView); !view.TemplateReady || view.SubmissionStatus != "ACTIVE" {
		t.Fatalf("positive activation=%+v", view)
	}
	return id
}

func templateSEC01(t *testing.T, f *fixture, submissionID string) (string, enrollmentsecure.Envelope) {
	t.Helper()
	var id string
	var envelope enrollmentsecure.Envelope
	if err := f.db.QueryRowContext(context.Background(), `SELECT id,envelope_version,algorithm,key_version,aad,nonce,ciphertext FROM face_templates WHERE submission_id=?`, submissionID).Scan(&id, &envelope.Version, &envelope.Algorithm, &envelope.KeyVersion, &envelope.AAD, &envelope.Nonce, &envelope.Ciphertext); err != nil {
		t.Fatal(err)
	}
	return id, envelope
}

func assertTemplateVectorSEC01(t *testing.T, f *fixture, submissionID string, index int) {
	t.Helper()
	recordID, envelope := templateSEC01(t, f, submissionID)
	var grantID string
	if err := f.db.QueryRowContext(context.Background(), `SELECT consent_id FROM face_enrollment_submissions WHERE id=? AND person_id=?`, submissionID, f.person).Scan(&grantID); err != nil {
		t.Fatal(err)
	}
	manifest := syntheticManifest()
	plain, err := enrollmentsecure.Open(context.Background(), f.keys, enrollmentsecure.Binding{
		Purpose: enrollmentsecure.Template, RecordID: recordID, SubmissionID: submissionID,
		PersonID: f.person, ConsentID: grantID, ModelID: manifest.ModelID,
		ModelVersion: manifest.ModelVersion, ModelFingerprint: manifest.Fingerprint(),
	}, envelope)
	defer wipe(plain)
	if err != nil || len(plain) != 512 {
		t.Fatalf("record-bound template unavailable: %v", err)
	}
	for i := range 128 {
		want := uint32(0)
		if i == index {
			want = 0x3f800000
		}
		if binary.LittleEndian.Uint32(plain[i*4:]) != want {
			t.Fatal("concurrent processing mixed subject embeddings")
		}
	}
}

func secondSubjectSEC01(t *testing.T, f *fixture) *fixture {
	t.Helper()
	other := *f
	other.user, other.person, other.userSession = ids.New(), ids.New(), ids.New()
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO users(id,email,display_name,role,status) VALUES(?,?,'Synthetic second subject','mahasiswa','active')`,
			[]any{other.user, other.user + "@example.invalid"}},
		{`INSERT INTO people(id,external_code,display_name,account_user_id,identity_reference,identity_verified_at) VALUES(?,?,'Synthetic second person',?,'synthetic-second',?)`,
			[]any{other.person, ids.New(), other.user, nowText(f.clk.Now())}},
		{`INSERT INTO class_memberships(class_id,person_id,created_by) SELECT class_id,?,? FROM class_memberships WHERE person_id=? LIMIT 1`,
			[]any{other.person, f.admin, f.person}},
		{`INSERT INTO session_tokens(id,user_id,access_hash,refresh_hash,access_expires_at,refresh_expires_at,absolute_expires_at) VALUES(?,?,?,?,?,?,?)`,
			[]any{other.userSession, other.user, ids.New(), ids.New(), nowText(f.clk.Now().Add(time.Hour)), nowText(f.clk.Now().Add(time.Hour)), nowText(f.clk.Now().Add(time.Hour))}},
	} {
		if _, err := f.db.ExecContext(context.Background(), statement.query, statement.args...); err != nil {
			t.Fatal(err)
		}
	}
	return &other
}

func TestManifestFingerprintIncludesCompleteStablePipeline(t *testing.T) {
	base := syntheticManifest()
	fingerprint := base.Fingerprint()
	if decoded, err := hex.DecodeString(fingerprint); err != nil || len(decoded) != 32 || fingerprint != strings.ToLower(fingerprint) {
		t.Fatal("fingerprint is not lowercase SHA256")
	}
	upper := base
	upper.ModelChecksum = strings.ToUpper(upper.ModelChecksum)
	upper.DetectorChecksum = strings.ToUpper(upper.DetectorChecksum)
	if upper.Fingerprint() != fingerprint {
		t.Fatal("hexadecimal checksum case changed pipeline identity")
	}
	for _, change := range []func(*Manifest){
		func(m *Manifest) { m.SyntheticOnly = false },
		func(m *Manifest) { m.ModelID += "-other" },
		func(m *Manifest) { m.ModelVersion += "-other" },
		func(m *Manifest) { m.ModelChecksum = strings.Repeat("c", 64) },
		func(m *Manifest) { m.DetectorID += "-other" },
		func(m *Manifest) { m.DetectorVersion += "-other" },
		func(m *Manifest) { m.DetectorChecksum = strings.Repeat("c", 64) },
		func(m *Manifest) { m.PipelineRevision += "-other" },
	} {
		changed := base
		change(&changed)
		if changed.Fingerprint() == fingerprint {
			t.Fatal("pipeline identity field omitted from fingerprint")
		}
	}
	left, right := base, base
	left.ModelID, left.ModelVersion = "ab", "c"
	right.ModelID, right.ModelVersion = "a", "bc"
	if left.Fingerprint() == right.Fingerprint() {
		t.Fatal("field boundaries are ambiguous")
	}
}

func TestTemplateReadinessRejectsDatabaseRecordAndPipelineTampering(t *testing.T) {
	for _, attack := range []struct{ name, sql string }{
		{"model-id", `UPDATE face_templates SET model_id='other-model';UPDATE face_enrollments SET model_name='other-model'`},
		{"model-version", `UPDATE face_templates SET model_version='other-version';UPDATE face_enrollments SET model_version='other-version'`},
		{"model-checksum", `UPDATE face_templates SET model_checksum='` + strings.Repeat("c", 64) + `';UPDATE face_enrollments SET model_checksum='` + strings.Repeat("c", 64) + `'`},
		{"detector-id", `UPDATE face_templates SET detector_id='other-detector'`},
		{"detector-version", `UPDATE face_templates SET detector_version='other-version'`},
		{"detector-checksum", `UPDATE face_templates SET detector_checksum='` + strings.Repeat("c", 64) + `'`},
		{"pipeline-revision", `UPDATE face_templates SET pipeline_revision='other-pipeline'`},
		{"invalid-manifest", `UPDATE face_templates SET detector_id=''`},
		{"ciphertext-corruption", `UPDATE face_templates SET ciphertext=randomblob(528)`},
		{"envelope-aad", `UPDATE face_templates SET aad='untrusted-envelope-identity'`},
		{"activation-link", `UPDATE face_enrollments SET status='revoked'`},
	} {
		t.Run(attack.name, func(t *testing.T) {
			f := setup(t)
			activateSEC01(t, f)
			// Model a database-write attacker beyond the immutable application API.
			if _, err := f.db.ExecContext(context.Background(), `DROP TRIGGER enrollment_template_immutable;`+attack.sql); err != nil {
				t.Fatal(err)
			}
			assertUnavailableRetainedSEC01(t, f)
		})
	}
}

func assertUnavailableRetainedSEC01(t *testing.T, f *fixture) {
	t.Helper()
	for range 2 {
		view := run(t, f, f.request(Status)).Data.(EnrollmentView)
		if view.TemplateReady || view.ReasonCode != "BIOMETRIC_UNAVAILABLE" || view.SubmissionStatus != "ACTIVE" {
			t.Fatalf("tampered/unavailable template exposed as ready: %+v", view)
		}
		if err := f.s.Reconcile(context.Background()); err != nil {
			t.Fatal(err)
		}
		var err error
		f.s, err = New(f.db, f.clk, f.keys, f.files, &syntheticProcessor{})
		if err != nil {
			t.Fatal(err)
		}
	}
	if count(t, f, `SELECT count(*) FROM face_templates`) == 0 {
		t.Fatal("unavailability implicitly deleted quarantined ciphertext")
	}
}

func TestSamePersonEnvelopeTransplantsRejectEachIndependentIdentityChange(t *testing.T) {
	for _, identity := range []string{"template-record", "submission", "consent-grant"} {
		t.Run(identity, func(t *testing.T) {
			f := setup(t)
			submissionID := activateSEC01(t, f)
			recordID, saved := templateSEC01(t, f, submissionID)
			var grantID string
			if err := f.db.QueryRowContext(context.Background(), `SELECT consent_id FROM face_enrollment_submissions WHERE id=?`, submissionID).Scan(&grantID); err != nil {
				t.Fatal(err)
			}
			replacement := ids.New()
			err := f.s.transaction(context.Background(), func(c *sql.Conn) error {
				// Model coherent database tampering, not a rejected FK write:
				// every join remains valid and exactly ONE AAD identity changes.
				if _, err := c.ExecContext(context.Background(), `PRAGMA defer_foreign_keys=ON;DROP TRIGGER enrollment_template_immutable`); err != nil {
					return err
				}
				switch identity {
				case "template-record":
					if _, err := c.ExecContext(context.Background(), `UPDATE face_templates SET id=? WHERE id=?`, replacement, recordID); err != nil {
						return err
					}
				case "submission":
					if _, err := c.ExecContext(context.Background(), `DROP TRIGGER enrollment_submission_immutable;DROP TRIGGER enrollment_job_binding_update`); err != nil {
						return err
					}
					for _, query := range []string{
						`UPDATE face_enrollment_submissions SET id=? WHERE id=?`,
						`UPDATE face_templates SET submission_id=? WHERE submission_id=?`,
						`UPDATE enrollment_processing_jobs SET submission_id=? WHERE submission_id=?`,
					} {
						if _, err := c.ExecContext(context.Background(), query, replacement, submissionID); err != nil {
							return err
						}
					}
				case "consent-grant":
					if _, err := c.ExecContext(context.Background(), `DROP TRIGGER enrollment_consent_evidence_immutable;DROP TRIGGER enrollment_submission_immutable`); err != nil {
						return err
					}
					for _, query := range []string{
						`UPDATE consent_records SET id=? WHERE id=?`,
						`UPDATE face_enrollment_submissions SET consent_id=? WHERE consent_id=?`,
						`UPDATE face_templates SET consent_id=? WHERE consent_id=?`,
					} {
						if _, err := c.ExecContext(context.Background(), query, replacement, grantID); err != nil {
							return err
						}
					}
				}
				// Transplant the complete saved envelope, including AAD. Person,
				// model/version/fingerprint and the other two identities are unchanged.
				_, err := c.ExecContext(context.Background(), `UPDATE face_templates SET envelope_version=?,algorithm=?,key_version=?,aad=?,nonce=?,ciphertext=? WHERE person_id=?`,
					saved.Version, saved.Algorithm, saved.KeyVersion, saved.AAD, saved.Nonce, saved.Ciphertext, f.person)
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			if count(t, f, `SELECT count(*) FROM pragma_foreign_key_check`) != 0 {
				t.Fatal("identity attack did not preserve valid relational metadata")
			}
			assertUnavailableRetainedSEC01(t, f)
		})
	}
}

func TestTemplateEnvelopeSubmissionAndGrantSwapsFailClosed(t *testing.T) {
	for _, attack := range []string{"envelope", "submission", "consent"} {
		t.Run(attack, func(t *testing.T) {
			f := setup(t)
			id := activateSEC01(t, f)
			donor := secondSubjectSEC01(t, f)
			donorID := activateSEC01(t, donor)
			_, envelope := templateSEC01(t, donor, donorID)
			if _, err := f.db.ExecContext(context.Background(), `DROP TRIGGER enrollment_template_immutable`); err != nil {
				t.Fatal(err)
			}
			var err error
			switch attack {
			case "envelope":
				_, err = f.db.ExecContext(context.Background(), `DELETE FROM face_templates WHERE submission_id=?`, donorID)
				if err == nil {
					_, err = f.db.ExecContext(context.Background(), `UPDATE face_templates SET envelope_version=?,algorithm=?,key_version=?,aad=?,nonce=?,ciphertext=? WHERE submission_id=?`, envelope.Version, envelope.Algorithm, envelope.KeyVersion, envelope.AAD, envelope.Nonce, envelope.Ciphertext, id)
				}
			case "submission":
				_, err = f.db.ExecContext(context.Background(), `DELETE FROM face_templates WHERE submission_id=?`, donorID)
				if err == nil {
					_, err = f.db.ExecContext(context.Background(), `UPDATE face_templates SET submission_id=? WHERE submission_id=?`, donorID, id)
				}
			case "consent":
				_, err = f.db.ExecContext(context.Background(), `UPDATE face_templates SET consent_id=(SELECT consent_id FROM face_enrollment_submissions WHERE id=?) WHERE submission_id=?`, donorID, id)
			}
			if err != nil {
				t.Fatal(err)
			}
			assertUnavailableRetainedSEC01(t, f)
		})
	}
}

func TestPreviousGrantEnvelopeCannotBeReplayedForSamePerson(t *testing.T) {
	f := setup(t)
	oldSubmission := activateSEC01(t, f)
	_, old := templateSEC01(t, f, oldSubmission)
	withdraw := f.request(Withdraw)
	withdraw.Confirmed, withdraw.Scope = true, "ALL_FACE_ENROLLMENT"
	run(t, f, withdraw)
	if err := f.s.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	currentSubmission := activateSEC01(t, f)
	if _, err := f.db.ExecContext(context.Background(), `DROP TRIGGER enrollment_template_immutable;UPDATE face_templates SET aad=?,nonce=?,ciphertext=?,key_version=? WHERE submission_id=?`, old.AAD, old.Nonce, old.Ciphertext, old.KeyVersion, currentSubmission); err != nil {
		t.Fatal(err)
	}
	assertUnavailableRetainedSEC01(t, f)
}

func TestTemplateKeyLossAndServiceRestartPreserveCiphertext(t *testing.T) {
	f := setup(t)
	id := activateSEC01(t, f)
	_, before := templateSEC01(t, f, id)
	f.keys.setUnavailable(true)
	assertUnavailableRetainedSEC01(t, f)
	_, after := templateSEC01(t, f, id)
	if before.AAD != after.AAD || before.Version != after.Version || before.KeyVersion != after.KeyVersion || !bytes.Equal(before.Nonce, after.Nonce) || !bytes.Equal(before.Ciphertext, after.Ciphertext) {
		t.Fatal("key outage/restart/reconciliation changed ciphertext")
	}
	f.keys.setUnavailable(false)
	if view := run(t, f, f.request(Status)).Data.(EnrollmentView); !view.TemplateReady {
		t.Fatalf("restored original key did not recover readiness: %+v", view)
	}
}

func TestConsentGrantIdentityAndEvidenceCannotMutate(t *testing.T) {
	f := setup(t)
	grant := f.grant(t)
	for _, query := range []string{
		`UPDATE consent_records SET id=? WHERE id=?`,
		`UPDATE consent_records SET notice_version=? WHERE id=?`,
		`UPDATE consent_records SET policy_version=? WHERE id=?`,
	} {
		if _, err := f.db.ExecContext(context.Background(), query, ids.New(), grant); err == nil {
			t.Fatal("immutable grant identity/evidence changed")
		}
	}
	withdraw := f.request(Withdraw)
	withdraw.Confirmed, withdraw.Scope = true, "ALL_FACE_ENROLLMENT"
	run(t, f, withdraw)
	var id, state string
	if err := f.db.QueryRowContext(context.Background(), `SELECT id,status FROM consent_records WHERE id=?`, grant).Scan(&id, &state); err != nil || id != grant || state != "withdrawn" {
		t.Fatalf("withdrawal changed grant identity: %s %s %v", id, state, err)
	}
}

func TestLegacyTemplateUpgradeRemainsUnsupportedAndRetained(t *testing.T) {
	f := setupAtVersion(t, 7)
	_, submitted := f.submit(t, f.grant(t))
	id := submitted.Data.(MutationReceipt).ResourceID
	f.approve(t, id)
	activation, templateID := ids.New(), ids.New()
	manifest := syntheticManifest()
	// Construct an authentic historical v1 payload solely as upgrade input,
	// not as an alternate runtime codec or a pin on the new AAD serialization.
	var historicalAAD []byte
	for _, field := range []string{f.person, manifest.ModelID, manifest.ModelVersion, "test-1"} {
		historicalAAD = binary.BigEndian.AppendUint32(historicalAAD, uint32(len(field)))
		historicalAAD = append(historicalAAD, field...)
	}
	keyVersion, key, err := f.keys.Active(context.Background(), enrollmentsecure.Template)
	if err != nil {
		t.Fatal(err)
	}
	defer wipe(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	legacy := enrollmentsecure.Envelope{Version: 1, Algorithm: "AES-256-GCM", KeyVersion: keyVersion, AAD: hex.EncodeToString(historicalAAD), Nonce: bytes.Repeat([]byte{7}, 12)}
	plain := make([]byte, 512)
	binary.LittleEndian.PutUint32(plain, 0x3f800000)
	defer wipe(plain)
	legacy.Ciphertext = gcm.Seal(nil, legacy.Nonce, plain, []byte(legacy.AAD))
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO face_enrollments(id,person_id,status,model_name,model_version,model_checksum,sample_count,enrolled_at,created_by) VALUES(?,?,'active',?,?,?,1,?,?)`,
			[]any{activation, f.person, manifest.ModelID, manifest.ModelVersion, manifest.ModelChecksum, nowText(f.clk.Now()), f.admin}},
		{`INSERT INTO face_templates(id,person_id,activation_enrollment_id,consent_id,submission_id,ciphertext,nonce,aad,envelope_version,algorithm,key_version,embedding_dim,dtype,model_id,model_version,model_checksum,detector_id,detector_version,detector_checksum,pipeline_revision,synthetic_only,created_at) SELECT ?,person_id,?,consent_id,id,?,?,?,1,'AES-256-GCM',?,128,'float32-le',?,?,?,?,?,?,?,1,? FROM face_enrollment_submissions WHERE id=?`,
			[]any{templateID, activation, legacy.Ciphertext, legacy.Nonce, legacy.AAD, legacy.KeyVersion, manifest.ModelID, manifest.ModelVersion, manifest.ModelChecksum, manifest.DetectorID, manifest.DetectorVersion, manifest.DetectorChecksum, manifest.PipelineRevision, nowText(f.clk.Now()), id}},
		{`UPDATE face_enrollment_submissions SET status='ACTIVE',activated_enrollment_id=?,revision=revision+1 WHERE id=?`,
			[]any{activation, id}},
		{`UPDATE enrollment_processing_jobs SET state='FINISHED' WHERE submission_id=?`, []any{id}},
		{`DELETE FROM face_enrollment_photos WHERE submission_id=?`, []any{id}},
	} {
		result, err := f.db.ExecContext(context.Background(), statement.query, statement.args...)
		if err != nil {
			t.Fatal(err)
		}
		if affected, err := result.RowsAffected(); err != nil || affected != 1 {
			t.Fatalf("historical fixture statement affected %d rows: %v", affected, err)
		}
	}
	_, source, _, _ := runtime.Caller(0)
	full, err := enrollmentfixture.PrepareMigrations(filepath.Join(filepath.Dir(source), "..", "..", "migrations"), filepath.Join(t.TempDir(), "upgrade"))
	if err != nil {
		t.Fatal(err)
	}
	if status, err := migrations.Run(context.Background(), f.db, full); err != nil || status.CurrentVersion != 8 || status.Applied != 1 {
		t.Fatalf("legacy upgrade=%+v %v", status, err)
	}
	assertUnavailableRetainedSEC01(t, f)
	retainedID, retained := templateSEC01(t, f, id)
	if retainedID != templateID || retained.Version != 1 || retained.AAD != legacy.AAD || retained.KeyVersion != legacy.KeyVersion || !bytes.Equal(retained.Nonce, legacy.Nonce) || !bytes.Equal(retained.Ciphertext, legacy.Ciphertext) {
		t.Fatal("unsupported legacy envelope was converted or changed")
	}
	if count(t, f, `SELECT count(*) FROM pragma_foreign_key_check`) != 0 {
		t.Fatal("legacy upgrade broke foreign keys")
	}
}

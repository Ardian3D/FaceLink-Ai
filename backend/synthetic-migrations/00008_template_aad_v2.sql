-- FL-SEC01: disposable synthetic databases only; never compose into default migrations.
-- Retain legacy v1 bytes for quarantine. No decryption, conversion or deletion occurs.
-- face_templates has only outgoing FKs. Build/copy/drop/rename avoids renaming the
-- old table and redirecting any FK or trigger references to an obsolete name.
CREATE TABLE face_templates_aad_v2 (
 id TEXT PRIMARY KEY,
 person_id TEXT NOT NULL UNIQUE REFERENCES people(id) ON DELETE RESTRICT,
 activation_enrollment_id TEXT NOT NULL UNIQUE REFERENCES face_enrollments(id) ON DELETE RESTRICT,
 consent_id TEXT NOT NULL REFERENCES consent_records(id) ON DELETE RESTRICT,
 submission_id TEXT NOT NULL UNIQUE REFERENCES face_enrollment_submissions(id) ON DELETE RESTRICT,
 ciphertext BLOB NOT NULL CHECK(length(ciphertext)=528),
 nonce BLOB NOT NULL CHECK(length(nonce)=12),
 aad TEXT NOT NULL,
 envelope_version INTEGER NOT NULL CHECK(envelope_version IN (1,2)),
 algorithm TEXT NOT NULL CHECK(algorithm='AES-256-GCM'),
 key_version TEXT NOT NULL,
 embedding_dim INTEGER NOT NULL CHECK(embedding_dim=128),
 dtype TEXT NOT NULL CHECK(dtype='float32-le'),
 model_id TEXT NOT NULL, model_version TEXT NOT NULL, model_checksum TEXT NOT NULL CHECK(length(model_checksum)=64),
 detector_id TEXT NOT NULL, detector_version TEXT NOT NULL, detector_checksum TEXT NOT NULL CHECK(length(detector_checksum)=64),
 pipeline_revision TEXT NOT NULL,
 synthetic_only INTEGER NOT NULL CHECK(synthetic_only=1),
 created_at TEXT NOT NULL,
 UNIQUE(key_version,nonce)
);
INSERT INTO face_templates_aad_v2 (
 id,person_id,activation_enrollment_id,consent_id,submission_id,ciphertext,nonce,aad,
 envelope_version,algorithm,key_version,embedding_dim,dtype,model_id,model_version,
 model_checksum,detector_id,detector_version,detector_checksum,pipeline_revision,synthetic_only,created_at
) SELECT
 id,person_id,activation_enrollment_id,consent_id,submission_id,ciphertext,nonce,aad,
 envelope_version,algorithm,key_version,embedding_dim,dtype,model_id,model_version,
 model_checksum,detector_id,detector_version,detector_checksum,pipeline_revision,synthetic_only,created_at
 FROM face_templates;
DROP TRIGGER enrollment_template_immutable;
DROP TRIGGER enrollment_template_binding;
DROP TABLE face_templates;
ALTER TABLE face_templates_aad_v2 RENAME TO face_templates;
CREATE TRIGGER enrollment_template_immutable BEFORE UPDATE ON face_templates BEGIN SELECT RAISE(ABORT,'immutable enrollment template'); END;
CREATE TRIGGER enrollment_template_binding BEFORE INSERT ON face_templates
WHEN NEW.envelope_version<>2 OR NOT EXISTS(
 SELECT 1 FROM face_enrollment_submissions s
 JOIN face_enrollments a ON a.id=NEW.activation_enrollment_id
 JOIN consent_records c ON c.id=s.consent_id AND c.person_id=s.person_id AND c.subject_user_id=s.subject_user_id
 WHERE s.id=NEW.submission_id AND s.person_id=NEW.person_id AND s.consent_id=NEW.consent_id
 AND s.status='APPROVED_PROCESSING' AND s.review_decision='APPROVED'
 AND a.person_id=s.person_id AND a.status='active' AND c.status='granted')
BEGIN SELECT RAISE(ABORT,'template binding mismatch or unsupported write version'); END;

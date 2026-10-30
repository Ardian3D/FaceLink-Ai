-- FL-ENR01B: synthetic-only enrollment; apply only to an explicitly disposable DB.
-- No production startup injection is provided. Existing attendance/auth data survives.
ALTER TABLE people ADD COLUMN biometric_epoch INTEGER NOT NULL DEFAULT 0 CHECK (biometric_epoch >= 0);
ALTER TABLE people ADD COLUMN consent_revision INTEGER NOT NULL DEFAULT 0 CHECK (consent_revision >= 0);
ALTER TABLE users ADD COLUMN enrollment_auth_epoch INTEGER NOT NULL DEFAULT 0 CHECK (enrollment_auth_epoch >= 0);
ALTER TABLE session_tokens ADD COLUMN enrollment_epoch INTEGER NOT NULL DEFAULT 0 CHECK (enrollment_epoch >= 0);

-- Extend the CHECK without rebuilding a referenced parent or mislabelling a click as a signature.
ALTER TABLE consent_records RENAME COLUMN method TO method_legacy;
ALTER TABLE consent_records ADD COLUMN method TEXT NOT NULL DEFAULT 'written'
 CHECK (method IN ('written','digital_signature','verbal_witnessed','digital_explicit'));
UPDATE consent_records SET method=method_legacy;
ALTER TABLE consent_records DROP COLUMN method_legacy;
ALTER TABLE consent_records ADD COLUMN notice_version TEXT;
ALTER TABLE consent_records ADD COLUMN notice_digest TEXT;
ALTER TABLE consent_records ADD COLUMN subject_user_id TEXT REFERENCES users(id) ON DELETE RESTRICT;
ALTER TABLE consent_records ADD COLUMN withdrawn_by TEXT REFERENCES users(id) ON DELETE RESTRICT;
ALTER TABLE consent_records ADD COLUMN withdraw_reason_code TEXT;
ALTER TABLE consent_records ADD COLUMN policy_version TEXT;
CREATE UNIQUE INDEX uq_consent_enrollment_binding ON consent_records(id,person_id,subject_user_id);
CREATE TRIGGER enrollment_consent_evidence_immutable BEFORE UPDATE ON consent_records
WHEN OLD.notice_version IS NOT NULL AND (
 NEW.id IS NOT OLD.id OR NEW.person_id IS NOT OLD.person_id OR NEW.purpose IS NOT OLD.purpose OR
 NEW.subject_user_id IS NOT OLD.subject_user_id OR NEW.notice_version IS NOT OLD.notice_version OR
 NEW.notice_digest IS NOT OLD.notice_digest OR NEW.policy_version IS NOT OLD.policy_version OR
 NEW.method IS NOT OLD.method OR NEW.granted_at IS NOT OLD.granted_at OR NEW.recorded_by IS NOT OLD.recorded_by OR
 (OLD.status='withdrawn' AND NEW.status<>'withdrawn'))
BEGIN SELECT RAISE(ABORT,'immutable enrollment consent evidence'); END;
CREATE TRIGGER enrollment_consent_evidence_required BEFORE INSERT ON consent_records
WHEN NEW.method='digital_explicit' AND (NEW.purpose<>'face_enrollment' OR NEW.subject_user_id IS NULL OR
 NEW.recorded_by IS NOT NEW.subject_user_id OR NEW.notice_version IS NULL OR length(NEW.notice_digest)<>64 OR NEW.policy_version IS NULL)
BEGIN SELECT RAISE(ABORT,'incomplete enrollment consent evidence'); END;

CREATE TABLE face_enrollment_submissions (
 id TEXT PRIMARY KEY,
 person_id TEXT NOT NULL REFERENCES people(id) ON DELETE RESTRICT,
 subject_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 subject_session_id TEXT NOT NULL,
 subject_session_epoch INTEGER NOT NULL CHECK(subject_session_epoch>=0),
 consent_id TEXT NOT NULL,
 status TEXT NOT NULL CHECK(status IN ('PENDING_APPROVAL','APPROVED_PROCESSING','ACTIVE','REJECTED','FAILED_PROCESSING','REVOKED','EXPIRED')),
 submitted_at TEXT NOT NULL,
 photo_expires_at TEXT NOT NULL CHECK(photo_expires_at>submitted_at),
 canonical_sha256 TEXT NOT NULL CHECK(length(canonical_sha256)=64),
 revision INTEGER NOT NULL DEFAULT 1 CHECK(revision>0),
 captured_epoch INTEGER NOT NULL CHECK(captured_epoch>=0),
 review_decision TEXT CHECK(review_decision IN ('APPROVED','REJECTED')),
 reviewer_admin_id TEXT REFERENCES users(id) ON DELETE RESTRICT,
 reviewer_epoch INTEGER,
 reviewed_at TEXT,
 identity_check_method TEXT CHECK(identity_check_method IS NULL OR identity_check_method='CAMPUS_AUTHORIZED_CHECK'),
 identity_attestation_version TEXT,
 campus_check_ref TEXT,
 rejection_reason_code TEXT CHECK(rejection_reason_code IS NULL OR rejection_reason_code IN ('IDENTITY_MISMATCH','EVIDENCE_INSUFFICIENT','UNSUITABLE_SUBMISSION')),
 processing_error_code TEXT,
 activated_enrollment_id TEXT UNIQUE REFERENCES face_enrollments(id) ON DELETE RESTRICT,
 revoked_reason_code TEXT,
 updated_at TEXT NOT NULL,
 FOREIGN KEY(consent_id,person_id,subject_user_id) REFERENCES consent_records(id,person_id,subject_user_id) ON DELETE RESTRICT,
 CHECK ((review_decision IS NULL AND reviewer_admin_id IS NULL AND reviewed_at IS NULL AND reviewer_epoch IS NULL) OR
        (review_decision IS NOT NULL AND reviewer_admin_id IS NOT NULL AND reviewed_at IS NOT NULL AND reviewer_epoch IS NOT NULL)),
 CHECK (review_decision IS NOT 'APPROVED' OR (identity_check_method IS 'CAMPUS_AUTHORIZED_CHECK' AND identity_attestation_version IS 'synthetic-v1' AND campus_check_ref IS NOT NULL)),
 CHECK (review_decision IS NOT 'REJECTED' OR rejection_reason_code IS NOT NULL),
 CHECK (status NOT IN ('APPROVED_PROCESSING','ACTIVE','FAILED_PROCESSING') OR review_decision IS 'APPROVED'),
 CHECK (status<>'REJECTED' OR review_decision IS 'REJECTED'),
 CHECK (status<>'ACTIVE' OR activated_enrollment_id IS NOT NULL),
 CHECK (reviewer_admin_id IS NULL OR reviewer_admin_id<>subject_user_id)
);
CREATE UNIQUE INDEX uq_enrollment_live_person ON face_enrollment_submissions(person_id) WHERE status IN ('PENDING_APPROVAL','APPROVED_PROCESSING','FAILED_PROCESSING','ACTIVE');
CREATE UNIQUE INDEX uq_enrollment_live_subject ON face_enrollment_submissions(subject_user_id) WHERE status IN ('PENDING_APPROVAL','APPROVED_PROCESSING','FAILED_PROCESSING','ACTIVE');
CREATE INDEX idx_enrollment_queue ON face_enrollment_submissions(status,submitted_at,id);
CREATE TRIGGER enrollment_submission_immutable BEFORE UPDATE ON face_enrollment_submissions
WHEN NEW.id IS NOT OLD.id OR NEW.person_id IS NOT OLD.person_id OR NEW.subject_user_id IS NOT OLD.subject_user_id OR
 NEW.subject_session_id IS NOT OLD.subject_session_id OR NEW.subject_session_epoch IS NOT OLD.subject_session_epoch OR
 NEW.consent_id IS NOT OLD.consent_id OR NEW.submitted_at IS NOT OLD.submitted_at OR NEW.photo_expires_at IS NOT OLD.photo_expires_at OR NEW.captured_epoch IS NOT OLD.captured_epoch OR
 NEW.canonical_sha256 IS NOT OLD.canonical_sha256 OR
 (OLD.review_decision IS NOT NULL AND (NEW.review_decision IS NOT OLD.review_decision OR NEW.reviewer_admin_id IS NOT OLD.reviewer_admin_id OR NEW.reviewer_epoch IS NOT OLD.reviewer_epoch OR NEW.reviewed_at IS NOT OLD.reviewed_at OR NEW.identity_check_method IS NOT OLD.identity_check_method OR NEW.identity_attestation_version IS NOT OLD.identity_attestation_version OR NEW.campus_check_ref IS NOT OLD.campus_check_ref OR NEW.rejection_reason_code IS NOT OLD.rejection_reason_code)) OR
 (OLD.status IN ('REJECTED','REVOKED','EXPIRED') AND NEW.status<>OLD.status) OR
 (NEW.status<>OLD.status AND NOT (
 (OLD.status='PENDING_APPROVAL' AND NEW.status IN ('APPROVED_PROCESSING','REJECTED','REVOKED','EXPIRED')) OR
 (OLD.status='APPROVED_PROCESSING' AND NEW.status IN ('ACTIVE','FAILED_PROCESSING','REVOKED','EXPIRED')) OR
 (OLD.status='FAILED_PROCESSING' AND NEW.status IN ('APPROVED_PROCESSING','REVOKED','EXPIRED')) OR
 (OLD.status='ACTIVE' AND NEW.status='REVOKED'))) OR NEW.revision<>OLD.revision+1
BEGIN SELECT RAISE(ABORT,'immutable enrollment submission or decision'); END;

CREATE TABLE face_enrollment_photos (
 submission_id TEXT PRIMARY KEY REFERENCES face_enrollment_submissions(id) ON DELETE RESTRICT,
 key_version TEXT NOT NULL,
 nonce BLOB NOT NULL CHECK(length(nonce)=12),
 storage_state TEXT NOT NULL CHECK(storage_state IN ('STAGED','READY')),
 mime TEXT NOT NULL DEFAULT 'image/jpeg' CHECK(mime='image/jpeg'),
 width INTEGER NOT NULL CHECK(width BETWEEN 112 AND 1920),
 height INTEGER NOT NULL CHECK(height BETWEEN 112 AND 1920),
 byte_length INTEGER NOT NULL CHECK(byte_length BETWEEN 1 AND 1048576),
 canonical_sha256 TEXT NOT NULL CHECK(length(canonical_sha256)=64),
 expires_at TEXT NOT NULL,
 CHECK(width*height<=2073600),
 UNIQUE(key_version,nonce)
);
CREATE TRIGGER enrollment_photo_immutable BEFORE UPDATE ON face_enrollment_photos
WHEN NOT (OLD.storage_state='STAGED' AND NEW.storage_state='READY' AND NEW.submission_id=OLD.submission_id AND NEW.key_version=OLD.key_version AND NEW.nonce=OLD.nonce AND NEW.mime=OLD.mime AND NEW.width=OLD.width AND NEW.height=OLD.height AND NEW.byte_length=OLD.byte_length AND NEW.expires_at=OLD.expires_at AND NEW.canonical_sha256=OLD.canonical_sha256)
BEGIN SELECT RAISE(ABORT,'immutable enrollment photo'); END;
CREATE TRIGGER enrollment_photo_insert_guard BEFORE INSERT ON face_enrollment_photos
WHEN NOT EXISTS(SELECT 1 FROM face_enrollment_submissions s WHERE s.id=NEW.submission_id AND s.status='PENDING_APPROVAL' AND s.photo_expires_at=NEW.expires_at AND s.canonical_sha256=NEW.canonical_sha256)
BEGIN SELECT RAISE(ABORT,'photo submission mismatch'); END;

CREATE TABLE face_templates (
 id TEXT PRIMARY KEY,
 person_id TEXT NOT NULL UNIQUE REFERENCES people(id) ON DELETE RESTRICT,
 activation_enrollment_id TEXT NOT NULL UNIQUE REFERENCES face_enrollments(id) ON DELETE RESTRICT,
 consent_id TEXT NOT NULL REFERENCES consent_records(id) ON DELETE RESTRICT,
 submission_id TEXT NOT NULL UNIQUE REFERENCES face_enrollment_submissions(id) ON DELETE RESTRICT,
 ciphertext BLOB NOT NULL CHECK(length(ciphertext)=528),
 nonce BLOB NOT NULL CHECK(length(nonce)=12),
 aad TEXT NOT NULL,
 envelope_version INTEGER NOT NULL CHECK(envelope_version=1),
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
CREATE TRIGGER enrollment_template_immutable BEFORE UPDATE ON face_templates BEGIN SELECT RAISE(ABORT,'immutable enrollment template'); END;
CREATE TRIGGER enrollment_template_binding BEFORE INSERT ON face_templates
WHEN NOT EXISTS(SELECT 1 FROM face_enrollment_submissions s JOIN face_enrollments a ON a.id=NEW.activation_enrollment_id JOIN consent_records c ON c.id=s.consent_id WHERE s.id=NEW.submission_id AND s.person_id=NEW.person_id AND s.consent_id=NEW.consent_id AND s.status='APPROVED_PROCESSING' AND s.review_decision='APPROVED' AND a.person_id=s.person_id AND a.status='active' AND c.status='granted')
BEGIN SELECT RAISE(ABORT,'template binding mismatch'); END;

CREATE TABLE enrollment_processing_jobs (
 submission_id TEXT PRIMARY KEY REFERENCES face_enrollment_submissions(id) ON DELETE RESTRICT,
 attempt_id TEXT NOT NULL UNIQUE,
 attempts_started INTEGER NOT NULL CHECK(attempts_started BETWEEN 1 AND 3),
 expected_revision INTEGER NOT NULL,
 expected_epoch INTEGER NOT NULL,
 expected_photo_hash TEXT NOT NULL CHECK(length(expected_photo_hash)=64),
 state TEXT NOT NULL CHECK(state IN ('QUEUED','RUNNING','FINISHED')),
 lease_until TEXT,
 last_error_code TEXT
);
CREATE TABLE biometric_deletion_jobs (
 person_id TEXT PRIMARY KEY REFERENCES people(id) ON DELETE RESTRICT,
 requested_at TEXT NOT NULL,
 reason_code TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('PENDING','COMPLETE','FAILED')),
 completed_at TEXT,
 last_error_code TEXT,
 revision INTEGER NOT NULL DEFAULT 1 CHECK(revision>0)
);
CREATE TABLE enrollment_file_deletions (
 submission_id TEXT PRIMARY KEY,
 person_id TEXT NOT NULL REFERENCES people(id) ON DELETE RESTRICT
);
CREATE TRIGGER enrollment_photo_delete BEFORE DELETE ON face_enrollment_photos BEGIN
 INSERT OR IGNORE INTO enrollment_file_deletions(submission_id,person_id) SELECT OLD.submission_id,person_id FROM face_enrollment_submissions WHERE id=OLD.submission_id;
END;
CREATE TABLE enrollment_rate_buckets (
 scope TEXT NOT NULL, window_start INTEGER NOT NULL,
 count INTEGER NOT NULL CHECK(count>0), expires_at INTEGER NOT NULL,
 PRIMARY KEY(scope,window_start)
);

-- Epochs are monotonic fences: restoring a role/link/roster/session cannot revive old work.
-- These hooks do not modify auth algorithms and NEVER delete templates on role changes.
CREATE TRIGGER enrollment_user_fence AFTER UPDATE OF role,status ON users
WHEN NEW.role IS NOT OLD.role OR NEW.status IS NOT OLD.status BEGIN
 UPDATE users SET enrollment_auth_epoch=enrollment_auth_epoch+1 WHERE id=NEW.id;
 UPDATE people SET biometric_epoch=biometric_epoch+1 WHERE account_user_id=NEW.id;
END;
CREATE TRIGGER enrollment_person_fence AFTER UPDATE OF account_user_id,status,deleted_at,identity_verified_at,identity_reference ON people
WHEN NEW.account_user_id IS NOT OLD.account_user_id OR NEW.status IS NOT OLD.status OR NEW.deleted_at IS NOT OLD.deleted_at OR NEW.identity_verified_at IS NOT OLD.identity_verified_at OR NEW.identity_reference IS NOT OLD.identity_reference BEGIN
 UPDATE people SET biometric_epoch=biometric_epoch+1 WHERE id=NEW.id;
END;
CREATE TRIGGER enrollment_roster_update_fence AFTER UPDATE OF left_at,person_id,class_id ON class_memberships
WHEN NEW.left_at IS NOT OLD.left_at OR NEW.person_id IS NOT OLD.person_id OR NEW.class_id IS NOT OLD.class_id BEGIN
 UPDATE people SET biometric_epoch=biometric_epoch+1 WHERE id IN (OLD.person_id,NEW.person_id);
END;
CREATE TRIGGER enrollment_roster_delete_fence AFTER DELETE ON class_memberships BEGIN
 UPDATE people SET biometric_epoch=biometric_epoch+1 WHERE id=OLD.person_id;
END;
CREATE TRIGGER enrollment_class_fence AFTER UPDATE OF status,deleted_at ON classes
WHEN NEW.status IS NOT OLD.status OR NEW.deleted_at IS NOT OLD.deleted_at BEGIN
 UPDATE people SET biometric_epoch=biometric_epoch+1 WHERE id IN (SELECT person_id FROM class_memberships WHERE class_id=NEW.id);
END;
CREATE TRIGGER enrollment_session_fence AFTER UPDATE OF revoked_at,user_id,access_expires_at,absolute_expires_at ON session_tokens
WHEN NEW.revoked_at IS NOT OLD.revoked_at OR NEW.user_id IS NOT OLD.user_id OR NEW.access_expires_at<OLD.access_expires_at OR NEW.absolute_expires_at<OLD.absolute_expires_at BEGIN
 UPDATE session_tokens SET enrollment_epoch=enrollment_epoch+1 WHERE id=NEW.id;
 UPDATE people SET biometric_epoch=biometric_epoch+1 WHERE account_user_id=OLD.user_id;
END;
CREATE TRIGGER enrollment_session_delete_fence AFTER DELETE ON session_tokens BEGIN
 UPDATE people SET biometric_epoch=biometric_epoch+1 WHERE account_user_id=OLD.user_id;
END;
CREATE TRIGGER enrollment_consent_withdraw_fence AFTER UPDATE OF status ON consent_records
WHEN NEW.purpose='face_enrollment' AND OLD.status='granted' AND NEW.status='withdrawn' BEGIN
 UPDATE people SET biometric_epoch=biometric_epoch+1 WHERE id=NEW.person_id;
END;

CREATE TRIGGER enrollment_person_epoch_monotonic BEFORE UPDATE OF biometric_epoch ON people WHEN NEW.biometric_epoch<OLD.biometric_epoch BEGIN SELECT RAISE(ABORT,'biometric epoch cannot decrease'); END;
CREATE TRIGGER enrollment_reviewer_epoch_monotonic BEFORE UPDATE OF enrollment_auth_epoch ON users WHEN NEW.enrollment_auth_epoch<OLD.enrollment_auth_epoch BEGIN SELECT RAISE(ABORT,'reviewer epoch cannot decrease'); END;
CREATE TRIGGER enrollment_session_epoch_monotonic BEFORE UPDATE OF enrollment_epoch ON session_tokens WHEN NEW.enrollment_epoch<OLD.enrollment_epoch BEGIN SELECT RAISE(ABORT,'session epoch cannot decrease'); END;
CREATE TRIGGER enrollment_job_binding_insert BEFORE INSERT ON enrollment_processing_jobs
WHEN NOT EXISTS(SELECT 1 FROM face_enrollment_submissions s WHERE s.id=NEW.submission_id AND s.status='APPROVED_PROCESSING' AND s.revision=NEW.expected_revision AND s.captured_epoch=NEW.expected_epoch AND s.canonical_sha256=NEW.expected_photo_hash)
BEGIN SELECT RAISE(ABORT,'job binding mismatch'); END;
CREATE TRIGGER enrollment_job_binding_update BEFORE UPDATE ON enrollment_processing_jobs
WHEN NEW.submission_id IS NOT OLD.submission_id OR NEW.expected_epoch IS NOT OLD.expected_epoch OR NEW.expected_photo_hash IS NOT OLD.expected_photo_hash OR
 (NEW.attempt_id=OLD.attempt_id AND (NEW.expected_revision<>OLD.expected_revision OR NEW.attempts_started<>OLD.attempts_started)) OR
 (NEW.attempt_id<>OLD.attempt_id AND NOT (OLD.state='FINISHED' AND NEW.state='QUEUED' AND NEW.attempts_started=OLD.attempts_started+1 AND NEW.expected_revision>OLD.expected_revision))
BEGIN SELECT RAISE(ABORT,'immutable job binding'); END;

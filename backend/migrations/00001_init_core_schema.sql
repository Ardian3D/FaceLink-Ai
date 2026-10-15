-- 00001_init_core_schema.sql
-- Skema inti FaceLink AI: 13 tabel nonbiometrik (docs/DATA_MODEL.md §4.1-§4.10, §4.13-§4.15).
--
-- Tabel face_templates (§4.12) dan face_enrollment_samples (§4.11) SENGAJA tidak
-- dibuat di sini: keduanya bergantung pada rancangan manajemen kunci biometrik
-- (D-030) yang belum diimplementasikan. Membuatnya sekarang berarti menetapkan
-- bentuk penyimpanan biometrik sebelum aturan enkripsinya disetujui.
--
-- Konvensi kolom (docs/DATA_MODEL.md §3): id TEXT UUID v4, waktu ISO-8601 UTC
-- berakhiran Z, indeks berawalan idx_/uq_, batasan berawalan ck_.

-- §4.3 roles ---------------------------------------------------------------
CREATE TABLE roles (
  name        TEXT PRIMARY KEY,
  description TEXT NOT NULL,
  created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

INSERT INTO roles (name, description) VALUES
  ('operator', 'Mengoperasikan sesi absensi dan meninjau hasil'),
  ('admin',    'Mengelola pengguna, kelas, peserta, dan konfigurasi'),
  ('device',   'Terminal IoT yang menandai kehadiran atas nama sesi terbuka');

-- §4.1 users ---------------------------------------------------------------
CREATE TABLE users (
  id            TEXT PRIMARY KEY,
  email         TEXT NOT NULL,
  display_name  TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  role          TEXT NOT NULL REFERENCES roles(name) ON DELETE RESTRICT,
  status        TEXT NOT NULL DEFAULT 'active'
                  CHECK (status IN ('active','disabled')),
  created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  deleted_at    TEXT
);

CREATE UNIQUE INDEX uq_users_email ON users (email) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_role ON users (role);
CREATE INDEX idx_users_deleted_at ON users (deleted_at);

-- §4.2 operator_tokens ------------------------------------------------------
CREATE TABLE operator_tokens (
  id          TEXT PRIMARY KEY,
  user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash  TEXT NOT NULL,
  label       TEXT,
  expires_at  TEXT NOT NULL,
  revoked_at  TEXT,
  last_used_at TEXT,
  created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE UNIQUE INDEX uq_operator_tokens_hash ON operator_tokens (token_hash);
CREATE INDEX idx_operator_tokens_user ON operator_tokens (user_id);

-- §4.4 people ---------------------------------------------------------------
CREATE TABLE people (
  id            TEXT PRIMARY KEY,
  external_code TEXT NOT NULL,
  display_name  TEXT NOT NULL,
  status        TEXT NOT NULL DEFAULT 'active'
                  CHECK (status IN ('active','inactive')),
  consent_state TEXT NOT NULL DEFAULT 'unknown'
                  CHECK (consent_state IN ('unknown','granted','withdrawn','expired')),
  created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  deleted_at    TEXT
);

CREATE UNIQUE INDEX uq_people_external_code ON people (external_code) WHERE deleted_at IS NULL;
CREATE INDEX idx_people_consent_state ON people (consent_state);
CREATE INDEX idx_people_deleted_at ON people (deleted_at);

-- §4.5 classes --------------------------------------------------------------
CREATE TABLE classes (
  id          TEXT PRIMARY KEY,
  code        TEXT NOT NULL,
  title       TEXT NOT NULL,
  description TEXT,
  status      TEXT NOT NULL DEFAULT 'active'
                CHECK (status IN ('active','archived')),
  created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  deleted_at  TEXT
);

CREATE UNIQUE INDEX uq_classes_code ON classes (code) WHERE deleted_at IS NULL;
CREATE INDEX idx_classes_status ON classes (status);

-- §4.6 class_memberships ----------------------------------------------------
CREATE TABLE class_memberships (
  class_id   TEXT NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  person_id  TEXT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
  joined_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  left_at    TEXT,
  created_by TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  PRIMARY KEY (class_id, person_id)
);

CREATE INDEX idx_class_memberships_person ON class_memberships (person_id);
CREATE INDEX idx_class_memberships_active ON class_memberships (class_id) WHERE left_at IS NULL;

-- §4.13 devices -------------------------------------------------------------
CREATE TABLE devices (
  id            TEXT PRIMARY KEY,
  device_code   TEXT NOT NULL,
  label         TEXT,
  location      TEXT,
  token_hash    TEXT,
  status        TEXT NOT NULL DEFAULT 'active'
                  CHECK (status IN ('active','revoked','maintenance')),
  last_seen_at  TEXT,
  created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  revoked_at    TEXT
);

CREATE UNIQUE INDEX uq_devices_code ON devices (device_code);
CREATE INDEX idx_devices_status ON devices (status);

-- §4.7 attendance_sessions --------------------------------------------------
CREATE TABLE attendance_sessions (
  id            TEXT PRIMARY KEY,
  class_id      TEXT NOT NULL REFERENCES classes(id) ON DELETE RESTRICT,
  device_id     TEXT REFERENCES devices(id) ON DELETE SET NULL,
  status        TEXT NOT NULL DEFAULT 'open'
                  CHECK (status IN ('open','closed','cancelled')),
  opens_at      TEXT,
  closes_at     TEXT NOT NULL,
  late_after    TEXT,
  note          TEXT,
  created_by    TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  closed_at     TEXT,
  closed_by     TEXT REFERENCES users(id) ON DELETE RESTRICT,
  CONSTRAINT ck_sessions_time_order CHECK (
    (closes_at > opens_at OR opens_at IS NULL)
    AND (late_after IS NULL OR late_after <= closes_at))
);

-- Aturan domain: paling banyak satu sesi terbuka per kelas dan per perangkat.
-- Ditegakkan database, bukan hanya aplikasi (docs/DATA_MODEL.md §4.7).
CREATE UNIQUE INDEX uq_sessions_one_open_per_class
  ON attendance_sessions (class_id) WHERE status = 'open';
CREATE UNIQUE INDEX uq_sessions_one_open_per_device
  ON attendance_sessions (device_id) WHERE status = 'open' AND device_id IS NOT NULL;
CREATE INDEX idx_sessions_class_status ON attendance_sessions (class_id, status);
CREATE INDEX idx_sessions_closes_at ON attendance_sessions (closes_at);

-- §4.8 attendance_records ---------------------------------------------------
CREATE TABLE attendance_records (
  id            TEXT PRIMARY KEY,
  session_id    TEXT NOT NULL REFERENCES attendance_sessions(id) ON DELETE CASCADE,
  person_id     TEXT NOT NULL REFERENCES people(id) ON DELETE RESTRICT,
  status        TEXT NOT NULL
                  CHECK (status IN ('present','late','absent','excused')),
  source        TEXT NOT NULL
                  CHECK (source IN ('recognition','manual','import')),
  confidence    REAL,
  checked_in_at TEXT NOT NULL,
  device_id     TEXT REFERENCES devices(id) ON DELETE SET NULL,
  created_by    TEXT REFERENCES users(id) ON DELETE RESTRICT,
  reason        TEXT,
  created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  CONSTRAINT ck_attendance_manual_fields CHECK (
    source <> 'manual' OR (created_by IS NOT NULL AND reason IS NOT NULL)),
  CONSTRAINT ck_attendance_confidence_range CHECK (
    confidence IS NULL OR (confidence >= 0.0 AND confidence <= 1.0))
);

-- Pengamanan absensi ganda pada tingkat database (AGENTS.md §3).
CREATE UNIQUE INDEX uq_attendance_session_person
  ON attendance_records (session_id, person_id);
CREATE INDEX idx_attendance_person ON attendance_records (person_id, checked_in_at);
CREATE INDEX idx_attendance_session_status ON attendance_records (session_id, status);

-- §4.9 consent_records ------------------------------------------------------
CREATE TABLE consent_records (
  id            TEXT PRIMARY KEY,
  person_id     TEXT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
  purpose       TEXT NOT NULL
                  CHECK (purpose IN ('face_enrollment','attendance_photo','data_retention')),
  status        TEXT NOT NULL
                  CHECK (status IN ('granted','withdrawn')),
  granted_at    TEXT NOT NULL,
  withdrawn_at  TEXT,
  method        TEXT NOT NULL
                  CHECK (method IN ('written','digital_signature','verbal_witnessed')),
  recorded_by   TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  document_ref  TEXT,
  notes         TEXT,
  created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  CONSTRAINT ck_consent_withdraw_fields CHECK (
    (status = 'withdrawn') = (withdrawn_at IS NOT NULL))
);

CREATE UNIQUE INDEX uq_consent_active_per_purpose
  ON consent_records (person_id, purpose) WHERE status = 'granted';
CREATE INDEX idx_consent_person ON consent_records (person_id, purpose);

-- §4.10 face_enrollments ----------------------------------------------------
CREATE TABLE face_enrollments (
  id              TEXT PRIMARY KEY,
  person_id       TEXT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
  status          TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','active','revoked','failed')),
  model_name      TEXT,
  model_version   TEXT,
  model_checksum  TEXT,
  threshold       REAL,
  sample_count    INTEGER NOT NULL DEFAULT 0 CHECK (sample_count >= 0),
  enrolled_at     TEXT,
  revoked_at      TEXT,
  revoked_reason  TEXT,
  created_by      TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX idx_face_enrollments_person ON face_enrollments (person_id, status);
CREATE UNIQUE INDEX uq_face_enrollments_active
  ON face_enrollments (person_id) WHERE status = 'active';

-- §4.14 audit_events --------------------------------------------------------
CREATE TABLE audit_events (
  id            TEXT PRIMARY KEY,
  occurred_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  actor_type    TEXT NOT NULL
                  CHECK (actor_type IN ('user','device','system')),
  actor_id      TEXT,
  action        TEXT NOT NULL,
  entity_type   TEXT,
  entity_id     TEXT,
  result        TEXT NOT NULL
                  CHECK (result IN ('success','failure')),
  reason        TEXT,
  request_id    TEXT,
  metadata_json TEXT
);

CREATE INDEX idx_audit_occurred_at ON audit_events (occurred_at);
CREATE INDEX idx_audit_actor ON audit_events (actor_type, actor_id);
CREATE INDEX idx_audit_entity ON audit_events (entity_type, entity_id);
CREATE INDEX idx_audit_action ON audit_events (action);

-- §4.15 idempotency_records -------------------------------------------------
CREATE TABLE idempotency_records (
  scope          TEXT NOT NULL,
  key            TEXT NOT NULL,
  request_hash   TEXT NOT NULL,
  response_code  INTEGER NOT NULL,
  response_body  TEXT NOT NULL,
  created_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  expires_at     TEXT NOT NULL,
  PRIMARY KEY (scope, key)
);

CREATE INDEX idx_idempotency_expires_at ON idempotency_records (expires_at);

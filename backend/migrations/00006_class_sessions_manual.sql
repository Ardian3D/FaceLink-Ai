-- FL-CORE02: sesi dan absensi manual NONBIOMETRIK.
-- Ikuti pola 00002: ganti kolom status tanpa DROP tabel induk. Dengan foreign
-- key tetap aktif, attendance_records dan seluruh data/kolom lama dipertahankan.
ALTER TABLE attendance_sessions RENAME COLUMN status TO status_legacy;
ALTER TABLE attendance_sessions ADD COLUMN status TEXT NOT NULL DEFAULT 'draft'
  CHECK (status IN ('draft','open','closed','cancelled'));
UPDATE attendance_sessions SET status = status_legacy;

DROP INDEX uq_sessions_one_open_per_class;
DROP INDEX uq_sessions_one_open_per_device;
DROP INDEX idx_sessions_class_status;
ALTER TABLE attendance_sessions DROP COLUMN status_legacy;

CREATE UNIQUE INDEX uq_sessions_one_open_per_class
  ON attendance_sessions (class_id) WHERE status = 'open';
CREATE UNIQUE INDEX uq_sessions_one_open_per_device
  ON attendance_sessions (device_id) WHERE status = 'open' AND device_id IS NOT NULL;
CREATE INDEX idx_sessions_class_status ON attendance_sessions (class_id, status);
CREATE INDEX idx_sessions_device ON attendance_sessions (device_id);
CREATE INDEX idx_sessions_created ON attendance_sessions (created_at, id);

ALTER TABLE attendance_sessions ADD COLUMN title TEXT
  CHECK (title IS NULL OR length(title) BETWEEN 1 AND 120);
ALTER TABLE attendance_sessions ADD COLUMN started_at TEXT;
ALTER TABLE attendance_sessions ADD COLUMN close_reason TEXT
  CHECK (close_reason IS NULL OR length(close_reason) <= 200);
-- opens_at adalah bukti pembukaan historis; jangan menebak dari created_at.
UPDATE attendance_sessions SET started_at = opens_at WHERE opens_at IS NOT NULL;

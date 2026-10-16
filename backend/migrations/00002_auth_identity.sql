-- 00002_auth_identity.sql
-- Autentikasi Google dan tiga role manusia (FL-011, keputusan D-031..D-037).
--
-- Prinsip yang dipegang berkas ini:
--   1. Migrasi 00001 TIDAK diubah.
--   2. Data lama dipertahankan: baris domain tetap ada, kolom yang digantikan
--      dipetakan (peran `operator` menjadi `dosen`, status `disabled` menjadi
--      `suspended`, email dinormalkan), dan baris sesi lama — yang memakai skema
--      token kata sandi dan tidak dapat dipetakan ke token opak — dibuang dengan
--      peristiwa yang dicatat di `audit_events`, bukan dipalsukan.
--   3. Tidak ada tabel biometrik yang dibuat di sini. D-030 tetap tertunda.
--   4. Tidak ada kredensial, token, atau kata sandi yang ditulis di berkas ini.
--
-- Catatan teknis penting: tabel `users` **tidak dibangun ulang**. SQLite tidak
-- dapat mengubah CHECK constraint lewat ALTER TABLE, sehingga kolom `status`
-- lama dipindahkan ke kolom sementara lalu dibuang — memakai ALTER TABLE
-- ADD/DROP/RENAME COLUMN yang didukung SQLite 3.35+. Alasannya bukan sekadar
-- kenyamanan: pola "buat tabel baru -> salin -> jatuhkan tabel lama" menuntut
-- `PRAGMA foreign_keys=OFF` di luar transaksi, sedangkan runner migrasi selalu
-- membungkus satu berkas dalam satu transaksi (`applyOne`). Menjatuhkan tabel
-- induk di dalam transaksi dengan foreign key aktif akan menaikkan penghitung
-- pelanggaran yang tidak pernah turun, dan COMMIT gagal. Pendekatan ALTER TABLE
-- membuat tabel anak (sesi, kehadiran, konsen, penugasan) tidak pernah kehilangan
-- induknya.
--
-- Batas peran `users.role` ditegakkan oleh foreign key ke tabel acuan `roles`
-- yang — setelah migrasi ini — hanya berisi tiga role manusia; CHECK tambahan
-- pada kolom lama tidak dapat ditambahkan tanpa membangun ulang tabel.

-- ---------------------------------------------------------------------------
-- 1. Tabel acuan peran: tiga role manusia. `device` bukan role manusia.
-- ---------------------------------------------------------------------------
INSERT INTO roles (name, description) VALUES
  ('dosen',     'Pengajar yang hanya mengelola kelas yang ditugaskan kepadanya'),
  ('mahasiswa', 'Peserta yang hanya mengakses data miliknya sendiri')
ON CONFLICT (name) DO NOTHING;

UPDATE roles SET description = 'Administrator sistem: akun, role, kelas, perangkat, audit'
WHERE name = 'admin';

-- ---------------------------------------------------------------------------
-- 2. Tabel `users`: tanpa kata sandi, dengan lifecycle undangan.
-- ---------------------------------------------------------------------------
-- Status lama ('active'/'disabled') dipindahkan ke kolom sementara agar CHECK
-- baru dapat berlaku tanpa membangun ulang tabel.
ALTER TABLE users RENAME COLUMN status TO status_legacy;

ALTER TABLE users ADD COLUMN status TEXT NOT NULL DEFAULT 'invited'
  CHECK (status IN ('invited','active','suspended'));

UPDATE users SET status = CASE WHEN status_legacy = 'disabled' THEN 'suspended' ELSE 'active' END;

-- Pemetaan peran: `operator` menjadi `dosen` (D-031). Peran lain yang bukan
-- manusia (mis. `device` dari skema lama) diturunkan ke hak paling kecil, bukan
-- dinaikkan; akun seperti itu tetap tidak dapat masuk karena login menuntut
-- identitas Google yang tertaut.
UPDATE users SET role = 'dosen'     WHERE role = 'operator';
UPDATE users SET role = 'mahasiswa' WHERE role NOT IN ('admin','dosen','mahasiswa');

-- Email disimpan dalam huruf kecil tanpa spasi tepi supaya pencocokan undangan
-- pada login pertama tidak bergantung pada penulisan (D-032).
UPDATE users SET email = lower(trim(email));

ALTER TABLE users ADD COLUMN created_by TEXT REFERENCES users(id) ON DELETE RESTRICT;

-- Waktu login berhasil terakhir (docs/DATA_MODEL.md §4.1, docs/API_CONTRACT.md
-- §5.2). Baris lama tetap NULL: skema sebelumnya tidak mencatatnya.
ALTER TABLE users ADD COLUMN last_login_at TEXT;

-- Indeks lama yang mengacu kolom yang akan dibuang harus dilepas lebih dahulu.
DROP INDEX IF EXISTS uq_users_email;
DROP INDEX IF EXISTS idx_users_deleted_at;

-- Kata sandi internal dan penghapusan lunak akun tidak lagi dipakai: identitas
-- pengguna sepenuhnya lewat Google, dan penonaktifan memakai status `suspended`.
ALTER TABLE users DROP COLUMN status_legacy;
ALTER TABLE users DROP COLUMN password_hash;
ALTER TABLE users DROP COLUMN deleted_at;

CREATE UNIQUE INDEX uq_users_email ON users (email);
CREATE INDEX idx_users_status ON users (status);

-- `operator` dan `device` bukan role manusia lagi. Seluruh baris `users` sudah
-- dipetakan di atas, sehingga penghapusan baris acuan ini tidak memutus relasi.
DELETE FROM roles WHERE name = 'operator';
DELETE FROM roles WHERE name = 'device'
  AND NOT EXISTS (SELECT 1 FROM users WHERE role = 'device');

-- ---------------------------------------------------------------------------
-- 3. Tabel `session_tokens`: sesi opak dengan rotasi penyegar.
-- ---------------------------------------------------------------------------
CREATE TABLE session_tokens (
  id                  TEXT PRIMARY KEY,
  user_id             TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  access_hash         TEXT NOT NULL,
  access_expires_at   TEXT NOT NULL,
  refresh_hash        TEXT NOT NULL,
  refresh_expires_at  TEXT NOT NULL,
  absolute_expires_at TEXT NOT NULL,
  prev_refresh_hash   TEXT,
  revoked_at          TEXT,
  revoked_reason      TEXT,
  last_seen_at        TEXT,
  user_agent_ringkas  TEXT,
  created_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  CONSTRAINT ck_session_tokens_absolute CHECK (refresh_expires_at <= absolute_expires_at),
  CONSTRAINT ck_session_tokens_access   CHECK (access_expires_at  <= absolute_expires_at)
);

CREATE UNIQUE INDEX uq_session_tokens_access   ON session_tokens (access_hash);
CREATE UNIQUE INDEX uq_session_tokens_refresh  ON session_tokens (refresh_hash);
CREATE INDEX idx_session_tokens_user           ON session_tokens (user_id);
CREATE INDEX idx_session_tokens_absolute       ON session_tokens (absolute_expires_at);
CREATE INDEX idx_session_tokens_refresh_exp    ON session_tokens (refresh_expires_at);

-- Sesi lama memakai skema token kata sandi yang tidak lagi berlaku dan tidak
-- dapat dipetakan ke hash akses/penyegar di atas. Barisnya dibuang dan
-- peristiwanya dicatat; seluruh sesi lama otomatis tidak valid.
INSERT INTO audit_events (id, actor_type, actor_id, action, entity_type, result, reason, metadata_json)
SELECT
  'migrasi-00002-reset-sesi-lama',
  'system', NULL, 'auth.session.reset_legacy', 'session_token', 'success',
  'Migrasi 00002: sesi token lama dibuang karena skema token opak menggantikan token kata sandi',
  '{"migration":2}'
WHERE EXISTS (SELECT 1 FROM operator_tokens);

DROP TABLE operator_tokens;

-- ---------------------------------------------------------------------------
-- 4. Tabel `user_identities`: penautan identitas Google memakai klaim `sub`.
-- ---------------------------------------------------------------------------
CREATE TABLE user_identities (
  id                TEXT PRIMARY KEY,
  user_id           TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider          TEXT NOT NULL CHECK (provider = 'google'),
  subject           TEXT NOT NULL,
  email_at_provider TEXT,
  linked_at         TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

-- Penjaga terakhir penautan: satu identitas hanya boleh menautkan satu akun, dan
-- satu akun hanya boleh memiliki satu identitas per penyedia. Inilah yang membuat
-- dua permintaan login bersamaan tidak dapat menghasilkan penautan ganda.
CREATE UNIQUE INDEX uq_user_identities_provider_subject ON user_identities (provider, subject);
CREATE UNIQUE INDEX uq_user_identities_user_provider    ON user_identities (user_id, provider);
CREATE INDEX idx_user_identities_user ON user_identities (user_id);

-- ---------------------------------------------------------------------------
-- 5. Tabel `auth_nonces`: nonce OIDC sekali pakai (D-033).
-- ---------------------------------------------------------------------------
CREATE TABLE auth_nonces (
  nonce_hash TEXT PRIMARY KEY,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  expires_at TEXT NOT NULL
);

CREATE INDEX idx_auth_nonces_expires ON auth_nonces (expires_at);

-- ---------------------------------------------------------------------------
-- 6. Tabel `class_lecturers`: dasar cakupan akses dosen.
-- ---------------------------------------------------------------------------
CREATE TABLE class_lecturers (
  class_id    TEXT NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  user_id     TEXT NOT NULL REFERENCES users(id)   ON DELETE RESTRICT,
  assigned_by TEXT NOT NULL REFERENCES users(id)   ON DELETE RESTRICT,
  assigned_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  PRIMARY KEY (class_id, user_id)
);

CREATE INDEX idx_class_lecturers_user ON class_lecturers (user_id);

-- ---------------------------------------------------------------------------
-- 7. Pengaitan akun mahasiswa dengan baris peserta (people).
-- ---------------------------------------------------------------------------
ALTER TABLE people ADD COLUMN account_user_id TEXT REFERENCES users(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX uq_people_account_user ON people (account_user_id)
  WHERE account_user_id IS NOT NULL;

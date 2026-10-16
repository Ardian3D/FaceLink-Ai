-- 00003_auth_session_history.sql
-- Riwayat rotasi token penyegar (FL-011, D-033, kontrak API §5.1).
--
-- Alasan keberadaan tabel ini: `session_tokens.prev_refresh_hash` hanya
-- menautkan hash penyegar satu generasi ke belakang. Setelah penyegaran kedua,
-- hash generasi pertama tidak lagi tersimpan di mana pun, sehingga pemakaian
-- ulangnya tidak dapat dibedakan dari token asing: sesi tidak dicabut dan tidak
-- ada peristiwa `auth.session.reuse_detected`. Kontrak §5.1 menuntut pemakaian
-- ulang penyegar yang sudah dirotasi mencabut seluruh sesi pengguna itu dan
-- menulis audit; tabel ini menyediakan jejaknya.
--
-- Prinsip yang dipegang berkas ini:
--   1. Migrasi 00001 dan 00002 TIDAK diubah.
--   2. Data lama dipertahankan; tabel baru dimulai kosong karena jejak generasi
--      sebelumnya memang tidak pernah disimpan.
--   3. Yang disimpan hanya hash SHA-256 penyegar — bentuk yang sama dengan kolom
--      `session_tokens.refresh_hash`. Token mentah tidak pernah menyentuh
--      database (D-033).
--   4. Tidak ada tabel biometrik dan tidak ada perubahan D-030.

-- ---------------------------------------------------------------------------
-- 1. Tabel `session_token_history`: hash penyegar yang sudah digantikan rotasi.
-- ---------------------------------------------------------------------------
-- Satu hash hanya boleh menunjuk satu sesi; `refresh_hash` dijadikan kunci
-- utama karena hash itu unik secara alami (256 bit acak) dan pencarian pada
-- jalur penyegaran harus selalu lewat indeks, bukan pemindaian tabel: endpoint
-- `/auth/refresh` bersifat publik, sehingga kueri di sana berjalan untuk setiap
-- token asing yang dikirim penyerang.
CREATE TABLE session_token_history (
  refresh_hash  TEXT PRIMARY KEY,
  session_id    TEXT NOT NULL REFERENCES session_tokens(id) ON DELETE CASCADE,
  superseded_at TEXT NOT NULL
);

CREATE INDEX idx_session_token_history_session ON session_token_history (session_id);

-- ---------------------------------------------------------------------------
-- 2. Indeks yang hilang pada `session_tokens`.
-- ---------------------------------------------------------------------------
-- `prev_refresh_hash` disaring pada setiap penyegaran token yang sudah diputar.
-- Tanpa indeks, kueri itu memindai seluruh tabel sambil memegang kunci tulis
-- (`BEGIN IMMEDIATE`), sehingga permintaan `/auth/refresh` tanpa autentikasi
-- dapat memperlambat seluruh penulis lain.
CREATE INDEX IF NOT EXISTS idx_session_tokens_prev_refresh ON session_tokens (prev_refresh_hash);

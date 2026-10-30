-- FL-CORE01: fondasi roster dan identitas NONBIOMETRIK.
--
-- Migrasi ini tidak membuat face_templates, tidak menyimpan foto, tidak
-- menyentuh consent_records, dan tidak mengubah tabel akun/sesi. Tujuannya
-- hanya melengkapi data peserta yang sudah ada pada 00001/00002.
--
-- `external_code` adalah NIM/kode kampus yang dipakai sebagai identitas roster.
-- Indeks unik parsial 00001 hanya berlaku untuk baris yang belum dihapus,
-- sedangkan docs/DATA_MODEL.md §4.4 menetapkan kode tidak boleh dipakai ulang
-- agar audit dan riwayat kehadiran lama tetap bermakna. Indeks global di bawah
-- menegakkan aturan dokumen itu; migrasi gagal bila database sudah memuat
-- duplikat, bukan memilih salah satu baris secara diam-diam.
ALTER TABLE people ADD COLUMN note TEXT CHECK (note IS NULL OR length(note) <= 200);
ALTER TABLE people ADD COLUMN identity_reference TEXT
  CHECK (identity_reference IS NULL OR
         (length(trim(identity_reference)) BETWEEN 1 AND 120));
-- Baris lama tetap tidak terverifikasi; tidak ada backfill berdasarkan email.
ALTER TABLE people ADD COLUMN identity_verified_at TEXT
  CHECK (identity_verified_at IS NULL OR
         (length(trim(identity_verified_at)) > 0 AND
          identity_reference IS NOT NULL AND length(trim(identity_reference)) > 0));

CREATE UNIQUE INDEX uq_people_external_code_all ON people (external_code);

-- NIM tidak dapat dipindahkan, termasuk sesudah peserta dihapus lunak.
CREATE TRIGGER people_external_code_immutable
BEFORE UPDATE OF external_code ON people
WHEN NEW.external_code IS NOT OLD.external_code
BEGIN
  SELECT RAISE(ABORT, 'people.external_code is immutable');
END;

# Rancangan Model Data dan Aturan Integritas

**Status:** Usulan v0.2; tabel dan migrasi database **belum dibuat**. Tidak ada data biometrik peserta nyata yang boleh masuk fixture publik atau repository.

## 1. Entitas inti

Nama tabel/kolom ditulis dalam bahasa teknis agar nanti mudah digunakan konsisten antara backend dan database.

| Entitas | Atribut inti yang diusulkan | Aturan penting |
|---|---|---|
| `users` | `id`, `email`, `password_hash`, `role`, `is_active`, timestamps | Email dinormalisasi dan unik; awalnya hanya `operator` |
| `people` | `id`, `external_code`, `display_name`, `status` | Kode peserta unik; gunakan kode demo, bukan NIK |
| `classes` | `id`, `title`, `code` | Kode kelas unik; hanya satu kelas wajib pada demo |
| `class_memberships` | `class_id`, `person_id`, `created_at` | Unik pasangan kelas-peserta |
| `consent_records` | `id`, `person_id`, `purpose`, `version`, `granted_at`, `withdrawn_at`, `capture_method` | Izin aktif wajib untuk enrollment dan recognition |
| `face_templates` | `id`, `person_id`, `model_id`, `model_version`, `embedding_ciphertext`, `nonce`, timestamps | Template terenkripsi, model jelas; beberapa template per orang jika disetujui |
| `attendance_sessions` | `id`, `class_id`, `started_at`, `closes_at`, `late_after`, `status`, `device_id` | Status `draft/open/closed`; perangkat memiliki maksimal satu sesi aktif |
| `attendance_records` | `id`, `session_id`, `person_id`, `checked_in_at`, `status`, `source`, `created_by`, timestamps | **UNIQUE(session_id, person_id)** |
| `devices` | `id`, `display_name`, `token_hash`, `revoked_at`, `last_seen_at` | Token disimpan dalam bentuk hash atau mekanisme aman sejenis |
| `audit_events` | `id`, `actor_type`, `actor_id`, `action`, `entity_type`, `entity_id`, `time`, `reason`, `safe_metadata` | Tambah-saja; tanpa JPEG atau embedding |

## 2. Konsep turunan

- **Hadir (`present`):** peserta tercatat valid sebelum batas keterlambatan.
- **Terlambat (`late`):** peserta check-in setelah ambang yang telah disetujui pada sesi.
- **Belum hadir / absen (`absent`):** anggota kelas yang tidak memiliki catatan saat sesi telah ditutup; **bukan** keputusan yang dibuat AI ketika gagal mengenali wajah.
- **Sudah absen (`already_checked_in`):** hasil respons permintaan ulang; bukan baris kehadiran baru.
- **Wajah asing / buram / ambigu:** hasil proses sementara; bukan identitas peserta dan tidak perlu disimpan sebagai foto orang tak dikenal.

## 3. Aturan integritas yang wajib dijamin

1. Database menjadi sumber kebenaran untuk keunikan `(session_id, person_id)`. Pemeriksaan “sudah ada?” oleh aplikasi saja **tidak cukup** menghadapi request bersamaan.
2. Check-in dengan sumber `face` memerlukan: persetujuan aktif, kelas cocok, sesi terbuka, perangkat valid, kualitas memadai, nilai/margin pencocokan terkalibrasi, dan tidak ada duplikasi.
3. Check-in atau koreksi manual memerlukan autentikasi operator, alasan yang tidak kosong, serta peristiwa audit dengan waktu dan aktor.
4. Sesudah sesi ditutup, kiriman kamera tidak boleh menambah catatan. Koreksi oleh operator harus menjadi operasi terpisah dan diaudit.
5. Jika izin biometrik dicabut, template dihapus dari penyimpanan dan **seluruh indeks/cache aktif**, bukan hanya ditandai pada database.
6. Catatan kehadiran nonbiometrik memiliki kebijakan retensi tersendiri; jangan otomatis menyimpan riwayat selamanya. Untuk data demo, utamakan penghapusan di akhir kegiatan.
7. Backend menyimpan waktu dalam UTC; aplikasi mengubah tampilan ke `Asia/Jakarta` atau zona yang diatur.
8. Model dan versi template tidak boleh dicampur secara diam-diam setelah pergantian model AI.

## 4. Indeks yang disarankan

- `people(external_code)` unik.
- `classes(code)` unik.
- `class_memberships(class_id, person_id)` unik.
- `attendance_records(session_id, person_id)` unik.
- `attendance_records(session_id, checked_in_at)` untuk urutan daftar hadir.
- `face_templates(person_id, model_version)` untuk pengelolaan model dan penghapusan.
- `audit_events(entity_type, entity_id, time)` untuk penelusuran koreksi.

## 5. Aturan migrasi database

- Setiap perubahan skema mempunyai nomor versi, langkah penerapan, dan pengujian migrasi.
- Sediakan rollback bila mungkin; jika migrasi destruktif, tandai ketidakmampuannya dibatalkan dan minta izin.
- Gunakan seed data sintetis; jangan simpan foto wajah atau vektor template sungguhan dalam SQL contoh.
- Tidak boleh menghapus data produksi/nonuji tanpa backup dan izin operator.
- Skema dan `API_CONTRACT.md` harus diperbarui bersama untuk perubahan yang memengaruhi aplikasi.

## 6. Catatan pengembangan organisasi di masa depan

Penamaan `people` dan `classes` sekarang cukup untuk demo. Nantinya bisa ditambah organisasi dan unit kerja melalui migrasi yang dipikirkan secara sadar. **Jangan mengimplementasikan multi-tenant sekarang** hanya demi kemungkinan penggunaan di kantor.

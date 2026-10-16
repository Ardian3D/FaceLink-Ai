# Daftar Pekerjaan MVP FaceLink AI

Daftar ini **usulan pembagian tugas**, bukan GitHub Issues yang telah dibuat. Kerjakan per tugas dan validasi syaratnya. Tidak ada tugas cloud sebelum MVP lokal dinyatakan siap dan pemilik memintanya.

| Tugas | Hasil yang diinginkan | Ketergantungan | Bukti penerimaan |
|---|---|---|---|
| FL-001 | Rapikan repo, dokumen, `.gitignore`, pemeriksaan rahasia | Review pemilik | Tidak ada rahasia/data biometrik di Git |
| FL-002 | Putuskan stack Go/Python dan PostgreSQL/SQLite | FL-001 | Status D-010/D-011 diubah dengan persetujuan |
| FL-003 | Tetapkan kontrak API, model data, dan stack final MVP | FL-002 | `API_CONTRACT.md` v1.0 dan `DATA_MODEL.md` (15 tabel) selesai; D-010/D-012/D-020–D-023/D-025–D-028 DISETUJUI, D-019 DITOLAK, D-011 DITUNDA; masih perlu review pemilik sebelum di-commit (**dokumen saja, bukan izin implementasi**) |
| FL-010 | Tetapkan peran, izin, dan matriks otorisasi sebelum kode backend | FL-002 | Peran operator vs perangkat tertulis dan disetujui; menjadi acuan FL-013 |
| FL-010U | Selaraskan dokumentasi dengan arsitektur **tiga role** (`admin`, `dosen`, `mahasiswa`) dan **Google Sign-In**, serta tegaskan `device` sebagai autentikasi mesin terpisah | FL-010 | **Rancangan Google auth + tiga role** tertulis di `docs/AUTHORIZATION.md` (sumber tunggal matriks peran/izin), `docs/API_CONTRACT.md`, `docs/DATA_MODEL.md`, `docs/UX_FLOWS.md`, `docs/SECURITY_PRIVACY.md`, `docs/TEST_PLAN.md`, `docs/ARCHITECTURE.md`, `docs/OPEN_QUESTIONS.md`, `docs/ROADMAP.md`, `docs/BACKLOG.md`, dan `README.md`; **dokumen diselaraskan, tanpa kode** — keluaran peran pada FL-010 direvisi (role `operator` digantikan `dosen`, akun `mahasiswa` kini ada). FL-010U **bukan** izin implementasi; seluruh skenario runtime berstatus BELUM TERVERIFIKASI |
| FL-011 | Awal backend dan endpoint kesehatan | FL-002 | Server lokal berjalan; tes kesehatan lulus |
| FL-012 | Skema dan migrasi operator, orang, kelas, consent | FL-011 | Migrasi diuji; batas unik berfungsi |
| FL-013 | Login **Google Sign-In**, autentikasi, otorisasi **tiga role** (`admin`, `dosen`, `mahasiswa`), dan sesi berbasis `session_tokens` | FL-012 | Migrasi **00002+** (tabel `user_identities`, `session_tokens` pengganti `operator_tokens`, `class_lecturers`, kolom `people.account_user_id`, penyesuaian `users`/`roles`) diterapkan tanpa mengubah `00001_init_core_schema.sql`; verifikasi ID token Google (JWKS/RS256/`iss`/`aud`/`nonce`) berjalan; akses tanpa izin ditolak pada tes; rancangan acuan: `docs/AUTHORIZATION.md` |
| FL-014 | Kelola sesi, roster, keterlambatan | FL-012 | Sesi hanya dapat menerima input saat terbuka |
| FL-015 | Absensi manual dan anti-duplikasi | FL-013/014 | Check-in bersamaan hanya menghasilkan satu catatan |
| FL-016 | CSV dan riwayat koreksi | FL-015 | CSV tidak berisi biometrik, alasan koreksi tercatat |
| FL-021 | Model AI lokal, versi/checksum/lisensi | FL-002 | Memuat model offline dan mencatat versinya |
| FL-022 | Pemeriksaan kualitas dan tepat satu wajah | FL-021 | Tanpa wajah, lebih dari satu, blur ditolak |
| FL-023 | Enrollment berizin dan penghapusan template | FL-012/022 | Setelah izin dicabut, template tidak ada di cache |
| FL-024 | Matching dan kasus ambigu | FL-023 | Wajah asing/ambigu tidak pernah check-in |
| FL-031 | Dasar React Native + Expo | FL-002 | Aplikasi tampil di Android dan lolos typecheck |
| FL-032 | Login dan sesi di aplikasi | FL-031/014 | Kelola sesi dengan API bekerja |
| FL-033 | Peserta, enrollment, dan laporan | FL-031/016/023 | Alur izin dan fallback manual dapat dijalankan |
| FL-041 | Adapter webcam dan integrasi lokal | FL-024/032 | Alur kamera→AI→catatan→UI dengan tes negatif |
| FL-042 | Status koneksi dan pemulihan | FL-041 | Gangguan jaringan/mesin AI tampil tanpa check-in palsu |
| FL-051 | Tetapkan board ESP32-S3 CAM dan daftar belanja | FL-042 | Harga total memenuhi anggaran dan kompatibilitas diperiksa |
| FL-052 | Firmware kamera, autentikasi perangkat dan LED | FL-051 | Absen melalui kamera IoT memakai API yang sama |
| FL-061 | Pengujian lengkap termasuk kacamata dan latensi | FL-052 | Laporan hasil terukur dan keterbatasan tersedia |
| FL-062 | APK Android dan panduan presentasi | FL-061 | APK terpasang dan demo dapat diulang |

Aturan: agent harus menunjukkan perubahan kode serta hasil tes nyata sebelum menandai tugas selesai.

## Catatan FL-010U (16 Oktober 2026)

Baris **FL-010U** dan revisi baris **FL-013** berasal dari **FL-010U (16 Oktober 2026)**, cabang `docs/fl-010u-google-three-roles` dari `origin/main` (`804c2f7`). Perubahan ini **dokumentasi saja**: tidak ada kode Go/TypeScript/SQL dan tidak ada berkas migrasi yang ditulis. Nomor tugas lain tidak diubah.

Revisi FL-013 menggantikan cakupan lamanya ("Login, autentikasi, otorisasi") dengan **Google Sign-In, tiga role `admin`/`dosen`/`mahasiswa`, dan sesi `session_tokens`**, serta menambahkan **migrasi 00002+** sebagai bagian penerimaannya. Rancangan acuan matriks peran/izin adalah `docs/AUTHORIZATION.md`. Sampai tugas implementasi dijalankan dan hasil tesnya nyata, seluruh skenario runtime tiga role dan Google Sign-In tetap berstatus **BELUM TERVERIFIKASI**.

# Daftar Pekerjaan MVP FaceLink AI

Daftar ini **usulan pembagian tugas**, bukan GitHub Issues yang telah dibuat. Kerjakan per tugas dan validasi syaratnya. Tidak ada tugas cloud sebelum MVP lokal dinyatakan siap dan pemilik memintanya.

| Tugas | Hasil yang diinginkan | Ketergantungan | Bukti penerimaan |
|---|---|---|---|
| FL-001 | Rapikan repo, dokumen, `.gitignore`, pemeriksaan rahasia | Review pemilik | Tidak ada rahasia/data biometrik di Git |
| FL-002 | Putuskan stack Go/Python dan PostgreSQL/SQLite | FL-001 | Status D-010/D-011 diubah dengan persetujuan |
| FL-003 | Tetapkan kontrak API, model data, dan stack final MVP | FL-002 | `API_CONTRACT.md` v1.0 dan `DATA_MODEL.md` (15 tabel) selesai; D-010/D-012/D-020–D-023/D-025–D-028 DISETUJUI, D-019 DITOLAK, D-011 DITUNDA; masih perlu review pemilik sebelum di-commit (**dokumen saja, bukan izin implementasi**) |
| FL-010 | Tetapkan peran, izin, dan matriks otorisasi sebelum kode backend | FL-002 | Peran operator vs perangkat tertulis dan disetujui; menjadi acuan FL-013 |
| FL-011 | Awal backend dan endpoint kesehatan | FL-002 | Server lokal berjalan; tes kesehatan lulus |
| FL-012 | Skema dan migrasi operator, orang, kelas, consent | FL-011 | Migrasi diuji; batas unik berfungsi |
| FL-013 | Login, autentikasi, otorisasi | FL-012 | Akses tanpa izin ditolak pada tes |
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

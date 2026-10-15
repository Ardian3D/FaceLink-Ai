# Pertanyaan Terbuka Sebelum Implementasi

Belum semua rancangan merupakan keputusan final. Pertanyaan berikut dijawab saat fase terkait akan dikerjakan; pertanyaan fase hardware **tidak** menghambat review dokumen tahap awal.

| ID | Pertanyaan | Saran awal (belum disahkan) | Dibutuhkan |
|---|---|---|---|
| Q-01 | Backend Go+Python atau Python saja? | **DIPUTUSKAN pada FL-003: Go + `chi` sebagai backend utama, Python sebagai mesin AI terpisah (D-010 DISETUJUI).** D-019 (Python + FastAPI tunggal) DITOLAK; FastAPI hanya boleh dipakai sebagai transport internal Python bila diperlukan | Selesai |
| Q-02 | PostgreSQL lokal atau SQLite? | **DIPUTUSKAN pada FL-003: SQLite mode WAL lebih dahulu (D-020 DISETUJUI)**, dengan skema dan migrasi tetap portabel ke PostgreSQL. D-011 DITUNDA sampai ada kebutuhan nyata | Selesai (tinjauan ulang bila beban berubah) |
| Q-03 | Akun operator saja atau juga akun peserta? | Operator wajib; akun peserta prioritas tambahan | Fase 3 |
| Q-04 | Siapa peserta uji dan berapa orang? | Sekitar 5–10 sukarelawan dewasa dengan persetujuan | Fase 2 |
| Q-05 | Bagaimana formulir persetujuan dan tenggat penghapusan biometrik? | Template dihapus jika izin dicabut dan setelah demo selesai | Fase 2 |
| Q-06 | Aturan terlambat dan koreksi manual? | Waktu toleransi bisa diatur per sesi, alasan koreksi wajib | Fase 1 |
| Q-07 | Model pengenalan wajah dan pemilihan ambang kecocokan? | YuNet + SFace, kalibrasi melalui data uji yang sah | Fase 2 |
| Q-08 | Model ESP32-S3 CAM yang persis serta pemetaan GPIO? | Tentukan sesudah integrasi webcam stabil | Fase 5 |
| Q-09 | Hak cipta kontribusi tim dan pilihan lisensi? | Belum menambahkan LICENSE sebelum disepakati | Fase 0 |
| Q-10 | URL repository GitHub, hak akses anggota, aturan branch? | Verifikasi repository sungguhan terlebih dahulu | Fase 0 |
| Q-11 | Spesifikasi laptop Windows, webcam, HP Android untuk uji? | Sesuaikan perangkat yang sudah dimiliki | Fase 1 |
| Q-12 | Apakah `deepseek-v4.1-flash` mendukung tool OmO tanpa biaya? | Uji dengan tugas nonrahasia; nonaktifkan fallback berbayar | Fase 0–1 |
| Q-13 | Kode peserta demo memakai NIM asli atau kode fiktif? | Kode sintetis untuk demo/publikasi | Fase 1 |
| Q-14 | Pembaruan realtime dengan polling atau WebSocket? | **DIPUTUSKAN pada FL-003: polling interval dengan `ETag` lebih dahulu (D-021 DISETUJUI)**; WebSocket hanya setelah kebutuhan pembaruan seketika terbukti | Fase 4 |
| Q-15 | Branding aplikasi dan nama paket Android? | FaceLink AI, dipastikan ketika app disiapkan | Fase 3 |
| Q-16 | Bagaimana kunci enkripsi template biometrik dikelola (lokasi, rotasi, pemulihan)? | **Baru dari FL-002.** Bentuk penyimpanan sudah final (AES-256-GCM per baris, AAD mengikat `person_id`+`model_id`+`model_version`+`key_version`; D-022 DISETUJUI). **FL-004 mengajukan jawabannya sebagai D-030 (USULAN):** kunci di luar database dan di luar Git, dibungkus DPAPI cakupan pengguna di `%LOCALAPPDATA%\FaceLink\keys\`, akses lewat `KeyProvider` Go, `key_version` pertama `'1'`, rotasi disiapkan tetapi belum aktif, tanpa kunci pemulihan otomatis, dan periode pemulihan 30 hari. **Belum disetujui — Q-16 tetap TERBUKA dan tetap menghambat implementasi `face_templates`.** | Fase 2 (menunggu persetujuan pemilik) |
| Q-17 | Apakah deteksi wajah dijalankan di ESP32-S3 untuk menghemat bandwidth, atau gambar dikirim utuh ke backend? | **Baru dari FL-002.** Usulan: kirim utuh dulu; deteksi di perangkat dievaluasi belakangan | Fase 5 |
| Q-18 | Batas laju dan batas ukuran unggahan gambar dari perangkat kamera? | **Baru dari FL-002.** Belum ada angka; harus diukur, bukan dikira-kira | Fase 1 |

Setelah diputuskan, buat/perbarui entri di `DECISIONS.md` dan sesuaikan `PROJECT_STATE.md`.

## Catatan FL-004 (15 Oktober 2026)

FL-004 **mengajukan jawaban** untuk Q-16 melalui **D-030 berstatus USULAN**: kunci AES-256-GCM disimpan di luar database dan di luar Git, dibungkus Windows DPAPI cakupan pengguna, diakses lewat satu antarmuka `KeyProvider` di Go, dengan rotasi yang disiapkan tetapi belum diaktifkan dan tanpa kunci pemulihan otomatis. Rinciannya ada di `docs/SECURITY_PRIVACY.md` §5a–§5d dan `docs/DECISIONS.md`.

**FL-004 tidak menutup Q-16.** Usulan bukan keputusan; sesuai protokol perubahan keputusan dan `AGENTS.md` §1 butir 5, status ditentukan pemilik project. **Q-16 tetap terbuka dan `face_templates` tetap tidak boleh diimplementasikan** sampai D-030 disetujui. Q-07 dan Q-18 **juga tetap terbuka** dan tidak diisi angka apa pun oleh FL-004.

Satu temuan FL-004 yang perlu dicatat terpisah: **DPAPI tidak menjamin keamanan terhadap semua jenis serangan** — ia terikat pada akun Windows, permudah dibaca oleh program lain yang berjalan dengan kredensial yang sama, dan pada profil roaming dapat didekripsi dari komputer lain. Batas ini dicatat di `docs/SECURITY_PRIVACY.md` §5b dan harus dijelaskan apa adanya, bukan diringkas menjadi "kunci sudah terenkripsi".

## Catatan FL-003 (15 Oktober 2026)

FL-003 **menutup** Q-01, Q-02, dan Q-14: pemilik project menetapkan Go + `chi` (D-010 DISETUJUI), SQLite mode WAL (D-020 DISETUJUI), dan polling + `ETag` (D-021 DISETUJUI); D-019 DITOLAK dan D-011 DITUNDA. Q-07 (angka ambang kecocokan) dan Q-16 (pengelolaan kunci enkripsi) **tetap terbuka** dan tidak boleh diisi dengan angka atau kebijakan yang dikira-kira. Q-16 kini menghambat implementasi `face_templates`, dan Q-07 menghambat pemberlakuan keputusan kecocokan.

## Catatan FL-002 (15 Oktober 2026)

FL-002 **tidak menutup** Q-01, Q-02, dan Q-14 — ketiganya kini memiliki rekomendasi yang tercatat, tetapi keputusan akhir tetap milik pemilik project. Q-07 (model dan ambang kecocokan) **tetap terbuka** karena angka ambang hanya boleh ditetapkan setelah pengukuran pada peserta uji, bukan disalin dari contoh mana pun. Tiga pertanyaan baru (Q-16 sampai Q-18) muncul dari analisis FL-002.

# FaceLink AI

> Sistem absensi otomatis berbasis pengenalan wajah, kamera IoT, dan aplikasi React Native. Dibangun secara lokal terlebih dahulu, dengan perhatian khusus pada privasi dan kemudahan pengembangan.

**Status:** Fondasi dokumentasi versi 0.3 — **belum ada implementasi aplikasi**.  
**Lingkup:** Prototipe tugas kampus berskala kecil: satu kelas, satu terminal kamera, dan sekitar 5–10 peserta sukarela yang memberikan persetujuan.  
**Biaya:** Seluruh perangkat lunak dan pemrosesan AI lokal harus dapat berjalan dengan biaya lisensi/layanan **Rp0**. Anggaran hardware maksimal **Rp1.000.000**.  
**Cloud:** Google Cloud dan layanan cloud lain ditunda sampai sistem lokal selesai diuji serta mendapat persetujuan pengembang.  
**Pengembangan dengan AI:** OmO direncanakan sebagai pengatur agent dengan model awal `deepseek-v4.1-flash`. Semua spesifikasi harus dapat dibaca agent dan model lain.

## Masalah yang ingin diselesaikan

Absensi manual memerlukan waktu dan dapat menimbulkan kesalahan pencatatan. FaceLink AI menguji apakah proses kehadiran satu kelas dapat dibantu oleh terminal kamera yang mengenali peserta **yang telah mendaftar secara sukarela**, lalu mencatat kehadiran secara otomatis pada sesi yang dibuka operator.

Proyek ini adalah **prototipe akademik**, bukan alat pengawasan massal atau sistem biometrik siap produksi. Sistem harus menyediakan jalur absensi manual dan **tidak boleh menyimpulkan seseorang tidak hadir hanya karena pengenalan wajah gagal**.

## Alur singkat

1. Operator masuk aplikasi dan membuka sesi absensi kelas.
2. Peserta telah didaftarkan dengan persetujuan yang jelas; data wajah diproses secara lokal.
3. Peserta berdiri seorang diri di depan webcam atau kamera IoT.
4. Mesin pengenalan wajah memeriksa kualitas gambar, mencari kecocokan, dan mengembalikan hasil terbatas.
5. Backend memeriksa persetujuan, keanggotaan kelas, status sesi, dan kemungkinan absensi ganda.
6. Jika seluruh pemeriksaan lolos, satu catatan kehadiran dibuat; dashboard React Native menampilkan pembaruannya.
7. Kegagalan deteksi, kecocokan meragukan, atau layanan yang tidak tersedia menghasilkan instruksi coba lagi/verifikasi manual, bukan absensi otomatis.

## Batasan produk

- **Untuk sekarang:** satu kelas demo, satu kamera, operator, peserta terbatas, rekap kehadiran, ekspor laporan, dan APK Android.
- **Untuk nanti:** bisa diadaptasi ke kantor/organisasi lain melalui desain modular, tetapi belum perlu fitur multiorganisasi, penggajian, atau integrasi HRIS.
- **Pengenalan wajah dengan kacamata:** tujuan pengujian, **bukan janji akurasi sempurna**.
- **Pencegahan spoofing dasar:** sasaran eksperimen; kedipan atau gerakan kepala saja tidak menjamin ketahanan terhadap foto/video.
- **iOS:** dapat mempertimbangkan web/PWA atau pengujian Expo Go, tetapi distribusi native iOS tidak dijanjikan gratis.

## Peta dokumen

| Dibaca | Dokumen | Tujuan |
|---|---|---|
| 1 | [`AGENTS.md`](AGENTS.md) | Aturan wajib dan cara kerja AI coding agent |
| 2 | [`PROJECT_STATE.md`](PROJECT_STATE.md) | Kondisi aktual project dan pekerjaan yang boleh dilakukan |
| 3 | [`docs/PRD.md`](docs/PRD.md) | Kebutuhan produk, ruang lingkup, dan kriteria penerimaan |
| 4 | [`docs/DECISIONS.md`](docs/DECISIONS.md) | Keputusan disetujui vs usulan yang belum disahkan |
| 5 | [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | Batas modul, pola komunikasi, dan urutan pengembangan |
| 6 | [`docs/API_CONTRACT.md`](docs/API_CONTRACT.md) dan [`docs/DATA_MODEL.md`](docs/DATA_MODEL.md) | Rancangan API dan database sebelum implementasi |
| 7 | [`docs/FACE_ENGINE.md`](docs/FACE_ENGINE.md) dan [`docs/SECURITY_PRIVACY.md`](docs/SECURITY_PRIVACY.md) | Pengenalan wajah, keamanan, dan persetujuan data |
| 8 | [`docs/UX_FLOWS.md`](docs/UX_FLOWS.md) dan [`docs/TEST_PLAN.md`](docs/TEST_PLAN.md) | Alur aplikasi dan pengujian |
| 9 | [`docs/ROADMAP.md`](docs/ROADMAP.md) dan [`docs/BACKLOG.md`](docs/BACKLOG.md) | Tahapan dan unit pekerjaan AI coding |
| 10 | [`docs/DEVICE_HARDWARE.md`](docs/DEVICE_HARDWARE.md) | Rencana perangkat IoT di fase berikutnya |
| 11 | [`docs/AGENT_BOOTSTRAP.md`](docs/AGENT_BOOTSTRAP.md) dan [`docs/COLLABORATION.md`](docs/COLLABORATION.md) | Mulai sesi agent dan kolaborasi GitHub |
| 12 | [`docs/OPEN_QUESTIONS.md`](docs/OPEN_QUESTIONS.md) dan [`docs/REFERENCES.md`](docs/REFERENCES.md) | Keputusan terbuka dan bacaan teknis |
| — | [`research/README.md`](research/README.md) | Tempat terpisah untuk jurnal/riset akademik tim |

## Kandidat teknologi (stack final MVP setelah FL-003)

- **Aplikasi:** React Native + Expo + TypeScript, Android lebih dahulu.
- **Backend:** **Go + `chi`** sebagai backend utama dan pemilik keputusan domain (D-010 DISETUJUI). Python tetap dipakai, tetapi hanya sebagai **mesin pengenalan wajah terpisah**, bukan backend publik; satu backend Python + FastAPI ditolak (D-019).
- **AI lokal:** Python + OpenCV dengan **YuNet** (deteksi) dan **SFace** (pengenalan) sebagai model MVP (D-012); bobot berlisensi non-permisif seperti InsightFace/ArcFace tidak dipakai (D-025). **Ambang kecocokan belum ditetapkan** dan harus dikalibrasi pada peserta uji.
- **Database:** **SQLite mode WAL** untuk MVP (D-020), dengan skema dan migrasi tetap portabel ke PostgreSQL; PostgreSQL awal ditunda (D-011). Migrasi dijalankan dari sisi Go (D-026).
- **Perangkat kamera:** webcam selama development; ESP32-S3 CAM setelah software bekerja, sebagai klien pengunggah JPEG (D-023), bukan host model AI.

Kontrak dan skema rinci ada di [`docs/API_CONTRACT.md`](docs/API_CONTRACT.md) dan [`docs/DATA_MODEL.md`](docs/DATA_MODEL.md). **Persetujuan stack ini adalah persetujuan arsitektur, bukan izin implementasi.**

## Kode yang sudah ada (FL-005)

Satu bagian sudah benar-benar ditulis dan diuji: **fondasi backend Go** di folder [`backend/`](backend/README.md). Isinya kerangka layanan, basis data SQLite mode WAL, migrasi SQL berversi, log JSON, dan dua titik kesehatan `GET /api/v1/healthz` serta `GET /api/v1/readyz`. Belum ada autentikasi, endpoint absensi, maupun kode biometrik.

```powershell
cd backend
go run ./cmd/server -migrate   # terapkan migrasi
go run ./cmd/server            # jalankan, mendengarkan di 127.0.0.1:8080
```

Seluruh variabel lingkungan didokumentasikan pada [`.env.example`](.env.example). Petunjuk lengkap, termasuk cara menjalankan pengujian, ada di [`backend/README.md`](backend/README.md).

> Aplikasi React Native, mesin AI Python, dan firmware ESP32 **belum dimulai**. Jangan membaca keberadaan folder `backend/` sebagai tanda fitur absensi sudah bekerja.

## Catatan asal README

Teks berikut adalah isi `README.md` asli yang sudah ada di branch `main` sebelum FL-001, dipertahankan apa adanya sebagai rujukan:

> # FaceLink-Ai
> FaceLink AI is a local-first, AI-powered smart attendance system integrating facial recognition, IoT, and a React Native mobile application.

Isi di atas tetap berlaku dan telah diterjemahkan ke seluruh dokumen berbahasa Indonesia ini. Judul `FaceLink-Ai` digantikan `FaceLink AI` agar konsisten dengan nama yang dipakai di seluruh dokumentasi.

## Keamanan repository

Jangan commit foto wajah nyata, embedding biometrik, data peserta yang dapat diidentifikasi, kata sandi, token perangkat, `.env`, ekspor database, atau data uji rahasia. Gunakan data sintetis untuk contoh publik. Buat `.gitignore` **sebelum** menambahkan kode dan data lokal.

## Status hak penggunaan

Belum ada keputusan lisensi untuk kode FaceLink AI. Repository publik tanpa berkas `LICENSE` **bukan** berarti kode bebas dipakai ulang. Setiap dependensi dan model pihak ketiga mempunyai ketentuan lisensi masing-masing.

**Mulai membaca:** `AGENTS.md` → `PROJECT_STATE.md` → `docs/PRD.md` → `docs/DECISIONS.md`.

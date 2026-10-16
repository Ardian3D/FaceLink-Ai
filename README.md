# FaceLink AI

> Sistem absensi otomatis berbasis pengenalan wajah, kamera IoT, dan aplikasi React Native. Dibangun secara lokal terlebih dahulu, dengan perhatian khusus pada privasi dan kemudahan pengembangan.

**Status:** Fondasi dokumentasi versi 0.3 — **belum ada implementasi aplikasi**. Revisi terakhir: **FL-010V (16 Oktober 2026)** memfinalkan keputusan arsitektur Google Auth + tiga role (**D-032–D-037 disetujui**; D-035 pada tingkat kebijakan) di atas penyelarasan **FL-010U (16 Oktober 2026)**; keputusan itu masih sebatas rancangan dan **belum ada kode yang mengimplementasikannya**.
**Lingkup:** Prototipe tugas kampus berskala kecil: satu kelas, satu terminal kamera, dan sekitar 5–10 peserta sukarela yang memberikan persetujuan.  
**Biaya:** Seluruh perangkat lunak dan pemrosesan AI lokal harus dapat berjalan dengan biaya lisensi/layanan **Rp0**. Anggaran hardware maksimal **Rp1.000.000**.  
**Cloud:** Google Cloud dan layanan cloud lain ditunda sampai sistem lokal selesai diuji serta mendapat persetujuan pengembang.  
**Pengembangan dengan AI:** OmO direncanakan sebagai pengatur agent dengan model awal `deepseek-v4.1-flash`. Semua spesifikasi harus dapat dibaca agent dan model lain.

## Masalah yang ingin diselesaikan

Absensi manual memerlukan waktu dan dapat menimbulkan kesalahan pencatatan. FaceLink AI menguji apakah proses kehadiran satu kelas dapat dibantu oleh terminal kamera yang mengenali peserta **yang telah mendaftar secara sukarela**, lalu mencatat kehadiran secara otomatis pada sesi yang dibuka **dosen** di kelas yang ditugaskan kepadanya (rancangan **FL-010U**).

Proyek ini adalah **prototipe akademik**, bukan alat pengawasan massal atau sistem biometrik siap produksi. Sistem harus menyediakan jalur absensi manual dan **tidak boleh menyimpulkan seseorang tidak hadir hanya karena pengenalan wajah gagal**.

## Alur singkat

Alur di bawah adalah **rancangan FL-010U (16 Oktober 2026)** untuk tiga role pengguna manusia; belum ada bagian mana pun yang diimplementasikan atau diuji.

1. Pengguna masuk aplikasi dengan **Google Sign-In** ("Continue with Google"). Backend memverifikasi Google ID token (OpenID Connect, RS256, `iss`/`aud`/`nonce`), menautkan identitas ke akun internal, lalu mengirim **`role` dari database** — **`admin`, `dosen`, atau `mahasiswa`**. Role tidak pernah berasal dari Google maupun dari aplikasi React Native.
2. `admin` menyiapkan akun dan role, kelas beserta penugasan dosen, peserta, serta konsen; `dosen` masuk ke **kelas yang ditugaskan kepadanya** dan membuka sesi absensi.
3. Peserta telah didaftarkan dengan persetujuan yang jelas; data wajah diproses secara lokal.
4. Peserta berdiri seorang diri di depan webcam atau kamera IoT.
5. Mesin pengenalan wajah memeriksa kualitas gambar, mencari kecocokan, dan mengembalikan hasil terbatas.
6. Backend memeriksa persetujuan, keanggotaan kelas, status sesi, dan kemungkinan absensi ganda.
7. Jika seluruh pemeriksaan lolos, satu catatan kehadiran dibuat; `dosen` melihat daftar hadir berjalan dan rekap kelasnya, sedangkan `mahasiswa` melihat status dan riwayat kehadirannya **sendiri** lewat portal self-service (`/me/*`), tanpa akses ke data biometrik atau data peserta lain.
8. Kegagalan deteksi, kecocokan meragukan, atau layanan yang tidak tersedia menghasilkan instruksi coba lagi/verifikasi manual, bukan absensi otomatis.

`device` **bukan** role pengguna manusia: perangkat IoT tetap memakai identitas dan autentikasi mesin terpisah (token perangkat) dan hanya boleh memanggil endpoint pengiriman percobaan pengenalan. Rancangan lengkap matriks peran/izin ada di [`docs/AUTHORIZATION.md`](docs/AUTHORIZATION.md). **Belum ada satu pun langkah di atas yang terverifikasi berjalan.**

## Batasan produk

- **Untuk sekarang:** satu kelas demo, satu kamera, **tiga role pengguna (`admin`, `dosen`, `mahasiswa`) dengan Google Sign-In**, peserta terbatas, rekap kehadiran, ekspor laporan, dan APK Android.
- **Untuk nanti:** bisa diadaptasi ke kantor/organisasi lain melalui desain modular, tetapi belum perlu fitur multiorganisasi, penggajian, atau integrasi HRIS.
- **Perangkat IoT (`device`) bukan role manusia:** perangkat tetap memakai identitas dan autentikasi mesin terpisah (token perangkat, bukan Google), hanya untuk mengirim percobaan pengenalan.
- **Jaringan backend:** kebijakan final **FL-010V (16 Oktober 2026)** menetapkan **default loopback** (kode FL-005 masih menolak alamat non-loopback) dengan akses HP lewat LAN privat hanya via konfigurasi eksplisit dan **wajib HTTPS bersertifikat tepercaya** (D-035); HP karena itu **belum** dapat menjangkau backend lewat Wi-Fi kelas sampai pengikatan LAN beserta TLS diimplementasikan. Rincian penyediaan sertifikat lokal dan trust chain Android **belum diuji** (pertanyaan terbuka **Q-25**).
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
| 7 | [`docs/AUTHORIZATION.md`](docs/AUTHORIZATION.md) | **Sumber tunggal matriks peran dan izin** (`admin`, `dosen`, `mahasiswa`, `device`) |
| 8 | [`docs/FACE_ENGINE.md`](docs/FACE_ENGINE.md) dan [`docs/SECURITY_PRIVACY.md`](docs/SECURITY_PRIVACY.md) | Pengenalan wajah, keamanan, dan persetujuan data |
| 9 | [`docs/UX_FLOWS.md`](docs/UX_FLOWS.md) dan [`docs/TEST_PLAN.md`](docs/TEST_PLAN.md) | Alur aplikasi dan pengujian |
| 10 | [`docs/ROADMAP.md`](docs/ROADMAP.md) dan [`docs/BACKLOG.md`](docs/BACKLOG.md) | Tahapan dan unit pekerjaan AI coding |
| 11 | [`docs/DEVICE_HARDWARE.md`](docs/DEVICE_HARDWARE.md) | Rencana perangkat IoT di fase berikutnya |
| 12 | [`docs/AGENT_BOOTSTRAP.md`](docs/AGENT_BOOTSTRAP.md) dan [`docs/COLLABORATION.md`](docs/COLLABORATION.md) | Mulai sesi agent dan kolaborasi GitHub |
| 13 | [`docs/OPEN_QUESTIONS.md`](docs/OPEN_QUESTIONS.md) dan [`docs/REFERENCES.md`](docs/REFERENCES.md) | Keputusan terbuka dan bacaan teknis |
| — | [`research/README.md`](research/README.md) | Tempat terpisah untuk jurnal/riset akademik tim |

## Kandidat teknologi (stack final MVP setelah FL-003)

- **Aplikasi:** React Native + Expo + TypeScript, Android lebih dahulu.
- **Backend:** **Go + `chi`** sebagai backend utama dan pemilik keputusan domain (D-010 DISETUJUI). Python tetap dipakai, tetapi hanya sebagai **mesin pengenalan wajah terpisah**, bukan backend publik; satu backend Python + FastAPI ditolak (D-019).
- **AI lokal:** Python + OpenCV dengan **YuNet** (deteksi) dan **SFace** (pengenalan) sebagai model MVP (D-012); bobot berlisensi non-permisif seperti InsightFace/ArcFace tidak dipakai (D-025). **Ambang kecocokan belum ditetapkan** dan harus dikalibrasi pada peserta uji.
- **Database:** **SQLite mode WAL** untuk MVP (D-020), dengan skema dan migrasi tetap portabel ke PostgreSQL; PostgreSQL awal ditunda (D-011). Migrasi dijalankan dari sisi Go (D-026).
- **Perangkat kamera:** webcam selama development; ESP32-S3 CAM setelah software bekerja, sebagai klien pengunggah JPEG (D-023), bukan host model AI.
- **Autentikasi:** **Google Sign-In** sebagai metode utama bagi tiga role manusia (`admin`, `dosen`, `mahasiswa`) — rancangan FL-010U, belum diimplementasikan. Tidak ada login email/kata sandi buatan sendiri; role **hanya** ditetapkan `admin` di backend dan tidak pernah berasal dari Google atau dari aplikasi. Sesi aplikasi memakai token opak (bukan ID token Google) yang disimpan sebagai hash di tabel `session_tokens`. `device` memakai jalur autentikasi mesin yang terpisah.

Kontrak dan skema rinci ada di [`docs/API_CONTRACT.md`](docs/API_CONTRACT.md) dan [`docs/DATA_MODEL.md`](docs/DATA_MODEL.md); matriks peran/izin ada di [`docs/AUTHORIZATION.md`](docs/AUTHORIZATION.md). **Persetujuan stack ini adalah persetujuan arsitektur, bukan izin implementasi.**

## Kode yang sudah ada (FL-005)

Satu bagian sudah benar-benar ditulis dan diuji: **fondasi backend Go** di folder [`backend/`](backend/README.md). Isinya kerangka layanan, basis data SQLite mode WAL, migrasi SQL berversi, log JSON, dan dua titik kesehatan `GET /api/v1/healthz` serta `GET /api/v1/readyz`. Belum ada autentikasi, endpoint absensi, maupun kode biometrik — **tidak ada satu pun bagian rancangan tiga role dan Google Sign-In pada FL-010U yang sudah ditulis**. Perlu dicatat apa adanya: kode ini masih menolak pengikatan ke alamat non-loopback, sehingga HP belum dapat menjangkau backend lewat Wi-Fi kelas sampai FL-011 mengimplementasikan pengikatan LAN beserta TLS (kebijakan **D-035/FL-010V**; rincian sertifikat belum diuji, lihat **Q-25** pada [`docs/OPEN_QUESTIONS.md`](docs/OPEN_QUESTIONS.md)).

```powershell
cd backend
go run ./cmd/server -migrate   # terapkan migrasi
go run ./cmd/server            # jalankan, mendengarkan di 127.0.0.1:8080
```

Seluruh variabel lingkungan didokumentasikan pada [`.env.example`](.env.example). Petunjuk lengkap, termasuk cara menjalankan pengujian, ada di [`backend/README.md`](backend/README.md).

`backend/` di akar repositori adalah **lokasi resmi** backend Go (lihat [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) §8.1). Usulan lama `apps/api/` tidak dipakai. Quality gate otomatis ada di [`.github/workflows/backend-go.yml`](.github/workflows/backend-go.yml) dan berjalan pada Windows dan Ubuntu untuk setiap Pull Request ke `main`.

> Aplikasi React Native, mesin AI Python, dan firmware ESP32 **belum dimulai**. Jangan membaca keberadaan folder `backend/` sebagai tanda fitur absensi sudah bekerja.

## Catatan revisi FL-010U (16 Oktober 2026)

Revisi README ini berasal dari **FL-010U (16 Oktober 2026)**, cabang `docs/fl-010u-google-three-roles` dari `origin/main` (`804c2f7`), dan **hanya dokumentasi** — tidak ada kode maupun migrasi yang ditulis.

- **Tiga role manusia** (`admin`, `dosen`, `mahasiswa`) dengan **Google Sign-In** sebagai metode masuk; akun mahasiswa kini ada. Role `operator` lama digantikan `dosen`, dan **cakupan `operator` lama terbagi menjadi `admin` dan `dosen`** — bukan penggantian nama dengan cakupan identik.
- **`device` bukan role pengguna manusia**: perangkat IoT tetap memakai identitas dan autentikasi mesin terpisah.
- Rancangan lengkap matriks peran/izin: [`docs/AUTHORIZATION.md`](docs/AUTHORIZATION.md). Kontrak API: [`docs/API_CONTRACT.md`](docs/API_CONTRACT.md); model data: [`docs/DATA_MODEL.md`](docs/DATA_MODEL.md); alur aplikasi: [`docs/UX_FLOWS.md`](docs/UX_FLOWS.md); pertanyaan yang masih terbuka (termasuk pengikatan jaringan dan cakupan dosen): [`docs/OPEN_QUESTIONS.md`](docs/OPEN_QUESTIONS.md).
- **Tidak ada klaim fitur sudah ada.** Seluruh skenario runtime tiga role dan Google Sign-In berstatus **BELUM TERVERIFIKASI** sampai tugas implementasi dijalankan dan hasil tesnya nyata.

## Catatan revisi FL-010V (16 Oktober 2026)

Revisi README ini berasal dari **FL-010V (16 Oktober 2026)**, cabang `docs/fl-010v-final-auth-decisions`, dan **hanya dokumentasi** — tidak ada kode maupun migrasi yang ditulis, dan matriks 53 endpoint tidak diubah.

- **Keputusan arsitektur difinalkan:** **D-032–D-037 disetujui** (D-035 pada tingkat kebijakan) — lifecycle akun, session management, hak akses `dosen`, akses jaringan lokal, kebijakan audit keamanan, dan **satu role utama per akun**. Status resmi keputusan ada di [`docs/DECISIONS.md`](docs/DECISIONS.md).
- **Pertanyaan terbuka bergerak sebagian saja:** **Q-19** menjadi kebijakan yang diputuskan (D-035), **Q-20 → D-034**, **Q-21 → D-037**, dan **Q-25** (metode penyediaan sertifikat lokal & trust chain Android) baru dicatat; **Q-22, Q-23, dan Q-24 tetap terbuka** dan tidak otomatis disetujui ([`docs/OPEN_QUESTIONS.md`](docs/OPEN_QUESTIONS.md)).
- **FL-011 siap dimulai** sebagai handoff implementasi Google Authentication & Authorization backend dengan **12 butir lingkup wajib**, dan **FL-005S** didaftarkan sebagai penguatan wajib — access log Go dapat melaporkan status `200` untuk respons `503` batas waktu dan tidak mencatat respons `500` hasil pemulihan panic, sehingga logging keamanan baru dianggap andal setelah tugas itu selesai ([`docs/BACKLOG.md`](docs/BACKLOG.md)). Urutan fase pada [`docs/ROADMAP.md`](docs/ROADMAP.md) **tidak berubah**.
- **Keamanan jaringan** ditambahkan ke bagian **Keamanan repository** di bawah: default loopback, akses LAN wajib HTTPS bersertifikat tepercaya, dan backend **tidak pernah** dibuka ke internet publik. Rincian teknis TLS LAN **belum diuji**.
- **Tidak ada klaim fitur sudah ada.** Seluruh skenario runtime tiga role, Google Sign-In, dan TLS LAN berstatus **BELUM TERVERIFIKASI** sampai tugas implementasi dijalankan dan hasil tesnya nyata.

## Catatan asal README

Teks berikut adalah isi `README.md` asli yang sudah ada di branch `main` sebelum FL-001, dipertahankan apa adanya sebagai rujukan:

> # FaceLink-Ai
> FaceLink AI is a local-first, AI-powered smart attendance system integrating facial recognition, IoT, and a React Native mobile application.

Isi di atas tetap berlaku dan telah diterjemahkan ke seluruh dokumen berbahasa Indonesia ini. Judul `FaceLink-Ai` digantikan `FaceLink AI` agar konsisten dengan nama yang dipakai di seluruh dokumentasi.

## Keamanan repository

Jangan commit foto wajah nyata, embedding biometrik, data peserta yang dapat diidentifikasi, kata sandi, token perangkat, `.env`, ekspor database, atau data uji rahasia. Gunakan data sintetis untuk contoh publik. Buat `.gitignore` **sebelum** menambahkan kode dan data lokal.

**Jaringan backend (FL-010V, 16 Oktober 2026):** backend berjalan dengan **default loopback**; akses dari HP lewat **LAN privat** hanya boleh diaktifkan lewat konfigurasi eksplisit dan **wajib HTTPS dengan sertifikat yang dipercaya perangkat** — validasi TLS tidak boleh dimatikan (`InsecureSkipVerify` atau ekuivalen dilarang) dan Wi-Fi privat **tidak** dianggap cukup melindungi access/refresh token. **Jangan pernah membuka backend ke internet publik.**

## Status hak penggunaan

Belum ada keputusan lisensi untuk kode FaceLink AI. Repository publik tanpa berkas `LICENSE` **bukan** berarti kode bebas dipakai ulang. Setiap dependensi dan model pihak ketiga mempunyai ketentuan lisensi masing-masing.

**Mulai membaca:** `AGENTS.md` → `PROJECT_STATE.md` → `docs/PRD.md` → `docs/DECISIONS.md`.

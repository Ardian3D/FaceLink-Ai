# Arsitektur Teknis FaceLink AI

**Status:** Spesifikasi arsitektur v0.3 untuk stack yang **sudah disetujui pada FL-003**; belum ada kode/deployment yang telah diverifikasi.
**Gaya rancangan:** modular, lokal terlebih dahulu, biaya perangkat lunak Rp0, kamera dapat diganti.

**Revisi FL-010U (16 Oktober 2026).** Dokumen ini diselaraskan dengan keputusan FL-010U: **tiga role pengguna manusia (`admin`, `dosen`, `mahasiswa`) memakai Google Sign-In**, sementara **`device` tetap identitas mesin terpisah**. Revisi ini bercabang dari `origin/main` (`804c2f7`). Yang berubah hanya bagian yang disebut pada §1, §2, §3, §4, §5, §9, dan §14; seluruh keputusan yang sudah disetujui (D-010, D-020, D-021, D-022, D-026, D-027, D-030, struktur repositori §8, serta catatan historis §11 dan §12) **tidak diubah**. Rancangan autentikasi Google dan sesi di sini berstatus **rancangan** dan **BELUM TERVERIFIKASI**.

**Revisi FL-010V (16 Oktober 2026).** Kebijakan akses jaringan lokal **D-035 naik status menjadi DISETUJUI pada tingkat kebijakan**, sehingga §5.1 diganti dari “temuan + tiga pilihan yang belum diputuskan” menjadi **kebijakan yang berlaku** beserta opsi penyediaan sertifikat dan batasannya. **Rincian pelaksanaan TLS LAN belum diuji dan tidak boleh diklaim sudah berfungsi.** Yang berubah pada revisi ini hanya §5.1, baris status keputusan di §9, dan dua baris risiko/pekerjaan di §14; seluruh keputusan lain (D-010, D-020, D-021, D-022, D-026, D-027, D-030, struktur repositori §8, serta catatan historis §11 dan §12) **tidak diubah**. FL-010V adalah **tahap dokumentasi saja**: tidak ada kode, konfigurasi, SQL, atau migrasi yang ditulis, dan **tidak ada skenario runtime yang diuji**.

## 1. Prinsip desain

- Backend utama adalah **Go + `chi`**, dan pengenalan wajah dijalankan oleh **AI Engine Python yang terpisah**. Backend tetap modular; jangan menambah microservice lain untuk tugas satu kelas.
- **Google hanya penyedia identitas, bukan pemilik keputusan otorisasi (FL-010U).** Google Sign-In membuktikan siapa pengguna; **otorisasi tetap milik backend Go**: role, cakupan kelas, dan kepemilikan data dibaca dari database FaceLink AI sendiri. Role **tidak pernah** diambil dari Google, dari klien React Native, atau dari klaim token; Google ID token **tidak pernah** dipakai sebagai sesi aplikasi.
- Pisahkan **keputusan absensi** dari **hasil pengenalan wajah**. AI Engine mengusulkan identitas dan skor; backend Go yang memutuskan apakah check-in legal (D-027).
- Gunakan antarmuka kamera seragam agar webcam pada fase software dapat diganti ESP32-S3 CAM tanpa mengubah domain absensi.
- Jangan membuat ketergantungan pada cloud, penyedia AI berbayar, atau OmO saat aplikasi digunakan.
- Semua data wajah tetap di lingkungan lokal; AI Engine hanya boleh mendengarkan di `127.0.0.1` atau IPC, tidak pernah terbuka pada jaringan.

## 2. Gambaran deployment lokal

```text
  [Penyedia identitas Google]  ← HANYA dipanggil saat login/re-autentikasi
   (OpenID Connect / JWKS)          tidak pernah saat operasi harian
              ^
              | 1. Google ID token (JWT) diverifikasi backend
              |
  +-----------+--------------------------------------------+
  |                                                        |
  |   TIGA KLIEN ROLE (satu aplikasi React Native + Expo)  |
  |                                                        |
  |   [admin]        [dosen]        [mahasiswa]            |
  |   dashboard      kelas yang     beranda pribadi,       |
  |   sistem         ditugaskan     /me/* sendiri          |
  |                                                        |
  +---------------------------+----------------------------+
              |
     LAN privat eksplisit + HTTPS tepercaya, atau loopback/adb reverse (§5.1 — D-035)
              |
       [API Backend - Go + chi]
      (`/api/v1`, pemilik keputusan domain)
       |       |       \
       |       |        [Database lokal: SQLite WAL]
       |       |         users · user_identities · session_tokens
       |       |
       |  [Aturan sesi, izin, konsen, absensi, audit, laporan]
       |
       |  loopback 127.0.0.1 : kontrak internal (API_CONTRACT.md seksi 6)
       v
 [AI Engine - Python + OpenCV]
       |
 [OpenCV: kualitas, deteksi, embedding, pencocokan]
       ^
       |
 [Adapter kamera: webcam sekarang / ESP32 nanti]
       ^
       |
 [device: identitas mesin terpisah, token perangkat — bukan Google]
```

Penjelasan tambahan FL-010U (16 Oktober 2026):

- Kotak **Penyedia identitas Google** hanya aktif pada alur login: klien memperoleh ID token di perangkat, lalu backend memverifikasinya (`POST /api/v1/auth/google`). Setelah sesi FaceLink AI terbit, **operasi harian tidak memanggil Google**. Google tidak pernah menjadi bagian dari jalur absensi, tidak menerima gambar wajah, dan tidak menyimpan data kehadiran.
- **Tiga klien role** di atas adalah **satu aplikasi** dengan navigasi berbeda menurut `role` dari backend; menyembunyikan menu **bukan** pengendalian akses.
- **`device` bukan klien role.** Perangkat IoT tetap memakai token perangkat dan tidak memakai Google Sign-In; satu-satunya endpoint-nya adalah `POST /api/v1/devices/{device_id}/recognition-attempts`.

Diagram adalah **rancangan tanggung jawab**, bukan pernyataan bahwa service sudah berjalan. Untuk keamanan jaringan, permintaan kamera harus masuk melalui endpoint backend berautentikasi, dan **backend-lah** yang memanggil AI Engine secara internal. Pembagian tanggung jawab lengkap ada pada `docs/API_CONTRACT.md` seksi 6.8.

> **Catatan historis:** sebelum FL-003, dokumen ini pernah mengusulkan **satu backend Python + FastAPI** (D-019). Usulan itu **DITOLAK** oleh pemilik project. Alasan lengkapnya dipindahkan ke bagian catatan historis dan **tidak berlaku** untuk implementasi.

## 3. Pembagian tanggung jawab

| Komponen | Tanggung jawab | Bukan tanggung jawabnya |
|---|---|---|
| Aplikasi React Native | Memuat **Google Sign-In UI** (tombol "Continue with Google"), menyimpan token sesi di **penyimpanan aman perangkat** (Android Keystore melalui penyimpanan aman Expo), mengelola sesi, tampilan roster, status, laporan, consent UI, serta navigasi per role | Menentukan identitas dari gambar secara independen, menentukan role, atau menyimpan ID token Google sebagai kredensial sesi |
| API backend (**Go + `chi`**) | Memverifikasi **OIDC** (ID token Google) dan **menerbitkan sesi** FaceLink AI; autentikasi, otorisasi, persetujuan, check-in idempoten, keputusan hadir, audit, laporan, seluruh penulisan database | Mempercayai role atau cakupan dari klien, menerima ID token Google sebagai sesi aplikasi, melatih model, menyimpan foto mentah permanen, atau mengimplementasikan visi komputer |
| **AI Engine Python** (proses terpisah, loopback/IPC) | Cek kualitas, deteksi satu wajah, ekstrak embedding, cocokkan kandidat, kembalikan skor dan alasan gagal | Mengubah catatan kehadiran, menulis database, atau membuka port jaringan |
| Database | Data pengguna, peserta, kelas, persetujuan, sesi, catatan dan metadata template terlindungi | Menjadi lokasi dump gambar/raw secret di Git |
| Adapter kamera | Menyediakan frame, waktu tangkap, identitas perangkat dan status | Menentukan kebijakan absensi |
| ESP32-S3 CAM (nanti) | Menangkap JPEG, autentikasi perangkat, LED/buzzer | Menjalankan seluruh model AI besar atau menyimpan database kelas |

## 4. Modul backend Go (disetujui untuk dirancang)

- `auth`: **verifikasi OIDC (ID token Google) + sesi aplikasi + bootstrap admin**; identitas, penerbitan/rotasi/pencaburan token, dan pemeriksaan hak akses. Memakai tabel `user_identities` dan `session_tokens` (nama baru dari `operator_tokens`). Google **tidak** menentukan role; role dibaca dari database pada setiap permintaan.
- `accounts` (**baru, FL-010U**): pengelolaan **akun dan role** — daftar/undang akun `admin`/`dosen`/`mahasiswa`, perubahan role, penonaktifan akun (mencabut seluruh sesi), serta penautan akun `mahasiswa` ke baris `people` (`account_user_id`).
- `self-service` (**baru, FL-010U**): portal `dosen` dan `mahasiswa` pada `/me/*` — kelas yang ditugaskan/diikuti, jadwal sesi absensi, riwayat kehadiran sendiri, dan status persetujuan sendiri (baca saja). Tidak menyentuh `face_templates`.
- `people`: profil peserta demo, kode unik, status peserta.
- `classes`: kelas demo, keanggotaan, dan **penugasan dosen** (`class_lecturers`).
- `consent`: persetujuan pemrosesan biometrik, penarikan persetujuan, penghapusan.
- `sessions`: membuat, membuka, menutup sesi dan aturan keterlambatan.
- `attendance`: validasi check-in otomatis/manual, pencegahan duplikasi, koreksi.
- `devices`: registrasi perangkat, pembatasan sesi, token perangkat, kesehatan terminal.
- `reports`: data dashboard, ekspor CSV.
- `audit`: riwayat tindakan penting tanpa data sensitif mentah.
- `face_adapter`: klien internal untuk mesin AI, timeout, validasi respons dan pemetaan status.

## 5. Zona kepercayaan dan batas komunikasi

1. **HP pengguna ↔ backend:** wajib autentikasi sesi FaceLink AI (token opak, `Authorization: Bearer`). Kebijakan jaringan yang berlaku adalah **D-035** (lihat §5.1): default loopback, LAN privat hanya lewat konfigurasi eksplisit, **HTTPS wajib dengan sertifikat tepercaya**, tanpa pengecualian untuk menonaktifkan validasi TLS, dan tanpa mengekspos port ke internet publik. **Jangan menganggap Wi-Fi privat sudah melindungi token**; enkripsi tetap wajib, dan rincian pelaksanaan TLS LAN masih **belum diuji**.
2. **HP pengguna ↔ Google (hanya saat login):** Google Sign-In berjalan di perangkat dan menghasilkan ID token; backend memverifikasi token itu lewat **JWKS** (`iss`, `aud`, `alg` RS256, `exp`/`iat`, `nonce`, `email_verified`) dan **tidak** menyimpan ID token sebagai sesi. Operasi harian tidak memanggil Google; lihat §14 soal ketergantungan internet.
3. **Kamera ↔ backend:** kredensial khusus perangkat; hanya sesi/perangkat yang sah boleh mengirim gambar; batas ukuran dan laju permintaan.
4. **Backend ↔ AI lokal:** proses terpisah melalui `127.0.0.1` atau komunikasi antarproses; tidak ada endpoint AI publik.
5. **Backend ↔ database:** kredensial lokal terbatas; backup dan retensi dengan perlindungan.
6. **Repository ↔ mesin pengembang:** hanya source code dan fixture sintetis; rahasia/data wajah tidak pernah masuk Git.

### 5.1 Kebijakan akses jaringan lokal (**D-035 DISETUJUI pada tingkat kebijakan — FL-010V, 16 Oktober 2026**) dan opsi penyediaan sertifikat yang **BELUM DIUJI**

**Kebijakan yang berlaku (D-035, disetujui pemilik pada tingkat kebijakan; FL-010V, 16 Oktober 2026).** Bagian ini **bukan lagi usulan yang menunggu keputusan**: arah kebijakannya sudah ditetapkan, sedangkan **rincian pelaksanaannya belum diuji**. Yang mengikat:

1. **Default loopback.** Nilai bawaan adalah `127.0.0.1` agar tidak ada eksposur jaringan yang terjadi tanpa sengaja. Tidak ada default yang membuka jaringan.
2. **LAN privat hanya lewat konfigurasi eksplisit.** Akses dari HP lewat LAN privat **boleh** diaktifkan, tetapi **hanya** sebagai konfigurasi eksplisit dan sadar — bukan nilai bawaan, bukan akibat samping dari menjalankan perintah.
3. **HTTPS wajib dengan sertifikat tepercaya.** Komunikasi aplikasi → backend pada LAN **wajib** HTTPS dengan sertifikat yang **dipercaya perangkat**. Wi-Fi privat **bukan** pengganti enkripsi: access token dan refresh token tidak boleh dikirim melintas jaringan tanpa TLS.
4. **Larangan TLS bypass (tanpa pengecualian).** **Dilarang**: HTTP terbuka sebagai konfigurasi deployment yang direkomendasikan; menonaktifkan validasi TLS; `InsecureSkipVerify` atau ekuivalen; menerima sertifikat apa pun tanpa pemeriksaan; melemahkan pemeriksaan hostname untuk “sekadar bisa demo”. Tidak ada baris pada dokumen ini yang boleh dibaca sebagai izin untuk salah satu hal itu.
5. **`adb reverse` untuk development.** Untuk development, loopback atau `adb reverse` (mis. `adb reverse tcp:8080 tcp:8080`) boleh dipertahankan bila sesuai dengan keamanan platform; jalur ini tidak menambah permukaan jaringan.
6. **Larangan eksposur internet publik.** Backend **tidak boleh** dibuka ke internet publik dan tidak boleh bergantung pada cloud hosting berbayar. Akses terbatas pada loopback, LAN privat yang dikendalikan, atau jalur USB.

**Temuan kode yang tetap berlaku.** Berkas `backend/internal/config/config.go` memuat fungsi `validateAddr` yang **menolak** alamat listen di luar loopback: hanya `127.0.0.1`, `localhost`, dan `::1` diterima, host kosong ditolak ("host kosong berarti mendengarkan semua antarmuka"), dan alamat lain menghasilkan galat yang berbunyi *"alamat ... berada di luar loopback; MVP hanya boleh mendengarkan 127.0.0.1 atau localhost (docs/SECURITY_PRIVACY.md §6)"*. Nilai bawaan pun sengaja loopback.

**Konsekuensi yang harus dinyatakan apa adanya:** selama kode itu berlaku, **HP tidak dapat menjangkau backend lewat Wi-Fi kelas**. Backend yang mendengarkan hanya di `127.0.0.1` tidak dapat dihubungi dari perangkat lain di LAN, sehingga aplikasi React Native pada HP tidak dapat menyelesaikan login maupun operasi harian tanpa perlakuan tambahan. Karena itu **pengikatan LAN beserta TLS adalah pekerjaan FL-011**, bukan sesuatu yang sudah ada: kode FL-005 yang berlaku **menolak** `FACELINK_ADDR` non-loopback, dan D-035 tidak mengubah kode itu.

#### 5.1.1 Opsi penyediaan sertifikat (rancangan; **semuanya BELUM DIUJI**)

Kebijakan pada butir 3–5 di atas menuntut sertifikat yang dipercaya perangkat. Opsi berikut **boleh dirancang dan didokumentasikan**, tetapi **belum satu pun dijalankan atau diuji** pada proyek ini. Jangan membaca daftar ini sebagai pernyataan bahwa TLS LAN sudah berfungsi.

| Opsi | Isi | Batasan nyata yang harus diakui |
|---|---|---|
| (a) CA lokal + sertifikat server dengan SAN alamat IP LAN | CA lokal milik proyek menerbitkan sertifikat server dengan **SAN berisi alamat IP LAN** (bukan hanya nama host); CA itu dipasang sebagai CA tepercaya di Android | Android 7+ **tidak** mempercayai CA yang dipasang pengguna untuk aplikasi kecuali aplikasi memakai `network_security_config` atau **build kustom** — lihat 5.1.2 |
| (b) Reverse proxy lokal yang menyediakan TLS | Proksi lokal (mis. di mesin pengembang) memegang sertifikat dan mengakhiri TLS, backend tetap loopback di belakangnya | Menambah satu komponen dan titik gagal; tetap menuntut sertifikat tepercaya di perangkat, sehingga batasan 5.1.2 **tidak hilang** |
| (c) `adb reverse` / loopback untuk development | HP menjangkau backend lewat USB (`adb reverse`), sehingga tidak ada trafik LAN sama sekali | Hanya cocok untuk demo berkabel dengan perangkat yang mendukung `adb`; tidak menggantikan TLS untuk skenario nirkabel |

#### 5.1.2 Batasan Android yang membuat opsi (a) dan (b) menuntut build kustom (**belum diuji**)

**Android 7 (API 24) dan yang lebih baru tidak mempercayai CA yang dipasang pengguna untuk aplikasi**, kecuali aplikasi secara eksplisit mengizinkannya lewat `network_security_config` atau dibangun sebagai build kustom. Konsekuensinya harus dinyatakan jujur:

- **Expo Go tidak memadai** untuk opsi (a) atau (b): Expo Go tidak dapat memuat `network_security_config` milik proyek, sehingga sertifikat CA lokal tidak akan dipercaya dan koneksi akan gagal verifikasi.
- Pemenuhannya menuntut **development build atau production build** aplikasi React Native, dan itulah yang **harus diuji** lebih dahulu. Selama pengujian itu belum dilakukan, status opsi (a) dan (b) adalah **BELUM DIUJI** dan tidak boleh diklaim berfungsi.
- Larangan pada butir 4 **tidak** boleh “diselesaikan” dengan mematikan validasi TLS di klien; itu bukan penyelesaian melainkan pelanggaran kebijakan.

#### 5.1.3 Batasan operasional Windows dan jaringan (**belum diuji**)

| Aspek | Yang harus diperhatikan |
|---|---|
| Firewall Windows | Izin masuk harus diberikan **hanya pada profil jaringan privat**, bukan profil publik. Membuka port pada profil publik bertentangan dengan larangan eksposur dan harus dihindari. |
| Pergantian alamat IP (DHCP) | Alamat LAN yang diperoleh lewat DHCP dapat berubah, sehingga sertifikat dengan SAN alamat IP dan konfigurasi klien menjadi tidak valid secara diam-diam. Alamat IP sebaiknya **dipatok** (IP statis atau reservasi DHCP) sebelum TLS LAN diuji. |
| Pemilihan port dan `FACELINK_ADDR` | Port harus dipilih eksplisit dan tidak bertabrakan dengan layanan lain di mesin pengembang; `FACELINK_ADDR` harus menyatakan host dan port yang dimaksud. Karena kode FL-005 menolak host non-loopback, **konfigurasi ini belum dapat dipakai sampai FL-011 mengimplementasikannya**. |
| Laptop berpindah jaringan | Bila laptop berpindah Wi-Fi (kampus → rumah → tethering), alamat LAN, aturan firewall, dan kesesuaian sertifikat dapat berubah sekaligus; skenario ini harus diuji ulang, bukan diasumsikan tetap berjalan. |
| Wi-Fi privat bukan perlindungan | Jaringan privat hanya membatasi siapa yang dapat mencoba menghubungi; ia **tidak** melindungi token di jaringan. Enkripsi tetap wajib (butir 3). |

**Status penutup:** arah kebijakan D-035 **sudah disetujui**, tetapi **belum ada satu pun skenario TLS LAN yang dijalankan atau diukur**. Tidak ada kode, konfigurasi, sertifikat, atau migrasi yang dibuat untuk itu pada FL-010V; pelaksanaannya adalah pekerjaan **FL-011**.

> **Catatan penggantian (FL-010V, 16 Oktober 2026):** tabel “tiga pilihan penyelesaian yang dicatat (belum diputuskan)” dan pernyataan “keputusan pemilik belum diambil” dari FL-010U **tidak lagi berlaku** dan dihapus dari bagian ini, karena D-035 **sudah disetujui pada tingkat kebijakan**: opsi (a) `adb reverse`/loopback, (b) LAN privat lewat konfigurasi eksplisit, dan (c) proksi lokal **beralih status** dari “usulan yang menunggu keputusan” menjadi opsi pelaksanaan di bawah kebijakan yang mengikat (5.1.1). Yang **belum** ada bukan lagi keputusan kebijakan, melainkan **rincian pelaksanaan dan pengujian** TLS LAN, dan itu tetap berstatus **BELUM DIUJI**. Temuan kode `validateAddr` di atas tetap berlaku dan tetap menjadi penghambat sampai FL-011 mengimplementasikan pengikatan LAN.

## 6. Kontrak masukan kamera (usulan)

Adapter menyediakan tangkapan `JPEG`, `device_id`, `captured_at_utc`, `correlation_id`. Backend memeriksa identitas terminal, usia frame, ukuran/tipe gambar, dan sesi aktif sebelum mengirim frame sementara ke AI. Hasil yang diteruskan ke perangkat cukup kategori `accepted`, `duplicate`, `retry`, `manual_review`, atau `error`, tanpa embedding dan tanpa identitas yang ditebak.

## 7. Penanganan kegagalan

- AI mati / batas waktu terlewati: beri status layanan terganggu, **jangan** mencatat hadir.
- Wi-Fi putus: aplikasi menandai koneksi terputus; tidak menampilkan keberhasilan palsu.
- Kecocokan tidak jelas: `manual_review`; jangan tampilkan kandidat yang tidak pasti di terminal.
- Dua kiriman bersamaan: pembatasan unik database menjadi pengaman terakhir.
- Sesi selesai saat request diproses: backend wajib memeriksa status sesi pada transaksi pencatatan.
- Persetujuan ditarik: hapus data pencocokan dan bersihkan cache sebelum pemrosesan selanjutnya.

### 7.1 Ketersediaan layanan: apa yang berhenti dan apa yang tetap jalan (FL-010U)

Karena identitas pengguna diverifikasi **sekali saat login** (bukan pada setiap permintaan), ketergantungan internet tidak sama dengan ketergantungan backend. Perbedaan ini harus tampil di UI dan tidak boleh disatukan menjadi satu pesan "sistem gangguan".

| Keadaan | Yang tetap berjalan | Yang berhenti | Pesan yang benar ke pengguna |
|---|---|---|---|
| Internet mati, Google tidak terjangkau, **sesi aplikasi masih sah** | Seluruh operasi aplikasi lewat LAN ke backend (absensi manual, sesi, peserta, rekap) | Hanya login baru dan re-autentikasi | Tidak ada gangguan; aplikasi **tidak** memanggil Google saat operasi harian |
| Internet mati, **sesi aplikasi tidak berlaku** (kedaluwarsa/dicabut) | — | Seluruh akses aplikasi; **tidak ada** jalur absensi di dalam aplikasi | "Perlu internet untuk masuk dengan Google." Aktivitas kelas dicatat di luar aplikasi sampai jaringan tersedia |
| LAN/Wi-Fi ke backend mati | — | Seluruh aplikasi (semua keputusan domain ada di backend) | Status koneksi jelas; **tidak ada** catatan kehadiran lokal yang dibuat |
| Backend mati atau tidak menjawab | — | Seluruh aplikasi | Status layanan tidak tersedia |
| Backend hidup, mesin AI mati | Absensi manual, kelola peserta/kelas, sesi, rekap, autentikasi, audit | Enrollment, pencocokan otomatis, penulisan template | Kesiapan dilaporkan terpisah pada `/readyz` (`docs/SECURITY_PRIVACY.md` §5e) |
| Kunci identitas Google tidak dapat dihubungi saat login | Semua yang memakai sesi sah | Login/re-autentikasi | `GOOGLE_UNAVAILABLE` — dibedakan dari backend mati dan dari mesin AI mati |

Ketetapan yang mengikat: **Google Sign-In tidak pernah dijanjikan berjalan offline**; sesi pengguna berumur token akses 60 menit dan token penyegar 30 hari dengan batas absolut 30 hari, sehingga pengguna yang membuka aplikasi saat jaringan tersedia minimal sekali dalam 30 hari tidak perlu internet setiap hari. Bila sesi melewati batas itu, **satu-satunya** jalan masuk adalah Google Sign-In saat internet tersedia. Rincian sesi: `docs/AUTHORIZATION.md` §3.1 dan `docs/DECISIONS.md` D-033.

## 8. Struktur repository yang direncanakan

```text
FaceLink-AI/                      # ← kondisi FL-001: hanya berkas di bawah ini yang ada
├── README.md
├── AGENTS.md
├── PROJECT_STATE.md
├── .gitignore                    # ditambahkan pada FL-001
├── docs/                         # ADA — 16 dokumen perencanaan
├── research/                     # ADA — ruang kajian jurnal (referensi saja)
├── apps/                         # ADA — aplikasi klien
│   └── mobile/                   # ADA sejak FL-UI01 — React Native + Expo (UI preview)
│                                 # TIDAK ADA `apps/api/` — backend Go ada di `backend/` (§8.1)
├── services/                     # ADA sejak FL-CV01
│   └── face-engine/              # ADA sejak FL-CV01 — prototype Python + OpenCV (YuNet/SFace)
│                                 # BUKAN layanan jaringan dan BUKAN bagian absensi
├── firmware/                     # BELUM ADA (fase hardware)
│   └── esp32-s3-cam/             # BELUM ADA (firmware, fase 5)
├── tests/                        # BELUM ADA (pengujian integrasi)
├── design-inputs/                # LOKAL SAJA — tidak di-commit (.gitignore):
│                                 # logo master, gambar referensi, video acuan.
│                                 # Aset yang dipakai aplikasi disalin ke
│                                 # apps/mobile/assets/branding/ (FL-UI01)
├── .github/workflows/            # ADA (FL-005R) — quality gate CI backend Go
└── backend/                      # ADA (FL-005) — LOKASI RESMI Go Backend
    ├── cmd/server/
    ├── internal/
    └── migrations/
```

> **Catatan FL-001 (15 Oktober 2026):** pada saat fondasi dokumentasi dibuat, hanya `README.md`, `AGENTS.md`, `PROJECT_STATE.md`, `.gitignore`, `docs/`, dan `research/` yang benar-benar ada. Folder `apps/`, `services/`, `firmware/`, dan `tests/` sengaja **belum dibuat**; keberadaannya di atas hanya rencana. Jangan menafsirkan diagram ini sebagai bukti implementasi. Nama dan struktur akhir menunggu review arsitektur.

### 8.1 Lokasi resmi backend Go (**DIPUTUSKAN pada FL-005R**)

**`backend/` di akar repositori adalah lokasi resmi Go Backend FaceLink AI.** Keputusan ini berlaku sejak FL-005R dan menggantikan usulan `apps/api/` dari fase dokumentasi.

Alasan: kode backend sudah ditulis, diuji, dan di-merge ke `main` pada `backend/` (FL-005, PR #7). Memindahkannya ke `apps/api/` hanya untuk menyesuaikan diri dengan diagram lama akan memindahkan direktori build, jalur migrasi SQL, dan working directory CI tanpa manfaat teknis, serta menambah risiko tanpa nilai bagi pengguna. Yang diselaraskan adalah dokumennya, bukan kodenya.

Konsekuensi yang berlaku:

- **`apps/api/` TIDAK dipakai.** Rujukan aktif ke jalur itu dihapus dari dokumen; tidak ada berkas yang perlu ditulis di sana.
- **`apps/` tetap ada dan tetap dipakai** untuk aplikasi klien; `apps/mobile/` (React Native + Expo) tetap rencana fase 3.
- **`services/face-engine/`** berisi **prototype FL-CV01** (Python 3.12 + OpenCV, YuNet + SFace) beserta **alat kalibrasi FL-CV02A** (`evaluation.py`, `calibration.py`, `calibrate.py`). Prototype ini berjalan sebagai program lokal dengan jendela webcam, **bukan** layanan jaringan, tidak membuka port, dan tidak terhubung ke `backend/`. Alat kalibrasi juga tidak menulis berkas dan tidak mempersistensikan biometrik. Kontrak internal `docs/API_CONTRACT.md` §6 **belum diimplementasikan**; prototype tidak membaca atau menulis database. Backend Go di `backend/` tetap satu-satunya layanan yang berjalan saat ini.
- **`firmware/`** dan **`tests/`** tetap rencana.

> **Catatan historis:** usulan `apps/api/` berasal dari fase dokumentasi FL-001–FL-003, ketika belum ada kode sama sekali. Catatan itu tidak lagi menggambarkan jalur aktif dan **tidak boleh** diikuti oleh tugas implementasi berikutnya.

## 9. Status keputusan setelah FL-003

Status per **FL-003 (15 Oktober 2026)**, ditambah catatan **FL-004**, **FL-004S** pada baris pengelolaan kunci, dan **FL-010U (16 Oktober 2026)** pada baris otorisasi/autentikasi, serta **FL-010V (16 Oktober 2026)** pada baris otorisasi/autentikasi dan baris akses jaringan lokal. Tabel ini merangkum keputusan yang **sudah berlaku**; teks keputusan yang mengikat tetap hanya ada di `docs/DECISIONS.md`. Analisis FL-002 yang ditolak dipindahkan ke §11 dan §12 sebagai catatan historis. Usulan FL-004 (D-030) **sudah berlaku sejak disetujui pada FL-004S**, tetapi berlaku sebagai **rancangan** — belum ada kode dan belum ada pengujian, sehingga tidak ada baris di sini yang boleh dibaca sebagai bukti fitur berjalan. Hal yang sama berlaku untuk FL-010U dan FL-010V: **yang ada hanya rancangan dan dokumen, belum ada implementasi autentikasi Google, akun, role, sesi, maupun TLS LAN.**

| Pertanyaan | Status setelah FL-003 |
|---|---|
| Go + Python vs Python saja | **DIPUTUSKAN** (D-010 DISETUJUI): **Go + `chi`** sebagai backend utama, **Python sebagai AI Engine terpisah**. D-019 DITOLAK. |
| PostgreSQL vs SQLite | **DIPUTUSKAN** (D-020 DISETUJUI): SQLite mode WAL lebih dahulu; D-011 (PostgreSQL awal) DITUNDA dengan skema tetap portabel. |
| Polling lebih dahulu atau WebSocket langsung | **DIPUTUSKAN** (D-021 DISETUJUI): polling interval dengan `ETag` lebih dahulu; WebSocket sebagai peningkatan setelah kebutuhan terbukti. |
| Format otorisasi internal dan penyimpanan template | **Sebagian diputuskan**: penyimpanan template memakai AES-256-GCM per baris (D-022 DISETUJUI) dengan kunci di luar database dan di luar Git; format autentikasi internal mengikuti `API_CONTRACT.md` §6. **Pengelolaan kunci (lokasi, rotasi, pemulihan) sudah ditetapkan sebagai D-030 dan DISETUJUI pada FL-004S**; yang belum ada adalah implementasi dan pengujiannya. |
| **Otorisasi dan autentikasi pengguna (FL-010U, 16 Oktober 2026; difinalkan pada FL-010V, 16 Oktober 2026)** | **DIPUTUSKAN pemilik, tetapi hanya sebagai rancangan:** tiga role pengguna manusia (`admin`, `dosen`, `mahasiswa`) dengan **Google Sign-In** sebagai metode autentikasi utama, tanpa login email/kata sandi buatan sendiri; `operator` lama digantikan `dosen`; akun `mahasiswa` kini ada; **`device` tetap identitas mesin terpisah**. Google hanya penyedia identitas — **otorisasi tetap milik backend Go**. Kontrak sesi: token opak, akses 60 menit, penyegar 30 hari dengan batas absolut 30 hari, hash SHA-256 di server, rotasi + deteksi pemakaian ulang, pencaburan per sesi/akun, role dibaca ulang dari database setiap permintaan. Matriks peran dan izin yang mengikat ada di `docs/AUTHORIZATION.md`. **FL-010V (16 Oktober 2026) memfinalkan status kebijakannya:** D-032 (lifecycle akun `invited`/`active`/`suspended` + praregistrasi admin + penautan atomik + bootstrap `created_by = NULL`), D-033 (sesi + batas absolut 30 hari), D-034 (hak `dosen` dan batas admin-only; matriks 53 endpoint **tidak berubah**), D-036 (perluasan audit keamanan wajib), dan D-037 (**tepat satu role utama aktif per akun**; satu identitas Google tidak boleh menghasilkan beberapa sesi role) **DISETUJUI**; rincian datanya ada di `docs/DATA_MODEL.md` §4.1, §4.2, §4.16, dan §4.19. **Belum ada implementasi dan belum ada pengujian**; status keputusan formalnya mengikuti `docs/DECISIONS.md` dan `docs/AUTHORIZATION.md`, bukan tabel ini. |
| **Akses jaringan lokal (FL-010V, 16 Oktober 2026)** | **DISETUJUI pada tingkat kebijakan (D-035):** default **loopback**; LAN privat hanya lewat konfigurasi eksplisit; **HTTPS wajib dengan sertifikat tepercaya**; **dilarang** menonaktifkan validasi TLS atau `InsecureSkipVerify`; **dilarang** mengekspos backend ke internet publik; `adb reverse` untuk development. **Rincian pelaksanaan (CA lokal + SAN alamat IP LAN, reverse proxy lokal, build kustom Android) belum diuji**, dan Android 7+ tidak mempercayai CA pengguna untuk aplikasi kecuali memakai `network_security_config`/build kustom sehingga Expo Go tidak memadai — lihat §5.1. Kode FL-005 yang berlaku masih **menolak** `FACELINK_ADDR` non-loopback, sehingga pengikatan LAN + TLS adalah pekerjaan **FL-011**. |
| Versi model AI, toleransi matching dan jumlah sampel | **Masih terbuka** — YuNet + SFace sudah final (D-012), tetapi angka ambang tetap Q-07; hanya dapat ditetapkan setelah pengukuran memakai peserta uji. Jangan mengarang ambang. |

Lihat `DECISIONS.md` dan `OPEN_QUESTIONS.md`. Konsekuensi yang berlaku:

- Proses backend adalah Go; Python hanya berjalan sebagai AI Engine lokal di balik kontrak internal `docs/API_CONTRACT.md` §6, tanpa akses ke database.
- Seluruh keputusan kehadiran, penulisan database, dan audit adalah tanggung jawab Go (D-026, D-027).
- **Konsekuensi FL-010U (16 Oktober 2026):** Google **bukan** pemilik keputusan otorisasi; role, cakupan kelas, dan kepemilikan data dibaca dari database FaceLink AI pada setiap permintaan. Matriks peran/izin yang mengikat ada di `docs/AUTHORIZATION.md`; daftar endpoint beserta kode kesalahan stabil autentikasi/otorisasi ada di `docs/API_CONTRACT.md` §1.5 dan §4. Modul `auth` menangani OIDC + sesi + bootstrap admin, dan modul baru `accounts` serta `self-service` menangani akun/role dan portal `dosen`/`mahasiswa` (§4). **Tidak ada satu pun yang sudah diimplementasikan atau diuji.**
- **Konsekuensi FL-010V (16 Oktober 2026):** kebijakan autentikasi/otorisasi dan jaringan lokal **sudah final pada tingkat kebijakan**, sehingga tidak ada lagi butir FL-010U di §5.1 yang menunggu keputusan pemilik. Yang berpindah menjadi pekerjaan implementasi FL-011: migrasi `00002+` (termasuk tabel target `auth_nonces` dan kolom `session_tokens.absolute_expires_at`), penautan identitas yang atomik, nonce sekali pakai, penegakan batas absolut 30 hari, audit sesuai D-036, **serta pengikatan LAN + TLS yang belum diuji (§5.1)**. Matriks endpoint (53 baris pada FL-010V, menjadi **54** pada FL-011 setelah `POST /auth/nonce` ditambahkan), nama peran, dan kode kesalahan **tidak berubah** oleh FL-010V. **FL-011 (16 Oktober 2026) sudah mengimplementasikan** migrasi `00002`/`00003`, penautan identitas atomik, nonce sekali pakai yang diterbitkan backend, penegakan batas absolut 30 hari, dan audit D-036 pada cabang `feat/fl-011-google-auth-backend`; pengikatan LAN + TLS tetap belum diuji.

## 10. Jalur ekspansi masa depan

Pertahankan konsep umum `person`, `group/class`, `session`, `device`, dan `attendance` sehingga nantinya kelas bisa dipetakan ke departemen/organisasi. **Tidak perlu** membangun multi-tenant, organisasi bertingkat, penggajian, atau manajemen cabang sekarang. Cloud baru dievaluasi setelah sistem lokal selesai dan diuji.

## 11. Catatan historis FL-002

§11.1 di bawah adalah **catatan keputusan historis** untuk usulan yang **ditolak** (D-019), dipertahankan agar alasan penolakannya dapat ditelusuri; jangan memakainya untuk merancang kode. §11.2–§11.6 menjelaskan pilihan yang **benar-benar dipakai**, sehingga tetap menjadi acuan.

### 11.1 Catatan historis: bahasa backend (D-019 DITOLAK, arsitektur berlaku = Go + Python)

> Ringkasan: arsitektur yang berlaku adalah diagram §2 — backend **Go + `chi`** dengan **Python sebagai AI Engine terpisah**. Tabel di bawah hanya mencatat perbandingan yang mendasari penolakan D-019; angka di dalamnya belum diukur pada mesin pengembang dan tidak boleh dianggap bukti.

| Aspek | Satu backend Python (FastAPI) — **ditolak (D-019)** | Go + Python — **dipakai (D-010)** |
|---|---|---|
| Jumlah proses yang harus dijalankan | 1 proses aplikasi | 2 proses + 1 kontrak internal |
| Pustaka pengenalan wajah | Native satu bahasa; `opencv-python` dan model ONNX dipanggil langsung dari proses backend | Wajib kontrak HTTP/IPC Go↔Python, timeout, dan pemetaan galat |
| Beban kerja nyata MVP | Satu kelas, satu sesi terbuka, puluhan peserta, satu kios kamera | Sama |
| Sumber daya developer | Satu bahasa, satu alat uji, satu cara deploy | Dua bahasa, dua ekosistem dependensi, dua lintasan CI |
| Risiko terbesar | Throughput HTTP lebih rendah pada beban sangat besar | Kompleksitas integrasi dan titik gagal bertambah tanpa manfaat terukur pada beban MVP |
| Biaya lisensi | Rp0 | Rp0 |

**Isi rekomendasi lama (arsip):** mulai dengan satu backend Python dan pertahankan batas modul di dalamnya. **Rekomendasi ini tidak dipakai.**

**Alasan penolakan (keputusan pemilik project, FL-003):** batas kepercayaan harus tegas — keputusan kehadiran, penulisan database, dan audit berada di proses yang tidak memuat dependensi visi komputer, sementara AI Engine dapat dimatikan, diuji, dan diganti tanpa membongkar backend. Biaya memelihara dua bahasa diterima secara sadar, dan rantai pengujiannya sudah ditetapkan pada `docs/API_CONTRACT.md` §9. Keuntungan performa Go bukan faktor penentu pada skala satu kelas.

### 11.2 Database: SQLite vs PostgreSQL

| Aspek | SQLite (WAL) — **diusulkan** | PostgreSQL lokal (D-011 lama) |
|---|---|---|
| Instalasi di Windows | Tanpa layanan tambahan; pustaka bawaan Python | Perlu memasang dan mengelola layanan database |
| Penulis tunggal | Sangat cepat untuk commit/read titik | Juga cepat |
| Beberapa penulis bersamaan | Commit terburuk melebar tajam saat penulis bertambah | Tetap stabil pada beban penulis bersamaan |
| Pencegahan duplikasi absensi | `UNIQUE(session_id, person_id)` tetap berlaku | Sama |
| Kesesuaian dengan beban MVP | Cukup: satu backend, satu penulis logis | Melebihi kebutuhan |
| Jalur keluar | Migrasi skema terkontrol ke PostgreSQL bila beban/kebutuhan berubah | Tetap |

**Rekomendasi:** **SQLite** dengan `PRAGMA journal_mode=WAL`, `busy_timeout`, dan pembatasan jumlah penulis pada tingkat aplikasi (satu proses backend, penulisan melalui transaksi pendek). Skema ditulis dengan tipe dan batasan yang portabel agar perpindahan ke PostgreSQL pada Fase 7 tidak memerlukan penulisan ulang aturan domain.

**Risiko yang harus disadari:** SQLite bukan pilihan yang dapat dibiarkan selamanya bila nanti ada beberapa proses penulis (misalnya dashboard terpisah atau sinkronisasi). Karena itu `OPEN_QUESTIONS.md` tetap memuat pertanyaan perpindahan, dan migrasi harus diuji sebagai pekerjaan tersendiri — bukan diasumsikan gratis.

### 11.3 Pustaka dan model pengenalan wajah

| Kandidat | Lisensi | Catatan |
|---|---|---|
| OpenCV Zoo **YuNet** (deteksi) | Apache-2.0 | Deteksi wajah + 5 titik; bobot kecil, cocok untuk CPU |
| OpenCV Zoo **SFace** (embedding) | Apache-2.0 | MobileFaceNet dilatih dengan SFace loss; jarak kosinus untuk pencocokan |
| **ONNX Runtime** | MIT | Jalur inferensi alternatif bila model ingin dijalankan tanpa modul OpenCV DNN |
| InsightFace `buffalo_*` (ArcFace) | Kode MIT, **bobot hanya untuk riset non-komersial** | Akurasi lebih tinggi, tetapi ketentuan bobot membatasi penggunaan komersial. **Jangan pakai pada MVP tanpa keputusan pemilik.** |
| DeepFace (pembungkus) | MIT | Berguna sebagai pembanding, tetapi menarik banyak dependensi |

**Rekomendasi:** pertahankan **YuNet + SFace** (D-012 tetap berlaku) karena lisensinya permisif, ukurannya kecil, dan jalurnya sudah dijelaskan di `FACE_ENGINE.md`. Sekaligus catat bahwa **angka ambang** masih harus ditentukan lewat pengukuran lokal, bukan disalin dari tutorial.

### 11.4 Komunikasi backend ↔ React Native

- **Polling** dengan interval dan `ETag`/penanda perubahan adalah jalur pertama: sederhana di React Native, tahan terhadap putusnya Wi-Fi, mudah diuji.
- **WebSocket** ditambahkan hanya bila kebutuhan pembaruan seketika benar-benar terbukti pada demo; menyediakan dua jalur sejak awal menggandakan permukaan uji.
- Bentuk respons harus sama untuk kedua jalur agar perpindahan nanti tidak mengubah UI.

### 11.5 Keamanan penyimpanan template biometrik

- Template disimpan sebagai **ciphertext** dengan `nonce` per baris (AES-256-GCM; D-022). Karena backend MVP adalah **Go** (D-010), implementasi enkripsi berada di Go — pustaka Python `cryptography` yang disebut pada draf FL-002 **bukan** jalur yang dipakai. Kunci berasal dari konfigurasi lokal, **tidak** masuk database maupun repositori.
- Data tambahan yang ikut diautentikasi: `person_id`, `model_id`, dan `key_version` sesuai `docs/DATA_MODEL.md` §4.12, sehingga ciphertext tidak dapat dipindahkan antaridentitas tanpa terdeteksi.
- Enkripsi tingkat aplikasi dipilih daripada enkripsi seluruh berkas database agar kebijakan dapat diuji per baris dan agar penghapusan template saat persetujuan dicabut dapat diverifikasi langsung.
- Hasil ini hanya berlaku bila pengelolaan kunci benar; **pengelolaan kunci kini diputuskan melalui D-030 (DISETUJUI pada FL-004S) setelah sebelumnya menjadi pertanyaan terbuka Q-16.** Kebijakannya: kunci di luar database dan di luar Git, dibungkus Windows DPAPI cakupan pengguna, diakses lewat satu antarmuka `KeyProvider` di Go, dengan periode pemulihan 30 hari dan tanpa kunci pemulihan otomatis. Karena DPAPI terikat pada akun Windows dan **tidak** melindungi dari program lain yang berjalan dengan kredensial yang sama, jangan menganggap lapisan ini melindungi template dari penyerang yang sudah menguasai akun. Dua klarifikasi pemilik pada FL-004S: **kesiapan layanan biometrik dipisahkan dari kesiapan absensi manual**, dan **penghapusan mengakui keterbatasan jaminan** pada WAL, cadangan, serta media penyimpanan. Rincian: `docs/SECURITY_PRIVACY.md` §5a–§5f.
- Jangan pernah menulis gambar wajah, embedding, kunci, atau token ke log.

### 11.6 Risiko spoofing

- Tidak ada model anti-spoofing kuat di jalur MVP. Uji foto/layar/cetak wajib dijalankan sebagai **pengujian keterbatasan**, bukan klaim keamanan.
- Rujukan penilaian yang benar adalah kerangka PAD (contoh ISO/IEC 30107-3) untuk **cara melaporkan** hasil, bukan untuk mengklaim sertifikasi.
- Konsekuensi arsitektur: kecocokan ambigu atau keyakinan rendah selalu mengarah ke `manual_review`; absensi otomatis tidak pernah menjadi satu-satunya jalur, dan koreksi operator selalu tersedia.

## 12. Catatan historis: diagram FL-002 untuk D-019 (**tidak berlaku**)

> Diagram yang **berlaku** adalah §2 (Go + `chi` dan AI Engine Python). Blok di bawah adalah arsip diagram usulan satu-backend-Python yang **ditolak** pada FL-003; jangan memakainya untuk merancang kode, konfigurasi, atau pengujian.

```text
        ARSIP — DIAGRAM D-019 YANG DITOLAK — JANGAN DIPAKAI UNTUK MERANCANG KODE

        [Aplikasi React Native operator]
                    |
          Wi-Fi lokal (HTTP + token)
                    |
                    v
        +-----------------------------+
        |   Backend API (Python)      |
        |   FastAPI  /api/v1          |
        |                             |
        |  auth  people  classes      |
        |  consent  sessions          |
        |  attendance  devices        |
        |  reports  audit             |
        +------+---------------+------+
               |               |
               |               | SQLite (WAL)
               |               v
               |      +--------------------+
               |      |  Database lokal    |
               |      |  portabel ke PG    |
               |      +--------------------+
               |
               v  (panggilan dalam proses / batas modul)
        +-----------------------------+
        |   Modul face_engine         |
        |   OpenCV: kualitas,         |
        |   deteksi 1 wajah,          |
        |   embedding, pencocokan     |
        +--------------+--------------+
                       ^
                       | kontrak CameraAdapter (JPEG)
                       |
        +--------------+--------------+
        |                             |
 [adapter_webcam]            [adapter_esp32] (fase 5)
  webcam laptop Windows       ESP32-S3 CAM mengunggah JPEG
                              ke endpoint perangkat terautentikasi
```

> **Peringatan pembacaan:** blok di atas menggambarkan arsitektur yang **TIDAK BERLAKU** (D-019 DITOLAK). Diagram arsitektur yang berlaku ada di **§2**. Alasan penolakan dicatat di §11.1 dan `docs/DECISIONS.md` D-019.

**Catatan penting:** garis ke `adapter_esp32` **belum ada implementasinya**. Yang dirancang sekarang hanyalah kontraknya, agar ESP32 tidak perlu diubah dari sisi domain absensi saat benar-benar dipasang. Perhatikan juga bahwa diagram arsip di atas menempatkan FastAPI sebagai backend publik dan `face_engine` di dalam proses yang sama — **keduanya tidak sesuai** dengan keputusan FL-003.

## 13. Rencana migrasi webcam laptop → kamera IoT

Prinsip: **domain absensi tidak boleh mengetahui jenis kamera**. Semua sumber gambar masuk melalui kontrak yang sama, dan hanya adapter yang berbeda.

| Tahap | Sumber gambar | Yang berubah | Yang tidak boleh berubah |
|---|---|---|---|
| 0 (sekarang) | Belum ada kamera; hanya rencana | — | Dokumen dan kontrak |
| 1 | Webcam laptop Windows melalui `adapter_webcam` | Menulis adapter pertama | Skema database, aturan sesi, UI |
| 2 | Webcam laptop + pengujian kegagalan | *Tidak ada perubahan sumber*; menambah uji kualitas dan kegagalan | Kontrak `CameraAdapter` |
| 3 | ESP32-S3 CAM melalui `adapter_esp32` | Firmware mengunggah JPEG ke `POST /api/v1/devices/{device_id}/recognition-attempts` memakai token perangkat | Endpoint, kategori hasil, aturan absensi |
| 4 | Dua sumber bersamaan (uji regresi) | Registrasi perangkat kedua | Unik `(session_id, person_id)` |

**Konsekuensi teknis yang harus dijaga sejak sekarang:**

1. Kontrak adapter mengirim **JPEG beserta metadata**, bukan objek frame OpenCV, sehingga webcam lokal dan unggahan jaringan memakai jalur yang sama.
2. Backend tidak boleh menyimpan referensi ke `cv2.VideoCapture` atau API kamera apa pun di luar adapter.
3. Batas ukuran, batas laju, dan batas waktu wajib berlaku untuk **kedua** sumber; jaringan membuat kegagalan lebih mungkin, bukan lebih mustahil.
4. **ESP32-S3 tidak diasumsikan menjalankan model AI kompleks.** Bukti pustaka Espressif menunjukkan deteksi wajah pada perangkat memang mungkin di ESP32-S3, tetapi pencocokan identitas kelas tetap menjadi tanggung jawab backend. Pembagian yang diusulkan: ESP32 menangkap dan mengirim gambar; backend memutuskan identitas. Opsi "deteksi di perangkat" boleh dievaluasi belakangan sebagai penghematan bandwidth, bukan sebagai syarat MVP.
5. `X-Request-Id`/kunci idempotensi wajib ada sebelum ESP32 masuk, karena percobaan ulang pengiriman adalah kondisi normal pada jaringan nirkabel.
6. Adaptor webcam memakai backend DirectShow (`cv2.CAP_DSHOW`) sebagai jalur utama di Windows apabila kamera gagal dibuka melalui backend bawaan; ini kandidat yang harus diuji, bukan jaminan.

## 14. Risiko, pertanyaan terbuka, dan biaya

| Kategori | Uraian |
|---|---|
| Risiko teknis | Model yang dipilih belum diukur pada peserta nyata; ambang matching belum ada angkanya dan **tidak boleh diarang** (Q-07, kalibrasi fase AI Engine). |
| Risiko teknis | Beban penulisan bersamaan pada SQLite belum diuji untuk skenario antrean kios. |
| Risiko operasional | Pengelolaan kunci enkripsi template **sudah ditetapkan sebagai D-030 (DISETUJUI pada FL-004S)**, tetapi **implementasi `face_templates` masih belum dikerjakan**. Risiko yang diterima tetap berlaku: kehilangan kunci berarti kehilangan template, dan pemulihan bergantung pada berkas kunci yang tersimpan pada akun Windows yang sama. Ditambah keterbatasan yang diakui pada FL-004S: penghapusan **tidak** menjamin isi media penyimpanan bersih secara fisik (`docs/SECURITY_PRIVACY.md` §5f). |
| Risiko privasi | Spoofing tidak tertangani secara kuat; verifikasi manual wajib tetap tersedia. |
| Risiko operasional (**FL-010U, 16 Oktober 2026**) | **Login dan re-autentikasi bergantung pada internet.** Google Sign-In memerlukan koneksi ke penyedia identitas Google; bila internet mati sementara sesi aplikasi masih sah, operasi harian lewat LAN ke backend tetap berjalan karena aplikasi tidak memanggil Google saat itu. Bila sesi tidak berlaku lagi (kedaluwarsa atau dicabut), **login tidak mungkin sampai jaringan tersedia** — pesan yang benar adalah "Perlu internet untuk masuk dengan Google." Konsekuensi rancangan: masa berlaku penyegar **30 hari** dengan batas absolut sesi **30 hari** membuat pengguna yang aktif secara berkala tidak perlu internet setiap hari, tetapi sesi yang melewati 30 hari menuntut Google Sign-In ulang, dan **Google Sign-In tidak boleh dijanjikan berjalan offline**. Tersedianya Google juga bergantung pada cache JWKS: bila kunci tidak dapat dihubungi dan `kid` tidak dikenal, backend menjawab `503 GOOGLE_UNAVAILABLE`. Absensi manual tetap jalur tetap, tetapi **tidak ada jalur absensi di aplikasi tanpa sesi yang sah** — butir kebijakan ini masih menunggu keputusan pemilik. |
| Risiko operasional (**FL-010U, 16 Oktober 2026**; status diperbarui **FL-010V, 16 Oktober 2026**) | **HP belum dapat menjangkau backend lewat Wi-Fi kelas** karena `backend/internal/config/config.go` (`validateAddr`) menolak alamat listen di luar loopback; lihat §5.1. Kebijakan penyelesaiannya **sudah disetujui pada tingkat kebijakan (D-035, FL-010V)**: default loopback, LAN privat lewat konfigurasi eksplisit, HTTPS wajib dengan sertifikat tepercaya, tanpa bypass TLS, dan tanpa eksposur internet publik. Yang **belum** ada adalah implementasi dan pengujiannya — kode FL-005 yang berlaku masih menolak `FACELINK_ADDR` non-loopback, sehingga pengikatan LAN + TLS menjadi pekerjaan **FL-011**. |
| Risiko operasional (**FL-010V, 16 Oktober 2026**) | **Penyediaan TLS untuk akses LAN belum diuji dan belum diimplementasikan.** Kebijakan D-035 menetapkan HTTPS wajib dengan sertifikat tepercaya untuk LAN, tetapi **tidak ada satu pun** opsi penyediaan sertifikat (§5.1.1) yang sudah dijalankan atau diukur: CA lokal dengan SAN alamat IP, reverse proxy lokal, maupun build kustom Android. Batasan yang harus diakui: Android 7+ **tidak** mempercayai CA yang dipasang pengguna untuk aplikasi kecuali aplikasi memakai `network_security_config`/build kustom, sehingga **Expo Go tidak memadai** dan opsi-opsi itu menuntut development/production build. Tambahan: kode FL-005 yang berlaku masih **menolak** `FACELINK_ADDR` non-loopback, sehingga pengikatan LAN + TLS adalah pekerjaan **FL-011**. **Jangan membaca baris ini sebagai pernyataan bahwa TLS LAN sudah berfungsi.** |
| Pekerjaan wajib — **SELESAI (FL-005S, 16 Oktober 2026;** dicatat pada FL-010V) | **Penguatan log respons backend SELESAI secara implementasi** pada cabang `fix/fl-005s-http-access-logging`. Riwayat masalah (tetap berlaku sebagai catatan): access log Go dapat melaporkan status **`200`** untuk respons **`503`** batas waktu dan **tidak mencatat** respons **`500`** hasil pemulihan panic. Perbaikan: `backend/internal/httpapi/router.go` menetapkan urutan middleware **satu kali** pada fungsi `chain` (`RequestID → echoRequestID → accessLog → recoverer → timeout → bodyLimit`), sehingga status yang dicatat adalah status final yang benar-benar dikirim klien (`503` batas waktu dan `500` panic tercatat) dan barisnya ditulis **tepat satu kali per permintaan** saat respons selesai; diuji oleh `backend/internal/httpapi/accesslog_test.go` (12 skenario) dengan gate lokal lulus. **FL-010V tidak mengubah source code apa pun**; **FL-005S belum di-merge** ke `main` sampai PR ditinjau dan CI belum dilaporkan lulus, sehingga keandalan logging keamanan (D-036) baru boleh dinyatakan terverifikasi secara operasional setelah itu. |
| Risiko proyek | Repositori belum memiliki `LICENSE` (D-017), sehingga ketentuan penggunaan karya belum jelas. |
| Biaya | Komponen arsitektur yang berlaku berlisensi permisif dan **Rp0**: Go + `chi`, Python + OpenCV (YuNet/SFace), ONNX Runtime, SQLite, React Native/Expo. `cryptography` (Python) **tidak dipakai** — enkripsi template dilakukan di Go memakai pustaka standar `crypto/aes` + `crypto/cipher` (D-022, `docs/API_CONTRACT.md` §6.7). FastAPI sebagai server backend juga tidak dipakai (D-019 DITOLAK). |

Kontrak yang mengikat dan berlaku satu arah dari dokumen ini: `docs/API_CONTRACT.md` (API + kontrak internal Go↔Python) dan `docs/DATA_MODEL.md` (SQLite WAL, migrasi berversi). Bila dokumen ini berbeda dengan keduanya, **kontrak yang menang** dan dokumen ini harus diperbaiki.

**FL-010U (16 Oktober 2026):** jumlah dan daftar tabel yang berlaku mengikuti `docs/DATA_MODEL.md`, yang pada FL-010U menerima delta tanpa SQL — tabel baru `user_identities` dan `class_lecturers`, penggantian nama `operator_tokens` → `session_tokens`, penghapusan `users.password_hash`, serta kolom `people.account_user_id`. Migrasinya dikerjakan pada tugas tersendiri (FL-011) dan **belum ada**. Jangan menyalin jumlah tabel dari dokumen ini sebagai fakta; rujuk `docs/DATA_MODEL.md`.

**FL-010V (16 Oktober 2026):** delta model data bertambah **satu tabel target `auth_nonces`** (nonce sekali pakai, TTL 5 menit, untuk `POST /auth/google`) dan **satu kolom target `session_tokens.absolute_expires_at`** (batas absolut sesi `created_at` + maksimum 30 hari, tidak pernah diperpanjang rotasi penyegar) — rincian di `docs/DATA_MODEL.md` §4.2 dan §4.19. Keduanya **sudah ada** pada skema yang berlaku: `auth_nonces` dibuat migrasi `00002_auth_identity.sql` dan `session_tokens.absolute_expires_at` ditambahkan pada migrasi yang sama; `00003_auth_session_history.sql` menambahkan riwayat rotasi penyegar. **FL-011 (16 Oktober 2026) mengimplementasikannya** pada cabang `feat/fl-011-google-auth-backend`. FL-010V juga menetapkan **tepat satu role utama aktif per akun** dan penautan identitas Google yang **atomik** dengan email sekali pakai (`docs/DATA_MODEL.md` §4.1, §4.16). Tidak ada kode, SQL, migrasi, atau konfigurasi yang ditulis pada FL-010V; matriks 53 endpoint, nama peran, dan kode kesalahan **tidak berubah**.

Pertanyaan yang harus dijawab pemilik sebelum implementasi: `docs/OPEN_QUESTIONS.md` — terutama **Q-16** (pengelolaan kunci — **sudah diputuskan pada FL-004S**, sisa pekerjaannya implementasi dan pengujian) dan **Q-07** (angka ambang kecocokan, kalibrasi fase AI Engine — **masih terbuka**).

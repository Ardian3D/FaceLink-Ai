# Arsitektur Teknis FaceLink AI

**Status:** Spesifikasi arsitektur v0.3 untuk stack yang **sudah disetujui pada FL-003**; belum ada kode/deployment yang telah diverifikasi.  
**Gaya rancangan:** modular, lokal terlebih dahulu, biaya perangkat lunak Rp0, kamera dapat diganti.

## 1. Prinsip desain

- Backend utama adalah **Go + `chi`**, dan pengenalan wajah dijalankan oleh **AI Engine Python yang terpisah**. Backend tetap modular; jangan menambah microservice lain untuk tugas satu kelas.
- Pisahkan **keputusan absensi** dari **hasil pengenalan wajah**. AI Engine mengusulkan identitas dan skor; backend Go yang memutuskan apakah check-in legal (D-027).
- Gunakan antarmuka kamera seragam agar webcam pada fase software dapat diganti ESP32-S3 CAM tanpa mengubah domain absensi.
- Jangan membuat ketergantungan pada cloud, penyedia AI berbayar, atau OmO saat aplikasi digunakan.
- Semua data wajah tetap di lingkungan lokal; AI Engine hanya boleh mendengarkan di `127.0.0.1` atau IPC, tidak pernah terbuka pada jaringan.

## 2. Gambaran deployment lokal

```text
[React Native / APK operator]
             |
     Wi-Fi lokal tepercaya
             |
       [API Backend - Go + chi]
      (`/api/v1`, pemilik keputusan domain)
       |       |       \
       |       |        [Database lokal: SQLite WAL]
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
```

Diagram adalah **rancangan tanggung jawab**, bukan pernyataan bahwa service sudah berjalan. Untuk keamanan jaringan, permintaan kamera harus masuk melalui endpoint backend berautentikasi, dan **backend-lah** yang memanggil AI Engine secara internal. Pembagian tanggung jawab lengkap ada pada `docs/API_CONTRACT.md` seksi 6.8.

> **Catatan historis:** sebelum FL-003, dokumen ini pernah mengusulkan **satu backend Python + FastAPI** (D-019). Usulan itu **DITOLAK** oleh pemilik project. Alasan lengkapnya dipindahkan ke bagian catatan historis dan **tidak berlaku** untuk implementasi.

## 3. Pembagian tanggung jawab

| Komponen | Tanggung jawab | Bukan tanggung jawabnya |
|---|---|---|
| Aplikasi React Native | Login, kelola sesi, tampilan roster, status, laporan, consent UI | Menentukan identitas dari gambar secara independen |
| API backend (**Go + `chi`**) | Autentikasi, otorisasi, sesi, persetujuan, check-in idempoten, keputusan hadir, audit, laporan, seluruh penulisan database | Melatih model, menyimpan foto mentah permanen, atau mengimplementasikan visi komputer |
| **AI Engine Python** (proses terpisah, loopback/IPC) | Cek kualitas, deteksi satu wajah, ekstrak embedding, cocokkan kandidat, kembalikan skor dan alasan gagal | Mengubah catatan kehadiran, menulis database, atau membuka port jaringan |
| Database | Data pengguna, peserta, kelas, persetujuan, sesi, catatan dan metadata template terlindungi | Menjadi lokasi dump gambar/raw secret di Git |
| Adapter kamera | Menyediakan frame, waktu tangkap, identitas perangkat dan status | Menentukan kebijakan absensi |
| ESP32-S3 CAM (nanti) | Menangkap JPEG, autentikasi perangkat, LED/buzzer | Menjalankan seluruh model AI besar atau menyimpan database kelas |

## 4. Modul backend Go (disetujui untuk dirancang)

- `auth`: identitas operator, login, token/sesi, pemeriksaan hak akses.
- `people`: profil peserta demo, kode unik, status peserta.
- `classes`: kelas demo dan keanggotaan.
- `consent`: persetujuan pemrosesan biometrik, penarikan persetujuan, penghapusan.
- `sessions`: membuat, membuka, menutup sesi dan aturan keterlambatan.
- `attendance`: validasi check-in otomatis/manual, pencegahan duplikasi, koreksi.
- `devices`: registrasi perangkat, pembatasan sesi, token perangkat, kesehatan terminal.
- `reports`: data dashboard, ekspor CSV.
- `audit`: riwayat tindakan penting tanpa data sensitif mentah.
- `face_adapter`: klien internal untuk mesin AI, timeout, validasi respons dan pemetaan status.

## 5. Zona kepercayaan dan batas komunikasi

1. **HP operator ↔ backend:** wajib autentikasi; pada demo lokal gunakan jaringan yang dikendalikan, tanpa mengekspos port ke internet. Sebelum penggunaan nyata dengan data personal di jaringan bersama, perlu TLS dan pemeriksaan ancaman lebih lanjut.
2. **Kamera ↔ backend:** kredensial khusus perangkat; hanya sesi/perangkat yang sah boleh mengirim gambar; batas ukuran dan laju permintaan.
3. **Backend ↔ AI lokal:** proses terpisah melalui `127.0.0.1` atau komunikasi antarproses; tidak ada endpoint AI publik.
4. **Backend ↔ database:** kredensial lokal terbatas; backup dan retensi dengan perlindungan.
5. **Repository ↔ mesin pengembang:** hanya source code dan fixture sintetis; rahasia/data wajah tidak pernah masuk Git.

## 6. Kontrak masukan kamera (usulan)

Adapter menyediakan tangkapan `JPEG`, `device_id`, `captured_at_utc`, `correlation_id`. Backend memeriksa identitas terminal, usia frame, ukuran/tipe gambar, dan sesi aktif sebelum mengirim frame sementara ke AI. Hasil yang diteruskan ke perangkat cukup kategori `accepted`, `duplicate`, `retry`, `manual_review`, atau `error`, tanpa embedding dan tanpa identitas yang ditebak.

## 7. Penanganan kegagalan

- AI mati / batas waktu terlewati: beri status layanan terganggu, **jangan** mencatat hadir.
- Wi-Fi putus: aplikasi menandai koneksi terputus; tidak menampilkan keberhasilan palsu.
- Kecocokan tidak jelas: `manual_review`; jangan tampilkan kandidat yang tidak pasti di terminal.
- Dua kiriman bersamaan: pembatasan unik database menjadi pengaman terakhir.
- Sesi selesai saat request diproses: backend wajib memeriksa status sesi pada transaksi pencatatan.
- Persetujuan ditarik: hapus data pencocokan dan bersihkan cache sebelum pemrosesan selanjutnya.

## 8. Struktur repository yang direncanakan

```text
FaceLink-AI/                      # ← kondisi FL-001: hanya berkas di bawah ini yang ada
├── README.md
├── AGENTS.md
├── PROJECT_STATE.md
├── .gitignore                    # ditambahkan pada FL-001
├── docs/                         # ADA — 16 dokumen perencanaan
├── research/                     # ADA — ruang kajian jurnal (referensi saja)
├── apps/                         # BELUM ADA — dibuat pada fase 1 dan 3
│   ├── mobile/                   # BELUM ADA (React Native + Expo, fase 3)
│   └── api/                      # BELUM ADA (Go API kandidat, fase 1)
├── services/                     # BELUM ADA
│   └── face-engine/              # BELUM ADA (Python + OpenCV, fase 2)
├── firmware/                     # BELUM ADA (fase hardware)
│   └── esp32-s3-cam/             # BELUM ADA (firmware, fase 5)
└── tests/                        # BELUM ADA (pengujian integrasi)
```

> **Catatan FL-001 (15 Oktober 2026):** pada saat fondasi dokumentasi dibuat, hanya `README.md`, `AGENTS.md`, `PROJECT_STATE.md`, `.gitignore`, `docs/`, dan `research/` yang benar-benar ada. Folder `apps/`, `services/`, `firmware/`, dan `tests/` sengaja **belum dibuat**; keberadaannya di atas hanya rencana. Jangan menafsirkan diagram ini sebagai bukti implementasi. Nama dan struktur akhir menunggu review arsitektur.

## 9. Status keputusan setelah FL-003

Status per **FL-003 (15 Oktober 2026)**, ditambah catatan **FL-004** dan **FL-004S** pada baris pengelolaan kunci. Tabel ini merangkum keputusan yang **sudah berlaku**; teks keputusan yang mengikat tetap hanya ada di `docs/DECISIONS.md`. Analisis FL-002 yang ditolak dipindahkan ke §11 dan §12 sebagai catatan historis. Usulan FL-004 (D-030) **sudah berlaku sejak disetujui pada FL-004S**, tetapi berlaku sebagai **rancangan** — belum ada kode dan belum ada pengujian, sehingga tidak ada baris di sini yang boleh dibaca sebagai bukti fitur berjalan.

| Pertanyaan | Status setelah FL-003 |
|---|---|
| Go + Python vs Python saja | **DIPUTUSKAN** (D-010 DISETUJUI): **Go + `chi`** sebagai backend utama, **Python sebagai AI Engine terpisah**. D-019 DITOLAK. |
| PostgreSQL vs SQLite | **DIPUTUSKAN** (D-020 DISETUJUI): SQLite mode WAL lebih dahulu; D-011 (PostgreSQL awal) DITUNDA dengan skema tetap portabel. |
| Polling lebih dahulu atau WebSocket langsung | **DIPUTUSKAN** (D-021 DISETUJUI): polling interval dengan `ETag` lebih dahulu; WebSocket sebagai peningkatan setelah kebutuhan terbukti. |
| Format otorisasi internal dan penyimpanan template | **Sebagian diputuskan**: penyimpanan template memakai AES-256-GCM per baris (D-022 DISETUJUI) dengan kunci di luar database dan di luar Git; format autentikasi internal mengikuti `API_CONTRACT.md` §6. **Pengelolaan kunci (lokasi, rotasi, pemulihan) sudah ditetapkan sebagai D-030 dan DISETUJUI pada FL-004S**; yang belum ada adalah implementasi dan pengujiannya. |
| Versi model AI, toleransi matching dan jumlah sampel | **Masih terbuka** — YuNet + SFace sudah final (D-012), tetapi angka ambang tetap Q-07; hanya dapat ditetapkan setelah pengukuran memakai peserta uji. Jangan mengarang ambang. |

Lihat `DECISIONS.md` dan `OPEN_QUESTIONS.md`. Konsekuensi yang berlaku:

- Proses backend adalah Go; Python hanya berjalan sebagai AI Engine lokal di balik kontrak internal `docs/API_CONTRACT.md` §6, tanpa akses ke database.
- Seluruh keputusan kehadiran, penulisan database, dan audit adalah tanggung jawab Go (D-026, D-027).

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
| Risiko proyek | Repositori belum memiliki `LICENSE` (D-017), sehingga ketentuan penggunaan karya belum jelas. |
| Biaya | Komponen arsitektur yang berlaku berlisensi permisif dan **Rp0**: Go + `chi`, Python + OpenCV (YuNet/SFace), ONNX Runtime, SQLite, React Native/Expo. `cryptography` (Python) **tidak dipakai** — enkripsi template dilakukan di Go memakai pustaka standar `crypto/aes` + `crypto/cipher` (D-022, `docs/API_CONTRACT.md` §6.7). FastAPI sebagai server backend juga tidak dipakai (D-019 DITOLAK). |

Kontrak yang mengikat dan berlaku satu arah dari dokumen ini: `docs/API_CONTRACT.md` (API + kontrak internal Go↔Python) dan `docs/DATA_MODEL.md` (skema 15 tabel, SQLite WAL, migrasi berversi). Bila dokumen ini berbeda dengan keduanya, **kontrak yang menang** dan dokumen ini harus diperbaiki.

Pertanyaan yang harus dijawab pemilik sebelum implementasi: `docs/OPEN_QUESTIONS.md` — terutama **Q-16** (pengelolaan kunci — **sudah diputuskan pada FL-004S**, sisa pekerjaannya implementasi dan pengujian) dan **Q-07** (angka ambang kecocokan, kalibrasi fase AI Engine — **masih terbuka**).

# Arsitektur Teknis FaceLink AI

**Status:** Usulan v0.2, belum ada kode/deployment yang telah diverifikasi.  
**Gaya rancangan:** modular, lokal terlebih dahulu, biaya perangkat lunak Rp0, kamera dapat diganti.

## 1. Prinsip desain

- Mulai dari **satu backend modular** dan **satu pekerja AI lokal** bila Go dipilih; hindari microservices berlebihan untuk tugas kelas.
- Pisahkan **keputusan absensi** dari **hasil pengenalan wajah**. AI mengusulkan identitas; backend memutuskan apakah check-in legal.
- Gunakan antarmuka kamera seragam agar webcam pada fase software dapat diganti ESP32-S3 CAM tanpa mengubah domain absensi.
- Jangan membuat ketergantungan pada cloud, penyedia AI berbayar, atau OmO saat aplikasi digunakan.
- Semua data wajah tetap di lingkungan lokal; pertimbangkan perlindungan koneksi bila perangkat berada pada jaringan bersama.

## 2. Gambaran deployment lokal

```text
[React Native / APK operator]
             |
     Wi-Fi lokal tepercaya
             |
       [API Backend]
      (Go, kandidat)
       |       |       \
       |       |        [Database lokal]
       |       |
       |  [Aturan sesi, izin, absensi dan laporan]
       |
 [Pekerja AI Python lokal]
       |
 [OpenCV: kualitas, deteksi, embedding, pencocokan]
       ^
       |
 [Adapter kamera: webcam sekarang / ESP32 nanti]
```

Diagram adalah **rancangan tanggung jawab**, bukan pernyataan bahwa service sudah berjalan. Untuk keamanan jaringan, permintaan kamera sebaiknya masuk melalui endpoint backend berautentikasi, dan **backend-lah** yang memanggil pekerja AI secara internal.

## 3. Pembagian tanggung jawab

| Komponen | Tanggung jawab | Bukan tanggung jawabnya |
|---|---|---|
| Aplikasi React Native | Login, kelola sesi, tampilan roster, status, laporan, consent UI | Menentukan identitas dari gambar secara independen |
| API backend | Autentikasi, otorisasi, sesi, persetujuan, check-in idempoten, audit, laporan | Melatih model atau menyimpan foto mentah permanen |
| Mesin AI Python | Cek kualitas, deteksi satu wajah, ekstrak embedding, cocokkan kandidat, alasan gagal | Mengubah catatan kehadiran langsung |
| Database | Data pengguna, peserta, kelas, persetujuan, sesi, catatan dan metadata template terlindungi | Menjadi lokasi dump gambar/raw secret di Git |
| Adapter kamera | Menyediakan frame, waktu tangkap, identitas perangkat dan status | Menentukan kebijakan absensi |
| ESP32-S3 CAM (nanti) | Menangkap JPEG, autentikasi perangkat, LED/buzzer | Menjalankan seluruh model AI besar atau menyimpan database kelas |

## 4. Modul backend yang diusulkan

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

## 9. Pilihan yang belum final

Status per **FL-002 (15 Oktober 2026)**. Analisis lengkap pada §11–§13.

| Pertanyaan | Status setelah FL-002 |
|---|---|
| Go + Python vs Python saja | **Ada rekomendasi** (D-019): satu backend Python + FastAPI. Menunggu persetujuan pemilik. |
| PostgreSQL vs SQLite | **Ada rekomendasi** (D-020): SQLite mode WAL lebih dahulu, jalur migrasi ke PostgreSQL disiapkan. Menunggu persetujuan. |
| Polling lebih dahulu atau WebSocket langsung | **Ada rekomendasi** (D-021): polling interval dengan `ETag` lebih dahulu; WebSocket sebagai peningkatan opsional. Menunggu persetujuan. |
| Format otorisasi internal dan penyimpanan template | **Ada rekomendasi** (D-022 untuk penyimpanan template terenkripsi; format autentikasi mengikuti `API_CONTRACT.md` §4). Menunggu persetujuan. |
| Versi model AI, toleransi matching dan jumlah sampel | **Masih terbuka** — tetap Q-07; hanya dapat ditetapkan setelah pengukuran memakai model terpilih dan peserta uji. Jangan mengarang ambang. |

Lihat `DECISIONS.md` dan `OPEN_QUESTIONS.md`; jangan menganggap usulan sebagai keputusan yang sudah disahkan.

## 10. Jalur ekspansi masa depan

Pertahankan konsep umum `person`, `group/class`, `session`, `device`, dan `attendance` sehingga nantinya kelas bisa dipetakan ke departemen/organisasi. **Tidak perlu** membangun multi-tenant, organisasi bertingkat, penggajian, atau manajemen cabang sekarang. Cloud baru dievaluasi setelah sistem lokal selesai dan diuji.

## 11. Analisis keputusan FL-002

Bagian ini memuat **perbandingan dan alasan** yang mendasari usulan D-019 sampai D-023. Semuanya masih **USULAN** dan belum mengikat. Angka atau klaim di bawah hanya boleh dianggap bukti bila disertai hasil pengukuran yang dapat diulang pada mesin pengembang.

### 11.1 Bahasa backend: Python saja vs Go + Python

| Aspek | Satu backend Python (FastAPI) — **diusulkan** | Go + Python (D-010 lama) |
|---|---|---|
| Jumlah proses yang harus dijalankan | 1 proses aplikasi | 2 proses + 1 kontrak internal |
| Pustaka pengenalan wajah | Native satu bahasa; `opencv-python` dan model ONNX dipanggil langsung dari proses backend | Wajib kontrak HTTP/IPC Go↔Python, timeout, dan pemetaan galat |
| Beban kerja nyata MVP | Satu kelas, satu sesi terbuka, puluhan peserta, satu kios kamera | Sama |
| Sumber daya developer | Satu bahasa, satu alat uji, satu cara deploy | Dua bahasa, dua ekosistem dependensi, dua lintasan CI |
| Risiko terbesar | Throughput HTTP lebih rendah pada beban sangat besar | Kompleksitas integrasi dan titik gagal bertambah tanpa manfaat terukur pada beban MVP |
| Biaya lisensi | Rp0 | Rp0 |

**Rekomendasi:** mulai dengan **satu backend Python** dan pertahankan batas modul di dalamnya. Pembagian bahasa tetap mungkin dilakukan **setelah** ada pengukuran yang menunjukkan Python menjadi penghambat, dan karena itu rancangan modul harus tetap memisahkan `face_engine` dari lapisan HTTP agar pemindahan nanti tidak membongkar domain absensi.

**Alasan utama:** keuntungan performa Go tidak menjadi faktor penentu pada skala satu kelas, sedangkan biaya pemeliharaan dua bahasa nyata sejak hari pertama. Pekerjaan akademik juga lebih mudah dipertanggungjawabkan bila hanya ada satu jalur proses yang perlu diuji.

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

- Template disimpan sebagai **ciphertext** dengan `nonce` per baris (AES-GCM melalui pustaka `cryptography`, lisensi Apache-2.0/BSD); kunci berasal dari konfigurasi lokal, **tidak** masuk repository.
- Data tambahan yang ikut diautentikasi: `person_id`, `model_id`, `model_version`, sehingga ciphertext tidak dapat dipindahkan antaridentitas tanpa terdeteksi.
- Enkripsi tingkat aplikasi dipilih daripada enkripsi seluruh berkas database agar kebijakan dapat diuji per baris dan agar penghapusan template saat persetujuan dicabut dapat diverifikasi langsung.
- Hasil ini hanya berlaku bila pengelolaan kunci benar; **pengelolaan kunci masih menjadi pertanyaan terbuka** (Q-16).
- Jangan pernah menulis gambar wajah, embedding, kunci, atau token ke log.

### 11.6 Risiko spoofing

- Tidak ada model anti-spoofing kuat di jalur MVP. Uji foto/layar/cetak wajib dijalankan sebagai **pengujian keterbatasan**, bukan klaim keamanan.
- Rujukan penilaian yang benar adalah kerangka PAD (contoh ISO/IEC 30107-3) untuk **cara melaporkan** hasil, bukan untuk mengklaim sertifikasi.
- Konsekuensi arsitektur: kecocokan ambigu atau keyakinan rendah selalu mengarah ke `manual_review`; absensi otomatis tidak pernah menjadi satu-satunya jalur, dan koreksi operator selalu tersedia.

## 12. Diagram arsitektur MVP (usulan setelah FL-002)

Menggantikan diagram §2 **bila** D-019 disetujui. Bila D-010 dipertahankan, diagram §2 yang berlaku.

```text
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

**Catatan penting:** garis ke `adapter_esp32` **belum ada implementasinya**. Yang dirancang sekarang hanyalah kontraknya, agar ESP32 tidak perlu diubah dari sisi domain absensi saat benar-benar dipasang.

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
| Risiko teknis | Model yang dipilih belum diukur pada peserta nyata; ambang matching belum ada angkanya dan **tidak boleh diarang**. |
| Risiko teknis | Beban penulisan bersamaan pada SQLite belum diuji untuk skenario antrean kios. |
| Risiko operasional | Pengelolaan kunci enkripsi template belum ditetapkan. |
| Risiko privasi | Spoofing tidak tertangani secara kuat; verifikasi manual wajib tetap tersedia. |
| Risiko proyek | Repositori belum memiliki `LICENSE` (D-017), sehingga ketentuan penggunaan karya belum jelas. |
| Biaya | Seluruh komponen yang diusulkan berlisensi permisif dan **Rp0**: Python, FastAPI, OpenCV, ONNX Runtime, SQLite, React Native/Expo, pustaka `cryptography`, model YuNet/SFace. |

Pertanyaan yang harus dijawab pemilik sebelum implementasi: lihat daftar persetujuan pada laporan FL-002 dan `OPEN_QUESTIONS.md`.

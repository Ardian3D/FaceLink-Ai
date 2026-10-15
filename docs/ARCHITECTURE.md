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

- Go + Python vs Python saja.
- PostgreSQL vs SQLite.
- Polling lebih dahulu atau WebSocket langsung.
- Format otorisasi internal antarlayanan dan penyimpanan template biometrik.
- Versi model AI, toleransi matching dan aturan jumlah sampel.

Lihat `DECISIONS.md` dan `OPEN_QUESTIONS.md`; jangan menganggap usulan sebagai keputusan yang sudah disahkan.

## 10. Jalur ekspansi masa depan

Pertahankan konsep umum `person`, `group/class`, `session`, `device`, dan `attendance` sehingga nantinya kelas bisa dipetakan ke departemen/organisasi. **Tidak perlu** membangun multi-tenant, organisasi bertingkat, penggajian, atau manajemen cabang sekarang. Cloud baru dievaluasi setelah sistem lokal selesai dan diuji.

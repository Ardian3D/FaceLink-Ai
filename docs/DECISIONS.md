# Catatan Keputusan FaceLink AI

Dokumen ini memisahkan apa yang **sudah disepakati**, apa yang **masih merupakan usulan**, dan apa yang **ditunda**. Agent dilarang mengubah status usulan menjadi disetujui tanpa konfirmasi pemilik project.

**Status:** `DISETUJUI`, `USULAN`, `DITUNDA`, `TERBUKA`.

> **Catatan FL-003 (15 Oktober 2026).** Pemilik project menetapkan stack melalui tugas FL-003. Keputusan berikut sekarang **berstatus DISETUJUI** sebagai keputusan arsitektur final tahap MVP: **D-010** (Go + `chi` sebagai backend utama, Python sebagai AI engine terpisah), **D-020** (SQLite mode WAL sebagai database MVP), **D-012** (YuNet + SFace sebagai model MVP), **D-021** (polling + `ETag` lebih dahulu), **D-022** (AES-256-GCM per baris untuk template biometrik), **D-023** (ESP32-S3 CAM sebagai pengunggah JPEG), dan **D-025** (menolak bobot model berlisensi non-permisif).
>
> Keputusan yang **tidak dipilih**: **D-019** (satu backend Python + FastAPI) berstatus **DITOLAK**; **D-011** (PostgreSQL sebagai database awal) berstatus **DITUNDA** sampai ada kebutuhan nyata.
>
> Persetujuan ini adalah **persetujuan arsitektur**, bukan izin implementasi. Penulisan kode backend, AI engine, aplikasi React Native, dan firmware tetap memerlukan tugas terpisah yang disetujui.
>
> Analisis, trade-off, dan risiko lengkap ada di `docs/ARCHITECTURE.md` §11–§14. Kontrak API ada di `docs/API_CONTRACT.md`; skema data ada di `docs/DATA_MODEL.md`.

| ID | Status | Keputusan dan alasan |
|---|---|---|
| D-001 | DISETUJUI | FaceLink AI adalah **MVP tugas kampus berskala kecil**, tetapi kode dibuat modular agar lebih mudah diadaptasi ke kantor/organisasi nanti. |
| D-002 | DISETUJUI | Software, pemrosesan AI, database, dan layanan lokal harus **Rp0**; total hardware **maksimal Rp1.000.000**. |
| D-003 | DISETUJUI | **Lokal terlebih dahulu**; Google Cloud Free Trial baru dipertimbangkan setelah MVP selesai, diuji, dan mendapat izin terpisah. |
| D-004 | DISETUJUI | Urutan fase: arsitektur/backend → mesin pengenalan wajah → UI React Native → integrasi software lewat webcam → hardware → validasi dan APK → cloud opsional. |
| D-005 | DISETUJUI | Aplikasi utama menggunakan **React Native**, Android harus dapat menghasilkan APK. Distribusi native iOS bukan kewajiban gratis. |
| D-006 | DISETUJUI | Fokus tim pengembangan ini adalah software. Tinjauan jurnal akademik ditangani rekan tim di jalur terpisah. |
| D-007 | DISETUJUI | GitHub menjadi pusat kode, PRD, arsitektur, keputusan dan status; seluruh dokumen harus **tidak bergantung pada satu model AI**. |
| D-008 | DISETUJUI | OmO direncanakan untuk AI coding, dengan `deepseek-v4.1-flash` sebagai model awal. Dukungan tool dan kuota tanpa biaya masih harus diuji. |
| D-009 | DISETUJUI | Hardware dibeli setelah software dapat diuji memakai webcam. Pengenalan wajah berkacamata adalah tujuan yang harus diverifikasi. |
| D-010 | DISETUJUI (FL-003) | Backend utama **Go + `chi`**; mesin CV lokal **Python + OpenCV** berjalan sebagai proses terpisah. Alasan: Go memegang autentikasi, otorisasi, absensi, sesi, orang, perangkat, rekap, audit, database, dan seluruh keputusan domain; Python hanya melakukan deteksi wajah, kualitas gambar, embedding, dan pencocokan. Ini menutup persaingan dengan D-019. Konsekuensi: dua bahasa, dua paket dependensi, satu kontrak internal yang harus dijaga. Kondisi peninjauan ulang: jika biaya integrasi dua proses terbukti melebihi manfaat, atau jika pemilik memutuskan sebaliknya. |
| D-011 | DITUNDA (FL-003) | PostgreSQL sebagai **database awal** ditunda. Skema dan migrasi tetap dirancang portabel supaya perpindahan nanti tidak memerlukan desain ulang. Kondisi peninjauan ulang: kebutuhan penulisan bersamaan dari banyak proses, kebutuhan replikasi, atau dataset uji yang membuat SQLite menjadi hambatan nyata. |
| D-012 | DISETUJUI (FL-003) | Model MVP: OpenCV Zoo **YuNet** (deteksi) + **SFace** (pengenalan). Wajib: catat versi model dan checksum di repositori, dan verifikasi lisensi berkas bobot yang benar-benar diunduh sebelum dipakai. Ambang kecocokan tetap harus dikalibrasi memakai data uji berizin dan **tidak boleh** disalin dari contoh mana pun. |
| D-013 | USULAN | Operator/admin sebagai satu-satunya pengguna aplikasi wajib di MVP; akun peserta opsional. Perangkat kamera memakai identitas terpisah. |
| D-014 | USULAN | Kios kamera **satu orang per pemindaian**, bukan mengenali seluruh ruangan. Kandidat ESP32-S3 CAM. |
| D-015 | USULAN | Menyimpan template/embedding terenkripsi, tanpa menyimpan foto mentah secara default; menyediakan penghapusan dan absensi manual. |
| D-016 | DITUNDA | Cloud, sinkronisasi luar jaringan, multiorganisasi, integrasi HRIS, billing, CCTV terus-menerus, dan rilis native iOS berbayar. |
| D-017 | TERBUKA | Lisensi kode asli FaceLink AI serta pembagian kepemilikan karya antaranggota tim. Untuk sementara **tidak memilih `LICENSE`**. D-025 menutup risiko lisensi bobot model, tetapi tidak menutup risiko lisensi kode asli. |
| D-018 | TERBUKA | Alamat dan kondisi repository GitHub perlu diperiksa sebelum ada perintah commit/push. **Telah diverifikasi pada FL-001/FL-001R** (lihat D-024). |
| D-019 | DITOLAK (FL-003) | **Satu backend Python + FastAPI** sebagai pengganti Go. Alasan penolakan: kepemilikan database, keputusan domain absensi, dan API publik harus berada pada satu pemilik yang kuat; pemilik project juga menetapkan Go sebagai backend utama. Ini bukan penilaian atas kualitas FastAPI; FastAPI tetap boleh dipakai sebagai transport internal Python bila diperlukan. Dampak: semua teks yang menyebut FastAPI sebagai backend publik harus dianggap tidak berlaku. |
| D-020 | DISETUJUI (FL-003) | **SQLite mode WAL** sebagai database MVP, dengan pengaturan `foreign_keys=ON`, `busy_timeout`, transaksi pendek, dan prosedur pencadangan yang jelas. Alasan: satu proses penulis (Go), skala demo satu kelas, tanpa layanan database terpisah, dan tanpa biaya. Konsekuensi: penulisan bersamaan terbatas; skema dan migrasi harus tetap portabel ke PostgreSQL. Detail: `docs/DATA_MODEL.md` §8. |
| D-021 | DISETUJUI (FL-003) | **Polling interval dengan `ETag`/`Last-Modified`** sebagai jalur komunikasi pertama ke React Native. Alasan: sederhana, mudah diuji, tahan terhadap jaringan lokal yang tidak stabil, dan tidak memerlukan infrastruktur tambahan. WebSocket ditunda hingga ada kebutuhan pembaruan seketika yang terbukti, bukan diasumsikan. |
| D-022 | DISETUJUI (FL-003) | **Enkripsi tingkat aplikasi AES-256-GCM per baris** untuk template biometrik: nonce 12 byte dari sumber acak kriptografis **unik per baris**, AAD mengikat `person_id` + `model_id` + `model_version` + `key_version` (spesifikasi lengkap: `docs/DATA_MODEL.md` §4.12), verifikasi integritas sebelum dekripsi, dan penolakan bila dekripsi gagal. Bukan enkripsi seluruh berkas database. Kewajiban kunci: disimpan di luar database dan di luar Git. **Spesifikasi pengelolaan kunci (lokasi, rotasi, pemulihan) belum ditulis dan tetap Q-16**; jangan menganggapnya sudah ada. |
| D-023 | DISETUJUI (FL-003) | **Kontrak `CameraAdapter` berbasis JPEG + metadata** sebagai satu-satunya jalur masuk gambar; ESP32-S3 CAM kelak menjadi **klien pengunggah JPEG**, bukan host model AI. Identifikasi wajah tetap dijalankan pada sistem lokal. Rencana migrasi: `docs/ARCHITECTURE.md` §13. |
| D-024 | DISETUJUI (fakta) | Remote `origin` = `https://github.com/Ardian3D/FaceLink-Ai`. FL-001 di-merge ke `main` (`cd31ec0`) dan FL-002 di-merge ke `main` (`dee62b4`). Dicatat sebagai **fakta terverifikasi**, bukan keputusan desain. |
| D-025 | DISETUJUI (FL-003) | **Menghindari dependensi pada bobot model yang ketentuan penggunaannya tidak sesuai** dengan penggunaan project. Konsekuensi langsung: **InsightFace/ArcFace (`buffalo_*`) tidak dipakai** karena bobotnya berketentuan riset non-komersial, sedangkan YuNet + SFace memakai lisensi permisif dan wajib diverifikasi ulang pada berkas bobot yang benar-benar diunduh. |
| D-026 | DISETUJUI (FL-003) | **Migrasi skema memakai satu mekanisme SQL berversi** yang dijalankan dari sisi Go. **Alembic tidak dipakai sebagai migrator utama** karena backend bukan Python. Nama berkas, urutan versi, penanganan kegagalan, dan pencadangan sebelum migrasi berisiko ada di `docs/DATA_MODEL.md` §9. |
| D-027 | DISETUJUI (FL-003) | **Kepemilikan tunggal keputusan domain absensi dan penulisan database berada pada Go.** Python tidak boleh langsung menetapkan peserta sebagai hadir, dan tidak boleh menulis tabel kehadiran. Kontrak internal: `docs/API_CONTRACT.md` §6. |
| D-028 | DISETUJUI (FL-003) | **UI React Native bukan penentu kehadiran.** Backend Go adalah sumber kebenaran; aplikasi hanya menampilkan hasil dan mengirim permintaan manual melalui endpoint yang diaudit. Ketentuan per layar: `docs/UX_FLOWS.md` §10. |
| D-029 | TERBUKA | **Liveness detection bukan bagian dari YuNet/SFace.** Keduanya tidak boleh diklaim menyediakan perlindungan anti-spoofing lengkap. Mitigasi tahap MVP bersifat prosedural (operator hadir, satu orang per pemindaian, verifikasi manual) dan harus diuji sebagai eksperimen terpisah. |

## Dampak FL-003 terhadap keputusan lama

| Keputusan lama | Status baru | Dampak yang harus dipahami |
|---|---|---|
| D-010 (Go + Python) | DISETUJUI | Tidak ada perubahan teks keputusan; statusnya naik dari USULAN. |
| D-011 (PostgreSQL awal) | DITUNDA | Tidak dihapus. Rancangan skema tetap portabel; tidak ada janji bahwa migrasi SQLite→PostgreSQL berjalan identik tanpa penyesuaian. |
| D-019 (Python-only) | DITOLAK | Rekomendasi FL-002 dibatalkan oleh keputusan pemilik. Semua rujukan FastAPI sebagai backend publik dihapus dari dokumen terkait. |
| D-020 (SQLite WAL) | DISETUJUI | Sama seperti D-011: keputusan database awal kini final. |
| D-012, D-021, D-022, D-023, D-025 | DISETUJUI | Masing-masing menjadi basis kontrak API, model data, dan keamanan. |
| D-024 | Diperluas | Ditambah fakta merge FL-002 (`dee62b4`). |

## Protokol perubahan keputusan

1. Jelaskan masalah dan pilihan yang tersedia.
2. Cantumkan biaya, risiko, dampak ke API/data/privasi, dan pengaruh terhadap roadmap.
3. Minta persetujuan pemilik project.
4. Perbarui status keputusan beserta tanggal dan alasan jika disetujui.
5. Perbarui `PROJECT_STATE.md` serta spesifikasi lain yang terpengaruh.

**Jangan diam-diam mengganti stack, menetapkan layanan berbayar, atau menambah fitur di luar lingkup.**

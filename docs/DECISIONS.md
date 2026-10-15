# Catatan Keputusan FaceLink AI

Dokumen ini memisahkan apa yang **sudah disepakati**, apa yang **masih merupakan usulan**, dan apa yang **ditunda**. Agent dilarang mengubah status usulan menjadi disetujui tanpa konfirmasi pemilik project.

**Status:** `DISETUJUI`, `USULAN`, `DITUNDA`, `TERBUKA`.

> **Catatan FL-002 (15 Oktober 2026).** FL-002 menambahkan **D-019 sampai D-025**. Seluruhnya berstatus **USULAN** kecuali D-024 yang hanya mencatat fakta Git terverifikasi. D-019 dan D-020 **bersaing langsung** dengan D-010 dan D-011: selama pemilik belum memilih, **kedua pasangan belum berlaku** dan implementasi backend/database belum boleh dimulai. Analisis, trade-off, dan risiko lengkap ada di `docs/ARCHITECTURE.md` §11–§14.

> **Catatan ini bukan persetujuan.** Tidak ada satu pun rekomendasi FL-002 yang berlaku sebelum pemilik project menyatakannya.

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
| D-010 | USULAN (direvisi FL-002) | Backend utama **Go + `chi`**, mesin CV lokal **Python + OpenCV**. Alternatif: satu backend Python yang lebih sederhana. **FL-002 merekomendasikan alternatifnya** (lihat D-019); keputusan asli belum dicabut. |
| D-011 | USULAN (direvisi FL-002) | Database **PostgreSQL lokal**, dengan **SQLite** sebagai alternatif untuk mempermudah MVP di Windows. **FL-002 merekomendasikan SQLite lebih dahulu** (lihat D-020); keputusan asli belum dicabut. |
| D-012 | USULAN | Model awal: OpenCV Zoo **YuNet + SFace**, setelah pemeriksaan ketentuan model dan pengujian lokal. |
| D-013 | USULAN | Operator/admin sebagai satu-satunya pengguna aplikasi wajib di MVP; akun peserta opsional. Perangkat kamera memakai identitas terpisah. |
| D-014 | USULAN | Kios kamera **satu orang per pemindaian**, bukan mengenali seluruh ruangan. Kandidat ESP32-S3 CAM. |
| D-015 | USULAN | Menyimpan template/embedding terenkripsi, tanpa menyimpan foto mentah secara default; menyediakan penghapusan dan absensi manual. |
| D-016 | DITUNDA | Cloud, sinkronisasi luar jaringan, multiorganisasi, integrasi HRIS, billing, CCTV terus-menerus, dan rilis native iOS berbayar. |
| D-017 | TERBUKA | Lisensi kode asli serta pembagian kepemilikan karya antaranggota tim. Untuk sementara **tidak memilih `LICENSE`**. |
| D-018 | TERBUKA | Alamat dan kondisi repository GitHub perlu diperiksa sebelum ada perintah commit/push. **Telah diverifikasi pada FL-001/FL-001R** (lihat D-024). |
| D-019 | USULAN (FL-002) | **Satu backend Python + FastAPI**, modul `face_engine` memakai OpenCV di dalam proses yang sama. Menunggu keputusan pemilik antara D-010 dan D-019. Analisis: `ARCHITECTURE.md` §11.1. |
| D-020 | USULAN (FL-002) | **SQLite mode WAL** lebih dahulu, dengan skema portabel dan jalur migrasi ke PostgreSQL disiapkan. Menunggu keputusan pemilik antara D-011 dan D-020. Analisis: `ARCHITECTURE.md` §11.2. |
| D-021 | USULAN (FL-002) | **Polling interval dengan `ETag`** sebagai jalur komunikasi pertama ke React Native; WebSocket hanya bila kebutuhan seketika terbukti. Analisis: `ARCHITECTURE.md` §11.4. |
| D-022 | USULAN (FL-002) | **Enkripsi tingkat aplikasi AES-GCM per baris** untuk template biometrik (nonce per baris, `person_id`/`model_id` ikut diautentikasi), bukan enkripsi seluruh berkas database. Pengelolaan kunci masih terbuka (Q-16). Analisis: `ARCHITECTURE.md` §11.5. |
| D-023 | USULAN (FL-002) | **Kontrak `CameraAdapter` berbasis JPEG + metadata** sebagai satu-satunya jalur masuk gambar; ESP32 kelak menjadi klien pengunggah, **bukan host model AI**. Rencana migrasi: `ARCHITECTURE.md` §13. |
| D-024 | DISETUJUI (fakta) | Remote `origin` = `https://github.com/Ardian3D/FaceLink-Ai`. FL-001 telah di-merge ke `main` (`cd31ec0`) pada 15 Oktober 2026. Dicatat sebagai **fakta terverifikasi**, bukan keputusan desain. |
| D-025 | USULAN (FL-002) | **InsightFace/ArcFace (`buffalo_*`) tidak dipakai** pada MVP karena bobotnya berketentuan riset non-komersial, sedangkan YuNet+SFace berlisensi Apache-2.0. Ini menutup salah satu risiko D-017, tetapi `LICENSE` repository sendiri **tetap belum dipilih**. |

## Protokol perubahan keputusan

1. Jelaskan masalah dan pilihan yang tersedia.
2. Cantumkan biaya, risiko, dampak ke API/data/privasi, dan pengaruh terhadap roadmap.
3. Minta persetujuan pemilik project.
4. Perbarui status keputusan beserta tanggal dan alasan jika disetujui.
5. Perbarui `PROJECT_STATE.md` serta spesifikasi lain yang terpengaruh.

**Jangan diam-diam mengganti stack, menetapkan layanan berbayar, atau menambah fitur di luar lingkup.**

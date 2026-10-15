# Catatan Keputusan FaceLink AI

Dokumen ini memisahkan apa yang **sudah disepakati**, apa yang **masih merupakan usulan**, dan apa yang **ditunda**. Agent dilarang mengubah status usulan menjadi disetujui tanpa konfirmasi pemilik project.

**Status:** `DISETUJUI`, `USULAN`, `DITUNDA`, `TERBUKA`.

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
| D-010 | USULAN | Backend utama **Go + `chi`**, mesin CV lokal **Python + OpenCV**. Alternatif: satu backend Python yang lebih sederhana. |
| D-011 | USULAN | Database **PostgreSQL lokal**, dengan **SQLite** sebagai alternatif untuk mempermudah MVP di Windows. |
| D-012 | USULAN | Model awal: OpenCV Zoo **YuNet + SFace**, setelah pemeriksaan ketentuan model dan pengujian lokal. |
| D-013 | USULAN | Operator/admin sebagai satu-satunya pengguna aplikasi wajib di MVP; akun peserta opsional. Perangkat kamera memakai identitas terpisah. |
| D-014 | USULAN | Kios kamera **satu orang per pemindaian**, bukan mengenali seluruh ruangan. Kandidat ESP32-S3 CAM. |
| D-015 | USULAN | Menyimpan template/embedding terenkripsi, tanpa menyimpan foto mentah secara default; menyediakan penghapusan dan absensi manual. |
| D-016 | DITUNDA | Cloud, sinkronisasi luar jaringan, multiorganisasi, integrasi HRIS, billing, CCTV terus-menerus, dan rilis native iOS berbayar. |
| D-017 | TERBUKA | Lisensi kode asli serta pembagian kepemilikan karya antaranggota tim. Untuk sementara **tidak memilih `LICENSE`**. |
| D-018 | TERBUKA | Alamat dan kondisi repository GitHub perlu diperiksa sebelum ada perintah commit/push. |

## Protokol perubahan keputusan

1. Jelaskan masalah dan pilihan yang tersedia.
2. Cantumkan biaya, risiko, dampak ke API/data/privasi, dan pengaruh terhadap roadmap.
3. Minta persetujuan pemilik project.
4. Perbarui status keputusan beserta tanggal dan alasan jika disetujui.
5. Perbarui `PROJECT_STATE.md` serta spesifikasi lain yang terpengaruh.

**Jangan diam-diam mengganti stack, menetapkan layanan berbayar, atau menambah fitur di luar lingkup.**

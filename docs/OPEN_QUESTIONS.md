# Pertanyaan Terbuka Sebelum Implementasi

Belum semua rancangan merupakan keputusan final. Pertanyaan berikut dijawab saat fase terkait akan dikerjakan; pertanyaan fase hardware **tidak** menghambat review dokumen tahap awal.

| ID | Pertanyaan | Saran awal (belum disahkan) | Dibutuhkan |
|---|---|---|---|
| Q-01 | Backend Go+Python atau Python saja? | Go + `chi` untuk API, Python untuk AI; tetap pertimbangkan kesederhanaan | Fase 1 |
| Q-02 | PostgreSQL lokal atau SQLite? | PostgreSQL untuk konsistensi, SQLite jika setup Windows menjadi hambatan | Fase 1 |
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
| Q-14 | Pembaruan realtime dengan polling atau WebSocket? | Polling lebih sederhana untuk awal; tingkatkan setelah stabil | Fase 4 |
| Q-15 | Branding aplikasi dan nama paket Android? | FaceLink AI, dipastikan ketika app disiapkan | Fase 3 |

Setelah diputuskan, buat/perbarui entri di `DECISIONS.md` dan sesuaikan `PROJECT_STATE.md`.

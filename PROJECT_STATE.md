# Status Project FaceLink AI

**Tanggal acuan dokumentasi:** 15 Oktober 2026
**Versi dokumen:** 0.2 (rancangan berbahasa Indonesia)
**Status aktual:** **DOKUMENTASI FONDASI DI REPOSITORY — BELUM ADA IMPLEMENTASI TERBUKTI**.

Sejak FL-001, paket dokumentasi ini berada di repository Git lokal dan cabang kerja `docs/fl-001-foundation`, dengan remote `https://github.com/Ardian3D/FaceLink-Ai`. Keberadaan dokumen di Git **bukan** bukti implementasi. Jangan menyimpulkan backend, aplikasi Android, AI, firmware, atau alat fisik telah dibuat hanya dari keberadaan berkas ini.

| Bagian | Status | Bukti saat ini |
|---|---|---|
| Arah MVP dan batas biaya | SUDAH DIDISKUSIKAN | Dicatat pada `docs/DECISIONS.md` |
| Paket dokumentasi | SELESAI (draf 0.2) | 20 berkas Markdown dalam repository |
| Fondasi repository (FL-001) | SELESAI | `.gitignore` 237 pola; berkas wajib lengkap; commit dan push ke `docs/fl-001-foundation` |
| Pemeriksaan rahasia pra-commit | SELESAI | Pindai 15 pola rahasia/PII atas seluruh berkas: 0 temuan |
| Pemilihan backend final | MENUNGGU KEPUTUSAN | Go/Python masih perlu persetujuan |
| Database final | MENUNGGU KEPUTUSAN | PostgreSQL/SQLite belum dipilih |
| Backend dan autentikasi | BELUM DIMULAI | Belum ada bukti kode/uji |
| Mesin pengenalan wajah | BELUM DIMULAI | Belum ada model dan evaluasi |
| Aplikasi React Native/APK | BELUM DIMULAI | Belum ada aplikasi terpasang |
| Integrasi webcam end-to-end | BELUM DIMULAI | Belum ada uji lintas sistem |
| Firmware dan hardware ESP32 | DITUNDA | Baru kandidat perangkat |
| Pengujian sukarelawan | BELUM DIMULAI | Belum ada data uji |
| Google Cloud | DITUNDA | Hanya setelah MVP lokal selesai dan disetujui |

## Langkah terdekat yang diizinkan

1. Tinjau paket dokumentasi bahasa Indonesia bersama pemilik project, termasuk temuan audit FL-001 (lihat riwayat di bawah).
2. Selesaikan keputusan terbuka yang **menghambat fase berikutnya**, terutama Go/Python (D-010) dan PostgreSQL/SQLite (D-011) melalui FL-002.
3. Kerjakan satu tugas terukur sesuai `docs/BACKLOG.md` dengan izin pemilik; `.gitignore` dan pemeriksaan kerahasiaan sudah tersedia, tetapi **tetap** jalankan `git status --ignored` sebelum commit source/data pertama.
4. Jaga agar tidak ada rahasia, foto wajah nyata, embedding, atau ekspor database yang masuk Git pada setiap commit berikutnya.

## Aturan memperbarui berkas ini

Setiap akhir tugas implementasi: catat tanggal, branch/commit bila ada, berkas yang diubah, perintah pengujian, hasil yang benar-benar terlihat, masalah yang belum selesai, dan satu langkah berikutnya. Gunakan status **BELUM DIMULAI / BERJALAN / TERHAMBAT / SELESAI**, tanpa klaim progres berdasarkan dugaan.

## Riwayat ringkas

- **2026-10-15:** Persiapan dokumentasi fondasi dan kebutuhan small-scale MVP; belum ada implementasi yang diverifikasi.
- **2026-10-15:** Paket dokumentasi disusun ulang dalam bahasa Indonesia sebagai draf 0.2; status kode tetap belum diverifikasi.
- **2026-10-15 (FL-001):** Fondasi repository. **Perubahan berkas:** `.gitignore` baru di root; `PROJECT_STATE.md`, `docs/BACKLOG.md`, `docs/ARCHITECTURE.md`, `docs/DATA_MODEL.md`, `docs/TEST_PLAN.md` diperbarui. **Pengujian yang dijalankan:** pemeriksaan kelengkapan 20 dokumen wajib; pemeriksaan tautan Markdown relatif (0 tautan rusak); pindai 15 pola rahasia/PII atas seluruh berkas working tree (0 temuan); pemeriksaan struktur `.gitignore` (237 pola, 0 pola ganda, 0 pola yang mengabaikan `docs/`/`research/`/`*.md`). **Belum diuji:** tidak ada kode aplikasi, jadi tidak ada pengujian perangkat lunak. **Masalah terbuka:** penamaan/rentang ID fase pada `ROADMAP` masih perlu keputusan pemilik; lisensi project belum ditetapkan.

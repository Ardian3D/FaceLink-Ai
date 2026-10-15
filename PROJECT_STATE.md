# Status Project FaceLink AI

**Tanggal acuan dokumentasi:** 15 Oktober 2026
**Versi dokumen:** 0.2 (rancangan berbahasa Indonesia)
**Status aktual:** **DOKUMENTASI FONDASI + KEPUTUSAN ARSITEKTUR FL-002 — BELUM ADA IMPLEMENTASI TERBUKTI**.

FL-001 telah **di-merge ke `main`** oleh pemilik repository pada 15 Oktober 2026 (commit merge `cd31ec0`, PR #1); pekerjaan dokumentasi berikutnya dilakukan pada cabang `docs/fl-002-architecture` yang dibuat dari `origin/main`. Remote: `https://github.com/Ardian3D/FaceLink-Ai`. Keberadaan dokumen di Git **bukan** bukti implementasi. Jangan menyimpulkan backend, aplikasi Android, AI, firmware, atau alat fisik telah dibuat hanya dari keberadaan berkas ini.

| Bagian | Status | Bukti saat ini |
|---|---|---|
| Arah MVP dan batas biaya | SUDAH DIDISKUSIKAN | Dicatat pada `docs/DECISIONS.md` |
| Paket dokumentasi | SELESAI (draf 0.2) | 20 berkas Markdown dalam repository |
| Fondasi repository (FL-001) | SELESAI, SUDAH DI-MERGE | `.gitignore` 237 pola; berkas wajib lengkap; `dca341a` dipublikasikan lewat `docs/fl-001-pr` (`ca2cc75`) lalu di-merge ke `main` sebagai `cd31ec0` |
| Keputusan arsitektur (FL-002) | SELESAI SEBAGAI USULAN | `docs/ARCHITECTURE.md` §11–§14 dan D-019..D-025 di `docs/DECISIONS.md` — **semua USULAN, belum disetujui** |
| Pemeriksaan rahasia pra-commit | SELESAI | Pindai 15 pola rahasia/PII atas seluruh berkas: 0 temuan |
| Pemilihan backend final | MENUNGGU KEPUTUSAN | D-010 (Go+Python) bersaing dengan D-019 (Python+FastAPI); belum dipilih pemilik |
| Database final | MENUNGGU KEPUTUSAN | D-011 (PostgreSQL) bersaing dengan D-020 (SQLite WAL); belum dipilih pemilik |
| Backend dan autentikasi | BELUM DIMULAI | Belum ada bukti kode/uji |
| Mesin pengenalan wajah | BELUM DIMULAI | Belum ada model dan evaluasi |
| Aplikasi React Native/APK | BELUM DIMULAI | Belum ada aplikasi terpasang |
| Integrasi webcam end-to-end | BELUM DIMULAI | Belum ada uji lintas sistem |
| Firmware dan hardware ESP32 | DITUNDA | Baru kandidat perangkat; kontrak adapter sudah dirancang (D-023), firmware belum ditulis |
| Model pengenalan wajah | BELUM DIPUTUSKAN | Usulan tetap YuNet+SFace (D-012/D-025); ambang kecocokan belum ada angkanya (Q-07) |
| Pengujian sukarelawan | BELUM DIMULAI | Belum ada data uji |
| Google Cloud | DITUNDA | Hanya setelah MVP lokal selesai dan disetujui |

## Langkah terdekat yang diizinkan

1. **Putuskan dua konflik stack FL-002:** D-010 vs D-019 (bahasa backend) dan D-011 vs D-020 (database). Selama belum diputuskan, implementasi backend dan database belum boleh dimulai.
2. Putuskan arah komunikasi React Native (D-021) dan pengelolaan kunci enkripsi template (Q-16), karena keduanya membentuk antarmuka dan skema data awal.
3. Tinjau paket dokumentasi bahasa Indonesia bersama pemilik project, termasuk temuan audit FL-001 (lihat riwayat di bawah).
4. Kerjakan satu tugas terukur sesuai `docs/BACKLOG.md` dengan izin pemilik; `.gitignore` dan pemeriksaan kerahasiaan sudah tersedia, tetapi **tetap** jalankan `git status --ignored` sebelum commit source/data pertama.
5. Jaga agar tidak ada rahasia, foto wajah nyata, embedding, atau ekspor database yang masuk Git pada setiap commit berikutnya.

## Aturan memperbarui berkas ini

Setiap akhir tugas implementasi: catat tanggal, branch/commit bila ada, berkas yang diubah, perintah pengujian, hasil yang benar-benar terlihat, masalah yang belum selesai, dan satu langkah berikutnya. Gunakan status **BELUM DIMULAI / BERJALAN / TERHAMBAT / SELESAI**, tanpa klaim progres berdasarkan dugaan.

## Riwayat ringkas

- **2026-10-15:** Persiapan dokumentasi fondasi dan kebutuhan small-scale MVP; belum ada implementasi yang diverifikasi.
- **2026-10-15:** Paket dokumentasi disusun ulang dalam bahasa Indonesia sebagai draf 0.2; status kode tetap belum diverifikasi.
- **2026-10-15 (FL-001):** Fondasi repository. **Perubahan berkas:** `.gitignore` baru di root; `PROJECT_STATE.md`, `docs/BACKLOG.md`, `docs/ARCHITECTURE.md`, `docs/DATA_MODEL.md`, `docs/TEST_PLAN.md` diperbarui. **Pengujian yang dijalankan:** pemeriksaan kelengkapan 20 dokumen wajib; pemeriksaan tautan Markdown relatif (0 tautan rusak); pindai 15 pola rahasia/PII atas seluruh berkas working tree (0 temuan); pemeriksaan struktur `.gitignore` (237 pola, 0 pola ganda, 0 pola yang mengabaikan `docs/`/`research/`/`*.md`). **Belum diuji:** tidak ada kode aplikasi, jadi tidak ada pengujian perangkat lunak. **Masalah terbuka:** penamaan/rentang ID fase pada `ROADMAP` masih perlu keputusan pemilik; lisensi project belum ditetapkan.
- **2026-10-15 (FL-001R):** Perbaikan leluhur cabang Git. `docs/fl-001-pr` dibuat dari `origin/main`, isi FL-001 dipindahkan pada tingkat objek Git, commit `ca2cc75` dipush. **Bukti:** lima pemeriksaan leluhur lolos; `git merge-tree` keluar 0 tanpa konflik. `main` dan `docs/fl-001-foundation` tidak disentuh.
- **2026-10-15 (merge FL-001):** PR #1 di-merge pemilik repository menjadi commit `cd31ec0`. Referensi remote usang dibersihkan oleh `git fetch --prune`.
- **2026-10-15 (FL-002):** Keputusan stack dan arsitektur MVP, **tahap dokumentasi saja**. **Berkas yang diubah:** `docs/ARCHITECTURE.md` (§9 diperbarui, §11–§14 baru), `docs/DECISIONS.md` (D-010/D-011 ditandai direvisi, D-019..D-025 baru), `docs/OPEN_QUESTIONS.md` (Q-01/Q-02/Q-14 diberi rekomendasi, Q-16..Q-18 baru), `PROJECT_STATE.md`. **Pengujian yang dijalankan:** kelengkapan dokumen, tautan Markdown relatif, pindai pola rahasia/PII, `git status --ignored`, dan `git diff --cached`. **Belum diuji:** FL-002 tidak menulis kode aplikasi apa pun, sehingga **tidak ada** hasil pengujian perangkat lunak, pengukuran model, atau benchmark database yang boleh diklaim. **Masalah terbuka:** dua konflik keputusan (D-010/D-019 dan D-011/D-020) menunggu pemilik; `LICENSE` belum dipilih; angka ambang matching belum ada.

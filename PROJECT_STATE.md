# Status Project FaceLink AI

**Tanggal acuan dokumentasi:** 15 Oktober 2026
**Versi dokumen:** 0.3 (rancangan berbahasa Indonesia)
**Status aktual:** **DOKUMENTASI FONDASI + KEPUTUSAN ARSITEKTUR FL-003 — BELUM ADA IMPLEMENTASI TERBUKTI**.

FL-001 telah **di-merge ke `main`** oleh pemilik repository pada 15 Oktober 2026 (commit merge `cd31ec0`, PR #1); FL-002 di-merge sebagai `dee62b4` (PR #2). Pekerjaan FL-003 dilakukan pada cabang `docs/fl-003-api-database-contract` yang dibuat dari `origin/main`; hasilnya **belum di-merge** dan masih menunggu tinjauan pemilik project. Remote: `https://github.com/Ardian3D/FaceLink-Ai`. Keberadaan dokumen di Git **bukan** bukti implementasi. Jangan menyimpulkan backend, aplikasi Android, AI, firmware, atau alat fisik telah dibuat hanya dari keberadaan berkas ini.

| Bagian | Status | Bukti saat ini |
|---|---|---|
| Arah MVP dan batas biaya | SUDAH DIDISKUSIKAN | Dicatat pada `docs/DECISIONS.md` |
| Paket dokumentasi | SELESAI (draf 0.3) | 20 berkas Markdown dalam repository |
| Fondasi repository (FL-001) | SELESAI, SUDAH DI-MERGE | `.gitignore` 237 pola; berkas wajib lengkap; `dca341a` dipublikasikan lewat `docs/fl-001-pr` (`ca2cc75`) lalu di-merge ke `main` sebagai `cd31ec0` |
| Keputusan arsitektur (FL-002) | SELESAI, DIREVISI OLEH FL-003 | `docs/ARCHITECTURE.md` §11–§14 berisi **analisis**; status keputusan yang berlaku ada di `docs/DECISIONS.md` |
| Keputusan arsitektur (FL-003) | **DISETUJUI PEMILIK, MENUNGGU TINJAUAN PR** | D-010/D-012/D-020/D-021/D-022/D-023/D-025/D-026/D-027/D-028 **DISETUJUI**; D-019 **DITOLAK**; D-011 **DITUNDA**. Mencakup `docs/API_CONTRACT.md` v1.0, `docs/DATA_MODEL.md` (15 tabel + SQLite WAL + migrasi berversi), `docs/SECURITY_PRIVACY.md` §9, dan §6/§10 kontrak internal |
| Pemeriksaan rahasia pra-commit | SELESAI | Pindai 15 pola rahasia/PII atas seluruh berkas: 0 temuan |
| Pemilihan backend final | **DIPUTUSKAN** | **Go + `chi`** sebagai backend utama, **Python** sebagai mesin AI terpisah (D-010 **DISETUJUI**). D-019 (Python+FastAPI tunggal) **DITOLAK** |
| Database final | **DIPUTUSKAN** | **SQLite mode WAL** untuk MVP (D-020 **DISETUJUI**). D-011 (PostgreSQL) **DITUNDA**, dengan skema tetap portabel |
| Backend dan autentikasi | BELUM DIMULAI | Belum ada bukti kode/uji |
| Mesin pengenalan wajah | BELUM DIMULAI | Belum ada model dan evaluasi |
| Aplikasi React Native/APK | BELUM DIMULAI | Belum ada aplikasi terpasang |
| Integrasi webcam end-to-end | BELUM DIMULAI | Belum ada uji lintas sistem |
| Firmware dan hardware ESP32 | DITUNDA | Baru kandidat perangkat; kontrak adapter sudah dirancang (D-023), firmware belum ditulis |
| Model pengenalan wajah | **DIPUTUSKAN, BELUM DIUJI** | YuNet + SFace final untuk MVP (D-012 **DISETUJUI**, D-025 menolak bobot non-permisif); **ambang kecocokan belum ada angkanya** dan wajib dikalibrasi (Q-07) |
| Kunci enkripsi template | BELUM DIPUTUSKAN | Skema penyimpanan final (AES-256-GCM per baris, D-022), tetapi pengelolaan kunci/rotasi/pemulihan tetap Q-16 |
| Pengujian sukarelawan | BELUM DIMULAI | Belum ada data uji |
| Google Cloud | DITUNDA | Hanya setelah MVP lokal selesai dan disetujui |

## Langkah terdekat yang diizinkan

1. **Tinjau PR FL-003.** Commit dan push ke cabang `docs/fl-003-api-database-contract` sudah dilakukan atas instruksi pemilik; yang belum dilakukan adalah **merge ke `main`**. PR tidak di-merge otomatis dan menunggu keputusan pemilik project.
2. Putuskan pengelolaan kunci enkripsi template (Q-16): lokasi kunci, rotasi, dan pemulihan. Ini menghambat implementasi `face_templates` dan tidak boleh dikira-kira.
3. Tetapkan angka ambang kecocokan YuNet+SFace lewat pengukuran pada peserta uji (Q-07); jangan menyalin angka dari contoh mana pun.
4. Kerjakan **satu** tugas implementasi terukur sesuai `docs/BACKLOG.md` setelah kode diizinkan; `.gitignore` dan pemeriksaan kerahasiaan sudah tersedia, tetapi **tetap** jalankan `git status --ignored` sebelum commit source/data pertama.
5. Jaga agar tidak ada rahasia, foto wajah nyata, embedding, atau ekspor database yang masuk Git pada setiap commit berikutnya.

> **Persetujuan FL-003 adalah persetujuan arsitektur, bukan izin implementasi.** Penulisan kode Go, mesin Python, aplikasi React Native, dan firmware tetap memerlukan tugas terpisah yang disetujui pemilik.

## Aturan memperbarui berkas ini

Setiap akhir tugas implementasi: catat tanggal, branch/commit bila ada, berkas yang diubah, perintah pengujian, hasil yang benar-benar terlihat, masalah yang belum selesai, dan satu langkah berikutnya. Gunakan status **BELUM DIMULAI / BERJALAN / TERHAMBAT / SELESAI**, tanpa klaim progres berdasarkan dugaan.

## Riwayat ringkas

- **2026-10-15:** Persiapan dokumentasi fondasi dan kebutuhan small-scale MVP; belum ada implementasi yang diverifikasi.
- **2026-10-15:** Paket dokumentasi disusun ulang dalam bahasa Indonesia sebagai draf 0.2; status kode tetap belum diverifikasi.
- **2026-10-15 (FL-001):** Fondasi repository. **Perubahan berkas:** `.gitignore` baru di root; `PROJECT_STATE.md`, `docs/BACKLOG.md`, `docs/ARCHITECTURE.md`, `docs/DATA_MODEL.md`, `docs/TEST_PLAN.md` diperbarui. **Pengujian yang dijalankan:** pemeriksaan kelengkapan 20 dokumen wajib; pemeriksaan tautan Markdown relatif (0 tautan rusak); pindai 15 pola rahasia/PII atas seluruh berkas working tree (0 temuan); pemeriksaan struktur `.gitignore` (237 pola, 0 pola ganda, 0 pola yang mengabaikan `docs/`/`research/`/`*.md`). **Belum diuji:** tidak ada kode aplikasi, jadi tidak ada pengujian perangkat lunak. **Masalah terbuka:** penamaan/rentang ID fase pada `ROADMAP` masih perlu keputusan pemilik; lisensi project belum ditetapkan.
- **2026-10-15 (FL-001R):** Perbaikan leluhur cabang Git. `docs/fl-001-pr` dibuat dari `origin/main`, isi FL-001 dipindahkan pada tingkat objek Git, commit `ca2cc75` dipush. **Bukti:** lima pemeriksaan leluhur lolos; `git merge-tree` keluar 0 tanpa konflik. `main` dan `docs/fl-001-foundation` tidak disentuh.
- **2026-10-15 (merge FL-001):** PR #1 di-merge pemilik repository menjadi commit `cd31ec0`. Referensi remote usang dibersihkan oleh `git fetch --prune`.
- **2026-10-15 (FL-002):** Keputusan stack dan arsitektur MVP, **tahap dokumentasi saja**. **Berkas yang diubah:** `docs/ARCHITECTURE.md` (§9 diperbarui, §11–§14 baru), `docs/DECISIONS.md` (D-010/D-011 ditandai direvisi, D-019..D-025 baru), `docs/OPEN_QUESTIONS.md` (Q-01/Q-02/Q-14 diberi rekomendasi, Q-16..Q-18 baru), `PROJECT_STATE.md`. **Pengujian yang dijalankan:** kelengkapan dokumen, tautan Markdown relatif, pindai pola rahasia/PII, `git status --ignored`, dan `git diff --cached`. **Belum diuji:** FL-002 tidak menulis kode aplikasi apa pun, sehingga **tidak ada** hasil pengujian perangkat lunak, pengukuran model, atau benchmark database yang boleh diklaim. **Masalah terbuka:** dua konflik keputusan (D-010/D-019 dan D-011/D-020) menunggu pemilik; `LICENSE` belum dipilih; angka ambang matching belum ada.
- **2026-10-15 (FL-003):** Penetapan stack final dan kontrak API/database, **tahap dokumentasi saja**. **Berkas yang diubah:** `docs/API_CONTRACT.md` (spesifikasi v1.0: 13 modul, kontrak internal Go↔Python §6, tabel 10 `outcome` §7, idempotensi §8, pengujian kontrak §9), `docs/DATA_MODEL.md` (15 tabel, konvensi penamaan indeks/unique/check, §7 SQLite WAL, §8 driver SQLite Go, §9 migrasi SQL berversi), `docs/DECISIONS.md` (catatan FL-003; D-010/D-012/D-020/D-021/D-022/D-023/D-025 **DISETUJUI**, D-019 **DITOLAK**, D-011 **DITUNDA**, D-026..D-028 baru; tabel dampak terhadap keputusan lama), `PROJECT_STATE.md`, `docs/BACKLOG.md`, `docs/OPEN_QUESTIONS.md`, `README.md`. **Pengujian yang dijalankan (15 Oktober 2026, tahap final review):** kelengkapan 20 dokumen wajib (20/20 ada, 0 hilang); tautan Markdown relatif 21 diperiksa, 0 rusak; pindai 15 pola rahasia/PII atas 20 dokumen, 0 temuan; pemeriksaan referensi ID keputusan `D-001`–`D-029` dan `Q-01`–`Q-18`, 0 rujukan tanpa definisi; pencarian sisa kontradiksi backend Python-only dan rujukan palsu ke spesifikasi kunci, 0 temuan; `git status --porcelain=v1 --branch`. **Belum diuji:** FL-003 tidak menulis kode aplikasi apa pun, jadi **tidak ada** hasil pengujian perangkat lunak, pengukuran model, atau tolok ukur database yang boleh diklaim. Ambang kecocokan belum ditetapkan (Q-07) dan pengelolaan kunci enkripsi belum diputuskan (Q-16). **Masalah terbuka:** `docs/SECURITY_PRIVACY.md` §5 kini menyatakan aturan kunci **dan menandai Q-16 sebagai penghambat implementasi**; dokumen itu **tidak** memuat kebijakan kunci, sehingga langkah berikutnya tetap keputusan pemilik project. Audit keselarasan AAD (`DECISIONS.md` D-022 vs `DATA_MODEL.md` §4.12) **sudah diselesaikan**: keduanya kini memakai `person_id` + `model_id` + `model_version` + `key_version`.

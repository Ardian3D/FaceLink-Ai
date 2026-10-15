# AGENTS.md — Aturan Kerja AI Coding FaceLink AI

Berlaku untuk **OmO, `deepseek-v4.1-flash`, Codex, dan setiap AI coding agent berikutnya**. Dokumen ini tidak bergantung pada model maupun penyedia API tertentu.

## 1. Prosedur wajib sebelum bekerja

1. Baca `README.md`, `PROJECT_STATE.md`, `docs/PRD.md`, `docs/DECISIONS.md`, `docs/ARCHITECTURE.md`, dan `docs/SECURITY_PRIVACY.md`. Baca pula spesifikasi terkait tugas yang diminta.
2. Periksa **keadaan repository nyata**: branch aktif, perubahan yang belum di-commit, struktur folder, dependensi, serta pengujian tersedia.
3. Jangan menganggap fitur sudah ada hanya karena ditulis dalam PRD, roadmap, atau jawaban AI sebelumnya.
4. Jelaskan ringkas: tujuan tugas, batas perubahan, berkas yang akan disentuh, keputusan terbuka, risiko, dan bukti pengujian yang direncanakan.
5. Bila tugas belum disetujui atau bertentangan dengan keputusan terkonfirmasi, **jangan langsung mengubah kode**. Ajukan pertanyaan spesifik.
6. Kerjakan satu tugas terukur dalam satu waktu; jangan mencoba membangun seluruh FaceLink AI sekaligus.

## 2. Aturan yang tidak boleh dilanggar

- MVP hanya untuk **demo satu kelas, satu terminal**, bukan sistem pengawasan massal, perusahaan multitenant, atau layanan cloud sejak awal.
- Gunakan perangkat lunak, pustaka, AI lokal, dan database **tanpa biaya langganan atau API berbayar**. Tidak boleh mengaktifkan trial, kartu pembayaran, atau fallback berbayar tanpa persetujuan eksplisit.
- **Jangan integrasikan Google Cloud** sebelum MVP lokal selesai divalidasi dan pengembang meminta secara eksplisit.
- Hardware dan firmware ESP32 baru dikerjakan di fase IoT; pada tahap software gunakan antarmuka kamera abstrak, webcam, atau data uji sintetis.
- **Jangan** mengirim foto wajah, embedding, identitas peserta, kata sandi, atau riwayat kehadiran yang dapat diidentifikasi ke LLM, layanan telemetry, atau repository publik.
- Enrollment biometrik harus dilakukan berdasarkan persetujuan sukarela yang tercatat. Harus tersedia alternatif absensi manual.
- Face recognition menghasilkan **estimasi probabilistik**, bukan kepastian identitas. Kasus wajah tidak terdeteksi, kualitas buruk, lebih dari satu wajah, cocok ganda, atau keyakinan rendah **tidak boleh menghasilkan absensi otomatis**.
- Toleransi kacamata merupakan target uji, bukan jaminan. Kedipan/gerakan kepala bukan bukti anti-spoofing kuat.
- Mesin AI lokal tidak boleh terbuka bebas pada jaringan. Batasi ke `127.0.0.1`/IPC atau gunakan autentikasi internal yang kuat.
- Jangan push langsung ke `main`, force push, menghapus data, mengubah lisensi, atau mengubah skema secara destruktif tanpa izin.

## 3. Standar rekayasa

- Utamakan desain sederhana dan dapat dipelihara. Kandidat Go backend modular + pekerja AI Python adalah **usulan**, bukan keputusan final.
- Sepakati kontrak API, aturan domain, struktur data, dan migrasi sebelum mengimplementasikan konsumennya.
- Gunakan TypeScript ketat, identitas UUID, waktu penyimpanan UTC, kesalahan API yang konsisten, dan pengamanan absensi ganda pada tingkat database.
- Alamat backend di aplikasi harus dapat dikonfigurasi, bukan IP laptop yang ditanam permanen.
- Terapkan autentikasi dan otorisasi untuk operator, kredensial terpisah untuk terminal IoT, validasi input gambar, pembatasan ukuran permintaan, dan batas waktu pemrosesan.
- Jangan pernah mencetak JPEG mentah, embedding, kunci, token, atau detail biometrik ke log.
- Simpan versi serta checksum model AI dan hasil evaluasi. Jangan mengasumsikan ambang contoh model cocok untuk peserta kita.
- Tambahkan pengujian kasus sukses **dan gagal**: tidak terdaftar, izin dicabut, sesi tutup, input rusak, duplikasi simultan, model timeout, dan perbaikan data manual.
- Jangan menulis klaim “berfungsi”, “lulus pengujian”, atau “teruji akurat” tanpa hasil nyata yang dapat diulang.

## 4. Definisi tugas selesai

Tugas berstatus **SELESAI** hanya jika:

- Kebutuhan dan kriteria penerimaan yang disetujui benar-benar terimplementasi.
- Perintah pengujian dijalankan, hasil asli dicatat, dan kegagalan diungkapkan.
- Keamanan/privasi pada perubahan yang relevan sudah diperiksa.
- Dokumentasi terkait (termasuk `PROJECT_STATE.md`) diperbarui sesuai fakta.
- Tidak ada rahasia, data wajah nyata, atau artefak model dengan lisensi tak jelas yang masuk commit.
- Agent memberikan ringkasan berkas yang berubah, bukti uji, keterbatasan, dan langkah berikutnya.

## 5. Urutan otoritas informasi

1. Instruksi terbaru yang dinyatakan langsung oleh pemilik project.
2. `docs/DECISIONS.md` dengan status **DISETUJUI**.
3. `docs/PRD.md` dan persyaratan `docs/SECURITY_PRIVACY.md`.
4. Kontrak API, model data, dan tugas implementasi yang telah ditinjau.
5. Usulan arsitektur, roadmap, backlog, atau opini agent.

Jika sumber bertentangan, **laporkan konflik dan minta keputusan** sebelum mengubah bagian yang tidak dapat dipulihkan.

## 6. Penggunaan OmO dan pergantian model

OmO hanya alat pengembangan, **bukan komponen yang dibutuhkan FaceLink AI saat dipakai pengguna**. Model utama yang direncanakan adalah `deepseek-v4.1-flash`, dengan syarat dukungan penggunaan tool, edit berkas, dan kuota tanpa biaya sudah diuji. Konfigurasi tidak boleh otomatis berpindah ke model berbayar. Ketika model berganti, agent baru wajib membaca repository dan status terkini, bukan mengandalkan konteks chat sebelumnya.

## 7. Format laporan akhir agent

Gunakan bahasa Indonesia dengan struktur: **Tugas → Perubahan → Berkas → Pengujian dan hasil → Masalah/risiko → Keputusan yang dibutuhkan → Rekomendasi tugas berikutnya**.

# Panduan Memulai AI Coding Agent

Tujuan dokumen ini adalah membantu **OmO maupun model lain** melanjutkan project tanpa bergantung pada histori percakapan. Semua prompt di bawah berbahasa Indonesia dan sengaja dibuat **tidak bergantung pada model**.

## A. Saat agent baru pertama kali membuka repository (hanya baca)

> Kamu bertugas sebagai AI coding agent untuk FaceLink AI. Jangan mengubah berkas atau menjalankan perintah destruktif dulu. Baca `AGENTS.md`, `PROJECT_STATE.md`, `docs/PRD.md`, `docs/DECISIONS.md`, `docs/ARCHITECTURE.md`, dan `docs/SECURITY_PRIVACY.md`. Setelah itu periksa keadaan repository sebenarnya: branch, diff, struktur kode, dependensi, dan pengujian. Buat ringkasan berbahasa Indonesia tentang fitur yang benar-benar terimplementasi, bukti pengujian yang tersedia, keputusan yang masih terbuka, risiko, serta tiga tugas kecil berikutnya. Jangan menganggap dokumentasi sebagai bukti fitur telah dibuat.

## B. Saat diberi satu tugas yang sudah disetujui

> Kerjakan hanya tugas **[ID dan judul tugas]**. Baca dokumen terkait dan sampaikan rencana, daftar berkas yang mungkin diubah, kriteria penerimaan, dan pengujian terlebih dahulu. Jangan memperluas ruang lingkup, mengganti stack, menggunakan layanan berbayar, atau mengirim biometrik ke API eksternal. Setelah disetujui, implementasikan, jalankan pengujian yang benar-benar ada, lalu laporkan hasil aktual, keterbatasan, dan perbarui `PROJECT_STATE.md` serta keputusan terkait bila diperlukan. Jangan push ke `main` tanpa persetujuan.

## C. Saat agent lain diminta melakukan audit/review

> Tinjau perubahan terbaru pada branch ini secara **hanya baca**. Prioritaskan bug nyata, pelanggaran PRD, keamanan data wajah, salah otorisasi, absensi ganda, kebocoran rahasia, konflik kontrak API, dan tes yang tidak membuktikan klaim. Berikan temuan berdasarkan tingkat dampak dengan referensi berkas/baris bila tersedia. Jangan membuat perubahan kode sebelum diminta.

## D. Saat melakukan pergantian model atau sesi

> Siapkan laporan serah-terima berdasarkan keadaan Git saat ini, bukan hanya ringkasan chat. Sertakan: branch/commit, tujuan tugas, perubahan yang dilakukan, pengujian beserta hasil nyata, dokumen yang diperbarui, keputusan terbuka, bug/blokir, serta satu tindakan berikutnya yang aman. Perbarui `PROJECT_STATE.md` untuk fakta yang sudah diverifikasi. Hindari klaim “selesai” tanpa bukti.

## E. Ketentuan khusus OmO

- Gunakan OmO sebagai alat orkestrasi saja; model yang direncanakan `deepseek-v4.1-flash` **bukan komponen runtime aplikasi**.
- Periksa dahulu apakah endpoint model mendukung tool calling, pembacaan/pengeditan berkas, serta kuota gratis yang cukup; kehadiran model dalam daftar provider **bukan** bukti kompatibilitas.
- Nonaktifkan fallback yang dapat menimbulkan biaya; jangan mengirim foto wajah/embedding ke provider AI coding.
- Jika perintah spesifik OmO berubah antarrilis, ikuti dokumentasi versi yang digunakan. **Jangan menganggap contoh perintah sebagai API stabil**.
- Delegasikan pekerjaan yang independen, tetapi semua perubahan yang menyentuh skema, kontrak, atau keputusan utama tetap melalui review pemilik project.

## F. Aturan kerja ketika ada konflik dokumen

Gunakan urutan otoritas pada `AGENTS.md`. Jika contoh prompt berbeda dari PRD yang sudah disahkan, **PRD dan keputusan yang disetujui menang**. Ajukan pertanyaan dan jangan membuat desain baru diam-diam.

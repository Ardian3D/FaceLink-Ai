# PRD — Persyaratan Produk FaceLink AI

**Versi:** 0.2 · **Status:** Draf untuk ditinjau · **Produk:** Prototipe akademik berskala kecil  
**Penting:** PRD mendeskripsikan kebutuhan, **bukan** menyatakan bahwa fitur telah diimplementasikan.

## 1. Ringkasan produk

FaceLink AI adalah sistem absensi kelas berbasis pengenalan wajah yang menggabungkan terminal kamera, mesin AI lokal, backend, dan aplikasi React Native. Operator membuka sesi kelas; peserta sukarela yang sudah melakukan pendaftaran biometrik mendekati kamera; sistem mengenali peserta dengan tingkat keyakinan memadai dan mencatat kehadiran sekali pada sesi tersebut. Peserta yang tidak dapat/tidak ingin menggunakan biometrik mendapat jalur absensi manual yang setara.

**Prioritas produk:** demo IoT dan software yang berfungsi end-to-end, dapat diuji, memahami batas kemampuannya, dan tidak membuat klaim berlebihan mengenai keamanan atau akurasi.

## 2. Masalah dan peluang

Absensi manual dalam kegiatan kelas kecil bisa memakan waktu, rawan salah input, dan kurang praktis untuk direkap. Kamera dan AI lokal membuka kemungkinan pencatatan otomatis tanpa biaya API cloud. Namun identifikasi wajah dapat salah—khususnya dalam kondisi cahaya buruk, kacamata memantul, atau wajah tertutup. Karena itu solusi harus dapat ditinjau operator, bukan sepenuhnya dipercayai tanpa pengawasan.

## 3. Pengguna dan hak akses

| Peran | Kebutuhan | Hak awal |
|---|---|---|
| Operator/dosen | Membuka sesi, mendaftarkan peserta berizin, melihat daftar hadir, memperbaiki catatan, ekspor laporan | Akses terautentikasi ke fungsi administrasi kelas demo |
| Peserta/mahasiswa | Memberi/menarik persetujuan, ikut absensi, memilih jalur manual | Tidak perlu akun aplikasi pada MVP; fitur akun peserta hanya usulan lanjutan |
| Perangkat kamera | Mengirim gambar untuk sesi yang sah dan menerima status untuk LED/buzzer | Kredensial khusus perangkat, bukan akun operator |

## 4. Tujuan MVP

1. Memproses data **secara lokal** menggunakan laptop dan jaringan Wi-Fi yang tersedia.
2. Mendemonstrasikan pendaftaran wajah dengan persetujuan dan 3–10 sampel berkualitas untuk setiap peserta yang bersedia.
3. Mencatat otomatis tepat **satu** kehadiran per peserta per sesi pada hasil pengenalan yang memenuhi syarat.
4. Mendukung satu kelas dan satu terminal, kira-kira 5–10 sukarelawan dewasa.
5. Menampilkan sesi, status peserta, pembaruan hasil, dan rekap pada React Native Android; APK dapat dipasang langsung.
6. Memberikan ekspor CSV tanpa data biometrik, koreksi manual terlacak, dan penghapusan template biometrik.
7. Menguji kondisi tanpa kacamata, dengan kacamata bening, sudut dan cahaya berbeda, serta kasus sistem gagal.
8. Menjaga biaya perangkat lunak dan inferensi AI **Rp0**; anggaran perangkat keras maksimal Rp1.000.000.

## 5. Di luar lingkup MVP

Tidak membuat absensi serentak seluruh ruangan, CCTV pengawasan terus-menerus, deteksi emosi/atribut sensitif, pengenalan wajah tak dikenal untuk identifikasi umum, integrasi sistem akademik/HRIS, pembayaran, multi-tenant, cabang kantor, infrastruktur cloud, atau distribusi App Store/TestFlight berbayar. PWA iPhone boleh dipertimbangkan belakangan tanpa menghambat Android.

## 6. Alur pengguna utama

**A. Persiapan:** Operator masuk → membuat kelas demo → menambahkan peserta beridentitas contoh → meminta persetujuan → melakukan enrollment melalui webcam → menguji kualitas sampel → menyimpan template sesuai kebijakan.

**B. Absensi:** Operator membuat/membuka sesi → peserta menghadap kamera satu per satu → AI menilai kualitas dan kecocokan → backend memvalidasi sesi, persetujuan, keanggotaan, duplikasi → membuat catatan → UI menerima hasil.

**C. Penanganan gagal:** Tidak ada wajah/lebih dari satu/blur/kecocokan ambigu/model mati → tidak ada check-in otomatis → sistem memberi alasan aman dan opsi mengulang/verifikasi manual.

**D. Penutupan:** Operator menutup sesi → rekap dan daftar belum tercatat dihitung dari peserta kelas → ekspor CSV → koreksi manual bila diperlukan, disertai alasan.

**E. Penarikan persetujuan:** Operator memproses permintaan → template dan cache biometrik dihapus → jalur manual tetap tersedia → kebijakan data absensi nonbiometrik ditetapkan secara terbatas.

## 7. Kebutuhan fungsional dan prioritas

Prioritas: **P0 = harus ada untuk MVP**, **P1 = diutamakan bila waktu cukup**, **P2 = setelah demo utama**.

| ID | Prioritas | Kebutuhan | Kriteria penerimaan ringkas |
|---|---|---|---|
| FR-01 | P0 | Login operator | Kredensial benar masuk; salah ditolak; rute sensitif wajib autentikasi |
| FR-02 | P0 | Kelola orang dan keanggotaan satu kelas | Kode peserta demo unik; hanya anggota sesi yang dapat dicatat |
| FR-03 | P0 | Persetujuan biometrik | Tanpa persetujuan aktif tidak dapat enrollment/pengenalan |
| FR-04 | P0 | Enrollment berkualitas | 3–10 sampel terpilih, deteksi satu wajah dan feedback kegagalan |
| FR-05 | P0 | Sesi absensi | Status draf/buka/tutup dan batas keterlambatan yang terdefinisi |
| FR-06 | P0 | Pencocokan wajah lokal | Wajah asing/ambigu/kualitas rendah tidak mencatat kehadiran |
| FR-07 | P0 | Tepat satu catatan per orang per sesi | Percobaan bersamaan menghasilkan satu rekaman, bukan duplikasi |
| FR-08 | P0 | Absensi manual & koreksi | Hanya operator, wajib alasan dan riwayat audit |
| FR-09 | P0 | Dashboard daftar hadir | Menampilkan pembaruan sesi, status, dan waktu secara konsisten |
| FR-10 | P0 | Penghapusan data biometrik | Template terhapus dari penyimpanan dan indeks/cache pencocokan |
| FR-11 | P0 | Ekspor CSV | Tidak memuat foto, embedding, token, atau kata sandi |
| FR-12 | P0 | Sumber gambar webcam | Demo software lengkap sebelum hardware dibeli |
| FR-13 | P0 | APK Android | Terpasang dan dapat terhubung ke backend lokal pada HP uji |
| FR-14 | P1 | Indikator realtime | WebSocket atau polling dengan keadaan koneksi jelas |
| FR-15 | P1 | Tantangan liveness dasar | Diuji sebagai mitigasi terbatas, tidak diklaim anti-spoofing sempurna |
| FR-16 | P1 | Halaman peserta | Lihat status sendiri tanpa akses ke data biometrik peserta lain |
| FR-17 | P2 | Kamera ESP32-S3 dan indikator | Input kamera fisik menggantikan webcam tanpa mengubah aturan absensi |

## 8. Aturan bisnis dan keadaan data

- Orang terdaftar **belum berarti** boleh dikenali; persetujuan biometrik masih harus aktif.
- Sesi hanya dapat menerima absensi saat `open`. Sesudah `closed`, mesin kamera tidak dapat menambah kehadiran.
- Kehadiran `present` atau `late` ditentukan waktu check-in dan aturan sesi; `absent` adalah status turunan setelah sesi ditutup bagi anggota yang belum tercatat.
- Jika AI memberikan dua kandidat berdekatan atau kualitas di bawah ambang, keluaran harus **perlu pemeriksaan**, tanpa mengungkap tebakan identitas ke layar publik.
- Pelaku absensi ganda menerima `already_checked_in`; waktu kehadiran pertama tidak boleh tertimpa.
- Verifikasi manual tidak boleh dianggap lebih rendah atau menghukum orang yang menolak biometrik.
- Setiap koreksi memiliki operator, waktu, alasan, dan catatan perubahan.

## 9. Persyaratan nonfungsional

- **Biaya:** gratis tanpa pembayaran software/AI/cloud. Tidak mengandalkan trial sebagai komponen wajib.
- **Privasi:** data wajah diproses lokal, tidak dikirim ke AI coding agent, tidak dimasukkan Git, template dilindungi, izin dapat ditarik.
- **Keamanan:** autentikasi dan otorisasi, validasi JPEG, pembatasan ukuran dan laju, kredensial khusus terminal, log yang disaring.
- **Ketahanan:** bila AI/backend/Wi-Fi mati, tidak menciptakan catatan kehadiran palsu; operator diberi status gangguan.
- **Performa:** ukur latensi kamera→hasil dan latensi UI melalui pengujian; **tidak menetapkan angka performa sebagai klaim sebelum pengukuran**.
- **Aksesibilitas:** status tidak hanya ditunjukkan warna; teks umpan balik, ukuran sentuh memadai, dan jalur manual tersedia.
- **Portabilitas:** Windows sebagai lingkungan awal; kontrak kamera tidak mengunci jenis perangkat.

## 10. Definisi keberhasilan demo

Prototipe dinilai siap demo bila operator dapat melakukan alur lengkap: membuat kelas → mendapatkan persetujuan → enrollment beberapa sukarelawan → membuka sesi → absensi wajah yang jelas → menolak kasus salah/duplikat → absensi manual → menutup sesi → ekspor laporan → menjalankan APK pada perangkat uji. Semua hasil uji dan kelemahannya dicatat. Pengujian harus mencakup peserta berkacamata, tetapi keberhasilan tanpa kacamata **tidak cukup** untuk mengklaim berhasil pada seluruh jenis kacamata.

## 11. Risiko dan ketergantungan

- Salah kenal atau salah menolak dapat memengaruhi catatan kehadiran; sistem wajib gagal secara aman.
- Foto/video palsu dapat menipu kamera RGB; gunakan pengawasan operator dan tidak melebih-lebihkan liveness.
- Kacamata dan pencahayaan menurunkan mutu gambar; perlu kalibrasi, diffuser, dan pengujian.
- Model AI dan bobotnya mungkin mempunyai lisensi berbeda dari kode pustakanya; periksa sebelum dipaketkan.
- Perangkat ESP32-S3 bervariasi pada pin, PSRAM, dan driver; jangan membeli tanpa memeriksa model persis.
- Keputusan backend, database, akun mahasiswa, dan retensi data belum semuanya final. Lihat `OPEN_QUESTIONS.md`.

## 12. Batas persetujuan implementasi

PRD ini **masih draf**. Sebelum memulai kode untuk fitur tertentu, pemilik project menyetujui keputusan yang dibutuhkan fase tersebut. Jangan mengubah ruang lingkup atau mengaktifkan cloud atas inisiatif agent sendiri.

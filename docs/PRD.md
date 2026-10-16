# PRD — Persyaratan Produk FaceLink AI

**Versi:** 0.3 · **Status:** Draf untuk ditinjau · **Produk:** Prototipe akademik berskala kecil
**Revisi FL-010U (16 Oktober 2026):** diselaraskan dengan keputusan arsitektur tiga role manusia (`admin`, `dosen`, `mahasiswa`) yang masuk dengan **Google Sign-In**, dan `device` sebagai autentikasi mesin terpisah. Cabang kerja `docs/fl-010u-google-three-roles`, bercabang dari `origin/main` (`804c2f7`). Matriks izin lengkap ada di `docs/AUTHORIZATION.md` §2 dan §4.
**Penting:** PRD mendeskripsikan kebutuhan, **bukan** menyatakan bahwa fitur telah diimplementasikan.

## 1. Ringkasan produk

FaceLink AI adalah sistem absensi kelas berbasis pengenalan wajah yang menggabungkan terminal kamera, mesin AI lokal, backend, dan aplikasi React Native. Aplikasi React Native dipakai oleh **tiga role manusia** — `admin`, `dosen`, dan `mahasiswa` — yang semuanya masuk dengan **Google Sign-In**; tidak ada login email/kata sandi buatan sendiri. `admin` mengelola akun dan role, kelas beserta penugasan dosen, peserta, persetujuan biometrik, enrollment, perangkat, sesi, rekap, audit, dan ekspor; `dosen` membuka/menutup sesi dan menjalankan koreksi manual **hanya pada kelas yang ditugaskan kepadanya**; `mahasiswa` melihat kelas yang diikuti, jadwal sesi, status dan riwayat kehadiran, serta status persetujuannya **sendiri**. Terminal kamera bukan pengguna manusia: perangkat IoT memakai identitas dan autentikasi mesin terpisah (`device`), dan role tidak pernah ditentukan oleh Google maupun oleh klien.

Peserta sukarela yang sudah melakukan pendaftaran biometrik mendekati kamera; sistem mengenali peserta dengan tingkat keyakinan memadai dan mencatat kehadiran sekali pada sesi tersebut. Peserta yang tidak dapat/tidak ingin menggunakan biometrik mendapat jalur absensi manual yang setara.

**Prioritas produk:** demo IoT dan software yang berfungsi end-to-end, dapat diuji, memahami batas kemampuannya, dan tidak membuat klaim berlebihan mengenai keamanan atau akurasi.

## 2. Masalah dan peluang

Absensi manual dalam kegiatan kelas kecil bisa memakan waktu, rawan salah input, dan kurang praktis untuk direkap. Kamera dan AI lokal membuka kemungkinan pencatatan otomatis tanpa biaya API cloud. Namun identifikasi wajah dapat salah—khususnya dalam kondisi cahaya buruk, kacamata memantul, atau wajah tertutup. Karena itu solusi harus dapat ditinjau petugas (`admin`/`dosen`), bukan sepenuhnya dipercayai tanpa pengawasan.

## 3. Pengguna dan hak akses

Revisi FL-010U (16 Oktober 2026) menggantikan tabel pengguna pada versi sebelumnya. Dua keputusan lama **DIGANTIKAN**: (a) role `operator` digantikan oleh `dosen`; cakupan `operator` lama terbagi antara `admin` dan `dosen`, sehingga ini **bukan** penggantian nama dengan cakupan identik; (b) pernyataan lama "mahasiswa tidak memiliki akun pada MVP" digantikan oleh keputusan bahwa **akun `mahasiswa` ada**. Tiga role manusia masuk dengan **Google Sign-In**; Google **tidak** menentukan hak akses — role ditetapkan dan dikelola `admin` di backend FaceLink AI. `device` bukan role manusia.

| Aktor | Jenis | Kredensial | Hak awal |
|---|---|---|---|
| `admin` | Manusia | Google Sign-In | Seluruh sistem: akun dan role, penautan akun–peserta, peserta, persetujuan biometrik, enrollment, kelas + penugasan dosen, perangkat, sesi, rekap, ekspor CSV, audit |
| `dosen` | Manusia | Google Sign-In | Hanya kelas yang ditugaskan kepadanya: baca peserta kelas itu, membuat/membuka/menutup sesi, koreksi manual dengan alasan, rekap sesinya |
| `mahasiswa` | Manusia | Google Sign-In | Hanya data sendiri: kelas yang diikuti, jadwal sesi absensi kelas yang diikuti, status dan riwayat kehadiran sendiri, status persetujuan sendiri (baca saja) |
| `device` | Mesin | Token perangkat (bukan Google) | Hanya `POST /devices/{device_id}/recognition-attempts` |

Empat endpoint portal self-service (`GET /me/classes`, `GET /me/schedule`, `GET /me/attendance`, `GET /me/consent`) pada akun `mahasiswa` hanya dapat dipakai bila akun itu sudah ditautkan ke baris peserta; akun yang belum tertaut menerima `409 ACCOUNT_NOT_LINKED` (`GET /me/classes` juga tersedia bagi `dosen`, yang menerima hanya kelas yang ditugaskan). Otorisasi selalu diputuskan di backend Go dengan prinsip deny by default dan least privilege, memakai pemeriksaan kepemilikan/cakupan kelas serta anti-enumeration; menyembunyikan menu di aplikasi **bukan** pengendalian akses. Matriks izin per endpoint ada di `docs/AUTHORIZATION.md` §2 dan §4, dan endpoint yang tidak tercantum di sana tidak boleh dibuat. Hak `mahasiswa` atas riwayatnya sendiri **tidak** memberi akses ke template biometrik, embedding, atau data peserta lain.

## 4. Tujuan MVP

1. Memproses data **secara lokal** menggunakan laptop dan jaringan Wi-Fi yang tersedia.
2. Mendemonstrasikan pendaftaran wajah dengan persetujuan dan 3–10 sampel berkualitas untuk setiap peserta yang bersedia.
3. Mencatat otomatis tepat **satu** kehadiran per peserta per sesi pada hasil pengenalan yang memenuhi syarat.
4. Mendukung satu kelas dan satu terminal, kira-kira 5–10 sukarelawan dewasa.
5. Menampilkan sesi, status peserta, pembaruan hasil, dan rekap pada React Native Android untuk tiga role (`admin`, `dosen`, `mahasiswa`) yang masuk dengan Google Sign-In; APK dapat dipasang langsung.
6. Memberikan ekspor CSV tanpa data biometrik, koreksi manual terlacak, dan penghapusan template biometrik.
7. Menguji kondisi tanpa kacamata, dengan kacamata bening, sudut dan cahaya berbeda, serta kasus sistem gagal.
8. Menjaga biaya perangkat lunak dan inferensi AI **Rp0**; anggaran perangkat keras maksimal Rp1.000.000.

## 5. Di luar lingkup MVP

Tidak membuat absensi serentak seluruh ruangan, CCTV pengawasan terus-menerus, deteksi emosi/atribut sensitif, pengenalan wajah tak dikenal untuk identifikasi umum, integrasi sistem akademik/HRIS, pembayaran, multi-tenant, cabang kantor, infrastruktur cloud, atau distribusi App Store/TestFlight berbayar. PWA iPhone boleh dipertimbangkan belakangan tanpa menghambat Android.

## 6. Alur pengguna utama

**A. Persiapan (oleh `admin`):** `admin` masuk dengan Google Sign-In → mengelola akun dan role → membuat kelas demo dan menugaskan `dosen` pada kelas itu → menautkan akun `mahasiswa` ke baris peserta → menambahkan peserta beridentitas contoh → meminta persetujuan → melakukan enrollment melalui webcam → menguji kualitas sampel → menyimpan template sesuai kebijakan. Enrollment dan pencatatan persetujuan pada revisi ini berada pada jalur `admin`.

**B. Absensi (oleh `dosen`):** `dosen` masuk dengan Google Sign-In dan hanya melihat kelas yang ditugaskan → membuat/membuka sesi kelas itu → peserta menghadap kamera satu per satu → AI menilai kualitas dan kecocokan → backend memvalidasi sesi, persetujuan, keanggotaan, cakupan kelas, dan duplikasi → membuat catatan → UI menerima hasil.

**C. Penanganan gagal:** Tidak ada wajah/lebih dari satu/blur/kecocokan ambigu/model mati → tidak ada check-in otomatis → sistem memberi alasan aman dan opsi mengulang/verifikasi manual oleh petugas (`admin` atau `dosen` kelas tersebut).

**D. Penutupan:** `dosen` menutup sesi → rekap dan daftar belum tercatat dihitung dari peserta kelas → ekspor CSV oleh `admin` → koreksi manual bila diperlukan oleh `admin`/`dosen` pada kelasnya, disertai alasan.

**E. Penarikan persetujuan:** `admin` memproses permintaan → template dan cache biometrik dihapus → jalur manual tetap tersedia → kebijakan data absensi nonbiometrik ditetapkan secara terbatas. Penarikan konsen oleh mahasiswa sendiri dari aplikasi belum dirancang (lihat `docs/OPEN_QUESTIONS.md`).

**F. Mahasiswa melihat datanya sendiri:** `mahasiswa` masuk dengan Google Sign-In → beranda pribadi: kelas yang diikuti, jadwal sesi absensi kelas yang diikuti, status dan riwayat kehadiran sendiri, serta status persetujuan sendiri (baca saja). Akun `mahasiswa` yang belum ditautkan ke baris peserta menerima `409 ACCOUNT_NOT_LINKED`; `mahasiswa` tidak melihat data peserta lain dan tidak dapat mengubah catatan kehadiran.

## 7. Kebutuhan fungsional dan prioritas

Prioritas: **P0 = harus ada untuk MVP**, **P1 = diutamakan bila waktu cukup**, **P2 = setelah demo utama**.

| ID | Prioritas | Kebutuhan | Kriteria penerimaan ringkas |
|---|---|---|---|
| FR-01 | P0 | Login dengan Google Sign-In | ID token Google diverifikasi menurut OpenID Connect (tanda tangan lewat JWKS, `alg` RS256, `iss`, `aud`, `exp`/`iat`, `nonce`, `email_verified`); akun tanpa akses, `invited`, atau `suspended` ditolak dengan pesan generik; sesi aplikasi diterbitkan backend; rute sensitif wajib autentikasi |
| FR-02 | P0 | Kelola orang dan keanggotaan satu kelas | Kode peserta demo unik; hanya anggota sesi yang dapat dicatat |
| FR-03 | P0 | Persetujuan biometrik | Tanpa persetujuan aktif tidak dapat enrollment/pengenalan |
| FR-04 | P0 | Enrollment berkualitas | 3–10 sampel terpilih, deteksi satu wajah dan feedback kegagalan |
| FR-05 | P0 | Sesi absensi | Status draf/buka/tutup dan batas keterlambatan yang terdefinisi |
| FR-06 | P0 | Pencocokan wajah lokal | Wajah asing/ambigu/kualitas rendah tidak mencatat kehadiran |
| FR-07 | P0 | Tepat satu catatan per orang per sesi | Percobaan bersamaan menghasilkan satu rekaman, bukan duplikasi |
| FR-08 | P0 | Absensi manual & koreksi | Hanya `admin` — atau `dosen` pada kelas yang ditugaskan — wajib alasan dan riwayat audit |
| FR-09 | P0 | Dashboard daftar hadir | Menampilkan pembaruan sesi, status, dan waktu secara konsisten |
| FR-10 | P0 | Penghapusan data biometrik | Template terhapus dari penyimpanan dan indeks/cache pencocokan |
| FR-11 | P0 | Ekspor CSV | Tidak memuat foto, embedding, token, atau kata sandi |
| FR-12 | P0 | Sumber gambar webcam | Demo software lengkap sebelum hardware dibeli |
| FR-13 | P0 | APK Android | Terpasang dan dapat terhubung ke backend lokal pada HP uji |
| FR-14 | P1 | Indikator realtime | WebSocket atau polling dengan keadaan koneksi jelas |
| FR-15 | P1 | Tantangan liveness dasar | Diuji sebagai mitigasi terbatas, tidak diklaim anti-spoofing sempurna |
| FR-16 | P0 | Halaman peserta (mahasiswa) | Lihat kelas yang diikuti, jadwal sesi absensi kelas itu, status dan riwayat kehadiran sendiri, serta status persetujuan sendiri, tanpa akses ke template biometrik, embedding, atau data peserta lain |
| FR-17 | P2 | Kamera ESP32-S3 dan indikator | Input kamera fisik menggantikan webcam tanpa mengubah aturan absensi |
| FR-19 | P0 | Pengelolaan akun dan role oleh `admin` | `admin` mengundang akun `invited` (tanpa kata sandi), mengubah `role`/`status`/`display_name`, dan menautkan akun `mahasiswa` ke baris peserta; `409 LAST_ADMIN_PROTECTED` dan `409 SELF_ACCOUNT_PROTECTED` menahan operasi yang menghapus admin aktif terakhir atau akun sendiri; role tidak pernah diambil dari Google atau klien |
| FR-20 | P0 | `dosen` hanya mengakses kelas yang ditugaskan | Daftar kelas, sesi, dan koreksi manual `dosen` terbatas pada penugasan `class_lecturers`; objek kelas di luar penugasan dijawab `404 NOT_FOUND` (bukan `403`) |

**Catatan penomoran:** FR-16 dinaikkan dari P1 ke **P0** oleh keputusan FL-010U (16 Oktober 2026) seiring adanya akun `mahasiswa`; FR-18 tidak dipakai pada revisi ini.

## 8. Aturan bisnis dan keadaan data

- Orang terdaftar **belum berarti** boleh dikenali; persetujuan biometrik masih harus aktif.
- Sesi hanya dapat menerima absensi saat `open`. Sesudah `closed`, mesin kamera tidak dapat menambah kehadiran.
- Kehadiran `present` atau `late` ditentukan waktu check-in dan aturan sesi; `absent` adalah status turunan setelah sesi ditutup bagi anggota yang belum tercatat.
- Jika AI memberikan dua kandidat berdekatan atau kualitas di bawah ambang, keluaran harus **perlu pemeriksaan**, tanpa mengungkap tebakan identitas ke layar publik.
- Pelaku absensi ganda menerima `already_checked_in`; waktu kehadiran pertama tidak boleh tertimpa.
- Verifikasi manual tidak boleh dianggap lebih rendah atau menghukum orang yang menolak biometrik.
- Setiap koreksi memiliki petugas pelaku (`admin`/`dosen`), waktu, alasan, dan catatan perubahan.
- Akun manusia berstatus `invited`, `active`, atau `suspended`. Role dan status selalu dibaca ulang dari database pada setiap permintaan karena token aplikasi tidak memuat klaim peran; perubahan role dan penonaktifan akun karena itu berlaku pada permintaan berikutnya, bukan menunggu token kedaluwarsa.
- Penonaktifan akun mencabut seluruh sesi akun tersebut. Akun `mahasiswa` yang belum ditautkan ke baris peserta tidak dapat membuka data kehadiran atau persetujuannya sendiri (`409 ACCOUNT_NOT_LINKED`), dan penautan akun **tidak** memberi akses ke template biometrik.

## 9. Persyaratan nonfungsional

- **Biaya:** gratis tanpa pembayaran software/AI/cloud. Tidak mengandalkan trial sebagai komponen wajib.
- **Privasi:** data wajah diproses lokal, tidak dikirim ke AI coding agent, tidak dimasukkan Git, template dilindungi, izin dapat ditarik.
- **Keamanan:** autentikasi Google Sign-In (verifikasi ID token OpenID Connect) ditambah sesi aplikasi sendiri berupa token opak yang dapat dicabut dan hanya disimpan sebagai hash di server; otorisasi berbasis role dari database, validasi JPEG, pembatasan ukuran dan laju, kredensial khusus terminal, log yang disaring. ID token Google **bukan** token sesi aplikasi.
- **Ketahanan:** bila AI/backend/Wi-Fi mati, tidak menciptakan catatan kehadiran palsu; petugas (`admin`/`dosen`) diberi status gangguan. Google Sign-In **tidak** dijanjikan berjalan offline: selama sesi aplikasi masih sah, operasi lewat LAN tetap berjalan tanpa memanggil Google.
- **Performa:** ukur latensi kamera→hasil dan latensi UI melalui pengujian; **tidak menetapkan angka performa sebagai klaim sebelum pengukuran**.
- **Aksesibilitas:** status tidak hanya ditunjukkan warna; teks umpan balik, ukuran sentuh memadai, dan jalur manual tersedia.
- **Portabilitas:** Windows sebagai lingkungan awal; kontrak kamera tidak mengunci jenis perangkat.

## 10. Definisi keberhasilan demo

Prototipe dinilai siap demo bila `admin` dan `dosen` dapat menjalankan alur lengkap sesuai cakupannya: `admin` masuk dengan Google Sign-In → mengelola akun/role, kelas, dan penugasan dosen → mendapatkan persetujuan → enrollment beberapa sukarelawan → menugaskan `dosen` → `dosen` membuka sesi pada kelas yang ditugaskan → absensi wajah yang jelas → sistem menolak kasus salah/duplikat → absensi manual → menutup sesi → `admin` mengekspor laporan → menjalankan APK pada perangkat uji; `mahasiswa` dapat melihat kelas, jadwal sesi, status kehadiran, dan status persetujuannya sendiri. Semua hasil uji dan kelemahannya dicatat. Pengujian harus mencakup peserta berkacamata, tetapi keberhasilan tanpa kacamata **tidak cukup** untuk mengklaim berhasil pada seluruh jenis kacamata.

## 11. Risiko dan ketergantungan

- Salah kenal atau salah menolak dapat memengaruhi catatan kehadiran; sistem wajib gagal secara aman.
- Foto/video palsu dapat menipu kamera RGB; gunakan pengawasan petugas dan tidak melebih-lebihkan liveness.
- Kacamata dan pencahayaan menurunkan mutu gambar; perlu kalibrasi, diffuser, dan pengujian.
- Model AI dan bobotnya mungkin mempunyai lisensi berbeda dari kode pustakanya; periksa sebelum dipaketkan.
- Perangkat ESP32-S3 bervariasi pada pin, PSRAM, dan driver; jangan membeli tanpa memeriksa model persis.
- **Ketergantungan internet saat login (FL-010U).** Google Sign-In menuntut internet. Selama sesi aplikasi masih sah, operasi lokal tetap berjalan, tetapi login baru dan re-autentikasi tidak dapat dilakukan tanpa jaringan; masa berlaku token penyegar 30 hari membuat pengguna yang aktif berkala tidak perlu internet setiap hari. Risikonya dinyatakan apa adanya: tidak ada jalur absensi alternatif bila internet mati dan sesi aplikasi sudah tidak berlaku.
- **Jadwal akademik penuh belum ada (FL-010U).** MVP tidak memiliki tabel jadwal kuliah; yang ditampilkan ke `mahasiswa` hanya sesi absensi nyata (rencana jadwal akademik penuh dicatat sebagai pertanyaan terbuka).
- Keputusan backend, database, penautan akun `mahasiswa` ke baris peserta, dan retensi data belum semuanya final. Lihat `docs/OPEN_QUESTIONS.md`.

## 12. Batas persetujuan implementasi

PRD ini **masih draf**. Sebelum memulai kode untuk fitur tertentu, pemilik project menyetujui keputusan yang dibutuhkan fase tersebut. Jangan mengubah ruang lingkup atau mengaktifkan cloud atas inisiatif agent sendiri.

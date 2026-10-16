# Alur Pengguna dan UI/UX React Native

**Status:** Rancangan v0.3, belum berupa tampilan atau aplikasi jadi.
**Revisi FL-010U (16 Oktober 2026):** rancangan diselaraskan dengan tiga role manusia (`admin`, `dosen`, `mahasiswa`) yang masuk dengan **Google Sign-In**, dan `device` sebagai autentikasi mesin terpisah. Cabang `docs/fl-010u-google-three-roles` dari `origin/main` (`804c2f7`). Role `operator` lama digantikan `dosen`; akun `mahasiswa` kini ada. Matriks izin lengkap: `docs/AUTHORIZATION.md` §2 dan §4.
**Revisi FL-010V (16 Oktober 2026):** ditambahkan keadaan **sesi dicabut/pengguna dinonaktifkan** dan **kegagalan TLS/sertifikat tidak tepercaya** (§2.1 dan §3), catatan **satu role utama per akun sehingga navigasi tidak berpindah role** (§2), serta sub-bagian **§3a prosedur offline saat sesi tidak sah** — **tanpa bypass autentikasi**, rujuk Q-24. Seluruh keadaan baru berstatus **BELUM TERVERIFIKASI**. Rancangan biometrik dan aturan konsen **tidak diubah**.
**Platform utama:** Android melalui React Native + Expo + TypeScript. Dashboard berfokus pada tiga role manusia pada MVP.

## 1. Tujuan pengalaman pengguna

`admin` dapat menyiapkan demo satu kelas tanpa perlu memahami AI atau pengaturan perangkat yang rumit: mengelola akun dan role, mengelola kelas beserta penugasan dosen, peserta, persetujuan, dan enrollment. `dosen` dapat menjalankan sesi absensi pada kelas yang ditugaskan kepadanya dan memahami kapan hasil perlu diperiksa ulang. `mahasiswa` memahami kapan mereka boleh menghadap kamera, bagaimana status absensinya diketahui, apa yang harus dilakukan jika pengenalan gagal, dan dapat melihat status serta riwayat kehadirannya sendiri tanpa melihat data peserta lain. Pengguna juga memahami bahwa masuk memerlukan internet (Google Sign-In) sedangkan operasi sesi tetap berjalan lewat LAN selama sesi aplikasi masih sah.

## 2. Struktur navigasi per role dan alur masuk Google

Navigasi ditentukan oleh `role` yang dikembalikan backend. Menyembunyikan menu **bukan** pengendalian akses: setiap permintaan tetap diperiksa backend. Role manusia yang berlaku pada MVP adalah `admin`, `dosen`, dan `mahasiswa`, semuanya masuk dengan Google Sign-In; `device` adalah autentikasi mesin terpisah dan tidak memakai aplikasi ini. Role `operator` lama digantikan `dosen` (cakupannya terbagi antara `admin` dan `dosen`), dan akun `mahasiswa` kini ada; seluruh matriks izin ada di `docs/AUTHORIZATION.md` §2 dan §4.

**Satu role utama per akun (FL-010V, 16 Oktober 2026 — D-037).** Satu akun internal memiliki **tepat satu role utama aktif**, dan **satu identitas Google tidak boleh menghasilkan beberapa sesi role berbeda**. Karena itu **navigasi tidak pernah berpindah role** di dalam aplikasi: tidak ada tombol, menu, atau alur **multi-role switching** pada MVP, dan satu perangkat yang sudah masuk sebagai satu role tetap memakai navigasi role itu sampai keluar. Menjalankan demo sebagai `admin` sekaligus `dosen` menuntut **dua akun berbeda**, bukan satu akun dengan dua peran. Hak istimewa `admin` atas operasi kelas ditetapkan backend dan **tidak** menjadikan akun itu `dosen` kedua. Perubahan role hanya oleh `admin` berwenang, **wajib diaudit** (`user.role_changed`), dan berlaku pada permintaan berikutnya karena role dibaca ulang dari database — aplikasi tidak mengubah navigasi berdasarkan klaim token.

### 2.1 Alur masuk dengan Google Sign-In

1. Pengguna menekan **"Continue with Google"**; tidak ada form email/kata sandi.
2. Aplikasi membuat **nonce acak per percobaan login**, mengirimkannya ke Google dan ke backend, lalu mengirim Google ID token ke backend (`POST /auth/google`). Aplikasi **tidak pernah** menentukan role. Nonce berlaku sekali pakai (kedaluwarsa 5 menit); percobaan login yang gagal memerlukan nonce baru.
3. Backend memverifikasi ID token, mencari akun internal, memeriksa `status`, lalu menerbitkan sesi aplikasi; respons memuat `role` dan aplikasi mengarahkan ke beranda role tersebut.
4. Sesi disimpan pada **penyimpanan kredensial platform** (Android Keystore lewat penyimpanan aman Expo), **bukan** plaintext di `AsyncStorage`, dan tidak pernah ditulis ke log atau berkas konfigurasi. Google ID token tidak disimpan sebagai kredensial.

Keadaan gagal yang wajib dibedakan. Untuk keadaan yang menyangkut keberadaan atau status akun, pesan yang ditampilkan **satu pesan generik** (anti-enumeration) dan tidak membedakan sebabnya:

| Keadaan | Yang ditampilkan |
|---|---|
| Akun belum diberi akses | Pesan generik bahwa akun ini belum diberi akses ke FaceLink AI, disertai arahan menghubungi admin; **tidak** membedakan "email belum terdaftar" dari "identitas belum ditautkan" |
| Akun dinonaktifkan (`suspended`) | Pesan generik senada dengan di atas; mencoba masuk lagi tidak akan berhasil sampai `admin` mengaktifkan kembali |
| Google tidak terjangkau | Pesan gangguan jaringan bahwa layanan identitas tidak dapat dihubungi saat ini, dengan opsi mencoba lagi; **tidak ada** sesi yang diterbitkan |
| Sesi berakhir | Sesi aplikasi tidak lagi sah (kedaluwarsa atau dicabut); pengguna wajib masuk kembali dengan Google dan memerlukan internet saat itu |
| Akun belum tertaut | Akun `mahasiswa` aktif tetapi belum ditautkan ke baris peserta (`409 ACCOUNT_NOT_LINKED`); dijelaskan bahwa data pribadi belum tersedia dan `admin` perlu menautkan akun |
| **Sesi dicabut saat aplikasi berjalan (FL-010V)** | Permintaan berikutnya ditolak `401 SESSION_REVOKED`; aplikasi menghapus token yang tersimpan, menutup layar operasi, dan **kembali ke layar masuk Google** dengan penjelasan singkat bahwa sesi tidak lagi berlaku. Tidak ada mode kerja lanjutan, tidak ada data lama yang ditampilkan, dan tidak ada opsi "lanjut tanpa masuk" |
| **Pengguna dinonaktifkan saat aplikasi berjalan (FL-010V)** | Sama seperti di atas: sesi dicabut dan aplikasi **kembali ke layar masuk Google** dengan pesan generik §2.1 (arahan menghubungi `admin`); mencoba masuk lagi gagal sampai `admin` mengaktifkan kembali. Pesan **tidak** membedakan penonaktifan dari pencaburan lain |
| **Kegagalan TLS / sertifikat tidak tepercaya (FL-010V)** | Pesan jelas bahwa koneksi ke server tidak aman atau sertifikat server tidak tepercaya, dengan arahan menghubungi `admin` untuk memasang sertifikat yang benar. **Tidak ada** opsi "abaikan", "lanjut saja", atau "nonaktifkan validasi sertifikat", dan **tidak ada** pengiriman token atau ID token di atas koneksi yang gagal diverifikasi |

**Catatan FL-010V (16 Oktober 2026).** Kegagalan TLS **tidak boleh** diselesaikan dengan menonaktifkan validasi (`InsecureSkipVerify` atau ekuivalen) dan tidak boleh ditawarkan sebagai pilihan kepada pengguna. Bila sertifikat tidak tepercaya, aplikasi berhenti dan melaporkan; perbaikan dilakukan pada konfigurasi server/sertifikat (lihat `docs/SECURITY_PRIVACY.md` §6).

### 2.2 Navigasi `admin`

| Layar | Isi utama | Tindakan utama |
|---|---|---|
| Masuk | Tombol "Continue with Google" dan keadaan gagal §2.1 | Masuk |
| Beranda sistem | Status server/AI/kesiapan, ringkasan kelas dan sesi | Pilih area kerja |
| Akun & Role | Daftar akun (`invited`/`active`/`suspended`), role, status penautan ke peserta | Undang akun, ubah role/status, tautkan/lepas akun peserta |
| Kelas & Penugasan | Daftar kelas, dosen yang ditugaskan, anggota kelas | Buat kelas, tugaskan/lepas dosen, kelola anggota |
| Peserta & Persetujuan | Kode peserta demo, status konsen, status penautan akun | Tambah peserta, catat/cabut konsen |
| Enrollment | Penjelasan consent, kualitas kamera, progres sampel | Mulai/batalkan enrollment, hapus template |
| Perangkat | Daftar perangkat dan status token | Daftarkan perangkat, putar token, cabut |
| Sesi & Rekap | Sesi lintas kelas, daftar hadir, belum tercatat | Pantau, koreksi manual, ekspor CSV |
| Audit | Daftar `audit_events` tersaring | Telusuri insiden dan perubahan akun/role |

### 2.3 Navigasi `dosen`

| Layar | Isi utama | Tindakan utama |
|---|---|---|
| Masuk | Tombol "Continue with Google" dan keadaan gagal §2.1 | Masuk |
| Beranda kelas saya | Kelas yang **ditugaskan** kepadanya beserta sesi berjalan | Pilih kelas |
| Peserta kelas | Daftar peserta kelas itu dan status persetujuan (baca) | — |
| Sesi Absensi | Mulai/tutup sesi, waktu, daftar hadir berjalan, peserta belum tercatat | Jalankan sesi, lihat hasil |
| Pemeriksaan Manual | Peserta, alasan, sumber data, tindakan koreksi | Check-in manual atau koreksi dengan alasan |
| Rekap kelas saya | Jumlah hadir/terlambat/belum tercatat pada kelasnya | Lihat ringkasan; ekspor CSV dilakukan `admin` |

Kelas di luar penugasan tidak ditampilkan; bila tetap diminta, backend menjawab `404 NOT_FOUND`.

### 2.4 Navigasi `mahasiswa`

| Layar | Isi utama | Tindakan utama |
|---|---|---|
| Masuk | Tombol "Continue with Google" dan keadaan gagal §2.1 | Masuk |
| Beranda pribadi | Sesi absensi berikutnya dari kelas yang diikuti, ringkasan kehadiran sendiri | Lihat detail |
| Jadwal saya | Sesi absensi kelas yang diikuti pada rentang waktu yang dipilih | Lihat detail sesi |
| Kehadiran saya | Status, sumber, waktu, dan alasan bila ada — hanya data sendiri | — |
| Persetujuan saya | Status persetujuan biometrik sendiri (baca saja) | Diarahkan ke petugas untuk perubahan |
| Profil | Akun tertaut, role, tindakan keluar | Logout |

`mahasiswa` tidak melihat daftar peserta lain, tidak melihat data biometrik siapa pun, dan tidak dapat mengubah catatan kehadiran. Penarikan persetujuan oleh mahasiswa sendiri dari aplikasi belum dirancang (lihat `docs/OPEN_QUESTIONS.md`).

## 3. Keadaan UI yang wajib berbeda

- **Memuat:** tampilkan indikator tanpa membuat asumsi hasil.
- **Siap:** koneksi lokal dan sesi valid.
- **Sedang memproses:** kamera/AI berjalan; cegah pengiriman berulang tanpa kontrol.
- **Berhasil:** hasil sah dan diterima backend, bukan semata-mata kemunculan wajah di kamera.
- **Sudah absen:** informasi bahwa peserta telah tercatat, tanpa membuat row baru.
- **Perlu ulang:** contoh wajah blur, pantulan kacamata, posisi wajah tidak tepat.
- **Butuh petugas (`admin`/`dosen`):** hasil ambigu, peserta tidak mendaftar, persetujuan belum aktif.
- **Offline/gangguan:** koneksi server atau mesin AI bermasalah; tampilkan pesan yang dapat ditindaklanjuti.
- **Sesi/akun (FL-010U):** keadaan berikut harus punya tampilan terpisah dari keadaan kamera di atas — **sedang masuk** (Google Sign-In berjalan), **sesi berakhir** (sesi aplikasi kedaluwarsa atau dicabut → wajib masuk lagi), **akun belum diberi akses atau dinonaktifkan** (satu pesan generik, lihat §2.1), **akun belum tertaut** (`409 ACCOUNT_NOT_LINKED`), dan **perlu internet untuk masuk** (Google tidak terjangkau). Selama sesi masih sah, aplikasi tidak memanggil Google, jadi keadaan ini tidak muncul saat internet mati tetapi sesi hidup.
- **Sesi dicabut atau pengguna dinonaktifkan saat aplikasi berjalan (FL-010V, 16 Oktober 2026):** tampilan terpisah yang menutup layar operasi, menghapus token tersimpan, dan **mengembalikan pengguna ke layar masuk Google** dengan penjelasan singkat. Tidak ada mode kerja lanjutan, tidak ada tampilan data lama, dan tidak ada tombol "lanjut tanpa masuk". Pesan penonaktifan memakai pesan generik §2.1 agar tidak membedakan sebabnya.
- **Kegagalan TLS atau sertifikat tidak tepercaya (FL-010V, 16 Oktober 2026):** pesan jelas bahwa koneksi tidak dapat diverifikasi kepercayaannya, disertai arahan menghubungi `admin`. **Tidak ada** opsi menonaktifkan validasi sertifikat, mengabaikan peringatan, atau melanjutkan pengiriman data.
- **Offline dengan sesi tidak sah (FL-010V, 16 Oktober 2026):** internet mati **dan** sesi aplikasi tidak berlaku; tampilkan penjelasan bahwa masuk kembali memerlukan internet, dan **tidak ada** jalur operasi di aplikasi. Rincian prosedur mengikuti §3a.

Jangan memakai warna saja untuk menyampaikan status: sertakan ikon dan teks jelas.

### 3a. Prosedur offline saat sesi tidak sah (FL-010V, 16 Oktober 2026)

Internet tetap diperlukan untuk **login awal dan re-autentikasi** serta pengambilan kunci/metadata Google. Sesi aplikasi yang masih sah dipakai pada backend lokal tanpa menghubungi Google pada setiap permintaan, tetapi **login Google baru tidak boleh dijanjikan berjalan offline**.

Bila internet terputus **dan** sesi aplikasi tidak berlaku (kedaluwarsa, dicabut, atau batas absolut 30 hari tercapai):

1. **Tidak ada bypass autentikasi.** Aplikasi **tidak** menyediakan mode operasi, kunci darurat, akun lokal, atau jalur apa pun yang melewati pemeriksaan sesi, status akun, role, dan cakupan resource. Endpoint terproteksi tetap menolak permintaan tanpa sesi yang sah (`401`), dan penolakan itu **tidak** dilonggarkan hanya karena jaringan bermasalah.
2. **Aplikasi tidak menampilkan data operasi.** Layar sesi, daftar hadir, dan data peserta tidak dibuka kembali dari salinan lama; yang tampil hanya penjelasan keadaan dan langkah yang harus dilakukan.
3. **Absensi manual mengikuti prosedur operasional aman.** Kehadiran yang harus dicatat selama keadaan ini dicatat di luar aplikasi mengikuti prosedur operasional kelas yang disetujui (daftar hadir manual yang ditandatangani/diverifikasi petugas yang berwenang), **bukan** melalui klaim aplikasi.
4. **Operator memasukkannya setelah terautentikasi kembali.** Setelah jaringan dan sesi pulih, petugas yang berwenang (peran yang berhak menurut `docs/AUTHORIZATION.md` §4) memasukkan catatan itu melalui **absensi manual yang terautentikasi**, dengan `reason`, sehingga setiap baris kehadiran dapat dipertanggungjawabkan dan tercatat pada `audit_events`.
5. **Tidak ada kehadiran yang diisi tebakan.** Kehadiran yang tidak dapat dipastikan dibiarkan **tidak tercatat**, bukan diisi perkiraan (D-028).
6. **Rincian prosedur masih terbuka.** Rancangan rinci — termasuk bila keadaan ini tidak dapat diterima untuk demo — tetap **Q-24** pada `docs/OPEN_QUESTIONS.md` dan **belum diputuskan**; bagian ini hanya menetapkan larangan bypass autentikasi dan arah prosedur, **bukan** janji bahwa jalur offline sudah dirancang atau teruji.

**Status: BELUM TERVERIFIKASI.** Tidak ada bagian dari prosedur ini yang sudah dijalankan atau diuji.

## 4. Alur demo kelas pertama

1. `admin` masuk dengan Google Sign-In dan melihat kesehatan server lokal; bila akun pertama belum ada, akun `admin` disiapkan lebih dahulu lewat langkah bootstrap lokal oleh pemilik.
2. `admin` mengelola akun: mengundang akun `dosen` dan `mahasiswa`, lalu menautkan akun `mahasiswa` ke baris peserta. Satu akun Google = satu akun = satu role, sehingga menjalankan demo sebagai admin sekaligus dosen memerlukan dua akun berbeda.
3. `admin` membuat kelas, menugaskan `dosen`, meninjau 5–10 peserta demo yang telah memberi persetujuan.
4. Jika ada peserta belum enrollment, `admin` membuka layar enrollment berizin. Enrollment dan pencatatan persetujuan berada pada jalur `admin`, bukan `dosen`.
5. `dosen` masuk dan hanya melihat kelas yang ditugaskan; `dosen` membuka sesi dengan aturan waktu keterlambatan.
6. Peserta berdiri bergantian di depan webcam, status muncul pada terminal serta halaman sesi.
7. Peserta mencoba absen kedua kali; aplikasi menampilkan **sudah absen**.
8. `dosen` mencoba contoh kasus gagal dan menggunakan absensi manual dengan alasan bila perlu.
9. `dosen` menutup sesi; `admin` mengekspor CSV.
10. `mahasiswa` masuk, membuka beranda pribadi, dan melihat kelas yang diikuti, jadwal sesi, riwayat kehadirannya sendiri, serta status persetujuannya.

## 5. Skenario peserta berkacamata

- Sediakan petunjuk posisi wajah dan pencahayaan yang mengurangi pantulan.
- Beri pilihan mencoba ulang jika confidence tidak mencukupi, **tanpa menampilkan skor teknis atau tebakan identitas ke layar umum**.
- Rekam hasil pengujian berkacamata secara agregat; jangan menyembunyikan kasus gagal.
- Jika tetap gagal, petugas (`admin`/`dosen` pada kelas itu) dapat mencatat kehadiran secara manual setelah pemeriksaan yang sesuai.

## 6. Pencabutan persetujuan

`admin` membuka profil peserta → memeriksa permintaan pencabutan → sistem memperlihatkan konsekuensi (pengenalan wajah berhenti, manual tetap bisa) → memproses penghapusan template dan cache → memberi konfirmasi berdasarkan hasil server. Jangan menyatakan sudah terhapus sebelum server mengonfirmasi. `mahasiswa` hanya melihat status persetujuannya sendiri (baca saja); pencatatan dan pencabutan konsen tetap pada jalur `admin`.

## 7. Pedoman visual

- Tampilan bersih, mudah dibaca saat demo kelas, dan tidak membutuhkan animasi berat.
- Kontras memadai; tombol dengan label jelas; ukuran target sentuh ramah mobile.
- Tampilkan status koneksi/kamera dan waktu sesi yang konsisten dalam zona Indonesia.
- Hindari menampilkan seluruh nama peserta di layar kamera publik; informasi lengkap hanya pada dashboard petugas (`admin`/`dosen`) yang sudah masuk.
- Tidak perlu fitur ratusan kelas atau dashboard enterprise sebelum MVP stabil.

## 8. Kriteria penerimaan UX

`admin` dapat menyelesaikan alur login Google → kelola akun dan role → kelas dan penugasan dosen → enrollment berizin → ekspor; `dosen` dapat menyelesaikan alur login Google → kelas yang ditugaskan → mulai sesi → hasil → manual → tutup sesi; `mahasiswa` dapat menyelesaikan alur login Google → beranda pribadi → jadwal sesi → riwayat kehadiran sendiri → status persetujuan sendiri, dan akun yang belum tertaut mendapat penjelasan yang jelas. Semua role mengetahui gangguan tanpa tertipu hasil palsu, membedakan gagal pengenalan dari tidak hadir, dan tidak pernah melihat data di luar cakupannya. Navigasi dan tampilan diuji pada HP Android sungguhan sebelum presentasi.

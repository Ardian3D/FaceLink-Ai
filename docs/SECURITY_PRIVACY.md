# Keamanan, Privasi, dan Persetujuan Biometrik

**Status:** Persyaratan minimal untuk MVP akademik. Dokumen ini adalah panduan engineering, **bukan nasihat hukum**. Untuk penggunaan institusional, konsultasikan pengelolaan data dan kebijakan yang berlaku.

## 1. Mengapa penting

Data wajah dan template biometrik bersifat sangat sensitif. Di Indonesia, **UU Nomor 27 Tahun 2022 tentang Pelindungan Data Pribadi** menggolongkan data biometrik sebagai data pribadi bersifat spesifik. Walaupun project hanya tugas kampus, peserta tetap perlu mendapat penjelasan, pilihan yang adil, dan perlindungan data.

## 2. Klasifikasi data

| Data | Risiko | Aturan awal |
|---|---|---|
| Gambar wajah mentah | Tinggi | Diproses sementara; tidak disimpan otomatis |
| Embedding/template wajah | Tinggi | Lokal, dienkripsi, akses terbatas, bisa dihapus |
| Nama/kode peserta | Pribadi | Minimalkan; gunakan kode fiktif untuk konten publik |
| Kehadiran dan jadwal | Pribadi | Operator yang sah saja; retensi dibatasi |
| Login, token perangkat, kunci enkripsi | Rahasia | Jangan log/commit; simpan terproteksi |
| Log kesehatan perangkat | Operasional | Tanpa JPEG/embedding dan tanpa identitas yang tidak perlu |

## 3. Persetujuan dan alternatif

- Sebelum enrollment, jelaskan: tujuan absensi, data yang diambil, lokasi pemrosesan, siapa yang bisa mengakses, masa simpan, cara pencabutan izin, dan alternatif manual.
- Peroleh tindakan persetujuan yang **jelas, sukarela, dan terdokumentasi**. Jangan menganggap kehadiran di ruang kelas sebagai persetujuan biometrik.
- Peserta yang menolak harus tetap dapat hadir melalui absensi manual tanpa perlakuan merugikan.
- Setelah izin dicabut, hentikan proses biometrik orang tersebut dan hapus template dari disk, database, indeks, cache, serta backup bila kebijakan dan mekanismenya mengharuskan; catat penyelesaiannya.
- Pemrosesan dan penyimpanan data untuk jurnal atau publikasi akademik memerlukan kajian dan izin tersendiri; jangan mengambil otomatis dari sistem produk.

## 4. Ancaman utama dan mitigasi

| Ancaman | Mitigasi awal |
|---|---|
| Akses operator tidak sah | Login, hash kata sandi aman, otorisasi tiap endpoint, pembatasan percobaan |
| Perangkat palsu mengirim absensi | Kredensial khusus device, daftar perangkat diizinkan, rotasi/revokasi |
| Request JPEG berbahaya | Validasi MIME dan decode, batas ukuran/laju, timeout, perlindungan memori |
| Foto/video untuk titip absen | Deteksi presentasi dasar yang diuji, pengawasan operator, fallback manual; jangan klaim kebal |
| Bocor embedding/foto | Penyimpanan lokal terlindungi, enkripsi template AES-256-GCM per baris (D-022), tidak ada telemetri eksternal |
| Pengiriman melalui Wi-Fi bersama | Jaringan lab terisolasi; untuk pemakaian riil memerlukan transport terenkripsi |
| Data pribadi ter-commit | `.gitignore`, pemeriksaan sebelum commit, fixtures sintetis, pembatalan dan penanganan kebocoran |
| Duplikasi request atau perubahan sesi | Constraint database, transaksi, idempotensi, validasi ulang status |
| Pemrosesan setelah izin dicabut | Cabut akses template, bersihkan cache, uji penghapusan |

## 5. Penyimpanan, enkripsi, dan retensi

- Tentukan umur simpan **sebelum** pengambilan data asli. Prinsip awal: simpan seminimal mungkin, hapus sesegera mungkin setelah presentasi dan evaluasi selesai.
- Template biometrik disimpan **terenkripsi AES-256-GCM per baris** (D-022): nonce acak kriptografis per baris, tag autentikasi diverifikasi sebelum dekripsi, dan dekripsi yang gagal **tidak** diganti dengan tebakan. Satu-satunya lokasi penyimpanan biometrik adalah tabel `face_templates` (`docs/DATA_MODEL.md` §4.12).
- Kunci enkripsi **tidak** boleh berada di dalam database, di dalam commit, atau di berkas konfigurasi yang ikut ter-commit. Bentuk penyimpanan final (AES-256-GCM per baris, D-022); kebijakan pengelolaannya ditetapkan pada **D-030 (DISETUJUI pada FL-004S dengan dua klarifikasi wajib)** dan dirinci pada §5a–§5f di bawah. Q-16 **DIPUTUSKAN** untuk rancangan MVP, tetapi **implementasi penyimpanan biometrik dan seluruh pengujiannya masih tertunda** dan menunggu tugas implementasi tersendiri.
- Enkripsi template bukan enkripsi seluruh berkas database: rekap kehadiran dan nama peserta tetap terbaca bagi siapa pun yang memegang berkas SQLite. Jangan mengklaim sebaliknya.
- Cadangan data, jika benar-benar dibutuhkan, harus tunduk pada retensi/penghapusan yang sama; jangan menyisakan template yang sudah dicabut di backup tanpa kebijakan.
- **Pencabutan persetujuan tetap merupakan penghapusan, bukan penonaktifan.** Kunci tidak menyimpan salinan template, sehingga penghapusan pada `face_templates` (`docs/DATA_MODEL.md` §5.3) bersifat final dan tidak dapat dibatalkan dengan memulihkan kunci (D-030 butir 7).
- **Cadangan database bukan cadangan biometrik.** Berkas `facelink.db`, `-wal`, dan `-shm` hanya berguna bersama berkas kunci terlindungi. Cadangan yang hanya berisi database harus dijelaskan apa adanya kepada pemilik project, bukan disajikan sebagai pemulihan penuh.

### 5a. Kebijakan kunci yang berlaku (D-030)

Bagian ini memuat **kebijakan yang menjawab Q-16**. D-030 **DISETUJUI pemilik project pada 15 Oktober 2026 (FL-004S)** dengan dua klarifikasi wajib, yang ketentuannya ada pada §5e dan §5f. **Disetujui bukan berarti diimplementasikan:** belum ada kode yang ditulis dan belum ada pengujian yang dijalankan.

1. **Pemilik kunci adalah backend Go.** Python tidak memegang kunci, tidak menerima kunci, dan tidak mendekripsi template. Ini memperluas D-027 dan `docs/API_CONTRACT.md` §6.8.
2. **Lokasi penyimpanan:** berkas kunci terlindungi di `%LOCALAPPDATA%\FaceLink\keys\` pada akun Windows yang menjalankan backend, dengan ACL yang hanya mengizinkan akun itu. Jangan memakai folder yang disinkronkan ke penyimpanan awan.
3. **Bentuk perlindungan:** materi kunci dibungkus **Windows DPAPI mode pengguna** (`CryptProtectData`, tanpa `CRYPTPROTECT_LOCAL_MACHINE`). Mode mesin **ditolak** karena akan membuka kunci bagi setiap pengguna mesin tersebut.
4. **Satu pintu akses:** seluruh kode memakai antarmuka `KeyProvider` di Go untuk mengambil kunci aktif dan kunci berdasarkan versi. Tidak ada modul lain yang membaca berkas kunci langsung, dan tidak ada operasi yang mengembalikan kunci dalam bentuk yang dapat dicetak ke log.
5. **Penamaan versi:** `key_version` adalah **teks**, dimulai dari `'1'`, naik bilangan bulat. Nilai ini menjadi bagian AAD (D-022), sehingga baris dengan versi `'1'` tidak dapat dibaca memakai kunci berlabel `'2'`.
6. **Tidak ada kunci default, tidak ada kunci contoh, dan tidak ada kunci yang dibangkitkan diam-diam.** Bila kunci aktif tidak ada saat pertama kali dibutuhkan, penyalaan backend **gagal terang-terangan** dengan instruksi menyiapkan kunci, bukan dengan membuat kunci baru. Kunci yang dibuat otomatis pada mesin yang salah adalah cara kehilangan template secara diam-diam.
7. **Tidak ada kunci pemulihan otomatis.** Yang dapat dipulihkan adalah keberadaan sistem, bukan data biometrik. Ketentuan **periode pemulihan 30 hari** dijelaskan pada §5d butir 1 dan **bukan** jaminan pemulihan kunci.

### 5b. Batas DPAPI yang harus diakui

**Jangan menganggap DPAPI menjamin keamanan terhadap semua jenis serangan.**

| Batas | Arti bagi project ini |
|---|---|
| Aplikasi apa pun yang berjalan dengan kredensial pengguna yang sama dapat membuka blob | DPAPI melindungi dari **pengguna lain di mesin yang sama** dan dari **berkas yang dicuri lalu dibuka di mesin lain**. Ia **tidak** melindungi dari malware di sesi pengguna yang sama, dan tidak melindungi dari operator yang menyalahgunakan mesinnya |
| Profil **roaming** dapat didekripsi dari komputer lain | Bila akun Windows memakai profil roaming di domain, kunci ikut berpindah. Asumsi "tidak ada profil roaming" harus **diverifikasi** pada mesin demo, bukan diandaikan |
| Materi kunci berada di dalam profil pengguna | Bila profil tidak termuat atau sudah direset, dekripsi **gagal**; template lama tidak dapat dibaca |
| Cakupan pengguna, bukan cakupan mesin | Disengaja. Cakupan mesin akan membuka kunci bagi semua pengguna mesin dan karena itu **dilarang** |

Kesimpulan yang harus disampaikan tanpa diperhalus: **AES-256-GCM melindungi isi database bila berkasnya bocor; DPAPI melindungi kunci dari pengguna lain dan dari pencurian berkas. Tidak ada lapisan di rancangan ini yang melindungi template biometrik dari akun Windows yang sudah dikuasai penyerang.** MVP tidak memiliki jawaban untuk kondisi itu.

### 5c. Dampak kunci terhadap SQLite WAL dan cadangan

1. Berkas `facelink.db`, `facelink.db-wal`, dan `facelink.db-shm` tetap diperlakukan sebagai satu kesatuan (`docs/DATA_MODEL.md` §7.1).
2. Berkas kunci adalah **berkas keempat** yang harus ikut dipindahkan. Menyalin tiga berkas database tanpa kunci menghasilkan database yang berisi kolom `ciphertext` yang tidak dapat dibaca.
3. Titik periksa WAL (`PRAGMA wal_checkpoint(TRUNCATE)`) **tidak** bergantung pada kunci dan tidak mengubah isi `ciphertext`. Menjalankannya tidak dapat "membuka" template dan tidak dapat menutup kunci yang bocor.
4. Karena `synchronous=NORMAL` (`docs/DATA_MODEL.md` §13 butir 2), kegagalan daya dapat menghilangkan transaksi terakhir. Bila kunci juga hilang pada kejadian yang sama, tidak ada cara memisahkan keduanya setelah fakta.
5. Cadangan **tidak boleh** memuat template dari peserta yang persetujuannya sudah dicabut. Cadangan lama harus mengikuti retensi yang sama (`docs/SECURITY_PRIVACY.md` §5 butir 5), bukan menyimpannya sebagai arsip.

### 5d. Kondisi kunci, kegagalan dekripsi, dan enrollment ulang

**Periode pemulihan 30 hari bukan jaminan pemulihan.** Angka itu adalah batas waktu **upaya** operator sebelum template dinyatakan hilang, bukan masa berlaku yang menjamin template bisa dibaca kembali pada hari ke-29. Selama 30 hari itu tidak ada jaminan teknis apa pun: bila kunci hilang atau rusak secara permanen, menunggu sampai hari ke-29 tidak mengembalikan apa pun. Yang tersedia hanyalah kesempatan operator memeriksa apakah salinan kunci masih ada.

#### 5d.1 Tiga kondisi yang harus dibedakan

Kegagalan membaca template **tidak** boleh diperlakukan sebagai satu peristiwa tunggal. Backend wajib memisahkan tiga kondisi berikut sebelum mengambil tindakan apa pun.

| Kondisi | Ciri yang tampak | Tindakan yang diizinkan |
|---|---|---|
| **A. Kunci sementara tidak tersedia** | Berkas kunci tidak ditemukan **atau** tidak dapat dibuka **saat ini** — profil pengguna belum termuat, berkas sedang terkunci proses lain, media sedang tidak terpasang, `KeyProvider` gagal karena I/O | **Jangan** menghapus apa pun. Tolak operasi yang membutuhkan template, catat `audit_events`, dan coba lagi setelah penyebabnya hilang. Template tetap utuh |
| **B. Kunci ada tetapi tidak dapat dipakai** | Berkas kunci ada dan terbaca, tetapi hasil dekripsi gagal — `auth_tag` tidak cocok, AAD tidak cocok, `key_version` tidak memiliki kunci padanannya, atau berkas kunci rusak/terpotong | Karantina: tandai baris sebagai **tidak dapat didekripsi**, jangan tampilkan, jangan hitung sebagai cocok. Periksa salinan kunci lain **sebelum** menyimpulkan apa pun |
| **C. Kunci dipastikan tidak dapat dipulihkan** | Operator telah memeriksa seluruh salinan kunci yang sah, periode 30 hari terlampaui, dan tidak ada cara memulihkan | Baru pada tahap ini penghapusan diizinkan (§5d.2) |

Kesalahan yang paling merugikan adalah memperlakukan kondisi A atau B sebagai kondisi C. Menghapus template karena berkas kunci sedang terkunci — atau karena satu berkas kunci rusak padahal salinan lain masih sah — **tidak dapat dibatalkan** dan menghancurkan data biometrik yang masih dapat dipulihkan. Bila backend tidak sanggup memastikan perbedaannya, tindakan yang benar adalah **berhenti dan melaporkan**, bukan menghapus.

#### 5d.2 Template tidak boleh langsung dihapus karena kegagalan sementara

1. **Kegagalan dekripsi satu kali bukan bukti kehilangan permanen.** Penghapusan **dilarang** selama kondisi masih A atau B.
2. **Ada masa tunggu wajib** antara kegagalan pertama dan penghapusan: minimal **30 hari** sejak kegagalan pertama tercatat, dan selama masa itu penyebabnya harus diperiksa berulang dengan hasil tercatat.
3. **Pemeriksaan salinan kunci adalah syarat wajib** sebelum penghapusan. Selama masih ada salinan kunci sah yang belum diuji, penghapusan **dilarang**.
4. **Penghapusan tidak pernah otomatis.** Ia dijalankan operator setelah kondisi C ditetapkan, tercatat pada `audit_events`, dan tidak boleh dipicu oleh tugas terjadwal tanpa campur tangan manusia.
5. **Penghapusan hanya menyentuh baris `face_templates`.** Nama peserta, rekap kehadiran, dan catatan audit tidak ikut dihapus; kehadiran yang sudah tercatat tetap sah dan tidak boleh ditarik kembali hanya karena template hilang (D-028).
6. **Karantina bersifat sementara dan tidak permanen.** Baris pada kondisi B tetap boleh ada. Baris tersebut tidak boleh dihitung sebagai template aktif pada pengecekan kehadiran, sehingga kehadirannya otomatis melalui jalur manual.

#### 5d.3 Prosedur penanganan insiden kunci

Diurutkan, dan setiap langkah dicatat pada `audit_events` **tanpa** menyimpan materi kunci atau nilai embedding.

1. **Kenali dan hentikan.** Hentikan sementara jalur otomatis yang memakai template; hentikan backend bila ada kemungkinan kunci terpapar (misalnya berkas kunci keluar dari `%LOCALAPPDATA%`). Jangan menyembunyikan masalahnya.
2. **Tentukan tingkatnya.** Bedakan: kunci tidak ditemukan (A), kunci ada tetapi tidak dapat dipakai (B), kunci diduga bocor, atau perangkat/mesin hilang. Klasifikasi ini menentukan langkah berikutnya; tidak semua insiden menuntut tindakan yang sama.
3. **Amankan sebelum menyelidiki.** Bila bocor diduga: hentikan pemrosesan, amankan kredensial, dan periksa jangkauan; jangan terlebih dahulu membuat kunci baru (menghapus bukti, tidak menyelesaikan apa pun).
4. **Periksa salinan kunci yang sah.** Bandingkan `key_version` yang dibutuhkan baris dengan versi yang tersedia; uji tiap salinan secara terpisah.
5. **Nyatakan hasilnya apa adanya.** Pesan ke operator memakai bahasa yang benar: "template sementara tidak tersedia", "template tidak dapat didekripsi", atau "template dinyatakan tidak dapat dipulihkan". Jangan memakai kata "hilang permanen" untuk kondisi A atau B.
6. **Putuskan pemulihan, rotasi, atau penghapusan** sesuai kondisi, dengan ketentuan §5d.2 untuk penghapusan.
7. **Tetapkan dan catat jangkauannya.** Sebutkan berapa template terdampak, sejak kapan, dan apakah data non-biometrik ikut terpengaruh.
8. **Pulihkan layanan tanpa biometrik lebih dulu.** Absensi manual dipakai selama template belum tersedia (§5d.4), agar kegiatan kelas tidak terhenti menunggu penyelesaian insiden.
9. **Tinjau setelah selesai.** Perbarui kebijakan bila ada asumsi yang ternyata salah. Bila insiden menunjukkan batas DPAPI yang belum tercatat pada §5b, tambahkan ke tabel itu.

Yang **dilarang** selama insiden: membangkitkan kunci baru diam-diam untuk "memperbaiki" keadaan (§5a butir 6), memindahkan kunci lewat Git, surel, penyimpanan awan, atau lampiran pesan, dan menyebarkan materi kunci ke catatan insiden.

#### 5d.4 Enrollment ulang dan pencatatan persetujuan

1. **Persetujuan baru, bukan persetujuan lama yang diwariskan.** Peserta **diminta ulang** persetujuannya (`docs/PRD.md`, §3) sebelum enrollment baru. Keikutsertaan sebelumnya tidak dianggap sebagai izin untuk pendaftaran berikutnya.
2. **Penjelasan sebelum meminta.** Peserta diberi tahu lebih dulu: apa yang hilang, mengapa, apakah data lamanya masih ada, dan apa yang akan disimpan bila ia setuju. Tanpa penjelasan itu, persetujuan tidak sah.
3. **Penolakan tidak boleh merugikan.** Peserta yang menolak mendaftar ulang tetap dapat hadir melalui absensi manual. Penolakan tidak boleh memengaruhi penilaian, keanggotaan, atau aksesnya.
4. **Yang tercatat adalah faktanya, bukan wajahnya.** `audit_events` mencatat waktu, operator, dan **status** persetujuan (diberikan/ditolak/dicabut). Dilarang mencatat potret, embedding, atau materi kunci sebagai bukti persetujuan.
5. **Pencabutan tetap berlaku seketika.** Bila peserta mencabut persetujuannya di tengah proses pemulihan, penghapusan template berjalan lebih dulu dan **tidak** menunggu masa tunggu §5d.2 — pencabutan adalah permintaan peserta, bukan diagnosis kegagalan.
6. **Template lama tidak dihidupkan kembali.** Setelah kondisi C, tidak ada upaya mencocokkan wajah peserta dengan potret lama atau data lain untuk "mengembalikan" template; enrollment baru selalu diambil ulang.

#### 5d.5 Absensi manual sebagai jalur tetap

1. **Absensi manual bukan keadaan darurat sementara, melainkan jalur yang selalu tersedia** (`docs/API_CONTRACT.md` §5.8, `docs/PRD.md`). Ia harus sudah berfungsi sebelum template apa pun ditulis.
2. Selama template tidak tersedia untuk seorang peserta — kondisi A, B, maupun C — kehadirannya dicatat melalui jalur manual dan **berlaku sama sahnya** seperti kehadiran otomatis.
3. **Ketiadaan template tidak boleh menjadi alasan menolak kehadiran**, menandai peserta bermasalah, atau menahan hak apa pun.
4. Bila jalur manual tidak dapat dipakai pada suatu sesi, sesi itu **tidak boleh** dilanjutkan dengan klaim otomatis yang tidak berdasar (D-028); kehadiran yang tidak dapat dipastikan dibiarkan tidak tercatat, bukan diisi tebakan.

#### 5d.6 Cadangan, pemindahan, dan rotasi

1. Perpindahan mesin hanya lewat penyalinan berkas kunci terlindungi oleh operator dan harus dicatat pada `audit_events`. **Dilarang** memindahkan kunci lewat Git, surel, penyimpanan awan, atau lampiran pesan.
2. Rotasi kunci **disiapkan tetapi belum diaktifkan** (D-030 butir 6). Bila nanti dijalankan, kunci lama dihapus hanya setelah tidak ada baris yang menunjuk versinya — dan hanya setelah penghapusan itu sendiri ditinjau, karena baris yang masih menunjuk versi lama tidak dapat dibaca setelah kuncinya hilang.

Satu hal yang harus dinyatakan jujur kepada peserta dan pemilik project: **kehilangan kunci berarti kehilangan data biometrik, dan satu-satunya jalan kembali adalah mendaftar ulang dengan persetujuan baru.** Namun kehilangan kunci **bukan** alasan menghapus template lebih cepat daripada kepastian; selama kondisi masih A atau B, yang benar adalah menunggu, memeriksa, dan melaporkan.
- Foto JPEG pemindaian rutin tidak disimpan secara default. Cukup simpan hasil status dan metadata seperlunya.
- Catatan audit menunjukkan tindakan tanpa merekam informasi wajah atau nilai embedding.

### 5e. Kesiapan layanan dipisahkan (klarifikasi wajib pemilik, FL-004S)

Klarifikasi ini menetapkan bahwa **kegagalan kunci atau `KeyProvider` tidak otomatis mematikan seluruh backend**. Yang dipisahkan adalah **kesiapan layanan**, bukan kepemilikan keputusan domain: absensi manual tetap ditulis oleh Go sesuai D-027 dan `docs/API_CONTRACT.md` §5.8. Rujukan keputusan: `docs/DECISIONS.md` D-030 butir 10.

**1. Dua status kesiapan yang terpisah.** Backend melaporkan kesiapan **layanan absensi manual** dan kesiapan **layanan biometrik** secara terpisah, masing-masing dengan alasannya sendiri. Kegagalan pada layanan biometrik **tidak boleh** menurunkan status layanan absensi manual, dan tidak boleh membuat seluruh backend berhenti melayani.

**2. Yang tetap berjalan ketika kunci hilang atau `KeyProvider` gagal:**

| Tetap berjalan | Keterangan |
|---|---|
| Absensi manual dan koreksi kehadiran manual | Jalur tetap, bukan darurat (§5d.5) |
| Rekap dan ekspor kehadiran | Membaca data yang sudah tersimpan |
| Kelola peserta, kelas, dan sesi | Tidak menyentuh template |
| Autentikasi dan otorisasi operator | Dasar keamanan; harus hidup agar jalur manual dapat dipertanggungjawabkan |
| Pencatatan `audit_events` | Justru wajib hidup saat insiden berlangsung |

Perubahan izin administratif yang tidak menyentuh biometrik **tidak** ikut berhenti.

**3. Yang berhenti dan menolak bekerja dengan pesan jelas:** enrollment biometrik, pembuatan template baru, pencocokan otomatis, dan **setiap penulisan** ke `face_templates`. Endpoint biometrik mengembalikan hasil "tidak dapat diverifikasi" (`docs/API_CONTRACT.md` §7), **bukan** kesalahan server umum yang menyiratkan backend rusak.

**4. Tetap gagal secara aman.** Pemisahan kesiapan **bukan** pembenaran untuk melanjutkan operasi yang keamanan atau integritasnya tidak dapat dijamin. Backend **wajib** berhenti keras bila salah satu hal berikut tidak dapat dijamin:

1. Integritas, keaslian, dan keterpulihan database serta migrasinya.
2. Autentikasi dan otorisasi operator.
3. Konsistensi kunci dengan AAD dan `key_version`, **termasuk** ketika tidak dapat dipastikan apakah kunci yang tersedia benar-benar kunci yang sesuai — keadaan itu adalah kondisi C yang belum pasti, dan perilakunya **berhenti dan melaporkan**, bukan menghapus.
4. Keutuhan riwayat audit dan catatan kehadiran.
5. Batas ukuran dan jenis input serta otorisasi pada endpoint absensi manual.

Menonaktifkan komponen keamanan agar layanan "tetap hidup" adalah pelanggaran, bukan pemulihan.

**5. Urutan pemulihan layanan.** Setelah gangguan, **absensi manual dipulihkan lebih dahulu**, baru layanan biometrik (§5d.3 langkah 8). Kegiatan kelas tidak boleh menunggu penyelesaian insiden kunci.

### 5f. Retensi, pembersihan, dan keterbatasan jaminan penghapusan (klarifikasi wajib pemilik, FL-004S)

Klarifikasi ini menetapkan bahwa kebijakan penghapusan **mengakui keterbatasan jaminan** pada SQLite WAL, salinan cadangan, dan media penyimpanan. Dokumen ini **tidak** boleh menyatakan data biometrik pasti lenyap dari disk. Rujukan keputusan: `docs/DECISIONS.md` D-030 butir 11.

**1. Urutan pembersihan yang ditetapkan.** Penghapusan biometrik dijalankan berurutan, dan setiap langkah dicatat pada `audit_events`:

1. Hapus baris pada `face_templates` (dan baris terkait yang menunjuk padanya).
2. Jalankan titik periksa WAL dengan pemotongan (`PRAGMA wal_checkpoint(TRUNCATE)`) agar isi WAL dipindahkan ke berkas utama dan berkas `-wal` dikosongkan.
3. Jalankan `VACUUM` agar halaman bebas dipadatkan dan ditimpa.
4. Hapus seluruh salinan cadangan yang dikenal, termasuk berkas `-wal` dan `-shm` pada salinan itu.
5. Bersihkan cache lapisan kamera dan antarmuka yang menyimpan gambar atau data turunannya.
6. Catat berkas yang disentuh, waktu, operator, dan **lokasi yang belum terjangkau**.

**2. Yang dapat diklaim setelah prosedur itu:** baris pada tabel sudah tidak ada; berkas database aktif tidak lagi memuat baris tersebut; WAL dan SHM sudah dipadatkan lalu ditimpa; salinan cadangan yang dikenal sudah dihapus.

**3. Yang tidak dapat diklaim, dan tidak boleh ditulis:**

| Klaim | Alasan tidak boleh |
|---|---|
| "Terhapus permanen dari disk" | SSD dan media flash memakai pemetaan ulang blok, TRIM, dan daur ulang blok di luar kendali aplikasi. Salinan lama dapat tetap berada pada blok yang tidak terjangkau berkas |
| "Tidak dapat dipulihkan secara forensik" | Berbeda dari "tidak ada lagi di berkas yang kita kendalikan"; pernyataan forensik menuntut pengukuran, bukan asumsi |
| "Cadangan sudah bersih" | Hanya sah bila setiap cadangan dan setiap platform sinkronisasi benar-benar sudah diperiksa dan dihapus |
| "Penghapusan terverifikasi" | Hanya sah bila verifikasi benar-benar dijalankan dan hasilnya dicatat |

**4. Jawaban MVP bersifat prosedural, bukan kriptografis.** Karena jaminan fisik tidak tersedia tanpa enkripsi khusus media, yang dikerjakan MVP adalah: **kenali** setiap lokasi salinan, **batasi** lokasi penyimpanan, **hapus** seluruh salinan yang dikenal pada setiap tahap, dan **catat** lokasi yang belum terjangkau sebagai keterbatasan yang belum diselesaikan. Cara ini mengurangi sisa data; ia **tidak** menjamin penghapusan fisik, dan itu harus diucapkan apa adanya.

**5. Lokasi yang harus diperiksa pada setiap pencabutan persetujuan:**

1. Berkas `facelink.db`, `facelink.db-wal`, dan `facelink.db-shm`.
2. Setiap salinan cadangan database beserta berkas `-wal` dan `-shm` pada salinan itu.
3. Cache lapisan kamera dan antarmuka, termasuk berkas sementara hasil dekode gambar.
4. Berkas log dan berkas kesalahan — yang **tidak** boleh pernah memuat isi JPEG, embedding, atau nilai biometrik.
5. Media penyimpanan itu sendiri, yang **tidak** dapat dibersihkan aplikasi.

**6. Keterbatasan ini bukan alasan menolak pencabutan persetujuan.** Peserta yang mencabut persetujuannya tetap dilayani seketika (§5d.4 butir 5); keterbatasan jaminan fisik dicatat sebagai keterbatasan sistem, bukan dibebankan kepada peserta. **Absensi manual tetap sah dan operasional** selama proses ini berjalan.

## 6. Pengendalian akses dan transportasi

- Operator hanya mendapat hak sesuai kebutuhannya; token perangkat tidak boleh digunakan untuk endpoint admin.
- Endpoint AI Engine hanya untuk backend lokal; batasi ke loopback `127.0.0.1`/IPC dengan kontrak internal `docs/API_CONTRACT.md` §6, dan jangan pernah membukanya pada jaringan kelas.
- Hanya backend Go yang boleh menulis database dan menetapkan kehadiran (D-027); AI Engine tidak memegang kredensial database, tidak membaca tabel biometrik, dan tidak boleh menyimpan embedding di disk.
- Hindari hardcode IP, password, kunci, atau token dalam firmware dan aplikasi. Rahasia perangkat bisa diprovisikan saat demo dan diganti bila bocor.
- Semua penyimpanan, koneksi, dan alur rilis yang terkait biometrik harus diuji sebelum penggunaan orang sungguhan.

## 7. Tindakan ketika gagal atau terjadi insiden

- Jika AI ragu, hentikan check-in otomatis; operator membantu verifikasi manual.
- Jika server atau jaringan mati, jangan membuat catatan kehadiran sementara yang seolah-olah sudah sah.
- Jika data berpotensi bocor, hentikan pemrosesan, amankan kredensial, cari jangkauan insiden dan lakukan penanganan sesuai ketentuan yang berlaku.
- **Jika template tidak dapat dibaca, bedakan kondisinya lebih dahulu (§5d.1) dan jangan menghapus apa pun selama kunci hanya sementara tidak tersedia atau masih ada salinan yang belum diuji (§5d.2).** Kegagalan dekripsi bukan bukti kehilangan permanen.
- **Insiden kunci memiliki prosedur tersendiri pada §5d.3**, termasuk larangan membangkitkan kunci baru untuk "memperbaiki" keadaan dan larangan memindahkan kunci lewat Git, surel, penyimpanan awan, atau lampiran pesan.
- **Selama template tidak tersedia, absensi manual tetap berjalan (§5d.5)** dan kehadiran tidak boleh dihambat karena ketiadaan template.
- **Kegagalan layanan biometrik bukan kegagalan seluruh backend (§5e).** Laporkan kesiapan kedua layanan secara terpisah; jangan menghentikan jalur absensi manual karena kunci bermasalah, dan jangan melumpuhkan pemeriksaan keamanan agar layanan terlihat hidup.
- **Jangan menyatakan penghapusan selesai melebihi bukti yang ada (§5f).** Sebut berkas yang benar-benar dihapus dan sebut lokasi yang belum terjangkau; jangan memakai kata "permanen" atau "forensik" tanpa pengukuran.
- Jangan menyembunyikan keterbatasan akurasi atau pengujian dengan kacamata dalam presentasi.

## 8. Syarat keamanan sebelum demo

- [ ] Dokumen persetujuan tersedia dan dipahami peserta.
- [ ] Jalur absensi manual berfungsi, termasuk ketika tidak ada satu pun template yang tersedia (§5d.5).
- [ ] Data pribadi/bio tidak masuk Git, log, atau LLM API.
- [ ] Token operator dan perangkat memiliki izin berbeda.
- [ ] Foto mentah tidak ditahan tanpa alasan/izin.
- [ ] Penghapusan template teruji pada disk, **cache**, **berkas `-wal`/`-shm`, dan salinan cadangan** (§5c, `docs/DATA_MODEL.md` §5.3).
- [ ] Berkas kunci tersedia, terlindungi DPAPI, dan **tidak** berada di dalam folder yang disinkronkan ke awan.
- [ ] Backend menolak menyala dengan pesan jelas bila kunci tidak ada, dan **tidak** membuat kunci baru sendiri (D-030 butir 6).
- [ ] Penyalinan berkas kunci ke mesin lain (simulasi penggantian perangkat) diuji dan hasilnya dicatat apa adanya, termasuk bila gagal.
- [ ] **Ketiga kondisi §5d.1 dapat dibedakan pada mesin nyata:** kunci sementara tidak tersedia, kunci ada tetapi tidak dapat dipakai, dan kunci dipastikan tidak dapat dipulihkan.
- [ ] **Kesiapan kedua layanan terbukti terpisah (§5e):** dengan kunci sengaja dirusak, absensi manual, kelola peserta, autentikasi, dan pencatatan audit tetap berjalan, sementara endpoint biometrik menolak dengan alasan yang jelas.
- [ ] **Gagal-aman tetap bekerja (§5e butir 4):** pemeriksaan integritas database, autentikasi operator, dan otorisasi endpoint manual masih menolak ketika syaratnya tidak terpenuhi.
- [ ] **Keterbatasan penghapusan diuji dan dicatat (§5f):** setelah urutan pembersihan, periksa isi berkas database, `-wal`, `-shm`, dan salinan cadangan, lalu catat apa yang masih dapat ditemukan **tanpa** mengklaim penghapusan fisik. Hasilnya dicatat apa adanya.
- [ ] **Kegagalan dekripsi sementara tidak menghapus template** — diuji dengan mengunci/menyembunyikan berkas kunci lalu memastikan jumlah baris `face_templates` tidak berubah.
- [ ] Batas DPAPI dijelaskan kepada pemilik project dengan kalimat §5b, bukan diringkas menjadi "kunci sudah terenkripsi".
- [ ] Gagal koneksi, wajah asing, dan hasil ambigu tidak memproduksi check-in.
- [ ] Pembatasan keamanan kamera/jaringan lokal dan kekurangan anti-spoofing dijelaskan.
- [ ] Lisensi model pihak ketiga ditinjau.

## 9. Keselarasan dengan keputusan dan kontrak lain

| Dokumen | Bagian yang mengikat |
|---|---|
| `docs/DECISIONS.md` | D-015 (template terenkripsi, tanpa foto mentah), D-022 (AES-256-GCM per baris), **D-030 (kebijakan kunci — DISETUJUI pada FL-004S dengan dua klarifikasi wajib)**, D-023 (kontrak kamera JPEG), D-025 (lisensi bobot model), D-027 (Go pemilik keputusan dan database), D-028 (Go sumber kebenaran, UI bukan penentu), D-029 (liveness terpisah) |
| `docs/DATA_MODEL.md` | §4.12 `face_templates` (kolom, AAD, perilaku dekripsi gagal, penghapusan keras), §5.3–§5.4 (penghapusan dan pencabutan), §6, §7.1–§7.3 (WAL dan pencadangan), §11 butir 4–7 (larangan menyimpan kunci, cadangan, dan larangan menulis `face_templates` sebelum D-030) |
| `docs/API_CONTRACT.md` | §6.7 (penanganan gambar), §6.8 (pemisahan tanggung jawab, kunci dimiliki Go), §7 (status kegagalan wajah) |
| `docs/OPEN_QUESTIONS.md` | Q-16 (**DIPUTUSKAN** untuk rancangan MVP melalui D-030; implementasi dan pengujian masih tertunda), Q-07 (angka ambang kalibrasi — tetap terbuka), Q-18 (batas laju/ukuran gambar — tetap terbuka) |

Tidak ada angka ambang kecocokan yang ditetapkan di dokumen ini; Q-07 tetap pekerjaan kalibrasi pada fase AI Engine. Kebijakan pengelolaan kunci pada §5a–§5f **berlaku** sejak D-030 disetujui pemilik project pada FL-004S, **tetapi belum diimplementasikan dan belum diuji**. **Periode pemulihan 30 hari pada §5d adalah batas waktu upaya operator, bukan jaminan pemulihan kunci**, dan **penghapusan pada §5f tidak menjamin data hilang secara fisik dari media penyimpanan**.

Lihat pula `TEST_PLAN.md`, `FACE_ENGINE.md`, dan `REFERENCES.md`.

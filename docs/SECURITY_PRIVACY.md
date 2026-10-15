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
- Kunci enkripsi **tidak** boleh berada di dalam database, di dalam commit, atau di berkas konfigurasi yang ikut ter-commit. Bentuk penyimpanan final (AES-256-GCM per baris, D-022); kebijakan pengelolaannya diusulkan pada **D-030 (USULAN, menunggu persetujuan pemilik project)** dan dirinci pada §5a–§5d di bawah. Sampai D-030 disetujui, **Q-16 tetap terbuka dan implementasi penyimpanan biometrik tetap terhambat.**
- Enkripsi template bukan enkripsi seluruh berkas database: rekap kehadiran dan nama peserta tetap terbaca bagi siapa pun yang memegang berkas SQLite. Jangan mengklaim sebaliknya.
- Cadangan data, jika benar-benar dibutuhkan, harus tunduk pada retensi/penghapusan yang sama; jangan menyisakan template yang sudah dicabut di backup tanpa kebijakan.
- **Pencabutan persetujuan tetap merupakan penghapusan, bukan penonaktifan.** Kunci tidak menyimpan salinan template, sehingga penghapusan pada `face_templates` (`docs/DATA_MODEL.md` §5.3) bersifat final dan tidak dapat dibatalkan dengan memulihkan kunci (D-030 butir 7).
- **Cadangan database bukan cadangan biometrik.** Berkas `facelink.db`, `-wal`, dan `-shm` hanya berguna bersama berkas kunci terlindungi. Cadangan yang hanya berisi database harus dijelaskan apa adanya kepada pemilik project, bukan disajikan sebagai pemulihan penuh.

### 5a. Kebijakan kunci yang diusulkan (D-030, belum disetujui)

Bagian ini memuat **usulan** yang menjawab Q-16. Berlaku sebagai kebijakan hanya setelah status D-030 berubah menjadi DISETUJUI.

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
- [ ] **Ketiga kondisi §5d.1 dapat dibedakan pada mesin nyata:** kunci sementara tidak tersedia, kunci ada tetapi tidak dapat dipakai, dan kunci dipastikan tidak dapat dipulihkan. Hasilnya dicatat apa adanya.
- [ ] **Kegagalan dekripsi sementara tidak menghapus template** — diuji dengan mengunci/menyembunyikan berkas kunci lalu memastikan jumlah baris `face_templates` tidak berubah.
- [ ] Batas DPAPI dijelaskan kepada pemilik project dengan kalimat §5b, bukan diringkas menjadi "kunci sudah terenkripsi".
- [ ] Gagal koneksi, wajah asing, dan hasil ambigu tidak memproduksi check-in.
- [ ] Pembatasan keamanan kamera/jaringan lokal dan kekurangan anti-spoofing dijelaskan.
- [ ] Lisensi model pihak ketiga ditinjau.

## 9. Keselarasan dengan keputusan dan kontrak lain

| Dokumen | Bagian yang mengikat |
|---|---|
| `docs/DECISIONS.md` | D-015 (template terenkripsi, tanpa foto mentah), D-022 (AES-256-GCM per baris), **D-030 (kebijakan kunci — USULAN)**, D-023 (kontrak kamera JPEG), D-025 (lisensi bobot model), D-027 (Go pemilik keputusan dan database), D-028 (Go sumber kebenaran, UI bukan penentu), D-029 (liveness terpisah) |
| `docs/DATA_MODEL.md` | §4.12 `face_templates` (kolom, AAD, perilaku dekripsi gagal, penghapusan keras), §5.3–§5.4 (penghapusan dan pencabutan), §6, §7.1–§7.3 (WAL dan pencadangan), §11 butir 4–7 (larangan menyimpan kunci, cadangan, dan larangan menulis `face_templates` sebelum D-030) |
| `docs/API_CONTRACT.md` | §6.7 (penanganan gambar), §6.8 (pemisahan tanggung jawab, kunci dimiliki Go), §7 (status kegagalan wajah) |
| `docs/OPEN_QUESTIONS.md` | Q-16 (pengelolaan kunci — **blocker** penyimpanan biometrik, usulan ada di D-030), Q-07 (angka ambang kalibrasi), Q-18 (batas laju/ukuran gambar) |

Tidak ada angka ambang kecocokan yang ditetapkan di dokumen ini. Kebijakan pengelolaan kunci pada §5a–§5d berstatus **usulan** sampai pemilik project menyetujui D-030. **Periode pemulihan 30 hari pada §5d adalah batas waktu upaya operator, bukan jaminan pemulihan kunci.**

Lihat pula `TEST_PLAN.md`, `FACE_ENGINE.md`, dan `REFERENCES.md`.

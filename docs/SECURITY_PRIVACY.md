# Keamanan, Privasi, dan Persetujuan Biometrik

**Status:** Persyaratan minimal untuk MVP akademik. Dokumen ini adalah panduan engineering, **bukan nasihat hukum**. Untuk penggunaan institusional, konsultasikan pengelolaan data dan kebijakan yang berlaku.

**Revisi FL-010U (16 Oktober 2026).** Dokumen ini diselaraskan dengan keputusan FL-010U: **tiga role pengguna manusia (`admin`, `dosen`, `mahasiswa`) dengan Google Sign-In**, dan `device` sebagai **identitas mesin terpisah** yang tidak berubah. Revisi ini bercabang dari `origin/main` (`804c2f7`). Matriks peran dan izin yang mengikat ada di `docs/AUTHORIZATION.md`; seluruh isi yang sudah disetujui — khususnya **D-022, D-030, dan §5a–§5f** — **tidak diubah** oleh revisi ini. Rancangan autentikasi Google dan sesi aplikasi pada dokumen ini berstatus **rancangan** dan **BELUM TERVERIFIKASI** pada runtime mana pun.

**Catatan istilah.** Beberapa bagian lama masih memakai kata "operator" sebagai sebutan umum bagi manusia yang menjalankan sistem. Sejak FL-010U, sebutan itu merujuk pada role yang berlaku menurut `docs/AUTHORIZATION.md` — `admin` untuk operasi administratif dan perangkat, `dosen` untuk kelas yang ditugaskan kepadanya. Kata "operator" pada kutipan keputusan lama **tidak dihapus** agar riwayatnya tetap dapat ditelusuri; ia **bukan** role tersendiri pada FL-010U.

**Revisi FL-010V (16 Oktober 2026).** Dokumen ini diselaraskan dengan keputusan final FL-010V: **D-032 (lifecycle akun), D-033 (sesi token opak + rotasi + batas absolut 30 hari), D-034 (hak akses `dosen`), D-035 (LAN privat + HTTPS tepercaya — tingkat kebijakan), D-036 (perluasan daftar audit keamanan — sekarang BERLAKU), dan D-037 (satu role utama per akun)**. Yang berubah: §2 (baris hash nonce `auth_nonces`), §5g (satu kalimat penegas Q-22, tanpa mengubah paragraf FL-010U), §6 (daftar audit berlaku, larangan isi audit, aturan transport LAN, aturan penyimpanan token React Native), §8 (butir uji baru), dan §9 (tabel keselarasan). **D-030, §5a–§5f, Q-07, matriks 53 endpoint, nama peran, dan kode kesalahan tidak diubah.** Seluruh skenario runtime tetap **BELUM TERVERIFIKASI**.

**FL-OTP01 / D-038:** Email OTP utama mengikuti [`EMAIL_OTP.md`](EMAIL_OTP.md):
8 digit CSPRNG, 5 menit, verifier HMAC key server terlindungi, consume atomik,
limit persisten dan respons request anti-enumeration. SMTP TLS tervalidasi,
From terotorisasi; kode/token/SMTP secret dilarang di log/Git. Email OTP bukan
MFA tahan phishing: passkey atau TOTP direkomendasikan sebelum produksi/data
sensitif. Admin lama tidak dihapus atau diganti emailnya selama transisi.

**FL-STAB01 (27 Oktober 2026):** logout memakai refresh credential dalam JSON
melalui HTTPS, bukan access token expired atau ID sesi dari client. Current/
immediately previous hash hanya mencabut satu sesi dengan audit atomik; logout
tidak memicu aturan refresh-reuse lintas perangkat. Rotasi/replay dan batas
absolut 30 hari tetap. Credential tidak masuk URL/log. SecureStore load/save
difence terhadap logout; pending acceptance/refresh writes diselesaikan sebelum
penghapusan akhir. Jaringan gagal berarti revocation remote belum terkonfirmasi,
bukan logout server sukses; kegagalan penghapusan lokal juga ditampilkan.
Lifecycle video menghentikan animasi dengan ownership invalidation sebelum Expo
melepas player; tanpa suppression/catch-all. Bukti dan residual:
[`FL_STAB01_TECHNICAL_REPORT.md`](FL_STAB01_TECHNICAL_REPORT.md).

**FL-ENR04 (27 Oktober 2026):** model nyata hanya untuk data sintetis dan
staging disposable. Pipe privat HMAC memakai secret acak per child dalam
environment minimal, UUID/digest/manifest terikat, request/response bounded,
timeout10detik dan kill/reap. JPEG dibatasi sebelum decode; dua checksum
diverifikasi sebelum model load. Python tidak menerima key enkripsi/DB dan
tidak memutuskan ACTIVE. Tidak ada image/vector temp file atau payload log.
Ini bukan sandbox OS: operator/source/interpreter/model directory dipercaya,
tidak ada hard memory cap atau jaminan zeroization, dan marker synthetic_only
bukan klasifikasi asal gambar. Tidak mengubah D-030 atau izin manusia/production.
Bukti: [`FL_ENR04_TECHNICAL_REPORT.md`](FL_ENR04_TECHNICAL_REPORT.md).

**FL-SEC01 — kontrak remediasi yang disetujui pemilik:** template envelope/AADv2
mengikat domain/purpose/version FaceLink, template UUID, submission UUID, person,
immutable consent grant UUID, model ID/version, stable model/pipeline fingerprint
dan key version. ID template dibuat sebelum encryption; expected binding
direkonstruksi dari relasi target tervalidasi. Format exact DATA_MODEL §4.12.
Withdrawal tidak mengganti grant identity; authorization tetap diperiksa dan
payload dihapus sesuai kontrak. Pending-photo v1 tidak berubah.

Template v1/unknown ditolak tanpa fallback; tidak ditulis lagi, tidak ready,
dan dipertahankan sebagai quarantine. Migration terpisah00008 hanya disposable,
tidak mengonversi ciphertext v1. Data non-disposable tidak dibaca/dimigrasi/
dihapus oleh tugas ini; jika ditemukan perlu containment dan rencana kompatibilitas
yang diotorisasi terpisah. KeyProvider/purpose separation/AES256GCM tetap.
Bindingv2 menolak transfer envelope ke target identity/metadata berbeda; **tidak**
mengklaim anti-rollback seluruh DB/state atau perlindungan bila key dikuasai.
SQL guard, consent/reviewer eligibility, lease dan audit tetap kontrol tambahan.
D-030 recovery timer/irreversible deletion tetap BLOCKED.

**FL-ENR05 — GO Synthetic Integrated E2E:** normalOTP dan verifiedHTTPS actual
TS clients→Go→real YuNet/SFace→AADv2→ACTIVE→withdraw/REVOKED/cleanup0 pada satu
SQLite WAL disposable. Tidak mengaktifkan enrollment pada command produk.
Staging controls/safe snapshot/OTP delivery barrier bounded hanya stdin;
tidak membaca verifier, kunci atau token untuk output dan tidak mengubah auth.
Evidence hanya hash/scalar/UUID sintetis/aksi audit; tanpa JPEG/vector/OTP/key.
Client token-null membuat cached ACTIVE stale sampai backend dibaca ulang.
Real commit dengan response hilang tetap uncertain; exact replay tidak
menggandakan approval/audit. Snapshot cleanup dijalankan setelah reconciliation.
Independent review menutup temuan freshness, readiness OTP dan timing test;
bukan bukti Android rendering, zeroization, forensic erase, antirollback,
physical network-disconnect atau OS-process-kill recovery. D-030, backup/
insiden, native cache/PAD/kalibrasi/F3 tetap residual. Tidak ada human/camera/
ADB/productionDB atau perubahan SMTP/role/session policy.
Laporan lengkap: [`FL_ENR05_TECHNICAL_REPORT.md`](FL_ENR05_TECHNICAL_REPORT.md).

Paragraf R2 berikut adalah temuan historis sebelum remediasi FL-SEC01.

**FL-ENR04-R2:** positive pipeline sintetis dan encrypted ACTIVE→withdraw terbukti
melalui HTTPS disposable. **Celah binding AAD v1 terkonfirmasi:** template AAD
mengikat person/model/model-version/key-version, bukan template/submission ID.
Pada dua snapshot SQLite terisolasi, pertukaran seluruh envelope dengan metadata
target tetap menghasilkan payload record lain (2/2). UNIQUE(person_id) dan trigger
immutability menolak coexistence/UPDATE normal; diagnostik memodelkan penulis DB
offline yang dapat melewati guard, **bukan exploit endpoint**. AAD/key provider/
ciphertext existing tidak diubah. Usulan AADv2 mengikat immutable template_id,
dengan keputusan eksplisit mengenai submission/consent/fingerprint dan penolakan
v1 pada consumer baru. **NO-GO FL-ENR05** sampai keputusan/remediasi yang disetujui.
Bukti: [`FL_ENR04_R2_TECHNICAL_REPORT.md`](FL_ENR04_R2_TECHNICAL_REPORT.md).


**Cutover FL-ENR01B (26 Oktober 2026): DESIGN APPROVED / SYNTHETIC IMPLEMENTATION AUTHORIZED.** Pengecualian terbatas pending-photo persistence: satu JPEG canonical sintetis/submission dalam AES-256-GCM private file di luar repo/static/public directory; purpose `pending-photo` terpisah dari `template`/OTP. TTL72jam sejak penerimaan pertama backend, tidak diperpanjang retry/preview. Mahasiswa self-consent/withdraw; Admin review/assisted withdrawal; Dosen tidak. Consent bukan syarat absensi manual. Tidak ada izin manusia/production/migration Product Owner; notice1 draft sintetis, metadata retention/recovery irreversible D-030 belum final. Payload tidak masuk backup rutin.

**FL-ENR03 Admin review UI (27 Oktober 2026):** staging Admin hanya memakai fixture sintetis, default-off di main, dan endpoint enrollment existing. Preview mengambil JPEG dengan `Authorization` header, `Cache-Control: no-store`, `credentials: omit`, `redirect: error`, batas 1 MiB, serta validasi MIME/magic byte; tidak ada token di URL, path file, gallery, AsyncStorage, analytics, atau log payload. Web memakai Blob URL yang direvoke saat unmount; native memakai data URI tanpa API filesystem. Menurut source RN/Fresco, data URI tidak membuat entry disk-cache, tetapi bitmap dapat tetap berada di bitmap memory cache Fresco dan tidak ada API JS untuk evict satu gambar; ini **bukan klaim zeroization**. Riwayat review dibatasi50 event terpilih dan metadata mentah dibuang. Login OTP web tetap tidak didukung karena SecureStore; jangan menambah bypass hanya untuk smoke test.

**FL-ENR02A staging UI (26 Oktober 2026):** aplikasi `research/fl-enr02a-host/stage` memakai package terpisah `ai.facelink.enr02host`, menu utama default-off, dan route enrollment hanya ada di staging. Alur consent → scan → submission memakai JPEG pola sintetis di memori; tidak ada permission kamera, preview gambar, gallery, file capture, AsyncStorage, analytics, atau log JPEG/base64/token. Consent dan status berasal dari backend; label “Wajah terdaftar” hanya muncul untuk snapshot ACTIVE + `template_ready` + consent ACTIVE yang masih segar. Build utama, SecureStore, database Product Owner, dan migrasi production tidak diubah. Ini bukan bukti capture manusia, akurasi, atau kesiapan production.

**Batas operasional implementasi sintetis:** normalisasi ≤1 MiB/2.073.600 pixel
dengan slot decode terbatas; EXIF tidak normal/berbahaya ditolak dan output
di-encode ulang tanpa metadata. Preview Admin memerlukan pemeriksaan sebelum
decrypt dan pemeriksaan ulang + audit sebelum respons; URL bukan public,
`no-store`/`nosniff`, tanpa Range. Byte yang sudah diotorisasi/in-flight tidak
dapat ditarik kembali oleh withdrawal; permintaan berikutnya ditolak.
Kegagalan key/integritas tidak menjadi izin penghapusan otomatis. Foto retained
pada `FAILED_PROCESSING / BIOMETRIC_UNAVAILABLE` hanya boleh di-retry setelah
guard/hash/decrypt pulih, sebelum TTL dan tanpa memperpanjangnya. Withdrawal
eksplisit dan expiry foto tetap menjalankan logical revoke serta cleanup durable.
Default discovery `migrations/` tidak memuat migrasi enrollment sintetis;
`synthetic-migrations/00007_synthetic_enrollment.sql` hanya digabung oleh harness
ke direktori disposable baru. Server biasa tidak menginjeksi layanan enrollment.


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
| Identitas Google (`sub`, email akun) | Pribadi | `sub` adalah pengenal stabil dan menjadi kunci penautan pada `user_identities`; email disimpan huruf kecil tanpa spasi tepi. Dipakai **hanya** untuk menautkan identitas ke akun, bukan untuk menentukan hak akses; tidak boleh masuk log, audit `safe_metadata`, atau LLM API |
| Token sesi aplikasi (token akses dan token penyegar) | Rahasia | Token opak 256-bit; server hanya menyimpan **hash SHA-256** (`session_tokens`). Token mentah tidak pernah dicatat, tidak masuk Git, dan disimpan di penyimpanan aman perangkat (Android Keystore) |
| Nonce login Google (hash pada tabel `auth_nonces`) | Internal (teknis) | Data teknis internal, bukan rahasia pengguna, tetapi tidak boleh dibocorkan: nilai acak sekali pakai dengan masa berlaku **5 menit**, disimpan **hanya sebagai hash**, dan ditolak bila dipakai ulang. Tidak pernah masuk log, `audit_events`, atau LLM API |
| Log kesehatan perangkat | Operasional | Tanpa JPEG/embedding dan tanpa identitas yang tidak perlu |

**FL-010V (16 Oktober 2026)** menambahkan baris nonce di atas sebagai data teknis internal. Nonce **bukan** kredensial pengguna dan **bukan** pengganti sesi aplikasi; ia hanya penanda sekali pakai yang mengikat satu percobaan login Google ke satu ID token, dan karena itu diperlakukan sebagai rahasia transport, bukan sebagai data pribadi. Baris biometrik dan baris identitas/sesi yang sudah ada **tidak diubah**.

Catatan FL-010U: **Google ID token bukan data yang disimpan.** Ia hanya bukti yang diverifikasi pada `POST /api/v1/auth/google` (§6), tidak pernah dijadikan sesi aplikasi, dan tidak pernah ditulis ke log atau `audit_events`. Klasifikasi pada tabel di atas menambahkan dua baris identitas/sesi tanpa mengubah baris biometrik yang sudah ada.

## 3. Persetujuan dan alternatif

- Sebelum enrollment, jelaskan: tujuan absensi, data yang diambil, lokasi pemrosesan, siapa yang bisa mengakses, masa simpan, cara pencabutan izin, dan alternatif manual.
- Peroleh tindakan persetujuan yang **jelas, sukarela, dan terdokumentasi**. Jangan menganggap kehadiran di ruang kelas sebagai persetujuan biometrik.
- Peserta yang menolak harus tetap dapat hadir melalui absensi manual tanpa perlakuan merugikan.
- Setelah izin dicabut, hentikan proses biometrik orang tersebut dan hapus template dari disk, database, indeks, cache, serta backup bila kebijakan dan mekanismenya mengharuskan; catat penyelesaiannya.
- Pemrosesan dan penyimpanan data untuk jurnal atau publikasi akademik memerlukan kajian dan izin tersendiri; jangan mengambil otomatis dari sistem produk.

## 4. Ancaman utama dan mitigasi

| Ancaman | Mitigasi awal |
|---|---|
| Akses pengguna tidak sah | Google Sign-In (OpenID Connect) untuk `admin`/`dosen`/`mahasiswa`, otorisasi tiap endpoint, pembatasan percobaan. Baris lama "Login, hash kata sandi aman" **DIGANTIKAN pada FL-010U (16 Oktober 2026)**: `users.password_hash` **dihapus** dan tidak ada login email/kata sandi buatan sendiri |
| Token Google palsu atau token milik aplikasi lain dipantulkan ke layanan ini | Verifikasi penuh ID token sesuai OpenID Connect: tanda tangan lewat **JWKS** (`kid`, kunci di-cache dan dirotasi bila `kid` tidak dikenal); `alg` **wajib** RS256 (menolak `none`, HS256, dan algoritme simetris); `iss` ∈ {`https://accounts.google.com`, `accounts.google.com`}; `aud` ∈ daftar client ID resmi yang dikonfigurasi (**tidak ada client ID yang ditanam di kode**); `exp`/`iat` dengan toleransi selisih waktu terbatas (usulan 120 detik); `nonce` cocok bila dipakai; `azp` diperiksa bila token punya beberapa audience; `email_verified` = true. Gagal → `401 IDENTITY_TOKEN_INVALID`; kunci/penyedia tidak dapat dihubungi → `503 GOOGLE_UNAVAILABLE` |
| Orang dengan akun Google sah tetapi tanpa izin | Akun **hanya** dibuat oleh `admin`; tidak ada pendaftaran mandiri dan tidak ada role otomatis. Penautan hanya bila sudah ada baris `users` berstatus `invited` dengan email **sama persis** dan `email_verified=true`; selain itu **tidak ada akun yang dibuat**. Penolakan memakai **satu pesan generik** `403 IDENTITY_NOT_LINKED` untuk semua sebab (anti-enumeration). Role **tidak pernah** diambil dari Google, dari klien, atau dari klaim token |
| Pencurian token sesi | Token **opak** acak 256-bit; server **hanya** menyimpan hash SHA-256 pada `session_tokens`. Rotasi: setiap `POST /auth/refresh` menerbitkan pasangan token baru. Penyegar yang sudah dirotasi lalu dipakai ulang → **seluruh sesi dicabut** + audit `auth.session.reuse_detected`. Penyimpanan di perangkat memakai penyimpanan aman (Android Keystore); token mentah dilarang di log, Git, dan berkas konfigurasi |
| Perubahan role tidak segera berlaku | Peran **selalu dibaca ulang dari database pada setiap permintaan**; token tidak memuat klaim peran, sehingga perubahan role dan penonaktifan akun berlaku pada permintaan berikutnya, bukan menunggu token kedaluwarsa. Penonaktifan akun mencabut seluruh sesi akun itu (`401 SESSION_REVOKED`) |
| Google ID token dipakai sebagai sesi aplikasi | **Dilarang keras.** ID token Google **bukan** pengganti sistem otorisasi internal dan **tidak pernah** disimpan sebagai sesi aplikasi: umurnya pendek, tidak dapat dicabut, dan tidak memuat keputusan otorisasi internal |
| Perangkat palsu mengirim absensi | Kredensial khusus device, daftar perangkat diizinkan, rotasi/revokasi. `device` tetap **identitas mesin terpisah** dan tidak berubah oleh FL-010U |
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
- Kunci enkripsi **tidak** boleh berada di database, commit, atau konfigurasi yang ikut ter-commit. D-022/D-030 dan §5a–§5f tetap kebijakan mengikat. FL-ENR01B mengimplementasikan AES-256-GCM dan KeyProvider hanya untuk fixture sintetis/disposable dengan provisioning eksplisit; bukan izin menyimpan biometrik manusia atau klaim production security.
- Enkripsi template bukan enkripsi seluruh berkas database: rekap kehadiran dan nama peserta tetap terbaca bagi siapa pun yang memegang berkas SQLite. Jangan mengklaim sebaliknya.
- Cadangan data, jika benar-benar dibutuhkan, harus tunduk pada retensi/penghapusan yang sama; jangan menyisakan template yang sudah dicabut di backup tanpa kebijakan.
- **Pencabutan persetujuan tetap merupakan penghapusan, bukan penonaktifan.** Kunci tidak menyimpan salinan template, sehingga penghapusan pada `face_templates` (`docs/DATA_MODEL.md` §5.3) bersifat final dan tidak dapat dibatalkan dengan memulihkan kunci (D-030 butir 7).
- **Cadangan database bukan cadangan biometrik.** Berkas `facelink.db`, `-wal`, dan `-shm` hanya berguna bersama berkas kunci terlindungi. Cadangan yang hanya berisi database harus dijelaskan apa adanya kepada pemilik project, bukan disajikan sebagai pemulihan penuh.

### 5a. Kebijakan kunci yang berlaku (D-030)

Bagian ini memuat **kebijakan yang menjawab Q-16**. D-030 **DISETUJUI pemilik project pada 15 Oktober 2026 (FL-004S)** dengan dua klarifikasi wajib pada §5e dan §5f. FL-ENR01B mengimplementasikan batas key/photo/template pada lingkungan sintetis saja. Konflik timer recovery 30 hari versus larangan menjadikan key-loss pemicu deletion otomatis tidak diselesaikan dengan TTL foto 72 jam; irreversible production deletion tetap **BLOCKED**.

1. **Pemilik kunci adalah backend Go.** Python tidak memegang kunci, tidak menerima kunci, dan tidak mendekripsi template. Ini memperluas D-027 dan `docs/API_CONTRACT.md` §6.8.
2. **Lokasi penyimpanan:** berkas kunci terlindungi di `%LOCALAPPDATA%\FaceLink\keys\` pada akun Windows yang menjalankan backend, dengan ACL yang hanya mengizinkan akun itu. Jangan memakai folder yang disinkronkan ke penyimpanan awan.
3. **Bentuk perlindungan:** materi kunci dibungkus **Windows DPAPI mode pengguna** (`CryptProtectData`, tanpa `CRYPTPROTECT_LOCAL_MACHINE`). Mode mesin **ditolak** karena akan membuka kunci bagi setiap pengguna mesin tersebut.
4. **Satu pintu akses:** seluruh kode memakai antarmuka `KeyProvider` di Go untuk mengambil kunci aktif dan kunci berdasarkan versi. Tidak ada modul lain yang membaca berkas kunci langsung, dan tidak ada operasi yang mengembalikan kunci dalam bentuk yang dapat dicetak ke log.
5. **Penamaan versi:** `key_version` adalah **teks**, dimulai dari `'1'`, naik bilangan bulat. Nilai ini menjadi bagian AAD (D-022), sehingga baris dengan versi `'1'` tidak dapat dibaca memakai kunci berlabel `'2'`.
6. **Tidak ada kunci default, tidak ada kunci contoh, dan tidak ada kunci yang dibangkitkan diam-diam.** Bila kunci aktif tidak ada saat pertama kali dibutuhkan, **layanan biometrik** menolak bekerja dengan pesan jelas dan instruksi menyiapkan kunci — **bukan** dengan membuat kunci baru, dan **bukan** dengan mematikan seluruh backend (§5e). Kunci yang dibuat otomatis pada mesin yang salah adalah cara kehilangan template secara diam-diam; matinya seluruh backend adalah cara kehilangan absensi manual.
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

**Bukti parsial availability D-030 pada FL-CORE02 (25 Oktober 2026):** sesi dan
fallback manual terautentikasi berjalan dengan biometrik belum diimplementasikan.
Readiness memisahkan `manual_attendance` dari `biometrics`; komponen biometrik
tetap unavailable dengan kode BIOMETRIC_NOT_IMPLEMENTED, tanpa menghalangi core
yang sehat. Person roster terverifikasi tetap wajib, consent biometrik tidak
menjadi prasyarat manual. Alasan fallback/koreksi tersimpan pada record dan
audit.reason; metadata status/waktu/source/actor tidak memuat nama/NIM/alasan
bebas, dan log tidak memuat payload. Audit gagal menyebabkan rollback.
Uji dan smoke hanya disposable/sintetis, tidak menjalankan engine production.
**Ini bukan penutupan acceptance KeyProvider/key-loss/enkripsi/deletion D-030:**
komponen tersebut belum diimplementasikan. Perlindungan auth, scope dan integritas
tetap diwajibkan; tidak ada jalur biometrik atau keberhasilan biometrik palsu.


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

### 5g. Penegasan FL-010U: autentikasi Google bukan persetujuan biometrik

**Ditambahkan pada FL-010U (16 Oktober 2026).** Seluruh keputusan biometrik pada §5 dan §5a–§5f di atas **tidak diubah** oleh revisi ini; paragraf ini hanya menegaskan batasnya. **Berhasil masuk dengan Google Sign-In bukan persetujuan pemrosesan wajah.** Persetujuan biometrik tetap berdiri sendiri, terpisah, tercatat, dan dapat dicabut (§3), dan seorang `mahasiswa` yang berhasil masuk lalu melihat riwayat kehadirannya sendiri **tidak** memperoleh akses ke `face_templates`, embedding, atau data peserta lain: penautan akun ke baris peserta lewat `people.account_user_id` **tidak** membuka akses ke template (lihat pula `docs/AUTHORIZATION.md` §2 dan §4). Hak akses mahasiswa terbatas pada data miliknya sendiri melalui endpoint `/me/*`, sedangkan enrollment, pencatatan konsen, dan pengelolaan peserta tetap jalur `admin`.

**FL-010V (16 Oktober 2026).** Paragraf penegas di atas **tidak diubah**: penautan akun peserta **tidak** membuka template biometrik. Yang ditambahkan hanyalah penegasan status pertanyaan terkait: **Q-22 (penarikan persetujuan mandiri oleh mahasiswa) tetap TERBUKA** dan **tidak** disetujui oleh FL-010V. Penarikan konsen tetap jalur `admin` sampai ada keputusan pemilik; tidak ada endpoint `/me/*` yang boleh mencabut konsen atau menghapus template.

## 6. Pengendalian akses dan transportasi

**Matriks peran dan izin yang mengikat ada di `docs/AUTHORIZATION.md`.** Bagian ini menyebut prinsipnya; dokumen itu menyebut per-endpoint.

**FL-010U (16 Oktober 2026) — tiga peran manusia dan identitas mesin.** Ada **tiga role pengguna manusia** — `admin`, `dosen`, dan `mahasiswa` — yang semuanya masuk memakai **Google Sign-In**, ditambah **`device` sebagai identitas mesin terpisah** yang tidak memakai Google dan tidak dapat menggantikan sesi manusia. `dosen` dan `mahasiswa` hanya dapat melihat sesuai cakupan yang diberikan `admin` (`dosen`: kelas yang ditugaskan; `mahasiswa`: data miliknya sendiri melalui `/me/*`); rincian matriks ada di `docs/AUTHORIZATION.md` §2 dan §4. Google **hanya** penyedia identitas: ia **tidak** menentukan hak akses, dan role tidak pernah diambil dari Google, dari klien, atau dari klaim token.

**Aturan sesi aplikasi (rancangan MVP; diimplementasikan pada FL-011, 16 Oktober 2026).** Sesi FaceLink AI memakai token **opak** 256-bit, bukan JWT. Masa berlaku token akses **60 menit** dan token penyegar **30 hari**, dengan **batas absolut sesi 30 hari** yang tidak diperpanjang tanpa batas; setelah itu pengguna **wajib** Google Sign-In lagi. Server hanya menyimpan **hash SHA-256** pada `session_tokens`. Pencaburan: `POST /auth/logout` mencabut sesi itu (panggilan kedua tetap `204`); `DELETE /auth/sessions/{session_id}` mencabut per sesi oleh `admin`; penonaktifan akun mencabut **seluruh** sesi akun tersebut; pemakaian ulang penyegar yang sudah dirotasi mencabut seluruh sesi disertai audit `auth.session.reuse_detected`. **Efek pencaburan berlaku lintas sesi dan langsung:** peran dibaca ulang dari database pada setiap permintaan, sehingga token yang masih berlaku pun kehilangan haknya pada permintaan berikutnya (`401 SESSION_REVOKED`).

**Nonce sekali pakai (FL-011, 16 Oktober 2026).** Nonce **diterbitkan backend** lewat `POST /api/v1/auth/nonce` (256 bit acak, TTL 5 menit, hanya hash SHA-256 yang disimpan pada `auth_nonces`). Aplikasi memakainya pada permintaan otorisasi Google dan mengirimkan nilai yang sama bersama ID token ke `POST /api/v1/auth/google`. Nonce yang tidak terdaftar, sudah dipakai, atau kedaluwarsa ditolak `401 IDENTITY_TOKEN_INVALID`; pemakaian nonce bersifat atomik (satu pernyataan `DELETE` bersyarat), sehingga dua permintaan bersamaan dengan nonce yang sama hanya satu yang berhasil.

**Penyensoran log (FL-011, 16 Oktober 2026).** Rahasia yang tidak sengaja terbawa nilai panic atau pesan galat disensor di **handler log** (`slog` `ReplaceAttr`), bukan bergantung pada disiplin tiap pemanggil. `logging.PanicSummary` mencatat jenis nilai panic beserta pesan yang sudah disensor (bukan isi mentah), dan `logging.StackTrace` menyimpan jejak fungsi/berkas/baris untuk diagnosis tanpa isi nilai. Isi panic mentah **tidak pernah** masuk log — temuan FL-005S ditutup pada FL-011.

**Larangan menyimpan ID token Google sebagai sesi.** Google ID token **tidak boleh** dijadikan token sesi aplikasi, tidak boleh disimpan di perangkat sebagai kredensial, dan tidak boleh dicatat. Ia hanya bukti identitas berumur pendek yang diverifikasi pada `POST /api/v1/auth/google`, tidak dapat dicabut, dan tidak memuat keputusan otorisasi internal.

- Setiap role hanya mendapat hak sesuai kebutuhannya; token perangkat tidak boleh digunakan untuk endpoint `admin`, `dosen`, atau `mahasiswa`.
- Endpoint AI Engine hanya untuk backend lokal; batasi ke loopback `127.0.0.1`/IPC dengan kontrak internal `docs/API_CONTRACT.md` §6, dan jangan pernah membukanya pada jaringan kelas.
- Hanya backend Go yang boleh menulis database dan menetapkan kehadiran (D-027); AI Engine tidak memegang kredensial database, tidak membaca tabel biometrik, dan tidak boleh menyimpan embedding di disk.
- Hindari hardcode IP, password, kunci, atau token dalam firmware dan aplikasi. Rahasia perangkat bisa diprovisikan saat demo dan diganti bila bocor.
- Semua penyimpanan, koneksi, dan alur rilis yang terkait biometrik harus diuji sebelum penggunaan orang sungguhan.
- **Deny by default:** endpoint hanya dapat diakses bila peran pemanggilnya tercantum eksplisit. **Tiga jalur autentikasi yang tidak dapat saling menggantikan** adalah sesi pengguna manusia (`admin`/`dosen`/`mahasiswa` lewat Google Sign-In), **identitas mesin `device`** (token perangkat), dan token internal `ai-engine` (loopback) — lihat `docs/AUTHORIZATION.md` §1 dan §3. Akun `mahasiswa` **ada** pada FL-010U; keputusan lama "peserta tidak memiliki akun pada MVP" **DIGANTIKAN** (16 Oktober 2026) dan identitas peserta tetap merupakan **baris data** `people`, bukan kredensial terpisah.
- **Otorisasi diambil di backend Go, tidak di klien.** Menyembunyikan tombol pada aplikasi bukan pengendalian akses; setiap permintaan diperiksa ulang di server (D-028).
- **Tidak membocorkan keberadaan sumber daya.** Sumber daya di luar cakupan pemanggil menjawab `404`, bukan `403` (`docs/AUTHORIZATION.md` §5).
- **Gagal-aman.** Bila autentikasi atau otorisasi operator tidak dapat dijamin, endpoint operator **wajib** menolak melayani, sesuai §5e butir 4. Absensi manual tetap tersedia, tetapi **hanya** lewat endpoint terautentikasi, bukan lewat jalur yang melumpuhkan pemeriksaan keamanan.
- **Penolakan bermakna keamanan dicatat.** Penolakan pada jalur administratif dan absensi manual meninggalkan `audit_events` tanpa data biometrik, token, sandi, atau email (`docs/AUTHORIZATION.md` §6).

**FL-010V (16 Oktober 2026) — daftar audit keamanan berlaku (D-036).** Perluasan daftar minimum audit keamanan **bukan lagi usulan**: ia **BERLAKU** setelah D-036 disetujui pemilik. Daftar nama event final (selaras `docs/API_CONTRACT.md` §5.13):

`auth.google.succeeded`, `auth.google.linked`, `auth.login.denied` (dengan `reason` ber-kode: `no_account`, `not_linked`, `suspended`, `email_mismatch`), `auth.session.refreshed`, `auth.session.revoked`, `auth.session.reuse_detected`, `auth.logout`, `authz.denied`, `auth.device.mismatch`, `user.bootstrapped`, `user.invited`, `user.role_changed`, `user.status_changed`, `person.account_linked`, `person.account_unlinked`, `class.dosen_assigned`, `class.dosen_removed` — ditambah `consent.granted` dan `consent.withdrawn` yang **sudah berlaku sebelumnya**.

**Larangan isi audit (wajib).** `audit_events` dan `safe_metadata` **tidak boleh** memuat: password, access token, refresh token, Google ID token, embedding wajah, foto wajah, maupun kredensial perangkat. **Penegakan FL-011:** `internal/audit` menolak metadata terlarang pada tingkat penulisan (diuji `TestRecordMenolakMetadataTerlarang`), dan `TestLogDanAuditTidakMembocorkanRahasia` memverifikasi bahwa token, nonce, dan email tidak muncul pada log maupun metadata audit selama alur autentikasi berjalan. Yang dipakai adalah identifier internal dan metadata seminimal mungkin; sebab penolakan akun memakai `reason` ber-kode, **bukan** email.

**FL-010V (16 Oktober 2026) — transport LAN (D-035).** Aturan yang mengikat untuk akses perangkat ke backend:

- **Default development adalah loopback** (`127.0.0.1`) agar tidak ada eksposur tak sengaja. Backend **tidak boleh** dibuka ke internet publik, dan pengikatan LAN hanya boleh aktif lewat konfigurasi eksplisit.
- Komunikasi aplikasi → backend pada LAN **wajib HTTPS dengan sertifikat yang dipercaya perangkat**.
- **Dilarang**: `InsecureSkipVerify` atau ekuivalennya, dan menonaktifkan validasi TLS dengan cara apa pun.
- **Tidak direkomendasikan dan tidak boleh menjadi konfigurasi deployment: HTTP terbuka.** Membuka backend lewat HTTP untuk menghindari kerumitan sertifikat adalah **pelanggaran**, bukan solusi sementara. Bila TLS gagal, yang benar adalah memperbaiki sertifikat/konfigurasi server, **bukan** menurunkan transport ke HTTP.
- **Wi-Fi privat bukan pelindung token.** Jangan menganggap jaringan lokal yang privat sudah mengamankan access/refresh token; hanya TLS dengan sertifikat tepercaya yang memberi jaminan transport.
- Rincian pelaksanaan (CA lokal dengan SAN alamat IP LAN, reverse proxy lokal, `adb reverse`/loopback untuk development) dan batasan Android 7+ terhadap CA yang dipasang pengguna mengikuti D-035 dan **BELUM TERVERIFIKASI**; lihat pula §9.

**FL-010V (16 Oktober 2026) — penyimpanan token di React Native (D-033).** Token akses dan token penyegar **wajib** disimpan pada **penyimpanan kredensial platform** (Android Keystore lewat penyimpanan aman Expo). **Dilarang** menyimpan token sensitif dalam bentuk plaintext di `AsyncStorage` (atau penyimpanan tak terlindungi ekuivalen). Google ID token **bukan token otorisasi API** FaceLink AI: ia hanya bukti identitas berumur pendek pada `POST /api/v1/auth/google`, tidak disimpan sebagai kredensial, dan tidak pernah dipakai pada endpoint terproteksi.

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
- [ ] Token sesi pengguna (`admin`/`dosen`/`mahasiswa`) dan token perangkat memiliki izin berbeda; lihat butir tambahan FL-010U di bawah.
- [ ] Foto mentah tidak ditahan tanpa alasan/izin.
- [ ] Penghapusan template teruji pada disk, **cache**, **berkas `-wal`/`-shm`, dan salinan cadangan** (§5c, `docs/DATA_MODEL.md` §5.3).
- [ ] Berkas kunci tersedia, terlindungi DPAPI, dan **tidak** berada di dalam folder yang disinkronkan ke awan.
- [ ] Subsystem biometrik menolak operasi bila key tidak siap; **tidak** membuat replacement key otomatis. Core/auth/audit/manual tetap berjalan jika sehat (D-030 butir10/§5e; rekonsiliasi FL-ENR01B).
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

**Butir tambahan FL-010U (16 Oktober 2026) — autentikasi dan sesi.** Semua butir di bawah berstatus **BELUM TERVERIFIKASI** sampai benar-benar dijalankan dan hasilnya dicatat apa adanya.

- [ ] **Konfigurasi Google:** client ID resmi terdaftar pada konfigurasi backend (bukan ditanam di kode), **tanpa client secret di repositori** — backend hanya memverifikasi ID token sehingga tidak membutuhkan secret.
- [ ] **Token sesi tidak masuk log atau Git:** periksa keluaran log backend dan klien, `git status`, serta berkas konfigurasi; token akses/penyegar, ID token, dan `sub` tidak boleh muncul (audit `safe_metadata` juga tidak memuatnya).
- [ ] **Penonaktifan akun mencabut sesi:** akun yang dinonaktifkan langsung ditolak pada permintaan berikutnya meski tokennya belum kedaluwarsa (`401 SESSION_REVOKED`), dan seluruh sesinya tercabut.
- [ ] **Uji negatif token Google:** token palsu/tanda tangan salah, `aud` milik aplikasi lain, `iss` asing, token kedaluwarsa, `alg` simetris/`none`, dan `nonce` tidak cocok — semuanya ditolak `401 IDENTITY_TOKEN_INVALID`; kunci Google tidak dapat dihubungi → `503 GOOGLE_UNAVAILABLE`.
- [ ] **Uji penolakan di luar cakupan kelas:** `dosen` menolak kelas yang tidak ditugaskan, `mahasiswa` menolak data peserta lain, dan sumber daya di luar cakupan menjawab `404` (bukan `403`), sesuai `docs/AUTHORIZATION.md` §5.
- [ ] **Pemakaian ulang penyegar terdeteksi:** penyegar yang sudah dirotasi dipakai ulang → seluruh sesi dicabut dan tercatat `auth.session.reuse_detected`.
- [ ] **Penautan akun–peserta diuji:** akun `mahasiswa` tanpa tautan `people` menerima `409 ACCOUNT_NOT_LINKED` pada `/me/*`, dan tautan `people.account_user_id` terbukti **tidak** membuka akses ke `face_templates`.

**Butir tambahan FL-010V (16 Oktober 2026) — token, nonce, batas sesi, TLS LAN, dan audit.** Semua butir di bawah berstatus **BELUM TERVERIFIKASI** sampai benar-benar dijalankan dan hasilnya dicatat apa adanya. Tidak satu pun boleh "diluluskan" dengan melonggarkan pemeriksaan keamanan.

- [ ] **Token tersimpan aman di perangkat:** token akses dan penyegar tersimpan pada penyimpanan kredensial platform (Android Keystore lewat penyimpanan aman Expo); diperiksa bahwa **tidak ada** token plaintext pada `AsyncStorage`, berkas aplikasi, log, atau `git status`; ID token Google tidak disimpan sebagai kredensial.
- [ ] **Nonce sekali pakai:** nonce cocok diverifikasi, nilai disimpan **hanya sebagai hash** pada `auth_nonces`, kedaluwarsa 5 menit, dan **nonce yang dipakai ulang ditolak**; pemeriksaan mencakup nonce kedaluwarsa dan nonce milik percobaan login lain.
- [ ] **Batas absolut sesi 30 hari:** penyegaran berulang **tidak** memperpanjang batas absolut 30 hari sejak sesi awal dibuat; setelah batas tercapai, penyegaran ditolak dan pengguna wajib Google Sign-In ulang.
- [ ] **TLS LAN dengan sertifikat tepercaya:** akses perangkat ke backend lewat LAN berhasil hanya dengan HTTPS bersertifikat tepercaya perangkat; sertifikat tidak tepercaya ditolak, dan **tidak ada** jalur `InsecureSkipVerify`/penonaktifan validasi TLS pada konfigurasi mana pun.
- [ ] **Audit event keamanan terisi tanpa isi terlarang:** event D-036 benar-benar tercatat pada skenario terkait (`auth.google.succeeded`, `auth.google.linked`, `auth.login.denied`, `auth.session.refreshed`, `auth.session.revoked`, `auth.session.reuse_detected`, `auth.logout`, `authz.denied`, `auth.device.mismatch`, `user.bootstrapped`, `user.invited`, `user.role_changed`, `user.status_changed`, `person.account_linked`, `person.account_unlinked`, `class.dosen_assigned`, `class.dosen_removed`), dan isinya **tidak** memuat password, access token, refresh token, Google ID token, embedding, foto, maupun kredensial perangkat. Butir ini hanya sah bila **FL-005S** (penguatan log respons) sudah selesai — **FL-005S SELESAI secara implementasi pada 16 Oktober 2026** (cabang `fix/fl-005s-http-access-logging`; `backend/internal/httpapi/accesslog_test.go`), tetapi **belum di-merge** sampai PR ditinjau, sehingga hasil ujinya tetap **BELUM TERVERIFIKASI** — lihat `docs/TEST_PLAN.md` §4.

## 9. Keselarasan dengan keputusan dan kontrak lain

| Dokumen | Bagian yang mengikat |
|---|---|
| `docs/DECISIONS.md` | D-015 (template terenkripsi, tanpa foto mentah), D-022 (AES-256-GCM per baris), **D-030 (kebijakan kunci — DISETUJUI pada FL-004S dengan dua klarifikasi wajib; tidak diubah FL-010V)**, D-023 (kontrak kamera JPEG), D-025 (lisensi bobot model), D-027 (Go pemilik keputusan dan database), D-028 (Go sumber kebenaran, UI bukan penentu), D-029 (liveness terpisah); **FL-010V: D-032 (lifecycle akun), D-033 (sesi token opak + rotasi + batas absolut 30 hari), D-034 (hak akses `dosen`), D-035 (LAN privat + HTTPS tepercaya — tingkat kebijakan), D-036 (perluasan audit keamanan — berlaku), D-037 (satu role utama per akun)** |
| `docs/DATA_MODEL.md` | §4.12 `face_templates` (kolom, AAD, perilaku dekripsi gagal, penghapusan keras), §5.3–§5.4 (penghapusan dan pencabutan), §6, §7.1–§7.3 (WAL dan pencadangan), §11 butir 4–7 (larangan menyimpan kunci, cadangan, dan larangan menulis `face_templates` sebelum D-030); **FL-010V: `users`, `user_identities`, `session_tokens`, `auth_nonces` (nonce sekali pakai disimpan sebagai hash, TTL 5 menit), `people.account_user_id`, `class_lecturers`** |
| `docs/API_CONTRACT.md` | §6.7 (penanganan gambar), §6.8 (pemisahan tanggung jawab, kunci dimiliki Go), §7 (status kegagalan wajah); **FL-010V: §5.13 (nama event audit keamanan yang berlaku menurut D-036)** |
| `docs/OPEN_QUESTIONS.md` | Q-16 (**DIPUTUSKAN** untuk rancangan MVP melalui D-030; implementasi dan pengujian masih tertunda), Q-07 (angka ambang kalibrasi — tetap terbuka), Q-18 (batas laju/ukuran gambar — tetap terbuka); **FL-010V: Q-22 (penarikan persetujuan mandiri mahasiswa — tetap TERBUKA), Q-24 (prosedur offline saat sesi tidak sah — tetap TERBUKA)** |
| `docs/UX_FLOWS.md` | **FL-010V: §2.1 dan §3 (keadaan sesi dicabut/pengguna dinonaktifkan kembali ke layar masuk Google, kegagalan TLS/sertifikat tidak tepercaya tanpa opsi menonaktifkan validasi, satu role utama per akun sehingga navigasi tidak berpindah role), §3a (prosedur offline saat sesi tidak sah — tanpa bypass autentikasi, rujuk Q-24)** |
| `docs/TEST_PLAN.md` | **FL-010V: §4 (persyaratan log akses mencatat status respons sebenarnya — kini SUDAH DIPENUHI oleh FL-005S, 16 Oktober 2026), §8.3 (GA-13..GA-18), §8.4 (TA-23..TA-26) — skenario GA/TA tetap BELUM TERVERIFIKASI** |
| `docs/AUTHORIZATION.md` | §2 (peran yang berlaku: `admin`, `dosen`, `mahasiswa` + `device` sebagai identitas mesin), §3 (tiga jenis autentikasi yang tidak dapat saling menggantikan), §4 (matriks otorisasi `endpoint × peran`), §5 (aturan kepemilikan/cakupan, `404` di luar cakupan), §6 (aturan 401 vs 403 dan audit). Sumber tunggal matriks peran/izin |
| `docs/ARCHITECTURE.md` | §1 (Google hanya penyedia identitas), §2 (diagram deployment dengan penyedia identitas Google), §3 (pembagian tanggung jawab klien/backend), §4 (modul `auth`, `accounts`, `self-service`), §5 (temuan loopback vs LAN), §9 (status keputusan otorisasi/autentikasi), §14 (ketergantungan internet) |
| `docs/API_CONTRACT.md` §1.2, §1.5 (kode kesalahan stabil autentikasi/otorisasi), dan §4 (Modul 1, 2, 14); `docs/DATA_MODEL.md` §4 (`users`, `user_identities`, `session_tokens`, `people.account_user_id`, `class_lecturers`) | Kontrak autentikasi Google, sesi, akun/role, dan portal self-service yang dirujuk dokumen ini |

Revisi FL-010U (16 Oktober 2026) menambahkan baris `docs/AUTHORIZATION.md` dan `docs/ARCHITECTURE.md` di atas serta memperbarui rujukan kontrak autentikasi/sesi. Baris biometrik pada tabel ini **tidak diubah**. Seluruh rancangan autentikasi Google dan sesi pada dokumen ini masih **BELUM TERVERIFIKASI** pada runtime; yang ada baru rancangan, bukan hasil pengujian.

**FL-010V (16 Oktober 2026).** Baris `docs/UX_FLOWS.md` dan `docs/TEST_PLAN.md` ditambahkan, dan baris D-032–D-037 serta `auth_nonces` diselaraskan. D-030, §5a–§5f, Q-07, dan seluruh aturan penghapusan biometrik **tidak diubah** oleh revisi ini. Status audit keamanan naik dari *usulan* menjadi **berlaku** menurut D-036, tetapi **kode audit belum ditulis dan belum diuji**; karenanya seluruh skenario runtime tetap **BELUM TERVERIFIKASI**.

Tidak ada angka ambang kecocokan yang ditetapkan di dokumen ini; Q-07 tetap pekerjaan kalibrasi pada fase AI Engine. Kebijakan pengelolaan kunci pada §5a–§5f **berlaku** sejak D-030 disetujui pemilik project pada FL-004S, **tetapi belum diimplementasikan dan belum diuji**. **Periode pemulihan 30 hari pada §5d adalah batas waktu upaya operator, bukan jaminan pemulihan kunci**, dan **penghapusan pada §5f tidak menjamin data hilang secara fisik dari media penyimpanan**.

Lihat pula `TEST_PLAN.md`, `FACE_ENGINE.md`, dan `REFERENCES.md`.

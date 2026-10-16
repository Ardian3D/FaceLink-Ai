# Catatan Keputusan FaceLink AI

Dokumen ini memisahkan apa yang **sudah disepakati**, apa yang **masih merupakan usulan**, dan apa yang **ditunda**. Agent dilarang mengubah status usulan menjadi disetujui tanpa konfirmasi pemilik project.

**Status:** `DISETUJUI`, `USULAN`, `DITUNDA`, `TERBUKA`.

> **Catatan FL-003 (15 Oktober 2026).** Pemilik project menetapkan stack melalui tugas FL-003. Keputusan berikut sekarang **berstatus DISETUJUI** sebagai keputusan arsitektur final tahap MVP: **D-010** (Go + `chi` sebagai backend utama, Python sebagai AI engine terpisah), **D-020** (SQLite mode WAL sebagai database MVP), **D-012** (YuNet + SFace sebagai model MVP), **D-021** (polling + `ETag` lebih dahulu), **D-022** (AES-256-GCM per baris untuk template biometrik), **D-023** (ESP32-S3 CAM sebagai pengunggah JPEG), dan **D-025** (menolak bobot model berlisensi non-permisif).
>
> Keputusan yang **tidak dipilih**: **D-019** (satu backend Python + FastAPI) berstatus **DITOLAK**; **D-011** (PostgreSQL sebagai database awal) berstatus **DITUNDA** sampai ada kebutuhan nyata.
>
> Persetujuan ini adalah **persetujuan arsitektur**, bukan izin implementasi. Penulisan kode backend, AI engine, aplikasi React Native, dan firmware tetap memerlukan tugas terpisah yang disetujui.
>
> Analisis, trade-off, dan risiko lengkap ada di `docs/ARCHITECTURE.md` §11–§14. Kontrak API ada di `docs/API_CONTRACT.md`; skema data ada di `docs/DATA_MODEL.md`.

> **Catatan FL-010U (16 Oktober 2026).** Pemilik project menetapkan **tiga role pengguna manusia (`admin`, `dosen`, `mahasiswa`) dengan Google Sign-In sebagai metode autentikasi utama**, dan memisahkan identitas mesin `device` dari role manusia. Keputusan ini **menggantikan D-013** (lihat D-031). Matriks otorisasi yang berlaku ada di `docs/AUTHORIZATION.md` v2.0 (53 endpoint). Rancangan autentikasi dan lifecycle akun ada pada **D-032/D-033** (**diputuskan untuk rancangan, menunggu persetujuan pemilik**), sedangkan **D-034** (cakupan dosen) dan **D-035** (pengikatan jaringan backend) masih menunggu keputusan. **Belum ada kode yang ditulis** untuk autentikasi, role, atau Google Sign-In.

| ID | Status | Keputusan dan alasan |
|---|---|---|
| D-001 | DISETUJUI | FaceLink AI adalah **MVP tugas kampus berskala kecil**, tetapi kode dibuat modular agar lebih mudah diadaptasi ke kantor/organisasi nanti. |
| D-002 | DISETUJUI | Software, pemrosesan AI, database, dan layanan lokal harus **Rp0**; total hardware **maksimal Rp1.000.000**. |
| D-003 | DISETUJUI | **Lokal terlebih dahulu**; Google Cloud Free Trial baru dipertimbangkan setelah MVP selesai, diuji, dan mendapat izin terpisah. |
| D-004 | DISETUJUI | Urutan fase: arsitektur/backend → mesin pengenalan wajah → UI React Native → integrasi software lewat webcam → hardware → validasi dan APK → cloud opsional. |
| D-005 | DISETUJUI | Aplikasi utama menggunakan **React Native**, Android harus dapat menghasilkan APK. Distribusi native iOS bukan kewajiban gratis. |
| D-006 | DISETUJUI | Fokus tim pengembangan ini adalah software. Tinjauan jurnal akademik ditangani rekan tim di jalur terpisah. |
| D-007 | DISETUJUI | GitHub menjadi pusat kode, PRD, arsitektur, keputusan dan status; seluruh dokumen harus **tidak bergantung pada satu model AI**. |
| D-008 | DISETUJUI | OmO direncanakan untuk AI coding, dengan `deepseek-v4.1-flash` sebagai model awal. Dukungan tool dan kuota tanpa biaya masih harus diuji. |
| D-009 | DISETUJUI | Hardware dibeli setelah software dapat diuji memakai webcam. Pengenalan wajah berkacamata adalah tujuan yang harus diverifikasi. |
| D-010 | DISETUJUI (FL-003) | Backend utama **Go + `chi`**; mesin CV lokal **Python + OpenCV** berjalan sebagai proses terpisah. Alasan: Go memegang autentikasi, otorisasi, absensi, sesi, orang, perangkat, rekap, audit, database, dan seluruh keputusan domain; Python hanya melakukan deteksi wajah, kualitas gambar, embedding, dan pencocokan. Ini menutup persaingan dengan D-019. Konsekuensi: dua bahasa, dua paket dependensi, satu kontrak internal yang harus dijaga. Kondisi peninjauan ulang: jika biaya integrasi dua proses terbukti melebihi manfaat, atau jika pemilik memutuskan sebaliknya. |
| D-011 | DITUNDA (FL-003) | PostgreSQL sebagai **database awal** ditunda. Skema dan migrasi tetap dirancang portabel supaya perpindahan nanti tidak memerlukan desain ulang. Kondisi peninjauan ulang: kebutuhan penulisan bersamaan dari banyak proses, kebutuhan replikasi, atau dataset uji yang membuat SQLite menjadi hambatan nyata. |
| D-012 | DISETUJUI (FL-003) | Model MVP: OpenCV Zoo **YuNet** (deteksi) + **SFace** (pengenalan). Wajib: catat versi model dan checksum di repositori, dan verifikasi lisensi berkas bobot yang benar-benar diunduh sebelum dipakai. Ambang kecocokan tetap harus dikalibrasi memakai data uji berizin dan **tidak boleh** disalin dari contoh mana pun. |
| D-013 | **DIGANTIKAN oleh D-031 (FL-010U, 16 Oktober 2026)** | Operator/admin sebagai satu-satunya pengguna aplikasi wajib di MVP; akun peserta opsional. Perangkat kamera memakai identitas terpisah. **Riwayat:** FL-010 (16 Oktober 2026) mengusulkan statusnya naik menjadi `DISETUJUI` sambil menegaskan tidak ada peran `dosen`/`mahasiswa`. Usulan itu **tidak pernah disetujui**; pemilik project menggantinya pada FL-010U dengan **tiga role manusia (`admin`, `dosen`, `mahasiswa`)** dan **akun mahasiswa**. Baris ini dipertahankan sebagai riwayat; yang berlaku adalah D-031 dan `docs/AUTHORIZATION.md` v2.0. |
| D-014 | USULAN | Kios kamera **satu orang per pemindaian**, bukan mengenali seluruh ruangan. Kandidat ESP32-S3 CAM. |
| D-015 | USULAN | Menyimpan template/embedding terenkripsi, tanpa menyimpan foto mentah secara default; menyediakan penghapusan dan absensi manual. |
| D-016 | DITUNDA | Cloud, sinkronisasi luar jaringan, multiorganisasi, integrasi HRIS, billing, CCTV terus-menerus, dan rilis native iOS berbayar. |
| D-017 | TERBUKA | Lisensi kode asli FaceLink AI serta pembagian kepemilikan karya antaranggota tim. Untuk sementara **tidak memilih `LICENSE`**. D-025 menutup risiko lisensi bobot model, tetapi tidak menutup risiko lisensi kode asli. |
| D-018 | TERBUKA | Alamat dan kondisi repository GitHub perlu diperiksa sebelum ada perintah commit/push. **Telah diverifikasi pada FL-001/FL-001R** (lihat D-024). |
| D-019 | DITOLAK (FL-003) | **Satu backend Python + FastAPI** sebagai pengganti Go. Alasan penolakan: kepemilikan database, keputusan domain absensi, dan API publik harus berada pada satu pemilik yang kuat; pemilik project juga menetapkan Go sebagai backend utama. Ini bukan penilaian atas kualitas FastAPI; FastAPI tetap boleh dipakai sebagai transport internal Python bila diperlukan. Dampak: semua teks yang menyebut FastAPI sebagai backend publik harus dianggap tidak berlaku. |
| D-020 | DISETUJUI (FL-003) | **SQLite mode WAL** sebagai database MVP, dengan pengaturan `foreign_keys=ON`, `busy_timeout`, transaksi pendek, dan prosedur pencadangan yang jelas. Alasan: satu proses penulis (Go), skala demo satu kelas, tanpa layanan database terpisah, dan tanpa biaya. Konsekuensi: penulisan bersamaan terbatas; skema dan migrasi harus tetap portabel ke PostgreSQL. Detail: `docs/DATA_MODEL.md` §8. |
| D-021 | DISETUJUI (FL-003) | **Polling interval dengan `ETag`/`Last-Modified`** sebagai jalur komunikasi pertama ke React Native. Alasan: sederhana, mudah diuji, tahan terhadap jaringan lokal yang tidak stabil, dan tidak memerlukan infrastruktur tambahan. WebSocket ditunda hingga ada kebutuhan pembaruan seketika yang terbukti, bukan diasumsikan. |
| D-022 | DISETUJUI (FL-003) | **Enkripsi tingkat aplikasi AES-256-GCM per baris** untuk template biometrik: nonce 12 byte dari sumber acak kriptografis **unik per baris**, AAD mengikat `person_id` + `model_id` + `model_version` + `key_version` (spesifikasi lengkap: `docs/DATA_MODEL.md` §4.12), verifikasi integritas sebelum dekripsi, dan penolakan bila dekripsi gagal. Bukan enkripsi seluruh berkas database. Kewajiban kunci: disimpan di luar database dan di luar Git. **Spesifikasi pengelolaan kunci (lokasi, rotasi, pemulihan) belum ditulis dan tetap Q-16**; jangan menganggapnya sudah ada. |
| D-023 | DISETUJUI (FL-003) | **Kontrak `CameraAdapter` berbasis JPEG + metadata** sebagai satu-satunya jalur masuk gambar; ESP32-S3 CAM kelak menjadi **klien pengunggah JPEG**, bukan host model AI. Identifikasi wajah tetap dijalankan pada sistem lokal. Rencana migrasi: `docs/ARCHITECTURE.md` §13. |
| D-024 | DISETUJUI (fakta) | Remote `origin` = `https://github.com/Ardian3D/FaceLink-Ai`. FL-001 di-merge ke `main` (`cd31ec0`) dan FL-002 di-merge ke `main` (`dee62b4`). Dicatat sebagai **fakta terverifikasi**, bukan keputusan desain. |
| D-025 | DISETUJUI (FL-003) | **Menghindari dependensi pada bobot model yang ketentuan penggunaannya tidak sesuai** dengan penggunaan project. Konsekuensi langsung: **InsightFace/ArcFace (`buffalo_*`) tidak dipakai** karena bobotnya berketentuan riset non-komersial, sedangkan YuNet + SFace memakai lisensi permisif dan wajib diverifikasi ulang pada berkas bobot yang benar-benar diunduh. |
| D-026 | DISETUJUI (FL-003) | **Migrasi skema memakai satu mekanisme SQL berversi** yang dijalankan dari sisi Go. **Alembic tidak dipakai sebagai migrator utama** karena backend bukan Python. Nama berkas, urutan versi, penanganan kegagalan, dan pencadangan sebelum migrasi berisiko ada di `docs/DATA_MODEL.md` §9. |
| D-027 | DISETUJUI (FL-003) | **Kepemilikan tunggal keputusan domain absensi dan penulisan database berada pada Go.** Python tidak boleh langsung menetapkan peserta sebagai hadir, dan tidak boleh menulis tabel kehadiran. Kontrak internal: `docs/API_CONTRACT.md` §6. |
| D-028 | DISETUJUI (FL-003) | **UI React Native bukan penentu kehadiran.** Backend Go adalah sumber kebenaran; aplikasi hanya menampilkan hasil dan mengirim permintaan manual melalui endpoint yang diaudit. Ketentuan per layar: `docs/UX_FLOWS.md` §10. **Catatan FL-010U (16 Oktober 2026):** rujukan `docs/UX_FLOWS.md` §10 **tidak dapat diverifikasi** — dokumen itu hanya memiliki §1–§8 dan tidak ada §10 pada riwayat berkas. Target yang dimaksud **tidak ditentukan dengan pasti** dan **tidak boleh ditebak**; masalah ini dicatat terbuka di `docs/AUTHORIZATION.md` §9 butir 8 dan menunggu pemilik project menyebut bagian yang dimaksud. |
| D-029 | TERBUKA | **Liveness detection bukan bagian dari YuNet/SFace.** Keduanya tidak boleh diklaim menyediakan perlindungan anti-spoofing lengkap. Mitigasi tahap MVP bersifat prosedural (operator hadir, satu orang per pemindaian, verifikasi manual) dan harus diuji sebagai eksperimen terpisah. |
| D-030 | DISETUJUI (FL-004S) | **Kebijakan pengelolaan kunci biometrik**: kunci AES-256-GCM disimpan di **luar database dan di luar Git**, dibungkus **Windows DPAPI cakupan pengguna**, berkas di `%LOCALAPPDATA%\FaceLink\keys\`, `key_version` pertama bernilai teks `'1'`, akses lewat satu antarmuka `KeyProvider` di Go, rotasi disiapkan tetapi belum diaktifkan, dan **tanpa kunci pemulihan otomatis**. **Kegagalan membaca template dipisahkan tiga kondisi** (kunci sementara tidak tersedia / kunci ada tetapi tidak dapat dipakai / kunci dipastikan tidak dapat dipulihkan) dan **template tidak boleh dihapus karena kegagalan sementara**. **Disetujui pemilik project 15 Oktober 2026 (FL-004S) dengan dua klarifikasi wajib:** (1) **kesiapan layanan biometrik dipisahkan dari kesiapan absensi manual**; kegagalan kunci **tidak** otomatis mematikan seluruh backend, tetapi layanan tetap **gagal secara aman** bila keamanan atau integritasnya tidak dapat dijamin (butir 10); (2) **penghapusan mengakui keterbatasan jaminan** SQLite WAL, cadangan, dan media penyimpanan, dengan retensi serta pembersihan terdokumentasi tanpa klaim tak teruji (butir 11). Menjawab Q-16. **Implementasi dan pengujian nyata `face_templates` tetap tertunda.** Spesifikasi lengkap: `docs/SECURITY_PRIVACY.md` §5a–§5f. |
| D-031 | **DISETUJUI (FL-010U)** | **Tiga role pengguna manusia (`admin`, `dosen`, `mahasiswa`) dengan Google Sign-In sebagai metode autentikasi utama, dan identitas mesin `device` yang terpisah.** Keputusan pemilik project 16 Oktober 2026: satu aplikasi React Native dengan tiga role; `dosen` menggantikan `operator`; **mahasiswa kini memiliki akun dan akses pribadi**; `device` **bukan** role pengguna manusia dan memakai kredensial perangkat tersendiri. Google **tidak** menentukan hak akses, tidak membuat akun admin, dan tidak memberikan role secara otomatis. **Menggantikan D-013.** Konsekuensi: matriks otorisasi menjadi 53 endpoint (`docs/AUTHORIZATION.md` v2.0), kontrak API dan model data berubah, dan kata sandi internal dihapus dari rancangan. Kondisi peninjauan ulang: kebutuhan satu orang memegang dua role, atau penambahan penyedia identitas kedua. |
| D-032 | **DIPUTUSKAN UNTUK RANCANGAN (FL-010U) — MENUNGGU PERSETUJUAN PEMILIK** | **Lifecycle akun: praregistrasi oleh `admin` (allowlist) dengan identitas Google sebagai kunci penautan.** Akun hanya dibuat `admin` dengan status `invited`; identitas Google tertaut pada login pertama memakai klaim **`sub`** (bukan email) setelah `email_verified` diverifikasi, lalu status menjadi `active`; `suspended` mencabut seluruh sesi. **Tidak ada pendaftaran mandiri, tidak ada akun `pending` otomatis, dan pengguna Google pertama tidak menjadi admin.** Bootstrap admin pertama dilakukan lokal, sekali, lewat `-bootstrap-admin`. Alternatif yang ditolak untuk MVP: auto-provision saat login pertama (menulis baris untuk orang asing, menambah antrean persetujuan pada jalur login, dan membuka pengambilalihan baris `invited` yang belum tertaut). |
| D-033 | **DIPUTUSKAN UNTUK RANCANGAN (FL-010U) — MENUNGGU PERSETUJUAN PEMILIK** | **Kebijakan sesi aplikasi:** token **opak** (bukan JWT) yang di server disimpan **hanya sebagai hash SHA-256**; token akses 60 menit; token penyegar 30 hari dengan **batas absolut sesi 30 hari**; rotasi pada setiap penyegaran dengan deteksi pemakaian ulang (seluruh sesi dicabut + audit); `admin` dapat mencabut sesi mana pun; penonaktifan akun mencabut seluruh sesi akun itu; **peran dibaca ulang dari database pada setiap permintaan** sehingga perubahan role berlaku tanpa menunggu token kedaluwarsa; token klien disimpan di penyimpanan aman perangkat. **Google ID token tidak pernah dipakai sebagai sesi aplikasi dan tidak pernah disimpan.** |
| D-034 | **USULAN (FL-010U)** | **Cakupan `dosen` dibatasi pada kelas yang ditugaskan** (`class_lecturers`): membaca peserta kelasnya, membuka/menutup sesi, koreksi manual, dan melihat rekap kelasnya. Rancangan mengikuti daftar tugas pemilik secara harfiah, sehingga **enrollment biometrik, pencatatan/pencabutan konsen, pengelolaan peserta, ekspor CSV, dan seluruh operasi perangkat berada pada `admin`**. **Menunggu keputusan pemilik:** bila alur demo menuntut dosen melakukan enrollment atau mencatat konsen di kelasnya, matriks `docs/AUTHORIZATION.md` §4 harus diperluas — perubahan izin, bukan endpoint baru. |
| D-035 | **TERBUKA (FL-010U)** | **Bagaimana aplikasi di HP menjangkau backend?** Kode FL-005 yang sudah di-merge menolak `FACELINK_ADDR` non-loopback (`backend/internal/config/config.go`), sedangkan aplikasi React Native berjalan di perangkat terpisah dari laptop. Pilihan yang tersedia: (a) tetap loopback dan memakai `adb reverse tcp:8080 tcp:8080` pada demo, (b) mengizinkan pengikatan ke alamat LAN privat dengan persetujuan eksplisit dan dokumentasi keamanan, atau (c) menempatkan proksi lokal berautentikasi. **Belum diputuskan**; FL-010U hanya mencatat temuan dan tidak mengubah kode. |

## Kebijakan pengelolaan kunci biometrik (D-030)

> **Status: DISETUJUI oleh pemilik project pada 15 Oktober 2026 (FL-004S).** Diajukan pada FL-004 dan dipertajam pada FL-004R untuk menjawab **Q-16**. Persetujuan diberikan **dengan dua klarifikasi wajib** dari pemilik project (butir 10 dan butir 11 di bawah); keduanya bagian dari keputusan dan tidak boleh diabaikan saat implementasi. **Disetujui bukan berarti diimplementasikan:** penulisan kode `face_templates`, penyimpanan kunci, dan pengujian nyata tetap memerlukan tugas implementasi tersendiri yang disetujui pemilik.

### Masalah yang harus dijawab

D-022 menetapkan template biometrik disimpan sebagai AES-256-GCM per baris di `face_templates`, dengan AAD mengikat `person_id` + `model_id` + `model_version` + `key_version`. Yang belum dijawab: **dari mana kunci itu datang, di mana ia disimpan, bagaimana ia dirotasi, dan apa yang terjadi bila ia hilang.** Tanpa jawaban itu, tabel biometrik tidak boleh ditulis.

Q-16 adalah pertanyaan **operasional**, bukan pertanyaan kriptografi. Algoritme, panjang nonce, dan AAD sudah final di D-022. Yang tersisa adalah siapa yang memegang kunci dan apa yang terjadi saat pemegangnya hilang.

### Isi keputusan yang berlaku

**1. Kunci tidak pernah berada di database, di Git, atau di lingkungan yang ikut ter-commit.** Ini melanjutkan `docs/DATA_MODEL.md` §11 butir 4 dan `docs/SECURITY_PRIVACY.md` §5. Larangannya tegas: tidak ada kunci di berkas `.env` yang ter-commit, tidak ada kunci di variabel lingkungan yang ditulis ke berkas konfigurasi, tidak ada kunci di dalam repositori, dan tidak ada kunci di dalam berkas `facelink.db`.

**2. Materi kunci dibungkus Windows DPAPI cakupan pengguna (CurrentUser).** Berkas kunci disimpan di `%LOCALAPPDATA%\FaceLink\keys\` dengan ACL yang dibatasi pada akun pengguna yang menjalankan backend. Berkas itu **tidak** memuat kunci mentah; ia memuat hasil `CryptProtectData`, sehingga menyalin berkasnya ke mesin lain tidak cukup untuk memakainya. Alasan memilih DPAPI: tidak ada biaya, tidak ada layanan tambahan, tidak ada frasa sandi yang harus diketik pada setiap penyalaan, dan Office/Chrome memakai pola perlindungan yang sama untuk kredensial lokal.

**3. Akses kunci disalurkan melalui satu antarmuka `KeyProvider` di Go.** Seluruh kode yang butuh kunci memanggil antarmuka ini, bukan membaca berkas kunci sendiri. Modul ini mengekspos operasi yang sempit: membaca kunci aktif, membaca kunci berdasarkan versi (untuk dekripsi baris lama setelah rotasi), dan memuat kunci baru. Tidak ada operasi "daftar semua kunci" dan tidak ada jalur untuk menuliskan kunci ke log. Dengan begitu, aturan "jangan mencetak kunci ke log" (`AGENTS.md` §3) menjadi properti struktural, bukan disiplin manual.

**4. `key_version` pertama bernilai teks `'1'`.** Kolom `key_version` bertipe `TEXT` (`docs/DATA_MODEL.md` §4.12), jadi versi ditulis sebagai `'1'`, bukan `1`. Nilai ini masuk ke AAD, sehingga baris yang ditulis dengan `'1'` **tidak** dapat didekripsi dengan kunci apa pun yang diberi label `'2'`. Format penamaan disiapkan untuk rotasi: kenaikan bilangan bulat sederhana, tanpa tanggal dan tanpa nama pengguna di dalam nilai.

**5. Dekripsi gagal berperilaku seperti yang sudah ditetapkan, tanpa tambahan.** Bila kunci hilang, rusak, atau AAD tidak cocok, backend menandai kegagalan, menulis audit, dan mengembalikan "tidak dapat diverifikasi" kepada operator. **Tidak ada** kunci pengganti otomatis, **tidak ada** pembuatan template baru secara diam-diam, dan **tidak ada** tebakan identitas. Ini hanya menegaskan kembali `docs/DATA_MODEL.md` §4.12, bukan aturan baru.

**5a. Kegagalan membaca template dipisahkan menjadi tiga kondisi, dan hanya kondisi ketiga yang boleh berakhir dengan penghapusan.** Perbedaan ini wajib, karena memperlakukan semua kegagalan sebagai kehilangan permanen akan menghapus template yang sebenarnya masih dapat dibaca. Rinciannya di `docs/SECURITY_PRIVACY.md` §5d.1:

| Kondisi | Ciri | Tindakan |
|---|---|---|
| **A. Kunci sementara tidak tersedia** | berkas kunci tidak ditemukan atau tidak dapat dibuka **saat ini** (profil belum termuat, berkas terkunci proses lain, media tidak terpasang, I/O gagal) | **jangan hapus apa pun**; tolak operasi yang butuh template, catat `audit_events`, coba lagi |
| **B. Kunci ada tetapi tidak dapat dipakai** | `auth_tag` tidak cocok, AAD tidak cocok, `key_version` tanpa kunci padanan, berkas kunci rusak/terpotong | karantina: tandai **tidak dapat didekripsi**, jangan ditampilkan, jangan dihitung cocok; periksa salinan kunci lain lebih dahulu |
| **C. Kunci dipastikan tidak dapat dipulihkan** | seluruh salinan sah sudah diperiksa, periode 30 hari terlampaui, tidak ada cara memulihkan | baru di sini penghapusan diizinkan |

**5b. Template tidak boleh dihapus karena kegagalan dekripsi sementara.** Penghapusan **dilarang** selama kondisi masih A atau B. Bila backend tidak dapat memastikan perbedaan ketiganya, tindakan yang benar adalah **berhenti dan melaporkan**, bukan menghapus. Penghapusan hanya dijalankan operator setelah kondisi C ditetapkan, tercatat pada `audit_events`, dan **tidak pernah** dipicu tugas terjadwal otomatis (`docs/SECURITY_PRIVACY.md` §5d.2).

**6. Rotasi dirancang tetapi belum diaktifkan.** Rotasi berarti: buat kunci versi baru, tulis template baru dengan versi itu, dekripsi baris lama memakai kunci lamanya, dan hapus kunci lama **hanya setelah** tidak ada baris yang menunjuk versinya. Prosedurnya disiapkan sekarang agar format berkas dan AAD siap, tetapi operasinya tidak dijalankan sampai ada permintaan nyata dan pemilik project menyetujuinya. Alasan: MVP hanya menyimpan template satu kelas dalam satu sesi demo; rotasi yang tidak diminta hanya menambah jalur kegagalan baru.

**7. Tidak ada kunci pemulihan cadangan otomatis.** Kesepakatan ini disengaja dan harus dipahami konsekuensinya: **pemulihan bukan operasi yang menjamin template kembali.** Yang dapat dipulihkan adalah keberadaan sistem, bukan data biometriknya. Untuk memastikan kapan pemulihan masih memungkinkan, kebijakan ini menetapkan **periode pemulihan 30 hari** yang dihitung dari tanggal pemulihan aktif terakhir yang tercatat pada `audit_events`; di luar periode itu, template dianggap tidak dapat dipulihkan dan peserta harus mendaftar ulang.

**Angka 30 hari itu bukan jaminan pemulihan kunci.** Ia adalah batas waktu **upaya** operator sebelum template dinyatakan hilang. Selama periode itu tidak ada jaminan teknis apa pun bahwa template dapat dibaca kembali: bila kunci hilang atau rusak secara permanen, menunggu sampai hari ke-29 tidak mengembalikan apa pun. Yang benar-benar menentukan adalah apakah masih ada salinan kunci sah yang dapat dipakai — bukan berapa hari telah berlalu. Karena itu periode ini **tidak boleh** disajikan kepada pemilik project atau peserta sebagai jaminan bahwa data dapat dipulihkan.

**8. Kehilangan kunci adalah kehilangan template, dan itu berakhir dengan enrollment ulang ber-consent.** Bila kunci tidak dapat dipulihkan (berkas hilang, profil Windows direset, perangkat diganti tanpa salinan terlindungi, atau periode 30 hari terlampaui), template lama dihapus, peristiwa dicatat di `audit_events`, dan peserta **diminta ulang** persetujuannya sebelum melakukan enrollment baru. Kehadiran selama template tidak tersedia ditangani melalui jalur absensi manual, bukan melalui pembuatan template tironan.

**8a. Insiden kunci memiliki prosedur tersendiri.** Langkahnya diurutkan pada `docs/SECURITY_PRIVACY.md` §5d.3: kenali dan hentikan, tentukan tingkatnya, amankan sebelum menyelidiki, periksa salinan kunci sah, nyatakan hasilnya apa adanya, putuskan pemulihan/rotasi/penghapusan, tetapkan jangkauan, pulihkan layanan tanpa biometrik lebih dahulu, lalu tinjau. Selama insiden **dilarang** membangkitkan kunci baru untuk "memperbaiki" keadaan, memindahkan kunci lewat Git/surel/penyimpanan awan, dan menyalin materi kunci ke catatan insiden.

**8b. Absensi manual adalah jalur tetap, bukan keadaan darurat sementara.** Ia harus sudah berfungsi sebelum template apa pun ditulis, dan kehadiran melalui jalur itu **berlaku sama sahnya** seperti kehadiran otomatis. Ketiadaan template tidak boleh menjadi alasan menolak kehadiran, menandai peserta bermasalah, atau menahan hak apa pun (`docs/API_CONTRACT.md` §5.8, D-028). Bila jalur manual tidak dapat dipakai pada suatu sesi, sesi itu **tidak boleh** dilanjutkan dengan klaim otomatis yang tidak berdasar.

**9. Tidak ada penyalinan kunci antar mesin.** Batas cakupan pengguna membuat kunci terikat pada akun Windows **ketika profilnya tidak roaming**; DPAPI sendiri **tidak** mengikat blob ke mesin secara mutlak, dan pemilik profil roaming dapat membukanya dari komputer lain di jaringan. Karena itu perpindahan kunci antar mesin hanya melalui berkas terlindungi yang disalin sadar oleh operator, dan **tidak boleh** melalui Git, surel, penyimpanan awan, atau lampiran pesan. Bila kunci harus dipindahkan, itu operasi tercatat, bukan hal yang terjadi sendiri.

**10. (Klarifikasi wajib pemilik, FL-004S) Kesiapan layanan biometrik dipisahkan dari kesiapan layanan absensi manual.** Kegagalan `KeyProvider` atau kunci biometrik **tidak** boleh otomatis mematikan seluruh backend. Yang dipisahkan adalah **kesiapan layanan**, bukan kepemilikan keputusan domain: D-027 tetap berlaku dan absensi manual tetap ditulis oleh Go (`docs/API_CONTRACT.md` §5.8, §6). Ketentuannya di `docs/SECURITY_PRIVACY.md` §5e:

1. **Dua status kesiapan yang terpisah dan dapat dilaporkan sendiri-sendiri:** kesiapan **layanan absensi manual** dan kesiapan **layanan biometrik**. Kegagalan pada yang kedua **tidak boleh** menurunkan status yang pertama.
2. **Yang tetap berjalan saat kunci hilang atau `KeyProvider` gagal:** absensi manual, koreksi kehadiran manual, rekap kehadiran, kelola peserta dan kelas, autentikasi operator, serta pencatatan `audit_events`. Perubahan izin administratif yang tidak menyentuh biometrik **tidak** ikut berhenti.
3. **Yang berhenti dan menolak bekerja dengan pesan jelas:** enrollment biometrik, pembuatan template baru, pencocokan otomatis, dan **setiap penulisan** ke `face_templates`. Endpoint biometrik mengembalikan hasil "tidak dapat diverifikasi", **bukan** kesalahan server umum.
4. **Tetap gagal secara aman.** Pemisahan **bukan** pembenaran untuk melanjutkan operasi yang keamanan atau integritasnya tidak dapat dijamin. Backend **wajib** berhenti keras bila hal berikut tidak dapat dijamin: integritas dan keaslian database serta migrasinya; autentikasi operator; konsistensi kunci dengan AAD dan `key_version`, termasuk ketidakpastian apakah kunci benar-benar sesuai (kondisi C) — dalam ketidakpastian itu perilakunya **berhenti dan melaporkan**, bukan menghapus; keutuhan riwayat audit dan kehadiran; serta batas input dan otorisasi pada endpoint manual.
5. **Pemulihan layanan berurutan.** Setelah gangguan, absensi manual dipulihkan lebih dahulu (§5d.3 langkah 8), baru layanan biometrik.

**11. (Klarifikasi wajib pemilik, FL-004S) Kebijakan penghapusan mengakui keterbatasan jaminan penghapusan.** Tidak ada klaim bahwa data biometrik pasti hilang dari media penyimpanan. Retensi, pembersihan, dan batas jaminannya ada di `docs/SECURITY_PRIVACY.md` §5f, serta pada butir 5 dan 5a di bawah:

1. **Retensi dan pembersihan dicatat sebagai prosedur, bukan diandaikan selesai.** Setiap tahap yang menjanjikan penghapusan harus menyebut berkas yang disentuh dan cara memverifikasinya.
2. **Tiga lapisan yang tidak boleh disembunyikan:** berkas `facelink.db` beserta `-wal` dan `-shm`, salinan cadangan, dan cache lapisan kamera/antarmuka.
3. **Yang dapat diklaim:** baris pada tabel, berkas database aktif, WAL dan SHM yang dipadatkan lalu ditimpa, serta salinan cadangan yang dihapus. **Yang tidak dapat diklaim:** bahwa isi tabel lenyap dari media penyimpanan secara fisik (SSD dan media flash memakai pemetaan ulang blok, TRIM, dan daur ulang blok di luar kendali aplikasi), dan bahwa datanya lenyap dari cadangan, snapshot, atau platform sinkronisasi yang belum diperiksa.
4. **Jawaban MVP bersifat prosedural, bukan kriptografis:** kenali setiap lokasi salinan, batasi lokasi penyimpanan, hapus seluruh salinan yang dikenal pada setiap tahap, dan **catat lokasi yang belum terjangkau sebagai keterbatasan yang belum diselesaikan** — jangan dinyatakan bersih.
5. **Kehadiran manual tetap sah dan operasional** meskipun keterbatasan ini ada; keterbatasan penghapusan bukan alasan menolak pencabutan persetujuan.

### Batasan yang harus dinyatakan jujur

**DPAPI bukan kotak ajaib, dan dokumen ini tidak boleh menggambarkannya begitu.** Empat batas berikut dibuktikan oleh sumber Microsoft dan harus dibaca sebelum menganggap kunci aman:

| Batas | Konsekuensi nyata |
|---|---|
| Setiap aplikasi yang berjalan dengan kredensial pengguna yang sama dapat membuka blob DPAPI | DPAPI melindungi dari **pengguna lain di mesin yang sama** dan dari **berkas yang dicuri lalu dibaca di mesin lain**. Ia **tidak** melindungi dari malware yang berjalan sebagai pengguna yang sama, dan tidak melindungi dari operator yang menyalahgunakan mesinnya sendiri |
| Profil **roaming** dapat didekripsi dari komputer lain di jaringan | Jika akun Windows di domain memakai profil roaming, template dapat didekripsi di mesin lain. Asumsi ini harus **diverifikasi**, bukan diandaikan |
| DPAPI menyimpan materi kunci di dalam profil pengguna | Bila profil tidak termuat, dekripsi **gagal**. Reset profil, penghapusan akun, atau profil yang rusak menghilangkan kemampuan membaca template lama |
| DPAPI memakai mode pengguna, bukan mode mesin | Ini disengaja. Mode mesin (`CRYPTPROTECT_LOCAL_MACHINE`) akan membuka kunci bagi **semua** pengguna mesin itu, sehingga **ditolak** untuk project ini |

Kesimpulan yang harus diucapkan apa adanya kepada pemilik project: **AES-256-GCM melindungi isi database bila berkasnya bocor; DPAPI melindungi kunci dari pengguna lain dan dari pencurian berkas. Tidak ada lapisan di sini yang melindungi data biometrik dari akun Windows yang sudah dikuasai penyerang.** MVP tidak memiliki jawaban untuk itu.

### Alasan menolak alternatif

| Alternatif | Alasan tidak dipilih |
|---|---|
| Kunci di berkas konfigurasi atau `.env` ter-commit | Dilarang tegas oleh `docs/DATA_MODEL.md` §11 butir 4 dan `docs/SECURITY_PRIVACY.md` §5; kunci akan ikut bocor bersama repositori |
| Kunci diturunkan dari kata sandi operator | Menambah biaya operasional (wajib diketik saat penyalaan), menciptakan ketergantungan pada kebijakan kata sandi, dan memperkenalkan jalur pemulihan ilegal saat kata sandi dilupakan |
| Penyimpanan rahasia sistem (Windows Credential Manager) | Menjadi **alternatif yang dipertimbangkan**, bukan pendamping. Ia menyimpan rahasia per-entri dengan perlindungan serupa DPAPI, tetapi membatasi ukuran dan format nilai serta menyulitkan penyimpanan lebih dari satu versi kunci sekaligus, padahal rotasi membutuhkan itu |
| Kunci di variabel lingkungan | Sering tersalin ke berkas konfigurasi, terbaca oleh proses anak, dan muncul pada pesan kesalahan. Tidak memenuhi syarat "berkas key yang tidak masuk Git" |
| Layanan manajemen kunci awan | Melanggar batas Rp0 dan batas lokal MVP (D-002, D-003) |

### Konsekuensi yang diterima

1. **Tidak ada jaminan pemulihan.** Bila kunci hilang dan periode pemulihan berakhir, semua template hilang dan peserta harus mendaftar ulang. Ini konsekuensi sadar, bukan kelalaian. **Periode 30 hari bukan jaminan bahwa template dapat dipulihkan selama masa itu** — ia hanya batas waktu upaya operator.
2. **Satu mesin, satu akun.** MVP berjalan pada satu laptop Windows dengan satu akun operator. Perpindahan mesin adalah operasi manual yang tercatat.
3. **`synchronous=NORMAL` pada SQLite tetap berlaku** (`docs/DATA_MODEL.md` §13 butir 2). Kunci yang tersimpan aman dari kegagalan proses, tetapi kombinasi kunci hilang + transaksi terakhir hilang tetap mungkin terjadi pada mati listrik. Ini harus dinyatakan, bukan disembunyikan.
4. **Backup database tidak memulihkan template tanpa kunci.** Menyalin `facelink.db` beserta `-wal` dan `-shm` tanpa memindahkan berkas kunci terlindungi menghasilkan database yang isinya **tidak dapat dibaca**. Prosedur cadangan di `docs/DATA_MODEL.md` §7.3 harus diperlakukan sebagai **satu kesatuan dengan kunci**, bukan hanya tiga berkas database.
5. **Aturan penghapusan harus menjangkau lebih dari tabel `face_templates`.** Penghapusan biometrik yang benar mencakup **berkas `-wal` dan `-shm`**, **salinan cadangan**, dan **seluruh lapisan cache** — termasuk berkas yang dibuat lapisan kamera/antarmuka bila ada. Menghapus baris pada tabel induk sementara data biometrik masih tertinggal di WAL, cadangan, atau cache adalah penghapusan yang **belum selesai**, dan itu harus diuji, bukan diandaikan (T-25, T-27).
5a. **(Klarifikasi wajib pemilik, FL-004S) Keterbatasan jaminan penghapusan diakui, bukan ditutupi.** Urutan pembersihan yang ditetapkan: hapus baris pada `face_templates`, jalankan checkpoint WAL dengan pemotongan, lalu `VACUUM` untuk memadatkan dan menimpa halaman bebas, baru katakan "sudah dihapus" — dan katakan **sebatas** itu. Yang **tidak** diklaim: bahwa isi media penyimpanan bersih secara fisik. SSD dan media flash memakai pemetaan ulang blok, TRIM, dan daur ulang blok di luar kendali aplikasi, sehingga salinan lama dapat tetap berada di blok yang tidak terjangkau berkas. Cadangan, snapshot, atau platform sinkronisasi yang belum diperiksa **tidak** boleh dihitung sudah bersih. Prosedur lengkap dan cara verifikasinya: `docs/SECURITY_PRIVACY.md` §5f. **Jangan** menulis klaim "terhapus permanen", "tidak dapat dipulihkan", atau "aman secara forensik" tanpa pengukuran yang benar-benar dijalankan.
6. **(Klarifikasi wajib pemilik, FL-004S) Kegagalan layanan biometrik bukan kegagalan seluruh backend.** Pemisahan kesiapan layanan (butir 10) berarti satu layanan dapat hidup sementara yang lain mati. Konsekuensi yang diterima: pada masa gangguan kunci, absensi tetap berjalan melalui jalur manual, dan itu **bukan** penurunan mutu sementara yang ditoleransi melainkan jalur yang memang sah (D-028, `docs/API_CONTRACT.md` §5.8). Bila absensi manual pun tidak dapat berjalan, kelas kehilangan jalur pencatatan kehadiran — inilah alasan §5d.5 menetapkan absensi manual **harus sudah berfungsi sebelum template apa pun ditulis**.

### Kondisi peninjauan ulang

Usulan ini harus ditinjau ulang bila salah satu terjadi: akun Windows di domain dengan profil roaming; kebutuhan memindahkan backend ke mesin lain; kebutuhan rotasi kunci yang nyata; atau keputusan pindah ke PostgreSQL (D-011). Pada dua kondisi pertama, DPAPI cakupan pengguna **tidak** memenuhi kebutuhan dan rancangan harus diganti, bukan ditambal.

**Catatan persetujuan:** pemilik project menyetujui rancangan ini pada 15 Oktober 2026 (**FL-004S**) dengan **dua klarifikasi wajib** yang kini menjadi butir 10 dan butir 11 keputusan ini. Status D-030 karena itu **DISETUJUI**, dan Q-16 **DIPUTUSKAN untuk rancangan MVP**. Persetujuan ini **bukan** izin implementasi: penulisan kode `face_templates`, penyimpanan kunci, dan seluruh pengujian nyata (T-21–T-38) tetap tertunda sampai ada tugas implementasi tersendiri yang disetujui pemilik.

## Dampak FL-003 terhadap keputusan lama

| Keputusan lama | Status baru | Dampak yang harus dipahami |
|---|---|---|
| D-010 (Go + Python) | DISETUJUI | Tidak ada perubahan teks keputusan; statusnya naik dari USULAN. |
| D-011 (PostgreSQL awal) | DITUNDA | Tidak dihapus. Rancangan skema tetap portabel; tidak ada janji bahwa migrasi SQLite→PostgreSQL berjalan identik tanpa penyesuaian. |
| D-019 (Python-only) | DITOLAK | Rekomendasi FL-002 dibatalkan oleh keputusan pemilik. Semua rujukan FastAPI sebagai backend publik dihapus dari dokumen terkait. |
| D-020 (SQLite WAL) | DISETUJUI | Sama seperti D-011: keputusan database awal kini final. |
| D-012, D-021, D-022, D-023, D-025 | DISETUJUI | Masing-masing menjadi basis kontrak API, model data, dan keamanan. |
| D-024 | Diperluas | Ditambah fakta merge FL-002 (`dee62b4`). |

## Protokol perubahan keputusan

1. Jelaskan masalah dan pilihan yang tersedia.
2. Cantumkan biaya, risiko, dampak ke API/data/privasi, dan pengaruh terhadap roadmap.
3. Minta persetujuan pemilik project.
4. Perbarui status keputusan beserta tanggal dan alasan jika disetujui.
5. Perbarui `PROJECT_STATE.md` serta spesifikasi lain yang terpengaruh.

**Jangan diam-diam mengganti stack, menetapkan layanan berbayar, atau menambah fitur di luar lingkup.**

## Catatan FL-010 (16 Oktober 2026) — matriks peran dan otorisasi

FL-010 memfinalkan rancangan peran dan izin MVP pada `docs/AUTHORIZATION.md`. Dokumen itu **tidak** mengubah status keputusan apa pun. Tiga hal yang menuntut persetujuan pemilik sebelum berlaku (rinciannya di `docs/AUTHORIZATION.md` §9):

1. **D-013** diusulkan naik dari `USULAN` menjadi `DISETUJUI`, dengan penegasan bahwa tidak ada peran `dosen`/`mahasiswa` di MVP. Statusnya **belum** diubah.
2. Dua aksi audit baru diusulkan (`authz.denied`, `auth.device.mismatch`) untuk menambahkan daftar minimum `docs/API_CONTRACT.md` §5.13. Daftar audit minimum adalah bagian kontrak, sehingga perubahan itu tidak dilakukan sendiri.
3. Larangan operator menonaktifkan akunnya sendiri diusulkan; belum diatur di kontrak.

**D-014 dan D-015 tetap `USULAN`** dan tidak disentuh FL-010: keduanya soal kios dan penyimpanan template, bukan soal peran/otorisasi.

## Catatan FL-010U (16 Oktober 2026) — Google Sign-In dan tiga role

FL-010U mengubah **arsitektur autentikasi, peran, dan otorisasi** atas keputusan pemilik project. Dua perintah kerja yang dijalankan: tiga role pengguna manusia (`admin`, `dosen`, `mahasiswa`) dengan **Google Sign-In** sebagai metode masuk, dan pemisahan tegas identitas mesin `device` dari role manusia.

**Keputusan yang lahir dari tugas ini:** D-031 (**DISETUJUI** pemilik), D-032 dan D-033 (**diputuskan untuk rancangan**, menunggu persetujuan), D-034 dan D-035 (**usulan/terbuka**).

**Yang digantikan:** D-013 → **DIGANTIKAN oleh D-031**. Keputusan lama "tidak ada akun mahasiswa dan tidak ada peran `participant` pada MVP" tidak berlaku lagi; mahasiswa kini memiliki akun dengan akses baca-sendiri. Matriks 43 endpoint FL-010/FL-010R digantikan matriks **53 endpoint** di `docs/AUTHORIZATION.md` v2.0.

**Yang tidak diubah:** D-010, D-011, D-012, D-020, D-021, D-022, D-023, D-025, D-026, D-027, D-028, dan D-030 berlaku apa adanya. Khususnya: kebijakan kunci biometrik, pemisahan kesiapan layanan (§5e), dan keterbatasan jaminan penghapusan (§5f) **tidak disentuh** FL-010U, dan `face_templates` tetap belum diimplementasikan.

**Temuan yang harus diselesaikan lebih dahulu:** berkas kode `backend/internal/config/config.go` menolak alamat non-loopback, sehingga HP belum dapat menjangkau backend lewat Wi-Fi kelas. Tercatat sebagai D-035 dan `docs/AUTHORIZATION.md` §9.

**Belum ada implementasi.** Tidak ada kode Go, SQL, migrasi, atau TypeScript yang ditulis pada FL-010U; seluruh skenario uji autentikasi dan otorisasi berstatus **BELUM TERVERIFIKASI**.

# Matriks Peran dan Otorisasi FaceLink AI (MVP)

**Versi:** 2.0 · **Status:** **ARSITEKTUR DISETUJUI PEMILIK PROJECT PADA FL-010U (16 Oktober 2026); BELUM DIIMPLEMENTASIKAN.** Dokumen ini menggantikan rancangan peran FL-010/FL-010R. Butir yang masih menuntut keputusan pemilik ada di §9.

**Lingkup:** demo **satu kelas, satu terminal** (`docs/PRD.md` §4). Tidak ada multi-tenant, multi-organisasi, SSO perusahaan, billing, atau HRIS. Semua isi dokumen ini berlaku untuk MVP lokal dan **tidak** boleh diperluas diam-diam.

**Tanggal:** 16 Oktober 2026 · **Tugas:** FL-010U · **Cabang:** `docs/fl-010u-google-three-roles` (dari `origin/main` = `804c2f7`)

**Rujukan yang mengikat:** `docs/PRD.md` §3 dan §7–§8 · `docs/API_CONTRACT.md` §1–§5 · `docs/DATA_MODEL.md` §4 · `docs/SECURITY_PRIVACY.md` §6 dan §5e · `docs/DECISIONS.md` D-031–D-034 · `docs/TEST_PLAN.md` §2 dan §7–§8 · `docs/UX_FLOWS.md` §2.

> **Kode autentikasi dan otorisasi belum ditulis.** Dokumen ini adalah rancangan yang dapat diuji, bukan laporan implementasi. Seluruh skenario pada §8 berstatus **BELUM TERVERIFIKASI**.

---

## 1. Prinsip otorisasi yang berlaku

1. **Deny by default.** Endpoint hanya dapat diakses bila peran pemanggilnya tercantum eksplisit pada matriks §4. Tidak ada endpoint yang terbuka bagi "semua yang sudah login" kecuali dinyatakan.
2. **Least privilege.** Setiap peran hanya mendapat hak yang dibutuhkannya untuk demo satu kelas. `dosen` tidak mengelola akun, role, atau perangkat; `mahasiswa` tidak mengubah data apa pun; `device` hanya boleh mengirim satu jenis permintaan.
3. **Tiga jalur autentikasi yang terpisah dan tidak dapat saling menggantikan.** Sesi pengguna (Google Sign-In), dan token perangkat adalah dua hal berbeda (§3). Token perangkat **tidak boleh** dipakai pada endpoint manusia, dan sebaliknya (`docs/SECURITY_PRIVACY.md` §6).
4. **Otorisasi diperiksa di backend Go, tidak pernah di klien.** UI React Native hanya menyembunyikan tombol; keputusan izin, kepemilikan, keanggotaan, dan penugasan kelas selalu diambil ulang di backend (D-028, `docs/API_CONTRACT.md` §1.1 butir 3).
5. **Google hanya penyedia identitas.** Google memverifikasi siapa pengguna, tetapi **tidak** menentukan hak akses, tidak membuat akun, dan tidak memberikan role. Role hanya ditetapkan `admin` di backend FaceLink AI.
6. **Peran dibaca ulang dari database pada setiap permintaan.** Token sesi tidak memuat klaim peran, sehingga perubahan role dan penonaktifan akun berlaku pada permintaan berikutnya — bukan menunggu token kedaluwarsa.
7. **Tidak membocorkan keberadaan sumber daya yang tidak boleh dilihat.** Sumber daya di luar cakupan pemanggil mengembalikan `404 NOT_FOUND`, bukan `403` (`docs/API_CONTRACT.md` §1.5).
8. **Setiap penolakan yang bermakna keamanan dicatat.** Penolakan pada jalur administratif, autentikasi, dan absensi manual **wajib** meninggalkan `audit_events` (§6.2).
9. **Kepemilikan dan cakupan diperiksa, bukan hanya peran.** Peran yang sah pada objek yang bukan miliknya, bukan kelasnya, atau bukan dirinya tetap ditolak (§5).

---

## 2. Peran final yang berlaku di MVP

### 2.1 Tiga role pengguna manusia

| Peran | Siapa | Dashboard dan kewajiban di MVP | Bukti referensi |
|---|---|---|---|
| `admin` | Administrator sistem | Mengelola akun dan role, kelas dan penugasan dosen, peserta, persetujuan biometrik, enrollment, perangkat IoT, laporan dan ekspor, serta meninjau audit | Keputusan pemilik FL-010U §4 |
| `dosen` | Pengajar yang ditugaskan pada satu atau lebih kelas | Dashboard kelasnya sendiri: melihat mahasiswa kelas yang ditugaskan, membuka/menutup sesi absensi, menjalankan koreksi manual, melihat status dan rekap kelasnya | Keputusan pemilik FL-010U §4 |
| `mahasiswa` | Peserta yang memiliki akun dan akses pribadi | Dashboard pribadi: profil, jadwal sesi kelas yang diikuti, status dan riwayat kehadiran sendiri, status persetujuan sendiri | Keputusan pemilik FL-010U §4 |

Ketiganya masuk melalui **Google Sign-In** dan memakai **satu aplikasi React Native yang sama** dengan navigasi berbeda menurut role dari backend.

### 2.2 Aktor mesin dan proses internal (bukan role pengguna manusia)

| Aktor | Apakah punya akun pengguna? | Bagaimana haknya ditentukan |
|---|---|---|
| `device` | **Tidak.** Bukan baris `users` dan bukan role manusia. | Token perangkat pada tabel `devices`; terikat satu `device_id`; hanya boleh memanggil `POST /devices/{device_id}/recognition-attempts` |
| `ai-engine` (Python) | Tidak | Bukan peran API publik. Hanya endpoint internal loopback `docs/API_CONTRACT.md` §6; tidak memegang kredensial database (D-027) |
| OmO / agent AI | Tidak | Alat pengembangan, bukan komponen runtime FaceLink AI (`AGENTS.md` §6) |

### 2.3 Perbandingan peran lama dan baru

| Aspek | FL-010/FL-010R (lama) | FL-010U (berlaku sekarang) |
|---|---|---|
| Role pengguna manusia | `operator`, `admin` | `admin`, `dosen`, `mahasiswa` |
| Mahasiswa | Tidak punya akun; hanya baris data `people` | **Punya akun, dapat masuk aplikasi, hanya melihat data sendiri** |
| Metode login | Email + kata sandi internal | **Google Sign-In (OpenID Connect)** |
| Perangkat | Diperlakukan sebagai salah satu "tiga peran" | **Dipisahkan tegas** dari role manusia; identitas mesin tersendiri |
| Jumlah endpoint pada matriks | 43 | **53** |
| Cakupan baca peserta | `operator` dan `admin` (satu kelas) | `admin` (semua) dan `dosen` (hanya kelas yang ditugaskan) |
| Konsen, enrollment, peserta, perangkat | `operator`/`admin` | **`admin` saja** |
| Audit, ekspor, akun, role | `operator` dapat membaca audit | **`admin` saja** |

**Penggantian `operator` → `dosen` bukan penggantian nama dengan cakupan identik.** Cakupan `operator` lama terbagi: kemampuan operasional kelas menjadi `dosen`, sedangkan pengelolaan akun, peserta, konsen, enrollment, perangkat, laporan, dan audit menjadi `admin`. Perbedaan ini dinyatakan terbuka pada §9 butir 1 dan merupakan butir keputusan yang diminta dari pemilik.

### 2.4 Supersesi keputusan lama

- Keputusan **D-013** ("operator/admin sebagai satu-satunya pengguna aplikasi wajib di MVP; akun peserta opsional") **DIGANTIKAN** oleh keputusan pemilik pada FL-010U. Riwayatnya dipertahankan di `docs/DECISIONS.md`.
- Keputusan lama **"tidak ada akun mahasiswa dan tidak ada peran `participant` pada MVP"** **DIGANTIKAN**: mahasiswa kini memiliki akun dengan akses baca-sendiri. Statusnya di `docs/DECISIONS.md` D-031.
- Rancangan matriks 43 endpoint FL-010/FL-010R **digantikan** matriks 53 endpoint pada §4.

---

## 3. Jalur autentikasi yang dibedakan tegas

### 3.1 Sesi pengguna (`admin`, `dosen`, `mahasiswa`)

| Aspek | Ketetapan MVP |
|---|---|
| Bukti identitas | **Google ID token** (JWT) yang dikirim aplikasi ke `POST /api/v1/auth/google` |
| Verifikasi | Backend memverifikasi tanda tangan (JWKS), `alg` RS256, `iss`, `aud`, `exp`/`iat`, `nonce`, dan `email_verified` sebelum menerima apa pun |
| Pengenal identitas | Klaim **`sub`** Google, disimpan pada `user_identities` sebagai kunci stabil. **Email bukan kunci identitas.** |
| Akun internal | Hanya dibuat `admin` (praregistrasi). Identitas Google tertaut pada login pertama, hanya bila email sama dan sudah diverifikasi Google |
| Bentuk sesi | Token **opak** acak 256-bit: token akses (60 menit) + token penyegar (30 hari, batas absolut 30 hari), disimpan server **hanya sebagai hash SHA-256** |
| Penyimpanan klien | Penyimpanan aman perangkat; **dilarang** ditulis ke log, Git, atau berkas konfigurasi |
| Pencabutan | Logout mencabut sesi itu; `admin` dapat mencabut sesi mana pun; penonaktifan akun mencabut seluruh sesi akun itu |
| Rotasi dan penyalahgunaan | Setiap penyegaran menerbitkan token baru; pemakaian ulang penyegar lama → **seluruh sesi dicabut** + audit `auth.session.reuse_detected` |
| Aturan keras | Google ID token **tidak pernah** dipakai sebagai sesi aplikasi dan **tidak pernah** disimpan |

**Bootstrap admin pertama** dilakukan **lokal, sekali, oleh operator mesin**: perintah `-bootstrap-admin` pada `cmd/server` menulis satu baris `users` (`role='admin'`, `status='invited'`) untuk email Google yang disebutkan, menolak berjalan bila sudah ada akun `admin` aktif atau `invited`, tidak menerima kata sandi, dan diaudit `user.bootstrapped`. **Pengguna Google pertama tidak pernah otomatis menjadi admin.**

### 3.2 Token perangkat (`device`)

| Aspek | Ketetapan MVP |
|---|---|
| Pembuatan | Hanya `admin` lewat `POST /devices`; token ditampilkan **sekali**, disimpan sebagai `token_hash` |
| Pengikatan | Token **terikat satu `device_id`**. `device_id` pada jalur tidak cocok dengan token → `403` |
| Cakupan | **Hanya** `POST /api/v1/devices/{device_id}/recognition-attempts` |
| Rotasi dan pencabutan | `POST /devices/{device_id}/rotate-token` dan `DELETE /devices/{device_id}` — keduanya **hanya `admin`** |
| Aturan keras | `token_hash` **tidak pernah** dikembalikan endpoint baca; perangkat **tidak boleh** memanggil endpoint manusia mana pun (`docs/SECURITY_PRIVACY.md` §6) |

### 3.3 Mesin AI internal

Bukan peran API publik. Hanya loopback `127.0.0.1`/IPC dengan token internal (`docs/API_CONTRACT.md` §6). Tidak memegang kredensial database dan tidak menetapkan kehadiran (D-027).

### 3.4 Matriks "jalur autentikasi × endpoint"

| Kategori endpoint | Sesi pengguna | Token perangkat | Tanpa token |
|---|---|---|---|
| `/healthz` | boleh | boleh (diabaikan) | **boleh** (probe lokal) |
| `/auth/google` | — | — | **boleh**, dengan batas laju ketat |
| Seluruh endpoint manusia lainnya (§4) | **harus**, dengan role yang tercantum | **ditolak** (`403 FORBIDDEN_ROLE`) | ditolak (`401`) |
| `/devices/{device_id}/recognition-attempts` | **ditolak** (`403`) | **harus** (`device_id` cocok) | ditolak (`401`) |
| Endpoint internal AI Engine | bukan peran publik | bukan peran publik | hanya dari `127.0.0.1` |

---

## 4. Matriks otorisasi `endpoint × peran`

Diturunkan langsung dari `docs/API_CONTRACT.md` §4. **Total 53 baris endpoint.** Kolom "Peran yang diizinkan" adalah **satu-satunya** sumber kebenaran otorisasi; endpoint yang tidak tercantum **tidak boleh** dibuat.

Dari 53 endpoint, **2** dapat dipanggil tanpa token (`POST /auth/google`, `GET /healthz`) dan **51** menuntut kredensial yang sah; jumlah 51 adalah soal autentikasi, bukan soal peran.

Istilah: `publik` = boleh tanpa token · `device` = token perangkat, bukan role manusia · `admin`, `dosen`, `mahasiswa` = tiga role pengguna manusia.

### 4.1 Modul 1 — Autentikasi, sesi, dan profil

| Metode | Jalur | Peran | Kewajiban tambahan |
|---|---|---|---|
| `POST` | `/auth/google` | publik | Batas laju ketat per alamat dan per identitas; audit tiap percobaan berhasil/gagal |
| `POST` | `/auth/refresh` | admin, dosen, mahasiswa | Token penyegar lama tidak dapat dipakai dua kali; rotasi + deteksi pemakaian ulang |
| `POST` | `/auth/logout` | admin, dosen, mahasiswa | Mencabut sesi sendiri; panggilan kedua tetap `204` |
| `GET` | `/me` | admin, dosen, mahasiswa | Mengembalikan role, status, `person_id`, dan `class_ids` milik pemanggil |
| `GET` | `/auth/sessions` | **admin saja** | Melihat sesi aktif seluruh akun |
| `DELETE` | `/auth/sessions/{session_id}` | **admin saja** | Mencabut sesi mana pun; audit `auth.session.revoked` |

### 4.2 Modul 2 — Akun pengguna dan penautan peserta

| Metode | Jalur | Peran | Kewajiban tambahan |
|---|---|---|---|
| `GET` | `/users` | **admin saja** | Daftar akun beserta role dan status |
| `POST` | `/users` | **admin saja** | Membuat akun `invited` (email + role); tidak ada kata sandi; tidak ada role dari klien; audit `user.invited` |
| `PATCH` | `/users/{user_id}` | **admin saja** | Mengubah role/status/nama; `409 LAST_ADMIN_PROTECTED`, `409 SELF_ACCOUNT_PROTECTED`; audit sebelum/sesudah |
| `PUT` | `/people/{person_id}/account-link` | **admin saja** | Menautkan akun ber-role `mahasiswa` ke baris peserta; satu-ke-satu; audit `person.account_linked` |
| `DELETE` | `/people/{person_id}/account-link` | **admin saja** | Melepas tautan; audit `person.account_unlinked`; tidak menghapus data peserta |

### 4.3 Modul 3 — Peserta

| Metode | Jalur | Peran | Kewajiban tambahan |
|---|---|---|---|
| `GET` | `/people` | admin, dosen | `dosen` hanya melihat anggota kelas yang ditugaskan; filter `class_id` di luar penugasan → `404` |
| `POST` | `/people` | **admin saja** | Audit `person.created` |
| `GET` | `/people/{person_id}` | admin, dosen | `dosen` hanya untuk peserta pada kelas yang ditugaskan; selain itu `404` |
| `PATCH` | `/people/{person_id}` | **admin saja** | Audit daftar nama field yang berubah |
| `DELETE` | `/people/{person_id}` | **admin saja** | Wajib menghapus template terkait dalam transaksi sama (D-015); `409 POLICY_CONFLICT` bila retensi belum ditetapkan |

### 4.4 Modul 4 — Persetujuan biometrik

| Metode | Jalur | Peran | Kewajiban tambahan |
|---|---|---|---|
| `GET` | `/people/{person_id}/consent` | **admin saja** | Menampilkan status, bukan isi biometrik |
| `POST` | `/people/{person_id}/consent` | **admin saja** | Mencatat `recorded_by`; audit `consent.granted` |
| `POST` | `/people/{person_id}/consent/withdraw` | **admin saja** | Wajib alasan; memicu prosedur penghapusan `docs/SECURITY_PRIVACY.md` §5f; audit `consent.withdrawn` |

### 4.5 Modul 5 — Enrollment wajah

| Metode | Jalur | Peran | Kewajiban tambahan |
|---|---|---|---|
| `POST` | `/people/{person_id}/face-enrollments` | **admin saja** | Wajib konsen aktif; `422 CONSENT_REQUIRED` |
| `GET` | `/people/{person_id}/face-enrollments` | **admin saja** | Status/progres saja; tidak mengembalikan embedding atau gambar |
| `POST` | `/face-enrollments/{enrollment_id}/samples` | **admin saja** | Validasi jenis/ukuran/waktu proses; gambar tidak disimpan permanen |
| `POST` | `/face-enrollments/{enrollment_id}/complete` | **admin saja** | Audit `enrollment.completed` |
| `DELETE` | `/face-enrollments/{enrollment_id}` | **admin saja** | Audit `template.deleted` |

### 4.6 Modul 6 — Kelas, keanggotaan, dan penugasan dosen

| Metode | Jalur | Peran | Kewajiban tambahan |
|---|---|---|---|
| `GET` | `/classes` | admin, dosen | `dosen` hanya menerima kelas yang ditugaskan kepadanya |
| `POST` | `/classes` | **admin saja** | Audit `class.created` |
| `GET` | `/classes/{class_id}/members` | admin, dosen | `dosen` hanya untuk kelas yang ditugaskan; selain itu `404` |
| `PUT` | `/classes/{class_id}/members/{person_id}` | **admin saja** | Audit `class.member.added` |
| `DELETE` | `/classes/{class_id}/members/{person_id}` | **admin saja** | Audit `class.member.removed` |
| `GET` | `/classes/{class_id}/dosen` | admin, dosen | Daftar penugasan dosen kelas tersebut |
| `PUT` | `/classes/{class_id}/dosen/{user_id}` | **admin saja** | Hanya akun ber-role `dosen`; audit `class.dosen_assigned` |
| `DELETE` | `/classes/{class_id}/dosen/{user_id}` | **admin saja** | Audit `class.dosen_removed` |

### 4.7 Modul 7 — Sesi absensi

| Metode | Jalur | Peran | Kewajiban tambahan |
|---|---|---|---|
| `GET` | `/sessions` | admin, dosen | `dosen` hanya sesi kelas yang ditugaskan |
| `POST` | `/sessions` | admin, dosen | `dosen` hanya untuk kelas yang ditugaskan; `created_by` = pemanggil; `409 SESSION_ALREADY_OPEN` |
| `GET` | `/sessions/{session_id}` | admin, dosen | `dosen` hanya kelas yang ditugaskan; selain itu `404` |
| `POST` | `/sessions/{session_id}/open` | admin, dosen | Hanya dari `draft`; perangkat terkait harus ada dan tidak dicabut |
| `POST` | `/sessions/{session_id}/close` | admin, dosen | Audit `session.closed`; setelah tertutup kiriman kamera ditolak |

### 4.8 Modul 9 — Koreksi manual dan jalur alternatif

| Metode | Jalur | Peran | Kewajiban tambahan |
|---|---|---|---|
| `POST` | `/sessions/{session_id}/manual-checkins` | admin, dosen | **Wajib** `reason`; peserta harus anggota kelas sesi; `dosen` hanya kelas yang ditugaskan; **tidak boleh** dipakai perangkat; audit `attendance.manual_corrected` |
| `GET` | `/sessions/{session_id}/manual-checkins` | admin, dosen | Menampilkan aktor dan alasan; `dosen` hanya kelas yang ditugaskan |

### 4.9 Modul 10 — Perangkat

| Metode | Jalur | Peran | Kewajiban tambahan |
|---|---|---|---|
| `POST` | `/devices` | **admin saja** | Token ditampilkan sekali; audit `device.created` |
| `GET` | `/devices` | **admin saja** | `token_hash`/token mentah tidak pernah dikembalikan |
| `POST` | `/devices/{device_id}/rotate-token` | **admin saja** | Audit `device.token_rotated` |
| `DELETE` | `/devices/{device_id}` | **admin saja** | Perangkat terikat sesi `open` → `409`; audit `device.revoked` |

### 4.10 Modul 11 dan 13 — Rekap, ekspor, dan audit

| Metode | Jalur | Peran | Kewajiban tambahan |
|---|---|---|---|
| `GET` | `/sessions/{session_id}/attendance` | admin, dosen | Tanpa data biometrik; `dosen` hanya kelas yang ditugaskan |
| `GET` | `/sessions/{session_id}/export.csv` | **admin saja** | Tanpa data biometrik; audit `session.exported` (butir §9 butir 2) |
| `GET` | `/audit-events` | **admin saja** | `safe_metadata` tidak memuat JPEG/embedding/token/sandi/email |

### 4.11 Modul 12 — Kesehatan dan kesiapan

| Metode | Jalur | Peran | Kewajiban tambahan |
|---|---|---|---|
| `GET` | `/healthz` | publik (lokal) | Tidak membocorkan rahasia atau jalur berkas |
| `GET` | `/readyz` | admin, dosen | Melaporkan kesiapan layanan absensi manual dan biometrik **terpisah** (`docs/SECURITY_PRIVACY.md` §5e) |

### 4.12 Modul 14 — Portal self-service dosen dan mahasiswa

| Metode | Jalur | Peran | Kewajiban tambahan |
|---|---|---|---|
| `GET` | `/me/classes` | dosen, mahasiswa | `dosen` → kelas yang ditugaskan; `mahasiswa` → kelas yang diikuti; akun `mahasiswa` belum tertaut → `409 ACCOUNT_NOT_LINKED` |
| `GET` | `/me/schedule` | **mahasiswa saja** | Sesi absensi kelas yang diikuti; **tidak** memuat data peserta lain |
| `GET` | `/me/attendance` | **mahasiswa saja** | Riwayat kehadiran sendiri; tanpa embedding, tanpa template, tanpa data orang lain |
| `GET` | `/me/consent` | **mahasiswa saja** | Status persetujuan sendiri, baca saja |

### 4.13 Terminal kamera

| Metode | Jalur | Peran | Kewajiban tambahan |
|---|---|---|---|
| `POST` | `/devices/{device_id}/recognition-attempts` | **device saja** | Token harus cocok `device_id`; `X-Idempotency-Key` wajib; token sesi pengguna **ditolak** |

### 4.14 Rekapitulasi izin

| Kelompok akses | Jumlah endpoint | Cakupan |
|---|---|---|
| Hanya `admin` | 29 | Akun, role, penautan peserta, persetujuan, enrollment, kelola peserta, penugasan dosen, perangkat, ekspor, audit |
| `admin` + `dosen` | 14 | Baca peserta dan kelas yang ditugaskan, sesi absensi, koreksi manual, rekap, kesiapan |
| `admin` + `dosen` + `mahasiswa` | 3 | Sesi sendiri: `refresh`, `logout`, `/me` |
| `dosen` + `mahasiswa` | 1 | `GET /me/classes` |
| Hanya `mahasiswa` | 3 | Jadwal, riwayat kehadiran sendiri, status konsen sendiri |
| Hanya `device` | 1 | Terminal kamera |
| Publik tanpa token | 2 | `POST /auth/google`, `GET /healthz` |
| **Total** | **53** | 29 + 14 + 3 + 1 + 3 + 1 + 2 |

Jumlah endpoint yang boleh dipanggil tiap peran: `admin` **46** · `dosen` **18** · `mahasiswa` **7** · `device` **1** · publik **2**.

**Jumlah lintas peran tidak dijumlahkan menjadi 53.** Satu endpoint dapat diakses beberapa peran; karena itu kolom "Kelompok akses" di atas memakai pengelompokan yang **saling lepas** (setiap endpoint tepat satu kelompok), sedangkan baris "jumlah per peran" boleh tumpang tindih. Angka pada tabel ini **wajib diverifikasi programatis** terhadap tabel §4.1–§4.13 sebelum dipakai sebagai acuan implementasi. Panjang baris 53 dan rekap ini menggantikan rekap 43 endpoint FL-010/FL-010R.

---

## 5. Aturan kepemilikan dan cakupan (deny by default)

1. **Cakupan `admin` = seluruh sistem** dalam lingkup satu kelas demo.
2. **Cakupan `dosen` = kelas yang ditugaskan kepadanya.** Penugasan disimpan sebagai baris `class_lecturers`. Setiap operasi `dosen` pada objek kelas lain → `404 NOT_FOUND` (bukan `403`), dan tidak ada daftar kelas lain yang dibocorkan. Daftar `GET /classes` dan `GET /sessions` **disaring**, bukan ditolak.
3. **Cakupan `mahasiswa` = dirinya sendiri.** Sumber tunggalnya adalah tautan `people.account_user_id` pada baris peserta miliknya. `GET /me/attendance`, `GET /me/schedule`, dan `GET /me/consent` **tidak boleh** menerima parameter `person_id` dari klien. Akun `mahasiswa` yang belum tertaut → `409 ACCOUNT_NOT_LINKED`.
4. **Sesi milik kelasnya.** Seluruh operasi sesi hanya sah bila `session_id` ada dan kelasnya sesuai cakupan pemanggil. Koreksi manual menuntut `person_id` adalah anggota kelas sesi, jika tidak → `422 CLASS_MISMATCH`.
5. **Perangkat milik sesinya.** Token perangkat hanya sah untuk `recognition-attempts` pada `device_id` miliknya; perangkat yang tidak terikat sesi yang dikirim → `422 DEVICE_SESSION_MISMATCH`.
6. **`recognition-attempts` tidak pernah menerima sesi pengguna.** Bahkan `admin` **tidak boleh** menetapkan kehadiran lewat jalur kamera; itu jalur domain mesin AI, bukan jalur administratif.
7. **Tidak ada eskalasi role lewat API.** Tidak ada endpoint yang mengubah role akun sendiri; role hanya ditetapkan `admin` lewat `POST /users` dan `PATCH /users/{user_id}`; tidak ada operasi role massal; tidak ada role yang dibaca dari klien.
8. **Admin terakhir dan akun sendiri dilindungi.** Menonaktifkan atau menurunkan peran admin aktif terakhir → `409 LAST_ADMIN_PROTECTED`. Admin menonaktifkan atau menurunkan peran akunnya sendiri → `409 SELF_ACCOUNT_PROTECTED`.
9. **Sumber daya tersembunyi = `404`.** Bila objek ada tetapi di luar cakupan pemanggil, jawabannya `404 NOT_FOUND`. `403` dipakai ketika peran jelas tidak berwenang atas operasi tersebut, mis. `dosen` memanggil `/users`.
10. **Penautan akun bukan izin biometrik.** `PUT/DELETE /people/{person_id}/account-link` hanya menyambungkan akun mahasiswa ke baris peserta. Ia **tidak** memberi akses ke `face_templates`, embedding, atau berkas enrollment.

---

## 6. Aturan 401 vs 403, anti-enumeration, dan audit

### 6.1 Pembedaan 401 dan 403

| Kondisi | HTTP | `code` | Catatan |
|---|---|---|---|
| Tidak ada token | `401` | `UNAUTHENTICATED` | — |
| Token tidak sah/rusak | `401` | `UNAUTHENTICATED` | Pesan tidak menjelaskan alasan detail |
| Token akses kedaluwarsa | `401` | `TOKEN_EXPIRED` | Klien memakai `refresh` |
| Sesi sudah dicabut (logout admin, akun dinonaktifkan, penyegar dipakai ulang) | `401` | `SESSION_REVOKED` | Klien wajib Google Sign-In ulang |
| Google ID token gagal diverifikasi (issuer, audience, tanda tangan, kedaluwarsa, nonce) | `401` | `IDENTITY_TOKEN_INVALID` | Tidak menjelaskan klaim mana yang gagal |
| Identitas Google tidak tertaut, atau akun belum diberi akses/dinonaktifkan | `403` | `IDENTITY_NOT_LINKED` | **Satu pesan generik** untuk semua sebab; alasan rinci hanya masuk audit |
| Token perangkat pada endpoint manusia | `403` | `FORBIDDEN_ROLE` | Tokennya sah, tetapi salah jenis |
| Sesi pengguna pada endpoint perangkat | `403` | `FORBIDDEN_ROLE` | Idem |
| `dosen`/`mahasiswa` memanggil endpoint khusus `admin` | `403` | `FORBIDDEN_ROLE` | Tidak membocorkan keberadaan operasi bagi peran lain |
| Perangkat dengan `device_id` tidak cocok | `403` | `FORBIDDEN_ROLE` | `docs/API_CONTRACT.md` §5.10 |
| Perangkat dicabut/tidak aktif | `403` | `DEVICE_REVOKED` | `docs/API_CONTRACT.md` §1.5 |
| Objek ada tetapi di luar cakupan (kelas lain, peserta lain, orang lain) | `404` | `NOT_FOUND` | Tidak membocorkan keberadaan |
| Objek benar-benar tidak ada | `404` | `NOT_FOUND` | Tidak dapat dibedakan dari baris di atas |
| Kunci identitas Google tidak dapat dihubungi | `503` | `GOOGLE_UNAVAILABLE` | Berbeda dari `AI_ENGINE_UNAVAILABLE` dan dari backend mati |
| Akun mahasiswa belum tertaut peserta | `409` | `ACCOUNT_NOT_LINKED` | Hanya untuk endpoint `/me/*` milik sendiri |
| Operasi menghapus admin aktif terakhir | `409` | `LAST_ADMIN_PROTECTED` | — |
| Admin menonaktifkan/menurunkan akunnya sendiri | `409` | `SELF_ACCOUNT_PROTECTED` | — |

**Aturan keras:** perbedaan 401 vs 403 **tidak boleh** dipakai untuk memberi tahu klien hal yang tidak boleh diketahuinya. Respons autentikasi **tidak membedakan** "email tidak terdaftar", "akun dinonaktifkan", dan "identitas belum tertaut"; hanya audit internal yang menyimpan sebabnya.

### 6.2 Audit otorisasi dan tindakan administratif

| Peristiwa | Dicatat ke `audit_events`? | Alasan |
|---|---|---|
| `auth.google.succeeded`, `auth.google.linked` | **Ya** | Jejak pembuatan sesi dan penautan identitas pada login pertama |
| `auth.login.denied` | **Ya** | Menyimpan `reason` ber-kode (`no_account`, `not_linked`, `suspended`, `email_mismatch`), **tanpa** email atau `sub` |
| `auth.session.refreshed`, `auth.session.revoked`, `auth.session.reuse_detected`, `auth.logout` | **Ya** | Pencabutan dan deteksi penyalahgunaan harus dapat ditelusuri |
| `user.bootstrapped`, `user.invited`, `user.role_changed`, `user.status_changed` | **Ya** (usulan perluasan, §9 butir 3) | Perubahan hak akses |
| `person.account_linked`, `person.account_unlinked` | **Ya** (usulan perluasan) | Penautan akun–biometrik harus dapat diaudit |
| `class.dosen_assigned`, `class.dosen_removed` | **Ya** (usulan perluasan) | Perubahan cakupan akses |
| Penolakan otorisasi pada jalur administratif (`authz.denied`) | **Usulan lama, belum disetujui** | FL-010 §9 butir 4 masih menunggu keputusan pemilik; jangan diperlakukan sebagai kontrak yang berlaku |
| Perangkat ditolak karena `device_id` tidak cocok (`auth.device.mismatch`) | **Usulan lama, belum disetujui** | Idem |
| Permintaan tanpa token pada endpoint terlindungi | **Tidak wajib** | Volume tinggi, tanpa aktor teridentifikasi |
| Permintaan ke sumber daya di luar cakupan (`404`) | **Tidak wajib** | Tidak dapat dibedakan dari "tidak ada" tanpa membocorkan |

**Bidang audit yang dilarang:** `safe_metadata` **tidak pernah** memuat JPEG, embedding, token, hash, kata sandi, alamat email, klaim `sub`, atau ID token Google.

### 6.3 Gagal-aman

Sesuai `docs/SECURITY_PRIVACY.md` §5e butir 4, otorisasi dan autentikasi **tidak boleh** dilumpuhkan agar layanan terlihat hidup. Bila komponen autentikasi tidak sehat, endpoint manusia **wajib** menolak melayani. Absensi manual tetap menjadi jalur tetap, tetapi **hanya** melalui endpoint terautentikasi `POST /sessions/{session_id}/manual-checkins` — tidak pernah melalui jalur yang menonaktifkan keamanan, dan tidak pernah melalui jalur tanpa token.

---

## 7. Siklus hidup entitas dan aturan lifecycle otorisasi

| Entitas | Keadaan | Siapa yang boleh mengubah | Aturan otorisasi terkait |
|---|---|---|---|
| `users` | `invited` / `active` / `suspended` | Hanya `admin` | Tidak ada penghapusan keras; akun tetap ada sebagai `actor_id` audit; admin terakhir dan akun sendiri dilindungi (§5 butir 8) |
| `user_identities` | tertaut / tidak ada | Sistem pada login pertama | Kunci `(provider='google', subject)`; satu akun satu identitas; tautan tidak dapat dipindah ke akun lain lewat API |
| `session_tokens` | aktif / revoked / expired | Pemilik (logout), `admin` (cabut), sistem (penonaktifan akun, deteksi pemakaian ulang) | Token tidak berlaku setelah `revoked_at`; baris kedaluwarsa dibersihkan pemeliharaan |
| `people` | active/inactive/soft-deleted | `admin` (baca juga `dosen` sesuai penugasan) | Hapus menuntut penghapusan template dalam transaksi sama (D-015) |
| `people.account_user_id` | tertaut / null | Hanya `admin` | Satu-ke-satu dengan akun ber-role `mahasiswa`; memberi akses baca-sendiri, **bukan** akses biometrik |
| `classes` | aktif / soft-deleted | `admin` | `dosen` hanya melihat kelas yang ditugaskan |
| `class_lecturers` | ada / tidak ada | Hanya `admin` | Dasar seluruh cakupan `dosen` |
| `class_memberships` | ada / tidak ada | Hanya `admin` | Penghapusan keras, tanpa makna historis sendiri |
| `attendance_sessions` | draft → open → closed | `admin`, `dosen` (kelas yang ditugaskan) | Satu sesi `open` per kelas **dan** per perangkat dijaga indeks unik parsial |
| `attendance_records` | ditulis sekali, dikoreksi in-place | Backend (device/manual) | **Tidak pernah** ditulis `absent`; koreksi memperbarui baris yang sama |
| `consent_records` | aktif / withdrawn | Hanya `admin` | Pencabutan memicu penghapusan template dan menghentikan pencocokan |
| `face_enrollments`/`samples` | berjalan / selesai / dibatalkan | Hanya `admin` | Wajib konsen aktif; gambar tidak ditahan |
| `face_templates` | ada / dihapus | Hanya `admin` | **Belum diimplementasikan**; kebijakan D-022/D-030 tidak berubah oleh FL-010U |
| `devices` | aktif / revoked | Hanya `admin` | Token perangkat terikat satu device |
| `audit_events` | hanya tulis | Sistem | Tidak ada endpoint yang mengubah/menghapus audit; hanya `GET /audit-events` oleh `admin` |

**Aturan turunan yang mengikat otorisasi:**

- **Tidak ada jalur yang menulis keputusan kehadiran tanpa autentikasi.** Baik `recognition-attempts` (device) maupun `manual-checkins` (admin/dosen) menuntut kredensial.
- **Perubahan izin tidak menyentuh biometrik.** Menambah/menonaktifkan akun atau mengubah role **tidak boleh** menghapus atau membuat template (`docs/SECURITY_PRIVACY.md` §5e butir 2).
- **Koreksi setelah sesi ditutup hanya lewat jalur manual teraudit** (`docs/API_CONTRACT.md` §5.6), dan itu **hanya** `admin`/`dosen` pada cakupannya.
- **Pencabutan akses berlaku cepat.** Penonaktifan akun mencabut seluruh sesi akun itu dan role dibaca ulang pada setiap permintaan, sehingga sesi yang masih tersimpan di perangkat tidak melanjutkan akses.

---

## 8. Kriteria penerimaan dan skenario uji penolakan akses

Mengunci keputusan FL-010U sebagai **kontrak yang dapat diuji** untuk implementasi autentikasi dan seterusnya.

### 8.1 Kriteria penerimaan otorisasi

| ID | Kriteria penerimaan |
|---|---|
| AC-AUTHZ-01 | Setiap endpoint §4 menolak pemanggil di luar peran yang tercantum: `401` bila tanpa/token tidak sah, `403 FORBIDDEN_ROLE` bila peran salah |
| AC-AUTHZ-02 | Token perangkat **tidak** dapat memanggil endpoint manusia mana pun; diuji untuk minimal `/me`, `/people`, `/sessions`, `/users` |
| AC-AUTHZ-03 | Sesi pengguna **tidak** dapat memanggil `recognition-attempts` (termasuk `admin`) |
| AC-AUTHZ-04 | `dosen` dan `mahasiswa` mendapat `403 FORBIDDEN_ROLE` pada endpoint khusus `admin` (akun, role, penautan, konsen, enrollment, perangkat, ekspor, audit) |
| AC-AUTHZ-05 | Sumber daya di luar cakupan mengembalikan `404 NOT_FOUND`, termasuk kelas lain bagi `dosen` dan peserta lain bagi `mahasiswa` |
| AC-AUTHZ-06 | `POST /sessions/{session_id}/manual-checkins` menolak penulisan tanpa `reason` dan tanpa anggota kelas |
| AC-AUTHZ-07 | Setiap penolakan otorisasi pada jalur administratif/absensi manual dan setiap kegagalan autentikasi masuk `audit_events` **tanpa** data biometrik, token, email, atau `sub` |
| AC-AUTHZ-08 | Tidak ada endpoint yang mengubah role akun sendiri; tidak ada operasi role massal (tidak ada eskalasi) |
| AC-AUTHZ-09 | Ketika komponen autentikasi tidak sehat, endpoint manusia menolak melayani (gagal-aman), bukan melewati pemeriksaan |
| AC-AUTHZ-10 | `GET /audit-events` tidak mengembalikan `safe_metadata` yang memuat token, sandi, JPEG, embedding, atau email |
| AC-AUTHZ-11 | Google ID token yang gagal verifikasi (issuer/audience/tanda tangan/kedaluwarsa/nonce) ditolak dan tidak pernah menghasilkan sesi |
| AC-AUTHZ-12 | Role **hanya** berasal dari tabel `users`; tidak ada klaim Google, permintaan klien, atau keadaan default yang dapat menetapkannya |
| AC-AUTHZ-13 | Tidak ada akun yang dibuat otomatis untuk identitas Google yang belum didaftarkan `admin`; pengguna Google pertama tidak menjadi admin |
| AC-AUTHZ-14 | Penonaktifan akun mencabut seluruh sesinya, dan permintaan berikutnya dengan token lama ditolak |
| AC-AUTHZ-15 | Perubahan role berlaku pada permintaan berikutnya tanpa menunggu token kedaluwarsa |
| AC-AUTHZ-16 | Admin terakhir dan akun sendiri tidak dapat dinonaktifkan/diturunkan perannya (`409`) |
| AC-AUTHZ-17 | `mahasiswa` hanya dapat membaca datanya sendiri; tidak ada parameter klien yang dapat mengarahkan ke `person_id` lain |
| AC-AUTHZ-18 | Pemakaian ulang token penyegar yang sudah dirotasi mencabut sesi dan tercatat di audit |

### 8.2 Skenario uji penolakan akses (negatif)

Melengkapi `docs/TEST_PLAN.md` §2 dan §7–§8. Diberi ID `TA-xx` agar dapat dirujuk tanpa menimpa penomoran T yang sudah ada.

| ID | Skenario | Harapan |
|---|---|---|
| TA-01 | Tanpa token memanggil `GET /people` | `401 UNAUTHENTICATED`; tanpa data |
| TA-02 | Token akses kedaluwarsa memanggil `GET /sessions` | `401 TOKEN_EXPIRED`; tidak ada data |
| TA-03 | Token perangkat memanggil `/me`, `/people`, `/sessions`, `/users` | `403 FORBIDDEN_ROLE` pada keempat jalur |
| TA-04 | Sesi pengguna memanggil `POST /devices/{id}/recognition-attempts` | `403 FORBIDDEN_ROLE` (bukan `200`) |
| TA-05 | Token perangkat memakai `device_id` perangkat lain | `403` |
| TA-06 | `dosen` memanggil `GET /users` dan `POST /users` | `403 FORBIDDEN_ROLE` |
| TA-07 | `dosen` memanggil `DELETE /people/{id}`, `POST /devices`, dan `GET /audit-events` | `403 FORBIDDEN_ROLE` pada ketiganya |
| TA-08 | `dosen` mencoba mengubah role dirinya sendiri | Tidak ada endpoint yang mengizinkan; role tidak berubah |
| TA-09 | Koreksi manual untuk peserta bukan anggota kelas | `422 CLASS_MISMATCH`; tidak ada baris kehadiran |
| TA-10 | Koreksi manual tanpa `reason` | `400`/`422 REASON_REQUIRED` |
| TA-11 | Perangkat terikat sesi A mengirim percobaan untuk sesi B | `422 DEVICE_SESSION_MISMATCH` |
| TA-12 | Perangkat tercabut mengirim `recognition-attempts` | `403 DEVICE_REVOKED` |
| TA-13 | `dosen` membuka kelas/sesi di luar penugasan | `404 NOT_FOUND`; tidak membocorkan jumlah kelas lain |
| TA-14 | `GET /audit-events` pada sistem yang punya koreksi manual | `safe_metadata` bebas token/sandi/JPEG/embedding/email |
| TA-15 | Komponen autentikasi sengaja dimatikan, lalu `POST /manual-checkins` | Ditolak (gagal-aman); absensi manual hanya lewat jalur terautentikasi yang sehat |
| TA-16 | `mahasiswa` memanggil `GET /me/attendance?person_id=<orang lain>` | Parameter diabaikan/ditolak; **tidak ada** data orang lain yang keluar |
| TA-17 | `mahasiswa` memanggil `GET /sessions/{id}/attendance` atau `/people` | `403 FORBIDDEN_ROLE` |
| TA-18 | Akun `mahasiswa` belum tertaut memanggil `/me/attendance` | `409 ACCOUNT_NOT_LINKED`; tidak ada data |
| TA-19 | Google ID token dengan audience/issuer/tanda tangan salah | `401 IDENTITY_TOKEN_INVALID`; tidak ada sesi baru |
| TA-20 | Identitas Google sah yang belum didaftarkan admin masuk | `403 IDENTITY_NOT_LINKED`; **tidak ada** baris `users` baru; audit `auth.login.denied` |
| TA-21 | Admin mencoba menonaktifkan admin aktif terakhir dan akunnya sendiri | `409 LAST_ADMIN_PROTECTED` dan `409 SELF_ACCOUNT_PROTECTED` |
| TA-22 | Token penyegar lama dikirim setelah rotasi | `401 SESSION_REVOKED`; seluruh sesi tercabut; audit mencatat pemakaian ulang |

**Catatan pelaksanaan:** skenario ini **belum dijalankan** karena kode autentikasi belum ada. Statusnya **BELUM TERVERIFIKASI** dan menjadi bagian tugas implementasi berikutnya.

---

## 9. Konflik, koreksi, dan hal yang butuh keputusan pemilik

Bagian ini dicatat apa adanya. FL-010U menuliskan rancangan berdasarkan keputusan pemilik; hal-hal berikut **belum** diputuskan dan tidak boleh diperlakukan sebagai kontrak yang berlaku.

1. **Cakupan `dosen` mengikuti daftar tugas FL-010U §4 secara harfiah.** Akibat yang harus dipahami pemilik: **enrollment biometrik, pencatatan/pencabutan konsen, pengelolaan peserta, ekspor CSV, dan seluruh operasi perangkat menjadi milik `admin`**. Bila alur demo menuntut dosen melakukan enrollment atau mencatat konsen di kelas yang ditugaskannya, matriks §4 harus diperluas lewat keputusan pemilik, bukan lewat penafsiran agent.
2. **Ekspor CSV hanya `admin`.** Dosen dapat melihat rekap pada aplikasi tetapi tidak mengunduh berkas ekspor. Bila dosen memerlukannya, yang berubah hanya kolom peran pada baris itu — bukan endpoint baru.
3. **Perluasan daftar audit minimum** (§6.2) menunggu persetujuan pemilik. Selama belum disetujui, aksi baru ditulis sebagai usulan, dan `authz.denied` serta `auth.device.mismatch` tetap usulan lama FL-010 yang belum disetujui.
4. **Satu akun Google = satu baris `users` = satu role.** Menjalankan demo sebagai admin sekaligus dosen memerlukan dua akun Google berbeda. Bila pemilik menghendaki satu orang memegang dua role (mis. dosen yang juga mengelola akunnya sendiri), itu perubahan model data dan otorisasi yang harus diputuskan lebih dahulu.
5. **Jalur absensi saat internet mati dan sesi habis tidak dirancang.** Google Sign-In menuntut internet (§10 butir 1). Bila pemilik menghendaki absensi tetap dapat dicatat tanpa sesi yang sah, itu keputusan kebijakan tersendiri — bukan penambahan yang boleh diputuskan agent.
6. **Penarikan konsen oleh mahasiswa sendiri dari aplikasi** (P1) belum dirancang; pencatatan dan pencabutan konsen tetap melalui `admin` sesuai matriks §4.
7. **Jadwal akademik penuh tidak ada.** `GET /me/schedule` menampilkan sesi absensi nyata, bukan jadwal mata kuliah. Bila diperlukan tabel jadwal, itu keputusan model data baru.
8. **Rujukan rusak pada keputusan lama.** `docs/DECISIONS.md` D-028 menunjuk `docs/UX_FLOWS.md` §10, sedangkan dokumen itu hanya memiliki §1–§8. Target yang dimaksud **tidak dapat ditentukan dengan pasti** dan **tidak boleh ditebak**; pemilik perlu menyebut bagian yang dimaksud. Dicatat sebagai masalah terbuka.
9. **Selisih dokumen vs migrasi yang sudah dijalankan.** `docs/DATA_MODEL.md` §4.1–§4.3 (versi dokumen) berbeda dari `backend/migrations/00001_init_core_schema.sql` yang sudah dijalankan (mis. `users.status` bernilai `active/disabled`, adanya `deleted_at`, `people.consent_state`, `operator_tokens` tanpa kolom refresh). FL-011 wajib memulai dari keadaan nyata migrasi 00001 dan menulis migrasi **baru**, bukan mengubah 00001. Selisih lengkap dicatat di `docs/DATA_MODEL.md`.

---

## 10. Yang secara sengaja tidak ada di MVP

- Tidak ada multi-tenancy, organisasi, atau isolasi antar-kelas di atas satu kelas.
- Tidak ada kata sandi internal, kata sandi sementara, atau pemulihan kata sandi: identitas pengguna sepenuhnya melalui Google Sign-In.
- Tidak ada pendaftaran mandiri, tidak ada approval queue `pending`, dan tidak ada akun otomatis dari login Google.
- Tidak ada peran dinamis/tabel izin-detail (`roles` hanya daftar acuan, bukan sistem izin dinamis).
- Tidak ada login perangkat lewat Google, dan tidak ada perangkat yang dihitung sebagai pengguna manusia.
- Tidak ada endpoint yang menulis kehadiran tanpa kredensial.
- Tidak ada SSO organisasi, LDAP, atau penyedia identitas kedua pada MVP.
- Tidak ada cloud, billing, HRIS, atau CCTV terus-menerus (D-016 `DITUNDA`).

---

## 11. Rujukan silang

| Dokumen | Bagian yang dikunci oleh dokumen ini |
|---|---|
| `docs/API_CONTRACT.md` | §1.2 (peran), §4 (per-endpoint), §5.1/§5.2/§5.6/§5.8/§5.9/§5.11/§5.13 (aturan spesifik), kode kesalahan baru |
| `docs/DATA_MODEL.md` | §4.1–§4.4 (users, session_tokens, roles, people), `user_identities`, `class_lecturers` |
| `docs/SECURITY_PRIVACY.md` | §6 (kendali akses dan sesi), §5e (gagal-aman dan kesiapan terpisah) |
| `docs/DECISIONS.md` | D-013 (digantikan), D-028, D-031–D-034 |
| `docs/TEST_PLAN.md` | §2, §7, dan §8 (TA-01..TA-22, GA-xx) |
| `docs/UX_FLOWS.md` | §2 (navigasi tiga role), §3 (keadaan UI) |
| `docs/PRD.md` | §3 (pengguna dan hak akses), §7–§8 (FR dan aturan bisnis) |
| `docs/ARCHITECTURE.md` | §4 (modul backend), §5 (zona kepercayaan), §9 (status keputusan) |

# Model Data FaceLink AI

**Versi:** 1.0 · **Status:** SPESIFIKASI FINAL TAHAP DOKUMENTASI; **BELUM DIIMPLEMENTASIKAN** dan belum ada migrasi yang dijalankan.

**Revisi FL-010U (16 Oktober 2026):** diselaraskan dengan arsitektur **tiga role manusia** (`admin`, `dosen`, `mahasiswa`) yang masuk lewat **Google Sign-In**, dan `device` sebagai **autentikasi mesin terpisah** (bukan role manusia). Revisi ini berasal dari cabang `docs/fl-010u-google-three-roles`, bercabang dari `origin/main` (`804c2f7`). Matriks izin lengkap ada di `docs/AUTHORIZATION.md`. **FL-010U adalah tahap dokumentasi saja:** tidak ada kode, SQL, atau migrasi yang ditulis pada revisi ini. Perubahan skema pada §4.1–§4.4, §4.16, dan §4.17 adalah **model data target**, bukan keadaan database saat ini; selisihnya terhadap migrasi yang sudah dijalankan dicatat di §4.18.

**Revisi FL-010V (16 Oktober 2026):** memfinalkan aturan data untuk autentikasi Google dan **satu role utama per akun** (D-032, D-033, D-034, D-035, D-036, D-037 — lihat `docs/DECISIONS.md`). Yang berubah: §4.1 (satu role utama aktif + siklus hidup status + `created_by = NULL` pada bootstrap), §4.2 (kolom `absolute_expires_at` beserta aturannya), §4.16 (aturan penautan atomik dan tahan-race, email sekali pakai), §4.19 (**tabel target baru `auth_nonces`**), §12 (pengujian wajib tambahan), dan §13 (daftar hal terbuka). **FL-010V adalah tahap dokumentasi saja:** tidak ada kode, SQL, atau migrasi yang ditulis, dan **tidak ada migrasi yang dijalankan**. Seluruh pernyataan runtime pada dokumen ini tetap **BELUM TERVERIFIKASI**. Matriks izin tetap bersumber tunggal pada `docs/AUTHORIZATION.md`.

Dokumen ini melengkapi `docs/API_CONTRACT.md` dan tidak menggantikan `docs/PRD.md`. Setiap perubahan skema setelah dokumen ini harus melalui berkas migrasi baru, bukan dengan mengubah migrasi yang sudah pernah dijalankan.

## 1. Ruang lingkup dan asumsi

1. MVP melayani **satu kelas dan satu terminal**, dipakai oleh **tiga peran manusia** (`admin`, `dosen`, `mahasiswa`) yang masuk lewat Google Sign-In, ditambah **`device`** yang bukan peran manusia melainkan identitas mesin terpisah (batasan AGENTS.md §2; rujukan peran: `docs/AUTHORIZATION.md` §2). Skema tetap tidak boleh mengunci diri sehingga organisasi lain mustahil ditambahkan (D-020 catatan portabilitas).
2. **Tidak ada multi-tenancy**, tidak ada payroll, tidak ada billing, dan tidak ada tabel `organization_id` yang dipaksakan pada seluruh tabel. Namun setiap entitas utama memiliki `id` UUID dan `created_at`, sehingga penambahan kolom pengelompokan di masa depan bersifat aditif.
3. Database MVP: **SQLite mode WAL** (D-020); target lanjutan: **PostgreSQL** (D-011 masih DITUNDA).
4. Semua waktu disimpan sebagai **TEXT ISO 8601 UTC** (`2026-10-15T08:00:00Z`). Tidak ada waktu lokal yang disimpan tanpa penanda zona.
5. Semua identitas memakai **UUID v4 berbentuk TEXT** agar portabel ke PostgreSQL dan mudah disalin antar berkas uji. Tidak ada primary key berupa auto-increment bilangan bulat yang bocor ke API, **kecuali** tabel teknis yang tidak pernah menjadi identitas domain (§4.14).
6. Nilai boolean disimpan sebagai `INTEGER` 0/1 pada SQLite dan dipetakan ke `BOOLEAN` pada PostgreSQL.
7. JSON disimpan sebagai `TEXT` berisi JSON yang valid pada SQLite; pada PostgreSQL menjadi `JSONB`.

## 2. Ikhtisar tabel

| # | Tabel | Kegunaan | Modul API terkait |
|---|---|---|---|
| 1 | `users` | Akun manusia (`admin`, `dosen`, `mahasiswa`) | §5.1, §5.2 |
| 2 | `session_tokens` | Token sesi aplikasi (akses + penyegar) yang dapat dicabut | §5.1 |
| 3 | `roles` | Daftar peran manusia yang sah beserta deskripsinya | §5.1 |
| 4 | `people` | Peserta (identitas non-biometrik) | §5.2 |
| 5 | `classes` | Kelas atau kelompok | §5.5 |
| 6 | `class_memberships` | Keanggotaan peserta pada kelas | §5.5 |
| 7 | `attendance_sessions` | Sesi absensi | §5.6 |
| 8 | `attendance_records` | Catatan kehadiran (penjaga anti-duplikasi) | §5.7, §5.8, §5.10 |
| 9 | `consent_records` | Riwayat persetujuan biometrik | §5.3 |
| 10 | `face_enrollments` | Sesi pendaftaran wajah berjangka waktu | §5.4 |
| 11 | `face_enrollment_samples` | Metadata kualitas sampel enrollment (bukan gambar) | §5.4 |
| 12 | `face_templates` | Template wajah terenkripsi | §5.3, §5.4, §6 |
| 13 | `devices` | Perangkat kamera terdaftar (identitas mesin) | §5.9 |
| 14 | `audit_events` | Jejak aktivitas penting | §5.13 |
| 15 | `idempotency_records` | Penjaga pengulangan permintaan jaringan | §8 |
| 16 | `user_identities` | Penautan identitas penyedia (Google) ke akun `users` (§4.16) | §5.1 |
| 17 | `class_lecturers` | Penugasan dosen pada kelas (§4.17) | §5.5 |
| 19 | `auth_nonces` | Nonce sekali pakai untuk `POST /auth/google` (§4.19) | §5.1 |

Tabel 16 dan 17 adalah **tabel baru pada model data target** FL-010U; keduanya **belum ada** pada skema yang sudah dijalankan (§4.18). Tabel 19 (`auth_nonces`) adalah **tabel target baru FL-010V (16 Oktober 2026)** yang juga **belum ada** pada skema yang sudah dijalankan. Penomoran `#` mengikuti nomor sub-bagian pada §4; nomor 18 tidak muncul di sini karena §4.18 adalah catatan selisih migrasi, bukan definisi tabel.

Diagram hubungan ringkas (`||--o{` berarti satu-ke-banyak):

```text
users ||--o{ session_tokens
users ||--o{ user_identities         (identitas Google; ON DELETE CASCADE)
users ||--o{ audit_events            (actor_id, tanpa foreign key keras ke users)
roles ||--o{ users                   (role disimpan sebagai TEXT + daftar acuan)
users ||--o{ classes                 (class_lecturers: penugasan dosen, §4.17)
users ||--o{ people                  (people.account_user_id, ON DELETE SET NULL, unik)
classes ||--o{ class_memberships
classes ||--o{ class_lecturers
people  ||--o{ class_memberships
classes ||--o{ attendance_sessions
people  ||--o{ attendance_records
attendance_sessions ||--o{ attendance_records
people  ||--o{ consent_records
people  ||--o{ face_enrollments
face_enrollments ||--o{ face_enrollment_samples
people  ||--o{ face_templates
devices ||--o{ attendance_sessions    (satu perangkat, banyak sesi sepanjang waktu)
```

## 3. Aturan umum kolom

| Kolom | Tipe SQLite | Tipe PostgreSQL | Aturan |
|---|---|---|---|
| `id` | `TEXT` | `UUID` | Primary key, UUID v4, wajib |
| `created_at` | `TEXT` | `TIMESTAMPTZ` | Wajib, bawaan waktu UTC sekarang |
| `updated_at` | `TEXT` | `TIMESTAMPTZ` | Nullable sampai baris pertama diubah |
| `deleted_at` | `TEXT` | `TIMESTAMPTZ` | Nullable; hanya pada tabel yang memakai penghapusan lunak |

Konvensi penamaan: `snake_case`; kunci asing bernama `<tabel_tunggal>_id`; indeks bernama `idx_<tabel>_<kolom>`; batasan unik bernama `uq_<tabel>_<kolom>`; `CHECK` bernama `ck_<tabel>_<aturan>`.

## 4. Definisi tabel

### 4.1 `users`

Akun manusia yang memakai aplikasi. Terdapat **tiga peran manusia**: `admin`, `dosen`, dan `mahasiswa`. Tidak ada login email/kata sandi buatan sendiri; autentikasi memakai **Google Sign-In** dan identitas penyedia disimpan di `user_identities` (§4.16). Revisi FL-010U (16 Oktober 2026).

| Kolom | Tipe | Null | Aturan |
|---|---|---|---|
| `id` | TEXT | tidak | PK |
| `email` | TEXT | tidak | Unik, disimpan huruf kecil |
| `display_name` | TEXT | tidak | 1–120 karakter |
| `role` | TEXT | tidak | `CHECK (role IN ('admin','dosen','mahasiswa'))` |
| `status` | TEXT | tidak | `CHECK (status IN ('invited','active','suspended'))`, bawaan `invited` |
| `created_by` | TEXT | ya | FK → `users.id` — admin yang membuat akun; **null** hanya untuk akun bootstrap pertama |
| `last_login_at` | TEXT | ya | Diperbarui saat login berhasil |
| `created_at` | TEXT | tidak | — |
| `updated_at` | TEXT | ya | — |

- **Dihapus dari model data FL-010U (DIGANTIKAN):** kolom `password_hash` dan `must_change_password`. Alasannya: Google Sign-In menggantikan kata sandi buatan sendiri, sehingga tidak ada kata sandi yang perlu di-hash atau dipaksa diganti. Tidak ada kolom pengganti berbentuk rahasia apa pun pada tabel ini.
- **Unik:** `email`.
- **Indeks:** `idx_users_role`, `idx_users_status`.
- **Pembuatan akun:** akun **dibuat oleh `admin` (praregistrasi)** dengan status `invited`; tidak ada pendaftaran mandiri dan **tidak ada role otomatis**. Baris `users` bertambah hanya lewat tindakan admin (atau bootstrap pertama saat sistem masih kosong).
- **Penautan identitas Google:** identitas Google ditautkan pada **login pertama**. Backend boleh menautkan **hanya bila** ada baris `users` berstatus `invited` dengan `email` yang **sama persis** (huruf kecil, tanpa spasi tepi) dan token Google menyatakan `email_verified=true`; baris itu lalu menjadi `active`. Selain itu **tidak ada** akun yang dibuat dan tidak ada role yang ditetapkan. Rincian kolom ada di §4.16.
- **Status:** `invited` = akun sudah dipraregistrasi tetapi identitas Google belum tertaut atau belum diaktifkan; `active` = boleh memakai aplikasi; `suspended` = ditolak. `invited` (belum tertaut) dan `suspended` sama-sama **ditolak** dengan satu pesan generik (anti-enumeration).
- **Satu role utama per akun (D-037, DISETUJUI untuk MVP; FL-010V, 16 Oktober 2026):** satu akun internal memiliki **tepat satu** role utama aktif. Kolom `role` bersifat tunggal dan tidak ada tabel relasi akun–role pada MVP; **tidak ada multi-role switching**. Satu identitas Google **tidak boleh** diam-diam menghasilkan beberapa sesi dengan role berbeda. Hak istimewa `admin` ditetapkan backend dan memungkinkan pengelolaan kelas sesuai matriks **tanpa** mengubah admin menjadi `dosen` (`docs/AUTHORIZATION.md`). Perubahan role hanya oleh `admin` berwenang dan **wajib diaudit** (`user.role_changed`). Aturan `409 LAST_ADMIN_PROTECTED` (penonaktifan admin aktif terakhir) dan `409 SELF_ACCOUNT_PROTECTED` (akun sendiri) tetap berlaku apa adanya.
- **Siklus hidup status yang disetujui (D-032; FL-010V, 16 Oktober 2026):** urutan yang sah adalah `invited` → `active` → `suspended`. Perpindahan `invited` → `active` **hanya** terjadi lewat penautan identitas Google pada login pertama (§4.16), bukan lewat tindakan lain. Tidak ada penghapusan keras akun; akun yang tidak boleh dipakai lagi berstatus `suspended`. Perpindahan status dicatat di audit (`user.status_changed`), dan penonaktifan akun **mencabut seluruh sesi** akun tersebut (§4.2).
- **Bootstrap admin pertama (D-032; FL-010V, 16 Oktober 2026):** proses **lokal, eksplisit, sekali, dan dapat diaudit**, tanpa kata sandi/token/kredensial yang tertanam di kode atau repositori. Proses ini **menolak berjalan** bila sudah ada akun `admin` berstatus `active` atau `invited`. Baris yang ditulisnya adalah **satu** baris `users` dengan `role='admin'` dan `status='invited'`, dengan **`created_by = NULL`** — inilah satu-satunya baris `users` yang boleh bernilai `created_by = NULL`. Baris itu ditautkan ke identitas Google pada login pertama seperti akun undangan lain, dan pembuatannya dicatat di audit (`user.bootstrapped`).
- **Perubahan role dan status berlaku pada permintaan berikutnya**, bukan menunggu token kedaluwarsa: role **selalu** dibaca ulang dari database pada setiap permintaan dan **tidak pernah** diambil dari Google, dari aplikasi React Native, atau dari klaim token.
- **Penghapusan:** akun tidak dihapus keras selama masih menjadi `actor_id` pada audit maupun `created_by`; gunakan `status = 'suspended'`. Penonaktifan akun **mencabut seluruh sesi** akun tersebut (`session_tokens`, §4.2).
- **Retensi:** selama sistem dipakai.

### 4.2 `session_tokens`

Menyimpan token sesi aplikasi agar dapat dicabut. **Nama tabel ini menggantikan `operator_tokens`** (FL-010U §8; lihat §4.18). Yang disimpan **hanya hash SHA-256**; token mentah **tidak pernah** disimpan atau dicatat di log. Token berbentuk **opak acak 256-bit** (bukan JWT) dan dikirim sebagai `Authorization: Bearer <token>`; Google ID token **bukan** token sesi aplikasi ini.

| Kolom | Tipe | Null | Aturan |
|---|---|---|---|
| `id` | TEXT | tidak | PK |
| `user_id` | TEXT | tidak | FK → `users.id` `ON DELETE CASCADE` |
| `access_hash` | TEXT | tidak | Unik; hash token akses |
| `access_expires_at` | TEXT | tidak | Masa berlaku token akses 60 menit |
| `refresh_hash` | TEXT | tidak | Unik; hash token penyegar aktif |
| `refresh_expires_at` | TEXT | tidak | Masa berlaku token penyegar 30 hari |
| `absolute_expires_at` | TEXT | tidak | **FL-010V (16 Oktober 2026).** Batas absolut sesi: `created_at` + maksimum 30 hari, **tidak pernah** diperpanjang oleh rotasi penyegar |
| `prev_refresh_hash` | TEXT | ya | Unik bila ada; hash penyegar **sebelum** rotasi terakhir, disimpan satu langkah untuk deteksi pemakaian ulang |
| `revoked_at` | TEXT | ya | Diisi saat logout, pencabutan admin, penonaktifan akun, atau deteksi pemakaian ulang |
| `revoked_reason` | TEXT | ya | Wajib bila `revoked_at` diisi (mis. `logout`, `admin_revoke`, `account_suspended`, `refresh_reuse`) |
| `last_seen_at` | TEXT | ya | Diperbarui pada pemakaian token |
| `user_agent_ringkas` | TEXT | ya | Maksimum 120 karakter |
| `created_at` | TEXT | tidak | — |

- **Unik:** `access_hash`, `refresh_hash`, `prev_refresh_hash`.
- **Indeks:** `idx_session_tokens_user`, `idx_session_tokens_expires`, `idx_session_tokens_absolute_expires`.
- **Batas absolut sesi (D-033; FL-010V, 16 Oktober 2026):** kolom `absolute_expires_at` wajib diisi saat baris sesi dibuat dan nilainya **paling lama** `created_at` + 30 hari. Batas ini **tidak pernah** diperpanjang oleh `POST /auth/refresh`; rotasi penyegar hanya menerbitkan token baru yang tetap tunduk pada batas yang sama. Setiap penyegaran **wajib** memeriksa `now < absolute_expires_at` sebelum menerbitkan pasangan token baru; bila batas itu terlewati, penyegaran ditolak dan sesi dicabut. Satu-satunya jalan melewati batas ini adalah **autentikasi baru yang sah** lewat Google Sign-In ulang, yang membuat **baris sesi baru** (dengan `created_at` dan `absolute_expires_at` baru), bukan memperpanjang baris lama.
- **Batas relatif vs batas absolut:** `access_expires_at` (60 menit) dan `refresh_expires_at` (30 hari) adalah masa berlaku token; `absolute_expires_at` adalah batas umur **sesi**. Karena itu `refresh_expires_at` **tidak boleh** melampaui `absolute_expires_at`, dan rotasi berulang tidak dapat memindahkan batas absolut.
- **Rotasi token penyegar (D-033):** setiap `POST /auth/refresh` menerbitkan **pasangan token baru** (akses + penyegar) dan menggantikan hash penyegar lama. Hash penyegar sebelumnya dipindahkan ke `prev_refresh_hash` **satu langkah** saja, agar pemakaian kembali penyegar lama dapat dikenali.
- **Deteksi pemakaian ulang (D-033):** penyegar yang **sudah dirotasi** dipakai kembali → **seluruh sesi dicabut** (`revoked_at` diisi, `revoked_reason = 'refresh_reuse'`) dan ditulis audit `auth.session.reuse_detected`.
- **Penyimpanan hash saja (D-033):** yang disimpan pada `access_hash`, `refresh_hash`, dan `prev_refresh_hash` adalah **hash SHA-256** token. Token mentah **tidak pernah** ditulis ke database, log, atau berkas konfigurasi.
- **Pencabutan:** `POST /auth/logout` mencabut sesi itu (panggilan kedua tetap `204`); `DELETE /auth/sessions/{session_id}` mencabut per sesi oleh `admin`; penonaktifan akun mencabut **seluruh** sesi akun itu. Sesi yang sudah dicabut ditolak sebagai `SESSION_REVOKED`.
- **Penyimpanan klien:** penyimpanan aman perangkat (Android Keystore lewat penyimpanan aman Expo); **dilarang** di log, Git, atau berkas konfigurasi.
- **Re-autentikasi:** setelah sesi tidak berlaku lagi, pengguna **wajib** melakukan Google Sign-In lagi; Google Sign-In **tidak** dijanjikan berjalan offline.
- **Retensi:** baris kedaluwarsa dihapus oleh pemeliharaan setelah 7 hari, terpisah dari audit.

### 4.3 `roles`

Tabel acuan kecil agar nama peran tidak tersebar sebagai string bebas. **Tidak** menjadi sistem izin dinamis; ini hanya daftar sah beserta deskripsi.

| Kolom | Tipe | Null | Aturan |
|---|---|---|---|
| `name` | TEXT | tidak | PK; `admin`, `dosen`, `mahasiswa` |
| `description` | TEXT | tidak | Penjelasan singkat |
| `created_at` | TEXT | tidak | — |

- **Daftar peran manusia adalah `admin`, `dosen`, dan `mahasiswa`.** Peran lama `operator` **DIGANTIKAN** oleh `dosen` dengan catatan jujur: cakupan `operator` lama **terbagi** antara `admin` dan `dosen`, sehingga ini **bukan** penggantian nama dengan cakupan identik (`docs/AUTHORIZATION.md`).
- **`device` bukan peran manusia** dan karena itu **tidak** ada pada tabel ini (DIGANTIKAN dari daftar lama `operator`, `admin`, `device`). Identitas mesin perangkat tetap terpisah dan tersimpan sebagai `devices.token_hash` (§4.13); alur autentikasi perangkat bukan Google Sign-In. `ai-engine` juga bukan peran di tabel ini (proses lokal dengan token internal loopback).
- Baris acuan ditulis oleh **migrasi baru** (FL-011), bukan dengan mengubah migrasi yang sudah dijalankan (§4.18). `users.role` tetap divalidasi `CHECK`, dan tidak ada tabel relasi izin-detail pada MVP.

### 4.4 `people`

Peserta absensi. Identitas biometrik **tidak** ada di tabel ini.

| Kolom | Tipe | Null | Aturan |
|---|---|---|---|
| `id` | TEXT | tidak | PK |
| `external_code` | TEXT | tidak | Unik; 1–32 karakter; **bukan NIK asli** untuk data demo |
| `display_name` | TEXT | tidak | 1–120 karakter |
| `status` | TEXT | tidak | `CHECK (status IN ('active','inactive'))`, bawaan `active` |
| `note` | TEXT | ya | Maksimum 200 karakter |
| `account_user_id` | TEXT | ya | FK → `users.id` `ON DELETE SET NULL`; **unik** bila tidak null — menautkan baris peserta ke akun `mahasiswa` |
| `created_at` | TEXT | tidak | — |
| `updated_at` | TEXT | ya | — |
| `deleted_at` | TEXT | ya | Penghapusan lunak, lihat §6.3 |

- **Unik:** `external_code` (tanpa memandang `deleted_at`; kode tidak boleh dipakai ulang agar audit lama tetap bermakna).
- **Unik:** `account_user_id` bila tidak null — satu akun `mahasiswa` paling banyak menautkan **satu** baris peserta, dan satu baris peserta paling banyak memiliki **satu** akun. Dijaga oleh indeks unik parsial.
- **Indeks:** `idx_people_status`, `idx_people_display_name`, `idx_people_account_user`.
- **Penautan akun mahasiswa:** `account_user_id` diisi oleh `admin` pada tindakan penautan eksplisit, **bukan** otomatis saat login. Kandidat yang sah hanyalah akun berperan `mahasiswa`; akun `admin`/`dosen` tidak ditautkan ke baris peserta. Penautan dan pelepasan penautan dicatat di audit (`person.account_linked`, `person.account_unlinked`). Selama `account_user_id` kosong, endpoint `/me/*` milik akun itu ditolak `409 ACCOUNT_NOT_LINKED`.
- **Penegasan biometrik (FL-010U):** `account_user_id` **hanya** menghubungkan akun manusia ke baris peserta. Kolom ini **tidak** membuka akses ke `face_templates`, tidak menambah kolom biometrik, dan tidak memberi mahasiswa akses ke data peserta lain. Aturan kunci dan biometrik (D-030) tetap berlaku apa adanya (§11).
- **Penghapusan:** `DELETE /people/{id}` menghapus `face_templates` terkait pada transaksi yang sama (D-015), lalu mengisi `deleted_at`. Baris tidak hilang agar `attendance_records` historis tetap dapat dibaca. Penghapusan akun `users` **tidak** menghapus baris peserta; `ON DELETE SET NULL` hanya mengosongkan penautan.

### 4.5 `classes`

| Kolom | Tipe | Null | Aturan |
|---|---|---|---|
| `id` | TEXT | tidak | PK |
| `code` | TEXT | tidak | Unik; 1–32 karakter |
| `title` | TEXT | tidak | 1–120 karakter |
| `created_at` | TEXT | tidak | — |
| `updated_at` | TEXT | ya | — |
| `deleted_at` | TEXT | ya | Penghapusan lunak |

- **Unik:** `code`.
- **Indeks:** `idx_classes_deleted`.

### 4.6 `class_memberships`

| Kolom | Tipe | Null | Aturan |
|---|---|---|---|
| `class_id` | TEXT | tidak | FK → `classes.id` `ON DELETE RESTRICT` |
| `person_id` | TEXT | tidak | FK → `people.id` `ON DELETE RESTRICT` |
| `created_at` | TEXT | tidak | — |

- **Primary key gabungan:** `(class_id, person_id)` — mencegah keanggotaan ganda tanpa perlu kolom `id`.
- **Indeks:** `idx_class_memberships_person`.
- **Penghapusan:** baris keanggotaan dihapus keras (bukan lunak) karena tidak memiliki makna historis sendiri; riwayat kehadiran tetap ada di `attendance_records`.

### 4.7 `attendance_sessions`

| Kolom | Tipe | Null | Aturan |
|---|---|---|---|
| `id` | TEXT | tidak | PK |
| `class_id` | TEXT | tidak | FK → `classes.id` `ON DELETE RESTRICT` |
| `device_id` | TEXT | ya | FK → `devices.id` `ON DELETE SET NULL` |
| `title` | TEXT | ya | 1–120 karakter |
| `status` | TEXT | tidak | `CHECK (status IN ('draft','open','closed'))`, bawaan `draft` |
| `opens_at` | TEXT | ya | Diset saat dibuka |
| `closes_at` | TEXT | tidak | Wajib; harus setelah `opens_at` |
| `late_after` | TEXT | ya | Wajib ≤ `closes_at` bila ada |
| `started_at` | TEXT | ya | Sama dengan `opens_at` saat dibuka, disimpan eksplisit untuk pembacaan rekap |
| `closed_at` | TEXT | ya | Diisi saat ditutup |
| `close_reason` | TEXT | ya | Maksimum 200 karakter |
| `created_by` | TEXT | tidak | FK → `users.id` `ON DELETE RESTRICT` |
| `created_at` | TEXT | tidak | — |
| `updated_at` | TEXT | ya | — |

- **CHECK tambahan:** `ck_sessions_time_order`: `closes_at > opens_at` bila `opens_at` tidak null, dan `late_after IS NULL OR late_after <= closes_at`.
- **Indeks:** `idx_sessions_class_status`, `idx_sessions_closes_at`, `idx_sessions_device`.
- **Aturan satu sesi terbuka per kelas:** dijaga di lapisan aplikasi **dan** oleh indeks unik parsial: `CREATE UNIQUE INDEX uq_sessions_one_open_per_class ON attendance_sessions(class_id) WHERE status = 'open';` SQLite mendukung indeks parsial; PostgreSQL juga. Ini lebih kuat daripada sekadar pemeriksaan aplikasi.
- **Aturan satu sesi terbuka per perangkat:** `CREATE UNIQUE INDEX uq_sessions_one_open_per_device ON attendance_sessions(device_id) WHERE status = 'open' AND device_id IS NOT NULL;`
- **Penghapusan:** sesi tidak dihapus keras bila sudah memiliki `attendance_records`; gunakan pembatalan di lapisan aplikasi (di luar MVP) atau biarkan tersimpan sebagai riwayat.
- **Retensi:** sesi dan rekapnya disimpan sesuai kebijakan retensi akademik kampus (belum diputuskan — lihat §13).

### 4.8 `attendance_records`

Tabel keputusan domain absensi. **Hanya ditulis oleh backend Go** (D-027).

| Kolom | Tipe | Null | Aturan |
|---|---|---|---|
| `id` | TEXT | tidak | PK |
| `session_id` | TEXT | tidak | FK → `attendance_sessions.id` `ON DELETE RESTRICT` |
| `person_id` | TEXT | tidak | FK → `people.id` `ON DELETE RESTRICT` |
| `status` | TEXT | tidak | `CHECK (status IN ('present','late','excused'))` |
| `source` | TEXT | tidak | `CHECK (source IN ('face','manual'))` |
| `checked_in_at` | TEXT | tidak | Waktu keputusan kehadiran |
| `created_by` | TEXT | ya | FK → `users.id` `ON DELETE RESTRICT`; wajib bila `source='manual'` |
| `reason` | TEXT | ya | Wajib bila `source='manual'`; maksimum 200 karakter |
| `match_score` | REAL | ya | Opsional, hanya untuk keputusan `face`; **bukan** data biometrik yang dapat direkonstruksi |
| `model_id` | TEXT | ya | Model yang dipakai saat keputusan `face` |
| `created_at` | TEXT | tidak | — |
| `updated_at` | TEXT | ya | Diperbarui saat koreksi manual |

- **Batasan unik penjaga terakhir:**
  `CREATE UNIQUE INDEX uq_attendance_session_person ON attendance_records(session_id, person_id);`
  Ini adalah **penjaga terakhir** terhadap absensi ganda. Pemeriksaan di aplikasi tetap dilakukan untuk pesan yang ramah, tetapi kebenaran data bergantung pada indeks ini, bukan pada pemeriksaan aplikasi.
- **Mengapa `(session_id, person_id)` dan bukan `(session_id, person_id, source)`:** satu peserta hanya boleh memiliki satu catatan per sesi. Koreksi manual **memperbarui** baris yang sama (mengubah `source` menjadi `manual` dan mengisi `reason`), bukan menambah baris kedua. Bila keputusan domain berubah sehingga peserta boleh punya dua catatan (misalnya check-in dan check-out), maka indeks ini harus diganti menjadi `(session_id, person_id, record_kind)` melalui migrasi baru — bukan dengan melemahkan indeks tanpa jejak.
- **CHECK konsistensi:** `ck_attendance_manual_fields`: `source <> 'manual' OR (created_by IS NOT NULL AND reason IS NOT NULL)`.
- **Indeks:** `idx_attendance_records_session_status`, `idx_attendance_records_person_created`.
- **Aturan `absent`:** status `absent` **tidak pernah** ditulis ke tabel ini. Nilai itu dihitung dari daftar anggota kelas dikurangi catatan yang ada, dan hanya bermakna setelah sesi `closed`. Karena tidak ada baris `absent`, penghapusan peserta yang tidak pernah hadir tidak menghasilkan catatan hantu.
- **Retensi:** selama masa retensi akademik; tidak ada kolom biometrik di sini sehingga ekspor relatif aman.

### 4.9 `consent_records`

| Kolom | Tipe | Null | Aturan |
|---|---|---|---|
| `id` | TEXT | tidak | PK |
| `person_id` | TEXT | tidak | FK → `people.id` `ON DELETE CASCADE` |
| `purpose` | TEXT | tidak | Misal `attendance_face_recognition` |
| `version` | TEXT | tidak | Versi teks persetujuan yang ditandatangani |
| `capture_method` | TEXT | tidak | `CHECK (capture_method IN ('written_form','digital_form'))` |
| `granted_at` | TEXT | tidak | — |
| `withdrawn_at` | TEXT | ya | Diisi saat pencabutan |
| `withdraw_reason` | TEXT | ya | Wajib bila `withdrawn_at` diisi |
| `recorded_by` | TEXT | tidak | FK → `users.id` `ON DELETE RESTRICT` |
| `created_at` | TEXT | tidak | — |

- **Aturan persetujuan aktif:** didefinisikan sebagai baris dengan `withdrawn_at IS NULL` dan `purpose = 'attendance_face_recognition'` untuk peserta tersebut.
- **Indeks unik parsial:** memastikan hanya satu persetujuan aktif per tujuan dan peserta:
  `CREATE UNIQUE INDEX uq_consent_active_per_purpose ON consent_records(person_id, purpose) WHERE withdrawn_at IS NULL;`
- **CHECK:** `ck_consent_withdraw_fields`: `withdrawn_at IS NULL OR withdraw_reason IS NOT NULL`.
- **Penghapusan:** tidak ada penghapusan keras lewat API. Persetujuan dicabut, bukan dihapus, karena jejak persetujuan adalah bukti kepatuhan.
- **Retensi:** wajib disimpan selama sistem menyimpan template terkait; setelah peserta dihapus, baris ikut terhapus melalui `ON DELETE CASCADE`.

### 4.10 `face_enrollments`

Sesi pendaftaran wajah berjangka waktu. Tidak menyimpan gambar.

| Kolom | Tipe | Null | Aturan |
|---|---|---|---|
| `id` | TEXT | tidak | PK |
| `person_id` | TEXT | tidak | FK → `people.id` `ON DELETE CASCADE` |
| `status` | TEXT | tidak | `CHECK (status IN ('pending','completed','cancelled','expired'))` |
| `accepted_samples` | INTEGER | tidak | Bawaan 0 |
| `samples_required_min` | INTEGER | tidak | Bawaan 3 |
| `expires_at` | TEXT | tidak | Bawaan `created_at + 10 menit` |
| `model_hint` | TEXT | ya | Saran model, boleh diabaikan |
| `started_by` | TEXT | tidak | FK → `users.id` `ON DELETE RESTRICT` |
| `created_at` | TEXT | tidak | — |
| `updated_at` | TEXT | ya | — |

- **Indeks:** `idx_face_enrollments_person_status`, `idx_face_enrollments_expires`.
- **CHECK:** `ck_enrollments_sample_count`: `accepted_samples >= 0`.
- **Penghapusan:** enrollment selesai dapat dihapus keras setelah template dibuat, karena datanya hanya bersifat proses.

### 4.11 `face_enrollment_samples`

Menyimpan **metadata kualitas**, bukan gambar dan bukan embedding.

| Kolom | Tipe | Null | Aturan |
|---|---|---|---|
| `id` | TEXT | tidak | PK |
| `enrollment_id` | TEXT | tidak | FK → `face_enrollments.id` `ON DELETE CASCADE` |
| `sample_index` | INTEGER | tidak | Dimulai dari 1 |
| `quality_status` | TEXT | tidak | `CHECK (quality_status IN ('accepted','rejected'))` |
| `quality_score` | REAL | ya | Skor kualitas dari mesin AI |
| `reject_reason` | TEXT | ya | Wajib bila ditolak: `NO_FACE`, `MULTIPLE_FACES`, `LOW_QUALITY`, `FACE_TOO_SMALL` |
| `created_at` | TEXT | tidak | — |

- **Unik:** `uq_enrollment_sample_index` pada `(enrollment_id, sample_index)`.
- **CHECK:** `ck_samples_reject_reason`: `quality_status <> 'rejected' OR reject_reason IS NOT NULL`.
- **Retensi:** dihapus saat enrollment selesai atau dibatalkan. **Tidak ada** kolom `image_blob` dan tidak boleh ditambahkan tanpa keputusan baru.

### 4.12 `face_templates`

Satu-satunya tempat data biometrik disimpan, dan hanya dalam bentuk terenkripsi (D-022).

| Kolom | Tipe | Null | Aturan |
|---|---|---|---|
| `id` | TEXT | tidak | PK |
| `person_id` | TEXT | tidak | FK → `people.id` `ON DELETE CASCADE` |
| `ciphertext` | BLOB | tidak | Embedding terenkripsi AES-256-GCM |
| `nonce` | BLOB | tidak | 12 byte, dari sumber acak kriptografis, **unik per baris** |
| `auth_tag` | BLOB | tidak | 16 byte tag autentikasi |
| `aad` | TEXT | tidak | Nilai AAD yang dipakai, untuk pemeriksaan ulang |
| `algorithm` | TEXT | tidak | `CHECK (algorithm = 'AES-256-GCM')` |
| `key_version` | TEXT | tidak | Menunjuk kunci enkripsi aktif saat baris ditulis. Versi pertama bernilai teks `'1'`; nilainya masuk ke AAD |
| `embedding_dim` | INTEGER | tidak | Dimensi vektor asli, untuk verifikasi setelah dekripsi |
| `model_id` | TEXT | tidak | Misal `sface` |
| `model_version` | TEXT | tidak | Versi bobot; template hanya dicocokkan dengan versi yang sama |
| `detector_id` | TEXT | tidak | Misal `yunet` |
| `created_at` | TEXT | tidak | — |
| `revoked_at` | TEXT | ya | Bila template dinonaktifkan tanpa penghapusan |

- **Unik:** `uq_face_templates_person_model` pada `(person_id, model_id, model_version)` — mencegah template ganda untuk model yang sama.
- **Indeks:** `idx_face_templates_person`, `idx_face_templates_key_version`.
- **AAD yang diikat (ditulis apa adanya pada kolom `aad` agar dapat diverifikasi ulang):** gabungan berurutan tanpa pemisah ambigu, misalnya `person_id || '|' || model_id || '|' || model_version || '|' || key_version`. Mengikat AAD ke identitas dan versi model membuat ciphertext tidak dapat dipindahkan ke orang lain atau dipakai dengan model lain tanpa gagal verifikasi.
- **Perilaku saat dekripsi gagal:** baris **tidak** dipakai dan **tidak** diganti dengan tebakan. Backend menandai kegagalan, menulis audit, dan mengembalikan hasil “tidak dapat diverifikasi” kepada pemanggil. Tidak ada kunci pengganti otomatis dan tidak ada pembuatan template baru secara diam-diam. Kunci yang hilang tidak menghasilkan template baru; peserta harus mendaftar ulang dengan persetujuan baru (`docs/SECURITY_PRIVACY.md` §5d). **Kegagalan dekripsi bukan alasan menghapus baris.** Tiga kondisi dibedakan (`docs/SECURITY_PRIVACY.md` §5d.1–§5d.2): kunci **sementara tidak tersedia** dan kunci **ada tetapi tidak dapat dipakai** **dilarang** memicu penghapusan; baris pada kondisi kedua dikarantina (tidak ditampilkan, tidak dihitung cocok) tetapi tetap ada. Penghapusan hanya dijalankan oleh manusia berwenang setelah kunci **dipastikan tidak dapat dipulihkan**, dan tidak pernah oleh tugas terjadwal otomatis.
- **Penghapusan:** pencabutan persetujuan dan penghapusan peserta **menghapus baris ini** (bukan menandai). Template tidak dapat dipulihkan setelah dihapus karena kunci terpisah tidak menyimpan salinan. Penghapusan biometrik **belum selesai** bila jejaknya masih ada di berkas `-wal`/`-shm`, salinan cadangan, atau lapisan cache (`docs/SECURITY_PRIVACY.md` §5c, D-030 konsekuensi butir 5).
- **Retensi:** selama persetujuan aktif. Bila persetujuan dicabut, baris harus terhapus pada transaksi yang sama.
- **Pengelolaan kunci (D-030, DISETUJUI pada FL-004S):** `key_version` diisi kunci aktif saat penulisan; versi pertama bernilai teks `'1'`. Kunci disimpan di luar database dan di luar Git, dibungkus DPAPI cakupan pengguna, dan hanya diakses lewat antarmuka `KeyProvider` di Go. Indeks `idx_face_templates_key_version` ada agar penghapusan kunci lama setelah rotasi dapat diperiksa terhadap baris yang menunjuk versinya. Kebijakan penuh: `docs/SECURITY_PRIVACY.md` §5a–§5f.
- **Peringatan lisensi model:** pemilihan `model_id`/`model_version` dibatasi pada bobot dengan ketentuan penggunaan yang sesuai (D-025). Jangan menambahkan model tanpa memeriksa lisensi.

### 4.13 `devices`

| Kolom | Tipe | Null | Aturan |
|---|---|---|---|
| `id` | TEXT | tidak | PK |
| `display_name` | TEXT | tidak | 1–120 karakter |
| `kind` | TEXT | tidak | `CHECK (kind IN ('webcam','esp32_cam'))` |
| `token_hash` | TEXT | tidak | Unik; hash token perangkat, bukan token mentah |
| `token_prefix` | TEXT | tidak | 4–8 karakter untuk identifikasi administratif |
| `revoked_at` | TEXT | ya | Pencabutan lunak |
| `last_seen_at` | TEXT | ya | Diperbarui pada percobaan pengenalan terakhir |
| `created_at` | TEXT | tidak | — |
| `updated_at` | TEXT | ya | — |

- **Indeks:** `idx_devices_revoked`.
- **Penghapusan:** `DELETE /devices/{id}` mengisi `revoked_at`, tidak menghapus baris, agar `attendance_sessions` lama tetap dapat ditelusuri.

### 4.14 `audit_events`

| Kolom | Tipe | Null | Aturan |
|---|---|---|---|
| `id` | TEXT | tidak | PK |
| `actor_type` | TEXT | tidak | `CHECK (actor_type IN ('user','device','system'))` |
| `actor_id` | TEXT | ya | UUID aktor; **tanpa** foreign key keras agar jejak audit tidak terhapus |
| `action` | TEXT | tidak | Misal `attendance.manual_corrected` |
| `entity_type` | TEXT | ya | Misal `attendance_record` |
| `entity_id` | TEXT | ya | UUID entitas |
| `reason` | TEXT | ya | Alasan manusia untuk tindakan sensitif |
| `safe_metadata` | TEXT | ya | JSON yang sudah dibersihkan; divalidasi sebelum ditulis |
| `correlation_id` | TEXT | ya | Penautan lintas permintaan |
| `created_at` | TEXT | tidak | — |

- **Indeks:** `idx_audit_events_action_time`, `idx_audit_events_entity`, `idx_audit_events_actor_time`.
- **Aturan keras `safe_metadata`:** dilarang memuat JPEG, embedding, token, hash, kata sandi, atau nomor identitas. Penulisan audit harus melalui fungsi pembantu yang memvalidasi kunci yang diizinkan, bukan JSON bebas.
- **Penghapusan:** baris audit tidak dihapus melalui API. Penghapusan hanya oleh proses pemeliharaan berbasis retensi yang disetujui, dan itu pun harus dicatat.
- **Retensi:** minimal selama masa retensi kehadiran agar koreksi manual tetap dapat dipertanggungjawabkan.

### 4.15 `idempotency_records`

| Kolom | Tipe | Null | Aturan |
|---|---|---|---|
| `key` | TEXT | tidak | PK gabungan bagian pertama; 8–128 karakter |
| `scope` | TEXT | tidak | PK gabungan bagian kedua: `operator:<user_id>` atau `device:<device_id>` |
| `request_hash` | TEXT | tidak | Hash isi permintaan (tanpa gambar mentah) |
| `response_code` | INTEGER | tidak | Kode status respons pertama |
| `response_body` | TEXT | ya | Badan respons ringkas yang aman disimpan ulang |
| `created_at` | TEXT | tidak | — |
| `expires_at` | TEXT | tidak | Bawaan `created_at + 24 jam` |

- **Primary key:** `(scope, key)` — kunci dari perangkat berbeda tidak boleh saling bertabrakan.
- **Catatan FL-010U (16 Oktober 2026):** literal awalan `operator:<user_id>` adalah bentuk yang tertulis pada kontrak yang sudah ada dan **belum** diselaraskan dengan model peran baru (`admin`/`dosen`/`mahasiswa`). Penamaan akhirnya dibiarkan apa adanya pada revisi dokumentasi ini; penyelarasannya harus diputuskan bersama `docs/API_CONTRACT.md` dan diterapkan pada migrasi FL-011. FL-010U tidak mengubah nilainya dan tidak menambah nilai baru.
- **Indeks:** `idx_idempotency_expires` untuk pembersihan berkala.
- **Penyimpanan gambar:** **tidak**. Yang disimpan hanya hash permintaan dan respons ringkas.
- **Retensi:** 24 jam, dihapus oleh proses pemeliharaan. Tabel ini **bukan** penjaga terakhir absensi ganda; penjaga itu ada di `attendance_records`.

### 4.16 `user_identities`

**Tabel baru pada model data target FL-010U (16 Oktober 2026); belum ada pada skema yang sudah dijalankan (§4.18).** Menautkan identitas dari penyedia autentikasi (Google) ke akun internal `users` (§4.1). Tabel ini **tidak** menyimpan ID token, kode otorisasi, atau kredensial apa pun.

| Kolom | Tipe | Null | Aturan |
|---|---|---|---|
| `id` | TEXT | tidak | PK |
| `user_id` | TEXT | tidak | FK → `users.id` `ON DELETE CASCADE` |
| `provider` | TEXT | tidak | `CHECK (provider = 'google')` |
| `subject` | TEXT | tidak | Klaim `sub` Google — **pengenal identitas yang stabil** |
| `email_at_provider` | TEXT | ya | Email yang dilaporkan penyedia saat penautan; **bukan** kunci identitas |
| `linked_at` | TEXT | tidak | Waktu penautan |
| `created_at` | TEXT | tidak | — |

- **Unik:** `(provider, subject)` — satu identitas penyedia paling banyak menautkan satu akun. Batasan ini adalah **penjaga** aturan “satu identitas Google tidak boleh menautkan beberapa akun”.
- **Unik:** `(user_id, provider)` — satu akun paling banyak memiliki satu identitas per penyedia. Bersama batasan di atas, pasangan ini mencegah dua akun berbagi satu identitas Google.
- **Indeks:** `idx_user_identities_user`.
- **Kunci pencarian saat login:** identitas dicari dengan kunci `(provider='google', subject=sub)`. **`email` bukan kunci identitas**; email hanya dipakai untuk mencocokkan akun `invited` pada penautan pertama, dan perbandingannya harus **sama persis** (huruf kecil, tanpa spasi tepi) disertai `email_verified=true`.
- **Aturan penautan:** penautan hanya terjadi bila (a) baris `user_identities` sudah ada untuk `sub` tersebut, atau (b) ada baris `users` berstatus `invited` dengan email identik. Selain itu **tidak ada akun yang dibuat** dan **tidak ada role yang ditetapkan**; permintaan ditolak dengan pesan generik `403 IDENTITY_NOT_LINKED` (anti-enumeration).
- **Penautan atomik dan tahan-race (D-032; FL-010V, 16 Oktober 2026):** penautan identitas Google dilakukan **di dalam satu transaksi** yang mencakup pencocokan akun `invited`, `INSERT` baris `user_identities`, dan perubahan `users.status` menjadi `active`. Bila dua permintaan `POST /auth/google` dengan `sub` yang sama (atau email undangan yang sama) masuk bersamaan, **hanya satu** yang boleh menautkan; permintaan yang kalah menerima konflik batasan unik dan **tidak** membuat baris kedua. Yang menjadi **penjaga terakhir adalah batasan unik** `(provider, subject)` dan `(user_id, provider)`, bukan pemeriksaan aplikasi: pemeriksaan di aplikasi hanya untuk pesan yang ramah. Hasilnya harus konsisten — tidak boleh ada keadaan “identitas tertaut tetapi akun masih `invited`”, dan tidak boleh ada dua baris identitas untuk satu `sub`.
- **Email hanya sekali pakai untuk penautan (D-032; FL-010V, 16 Oktober 2026):** kecocokan email **hanya** dipakai pada penautan **pertama**, yaitu selama belum ada baris `user_identities` untuk `sub` tersebut. Setelah identitas tertaut, **pencocokan email tidak pernah dipakai lagi**; login berikutnya memakai `sub`. Karena itu **perubahan alamat email di penyedia tidak pernah memindahkan akun**: mengubah email di Google tidak menautkan `sub` itu ke akun lain, dan tidak memindahkan akun yang sudah tertaut ke pemilik email baru. Ini yang menutup jalur account takeover lewat perubahan email. `email_at_provider` hanya catatan historis saat penautan dan **bukan** kunci identitas.
- **Satu identitas Google untuk satu akun (D-037; FL-010V, 16 Oktober 2026):** satu identitas Google **tidak boleh** ditautkan ke beberapa akun, dan **tidak boleh** menghasilkan beberapa sesi dengan role berbeda. Bila ternyata ada kebutuhan satu identitas dipakai di beberapa akun, itu **wajib** lewat **mekanisme resolusi yang dirancang dan disetujui lebih dahulu**; tanpa mekanisme itu, permintaan ditolak dan dicatat sebagai masalah terbuka (§13). Batasan unik di atas tetap berlaku apa pun keputusan itu nanti.
- **Role tidak pernah berasal dari Google.** Identitas penyedia hanya membuktikan siapa yang masuk; keputusan otorisasi tetap di database FaceLink AI.
- **Penghapusan akun:** baris identitas ikut terhapus lewat `ON DELETE CASCADE`. Pelepasan penautan identitas (tanpa menghapus akun) belum dirancang dan dicatat sebagai butir terbuka (§13).
- **Audit:** `auth.google.succeeded`, `auth.google.linked`, `auth.login.denied` (dengan `reason` ber-kode, **bukan** email), `user.invited`, `user.role_changed`, `user.status_changed`. `safe_metadata` **tidak pernah** memuat email, ID token, `sub`, atau token sesi.
- **Retensi:** selama akun `users` terkait ada.

### 4.17 `class_lecturers`

**Tabel baru pada model data target FL-010U (16 Oktober 2026); belum ada pada skema yang sudah dijalankan (§4.18).** Menyatakan dosen mana yang **ditugaskan** pada kelas mana. Inilah dasar cakupan `dosen`: ia hanya boleh mengelola kelas yang ditugaskan kepadanya (`docs/AUTHORIZATION.md`).

| Kolom | Tipe | Null | Aturan |
|---|---|---|---|
| `class_id` | TEXT | tidak | FK → `classes.id` `ON DELETE CASCADE`; bagian PK |
| `user_id` | TEXT | tidak | FK → `users.id` `ON DELETE RESTRICT`; bagian PK |
| `assigned_by` | TEXT | tidak | FK → `users.id` `ON DELETE RESTRICT`; admin yang menugaskan |
| `assigned_at` | TEXT | tidak | Waktu penugasan |

- **Primary key gabungan:** `(class_id, user_id)` — satu dosen paling banyak ditugaskan sekali pada sebuah kelas.
- **Indeks:** `idx_class_lecturers_user` — agar daftar kelas milik seorang dosen dapat dibaca cepat.
- **Keanggotaan:** hanya akun berperan `dosen` yang sah ditugaskan. `mahasiswa` tidak masuk tabel ini (keanggotaan peserta ada di `class_memberships` §4.6).
- **Penugasan hanya oleh `admin`**; penugasan dan pencabutan penugasan dicatat di audit (`class.dosen_assigned`, `class.dosen_removed`).
- **Penghapusan:** baris penugasan dihapus keras (tanpa makna historis sendiri) mengikuti pola `class_memberships`. Penghapusan kelas mengikuti `ON DELETE CASCADE`; penghapusan akun dosen **ditolak** selama masih ada penugasan (`RESTRICT`) agar kelas tidak kehilangan pengampu secara diam-diam.
- **Retensi:** selama kelas dan akun terkait ada.

### 4.18 Catatan selisih terhadap migrasi yang sudah dijalankan

**Bagian ini penting dan harus dibaca apa adanya.** Migrasi `backend/migrations/00001_init_core_schema.sql` **sudah dijalankan**, sehingga bentuk tabel yang benar-benar ada di database berbeda dari model data target pada §4.1–§4.4, §4.16, dan §4.17. Perbedaan yang diketahui:

| Tabel | Keadaan pada `00001_init_core_schema.sql` | Model data target (§4.1–§4.4, §4.16, §4.17) |
|---|---|---|
| `users` | `status` bernilai `active`/`disabled`; ada `deleted_at`; ada `password_hash`; `role` mengacu ke `roles(name)` dengan data `operator`/`admin` | `status` bernilai `invited`/`active`/`suspended`; **tanpa** `password_hash` dan `must_change_password`; tanpa `deleted_at`; tambah `created_by` |
| `people` | ada `consent_state` (`unknown`/`granted`/`withdrawn`/`expired`) | tanpa `consent_state` pada dokumen ini; tambah `account_user_id` |
| `operator_tokens` | bernama `operator_tokens`; berkolom `token_hash`/`label`/`last_used_at`/`expires_at` (tanpa pemisahan akses/penyegar) | bernama `session_tokens`; berkolom `access_hash`/`access_expires_at`/`refresh_hash`/`refresh_expires_at`/`absolute_expires_at`/`prev_refresh_hash`/`revoked_reason`/`last_seen_at` (§4.2) |
| `roles` | berisi `operator`, `admin`, `device` | berisi `admin`, `dosen`, `mahasiswa` |
| `classes` | ada kolom `status` (`active`/`archived`) dan `description` | dokumen ini belum menyatakan `status`/`description` pada §4.5 |
| `consent_records` | berisi `purpose`/`status`/`method`/`document_ref`/`notes` | dokumen ini memakai `purpose`/`version`/`capture_method`/`withdraw_reason` (§4.9) |
| `user_identities`, `class_lecturers` | **tidak ada** | tabel baru (§4.16, §4.17) |
| `auth_nonces` | **tidak ada** | tabel target baru FL-010V (§4.19); nonce sekali pakai untuk `POST /auth/google` |
| `face_templates`, `face_enrollment_samples` | **tidak dibuat** (sengaja) | tetap **belum dibuat**; kebijakan D-030 tetap berlaku apa adanya |

- **Tidak ada SQL, migrasi, atau kode yang ditulis pada FL-010U.** FL-010U adalah tahap dokumentasi.
- **FL-010V (16 Oktober 2026) menambah satu tabel target dan satu kolom target, tanpa SQL:** tabel `auth_nonces` (§4.19) dan kolom `session_tokens.absolute_expires_at` (§4.2). Keduanya **belum ada** pada `00001_init_core_schema.sql` dan wajib dibuat pada migrasi `00002+` FL-011. FL-010V tidak mengubah berkas migrasi apa pun dan tidak menjalankan migrasi.
- **Migrasi `00001_init_core_schema.sql` tidak boleh diubah.** Seluruh penyesuaian di atas wajib dituangkan sebagai **migrasi baru (`00002+`) pada tugas implementasi FL-011**, dengan konvensi penomoran §9.3 (nomor tidak pernah dipakai ulang) dan pola *buat tabel baru → salin → tukar nama* untuk bentuk `ALTER TABLE` yang tidak didukung SQLite (§9.3).
- Sebelum migrasi yang mengubah atau menghapus kolom, cadangan harus dibuat mengikuti §7.3; rollback destruktif **dilarang** (§9.5).
- Selisih di atas adalah **usulan model target**, bukan keadaan terverifikasi. Semua pernyataan runtime pada dokumen ini berstatus **BELUM TERVERIFIKASI**.

### 4.19 `auth_nonces`

**Tabel target baru FL-010V (16 Oktober 2026); belum ada pada skema yang sudah dijalankan (§4.18).** Menyimpan nonce sekali pakai untuk mencegah **replay** pada pertukaran identitas `POST /auth/google`. Tabel ini **tidak** menyimpan ID token, kode otorisasi, `sub`, email, atau kredensial apa pun; yang disimpan hanyalah **hash** nonce.

| Kolom | Tipe | Null | Aturan |
|---|---|---|---|
| `nonce_hash` | TEXT | tidak | PK; hash SHA-256 nonce yang dikirim klien; nonce mentah tidak pernah disimpan |
| `created_at` | TEXT | tidak | Waktu nonce dicatat |
| `expires_at` | TEXT | tidak | Wajib; `created_at` + **5 menit** |

- **Primary key:** `nonce_hash` — satu nonce hanya boleh ada satu kali. Inilah penjaga terakhir aturan sekali pakai, bukan pemeriksaan aplikasi.
- **Indeks:** `idx_auth_nonces_expires` untuk pembersihan berkala.
- **Alur sekali pakai (FL-010V):** aplikasi menghasilkan nonce acak **per percobaan login** dan mengirimkannya ke Google **dan** ke backend. Backend memverifikasi kecocokan nonce, memastikan `now < expires_at`, lalu **menghapus/menandai** baris itu **di dalam transaksi yang sama** dengan pemrosesan login. Nonce yang **sudah dipakai** ditolak; karena penghapusan bersifat atomik dan `nonce_hash` adalah primary key, dua permintaan bersamaan dengan nonce yang sama hanya boleh berhasil satu kali.
- **TTL 5 menit:** nonce yang lebih tua dari `expires_at` **ditolak** dan tidak boleh dipakai, meskipun barisnya belum dibersihkan.
- **Pembersihan:** baris kedaluwarsa dihapus oleh **proses pemeliharaan** (sama pola dengan `idempotency_records`, §4.15 dan §7.2), terpisah dari audit. Pemeliharaan bukan penjaga keamanan; penjaga keamanannya adalah pemeriksaan `expires_at` dan aturan sekali pakai.
- **Audit:** kegagalan nonce (tidak cocok, kedaluwarsa, atau dipakai ulang) tercatat sebagai `auth.login.denied` dengan `reason` ber-kode. `safe_metadata` **tidak pernah** memuat nonce mentah, ID token, `sub`, atau email.
- **Retensi:** baris dihapus segera setelah dipakai atau setelah `expires_at` terlewati; tidak ada riwayat jangka panjang pada tabel ini.

## 5. Alur transaksi penting

### 5.1 Pencatatan kehadiran dari kamera (`POST /devices/{id}/recognition-attempts`)

Satu transaksi tulis, urutan tetap, satu penulis (Go):

1. `BEGIN IMMEDIATE` — meminta kunci tulis sejak awal agar konflik terdeteksi sebelum pemeriksaan panjang, bukan setelah.
2. Terapkan idempotensi: `SELECT` pada `idempotency_records` dengan `(scope, key)`. Bila ada dan `request_hash` cocok, `COMMIT` tanpa perubahan dan kembalikan respons tersimpan.
3. Baca sesi dari `attendance_sessions` **di dalam** transaksi: harus `open`, `closes_at` belum lewat.
4. Panggil mesin AI (di luar transaksi bila memungkinkan untuk memperpendek durasi kunci; hasilnya kemudian divalidasi ulang di dalam transaksi pada langkah 5).
5. Validasi ulang keadaan setelah panggilan AI: sesi masih `open`, peserta masih memiliki persetujuan aktif, peserta masih anggota kelas, perangkat belum dicabut.
6. Tentukan `status` = `present` atau `late` dari `late_after`.
7. `INSERT INTO attendance_records ... ON CONFLICT (session_id, person_id) DO NOTHING`.
8. Bila `changes() = 0`, artinya sudah ada catatan: baca baris yang ada, kembalikan `outcome = already_recorded` dengan `signal = green`, **tanpa** memperbarui `checked_in_at`.
9. `INSERT INTO audit_events` dengan aksi `attendance.recorded`.
10. `INSERT INTO idempotency_records` (respons ringkas).
11. `COMMIT`.

Aturan tambahan: panggilan mesin AI tidak boleh menahan transaksi tulis lebih lama dari yang diperlukan; bila implementasi akhirnya memanggil AI di dalam transaksi, maka `busy_timeout` harus memperhitungkan durasi itu, dan ini harus diukur, bukan diasumsikan.

### 5.2 Koreksi manual

1. `BEGIN IMMEDIATE`.
2. Periksa peserta adalah anggota kelas sesi, dan sesi ada (boleh `closed`).
3. `INSERT ... ON CONFLICT (session_id, person_id) DO UPDATE SET status=excluded.status, source='manual', created_by=..., reason=..., updated_at=...`.
4. Tulis `audit_events` `attendance.manual_corrected` beserta nilai sebelumnya dan sesudahnya.
5. `COMMIT`.

### 5.3 Pencabutan persetujuan

1. `BEGIN IMMEDIATE`.
2. `UPDATE consent_records SET withdrawn_at=..., withdraw_reason=... WHERE ... AND withdrawn_at IS NULL`. Bila tidak ada baris yang berubah, `ROLLBACK` dan balas `409`.
3. `DELETE FROM face_templates WHERE person_id = ...`.
4. Tulis `audit_events` `consent.withdrawn` beserta jumlah template yang terhapus.
5. `COMMIT`.

Ketiga operasi ini harus berada dalam **satu** transaksi. Tidak boleh ada keadaan di mana persetujuan sudah dicabut tetapi template masih ada.

### 5.4 Penutupan sesi

1. `BEGIN IMMEDIATE`.
2. `UPDATE attendance_sessions SET status='closed', closed_at=... WHERE id=? AND status='open'`. Bila `changes() = 0`, `ROLLBACK` dan balas `409 SESSION_CLOSED`.
3. Tulis audit `session.closed`.
4. `COMMIT`.

## 6. Konkurensi, penguncian, dan pemulihan

### 6.1 Model penulisan SQLite

1. `PRAGMA journal_mode = WAL` — pembaca tidak memblokir penulis dan sebaliknya.
2. `PRAGMA foreign_keys = ON` — **wajib** dinyalakan per koneksi; SQLite mematikannya secara bawaan.
3. `PRAGMA busy_timeout = 5000` (kandidat awal, harus diukur) — agar penulis kedua menunggu, bukan langsung gagal.
4. `PRAGMA synchronous = NORMAL` dipilih untuk MVP: tahan terhadap kegagalan proses atau sistem, tetapi **tidak** sepenuhnya tahan terhadap mati listrik mendadak. Konsekuensinya harus dinyatakan jujur: pada pemadaman mendadak, transaksi terakhir yang sudah dilaporkan berhasil dapat hilang. Bila kebutuhan demo kampus menuntut ketahanan lebih tinggi, `synchronous = FULL` menjadi pilihan, dengan biaya kecepatan tulis. Perubahan ini adalah keputusan, bukan nilai bawaan yang boleh diubah tanpa catatan.
5. **Transaksi pendek**: tidak ada transaksi yang menunggu masukan pengguna, unggahan jaringan, atau panggilan mesin AI yang berdurasi panjang.
6. **Satu penulis**: hanya proses Go yang menulis. Mesin AI Python tidak pernah membuka database. Ini berlaku juga untuk skrip bantu: semua penulisan lewat aplikasi atau lewat migrasi terkontrol.
7. **Antrean penulisan aplikasi**: permintaan tulis dari HTTP boleh masuk bersamaan, tetapi pemakaian `BEGIN IMMEDIATE` membuat penulis kedua menunggu secara eksplisit sehingga konflik terdeteksi di awal transaksi.
8. `SQLITE_BUSY` yang melewati `busy_timeout` dipetakan ke `503 DATABASE_BUSY` — **tanpa** penulisan parsial, karena transaksi yang gagal di-rollback.

### 6.2 Penanganan kegagalan database terkunci

- Kesalahan `SQLITE_BUSY`/`SQLITE_LOCKED` **tidak** boleh diterjemahkan menjadi “kehadiran gagal dicatat”. Pesan ke perangkat tetap `retry` dengan `signal: red` bila berasal dari infrastruktur.
- Percobaan ulang di sisi server dibatasi satu kali untuk kesalahan penguncian, dengan jeda pendek acak (puluhan milidetik), bukan pengulangan tanpa batas.
- Bila penguncian berlanjut, itu pertanda proses lain memegang transaksi tulis terlalu lama; ini harus **diukur** dan dilaporkan, bukan disembunyikan dengan menaikkan `busy_timeout` tanpa batas.

### 6.3 Penghapusan dan kaskade

| Relasi | Aturan | Alasan |
|---|---|---|
| `session_tokens` → `users` | `ON DELETE CASCADE` | Token tanpa pemilik tidak bermakna |
| `user_identities` → `users` | `ON DELETE CASCADE` | Identitas penyedia tidak bermakna tanpa akun |
| `people.account_user_id` → `users` | `ON DELETE SET NULL` | Penautan akun hilang, baris peserta tetap ada |
| `class_lecturers` → `classes` | `ON DELETE CASCADE` | Penugasan tidak bermakna tanpa kelas |
| `class_lecturers` → `users` | `ON DELETE RESTRICT` | Kelas tidak boleh kehilangan pengampu secara diam-diam |
| `face_templates` → `people` | `ON DELETE CASCADE` | Data biometrik wajib hilang bersama subjeknya |
| `face_enrollments` → `people` | `ON DELETE CASCADE` | Proses enrollment tidak bermakna tanpa peserta |
| `face_enrollment_samples` → `face_enrollments` | `ON DELETE CASCADE` | Metadata sampel mengikuti enrollment |
| `consent_records` → `people` | `ON DELETE CASCADE` | Jejak persetujuan mengikuti subjek sesuai D-015 |
| `attendance_records` → `people` | `ON DELETE RESTRICT` | Riwayat kehadiran tidak boleh hilang diam-diam |
| `attendance_records` → `attendance_sessions` | `ON DELETE RESTRICT` | Sesi dengan catatan tidak boleh dihapus |
| `class_memberships` → `classes`/`people` | `ON DELETE RESTRICT` | Keanggotaan harus dilepas sadar |
| `attendance_sessions` → `classes` | `ON DELETE RESTRICT` | Kelas dengan sesi tidak boleh hilang |
| `attendance_sessions` → `devices` | `ON DELETE SET NULL` | Perangkat dapat dicabut tanpa menghapus sesi |
| `audit_events` → apa pun | tanpa foreign key | Jejak audit tidak boleh terhapus oleh penghapusan entitas |

Karena `attendance_records.person_id` memakai `RESTRICT`, penghapusan peserta yang memiliki catatan kehadiran **tidak dapat** dilakukan sebelum kebijakan retensi diputuskan. API harus mengembalikan `409 POLICY_CONFLICT` dan menjelaskan alasannya, bukan menghapus riwayat.

## 7. Strategi SQLite WAL

### 7.1 Pengaturan wajib

| Pengaturan | Nilai | Alasan |
|---|---|---|
| `journal_mode` | `WAL` | Pembaca tidak memblokir penulis; penting karena rekap dibaca sambil sesi berjalan |
| `foreign_keys` | `ON` | SQLite mematikannya secara bawaan; tanpa ini seluruh aturan kaskade hanya dekorasi |
| `busy_timeout` | 5000 ms (kandidat) | Menahan penulis kedua sebentar, bukan langsung gagal |
| `synchronous` | `NORMAL` | Keseimbangan yang sadar antara kecepatan dan ketahanan |
| `wal_autocheckpoint` | 1000 halaman (bawaan) | Titik periksa otomatis |
| `temp_store` | `MEMORY` | Mengurangi penulisan berkas sementara |
| `mmap_size` | bawaan | Tidak diubah tanpa pengukuran |

Berkas yang menyertai database: `facelink.db`, `facelink.db-wal`, `facelink.db-shm`. Ketiganya harus diperlakukan sebagai satu kesatuan.

### 7.2 Perawatan

1. Titik periksa otomatis berjalan pada ambang halaman; titik periksa penuh manual dijalankan saat pemeliharaan dengan `PRAGMA wal_checkpoint(TRUNCATE)` setelah memastikan tidak ada penulis aktif.
2. Jalankan `PRAGMA optimize` secara berkala (misalnya saat penutupan aplikasi atau mingguan), bukan pada setiap permintaan.
3. Jangan menjalankan `VACUUM` saat sesi absensi berjalan; `VACUUM` menahan kunci tulis.
4. Pemantauan sederhana: ukuran `-wal` yang terus tumbuh tanpa mengecil berarti titik periksa tidak berjalan. Ini indikator yang harus diperiksa, bukan diabaikan.

### 7.3 Cadangan dan pemulihan

Prosedur cadangan yang benar untuk database WAL:

1. Hentikan penulisan (tutup aplikasi atau lewat mekanisme pemeliharaan) **atau** gunakan perintah cadangan aman.
2. Cara paling aman: `sqlite3 facelink.db ".backup 'cadangan/facelink-2026-10-15.db'"` — menyertakan isi WAL tanpa menyalin berkas secara manual.
3. Alternatif saat aplikasi berhenti: salin ketiga berkas (`db`, `-wal`, `-shm`) **setelah** titik periksa selesai.
4. Jangan pernah menyalin **hanya** `facelink.db` saat WAL berisi perubahan belum titik-periksa: hasilnya database yang kehilangan transaksi terakhir secara diam-diam.
5. Pemulihan: hentikan aplikasi, ganti ketiga berkas bersama-sama, jalankan `PRAGMA integrity_check`, lalu `PRAGMA foreign_key_check`.
6. Catat setiap pencadangan dan pemulihan pada `audit_events` dengan aksi `maintenance.backup`/`maintenance.restore` (tanpa jalur berkas sensitif bila jalur itu memuat nama pengguna Windows).

### 7.4 Larangan berkas jaringan

Database aktif **dilarang** diletakkan pada:

- berkas bersama jaringan (SMB/UNC, mis. `\\server\share\`),
- folder tersinkronisasi awan (OneDrive, Google Drive, Dropbox),
- drive yang dipetakan jaringan.

Alasan: penguncian berkas di atas protokol jaringan tidak andal, dan WAL membutuhkan memori terbagi yang tidak berfungsi lintas jaringan. Gejalanya adalah kerusakan database yang sulit direproduksi. Database harus berada di disk lokal. Berkas cadangan boleh disalin ke lokasi lain **setelah** dibuat, tetapi database yang sedang berjalan tidak boleh berada di sana.

## 8. Driver SQLite untuk Go

### 8.1 Pilihan

| Kandidat | Sifat | Catatan |
|---|---|---|
| `modernc.org/sqlite` | **Pure Go** (terjemahan otomatis) | Tidak butuh C compiler, mudah di Windows, silang-kompilasi paling sederhana |
| `mattn/go-sqlite3` | cgo | Membutuhkan toolchain C; riwayat panjang dan stabil, tetapi menambah beban penyiapan di Windows |
| `zombiezen.com/go/sqlite` | Pembungkus di atas `modernc.org/sqlite` | API lebih rendah, berguna bila kontrol pernyataan diinginkan |

**Rekomendasi:** mulai dengan **`modernc.org/sqlite`** karena tujuan utamanya adalah memindahkan beban penyiapan, bukan mengejar performa puncak. Menghindari cgo berarti mahasiswa tidak perlu memasang toolchain C, dan kompilasi silang tetap mungkin. Untuk demo satu kelas dengan puluhan peserta, perbedaan performa dengan cgo tidak relevan.

Catatan penting: bila ternyata muncul masalah performa atau perilaku yang hanya muncul pada driver pure-Go, penggantian driver adalah perubahan terbatas pada lapisan pembuka koneksi karena skema tidak bergantung driver. Itu alasan tambahan untuk tidak menulis kode yang bergantung pada keanehan satu driver.

### 8.2 Lisensi dan kompatibilitas

- `modernc.org/sqlite` berlisensi BSD-3-Clause — sesuai untuk proyek ini dan tidak menambahkan kewajiban yang bertentangan dengan D-017.
- `mattn/go-sqlite3` berlisensi MIT, tetapi ketergantungan kompilasinya yang perlu dipertimbangkan, bukan lisensinya.
- **Versi dependency tidak dikunci di dokumen ini.** Versi harus ditetapkan saat implementasi berdasarkan hasil `go get` yang nyata dan dicatat pada `go.mod`, bukan berdasarkan perkiraan. Mengunci versi tanpa menjalankan pemasangan adalah klaim tanpa bukti.

## 9. Migrasi berversi

### 9.1 Mengapa bukan Alembic

Backend adalah **Go** (D-010) dan Alembic adalah alat migrasi Python yang berorientasi pada SQLAlchemy. Memakainya berarti menambahkan runtime Python kedua dengan rantai ketergantungan sendiri hanya untuk mengubah skema, sementara mesin AI Python sudah ada tetapi tidak memiliki hak menulis database (D-027). Alembic **bukan** migrator utama (D-026).

### 9.2 Pilihan mekanisme

| Alat | Lisensi | Kelebihan | Kekurangan |
|---|---|---|---|
| `pressly/goose` | MIT (versi terbaru) | Dapat dipakai sebagai pustaka dan CLI; mendukung berkas SQL murni; adaptor untuk `database/sql` termasuk driver SQLite | Fitur bawaan lebih sederhana |
| `golang-migrate/migrate` | MIT | Mature, banyak sumber contoh, mendukung berkas SQL | Lebih berat; beberapa sumber daya lebih rumit untuk SQLite |

**Rekomendasi:** **goose** dijalankan sebagai pustaka di dalam proses Go saat startup (atau sebagai perintah terpisah), memakai **berkas SQL murni** agar migrasi tetap dapat dibaca tanpa memahami kode Go. Perhatikan bahwa lisensi dan versi goose harus diperiksa ulang saat implementasi; dokumen ini tidak mengunci versi.

### 9.3 Konvensi berkas

```text
migrations/
  00001_init_core_tables.sql
  00002_attendance_and_consent.sql
  00003_face_templates.sql
  00004_devices_and_audit.sql
  00005_indexes_concurrency.sql
```

- Nama: `NNNNN_deskripsi_singkat.sql`. Nomor berurutan, lima digit, tidak pernah dipakai ulang.
- Setiap migrasi **naik** wajib tersedia. Migrasi **turun** bersifat opsional dan **tidak boleh** menghapus data pada migrasi produksi; alasannya ada di §9.5.
- Satu berkas boleh berisi lebih dari satu pernyataan, tetapi harus dijalankan dalam satu transaksi bila pernyataan itu saling bergantung.
- SQLite tidak mendukung semua bentuk `ALTER TABLE`; migrasi yang rumit dilakukan dengan pola *buat tabel baru → salin → tukar nama*, dan itu harus ditulis eksplisit di berkas migrasi.

### 9.4 Versi skema dan penerapan

1. Versi skema disimpan pada tabel internal migrator (bukan tabel domain), dan tidak diekspos ke API klien.
2. Penerapan hanya dari **satu** proses saat startup, dilindungi kunci proses. Menjalankan dua migrasi bersamaan adalah kesalahan serius.
3. Bila migrasi gagal, status dicatat pada `audit_events` (`migration.failed`) dan aplikasi **menolak melayani permintaan tulis** sampai keadaannya jelas. Melayani tulisan dengan skema setengah jadi adalah sumber kerusakan data.
4. Sebelum migrasi yang mengubah atau menghapus kolom: buat cadangan dengan prosedur §7.3 terlebih dahulu.
5. Versi skema ditampilkan pada `GET /readyz` sebagai `migrations: applied`.

### 9.5 Larangan rollback destruktif

Rollback yang menjatuhkan kolom atau tabel **dilarang** pada database yang pernah dipakai. Bila sebuah migrasi perlu diurungkan, buat migrasi maju baru yang memperbaiki keadaan. Alasannya: rollback diasumsikan tidak berbahaya padahal dapat menghapus data kehadiran atau template terenkripsi tanpa jejak. Pengecualian hanya untuk database pengembangan yang belum pernah menerima data nyata, dan itu pun harus dinyatakan eksplisit.

### 9.6 Portabilitas ke PostgreSQL

**Migrasi SQLite tidak dijamin berjalan identik di PostgreSQL.** Perbedaan yang harus diperhitungkan:

| Aspek | SQLite | PostgreSQL |
|---|---|---|
| Jenis data | `TEXT`/`INTEGER`/`REAL`/`BLOB` | `UUID`/`BOOLEAN`/`TIMESTAMPTZ`/`BYTEA`/`JSONB` |
| Waktu sekarang | `CURRENT_TIMESTAMP` | `now()` |
| Naikkan nilai | `INSERT OR REPLACE` | `INSERT ... ON CONFLICT ... DO UPDATE` |
| Abaikan konflik | `INSERT OR IGNORE` | `INSERT ... ON CONFLICT DO NOTHING` |
| Indeks parsial | Didukung | Didukung |
| Ubah kolom | Terbatas | `ALTER TABLE ... ALTER COLUMN` |
| Kunci tulis | Satu penulis | MVCC penuh |

Strategi:

1. Tulis migrasi dengan **SQL yang dapat dipahami keduanya** bila mungkin, dan tandai pernyataan yang khusus SQLite.
2. Untuk perpindahan nyata, siapkan dua rangkaian migrasi atau rangkaian migrasi khusus PostgreSQL. Berkas migrasi SQLite yang sudah ada **tidak diubah**; berkas baru dibuat untuk PostgreSQL.
3. Data dipindahkan dengan ekspor-impor terkontrol, dan kolom terenkripsi dipindahkan **apa adanya** karena enkripsi tidak bergantung pada mesin database. Kunci tetap terpisah.
4. Perpindahan dilakukan saat tidak ada sesi berjalan, dengan cadangan penuh sebelum dan sesudah.
5. Jangan mengklaim skema saat ini sudah “siap PostgreSQL”. Yang benar: skema **dirancang agar portabel**, dan perpindahan tetap memerlukan pekerjaan migrasi nyata.

## 10. Indeks dan kinerja yang harus dipantau

1. Indeks minimum yang wajib ada tercantum pada setiap tabel. Indeks tambahan hanya ditambahkan berdasarkan keluhan kinerja yang terukur, bukan dugaan.
2. Permintaan yang paling sering: rekap sesi (`session_id`), pencarian peserta (`external_code`), daftar anggota kelas (`class_id`), dan percobaan pengenalan terakhir per perangkat.
3. `EXPLAIN QUERY PLAN` dipakai sebelum menambah indeks.
4. Batas nyata: satu kelas berisi puluhan peserta; bila ternyata tumbuh jauh lebih besar, ukuran halaman dan indeks harus ditinjau ulang dengan data nyata.

## 11. Keamanan data pada tingkat database

1. Berkas database harus berada di direktori dengan ACL terbatas pada akun pengguna yang menjalankan backend. SQLite tidak memiliki pengguna internal; keamanan berkasnya bergantung pada sistem berkas.
2. Enkripsi tingkat aplikasi (D-022) melindungi template biometrik meskipun berkas database tersalin. **Tidak** ada klaim bahwa seluruh isi database terenkripsi: rekap kehadiran dan nama peserta tetap terbaca oleh siapa pun yang memiliki berkasnya. Ini harus dinyatakan jujur kepada pemilik proyek.
3. Bila kelak seluruh berkas perlu terenkripsi, itu keputusan baru (misalnya SQLCipher), bukan asumsi implisit dari dokumen ini.
4. Jangan menyimpan kunci enkripsi di dalam database, di dalam repositori, atau di dalam variabel lingkungan yang tercatat pada berkas konfigurasi yang ikut ter-commit.
5. Kunci enkripsi berada di luar berkas database, sehingga **menyalin `facelink.db`, `-wal`, dan `-shm` saja tidak pernah cukup** untuk memulihkan template. Pencadangan (`docs/DATA_MODEL.md` §7.3) harus diperlakukan sebagai satu kesatuan: tiga berkas database **plus** berkas kunci terlindungi. Cadangan yang tidak memuat kunci harus dijelaskan sebagai cadangan kehadiran, bukan cadangan biometrik.
6. `PRAGMA wal_checkpoint(TRUNCATE)` dan `VACUUM` **tidak** bergantung pada kunci dan tidak mengubah isi kolom `ciphertext`. Keduanya tidak dapat memulihkan template yang kuncinya sudah hilang, dan tidak boleh diusulkan sebagai cara pemulihan.
7. Kebijakan kunci lengkap (lokasi, perlindungan, rotasi, pemulihan, dan batas jaminan penghapusan) ada di `docs/SECURITY_PRIVACY.md` §5a–§5f sebagai **D-030 yang sudah disetujui pada FL-004S**. Meskipun begitu, sampai ada tugas implementasi tersendiri yang disetujui pemilik, tabel ini **tidak boleh** ditulis oleh kode apa pun — persetujuan berlaku atas rancangan, bukan izin menulis kode.
8. **FL-010U (16 Oktober 2026) tidak mengubah aturan kunci atau biometrik apa pun.** D-030, kebijakan `KeyProvider`, AES-256-GCM, AAD, dan seluruh batasan DPAPI tetap berlaku **apa adanya**; revisi tiga peran + Google Sign-In tidak menyentuh kebijakan itu.
9. **`people.account_user_id` bukan jalur ke template.** Menautkan akun `mahasiswa` ke baris peserta (§4.4) tidak membuat `face_templates` dapat dibaca melalui akun itu, tidak menambah kolom biometrik, dan tidak memperluas akses ke data peserta lain. Enkripsi template tetap satu-satunya perlindungannya (butir 2).

## 12. Pengujian basis data yang harus ada saat implementasi

**Belum ada pengujian yang dijalankan pada FL-003** karena belum ada kode aplikasi. Daftar berikut adalah syarat penerimaan untuk tugas implementasi.

| # | Skenario | Harapan |
|---|---|---|
| 1 | Dua `INSERT` ke `attendance_records` dengan `(session_id, person_id)` sama | Yang kedua gagal pada indeks unik |
| 2 | Dua sesi `open` untuk kelas yang sama | Indeks unik parsial menolak yang kedua |
| 3 | Dua sesi `open` untuk perangkat yang sama | Ditolak |
| 4 | Cabut persetujuan ketika template ada | Dalam satu transaksi: `withdrawn_at` terisi **dan** baris `face_templates` hilang |
| 5 | Gagal di tengah transaksi pencabutan | Tidak ada keadaan “persetujuan dicabut tetapi template masih ada” |
| 6 | Hapus peserta yang memiliki catatan kehadiran | `409 POLICY_CONFLICT`, tidak ada data hilang |
| 7 | `PRAGMA foreign_keys` tidak dinyalakan | Pengujian harus **mendeteksi** ini sebagai kesalahan konfigurasi, bukan lulus diam-diam |
| 8 | Buka database dengan koneksi yang salah mengaktifkan `foreign_keys` | Kaskade tidak berjalan; harus terdeteksi oleh pengujian, bukan oleh pengguna |
| 9 | Penulis kedua saat penulis pertama menahan transaksi | Menunggu hingga `busy_timeout`, lalu `503 DATABASE_BUSY` bila tetap terkunci |
| 10 | Kiriman ulang setelah mati listrik dengan `synchronous=NORMAL` | Transaksi terakhir dapat hilang — perilaku ini **diuji dan didokumentasikan**, bukan diasumsikan aman |
| 11 | Cadangan dengan `.backup` saat WAL berisi perubahan | Pemulihan memuat seluruh transaksi |
| 12 | Salin hanya `facelink.db` tanpa `-wal` | Pengujian menunjukkan kehilangan data agar prosedur salah tidak dipakai |
| 13 | Terapkan migrasi dua kali | Yang kedua tidak mengubah apa pun dan tidak gagal |
| 14 | Migrasi gagal di tengah | Versi skema tidak naik; aplikasi menolak melayani tulisan |
| 15 | `DELETE` pada `face_templates` lalu `SELECT` | Tidak ada sisa data embedding |
| 16 | Dekripsi dengan kunci berbeda | Gagal verifikasi; tidak ada penggantian kunci otomatis |
| 17 | `wal_checkpoint(TRUNCATE)` setelah semua penulis berhenti | Ukuran `-wal` menyusut |
| 18 | Database diletakkan di folder tersinkronisasi awan | Ditolak atau diberi peringatan keras saat inisialisasi |

Pengujian tambahan untuk identitas Google, sesi aplikasi, dan penautan akun (FL-010U, 16 Oktober 2026). Semuanya berstatus **BELUM TERVERIFIKASI**; daftar ini adalah syarat penerimaan untuk tugas implementasi, bukan laporan hasil.

| # | Skenario | Harapan |
|---|---|---|
| 19 | Penautan pertama: `sub` Google baru, ada baris `users` berstatus `invited` dengan email **sama persis** dan `email_verified=true` | Satu baris `user_identities` dibuat; `users.status` menjadi `active`; audit `auth.google.linked` |
| 20 | `sub` Google baru, **tidak** ada baris `users` yang cocok | **Tidak ada akun dibuat**, **tidak ada role ditetapkan**; ditolak `403 IDENTITY_NOT_LINKED` dengan pesan generik |
| 21 | `sub` Google baru, email cocok tetapi `email_verified=false` | Ditolak; tidak ada penautan dan tidak ada akun baru |
| 22 | `sub` Google baru, email cocok kecuali beda huruf besar/kecil atau ada spasi tepi | **Tidak** cocok; ditolak (perbandingan sama persis setelah huruf kecil dan pemangkasan) |
| 23 | Email cocok dengan akun berstatus `suspended` | Ditolak dengan pesan yang **sama** seperti kasus tanpa akun (anti-enumeration) |
| 24 | Dua baris `user_identities` dengan `(provider, subject)` sama | Yang kedua ditolak indeks unik |
| 25 | Dua baris `user_identities` dengan `(user_id, provider)` sama | Yang kedua ditolak indeks unik |
| 26 | `POST /auth/refresh` dipakai dua kali dengan penyegar **yang sama** | Percobaan kedua memicu pencabutan **seluruh sesi** dan audit `auth.session.reuse_detected` |
| 27 | Token akses dipakai setelah `POST /auth/logout` | Ditolak `401 SESSION_REVOKED`; panggilan logout kedua tetap `204` |
| 28 | Akun `dosen` diturunkan menjadi `mahasiswa` saat permintaan sedang berjalan | Permintaan **berikutnya** memakai role baru; token tidak memuat klaim role |
| 29 | Akun dinonaktifkan (`suspended`) saat sesi masih berlaku | **Seluruh** sesi akun itu tercabut; permintaan berikutnya ditolak `401 SESSION_REVOKED` |
| 30 | Dua baris `people` ditautkan ke `account_user_id` yang sama | Yang kedua ditolak indeks unik parsial |
| 31 | Penghapusan keras baris `users` yang ditautkan (uji tingkat skema; kebijakan normal memakai `status = 'suspended'`) | `people.account_user_id` menjadi `NULL`; baris peserta dan `attendance_records` historis **tetap ada** |
| 32 | Akun `mahasiswa` tanpa penautan memanggil `GET /me/attendance` | Ditolak `409 ACCOUNT_NOT_LINKED` |
| 33 | Akun `dosen` mengakses kelas yang **tidak** ditugaskan padanya | Ditolak `404 NOT_FOUND` (deny by default) |
| 34 | Penugasan dosen ganda pada `(class_id, user_id)` yang sama | Yang kedua ditolak primary key gabungan |
| 35 | Akun `dosen` dihapus saat masih punya penugasan `class_lecturers` | Ditolak `RESTRICT`; penugasan harus dicabut lebih dahulu |
| 36 | Migrasi `00002+` dijalankan di atas database yang sudah menjalankan `00001_init_core_schema.sql` | Berhasil **tanpa** mengubah berkas `00001`; isi `users`/`people` lama tetap terbaca |
| 37 | Kolom `password_hash` masih dirujuk kode setelah migrasi FL-011 | Terdeteksi sebagai kegagalan (tidak ada kolom kata sandi pada model target) |

Pengujian tambahan FL-010V (16 Oktober 2026) untuk penautan atomik, nonce sekali pakai, dan batas absolut sesi. Semuanya berstatus **BELUM TERVERIFIKASI**; daftar ini adalah syarat penerimaan untuk tugas implementasi FL-011, bukan laporan hasil.

| # | Skenario | Harapan |
|---|---|---|
| 38 | Dua `POST /auth/google` bersamaan dengan `sub` yang sama dan satu akun `invited` yang cocok (uji penautan bersamaan, race detector **tidak** dimatikan) | **Hanya satu** permintaan menautkan; tepat **satu** baris `user_identities` untuk `sub` itu; akun menjadi `active` satu kali; permintaan lain ditolak konflik unik tanpa membuat baris kedua |
| 39 | Dua `POST /auth/google` bersamaan dengan email undangan yang sama tetapi `sub` berbeda | Paling banyak **satu** penautan berhasil; tidak ada dua akun berbagi satu identitas dan tidak ada dua identitas pada satu akun |
| 40 | Nonce yang **sudah dipakai** dikirim ulang pada `POST /auth/google` | Ditolak (pelanggaran primary key `auth_nonces.nonce_hash`); tidak ada sesi diterbitkan; audit `auth.login.denied` |
| 41 | Nonce dengan `expires_at` sudah terlewati (lebih dari 5 menit) | Ditolak; tidak ada sesi diterbitkan; nonce tidak dapat dipakai lagi |
| 42 | Dua permintaan bersamaan memakai nonce yang sama | Hanya **satu** yang berhasil; yang kedua ditolak |
| 43 | `POST /auth/refresh` berulang selama lebih dari 30 hari sejak `created_at` sesi | Penyegaran **ditolak** setelah `now >= absolute_expires_at`; sesi dicabut; token baru **tidak** diterbitkan dan `absolute_expires_at` **tidak** bergeser |
| 44 | Rotasi penyegar berkali-kali dalam masa berlaku | `absolute_expires_at` **tetap** sama seperti saat baris sesi dibuat; `refresh_expires_at` tidak pernah melampauinya |
| 45 | Google Sign-In ulang yang sah setelah batas absolut terlewati | Baris sesi **baru** dibuat dengan `created_at` dan `absolute_expires_at` baru; baris lama tidak diperpanjang |
| 46 | Satu identitas Google (`sub` yang sama) dicoba ditautkan ke akun kedua | Ditolak oleh batasan unik `(provider, subject)`; tidak ada mekanisme resolusi otomatis |
| 47 | Alamat email di penyedia diubah setelah identitas tertaut, lalu login lagi | Akun **tetap** sama dan **tidak** berpindah; pencocokan email tidak dipakai lagi; tidak ada penautan baru |
| 48 | Alamat email di penyedia diubah sebelum identitas tertaut (akun `invited` dengan email lama) | Penautan **tidak** terjadi karena email tidak lagi cocok; tidak ada akun baru; ditolak dengan pesan generik |
| 49 | Dua akun mencoba memakai identitas Google yang sama | Ditolak oleh `(user_id, provider)`/`(provider, subject)`; satu identitas hanya boleh menghasilkan **satu** sesi role |
| 50 | Bootstrap admin dijalankan saat sudah ada akun `admin` berstatus `active` atau `invited` | Proses **menolak berjalan**; tidak ada baris `users` baru; tidak ada kredensial tertanam yang dipakai |
| 51 | Baris `users` hasil bootstrap diperiksa | `role='admin'`, `status='invited'`, **`created_by IS NULL`**; akun menjadi `active` hanya setelah penautan Google pertama; audit `user.bootstrapped` |
| 52 | Akun dengan `status='invited'` (belum tertaut) memanggil endpoint terproteksi | Ditolak dengan pesan generik yang **sama** seperti akun `suspended` (anti-enumeration) |
| 53 | Baris `auth_nonces` yang sudah lewat `expires_at` dibiarkan, lalu pemeliharaan dijalankan | Baris terhapus oleh pemeliharaan; penghapusan **tidak** menjadi syarat keamanan karena pemeriksaan `expires_at` sudah menolaknya |

## 13. Pertanyaan terbuka dan hal yang belum diputuskan

1. **Kebijakan retensi kehadiran** dan hubungannya dengan penghapusan peserta belum ditetapkan (lihat `docs/OPEN_QUESTIONS.md`). Ini yang menghambat `DELETE /people` karena `attendance_records` memakai `RESTRICT`.
2. **`synchronous`** untuk kebutuhan demo: `NORMAL` dipilih sebagai kandidat, tetapi bila ketahanan data lebih penting daripada kecepatan, `FULL` adalah pilihan yang sah dan harus diputuskan sadar.
3. **Angka `busy_timeout`** dan batas laju belum diukur (Q-18).
4. **Ambang dan margin pencocokan** tidak berada di database; ia milik backend Go dan harus terkalibrasi (Q-07).
5. **Apakah peserta boleh memiliki lebih dari satu template per model** (misalnya dengan dan tanpa kacamata) belum diputuskan; skema saat ini mengizinkan satu per `(person_id, model_id, model_version)`.
6. **Enkripsi seluruh berkas database** (mis. SQLCipher) bukan bagian dari MVP dan belum diputuskan.
7. **Tabel `notifications` atau antrean pesan** tidak ada dan tidak dibutuhkan selama D-021 berlaku (polling, bukan WebSocket).
8. **DIGANTIKAN (FL-010U, 16 Oktober 2026): asumsi lama "akun peserta opsional / mahasiswa tidak memiliki akun pada MVP" tidak lagi berlaku.** Keputusan pemilik pada FL-010U menetapkan bahwa `mahasiswa` **memiliki akun** dan masuk lewat Google Sign-In; akun itu dibuat `admin` (praregistrasi) dan ditautkan ke baris peserta melalui `people.account_user_id` (§4.4). Karena itu tidak ada lagi butir pada dokumen ini yang menyatakan akun peserta sebagai opsional atau tidak ada. Baris **D-013** pada `docs/DECISIONS.md` belum diubah pada tugas ini; pembaruan statusnya menunggu pemilik (dokumen itu tidak disunting oleh FL-010U).
9. **Cakupan `dosen`** belum final. Rancangan FL-010U mengikuti daftar tugas pada permintaan FL-010U secara harfiah; konsekuensinya **enrollment biometrik, pencatatan konsen, pengelolaan peserta, ekspor CSV, dan seluruh operasi perangkat menjadi milik `admin`**. Bila alur demo menuntut dosen melakukan enrollment/konsen pada kelas yang ditugaskan, `class_lecturers` (§4.17) tetap sama, tetapi matriks izin `docs/AUTHORIZATION.md` perlu diputuskan lebih dahulu.
10. **Identitas tunggal per orang:** satu akun Google = satu baris `users` = **satu** role. Menjalankan demo sebagai `admin` sekaligus `dosen` memerlukan **dua akun Google berbeda**. Bila pemilik menghendaki satu orang memegang dua role, itu keputusan model data baru (mis. tabel relasi akun–role, yang belum dirancang).
11. **Pelepasan penautan identitas Google dari akun yang sudah ada** (unlink tanpa menghapus akun) belum dirancang; baris `user_identities` saat ini hanya hilang lewat `ON DELETE CASCADE` saat akun dihapus (§4.16).
12. **Bentuk `classes` dan `consent_records` pada model target belum disamakan dengan migrasi yang sudah dijalankan.** `00001_init_core_schema.sql` memuat `classes.status`/`classes.description` serta `consent_records.purpose`/`status`/`method`, sedangkan dokumen ini memakai `version`/`capture_method`/`withdraw_reason` (§4.9, §4.18). Mana yang berlaku pada FL-011 harus diputuskan pemilik sebelum migrasi `00002+` ditulis; FL-010U hanya mencatat selisihnya dan **tidak** menulis migrasi.

Butir tambahan FL-010V (16 Oktober 2026):

13. **Q-22 (penarikan persetujuan mandiri oleh mahasiswa) TETAP TERBUKA.** FL-010V **tidak** menyetujuinya dan tidak mengubah aturan konsen; penarikan persetujuan tetap dilakukan lewat jalur yang berlaku di `docs/SECURITY_PRIVACY.md` §5a–§5f dan D-030. Jangan membaca keputusan FL-010V mana pun sebagai persetujuan Q-22.
14. **Q-24 (prosedur operasional saat sesi habis dan internet tidak tersedia) TETAP TERBUKA.** Yang berlaku hanya ketetapan bahwa **tidak ada bypass autentikasi** saat sesi tidak sah; rincian prosedurnya belum diputuskan dan tidak boleh diciptakan pada dokumen ini.
15. **Metode penyediaan sertifikat lokal untuk akses LAN Android belum ditentukan (pertanyaan terbuka baru, FL-010V).** Kebijakan D-035 menetapkan LAN privat + HTTPS dengan sertifikat tepercaya, tetapi **metode** pemenuhannya belum dipilih dan **belum diuji**: Android 7+ tidak mempercayai CA yang dipasang pengguna untuk aplikasi kecuali aplikasi memakai `network_security_config` atau build kustom, sehingga Expo Go tidak memadai. Opsi yang tercatat (CA lokal + SAN alamat IP LAN, reverse proxy lokal dengan TLS, atau `adb reverse` untuk development) beserta batasannya ada di `docs/ARCHITECTURE.md` §5.1. Keputusan metode ini adalah prasyarat pekerjaan pengikatan LAN + TLS pada FL-011.
16. **Mekanisme resolusi bila satu identitas Google perlu dipakai di beberapa akun belum ada.** D-037 menetapkan satu role utama per akun dan melarang satu identitas Google menghasilkan beberapa akun/sesi role; batasan unik §4.16 menegakkannya. Bila kebutuhan itu muncul, ia menuntut keputusan model data baru (misalnya tabel relasi akun–identitas atau akun–role) yang **belum dirancang**, dan sampai itu ada, permintaan seperti itu **ditolak**.
17. **Penguatan log respons backend (FL-005S) masih menjadi pekerjaan terpisah dan belum dikerjakan.** Access log Go saat ini dapat melaporkan status `200` untuk respons `503` batas waktu dan tidak mencatat respons `500` hasil pemulihan panic, sehingga keandalan jejak audit/keamanan belum dapat dijamin sebelum tugas itu selesai. FL-010V **tidak** mengubah kode apa pun; pekerjaan itu tercatat juga di `docs/ARCHITECTURE.md` §14.

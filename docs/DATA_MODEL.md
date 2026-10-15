# Model Data FaceLink AI

**Versi:** 1.0 · **Status:** SPESIFIKASI FINAL TAHAP DOKUMENTASI; **BELUM DIIMPLEMENTASIKAN** dan belum ada migrasi yang dijalankan.

Dokumen ini melengkapi `docs/API_CONTRACT.md` dan tidak menggantikan `docs/PRD.md`. Setiap perubahan skema setelah dokumen ini harus melalui berkas migrasi baru, bukan dengan mengubah migrasi yang sudah pernah dijalankan.

## 1. Ruang lingkup dan asumsi

1. MVP melayani **satu kelas, satu terminal, satu operator** (batasan AGENTS.md §2). Skema tetap tidak boleh mengunci diri sehingga organisasi lain mustahil ditambahkan (D-020 catatan portabilitas).
2. **Tidak ada multi-tenancy**, tidak ada payroll, tidak ada billing, dan tidak ada tabel `organization_id` yang dipaksakan pada seluruh tabel. Namun setiap entitas utama memiliki `id` UUID dan `created_at`, sehingga penambahan kolom pengelompokan di masa depan bersifat aditif.
3. Database MVP: **SQLite mode WAL** (D-020); target lanjutan: **PostgreSQL** (D-011 masih DITUNDA).
4. Semua waktu disimpan sebagai **TEXT ISO 8601 UTC** (`2026-10-15T08:00:00Z`). Tidak ada waktu lokal yang disimpan tanpa penanda zona.
5. Semua identitas memakai **UUID v4 berbentuk TEXT** agar portabel ke PostgreSQL dan mudah disalin antar berkas uji. Tidak ada primary key berupa auto-increment bilangan bulat yang bocor ke API, **kecuali** tabel teknis yang tidak pernah menjadi identitas domain (§4.14).
6. Nilai boolean disimpan sebagai `INTEGER` 0/1 pada SQLite dan dipetakan ke `BOOLEAN` pada PostgreSQL.
7. JSON disimpan sebagai `TEXT` berisi JSON yang valid pada SQLite; pada PostgreSQL menjadi `JSONB`.

## 2. Ikhtisar tabel

| # | Tabel | Kegunaan | Modul API terkait |
|---|---|---|---|
| 1 | `users` | Akun operator dan admin | §5.1, §5.2 |
| 2 | `operator_tokens` | Token sesi operator yang dapat dicabut | §5.1 |
| 3 | `roles` | Daftar peran yang sah beserta deskripsinya | §5.1 |
| 4 | `people` | Peserta (identitas non-biometrik) | §5.2 |
| 5 | `classes` | Kelas atau kelompok | §5.5 |
| 6 | `class_memberships` | Keanggotaan peserta pada kelas | §5.5 |
| 7 | `attendance_sessions` | Sesi absensi | §5.6 |
| 8 | `attendance_records` | Catatan kehadiran (penjaga anti-duplikasi) | §5.7, §5.8, §5.10 |
| 9 | `consent_records` | Riwayat persetujuan biometrik | §5.3 |
| 10 | `face_enrollments` | Sesi pendaftaran wajah berjangka waktu | §5.4 |
| 11 | `face_enrollment_samples` | Metadata kualitas sampel enrollment (bukan gambar) | §5.4 |
| 12 | `face_templates` | Template wajah terenkripsi | §5.3, §5.4, §6 |
| 13 | `devices` | Perangkat kamera terdaftar | §5.9 |
| 14 | `audit_events` | Jejak aktivitas penting | §5.13 |
| 15 | `idempotency_records` | Penjaga pengulangan permintaan jaringan | §8 |

Diagram hubungan ringkas (`||--o{` berarti satu-ke-banyak):

```text
users ||--o{ operator_tokens
users ||--o{ audit_events            (actor_id, tanpa foreign key keras ke users)
roles ||--o{ users                   (role disimpan sebagai TEXT + daftar acuan)
classes ||--o{ class_memberships
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

Akun manusia yang memakai aplikasi. Hanya peran `operator` dan `admin` yang ada di MVP.

| Kolom | Tipe | Null | Aturan |
|---|---|---|---|
| `id` | TEXT | tidak | PK |
| `email` | TEXT | tidak | Unik, disimpan huruf kecil |
| `display_name` | TEXT | tidak | 1–120 karakter |
| `password_hash` | TEXT | tidak | Hash lambat dengan garam unik; **tidak pernah** keluar dari backend |
| `role` | TEXT | tidak | `CHECK (role IN ('operator','admin'))` |
| `must_change_password` | INTEGER | tidak | Bawaan 1 untuk akun baru |
| `is_active` | INTEGER | tidak | Bawaan 1 |
| `last_login_at` | TEXT | ya | Diperbarui saat login berhasil |
| `created_at` | TEXT | tidak | — |
| `updated_at` | TEXT | ya | — |

- **Unik:** `email`.
- **Indeks:** `idx_users_role`.
- **Penghapusan:** akun tidak dihapus keras selama masih menjadi `actor_id` pada audit; gunakan `is_active = 0`.
- **Retensi:** selama sistem dipakai.

### 4.2 `operator_tokens`

Menyimpan token sesi operator agar dapat dicabut. Token disimpan dalam bentuk hash.

| Kolom | Tipe | Null | Aturan |
|---|---|---|---|
| `id` | TEXT | tidak | PK |
| `user_id` | TEXT | tidak | FK → `users.id` `ON DELETE CASCADE` |
| `token_hash` | TEXT | tidak | Unik; hash token akses |
| `refresh_hash` | TEXT | ya | Unik bila ada |
| `expires_at` | TEXT | tidak | — |
| `revoked_at` | TEXT | ya | Diisi saat logout atau pencabutan admin |
| `user_agent_ringkas` | TEXT | ya | Maksimum 120 karakter |
| `created_at` | TEXT | tidak | — |

- **Indeks:** `idx_operator_tokens_user`, `idx_operator_tokens_expires`.
- **Retensi:** baris kedaluwarsa dihapus oleh pemeliharaan setelah 7 hari, terpisah dari audit.

### 4.3 `roles`

Tabel acuan kecil agar nama peran tidak tersebar sebagai string bebas. **Tidak** menjadi sistem izin dinamis; ini hanya daftar sah beserta deskripsi.

| Kolom | Tipe | Null | Aturan |
|---|---|---|---|
| `name` | TEXT | tidak | PK; `operator`, `admin`, `device` |
| `description` | TEXT | tidak | Penjelasan singkat |
| `created_at` | TEXT | tidak | — |

- Baris awal diisi oleh migrasi awal. `users.role` tetap divalidasi `CHECK`, dan tidak ada tabel relasi izin-detail pada MVP.

### 4.4 `people`

Peserta absensi. Identitas biometrik **tidak** ada di tabel ini.

| Kolom | Tipe | Null | Aturan |
|---|---|---|---|
| `id` | TEXT | tidak | PK |
| `external_code` | TEXT | tidak | Unik; 1–32 karakter; **bukan NIK asli** untuk data demo |
| `display_name` | TEXT | tidak | 1–120 karakter |
| `status` | TEXT | tidak | `CHECK (status IN ('active','inactive'))`, bawaan `active` |
| `note` | TEXT | ya | Maksimum 200 karakter |
| `created_at` | TEXT | tidak | — |
| `updated_at` | TEXT | ya | — |
| `deleted_at` | TEXT | ya | Penghapusan lunak, lihat §6.3 |

- **Unik:** `external_code` (tanpa memandang `deleted_at`; kode tidak boleh dipakai ulang agar audit lama tetap bermakna).
- **Indeks:** `idx_people_status`, `idx_people_display_name`.
- **Penghapusan:** `DELETE /people/{id}` menghapus `face_templates` terkait pada transaksi yang sama (D-015), lalu mengisi `deleted_at`. Baris tidak hilang agar `attendance_records` historis tetap dapat dibaca.

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
- **Perilaku saat dekripsi gagal:** baris **tidak** dipakai dan **tidak** diganti dengan tebakan. Backend menandai kegagalan, menulis audit, dan mengembalikan hasil “tidak dapat diverifikasi” kepada operator. Tidak ada kunci pengganti otomatis dan tidak ada pembuatan template baru secara diam-diam. Kunci yang hilang tidak menghasilkan template baru; peserta harus mendaftar ulang dengan persetujuan baru (`docs/SECURITY_PRIVACY.md` §5d).
- **Penghapusan:** pencabutan persetujuan dan penghapusan peserta **menghapus baris ini** (bukan menandai). Template tidak dapat dipulihkan setelah dihapus karena kunci terpisah tidak menyimpan salinan.
- **Retensi:** selama persetujuan aktif. Bila persetujuan dicabut, baris harus terhapus pada transaksi yang sama.
- **Pengelolaan kunci (usulan D-030, belum disetujui):** `key_version` diisi kunci aktif saat penulisan; versi pertama bernilai teks `'1'`. Kunci disimpan di luar database dan di luar Git, dibungkus DPAPI cakupan pengguna, dan hanya diakses lewat antarmuka `KeyProvider` di Go. Indeks `idx_face_templates_key_version` ada agar penghapusan kunci lama setelah rotasi dapat diperiksa terhadap baris yang menunjuk versinya. Kebijakan penuh: `docs/SECURITY_PRIVACY.md` §5a–§5d.
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
- **Indeks:** `idx_idempotency_expires` untuk pembersihan berkala.
- **Penyimpanan gambar:** **tidak**. Yang disimpan hanya hash permintaan dan respons ringkas.
- **Retensi:** 24 jam, dihapus oleh proses pemeliharaan. Tabel ini **bukan** penjaga terakhir absensi ganda; penjaga itu ada di `attendance_records`.

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
| `operator_tokens` → `users` | `ON DELETE CASCADE` | Token tanpa pemilik tidak bermakna |
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
7. Kebijakan kunci lengkap (lokasi, perlindungan, rotasi, pemulihan) ada di `docs/SECURITY_PRIVACY.md` §5a–§5d sebagai **usulan D-030**. Selama D-030 belum disetujui, tabel ini **tidak boleh** ditulis oleh kode apa pun.

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

## 13. Pertanyaan terbuka dan hal yang belum diputuskan

1. **Kebijakan retensi kehadiran** dan hubungannya dengan penghapusan peserta belum ditetapkan (lihat `docs/OPEN_QUESTIONS.md`). Ini yang menghambat `DELETE /people` karena `attendance_records` memakai `RESTRICT`.
2. **`synchronous`** untuk kebutuhan demo: `NORMAL` dipilih sebagai kandidat, tetapi bila ketahanan data lebih penting daripada kecepatan, `FULL` adalah pilihan yang sah dan harus diputuskan sadar.
3. **Angka `busy_timeout`** dan batas laju belum diukur (Q-18).
4. **Ambang dan margin pencocokan** tidak berada di database; ia milik backend Go dan harus terkalibrasi (Q-07).
5. **Apakah peserta boleh memiliki lebih dari satu template per model** (misalnya dengan dan tanpa kacamata) belum diputuskan; skema saat ini mengizinkan satu per `(person_id, model_id, model_version)`.
6. **Enkripsi seluruh berkas database** (mis. SQLCipher) bukan bagian dari MVP dan belum diputuskan.
7. **Tabel `notifications` atau antrean pesan** tidak ada dan tidak dibutuhkan selama D-021 berlaku (polling, bukan WebSocket).

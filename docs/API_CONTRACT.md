# Kontrak API FaceLink AI

**Versi:** 1.0 · **Status:** SPESIFIKASI FINAL TAHAP DOKUMENTASI; **BELUM DIIMPLEMENTASIKAN** dan belum menjadi berkas OpenAPI yang dapat dijalankan.

Dokumen ini adalah acuan wajib yang harus ditinjau sebelum membuat handler Go atau layar React Native yang memanggilnya. Kontrak ini **mengikuti** keputusan yang sudah disetujui di `docs/DECISIONS.md` (D-010, D-020, D-021, D-022, D-027, D-028) dan **tidak menggantikan** `docs/PRD.md`.

**Tidak ada kode aplikasi yang ditulis pada tugas ini.** Seluruh contoh di bawah adalah ilustrasi bentuk data, bukan bukti endpoint sudah berjalan.

## 1. Aturan umum

### 1.1 Prinsip dasar

1. Seluruh API publik berada di bawah awalan **`/api/v1`**. Tidak ada endpoint publik di luar awalan itu, kecuali pemeriksaan kesehatan minimum pada §5.12 yang boleh tetap tersedia tanpa versi untuk kebutuhan probe lokal.
2. **Backend Go adalah satu-satunya penulis database dan satu-satunya pengambil keputusan domain absensi** (D-027). Mesin AI Python tidak pernah menetapkan peserta sebagai hadir dan tidak pernah menulis tabel kehadiran.
3. **UI tidak menentukan kehadiran** (D-028). Aplikasi React Native hanya menampilkan hasil dan mengirim permintaan manual melalui endpoint yang diaudit.
4. Format pertukaran normal: **JSON UTF-8**. Unggahan gambar: **`image/jpeg`** biner atau `multipart/form-data` sesuai endpoint.
5. Identitas record: **UUID v4 tanpa makna** (bukan NIK atau nomor absen). Waktu: **ISO 8601 UTC** dengan akhiran `Z` (`2026-10-15T08:00:00Z`); aplikasi menampilkan zona lokal pengguna.
6. Setiap respons menyertakan `request_id` yang tidak mengandung data pribadi, untuk penelusuran log.
7. Semua endpoint selain login dan kesehatan memerlukan autentikasi. Token operator dan token perangkat **terpisah** dan tidak dapat saling menggantikan.
8. Respons tidak boleh membocorkan embedding, gambar mentah, hash kata sandi, token, atau dugaan identitas yang meragukan.
9. Jangan pernah mencatat payload biometrik, JPEG, token, atau kunci ke log.
10. Validasi yang wajib ada di setiap endpoint unggahan: jenis konten, ukuran maksimum, dan batas waktu proses.

### 1.2 Peran dan cakupan akses

| Peran | Cakupan MVP | Catatan |
|---|---|---|
| `operator` | Seluruh endpoint operator: orang, kelas, konsen, enrollment, sesi, rekap, koreksi manual, perangkat | Satu-satunya pengguna aplikasi yang wajib ada di MVP (D-013 masih USULAN pada bagian akun peserta) |
| `admin` | Sama seperti operator, ditambah pengelolaan akun operator dan pencabutan perangkat | Dipisahkan agar audit tetap jelas; jumlah akun admin minimal |
| `device` | Hanya `POST /api/v1/devices/{device_id}/recognition-attempts` | Token perangkat terikat satu `device_id`; tidak dapat membaca rekap atau daftar peserta |
| `ai-engine` (internal) | Hanya endpoint internal §6 pada `127.0.0.1` | Bukan peran API publik; tidak melewati router publik |

### 1.3 Format respons sukses

Objek tunggal:

```json
{
  "data": {
    "id": "11111111-1111-4111-8111-111111111111"
  },
  "request_id": "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
}
```

Daftar dengan penomoran:

```json
{
  "data": [
    { "id": "11111111-1111-4111-8111-111111111111" }
  ],
  "page": {
    "next_cursor": "MjAyNi0xMC0xNVQwODowMDowMFo",
    "has_more": true,
    "limit": 50
  },
  "request_id": "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
}
```

Aturan penomoran: gunakan **cursor**, bukan nomor halaman. `limit` bawaan 50, maksimum 200. Cursor bersifat buram bagi klien dan tidak boleh dirakit sendiri oleh aplikasi. Pengurutan bawaan: `created_at` menurun kecuali dinyatakan lain.

### 1.4 Format kesalahan

```json
{
  "error": {
    "code": "SESSION_CLOSED",
    "message": "Sesi absensi sudah ditutup.",
    "details": [
      { "field": "session_id", "issue": "status saat ini: closed" }
    ],
    "request_id": "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
  }
}
```

Aturan keras:

- `code` **stabil** dan boleh dipakai klien untuk percabangan logika; `message` hanya untuk manusia dan boleh berubah.
- `details` bersifat opsional, tidak boleh memuat data biometrik, dan hanya menyebut nama field.
- Kesalahan validasi tidak memakai pesan internal database.

### 1.5 Kode status HTTP dan kode kesalahan stabil

| HTTP | Kapan dipakai | Contoh `code` |
|---|---|---|
| `200` | Permintaan berhasil dibaca atau memperbarui data | — |
| `201` | Sumber daya baru dibuat | — |
| `204` | Penghapusan/penutupan berhasil tanpa isi | — |
| `304` | `If-None-Match` cocok, tidak ada perubahan | — |
| `400` | JSON rusak, tipe salah, parameter wajib hilang | `MALFORMED_JSON`, `MISSING_FIELD` |
| `401` | Belum terautentikasi atau token tidak sah | `UNAUTHENTICATED`, `TOKEN_EXPIRED` |
| `403` | Terautentikasi tetapi tidak berwenang | `FORBIDDEN_ROLE`, `DEVICE_REVOKED` |
| `404` | Sumber daya tidak ada atau tidak terlihat oleh peran ini | `NOT_FOUND` |
| `405` | Metode tidak diizinkan pada jalur tersebut | `METHOD_NOT_ALLOWED` |
| `409` | Konflik keadaan atau operasi berulang | `ALREADY_CHECKED_IN`, `SESSION_CLOSED` |
| `412` | `If-Match`/prasyarat versi tidak cocok | `PRECONDITION_FAILED` |
| `413` | JPEG melampaui batas ukuran | `IMAGE_TOO_LARGE` |
| `415` | Jenis konten bukan JPEG yang diterima | `UNSUPPORTED_MEDIA_TYPE` |
| `422` | JSON sah tetapi melanggar aturan domain | `CONSENT_REQUIRED`, `CLASS_MISMATCH` |
| `429` | Melampaui batas laju | `RATE_LIMITED` |
| `500` | Kesalahan tak terduga di backend | `INTERNAL_ERROR` |
| `503` | Ketergantungan tidak tersedia | `AI_ENGINE_UNAVAILABLE`, `DATABASE_BUSY` |
| `504` | Ketergantungan melewati batas waktu | `AI_ENGINE_TIMEOUT` |

Kode kesalahan stabil minimum yang dipakai lintas modul:

| `code` | HTTP | Makna |
|---|---|---|
| `VALIDATION_FAILED` | 400/422 | Satu atau beberapa field tidak memenuhi aturan |
| `UNAUTHENTICATED` | 401 | Tidak ada atau token tidak sah |
| `TOKEN_EXPIRED` | 401 | Token melewati masa berlaku |
| `FORBIDDEN_ROLE` | 403 | Peran tidak memiliki akses |
| `NOT_FOUND` | 404 | Sumber daya tidak ditemukan |
| `CONSENT_REQUIRED` | 422 | Peserta tidak memiliki persetujuan biometrik aktif |
| `CONSENT_WITHDRAWN` | 422 | Persetujuan dicabut; template dihapus |
| `SESSION_NOT_OPEN` | 422 | Sesi bukan dalam keadaan terbuka |
| `SESSION_CLOSED` | 409 | Sesi sudah ditutup saat penulisan dilakukan |
| `CLASS_MISMATCH` | 422 | Peserta bukan anggota kelas sesi |
| `ALREADY_CHECKED_IN` | 409 | Sudah ada catatan kehadiran pada sesi yang sama |
| `DEVICE_REVOKED` | 403 | Perangkat dicabut atau tidak aktif |
| `DEVICE_SESSION_MISMATCH` | 422 | Perangkat tidak terhubung ke sesi yang dikirim |
| `IMAGE_TOO_LARGE` | 413 | Melebihi batas ukuran |
| `UNSUPPORTED_MEDIA_TYPE` | 415 | Bukan JPEG yang diterima |
| `IMAGE_DECODE_FAILED` | 422 | JPEG rusak atau tidak dapat dibaca |
| `RATE_LIMITED` | 429 | Terlalu banyak permintaan |
| `AI_ENGINE_UNAVAILABLE` | 503 | Pekerja AI tidak dapat dihubungi |
| `AI_ENGINE_TIMEOUT` | 504 | Pekerja AI melewati batas waktu |
| `DATABASE_BUSY` | 503 | Database terkunci melewati batas tunggu |
| `INTERNAL_ERROR` | 500 | Kesalahan tak terduga |

## 2. Strategi versi

1. Versi mayor ada pada jalur (`/api/v1`). Perubahan yang **memutus kompatibilitas** menaikkan nomor mayor dan menambah jalur baru; jalur lama tidak dihapus dalam tugas yang sama.
2. Perubahan **aditif** (field baru opsional, endpoint baru) tidak menaikkan versi. Klien wajib mengabaikan field yang tidak dikenal.
3. Field yang akan dihapus ditandai `deprecated` di dokumentasi minimal satu rilis sebelum dihapus.
4. Penghapusan field wajib, perubahan arti field, dan perubahan bentuk `data` dari objek menjadi daftar dianggap **memutus kompatibilitas**.
5. Versi kontrak internal Go↔Python (§6) bergerak **terpisah** dari versi API publik; keduanya tidak boleh diasumsikan sama.

## 3. Ikhtisar modul

Tiga belas modul yang diminta, dipetakan ke jalur nyata. Beberapa kebutuhan **sengaja tidak menjadi endpoint publik** karena merupakan operasi internal atau efek samping endpoint lain.

| # | Modul | Jalur | Catatan |
|---|---|---|---|
| 1 | Autentikasi dan manajemen sesi operator | `/auth/*`, `/me`, `/operator-sessions` | Token operator |
| 2 | Pengelolaan pengguna dan peserta | `/people/*`, `/users/*` | `/users` hanya untuk admin |
| 3 | Persetujuan biometrik | `/people/{person_id}/consent*` | Wajib sebelum enrollment |
| 4 | Enrollment wajah | `/people/{person_id}/face-enrollments*` | Gambar tidak disimpan permanen |
| 5 | Kelas dan keanggotaan | `/classes/*` | Satu kelas untuk demo |
| 6 | Sesi absensi | `/sessions/*` | Siklus draft → open → closed |
| 7 | Hasil verifikasi wajah | Bagian dari `recognition-attempts` | **Bukan** endpoint terpisah |
| 8 | Pencatatan kehadiran | Efek dari pengenalan atau koreksi manual | **Bukan** endpoint publik terpisah |
| 9 | Penolakan dan jalur manual | `/sessions/{id}/manual-checkins` | Wajib ada sebagai alternatif |
| 10 | Pendaftaran dan autentikasi perangkat | `/devices/*` | Token terpisah |
| 11 | Rekap dan ekspor laporan | `/sessions/{id}/attendance`, `/export.csv` | Tanpa data biometrik |
| 12 | Kesehatan dan kesiapan | `/healthz`, `/readyz` | Tidak membocorkan rahasia |
| 13 | Audit aktivitas penting | `/audit-events` | Hanya admin/operator |

Justifikasi pemisahan modul 7 dan 8: hasil verifikasi wajah dan pencatatan kehadiran **bukan** dua operasi yang dapat dipanggil klien secara terpisah. Kalau keduanya menjadi endpoint publik, akan ada jalur yang memungkinkan klien menetapkan kehadiran tanpa validasi wajah, atau mencatat hasil pengenalan tanpa memeriksa konsen, kelas, dan sesi. Karena itu modul 7 dan 8 diwujudkan sebagai **hasil satu endpoint terminal** (§5.10) yang di dalam backend Go menjalankan seluruh pemeriksaan berurutan.

## 4. Ikhtisar endpoint

Semua jalur diawali `/api/v1`.

| Modul | Metode | Jalur | Peran |
|---|---|---|---|
| 1 | `POST` | `/auth/login` | publik |
| 1 | `POST` | `/auth/refresh` | operator, admin |
| 1 | `POST` | `/auth/logout` | operator, admin |
| 1 | `GET` | `/me` | operator, admin |
| 1 | `GET` | `/operator-sessions` | admin |
| 1 | `DELETE` | `/operator-sessions/{session_id}` | admin |
| 2 | `GET` | `/people` | operator, admin |
| 2 | `POST` | `/people` | operator, admin |
| 2 | `GET` | `/people/{person_id}` | operator, admin |
| 2 | `PATCH` | `/people/{person_id}` | operator, admin |
| 2 | `DELETE` | `/people/{person_id}` | admin |
| 2 | `GET` | `/users` | admin |
| 2 | `POST` | `/users` | admin |
| 3 | `GET` | `/people/{person_id}/consent` | operator, admin |
| 3 | `POST` | `/people/{person_id}/consent` | operator, admin |
| 3 | `POST` | `/people/{person_id}/consent/withdraw` | operator, admin |
| 4 | `POST` | `/people/{person_id}/face-enrollments` | operator, admin |
| 4 | `GET` | `/people/{person_id}/face-enrollments` | operator, admin |
| 4 | `POST` | `/face-enrollments/{enrollment_id}/samples` | operator, admin |
| 4 | `POST` | `/face-enrollments/{enrollment_id}/complete` | operator, admin |
| 4 | `DELETE` | `/face-enrollments/{enrollment_id}` | operator, admin |
| 5 | `GET` | `/classes` | operator, admin |
| 5 | `POST` | `/classes` | operator, admin |
| 5 | `GET` | `/classes/{class_id}/members` | operator, admin |
| 5 | `PUT` | `/classes/{class_id}/members/{person_id}` | operator, admin |
| 5 | `DELETE` | `/classes/{class_id}/members/{person_id}` | operator, admin |
| 6 | `GET` | `/sessions` | operator, admin |
| 6 | `POST` | `/sessions` | operator, admin |
| 6 | `GET` | `/sessions/{session_id}` | operator, admin |
| 6 | `POST` | `/sessions/{session_id}/open` | operator, admin |
| 6 | `POST` | `/sessions/{session_id}/close` | operator, admin |
| 9 | `POST` | `/sessions/{session_id}/manual-checkins` | operator, admin |
| 9 | `GET` | `/sessions/{session_id}/manual-checkins` | operator, admin |
| 10 | `POST` | `/devices` | admin |
| 10 | `GET` | `/devices` | operator, admin |
| 10 | `POST` | `/devices/{device_id}/rotate-token` | admin |
| 10 | `DELETE` | `/devices/{device_id}` | admin |
| 11 | `GET` | `/sessions/{session_id}/attendance` | operator, admin |
| 11 | `GET` | `/sessions/{session_id}/export.csv` | operator, admin |
| 12 | `GET` | `/healthz` | publik (lokal) |
| 12 | `GET` | `/readyz` | operator, admin |
| 13 | `GET` | `/audit-events` | operator, admin |
| — | `POST` | `/devices/{device_id}/recognition-attempts` | device |

## 5. Detail endpoint

Notasi: **wajib** berarti field harus ada. Semua contoh JSON memakai UUID dan nilai buatan, tanpa data biometrik nyata.

### 5.1 Modul 1 — Autentikasi dan manajemen sesi operator

#### `POST /api/v1/auth/login`

- **Peran:** publik (dengan pembatasan laju ketat).
- **Tubuh permintaan:** `email` (wajib, string), `password` (wajib, string).
- **Respons `200`:**

```json
{
  "data": {
    "access_token": "<token>",
    "expires_in": 3600,
    "operator": { "id": "33333333-3333-4333-8333-333333333333", "display_name": "Operator Demo", "role": "operator" }
  },
  "request_id": "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
}
```

- **Kode status:** `200`, `400`, `401`, `429`.
- **Kesalahan:** `VALIDATION_FAILED`, `UNAUTHENTICATED`, `RATE_LIMITED`.
- **Validasi:** email dinormalisasi (huruf kecil, tanpa spasi tepi); kata sandi minimal 8 karakter.
- **Efek database:** memperbarui `users.last_login_at`; menambah `audit_events` dengan aksi `auth.login.succeeded` atau `auth.login.failed` (tanpa alamat email pada metadata aman; simpan `actor_id` bila diketahui).
- **Idempotensi:** tidak berlaku; setiap percobaan menambah baris audit.
- **Catatan keamanan:** pesan kesalahan tidak boleh membedakan “email tidak ada” dan “kata sandi salah”.

#### `POST /api/v1/auth/refresh`

- **Peran:** operator, admin.
- **Tubuh permintaan:** `refresh_token` (wajib).
- **Respons `200`:** sama bentuknya dengan login.
- **Kode status:** `200`, `401`.
- **Efek database:** menandai token lama sebagai tidak berlaku; menambah audit `auth.token.refreshed`.
- **Idempotensi:** token lama tidak dapat dipakai dua kali.

#### `POST /api/v1/auth/logout`

- **Peran:** operator, admin.
- **Tubuh permintaan:** kosong.
- **Respons:** `204` tanpa isi.
- **Efek database:** mencabut sesi operator aktif; audit `auth.logout`.
- **Idempotensi:** panggilan kedua tetap `204` tanpa perubahan tambahan.

#### `GET /api/v1/me`

- **Peran:** operator, admin.
- **Query:** —
- **Respons `200`:** `{ "data": { "id", "display_name", "email", "role" } }`. Hash kata sandi tidak pernah dikirim.
- **Kode status:** `200`, `401`.
- **Efek database:** —
- **Idempotensi:** aman diulang.

#### `GET /api/v1/operator-sessions`

- **Peran:** admin.
- **Respons `200`:** daftar sesi operator aktif dengan `id`, `user_id`, `created_at`, `last_seen_at`, `user_agent_ringkas`.
- **Kode status:** `200`, `403`.
- **Efek database:** —
- **Idempotensi:** aman diulang.

#### `DELETE /api/v1/operator-sessions/{session_id}`

- **Peran:** admin.
- **Respons:** `204`.
- **Kode status:** `204`, `403`, `404`.
- **Efek database:** token sesi dicabut; audit `auth.session.revoked`.
- **Idempotensi:** mencabut sesi yang sudah tercabut mengembalikan `404`.

### 5.2 Modul 2 — Pengelolaan pengguna dan peserta

#### `GET /api/v1/people`

- **Peran:** operator, admin.
- **Query:** `status` (opsional: `active`, `inactive`), `class_id` (opsional), `cursor`, `limit`.
- **Respons `200`:** daftar `{ id, external_code, display_name, status, consent_state, enrollment_state }`.
  - `consent_state`: `active`, `none`, atau `withdrawn`.
  - `enrollment_state`: `none` atau `enrolled`.
  - Nilai turunan ini **tidak** membocorkan jumlah atau isi template.
- **Kode status:** `200`, `401`.
- **Efek database:** —
- **Idempotensi:** aman diulang.

#### `POST /api/v1/people`

- **Peran:** operator, admin.
- **Tubuh permintaan:** `external_code` (wajib, unik), `display_name` (wajib), `note` (opsional, ≤200 karakter).
- **Respons `201`:** objek `people` yang dibuat.
- **Kode status:** `201`, `400`, `409`, `422`.
- **Kesalahan:** `VALIDATION_FAILED`, `DUPLICATE_EXTERNAL_CODE`.
- **Validasi:** `external_code` 1–32 karakter, hanya huruf/angka/tanda hubung; **dilarang** memakai NIK asli untuk data demo (Q-13).
- **Efek database:** `INSERT INTO people`; audit `person.created`.
- **Idempotensi:** **tidak** idempoten secara bawaan; pengulangan menghasilkan `409` karena `external_code` unik.

#### `GET /api/v1/people/{person_id}`

- **Peran:** operator, admin.
- **Respons `200`:** detail peserta termasuk `consent_state` dan `enrollment_state`.
- **Kode status:** `200`, `404`.
- **Efek database:** —
- **Idempotensi:** aman diulang.

#### `PATCH /api/v1/people/{person_id}`

- **Peran:** operator, admin.
- **Tubuh permintaan:** `display_name`, `status`, `note` (semua opsional; minimal satu).
- **Respons `200`:** objek setelah perubahan.
- **Kode status:** `200`, `400`, `404`, `422`.
- **Efek database:** `UPDATE people`; audit `person.updated` dengan daftar nama field yang berubah (bukan nilai lama/baru penuh).
- **Idempotensi:** aman diulang untuk nilai yang sama.

#### `DELETE /api/v1/people/{person_id}`

- **Peran:** admin.
- **Respons:** `204`.
- **Kode status:** `204`, `403`, `404`, `409`.
- **Validasi/aturan:** penghapusan **wajib** menghapus template biometrik terkait pada transaksi yang sama (D-015). Bila peserta memiliki catatan kehadiran dan kebijakan retensi belum ditetapkan, endpoint mengembalikan `409 POLICY_CONFLICT` alih-alih menghapus data kehadiran diam-diam.
- **Efek database:** `DELETE FROM face_templates`; menonaktifkan atau menghapus `people`; audit `person.deleted`.
- **Idempotensi:** penghapusan kedua mengembalikan `404`.

#### `GET /api/v1/users`, `POST /api/v1/users`

- **Peran:** admin.
- **Tubuh permintaan `POST`:** `email`, `display_name`, `role` (`operator`/`admin`), `password` sementara yang wajib diganti saat login pertama.
- **Kode status:** `201`, `400`, `409`, `403`.
- **Efek database:** `INSERT INTO users`; audit `user.created`.
- **Catatan:** tidak ada pendaftaran mandiri. Tidak ada endpoint manipulasi peran massal.

### 5.3 Modul 3 — Persetujuan biometrik

#### `GET /api/v1/people/{person_id}/consent`

- **Peran:** operator, admin.
- **Respons `200`:** riwayat persetujuan: `{ id, purpose, version, capture_method, granted_at, withdrawn_at }`.
- **Kode status:** `200`, `404`.
- **Efek database:** —
- **Idempotensi:** aman diulang.

#### `POST /api/v1/people/{person_id}/consent`

- **Peran:** operator, admin.
- **Tubuh permintaan:** `purpose` (wajib, mis. `attendance_face_recognition`), `version` (wajib, versi teks persetujuan), `capture_method` (wajib: `written_form`/`digital_form`), `granted_at` (opsional; bawaan waktu server).
- **Respons `201`:** objek persetujuan.
- **Kode status:** `201`, `400`, `404`, `409`.
- **Validasi:** `version` tidak boleh kosong; persetujuan tidak boleh dicatat untuk peserta yang tidak ada; persetujuan ganda aktif dengan `version` sama ditolak `409`.
- **Efek database:** `INSERT INTO consent_records`; audit `consent.granted`.
- **Idempotensi:** menggunakan `Idempotency-Key` opsional; tanpa kunci, pengulangan identik menghasilkan `409`.

#### `POST /api/v1/people/{person_id}/consent/withdraw`

- **Peran:** operator, admin.
- **Tubuh permintaan:** `reason` (wajib, tidak boleh kosong), `withdrawn_at` (opsional).
- **Respons `200`:** objek persetujuan setelah pencabutan.
- **Kode status:** `200`, `400`, `404`, `409`.
- **Efek database (satu transaksi):** mengisi `consent_records.withdrawn_at`; **menghapus** baris `face_templates` milik peserta; audit `consent.withdrawn` yang mencatat jumlah template yang dihapus.
- **Idempotensi:** pencabutan ulang ketika sudah dicabut mengembalikan `409 CONSENT_WITHDRAWN`.
- **Catatan:** setelah pencabutan, upaya pengenalan tidak boleh menampilkan peserta sebagai kandidat, karena template sudah tidak ada. Ini **wajib diuji** sebagai skenario kontrak (§8).

### 5.4 Modul 4 — Enrollment wajah

Enrollment adalah operasi **berjangka waktu**. Tidak ada endpoint unggah bebas yang memungkinkan seseorang mendaftarkan wajah ke identitas orang lain.

#### `POST /api/v1/people/{person_id}/face-enrollments`

- **Peran:** operator, admin.
- **Tubuh permintaan:** `model_hint` (opsional; diabaikan bila tidak dikenal).
- **Respons `201`:** `{ "data": { "id": "44444444-4444-4444-8444-444444444444", "expires_at": "2026-10-15T08:10:00Z", "samples_required_min": 3, "samples_required_max": 10 } }`.
- **Kode status:** `201`, `404`, `422`.
- **Kesalahan:** `CONSENT_REQUIRED`, `CONSENT_WITHDRAWN`.
- **Validasi:** peserta wajib ada dan memiliki persetujuan aktif. Masa berlaku bawaan 10 menit.
- **Efek database:** `INSERT INTO face_enrollments` berstatus `pending`; audit `enrollment.started`.
- **Idempotensi:** `Idempotency-Key` dihormati; tanpa kunci, percobaan ganda membuat sesi enrollment baru dan menutup yang sebelumnya.

#### `GET /api/v1/people/{person_id}/face-enrollments`

- **Peran:** operator, admin.
- **Respons `200`:** daftar sesi enrollment peserta dengan status dan alasan penolakan sampel.
- **Kode status:** `200`, `404`.
- **Efek database:** —
- **Idempotensi:** aman diulang.

#### `POST /api/v1/face-enrollments/{enrollment_id}/samples`

- **Peran:** operator, admin.
- **Konten:** `image/jpeg` biner.
- **Header:** `Content-Length` wajib; `If-Match` opsional untuk mencegah sampel ganda akibat retry.
- **Respons `201`:** `{ "data": { "sample_index": 2, "quality": { "status": "accepted", "score": 0.71 } } }`.
- **Respons kegagalan sampel:** `422` dengan `SAMPLES_REJECTED` dan `details[].issue` bernilai `NO_FACE`, `MULTIPLE_FACES`, `LOW_QUALITY`, atau `FACE_TOO_SMALL`. Ini **penolakan sampel**, bukan kegagalan server.
- **Kode status:** `201`, `400`, `404`, `409`, `413`, `415`, `422`, `503`, `504`.
- **Efek database:** `INSERT INTO face_enrollment_samples` (menyimpan hanya metadata kualitas dan indeks, **bukan** JPEG mentah); menambah `face_enrollments.accepted_samples`.
- **Idempotensi:** `Idempotency-Key` dihormati; pengulangan dengan kunci sama mengembalikan hasil pertama tanpa menambah sampel.
- **Retensi:** JPEG sementara di memori dan dibuang setelah pemrosesan; tidak ada penyimpanan gambar mentah permanen secara bawaan (D-015).

#### `POST /api/v1/face-enrollments/{enrollment_id}/complete`

- **Peran:** operator, admin.
- **Tubuh permintaan:** kosong.
- **Respons `200`:** `{ "data": { "template_id": "...", "model_id": "sface", "model_version": "..." , "created_at": "..." } }`.
- **Kode status:** `200`, `409`, `422`, `503`, `504`.
- **Kesalahan:** `INSUFFICIENT_SAMPLES` (minimal 3 sampel diterima), `ENROLLMENT_EXPIRED`, `CONSENT_REQUIRED`.
- **Efek database (satu transaksi):** menghitung embedding rata-rata dari sampel yang diterima melalui mesin AI, mengenkripsi hasilnya (D-022), `INSERT INTO face_templates`, menandai enrollment `completed`, lalu menghapus baris `face_enrollment_samples`.
- **Idempotensi:** menyelesaikan enrollment yang sudah selesai mengembalikan `409`.
- **Catatan:** bila `model_version` berbeda dari template yang sudah ada untuk peserta yang sama, template lama **tidak** dihapus otomatis; keputusan ada di `docs/FACE_ENGINE.md`.

#### `DELETE /api/v1/face-enrollments/{enrollment_id}`

- **Peran:** operator, admin.
- **Respons:** `204`.
- **Kode status:** `204`, `404`, `409`.
- **Efek database:** enrollment `pending` dibatalkan; bila sudah `completed`, endpoint ini **tidak** menghapus template — penghapusan template memakai §5.3 withdraw atau jalur admin yang diaudit.
- **Idempotensi:** penghapusan kedua mengembalikan `404`.

### 5.5 Modul 5 — Kelas dan keanggotaan

#### `GET /api/v1/classes`

- **Peran:** operator, admin.
- **Query:** `cursor`, `limit`.
- **Respons `200`:** daftar `{ id, code, title, member_count, active_session_id }`.
- **Kode status:** `200`, `401`.
- **Efek database:** —
- **Idempotensi:** aman diulang.

#### `POST /api/v1/classes`

- **Peran:** operator, admin.
- **Tubuh permintaan:** `code` (wajib, unik), `title` (wajib).
- **Respons `201`:** objek kelas.
- **Kode status:** `201`, `400`, `409`.
- **Efek database:** `INSERT INTO classes`; audit `class.created`.
- **Idempotensi:** `code` unik membuat pengulangan menghasilkan `409`.

#### `GET /api/v1/classes/{class_id}/members`

- **Peran:** operator, admin.
- **Respons `200`:** daftar peserta aktif kelas dengan `person_id`, `external_code`, `display_name`.
- **Kode status:** `200`, `404`.
- **Efek database:** —
- **Idempotensi:** aman diulang.

#### `PUT /api/v1/classes/{class_id}/members/{person_id}`

- **Peran:** operator, admin.
- **Tubuh permintaan:** kosong.
- **Respons `201`** bila baru dibuat, `200` bila sudah menjadi anggota.
- **Kode status:** `200`, `201`, `404`.
- **Efek database:** `INSERT ... ON CONFLICT DO NOTHING` pada `class_memberships`; audit `class.member.added` hanya saat benar-benar bertambah.
- **Idempotensi:** **idempoten**; pengulangan tidak menghasilkan baris ganda.

#### `DELETE /api/v1/classes/{class_id}/members/{person_id}`

- **Peran:** operator, admin.
- **Respons:** `204`.
- **Kode status:** `204`, `404`, `409`.
- **Validasi:** keanggotaan tidak boleh dihapus bila peserta memiliki catatan kehadiran pada sesi **terbuka** untuk kelas itu (`409`).
- **Efek database:** `DELETE FROM class_memberships`; audit `class.member.removed`.
- **Idempotensi:** penghapusan kedua `404`.

### 5.6 Modul 6 — Sesi absensi

#### `GET /api/v1/sessions`

- **Peran:** operator, admin.
- **Query:** `class_id`, `status` (`draft`/`open`/`closed`), `from`, `to`, `cursor`, `limit`.
- **Respons `200`:** daftar sesi dengan ringkasan jumlah hadir.
- **Kode status:** `200`, `401`.
- **Efek database:** —
- **Idempotensi:** aman diulang.

#### `POST /api/v1/sessions`

- **Peran:** operator, admin.
- **Tubuh permintaan:** `class_id` (wajib), `device_id` (opsional), `closes_at` (wajib), `late_after` (opsional), `title` (opsional).
- **Respons `201`:** sesi berstatus `draft`.
- **Kode status:** `201`, `400`, `404`, `409`, `422`.
- **Validasi:** `closes_at` harus di masa depan dan setelah `late_after`; `device_id` tidak boleh sudah terikat sesi terbuka lain (`409 DEVICE_ALREADY_BOUND`); satu kelas maksimum satu sesi `open` (`409 SESSION_ALREADY_OPEN`).
- **Efek database:** `INSERT INTO attendance_sessions`; audit `session.created`.
- **Idempotensi:** `Idempotency-Key` dihormati untuk mencegah dua sesi ganda akibat klik dobel.

#### `GET /api/v1/sessions/{session_id}`

- **Peran:** operator, admin.
- **Respons `200`:** detail sesi termasuk `started_at`, `closes_at`, `late_after`, `status`, `device_id`, `counts` (`present`, `late`, `absent`, `pending_review`).
- **Kode status:** `200`, `404`.
- **Efek database:** —
- **Idempotensi:** aman diulang.

#### `POST /api/v1/sessions/{session_id}/open`

- **Peran:** operator, admin.
- **Tubuh permintaan:** kosong.
- **Respons `200`:** sesi berstatus `open` dengan `started_at`.
- **Kode status:** `200`, `404`, `409`, `422`.
- **Validasi:** hanya dari `draft`; `closes_at` belum lewat; perangkat terkait ada dan tidak dicabut.
- **Efek database:** `UPDATE attendance_sessions`; audit `session.opened`.
- **Idempotensi:** membuka sesi yang sudah `open` mengembalikan `409 SESSION_ALREADY_OPEN`.

#### `POST /api/v1/sessions/{session_id}/close`

- **Peran:** operator, admin.
- **Tubuh permintaan:** `reason` (opsional).
- **Respons `200`:** sesi berstatus `closed`.
- **Kode status:** `200`, `404`, `409`.
- **Efek database:** `UPDATE attendance_sessions.status='closed'`, `closed_at=now`; audit `session.closed`.
- **Idempotensi:** menutup sesi yang sudah tertutup mengembalikan `409 SESSION_CLOSED`.
- **Aturan penting:** sesudah tertutup, kiriman kamera tidak lagi boleh menambah catatan. Koreksi setelah penutupan hanya melalui §5.9 dan selalu diaudit.

### 5.7 Modul 7 dan 8 — Hasil verifikasi dan pencatatan kehadiran

Tidak ada endpoint publik terpisah. Keduanya dijalankan di dalam §5.10. Tabel keputusan domain dan statusnya ada di §7.

### 5.8 Modul 9 — Koreksi manual dan jalur alternatif absensi

#### `POST /api/v1/sessions/{session_id}/manual-checkins`

- **Peran:** operator, admin.
- **Tubuh permintaan:** `person_id` (wajib), `reason` (wajib, tidak boleh kosong, ≤200 karakter), `status` (wajib: `present`/`late`/`excused`), `checked_in_at` (opsional).
- **Respons `201`** bila catatan baru dibuat; `200` bila mengoreksi catatan yang sudah ada.
- **Kode status:** `201`, `200`, `400`, `404`, `409`, `422`.
- **Kesalahan:** `VALIDATION_FAILED`, `CLASS_MISMATCH`, `REASON_REQUIRED`, `NOT_FOUND`.
- **Validasi:** peserta wajib anggota kelas sesi. Koreksi **diperbolehkan** setelah sesi ditutup — inilah jalur yang menggantikan “peserta dianggap tidak hadir karena pengenalan gagal”.
- **Efek database:** `INSERT` atau `UPDATE attendance_records` dengan `source='manual'`, `created_by` = id operator, `reason` wajib; audit `attendance.manual_corrected` yang mencatat nilai sebelumnya dan sesudahnya.
- **Idempotensi:** `Idempotency-Key` wajib bermanfaat di sini; tanpa kunci, permintaan kedua dengan isi sama menghasilkan `409 ALREADY_CHECKED_IN` kecuali `checked_in_at` dikirim eksplisit untuk koreksi.
- **Penting:** endpoint ini **tidak boleh** dipakai perangkat dan tidak boleh ada versi publik tanpa autentikasi operator.

#### `GET /api/v1/sessions/{session_id}/manual-checkins`

- **Peran:** operator, admin.
- **Respons `200`:** daftar koreksi manual pada sesi tersebut, beserta aktor dan alasannya.
- **Kode status:** `200`, `404`.
- **Efek database:** —
- **Idempotensi:** aman diulang.

### 5.9 Modul 10 — Pendaftaran dan autentikasi perangkat

#### `POST /api/v1/devices`

- **Peran:** admin.
- **Tubuh permintaan:** `display_name` (wajib), `kind` (wajib: `webcam`/`esp32_cam`).
- **Respons `201`:** `{ "data": { "id", "display_name", "kind", "device_token" } }`. `device_token` ditampilkan **sekali** dan tidak dapat dibaca lagi.
- **Kode status:** `201`, `400`, `403`.
- **Efek database:** `INSERT INTO devices` dengan `token_hash` (bukan token mentah) dan `token_prefix` untuk identifikasi administratif; audit `device.created`.
- **Idempotensi:** pengulangan membuat perangkat baru dengan token berbeda. Ini disengaja agar admin sadar; klien tidak boleh retry otomatis.

#### `GET /api/v1/devices`

- **Peran:** operator, admin.
- **Respons `200`:** daftar `{ id, display_name, kind, revoked_at, last_seen_at, token_prefix }`.
- **Kode status:** `200`, `401`.
- **Efek database:** —
- **Idempotensi:** aman diulang.
- **Catatan:** `token_hash` dan `device_token` **tidak pernah** dikembalikan.

#### `POST /api/v1/devices/{device_id}/rotate-token`

- **Peran:** admin.
- **Respons `200`:** token baru ditampilkan sekali.
- **Kode status:** `200`, `403`, `404`.
- **Efek database:** memperbarui `token_hash`; audit `device.token_rotated`.
- **Idempotensi:** tidak; rotasi berulang menghasilkan token baru setiap kali.

#### `DELETE /api/v1/devices/{device_id}`

- **Peran:** admin.
- **Respons:** `204`.
- **Kode status:** `204`, `403`, `404`, `409`.
- **Validasi:** perangkat yang sedang terikat sesi `open` harus ditutup sesinya lebih dahulu (`409`).
- **Efek database:** mengisi `revoked_at` (pencabutan lunak) alih-alih menghapus baris, agar audit percobaan lama tetap dapat ditelusuri; audit `device.revoked`.
- **Idempotensi:** pencabutan kedua `404`.

### 5.10 Endpoint terminal kamera — pengenalan dan pencatatan

#### `POST /api/v1/devices/{device_id}/recognition-attempts`

- **Peran:** `device`.
- **Autentikasi:** header token perangkat. Token terikat pada `device_id` di jalur; ketidakcocokan menghasilkan `403`.
- **Konten:** `image/jpeg` biner. `Content-Length` wajib.
- **Header:** `X-Idempotency-Key` (wajib untuk aman terhadap retry), `X-Correlation-Id` (opsional), `X-Captured-At` (opsional, ISO 8601 UTC).
- **Tubuh:** kosong (gambar ada di badan biner). Bila memakai `multipart/form-data`, satu bagian bernama `image` plus field `session_id` opsional.
- **Batas awal (kandidat, wajib diukur, bukan dikira-kira — Q-18):** ukuran JPEG ≤1 MB, resolusi sisi terpanjang ≤1920 px, batas laju bawaan 2 percobaan/detik/perangkat.

**Respons `200` — permintaan teknis berhasil diproses.** Bentuk respons bersifat **tetap** untuk semua hasil; klien tidak boleh bergantung pada perkecualian.

```json
{
  "data": {
    "outcome": "recognized",
    "decision": "present",
    "attendance": {
      "id": "55555555-5555-4555-8555-555555555555",
      "session_id": "11111111-1111-4111-8111-111111111111",
      "person_id": "22222222-2222-4222-8222-222222222222",
      "display_name": "Peserta Demo",
      "status": "present",
      "source": "face",
      "checked_in_at": "2026-10-15T08:00:00Z"
    },
    "signal": { "kind": "green", "label": "Hadir" },
    "retry_after_hint_ms": null
  },
  "request_id": "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
}
```

```json
{
  "data": {
    "outcome": "retry",
    "decision": "no_attendance",
    "attendance": null,
    "signal": { "kind": "yellow", "label": "Coba lagi" },
    "retry_after_hint_ms": 1500
  },
  "request_id": "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
}
```

- **`signal.kind`:** hanya `green` (berhasil), `yellow` (coba lagi), `red` (gangguan/verifikasi manual). Perangkat memetakan ini ke LED; perangkat **tidak** menampilkan nama peserta (D-014).
- **`decision`:** `present` atau `no_attendance`. Nilai `decision` adalah **keputusan domain yang dibuat backend Go**, bukan saran mesin AI.
- **Kode status HTTP:** `200` untuk seluruh hasil pengenalan yang berhasil diproses oleh backend; `400`, `401`, `403`, `409`, `413`, `415`, `422`, `429`, `503`, `504` untuk kegagalan permintaan/infrastruktur.
- **Kesalahan yang tetap berupa HTTP:** `DEVICE_REVOKED` (403), `DEVICE_SESSION_MISMATCH` (422), `SESSION_NOT_OPEN` (422), `SESSION_CLOSED` (409), `IMAGE_TOO_LARGE` (413), `UNSUPPORTED_MEDIA_TYPE` (415), `IMAGE_DECODE_FAILED` (422), `AI_ENGINE_UNAVAILABLE` (503), `AI_ENGINE_TIMEOUT` (504), `RATE_LIMITED` (429).

**Semantik status `200` wajib dibedakan dari kegagalan:** “wajah tidak dikenali” adalah **hasil valid**, bukan error server. Perangkat harus dapat memakai satu jalur kode tunggal. Perbedaan tegasnya ada di §7.

- **Urutan pemeriksaan di backend Go (satu transaksi, urutan tetap):**

  1. Validasi token, `device_id`, dan status perangkat.
  2. Validasi jenis konten, ukuran, dan kemampuan dekode JPEG.
  3. Periksa `X-Idempotency-Key`: bila sudah pernah diproses, kembalikan hasil pertama tanpa efek samping baru.
  4. Periksa sesi terkait perangkat: harus `open` dan belum melewati `closes_at`.
  5. Panggil mesin AI internal (§6) untuk kualitas gambar, deteksi, dan embedding.
  6. **Bila ada tepat satu wajah**, minta pencocokan terhadap template aktif pada mesin AI atau bandingkan di backend memakai embedding yang dikembalikan.
  7. Periksa persetujuan biometrik aktif untuk kandidat teratas.
  8. Periksa keanggotaan kelas sesi.
  9. Periksa margin kecocokan/ambiguitas memakai ambang terkalibrasi.
  10. Periksa duplikasi: `UNIQUE(session_id, person_id)`.
  11. Tulis satu baris `attendance_records` dengan `source='face'`; tentukan `present` atau `late` berdasarkan `late_after`.
  12. Tambahkan `audit_events` `attendance.recorded`.
  13. Simpan hasil pada `idempotency_records` beserta respons ringkasnya.
  14. Kembalikan respons sesuai `outcome`.

- **Efek database:** lihat langkah 11–13. **Tidak ada** penyimpanan JPEG pada jalur ini; gambar hanya dipakai dalam memori permintaan.
- **Idempotensi:** `X-Idempotency-Key` **wajib**. Andaikan perangkat kehilangan balasan dan mencoba lagi dengan kunci sama, backend mengembalikan respons pertama (termasuk `checked_in_at` asli) dan **tidak** memperbarui waktu check-in.
- **Aturan keras:** endpoint ini **tidak pernah** menyimpan embedding ke database. Embedding hanya ada pada tabel `face_templates` hasil enrollment. Respons tidak boleh memuat embedding, skor, atau daftar kandidat.

### 5.11 Modul 11 — Rekap dan ekspor laporan

#### `GET /api/v1/sessions/{session_id}/attendance`

- **Peran:** operator, admin.
- **Query:** `cursor`, `limit`, `include` (opsional: `counts`).
- **Header:** respons menyertakan `ETag`. Klien boleh mengirim `If-None-Match` dan menerima `304` bila tidak ada perubahan (D-021).
- **Respons `200`:**

```json
{
  "data": [
    {
      "person_id": "22222222-2222-4222-8222-222222222222",
      "external_code": "DEMO-001",
      "display_name": "Peserta Demo",
      "status": "present",
      "source": "face",
      "checked_in_at": "2026-10-15T08:00:00Z",
      "reason": null
    }
  ],
  "counts": { "present": 4, "late": 1, "excused": 1, "absent": 3, "pending_review": 0 },
  "page": { "next_cursor": null, "has_more": false, "limit": 50 },
  "request_id": "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
}
```

- **Aturan penting:** peserta anggota kelas yang **belum** memiliki catatan hanya ditampilkan sebagai `absent` ketika sesi sudah `closed`. Selama sesi `open`, status mereka adalah `pending`. Ini mencegah aplikasi menyimpulkan “tidak hadir” hanya karena pengenalan wajah belum berhasil.
- **Kode status:** `200`, `304`, `404`.
- **Efek database:** —
- **Idempotensi:** aman diulang.

#### `GET /api/v1/sessions/{session_id}/export.csv`

- **Peran:** operator, admin.
- **Respons `200`:** `text/csv; charset=utf-8` dengan header `Content-Disposition`.
- **Kolom:** `external_code`, `display_name`, `status`, `source`, `checked_in_at`, `reason`, `session_title`, `class_code`.
- **Kode status:** `200`, `404`.
- **Efek database:** audit `session.exported` (jumlah baris, bukan isi).
- **Idempotensi:** aman diulang.
- **Aturan keras:** ekspor **tidak boleh** memuat embedding, skor kecocokan, jalur berkas gambar, atau ID template.

### 5.12 Modul 12 — Kesehatan dan kesiapan

#### `GET /api/v1/healthz`

- **Peran:** publik (dibatasi jaringan lokal).
- **Respons `200`:** `{ "status": "ok", "version": "<versi build>", "time": "2026-10-15T08:00:00Z" }`.
- **Kode status:** `200`.
- **Aturan keras:** tidak boleh memuat jalur berkas, versi pustaka internal, nama basis data, atau status perangkat.

#### `GET /api/v1/readyz`

- **Peran:** operator, admin.
- **Respons `200`:** `{ "database": "ok", "ai_engine": "ok", "migrations": "applied", "wal": { "journal_mode": "wal" } }`.
- **Respons `503`:** salah satu komponen `unavailable` dengan `code` `AI_ENGINE_UNAVAILABLE` atau `DATABASE_BUSY`.
- **Kode status:** `200`, `503`.
- **Efek database:** pembacaan ringan; **tidak** menjalankan `PRAGMA integrity_check` penuh pada setiap permintaan.
- **Idempotensi:** aman diulang.

### 5.13 Modul 13 — Audit aktivitas penting

#### `GET /api/v1/audit-events`

- **Peran:** operator, admin.
- **Query:** `actor_id`, `action`, `entity_type`, `entity_id`, `from`, `to`, `cursor`, `limit`.
- **Respons `200`:** daftar `{ id, actor_type, actor_id, action, entity_type, entity_id, time, reason, safe_metadata }`.
- **Kode status:** `200`, `400`, `403`.
- **Aturan keras:** `safe_metadata` tidak boleh memuat JPEG, embedding, token, kata sandi, atau data biometrik. `reason` memuat alasan manusia untuk koreksi manual.
- **Efek database:** —
- **Idempotensi:** aman diulang.
- **Aksi yang wajib diaudit (minimum):** `auth.login.*`, `auth.logout`, `auth.session.revoked`, `user.created`, `person.created`, `person.updated`, `person.deleted`, `consent.granted`, `consent.withdrawn`, `enrollment.started`, `enrollment.completed`, `template.deleted`, `class.created`, `class.member.added`, `class.member.removed`, `session.created`, `session.opened`, `session.closed`, `attendance.recorded`, `attendance.manual_corrected`, `session.exported`, `device.created`, `device.token_rotated`, `device.revoked`, `migration.applied`.

## 6. Kontrak internal Go ↔ mesin AI Python

Bagian ini adalah kontrak **internal**, bukan API publik. Endpoint di bawah tidak boleh dirutekan melalui API publik dan tidak boleh dapat diakses dari jaringan.

### 6.1 Batas keamanan dan pengikatan alamat

1. Layanan Python **hanya** mendengarkan pada `127.0.0.1` (loopback). Jangan `0.0.0.0`, jangan alamat LAN.
2. Port dipilih dari rentang lokal dan **dapat dikonfigurasi** lewat berkas konfigurasi lokal; jangan ditanam permanen di kode.
3. Antara Go dan Python berlaku **token internal bersama** yang dibuat saat inisialisasi sistem, disimpan di lokasi konfigurasi lokal yang dilindungi ACL, dan **tidak** berada di Git. Token ini menutup risiko proses lokal lain memanggil mesin AI.
4. **Loopback HTTP bukan jaminan aman.** Bila kelak layanan Python dipisahkan ke mesin lain, pengikatan `127.0.0.1` tidak berlaku lagi dan wajib diganti mekanisme yang lebih kuat (mis. mTLS atau IPC terautentikasi) **sebelum** pemisahan dilakukan. Kontrak ini tidak boleh ditafsirkan sebagai izin memindahkan layanan tanpa perubahan.
5. Endpoint internal tidak boleh mengembalikan data yang memungkinkan pihak ketiga menebak identitas; pencocokan mengembalikan skor dan ID template, bukan identitas orang.

### 6.2 Jalur internal

| Metode | Jalur | Tujuan |
|---|---|---|
| `GET` | `/internal/v1/health` | Kesiapan dan versi model |
| `POST` | `/internal/v1/face/detect` | Deteksi wajah dan penilaian kualitas |
| `POST` | `/internal/v1/face/embed` | Ekstraksi embedding dari satu wajah |
| `POST` | `/internal/v1/face/match` | Pencocokan embedding permintaan terhadap himpunan template yang dikirim backend |
| `POST` | `/internal/v1/face/verify` | Deteksi + embedding + pencocokan dalam satu panggilan (jalur utama terminal) |

### 6.3 Format masukan

- **Konten:** `image/jpeg` biner untuk endpoint yang menerima gambar; JSON untuk endpoint yang hanya menerima vektor.
- **Header wajib:** `Content-Type`, `Content-Length`, `X-Correlation-Id`, `X-Model-Revision` (opsional; untuk penelusuran).
- **Metadata konteks yang boleh dikirim:** `correlation_id`, `device_id` (UUID), `session_id` (UUID), `captured_at` (ISO 8601 UTC), `purpose` (`enroll`/`verify`).
- **Metadata yang dilarang dikirim ke mesin AI:** nama peserta, `external_code`, email, nomor telepon, atau dokumen identitas apa pun. Mesin AI tidak perlu tahu identitas, dan tidak boleh mengetahuinya.
- **Validasi ukuran di sisi Python (pertahanan berlapis):** jenis JPEG, batas ukuran, dan batas resolusi diperiksa **sekali lagi** di Python meskipun Go sudah memeriksanya.

### 6.4 Format keluaran

`POST /internal/v1/face/verify` mengembalikan:

```json
{
  "result": "ok",
  "faces_detected": 1,
  "quality": { "status": "accepted", "score": 0.71, "issues": [] },
  "embedding": [0.013, -0.004, "..."],
  "matches": [
    { "template_id": "66666666-6666-4666-8666-666666666666", "score": 0.52, "metric": "cosine" },
    { "template_id": "77777777-7777-4777-8777-777777777777", "score": 0.31, "metric": "cosine" }
  ],
  "model": { "id": "sface", "version": "<versi berkas bobot>", "detector": "yunet", "detector_version": "<versi>" },
  "timing_ms": { "detect": 38, "embed": 91, "match": 12 }
}
```

Aturan keluaran:

- `result` bernilai `ok` atau salah satu kode kegagalan §7.
- `faces_detected` adalah jumlah wajah yang terdeteksi, **bukan** identitas.
- `matches` memuat **hanya** `template_id` dan skor — bukan nama, bukan `external_code`.
- `embedding` dikembalikan hanya karena backend Go membutuhkannya untuk memutuskan; nilai ini **tidak boleh** dicatat ke log, tidak boleh dikembalikan ke klien, dan tidak boleh disimpan kecuali di dalam `face_templates` terenkripsi.
- Ambang penerimaan **tidak** ditentukan mesin AI. Mesin AI mengembalikan skor; ambang, margin, dan keputusan ada di Go. Ini menjaga D-027.

### 6.5 Versi model dan metadata

1. Mesin AI mengembalikan `model.id`, `model.version`, dan identitas detektor pada setiap respons.
2. Backend Go membandingkannya dengan `face_templates.model_id`/`model_version`. Ketidakcocokan **tidak** boleh dicampur diam-diam: baris template dengan versi berbeda tidak boleh dipakai untuk pencocokan sampai keputusan eksplisit diambil.
3. Versi dan checksum berkas bobot dicatat di repositori dokumentasi (bukan berkas bobotnya) setelah verifikasi lisensi.

### 6.6 Batas waktu, kegagalan, dan percobaan ulang

| Keadaan | Perilaku Go | Balasan ke klien |
|---|---|---|
| Python tidak berjalan | Gagal cepat; tidak mencoba berulang tanpa batas | `503 AI_ENGINE_UNAVAILABLE` |
| Python melewati batas waktu (usulan awal 3 detik untuk `verify`) | Batalkan permintaan | `504 AI_ENGINE_TIMEOUT` |
| Python mengembalikan `faces_detected = 0` | Hasil valid | `200` dengan `outcome: "retry"` |
| Python mengembalikan `faces_detected > 1` | Hasil valid | `200` dengan `outcome: "retry"` |
| Kualitas tidak memadai | Hasil valid | `200` dengan `outcome: "retry"` |
| Python mengembalikan kesalahan internal | Satu percobaan ulang cepat, lalu gagal | `503 AI_ENGINE_UNAVAILABLE` |
| Database terkunci melewati `busy_timeout` | Tidak ada penulisan parsial | `503 DATABASE_BUSY` |

Aturan percobaan ulang:

- Percobaan ulang hanya untuk kesalahan **idempoten** (koneksi gagal, timeout). Percobaan ulang **tidak** boleh dilakukan bila jawaban sudah diterima.
- Maksimum satu percobaan ulang cepat untuk `verify`; tidak ada backoff panjang yang menahan permintaan.
- Klien perangkat melakukan percobaan ulang gambar dengan `X-Idempotency-Key` **baru** hanya bila gambar diambil ulang; kunci lama dipakai ulang bila gambar yang sama dikirim lagi.

### 6.7 Perlindungan data pada komunikasi internal

1. Gambar hanya hidup di memori dan dibuang setelah respons; tidak ada penulisan sementara ke disk secara bawaan.
2. Bila suatu saat penyimpanan sementara memang diperlukan, berkas wajib dibuat di direktori terlindungi, dihapus segera, dan tidak boleh berada di jalur yang dipantau sinkronisasi awan.
3. Log mesin AI **tidak boleh** memuat gambar, embedding, atau identitas. Log boleh memuat `correlation_id`, `session_id`, waktu, kode hasil, dan durasi.
4. Embedding yang dikirim melalui loopback tetap dianggap data sensitif: jangan pernah mencetaknya, jangan pernah mengirimnya ke layanan eksternal, jangan pernah menyimpannya di repositori.

### 6.8 Pemisahan tanggung jawab

| Tanggung jawab | Pemilik |
|---|---|
| Deteksi wajah, kualitas gambar, embedding, skor pencocokan | Python |
| Keputusan hadir/tidak hadir, penerapan ambang dan margin | **Go** (D-027) |
| Pemeriksaan konsen, keanggotaan kelas, status sesi, perangkat | **Go** |
| Penulisan seluruh tabel database | **Go** |
| Idempotensi dan audit | **Go** |
| Penyimpanan template terenkripsi | **Go** (kunci di luar database dan di luar Git; bentuk penyimpanan AES-256-GCM final, pengelolaan kunci diusulkan pada **D-030 (USULAN)** dan dirinci di `docs/SECURITY_PRIVACY.md` §5a–§5d) |

## 7. Status kegagalan pengenalan wajah dan pemisahannya dari error HTTP

Sepuluh kondisi yang diminta, dipisahkan menjadi **status teknis permintaan** dan **keputusan domain absensi**. Aturan besar: **kegagalan mengenali wajah bukan kegagalan permintaan.** Selama backend dapat memproses gambar dan memeriksa aturan, balasannya `200` dengan `decision: "no_attendance"`.

| # | `outcome` | Makna operasional | HTTP | `decision` | `signal` | Catatan |
|---|---|---|---|---|---|---|
| 1 | `no_face` | Tidak ada wajah terdeteksi | `200` | `no_attendance` | `yellow` | Bukan error; minta peserta masuk bingkai |
| 2 | `multiple_faces` | Lebih dari satu wajah | `200` | `no_attendance` | `yellow` | Kios satu orang per pemindaian (D-014) |
| 3 | `low_quality` | Ketajaman/pencahayaan/ukuran wajah tidak memadai | `200` | `no_attendance` | `yellow` | Termasuk kasus kacamata memantulkan cahaya |
| 4 | `unknown_face` | Tidak ada kecocokan yang memenuhi ambang | `200` | `no_attendance` | `yellow` | **Tidak** berarti orang itu tidak hadir |
| 5 | `ambiguous_match` | Dua kandidat atau lebih terlalu berdekatan | `200` | `no_attendance` | `red` | Wajib verifikasi manual |
| 6 | `liveness_not_satisfied` | Pemeriksaan liveness terpisah tidak lolos atau tidak tersedia | `200` | `no_attendance` | `red` | D-029: mekanisme terpisah, tidak ditawarkan YuNet/SFace |
| 7 | `session_inactive` | Sesi belum dibuka atau sudah ditutup | `409`/`422` | `no_attendance` | `red` | **Error HTTP**: keadaan diminta tidak berlaku |
| 8 | `consent_missing` | Kandidat tidak memiliki persetujuan biometrik aktif | `200` | `no_attendance` | `red` | Sistem sengaja **tidak** memberi tahu “wajah Anda dikenali tetapi tidak berizin”; pesan ke perangkat tetap umum |
| 9 | `already_recorded` | Kandidat sudah tercatat pada sesi itu | `200` | `no_attendance`+`already` | `green` | Catatan pertama dipertahankan; waktu check-in tidak diubah |
| 10 | `ai_engine_disturbed` | Mesin AI tidak tersedia, gagal, atau melewati batas waktu | `503`/`504` | `no_attendance` | `red` | Satu-satunya kondisi yang benar-benar gangguan sistem |

Catatan tambahan yang harus dibaca bersama tabel:

- `no_face`, `multiple_faces`, `low_quality`, `unknown_face`, `ambiguous_match`, `consent_missing`, `already_recorded`, dan `liveness_not_satisfied` **bukan** kegagalan server. Membalasnya dengan `5xx` akan membuat perangkat menandai sistem rusak padahal sistem bekerja normal.
- `session_inactive`, `image_too_large`, `unsupported_media_type`, dan `rate_limited` adalah **kesalahan permintaan** dan tetap memakai kode HTTP bukan-`200`.
- `already_recorded` memakai `signal: "green"` karena bagi peserta dan perangkat, hasilnya benar: kehadiran sudah tercatat.
- **`decision` tetap milik Go.** Mesin AI hanya mengembalikan `result`, `faces_detected`, `quality`, dan `matches` (§6.4); ia tidak pernah mengembalikan kata “hadir”.
- Untuk `unknown_face`, backend **tidak** menyimpan foto atau embedding orang tak dikenal. Tidak ada penumpukan data biometrik orang yang tidak mendaftar.
- `pending_review` dihitung pada rekap adalah keadaan **turunan**: peserta yang tidak punya catatan pada sesi yang sudah ditutup dan tidak dikoreksi manual. Nilai ini tidak pernah ditulis ke `attendance_records` sebagai keputusan “tidak hadir”.

## 8. Idempotensi dan transaksi

1. **Kunci idempotensi** dikirim klien pada header `Idempotency-Key` (operator) atau `X-Idempotency-Key` (perangkat), berupa string 8–128 karakter. Kunci disimpan pada `idempotency_records` bersama hash permintaan, kode status, dan badan respons ringkas.
2. Kunci yang sama dengan hash permintaan yang sama mengembalikan **respons pertama** tanpa efek samping baru.
3. Kunci yang sama dengan hash permintaan **berbeda** mengembalikan `409 IDEMPOTENCY_KEY_REUSED`.
4. Kunci disimpan maksimum 24 jam untuk menahan retry jaringan; setelah itu dihapus oleh proses pemeliharaan. **Database tetap menjadi penjaga terakhir**, bukan tabel kunci ini.
5. `UNIQUE(session_id, person_id)` pada `attendance_records` adalah penjaga terakhir terhadap absensi ganda. Aplikasi Go boleh memeriksa lebih dulu, tetapi pemeriksaan aplikasi saja **tidak cukup**.
6. Pemeriksaan konsen, keanggotaan kelas, dan status sesi dilakukan **di dalam** transaksi penulisan, bukan hanya pada awal permintaan. Sesi dapat ditutup atau konsen dicabut di antara dua langkah.
7. Bila penulisan gagal karena batasan unik, transaksi di-rollback dan backend membalas `already_recorded` (`200`) dengan mengambil catatan yang sudah ada — bukan `500`.
8. Waktu `checked_in_at` **tidak** diperbarui oleh percobaan ulang.
9. Penutupan sesi bersifat idempoten pada keadaan akhir (sesi tertutup), tetapi permintaan kedua tetap membalas `409` agar operator mengetahui klik berulang.

## 9. Pengujian kontrak wajib

Daftar berikut harus terwujud sebagai pengujian saat implementasi. **Belum ada pengujian yang dijalankan pada FL-003** karena belum ada kode aplikasi.

| # | Skenario | Harapan |
|---|---|---|
| 1 | Operator tanpa token memanggil `/people` | `401 UNAUTHENTICATED` |
| 2 | Operator memanggil `/users` | `403 FORBIDDEN_ROLE` |
| 3 | Perangkat memanggil `/sessions/{id}/attendance` | `403 FORBIDDEN_ROLE` |
| 4 | Persetujuan dicabut sebelum pengenalan | `200`, `outcome: "consent_missing"`, tidak ada template tersisa |
| 5 | Pengenalan ketika sesi sudah ditutup | `409 SESSION_CLOSED`, tanpa catatan baru |
| 6 | Dua kiriman simultan untuk peserta yang sama | Satu baris; yang kedua `already_recorded` |
| 7 | Kiriman ulang dengan `X-Idempotency-Key` sama | Respons pertama dikembalikan; waktu check-in tidak berubah |
| 8 | Kunci idempotensi sama dengan isi berbeda | `409 IDEMPOTENCY_KEY_REUSED` |
| 9 | JPEG terpotong | `422 IMAGE_DECODE_FAILED` |
| 10 | JPEG 4 MB | `413 IMAGE_TOO_LARGE` |
| 11 | `Content-Type: image/png` | `415 UNSUPPORTED_MEDIA_TYPE` |
| 12 | Wajah orang tak terdaftar | `200`, `outcome: "unknown_face"`, tanpa penulisan |
| 13 | Dua wajah dalam satu bingkai | `200`, `outcome: "multiple_faces"` |
| 14 | Mesin AI dihentikan | `503 AI_ENGINE_UNAVAILABLE`, tidak ada catatan |
| 15 | Mesin AI lambat melampaui batas waktu | `504 AI_ENGINE_TIMEOUT` |
| 16 | Database terkunci | `503 DATABASE_BUSY`, tidak ada penulisan parsial |
| 17 | Ekspor CSV | Tidak ada kolom embedding/skor/jalur gambar |
| 18 | Peserta bukan anggota kelas sesi | `200`, `outcome` tanpa catatan, alasan `CLASS_MISMATCH` pada audit |
| 19 | Koreksi manual tanpa alasan | `400`, `REASON_REQUIRED` |
| 20 | Koreksi manual peserta di luar kelas | `422 CLASS_MISMATCH` |
| 21 | Pemanggilan ulang setelah koneksi mobile putus | Tidak ada duplikasi; `304` bila `ETag` cocok |
| 22 | Perangkat dicabut mengirim gambar | `403 DEVICE_REVOKED` |
| 23 | Perangkat sesi A mengirim dengan `session_id` sesi B | `422 DEVICE_SESSION_MISMATCH` |
| 24 | Dua permintaan menutup sesi bersamaan | Satu berhasil; yang kedua `409 SESSION_CLOSED` |
| 25 | `If-None-Match` yang cocok pada rekap | `304` tanpa badan |

## 10. Hal yang belum disetujui atau menunggu keputusan

1. **Pengelolaan kunci biometrik (Q-16).** Kebijakan lengkap ada sebagai **D-030 (DISETUJUI pada FL-004S)** di `docs/DECISIONS.md` dan `docs/SECURITY_PRIVACY.md` §5a–§5f. **Status: disetujui atas rancangan, belum diimplementasikan dan belum diuji**, sehingga tabel `face_templates` masih tidak boleh ditulis sebelum ada tugas implementasi tersendiri yang disetujui pemilik project — jangan menganggap persetujuan rancangan sebagai izin menulis kode.
2. **Angka batas** ukuran JPEG, resolusi maksimum, batas laju, dan `retry_after_hint_ms` masih kandidat dan harus **diukur** (Q-18), bukan ditetapkan dari perkiraan.
3. **Ambang kecocokan dan margin** belum ada (Q-07). Kontrak ini hanya menetapkan bahwa ambangnya milik Go dan harus terkalibrasi.
4. **Masa berlaku token** operator dan perangkat, serta kebijakan rotasi, masih perlu keputusan.
5. **Akun peserta** (D-013 masih USULAN) akan menambah modul autentikasi kedua bila disetujui.
6. **Liveness** tidak dijanjikan ada pada MVP (D-029). Bila mekanisme dipilih, ia menambah endpoint atau kolom baru.
7. **Halaman pengelolaan perangkat di aplikasi** belum masuk PRD; endpointnya sudah disiapkan agar tidak menghambat.
8. **Kebijakan retensi catatan kehadiran** dan hubungannya dengan penghapusan peserta belum diputuskan (`docs/OPEN_QUESTIONS.md`).
9. **WebSocket** belum diaktifkan (D-021); bila kebutuhan muncul, ia akan menjadi modul baru, bukan perubahan pada endpoint rekap.

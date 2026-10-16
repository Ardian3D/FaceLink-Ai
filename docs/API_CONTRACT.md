# Kontrak API FaceLink AI

**Versi:** 1.0 · **Revisi:** FL-010V (16 Oktober 2026) · **Status:** SPESIFIKASI FINAL TAHAP DOKUMENTASI; **BELUM DIIMPLEMENTASIKAN** dan belum menjadi berkas OpenAPI yang dapat dijalankan.

Dokumen ini adalah acuan wajib yang harus ditinjau sebelum membuat handler Go atau layar React Native yang memanggilnya. Kontrak ini **mengikuti** keputusan yang sudah disetujui di `docs/DECISIONS.md` (D-010, D-020, D-021, D-022, D-027, D-028, D-031–D-037) dan **tidak menggantikan** `docs/PRD.md`.

**Tidak ada kode aplikasi yang ditulis pada tugas ini.** Seluruh contoh di bawah adalah ilustrasi bentuk data, bukan bukti endpoint sudah berjalan.

> **Revisi FL-010U (16 Oktober 2026), cabang `docs/fl-010u-google-three-roles` dari `origin/main` (`804c2f7`).**
> Revisi ini menyelaraskan kontrak dengan arsitektur tiga role manusia (`admin`, `dosen`, `mahasiswa`) + Google Sign-In, dan `device` sebagai autentikasi mesin yang terpisah. Perubahan bersifat **dokumentasi saja**: tidak ada kode Go, TypeScript, SQL, atau berkas migrasi yang ditulis pada FL-010U. Seluruh skenario runtime di dokumen ini berstatus **BELUM TERVERIFIKASI**. Kontrak kanonik revisi ini ada di dalam repo: matriks peran/izin tunggal pada `docs/AUTHORIZATION.md` §4, ketentuan sesi pada `docs/DECISIONS.md` D-031–D-033, dan tabel ketersediaan layanan pada `docs/ARCHITECTURE.md` §7.1.

> **Revisi FL-010V (16 Oktober 2026), cabang `docs/fl-010v-final-auth-decisions`.**
> Revisi ini mencatat **persetujuan final** pemilik project atas keputusan autentikasi dan otorisasi: **D-032** (lifecycle akun), **D-033** (session management), **D-034** (hak akses `dosen`), **D-035** (akses jaringan lokal — **disetujui pada tingkat kebijakan**), **D-036** (perluasan daftar minimum audit keamanan — **kini BERLAKU**), dan **D-037** (satu role utama aktif per akun — disetujui untuk MVP); **D-031 tetap DISETUJUI**. Perubahan pada FL-010V bersifat **dokumentasi saja**: tidak ada kode Go, TypeScript, SQL, atau berkas migrasi yang ditulis.
>
> **Tidak ada endpoint yang berubah pada FL-010V.** Matriks 53 endpoint (§4) beserta metode, jalur, dan himpunan perannya **tidak diubah**; nama peran, nama tabel, dan kode kesalahan stabil juga tidak diubah. Yang ditambahkan hanyalah aturan dan catatan yang mengikat di dalam kontrak: transport dan jaringan (§1.6), aturan nonce dan batas absolut sesi (§5.1), lifecycle akun (§5.2), perluasan audit (§5.13), dan daftar hal yang masih menunggu keputusan (§10). Kontrak kanonik revisi ini ada di dalam repo: keputusan pada `docs/DECISIONS.md` D-031–D-037, matriks peran/izin tunggal pada `docs/AUTHORIZATION.md` §4, kebijakan audit pada `docs/SECURITY_PRIVACY.md` §6, dan detail transport pada `docs/ARCHITECTURE.md` §5.1. Seluruh skenario runtime di dokumen ini tetap berstatus **BELUM TERVERIFIKASI**.

## 1. Aturan umum

### 1.1 Prinsip dasar

1. Seluruh API publik berada di bawah awalan **`/api/v1`**. Tidak ada endpoint publik di luar awalan itu, kecuali pemeriksaan kesehatan minimum pada §5.12 yang boleh tetap tersedia tanpa versi untuk kebutuhan probe lokal.
2. **Backend Go adalah satu-satunya penulis database dan satu-satunya pengambil keputusan domain absensi** (D-027). Mesin AI Python tidak pernah menetapkan peserta sebagai hadir dan tidak pernah menulis tabel kehadiran.
3. **UI tidak menentukan kehadiran** (D-028). Aplikasi React Native hanya menampilkan hasil dan mengirim permintaan manual melalui endpoint yang diaudit.
4. Format pertukaran normal: **JSON UTF-8**. Unggahan gambar: **`image/jpeg`** biner atau `multipart/form-data` sesuai endpoint.
5. Identitas record: **UUID v4 tanpa makna** (bukan NIK atau nomor absen). Waktu: **ISO 8601 UTC** dengan akhiran `Z` (`2026-10-15T08:00:00Z`); aplikasi menampilkan zona lokal pengguna.
6. Setiap respons menyertakan `request_id` yang tidak mengandung data pribadi, untuk penelusuran log.
7. **Identitas pengguna manusia ditegakkan lewat Google Sign-In (OpenID Connect).** Aplikasi React Native memperoleh Google ID token di sisi perangkat dan menukarkannya di `POST /auth/google`; backend memverifikasi token itu lalu menerbitkan sesi FaceLink AI sendiri (token opak, §5.1). **Tidak ada kata sandi internal**, tidak ada kata sandi sementara, tidak ada `must_change_password`, dan tidak ada hash kata sandi pada kontrak ini.
8. **Backend Go tetap pemilik otorisasi.** Role, status akun, dan cakupan kelas selalu dibaca dari database pada setiap permintaan; role tidak pernah diambil dari Google, dari klien, atau dari klaim token. Google hanya membuktikan identitas, **bukan** hak akses.
9. Semua endpoint selain `POST /auth/google` dan pemeriksaan kesehatan memerlukan autentikasi. Token sesi pengguna (admin/dosen/mahasiswa) dan token perangkat **terpisah** dan tidak dapat saling menggantikan.
10. Respons tidak boleh membocorkan embedding, gambar mentah, Google ID token, klaim `sub`, token sesi, atau dugaan identitas yang meragukan.
11. Jangan pernah mencatat payload biometrik, JPEG, token, atau kunci ke log.
12. Validasi yang wajib ada di setiap endpoint unggahan: jenis konten, ukuran maksimum, dan batas waktu proses.

### 1.2 Peran dan cakupan akses

| Peran | Jenis | Kredensial | Cakupan MVP |
|---|---|---|---|
| `admin` | Manusia | Google Sign-In | Seluruh sistem: akun & role, kelas + penugasan dosen, peserta, konsen, enrollment, perangkat, sesi, rekap, ekspor, audit |
| `dosen` | Manusia | Google Sign-In | **Hanya kelas yang ditugaskan kepadanya** (lihat §5.5, §5.6, §5.8); kelas di luar penugasan → `404 NOT_FOUND` |
| `mahasiswa` | Manusia | Google Sign-In | **Hanya data miliknya sendiri** (§5.14); memerlukan penautan akun ke baris `people` |
| `device` | Mesin | Token perangkat (bukan Google) | Hanya `POST /api/v1/devices/{device_id}/recognition-attempts` |
| `ai-engine` (internal) | Proses lokal | Token internal loopback | Hanya endpoint internal §6 pada `127.0.0.1`; bukan peran API publik |

**Ketiga role manusia (`admin`, `dosen`, `mahasiswa`) adalah daftar final MVP.** Role `operator` lama **DIGANTIKAN** oleh `dosen` dan `admin`: cakupan `operator` lama terbagi ke keduanya, sehingga ini **bukan penggantian nama dengan cakupan identik** (dinyatakan eksplisit di `docs/AUTHORIZATION.md`). Keputusan lama "mahasiswa tidak memiliki akun pada MVP" juga **DIGANTIKAN** — akun mahasiswa kini ada dan dikelola admin. Persona "operator/dosen" pada `docs/PRD.md` §3 kini dipenuhi oleh `dosen` dan `admin`, bukan oleh role `operator`. Matriks `endpoint × peran` lengkap, aturan `401`/`403`, dan aturan audit penolakan ada di `docs/AUTHORIZATION.md`.

Dua jalur autentikasi yang berbeda — sesi pengguna (Google Sign-In) dan token perangkat — **tidak dapat saling menggantikan**. Role dan cakupan kelas selalu diputuskan backend Go, bukan oleh Google maupun klien.

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

**Kesenjangan bentuk galat yang dicatat apa adanya (FL-011, 16 Oktober 2026).** Amplop di atas dipakai **seluruh endpoint FL-011** dan lapisan respons baru (`internal/httpapi/respond.go`). Namun **`GET /api/v1/healthz`, `404` rute tidak dikenal, dan `405` metode tidak diizinkan masih memakai bentuk datar lama** (`{ "code", "message", "request_id" }` tanpa pembungkus `error`), warisan FL-005 yang belum diseragamkan. Akibatnya klien **tidak boleh** mengasumsikan satu bentuk galat tunggal untuk seluruh API: klien yang mem-parse `error.code` akan gagal membaca galat dari ketiga jalur itu, dan klien yang mem-parse bentuk datar akan gagal membaca galat dari endpoint FL-011. Penyeragaman bentuk galat ini **belum dikerjakan** dan dicatat sebagai sisa pekerjaan pada §10.

### 1.5 Kode status HTTP dan kode kesalahan stabil

| HTTP | Kapan dipakai | Contoh `code` |
|---|---|---|
| `200` | Permintaan berhasil dibaca atau memperbarui data | — |
| `201` | Sumber daya baru dibuat | — |
| `204` | Penghapusan/penutupan berhasil tanpa isi | — |
| `304` | `If-None-Match` cocok, tidak ada perubahan | — |
| `400` | JSON rusak, tipe salah, parameter wajib hilang | `MALFORMED_JSON`, `MISSING_FIELD` |
| `401` | Belum terautentikasi, token tidak sah, atau sesi dicabut | `UNAUTHENTICATED`, `TOKEN_EXPIRED`, `IDENTITY_TOKEN_INVALID`, `SESSION_REVOKED` |
| `403` | Terautentikasi tetapi tidak berwenang | `FORBIDDEN_ROLE`, `DEVICE_REVOKED`, `IDENTITY_NOT_LINKED` |
| `404` | Sumber daya tidak ada atau tidak terlihat oleh peran ini | `NOT_FOUND` |
| `405` | Metode tidak diizinkan pada jalur tersebut | `METHOD_NOT_ALLOWED` |
| `409` | Konflik keadaan atau operasi berulang | `ALREADY_CHECKED_IN`, `SESSION_CLOSED`, `ACCOUNT_NOT_LINKED`, `LAST_ADMIN_PROTECTED`, `SELF_ACCOUNT_PROTECTED` |
| `412` | `If-Match`/prasyarat versi tidak cocok | `PRECONDITION_FAILED` |
| `413` | JPEG melampaui batas ukuran | `IMAGE_TOO_LARGE` |
| `415` | Jenis konten bukan JPEG yang diterima | `UNSUPPORTED_MEDIA_TYPE` |
| `422` | JSON sah tetapi melanggar aturan domain | `CONSENT_REQUIRED`, `CLASS_MISMATCH` |
| `429` | Melampaui batas laju | `RATE_LIMITED` |
| `500` | Kesalahan tak terduga di backend | `INTERNAL_ERROR` |
| `503` | Ketergantungan tidak tersedia | `AI_ENGINE_UNAVAILABLE`, `GOOGLE_UNAVAILABLE`, `DATABASE_BUSY` |
| `504` | Ketergantungan melewati batas waktu | `AI_ENGINE_TIMEOUT` |

Kode kesalahan stabil minimum yang dipakai lintas modul:

| `code` | HTTP | Makna |
|---|---|---|
| `VALIDATION_FAILED` | 400/422 | Satu atau beberapa field tidak memenuhi aturan |
| `UNAUTHENTICATED` | 401 | Tidak ada atau token tidak sah |
| `TOKEN_EXPIRED` | 401 | Token melewati masa berlaku |
| `IDENTITY_TOKEN_INVALID` | 401 | Google ID token gagal diverifikasi (issuer, audience, tanda tangan, kedaluwarsa, nonce) |
| `SESSION_REVOKED` | 401 | Sesi sudah dicabut (logout oleh admin, penonaktifan akun, atau deteksi pemakaian ulang penyegar) |
| `IDENTITY_NOT_LINKED` | 403 | Identitas tidak tertaut akun / akun belum diberi akses — **satu pesan generik** untuk semua sebab (anti-enumeration) |
| `FORBIDDEN_ROLE` | 403 | Peran tidak memiliki akses |
| `NOT_FOUND` | 404 | Sumber daya tidak ditemukan (termasuk kelas di luar penugasan `dosen`) |
| `CONSENT_REQUIRED` | 422 | Peserta tidak memiliki persetujuan biometrik aktif |
| `CONSENT_WITHDRAWN` | 422 | Persetujuan dicabut; template dihapus |
| `SESSION_NOT_OPEN` | 422 | Sesi bukan dalam keadaan terbuka |
| `SESSION_CLOSED` | 409 | Sesi sudah ditutup saat penulisan dilakukan |
| `CLASS_MISMATCH` | 422 | Peserta bukan anggota kelas sesi |
| `ALREADY_CHECKED_IN` | 409 | Sudah ada catatan kehadiran pada sesi yang sama |
| `ACCOUNT_NOT_LINKED` | 409 | Akun `mahasiswa` belum ditautkan ke baris peserta `people` |
| `LAST_ADMIN_PROTECTED` | 409 | Operasi akan menghilangkan admin aktif terakhir |
| `SELF_ACCOUNT_PROTECTED` | 409 | Admin mencoba menonaktifkan atau menurunkan peran akunnya sendiri |
| `DEVICE_REVOKED` | 403 | Perangkat dicabut atau tidak aktif |
| `DEVICE_SESSION_MISMATCH` | 422 | Perangkat tidak terhubung ke sesi yang dikirim |
| `IMAGE_TOO_LARGE` | 413 | Melebihi batas ukuran |
| `UNSUPPORTED_MEDIA_TYPE` | 415 | Bukan JPEG yang diterima |
| `IMAGE_DECODE_FAILED` | 422 | JPEG rusak atau tidak dapat dibaca |
| `RATE_LIMITED` | 429 | Terlalu banyak permintaan |
| `AI_ENGINE_UNAVAILABLE` | 503 | Pekerja AI tidak dapat dihubungi |
| `AI_ENGINE_TIMEOUT` | 504 | Pekerja AI melewati batas waktu |
| `GOOGLE_UNAVAILABLE` | 503 | Kunci atau penyedia identitas Google tidak dapat dihubungi |
| `DATABASE_BUSY` | 503 | Database terkunci melewati batas tunggu |
| `INTERNAL_ERROR` | 500 | Kesalahan tak terduga |

### 1.6 Transport dan jaringan (FL-010V)

Aturan berikut menetapkan **transport dan pengikatan jaringan** untuk komunikasi aplikasi → backend. Ketentuan ini bersifat **kebijakan (D-035, disetujui pada tingkat kebijakan pada FL-010V)**; rincian pelaksanaan sertifikat belum diuji. Tidak ada endpoint yang berubah oleh aturan ini — ia hanya mengikat bagaimana endpoint yang sudah ada dijangkau.

1. **Default development adalah loopback** (`127.0.0.1`). Tidak ada nilai bawaan yang membuka backend ke antarmuka lain.
2. **Akses dari HP lewat LAN privat hanya boleh diaktifkan lewat konfigurasi eksplisit** (mis. nilai alamat yang dipilih operator). Tidak ada default yang membuka jaringan, dan pengikatan LAN **tidak boleh** menjadi perilaku otomatis.
3. **Setiap komunikasi aplikasi → backend pada LAN wajib HTTPS dengan sertifikat yang dipercaya perangkat.** Sertifikat harus dipasang/dipercayai sebagai bagian dari penyiapan yang disengaja, bukan diterima begitu saja oleh aplikasi.
4. **Dilarang**: HTTP terbuka sebagai konfigurasi deployment yang direkomendasikan; menonaktifkan validasi TLS; `InsecureSkipVerify` atau ekuivalen pada klien mana pun. Tidak ada flag "abaikan sertifikat" pada aplikasi.
5. **Backend tidak boleh dibuka ke internet publik.** Jangan menganggap Wi-Fi privat saja sudah melindungi access/refresh token; token tetap menuntut transport terenkripsi dan jaringan yang dikendalikan.
6. **Untuk development, loopback atau `adb reverse` boleh dipakai** bila sesuai keamanan platform. `adb reverse tcp:8080 tcp:8080` mempertahankan backend pada loopback tanpa menambah permukaan jaringan.
7. **Rincian sertifikat lokal dan trust chain Android masih pekerjaan teknis yang BELUM DIUJI.** Catatan pembatas yang wajib dibaca bersama aturan ini: Android 7+ **tidak** mempercayai CA yang dipasang pengguna untuk aplikasi kecuali aplikasi memakai `network_security_config` atau build kustom (Expo Go tidak dapat memuat konfigurasi itu), sehingga opsi CA lokal/reverse proxy menuntut development build/production build — dan **itu yang harus diuji**. Detail temuan dan batasan ini ada di `docs/ARCHITECTURE.md` §5.1 dan kebijakan privasi/keamanannya di `docs/SECURITY_PRIVACY.md` §6.
8. **Kode yang berlaku saat ini menolak pengikatan non-loopback** (`FACELINK_ADDR` selain `127.0.0.1`/`localhost`/`::1` ditolak oleh `backend/internal/config/config.go`), sehingga pengikatan LAN harus diimplementasikan pada **FL-011** bersama TLS. Selama aturan itu berlaku, opsi yang tersedia untuk demo adalah loopback + `adb reverse`.
9. **Status verifikasi:** tidak ada satu pun ketentuan pada sub-bagian ini yang sudah diuji. Seluruhnya berstatus **BELUM TERVERIFIKASI**.

## 2. Strategi versi

1. Versi mayor ada pada jalur (`/api/v1`). Perubahan yang **memutus kompatibilitas** menaikkan nomor mayor dan menambah jalur baru; jalur lama tidak dihapus dalam tugas yang sama.
2. Perubahan **aditif** (field baru opsional, endpoint baru) tidak menaikkan versi. Klien wajib mengabaikan field yang tidak dikenal.
3. Field yang akan dihapus ditandai `deprecated` di dokumentasi minimal satu rilis sebelum dihapus.
4. Penghapusan field wajib, perubahan arti field, dan perubahan bentuk `data` dari objek menjadi daftar dianggap **memutus kompatibilitas**.
5. Versi kontrak internal Go↔Python (§6) bergerak **terpisah** dari versi API publik; keduanya tidak boleh diasumsikan sama.
6. **Pengecualian tercatat pada FL-010U (16 Oktober 2026):** revisi ini mengganti `POST /auth/login` menjadi `POST /auth/google` dan mengganti nama `GET/DELETE /operator-sessions*` menjadi `GET/DELETE /auth/sessions*`. Menurut aturan butir 1 dan 4 di atas, keduanya seharusnya menaikkan nomor mayor, tetapi **tidak ada kode maupun klien yang memakai jalur lama** (kontrak ini belum diimplementasikan), sehingga jalur lama tidak perlu dipertahankan dan versi mayor tidak dinaikkan. Bila jalur lama ternyata sudah dipakai klien mana pun, aturan butir 1 berlaku penuh.

## 3. Ikhtisar modul

Empat belas modul, dipetakan ke jalur nyata. Penomoran modul 1–13 **dipertahankan** dari versi kontrak sebelumnya; Modul 14 adalah modul **baru** pada FL-010U. Beberapa kebutuhan **sengaja tidak menjadi endpoint publik** karena merupakan operasi internal atau efek samping endpoint lain.

| # | Modul | Jalur | Catatan |
|---|---|---|---|
| 1 | Autentikasi Google dan sesi | `/auth/*`, `/me` | Google Sign-In (OIDC) → sesi opak FaceLink AI |
| 2 | Akun pengguna, peserta, dan penautan peserta | `/users/*`, `/people/*`, `/people/{person_id}/account-link` | `/users` dan penautan akun hanya untuk admin |
| 3 | Persetujuan biometrik | `/people/{person_id}/consent*` | Wajib sebelum enrollment; hanya `admin` |
| 4 | Enrollment wajah | `/people/{person_id}/face-enrollments*` | Gambar tidak disimpan permanen; hanya `admin` |
| 5 | Kelas, keanggotaan, dan penugasan dosen | `/classes/*` | Termasuk `GET/PUT/DELETE /classes/{class_id}/dosen/{user_id}` |
| 6 | Sesi absensi | `/sessions/*` | Siklus draft → open → closed |
| 7 | Hasil verifikasi wajah | Bagian dari `recognition-attempts` | **Bukan** endpoint terpisah |
| 8 | Pencatatan kehadiran | Efek dari pengenalan atau koreksi manual | **Bukan** endpoint publik terpisah |
| 9 | Penolakan dan jalur manual | `/sessions/{id}/manual-checkins` | Wajib ada sebagai alternatif |
| 10 | Pendaftaran dan autentikasi perangkat | `/devices/*` | Token terpisah; seluruhnya `admin` |
| 11 | Rekap dan ekspor laporan | `/sessions/{id}/attendance`, `/export.csv` | Tanpa data biometrik |
| 12 | Kesehatan dan kesiapan | `/healthz`, `/readyz` | Tidak membocorkan rahasia |
| 13 | Audit aktivitas penting | `/audit-events` | **Hanya `admin`** |
| 14 | Portal self-service dosen dan mahasiswa | `/me/classes`, `/me/schedule`, `/me/attendance`, `/me/consent` | Modul **baru** FL-010U; lihat §5.14 |

Justifikasi pemisahan modul 7 dan 8: hasil verifikasi wajah dan pencatatan kehadiran **bukan** dua operasi yang dapat dipanggil klien secara terpisah. Kalau keduanya menjadi endpoint publik, akan ada jalur yang memungkinkan klien menetapkan kehadiran tanpa validasi wajah, atau mencatat hasil pengenalan tanpa memeriksa konsen, kelas, dan sesi. Karena itu modul 7 dan 8 diwujudkan sebagai **hasil satu endpoint terminal** (§5.10) yang di dalam backend Go menjalankan seluruh pemeriksaan berurutan.

Catatan rujukan silang: matriks peran/izin pada `docs/AUTHORIZATION.md` §4 memakai pengelompokan modul yang berbeda (mis. "Modul 6 — Kelas, keanggotaan, penugasan dosen" di sana setara Modul 5 di sini, dan "Modul 7 — Sesi absensi" setara Modul 6 di sini). Yang wajib identik antara kedua dokumen adalah **metode, jalur, dan himpunan peran** tiap endpoint — bukan nomor modulnya.

## 4. Ikhtisar endpoint

Semua jalur diawali `/api/v1` **kecuali** `/healthz` (tanpa versi, hanya untuk probe lokal). Matriks di bawah adalah **54 endpoint** dan wajib identik dengan matriks peran pada `docs/AUTHORIZATION.md` §4: metode, jalur, dan himpunan peran harus sama persis. Nomor pada kolom Modul mengikuti penomoran modul §3 dokumen ini.

| Modul | Metode | Jalur | Peran |
|---|---|---|---|
| 1 | `POST` | `/auth/nonce` | publik |
| 1 | `POST` | `/auth/google` | publik |
| 1 | `POST` | `/auth/refresh` | admin, dosen, mahasiswa |
| 1 | `POST` | `/auth/logout` | admin, dosen, mahasiswa |
| 1 | `GET` | `/me` | admin, dosen, mahasiswa |
| 1 | `GET` | `/auth/sessions` | admin |
| 1 | `DELETE` | `/auth/sessions/{session_id}` | admin |
| 2 | `GET` | `/users` | admin |
| 2 | `POST` | `/users` | admin |
| 2 | `PATCH` | `/users/{user_id}` | admin |
| 2 | `PUT` | `/people/{person_id}/account-link` | admin |
| 2 | `DELETE` | `/people/{person_id}/account-link` | admin |
| 2 | `GET` | `/people` | admin, dosen |
| 2 | `POST` | `/people` | admin |
| 2 | `GET` | `/people/{person_id}` | admin, dosen |
| 2 | `PATCH` | `/people/{person_id}` | admin |
| 2 | `DELETE` | `/people/{person_id}` | admin |
| 3 | `GET` | `/people/{person_id}/consent` | admin |
| 3 | `POST` | `/people/{person_id}/consent` | admin |
| 3 | `POST` | `/people/{person_id}/consent/withdraw` | admin |
| 4 | `POST` | `/people/{person_id}/face-enrollments` | admin |
| 4 | `GET` | `/people/{person_id}/face-enrollments` | admin |
| 4 | `POST` | `/face-enrollments/{enrollment_id}/samples` | admin |
| 4 | `POST` | `/face-enrollments/{enrollment_id}/complete` | admin |
| 4 | `DELETE` | `/face-enrollments/{enrollment_id}` | admin |
| 5 | `GET` | `/classes` | admin, dosen |
| 5 | `POST` | `/classes` | admin |
| 5 | `GET` | `/classes/{class_id}/members` | admin, dosen |
| 5 | `PUT` | `/classes/{class_id}/members/{person_id}` | admin |
| 5 | `DELETE` | `/classes/{class_id}/members/{person_id}` | admin |
| 5 | `GET` | `/classes/{class_id}/dosen` | admin, dosen |
| 5 | `PUT` | `/classes/{class_id}/dosen/{user_id}` | admin |
| 5 | `DELETE` | `/classes/{class_id}/dosen/{user_id}` | admin |
| 6 | `GET` | `/sessions` | admin, dosen |
| 6 | `POST` | `/sessions` | admin, dosen |
| 6 | `GET` | `/sessions/{session_id}` | admin, dosen |
| 6 | `POST` | `/sessions/{session_id}/open` | admin, dosen |
| 6 | `POST` | `/sessions/{session_id}/close` | admin, dosen |
| 9 | `POST` | `/sessions/{session_id}/manual-checkins` | admin, dosen |
| 9 | `GET` | `/sessions/{session_id}/manual-checkins` | admin, dosen |
| 10 | `POST` | `/devices` | admin |
| 10 | `GET` | `/devices` | admin |
| 10 | `POST` | `/devices/{device_id}/rotate-token` | admin |
| 10 | `DELETE` | `/devices/{device_id}` | admin |
| 11 | `GET` | `/sessions/{session_id}/attendance` | admin, dosen |
| 11 | `GET` | `/sessions/{session_id}/export.csv` | admin |
| 12 | `GET` | `/healthz` | publik |
| 12 | `GET` | `/readyz` | admin, dosen |
| 13 | `GET` | `/audit-events` | admin |
| 14 | `GET` | `/me/classes` | dosen, mahasiswa |
| 14 | `GET` | `/me/schedule` | mahasiswa |
| 14 | `GET` | `/me/attendance` | mahasiswa |
| 14 | `GET` | `/me/consent` | mahasiswa |
| — | `POST` | `/devices/{device_id}/recognition-attempts` | device |

Rekapitulasi kelompok akses (dihitung dari tabel di atas; satu endpoint dapat diakses beberapa peran sehingga jumlah kolom tidak dijumlahkan menjadi 53):

| Kelompok akses | Jumlah | Cakupan |
|---|---|---|
| Hanya `admin` | 29 | Akun, role, penautan, konsen, enrollment, kelola peserta, penugasan dosen, perangkat, ekspor, audit |
| `admin` + `dosen` | 14 | Baca peserta/kelas, sesi absensi, koreksi manual, rekap, kesiapan |
| `admin` + `dosen` + `mahasiswa` | 3 | Sesi sendiri: refresh, logout, `/me` |
| `dosen` + `mahasiswa` | 1 | `GET /me/classes` |
| Hanya `mahasiswa` | 3 | Jadwal, riwayat kehadiran sendiri, status konsen sendiri |
| Hanya `device` | 1 | Terminal kamera |
| Publik tanpa token | 3 | `POST /auth/nonce`, `POST /auth/google`, `GET /healthz` |
| **Total endpoint** | **54** | 29 + 14 + 3 + 1 + 3 + 1 + 3 |

Jumlah endpoint yang dapat dipanggil tiap peran: `admin` 46 · `dosen` 18 · `mahasiswa` 7 · `device` 1 · publik 3. Angka-angka ini **BELUM TERVERIFIKASI secara programatis** pada FL-010U dan wajib dihitung ulang setelah dokumen ditulis; tabel rekap disesuaikan bila tabel endpoint berubah.

**Status implementasi per endpoint (FL-011, 16 Oktober 2026).** Matriks 54 endpoint di atas **tidak diubah** oleh FL-011 kecuali satu baris baru — `POST /auth/nonce`, penerbit nonce yang membuat aturan nonce dapat ditegakkan. Dari 54 endpoint, **18 sudah terimplementasi** dan **36 masih kontrak saja**. Daftar ringkas berikut adalah penanda, bukan matriks baru; status per endpoint juga diulang pada pembuka tiap sub-bagian §5.

| # | Metode | Jalur | Status (FL-011) |
|---|---|---|---|
| 1 | `POST` | `/auth/nonce` | **TERIMPLEMENTASI** |
| 2 | `POST` | `/auth/google` | **TERIMPLEMENTASI** |
| 3 | `POST` | `/auth/refresh` | **TERIMPLEMENTASI** |
| 4 | `POST` | `/auth/logout` | **TERIMPLEMENTASI** |
| 5 | `GET` | `/me` | **TERIMPLEMENTASI** |
| 6 | `GET` | `/auth/sessions` | **TERIMPLEMENTASI** — wajib parameter `user_id` (lihat §5.1) |
| 7 | `DELETE` | `/auth/sessions/{session_id}` | **TERIMPLEMENTASI** |
| 8 | `GET` | `/users` | **TERIMPLEMENTASI** |
| 9 | `POST` | `/users` | **TERIMPLEMENTASI** |
| 10 | `PATCH` | `/users/{user_id}` | **TERIMPLEMENTASI** |
| 11 | `PUT` | `/people/{person_id}/account-link` | **TERIMPLEMENTASI** |
| 12 | `DELETE` | `/people/{person_id}/account-link` | **TERIMPLEMENTASI** |
| 13 | `GET` | `/classes/{class_id}/dosen` | **TERIMPLEMENTASI** |
| 14 | `PUT` | `/classes/{class_id}/dosen/{user_id}` | **TERIMPLEMENTASI** |
| 15 | `DELETE` | `/classes/{class_id}/dosen/{user_id}` | **TERIMPLEMENTASI** |
| 16 | `GET` | `/healthz` | **TERIMPLEMENTASI** — kode yang ada memasangnya pada `/api/v1/healthz` (lihat §5.12) |
| 17 | `GET` | `/readyz` | **TERIMPLEMENTASI** |
| 18 | `GET` | `/me/classes` | **TERIMPLEMENTASI** |

Seluruh **36 endpoint lainnya** pada matriks di atas berstatus **BELUM DIIMPLEMENTASIKAN (kontrak saja)**: modul konsen biometrik (§5.3), enrollment (§5.4), sesi absensi (§5.6), koreksi manual (§5.8), perangkat dan terminal kamera (§5.9, §5.10), rekap/ekspor (§5.11), audit-events (§5.13), portal `/me/schedule` `/me/attendance` `/me/consent` (§5.14), serta seluruh endpoint `/people` dan `/classes` selain penugasan dosen. **Tidak ada satu pun endpoint pada daftar 36 itu yang boleh dianggap berfungsi**, dan tidak ada pernyataan di dokumen ini yang boleh dibaca seolah-olah ia sudah berjalan.

Kesenjangan yang wajib dibaca bersama status di atas dicatat pada §5.1 (nonce wajib dan diterbitkan backend, `user_id` pada daftar sesi), §1.4 (bentuk galat yang belum diseragamkan), dan §10 (pembatasan laju, endpoint yang belum ada, TLS LAN).

## 5. Detail endpoint

Notasi: **wajib** berarti field harus ada. Semua contoh JSON memakai UUID dan nilai buatan, tanpa data biometrik nyata.

### 5.1 Modul 1 — Autentikasi Google dan manajemen sesi

**Status implementasi (FL-011, 16 Oktober 2026): ketujuh endpoint pada modul ini TERIMPLEMENTASI** — `POST /auth/nonce`, `POST /auth/google`, `POST /auth/refresh`, `POST /auth/logout`, `GET /me`, `GET /auth/sessions`, dan `DELETE /auth/sessions/{session_id}`. Deviasi yang berlaku pada implementasi dan **tercatat langsung pada endpointnya**: **`nonce` kini wajib dan diterbitkan backend** (`POST /auth/nonce`, bukan "wajib bila aplikasi memakai nonce"), **`GET /auth/sessions` menuntut parameter `user_id`** dan hanya menyajikan sesi **aktif**, serta **respons `POST /auth/refresh` hanya memuat pasangan token** tanpa objek `user`. Pembatasan laju pada `/auth/google`, `/auth/nonce`, dan `/auth/refresh` **belum diimplementasikan** (§10).

Ringkasan alur (selengkapnya pada `docs/AUTHORIZATION.md` §3.1 dan `docs/DECISIONS.md` D-031): aplikasi React Native meminta Google ID token (JWT) di sisi perangkat, mengirimkannya ke `POST /auth/google`, backend memverifikasi token menurut OpenID Connect, mencari identitas pada `user_identities` dengan kunci `(provider='google', subject=sub)`, memeriksa status akun `users`, lalu menerbitkan **sesi FaceLink AI** berupa token opak. Google ID token **bukan** sesi aplikasi dan **tidak pernah** disimpan sebagai sesi.

Bentuk sesi (`docs/AUTHORIZATION.md` §3.1 dan `docs/DECISIONS.md` **D-033, DISETUJUI pada FL-010V**): token opak acak 256-bit dikirim sebagai `Authorization: Bearer <token>`; di server hanya **hash SHA-256** yang disimpan pada `session_tokens`; masa berlaku token akses 60 menit; token penyegar 30 hari dengan **batas absolut sesi 30 hari dihitung sejak sesi awal dibuat**; penyimpanan klien di penyimpanan aman perangkat. Peran **tidak** disimpan di dalam token: role selalu dibaca ulang dari database pada setiap permintaan.

#### `POST /api/v1/auth/google`

- **Status (FL-011, 16 Oktober 2026): TERIMPLEMENTASI.** Endpoint ini menjawab `503 GOOGLE_UNAVAILABLE` selama `FACELINK_GOOGLE_CLIENT_IDS` kosong; endpoint lokal lain (termasuk `/healthz`, `/readyz`, dan seluruh jalur sesi yang sudah ada) tetap berjalan. Endpoint **belum** memiliki pembatasan laju (§10).
- **Peran:** publik (dengan pembatasan laju ketat).
- **Tubuh permintaan:** `id_token` (wajib, string — Google ID token/JWT), `nonce` (**wajib** sejak FL-011, 16 Oktober 2026; lihat aturan nonce di bawah).
- **Verifikasi OIDC yang wajib:** tanda tangan lewat **JWKS** Google (`kid`; kunci di-cache dan dirotasi saat `kid` tidak dikenal); `alg` **wajib** RS256 (menolak `none`, HS256, dan algoritme simetris); `iss` ∈ {`https://accounts.google.com`, `accounts.google.com`}; `aud` ∈ daftar client ID resmi yang dikonfigurasi (tidak ada client ID yang ditanam di kode); `exp`/`iat` dengan toleransi selisih waktu terbatas (usulan 120 detik); `nonce` **wajib cocok**; `azp` diperiksa bila token punya beberapa audience; `email_verified` = true. Alamat JWKS diambil dari konfigurasi `FACELINK_GOOGLE_JWKS_URL` (bawaan endpoint Google, wajib HTTPS), bukan dari klaim token.
- **Aturan nonce (FL-010V, 16 Oktober 2026; ditegaskan wajib pada FL-011):** aplikasi meminta **nonce acak sekali pakai** ke backend lewat `POST /auth/nonce` (lihat di bawah), mengirimkannya **ke Google** sebagai parameter `nonce` pada permintaan otorisasi, lalu mengirimkannya kembali bersama ID token ke endpoint ini. Backend memverifikasi kecocokan nonce, lalu **menolak nonce yang sudah pernah dipakai atau sudah kedaluwarsa** — nonce bersifat **sekali pakai** dengan **TTL 5 menit**, disimpan sebagai **hash** (bukan nilai mentah) pada tabel `auth_nonces`. Login tanpa nonce yang sah atau dengan nonce kedaluwarsa/terpakai gagal dengan `401 IDENTITY_TOKEN_INVALID`. Rujukan kebijakan: `docs/DECISIONS.md` D-031 dan `docs/SECURITY_PRIVACY.md` §2.
- **Konsekuensi penerbitan nonce di backend (FL-011, 16 Oktober 2026).** Nonce **hanya sah bila terdaftar lebih dahulu** oleh backend. Alasannya: pengikatan nonce yang dituntut kontrak ini tidak dapat dipenuhi bila klien mendaftarkan sendiri nonce-nya **setelah** login — token yang bocor dapat diputar ulang dengan nonce karangan. Karena itu backend menyediakan penerbitnya (`POST /auth/nonce`) dan menolak nonce yang tidak pernah diterbitkannya. Nonce yang sudah dipakai tidak pernah didaftarkan ulang: barisnya dipertahankan 10 menit (dua kali TTL) agar pemakaian ulang tetap dikenali sebagai `nonce_replay`, bukan sebagai nonce asing.
- **`nonce` kini WAJIB — penguatan FL-011 (16 Oktober 2026).** Kontrak sebelumnya berbunyi "`nonce` wajib bila aplikasi memakai nonce", sehingga implementasi yang tidak mengirim nonce tetap sah. **FL-011 mempersempitnya: `nonce` wajib dikirim pada setiap permintaan**, dan backend menolak permintaan tanpa nonce sebagai `400 MISSING_FIELD` sebelum ID token diverifikasi. Alasannya: nonce adalah pengikat tunggal antara ID token dan percobaan login aplikasi ini, sehingga tanpa nonce token yang bocor/tertangkap dapat dipakai ulang terhadap backend. **Konsekuensi bagi aplikasi klien React Native:** aplikasi **wajib** meminta nonce ke `POST /auth/nonce`, mengirimkannya **sebagai parameter `nonce` pada permintaan otorisasi Google** dan **dengan nilai yang sama** pada field `nonce` tubuh `POST /auth/google`; aplikasi yang tidak mengirim nonce ke Google akan selalu gagal di sini, dan aplikasi yang mengirim nonce berbeda di kedua jalur gagal dengan `401 IDENTITY_TOKEN_INVALID`. Ini adalah **penguatan untuk D-033**, bukan penolakan keputusan; tidak ada klaim bahwa nonce sebelumnya opsional pada implementasi yang berlaku.
- **Tujuan ID token (FL-010V, 16 Oktober 2026):** Google ID token dipakai **hanya untuk pertukaran identitas** pada endpoint ini — ia **bukan** token otorisasi API FaceLink AI dan **tidak pernah** menjadi sesi aplikasi. Setelah pertukaran berhasil, seluruh permintaan berikutnya memakai token sesi opak FaceLink AI, dan role/cakupan tetap diputuskan backend dari database.
- **Aturan penautan akun:** identitas dicari pada `user_identities` lewat `sub` (pengenal stabil; **email bukan kunci identitas**). Identitas belum tertaut boleh tertaut **hanya bila** ada baris `users` berstatus `invited` dengan `email` yang sama persis (huruf kecil, tanpa spasi tepi) dan klaim `email_verified=true`, lalu baris itu diaktifkan. Selain itu **tidak ada akun yang dibuat**: tidak ada pendaftaran mandiri dan tidak ada role otomatis.

#### `POST /api/v1/auth/nonce`

- **Status (FL-011, 16 Oktober 2026): TERIMPLEMENTASI.** Endpoint baru; kontrak sebelumnya menyebut "aplikasi menghasilkan nonce" tanpa menyediakan jalurnya, sehingga pengikatan nonce tidak dapat ditegakkan.
- **Peran:** publik. Tidak menerima rahasia apa pun; pembatasan laju menyusul bersama `/auth/google` (§10).
- **Tubuh permintaan:** kosong.
- **Respons `200`:** `{ "data": { "nonce": "<43 karakter base64url>", "expires_in": 300 }, "request_id": "..." }`. Nilai `nonce` adalah 256 bit acak dari sumber kriptografis.
- **Kode status:** `200`, `503`.
- **Kesalahan:** `AUTH_UNAVAILABLE` (503) bila komponen autentikasi belum dikonfigurasi.
- **Efek database:** `INSERT INTO auth_nonces` (hanya hash SHA-256 yang disimpan).
- **Idempotensi:** tidak; setiap panggilan menerbitkan nonce baru.
- **Catatan:** satu nonce untuk satu percobaan login. Aplikasi yang mengulang percobaan harus meminta nonce baru.
- **Aturan penautan akun:** identitas dicari pada `user_identities` lewat `sub` (pengenal stabil; **email bukan kunci identitas**). Identitas belum tertaut boleh tertaut **hanya bila** ada baris `users` berstatus `invited` dengan `email` yang sama persis (huruf kecil, tanpa spasi tepi) dan klaim `email_verified=true`, lalu baris itu diaktifkan. Selain itu **tidak ada akun yang dibuat**: tidak ada pendaftaran mandiri dan tidak ada role otomatis.
- **Respons `200`:**

```json
{
  "data": {
    "access_token": "<token opak>",
    "refresh_token": "<token opak>",
    "expires_in": 3600,
    "user": {
      "id": "33333333-3333-4333-8333-333333333333",
      "display_name": "Dosen Demo",
      "role": "dosen",
      "person_id": null,
      "class_ids": ["11111111-1111-4111-8111-111111111111"]
    }
  },
  "request_id": "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
}
```

- **Catatan bentuk respons:** `person_id` bernilai `null` untuk akun yang tidak tertaut baris `people` (umumnya `admin`/`dosen`); `class_ids` berisi kelas yang ditugaskan (`dosen`) atau diikuti (`mahasiswa`), dan kosong untuk `admin`. Aplikasi mengarahkan pengguna berdasarkan `role` dari backend.
- **Kode status:** `200`, `400`, `401`, `403`, `429`, `503`. **Catatan implementasi (FL-011):** `400` (JSON rusak / `id_token` atau `nonce` tidak diisi), `401`, `403`, dan `503 GOOGLE_UNAVAILABLE` sudah dihasilkan kode; **`429` belum pernah dihasilkan** karena pembatasan laju belum diimplementasikan (§10).
- **Kesalahan:** `VALIDATION_FAILED`, `MISSING_FIELD` (400 — termasuk `nonce` tidak diisi), `IDENTITY_TOKEN_INVALID` (401), `IDENTITY_NOT_LINKED` (403), `RATE_LIMITED` (429 — **belum dihasilkan kode**), `GOOGLE_UNAVAILABLE` (503).
- **Efek database:** memperbarui `users.last_login_at`; `INSERT INTO user_identities` saat penautan pertama; menambah `audit_events` (`auth.google.succeeded`, `auth.google.linked`, atau `auth.login.denied`). Untuk `auth.login.denied`, `safe_metadata` menyimpan `reason` ber-kode (`no_account`, `not_linked`, `suspended`, `email_mismatch`) — **bukan** email.
- **Idempotensi:** tidak berlaku; setiap percobaan menambah baris audit. Pengulangan dengan ID token yang sama menerbitkan sesi baru.
- **Catatan keamanan:** `IDENTITY_NOT_LINKED` memakai **satu pesan generik** untuk semua sebab (anti-enumeration) sehingga pemanggil tidak dapat membedakan "email tidak terdaftar", "belum diberi akses", dan "akun dinonaktifkan". Status `invited` (belum tertaut) dan `suspended` sama-sama ditolak. Google ID token, klaim `sub`, dan `email` **tidak pernah** ditulis ke log.

#### `POST /api/v1/auth/refresh`

- **Status (FL-011, 16 Oktober 2026): TERIMPLEMENTASI.** Pembatasan laju **belum** ada (§10).
- **Peran:** admin, dosen, mahasiswa.
- **Tubuh permintaan:** `refresh_token` (wajib).
- **Respons `200`:** pasangan token baru — `{ "data": { "access_token", "refresh_token", "expires_in" } }`. Bentuk `user` tidak dikirim ulang; aplikasi memakai `GET /me` bila memerlukannya.
- **Kode status:** `200`, `400`, `401`.
- **Kesalahan:** `VALIDATION_FAILED`, `UNAUTHENTICATED`, `TOKEN_EXPIRED`, `SESSION_REVOKED`.
- **Rotasi:** setiap penyegaran menerbitkan pasangan token baru; hash penyegar sebelumnya disimpan satu langkah (`prev_refresh_hash`) dan **setiap hash yang digantikan dicatat pada `session_token_history`** (migrasi 00003) agar pemakaian ulang masih dapat dikenali setelah beberapa generasi rotasi.
- **Deteksi penyalahgunaan:** penyegar yang sudah dirotasi dipakai ulang → **seluruh sesi pengguna itu dicabut** (`revoked_reason='refresh_reuse'`) dan audit `auth.session.reuse_detected` ditulis. Cakupan "seluruh sesi" ditegakkan kode: token penyegar yang dipakai ulang menandakan salinan bocor, sehingga sesi lain milik pengguna yang sama ikut dicabut, bukan hanya sesi yang tokennya diputar.
- **Batas absolut 30 hari sejak sesi awal dibuat (FL-010V, 16 Oktober 2026):** batas absolut dihitung dari **waktu sesi awal dibuat**, bukan dari penyegaran terakhir. Rotasi penyegar **tidak boleh memperpanjang** batas absolut itu; sesi yang telah melewati 30 hari ditolak dan menuntut **Google Sign-In ulang** yang sah sebagai satu-satunya cara memperpanjang. Batas ini berlaku sebagai kebijakan yang disetujui (**D-033**) dan **sudah diimplementasikan pada FL-011** (kolom `session_tokens.absolute_expires_at` dengan `CHECK` yang menjaga `refresh_expires_at`/`access_expires_at` tidak melampauinya); hasil pengujiannya dicatat oleh agen induk pada `PROJECT_STATE.md`, bukan di dokumen ini.
- **Efek database:** `UPDATE session_tokens`; audit `auth.session.refreshed`.
- **Idempotensi:** token penyegar lama tidak dapat dipakai dua kali.

#### `POST /api/v1/auth/logout`

- **Status (FL-011, 16 Oktober 2026): TERIMPLEMENTASI.**
- **Peran:** admin, dosen, mahasiswa.
- **Tubuh permintaan:** kosong.
- **Respons:** `204` tanpa isi.
- **Kode status:** `204`, `401`.
- **Efek database:** mencabut sesi yang sedang dipakai (`session_tokens.revoked_at`); audit `auth.logout`.
- **Idempotensi:** panggilan kedua tetap `204` tanpa perubahan tambahan.

#### `GET /api/v1/me`

- **Status (FL-011, 16 Oktober 2026): TERIMPLEMENTASI.**
- **Peran:** admin, dosen, mahasiswa.
- **Query:** —
- **Respons `200`:**

```json
{
  "data": {
    "id": "33333333-3333-4333-8333-333333333333",
    "display_name": "Dosen Demo",
    "email": "dosen.demo@example.test",
    "role": "dosen",
    "status": "active",
    "person_id": null,
    "class_ids": ["11111111-1111-4111-8111-111111111111"]
  },
  "request_id": "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
}
```

- **Catatan:** `role` dan `status` selalu dibaca dari database pada setiap permintaan, sehingga perubahan role dan penonaktifan akun berlaku pada permintaan berikutnya. `person_id` `null` berarti akun tidak tertaut baris `people`; `class_ids` kosong untuk `admin`. Tidak ada field kata sandi, hash kata sandi, atau token pada respons ini.
- **Kode status:** `200`, `401`, `403`.
- **Efek database:** —
- **Idempotensi:** aman diulang.

#### `GET /api/v1/auth/sessions`

- **Status (FL-011, 16 Oktober 2026): TERIMPLEMENTASI.**
- **Peran:** admin.
- **Query:** `user_id` (**wajib sejak FL-011, 16 Oktober 2026**). Tanpa `user_id`, backend menjawab `400 MISSING_FIELD`; `user_id` yang tidak menunjuk akun dikenal dijawab `404 NOT_FOUND`. **Kesenjangan yang dicatat apa adanya:** implementasi yang berlaku **hanya** dapat mendaftar sesi **satu akun pada satu waktu**, sehingga **belum ada** daftar sesi lintas akun. Kontrak sebelumnya tidak menyebut parameter ini karena mengasumsikan daftar lintas akun; kemampuan itu belum ada dan tidak boleh diasumsikan. Parameter `user_id` tidak memperluas izin: endpoint ini tetap **hanya `admin`**, dan admin yang membaca sesi akun lain tidak memperoleh hak lain atas akun itu.
- **Catatan penamaan:** endpoint ini **menggantikan** `GET /operator-sessions` (nama lama salah karena kini menampung tiga role manusia). Nama lama tidak dipakai lagi; belum ada klien yang memakainya, sehingga tidak ada klien yang rusak.
- **Respons `200`:** daftar sesi aktif **milik akun pada `user_id`** dengan `id`, `user_id`, `role`, `created_at`, `last_seen_at`, `absolute_expires_at`, `user_agent_ringkas`. Sesi yang sudah dicabut **tidak** disertakan: daftar ini menyajikan sesi yang masih dapat dipakai, bukan riwayat.
- **Kode status:** `200`, `400`, `403`, `404`.
- **Kesalahan:** `MISSING_FIELD` (400, `user_id` tidak diisi), `FORBIDDEN_ROLE` (403), `NOT_FOUND` (404).
- **Efek database:** —
- **Idempotensi:** aman diulang.

#### `DELETE /api/v1/auth/sessions/{session_id}`

- **Status (FL-011, 16 Oktober 2026): TERIMPLEMENTASI.**
- **Peran:** admin.
- **Catatan penamaan:** **menggantikan** `DELETE /operator-sessions/{session_id}`.
- **Respons:** `204`.
- **Kode status:** `204`, `403`, `404`.
- **Efek database:** token sesi dicabut (`revoked_at`, `revoked_reason`); audit `auth.session.revoked`. Penonaktifan akun mencabut seluruh sesi akun itu.
- **Idempotensi:** mencabut sesi yang sudah tercabut mengembalikan `404`. Sesi yang sudah melewati batas waktunya juga `404`: tidak ada yang dapat dicabut, dan keadaan itu dibedakan dari pencabutan yang benar-benar terjadi.

### 5.2 Modul 2 — Akun pengguna, peserta, dan penautan peserta

**Status implementasi (FL-011, 16 Oktober 2026).** **TERIMPLEMENTASI:** `GET /users`, `POST /users`, `PATCH /users/{user_id}`, `PUT /people/{person_id}/account-link`, dan `DELETE /people/{person_id}/account-link`. **BELUM DIIMPLEMENTASIKAN (kontrak saja):** seluruh endpoint `/people` pada modul ini (`GET`, `POST`, `GET /{person_id}`, `PATCH`, `DELETE`) — pencatatan peserta dan pengelolaannya belum ada. Akibatnya penautan akun dapat dipanggil, tetapi baris peserta yang menjadi sasarannya harus sudah ada di database dari jalur lain; tidak ada endpoint produk yang membuatnya.

#### Lifecycle akun (FL-010V, 16 Oktober 2026)

Aturan berikut menetapkan **lifecycle akun** dan berlaku sebagai kebijakan yang disetujui (**D-032**, disetujui FL-010V). Tidak ada endpoint baru: seluruhnya mengikat perilaku endpoint pada §5.1 dan §5.2 yang sudah ada.

1. **Alur ringkas:** admin membuat undangan memakai alamat email Google calon pengguna → admin menetapkan **tepat satu** role utama (`admin`/`dosen`/`mahasiswa`) → akun berstatus **`invited`** → pengguna melakukan Google Sign-In → backend memverifikasi ID token menurut OpenID Connect (termasuk `email_verified` dan `sub`) → **penautan atomik** pada login pertama → akun berstatus **`active`**. Status akun: `invited` → `active` → `suspended`; **tidak ada penghapusan keras akun**.
2. **Penautan atomik pada login pertama.** Untuk akun undangan yang belum tertaut, email terverifikasi dicocokkan secara aman lalu `provider='google'` + `subject=sub` ditautkan **secara atomik**; hanya setelah itu akun menjadi `active`. Login berikutnya memakai identitas tertaut (`sub`), **bukan** email sebagai pengenal utama.
3. **Tanpa auto-registration.** Tidak ada akun yang dibuat otomatis saat login Google. Pengguna tak terdaftar atau nonaktif **tidak** mendapat akses dashboard.
4. **Tanpa role dari Google.** Role tidak pernah berasal dari Google maupun dari klien React Native; penetapan role hanya oleh `admin` berwenang melalui `PATCH /users/{user_id}` dan wajib diaudit.
5. **Tanpa auto-admin.** Google pertama yang login **tidak pernah** otomatis menjadi `admin`; admin pertama hanya dibuat lewat bootstrap lokal (butir 8).
6. **Tanpa account takeover lewat perubahan email.** Email hanya dipakai **sekali** untuk penautan pertama. Sesudah tertaut, pencocokan email **tidak pernah** dipakai lagi, sehingga perubahan alamat email di penyedia identitas tidak dapat mengalihkan akun. Ditegakkan bersama batasan unik `(provider, subject)` dan `(user_id, provider)`: satu identitas Google tidak boleh tertaut ke beberapa akun.
7. **Satu role utama per akun (D-037).** Satu akun internal memiliki **tepat satu role utama aktif**; tidak ada multi-role switching pada MVP. Hak istimewa `admin` untuk mengelola kelas sesuai matriks **tidak** menjadikannya role `dosen` kedua.
8. **Bootstrap admin lokal.** Proses **lokal, eksplisit, sekali, dan dapat diaudit**: tanpa hardcoded password/token/kredensial, menolak berjalan bila sudah ada akun `admin` berstatus `active` atau `invited`, dan menulis **satu** baris `users` (`role='admin'`, `status='invited'`) yang ditautkan pada login Google pertama. Perintahnya `-bootstrap-admin` pada `cmd/server` (diimplementasikan pada FL-011) dengan audit `user.bootstrapped`; lihat `docs/AUTHORIZATION.md` §3.1.
9. **Tahan request bersamaan.** Pendaftaran dan penautan harus tahan permintaan bersamaan: transaksi atomik ditambah batasan unik sebagai penjaga, dan **hanya satu** permintaan yang boleh menautkan. Bila batasan unik menolak, transaksi di-rollback dan permintaan kalah tidak boleh menulis penautan kedua.
10. **Status verifikasi (diperbarui FL-011, 16 Oktober 2026):** butir 1–9 kini **diimplementasikan** sebagai kode pada cabang `feat/fl-011-google-auth-backend` — bootstrap admin lokal (butir 8), penautan atomik berbasis batasan unik `(provider, subject)` dan `(user_id, provider)` (butir 2, 9), penolakan auto-registration/auto-admin (butir 3, 5), role hanya dari database (butir 4, 7), dan pencocokan email hanya pada penautan pertama (butir 6). Yang **belum** ada adalah **hasil pengujian yang dinyatakan di dokumen ini**: angka dan status kelulusan dicatat oleh agen induk pada `PROJECT_STATE.md`, bukan di sini. Skenario GA-06, GA-13, GA-14, dan GA-15 pada `docs/TEST_PLAN.md` §8.1/§8.3 tetap berstatus **BELUM TERVERIFIKASI** sampai hasil nyata dicatat. Implementasi ini **tidak** mengubah keputusan D-032 dan tidak menambah endpoint.

#### `GET /api/v1/people`

- **Status (FL-011, 16 Oktober 2026): BELUM DIIMPLEMENTASIKAN (kontrak saja).**
- **Peran:** admin, dosen. Untuk `dosen`, hasil dibatasi pada peserta kelas yang ditugaskan kepadanya.
- **Query:** `status` (opsional: `active`, `inactive`), `class_id` (opsional), `cursor`, `limit`.
- **Respons `200`:** daftar `{ id, external_code, display_name, status, consent_state, enrollment_state }`.
  - `consent_state`: `active`, `none`, atau `withdrawn`.
  - `enrollment_state`: `none` atau `enrolled`.
  - Nilai turunan ini **tidak** membocorkan jumlah atau isi template.
- **Kode status:** `200`, `401`.
- **Efek database:** —
- **Idempotensi:** aman diulang.

#### `POST /api/v1/people`

- **Status (FL-011, 16 Oktober 2026): BELUM DIIMPLEMENTASIKAN (kontrak saja).**
- **Peran:** admin.
- **Tubuh permintaan:** `external_code` (wajib, unik), `display_name` (wajib), `note` (opsional, ≤200 karakter).
- **Respons `201`:** objek `people` yang dibuat.
- **Kode status:** `201`, `400`, `409`, `422`.
- **Kesalahan:** `VALIDATION_FAILED`, `DUPLICATE_EXTERNAL_CODE`.
- **Validasi:** `external_code` 1–32 karakter, hanya huruf/angka/tanda hubung; **dilarang** memakai NIK asli untuk data demo (Q-13).
- **Efek database:** `INSERT INTO people`; audit `person.created`.
- **Idempotensi:** **tidak** idempoten secara bawaan; pengulangan menghasilkan `409` karena `external_code` unik.

#### `GET /api/v1/people/{person_id}`

- **Status (FL-011, 16 Oktober 2026): BELUM DIIMPLEMENTASIKAN (kontrak saja).**
- **Peran:** admin, dosen. Untuk `dosen`, hanya peserta yang menjadi anggota kelas yang ditugaskan kepadanya; peserta di luar cakupan → `404 NOT_FOUND`.
- **Respons `200`:** detail peserta termasuk `consent_state` dan `enrollment_state`.
- **Kode status:** `200`, `404`.
- **Efek database:** —
- **Idempotensi:** aman diulang.

#### `PATCH /api/v1/people/{person_id}`

- **Status (FL-011, 16 Oktober 2026): BELUM DIIMPLEMENTASIKAN (kontrak saja).**
- **Peran:** admin.
- **Tubuh permintaan:** `display_name`, `status`, `note` (semua opsional; minimal satu).
- **Respons `200`:** objek setelah perubahan.
- **Kode status:** `200`, `400`, `404`, `422`.
- **Efek database:** `UPDATE people`; audit `person.updated` dengan daftar nama field yang berubah (bukan nilai lama/baru penuh).
- **Idempotensi:** aman diulang untuk nilai yang sama.

#### `DELETE /api/v1/people/{person_id}`

- **Status (FL-011, 16 Oktober 2026): BELUM DIIMPLEMENTASIKAN (kontrak saja).**
- **Peran:** admin.
- **Respons:** `204`.
- **Kode status:** `204`, `403`, `404`, `409`.
- **Validasi/aturan:** penghapusan **wajib** menghapus template biometrik terkait pada transaksi yang sama (D-015). Bila peserta memiliki catatan kehadiran dan kebijakan retensi belum ditetapkan, endpoint mengembalikan `409 POLICY_CONFLICT` alih-alih menghapus data kehadiran diam-diam.
- **Efek database:** `DELETE FROM face_templates`; menonaktifkan atau menghapus `people`; audit `person.deleted`.
- **Idempotensi:** penghapusan kedua mengembalikan `404`.

#### `GET /api/v1/users`

- **Status (FL-011, 16 Oktober 2026): TERIMPLEMENTASI.**
- **Peran:** admin.
- **Query:** `role` (opsional: `admin`/`dosen`/`mahasiswa`), `status` (opsional: `invited`/`active`/`suspended`), `cursor`, `limit`.
- **Respons `200`:** daftar `{ id, email, display_name, role, status, created_by, created_at, last_login_at }`. Tidak ada field kata sandi atau hash kata sandi.
- **Kode status:** `200`, `403`.
- **Efek database:** —
- **Idempotensi:** aman diulang.

#### `POST /api/v1/users`

- **Status (FL-011, 16 Oktober 2026): TERIMPLEMENTASI.** Akun yang dibuat berstatus `invited` dan baru menjadi `active` pada login Google pertama (lihat §5.2 pembuka dan `README.md`).
- **Peran:** admin.
- **Tubuh permintaan:** `email` (wajib, email Google; dinormalisasi huruf kecil tanpa spasi tepi), `display_name` (wajib), `role` (wajib: `admin`/`dosen`/`mahasiswa`). **Tidak ada kata sandi** — akun dibuat berstatus `invited` dan diaktifkan pada Google Sign-In pertama.
- **Respons `201`:** objek `users` yang dibuat, berstatus `invited`.
- **Kode status:** `201`, `400`, `403`, `409`.
- **Kesalahan:** `VALIDATION_FAILED`, `FORBIDDEN_ROLE`; `409` bila email sudah dipakai.
- **Efek database:** `INSERT INTO users` dengan `status='invited'` dan `created_by` = id admin pembuat; audit `user.invited`.
- **Idempotensi:** tidak idempoten; pengulangan dengan email sama menghasilkan `409`.
- **Catatan:** tidak ada pendaftaran mandiri, tidak ada role yang berasal dari klien, dan tidak ada endpoint manipulasi peran massal. Google pertama yang login **tidak pernah** otomatis menjadi admin; admin pertama hanya dibuat lewat bootstrap lokal sekali (perintah `-bootstrap-admin` pada `cmd/server`, diimplementasikan pada FL-011, audit `user.bootstrapped`).

#### `PATCH /api/v1/users/{user_id}`

- **Status (FL-011, 16 Oktober 2026): TERIMPLEMENTASI.** Aturan `LAST_ADMIN_PROTECTED` dan `SELF_ACCOUNT_PROTECTED` ditegakkan kode.
- **Peran:** admin.
- **Tubuh permintaan:** `role`, `status`, `display_name` (semua opsional; minimal satu).
- **Respons `200`:** objek akun setelah perubahan.
- **Kode status:** `200`, `400`, `403`, `404`, `409`.
- **Kesalahan:** `VALIDATION_FAILED`, `NOT_FOUND`, `LAST_ADMIN_PROTECTED` (409), `SELF_ACCOUNT_PROTECTED` (409).
- **Aturan pencegahan eskalasi dan penguncian:** tidak ada endpoint yang mengubah role akun sendiri; admin aktif terakhir tidak boleh dinonaktifkan atau diturunkan perannya; admin tidak boleh menonaktifkan atau menurunkan peran akunnya sendiri; penonaktifan akun mencabut seluruh sesi akun itu.
- **Efek database:** `UPDATE users`; audit `user.role_changed` atau `user.status_changed` beserta nilai sebelum/sesudah.
- **Idempotensi:** aman diulang untuk nilai yang sama.

#### `PUT /api/v1/people/{person_id}/account-link`

- **Status (FL-011, 16 Oktober 2026): TERIMPLEMENTASI.**
- **Peran:** admin.
- **Tubuh permintaan:** `user_id` (wajib; akun ber-role `mahasiswa`).
- **Respons `200`:** objek peserta dengan `account_user_id` terisi.
- **Kode status:** `200`, `400`, `403`, `404`, `409`.
- **Kesalahan:** `VALIDATION_FAILED`, `NOT_FOUND`, `409` bila akun sudah tertaut peserta lain (relasi satu-ke-satu) atau role akun bukan `mahasiswa`.
- **Aturan:** penautan bersifat satu-ke-satu dan **tidak memberi akses apa pun ke template biometrik**; `account_user_id` tidak membuka `face_templates`.
- **Efek database:** `UPDATE people SET account_user_id`; audit `person.account_linked`.
- **Idempotensi:** idempoten untuk pasangan yang sama.

#### `DELETE /api/v1/people/{person_id}/account-link`

- **Status (FL-011, 16 Oktober 2026): TERIMPLEMENTASI.**
- **Peran:** admin.
- **Respons:** `204`.
- **Kode status:** `204`, `403`, `404`, `409`.
- **Kesalahan:** `NOT_FOUND`; `409` bila peserta memang belum tertaut.
- **Efek database:** `UPDATE people SET account_user_id = NULL`; audit `person.account_unlinked`.
- **Idempotensi:** penghapusan kedua mengembalikan `404`/`409`.

### 5.3 Modul 3 — Persetujuan biometrik

**Status implementasi (FL-011, 16 Oktober 2026): SELURUH modul ini BELUM DIIMPLEMENTASIKAN (kontrak saja)** — `GET`, `POST`, dan `POST .../withdraw` belum ada di kode. Pencatatan konsen biometrik belum berjalan; tabel `consent_records` dan `face_templates` masih tidak ditulis oleh endpoint produk mana pun.

Seluruh endpoint pada modul ini **hanya `admin`**; `dosen` tidak mencatat atau mencabut konsen (lihat `docs/AUTHORIZATION.md` §9 butir 1).

#### `GET /api/v1/people/{person_id}/consent`

- **Peran:** admin.
- **Respons `200`:** riwayat persetujuan: `{ id, purpose, version, capture_method, granted_at, withdrawn_at }`.
- **Kode status:** `200`, `404`.
- **Efek database:** —
- **Idempotensi:** aman diulang.

#### `POST /api/v1/people/{person_id}/consent`

- **Peran:** admin.
- **Tubuh permintaan:** `purpose` (wajib, mis. `attendance_face_recognition`), `version` (wajib, versi teks persetujuan), `capture_method` (wajib: `written_form`/`digital_form`), `granted_at` (opsional; bawaan waktu server).
- **Respons `201`:** objek persetujuan.
- **Kode status:** `201`, `400`, `404`, `409`.
- **Validasi:** `version` tidak boleh kosong; persetujuan tidak boleh dicatat untuk peserta yang tidak ada; persetujuan ganda aktif dengan `version` sama ditolak `409`.
- **Efek database:** `INSERT INTO consent_records`; audit `consent.granted`.
- **Idempotensi:** menggunakan `Idempotency-Key` opsional; tanpa kunci, pengulangan identik menghasilkan `409`.

#### `POST /api/v1/people/{person_id}/consent/withdraw`

- **Peran:** admin.
- **Tubuh permintaan:** `reason` (wajib, tidak boleh kosong), `withdrawn_at` (opsional).
- **Respons `200`:** objek persetujuan setelah pencabutan.
- **Kode status:** `200`, `400`, `404`, `409`.
- **Efek database (satu transaksi):** mengisi `consent_records.withdrawn_at`; **menghapus** baris `face_templates` milik peserta; audit `consent.withdrawn` yang mencatat jumlah template yang dihapus.
- **Idempotensi:** pencabutan ulang ketika sudah dicabut mengembalikan `409 CONSENT_WITHDRAWN`.
- **Catatan:** setelah pencabutan, upaya pengenalan tidak boleh menampilkan peserta sebagai kandidat, karena template sudah tidak ada. Ini **wajib diuji** sebagai skenario kontrak (§8).

### 5.4 Modul 4 — Enrollment wajah

**Status implementasi (FL-011, 16 Oktober 2026): SELURUH modul ini BELUM DIIMPLEMENTASIKAN (kontrak saja).** Tidak ada endpoint enrollment pada kode, dan tidak ada penulisan `face_templates`/`face_enrollment_samples` (D-030 masih berlaku apa adanya).

Enrollment adalah operasi **berjangka waktu**. Tidak ada endpoint unggah bebas yang memungkinkan seseorang mendaftarkan wajah ke identitas orang lain. Seluruh endpoint pada modul ini **hanya `admin`**; `dosen` tidak melakukan enrollment biometrik (lihat `docs/AUTHORIZATION.md` §9 butir 1).

#### `POST /api/v1/people/{person_id}/face-enrollments`

- **Peran:** admin.
- **Tubuh permintaan:** `model_hint` (opsional; diabaikan bila tidak dikenal).
- **Respons `201`:** `{ "data": { "id": "44444444-4444-4444-8444-444444444444", "expires_at": "2026-10-15T08:10:00Z", "samples_required_min": 3, "samples_required_max": 10 } }`.
- **Kode status:** `201`, `404`, `422`.
- **Kesalahan:** `CONSENT_REQUIRED`, `CONSENT_WITHDRAWN`.
- **Validasi:** peserta wajib ada dan memiliki persetujuan aktif. Masa berlaku bawaan 10 menit.
- **Efek database:** `INSERT INTO face_enrollments` berstatus `pending`; audit `enrollment.started`.
- **Idempotensi:** `Idempotency-Key` dihormati; tanpa kunci, percobaan ganda membuat sesi enrollment baru dan menutup yang sebelumnya.

#### `GET /api/v1/people/{person_id}/face-enrollments`

- **Peran:** admin.
- **Respons `200`:** daftar sesi enrollment peserta dengan status dan alasan penolakan sampel.
- **Kode status:** `200`, `404`.
- **Efek database:** —
- **Idempotensi:** aman diulang.

#### `POST /api/v1/face-enrollments/{enrollment_id}/samples`

- **Peran:** admin.
- **Konten:** `image/jpeg` biner.
- **Header:** `Content-Length` wajib; `If-Match` opsional untuk mencegah sampel ganda akibat retry.
- **Respons `201`:** `{ "data": { "sample_index": 2, "quality": { "status": "accepted", "score": 0.71 } } }`.
- **Respons kegagalan sampel:** `422` dengan `SAMPLES_REJECTED` dan `details[].issue` bernilai `NO_FACE`, `MULTIPLE_FACES`, `LOW_QUALITY`, atau `FACE_TOO_SMALL`. Ini **penolakan sampel**, bukan kegagalan server.
- **Kode status:** `201`, `400`, `404`, `409`, `413`, `415`, `422`, `503`, `504`.
- **Efek database:** `INSERT INTO face_enrollment_samples` (menyimpan hanya metadata kualitas dan indeks, **bukan** JPEG mentah); menambah `face_enrollments.accepted_samples`.
- **Idempotensi:** `Idempotency-Key` dihormati; pengulangan dengan kunci sama mengembalikan hasil pertama tanpa menambah sampel.
- **Retensi:** JPEG sementara di memori dan dibuang setelah pemrosesan; tidak ada penyimpanan gambar mentah permanen secara bawaan (D-015).

#### `POST /api/v1/face-enrollments/{enrollment_id}/complete`

- **Peran:** admin.
- **Tubuh permintaan:** kosong.
- **Respons `200`:** `{ "data": { "template_id": "...", "model_id": "sface", "model_version": "..." , "created_at": "..." } }`.
- **Kode status:** `200`, `409`, `422`, `503`, `504`.
- **Kesalahan:** `INSUFFICIENT_SAMPLES` (minimal 3 sampel diterima), `ENROLLMENT_EXPIRED`, `CONSENT_REQUIRED`.
- **Efek database (satu transaksi):** menghitung embedding rata-rata dari sampel yang diterima melalui mesin AI, mengenkripsi hasilnya (D-022), `INSERT INTO face_templates`, menandai enrollment `completed`, lalu menghapus baris `face_enrollment_samples`.
- **Idempotensi:** menyelesaikan enrollment yang sudah selesai mengembalikan `409`.
- **Catatan:** bila `model_version` berbeda dari template yang sudah ada untuk peserta yang sama, template lama **tidak** dihapus otomatis; keputusan ada di `docs/FACE_ENGINE.md`.

#### `DELETE /api/v1/face-enrollments/{enrollment_id}`

- **Peran:** admin.
- **Respons:** `204`.
- **Kode status:** `204`, `404`, `409`.
- **Efek database:** enrollment `pending` dibatalkan; bila sudah `completed`, endpoint ini **tidak** menghapus template — penghapusan template memakai §5.3 withdraw atau jalur admin yang diaudit.
- **Idempotensi:** penghapusan kedua mengembalikan `404`.

### 5.5 Modul 5 — Kelas, keanggotaan, dan penugasan dosen

**Status implementasi (FL-011, 16 Oktober 2026).** **TERIMPLEMENTASI:** `GET /classes/{class_id}/dosen`, `PUT /classes/{class_id}/dosen/{user_id}`, dan `DELETE /classes/{class_id}/dosen/{user_id}` (penugasan dosen). **BELUM DIIMPLEMENTASIKAN (kontrak saja):** `GET /classes`, `POST /classes`, `GET /classes/{class_id}/members`, `PUT /classes/{class_id}/members/{person_id}`, dan `DELETE /classes/{class_id}/members/{person_id}`. Akibatnya kelas dan keanggotaan peserta **belum** dapat dibuat atau diubah lewat API; barisnya harus sudah ada di database. Perhatikan bahwa `GET /me/classes` (§5.14) **sudah** berjalan dan membaca penugasan/keanggotaan, sehingga isinya bergantung pada data yang belum ada jalur produknya.

**Catatan cakupan `dosen`:** `dosen` hanya melihat dan mengelola kelas yang **ditugaskan** kepadanya lewat `class_lecturers`. Objek kelas di luar penugasan → **`404 NOT_FOUND`** (bukan `403`), agar keberadaan kelas lain tidak dapat ditebak.

#### `GET /api/v1/classes`

- **Status (FL-011, 16 Oktober 2026): BELUM DIIMPLEMENTASIKAN (kontrak saja).**
- **Peran:** admin, dosen. Untuk `dosen`, respons hanya memuat kelas yang ditugaskan kepadanya.
- **Query:** `cursor`, `limit`.
- **Respons `200`:** daftar `{ id, code, title, member_count, active_session_id }`.
- **Kode status:** `200`, `401`.
- **Efek database:** —
- **Idempotensi:** aman diulang.

#### `POST /api/v1/classes`

- **Status (FL-011, 16 Oktober 2026): BELUM DIIMPLEMENTASIKAN (kontrak saja).**
- **Peran:** admin.
- **Tubuh permintaan:** `code` (wajib, unik), `title` (wajib).
- **Respons `201`:** objek kelas.
- **Kode status:** `201`, `400`, `409`.
- **Efek database:** `INSERT INTO classes`; audit `class.created`.
- **Idempotensi:** `code` unik membuat pengulangan menghasilkan `409`.

#### `GET /api/v1/classes/{class_id}/members`

- **Status (FL-011, 16 Oktober 2026): BELUM DIIMPLEMENTASIKAN (kontrak saja).**
- **Peran:** admin, dosen. Untuk `dosen`, hanya kelas yang ditugaskan kepadanya.
- **Respons `200`:** daftar peserta aktif kelas dengan `person_id`, `external_code`, `display_name`.
- **Kode status:** `200`, `404`.
- **Efek database:** —
- **Idempotensi:** aman diulang.

#### `PUT /api/v1/classes/{class_id}/members/{person_id}`

- **Status (FL-011, 16 Oktober 2026): BELUM DIIMPLEMENTASIKAN (kontrak saja).**
- **Peran:** admin.
- **Tubuh permintaan:** kosong.
- **Respons `201`** bila baru dibuat, `200` bila sudah menjadi anggota.
- **Kode status:** `200`, `201`, `404`.
- **Efek database:** `INSERT ... ON CONFLICT DO NOTHING` pada `class_memberships`; audit `class.member.added` hanya saat benar-benar bertambah.
- **Idempotensi:** **idempoten**; pengulangan tidak menghasilkan baris ganda.

#### `DELETE /api/v1/classes/{class_id}/members/{person_id}`

- **Status (FL-011, 16 Oktober 2026): BELUM DIIMPLEMENTASIKAN (kontrak saja).**
- **Peran:** admin.
- **Respons:** `204`.
- **Kode status:** `204`, `404`, `409`.
- **Validasi:** keanggotaan tidak boleh dihapus bila peserta memiliki catatan kehadiran pada sesi **terbuka** untuk kelas itu (`409`).
- **Efek database:** `DELETE FROM class_memberships`; audit `class.member.removed`.
- **Idempotensi:** penghapusan kedua `404`.

#### `GET /api/v1/classes/{class_id}/dosen`

- **Status (FL-011, 16 Oktober 2026): TERIMPLEMENTASI.** Cakupan `dosen` (hanya kelas yang ditugaskan) diperiksa di handler; kelas di luar cakupan → `404`.
- **Peran:** admin, dosen. Untuk `dosen`, hanya kelas yang ditugaskan kepadanya.
- **Respons `200`:** daftar `{ user_id, display_name, email, assigned_at }` untuk akun ber-role `dosen` yang ditugaskan pada kelas itu.
- **Kode status:** `200`, `403`, `404`.
- **Efek database:** —
- **Idempotensi:** aman diulang.

#### `PUT /api/v1/classes/{class_id}/dosen/{user_id}`

- **Status (FL-011, 16 Oktober 2026): TERIMPLEMENTASI.** Hanya akun ber-role `dosen` yang dapat ditugaskan.
- **Peran:** admin.
- **Tubuh permintaan:** kosong.
- **Respons `201`** bila penugasan baru dibuat, `200` bila penugasan sudah ada.
- **Kode status:** `200`, `201`, `400`, `404`, `409`.
- **Kesalahan:** `VALIDATION_FAILED` (akun bukan ber-role `dosen`), `NOT_FOUND`.
- **Aturan:** penugasan **hanya** untuk akun ber-role `dosen`; role `dosen` tidak dapat ditetapkan lewat endpoint ini (penetapan role ada di `PATCH /users/{user_id}`).
- **Efek database:** `INSERT INTO class_lecturers` (`class_id`, `user_id`, `assigned_by`, `assigned_at`); audit `class.dosen_assigned`.
- **Idempotensi:** **idempoten**; pengulangan tidak menghasilkan baris ganda.

#### `DELETE /api/v1/classes/{class_id}/dosen/{user_id}`

- **Status (FL-011, 16 Oktober 2026): TERIMPLEMENTASI.**
- **Peran:** admin.
- **Respons:** `204`.
- **Kode status:** `204`, `404`, `409`.
- **Efek database:** `DELETE FROM class_lecturers`; audit `class.dosen_removed`.
- **Idempotensi:** penghapusan kedua `404`.

### 5.6 Modul 6 — Sesi absensi

**Status implementasi (FL-011, 16 Oktober 2026): SELURUH modul ini BELUM DIIMPLEMENTASIKAN (kontrak saja)** — `GET /sessions`, `POST /sessions`, `GET /sessions/{session_id}`, `POST /sessions/{session_id}/open`, dan `POST /sessions/{session_id}/close` belum ada di kode. Tidak ada sesi absensi yang dapat dibuat, dibuka, atau ditutup lewat API.

**Catatan cakupan `dosen`:** `dosen` hanya boleh membuat, membuka, dan menutup sesi pada kelas yang ditugaskan kepadanya. Di luar itu → `404 NOT_FOUND`.

#### `GET /api/v1/sessions`

- **Peran:** admin, dosen. Untuk `dosen`, hanya sesi kelas yang ditugaskan kepadanya.
- **Query:** `class_id`, `status` (`draft`/`open`/`closed`), `from`, `to`, `cursor`, `limit`.
- **Respons `200`:** daftar sesi dengan ringkasan jumlah hadir.
- **Kode status:** `200`, `401`.
- **Efek database:** —
- **Idempotensi:** aman diulang.

#### `POST /api/v1/sessions`

- **Peran:** admin, dosen (untuk `dosen`, hanya kelas yang ditugaskan; di luar itu `404`).
- **Tubuh permintaan:** `class_id` (wajib), `device_id` (opsional), `closes_at` (wajib), `late_after` (opsional), `title` (opsional).
- **Respons `201`:** sesi berstatus `draft`.
- **Kode status:** `201`, `400`, `404`, `409`, `422`.
- **Validasi:** `closes_at` harus di masa depan dan setelah `late_after`; `device_id` tidak boleh sudah terikat sesi terbuka lain (`409 DEVICE_ALREADY_BOUND`); satu kelas maksimum satu sesi `open` (`409 SESSION_ALREADY_OPEN`).
- **Efek database:** `INSERT INTO attendance_sessions`; audit `session.created`.
- **Idempotensi:** `Idempotency-Key` dihormati untuk mencegah dua sesi ganda akibat klik dobel.

#### `GET /api/v1/sessions/{session_id}`

- **Peran:** admin, dosen (untuk `dosen`, hanya kelas yang ditugaskan; di luar itu `404`).
- **Respons `200`:** detail sesi termasuk `started_at`, `closes_at`, `late_after`, `status`, `device_id`, `counts` (`present`, `late`, `absent`, `pending_review`).
- **Kode status:** `200`, `404`.
- **Efek database:** —
- **Idempotensi:** aman diulang.

#### `POST /api/v1/sessions/{session_id}/open`

- **Peran:** admin, dosen (untuk `dosen`, hanya kelas yang ditugaskan; di luar itu `404`).
- **Tubuh permintaan:** kosong.
- **Respons `200`:** sesi berstatus `open` dengan `started_at`.
- **Kode status:** `200`, `404`, `409`, `422`.
- **Validasi:** hanya dari `draft`; `closes_at` belum lewat; perangkat terkait ada dan tidak dicabut.
- **Efek database:** `UPDATE attendance_sessions`; audit `session.opened`.
- **Idempotensi:** membuka sesi yang sudah `open` mengembalikan `409 SESSION_ALREADY_OPEN`.

#### `POST /api/v1/sessions/{session_id}/close`

- **Peran:** admin, dosen (untuk `dosen`, hanya kelas yang ditugaskan; di luar itu `404`).
- **Tubuh permintaan:** `reason` (opsional).
- **Respons `200`:** sesi berstatus `closed`.
- **Kode status:** `200`, `404`, `409`.
- **Efek database:** `UPDATE attendance_sessions.status='closed'`, `closed_at=now`; audit `session.closed`.
- **Idempotensi:** menutup sesi yang sudah tertutup mengembalikan `409 SESSION_CLOSED`.
- **Aturan penting:** sesudah tertutup, kiriman kamera tidak lagi boleh menambah catatan. Koreksi setelah penutupan hanya melalui §5.8 dan selalu diaudit.

### 5.7 Modul 7 dan 8 — Hasil verifikasi dan pencatatan kehadiran

**Status implementasi (FL-011, 16 Oktober 2026): BELUM DIIMPLEMENTASIKAN (kontrak saja)** — bergantung penuh pada §5.10 yang belum ada. Tidak ada verifikasi wajah maupun pencatatan kehadiran yang berjalan.

Tidak ada endpoint publik terpisah. Keduanya dijalankan di dalam §5.10. Tabel keputusan domain dan statusnya ada di §7. Pada revisi FL-010U tidak ada perubahan pada kedua modul ini.

### 5.8 Modul 9 — Koreksi manual dan jalur alternatif absensi

**Status implementasi (FL-011, 16 Oktober 2026): SELURUH modul ini BELUM DIIMPLEMENTASIKAN (kontrak saja)** — `POST` dan `GET /sessions/{session_id}/manual-checkins` belum ada di kode. Jalur absensi manual — yang di seluruh dokumen dinyatakan wajib tersedia sebagai alternatif — **belum berjalan**.

**Catatan cakupan `dosen`:** koreksi manual hanya untuk sesi kelas yang ditugaskan kepadanya. Di luar itu → `404 NOT_FOUND`.

#### `POST /api/v1/sessions/{session_id}/manual-checkins`

- **Peran:** admin, dosen (untuk `dosen`, hanya kelas yang ditugaskan; di luar itu `404`).
- **Tubuh permintaan:** `person_id` (wajib), `reason` (wajib, tidak boleh kosong, ≤200 karakter), `status` (wajib: `present`/`late`/`excused`), `checked_in_at` (opsional).
- **Respons `201`** bila catatan baru dibuat; `200` bila mengoreksi catatan yang sudah ada.
- **Kode status:** `201`, `200`, `400`, `404`, `409`, `422`.
- **Kesalahan:** `VALIDATION_FAILED`, `CLASS_MISMATCH`, `REASON_REQUIRED`, `NOT_FOUND`.
- **Validasi:** peserta wajib anggota kelas sesi. Koreksi **diperbolehkan** setelah sesi ditutup — inilah jalur yang menggantikan “peserta dianggap tidak hadir karena pengenalan gagal”.
- **Efek database:** `INSERT` atau `UPDATE attendance_records` dengan `source='manual'`, `created_by` = id pengguna yang mengoreksi (`admin`/`dosen`), `reason` wajib; audit `attendance.manual_corrected` yang mencatat nilai sebelumnya dan sesudahnya.
- **Idempotensi:** `Idempotency-Key` wajib bermanfaat di sini; tanpa kunci, permintaan kedua dengan isi sama menghasilkan `409 ALREADY_CHECKED_IN` kecuali `checked_in_at` dikirim eksplisit untuk koreksi.
- **Penting:** endpoint ini **tidak boleh** dipakai perangkat dan tidak boleh ada versi publik tanpa autentikasi `admin`/`dosen`.

#### `GET /api/v1/sessions/{session_id}/manual-checkins`

- **Peran:** admin, dosen (untuk `dosen`, hanya kelas yang ditugaskan; di luar itu `404`).
- **Respons `200`:** daftar koreksi manual pada sesi tersebut, beserta aktor dan alasannya.
- **Kode status:** `200`, `404`.
- **Efek database:** —
- **Idempotensi:** aman diulang.

### 5.9 Modul 10 — Pendaftaran dan autentikasi perangkat

**Status implementasi (FL-011, 16 Oktober 2026): SELURUH modul ini BELUM DIIMPLEMENTASIKAN (kontrak saja)** — `POST /devices`, `GET /devices`, `POST /devices/{device_id}/rotate-token`, dan `DELETE /devices/{device_id}` belum ada di kode. Tidak ada token perangkat yang dapat dibuat lewat API.

**Seluruh operasi perangkat hanya `admin`** (FL-010U). `dosen` tidak dapat memanggil `GET /devices` maupun endpoint perangkat lainnya.

#### `POST /api/v1/devices`

- **Peran:** admin.
- **Tubuh permintaan:** `display_name` (wajib), `kind` (wajib: `webcam`/`esp32_cam`).
- **Respons `201`:** `{ "data": { "id", "display_name", "kind", "device_token" } }`. `device_token` ditampilkan **sekali** dan tidak dapat dibaca lagi.
- **Kode status:** `201`, `400`, `403`.
- **Efek database:** `INSERT INTO devices` dengan `token_hash` (bukan token mentah) dan `token_prefix` untuk identifikasi administratif; audit `device.created`.
- **Idempotensi:** pengulangan membuat perangkat baru dengan token berbeda. Ini disengaja agar admin sadar; klien tidak boleh retry otomatis.

#### `GET /api/v1/devices`

- **Peran:** admin.
- **Respons `200`:** daftar `{ id, display_name, kind, revoked_at, last_seen_at, token_prefix }`.
- **Kode status:** `200`, `401`, `403`.
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

**Status implementasi (FL-011, 16 Oktober 2026): BELUM DIIMPLEMENTASIKAN (kontrak saja)** — `POST /devices/{device_id}/recognition-attempts` belum ada di kode, dan mesin AI Python belum dimulai. Seluruh tabel keputusan pada §7 belum berlaku pada runtime.

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

**Status implementasi (FL-011, 16 Oktober 2026): SELURUH modul ini BELUM DIIMPLEMENTASIKAN (kontrak saja)** — `GET /sessions/{session_id}/attendance` dan `GET /sessions/{session_id}/export.csv` belum ada di kode.

#### `GET /api/v1/sessions/{session_id}/attendance`

- **Peran:** admin, dosen (untuk `dosen`, hanya kelas yang ditugaskan; di luar itu `404`).
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

- **Peran:** **admin saja.** Ekspor CSV tidak tersedia untuk `dosen`; cakupan ini **diputuskan** oleh pemilik pada FL-010V sebagai bagian D-034 dan **tidak berubah** dari matriks FL-010U (lihat `docs/DECISIONS.md` D-034 dan `docs/AUTHORIZATION.md` §4.10, §9 butir 2).
- **Respons `200`:** `text/csv; charset=utf-8` dengan header `Content-Disposition`.
- **Kolom:** `external_code`, `display_name`, `status`, `source`, `checked_in_at`, `reason`, `session_title`, `class_code`.
- **Kode status:** `200`, `404`.
- **Efek database:** audit `session.exported` (jumlah baris, bukan isi).
- **Idempotensi:** aman diulang.
- **Aturan keras:** ekspor **tidak boleh** memuat embedding, skor kecocokan, jalur berkas gambar, atau ID template.

### 5.12 Modul 12 — Kesehatan dan kesiapan

**Status implementasi (FL-011, 16 Oktober 2026): KEDUA endpoint TERIMPLEMENTASI** — `GET /api/v1/healthz` (liveness) dan `GET /api/v1/readyz` (kesiapan). **Catatan apa adanya mengenai jalur:** kode yang berlaku memasang keduanya **di dalam** awalan versi, yaitu `/api/v1/healthz` dan `/api/v1/readyz`, sedangkan baris matriks §4 menyebut `GET /healthz` tanpa awalan. Matriks **tidak diubah** oleh FL-011; selisih penamaan jalur ini dicatat di sini dan di §10 agar tidak ada pembaca yang menganggap `/healthz` tanpa awalan sudah tersedia. Endpoint ini juga **tidak** memakai bentuk galat amplop kontrak (lihat §1.4).

`GET /healthz` adalah **satu-satunya** endpoint pada §5 yang dipanggil tanpa awalan `/api/v1` (lihat §1.1 butir 1 dan §4).

#### `GET /healthz` (tanpa awalan `/api/v1`)

- **Status (FL-011, 16 Oktober 2026): TERIMPLEMENTASI pada jalur `/api/v1/healthz`** (lihat catatan jalur di atas).
- **Peran:** publik (dibatasi jaringan lokal).
- **Respons `200`:** `{ "status": "ok", "version": "<versi build>", "time": "2026-10-15T08:00:00Z" }`.
- **Kode status:** `200`.
- **Aturan keras:** tidak boleh memuat jalur berkas, versi pustaka internal, nama basis data, atau status perangkat.

#### `GET /api/v1/readyz`

- **Status (FL-011, 16 Oktober 2026): TERIMPLEMENTASI.** Badan respons yang dihasilkan kode memuat `status` tambahan di samping `database`, `ai_engine`, `migrations`, dan `wal`. **Deviasi yang dicatat apa adanya:** pada kode yang berlaku endpoint ini terpasang **tanpa middleware autentikasi**, sehingga dapat dipanggil tanpa token meskipun matriks §4 menyebut peran `admin` dan `dosen`. Matriks tidak diubah oleh FL-011; perlindungan peran untuk jalur ini **belum sesuai matriks** dan dicatat pada §10.1.
- **Peran:** admin, dosen.
- **Respons `200`:** `{ "database": "ok", "ai_engine": "ok", "migrations": "applied", "wal": { "journal_mode": "wal" } }`.
- **Respons `503`:** salah satu komponen `unavailable` dengan `code` `AI_ENGINE_UNAVAILABLE` atau `DATABASE_BUSY`.
- **Kode status:** `200`, `503`.
- **Efek database:** pembacaan ringan; **tidak** menjalankan `PRAGMA integrity_check` penuh pada setiap permintaan.
- **Idempotensi:** aman diulang.

### 5.13 Modul 13 — Audit aktivitas penting

**Status implementasi (FL-011, 16 Oktober 2026): endpoint `GET /api/v1/audit-events` BELUM DIIMPLEMENTASIKAN (kontrak saja).** Penulis audit dengan daftar kunci metadata tertutup **sudah ada** dan dipakai endpoint autentikasi/administrasi FL-011, tetapi **belum ada endpoint untuk membaca** `audit_events`. Karena itu baris audit yang ditulis FL-011 hanya dapat diperiksa langsung di database, bukan lewat API.

#### `GET /api/v1/audit-events`

- **Status (FL-011, 16 Oktober 2026): BELUM DIIMPLEMENTASIKAN (kontrak saja).**
- **Peran:** **admin saja.** `dosen` tidak dapat membaca audit pada FL-010U.
- **Query:** `actor_id`, `action`, `entity_type`, `entity_id`, `from`, `to`, `cursor`, `limit`.
- **Respons `200`:** daftar `{ id, actor_type, actor_id, action, entity_type, entity_id, time, reason, safe_metadata }`.
- **Kode status:** `200`, `400`, `403`.
- **Aturan keras:** `safe_metadata` tidak boleh memuat JPEG, embedding, token, kata sandi, email, ID token, `sub`, data biometrik, atau **kredensial perangkat**. `reason` memuat alasan manusia untuk koreksi manual.
- **Efek database:** —
- **Idempotensi:** aman diulang.
- **Aksi yang berlaku saat ini (minimum, tetap dari rancangan sebelumnya):** `auth.logout`, `auth.session.revoked`, `user.created`, `person.created`, `person.updated`, `person.deleted`, `consent.granted`, `consent.withdrawn`, `enrollment.started`, `enrollment.completed`, `template.deleted`, `class.created`, `class.member.added`, `class.member.removed`, `session.created`, `session.opened`, `session.closed`, `attendance.recorded`, `attendance.manual_corrected`, `session.exported`, `device.created`, `device.token_rotated`, `device.revoked`, `migration.applied`.

**Perluasan daftar aksi minimum — BERLAKU (D-036, disetujui FL-010V):** daftar berikut **berlaku**, bukan usulan lagi, dan menjadi bagian daftar minimum audit keamanan: `auth.google.succeeded`, `auth.google.linked`, `auth.login.denied` (dengan `reason` ber-kode: `no_account`, `not_linked`, `suspended`, `email_mismatch`), `auth.session.refreshed`, `auth.session.revoked`, `auth.session.reuse_detected`, `auth.logout`, `authz.denied`, `auth.device.mismatch`, `user.bootstrapped`, `user.invited`, `user.role_changed`, `user.status_changed`, `person.account_linked`, `person.account_unlinked`, `class.dosen_assigned`, `class.dosen_removed`. `authz.denied` dan `auth.device.mismatch` yang sebelumnya masih berstatus usulan **kini ikut berlaku** melalui D-036. Untuk `auth.login.denied`, `safe_metadata` menyimpan `reason` ber-kode (`no_account`, `not_linked`, `suspended`, `email_mismatch`) — **bukan** email. Rujukan: `docs/DECISIONS.md` D-036, `docs/SECURITY_PRIVACY.md` §6, dan `docs/AUTHORIZATION.md` §6.2.

**Larangan isi audit (tetap, ditegaskan FL-010V):** `safe_metadata` dan isi audit **tidak boleh** memuat password, access token, refresh token, Google ID token, **kredensial perangkat**, embedding wajah, maupun foto wajah; gunakan identifier internal dan metadata seminimal mungkin.

**Status implementasi (FL-011, 16 Oktober 2026):** daftar di atas adalah **kebijakan yang berlaku**, dan **sebagian sudah ditegakkan kode**: penulis audit pada `internal/audit` memakai **daftar kunci metadata tertutup** (kunci di luar daftar ditolak sebelum ditulis), sehingga jaminan "tanpa email/token/embedding" tidak lagi hanya niat. Event yang benar-benar ditulis oleh jalur FL-011 adalah peristiwa autentikasi, akun, penautan peserta, dan penugasan dosen (mis. `auth.google.succeeded`, `auth.google.linked`, `auth.login.denied`, `auth.session.*`, `user.invited`, `user.role_changed`, `user.status_changed`, `user.bootstrapped`, `person.account_linked`, `person.account_unlinked`, `class.dosen_assigned`, `class.dosen_removed`) beserta `auth.session.reset_legacy` yang ditulis migrasi 00002. **Event yang bergantung pada endpoint produk yang belum ada** (mis. `consent.granted`, `enrollment.*`, `session.*`, `attendance.*`, `device.*`, `class.created`, `class.member.*`, `person.created/updated/deleted`, `session.exported`) **belum pernah ditulis**. Pencatatan audit keamanan baru boleh dianggap andal setelah tugas **FL-005S** (penguatan log respons & pemulihan panic) selesai — **FL-005S SELESAI secara implementasi pada 16 Oktober 2026** (cabang `fix/fl-005s-http-access-logging`), tetapi **belum di-merge** sampai PR ditinjau; lihat §10 dan `docs/TEST_PLAN.md` §4. Hasil pengujian nyata dicatat agen induk pada `PROJECT_STATE.md`; dokumen ini tidak menuliskan angka kelulusan.

### 5.14 Modul 14 — Portal self-service dosen dan mahasiswa

**Status implementasi (FL-011, 16 Oktober 2026).** **TERIMPLEMENTASI:** `GET /me/classes` (kelas yang ditugaskan untuk `dosen`, kelas yang diikuti untuk `mahasiswa`; akun `mahasiswa` tanpa penautan `people.account_user_id` dijawab `409 ACCOUNT_NOT_LINKED`). **BELUM DIIMPLEMENTASIKAN (kontrak saja):** `GET /me/schedule`, `GET /me/attendance`, dan `GET /me/consent` — ketiganya bergantung pada sesi absensi, kehadiran, dan konsen yang belum ada. Jadi dari empat endpoint portal, **hanya satu** yang berjalan, dan isinya masih bergantung pada data kelas/keanggotaan yang belum punya jalur API (§5.5).

Modul **baru** pada FL-010U. Portal self-service hanya membaca data milik sendiri; **tidak ada** endpoint `/me/*` yang mengubah data. Untuk akun `mahasiswa` yang belum tertaut baris `people` (`people.account_user_id` kosong), **keempat** endpoint pada modul ini mengembalikan **`409 ACCOUNT_NOT_LINKED`**, karena daftar kelas, jadwal, riwayat, maupun status konsen tidak dapat ditentukan tanpa penautan. Aturan ini ditetapkan pada FL-010U (bukan hasil uji) dan berstatus **BELUM TERVERIFIKASI**; lihat §10 butir 15. Penautan akun dilakukan `admin` lewat §5.2.

#### `GET /api/v1/me/classes`

- **Status (FL-011, 16 Oktober 2026): TERIMPLEMENTASI.**
- **Peran:** dosen, mahasiswa.
- **Cakupan:** `dosen` → kelas yang ditugaskan kepadanya; `mahasiswa` → kelas yang diikuti (keanggotaan aktif).
- **Query:** `cursor`, `limit`.
- **Respons `200`:** daftar `{ id, code, title, member_count, active_session_id }` dengan bentuk yang sama seperti §5.5.
- **Kode status:** `200`, `401`, `403`, `409`.
- **Kesalahan:** `ACCOUNT_NOT_LINKED` (409) untuk akun `mahasiswa` yang belum tertaut baris `people`; berlaku sama untuk keempat endpoint Modul 14 (§5.14 pembuka).
- **Efek database:** —
- **Idempotensi:** aman diulang.

#### `GET /api/v1/me/schedule`

- **Status (FL-011, 16 Oktober 2026): BELUM DIIMPLEMENTASIKAN (kontrak saja).**
- **Peran:** mahasiswa.
- **Cakupan:** sesi absensi kelas yang diikuti (`closes_at` belum lewat, atau sesi `closed` di dalam rentang kueri).
- **Query:** `from`, `to` (opsional), `cursor`, `limit`.
- **Respons `200`:** daftar `{ session_id, class_id, class_code, title, status, started_at, closes_at, late_after }`.
- **Kode status:** `200`, `401`, `403`, `409`.
- **Kesalahan:** `ACCOUNT_NOT_LINKED` (409) bila akun belum tertaut baris `people`.
- **Catatan keras:** **tidak ada tabel jadwal akademik pada MVP**. Yang ditampilkan di sini adalah sesi absensi nyata, bukan jadwal mata kuliah. Kebutuhan jadwal kuliah penuh dicatat sebagai pertanyaan terbuka (lihat §10 butir 14).
- **Efek database:** —
- **Idempotensi:** aman diulang.

#### `GET /api/v1/me/attendance`

- **Status (FL-011, 16 Oktober 2026): BELUM DIIMPLEMENTASIKAN (kontrak saja).**
- **Peran:** mahasiswa.
- **Cakupan:** riwayat kehadiran milik sendiri — `status`, `source`, `checked_in_at`, `reason` — **tanpa** data biometrik dan **tanpa** data peserta lain.
- **Query:** `class_id` (opsional), `from`, `to` (opsional), `cursor`, `limit`.
- **Respons `200`:** daftar `{ session_id, class_id, class_code, session_title, status, source, checked_in_at, reason }`.
- **Kode status:** `200`, `401`, `403`, `409`.
- **Kesalahan:** `ACCOUNT_NOT_LINKED` (409).
- **Aturan keras:** akses ke riwayat sendiri **tidak** memberi akses ke `face_templates`, embedding, atau data peserta lain.
- **Efek database:** —
- **Idempotensi:** aman diulang.

#### `GET /api/v1/me/consent`

- **Status (FL-011, 16 Oktober 2026): BELUM DIIMPLEMENTASIKAN (kontrak saja).**
- **Peran:** mahasiswa.
- **Cakupan:** status persetujuan biometrik milik sendiri — **baca saja**.
- **Respons `200`:** `{ "data": { "person_id", "consent_state", "records": [ { "purpose", "version", "capture_method", "granted_at", "withdrawn_at" } ] } }`. `consent_state` bernilai `active`, `none`, atau `withdrawn`.
- **Kode status:** `200`, `401`, `403`, `409`.
- **Kesalahan:** `ACCOUNT_NOT_LINKED` (409).
- **Catatan:** pencatatan dan pencabutan konsen tetap melalui jalur petugas/`admin` (§5.3) sesuai rancangan yang berlaku; **tidak ada** endpoint mahasiswa untuk mencatat atau mencabut konsen pada MVP (lihat §10 butir 4).
- **Efek database:** —
- **Idempotensi:** aman diulang.

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
| Penyimpanan template terenkripsi | **Go** (kunci di luar database dan di luar Git; bentuk penyimpanan AES-256-GCM final, pengelolaan kunci **D-030 (DISETUJUI pada FL-004S atas rancangan; belum diimplementasikan dan belum diuji)** dan dirinci di `docs/SECURITY_PRIVACY.md` §5a–§5d) |

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

1. **Kunci idempotensi** dikirim klien pada header `Idempotency-Key` (pengguna aplikasi: `admin`, `dosen`, `mahasiswa`) atau `X-Idempotency-Key` (perangkat), berupa string 8–128 karakter. Kunci disimpan pada `idempotency_records` bersama hash permintaan, kode status, dan badan respons ringkas.
2. Kunci yang sama dengan hash permintaan yang sama mengembalikan **respons pertama** tanpa efek samping baru.
3. Kunci yang sama dengan hash permintaan **berbeda** mengembalikan `409 IDEMPOTENCY_KEY_REUSED`.
4. Kunci disimpan maksimum 24 jam untuk menahan retry jaringan; setelah itu dihapus oleh proses pemeliharaan. **Database tetap menjadi penjaga terakhir**, bukan tabel kunci ini.
5. `UNIQUE(session_id, person_id)` pada `attendance_records` adalah penjaga terakhir terhadap absensi ganda. Aplikasi Go boleh memeriksa lebih dulu, tetapi pemeriksaan aplikasi saja **tidak cukup**.
6. Pemeriksaan konsen, keanggotaan kelas, dan status sesi dilakukan **di dalam** transaksi penulisan, bukan hanya pada awal permintaan. Sesi dapat ditutup atau konsen dicabut di antara dua langkah.
7. Bila penulisan gagal karena batasan unik, transaksi di-rollback dan backend membalas `already_recorded` (`200`) dengan mengambil catatan yang sudah ada — bukan `500`.
8. Waktu `checked_in_at` **tidak** diperbarui oleh percobaan ulang.
9. Penutupan sesi bersifat idempoten pada keadaan akhir (sesi tertutup), tetapi permintaan kedua tetap membalas `409` agar `admin`/`dosen` mengetahui klik berulang.

## 9. Pengujian kontrak wajib

Daftar berikut harus terwujud sebagai pengujian saat implementasi. **Belum ada pengujian yang dijalankan** karena belum ada kode aplikasi; seluruh harapan di bawah berstatus **BELUM TERVERIFIKASI**. Baris 1–2 diperbarui pada FL-010U untuk mengganti role `operator`; skenario tambahan untuk Google Sign-In, penautan akun, cakupan `dosen`, dan portal `/me/*` wajib disusun bersama `docs/AUTHORIZATION.md` pada tugas implementasi, bukan pada tahap dokumentasi ini.

**Catatan FL-011 (16 Oktober 2026).** Sebagian skenario di atas kini **berkaitan dengan endpoint yang sudah ada**, tetapi **status barisnya tidak diubah**. Baris 2 (`dosen` memanggil `/users` → `403 FORBIDDEN_ROLE`) kini menunjuk endpoint yang **sudah berjalan** sehingga dapat dijalankan sebagai uji otomatis; sebaliknya baris 1, 17, 19, 20, 21, dan 24 masih menyebut endpoint yang **belum diimplementasikan** (`/people`, ekspor CSV, koreksi manual, `/sessions/*`) sehingga tetap tidak dapat dijalankan. Baris 3 dan 22–23 menuntut endpoint perangkat yang belum ada. Rincian berkas uji nyata ada di `docs/TEST_PLAN.md` (blok "Status implementasi uji FL-011"). **Angka kelulusan tidak ditulis di dokumen ini**; hasil nyata dicatat agen induk pada `PROJECT_STATE.md`.

| # | Skenario | Harapan |
|---|---|---|
| 1 | Pengguna tanpa token memanggil `/people` | `401 UNAUTHENTICATED` |
| 2 | `dosen` memanggil `/users` | `403 FORBIDDEN_ROLE` |
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

**Revisi FL-010V (16 Oktober 2026).** Butir yang **sudah diputuskan pemilik** pada FL-010V **dihapus dari daftar ini** karena statusnya kini berlaku: cakupan `dosen` (D-034), perluasan daftar audit (D-036), pengikatan jaringan dan transport (D-035), lifecycle akun (D-032), dan aturan sesi (D-033). Penghapusan ini **bukan** penghapusan riwayat keputusan: alasan dan isi keputusannya tetap ada di `docs/DECISIONS.md` D-032–D-037, dan matriks §4 tidak berubah. Daftar di bawah hanya memuat hal yang **benar-benar masih terbuka**; tidak satu pun boleh dibaca sebagai keputusan yang berlaku.

Catatan riwayat yang dipertahankan: butir lama **"akun peserta" (D-013) yang akan menambah modul autentikasi kedua** sudah **DIGANTIKAN pada FL-010U (16 Oktober 2026)** — akun `mahasiswa` kini ada dan merupakan bagian keputusan yang sudah disetujui pemilik; penautannya ke baris `people` diatur pada §5.2 (`/people/{person_id}/account-link`) dan portal self-service-nya pada §5.14. Butir itu karena itu tidak lagi ada pada daftar di bawah, bukan karena ditolak.

1. **Pengelolaan kunci biometrik (Q-16).** Kebijakan lengkap ada sebagai **D-030 (DISETUJUI pada FL-004S)** di `docs/DECISIONS.md` dan `docs/SECURITY_PRIVACY.md` §5a–§5f. **Status: disetujui atas rancangan, belum diimplementasikan dan belum diuji**, sehingga tabel `face_templates` masih tidak boleh ditulis sebelum ada tugas implementasi tersendiri yang disetujui pemilik project — jangan menganggap persetujuan rancangan sebagai izin menulis kode.
2. **Angka batas** ukuran JPEG, resolusi maksimum, batas laju, dan `retry_after_hint_ms` masih kandidat dan harus **diukur** (Q-18), bukan ditetapkan dari perkiraan.
3. **Ambang kecocokan dan margin** belum ada (Q-07). Kontrak ini hanya menetapkan bahwa ambangnya milik Go dan harus terkalibrasi.
4. **Penarikan persetujuan mandiri oleh `mahasiswa` (Q-22) — tetap terbuka (FL-010V, 16 Oktober 2026).** FL-010V **tidak** menyetujui penarikan konsen mandiri. Pada rancangan yang berlaku, `GET /me/consent` (§5.14) tetap **baca saja** dan pencatatan/pencabutan konsen tetap melalui jalur `admin` (§5.3). Bila kelak dirancang, ia menambah capability baru pada matriks dan menuntut keputusan pemilik tersendiri.
5. **Prosedur offline saat sesi habis (Q-24) — tetap terbuka (FL-010V, 16 Oktober 2026).** Bila internet mati **dan** sesi aplikasi sudah tidak berlaku, **tidak ada bypass autentikasi** dan tidak ada jalur absensi di aplikasi; operasi manual mengikuti prosedur operasional aman lalu dimasukkan setelah operator terautentikasi kembali. Rincian prosedurnya belum diputuskan (`docs/OPEN_QUESTIONS.md` Q-24).
6. **Rincian sertifikat lokal dan trust chain Android belum diuji (Q-25) (FL-010V, 16 Oktober 2026).** Kebijakan transport pada §1.6 **disetujui pada tingkat kebijakan** (D-035), tetapi **rincian pelaksanaannya belum diuji**: Android 7+ **tidak** mempercayai CA yang dipasang pengguna untuk aplikasi kecuali aplikasi memakai `network_security_config` atau build kustom (Expo Go tidak dapat memuat konfigurasi itu), sehingga opsi CA lokal/reverse proxy menuntut development build/production build. Ini pekerjaan teknis lanjutan yang harus diuji; temuan dan batasannya ada di `docs/ARCHITECTURE.md` §5.1.
7. **Penamaan jalur `/auth/sessions` (FL-010V, 16 Oktober 2026).** `GET /auth/sessions` dan `DELETE /auth/sessions/{session_id}` adalah **nama yang berlaku pada kontrak ini** dan menggantikan nama lama `GET/DELETE /operator-sessions*` (§5.1, §2 butir 6). Ini **catatan penamaan, bukan perubahan izin**: metode, jalur, dan himpunan peran pada matriks §4 tidak berubah, dan penggantian nama tidak menambah maupun mengurangi akses peran mana pun.
8. **Rujukan rusak D-028 → `docs/UX_FLOWS.md` §10 — tetap masalah terbuka (FL-010V, 16 Oktober 2026).** Dokumen itu hanya memiliki §1–§8. Bagian yang dimaksud tidak boleh ditebak; pemilik perlu menyebut bagian yang benar. Rujukan ini tetap bermasalah dan **tidak** diperbaiki oleh FL-010V.
9. **Tugas penguatan log respons backend — FL-005S (FL-010V, 16 Oktober 2026) — SELESAI secara implementasi pada 16 Oktober 2026.** *Riwayat (tetap berlaku, tidak diubah):* pada FL-010V, access log Go yang berlaku saat itu dapat melaporkan status `200` untuk respons `503` batas waktu dan **tidak mencatat** respons `500` hasil pemulihan panic, sehingga pencatatan audit keamanan **belum boleh dianggap andal** sampai tugas **FL-005S** (penguatan log respons & pemulihan panic) selesai; tugas ini **tidak** dikerjakan pada FL-010V dan tidak ada kode yang diubah. *Pembaruan FL-005S (16 Oktober 2026):* pekerjaan itu **SELESAI secara implementasi** pada cabang `fix/fl-005s-http-access-logging` — `backend/internal/httpapi/router.go` menetapkan urutan middleware **satu kali** pada fungsi `chain` (`RequestID → echoRequestID → accessLog → recoverer → timeout → bodyLimit`), sehingga access log mencatat status final yang benar-benar dikirim klien (`503` batas waktu dan `500` panic tercatat) dan ditulis **tepat satu kali per permintaan** saat respons selesai; diuji oleh `backend/internal/httpapi/accesslog_test.go` (12 skenario) dengan gate lokal lulus. **Belum di-merge** ke `main` sampai PR ditinjau dan CI belum dilaporkan lulus, sehingga keandalan logging keamanan tetap tidak boleh dinyatakan terverifikasi secara operasional sebelum itu. Rujukan: `docs/SECURITY_PRIVACY.md` §6 dan `docs/TEST_PLAN.md` §4.
10. **Liveness** tidak dijanjikan ada pada MVP (D-029). Bila mekanisme dipilih, ia menambah endpoint atau kolom baru.
11. **Halaman pengelolaan perangkat di aplikasi** belum masuk PRD; endpointnya sudah disiapkan agar tidak menghambat.
12. **Kebijakan retensi catatan kehadiran** dan hubungannya dengan penghapusan peserta belum diputuskan (`docs/OPEN_QUESTIONS.md`).
13. **WebSocket** belum diaktifkan (D-021); bila kebutuhan muncul, ia akan menjadi modul baru, bukan perubahan pada endpoint rekap.
14. **Jadwal akademik penuh (Q-23) — tetap terbuka.** Tidak ada tabel jadwal mata kuliah pada MVP; `/me/schedule` (§5.14) hanya menampilkan sesi absensi nyata. Bila pemilik menghendaki jadwal kuliah penuh, itu model data dan endpoint baru.
15. **Perilaku `/me/*` pada akun `mahasiswa` yang belum tertaut — ditetapkan pada FL-010U.** Keempat endpoint Modul 14 memakai `409 ACCOUNT_NOT_LINKED` secara konsisten, karena daftar kelas, jadwal, riwayat, maupun status konsen tidak dapat ditentukan tanpa penautan `people.account_user_id`. Tetapan ini adalah perilaku rancangan yang **BELUM TERVERIFIKASI**, bukan hasil uji.

**Ketergantungan Google dan offline.** Google Sign-In **tidak boleh** dijanjikan berjalan offline. Selama sesi aplikasi masih sah, seluruh operasi aplikasi lewat LAN ke backend tetap berjalan tanpa memanggil Google; yang berhenti hanya login baru/re-autentikasi (pesan: "Perlu internet untuk masuk dengan Google"). Bila LAN ke backend mati, seluruh aplikasi berhenti karena semua keputusan ada di backend — **tidak ada** catatan kehadiran lokal. Ketentuan lengkap dan tabel ketersediaan layanan ada di `docs/ARCHITECTURE.md` §7.1 dan `docs/AUTHORIZATION.md` §9 butir 5.

### 10.1 Sisa pekerjaan setelah FL-011 (16 Oktober 2026)

Bagian ini **menambah** daftar di atas, bukan menggantikannya, dan hanya memuat pekerjaan yang **belum dikerjakan** pada FL-011. Tidak satu pun butir di bawah boleh dibaca sebagai sudah berjalan.

1. **Pembatasan laju (rate limit) BELUM diimplementasikan.** Endpoint `POST /auth/google` dan `POST /auth/refresh` **belum** memiliki pembatasan laju per alamat maupun per identitas; yang berlaku saat ini hanya **batas ukuran badan permintaan** (64 KiB untuk jalur autentikasi, 1 MiB dari middleware umum) dan batas waktu permintaan. Akibatnya kode kesalahan `429 RATE_LIMITED` **tidak pernah dihasilkan**, sementara matriks §1.5 dan `docs/AUTHORIZATION.md` §4.1 masih menyebutnya sebagai kewajiban tambahan endpoint tersebut. Kewajiban itu tetap berlaku sebagai sasaran dan **belum dipenuhi**.
2. **Bentuk galat belum diseragamkan.** `GET /api/v1/healthz`, `404` rute tidak dikenal, dan `405` metode tidak diizinkan masih memakai bentuk datar lama, berbeda dari amplop `{"error": {...}}` yang dipakai seluruh endpoint FL-011 (lihat §1.4). Penyeragaman ini pekerjaan tersendiri karena menyentuh kontrak respons yang sudah diuji FL-005.
3. **Selisih jalur `/healthz`.** Kode memasang liveness pada `/api/v1/healthz`, sedangkan matriks §4 menyebut `GET /healthz` tanpa awalan. Salah satu harus diselaraskan; sampai itu diputuskan, pembaca tidak boleh menganggap `/healthz` tanpa awalan tersedia (§5.12).
4. **`GET /api/v1/readyz` belum terlindungi peran.** Endpoint terpasang tanpa middleware autentikasi, sehingga dapat dipanggil tanpa token meskipun matriks §4 menyebut `admin` dan `dosen`. Ini **belum** sesuai matriks dan harus diperbaiki atau diputuskan ulang secara eksplisit; jangan menganggap kesiapan backend sudah terlindungi (§5.12).
5. **`GET /auth/sessions` belum mendaftar lintas akun.** Endpoint yang berjalan menuntut parameter `user_id` dan hanya mengembalikan sesi satu akun; belum ada tampilan sesi seluruh akun yang diasumsikan kontrak sebelumnya (§5.1).
6. **Endpoint produk yang belum ada tetap belum ada.** Modul konsen biometrik (§5.3), enrollment (§5.4), sesi absensi (§5.6), koreksi manual (§5.8), perangkat dan terminal kamera (§5.9, §5.10), rekap/ekspor (§5.11), `audit-events` (§5.13), serta `/me/schedule` `/me/attendance` `/me/consent` (§5.14) **belum diimplementasikan**. Konsekuensi yang harus disebut apa adanya: absensi (otomatis maupun manual) belum berjalan, konsen biometrik belum dapat dicatat, kelas/keanggotaan peserta belum punya jalur API, dan audit baru dapat dibaca langsung dari database. **RBAC untuk endpoint yang belum ada** sudah dirancang dan diuji sebagai kebijakan pada harness `internal/authz`, tetapi **belum** diuji lewat HTTP karena handler-nya belum ada.
7. **TLS LAN dan pengikatan non-loopback (Q-25) belum dikerjakan.** Kode masih menolak pengikatan ke alamat non-loopback, sehingga HP belum dapat menjangkau backend lewat Wi-Fi kelas; pengikatan LAN beserta TLS bersertifikat tepercaya **belum diimplementasikan maupun diuji** (D-035 tetap berlaku sebagai kebijakan, §1.6).

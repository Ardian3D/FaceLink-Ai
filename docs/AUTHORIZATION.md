# Matriks Peran dan Otorisasi FaceLink AI (MVP)

**Status:** **USULAN FINAL — MENUNGGU PERSETUJUAN PEMILIK PROJECT.** Dokumen ini memfinalkan peran, izin, dan aturan otorisasi untuk MVP, tetapi status keputusan **D-013** tetap `USULAN` sampai pemilik project menyetujuinya secara eksplisit (protokol `docs/DECISIONS.md` "Protokol perubahan keputusan").

**Lingkup:** demo **satu kelas, satu terminal** (`docs/PRD.md` §4). Tidak ada multi-tenant, multi-organisasi, SSO, billing, atau HRIS. Semua isi dokumen ini berlaku untuk MVP lokal dan **tidak** boleh diperluas diam-diam.

**Tanggal:** 16 Oktober 2026 · **Tugas:** FL-010 · **Cabang:** `docs/fl-010-authorization-contract`

**Rujukan yang mengikat:** `docs/PRD.md` §3 dan §7–§8 · `docs/API_CONTRACT.md` §1–§5 · `docs/DATA_MODEL.md` §4 · `docs/SECURITY_PRIVACY.md` §6 dan §5e · `docs/DECISIONS.md` D-010, D-013, D-014, D-022, D-027, D-028, D-030 · `docs/TEST_PLAN.md` §2 · `docs/UX_FLOWS.md` §2.

> Dokumen ini **tidak** mengubah kontrak API atau skema yang sudah disetujui. Setiap tempat yang berpotensi bertentangan dengan kontrak yang sudah disetujui ditandai pada §9 (Konflik dan hal yang butuh keputusan pemilik). Kode autentikasi dan otorisasi **belum ditulis**; ini rancangan, bukan laporan implementasi.

---

## 1. Prinsip otorisasi yang berlaku

1. **Deny by default.** Endpoint hanya dapat diakses bila peran pemanggilnya tercantum eksplisit pada matriks §4. Tidak ada endpoint yang terbuka bagi "semua yang sudah login" kecuali dinyatakan.
2. **Least privilege.** Setiap peran hanya mendapat hak yang dibutuhkannya untuk demo satu kelas. `operator` tidak mengelola akun operator lain; `device` hanya boleh mengirim satu jenis permintaan.
3. **Tiga jalur autentikasi yang terpisah dan tidak dapat saling menggantikan.** Token operator, identitas peserta (bila ada), dan token perangkat adalah tiga hal berbeda (§3). Token perangkat **tidak boleh** dipakai pada endpoint operator, dan sebaliknya (`docs/SECURITY_PRIVACY.md` §6).
4. **Otorisasi diperiksa di backend Go, tidak pernah di klien.** UI React Native hanya menyembunyikan tombol; keputusan izin, kepemilikan, dan keanggotaan selalu diambil ulang di backend (D-028, `docs/API_CONTRACT.md` §1.1 butir 3).
5. **Tidak membocorkan keberadaan sumber daya yang tidak boleh dilihat.** Sumber daya yang tidak terlihat oleh peran pemanggil mengembalikan `404 NOT_FOUND`, bukan `403`, agar tidak membocorkan bahwa sumber daya itu ada (`docs/API_CONTRACT.md` §1.5).
6. **Setiap penolakan yang bermakna keamanan dicatat.** Penolakan otorisasi pada jalur administratif dan absensi manual **wajib** meninggalkan `audit_events` (§6).
7. **Kepemilikan diperiksa, bukan hanya peran.** Peran yang sah pada objek yang bukan miliknya/kelasnya tetap ditolak (§5).

---

## 2. Peran final yang berlaku di MVP

### 2.1 Peran autentikasi (yang benar-benar ada di sistem)

Hanya tiga peran yang ada. Ini **sama** dengan `docs/API_CONTRACT.md` §1.2 dan `docs/DATA_MODEL.md` §4.1/§4.3 — FL-010 **memfinalkan** D-013, tidak menambah peran baru.

| Peran | Siapa | Kewajiban di MVP | Bukti referensi |
|---|---|---|---|
| `operator` | Dosen/pengelola kelas **dalam satu peran yang digabung** — "dosen" dan "admin kelas" adalah **orang yang sama** pada demo satu kelas | Wajib. Log in ke aplikasi, jalankan sesi absensi, kelola peserta, enrollment berizin, koreksi manual, ekspor | `docs/PRD.md` §3 (baris "Operator/dosen"), D-013, `docs/UX_FLOWS.md` §2 |
| `admin` | Operator dengan hak tambahan langka: mengelola akun operator lain dan mencabut/mengganti token perangkat | Wajib ada minimal satu akun, tetapi **bukan** peran terpisah dari operator | `docs/API_CONTRACT.md` §1.2 |
| `device` | Perangkat kamera (webcam atau ESP32-S3 CAM) yang mengunggah JPEG | Opsional pada demo awal (boleh hanya webcam); wajib saat fase IoT | D-014, `docs/API_CONTRACT.md` §1.2 |

**Catatan penting tentang konflik peran (temuan FL-010):** tidak ada peran `dosen` dan `mahasiswa` di tabel ini. `docs/PRD.md` §3 memang menyebut persona "operator/dosen" dan "peserta/mahasiswa", tetapi keduanya **bukan** peran autentikasi: "dosen" = `operator`, dan "mahasiswa" = **bukan pengguna aplikasi** di MVP (`docs/UX_FLOWS.md` §2: "Akun mahasiswa tidak wajib pada MVP"). Lihat §9 butir 1.

### 2.2 Aktor non-aplikasi

| Aktor | Apakah punya akun? | Bagaimana haknya ditentukan |
|---|---|---|
| Peserta/mahasiswa | **Tidak** pada MVP | Bukan subjek otorisasi API. Datanya dikelola **atas nama** oleh operator. Fitur "halaman peserta" (`docs/PRD.md` FR-16) bersifat P1 dan **tidak** mengubah peran MVP. |
| Mesin AI Python | Tidak | Bukan peran API publik. Berjalan di loopback `127.0.0.1` dengan kontrak internal `docs/API_CONTRACT.md` §6, tidak memegang kredensial database (D-027). |
| OmO / agent AI | Tidak | Alat pengembangan, bukan komponen runtime FaceLink AI (`AGENTS.md` §6). |

### 2.3 D-013 difinalkan sebagai berikut

D-013 menyatakan "Operator/admin sebagai satu-satunya pengguna aplikasi wajib di MVP; akun peserta opsional. Perangkat kamera memakai identitas terpisah." FL-010 **memvalidasi ini terhadap kontrak yang ada dan menyimpulkan** bahwa D-013 sudah sesuai dengan `docs/API_CONTRACT.md` §1.2 dan `docs/DATA_MODEL.md`. Karena itu, usulan FL-010 adalah **menaikkan D-013 dari `USULAN` menjadi `DISETUJUI`** pada `docs/DECISIONS.md`, dengan tambahan penegasan:

- Tidak ada peran `dosen` atau `mahasiswa` pada MVP; "dosen" diserap oleh `operator`, "mahasiswa" tidak memiliki akun.
- **`admin` adalah `operator` dengan hak tambahan, dan bukan peran yang terpisah jalurnya.** Setiap endpoint yang boleh dipanggil `operator` juga boleh dipanggil `admin` (`admin` ⊇ `operator`, terbukti pada §4.12: 32 jalur bersama). Yang membedakan hanyalah **delapan jalur khusus `admin`** (§4.12). Karena itu tidak ada endpoint yang menjadi milik `operator` dan tertutup bagi `admin`. Pengecualian tunggal berlaku untuk `recognition-attempts`: jalur itu bukan milik peran operator maupun admin, melainkan `device` saja (§4.9).
- **Tidak ada akun mahasiswa dan tidak ada peran `participant` pada MVP.** Hanya `admin` dan `operator` yang memiliki akun di tabel `users`; peran `device` bukan akun pengguna melainkan identitas perangkat (§3.2).
- Bila akun peserta kelak ditambahkan (pasca-MVP), ia **bukan** varian dari `operator`/`admin`, melainkan peran keempat (`participant`) dengan hak **baca-sendiri saja** (`docs/UX_FLOWS.md` §2), dan penambahannya menuntut migrasi `CHECK` peran di `users` serta kenaikan versi API (§9 butir 1).
- **Status D-013 tetap `USULAN` sampai pemilik menyetujui.** Agent tidak boleh mengubahnya sendiri (`docs/DECISIONS.md` baris 3).

---

## 3. Tiga jenis autentikasi yang dibedakan tegas

Ketiganya berbeda dalam kredensial, media penyimpanan hash, masa berlaku, cakupan, dan cara pencabutan. **Tidak ada satu pun yang dapat menggantikan yang lain.**

### 3.1 Token operator (`operator`/`admin`)

| Aspek | Ketetapan MVP |
|---|---|
| Kredensial awal | `email` + `password`; kata sandi di-hash lambat bergaram unik, minimal 8 karakter (`docs/API_CONTRACT.md` §5.1, `docs/DATA_MODEL.md` §4.1) |
| Bentuk token | Token akses berumur pendek (`expires_in` 3600 detik) + token penyegar; disimpan **hanya sebagai hash** pada `operator_tokens` (`docs/DATA_MODEL.md` §4.2) |
| Penyimpanan klien | Aplikasi operator lokal; **tidak boleh** ditulis ke log, Git, atau berkas konfigurasi |
| Masa berlaku | Token akses 1 jam; penyegar mencabut token lama (`docs/API_CONTRACT.md` §5.1 `refresh`) |
| Cakupan | Seluruh endpoint dengan peran `operator`/`admin` pada matriks §4 — **kecuali** kecuali yang khusus `admin` |
| Wajib ganti kata sandi | Akun baru memiliki `must_change_password = 1` (`docs/DATA_MODEL.md` §4.1) |
| Pencabutan | Logout mencabut sesi itu; admin mencabut sesi mana pun lewat `DELETE /operator-sessions/{session_id}` (`auth.session.revoked`) |
| Pembatasan | Batas laju ketat pada login; pesan kesalahan **tidak** membedakan "email tidak ada" dan "kata sandi salah" (`docs/API_CONTRACT.md` §5.1) |
| Perbedaan `operator` vs `admin` | Hanya lewat daftar izin §4. `admin` yang berwenang pada: `/users`, `/operator-sessions`, `DELETE /people`, `POST/DELETE /devices`, `rotate-token` |

### 3.2 Identitas peserta

**Tidak ada di MVP.** Peserta tidak memiliki kredensial, tidak log in, dan tidak dapat memanggil API. Identitasnya adalah **baris data** (`people`) yang dikelola operator. Konsekuensi otorisasi:

- Tidak ada endpoint yang menerima token peserta; tidak ada jalur "peserta melihat datanya sendiri" pada MVP.
- FR-16 (`docs/PRD.md`) adalah P1 dan **tidak boleh** diimplementasikan sebagai endpoint tanpa tugas terpisah dan tanpa meninjau ulang dokumen ini.

### 3.3 Token perangkat (`device`)

| Aspek | Ketetapan MVP |
|---|---|
| Pembuatan | Hanya `admin` lewat `POST /devices`; token ditampilkan **sekali**, disimpan sebagai `token_hash` (`docs/API_CONTRACT.md` §5.9, `docs/DATA_MODEL.md` §4.13) |
| Pengikatan | Token **terikat satu `device_id`**. `device_id` pada jalur tidak cocok dengan token → `403` (`docs/API_CONTRACT.md` §5.10) |
| Cakupan | **Hanya** `POST /api/v1/devices/{device_id}/recognition-attempts`. Tidak dapat membaca rekap, daftar peserta, konsen, atau apa pun (`docs/API_CONTRACT.md` §1.2) |
| Masa berlaku & rotasi | Rotasi lewat `POST /devices/{device_id}/rotate-token` (hanya `admin`); token lama tidak berlaku setelah rotasi |
| Pencabutan | `DELETE /devices/{device_id}` mengisi `revoked_at` (pencabutan lunak); perangkat tercabut → `403 DEVICE_REVOKED` |
| Aturan keras | `token_hash` dan `device_token` **tidak pernah** dikembalikan endpoint baca (`docs/API_CONTRACT.md` §5.9). Perangkat **tidak boleh** mengakses endpoint admin (`docs/SECURITY_PRIVACY.md` §6) |
| Penyimpanan di perangkat | Tidak boleh hardcode token di firmware (`docs/SECURITY_PRIVACY.md` §6); diprovisikan saat demo dan diganti bila bocor |

### 3.4 Matriks "jalur autentikasi × endpoint"

| Kategori endpoint | Token operator | Token perangkat | Tanpa token |
|---|---|---|---|
| `/healthz` | boleh | boleh (diabaikan) | **boleh** (probe lokal, `docs/API_CONTRACT.md` §4) |
| `/auth/login` | — | — | **boleh**, dengan batas laju ketat |
| `/readyz` dan seluruh `/people`, `/classes`, `/sessions`, `/devices` (baca), `/audit-events`, `/me` | **harus** | **ditolak** | ditolak (`401`) |
| `/devices/{id}/recognition-attempts` | **ditolak** | **harus** (token `device_id` cocok) | ditolak (`401`) |
| Endpoint internal `docs/API_CONTRACT.md` §6 (AI Engine) | bukan peran publik | bukan peran publik | hanya dari `127.0.0.1` |

---

## 4. Matriks otorisasi `endpoint × peran`

Diturunkan langsung dari `docs/API_CONTRACT.md` §4. **Total 43 baris endpoint**: 41 endpoint ber-versi (`/api/v1/...`) pada Modul 1–11 dan 13, **ditambah** `/healthz` dan `/readyz` dari Modul 12. Kolom "Peran yang diizinkan" adalah **satu-satunya** sumber kebenaran otorisasi; endpoint yang tidak tercantum **tidak boleh** dibuat.

Istilah pada bagian ini **tidak boleh** dicampur: "ber-versi" berarti endpoint berada di bawah prefiks `/api/v1/` dan **tetap menuntut token** kecuali dinyatakan lain; sedangkan **publik tanpa token** berarti endpoint yang boleh dipanggil tanpa autentikasi sama sekali — hanya `/auth/login` dan `/healthz`. Dari 43 endpoint, **2** dapat dipanggil tanpa token dan **41** menuntut token peran yang sah; jumlah 41 ini adalah soal autentikasi, bukan soal peran.

Keterangan: **A** = `admin`, **O** = `operator`, **D** = `device`, **P** = publik, **—** = tidak ada peran yang diizinkan (endpoint tidak ada).

### 4.1 Modul 1 — Autentikasi dan sesi operator

| Metode | Jalur | Peran | Kewajiban tambahan |
|---|---|---|---|
| `POST` | `/auth/login` | P | Batas laju ketat; audit tiap percobaan |
| `POST` | `/auth/refresh` | O, A | Token lama tidak dapat dipakai dua kali |
| `POST` | `/auth/logout` | O, A | Mencabut sesi sendiri |
| `GET` | `/me` | O, A | Mengembalikan peran sendiri; hash sandi tidak pernah dikirim |
| `GET` | `/operator-sessions` | **A saja** | Melihat sesi operator aktif |
| `DELETE` | `/operator-sessions/{session_id}` | **A saja** | Mencabut sesi operator; audit `auth.session.revoked` |

### 4.2 Modul 2 — Pengguna dan peserta

| Metode | Jalur | Peran | Kewajiban tambahan |
|---|---|---|---|
| `GET` | `/people` | O, A | Menghormati filter `class_id` setelah memeriksa keanggotaan |
| `POST` | `/people` | O, A | Audit `person.created` |
| `GET` | `/people/{person_id}` | O, A | `404` bila peserta sudah dihapus lunak |
| `PATCH` | `/people/{person_id}` | O, A | Audit daftar nama field yang berubah |
| `DELETE` | `/people/{person_id}` | **A saja** | Wajib menghapus template terkait dalam transaksi sama (D-015); `409 POLICY_CONFLICT` bila retensi belum ditetapkan |
| `GET` | `/users` | **A saja** | Daftar akun operator |
| `POST` | `/users` | **A saja** | Tidak ada pendaftaran mandiri; tidak ada manipulasi peran massal |

### 4.3 Modul 3 — Persetujuan biometrik

| Metode | Jalur | Peran | Kewajiban tambahan |
|---|---|---|---|
| `GET` | `/people/{person_id}/consent` | O, A | Menampilkan status, bukan isi biometrik |
| `POST` | `/people/{person_id}/consent` | O, A | Mencatat `recorded_by`; audit `consent.granted` |
| `POST` | `/people/{person_id}/consent/withdraw` | O, A | Wajib `withdraw_reason`; memicu prosedur penghapusan `docs/SECURITY_PRIVACY.md` §5f; audit `consent.withdrawn` |

### 4.4 Modul 4 — Enrollment wajah

| Metode | Jalur | Peran | Kewajiban tambahan |
|---|---|---|---|
| `POST` | `/people/{person_id}/face-enrollments` | O, A | **Wajib** konsen aktif; `422 CONSENT_REQUIRED` bila tidak |
| `GET` | `/people/{person_id}/face-enrollments` | O, A | Status/progres saja; tidak mengembalikan embedding atau gambar |
| `POST` | `/face-enrollments/{enrollment_id}/samples` | O, A | Validasi jenis/ukuran/waktu proses; gambar tidak disimpan permanen |
| `POST` | `/face-enrollments/{enrollment_id}/complete` | O, A | Audit `enrollment.completed` |
| `DELETE` | `/face-enrollments/{enrollment_id}` | O, A | Audit `template.deleted` |

### 4.5 Modul 5 — Kelas dan keanggotaan

| Metode | Jalur | Peran | Kewajiban tambahan |
|---|---|---|---|
| `GET` | `/classes` | O, A | Satu kelas pada demo |
| `POST` | `/classes` | O, A | Audit `class.created` |
| `GET` | `/classes/{class_id}/members` | O, A | — |
| `PUT` | `/classes/{class_id}/members/{person_id}` | O, A | Audit `class.member.added` |
| `DELETE` | `/classes/{class_id}/members/{person_id}` | O, A | Audit `class.member.removed` |

### 4.6 Modul 6 — Sesi absensi

| Metode | Jalur | Peran | Kewajiban tambahan |
|---|---|---|---|
| `GET` | `/sessions` | O, A | — |
| `POST` | `/sessions` | O, A | `created_by` = operator pemanggil; `409 SESSION_ALREADY_OPEN` bila kelas sudah punya sesi terbuka |
| `GET` | `/sessions/{session_id}` | O, A | — |
| `POST` | `/sessions/{session_id}/open` | O, A | Hanya dari `draft`; perangkat terkait harus ada dan tidak dicabut |
| `POST` | `/sessions/{session_id}/close` | O, A | Audit `session.closed`; setelah tertutup kiriman kamera ditolak |

### 4.7 Modul 9 — Koreksi manual dan jalur alternatif

| Metode | Jalur | Peran | Kewajiban tambahan |
|---|---|---|---|
| `POST` | `/sessions/{session_id}/manual-checkins` | O, A | **Wajib** `reason`; peserta harus anggota kelas sesi; **tidak boleh** dipakai perangkat (`docs/API_CONTRACT.md` §5.8); audit `attendance.manual_corrected` dengan nilai sebelum/sesudah |
| `GET` | `/sessions/{session_id}/manual-checkins` | O, A | Menampilkan aktor dan alasan |

### 4.8 Modul 10 — Perangkat

| Metode | Jalur | Peran | Kewajiban tambahan |
|---|---|---|---|
| `POST` | `/devices` | **A saja** | Token ditampilkan sekali; audit `device.created` |
| `GET` | `/devices` | O, A | `token_hash`/`device_token` tidak pernah dikembalikan |
| `POST` | `/devices/{device_id}/rotate-token` | **A saja** | Audit `device.token_rotated` |
| `DELETE` | `/devices/{device_id}` | **A saja** | Perangkat terikat sesi `open` → `409`; audit `device.revoked` |

### 4.9 Modul terminal kamera

| Metode | Jalur | Peran | Kewajiban tambahan |
|---|---|---|---|
| `POST` | `/devices/{device_id}/recognition-attempts` | **D saja** | Token harus cocok `device_id`; `X-Idempotency-Key` wajib; token operator **ditolak** |

### 4.10 Modul 11 dan 13 — Rekap, ekspor, audit

| Metode | Jalur | Peran | Kewajiban tambahan |
|---|---|---|---|
| `GET` | `/sessions/{session_id}/attendance` | O, A | Tanpa data biometrik |
| `GET` | `/sessions/{session_id}/export.csv` | O, A | Tanpa data biometrik; audit `session.exported` |
| `GET` | `/audit-events` | O, A | `safe_metadata` tidak memuat JPEG/embedding/token/sandi |

### 4.11 Modul 12 — Kesehatan dan kesiapan

| Metode | Jalur | Peran | Kewajiban tambahan |
|---|---|---|---|
| `GET` | `/healthz` | P (lokal) | Tidak membocorkan rahasia atau jalur berkas |
| `GET` | `/readyz` | O, A | Melaporkan kesiapan layanan absensi manual dan biometrik **terpisah** (`docs/SECURITY_PRIVACY.md` §5e) |

### 4.12 Rekapitulasi izin

| Peran | Jumlah endpoint yang diizinkan (dari 43) | Cakupan |
|---|---|---|
| `admin` | 40 | 32 jalur bersama `operator` + 8 jalur khusus `admin`. **Tidak** termasuk `recognition-attempts` (`D` saja) maupun `healthz` (publik) |
| `operator` | 32 | Seluruh jalur operasional kelas; 8 jalur khusus `admin` **ditolak** (`403`), dan `recognition-attempts` juga **ditolak** |
| `device` | 1 | Hanya `recognition-attempts` (`D` saja) |
| publik (tanpa token) | 2 | `login`, `healthz` |

Angka-angka ini dihitung dari tabel §4.1–§4.11 di atas (43 baris endpoint) dan **bukan** salinan dari `docs/API_CONTRACT.md`, yang tidak pernah menghitungnya secara eksplisit.

**Jumlah baris tabel di atas tidak boleh dibaca sebagai penjumlahan lintas peran.** Setiap endpoint dihitung tepat satu kali menurut cara aksesnya yang sah: 40 (token `admin`) + 1 (token `device`) + 2 (tanpa token) = 43. Karena itu `admin` **tidak** mengakses seluruh 43 endpoint: token `admin` **ditolak** pada `recognition-attempts` (`403 FORBIDDEN_ROLE`, §4.9 dan §5 butir 4) karena jalur itu milik domain perangkat/mesin AI, bukan jalur administratif. `admin` tetap **peran yang paling luas** — setiap endpoint yang boleh dipanggil `operator` juga boleh dipanggil `admin` (`admin` ⊇ `operator`), dan hanya `admin` yang boleh memanggil delapan jalur khusus di bawah ini.

**Delapan jalur khusus `admin`** (operator mendapat `403 FORBIDDEN_ROLE`): `GET /operator-sessions`, `DELETE /operator-sessions/{session_id}`, `DELETE /people/{person_id}`, `GET /users`, `POST /users`, `POST /devices`, `POST /devices/{device_id}/rotate-token`, `DELETE /devices/{device_id}`. Bila kontrak kelak menggabungkan `GET`/`POST /users` menjadi satu baris, angka 8 menjadi 7 **tanpa** mengubah izin apa pun; lihat §9 butir 3.

---

## 5. Aturan kepemilikan dan cakupan (deny by default)

1. **Cakupan data = satu kelas.** Operator hanya mengelola data kelas yang ada pada sistem demo. Karena MVP hanya memiliki satu kelas (`docs/PRD.md` §4), pemeriksaan keanggotaan tetap **wajib** ditulis sejak awal agar penambahan kelas kedua tidak membuka celah.
2. **Sesi milik kelasnya.** `POST /sessions/{session_id}/manual-checkins` dan seluruh operasi sesi hanya sah bila `session_id` ada. Koreksi manual menuntut `person_id` adalah anggota kelas sesi, jika tidak → `422 CLASS_MISMATCH`.
3. **Perangkat milik sesinya.** Token perangkat hanya sah untuk `recognition-attempts` pada `device_id` miliknya; perangkat lain → `403`. Perangkat yang tidak terikat sesi yang dikirim → `422 DEVICE_SESSION_MISMATCH`.
4. **`recognition-attempts` tidak pernah menerima token operator.** Bahkan `admin` **tidak boleh** menetapkan kehadiran lewat jalur kamera; itu jalur domain mesin AI, bukan jalur administratif.
5. **Peserta tidak dapat disentuh lintas kelas.** `GET /people?class_id=...`, `PUT/DELETE /classes/{class_id}/members/{person_id}` wajib memvalidasi `class_id` ada dan `person_id` ada sebelum menulis.
6. **Tidak ada eskalasi peran lewat API.** Tidak ada endpoint untuk mengubah `role` pengguna yang sudah ada; hanya `POST /users` (admin) yang menetapkan peran saat pembuatan (`docs/API_CONTRACT.md` §5.2 butir terakhir). Ini menutup jalur privilege-escalation.
7. **Sumber daya tersembunyi = `404`.** Bila objek ada tetapi pemanggil tidak berhak melihatnya dalam cakupannya, jawabannya `404 NOT_FOUND`, bukan `403`. `403` dipakai ketika peran **jelas** tidak berwenang atas operasi tersebut (mis. `operator` memanggil `/users`).

---

## 6. Aturan 401 vs 403 dan audit

### 6.1 Pembedaan 401 dan 403

| Kondisi | HTTP | `code` | Catatan |
|---|---|---|---|
| Tidak ada header token | `401` | `UNAUTHENTICATED` | — |
| Token tidak sah/rusak | `401` | `UNAUTHENTICATED` | Pesan tidak menjelaskan alasan detail |
| Token kedaluwarsa | `401` | `TOKEN_EXPIRED` | Klien memakai `refresh` |
| Token perangkat pada endpoint operator | `403` | `FORBIDDEN_ROLE` | Bukan `401`: tokennya sah, tetapi salah jenis |
| Token operator pada endpoint perangkat | `403` | `FORBIDDEN_ROLE` | Idem |
| `operator` memanggil endpoint `admin` | `403` | `FORBIDDEN_ROLE` | Tidak membocorkan bahwa operasi itu ada bagi peran lain selain pesan peran |
| Perangkat dengan `device_id` tidak cocok | `403` | `FORBIDDEN_ROLE` | `docs/API_CONTRACT.md` §5.10 |
| Perangkat dicabut/tidak aktif | `403` | `DEVICE_REVOKED` | `docs/API_CONTRACT.md` §1.5 |
| Objek ada tetapi di luar cakupan pemanggil | `404` | `NOT_FOUND` | Tidak membocorkan keberadaan |
| Objek benar-benar tidak ada | `404` | `NOT_FOUND` | Tidak dapat dibedakan dari baris di atas |

**Aturan keras:** perbedaan 401 vs 403 **tidak** boleh dipakai untuk memberi tahu klien hal yang tidak boleh diketahuinya. Pesan kesalahan tidak menyebut keberadaan sumber daya lain, dan tidak menyebut nama peran yang berwenang lebih dari yang sudah diketahui pemanggil.

### 6.2 Audit otorisasi dan tindakan administratif

`docs/API_CONTRACT.md` §5.13 menetapkan daftar aksi yang **wajib** diaudit. FL-010 menambahkan ketetapan **kapan** penolakan dicatat:

| Peristiwa | Apakah dicatat ke `audit_events`? | Alasan |
|---|---|---|
| `auth.login.failed` | **Ya** | Terhadap akun mana pun; tanpa email pada `safe_metadata` |
| Token perangkat ditolak karena `device_id` tidak cocok | **Ya**, aksi `auth.device.mismatch` (usulan baru, §9 butir 4) | Indikasi perangkat/salin konfigurasi yang salah |
| `operator` ditolak pada endpoint `admin` | **Ya**, aksi `authz.denied` (usulan baru, §9 butir 4) | Indikasi penyalahgunaan atau salah konfigurasi |
| Permintaan tanpa token pada endpoint terlindungi | **Tidak wajib** | Volume tinggi, tanpa aktor teridentifikasi; cukup log biasa tanpa PII |
| Permintaan ke sumber daya di luar cakupan (`404`) | **Tidak wajib** | Tidak dapat dibedakan dari "tidak ada" tanpa membocorkan |
| `consent.granted`/`withdrawn`, `attendance.manual_corrected`, `device.revoked`, `session.exported` | **Ya** (sudah di kontrak) | Jejak akuntabilitas wajib |

**Bidang audit yang dilarang:** `safe_metadata` **tidak pernah** memuat JPEG, embedding, token, hash kata sandi, nilai biometrik, atau alamat email (`docs/API_CONTRACT.md` §5.13, `docs/SECURITY_PRIVACY.md` §6). Untuk `authz.denied`, cukup simpan `actor_id`, peran, jalur, metode, dan `request_id`.

### 6.3 Gagal-aman

Sesuai `docs/SECURITY_PRIVACY.md` §5e butir 4, otorisasi dan autentikasi operator **tidak boleh** dilumpuhkan agar layanan terlihat hidup. Bila komponen autentikasi tidak sehat, endpoint operator **wajib** menolak melayani; ini kegagalan yang benar, bukan alasan menurunkan pemeriksaan. Absensi manual tetap menjadi jalur tetap, tetapi **hanya melalui endpoint terautentikasi** `POST /sessions/{session_id}/manual-checkins` — tidak pernah melalui jalur yang menonaktifkan keamanan.

---

## 7. Siklus hidup entitas dan aturan lifecycle otorisasi

| Entitas | Keadaan | Siapa yang boleh mengubah | Aturan otorisasi terkait |
|---|---|---|---|
| `users` | aktif / `is_active=0` | Hanya `admin` membuat; sistem menonaktifkan | Akun **tidak dihapus keras** selama menjadi `actor_id` audit (`docs/DATA_MODEL.md` §4.1); operator tidak dapat menonaktifkan dirinya sendiri (usulan baru, §9 butir 5) |
| `operator_tokens` | aktif / revoked / expired | Pemilik (logout) atau `admin` (cabut) | Token tidak dapat dipakai setelah `revoked_at`; baris kedaluwarsa dibersihkan pemeliharaan |
| `people` | active/inactive/soft-deleted | O, A; **hapus = admin** | Hapus menuntut penghapusan template dalam transaksi sama (D-015) |
| `classes` | aktif / soft-deleted | O, A | Penghapusan bertahap; keanggotaan tidak lunak (`docs/DATA_MODEL.md` §4.6) |
| `class_memberships` | ada / tidak ada | O, A | Penghapusan keras, tanpa makna historis sendiri |
| `attendance_sessions` | draft → open → closed | O, A | Satu sesi `open` per kelas **dan** per perangkat dijaga indeks unik parsial (`docs/DATA_MODEL.md` §4.7) |
| `attendance_records` | ditulis sekali, dikoreksi in-place | Backend (device/manual) | **Tidak pernah** ditulis `absent`; koreksi memperbarui baris yang sama |
| `consent_records` | aktif / withdrawn | O, A | Pencabutan memicu penghapusan template dan menghentikan pencocokan |
| `face_enrollments`/`samples` | berjalan / selesai / dibatalkan | O, A | Wajib konsen aktif; gambar tidak ditahan |
| `face_templates` | ada / dihapus | O, A | **Belum diimplementasikan** (FL-005 sengaja menundanya); kebijakan di D-022/D-030 |
| `devices` | aktif / revoked | **admin** | Pencabutan lunak; token perangkat terikat satu device |
| `audit_events` | hanya tulis | Sistem | Tidak ada endpoint yang mengubah/menghapus audit; hanya `GET /audit-events` |

**Aturan turunan yang mengikat otorisasi:**

- **Tidak ada jalur yang menulis keputusan kehadiran tanpa autentikasi.** Baik `recognition-attempts` (device) maupun `manual-checkins` (operator) menuntut token.
- **Perubahan izin tidak menyentuh biometrik.** Menambah/menonaktifkan operator atau mencabut perangkat **tidak boleh** menghapus atau membuat template (`docs/SECURITY_PRIVACY.md` §5e butir 2).
- **Koreksi setelah sesi ditutup hanya lewat jalur manual teraudit** (`docs/API_CONTRACT.md` §5.6 aturan penting), dan itu **hanya** `operator`/`admin`.

---

## 8. Kriteria penerimaan dan skenario uji penolakan akses

Mengunci keputusan FL-010 sebagai **kontrak yang dapat diuji** untuk FL-011 (implementasi autentikasi) dan seterusnya.

### 8.1 Kriteria penerimaan otorisasi

| ID | Kriteria penerimaan |
|---|---|
| AC-AUTHZ-01 | Setiap endpoint §4 menolak pemanggil di luar peran yang tercantum: `401` bila tanpa/token tidak sah, `403 FORBIDDEN_ROLE` bila peran salah. |
| AC-AUTHZ-02 | Token perangkat **tidak** dapat memanggil endpoint apa pun kecuali `recognition-attempts`; diuji untuk minimal `/people`, `/sessions`, `/devices`, `/audit-events`. |
| AC-AUTHZ-03 | Token operator **tidak** dapat memanggil `recognition-attempts` (bahkan sebagai `admin`). |
| AC-AUTHZ-04 | Operator mendapat `403 FORBIDDEN_ROLE` pada delapan jalur khusus admin (§4.12). |
| AC-AUTHZ-05 | Sumber daya di luar cakupan mengembalikan `404 NOT_FOUND`, bukan `403`, dan tidak membocorkan keberadaan objek. |
| AC-AUTHZ-06 | `POST /sessions/{session_id}/manual-checkins` menolak penulisan tanpa `reason` dan tanpa anggota kelas. |
| AC-AUTHZ-07 | Setiap penolakan otorisasi pada jalur administratif/absensi manual masuk `audit_events` **tanpa** data biometrik, token, atau email. |
| AC-AUTHZ-08 | Tidak ada endpoint yang mengubah peran pengguna setelah akun dibuat (tidak ada eskalasi). |
| AC-AUTHZ-09 | Ketika komponen autentikasi tidak sehat, endpoint operator menolak melayani (gagal-aman), bukan melewati pemeriksaan. |
| AC-AUTHZ-10 | `GET /audit-events` tidak mengembalikan `safe_metadata` yang memuat token, sandi, JPEG, atau embedding. |

### 8.2 Skenario uji penolakan akses (negatif)

Melengkapi `docs/TEST_PLAN.md` §2 (T-01, T-02, T-15). Diberi ID `TA-xx` agar dapat dirujuk tanpa menimpa penomoran T yang sudah ada.

| ID | Skenario | Harapan |
|---|---|---|
| TA-01 | Tanpa token memanggil `GET /people` | `401 UNAUTHENTICATED`; tanpa data |
| TA-02 | Token kedaluwarsa memanggil `GET /sessions` | `401 TOKEN_EXPIRED`; tidak ada data |
| TA-03 | Token perangkat memanggil `GET /people`, `/sessions`, `/devices`, `/audit-events` | `403 FORBIDDEN_ROLE` pada keempat jalur |
| TA-04 | Token operator memanggil `POST /devices/{id}/recognition-attempts` | `403 FORBIDDEN_ROLE` (bukan 200) |
| TA-05 | Token perangkat memakai `device_id` perangkat lain | `403`; audit `auth.device.mismatch` (usulan) |
| TA-06 | `operator` memanggil `GET /users` dan `POST /users` | `403 FORBIDDEN_ROLE`; audit `authz.denied` (usulan) |
| TA-07 | `operator` memanggil `DELETE /people/{id}` dan `POST /devices` | `403 FORBIDDEN_ROLE` |
| TA-08 | `operator` mencoba mengubah `role` dirinya sendiri | Endpoint tidak ada → `404`/`405`; tidak ada perubahan peran |
| TA-09 | Koreksi manual untuk peserta bukan anggota kelas | `422 CLASS_MISMATCH`; tidak ada baris kehadiran |
| TA-10 | Koreksi manual tanpa `reason` | `400`/`422 REASON_REQUIRED` |
| TA-11 | Perangkat terikat sesi A mengirim percobaan untuk sesi B | `422 DEVICE_SESSION_MISMATCH` |
| TA-12 | Perangkat tercabut mengirim `recognition-attempts` | `403 DEVICE_REVOKED` |
| TA-13 | Akses objek kelas lain oleh operator | `404 NOT_FOUND`; tidak membocorkan jumlah kelas |
| TA-14 | `GET /audit-events` pada sistem yang punya koreksi manual | `safe_metadata` bebas token/sandi/JPEG/embedding |
| TA-15 | Komponen autentikasi sengaja dimatikan, lalu `POST /manual-checkins` | Ditolak (gagal-aman); absensi manual hanya lewat jalur terautentikasi yang sehat |

**Catatan pelaksanaan:** skenario ini **belum dijalankan** karena kode autentikasi belum ada. Statusnya **BELUM TERVERIFIKASI** dan menjadi bagian tugas implementasi berikutnya.

---

## 9. Konflik, koreksi, dan hal yang butuh keputusan pemilik

Bagian ini dicatat apa adanya. FL-010 **tidak** mengubah kontrak yang sudah disetujui; hal-hal berikut menuntut persetujuan pemilik sebelum diterapkan pada dokumen atau kode.

1. **Status D-013 masih `USULAN`.** FL-010 mengusulkan **D-013 → `DISETUJUI`** dengan penegasan §2.3. Status di `docs/DECISIONS.md` **belum** diubah pada tugas ini; hanya ditambahkan catatan bahwa usulan finalnya ada di dokumen ini.
2. **`docs/PRD.md` §3 menyebut persona "dosen" dan "mahasiswa"** sebagai pengguna. Dokumen ini menegaskan keduanya **bukan** peran autentikasi (`dosen` = `operator`; `mahasiswa` tanpa akun di MVP). Tidak ada perubahan pada PRD; perbedaan istilah dicatat di sini agar tidak disalahpahami sebagai peran baru.
3. **Keselarasan jumlah endpoint khusus admin.** `docs/API_CONTRACT.md` §4 tidak pernah menghitung jumlahnya secara eksplisit; §4.12 dokumen ini menghitungnya. Bila pemilik menghendaki penghitungan ringkas yang berbeda (mis. menggabungkan `GET`/`POST /users` menjadi satu baris), angka pada tabel itu dapat disesuaikan tanpa mengubah izin apa pun.
4. **Usulan dua aksi audit baru:** `authz.denied` dan `auth.device.mismatch`. Keduanya **belum** ada pada daftar wajib `docs/API_CONTRACT.md` §5.13. Menambahkannya mengubah daftar audit minimum, sehingga butuh persetujuan pemilik sebelum masuk ke kontrak implementasi.
5. **Usulan larangan operator menonaktifkan akunnya sendiri.** Belum diatur di `docs/API_CONTRACT.md`. Diusulkan agar operator tidak bisa mengunci dirinya keluar dari sistem demo satu akun.
6. **FR-16 (halaman peserta) tetap P1 dan tidak diimplementasikan.** Bila kelak dikerjakan, ia menuntut peran keempat (`participant`), migrasi `CHECK` peran di `users`, dan revisi dokumen ini.
7. **D-014 dan D-015 juga masih `USULAN`.** Keduanya relevan (kios satu orang per pemindaian; penyimpanan template terenkripsi), tetapi FL-010 tidak mengusulkan perubahan statusnya karena bukan lingkup peran/otorisasi. Disebut agar tidak dianggap sudah final.

### 9.1 Koreksi FL-010R (16 Oktober 2026)

FL-010R memeriksa ulang seluruh 43 endpoint dengan pemeriksaan programatis terhadap `docs/API_CONTRACT.md` §4 dan menemukan **kesalahan hitung pada rekap §4.12**, bukan kesalahan izin. Ringkasannya:

| Klaim | Sebelum FL-010R | Sesudah FL-010R | Sebab |
|---|---|---|---|
| `admin` | 43 | **40** | `recognition-attempts` bersifat `D` saja dan `healthz` publik; keduanya tidak dapat dipanggil token `admin` |
| `operator` | 40 | **32** | Tertukar dengan jumlah `admin`; `operator` hanya memegang 32 jalur bersama |
| `device` | 1 | 1 | Sudah benar |
| publik (tanpa token) | 2 | 2 | Sudah benar |
| Total endpoint | 43 | 43 | Sudah benar |

**Hak akses endpoint tidak diubah sama sekali oleh koreksi ini.** Yang berubah hanya angka rekap dan penjelasannya. Pemeriksaan programatis membuktikan tabel §4.1–§4.11 sama persis dengan `docs/API_CONTRACT.md` §4: 43 baris, tanpa endpoint hilang atau endpoint tambahan, dengan peran yang identik. Aturan tunggal yang dipakai: setiap endpoint dihitung tepat satu kali menurut pemanggil sahnya → 40 (`admin`) + 1 (`device`) + 2 (publik tanpa token) = 43.

**Rekomendasi yang tetap menunggu persetujuan pemilik** (tidak satu pun status keputusan diubah oleh FL-010R):

- **a.** Penegasan **D-013** — `admin`/`operator`/`device` sebagai peran final MVP, tanpa `dosen`/`mahasiswa`/`participant` — diusulkan naik dari `USULAN` menjadi `DISETUJUI` (§2.3, butir 1 di atas).
- **b.** Dua aksi audit tambahan, `authz.denied` dan `auth.device.mismatch`, untuk masuk daftar audit minimum `docs/API_CONTRACT.md` §5.13 (butir 4 di atas).
- **c.** Pencegahan **penonaktifan akun sendiri** dan **admin terakhir**, tanpa menghalangi logout normal. Diusulkan agar: (i) `operator`/`admin` **tidak** dapat menonaktifkan akunnya sendiri; (ii) `admin` **tidak** dapat menonaktifkan admin aktif terakhir; sedangkan `POST /auth/logout` **selalu** diizinkan bagi pemilik sesi dan **tidak** terkena larangan ini — logout mencabut sesi, bukan menonaktifkan akun. Butir ini memperluas butir 5 di atas dan belum diatur di `docs/API_CONTRACT.md`.

---

## 10. Yang secara sengaja tidak ada di MVP

- Tidak ada multi-tenancy, organisasi, atau isolasi antar-kelas di atas satu kelas.
- Tidak ada SSO, OAuth, atau identity provider eksternal.
- Tidak ada peran dinamis/tabel izin-detail (`docs/DATA_MODEL.md` §4.3: `roles` hanyalah daftar acuan, bukan sistem izin dinamis).
- Tidak ada akun peserta, tidak ada self-service.
- Tidak ada endpoint yang menulis kehadiran tanpa token.
- Tidak ada cloud, billing, HRIS, atau CCTV terus-menerus (D-016 `DITUNDA`).

---

## 11. Rujukan silang

| Dokumen | Bagian yang dikunci oleh dokumen ini |
|---|---|
| `docs/API_CONTRACT.md` | §1.2 (peran), §4 (per-endpoint), §5.1/§5.8/§5.9/§5.10/§5.13 (aturan spesifik) |
| `docs/DATA_MODEL.md` | §4.1–§4.3 (peran), §4.4–§4.13 (lifecycle entitas) |
| `docs/SECURITY_PRIVACY.md` | §6 (kendali akses), §5e (gagal-aman dan kesiapan terpisah) |
| `docs/DECISIONS.md` | D-013 (difinalkan di sini), D-014, D-015, D-027, D-028 |
| `docs/TEST_PLAN.md` | §2 (T-01/T-02/T-15 dan skenario baru TA-01..TA-15) |
| `docs/UX_FLOWS.md` | §2 (akun mahasiswa tidak wajib pada MVP) |
| `docs/PRD.md` | §3 (persona), §7–§8 (FR dan aturan bisnis) |

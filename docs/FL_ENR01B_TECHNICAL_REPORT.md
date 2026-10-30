# Laporan Teknis FL-ENR01B — Backend Enrollment Sintetis

**Tanggal:** 26 Oktober 2026
**Baseline:** `feat/fl-otp01-email-auth`, HEAD `87bac5e96602e9ff5568069d1c6b1bd1d96913b7`
**Status:** SELESAI untuk backend **sintetis/disposable**. **NO-GO** manusia,
production, migrasi database Product Owner, dan implementasi FL-ENR02 otomatis.
Tidak ada commit, push, merge, atau force push.

## A. Baseline kontrak

Dibaca dan dipakai sebagai sumber: `AGENTS.md`, `README.md`, `PROJECT_STATE.md`,
`docs/PRD.md`, `docs/DECISIONS.md`, `docs/ARCHITECTURE.md`,
`docs/SECURITY_PRIVACY.md`, `docs/API_CONTRACT.md` §5.3–§5.4,
`docs/DATA_MODEL.md` §4.9–§4.13, `docs/AUTHORIZATION.md`,
`docs/FACE_ENROLLMENT_V1.md`, `docs/FACE_ENROLLMENT_DECISION_REVIEW.md`, dan
`docs/TEST_PLAN.md`.

Kontrak yang dipertahankan:

- Consent biometrik eksplisit, dapat ditarik, dan bukan syarat absensi manual.
- Satu JPEG pending per submission, canonical, immutable, terenkripsi, TTL 72 jam.
- Review Admin berupa attestation identitas kampus sintetis; bukan verifikasi
  otomatis registrar dan bukan keputusan identitas oleh model.
- ACTIVE hanya setelah template terenkripsi tersimpan dan seluruh fence valid.
- Peran existing tidak diubah: `admin`, `dosen`, `mahasiswa`; `device` terpisah.
- OTP/session, akun testing, mobile UI, SMTP/DNS, video/backup tidak diubah.
- D-030 tetap kebijakan kunci; konflik timer recovery 30 hari masih **BLOCKED**.

Pekerjaan R5, FL-CORE01, FL-CORE02, FL-ENR00, dan FL-ENR00R di worktree
dipertahankan. Tidak ada reset, stash, atau pembuangan perubahan.

## B. Persetujuan dan batas

Yang diimplementasikan hanya jalur nonproduksi yang dapat diinjeksi secara
eksplisit. `cmd/server` tidak membuat atau menginjeksi `enrollment.Service`.
Tidak ada foto wajah manusia, capture kamera, model YuNet/SFace nyata, API
production, atau migrasi ke database Product Owner. Processor yang dipakai
command smoke adalah fixture sintetis yang tidak mendeteksi identitas.

## C. Implementasi

- `internal/enrollment`: state machine `NOT_ENROLLED → PENDING_APPROVAL →
  APPROVED_PROCESSING → ACTIVE`, plus `REJECTED`, `FAILED_PROCESSING`,
  `EXPIRED`, dan `REVOKED`; consent `NOT_GIVEN/ACTIVE/WITHDRAWN`.
- Consent self-service dengan notice version/digest, ETag/CAS, idempotency,
  withdrawal tanpa key/If-Match, assisted withdrawal Admin untuk subjek
  suspended, dan replay yang membaca state terbaru.
- Submission menerima satu JPEG canonical; hash SHA-256, ukuran, consent,
  person/link/roster, dan aktor diperiksa dalam transaksi. Tidak ada
  `user_id`/`person_id` yang dipercaya dari klien pada rute self.
- Review Admin memakai `If-Match` kuat dan `Idempotency-Key` UUIDv4; attestation
  `CAMPUS_AUTHORIZED_CHECK`/`CAMPUS_ROSTER_IN_PERSON` berlabel sintetis.
- Approval hanya mengantrekan pekerjaan. Worker memeriksa ulang aktor, sesi,
  subjek, consent, reviewer epoch, approval, hash foto, retensi, dan manifest
  sebelum menyimpan template.
- Template berupa 128 `float32` sintetis dalam envelope AES-256-GCM; aktivasi,
  audit, dan referensi foto berada dalam transaksi yang sama.
- Rekonsiliasi menangani staging yang sudah commit, worker yatim, file orphan,
  serta retry pembersihan. Key-loss tidak dijadikan izin penghapusan.
- Kegagalan inference biasa (tanpa wajah, banyak wajah, NaN, nol, norma salah,
  manifest salah) berakhir `FAILED_PROCESSING` tanpa template dan foto dipurge.
- Kegagalan key/integritas mempertahankan ciphertext dengan
  `FAILED_PROCESSING / BIOMETRIC_UNAVAILABLE`; retry hanya setelah decrypt/hash
  pulih, reviewer/consent/epoch/TTL masih sah, dan attempt < 3.
- Rate limit upload persisten dan terpisah dari OTP; quota dicek sebelum decode.

## D. Berkas

**Baru — domain dan fixture**

- `backend/internal/enrollment/{model,service,views,photos,lifecycle,processor,reconcile}.go`
- `backend/internal/enrollment/service_test.go`
- `backend/internal/enrollmentfixture/migrations.go`
- `backend/internal/migrations/enr01b_test.go`
- `backend/synthetic-migrations/00007_synthetic_enrollment.sql`

**Baru — storage, crypto, JPEG**

- `backend/internal/enrollmentsecure/{crypto,keys,files,jpeg,platform_windows,platform_other,publish_linux,publish_unsupported}.go`
- `backend/internal/enrollmentsecure/{crypto,files,jpeg,publication,platform_windows,platform_other}_test.go`

**Baru — HTTP dan command sintetis**

- `backend/internal/httpapi/enrollment_handlers.go`
- `backend/internal/httpapi/enrollment_test.go`
- `backend/cmd/enrollment-synthetic/main.go`

**Diubah**

- `backend/internal/httpapi/{server,router}.go` — field injeksi eksplisit dan
  registrasi 12 route; `cmd/server` tetap nil.
- `backend/internal/migrations/schema_test.go` — regresi default schema 6 dan
  tidak adanya tabel biometrik dikembalikan/diperbarui.
- `backend/README.md`, `docs/API_CONTRACT.md`, `docs/DATA_MODEL.md`,
  `docs/SECURITY_PRIVACY.md`, `docs/TEST_PLAN.md`, `PROJECT_STATE.md`.
- `.gitignore` — pengecualian sempit `backend/internal/enrollment/` agar paket
  Go tidak ikut tertutup pola data `enrollment/`; direktori data tetap diabaikan.

Perubahan pre-existing pada `README.md`, `apps/mobile/...`, `docs/AUTHORIZATION.md`,
`docs/BACKLOG.md`, `docs/DECISIONS.md`, `docs/EMAIL_OTP.md`, dan kode CORE01/02
tidak diubah oleh laporan ini dan tetap dipertahankan.

## E. Schema — hanya disposable

`synthetic-migrations/00007_synthetic_enrollment.sql` berada **di luar**
discovery `migrations/`. Server biasa tetap schema 6 dan **tidak** memiliki
`face_templates`; ini diuji secara eksplisit.

Migrasi menambah:

- Kolom fence: `people.biometric_epoch`, `people.consent_revision`,
  `users.enrollment_auth_epoch`, `session_tokens.enrollment_epoch`.
- Kolom consent: `notice_version`, `notice_digest`, `subject_user_id`,
  `withdrawn_by`, `withdraw_reason_code`, `policy_version`; method diperluas
  dengan `digital_explicit` melalui rebuild kolom pada database disposable.
- Tabel: `face_enrollment_submissions`, `face_enrollment_photos`,
  `face_templates`, `enrollment_processing_jobs`, `biometric_deletion_jobs`,
  `enrollment_file_deletions`, `enrollment_rate_buckets`.
- Trigger immutability, binding consent/submission/template, queue index, dan
  fence untuk perubahan user/person/roster/class/session/consent, serta epoch
  monotonic.

`internal/enrollmentfixture.PrepareMigrations` menyalin `migrations/*.sql` dan
satu file sintetis ke direktori baru yang belum ada. Helper tidak membuka
database, tidak menjalankan migrasi, dan tidak membaca konfigurasi produksi.
Runner/checksum existing tetap dipakai. Migration 00001–00006 dan ledger lama
tidak ditulis ulang.

**Jangan menerapkan ke database Product Owner tanpa izin baru.** Migrasi ini
memakai rename/drop kolom pada database disposable dan bukan migration
production-ready.

## F. API

Semua route hanya ada jika `Server.Enrollment` diinjeksi.

| Metode/path | Peran | Hasil |
|---|---|---|
| `GET /api/v1/me/consent` | mahasiswa | notice + state + ETag |
| `POST /api/v1/me/consent` | mahasiswa | grant/replay |
| `POST /api/v1/me/consent/withdraw` | mahasiswa | withdrawal/receipt |
| `POST /api/v1/people/{person_id}/consent/withdraw` | admin | assisted withdrawal |
| `GET /api/v1/me/face-enrollment` | mahasiswa | status sendiri |
| `POST /api/v1/me/face-enrollment-submissions` | mahasiswa | submit/replay |
| `GET /api/v1/face-enrollment-submissions` | admin | queue, cursor, filter |
| `GET /api/v1/face-enrollment-submissions/{id}` | admin | detail/riwayat |
| `GET /api/v1/face-enrollment-submissions/{id}/photo` | admin | preview private |
| `POST .../{id}/approve` | admin | antrean pemrosesan |
| `POST .../{id}/reject` | admin | reject + purge |
| `POST .../{id}/retry-processing` | admin | retry terbatas |

Preview `no-store`, `nosniff`, panjang tetap, tanpa Range/URL publik, dan audit
ditulis sebelum byte dikirim. Error tidak memuat path, key, payload, atau stack.

## G. Bukti otorisasi dan ownership

| Skenario | Bukti |
|---|---|
| Admin mengelola queue/detail/preview/review | `TestEnrollmentHTTPGuardsPreviewApprovalAndWithdrawal`; smoke Admin 200/202 |
| Dosen ditolak | route admin + `requireRole(admin)`; test dan smoke preview/approve `403` |
| Mahasiswa tidak mereview/preview | test dan smoke `403` sebelum lookup resource |
| Mahasiswa tidak mengganti subjek | query `person_id`/`user_id` pada self ditolak `400`; subjek diambil dari sesi lalu dipetakan lewat account link |
| Cross-user | target Admin harus UUID resource; self route tidak menerima target; akses tanpa link `ACCOUNT_NOT_LINKED` |
| Sesi/role/status berubah | `authorize` membaca ulang `users`+`session_tokens`; test sesi dicabut/role diubah |
| Reviewer kehilangan peran | approval terkait `FAILED_PROCESSING / REVIEWER_INELIGIBLE`; tidak hidup lagi saat peran dipulihkan |
| Roster/account/link hilang lalu pulih | approval lama tetap `REVOKED`; template tidak dibuat |
| Uploader access-token kedaluwarsa | review durable tetap sah; bukan session revocation |
| Withdrawal saat processing | hasil worker terlambat difence; template tidak tersimpan |

Tidak ada keputusan role baru. Dosen tetap tidak memiliki kewenangan enrollment.

## H. JPEG, key, dan file

- Input ≤1 MiB; sisi 112–1920 px; total ≤2.073.600 pixel; decoder dibatasi dan
  output di-encode ulang tanpa metadata. EXIF berbahaya/pointer/out-of-bounds
  dan orientation selain normal ditolak.
- Envelope mengautentikasi purpose, binding, nonce, ciphertext, dan versi kunci;
  nonce 12 byte acak; key 32 byte; AAD template mengikuti tuple D-022.
- Key pending-photo dan template terpisah; provisioning eksplisit menolak
  keyset existing/parsial; tidak ada pembuatan/penggantian otomatis.
- Windows: DPAPI CurrentUser, file/temp dibuat dengan DACL final current-user
  secara atomik, tanpa owner/SACL atau SYSTEM.
- Windows/Linux: publikasi ciphertext atomic no-replace dan sinkronisasi
  direktori; platform lain menolak FileStore secara eksplisit.
- Root direktori diikat ke identitas handle, sehingga penggantian root setelah
  konstruksi ditolak.

## I. Retensi, withdrawal, dan rekonsiliasi

- `photo_expires_at = submitted_at + 72 jam` UTC; replay/preview/retry tidak
  memperpanjang. Expiry berlaku logis saat dibaca walau scheduler mati.
- Withdrawal menutup readiness, memfence worker, menghapus referensi/template
  secara transaksional, dan mencatat cleanup durable; tidak membutuhkan key.
- Reconcile menyelesaikan staging yang sudah commit, menandai worker yatim,
  menghapus ciphertext yang tidak direferensikan, dan mempertahankan baris yang
  tidak dapat didekripsi sampai keputusan manusia.
- Key-loss bukan alasan menghapus template. Retry foto retained hanya dalam
  batas TTL dan tiga attempt.
- Penghapusan lokal terkontrol bukan forensic erase dan tidak menjangkau backup
  eksternal, SSD/TRIM, paging, atau salinan lain.

## J. Pengujian dan hasil

Lingkungan: Go 1.27.0, `windows/amd64`, `CGO_ENABLED=0`.

| Perintah | Hasil |
|---|---|
| `gofmt` pada paket yang disentuh | bersih |
| `go test -count=1 ./...` | **16 paket uji lulus**, 4 paket tanpa tes |
| `go vet ./...` | exit 0 |
| `GOOS=linux GOARCH=amd64 go vet` (paket enrollment/storage) | exit 0 |
| `GOOS=linux go test -c ./internal/enrollmentsecure` | kompilasi tes Linux berhasil; **tidak dijalankan** |
| `git diff --check` | exit 0; hanya peringatan konversi LF→CRLF dari Git |
| `go run ./cmd/enrollment-synthetic` | lulus; schema 7, HTTPS nyata, nol template/ciphertext setelah withdrawal |

Smoke statuses: notice 200; grant 201; submit 201; replay 200; Admin detail 200;
Admin private preview 200; Dosen/Mahasiswa preview 403; approve 202; active 200;
key-loss 200 dengan `template_ready=false` dan template tetap 1; create/open/
manual session 201/200/201 tanpa key; withdraw 200; revoked 200.

Kegagalan nyata yang ditemukan dan diperbaiki:

1. Run pertama: build gagal karena import tak terpakai; fixture domain memakai
   multi-statement SQL dengan argumen yang salah terikat → FK failure. Fixture
   dipisah dan build diperbaiki.
2. Run kedua: constraint `ck_session_tokens_absolute` menolak refresh sintetis
   pada uji expiry; data uji diselaraskan.
3. Run crypto pertama: `TestFileStoreRejectsRepositoryAndRootReplacement` gagal
   karena identitas root Windows di-resolve lazy setelah replacement. Identitas
   kini ditangkap eager melalui handle; regresi yang sama lulus.
4. Review independen menemukan migrasi schema7 dapat ditemukan startup normal
   dan dua crash-recovery file; semuanya diperbaiki dan direview ulang.

Race detector **NOT_RUN** (`CGO_ENABLED=0`; tidak ada runtime Linux untuk
`renameat2`). Kompilasi lintas platform bukan bukti runtime.

## K. Temuan keamanan

| Temuan | Status |
|---|---|
| `.gitignore` pola `enrollment/` menutupi paket Go domain | **Diperbaiki** dengan negasi sempit; data enrollment tetap diabaikan |
| Migrasi sintetis sempat berada di discovery startup normal | **Diperbaiki**; dipindah ke `synthetic-migrations/` |
| Crash Windows sebelum DACL final meninggalkan orphan tak terpulihkan | **Diperbaiki**; CREATE_NEW + DACL atomik, regresi restart |
| Publikasi Linux link/unlink dapat meninggalkan hardlink internal | **Diperbaiki**; `renameat2(RENAME_NOREPLACE)` |
| Identitas root Windows lazy setelah replacement | **Diperbaiki**; handle-based identity |
| Preview yang sudah diotorisasi/in-flight tidak dapat ditarik | **Residual**; request berikutnya ditolak, byte yang sudah dikirim tidak dapat dipanggil kembali |
| Key-loss/decrypt gagal | **Fail-closed**; ciphertext dipertahankan, bukan dihapus atau ditebak |
| WAL/backup/media | Enkripsi melindungi ciphertext, tetapi tidak menjamin erase forensik atau backup eksternal |
| DPAPI same-user malware/roaming profile | Di luar jaminan; sudah dinyatakan di `SECURITY_PRIVACY.md` §5b |
| Notice `synthetic-v1` | Bukan consent legal manusia; hanya fixture |

Review independen (`CryptoSkeptic`, `HTTPSkeptic`, dan perluasan domain) bersifat
**statis**. Keduanya menutup temuan setelah perbaikan; tidak ada temuan
actionable tersisa yang terbukti. Ini bukan runtime atau production PASS.

## L. Keterbatasan

- Tidak ada data manusia, model YuNet/SFace nyata, kalibrasi, atau recognition.
- Tidak ada worker otomatis production; `ProcessNext`/`Reconcile` dipanggil
  eksplisit oleh host sintetis.
- Tidak ada uji power-loss/durability, Linux runtime, atau race detector.
- `photo_expires_at` hanya TTL foto; metadata/retensi akademik belum final.
- Preview in-flight dan salinan eksternal tidak dapat dijamin.
- `cmd/server` tetap tidak memiliki route enrollment; aktivasi production
  memerlukan tugas, konfigurasi, dan persetujuan baru.
- Migrasi `00007` hanya untuk disposable; jangan dijalankan pada database PO.

## M. Keputusan terbuka dan konflik

- P01–P12 disetujui **sebagai arah desain** pada FL-ENR01B, bukan approval
  production/data manusia; detail operasional yang belum divalidasi tetap terbuka.
- Konflik D-030 antara periode recovery dan larangan menjadikan key-loss pemicu
  penghapusan otomatis **tetap BLOCKED** untuk irreversible production deletion.
- `DECISIONS.md` FL-ENR01B menyebut reject/processing failure memicu purge segera,
  sedangkan proposal yang disetujui (D.5) dan D-030 melarang penghapusan otomatis
  saat kunci/dekripsi gagal. Implementasi memilih fail-closed: kegagalan inference
  biasa dipurge, tetapi kegagalan key/integritas mempertahankan ciphertext dan
  hanya boleh di-retry setelah pulih. **Perbedaan ini memerlukan keputusan pemilik
  sebelum production**, bukan diselesaikan diam-diam di kode.
- Isi notice/consent legal manusia, retensi metadata, dan kebijakan backup belum
  final.
- Model/ambang/quality gate dan integrasi UI enrollment belum diputuskan.
- Aktivasi route production dan provisioning key produksi belum disetujui.

## N. Kepatuhan batas

Tidak ada perubahan kode OTP/session, role existing, akun testing, mobile UI,
SMTP/DNS, video/backup, atau database Product Owner. Migrasi sintetis menambah
kolom fence pada `users`, `session_tokens`, `people`, dan `consent_records`, tetapi
hanya diterapkan pada database disposable oleh fixture; perilaku auth existing
tidak diubah. Tidak ada foto/biometrik manusia, API enrollment production,
migrasi biometrik production, atau penggantian SQLite ke Neon. Tidak ada
commit/push/merge/force push.

## O. GO/NO-GO menuju FL-ENR02

**GO terbatas:** hasil FL-ENR01B dapat dipakai sebagai baseline review desain
FL-ENR02 dan untuk verifikasi ulang pada lingkungan disposable.

**NO-GO:** implementasi FL-ENR02 otomatis, aktivasi production, pemrosesan wajah
manusia, migrasi database Product Owner, atau merge PR. Prasyarat yang belum
terpenuhi: persetujuan P01–P12, keputusan D-030, model/ambang yang tervalidasi,
consent/notice legal, dan validasi manusia yang terpisah.

**STOP.** FL-ENR02 tidak dimulai.

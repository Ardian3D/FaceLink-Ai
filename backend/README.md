# Backend FaceLink AI (Go)

## FL-ENR05 — one-run real synthetic integration

Actual Student/Admin TS clients, normalOTP/verifiedHTTPS, satu WAL schema8
disposable dan real YuNet/SFace IPC mencapai encrypted ACTIVEv2 lalu
withdraw/REVOKED/cleanup0. Jalankan opt-in client test dari `apps/mobile/`:
`FACELINK_ENR05_E2E=1` dan `NODE_EXTRA_CA_CERTS` ke development CA sebelum Node.
Harness test meluncurkan staging dengan `-integration-fixtures` dan semua
`-cv-*` flag; HTTPS wajib, worker paused/step/snapshot/reconcile hanya stdin.
`otp.wait` menunggu committed delivery exact challenge, bukan auth bypass.
Mode default/Admin juga memakai barrier stdin dan EOF shutdown milik runner.
Tidak ada migration baru atau perubahan `cmd/server`/database Product Owner.
Commands,23failure cases, batas restart/response-loss dan trace A–N:
[`FL_ENR05_TECHNICAL_REPORT.md`](../docs/FL_ENR05_TECHNICAL_REPORT.md).
**GO sintetis saja; STOP tanpa manusia/production/tahap baru.**

## FL-SEC01 — template record binding v2

Seluruh template baru pada harness sintetis/disposable memakai AES-256-GCM
envelope/AADv2 yang mengikat template/submission/person/immutable consent grant,
model/version/fingerprint dan key version. Decrypt memakai trusted record
metadata; legacy templatev1/unknown unsupported/quarantined tanpa fallback.
Pending-photo v1 tetap format terpisah. KeyProvider dan purpose keys tetap.
Migration00008 terpisah dari00007 dan default migrations; hanya disusun
`enrollmentfixture.PrepareMigrations` untuk DB disposable. Jangan jalankan
pada database Product Owner atau mengonversi/menghapus data v1 non-disposable.
AAD bukan anti-rollback seluruh database; D-030 tetap BLOCKED.
Kontrak: [`DATA_MODEL.md §4.12`](../docs/DATA_MODEL.md#412-face_templates--synthetic-only-fl-sec01).

## FL-ENR04 — processor model nyata, sintetis/disposable saja

`cmd/server` tetap tidak menginjeksi enrollment. Command staging dapat memilih
processor nyata dengan **semua** flag di bawah (PowerShell dari `backend/`):

```powershell
go run ./cmd/enrollment-staging-serve `
  -cv-python "$((Resolve-Path ../services/face-engine/.venv/Scripts/python.exe).Path)" `
  -cv-engine-dir "$((Resolve-Path ../services/face-engine).Path)" `
  -cv-model-dir "$((Resolve-Path ../services/face-engine/models).Path)"
```

Default tetap fixture deterministik existing. Ketika flag nyata dipilih,
kegagalan tidak beralih ke fixture/model lain. `-admin-fixtures` dilarang bersama
processor nyata karena seed/control itu memerlukan processor deterministik.
DB/key hanya disposable; jalur interpreter/source/model harus dipercaya operator.
Child memakai HMAC pipe privat, tanpa port Python; shutdown menunggu worker/child.
Smoke awal membuktikan NO_FACE; R2 membuktikan positive YuNet/SFace dengan
fixture prosedural existing setelah perbaikan native scale, bukan akurasi manusia.
Kontrak: API_CONTRACT §6.9; laporan: `docs/FL_ENR04_TECHNICAL_REPORT.md`.

> **FL-STAB01:** `POST /api/v1/auth/logout` kini membutuhkan JSON
> `{ "refresh_token": "..." }`, tanpa bearer/access dependency. Current atau
> tepat previous proof mencabut hanya sesinya; lookup/revoke/audit atomik,
> ulang204 tanpa audit ganda, invalid/absolute-expired live proof401.
> Masa berlaku/refresh-reuse policy tetap; tidak ada migration.
> Kontrak: `docs/API_CONTRACT.md` §5.1; bukti:
> [`FL_STAB01_TECHNICAL_REPORT.md`](../docs/FL_STAB01_TECHNICAL_REPORT.md).


Fondasi backend untuk demo **satu kelas, satu terminal** (FL-005). Pada tahap ini
belum ada pengenalan wajah: yang dibangun adalah kerangka layanan, basis data,
migrasi berversi, serta titik kesehatan yang dapat diuji.

## Status implementasi

| Bagian | Status |
| --- | --- |
| Konfigurasi berbasis lingkungan + validasi ketat | Ada |
| Log terstruktur JSON dengan request ID | Ada |
| SQLite (WAL, foreign key, busy timeout) | Ada |
| Runner migrasi berversi + checksum | Ada |
| Router chi, enveloppe galat konsisten, middleware | Ada |
| `/api/v1/healthz` (liveness) dan `/api/v1/readyz` (readiness) | Ada |
| Penghentian rapi (graceful shutdown) | Ada |
| `face_templates`, AES-256-GCM, KeyProvider | FL-ENR01B: implementasi sintetis/disposable dengan injeksi eksplisit; tidak aktif di server production |

Migrasi `00001_init_core_schema.sql` membuat 13 tabel domain **non-biometrik**.
Tabel `face_templates` dan `face_enrollment_samples` sengaja **tidak** dibuat.

### FL-CORE01 — people, roster, dan binding nonbiometrik

`GET/POST /api/v1/people`, `GET/PATCH/DELETE /people/{person_id}`,
`GET/POST /classes`, dan `GET/PUT/DELETE` membership kelas sudah diimplementasikan
sesuai kontrak. Mutasi khusus Admin aktif; Dosen hanya membaca kelas/peserta
yang ditugaskan; Mahasiswa memakai kepemilikan `/me/classes` dari backend.
NIM adalah `external_code`, unik sepanjang riwayat dan immutable.

Admin membuat person dari roster yang diperiksa, bukan dari kesamaan email.
`identity_reference` adalah referensi opaque roster nonbiometrik, maksimum
120 karakter; jangan memasukkan foto, salinan kartu identitas, atau NIK.
Referensi nonkosong merupakan attestation Admin dan menghasilkan timestamp
verifikasi server + audit atomik. Tanpa referensi, person belum terverifikasi
dan tidak dapat di-bind. Binding menuntut akun mahasiswa active dan person
aktif terverifikasi; perubahan pasangan/referensi menuntut unlink dahulu.
Ini bukan verifikasi otomatis registrar atau approval biometrik.

**Jangan jalankan contoh `-migrate` di bawah pada database existing Product Owner
untuk FL-CORE01.** Migration `00005_roster_identity.sql` hanya diuji pada database
disposable: tiga kolom nonbiometrik, indeks NIM unik global, trigger NIM
immutable. Baris lama tidak di-backfill sebagai terverifikasi. NIM lama duplikat
menggagalkan migrasi atomik; tidak ada reset ledger `schema_migrations`.
Penerapan ke database existing membutuhkan izin tambahan.

Bukti 25 Oktober 2026: `go test -count=1 ./...`, `go vet ./...`, serta uji upgrade
`go test -count=1 -run TestCORE01UpgradePreservesLedgerAndLegacyIdentity -v ./internal/migrations`
lulus. Smoke program sementara melalui socket HTTPS dengan CA uji tepercaya
membuktikan create person/class, binding, PUT membership `201 → 200`,
ownership `/me/classes`, mahasiswa ditolak `403`, NIM duplikat `409`, dan audit
tunggal di SQLite WAL versi 5; program dan database sementara dibersihkan.
Tidak ada perubahan OTP/session, akun testing, mobile, SMTP/DNS, atau biometrik.
Catatan di atas adalah snapshot FL-CORE01; sesi/manual kini tersedia pada FL-CORE02 di bawah.

### FL-CORE02 — lifecycle sesi dan attendance fallback

Face Recognition tetap mekanisme absensi **utama** yang direncanakan.
FL-CORE02 mengaktifkan hanya fallback manual terotorisasi; tidak ada engine,
camera, enrollment, template, atau jalur penulisan biometric success.

Route canonical yang tersedia: `GET/POST /sessions`, `GET /sessions/{id}`,
`POST /sessions/{id}/open`, `POST /sessions/{id}/close`,
`GET/POST /sessions/{id}/manual-checkins`, `GET /sessions/{id}/attendance`,
dan `GET /me/attendance`, seluruhnya berawalan `/api/v1`.
Admin/Dosen sesuai scope kelas; Mahasiswa hanya riwayat sendiri.
Sesi aktif dibaca dengan filter `class_id` dan `status=open`, tanpa endpoint baru.

Create menghasilkan draft; open/close transaksional, satu open per kelas/device.
Check-in biasa hanya saat open sebelum deadline; koreksi setelah close harus
memakai `checked_in_at` eksplisit dan alasan. Waktu disimpan/dikembalikan UTC;
tampilan klien menggunakan Asia/Jakarta (UTC+07), tanpa perubahan mobile pada
tugas ini. Batas `late_after` inklusif present; setelahnya late. Backend menolak
status present/late yang tidak sesuai waktu; excused adalah koreksi beralasan.
Consent biometrik bukan prasyarat manual; roster terverifikasi tetap wajib.
Alasan koreksi disimpan di record dan audit; jangan memasukkan rahasia atau
identitas sensitif ke alasan bebas.

`Idempotency-Key` opsional pada create/manual, scope per aktor dan hash metode/
path/body; replay mengembalikan snapshot pertama setelah reotorisasi.
Tanpa key, retry manual biasa `409 ALREADY_CHECKED_IN`; koreksi eksplisit
identik no-op `200`. Uniqueness `(session_id,person_id)` tetap penjaga akhir.
TTL replay 24 jam; snapshot expired dibersihkan pada mutasi CORE02 berhasil
berikutnya—tidak ada background janitor, jadi database idle dapat menahannya.

Migration forward-only `00006_class_sessions_manual.sql` memakai ALTER kolom
status tanpa DROP tabel induk/FK-off, mempertahankan data/legacy status/FK dan
ledger, menambah title/started_at/close_reason. **Hanya diuji disposable;
jangan terapkan ke database Product Owner tanpa izin tambahan.**
Readiness memisahkan `manual_attendance` dari `biometrics`; biometrik tetap
`unavailable / BIOMETRIC_NOT_IMPLEMENTED` dan tidak mematikan core/manual.
Deviasi existing readiness publik tetap terbuka; ini bukan izin deployment.

Bukti: `go test -count=1 ./...` (14 paket uji), `go vet ./...` exit 0, dan
smoke HTTPS schema 6 disposable lulus. Satu attendance/two manual mutation
audits, retry tanpa duplikasi, own history, mahasiswa ditolak, close dan
koreksi historis terbukti. Pemetaan 18 acceptance dan kegagalan run awal:
`docs/TEST_PLAN.md` §9. Race detector **NOT_RUN** (`CGO_ENABLED=0`).
CSV, audit-events API, me/schedule, biometrik dan UI attendance belum tersedia.

### FL-ENR01B — backend enrollment sintetis terisolasi

`internal/enrollment` memisahkan consent, submission, review Admin, pemrosesan,
aktivasi sintetis, expiry, dan withdrawal. Dua belas route canonical hanya
terdaftar jika `Server.Enrollment` diinjeksi; `cmd/server` tidak melakukannya.
Tidak ada flag production, model YuNet/SFace nyata, capture manusia, atau mobile UI.

`synthetic-migrations/00007_synthetic_enrollment.sql` adalah migrasi forward-only
terpisah dari discovery normal `migrations/` (tetap versi 6).
`internal/enrollmentfixture.PrepareMigrations` menyusun salinan migrasi gabungan
ke direktori disposable baru; hanya harness sintetis yang menerapkannya.
Migration 00001–00006 dan ledger existing tidak ditulis ulang.
Tidak ada migrasi database Product Owner pada tugas ini.

Untuk smoke lokal dari folder `backend` pada Windows:

```powershell
go run ./cmd/enrollment-synthetic
```

Command ini tidak menerima path/argumen atau membaca konfigurasi production.
Ia membuat SQLite WAL, kunci DPAPI CurrentUser, encrypted file store, akun/roster,
dan JPEG pola sintetis sementara; menjalankan router nyata melalui TLS; lalu
menutup server/database dan membersihkan direktori sementara. Processor contoh
hanya ada pada command pengujian dan bukan SFace atau bukti identitas.

Foto pending memakai TTL tepat 72 jam sejak diterima, bukan sejak preview/retry.
Normalisasi dibatasi 1 MiB dan 2.073.600 pixel. EXIF rusak/pointer sensitif atau
orientation selain normal ditolak; metadata output dihilangkan.
Admin memberi attestation kampus sintetis; approval hanya antrean,
bukan ACTIVE. Worker internal memvalidasi ulang consent/epoch/identitas/reviewer
dan hash artefak sebelum menyimpan template sintetis terenkripsi.

Foto dan template menggunakan purpose key berbeda serta authenticated envelope.
Provisioning kunci eksplisit; kunci hilang/rusak tidak diganti otomatis. FileStore
mendukung Windows/Linux dengan publikasi atomic no-replace; persistent DPAPI
key store hanya Windows. Platform lain menolak FileStore secara eksplisit.

Host sintetis memanggil `Reconcile` sebelum request dan secara berkala untuk
expiry, abandoned worker, staged/orphan ciphertext dan retry cleanup. Readiness
production biometrik tetap unavailable; core/manual tidak tergantung kunci.
Penghapusan logical dan cleanup lokal terkontrol bukan forensic erase SSD/WAL,
backup eksternal, atau jaminan zeroization seluruh runtime.

Approval desain P01–P12 tidak memberi izin deployment manusia. Konflik timer
recovery D-030 tetap BLOCKED untuk irreversible production deletion; TTL foto
72 jam tidak mengubah kebijakan pemulihan kunci. Laporan dan bukti akhir:
`docs/FL_ENR01B_TECHNICAL_REPORT.md`.


### FL-ENR03 — staging Admin sintetis dan fixture control

`cmd/enrollment-staging-serve` tetap merupakan command disposable, bukan
production. Dua flag opsional menambah harness Admin review:

```powershell
go run ./cmd/enrollment-staging-serve -admin-fixtures -web-root <dir-export-expo>
```

- `-admin-fixtures` menyemai lima submission sintetis (`pending-approve`,
  `pending-reject`, `pending-expiry`, `pending-conflict`, `failed-processing`),
  akun `reviewer2.synthetic@campus.example`/`lecturer.synthetic@campus.example`,
  dan worker yang mulai **paused**. Login tetap OTP normal; startup tidak
  mencetak token.
- `-web-root` menyajikan hasil `expo export` pada origin HTTPS yang sama dengan
  `/api/v1`. Path `/api` selalu diteruskan ke router nyata; fallback SPA tidak
  pernah menggantikan respons API dan segmen path dengan trailing dot/space
  ditolak.
- Kontrol hanya lewat stdin JSONL (tanpa route publik): `worker.pause`,
  `worker.resume`, `worker.step`, `clock.advance`, `processor.mode`,
  `account.role`, dan `account.status`. Ack tidak memuat token. EOF menghentikan
  proses disposable.
- `cmd/server` tidak menerima flag ini dan tetap tidak menginjeksi enrollment.
  Database Product Owner, auth/session/role, SMTP/DNS, dan template manusia tidak
  berubah. Laporan: `docs/FL_ENR03_TECHNICAL_REPORT.md`.

## Prasyarat

- **Go 1.27 atau lebih baru** (dikembangkan dan diuji pada Go 1.27.0).
- Tidak butuh CGO. Driver SQLite yang dipakai murni Go (`modernc.org/sqlite`),
  sehingga `gcc` **tidak** diperlukan untuk sekadar membangun dan menguji.
  (Uji `-race` tetap butuh `CGO_ENABLED=1` dan `gcc`; lihat bagian Pengujian.)

Tidak ada langganan, API berbayar, atau layanan cloud yang diperlukan.

## Menjalankan

```powershell
cd backend

# Terapkan migrasi ke basis data.
go run ./cmd/server -migrate

# Periksa konfigurasi tanpa menjalankan layanan.
go run ./cmd/server -check-config

# Jalankan layanan.
go run ./cmd/server
```

Nilai bawaan yang aman: mendengarkan di `127.0.0.1:8080` dengan basis data
`data/facelink.db`. Konfigurasi **menolak** alamat non-loopback dan menolak jalur
basis data di dalam folder sinkronisasi cloud (OneDrive, Dropbox, Google Drive,
iCloud) karena sinkronisasi dapat merusak berkas WAL.

Semua variabel lingkungan didokumentasikan di `.env.example` pada akar repositori.
Backend tidak memuat `.env` otomatis; ekspor variabelnya lebih dulu.

### Memeriksa dari terminal lain

```powershell
curl.exe http://127.0.0.1:8080/api/v1/healthz
curl.exe http://127.0.0.1:8080/api/v1/readyz
```

`/readyz` melaporkan `ai_engine: "unavailable"` bila `FACELINK_AI_ENGINE_URL`
kosong. Itu **bukan** kegagalan: absensi manual harus tetap berjalan, jadi mesin
AI yang belum siap tidak menggagalkan kesiapan backend.

### HTTPS lokal (opsional, untuk emulator)

Backend dapat menyajikan HTTPS tanpa meninggalkan loopback. Sertifikat dan
kuncinya harus diberikan bersamaan; hanya salah satu diisi akan ditolak.

```powershell
# Sekali saja. Membuat CA dev + sertifikat server dengan SAN
# 127.0.0.1, 10.0.2.2, dan localhost.
bash scripts/gen-dev-tls.sh

$env:FACELINK_TLS_CERT = "dev/certs/server.pem"
$env:FACELINK_TLS_KEY  = "dev/certs/server-key.pem"
go run ./cmd/server
```

Log akan mencetak `"tls":"on"` dan `"skema":"https"`. Alamat tetap
`127.0.0.1:8080`; TLS tidak memperluas jangkauan jaringan.

Berkas `*.pem` diabaikan Git. Kunci privat CA dan server hanya untuk mesin
pengembang: **jangan** disalin ke perangkat lain atau dipakai di build release.
Sertifikat dev berlaku 397 hari tanpa rotasi otomatis; bila handshake mulai
gagal, jalankan ulang skrip setelah menghapus `backend/dev/certs/`.

Aplikasi Android mempercayai CA itu **hanya pada build debug**, lewat
`apps/mobile/plugins/with-dev-tls.js`. Build release memakai trust store sistem.

### Email OTP (FL-OTP01/02)

Login utama MVP adalah Email OTP. Endpoint `POST /api/v1/auth/email/request` dan
`POST /api/v1/auth/email/verify` **hanya menerima HTTPS** (`r.TLS`) dan menolak
melayani bila konfigurasi tidak lengkap. Kontrak keamanan lengkap ada di
`docs/EMAIL_OTP.md`.

Pengirim FL-OTP02 adalah `support@facelink.online` melalui Gmail SMTP, dengan
alias yang sudah diverifikasi sebagai **Send mail as**. App Password dibuat
sendiri oleh pemilik melalui 2-Step Verification dan **tidak boleh** dikirim ke
chat atau ditulis di Git.

```powershell
# 1. Sekali: simpan App Password sebagai ciphertext DPAPI + buat OTP HMAC key.
#    Input App Password tersembunyi; tidak ada secret yang dicetak.
powershell -ExecutionPolicy Bypass -File scripts/setup-smtp.ps1 `
  -Username "<smtp-account-email>" `
  -From "support@facelink.online" `
  -ReplyTo "support@facelink.online"

# 2. Validasi konfigurasi tanpa membuka port. HTTPS dev wajib sudah ada.
powershell -ExecutionPolicy Bypass -File scripts/start-server.ps1 -Check

# 3. Jalankan backend dengan secret hanya di environment proses.
powershell -ExecutionPolicy Bypass -File scripts/start-server.ps1
```

Alternatif manual (secret di environment proses, bukan di berkas):

```powershell
$env:SMTP_HOST = "smtp.gmail.com"
$env:SMTP_PORT = "587"                    # STARTTLS wajib; 465 juga didukung
$env:SMTP_USERNAME = "<akun Gmail>"
$env:SMTP_AUTH_SECRET = "<App Password>"
$env:SMTP_FROM = "support@facelink.online"
$env:SMTP_REPLY_TO = "support@facelink.online"
$env:SMTP_ALLOWED_FROM_DOMAINS = "facelink.online"
$env:FACELINK_OTP_HMAC_KEY_FILE = "$env:LOCALAPPDATA\FaceLink\secrets\email-otp.key"
$env:FACELINK_TLS_CERT = "dev/certs/server.pem"
$env:FACELINK_TLS_KEY  = "dev/certs/server-key.pem"
go run ./cmd/server -check-config
```

`SMTP_ALLOWED_FROM_DOMAINS` adalah pernyataan eksplisit bahwa alias sudah
terverifikasi di Gmail; tanpa itu, `SMTP_FROM` wajib sama dengan
`SMTP_USERNAME`. Backend tidak menulis ulang From. Jika Gmail menolak atau
mengganti alamat, lihat header email nyata dan laporkan; jangan memalsukan
identitas. SMTP memverifikasi rantai sertifikat dan hostname TLS; tidak ada opsi
mematikan verifikasi. Cloudflare Email Routing hanya menerima dan meneruskan
email—bukan pengirim OTP.

Uji header: `scripts/send-smtp-test.ps1 -To admin@facelink.online`, lalu unduh
email sebagai `.eml` dan jalankan
`scripts/check-email-headers.ps1 -Path <file.eml>`. Script pemeriksa tidak
mencetak body/OTP.

ACL secret hanya menyentuh DACL (tanpa `Set-Acl`, owner, atau SACL), sehingga
tidak memerlukan Administrator/`SeSecurityPrivilege`. DACL minimum: pengguna
aktif FullControl + SYSTEM Read; Everyone/Administrators tidak ditambahkan dan
auditing tidak disentuh. Regresi non-elevated:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/test-secure-local.ps1
```

## Tata letak

```
backend/
├── cmd/server/          # titik masuk: flag -check-config dan -migrate
├── internal/config/     # pembacaan lingkungan, validasi, Redacted()
├── internal/database/   # pembukaan SQLite, WAL, foreign key, checkpoint
├── internal/httpapi/    # router, endpoint kesehatan, enveloppe galat
├── internal/logging/    # log JSON, larangan mencetak data biometrik
├── internal/migrations/ # runner migrasi berversi + checksum SHA-256
├── internal/emailotp/   # challenge OTP, HMAC, limit persisten, worker pengiriman
├── internal/otpkey/     # key HMAC dari DPAPI CurrentUser / berkas owner-only
├── internal/smtpemail/  # SMTP TLS tervalidasi (Gmail), tanpa logging payload
└── migrations/          # berkas SQL, pola 00001_nama.sql
```

## Pengujian

```powershell
cd backend

gofmt -l .            # harus tidak menghasilkan keluaran
go vet ./...
go build ./...
go test ./... -count=1
```

Uji `-race` memerlukan `CGO_ENABLED=1` **dan** `gcc` di `PATH`. Pada mesin tanpa
keduanya, perintah itu gagal pada tahap kompilasi `runtime/cgo`; itu keterbatasan
lingkungan, bukan kegagalan kode.

## Keamanan dan privasi

- Layanan hanya mendengarkan di loopback. Mesin AI lokal tidak boleh terbuka
  bebas ke jaringan (AGENTS.md §2).
- Log tidak boleh memuat JPEG mentah, embedding, kunci, token, atau detail
  biometrik. Pengujian kebocoran di `internal/httpapi/leak_test.go` menembak
  server HTTP nyata dan memeriksa badan respons, tajuk, serta log akses.
- Data biometrik tidak boleh masuk riwayat Git. `.gitignore` mengabaikan SQLite
  beserta sidecar `-wal`/`-shm`, karena sidecar dapat memuat sisa baris yang
  sudah dihapus.
- Absensi otomatis dari pengenalan wajah belum ada pada tahap ini.

Dokumen rujukan: `docs/API_CONTRACT.md` (kontrak endpoint),
`docs/DATA_MODEL.md` (skema dan transaksi), `docs/SECURITY_PRIVACY.md`
(aturan data), `docs/DECISIONS.md` (keputusan, termasuk D-030).

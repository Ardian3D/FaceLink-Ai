# FL-STAB01 — Expired-Token Logout Revocation & VideoPlayer Lifecycle Fix

Tanggal: 27 Oktober 2026. **F1 PASS; F2 PASS pada skenario yang dijalankan.** Ini bukan jaminan seluruh race native hilang dan bukan izin FL-ENR04/manusia/production. F3 tidak diubah.

## A. Post-merge Git baseline

- Git tidak tersedia pada PATH, lokasi instalasi umum, atau registry yang diperiksa. Dipakai MinGit portable resmi Git for Windows 2.56.0.4, diekstrak hanya ke direktori temporer. SHA-256 ZIP diverifikasi terhadap digest rilis resmi: `e3bf78351fe777fba47ddee5950082f8a98bb0f02604fdc7ba3ee2cb68ffa877`. Tidak ada instalasi sistem/perubahan PATH permanen.
- `fetch --no-tags origin` berhasil tanpa mengubah worktree.
- [PR #19](https://github.com/Ardian3D/FaceLink-Ai/pull/19) dikonfirmasi API GitHub `merged=true`, `merged_at=2026-10-26T20:33:58Z`, merge commit `95d31d8fa03d9bdaaee035da9a0bba6b06b4bc00` di `origin/main`.
- Branch lokal tetap `feat/fl-otp01-email-auth`, HEAD `87bac5e96602e9ff5568069d1c6b1bd1d96913b7`. Shared base `aa36f2d243f8761879baf38a5b2735afab8f1b52`; tree merge PR identik dengan shared base. Divergensi commit: lokal21/main1 (merge commit), bukan berarti fitur lokal hilang/tertimpa.
- Baseline worktree: **40 tracked modified, 177 untracked files**. Inventaris exact path dan 21 commit lokal disimpan di `research/fl-enr02a-host/evidence-local/stab01/baseline-and-revocation.json` (lokal, diabaikan Git).
- Belum masuk main: commit OTP/session verification, Email OTP backend/mobile/kontrak, SMTP alias/DPAPI/helper/invite/header tools; serta dirty CORE01/CORE02, enrollment sintetis ENR01B–ENR03/R1, CAM00–03/native capture adapters, staging generators, evidence dan dokumentasinya. Seluruh pekerjaan R5/ENR00/ENR00R/video dipertahankan.
- Dipilih bekerja in-place di branch lokal karena tree main sama dengan shared base. Tidak pull/reset/stash/clean/switch/commit/push/merge dan tidak mencoba merge ulang PR19.

## B. F1 reproduction dan root cause

Pra-fix parent menjalankan:

```text
cd backend
go test ./internal/httpapi -run '^TestLogoutExpiredAccessWithRefreshRevokesSession$' -count=1
FAIL: logout dengan access kedaluwarsa = 401, ingin 204
UNAUTHENTICATED
```

Fixture sesi sah + injectable clock mempertahankan refresh valid setelah access expiry. Endpoint lama berada di bearer middleware; handler tidak tercapai tanpa access sah. Mobile `keluarSesi(accessToken)` mengabaikan respons, lalu `AuthFlow.keluar()` menghapus credential lokal dan error. SecureStore bersih bukan bukti server revoked.

Sesi tetap opak/hash-only; akses60menit, absolut30hari. OTP issuance, role assignment, rotasi/replay policy dan masa berlaku tidak didesain ulang.

## C. F1 source changes

### Backend

- `backend/internal/session/store.go`: `Logout(refreshToken, requestID)` mencari hash current/tepat previous dalam `BEGIN IMMEDIATE`, mencabut hanya sesi itu dan menulis tepat satu audit `auth.logout` pada koneksi/transaksi sama. Audit gagal rollback. Waktu refresh dibaca sesudah memperoleh transaction lock.
- `backend/internal/httpapi/{router,auth_handlers,auth_middleware}.go`: existing `POST /api/v1/auth/logout` memakai JSON `{refresh_token}`, bukan bearer. Tidak menerima ID sesi/pengguna dari client; field tak dikenal ditolak. Access-only logout path/middleware/context helpers dan `RevokeOwned` obsolete dihapus; admin revoke tidak berubah.
- `backend/internal/session/store_test.go`, `backend/internal/httpapi/{authflow,email_handlers,rbac}_test.go`: konsumen dimigrasikan dan regresi perilaku ditambah.

### Mobile dan CLI

- `apps/mobile/src/features/auth/authApi.ts`: kirim refresh dalam body HTTPS, tanpa token di URL/bearer logout. Hanya204 mengonfirmasi revocation; network/TLS/non204 mengembalikan kegagalan.
- `apps/mobile/src/features/auth/authFlow.ts`: logout memutus ownership sesi, menginvalidasi generasi acceptance, menunggu login/refresh writes yang sudah berjalan, menghapus SecureStore akhir, dan mencabut dengan refresh proof. Respons terlambat tidak menghidupkan login kembali. Refresh persistence gagal mencabut dengan credential baru dan membersihkan credential disk lama. Remote revocation yang belum terkonfirmasi tidak berubah menjadi sukses hanya karena logout diulang atau sesi lokal kosong.
- `apps/mobile/src/features/auth/tokenStore.ts`: mutation generation menolak hasil load basi sesudah delete/save; cache baru dipublikasikan setelah native save berhasil.
- `apps/mobile/src/types/index.ts`, `apps/mobile/app/index.tsx`: `signOutIncomplete` membedakan local deletion dan remote revocation; pesan login jujur. Guard video selama restore/redirect tetap.
- `scripts/invite-admin.mjs`: konsumen logout CLI dimigrasikan ke refresh body; respons non204/network memberi warning dan exit nonzero. Tidak mengubah SMTP/DNS atau alur undangan/role.
- Tests: `apps/mobile/tests/{authApi,authFlow,tokenStore}.test.ts`, `tiruan.ts`; deferred Google/email acceptance, refresh/native write, stale SecureStore load, failed rotation persistence, initial-restore/logout dan repeated unconfirmed logout.
- `apps/mobile/tests/adminEnrollmentE2E.test.ts`: harness menunggu asynchronous OTP outbox berubah sebelum verifikasi; tidak mengubah production OTP.

Tidak ada endpoint baru, migration, perubahan skema, atau akses database Product Owner. Kontrak detail: `API_CONTRACT.md` §5.1.

## D. Server revocation evidence

Smoke parent memakai backend HTTPS nyata `cmd/enrollment-staging-serve -admin-fixtures`, CA lokal tervalidasi, schema7 disposable, dan login OTP normal (tanpa auth bypass).

| Skenario | Hasil diamati |
|---|---|
| Access A1 lewat TTL | `/me`401 |
| Logout A1 dengan refresh valid, tanpa bearer | 204 |
| Refresh credential A1 sesudah logout | 401 |
| Logout ulang A1 | 204 |
| Sesi kedua A | Refresh200, tidak ikut dicabut |
| Sesi milik B | Refresh200, tidak ikut dicabut |
| SQLite + audit | `revoked_reason=logout`, tepat1 audit untuk sesi A1 |
| Perangkat TECNO, access dimajukan3601detik | Logout UI kembali ke login, sesi perangkat benar revoked di SQLite |
| Jaringan perangkat diputus saat logout | Local signout; pesan remote belum terkonfirmasi; sesi disposable server memang masih live |
| CLI request/finally aktual | HTTPS revoke berhasil, refresh sesudahnya401; tidak membuat akun |

CLI smoke mengeksekusi source fungsi request dan finally block aktual dengan sesi disposable, bukan seluruh prompt/proses undangan. `node --check scripts/invite-admin.mjs` juga lulus.

## E. Refresh rotation/replay regression

Suite Go final membuktikan normal/expired/invalid refresh logout, current/previous/older proof, duplicate/idempotent logout, cross-user dan other-session isolation, concurrent duplicates, dua urutan refresh/logout, previous-token versus next-rotation race, replay setelah logout, batas absolut, database close/reopen, dan audit rollback.

- Refresh menang dulu: logout dengan R0 yang kini previous mencabut sesi yang sama.
- Logout menang dulu: refresh gagal; tidak ada token baru usable.
- Credential sudah previous lalu rotasi berikutnya menang: menjadi dua generasi lama, logout401, tidak mencabut perangkat lain.
- Logout tidak memanggil jalur deteksi refresh-reuse global. Kebijakan refresh-reuse existing tetap dan seluruh suite OTP/session lulus.
- Revoked current/previous proof idempoten204, tanpa mengubah alasan/waktu/audit; live absolute-expired proof401, tanpa memperpanjang sesi.

## F. F2 reproduction dan root cause

Temuan native awal tercatat di laporan ENR03-R1. Pada tugas ini, reproduksi terkontrol pra-fix menjalankan source `VideoBackground.tsx`, hook Expo terpasang, lifecycle releasing hook Expo, dan implementasi RN `parallel`:

```text
VideoPlayer.pause: Cannot use shared object that was already released
... istirahatkan -> animation completion
trace: ... unmount, 0:release, 1:release
```

`Animated.parallel` completion belum dibatalkan saat unmount. Expo melepas player, lalu callback memanggil `istirahatkan(keluar)` → `pause()`. Listener cleanup saja tidak membatalkan callback animasi.

Harness mengendalikan fase React, native-player boundary dan delivery timing. Ia bukan renderer Android; reproduksi native pra-fix berasal dari evidence ENR03-R1, bukan rerun APK lama pada tugas ini. Awalnya parser harness menolak Flow syntax RN; prasyarat diperbaiki memakai parser Hermes terpasang sebelum reproduksi yang valid.

## G. F2 fix dan lifecycle evidence

`apps/mobile/src/components/VideoBackground.tsx` saja untuk fix video:

1. Simpan identitas pemilik player tuple dan animasi aktif.
2. Cleanup menghapus ownership dan referensi animasi **sebelum** `stop()`; kemudian Expo tetap melakukan release otomatis.
3. Completion hanya bekerja bila pemilik dan identitas animasi masih sama; event posisi/status/end dan first-frame difence.
4. Dua video asli, texture surfaces, playback handoff, crossfade450ms, mute/loop/fallback dan session-restore guard tetap. Tidak LogBox suppression, catch-all, manual release atau cleanup disable.

Pascafix harness yang sama PASS untuk queued completion, synchronous-stop completion dan alternating playback dua arah. Trace tidak mengakses player setelah release.

TECNO LJ7 setelah izin perangkat baru: APK `ai.facelink.enr02host`, SHA256 `c8cb730c590022d2dc538df3ace7ef50d4b726d393614f11c6d75a48a4ea8134`, tanpa CAMERA. Empat signout dengan login OTP Admin/Dosen/Mahasiswa, cold restore ke Admin, enam route mount/unmount, background/resume, dan cold restart sesudah signout dijalankan. Video original terlihat berjalan; screenshot login hanya memperlihatkan video existing, bukan capture kamera. Tidak ada match released-player/prop-player/FATAL pada log PID yang diperiksa selama rangkaian terukur. Ini bukan bukti semua race native hilang.

## H. Main/staging regression

| Perintah/surface | Hasil final |
|---|---|
| Backend `go test -count=1 ./...` | 17 paket ok,4 tanpa tes |
| Backend `go vet ./...` | Exit0 |
| Main `npm run typecheck`, `npm run lint` | PASS |
| Main `npm test` | 219 PASS,6 skipped (14 files pass,2 opt-in skipped) |
| Stage typecheck/lint/test | PASS;219 PASS,6 skipped |
| `FACELINK_ADMIN_ENR_E2E=1 ... vitest run tests/adminEnrollmentE2E.test.ts` | 5/5 PASS, HTTPS client+Go nyata |
| `FACELINK_ENR_E2E=1 ... vitest run tests/enrollmentE2E.test.ts` | 1/1 PASS, HTTPS client+Go nyata |
| Stage Gradle ARM64 debug offline max-workers1 | BUILD SUCCESSFUL6m58s;557 tasks |
| Main Android build | NOT_RUN; perubahan JS, tidak ada dependency/native/manifest utama diubah; main APK dipertahankan |
| TECNO role navigation | Admin `Beranda sistem`, Dosen `Kelas saya`, Mahasiswa `Beranda pribadi`; Admin enrollment queue4 tampil |
| Video harness pra/pascafix | FAIL sebelum, PASS sesudah pada tiga mode |

Kegagalan run awal diungkap: E2E Admin sempat3/5 gagal karena membaca kode OTP fixture sebelumnya segera sesudah request202. Request acceptance bukan delivery completion; harness kini menunggu perubahan outbox dan final5/5 lulus. Native automation juga sempat salah membaca state sewaktu transisi, Back hanya menutup keyboard, atau overlay Google Autofill menutupi OTP. Input/verifikasi kemudian dijalankan pada fokus yang benar tanpa bypass. Tool hierarchy sementara sempat salah eja API Java; diperbaiki ke API SDK aktual. Tidak ada OTP/token mentah disimpan dalam evidence; tidak ada perubahan settings untuk mengatasi overlay.

Build memunculkan warning Gradle deprecated DSL/API existing, bukan build failure. Build debug memuat bundle lewat Metro; bukan bukti APK release/standalone.

## I. Security review

Independent backend/mobile security review dan video lifecycle review selesai. Backend tidak menyisakan temuan. Review mobile menemukan empat race cleanup (late login, stale load, failed rotation persistence, warning yang hilang) dan dua gap honesty yang lebih sempit; semuanya diperbaiki dengan regresi deferred boundary, closure review akhir **findings=[]**. Video review tidak menemukan actionable finding. Reviewer tidak mengklaim menjalankan native/tests; hasil eksekusi di atas berasal dari parent.

Credential hanya pada body HTTPS; tidak menerima arbitrary session IDs, tidak menonaktifkan authorization, tidak memperpanjang masa berlaku atau mencabut semua perangkat lewat logout. Atomic audit failure disengaja fail-closed. Role/OTP/session issuance/replay policy tetap; database/akun asli/SMTP/DNS tidak disentuh.

## J. Remaining residuals dan cleanup

- **F3 P3 tetap terbuka**, tidak diubah.
- Native evidence terbatas pada perangkat dan skenario ini; iOS, release APK, seluruh kemungkinan jadwal callback/native teardown, dan TalkBack/font-scale khusus tidak diuji. Tidak ada klaim zeroization.
- Offline logout tidak dapat membuktikan remote revocation; aplikasi menghapus credential lokal dan menampilkan warning. Tidak menambah retry otomatis atau menyimpan token untuk retry. Unconfirmed status ditahan dalam instance AuthFlow, bukan persistent revocation queue.
- Logout yang mendahului initial SecureStore read dinyatakan remote-unconfirmed secara konservatif, bukan mengarang proof sesi.
- Kegagalan hapus SecureStore dilaporkan; tidak ada klaim disk bersih ketika native delete gagal.
- Tidak ada payload kamera baru, enrollment production, model/biometrik manusia, migrasi PO, atau perubahan P01–P12.
- Cleanup terukur: staging force-stopped; reverse milik tugas kosong; port8089/8090/8091 tidak menerima koneksi; Metro/relay/backend dihentikan dan root disposable dihapus. Tool UI jar/XML temporer di HP dihapus. APK staging final/evidence dan APK utama dipertahankan. Hash APK utama `f4cc2eae08fc95681960e9fbf33808e382125961d6a4304c7b1d2cc60967bf8b` tetap;10 settings sama dengan baseline, tidak ada settings put.
- Evidence lokal: `research/fl-enr02a-host/evidence-local/stab01/` (baseline inventory, revocation, acceptance, cleanup, screenshot UI sintetis/video existing). Tidak dipush.

## K. GO/PARTIAL/NO-GO menuju FL-ENR04

**GO bersyarat untuk peninjauan/perencanaan FL-ENR04 oleh Product Owner:** F1 dan jalur F2 yang direproduksi sudah ditutup dan acceptance terbatas lulus. Native assurance tetap terbatas; F3 tidak ditutup. Tidak ada keputusan otomatis bahwa biometrik/production siap.

**NO-GO otomatis** untuk FL-ENR04 execution, capture manusia, enrollment production, database Product Owner, ESP-CAM, atau deployment. Perlu task/otorisasi baru dari pemilik. Tidak ada commit/push/merge pada FL-STAB01. **STOP setelah laporan.**

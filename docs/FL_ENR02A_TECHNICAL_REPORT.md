# FL-ENR02A — KYC-Style Student Enrollment UI & Synthetic Integration

Tanggal: 26 Oktober 2026. Scope: staging sintetis saja.

**Hasil: implementasi dan verifikasi offline selesai; acceptance UI Android end-to-end masih PARTIAL.** Tidak ada akses ADB/install atau pengujian layar perangkat pada tugas ini. Tidak ada capture manusia, aktivasi enrollment production, migrasi database Product Owner, commit/push/merge, atau pekerjaan FL-ENR02B.

## A. Repository baseline

- Branch `feat/fl-otp01-email-auth`, HEAD `87bac5e96602e9ff5568069d1c6b1bd1d96913b7`.
- Baseline lanjutan: 34 berkas modified, 46 entri untracked, staged 0. Seluruh pekerjaan R5, CORE01/02, ENR00/00R, ENR01A/B dan CAM03 dipertahankan. Tidak reset/stash/clean.
- Canonical: README, PROJECT_STATE, PRD, DECISIONS, ARCHITECTURE, SECURITY_PRIVACY; kontrak enrollment ENR01B dan laporan CAM03. Approval P01–P12 adalah arah desain dan implementasi sintetis, bukan izin manusia/production.

## B. Source files changed

Baru untuk FL-ENR02A:

- `apps/mobile/src/features/enrollment/{EnrollmentScreen.tsx,api.ts,fixture.ts,flow.ts,model.ts,staging.ts,syntheticCapture.ts}`.
- `apps/mobile/tests/{enrollmentApi.test.ts,enrollmentFlow.test.ts,enrollmentE2E.test.ts,enr02aStaging.test.ts}`.
- `research/fl-enr02a-host/{prepare-enr02a.cjs,templates/enrollment.tsx,README.md,.gitignore}`.
- `backend/cmd/enrollment-staging-serve/main.go`.
- Laporan ini.

Dimodifikasi untuk integrasi:

- `apps/mobile/app/(app)/mahasiswa.tsx`: tautan staging di balik flag default-off; klaim consent hardcoded diganti belum tersedia.
- `apps/mobile/src/features/dashboard/mockData.ts`: menghapus klaim consent aktif dari fixture.
- `apps/mobile/src/features/auth/SessionProvider.tsx`: akses token melalui AuthFlow existing, tanpa menaruh token di React state atau mengganti login.
- `apps/mobile/src/features/api/http.ts`: body biner/header/ETag; timeout, pembatalan dan pemeriksaan konfigurasi existing tetap dipakai.
- `.gitignore`: pengecualian sempit source enrollment mobile dan template source staging; data biometrik tetap diabaikan.
- `PROJECT_STATE.md`, `README.md`, `apps/mobile/README.md`, `docs/{BACKLOG,SECURITY_PRIVACY,TEST_PLAN}.md`.

Perubahan lain yang telah ada di worktree bukan bagian baru tugas ini. `authSecurity.test.ts` telah berubah pada CAM03: penghapusan tes source-string incidental, bukan pengurangan pemeriksaan otorisasi runtime oleh ENR02A.

## C. Staging isolation evidence

- Staging diturunkan dari aplikasi FaceLink asli, bukan aplikasi mock terpisah: `research/fl-enr02a-host/stage`.
- Android application ID dan iOS bundle identifier staging: `ai.facelink.enr02host`; scheme `facelinkenr02`.
- Route enrollment hanya dibuat di staging. Main dashboard default-off jika `EXPO_PUBLIC_FACELINK_ENR_STAGING` tidak tepat `1`.
- Server staging hanya loopback, database/direktori baru disposable, kunci ephemeral terpisah per purpose, akun/roster sintetis. Tidak membaca konfigurasi atau database production.
- `cmd/server` tidak menginjeksi layanan enrollment; discovery migrasi produk tetap schema6. Tidak ada migrasi baru pada tugas UI ini.
- SecureStore implementation/key tidak diganti. Package Android berbeda mengisolasi sandbox; isolasi data perangkat belum diuji runtime pada ENR02A.

## D. Student UI implementation

Layar memakai komponen/theme FaceLink existing: intro, informasi notice dan explicit acceptance, persiapan, oval portrait/guidance, progress sintetis, cancel, pengiriman, pending review dan deadline, processing, rejected/expired/failed, withdrawn, serta ACTIVE terverifikasi.

Harness `SyntheticCapture` menggunakan clock yang dapat diinjeksi; tidak meminta permission atau membuka kamera. Capture selesai otomatis mengirim satu fixture JPEG pola sintetis; tidak ada shutter manual sebagai alur utama atau preview gambar. Upload memakai indikator indeterminate, bukan persentase palsu.

## E. Synthetic backend integration

Client memanggil endpoint ENR01B aktual: GET/POST consent, withdrawal, submit JPEG, dan status. Tidak ada endpoint produk tambahan untuk shortcut UI. Admin approval di E2E memakai sesi Admin fixture yang sah; bukan fungsi Admin di UI mahasiswa.

Command staging mendukung login Email OTP existing melalui TLS dan file inbox OTP sementara, tanpa SMTP. Mode `-insecure-http -test-tokens` hanya untuk harness E2E loopback; token fixture ditangkap oleh test runner, bukan login bypass aplikasi. Startup normal tidak mencetak OTP/token; test mode secara eksplisit mengeluarkan access token fixture dan tidak boleh dipakai untuk manual/device.

E2E menjalankan server Go nyata dan client TypeScript nyata: NOT_GIVEN → grant ACTIVE consent → submit PENDING_APPROVAL → Admin approve202 → worker ACTIVE/template_ready → withdraw → REVOKED/template_ready=false. Ini bukti client/backend, bukan interaksi layar atau TLS Android.

## F. Consent and status correctness

- Consent terpisah dari enrollment; login atau pilihan lokal tidak memberikan consent.
- Grant memakai notice version, ETag/If-Match, UUID idempotency; renewal membawa record ID consent terdahulu.
- Terminal submission tetap dipertahankan setelah renewal; server `SUBMIT` mengizinkan kembali ke preparation/scan, tanpa grant berulang yang konflik.
- “Wajah terdaftar” memerlukan backend ACTIVE, template_ready, consent ACTIVE pada kedua snapshot, tidak stale, dan umur snapshot maksimal30detik.
- Failed reload atau withdrawal yang sedang/ambigu tidak mempertahankan trust ACTIVE. Withdrawal dengan respons hilang direkonsiliasi lewat pembacaan backend.
- Snapshot/ETag dimuat ulang setelah grant gagal ambigu atau precondition berubah. Tidak mengklaim withdrawal berhasil hanya berdasarkan klik.

## G. Navigation and role boundaries

Main tidak memiliki route enrollment. Route staging berada di grup authenticated existing; screen mengalihkan non-mahasiswa ke home role existing. Backend tetap penentu role/ownership. DTO self tidak membawa person/user selector.

Back/cancel menghentikan scan lokal; blur/background membatalkan operasi, termasuk saat menunggu token. Foreground tanpa consent snapshot memuat ulang kedua snapshot. Upload tidak dibatalkan oleh refresh biasa. Request yang telah commit tidak dapat dibatalkan retroaktif.

## H. Privacy findings

Tidak ada camera permission, human imagery, image preview, gallery/file capture, AsyncStorage atau analytics pada fitur enrollment. JPEG fixture hanya di memori dan dikirim ke endpoint submission yang dipilih secara eksplisit; referensi buffer dikosongkan setelah request. Ini bukan jaminan seluruh salinan runtime telah terhapus secara forensik.

APK debug aktual diaudit melalui `aapt dump badging`: package `ai.facelink.enr02host`; CAMERA/storage-media/SYSTEM_ALERT_WINDOW tidak ada. Permission host yang tetap ada: INTERNET, VIBRATE, CHANGE_WIFI_MULTICAST_STATE, ACCESS_NETWORK_STATE, ACCESS_LOCAL_NETWORK, WAKE_LOCK, USE_BIOMETRIC, USE_FINGERPRINT, serta internal receiver. USE_BIOMETRIC/FINGERPRINT berasal dari host SecureStore; bukan capture wajah fitur ini.

Debug masih mempercayai CA development dan cleartext untuk Metro sesuai konfigurasi debug host; bukan konfigurasi release/production. Tidak mengklaim zero-telemetry, forensic storage proof atau runtime TLS perangkat.

## I. Tests/build results

| Pemeriksaan yang dijalankan | Hasil |
|---|---|
| Main `npm run typecheck && npm run lint && npm test` | Lulus;139 tes,1 E2E opt-in skipped |
| Staging perintah yang sama | Lulus;139 tes,1 skipped |
| Staging `FACELINK_ENR_E2E=1 npx vitest run tests/enrollmentE2E.test.ts` |1/1 lulus terhadap server Go nyata |
| `npx expo export --platform android --output-dir artifacts/export-final` | Lulus;1491 modules, Hermes bundle |
| Gradle `app:assembleDebug -PreactNativeArchitectures=arm64-v8a --offline --console=plain` | BUILD SUCCESSFUL,4m8s |
| Backend `go test -count=1 ./...` |16 paket uji lulus;5 paket tanpa tes |
| Backend `go vet ./...` | Lulus |
| Backend `go run ./cmd/enrollment-synthetic` | HTTPS nyata, schema7; key-loss fail-closed; manual tanpa kunci201; withdrawal menyisakan0 template/0 ciphertext |

APK: `research/fl-enr02a-host/stage/android/app/build/outputs/apk/debug/app-debug.apk`,100190664 byte; SHA256 `62751219c14fb1f104f3ceb2ba5170eb55c16e5727ab7c9054632174a67bea89`. Debug Development Build memuat JS melalui Metro; source final juga dibuktikan lewat export Android terpisah. APK tidak dipasang.

Tes regresi mencakup renewal menuju scan, status terminal, trust expiry, load gagal, withdrawal respons hilang, cancel saat token belum selesai, token null saat upload, resume setelah initial load terputus, single submission, API ETag/binary/error dan synthetic-only parsing. Tes navigation role existing ikut berjalan. Ini pengujian state/kontrak UI, bukan mounted visual/component interaction atau device E2E.

Kegagalan sebelumnya: withdrawal current envelope salah diparse sebagai ConsentView (diperbaiki); lint ref render (diperbaiki); build awal gagal karena keystore staging belum dibuat (keystore lokal khusus staging dibuat); typecheck lanjutan menemukan enum `timeout` yang tidak ada (dihapus; transport memetakan galat melalui enum existing). Run akhir di tabel lulus. Warning toolchain Gradle/Kotlin/deprecation tetap ada.

Smoke cleanup menemukan proses server fixture yatim karena `go run` wrapper di-kill pada Windows. Enam proses test-only yang teridentifikasi dihentikan. Runner kini mengirim EOF stdin dan menunggu exit; server test-only berhenti pada EOF. Readiness failure tetap memegang ownership child; galat fatal server dikembalikan melalui `run()` agar defer cleanup berjalan, bukan `os.Exit` di tengah setup. Smoke startup dengan sertifikat hilang menghasilkan galat yang diharapkan dan **0 direktori disposable tertinggal**; sesudah E2E, pemeriksaan proses menunjukkan **0 server fixture**. Review lifecycle lanjutan menutup temuan startup-failure dan defer tersebut.

## J. Independent security review

Review source independen menemukan dan menutup: replacement consent ID, scanner setelah renewal, error page tersembunyi, resume tanpa consent, cancellation saat token acquisition, stale ETag recovery, source/templates terabaikan Git, debug overlay, bundle ID iOS staging, trust ACTIVE setelah failed reload dan withdrawal ambigu. Perbaikan trust direview ulang: tidak ada tambahan actionable finding pada bagian yang diubah.

Review keamanan akhir source-to-sink dan generated config tidak menemukan actionable finding dalam scope sintetis. Tidak menjalankan tests/builds sendiri; hasil runtime tabel I berasal dari integrator, bukan reviewer. Review bukan sertifikasi production.

## K. Remaining blockers

- Tampilan dan interaksi UI Android, accessibility aktual, login OTP staging di HP, navigation/back/background saat request aktual, serta cross-app SecureStore runtime: **NOT_RUN**. Belum ada izin perangkat baru untuk ENR02A. Tidak mengklaim UI visual atau device E2E PASS.
- UI state dan client/server E2E terbukti terpisah; belum satu pembuktian end-to-end dari tap layar sampai backend.
- Tidak ada manusia, model YuNet/SFace nyata, PAD/liveness, akurasi, power-loss, atau production deployment.
- Konflik retensi/purge/key-loss D-030 pada ENR01B tetap terbuka; tugas UI tidak mengubah kebijakan itu. Notice legal manusia, resource benchmark dan retensi metadata belum final.

## L. GO/NO-GO menuju FL-ENR02B

**GO terbatas:** review hasil offline dan validasi UI Android sintetis pada staging terpisah, setelah izin ADB/install yang baru. Pertahankan aplikasi utama/data, tanpa camera permission atau capture manusia.

**NO-GO:** klaim acceptance ENR02A Android end-to-end lengkap, pelaksanaan FL-ENR02B otomatis, aktivasi enrollment production, capture/upload manusia, migrasi database Product Owner, merge PR19 atau commit/push/merge.

Keputusan yang dibutuhkan: izin perangkat untuk menyelesaikan acceptance UI Android sintetis; review Product Owner atas hasil ini. **STOP setelah laporan; FL-ENR02B tidak dimulai.**

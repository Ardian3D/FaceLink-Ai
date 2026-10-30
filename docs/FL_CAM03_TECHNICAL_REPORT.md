# FL-CAM03 — Expo Host Integration: Native KYC Capture Adapter

Tanggal: 26 Oktober 2026. Mode: scoped integration dan synthetic verification.

**Hasil: PASSED untuk host staging release Android dan consumer sintetis; PARTIAL untuk kesiapan integrasi berikutnya. NO-GO enrollment production atau capture wajah manusia. STOP setelah laporan ini.**

## A. Baseline/worktree — PASSED

- Branch baseline `feat/fl-otp01-email-auth`, HEAD `87bac5e96602e9ff5568069d1c6b1bd1d96913b7`.
- Baseline 26 tracked unstaged dan 27 untracked; tidak reset/stash/clean/commit/push/merge. Pekerjaan CORE01/CORE02, R5, ENR00/ENR00R dan seluruh proof terdahulu dipertahankan.
- Referensi: README, PROJECT_STATE, PRD, DECISIONS, ARCHITECTURE, SECURITY_PRIVACY, CAM00–CAM02, ENR01A technical report dan capture adapter contract.
- `research/fl-cam03-host/baseline.json` menyimpan checksum berkas utama terlindungi. Audit akhir membuktikan 134 input sumber tetap cocok dan salinan staging sesuai kontrak transformasi. Hash 21 berkas utama terlindungi juga diperiksa tanpa perubahan.
- Satu perubahan test-only di main: menghapus tes source-string incidental dan import tak terpakai dari `apps/mobile/tests/authSecurity.test.ts`. Tidak mengubah kode runtime/UI/auth utama. Perubahan video BACA-SAYA sudah ada sebelumnya.

## B. Expo host compatibility — PASSED terbatas / PARTIAL

Host benar-benar diturunkan dari `apps/mobile`, bukan App prototype pengganti. Stack: Expo58.0.13, RN0.88.0, React19.3.0, Nitro0.37.1; tambahan dipin VisionCamera5.2.3, detector2.1.0, Nitro Image0.15.2. Konfigurasi routing, video, SecureStore dan HTTPS/auth asli disalin.

- `npm ls --all --json`: tidak ada problems/error setelah koreksi instalasi file package.
- Autolink Android mendeteksi `FaceLinkCaptureHostPackage`; core tidak diduplikasi ke detector.
- `file:` package dipasang dengan `install-links=true`; pemetaan TS menunjuk declaration `react-native/types_generated/index.d.ts`, bukan runtime JS.
- Semua 886 baseline lock entries existing tetap identik; dependency baru terpisah dan dipin.
- Release host native berhasil compile dan berjalan. Runtime **debug Expo Development Build belum dijalankan**; Expo Go/iOS tidak divalidasi. Tidak mengklaim dukungan production host hanya berdasarkan metadata versi.

## C. Native packaging changes — PASSED

`packages/native-capture-host/` menyediakan Android library, autolink dan TS bridge metadata-only. Gradle mengambil lima core Kotlin ENR01A dari repository sebagai single source; `Cam02FrameStore.kt` historis tidak termasuk compile task. Kotlin Android mengabaikan include source-set pada konfigurasi awal; task-level exclusion memperbaiki kompilasi tanpa memasukkan dependency CameraX yang tidak dibutuhkan.

Staging: `research/fl-cam03-host/stage/`, application ID `ai.facelink.cam03host`, scheme `facelinkcam03`, label `FaceLink CAM03 Host`. Namespace Java asli tetap `ai.facelink.mobile`, sesuai komponen fully-qualified. Package ID dan UID berbeda dari aplikasi utama.

`prepare-host.cjs` tidak menjalankan prebuild atau membuang artefak. Main key, env privat dan data sesi tidak disalin; key lokal staging dibuat terpisah. Manifest menolak CAMERA, storage/media dan overlay. Release mempertahankan HTTPS policy host.

## D. Synthetic consumer — PASSED

Bridge `NativeModules.FaceLinkCaptureHost` mengekspos:

| API | Kontrak |
| --- | --- |
| `runSyntheticVerification()` | Laporan 13 kasus sintetis; JPEG/handle/hash konten tidak dikirim ke JS |
| `cancel()` | Pembatalan logis; ACK setelah physical drain, atau error drain-timeout yang eksplisit |
| `dispose()` | Terminal untuk instance; reservasi global tidak dilepas selama native masih berjalan |
| `getStatus()` | Phase, busy, foreground, safe error, batas resource; counter unknown tetap null |
| `getStorageInventory()` | Metadata sandbox berbatas; tidak membaca payload preference/database/sesi |

Consumer native menerima ByteBuffer read-only scoped, memvalidasi JPEG/dimensi dan kesesuaian receipt dengan frame/generation. Dua fixture YUV berbeda dibandingkan secara native tanpa mengekspos digest. Tidak ada kamera, file capture, gallery, upload, atau callback image/base64/URI ke JS.

## E. Ownership/frame evidence — PASSED

TECNO LJ7 Android16/API36 ARM64, page4096; izin perangkat baru diberikan sebelum ADB. Installed APK hash cocok artefak.

Suite aktual **13/13 PASSED**, kemudian diulang PASSED tanpa busy/reservation leak:

1. consume-once dan penolakan double-consume;
2. stale generation dan concurrent capture;
3. concurrent consume satu pemenang;
4. cancel pada first write codec nyata;
5. background native-boundary injection pada first write;
6. output exception pada first write;
7. queued input timeout sebelum codec;
8. ready TTL expiry;
9. consumer exception cleanup;
10. consume timeout dengan cooperative physical release;
11. owned-buffer budget;
12. real codec output cap failure;
13. dua konten/generation dengan input/frame/receipt terikat.

Kasus13: JPEG706 dan718 byte, 16×16, generation1/2, frame `host-synthetic-a`/`host-synthetic-b`, native content berbeda, kedua input ditutup sekali. Peak own buffers4194688 byte. Semua kasus akhir0 owned frames/0 owned bytes. Status akhir IDLE, busy=false, suiteDrained=true. Counter suite legacy yang tidak diketahui tidak dipalsukan sebagai zero.

Bukti: `research/fl-cam03-host/evidence/device-native-suite.json`, `storage-after.json`.

## F. Native/JS boundary security — PASSED terbatas

- Hanya scalar metadata/report/status/inventory; tidak ada JPEG/base64/pixel/embedding/handle pada bridge host.
- Single process-wide reservation mencegah overlap lintas ReactContext saat cancellation/dispose masih drain.
- Encoder/consumer di worker native; tidak menunggu codec pada UI thread. Buffer tetap dimiliki sampai callback native kembali; timeout tidak membebaskan memori yang masih dibaca.
- Host watchdog setelah task return mengeluarkan `E_CAPTURE_HOST_DRAIN_TIMEOUT` setelah8s, menyelesaikan promise dengan error tetapi menahan reservasi; polling1s berlanjut sampai physical drain untuk reclaim.
- **Batas:** synchronous native call yang tidak pernah kembali dari dalam `perform()` tidak dapat dipaksa berhenti oleh watchdog coordinator ini. Kontrak callback harus kembali; bukan hard-real-time interrupt/total zeroization.
- Storage traversal berbatas, tanpa mengikuti symlink atau membaca isi credential/preferences/database/video. Nama relatif/ukuran/mtime tetap metadata sensitif lokal, bukan telemetry yang boleh dikirim bebas.

## G. Android build results — PASSED release / NOT-RUN debug runtime

Final release: `assembleRelease -PreactNativeArchitectures=arm64-v8a --offline --console=plain`, BUILD SUCCESSFUL7m59s, 1197 actionable tasks (84 executed). lintVital dan R8 tetap aktif.

APK lokal: `research/fl-cam03-host/artifacts/cam03-host-arm64-release.apk` (diabaikan Git), 63965254 byte, SHA256:

`518acad24c21cbf3a006f96dfa205f6f5bc268eeb9d6ce365bf82798c0417e9f`

- Package `ai.facelink.cam03host`, minSDK24, target36, compile37.
- Embedded JS2846340 byte;33 native ARM64 libraries; zipalign dan native ZIP16KB alignment lulus. Ini bukan bukti runtime16KB; HP page4096.
- Nondebuggable. Signature v2 verified; sertifikat self-signed staging lokal, bukan production key.
- Permission: INTERNET, VIBRATE, ACCESS_NETWORK_STATE, WAKE_LOCK, USE_BIOMETRIC, USE_FINGERPRINT, internal receiver. Biometric permission berasal dari stack host/SecureStore; bukan implementasi enrollment. CAMERA/storage/media/overlay tidak ada.
- `allowBackup=true` mengikuti host beserta aturan backup SecureStore; bukan klaim semua data aplikasi dikecualikan backup. Tidak ada cleartext opt-in release. Debug host mempertahankan debug-only CA/Metro policy; debug APK/runtime final tidak divalidasi.

Kegagalan antara yang diungkap: cache Maven offline kurang Media3 muxer1.9.1 dan lifecycle-common-jvm2.9.0; dependency resmi di-resolve sekali, final build kembali offline. CMake/Ninja Windows gagal path-length pada screens/Nitro/app gesture codegen. Junction/subst tidak menyelesaikan canonical path. Build dilakukan dari salinan staging disposable `<short-staging-workspace>`, core tetap menunjuk repository melalui `FACELINK_CAPTURE_CORE_REPOSITORY`; app CMake buildStagingDirectory hanya pada salinan build diubah ke `<short-cmake-workspace>`. Tidak mengubah registry/settings atau dependency source upstream. Gradle deprecation/JVM metaspace warning tetap terlihat.

Reproduksi: jalankan `node research/fl-cam03-host/prepare-host.cjs`; `npm ci --offline --ignore-scripts --no-audit --no-fund` di stage; siapkan key debug khusus staging; jalankan audit/typecheck/test. Pada Windows path panjang, salin staging ke workspace pendek tanpa cache `.cxx`/`.gradle`/build lama, pertahankan source core canonical via environment variable, dan gunakan staging CMake path pendek sebelum assembleRelease. Jangan menimpa APK historis atau memakai key main.

Audit rinci: `research/fl-cam03-host/release-audit.json`.

## H. FaceLink regression — PASSED terbatas

| Pemeriksaan | Hasil aktual |
| --- | --- |
| Main `npm test` | 8 files,105 tes lulus |
| Main `npm run typecheck` | Lulus |
| Stage `npm test` final | 10 files,120 tes lulus |
| Stage `npm run typecheck` | Lulus |
| Core offline Kotlin | 224 assertion adapter +35 pixel-boundary lulus |
| Login release cold-start | COLD1024ms; proses berjalan tanpa Metro |
| Google | Button accessibility enabled=false |
| Email navigation | Input alamat dan CTA tersedia; tidak mengetik credential/meminta kode |
| Kembali eksplisit | Kembali ke login teramati |
| SecureStore | Availability=true; tidak mengambil payload sesi/token |
| Session/HTTPS | signedOut, config valid, protocol https: |
| Video asli | Kedua player ready/played/progressed; pair overlap teramati, tanpa errorObserved |

Hardware BACK pada pengamatan pertama tidak membuktikan kembali ke login; explicit `Kembali` diuji dan berhasil. Tidak mengubah behavior navigasi existing. Role-based routing dibuktikan regresi existing, **bukan** login role nyata pada HP.

Playback observer staging menyisipkan observer hanya untuk import `expo-video` dari VideoBackground asli; hook/player SDK asli dan aset asli tetap digunakan. Metadata bukan visual-proof: tidak menyatakan video rendered appearance PASS. Tidak mengambil screenshot/video frames login.

## I. Privacy/lifecycle — PASSED bounded / PARTIAL menyeluruh

- Metadata inventory sebelum/sesudah kedua run: tidak ada path baru atau perubahan bytes/mtime file yang tercatat.23 entry termasuk directory; file font, ART profile/cache, ExoPlayer/RKStorage dan preferences host sudah ada. Tidak membaca payloadnya.
- Warning `E_INVENTORY_ROOT_UNAVAILABLE_no_backup`; inventory tidak truncated, signatureBytesRead=0. Ini snapshot terbatas, bukan observer transient-write/whole-filesystem forensic proof.
- INTERNET tetap ada karena host nyata; UID traffic/packet capture/SDK destinations tidak diukur pada CAM03. Tidak mengklaim zero-telemetry atau memindahkan bukti UID0 dari prototype lama ke host baru.
- HOME lalu resume diagnostic menampilkan `Cancelled; native drain acknowledged.` Ini setelah suite selesai, bukan OS HOME tepat saat codec aktif. Codec background-boundary injection dibedakan eksplisit.
- Force-stop dan cold-start kedua berhasil (993ms); tidak otomatis menjalankan suite/kamera. Crash buffer akhir tidak memuat package staging.
- Enam setting awal/akhir identik: brightness3, mode0, timeout1800000, stay-awake15, accelerometer_rotation1, user_rotation0. Tidak menebak/memulihkan baseline CAM00 yang berbeda.
- Main installed APK SHA256 tetap `f4cc2eae08fc95681960e9fbf33808e382125961d6a4304c7b1d2cc60967bf8b`,100168685 byte; main tidak diluncurkan. Tidak menghapus app atau SecureStore existing.
- Akhir: stage force-stopped, XML UI sementara dihapus, tidak ada screenshot tersimpan, tidak ada kamera/OTP/account baru. APK stage tetap terpasang dan artefak historis dipertahankan.

Bukti perangkat: `research/fl-cam03-host/evidence/` (`device-baseline`, `device-install`, `device-login`, `device-playback-securestore`, `device-native-suite`, `storage-before`, `storage-after`, `email-navigation`, `lifecycle`, `device-end`).

## J. Independent review — PASSED source terbatas

Dua reviewer read-only memeriksa native concurrency/ownership dan staging/auth/video/audit secara terpisah. Temuan actionable diperbaiki:

1. Promise/reservasi dapat tertahan jika physical drain tak selesai: explicit error settlement, reservasi aman tetap ditahan, monitor terus sampai drain. Reviewer mengonfirmasi tidak menemukan double-settlement path baru; batas synchronous hang tetap dicatat.
2. Playback agregat dapat mencampur mount lama: agregat kini hanya newest pair dengan pair-specific overlap; regression overlapping mount.
3. Audit URL/instrumentation/generated list fail-open: URL parsing menolak credential/query/fragment; exact GENERATED_PATHS set/hash plus template/Metro suffix verification.
4. Copied Android staging belum dibandingkan hash: sekarang semua `android/**` non-generated dibandingkan source approved. Audit final lulus134 input.

Review bukan sertifikasi keamanan dan reviewer tidak menjalankan perangkat; runtime di atas dilakukan agent integrasi terpisah. Tidak membuat tes source-string/mock forwarding sebagai bukti fitur.

## K. Remaining blockers dan berkas

Berkas source baru: `packages/native-capture-host/` (package/autolink/Gradle/index.ts, HostVerification, HostIdentityVerification, HostStorageInventory, HostMetadata, module/package Android); `research/fl-cam03-host/` (prepare/audit, templates diagnostic/playback observer/regression, baseline, release-audit dan metadata evidence). Dokumen: laporan ini dan PROJECT_STATE. Test-only main: `apps/mobile/tests/authSecurity.test.ts`. Stage/artifacts lokal diabaikan Git.

Sisa batas: debug Development Build runtime, visual video appearance, native hang yang tidak kembali, transient cache/SDK traffic menyeluruh, bisnis consumer/main camera ingress, privacy/product approval dan P01–P12. Tidak ada endpoint enrollment, migration biometrik atau aktivasi workflow production.

## L. GO/NO-GO dan rekomendasi

**GO terbatas:** review komponen berikutnya untuk host Android/staging dan consumer native sintetis berdasarkan kontrak ownership ini.

**PARTIAL:** kesiapan host Expo Development Build debug dan privacy/telemetry end-to-end; release standalone berhasil bukan bukti semua mode build.

**NO-GO:** mengaktifkan Face Enrollment production, capture wajah manusia, template biometrik, approval P01–P12 otomatis, atau deployment/release produksi. Tidak ada perubahan OTP/session/role/database/SMTP/DNS/ESP-CAM, commit/push/merge PR19.

Keputusan dibutuhkan: Product Owner meninjau gate dan memberi task terpisah bila ingin debug Development Build verification atau integrasi consumer bisnis. Jangan otomatis membuat proof tambahan atau memulai enrollment. **STOP FL-CAM03.**

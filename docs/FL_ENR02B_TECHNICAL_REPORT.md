# FL-ENR02B — Native KYC Camera Integration (Offline, Synthetic Staging)

Tanggal: 26 Oktober 2026. **STATUS: PARTIAL; live camera execution NOT_RUN.**

Implementasi offline, synthetic-only dan source-complete tersedia. Build native staging diverifikasi setelah perbaikan artefak. **Tidak ada ADB, install, kamera, mannequin, wajah manusia, upload JPEG live, database Product Owner, atau production enrollment** pada tugas ini.

## A. Baseline/worktree

- Branch `feat/fl-otp01-email-auth`, HEAD `87bac5e96602e9ff5568069d1c6b1bd1d96913b7`.
- Baseline R2: staged0, modified38, untracked49. Tidak reset/stash/clean/commit/push/merge; APK/video/evidence dipertahankan.
- Laporan FL-CAM03, FL-ENR01A, FL-ENR02A, R1, R2 dan kontrak adapter dibaca. Fix video R2 tetap ada (`apps/mobile/app/index.tsx`).
- Main app tidak menerima route enrollment, kamera, dependency VisionCamera, atau permission CAMERA. `cmd/server` tetap tanpa injeksi enrollment.

## B. Native module integration

`packages/native-capture-host` sekarang memiliki dua bridge:

- `FaceLinkCaptureHost` — verifikasi sintetis FL-CAM03, tidak berubah.
- `FaceLinkCaptureLive` — port CAM00 metadata-only. API: `start`, `decide`, `consume`, `cancel`, `dispose`, `getStatus`, `addCaptureFrameListener`.
- Mode live memakai `Cam02FrameStore`, `NativeCaptureAdapter`, `AndroidCaptureEncoder`, `HybridFaceSpec`, CameraX/ML Kit dari detector 2.1.0 yang dipatch oleh `research/fl-enr02a-host/apply-detector-live.cjs`.
- Hanya satu pemilik class core: detektor yang dipatch saat live; host memakai synthetic core saat live tidak aktif. Build.gradle memilih source set secara conditional dan menolak live jika project detector tidak ada.

Bridge live mempertahankan generation/frame identity, single in-flight, opaque consume-once handle, foreground/FLAG_SECURE/permission checks, cancel/background/dispose, dan batas resource adapter. JS hanya menerima scalar metadata dan handle opaque; tidak ada JPEG/base64/pixel/path/pointer.

## C. UI integration

- `SYNTHETIC_FLOW` mempertahankan jalur lama: fixture memori, auto-submit, backend disposable.
- `MANNEQUIN_PREVIEW_ONLY` dibuat source-complete di `research/fl-enr02a-host/templates/nativeScanner.tsx`; staging prepare menyalinnya ke `src/features/enrollment/nativeScanner.tsx`.
- Main memakai stub `nativeScanner.tsx` tanpa import VisionCamera; mode mannequin tidak dapat aktif di luar staging.
- Mode dipilih build-time dengan `EXPO_PUBLIC_FACELINK_CAPTURE_MODE`; staging default `synthetic_flow`. `captureMode.ts` memaksa synthetic bila staging flag mati/unknown.
- UI mempertahankan consent → prepare → explicit start → preview; label mode ditampilkan. Kamera tidak start saat mount/resume.
- Scanner live: front camera, portrait guard, oval, detector guidance, dwell, single capture, consume metadata, cancel/retry; tidak menampilkan/mengunggah gambar. Parent tidak memanggil submission pada mode live.
- Terminology membedakan capture native, submission, review, dan ACTIVE backend. Capture native tidak menghasilkan status “Wajah terdaftar”.

## D. Permission dan consent lifecycle

- Urutan source: consent valid → penjelasan permission → tindakan eksplisit → preview.
- `NativeScanner` tidak memasang `Camera` sebelum tombol start dan permission resolution; resume tidak auto-start; background/unmount memanggil native cancel.
- Staging `app.json`/manifest/debug hanya menambahkan CAMERA pada staging package. Main manifest tetap tanpa CAMERA; media/storage/overlay tetap diblokir.
- Permission ditolak/revoked, account/role invalid, dan consent withdrawal tetap fail-closed; tidak ada camera resume otomatis.
- Camera permission pada APK staging adalah izin runtime normal, bukan bypass production. Build main tidak mendapatkannya.

## E. Native buffer/privacy contract

- `consume` menjalankan consumer native sinkron; buffer read-only scoped dan tidak pernah menjadi byte/base64/path di JS.
- Handle opaque hanya capability consume-once; bukan akses langsung ke byte. Nilai handle tidak dicetak/di-serialize.
- Tidak ada gallery/file writer/upload/image preview pada path scanner live.
- Log bridge/scanner tidak memuat payload gambar atau handle.
- Detector patch menambahkan `Cam02FrameStore` dari single source CAM00; tidak membuat camera stack baru.
- **Batas:** native consumer/CameraX/ML Kit internal allocations di luar cap adapter; bukan RSS/zeroization/forensic proof. Live frame tidak dibuktikan sampai device acceptance.

## F. Synthetic flow regression

- 147 tes mobile/staging lulus, 1 opt-in E2E skipped; typecheck/lint lulus.
- Main/staging mode test: staging false selalu synthetic; mannequin hanya explicit staging.
- Existing OTP/session/video/dashboard/auth tests tidak berubah.
- Backend `go test -count=1 ./...` 16 paket lulus/5 tanpa tes; `go vet ./...` lulus.
- Jakarta export Android 1622 modules lulus. Native test CAM00: 35 pixel-boundary +224 adapter assertions +21 guidance +10 adapter JS lulus.
- Live camera behavior, OTP on device, and real backend E2E for mode B **NOT_RUN**.

## G. Accessibility

- Scanner staging: 48dp start/cancel/retry/back, live-region guidance, progressbar label/value, explicit error copy, portrait warning, no image preview.
- Mode label and permission explanation; no ACTIVE stale claim from capture event.
- Main stub accessible.
- Residual: TalkBack/mannequin full traversal and large-font scanner layout **NOT_RUN** until device permission. Primary-button white-on-red contrast ≈3.90:1 remains a global brand-token residual; R2 exact issue not redesigned.

## H. Android build

| Pemeriksaan | Hasil |
|---|---|
| Stage TS + lint + tests | 147 passed, 1 skipped |
| Stage Android export | Lulus; 1622 modules |
| Stage live Gradle debug ARM64 | **BUILD SUCCESSFUL 5m3s**, 656 task (89 executed) |
| Final APK audit | mannequin: package `ai.facelink.enr02host`; 128950880 byte; SHA256 `0a19d170c99bbf11dee87debcd4d2e5dffdfaeff89bc071c2ef0618af40e2815`; CAMERA + INTERNET + host permissions; no storage/media/overlay. Synthetic: 102262250 byte; SHA256 `9a8c7b902815b160c35b1c21eef52e7b017aaf4004bc20383ff60264f8df42db`; **no CAMERA**, no storage/media/overlay |
| Detector patch | Applied/idempotent; CAM02 marker dan source set ada |
| Synthetic Gradle debug ARM64 | **BUILD SUCCESSFUL 3m34s**; APK synthetic copied to evidence |
| Main build | Tidak dijalankan; source/permission main tidak berubah |

Catatan build nyata: percobaan offline pertama gagal karena `lifecycle-livedata-2.10.0` tidak ada di cache; dependency di-resolve sekali, lalu build dijalankan ulang. Percobaan berikut menemukan duplicate `AdapterEncoder` akibat path project detector dan artefak build host lama; path build.gradle diperbaiki, build host/detector dibersihkan, versi paket host dinaikkan, dan build final berhasil. Tidak ada perubahan dependency upstream.

## I. Security findings

Independent security review menemukan tiga blocker high dan beberapa residual; semuanya diperbaiki dan direview ulang:

- **FLAG_SECURE:** staging MainActivity mannequin sekarang memasang `FLAG_SECURE` sebelum `super.onCreate` dan `setRecentsScreenshotEnabled(false)` pada API33+. Native tetap menolak start tanpa flag.
- **Install contract:** prepare menulis `.npmrc` `install-links=true`; README memakai `npm install` lalu `apply-detector-live.cjs` sebelum build. Dependency kamera hanya ada pada mode mannequin.
- **Detector patch:** urutan didokumentasikan; prepare gagal jika detector terpasang tetapi belum ber-marker `FL-ENR02B` saat mode mannequin.
- **Mode gating:** CAMERA, dependency kamera, live template, dan FLAG_SECURE hanya ditambahkan pada mannequin. Synthetic menghapus CAMERA dan mempertahankan stub scanner.
- **Availability:** `isCaptureLiveAvailable()` diekspor; bridge async; scanner memeriksa native module sebelum meminta permission.
- **Status contract:** `getStatus` memakai generation nullable; cancel membersihkan geometry.
- **Generation/consume state:** generation JS di-seed dari waktu; token consumed direset pada start/fail/stop.
- **Copy:** preview kamera dijelaskan sebagai tampilan lokal; hasil capture tidak disimpan/diunggah/ditampilkan sebagai gambar.

Review read-only mengonfirmasi temuan tertutup. Static checks: tidak ada import VisionCamera pada stub main, tidak ada upload/file/base64 pada scanner, dan hanya opaque consume token yang menyeberang. Build final dan APK audit mendukung packaging mannequin terpisah. Tidak ada klaim dynamic zero-leak/device proof.

## J. Device tests / NOT_RUN

Semua **NOT_RUN**: ADB/install, kamera fisik, mannequin, live guidance, CAPTURE_READY/event, live consume, permission prompt/revocation, background during guidance/encoding, slow init, duplicate/stale frame pada perangkat, dan TalkBack/font scanner. Build debug yang dibuat belum diinstal.

## K. Remaining blockers

1. Device gate baru untuk staging APK kamera dan pengujian mannequin; tanpa izin baru kamera tetap tidak boleh dinyalakan.
2. Live mode belum dijalankan; build native sukses bukan bukti preview/guidance/ownership runtime.
3. Detector dependency patch harus diterapkan setelah `npm install` pada staging; `package-lock` stage dan autolinking harus konsisten.
4. Final APK/permission/hash harus diaudit setelah build final.
5. TalkBack/font scale scanner, mannequin lighting/pose, dan resource cleanup perangkat.

## L. GO/PARTIAL/NO-GO menuju live mannequin acceptance

**PARTIAL:** source integration, staging build, native compile, and offline regressions dapat dilanjutkan. **GO bersyarat** untuk meminta izin perangkat baru guna live mannequin acceptance dengan satu mannequin, tanpa upload/save/JS image.

**NO-GO:** capture wajah manusia, live JPEG upload ke backend, enrollment production, database Product Owner, ESP-CAM, SMTP/DNS, commit/push/merge, atau mengaktifkan mode mannequin tanpa izin baru. D-030 tetap BLOCKED untuk production irreversible deletion. **STOP tanpa kamera/ADB.**

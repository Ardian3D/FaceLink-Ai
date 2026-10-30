# FL-ENR02B-R1 — Native KYC Live Mannequin Android Acceptance

Tanggal: 26 Oktober 2026. **STATUS: PARTIAL. NO-GO menuju Admin Enrollment UI integration berdasarkan acceptance ini.**

Preview live, permission, deteksi negatif/ukuran wajah, dan lifecycle diuji pada TECNO LJ7. **Same-frame auto-capture dan consume-once LIVE belum terbukti**: rekaman UI menunjukkan NO_FACE atau TOO_FAR, native ARMED, dan tidak ada trace CAPTURE/READY/CONSUME. Build dan unit test tidak menggantikan bukti tersebut. Tidak ada live submission atau enrollment manusia/production.

## A. Baseline dan package verification

- Branch `feat/fl-otp01-email-auth`, HEAD `87bac5e96602e9ff5568069d1c6b1bd1d96913b7`; baseline offline staged0, modified38, untracked51. Seluruh pekerjaan existing dipertahankan; tidak reset/stash/clean/commit/push/merge.
- APK existing mannequin: `research/fl-enr02a-host/evidence-local/r2/enr02b-mannequin-debug.apk`, package `ai.facelink.enr02host`, SHA256 `0a19d170c99bbf11dee87debcd4d2e5dffdfaeff89bc071c2ef0618af40e2815`, 128950880 byte. AAPT offline membuktikan CAMERA, tanpa storage/media/overlay.
- Package berbeda dari main `ai.facelink.mobile`. Package mannequin sama dengan staging sintetis: konsekuensi update ini dijelaskan sebelum izin. APK sintetis dan mannequin lama dipertahankan; tidak uninstall atau clear app data.
- APK perbaikan native yang dipasang: SHA256 `0ed8952eaa6468263033441aa8946add523bb88e2b19aca354cc9dbb7e751fd9`, package tetap `ai.facelink.enr02host`, CAMERA tanpa storage/media/overlay. Host package versi `0.2.3`.
- Debug Development Build memakai JS Metro dari stage mannequin; bukan release standalone. Metro localhost8089 memakai relay IPv4 localhost8090; reverse staging8089 menuju8090. HTTPS backend disposable localhost8091 memakai development CA existing. Tidak mematikan TLS validation atau memakai test-token login di perangkat.
- Main APK SHA256 awal/akhir identik: `f4cc2eae08fc95681960e9fbf33808e382125961d6a4304c7b1d2cc60967bf8b`.

## B. Device permission evidence

Sebelum ADB pada tugas ini, package/hash/manifest diverifikasi offline dan konsekuensi penggantian package staging dijelaskan. Pertanyaan izin baru menyebut TECNO LJ7, instalasi mannequin staging, kamera depan dengan satu mannequin, auto-capture/cleanup, tanpa manusia/upload/perubahan setting HP. Pemilik memilih **“Ya, izinkan pengujian mannequin”**. Izin task R2 tidak dipakai ulang.

Pemilik juga memilih **“Mannequin siap, bidang kamera aman”** sebelum kamera dan mengonfirmasi ulang fixture aman setelah jeda. Tidak ada izin tambahan Settings revocation, TalkBack, atau font scale; ketiganya tidak dilakukan. Android CAMERA diberikan melalui dialog normal **“Hanya kali ini”** setelah uji penolakan.

Bukti lokal ignored: `research/fl-enr02a-host/evidence-local/enr02b-r1/`. Kode OTP fixture tidak disimpan sebagai evidence atau laporan; login melalui UI normal dibantu pemilik, tanpa mengambil SecureStore.

## C. Startup/runtime

- Instal existing mannequin `adb install -r` berhasil; hash installed cocok. Cold-start mencapai login, lalu OTP fixture normal mencapai dashboard Mahasiswa Sintetis.
- **Defect D1:** reload/deep-link berikutnya gagal dengan `E_CAPTURE_UNAVAILABLE` pada `Cam02FrameStore.attach` saat pembuatan `FaceLinkCaptureLiveModule`.
- Source menunjukkan `onHostDestroy` hanya dispose store, sementara sink baru dilepas pada `invalidate`. Sink lama dapat tetap terdaftar saat React membuat modul pengganti. Fix minimal: `onHostDestroy` memakai identity-checked `detach(this)` dan menghentikan observer; late invalidate tidak men-dispose pemilik baru melalui jalur destroy. Single-owner attach guard tidak dilemahkan.
- Setelah rebuild/reinstall: cold-start, hot reload pemicu, login/session restore, dan restart berikutnya mencapai dashboard tanpa attach error yang sama. Tidak ada klaim seluruh variasi React-context interleaving sudah teruji.
- Setelah jeda, reverse ADB hilang; launcher melaporkan `ConnectException` ke127.0.0.1:8089. Mapping task dipulihkan dan startup pulih. Ini dependency debug/Metro, bukan missing native module.
- Sampling log PID staging pada sesi live: nol FATAL EXCEPTION dan nol warning `already released`/`Cannot set prop 'player'`. Bukan bukti semua log sepanjang task atau seluruh race expo-video.

## D. Camera preview

- Scanner sebelum tindakan eksplisit: **IDLE · OFF**, teks “Kamera mati”, belum meminta CAMERA. Consent dan layar penjelasan hadir lebih dulu.
- Setelah start eksplisit dan permission: Android mencatat camera client device1 untuk package staging; pemilik mengonfirmasi **“Preview normal, mannequin terlihat”**. Tidak ada foto/screenshot preview yang dikumpulkan; FLAG_SECURE tetap aktif.
- Verifikasi visual preview berasal dari observasi pemilik. Portrait/preview normal teramati; ketepatan numerik mirror, crop, dan empat sudut oval **belum terbukti** oleh observasi ini.
- Capture hasil tidak ditampilkan sebagai gambar. Crop screenshot diagnostik kontrol login yang dibuat sebelum kamera hanya berisi area hitam akibat FLAG_SECURE; tidak dipakai sebagai bukti preview.

## E. Guidance

- Runtime **NO_FACE, 0 wajah, 0/1500ms, 0 observasi** teramati; tidak terjadi auto-capture.
- Pemilik mengonfirmasi satu wajah/progress muncul. Snapshot berikutnya membuktikan **TOO_FAR, 1 wajah, 0/1500ms, 0 observasi**. Callback native berjalan: counter contoh99→217 frame dan925→992 frame; generation stabil, tanpa error.
- Setelah penyesuaian jarak, pemilik memilih “Bingkai selesai diproses”. Snapshot/trace saat itu tetap TOO_FAR/ARMED dengan trace capture kosong. Kedua sumber dicatat terpisah; konfirmasi tersebut **tidak dianggap bukti capture PASS**.
- Ulang setelah jeda/konfirmasi fixture: empat observasi NO_FACE, counter615→632, tanpa CAPTURE/READY/CONSUME.
- Tidak menurunkan threshold, mengubah dwell/sample requirement, menyuntik approval, atau mengganti fixture dengan manusia.
- Posisi valid di dalam oval, dwell1500ms/sampel terpenuhi, out-of-oval versus inside-oval positif, dan dua fixture nonmanusia **NOT_RUN sampai hasil yang dapat dibuktikan**. Penyebab tidak mencapai readiness belum dibedakan tuntas antara fixture/posisi/pencahayaan dan mapping geometri.

## F. Same-frame auto-capture

**LIVE NOT_PROVEN / blocked pada readiness.** Tidak ada event sah yang membuktikan CAPTURE_READY → CAPTURE → CONSUME pada perangkat. Tidak ada frame ID/generation dari capture sukses untuk dibandingkan dan tidak ada bukti single-trigger positif.

Instrumentasi sementara hanya pada scanner generated staging mencatat whitelist frame ID/generation/status/counter; tidak merekam gambar, bounds/pose/landmark, atau handle. Instrumentasi tidak memaksa capture. Seluruh instrumentasi dibuang dari source aktif melalui prepare canonical setelah run.

## G. Native buffer consume-once

- Snapshot LIVE yang teramati: ARMED, `ownedBytes=0`, `heldFrames=0`, `busy=false`, `lastError=null` pada fase guidance. Ini bukan bukti release JPEG setelah consume karena encoding tidak tercapai.
- Probe diagnostik sementara menyiapkan stale-generation consume, consume sah, lalu duplicate consume; probe berada setelah READY dan **tidak dieksekusi**, karena READY tidak tercapai. Tidak ada opaque token dicetak atau dimasukkan evidence.
- **Native unit PASS:**35 pixel-boundary dan224 adapter assertions; JS guidance21 dan facade10 lulus. Consumer concurrency, stale/cancel, replay dan ownership memakai fixture terkendali, bukan kamera live.
- Consume-once, duplicate consume ditolak, stale-generation ditolak, cancellation setelah READY, dan physical cleanup setelah live consume **NOT_RUN**.

## H. Privacy/storage

- Inventory staging memakai `run-as ai.facelink.enr02host du -ak .`, metadata nama/ukuran saja; tidak membuka SecureStore, main app data, JPEG, atau file biometrik.
- Sebelum/sesudah run tidak ditemukan nama path dengan ekstensi jpg/jpeg/png/webp/heic/nv21/yuv. Batas: snapshot/nama file tidak mendeteksi seluruh write sementara, file tanpa ekstensi, GPU/codec memory, paging atau telemetry. Tidak ada klaim zero-telemetry/total zeroization/no-temporary-write.
- Backend disposable perangkat setelah run: `face_enrollments=0`, `face_enrollment_submissions=0`, `face_enrollment_photos=0`, `face_templates=0`. Audit memuat consent.granted1 dan login normal; tidak ada enrollment.submitted. Tidak ada live upload, accidental submission, ACTIVE, atau template production.
- Tidak menjalankan migrasi database Product Owner. Data fixture dan migration schema7 hanya pada root disposable.

## I. Lifecycle dan cleanup

| Skenario | Bukti perangkat |
|---|---|
| Permission denied | Dialog “Jangan izinkan”; kembali ke persiapan, kamera tidak berlanjut |
| Grant normal | Dialog “Hanya kali ini”; kembali ke persiapan, perlu start eksplisit baru |
| Cancel preview | Tombol cancel kembali ke persiapan; camera client staging kosong |
| Back saat preview | Kembali ke persiapan; camera client staging kosong |
| Background/resume | HOME; client awal masih terlihat saat query langsung, kosong setelah settle1500ms; resume tetap persiapan tanpa camera auto-resume |
| Restart | Dashboard/session restore; kamera tidak auto-start |
| Unmount/navigation | Keluar scanner dan force-stop; tidak ada client kamera staging aktif |
| Cancel setelah capture / background codec | **NOT_RUN**, tidak mencapai capture/codec |
| Permission revoke via Settings | **NOT_RUN**, tidak ada izin tambahan |

Cleanup perangkat: staging force-stopped, tetap installed; main tidak diubah/uninstall. Mapping reverse task8089/8091 dihapus; XML UI sementara task dihapus. Metro, relay, backend task berhenti. Backend dihentikan Ctrl+C; supervisor melaporkan exit512, tetapi root fixture task sudah terhapus. Tidak menghapus APK/video/evidence lama.

**Setting:** tidak ada perintah settings put pada task ini. Sebelum jeda setting identik baseline. Setelah jeda teramati brightness4→1 dan screen_off_timeout15000→1800000; nilai tersebut dipertahankan, bukan di-restore diam-diam. Font1.0, rotation0/accelerometer1, TalkBack off/service null/touch0 dan setting lain yang dicatat tetap sama. Dengan demikian persyaratan “seluruh setting awal/akhir sama” **tidak dapat diklaim PASS**, walaupun agent tidak melakukan perubahan setting.

## J. UI dan accessibility

- Consent fixture disimpan backend sebelum scanner. Notice synthetic-v1 tetap notice uji sintetis, bukan consent legal enrollment manusia.
- Permission explanation, start eksplisit, guidance NO_FACE/TOO_FAR, progress nol, cancel dan Back teramati; status backend tetap Belum terdaftar. Tidak stuck loading pada jalur tersebut.
- **Defect D2:** intro mannequin masih menyebut “Tidak ada kamera nyata”, alur sintetis, review dan retensi foto pending. Fix lokal conditional mode pada `IntroPanel`: preview lokal mannequin, tanpa simpan/upload/submission/enrollment; synthetic copy tetap untuk mode sintetis. Runtime sesudah reload menampilkan copy yang benar.
- Label aksesibilitas start/cancel/oval/progress terbaca lewat UI hierarchy; tidak ada klaim spoken focus/TalkBack penuh. TalkBack/font scale/Settings tidak diubah. Kontras tombol merah residual R2 tidak dinyatakan selesai.
- Branding/dashboard/video tidak didesain ulang. Step tracker prepare masih jalur shared enrollment; acceptance positive progress/capture dan seluruh state error belum lengkap.

## K. Regression

| Pemeriksaan yang dijalankan | Hasil teramati |
|---|---|
| Main TypeScript, lint, mobile tests sesudah copy fix | PASS;147 tes,1 opt-in E2E skipped |
| Staging TypeScript, lint, mobile tests final | PASS sesudah diagnostik dihapus;147 tes,1 opt-in E2E skipped |
| Native pixel/adapter JVM | PASS;35+224 assertions; codec Android live tidak dicakup JVM |
| JS guidance/facade | PASS;21+10 |
| Android ARM64 build native lifecycle fix | BUILD PASS8m31s; APK0ed8952e…751fd9 dipasang; rebuild final sumber bersih PASS6m54s,656 task/41 executed, hash tetap sama |
| E2E client/backend sintetis | Dua percobaan readiness30detik gagal ketika build berjalan; run tanpa build PASS1/1 sebelum dan sesudah cleanup diagnostik (run akhir13,99detik). Readiness smoke3975ms/exit0/root dihapus. Penyebab timeout belum diisolasi sepenuhnya; tidak mengubah deadline untuk menyembunyikannya |
| Video lifecycle | Nol warning released-player/FATAL pada sampling PID live; jalur restart/session restore teramati, bukan ulangan seluruh A/B TalkBack R2 |
| Main isolation | Source main manifest/package tanpa CAMERA/VisionCamera/host; staging flag eksplisit; cmd/server tanpa injeksi Enrollment; APK main hash awal/akhir identik |

Regresi tidak mengubah OTP/session/role. Build PASS, unit PASS dan hasil LIVE dipisahkan dalam laporan ini.

Perintah: main/staging `npm run typecheck && npm run lint && npm test`;
staging E2E `FACELINK_ENR_E2E=1 npx vitest run tests/enrollmentE2E.test.ts`;
CAM00 `node plugins/detector/run-boundary-tests.cjs` dan
`node --test guidanceModel.test.mjs nativeCaptureAdapter.test.mjs`;
stage Android `gradlew.bat app:assembleDebug -PreactNativeArchitectures=arm64-v8a --offline --console=plain`.
Final AAPT audit tetap CAMERA, tanpa storage/media/overlay. Sesudah run E2E
terakhir:0 proses fixture dan port8089/8090/8091 kosong.

## L. Defect dan remaining blockers

Berkas source permanen yang diubah:

- `packages/native-capture-host/android/src/live/java/ai/facelink/capturehost/FaceLinkCaptureLiveModule.kt`: pelepasan sink saat host destruction.
- `packages/native-capture-host/package.json`:0.2.3 agar stage local-file package memuat native fix, bukan copy cache lama.
- `apps/mobile/src/features/enrollment/EnrollmentScreen.tsx`: copy intro mannequin sesuai mode, tanpa perubahan synthetic flow/dashboard.
- Laporan ini, PROJECT_STATE, README, BACKLOG dan TEST_PLAN: status faktual acceptance.

Blocker: positive readiness dan mapping oval/mirror/crop belum terbukti; same-frame capture/consume/cleanup positif, duplicate/stale live, dua wajah dan background codec belum teruji. Data yang ada tidak cukup untuk mengklaim defect geometri tertentu atau menjamin mannequin memenuhi deteksi ML Kit. Tidak ada workaround threshold atau bypass keamanan.

Observasi pemilik mengenai completion dan snapshot native tidak cocok; jangan mengubah rekaman kosong menjadi PASS. Perlu satu run terkontrol dengan fixture yang terdeteksi dan trace identitas dari READY sampai CONSUME. Settings differences setelah jeda dan ketergantungan Metro debug tetap didokumentasikan.

## M. GO/PARTIAL/NO-GO

**Acceptance keseluruhan PARTIAL. NO-GO menuju Admin Enrollment UI integration berdasarkan hasil task ini:** happy path native live belum dibuktikan, walaupun preview/negative guidance/lifecycle dan regresi offline menghasilkan bukti berguna.

Rekomendasi tugas berikutnya: selesaikan positive mannequin readiness dan verifikasi geometri tanpa melemahkan syarat, lalu buktikan satu capture same-frame, native stale/duplicate rejection dan release buffer. Device run berikutnya harus punya izin baru sesuai gate tugasnya.

Tidak ada izin otomatis live submission, manusia/mahasiswa, production/template, database Product Owner, ESP-CAM, SMTP/OTP/role, commit/push/merge atau PR19. **STOP setelah laporan.**

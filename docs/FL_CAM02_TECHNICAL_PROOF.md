# FL-CAM02 — KYC-Style Camera: Final Capture & Privacy Verification

Tanggal: 25 Oktober 2026. **Isolated technical verification — no production enrollment.**

**Gate akhir: PARTIAL untuk integration readiness.** Blocker terminal live dan same-frame JPEG sudah ditutup pada TECNO LJ7 melalui release R2. Jalur encoder yang diaudit tidak menulis file gambar; observasi sandbox/UID mendukung hasil itu dalam jendela uji. Interupsi tepat saat codec/bitmap masih aktif belum teramati, observasi SDK/OS tidak menyeluruh, dan integrasi utama belum disetujui. **Capture manusia dan integrasi production tetap NO-GO.** Tidak menyamakan batas ini dengan kegagalan deteksi/capture normal yang sudah terbukti.

Tidak mengklaim Enrollment E2E PASS, Biometric Security PASS, Liveness/PAD PASS, Recognition Accuracy PASS, zero-telemetry, secure-erasure forensik, atau kesiapan deployment. Tidak memulai FL-ENR01/ESP-CAM.

## A. Baseline dan preservation

- Branch `feat/fl-otp01-email-auth`; HEAD `87bac5e96602e9ff5568069d1c6b1bd1d96913b7`.
- Awal CAM02: 26 tracked unstaged, 24 untracked, 0 staged. Tidak reset/stash/commit/push/merge; pekerjaan CORE01/CORE02, R5 dan dokumen sebelumnya dipertahankan.
- Dibaca: README, PROJECT_STATE, PRD, DECISIONS, ARCHITECTURE, SECURITY_PRIVACY, [CAM00](FL_CAM00_TECHNICAL_PROOF.md), [CAM01](FL_CAM01_TECHNICAL_PROOF.md), [proposal enrollment](FACE_ENROLLMENT_V1.md), dan [decision review](FACE_ENROLLMENT_DECISION_REVIEW.md), beserta source/dependency prototype.
- Perubahan terbatas pada `research/fl-cam00-proof/` dan dokumentasi laporan/status. Tidak mengubah `apps/mobile` / `ai.facelink.mobile`, OTP/session, role, backend, database Product Owner, migration, P01–P12, DNS/SMTP, video atau backup. Tidak menjalankan ESP-CAM atau mengaktifkan API/template enrollment.
- Semua APK root CAM00/CAM01 dipertahankan. Sebelum build, generated `android/app/build/outputs/apk/release/app-release.apk` disalin eksklusif ke `cam01-generated-release-preserved.apk` (isi sama dengan CAM01 R2). APK CAM02 R1/diagnostik/R2 juga disimpan terpisah; generated build output kemudian diperbarui normal oleh Gradle.
- **Tidak menjalankan Expo prebuild** pada CAM02. Kehilangan generated x86_64 APK saat CAM01 tetap catatan historis, bukan dipulihkan/dihilangkan dari laporan.
- Product Owner memberi izin perangkat dan arah fixture secara eksplisit. Kamera awal OFF; pengujian hanya mannequin/fixture nonmanusia. Tidak meminta perubahan pengaturan sistem.

## B. Source changes

Path relatif terhadap `research/fl-cam00-proof/`:

| Berkas | Perubahan |
|---|---|
| `App.tsx` | Native tagged `Cam02Frame` sebagai input live, trace transisi tanpa UI throttle, keputusan native per frame, begin/cancel/lifecycle guard, hasil JPEG metadata terpisah dari approval logis, UI CAM02 |
| `captureController.ts` | High-water mark ID frame decimal tanpa kehilangan presisi Long, satu approval per siklus, penolakan hasil stale/mismatched/unsafe |
| `guidanceModel.ts`, `guidanceModel.test.mjs` | Error native dapat mencabut approval terminal; regresi duplicate/stale/background/result validation; threshold guidance tetap |
| `plugins/detector/src/main/kotlin/.../Cam02FrameStore.kt` | Lease ImageProxy yang sama, deadline native, epoch, worker encoder, reservation cap, bitmap TTL/cleanup dan diagnostik coverage perangkat |
| `plugins/detector/src/main/kotlin/.../Cam02Pixels.kt` | Sampling YUV strided langsung ke NV21 upright/mirrored, crop konservatif, bounded OutputStream, parser JPEG/EXIF; fix float roundtrip |
| `plugins/android/Cam02CaptureModule.kt` | Bridge tanpa payload gambar, native lifecycle, secure dummy dialog, bounded FileObserver dan UID TrafficStats |
| `plugins/android/Cam01PrivacyPackage.kt`, `plugins/withCam01Privacy.js` | Registrasi/sinkronisasi modul baru; proteksi CAM01 dipertahankan |
| `plugins/detector/patch-detector.cjs` | Patch analyzer/Gradle terikat hash upstream detector 2.1.0; source-set helper prototype dan sinkronisasi generated Kotlin |
| `plugins/detector/tests/Cam02PixelsTest.kt`, `run-boundary-tests.cjs` | Regresi pixel/format/resource boundary JVM offline |
| `package.json`, `package-lock.json` | Bootstrap postinstall patch dan metadata install-script; tidak menaikkan dependency |
| `audit-apk.mjs` | Permission release memakai allowlist eksak CAMERA + permission receiver internal |
| `fixture-viewer.html` | Refresh sekarang langsung menampilkan satu mannequin simetris agar fixture tidak diam-diam kembali kosong; tombol 0/1/2 tetap tersedia |

Generated Android dan dependency lokal mengikuti patch reproducible, bukan edit tersembunyi yang hilang saat install ulang. Patcher memeriksa versi dan hash analyzer/Gradle/geometry dependency sebelum menulis; menerima hanya upstream yang dikenal atau hasil patch yang dapat dibalik persis ke hash tersebut. Drift ditolak, bukan dipatch secara best-effort. Native lease diserahkan hanya dari completion ML Kit, bukan saat ML Kit masih membaca image.

Alternatif `useFrameOutput` diperiksa: membutuhkan paket worklets tambahan yang belum terpasang. Output native existing dipertahankan agar tidak menambah stack/dependency baru. Tugas integrasi tidak boleh menyalin patch ini ke aplikasi utama tanpa review tersendiri.

## C. Live terminal capture evidence

**Bukti B (kamera live), bukan bukti A (unit/synthetic):** pengguna mengarahkan kamera depan TECNO ke viewer mannequin laptop. Media CAMiller62 `CosSchlHead2on112010`, CC BY-SA 3.0, [Wikimedia Commons](https://commons.wikimedia.org/wiki/File:CosSchlHead2on112010.jpg); turunan simetris tetap objek mannequin, bukan manusia. Atribusi/aset lama dipertahankan.

Trace R1 membuktikan NO_FACE (0), MULTIPLE_FACES (2), STABLE, OUTSIDE_OVAL, MOVEMENT dan ROLL yang mengulang dwell. R1 kemudian benar-benar mencapai terminal live, tetapi encoder gagal geometry; kegagalan itu tidak dihitung sebagai JPEG PASS.

Trace sukses R2, siklus/generation 3:

| Event | Frame ID | Wajah | Dwell ms | Sampel | Trigger logis/native |
|---|---|---:|---:|---:|---|
| FRAME → CAPTURE_READY | 2668 | 1 | 1507.129 | 31 | 0 / belum encode |
| FRAME → SIMULATED_CAPTURE | 2669 | 1 | 1553.022 | 32 | 1 / approval dipilih |
| NATIVE_CAPTURE_RESULT → JPEG_READY | **2669** | 1 | 1553.022 | 32 | **1 / 1** |

Event FRAME terminal muncul sekali dalam siklus itu, dan event hasil native juga sekali. `state=SIMULATED_CAPTURE` pada record hasil adalah konteks fase, **bukan trigger capture kedua**. Hitung berdasarkan `event`, `cycle`, `frameId` dan trigger count, bukan jumlah baris yang mengandung nama state.

Threshold tetap: dwell ≥1500 ms, minimum 6 sampel, gap maksimum 400 ms; satu wajah, seluruh box dalam ellipse, ukuran/pose memenuhi guidance. Tidak melonggarkan pose/stabilitas agar tes lulus. CAPTURE_READY harus diikuti observasi valid berikutnya. Frame invalid/hilang/lebih dari satu, drift, gap, layout atau epoch lama tidak dapat membawa dwell lama ke capture.

Trace `CAM02_EVENT` memuat source, cycle/generation, sequence, frame ID, state/reason, count, dwell/samples, clock dan trigger count; hasil menambah ukuran/waktu/limit JPEG. Tidak memuat piksel, embedding, pose/face box, OTP, token atau identitas. Source `synthetic_fixture` dan clock virtual terpisah; fixture unit tidak memanggil native encode. Pengujian unit tidak dipakai untuk menyatakan live PASS.

Penjelasan gap CAM01: logging berada di publication UI yang dibatasi frekuensi, sehingga transisi pendek dapat hilang. CAM02 memisahkan trace dari throttle UI. Perbaikan observability itu sendiri bukan bukti live; record R2 di atas adalah bukti actual live.

## D. Frame-to-JPEG pipeline

1. Analyzer mencatat epoch dan waktu monotonic **sebelum ML Kit**. Completion memegang ImageProxy beserta hasil analisis dari image tersebut.
2. Native hanya menawarkan satu kandidat outstanding, dengan ID unik monotonic. Tidak ada slot “latest photo” yang dapat diganti frame berikutnya.
3. JS menilai metadata frame bertag itu melalui converter sensor↔preview native. Frame nonterminal ditolak/dilepas; frame terminal disetujui sekali dengan empat sudut preview dalam sensor coordinates.
4. Native memeriksa generation/ID/freshness saat decision dan saat worker masuk; ImageProxy yang disetujui itulah yang disampling. Tidak memanggil shutter, `takePhoto`, snapshot preview lain, atau file detector sebagai pengganti.
5. Empat sudut dipetakan ke buffer ImageProxy; crop harus contained dan orthogonal. Sampling mengikuti rowStride, pixelStride dan posisi ByteBuffer langsung ke NV21 final, termasuk rotasi/mirror dari transform. Tidak menyalin seluruh input ke JS/RGB per frame.
6. `YuvImage.compressToJpeg` menulis ke capped OutputStream berbasis byte array native. Tidak memakai file writer, temp URI, MediaStore, encoder network atau image-loader URI.
7. Parser marker memeriksa JPEG/EXIF, decode memakai jumlah byte valid (bukan kapasitas backing), dimensi diverifikasi. JS menerima **metadata saja**. Bitmap hasil decode ditampilkan melalui native secure ImageView, bukan loader file/cache.
8. Backing NV21/JPEG diisi nol di `finally` sebelum publication hasil. Bitmap mutable dihapus/recycle ketika dialog ditutup/cancel/background atau TTL 15 detik. Input CameraX dilepas, bukan diklaim seluruh buffer milik platform ikut di-zeroize.

Pengukuran satu hasil normal R2:

| Metrik | Aktual |
|---|---:|
| JPEG | 133692 byte |
| Dimensi | 960 × 1220 |
| Encode + validasi native | 82 ms |
| Usia sejak analyzer-entry saat worker mulai | 47 ms |
| Peak explicit owned image-buffer reservation | 10635904 byte |
| EXIF ditemukan | false |

Batas: input/output ≤2.073.600 pixel, satu kandidat, JPEG backing ≤4 MiB, explicit owned-image reservation ≤16 MiB, freshness ≤400 ms. Batas ini **bukan hard cap seluruh proses**; CameraX/ML Kit, codec scratch, GPU, allocator/JVM dan OS tidak tercakup. Snapshot live terpisah: PSS 170940 KiB, RSS 278865 KiB, swap PSS 37874 KiB; bukan soak/leak proof.

JPEG dibuat dari YUV sehingga metadata EXIF sumber tidak diteruskan; parser juga menolak EXIF. Tidak upload ke backend/produksi. Tidak ada byte JPEG/base64/image buffer di trace atau hasil bridge. Hasil JPEG_READY adalah metadata historis; backing JPEG sudah dibersihkan, sedangkan bitmap sementara memiliki lifecycle tersendiri. Consumer enrollment belum dibuat.

### Kegagalan nyata dan fix minimum

R1 mencapai CAPTURE_READY 1532.939 ms/31 sampel lalu SIMULATED_CAPTURE 1580.077 ms/32 sampel, frame 8074, trigger 1; kemudian **ERROR / CAPTURE_ERROR / E_CAM02_PREVIEW_NOT_CONTAINED** sebelum alokasi JPEG. Kamera tertutup dan peak ownedBytes tetap 0. Konfirmasi visual pengguna saat percobaan awal tidak dipakai untuk menutupi trace kegagalan ini.

APK diagnostik mengukur buffer 1280×960, rotation 270°, crop [0,0,1280,960]. Preview dipetakan ke x=[28.5258407593,1251.4739990234], y=[0,960.0000610352]. Overflow hanya **0.00006103515625 pixel**. Ini numerical roundtrip, bukan kekurangan FOV atau kesalahan density pada perangkat ini.

Fix: margin comparison 0.05 pixel, pembulatan even **ke dalam** tetap, lalu guard exact crop containment sebelum sampling. Tidak broad clamp/intersection, tidak menambah density/manual rotation atau mengganti stream. Output berada di crop; inset quantization maksimal kurang dari dua buffer pixel per sisi. Regresi measured geometry gagal sebelum fix dan lulus setelahnya, termasuk signed noise empat sisi, odd crop dan penolakan overflow >margin. R2 lalu membuktikan JPEG actual. Perangkat lain dengan coverage benar-benar tidak contained tetap ditolak.

## E. Cache/storage evidence

Inventaris metadata membaca path relatif, ukuran, mtime dan signature gambar; tidak mengekstrak piksel, isi preference/database SDK atau storage pengguna lain. Tidak menghapus seluruh cache untuk memperoleh hasil bersih.

| Snapshot | File | Total byte |
|---|---:|---:|
| Sebelum update CAM02 | 11 | 4871132 |
| R1 terpasang, sebelum live | 11 | 4887788 |
| Akhir sesudah R2/JPEG/force-stop/restart | 11 | 4283036 |

Tidak ada path baru. Perbedaan final terhadap baseline release hanya `files/profileInstalled` (24 byte; mtime berubah) dan `cache/oat_primary/arm64/base.art` (704784 → 100032 byte; mtime berubah), yaitu metadata/profile/code-cache, bukan output foto. Tidak mengklaim semua file byte-identical dari metadata saja.

Tetap ada cache historis CAM00:
- PNG signature 187 byte di `cache/image_cache/v2.ols100.1/8.0/vmYuN2Tf2zQJBNPh_9xpXzOkGlo.cnt`; ukuran/mtime 1792886949000 tetap sama.
- DataTransport DB 65536 byte, mtime 1792887045000; journal 0 byte; ML Kit preference lama.
- Dev JS bundle lama 4116651 byte pada app data; bukan bukti APK release memerlukan Metro.

Watcher selama siklus JPEG sukses: 15 direktori, 25 baseline entries, create/close-write/delete/move=0, registration error/overflow/truncation flags=0. Snapshot post-TTL: heldFrames=0, ownedBytes=0, busy=false, displayed=false. Sesudah force-stop/cold restart status kembali IDLE dengan nilai resource 0. Watcher dimulai saat begin, berhenti pada completion/cancel; event queue/dynamic-directory race, batas 128 direktori/1024 nama/32 event, external/system storage dan waktu di luar window tidak ditutupi. **Tidak menyimpulkan nihil temporary write global atau secure-erasure forensik.**

Source encoder tidak menulis gallery; tidak membuka gallery pengguna untuk audit. Force-stop tidak menjalankan Kotlin finally; proses direklamasi OS. File gambar baru tidak ditemukan sesudah pengujian, tetapi kill/crash tepat di tengah encode/decode **belum teramati**.

## F. Telemetry findings

ML Kit bundled 16.1.7 tetap memakai komponen common/Google dan DataTransport. Cache Maven yang diperiksa mencakup common 18.11.0, transport-api 2.2.1, transport-runtime 2.2.6 dan backend-cct 2.3.3; tidak menambah SDK telemetry baru.

[ML Kit terms](https://developers.google.com/ml-kit/terms) menyatakan input gambar/video dan hasil diproses on-device, tidak dikirim ke Google oleh ML Kit. [Android disclosure](https://developers.google.com/ml-kit/android-data-disclosure) menyebut device/app information, per-installation identifier, performance, API configuration/input-output size, version, event/error metadata. Ini disclosure vendor, bukan packet capture/pembuktian pinned binary seluruh SDK.

Standalone release actual tidak memiliki INTERNET/ACCESS_NETWORK_STATE/ACCESS_LOCAL_NETWORK. UID TrafficStats valid, bukan -1:

| Counter UID lifetime | Sebelum scan | Sesudah JPEG/force-stop/restart | Delta |
|---|---:|---:|---:|
| Tx byte | 399794 | 399794 | 0 |
| Rx byte | 8470177 | 8470177 | 0 |
| Tx packet | 2014 | 2014 | 0 |
| Rx packet | 2698 | 2698 | 0 |

Angka awal mencakup traffic debug historis; bukan traffic CAM02. Tidak ada destination outbound standalone yang teramati/diatribusikan pada window ini. Tidak dilakukan packet capture/root/VPN atau audit seluruh system-service IPC. Server viewer localhost laptop dan bundler build-time adalah development traffic, bukan upload frame dari release.

**Bukan zero-telemetry.** Tidak adanya permission socket dan delta UID tidak menghapus local queue, disclosure vendor atau kemungkinan jalur sistem. Menambahkan INTERNET saat integrasi utama akan mengubah batas perlindungan ini dan memerlukan penilaian tersendiri. DataTransport DB bukan bukti pengiriman gambar biometrik.

## G. Android lifecycle dan device safety

- Start eksplisit, foreground/permission/privacy bridge wajib; resume tidak rearm. Native onHostPause/destroy juga membatalkan epoch tanpa menunggu JS.
- Background live menghasilkan CANCELLED/BACKGROUND; hot resume tetap OFF, faces/stability/samples kembali nol. Contoh R2: 2395 frame sudah diterima dalam siklus NO_FACE sebelum background.
- Tombol cancel benar-benar diuji: siklus 7 → CANCELLED/OPERATOR_STOP, generation 8, trigger 0. Bukan sekadar unit test.
- Force-stop saat camera client ID1 milik prototype aktif (PID29781) menutup device 0/1/no client. Cold restart kembali IDLE/OFF tanpa auto camera.
- Upaya otomatis JPEG→HOME **tidak menghasilkan event yang dituju**: satu scan tetap NO_FACE dan satu listener tidak mendapatkan siklus capture target; deadline HOME tidak dianggap encoder-interruption PASS. Tidak mengarang coverage cancel/background ketika codec sedang bekerja.
- Cooperative cancellation dicek per row dan sebelum publication. Platform JPEG encode/decode yang sudah masuk harus kembali sebelum alokasi dapat dibersihkan. ML Kit yang tidak pernah complete tidak aman dipaksa menutup proxy; disposal tidak memblok UI, tetapi kasus hang tersebut belum diuji.
- FLAG_SECURE activity dan native dialog dipertahankan; recent-app snapshots dinonaktifkan native. Screenshot scan R2 via exec-out hanya dianalisis dalam RAM host: 1080×2460, area tengah max RGB=0. Tidak mematikan proteksi untuk melihat kamera. Screenshot dialog JPEG dan recent thumbnail R2 tidak diperiksa independen; control source tetap ada. Pengguna mengonfirmasi JPEG R2 sesuai objek/orientation/mirror/crop preview, bukan kalibrasi pixel-level lintas perangkat.
- CAMERA granted pada penggunaan live; tidak memakai pm grant/revoke. Denied/revoked/one-time permission di perangkat bukan PASS; fail-closed diuji pada reducer/gate. Tidak reboot/uninstall/root atau menonaktifkan keamanan Android.

Rekonsiliasi read-only:

| Pengaturan | Awal CAM02 | Akhir CAM02 |
|---|---:|---:|
| screen_brightness | 4 | 4 |
| screen_brightness_mode | 0 | 0 |
| screen_off_timeout | **1800000** | **15000** |
| stay_on_while_plugged_in | 15 | 15 |
| accelerometer_rotation | 0 | 0 |
| user_rotation | 0 | 0 |

**Asal perubahan timeout belum diketahui.** CAM02 tidak mengeluarkan command settings put atau perubahan pengaturan lain; grep source App/plugins tidak menemukan setter terkait, dan APK tidak meminta WRITE_SETTINGS. Tidak menyimpulkan siapa/apa yang mengubahnya, tidak mengembalikan nilai sebelumnya tanpa izin. Nilai sebelum CAM00 yang tidak diketahui juga tidak ditebak. Pengaturan global lain tidak diubah oleh CAM02; portrait adalah policy aplikasi lokal.

## H. Standalone validation

Stack tidak diganti: Expo 58.0.19 / React19.3 / RN0.88 / VisionCamera5.2.3 / detector2.1.0 / Nitro Modules0.37.1 / Nitro Image0.15.2. TECNO LJ7 Android16/API36 ARM64; actual camera buffer 1280×960. Landscape unsupported, portrait-only tetap.

Build `gradlew.bat assembleRelease -PreactNativeArchitectures=arm64-v8a --console=plain`: R1 sukses 6m14s; diagnostik 3m33s; R2 3m23s. LintVital app dan R8 aktif. Warning upstream/deprecation AGP/Kotlin nonfatal tidak disembunyikan.

Audit final: package `ai.facelink.cam00proof`, permission hanya CAMERA dan receiver internal; nondebuggable, cleartext/backup off; ARM64-only, 23 library/69 ELF LOAD, ELF/ZIP alignment 16KB PASS; JS bundle embedded 1382456 byte. HP memakai page size4KB, bukan runtime16KB proof. Release memakai debug signing lokal untuk update in-place tanpa wipe, bukan distribusi produksi; tidak menambahkan secret produksi.

R2 explicit cold-start setelah force-stop: total436/wait443 ms; cold-start setelah force-stop aktif:343/352 ms, UI IDLE/OFF. Launch WARM190/201 ms yang sempat terjadi tidak dihitung sebagai cold. Installed base.apk SHA cocok dengan artefak R2. Tidak ada Metro runtime, reverse ADB kosong. Metro bundler yang muncul di log build hanya membuat embedded bundle, bukan service runtime yang dibutuhkan APK.

Semua path artefak di root prototype dipertahankan:

| APK | SHA-256 | Hasil |
|---|---|---|
| `cam02-arm64-release.apk` | `f59604a278919dc24d563a1faaa3c71ed97c8a5c9a8697191b64c9f2fadb3e9c` | R1; terminal live PASS, JPEG geometry FAIL |
| `cam02-arm64-release-diag.apk` | `839999c2cde86a7b06478f4586c2e83d2d085a963e8662b3e169f089371a1032` | Membuktikan overflow numerik |
| **`cam02-arm64-release-r2.apk`** | **`e97cac1916731ab9c1ab544b32fb06aab557d155cb16d66def002d28fd225acb`** | Final; actual same-frame JPEG PASS terbatas |

Tidak ada crash native baru yang teramati selama skenario CAM02. Crash buffer masih memuat CAM01 R1 PID30592 pukul11:30:57; buffer tidak dihapus. Handled CAPTURE_ERROR R1 CAM02 bukan native process crash.

## I. Tests and regression results

Perintah dari prototype:

```text
node plugins/detector/patch-detector.cjs
node plugins/detector/run-boundary-tests.cjs
node --experimental-strip-types --test guidanceModel.test.mjs
npm run typecheck
```

- Final: **35 assertion boundary Kotlin PASS**, **21 tes guidance/capture PASS**, TypeScript exit0. Kotlin executable offline memakai compiler cache2.2.21; build Android memakai Kotlin2.2.10/JDK17/AGP9.2.1/Gradle9.4.1. Tidak men-download runtime/model baru untuk uji boundary.
- Boundary: row/pixel stride dan ByteBuffer position, rotasi/mirror/pixel mapping, conservative inset/containment, cancellation checks, resource/stream caps, parser EXIF termasuk post-scan/escaped markers dan exact valid-byte count.
- Guidance/controller: 0/1/multiple, ellipse/pose/drift, dwell/sample/gap, readiness invalidation, clock/layout reset, duplicate frame/trigger, background/cancel/restart generations, synthetic isolation, stale/mismatched/unsafe result, native error setelah logical terminal.
- Gagal sebelum fix: typecheck callback NativeEventEmitter tidak cocok dengan variadic Object signature. Adapter callback bertipe eksplisit diperbaiki, kemudian typecheck lulus; tidak memakai broad any.
- Regresi measured crop gagal E_CAM02_PREVIEW_NOT_CONTAINED sebelum perubahan helper, lalu lulus setelah fix numerik.
- Physical smoke: live terminal, JPEG, visual confirmation pengguna, TTL cleanup, NO_FACE rejection, background/resume, operator cancel, screenshot scan, force-stop aktif, cold restart dan metadata storage/UID. Tidak mengganti smoke dengan unit test.
- Belum: forced native crash, kill tepat di codec, interupsi dengan bitmap masih tampil yang terikat trace, permission revocation actual, soak/thermal/leak, runtime16KB, perangkat lain/landscape. Regresi main app/backend tidak diulang karena tidak disentuh.
- Tooling: dump UI gagal idle saat callback aktif; file XML lama tidak dipakai. Parser audit juga menangani atribut XML single-quoted agar JSON metadata tidak hilang. XML sementara milik CAM02 dibersihkan.

## J. Independent security review

Reviewer terpisah `Cam02Review`, read-only actual source; tidak menjalankan check atau mengedit source. Meninjau race lease/epoch, stale/duplicate, geometry, caps, EXIF, cleanup, telemetry dan patch provenance. Tidak menemukan actionable source issue pada final fix. Review awal tidak membuktikan runtime dan tidak menangkap error numerik perangkat; diagnosis aktual serta fix kemudian direview kembali. Ini bukan security certification.

Reviewer mengonfirmasi margin hanya comparison, rounding tetap inward, final exact guard authoritative, dan regresi empat sisi/odd crop/excess overflow sesuai. Batas yang dipertahankan: cooperative codec cancellation, force-stop bukan finally/zeroization, cap hanya own buffers, JPEG_READY metadata historis, observer/UID bukan forensik atau zero-telemetry. Karena prototype ignored/untracked, scoped Git diff/history tidak memberi provenance before/after; reviewer memakai source aktual serta pinned upstream hash, bukan mengarang tracked diff.

## K. Remaining blockers dan lokasi evidence

Evidence metadata (tanpa gambar) pada `research/fl-cam00-proof/evidence/`:

| Berkas | Isi |
|---|---|
| `cam02-r1-live.jsonl` | Live 0/1/2/guidance, terminal dan error R1 |
| `cam02-r1-storage.json` | Git/APK/settings baseline, inventaris dan status error |
| `cam02-diag-coverage.json` | Terminal diagnostik dan geometri overflow terukur |
| `cam02-r2-live.jsonl` | Terminal/result frame2669, no-face/background/cancel dan siklus lain aktual |
| `cam02-r2-privacy.json` | Native status sebelum/TTL/restart, inventaris lengkap, UID, screenshot stats, force-stop, settings delta dan batas uji |
| `cam02-review.json` | Hasil/batas independent source review |

Blocker tersisa untuk gate lebih luas:
1. Interupsi saat encode/decode atau bitmap masih aktif belum memiliki physical trace yang dimaksud; tidak dianggap tertutup oleh timeout atau unit test.
2. Memory-only terbukti untuk mekanisme encoder yang diaudit dan didukung jendela observasi, **bukan semua SDK/OS/system cache**. Tidak ada forensic write tracing, packet capture atau full-process memory cap.
3. Re-enabling jaringan pada main app mengubah boundary telemetry; disclosure/kontrol integrasi harus direview. Tidak mengklaim izin network hilang sebagai solusi otomatis aplikasi utama.
4. P01–P12/consent/retensi/KeyProvider/consumer foto tetap belum approved/implemented. Metadata proof bukan artefak submission yang dapat di-upload; JPEG backing sengaja dibersihkan.
5. Coverage perangkat/permission/hang/soak masih terbatas; signing proof bukan signing deployment.
6. Timeout HP berubah tanpa asal terkonfirmasi; tidak boleh menyatakan semua setting identik atau memulihkan tanpa izin.

## L. Final GO / PARTIAL / NO-GO

| Lingkup | Gate |
|---|---|
| Terminal KYC-style live dan same-frame JPEG normal di prototype TECNO | **GO teknis terbatas** — terbukti, tidak ada shutter manual |
| Integration readiness keseluruhan CAM02 | **PARTIAL** — gap interupsi/SDK-system coverage tetap eksplisit |
| Capture manusia, integrasi production/main app, enrollment/deployment | **NO-GO** tanpa approval dan prerequisite terpisah |
| PAD/liveness, recognition accuracy, enrollment E2E | **TIDAK DIUJI / bukan hasil tugas ini** |

Tidak menghalangi pengakuan keberhasilan kamera normal dengan keputusan produk yang belum approved, tetapi juga tidak mengubah GO kamera menjadi persetujuan enrollment.

## M. Minimal next implementation step dan STOP

Product Owner meninjau bukti/gap ini, bukan otomatis membuat ronde proof umum atau memulai enrollment. Bila integrasi kelak disetujui, langkah implementasi paling kecil adalah kontrak adapter kamera untuk konsumsi native buffer sekali pakai oleh consumer yang diotorisasi, terikat frame ID/generation dan cleanup; tidak menyelundupkan API enrollment, perubahan OTP/role/schema atau persistence foto. Kebutuhan acceptance interupsi codec dan boundary telemetry main app harus diputuskan eksplisit sebelum capture manusia.

**STOP FL-CAM02.** Prototype akhir force-stopped; camera client tertutup, reader log/tab/server fixture milik audit dihentikan, reverse proof kosong. APK, fixture, video/backup dan bukti historis dipertahankan. Tidak commit/push/merge PR #19, tidak mengubah P01–P12, tidak memulai FL-ENR01 atau ESP-CAM.

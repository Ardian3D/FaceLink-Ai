# FL-ENR01A — Reusable Native Capture Adapter

## Tugas dan keputusan akhir

**SELESAI untuk implementasi dan verifikasi terisolasi yang dilaporkan di sini.** Adapter Android mendapat **GO teknis terbatas** untuk reuse dengan fixture sintetis dan kontrak ownership yang terdokumentasi. **NO-GO integrasi KYC-style ke aplikasi FaceLink utama saat ini**: perubahan aplikasi utama tidak diizinkan pada task ini, packaging/consumer bisnis belum diintegrasikan, runtime Development Build utama belum diuji, dan approval produk/privasi P01–P12 tetap belum diberikan.

Tidak ada enrollment API, template SFace, database migration, perubahan OTP/session/role, pengambilan wajah manusia, ESP-CAM, commit/push/merge PR #19, atau persetujuan P01–P12. Kamera HP **tidak dibuka** pada FL-ENR01A; pengujian Android menggunakan frame YUV sintetis dengan codec produksi. Bukti live frame 2669 CAM02 adalah bukti historis, bukan pengujian ulang jalur live adapter baru.

| Output | Status | Hasil |
|---|---|---|
| A. Source adapter | PASSED | Core native reusable, same-input encoder, bounded ownership, consume-once handle |
| B. API/adapter contract | PASSED | Start/guidance/ready/consume/cancel/error/dispose dan batas permission/privasi terdokumentasi |
| C. Unit/concurrency | PASSED | 224 assertion adapter JVM, 35 pixel-boundary, 31 tes JS, TypeScript |
| D. Fault injection | PASSED | 12/12 kasus Android; cancel/background-boundary/sink exception di write callback codec nyata |
| E. Buffer/cleanup | PASSED terbatas; privacy keseluruhan PARTIAL | Ownership akhir nol; tidak ada path capture baru pada jendela observasi; bukan zeroization/forensic/SDK-wide proof |
| F. Security review | PASSED untuk source review | Dua P2 diperbaiki dan direview ulang; bukan sertifikasi biometric security |
| G. Integration dependencies | PARTIAL | Stack cocok dan prototype ARM64 berjalan; main Expo Development Build belum diintegrasikan/diuji |
| H. Files changed | PASSED | Source dan evidence terisolasi, kontrak/laporan/PROJECT_STATE diperbarui |
| I. Integrasi KYC ke FaceLink utama | NO-GO | Perlu task integrasi dan persetujuan Product Owner; GO adapter tidak mengizinkan enrollment |

## A. Baseline, preservation dan source adapter

Baseline aktual sebelum perubahan:

- Branch `feat/fl-otp01-email-auth`.
- HEAD `87bac5e96602e9ff5568069d1c6b1bd1d96913b7`.
- Staged 0, tracked unstaged 26, untracked 25. Pekerjaan CORE01/CORE02, R5 dan dokumentasi sebelumnya dipertahankan; tidak reset/stash.
- APK CAM00/CAM01/CAM02 yang ada dipertahankan. Generated APK CAM02 disalin sebelum rebuild ke `enr01a-cam02-generated-preserved.apk`, SHA-256 `e97cac1916731ab9c1ab544b32fb06aab557d155cb16d66def002d28fd225acb`.
- Tidak menjalankan prebuild, uninstall, migration atau perubahan main app. Source bridge lama yang obsolete diganti, bukan dipertahankan sebagai alias.
- Semua audit/coding/unit/smoke host/build/audit APK selesai sebelum meminta izin perangkat. Product Owner kemudian memilih **“Tersedia — izinkan pengujian”**. ADB pertama hanya setelah izin tersebut. Guard parser enumerasi awal salah mengasumsikan padding whitespace; enumerasi yang sudah diperoleh menunjukkan perangkat authorized, parser diperbaiki tanpa mengubah konfigurasi perangkat.

`NativeCaptureAdapter` menerima ownership input hanya bila `capture()` mengembalikan true. Satu transaksi mencakup queued/encoding/READY/consuming/draining; generation meningkat ketat dan capture yang diterima menghabiskan generation itu. Reject tidak menutup input milik caller atau meng-close ulang objek aktif yang sama. Handle acak sekali pakai tidak pernah menjadi payload gambar JS.

`Cam02FrameStore` mempertahankan handoff ImageProxy setelah ML Kit complete, sensor-to-buffer transform, contained/even crop, mirror/orientation dan numeric tolerance CAM02. Ia menyerahkan frame terpilih yang sama ke core. Encoder tunggal `AndroidCaptureEncoder` menyalin crop YUV ke scratch NV21 milik adapter, lalu `YuvImage.compressToJpeg` menulis ke capped buffer JPEG milik adapter. Tidak ada second camera capture, Bitmap/Dialog preview, atau jalur encoder lama paralel.

Batas produksi: input/output ≤2.073.600 piksel; JPEG backing 4.194.304 byte; explicit owned image buffers ≤16.777.216 byte; freshness 400 ms; encode timeout/READY TTL/consumer timeout masing-masing 5.000 ms. Deadline per fase, bukan total 5 detik. Release codec/consumer aktif bersifat cooperative; bukan hard real-time atau batas RSS seluruh proses.

## B. API dan integration contract

Kontrak lengkap: [`FL_ENR01A_CAPTURE_ADAPTER_CONTRACT.md`](FL_ENR01A_CAPTURE_ADAPTER_CONTRACT.md).

- Bridge `NativeModules.FaceLinkCapture`, event `CaptureFrame`; tidak ada alias public `Cam02Capture`/`Cam02Frame`.
- Facade `CaptureAdapter.start()`/`decide()` mengeluarkan `CaptureLease` dengan metadata whitelist dan `consumeOnce()`.
- `consume()` native memberi ByteBuffer read-only dengan capacity/limit sesuai panjang JPEG sebenarnya, hanya selama callback sinkron. Prototype memakai consumer validasi marker dan bounds JPEG yang benar-benar membaca buffer di RAM.
- Receipt CONSUMED tidak berisi gambar/base64/path/URI. Buffer JPEG dan scratch dibersihkan; host baru menutup kamera setelah konsumsi selesai. Cancel, source close, timeout, background, error dan dispose mencabut hasil yang belum dikonsumsi.
- Guidance tetap kewajiban host tepercaya: exact one face, oval/pose/drift/dwell/sampel/gap dan observasi valid berikutnya. Boolean approval bukan consent, PAD, atau bukti kriptografis identitas.
- Foreground/Activity/FLAG_SECURE/CAMERA permission ditegakkan di bridge scanning. Suite diagnostik tidak meminta permission kamera atau membuka kamera. Instance suite terpisah dan reservasi global mencegah overlap dengan scanning.
- Callback yang sudah di-reserve dapat berlomba dengan dimulainya cancel; handle sudah tidak valid bila dicabut. Consume otoritatif, dan drain/new generation menunggu callback kembali.

## C. Unit, concurrency dan regression results

Perintah aktual dari prototype, kecuali path runner dapat dijalankan dari root:

```text
node plugins/detector/patch-detector.cjs
node plugins/detector/run-boundary-tests.cjs
node --experimental-strip-types --test guidanceModel.test.mjs nativeCaptureAdapter.test.mjs
npm run typecheck
```

Hasil akhir: **35 pixel assertion + 224 adapter JVM assertion + 31 tes JS lulus; TypeScript exit 0**. JVM memakai controlled encoders, bukan Android codec. Cakupan termasuk simultaneous capture/consume winner, duplicate/stale generation, rejected-input ownership, queued freshness, timeout saat encode/borrow, cancel/dispose, consumer/encoder/source/callback failure, scratch reservation rollback, scoped read-only length dan cleanup.

Regresi notification/drain ditambahkan setelah independent review. **Gagal sebelum fix** dengan `Drain acknowledged before the in-flight READY notification returned`; **lulus sesudah fix** untuk cancel dan dispose. Physical cleanup tidak perlu menunggu callback metadata, tetapi slot transaksi/drain acknowledgement menunggunya.

TypeScript pertama gagal `TS2345` karena nullable READY result belum dipersempit oleh control-flow terminator. `fail` dijadikan function declaration dengan return `never`; pemeriksaan berikutnya lulus. Warning Node `MODULE_TYPELESS_PACKAGE_JSON` tetap dilaporkan; tidak mengubah convention CommonJS plugin hanya untuk menyembunyikannya.

Smoke host terpisah menjalankan core dengan **codec JPEG JDK ImageIO nyata**, fixture merah/biru sintetis, memory image streams eksplisit: frame17/generation4 menghasilkan 730 byte, decoded pixels sesuai fixture, consume kedua ditolak, ownership akhir 0/0, close input sekali, direktori cache ImageIO khusus tetap kosong. Script/classes sementara dibersihkan. Ini bukan bukti Android YuvImage atau kamera live.

## D. Actual Android codec dan fault injection

TECNO LJ7, Android16/API36, ARM64, page size4096. Source native dan embedded JS berasal dari release R2 yang SHA-nya cocok dengan installed base.apk.

Tombol **“Uji adapter native (sintetis)”** dijalankan secara eksplisit, kamera tetap OFF. Structured trace `ENR01A_SUITE`/`ENR01A_CASE` mencatat **12/12 PASSED** dengan source `synthetic_yuv_android_codec`:

1. Consume once dan double reject.
2. Stale generation dan concurrent capture reject.
3. Concurrent consume satu pemenang.
4. Cancel pada codec first write.
5. Background melalui shared native lifecycle boundary pada codec first write.
6. Output/sink exception pada codec first write.
7. Queued input timeout tanpa codec masuk setelah expiry.
8. READY TTL expiry dan handle tidak dapat dipakai.
9. Consumer exception dan cleanup.
10. Consumer timeout: buffer tetap hidup saat callback masih membaca, hasil timeout bukan late success.
11. Owned-buffer budget rejection.
12. Real-codec capped-output failure.

Tiga first-write fault mengamati `codecActive` dan frame stack `android.graphics.YuvImage.compressToJpeg` sebelum injeksi. Native boundary background memakai fungsi yang juga dipakai jalur production pause; **bukan klaim HOME fisik tepat saat codec aktif**. Semua kasus memiliki callback completion satu, close input satu, ownedBytes/ownedFrames akhir nol dan drain diakui.

Fixture Android 16×16 menghasilkan JPEG709 byte pada kasus konsumsi normal; exact-length read-only bounds decoding lulus. Peak owned bytes maksimum yang teramati **4.194.688** (JPEG backing +384 byte NV21), bukan pengukuran RSS. Full-resolution/soak/multi-device belum dibenchmark. Error sink/capped-output nyata terlapor sebagai safe `E_CAM02_JPEG_FORMAT`; tidak dipaksa menjadi prefix error tertentu dan tidak menghasilkan handle.

Ini membuktikan jalur adapter/encoder native dengan frame sintetis. Tidak ada observasi live mannequin baru, enrollment submission, recognition atau liveness pada task ini.

## E. Buffer, storage, telemetry dan Android lifecycle

### Ownership dan storage

Core membersihkan scratch saat encoder keluar dan JPEG setelah konsumsi/cleanup; tidak wipe dari bawah codec/consumer yang masih memakai buffer. Input CameraX/ML Kit dan alokasi internal JNI/decoder/GPU/OS dibedakan dari buffer adapter. Mengisi nol array milik adapter **bukan total JVM/native memory zeroization**.

Snapshot private sandbox lengkap sebelum dan sesudah suite sama: **11 file /4.887.772 byte**, tanpa path baru/hilang atau perubahan metadata. Watcher aktif selama422 ms pada15 direktori, baseline25 entries; create/delete/move/closeWrite/overflow/registration error semuanya0, tanpa truncation yang dilaporkan. Ini bounded directory observation, bukan forensic all-process write tracing.

Cache historis tidak dihapus:

- PNG187 byte `cache/image_cache/v2.ols100.1/8.0/vmYuN2Tf2zQJBNPh_9xpXzOkGlo.cnt`, mtime1792886949000 tetap.
- Database DataTransport65536 byte, journal0 byte, metadata lama tetap.
- `files/BridgelessReactNativeDevBundle.js` historis tetap ada; tidak dihapus untuk mempercantik audit. APK release membawa embedded JS baru dan menjalankan UI/suite ENR01A, bukan bergantung Metro.

Setelah force-stop dan cold restart: **10 file /4.183.004 byte**, tidak ada path baru; `cache/oat_primary/arm64/base.art`704768 byte tidak lagi terlihat dan mtime `files/profileInstalled` berubah. Tidak ada command penghapusan cache dari agent. Ini perubahan ART/profile metadata yang teramati setelah restart, bukan atribusi forensik setiap tindakan OS. PNG/DataTransport historis tetap; tidak ada JPEG capture baru yang teramati.

Jalur source adapter tidak memanggil file/gallery writer, image loader, backend atau jaringan. Gallery pengguna tidak diinspeksi. Uji file mannequin historis pada prototype adalah jalur terpisah dan tidak dijalankan. Tidak menganggap absennya storage permission saja membuktikan MediaStore tidak dapat dipakai.

### Telemetry

Counter UID10002 sebelum/sesudah suite dan cold restart tetap RX8470177/TX399794 byte; delta0. Counter mencakup sejarah debug, bukan packet capture. Release tidak punya INTERNET permission, tetapi ML Kit/DataTransport dependency dan metadata/cache/IPC tidak hilang. Tidak ada bukti pengiriman frame/gambar pada jalur yang diuji; **tidak mengklaim zero-telemetry, daftar network destinations aktual, atau semua SDK tidak menyimpan/menyalin data**. Main app dengan jaringan mempunyai boundary berbeda dan perlu review/disclosure lagi sebelum data manusia.

### Lifecycle dan layar

- HOME setelah suite selesai mempertahankan proses dan menghasilkan trace `CANCELLED/BACKGROUND`. Resume memperlihatkan **KAMERA OFF**, tidak auto-start; bukan pengujian OS pause tepat di codec.
- Force-stop prototype membuat PID hilang. Cold restart membuat proses baru dan UI **IDLE/OFF**. Force-stop dilakukan setelah suite/cleanup selesai, bukan saat codec aktif.
- Tidak ada baris crash `AndroidRuntime:E`/`libc:F` pada dua PID prototype yang diperiksa. Ini jendela observasi, bukan jaminan tidak pernah crash.
- FLAG_SECURE diperiksa native dan konten aplikasi pada ROI `[70,120,1010,2350]` terukur hitam (0 piksel RGB nonblack). **Whole-screen all-black PARTIAL**: ada piksel nonblack di area atas/bawah dan strip kanan x1066–1079/y514–662; asal overlay tidak diatribusi independen. Jangan mengklaim seluruh display hitam. Screenshot hanya diproses sebagai statistik host, tidak disimpan; tidak menonaktifkan security atau mengambil thumbnail aplikasi pengguna. Recents protection tetap dikonfigurasi native, bukan visual proof thumbnail baru pada task ini.
- Tidak mengubah permission sistem atau menguji physical revoke; permission/error behavior yang dilaporkan berasal dari source checks/regression. Kamera tidak dibuka selama task.

### Device safety

Enam setting read-only awal/akhir identik: brightness4, mode0, screen timeout1800000, stay-on15, accelerometer rotation0, user rotation0. Tidak ada `settings put`, perubahan brightness/timeout/orientasi global/developer/USB/DNS/SMTP. Timeout akhir CAM02 dahulu15000 tetapi baseline task ini1800000; asal perubahan antarsesi tidak diketahui dan tidak dipulihkan/ditebak.

Akhir: prototype force-stopped, reader log milik task dihentikan, XML UI sementara milik audit dihapus, tidak ada Metro/reverse yang dibuat task. Main app PID kosong sebelum/akhir dan tidak dibuka/diinstal/diubah. APK dan semua backup/video dipertahankan.

## F. Independent security review

Dua reviewer read-only independen memeriksa core concurrency/ownership dan Android/RN/privacy/Expo boundary. Keduanya tidak menjalankan tes/ADB; runtime di atas dieksekusi terpisah oleh integration owner.

1. **P2 notification/drain race:** notified di-reserve sebelum callback sehingga cancel dapat mengakui drain lebih dahulu. Fix `notificationInFlight` + `cleanupComplete` mempertahankan slot hingga callback kembali sambil membolehkan physical buffer cleanup. Failing-before/passing-after regression dan source re-review mengonfirmasi closure. Caveat reserved READY vs cancel mulai tetap eksplisit; consume menolak handle yang sudah dicabut.
2. **P2 observer cross-session race:** task consume lama dapat menghentikan observer sesi baru. Fix identitas `watchSession`, generation dan idle/synthetic guard di bawah lifecycle lock; generated source disinkronkan. Independent re-review mengonfirmasi source fix/parity. Tidak mengklaim regression Android module race tersendiri yang belum dijalankan.

Re-review tidak menemukan additional evidence-backed defect dalam fix yang diperiksa. Ini bukan sertifikasi security biometrik, malicious-native-consumer isolation, total zeroization, atau bukti seluruh dependency bebas telemetry.

## G. Native build dan integration dependencies

Command aktual:

```text
gradlew.bat assembleRelease -PreactNativeArchitectures=arm64-v8a --offline --console=plain
```

R1 berhasil7m19s; R2 final4m5s, app lintVital dan R8 aktif. R2 menghapus assertion prefix error incidental pada fault suite dan tetap menguji actual codec/no-handle/cleanup. Warning upstream/deprecated Gradle dan override `Cam01PrivacyPackage` tidak disembunyikan.

| APK | SHA-256 | Keterangan |
|---|---|---|
| `enr01a-arm64-release.apk` | `89913b5d452d95e233c15a0925f1a95d84cf1614d02908d5eb97e6ece059a97d` | R1 dipertahankan |
| `enr01a-arm64-release-r2.apk` | `03bcf25e56e1004c5a5506c949051e55107e69862d5509531dacdda9d9f71f26` | Final,44778102 byte, terpasang dan diuji |

Audit APK R2: `ai.facelink.cam00proof`, minSDK24/target36/compile37, nondebuggable, backup/cleartext off, portrait lokal, hanya CAMERA dan receiver internal permission. Tidak ada INTERNET/storage/media/overlay permission. Embedded `assets/index.android.bundle`1395924 byte; ARM64-only,23 library/69 ELF LOAD, ELF dan ZIP16KB alignment lulus. Library native R2 byte-identical dengan R1 yang diaudit; device aktual4KB, bukan runtime16KB proof. Signature v2 valid, local Android debug certificate sama dengan CAM02 sehingga update in-place tanpa wipe; bukan signing distribusi produksi atau whole-APK secret certification. Tidak menambahkan secret production.

Actual cold-start tanpa Metro: pertama total519/wait681 ms, setelah force-stop total419/wait432 ms. Resume41 ms dengan LaunchState UNKNOWN tidak dihitung cold. Installed base.apk SHA cocok dengan R2.

Installed prototype: Expo58.0.19, React19.3, RN0.88, Nitro Modules0.37.1, Nitro Image0.15.2, VisionCamera5.2.3, detector2.1.0, TypeScript6.0.3. Main package memiliki Expo58/RN0.88/React19.3/Nitro0.37.1 dan expo-dev-client, tetapi belum memasukkan camera stack/patch/module tersebut. Build dan panggilan module baru terbukti pada prototype RN new architecture. **Runtime Expo Development Build aplikasi utama belum diuji.** Expo Go/iOS tidak didukung; packaging/autolinking/consumer bisnis tetap tugas integrasi tersendiri, bukan shim atau backend enrollment yang diam-diam dibuat.

## H. Files changed dan lokasi evidence

Root source relatif `research/fl-cam00-proof/`, kecuali docs:

- Baru `plugins/detector/src/main/kotlin/com/margelo/nitro/camera/facedetector/{NativeCaptureAdapter,AndroidCaptureEncoder,NativeJpegValidationConsumer,CaptureSyntheticVerification}.kt`.
- Diubah `.../Cam02FrameStore.kt`; `Cam02Pixels`/guidance math existing dipakai ulang, tidak dibuat convention encoder kedua.
- Baru `plugins/detector/tests/NativeCaptureAdapterTest.kt`; runner `plugins/detector/run-boundary-tests.cjs` diperbarui.
- Baru `plugins/android/FaceLinkCaptureModule.kt`; `Cam01PrivacyPackage.kt`, `plugins/withCam01Privacy.js`, dan `plugins/detector/patch-detector.cjs` diperbarui. Source/generated `Cam02CaptureModule.kt` obsolete dihapus; bukan APK/prototype/app uninstall.
- Generated `android/app/src/main/java/ai/facelink/cam00proof/{FaceLinkCaptureModule,Cam01PrivacyPackage}.kt` disinkronkan.
- Baru `nativeCaptureAdapter.ts` dan `nativeCaptureAdapter.test.mjs`; `captureController.ts`, `guidanceModel.test.mjs`, `App.tsx` diadaptasi untuk READY→consume-once→CONSUMED dan diagnostik eksplisit. Tidak redesign UI aplikasi utama.
- Baru kontrak/laporan ini, `PROJECT_STATE.md` diperbarui; tidak mengubah keputusan kanonik P01–P12.
- APK baru/preservation yang disebut di atas; semua APK historis tetap.

Evidence metadata tanpa gambar pada `research/fl-cam00-proof/evidence/`:

| Berkas | Isi |
|---|---|
| `enr01a-baseline.json` | Git baseline dan APK preservation sebelum ADB |
| `enr01a-device-baseline.json` | Izin eksplisit, device/settings awal |
| `enr01a-offline-checks.json` | Unit/regression/TypeScript dan scope runtime terpisah |
| `enr01a-host-smoke.json` | Actual JPEG host; jelas bukan Android |
| `enr01a-native-codec.json` | Summary +12 kasus actual Android beserta assertion/observasi |
| `enr01a-native-trace.jsonl` | Structured suite/case/background trace, tanpa gambar/handle |
| `enr01a-storage-before.json`, `enr01a-storage-codec.json` | Inventaris lengkap, watcher/UID dan delta setelah restart |
| `enr01a-lifecycle.json` | Cold start, HOME/resume, force-stop, installed SHA, settings akhir |
| `enr01a-screenshot-privacy.json` | Statistik ROI dan batas whole-screen; tidak ada PNG disimpan |
| `enr01a-security-review.json` | Dua temuan, perbaikan, re-review dan batasnya |
| `enr01a-release-audit.json`, `enr01a-release-r1-audit.json` | APK/manifest/signature/ABI/ELF/ZIP/bundle audit |

## I. Remaining blockers, keputusan dan langkah minimal berikutnya

**GO teknis terbatas:** core adapter Android reusable dan pipeline JPEG/lease/cleanup sintetis yang benar-benar diuji. Tidak perlu ronde proof umum lagi untuk mengulang hasil yang sudah cukup ini.

**NO-GO integrasi main app/capture manusia/enrollment production saat ini.** Prasyarat yang masih nyata:

1. Product Owner menyetujui task integrasi dan keputusan produk/privasi yang relevan; P01–P12 belum otomatis approved.
2. Packaging native/autolinking/config Expo Development Build utama, scanning host/permission lifecycle dan consumer bisnis yang diotorisasi belum diintegrasikan. Tidak boleh menganggap consumer validasi bounds sebagai enrollment E2E.
3. Boundary main app berjaringan, disclosure ML Kit/DataTransport, consent/retensi/KeyProvider/consumer data manusia perlu keputusan dan verifikasi terpisah. Observasi prototype tanpa INTERNET tidak menghapus risiko itu.
4. Full-process/SDK/OS memory persistence, hard realtime codec hang, full-resolution/soak/device matrix, runtime16KB, current live-camera adapter cutover, physical permission revoke, dan thumbnail recents baru bukan hasil PASS task ini.

Langkah minimal yang direkomendasikan: Product Owner meninjau kontrak dan memberi keputusan untuk **packaging adapter Android ke Expo Development Build dalam task terpisah**, tetap memakai fixture sintetis dan tanpa mengaktifkan enrollment. Tidak membuat endpoint/migration atau memulai capture manusia otomatis.

Tidak mengklaim **Enrollment E2E PASS, Biometric security PASS, Liveness/PAD PASS, atau Recognition accuracy PASS**.

**STOP FL-ENR01A.** Prototype akhir force-stopped; tidak memulai enrollment nyata atau ESP-CAM.

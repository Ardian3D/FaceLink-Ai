# FL-ENR02B-R2 — Live KYC Readiness & Same-Frame Capture Closure

Tanggal: 26 Oktober 2026. **Status keseluruhan: PARTIAL. Happy path kamera-ke-consume/release: LIVE DEVICE PASS.** Blocker positive readiness dan same-frame capture R1 ditutup oleh satu siklus teramati. Ini bukan izin production, human capture, live submission, Admin Enrollment UI implementation, atau FL-ENR03 otomatis.

## A. Baseline dan preservation

- Baseline branch `feat/fl-otp01-email-auth`, HEAD `87bac5e96602e9ff5568069d1c6b1bd1d96913b7`; pekerjaan uncommitted terdahulu dipertahankan. Tidak ada reset/stash/clean/commit/push/merge.
- Laporan R1, kontrak ENR01A, source guidance/adapter/host, dan dokumentasi canonical dibaca. Laporan lama merupakan bukti historis, bukan hasil R2.
- Main APK `ai.facelink.mobile` tidak dipasang ulang, dibuka, atau diubah. SHA-256 awal/akhir: `f4cc2eae08fc95681960e9fbf33808e382125961d6a4304c7b1d2cc60967bf8b`.
- Tidak ada perubahan OTP/session/role, database Product Owner, native capture implementation, model, threshold, SMTP/DNS, ESP-CAM, P01–P12 atau keputusan D-030.

## B. Diagnosis TOO_FAR

Review geometri independen tidak menemukan defect mapping yang terbukti menghasilkan pengecilan bounding box. Gate TOO_FAR membandingkan **bounding box preview dalam dp** dengan `0.7 * oval.rx` dan `0.65 * oval.ry`; dimensi frame hanya divalidasi, bukan denominator gate.

Ruang koordinat tidak boleh dicampur:

- `face.bounds`: CameraX sensor-active-array pixels setelah transform ML Kit ke sensor.
- `face.frameWidth/frameHeight`: dimensi gambar ML Kit setelah rotasi, bukan ukuran sensor.
- `previewGeometry.inputWidth/inputHeight`: dimensi buffer ImageAnalysis mentah.
- `previewBoxNormalized` dan `roiNormalized`: geometri dalam ruang preview yang sama.

Review menemukan kesalahan unit pada **trace baru sebelum device run**: sensor bounds sempat dibagi dimensi ML Kit. Ini diperbaiki menjadi `detectorBoundsSensor` mentah + `analysisImage`/`buffer` terpisah. Kesalahan diagnostik tersebut tidak berada dalam gate produksi dan tidak pernah digunakan sebagai bukti device.

Pada R2, pemilik tidak memiliki mannequin fisik. Viewer lokal lama ditemukan dan dibuka kembali setelah pemilik memintanya. Kamera nyata kemudian diarahkan ke **gambar mannequin nonmanusia pada layar laptop**, bukan wajah orang. NO_FACE, OUTSIDE_OVAL, HOLD_STILL dan reset akibat gerakan teramati; setelah posisi dan stabilitas memadai, READY tercapai. Ini tidak menetapkan penyebab tunggal kegagalan R1 yang tidak memiliki angka bbox/ROI.

## C. Geometry/mirror/crop evidence

Preview terukur `323.70367 x 473.70367 dp`; gambar ML Kit `960 x 1280`; buffer native saat READY `1280 x 960`, rotasi `270` derajat.

Contoh frame guidance **1495**, sebelum frame terminal:

| Besaran normalized preview | Teramati | Batas minimum |
|---|---:|---:|
| Lebar bbox | 0.56363 | 0.28700 |
| Tinggi bbox | 0.38438 | 0.24767 |

Frame itu memiliki `HOLD_STILL`, 15 sampel, dwell `663.73608 ms`; posisi bbox `(0.18560, 0.29688)`. ROI: pusat `(0.5, 0.5)`, radius `(0.41, 0.38103)`. Gate containment dan pose tetap berlaku. Nilai ini milik frame 1495, bukan diklaim sebagai bbox frame terminal 1512.

- Error corner round-trip view→sensor→view teramati `0.00003 dp`. Ini bukti konsistensi inverse, **bukan** kalibrasi skala absolut atau jaminan akurasi visual.
- Native READY melaporkan `mirrored=true`, rotation270 dan `nativeCropNormalized=(0,0,1,1)` untuk cropRect buffer input. CropRect input bukan klaim bahwa JPEG akhir memuat seluruh sensor/preview tanpa inset.
- Native plan/encoder/consumer menerima capture dan JPEG dikonsumsi. JPEG/pixel tidak dibuka di JS atau disimpan untuk inspeksi visual; verifikasi visual crop JPEG akhir **NOT_RUN**.
- Fixture memiliki marker L/R. Tidak ada klaim pengukuran visual marker/mirror absolut yang tidak direkam.

## D. Positive readiness evidence

Generation terminal: **1793029361952**. Frame terminal: **1512**.

`CAPTURE_READY` teramati dengan **1504.65792 ms dwell dan32 sampel**. Ketentuan tidak diubah: dwell≥1500ms, sampel≥6, gap≤400ms, tepat satu wajah, containment/ukuran/pose/drift valid. Frame guidance yang tersimpan memiliki gap sekitar43–56ms.

FRAME satu wajah + HOLD_STILL dan akumulasi sampel merupakan bukti detected/geometry-valid/stable sebelum READY. Prefix event literal DETECTED/GEOMETRY_VALID/STABLE tidak seluruhnya bertahan di ring logcat sampai snapshot akhir; jangan mengklaim arsip event prefix lengkap. Terminal chain di bawah tersimpan lengkap.

## E. Dwell dan same-frame evidence

Semua event berikut membawa generation **1793029361952** dan frame **1512**:

| Event | Bukti teramati |
|---|---|
| CAPTURE_READY | Dwell1504.65792ms,32 sampel |
| CAPTURE | Logical captures=1 |
| JPEG_READY |146991 byte; nativePhase READY; nativeError null |
| STALE_REJECTED | `E_CAPTURE_GENERATION` untuk generation−1 |
| CONSUMED | Captures=1, consumes=1 |
| DUPLICATE_REJECTED | `E_CAPTURE_HANDLE` setelah consume sah |
| RELEASED | Releases=1; nativePhase IDLE; ownedBytes0; heldFrames0; busy=false |

Tidak ada capture manual kedua, takePhoto terpisah, atau substitusi JPEG fixture pada jalur live. Tidak ada JPEG/base64/pixel yang diteruskan ke JS. UI menampilkan `Bingkai sudah diproses` / `CONSUMED · CONSUMED`, tetapi enrollment tetap **Belum terdaftar**, bukan ACTIVE atau Wajah terdaftar.

## F. Consume-once, stale dan duplicate

Probe debug-only memakai capability yang tetap private di `LiveCaptureClient`: wrong-generation consume sebelum consume sah, kemudian duplicate consume sesudahnya. Tidak ada capability/token dalam trace. Penolakan native teramati, bukan hanya penolakan facade JS.

Probe serial membuktikan stale replay dan post-consume replay; **bukan** live concurrent-race proof. Single-winner concurrency tetap dicakup assertion adapter JVM dan facade JS offline. Probe hanya efektif dengan debug runtime + mannequin mode + TRACE=1 + PROBES=1; default keduanya0.

## G. Buffer cleanup/privacy/storage

- Pada READY: ownedBytes `4194304`, heldFrames `0`, busy `true`; JPEG aktual `146991` byte. ownedBytes adalah reservasi adapter, bukan ukuran JPEG.
- Setelah consume: ownedBytes `0`, heldFrames `0`, busy `false`; peakOwnedBytes `5865928` adalah maksimum lifetime adapter, bukan retensi saat itu.
- RELEASED hanya untuk snapshot setelah consume yang masih membawa frame identity. Cancel memakai DRAINED terpisah dan tidak menggandakan counter release capture.
- Active Camera Clients sesudah consume/close kosong. Gunakan blok **Active Camera Clients**, bukan cached/historical client dumps dalam keluaran dumpsys.
- Inventory sandbox staging awal/akhir:32 berkas, tanpa image-named paths. Tidak ada JPEG kamera yang disimpan sebagai evidence, tidak ada gallery/upload/image file path.
- Database disposable: `face_enrollments=0`, `face_enrollment_submissions=0`, `face_enrollment_photos=0`, `face_templates=0`. Audit:2 request OTP,1 login sukses,1 consent.granted, seed identity/roster; tidak ada enrollment.submit.
- Batas bukti: bukan proof total memory zeroization, zero telemetry, atau ketiadaan semua temporary write platform/SDK.

## H. Lifecycle

| Skenario device R2 | Hasil dan batas |
|---|---|
| Close/Back sesudah capture | Kembali ke persiapan; kamera kosong; enrollment belum terdaftar |
| Back saat start | Start dibatalkan sebelum capture; DRAINED owned0/held0/busyfalse |
| Background setelah FRAME preview nyata | Host reader mengirim HOME setelah FRAME1513, generation1793029361957; DRAINED owned0/held0/busyfalse; tidak ada capture baru |
| Physical camera release setelah background | Snapshot awal masih mencatat client ketika teardown async berlangsung; setelah settle1500ms Active Camera Clients kosong |
| Resume | Kembali ke persiapan/Lanjut ke kamera; tidak auto-start, clients kosong |
| Cold restart/session restore | Setelah UI settle: Beranda pribadi / Halo Mahasiswa Sintetis; clients kosong; aplikasi kemudian force-stopped |
| Background tepat selama encoding | **NOT_RUN**; tidak mengklaim sampling pada fase ENCODING |
| Cancel tepat setelah selection saat codec belum selesai | **NOT_RUN**; cancel sesudah consume bukan pengganti uji ini |
| Settings revocation/TalkBack/font scale | Tidak diulang atau diubah pada R2; di luar perubahan UI diagnostik ini |

Native buffer acknowledgement dan physical CameraX disconnect adalah dua observasi terpisah. Release kamera tidak diklaim seketika hanya karena DRAINED sudah diterima.

## I. Android/device evidence

- Izin eksplisit **Izinkan pengujian R2** diperoleh sebelum SEMUA ADB, termasuk enumeration.
- Pemilik mengonfirmasi fixture browser sudah terlihat dan HP diarahkan hanya ke gambar mannequin sebelum start.
- APK staging: `ai.facelink.enr02host`,126329567 byte, SHA-256 `0ed8952eaa6468263033441aa8946add523bb88e2b19aca354cc9dbb7e751fd9`.
- APK terpasang sudah identik: install/update tidak diperlukan. CAMERA ada; storage/media/overlay tidak ada. Main package berbeda dan utuh.
- Debug runtime memerlukan Metro lokal + backend HTTPS disposable. Consent fixture melalui UI biasa sebelum kamera. Android one-time permission diberikan lewat dialog normal; scanner tidak auto-resume setelah dialog, sehingga dilakukan start eksplisit lagi.
- OTP pertama tidak terverifikasi karena fokus input/autofill; pemilik mengetuk kolom kode, lalu input lokal diverifikasi normal. Tidak ada OTP/token dalam chat/evidence dan tidak ada auth bypass.
- Sepuluh settings awal/akhir identik: font1.0, brightness3, mode0, timeout1800000, accelerometer1, rotation0, stay_on15, accessibility0, servicesnull, touch_exploration0. Tidak ada settings put/restore diam-diam.

## J. Regression/build evidence

| Pemeriksaan yang dijalankan | Hasil |
|---|---|
| Main mobile unit suite |147 PASS,1 opt-in E2E skipped |
| Mannequin staging TypeScript + lint | PASS sesudah perubahan trace final |
| Staging unit suite |147 PASS,1 skipped; diulang sesudah cleanup mode synthetic: sama |
| Trace privacy/rate-limit suite |5 PASS; unknown/payload fields dibuang, sampling dibatasi, terminal budget dipertahankan |
| Oracle helper guidance aktual via TypeScript AST, koordinat sintetis | TOO_FAR/HOLD_STILL/NO_FACE/MULTIPLE_FACES/YAW sesuai gate; bukan camera proof |
| Kotlin pixel boundary |35 assertions PASS |
| JVM adapter ownership |224 assertions PASS, controlled encoders; bukan Android codec |
| JS guidance + facade |31 tests PASS |
| Android ARM64 debug | **BUILD SUCCESSFUL7m7s**,656 tasks:37 executed/619 up-to-date; offline, max-workers=1 |
| HTTPS synthetic E2E | Tidak diulang R2: kontrak/backend tidak diubah; live UI memakai HTTPS disposable dan query audit/count faktual |

Build pertama R2 gagal membaca `PromiseType.hpp` pada Nitro Image dengan unspecified system_category error. File tersedia/terbaca sesudah kegagalan; build ulang worker tunggal berhasil. Penyebab kegagalan baca sementara belum diisolasi; bukan disembunyikan sebagai PASS run pertama dan tidak memerlukan perubahan dependency/source native.

Tidak ada regresi auth/video yang teramati pada startup/login/navigation/restart yang dijalankan; ini bukan ulangan seluruh A/B video/TalkBack R2 lama. Native codec live sukses pada siklus capture di atas, terpisah dari assertion encoder sintetis.

## K. Perubahan, berkas dan review

Source permanen:

- `research/fl-enr02a-host/templates/nativeScanner.tsx`: metadata trace opt-in, probe stale/duplicate native, READY accounting, error sebelum cancel, CONSUMED ordering dan release/drain terpisah; tidak mengubah threshold/guidance mapping.
- `research/fl-enr02a-host/templates/mannequinTrace.ts`: explicit whitelist, max90 FRAME (≤1/detik), max64 transition, sub-cap16 RESET; debug-only.
- `research/fl-enr02a-host/mannequinTrace.test.mjs`:5 pengujian perilaku privasi/pembatasan.
- `research/fl-enr02a-host/prepare-enr02a.cjs`: generate helper hanya mannequin; TRACE/PROBES default0 dan tidak efektif di synthetic.
- `research/fl-enr02a-host/README.md`: penggunaan flag, unit koordinat, batas bukti.
- Laporan ini, PROJECT_STATE, README utama, BACKLOG dan TEST_PLAN: status R2 faktual.

Dua scout read-only melakukan review terpisah: coordinate chain dan ownership/privacy. Temuan unit trace, READY retention yang semula tidak dicatat, error evidence yang hilang sebelum cancel, urutan receipt dan double-count release ditutup sebelum capture R2. Tidak ada source native/main/auth/schema yang diubah pada R2.

Evidence lokal ignored: `research/fl-enr02a-host/evidence-local/enr02b-r2/` berisi baseline, live-trace, lifecycle, device-final dan restart-settled. `device-final` memiliki snapshot restart awal belum settle; gunakan `restart-settled.json` untuk hasil UI restart yang lengkap. Tidak ada image kamera atau capability dalam evidence.

## L. Cleanup dan remaining limitations

- Kamera tertutup; staging tetap installed dan force-stopped; tidak uninstall atau clear SecureStore.
- Reverse8089/8091 dihapus; port8089/8090/8091/8769 kosong dan0 proses fixture backend.
- Backend menerima Ctrl+C; wrapper melaporkan exit512, tetapi root disposable benar-benar sudah dihapus dan tidak ada orphan process/port. Tidak mengklaim exit0.
- Metro, relay, fixture server dan log reader task berhenti. Tab verifikasi tool dilepas; browser desktop yang dibuka atas permintaan pemilik tidak ditutup. Halaman yang sudah dimuat dapat tetap terlihat; server fixture telah dihentikan sehingga refresh memerlukan server lagi.
- Staging di-prepare kembali ke `synthetic_flow`, TRACE=0, PROBES=0; typecheck/lint dan147 tes/1 skipped lulus. Main tetap default-OFF/tanpa CAMERA, route enrollment atau import helper diagnostik; `cmd/server` tidak menginjeksi enrollment.
- Remaining: full prefix event retention, verifikasi visual mirror/crop absolut, cancel/background tepat saat encoding, live concurrent race, revocation dan TalkBack scanner belum dibuktikan di R2. Tidak ada klaim PAD/liveness/recognition accuracy.

## M. GO/PARTIAL/NO-GO dan keputusan

**LIVE DEVICE PASS untuk positive readiness → same-frame JPEG → native consume-once → release.** Happy path R1 yang sebelumnya NOT_PROVEN kini memiliki bukti nyata.

**Keseluruhan PARTIAL** karena batas observabilitas dan lifecycle di §C/§D/§H/§L. Tidak ada keputusan produk yang diubah. Rekomendasi berikutnya: pemilik meninjau evidence dan residual sebelum menetapkan acceptance berikutnya; jika memerlukan closure lifecycle codec, buat pengujian terpisah dengan bukti fase ENCODING dan izin perangkat baru.

**NO-GO otomatis** untuk capture manusia, upload JPEG live, enrollment/template production, migrasi database Product Owner, Admin Enrollment UI implementation, FL-ENR03, ESP-CAM, SMTP/DNS, commit/push/merge atau PR19. **STOP setelah laporan R2.**

# FL-STAB02 — Android Native Crash Diagnosis & Lifecycle Closure

**Hasil akhir FL-STAB02: crash UNRESOLVED; Fast-Track PARTIAL.** Preflight offline dilanjutkan dengan izin eksplisit ADB/install/diagnosis pada TECNO LJ7. Pembanding release-like dan kedua debug staging diuji terbatas; tidak ada fatal baru pada crash buffer selama pengamatan, tetapi crash asli belum berhasil direproduksi dan tidak dinyatakan fixed. Tidak mengubah auth/AADv2/consent guards, dependency atau source aplikasi; tidak memulai FL-UI01.

## 1. Baseline dan preservation terbaru

Git tidak tersedia melalui PATH (`where.exe git`); executable portable ditemukan dan benar-benar dijalankan: `<short-build-workspace>'s/AppData/Local/Temp/facelink-enr04-tools-IwO0S9/git/cmd/git.exe`, versi `2.56.0.windows.4`.

Baseline FL-STAB02 tersimpan di `research/fl-enr02a-host/evidence-local/stab02/baseline.json`: branch `feat/fl-otp01-email-auth`, HEAD `87bac5e96602e9ff5568069d1c6b1bd1d96913b7`, local origin/main `95d31d8fa03d9bdaaee035da9a0bba6b06b4bc00`; 60 tracked modifications, 217 untracked files. Hash 438 existing tracked/untracked files dan APK utama/staging, video serta model dicatat sebelum perubahan. Ini verifikasi baru, bukan salinan baseline laporan sebelumnya. Tidak ada reset/stash/clean/rebase/pull/commit/push/merge atau penghapusan evidence lama.

Pemeriksaan akhir berbasis hash `device/final-verification.json`: kedelapan artefak protected (APK utama, tiga APK staging/backup existing, dua video dan dua model) tidak berubah. Dari438 file baseline, perubahan hanya empat dokumentasi status yang disengaja; tidak ada unintended drift. Laporan STAB02/evidence diagnostik baru terpisah. Tidak menjalankan ulang Git untuk mengklaim preservation; bukti akhir berasal dari pembandingan bytes/hash.

Canonical yang dibaca: README, PROJECT_STATE, PRD, DECISIONS, ARCHITECTURE, SECURITY_PRIVACY dan laporan FL-ENR06-R1. Scope tetap diagnosis stabilitas sintetis/lokal; kamera, biometrik manusia, production enrollment, ESP-CAM dan migrasi Product Owner dilarang.

## 2. Crash evidence dan root-cause assessment

Evidence asli dipertahankan:

- `research/fl-enr02a-host/evidence-local/enr06-r1/run4/native-crash.txt`: seluruh 15 baris stack Java yang tersimpan.
- `run4/metro.log`: pesan crash yang sama; log CI menyatakan reload watch otomatis disabled, bukan inspector disabled.
- `run4/http.jsonl`, screenshots dan `negative-and-runtime.json` untuk urutan pengujian.
- Analisis chronology baru: `evidence-local/stab02/archived-crash-analysis.json`.

Fakta: **Java `NullPointerException: java.lang.NullPointerException`, thread `main`, process `ai.facelink.enr02host`, PID30757**. Bukan bukti SIGSEGV atau crash Hermes JS. Stack masuk ke native method `CxxInspectorPackagerConnection$WebSocketDelegate.didReceiveMessage`, kemudian Kotlin callback `CxxInspectorPackagerConnection.kt:202`, Handler/Looper dan ActivityThread.

**Koreksi evidence setelah izin perangkat:** buffer crash lama ternyata masih tersedia. `device/facelink-crashes-before.txt` merekam onset fatal **2026-10-28T18:22:13.755Z**, PID30757, thread main, dengan seluruh stack Java yang sama. Timestamp ini berasal dari logcat epoch, bukan waktu simpan host. Native backtrace/tombstone serta event attach/failure/close/reset tepat sebelum crash tetap tidak tersedia. Screenshot Admin logout disimpan18:21:44.039Z dan route-denied18:21:46.540Z; kedekatan waktu tidak membuktikan logout menyebabkan crash pada process Student.

### Mekanisme yang didukung source/binary

- RN Kotlin `CxxInspectorPackagerConnection.kt:181–228`: listener OkHttp mem-post `onMessage/onOpen/onFailure/onClosed` ke main Handler. Terminal failure/closed memanggil delegate lalu `delegate.close()`; `WebSocketDelegate.close():64–65` memanggil `mHybridData.resetNative()`.
- JNI `JCxxInspectorPackagerConnectionWebSocketDelegate.cpp:37–41`: native delegate yang masih valid memakai `weak_ptr.lock()`. Target C++ yang sudah expired diabaikan; expired weak pointer sendiri tidak membuktikan penyebab NPE.
- fbjni0.7.0 dari AAR lokal: `prefab/modules/fbjni/include/fbjni/fbjni.cpp:215–223` melempar NPE dengan **pesan persis yang tersimpan** bila `mNativePointer==0`. `Hybrid.h:292–329` juga memiliki jalur NPE untuk hybrid field yang null; sebab itu satu pesan saja tidak membuktikan seluruh urutan.
- `javap -c` pada `HybridData` dan `HybridData$Destructor` dari AAR asli mengonfirmasi `resetNative()` memanggil destructor yang menghapus native object lalu mengisi pointer dengan0.
- [INFERENCE] akses callback setelah native hybrid di-reset adalah penjelasan kuat untuk boundary ini. Tetapi belum terbukti callback mana mendahului reset, apakah terminal callback beradu dengan message dari thread lain, atau apakah ada pemicu lain. **Belum ada reproduksi deterministik; tidak dilabeli FIXED.**

Tidak ada catch/suppression, perubahan `node_modules`, upgrade dependency atau patch native spekulatif.

## 3. Versi aktual dan dev-only versus app/runtime

| Komponen | Bukti aktual |
|---|---|
| React Native | npm installed0.88.0; cached debug/release AAR0.88.0 |
| Expo | installed58.0.13 |
| Expo dev client/dev launcher |58.0.14 |
| Metro | installed0.87.1 |
| Hermes | package/compiler260318099.0.4; compiler melaporkan Hermes1.0.0/HBC99. Compiler host berlabel DEBUG tidak menentukan variant APK |
| fbjni | RN dependency0.7.0; cached AAR header dan classes.jar diperiksa |
| Android Gradle Plugin |9.2.1, dibuktikan dari `META-INF/com/android/build/gradle/app-metadata.properties` pada APK debug crash yang diaudit |
| Build saat crash | `debug`, ARM64, debuggable, New Architecture dan Hermes aktif, Expo dev client/Metro; network inspector flag true |

Binary cached RN BuildConfig diperiksa dengan javap: debug DEBUG=true; release DEBUG=false dan `UNSTABLE_ENABLE_FUSEBOX_RELEASE=false`. Hasil disimpan dalam `react-buildconfig-binaries.json`.

Source gate release: `ExpoReactHostFactory.kt:97–138` memakai `ReactBuildConfig.DEBUG`; `DefaultDevSupportManagerFactory.kt:68–85` memilih ReleaseDevSupportManager saat release/devsupport false dan Fusebox release flag false; `ReleaseDevSupportManager.kt:60–62` tidak membuka/menutup inspector. Expo dev launcher `android/build.gradle:16,53–60` memakai `disableInRelease` kecuali configureInRelease secara eksplisit true. Tidak mengaktifkan opsi tersebut pada pembanding.

**Batas klasifikasi:** source/binary menempatkan frame crash pada inspector dev-support; pembanding release-like kini benar-benar berjalan standalone pada TECNO tanpa target inspector Metro. Kasus terbatasnya tidak menghasilkan fatal baru. Itu memperkuat pemisahan jalur tooling, **bukan** bukti akar masalah sudah terisolasi penuh, semua release bebas crash, atau aplikasi mustahil menjadi pemicu tidak langsung.

### Hipotesis dan eksperimen pembeda

| Hipotesis | Evidence offline dan status | Bukti perangkat yang diperlukan |
|---|---|---|
| H1 — race DevTools/Metro inspector | Callback/reset JNI memberi mekanisme yang masuk akal. Debugger frontend attach tidak diperlukan agar menerima pesan inspector. Root race belum direproduksi | Catat inspector device/debugger connection lifecycle serta crash saat attach/detach dibanding tanpa debugger frontend |
| H2 — lifecycle/callback aplikasi | Tidak ditemukan pemanggilan inspector/reload/ReactHost.destroy langsung dalam app/src, routes dan native host yang ditelusuri. Auth logout mengubah AuthFlow/route, preview fetch di-abort/unmount; video ownership guard existing tetap. Tidak ada frame FaceLink/video/capture dalam stack ini. Ini bukan bukti aplikasi mustahil menjadi pemicu tidak langsung | Bandingkan idle/login/logout/preview dengan tindakan host inspector yang sama; cari exception aplikasi sebelum fatal |
| H3 — debugging WebSocket + background/reload/disconnect | ReactHost pause menonaktifkan dev support; DevSupportManagerBase reload(false) menutup **packager connection**, bukan otomatis inspector connection. ReactHost teardown memanggil stopInspector. Jadi pause, packager disconnect, inspector close dan teardown tidak boleh disamakan | Run terbatas background/resume, reload dan host-only disconnect/reconnect dengan event timestamps; jangan mengubah Wi-Fi/setting HP |
| H4 — framework juga memengaruhi release | Release guards menonaktifkan jalur inspector pada konfigurasi standar. Tidak ada bukti runtime release pada crash lama | Bandingkan isolated release-like standalone, Metro tidak diperlukan; audit debugger endpoint tidak terhubung dan amati jalur lifecycle sama |

### Smoke offline konkret

Menjalankan **middleware inspector installed yang asli** pada HTTP loopback ephemeral dan satu host-only synthetic WebSocket peer, tanpa debugger frontend dan tanpa Android. Middleware mengirim `getPages` dua kali dengan debugger_connections0. Hasil `inspector-no-debugger.json`. Ini membuktikan pesan inspector otomatis tanpa debugger attach; **bukan reproduksi crash native atau pengganti pengujian Android**. Semua socket/server smoke ditutup.

## 4. Perubahan source dan build pembanding

Tidak ada perubahan production/app source, auth, consent, AADv2, AI atau dependencies. Hanya generated staging tree yang diabaikan Git disiapkan untuk pembanding:

- `android/app/build.gradle`: applicationId terpisah `ai.facelink.stab02release`; konfigurasi sebelumnya disalin ke evidence.
- `src/release/AndroidManifest.xml` dan resources release baru: label FaceLink STAB02 Release-like; TLS dengan CA sistem dan dev CA hanya pada domain loopback127.0.0.1/localhost; cleartext ditolak. Tidak memerlukan perubahan trust/setting global HP.
- Standard `release` variant, debug-keystore signing **untuk staging saja**, minification/R8 dimatikan agar diagnosis tidak tercampur dengan perubahan optimisasi. Ini **release-like**, bukan production-certified atau pembuktian variant R8 production.
- JS embedded di APK release; staging synthetic flags dan pinned fixture existing tetap; tidak menambahkan kamera. APK utama dan APK debug lama tidak ditimpa.

**Build pembanding selesai:** `:app:assembleRelease -PreactNativeArchitectures=arm64-v8a -Pandroid.enableMinifyInReleaseBuilds=false --offline --max-workers=1 --console=plain --no-daemon`, exit0, `BUILD SUCCESSFUL`,951tasks. APK disimpan sebagai `research/fl-enr02a-host/artifacts/stab02-release-like-70160d4f3c407f3117a3a8705e0e5bf9c69d1c92bfddc0606550a772612353be.apk`,49730987byte, SHA-256 `70160d4f3c407f3117a3a8705e0e5bf9c69d1c92bfddc0606550a772612353be`.
Audit `aapt` dan `apksigner` pada APK aktual: package `ai.facelink.stab02release`, non-debuggable, embedded `index.android.bundle`, Hermes ARM64, AGP9.2.1, signature valid dengan staging debug signerSHA256 `e0992e2706f8fbbe23cc7c8b846a833b5f61ac08bcd9f5ea930e2560689da68d`. Permissions: INTERNET, VIBRATE, ACCESS_NETWORK_STATE, WAKE_LOCK, USE_BIOMETRIC, USE_FINGERPRINT dan own DYNAMIC_RECEIVER_NOT_EXPORTED_PERMISSION. Tidak ada CAMERA/media/storage/overlay.
Manifest packaged cleartext=false; XML resource dioptimasi AAPT menjadi `res/U4.xml`, di-resolve melalui resource table sebelum diperiksa: dev CA hanya untuk127.0.0.1/localhost dan cleartext=false. Kegagalan lookup nama XML source bukan kegagalan TLS; audit diselesaikan memakai path resource actual.
Generated BuildConfig DEBUG=false, Hermes/New Architecture=true; ini dan APK audit sendiri **bukan** bukti runtime TECNO lulus. `debug-apk-audit.json` juga membuktikan kembali checksum/signature/permissions kedua APK debug existing sebelum ADB. Build logs, audit manifest/network, `release-apk-audit.json` dan `release-source-overlay.zip` disimpan di evidence STAB02. Setelah APK disalin, generated applicationId dikembalikan dan overlay diagnostik dilepas supaya tidak mengubah default build berikutnya. APK terisolasi kemudian dipasang sesudah izin perangkat baru, sebagaimana §5.

Build pertama terputus dari sesi tool sebelum menghasilkan APK; tidak ada hasil sukses yang dapat diklaim. Wrapper persisten berikutnya sempat gagal karena quoting cmd.exe dan dikoreksi, tanpa perubahan dependencies. Log kegagalan dipertahankan.

## 5. Hasil lifecycle Android dan acceptance tersisa

### Izin dan environment aktual

Product Owner memberi izin baru untuk ADB, identifikasi/inventaris, instalasi APK release-like yang diaudit, interaksi sintetis, screenshots/log tersanitasi, metadata cache dan cleanup runtime staging. TECNO LJ7/Android16, serial [perangkat uji disamarkan] diverifikasi sesudah izin. Tidak ada kamera, perubahan Wi-Fi/setting global, uninstall atau akses database Product Owner.

Evidence perangkat: `research/fl-enr02a-host/evidence-local/stab02/device/`. Host memakai existing `research/fl-enr06r1-host/session.cjs` dengan run label `stab02-native2`; evidence host terpisah di `evidence-local/enr06-r1/stab02-native2/`. Go HTTPS8095, verified HTTPS relay8094 dan Metro8083 hanya loopback, diteruskan melalui reverse8083/8094. Release-like memakai embedded JS dan tidak memerlukan Metro. Metro pernah di-restart pada PID yang diverifikasi milik tugas; PID pengganti juga ditutup saat selesai.

Satu fixture disposable minimal diperlukan untuk membuktikan offline ACTIVE dan preview lifecycle pada pembanding. Ini bukan pengulangan seluruh FL-ENR06-R1. Login OTP normal, WAL/schema8 disposable dan real YuNet/SFace existing dipakai tanpa SQL activation shortcut:

- Trace `<redacted-id>`; submission `<redacted-id>`.
- Release-like Student → debug Admin preview/approval202 → worker nyata → ACTIVE/AADv2 → release-like Student offline-unverified → withdrawal/REVOKED.
- Satu processing call, satu wajah, embedding128D; template envelope2/binding valid. Canonical SHA `00eaa7792b75bd3f7f2ad10bfcbbb5fd3afe64fec4b626761a0e42b239cace76` sama untuk submission, preview dan input worker.
- Setelah reconcile: photo0/template0/active0/ciphertext-file0. Satu audit approval meskipun native transport mengulang request.

### Kasus yang benar-benar dijalankan

| Kasus | Hasil dan batas |
|---|---|
| Release-like standalone | Login OTP dan dashboard berjalan; tidak muncul sebagai target `/json/list` Metro |
| Release-like background/resume dan cold restore | Sesi tersimpan pulih; setelah logout lalu force-stop/cold launch, tetap layar login |
| Debug tanpa debugger frontend | Inspector device target tetap tersedia; background/resume terbatas dan native Reload dijalankan |
| Frontend attach/detach | Handshake WebSocket dengan origin sah berhasil. Node client hanya merekam `open`; tidak mendapat respons `Runtime.enable`. **Bukan bukti sesi DevTools/CDP lengkap berhasil** |
| Disconnect | Melepas reverse8083 saja tidak memutus socket existing; tidak dihitung sebagai disconnect. Metro milik tugas benar-benar dihentikan, lalu dihidupkan kembali dan debug app dibuka lewat dev client; UI pulih. Salah satu perintah ADB timed out, dicatat sebagai kegagalan tooling |
| Debug Admin cold restore | Sempat berhenti pada launcher Expo; memilih server existing memulihkan dashboard sesi, bukan login ulang. Preview tidak otomatis terbuka kembali |
| Preview background/offline | Preview ditutup setelah background/resume serta setelah relay backend offline; Metro tetap tersedia pada eksperimen offline API |
| ACTIVE offline | Release-like menampilkan “Status aktif belum terverifikasi”, bukan mempercayai cached ACTIVE |
| Package debug Student asli | `ai.facelink.enr02host` login OTP, background saat Admin logout, resume lalu native Reload; sesi pulih. Target inspector Student dan Admin memiliki ID berbeda dalam pengamatan ini |
| Logout/revocation | Release-like dan kedua debug memakai logout normal; Student withdrawal benar-benar REVOKED/cleanup0. Preview tidak tersedia setelah revocation |
| Approval response-loss | Upstream202 dibuang relay; native otomatis melakukan identical replay200, body/key/ETag sama, audit approval tetap satu |
| Uncertainty/manual replay UI | **NOT_DEMONSTRATED**: retry native menutup window uncertainty. Tidak mengklaim klik replay manual atau uncertainty UI lulus; tidak menambah fault mode atau mengubah client demi memaksakan hasil |
| Crash sesudah kasus terbatas | Satu fatal historis sebelum dan sesudah; **0 fatal baru pada buffer yang diamati**. Crash asli tidak terulang, bukan bukti fixed/zero-crash |

Action timestamps, snapshot metadata, aktivitas Android tersaring, crash buffer sebelum/sesudah dan screenshot terkait disimpan di `device/`; ringkasan `lifecycle-result.json`. Percobaan handshake awal ditolak401 karena origin tidak sah; diperbaiki memakai origin localhost. Pemanggilan Node awal gagal me-resolve `ws`, kemudian memakai working directory stage; tidak dihitung sebagai native crash.

Run host pertama sempat terhenti bersama eval/tool timeout sebelum fixture lifecycle selesai. Itu kegagalan orchestration, bukan crash aplikasi. Environment baru `stab02-native2` dipakai untuk hasil di atas; hasil antar-run tidak digabung sebagai satu positive chain.

Cache isolation terbatas pada preview unmount/authorization dan **nama file** di files/cache kedua debug staging: tidak ditemukan nama berakhiran JPEG/PNG/WebP/HEIC. Tidak membuktikan contents cache, OS snapshot, native decoder memory zeroization atau forensic erase. Private data release-like dan kedua prototype non-debuggable tidak diekspor/diperiksa.

### Shutdown teramati

Ketiga package yang diuji di-force-stop, tanpa menyentuh main/prototype. Hanya reverse8083/8094 milik tugas dilepas. Existing host menerima shutdown melalui stdin control, Go exit0 dan disposable root dihapus; Metro pengganti milik tugas ditutup. Port8083/8094/8095 tertutup, reverse list kosong. Bukti `device/shutdown.json`. APK, log, screenshot dan evidence tetap disimpan; **tidak ada uninstall**.

## 6. Regression relevan

`npx vitest run tests/authFlow.test.ts tests/adminEnrollmentFlow.test.ts tests/enrollmentFlow.test.ts` dijalankan: **85/85 PASS, tiga file PASS**. Ini controller regression, bukan lifecycle Android rendering/native proof. Tidak mengulang seluruh backend atau seluruh enrollment E2E. Build/typecheck/lint hanya diperlukan sesuai source yang terdampak; tidak ada perubahan TypeScript pada FL-STAB02.

Review paralel specialist tidak menghasilkan review: kedua child berhenti tanpa temuan. Diagnosis dilanjutkan inline; tidak mengklaim independent review PASS yang tidak terjadi.

## 7. Inventaris APK dan kandidat cleanup

Inventaris aktual sebelum pengujian menemukan lima package; sesudah instalasi pembanding menjadi **enam**. Semua versionCode1. `device/packages-before.json` dan `packages-final.json` mencatat install/update time, hash APK terpasang, debuggability dan batas inspeksi data.

| Package | Versi / klasifikasi | Kandidat cleanup dan dampak |
|---|---|---|
| `ai.facelink.mobile` |0.1.0; aplikasi utama, debug existing | **WAJIB DIPERTAHANKAN.** Tidak diluncurkan, ditimpa, di-force-stop atau dibaca private datanya |
| `ai.facelink.enr02host` |0.1.0; Student debug existing, package crash asli | Kandidat setelah diagnosis/retensi selesai. Uninstall menghapus preferences, sesi/cache lokal dan kemungkinan evidence privat; logout bukan bukti semua data lokal sudah hilang |
| `ai.facelink.enr06admin` |0.1.0; Admin debug existing | Kandidat setelah diagnosis/retensi selesai; dampak sesi/preferences/cache sama |
| `ai.facelink.stab02release` |0.1.0; pembanding release-like non-debuggable baru | Kandidat setelah pembanding tidak diperlukan; uninstall membuang state lokalnya. Private data belum diinspeksi |
| `ai.facelink.cam03host` |0.1.0; prototype host lama non-debuggable | Kandidat evaluasi build lama, **bukan izin hapus**. Tidak diluncurkan; data lokal/evidence belum diperiksa atau dibackup |
| `ai.facelink.cam00proof` |0.0.1; prototype proof lama non-debuggable; CAMERA dideklarasikan | Kandidat evaluasi build lama. Tidak diluncurkan dan kamera tidak dibuka; isi data lokal/evidence belum diperiksa atau dibackup |

Hash main terpasang tetap `f4cc2eae08fc95681960e9fbf33808e382125961d6a4304c7b1d2cc60967bf8b`, sama dengan sebelum pengujian. Hash debug Student `d9a32e5067dba37a9e37668fee45f3a048c54b0c2c7763feef434851ac359967`, Admin `67d69b92a84d5462e0acfbd47ba99cf35ea412b69612e8d2411bddcc1686487a`, release-like `70160d4f3c407f3117a3a8705e0e5bf9c69d1c92bfddc0606550a772612353be` cocok dengan artefak yang diaudit. Hash prototype juga disimpan pada inventaris JSON.

**Tidak uninstall/clear-data apa pun dalam FL-STAB02.** Kelima non-main hanya kandidat; sebelum cleanup, Product Owner perlu menerima daftar package yang dipilih, dampak penghapusan data privat dan keputusan retensi/backup, lalu memberi persetujuan eksplisit pada tugas tersendiri. APK/evidence host bukan otomatis backup private data Android.

## 8. Status crash dan verdict Fast-Track

**Crash: UNRESOLVED. Fast-Track: PARTIAL; bukan FIXED atau GO tanpa syarat.**

- Boundary JNI/native-hybrid yang nol dipersempit; pesan inspector tanpa debugger frontend terbukti. Timestamp fatal lama berhasil dipulihkan dari perangkat.
- Source/binary gates dan runtime standalone mendukung pemisahan jalur dev inspector dari pembanding release-like. Tidak ada fatal baru pada bounded debug/release-like cases. **Urutan callback/reset pemicu asli belum terbukti**, dan attach frontend belum menunjukkan round-trip CDP lengkap.
- Tidak ada root cause terbukti yang membenarkan patch; tidak melakukan upgrade, patch node_modules, suppress NPE, atau perubahan auth/video/AI spekulatif.
- Preview/session/offline trust dan withdrawal pada kasus yang tercatat berjalan. Uncertainty/manual replay UI tetap belum terdemonstrasi; keterbatasan native cache/decoder tetap terbuka.
- GO terbatas untuk meninjau bukti dan merencanakan pekerjaan terpisah. **NO-GO untuk menyatakan blocker crash selesai, seluruh Android lifecycle lulus, APK production-ready, human enrollment atau FL-UI01 otomatis.**

Rekomendasi berikutnya: tugas diagnosis terarah dengan ring buffer timestamp untuk callback inspector message/failure/closed/reset dan lifecycle ReactHost, serta sesi frontend CDP yang benar-benar merespons, sebelum memilih fix upstream/minimal. Instrumen native atau rebuild/install berikut memerlukan scope dan izin perangkat baru; jangan memperluas telemetry ke token/heap/biometrik.

**STOP setelah laporan FL-STAB02.** Runtime task sudah ditutup. Tidak ada commit/push/merge, uninstall atau tahap baru.

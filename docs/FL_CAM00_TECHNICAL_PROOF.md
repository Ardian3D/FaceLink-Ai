# FL-CAM00 — Isolated camera technical proof

Tanggal: 25 Oktober 2026. **Hasil: kelayakan dasar kandidat terbukti pada TECNO LJ7; NO-GO integrasi kamera V1/enrollment saat ini.** Ini hasil eksperimen, bukan persetujuan P01–P12, bukti akurasi recognition, atau klaim liveness/PAD.

> Lanjutan: [FL-CAM01 — laporan acceptance A–M](FL_CAM01_TECHNICAL_PROOF.md) mencatat release R2 standalone, live mannequin 0/1/2 wajah, portrait/guidance, audit storage/telemetry dan rekonsiliasi pengaturan. CAM00 di bawah tetap catatan historis; gate integrasi utama/capture manusia masih NO-GO.

## A. Dependency dan versi

Proof: `research/fl-cam00-proof`, terpisah dari `apps/mobile`. Versi terpasang (`npm ls --depth=0`):

| Komponen | Versi |
|---|---|
| Expo | 58.0.19 (SDK 58; aplikasi utama sebelumnya 58.0.13) |
| React Native / React | 0.88.0 / 19.3.0 |
| VisionCamera | 5.2.3 |
| Face detector plugin | 2.1.0 |
| Nitro Modules / Nitro Image | 0.37.1 / 0.15.2 |
| TypeScript | 6.0.3 |
| Bundled ML Kit face detection | 16.1.7, dependency native plugin |

VisionCamera, plugin, Nitro, Expo, React dan RN memakai lisensi MIT menurut manifest paket yang diperiksa. ML Kit tunduk pada terms Google, bukan disimpulkan berlisensi MIT. Tidak memakai API berbayar, trial, Google Cloud, atau API key. Android native memakai CameraX; plugin mendeklarasikan `camera-core:1.7.0-alpha01`—perlu perhatian saat pinning dependency integrasi.

## B. Android build

Build debug ARM64 dan x86_64 berhasil pada sesi ini menggunakan Gradle 9.4.1, JDK 17, NDK 27.1.12297006, compile SDK 37, target SDK 36, min SDK 24. Upaya awal gagal karena path panjang Ninja/CMake; konfigurasi CMake 3.31.6 melalui `android/local.properties` menyelesaikan build. Jangan menerapkan perubahan toolchain ini ke aplikasi utama secara otomatis.

Perintah build yang berhasil, dari direktori `research/fl-cam00-proof/android`:

```powershell
cmd.exe /c "gradlew.bat assembleDebug -PreactNativeArchitectures=arm64-v8a --console=plain"
cmd.exe /c "gradlew.bat assembleDebug -PreactNativeArchitectures=x86_64 --console=plain"
```

`npm run typecheck` final: exit 0. APK debug memuat JS dari Metro, bukan standalone/offline release; cold start tanpa Metro **belum dibuktikan**. Build release tidak dijalankan.

## C. Identitas APK dan ABI

Application ID: **`ai.facelink.cam00proof`**, bukan `ai.facelink.mobile`. ARM64 APK lokal: `research/fl-cam00-proof/cam00-arm64-debug.apk`, SHA-256:

```text
5fcecf9af04945769217f31019c4affe8eaf1251bb451b295f92c0c3c3dadf35
```

APK tersebut berisi ARM64 saja. Bundle JS final dilayani Metro; checksum APK tidak mengikat versi JS development yang dilayani kemudian.

Verifikasi 16 KB:

- `zipalign -c -P 16 4 cam00-arm64-debug.apk`: exit 0.
- Audit program header ELF seluruh **23 library / 69 segmen PT_LOAD**: tidak ada alignment di bawah 16384 dan tidak ada ketidakcocokan offset/virtual-address modulo 16384.
- TECNO memakai page size **4096**. Ini bukti packaging/ELF, **bukan runtime pada OS ber-page-size 16 KB**.

## D. Preview kamera

TECNO LJ7, Android 16/API 36, ARM64: aplikasi terpasang dan MainActivity berjalan. Cold launch terakhir melalui `am start -W`: Status ok, TotalTime 815 ms / WaitTime 841 ms; ini waktu ActivityManager, bukan latensi kamera-ke-deteksi. Kamera depan ID `1`; preview nyata nonblank terlihat dengan oval pada objek nonwajah. Kamera awal sekarang mati sampai operator menekan Mulai kamera.

Izin diuji melalui dialog sistem Android berbahasa Indonesia, dipilih **Hanya kali ini**. Deteksi file dummy bisa dijalankan tanpa izin kamera. Kamera fisik baru diaktifkan setelah pemilik menyatakan arahnya aman, tanpa wajah nyata. Tidak ada foto mahasiswa atau capture foto/video yang dibuat oleh aplikasi proof.

## E. Deteksi wajah dengan dummy

Input lokal: foto kepala mannequin, **bukan orang nyata**, `CosSchlHead2on112010` oleh CAMiller62, CC BY-SA 3.0, Wikimedia Commons:

https://commons.wikimedia.org/wiki/File:CosSchlHead2on112010.jpg

Checksum fixture lokal: `3d3d2f4978058389b8b8197b938dac67d476ae7d3af1aaaa233584a209e9ec089`.

| Lingkungan | Hasil deteksi file | Waktu satu pemanggilan |
|---|---|---|
| Emulator API 36 x86_64 | 1 wajah | 2213 ms |
| TECNO LJ7 ARM64 | 1 wajah | 317 ms |

Pose dummy miring melampaui ambang guidance; tidak memicu auto-capture dari hasil file tersebut. Preview objek nonwajah menghasilkan `NO_FACE`. **Live deteksi dummy di depan lensa belum diuji**; hasil file bukan bukti transform bounding box/pose live yang benar.

Fixture disediakan sementara di `files/dummy/mannequin.jpg` lewat ADB/run-as, bukan asset wajah orang di APK atau upload. Salinan fixture pada perangkat dan staging `/data/local/tmp` dibersihkan setelah pengujian. Untuk mengulang tombol Uji dummy lokal, fixture harus disediakan kembali; tanpa fixture tombol menghasilkan error file, bukan hasil deteksi palsu.

## F. Guidance overlay dan auto-capture

Oval, pesan NO_FACE, metrik pose, dan kontrol kamera terlihat di perangkat. Simulasi stabil menghasilkan event native-app log berikut pada TECNO:

```text
07:08:05.556 AUTOCAPTURE_SIMULATION_START source=manual_simulation_event
07:08:07.451 AUTOCAPTURE_SIMULATED stableFrames=6 storedImage=false uploaded=false
```

Ini **simulasi event enam langkah timer**, bukan enam frame kamera tervalidasi, shutter foto, encoding JPEG, atau pemilihan frame nyata. Tidak boleh dipakai sebagai bukti liveness ataupun end-to-end capture memory-only. Tidak ada YuNet/SFace, embedding, enrollment, atau panggilan backend.

Rotasi landscape → portrait mempertahankan preview, tetapi screenshot landscape menunjukkan panel header/guidance bertumpuk dan oval tertutup sebagian. **Uji layout landscape gagal**; tidak disembunyikan sebagai PASS. Mirror diatur `auto`, tetapi kebenaran mirror/orientasi koordinat belum dibuktikan dengan target asimetris.

## G. Native memory dan cache

Source native plugin memakai `InputImage.fromMediaImage`, single-flight `isBusy`, `STRATEGY_KEEP_ONLY_LATEST`, serta menutup ImageProxy pada completion/error/drop. Tidak menyalin piksel frame ke JS pada jalur yang dipakai. Ini audit source, bukan bukti semua jalur SDK tidak menulis disk.

Inventaris privat TECNO setelah uji berisi fixture dummy yang sengaja disediakan, dev JS bundle, preferences, cache HTTP journal, satu PNG cache **187 byte, 28×28**, serta database `com.google.android.datatransport.events`. Tidak ditemukan output foto/video kamera dalam inventaris tersebut. PNG kecil itu bukan frame kamera; asal persis icon belum ditentukan. Tidak membaca payload database telemetry.

**Tidak mengklaim memory-only end-to-end**: tidak ada encoder/upload/preview Admin yang diimplementasikan atau diuji; inventaris bukan forensic disk proof. Screenshot diagnostik dibuat lewat ADB selama verifikasi visual; bukan photo capture aplikasi. Screenshot fisik sementara dibersihkan sesudah dicatat.

## H. Lifecycle kamera

Transisi TECNO yang direkam langsung:

| Event | Waktu lokal logcat |
|---|---|
| App background | 07:08:08.283 |
| Preview stopped | 07:08:08.637 |
| App active | 07:08:12.337 |
| Preview started | 07:08:12.744 |
| Operator stop requested | 07:08:15.482 |
| Preview stopped | 07:08:15.528 |

`dumpsys media.camera` saat background menunjukkan Device 0 dan 1 **closed, no client instance**. Kembali foreground memulihkan preview. Setelah uji final proof di-force-stop, reverse TCP proof dilepas, dan Metro proof dihentikan. Paket utama tidak di-install ulang atau diubah. Runtime permission tidak sama dengan kepemilikan resource kamera; tidak mengklaim izin OS dicabut setiap background.

## I. Performa terukur

- Deteksi file dummy TECNO: satu observasi 317 ms, bukan distribusi benchmark.
- Snapshot terakhir preview TECNO: **25 callback deteksi/detik, 1145 callback total**. Bukan rendering FPS layar atau throughput dengan wajah live. Penghitung total awal membaca mutable ref setelah reset dalam updater React; diperbaiki dengan snapshot nilai lokal dan dibuktikan di surface final.
- PSS TECNO saat preview: **323979 KiB**, RSS **397263 KiB**, swap PSS **91432 KiB**. Satu snapshot, **bukan bukti tidak ada leak**.
- Emulator software rendering pernah crash; hasil jank/FPS emulator tidak digeneralisasi ke TECNO. Tidak ada native crash yang teramati pada smoke TECNO ini; soak/thermal/leak test belum dilakukan.

## J. Temuan keamanan dan kompatibilitas

1. Hook plugin 2.1.0 membuat rest-options object baru setiap render, lalu memakai `[options]` untuk useMemo. Pada proof ini menyebabkan output berubah dan CameraX stop/start berulang. Workaround lokal: `createFaceDetectorOutput` dalam useMemo sekali, callback melalui ref, serta array outputs stabil. Log setelah workaround membuktikan preview tidak lagi churn saat state berubah.
2. Bundled ML Kit **bukan jaminan tanpa telemetry**. Database DataTransport ada di perangkat; packet capture/atribusi trafik belum dilakukan. Dokumentasi resmi menyatakan metrik performa/penggunaan dapat dikirim. Tidak mengklaim nol komunikasi Google:
   - https://developers.google.com/ml-kit/terms
   - https://developers.google.com/ml-kit/android-data-disclosure
3. APK debug masih mendeklarasikan `SYSTEM_ALERT_WINDOW` dari manifest debug walaupun Expo config memblokirnya pada main manifest. Ada warning SafeAreaView deprecated dan overlay debugger yang sempat menghalangi tombol. Release permission audit belum dilakukan; bukan APK production.
4. Batas analisis native terbukti dari source; timeout/hang native dan resource leak jangka panjang belum diuji.
5. Sesi awal sebelum resume pernah mengubah brightness, timeout layar, stay-awake dan mencoba pengaturan lock screen pada perangkat. Nilai awal seluruh pengaturan itu tidak dicatat, sehingga tidak diklaim dipulihkan. Resume ini tidak meminta PIN atau menonaktifkan pengamanan; rotasi yang diubah pada resume dipulihkan ke nilai yang dibaca sebelum uji. Perubahan setting sesi awal perlu ditinjau pemilik.

## K. Kompatibilitas Expo 58

Native debug build, loading modul Nitro/VisionCamera/plugin, UI RN, deteksi file dan live analysis nonwajah terbukti pada Expo 58.0.19 + RN 0.88.0. Ini bukan Expo Go: aplikasi native development terpisah diperlukan. Tidak membuktikan semua dependency aplikasi utama kompatibel jika digabungkan; aplikasi utama tidak diubah. Perbedaan patch Expo harus diputuskan saat tugas integrasi, bukan dipindahkan diam-diam.

## L. Rekomendasi stack

Pertahankan **VisionCamera 5.2.3 + plugin 2.1.0 + bundled ML Kit** sebagai kandidat teknis, dengan output stabil seperti proof. Alternatif native Expo/CameraX tidak dibangun karena kandidat utama sudah berhasil pada target dasar. Tidak mengganti stack aplikasi utama, tidak menganggap plugin siap produksi, dan tidak menganggap model deteksi sebagai recognition/PAD.

Reproduksi lokal dari source proof: `npm ci`, `npx expo prebuild --platform android --no-install`, konfigurasi CMake 3.31.6 lokal, lalu build seperti §B. Android/node_modules/build/fixture/screenshot/APK diabaikan Git. Jalankan Metro localhost pada port bebas, hubungkan ADB reverse hanya untuk package proof, instal APK proof, lalu arahkan kamera ke objek nonpersonal sebelum memberi izin. Jangan menyalin kredensial/token aplikasi utama. Fixture lokal diperlukan hanya untuk tombol deteksi file; lisensi/atribusi harus dipertahankan jika didistribusikan.

## M. GO / NO-GO

**GO kelayakan dasar kandidat pada TECNO LJ7:** build ARM64, native module load, permission, preview nonblank, oval, deteksi dummy file, live NO_FACE, simulasi auto-capture, stop/background/resume dan alignment statis terbukti.

**NO-GO integrasi kamera V1 / enrollment production:** layout landscape masih gagal; live dummy dan koordinat mirror/crop/oval belum tervalidasi; standalone cold start/release permission, memory-only frame→JPEG, cache/telemetry end-to-end, runtime 16 KB, timeout/leak/soak dan kontrak submission/consent/crypto belum terbukti. P01–P12 tetap belum disahkan; tidak ada perubahan database, OTP/session, role, UI utama, SMTP/DNS, video/backup, commit/push/merge atau PR #19.

Rekomendasi tugas berikutnya: tugas proof lanjutan yang secara eksplisit menutup gap kamera/privasi di atas sebelum integrasi. Product Owner tetap perlu menyetujui keputusan enrollment secara terpisah. STOP; tidak memulai implementasi enrollment.

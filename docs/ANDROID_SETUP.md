# Setup Build Android (FL-ANDROID01)

Panduan menyiapkan JDK 17, Android SDK, ADB, Expo development build, dan Google
OAuth Android untuk FaceLink AI di **Windows**.

> **Status dokumen ini:** seluruh angka versi di bawah **dibaca dari berkas yang
> benar-benar ada di repository ini**, yaitu hasil `npx expo prebuild
> --platform android` atas `apps/mobile` (Expo SDK 58.0.13, React Native 0.88.0).
> **Toolchain sudah dipasang dan build sudah dibuktikan pada 20 Oktober 2026**
> dengan izin pemilik: `assembleDebug` menghasilkan APK arm64 tersigning debug
> (`ai.facelink.mobile`, compileSdk 37, targetSdk 36). Bagian §2–§4, §7, dan §8
> memuat langkah yang benar-benar dipakai, termasuk dua kendala nyata saat build
> (nama paket platform dan batas jalur ninja, §3c).
>
> **Yang belum terverifikasi:** Google Sign-In dan seluruh skenario autentikasi
> §8 — kredensial OAuth belum ada. **Yang sudah terverifikasi (21 Oktober
> 2026):** APK terpasang dan berjalan di HP fisik TECNO LJ7 (Android 16,
> arm64-v8a); layar masuk dirender native, latar video aktif, dan diagnostik
> aplikasi menjangkau Go Backend lewat `adb reverse`. Rincian dan batasannya di
> **§12**.
>
> Seluruh pemasangan dijalankan manual oleh pemilik atau atas izinnya; tidak ada
> skrip di repository ini yang mengunduh atau memasang paket.

## 1. Persyaratan yang dituntut repository ini

Sumber setiap angka tercantum agar dapat diperiksa ulang.

| Komponen | Versi | Sumber |
|---|---|---|
| JDK | **17** (minimum dan default) | AGP 9.2 compatibility table; `android/gradle/wrapper` memakai Gradle 9.4.1 yang menuntut JVM 17+ |
| Gradle | **9.4.1** | `apps/mobile/android/gradle/wrapper/gradle-wrapper.properties` |
| Android Gradle Plugin | **9.2.1** | `node_modules/@react-native/gradle-plugin/gradle/libs.versions.toml` |
| SDK Build Tools | **37.0.0** (minimum 36.0.0) | `ExpoRootProjectPlugin.kt` (`defineDefaultProperties`) |
| `compileSdk` | **37** | idem |
| `targetSdk` | **36** | idem |
| `minSdk` | **24** | idem |
| NDK | **27.1.12297006** | idem; dipakai `react-native-nitro-modules` dan `react-native-nitro-google-signin` (C++/CMake) |
| CMake | **3.22.1** — `[INFERENCE]` | `android/app/build.gradle` tidak menyetel `externalNativeBuild`; modul Nitro memakai `cmake {}` sehingga versi ditentukan AGP. Skrip CMake project meminta minimum 3.13 (`ReactNative-application.cmake`). 3.22.1 adalah versi bawaan AGP yang lazim; **belum diverifikasi** pada mesin ini karena cache Gradle masih kosong. Bila `sdkmanager` menolak 3.22.1, pasang versi yang diminta pesan galat Gradle |
| Kotlin Gradle Plugin | 2.2.10 (dibawa AGP) | AGP 9 built-in Kotlin |
| Package Android | `ai.facelink.mobile` | `apps/mobile/app.json` → `android.package` |
| Ruang disk | ≥ 15 GB | perkiraan SDK + NDK + build pertama |

Berkas `apps/mobile/scripts/check-native-env.mjs` memeriksa semua ini pada mesin
nyata dan mencetak PASSED / FAILED / BLOCKED. Jalankan lebih dahulu, dan lagi
setelah setiap langkah di bawah. Skrip ini hanya membaca keadaan: tidak
memasang, tidak mengunduh, dan tidak menulis berkas apa pun. Client ID dibaca
dari environment shell **atau** `apps/mobile/.env` (Expo memuat `.env` otomatis,
jadi nilai biasanya hanya ada di sana).

```bash
cd apps/mobile
npm run check:native
```

### Status mesin pengembangan (diperbarui 21 Oktober 2026)

Keluaran nyata `npm run check:native`. **Sebelum** instalasi: 3 PASSED, 0 FAILED,
11 BLOCKED. **Sesudah** instalasi toolchain: **11 PASSED, 0 FAILED, 3 BLOCKED**.

| Butir | Sebelum | Sesudah | Keterangan |
|---|---|---|---|
| JDK 17 | BLOCKED | **PASSED** | Temurin 17.0.20.1 di `%LOCALAPPDATA%\facelink-toolchain` |
| `JAVA_HOME` | BLOCKED | **PASSED** | lingkup User, tanpa administrator |
| `ANDROID_HOME` | BLOCKED | **PASSED** | `%LOCALAPPDATA%\Android\Sdk` |
| platform `android-37` | BLOCKED | **PASSED** | folder nyata bernama `android-37.0` |
| build-tools `37.0.0` | BLOCKED | **PASSED** | |
| CMake | BLOCKED | **PASSED** | 3.22.1 dan 3.31.6 terpasang |
| `NDK 27.1.12297006` | BLOCKED | **PASSED** | |
| `adb` | BLOCKED | **PASSED** | Platform-Tools 37.0.1 |
| perangkat terhubung | BLOCKED | **PASSED** (21 Okt 2026) | `adb devices` → TECNO LJ7 berstatus `device`; sekarang **tidak tersambung** karena pengujian dihentikan (§12) |
| AVD emulator | BLOCKED | INFO | emulator tidak dipasang (target HP fisik); kelayakannya diaudit di §13 |
| `EXPO_PUBLIC_FACELINK_GOOGLE_WEB_CLIENT_ID` | BLOCKED | **BLOCKED** | menunggu OAuth client dari pemilik (§5) |
| `android/` hasil prebuild | PASSED | **PASSED** | diabaikan Git |
| keystore debug | PASSED | **PASSED** | SHA-1 sudah diambil (§5) |
| ninja (jalur panjang) | — | **PASSED** | ninja 1.12.1 dari CMake 3.31.6 |
| ruang disk | PASSED | **PASSED** | 223 GB bebas |

**Satu BLOCKED yang tersisa bukan cacat pemasangan:** kredensial OAuth menunggu
tindakan pemilik (§5). Perangkat fisik sudah terhubung dan diuji pada 21 Oktober
2026 (§12).

**Yang sudah terbukti:** `assembleDebug` berhasil dan APK tersigning; APK itu
**terpasang dan berjalan** di HP fisik, dan `check:native` mencapai
**13 PASSED, 0 FAILED, 1 BLOCKED** saat perangkat tersambung.
**Yang belum terbukti:** Google Sign-In end-to-end dan skenario A1–A12 (§8).

## 2. JDK 17

**Jalur yang dipakai dan terbukti (tanpa administrator).** Unduh arsip Temurin 17
dari API Adoptium lalu ekstrak ke folder pengguna — tidak ada installer, tidak
ada entri sistem, dan penghapusan cukup dengan menghapus foldernya:

```powershell
$dest = "$env:LOCALAPPDATA\facelink-toolchain\jdk"
New-Item -ItemType Directory -Force $dest | Out-Null
curl.exe -sSL -o "$env:TEMP\jdk17.zip" `
  "https://api.adoptium.net/v3/binary/latest/17/ga/windows/x64/jdk/hotspot/normal/eclipse?project=jdk"
tar -xf "$env:TEMP\jdk17.zip" -C $dest
Get-ChildItem $dest   # jdk-17.0.20.1+1
```

`tar` dipakai, bukan `unzip`: `unzip` gagal pada jalur yang memuat apostrof
(pada nama profil Windows), sedangkan `tar` (bawaan Windows 10+) menerimanya.

Setel `JAVA_HOME` pada lingkup **User** (tidak menuntut administrator), lalu buka
terminal baru:

```powershell
[Environment]::SetEnvironmentVariable('JAVA_HOME',
  "$env:LOCALAPPDATA\facelink-toolchain\jdk\jdk-17.0.20.1+1", 'User')
```

**Alternatif installer** (juga tanpa biaya):
`winget install --id EclipseAdoptium.Temurin.17.JDK` atau
`winget install --id Microsoft.OpenJDK.17`; keduanya menulis ke
`<program-files>` sehingga menuntut Administrator.

**JDK yang lebih baru dari 17 tidak direkomendasikan** oleh React Native; AGP 9.2
menyatakan minimum 17, dan JDK 26 pernah dilaporkan memicu kegagalan
`JdkImageTransform` (isu AGP #486844145).
## 3. Android SDK

### 3a. Command-line saja (jalur yang dipakai, tanpa administrator)

```powershell
$sdk = "$env:LOCALAPPDATA\Android\Sdk"
New-Item -ItemType Directory -Force "$sdk\cmdline-tools" | Out-Null
curl.exe -sSL -o "$env:TEMP\cmdline.zip" `
  "https://dl.google.com/android/repository/commandlinetools-win-13114758_latest.zip"
tar -xf "$env:TEMP\cmdline.zip" -C "$sdk\cmdline-tools"
# SDK Manager menuntut folder bernama "latest"
Move-Item "$sdk\cmdline-tools\cmdline-tools" "$sdk\cmdline-tools\latest"
```

Terima lisensi lalu pasang paket yang dituntut repository ini:

```powershell
$env:JAVA_HOME = "$env:LOCALAPPDATA\facelink-toolchain\jdk\jdk-17.0.20.1+1"
& "$sdk\cmdline-tools\latest\bin\sdkmanager.bat" --licenses
& "$sdk\cmdline-tools\latest\bin\sdkmanager.bat" `
  "platform-tools" "platforms;android-37.0" "build-tools;37.0.0" `
  "ndk;27.1.12297006" "cmake;3.31.6"
```

**Nama paket platform memakai revisi minor.** Yang benar-benar dipakai adalah
`platforms;android-37.0`, bukan `platforms;android-37`; nama lama gagal dengan
"package not available". Gradle tetap memakai `compileSdk 37`, sehingga
`check:native` menerima folder `android-37.0` sebagai pemenuhan `android-37`.

**Emulator tidak dipasang.** Pemilik membatasi pemasangan dan target pengujian
adalah HP fisik; image Play Store sendiri beberapa GB.

Setel variabel lingkungan pada lingkup **User**:

```powershell
[Environment]::SetEnvironmentVariable('ANDROID_HOME', $sdk, 'User')
$p = [Environment]::GetEnvironmentVariable("PATH", "User")
[Environment]::SetEnvironmentVariable("PATH",
  $p + ";" + $env:JAVA_HOME + "\bin;" + $sdk + "\platform-tools;" + $sdk + "\cmdline-tools\latest\bin", "User")
```

### 3b. Android Studio (alternatif)

Pasang Android Studio, lalu **SDK Manager → SDK Platforms → Show Package
Details**: centang **Android SDK Platform 37.0**. Pada **SDK Tools**:
**Build-Tools 37.0.0**, **Command-line Tools (latest)**, **Platform-Tools**,
**NDK (Side by side) 27.1.12297006**, dan **CMake 3.31.6**. Emulator tidak
diperlukan untuk HP fisik.

### 3c. Batas jalur 260 karakter (kendala nyata, wajib dibaca)

Build pertama **gagal**:

```
ninja: error: mkdir(CMakeFiles/worklets.dir/C_/visual-studio-code/facelink-ai/
apps/mobile/node_modules/react-native-worklets/Common/cpp/worklets/SharedItems/
Serializable): No such file or directory
```

Sebabnya bukan kode project: `react-native-worklets` mengompilasi C++ di dalam
`node_modules`, dan jalur objeknya mencapai **268 karakter**. CMake 3.22.1
membawa **ninja 1.10**, yang gagal membuat folder pada jalur sepanjang itu
walaupun `LongPathsEnabled=1` sudah aktif. Memperpendek jalur lewat `subst`
atau junction **tidak menolong**: CMake menormalkan kembali ke jalur asli, dan
`subst` juga mematahkan autolinking Expo (pencarian `package.json` berhenti di
akar drive).

**Perbaikan yang terbukti:** pasang **CMake 3.31.6** (membawa ninja 1.12.1) dan
arahkan Gradle ke sana lewat `android/local.properties` (diabaikan Git):

```
sdk.dir=<android-sdk>
cmake.dir=<android-sdk>/cmake/3.31.6
```

Pakai **garis miring maju**. Garis miring terbalik pada `cmake.dir` membuat
Gradle gagal lebih awal dengan "The filename, directory name, or volume label
syntax is incorrect". `check:native` memeriksa hal ini pada baris
`ninja (jalur panjang)`.

`npx expo prebuild` **menghapus** `local.properties`; buat ulang setiap kali
prebuild dijalankan.
## 4. Perangkat fisik (target pengujian)

Emulator **tidak dipasang**: pemilik membatasi pemasangan dan target pengujian
adalah HP fisik; image Play Store juga beberapa GB.

**Perangkat fisik:**

1. Aktifkan *Opsi pengembang* (ketuk *Nomor bentukan* 7 kali) → *USB debugging*.
2. Sambungkan kabel USB data, lalu terima dialog otorisasi RSA di HP.
3. Verifikasi:

```powershell
adb devices -l      # harus berstatus "device", bukan "unauthorized"
adb shell getprop ro.product.model
```

Bila status `unauthorized`: cabut-sambung kabel dan pilih *File transfer*, atau
hapus otorisasi lama di *Opsi pengembang → Revoke USB debugging authorizations*.

**Koneksi ke backend lokal** memakai `adb reverse` sehingga backend tetap
mendengarkan `127.0.0.1` dan tidak terbuka ke jaringan:

```powershell
adb reverse tcp:8080 tcp:8080
```

`adb reverse` hilang setiap kali kabel dicabut atau server ADB dimulai ulang.

**Emulator (bila kelak dibutuhkan):** pasang
`"emulator" "system-images;android-36;google_apis_playstore;x86_64"`, lalu
`avdmanager create avd -n facelink -k "system-images;android-36;google_apis_playstore;x86_64"`.
Image harus bertanda *Play Store* agar Credential Manager tersedia;
`google_apis` atau AOSP tidak cukup.
## 5. Google Cloud Console — **tindakan manual pemilik**

Bagian ini tidak dapat dikerjakan agent: ia menuntut akun Google pemilik dan
persetujuan atas kredensial yang dibuat. **Tanpa langkah ini Google Sign-In tidak
mungkin berhasil**, sekalipun build berhasil.

1. Buat project di <https://console.cloud.google.com> (tanpa penagihan aktif;
   Google Sign-In tidak menuntut billing).
2. **APIs & Services → OAuth consent screen**: tipe *External*, isi nama aplikasi
   dan email dukungan. Selama masih *Testing*, tambahkan akun uji pada
   **Test users** — akun yang tidak terdaftar akan ditolak Google.
3. **APIs & Services → Credentials → Create credentials → OAuth client ID**:
   - **Web application** → catat `Client ID` (`…apps.googleusercontent.com`).
     Ini yang dipakai sebagai `EXPO_PUBLIC_FACELINK_GOOGLE_WEB_CLIENT_ID` **dan**
     sebagai entri `FACELINK_GOOGLE_CLIENT_IDS` di backend.
   - **Android** → isi *Package name* `ai.facelink.mobile` dan *SHA-1* keystore
     debug.

**Nilai nyata mesin ini (20 Oktober 2026, diverifikasi dengan
`apksigner verify --print-certs` atas APK hasil build):**

| Butir | Nilai |
|---|---|
| Application ID | `ai.facelink.mobile` (`android/app/build.gradle` → `applicationId`) |
| SHA-1 sertifikat debug | `5E:8F:16:06:2E:A3:CD:2C:4A:0D:54:78:76:BA:A6:F3:8C:AB:F6:25` |
| SHA-256 sertifikat debug | `FA:C6:17:45:DC:09:03:78:6F:B9:ED:E6:2A:96:2B:39:9F:73:48:F0:BB:6F:89:9B:83:32:66:75:91:03:3B:9C` |

Daftarkan **SHA-1** itu (bukan SHA-256) pada OAuth client Android. Nilai ini
berasal dari `android/app/debug.keystore` yang dihasilkan prebuild dan
**diabaikan Git**. Prebuild ulang pada mesin ini menghasilkan SHA-1 yang sama;
mesin lain akan berbeda.

Ambil ulang SHA-1 bila perlu (pilih salah satu):

```powershell
# A. keytool langsung
keytool -list -v -keystore apps/mobile/android/app/debug.keystore `
        -alias androiddebugkey -storepass android -keypass android |
  Select-String "SHA1:"
# B. lewat Gradle
cd apps/mobile/android; .\gradlew signingReport
# C. dari APK yang sudah dibangun
apksigner verify --print-certs app/build/outputs/apk/debug/app-debug.apk
```

Setelah SHA-1 didaftarkan, Google memerlukan beberapa menit sampai client ID
Android aktif. Kegagalan dengan kode `DEVELOPER_ERROR` biasanya berarti
SHA-1/package belum cocok.

**Sinkronisasi audience.** Client ID **Web** yang sama dipakai di dua tempat:
`EXPO_PUBLIC_FACELINK_GOOGLE_WEB_CLIENT_ID` (nilai `aud` pada ID token) dan
`FACELINK_GOOGLE_CLIENT_IDS` di backend. Client ID **Android** hanya dipakai
Google untuk memverifikasi paket + SHA-1; ia **bukan** audience backend dan
**tidak** ditanam di aplikasi.

### 5b. Langkah setelah kedua client ID ada (urutan yang terverifikasi)

Semua nilai `EXPO_PUBLIC_*` disisipkan **saat Metro membundel**, bukan saat APK
dikompilasi. Karena itu **APK tidak perlu dibangun ulang**, tetapi dev server
**wajib dijalankan ulang**. Urutannya:

```powershell
# 1. isi .env aplikasi (berkas diabaikan Git)
cd apps/mobile
Copy-Item .env.example .env -ErrorAction SilentlyContinue
#    EXPO_PUBLIC_FACELINK_API_BASE_URL=http://127.0.0.1:8080
#    EXPO_PUBLIC_FACELINK_GOOGLE_WEB_CLIENT_ID=<Web client ID>

# 2. jalankan backend DENGAN client ID yang sama
cd ..\..\backend
$env:FACELINK_GOOGLE_CLIENT_IDS = "<Web client ID>"
.\facelink-server.exe
#    Sebelum variabel ini disetel, backend mencetak:
#      WARN login Google belum dikonfigurasi; setel FACELINK_GOOGLE_CLIENT_IDS
#    Setelah disetel, baris WARN itu tidak muncul lagi. Itu bukti konfigurasi
#    terbaca, bukan sekadar berkasnya ada.

# 3. jalankan ulang Metro (CI dihapus, port yang benar)
cd ..\apps\mobile
Remove-Item Env:CI -ErrorAction SilentlyContinue
npx expo start --dev-client --port 8082

# 4. sambungkan perangkat
adb reverse tcp:8082 tcp:8082      # Metro
adb reverse tcp:8080 tcp:8080      # backend

# 5. di perangkat: tutup paksa aplikasi lalu buka lagi, tekan Continue with Google
adb shell am force-stop ai.facelink.mobile
adb shell monkey -p ai.facelink.mobile -c android.intent.category.LAUNCHER 1
```

**Cara membedakan kegagalan** (semuanya sudah punya pesan sendiri di aplikasi):

| Gejala di layar | Arti | Periksa |
|---|---|---|
| "Aplikasi belum dikonfigurasi" | `EXPO_PUBLIC_FACELINK_GOOGLE_WEB_CLIENT_ID` kosong pada bundel | `.env` terisi, Metro **dijalankan ulang** setelah `.env` diubah |
| "Layanan masuk belum siap" | backend hidup tanpa `FACELINK_GOOGLE_CLIENT_IDS` | baris WARN backend; `503 GOOGLE_UNAVAILABLE` |
| "Backend tidak terjangkau" | `adb reverse` hilang | `adb reverse --list` harus memuat 8080 dan 8082 |
| Google menolak sebelum kembali ke aplikasi | SHA-1/package belum cocok atau akun tak ada di *Test users* | `DEVELOPER_ERROR` di logcat, §5 |
| Google mengembalikan token tetapi backend menolak | `aud` ≠ `FACELINK_GOOGLE_CLIENT_IDS` | `IDENTITY_TOKEN_INVALID` di log backend |

Akun uji harus dibuat lebih dahulu (§9): bootstrap admin → masuk sekali →
undang `dosen`/`mahasiswa`. Tanpa itu, login berhasil di sisi Google tetapi
aplikasi berhenti di layar "Anda belum terdaftar" — itu perilaku yang benar,
bukan kerusakan.

## 6. Berkas `.env` aplikasi

`apps/mobile/.env` (diabaikan Git):

```
EXPO_PUBLIC_FACELINK_API_BASE_URL=https://10.0.2.2:8080
EXPO_PUBLIC_FACELINK_GOOGLE_WEB_CLIENT_ID=<Web client ID dari langkah 5>
```

Backend harus menerima client ID yang sama:

```powershell
cd backend
$env:FACELINK_GOOGLE_CLIENT_IDS = "<Web client ID>"
go run ./cmd/server
```

Untuk autentikasi, backend juga wajib dijalankan dengan
`FACELINK_TLS_CERT`/`FACELINK_TLS_KEY` dan CA debug yang dipercaya (§17–§18).
HTTP loopback hanya untuk diagnostik tanpa token/Metro, bukan login atau sesi.

## 7. Build dan jalankan

```powershell
cd apps/mobile
npx expo prebuild --platform android   # folder android/ (diabaikan Git)
# Prebuild menghapus local.properties; buat ulang (garis miring MAJU):
$base = $env:LOCALAPPDATA.Replace([char]92, "/")
"sdk.dir=$base/Android/Sdk"                | Set-Content android/local.properties
"cmake.dir=$base/Android/Sdk/cmake/3.31.6" | Add-Content android/local.properties
npm run check:native                   # semua prasyarat harus PASSED
npx expo run:android                   # build debug + pasang ke perangkat
adb reverse tcp:8080 tcp:8080          # perangkat → backend laptop
```

**Skrip npm:** `npm run android` kini menunjuk `expo run:android` (dev build),
bukan Expo Go. Tersedia juga `npm run android:device` (memilih perangkat) dan
`npm run android:go` (jalur Expo Go lama, **tidak** dapat dipakai untuk login
Google karena `expo-secure-store` dan `react-native-nitro-google-signin` adalah
modul native).

**Mempersingkat build** dengan satu ABI:

```powershell
cd android
.\gradlew assembleDebug -PreactNativeArchitectures=arm64-v8a --console=plain
```

Hasil: `android/app/build/outputs/apk/debug/app-debug.apk`.

**Bukti build pada mesin ini (20 Oktober 2026):** `BUILD SUCCESSFUL`; APK
`ai.facelink.mobile` versionCode 1, compileSdk 37, targetSdk 36, minSdk 24,
tersigning sertifikat debug dengan SHA-1 yang cocok dengan §5. Bundel
JavaScript Android juga diverifikasi terpisah dengan
`npx expo export --platform android` (3,4 MB, exit 0). **Pemasangan ke HP dan
pengoperasian aplikasi dibuktikan pada 21 Oktober 2026** (FL-ANDROID01C, §12);
APK yang berjalan di perangkat terbukti identik dengan artefak ini.
## 8. Urutan pengujian setelah build terpasang

**Status skenario A1–A12: sebagian PASSED.** Kredensial OAuth kini tersedia dan
login nyata sudah diuji; status per butir ada di **§17c** (A1, A6, A7, A9, A12
lulus; A11 parsial; A8 parsial; A5 belum terbukti; A2/A3/A4/A10 menunggu akun
tambahan). Butir di bawah tetap menjadi rujukan langkahnya. **Yang SUDAH diuji pada HP
fisik tanpa kredensial** adalah lapisan perangkat: pemasangan, splash, layar
masuk, latar video, safe area, dan konektivitas `adb reverse` — hasil nyata di
**§12**.

Semua langkah di bawah menuntut akun uji terdaftar (lihat bagian 9).

| # | Skenario | Cara | Kriteria lulus |
|---|---|---|---|
| A1 | Masuk berhasil | Tekan **Continue with Google**, pilih akun `admin` | Beranda admin tampil; role berasal dari backend |
| A2 | Role dosen | Keluar, masuk dengan akun `dosen` | Beranda dosen, bukan admin |
| A3 | Role mahasiswa | Keluar, masuk dengan akun `mahasiswa` | Beranda mahasiswa |
| A4 | Akun belum terdaftar | Masuk dengan Google yang **tidak** diundang | Layar "Anda belum terdaftar", **bukan** galat jaringan, **bukan** dashboard |
| A5 | Batal | Tekan tombol lalu tutup lembar akun | Kembali ke layar masuk tanpa pesan galat |
| A6 | SecureStore | Masuk, tutup paksa aplikasi, buka lagi | Langsung masuk tanpa memilih akun lagi |
| A7 | Pemulihan sesi | Matikan backend, buka aplikasi | Layar masuk dengan "Coba lagi"; **token tidak hilang** |
| A8 | Penyegaran | Advance clock server terisolasi dalam pengujian; E2E memakai expiry sebenarnya, bukan menggeser jam perangkat | Access expired ditolak, refresh berotasi, token baru memulihkan sesi |
| A9 | Logout | Pengaturan → Keluar | Layar masuk; token hilang; backend mencatat `auth.logout` |
| A10 | Cabut akses | Sebagai admin, `PATCH /users/{id}` status `suspended`, lalu buka aplikasi | Kembali ke layar masuk/ditolak, bukan dashboard |
| A11 | Routing lintas-role | Buka `facelinkai:///admin` dengan sesi mahasiswa | Dialihkan ke beranda mahasiswa |
| A12 | Tidak ada demo | Periksa lembar pengaturan | Tidak ada tombol ganti role/demo |

Bukti yang dikumpulkan per langkah: tangkapan layar, keluaran `adb logcat` yang
relevan (tanpa token), dan baris audit backend.

## 9. Akun uji

Backend tidak memiliki pendaftaran mandiri. Urutannya:

1. Bootstrap admin pertama: `cd backend; .\facelink-server.exe -bootstrap-admin`
   (sekali saja; menolak berjalan bila sudah ada admin).
2. Masuk sebagai admin itu lewat aplikasi → akun menjadi `active`.
3. Undang akun `dosen` dan `mahasiswa` lewat `POST /api/v1/users` (status
   `invited`), lalu masing-masing masuk sekali agar tertaut.
4. Untuk uji A4, gunakan akun Google yang **tidak** diundang.

## 10. Masalah yang sudah diketahui

| Gejala | Sebab | Tindakan |
|---|---|---|
| `PLAY_SERVICES_NOT_AVAILABLE` | Emulator tanpa image Play Store | Pakai `google_apis_playstore` |
| `DEVELOPER_ERROR` | SHA-1 atau package name tidak cocok | Daftarkan ulang SHA-1 hasil langkah 5 |
| `IDENTITY_TOKEN_INVALID` | `aud` token ≠ client ID backend | Samakan `EXPO_PUBLIC_FACELINK_GOOGLE_WEB_CLIENT_ID` dengan `FACELINK_GOOGLE_CLIENT_IDS` |
| `503 GOOGLE_UNAVAILABLE` | Backend tanpa `FACELINK_GOOGLE_CLIENT_IDS` | Setel variabel itu sebelum menjalankan server |
| "Backend tidak terjangkau" | `adb reverse` hilang setelah cabut/ADB restart | Ulangi `adb reverse tcp:8080 tcp:8080` |
| Aplikasi menampilkan "Masuk Google tidak tersedia di sini" | Berjalan di Expo Go atau web | Gunakan dev build (`npx expo run:android`) |

## 11. Yang tidak diubah oleh panduan ini

- Tidak ada konfigurasi plugin Expo yang ditambahkan untuk
  `react-native-nitro-google-signin`. Untuk Android dengan `webClientId`
  eksplisit, config plugin itu **tidak diperlukan** (lihat `apps/mobile/README.md`
  §"Development build" untuk dasar teknisnya dari sumber paket 2.3.0).
- Tidak ada target iOS. `iosUrlScheme` belum dikonfigurasi dan belum diuji.
- Tidak ada kredensial, keystore rilis, atau `google-services.json` yang dibuat.
- `edgeToEdgeEnabled` **dihapus** dari `app.json` pada FL-ANDROID01B: Expo SDK 54+
  menyatakan properti itu tidak lagi tersedia (Android 16 memaksa edge-to-edge),
  dan prebuild hanya mencetak peringatan. Verifikasi sebelum/sesudah menunjukkan
  `AndroidManifest.xml` dan `res/values/styles.xml` **byte-identik**, sehingga
  penghapusan tidak mengubah tata letak; safe area tetap ditangani
  `SafeAreaProvider` dan `Screen` (`edges={["top","bottom"]}`).

## 12. Bukti pengujian perangkat fisik (FL-ANDROID01C, 21 Oktober 2026)

Dijalankan pada HP yang benar-benar tersambung USB dengan USB debugging aktif.
Tidak ada perubahan kode aplikasi pada tahap ini: verifikasi dan dokumentasi saja.

**Perangkat**

| Butir | Nilai (dibaca dari perangkat) |
|---|---|
| Model | TECNO LJ7 (`ro.product.model` = `TECNO_LJ7`) |
| Android | 16 (`ro.build.version.release` = 16, SDK 36) |
| ABI | `arm64-v8a` (satu-satunya pada `ro.product.cpu.abilist`) |
| Layar | 1080×2460, densitas 440 (override 432) |
| Transport ADB | `device` (bukan `unauthorized`), `transport_id=4` |

**Kesesuaian APK.** `unzip -l app-debug.apk` → satu-satunya direktori
`lib/` adalah `arm64-v8a`, cocok dengan ABI perangkat. Aplikasi sudah terpasang
(`ai.facelink.mobile`, versionCode 1, minSdk 24, targetSdk 36,
`primaryCpuAbi=arm64-v8a`). `base.apk` yang ditarik dari perangkat memiliki
**SHA-256 identik** dengan `android/app/build/outputs/apk/debug/app-debug.apk`
(`31d51107…79c4801`), jadi yang berjalan memang artefak build ini, bukan versi
lain.

**Hasil per butir**

| # | Pemeriksaan | Hasil | Bukti |
|---|---|---|---|
| D1 | ADB mendeteksi perangkat berstatus `device` | **PASSED** | `adb devices -l` |
| D2 | ABI APK cocok dengan perangkat | **PASSED** | `lib/arm64-v8a` vs `ro.product.cpu.abilist` |
| D3 | Instalasi tanpa menghapus aplikasi lain | **PASSED** | paket sudah ada, data tidak disentuh |
| D4 | Splash screen tampil | **PASSED** | tangkapan 2–3 setelah `am start`: latar hitam (luma 16) dengan ikon di tengah (luma rata-rata 128) |
| D5 | Layar masuk dirender native | **PASSED** | dump UI: `FaceLink` + `Ai`, tombol `Continue with Google` (`clickable="true"`), teks keterangan, `Privacy policy` / `Terms of service` |
| D6 | Latar video aktif (bukan animasi cadangan) | **PASSED** | `TextureView` penuh layar di hierarki; bundel Metro memuat `video-clips.local` + `video1/video2.mp4`; dekoder (`MediaCodec`) berpindah status tiap ±8 dtk; dua tangkapan 3 dtk terpisah berbeda (PSNR 21,8 dB) |
| D7 | Safe area dan status bar | **PASSED** | blok merek mulai y=1096 dan baris terbawah y=2352, keduanya di dalam area aman (status bar 0–105, nav bar 2417–2460); jam/ikon status bar terbaca di tangkapan (luma > 180 pada 11% piksel) |
| D8 | Aplikasi tidak crash | **PASSED** | `FATAL EXCEPTION`/ANR untuk `ai.facelink.mobile` = 0; proses hidup setelah restart paksa |
| D9 | `adb reverse` menjangkau Go Backend | **PASSED** | layar Diagnostik di perangkat: `healthz: ok`, `readyz: ready`, `database: ok`, `migrations: applied`, `WAL: wal`; log backend mencatat `GET /api/v1/healthz` dan `/readyz` status 200 dari perangkat |
| D10 | Tombol masuk menolak tanpa Web client ID | **PASSED** | tekan tombol → "Aplikasi belum dikonfigurasi" + alasan `EXPO_PUBLIC_FACELINK_GOOGLE_WEB_CLIENT_ID belum disetel`; **tidak ada** permintaan ke backend (log backend bersih) dan tidak ada sesi palsu |
| D11 | Sesi kosong setelah restart | **PASSED** | `am force-stop` lalu buka lagi → layar masuk biasa, tanpa dashboard |
| D12 | Google Sign-In nyata (A1–A12) | **BLOCKED** | Web/Android OAuth client belum ada (§5) |

**Dua kendala nyata saat menjalankan dev server** (bukan cacat aplikasi, tetapi
memakan waktu diagnosis):

1. **Port 8081 dipakai Docker Desktop.** `127.0.0.1:8081` menjawab `200` dengan
   badan **0 byte**, sehingga pemeriksaan `curl` tampak berhasil padahal Metro
   tidak menerima permintaan. Gejala di perangkat: *"There was a problem loading
   the project."* Solusi yang dipakai: `npx expo start --dev-client --port 8082`
   + `adb reverse tcp:8082 tcp:8082`.
2. **`CI=true` mematikan watch mode.** Metro mencetak
   `Metro is running in CI mode, reloads are disabled`; dev server tetap
   melayani bundel, tetapi Fast Refresh mati. Jalankan dengan `CI` dihapus.

**Catatan menjalankan `check:native`.** Variabel `JAVA_HOME`/`ANDROID_HOME`
tersimpan pada lingkup *User* Windows; shell yang belum login ulang tidak
mewariskannya. Bila skrip melaporkan JDK/SDK BLOCKED padahal foldernya ada,
berikan variabel itu eksplisit, dengan **PATH bergaya Windows** (`;`):

```powershell
$jdk = "$env:LOCALAPPDATA\facelink-toolchain\jdk\jdk-17.0.20.1+1"
$env:JAVA_HOME = $jdk
$env:ANDROID_HOME = "$env:LOCALAPPDATA\Android\Sdk"
$env:Path = "$jdk\bin;$env:ANDROID_HOME\platform-tools;$env:Path"
cd apps/mobile; npm run check:native
```

Hasil nyata dengan perangkat tersambung: **13 PASSED, 0 FAILED, 1 BLOCKED**
(satu-satunya BLOCKED adalah `EXPO_PUBLIC_FACELINK_GOOGLE_WEB_CLIENT_ID`), dan
`npm test` **80/80 lulus**.

**Langkah berikutnya (tindakan pemilik, §5):** buat OAuth client **Web** dan
**Android**, lalu isi `apps/mobile/.env`
(`EXPO_PUBLIC_FACELINK_GOOGLE_WEB_CLIENT_ID`) dan jalankan backend dengan
`FACELINK_GOOGLE_CLIENT_IDS` bernilai sama. Setelah itu skenario A1–A12 baru
dapat dijalankan. Membangun ulang APK **tidak** diperlukan karena nilai
`EXPO_PUBLIC_*` disisipkan saat Metro membundel JS, bukan saat APK dikompilasi —
tetapi dev server **harus dihentikan dan dijalankan ulang** agar `.env` dibaca
ulang, lalu aplikasi memuat bundel baru (Reload).

## 13. Kelayakan Android Emulator (FL-ANDROID02A, audit 21 Oktober 2026)

**Tahap audit saja.** Tidak ada Windows Optional Feature yang diubah, tidak ada
perintah administrator, tidak ada unduhan emulator/image, tidak ada restart, dan
tidak ada container/proses yang dihentikan. Seluruh angka di bawah dibaca dari
mesin ini pada 21 Oktober 2026.

### 13a. Resource laptop

| Butir | Nilai nyata | Catatan |
|---|---|---|
| Model | Lenovo 83S0 (LOQ), Windows 11 **Home Single Language** 26200 | — |
| CPU | AMD Ryzen 7 170, 8 core / 16 thread, 3.2 GHz | beban 18% saat audit |
| RAM fisik | **15,19 GB** — **satu** modul 16 GB (Samsung, 5600 MT/s) | 2 slot tersedia, **1 kosong**; kapasitas maksimum 64 GB |
| RAM tersedia | **0,6–1,7 GB** (fluktuasi) | ≈ 4–11% dari total |
| Commit charge | **52,4 GB / 61,2 GB (85%)** | indikator paling jujur: sistem sudah menekan pagefile |
| Pagefile | 47 GB dialokasikan, 8,9 GB terpakai, puncak 12,2 GB | tanda tekanan memori berulang |
| Disk C: | 219,5 GB bebas dari 474,5 GB | cukup |
| GPU | NVIDIA RTX 3050 Laptop 6 GB + AMD Radeon 680M | emulator dapat memakai GPU host |

**Pemakai RAM terbesar (agregat per aplikasi):**

| Proses | Jumlah | RAM |
|---|---|---|
| Chrome | 61 proses | **4,3–5,6 GB** |
| ChatGPT (desktop) | 13 proses | 0,86 GB |
| OMP (sesi ini) | 9 proses | 0,71 GB |
| VS Code | 20 proses | 0,60 GB |
| Docker Desktop + `vmmemWSL` | 6 proses + 1 VM | 0,54 GB |
| Telegram, LegionSpace, Edge, layanan sistem | — | 1,1 GB |

Docker menjalankan **5 container** yang sudah hidup 4 hari: `tracker-worker`,
`tracker-dashboard`, `tracker-db` (postgres), `enowx-rag-local-qdrant-1`, dan
`enowx-rag-local-tei-1`. **Tidak dihentikan** — itu pekerjaan Anda, dan
penghentiannya butuh persetujuan.

### 13b. Virtualisasi

| Butir | Nilai | Arti |
|---|---|---|
| `VirtualizationFirmwareEnabled` | **True** | AMD-V aktif di firmware; **tidak perlu** masuk BIOS |
| Hypervisor berjalan | `HypervisorPresent = True`; `hvhost` + `vmcompute` aktif | Windows sudah menjalankan hypervisor |
| VBS / Core Isolation | **aktif** (`VirtualizationBasedSecurityStatus = 2`) | memakai hypervisor yang sama |
| `HypervisorPlatform` (WHPX) | **terdaftar, InstallState = 2 (Disabled)** | **dapat diaktifkan tanpa BIOS**, tetapi butuh administrator + restart |
| `VirtualMachinePlatform` | InstallState = 1 (Enabled) | dipakai WSL2/Docker; **bukan** WHPX |
| Hyper-V lengkap | tidak terpasang | tidak diperlukan |
| AEHD | tidak terpasang; service `aehd` tidak ada | **tidak disarankan** — diumumkan *sunset* 31 Desember 2026 dan berkonflik dengan VBS |
| `VMMonitorModeExtensions` / SLAT (WMI) | `False` | **bukan** bukti virtualisasi mati: ketika hypervisor sudah berjalan, Windows menyembunyikan flag ini dari sistem operasi tamu |

**Kesimpulan virtualisasi:** hambatannya **hanya satu fitur Windows** yang perlu
diaktifkan, bukan BIOS dan bukan perubahan keamanan. Jalur WHPX kompatibel dengan
VBS yang sudah aktif.

### 13c. Paket yang tersedia (diverifikasi lewat SDK Manager + metadata Google)

| Paket | Versi | Unduhan | Status |
|---|---|---|---|
| `emulator` | 37.2.12 (stable) | **434 MB** (Windows x64) | tersedia, belum dipasang |
| `system-images;android-36;google_apis_playstore;x86_64` | rev 1 | **1.836 MB** | tersedia |
| `system-images;android-36.1;google_apis_playstore;x86_64` | rev 4 | 1.909 MB | tersedia |
| `system-images;android-37.0;google_apis_playstore;x86_64` | rev 1 | 2.218 MB | tersedia (paling baru) |
| `system-images;android-35;google_apis_playstore;x86_64` | — | 1.680 MB | tersedia (paling ringan yang masih Play Store) |
| `extras;google;Android_Emulator_Hypervisor_Driver` | 2.2.0 | — | tersedia; **tidak disarankan** (lihat 13b) |

Tidak ada `emulator/`, `system-images/`, maupun AVD di mesin ini — semuanya masih
perlu diunduh. Profil perangkat yang tersedia untuk AVD antara lain
`medium_phone`, `pixel_6`, `pixel_7`, dan `small_phone`.

### 13d. Kesesuaian build: **x86_64 didukung, tetapi APK saat ini tidak**

- `android/gradle.properties` → `reactNativeArchitectures=armeabi-v7a,arm64-v8a,x86,x86_64`.
- `react-native-nitro-google-signin` dan `react-native-nitro-modules` menyaring ABI
  dengan daftar yang **memuat `x86_64`** (`abiFilters`), jadi modul native
  (termasuk Google Sign-In) dapat dikompilasi untuk emulator.
- APK yang ada **hanya berisi `lib/arm64-v8a`** karena build sebelumnya dibatasi
  `-PreactNativeArchitectures=arm64-v8a`. APK itu **tidak dapat dipasang** di
  emulator x86_64; perlu build baru dengan `-PreactNativeArchitectures=x86_64`.
- Gradle memakai `-Xmx2048m` (`org.gradle.jvmargs`), jadi build x86_64 menambah
  beban ±2–3 GB selama kompilasi. Build **tidak** dijalankan pada audit ini
  (menunggu keputusan), karena justru akan menghabiskan sisa RAM.

### 13e. Anggaran RAM: Emulator + Metro + OMP + backend

| Komponen | Perkiraan | Dasar |
|---|---|---|
| Emulator x86_64 (2 core, 2048 MB RAM tamu) | 2,5–3,5 GB | RAM tamu 2 GB + overhead QEMU/GPU ±0,5–1,5 GB |
| Metro (`expo start`) | 0,4–0,9 GB | proses Node saat melayani bundel |
| OMP | 0,7 GB | terukur pada sesi ini |
| Go Backend + SQLite | 0,1–0,2 GB | biner kecil |
| **Subtotal yang dibutuhkan** | **± 4–5,5 GB** | — |
| **Tersedia saat audit** | **0,6–1,7 GB** | — |

**Selisih ± 3–4 GB.** Emulator **tidak akan** berjalan layak tanpa membebaskan
memori lebih dahulu. Dua sumber terbesar yang dapat dibebaskan **tanpa**
menyentuh pekerjaan Anda:

| Tindakan | Bebas | Dampak |
|---|---|---|
| Tutup tab/instance Chrome yang tidak dipakai (61 proses, 4,3–5,6 GB) | 2–4 GB | tidak ada; dapat dibuka lagi |
| Tutup ChatGPT desktop (0,86 GB) dan Telegram (0,2 GB) | ± 1 GB | tidak ada |
| Tutup Docker Desktop **hanya bila** pekerjaan tracker/rag boleh berhenti | 0,5–1,5 GB | **butuh persetujuan Anda** — 5 container aktif 4 hari |

Dengan menutup Chrome + ChatGPT + Telegram saja, tersedia ± 3,5–6,5 GB — cukup
untuk emulator 2 GB. **Docker tidak perlu dihentikan** untuk konfigurasi ini.

### 13f. Konfigurasi paling ringan yang direkomendasikan

| Keputusan | Nilai | Alasan |
|---|---|---|
| Image | `system-images;android-36;google_apis_playstore;x86_64` (1.836 MB) | **Play Store wajib** untuk Credential Manager/Google Sign-In; Android 36 mencakup `targetSdk 36` aplikasi; 382 MB lebih kecil dari 37.0 dan 73 MB lebih kecil dari 36.1 |
| Alternatif lebih hemat | `system-images;android-35;...;x86_64` (1.680 MB) | hanya bila ruang/unduhan jadi masalah; tetap Play Store |
| Profil AVD | `medium_phone` (1080×2400, 420 dpi) | lebih ringan dari `pixel_7` (1080×2400 @ 420 dpi dengan fitur tambahan); cukup untuk tata letak aplikasi |
| RAM tamu | **2048 MB** | batas bawah yang masih stabil untuk Android 36 + Play Services |
| Heap tamu | `vm.heapSize=256` | mengurangi tekanan |
| Core | **2** | menyisakan 14 thread untuk host |
| Penyimpanan AVD | 6 GB (data partition) | aplikasi debug 100 MB + cache Play Services |
| GPU | `-gpu host` (fallback `swiftshader_indirect`) | RTX 3050 tersedia; fallback bila driver bermasalah |
| Boot | `-no-boot-anim -no-snapshot-load` | boot lebih cepat, RAM lebih rendah |
| Disk cache | `-no-audio` | menghemat CPU/RAM |

**Jangan** memakai `-accel off` sebagai jalur utama: tanpa akselerasi emulator
berjalan sebagai emulasi penuh dan boot dapat memakan puluhan menit — hasil
ujinya tidak mewakili performa nyata.

### 13g. Estimasi unduhan dan ruang SSD

| Tahap | Unduhan | Setelah ekstrak (perkiraan) |
|---|---|---|
| `emulator` 37.2.12 | 434 MB | ± 1,2 GB |
| Image `android-36;google_apis_playstore;x86_64` | 1.836 MB | ± 4,0 GB |
| AVD (data + snapshot awal) | — | ± 2–6 GB |
| **Total** | **± 2,3 GB** | **± 7–11 GB** |

Ruang bebas 219,5 GB — jauh di atas kebutuhan. Angka "setelah ekstrak" adalah
perkiraan, bukan hasil pengukuran.

### 13h. Perubahan Windows yang diperlukan

| Perubahan | Butuh | Dampak |
|---|---|---|
| `Enable-WindowsOptionalFeature -Online -FeatureName HypervisorPlatform -All` | **administrator + restart** | mengaktifkan API WHPX; hypervisor sudah berjalan karena VBS, jadi tidak menambah lapisan baru secara konseptual |
| Restart Windows | **ya**, sekali | wajib agar fitur aktif |
| Perubahan BIOS/UEFI | **tidak** | AMD-V sudah aktif |
| Mematikan VBS/Core Isolation | **tidak** | WHPX berjalan **di atas** VBS |
| Mematikan Hyper-V | **tidak** | Hyper-V penuh tidak terpasang |
| AEHD | **tidak** | *sunset* 31 Des 2026 dan berkonflik dengan VBS |

**Risiko:** mengaktifkan WHPX menambah permukaan virtualisasi yang dapat dipakai
proses lain, dan sebagian kecil pengguna melaporkan penurunan performa game/GPU
setelah fitur ini aktif. Fitur dapat dinonaktifkan kembali dengan perintah yang
sama (`-Disable`) bila tidak dipakai.

### 13i. Alternatif bila emulator tetap terlalu berat

| Alternatif | Kelebihan | Kekurangan |
|---|---|---|
| **Tetap HP fisik, hanya saat perlu** | bukti paling nyata; sudah terbukti (FL-ANDROID01C) | menuntut kabel; tidak dapat dipakai bersamaan dengan mobilitas |
| **Emulator 1 GB RAM + Android 35** | jejak RAM ± 2 GB | lebih lambat, Play Services berat |
| **Tambah RAM fisik** (slot ke-2 kosong, maks 64 GB) | menghilangkan hambatan akar | biaya perangkat keras |
| **Uji UI di preview web** (`npm run web`) | nol biaya RAM tambahan | tanpa modul native, tanpa Google Sign-In |
| **Uji logika di Node** (`npm test`) | sudah berjalan 80/80 | tidak menguji rendering/perangkat |

### 13j. Keputusan yang diminta

1. **Izin administrator + restart** untuk mengaktifkan `HypervisorPlatform`.
2. **Izin unduh ± 2,3 GB** (`emulator` + image `android-36` Play Store x86_64).
3. **Konfirmasi bahwa menutup Chrome/ChatGPT/Telegram sementara** boleh
   dilakukan saat pengujian (Docker **tidak** perlu dihentikan).
4. **Konfirmasi build APK x86_64** boleh dijalankan (menambah ± 2–3 GB RAM selama
   kompilasi; sebaiknya setelah Chrome ditutup).

**Yang belum dilakukan dan tidak akan dilakukan tanpa izin:** aktivasi fitur,
unduhan, pembuatan AVD, build x86_64, restart, dan penghentian container.

## 14. FL-ANDROID02B — Persiapan bertahap (dimulai 21 Oktober 2026)

Tugas ini dibagi menjadi gate. Status saat ini: **gate B4 dan C3 belum dilewati**,
sehingga tidak ada perubahan sistem, unduhan, atau instalasi yang dijalankan.

### 14a. Baseline resource (21 Oktober 2026, sebelum optimasi)

| Butir | Nilai | Ambang yang disepakati |
|---|---|---|
| RAM fisik | 15,19 GB (satu modul 16 GB; slot ke-2 kosong) | — |
| RAM tersedia | **0,96–1,10 GB** (5 sampel) | target 5–6 GB sebelum emulator |
| Commit charge | **51,8 / 61,2 GB (85%)** | — |
| Pagefile | 46 GB dialokasikan, 9,1 GB terpakai, puncak 11,9 GB | — |
| Disk C: | **219,1 GB bebas** dari 474,5 GB | ≥ 15 GB |
| CPU | Ryzen 7 170, 8C/16T | — |

**Kesimpulan baseline:** RAM tersedia **≈ 1 GB**, jadi emulator **tidak boleh**
dijalankan sekarang. Selisih terhadap target ± 4–5 GB.

### 14b. Proses yang dapat dibebaskan (tanpa dihentikan oleh agent)

Angka adalah *working set* agregat; jumlah yang benar-benar kembali ke sistem
biasanya lebih besar daripada selisihnya, karena memori bersama dilepas saat
semua proses pemiliknya berhenti.

| Aplikasi | Proses | Working set | Perlu ditutup seluruhnya? |
|---|---|---|---|
| Chrome | 63 | **5.858 MB** | ya — menutup sebagian tab tidak cukup |
| ChatGPT desktop | 13 | 957 MB | ya |
| Telegram | 1 | 294 MB | ya |
| Microsoft Edge | 7 | 307 MB | ya |
| LegionSpace | 10 | 211 MB | opsional |
| codex | 3 | 196 MB | opsional |

**Yang TIDAK boleh ditutup untuk keperluan ini:** OMP (sesi ini), VS Code,
Docker Desktop + `vmmemWSL` (5 container: tracker-worker, tracker-dashboard,
tracker-db, qdrant, tei), dan Go/JDK yang dipakai build. Agent tidak
menghentikan satu pun dari mereka.

**Perkiraan setelah penutupan:** menutup Chrome + ChatGPT + Telegram + Edge
membebaskan ± 5,5–7 GB working set; RAM tersedia diperkirakan **5–7 GB**.
Angka ini **perkiraan**, bukan jaminan — harus diukur ulang (gate B4).

### 14c. Status WHPX (gate C3)

| Butir | Nilai |
|---|---|
| `HypervisorPlatform` (WHPX) | **InstallState = 2 (Disabled)** — terdaftar, belum aktif |
| `VirtualMachinePlatform` | InstallState = 1 (Enabled) — untuk WSL2, bukan WHPX |
| `VirtualizationFirmwareEnabled` | **True** — AMD-V aktif, **BIOS tidak perlu diubah** |
| Hypervisor berjalan | **True** (VBS aktif, `VirtualizationBasedSecurityStatus = 2`) |
| AEHD | tidak terpasang — **tidak disarankan** (sunset 31 Des 2026, konflik VBS) |

**Perintah resmi (belum dijalankan; menuntut PowerShell Administrator):**

```powershell
# aktifkan (butuh administrator + satu restart)
Enable-WindowsOptionalFeature -Online -FeatureName HypervisorPlatform -All

# membatalkan bila kelak tidak diperlukan
Disable-WindowsOptionalFeature -Online -FeatureName HypervisorPlatform -All
```

Setelah restart, verifikasi harus memakai biner emulator, bukan asumsi:

```powershell
& "$env:LOCALAPPDATA\Android\Sdk\emulator\emulator.exe" -accel-check
```

### 14d. Paket yang menunggu izin unduh (gate D)

Terverifikasi ulang lewat `sdkmanager --list` pada 21 Oktober 2026:

| Paket | Versi | Unduhan |
|---|---|---|
| `emulator` | 37.2.12 | 434 MB |
| `system-images;android-36;google_apis_playstore;x86_64` | rev 7 | 1.836 MB |

Folder `emulator/`, `system-images/`, dan AVD **belum ada** di mesin ini.

### 14e. Urutan gate yang disepakati

| Gate | Isi | Status |
|---|---|---|
| B4 | pemilik menutup aplikasi, agent mengukur ulang; target 5–6 GB | **menunggu tindakan pemilik** |
| C3 | izin administrator + restart untuk aktivasi WHPX | **menunggu persetujuan** |
| D | unduh emulator + image, buat AVD 2048 MB / 2 vCPU | terkunci oleh B4 dan C3 |
| D4 | boot emulator, ukur RAM aktual | terkunci |
| E | build APK x86_64, instal, jalankan, logcat | terkunci |

**Aturan berhenti yang berlaku:** bila setelah optimasi RAM tersedia masih jauh
di bawah target, rencana instalasi dihentikan dan dilaporkan — bukan dipaksakan.

## 15. FL-ANDROID02C — Uji FaceLink AI di emulator (22 Oktober 2026)

Emulator dipasang **manual oleh pemilik** lewat Android Studio. Agent tidak
mengubah Windows Hypervisor Platform, tidak melakukan restart, dan tidak
menghentikan container. Seluruh angka di bawah adalah hasil nyata dari sesi ini.

### 15a. Perangkat dan kondisi host

| Butir | Nilai |
|---|---|
| Perangkat | `emulator-5554`, model `sdk_gphone64_x86_64` |
| Android | **16 / API 36**, x86_64, **Google Play** (`google_apis_playstore`) |
| AVD | `Medium_Phone`, RAM tamu **2048 MB**, **4 core**, heap 228 MB, `hw.gpu.enabled=yes` |
| Layar | 1080×2400 @ 420 dpi |
| RAM host sebelum build | **0,16 GB** tersedia, commit 47,4/61,2 GB |
| RAM host setelah boot + Metro | **1,26 GB** tersedia, commit 52,6/61,2 GB |
| Proses emulator (`qemu-system-x86_64`) | **1.084 MB** working set |

RAM host tidak pernah dipulihkan ke 5–6 GB seperti target §14, tetapi emulator
tetap stabil. Build dijalankan **serial** (`--max-workers=2`) agar tidak
menambah tekanan memori.

### 15b. Hasil per butir

| # | Pemeriksaan | Hasil | Bukti |
|---|---|---|---|
| E1 | Emulator terdeteksi ADB | **PASSED** | `adb devices -l` → `emulator-5554 device` |
| E2 | Branch dan working tree bersih | **PASSED** | `feat/fl-ui02-google-auth-android`, `git status` kosong |
| E3 | Build APK debug x86_64 | **PASSED** | `assembleDebug -PreactNativeArchitectures=x86_64` → `BUILD SUCCESSFUL in 9m 56s`, APK **99.792.021 B**, hanya `lib/x86_64/` |
| E4 | Instalasi tanpa menghapus data | **PASSED** | `adb install -r` → `Success`; tidak ada `uninstall`/`clear` |
| E5 | ABI yang berjalan | **PASSED** | `dumpsys package` → `primaryCpuAbi=x86_64` |
| E6 | Metro terjangkau dari emulator | **PASSED** (setelah perbaikan, §15c) | bundel `Android Bundled ... (1616 modules)` di log Metro |
| E7 | Tidak ada "Problem loading the project" | **PASSED** | layar masuk tampil; tidak ada `DevLauncherErrorActivity` |
| E8 | Login Screen lengkap | **PASSED** | dump UI: `FaceLink`+`Ai`, tombol `Continue with Google` (`clickable="true"`), teks keterangan, dua label footer |
| E9 | Background video aktif | **PASSED** | media3 `state=PLAYING`, posisi 2941→5960 ms, buffered 8000 ms; dua tangkapan berjarak 3 dtk berbeda (PSNR 24,1 dB) |
| E10 | Safe area | **PASSED** | status bar 0–63, nav bar 2337–2400; isi mulai y=1050 dan berakhir y=2274 |
| E11 | Native module terinisialisasi | **PASSED** | `NitroGoogleSignin C++ library` loaded; `HybridObjectRegistry` mendaftarkan `GoogleSignInButton`; `libexpo-modules-core.so` dan `libexpo-modules-jsi.so` dimuat dari `lib/x86_64/` |
| E12 | Tanpa crash/ANR | **PASSED** | `FATAL EXCEPTION`/ANR untuk `ai.facelink.mobile` = **0**; proses bertahan setelah `force-stop` + buka ulang |
| E13 | Tombol masuk menolak tanpa OAuth | **PASSED** | tekan tombol → "Aplikasi belum dikonfigurasi"; **tidak ada** permintaan jaringan ke backend |
| E14 | Route lintas-peran tanpa sesi | **PASSED** | `facelinkai:///admin` tanpa sesi → tetap layar masuk (tidak ada dashboard) |
| E15 | Unit test, typecheck, lint | **PASSED** | Vitest **80/80**, `tsc --noEmit` exit 0, `expo lint` exit 0 |
| E16 | Google Authentication nyata | **BLOCKED** | Web/Android OAuth client belum ada |

### 15c. Dua masalah nyata yang ditemukan dan akar penyebabnya

**1. Video gagal dengan `Source error` — bukan cacat UI.**

Gejala: layar masuk tampil, tetapi panel tengah gelap; logcat berisi
`ExoPlaybackException: Source error` berulang, dengan rantai penyebab
`java.net.ConnectException: Failed to connect to /127.0.0.1:8082`.

Akar masalah: **`adb reverse` hilang** ketika ADB melakukan reconnect
(`transport_id` berubah 3→4→6). Setelah tunnel mati, aset video yang disajikan
Metro tidak dapat diambil.

Perbaikan yang terbukti: arahkan aplikasi ke alias host emulator
**`10.0.2.2`** alih-alih `127.0.0.1`:

```bash
adb shell am start -a android.intent.action.VIEW \
  -d "exp+facelink-ai://expo-development-client/?url=http%3A%2F%2F10.0.2.2%3A8082" \
  ai.facelink.mobile
```

Setelah itu media3 melaporkan `state=PLAYING` dan tidak ada lagi
`ExoPlaybackException` pada cold start berikutnya. `10.0.2.2` juga sudah
diizinkan oleh `src/features/api/config.ts` (alias host emulator), sehingga
aturan keamanan D-035 tidak dilanggar.

**2. Metro hanya mengikat IPv6 bila dimulai dengan `--localhost`.**

`npx expo start --localhost` membuat listener hanya pada `[::1]:8082`; `curl`
ke `127.0.0.1:8082` menjawab **200 dengan badan 0 byte** sehingga tampak
"berhasil" padahal permintaan tidak pernah sampai ke Metro. Tanpa `--localhost`,
Metro mengikat `0.0.0.0` dan `[::]`, dan manifes berukuran 2.426 B.

### 15d. Peringatan yang bukan kegagalan

| Pesan | Arti |
|---|---|
| `ExpoVideo: Current activity does not support picture-in-picture` | aplikasi tidak memakai PiP; tidak memengaruhi pemutaran |
| `ViewManagerPropertyUpdater: Could not find generated setter ...` | peringatan Fabric untuk view manager pihak ketiga; tidak menghalangi rendering |
| `uiautomator: could not get idle state` / `null root node` | dump UI gagal saat video beranimasi; diulang setelah jeda dan berhasil |
| `MediaPlayerWrapper: tried to update with no new data` | siklus media session saat video berganti klip |

### 15e. Cara menjalankan ulang (urutan yang terbukti)

```powershell
# 1. Metro (JANGAN pakai --localhost; CI harus dihapus)
cd apps/mobile
Remove-Item Env:CI -ErrorAction SilentlyContinue
npx expo start --dev-client --port 8082

# 2. arahkan emulator ke alias host, bukan 127.0.0.1
adb shell am force-stop ai.facelink.mobile
adb shell am start -a android.intent.action.VIEW `
  -d "exp+facelink-ai://expo-development-client/?url=http%3A%2F%2F10.0.2.2%3A8082" `
  ai.facelink.mobile

# 3. backend (opsional; hanya bila menguji alur selain login)
adb reverse tcp:8080 tcp:8080
```

`adb reverse` untuk port 8082 **tidak diperlukan** bila memakai `10.0.2.2`,
sehingga emulator tidak terpengaruh saat ADB reconnect.

### 15f. Yang masih BLOCKED

- **Google Sign-In end-to-end**: Web + Android OAuth client belum dibuat (§5).
  Tombol masuk sengaja berhenti di "Aplikasi belum dikonfigurasi" tanpa memanggil
  backend.
- **Keystore perangkat**: belum teruji karena login belum pernah berhasil.
- **Skenario A1–A12** (§8): seluruhnya menuntut kredensial OAuth.

## 16. FL-ANDROID02D — Audit kesiapan login nyata (22 Oktober 2026)

Tahap ini **audit kesiapan + verifikasi prasyarat**, bukan pengujian login
berhasil. Login nyata **BLOCKED** oleh satu sebab yang terukur: emulator tidak
memiliki akun Google. Tidak ada perubahan kode, UI, atau konfigurasi Windows.

### 16a. Prasyarat konfigurasi

| # | Butir | Hasil | Bukti |
|---|---|---|---|
| P1 | `.env` terbaca Expo | **PASSED** | Metro mencetak `env: load .env`; bundel 7.347.239 B memuat client ID dan `http://10.0.2.2:8080` |
| P2 | `.env` tidak ter-commit | **PASSED** | `git check-ignore -v` → `.gitignore:18:*.env`; `git ls-files apps/mobile/.env` → tidak ada |
| P3 | `.env.example` tetap generik | **PASSED** | nilai client ID **kosong**; tidak memuat ID nyata; URL tetap contoh |
| P4 | Client ID mobile = backend | **PASSED** | backend mencetak `google_oauth: "configured (1 client id)"` dan tidak lagi mencetak WARN "belum dikonfigurasi" |
| P5 | Package Android | **PASSED** | `applicationId 'ai.facelink.mobile'` = yang didaftarkan di OAuth client |
| P6 | SHA-1 APK = yang terdaftar | **PASSED** | `apksigner` → `5E8F16062EA3CD2C4A0D547876BAA6F38CABF625`, identik dengan catatan §5 |
| P7 | ABI APK = emulator | **PASSED** | APK hanya `lib/x86_64/`; perangkat x86_64 |
| P8 | Play Services tersedia | **PASSED** | `com.google.android.gms` 26.40.32 dan `com.android.vending` terpasang |

### 16b. Audit transport (D-035)

| Pemeriksaan | Hasil |
|---|---|
| Bypass TLS di aplikasi | **tidak ada** — `InsecureSkipVerify`, `rejectUnauthorized:false`, `NODE_TLS_REJECT_UNAUTHORIZED` = 0 kemunculan |
| Bypass TLS di backend | **tidak ada** — 0 kemunculan `InsecureSkipVerify` |
| Pemakaian `http://` | **hanya** untuk `127.0.0.1`, `localhost`, `::1`, dan `10.0.2.2` (daftar putih `config.ts`); `https:` selalu diterima; alamat LAN dengan `http:` **ditolak** |
| Backend mendengarkan di luar loopback | **ditolak** — `FACELINK_ADDR=0.0.0.0:8080` → `-check-config` exit 1 dengan alasan "di luar loopback; MVP hanya boleh mendengarkan 127.0.0.1 atau localhost" |
| Client ID tidak sah | **ditolak** — `FACELINK_GOOGLE_CLIENT_IDS=bukan-client-id` → exit 1 "bukan client ID Google yang dikenal" |
| Token/cookie di log | **bersih** — 0 kemunculan JWT/`id_token`/`access_token`/`refresh_token` di logcat; backend tidak mencatat body maupun header otorisasi |
| Token tersimpan | `expo-secure-store` (Keystore), belum teruji karena login belum pernah berhasil |

**Kesimpulan transport:** jalur emulator→backend memakai loopback host lewat
alias `10.0.2.2`, sehingga token tidak pernah melintasi jaringan fisik. Tidak ada
bypass keamanan yang dipakai.

### 16c. Bukti bahwa aplikasi benar-benar menjangkau backend

`POST /api/v1/auth/nonce` dari aplikasi di emulator **berhasil**:

```
{"method":"POST","path":"/api/v1/auth/nonce","status":200,"duration_ms":2}
```

Permintaan ini terjadi pada penekanan tombol masuk di emulator, dengan
`EXPO_PUBLIC_FACELINK_API_BASE_URL=http://10.0.2.2:8080` dan **tanpa**
`adb reverse tcp:8080`. Artinya jalur aplikasi→backend sudah berfungsi; yang
gagal adalah langkah Google berikutnya.

### 16d. BLOCKED: emulator tidak memiliki akun Google

```
adb shell dumpsys account  →  Accounts: 0
```

Rantai kejadian yang terbukti:

1. Aplikasi meminta nonce → **200** (backend sehat).
2. `CredentialManager` memulai `getCredential` untuk `GoogleIdService`.
3. Status menjadi **`CANCELED`, source `REMOTE_PROVIDER`**, lalu
   `GetCredentialResponse error returned from framework`.
4. `signIn()` → tanpa hasil → `createAccount()` → juga tanpa akun.
5. Aplikasi menampilkan galat.

**Solusi paling sederhana dan aman:** tambahkan akun Google penguji ke emulator
lewat UI Android, bukan lewat perintah atau bypass:

1. Buka **Settings → Passwords & accounts → Add account → Google**.
2. Masuk dengan akun penguji yang sudah didaftarkan sebagai *Test users*.
3. Selesaikan verifikasi 2 langkah bila diminta.
4. Pastikan `adb shell dumpsys account | grep -c "Account {"` ≥ 1.
5. Jalankan ulang aplikasi, tekan **Continue with Google**.

Jangan menambahkan akun lewat `adb`, `pm`, atau injeksi database: itu melewati
persetujuan Google dan tidak sah sebagai bukti login.

### 16e. Temuan nyata: pesan galat menyesatkan

Galat yang tampil:

> **Masuk gagal** — "Backend menjawab dengan keadaan yang belum dikenal aplikasi
> ini (google_tanpa_id_token)."

Pesan ini **salah menyebut pihak yang gagal**. Kegagalan terjadi **lokal** di
Credential Manager (tidak ada akun), bukan di backend — backend tidak menerima
permintaan `/auth/google` sama sekali pada percobaan itu.

Sebabnya: `signIn()` dan `createAccount()` sama-sama tidak menghasilkan
`success` maupun `cancelled` yang dikenali, sehingga
`apps/mobile/src/features/auth/googleSignIn.ts:171` mengembalikan
`{ kind: "unknown", message: "google_tanpa_id_token" }`, lalu
`apps/mobile/app/index.tsx:259` menampilkan kalimat "Backend menjawab…".

**Rekomendasi perbaikan minimal (belum diterapkan, menunggu persetujuan karena
mengubah teks yang dilihat pengguna):** petakan kasus "tidak ada akun di
perangkat" ke keadaan tersendiri dengan teks "Tidak ada akun Google di perangkat
ini", dan ubah kalimat `kind: "unknown"` agar tidak menyebut backend kecuali
galat memang berasal dari respons HTTP. Ini perbaikan pesan, bukan desain ulang.

### 16f. Temuan operasional: ANR saat memuat bundel

Saat host hanya menyisakan **0,70 GB** RAM, Android menampilkan "FaceLink Ai
isn't responding" dengan alasan `failed to complete startup`. Setelah memilih
**Wait**, aplikasi tetap berjalan normal dan layar masuk tampil. Penyebabnya
tekanan memori host saat Metro menyajikan bundel dev 7,3 MB, bukan crash
aplikasi (`ReactNativeJS` tanpa galat; 0 `FATAL EXCEPTION`).

**Tindakan:** tutup Chrome (61–63 proses) sebelum sesi uji panjang. Target
≥ 3 GB tersedia.

### 16g. Status skenario A1–A12

| Skenario | Status | Penghalang |
|---|---|---|
| A1 masuk berhasil | **BLOCKED** | belum ada akun Google di emulator |
| A2 role dosen | **BLOCKED** | butuh A1 + akun dosen terundang |
| A3 role mahasiswa | **BLOCKED** | butuh A1 + akun mahasiswa terundang |
| A4 akun belum terdaftar | **BLOCKED** | butuh akun Google yang tidak diundang |
| A5 batal | **BLOCKED** | butuh lembar akun tampil (ada akun) |
| A6 SecureStore | **BLOCKED** | butuh login berhasil |
| A7 pemulihan sesi | **BLOCKED** | butuh sesi tersimpan |
| A8 penyegaran | **BLOCKED** | butuh sesi tersimpan |
| A9 logout | **BLOCKED** | butuh login berhasil |
| A10 cabut akses | **BLOCKED** | butuh A1 + admin |
| A11 routing lintas-role | **PASSED (parsial)** | tanpa sesi, `facelinkai:///admin` tetap di layar masuk (§15 butir E14); versi "dengan sesi mahasiswa" butuh login |
| A12 tidak ada demo | **PASSED** | lembar pengaturan/akun tidak menampilkan tombol ganti role; hanya `Continue with Google` |

**Bootstrap Admin: BELUM dijalankan.** `SELECT count(*) FROM users` → **0 baris**.
Perintahnya butuh email pemilik:

```powershell
cd backend
.\facelink-server.exe -bootstrap-admin -email "<alamat-google-admin>"
```

Perintah ini **tidak dijalankan** karena menulis data akun ke database dan
menuntut alamat email nyata pemilik. Tanpa admin, akun dosen/mahasiswa tidak
dapat diundang (`POST /api/v1/users`), sehingga A2, A3, dan A10 tetap tertutup
meskipun akun Google sudah ditambahkan ke emulator.

### 16h. Urutan gate berikutnya

| Gate | Isi | Status |
|---|---|---|
| G1 | tambahkan akun Google penguji ke emulator (UI Android) | **menunggu pemilik** |
| G2 | bootstrap admin dengan `-email` nyata | **menunggu pemilik** |
| G3 | A1: login admin pertama → akun `active` | terkunci G1+G2 |
| G4 | undang `dosen` + `mahasiswa`, jalankan A2–A12 | terkunci G3 |
| G5 | A4 dengan akun yang tidak diundang | terkunci G1 |

**Yang sudah siap dan tidak perlu diulang:** `.env`, client ID sinkron,
package + SHA-1 cocok, APK x86_64 terpasang, Metro stabil di `:8082`,
transport aman, backend menolak bind non-loopback dan client ID palsu, serta
aplikasi terbukti menjangkau backend (`nonce` 200).

## 17. FL-ANDROID02E — Login nyata di atas HTTPS lokal (22 Oktober 2026)

Tahap ini **mengubah kode**: HTTPS loopback di backend, CA development khusus
build debug, dan perbaikan pesan galat Google. Login Google nyata berhasil
diuji di emulator untuk pertama kalinya.

### 17a. Transport HTTPS tervalidasi

| # | Pemeriksaan | Hasil | Bukti |
|---|---|---|---|
| T1 | Backend dapat menyajikan HTTPS | **PASSED** | `FACELINK_TLS_CERT` + `FACELINK_TLS_KEY` → log `"tls":"on"`, `"skema":"https"` |
| T2 | Sertifikat tervalidasi terhadap CA | **PASSED** | `openssl s_client -CAfile dev-ca.pem` → **`Verify return code: 0 (ok)`** |
| T3 | Tanpa CA, handshake ditolak | **PASSED** | `curl` tanpa `--cacert` → exit **60** (sertifikat tidak tepercaya) |
| T4 | Backend tetap loopback-only | **PASSED** | `validateAddr` tidak diubah; `FACELINK_ADDR=0.0.0.0:8080` tetap **exit 1** |
| T5 | Tidak ada bypass TLS | **PASSED** | 0 kemunculan `InsecureSkipVerify`/`rejectUnauthorized:false` di TS maupun Go |
| T6 | CA hanya pada build debug | **PASSED** | `res/raw/dev_ca.pem` + `network_security_config.xml` hanya di `src/debug/`; manifest utama tanpa `networkSecurityConfig` |
| T7 | Peer certificate cocok dengan kunci | **PASSED** | MD5 public key sertifikat == kunci privat (`838d613b…`) |
| T8 | SAN mencakup emulator | **PASSED** | `IP:127.0.0.1`, `IP:10.0.2.2`, `DNS:localhost` |
| T9 | Kunci privat tidak masuk Git | **PASSED** | `.gitignore:22:*.pem` untuk `dev-ca-key.pem`, `server-key.pem`, `server.pem` |
| T10 | Signing identity APK tidak berubah | **PASSED** | `apksigner` → `5E:8F:16:…:F6:25`, identik setelah prebuild ulang |

**Keputusan desain:** CA lokal + `network_security_config` dipilih alih-alih
reverse proxy karena tidak menambah proses yang harus diamankan. Rinciannya
dicatat sebagai jawaban Q-25 di `docs/DECISIONS.md`.

**Kejujuran batas:** sertifikat dev berlaku 397 hari tanpa rotasi otomatis;
jalur LAN untuk HP nyata **tidak** diuji dan tidak diklaim berfungsi; build
release memakai kebijakan bawaan Android (tanpa CA dev) dan belum diuji
terhadap backend HTTPS.

### 17b. Perbaikan pesan galat yang menyesatkan

Sebelumnya kegagalan lokal Credential Manager dilaporkan sebagai
*"Backend menjawab dengan keadaan yang belum dikenal…"* padahal backend tidak
pernah menerima permintaan. Sekarang:

- `googleSignIn.ts` mengembalikan `kind: "googleDevice"` ketika Credential
  Manager tidak menghasilkan kredensial, dan `"unknown"` hanya untuk bentuk
  respons yang benar-benar tak terduga.
- `index.tsx` menampilkan **"Tidak ada akun Google"** dengan petunjuk menambah
  akun lewat Pengaturan Android.
- Kalimat `kind: "unknown"` diubah menjadi *"Aplikasi menerima keadaan yang
  belum dikenal"* — tidak lagi menyalahkan backend.

**Terverifikasi di perangkat:** pesan baru tampil pada kondisi kesiapan login
yang belum terpenuhi; tidak disamakan dengan galat backend.

### 17c. Hasil A1–A12

| # | Skenario | Hasil | Bukti |
|---|---|---|---|
| A1 | Masuk berhasil (admin) | **PASSED** | Login dan navigasi role berhasil; rincian status akun/last-login tidak dipublikasikan |
| A2 | Role dosen | **BLOCKED** | butuh akun Google dosen yang diundang — hanya satu akun penguji |
| A3 | Role mahasiswa | **BLOCKED** | idem |
| A4 | Akun belum terdaftar | **BLOCKED** | butuh akun Google kedua yang tidak diundang |
| A5 | Batal | **NOT VERIFIED** | Percobaan berulang kali berakhir `Remote provider response timed out` (GMS memberi batas ±3 dtk) sebelum lembar akun muncul, sehingga **pembatalan oleh pengguna tidak pernah terjadi**. Tidak diklaim lulus. Layar kembali ke keadaan semula tanpa pesan galat, tetapi itu akibat timeout, bukan pembatalan |
| A6 | SecureStore (tutup paksa lalu buka) | **PASSED** | `am force-stop` → buka → langsung beranda Administrator tanpa memilih akun |
| A7 | Pemulihan saat backend mati | **PASSED** | backend dimatikan → "Backend tidak terjangkau" + "Coba lagi"; sesi tidak hilang |
| A8 | Penyegaran token | **PARTIAL** | backend hidup lagi → `GET /me` **200** (access token masih valid 60 menit). Jalur refresh belum terpicu, jadi `auth.session.refreshed` **belum** teruji |
| A9 | Logout | **PASSED** | Keluar kembali ke layar masuk; hasil pemeriksaan backend diringkas tanpa riwayat sesi per akun |
| A10 | Cabut akses (suspend) | **BLOCKED** | butuh admin kedua/dosen; `PATCH /users/{id}` belum diuji dari UI |
| A11 | Routing lintas-role | **PASSED (parsial)** | `facelinkai:///admin` tanpa sesi → tetap layar masuk. Versi "sesi mahasiswa dialihkan" butuh akun mahasiswa |
| A12 | Tidak ada demo | **PASSED** | panel pengaturan: 0 kemunculan `demo`, `ganti role`, `Dosen`, `Mahasiswa` |

**Hasil audit historis:** pemeriksaan lokal mendukung kesimpulan login/logout
di atas. Urutan event, jumlah kegagalan dan transisi status akun nyata tidak
dipublikasikan; observasi ini tidak diganti dengan fixture sintetis.

Tidak ada token, ID token, atau email di log: `logcat` → **0** kemunculan
`eyJhbGciOi`/`refresh_token`/`access_token`.

### 17d. Temuan nyata: error lifecycle `expo-video`

LogBox menampilkan (non-fatal, aplikasi tetap berjalan):

```
Console Error
Cannot set prop 'player' on view 'class expo.modules.video.TextureVideoView'
→ Caused by: Cannot use shared object that was already released.
```

**Hipotesis, belum root cause terbukti:** lifetime `useVideoPlayer` dan teardown
`VideoView` mungkin bertumpang tindih saat unmount. Log di atas tidak membuktikan
urutan release native. Tidak ada crash yang teramati saat temuan lama, tetapi
itu bukan jaminan bahwa dampaknya selalu kosmetik.

**Status FL-ENR02A-R2 (26 Oktober 2026): FIXED untuk jalur startup yang dapat
direproduksi.** Video login tidak lagi dipasang selama sesi masih dipulihkan;
A/B TECNO LJ7 menunjukkan warning pra-fix dan tidak ada warning pasca-fix pada
urutan restart yang sama. Tidak ada FATAL. Root cause lain di luar urutan
mount→redirect belum diinstrumentasi. Detail: `FL_ENR02A_R2_VIDEO_ACCESSIBILITY_REPORT.md`.

**Catatan historis:** perbaikan ini menunggu reproduksi yang dapat diandalkan;
hasil upaya FL-ANDROID02F dicatat di §18. Root cause lain di luar
mount→redirect belum dibuktikan.

### 17e. Kendala build yang ditemukan dan solusinya

| Gejala | Sebab | Solusi yang dipakai |
|---|---|---|
| `ninja: error: mkdir(...): No such file or directory` | Path objek CMake mencapai **260 karakter** (batas MAX_PATH) dengan CMake 3.22.1 + ninja 1.10.2 | Pakai `cmake.dir=…/cmake/3.31.6` (**ninja 1.12.1**, long-path aware) — sesuai §7 yang sudah menetapkan versi itu |
| `compressDebugAssets … already contains entry` | Sisa build lintas drive (`C:` vs junction) | Hapus `android/app/build` lalu build ulang |
| `TLS handshake error … EOF` di log | Probe `openssl`/`curl` tanpa CA, bukan aplikasi | Tidak ada tindakan; probe sengaja |

### 17f. Bootstrap Admin (dijalankan sekali)

Dikonfirmasi pemilik sebelum eksekusi. Hasil:

Bootstrap pertama berhasil; output UUID/status akun tidak dipublikasikan.

Eksekusi kedua **ditolak** (`exit 1`, "sudah ada akun admin dengan status
invited atau active") — penjaga `COUNT(*)` di dalam transaksi bekerja.
Database yang dipakai: `backend/data/facelink.db`.

### 17g. Gate kualitas

| Pemeriksaan | Hasil |
|---|---|
| Vitest | **82/82 lulus** (2 tes regresi baru untuk `googleDevice`) |
| `tsc --noEmit` | exit 0 |
| `expo lint` | exit 0 |
| `go test ./...` | **11 paket lulus** (termasuk 4 kasus TLS baru di `internal/config`) |

### 17h. Yang masih terbuka

1. **A2/A3/A4/A10** — butuh 2–3 akun Google tambahan yang diundang sebagai
   `dosen`/`mahasiswa`, dan satu akun yang **tidak** diundang untuk A4.
2. **A8 penuh** — clock injection pada database uji membuktikan expiry/rotation;
   bukti refresh nyata melalui SecureStore emulator masih parsial. Jangan
   menggeser jam perangkat: emulator Play Store tidak perlu diganti/root.
   Image `google_apis` tidak otomatis berarti tanpa Google Play Services.
3. **Error lifecycle `expo-video`** — dikenal, root cause belum terbukti (§17d/§18).
4. **Rotasi sertifikat dev** — manual, belum otomatis.

## 18. FL-ANDROID02F — Verifikasi lanjutan (22 Oktober 2026)

Cabang tetap `feat/fl-ui02-google-auth-android`; tanpa merge/force push,
perubahan desain, akun palsu, bootstrap ulang, atau perubahan TTL production.
Hasil 02E yang diterima pemilik tidak dijalankan ulang sebagai audit lengkap.

### 18a. Status A1–A12

| Skenario | Status E2E | Bukti / batas |
|---|---|---|
| A1 Administrator | PASSED | Bukti login Google nyata 02E diterima pemilik; role backend, bukan role demo |
| A2 Dosen | BLOCKED | Belum ada akun Google tambahan yang disetujui dan diundang |
| A3 Mahasiswa | BLOCKED | Sama; tidak mengubah Admin menjadi mahasiswa |
| A4 Tidak diundang | BLOCKED | Perlu Google OAuth Test User sah yang tidak ada dalam allowlist FaceLink |
| A5 Pembatalan | PASSED | Chooser `com.google.android.gms/.identitycredentials.ui.CredentialChooserActivity` benar-benar tampil; `adb shell input keyevent KEYCODE_BACK` menutup chooser; MainActivity kembali, tombol Google terlihat, tanpa error perangkat/Coba Lagi. Ini pembatalan UI native terotomasi, bukan timeout atau pembatalan manual pemilik |
| A6 SecureStore | PASSED | Force-stop/restore nyata 02E; regresi 02F memakai tiruan storage, bukan bukti native tambahan |
| A7 Pemulihan offline | PASSED | Bukti 02E; regresi 02F memeriksa seluruh sesi persisten tetap utuh saat network error |
| A8 Expiry + refresh | PARTIAL | Clock server terisolasi + HTTP router nyata: expiry → 401, refresh → token baru, `/me` → 200. Native SecureStore setelah expiry belum dibuktikan |
| A9 Logout | PASSED | Logout nyata 02E; regresi 02F memastikan refresh lama ditolak sesudah logout |
| A10 Suspended/revocation | BLOCKED | Otomatis backend lulus; E2E butuh akun kedua. Admin aktif terakhir tidak dinonaktifkan |
| A11 Routing lintas-role | PARTIAL | Bukti 02E bukan sesi mahasiswa nyata; perlu A3 dahulu |
| A12 Tanpa demo | PASSED | Bukti 02E tetap berlaku |

Undangan sah setelah alamat/izin penguji tersedia: Admin memanggil
`POST /api/v1/users` melalui HTTPS dengan `email`, `display_name`, dan
`role=dosen` atau `mahasiswa`. Backend membuat `invited`, audit `user.invited`;
login Google pertama menautkan identitas dan mengaktifkan akun. Validasi
`GET /me`, role/status database, dan akses route backend. Akun A4 sengaja
tidak diundang; jangan membuat identitas/token Google tiruan di database nyata.
Token Admin tidak dicetak atau ditanam pada command history. Tidak ada undangan
baru yang dieksekusi pada checkpoint ini.

### 18b. Regresi sesi dan perbaikan transport

- Memperkuat `TestPenyegaranMemutarToken`: advance `clock.Fake` 61 menit,
  expiry access, rotasi pasangan, identitas/role tetap, access baru valid.
- Concurrent refresh: tepat satu menang, replay mencabut sesi, pasangan pemenang
  juga tidak dapat dipakai lagi. Tidak memperlemah deteksi replay.
- Refresh setelah logout ditolak. Restore dengan `/me` yang mencabut sesi
  menghapus storage dan access dalam memori; network error mempertahankan sesi.
- Ditemukan klien auth menerima HTTP loopback/10.0.2.2. Dua tes gagal sebelum
  perbaikan; kini seluruh endpoint auth menolak HTTP sebelum `fetch`.
  Diagnostik tanpa token dan Metro tetap boleh HTTP sesuai batas sebelumnya.
- Judul `googleDevice` diganti menjadi “Pemilihan akun Google belum berhasil”,
  bukan menyimpulkan akun tidak ada. Tidak mengubah layout/desain.

### 18c. Transport, release, dan rahasia

- Server smoke: `127.0.0.1:8080`, TLS aktif, database yang sama,
  migrasi terapan baru **0**. Aplikasi Android meminta `/auth/nonce` **200**
  pada 19:43:04 dan 19:51:08 WIB, sebelum chooser; tidak ada `/auth/google`
  dalam service log pengujian ini.
- OpenSSL dengan CA lokal + `-verify_ip 10.0.2.2`: **verify 0**.
  CA sama + hostname salah: **verify 62 (hostname mismatch)**.
  Tanpa CA lokal: **verify 20 (unable to get local issuer certificate)**.
  Dua hasil negatif adalah penolakan yang diharapkan, bukan bypass TLS.
  Ini bukti klien OpenSSL, bukan negative-hostname test pada Android.
- Plugin menulis CA/NSC hanya ke `src/debug`. APK debug yang tersedia memuat
  **1** resource `dev_ca`; scan marker private key / Google Client Secret:
  **0** entry. File Git terlacak: **0** marker private key/`GOCSPX-`, **0** `.env`.
  Scan logcat: **0** pola JWT dan **0** nilai token pada field auth yang dipindai.
  Scan berbasis pola tidak membuktikan ketiadaan semua bentuk secret.
- **PARTIAL release:** tidak ada APK release untuk dibongkar. Isolasi source set
  bukan bukti isi release APK. Build native/release baru ditunda karena RAM bebas
  turun 1,59 → 0,45 GB dan kemudian 0,73 GB; tidak menjalankan build berat.
  Perubahan runtime hanya TS/teks, tidak memerlukan perubahan native dependency.
- **Risiko debug:** CA dev di `base-config` dipercaya untuk semua domain pada
  debug; cleartext diizinkan untuk host lokal demi Metro. Klien auth kini tetap
  mewajibkan HTTPS. Release tidak boleh mewarisi trust debug.
- **BLOCKED:** rotasi Web OAuth Client Secret di Google Cloud belum ada buktinya.
  Tidak meminta nilai secret baru; perlu konfirmasi pemilik bahwa secret lama
  sudah dicabut/diganti. Tidak dapat disimpulkan dari kebersihan repository.

### 18d. Video dan batas reproduksi native

Layar Login dan latar video terlihat pada emulator. Background/foreground,
force-stop/relaunch, serta deep link `/dev-preview` → `/` dicoba. Percobaan
awal membuat ADB sementara offline; percobaan sesudah relaunch sempat masih
di splash ketika deep link dikirim. Karena itu urutan tersebut **tidak**
membuktikan unmount VideoView saat navigasi. Scan logcat sesi ini menemukan
**0** `already released` / `Cannot set prop player`.
Tidak ada login→dashboard baru dalam checkpoint ini; reproduksi lengkap belum
tercapai. Issue 02E tetap **known issue, belum diperbaiki**, root cause belum
terbukti; tidak menerapkan patch lifecycle spekulatif.

Emulator ditemukan memakai `auto_time=0` dari eksperimen sebelumnya.
Dikembalikan ke `auto_time=1` dan dibaca ulang **1**. Expiry test tidak lagi
mengubah jam Android. Tidak mengubah Windows/BIOS, menghapus aplikasi/data,
atau menghentikan pekerjaan pengguna.

### 18e. Pemeriksaan dan keputusan review

| Pemeriksaan | Hasil aktual |
|---|---|
| `go test -count=1 ./...` | PASSED, 11 paket uji; `cmd/server` dan `clock` tanpa tes |
| Vitest akhir | PASSED, **85/85**, 7 berkas |
| `npm run typecheck` | PASSED, exit 0 |
| `npm run lint` | PASSED, exit 0 |
| Native smoke | Login/video terlihat, `/auth/nonce` HTTPS 200, pembatalan chooser nyata kembali ke login |
| APK Android baru / audit release APK | BLOCKED pada checkpoint ini oleh tekanan RAM; APK debug 02E tetap dipakai dengan JS dari Metro 8082 |

Layak **draft PR untuk review kode**, belum layak dinyatakan FL-UI02 selesai
atau siap merge/release. Gate tersisa: akun nyata A2–A4/A10/A11, A8 native,
reproduksi video login/unmount, negative TLS Android + release APK tanpa CA dev,
dan konfirmasi pencabutan secret lama. Tidak membuat/merge PR otomatis.

Checkpoint disimpan sebagai perubahan working tree; belum commit/push.
Metro dan backend smoke yang dimulai pada sesi ini dihentikan sesudah pengujian;
tidak ada listener 8080/8082 teramati pada pemeriksaan akhir. Emulator/ADB dan
aplikasi/data pengguna tidak dihapus. Screenshot/hierarchy sementara buatan
sesi ini dibersihkan dari folder TEMP dan `/sdcard`; tidak dimasukkan Git.

## 19. FL-OTP01 — Email OTP di Android (23 Oktober 2026)

Branch `feat/fl-otp01-email-auth`. Login utama kini Email OTP; Google tetap ada tetapi tombolnya disabled. Kontrak keamanan dan prosedur Admin: `docs/EMAIL_OTP.md`.

**Perubahan UI:** `app/index.tsx` menampilkan **dua** tombol — `Continue with FaceLink` aktif dan `Continue with Google · Segera hadir` yang benar-benar disabled (`onPress` tidak dipasang, `accessibilityState.disabled`). Layar `app/email-login.tsx` memakai komponen, video, gradient, logo, tema, dan spacing yang sudah ada; tidak ada redesign. Layar email → OTP memakai `EmailOtpFlow` (single-flight, deadline dari respons server, resend cooldown, abaikan respons layar lama), keyboard email/number-pad, `one-time-code` autofill, label aksesibilitas, dan pemulihan countdown saat aplikasi kembali aktif.

**Bukti otomatis:** Vitest 106/106, `tsc --noEmit`, dan `expo lint` lulus. Tes mencakup request/verify HTTPS, 429 `Retry-After`, challenge/expiry, kode salah, jaringan putus, respons terlambat setelah Back, penyimpanan sesi gagal, serta Google tetap utuh.

**Bukti runtime backend (bukan Android):** binary nyata dengan TLS + DB **sementara** (bukan `backend/data/facelink.db`) memakai DPAPI key; `email_otp:true`; permintaan akun tak dikenal dijawab 202 identik; worker SMTP gagal menandai `failed`; verifier 32 byte; log tanpa email/kode/secret. Tanpa konfigurasi SMTP, endpoint menjawab 503.

**Android native: TERHAMBAT.** Saat smoke, host hanya memiliki **0,26 GB** RAM bebas. Aplikasi berhasil memulai proses tetapi tidak menyelesaikan startup dalam batas ANR: logcat mencatat `ANR in ai.facelink.mobile` dan `Killing … (adj 0): bg anr` sebelum layar React tampil. Itu **tidak** membuktikan bug UI, dan juga **tidak** membuktikan UI berjalan. Tidak ada build native baru; APK debug lama dipakai dengan Metro 8082. Emulator dan data pengguna tidak dihapus.

**Belum dibuktikan (BLOCKED):** login OTP sukses di emulator, penerimaan email Cloudflare, delay/penerimaan Gmail, login Admin/Dosen/Mahasiswa dengan email resmi, dan role routing native dari sesi OTP. Semua memerlukan SMTP Gmail terotorisasi + RAM host yang cukup. Jangan menganggap 202 sebagai bukti email terkirim. Tidak ada OTP tetap, print-code, atau bypass yang ditambahkan.

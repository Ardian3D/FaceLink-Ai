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
(`Ardian's`), sedangkan `tar` (bawaan Windows 10+) menerimanya.

Setel `JAVA_HOME` pada lingkup **User** (tidak menuntut administrator), lalu buka
terminal baru:

```powershell
[Environment]::SetEnvironmentVariable('JAVA_HOME',
  "$env:LOCALAPPDATA\facelink-toolchain\jdk\jdk-17.0.20.1+1", 'User')
```

**Alternatif installer** (juga tanpa biaya):
`winget install --id EclipseAdoptium.Temurin.17.JDK` atau
`winget install --id Microsoft.OpenJDK.17`; keduanya menulis ke
`C:\Program Files` sehingga menuntut Administrator.

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
sdk.dir=C:/Users/<anda>/AppData/Local/Android/Sdk
cmake.dir=C:/Users/<anda>/AppData/Local/Android/Sdk/cmake/3.31.6
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
EXPO_PUBLIC_FACELINK_API_BASE_URL=http://127.0.0.1:8080
EXPO_PUBLIC_FACELINK_GOOGLE_WEB_CLIENT_ID=<Web client ID dari langkah 5>
```

Backend harus menerima client ID yang sama:

```powershell
cd backend
$env:FACELINK_GOOGLE_CLIENT_IDS = "<Web client ID>"
go run ./cmd/server
```

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

**Status skenario A1–A12: BLOCKED** — kredensial OAuth belum dibuat, sehingga
tidak satu pun butir di bawah pernah dijalankan. **Yang SUDAH diuji pada HP
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
| A8 | Penyegaran | Ubah jam perangkat +1 jam, buka aplikasi, nyalakan backend | Sesi pulih; log backend memuat tepat satu `auth.session.refreshed` |
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

## 13. Kelayakan Android Emulator di laptop ini (audit 21 Oktober 2026)

Audit **hanya membaca keadaan**: tidak ada paket yang dipasang dan tidak ada
perubahan sistem. Tujuannya menjawab satu pertanyaan: dapatkah HP fisik diganti
emulator untuk pengujian rutin?

### 13a. Temuan lingkungan (terukur)

| Butir | Nilai nyata | Arti |
|---|---|---|
| CPU | AMD Ryzen 7 170, 8 core / 16 thread | Cukup untuk emulator x86_64 |
| RAM total / bebas | **15,2 GB / 2,1 GB** saat audit | **Kendala utama** — emulator menuntut 2–4 GB khusus untuk dirinya |
| Virtualisasi firmware | `VirtualizationFirmwareEnabled = True` | AMD-V aktif di BIOS |
| Hypervisor berjalan | `HypervisorPresent = True`, `hvhost` + `vmcompute` berjalan | Windows sudah menjalankan hypervisor (VBS/Core Isolation) |
| `HypervisorPlatform` (WHPX) | **InstallState = 2 (Disabled)** | **Android Emulator tidak dapat memakai akselerasi** |
| `VirtualMachinePlatform` | InstallState = 1 (Enabled) | Cukup untuk WSL2/Docker, **bukan** untuk WHPX emulator |
| Hyper-V lengkap | tidak terpasang | — |
| AEHD (Android Emulator Hypervisor Driver) | **tidak terpasang**, service `aehd` tidak ada | Tidak ada akselerasi alternatif |
| Folder `emulator/` | **tidak ada** | Emulator belum diunduh |
| `system-images/` | **tidak ada** | Belum ada image |
| AVD | **0** (`avdmanager list avd` kosong) | — |
| Disk bebas | 221 GB | Cukup (emulator ± 1 GB, image ± 1,5 GB, AVD ± 8 GB) |

### 13b. Kesimpulan: **LAYAK dengan satu hambatan yang butuh keputusan pemilik**

Tanpa akselerasi, emulator x86_64 tetap dapat dijalankan memakai
`-accel off` (emulasi penuh, QEMU TCG). Itu **tidak** cocok untuk pengujian
rutin: boot dapat memakan puluhan menit dan UI menjadi sangat lambat, sehingga
hasil "aplikasi berjalan" tidak mewakili performa nyata.

Akselerasi dapat diperoleh lewat salah satu dari dua jalur, **keduanya menuntut
hak administrator dan restart**:

| Jalur | Yang diubah | Konsekuensi |
|---|---|---|
| **A. Windows Hypervisor Platform (WHPX)** — disarankan | aktifkan fitur `HypervisorPlatform` (dan `VirtualMachinePlatform` yang sudah aktif) | memakai hypervisor yang sudah berjalan; **tidak** menuntut perubahan BIOS; emulator memakai `-accel on` dengan backend WHPX |
| **B. AEHD** | pasang `extras;google;Android_Emulator_Hypervisor_Driver` lalu `sc start aehd` | **tidak** dapat dipakai bersamaan dengan Hyper-V/VBS yang aktif; pada mesin ini VBS sedang berjalan, jadi jalur ini kemungkinan besar ditolak sampai VBS dimatikan |

**Rekomendasi:** jalur **A**. Jalur B berkonflik dengan VBS yang sudah aktif
(Status VBS: *Running*), dan mematikannya mengurangi keamanan sistem demi
emulator.

### 13c. Yang harus diputuskan sebelum instalasi

1. **Izin administrator + restart** untuk mengaktifkan `HypervisorPlatform`.
   Ini perubahan tingkat sistem, berbeda dari pemasangan user-local sebelumnya.
2. **Anggaran RAM.** Mesin ini hanya menyisakan **2,1 GB** bebas dari 15,2 GB.
   Emulator perlu 2–4 GB; tanpa menutup aplikasi lain (Docker Desktop, beberapa
   dev server Node, dan IDE) emulator akan tertukar ke disk. Sebelum menjalankan
   emulator, tutup Docker Desktop dan dev server lain.
3. **Image mana.** Untuk login Google, image harus bertanda **Google Play**
   (`google_apis_playstore`) agar Credential Manager tersedia;
   `google_apis`/AOSP tidak cukup. Android 36.1 tersedia untuk x86_64.
4. **ABI APK.** APK debug yang ada **hanya berisi `lib/arm64-v8a`**, sedangkan
   emulator di laptop ini memakai **x86_64**. Jadi APK itu **tidak dapat
   dipasang di emulator**; perlu build ulang dengan
   `-PreactNativeArchitectures=x86_64` (gradle.properties sudah mencantumkan
   `x86_64` pada `reactNativeArchitectures`, jadi cukup satu flag build).

### 13d. Rencana langkah bila disetujui

```powershell
# 1. (Administrator, sekali, butuh restart) aktifkan WHPX
Enable-WindowsOptionalFeature -Online -FeatureName HypervisorPlatform -All
# 2. unduh emulator + image Play Store x86_64
sdkmanager "emulator" "system-images;android-36.1;google_apis_playstore;x86_64"
# 3. buat AVD
avdmanager create avd -n facelink -k "system-images;android-36.1;google_apis_playstore;x86_64" -d pixel_7
# 4. verifikasi akselerasi SEBELUM boot
emulator -accel-check
# 5. APK x86_64 (wajib; APK arm64 yang ada tidak cocok)
cd apps/mobile/android
.\gradlew assembleDebug -PreactNativeArchitectures=x86_64 --console=plain
# 6. jalankan
emulator -avd facelink
adb install -r app/build/outputs/apk/debug/app-debug.apk
adb reverse tcp:8082 tcp:8082     # Metro; 8080 untuk backend
```

**Catatan penting yang tidak berubah:** emulator **tidak** menghapus kebutuhan
HP fisik untuk verifikasi akhir. Emulator memakai GPU dan sensor tiruan, tidak
memuat Play Services sungguhan pada sebagian image, dan tidak membuktikan
perilaku perangkat nyata (kamera, sensor, Keystore perangkat). Untuk FaceLink AI
— yang kelak memakai kamera dan penyimpanan kredensial — HP fisik tetap menjadi
bukti akhir; emulator hanya mempercepat iterasi UI dan alur non-kamera.

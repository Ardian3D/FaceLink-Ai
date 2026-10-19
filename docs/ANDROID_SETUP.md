# Setup Build Android (FL-ANDROID01)

Panduan menyiapkan JDK 17, Android SDK, ADB, Expo development build, dan Google
OAuth Android untuk FaceLink AI di **Windows**.

> **Status dokumen ini:** seluruh angka versi di bawah **dibaca dari berkas yang
> benar-benar ada di repository ini**, yaitu hasil `npx expo prebuild
> --platform android` atas `apps/mobile` (Expo SDK 58.0.13, React Native 0.88.0).
> Panduan ini **belum pernah dijalankan sampai tuntas** di mesin pengembangan
> saat ini karena JDK, Android SDK, dan ADB belum terpasang. Setiap langkah yang
> belum dijalankan ditandai **BELUM TERVERIFIKASI**.
>
> Dokumen ini tidak meminta pemasangan apa pun secara otomatis: tidak ada skrip
> yang mengunduh atau memasang paket. Semua langkah dijalankan manual oleh
> pemilik project.

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

### Status mesin pengembangan saat panduan ini ditulis (2026-10-20)

Keluaran nyata `npm run check:native`, bukan perkiraan:

| Butir | Status | Keterangan |
|---|---|---|
| JDK 17 | **BLOCKED** | `java` tidak ada pada PATH |
| `JAVA_HOME` | **BLOCKED** | belum disetel |
| `ANDROID_HOME` | **BLOCKED** | belum disetel |
| platform `android-37` | **BLOCKED** | menunggu `ANDROID_HOME` |
| build-tools `37.0.0` | **BLOCKED** | idem |
| CMake | **BLOCKED** | idem; yang diperiksa "ada versi ≥ 3.13", bukan satu nomor |
| `NDK 27.1.12297006` | **BLOCKED** | idem; versi lain tidak memenuhi |
| `adb` | **BLOCKED** | idem |
| perangkat/emulator terhubung | **BLOCKED** | idem |
| AVD emulator | **BLOCKED** | idem |
| `EXPO_PUBLIC_FACELINK_GOOGLE_WEB_CLIENT_ID` | **BLOCKED** | belum ada di environment maupun `.env`; butuh langkah §5 |
| `android/` hasil prebuild | **PASSED** | dibuat 2026-10-20, diabaikan Git |
| keystore debug | **PASSED** | SHA-1 belum diambil (butuh `keytool` dari JDK) |
| ruang disk | **PASSED** | 230 GB bebas (perlu ≥ 15 GB) |

**Ringkas: 3 PASSED, 0 FAILED, 11 BLOCKED.** Tidak ada butir gagal; semuanya
hanya belum terpasang. Karena itu panduan ini **belum terbukti sampai tuntas** —
tidak satu pun skenario §8 pernah dijalankan.

## 2. JDK 17

Pilih **satu** distribusi. Ketiganya tanpa biaya dan tanpa akun berbayar.

| Pilihan | Perintah (PowerShell, Administrator) |
|---|---|
| Eclipse Temurin | `winget install --id EclipseAdoptium.Temurin.17.JDK` |
| Microsoft OpenJDK | `winget install --id Microsoft.OpenJDK.17` |
| Chocolatey | `choco install -y microsoft-openjdk17` |

Lalu setel `JAVA_HOME` **permanen** (bukan hanya sesi ini):

```powershell
$jdk = (Get-ChildItem 'C:\Program Files\Eclipse Adoptium' -Directory | Select-Object -First 1).FullName
[Environment]::SetEnvironmentVariable('JAVA_HOME', $jdk, 'User')
$env:JAVA_HOME = $jdk
java -version
```

Buka terminal baru setelahnya. **JDK yang lebih baru dari 17 tidak
direkomendasikan** oleh React Native; AGP 9.2 menyatakan minimum 17, dan JDK 26
pernah dilaporkan memicu kegagalan `JdkImageTransform` (isu AGP #486844145).

## 3. Android SDK

Dua jalur; pilih satu.

### 3a. Android Studio (paling sedikit langkah manual)

1. Pasang Android Studio, centang **Android SDK**, **Android SDK Platform**, dan
   **Android Virtual Device**.
2. **SDK Manager → SDK Platforms → Show Package Details**, centang
   **Android SDK Platform 37**.
3. **SDK Manager → SDK Tools → Show Package Details**, centang:
   - **Android SDK Build-Tools 37.0.0**
   - **Android SDK Command-line Tools (latest)**
   - **Android SDK Platform-Tools** (berisi `adb`)
   - **NDK (Side by side) 27.1.12297006**
   - **CMake** (versi bawaan yang ditawarkan SDK Manager; minimum 3.13)
   - **Android Emulator**
4. Setel variabel lingkungan:

```powershell
[Environment]::SetEnvironmentVariable('ANDROID_HOME', "$env:LOCALAPPDATA\Android\Sdk", 'User')
$env:ANDROID_HOME = "$env:LOCALAPPDATA\Android\Sdk"
$env:PATH = "$env:ANDROID_HOME\platform-tools;$env:ANDROID_HOME\emulator;$env:PATH"
```

### 3b. Command-line saja (tanpa Android Studio)

Unduh **Command-line tools** dari
<https://developer.android.com/studio#command-line-tools-only>, ekstrak ke
`%LOCALAPPDATA%\Android\Sdk\cmdline-tools\latest`, lalu:

```powershell
sdkmanager --licenses
sdkmanager "platform-tools" "platforms;android-37" "build-tools;37.0.0" `
           "ndk;27.1.12297006" "cmake;3.22.1" "emulator" `
           "system-images;android-36;google_apis_playstore;x86_64"
```

`google_apis_playstore` penting: **Credential Manager menuntut Google Play
Services**, yang hanya ada pada image bertanda *Play Store*, bukan `google_apis`
atau AOSP. Tanpa itu `checkPlayServices()` gagal dengan
`PLAY_SERVICES_NOT_AVAILABLE` dan aplikasi menampilkan
"Google Play Services tidak tersedia".

## 4. Emulator atau perangkat fisik

**Emulator:**

```powershell
avdmanager create avd -n facelink -k "system-images;android-36;google_apis_playstore;x86_64" --device pixel_7
emulator -avd facelink
```

**Perangkat fisik:** aktifkan *Opsi pengembang* → *USB debugging*, sambungkan,
lalu `adb devices` harus menampilkan perangkat berstatus `device`.

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

Ambil SHA-1 keystore debug **setelah JDK terpasang** (pilih salah satu):

```powershell
# A. keytool langsung
keytool -list -v -keystore apps/mobile/android/app/debug.keystore `
        -alias androiddebugkey -storepass android -keypass android |
  Select-String "SHA1:"

# B. lewat Gradle (wajib setelah build pertama berhasil)
cd apps/mobile/android; .\gradlew signingReport
```

Nilai SHA-1 keystore bawaan berbeda di setiap mesin dan **tidak boleh
di-hardcode** di repository. Setelah SHA-1 didaftarkan, Google memerlukan waktu
beberapa menit sampai client ID Android aktif; kegagalan dengan kode
`DEVELOPER_ERROR` biasanya berarti SHA-1/package belum cocok.

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
npx expo run:android                   # build debug + pasang ke perangkat
adb reverse tcp:8080 tcp:8080          # perangkat → backend laptop
npm start                              # Metro; aplikasi dev build menyambung
```

> **Catatan `npm run android`:** skrip itu masih berisi `expo start --android`
> (Expo Go). **Expo Go tidak dapat dipakai** karena `expo-secure-store` dan
> `react-native-nitro-google-signin` adalah modul native. Gunakan
> `npx expo run:android`. Mengubah skrip itu menjadi `expo run:android` adalah
> keputusan pemilik, bukan perubahan yang dilakukan otomatis oleh dokumen ini.

Build pertama mengunduh Gradle 9.4.1 + dependency Maven dan mengompilasi C++
Nitro untuk empat ABI; perkirakan 10–20 menit dan beberapa GB disk. Untuk
mempercepat, batasi ke satu ABI: `.\gradlew assembleDebug -PreactNativeArchitectures=x86_64`.

## 8. Urutan pengujian setelah build terpasang

Semua langkah menuntut akun uji terdaftar (lihat bagian 9).

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

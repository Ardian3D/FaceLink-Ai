# FaceLink AI — Aplikasi Mobile (Expo)

Aplikasi klien React Native + Expo untuk tiga role manusia: `admin`, `dosen`, dan
`mahasiswa`. Target utama **Android**; web dipakai hanya sebagai pratinjau tata
letak dan **tidak** dapat melakukan Google Sign-In.

> **Status: FL-UI02.** Masuk Google sungguhan ke Go Backend sudah terpasang —
> nonce dari backend, Google Credential Manager, pertukaran ID token, token sesi
> di penyimpanan kredensial platform, dan pemulihan sesi yang diverifikasi ulang
> ke backend. Belum ada fitur dashboard yang memakai API; seluruh angka di
> dashboard masih **data contoh** (`src/features/dashboard/mockData.ts`).

## Prasyarat

- Node.js 20 atau lebih baru (diuji pada Node 24).
- npm (proyek ini memakai `npm`, bukan yarn/pnpm).
- Go 1.22+ bila menjalankan backend lokal.
- Untuk Android: **development build** (lihat bawah). Expo Go **tidak** dapat
  dipakai lagi: Google Credential Manager adalah modul native.
- **JDK 17** dan **Android SDK** (`ANDROID_HOME` + `adb`) untuk `expo prebuild`
  dan build native. Ketiganya **belum ada** di mesin pengembangan ini, jadi
  langkah itu belum pernah dijalankan.
- Tidak perlu WSL, Docker, atau layanan berbayar.

## Menjalankan

```bash
cd apps/mobile
npm install
cp .env.example .env    # isi alamat backend + Web OAuth client ID
npm start
```

Pemeriksaan mutu:

```bash
npm run typecheck   # tsc --noEmit, TypeScript strict
npm run lint        # eslint-config-expo
npm test            # vitest: pengujian autentikasi (tanpa perangkat Android)
```

Pengujian berjalan di Node dan **tidak** memerlukan JDK, Android SDK, maupun
`adb`. Modul native Google, SecureStore, dan `fetch` diganti tiruan; yang diuji
adalah perilaku (`src/features/auth/authFlow.ts` dan lapisan API), bukan
rendering. Rinciannya di `tests/`.

### Development build (wajib untuk masuk Google)

Panduan lengkap Windows (JDK 17, Android SDK, ADB, emulator, OAuth Android,
urutan pengujian di perangkat): **`docs/ANDROID_SETUP.md`**.

Periksa prasyarat mesin ini kapan saja:

```bash
npm run check:native   # PASSED / FAILED / BLOCKED per komponen
```

```bash
npx expo prebuild        # menghasilkan folder android/
npx expo run:android     # build + pasang ke perangkat/emulator
```

`npm run android` masih menjalankan `expo start --android` (Expo Go) dan **tidak
dapat** dipakai untuk login Google; gunakan `npx expo run:android`.

`adb reverse tcp:8080 tcp:8080` harus dipasang agar perangkat menjangkau backend
loopback di laptop (lihat bagian konektivitas).

**Target native saat ini Android saja.** `react-native-nitro-google-signin`
tidak didaftarkan sebagai config plugin di `app.json`. Dasar teknisnya, diperiksa
pada paket versi **2.3.0** yang terpasang:

- `android/build.gradle` menambahkan sendiri `androidx.credentials:credentials`,
  `credentials-play-services-auth`, dan `play-services-auth`; tidak ada
  ketergantungan pada berkas `google-services.json`.
- `GoogleSignInController.kt` memakai `webClientId` yang diberikan lewat
  `configure()` (`setServerClientId(clientId)`) dan meneruskan nonce dengan
  `setNonce(resolveNonce())`; nilai `"autoDetect"` hanya dipakai bila kita
  memintanya, dan di situlah `default_web_client_id` baru diperlukan.
- Config plugin hanya wajib untuk dua jalur: Firebase/`google-services.json`
  (menghasilkan `default_web_client_id`) dan iOS (`iosUrlScheme`). Keduanya
  tidak dipakai build Android dengan `webClientId` eksplisit.

Build iOS menuntut tambahan yang belum dikonfigurasi — `iosUrlScheme` berisi
*reversed* client ID (`com.googleusercontent.apps.…`) pada opsi plugin. Tanpa itu
ID token tidak dapat diterima dan `masukDenganGoogle` akan gagal, jadi jangan
menganggap iOS sudah siap sebelum plugin itu ditambahkan dan diuji.

**Status: BELUM TERVERIFIKASI pada perangkat.** Pemeriksaan di atas dibaca dari
kode sumber paket, bukan dari hasil build; `expo prebuild` dan `expo run:android`
belum pernah dijalankan pada mesin ini karena JDK, Android SDK, dan `adb` tidak
tersedia.

**Pratinjau web tetap berguna** untuk memeriksa tata letak: `npm run web`. Di
sana tombol masuk berhenti dengan pesan *"Masuk Google tidak tersedia di sini"* —
bukan galat jaringan, dan bukan sesi palsu.

Klip latar layar masuk (opsional, lokal saja):

```bash
npm run video:local   # salin klip dari design-inputs/video/ ke aset lokal
npm run video:off     # hapus salinan lokal; layar masuk memakai latar animasi
```

Klip video **tidak** di-commit karena memuat wajah orang yang dapat dikenali.

## Alur masuk yang berlaku

```
Layar masuk ──(nonce + Google)──→ Layar pemeriksaan peran ──(sesi sah)──→ Beranda role
     ▲                                        │
     │                                        └─(403 IDENTITY_NOT_LINKED)─→ Layar penjelasan
     └────────────────────────────────────────────────────────────────────────┘
                     (gagal / dibatalkan / sesi berakhir)
```

1. **Layar masuk** meminta nonce ke `POST /api/v1/auth/nonce`, lalu membuka
   Google Credential Manager dengan nonce itu sebagai parameter.
2. Setelah pengguna memilih akun, **layar pemeriksaan peran** tampil selama ID
   token ditukar ke `POST /api/v1/auth/google` dan role dibaca dari jawaban
   backend. Ketiga langkah pada layar itu berasal dari kemajuan nyata
   (`tahap` pada `SessionProvider`), bukan penundaan buatan.
3. **Beranda role** dibuka berdasarkan `role` dari backend. Tidak ada pemilih
   role di aplikasi: satu akun hanya punya satu role utama (D-037).
4. **Akun ditolak** (`403 IDENTITY_NOT_LINKED`) diarahkan ke layar penjelasan.
   Kontrak memakai satu jawaban generik untuk "belum diundang", "belum
   ditautkan", dan "ditangguhkan" (anti-enumerasi), jadi aplikasi **tidak**
   membedakan ketiganya.

Pada peluncuran berikutnya, sesi tersimpan diverifikasi ulang lewat `GET /me`
(dengan penyegaran token bila perlu). Token lokal tidak pernah dipercaya sebagai
bukti identitas. **Kegagalan jaringan tidak menghapus token**: layar masuk
menawarkan "Coba lagi" tanpa memaksa memilih akun Google lagi.

Penyegaran token bersifat **satu penerbangan**: permintaan yang datang saat
penyegaran berjalan menunggu hasil yang sama, bukan memakai token penyegar yang
sama dua kali. Ini bukan optimasi kinerja — backend mencabut **seluruh** sesi
pengguna bila token penyegar yang sudah dirotasi dipakai ulang
(`docs/API_CONTRACT.md` §5.1). Perilakunya diuji di
`tests/authFlow.test.ts`.

## Konfigurasi

Berkas `.env` di folder ini (diabaikan Git; contohnya `.env.example`):

| Variabel | Wajib | Keterangan |
|---|---|---|
| `EXPO_PUBLIC_FACELINK_API_BASE_URL` | build produksi | Alamat Go Backend. Bawaan development: `http://127.0.0.1:8080`. |
| `EXPO_PUBLIC_FACELINK_GOOGLE_WEB_CLIENT_ID` | ya, untuk masuk | **Web** OAuth client ID, berakhiran `.apps.googleusercontent.com`. Harus sama dengan salah satu entri `FACELINK_GOOGLE_CLIENT_IDS` backend. |

Aturan alamat backend yang ditegakkan `src/features/api/config.ts`:

| Alamat | Hasil |
|---|---|
| `https://…` | diterima |
| `http://127.0.0.1:8080`, `http://localhost:…`, `http://[::1]:…` | diterima (loopback) |
| `http://10.0.2.2:8080` | diterima (alias host emulator Android) |
| `http://192.168.x.x:…` | **ditolak** — HTTP polos di LAN dilarang D-035 |
| `user:pass@host`, query, atau fragmen | ditolak |

Client ID Android **tidak** dapat dipakai sebagai nilai ini: `aud` pada ID token
Android bukan client ID itu, sehingga backend menjawab `IDENTITY_TOKEN_INVALID`.
Aplikasi memeriksa bentuknya saat modul dimuat dan menolak nilai yang salah
sebelum satu pun permintaan dikirim.

## Menjalankan backend

```bash
cd backend
go run ./cmd/server        # mendengarkan 127.0.0.1:8080
```

Agar `POST /auth/google` melayani, backend memerlukan `FACELINK_GOOGLE_CLIENT_IDS`
(daftar client ID yang diterima sebagai `aud`). Tanpa itu, endpoint menjawab
`503 GOOGLE_UNAVAILABLE` dan aplikasi menampilkan "Layanan masuk belum siap" —
bukan "backend tidak terjangkau". Admin pertama dibuat lewat bootstrap lokal
(`cmd/server -bootstrap-admin`), bukan lewat login Google pertama.

### Perangkat Android via USB

```bash
adb devices                       # pastikan perangkat terdeteksi
adb reverse tcp:8080 tcp:8080     # port perangkat → port laptop
```

`adb reverse` hilang setiap perangkat dicabut atau server ADB di-restart; ulangi
perintahnya. Untuk emulator, `http://10.0.2.2:8080` juga dapat dipakai tanpa
`adb reverse`.

## Diagnostik (development saja)

Lembar pengaturan memuat **Diagnostik (dev)** yang menampilkan:

- alamat backend yang benar-benar dipakai bundel ini beserta sumbernya;
- Web OAuth client ID yang terbaca (atau alasan nilainya tidak sah);
- status sesi saat ini (role dan nama tampilan, **tanpa** token);
- dua permintaan nyata ke `GET /api/v1/healthz` dan `GET /api/v1/readyz`.

Layar ini tidak menampilkan token dan tidak menawarkan jalur masuk alternatif.
Pada build rilis (`__DEV__ === false`) tombolnya tidak ada dan rutenya hanya
menampilkan "Tidak tersedia pada build ini".

`ai_engine: unavailable` **tidak** dianggap kegagalan: mesin AI yang belum siap
tidak menghalangi absensi manual (D-030 butir 10).

> **Catatan web:** backend belum mengirim header CORS, jadi diagnostik **gagal
> di browser** meskipun backend hidup. Ini bukan kerusakan aplikasi: `fetch`
> native Android tidak menerapkan CORS.

## Batas keamanan

- **Tidak ada jalur masuk tanpa verifikasi backend.** Mode demo dan pemilih role
  sudah dihapus; satu-satunya jalur adalah Google → `POST /auth/google`.
- **Token** hanya berada di `expo-secure-store` (Android Keystore / iOS
  Keychain). Tidak ada `AsyncStorage`, tidak ada fallback ke penyimpanan biasa.
  Bila penyimpanan aman tidak tersedia (mis. pratinjau web), sesi **ditolak**
  dengan pesan jelas alih-alih disimpan di tempat yang salah.
- **Google ID token tidak pernah disimpan**, tidak pernah dicatat, dan tidak
  pernah menjadi token otorisasi API FaceLink AI. Satu nonce untuk satu
  percobaan; percobaan ulang meminta nonce baru.
- **Role tidak pernah datang dari klien.** Nilai `role` pada UI berasal dari
  `POST /auth/google` atau `GET /me`; setiap permintaan tetap diotorisasi ulang
  oleh backend.
- **Layar tidak dapat menampilkan token**: `Session` hanya memuat `user`.
- Status `unregistered` tidak pernah membawa sesi, dan layar penjelasannya
  menolak dibuka langsung tanpa status itu.

## Struktur

```
apps/mobile/
├── app/                     # rute Expo Router
│   ├── _layout.tsx          # provider sesi, status bar, tumpukan layar
│   ├── index.tsx            # layar masuk (nonce + Google)
│   ├── role-checking.tsx    # layar pemeriksaan peran (kemajuan nyata)
│   ├── not-registered.tsx   # penjelasan akun ditolak
│   ├── dev-preview.tsx      # diagnostik build (development saja)
│   ├── +not-found.tsx
│   └── (app)/               # area terautentikasi
│       ├── _layout.tsx      # penjaga navigasi: tanpa sesi → layar masuk
│       ├── admin.tsx
│       ├── dosen.tsx
│       └── mahasiswa.tsx
├── src/
│   ├── components/          # komponen UI yang dipakai ulang
│   ├── features/
│   │   ├── api/             # transport HTTP, konfigurasi alamat, diagnostik
│   │   ├── auth/            # sesi, token store, Google Sign-In, klien auth
│   │   └── dashboard/       # blok dashboard dan data contoh
│   ├── theme/               # design tokens (satu sumber kebenaran)
│   └── types/               # tipe bersama
├── scripts/                 # pemasangan klip video lokal (tidak di-commit)
├── metro.config.js          # memilih daftar klip saat bundel dibuat
└── assets/
    ├── branding/            # logo dan ikon aplikasi — DI-COMMIT
    └── video/               # modul daftar klip; berkas .mp4 lokal, tidak di-commit
```

## Aset brand — logo resmi

`assets/branding/` berisi turunan **logo resmi FaceLink AI** yang diberikan
pemilik project. Mark dan ikon aplikasi berasal dari
`design-inputs/logo/logo.png` (latar penuh) dan `logo-white-noBg.png` (latar
dihapus); turunannya di-commit di sini sebagai aset aplikasi:

| Berkas | Isi |
|---|---|
| `logo-mark.png` | mark saja, 512×512, latar transparan — dipakai di layar masuk |
| `icon.png` | ikon aplikasi 1024×1024 |
| `adaptive-icon-foreground.png` | mark untuk ikon adaptif Android |
| `adaptive-icon-monochrome.png` | mark satu warna untuk tema Android |
| `splash-icon.png` | gambar splash |

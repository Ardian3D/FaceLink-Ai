# FaceLink AI — Aplikasi Mobile (Expo)

Aplikasi klien React Native + Expo untuk tiga role manusia: `admin`, `dosen`, dan
`mahasiswa`. Target utama **Android**; web dipakai hanya sebagai jalur preview.

> **Status: FL-UI01 + FL-NET01.** UI lengkap untuk preview dan diagnostik
> koneksi ke Go Backend (development saja). Belum ada Google OAuth sungguhan,
> belum ada sesi yang disimpan, dan belum ada fitur yang memakai API. Seluruh
> angka di dashboard adalah **data contoh**.

## Prasyarat

- Node.js 20 atau lebih baru (diuji pada Node 24).
- npm (proyek ini memakai `npm`, bukan yarn/pnpm).
- Untuk Android: aplikasi **Expo Go** di HP, atau Android Studio + emulator.
- Tidak perlu WSL, Docker, atau layanan berbayar.

## Menjalankan

```bash
cd apps/mobile
npm install
npm start          # buka Expo dev server
```

Setelah dev server berjalan:

- **HP Android (paling cepat):** pasang Expo Go, sambungkan HP dan laptop ke
  Wi-Fi yang sama, lalu pindai QR yang muncul di terminal.
- **Emulator Android:** tekan `a` pada terminal dev server (Android Studio harus
  terpasang lebih dahulu).
- **Browser (hanya untuk preview tata letak):** tekan `w`, atau
  `npm run web` lalu buka `http://localhost:8081`.

Pemeriksaan mutu:

```bash
npm run typecheck   # tsc --noEmit, TypeScript strict
npm run lint        # eslint-config-expo
```

Klip latar layar masuk (opsional, lokal saja):

```bash
npm run video:local   # salin klip dari design-inputs/video/ ke aset lokal
npm run video:off     # hapus salinan lokal; layar masuk memakai latar animasi
```

Klip video **tidak** di-commit karena memuat wajah orang yang dapat dikenali.
Tanpa klip, layar masuk tetap tampil utuh dengan latar animasi yang digambar di
kode. Rinciannya ada di `assets/video/BACA-SAYA.md`.

## Alur preview saat ini

```
Layar masuk  →  Role Checking  →  Beranda role
                     │
                     └─ akun belum terdaftar / ditangguhkan  →  Layar penjelasan
```

- **Layar masuk:** latar video atau animasi pindai, logo resmi FaceLink Ai, satu
  tombol **Continue with Google**, dan label kebijakan di dasar layar. Tidak ada
  tombol "Skip" dan tidak ada jalur masuk lain.
- **Role Checking:** animasi pemindaian dan teks "Sedang mengecek peran anda…".
  Penundaan singkat di layar ini **hanya untuk preview** dan bukan mekanisme
  autentikasi.
- **Beranda role:** admin (ringkasan, kelas aktif, status perangkat, aktivitas,
  pintasan pengelolaan), dosen (kerangka), mahasiswa (kerangka baca-sendiri).
  Perpindahan role saat preview memakai tombol "Ganti role (dev)" di lembar
  pengaturan, bukan layar pemilih role di tengah alur masuk.
- **Layar penjelasan akun ditolak:** dipakai bila backend menjawab bahwa akun
  belum terdaftar (`notGranted`) atau sedang ditangguhkan (`suspended`). Layar
  ini **tidak** memberi sesi, tidak menawarkan "coba lagi", dan hanya memuat
  satu langkah keluar: kembali ke layar masuk. Role ditetapkan admin (D-037),
  jadi mengulang login tidak mengubah keadaan sebelum admin mendaftarkan akun.
  Kedua keadaan itu dapat dipreview dari `dev-preview` lewat tombol
  *Preview akun belum terdaftar* dan *Preview akun ditangguhkan*.

## Menghubungkan aplikasi ke Go Backend (FL-NET01)

Alamat backend **tidak** ditanam di kode. Aplikasi membacanya dari
`EXPO_PUBLIC_FACELINK_API_BASE_URL`, yang di-*inline* saat bundel dibuat:

```bash
cd apps/mobile
cp .env.example .env          # lalu sesuaikan bila perlu
npm start
```

Nilai bawaan development adalah `http://127.0.0.1:8080` (loopback, port yang
sama dengan `config.DefaultAddr` backend). Build produksi **tidak** punya nilai
bawaan: tanpa variabel itu, diagnostik melaporkan alasan dan tidak ada
permintaan yang dikirim.

Aturan yang ditegakkan `src/features/api/config.ts`:

| Alamat | Hasil |
|---|---|
| `https://…` | diterima |
| `http://127.0.0.1:8080`, `http://localhost:…`, `http://[::1]:…` | diterima (loopback) |
| `http://10.0.2.2:8080` | diterima (alias host emulator Android) |
| `http://192.168.x.x:…` | **ditolak** — HTTP polos di LAN dilarang D-035 |
| `user:pass@host`, query, atau fragmen | ditolak |

### Menjalankan backend

```bash
cd backend
go run ./cmd/server        # mendengarkan 127.0.0.1:8080
```

Backend sengaja mengikat **loopback**. Jangan mengubahnya menjadi `0.0.0.0`
hanya agar perangkat Android dapat tersambung; pakai `adb reverse`.

### Perangkat Android via USB

```bash
adb devices                       # pastikan perangkat terdeteksi
adb reverse tcp:8080 tcp:8080     # port perangkat → port laptop
```

Setelah itu `http://127.0.0.1:8080` di dalam aplikasi menunjuk ke backend laptop.
`adb reverse` hilang setiap perangkat dicabut atau server ADB di-restart; ulangi
perintahnya. Untuk emulator, `http://10.0.2.2:8080` juga dapat dipakai tanpa
`adb reverse`.

### Diagnostik koneksi (development saja)

Buka `dev-preview` dari lembar pengaturan (tombol "Ganti role (dev)"), lalu
gulir ke bagian **Koneksi Go Backend**. Komponen
`src/features/api/ConnectionDiagnostics.tsx` mengirim dua permintaan nyata —
`GET /api/v1/healthz` dan `GET /api/v1/readyz` — tanpa login, token, atau data
peserta, dan menyediakan tombol **Periksa ulang**.

Tiga hasil dibedakan, bukan hanya dari warna:

- **tidak terjangkau** — permintaan tidak sampai (backend mati, port salah,
  `adb reverse` belum dipasang);
- **menjawab tetapi galat** — status bukan `200`, atau bentuk badan tidak sesuai
  kontrak;
- **sukses** — `healthz: ok` dan `readyz: ready`.

`ai_engine: unavailable` **tidak** dianggap kegagalan: mesin AI yang belum siap
tidak menghalangi absensi manual (D-030 butir 10, `docs/SECURITY_PRIVACY.md`
§5e). Layar ini tidak dirender pada build produksi; `DEMO_MODE` sudah memaksa
`dev-preview` menampilkan "Tidak tersedia pada build ini" sebelum komponennya
tercapai.

> **Catatan web:** backend belum mengirim header CORS, jadi diagnostik **gagal
> di browser** (`Failed to fetch`) meskipun backend hidup. Ini bukan kerusakan
> aplikasi: `fetch` native Android tidak menerapkan CORS. Preview web tetap
> berguna untuk tata letak, bukan untuk menguji konektivitas.

## Batas keamanan mode demo

Mode demo hanya aktif pada **development build** (`__DEV__`), lihat
`src/features/auth/demoMode.ts`.

- Tidak memanggil backend, tidak menerbitkan token, dan tidak menyimpan
  kredensial apa pun.
- Build produksi menolak tombol masuk dengan pesan "belum tersedia" dan layar
  pemilih role tidak menawarkan pilihan apa pun — **tidak ada** jalur yang
  menaikkan hak akses.
- Layar **tidak** lagi menampilkan spanduk "DEMO" (dihapus atas permintaan
  pemilik project, 18 Oktober 2026, agar preview sama dengan tampilan akhir).
  Karena itu angka pada beranda development build adalah **data contoh** dan
  tidak boleh dikira data nyata; sumbernya `src/features/dashboard/mockData.ts`.
- Status `unregistered` **tidak pernah** membawa sesi. Penjaga navigasi
  mengalihkan ke layar penjelasan, dan layar itu sendiri menolak dibuka langsung
  tanpa status tersebut — jadi tidak ada jalan masuk ke beranda lewat keadaan
  akun yang ditolak.
- Role yang dipilih di preview **tidak pernah** dikirim sebagai klaim ke backend.
  Setelah integrasi, role hanya berasal dari sesi yang divalidasi Go Backend
  (lihat `docs/AUTHORIZATION.md` §5).

## Struktur

```
apps/mobile/
├── app/                     # rute Expo Router
│   ├── _layout.tsx          # provider sesi, status bar, tumpukan layar
│   ├── index.tsx            # layar masuk
│   ├── role-checking.tsx    # layar pemeriksaan peran
│   ├── not-registered.tsx   # penjelasan akun belum terdaftar / ditangguhkan
│   ├── dev-preview.tsx      # pemilih role (development saja)
│   ├── +not-found.tsx
│   └── (app)/               # area terautentikasi
│       ├── _layout.tsx      # penjaga navigasi: tanpa sesi → layar masuk
│       ├── admin.tsx
│       ├── dosen.tsx
│       └── mahasiswa.tsx
├── src/
│   ├── components/          # komponen UI yang dipakai ulang
│   ├── features/
│   │   ├── api/             # klien HTTP + diagnostik koneksi backend (dev)
│   │   ├── auth/            # sesi, mode demo, pemetaan role → rute
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

`assets/branding/` kini berisi turunan **logo resmi FaceLink AI** yang diberikan
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
| `favicon.png` | ikon tab web |

Latar aplikasi, splash, dan ikon adaptif memakai **hitam pekat** (`#000000`),
sesuai latar logo resmi. Aksen merah `#FF171B` diambil langsung dari piksel logo
dan menjadi satu-satunya warna merek di `src/theme/tokens.ts`.

Wordmark "FaceLink Ai" pada layar masuk digambar sebagai teks (bukan bagian dari
berkas gambar) supaya tajam di semua kerapatan layar. Jenis hurufnya **Poppins**
(SIL Open Font License, gratis) sesuai pilihan pemilik project, dimuat di
`app/_layout.tsx` lewat `@expo-google-fonts/poppins`. Yang dibundel hanya lima
ketebalan yang dipakai (400, 500, 600, 700, 800); impor dari akar paket akan
membundel seluruh 18 berkas dan menambah ~2 MB tanpa manfaat.

Pemilihan ketebalan memakai `fontFamily` (satu berkas per ketebalan), bukan
`fontWeight`: pada Android, memaksa `fontWeight` untuk font kustom membuat
sistem memilih fallback. Bila pemilik memberikan logo wordmark sebagai gambar,
cukup ganti komponen `BrandLogo`.

### Menaruh bahan referensi dan video

Bahan mentah desain ditaruh di **`design-inputs/`** pada akar repository. Folder
itu **tidak di-commit** (lihat `.gitignore`): isinya boleh berupa karya pihak
lain, berkas berukuran besar, atau rekaman yang memuat wajah orang.

| Folder | Isi |
|---|---|
| `design-inputs/logo/` | Logo resmi (master) — `.svg`, `.ai`, `.psd`, atau PNG besar |
| `design-inputs/referensi-ui/` | Mockup, sketsa, moodboard, screenshot acuan |
| `design-inputs/video/` | Video acuan (`.mp4`/`.mov`/`.webm`) |

**Batas yang tidak boleh dilanggar:** video atau gambar yang memuat wajah orang
tidak boleh ikut di-commit. Klip latar layar masuk karena itu **tidak** masuk
repository; lihat `assets/video/BACA-SAYA.md` untuk cara memasangnya di laptop
sendiri dan konsekuensinya pada mesin lain.

Mengganti aset brand:

1. Timpa `assets/branding/logo-mark.png` (mark di dalam aplikasi, latar
   transparan, rasio 1:1).
2. Timpa `icon.png`, `adaptive-icon-foreground.png`,
   `adaptive-icon-monochrome.png`, `splash-icon.png`, dan `favicon.png` bila
   ukuran berbeda diperlukan.
3. Warna brand diatur terpusat di `src/theme/tokens.ts`.

Tidak ada penyesuaian layout yang diperlukan: ukuran logo diambil dari token
`layout.logoSize`, dan gambar memakai `resizeMode: contain` sehingga proporsi asli
selalu terjaga.

## Yang belum ada (di luar lingkup FL-UI01/FL-NET01)

Google OAuth sungguhan, penyimpanan sesi/token, login pengguna produksi, CRUD
admin, API absensi, kamera IoT, pengenalan wajah, enrollment biometrik,
notifikasi, dan TLS LAN produksi. Diagnostik koneksi hanya membaca status
layanan; ia bukan lapisan API untuk fitur.

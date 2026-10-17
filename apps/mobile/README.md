# FaceLink AI — Aplikasi Mobile (Expo)

Aplikasi klien React Native + Expo untuk tiga role manusia: `admin`, `dosen`, dan
`mahasiswa`. Target utama **Android**; web dipakai hanya sebagai jalur preview.

> **Status: FL-UI01 — fondasi dan preview UI.** Belum ada integrasi backend,
> belum ada Google OAuth sungguhan, dan belum ada sesi yang disimpan. Seluruh
> angka di dashboard adalah **data contoh (DEMO)**.

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
Layar masuk  →  Role Checking  →  Pemilih role (development)  →  Beranda role
```

- **Layar masuk:** latar video atau animasi pindai, logo resmi FaceLink Ai, satu
  tombol **Continue with Google**, dan label kebijakan di dasar layar. Tidak ada
  tombol "Skip" dan tidak ada jalur masuk lain.
- **Role Checking:** animasi pemindaian dan teks "Sedang mengecek peran anda…".
  Penundaan singkat di layar ini **hanya untuk preview** dan bukan mekanisme
  autentikasi.
- **Pemilih role (development):** melihat beranda Admin, Dosen, dan Mahasiswa
  tanpa Google Sign-In.
- **Beranda role:** admin (ringkasan, kelas aktif, status perangkat, aktivitas,
  pintasan pengelolaan), dosen (kerangka), mahasiswa (kerangka baca-sendiri).

## Batas keamanan mode demo

Mode demo hanya aktif pada **development build** (`__DEV__`), lihat
`src/features/auth/demoMode.ts`.

- Tidak memanggil backend, tidak menerbitkan token, dan tidak menyimpan
  kredensial apa pun.
- Build produksi menolak tombol masuk dengan pesan "belum tersedia" dan layar
  pemilih role tidak menawarkan pilihan apa pun — **tidak ada** jalur yang
  menaikkan hak akses.
- Setiap layar yang memakai data contoh menampilkan penanda **DEMO**.
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

## Yang belum ada (di luar lingkup FL-UI01)

Google OAuth sungguhan, penyimpanan sesi/token, panggilan API backend, absensi,
kamera IoT, pengenalan wajah, enrollment biometrik, notifikasi, dan seluruh layar
CRUD admin.

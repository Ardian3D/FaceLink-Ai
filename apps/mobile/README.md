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

## Alur preview saat ini

```
Layar masuk  →  Role Checking  →  Pemilih role (development)  →  Beranda role
```

- **Layar masuk:** logo placeholder, nama brand, satu tombol
  **Continue with Google**. Tidak ada tombol "Skip" dan tidak ada jalur masuk lain.
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
└── assets/branding/         # PLACEHOLDER logo/ikon — lihat di bawah
```

## Aset brand — masih placeholder

`assets/branding/` berisi **grafik sintetis sementara** yang dibuat sendiri:
cincin pemindai geometris, bukan logo brand mana pun dan bukan foto wajah. Logo
resmi FaceLink AI belum tersedia di repository.

### Menaruh aset resmi dan bahan referensi

Bahan mentah desain ditaruh di **`design-inputs/`** pada akar repository. Folder
itu **tidak di-commit** (lihat `.gitignore`): isinya boleh berupa karya pihak
lain, berkas berukuran besar, atau rekaman yang memuat wajah orang.

| Folder | Isi |
|---|---|
| `design-inputs/logo/` | Logo resmi (master) — `.svg`, `.ai`, `.psd`, atau PNG besar |
| `design-inputs/referensi-ui/` | Mockup, sketsa, moodboard, screenshot acuan |
| `design-inputs/video/` | Video acuan (`.mp4`/`.mov`/`.webm`) |

Yang **di-commit** hanyalah aset yang benar-benar dibangun aplikasi:
`apps/mobile/assets/branding/`. Dari logo master di `design-inputs/logo/`, salinan
yang dipakai aplikasi diletakkan di sini — konversi format dan pembuatan ukuran
ikon dilakukan saat pemasangan, bukan dengan menaruh berkas master besar di
repository.

**Batas yang tidak boleh dilanggar:** video atau gambar yang memuat wajah orang
tidak boleh ikut di-commit. Bila video ingin dipakai sebagai latar di dalam
aplikasi, pilihannya adalah visual animasi yang dibuat di kode, video tanpa orang
yang dapat dikenali, atau berkas yang tetap lokal untuk demo di laptop pemilik.

Menggantinya setelah aset resmi diberikan:

1. Timpa `assets/branding/logo-mark.png` (logo di dalam aplikasi, latar
   transparan, rasio 1:1).
2. Timpa `icon.png`, `adaptive-icon-foreground.png`,
   `adaptive-icon-background.png`, `adaptive-icon-monochrome.png`,
   `splash-icon.png`, dan `favicon.png` bila ukuran berbeda diperlukan.
3. Warna brand diatur terpusat di `src/theme/tokens.ts` — nilai sekarang
   **sementara** dan tidak boleh dianggap keputusan final.

Tidak ada penyesuaian layout yang diperlukan: ukuran logo diambil dari token
`layout.logoSize`, dan gambar memakai `resizeMode: contain` sehingga proporsi asli
selalu terjaga.

## Yang belum ada (di luar lingkup FL-UI01)

Google OAuth sungguhan, penyimpanan sesi/token, panggilan API backend, absensi,
kamera IoT, pengenalan wajah, enrollment biometrik, notifikasi, dan seluruh layar
CRUD admin.

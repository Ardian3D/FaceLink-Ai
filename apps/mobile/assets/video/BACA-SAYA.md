# Klip video layar masuk — TIDAK di-commit

Folder ini memuat modul daftar klip latar layar masuk. Yang **di-commit** hanya
kode dan dokumentasinya; **berkas videonya tidak**.

## Mengapa videonya tidak boleh masuk repository

Klip yang diberikan pemilik project memuat **wajah orang yang dapat dikenali**.
Aturan yang berlaku di repository ini melarangnya:

- `AGENTS.md` §2 — tidak ada foto/identitas peserta yang dapat diidentifikasi
  yang masuk ke repository publik.
- `.gitignore` — pola `*.mp4` (dan `*.mov`, `*.avi`, `*.mkv`, …) diblokir secara
  global justru karena alasan ini.
- `design-inputs/BACA-SAYA.md` — bahan mentah desain tidak pernah di-commit; yang
  masuk repository hanya hasil desainnya.

Karena itu klip hidup **hanya di mesin yang memilikinya**, sebagai bahan demo.
Folder ini (`apps/mobile/assets/video/`) memakai aturan yang sama: berkas
`*.mp4` di sini diabaikan Git.

## Berkas yang ada di sini

| Berkas | Di-commit | Isi |
|---|---|---|
| `BACA-SAYA.md` | ya | dokumen ini |
| `video-clips.d.ts` | ya | tipe modul `../../assets/video/video-clips` |
| `video-clips.none.js` | ya | daftar kosong — dipakai bila klip tidak ada |
| `video-clips.local.js` | ya | daftar klip; hanya dipakai bila klipnya ada |
| `video1.mp4`, `video2.mp4` | **tidak** | klip latar sebenarnya (lokal) |

`metro.config.js` memilih `video-clips.local.js` atau `video-clips.none.js`
**saat bundel dibuat**, berdasarkan ada/tidaknya kedua berkas klip. Jadi:

- **Klon baru / CI / laptop lain** — tidak ada klip → daftar kosong → layar
  masuk memakai latar animasi yang digambar di kode (`ScanBackdrop`). Bundel
  tetap berhasil; tidak ada berkas yang hilang.
- **Laptop pemilik project** — klip ada → kedua klip diputar bergantian **tanpa
  jeda dan tanpa layar hitam**: masing-masing klip menempati lapisan video
  sendiri yang sudah termuat sejak awal, lalu keduanya saling menyeberang
  (crossfade 450 ms) sebelum klip lama habis. Rinciannya di komentar
  `src/components/VideoBackground.tsx`.

## Memasang / melepas klip di laptop sendiri

```bash
cd apps/mobile
npm run video:local     # salin klip dari design-inputs/video/ ke folder ini
npm run video:off       # hapus salinan lokal, kembali ke latar animasi kode
```

Setelah salah satu perintah di atas, **mulai ulang** `npm start` — Metro
menyimpan hasil resolusi modul di cache, jadi perubahan tidak terlihat sebelum
dev server dijalankan ulang.

Sumber salinannya adalah `design-inputs/video/` di akar repository (folder yang
juga tidak di-commit). Bila ingin memakai klip lain, letakkan berkas bernama
`video1.mp4` dan `video2.mp4` langsung di folder ini.

## Pemulihan FL-UI03 pada HP fisik (24 Oktober 2026)

Sesudah pengecualian fallback R4, kedua MP4 **disalin kembali**, bukan dipindahkan,
dari `%TEMP%\FaceLink-R4-video-preserved-1792813932286` ke folder ini. Backup
tersebut harus dipertahankan sampai pemilik memberi izin menghapusnya.

| Berkas | Ukuran byte | SHA-256 salinan aktif dan preserved |
|---|---:|---|
| `video1.mp4` | 2080600 | `6bb00d5bcb3c9df068cd16da32b53a0069f0784ab8906acc7e9ecfe85ccd6ef3` |
| `video2.mp4` | 1819238 | `12e169fbf412640a205e79defbf9e82101c8b45bfabd58cbffdcef871cb7dac4` |

Pemulihan tidak memerlukan perubahan komponen atau APK: resolver existing
memilih `video-clips.local.js` ketika kedua file tersedia. Metro direstart dengan
cache bersih pada localhost 8082; bundle dibangun sebelum reproduksi di HP.
Backend tetap HTTPS loopback 8080 dan kedua port memakai ADB reverse.
Ini prosedur setup uji, bukan bukti bahwa pemanasan Metro memperbaiki suatu bug.

Bukti pada TECNO LJ7/Android 16/API 36: kedua file H.264/yuv420p, 720×1280,
24 fps, durasi delapan detik lolos decode penuh dengan ffmpeg; endpoint asset
Metro melayani byte-range HTTP 206 `video/mp4`. Diagnostik sementara mengamati
`onFirstFrameRender` kedua lapisan dan enam pergantian lapisan dengan
`siap=true`, `gagal=false`, tanpa timeout media/fatal error pada reproduksi.
Diagnostik itu sudah dihapus; source kembali ke versi semula. Frame wajah tidak
ditampilkan atau disimpan sebagai bukti pengujian.

Cold start setelah penghapusan diagnostik kembali menampilkan kontrol login:
FaceLink aktif, Google disabled. FaceLink membuka form email; tombol kirim kode
masih disabled dan tidak ada OTP diminta. Pemeriksaan konten dilakukan setelah
React Native siap; keluaran `am start` saja belum membuktikan UI sudah termuat.
Diagnostik aplikasi juga menunjukkan healthz ok/readyz ready melalui HTTPS.
Tidak mengulang login Dosen/Mahasiswa atau merekam animasi splash secara terpisah.

**Batas RCA:** fallback setelah R4 aktif karena kedua file sengaja dipindahkan.
Blank awal sebelum pemindahan pernah disertai timeout media, tetapi tidak
terulang pada reproduksi dengan asset kembali dan Metro hangat. Belum ada bukti
bahwa codec, renderer, atau transport tertentu adalah penyebab pasti blank
intermiten tersebut; jangan menambahkan patch spekulatif atau menyatakan akar
masalahnya telah diperbaiki.

## Yang belum diputuskan pemilik project

Agar video dapat ikut ter-commit dan tampil di mesin mana pun, dibutuhkan salah
satu dari ini — semuanya menunggu keputusan pemilik:

1. **Klip tanpa orang yang dapat dikenali** (mis. animasi/grafis FaceLink saja)
   yang boleh di-commit, lalu aturan `*.mp4` dibuka khusus untuk folder ini.
2. **Latar animasi kode saja** — `ScanBackdrop` yang sekarang dipakai sebagai
   cadangan dijadikan tampilan tetap, dan berkas video tidak dipakai sama sekali.
3. **Klip tetap lokal** seperti sekarang: tampil di laptop demo, sementara mesin
   lain memakai latar animasi kode.

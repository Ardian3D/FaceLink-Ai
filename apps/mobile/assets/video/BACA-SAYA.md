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
- **Laptop pemilik project** — klip ada → kedua klip diputar bergantian tanpa
  jeda sebagai latar layar masuk.

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

## Yang belum diputuskan pemilik project

Agar video dapat ikut ter-commit dan tampil di mesin mana pun, dibutuhkan salah
satu dari ini — semuanya menunggu keputusan pemilik:

1. **Klip tanpa orang yang dapat dikenali** (mis. animasi/grafis FaceLink saja)
   yang boleh di-commit, lalu aturan `*.mp4` dibuka khusus untuk folder ini.
2. **Latar animasi kode saja** — `ScanBackdrop` yang sekarang dipakai sebagai
   cadangan dijadikan tampilan tetap, dan berkas video tidak dipakai sama sekali.
3. **Klip tetap lokal** seperti sekarang: tampil di laptop demo, sementara mesin
   lain memakai latar animasi kode.

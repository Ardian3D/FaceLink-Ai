/**
 * Daftar klip latar video — varian "dengan klip".
 *
 * Berkas ini hanya dipakai Metro bila **kedua** berkas klip ada di
 * `assets/video/` pada mesin yang sedang membangun. Berkas klip tidak ada di
 * repository (`*.mp4` di-`gitignore`) karena memuat wajah orang yang dapat
 * dikenali; lihat `BACA-SAYA.md` di folder ini.
 *
 * Untuk memasangnya di laptop sendiri: `npm run video:local`, lalu mulai ulang
 * dev server. `npm run video:off` mengembalikannya ke latar animasi kode.
 */
export const localVideoClips = [
  require("./video1.mp4"),
  require("./video2.mp4"),
];

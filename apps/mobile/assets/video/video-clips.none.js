/**
 * Daftar klip latar video — varian "tanpa klip".
 *
 * Berkas ini yang dipakai Metro pada mesin yang tidak memiliki berkas video di
 * `assets/video/`: CI, klon baru, dan laptop lain. Isinya sengaja kosong
 * sehingga repo dapat dibundel tanpa berkas video apa pun. Daftar kosong bukan
 * keadaan rusak — `VideoBackground` akan memakai latar animasi yang digambar di
 * kode.
 *
 * Pasangannya adalah `video-clips.local.js` yang memuat `require("./video1.mp4")`.
 * Pemilihan di antara keduanya dilakukan `metro.config.js`. Jangan mengimpor
 * berkas ini dari kode aplikasi; impor selalu `../../assets/video/video-clips`.
 */
export const localVideoClips = [];

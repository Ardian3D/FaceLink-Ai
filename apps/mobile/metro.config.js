// Konfigurasi Metro untuk aplikasi mobile FaceLink AI.
//
// Satu-satunya alasan berkas ini ada: **klip latar layar masuk tidak boleh
// di-commit** (memuat wajah orang yang dapat dikenali — lihat
// `assets/video/BACA-SAYA.md` dan `docs/SECURITY_PRIVACY.md`). Karena itu
// permintaan modul `assets/video/video-clips` diarahkan ke salah satu dari dua
// berkas yang keduanya ada di repository:
//
//   video-clips.local.js  → memuat `require("./video1.mp4")` dan seterusnya
//   video-clips.none.js   → daftar kosong; aplikasi memakai latar animasi kode
//
// Pilihannya ditentukan saat bundel dibuat: klip dipakai hanya bila berkasnya
// benar-benar ada di `assets/video/`. Akibatnya repo ini tetap dapat dibangun
// di mesin mana pun (CI, klon baru, laptop lain) tanpa berkas video, dan demo
// di laptop pemilik tetap memakai video aslinya.
//
// Menyalakan/mematikan klip: `npm run video:local` / `npm run video:off`.
// Setelah mengubahnya, mulai ulang dev server supaya resolusi modul dihitung
// ulang (Metro menyimpan hasil resolusi di cache).
const path = require("node:path");
const fs = require("node:fs");
const { getDefaultConfig } = require("expo/metro-config");

const config = getDefaultConfig(__dirname);

/** Nama folder aset video di dalam paket mobile ini. */
const FOLDER_KLIP = path.join(__dirname, "assets", "video");

/**
 * Nama berkas klip yang harus ada agar video dipakai. Berkas ini sengaja
 * di-`gitignore` (`*.mp4`); ketiadaannya adalah keadaan normal, bukan galat.
 */
const BERKAS_KLIP = ["video1.mp4", "video2.mp4"].map((nama) =>
  path.join(FOLDER_KLIP, nama)
);

/** Modul virtual yang dicari aplikasi: `assets/video/video-clips`. */
const MODUL_KLIP = "video-clips";

/** Resolver bawaan Expo (bila ada) tetap dipakai untuk modul lain. */
const resolverBawaan = config.resolver.resolveRequest;

config.resolver.resolveRequest = (context, moduleName, platform) => {
  if (moduleName === MODUL_KLIP || moduleName.endsWith(`/${MODUL_KLIP}`)) {
    const lengkap = BERKAS_KLIP.every((berkas) => fs.existsSync(berkas));
    const berkas = path.join(FOLDER_KLIP, lengkap ? "video-clips.local.js" : "video-clips.none.js");
    return context.resolveRequest(context, berkas, platform);
  }

  return (resolverBawaan ?? context.resolveRequest)(context, moduleName, platform);
};

module.exports = config;

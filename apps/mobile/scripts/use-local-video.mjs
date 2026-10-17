#!/usr/bin/env node
/**
 * Memasang atau melepas klip latar layar masuk di mesin lokal.
 *
 *   node scripts/use-local-video.mjs        # salin klip dari design-inputs/video/
 *   node scripts/use-local-video.mjs --off  # hapus salinan lokal
 *
 * Klip **tidak** di-commit: berkasnya memuat wajah orang yang dapat dikenali
 * (lihat assets/video/BACA-SAYA.md). Skrip ini hanya memindahkan salinan ke
 * folder yang dibaca `metro.config.js` saat bundel dibuat.
 */
import { copyFile, mkdir, rm, stat } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const paketMobile = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const akarRepo = path.resolve(paketMobile, "..", "..");

const SUMBER = path.join(akarRepo, "design-inputs", "video");
const TUJUAN = path.join(paketMobile, "assets", "video");
const NAMA_KLIP = ["video1.mp4", "video2.mp4"];

const matikan = process.argv.includes("--off");

/** Ukuran berkas dalam MiB, untuk laporan singkat. */
async function ukuranMiB(berkas) {
  const info = await stat(berkas);
  return (info.size / (1024 * 1024)).toFixed(1);
}

async function hapus() {
  let dihapus = 0;
  for (const nama of NAMA_KLIP) {
    const berkas = path.join(TUJUAN, nama);
    try {
      await rm(berkas);
      dihapus += 1;
      console.log(`dihapus  : ${path.relative(akarRepo, berkas)}`);
    } catch (galat) {
      if (galat.code !== "ENOENT") throw galat;
    }
  }
  console.log(
    dihapus === 0
      ? "\nTidak ada klip lokal untuk dihapus. Tidak ada perubahan."
      : `\n${dihapus} klip lokal dihapus. Mulai ulang dev server; layar masuk kembali memakai latar animasi kode.`
  );
}

async function pasang() {
  await mkdir(TUJUAN, { recursive: true });

  const hilang = [];
  for (const nama of NAMA_KLIP) {
    try {
      await stat(path.join(SUMBER, nama));
    } catch {
      hilang.push(nama);
    }
  }
  if (hilang.length > 0) {
    console.error(
      `Berkas berikut tidak ada di ${path.relative(akarRepo, SUMBER)}: ${hilang.join(", ")}.\n` +
        "Letakkan klip bernama video1.mp4 dan video2.mp4 di folder itu, lalu jalankan lagi."
    );
    process.exitCode = 1;
    return;
  }

  for (const nama of NAMA_KLIP) {
    const asal = path.join(SUMBER, nama);
    const berkas = path.join(TUJUAN, nama);
    await copyFile(asal, berkas);
    console.log(`disalin  : ${path.relative(akarRepo, berkas)} (${await ukuranMiB(berkas)} MiB)`);
  }

  console.log(
    "\nKlip terpasang untuk laptop ini saja (tidak akan ikut ter-commit).\n" +
      "Mulai ulang dev server (`npm start`) supaya Metro memakai klipnya;\n" +
      "hapus lagi dengan `npm run video:off`."
  );
}

await (matikan ? hapus() : pasang());

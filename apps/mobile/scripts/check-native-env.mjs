#!/usr/bin/env node
/**
 * Pemeriksa prasyarat build Android FaceLink AI (Windows).
 *
 * Menjalankan pemeriksaan **nyata** terhadap mesin ini dan mencetak satu baris
 * per prasyarat dengan status PASSED / FAILED / BLOCKED. Skrip ini tidak
 * memasang apa pun: ia hanya membaca keadaan, sehingga aman dijalankan berulang
 * dan tidak pernah mengubah konfigurasi keamanan.
 *
 * Status:
 *   PASSED  — terpasang dan versinya memenuhi syarat yang tertulis di
 *             `android/` hasil `expo prebuild` (lihat docs/ANDROID_SETUP.md).
 *   FAILED  — terpasang tetapi tidak memenuhi syarat, atau konfigurasi salah.
 *   BLOCKED — belum ada, sehingga langkah berikutnya tidak dapat dijalankan.
 *
 * Pemakaian: node scripts/check-native-env.mjs
 */
import { spawnSync } from "node:child_process";
import { existsSync, readdirSync, readFileSync, statfsSync } from "node:fs";
import path from "node:path";

const akar = path.resolve(import.meta.dirname, "..");

/** Syarat minimum, diambil dari berkas hasil `expo prebuild`. */
const SYARAT = {
  jdk: 17,
  compileSdk: 37,
  buildTools: "37.0.0",
  minSdk: 24,
  targetSdk: 36,
  // Gradle 9.x menuntut JVM 17+; AGP 9.2 menuntut build-tools 36.0.0 minimum.
  buildToolsMinimum: "36.0.0",
  cmake: "3.22.1",
  freeDiskGb: 15,
};

/**
 * Menjalankan perintah dan mengembalikan keluaran, atau null bila gagal.
 *
 * Perintah diterima sebagai satu string dan dijalankan lewat shell karena
 * `java` dan `adb` berada di PATH sebagai `.exe` yang tidak selalu dapat
 * ditemukan tanpa shell di Windows. Argumen yang digabung hanya berasal dari
 * daftar tetap di berkas ini (tidak ada masukan pengguna).
 */
function jalankan(perintah) {
  // `java -version` menulis ke stderr, sedangkan `adb version` ke stdout.
  // Keduanya digabung supaya versi terbaca apa pun kanal yang dipakai.
  const hasil = spawnSync(perintah, {
    encoding: "utf8",
    stdio: ["ignore", "pipe", "pipe"],
    timeout: 20_000,
    shell: true,
  });
  const gabungan = `${hasil.stdout ?? ""}${hasil.stderr ?? ""}`.trim();
  // Perintah yang tidak ada memberi kode 1 dengan pesan shell; keduanya harus
  // dibedakan dari perintah yang berhasil tetapi keluar bukan-nol.
  if (hasil.error || hasil.status !== 0) return { ok: false, teks: null };
  return { ok: true, teks: gabungan || null };
}

const hasil = [];
function catat(nama, status, detail) {
  hasil.push({ nama, status, detail });
}

/**
 * Membaca satu kunci dari `apps/mobile/.env` tanpa pustaka tambahan.
 *
 * Expo memuat `.env` otomatis saat bundling, sehingga nilai client ID lazim
 * berada di berkas itu dan **tidak** ada di environment shell. Tanpa pembacaan
 * ini pemeriksa akan melaporkan BLOCKED padahal konfigurasi sudah benar.
 */
function dariEnvFile(kunci) {
  const berkas = path.join(akar, ".env");
  if (!existsSync(berkas)) return "";
  const baris = readFileSync(berkas, "utf8").split(/\r?\n/);
  for (const b of baris) {
    const t = b.trim();
    if (!t || t.startsWith("#")) continue;
    const pisah = t.indexOf("=");
    if (pisah < 0) continue;
    if (t.slice(0, pisah).trim() !== kunci) continue;
    return t.slice(pisah + 1).trim().replace(/^["']|["']$/g, "");
  }
  return "";
}

/** Versi mayor dari string seperti "17.0.9" atau `java version "17.0.9"`. */
function versiMayor(teks) {
  const cocok = /(\d+)(?:\.(\d+))?/.exec(teks ?? "");
  if (!cocok) return null;
  const major = Number(cocok[1]);
  // Java 1.8 dilaporkan sebagai "1.8".
  return major === 1 ? Number(cocok[2] ?? 0) : major;
}

function bandingkanVersi(a, b) {
  const pa = a.split(".").map(Number);
  const pb = b.split(".").map(Number);
  for (let i = 0; i < Math.max(pa.length, pb.length); i += 1) {
    const d = (pa[i] ?? 0) - (pb[i] ?? 0);
    if (d !== 0) return d;
  }
  return 0;
}

// ---------------------------------------------------------------- JDK
const hasilJava = jalankan("java -version");
const keluaranJava = hasilJava.ok ? hasilJava.teks : null;
const majorJava = versiMayor(keluaranJava?.replace(/^.*version\s+/, "") ?? "");
if (!hasilJava.ok || keluaranJava === null) {
  catat("JDK", "BLOCKED", `java tidak ada pada PATH (perlu JDK ${SYARAT.jdk})`);
} else if (majorJava === null) {
  catat("JDK", "FAILED", `versi tidak terbaca: ${keluaranJava.split("\n")[0]}`);
} else if (majorJava < SYARAT.jdk) {
  catat("JDK", "FAILED", `Java ${majorJava} terpasang; Gradle 9 memerlukan ${SYARAT.jdk}+`);
} else {
  catat("JDK", "PASSED", `Java ${majorJava}`);
}

const javaHome = process.env.JAVA_HOME;
if (!javaHome) {
  catat("JAVA_HOME", "BLOCKED", "belum disetel; Gradle memakainya untuk menemukan JDK");
} else if (!existsSync(path.join(javaHome, "bin"))) {
  catat("JAVA_HOME", "FAILED", `menunjuk folder tanpa bin/: ${javaHome}`);
} else {
  catat("JAVA_HOME", "PASSED", javaHome);
}

// ------------------------------------------------------------ Android SDK
const sdk = process.env.ANDROID_HOME || process.env.ANDROID_SDK_ROOT || "";
if (!sdk) {
  catat("ANDROID_HOME", "BLOCKED", "belum disetel; expo run:android mencarinya");
} else if (!existsSync(sdk)) {
  catat("ANDROID_HOME", "FAILED", `folder tidak ada: ${sdk}`);
} else {
  catat("ANDROID_HOME", "PASSED", sdk);
}

if (sdk && existsSync(sdk)) {
  // SDK Manager menamai folder platform dengan revisi minor (`android-37.0`),
  // sedangkan `compileSdk` di Gradle hanya memakai nomor API. Keduanya sah.
  const platformDir = path.join(sdk, "platforms");
  const platformCocok = existsSync(platformDir)
    ? readdirSync(platformDir).filter((n) => n === `android-${SYARAT.compileSdk}` || n.startsWith(`android-${SYARAT.compileSdk}.`))
    : [];
  catat(
    `platform android-${SYARAT.compileSdk}`,
    platformCocok.length > 0 ? "PASSED" : "BLOCKED",
    platformCocok.length > 0
      ? path.join(platformDir, platformCocok[0])
      : `belum dipasang (compileSdk hasil prebuild = ${SYARAT.compileSdk})`
  );

  const tools = path.join(sdk, "build-tools", SYARAT.buildTools);
  catat(
    `build-tools ${SYARAT.buildTools}`,
    existsSync(tools) ? "PASSED" : "BLOCKED",
    existsSync(tools) ? tools : `belum dipasang (versi ini dipakai hasil prebuild)`
  );

  // Versi CMake ditentukan AGP, bukan berkas project, sehingga yang diperiksa
  // adalah "ada versi >= 3.13" alih-alih satu nomor tertentu.
  const cmakeDir = path.join(sdk, "cmake");
  const cmakeTerpasang = existsSync(cmakeDir)
    ? readdirSync(cmakeDir).filter((v) => bandingkanVersi(v, "3.13.0") >= 0)
    : [];
  catat(
    "cmake",
    cmakeTerpasang.length > 0 ? "PASSED" : "BLOCKED",
    cmakeTerpasang.length > 0
      ? `terpasang: ${cmakeTerpasang.join(", ")}`
      : "belum dipasang — Nitro Modules mengompilasi C++ melalui CMake (perlu >= 3.13)"
  );

  const ndkDir = path.join(sdk, "ndk");
  const ndk = existsSync(ndkDir) ? readdirSync(ndkDir) : [];
  const ndkWajib = "27.1.12297006";
  catat(
    `NDK ${ndkWajib}`,
    ndk.includes(ndkWajib) ? "PASSED" : "BLOCKED",
    ndk.includes(ndkWajib)
      ? path.join(ndkDir, ndkWajib)
      : ndk.length > 0
        ? `versi wajib belum ada; terpasang: ${ndk.join(", ")}`
        : "belum ada (rootProject.ext.ndkVersion = 27.1.12297006)"
  );

  const adbPath = path.join(sdk, "platform-tools", "adb.exe");
  const hasilAdb = jalankan(`"${adbPath}" version`);
  catat(
    "adb",
    hasilAdb.ok ? "PASSED" : "BLOCKED",
    hasilAdb.ok ? (hasilAdb.teks ?? "ada").split("\n")[0] : "platform-tools belum terpasang"
  );
  if (hasilAdb.ok) {
    const perangkat = jalankan(`"${adbPath}" devices`);
    const siap = (perangkat.teks ?? "").split(/\r?\n/).filter((l) => /\tdevice$/.test(l));
    catat(
      "perangkat/emulator terhubung",
      siap.length > 0 ? "PASSED" : "BLOCKED",
      siap.length > 0 ? `${siap.length} perangkat siap` : "tidak ada perangkat berstatus device"
    );
  }

  // Emulator sengaja TIDAK diwajibkan: target pengujian adalah HP fisik.
  const hasilEmulator = jalankan(`"${path.join(sdk, "emulator", "emulator.exe")}" -list-avds`);
  const avds = (hasilEmulator.teks ?? "").split(/\r?\n/).filter(Boolean);
  catat(
    "AVD emulator (opsional)",
    "INFO",
    hasilEmulator.ok
      ? avds.length > 0
        ? `tersedia: ${avds.join(", ")}`
        : "tidak ada AVD — boleh diabaikan bila memakai HP fisik"
      : "emulator tidak dipasang — boleh diabaikan, target uji adalah HP fisik"
  );
} else {
  for (const nama of [
    `platform android-${SYARAT.compileSdk}`,
    `build-tools ${SYARAT.buildTools}`,
    "cmake",
    "NDK 27.1.12297006",
    "adb",
    "perangkat/emulator terhubung",
    "AVD emulator (opsional)",
  ]) {
    catat(
      nama,
      nama === "AVD emulator (opsional)" ? "INFO" : "BLOCKED",
      nama === "AVD emulator (opsional)" ? "emulator tidak diperlukan untuk HP fisik" : "menunggu ANDROID_HOME yang sah"
    );
  }
}

// ------------------------------------------------------------ CMake + ninja
// Ninja bawaan CMake 3.22.1 (versi 1.10) gagal membuat folder objek ketika
// jalur build melewati 260 karakter — keadaan yang lazim terjadi pada proyek
// React Native di `node_modules`. CMake 3.31.x membawa ninja 1.12 yang
// menanganinya. Karena itu `android/local.properties` perlu `cmake.dir`.
if (sdk && existsSync(sdk)) {
  const lokal = path.join(akar, "android", "local.properties");
  const isiLokal = existsSync(lokal) ? readFileSync(lokal, "utf8") : "";
  const cocok = /^cmake\.dir=(.+)$/m.exec(isiLokal);
  const cmakeDir =
    cocok?.[1]?.trim() ||
    (existsSync(path.join(sdk, "cmake", "3.31.6"))
      ? path.join(sdk, "cmake", "3.31.6")
      : path.join(sdk, "cmake", "3.22.1"));
  const ninjaPath = path.join(cmakeDir, "bin", "ninja.exe");
  const hasilNinja = existsSync(ninjaPath) ? jalankan(`"${ninjaPath}" --version`) : { ok: false };
  const versiNinja = hasilNinja.ok ? (hasilNinja.teks ?? "").trim() : "";
  // Objek ninja berada di <akar>/node_modules/<paket>/... ; dengan prefiks
  // .cxx/CMakeFiles/<target>.dir/ jalur ini mudah melewati 260 karakter.
  const akarPanjang = path.join(akar, "node_modules").length > 55;
  const cukup = bandingkanVersi(versiNinja || "0", "1.11.0") >= 0;
  catat(
    "ninja (jalur panjang)",
    !hasilNinja.ok ? "BLOCKED" : akarPanjang && !cukup ? "FAILED" : "PASSED",
    !hasilNinja.ok
      ? "ninja belum ada; pasang CMake lewat sdkmanager"
      : akarPanjang && !cukup
        ? `ninja ${versiNinja} dipakai; jalur build > 260 karakter. Setel cmake.dir ke CMake 3.31.6 lewat android/local.properties`
        : `ninja ${versiNinja} dari ${cmakeDir}`
  );
}

// ------------------------------------------------------- konfigurasi OAuth
const webClientId =
  (process.env.EXPO_PUBLIC_FACELINK_GOOGLE_WEB_CLIENT_ID ?? "").trim() ||
  dariEnvFile("EXPO_PUBLIC_FACELINK_GOOGLE_WEB_CLIENT_ID");
if (!webClientId) {
  catat(
    "EXPO_PUBLIC_FACELINK_GOOGLE_WEB_CLIENT_ID",
    "BLOCKED",
    "belum disetel di environment maupun .env — tanpa ini tombol masuk menolak berjalan"
  );
} else if (!/^[0-9A-Za-z_-]+\.apps\.googleusercontent\.com$/.test(webClientId)) {
  catat(
    "EXPO_PUBLIC_FACELINK_GOOGLE_WEB_CLIENT_ID",
    "FAILED",
    "bukan client ID bertipe Web (harus berakhiran .apps.googleusercontent.com)"
  );
} else {
  catat(
    "EXPO_PUBLIC_FACELINK_GOOGLE_WEB_CLIENT_ID",
    "PASSED",
    process.env.EXPO_PUBLIC_FACELINK_GOOGLE_WEB_CLIENT_ID
      ? "bentuk nilai sah (environment)"
      : "bentuk nilai sah (dari .env)"
  );
}

// ------------------------------------------------------------- berkas build
catat(
  "folder android/",
  existsSync(path.join(akar, "android")) ? "PASSED" : "BLOCKED",
  existsSync(path.join(akar, "android"))
    ? "hasil expo prebuild (diabaikan Git)"
    : "jalankan: npx expo prebuild --platform android"
);
catat(
  "debug keystore",
  existsSync(path.join(akar, "android", "app", "debug.keystore")) ? "PASSED" : "BLOCKED",
  "SHA-1 dari keystore ini wajib didaftarkan pada OAuth client Android"
);

// -------------------------------------------------------------------- disk
try {
  const fs = statfsSync(akar);
  const gb = (fs.bavail * fs.bsize) / 1024 ** 3;
  catat(
    "ruang disk",
    gb >= SYARAT.freeDiskGb ? "PASSED" : "FAILED",
    `${gb.toFixed(0)} GB bebas (perlu >= ${SYARAT.freeDiskGb} GB untuk SDK + build)`
  );
} catch {
  catat("ruang disk", "BLOCKED", "tidak dapat dibaca");
}

// ------------------------------------------------------------------ cetak
const lebar = Math.max(...hasil.map((h) => h.nama.length));
console.log("Pemeriksaan prasyarat build Android FaceLink AI\n");
for (const h of hasil) {
  console.log(`${h.status.padEnd(8)} ${h.nama.padEnd(lebar)}  ${h.detail}`);
}
const jumlah = (s) => hasil.filter((h) => h.status === s).length;
console.log(
  `\nRingkas: ${jumlah("PASSED")} PASSED, ${jumlah("FAILED")} FAILED, ${jumlah("BLOCKED")} BLOCKED`
);
// Kode keluar 1 hanya bila ada yang FAILED (salah konfigurasi). BLOCKED berarti
// prasyarat belum dipasang, bukan kegagalan perangkat lunak project.
process.exitCode = jumlah("FAILED") > 0 ? 1 : 0;

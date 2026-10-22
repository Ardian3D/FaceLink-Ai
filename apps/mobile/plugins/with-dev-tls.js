/**
 * Config plugin: memercayai CA development HANYA pada build debug.
 *
 * Latar belakang (docs/SECURITY_PRIVACY.md §6, dokumen D-035): backend MVP
 * hanya mendengarkan loopback. Emulator menjangkaunya lewat alias host
 * `10.0.2.2`, jadi token tidak pernah melintasi jaringan fisik. Agar lalu
 * lintas itu tetap terenkripsi, backend menyajikan HTTPS dengan sertifikat
 * yang diterbitkan CA development lokal.
 *
 * Yang dilakukan plugin ini:
 *  1. Menyalin `backend/dev/certs/dev-ca.pem` ke `res/raw/dev_ca.pem`
 *     pada **source set debug** saja.
 *  2. Menulis `res/xml/network_security_config.xml` pada source set debug,
 *     yang mempercayai sistem + CA dev tersebut, dan **hanya** mengizinkan
 *     cleartext untuk Metro (`10.0.2.2`, `localhost`, `127.0.0.1`).
 *  3. Menambahkan `android:networkSecurityConfig` pada manifest **debug**.
 *
 * Build release tidak tersentuh: tidak ada CA tambahan, tidak ada pengecualian
 * cleartext, dan tidak ada atribut manifest tambahan. Karena itu
 * `InsecureSkipVerify` atau penghapusan validasi sertifikat tidak diperlukan
 * di mana pun.
 *
 * Bila berkas CA belum ada (mis. clone baru sebelum menjalankan
 * `scripts/gen-dev-tls.sh`), plugin tidak melakukan apa pun dan mencetak
 * peringatan: build tetap berhasil, hanya HTTPS dev yang tidak tepercaya.
 */
const fs = require("node:fs");
const path = require("node:path");
const { withDangerousMod } = require("expo/config-plugins");

const CA_SOURCE = path.join("backend", "dev", "certs", "dev-ca.pem");

const NETWORK_SECURITY_CONFIG = `<?xml version="1.0" encoding="utf-8"?>
<!-- Dihasilkan oleh plugins/with-dev-tls.js — HANYA untuk build debug. -->
<network-security-config>
    <!-- Bawaan: hanya CA sistem dan CA dev yang dibundel; cleartext ditolak. -->
    <base-config cleartextTrafficPermitted="false">
        <trust-anchors>
            <certificates src="system" />
            <certificates src="@raw/dev_ca" />
        </trust-anchors>
    </base-config>
    <!-- Metro menyajikan bundel JS lewat HTTP di loopback/alias emulator. -->
    <domain-config cleartextTrafficPermitted="true">
        <domain includeSubdomains="false">10.0.2.2</domain>
        <domain includeSubdomains="false">localhost</domain>
        <domain includeSubdomains="false">127.0.0.1</domain>
    </domain-config>
</network-security-config>
`;

const DEBUG_MANIFEST = `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    xmlns:tools="http://schemas.android.com/tools">

    <uses-permission android:name="android.permission.SYSTEM_ALERT_WINDOW"/>

    <!-- Hanya pada build debug: percayai CA development dan izinkan cleartext
         terbatas untuk Metro. Build release memakai kebijakan bawaan Android. -->
    <application
        android:usesCleartextTraffic="true"
        android:networkSecurityConfig="@xml/network_security_config"
        tools:targetApi="28"
        tools:ignore="GoogleAppIndexingWarning"
        tools:replace="android:usesCleartextTraffic" />
</manifest>
`;

module.exports = function withDevTLS(config) {
  return withDangerousMod(config, [
    "android",
    async (cfg) => {
      const projectRoot = cfg.modRequest.projectRoot;
      const androidRoot = path.join(projectRoot, "android");
      const debugRoot = path.join(androidRoot, "app", "src", "debug");

      if (!fs.existsSync(androidRoot)) {
        // `npx expo prebuild` belum dijalankan; tidak ada yang bisa disunting.
        return cfg;
      }

      const caPath = path.join(projectRoot, "..", "..", CA_SOURCE);
      if (!fs.existsSync(caPath)) {
        console.warn(
          `[with-dev-tls] CA dev tidak ditemukan di ${caPath}. ` +
            "Jalankan scripts/gen-dev-tls.sh lalu prebuild ulang bila membutuhkan HTTPS lokal."
        );
        return cfg;
      }

      const rawDir = path.join(debugRoot, "res", "raw");
      const xmlDir = path.join(debugRoot, "res", "xml");
      fs.mkdirSync(rawDir, { recursive: true });
      fs.mkdirSync(xmlDir, { recursive: true });

      fs.copyFileSync(caPath, path.join(rawDir, "dev_ca.pem"));
      fs.writeFileSync(
        path.join(xmlDir, "network_security_config.xml"),
        NETWORK_SECURITY_CONFIG
      );
      fs.writeFileSync(path.join(debugRoot, "AndroidManifest.xml"), DEBUG_MANIFEST);

      console.log("[with-dev-tls] CA dev dipasang untuk build debug saja.");
      return cfg;
    },
  ]);
};

#!/usr/bin/env node
/**
 * Mengundang akun baru melalui API resmi FaceLink AI.
 *
 * Alur: minta OTP untuk Admin lama -> verifikasi OTP (input tersembunyi) ->
 * POST /api/v1/users dengan Bearer token -> logout. Tidak ada akses SQL,
 * tidak ada bootstrap ulang, dan OTP/token tidak pernah dicetak.
 *
 * Jalankan backend HTTPS lebih dahulu, lalu:
 *   node scripts/invite-admin.mjs --admin-email "existing-admin@example.test" --yes
 *
 * Gunakan --dry-run untuk melihat rencana tanpa mengirim OTP.
 */
import fs from "node:fs";
import https from "node:https";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const args = process.argv.slice(2);

function arg(name, fallback) {
  const i = args.indexOf(name);
  return i >= 0 && i + 1 < args.length ? args[i + 1] : fallback;
}
const has = (name) => args.includes(name);

if (has("--help")) {
  console.log(`Penggunaan:
  node scripts/invite-admin.mjs --admin-email <email-admin-lama> [opsi]

Opsi:
  --email <alamat>       Akun baru (default: admin@facelink.online)
  --name <nama>          Nama tampilan (default: Administrator)
  --role <peran>         admin|dosen|mahasiswa (default: admin)
  --api <url>            Default: https://127.0.0.1:8080
  --ca <path>            Default: backend/dev/certs/dev-ca.pem
  --dry-run              Tampilkan rencana, tanpa menghubungi server
  --yes                  Konfirmasi pembuatan akun

OTP diminta secara tersembunyi. Jangan menjalankan dengan input yang dialihkan.`);
  process.exit(0);
}

const api = new URL(arg("--api", "https://127.0.0.1:8080"));
if (api.protocol !== "https:") throw new Error("--api wajib https.");
const caPath = path.resolve(root, arg("--ca", "backend/dev/certs/dev-ca.pem"));
const adminEmail = (arg("--admin-email", "") || "").trim().toLowerCase();
const email = (arg("--email", "admin@facelink.online") || "").trim().toLowerCase();
const name = (arg("--name", "Administrator") || "").trim();
const role = (arg("--role", "admin") || "").trim().toLowerCase();
const dryRun = has("--dry-run");
const yes = has("--yes");

if (!["admin", "dosen", "mahasiswa"].includes(role)) throw new Error("Peran tidak sah.");
for (const [label, value] of [["--admin-email", adminEmail], ["--email", email]]) {
  if (!/^[^\s@<>]+@[^\s@<>]+\.[^\s@<>]+$/.test(value)) throw new Error(`${label} tidak sah.`);
}
if (!name) throw new Error("Nama tampilan wajib diisi.");
if (!fs.existsSync(caPath)) throw new Error(`CA tidak ditemukan: ${caPath}`);

console.log(`Rencana: undang ${email} sebagai ${role} melalui sesi ${adminEmail}.`);
if (dryRun) {
  console.log("DRY RUN: tidak ada OTP yang diminta dan tidak ada akun yang dibuat.");
  process.exit(0);
}
if (!yes) {
  console.log("Tidak ada perubahan. Tambahkan --yes setelah memastikan alamat dan peran benar.");
  process.exit(2);
}

const ca = fs.readFileSync(caPath);
function request(method, pathname, body, token) {
  return new Promise((resolve, reject) => {
    const payload = body === undefined ? null : JSON.stringify(body);
    const req = https.request(
      {
        hostname: api.hostname,
        port: api.port || 443,
        path: pathname,
        method,
        ca,
        headers: {
          Accept: "application/json",
          ...(payload ? { "Content-Type": "application/json", "Content-Length": Buffer.byteLength(payload) } : {}),
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
      },
      (res) => {
        let text = "";
        res.on("data", (chunk) => { text += chunk; });
        res.on("end", () => {
          let json = null;
          try { json = JSON.parse(text); } catch { /* respons non-JSON tidak ditampilkan */ }
          resolve({ status: res.statusCode, json });
        });
      }
    );
    req.on("error", (err) => reject(new Error(`Koneksi gagal: ${err.message}`)));
    req.setTimeout(20000, () => req.destroy(new Error("Batas waktu permintaan.")));
    if (payload) req.write(payload);
    req.end();
  });
}

function kodeGalat(res) {
  return res.json?.error?.code ?? `HTTP_${res.status}`;
}

async function bacaTersembunyi(prompt) {
  if (!process.stdin.isTTY) throw new Error("OTP harus dimasukkan dari terminal interaktif.");
  process.stdout.write(prompt);
  process.stdin.setRawMode(true);
  process.stdin.resume();
  process.stdin.setEncoding("utf8");
  return new Promise((resolve, reject) => {
    let buffer = "";
    const selesai = (fn, value) => {
      process.stdin.setRawMode(false);
      process.stdin.pause();
      process.stdin.removeListener("data", onData);
      process.stdout.write("\n");
      fn(value);
    };
    const onData = (chunk) => {
      for (const ch of chunk) {
        if (ch === "\u0003") { selesai(reject, new Error("Dibatalkan.")); return; }
        if (ch === "\r" || ch === "\n") { selesai(resolve, buffer); return; }
        if (ch === "\u007f") { buffer = buffer.slice(0, -1); continue; }
        if (/^[0-9]$/.test(ch) && buffer.length < 8) buffer += ch;
      }
    };
    process.stdin.on("data", onData);
  });
}

let accessToken = "";
let refreshToken = "";
try {
  const minta = await request("POST", "/api/v1/auth/email/request", { email: adminEmail });
  if (minta.status !== 202 || !minta.json?.data?.challenge_id) {
    throw new Error(`Permintaan OTP ditolak (${kodeGalat(minta)}).`);
  }
  const challengeId = minta.json.data.challenge_id;
  console.log("Jika akun admin memenuhi syarat, OTP dikirim. Periksa kotak masuk dan spam.");
  const code = await bacaTersembunyi("Kode OTP 8 digit (tersembunyi): ");
  if (!/^\d{8}$/.test(code)) throw new Error("Kode harus 8 digit.");

  const verifikasi = await request("POST", "/api/v1/auth/email/verify", {
    challenge_id: challengeId,
    code,
  });
  if (verifikasi.status !== 200 || !verifikasi.json?.data?.access_token || !verifikasi.json?.data?.refresh_token) {
    throw new Error(`Verifikasi OTP ditolak (${kodeGalat(verifikasi)}).`);
  }
  accessToken = verifikasi.json.data.access_token;
  refreshToken = verifikasi.json.data.refresh_token;

  const undang = await request("POST", "/api/v1/users", { email, display_name: name, role }, accessToken);
  if (undang.status !== 201) {
    throw new Error(`Undangan ditolak (${kodeGalat(undang)}).`);
  }
  const user = undang.json?.data ?? {};
  console.log(`PASSED: akun ${email} dibuat dengan status ${user.status ?? "invited"} dan role ${user.role ?? role}.`);
  console.log(`user_id=${user.id ?? "(tidak tersedia)"}`);
  console.log("Akun menjadi active pada login OTP pertama. OTP dan token tidak dicetak.");
} catch (err) {
  console.error(`FAILED: ${err.message}`);
  process.exitCode = 1;
} finally {
  if (refreshToken) {
    try {
      const logout = await request("POST", "/api/v1/auth/logout", { refresh_token: refreshToken });
      if (logout.status !== 204) throw new Error("Pencabutan sesi server belum terkonfirmasi.");
    } catch {
      console.error("WARNING: pencabutan sesi server belum terkonfirmasi; minta Admin mencabut sesi CLI ini.");
      process.exitCode = 1;
    }
  }
  accessToken = "";
  refreshToken = "";
  process.stdin.pause();
}

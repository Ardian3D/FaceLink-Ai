/**
 * Klien endpoint autentikasi Go Backend FaceLink AI (`docs/API_CONTRACT.md` §5.1).
 *
 * Satu tempat untuk empat panggilan yang membentuk alur masuk:
 *
 *  1. `POST /auth/nonce`   — nonce sekali pakai (TTL 5 menit, disimpan backend
 *                            sebagai hash). Nonce **wajib** dikirim ke Google dan
 *                            kembali bersama ID token; backend menolak nonce yang
 *                            tidak pernah diterbitkannya.
 *  2. `POST /auth/google`  — menukar Google ID token menjadi sesi aplikasi.
 *  3. `POST /auth/refresh` — merotasi pasangan token.
 *  4. `POST /auth/logout`  — mencabut sesi yang sedang dipakai.
 *  5. `GET  /me`           — membaca identitas dan cakupan kelas.
 *
 * **Yang tidak dilakukan modul ini:** menyimpan token (itu `tokenStore`),
 * menebak role (role selalu dari backend), dan mengirim apa pun ke Google.
 *
 * Kegagalan dikembalikan sebagai nilai, bukan dilempar: alur masuk punya lima
 * keadaan UI yang berbeda dan pemanggil harus dapat membedakannya tanpa
 * membaca teks pesan.
 */
import { konfigurasiApi } from "@/features/api/config";
import { adalahObjek, bacaJson, mintaHttp, type JawabanHttp } from "@/features/api/http";
import type { AccountStatus, CurrentUser, Role, SessionError } from "@/types";

/** Waktu tunggu khusus login: satu percobaan mencakup verifikasi JWKS di backend. */
const TIMEOUT_AUTH_MS = 15_000;

export type HasilAuth<T> =
  | { ok: true; data: T }
  | { ok: false; error: SessionError };

/** Sesi yang diterbitkan backend setelah pertukaran ID token berhasil. */
export type SesiBackend = {
  accessToken: string;
  refreshToken: string;
  /** Waktu kedaluwarsa token akses, dihitung dari `expires_in` milik backend. */
  expiresAtMs: number;
  user: CurrentUser;
};

const ROLE_DIKENAL: readonly string[] = ["admin", "dosen", "mahasiswa"];
const STATUS_DIKENAL: readonly string[] = ["invited", "active", "suspended"];

/**
 * Alamat backend yang berlaku. Kesalahan konfigurasi dilaporkan sebagai
 * keadaan tersendiri, bukan sebagai "jaringan mati": pengguna tidak dapat
 * memperbaiki alamat backend dari HP, dan pesan yang menyalahkan Wi-Fi akan
 * menyesatkan.
 */
function alamatBackend(): { ok: true; baseUrl: string } | { ok: false; error: SessionError } {
  if (!konfigurasiApi.ok) {
    return {
      ok: false,
      error: { kind: "config", message: konfigurasiApi.alasan },
    };
  }
  // HTTP loopback tetap boleh untuk diagnostik tanpa token, bukan autentikasi.
  // Metro juga memakai HTTP, tetapi tidak melewati klien ini.
  if (new URL(konfigurasiApi.baseUrl).protocol !== "https:") {
    return {
      ok: false,
      error: { kind: "config", message: "Autentikasi memerlukan HTTPS dengan sertifikat tepercaya." },
    };
  }
  return { ok: true, baseUrl: konfigurasiApi.baseUrl };
}

/** Membaca `{ error: { code, message } }`; `null` bila bentuknya bukan itu. */
function bacaGalat(teks: string): { code: string; message: string } | null {
  const isi = bacaJson(teks);
  if (!adalahObjek(isi) || !adalahObjek(isi.error)) return null;
  const { code, message } = isi.error;
  if (typeof code !== "string") return null;
  return { code, message: typeof message === "string" ? message : "" };
}

/**
 * Menerjemahkan kegagalan transport menjadi keadaan UI.
 *
 * Galat sertifikat dipisahkan dari "tidak terjangkau" karena tindakannya
 * berbeda: pada kasus TLS, backend hidup dan alamatnya benar — yang salah
 * adalah sertifikat atau kepercayaan perangkat. Menyuruh pengguna memeriksa
 * `adb reverse` untuk kegagalan itu hanya membuang waktu.
 */
function galatTransport(jawaban: Extract<JawabanHttp, { jenis: "jaringan" | "timeout" }>): SessionError {
  if (jawaban.jenis === "timeout") {
    return { kind: "network" };
  }
  if (/certificate|ssl|tls|trust anchor|self.signed/i.test(jawaban.pesan)) {
    return { kind: "tls" };
  }
  return { kind: "network" };
}

/**
 * Menerjemahkan status HTTP + kode galat backend menjadi keadaan UI.
 *
 * `IDENTITY_NOT_LINKED` sengaja **tidak** dibedakan menjadi "belum diundang"
 * dan "ditangguhkan": kontrak memakai satu jawaban generik untuk mencegah
 * enumerasi akun, dan aplikasi tidak boleh menebak lebih banyak daripada yang
 * diberitahukan backend.
 */
function galatHttp(
  status: number,
  teks: string,
  emailDariGoogle?: string
): SessionError {
  const galat = bacaGalat(teks);
  if (status === 403 && galat?.code === "IDENTITY_NOT_LINKED") {
    return emailDariGoogle === undefined
      ? { kind: "notGranted" }
      : { kind: "notGranted", email: emailDariGoogle };
  }
  if (status === 503) {
    return { kind: "unavailable", message: galat?.message ?? "" };
  }
  // 401 pada endpoint bersesi (/me, /refresh, /logout) berarti sesi tidak
  // berlaku lagi; pada /auth/google artinya ID token Google yang ditolak.
  // Keduanya berbeda tindakan, jadi dibedakan lewat kode — bukan status saja.
  if (status === 401) {
    if (galat?.code === "IDENTITY_TOKEN_INVALID") {
      return { kind: "identityRejected" };
    }
    return { kind: "sessionEnded" };
  }
  return { kind: "unknown", message: galat?.code ?? `HTTP ${status}` };
}

function keCurrentUser(nilai: unknown): CurrentUser | null {
  if (!adalahObjek(nilai)) return null;
  const { id, email, display_name, role, status, person_id, class_ids } = nilai;
  if (typeof id !== "string" || id === "") return null;
  if (typeof display_name !== "string") return null;
  if (typeof role !== "string" || !ROLE_DIKENAL.includes(role)) return null;
  if (typeof status !== "string" || !STATUS_DIKENAL.includes(status)) return null;
  if (person_id !== null && typeof person_id !== "string") return null;
  if (!Array.isArray(class_ids) || class_ids.some((k) => typeof k !== "string")) {
    return null;
  }
  return {
    id,
    // `email` memakai `omitempty` pada backend; ketiadaannya bukan galat.
    email: typeof email === "string" ? email : "",
    displayName: display_name,
    role: role as Role,
    status: status as AccountStatus,
    personId: person_id,
    classIds: class_ids as string[],
  };
}

/** Membaca amplop sukses `{ data: ... }`. */
function bacaData(teks: string): unknown {
  const isi = bacaJson(teks);
  if (!adalahObjek(isi)) return null;
  return isi.data ?? null;
}

/**
 * `POST /auth/nonce` — meminta nonce sekali pakai untuk satu percobaan login.
 *
 * Nonce yang dikembalikan **harus** dikirim ke Google sebagai parameter
 * `nonce` pada permintaan otorisasi, lalu dikirim kembali ke backend bersama ID
 * token. Satu nonce untuk satu percobaan: percobaan ulang memerlukan nonce
 * baru, karena nonce lama mungkin sudah terpakai.
 */
export async function mintaNonce(): Promise<HasilAuth<string>> {
  const alamat = alamatBackend();
  if (!alamat.ok) return { ok: false, error: alamat.error };

  const jawaban = await mintaHttp("/api/v1/auth/nonce", {
    baseUrl: alamat.baseUrl,
    method: "POST",
    timeoutMs: TIMEOUT_AUTH_MS,
  });
  if (jawaban.jenis !== "jawaban") {
    return { ok: false, error: galatTransport(jawaban) };
  }
  if (jawaban.status !== 200) {
    return { ok: false, error: galatHttp(jawaban.status, jawaban.teks) };
  }
  const data = bacaData(jawaban.teks);
  if (!adalahObjek(data) || typeof data.nonce !== "string" || data.nonce === "") {
    return { ok: false, error: { kind: "unknown", message: "bentuk_respons_nonce" } };
  }
  return { ok: true, data: data.nonce };
}

/**
 * `POST /auth/google` — menukar ID token Google menjadi sesi FaceLink AI.
 *
 * `email` dari klaim Google hanya dipakai untuk **ditampilkan** pada layar
 * penolakan; ia tidak pernah dikirim balik ke backend dan tidak menentukan
 * role. Backend tetap memakai `sub` sebagai kunci identitas.
 */
export async function tukarIdToken(
  idToken: string,
  nonce: string,
  emailGoogle?: string
): Promise<HasilAuth<SesiBackend>> {
  const alamat = alamatBackend();
  if (!alamat.ok) return { ok: false, error: alamat.error };

  const jawaban = await mintaHttp("/api/v1/auth/google", {
    baseUrl: alamat.baseUrl,
    method: "POST",
    body: { id_token: idToken, nonce },
    timeoutMs: TIMEOUT_AUTH_MS,
  });
  if (jawaban.jenis !== "jawaban") {
    return { ok: false, error: galatTransport(jawaban) };
  }
  if (jawaban.status !== 200) {
    return { ok: false, error: galatHttp(jawaban.status, jawaban.teks, emailGoogle) };
  }
  return bacaSesi(jawaban.teks);
}

/** Satu parser sesi untuk Google dan Email OTP; role selalu dari server. */
function bacaSesi(teks: string): HasilAuth<SesiBackend> {
  const data = bacaData(teks);
  if (!adalahObjek(data)) {
    return { ok: false, error: { kind: "unknown", message: "bentuk_respons_login" } };
  }
  const { access_token, refresh_token, expires_in } = data;
  const user = keCurrentUser(data.user);
  if (
    typeof access_token !== "string" ||
    access_token === "" ||
    typeof refresh_token !== "string" ||
    refresh_token === "" ||
    typeof expires_in !== "number" ||
    !Number.isFinite(expires_in) ||
    user === null
  ) {
    return { ok: false, error: { kind: "unknown", message: "bentuk_respons_login" } };
  }
  return {
    ok: true,
    data: {
      accessToken: access_token,
      refreshToken: refresh_token,
      // `expires_in` dihitung backend dari jam server; jam lokal hanya dipakai
      // untuk menyegarkan lebih awal, bukan untuk memutuskan keabsahan.
      expiresAtMs: Date.now() + expires_in * 1000,
      user,
    },
  };
}

export type EmailOtpError =
  | SessionError
  | { kind: "otpInvalid" }
  | { kind: "validation" }
  | { kind: "rateLimited"; retryAfter: number };

export type HasilEmail<T> =
  | { ok: true; data: T }
  | { ok: false; error: EmailOtpError };

export type EmailChallenge = {
  challengeId: string;
  expiresIn: number;
  retryAfter: number;
};

function galatEmail(jawaban: Extract<JawabanHttp, { jenis: "jawaban" }>): EmailOtpError {
  const galat = bacaGalat(jawaban.teks);
  if (jawaban.status === 429) {
    const header = jawaban.retryAfter?.trim() ?? "";
    const tanggal = Date.parse(header);
    const server = Date.parse(jawaban.serverDate ?? "");
    const detik = /^\d+$/.test(header)
      ? Number(header)
      : Math.ceil((tanggal - (Number.isFinite(server) ? server : Date.now())) / 1000);
    return { kind: "rateLimited", retryAfter: Number.isFinite(detik) ? Math.max(1, detik) : 60 };
  }
  if (jawaban.status === 401) return { kind: "otpInvalid" };
  if (jawaban.status === 400) return { kind: "validation" };
  if (jawaban.status === 403 && galat?.code === "HTTPS_REQUIRED") {
    return { kind: "config", message: "Autentikasi memerlukan HTTPS dengan sertifikat tepercaya." };
  }
  if (jawaban.status === 503) return { kind: "unavailable", message: "" };
  return { kind: "unknown", message: "respons_autentikasi" };
}

export async function mintaKodeEmail(email: string, signal?: AbortSignal): Promise<HasilEmail<EmailChallenge>> {
  const alamat = alamatBackend();
  if (!alamat.ok) return alamat;
  if (signal?.aborted) return { ok: false, error: { kind: "cancelled" } };
  const jawaban = await mintaHttp("/api/v1/auth/email/request", {
    baseUrl: alamat.baseUrl,
    method: "POST",
    body: { email: email.trim().toLowerCase() },
    timeoutMs: TIMEOUT_AUTH_MS,
    signal,
  });
  if (signal?.aborted) return { ok: false, error: { kind: "cancelled" } };
  if (jawaban.jenis !== "jawaban") return { ok: false, error: galatTransport(jawaban) };
  if (jawaban.status !== 202) return { ok: false, error: galatEmail(jawaban) };
  const data = bacaData(jawaban.teks);
  if (
    !adalahObjek(data) ||
    typeof data.challenge_id !== "string" ||
    !/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(data.challenge_id) ||
    typeof data.expires_in !== "number" || !Number.isSafeInteger(data.expires_in) || data.expires_in <= 0 ||
    typeof data.retry_after !== "number" || !Number.isSafeInteger(data.retry_after) || data.retry_after < 0
  ) {
    return { ok: false, error: { kind: "unknown", message: "bentuk_respons_challenge" } };
  }
  return { ok: true, data: {
    challengeId: data.challenge_id,
    expiresIn: data.expires_in,
    retryAfter: data.retry_after,
  } };
}

export async function verifikasiKodeEmail(
  challengeId: string, code: string, signal?: AbortSignal
): Promise<HasilEmail<SesiBackend>> {
  const alamat = alamatBackend();
  if (!alamat.ok) return alamat;
  if (signal?.aborted) return { ok: false, error: { kind: "cancelled" } };
  const jawaban = await mintaHttp("/api/v1/auth/email/verify", {
    baseUrl: alamat.baseUrl,
    method: "POST",
    body: { challenge_id: challengeId, code },
    timeoutMs: TIMEOUT_AUTH_MS,
    signal,
  });
  // A successful session must reach AuthFlow even after cancellation so it can
  // be discarded/revoked there instead of silently retaining credentials.
  if (jawaban.jenis !== "jawaban") {
    return { ok: false, error: signal?.aborted ? { kind: "cancelled" } : galatTransport(jawaban) };
  }
  if (jawaban.status !== 200) return { ok: false, error: galatEmail(jawaban) };
  return bacaSesi(jawaban.teks);
}

/**
 * `POST /auth/refresh` — merotasi pasangan token.
 *
 * Respons backend hanya memuat pasangan token (tanpa `user`), sesuai kontrak:
 * identitas tidak berubah oleh rotasi, dan pemanggil memakai `GET /me` bila
 * memerlukannya. Karena itu fungsi ini mengembalikan token saja.
 */
export async function segarkanSesi(
  refreshToken: string
): Promise<HasilAuth<{ accessToken: string; refreshToken: string; expiresAtMs: number }>> {
  const alamat = alamatBackend();
  if (!alamat.ok) return { ok: false, error: alamat.error };

  const jawaban = await mintaHttp("/api/v1/auth/refresh", {
    baseUrl: alamat.baseUrl,
    method: "POST",
    body: { refresh_token: refreshToken },
    timeoutMs: TIMEOUT_AUTH_MS,
  });
  if (jawaban.jenis !== "jawaban") {
    return { ok: false, error: galatTransport(jawaban) };
  }
  if (jawaban.status !== 200) {
    return { ok: false, error: galatHttp(jawaban.status, jawaban.teks) };
  }
  const data = bacaData(jawaban.teks);
  if (!adalahObjek(data)) {
    return { ok: false, error: { kind: "unknown", message: "bentuk_respons_refresh" } };
  }
  const { access_token, refresh_token, expires_in } = data;
  if (
    typeof access_token !== "string" ||
    access_token === "" ||
    typeof refresh_token !== "string" ||
    refresh_token === "" ||
    typeof expires_in !== "number" ||
    !Number.isFinite(expires_in)
  ) {
    return { ok: false, error: { kind: "unknown", message: "bentuk_respons_refresh" } };
  }
  return {
    ok: true,
    data: {
      accessToken: access_token,
      refreshToken: refresh_token,
      expiresAtMs: Date.now() + expires_in * 1000,
    },
  };
}

/**
 * `GET /me` — membaca identitas dan cakupan kelas dengan token akses.
 *
 * Backend membaca role dan status dari database pada setiap permintaan
 * (D-033), sehingga fungsi ini juga menjadi cara mendeteksi akun yang
 * ditangguhkan setelah sesi terbit.
 */
export async function ambilSaya(accessToken: string): Promise<HasilAuth<CurrentUser>> {
  const alamat = alamatBackend();
  if (!alamat.ok) return { ok: false, error: alamat.error };

  const jawaban = await mintaHttp("/api/v1/me", {
    baseUrl: alamat.baseUrl,
    token: accessToken,
    timeoutMs: TIMEOUT_AUTH_MS,
  });
  if (jawaban.jenis !== "jawaban") {
    return { ok: false, error: galatTransport(jawaban) };
  }
  if (jawaban.status !== 200) {
    return { ok: false, error: galatHttp(jawaban.status, jawaban.teks) };
  }
  const user = keCurrentUser(bacaData(jawaban.teks));
  if (user === null) {
    return { ok: false, error: { kind: "unknown", message: "bentuk_respons_me" } };
  }
  return { ok: true, data: user };
}

/**
 * Revokes only the session proven by this refresh credential, even after
 * access expiry. A missing 204 is not proof of remote revocation.
 */
export async function keluarSesi(refreshToken: string): Promise<HasilAuth<void>> {
  const alamat = alamatBackend();
  if (!alamat.ok) return alamat;
  const jawaban = await mintaHttp("/api/v1/auth/logout", {
    baseUrl: alamat.baseUrl,
    method: "POST",
    body: { refresh_token: refreshToken },
    timeoutMs: TIMEOUT_AUTH_MS,
  });
  if (jawaban.jenis !== "jawaban") return { ok: false, error: galatTransport(jawaban) };
  if (jawaban.status !== 204) {
    return { ok: false, error: galatHttp(jawaban.status, jawaban.teks) };
  }
  return { ok: true, data: undefined };
}

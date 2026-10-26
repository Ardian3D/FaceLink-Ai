/**
 * Tiruan bersama untuk pengujian autentikasi.
 *
 * Tiruan di sini **mencatat panggilan** dan dapat diprogram gagal; tidak ada
 * yang otomatis berhasil. Setiap perilaku yang diuji harus menentukan sendiri
 * jawaban backend, karena pengujian yang selalu mendapat `ok: true` tidak
 * membuktikan apa pun.
 */
import type { CurrentUser, SessionError } from "@/types";
import type { ApiAuth, GoogleAuth, PenyimpananSesi, SesiPerangkat } from "@/features/auth/authFlow";
import type { HasilAuth, SesiBackend } from "@/features/auth/authApi";
import type { HasilGoogle } from "@/features/auth/googleSignIn";

export const MAHASISWA: CurrentUser = {
  id: "11111111-1111-4111-8111-111111111111",
  email: "mahasiswa@example.test",
  displayName: "Mahasiswa Uji",
  role: "mahasiswa",
  status: "active",
  personId: "p-1",
  classIds: ["c-1"],
};

export const DOSEN: CurrentUser = {
  ...MAHASISWA,
  id: "22222222-2222-4222-8222-222222222222",
  email: "dosen@example.test",
  displayName: "Dosen Uji",
  role: "dosen",
  personId: null,
  classIds: ["c-1"],
};

export const ADMIN: CurrentUser = {
  ...MAHASISWA,
  id: "33333333-3333-4333-8333-333333333333",
  email: "admin@example.test",
  displayName: "Admin Uji",
  role: "admin",
  personId: null,
  classIds: [],
};

export function sesiUntuk(user: CurrentUser, n = 1): SesiBackend {
  return {
    accessToken: `access-${n}`,
    refreshToken: `refresh-${n}`,
    expiresAtMs: Date.now() + 3_600_000,
    user,
  };
}

/** Menunda satu putaran microtask supaya urutan async dapat diamati. */
export function jeda(): Promise<void> {
  return new Promise((r) => setTimeout(r, 0));
}

export type CatatanApi = {
  nonce: number;
  tukar: Array<{ idToken: string; nonce: string; email?: string }>;
  segarkan: number;
  saya: string[];
  keluar: string[];
};

export function apiTiruan(
  jawaban: {
    nonce?: () => Promise<HasilAuth<string>>;
    tukar?: () => Promise<HasilAuth<SesiBackend>>;
    segarkan?: () => Promise<
      HasilAuth<{ accessToken: string; refreshToken: string; expiresAtMs: number }>
    >;
    saya?: () => Promise<HasilAuth<CurrentUser>>;
  } = {}
): { api: ApiAuth; catatan: CatatanApi } {
  const catatan: CatatanApi = { nonce: 0, tukar: [], segarkan: 0, saya: [], keluar: [] };
  let hitungSegarkan = 0;
  const api: ApiAuth = {
    async mintaNonce() {
      catatan.nonce += 1;
      return jawaban.nonce ? jawaban.nonce() : { ok: true, data: "nonce-dari-backend" };
    },
    async tukarIdToken(idToken, nonce, email) {
      catatan.tukar.push({ idToken, nonce, email });
      return jawaban.tukar ? jawaban.tukar() : { ok: true, data: sesiUntuk(MAHASISWA) };
    },
    async segarkanSesi() {
      catatan.segarkan += 1;
      hitungSegarkan += 1;
      return jawaban.segarkan
        ? jawaban.segarkan()
        : {
            ok: true,
            data: {
              accessToken: `access-baru-${hitungSegarkan}`,
              refreshToken: `refresh-baru-${hitungSegarkan}`,
              expiresAtMs: Date.now() + 3_600_000,
            },
          };
    },
    async ambilSaya(token) {
      catatan.saya.push(token);
      return jawaban.saya ? jawaban.saya() : { ok: true, data: MAHASISWA };
    },
    async keluarSesi(token) {
      catatan.keluar.push(token);
    },
  };
  return { api, catatan };
}

export type CatatanGoogle = { masuk: string[]; lupakan: number };

export function googleTiruan(
  jawaban: {
    periksa?: () => SessionError | null;
    masuk?: () => Promise<HasilGoogle>;
  } = {}
): { google: GoogleAuth; catatan: CatatanGoogle } {
  const catatan: CatatanGoogle = { masuk: [], lupakan: 0 };
  const google: GoogleAuth = {
    periksaDukungan: jawaban.periksa ?? (() => null),
    async masukDenganGoogle(nonce) {
      catatan.masuk.push(nonce);
      return jawaban.masuk ? jawaban.masuk() : { ok: true, idToken: "id-token", email: "a@b.test" };
    },
    async lupakanGoogle() {
      catatan.lupakan += 1;
    },
  };
  return { google, catatan };
}

export type PenyimpananUji = PenyimpananSesi & {
  isi: SesiPerangkat | null;
  simpanGagal: boolean;
  hitungSimpan: number;
  hitungHapus: number;
};

/**
 * Penyimpanan kredensial tiruan. `masihBerlaku` meniru margin 60 detik pada
 * implementasi nyata sehingga kedaluwarsa dapat dikendalikan uji.
 */
export function penyimpananTiruan(awal: SesiPerangkat | null = null): PenyimpananUji {
  const p: PenyimpananUji = {
    isi: awal,
    simpanGagal: false,
    hitungSimpan: 0,
    hitungHapus: 0,
    async muat() {
      return p.isi;
    },
    async simpan(sesi) {
      p.hitungSimpan += 1;
      if (p.simpanGagal) throw new Error("Keystore terkunci");
      p.isi = sesi;
    },
    async hapus() {
      p.hitungHapus += 1;
      p.isi = null;
    },
    masihBerlaku(sesi) {
      return sesi.expiresAtMs - 60_000 > Date.now();
    },
  };
  return p;
}

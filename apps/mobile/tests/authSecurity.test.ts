/**
 * Jaminan keamanan yang tidak terikat satu alur.
 *
 * Dua hal diuji di sini karena keduanya mudah rusak tanpa disadari:
 *
 *  1. **Tidak ada bypass.** Mode demo dan pemilih role sudah dihapus; tidak
 *     boleh ada jalur yang memberi sesi tanpa `POST /auth/google`.
 *  2. **Tidak ada kebocoran ke log.** Token dan email tidak boleh tercetak,
 *     termasuk saat kegagalan — pesan galat yang menyertakan token akan
 *     berakhir di log perangkat atau laporan crash.
 */
import fs from "node:fs";
import path from "node:path";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AuthFlow } from "@/features/auth/authFlow";
import { apiTiruan, googleTiruan, penyimpananTiruan, sesiUntuk, MAHASISWA } from "./tiruan";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("tidak ada bypass masuk", () => {
  it("modul mode demo sudah tidak ada dan tidak dirujuk lagi", async () => {
    // Kalau modul ini pernah dikembalikan, jalur masuk tanpa backend ikut
    // kembali. Diperiksa di berkas, bukan lewat impor dinamis, supaya
    // pemeriksa tipe tidak menuntut modul yang memang sudah dihapus.
    const akar = path.resolve(import.meta.dirname, "../src/features/auth");
    expect(fs.existsSync(path.join(akar, "demoMode.ts"))).toBe(false);
    expect(fs.existsSync(path.join(akar, "demoAccounts.ts"))).toBe(false);

    // Tidak ada berkas sumber yang masih merujuk modul itu. Daftar berkas
    // dikumpulkan sendiri agar tidak bergantung pada perilaku Dirent versi Node.
    const akarSumber = path.resolve(import.meta.dirname, "../src");
    const akarApp = path.resolve(import.meta.dirname, "../app");
    const kumpulkan = (dir: string): string[] =>
      fs.readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
        const penuh = path.join(dir, e.name);
        if (e.isDirectory()) return kumpulkan(penuh);
        return /\.(ts|tsx)$/.test(e.name) ? [penuh] : [];
      });
    const merujuk = [...kumpulkan(akarSumber), ...kumpulkan(akarApp)].filter((berkas) => {
      const isi = fs.readFileSync(berkas, "utf8");
      return isi.includes("demoMode") || isi.includes("demoAccounts");
    });
    expect(merujuk).toEqual([]);
  });

  it("tidak ada sesi yang terbit tanpa pertukaran ID token", async () => {
    const { api, catatan } = apiTiruan();
    const { google } = googleTiruan({
      masuk: async () => ({ ok: true, idToken: "id-token" }),
    });
    const store = penyimpananTiruan();
    const alur = new AuthFlow(api, google, store, () => {});
    await alur.pulihkan();
    await alur.masuk();

    // (a) backend dipanggil untuk nonce dan pertukaran,
    expect(catatan.nonce).toBe(1);
    expect(catatan.tukar).toHaveLength(1);
    // (b) sesi yang terbit berasal dari backend, bukan dari Google atau klien.
    expect(alur.keadaan().session?.user.id).toBe(MAHASISWA.id);
  });

  it("menolak masuk ketika backend tidak dapat menerbitkan sesi", async () => {
    const { api } = apiTiruan({
      tukar: async () => ({ ok: false, error: { kind: "identityRejected" } }),
    });
    const { google } = googleTiruan();
    const store = penyimpananTiruan();
    const alur = new AuthFlow(api, google, store, () => {});
    await alur.pulihkan();
    await alur.masuk();

    expect(alur.keadaan().status).toBe("failed");
    expect(alur.keadaan().session).toBeNull();
    expect(store.isi).toBeNull();
  });
});

describe("tidak ada token di log", () => {
  it("tidak mencetak token saat masuk, memulihkan, dan keluar", async () => {
    const log = vi.spyOn(console, "log").mockImplementation(() => {});
    const info = vi.spyOn(console, "info").mockImplementation(() => {});
    const debug = vi.spyOn(console, "debug").mockImplementation(() => {});
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const error = vi.spyOn(console, "error").mockImplementation(() => {});

    const { api } = apiTiruan({ tukar: async () => ({ ok: true, data: sesiUntuk(MAHASISWA, 7) }) });
    const { google } = googleTiruan();
    const store = penyimpananTiruan();
    const alur = new AuthFlow(api, google, store, () => {});

    await alur.pulihkan();
    await alur.masuk();
    await alur.tokenAkses();
    await alur.keluar();

    const semua = [log, info, debug, warn, error].flatMap((s) => s.mock.calls);
    const teks = JSON.stringify(semua);
    expect(teks).not.toContain("access-7");
    expect(teks).not.toContain("refresh-7");
    expect(teks).not.toContain("id-token");
    expect(teks).not.toContain("nonce-dari-backend");
    // Tidak ada logging sama sekali pada jalur autentikasi: diam lebih aman
    // daripada mencoba menyaring nilai sensitif dari pesan.
    expect(semua).toHaveLength(0);
  });

  it("tidak membocorkan token lewat pesan galat penyimpanan", async () => {
    const { api } = apiTiruan();
    const { google } = googleTiruan();
    const store = penyimpananTiruan();
    store.simpan = async () => {
      throw new Error("gagal");
    };
    const alur = new AuthFlow(api, google, store, () => {});

    await alur.pulihkan();
    await alur.masuk();

    const pesan = JSON.stringify(alur.keadaan().error);
    expect(pesan).not.toContain("access-");
    expect(pesan).not.toContain("refresh-");
  });
});

describe("state yang terlihat UI", () => {
  it("tidak pernah memuat token di objek sesi", async () => {
    const { api } = apiTiruan();
    const { google } = googleTiruan();
    const alur = new AuthFlow(api, google, penyimpananTiruan(), () => {});
    await alur.pulihkan();
    await alur.masuk();

    const sesi = alur.keadaan().session;
    expect(sesi).not.toBeNull();
    expect(Object.keys(sesi ?? {})).toEqual(["user"]);
    expect(JSON.stringify(sesi)).not.toMatch(/access-|refresh-/);
  });
});

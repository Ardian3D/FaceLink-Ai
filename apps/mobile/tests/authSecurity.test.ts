/**
 * Jaminan keamanan yang tidak terikat satu alur.
 *
 * Dua hal diuji di sini karena keduanya mudah rusak tanpa disadari:
 *
 *  1. **Tidak ada bypass.** Sesi hanya dapat terbit setelah pertukaran yang
 *     berhasil dengan backend; penolakan tidak boleh menghasilkan sesi lokal.
 *  2. **Tidak ada kebocoran ke log.** Token dan email tidak boleh tercetak,
 *     termasuk saat kegagalan — pesan galat yang menyertakan token akan
 *     berakhir di log perangkat atau laporan crash.
 */
import { afterEach, describe, expect, it, vi } from "vitest";
import { AuthFlow } from "@/features/auth/authFlow";
import { apiTiruan, googleTiruan, penyimpananTiruan, sesiUntuk, MAHASISWA } from "./tiruan";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("tidak ada bypass masuk", () => {

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

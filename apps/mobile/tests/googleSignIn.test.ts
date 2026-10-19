/**
 * Adapter Google Sign-In.
 *
 * Modul native tidak dijalankan di sini. Yang diuji adalah keputusan adapter:
 * nonce diteruskan mentah, client ID yang dipakai bertipe Web, urutan panggilan
 * sesuai anjuran pustaka, dan setiap kelas kegagalan diterjemahkan ke keadaan
 * UI yang berbeda.
 */
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  hambatanMasuk,
  jalankanMasukGoogle,
  type ModulGoogle,
} from "@/features/auth/googleSignIn";

const OK_KONFIG = {
  ok: true as const,
  webClientId: "123-abc.apps.googleusercontent.com",
};

const status = {
  PLAY_SERVICES_NOT_AVAILABLE: "PLAY_SERVICES_NOT_AVAILABLE",
  SIGN_IN_CANCELLED: "SIGN_IN_CANCELLED",
};

type JawabanGoogle = Awaited<ReturnType<ModulGoogle["GoogleOneTapSignIn"]["signIn"]>>;

function modulTiruan(): {
  modul: ModulGoogle;
  configure: ReturnType<typeof vi.fn>;
  signIn: ReturnType<typeof vi.fn>;
  createAccount: ReturnType<typeof vi.fn>;
  checkPlayServices: ReturnType<typeof vi.fn>;
} {
  const configure = vi.fn();
  const signIn = vi.fn();
  const createAccount = vi.fn();
  const checkPlayServices = vi.fn();
  const modul: ModulGoogle = {
    GoogleOneTapSignIn: {
      configure,
      checkPlayServices,
      signIn,
      createAccount,
      signOut: vi.fn(),
    },
    isErrorWithCode: (e): e is { code: string; message: string } =>
      typeof e === "object" && e !== null && "code" in (e as Record<string, unknown>),
    statusCodes: status,
  };
  return { modul, configure, signIn, createAccount, checkPlayServices };
}

const sukses = (idToken: string, email: string | null = "a@b.test"): JawabanGoogle => ({
  type: "success",
  data: { idToken, user: { email } },
});

beforeEach(() => {
  vi.restoreAllMocks();
});

describe("hambatanMasuk", () => {
  it("menolak web sebelum apa pun dijalankan", () => {
    expect(hambatanMasuk("web", OK_KONFIG, true)).toEqual({ kind: "unsupported" });
  });

  it("melaporkan konfigurasi client ID yang tidak sah", () => {
    expect(
      hambatanMasuk("android", { ok: false, alasan: "client ID Android" }, true)
    ).toEqual({ kind: "config", message: "client ID Android" });
  });

  it("melaporkan modul native yang tidak tertaut", () => {
    expect(hambatanMasuk("android", OK_KONFIG, false)).toEqual({ kind: "unsupported" });
  });

  it("mengizinkan Android dengan konfigurasi dan modul lengkap", () => {
    expect(hambatanMasuk("android", OK_KONFIG, true)).toBeNull();
  });
});

describe("jalankanMasukGoogle", () => {
  it("meneruskan nonce backend apa adanya ke Credential Manager", async () => {
    const t = modulTiruan();
    t.signIn.mockResolvedValue(sukses("id-token"));

    const hasil = await jalankanMasukGoogle(t.modul, OK_KONFIG, "nonce-dari-backend");

    expect(hasil).toEqual({ ok: true, idToken: "id-token", email: "a@b.test" });
    // Backend membandingkan klaim `nonce` dengan nilai yang ia terbitkan; hash
    // di sini akan membuat setiap login gagal.
    expect(t.configure).toHaveBeenCalledWith(
      expect.objectContaining({ nonce: "nonce-dari-backend" })
    );
  });

  it("memakai client ID Web dan tidak meminta refresh token Google", async () => {
    const t = modulTiruan();
    t.signIn.mockResolvedValue(sukses("id-token"));
    await jalankanMasukGoogle(t.modul, OK_KONFIG, "n");

    expect(t.configure).toHaveBeenCalledWith(
      expect.objectContaining({
        webClientId: OK_KONFIG.webClientId,
        offlineAccess: false,
        autoSelectOnSignIn: false,
      })
    );
  });

  it("memeriksa Play Services sebelum membuka lembar akun", async () => {
    const t = modulTiruan();
    const urutan: string[] = [];
    t.checkPlayServices.mockImplementation(async () => urutan.push("play"));
    t.signIn.mockImplementation(async () => {
      urutan.push("signIn");
      return sukses("id-token");
    });
    await jalankanMasukGoogle(t.modul, OK_KONFIG, "n");
    expect(urutan).toEqual(["play", "signIn"]);
  });

  it("mencoba signIn lalu createAccount bila belum ada kredensial tersimpan", async () => {
    const t = modulTiruan();
    t.signIn.mockResolvedValue({ type: "noSavedCredentialFound", data: null });
    t.createAccount.mockResolvedValue(sukses("id-token", null));

    const hasil = await jalankanMasukGoogle(t.modul, OK_KONFIG, "n");

    expect(hasil).toEqual({ ok: true, idToken: "id-token" });
    expect(t.createAccount).toHaveBeenCalledTimes(1);
  });

  it("tidak memanggil createAccount bila signIn sudah berhasil", async () => {
    const t = modulTiruan();
    t.signIn.mockResolvedValue(sukses("id-token"));
    await jalankanMasukGoogle(t.modul, OK_KONFIG, "n");
    expect(t.createAccount).not.toHaveBeenCalled();
  });

  it("melaporkan pembatalan pengguna sebagai pembatalan, bukan kegagalan", async () => {
    const t = modulTiruan();
    t.signIn.mockResolvedValue({ type: "cancelled", data: null });
    expect(await jalankanMasukGoogle(t.modul, OK_KONFIG, "n")).toEqual({
      ok: false,
      error: { kind: "cancelled" },
    });
  });

  it("menerjemahkan galat Play Services", async () => {
    const t = modulTiruan();
    t.checkPlayServices.mockRejectedValue({
      code: status.PLAY_SERVICES_NOT_AVAILABLE,
      message: "perlu diperbarui",
    });
    expect(await jalankanMasukGoogle(t.modul, OK_KONFIG, "n")).toEqual({
      ok: false,
      error: { kind: "playServices" },
    });
  });

  it("menerjemahkan galat pembatalan dari pustaka", async () => {
    const t = modulTiruan();
    t.signIn.mockRejectedValue({ code: status.SIGN_IN_CANCELLED, message: "batal" });
    expect(await jalankanMasukGoogle(t.modul, OK_KONFIG, "n")).toEqual({
      ok: false,
      error: { kind: "cancelled" },
    });
  });

  it("meneruskan kode galat yang tidak dikenal apa adanya", async () => {
    const t = modulTiruan();
    t.signIn.mockRejectedValue({ code: "ONE_TAP_START_FAILED", message: "gagal" });
    expect(await jalankanMasukGoogle(t.modul, OK_KONFIG, "n")).toMatchObject({
      ok: false,
      error: { kind: "unknown", message: "ONE_TAP_START_FAILED" },
    });
  });

  it("menolak keberhasilan tanpa ID token", async () => {
    const t = modulTiruan();
    t.signIn.mockResolvedValue({ type: "success", data: { idToken: "", user: { email: null } } });
    expect(await jalankanMasukGoogle(t.modul, OK_KONFIG, "n")).toEqual({
      ok: false,
      error: { kind: "unknown", message: "google_tanpa_id_token" },
    });
  });

  it("menolak konfigurasi yang tidak sah tanpa menyentuh modul", async () => {
    const t = modulTiruan();
    const hasil = await jalankanMasukGoogle(
      t.modul,
      { ok: false, alasan: "client ID Android" },
      "n"
    );
    expect(hasil).toMatchObject({ ok: false, error: { kind: "config" } });
    expect(t.configure).not.toHaveBeenCalled();
  });
});

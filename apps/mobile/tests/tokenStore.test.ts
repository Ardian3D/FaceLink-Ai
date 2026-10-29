/**
 * Penyimpanan token di perangkat.
 *
 * `expo-secure-store` diganti tiruan karena modul aslinya native. Yang diuji
 * adalah **aturan**-nya: token hanya lewat SecureStore, data rusak dibuang,
 * dan platform tanpa penyimpanan kredensial tidak pernah jatuh ke penyimpanan
 * biasa.
 */
import { beforeEach, describe, expect, it, vi } from "vitest";

/** Tiruan SecureStore. `tersedia: false` meniru bundel web. */
const toko = {
  isi: new Map<string, string>(),
  tersedia: true,
  gagalTulis: false,
  getItemAsync: vi.fn(),
  setItemAsync: vi.fn(),
  deleteItemAsync: vi.fn(),
  isAvailableAsync: vi.fn(),
  WHEN_UNLOCKED_THIS_DEVICE_ONLY: "when-unlocked-this-device-only",
};

function pasangTiruan(): void {
  vi.doMock("expo-secure-store", () => ({
    isAvailableAsync: toko.isAvailableAsync,
    getItemAsync: toko.getItemAsync,
    setItemAsync: toko.setItemAsync,
    deleteItemAsync: toko.deleteItemAsync,
    WHEN_UNLOCKED_THIS_DEVICE_ONLY: toko.WHEN_UNLOCKED_THIS_DEVICE_ONLY,
  }));
}

async function muat() {
  vi.resetModules();
  pasangTiruan();
  return await import("@/features/auth/tokenStore");
}

beforeEach(() => {
  toko.isi.clear();
  toko.tersedia = true;
  toko.gagalTulis = false;
  toko.isAvailableAsync.mockReset().mockImplementation(async () => toko.tersedia);
  toko.getItemAsync.mockReset().mockImplementation(async (kunci: string) => toko.isi.get(kunci) ?? null);
  toko.setItemAsync
    .mockReset()
    .mockImplementation(async (kunci: string, nilai: string) => {
      if (toko.gagalTulis) throw new Error("Keystore terkunci");
      toko.isi.set(kunci, nilai);
    });
  toko.deleteItemAsync.mockReset().mockImplementation(async (kunci: string) => {
    toko.isi.delete(kunci);
  });
});

describe("menyimpan sesi", () => {
  it("menulis token lewat SecureStore dengan opsi perangkat ini saja", async () => {
    const { simpanSesi } = await muat();
    await simpanSesi({ accessToken: "a", refreshToken: "r", expiresAtMs: 1_000 });

    expect(toko.setItemAsync).toHaveBeenCalledTimes(1);
    const [kunci, nilai, opsi] = toko.setItemAsync.mock.calls[0] ?? [];
    // Kunci tidak menyebut istilah token agar tidak memberi petunjuk.
    expect(kunci).toBe("facelink.session.v1");
    expect(JSON.parse(String(nilai))).toEqual({
      accessToken: "a",
      refreshToken: "r",
      expiresAtMs: 1_000,
    });
    // Token tidak boleh ikut dicadangkan ke perangkat lain (D-033).
    expect(opsi).toMatchObject({ keychainAccessible: toko.WHEN_UNLOCKED_THIS_DEVICE_ONLY });
  });

  it("melempar, bukan menyimpan di tempat lain, saat penyimpanan aman tidak ada", async () => {
    toko.tersedia = false;
    const { simpanSesi } = await muat();
    await expect(
      simpanSesi({ accessToken: "a", refreshToken: "r", expiresAtMs: 1_000 })
    ).rejects.toThrow(/penyimpanan kredensial/);
    // Tidak ada fallback ke penyimpanan biasa.
    expect(toko.setItemAsync).not.toHaveBeenCalled();
  });

  it("meneruskan kegagalan Keystore kepada pemanggil", async () => {
    const { simpanSesi } = await muat();
    toko.gagalTulis = true;
    await expect(
      simpanSesi({ accessToken: "a", refreshToken: "r", expiresAtMs: 1_000 })
    ).rejects.toThrow("Keystore terkunci");
  });
});

describe("membaca sesi", () => {
  it("mengembalikan null saat belum pernah masuk", async () => {
    const { muatSesiTersimpan } = await muat();
    expect(await muatSesiTersimpan()).toBeNull();
  });

  it("mengembalikan sesi yang tersimpan sebelumnya", async () => {
    toko.isi.set(
      "facelink.session.v1",
      JSON.stringify({ accessToken: "a", refreshToken: "r", expiresAtMs: 42 })
    );
    const { muatSesiTersimpan } = await muat();
    expect(await muatSesiTersimpan()).toEqual({
      accessToken: "a",
      refreshToken: "r",
      expiresAtMs: 42,
    });
  });

  it("membuang data rusak alih-alih mengirimnya sebagai Bearer", async () => {
    toko.isi.set("facelink.session.v1", "{bukan json");
    const { muatSesiTersimpan } = await muat();
    expect(await muatSesiTersimpan()).toBeNull();
    expect(toko.deleteItemAsync).toHaveBeenCalledWith("facelink.session.v1");
  });

  it("menolak sesi dengan token kosong atau kedaluwarsa yang tidak wajar", async () => {
    const { muatSesiTersimpan } = await muat();
    for (const rusak of [
      { accessToken: "", refreshToken: "r", expiresAtMs: 1 },
      { accessToken: "a", refreshToken: "", expiresAtMs: 1 },
      { accessToken: "a", refreshToken: "r", expiresAtMs: "besok" },
      { accessToken: "a", refreshToken: "r" },
    ]) {
      toko.isi.set("facelink.session.v1", JSON.stringify(rusak));
      // Modul menyimpan cache; muat ulang agar isi baru dibaca.
      const modul = await muat();
      expect(await modul.muatSesiTersimpan()).toBeNull();
    }
  });

  it("tidak mencoba membaca saat penyimpanan kredensial tidak tersedia", async () => {
    toko.tersedia = false;
    const { muatSesiTersimpan } = await muat();
    expect(await muatSesiTersimpan()).toBeNull();
    expect(toko.getItemAsync).not.toHaveBeenCalled();
  });
});

describe("menghapus sesi", () => {
  it("mengosongkan penyimpanan dan cache", async () => {
    const { simpanSesi, hapusSesiTersimpan, muatSesiTersimpan } = await muat();
    await simpanSesi({ accessToken: "a", refreshToken: "r", expiresAtMs: 1_000 });
    await hapusSesiTersimpan();
    expect(toko.deleteItemAsync).toHaveBeenCalledWith("facelink.session.v1");
    // Cache tidak boleh menghidupkan kembali token yang sudah dihapus.
    expect(await muatSesiTersimpan()).toBeNull();
  });

  it("tidak melempar saat penyimpanan tidak tersedia", async () => {
    toko.tersedia = false;
    const { hapusSesiTersimpan } = await muat();
    await expect(hapusSesiTersimpan()).resolves.toBeUndefined();
  });

  it("does not refill cache from a read begun before deletion", async () => {
    const old = JSON.stringify({ accessToken: "old-a", refreshToken: "old-r", expiresAtMs: 42 });
    const pending = Promise.withResolvers<string | null>();
    const started = Promise.withResolvers<void>();
    toko.getItemAsync.mockImplementation(async () => {
      started.resolve();
      return pending.promise;
    });
    const store = await muat();
    const reading = store.muatSesiTersimpan();
    await started.promise;
    await store.hapusSesiTersimpan();
    pending.resolve(old);
    expect(await reading).toBeNull();
    expect(await store.muatSesiTersimpan()).toBeNull();
  });

  it("does not publish an unpersisted rotation in cache", async () => {
    const store = await muat();
    const old = { accessToken: "old-a", refreshToken: "old-r", expiresAtMs: 42 };
    await store.simpanSesi(old);
    toko.gagalTulis = true;
    await expect(store.simpanSesi({ ...old, refreshToken: "new-r" })).rejects.toThrow();
    expect(await store.muatSesiTersimpan()).toEqual(old);
  });
});

describe("masih berlaku", () => {
  it("menyisakan margin 60 detik sebelum kedaluwarsa", async () => {
    const { aksesMasihBerlaku } = await muat();
    const now = Date.now();
    expect(aksesMasihBerlaku({ accessToken: "a", refreshToken: "r", expiresAtMs: now + 61_000 })).toBe(
      true
    );
    // Kurang dari satu menit lagi: sudah dianggap perlu disegarkan.
    expect(aksesMasihBerlaku({ accessToken: "a", refreshToken: "r", expiresAtMs: now + 30_000 })).toBe(
      false
    );
  });
});

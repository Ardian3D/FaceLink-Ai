/**
 * Pengujian perilaku inti autentikasi (`authFlow.ts`).
 *
 * Setiap kasus menyatakan jawaban backend secara eksplisit. Tidak ada tiruan
 * yang "selalu berhasil": keberhasilan diuji dengan memeriksa **apa yang
 * dikirim ke backend** dan **apa yang tersisa di penyimpanan perangkat**,
 * sedangkan kegagalan diuji dengan memaksa setiap kelas kegagalan.
 */
import { describe, expect, it } from "vitest";
import { AuthFlow } from "@/features/auth/authFlow";
import type { AuthState } from "@/features/auth/authFlow";
import {
  ADMIN,
  DOSEN,
  MAHASISWA,
  apiTiruan,
  googleTiruan,
  jeda,
  penyimpananTiruan,
  sesiUntuk,
} from "./tiruan";

/** Merakit alur beserta tiruannya; mengembalikan state terakhir yang terlihat. */
function rakit(
  opsi: {
    api?: Parameters<typeof apiTiruan>[0];
    google?: Parameters<typeof googleTiruan>[0];
    penyimpanan?: Parameters<typeof penyimpananTiruan>[0];
  } = {}
) {
  const { api, catatan: catatanApi } = apiTiruan(opsi.api);
  const { google, catatan: catatanGoogle } = googleTiruan(opsi.google);
  const store = penyimpananTiruan(opsi.penyimpanan ?? null);
  const riwayat: AuthState[] = [];
  const alur = new AuthFlow(api, google, store, (s) => riwayat.push(s));
  return { alur, store, catatanApi, catatanGoogle, riwayat };
}

describe("login", () => {
  it.each([
    ["admin", ADMIN],
    ["dosen", DOSEN],
    ["mahasiswa", MAHASISWA],
  ])("memakai role %s persis dari backend", async (_nama, user) => {
    const { alur, store, catatanApi } = rakit({
      api: { tukar: async () => ({ ok: true, data: sesiUntuk(user) }) },
    });
    await alur.pulihkan();
    await alur.masuk();

    expect(alur.keadaan().status).toBe("signedIn");
    // Role tidak pernah dikarang aplikasi: nilainya sama dengan jawaban backend.
    expect(alur.keadaan().session?.user.role).toBe(user.role);
    expect(alur.keadaan().session?.user.id).toBe(user.id);
    // Token tersimpan di penyimpanan kredensial, bukan di state UI.
    expect(store.isi?.accessToken).toBe("access-1");
    expect(Object.keys(alur.keadaan().session ?? {})).toEqual(["user"]);
    // Nonce yang dikirim ke Google adalah nonce dari backend, bukan karangan.
    expect(catatanApi.tukar).toHaveLength(1);
    expect(catatanApi.tukar[0]).toMatchObject({
      idToken: "id-token",
      nonce: "nonce-dari-backend",
    });
  });

  it("meminta nonce baru untuk setiap percobaan", async () => {
    const { alur, catatanApi } = rakit();
    await alur.pulihkan();
    await alur.masuk();
    await alur.masuk();
    expect(catatanApi.nonce).toBe(2);
    expect(catatanApi.tukar.map((t) => t.nonce)).toEqual([
      "nonce-dari-backend",
      "nonce-dari-backend",
    ]);
  });

  it("mengabaikan ketukan ganda saat proses masuk berjalan", async () => {
    let lepas: (v: void) => void = () => {};
    const { alur, catatanApi } = rakit({
      api: {
        nonce: async () => {
          await new Promise<void>((r) => (lepas = r));
          return { ok: true, data: "nonce-dari-backend" };
        },
      },
    });
    await alur.pulihkan();
    const pertama = alur.masuk();
    await jeda();
    const kedua = alur.masuk();
    lepas();
    await Promise.all([pertama, kedua]);
    // Satu nonce, satu lembar akun, satu pertukaran.
    expect(catatanApi.nonce).toBe(1);
    expect(catatanApi.tukar).toHaveLength(1);
  });

  it("menghapus sesi lokal bila penyimpanan kredensial gagal", async () => {
    const { alur, store } = rakit();
    await alur.pulihkan();
    store.simpanGagal = true;
    await alur.masuk();
    // Sesi yang tidak dapat disimpan tidak boleh dipakai sama sekali.
    expect(alur.keadaan().status).toBe("failed");
    expect(alur.keadaan().error?.kind).toBe("storage");
    expect(alur.keadaan().session).toBeNull();
    expect(store.isi).toBeNull();
  });
});

describe("akun ditolak", () => {
  it("tidak memberi sesi dan tidak membuka dashboard", async () => {
    const { alur, store } = rakit({
      api: {
        tukar: async () => ({
          ok: false,
          error: { kind: "notGranted", email: "mahasiswa@example.test" },
        }),
      },
    });
    await alur.pulihkan();
    await alur.masuk();

    expect(alur.keadaan().status).toBe("unregistered");
    expect(alur.keadaan().session).toBeNull();
    expect(store.isi).toBeNull();
  });

  it("membedakan backend mati dari akun belum terdaftar", async () => {
    const { alur, store } = rakit({
      api: { tukar: async () => ({ ok: false, error: { kind: "network" } }) },
    });
    await alur.pulihkan();
    await alur.masuk();

    // Bukan "belum terdaftar": pesan yang salah akan menyuruh pengguna
    // menghubungi admin padahal yang salah adalah jaringannya.
    expect(alur.keadaan().status).toBe("failed");
    expect(alur.keadaan().error?.kind).toBe("network");
    expect(store.isi).toBeNull();
  });
});

describe("Google Sign-In", () => {
  it("kembali tenang tanpa pesan saat pengguna membatalkan", async () => {
    const { alur, catatanApi } = rakit({
      google: { masuk: async () => ({ ok: false, error: { kind: "cancelled" } }) },
    });
    await alur.pulihkan();
    await alur.masuk();

    expect(alur.keadaan().status).toBe("signedOut");
    expect(alur.keadaan().error).toBeNull();
    // Pembatalan tidak boleh membakar permintaan pertukaran token.
    expect(catatanApi.tukar).toHaveLength(0);
  });

  it("melaporkan galat TLS sebagai galat TLS, bukan galat akun", async () => {
    const { alur } = rakit({
      api: { nonce: async () => ({ ok: false, error: { kind: "tls" } }) },
    });
    await alur.pulihkan();
    await alur.masuk();
    expect(alur.keadaan().status).toBe("failed");
    expect(alur.keadaan().error?.kind).toBe("tls");
  });

  it("menolak platform tanpa Credential Manager sebelum menyentuh jaringan", async () => {
    const { alur, catatanApi } = rakit({
      google: { periksa: () => ({ kind: "unsupported" }) },
    });
    await alur.pulihkan();
    await alur.masuk();
    expect(catatanApi.nonce).toBe(0);
    expect(alur.keadaan().error?.kind).toBe("unsupported");
  });

  it("menolak konfigurasi client ID yang salah sebelum menyentuh jaringan", async () => {
    const { alur, catatanApi } = rakit({
      google: { periksa: () => ({ kind: "config", message: "client ID Android" }) },
    });
    await alur.pulihkan();
    await alur.masuk();
    expect(catatanApi.nonce).toBe(0);
    expect(alur.keadaan().error?.kind).toBe("config");
  });
});

describe("penyegaran token", () => {
  it("menyegarkan sekali dan menyimpan pasangan token baru", async () => {
    const { alur, store, catatanApi } = rakit({
      penyimpanan: {
        accessToken: "akses-lama",
        refreshToken: "refresh-lama",
        expiresAtMs: Date.now() - 1_000,
      },
      api: { saya: async () => ({ ok: true, data: MAHASISWA }) },
    });
    await alur.pulihkan();

    expect(catatanApi.segarkan).toBe(1);
    expect(store.isi?.accessToken).toBe("access-baru-1");
    expect(store.isi?.refreshToken).toBe("refresh-baru-1");
    // Identitas diverifikasi ulang lewat token yang baru.
    expect(catatanApi.saya).toEqual(["access-baru-1"]);
    expect(alur.keadaan().status).toBe("signedIn");
  });

  it("tidak menyegarkan saat token akses masih berlaku", async () => {
    const { alur, catatanApi } = rakit({
      penyimpanan: {
        accessToken: "akses-lama",
        refreshToken: "refresh-lama",
        expiresAtMs: Date.now() + 3_600_000,
      },
    });
    await alur.pulihkan();
    expect(catatanApi.segarkan).toBe(0);
    expect(catatanApi.saya).toEqual(["akses-lama"]);
  });

  it("menjalankan satu penyegaran untuk pemanggil paralel", async () => {
    // Penyegaran ditahan di tengah jalan, lalu tiga pemanggil meminta token.
    // Ketiganya harus berbagi satu penerbangan: memakai token penyegar yang
    // sama dua kali akan membuat backend mencabut seluruh sesi pengguna
    // (kontrak §5.1, deteksi penyalahgunaan rotasi).
    let lepasSegarkan: () => void = () => {};
    const tertahan = new Promise<void>((r) => (lepasSegarkan = r));
    const { alur, catatanApi, store } = rakit({
      penyimpanan: {
        accessToken: "akses-lama",
        refreshToken: "refresh-lama",
        expiresAtMs: Date.now() - 1_000,
      },
      api: {
        segarkan: async () => {
          await tertahan;
          return {
            ok: true,
            data: {
              accessToken: "akses-baru",
              refreshToken: "refresh-baru",
              expiresAtMs: Date.now() + 3_600_000,
            },
          };
        },
      },
    });

    const pulih = alur.pulihkan();
    await jeda(); // penyegaran sudah berjalan dan tertahan
    const bersamaan = [alur.tokenAkses(), alur.tokenAkses(), alur.tokenAkses()];
    await jeda();
    lepasSegarkan();
    const [, ...token] = await Promise.all([pulih, ...bersamaan]);

    // Tiga pemanggil tambahan, tetap satu rotasi.
    expect(catatanApi.segarkan).toBe(1);
    expect(token).toEqual(["akses-baru", "akses-baru", "akses-baru"]);
    expect(store.hitungSimpan).toBe(1);
    expect(store.isi?.refreshToken).toBe("refresh-baru");
  });

  it("membuang sesi saat backend menolak token penyegar", async () => {
    const { alur, store } = rakit({
      penyimpanan: {
        accessToken: "akses-lama",
        refreshToken: "refresh-lama",
        expiresAtMs: Date.now() - 1_000,
      },
      api: { segarkan: async () => ({ ok: false, error: { kind: "sessionEnded" } }) },
    });
    await alur.pulihkan();

    expect(alur.keadaan().status).toBe("signedOut");
    expect(alur.keadaan().error?.kind).toBe("sessionEnded");
    expect(store.isi).toBeNull();
  });

  it("mempertahankan token saat penyegaran gagal karena jaringan", async () => {
    const { alur, store } = rakit({
      penyimpanan: {
        accessToken: "akses-lama",
        refreshToken: "refresh-lama",
        expiresAtMs: Date.now() - 1_000,
      },
      api: { segarkan: async () => ({ ok: false, error: { kind: "network" } }) },
    });
    const tersimpan = { ...store.isi };
    await alur.pulihkan();

    expect(alur.keadaan().status).toBe("signedOut");
    expect(store.isi).toEqual(tersimpan);
    expect(store.hitungHapus).toBe(0);
  });
});

describe("pemulihan sesi", () => {
  it("memakai ulang token tersimpan setelah aplikasi dibuka ulang", async () => {
    const { alur, catatanApi } = rakit({
      penyimpanan: {
        accessToken: "akses-tersimpan",
        refreshToken: "refresh-tersimpan",
        expiresAtMs: Date.now() + 3_600_000,
      },
    });
    await alur.pulihkan();

    expect(alur.keadaan().status).toBe("signedIn");
    expect(catatanApi.segarkan).toBe(0);
    expect(catatanApi.saya).toEqual(["akses-tersimpan"]);
    // Role berasal dari `/me`, bukan dari token tersimpan.
    expect(alur.keadaan().session?.user.role).toBe("mahasiswa");
  });

  it("mengikuti role terbaru dari backend, bukan role lama di perangkat", async () => {
    const { alur } = rakit({
      penyimpanan: {
        accessToken: "akses-tersimpan",
        refreshToken: "refresh-tersimpan",
        expiresAtMs: Date.now() + 3_600_000,
      },
      api: { saya: async () => ({ ok: true, data: ADMIN }) },
    });
    await alur.pulihkan();
    expect(alur.keadaan().session?.user.role).toBe("admin");
  });

  it("mengarahkan akun yang kehilangan akses ke keadaan ditolak", async () => {
    const { alur, store } = rakit({
      penyimpanan: {
        accessToken: "akses-tersimpan",
        refreshToken: "refresh-tersimpan",
        expiresAtMs: Date.now() + 3_600_000,
      },
      api: { saya: async () => ({ ok: false, error: { kind: "notGranted" } }) },
    });
    await alur.pulihkan();

    expect(alur.keadaan().status).toBe("unregistered");
    expect(alur.keadaan().session).toBeNull();
    // Token akun yang ditolak tidak boleh tertinggal di perangkat.
    expect(store.isi).toBeNull();
  });

  it("menghapus sesi tersimpan bila /me menyatakan sesi dicabut", async () => {
    const { alur, store, catatanApi } = rakit({
      penyimpanan: {
        accessToken: "akses-tersimpan",
        refreshToken: "refresh-tersimpan",
        expiresAtMs: Date.now() + 3_600_000,
      },
      api: { saya: async () => ({ ok: false, error: { kind: "sessionEnded" } }) },
    });
    await alur.pulihkan();

    expect(catatanApi.segarkan).toBe(0);
    expect(catatanApi.saya).toEqual(["akses-tersimpan"]);
    expect(alur.keadaan().status).toBe("signedOut");
    expect(alur.keadaan().error?.kind).toBe("sessionEnded");
    expect(alur.keadaan().session).toBeNull();
    expect(store.isi).toBeNull();
    expect(store.hitungHapus).toBe(1);
    expect(await alur.tokenAkses()).toBeNull();
  });

  it("kembali ke layar masuk tanpa menghapus token saat backend mati", async () => {
    const { alur, store } = rakit({
      penyimpanan: {
        accessToken: "akses-tersimpan",
        refreshToken: "refresh-tersimpan",
        expiresAtMs: Date.now() + 3_600_000,
      },
      api: { saya: async () => ({ ok: false, error: { kind: "network" } }) },
    });
    const tersimpan = { ...store.isi };
    await alur.pulihkan();

    expect(alur.keadaan().status).toBe("signedOut");
    expect(alur.keadaan().error?.kind).toBe("network");
    expect(store.isi).toEqual(tersimpan);
    expect(store.hitungHapus).toBe(0);
  });

  it("memulihkan tanpa token tersimpan berarti belum masuk", async () => {
    const { alur, catatanApi } = rakit();
    await alur.pulihkan();
    expect(alur.keadaan().status).toBe("signedOut");
    expect(alur.keadaan().error).toBeNull();
    expect(catatanApi.saya).toHaveLength(0);
  });
});

describe("keluar", () => {
  it("menghapus token lokal, mencabut sesi, dan melupakan akun Google", async () => {
    const { alur, store, catatanApi, catatanGoogle } = rakit();
    await alur.pulihkan();
    await alur.masuk();
    expect(alur.keadaan().status).toBe("signedIn");

    await alur.keluar();

    expect(alur.keadaan().status).toBe("signedOut");
    expect(alur.keadaan().session).toBeNull();
    expect(store.isi).toBeNull();
    expect(catatanApi.keluar).toEqual(["refresh-1"]);
    expect(catatanGoogle.lupakan).toBe(1);
    // Setelah keluar, tidak ada token yang dapat dipakai lagi.
    expect(await alur.tokenAkses()).toBeNull();
  });

  it("tetap membersihkan perangkat bila pencabutan di server gagal", async () => {
    const { api, catatan } = apiTiruan();
    api.keluarSesi = async () => {
      throw new Error("jaringan mati");
    };
    const { google } = googleTiruan();
    const store = penyimpananTiruan();
    const alur = new AuthFlow(api, google, store, () => {});
    await alur.pulihkan();
    await alur.masuk();

    // Kegagalan pencabutan tidak boleh menghalangi penghapusan token lokal.
    await expect(alur.keluar()).resolves.toBeUndefined();
    expect(store.isi).toBeNull();
    expect(alur.keadaan().error).toEqual({
      kind: "signOutIncomplete", localCleared: true, remoteRevoked: false,
    });
    expect(catatan.keluar).toHaveLength(0);
  });

  it("does not resurrect a late refresh after logout and keeps remote failure visible", async () => {
    const { api } = apiTiruan();
    const { google } = googleTiruan();
    const store = penyimpananTiruan();
    const alur = new AuthFlow(api, google, store, () => {});
    await alur.pulihkan();
    await alur.masuk();
    store.masihBerlaku = () => false;
    const rotation = Promise.withResolvers<Awaited<ReturnType<typeof api.segarkanSesi>>>();
    api.segarkanSesi = () => rotation.promise;
    api.keluarSesi = async () => ({ ok: false, error: { kind: "network" } });
    const refreshing = alur.tokenAkses();
    const logout = alur.keluar();
    rotation.resolve({ ok: true, data: {
      accessToken: "rotated-access", refreshToken: "rotated-refresh",
      expiresAtMs: Date.now() + 3_600_000,
    } });
    await logout;
    expect(await refreshing).toBeNull();
    expect(await alur.tokenAkses()).toBeNull();
    expect(store.isi).toBeNull();
    expect(alur.keadaan()).toMatchObject({
      status: "signedOut", session: null,
      error: { kind: "signOutIncomplete", localCleared: true, remoteRevoked: false },
    });
  });

  it("drains an already-started credential write before final deletion", async () => {
    const { api } = apiTiruan();
    const { google } = googleTiruan();
    const store = penyimpananTiruan();
    const alur = new AuthFlow(api, google, store, () => {});
    await alur.pulihkan();
    await alur.masuk();
    store.masihBerlaku = () => false;
    const write = Promise.withResolvers<void>();
    const started = Promise.withResolvers<void>();
    store.simpan = async sesi => {
      started.resolve();
      await write.promise;
      store.isi = sesi;
    };
    const refreshing = alur.tokenAkses();
    await started.promise;
    const logout = alur.keluar();
    write.resolve();
    await logout;
    expect(await refreshing).toBeNull();
    expect(store.isi).toBeNull();
    expect(alur.keadaan()).toMatchObject({ status: "signedOut", session: null, error: null });
  });

  it("reports a failed SecureStore deletion without claiming the device is clean", async () => {
    const s = rakit();
    await s.alur.pulihkan();
    await s.alur.masuk();
    s.store.hapus = async () => { throw Error("Keystore locked"); };
    await s.alur.keluar();
    expect(await s.alur.tokenAkses()).toBeNull();
    expect(s.alur.keadaan()).toMatchObject({
      status: "signedOut", session: null,
      error: { kind: "signOutIncomplete", localCleared: false, remoteRevoked: true },
    });
  });

  it.each(["exchange", "write"] as const)("drains pending Google %s before completing logout", async boundary => {
    const { api } = apiTiruan();
    const { google } = googleTiruan();
    const store = penyimpananTiruan();
    const alur = new AuthFlow(api, google, store, () => {});
    await alur.pulihkan();
    const gate = Promise.withResolvers<void>();
    const started = Promise.withResolvers<void>();
    if (boundary === "exchange") {
      api.tukarIdToken = async () => {
        started.resolve();
        await gate.promise;
        return { ok: true, data: sesiUntuk(MAHASISWA) };
      };
    } else {
      store.simpan = async value => {
        started.resolve();
        await gate.promise;
        store.isi = value;
      };
    }
    api.keluarSesi = async () => ({ ok: false, error: { kind: "network" } });
    const login = alur.masuk();
    await started.promise;
    let completed = false;
    const logout = alur.keluar().then(() => { completed = true; });
    await jeda();
    expect(completed).toBe(false);
    gate.resolve();
    await Promise.all([login, logout]);
    expect(store.isi).toBeNull();
    expect(await alur.tokenAkses()).toBeNull();
    expect(alur.keadaan()).toMatchObject({
      status: "signedOut", error: { kind: "signOutIncomplete", remoteRevoked: false, localCleared: true },
    });
  });

  it("revokes the new refresh proof and deletes obsolete storage when rotation persistence fails", async () => {
    const { alur, store, catatanApi } = rakit();
    await alur.pulihkan();
    await alur.masuk();
    store.masihBerlaku = () => false;
    store.simpanGagal = true;
    expect(await alur.tokenAkses()).toBeNull();
    expect(store.isi).toBeNull();
    expect(catatanApi.keluar).toEqual(["refresh-baru-1"]);
    expect(alur.keadaan()).toMatchObject({ status: "signedOut", error: { kind: "storage" } });
  });

  it("does not erase unconfirmed remote revocation on a second local logout", async () => {
    const { api } = apiTiruan();
    api.keluarSesi = async () => ({ ok: false, error: { kind: "network" } });
    const { google } = googleTiruan();
    const store = penyimpananTiruan();
    const alur = new AuthFlow(api, google, store, () => {});
    await alur.pulihkan();
    await alur.masuk();
    await alur.keluar();
    await alur.keluar();
    expect(alur.keadaan()).toMatchObject({
      status: "signedOut", error: { kind: "signOutIncomplete", localCleared: true, remoteRevoked: false },
    });
  });

  it("reports remote revocation as unconfirmed when logout beats the initial restore read", async () => {
    const { api, catatan } = apiTiruan();
    const { google } = googleTiruan();
    const store = penyimpananTiruan();
    const pending = Promise.withResolvers<Awaited<ReturnType<typeof store.muat>>>();
    store.muat = () => pending.promise;
    const alur = new AuthFlow(api, google, store, () => {});
    const restoring = alur.pulihkan();
    await alur.keluar();
    pending.resolve({ accessToken: "old", refreshToken: "old-refresh", expiresAtMs: Date.now() + 3_600_000 });
    await restoring;
    expect(catatan.keluar).toEqual([]);
    expect(await alur.tokenAkses()).toBeNull();
    expect(alur.keadaan()).toMatchObject({
      status: "signedOut", error: { kind: "signOutIncomplete", localCleared: true, remoteRevoked: false },
    });
  });

  it("drains an aborted email acceptance write and reports failed compensating revocation", async () => {
    const { api } = apiTiruan({ email: async () => ({ ok: true, data: sesiUntuk(MAHASISWA) }) });
    const { google } = googleTiruan();
    const store = penyimpananTiruan();
    const alur = new AuthFlow(api, google, store, () => {});
    await alur.pulihkan();
    const gate = Promise.withResolvers<void>();
    const started = Promise.withResolvers<void>();
    store.simpan = async value => { started.resolve(); await gate.promise; store.isi = value; };
    api.keluarSesi = async () => ({ ok: false, error: { kind: "network" } });
    const login = alur.masukEmail("synthetic-challenge", "00000000", new AbortController().signal);
    await started.promise;
    const logout = alur.keluar();
    gate.resolve();
    await Promise.all([login, logout]);
    expect(store.isi).toBeNull();
    expect(alur.keadaan()).toMatchObject({
      status: "signedOut", error: { kind: "signOutIncomplete", localCleared: true, remoteRevoked: false },
    });
  });
});

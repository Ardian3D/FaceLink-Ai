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
    await alur.pulihkan();

    expect(alur.keadaan().status).toBe("signedOut");
    expect(store.isi?.refreshToken).toBe("refresh-lama");
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

  it("kembali ke layar masuk tanpa menghapus token saat backend mati", async () => {
    const { alur, store } = rakit({
      penyimpanan: {
        accessToken: "akses-tersimpan",
        refreshToken: "refresh-tersimpan",
        expiresAtMs: Date.now() + 3_600_000,
      },
      api: { saya: async () => ({ ok: false, error: { kind: "network" } }) },
    });
    await alur.pulihkan();

    expect(alur.keadaan().status).toBe("signedOut");
    expect(alur.keadaan().error?.kind).toBe("network");
    expect(store.isi?.refreshToken).toBe("refresh-tersimpan");
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
    expect(catatanApi.keluar).toEqual(["access-1"]);
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
    expect(catatan.keluar).toHaveLength(0);
  });
});

/**
 * Pemetaan galat HTTP → keadaan UI pada klien autentikasi.
 *
 * Ini bagian yang paling mudah salah dan paling mahal akibatnya: kalau `401`
 * pada `GET /me` dianggap sama dengan `401` pada `POST /auth/google`, pengguna
 * yang tokennya kedaluwarsa akan disuruh memeriksa konfigurasi build, dan
 * sebaliknya. Pengujian memakai `fetch` tiruan — bukan server — supaya setiap
 * kode dapat dipaksa muncul.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const BASE = "http://127.0.0.1:8080";

/** Memasang jawaban `fetch` untuk satu permintaan. */
function jawab(status: number, badan: unknown): void {
  const teks = typeof badan === "string" ? badan : JSON.stringify(badan);
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => new Response(teks, { status, headers: { "Content-Type": "application/json" } }))
  );
}

function gagalJaringan(pesan: string): void {
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => {
      throw new Error(pesan);
    })
  );
}

async function muatModul(alamat: string = BASE) {
  vi.resetModules();
  process.env.EXPO_PUBLIC_FACELINK_API_BASE_URL = alamat;
  (globalThis as unknown as { __DEV__: boolean }).__DEV__ = true;
  return await import("@/features/auth/authApi");
}

beforeEach(() => {
  process.env.EXPO_PUBLIC_FACELINK_API_BASE_URL = BASE;
});

afterEach(() => {
  vi.unstubAllGlobals();
  delete process.env.EXPO_PUBLIC_FACELINK_API_BASE_URL;
});

describe("mintaNonce", () => {
  it("mengembalikan nonce dari backend", async () => {
    const { mintaNonce } = await muatModul();
    jawab(200, { data: { nonce: "n".repeat(43), expires_in: 300 } });
    expect(await mintaNonce()).toEqual({ ok: true, data: "n".repeat(43) });
  });

  it("menolak respons yang bentuknya tidak sesuai kontrak", async () => {
    const { mintaNonce } = await muatModul();
    jawab(200, { data: { nonce: "" } });
    expect(await mintaNonce()).toEqual({
      ok: false,
      error: { kind: "unknown", message: "bentuk_respons_nonce" },
    });
  });

  it("melaporkan 503 sebagai layanan belum siap, bukan jaringan mati", async () => {
    const { mintaNonce } = await muatModul();
    jawab(503, { error: { code: "AUTH_UNAVAILABLE", message: "belum dikonfigurasi" } });
    expect(await mintaNonce()).toMatchObject({ ok: false, error: { kind: "unavailable" } });
  });

  it("membedakan galat sertifikat dari backend tidak terjangkau", async () => {
    const { mintaNonce } = await muatModul();
    gagalJaringan("java.security.cert.CertPathValidatorException: Trust anchor not found");
    expect(await mintaNonce()).toMatchObject({ ok: false, error: { kind: "tls" } });
  });

  it("melaporkan koneksi yang ditolak sebagai kegagalan jaringan", async () => {
    const { mintaNonce } = await muatModul();
    gagalJaringan("Network request failed");
    expect(await mintaNonce()).toMatchObject({ ok: false, error: { kind: "network" } });
  });

  it("melaporkan alamat backend yang tidak sah sebagai kesalahan konfigurasi", async () => {
    const { mintaNonce: _takDipakai } = await muatModul();
    // Alamat LAN ber-HTTP ditolak `config.ts` (D-035), bukan dicoba.
    const spy = vi.fn();
    vi.stubGlobal("fetch", spy);
    const hasil = await muatModul("http://192.168.1.9:8080").then((m) => m.mintaNonce());
    expect(hasil).toMatchObject({ ok: false, error: { kind: "config" } });
    expect(spy).not.toHaveBeenCalled();
  });
});

describe("tukarIdToken", () => {
  it("mengirim nonce dan ID token yang sama seperti yang diterima", async () => {
    const { tukarIdToken } = await muatModul();
    const panggilan: Array<{ url: string; badan: unknown }> = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (url: string, opsi: { body: string }) => {
        panggilan.push({ url, badan: JSON.parse(opsi.body) });
        return new Response(
          JSON.stringify({
            data: {
              access_token: "a",
              refresh_token: "r",
              expires_in: 3600,
              user: {
                id: "11111111-1111-4111-8111-111111111111",
                display_name: "Uji",
                email: "u@example.test",
                role: "dosen",
                status: "active",
                person_id: null,
                class_ids: [],
              },
            },
          }),
          { status: 200 }
        );
      })
    );
    const hasil = await tukarIdToken("id-token-google", "nonce-dari-backend");
    expect(hasil.ok).toBe(true);
    expect(panggilan[0]?.url).toBe(`${BASE}/api/v1/auth/google`);
    expect(panggilan[0]?.badan).toEqual({
      id_token: "id-token-google",
      nonce: "nonce-dari-backend",
    });
    if (hasil.ok) {
      // Role disalin apa adanya dari backend.
      expect(hasil.data.user.role).toBe("dosen");
      expect(hasil.data.expiresAtMs).toBeGreaterThan(Date.now());
    }
  });

  it("menolak role yang tidak dikenal kontrak", async () => {
    const { tukarIdToken } = await muatModul();
    jawab(200, {
      data: {
        access_token: "a",
        refresh_token: "r",
        expires_in: 3600,
        user: {
          id: "11111111-1111-4111-8111-111111111111",
          display_name: "Uji",
          role: "superadmin",
          status: "active",
          person_id: null,
          class_ids: [],
        },
      },
    });
    expect(await tukarIdToken("t", "n")).toEqual({
      ok: false,
      error: { kind: "unknown", message: "bentuk_respons_login" },
    });
  });

  it("menerjemahkan 403 IDENTITY_NOT_LINKED menjadi penolakan akun", async () => {
    const { tukarIdToken } = await muatModul();
    jawab(403, { error: { code: "IDENTITY_NOT_LINKED", message: "tidak dikenali" } });
    expect(await tukarIdToken("t", "n", "orang@example.test")).toEqual({
      ok: false,
      error: { kind: "notGranted", email: "orang@example.test" },
    });
  });

  it("membedakan 401 ID token dari 401 sesi berakhir", async () => {
    const { tukarIdToken, ambilSaya } = await muatModul();
    jawab(401, { error: { code: "IDENTITY_TOKEN_INVALID", message: "tidak sah" } });
    expect(await tukarIdToken("t", "n")).toMatchObject({
      ok: false,
      error: { kind: "identityRejected" },
    });

    jawab(401, { error: { code: "SESSION_REVOKED", message: "dicabut" } });
    expect(await ambilSaya("token")).toMatchObject({
      ok: false,
      error: { kind: "sessionEnded" },
    });
  });

  it("tidak menebak email saat backend tidak mengirimkannya", async () => {
    const { tukarIdToken } = await muatModul();
    jawab(403, { error: { code: "IDENTITY_NOT_LINKED", message: "generik" } });
    expect(await tukarIdToken("t", "n")).toEqual({ ok: false, error: { kind: "notGranted" } });
  });
});

describe("ambilSaya", () => {
  it("mengirim token sebagai Bearer dan mengembalikan identitas", async () => {
    const { ambilSaya } = await muatModul();
    let tajuk: Record<string, string> = {};
    vi.stubGlobal(
      "fetch",
      vi.fn(async (_url: string, opsi: { headers: Record<string, string> }) => {
        tajuk = opsi.headers;
        return new Response(
          JSON.stringify({
            data: {
              id: "11111111-1111-4111-8111-111111111111",
              display_name: "Uji",
              email: "u@example.test",
              role: "admin",
              status: "active",
              person_id: null,
              class_ids: ["c-1"],
            },
          }),
          { status: 200 }
        );
      })
    );
    const hasil = await ambilSaya("token-abc");
    expect(tajuk.Authorization).toBe("Bearer token-abc");
    // Token tidak boleh muncul di URL (akan tercatat di access log backend).
    expect(vi.mocked(fetch).mock.calls[0]?.[0]).toBe(`${BASE}/api/v1/me`);
    if (hasil.ok) expect(hasil.data.role).toBe("admin");
  });
});

describe("segarkanSesi", () => {
  it("mengembalikan pasangan token baru tanpa mengharapkan objek user", async () => {
    const { segarkanSesi } = await muatModul();
    jawab(200, { data: { access_token: "a2", refresh_token: "r2", expires_in: 3600 } });
    const hasil = await segarkanSesi("r1");
    expect(hasil.ok).toBe(true);
    if (hasil.ok) {
      expect(hasil.data.accessToken).toBe("a2");
      expect(hasil.data.refreshToken).toBe("r2");
    }
  });

  it("menolak respons tanpa pasangan token", async () => {
    const { segarkanSesi } = await muatModul();
    jawab(200, { data: { expires_in: 3600 } });
    expect(await segarkanSesi("r1")).toEqual({
      ok: false,
      error: { kind: "unknown", message: "bentuk_respons_refresh" },
    });
  });

  it("memetakan penyegar yang ditolak menjadi sesi berakhir", async () => {
    const { segarkanSesi } = await muatModul();
    jawab(401, { error: { code: "SESSION_REVOKED", message: "dicabut" } });
    expect(await segarkanSesi("r1")).toMatchObject({ ok: false, error: { kind: "sessionEnded" } });
  });
});

describe("keluarSesi", () => {
  it("mencabut sesi di server dengan token akses", async () => {
    const { keluarSesi } = await muatModul();
    const dipanggil: string[] = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (url: string) => {
        dipanggil.push(url);
        return new Response("", { status: 204 });
      })
    );
    await keluarSesi("token-abc");
    expect(dipanggil).toEqual([`${BASE}/api/v1/auth/logout`]);
  });

  it("tidak melempar walau jaringan mati", async () => {
    const { keluarSesi } = await muatModul();
    gagalJaringan("Network request failed");
    await expect(keluarSesi("token-abc")).resolves.toBeUndefined();
  });
});

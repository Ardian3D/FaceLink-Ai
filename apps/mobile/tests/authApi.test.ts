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

const BASE = "https://127.0.0.1:8080";

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

describe("transport autentikasi", () => {
  it.each(["http://127.0.0.1:8080", "http://10.0.2.2:8080"])(
    "tidak mengirim kredensial melalui %s, termasuk pada debug",
    async (alamat) => {
      const api = await muatModul(alamat);
      const fetch = vi.fn();
      vi.stubGlobal("fetch", fetch);
      for (const hasil of [
        await api.mintaNonce(),
        await api.tukarIdToken("id-token-uji", "nonce-uji"),
        await api.segarkanSesi("refresh-uji"),
        await api.ambilSaya("access-uji"),
        await api.mintaKodeEmail("orang@example.test"),
        await api.verifikasiKodeEmail("11111111-1111-4111-8111-111111111111", "01234567"),
      ]) {
        expect(hasil).toMatchObject({ ok: false, error: { kind: "config" } });
      }
      await api.keluarSesi("access-uji");
      expect(fetch).not.toHaveBeenCalled();
    }
  );
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
  it("reports a network failure as unconfirmed remote revocation", async () => {
    const { keluarSesi } = await muatModul();
    gagalJaringan("Network request failed");
    expect(await keluarSesi("synthetic-refresh")).toEqual({
      ok: false, error: { kind: "network" },
    });
  });

  it.each([200, 401, 500])("does not claim revocation after HTTP %i", async status => {
    const { keluarSesi } = await muatModul();
    jawab(status, { error: { code: "UNAUTHENTICATED" } });
    expect((await keluarSesi("synthetic-refresh")).ok).toBe(false);
  });
});

describe("Email OTP", () => {
  const challengeId = "11111111-1111-4111-8111-111111111111";

  it("mengirim email ternormalisasi dalam body, bukan URL", async () => {
    const api = await muatModul();
    jawab(202, { data: { challenge_id: challengeId, expires_in: 300, retry_after: 60, message: "generik" } });
    expect(await api.mintaKodeEmail("  Orang@Example.Test ")).toEqual({
      ok: true, data: { challengeId, expiresIn: 300, retryAfter: 60 },
    });
    expect(vi.mocked(fetch).mock.calls[0]?.[0]).toBe(`${BASE}/api/v1/auth/email/request`);
    expect(JSON.parse(vi.mocked(fetch).mock.calls[0]?.[1]?.body as string)).toEqual({ email: "orang@example.test" });
  });

  it.each([401, 400, 503, 403])("memetakan galat %s tanpa detail akun/delivery", async (status) => {
    const api = await muatModul();
    jawab(status, { error: { code: status === 403 ? "HTTPS_REQUIRED" : "OTP_INVALID", message: "detail-rahasia" } });
    const hasil = await api.verifikasiKodeEmail(challengeId, "01234567");
    expect(hasil).toMatchObject({ ok: false, error: { kind: ({ 401: "otpInvalid", 400: "validation", 503: "unavailable", 403: "config" } as Record<number, string>)[status] } });
    expect(JSON.stringify(hasil)).not.toContain("detail-rahasia");
  });

  it.each([
    ["90", "Thu, 22 Oct 2026 10:00:00 GMT", 90],
    ["Thu, 22 Oct 2026 10:02:00 GMT", "Thu, 22 Oct 2026 10:00:00 GMT", 120],
    ["rusak", "Thu, 22 Oct 2026 10:00:00 GMT", 60],
  ])("memakai Retry-After %s relatif ke jam server", async (retryAfter, serverDate, expected) => {
    const api = await muatModul();
    vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ error: { code: "RATE_LIMITED" } }), {
      status: 429, headers: { "Retry-After": String(retryAfter), Date: String(serverDate) },
    })));
    for (const hasil of [await api.mintaKodeEmail("orang@example.test"), await api.verifikasiKodeEmail(challengeId, "01234567")]) {
      expect(hasil).toEqual({ ok: false, error: { kind: "rateLimited", retryAfter: expected } });
    }
  });

  it("mempertahankan nol awal dan membaca sesi dengan parser yang sama", async () => {
    const api = await muatModul();
    jawab(200, { data: {
      access_token: "access", refresh_token: "refresh", expires_in: 3600,
      user: { id: challengeId, display_name: "Uji", role: "admin", status: "active", person_id: null, class_ids: [] },
    } });
    const hasil = await api.verifikasiKodeEmail(challengeId, "01234567");
    expect(hasil).toMatchObject({ ok: true, data: { accessToken: "access", user: { role: "admin" } } });
    expect(JSON.parse(vi.mocked(fetch).mock.calls[0]?.[1]?.body as string)).toEqual({ challenge_id: challengeId, code: "01234567" });
  });

  it("menolak bentuk challenge dan sesi yang tidak sah", async () => {
    const api = await muatModul();
    jawab(202, { data: { challenge_id: challengeId, expires_in: -1, retry_after: 60 } });
    expect(await api.mintaKodeEmail("orang@example.test")).toMatchObject({ ok: false, error: { kind: "unknown" } });
    jawab(200, { data: { access_token: "a", refresh_token: "r", expires_in: 3600, user: { role: "owner" } } });
    expect(await api.verifikasiKodeEmail(challengeId, "01234567")).toMatchObject({ ok: false, error: { kind: "unknown" } });
  });

  it("membatalkan fetch saat layar keluar dan tidak mengirim bila sudah batal", async () => {
    const api = await muatModul();
    const controller = new AbortController();
    const fetch = vi.fn((_url: string, init: RequestInit) => {
      const { promise, reject } = Promise.withResolvers<Response>();
      init.signal?.addEventListener("abort", () => reject(new Error("aborted")));
      return promise;
    });
    vi.stubGlobal("fetch", fetch);
    const request = api.mintaKodeEmail("orang@example.test", controller.signal);
    controller.abort();
    expect(await request).toEqual({ ok: false, error: { kind: "cancelled" } });
    expect(await api.verifikasiKodeEmail(challengeId, "01234567", controller.signal)).toEqual({ ok: false, error: { kind: "cancelled" } });
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  it("memetakan koneksi gagal sebagai galat jaringan tanpa mencoba ulang", async () => {
    const api = await muatModul();
    gagalJaringan("Network request failed");
    expect(await api.mintaKodeEmail("orang@example.test")).toEqual({ ok: false, error: { kind: "network" } });
    expect(fetch).toHaveBeenCalledTimes(1);
  });
});

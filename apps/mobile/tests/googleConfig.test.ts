/**
 * Aturan konfigurasi Google Sign-In.
 *
 * Client ID yang salah bentuk tidak dapat "diperbaiki dengan mencoba lagi": ia
 * menghasilkan ID token dengan `aud` yang tidak dikenal backend, dan pengguna
 * hanya akan melihat kegagalan masuk berulang. Karena itu bentuknya diperiksa
 * sebelum satu pun permintaan dikirim.
 */
import { describe, expect, it } from "vitest";
import { periksaWebClientId } from "@/features/auth/googleConfig";

describe("periksaWebClientId", () => {
  it("menerima client ID bertipe Web", () => {
    const nilai = "1234567890-abcdefg.apps.googleusercontent.com";
    expect(periksaWebClientId(nilai)).toEqual({ ok: true, webClientId: nilai });
  });

  it("memangkas spasi di tepi", () => {
    const hasil = periksaWebClientId("  123-abc.apps.googleusercontent.com\n");
    expect(hasil).toEqual({ ok: true, webClientId: "123-abc.apps.googleusercontent.com" });
  });

  it("menolak nilai kosong", () => {
    expect(periksaWebClientId("   ")).toEqual({ ok: false, alasan: "client ID Google kosong" });
  });

  it("menolak client ID Android/iOS", () => {
    // Bentuk ini tampak sah sekilas, tetapi `aud` pada ID token Android bukan
    // nilai ini; backend akan menolaknya sebagai IDENTITY_TOKEN_INVALID.
    expect(periksaWebClientId("1234567890-abcdefg.apps.googleusercontent.com.android")).toMatchObject({
      ok: false,
    });
    expect(periksaWebClientId("com.googleusercontent.apps.123-abc")).toMatchObject({
      ok: false,
    });
    expect(periksaWebClientId("1234567890-abcdefg")).toMatchObject({ ok: false });
  });
});

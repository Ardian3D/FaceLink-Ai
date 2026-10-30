import { describe, expect, it, vi } from "vitest";
import { EmailOtpFlow, pesanEmail } from "@/features/auth/emailOtpFlow";
import type { EmailChallenge, HasilEmail } from "@/features/auth/authApi";

const challenge: EmailChallenge = { challengeId: "11111111-1111-4111-8111-111111111111", expiresIn: 300, retryAfter: 60 };
const success = { ok: true as const, data: challenge };
const verified = { ok: true as const, data: undefined };

describe("EmailOtpFlow", () => {
  it("mengunci request bersamaan dan memakai deadline, bukan hitungan tick", async () => {
    let now = 1000;
    const pending = Promise.withResolvers<HasilEmail<EmailChallenge>>();
    const request = vi.fn<(email: string, signal: AbortSignal) => Promise<HasilEmail<EmailChallenge>>>(
      () => pending.promise
    );
    const flow = new EmailOtpFlow(request, vi.fn(async () => verified), () => now);
    const first = flow.minta("  Orang@Example.Test ");
    await flow.minta("lain@example.test");
    expect(request).toHaveBeenCalledTimes(1);
    expect(request.mock.calls[0]?.[0]).toBe("orang@example.test");
    pending.resolve(success);
    await first;
    expect(flow.keadaan()).toMatchObject({ email: "orang@example.test", retryAt: 61000, expiresAt: 301000, busy: null });
    now = 60999;
    await flow.minta("orang@example.test");
    expect(request).toHaveBeenCalledTimes(1);
    now = 61000; // Simulates a foreground jump without any interval ticks.
    await flow.minta("orang@example.test");
    expect(request).toHaveBeenCalledTimes(2);
  });

  it("mengabaikan hasil layar lama tanpa mengganti challenge baru", async () => {
    const old = Promise.withResolvers<HasilEmail<EmailChallenge>>();
    const request = vi.fn<(email: string, signal: AbortSignal) => Promise<HasilEmail<EmailChallenge>>>()
      .mockImplementationOnce(() => old.promise).mockResolvedValue(success);
    const flow = new EmailOtpFlow(request, async () => verified);
    const first = flow.minta("lama@example.test");
    const signal = request.mock.calls[0]?.[1];
    flow.cancel();
    await flow.minta("baru@example.test");
    old.resolve({ ok: true, data: { ...challenge, challengeId: "22222222-2222-4222-8222-222222222222" } });
    await first;
    expect(signal?.aborted).toBe(true);
    expect(flow.keadaan()).toMatchObject({ email: "baru@example.test", challenge, busy: null });
  });

  it("menonaktifkan challenge lama saat resend yang mungkin diterima server gagal jaringan", async () => {
    let now = 0;
    const request = vi.fn<(email: string, signal: AbortSignal) => Promise<HasilEmail<EmailChallenge>>>()
      .mockResolvedValueOnce(success).mockResolvedValue({ ok: false, error: { kind: "network" } });
    const verify = vi.fn(async () => verified);
    const flow = new EmailOtpFlow(request, verify, () => now);
    await flow.minta("orang@example.test");
    now = 60000;
    await flow.minta("orang@example.test");
    await flow.verifikasi("01234567");
    expect(verify).not.toHaveBeenCalled();
    expect(request).toHaveBeenCalledTimes(2);
    expect(flow.keadaan().expiresAt).toBe(0);
  });

  it("memakai cooldown 429 request dan verify dari server", async () => {
    let now = 0;
    const request = vi.fn<(email: string, signal: AbortSignal) => Promise<HasilEmail<EmailChallenge>>>()
      .mockResolvedValueOnce({ ok: false, error: { kind: "rateLimited", retryAfter: 90 } }).mockResolvedValue(success);
    const verify = vi.fn<(id: string, code: string, signal: AbortSignal) => Promise<HasilEmail<void>>>()
      .mockResolvedValue({ ok: false, error: { kind: "rateLimited", retryAfter: 120 } });
    const flow = new EmailOtpFlow(request, verify, () => now);
    await flow.minta("orang@example.test");
    now = 89999;
    await flow.minta("orang@example.test");
    expect(request).toHaveBeenCalledTimes(1);
    now = 90000;
    await flow.minta("orang@example.test");
    await flow.verifikasi("01234567");
    expect(flow.keadaan().verifyAt).toBe(210000);
    await flow.verifikasi("01234567");
    expect(verify).toHaveBeenCalledTimes(1);
    now = 210000;
    await flow.verifikasi("01234567");
    expect(verify).toHaveBeenCalledTimes(2);
  });

  it("mempertahankan kode lama bila resend definitif ditolak", async () => {
    let now = 0;
    const request = vi
      .fn<(email: string, signal: AbortSignal) => Promise<HasilEmail<EmailChallenge>>>()
      .mockResolvedValueOnce(success)
      .mockResolvedValue({ ok: false, error: { kind: "rateLimited", retryAfter: 30 } });
    const verify = vi.fn(async () => verified);
    const flow = new EmailOtpFlow(request, verify, () => now);
    await flow.minta("orang@example.test");
    now = 60000;
    await flow.minta("orang@example.test");
    // Server menolak sebelum membuat challenge baru; kode lama masih dapat dipakai.
    expect(flow.keadaan().expiresAt).toBe(300000);
    await flow.verifikasi("01234567");
    expect(verify).toHaveBeenCalledTimes(1);
  });

  it("memblokir submit verify ganda dan mengabaikan hasil setelah back", async () => {
    const pending = Promise.withResolvers<HasilEmail<void>>();
    const verify = vi.fn((_id: string, _code: string, _signal: AbortSignal) => pending.promise);
    const flow = new EmailOtpFlow(async () => success, verify);
    await flow.minta("orang@example.test");
    const first = flow.verifikasi("01234567");
    await flow.verifikasi("01234567");
    expect(verify).toHaveBeenCalledTimes(1);
    flow.back();
    expect(verify.mock.calls[0]?.[2].aborted).toBe(true);
    pending.resolve({ ok: false, error: { kind: "otpInvalid" } });
    await first;
    expect(flow.keadaan()).toMatchObject({ challenge: null, error: null, busy: null });
  });

  it("menolak tepat pada batas expiry tanpa mengirim kode", async () => {
    let now = 0;
    const verify = vi.fn(async () => verified);
    const flow = new EmailOtpFlow(async () => success, verify, () => now);
    await flow.minta("orang@example.test");
    await flow.verifikasi("123");
    expect(flow.keadaan().error?.kind).toBe("validation");
    now = 300000;
    await flow.verifikasi("01234567");
    expect(flow.keadaan().error?.kind).toBe("otpInvalid");
    expect(verify).not.toHaveBeenCalled();
    expect(JSON.stringify(flow.keadaan())).not.toContain("01234567");
  });

  it("mempertahankan challenge pada network verify failure dan tidak mengirim ulang", async () => {
    const request = vi.fn(async () => success);
    const flow = new EmailOtpFlow(request, async () => ({ ok: false, error: { kind: "network" } }));
    await flow.minta("orang@example.test");
    await flow.verifikasi("01234567");
    expect(flow.keadaan()).toMatchObject({ challenge, error: { kind: "network" }, busy: null });
    expect(request).toHaveBeenCalledTimes(1);
  });

  it("pesan generic tidak menyalin detail server atau email penolakan", () => {
    expect(pesanEmail({ kind: "notGranted", email: "privat@example.test" })).toBe(pesanEmail({ kind: "otpInvalid" }));
    expect(pesanEmail({ kind: "unknown", message: "privat@example.test kode" })).not.toContain("privat");
  });
});

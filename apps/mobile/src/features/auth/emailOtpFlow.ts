import type { EmailChallenge, EmailOtpError, HasilEmail } from "./authApi";

export type EmailOtpState = {
  email: string;
  challenge: EmailChallenge | null;
  expiresAt: number;
  retryAt: number;
  verifyAt: number;
  busy: "request" | "verify" | null;
  error: EmailOtpError | null;
};

const AWAL: EmailOtpState = {
  email: "", challenge: null, expiresAt: 0, retryAt: 0, verifyAt: 0, busy: null, error: null,
};

/** Deadlines, single-flight and cancellation are independent of React/native. */
export class EmailOtpFlow {
  private state: EmailOtpState = AWAL;
  private pending: AbortController | null = null;
  private listener?: (state: EmailOtpState) => void;

  constructor(
    private readonly request: (email: string, signal: AbortSignal) => Promise<HasilEmail<EmailChallenge>>,
    private readonly verify: (id: string, code: string, signal: AbortSignal) => Promise<HasilEmail<void>>,
    private readonly now: () => number = Date.now,
  ) {}

  keadaan(): EmailOtpState { return this.state; }

  subscribe(listener: (state: EmailOtpState) => void): () => void {
    this.listener = listener;
    listener(this.state);
    return () => { this.listener = undefined; };
  }

  private set(state: Partial<EmailOtpState>): void {
    this.state = { ...this.state, ...state };
    this.listener?.(this.state);
  }

  cancel(): void {
    this.pending?.abort();
    this.pending = null;
    this.state = AWAL;
    this.listener?.(this.state);
  }

  back(): void {
    const retryAt = this.state.retryAt;
    this.cancel();
    this.set({ retryAt });
  }

  async minta(email: string): Promise<void> {
    if (this.pending || this.now() < this.state.retryAt) return;
    const kendali = new AbortController();
    this.pending = kendali;
    const expirySebelumnya = this.state.challenge ? this.state.expiresAt : 0;
    this.set({
      busy: "request", error: null,
      // A timed-out request may already have superseded the old challenge.
      // Never offer verification of that old code or retry automatically.
      ...(this.state.challenge ? { expiresAt: 0 } : {}),
    });
    try {
      const hasil = await this.request(email.trim().toLowerCase(), kendali.signal);
      if (this.pending !== kendali || kendali.signal.aborted) return;
      if (hasil.ok) {
        const now = this.now();
        this.set({ email: email.trim().toLowerCase(), challenge: hasil.data,
          expiresAt: now + hasil.data.expiresIn * 1000,
          retryAt: now + hasil.data.retryAfter * 1000, verifyAt: 0 });
      } else {
        // Rejected sebelum server membuat challenge baru: kode lama tetap sah.
        // Jaringan/timeout tidak termasuk karena server mungkin sudah sempat
        // menggantikan challenge sebelum klien kehilangan jawabannya.
        const pastiDitolak =
          hasil.error.kind === "rateLimited" ||
          hasil.error.kind === "validation" ||
          hasil.error.kind === "config" ||
          hasil.error.kind === "tls";
        this.set({
          error: hasil.error,
          ...(pastiDitolak && expirySebelumnya > this.now() ? { expiresAt: expirySebelumnya } : {}),
          ...(hasil.error.kind === "rateLimited"
            ? { retryAt: this.now() + hasil.error.retryAfter * 1000 }
            : {}),
        });
      }
    } catch {
      if (this.pending === kendali && !kendali.signal.aborted) this.set({ error: { kind: "network" } });
    } finally {
      if (this.pending === kendali) {
        this.pending = null;
        this.set({ busy: null });
      }
    }
  }

  async verifikasi(code: string): Promise<void> {
    if (this.pending || !this.state.challenge || this.now() < this.state.verifyAt) return;
    if (this.now() >= this.state.expiresAt) {
      this.set({ error: { kind: "otpInvalid" } });
      return;
    }
    if (!/^\d{8}$/.test(code)) {
      this.set({ error: { kind: "validation" } });
      return;
    }
    const kendali = new AbortController();
    this.pending = kendali;
    this.set({ busy: "verify", error: null });
    try {
      const hasil = await this.verify(this.state.challenge.challengeId, code, kendali.signal);
      if (this.pending !== kendali || kendali.signal.aborted) return;
      if (!hasil.ok) {
        this.set({ error: hasil.error,
          ...(hasil.error.kind === "rateLimited" ? { verifyAt: this.now() + hasil.error.retryAfter * 1000 } : {}) });
      }
    } catch {
      if (this.pending === kendali && !kendali.signal.aborted) this.set({ error: { kind: "network" } });
    } finally {
      if (this.pending === kendali) {
        this.pending = null;
        this.set({ busy: null });
      }
    }
  }
}

/** Never display backend detail that could identify an account/delivery state. */
export function pesanEmail(error: EmailOtpError): string {
  switch (error.kind) {
    case "cancelled": return "Permintaan dibatalkan. Silakan coba lagi.";
    case "otpInvalid":
    case "notGranted":
    case "sessionEnded":
    case "identityRejected":
      return "Kode belum dapat digunakan. Periksa 8 digit kode terbaru atau minta kode baru setelah waktu tunggu. Jika masih bermasalah, hubungi admin.";
    case "validation": return "Periksa alamat email dan masukkan kode 8 digit dengan benar.";
    case "rateLimited": return "Terlalu banyak percobaan. Tunggu hingga hitung mundur selesai sebelum mencoba lagi.";
    case "network": return "Server tidak terjangkau. Periksa koneksi lalu coba lagi; kode tidak dikirim ulang otomatis.";
    case "tls": return "Sertifikat server tidak tepercaya. Hubungi admin untuk memperbaiki koneksi aman.";
    case "config": return "Autentikasi memerlukan alamat HTTPS dan sertifikat tepercaya. Hubungi admin.";
    case "storage": return "Sesi tidak dapat disimpan dengan aman. Buka kunci perangkat, lalu minta kode baru dan coba lagi.";
    case "unavailable": return "Layanan masuk belum siap. Coba lagi nanti atau hubungi admin.";
    default: return "Masuk belum berhasil. Coba lagi nanti atau hubungi admin.";
  }
}

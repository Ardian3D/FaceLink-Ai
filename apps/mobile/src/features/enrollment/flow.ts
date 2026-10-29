/**
 * Pure state machine for the staging enrollment screen.
 *
 * All backend values are kept as values, never inferred from local intent:
 * consent and enrollment are separate, and an ACTIVE snapshot is only trusted
 * while it is fresh. No persistence, logging, or image retention occurs here.
 */
import type {
  ConsentEnvelope,
  ConsentView,
  EnrollmentError,
  EnrollmentView,
  HasilEnrollment,
} from "./model";
import { statusTerpercaya } from "./model";
import { SyntheticCapture, type SyntheticCaptureState } from "./syntheticCapture";

export type EnrollmentPhase =
  | "loading"
  | "error"
  | "intro"
  | "consent"
  | "prepare"
  | "scan"
  | "uploading"
  | "pending"
  | "processing"
  | "active"
  | "rejected"
  | "expired"
  | "failed"
  | "withdrawn";

export type EnrollmentState = {
  phase: EnrollmentPhase;
  consent: ConsentView | null;
  enrollment: EnrollmentView | null;
  etag: string;
  busy: boolean;
  error: EnrollmentError | null;
  stale: boolean;
  lastUpdated: number | null;
  capture: SyntheticCaptureState | null;
  syntheticBackend: boolean;
};

export type EnrollmentApi = {
  readConsent: (token: string, signal?: AbortSignal) => Promise<HasilEnrollment<ConsentEnvelope>>;
  readStatus: (token: string, signal?: AbortSignal) => Promise<HasilEnrollment<EnrollmentView>>;
  grantConsent: (
    token: string,
    noticeVersion: string,
    ifMatch: string,
    replacesConsentId: string,
    signal?: AbortSignal
  ) => Promise<HasilEnrollment<ConsentView>>;
  withdrawConsent: (token: string, signal?: AbortSignal) => Promise<HasilEnrollment<EnrollmentView>>;
  submitJPEG: (
    token: string,
    consentId: string,
    jpeg: Uint8Array,
    signal?: AbortSignal
  ) => Promise<HasilEnrollment<EnrollmentView>>;
};

/** ACTIVE is trusted only for a short, fresh snapshot. */
const STATUS_TRUST_MS = 30_000;

const AWAL: EnrollmentState = {
  phase: "loading",
  consent: null,
  enrollment: null,
  etag: "",
  busy: false,
  error: null,
  stale: false,
  lastUpdated: null,
  capture: null,
  syntheticBackend: false,
};

function phaseDari(
  consent: ConsentView | null,
  enrollment: EnrollmentView | null
): EnrollmentPhase {
  if (enrollment !== null) {
    switch (enrollment.effectiveStatus) {
      case "PENDING_APPROVAL":
        return "pending";
      case "APPROVED_PROCESSING":
        return "processing";
      case "ACTIVE":
        return "active";
      case "REJECTED":
        return "rejected";
      case "EXPIRED":
        return "expired";
      case "FAILED_PROCESSING":
        return "failed";
      case "REVOKED":
        return "withdrawn";
      case "NOT_ENROLLED":
        break;
    }
  }
  if (consent?.state === "ACTIVE") return "prepare";
  return "intro";
}

export class EnrollmentFlow {
  private state: EnrollmentState = AWAL;
  private capture: SyntheticCapture | null = null;
  private abort: AbortController | null = null;
  private submitGuard = false;
  private loadSequence = 0;
  private refreshSequence = 0;
  private serverAnchor: { server: number; local: number } | null = null;
  private listener: ((state: EnrollmentState) => void) | undefined;

  constructor(
    private readonly api: EnrollmentApi,
    private token: () => Promise<string | null>,
    private readonly now: () => number = Date.now,
    private readonly onChange: (state: EnrollmentState) => void = () => {}
  ) {}

  setTokenProvider(token: () => Promise<string | null>): void {
    this.token = token;
  }

  subscribe(listener: (state: EnrollmentState) => void): () => void {
    this.listener = listener;
    listener(this.state);
    return () => {
      if (this.listener === listener) this.listener = undefined;
    };
  }

  keadaan(): EnrollmentState {
    return this.state;
  }

  trustedActive(): boolean {
    const last = this.state.lastUpdated;
    return (
      last !== null &&
      this.now() - last <= STATUS_TRUST_MS &&
      statusTerpercaya(
        this.state.enrollment,
        this.state.stale,
        this.state.consent
      )
    );
  }

  /**
   * Server-relative time for user-visible deadlines. The offset is captured
   * from the authenticated status response so a skewed device clock cannot
   * misstate photo retention; elapsed local time is clamped at zero.
   */
  serverNow(): number | null {
    const anchor = this.serverAnchor;
    if (anchor === null) return null;
    const local = this.now();
    if (!Number.isFinite(local)) return null;
    return anchor.server + Math.max(0, local - anchor.local);
  }

  private observeServerTime(value: string | null): void {
    if (value === null) return;
    const server = Date.parse(value);
    const local = this.now();
    if (!Number.isFinite(server) || !Number.isFinite(local)) return;
    this.serverAnchor = { server, local };
  }

  allowed(action: string): boolean {
    return this.state.enrollment?.allowedActions.includes(action) === true;
  }

  private set(sebagian: Partial<EnrollmentState>): void {
    this.state = { ...this.state, ...sebagian };
    this.onChange(this.state);
    this.listener?.(this.state);
  }

  private async tokenAkses(): Promise<string | null> {
    try {
      return await this.token();
    } catch {
      return null;
    }
  }

  private batalkan(): void {
    this.abort?.abort();
    this.abort = null;
  }

  /** Loads consent and status together; a failed half never discards the other. */
  async load(): Promise<void> {
    const urutan = ++this.loadSequence;
    this.batalkan();
    this.set({ busy: true, error: null, phase: "loading" });
    const kendali = new AbortController();
    this.abort = kendali;
    const token = await this.tokenAkses();
    if (kendali.signal.aborted || urutan !== this.loadSequence) return;
    if (token === null) {
      this.set({
        busy: false,
        stale: true,
        error: { kind: "sessionEnded" },
        phase: "error",
      });
      return;
    }
    const [consent, status] = await Promise.all([
      this.api.readConsent(token, kendali.signal),
      this.api.readStatus(token, kendali.signal),
    ]);
    if (kendali.signal.aborted) {
      if (urutan === this.loadSequence) {
        this.set({
          busy: false,
          phase: phaseDari(this.state.consent, this.state.enrollment),
        });
      }
      return;
    }
    if (urutan !== this.loadSequence) return;
    if (consent.ok) {
      this.set({ consent: consent.data.consent, etag: consent.data.etag });
    }
    if (status.ok) {
      this.observeServerTime(status.data.serverNow);
      this.set({
        enrollment: status.data,
        stale: false,
        lastUpdated: this.now(),
        syntheticBackend: true,
      });
    } else {
      this.set({ syntheticBackend: false });
    }
    const gagal = !consent.ok ? consent.error : !status.ok ? status.error : null;
    if (gagal !== null) {
      this.set({
        busy: false,
        error: gagal,
        stale: true,
        phase: this.state.consent === null ? "error" : phaseDari(this.state.consent, this.state.enrollment),
      });
      return;
    }
    this.set({
      busy: false,
      error: null,
      phase: phaseDari(this.state.consent, this.state.enrollment),
    });
  }

  masukConsent(): void {
    if (this.state.consent === null) return;
    this.set({ phase: "consent", error: null });
  }

  siapkan(): void {
    if (this.state.consent?.state !== "ACTIVE") {
      this.masukConsent();
      return;
    }
    if (this.hasLiveEnrollment()) return;
    this.set({ phase: "prepare", error: null });
  }

  batalConsent(): void {
    this.set({ phase: phaseDari(this.state.consent, this.state.enrollment), error: null });
  }

  async beriConsent(): Promise<void> {
    if (!this.state.syntheticBackend) {
      this.set({
        error: { kind: "unavailable", code: "SYNTHETIC_BACKEND_UNVERIFIED" },
      });
      return;
    }
    const consent = this.state.consent;
    if (consent === null || this.state.etag === "") {
      this.set({ error: { kind: "precondition" } });
      return;
    }
    this.batalkan();
    this.set({ busy: true, error: null });
    const kendali = new AbortController();
    this.abort = kendali;
    const token = await this.tokenAkses();
    if (kendali.signal.aborted) return;
    if (token === null) {
      this.set({ busy: false, error: { kind: "sessionEnded" } });
      return;
    }
    const hasil = await this.api.grantConsent(
      token,
      consent.notice.version,
      this.state.etag,
      consent.state === "ACTIVE" ? (consent.recordId ?? "") : "",
      kendali.signal
    );
    if (kendali.signal.aborted) return;
    if (!hasil.ok) {
      this.set({ busy: false, error: hasil.error });
      const perluMuatUlang =
        hasil.error.kind === "precondition" ||
        hasil.error.kind === "conflict" ||
        hasil.error.kind === "network" ||
        hasil.error.kind === "tls" ||
        hasil.error.kind === "unavailable";
      if (perluMuatUlang) {
        await this.load();
        if (this.state.consent?.state !== "ACTIVE") this.set({ error: hasil.error });
      }
      return;
    }
    this.set({ consent: hasil.data, busy: false, error: null });
    await this.load();
  }

  mulaiScan(): void {
    if (!this.state.syntheticBackend) {
      this.set({ error: { kind: "unavailable", code: "SYNTHETIC_BACKEND_UNVERIFIED" } });
      return;
    }
    if (this.state.consent?.state !== "ACTIVE") {
      this.set({ error: { kind: "conflict", code: "CONSENT_REQUIRED" } });
      return;
    }
    if (this.state.busy || (this.state.enrollment !== null && this.hasLiveEnrollment())) return;
    this.batalkan();
    this.capture = new SyntheticCapture();
    this.capture.start(this.now());
    this.set({
      phase: "scan",
      capture: this.capture.snapshot(this.now()),
      error: null,
      stale: false,
    });
  }

  /** UI calls this on an interval; completion automatically submits once. */
  tickScan(): void {
    if (this.state.phase !== "scan" || this.capture === null) return;
    const snapshot = this.capture.snapshot(this.now());
    this.set({ capture: snapshot });
    if (snapshot.phase === "ready" && !this.submitGuard) {
      void this.kirim();
    }
  }

  batalScan(): void {
    this.capture?.cancel();
    const snapshot = this.capture?.snapshot(this.now()) ?? null;
    this.set({
      capture: snapshot,
      phase: phaseDari(this.state.consent, this.state.enrollment),
      error: null,
    });
  }

  private async kirim(): Promise<void> {
    if (!this.state.syntheticBackend || this.submitGuard || this.capture === null) return;
    const consent = this.state.consent;
    if (consent?.recordId === null || consent?.recordId === undefined) {
      this.set({ error: { kind: "conflict", code: "CONSENT_REQUIRED" } });
      return;
    }
    const jpeg = this.capture.consume(this.now());
    if (jpeg === null) return;
    this.submitGuard = true;
    this.batalkan();
    this.set({ phase: "uploading", busy: true, error: null });
    const kendali = new AbortController();
    this.abort = kendali;
    const token = await this.tokenAkses();
    if (kendali.signal.aborted) {
      jpeg.fill(0);
      this.submitGuard = false;
      return;
    }
    if (token === null) {
      jpeg.fill(0);
      this.submitGuard = false;
      this.set({
        busy: false,
        error: { kind: "sessionEnded" },
        phase: phaseDari(this.state.consent, this.state.enrollment),
      });
      return;
    }
    try {
      const hasil = await this.api.submitJPEG(
        token,
        consent.recordId,
        jpeg,
        kendali.signal
      );
      if (kendali.signal.aborted) return;
      if (!hasil.ok) {
        this.set({
          busy: false,
          error: hasil.error,
          phase: phaseDari(this.state.consent, this.state.enrollment),
        });
        if (
          hasil.error.kind === "network" ||
          hasil.error.kind === "tls" ||
          hasil.error.kind === "unavailable"
        ) {
          // The server may have committed before the response was lost.
          await this.refresh(false);
        } else if (hasil.error.code === "ENROLLMENT_EXISTS") {
          await this.load();
        }
        return;
      }
      this.observeServerTime(hasil.data.serverNow);
      this.set({
        enrollment: hasil.data,
        busy: false,
        error: null,
        stale: false,
        lastUpdated: this.now(),
        capture: null,
        phase: phaseDari(this.state.consent, hasil.data),
      });
    } finally {
      jpeg.fill(0);
      this.submitGuard = false;
      if (this.abort === kendali) this.abort = null;
    }
  }

  async tarik(): Promise<void> {
    this.batalkan();
    this.capture?.cancel();
    this.capture = null;
    this.set({ phase: "loading", busy: true, error: null, capture: null, stale: true });
    const kendali = new AbortController();
    this.abort = kendali;
    const token = await this.tokenAkses();
    if (kendali.signal.aborted) return;
    if (token === null) {
      this.set({ busy: false, error: { kind: "sessionEnded" }, phase: "error" });
      return;
    }
    const hasil = await this.api.withdrawConsent(token, kendali.signal);
    if (kendali.signal.aborted) return;
    if (!hasil.ok) {
      this.set({
        busy: false,
        error: hasil.error,
        phase: phaseDari(this.state.consent, this.state.enrollment),
      });
      if (
        hasil.error.kind === "network" ||
        hasil.error.kind === "tls" ||
        hasil.error.kind === "unavailable"
      ) {
        await this.load();
      }
      return;
    }
    this.set({
      consent: this.state.consent
        ? { ...this.state.consent, state: "WITHDRAWN" }
        : null,
      enrollment: hasil.data,
      busy: false,
      error: null,
      stale: false,
      lastUpdated: this.now(),
      phase: "withdrawn",
    });
    await this.load();
  }

  /** Re-reads status. Background failures mark stale instead of asserting ACTIVE. */
  async refresh(manual = true): Promise<void> {
    if (this.state.phase === "uploading") return;
    if (this.state.consent === null) {
      await this.load();
      return;
    }
    const urutan = ++this.refreshSequence;
    this.batalkan();
    if (manual) this.set({ busy: true, error: null });
    const kendali = new AbortController();
    this.abort = kendali;
    const token = await this.tokenAkses();
    if (kendali.signal.aborted || urutan !== this.refreshSequence) return;
    if (token === null) {
      this.set({
        busy: false,
        stale: true,
        ...(manual ? { error: { kind: "sessionEnded" } } : {}),
      });
      return;
    }
    const hasil = await this.api.readStatus(token, kendali.signal);
    if (kendali.signal.aborted) {
      if (urutan === this.refreshSequence) this.set({ busy: false });
      return;
    }
    if (urutan !== this.refreshSequence) return;
    if (!hasil.ok) {
      this.set({
        busy: false,
        stale: true,
        ...(manual ? { error: hasil.error } : {}),
      });
      return;
    }
    this.observeServerTime(hasil.data.serverNow);
    this.set({
      enrollment: hasil.data,
      busy: false,
      error: null,
      stale: false,
      lastUpdated: this.now(),
      syntheticBackend: true,
      phase: phaseDari(this.state.consent, hasil.data),
    });
  }

  /** App background/blur: stop local work and stop trusting cached status. */
  suspend(): void {
    this.batalkan();
    this.set({ busy: false });
    if (this.state.phase === "scan" || this.state.phase === "uploading") {
      this.capture?.cancel();
      this.capture = null;
      this.set({
        capture: null,
        phase: phaseDari(this.state.consent, this.state.enrollment),
        busy: false,
      });
    }
    if (this.state.enrollment !== null) this.set({ stale: true });
  }

  dispose(): void {
    this.batalkan();
    this.capture?.cancel();
    this.capture = null;
  }

  private hasLiveEnrollment(): boolean {
    const status = this.state.enrollment?.effectiveStatus;
    return (
      status === "PENDING_APPROVAL" ||
      status === "APPROVED_PROCESSING" ||
      status === "ACTIVE" ||
      status === "FAILED_PROCESSING"
    );
  }
}

export function detikTersisa(iso: string | null, now: number): number | null {
  if (iso === null) return null;
  const waktu = Date.parse(iso);
  if (!Number.isFinite(waktu)) return null;
  return Math.max(0, Math.ceil((waktu - now) / 1000));
}

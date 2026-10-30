/**
 * Deterministic guided-capture harness for the staging UI.
 *
 * It does not open a camera, request permission, read a sensor, or receive a
 * human image. The host drives it with an injected clock; when the simulated
 * guidance completes it exposes the synthetic JPEG fixture exactly once. UI can
 * therefore test alignment/hold/cancel/background/retry without a device.
 */
import { syntheticJPEG } from "./fixture";

export type SyntheticCapturePhase =
  | "idle"
  | "aligning"
  | "holding"
  | "capturing"
  | "ready"
  | "cancelled";

export type SyntheticCaptureState = {
  phase: SyntheticCapturePhase;
  progress: number;
  instruction: string;
  accessibility: string;
  completed: boolean;
};

const ALIGN_UNTIL = 0.55;
const HOLD_UNTIL = 0.82;
const CAPTURE_UNTIL = 1;
const DEFAULT_DURATION_MS = 2_400;

export class SyntheticCapture {
  private startedAt: number | null = null;
  private durationMs = DEFAULT_DURATION_MS;
  private state: SyntheticCaptureState = {
    phase: "idle",
    progress: 0,
    instruction: "Tekan mulai untuk memulai simulasi pemindaian.",
    accessibility: "Pemindaian wajah sintetis belum dimulai.",
    completed: false,
  };
  private consumed = false;

  start(now: number): void {
    this.startedAt = now;
    this.consumed = false;
    this.update(now);
  }

  /** Cancel is idempotent; a late tick cannot revive a cancelled capture. */
  cancel(): void {
    if (this.state.phase === "ready" || this.state.phase === "cancelled") return;
    this.startedAt = null;
    this.consumed = true;
    this.state = {
      phase: "cancelled",
      progress: 0,
      instruction: "Pemindaian dibatalkan. Tidak ada foto yang dikirim.",
      accessibility: "Pemindaian wajah sintetis dibatalkan.",
      completed: false,
    };
  }

  snapshot(now: number): SyntheticCaptureState {
    this.update(now);
    return { ...this.state };
  }

  /** Returns the fixture once after readiness; later calls never resend it. */
  consume(now: number): Uint8Array | null {
    this.update(now);
    if (this.state.phase !== "ready" || this.consumed) return null;
    this.consumed = true;
    return syntheticJPEG();
  }

  private update(now: number): void {
    if (this.startedAt === null) return;
    if (this.state.phase === "ready" || this.state.phase === "cancelled") return;
    const progress = Math.min(
      1,
      Math.max(0, (now - this.startedAt) / this.durationMs)
    );
    if (progress < ALIGN_UNTIL) {
      this.state = {
        phase: "aligning",
        progress,
        instruction: "Posisikan wajah di dalam oval dan hadapkan ke kamera.",
        accessibility: `Posisikan wajah di dalam oval. Kemajuan ${Math.round(progress * 100)} persen.`,
        completed: false,
      };
      return;
    }
    if (progress < HOLD_UNTIL) {
      this.state = {
        phase: "holding",
        progress,
        instruction: "Tahan posisi. Jangan menoleh atau mendekat.",
        accessibility: `Tahan posisi. Kemajuan ${Math.round(progress * 100)} persen.`,
        completed: false,
      };
      return;
    }
    if (progress < CAPTURE_UNTIL) {
      this.state = {
        phase: "capturing",
        progress,
        instruction: "Mengambil satu bingkai sintetis…",
        accessibility: "Mengambil satu bingkai sintetis secara otomatis.",
        completed: false,
      };
      return;
    }
    this.state = {
      phase: "ready",
      progress: 1,
      instruction: "Bingkai sintetis siap dikirim.",
      accessibility: "Pemindaian sintetis selesai. Bingkai siap dikirim.",
      completed: true,
    };
  }
}

/* eslint-disable react-hooks/refs -- VisionCamera needs an imperative Camera ref; detector callbacks are invoked outside render. */
import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  AppState,
  Pressable,
  StyleSheet,
  Text,
  View,
  useWindowDimensions,
  type LayoutChangeEvent,
} from "react-native";
import {
  Camera,
  useCameraDevice,
  useCameraPermission,
  type CameraDevice,
  type CameraOutput,
  type CameraRef,
} from "react-native-vision-camera";
import { createFaceDetectorOutput } from "react-native-vision-camera-face-detector";
import {
  FaceLinkCaptureLive,
  isCaptureLiveAvailable,
} from "facelink-native-capture-host";
import { MannequinTrace } from "./mannequinTrace";

const acceptanceTrace = new MannequinTrace(
  __DEV__ && process.env.EXPO_PUBLIC_FACELINK_CAPTURE_TRACE === "1",
);
const acceptanceProbes =
  acceptanceTrace.enabled && process.env.EXPO_PUBLIC_FACELINK_CAPTURE_PROBES === "1";
const acceptanceCounts = { captures: 0, consumes: 0, releases: 0 };

async function traceRelease(frameId?: string): Promise<void> {
  if (!acceptanceTrace.enabled) return;
  try {
    const status = await FaceLinkCaptureLive.getStatus();
    const released = !status.busy && status.heldFrames === 0 && status.ownedBytes === 0;
    if (released && frameId !== undefined) acceptanceCounts.releases++;
    acceptanceTrace.record(released ? (frameId === undefined ? "DRAINED" : "RELEASED") : "ERROR", {
      generation: status.generation, frameId,
      reason: released ? status.phase : "E_CAPTURE_NOT_DRAINED",
      nativePhase: status.phase, nativeError: status.lastError,
      ownedBytes: status.ownedBytes, heldFrames: status.heldFrames, busy: status.busy,
      peakOwnedBytes: status.peakOwnedBytes, ...acceptanceCounts,
    });
  } catch (error) {
    acceptanceTrace.record("ERROR", { frameId, reason: safeCode(error) });
  }
}

/**
 * FL-ENR02B staging-only live scanner.
 *
 * Adapted from the CAM00 proof (`research/fl-cam00-proof`): the same front
 * Camera, face-detector output, preview-coordinate mapping, dwell thresholds,
 * generation/frame ownership and single-consume ordering. It is not part of a
 * main build. The parent must render it only in the disposable staging tree.
 *
 * Privacy:
 * - The camera is mounted only after the operator explicitly presses start and
 *   only after the permission decision resolves.
 * - A local camera preview is visible to the operator. No captured frame is
 *   rendered, saved, or uploaded from JavaScript. The bridge returns metadata
 *   plus a private consume step; `onConsumed` receives counts and identity only.
 */

export type NativeScannerProps = {
  onCancel(): void;
  onConsumed(meta: {
    width: number;
    height: number;
    jpegBytes: number;
    frameId: string;
    generation: number;
  }): void;
  onError(code: string): void;
};

type ReadyMetadata = {
  generation: number;
  frameId: string;
  width: number;
  height: number;
  jpegBytes: number;
  exif: boolean;
  mirrored?: boolean;
  status: "READY";
  handle: string;
  encodeMs: number;
  selectedAgeMs: number;
  peakOwnedBytes: number;
  expiresAtMs: number;
};

type ConsumeReceipt = {
  generation: number;
  frameId: string;
  width: number;
  height: number;
  jpegBytes: number;
  exif: boolean;
  status: "CONSUMED";
  consumed: true;
};

type CaptureLivePort = {
  start(generation: number): Promise<void>;
  decide(
    generation: number,
    frameId: string,
    capture: boolean,
    sensorCorners: number[] | null,
  ): Promise<ReadyMetadata | null>;
  consume(generation: number, handle: string): Promise<ConsumeReceipt>;
  cancel(): Promise<void>;
  addCaptureFrameListener(listener: (event: unknown) => void): {
    remove(): void;
  };
};

type FrameFace = {
  bounds: Rect;
  frameWidth: number;
  frameHeight: number;
  yawAngle: number;
  pitchAngle: number;
  rollAngle: number;
};

type FrameMetadata = {
  generation: number;
  frameId: string;
  acquiredAtMs: number;
  emittedAtMs: number;
  faces: FrameFace[];
};

type Point = { x: number; y: number };
type Size = { width: number; height: number };
type Rect = Point & Size;
type Ellipse = { cx: number; cy: number; rx: number; ry: number };
type Candidate = { box: Rect; yaw: number; pitch: number; roll: number; view: Size };

type Reason =
  | "OFF"
  | "WAIT_PREVIEW"
  | "NO_FACE"
  | "MULTIPLE_FACES"
  | "INVALID_GEOMETRY"
  | "OUTSIDE_OVAL"
  | "TOO_FAR"
  | "YAW"
  | "PITCH"
  | "ROLL"
  | "HOLD_STILL"
  | "MOVEMENT"
  | "CALLBACK_GAP"
  | "CLOCK_RESET"
  | "LAYOUT_CHANGED"
  | "READY"
  | "CAPTURING"
  | "CONSUMED"
  | "OPERATOR_STOP"
  | "BACKGROUND"
  | "PERMISSION_DENIED"
  | "PORTRAIT_REQUIRED"
  | "CAMERA_ERROR"
  | "DETECTOR_ERROR"
  | "CAPTURE_UNAVAILABLE"
  | "CAPTURE_ERROR"
  | "PREVIEW_STOPPED";

type Phase = "IDLE" | "STARTING" | "GUIDANCE" | "CAPTURING" | "CONSUMED" | "ERROR";

type UiState = {
  phase: Phase;
  reason: Reason;
  errorCode: string | null;
  stableMs: number;
  samples: number;
  faceCount: number;
  box: Rect | null;
};

const DWELL_MS = 1500;
const MAX_GAP_MS = 400;
const MIN_SAMPLES = 6;
const UI_INTERVAL_MS = 200;
const MAX_PIXELS = 2073600;
const MAX_JPEG_BYTES = 4194304;
const MAX_OWNED_BYTES = 16777216;

const now = () => performance.now();

// The native live module is process-wide, so generations must keep increasing
// across screen mounts. A per-component counter could restart at 1 and be
// rejected as stale by a previous scanner session.
let generationCounter = Date.now();
function nextGeneration(): number {
  generationCounter += 1;
  return generationCounter;
}

const messages: Record<Reason, string> = {
  OFF: "Kamera mati. Pemindai hanya menyala setelah Anda menekan tombol mulai.",
  WAIT_PREVIEW: "Menyiapkan izin dan pratinjau kamera…",
  NO_FACE: "Tidak ada wajah terdeteksi. Posisikan wajah di dalam oval.",
  MULTIPLE_FACES: "Lebih dari satu wajah. Pastikan hanya Anda di dalam bingkai.",
  INVALID_GEOMETRY: "Koordinat wajah belum valid. Tahan sebentar.",
  OUTSIDE_OVAL: "Seluruh wajah harus berada di dalam oval.",
  TOO_FAR: "Dekatkan wajah ke kamera.",
  YAW: "Hadapkan wajah lurus ke kamera.",
  PITCH: "Ratakan posisi dagu.",
  ROLL: "Sejajarkan kepala.",
  HOLD_STILL: "Tahan posisi — mengukur kestabilan.",
  MOVEMENT: "Posisi bergerak. Tahan lagi tanpa bergerak.",
  CALLBACK_GAP: "Aliran deteksi terputus; hitungan kestabilan dimulai ulang.",
  CLOCK_RESET: "Waktu perangkat tidak kontinu; hitungan dimulai ulang.",
  LAYOUT_CHANGED: "Ukuran pratinjau berubah; hitungan dimulai ulang.",
  READY: "Posisi stabil. Mengambil satu bingkai…",
  CAPTURING: "Memproses satu bingkai. Jangan menutup aplikasi.",
  CONSUMED: "Bingkai berhasil diproses. Tidak ada gambar yang ditampilkan atau disimpan.",
  OPERATOR_STOP: "Pemindaian dibatalkan. Mulai lagi secara eksplisit.",
  BACKGROUND: "Pemindaian berhenti karena aplikasi ke latar belakang. Mulai lagi secara eksplisit.",
  PERMISSION_DENIED: "Izin kamera tidak diberikan. Beri izin melalui pengaturan lalu coba lagi.",
  PORTRAIT_REQUIRED: "Gunakan orientasi portrait. Kamera tetap mati.",
  CAMERA_ERROR: "Kamera tidak tersedia atau gagal. Kamera sudah dimatikan.",
  DETECTOR_ERROR: "Detektor wajah gagal. Kamera sudah dimatikan.",
  CAPTURE_UNAVAILABLE: "Modul pemindai native tidak tersedia pada build ini. Kamera tidak dinyalakan.",
  CAPTURE_ERROR: "Pemrosesan bingkai gagal. Tidak ada bingkai sukses yang diasumsikan.",
  PREVIEW_STOPPED: "Pratinjau berhenti. Mulai lagi secara eksplisit.",
};

function codeMessage(code: string | null): string {
  switch (code) {
    case "E_PERMISSION_DENIED":
      return messages.PERMISSION_DENIED;
    case "E_CAPTURE_UNAVAILABLE":
    case "E_CAPTURE_LIVE_UNAVAILABLE":
    case "E_CAPTURE_NATIVE":
      return messages.CAPTURE_UNAVAILABLE;
    case "E_CAMERA_ERROR":
      return messages.CAMERA_ERROR;
    case "E_DETECTOR_ERROR":
      return messages.DETECTOR_ERROR;
    case "E_PORTRAIT_REQUIRED":
      return messages.PORTRAIT_REQUIRED;
    default:
      return code ? messages.CAPTURE_ERROR : messages.OFF;
  }
}

function validSize(size: Size): boolean {
  return (
    Number.isFinite(size.width) &&
    Number.isFinite(size.height) &&
    size.width > 0 &&
    size.height > 0
  );
}

function validRect(rect: Rect): boolean {
  return (
    validSize(rect) &&
    Number.isFinite(rect.x) &&
    Number.isFinite(rect.y) &&
    Number.isFinite(rect.x + rect.width) &&
    Number.isFinite(rect.y + rect.height)
  );
}

function previewOval(view: Size): Ellipse | null {
  if (!validSize(view)) return null;
  const rx = Math.min(view.width * 0.41, view.height * 0.32);
  const ry = Math.min(rx * 1.36, view.height * 0.43);
  if (!(rx > 0 && ry > 0)) return null;
  return { cx: view.width / 2, cy: view.height / 2, rx, ry };
}

function boxInsideEllipse(box: Rect, oval: Ellipse): boolean {
  return [box.x, box.x + box.width].every((x) =>
    [box.y, box.y + box.height].every(
      (y) =>
        ((x - oval.cx) / oval.rx) ** 2 + ((y - oval.cy) / oval.ry) ** 2 <= 1,
    ),
  );
}

function mapBoundsToView(
  box: Rect,
  convert: (point: Point) => Point,
): Rect | null {
  if (!validRect(box)) return null;
  try {
    const corners = [
      convert({ x: box.x, y: box.y }),
      convert({ x: box.x + box.width, y: box.y }),
      convert({ x: box.x, y: box.y + box.height }),
      convert({ x: box.x + box.width, y: box.y + box.height }),
    ];
    if (!corners.every((p) => Number.isFinite(p.x) && Number.isFinite(p.y))) {
      return null;
    }
    const x = Math.min(...corners.map((p) => p.x));
    const y = Math.min(...corners.map((p) => p.y));
    const result = {
      x,
      y,
      width: Math.max(...corners.map((p) => p.x)) - x,
      height: Math.max(...corners.map((p) => p.y)) - y,
    };
    return validRect(result) ? result : null;
  } catch {
    return null;
  }
}

type Guidance = { valid: boolean; reason: Reason };

function assessObservation(
  count: number,
  candidate: Candidate | null,
  frame: Size,
): Guidance {
  if (!Number.isInteger(count) || count < 0) {
    return { valid: false, reason: "INVALID_GEOMETRY" };
  }
  if (count === 0) return { valid: false, reason: "NO_FACE" };
  if (count > 1) return { valid: false, reason: "MULTIPLE_FACES" };
  if (
    !candidate ||
    !validSize(frame) ||
    !validSize(candidate.view) ||
    !validRect(candidate.box) ||
    ![candidate.yaw, candidate.pitch, candidate.roll].every(Number.isFinite)
  ) {
    return { valid: false, reason: "INVALID_GEOMETRY" };
  }
  const oval = previewOval(candidate.view);
  if (!oval || !boxInsideEllipse(candidate.box, oval)) {
    return { valid: false, reason: "OUTSIDE_OVAL" };
  }
  if (
    candidate.box.width < oval.rx * 0.7 ||
    candidate.box.height < oval.ry * 0.65
  ) {
    return { valid: false, reason: "TOO_FAR" };
  }
  if (Math.abs(candidate.yaw) > 12) return { valid: false, reason: "YAW" };
  if (Math.abs(candidate.pitch) > 12) return { valid: false, reason: "PITCH" };
  if (Math.abs(candidate.roll) > 10) return { valid: false, reason: "ROLL" };
  return { valid: true, reason: "HOLD_STILL" };
}

function samePosition(anchor: Candidate, current: Candidate): boolean {
  const a = anchor.box;
  const b = current.box;
  return (
    anchor.view.width === current.view.width &&
    anchor.view.height === current.view.height &&
    Math.abs(a.x + a.width / 2 - (b.x + b.width / 2)) <=
      current.view.width * 0.025 &&
    Math.abs(a.y + a.height / 2 - (b.y + b.height / 2)) <=
      current.view.height * 0.025 &&
    Math.abs(a.width - b.width) <= a.width * 0.06 &&
    Math.abs(a.height - b.height) <= a.height * 0.06 &&
    Math.abs(anchor.yaw - current.yaw) <= 3 &&
    Math.abs(anchor.pitch - current.pitch) <= 3 &&
    Math.abs(anchor.roll - current.roll) <= 3
  );
}

const validGeneration = (value: number) =>
  Number.isSafeInteger(value) && value > 0;

const validFrameId = (value: string) =>
  /^[1-9][0-9]{0,18}$/.test(value) &&
  (value.length < 19 || value <= "9223372036854775807");

function validImageMetadata(value: {
  generation: number;
  frameId: string;
  width: number;
  height: number;
  jpegBytes: number;
  exif: boolean;
}): boolean {
  return (
    validGeneration(value.generation) &&
    typeof value.frameId === "string" &&
    validFrameId(value.frameId) &&
    Number.isInteger(value.width) &&
    Number.isInteger(value.height) &&
    value.width > 0 &&
    value.height > 0 &&
    value.width * value.height <= MAX_PIXELS &&
    Number.isInteger(value.jpegBytes) &&
    value.jpegBytes > 0 &&
    value.jpegBytes <= MAX_JPEG_BYTES &&
    value.exif === false
  );
}

function validReady(
  value: ReadyMetadata,
  generation: number,
  frameId: string,
): boolean {
  return (
    validImageMetadata(value) &&
    value.generation === generation &&
    value.frameId === frameId &&
    value.status === "READY" &&
    typeof value.handle === "string" &&
    value.handle.length >= 16 &&
    value.handle.length <= 128 &&
    [value.encodeMs, value.selectedAgeMs, value.peakOwnedBytes, value.expiresAtMs].every(
      (n) => Number.isFinite(n) && n >= 0,
    ) &&
    value.selectedAgeMs <= 400 &&
    value.peakOwnedBytes >= value.jpegBytes &&
    value.peakOwnedBytes <= MAX_OWNED_BYTES &&
    value.expiresAtMs > 0
  );
}

function validReceipt(
  value: ConsumeReceipt,
  generation: number,
  frameId: string,
): boolean {
  return (
    validImageMetadata(value) &&
    value.status === "CONSUMED" &&
    value.consumed === true &&
    value.generation === generation &&
    value.frameId === frameId
  );
}

class ScannerError extends Error {
  readonly code: string;
  constructor(code: string) {
    super(code);
    this.name = "ScannerError";
    this.code = code;
  }
}

function safeCode(error: unknown): string {
  try {
    if (error && typeof error === "object" && "code" in error) {
      const code = error.code;
      if (typeof code === "string" && /^E_[A-Z0-9_]{1,60}$/.test(code)) {
        return code;
      }
    }
    return "E_CAPTURE_NATIVE";
  } catch {
    return "E_CAPTURE_NATIVE";
  }
}

function wrap(error: unknown): ScannerError {
  return error instanceof ScannerError ? error : new ScannerError(safeCode(error));
}

/**
 * Narrow client around the native live capture module:
 * - epochs invalidate late start/decide/consume completions;
 * - one generation at a time and one claimed capture frame per generation;
 * - native frame IDs are unique, increasing decimals, so one high-water mark
 *   rejects replays without retaining face/frame history;
 * - the opaque consume handle never leaves this class.
 */
class LiveCaptureClient {
  #port: CaptureLivePort;
  #epoch = 0;
  #generation: number | null = null;
  #highestGeneration = 0;
  #highestFrameId = "0";
  #pending: { generation: number; frameId: string; handle: string } | null = null;
  #captureClaimed = false;
  #disposed = false;

  constructor(port: CaptureLivePort) {
    this.#port = port;
  }

  claimFrame(frameId: string): boolean {
    if (!validFrameId(frameId)) return false;
    if (
      frameId.length < this.#highestFrameId.length ||
      (frameId.length === this.#highestFrameId.length &&
        frameId <= this.#highestFrameId)
    ) {
      return false;
    }
    this.#highestFrameId = frameId;
    return true;
  }

  async start(generation: number): Promise<void> {
    if (this.#disposed) throw new ScannerError("E_CAPTURE_DISPOSED");
    if (
      !validGeneration(generation) ||
      generation <= this.#highestGeneration
    ) {
      throw new ScannerError("E_CAPTURE_STALE_GENERATION");
    }
    if (this.#generation !== null) throw new ScannerError("E_CAPTURE_BUSY");
    const epoch = ++this.#epoch;
    this.#highestGeneration = generation;
    this.#generation = generation;
    this.#captureClaimed = false;
    try {
      await this.#port.start(generation);
      if (this.#disposed || this.#epoch !== epoch) {
        throw new ScannerError("E_CAPTURE_CANCELLED");
      }
    } catch (error) {
      if (this.#epoch === epoch) this.#generation = null;
      throw wrap(error);
    }
  }

  async decide(
    generation: number,
    frameId: string,
    capture: boolean,
    sensorCorners: number[] | null,
  ): Promise<ReadyMetadata | null> {
    if (this.#disposed) throw new ScannerError("E_CAPTURE_DISPOSED");
    if (!validGeneration(generation) || !validFrameId(frameId)) {
      throw new ScannerError("E_CAPTURE_IDENTITY");
    }
    if (capture && this.#generation !== generation) {
      throw new ScannerError("E_CAPTURE_STALE_GENERATION");
    }
    if (capture && this.#captureClaimed) {
      throw new ScannerError("E_CAPTURE_BUSY");
    }
    if (capture) this.#captureClaimed = true;
    const epoch = this.#epoch;
    try {
      const result = await this.#port.decide(
        generation,
        frameId,
        capture,
        capture ? sensorCorners : null,
      );
      if (!capture) return null;
      if (
        this.#disposed ||
        epoch !== this.#epoch ||
        generation !== this.#generation
      ) {
        throw new ScannerError("E_CAPTURE_CANCELLED");
      }
      if (!result || !validReady(result, generation, frameId)) {
        await this.#port.cancel();
        throw new ScannerError("E_CAPTURE_RESULT");
      }
      this.#pending = {
        generation,
        frameId,
        handle: result.handle,
      };
      if (acceptanceTrace.enabled) {
        const status = await FaceLinkCaptureLive.getStatus().catch(() => null);
        const geometry = status?.previewGeometry;
        acceptanceTrace.record("JPEG_READY", {
          generation: result.generation, frameId: result.frameId, jpegBytes: result.jpegBytes,
          mirrored: result.mirrored, peakOwnedBytes: result.peakOwnedBytes,
          nativePhase: status?.phase, nativeError: status?.lastError,
          ownedBytes: status?.ownedBytes, heldFrames: status?.heldFrames, busy: status?.busy,
          rotationDegrees: geometry?.rotationDegrees,
          buffer: geometry ? { width: geometry.inputWidth, height: geometry.inputHeight } : undefined,
          nativeCropNormalized: geometry && geometry.inputWidth > 0 && geometry.inputHeight > 0
            ? {
                x: geometry.cropLeft / geometry.inputWidth, y: geometry.cropTop / geometry.inputHeight,
                width: (geometry.cropRight - geometry.cropLeft) / geometry.inputWidth,
                height: (geometry.cropBottom - geometry.cropTop) / geometry.inputHeight,
              } : null,
        });
      }
      // The component only receives an explicit metadata whitelist; the handle
      // and host geometry extras stay private.
      return {
        generation: result.generation,
        frameId: result.frameId,
        width: result.width,
        height: result.height,
        jpegBytes: result.jpegBytes,
        exif: false,
        status: "READY",
        handle: "",
        encodeMs: result.encodeMs,
        selectedAgeMs: result.selectedAgeMs,
        peakOwnedBytes: result.peakOwnedBytes,
        expiresAtMs: result.expiresAtMs,
      };
    } catch (error) {
      if (capture && acceptanceTrace.enabled) {
        const status = await FaceLinkCaptureLive.getStatus().catch(() => null);
        const geometry = status?.previewGeometry;
        acceptanceTrace.record("ERROR", {
          generation, frameId, reason: safeCode(error),
          nativePhase: status?.phase, nativeError: status?.lastError,
          ownedBytes: status?.ownedBytes, heldFrames: status?.heldFrames, busy: status?.busy,
          rotationDegrees: geometry?.rotationDegrees,
          buffer: geometry ? { width: geometry.inputWidth, height: geometry.inputHeight } : undefined,
          nativePreviewNormalized: geometry && geometry.inputWidth > 0 && geometry.inputHeight > 0
            ? {
                x: geometry.minX / geometry.inputWidth, y: geometry.minY / geometry.inputHeight,
                width: (geometry.maxX - geometry.minX) / geometry.inputWidth,
                height: (geometry.maxY - geometry.minY) / geometry.inputHeight,
              } : null,
          nativeCropNormalized: geometry && geometry.inputWidth > 0 && geometry.inputHeight > 0
            ? {
                x: geometry.cropLeft / geometry.inputWidth, y: geometry.cropTop / geometry.inputHeight,
                width: (geometry.cropRight - geometry.cropLeft) / geometry.inputWidth,
                height: (geometry.cropBottom - geometry.cropTop) / geometry.inputHeight,
              } : null,
        });
      }
      throw wrap(error);
    }
  }

  async consume(
    generation: number,
    frameId: string,
  ): Promise<ConsumeReceipt> {
    const pending = this.#pending;
    if (
      !pending ||
      pending.generation !== generation ||
      pending.frameId !== frameId
    ) {
      throw new ScannerError("E_CAPTURE_ALREADY_CLAIMED");
    }
    // Single flight: a second consume can never reuse this handle.
    this.#pending = null;
    const handle = pending.handle;
    try {
      if (acceptanceProbes) {
        try {
          await this.#port.consume(generation - 1, handle);
          throw new ScannerError("E_ACCEPTANCE_STALE_ACCEPTED");
        } catch (error) {
          const code = safeCode(error);
          if (code !== "E_CAPTURE_GENERATION") throw error;
          acceptanceTrace.record("STALE_REJECTED", { generation, frameId, reason: code });
        }
      }
      const receipt = await this.#port.consume(generation, handle);
      if (!validReceipt(receipt, generation, frameId)) {
        throw new ScannerError("E_CAPTURE_RESULT");
      }
      if (acceptanceTrace.enabled) acceptanceCounts.consumes++;
      acceptanceTrace.record("CONSUMED", { generation, frameId, ...acceptanceCounts });
      if (acceptanceProbes) {
        try {
          await this.#port.consume(generation, handle);
          throw new ScannerError("E_ACCEPTANCE_DUPLICATE_ACCEPTED");
        } catch (error) {
          const code = safeCode(error);
          if (code !== "E_CAPTURE_HANDLE") throw error;
          acceptanceTrace.record("DUPLICATE_REJECTED", { generation, frameId, reason: code });
        }
      }
      return {
        generation: receipt.generation,
        frameId: receipt.frameId,
        width: receipt.width,
        height: receipt.height,
        jpegBytes: receipt.jpegBytes,
        exif: false,
        status: "CONSUMED",
        consumed: true,
      };
    } catch (error) {
      throw wrap(error);
    }
  }

  async cancel(): Promise<void> {
    this.#epoch++;
    this.#generation = null;
    this.#pending = null;
    try {
      await this.#port.cancel();
      acceptanceTrace.record("CANCELLED", {});
      await traceRelease();
    } catch (error) {
      throw wrap(error);
    }
  }
}

function resolveLivePort(): CaptureLivePort | null {
  try {
    if (!isCaptureLiveAvailable()) return null;
    // Validate the host export once; every later call uses this boundary type.
    const candidate: unknown = FaceLinkCaptureLive;
    if (!candidate || typeof candidate !== "object") return null;
    const port = candidate as Partial<CaptureLivePort>;
    return typeof port.start === "function" &&
      typeof port.decide === "function" &&
      typeof port.consume === "function" &&
      typeof port.cancel === "function" &&
      typeof port.addCaptureFrameListener === "function"
      ? (candidate as CaptureLivePort)
      : null;
  } catch {
    return null;
  }
}

type LiveCameraProps = {
  generation: number;
  device: CameraDevice;
  cameraRef: React.RefObject<CameraRef | null>;
  onReady(generation: number): void;
  onStopped(generation: number): void;
  onFailure(generation: number, reason: "CAMERA_ERROR" | "DETECTOR_ERROR"): void;
};

function LiveCamera(props: LiveCameraProps) {
  const callbacks = useRef(props);
  callbacks.current = props;
  const generation = useRef(props.generation).current;
  // Plugin 2.1.0's hook rest-options object invalidates its memo every render.
  // One detector output per explicitly started generation; its callbacks keep
  // that generation even after a late async completion.
  const output = useMemo(
    () =>
      createFaceDetectorOutput({
        cameraFacing: "front",
        mirrorMode: "on",
        autoMode: true,
        outputResolution: "preview",
        performanceMode: "fast",
        runLandmarks: false,
        runContours: false,
        runClassifications: false,
        minFaceSize: 0.15,
        trackingEnabled: false,
        // Only tagged CaptureFrame events drive guidance.
        onFacesDetected: () => {},
        onError: () =>
          callbacks.current.onFailure(generation, "DETECTOR_ERROR"),
      }),
    [generation],
  );
  const outputs = useMemo<CameraOutput[]>(() => [output], [output]);
  return (
    <Camera
      ref={props.cameraRef}
      style={StyleSheet.absoluteFill}
      device={props.device}
      isActive
      outputs={outputs}
      mirrorMode="on"
      orientationSource="interface"
      resizeMode="cover"
      implementationMode="compatible"
      onPreviewStarted={() => callbacks.current.onReady(generation)}
      onPreviewStopped={() => callbacks.current.onStopped(generation)}
      onError={() => callbacks.current.onFailure(generation, "CAMERA_ERROR")}
    />
  );
}

const initialUi: UiState = {
  phase: "IDLE",
  reason: "OFF",
  errorCode: null,
  stableMs: 0,
  samples: 0,
  faceCount: 0,
  box: null,
};

export function NativeScanner(props: NativeScannerProps) {
  const { onCancel, onConsumed, onError } = props;
  const callbacks = useRef({ onCancel, onConsumed, onError });
  callbacks.current = { onCancel, onConsumed, onError };
  const dimensions = useWindowDimensions();
  const portrait = dimensions.height >= dimensions.width;
  const device = useCameraDevice("front");
  const { hasPermission, canRequestPermission, requestPermission } =
    useCameraPermission();

  const [port] = useState<CaptureLivePort | null>(() => resolveLivePort());
  const [client] = useState<LiveCaptureClient | null>(() =>
    port ? new LiveCaptureClient(port) : null,
  );
  const [ui, setUi] = useState<UiState>(initialUi);
  const [session, setSession] = useState<number | null>(null);
  const [viewSize, setViewSize] = useState<Size>({ width: 0, height: 0 });

  const mounted = useRef(true);
  const foreground = useRef(
    AppState.currentState === "active" ||
      AppState.currentState === "unknown",
  );
  const generation = useRef(0);
  const activeGeneration = useRef<number | null>(null);
  const pendingStart = useRef(false);
  const consumedToken = useRef<number | null>(null);
  const camera = useRef<CameraRef | null>(null);
  const sizeRef = useRef(viewSize);
  const uiAt = useRef(-Infinity);
  const phaseRef = useRef<Phase>("IDLE");
  const reasonRef = useRef<Reason>("OFF");
  const errorRef = useRef<string | null>(null);
  const faceCountRef = useRef(0);
  const boxRef = useRef<Rect | null>(null);
  const dwell = useRef({
    anchor: null as Candidate | null,
    stableSince: null as number | null,
    stableMs: 0,
    samples: 0,
    lastFrameAt: null as number | null,
    lastAt: 0,
  });
  const traceStage = useRef({ generation: 0, stage: 0 });
  const captureStatus = useRef<
    "ARMED" | "CLAIMED" | "CONSUMING" | "CONSUMED"
  >("ARMED");

  const flush = useCallback((force = false) => {
    if (!mounted.current) return;
    const at = now();
    if (!force && at - uiAt.current < UI_INTERVAL_MS) return;
    uiAt.current = at;
    setUi({
      phase: phaseRef.current,
      reason: reasonRef.current,
      errorCode: errorRef.current,
      stableMs: dwell.current.stableMs,
      samples: dwell.current.samples,
      faceCount: faceCountRef.current,
      box: boxRef.current,
    });
  }, []);

  const resetDwell = useCallback(
    (reason: Reason, at: number) => {
      if (reasonRef.current !== reason || dwell.current.samples > 0) {
        acceptanceTrace.record("RESET", { generation: generation.current, reason });
      }
      dwell.current.anchor = null;
      dwell.current.stableSince = null;
      dwell.current.stableMs = 0;
      dwell.current.samples = 0;
      dwell.current.lastAt = at;
      reasonRef.current = reason;
    },
    [],
  );

  const current = useCallback(
    (token: number) =>
      mounted.current &&
      foreground.current &&
      generation.current === token &&
      activeGeneration.current === token,
    [],
  );

  const fail = useCallback(
    (token: number, reason: Reason, code: string) => {
      if (token !== generation.current) return;
      if (activeGeneration.current !== token && !pendingStart.current) return;
      acceptanceTrace.record("ERROR", { generation: token, reason: code });
      activeGeneration.current = null;
      pendingStart.current = false;
      generation.current = nextGeneration();
      consumedToken.current = null;
      captureStatus.current = "ARMED";
      phaseRef.current = "ERROR";
      reasonRef.current = reason;
      errorRef.current = code;
      faceCountRef.current = 0;
      boxRef.current = null;
      dwell.current = {
        anchor: null,
        stableSince: null,
        stableMs: 0,
        samples: 0,
        lastFrameAt: null,
        lastAt: 0,
      };
      if (mounted.current) {
        setSession(null);
        flush(true);
        callbacks.current.onError(code);
      }
      void client?.cancel().catch(() => {});
    },
    [client, flush],
  );

  const stop = useCallback(
    (reason: Reason, code: string | null) => {
      activeGeneration.current = null;
      pendingStart.current = false;
      generation.current = nextGeneration();
      consumedToken.current = null;
      captureStatus.current = "ARMED";
      phaseRef.current = code ? "ERROR" : "IDLE";
      reasonRef.current = reason;
      errorRef.current = code;
      faceCountRef.current = 0;
      boxRef.current = null;
      dwell.current = {
        anchor: null,
        stableSince: null,
        stableMs: 0,
        samples: 0,
        lastFrameAt: null,
        lastAt: 0,
      };
      if (mounted.current) {
        setSession(null);
        flush(true);
      }
      void client?.cancel().catch(() => {});
    },
    [client, flush],
  );

  const handleCancel = useCallback(() => {
    stop("OPERATOR_STOP", null);
    if (mounted.current) callbacks.current.onCancel();
  }, [stop]);

  const start = useCallback(async () => {
    if (
      !mounted.current ||
      !foreground.current ||
      pendingStart.current ||
      activeGeneration.current !== null
    ) {
      return;
    }
    if (!portrait) {
      phaseRef.current = "ERROR";
      reasonRef.current = "PORTRAIT_REQUIRED";
      errorRef.current = "E_PORTRAIT_REQUIRED";
      flush(true);
      callbacks.current.onError("E_PORTRAIT_REQUIRED");
      return;
    }
    if (!client || !port) {
      phaseRef.current = "ERROR";
      reasonRef.current = "CAPTURE_UNAVAILABLE";
      errorRef.current = "E_CAPTURE_UNAVAILABLE";
      flush(true);
      callbacks.current.onError("E_CAPTURE_UNAVAILABLE");
      return;
    }
    if (!device) {
      phaseRef.current = "ERROR";
      reasonRef.current = "CAMERA_ERROR";
      errorRef.current = "E_CAMERA_ERROR";
      flush(true);
      callbacks.current.onError("E_CAMERA_ERROR");
      return;
    }

    const token = nextGeneration();
    acceptanceTrace.record("START", { generation: token });
    generation.current = token;
    consumedToken.current = null;
    pendingStart.current = true;
    captureStatus.current = "ARMED";
    phaseRef.current = "STARTING";
    reasonRef.current = "WAIT_PREVIEW";
    errorRef.current = null;
    faceCountRef.current = 0;
    boxRef.current = null;
    dwell.current = {
      anchor: null,
      stableSince: null,
      stableMs: 0,
      samples: 0,
      lastFrameAt: null,
      lastAt: 0,
    };
    flush(true);

    const stillCurrent = () =>
      mounted.current &&
      foreground.current &&
      generation.current === token &&
      pendingStart.current;

    let granted = hasPermission;
    try {
      if (!granted && canRequestPermission) {
        // Explicit operator action: this is the only permission prompt path.
        granted = await requestPermission();
      }
    } catch {
      if (stillCurrent()) fail(token, "PERMISSION_DENIED", "E_PERMISSION_DENIED");
      return;
    }
    if (!stillCurrent()) return;
    if (!granted) {
      fail(token, "PERMISSION_DENIED", "E_PERMISSION_DENIED");
      return;
    }

    try {
      // Explicit restart drains any previous native ownership before arming.
      await client.cancel();
      if (!stillCurrent()) return;
      await client.start(token);
    } catch (error) {
      if (stillCurrent()) {
        fail(token, "CAPTURE_UNAVAILABLE", safeCode(error));
      }
      return;
    }
    if (!stillCurrent()) return;

    pendingStart.current = false;
    activeGeneration.current = token;
    phaseRef.current = "GUIDANCE";
    reasonRef.current = "WAIT_PREVIEW";
    setSession(token);
    flush(true);
  }, [
    canRequestPermission,
    client,
    device,
    fail,
    flush,
    foreground,
    hasPermission,
    portrait,
    port,
    requestPermission,
  ]);

  const onReady = useCallback(
    (token: number) => {
      if (!current(token)) return;
      phaseRef.current = "GUIDANCE";
      reasonRef.current = "NO_FACE";
      flush(true);
    },
    [current, flush],
  );

  const onStopped = useCallback(
    (token: number) => {
      if (token !== activeGeneration.current) return;
      fail(token, "PREVIEW_STOPPED", "E_PREVIEW_STOPPED");
    },
    [fail],
  );

  const onCameraFailure = useCallback(
    (token: number, reason: "CAMERA_ERROR" | "DETECTOR_ERROR") => {
      if (!current(token)) return;
      fail(
        token,
        reason,
        reason === "DETECTOR_ERROR" ? "E_DETECTOR_ERROR" : "E_CAMERA_ERROR",
      );
    },
    [current, fail],
  );

  const completeCapture = useCallback(
    async (token: number, frameId: string, corners: number[]) => {
      if (!client) return;
      try {
        captureStatus.current = "CLAIMED";
        phaseRef.current = "CAPTURING";
        reasonRef.current = "CAPTURING";
        flush(true);
        if (acceptanceTrace.enabled) acceptanceCounts.captures++;
        acceptanceTrace.record("CAPTURE", { generation: token, frameId, ...acceptanceCounts });
        const ready = await client.decide(token, frameId, true, corners);
        if (!current(token)) return;
        if (!ready) {
          fail(token, "CAPTURE_ERROR", "E_CAPTURE_RESULT");
          return;
        }
        captureStatus.current = "CONSUMING";
        const receipt = await client.consume(token, frameId);
        await traceRelease(frameId);
        if (!current(token)) return;
        captureStatus.current = "CONSUMED";
        consumedToken.current = token;
        activeGeneration.current = null;
        pendingStart.current = false;
        generation.current = nextGeneration();
        phaseRef.current = "CONSUMED";
        reasonRef.current = "CONSUMED";
        errorRef.current = null;
        faceCountRef.current = 0;
        boxRef.current = null;
        dwell.current = {
          anchor: null,
          stableSince: null,
          stableMs: 0,
          samples: 0,
          lastFrameAt: null,
          lastAt: 0,
        };
        const meta = {
          width: receipt.width,
          height: receipt.height,
          jpegBytes: receipt.jpegBytes,
          frameId: receipt.frameId,
          generation: receipt.generation,
        };
        if (mounted.current) {
          setSession(null);
          flush(true);
          void client.cancel().catch(() => {});
          callbacks.current.onConsumed(meta);
        }
      } catch (error) {
        if (current(token)) {
          fail(token, "CAPTURE_ERROR", safeCode(error));
        }
      }
    },
    [client, current, fail, flush],
  );

  const onFrame = useCallback(
    (input: FrameMetadata | undefined) => {
      if (!input || !client || consumedToken.current !== null) return;
      const token = input.generation;
      if (
        !Number.isInteger(token) ||
        typeof input.frameId !== "string" ||
        !validFrameId(input.frameId) ||
        !client.claimFrame(input.frameId)
      ) {
        return;
      }
      if (
        token !== activeGeneration.current ||
        token !== generation.current ||
        !mounted.current ||
        !foreground.current ||
        captureStatus.current !== "ARMED"
      ) {
        // Not ours. The native 400 ms ownership deadline releases it.
        return;
      }
      const preview = camera.current;
      if (!preview) return;

      if (
        !Array.isArray(input.faces) ||
        !Number.isFinite(input.acquiredAtMs) ||
        !Number.isFinite(input.emittedAtMs) ||
        input.emittedAtMs < input.acquiredAtMs ||
        input.emittedAtMs - input.acquiredAtMs > MAX_GAP_MS
      ) {
        fail(token, "DETECTOR_ERROR", "E_DETECTOR_ERROR");
        return;
      }

      const at = now();
      const view = sizeRef.current;
      const last = dwell.current;
      const traceFrame = acceptanceTrace.wantsFrame();
      const traceGap = traceFrame && last.lastFrameAt !== null ? at - last.lastFrameAt : 0;
      if (last.lastAt > 0 && at < last.lastAt) {
        resetDwell("CLOCK_RESET", at);
      } else if (
        last.lastFrameAt !== null &&
        at - last.lastFrameAt > MAX_GAP_MS
      ) {
        resetDwell("CALLBACK_GAP", at);
      }

      const faces = input.faces;
      let candidate: Candidate | null = null;
      let frame: Size = { width: 0, height: 0 };
      let box: Rect | null = null;
      if (faces.length === 1) {
        const face = faces[0];
        if (face) {
          const dimensions = {
            width: face.frameWidth,
            height: face.frameHeight,
          };
          if (validSize(dimensions) && validSize(view)) {
            const mapped = mapBoundsToView(face.bounds, (point) =>
              preview.convertCameraPointToViewPoint(point),
            );
            const pose = {
              yaw: face.yawAngle,
              pitch: face.pitchAngle,
              roll: face.rollAngle,
            };
            if (mapped && Object.values(pose).every(Number.isFinite)) {
              box = mapped;
              frame = dimensions;
              candidate = { box: mapped, ...pose, view };
            }
          }
        }
      }

      faceCountRef.current = faces.length;
      boxRef.current = box;

      const guidance = assessObservation(faces.length, candidate, frame);
      if (!guidance.valid || !candidate) {
        resetDwell(guidance.reason, at);
      } else {
        const anchor = dwell.current.anchor;
        const stable = anchor !== null && samePosition(anchor, candidate);
        if (!stable) {
          if (anchor !== null) {
            acceptanceTrace.record("RESET", { generation: token, frameId: input.frameId, reason: "MOVEMENT" });
          }
          dwell.current.anchor = candidate;
          dwell.current.stableSince = at;
          dwell.current.stableMs = 0;
          dwell.current.samples = 1;
          reasonRef.current = "HOLD_STILL";
        } else {
          dwell.current.samples += 1;
          dwell.current.stableMs =
            dwell.current.stableSince === null
              ? 0
              : at - dwell.current.stableSince;
        }
      }
      dwell.current.lastFrameAt = at;
      dwell.current.lastAt = at;

      const dwellReached =
        guidance.valid &&
        candidate !== null &&
        dwell.current.stableMs >= DWELL_MS &&
        dwell.current.samples >= MIN_SAMPLES;
      if (dwellReached) {
        reasonRef.current = "READY";
        acceptanceTrace.record("CAPTURE_READY", {
          generation: token, frameId: input.frameId,
          samples: dwell.current.samples, dwellMs: dwell.current.stableMs,
        });
      }
      if (acceptanceTrace.enabled) {
        if (traceStage.current.generation !== token) traceStage.current = { generation: token, stage: 0 };
        if (faces.length === 1 && traceStage.current.stage === 0) {
          acceptanceTrace.record("DETECTED", { generation: token, frameId: input.frameId, faceCount: 1 });
          traceStage.current.stage = 1;
        }
        if (guidance.valid && traceStage.current.stage === 1) {
          acceptanceTrace.record("GEOMETRY_VALID", { generation: token, frameId: input.frameId });
          traceStage.current.stage = 2;
        }
        if (guidance.valid && dwell.current.samples >= 2 && traceStage.current.stage === 2) {
          acceptanceTrace.record("STABLE", {
            generation: token, frameId: input.frameId,
            samples: dwell.current.samples, dwellMs: dwell.current.stableMs,
          });
          traceStage.current.stage = 3;
        }
      }
      if (traceFrame) {
        const detected = faces.length === 1 ? faces[0] : undefined;
        const oval = previewOval(view);
        let roundTripError: number | undefined;
        try {
          roundTripError = 0;
          for (const point of [
            { x: 0, y: 0 }, { x: view.width, y: 0 },
            { x: view.width, y: view.height }, { x: 0, y: view.height },
          ]) {
            const restored = preview.convertCameraPointToViewPoint(preview.convertViewPointToCameraPoint(point));
            roundTripError = Math.max(roundTripError, Math.hypot(restored.x - point.x, restored.y - point.y));
          }
        } catch {
          roundTripError = undefined;
        }
        acceptanceTrace.record("FRAME", {
          generation: token, frameId: input.frameId, faceCount: faces.length,
          reason: reasonRef.current, samples: dwell.current.samples,
          dwellMs: dwell.current.stableMs, gapMs: traceGap,
          preview: view,
          roundTripError,
          analysisImage: detected ? { width: detected.frameWidth, height: detected.frameHeight } : undefined,
          detectorBoundsSensor: detected ? detected.bounds : null,
          previewBoxNormalized: box && validSize(view)
            ? { x: box.x / view.width, y: box.y / view.height, width: box.width / view.width, height: box.height / view.height }
            : null,
          roiNormalized: oval && validSize(view)
            ? { cx: oval.cx / view.width, cy: oval.cy / view.height, rx: oval.rx / view.width, ry: oval.ry / view.height }
            : null,
        });
      }
      flush(dwellReached);

      if (dwellReached && captureStatus.current === "ARMED") {
        // Convert the four preview corners back to sensor space once. Native
        // rejects crops it cannot support; JS never guesses a transform.
        const points = [
          { x: 0, y: 0 },
          { x: view.width, y: 0 },
          { x: view.width, y: view.height },
          { x: 0, y: view.height },
        ];
        const corners = points.flatMap((point) => {
          const sensor = preview.convertViewPointToCameraPoint(point);
          return [sensor.x, sensor.y];
        });
        if (corners.every(Number.isFinite)) {
          void completeCapture(token, input.frameId, corners);
          return;
        }
        fail(token, "CAPTURE_ERROR", "E_CAPTURE_RESULT");
        return;
      }

      // Every claimed frame is explicitly acknowledged so native ownership
      // cannot accumulate while guidance is not ready.
      void client
        .decide(token, input.frameId, false, null)
        .catch(() => {});
    },
    [
      client,
      completeCapture,
      fail,
      flush,
      foreground,
      resetDwell,
    ],
  );

  useEffect(() => {
    mounted.current = true;
    foreground.current =
      AppState.currentState === "active" ||
      AppState.currentState === "unknown";
    const subscription = AppState.addEventListener("change", (state) => {
      const active = state === "active";
      foreground.current = active;
      if (!active) {
        if (
          activeGeneration.current !== null ||
          pendingStart.current
        ) {
          stop("BACKGROUND", null);
        }
      }
    });
    return () => {
      mounted.current = false;
      generation.current = nextGeneration();
      activeGeneration.current = null;
      pendingStart.current = false;
      consumedToken.current = null;
      subscription.remove();
      void client?.cancel().catch(() => {});
    };
  }, [client, stop]);

  useEffect(() => {
    if (
      !portrait &&
      (activeGeneration.current !== null || pendingStart.current)
    ) {
      stop("PORTRAIT_REQUIRED", null);
    }
  }, [portrait, stop]);

  useEffect(() => {
    if (!port || !client) return;
    let subscription: { remove(): void } | null = null;
    try {
      // The host owns the event subscription shape; no NativeEventEmitter.
      subscription = port.addCaptureFrameListener((payload) =>
        onFrame(payload as FrameMetadata),
      );
    } catch {
      phaseRef.current = "ERROR";
      reasonRef.current = "CAPTURE_UNAVAILABLE";
      errorRef.current = "E_CAPTURE_UNAVAILABLE";
      flush(true);
    }
    return () => subscription?.remove();
  }, [client, flush, onFrame, port]);

  useEffect(() => {
    if (!client) return;
    const timer = setInterval(() => {
      if (
        activeGeneration.current === null ||
        captureStatus.current !== "ARMED"
      ) {
        return;
      }
      const at = now();
      const lastFrameAt = dwell.current.lastFrameAt;
      if (lastFrameAt !== null && at - lastFrameAt > MAX_GAP_MS) {
        dwell.current.lastFrameAt = null;
        resetDwell("CALLBACK_GAP", at);
        flush(true);
      }
    }, UI_INTERVAL_MS);
    return () => clearInterval(timer);
  }, [client, flush, resetDwell]);

  useEffect(() => {
    if (session !== null && !hasPermission) {
      fail(session, "PERMISSION_DENIED", "E_PERMISSION_DENIED");
    }
  }, [fail, hasPermission, session]);

  const onLayout = useCallback(
    (event: LayoutChangeEvent) => {
      const { width, height } = event.nativeEvent.layout;
      if (width === sizeRef.current.width && height === sizeRef.current.height) {
        return;
      }
      const next = { width, height };
      sizeRef.current = next;
      setViewSize(next);
      boxRef.current = null;
      const token = activeGeneration.current;
      if (token !== null) {
        resetDwell("LAYOUT_CHANGED", now());
      }
      flush(true);
    },
    [flush, resetDwell],
  );

  const oval = previewOval(viewSize);
  const progress = Math.min(1, ui.stableMs / DWELL_MS);
  const percent = Math.round(progress * 100);
  const previewHeight = Math.min(
    Math.max(dimensions.width * 1.2, 260),
    dimensions.height * 0.52,
  );
  const active = session !== null;
  const busy =
    ui.phase === "STARTING" || ui.phase === "CAPTURING";
  const canStart =
    ui.phase === "IDLE" ||
    ui.phase === "ERROR" ||
    ui.phase === "GUIDANCE";
  const guidanceText =
    ui.errorCode !== null
      ? codeMessage(ui.errorCode)
      : messages[ui.reason];

  return (
    <View style={styles.screen}>
      <View
        testID="enr02b-preview"
        style={[styles.preview, { height: previewHeight }]}
        onLayout={onLayout}
      >
        {session !== null && device && portrait ? (
          <LiveCamera
            key={session}
            generation={session}
            device={device}
            cameraRef={camera}
            onReady={onReady}
            onStopped={onStopped}
            onFailure={onCameraFailure}
          />
        ) : (
          <View style={styles.placeholder}>
            <Text style={styles.placeholderTitle}>
              {ui.phase === "CONSUMED"
                ? "Bingkai sudah diproses"
                : "Kamera mati"}
            </Text>
            <Text style={styles.placeholderBody}>
              {ui.phase === "CONSUMED"
                ? "Tidak ada gambar yang ditampilkan, disimpan, atau diunggah."
                : "Kamera hanya menyala setelah Anda menekan tombol mulai. Aplikasi tidak menyalakan kamera saat dibuka atau dilanjutkan."}
            </Text>
          </View>
        )}
        <View
          pointerEvents="none"
          style={StyleSheet.absoluteFill}
          accessible
          accessibilityRole="image"
          accessibilityLabel="Oval panduan wajah di tengah pratinjau"
        >
          {oval ? (
            <View
              style={[
                styles.oval,
                {
                  left: oval.cx - oval.rx,
                  top: oval.cy - oval.rx,
                  width: oval.rx * 2,
                  height: oval.rx * 2,
                  borderRadius: oval.rx,
                  transform: [{ scaleY: oval.ry / oval.rx }],
                  borderColor:
                    ui.phase === "GUIDANCE" && ui.reason === "READY"
                      ? "#53e49d"
                      : ui.phase === "CAPTURING" || ui.phase === "CONSUMED"
                        ? "#53e49d"
                        : "#f5f9ff",
                },
              ]}
            />
          ) : null}
          {ui.box && active ? (
            <View
              style={[
                styles.faceBox,
                {
                  left: ui.box.x,
                  top: ui.box.y,
                  width: ui.box.width,
                  height: ui.box.height,
                },
              ]}
            />
          ) : null}
        </View>
      </View>

      <View
        accessible
        accessibilityLiveRegion="polite"
        accessibilityLabel={
          busy
            ? "Memproses bingkai"
            : ui.errorCode
              ? codeMessage(ui.errorCode)
              : messages[ui.reason]
        }
        style={styles.card}
      >
        <Text testID="enr02b-status" style={styles.state}>
          {ui.phase} · {ui.reason}
        </Text>
        <Text style={styles.guidance}>{guidanceText}</Text>
        <View
          style={[styles.progressTrack, { backgroundColor: "#314350" }]}
          accessibilityRole="progressbar"
          accessibilityLabel="Kemajuan kesiapan pemindaian"
          accessibilityValue={{ min: 0, max: 100, now: percent }}
        >
          <View
            style={[styles.progressFill, { width: `${percent}%` }]}
          />
        </View>
        <Text style={styles.small}>
          {Math.round(ui.stableMs)} / {DWELL_MS} ms stabil ·{" "}
          {ui.samples} observasi · {ui.faceCount} wajah
        </Text>
        <Text style={styles.small}>
          Preview kamera tampil hanya di perangkat ini. Hasil capture tidak disimpan,
          diunggah, atau ditampilkan sebagai gambar; hanya metadata sementara yang
          dipakai untuk menyelesaikan pemindaian.
        </Text>
      </View>

      <View style={styles.controls}>
        {ui.phase === "CONSUMED" ? (
          <Pressable
            testID="enr02b-done"
            accessibilityRole="button"
            accessibilityLabel="Kembali setelah pemindaian selesai"
            onPress={handleCancel}
            style={[styles.button, styles.primary]}
          >
            <Text style={styles.buttonText}>Kembali</Text>
          </Pressable>
        ) : active || busy ? (
          <Pressable
            testID="enr02b-cancel"
            accessibilityRole="button"
            accessibilityLabel="Batalkan pemindaian dan kembali"
            accessibilityState={{ disabled: false }}
            onPress={handleCancel}
            style={styles.button}
          >
            <Text style={styles.buttonText}>Batalkan pemindaian</Text>
          </Pressable>
        ) : (
          <Pressable
            testID="enr02b-start"
            accessibilityRole="button"
            accessibilityLabel={
              hasPermission
                ? "Mulai kamera"
                : canRequestPermission
                  ? "Mulai kamera dan minta izin"
                  : "Coba lagi; izin kamera perlu diberikan melalui pengaturan"
            }
            accessibilityState={{
              disabled: !canStart || !portrait,
              busy,
            }}
            disabled={!canStart || !portrait}
            onPress={() => {
              void start();
            }}
            style={[
              styles.button,
              styles.primary,
              (!canStart || !portrait) && styles.disabled,
            ]}
          >
            <Text style={styles.buttonText}>
              {ui.phase === "ERROR"
                ? "Coba lagi"
                : hasPermission
                  ? "Mulai kamera"
                  : canRequestPermission
                    ? "Mulai + minta izin"
                    : "Izin diperlukan (Pengaturan)"}
            </Text>
          </Pressable>
        )}
      </View>

      {!portrait ? (
        <Text style={styles.error}>
          PORTRAIT_REQUIRED — aktivitas kamera hanya dalam orientasi portrait.
        </Text>
      ) : null}
      {ui.phase === "CONSUMED" && ui.errorCode === null ? (
        <Text style={styles.small} accessibilityLiveRegion="polite">
          Satu bingkai telah diproses oleh modul native. Data gambar tidak
          pernah masuk ke JavaScript.
        </Text>
      ) : null}
    </View>
  );
}

export default NativeScanner;

const styles = StyleSheet.create({
  screen: { flex: 1, padding: 14, gap: 12, backgroundColor: "#080f16" },
  preview: {
    width: "100%",
    position: "relative",
    overflow: "hidden",
    borderRadius: 16,
    backgroundColor: "#12202b",
  },
  placeholder: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
    padding: 32,
  },
  placeholderTitle: {
    color: "#fff",
    fontSize: 20,
    textAlign: "center",
    fontWeight: "600",
  },
  placeholderBody: {
    color: "#b5c4d5",
    fontSize: 13,
    textAlign: "center",
    marginTop: 8,
    lineHeight: 18,
  },
  oval: { position: "absolute", borderWidth: 2, borderStyle: "dashed" },
  faceBox: { position: "absolute", borderWidth: 2, borderColor: "#52d9ff" },
  card: {
    padding: 12,
    borderRadius: 12,
    backgroundColor: "#14212d",
    gap: 7,
  },
  state: { color: "#89e8c4", fontSize: 12, fontWeight: "700" },
  guidance: { color: "#fff", fontSize: 16, fontWeight: "600", lineHeight: 22 },
  progressTrack: { height: 6, overflow: "hidden", borderRadius: 3 },
  progressFill: { height: 6, backgroundColor: "#53e49d" },
  small: { color: "#bbc8d6", fontSize: 12, lineHeight: 17 },
  controls: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
  button: {
    backgroundColor: "#2c4052",
    minHeight: 48,
    minWidth: 48,
    paddingVertical: 12,
    paddingHorizontal: 14,
    borderRadius: 9,
    justifyContent: "center",
  },
  primary: { backgroundColor: "#23664d" },
  disabled: { opacity: 0.4 },
  buttonText: { color: "#fff", fontSize: 13, fontWeight: "600" },
  error: { color: "#ffaaaa", fontSize: 13, lineHeight: 18 },
});

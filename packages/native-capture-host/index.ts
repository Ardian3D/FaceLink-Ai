import { NativeEventEmitter, NativeModules, Platform } from 'react-native';

export type Scalar = string | number | boolean | null;
export type CaseStatus = 'PASSED' | 'FAILED' | 'PARTIAL';

export interface HostCase {
  name: string;
  passed: boolean;
  status: CaseStatus;
  observations: Record<string, Scalar>;
  assertions: Record<string, boolean>;
}

export interface HostReport {
  suite: 'FL-CAM03';
  source: 'synthetic_native_host';
  passed: boolean;
  status: CaseStatus;
  cases: HostCase[];
}

export interface HostLimits {
  maxPixels: number;
  maxJpegBytes: number;
  maxOwnedBytes: number;
  freshnessMs: number;
  encodeTimeoutMs: number;
  readyTtlMs: number;
  consumeTimeoutMs: number;
}

export interface HostStatus {
  phase: 'IDLE' | 'RUNNING' | 'DRAINING' | 'DISPOSED';
  busy: boolean;
  foreground: boolean;
  disposed: boolean;
  /** ENR's private suite does not expose live counters; unknown is never reported as zero. */
  ownershipKnown: boolean;
  ownedBytes: number | null;
  ownedFrames: number | null;
  peakOwnedBytes: number | null;
  adapterPhase: string | null;
  suiteDrained: boolean;
  lastError: string | null;
  limits: HostLimits;
}

export type InventoryRoot = 'files' | 'cache' | 'code_cache' | 'no_backup' | 'databases' | 'shared_prefs';
export interface InventoryEntry {
  root: InventoryRoot;
  /** Relative name only; no absolute sandbox paths or file contents. */
  name: string;
  kind: 'file' | 'directory' | 'symlink' | 'other';
  bytes: number;
  modifiedAtMs: number;
  signature: 'jpeg' | 'png' | 'gif' | 'webp' | null;
}

export interface Inventory {
  scope: 'app_private_metadata_only';
  capturedAtMs: number;
  maxEntries: number;
  maxDepth: number;
  maxSignatureBytes: number;
  truncated: boolean;
  signatureBytesRead: number;
  entries: InventoryEntry[];
  warnings: string[];
}

interface CaptureHostBridge {
  runSyntheticVerification(): Promise<HostReport>;
  cancel(): Promise<void>;
  dispose(): Promise<void>;
  getStatus(): Promise<HostStatus>;
  getStorageInventory(): Promise<Inventory>;
}

function bridge(): CaptureHostBridge {
  const module = NativeModules.FaceLinkCaptureHost as CaptureHostBridge | undefined;
  if (Platform.OS !== 'android' || !module) {
    throw new Error('E_CAPTURE_HOST_UNAVAILABLE');
  }
  return module;
}

/** Synthetic native fixtures only; no image capability is exposed to JavaScript. */
export async function runSyntheticVerification(): Promise<HostReport> {
  return bridge().runSyntheticVerification();
}

/** Resolves only after running verification and native codec/consumer work physically drain. */
export async function cancel(): Promise<void> {
  return bridge().cancel();
}

/** Terminal for this native module instance; also waits for physical drain. */
export async function dispose(): Promise<void> {
  return bridge().dispose();
}

export async function getStatus(): Promise<HostStatus> {
  return bridge().getStatus();
}

/** Bounded metadata inventory; preference/database payloads are never opened. */
export async function getStorageInventory(): Promise<Inventory> {
  return bridge().getStorageInventory();
}

/*
 * FL-ENR02B live capture bridge (`FaceLinkCaptureLive`, ported from the CAM00 proof).
 *
 * Everything below is metadata-only. No JPEG bytes, base64 payloads, file paths, frame
 * buffers or camera handles cross React Native: `handle` is the single opaque consume-once
 * token and is returned only by `decide` after an approved capture.
 */

export interface CaptureFrameBounds {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface CaptureFrameFace {
  bounds: CaptureFrameBounds;
  frameWidth: number;
  frameHeight: number;
  yawAngle: number;
  pitchAngle: number;
  rollAngle: number;
}

/** Payload of the native `CaptureFrame` event. Contains geometry and labels only. */
export interface CaptureFrameEvent {
  generation: number;
  frameId: string;
  acquiredAtMs: number;
  emittedAtMs: number;
  faces: CaptureFrameFace[];
}

export interface LivePreviewGeometry {
  generation: number;
  inputWidth: number;
  inputHeight: number;
  rotationDegrees: number;
  minX: number;
  maxX: number;
  minY: number;
  maxY: number;
  cropLeft: number;
  cropTop: number;
  cropRight: number;
  cropBottom: number;
}

export interface LiveCaptureCaps {
  maxPixels: number;
  maxJpegBytes: number;
  maxOwnedBytes: number;
  freshnessMs: number;
  encodeTimeoutMs: number;
  readyTtlMs: number;
  consumeTimeoutMs: number;
}

/** Bounded private-directory observation; counters only, no sandbox paths or file contents. */
export interface SandboxWatchSnapshot {
  active: boolean;
  startedAtMs: number;
  stoppedAtMs: number;
  createCount: number;
  closeWriteCount: number;
  deleteCount: number;
  moveCount: number;
  overflowCount: number;
  registrationErrors: number;
  watchedDirectories: number;
  baselineEntries: number;
  baselineTruncated: boolean;
  directoriesTruncated: boolean;
  coverage: string;
}

/** Process-lifetime counters including historical debug traffic. */
export interface LiveUidTraffic {
  uid: number;
  txBytes: number;
  txPackets: number;
  rxBytes: number;
  rxPackets: number;
  scope: string;
}

/** Metadata-only status. Declared as a type alias so it stays assignable to Record<string, unknown>. */
export type LiveCaptureStatus = {
  /** `null` before the first `start` and after `cancel`. */
  generation: number | null;
  phase: string;
  heldFrames: number;
  ownedBytes: number;
  peakOwnedBytes: number;
  busy: boolean;
  lastResult: string;
  lastError: string | null;
  previewGeometry: LivePreviewGeometry | null;
  foreground: boolean;
  syntheticRunning: boolean;
  caps: LiveCaptureCaps;
  sandboxWatch: SandboxWatchSnapshot;
  uidTraffic: LiveUidTraffic;
};

export interface LiveCaptureReady {
  generation: number;
  frameId: string;
  /** Opaque consume-once capability; the only handle that crosses the bridge. */
  handle: string;
  jpegBytes: number;
  width: number;
  height: number;
  encodeMs: number;
  selectedAgeMs: number;
  peakOwnedBytes: number;
  expiresAtMs: number;
  exif: boolean;
  status: 'READY';
  mirrored: boolean;
  cropMode: 'CONTAINED_EVEN_INSET';
  cropInsetPixels: number;
}

export interface LiveConsumeReceipt {
  generation: number;
  frameId: string;
  width: number;
  height: number;
  jpegBytes: number;
  exif: boolean;
  consumed: true;
  status: 'CONSUMED';
}

export interface FaceLinkCaptureLiveApi {
  /** Arms the native frame store. Requires foreground, FLAG_SECURE and granted CAMERA. */
  start(generation: number): Promise<void>;
  /** Rejects are stable `E_CAPTURE_*` codes; `null` means the frame was released, not captured. */
  decide(
    generation: number,
    frameId: string,
    capture: boolean,
    sensorCorners: number[] | null
  ): Promise<LiveCaptureReady | null>;
  consume(generation: number, handle: string): Promise<LiveConsumeReceipt>;
  cancel(): Promise<void>;
  /** Terminal for this native module instance. */
  dispose(): Promise<void>;
  getStatus(): Promise<LiveCaptureStatus>;
  addCaptureFrameListener(listener: (event: CaptureFrameEvent) => void): { remove(): void };
}

interface CaptureLiveBridge {
  start(generation: number): Promise<unknown>;
  decide(
    generation: number,
    frameId: string,
    capture: boolean,
    sensorCorners: number[] | null
  ): Promise<LiveCaptureReady | null>;
  consume(generation: number, handle: string): Promise<LiveConsumeReceipt>;
  cancel(): Promise<unknown>;
  dispose(): Promise<unknown>;
  getStatus(): Promise<LiveCaptureStatus>;
  addListener(eventName: string): void;
  removeListeners(count: number): void;
}

/**
 * Present only when the installed face-detector carries the patched CAM02 source set
 * (`apply-detector-live.cjs`). Synthetic-only hosts reject with E_CAPTURE_LIVE_UNAVAILABLE.
 */
function liveBridge(): CaptureLiveBridge {
  const module = NativeModules.FaceLinkCaptureLive as CaptureLiveBridge | undefined;
  if (Platform.OS !== 'android' || !module) {
    throw new Error('E_CAPTURE_LIVE_UNAVAILABLE');
  }
  return module;
}

function settle(action: Promise<unknown>): Promise<void> {
  return action.then(() => undefined);
}

/** True only when the patched native live module is registered. */
export function isCaptureLiveAvailable(): boolean {
  return Platform.OS === 'android' && NativeModules.FaceLinkCaptureLive != null;
}

export const FaceLinkCaptureLive: FaceLinkCaptureLiveApi = Object.freeze({
  async start(generation: number): Promise<void> {
    await settle(liveBridge().start(generation));
  },
  async decide(
    generation: number,
    frameId: string,
    capture: boolean,
    sensorCorners: number[] | null
  ): Promise<LiveCaptureReady | null> {
    return liveBridge().decide(generation, frameId, capture, sensorCorners);
  },
  async consume(generation: number, handle: string): Promise<LiveConsumeReceipt> {
    return liveBridge().consume(generation, handle);
  },
  async cancel(): Promise<void> {
    await settle(liveBridge().cancel());
  },
  async dispose(): Promise<void> {
    await settle(liveBridge().dispose());
  },
  async getStatus(): Promise<LiveCaptureStatus> {
    return liveBridge().getStatus();
  },
  /** Registers the native listener counter through NativeEventEmitter and returns its subscription. */
  addCaptureFrameListener(listener: (event: CaptureFrameEvent) => void): { remove(): void } {
    const emitter = new NativeEventEmitter<{ CaptureFrame: [CaptureFrameEvent] }>(liveBridge());
    return emitter.addListener('CaptureFrame', listener);
  },
});

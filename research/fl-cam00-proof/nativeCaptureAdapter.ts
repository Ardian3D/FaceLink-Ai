export type ImageMetadata = Readonly<{
  generation: number;
  frameId: string;
  width: number;
  height: number;
  jpegBytes: number;
  exif: false;
}>;
export type ReadyMetadata = ImageMetadata & Readonly<{
  status: 'READY';
  encodeMs: number;
  selectedAgeMs: number;
  peakOwnedBytes: number;
  /** Android monotonic clock, not UTC or the JavaScript performance origin. */
  expiresAtMs: number;
}>;
export type ConsumeReceipt = ImageMetadata & Readonly<{ status: 'CONSUMED'; consumed: true }>;
type NativeReady = ReadyMetadata & { handle: string };
export type VerificationCase = {
  name: string;
  passed: boolean;
  observations: Record<string, string | number | boolean | null>;
  assertions: Record<string, boolean>;
};
export type VerificationReport = {
  suite: string;
  source: string;
  passed: boolean;
  cases: VerificationCase[];
};
export interface NativeCapturePort {
  start(generation: number): Promise<void>;
  decide(generation: number, frameId: string, capture: boolean, sensorCorners: number[] | null): Promise<NativeReady | null>;
  consume(generation: number, handle: string): Promise<ConsumeReceipt>;
  cancel(): Promise<void>;
  dispose(): Promise<void>;
  getStatus(): Promise<Record<string, unknown>>;
  runSyntheticVerification(): Promise<VerificationReport>;
  addListener(eventName: string): void;
  removeListeners(count: number): void;
}

export class CaptureAdapterError extends Error {
  readonly code: string;
  constructor(code: string) { super(code); this.name = 'CaptureAdapterError'; this.code = code; }
}
export function safeCaptureCode(error: unknown): string {
  try {
    const code = (error as { code?: unknown } | null)?.code;
    return typeof code === 'string' && /^E_(?:CAPTURE|ENR01A|CAM02)_[A-Z0-9_]{1,60}$/.test(code)
      ? code : 'E_CAPTURE_NATIVE';
  } catch { return 'E_CAPTURE_NATIVE'; }
}
function fail(code: string): never { throw new CaptureAdapterError(code); }
const validGeneration = (value: number) => Number.isSafeInteger(value) && value > 0;
const validFrameId = (value: string) => /^[1-9][0-9]{0,18}$/.test(value)
  && (value.length < 19 || value <= '9223372036854775807');
function imageMetadata(value: ImageMetadata): boolean {
  return validGeneration(value.generation) && typeof value.frameId === 'string' && validFrameId(value.frameId)
    && Number.isInteger(value.width) && Number.isInteger(value.height)
    && value.width > 0 && value.height > 0 && value.width * value.height <= 2073600
    && Number.isInteger(value.jpegBytes) && value.jpegBytes > 0 && value.jpegBytes <= 4194304
    && value.exif === false;
}
function readyMetadata(value: NativeReady, generation: number, frameId: string): boolean {
  return imageMetadata(value) && value.generation === generation && value.frameId === frameId
    && value.status === 'READY' && typeof value.handle === 'string'
    && value.handle.length >= 16 && value.handle.length <= 128
    && [value.encodeMs, value.selectedAgeMs, value.peakOwnedBytes, value.expiresAtMs].every(Number.isFinite)
    && value.encodeMs >= 0 && value.selectedAgeMs >= 0 && value.selectedAgeMs <= 400
    && value.peakOwnedBytes >= value.jpegBytes && value.peakOwnedBytes <= 16777216 && value.expiresAtMs > 0;
}
function publicReady(value: NativeReady): ReadyMetadata {
  return Object.freeze({
    generation: value.generation, frameId: value.frameId, width: value.width, height: value.height,
    jpegBytes: value.jpegBytes, exif: false, status: 'READY', encodeMs: value.encodeMs,
    selectedAgeMs: value.selectedAgeMs, peakOwnedBytes: value.peakOwnedBytes, expiresAtMs: value.expiresAtMs,
  });
}
function publicReceipt(value: ConsumeReceipt): ConsumeReceipt {
  return Object.freeze({
    generation: value.generation, frameId: value.frameId, width: value.width, height: value.height,
    jpegBytes: value.jpegBytes, exif: false, status: 'CONSUMED', consumed: true,
  });
}

/** Opaque capability stays private and is never serialized with diagnostic metadata. */
export class CaptureLease {
  readonly metadata: ReadyMetadata;
  #handle: string;
  #state: 'READY' | 'CONSUMING' | 'CONSUMED' | 'INVALID' = 'READY';
  #port: NativeCapturePort;
  #current: () => boolean;
  constructor(value: NativeReady, port: NativeCapturePort, current: () => boolean) {
    this.metadata = publicReady(value);
    this.#handle = value.handle;
    this.#port = port;
    this.#current = current;
  }
  invalidate(): void { this.#state = 'INVALID'; this.#handle = ''; }
  toJSON(): ReadyMetadata { return this.metadata; }
  async consumeOnce(): Promise<ConsumeReceipt> {
    if (this.#state !== 'READY') fail('E_CAPTURE_ALREADY_CLAIMED');
    if (!this.#current()) { this.invalidate(); fail('E_CAPTURE_STALE_GENERATION'); }
    this.#state = 'CONSUMING';
    const handle = this.#handle;
    this.#handle = '';
    try {
      const receipt = await this.#port.consume(this.metadata.generation, handle);
      if (!this.#current()) fail('E_CAPTURE_CANCELLED');
      if (!receipt || !imageMetadata(receipt) || receipt.status !== 'CONSUMED' || receipt.consumed !== true
        || receipt.generation !== this.metadata.generation || receipt.frameId !== this.metadata.frameId
        || receipt.width !== this.metadata.width || receipt.height !== this.metadata.height
        || receipt.jpegBytes !== this.metadata.jpegBytes) fail('E_CAPTURE_RESULT');
      this.#state = 'CONSUMED';
      return publicReceipt(receipt);
    } catch (error) {
      this.#state = 'INVALID';
      throw new CaptureAdapterError(safeCaptureCode(error));
    }
  }
}

/** Host owns permission/foreground and camera mounting; the native adapter independently guards them. */
export class CaptureAdapter {
  #epoch = 0;
  #generation: number | null = null;
  #highestGeneration = 0;
  #disposed = false;
  #lease: CaptureLease | null = null;
  #captureClaimed = false;
  #port: NativeCapturePort;
  constructor(port: NativeCapturePort) { this.#port = port; }
  async start(generation: number): Promise<void> {
    if (this.#disposed) fail('E_CAPTURE_DISPOSED');
    if (!validGeneration(generation) || generation <= this.#highestGeneration) fail('E_CAPTURE_STALE_GENERATION');
    if (this.#generation !== null) fail('E_CAPTURE_BUSY');
    const epoch = ++this.#epoch;
    this.#highestGeneration = generation;
    this.#generation = generation;
    this.#captureClaimed = false;
    try {
      await this.#port.start(generation);
      if (this.#disposed || this.#epoch !== epoch) fail('E_CAPTURE_CANCELLED');
    } catch (error) {
      if (this.#epoch === epoch) this.#generation = null;
      throw new CaptureAdapterError(safeCaptureCode(error));
    }
  }
  async decide(generation: number, frameId: string, capture: boolean, corners: number[] | null): Promise<CaptureLease | null> {
    if (this.#disposed) fail('E_CAPTURE_DISPOSED');
    if (!validGeneration(generation) || typeof frameId !== 'string' || !validFrameId(frameId)) fail('E_CAPTURE_IDENTITY');
    if (capture && this.#generation !== generation) fail('E_CAPTURE_STALE_GENERATION');
    if (capture && this.#captureClaimed) fail('E_CAPTURE_BUSY');
    if (capture) this.#captureClaimed = true;
    const epoch = this.#epoch;
    try {
      const result = await this.#port.decide(generation, frameId, capture, corners);
      if (!capture) return null;
      if (this.#disposed || epoch !== this.#epoch || generation !== this.#generation) fail('E_CAPTURE_CANCELLED');
      if (!result || !readyMetadata(result, generation, frameId)) {
        await this.cancel();
        fail('E_CAPTURE_RESULT');
      }
      const lease = new CaptureLease(result, this.#port,
        () => !this.#disposed && this.#epoch === epoch && this.#generation === generation);
      this.#lease = lease;
      return lease;
    } catch (error) { throw new CaptureAdapterError(safeCaptureCode(error)); }
  }
  async cancel(): Promise<void> {
    this.#epoch++;
    this.#generation = null;
    this.#lease?.invalidate();
    this.#lease = null;
    try { await this.#port.cancel(); }
    catch (error) { throw new CaptureAdapterError(safeCaptureCode(error)); }
  }
  async dispose(): Promise<void> {
    if (this.#disposed) return;
    this.#disposed = true;
    this.#epoch++;
    this.#generation = null;
    this.#lease?.invalidate();
    this.#lease = null;
    try { await this.#port.dispose(); }
    catch (error) { throw new CaptureAdapterError(safeCaptureCode(error)); }
  }
}

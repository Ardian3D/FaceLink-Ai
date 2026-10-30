/** Debug-only mannequin metadata. Generated into the staging tree, never main. */
export type TraceEvent =
  | "START" | "FRAME" | "RESET" | "DETECTED" | "GEOMETRY_VALID" | "STABLE" | "CAPTURE_READY" | "CAPTURE"
  | "JPEG_READY" | "CONSUMED" | "RELEASED" | "DRAINED" | "CANCELLED"
  | "STALE_REJECTED" | "DUPLICATE_REJECTED" | "ERROR";

type Size = { width: number; height: number };
type Rect = Size & { x: number; y: number };
type Oval = { cx: number; cy: number; rx: number; ry: number };
export type TraceFields = {
  generation?: number | null;
  frameId?: string;
  faceCount?: number;
  detectorBoundsSensor?: Rect | null;
  previewBoxNormalized?: Rect | null;
  nativeCropNormalized?: Rect | null;
  nativePreviewNormalized?: Rect | null;
  roiNormalized?: Oval | null;
  preview?: Size;
  analysisImage?: Size;
  buffer?: Size;
  rotationDegrees?: number;
  mirrored?: boolean;
  roundTripError?: number;
  nativePhase?: string;
  nativeError?: string | null;
  reason?: string;
  samples?: number;
  dwellMs?: number;
  gapMs?: number;
  ownedBytes?: number;
  peakOwnedBytes?: number;
  heldFrames?: number;
  busy?: boolean;
  captures?: number;
  consumes?: number;
  releases?: number;
  jpegBytes?: number;
};

const numericKeys = [
  "faceCount", "rotationDegrees", "roundTripError", "samples", "dwellMs", "gapMs",
  "ownedBytes", "peakOwnedBytes", "heldFrames", "captures", "consumes", "releases", "jpegBytes",
] as const;
const finite = (value: unknown): value is number =>
  typeof value === "number" && Number.isFinite(value);
const rounded = (value: number) => Math.round(value * 100000) / 100000;

export class MannequinTrace {
  private frames = 0;
  private transitions = 0;
  private resets = 0;
  readonly enabled: boolean;
  private readonly sink: (line: string) => void;
  private readonly clock: () => number;
  private lastFrameAt = -Infinity;

  constructor(
    enabled: boolean,
    sink: (line: string) => void = (line) => console.info("ENR02B_R2", line),
    clock: () => number = () => performance.now(),
  ) {
    this.enabled = enabled;
    this.sink = sink;
    this.clock = clock;
  }

  /** One geometry record/second, at most 90, plus 64 transition records/instance. */
  wantsFrame(): boolean {
    return this.enabled && this.frames < 90 && this.clock() - this.lastFrameAt >= 1000;
  }

  record(event: TraceEvent, fields: TraceFields): void {
    if (!this.enabled) return;
    const at = this.clock();
    if (!finite(at)) return;
    if (event === "FRAME") {
      if (!this.wantsFrame()) return;
      this.frames++;
      this.lastFrameAt = at;
    } else {
      // Preserve terminal evidence even if movement repeatedly resets the dwell.
      if (event === "RESET") {
        if (this.resets >= 16) return;
        this.resets++;
      }
      if (this.transitions >= 64) return;
      this.transitions++;
    }
    // Explicit projection: never spread fields or native replies (which can contain a handle).
    const record: Record<string, unknown> = { event, atMs: rounded(at) };
    if (fields.generation === null ||
      (Number.isSafeInteger(fields.generation) && fields.generation! > 0)) {
      record.generation = fields.generation;
    }
    if (typeof fields.frameId === "string" && /^[1-9][0-9]{0,18}$/.test(fields.frameId)) {
      record.frameId = fields.frameId;
    }
    for (const key of ["reason", "nativePhase", "nativeError"] as const) {
      const value = fields[key];
      if (typeof value === "string" && /^[A-Z][A-Z0-9_]{0,63}$/.test(value)) record[key] = value;
      else if (key === "nativeError" && value === null) record[key] = null;
    }
    for (const key of numericKeys) if (finite(fields[key])) record[key] = rounded(fields[key]!);
    for (const key of ["busy", "mirrored"] as const) {
      if (typeof fields[key] === "boolean") record[key] = fields[key];
    }
    for (const key of ["preview", "analysisImage", "buffer"] as const) {
      const value = fields[key];
      if (value && finite(value.width) && finite(value.height)) {
        record[key] = { width: rounded(value.width), height: rounded(value.height) };
      }
    }
    for (const key of ["detectorBoundsSensor", "previewBoxNormalized", "nativeCropNormalized", "nativePreviewNormalized"] as const) {
      const value = fields[key];
      if (value && [value.x, value.y, value.width, value.height].every(finite)) {
        record[key] = {
          x: rounded(value.x), y: rounded(value.y),
          width: rounded(value.width), height: rounded(value.height),
        };
      }
    }
    const oval = fields.roiNormalized;
    if (oval && [oval.cx, oval.cy, oval.rx, oval.ry].every(finite)) {
      record.roiNormalized = {
        cx: rounded(oval.cx), cy: rounded(oval.cy), rx: rounded(oval.rx), ry: rounded(oval.ry),
      };
    }
    this.sink(JSON.stringify(record));
  }
}

export type Point = { x: number; y: number };
export type Size = { width: number; height: number };
export type Rect = Point & Size;
export type Ellipse = { cx: number; cy: number; rx: number; ry: number };
export type Pose = { yaw: number; pitch: number; roll: number };
export type Candidate = Pose & { box: Rect; view: Size };
export type Reason =
  | 'OFF' | 'WAIT_PREVIEW' | 'NO_FACE' | 'MULTIPLE_FACES' | 'INVALID_GEOMETRY'
  | 'OUTSIDE_OVAL' | 'TOO_FAR' | 'YAW' | 'PITCH' | 'ROLL' | 'HOLD_STILL'
  | 'MOVEMENT' | 'CALLBACK_GAP' | 'CLOCK_RESET' | 'LAYOUT_CHANGED'
  | 'READY' | 'SIMULATED_ONLY' | 'OPERATOR_STOP' | 'BACKGROUND'
  | 'PERMISSION_DENIED' | 'PRIVACY_UNAVAILABLE' | 'CAMERA_ERROR'
  | 'DETECTOR_ERROR' | 'CAPTURE_UNAVAILABLE' | 'CAPTURE_ERROR'
  | 'PORTRAIT_REQUIRED' | 'PREVIEW_STOPPED';
export type Guidance = { valid: boolean; reason: Reason; candidate: Candidate | null };
export type Phase =
  | 'IDLE' | 'CAMERA_READY' | 'FACE_GUIDANCE' | 'STABLE'
  | 'CAPTURE_READY' | 'SIMULATED_CAPTURE' | 'CANCELLED' | 'ERROR';
export type Source = 'live' | 'synthetic_fixture';

export const DWELL_MS = 1500;
export const MAX_GAP_MS = 400;
export const MIN_SAMPLES = 6;

export function validSize(size: Size): boolean {
  return Number.isFinite(size.width) && Number.isFinite(size.height)
    && size.width > 0 && size.height > 0;
}

export function validRect(rect: Rect): boolean {
  return validSize(rect) && Number.isFinite(rect.x) && Number.isFinite(rect.y)
    && Number.isFinite(rect.x + rect.width) && Number.isFinite(rect.y + rect.height);
}

export function previewOval(view: Size): Ellipse | null {
  if (!validSize(view)) return null;
  const rx = Math.min(view.width * 0.41, view.height * 0.32);
  const ry = Math.min(rx * 1.36, view.height * 0.43);
  if (!(rx > 0 && ry > 0)) return null;
  return { cx: view.width / 2, cy: view.height / 2, rx, ry };
}

// The live path supplies VisionCamera's native converter. Transform all four
// corners: rotations/mirroring can reverse axes; scaling width alone is wrong.
export function mapBoundsToView(
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
    if (!corners.every((p) => Number.isFinite(p.x) && Number.isFinite(p.y))) return null;
    const x = Math.min(...corners.map((p) => p.x));
    const y = Math.min(...corners.map((p) => p.y));
    const result = {
      x, y,
      width: Math.max(...corners.map((p) => p.x)) - x,
      height: Math.max(...corners.map((p) => p.y)) - y,
    };
    return validRect(result) ? result : null;
  } catch {
    return null;
  }
}

// Pure reference converter for deterministic non-camera fixtures/tests only.
// Live sensor pixels are NOT normalized here: the native converter owns the
// sensor rotation, front mirror, CameraX crop, and exact preview cover transform.
export function coverFixtureConverter(
  frame: Size,
  view: Size,
  rotation: 0 | 90 | 180 | 270,
  mirrored: boolean,
): ((point: Point) => Point) | null {
  if (!validSize(frame) || !validSize(view) || ![0, 90, 180, 270].includes(rotation)) return null;
  const rotated = rotation === 90 || rotation === 270
    ? { width: frame.height, height: frame.width } : frame;
  const scale = Math.max(view.width / rotated.width, view.height / rotated.height);
  const dx = (view.width - rotated.width * scale) / 2;
  const dy = (view.height - rotated.height * scale) / 2;
  return (point) => {
    let x = point.x;
    let y = point.y;
    if (rotation === 90) { x = frame.height - point.y; y = point.x; }
    if (rotation === 180) { x = frame.width - point.x; y = frame.height - point.y; }
    if (rotation === 270) { x = point.y; y = frame.width - point.x; }
    if (mirrored) x = rotated.width - x;
    return { x: x * scale + dx, y: y * scale + dy };
  };
}

export function boxInsideEllipse(box: Rect, oval: Ellipse): boolean {
  if (!validRect(box) || !Object.values(oval).every(Number.isFinite)
    || oval.rx <= 0 || oval.ry <= 0) return false;
  // A rectangle is wholly inside a convex ellipse iff its four corners are.
  return [box.x, box.x + box.width].every((x) =>
    [box.y, box.y + box.height].every((y) =>
      ((x - oval.cx) / oval.rx) ** 2 + ((y - oval.cy) / oval.ry) ** 2 <= 1));
}

export function assessObservation(
  count: number,
  candidate: Candidate | null,
  frame: Size,
): Guidance {
  const reject = (reason: Reason): Guidance => ({ valid: false, reason, candidate: null });
  if (!Number.isInteger(count) || count < 0) return reject('INVALID_GEOMETRY');
  if (count === 0) return reject('NO_FACE');
  if (count > 1) return reject('MULTIPLE_FACES');
  if (!candidate || !validSize(frame) || !validSize(candidate.view)
    || !validRect(candidate.box)
    || ![candidate.yaw, candidate.pitch, candidate.roll].every(Number.isFinite)) {
    return reject('INVALID_GEOMETRY');
  }
  const oval = previewOval(candidate.view);
  if (!oval || !boxInsideEllipse(candidate.box, oval)) return reject('OUTSIDE_OVAL');
  if (candidate.box.width < oval.rx * 0.7 || candidate.box.height < oval.ry * 0.65) {
    return reject('TOO_FAR');
  }
  if (Math.abs(candidate.yaw) > 12) return reject('YAW');
  if (Math.abs(candidate.pitch) > 12) return reject('PITCH');
  if (Math.abs(candidate.roll) > 10) return reject('ROLL');
  return { valid: true, reason: 'HOLD_STILL', candidate };
}

export type Machine = {
  phase: Phase;
  reason: Reason;
  generation: number;
  source: Source | null;
  ready: boolean;
  lastAt: number | null;
  lastFrameAt: number | null;
  stableSince: number | null;
  stableMs: number;
  samples: number;
  anchor: Candidate | null;
  faceCount: number;
};

export function initialMachine(): Machine {
  return {
    phase: 'IDLE', reason: 'OFF', generation: 0, source: null, ready: false,
    lastAt: null, lastFrameAt: null, stableSince: null, stableMs: 0,
    samples: 0, anchor: null, faceCount: 0,
  };
}

type TimedEvent = { generation: number; at: number };
export type MachineEvent =
  | (TimedEvent & { type: 'START'; source: Source })
  | (TimedEvent & { type: 'READY' })
  | (TimedEvent & { type: 'FRAME'; count: number; guidance: Guidance })
  | (TimedEvent & { type: 'TICK' })
  | (TimedEvent & { type: 'RESET'; reason: 'LAYOUT_CHANGED' })
  | (TimedEvent & { type: 'CANCEL'; reason: Reason })
  | (TimedEvent & { type: 'ERROR'; reason: Reason });

function resetDwell(state: Machine, reason: Reason, at: number): Machine {
  return {
    ...state, phase: 'FACE_GUIDANCE', reason, lastAt: at,
    stableSince: null, stableMs: 0, samples: 0, anchor: null,
  };
}

function samePosition(anchor: Candidate, current: Candidate): boolean {
  const a = anchor.box;
  const b = current.box;
  return anchor.view.width === current.view.width && anchor.view.height === current.view.height
    && Math.abs((a.x + a.width / 2) - (b.x + b.width / 2)) <= current.view.width * 0.025
    && Math.abs((a.y + a.height / 2) - (b.y + b.height / 2)) <= current.view.height * 0.025
    && Math.abs(a.width - b.width) <= a.width * 0.06
    && Math.abs(a.height - b.height) <= a.height * 0.06
    && Math.abs(anchor.yaw - current.yaw) <= 3
    && Math.abs(anchor.pitch - current.pitch) <= 3
    && Math.abs(anchor.roll - current.roll) <= 3;
}

export function reduceMachine(state: Machine, event: MachineEvent): Machine {
  if (!Number.isFinite(event.at) || !Number.isInteger(event.generation)) return state;
  if (event.type === 'START') {
    if (event.generation <= state.generation) return state;
    return {
      ...initialMachine(), generation: event.generation, source: event.source,
      phase: 'CAMERA_READY', reason: 'WAIT_PREVIEW', lastAt: event.at,
    };
  }
  if (event.type === 'CANCEL') {
    if (event.generation <= state.generation) return state;
    return {
      ...initialMachine(), generation: event.generation, source: state.source,
      phase: 'CANCELLED', reason: event.reason, lastAt: event.at,
    };
  }
  if (event.generation !== state.generation) return state;
  if (['IDLE', 'CANCELLED', 'ERROR'].includes(state.phase)) return state;
  if (event.type === 'ERROR') {
    return {
      ...resetDwell(state, event.reason, event.at), phase: 'ERROR', ready: false,
      lastFrameAt: null,
    };
  }
  // Logical approval is not JPEG success: a native encode/geometry failure
  // must still revoke SIMULATED_CAPTURE instead of leaving false success.
  if (state.phase === 'SIMULATED_CAPTURE') return state;
  if (state.lastAt !== null && event.at < state.lastAt) {
    return { ...resetDwell(state, 'CLOCK_RESET', event.at), lastFrameAt: null };
  }
  if (event.type === 'READY') {
    if (state.ready) return state;
    return { ...state, ready: true, reason: 'NO_FACE', lastAt: event.at };
  }
  if (event.type === 'RESET') return { ...resetDwell(state, event.reason, event.at), lastFrameAt: null };
  if (!state.ready) return state;
  if (event.type === 'TICK') {
    if (state.lastFrameAt !== null && event.at - state.lastFrameAt > MAX_GAP_MS) {
      return { ...resetDwell(state, 'CALLBACK_GAP', event.at), lastFrameAt: null, faceCount: 0 };
    }
    return state;
  }
  const current = event.guidance.candidate;
  const frameState = { ...state, faceCount: event.count, lastFrameAt: event.at };
  if (!event.guidance.valid || !current || event.count !== 1) {
    return resetDwell(frameState, event.guidance.reason, event.at);
  }
  const gap = state.lastFrameAt !== null && event.at - state.lastFrameAt > MAX_GAP_MS;
  if (gap) {
    return { ...resetDwell(frameState, 'CALLBACK_GAP', event.at), lastFrameAt: event.at };
  }
  if (state.anchor && !samePosition(state.anchor, current)) {
    return resetDwell(frameState, 'MOVEMENT', event.at);
  }
  const since = state.stableSince ?? event.at;
  const stableMs = event.at - since;
  const samples = state.samples + 1;
  const dwell = {
    ...frameState, anchor: state.anchor ?? current, stableSince: since,
    stableMs, samples, lastAt: event.at,
  };
  if (state.phase === 'CAPTURE_READY') {
    return { ...dwell, phase: 'SIMULATED_CAPTURE', reason: 'SIMULATED_ONLY', ready: false };
  }
  if (stableMs >= DWELL_MS && samples >= MIN_SAMPLES) {
    return { ...dwell, phase: 'CAPTURE_READY', reason: 'READY' };
  }
  return { ...dwell, phase: 'STABLE', reason: 'HOLD_STILL' };
}

// Explicit virtual-time fixture, never scheduled as pretend camera callbacks.
// Every event passes through the very same reducer as the live stream.
export function stableFixtureEvents(
  generation: number,
  at: number,
  view: Size,
): MachineEvent[] {
  const box = { x: view.width * 0.30, y: view.height * 0.32, width: view.width * 0.40, height: view.height * 0.36 };
  const candidate = { box, view, yaw: 0, pitch: 0, roll: 0 };
  const guidance = assessObservation(1, candidate, view);
  return [
    { type: 'START', generation, at, source: 'synthetic_fixture' },
    { type: 'READY', generation, at },
    ...Array.from({ length: 10 }, (_, index): MachineEvent => ({
      type: 'FRAME', generation, at: at + index * 200, count: 1, guidance,
    })),
  ];
}

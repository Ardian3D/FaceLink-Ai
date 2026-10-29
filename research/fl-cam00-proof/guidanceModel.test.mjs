import assert from 'node:assert/strict';
import test from 'node:test';
import {
  assessObservation, boxInsideEllipse, coverFixtureConverter, DWELL_MS,
  initialMachine, mapBoundsToView, MAX_GAP_MS, previewOval, reduceMachine,
  stableFixtureEvents,
} from './guidanceModel.ts';
import { CaptureController } from './captureController.ts';

const view = { width: 300, height: 400 };
const candidate = {
  box: { x: 90, y: 128, width: 120, height: 144 },
  view, yaw: 0, pitch: 0, roll: 0,
};
const good = assessObservation(1, candidate, view);
const frame = (at, guidance = good, count = 1, generation = 1) =>
  ({ type: 'FRAME', at, generation, guidance, count });
function ready(source = 'live') {
  let state = reduceMachine(initialMachine(), { type: 'START', generation: 1, at: 0, source });
  assert.equal(state.phase, 'CAMERA_READY');
  state = reduceMachine(state, { type: 'READY', generation: 1, at: 0 });
  return state;
}
function dwell(state, start = 0) {
  for (let at = start; at <= start + 1600; at += 200) state = reduceMachine(state, frame(at));
  return state;
}

test('cover crop maps all four corners, not raw frame ratios', () => {
  const convert = coverFixtureConverter({ width: 400, height: 200 }, { width: 200, height: 200 }, 0, false);
  assert.ok(convert);
  assert.deepEqual(mapBoundsToView({ x: 120, y: 50, width: 40, height: 100 }, convert),
    { x: 20, y: 50, width: 40, height: 100 });
});

test('front mirror changes x without changing dimensions', () => {
  const convert = coverFixtureConverter({ width: 400, height: 200 }, { width: 200, height: 200 }, 0, true);
  assert.deepEqual(mapBoundsToView({ x: 120, y: 50, width: 40, height: 100 }, convert),
    { x: 140, y: 50, width: 40, height: 100 });
});

test('90/180/270 degree rotations and mirror preserve corner extents', () => {
  const frameSize = { width: 400, height: 200 };
  const box = { x: 20, y: 30, width: 80, height: 40 };
  assert.deepEqual(mapBoundsToView(box, coverFixtureConverter(frameSize, { width: 200, height: 400 }, 90, false)),
    { x: 130, y: 20, width: 40, height: 80 });
  assert.deepEqual(mapBoundsToView(box, coverFixtureConverter(frameSize, frameSize, 180, false)),
    { x: 300, y: 130, width: 80, height: 40 });
  assert.deepEqual(mapBoundsToView(box, coverFixtureConverter(frameSize, { width: 200, height: 400 }, 270, true)),
    { x: 130, y: 300, width: 40, height: 80 });
});

test('axis-reversing native-style transforms produce positive view-space extents', () => {
  const result = mapBoundsToView({ x: 10, y: 20, width: 30, height: 40 },
    (p) => ({ x: 200 - p.y * 2, y: p.x * 3 + 7 }));
  assert.deepEqual(result, { x: 80, y: 37, width: 80, height: 90 });
});

test('ellipse containment requires all corners, not merely center or oval bounding rect', () => {
  const oval = { cx: 100, cy: 100, rx: 100, ry: 100 };
  assert.equal(boxInsideEllipse({ x: 70, y: 70, width: 60, height: 60 }, oval), true);
  assert.equal(boxInsideEllipse({ x: 10, y: 10, width: 180, height: 180 }, oval), false);
  assert.equal(assessObservation(1, { ...candidate, box: { x: 20, y: 30, width: 260, height: 340 } }, view).reason, 'OUTSIDE_OVAL');
  assert.ok(previewOval(view));
});

test('invalid finite dimensions, bounds, transforms and poses never pass guidance', () => {
  assert.equal(coverFixtureConverter({ width: 0, height: 100 }, view, 0, false), null);
  assert.equal(previewOval({ width: NaN, height: 100 }), null);
  assert.equal(mapBoundsToView({ x: 0, y: 0, width: -1, height: 1 }, (p) => p), null);
  assert.equal(mapBoundsToView(candidate.box, () => ({ x: Infinity, y: 0 })), null);
  assert.equal(mapBoundsToView(candidate.box, () => { throw new Error('not ready'); }), null);
  assert.equal(assessObservation(1, candidate, { width: 0, height: 10 }).reason, 'INVALID_GEOMETRY');
  assert.equal(assessObservation(1, { ...candidate, yaw: NaN }, view).reason, 'INVALID_GEOMETRY');
  assert.equal(assessObservation(1, { ...candidate, box: { ...candidate.box, x: Infinity } }, view).valid, false);
  assert.equal(boxInsideEllipse(candidate.box, { cx: 0, cy: 0, rx: 0, ry: 1 }), false);
});

test('zero, one, multiple, too small and pose guidance are distinct', () => {
  assert.equal(assessObservation(0, null, view).reason, 'NO_FACE');
  assert.equal(assessObservation(2, null, view).reason, 'MULTIPLE_FACES');
  assert.equal(good.valid, true);
  assert.equal(assessObservation(1, { ...candidate, box: { x: 140, y: 190, width: 20, height: 20 } }, view).reason, 'TOO_FAR');
  for (const [field, reason] of [['yaw', 'YAW'], ['pitch', 'PITCH'], ['roll', 'ROLL']]) {
    assert.equal(assessObservation(1, { ...candidate, [field]: 15 }, view).reason, reason);
  }
});

test('live frames continuously dwell then require another valid frame to simulate capture', () => {
  let state = ready();
  state = reduceMachine(state, frame(0));
  assert.equal(state.phase, 'STABLE');
  assert.equal(state.stableMs, 0);
  for (let at = 200; at < DWELL_MS; at += 200) state = reduceMachine(state, frame(at));
  assert.equal(state.phase, 'STABLE');
  state = reduceMachine(state, frame(1600));
  assert.equal(state.phase, 'CAPTURE_READY');
  state = reduceMachine(state, frame(1800));
  assert.equal(state.phase, 'SIMULATED_CAPTURE');
  assert.equal(state.source, 'live');
  assert.equal(state.ready, false);
});

test('timers alone and frames before preview readiness cannot advance dwell', () => {
  let state = reduceMachine(initialMachine(), { type: 'START', generation: 1, at: 0, source: 'live' });
  state = reduceMachine(state, frame(100));
  assert.equal(state.samples, 0);
  state = reduceMachine(state, { type: 'READY', generation: 1, at: 200 });
  for (let at = 200; at < 5000; at += 200) state = reduceMachine(state, { type: 'TICK', generation: 1, at });
  assert.equal(state.stableMs, 0);
  assert.notEqual(state.phase, 'CAPTURE_READY');
});

test('dropout, zero and multiple faces reset accumulated dwell', () => {
  for (const count of [0, 2]) {
    let state = reduceMachine(ready(), frame(0));
    state = reduceMachine(state, frame(200));
    state = reduceMachine(state, frame(300, assessObservation(count, null, view), count));
    assert.equal(state.phase, 'FACE_GUIDANCE');
    assert.equal(state.samples, 0);
    assert.equal(state.stableSince, null);
    state = reduceMachine(state, frame(400));
    assert.equal(state.stableMs, 0);
  }
});

test('position, scale and pose drift are checked against dwell anchor, not adjacent frames', () => {
  for (const moved of [
    { ...candidate, box: { ...candidate.box, x: candidate.box.x + 9 } },
    { ...candidate, box: { ...candidate.box, width: candidate.box.width * 1.08 } },
    { ...candidate, yaw: 4 },
  ]) {
    let state = reduceMachine(ready(), frame(0));
    state = reduceMachine(state, frame(200, assessObservation(1, moved, view)));
    assert.equal(state.reason, 'MOVEMENT');
    assert.equal(state.stableMs, 0);
    assert.equal(state.anchor, null);
  }
});

test('callback gaps, stale heartbeat and clock rollback reset dwell instead of completing it', () => {
  let state = reduceMachine(ready(), frame(0));
  state = reduceMachine(state, frame(MAX_GAP_MS + 1));
  assert.equal(state.reason, 'CALLBACK_GAP');
  assert.equal(state.samples, 0);
  state = reduceMachine(state, frame(600));
  state = reduceMachine(state, { type: 'TICK', generation: 1, at: 600 + MAX_GAP_MS + 1 });
  assert.equal(state.reason, 'CALLBACK_GAP');
  assert.equal(state.faceCount, 0);
  state = reduceMachine(state, frame(1100));
  state = reduceMachine(state, frame(1000));
  assert.equal(state.reason, 'CLOCK_RESET');
  assert.equal(state.stableSince, null);
});

test('readiness is revoked if the very next frame is invalid', () => {
  let state = dwell(ready());
  assert.equal(state.phase, 'CAPTURE_READY');
  state = reduceMachine(state, frame(1800, assessObservation(0, null, view), 0));
  assert.equal(state.phase, 'FACE_GUIDANCE');
  assert.equal(state.reason, 'NO_FACE');
});

test('cancel/background and restart generations ignore late native frames and preview callbacks', () => {
  let state = reduceMachine(ready(), frame(0));
  state = reduceMachine(state, { type: 'CANCEL', generation: 2, at: 100, reason: 'BACKGROUND' });
  assert.equal(state.phase, 'CANCELLED');
  assert.strictEqual(reduceMachine(state, frame(200)), state);
  assert.strictEqual(reduceMachine(state, { type: 'READY', generation: 1, at: 200 }), state);
  assert.strictEqual(reduceMachine(state, { type: 'READY', generation: 2, at: 200 }), state);
  state = reduceMachine(state, { type: 'START', generation: 3, at: 300, source: 'live' });
  assert.strictEqual(reduceMachine(state, frame(400)), state);
  state = reduceMachine(state, { type: 'READY', generation: 3, at: 400 });
  assert.equal(reduceMachine(state, frame(500, good, 1, 3)).phase, 'STABLE');
});

test('permission/privacy/camera errors fail closed and explicit new start can retry', () => {
  for (const reason of ['PERMISSION_DENIED', 'PRIVACY_UNAVAILABLE', 'CAMERA_ERROR', 'DETECTOR_ERROR']) {
    let state = ready();
    state = reduceMachine(state, { type: 'ERROR', generation: 1, at: 100, reason });
    assert.equal(state.phase, 'ERROR');
    assert.equal(state.ready, false);
    assert.strictEqual(reduceMachine(state, frame(200)), state);
    state = reduceMachine(state, { type: 'START', generation: 2, at: 300, source: 'live' });
    assert.equal(state.phase, 'CAMERA_READY');
  }
});

test('layout change resets dwell; deterministic event fixture remains explicitly synthetic', () => {
  let state = reduceMachine(ready(), frame(0));
  state = reduceMachine(state, { type: 'RESET', generation: 1, at: 100, reason: 'LAYOUT_CHANGED' });
  assert.equal(state.anchor, null);
  assert.equal(state.reason, 'LAYOUT_CHANGED');
  state = stableFixtureEvents(2, 200, view).reduce(reduceMachine, state);
  assert.equal(state.phase, 'SIMULATED_CAPTURE');
  assert.equal(state.source, 'synthetic_fixture');
  assert.equal(state.stableMs, 1800);
});

test('one live terminal approval cannot trigger again for a duplicate or a later frame', () => {
  const gate = new CaptureController();
  gate.arm(1);
  const terminal = reduceMachine(dwell(ready()), frame(1800));
  assert.equal(gate.claimFrame('9007199254740993'), true);
  assert.equal(gate.approve(terminal, '9007199254740993'), true);
  assert.equal(gate.claimFrame('9007199254740993'), false);
  assert.equal(gate.claimFrame('9007199254740992'), false);
  assert.equal(gate.approve(terminal, '9007199254740993'), false);
  assert.equal(gate.claimFrame('9007199254740994'), true);
  assert.equal(gate.approve(terminal, '9007199254740994'), false);
  assert.equal(gate.state.triggerCount, 1);
  assert.equal(gate.state.frameId, '9007199254740993');
  assert.equal(gate.state.status, 'ENCODING');
});

test('logical synthetic approval and capture-ready alone cannot authorize a native candidate', () => {
  const gate = new CaptureController();
  gate.arm(1);
  assert.equal(gate.claimFrame('1'), true);
  assert.equal(gate.approve(dwell(ready()), '1'), false);
  const synthetic = stableFixtureEvents(1, 0, view).reduce(reduceMachine, initialMachine());
  assert.equal(synthetic.phase, 'SIMULATED_CAPTURE');
  assert.equal(gate.approve(synthetic, '1'), false);
  assert.equal(gate.state.triggerCount, 0);
});

const jpegMetadata = {
  generation: 1, frameId: '1', jpegBytes: 12000, width: 480, height: 640,
  encodeMs: 12, selectedAgeMs: 90, peakOwnedBytes: 2000000, exif: false, status: 'READY', expiresAtMs: 100000,
};
const consumedMetadata = { ...jpegMetadata, status: 'CONSUMED', consumed: true };

test('background, cancellation and restart refuse late readiness without mutating the new cycle', () => {
  const gate = new CaptureController();
  const terminal = reduceMachine(dwell(ready()), frame(1800));
  gate.arm(1);
  gate.claimFrame('1');
  gate.approve(terminal, '1');
  assert.equal(gate.ready(jpegMetadata, false), false);
  gate.invalidate();
  assert.equal(gate.ready(jpegMetadata, true), false);
  assert.equal(gate.state.result, null);
  gate.arm(3);
  const restarted = gate.state;
  assert.equal(gate.ready(jpegMetadata, true), false);
  gate.fail(1);
  assert.strictEqual(gate.state, restarted);
  assert.equal(gate.claimFrame('1'), false);
  assert.equal(gate.claimFrame('2'), true);
  assert.equal(gate.approve({ ...terminal, generation: 3 }, '2'), true);
  const result = { ...jpegMetadata, generation: 3, frameId: '2' };
  const receipt = { ...consumedMetadata, generation: 3, frameId: '2' };
  assert.equal(gate.ready(result, true), true);
  assert.equal(gate.state.status, 'READY');
  assert.equal(gate.complete(receipt, true), false);
  assert.equal(gate.consuming(3, '2'), true);
  assert.equal(gate.complete(receipt, false), false);
  assert.equal(gate.complete({ ...receipt, jpegBytes: receipt.jpegBytes + 1 }, true), false);
  assert.equal(gate.complete(receipt, true), true);
  assert.equal(gate.state.status, 'CONSUMED');
  assert.equal(gate.state.triggerCount, 1);
  assert.equal(gate.complete(receipt, true), false);
});

test('mismatched, stale-input or unsafe native metadata cannot advance guidance to READY', () => {
  const gate = new CaptureController();
  gate.arm(1);
  gate.claimFrame('1');
  gate.approve(reduceMachine(dwell(ready()), frame(1800)), '1');
  for (const result of [
    null, { ...jpegMetadata, generation: 2 }, { ...jpegMetadata, frameId: '2' },
    { ...jpegMetadata, selectedAgeMs: MAX_GAP_MS + 1 }, { ...jpegMetadata, exif: true },
    { ...jpegMetadata, jpegBytes: 4194305 }, { ...jpegMetadata, peakOwnedBytes: 16777217 },
    { ...jpegMetadata, expiresAtMs: 0 },
  ]) {
    assert.equal(gate.ready(result, true), false);
    assert.equal(gate.state.status, 'ENCODING');
    assert.equal(gate.state.result, null);
  }
  assert.equal(gate.ready(jpegMetadata, true), true);
  assert.equal(gate.state.status, 'READY');
});

test('native capture errors revoke logical terminal approval instead of leaving false success', () => {
  let state = reduceMachine(dwell(ready()), frame(1800));
  assert.equal(state.phase, 'SIMULATED_CAPTURE');
  state = reduceMachine(state, { type: 'ERROR', generation: 1, at: 1850, reason: 'CAPTURE_ERROR' });
  assert.equal(state.phase, 'ERROR');
  assert.equal(state.reason, 'CAPTURE_ERROR');
  assert.equal(state.ready, false);
  assert.strictEqual(reduceMachine(state, frame(2000)), state);
  state = reduceMachine(state, { type: 'START', generation: 2, at: 2200, source: 'live' });
  assert.equal(state.phase, 'CAMERA_READY');
  assert.equal(state.samples, 0);
});

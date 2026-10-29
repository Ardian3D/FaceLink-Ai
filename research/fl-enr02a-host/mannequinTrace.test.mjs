import assert from 'node:assert/strict';
import test from 'node:test';
import { MannequinTrace } from './templates/mannequinTrace.ts';

test('trace projects only allowed mannequin metadata, never native capabilities or payload fields', () => {
  const lines = [];
  const trace = new MannequinTrace(true, (line) => lines.push(line), () => 100);
  trace.record('FRAME', {
    generation: 17, frameId: '29', faceCount: 1,
    reason: 'TOO_FAR', handle: 'private-capture-capability', jpeg: 'private-payload',
    detectorBoundsSensor: { x: 0.3, y: 0.2, width: 0.2, height: 0.4, token: 'nested-capability' },
    preview: { width: 333, height: 487, path: 'private-image-path' },
  });
  const emitted = JSON.parse(lines[0]);
  assert.deepEqual(emitted.detectorBoundsSensor, { x: 0.3, y: 0.2, width: 0.2, height: 0.4 });
  assert.deepEqual(emitted.preview, { width: 333, height: 487 });
  assert.equal(lines.some((line) => /private-|nested-capability|handle|token|jpeg"|path/.test(line)), false);
});

test('geometry sampling is rate limited and transition capacity remains after geometry budget exhaustion', () => {
  const records = [];
  let time = 0;
  const trace = new MannequinTrace(true, (line) => records.push(JSON.parse(line)), () => time);
  for (time = 0; time < 200_000; time += 10) trace.record('FRAME', { generation: 1 });
  for (let i = 0; i < 100; i++) trace.record('JPEG_READY', { generation: 1, frameId: '2' });
  const frames = records.filter((record) => record.event === 'FRAME');
  assert.equal(frames.length, 90);
  assert.equal(records.filter((record) => record.event === 'JPEG_READY').length, 64);
  for (let i = 1; i < frames.length; i++) assert.ok(frames[i].atMs - frames[i - 1].atMs >= 1000);
});

test('non-finite geometry and unsafe identifiers/error text are omitted instead of serialized', () => {
  let record;
  const trace = new MannequinTrace(true, (line) => { record = JSON.parse(line); }, () => 0);
  trace.record('ERROR', {
    generation: NaN, frameId: 'secret-session-token', reason: 'error containing private input',
    dwellMs: Infinity, previewBoxNormalized: { x: NaN, y: 0, width: 1, height: 1 },
  });
  assert.deepEqual(record, { event: 'ERROR', atMs: 0 });
});

test('disabled trace cannot publish geometry or transitions', () => {
  const trace = new MannequinTrace(false, () => assert.fail('disabled trace emitted'), () => 0);
  assert.equal(trace.wantsFrame(), false);
  trace.record('FRAME', { generation: 1, faceCount: 1 });
  trace.record('CAPTURE_READY', { generation: 1, frameId: '2' });
});

test('repeated movement resets cannot consume the terminal capture evidence budget', () => {
  const records = [];
  const trace = new MannequinTrace(true, (line) => records.push(JSON.parse(line)), () => 0);
  for (let i = 0; i < 500; i++) trace.record('RESET', { generation: 1, reason: 'MOVEMENT' });
  trace.record('JPEG_READY', { generation: 1, frameId: '8' });
  trace.record('CONSUMED', { generation: 1, frameId: '8' });
  trace.record('RELEASED', { generation: 1, frameId: '8', ownedBytes: 0, heldFrames: 0, busy: false });
  assert.ok(records.length <= 64);
  assert.deepEqual(records.slice(-3).map((record) => record.event), ['JPEG_READY', 'CONSUMED', 'RELEASED']);
});

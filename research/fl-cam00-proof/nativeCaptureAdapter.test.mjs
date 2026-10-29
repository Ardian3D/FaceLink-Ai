import test from 'node:test';
import assert from 'node:assert/strict';
import { CaptureAdapter } from './nativeCaptureAdapter.ts';

// This suite exercises JS capability/lifecycle rules only, never the Android codec.
const handle = 'synthetic-capability-not-a-production-token';
const metadata = {
  generation: 1, frameId: '1', width: 480, height: 640, jpegBytes: 12000, exif: false,
  status: 'READY', encodeMs: 12, selectedAgeMs: 90, peakOwnedBytes: 2000000, expiresAtMs: 100000,
};
const receipt = { ...metadata, status: 'CONSUMED', consumed: true };
function port(overrides = {}) {
  return {
    start: async () => {}, decide: async () => ({ ...metadata, handle }), consume: async () => receipt,
    cancel: async () => {}, dispose: async () => {}, getStatus: async () => ({}),
    runSyntheticVerification: async () => { throw new Error('Not Android'); },
    addListener() {}, removeListeners() {}, ...overrides,
  };
}
function deferred() {
  let resolve;
  const promise = new Promise((done) => { resolve = done; });
  return { promise, resolve };
}
const hasCode = (code) => (error) => error.code === code && error.message === code;


test('simultaneous JS consumers cannot claim the same native capability twice', async () => {
  const pending = deferred();
  let consumers = 0;
  const adapter = new CaptureAdapter(port({ consume: () => { consumers++; return pending.promise; } }));
  await adapter.start(1);
  const lease = await adapter.decide(1, '1', true, Array(8).fill(0));
  const first = lease.consumeOnce();
  await assert.rejects(lease.consumeOnce(), hasCode('E_CAPTURE_ALREADY_CLAIMED'));
  assert.equal(consumers, 1);
  pending.resolve(receipt);
  assert.equal((await first).status, 'CONSUMED');
  await assert.rejects(lease.consumeOnce(), hasCode('E_CAPTURE_ALREADY_CLAIMED'));
  assert.equal(consumers, 1);
});

test('cancellation revokes a READY capability before a later consumer can use it', async () => {
  let consumers = 0;
  const adapter = new CaptureAdapter(port({ consume: async () => { consumers++; return receipt; } }));
  await adapter.start(1);
  const lease = await adapter.decide(1, '1', true, null);
  await adapter.cancel();
  await assert.rejects(lease.consumeOnce(), hasCode('E_CAPTURE_ALREADY_CLAIMED'));
  assert.equal(consumers, 0);
});

test('late successful native reply after cancellation is not JS consumption success', async () => {
  const pending = deferred();
  const adapter = new CaptureAdapter(port({ consume: () => pending.promise }));
  await adapter.start(1);
  const lease = await adapter.decide(1, '1', true, null);
  const consuming = lease.consumeOnce();
  await adapter.cancel();
  pending.resolve(receipt);
  await assert.rejects(consuming, hasCode('E_CAPTURE_CANCELLED'));
  await assert.rejects(lease.consumeOnce(), hasCode('E_CAPTURE_ALREADY_CLAIMED'));
});

test('an old readiness reply cannot replace or revoke the next generation lease', async () => {
  const pending = deferred();
  const adapter = new CaptureAdapter(port({
    decide: async (generation) => generation === 1 ? pending.promise : { ...metadata, generation: 2, frameId: '2', handle },
    consume: async () => ({ ...receipt, generation: 2, frameId: '2' }),
  }));
  await adapter.start(1);
  const oldReady = adapter.decide(1, '1', true, null);
  await adapter.cancel();
  await adapter.start(2);
  const current = await adapter.decide(2, '2', true, null);
  pending.resolve({ ...metadata, handle });
  await assert.rejects(oldReady, hasCode('E_CAPTURE_CANCELLED'));
  assert.equal((await current.consumeOnce()).generation, 2);
});

test('a pending capture occupies the JS transaction before native readiness returns', async () => {
  const pending = deferred();
  const adapter = new CaptureAdapter(port({ decide: () => pending.promise }));
  await adapter.start(1);
  const first = adapter.decide(1, '1', true, null);
  await assert.rejects(adapter.decide(1, '2', true, null), hasCode('E_CAPTURE_BUSY'));
  pending.resolve({ ...metadata, handle });
  await first;
  await assert.rejects(adapter.decide(1, '2', true, null), hasCode('E_CAPTURE_BUSY'));
  await adapter.cancel();
});

test('native consumption failure does not leave a reusable local capability or leak error text', async () => {
  let attempts = 0;
  const adapter = new CaptureAdapter(port({ consume: async () => {
    attempts++;
    throw { code: 'E_CAPTURE_CONSUMER', message: handle };
  } }));
  await adapter.start(1);
  const lease = await adapter.decide(1, '1', true, null);
  await assert.rejects(lease.consumeOnce(), hasCode('E_CAPTURE_CONSUMER'));
  await assert.rejects(lease.consumeOnce(), hasCode('E_CAPTURE_ALREADY_CLAIMED'));
  assert.equal(attempts, 1);
});

test('a receipt for another frame cannot be accepted as successful consumption', async () => {
  const adapter = new CaptureAdapter(port({ consume: async () => ({ ...receipt, frameId: '2' }) }));
  await adapter.start(1);
  const lease = await adapter.decide(1, '1', true, null);
  await assert.rejects(lease.consumeOnce(), hasCode('E_CAPTURE_RESULT'));
  await assert.rejects(lease.consumeOnce(), hasCode('E_CAPTURE_ALREADY_CLAIMED'));
});

test('out-of-contract readiness cancels native ownership instead of exposing a lease', async () => {
  for (const invalid of [
    { ...metadata, jpegBytes: 4194305 }, { ...metadata, width: 2073601 },
    { ...metadata, selectedAgeMs: 401 }, { ...metadata, peakOwnedBytes: 16777217 },
    { ...metadata, exif: true }, { ...metadata, generation: 2 },
  ]) {
    let cancelled = false;
    const adapter = new CaptureAdapter(port({
      decide: async () => ({ ...invalid, handle }), cancel: async () => { cancelled = true; },
    }));
    await adapter.start(1);
    await assert.rejects(adapter.decide(1, '1', true, null), hasCode('E_CAPTURE_RESULT'));
    assert.equal(cancelled, true);
    await assert.rejects(adapter.decide(1, '1', true, null), hasCode('E_CAPTURE_STALE_GENERATION'));
  }
});

test('diagnostic serialization does not publish the opaque capability or extra native fields', async () => {
  const adapter = new CaptureAdapter(port({ decide: async () => ({ ...metadata, handle, unexpected: 'private-native-value' }) }));
  await adapter.start(1);
  const lease = await adapter.decide(1, '1', true, null);
  const serialized = JSON.stringify(lease);
  assert.equal(serialized.includes(handle), false);
  assert.equal(serialized.includes('private-native-value'), false);
  await adapter.cancel();
});

test('dispose is terminal for the facade and revokes its outstanding lease', async () => {
  const adapter = new CaptureAdapter(port());
  await adapter.start(1);
  const lease = await adapter.decide(1, '1', true, null);
  await adapter.dispose();
  await assert.rejects(lease.consumeOnce(), hasCode('E_CAPTURE_ALREADY_CLAIMED'));
  await assert.rejects(adapter.start(2), hasCode('E_CAPTURE_DISPOSED'));
  await assert.rejects(adapter.decide(1, '1', true, null), hasCode('E_CAPTURE_DISPOSED'));
});

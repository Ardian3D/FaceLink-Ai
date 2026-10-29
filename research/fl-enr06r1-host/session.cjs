'use strict';
// Synthetic-only acceptance host. No control API; local command file owns stdin.
const fs = require('node:fs');
const path = require('node:path');
const cp = require('node:child_process');
const https = require('node:https');
const crypto = require('node:crypto');
const ROOT = path.resolve(__dirname, '../..');
const run = process.env.FACELINK_R1_RUN || 'run4';
if (!/^[a-z0-9-]{1,40}$/.test(run)) throw Error('Invalid local acceptance run name');
const OUT = path.join(ROOT, 'research/fl-enr02a-host/evidence-local/enr06-r1', run);
fs.mkdirSync(OUT, { recursive: true });
const commands = path.join(OUT, 'commands.jsonl');
fs.writeFileSync(commands, '', { flag: 'wx' });
const record = (name, value) => fs.appendFileSync(path.join(OUT, name), JSON.stringify(value) + '\n');
const state = { offline: false, dropApproval: false, staleEtag: false };
const pending = [];
let lastChallenge = null;
let startup = null;
let exiting = false;
const ca = fs.readFileSync(path.join(ROOT, 'backend/dev/certs/dev-ca.pem'));
const backend = cp.spawn('go', ['run', './cmd/enrollment-staging-serve', '-integration-fixtures', '-addr', '127.0.0.1:8095', '-cv-python', path.join(ROOT, 'services/face-engine/.venv/Scripts/python.exe'), '-cv-engine-dir', path.join(ROOT, 'services/face-engine'), '-cv-model-dir', path.join(ROOT, 'services/face-engine/models')], { cwd: path.join(ROOT, 'backend'), windowsHide: true, stdio: ['pipe', 'pipe', 'pipe'] });
let buffer = '';
backend.stdout.on('data', data => {
  buffer += data;
  let n;
  while ((n = buffer.indexOf('\n')) >= 0) {
    const line = buffer.slice(0, n); buffer = buffer.slice(n + 1);
    if (!line.trim()) continue;
    const value = JSON.parse(line);
    if (value.mode) {
      startup = value;
      fs.writeFileSync(path.join(OUT, 'startup.json'), JSON.stringify(value, null, 2), { flag: 'wx' });
      console.log('R1_BACKEND_READY');
    } else if (value.fixture_control) {
      const resolve = pending.shift(); if (resolve) resolve(value);
    }
  }
});
backend.stderr.on('data', data => fs.appendFileSync(path.join(OUT, 'backend.err'), data));
backend.on('error', () => { console.error('BACKEND_SPAWN_FAILED'); shutdown(); });
const relay = https.createServer({ cert: fs.readFileSync(path.join(ROOT, 'backend/dev/certs/server.pem')), key: fs.readFileSync(path.join(ROOT, 'backend/dev/certs/server-key.pem')) }, (req, res) => {
  const route = req.url.split('?')[0];
  if (state.offline) { record('http.jsonl', { method: req.method, route, fault: 'relay-offline' }); req.resume(); res.destroy(); return; }
  const headers = { ...req.headers };
  const approval = route.endsWith('/approve');
  const requestHash = approval ? crypto.createHash('sha256') : null;
  if (requestHash) req.on('data', chunk => requestHash.update(chunk));
  let bodyHash;
  if (requestHash) req.on('end', () => { bodyHash = requestHash.digest('hex'); });
  const staleInjected = approval && state.staleEtag;
  if (staleInjected) { headers['if-match'] = '"0"'; state.staleEtag = false; }
  const upstream = https.request({ hostname: '127.0.0.1', port: 8095, path: req.url, method: req.method, headers, ca, rejectUnauthorized: true }, response => {
    const chunks = [];
    response.on('data', chunk => chunks.push(chunk));
    response.on('end', () => {
      const bytes = Buffer.concat(chunks);
      const safe = { method: req.method, route, status: response.statusCode };
      if (approval) {
        safe.body_sha256 = bodyHash;
        safe.idempotency_key_sha256 = crypto.createHash('sha256').update(String(headers['idempotency-key'] || '')).digest('hex');
        safe.if_match = /^"[0-9]{1,10}"$/.test(String(headers['if-match'])) ? headers['if-match'] : '[invalid revision]';
        safe.stale_etag_injected = staleInjected;
      }
      if (route.endsWith('/auth/email/request')) {
        try { lastChallenge = JSON.parse(bytes).data.challenge_id; } catch {}
      }
      if (!route.includes('/auth/') && response.headers['content-type']?.includes('application/json')) {
        try { const data = JSON.parse(bytes).data; safe.state = data?.effective_status ?? data?.current?.effective_status; safe.template_ready = data?.template_ready ?? data?.current?.template_ready; safe.submission_id = data?.submission_id ?? data?.current?.submission_id; } catch {}
      }
      if (route.endsWith('/photo') && response.statusCode === 200) safe.preview_sha256 = crypto.createHash('sha256').update(bytes).digest('hex');
      if (approval && state.dropApproval && response.statusCode === 202) {
        state.dropApproval = false; safe.fault = 'approval-response-dropped-after-upstream'; record('http.jsonl', safe); res.destroy(); return;
      }
      record('http.jsonl', safe);
      res.writeHead(response.statusCode, response.headers); res.end(bytes);
    });
  });
  upstream.setTimeout(20000, () => upstream.destroy());
  upstream.on('error', () => res.destroy()); req.pipe(upstream);
});
relay.listen(8094, '127.0.0.1', () => console.log('R1_RELAY_READY'));
const metro = cp.spawn('node', [path.join(ROOT, 'research/fl-enr02a-host/stage/node_modules/expo/bin/cli'), 'start', path.join(ROOT, 'research/fl-enr02a-host/stage'), '--dev-client', '--port', '8083'], { cwd: path.join(ROOT, 'research/fl-enr02a-host/stage'), env: { ...process.env, CI: '1', EXPO_NO_TELEMETRY: '1' }, windowsHide: true, stdio: ['ignore', 'pipe', 'pipe'] });
for (const stream of [metro.stdout, metro.stderr]) stream.on('data', data => fs.appendFileSync(path.join(OUT, 'metro.log'), data));
function control(value) {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error('control deadline')), 20000);
    pending.push(ack => { clearTimeout(timer); resolve(ack); });
    backend.stdin.write(JSON.stringify(value) + '\n');
  });
}
async function shutdown() {
  if (exiting) return;
  exiting = true; clearInterval(timer);
  relay.closeAllConnections(); relay.close();
  backend.stdin.end();
  if (metro.pid) cp.spawnSync('taskkill', ['/PID', String(metro.pid), '/T', '/F'], { windowsHide: true });
}
backend.on('exit', code => {
  record('exit.jsonl', { code, root_removed: startup ? !fs.existsSync(startup.root) : null });
  shutdown(); console.log('R1_BACKEND_EXIT ' + code); process.exitCode = code || 0;
});
let offset = 0;
let working = false;
let snapshotCount = 0;
const timer = setInterval(async () => {
  if (working || exiting) return;
  working = true;
  try {
    if (fs.statSync(commands).size > 65536) {
      record('acks.jsonl', { error: 'host control size limit; shutting down' });
      await shutdown();
      return;
    }
    const bytes = fs.readFileSync(commands); const remaining = bytes.subarray(offset).toString();
    const end = remaining.lastIndexOf('\n');
    if (end < 0) return;
    offset += Buffer.byteLength(remaining.slice(0, end + 1));
    for (const line of remaining.slice(0, end).split('\n').filter(Boolean)) {
      const c = JSON.parse(line); let data;
      if (c.command === 'shutdown') { await shutdown(); return; }
      if (c.command === 'snapshot' && ++snapshotCount > 64) throw Error('acceptance snapshot limit');
      if (c.command === 'offline') { state.offline = c.enabled === true; data = { offline: state.offline }; }
      else if (c.command === 'drop-approval') { state.dropApproval = true; data = { armed: true }; }
      else if (c.command === 'stale-etag') { state.staleEtag = true; data = { armed: true }; }
      else if (c.command === 'otp.latest') { if (!lastChallenge) throw Error('no challenge'); data = await control({ command: 'otp.wait', challenge_id: lastChallenge }); }
      else data = await control(c);
      record('acks.jsonl', { command: c.command, data });
    }
  } catch { record('acks.jsonl', { error: 'host control failed' }); }
  finally { working = false; }
}, 100);
process.on('SIGTERM', shutdown);
process.on('SIGINT', shutdown);
console.log('R1_HOST_STARTED');

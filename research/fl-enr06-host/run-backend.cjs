'use strict';

// FL-ENR06 disposable backend host. It owns the Go staging server's stdin so
// the Android UI can be driven separately while the harness keeps its
// stdin-only integration controls. No HTTP control route is added.

const { spawn } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');

const ROOT = path.resolve(__dirname, '../..');
const EVIDENCE = path.join(ROOT, 'research/fl-enr02a-host/evidence-local/enr06/device');
const PORT = 8091;
const COMMANDS = path.join(EVIDENCE, 'commands.jsonl');
const ACKS = path.join(EVIDENCE, 'acks.jsonl');
const LOG = path.join(EVIDENCE, 'backend.jsonl');
const ERR = path.join(EVIDENCE, 'backend.err');

fs.mkdirSync(EVIDENCE, { recursive: true });
fs.writeFileSync(COMMANDS, '');
fs.writeFileSync(ACKS, '');
fs.writeFileSync(LOG, '');
fs.writeFileSync(ERR, '');

const python = path.join(ROOT, 'services/face-engine/.venv', process.platform === 'win32' ? 'Scripts/python.exe' : 'bin/python');
const engine = path.join(ROOT, 'services/face-engine');
const models = path.join(engine, 'models');

const child = spawn('go', [
  'run', './cmd/enrollment-staging-serve',
  '-integration-fixtures',
  '-addr', `127.0.0.1:${PORT}`,
  '-cv-python', python,
  '-cv-engine-dir', engine,
  '-cv-model-dir', models,
], {
  cwd: path.join(ROOT, 'backend'),
  windowsHide: true,
  stdio: ['pipe', 'pipe', 'pipe'],
});

let pending = '';
child.stdout.on('data', (chunk) => {
  pending += chunk.toString('utf8');
  let index;
  while ((index = pending.indexOf('\n')) >= 0) {
    const line = pending.slice(0, index);
    pending = pending.slice(index + 1);
    if (line.trim() === '') continue;
    fs.appendFileSync(LOG, `${line}\n`);
    let parsed;
    try { parsed = JSON.parse(line); } catch { continue; }
    if (parsed && parsed.mode === 'FL-ENR05-synthetic-integration') {
      fs.writeFileSync(path.join(EVIDENCE, 'startup.json'), `${JSON.stringify(parsed, null, 2)}\n`);
      console.log(`BACKEND_READY ${parsed.base_url}`);
    } else if (parsed && parsed.fixture_control === true) {
      fs.appendFileSync(ACKS, `${line}\n`);
    }
  }
});
child.stderr.on('data', (chunk) => fs.appendFileSync(ERR, chunk));

let offset = 0;
const timer = setInterval(() => {
  let stat;
  try { stat = fs.statSync(COMMANDS); } catch { return; }
  if (stat.size <= offset) return;
  const fd = fs.openSync(COMMANDS, 'r');
  try {
    const size = stat.size - offset;
    const buffer = Buffer.alloc(size);
    fs.readSync(fd, buffer, 0, size, offset);
    offset = stat.size;
    for (const line of buffer.toString('utf8').split('\n')) {
      if (line.trim() === '') continue;
      if (line.includes('"shutdown"')) {
        child.stdin.end();
        continue;
      }
      child.stdin.write(`${line}\n`);
    }
  } finally {
    fs.closeSync(fd);
  }
}, 100);

child.on('exit', (code) => {
  clearInterval(timer);
  fs.writeFileSync(path.join(EVIDENCE, 'backend.exit'), `${code}\n`);
  console.log(`BACKEND_EXIT ${code}`);
  process.exit(0);
});
child.on('error', () => {
  console.log('BACKEND_SPAWN_FAILED');
  process.exit(1);
});

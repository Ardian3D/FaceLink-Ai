'use strict';

// Append one stdin-only integration control and wait for its sanitized ack.
// Usage: node ctl.cjs '{"command":"reconcile"}'

const fs = require('node:fs');
const path = require('node:path');

const EVIDENCE = path.resolve(__dirname, '..', 'fl-enr02a-host/evidence-local/enr06/device');
const COMMANDS = path.join(EVIDENCE, 'commands.jsonl');
const ACKS = path.join(EVIDENCE, 'acks.jsonl');
const raw = process.argv[2];
if (!raw) {
  console.error('missing control JSON');
  process.exit(2);
}
let command;
try { command = JSON.parse(raw); } catch {
  console.error('invalid control JSON');
  process.exit(2);
}
if (typeof command.command !== 'string' || command.command === '') {
  console.error('missing command name');
  process.exit(2);
}
const before = fs.existsSync(ACKS) ? fs.readFileSync(ACKS, 'utf8').split('\n').filter(Boolean).length : 0;
fs.appendFileSync(COMMANDS, `${JSON.stringify(command)}\n`);

const deadline = Date.now() + 60_000;
const timer = setInterval(() => {
  const lines = fs.existsSync(ACKS) ? fs.readFileSync(ACKS, 'utf8').split('\n').filter(Boolean) : [];
  if (lines.length <= before) {
    if (Date.now() >= deadline) {
      clearInterval(timer);
      console.error('control acknowledgement deadline');
      process.exit(1);
    }
    return;
  }
  const ack = JSON.parse(lines[lines.length - 1]);
  clearInterval(timer);
  process.stdout.write(`${JSON.stringify(ack)}\n`);
  process.exit(ack.ok === true ? 0 : 1);
}, 100);

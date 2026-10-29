'use strict';

// Minimal Android UI driver for the FL-ENR06 disposable acceptance.
// It uses only ADB read/install/input/UI-dump/screenshot operations already
// approved by the Product Owner. It never changes device settings.

const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');

const ROOT = path.resolve(__dirname, '../..');
const OUT = path.join(ROOT, 'research/fl-enr02a-host/evidence-local/enr06/device');
const PACKAGE = 'ai.facelink.enr02host';
fs.mkdirSync(OUT, { recursive: true });

function adb(args, options = {}) {
  const result = spawnSync('adb', args, { encoding: options.binary ? 'buffer' : 'utf8', maxBuffer: 64 * 1024 * 1024 });
  if (result.status !== 0) {
    const detail = options.binary ? '' : String(result.stderr || result.stdout || '').trim();
    throw new Error(`adb ${args[0]} failed${detail ? `: ${detail.slice(0, 200)}` : ''}`);
  }
  return result.stdout;
}

function dump() {
  adb(['shell', 'uiautomator', 'dump', '/data/local/tmp/enr06.xml']);
  const xml = adb(['exec-out', 'cat', '/data/local/tmp/enr06.xml']);
  return { xml, nodes: parse(xml) };
}

function parse(xml) {
  const nodes = [];
  for (const match of xml.matchAll(/<node [^>]*>/g)) {
    const tag = match[0];
    const bounds = /bounds="\[(\d+),(\d+)\]\[(\d+),(\d+)\]"/.exec(tag);
    nodes.push({
      text: /text="([^"]*)"/.exec(tag)?.[1] ?? '',
      desc: /content-desc="([^"]*)"/.exec(tag)?.[1] ?? '',
      cls: /class="([^"]*)"/.exec(tag)?.[1] ?? '',
      clickable: /clickable="true"/.test(tag),
      enabled: /enabled="true"/.test(tag),
      bounds: bounds ? [Number(bounds[1]), Number(bounds[2]), Number(bounds[3]), Number(bounds[4])] : null,
    });
  }
  return nodes;
}

function find(nodes, query) {
  const exact = nodes.filter((node) => node.text === query || node.desc === query);
  const loose = nodes.filter((node) => node.text.includes(query) || node.desc.includes(query));
  return exact.find((node) => node.clickable) ?? exact[0]
    ?? loose.find((node) => node.clickable) ?? loose[0];
}

function center(node) {
  if (!node || !node.bounds) throw new Error('node has no bounds');
  const [x1, y1, x2, y2] = node.bounds;
  return [Math.round((x1 + x2) / 2), Math.round((y1 + y2) / 2)];
}

function tapAt(x, y) {
  adb(['shell', 'input', 'tap', String(x), String(y)]);
}

function tap(query, label = query) {
  const { nodes } = dump();
  const node = find(nodes, query);
  if (!node) throw new Error(`UI element not found: ${label}`);
  const [x, y] = center(node);
  tapAt(x, y);
  return { text: node.text || node.desc, x, y };
}

function type(text) {
  adb(['shell', 'input', 'text', String(text).replace(/ /g, '%s')]);
}

function key(code) {
  adb(['shell', 'input', 'keyevent', code]);
}

function wait(query, seconds = 30) {
  const deadline = Date.now() + seconds * 1000;
  while (Date.now() < deadline) {
    const { nodes } = dump();
    if (find(nodes, query)) return true;
    adb(['shell', 'sleep', '1']);
  }
  throw new Error(`UI wait timeout: ${query}`);
}

function screenshot(name) {
  const bytes = adb(['exec-out', 'screencap', '-p'], { binary: true });
  if (bytes.length < 8 || bytes[0] !== 0x89 || bytes[1] !== 0x50) throw new Error('screenshot is not PNG');
  const file = path.join(OUT, `${name}.png`);
  fs.writeFileSync(file, bytes);
  return file;
}

function saveDump(name) {
  const { xml, nodes } = dump();
  const file = path.join(OUT, `${name}.xml`);
  fs.writeFileSync(file, xml);
  return { file, nodes };
}

function visible(query, name) {
  const { nodes } = saveDump(name);
  const node = find(nodes, query);
  return node ? { visible: true, text: node.text || node.desc, bounds: node.bounds } : { visible: false };
}

const [action, ...args] = process.argv.slice(2);
try {
  switch (action) {
    case 'tap': console.log(JSON.stringify(tap(args[0]))); break;
    case 'at': tapAt(Number(args[0]), Number(args[1])); break;
    case 'type': type(args[0]); break;
    case 'key': key(args[0]); break;
    case 'wait': console.log(JSON.stringify({ visible: wait(args[0], args[1] ? Number(args[1]) : 30) })); break;
    case 'sees': console.log(JSON.stringify(visible(args[0], args[1] || 'ui'))); break;
    case 'dump': {
      const { nodes } = saveDump(args[0] || 'ui');
      console.log(JSON.stringify(nodes.filter((n) => n.text || n.desc).map((n) => ({
        text: n.text, desc: n.desc, cls: n.cls.split('.').pop(), bounds: n.bounds, clickable: n.clickable, enabled: n.enabled,
      })), null, 2));
      break;
    }
    case 'shot': console.log(screenshot(args[0] || 'screen')); break;
    case 'pid': console.log(adb(['shell', 'pidof', PACKAGE]).trim()); break;
    case 'launch':
      adb(['shell', 'am', 'force-stop', PACKAGE]);
      adb(['shell', 'am', 'start', '-a', 'android.intent.action.VIEW', '-d', 'exp+facelink-enr02a://expo-development-client/?url=http%3A%2F%2F127.0.0.1%3A8081', PACKAGE]);
      break;
    default: throw new Error(`unknown action: ${action}`);
  }
} catch (error) {
  console.error(error.message);
  process.exitCode = 1;
}

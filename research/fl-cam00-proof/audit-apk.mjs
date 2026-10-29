import { readFileSync } from 'node:fs';
import { inflateRawSync } from 'node:zlib';
import { execFileSync } from 'node:child_process';
import { join } from 'node:path';
import { createHash } from 'node:crypto';

const apkPath = process.argv[2];
if (!apkPath || !process.env.ANDROID_HOME) throw new Error('Usage: ANDROID_HOME=<SDK> node audit-apk.mjs <release.apk>');
const tools = join(process.env.ANDROID_HOME, 'build-tools', '37.0.0');
const executable = (name) => join(tools, name + (process.platform === 'win32' ? '.exe' : ''));
const run = (name, args) => execFileSync(executable(name), args, { encoding: 'utf8', maxBuffer: 16 * 1024 * 1024 });
const badging = run('aapt2', ['dump', 'badging', apkPath]);
const manifest = run('aapt2', ['dump', 'xmltree', apkPath, '--file', 'AndroidManifest.xml']);
if (!badging.includes("package: name='ai.facelink.cam00proof'")) throw new Error('Unexpected package ID');
const permissions = [...badging.matchAll(/uses-permission: name='([^']+)'/g)].map((match) => match[1]);
const allowedPermissions = new Set([
  'android.permission.CAMERA',
  'ai.facelink.cam00proof.DYNAMIC_RECEIVER_NOT_EXPORTED_PERMISSION',
]);
const prohibited = permissions.filter((permission) => !allowedPermissions.has(permission));
if (prohibited.length) throw new Error('Unexpected release permissions: ' + prohibited.join(', '));
if (!permissions.includes('android.permission.CAMERA')) throw new Error('Missing CAMERA declaration');
for (const attribute of ['debuggable', 'usesCleartextTraffic', 'allowBackup']) {
  const line = manifest.split('\n').find((line) => line.includes(':' + attribute + '('));
  const value = line?.slice(line.indexOf('=') + 1).trim();
  if (value && !/^(?:false|\(type 0x12\)0x0+)$/.test(value)) throw new Error('Release enables or cannot resolve ' + attribute);
  if (!line && attribute !== 'debuggable') throw new Error('Missing explicit release policy: ' + attribute);
}
if (!/:screenOrientation[^\n]*=(?:1|\(type 0x10\)0x0*1)\s*(?:\n|$)/.test(manifest)) throw new Error('Portrait-only activity not found');
const apk = readFileSync(apkPath);
let end = -1;
for (let i = apk.length - 22; i >= Math.max(0, apk.length - 65557); i -= 1) {
  if (apk.readUInt32LE(i) === 0x06054b50 && i + 22 + apk.readUInt16LE(i + 20) === apk.length) { end = i; break; }
}
if (end < 0) throw new Error('Missing ZIP directory');
const count = apk.readUInt16LE(end + 10);
let position = apk.readUInt32LE(end + 16);
let libraries = 0;
let loadSegments = 0;
let embeddedBundleBytes = 0;
const abis = new Set();
for (let i = 0; i < count; i += 1) {
  if (apk.readUInt32LE(position) !== 0x02014b50) throw new Error('Invalid ZIP directory');
  const method = apk.readUInt16LE(position + 10);
  const size = apk.readUInt32LE(position + 20);
  const uncompressedSize = apk.readUInt32LE(position + 24);
  const nameLength = apk.readUInt16LE(position + 28);
  const extraLength = apk.readUInt16LE(position + 30);
  const commentLength = apk.readUInt16LE(position + 32);
  const local = apk.readUInt32LE(position + 42);
  const name = apk.subarray(position + 46, position + 46 + nameLength).toString();
  position += 46 + nameLength + extraLength + commentLength;
  if (name === 'assets/index.android.bundle') embeddedBundleBytes = uncompressedSize;
  if (!/^lib\/[^/]+\/[^/]+\.so$/.test(name)) continue;
  abis.add(name.split('/')[1]);
  libraries += 1;
  const start = local + 30 + apk.readUInt16LE(local + 26) + apk.readUInt16LE(local + 28);
  const packed = apk.subarray(start, start + size);
  if (method !== 0 && method !== 8) throw new Error('Unsupported compression ' + name);
  const elf = method === 0 ? packed : inflateRawSync(packed);
  if (elf.readUInt32LE(0) !== 0x464c457f || elf[4] !== 2 || elf[5] !== 1 || elf.readUInt16LE(18) !== 183) throw new Error('Not ARM64 ELF: ' + name);
  const offset = Number(elf.readBigUInt64LE(32));
  const stride = elf.readUInt16LE(54);
  const segments = elf.readUInt16LE(56);
  for (let j = 0; j < segments; j += 1) {
    const p = offset + j * stride;
    if (elf.readUInt32LE(p) !== 1) continue;
    loadSegments += 1;
    const fileOffset = elf.readBigUInt64LE(p + 8);
    const virtualAddress = elf.readBigUInt64LE(p + 16);
    const alignment = elf.readBigUInt64LE(p + 48);
    if (alignment < 16384n || (fileOffset - virtualAddress) % 16384n !== 0n) throw new Error('16 KB ELF alignment failed: ' + name);
  }
}
if (abis.size !== 1 || !abis.has('arm64-v8a') || libraries === 0) throw new Error('APK is not ARM64-only');
if (embeddedBundleBytes === 0) throw new Error('Missing embedded JS bundle');
run('zipalign', ['-c', '-P', '16', '4', apkPath]);
console.log(JSON.stringify({ apk: apkPath, sha256: createHash('sha256').update(apk).digest('hex'), packageId: 'ai.facelink.cam00proof', permissions, abis: [...abis], libraries, loadSegments, embeddedBundleBytes, zipAlignment: 'PASS', elfAlignment: 'PASS', releaseManifest: 'PASS' }, null, 2));

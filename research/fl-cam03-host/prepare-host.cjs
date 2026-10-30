'use strict';

// Derive a disposable SOURCE overlay, never clean a build or invoke npm/prebuild.
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const ROOT = path.resolve(__dirname, '../..');
const SOURCE = path.join(ROOT, 'apps/mobile');
const STAGE = path.join(__dirname, 'stage');
const SENTINEL = '.fl-cam03-stage.json';
const IDENTITY = { owner: 'FL-CAM03', source: 'apps/mobile', applicationId: 'ai.facelink.cam03host' };
const CAMERA = {
  'react-native-vision-camera': '5.2.3',
  'react-native-vision-camera-face-detector': '2.1.0',
  'react-native-nitro-image': '0.15.2',
};
const LOCAL_PACKAGE = '../../../packages/native-capture-host';
const GENERATED_PATHS = ['package.json', 'package-lock.json', 'app.json', 'tsconfig.json', '.npmrc',
  'android/app/build.gradle', 'android/app/src/main/AndroidManifest.xml',
  'android/app/src/main/res/values/strings.xml', 'metro.config.js',
  'app/cam03-native-check.tsx', 'tests/cam03-host-regression.test.ts',
  'src/cam03/video-observer.ts', 'src/cam03/playback-metadata.ts',
  'tests/cam03-playback-metadata.test.ts'];
const EXCLUDED = /^(?:\.env(?:\..*)?|\.git|\.expo|node_modules|build|dist|coverage|\.gradle|\.kotlin|\.cxx|\.cache|cache|secrets?|credentials?|tokens?|private|backups?|.*[.~](?:bak|backup|old)|local\.properties|google-services\.json)$/i;
const PRIVATE_FILE = /(?:\.(?:apk|aab|jks|keystore|p12|pfx|key|hprof|log|bak|backup)|~)$/i;
const hash = (data) => crypto.createHash('sha256').update(data).digest('hex');
const json = (file) => JSON.parse(fs.readFileSync(file, 'utf8'));
const encode = (value) => `${JSON.stringify(value, null, 2)}\n`;
function fail(message) { throw new Error(`FL-CAM03 preparation: ${message}`); }
function publicApi(value) {
  let url;
  try { url = new URL(value); } catch { fail('set EXPO_PUBLIC_FACELINK_API_BASE_URL to a valid HTTPS API URL'); }
  if (url.protocol !== 'https:' || url.username || url.password || url.search || url.hash || /[?#\r\n]/.test(value)) {
    fail('public API URL must use HTTPS without credentials, query, or fragment');
  }
  return url.href.replace(/\/$/, '');
}
function readPublicApi() {
  // Never parse/log a full environment object, export other keys, or display values.
  if (process.env.EXPO_PUBLIC_FACELINK_API_BASE_URL) return publicApi(process.env.EXPO_PUBLIC_FACELINK_API_BASE_URL);
  const file = path.join(SOURCE, '.env');
  if (!fs.existsSync(file)) fail('missing public API prerequisite; set EXPO_PUBLIC_FACELINK_API_BASE_URL');
  const match = fs.readFileSync(file, 'utf8').match(/^\s*(?:export\s+)?EXPO_PUBLIC_FACELINK_API_BASE_URL\s*=\s*(.*?)\s*$/m);
  if (!match) fail('missing allowlisted public API setting');
  let value = match[1];
  if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))) value = value.slice(1, -1);
  return publicApi(value);
}
function assertRegular(file) {
  if (fs.lstatSync(file).isSymbolicLink()) fail(`refusing symbolic link: ${path.relative(ROOT, file)}`);
}
function listInputs(directory, prefix = '') {
  assertRegular(directory);
  const result = [];
  for (const entry of fs.readdirSync(directory, { withFileTypes: true }).sort((a, b) => a.name.localeCompare(b.name))) {
    if (EXCLUDED.test(entry.name) || PRIVATE_FILE.test(entry.name)) continue;
    const relative = prefix ? `${prefix}/${entry.name}` : entry.name;
    const full = path.join(directory, entry.name);
    if (entry.isSymbolicLink()) fail(`refusing input symbolic link: ${relative}`);
    if (entry.isDirectory()) result.push(...listInputs(full, relative));
    else if (entry.isFile()) {
      // Only the already-public development CA is eligible, never other PEM files.
      if (/\.pem$/i.test(entry.name) && relative !== 'android/app/src/debug/res/raw/dev_ca.pem') continue;
      result.push(relative);
    }
  }
  return result;
}
function guardedStage() {
  assertRegular(path.dirname(STAGE));
  if (!fs.existsSync(STAGE)) fs.mkdirSync(STAGE);
  assertRegular(STAGE);
  const marker = path.join(STAGE, SENTINEL);
  if (fs.existsSync(marker)) {
    assertRegular(marker);
    if (JSON.stringify(json(marker)) !== JSON.stringify(IDENTITY)) fail('stage sentinel mismatch; refusing writes');
  } else {
    if (fs.readdirSync(STAGE).length !== 0) fail('nonempty stage without ownership sentinel; refusing writes');
    fs.writeFileSync(marker, encode(IDENTITY), { flag: 'wx' });
  }
}
function output(relative, data) {
  const target = path.resolve(STAGE, relative);
  if (!target.startsWith(`${STAGE}${path.sep}`)) fail('output escapes stage');
  let cursor = STAGE;
  for (const part of path.relative(STAGE, target).split(path.sep).slice(0, -1)) {
    cursor = path.join(cursor, part);
    if (fs.existsSync(cursor)) assertRegular(cursor);
    else fs.mkdirSync(cursor);
  }
  if (fs.existsSync(target)) {
    assertRegular(target);
    if (fs.readFileSync(target).equals(Buffer.isBuffer(data) ? data : Buffer.from(data))) return;
  }
  fs.writeFileSync(target, data);
}
function replaceOnce(text, pattern, replacement, label) {
  const matches = text.match(new RegExp(pattern.source, `${pattern.flags.replace(/g/g, '')}g`));
  if (!matches || matches.length !== 1) fail(`main ${label} changed; review staging transform`);
  return text.replace(pattern, replacement);
}
function stageManifest(text) {
  text = replaceOnce(text, /android:name="\.MainApplication"/, 'android:name="ai.facelink.mobile.MainApplication"', 'application class');
  text = replaceOnce(text, /android:name="\.MainActivity"/, 'android:name="ai.facelink.mobile.MainActivity"', 'activity class');
  text = replaceOnce(text, /android:scheme="facelinkai"/, 'android:scheme="facelinkcam03"', 'scheme');
  text = replaceOnce(text, /android:scheme="exp\+facelink-ai"/, 'android:scheme="exp+facelink-cam03-host"', 'dev scheme');
  const forbidden = ['CAMERA', 'READ_EXTERNAL_STORAGE', 'WRITE_EXTERNAL_STORAGE', 'READ_MEDIA_IMAGES', 'READ_MEDIA_VIDEO', 'READ_MEDIA_AUDIO', 'READ_MEDIA_VISUAL_USER_SELECTED', 'SYSTEM_ALERT_WINDOW'];
  for (const permission of forbidden) {
    const name = `android.permission.${permission}`;
    text = text.replace(new RegExp(`^[ \\t]*<uses-permission\\b[^>]*android:name="${name.replace(/\./g, '\\.')}"[^>]*/>\\r?\\n?`, 'gm'), '');
  }
  // Manifest merger removals also defeat transitive camera/media declarations.
  const removals = forbidden.map((name) => `  <uses-permission android:name="android.permission.${name}" tools:node="remove"/>`).join('\n');
  text = text.replace('</manifest>', `${removals}\n</manifest>`);
  if (!text.includes('android.permission.INTERNET')) fail('main INTERNET permission missing');
  if (/networkSecurityConfig|usesCleartextTraffic="true"/.test(text)) fail('release manifest must not inherit a debug TLS policy');
  return text;
}
function prepare() {
  const api = readPublicApi();
  const required = ['app', 'src', 'assets', 'android', 'plugins', 'tests', 'scripts', 'package.json', 'package-lock.json', 'app.json', 'tsconfig.json', 'metro.config.js', 'vitest.config.mts', 'eslint.config.js'];
  for (const item of required) if (!fs.existsSync(path.join(SOURCE, item))) fail(`missing actual host prerequisite: ${item}`);
  const nativePath = path.join(ROOT, 'packages/native-capture-host/package.json');
  if (!fs.existsSync(nativePath)) fail('native host package is missing; complete packages/native-capture-host first');
  const native = json(nativePath);
  if (native.name !== 'facelink-native-capture-host') fail('native host package name does not match contract');
  const pkg = json(path.join(SOURCE, 'package.json'));
  const baseline = json(path.join(SOURCE, 'package-lock.json'));
  if (baseline.lockfileVersion !== 3) fail('expected actual host npm lockfile v3');
  const lock = structuredClone(baseline);
  const donor = json(path.join(ROOT, 'research/fl-cam00-proof/package-lock.json'));
  pkg.name = 'facelink-cam03-host';
  Object.assign(pkg.dependencies, CAMERA, { 'facelink-native-capture-host': `file:${LOCAL_PACKAGE}` });
  pkg.scripts['cam03:audit'] = 'node ../audit-host.cjs';
  lock.name = pkg.name;
  lock.packages[''].name = pkg.name;
  lock.packages[''].dependencies = pkg.dependencies;
  for (const [name, version] of Object.entries(CAMERA)) {
    const key = `node_modules/${name}`;
    if (lock.packages[key]) fail(`main already contains ${name}; staging dependency contract needs review`);
    const entry = donor.packages[key];
    if (!entry || entry.version !== version || entry.dependencies || !entry.integrity || !entry.resolved.startsWith('https://registry.npmjs.org/')) fail(`missing exact dependency-only lock record: ${name}`);
    lock.packages[key] = entry;
  }
  lock.packages['node_modules/facelink-native-capture-host'] = {
    version: native.version,
    resolved: `file:${LOCAL_PACKAGE}`,
    ...(native.peerDependencies ? { peerDependencies: native.peerDependencies } : {}),
  };
  const config = json(path.join(SOURCE, 'app.json'));
  config.expo.name = 'FaceLink CAM03 Host';
  config.expo.slug = 'facelink-cam03-host';
  config.expo.scheme = 'facelinkcam03';
  config.expo.android.package = IDENTITY.applicationId;
  config.expo.android.blockedPermissions = ['android.permission.CAMERA', 'android.permission.READ_EXTERNAL_STORAGE', 'android.permission.WRITE_EXTERNAL_STORAGE', 'android.permission.READ_MEDIA_IMAGES', 'android.permission.READ_MEDIA_VIDEO', 'android.permission.READ_MEDIA_AUDIO', 'android.permission.READ_MEDIA_VISUAL_USER_SELECTED', 'android.permission.SYSTEM_ALERT_WINDOW'];
  const inputs = [];
  for (const item of required) {
    const full = path.join(SOURCE, item);
    assertRegular(full);
    if (fs.statSync(full).isDirectory()) inputs.push(...listInputs(full, item));
    else inputs.push(item);
  }
  const planned = new Map();
  const manifest = [];
  for (const relative of inputs.sort()) {
    const data = fs.readFileSync(path.join(SOURCE, relative));
    manifest.push({ path: relative, bytes: data.length, sha256: hash(data), localVideo: /\.(?:mp4|mov|webm)$/i.test(relative) });
    planned.set(relative, data);
  }
  planned.set('package.json', encode(pkg));
  planned.set('package-lock.json', encode(lock));
  planned.set('app.json', encode(config));
  planned.set('.env', `EXPO_PUBLIC_FACELINK_API_BASE_URL=${api}\n`);
  planned.set('.npmrc', 'install-links=true\n');
  const tsconfig = json(path.join(SOURCE, 'tsconfig.json'));
  // install-links installs the package inside the stage, but its react-native
  // import still needs the actual declaration entry rather than runtime JS.
  tsconfig.compilerOptions = {
    ...tsconfig.compilerOptions,
    paths: {
      ...(tsconfig.compilerOptions?.paths ?? {}),
      'react-native': ['./node_modules/react-native/types_generated/index.d.ts'],
    },
  };
  planned.set('tsconfig.json', encode(tsconfig));
  planned.set('android/app/build.gradle', replaceOnce(planned.get('android/app/build.gradle').toString(), /applicationId 'ai\.facelink\.mobile'/, "applicationId 'ai.facelink.cam03host'", 'applicationId'));
  planned.set('android/app/src/main/AndroidManifest.xml', stageManifest(planned.get('android/app/src/main/AndroidManifest.xml').toString()));
  planned.set('android/app/src/main/res/values/strings.xml', replaceOnce(planned.get('android/app/src/main/res/values/strings.xml').toString(), /<string name="app_name">[^<]*<\/string>/, '<string name="app_name">FaceLink CAM03 Host</string>', 'native launcher label'));
  planned.set('app/cam03-native-check.tsx', fs.readFileSync(path.join(__dirname, 'templates/cam03-native-check.tsx')));
  planned.set('tests/cam03-host-regression.test.ts', fs.readFileSync(path.join(__dirname, 'templates/cam03-host-regression.test.ts')));
  planned.set('metro.config.js', `${planned.get('metro.config.js').toString()}${fs.readFileSync(path.join(__dirname, 'templates/cam03-metro-overlay.cjs'), 'utf8')}`);
  planned.set('src/cam03/video-observer.ts', fs.readFileSync(path.join(__dirname, 'templates/cam03-video-observer.ts')));
  planned.set('src/cam03/playback-metadata.ts', fs.readFileSync(path.join(__dirname, 'templates/cam03-playback-metadata.ts')));
  planned.set('tests/cam03-playback-metadata.test.ts', fs.readFileSync(path.join(__dirname, 'templates/cam03-playback-metadata.test.ts')));
  guardedStage();
  const previousManifest = path.join(STAGE, 'cam03-source-manifest.json');
  if (fs.existsSync(previousManifest)) {
    assertRegular(previousManifest);
    const currentPaths = new Set(manifest.map((file) => file.path));
    for (const file of json(previousManifest).files) {
      if (!currentPaths.has(file.path)) fail(`previously staged source no longer eligible: ${file.path}; review explicitly (no files were deleted)`);
    }
  }
  for (const [relative, data] of planned) output(relative, data);
  const generated = GENERATED_PATHS.map((file) => ({ path: file, sha256: hash(Buffer.from(planned.get(file))) }));
  output('cam03-source-manifest.json', encode({ schema: 1, source: 'apps/mobile', policy: 'metadata-only; local videos remain ignored; no environment or credential payloads', stagingInstrumentation: { origin: 'src/components/VideoBackground.tsx', originalSourcePreserved: true, overlay: 'Metro redirects only that origin expo-video import to src/cam03/video-observer.ts; actual SDK hook and VideoView retained', observation: 'bounded ready/playing/progress metadata only; no media sources, URI, paths, errors or frame payloads; no visual verification claim' }, files: manifest, generated }));
  output('cam03-lock-baseline.json', encode({ packages: baseline.packages }));
  // Do not delete stale source, build outputs, APKs, caches, or dependencies on rerun.
  console.log(`Prepared actual FaceLink host: ${manifest.length} source files; stage ignored; no install/build/device operation performed.`);
  console.log('Run audit-host.cjs after npm ci; use a stage-only debug keystore (main signing material is never copied).');
}
module.exports = { publicApi, stageManifest, CAMERA, GENERATED_PATHS, prepare };
if (require.main === module) {
  try { prepare(); } catch (error) { console.error(error.message); process.exitCode = 1; }
}

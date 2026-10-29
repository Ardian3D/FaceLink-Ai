'use strict';
// Parent-run offline audit. Reads source bytes for hashes only; never prints payloads.
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const ROOT = path.resolve(__dirname, '../..');
const STAGE = path.join(__dirname, 'stage');
const readJson = (name) => JSON.parse(fs.readFileSync(path.join(STAGE, name), 'utf8'));
function requireTrue(condition, message) { if (!condition) throw new Error(`FL-CAM03 audit: ${message}`); }
function sha(file) { return crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex'); }
function audit() {
  const manifest = readJson('cam03-source-manifest.json');
  const expectedGenerated = require('./prepare-host.cjs').GENERATED_PATHS;
  for (const file of manifest.files) {
    requireTrue(sha(path.join(ROOT, 'apps/mobile', file.path)) === file.sha256, `main source changed since preparation: ${file.path}`);
    if (/^(?:app|src|assets)\//.test(file.path)) requireTrue(sha(path.join(STAGE, file.path)) === file.sha256, `approved stage UI/auth/asset differs: ${file.path}`);
    if (file.path.startsWith('android/') && !expectedGenerated.includes(file.path)) {
      requireTrue(sha(path.join(STAGE, file.path)) === file.sha256, `stage Android copy differs from approved source: ${file.path}`);
    }
  }
  const lock = readJson('package-lock.json');
  const baseline = readJson('cam03-lock-baseline.json');
  for (const [key, entry] of Object.entries(baseline.packages)) {
    if (key === '') continue;
    requireTrue(JSON.stringify(lock.packages[key]) === JSON.stringify(entry), `existing main lock entry changed: ${key}`);
  }
  const pins = {
    expo: '58.0.13', 'react-native': '0.88.0', react: '19.3.0',
    'react-native-nitro-modules': '0.37.1', 'react-native-nitro-google-signin': '2.3.0',
    'expo-video': '58.0.11', 'expo-secure-store': '58.0.2', 'expo-dev-client': '58.0.14',
    'expo-router': '58.0.23', ...require('./prepare-host.cjs').CAMERA,
  };
  for (const [name, version] of Object.entries(pins)) {
    const keys = Object.keys(lock.packages).filter((key) => key === `node_modules/${name}` || key.endsWith(`/node_modules/${name}`));
    requireTrue(keys.length === 1, `duplicate or missing native dependency: ${name}`);
    requireTrue(lock.packages[keys[0]].version === version, `unexpected pinned version: ${name}`);
    const installed = path.join(STAGE, 'node_modules', name, 'package.json');
    requireTrue(fs.existsSync(installed), `npm ci prerequisite missing: ${name}`);
    requireTrue(JSON.parse(fs.readFileSync(installed, 'utf8')).version === version, `installed version mismatch: ${name}`);
  }
  const native = JSON.parse(fs.readFileSync(path.join(STAGE, 'node_modules/facelink-native-capture-host/package.json'), 'utf8'));
  requireTrue(native.name === 'facelink-native-capture-host', 'native host package missing');
  requireTrue(fs.realpathSync(path.join(STAGE, 'node_modules/facelink-native-capture-host')) === path.join(STAGE, 'node_modules/facelink-native-capture-host'), 'native host must be installed with install-links, not a symlink');
  requireTrue(fs.readFileSync(path.join(STAGE, '.npmrc'), 'utf8').trim() === 'install-links=true', 'install-links configuration missing');
  const tsconfig = readJson('tsconfig.json');
  requireTrue(tsconfig.compilerOptions.paths['react-native']?.[0] === './node_modules/react-native/types_generated/index.d.ts', 'native package TypeScript resolution missing');
  const envLines = fs.readFileSync(path.join(STAGE, '.env'), 'utf8').split(/\r?\n/).filter(Boolean);
  requireTrue(envLines.length === 1 && envLines[0].startsWith('EXPO_PUBLIC_FACELINK_API_BASE_URL='), 'staged env must contain only one public API setting');
  const envUrl = new URL(envLines[0].slice('EXPO_PUBLIC_FACELINK_API_BASE_URL='.length));
  requireTrue(envUrl.protocol === 'https:' && !envUrl.username && !envUrl.password && !envUrl.search && !envUrl.hash, 'staged env URL must be HTTPS without credentials, query, or fragment');
  requireTrue(Array.isArray(manifest.generated), 'generated manifest missing');
  const recorded = manifest.generated.map((item) => item.path).sort();
  requireTrue(JSON.stringify(recorded) === JSON.stringify([...expectedGenerated].sort()) && new Set(recorded).size === recorded.length, 'generated manifest does not exactly cover expected files');
  for (const file of expectedGenerated) {
    const item = manifest.generated.find((entry) => entry.path === file);
    requireTrue(item && sha(path.join(STAGE, file)) === item.sha256, `generated staging drift: ${file}`);
  }
  const templateCopies = [
    ['templates/cam03-native-check.tsx', 'app/cam03-native-check.tsx'],
    ['templates/cam03-host-regression.test.ts', 'tests/cam03-host-regression.test.ts'],
    ['templates/cam03-video-observer.ts', 'src/cam03/video-observer.ts'],
    ['templates/cam03-playback-metadata.ts', 'src/cam03/playback-metadata.ts'],
    ['templates/cam03-playback-metadata.test.ts', 'tests/cam03-playback-metadata.test.ts'],
  ];
  for (const [template, destination] of templateCopies) {
    requireTrue(sha(path.join(__dirname, template)) === sha(path.join(STAGE, destination)), `stage instrumentation differs from template: ${destination}`);
  }
  requireTrue(fs.readFileSync(path.join(STAGE, 'metro.config.js'), 'utf8').endsWith(fs.readFileSync(path.join(__dirname, 'templates/cam03-metro-overlay.cjs'), 'utf8')), 'stage Metro observer overlay differs from template');
  const config = readJson('app.json');
  requireTrue(config.expo.android.package === 'ai.facelink.cam03host' && config.expo.scheme === 'facelinkcam03', 'stage identity mismatch');
  const gradle = fs.readFileSync(path.join(STAGE, 'android/app/build.gradle'), 'utf8');
  requireTrue(gradle.includes("namespace 'ai.facelink.mobile'") && gradle.includes("applicationId 'ai.facelink.cam03host'"), 'Android namespace/applicationId mismatch');
  const mainManifest = fs.readFileSync(path.join(STAGE, 'android/app/src/main/AndroidManifest.xml'), 'utf8');
  requireTrue(mainManifest.includes('android.permission.INTERNET') && !/networkSecurityConfig|usesCleartextTraffic="true"/.test(mainManifest), 'release network policy regression');
  const forbiddenPermissions = ['CAMERA', 'READ_EXTERNAL_STORAGE', 'WRITE_EXTERNAL_STORAGE', 'READ_MEDIA_IMAGES', 'READ_MEDIA_VIDEO', 'READ_MEDIA_AUDIO', 'READ_MEDIA_VISUAL_USER_SELECTED', 'SYSTEM_ALERT_WINDOW'];
  for (const permission of forbiddenPermissions) {
    const name = `android.permission.${permission}`;
    const direct = mainManifest.split(/\r?\n/).filter((line) => line.includes(name) && !line.includes('tools:node="remove"'));
    requireTrue(direct.length === 0, `direct forbidden permission in stage main manifest: ${permission}`);
    requireTrue(mainManifest.split(/\r?\n/).some((line) => line.includes(name) && line.includes('tools:node="remove"')), `permission removal missing: ${permission}`);
  }
  const debugManifest = fs.readFileSync(path.join(STAGE, 'android/app/src/debug/AndroidManifest.xml'), 'utf8');
  requireTrue(!debugManifest.split(/\r?\n/).some((line) => line.includes('android.permission.CAMERA') && !line.includes('tools:node="remove"')), 'camera permission declared in debug stage');
  const nativeStrings = fs.readFileSync(path.join(STAGE, 'android/app/src/main/res/values/strings.xml'), 'utf8');
  requireTrue(nativeStrings.includes('<string name="app_name">FaceLink CAM03 Host</string>') && mainManifest.includes('android:label="@string/app_name"'), 'native launcher label is not isolated');
  requireTrue(mainManifest.includes('android:scheme="facelinkcam03"') && mainManifest.includes('android:scheme="exp+facelink-cam03-host"') && !/android:scheme="(?:facelinkai|exp\+facelink-ai)"/.test(mainManifest), 'native deep link identity collision');
  requireTrue(mainManifest.includes('android:name="ai.facelink.mobile.MainActivity"') && mainManifest.includes('android:name="ai.facelink.mobile.MainApplication"'), 'namespace-bound native entry classes changed');
  requireTrue(!/android:authorities="[^"]*ai\.facelink\.mobile/.test(mainManifest), 'hard-coded production provider authority collision');
  console.log(`FL-CAM03 audit PASSED: ${manifest.files.length} main inputs preserved; approved stage source/asset hashes preserved; existing lock entries exact; native pins and unique lock paths checked.`);
  console.log('Still required: npm ls --all, Expo/RN autolink inspection, final merged release manifest permissions/TLS, native build, actual-host Vitest/typecheck, synthetic native runtime. This audit is not a device or rendered-video claim.');
}
module.exports = { audit };
if (require.main === module) { try { audit(); } catch (error) { console.error(error.message); process.exitCode = 1; } }

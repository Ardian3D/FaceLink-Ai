'use strict';

// Derive a disposable FL-ENR02A staging app from the actual FaceLink mobile
// source. It never runs npm/prebuild, never touches the main Android package,
// and never copies local secrets or user data.
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');

const ROOT = path.resolve(__dirname, '../..');
const SOURCE = path.join(ROOT, 'apps/mobile');
const STAGE = path.join(__dirname, 'stage');
const SENTINEL = '.fl-enr02a-stage.json';
const IDENTITY = {
  owner: 'FL-ENR02A',
  source: 'apps/mobile',
  applicationId: 'ai.facelink.enr02host',
  scheme: 'facelinkenr02',
};
const EXCLUDED = /^(?:\.env(?:\..*)?|\.git|\.expo|node_modules|build|dist|coverage|\.gradle|\.kotlin|\.cxx|\.cache|cache|secrets?|credentials?|tokens?|private|backups?|.*[.~](?:bak|backup|old)|local\.properties|google-services\.json)$/i;
const PRIVATE_FILE = /(?:\.(?:apk|aab|jks|keystore|p12|pfx|key|hprof|log|bak|backup)|~)$/i;
const hash = (data) => crypto.createHash('sha256').update(data).digest('hex');
const encode = (value) => `${JSON.stringify(value, null, 2)}\n`;

const ENR06_FIXTURE = Object.freeze({
  path: 'research/fl-enr02a-host/evidence-local/enr04-r1/fixtures-v1-verified/procedural-mannequin-heightfield-v1-01.jpg',
  sha256: 'cf572a3ef381f4b52087c9dac062d0af4da6ca352b1e55742674a1c308e6cabe',
  minBytes: 1_001,
  maxBytes: 1_048_575,
});

function offlineFixtureEnabled(env, mode, adminReview) {
  const value = env.FACELINK_ENR06_FIXTURE;
  if (value === undefined || value === '0') return false;
  if (value !== '1') fail('FACELINK_ENR06_FIXTURE must be 0 or 1');
  if (mode !== 'synthetic_flow' || !adminReview) {
    fail('FACELINK_ENR06_FIXTURE requires synthetic_flow and Admin review');
  }
  return true;
}

function readPinnedFixture() {
  return verifyFixtureBytes(
    path.join(ROOT, ENR06_FIXTURE.path),
    ENR06_FIXTURE.sha256,
    ENR06_FIXTURE.minBytes,
    ENR06_FIXTURE.maxBytes
  );
}

/**
 * Verify one synthetic JPEG before it can enter the generated staging tree.
 * Symlinked path components, non-regular files, out-of-bound byte counts and
 * hash mismatch all fail closed. Only the fixed opt-in path is used in
 * production; the parameters exist so the guard itself can be tested.
 */
function verifyFixtureBytes(file, expectedSha256, minBytes, maxBytes) {
  if (!/^[0-9a-f]{64}$/.test(expectedSha256)) fail('offline fixture hash must be lowercase SHA-256');
  if (!Number.isSafeInteger(minBytes) || !Number.isSafeInteger(maxBytes) || minBytes < 1 || maxBytes > 1_048_576 || minBytes > maxBytes) {
    fail('offline fixture byte bounds are invalid');
  }
  const absolute = path.resolve(file);
  if (!fs.existsSync(absolute)) fail('offline fixture is missing');
  // Production passes a fixed file inside the repository, so every component
  // below the project root is checked. Test-only paths outside it are limited
  // to the file and its immediate parent to avoid inspecting unrelated system
  // junctions.
  const insideProject = absolute.startsWith(`${ROOT}${path.sep}`);
  const boundary = insideProject ? ROOT : path.dirname(absolute);
  const parts = path.relative(boundary, absolute).split(path.sep);
  let cursor = boundary;
  for (let index = 0; index < parts.length; index++) {
    cursor = path.join(cursor, parts[index]);
    const stat = fs.lstatSync(cursor);
    if (stat.isSymbolicLink()) fail('offline fixture symbolic links are forbidden');
    if (index < parts.length - 1 && !stat.isDirectory()) fail('offline fixture parent must be a directory');
    if (index === parts.length - 1 && !stat.isFile()) fail('offline fixture must be a regular file');
  }
  const fd = fs.openSync(absolute, fs.constants.O_RDONLY | (fs.constants.O_NOFOLLOW || 0));
  try {
    const stat = fs.fstatSync(fd);
    if (!stat.isFile()) fail('offline fixture must be a regular file');
    if (stat.size < minBytes || stat.size > maxBytes) {
      fail('offline fixture outside regular-file byte bounds');
    }
    const data = Buffer.alloc(stat.size + 1);
    let length = 0;
    while (length < data.length) {
      const count = fs.readSync(fd, data, length, data.length - length, null);
      if (count === 0) break;
      length += count;
    }
    const bytes = data.subarray(0, length);
    if (length !== stat.size || hash(bytes) !== expectedSha256) {
      fail('offline fixture pinned SHA-256 mismatch');
    }
    return bytes;
  } finally {
    fs.closeSync(fd);
  }
}

function stageOfflineFixture(source) {
  const bytes = readPinnedFixture();
  const header = [
    '/**',
    ' * FL-ENR06 generated staging fixture: the pinned procedural JPEG that',
    ' * already passed the real YuNet/SFace pipeline in FL-ENR04-R2.',
    ' * Synthetic-only; this generated tree is disposable and ignored by Git.',
    ` * Source SHA-256: ${ENR06_FIXTURE.sha256}`,
    ' */',
    '',
  ].join('\n');
  return {
    source: header + replaceOnce(source, /const FIXTURE_BASE64 = `[^`]*`;/, () => `const FIXTURE_BASE64 = \`${bytes.toString('base64')}\`;`, 'synthetic fixture constant'),
    provenance: {
      optIn: 'FACELINK_ENR06_FIXTURE=1',
      kind: 'wholly-procedural-mannequin-heightfield-v1',
      path: ENR06_FIXTURE.path,
      sha256: ENR06_FIXTURE.sha256,
      bytes: bytes.length,
      captureMode: 'synthetic_flow',
      adminReview: true,
      camera: false,
      downloads: false,
    },
  };
}

function fail(message) {
  throw new Error(`FL-ENR02A preparation: ${message}`);
}

function publicApi(value) {
  let url;
  try {
    url = new URL(value);
  } catch {
    fail('set EXPO_PUBLIC_FACELINK_API_BASE_URL to a valid HTTPS URL');
  }
  if (
    url.protocol !== 'https:' ||
    url.username ||
    url.password ||
    url.search ||
    url.hash ||
    !['127.0.0.1', 'localhost', '[::1]'].includes(url.hostname)
  ) {
    fail('staging API URL must be HTTPS loopback without credentials, query, or fragment');
  }
  return url.href.replace(/\/$/, '');
}

function readPublicApi() {
  if (process.env.EXPO_PUBLIC_FACELINK_API_BASE_URL) {
    return publicApi(process.env.EXPO_PUBLIC_FACELINK_API_BASE_URL);
  }
  const file = path.join(SOURCE, '.env');
  if (!fs.existsSync(file)) {
    fail('missing prerequisite; set EXPO_PUBLIC_FACELINK_API_BASE_URL');
  }
  const match = fs
    .readFileSync(file, 'utf8')
    .match(/^\s*(?:export\s+)?EXPO_PUBLIC_FACELINK_API_BASE_URL\s*=\s*(.*?)\s*$/m);
  if (!match) fail('missing allowlisted public API setting');
  let value = match[1];
  if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))) {
    value = value.slice(1, -1);
  }
  return publicApi(value);
}

function assertRegular(file) {
  if (fs.lstatSync(file).isSymbolicLink()) {
    fail(`refusing symbolic link: ${path.relative(ROOT, file)}`);
  }
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
    if (JSON.stringify(JSON.parse(fs.readFileSync(marker, 'utf8'))) !== JSON.stringify(IDENTITY)) {
      fail('stage sentinel mismatch; refusing writes');
    }
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

function stageManifest(text, allowCamera = false) {
  text = replaceOnce(text, /android:name="\.MainApplication"/, 'android:name="ai.facelink.mobile.MainApplication"', 'application class');
  text = replaceOnce(text, /android:name="\.MainActivity"/, 'android:name="ai.facelink.mobile.MainActivity"', 'activity class');
  text = replaceOnce(text, /android:scheme="facelinkai"/, 'android:scheme="facelinkenr02"', 'scheme');
  text = replaceOnce(text, /android:scheme="exp\+facelink-ai"/, 'android:scheme="exp+facelink-enr02a"', 'dev scheme');
  const forbidden = [
    ...(allowCamera ? [] : ['CAMERA']),
    'READ_EXTERNAL_STORAGE',
    'WRITE_EXTERNAL_STORAGE',
    'READ_MEDIA_IMAGES',
    'READ_MEDIA_VIDEO',
    'READ_MEDIA_AUDIO',
    'READ_MEDIA_VISUAL_USER_SELECTED',
    'SYSTEM_ALERT_WINDOW',
  ];
  for (const permission of forbidden) {
    const name = `android.permission.${permission}`;
    text = text.replace(
      new RegExp(`^[ \\t]*<uses-permission\\b[^>]*android:name="${name.replace(/\./g, '\\.')}"[^>]*/>\\r?\\n?`, 'gm'),
      ''
    );
  }
  const removals = forbidden
    .map((name) => `  <uses-permission android:name="android.permission.${name}" tools:node="remove"/>`)
    .join('\n');
  text = text.replace('</manifest>', `${removals}\n</manifest>`);
  if (!text.includes('android.permission.INTERNET')) fail('main INTERNET permission missing');
  if (allowCamera && !text.includes('android.permission.CAMERA')) {
    text = text.replace('</manifest>', '  <uses-permission android:name="android.permission.CAMERA"/>\n</manifest>');
  }
  if (/networkSecurityConfig|usesCleartextTraffic="true"/.test(text)) fail('release manifest must not inherit debug TLS policy');
  return text;
}

function stageVariantManifest(text, label) {
  const tanpaOverlay = text.replace(
    /^[ \t]*<uses-permission\b[^>]*android:name="android\.permission\.SYSTEM_ALERT_WINDOW"[^>]*\/>\r?\n?/gm,
    ''
  );
  if (tanpaOverlay.includes('android.permission.SYSTEM_ALERT_WINDOW')) {
    fail(`staging ${label} manifest still contains overlay permission`);
  }
  return tanpaOverlay;
}

function stageDevPlugin(source) {
  const caPattern = /const CA_SOURCE = path\.join\("backend", "dev", "certs", "dev-ca\.pem"\);/;
  if (!caPattern.test(source)) fail('with-dev-tls CA source changed; review staging transform');
  let hasil = source.replace(
    caPattern,
    'const CA_SOURCE = path.resolve(__dirname, "..", "..", "..", "..", "backend", "dev", "certs", "dev-ca.pem");'
  );
  const composePattern = /const caPath = path\.join\(projectRoot, "\.\.", "\.\.", CA_SOURCE\);/;
  if (!composePattern.test(hasil)) fail('with-dev-tls CA composition changed; review staging transform');
  hasil = hasil.replace(composePattern, 'const caPath = CA_SOURCE;');
  hasil = hasil.replace(
    /^[ \t]*<uses-permission\b[^>]*android:name="android\.permission\.SYSTEM_ALERT_WINDOW"[^>]*\/>\r?\n?/gm,
    ''
  );
  if (hasil.includes('android.permission.SYSTEM_ALERT_WINDOW')) {
    fail('staging dev plugin still writes overlay permission');
  }
  return hasil;
}

function stageMainActivity(text) {
  if (text.includes('FL-ENR02B secure activity')) return text;
  const anchor = '    super.onCreate(null)';
  if (text.split(anchor).length !== 2) fail('MainActivity onCreate anchor changed');
  return text.replace(
    anchor,
    `    // FL-ENR02B secure activity: preview cannot enter recents before native start.
    window.addFlags(android.view.WindowManager.LayoutParams.FLAG_SECURE)
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
      setRecentsScreenshotEnabled(false)
    }
${anchor}`
  );
}

function prepare() {
  const api = readPublicApi();
  const required = [
    'app',
    'src',
    'assets',
    'android',
    'plugins',
    'tests',
    'scripts',
    'package.json',
    'package-lock.json',
    'app.json',
    'tsconfig.json',
    'metro.config.js',
    'vitest.config.mts',
    'eslint.config.js',
  ];
  for (const item of required) {
    if (!fs.existsSync(path.join(SOURCE, item))) fail(`missing actual host prerequisite: ${item}`);
  }
  const requestedMode = (process.env.EXPO_PUBLIC_FACELINK_CAPTURE_MODE ?? 'synthetic_flow').trim().toLowerCase();
  if (!['synthetic_flow', 'mannequin_preview_only'].includes(requestedMode)) {
    fail('EXPO_PUBLIC_FACELINK_CAPTURE_MODE must be synthetic_flow or mannequin_preview_only');
  }
  const mannequin = requestedMode === 'mannequin_preview_only';
  const adminReview = process.env.EXPO_PUBLIC_FACELINK_ADMIN_ENR_STAGING === '1';
  if (adminReview && mannequin) fail('Admin review requires synthetic_flow; no camera build in FL-ENR03');
  const offlineFixture = offlineFixtureEnabled(process.env, requestedMode, adminReview);
  const captureTrace = mannequin && process.env.EXPO_PUBLIC_FACELINK_CAPTURE_TRACE === '1';
  const captureProbes = captureTrace && process.env.EXPO_PUBLIC_FACELINK_CAPTURE_PROBES === '1';
  const pkg = JSON.parse(fs.readFileSync(path.join(SOURCE, 'package.json'), 'utf8'));
  const config = JSON.parse(fs.readFileSync(path.join(SOURCE, 'app.json'), 'utf8'));
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
    manifest.push({ path: relative, bytes: data.length, sha256: hash(data) });
    planned.set(relative, data);
  }
  const fixture = offlineFixture
    ? stageOfflineFixture(planned.get('src/features/enrollment/fixture.ts').toString())
    : null;
  if (fixture) planned.set('src/features/enrollment/fixture.ts', fixture.source);

  pkg.name = 'facelink-enr02a-staging';
  if (mannequin) {
    pkg.dependencies['facelink-native-capture-host'] = 'file:../../../packages/native-capture-host';
    pkg.dependencies['react-native-nitro-image'] = '0.15.2';
    pkg.dependencies['react-native-vision-camera'] = '5.2.3';
    pkg.dependencies['react-native-vision-camera-face-detector'] = '2.1.0';
  }
  pkg.scripts['enr02a:prepare'] = 'node ../prepare-enr02a.cjs';
  config.expo.name = 'FaceLink ENR02A Staging';
  config.expo.slug = 'facelink-enr02a-staging';
  config.expo.scheme = IDENTITY.scheme;
  config.expo.android.package = IDENTITY.applicationId;
  if (config.expo.ios) config.expo.ios.bundleIdentifier = IDENTITY.applicationId;
  if (mannequin) config.expo.android.permissions = ['android.permission.CAMERA'];
  config.expo.android.blockedPermissions = [
    ...(mannequin ? [] : ['android.permission.CAMERA']),
    'android.permission.READ_EXTERNAL_STORAGE',
    'android.permission.WRITE_EXTERNAL_STORAGE',
    'android.permission.READ_MEDIA_IMAGES',
    'android.permission.READ_MEDIA_VIDEO',
    'android.permission.READ_MEDIA_AUDIO',
    'android.permission.READ_MEDIA_VISUAL_USER_SELECTED',
    'android.permission.SYSTEM_ALERT_WINDOW',
  ];
  planned.set('package.json', encode(pkg));
  planned.set('app.json', encode(config));
  planned.set('.npmrc', 'install-links=true\n');
  planned.set(
    '.env',
    `EXPO_PUBLIC_FACELINK_API_BASE_URL=${api}\nEXPO_PUBLIC_FACELINK_ENR_STAGING=1\nEXPO_PUBLIC_FACELINK_ADMIN_ENR_STAGING=${adminReview ? '1' : '0'}\nEXPO_PUBLIC_FACELINK_CAPTURE_MODE=${requestedMode}\nEXPO_PUBLIC_FACELINK_CAPTURE_TRACE=${captureTrace ? '1' : '0'}\nEXPO_PUBLIC_FACELINK_CAPTURE_PROBES=${captureProbes ? '1' : '0'}\n`
  );
  planned.set(
    'android/app/build.gradle',
    replaceOnce(planned.get('android/app/build.gradle').toString(), /applicationId 'ai\.facelink\.mobile'/, "applicationId 'ai.facelink.enr02host'", 'applicationId')
  );
  planned.set(
    'android/app/src/main/AndroidManifest.xml',
    stageManifest(planned.get('android/app/src/main/AndroidManifest.xml').toString(), mannequin)
  );
  planned.set(
    'android/app/src/main/res/values/strings.xml',
    replaceOnce(planned.get('android/app/src/main/res/values/strings.xml').toString(), /<string name="app_name">[^<]*<\/string>/, '<string name="app_name">FaceLink ENR02A Staging</string>', 'native launcher label')
  );
  for (const variant of ['debug', 'debugOptimized']) {
    const relative = `android/app/src/${variant}/AndroidManifest.xml`;
    if (planned.has(relative)) {
      planned.set(relative, stageVariantManifest(planned.get(relative).toString(), variant));
    }
  }
  planned.set('plugins/with-dev-tls.js', stageDevPlugin(planned.get('plugins/with-dev-tls.js').toString()));
  planned.set('app/(app)/enrollment.tsx', fs.readFileSync(path.join(__dirname, 'templates/enrollment.tsx')));
  if (adminReview) {
    planned.set('app/(app)/admin-enrollment.tsx', fs.readFileSync(path.join(__dirname, 'templates/admin-enrollment.tsx')));
  }
  if (mannequin) {
    planned.set(
      'src/features/enrollment/nativeScanner.tsx',
      fs.readFileSync(path.join(__dirname, 'templates/nativeScanner.tsx'))
    );
    planned.set(
      'src/features/enrollment/mannequinTrace.ts',
      fs.readFileSync(path.join(__dirname, 'templates/mannequinTrace.ts'))
    );
  }
  if (mannequin) {
    planned.set(
      'android/app/src/main/java/ai/facelink/mobile/MainActivity.kt',
      stageMainActivity(planned.get('android/app/src/main/java/ai/facelink/mobile/MainActivity.kt').toString())
    );
  }

  guardedStage();
  if (!adminReview) {
    // Remove only this generator's optional route when returning to default-OFF.
    const route = path.join(STAGE, 'app/(app)/admin-enrollment.tsx');
    if (fs.existsSync(route)) { assertRegular(path.dirname(route)); assertRegular(route); fs.unlinkSync(route); }
  }
  for (const [relative, data] of planned) output(relative, data);
  if (mannequin) {
    const detectorBuild = path.join(
      STAGE,
      'node_modules/react-native-vision-camera-face-detector/android/build.gradle'
    );
    if (fs.existsSync(detectorBuild)) {
      const patched = fs.readFileSync(detectorBuild, 'utf8').includes('FL-ENR02B');
      if (!patched) {
        fail('mannequin mode requires node research/fl-enr02a-host/apply-detector-live.cjs after npm install');
      }
    } else {
      console.log('Mannequin prerequisites: npm install, then node ../apply-detector-live.cjs before build.');
    }
  }
  output(
    'enr02a-source-manifest.json',
    encode({
      schema: 1,
      source: 'apps/mobile',
      policy: 'disposable staging; no environment, credential, camera, gallery, or user data copied',
      applicationId: IDENTITY.applicationId,
      scheme: IDENTITY.scheme,
      offlineIntegrationFixture: fixture ? fixture.provenance : null,
      generated: [
        'package.json',
        'app.json',
        '.env',
        'android/app/build.gradle',
        'android/app/src/main/AndroidManifest.xml',
        'android/app/src/main/res/values/strings.xml',
        'android/app/src/debug/AndroidManifest.xml',
        'android/app/src/debugOptimized/AndroidManifest.xml',
        'plugins/with-dev-tls.js',
        'app/(app)/enrollment.tsx',
        ...(adminReview ? ['app/(app)/admin-enrollment.tsx'] : []),
        ...(fixture ? ['src/features/enrollment/fixture.ts'] : []),
        '.npmrc',
        ...(mannequin
          ? [
              'src/features/enrollment/nativeScanner.tsx',
              'src/features/enrollment/mannequinTrace.ts',
              'android/app/src/main/java/ai/facelink/mobile/MainActivity.kt',
            ]
          : []),
      ],
      files: manifest,
    })
  );
  console.log(`Prepared FL-ENR02A staging from actual FaceLink mobile: ${manifest.length} source files.`);
  console.log('Stage is ignored by Git; no install/build/device/camera operation performed.');
}

module.exports = { publicApi, stageManifest, stageMainActivity, IDENTITY, EXCLUDED, ENR06_FIXTURE, offlineFixtureEnabled, readPinnedFixture, stageOfflineFixture, verifyFixtureBytes };
if (require.main === module) {
  try {
    prepare();
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}

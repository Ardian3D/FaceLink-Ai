'use strict';

// FL-ENR02B: install the audited CAM02 live-bridge sources into the face detector that the
// disposable ENR02A staging app already installed (react-native-vision-camera-face-detector
// 2.1.0). The host package (facelink-native-capture-host) detects the "FL-ENR02B" marker in
// the patched detector and then compiles FaceLinkCaptureLive.
//
// Properties:
//  * no app modules are copied: unlike research/fl-cam00-proof/plugins/detector/patch-detector.cjs
//    this never writes Cam01PrivacyPackage.kt / FaceLinkCaptureModule.kt into an app source tree;
//  * no npm, Gradle, prebuild or device operation is performed;
//  * every target is validated against the CAM00 pinned hashes before the first write, so an
//    upstream drift fails closed instead of text-patching the wrong file;
//  * idempotent: an install that already carries the CAM02 patch is verified and left untouched.
//
// Usage:
//   node research/fl-enr02a-host/apply-detector-live.cjs [detector-dir]
// Overrides: FACELINK_DETECTOR_DIR (detector package) and FACELINK_APP_ANDROID_DIR (the
// consuming app's android project root, used to keep the detector source set relative).

const fs = require('node:fs/promises');
const path = require('node:path');
const { createHash } = require('node:crypto');
const {
  GRADLE_HASH,
  pinnedFiles,
  replaceUnique,
  patched,
} = require('../fl-cam00-proof/plugins/detector/patch-detector.cjs');

const DETECTOR_PACKAGE = 'react-native-vision-camera-face-detector';
const SUPPORTED_VERSION = '2.1.0';
const MARKER = 'FL-ENR02B';
const DEFAULT_DETECTOR_DIR = path.resolve(__dirname, 'stage', 'node_modules', DETECTOR_PACKAGE);
const CAM02_SOURCE = path.resolve(__dirname, '../fl-cam00-proof/plugins/detector/src/main/kotlin');
const CAM02_PACKAGE = 'com/margelo/nitro/camera/facedetector';
// The source set compiles the whole CAM00 kotlin directory, so these files must all be present.
const CAM02_SOURCE_FILES = [
  'Cam02FrameStore.kt',
  'NativeCaptureAdapter.kt',
  'Cam02Pixels.kt',
  'AndroidCaptureEncoder.kt',
  'NativeJpegValidationConsumer.kt',
  'CaptureSyntheticVerification.kt',
];
const NAMESPACE_ANCHOR = '  namespace "com.margelo.nitro.camera.facedetector"';
const RUNTIME_PREREQUISITES = [
  'react-native-vision-camera',
  'react-native-nitro-modules',
  'facelink-native-capture-host',
];

const sha256 = (value) => createHash('sha256').update(value).digest('hex');
const toPosix = (value) => value.split(path.sep).join('/');

function fail(code, detail) {
  throw new Error(detail ? `${code}: ${detail}` : code);
}

/** Gradle expression that resolves the CAM00 kotlin dir from the detector project. */
function sourceSetExpression(appAndroidDir) {
  const relative = path.relative(appAndroidDir, CAM02_SOURCE);
  if (!relative || path.isAbsolute(relative)) {
    // Different Windows drive (or an unrelated root): a normalized absolute path is the only
    // stable reference. The staging tree is disposable, so baking it in is acceptable.
    return JSON.stringify(toPosix(CAM02_SOURCE));
  }
  return `rootProject.file(${JSON.stringify(toPosix(relative))})`;
}

function liveGradle(text, expression) {
  const sourceSet = `sourceSets { main.java.srcDir(${expression}) }`;
  if (text.includes(MARKER)) {
    // Already installed: accept only the exact source set this app layout requires.
    if (!text.includes(sourceSet)) fail('CAM02_LIVE_PATCH_STALE_SOURCE_SET', 'reinstall the detector, then rerun');
    return text;
  }
  if (sha256(text) !== GRADLE_HASH) {
    fail('CAM02_LIVE_PATCH_UNSUPPORTED_SOURCE', 'detector android/build.gradle drifted; reinstall 2.1.0');
  }
  return replaceUnique(
    text,
    NAMESPACE_ANCHOR,
    `${NAMESPACE_ANCHOR}

  // ${MARKER}: CAM02 live-bridge sources stay owned by the isolated CAM00 prototype.
  ${sourceSet}`
  );
}

async function readOptional(filename) {
  try {
    return await fs.readFile(filename, 'utf8');
  } catch (error) {
    if (error.code === 'ENOENT') return null;
    throw error;
  }
}

async function verifyPrerequisites(appRoot) {
  const missing = [];
  for (const name of RUNTIME_PREREQUISITES) {
    const filename = path.join(appRoot, 'node_modules', name, 'package.json');
    if (await readOptional(filename) === null) missing.push(name);
  }
  if (missing.length > 0) {
    // Not fatal for the patch itself, but the live build cannot configure without these projects.
    console.warn(`warning: live build prerequisites not installed in ${appRoot}: ${missing.join(', ')}`);
  }
}

async function applyDetectorLive(options = {}) {
  const detectorDir = path.resolve(options.detectorDir || process.env.FACELINK_DETECTOR_DIR || DEFAULT_DETECTOR_DIR);
  const appRoot = path.dirname(path.dirname(detectorDir));
  const appAndroidDir = path.resolve(options.appAndroidDir || process.env.FACELINK_APP_ANDROID_DIR || path.join(appRoot, 'android'));

  const manifestFile = path.join(detectorDir, 'package.json');
  const manifest = JSON.parse(await fs.readFile(manifestFile, 'utf8'));
  if (manifest.name !== DETECTOR_PACKAGE) fail('CAM02_LIVE_PATCH_UNSUPPORTED_PACKAGE', manifest.name);
  if (manifest.version !== SUPPORTED_VERSION) fail('CAM02_LIVE_PATCH_UNSUPPORTED_VERSION', manifest.version);

  const corePackageDirectory = path.join(CAM02_SOURCE, CAM02_PACKAGE);
  for (const name of CAM02_SOURCE_FILES) {
    const filename = path.join(corePackageDirectory, name);
    if (await readOptional(filename) === null) fail('CAM02_LIVE_PATCH_MISSING_CORE', filename);
  }

  // Validate every target before making any write.
  const changes = [];
  for (const [relative, expectedHash, replacements] of pinnedFiles) {
    if (relative === 'android/build.gradle') continue;
    const filename = path.join(detectorDir, relative);
    const before = await fs.readFile(filename, 'utf8');
    changes.push({ filename, before, after: patched(before, expectedHash, replacements) });
  }
  const gradleFile = path.join(detectorDir, 'android/build.gradle');
  const gradleBefore = await fs.readFile(gradleFile, 'utf8');
  changes.push({ filename: gradleFile, before: gradleBefore, after: liveGradle(gradleBefore, sourceSetExpression(appAndroidDir)) });

  const written = [];
  for (const change of changes) {
    if (change.before !== change.after) {
      await fs.writeFile(change.filename, change.after);
      written.push(path.relative(detectorDir, change.filename));
    }
  }

  await verifyPrerequisites(appRoot);
  return {
    detector: path.relative(process.cwd(), detectorDir) || '.',
    appRoot: path.relative(process.cwd(), appRoot) || '.',
    sourceSet: sourceSetExpression(appAndroidDir),
    written,
    alreadyPatched: written.length === 0,
  };
}

module.exports = { applyDetectorLive, sourceSetExpression, liveGradle, CAM02_SOURCE, DEFAULT_DETECTOR_DIR };

if (require.main === module) {
  applyDetectorLive({ detectorDir: process.argv[2] })
    .then((result) => {
      console.log(JSON.stringify(result, null, 2));
      if (result.alreadyPatched) console.log('CAM02 live bridge already installed; nothing written.');
      else console.log('CAM02 live bridge installed into the staging detector. No npm/build performed.');
    })
    .catch((error) => {
      console.error(error.message);
      process.exitCode = 1;
    });
}

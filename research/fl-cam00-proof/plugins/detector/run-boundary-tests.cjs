'use strict';

// No Gradle daemon, Android SDK, downloads, or device needed. Uses cached compiler jars.
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

async function cached(group, name, version) {
  const home = process.env.GRADLE_USER_HOME || path.join(os.homedir(), '.gradle');
  const root = path.join(home, 'caches/modules-2/files-2.1', group, name, version);
  const filename = `${name}-${version}.jar`;
  for (const entry of await fs.readdir(root, { withFileTypes: true })) {
    if (!entry.isDirectory()) continue;
    const candidate = path.join(root, entry.name, filename);
    try { await fs.access(candidate); return candidate; } catch { }
  }
  throw new Error(`Missing offline dependency: ${group}:${name}:${version}`);
}
function run(java, args) {
  const result = spawnSync(java, args, { stdio: 'inherit' });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`Kotlin boundary command failed: ${result.status ?? result.signal}`);
}
async function main() {
  const dependencies = await Promise.all([
    cached('org.jetbrains.kotlin', 'kotlin-compiler-embeddable', '2.2.21'),
    cached('org.jetbrains.kotlin', 'kotlin-stdlib', '2.2.21'),
    cached('org.jetbrains.kotlin', 'kotlin-script-runtime', '2.2.21'),
    cached('org.jetbrains.kotlin', 'kotlin-reflect', '1.6.10'),
    cached('org.jetbrains.kotlin', 'kotlin-daemon-embeddable', '2.2.21'),
    cached('org.jetbrains.kotlinx', 'kotlinx-coroutines-core-jvm', '1.8.0'),
    cached('org.jetbrains', 'annotations', '13.0'),
  ]);
  const java = process.env.JAVA_HOME ? path.join(process.env.JAVA_HOME, 'bin', process.platform === 'win32' ? 'java.exe' : 'java') : 'java';
  const temporary = await fs.mkdtemp(path.join(os.tmpdir(), 'cam02-kotlin-boundaries-'));
  try {
    const classes = path.join(temporary, 'classes');
    run(java, ['-cp', dependencies.join(path.delimiter), 'org.jetbrains.kotlin.cli.jvm.K2JVMCompiler',
      '-no-stdlib', '-no-reflect', '-classpath', dependencies[1], '-d', classes,
      path.join(__dirname, 'src/main/kotlin/com/margelo/nitro/camera/facedetector/Cam02Pixels.kt'),
      path.join(__dirname, 'src/main/kotlin/com/margelo/nitro/camera/facedetector/NativeCaptureAdapter.kt'),
      path.join(__dirname, 'tests/Cam02PixelsTest.kt'),
      path.join(__dirname, 'tests/NativeCaptureAdapterTest.kt')]);
    run(java, ['-cp', [classes, dependencies[1]].join(path.delimiter), 'com.margelo.nitro.camera.facedetector.Cam02PixelsTestKt']);
    run(java, ['-cp', [classes, dependencies[1]].join(path.delimiter), 'com.margelo.nitro.camera.facedetector.NativeCaptureAdapterTest']);
  } finally { await fs.rm(temporary, { recursive: true, force: true }); }
}
main().catch(error => { console.error(error.message); process.exitCode = 1; });

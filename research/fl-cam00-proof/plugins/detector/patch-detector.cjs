'use strict';

// CAM02 is deliberately pinned to audited 2.1.0 source, not best-effort text patching.
const fs = require('node:fs/promises');
const path = require('node:path');
const { createHash } = require('node:crypto');
const cryptoHash = value => createHash('sha256').update(value).digest('hex');
const ANALYZER_HASH = '0b9ea60f9d7083c8779f226e6d8360aae6106f8faafd3396b5ee5673f5247589';
const GRADLE_HASH = '6c0f4caf3ac37e2b0fdda2952332fd28057886096ea861e646e9c8e633651e27';
const analyzerChanges = [
  ['import android.graphics.Matrix', 'import android.graphics.Matrix\nimport android.os.SystemClock'],
  ['import java.util.concurrent.atomic.AtomicReference\n', 'import java.util.concurrent.Executor\n'],
  ['  private val faceDetectorTask = AtomicReference<Task<List<Face>>?>()', `  // CAM02: a completed task is the sole ownership hand-off point.
  private val lifecycleLock = Any()
  @Volatile private var disposed = false
  private var processing = false
  private var detectorClosed = false
  private val directExecutor = Executor { it.run() }`],
  [`  override fun dispose() {
    faceDetectorTask.get()?.let {
      try {
        Tasks.await(it)
      } catch (_: Exception) {}
    }

    faceDetector.close()
    executor.close()
    super.dispose()
  }`, `  override fun dispose() {
    synchronized(lifecycleLock) {
      disposed = true
      try { imageAnalysis?.clearAnalyzer() } catch (_: Throwable) { }
      Cam02FrameStore.cancelSource(this)
      closeDetectorIfIdle()
    }
    // Never wait for ML Kit or join an executor from the UI/dispose thread.
    executor.shutdown()
    super.dispose()
  }

  private fun closeDetectorIfIdle() {
    if (disposed && !processing && !detectorClosed) {
      detectorClosed = true
      try { faceDetector.close() } catch (_: Throwable) { }
    }
  }

  private fun reportError(error: Throwable) {
    if (!disposed) try { options.onError(error) } catch (_: Throwable) { }
  }`],
  [`    if (!isBusy.compareAndSet(false, true)) {`, `    val acquiredAtMs = SystemClock.elapsedRealtime()
    val acquiredEpoch = Cam02FrameStore.analysisEpoch()
    if (disposed || !isBusy.compareAndSet(false, true)) {`],
  [`    try {
      val mediaImage`, `    val released = AtomicBoolean(false)
    fun releaseImage() {
      if (released.compareAndSet(false, true)) {
        try { imageProxy.close() } finally { isBusy.set(false) }
      }
    }
    var submitted: Task<List<Face>>? = null
    val completedOnce = AtomicBoolean(false)
    fun complete(task: Task<List<Face>>) {
      if (!completedOnce.compareAndSet(false, true)) return
      // Mark ML Kit idle BEFORE an event/decision can release busy and start the next task.
      synchronized(lifecycleLock) { processing = false; closeDetectorIfIdle() }
      var retained = false
      try {
        if (task.isSuccessful) {
          val hybridFaces = task.result.map { HybridFace(it, configForCam02!!) }.toTypedArray<HybridFaceSpec>()
          synchronized(lifecycleLock) {
            if (!disposed) retained = Cam02FrameStore.offer(this, acquiredEpoch, imageProxy, acquiredAtMs, hybridFaces, ::releaseImage)
          }
          if (!disposed) try { options.onFacesDetected(hybridFaces) } catch (_: Throwable) { }
        } else {
          reportError(IllegalStateException("E_CAM02_DETECTOR"))
        }
      } catch (_: Throwable) {
        reportError(IllegalStateException("E_CAM02_DETECTOR_CALLBACK"))
      } finally {
        if (!retained) try { releaseImage() } catch (_: Throwable) { }
      }
    }
    try {
      val mediaImage`],
  [`    var submitted: Task<List<Face>>? = null`, `    var configForCam02: FaceProcessConfig? = null
    var submitted: Task<List<Face>>? = null`],
  // Preserve the upstream six-space blank line without source trailing whitespace.
  [`      val task = faceDetector.process(inputImage)
      faceDetectorTask.set(task)
${"      "}
      task.addOnSuccessListener { faces ->
        val hybridFaces =
          faces
            .map { HybridFace(it, config) }
            .toTypedArray<HybridFaceSpec>()
        options.onFacesDetected(hybridFaces)
      }.addOnFailureListener { error ->
        options.onError(error)
      }.addOnCompleteListener {
        imageProxy.close()
        faceDetectorTask.set(null)
        isBusy.set(false)
      }`, `      configForCam02 = config
      val task = synchronized(lifecycleLock) {
        check(!disposed) { "E_CAM02_DISPOSED" }
        processing = true
        try { faceDetector.process(inputImage) }
        catch (error: Throwable) { processing = false; closeDetectorIfIdle(); throw error }
      }
      submitted = task
      // Explicit executor avoids Activity/main-thread listener lifetime or disposal ordering.
      task.addOnCompleteListener(directExecutor) { complete(it) }`],
  [`    } catch (error: Throwable) {
      imageProxy.close()
      isBusy.set(false)
      options.onError(error)
    }`, `    } catch (error: Throwable) {
      val task = submitted
      if (task == null) {
        try { releaseImage() } catch (_: Throwable) { }
        reportError(IllegalStateException("E_CAM02_DETECTOR_SETUP"))
      } else {
        // A registration failure must not close a proxy while ML Kit still owns it.
        // This dedicated analyzer thread may wait; dispose never waits/interrupts it.
        var interrupted = false
        while (!task.isComplete) {
          try { Tasks.await(task) }
          catch (_: InterruptedException) { interrupted = true }
          catch (_: java.util.concurrent.ExecutionException) { break }
        }
        complete(task)
        if (interrupted) Thread.currentThread().interrupt()
      }
    }`],
];
const gradleChanges = [[
  '  namespace "com.margelo.nitro.camera.facedetector"',
  `  namespace "com.margelo.nitro.camera.facedetector"

  // CAM02 native proof sources remain owned by the isolated prototype.
  sourceSets { main.java.srcDir(rootProject.file("../plugins/detector/src/main/kotlin")) }`,
]];

function replaceUnique(text, before, after) {
  if (text.split(before).length !== 2) throw new Error('CAM02_PATCH_ANCHOR_MISMATCH');
  return text.replace(before, after);
}
function patched(text, expectedHash, changes) {
  if (cryptoHash(text) === expectedHash) return changes.reduce((value, [before, after]) => replaceUnique(value, before, after), text);
  // Already-installed patches are accepted only after reversing exactly to the pinned upstream bytes.
  let original = text;
  try {
    for (const [before, after] of [...changes].reverse()) original = replaceUnique(original, after, before);
  } catch { throw new Error('CAM02_PATCH_UNSUPPORTED_SOURCE'); }
  if (cryptoHash(original) !== expectedHash) throw new Error('CAM02_PATCH_UNSUPPORTED_SOURCE');
  return text;
}

// Shared pin list. The FL-ENR02B staging script reuses the analyzer replacement and the
// hash-only pins, but supplies its own gradle source-set line instead of gradleChanges.
const pinnedFiles = [
  ['android/src/main/java/com/margelo/nitro/camera/facedetector/HybridFaceDetectorOutput.kt', ANALYZER_HASH, analyzerChanges],
  ['android/build.gradle', GRADLE_HASH, gradleChanges],
  ['android/src/main/java/com/margelo/nitro/camera/facedetector/HybridFace.kt', 'c8555bd2890ec8586f86b30061ee7fe2ba88db9c433730ff7eae6578e491da3d', []],
  ['android/src/main/java/com/margelo/nitro/camera/facedetector/MLKitCoordinateTransform.kt', 'e30e6c949b00479a95ae7ae81e531b977c550ae1eb32cf502263ed7bf1091ebb', []],
  ['nitrogen/generated/android/kotlin/com/margelo/nitro/camera/facedetector/HybridFaceSpec.kt', '809c5f8b376416a7c285f95732bce84279e576184fab3da3fbf0cd4541713e6e', []],
];

async function applyCam02(projectRoot) {
  const dependency = path.join(projectRoot, 'node_modules/react-native-vision-camera-face-detector');
  const version = JSON.parse(await fs.readFile(path.join(dependency, 'package.json'), 'utf8')).version;
  if (version !== '2.1.0') throw new Error('CAM02_PATCH_UNSUPPORTED_VERSION');
  const files = pinnedFiles;
  // Validate every target before making any write; upstream drift fails closed.
  const changes = [];
  for (const [relative, hash, replacements] of files) {
    const filename = path.join(dependency, relative);
    const before = await fs.readFile(filename, 'utf8');
    changes.push({ filename, before, after: patched(before, hash, replacements) });
  }
  for (const change of changes) if (change.before !== change.after) await fs.writeFile(change.filename, change.after);
  const javaRoot = path.join(projectRoot, 'android/app/src/main/java/ai/facelink/cam00proof');
  await fs.mkdir(javaRoot, { recursive: true });
  await fs.rm(path.join(javaRoot, 'Cam02CaptureModule.kt'), { force: true });
  for (const name of ['Cam01PrivacyPackage.kt', 'FaceLinkCaptureModule.kt']) {
    const source = await fs.readFile(path.join(projectRoot, 'plugins/android', name));
    const destination = path.join(javaRoot, name);
    const current = await fs.readFile(destination).catch(error => { if (error.code === 'ENOENT') return null; throw error; });
    if (!current || !source.equals(current)) await fs.writeFile(destination, source);
  }
}
module.exports = {
  applyCam02,
  ANALYZER_HASH,
  GRADLE_HASH,
  analyzerChanges,
  gradleChanges,
  pinnedFiles,
  replaceUnique,
  patched,
};
if (require.main === module) applyCam02(path.resolve(__dirname, '../..')).catch(error => {
  console.error(error.message);
  process.exitCode = 1;
});

package ai.facelink.capturehost

import android.Manifest
import android.content.pm.PackageManager
import android.net.TrafficStats
import android.os.FileObserver
import android.os.Process
import android.os.SystemClock
import android.system.Os
import android.system.OsConstants
import android.view.WindowManager
import com.facebook.react.bridge.Arguments
import com.facebook.react.bridge.LifecycleEventListener
import com.facebook.react.bridge.Promise
import com.facebook.react.bridge.ReactApplicationContext
import com.facebook.react.bridge.ReactContextBaseJavaModule
import com.facebook.react.bridge.ReactMethod
import com.facebook.react.bridge.ReadableArray
import com.facebook.react.bridge.WritableMap
import com.facebook.react.common.LifecycleState
import com.facebook.react.modules.core.DeviceEventManagerModule
import com.margelo.nitro.camera.facedetector.Cam02FrameStore
import com.margelo.nitro.camera.facedetector.CancelReason
import com.margelo.nitro.camera.facedetector.CaptureFailure
import com.margelo.nitro.camera.facedetector.ConsumeReceipt
import java.io.File
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicInteger

/**
 * FL-ENR02B live bridge, ported from the isolated CAM00 proof.
 *
 * Only scalar metadata and an opaque consume-once token cross React Native: no JPEG bytes,
 * base64 payloads, filesystem paths or camera handles are ever placed in a promise result or
 * an event. The native side owns foreground/FLAG_SECURE/permission checks and the frame store.
 * The synthetic FL-CAM03 verification API lives in [FaceLinkCaptureHostModule] and is untouched.
 */
class FaceLinkCaptureLiveModule(context: ReactApplicationContext) : ReactContextBaseJavaModule(context), LifecycleEventListener, Cam02FrameStore.Sink {
  private val lifecycleLock = Any()
  @Volatile private var foreground = context.lifecycleState == LifecycleState.RESUMED
  private var disposed = false
  private val tasks = Executors.newSingleThreadExecutor { task -> Thread(task, "FaceLink-live-consumer").apply { isDaemon = true } }
  private val listeners = AtomicInteger(0)
  private val watch = SandboxWatch(context)
  private var watchSession = 0L // Accessed only under lifecycleLock.

  init {
    context.addLifecycleEventListener(this)
    // The store accepts exactly one sink per process; a stale instance surfaces E_CAPTURE_UNAVAILABLE.
    Cam02FrameStore.attach(this)
  }

  override fun getName() = "FaceLinkCaptureLive"

  @ReactMethod
  fun addListener(@Suppress("UNUSED_PARAMETER") eventName: String) { listeners.incrementAndGet() }

  @ReactMethod
  fun removeListeners(count: Double) {
    if (count.isFinite() && count > 0) listeners.updateAndGet { maxOf(0, it - minOf(count, Int.MAX_VALUE.toDouble()).toInt()) }
  }

  private fun requireForeground(cameraPermission: Boolean) {
    val activity = reactApplicationContext.currentActivity
    if (!foreground || activity == null || activity.isFinishing || activity.isDestroyed) throw CaptureFailure("E_CAPTURE_FOREGROUND")
    if ((activity.window.attributes.flags and WindowManager.LayoutParams.FLAG_SECURE) == 0) throw CaptureFailure("E_CAPTURE_PRIVACY")
    if (cameraPermission && reactApplicationContext.checkSelfPermission(Manifest.permission.CAMERA) != PackageManager.PERMISSION_GRANTED) {
      throw CaptureFailure("E_CAPTURE_PERMISSION")
    }
    if (disposed) throw CaptureFailure("E_CAPTURE_DISPOSED")
  }

  @ReactMethod
  fun start(generation: Double, promise: Promise) {
    synchronized(lifecycleLock) {
      try {
        requireForeground(true)
        Cam02FrameStore.start(generation)
        watchSession++
        watch.start()
        promise.resolve(null)
      } catch (failure: CaptureFailure) { reject(promise, failure.code) }
        catch (_: Throwable) { Cam02FrameStore.cancel(); watch.stop(); reject(promise, "E_CAPTURE_START") }
    }
  }

  // React Native fixed arity: false decisions also pass null as argument four.
  @ReactMethod
  fun decide(generation: Double, frameId: String, capture: Boolean, sensorCorners: ReadableArray?, promise: Promise) {
    synchronized(lifecycleLock) {
      try {
        requireForeground(true)
        Cam02FrameStore.checkedGeneration(generation)
        val corners = sensorCorners?.let { array ->
          if (array.size() != 8) throw CaptureFailure("E_CAPTURE_CORNERS")
          DoubleArray(8) { array.getDouble(it) }
        }
        Cam02FrameStore.decide(generation, frameId, capture, corners) { result, error ->
          if (error != null) reject(promise, error)
          else if (result == null) promise.resolve(null)
          else promise.resolve(resultMap(result))
        }
      } catch (failure: CaptureFailure) {
        releaseAddressed(generation, frameId)
        reject(promise, failure.code)
      } catch (_: Throwable) {
        releaseAddressed(generation, frameId)
        reject(promise, "E_CAPTURE_ARGUMENTS")
      }
    }
  }

  private fun releaseAddressed(generation: Double, frameId: String) {
    try { Cam02FrameStore.decide(generation, frameId, false, null) { _, _ -> } } catch (_: Throwable) { }
  }

  @ReactMethod
  fun consume(generation: Double, handle: String, promise: Promise) {
    synchronized(lifecycleLock) {
      try {
        requireForeground(true)
        val captureGeneration = Cam02FrameStore.checkedGeneration(generation)
        if (handle.isEmpty() || handle.length > 256) throw CaptureFailure("E_CAPTURE_HANDLE")
        val observingSession = watchSession
        tasks.execute {
          try {
            synchronized(lifecycleLock) { requireForeground(true) }
            val receipt = Cam02FrameStore.consume(generation, handle)
            promise.resolve(receiptMap(receipt))
          } catch (failure: CaptureFailure) { reject(promise, failure.code) }
            catch (_: Throwable) { reject(promise, "E_CAPTURE_CONSUMER") }
          finally {
            synchronized(lifecycleLock) {
              val status = Cam02FrameStore.status()
              if (watchSession == observingSession && status.core.generation == captureGeneration &&
                !status.core.busy && !status.syntheticRunning) watch.stop()
            }
          }
        }
      } catch (failure: CaptureFailure) { reject(promise, failure.code) }
        catch (_: Throwable) { reject(promise, "E_CAPTURE_CONSUMER") }
    }
  }

  @ReactMethod
  fun cancel(promise: Promise) = synchronized(lifecycleLock) {
    Cam02FrameStore.cancel { promise.resolve(null) }
    watch.stop()
  }

  @ReactMethod
  fun dispose(promise: Promise) = synchronized(lifecycleLock) {
    disposed = true
    Cam02FrameStore.dispose { promise.resolve(null) }
    watch.stop()
  }

  @ReactMethod
  fun getStatus(promise: Promise) {
    try {
      val status = Cam02FrameStore.status()
      val core = status.core
      promise.resolve(Arguments.createMap().apply {
        val activeGeneration = Cam02FrameStore.activeGeneration()
        if (activeGeneration == null) putNull("generation") else putDouble("generation", activeGeneration.toDouble())
        putString("phase", core.phase); putInt("heldFrames", status.heldFrames)
        putDouble("ownedBytes", core.ownedBytes.toDouble()); putDouble("peakOwnedBytes", core.peakOwnedBytes.toDouble())
        putBoolean("busy", core.busy); putString("lastResult", core.lastError ?: core.phase)
        if (core.lastError == null) putNull("lastError") else putString("lastError", safeCode(core.lastError!!))
        val geometry = status.previewGeometry
        if (geometry == null) putNull("previewGeometry") else putMap("previewGeometry", Arguments.createMap().apply { for ((key, value) in geometry) putDouble(key, value) })
        putBoolean("foreground", foreground)
        putBoolean("syntheticRunning", status.syntheticRunning)
        putMap("caps", Arguments.createMap().apply {
          putInt("maxPixels", core.limits.maxPixels); putInt("maxJpegBytes", core.limits.maxJpegBytes); putInt("maxOwnedBytes", core.limits.maxOwnedBytes)
          putDouble("freshnessMs", core.limits.freshnessMs.toDouble()); putDouble("encodeTimeoutMs", core.limits.encodeTimeoutMs.toDouble())
          putDouble("readyTtlMs", core.limits.readyTtlMs.toDouble()); putDouble("consumeTimeoutMs", core.limits.consumeTimeoutMs.toDouble())
        })
        putMap("sandboxWatch", watch.snapshot())
        putMap("uidTraffic", Arguments.createMap().apply {
          val uid = Process.myUid()
          putInt("uid", uid)
          putDouble("txBytes", TrafficStats.getUidTxBytes(uid).toDouble()); putDouble("txPackets", TrafficStats.getUidTxPackets(uid).toDouble())
          putDouble("rxBytes", TrafficStats.getUidRxBytes(uid).toDouble()); putDouble("rxPackets", TrafficStats.getUidRxPackets(uid).toDouble())
          putString("scope", "uid_lifetime_including_historical_debug")
        })
      })
    } catch (_: Throwable) { reject(promise, "E_CAPTURE_STATUS") }
  }

  override fun onFrame(event: Cam02FrameStore.FrameEvent) {
    if (!foreground || listeners.get() <= 0 || !reactApplicationContext.hasActiveReactInstance()) throw CaptureFailure("E_CAPTURE_LISTENER")
    val faces = Arguments.createArray()
    for (face in event.faces) faces.pushMap(Arguments.createMap().apply {
      putMap("bounds", Arguments.createMap().apply {
        putDouble("x", face.x); putDouble("y", face.y); putDouble("width", face.width); putDouble("height", face.height)
      })
      putDouble("frameWidth", face.frameWidth); putDouble("frameHeight", face.frameHeight)
      putDouble("yawAngle", face.yawAngle); putDouble("pitchAngle", face.pitchAngle); putDouble("rollAngle", face.rollAngle)
    })
    reactApplicationContext.getJSModule(DeviceEventManagerModule.RCTDeviceEventEmitter::class.java).emit("CaptureFrame", Arguments.createMap().apply {
      putDouble("generation", event.generation); putString("frameId", event.frameId)
      putDouble("acquiredAtMs", event.acquiredAtMs.toDouble()); putDouble("emittedAtMs", event.emittedAtMs.toDouble()); putArray("faces", faces)
    })
  }

  override fun onHostResume() { synchronized(lifecycleLock) { foreground = true } }

  override fun onHostPause() {
    synchronized(lifecycleLock) {
      foreground = false
      Cam02FrameStore.cancel(CancelReason.BACKGROUND)
      watch.stop()
    }
  }

  override fun onHostDestroy() {
    synchronized(lifecycleLock) {
      foreground = false
      disposed = true
      // Host destruction ends this sink's ownership before React creates its replacement.
      // Identity-checked detach also makes a late invalidate harmless to a new owner.
      Cam02FrameStore.detach(this)
      watch.stop()
    }
  }

  override fun invalidate() {
    onHostDestroy()
    reactApplicationContext.removeLifecycleEventListener(this)
    tasks.shutdown()
    super.invalidate()
  }

  private fun safeCode(code: String): String = when {
    code.startsWith("E_CAPTURE_") && code.all { it in 'A'..'Z' || it in '0'..'9' || it == '_' } -> code
    code.startsWith("E_CAM02_") -> "E_CAPTURE_" + code.removePrefix("E_CAM02_")
    else -> "E_CAPTURE_NATIVE"
  }

  private fun reject(promise: Promise, code: String) { val safe = safeCode(code); promise.reject(safe, safe) }

  private fun resultMap(result: Cam02FrameStore.CaptureResult) = Arguments.createMap().apply {
    val ready = result.ready
    putDouble("generation", ready.generation.toDouble()); putString("frameId", ready.frameId); putString("handle", ready.handle)
    putInt("jpegBytes", ready.jpegBytes); putInt("width", ready.image.width); putInt("height", ready.image.height)
    putDouble("encodeMs", result.encodeMs.toDouble()); putDouble("selectedAgeMs", result.selectedAgeMs.toDouble())
    putDouble("peakOwnedBytes", ready.ownedPeakBytes.toDouble()); putDouble("expiresAtMs", ready.expiresAtMs.toDouble())
    putBoolean("exif", ready.image.exifPresent); putString("status", "READY")
    putBoolean("mirrored", result.mirrored); putString("cropMode", "CONTAINED_EVEN_INSET"); putDouble("cropInsetPixels", result.cropInsetPixels)
  }

  private fun receiptMap(receipt: ConsumeReceipt) = Arguments.createMap().apply {
    putDouble("generation", receipt.generation.toDouble()); putString("frameId", receipt.frameId)
    putInt("width", receipt.image.width); putInt("height", receipt.image.height); putInt("jpegBytes", receipt.jpegBytes)
    putBoolean("exif", receipt.image.exifPresent); putBoolean("consumed", receipt.consumed); putString("status", "CONSUMED")
  }
}

/** Metadata-only evidence, not a claim of complete write coverage or zero telemetry.
 * Never opens file contents, creates directories, deletes SDK cache, or follows symlinks.
 */
private class SandboxWatch(private val context: ReactApplicationContext) {
  private val lock = Any()
  private val root = File(context.applicationInfo.dataDir)
  private val names = setOf("files", "cache", "code_cache", "no_backup", "databases", "shared_prefs")
  private val observers = LinkedHashMap<String, FileObserver>()
  private val baseline = HashSet<String>()
  private var active = false
  private var watchEpoch = 0L
  private var creates = 0L
  private var writes = 0L
  private var deletes = 0L
  private var moves = 0L
  private var overflows = 0L
  private var errors = 0
  private var baselineTruncated = false
  private var directoriesTruncated = false
  private var startedAt = 0L
  private var stoppedAt = 0L
  private val mask = FileObserver.CREATE or FileObserver.CLOSE_WRITE or FileObserver.DELETE or FileObserver.MOVED_TO or FileObserver.MOVED_FROM or FileObserver.DELETE_SELF or FileObserver.MOVE_SELF

  fun start() = synchronized(lock) {
    stop()
    observers.clear(); baseline.clear()
    creates = 0; writes = 0; deletes = 0; moves = 0; overflows = 0; errors = 0
    baselineTruncated = false; directoriesTruncated = false
    stoppedAt = 0
    // Bound all enumeration/all retained names. Establish baseline before activating observation.
    scan(root, "", baselineOnly = true)
    startedAt = SystemClock.elapsedRealtime()
    active = true
    addObserver(root, "")
    for (name in names) scan(File(root, name), name, baselineOnly = false)
  }

  fun stop() = synchronized(lock) {
    active = false
    watchEpoch++
    for (observer in observers.values) observer.stopWatching()
    if (startedAt != 0L && stoppedAt == 0L) stoppedAt = SystemClock.elapsedRealtime()
  }

  private fun included(relative: String) = relative.substringBefore('/') in names

  private fun directory(file: File): Boolean = try {
    val metadata = Os.lstat(file.path)
    OsConstants.S_ISDIR(metadata.st_mode) && !OsConstants.S_ISLNK(metadata.st_mode)
  } catch (failure: android.system.ErrnoException) {
    if (failure.errno != OsConstants.ENOENT) errors++
    false
  }

  private fun scan(file: File, relative: String, baselineOnly: Boolean) {
    if (relative.length > 240) { baselineTruncated = true; directoriesTruncated = true; return }
    if (!directory(file)) return
    if (baselineOnly) {
      if (baseline.size >= 1024) { baselineTruncated = true; return }
      if (relative.isNotEmpty()) baseline.add(relative)
    } else {
      if (observers.size >= 128) { directoriesTruncated = true; return }
      addObserver(file, relative)
    }
    val children = file.listFiles() ?: run { errors++; return }
    for (child in children) {
      val path = if (relative.isEmpty()) child.name else "$relative/${child.name}"
      if (!included(path)) continue
      if (path.length > 240) { baselineTruncated = true; continue }
      if (baselineOnly) {
        if (baseline.size >= 1024) { baselineTruncated = true; return }
        baseline.add(path)
      }
      if (directory(child)) scan(child, path, baselineOnly)
    }
  }

  @Suppress("DEPRECATION")
  private fun addObserver(file: File, relative: String) {
    if (observers.containsKey(relative)) return
    if (observers.size >= 128) { directoriesTruncated = true; return }
    try {
      val observerEpoch = watchEpoch
      val observer = object : FileObserver(file.path, mask) {
        override fun onEvent(event: Int, child: String?) = synchronized(lock) {
          if (!active || observerEpoch != watchEpoch) return@synchronized
          if ((event and 0x4000) != 0) { overflows++; return@synchronized }
          val path = if (child == null) relative else if (relative.isEmpty()) child else "$relative/$child"
          if (!included(path)) return@synchronized
          when {
            (event and CREATE) != 0 -> creates++
            (event and CLOSE_WRITE) != 0 -> writes++
            (event and (DELETE or DELETE_SELF)) != 0 -> deletes++
            (event and (MOVED_TO or MOVED_FROM or MOVE_SELF)) != 0 -> moves++
            else -> return@synchronized
          }
          // Counters only: no sandbox path is ever retained or returned to JavaScript.
          if ((event and (DELETE or DELETE_SELF or MOVED_FROM or MOVE_SELF)) != 0) {
            val removed = observers.keys.filter { it == path || it.startsWith("$path/") }
            for (key in removed) observers.remove(key)?.stopWatching()
          }
          if (child != null && (event and (CREATE or MOVED_TO)) != 0 && (event and 0x40000000) != 0) scan(File(file, child), path, baselineOnly = false)
        }
      }
      observers[relative] = observer
      observer.startWatching()
    } catch (_: Throwable) { errors++ }
  }

  fun snapshot(): WritableMap = synchronized(lock) {
    Arguments.createMap().apply {
      putBoolean("active", active); putDouble("startedAtMs", startedAt.toDouble()); putDouble("stoppedAtMs", stoppedAt.toDouble())
      putDouble("createCount", creates.toDouble()); putDouble("closeWriteCount", writes.toDouble()); putDouble("deleteCount", deletes.toDouble()); putDouble("moveCount", moves.toDouble())
      putDouble("overflowCount", overflows.toDouble()); putInt("registrationErrors", errors)
      putInt("watchedDirectories", observers.size); putInt("baselineEntries", baseline.size)
      putBoolean("baselineTruncated", baselineTruncated); putBoolean("directoriesTruncated", directoriesTruncated)
      putString("coverage", "bounded_private_directory_events_not_forensic_guarantee")
    }
  }
}

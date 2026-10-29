package ai.facelink.capturehost

import com.facebook.react.bridge.Arguments
import com.facebook.react.bridge.LifecycleEventListener
import com.facebook.react.bridge.Promise
import com.facebook.react.bridge.ReactApplicationContext
import com.facebook.react.bridge.ReactContextBaseJavaModule
import com.facebook.react.bridge.ReactMethod
import com.facebook.react.common.LifecycleState
import java.util.concurrent.Executors

/** Synthetic-only legacy bridge. No camera ingress, frame events, storage writer or network client. */
class FaceLinkCaptureHostModule(context: ReactApplicationContext) : ReactContextBaseJavaModule(context), LifecycleEventListener {
  private val lock = Any()
  private var foreground = context.lifecycleState == LifecycleState.RESUMED
  private var disposed = false
  private var active: HostVerification? = null
  private var cancellation: String? = null
  private var lastError: String? = null
  private var inventoryBusy = false
  private var pendingRunDrains = 0
  private val acknowledgements = ArrayList<Promise>()
  private val inventoryWorker = Executors.newSingleThreadExecutor { task ->
    Thread(task, "FaceLink-host-storage-metadata").apply { isDaemon = true }
  }

  init { context.addLifecycleEventListener(this) }
  override fun getName() = "FaceLinkCaptureHost"

  private fun isForeground(): Boolean {
    val activity = reactApplicationContext.currentActivity
    return foreground && reactApplicationContext.lifecycleState == LifecycleState.RESUMED && activity != null &&
      !activity.isFinishing && !activity.isDestroyed
  }

  @ReactMethod
  fun runSyntheticVerification(promise: Promise) {
    var run: HostVerification? = null
    var error: String? = null
    synchronized(lock) {
      error = when {
        disposed -> "E_CAPTURE_HOST_DISPOSED"
        !isForeground() -> "E_CAPTURE_HOST_BACKGROUND"
        active != null -> "E_CAPTURE_HOST_BUSY"
        else -> null
      }
      if (error == null) {
        val candidate = try {
          HostVerificationCoordinator.reserve {
            HostVerification { completedRun, report, failure -> finishVerification(completedRun, promise, report, failure) }
          }
        } catch (_: Throwable) {
          error = "E_CAPTURE_HOST_WORKER"
          null
        }
        if (candidate != null) {
          active = candidate
          cancellation = null
          lastError = null
          run = candidate
        } else if (error == null) {
          error = "E_CAPTURE_HOST_BUSY"
        }
      }
    }
    if (error != null) { reject(promise, error!!); return }
    val claimed = run
    if (claimed == null) { reject(promise, "E_CAPTURE_HOST_WORKER"); return }
    try { HostVerificationCoordinator.execute(claimed) }
    catch (_: Throwable) { HostVerificationCoordinator.submissionFailed(claimed) }
  }

  private fun finishVerification(run: HostVerification, promise: Promise, report: HostReport?, failure: String?) {
    synchronized(lock) {
      val error = cancellation ?: failure ?: when {
        disposed -> "E_CAPTURE_HOST_DISPOSED"
        !isForeground() -> "E_CAPTURE_HOST_BACKGROUND"
        report == null -> "E_CAPTURE_HOST_VERIFICATION"
        else -> null
      }
      if (active === run) active = null
      lastError = error ?: if (report?.passed != true) "E_CAPTURE_HOST_ASSERTIONS" else null
      // Foreground/cancellation check and promise settlement share the lifecycle linearization lock.
      // This tiny metadata-only callback cannot race a pause/dispose into a late success.
      try {
        if (error != null) reject(promise, error)
        else promise.resolve(HostMetadata.report(report!!))
      } catch (_: Throwable) {
        lastError = "E_CAPTURE_HOST_METADATA"
        try { reject(promise, "E_CAPTURE_HOST_METADATA") } catch (_: Throwable) { }
      }
    }
    acknowledgeIfIdle()
  }

  @ReactMethod
  fun cancel(promise: Promise) { stop("E_CAPTURE_HOST_CANCELLED", false, promise, processWide = true) }

  @ReactMethod
  fun dispose(promise: Promise) { stop("E_CAPTURE_HOST_DISPOSED", true, promise, processWide = true) }

  private fun stop(code: String, terminal: Boolean, promise: Promise? = null, processWide: Boolean = false) {
    val run = synchronized(lock) {
      if (terminal) disposed = true
      if (active != null && cancellation == null) cancellation = code
      if (active != null) lastError = cancellation
      // Explicit bridge cancellation also covers an older ReactContext's still-draining reservation.
      // Lifecycle callbacks target only this instance, never an unrelated newly-created context.
      val selected = active ?: if (processWide) HostVerificationCoordinator.current() else null
      if (promise != null) {
        acknowledgements.add(promise)
        if (selected != null) pendingRunDrains++
      }
      selected
    }
    if (terminal) inventoryWorker.shutdown()
    // Nonblocking logical cancellation. ACK waits for the WHOLE run, not an idle primary adapter.
    run?.cancel(code)
    if (promise != null && run != null) run.whenDrained { physicallyDrained ->
      if (physicallyDrained) synchronized(lock) { pendingRunDrains-- }
      else {
        // The reservation remains held, but this caller must not wait forever on a stuck codec.
        synchronized(lock) { acknowledgements.remove(promise); pendingRunDrains-- }
        try { reject(promise, "E_CAPTURE_HOST_DRAIN_TIMEOUT") } catch (_: Throwable) { }
      }
      acknowledgeIfIdle()
    }
    acknowledgeIfIdle()
  }

  private fun acknowledgeIfIdle() {
    val completed = synchronized(lock) {
      if (active != null || inventoryBusy || pendingRunDrains != 0) return
      acknowledgements.toList().also { acknowledgements.clear() }
    }
    for (promise in completed) try { promise.resolve(null) } catch (_: Throwable) { }
  }

  @ReactMethod
  fun getStatus(promise: Promise) {
    try {
      val status = synchronized(lock) {
        val run = HostVerificationCoordinator.current()
        val known = run?.ownershipKnown() ?: true
        val core = run?.adapterStatus()
        Arguments.createMap().apply {
          putString("phase", when {
            disposed -> "DISPOSED"
            run?.draining == true -> "DRAINING"
            run != null -> "RUNNING"
            else -> "IDLE"
          })
          putBoolean("busy", run != null)
          putBoolean("foreground", isForeground())
          putBoolean("disposed", disposed)
          putBoolean("ownershipKnown", known)
          if (known) {
            putInt("ownedBytes", core?.ownedBytes ?: 0)
            putInt("ownedFrames", core?.ownedFrames ?: 0)
            putInt("peakOwnedBytes", core?.peakOwnedBytes ?: 0)
          } else {
            putNull("ownedBytes"); putNull("ownedFrames"); putNull("peakOwnedBytes")
          }
          if (core == null || !known) putNull("adapterPhase") else putString("adapterPhase", core.phase)
          putBoolean("suiteDrained", run?.suiteDrained() ?: true)
          val failure = run?.lastError() ?: lastError
          if (failure == null) putNull("lastError") else putString("lastError", failure)
          putMap("limits", HostMetadata.limits())
        }
      }
      promise.resolve(status)
    } catch (_: Throwable) { reject(promise, "E_CAPTURE_HOST_STATUS") }
  }

  @ReactMethod
  fun getStorageInventory(promise: Promise) {
    val error = synchronized(lock) {
      when {
        disposed -> "E_CAPTURE_HOST_DISPOSED"
        inventoryBusy -> "E_CAPTURE_HOST_INVENTORY_BUSY"
        else -> { inventoryBusy = true; null }
      }
    }
    if (error != null) { reject(promise, error); return }
    try {
      inventoryWorker.execute {
        try { promise.resolve(HostStorageInventory(reactApplicationContext).collect()) }
        catch (_: Throwable) { reject(promise, "E_CAPTURE_HOST_INVENTORY") }
        finally {
          synchronized(lock) { inventoryBusy = false }
          acknowledgeIfIdle()
        }
      }
    } catch (_: Throwable) {
      synchronized(lock) { inventoryBusy = false }
      reject(promise, "E_CAPTURE_HOST_INVENTORY")
      acknowledgeIfIdle()
    }
  }

  override fun onHostResume() { synchronized(lock) { foreground = true } }
  override fun onHostPause() {
    synchronized(lock) { foreground = false }
    stop("E_CAPTURE_HOST_BACKGROUND", false)
  }
  override fun onHostDestroy() {
    synchronized(lock) { foreground = false }
    stop("E_CAPTURE_HOST_DISPOSED", true)
  }
  override fun invalidate() {
    onHostDestroy()
    reactApplicationContext.removeLifecycleEventListener(this)
    super.invalidate()
  }

  private fun reject(promise: Promise, code: String) { promise.reject(code, code) }
}

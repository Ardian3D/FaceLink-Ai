package ai.facelink.capturehost

import com.margelo.nitro.camera.facedetector.CaptureFailure
import com.margelo.nitro.camera.facedetector.CaptureSyntheticVerification
import java.util.concurrent.ScheduledThreadPoolExecutor
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean

internal data class HostCase(val name: String, val observations: Map<String, Any?>, val assertions: Map<String, Boolean>) {
  val passed get() = assertions.isNotEmpty() && assertions.values.all { it }
}
internal data class HostReport(val cases: List<HostCase>) {
  val passed get() = cases.size == 13 && cases.all { it.passed }
}

/** Reservation includes the entire ENR suite, native consumer, and any cooperative physical drain. */
internal class HostVerification(private val completion: (HostVerification, HostReport?, String?) -> Unit) {
  private val lock = Any()
  private val cancelled = AtomicBoolean(false)
  private val suite = CaptureSyntheticVerification()
  private val identity = HostIdentityVerification(cancelled)
  private var error: String? = null
  private var report: HostReport? = null
  private var completed = false
  private val drainWaiters = ArrayList<(Boolean) -> Unit>()
  @Volatile var suiteExecuting = false
    private set
  @Volatile var taskReturned = false
    private set
  @Volatile var draining = false
    private set
  @Volatile var taskReturnedAt = 0L
    private set
  @Volatile var stuckNotified = false
    private set
  @Volatile private var stuckCode: String? = null

  fun cancel(code: String) {
    synchronized(lock) {
      if (completed) return
      if (error == null) error = code
      cancelled.set(true)
      draining = true
    }
    // Foreign/native callbacks are deliberately outside the host state lock.
    suite.cancelLifecycle()
    identity.cancel()
  }

  fun whenDrained(callback: (Boolean) -> Unit) {
    val immediate = synchronized(lock) {
      if (completed) true else { drainWaiters.add(callback); false }
    }
    if (immediate) callback(stuckCode == null)
  }
  fun lastError(): String? = synchronized(lock) { error ?: stuckCode }
  fun suiteDrained() = suite.isDrained()
  fun ownershipKnown() = !suiteExecuting && suite.isDrained()
  fun adapterStatus() = identity.status()

  /** Settle externally-visible callbacks once, while the process-wide reservation stays held. */
  fun failStuck() {
    synchronized(lock) {
      if (completed) return
      if (error == null) error = "E_CAPTURE_HOST_DRAIN_TIMEOUT"
      stuckCode = "E_CAPTURE_HOST_DRAIN_TIMEOUT"
      stuckNotified = true
    }
    complete()
  }

  private fun checkActive() {
    if (cancelled.get()) throw CaptureFailure("E_CAPTURE_HOST_CANCELLED")
  }

  fun perform() {
    try {
      checkActive()
      suiteExecuting = true
      val legacy = try { suite.run() } finally { suiteExecuting = false }
      val cases = legacy.cases.mapTo(ArrayList<HostCase>(13)) { HostCase(it.name, it.observations, it.assertions) }
      checkActive()
      if (!suite.isDrained()) {
        cases.add(HostCase("native_identity_content_two_generations", mapOf("boundary" to "prior_codec_cleanup_pending"),
          mapOf("priorCleanupComplete" to false)))
      } else {
        cases.add(identity.run())
      }
      checkActive()
      synchronized(lock) { if (!cancelled.get()) report = HostReport(cases) }
    } catch (_: Throwable) {
      synchronized(lock) { if (error == null) error = "E_CAPTURE_HOST_VERIFICATION" }
    } finally {
      // Terminal primary adapter, including cancellation before its first synthetic input exists.
      identity.cancel()
      draining = true
      taskReturnedAt = System.currentTimeMillis()
      taskReturned = true
    }
  }

  fun physicallyDrained(): Boolean {
    if (!taskReturned || suiteExecuting || !suite.isDrained() || !identity.isDrained()) return false
    // Shutdown is nonblocking; reservation persists until our real codec/deadline threads exit.
    return identity.closeDrainedExecutors()
  }

  fun complete() {
    val result: HostReport?
    val failure: String?
    val waiters: List<(Boolean) -> Unit>
    synchronized(lock) {
      if (completed) return
      completed = true
      result = report
      failure = error ?: if (result == null) "E_CAPTURE_HOST_VERIFICATION" else null
      waiters = drainWaiters.toList()
      drainWaiters.clear()
    }
    try { completion(this, result, failure) }
    finally { for (waiter in waiters) try { waiter(stuckCode == null) } catch (_: Throwable) { } }
  }
}

/** Process-wide, not ReactContext-wide: a recreated bridge cannot overlap old JNI cleanup. */
internal object HostVerificationCoordinator {
  private val lock = Any()
  private var active: HostVerification? = null
  private val worker = ScheduledThreadPoolExecutor(1) { task ->
    Thread(task, "FaceLink-host-verification").apply { isDaemon = true }
  }.apply { removeOnCancelPolicy = true }

  fun current(): HostVerification? = synchronized(lock) { active }
  fun reserve(create: () -> HostVerification): HostVerification? = synchronized(lock) {
    if (active != null) null else create().also { active = it }
  }

  fun execute(run: HostVerification) {
    worker.execute {
      try { run.perform() }
      finally { finishWhenDrained(run) }
    }
  }

  /** Used if queue submission fails; preserve the same physical-drain reservation contract. */
  fun submissionFailed(run: HostVerification) {
    run.cancel("E_CAPTURE_HOST_WORKER")
    run.perform()
    finishWhenDrained(run)
  }

  private fun finishWhenDrained(run: HostVerification) {
    if (run.physicallyDrained()) {
      try { run.complete() }
      finally { synchronized(lock) { if (active === run) active = null } }
      return
    }
    val elapsed = System.currentTimeMillis() - run.taskReturnedAt
    if (run.taskReturned && !run.stuckNotified && elapsed >= STUCK_TIMEOUT_MS) {
      // Native codec/consumer is still active. Settle promises with an explicit error,
      // but keep the process-wide reservation so a new run cannot overlap the stuck codec.
      run.failStuck()
    }
    // Keep checking with a low-frequency timer: a later physical drain must reclaim the
    // reservation and allow the next explicit run without restarting the process.
    worker.schedule({ finishWhenDrained(run) }, if (run.stuckNotified) 1_000L else 20L, TimeUnit.MILLISECONDS)
  }

  private const val STUCK_TIMEOUT_MS = 8_000L
}

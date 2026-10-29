package com.margelo.nitro.camera.facedetector

import android.graphics.ImageFormat
import android.graphics.Matrix
import android.os.SystemClock
import androidx.camera.core.ImageProxy
import java.util.concurrent.Executors
import java.util.concurrent.ScheduledFuture
import java.util.concurrent.ScheduledThreadPoolExecutor
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean

/** ML Kit's completed task transfers the exact proxy here, never its success listener. */
object Cam02FrameStore {
  data class FaceMetadata(val x: Double, val y: Double, val width: Double, val height: Double,
    val frameWidth: Double, val frameHeight: Double, val yawAngle: Double, val pitchAngle: Double, val rollAngle: Double)
  data class FrameEvent(val generation: Double, val frameId: String, val acquiredAtMs: Long,
    val emittedAtMs: Long, val faces: List<FaceMetadata>)
  data class CaptureResult(val ready: ReadyCapture, val encodeMs: Long, val selectedAgeMs: Long,
    val mirrored: Boolean, val cropInsetPixels: Double)
  data class Status(val core: AdapterStatus, val heldFrames: Int, val previewGeometry: Map<String, Double>?, val syntheticRunning: Boolean)
  interface Sink { fun onFrame(event: FrameEvent) }
  private class Held(val source: Any, val proxy: ImageProxy, override val generation: Long,
    val epoch: Long, override val frameId: String, override val acquiredAtMs: Long,
    val release: () -> Unit) : AndroidYuvFrame {
    override val width get() = proxy.width
    override val height get() = proxy.height
    override lateinit var plan: Cam02Pixels.Plan
    private val released = AtomicBoolean(false)
    var timeout: ScheduledFuture<*>? = null
    override fun planes(): Array<Cam02Pixels.Plane> {
      if (proxy.format != ImageFormat.YUV_420_888) throw Cam02Pixels.Failure("E_CAPTURE_PIXEL_FORMAT")
      val planes = proxy.planes
      return Array(planes.size) { index -> planes[index].let { Cam02Pixels.Plane(it.buffer, it.rowStride, it.pixelStride) } }
    }
    override fun close() { if (released.compareAndSet(false, true)) try { release() } catch (_: Throwable) { } }
  }
  private val lock = Any()
  private val worker = Executors.newSingleThreadExecutor { task -> Thread(task, "FaceLink-capture-encoder").apply { isDaemon = true } }
  private val deadlines = ScheduledThreadPoolExecutor(1) { task ->
    Thread(task, "FaceLink-capture-deadline").apply { isDaemon = true }
  }.apply { removeOnCancelPolicy = true }
  private fun newAdapter() = NativeCaptureAdapter(SystemClock::elapsedRealtime, worker, deadlines, AndroidCaptureEncoder())
  private var adapter = newAdapter()
  private var disposed = false
  private var sink: Sink? = null
  private var epoch = 0L
  private var sequence = 0L
  private var generation: Long? = null
  private var pending: Held? = null
  private var approvedSource: Any? = null
  private var terminalSelected = false
  private var previewGeometry: Map<String, Double>? = null
  private var verification: CaptureSyntheticVerification? = null
  private var verificationComplete = false
  private fun verificationBusy(): Boolean {
    if (verificationComplete && verification?.isDrained() == true) verification = null
    return verification != null
  }
  /** Global reservation survives a React module recreation while a platform codec drains. */
  fun claimVerification(suite: CaptureSyntheticVerification) = synchronized(lock) {
    val core = adapter.status()
    if (disposed) throw CaptureFailure("E_CAPTURE_DISPOSED")
    if (verificationBusy() || pending != null || core.busy || core.phase == "ARMED") throw CaptureFailure("E_CAPTURE_BUSY")
    verification = suite
    verificationComplete = false
  }
  fun finishVerification(suite: CaptureSyntheticVerification) {
    synchronized(lock) { if (verification === suite) { verificationComplete = true; verificationBusy() }; Unit }
  }

  fun attach(value: Sink) = synchronized(lock) {
    if (sink != null && sink !== value) throw CaptureFailure("E_CAPTURE_UNAVAILABLE")
    if (disposed) {
      if (adapter.status().busy) throw CaptureFailure("E_CAPTURE_BUSY")
      adapter = newAdapter()
      disposed = false
    }
    sink = value
  }
  fun detach(value: Sink) { synchronized(lock) { if (sink === value) { dispose(); sink = null } } }

  fun checkedGeneration(value: Double): Long {
    if (!value.isFinite() || value < 0 || value > 9_007_199_254_740_991.0 || value != kotlin.math.floor(value)) {
      throw CaptureFailure("E_CAPTURE_GENERATION")
    }
    return value.toLong()
  }
  fun start(value: Double) = synchronized(lock) {
    val checked = checkedGeneration(value)
    if (disposed) throw CaptureFailure("E_CAPTURE_DISPOSED")
    if (verificationBusy()) throw CaptureFailure("E_CAPTURE_BUSY")
    if (pending != null || sink == null) throw CaptureFailure(if (sink == null) "E_CAPTURE_UNAVAILABLE" else "E_CAPTURE_BUSY")
    adapter.start(checked)
    epoch++
    generation = checked
    approvedSource = null
    terminalSelected = false
    previewGeometry = null
  }
  fun analysisEpoch(): Long? = synchronized(lock) { if (generation != null && !terminalSelected) epoch else null }
  fun activeGeneration(): Long? = synchronized(lock) { generation }

  /** Takes ownership iff true. Caller retains rejected proxies, including deadline setup failures. */
  fun offer(source: Any, acquiredEpoch: Long?, proxy: ImageProxy, acquiredAt: Long, faces: Array<HybridFaceSpec>, release: () -> Unit): Boolean {
    val target: Sink
    val held: Held
    val event: FrameEvent
    synchronized(lock) {
      val active = generation ?: return false
      target = sink ?: return false
      val now = SystemClock.elapsedRealtime()
      if (proxy.width <= 0 || proxy.height <= 0 || proxy.width.toLong() * proxy.height > Cam02Pixels.MAX_PIXELS) return false
      if (acquiredEpoch != epoch || terminalSelected || pending != null || now - acquiredAt !in 0..Cam02Pixels.FRESHNESS_MS) return false
      val metadata = faces.map { face ->
        val bounds = face.bounds
        FaceMetadata(bounds.x, bounds.y, bounds.width, bounds.height, face.frameWidth, face.frameHeight,
          face.yawAngle, face.pitchAngle, face.rollAngle)
      }
      val emittedAt = SystemClock.elapsedRealtime()
      if (emittedAt - acquiredAt !in 0..Cam02Pixels.FRESHNESS_MS) return false
      held = Held(source, proxy, active, epoch, (++sequence).toString(), acquiredAt, release)
      event = FrameEvent(active.toDouble(), held.frameId, acquiredAt, emittedAt, metadata)
      pending = held
      try { held.timeout = deadlines.schedule({ expire(held) }, Cam02Pixels.FRESHNESS_MS - (emittedAt - acquiredAt), TimeUnit.MILLISECONDS) }
      catch (_: Throwable) { pending = null; return false }
    }
    try { target.onFrame(event) } catch (_: Throwable) { expire(held) }
    return true
  }
  private fun expire(held: Held) {
    val close = synchronized(lock) {
      if (pending !== held) false else { pending = null; held.timeout?.cancel(false); held.timeout = null; true }
    }
    if (close) held.close()
  }

  fun decide(value: Double, frameId: String, capture: Boolean, corners: DoubleArray?, done: (CaptureResult?, String?) -> Unit) {
    val checked = checkedGeneration(value)
    synchronized(lock) {
      val held = pending
      if (generation != checked || held == null || held.generation != checked || held.frameId != frameId) {
        done(null, "E_CAPTURE_STALE_FRAME"); return
      }
      pending = null
      held.timeout?.cancel(false)
      held.timeout = null
      if (!capture) { held.close(); done(null, null); return }
      terminalSelected = true
      val selectedAt = SystemClock.elapsedRealtime()
      try {
        if (selectedAt - held.acquiredAtMs !in 0..Cam02Pixels.FRESHNESS_MS) throw CaptureFailure("E_CAPTURE_STALE_FRAME")
        if (corners == null || corners.size != 8 || corners.any { !it.isFinite() }) throw CaptureFailure("E_CAPTURE_CORNERS")
        val proxy = held.proxy
        val mapped = FloatArray(8) { corners[it].toFloat() }
        Matrix(proxy.imageInfo.sensorToBufferTransformMatrix).mapPoints(mapped)
        val rect = proxy.cropRect
        // Coverage diagnostics contain geometry only, never face data or sampled pixels.
        previewGeometry = mapOf("generation" to value,
          "inputWidth" to proxy.width.toDouble(), "inputHeight" to proxy.height.toDouble(),
          "rotationDegrees" to proxy.imageInfo.rotationDegrees.toDouble(),
          "minX" to (0..3).minOf { mapped[it * 2].toDouble() }, "maxX" to (0..3).maxOf { mapped[it * 2].toDouble() },
          "minY" to (0..3).minOf { mapped[it * 2 + 1].toDouble() }, "maxY" to (0..3).maxOf { mapped[it * 2 + 1].toDouble() },
          "cropLeft" to rect.left.toDouble(), "cropTop" to rect.top.toDouble(),
          "cropRight" to rect.right.toDouble(), "cropBottom" to rect.bottom.toDouble())
        held.plan = Cam02Pixels.plan(DoubleArray(8) { mapped[it].toDouble() }, intArrayOf(rect.left, rect.top, rect.right, rect.bottom), proxy.width, proxy.height)
        approvedSource = held.source
        val accepted = adapter.capture(held) { ready, error ->
          synchronized(lock) {
            if (held.epoch != epoch || generation != held.generation) done(null, "E_CAPTURE_CANCELLED")
            else done(ready?.let { CaptureResult(it, it.emittedAtMs - selectedAt, selectedAt - held.acquiredAtMs, held.plan.mirrored, held.plan.insetPixels) }, error)
          }
        }
        if (!accepted) { approvedSource = null; held.close(); done(null, "E_CAPTURE_REJECTED") }
      } catch (failure: CaptureFailure) { held.close(); done(null, failure.code) }
        catch (failure: Cam02Pixels.Failure) { held.close(); done(null, failure.code) }
        catch (_: Throwable) { held.close(); done(null, "E_CAPTURE_INPUT") }
    }
  }

  fun consume(value: Double, handle: String): ConsumeReceipt {
    val current = synchronized(lock) { adapter }
    return current.consume(checkedGeneration(value), handle, NativeJpegValidationConsumer::validate)
  }
  /** Invalidates synchronously; drain acknowledgement waits for active codec/consumer to return. */
  fun cancel(reason: CancelReason = CancelReason.CANCELLED, done: () -> Unit = {}) = synchronized(lock) {
    epoch++
    generation = null
    previewGeometry = null
    approvedSource = null
    val drop = pending
    pending = null
    drop?.timeout?.cancel(false)
    drop?.timeout = null
    drop?.close()
    if (reason == CancelReason.BACKGROUND) CaptureLifecycleBoundary.background(adapter, done) else adapter.cancel(reason, done)
  }
  fun dispose(done: () -> Unit = {}) = synchronized(lock) {
    cancel()
    disposed = true
    adapter.dispose(done)
  }
  /** Closing its source invalidates an approved READY handle too, not only the input proxy. */
  fun cancelSource(source: Any) = synchronized(lock) {
    if (approvedSource === source || pending?.source === source) cancel(CancelReason.SOURCE_CLOSED)
  }
  fun status() = synchronized(lock) {
    val core = adapter.status()
    Status(core, core.ownedFrames + if (pending != null) 1 else 0, previewGeometry, verificationBusy())
  }
}

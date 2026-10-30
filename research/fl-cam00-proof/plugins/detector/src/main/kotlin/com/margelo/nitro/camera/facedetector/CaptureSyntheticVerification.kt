package com.margelo.nitro.camera.facedetector

import android.os.SystemClock
import java.io.IOException
import java.io.OutputStream
import java.nio.ByteBuffer
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executor
import java.util.concurrent.Executors
import java.util.concurrent.ScheduledThreadPoolExecutor
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicReference

/** Explicit synthetic-only real-codec suite. Never opens a camera, file, path, or URL. */
class CaptureSyntheticVerification {
  data class Case(val name: String, val observations: Map<String, Any>, val assertions: Map<String, Boolean>) {
    val passed get() = assertions.isNotEmpty() && assertions.values.all { it }
  }
  data class Report(val cases: List<Case>) { val passed get() = cases.isNotEmpty() && cases.all { it.passed } }
  private val cancelled = AtomicBoolean(false)
  private val lifecycleLock = Any()
  @Volatile private var active: NativeCaptureAdapter? = null
  fun cancelLifecycle() = synchronized(lifecycleLock) {
    cancelled.set(true)
    // Disposal also prevents a pending synthetic fixture from arming after a host pause.
    active?.dispose()
  }
  fun isDrained(): Boolean = active?.status()?.busy != true

  private class SyntheticFrame(override val generation: Long = 1L,
    override val frameId: String = "synthetic", override val acquiredAtMs: Long = SystemClock.elapsedRealtime()) : AndroidYuvFrame {
    override val width = 16
    override val height = 16
    override val plan = Cam02Pixels.plan(doubleArrayOf(0.0, 0.0, 16.0, 0.0, 16.0, 16.0, 0.0, 16.0), intArrayOf(0, 0, 16, 16), 16, 16)
    private val y = ByteArray(256) { (32 + it % 180).toByte() }
    private val u = ByteArray(64) { 110.toByte() }
    private val v = ByteArray(64) { 145.toByte() }
    val closes = AtomicInteger(0)
    override fun planes() = arrayOf(Cam02Pixels.Plane(ByteBuffer.wrap(y), 16, 1),
      Cam02Pixels.Plane(ByteBuffer.wrap(u), 8, 1), Cam02Pixels.Plane(ByteBuffer.wrap(v), 8, 1))
    override fun close() { closes.incrementAndGet(); y.fill(0); u.fill(0); v.fill(0) }
  }
  private class RecordingEncoder(private val firstWrite: (() -> Unit)? = null) : AdapterEncoder {
    val writes = AtomicInteger(0)
    val codecActive = AtomicBoolean(false)
    val injectedWhileActive = AtomicBoolean(false)
    override fun encode(frame: AdapterFrame, output: OutputStream, scope: EncodingScope): ImageInfo {
      val first = AtomicBoolean(true)
      val observed = object : OutputStream() {
        private fun enteringWrite() {
          writes.incrementAndGet()
          if (first.compareAndSet(true, false)) {
            injectedWhileActive.set(codecActive.get() && Thread.currentThread().stackTrace.any {
              it.className == "android.graphics.YuvImage" && it.methodName == "compressToJpeg"
            })
            firstWrite?.invoke()
          }
        }
        override fun write(value: Int) { enteringWrite(); output.write(value) }
        override fun write(bytes: ByteArray, offset: Int, length: Int) { enteringWrite(); output.write(bytes, offset, length) }
      }
      codecActive.set(true)
      try { return AndroidCaptureEncoder().encode(frame, observed, scope) }
      finally { codecActive.set(false) }
    }
  }
  private class Fixture(val encoder: RecordingEncoder, limits: AdapterLimits = AdapterLimits(), queued: Boolean = false) {
    private val executor = Executors.newSingleThreadExecutor { task -> Thread(task, "FaceLink-synthetic-codec").apply { isDaemon = true } }
    private val deadlines = ScheduledThreadPoolExecutor(1).apply { removeOnCancelPolicy = true }
    private val queuedTask = AtomicReference<Runnable?>()
    val adapter = NativeCaptureAdapter(SystemClock::elapsedRealtime,
      if (queued) Executor { queuedTask.set(it) } else executor, deadlines, encoder, limits)
    val frame = SyntheticFrame()
    val done = CountDownLatch(1)
    val callbacks = AtomicInteger(0)
    val ready = AtomicReference<ReadyCapture?>()
    val error = AtomicReference<String?>()
    var accepted = false
    fun capture() {
      adapter.start(1)
      accepted = adapter.capture(frame) { value, failure ->
        callbacks.incrementAndGet(); ready.set(value); error.set(failure); done.countDown()
      }
    }
    fun await() = done.await(2500, TimeUnit.MILLISECONDS)
    fun releaseQueued() { queuedTask.getAndSet(null)?.run() }
    fun cleanup(): Boolean {
      val drained = CountDownLatch(1)
      adapter.dispose { drained.countDown() }
      releaseQueued()
      val released = drained.await(2000, TimeUnit.MILLISECONDS)
      executor.shutdown()
      deadlines.shutdown()
      if (!accepted && frame.closes.get() == 0) frame.close()
      return released
    }
  }
  private fun waitUntil(timeoutMs: Long = 1500, condition: () -> Boolean): Boolean {
    val deadline = SystemClock.elapsedRealtime() + timeoutMs
    while (!condition()) {
      if (SystemClock.elapsedRealtime() >= deadline) return false
      Thread.sleep(5)
    }
    return true
  }
  private fun failure(block: () -> Unit): String? = try { block(); null } catch (error: CaptureFailure) { error.code }
    catch (_: Throwable) { throw IllegalStateException("E_ENR01A_UNEXPECTED") }
  private fun runCase(name: String, block: () -> Case): Case {
    if (cancelled.get()) return Case(name, mapOf("boundary" to "host_lifecycle_cancelled"), mapOf("foregroundThroughout" to false))
    if (!isDrained()) return Case(name, mapOf("boundary" to "prior_codec_cleanup_pending"), mapOf("priorCleanupComplete" to false))
    return try { block() } catch (_: Throwable) { Case(name, mapOf("error" to "E_ENR01A_CASE"), mapOf("completed" to false)) }
  }
  private fun withFixture(name: String, encoder: RecordingEncoder = RecordingEncoder(), limits: AdapterLimits = AdapterLimits(),
    queued: Boolean = false, block: (Fixture, MutableMap<String, Any>, MutableMap<String, Boolean>) -> Unit): Case {
    val fixture = Fixture(encoder, limits, queued)
    synchronized(lifecycleLock) {
      active = fixture.adapter
      if (cancelled.get()) fixture.adapter.dispose()
    }
    val observations = linkedMapOf<String, Any>("source" to "synthetic_yuv_android_codec")
    val assertions = linkedMapOf<String, Boolean>()
    try { block(fixture, observations, assertions) }
    catch (_: Throwable) { assertions["completed"] = false; observations["error"] = "E_ENR01A_CASE" }
    finally {
      val drained = fixture.cleanup()
      val status = fixture.adapter.status()
      observations["codecWrites"] = encoder.writes.get()
      observations["firstWriteOnCodecStack"] = encoder.injectedWhileActive.get()
      observations["inputCloseCount"] = fixture.frame.closes.get()
      observations["callbacks"] = fixture.callbacks.get()
      observations["ownedBytesAfter"] = status.ownedBytes
      observations["ownedFramesAfter"] = status.ownedFrames
      observations["peakOwnedBytes"] = status.peakOwnedBytes
      assertions["cleanupAcknowledged"] = drained
      assertions["buffersZero"] = status.ownedBytes == 0
      assertions["framesZero"] = status.ownedFrames == 0
      assertions["releaseExactlyOnce"] = fixture.frame.closes.get() == 1
      assertions["oneCompletion"] = fixture.callbacks.get() == 1
      if (drained) synchronized(lifecycleLock) { if (active === fixture.adapter) active = null }
    }
    return Case(name, observations, assertions)
  }

  fun run(): Report {
    val results = ArrayList<Case>()
    results += runCase("consume_once_and_double_reject") {
      withFixture("consume_once_and_double_reject") { f, o, a ->
        f.capture(); a["accepted"] = f.accepted; a["readyArrived"] = f.await()
        val ready = f.ready.get() ?: throw IllegalStateException()
        var readBytes = 0
        val receipt = f.adapter.consume(1, ready.handle) { bytes, image ->
          readBytes = bytes.remaining(); a["readOnly"] = bytes.isReadOnly
          NativeJpegValidationConsumer.validate(bytes, image)
        }
        o["validatedBytes"] = readBytes; o["jpegBytes"] = receipt.jpegBytes
        a["exactLengthDecoded"] = readBytes == ready.jpegBytes && readBytes > 0 && receipt.consumed
        a["realCodecUsed"] = f.encoder.writes.get() > 0
        a["doubleConsumeRejected"] = failure { f.adapter.consume(1, ready.handle, NativeJpegValidationConsumer::validate) } != null
      }
    }
    results += runCase("stale_generation_and_concurrent_capture") {
      withFixture("stale_generation_and_concurrent_capture") { f, o, a ->
        f.capture(); a["readyArrived"] = f.await()
        val ready = f.ready.get() ?: throw IllegalStateException()
        a["staleGenerationRejected"] = failure { f.adapter.consume(2, ready.handle, NativeJpegValidationConsumer::validate) } != null
        val extra = SyntheticFrame(frameId = "synthetic-other")
        val accepted = f.adapter.capture(extra) { _, _ -> }
        a["concurrentCaptureRejected"] = !accepted
        if (!accepted) extra.close()
        a["rejectedCallerOwnsInput"] = extra.closes.get() == 1
        f.adapter.consume(1, ready.handle, NativeJpegValidationConsumer::validate)
        o["stage"] = "ready_claim"
      }
    }
    results += runCase("concurrent_consume_single_winner") {
      withFixture("concurrent_consume_single_winner") { f, o, a ->
        f.capture(); a["readyArrived"] = f.await()
        val ready = f.ready.get() ?: throw IllegalStateException()
        val entered = CountDownLatch(1); val release = CountDownLatch(1); val finished = CountDownLatch(1)
        val success = AtomicInteger(0)
        val thread = Thread {
          try { f.adapter.consume(1, ready.handle) { bytes, image ->
            NativeJpegValidationConsumer.validate(bytes, image); entered.countDown()
            if (!release.await(1500, TimeUnit.MILLISECONDS)) throw IOException()
          }; success.incrementAndGet() } catch (_: Throwable) { } finally { finished.countDown() }
        }.apply { isDaemon = true }
        thread.start()
        try {
          a["firstConsumerEntered"] = entered.await(1500, TimeUnit.MILLISECONDS)
          a["secondConsumerRejected"] = failure { f.adapter.consume(1, ready.handle, NativeJpegValidationConsumer::validate) } != null
        } finally { release.countDown() }
        a["consumerFinished"] = finished.await(1500, TimeUnit.MILLISECONDS)
        a["oneWinner"] = success.get() == 1
        o["successfulConsumers"] = success.get()
      }
    }
    for (mode in listOf("cancel", "lifecycle_boundary", "output_exception")) results += runCase("codec_first_write_$mode") {
      lateinit var target: NativeCaptureAdapter
      val encoder = RecordingEncoder {
        when (mode) {
          "cancel" -> target.cancel()
          "lifecycle_boundary" -> CaptureLifecycleBoundary.background(target)
          else -> throw IOException("E_ENR01A_SINK")
        }
      }
      withFixture("codec_first_write_$mode", encoder) { f, o, a ->
        target = f.adapter
        f.capture(); a["completionArrived"] = f.await()
        a["injectedInsideActualCodecWrite"] = encoder.writes.get() > 0 && encoder.injectedWhileActive.get()
        a["noHandleOrSuccess"] = f.ready.get() == null && f.error.get() != null
        a["eventualRelease"] = waitUntil { !f.adapter.status().busy && f.adapter.status().ownedBytes == 0 }
        o["stage"] = "YuvImage.compressToJpeg.OutputStream.firstWrite"
        o["injection"] = if (mode == "lifecycle_boundary") "native_lifecycle_boundary_injection_not_OS_background" else mode
        o["error"] = f.error.get() ?: "NONE"
      }
    }
    results += runCase("queued_input_timeout") {
      withFixture("queued_input_timeout", limits = AdapterLimits(freshnessMs = 80, encodeTimeoutMs = 100), queued = true) { f, o, a ->
        f.capture(); a["timeoutCompletion"] = f.await()
        f.releaseQueued()
        a["noLateHandle"] = f.ready.get() == null && f.error.get() != null
        a["codecNeverEnteredAfterExpiry"] = f.encoder.writes.get() == 0
        o["stage"] = "queued_before_codec"
      }
    }
    results += runCase("ready_ttl_expiry") {
      withFixture("ready_ttl_expiry", limits = AdapterLimits(readyTtlMs = 80)) { f, o, a ->
        f.capture(); a["readyArrived"] = f.await()
        val ready = f.ready.get() ?: throw IllegalStateException()
        a["ttlReleased"] = waitUntil { f.adapter.status().ownedBytes == 0 }
        a["expiredHandleRejected"] = failure { f.adapter.consume(1, ready.handle, NativeJpegValidationConsumer::validate) } != null
        o["stage"] = "ready_deadline"
      }
    }
    results += runCase("consumer_exception_cleanup") {
      withFixture("consumer_exception_cleanup") { f, o, a ->
        f.capture(); a["readyArrived"] = f.await()
        val ready = f.ready.get() ?: throw IllegalStateException()
        a["exceptionIsSafeFailure"] = failure { f.adapter.consume(1, ready.handle) { bytes, image ->
          NativeJpegValidationConsumer.validate(bytes, image); throw IOException("E_ENR01A_CONSUMER")
        } } != null
        a["failedConsumeNotReusable"] = failure { f.adapter.consume(1, ready.handle, NativeJpegValidationConsumer::validate) } != null
        o["stage"] = "native_consumer_callback"
      }
    }
    results += runCase("consumer_timeout_cooperative_release") {
      withFixture("consumer_timeout_cooperative_release", limits = AdapterLimits(consumeTimeoutMs = 80)) { f, o, a ->
        f.capture(); a["readyArrived"] = f.await()
        val ready = f.ready.get() ?: throw IllegalStateException()
        val entered = CountDownLatch(1); val release = CountDownLatch(1); val finished = CountDownLatch(1)
        val consumerError = AtomicReference<String?>()
        Thread {
          try { f.adapter.consume(1, ready.handle) { bytes, image ->
            NativeJpegValidationConsumer.validate(bytes, image); entered.countDown()
            if (!release.await(1500, TimeUnit.MILLISECONDS)) throw IOException()
          } } catch (failure: CaptureFailure) { consumerError.set(failure.code) }
            catch (_: Throwable) { consumerError.set("E_ENR01A_UNEXPECTED") }
            finally { finished.countDown() }
        }.apply { isDaemon = true }.start()
        try {
          a["consumerEntered"] = entered.await(1500, TimeUnit.MILLISECONDS)
          a["logicalTimeoutObserved"] = waitUntil { f.adapter.status().phase == "DRAINING" }
          a["bufferHeldWhileCallbackReads"] = f.adapter.status().ownedBytes > 0 && f.adapter.status().busy
        } finally { release.countDown() }
        a["consumerReturned"] = finished.await(1500, TimeUnit.MILLISECONDS)
        a["noLateConsumeSuccess"] = consumerError.get() == "E_CAPTURE_CONSUME_TIMEOUT"
        a["timedOutHandleRejected"] = failure { f.adapter.consume(1, ready.handle, NativeJpegValidationConsumer::validate) } != null
        o["stage"] = "native_consumer_logical_deadline"
        o["error"] = consumerError.get() ?: "NONE"
      }
    }
    results += runCase("owned_buffer_budget") {
      withFixture("owned_buffer_budget", limits = AdapterLimits(maxOwnedBytes = 128)) { f, o, a ->
        f.capture(); a["failureCompletion"] = f.await()
        a["budgetRejectedWithoutHandle"] = f.ready.get() == null && f.error.get() != null
        a["peakWithinBudget"] = f.adapter.status().peakOwnedBytes <= 128
        o["stage"] = "production_adapter_allocation_budget"
        o["error"] = f.error.get() ?: "NONE"
      }
    }
    results += runCase("real_codec_output_budget_failure") {
      withFixture("real_codec_output_budget_failure", limits = AdapterLimits(maxJpegBytes = 64)) { f, o, a ->
        f.capture(); a["failureCompletion"] = f.await()
        a["realCodecEntered"] = f.encoder.writes.get() > 0 && f.encoder.injectedWhileActive.get()
        a["outputFailureWithoutHandle"] = f.ready.get() == null && f.error.get() != null
        o["stage"] = "YuvImage.compressToJpeg.capped_output"
        o["error"] = f.error.get() ?: "NONE"
      }
    }
    return Report(results)
  }
}

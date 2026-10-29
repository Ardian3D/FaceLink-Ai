package com.margelo.nitro.camera.facedetector

import java.nio.ByteBuffer
import java.nio.ReadOnlyBufferException
import java.util.PriorityQueue
import java.util.concurrent.AbstractExecutorService
import java.util.concurrent.Callable
import java.util.concurrent.CountDownLatch
import java.util.concurrent.CyclicBarrier
import java.util.concurrent.Delayed
import java.util.concurrent.Executor
import java.util.concurrent.Executors
import java.util.concurrent.FutureTask
import java.util.concurrent.RejectedExecutionException
import java.util.concurrent.ScheduledExecutorService
import java.util.concurrent.ScheduledFuture
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicLong
import java.util.concurrent.atomic.AtomicReference

/**
 * Real JVM state/concurrency/resource tests with controlled encoders. JPEG marker fixtures test
 * adapter boundaries only, NOT Android YuvImage, decode correctness or real codec fault coverage.
 */
object NativeCaptureAdapterTest {
  private var assertions = 0
  private fun expect(value: Boolean, message: String = "Adapter assertion failed") { check(value) { message }; assertions++ }
  private fun await(latch: CountDownLatch) { check(latch.await(3, TimeUnit.SECONDS)) { "Latch deadline exceeded" } }
  private fun failure(code: String, action: () -> Unit) {
    try { action(); error("Expected $code") }
    catch (actual: CaptureFailure) { expect(actual.code == code, "Expected $code, got ${actual.code}") }
  }
  private val jpeg = byteArrayOf(-1, -40, -1, -32, 0, 2, -1, -38, 0, 2, 1, 2, -1, -39)
  private val exif = byteArrayOf(-1, -40, -1, -31, 0, 8, 69, 120, 105, 102, 0, 0, -1, -38, 0, 2, -1, -39)
  private val tiny = AdapterLimits(maxPixels = 100, maxJpegBytes = 64, maxOwnedBytes = 96,
    freshnessMs = 400, encodeTimeoutMs = 100, readyTtlMs = 80, consumeTimeoutMs = 60)
  private val fixtureEncoder = AdapterEncoder { _, output, scope ->
    scope.checkActive(); output.write(jpeg); ImageInfo(2, 2)
  }
  private class Frame(
    override val generation: Long = 1, override val frameId: String = "frame",
    override val acquiredAtMs: Long = 1_000, private val onClose: () -> Unit = {}
  ) : AdapterFrame {
    val closes = AtomicInteger()
    var unreadable = false
    override val width: Int get() { if (unreadable) error("closed source details"); return 2 }
    override val height = 2
    override fun close() { closes.incrementAndGet(); onClose() }
  }
  private class QueueWorker : Executor {
    private val jobs = ArrayList<Runnable>()
    override fun execute(command: Runnable) { synchronized(jobs) { jobs.add(command) } }
    fun runAll() {
      while (true) {
        val next = synchronized(jobs) { if (jobs.isEmpty()) null else jobs.removeAt(0) } ?: return
        next.run()
      }
    }
  }
  private class ManualScheduler(val time: AtomicLong = AtomicLong(1_000)) : AbstractExecutorService(), ScheduledExecutorService {
    private var stopped = false
    private var sequence = 0L
    private val jobs = PriorityQueue<Job<*>>()
    private inner class Job<V>(callable: Callable<V>, val at: Long, val order: Long) : FutureTask<V>(callable), ScheduledFuture<V> {
      override fun getDelay(unit: TimeUnit): Long = unit.convert(at - time.get(), TimeUnit.MILLISECONDS)
      override fun compareTo(other: Delayed): Int {
        val that = other as ManualScheduler.Job<*>
        val byTime = at.compareTo(that.at)
        return if (byTime == 0) order.compareTo(that.order) else byTime
      }
    }
    override fun <V> schedule(callable: Callable<V>, delay: Long, unit: TimeUnit): ScheduledFuture<V> = synchronized(jobs) {
      if (stopped) throw RejectedExecutionException()
      Job(callable, time.get() + unit.toMillis(delay), sequence++).also { jobs.add(it) }
    }
    override fun schedule(command: Runnable, delay: Long, unit: TimeUnit): ScheduledFuture<*> = schedule(Callable { command.run() }, delay, unit)
    override fun scheduleAtFixedRate(command: Runnable, initialDelay: Long, period: Long, unit: TimeUnit): ScheduledFuture<*> = throw UnsupportedOperationException()
    override fun scheduleWithFixedDelay(command: Runnable, initialDelay: Long, delay: Long, unit: TimeUnit): ScheduledFuture<*> = throw UnsupportedOperationException()
    override fun execute(command: Runnable) { schedule(command, 0, TimeUnit.MILLISECONDS) }
    override fun shutdown() { synchronized(jobs) { stopped = true } }
    override fun shutdownNow(): MutableList<Runnable> { shutdown(); synchronized(jobs) { jobs.clear() }; return mutableListOf() }
    override fun isShutdown(): Boolean = synchronized(jobs) { stopped }
    override fun isTerminated(): Boolean = isShutdown()
    override fun awaitTermination(timeout: Long, unit: TimeUnit): Boolean = isTerminated()
    fun advance(ms: Long) {
      time.addAndGet(ms)
      while (true) {
        val job = synchronized(jobs) { if (jobs.isNotEmpty() && jobs.peek().at <= time.get()) jobs.remove() else null } ?: return
        job.run()
      }
    }
  }
  private class Result {
    val calls = AtomicInteger()
    val ready = AtomicReference<ReadyCapture?>()
    val error = AtomicReference<String?>()
    val completed = CountDownLatch(1)
    val callback: (ReadyCapture?, String?) -> Unit = { value, failure ->
      ready.set(value); error.set(failure); calls.incrementAndGet(); completed.countDown()
    }
  }
  private fun assertDrained(adapter: NativeCaptureAdapter) {
    val status = adapter.status()
    expect(!status.busy && status.ownedFrames == 0 && status.ownedBytes == 0, "Physical resources did not drain")
  }
  private fun assertOtherThreadCanEnter(adapter: NativeCaptureAdapter) {
    val entered = CountDownLatch(1)
    val thread = Thread { adapter.status(); entered.countDown() }
    thread.start()
    await(entered)
    thread.join()
  }

  private fun lifetimeAndOwnership() {
    val scheduler = ManualScheduler()
    val worker = QueueWorker()
    var scratch: ByteArray? = null
    var backing: ByteArray? = null
    lateinit var adapter: NativeCaptureAdapter
    adapter = NativeCaptureAdapter(scheduler.time::get, worker, scheduler, AdapterEncoder { _, output, scope ->
      scratch = scope.allocateScratch(24).also { it.fill(7) }
      backing = (output as Cam02Pixels.CappedStream).backing
      expect(adapter.status().ownedBytes == 88)
      output.write(jpeg)
      ImageInfo(2, 2)
    }, tiny)
    adapter.start(1)
    val input = Frame(onClose = { assertOtherThreadCanEnter(adapter) })
    val result = Result()
    expect(adapter.capture(input, result.callback))
    expect(!adapter.capture(input) { _, _ -> error("Rejected duplicate callback") })
    expect(input.closes.get() == 0)
    val rejected = Frame(frameId = "other")
    expect(!adapter.capture(rejected) { _, _ -> error("Rejected callback") })
    expect(rejected.closes.get() == 0)
    rejected.close() // Explicit caller ownership on false.
    worker.runAll()
    val ready = result.ready.get()!!
    expect(result.calls.get() == 1 && result.error.get() == null)
    expect(input.closes.get() == 1 && rejected.closes.get() == 1)
    expect(scratch!!.all { it == 0.toByte() })
    expect(ready.generation == 1L && ready.frameId == "frame" && ready.acquiredAtMs == 1_000L)
    expect(ready.image == ImageInfo(2, 2) && ready.jpegBytes == jpeg.size && ready.ownedPeakBytes == 88)
    expect(ready.handle.length == 64 && ready.handle.all { it in "0123456789abcdef" })
    expect(adapter.status().ownedBytes == 64 && adapter.status().ownedFrames == 0)
    failure("E_CAPTURE_GENERATION") { adapter.consume(2, ready.handle) { _, _ -> error("wrong generation") } }
    failure("E_CAPTURE_HANDLE") { adapter.consume(1, "not-a-handle") { _, _ -> error("wrong handle") } }
    var retainedForWipeObservation: ByteBuffer? = null
    val receipt = adapter.consume(1, ready.handle) { buffer, image ->
      assertOtherThreadCanEnter(adapter)
      expect(image == ImageInfo(2, 2))
      expect(buffer.isReadOnly && buffer.position() == 0 && buffer.remaining() == jpeg.size && buffer.capacity() == jpeg.size)
      expect(!buffer.hasArray())
      try { buffer.put(0, 0); error("Writable view") } catch (_: ReadOnlyBufferException) { expect(true) }
      expect(buffer.get(0) == jpeg[0] && buffer.get(jpeg.lastIndex) == jpeg.last())
      expect(adapter.status().ownedBytes == 64 && backing!![0] == jpeg[0])
      // Test-only lifetime probe. Production consumers are forbidden from retaining this view.
      retainedForWipeObservation = buffer
    }
    expect(receipt.consumed && receipt.jpegBytes == jpeg.size && receipt.frameId == "frame")
    expect(backing!!.all { it == 0.toByte() })
    expect((0 until jpeg.size).all { retainedForWipeObservation!!.get(it) == 0.toByte() })
    assertDrained(adapter)
    failure("E_CAPTURE_HANDLE") { adapter.consume(1, ready.handle) { _, _ -> error("duplicate") } }
    val spent = Frame()
    expect(!adapter.capture(spent, result.callback) && spent.closes.get() == 0)
    spent.close()
    failure("E_CAPTURE_GENERATION") { adapter.start(1) }
    adapter.start(2)
    val second = Result()
    expect(adapter.capture(Frame(generation = 2, frameId = "second"), second.callback))
    worker.runAll()
    val next = second.ready.get()!!
    expect(next.handle != ready.handle)
    failure("E_CAPTURE_GENERATION") { adapter.consume(1, ready.handle) { _, _ -> error("old generation") } }
    failure("E_CAPTURE_HANDLE") { adapter.consume(2, ready.handle) { _, _ -> error("old handle") } }
    adapter.consume(2, next.handle) { _, _ -> }
    assertDrained(adapter)
  }

  private fun queuedFreshnessAndRestart() {
    val scheduler = ManualScheduler()
    val worker = QueueWorker()
    val encodes = AtomicInteger()
    val adapter = NativeCaptureAdapter(scheduler.time::get, worker, scheduler, AdapterEncoder { _, output, _ ->
      encodes.incrementAndGet(); output.write(jpeg); ImageInfo(2, 2)
    }, tiny)
    adapter.start(1)
    val result = Result()
    val frame = Frame()
    expect(adapter.capture(frame, result.callback))
    scheduler.advance(401)
    expect(result.error.get() == "E_CAPTURE_STALE_INPUT" && result.ready.get() == null)
    expect(frame.closes.get() == 1 && encodes.get() == 0 && result.calls.get() == 1)
    assertDrained(adapter)
    adapter.start(2)
    val stale = Frame(generation = 2)
    expect(!adapter.capture(stale, result.callback) && stale.closes.get() == 0)
    stale.close()
    val closed = Frame(generation = 2, acquiredAtMs = scheduler.time.get()).also { it.unreadable = true }
    expect(!adapter.capture(closed, result.callback) && closed.closes.get() == 0)
    closed.close()
    val fresh = Frame(generation = 2, acquiredAtMs = scheduler.time.get())
    val second = Result()
    expect(adapter.capture(fresh, second.callback))
    worker.runAll() // The expired first generation's queued runnable must not touch fresh input.
    expect(frame.closes.get() == 1 && encodes.get() == 1 && second.ready.get() != null)
    scheduler.advance(81)
    expect(adapter.status().lastError == "E_CAPTURE_READY_TIMEOUT")
    assertDrained(adapter)
    expect(second.calls.get() == 1 && fresh.closes.get() == 1)
    failure("E_CAPTURE_HANDLE") { adapter.consume(2, second.ready.get()!!.handle) { _, _ -> } }
    adapter.dispose()
    failure("E_CAPTURE_DISPOSED") { adapter.start(3) }
    val disposedInput = Frame(generation = 3, acquiredAtMs = scheduler.time.get())
    expect(!adapter.capture(disposedInput, result.callback) && disposedInput.closes.get() == 0)
    disposedInput.close()
    var drained = false
    adapter.dispose { drained = true }
    expect(drained && adapter.status().phase == "DISPOSED")
  }

  private fun captureRace() {
    val scheduler = ManualScheduler()
    val worker = QueueWorker()
    val adapter = NativeCaptureAdapter(scheduler.time::get, worker, scheduler, fixtureEncoder, tiny)
    adapter.start(1)
    val gate = CyclicBarrier(3)
    val inputs = arrayOf(Frame(frameId = "a"), Frame(frameId = "b"))
    val accepted = BooleanArray(2)
    val results = arrayOf(Result(), Result())
    val threads = (0..1).map { index -> Thread {
      gate.await(); accepted[index] = adapter.capture(inputs[index], results[index].callback)
    }.also { it.start() } }
    gate.await()
    threads.forEach { it.join(3_000); check(!it.isAlive) }
    expect(accepted.count { it } == 1)
    expect(inputs.all { it.closes.get() == 0 })
    inputs[accepted.indexOf(false)].close()
    worker.runAll()
    val winner = accepted.indexOf(true)
    expect(results[winner].ready.get() != null && results[1 - winner].calls.get() == 0)
    adapter.cancel()
    expect(inputs.all { it.closes.get() == 1 })
    assertDrained(adapter)
  }

  private fun encodingInvalidation(timeout: Boolean, terminal: Boolean = false) {
    val scheduler = ManualScheduler()
    val worker = Executors.newSingleThreadExecutor()
    val entered = CountDownLatch(1)
    val release = CountDownLatch(1)
    var scratch: ByteArray? = null
    var backing: ByteArray? = null
    val result = Result()
    lateinit var adapter: NativeCaptureAdapter
    adapter = NativeCaptureAdapter(scheduler.time::get, worker, scheduler, AdapterEncoder { _, output, scope ->
      scratch = scope.allocateScratch(16).also { it.fill(9) }
      backing = (output as Cam02Pixels.CappedStream).backing
      output.write(jpeg)
      entered.countDown()
      await(release)
      // Deliberately ignore cancellation: adapter must recheck even an uncooperative encoder.
      ImageInfo(2, 2)
    }, tiny)
    val input = Frame()
    val drained = CountDownLatch(1)
    try {
      adapter.start(1)
      expect(adapter.capture(input, result.callback))
      await(entered)
      if (timeout) {
        scheduler.advance(101)
        adapter.cancel { assertOtherThreadCanEnter(adapter); drained.countDown() }
      } else if (terminal) adapter.dispose { assertOtherThreadCanEnter(adapter); drained.countDown() }
      else adapter.cancel(CancelReason.BACKGROUND) { assertOtherThreadCanEnter(adapter); drained.countDown() }
      expect(result.calls.get() == 0 && drained.count == 1L && input.closes.get() == 0)
      expect(adapter.status().ownedBytes == 80 && adapter.status().ownedFrames == 1)
      expect(adapter.status().phase == if (terminal) "DISPOSED" else "DRAINING")
      expect(!adapter.capture(input, result.callback) && input.closes.get() == 0)
      failure(if (terminal) "E_CAPTURE_DISPOSED" else "E_CAPTURE_BUSY") { adapter.start(2) }
      expect(backing!![0] == jpeg[0] && scratch!![0] == 9.toByte())
      release.countDown()
      await(result.completed); await(drained)
      expect(result.ready.get() == null && result.calls.get() == 1)
      expect(result.error.get() == when { timeout -> "E_CAPTURE_ENCODE_TIMEOUT"; terminal -> "E_CAPTURE_DISPOSED"; else -> "E_CAPTURE_BACKGROUND" })
      expect(input.closes.get() == 1 && scratch!!.all { it == 0.toByte() } && backing!!.all { it == 0.toByte() })
      assertDrained(adapter)
      if (terminal) failure("E_CAPTURE_DISPOSED") { adapter.start(2) }
      else { adapter.start(2); adapter.dispose() }
    } finally { release.countDown(); worker.shutdownNow(); worker.awaitTermination(3, TimeUnit.SECONDS) }
  }

  private fun consumeRace(cancel: Boolean) {
    val scheduler = ManualScheduler()
    val adapter = NativeCaptureAdapter(scheduler.time::get, Executor { it.run() }, scheduler, fixtureEncoder, tiny)
    adapter.start(1)
    val result = Result()
    expect(adapter.capture(Frame(), result.callback))
    val ready = result.ready.get()!!
    val gate = CyclicBarrier(3)
    val entered = CountDownLatch(1)
    val release = CountDownLatch(1)
    val loserReturned = CountDownLatch(1)
    val calls = AtomicInteger()
    val receipts = AtomicInteger()
    val errors = arrayOf(AtomicReference<String?>(), AtomicReference<String?>())
    val buffer = AtomicReference<ByteBuffer?>()
    val threads = (0..1).map { index -> Thread {
      gate.await()
      try {
        adapter.consume(1, ready.handle) { bytes, _ ->
          calls.incrementAndGet(); buffer.set(bytes); entered.countDown(); await(release)
          check(bytes.get(0) == jpeg[0]) { "Wiped while callback was using bytes" }
        }
        receipts.incrementAndGet()
      } catch (error: CaptureFailure) {
        errors[index].set(error.code)
        if (error.code == "E_CAPTURE_HANDLE") loserReturned.countDown()
      }
    }.also { it.start() } }
    try {
      gate.await(); await(entered); await(loserReturned)
      expect(calls.get() == 1 && adapter.status().ownedBytes == 64)
      val drained = CountDownLatch(1)
      if (cancel) {
        adapter.cancel(CancelReason.SOURCE_CLOSED) { drained.countDown() }
        expect(drained.count == 1L && adapter.status().phase == "DRAINING")
        expect(buffer.get()!!.get(0) == jpeg[0])
      }
      release.countDown()
      threads.forEach { it.join(3_000); check(!it.isAlive) }
      if (cancel) {
        await(drained)
        expect(receipts.get() == 0 && errors.count { it.get() == "E_CAPTURE_SOURCE_CLOSED" } == 1)
      } else expect(receipts.get() == 1 && errors.count { it.get() == null } == 1)
      expect(errors.count { it.get() == "E_CAPTURE_HANDLE" } == 1)
      expect((0 until jpeg.size).all { buffer.get()!!.get(it) == 0.toByte() })
      assertDrained(adapter)
    } finally { release.countDown(); threads.forEach { it.join(3_000) } }
  }

  private fun consumerFailuresAndTimeout() {
    for (kind in listOf("exception", "timeout", "cancel")) {
      val scheduler = ManualScheduler()
      val adapter = NativeCaptureAdapter(scheduler.time::get, Executor { it.run() }, scheduler, fixtureEncoder, tiny)
      adapter.start(1)
      val result = Result()
      expect(adapter.capture(Frame(), result.callback))
      var view: ByteBuffer? = null
      failure(when (kind) { "exception" -> "E_CAPTURE_CONSUMER"; "timeout" -> "E_CAPTURE_CONSUME_TIMEOUT"; else -> "E_CAPTURE_CANCELLED" }) {
        adapter.consume(1, result.ready.get()!!.handle) { bytes, _ ->
          view = bytes
          assertOtherThreadCanEnter(adapter)
          if (kind == "exception") error("sensitive consumer implementation details")
          if (kind == "timeout") scheduler.advance(61) else adapter.cancel()
          expect(adapter.status().phase == "DRAINING" && adapter.status().ownedBytes == 64)
          expect(bytes.get(0) == jpeg[0])
        }
      }
      expect((0 until jpeg.size).all { view!!.get(it) == 0.toByte() })
      assertDrained(adapter)
    }
  }

  private fun encodingFailures() {
    data class Case(val code: String, val limits: AdapterLimits = tiny, val encode: AdapterEncoder)
    val cases = listOf(
      Case("E_CAPTURE_ENCODER", encode = AdapterEncoder { _, _, _ -> error("sensitive codec details") }),
      Case("E_CAPTURE_ENCODER", encode = AdapterEncoder { _, _, _ -> throw CaptureFailure("sensitive-untrusted-code") }),
      Case("E_CAM02_JPEG_CAP", encode = AdapterEncoder { _, out, _ -> out.write(ByteArray(65)); ImageInfo(2, 2) }),
      Case("E_CAM02_JPEG_FORMAT", encode = AdapterEncoder { _, out, _ -> out.write(byteArrayOf(1, 2, 3)); ImageInfo(2, 2) }),
      Case("E_CAPTURE_EXIF", encode = AdapterEncoder { _, out, _ -> out.write(exif); ImageInfo(2, 2) }),
      Case("E_CAPTURE_EXIF", encode = AdapterEncoder { _, out, _ -> out.write(jpeg); ImageInfo(2, 2, true) }),
      Case("E_CAPTURE_OUTPUT_CAP", encode = AdapterEncoder { _, out, _ -> out.write(jpeg); ImageInfo(11, 10) }),
      Case("E_CAPTURE_OWNED_CAP", encode = AdapterEncoder { _, _, scope -> scope.allocateScratch(33); ImageInfo(2, 2) }),
      Case("E_CAPTURE_OWNED_CAP", tiny.copy(maxOwnedBytes = 63), AdapterEncoder { _, _, _ -> error("must not encode without JPEG reservation") }),
      Case("E_CAPTURE_SOURCE_CLOSED", encode = AdapterEncoder { _, _, _ -> throw CaptureFailure("E_CAPTURE_SOURCE_CLOSED") })
    )
    for (case in cases) {
      val scheduler = ManualScheduler()
      var failedBacking: ByteArray? = null
      val observedEncoder = AdapterEncoder { frame, output, scope ->
        failedBacking = (output as Cam02Pixels.CappedStream).backing
        case.encode.encode(frame, output, scope)
      }
      val adapter = NativeCaptureAdapter(scheduler.time::get, Executor { it.run() }, scheduler, observedEncoder, case.limits)
      val input = Frame()
      val result = Result()
      adapter.start(1)
      expect(adapter.capture(input, result.callback))
      expect(result.error.get() == case.code && result.ready.get() == null, "Wrong encoding failure for ${case.code}: ${result.error.get()}")
      expect(input.closes.get() == 1 && result.calls.get() == 1)
      expect(failedBacking == null || failedBacking!!.all { it == 0.toByte() })
      assertDrained(adapter)
    }
    val scheduler = ManualScheduler()
    lateinit var adapter: NativeCaptureAdapter
    adapter = NativeCaptureAdapter(scheduler.time::get, Executor { it.run() }, scheduler, fixtureEncoder, tiny)
    val closeFailure = Frame(onClose = { assertOtherThreadCanEnter(adapter); error("source close details") })
    adapter.start(1)
    val result = Result()
    expect(adapter.capture(closeFailure, result.callback))
    expect(result.error.get() == "E_CAPTURE_SOURCE_CLOSED" && closeFailure.closes.get() == 1)
    assertDrained(adapter)
    adapter.start(2)
    val notifications = AtomicInteger()
    expect(adapter.capture(Frame(generation = 2)) { _, _ ->
      assertOtherThreadCanEnter(adapter); notifications.incrementAndGet(); error("notification failure")
    })
    expect(notifications.get() == 1 && adapter.status().lastError == "E_CAPTURE_CALLBACK")
    assertDrained(adapter)
  }

  private fun scratchReservationRollback() {
    val scheduler = ManualScheduler()
    var scratch: ByteArray? = null
    var backing: ByteArray? = null
    val adapter = NativeCaptureAdapter(scheduler.time::get, Executor { it.run() }, scheduler, AdapterEncoder { _, out, scope ->
      backing = (out as Cam02Pixels.CappedStream).backing
      out.write(jpeg)
      scratch = scope.allocateScratch(16).also { it.fill(13) }
      scope.allocateScratch(17) // JPEG64 + registered16 + requested17 exceeds96.
      error("Budget overflow accepted")
    }, tiny)
    adapter.start(1)
    val result = Result()
    val frame = Frame()
    expect(adapter.capture(frame, result.callback))
    expect(result.error.get() == "E_CAPTURE_OWNED_CAP" && result.ready.get() == null)
    expect(scratch!!.all { it == 0.toByte() } && backing!!.all { it == 0.toByte() })
    expect(frame.closes.get() == 1 && adapter.status().peakOwnedBytes == 80)
    assertDrained(adapter)
  }

  private fun rejectionAndLogicalDeadlines() {
    val scheduler = ManualScheduler()
    val workerFailure = NativeCaptureAdapter(scheduler.time::get, Executor { throw RejectedExecutionException() }, scheduler, fixtureEncoder, tiny)
    workerFailure.start(1)
    val frame = Frame()
    val result = Result()
    expect(workerFailure.capture(frame, result.callback))
    expect(result.error.get() == "E_CAPTURE_WORKER" && frame.closes.get() == 1)
    assertDrained(workerFailure)
    scheduler.shutdown()
    val failedTimer = NativeCaptureAdapter(scheduler.time::get, Executor { it.run() }, scheduler, fixtureEncoder, tiny)
    failedTimer.start(1)
    val timerFrame = Frame()
    val timerResult = Result()
    expect(failedTimer.capture(timerFrame, timerResult.callback))
    expect(timerResult.error.get() == "E_CAPTURE_SCHEDULER" && timerFrame.closes.get() == 1)
    assertDrained(failedTimer)

    val logicalClock = ManualScheduler()
    val timedEncode = NativeCaptureAdapter(logicalClock.time::get, Executor { it.run() }, logicalClock, AdapterEncoder { _, out, _ ->
      out.write(jpeg); logicalClock.time.addAndGet(101); ImageInfo(2, 2)
    }, tiny)
    timedEncode.start(1)
    val timedResult = Result()
    expect(timedEncode.capture(Frame(), timedResult.callback))
    expect(timedResult.error.get() == "E_CAPTURE_ENCODE_TIMEOUT" && timedResult.ready.get() == null)
    assertDrained(timedEncode)
    val readyClock = ManualScheduler()
    val readyAdapter = NativeCaptureAdapter(readyClock.time::get, Executor { it.run() }, readyClock, fixtureEncoder, tiny)
    readyAdapter.start(1)
    val readyResult = Result()
    expect(readyAdapter.capture(Frame(), readyResult.callback))
    readyClock.time.addAndGet(81) // Deliberately do not execute scheduled task: claim checks clock itself.
    failure("E_CAPTURE_READY_TIMEOUT") { readyAdapter.consume(1, readyResult.ready.get()!!.handle) { _, _ -> error("expired callback") } }
    assertDrained(readyAdapter)
    failure("E_CAPTURE_GENERATION") { readyAdapter.start(0) }
    failure("E_CAPTURE_GENERATION") { readyAdapter.start(9_007_199_254_740_992L) }
    try { tiny.copy(maxJpegBytes = Cam02Pixels.MAX_JPEG_BYTES + 1); error("Raised cap") } catch (_: IllegalArgumentException) { expect(true) }
    try { tiny.copy(readyTtlMs = 5_001); error("Raised deadline") } catch (_: IllegalArgumentException) { expect(true) }
  }

  private fun cancelQueuedAndReady() {
    val scheduler = ManualScheduler()
    val worker = QueueWorker()
    val adapter = NativeCaptureAdapter(scheduler.time::get, worker, scheduler, fixtureEncoder, tiny)
    adapter.start(1)
    val first = Frame()
    val result = Result()
    expect(adapter.capture(first, result.callback))
    var drained = 0
    adapter.cancel { assertOtherThreadCanEnter(adapter); drained++ }
    expect(drained == 1 && result.error.get() == "E_CAPTURE_CANCELLED" && first.closes.get() == 1)
    adapter.start(2)
    val second = Result()
    expect(adapter.capture(Frame(generation = 2), second.callback))
    worker.runAll()
    expect(second.ready.get() != null && first.closes.get() == 1)
    adapter.cancel { assertOtherThreadCanEnter(adapter); drained++ }
    expect(drained == 2 && second.calls.get() == 1)
    assertDrained(adapter)
    failure("E_CAPTURE_HANDLE") { adapter.consume(2, second.ready.get()!!.handle) { _, _ -> } }
    adapter.start(3)
    val third = Result()
    expect(adapter.capture(Frame(generation = 3), third.callback))
    worker.runAll()
    adapter.dispose { drained++ }
    expect(drained == 3 && third.calls.get() == 1 && adapter.status().phase == "DISPOSED")
    assertDrained(adapter)
    failure("E_CAPTURE_DISPOSED") { adapter.consume(3, third.ready.get()!!.handle) { _, _ -> } }
  }

  private fun realScheduledExpiry() {
    val scheduler = Executors.newSingleThreadScheduledExecutor()
    val worker = QueueWorker()
    val now = { System.nanoTime() / 1_000_000 }
    val adapter = NativeCaptureAdapter(now, worker, scheduler, fixtureEncoder, tiny.copy(freshnessMs = 30))
    try {
      adapter.start(1)
      val frame = Frame(acquiredAtMs = now())
      val result = Result()
      expect(adapter.capture(frame, result.callback))
      await(result.completed)
      expect(result.error.get() == "E_CAPTURE_STALE_INPUT" && frame.closes.get() == 1)
      worker.runAll()
      expect(frame.closes.get() == 1)
      assertDrained(adapter)
    } finally { scheduler.shutdownNow(); scheduler.awaitTermination(3, TimeUnit.SECONDS) }

    val deadlines = Executors.newSingleThreadScheduledExecutor()
    val encodingWorker = Executors.newSingleThreadExecutor()
    val entered = CountDownLatch(1)
    val release = CountDownLatch(1)
    val timed = NativeCaptureAdapter(now, encodingWorker, deadlines, AdapterEncoder { _, out, _ ->
      out.write(jpeg); entered.countDown(); await(release); ImageInfo(2, 2)
    }, tiny.copy(encodeTimeoutMs = 200))
    try {
      val workerReady = CountDownLatch(1)
      encodingWorker.execute { workerReady.countDown() }
      await(workerReady)
      timed.start(1)
      val result = Result()
      val input = Frame(acquiredAtMs = now())
      expect(timed.capture(input, result.callback))
      await(entered)
      val observed = CountDownLatch(1)
      val phase = AtomicReference<String?>()
      deadlines.schedule({ phase.set(timed.status().phase); observed.countDown() }, 250, TimeUnit.MILLISECONDS)
      await(observed)
      expect(phase.get() == "DRAINING" && result.calls.get() == 0 && input.closes.get() == 0)
      release.countDown(); await(result.completed)
      expect(result.error.get() == "E_CAPTURE_ENCODE_TIMEOUT" && input.closes.get() == 1)
      assertDrained(timed)
    } finally {
      release.countDown(); encodingWorker.shutdownNow(); deadlines.shutdownNow()
      encodingWorker.awaitTermination(3, TimeUnit.SECONDS); deadlines.awaitTermination(3, TimeUnit.SECONDS)
    }
  }

  private fun notificationDrainOrdering() {
    for (terminal in listOf(false, true)) {
      val scheduler = ManualScheduler()
      val worker = Executors.newSingleThreadExecutor()
      val entered = CountDownLatch(1)
      val release = CountDownLatch(1)
      val drained = CountDownLatch(1)
      val delivered = AtomicInteger()
      val deliveredAtDrain = AtomicInteger(-1)
      val adapter = NativeCaptureAdapter(scheduler.time::get, worker, scheduler, fixtureEncoder, tiny)
      try {
        adapter.start(1)
        expect(adapter.capture(Frame()) { ready, error ->
          entered.countDown()
          await(release)
          if (ready != null && error == null) delivered.incrementAndGet()
        })
        await(entered)
        val acknowledge = { deliveredAtDrain.set(delivered.get()); drained.countDown() }
        if (terminal) adapter.dispose(acknowledge) else adapter.cancel(drained = acknowledge)
        expect(drained.count == 1L, "Drain acknowledged before the in-flight READY notification returned")
        expect(adapter.status().busy, "A new generation could start while old notification is in flight")
        release.countDown()
        await(drained)
        expect(deliveredAtDrain.get() == 1, "READY notification occurred after drain acknowledgement")
        assertDrained(adapter)
      } finally {
        release.countDown()
        worker.shutdownNow()
        worker.awaitTermination(3, TimeUnit.SECONDS)
      }
    }
  }

  @JvmStatic fun main(args: Array<String>) {
    lifetimeAndOwnership()
    queuedFreshnessAndRestart()
    captureRace()
    encodingInvalidation(timeout = false)
    encodingInvalidation(timeout = true)
    encodingInvalidation(timeout = false, terminal = true)
    consumeRace(cancel = false)
    consumeRace(cancel = true)
    consumerFailuresAndTimeout()
    encodingFailures()
    scratchReservationRollback()
    rejectionAndLogicalDeadlines()
    cancelQueuedAndReady()
    realScheduledExpiry()
    notificationDrainOrdering()
    println("ENR01A pure JVM adapter assertions: $assertions (controlled encoders; no Android codec coverage)")
  }
}

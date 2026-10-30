package ai.facelink.capturehost

import android.os.SystemClock
import com.margelo.nitro.camera.facedetector.AdapterLimits
import com.margelo.nitro.camera.facedetector.AndroidCaptureEncoder
import com.margelo.nitro.camera.facedetector.AndroidYuvFrame
import com.margelo.nitro.camera.facedetector.Cam02Pixels
import com.margelo.nitro.camera.facedetector.CaptureFailure
import com.margelo.nitro.camera.facedetector.NativeCaptureAdapter
import com.margelo.nitro.camera.facedetector.NativeJpegValidationConsumer
import com.margelo.nitro.camera.facedetector.ReadyCapture
import java.nio.ByteBuffer
import java.security.MessageDigest
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.ScheduledThreadPoolExecutor
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicReference

/** Real Android encoder and scoped native consumer; no image capability leaves this class. */
internal class HostIdentityVerification(private val cancelled: AtomicBoolean) {
  private class SyntheticFrame(override val generation: Long, override val frameId: String, alternate: Boolean) : AndroidYuvFrame {
    override val acquiredAtMs = SystemClock.elapsedRealtime()
    override val width = 16
    override val height = 16
    override val plan = Cam02Pixels.plan(
      doubleArrayOf(0.0, 0.0, 16.0, 0.0, 16.0, 16.0, 0.0, 16.0), intArrayOf(0, 0, 16, 16), 16, 16)
    private val y = ByteArray(256) { if (alternate) (210 - it % 70).toByte() else (32 + it % 100).toByte() }
    private val u = ByteArray(64) { if (alternate) 155.toByte() else 105.toByte() }
    private val v = ByteArray(64) { if (alternate) 95.toByte() else 150.toByte() }
    val closes = AtomicInteger(0)
    override fun planes() = arrayOf(Cam02Pixels.Plane(ByteBuffer.wrap(y), 16, 1),
      Cam02Pixels.Plane(ByteBuffer.wrap(u), 8, 1), Cam02Pixels.Plane(ByteBuffer.wrap(v), 8, 1))
    override fun close() { closes.incrementAndGet(); y.fill(0); u.fill(0); v.fill(0) }
    fun wiped() = y.all { it == 0.toByte() } && u.all { it == 0.toByte() } && v.all { it == 0.toByte() }
  }

  private val codec = Executors.newSingleThreadExecutor { task ->
    Thread(task, "FaceLink-host-synthetic-codec").apply { isDaemon = true }
  }
  private val deadlines = ScheduledThreadPoolExecutor(1).apply { removeOnCancelPolicy = true }
  private val closed = AtomicBoolean(false)
  private val adapter = NativeCaptureAdapter(SystemClock::elapsedRealtime, codec, deadlines, AndroidCaptureEncoder())

  fun cancel() { adapter.dispose() }
  fun isDrained() = !adapter.status().busy
  fun status() = adapter.status()

  /** Called only once run() returned AND the adapter's physical drain is observable. */
  fun closeDrainedExecutors(): Boolean {
    if (!isDrained()) return false
    if (closed.compareAndSet(false, true)) { codec.shutdown(); deadlines.shutdown() }
    return codec.isTerminated && deadlines.isTerminated
  }

  private fun checkActive() {
    if (cancelled.get()) throw CaptureFailure("E_CAPTURE_HOST_CANCELLED")
  }

  private fun awaitIdle(): Boolean {
    val until = SystemClock.elapsedRealtime() + 2_000
    while (adapter.status().busy) {
      checkActive()
      if (SystemClock.elapsedRealtime() >= until) return false
      Thread.sleep(2)
    }
    return true
  }

  private fun capture(frame: SyntheticFrame): ReadyCapture {
    var accepted = false
    try {
      checkActive()
      adapter.start(frame.generation)
      val done = CountDownLatch(1)
      val ready = AtomicReference<ReadyCapture?>()
      val error = AtomicReference<String?>()
      accepted = adapter.capture(frame) { result, failure -> ready.set(result); error.set(failure); done.countDown() }
      if (!accepted) throw CaptureFailure("E_CAPTURE_HOST_INPUT_REJECTED")
      if (!done.await(6_000, TimeUnit.MILLISECONDS)) throw CaptureFailure("E_CAPTURE_HOST_READY_TIMEOUT")
      checkActive()
      return ready.get() ?: throw CaptureFailure(error.get() ?: "E_CAPTURE_HOST_READY")
    } finally {
      // A cancelled/rejected start never transfers ownership, including a lifecycle race before capture().
      if (!accepted) frame.close()
    }
  }

  private fun rejection(block: () -> Unit): String? = try { block(); null } catch (failure: CaptureFailure) { failure.code }

  fun run(): HostCase {
    val observations = linkedMapOf<String, Any?>("source" to "synthetic_yuv_android_codec", "format" to "image/jpeg",
      "fixtureCount" to 2, "width" to 16, "height" to 16)
    val assertions = linkedMapOf<String, Boolean>()
    val frames = ArrayList<SyntheticFrame>(2)
    var firstDigest: ByteArray? = null
    var secondDigest: ByteArray? = null
    try {
      checkActive()
      var prior: ReadyCapture? = null
      for (index in 0..1) {
        checkActive()
        val frame = SyntheticFrame((index + 1).toLong(), if (index == 0) "host-synthetic-a" else "host-synthetic-b", index == 1)
        frames.add(frame)
        val ready = capture(frame)
        val prefix = if (index == 0) "first" else "second"
        observations["${prefix}Generation"] = ready.generation
        observations["${prefix}FrameId"] = ready.frameId
        observations["${prefix}JpegBytes"] = ready.jpegBytes
        assertions["${prefix}ReadyMatchesSelectedInput"] = ready.generation == frame.generation &&
          ready.frameId == frame.frameId && ready.acquiredAtMs == frame.acquiredAtMs
        assertions["${prefix}ReadyDimensionsExact"] = ready.image.width == frame.plan.width &&
          ready.image.height == frame.plan.height && !ready.image.exifPresent
        prior?.let { previous ->
          assertions["stalePriorGenerationRejected"] = rejection {
            adapter.consume(previous.generation, previous.handle, NativeJpegValidationConsumer::validate)
          } == "E_CAPTURE_GENERATION"
          assertions["priorHandleRejectedInNewGeneration"] = rejection {
            adapter.consume(ready.generation, previous.handle, NativeJpegValidationConsumer::validate)
          } == "E_CAPTURE_HANDLE"
        }
        checkActive()
        val receipt = adapter.consume(ready.generation, ready.handle) { bytes, image ->
          checkActive()
          assertions["${prefix}ReadOnlyExactLengthScope"] = bytes.isReadOnly && bytes.position() == 0 &&
            bytes.limit() == ready.jpegBytes && bytes.capacity() == ready.jpegBytes && bytes.remaining() == ready.jpegBytes
          assertions["${prefix}ConsumerImageInfoExact"] = image == ready.image && image.width == 16 && image.height == 16 && !image.exifPresent
          if (assertions["${prefix}ReadOnlyExactLengthScope"] != true || assertions["${prefix}ConsumerImageInfoExact"] != true) {
            throw CaptureFailure("E_CAPTURE_HOST_CONSUMER")
          }
          // Duplicate BEFORE the bounds validator consumes its view. Never extract/copy a JPEG array.
          val digestView = bytes.duplicate()
          NativeJpegValidationConsumer.validate(bytes, image)
          assertions["${prefix}JpegFormatAndBoundsValidated"] = true
          val digester = MessageDigest.getInstance("SHA-256")
          try {
            digester.update(digestView)
            val digest = digester.digest()
            if (index == 0) firstDigest = digest else secondDigest = digest
          } finally { digester.reset() }
          checkActive()
        }
        assertions["${prefix}ReceiptMatchesInputAndReady"] = receipt.consumed && receipt.generation == frame.generation &&
          receipt.frameId == frame.frameId && receipt.acquiredAtMs == frame.acquiredAtMs && receipt.image == ready.image &&
          receipt.jpegBytes == ready.jpegBytes
        assertions["${prefix}DoubleConsumeRejected"] = rejection {
          adapter.consume(ready.generation, ready.handle, NativeJpegValidationConsumer::validate)
        } == "E_CAPTURE_HANDLE"
        assertions["${prefix}PhysicallyReleasedBeforeNextGeneration"] = awaitIdle()
        if (assertions["${prefix}PhysicallyReleasedBeforeNextGeneration"] != true) throw CaptureFailure("E_CAPTURE_HOST_DRAIN")
        prior = ready
      }
      val left = firstDigest
      val right = secondDigest
      val differs = left != null && right != null && !MessageDigest.isEqual(left, right)
      // Only this boolean crosses the metadata boundary; neither digest is serialized or logged.
      observations["nativeContentDifferent"] = differs
      assertions["twoDistinctFixtureContents"] = differs
      assertions["strictlyIncreasingGeneration"] = prior?.generation == 2L
      assertions["staleGenerationCannotRearm"] = rejection { adapter.start(1) } == "E_CAPTURE_GENERATION"
    } catch (_: Throwable) {
      observations["error"] = if (cancelled.get()) "E_CAPTURE_HOST_CANCELLED" else "E_CAPTURE_HOST_IDENTITY"
      assertions["completed"] = false
    } finally {
      firstDigest?.fill(0); secondDigest?.fill(0)
      val drained = CountDownLatch(1)
      adapter.dispose { drained.countDown() }
      val acknowledged = drained.await(2_000, TimeUnit.MILLISECONDS)
      val status = adapter.status()
      observations["ownedBytesAfter"] = status.ownedBytes
      observations["ownedFramesAfter"] = status.ownedFrames
      observations["peakOwnedBytes"] = status.peakOwnedBytes
      observations["inputCloseCount"] = frames.sumOf { it.closes.get() }
      assertions["cleanupAcknowledged"] = acknowledged && !status.busy
      assertions["buffersZero"] = status.ownedBytes == 0
      assertions["framesZero"] = status.ownedFrames == 0
      assertions["bothInputsReleasedExactlyOnce"] = frames.size == 2 && frames.all { it.closes.get() == 1 }
      assertions["fixtureArraysWipedBestEffort"] = frames.all { it.wiped() }
      assertions["peakWithinCoreBudget"] = status.peakOwnedBytes <= AdapterLimits().maxOwnedBytes
    }
    return HostCase("native_identity_content_two_generations", observations, assertions)
  }
}

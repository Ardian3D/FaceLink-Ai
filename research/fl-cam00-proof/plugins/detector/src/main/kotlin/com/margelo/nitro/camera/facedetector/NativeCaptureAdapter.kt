package com.margelo.nitro.camera.facedetector

import java.io.OutputStream
import java.nio.ByteBuffer
import java.security.SecureRandom
import java.util.concurrent.Executor
import java.util.concurrent.ScheduledExecutorService
import java.util.concurrent.ScheduledFuture
import java.util.concurrent.TimeUnit

/** capture(true) transfers this exact input; capture(false) never closes it, even on duplicates. */
interface AdapterFrame {
  val generation: Long
  val frameId: String
  val acquiredAtMs: Long
  val width: Int
  val height: Int
  fun close()
}

fun interface AdapterEncoder {
  fun encode(frame: AdapterFrame, output: OutputStream, scope: EncodingScope): ImageInfo
}

interface EncodingScope {
  /** Also checks logical cancellation/deadlines between non-interruptible platform calls. */
  fun checkActive()
  /** Adapter-owned, zeroed on encode exit. Do not retain or release this array yourself. */
  fun allocateScratch(bytes: Int): ByteArray
}

data class ImageInfo(val width: Int, val height: Int, val exifPresent: Boolean = false)
data class AdapterLimits(
  val maxPixels: Int = Cam02Pixels.MAX_PIXELS,
  val maxJpegBytes: Int = Cam02Pixels.MAX_JPEG_BYTES,
  val maxOwnedBytes: Int = Cam02Pixels.MAX_OWNED_BYTES,
  val freshnessMs: Long = Cam02Pixels.FRESHNESS_MS,
  val encodeTimeoutMs: Long = 5_000,
  val readyTtlMs: Long = 5_000,
  val consumeTimeoutMs: Long = 5_000
) {
  init {
    require(maxPixels in 1..Cam02Pixels.MAX_PIXELS)
    require(maxJpegBytes in 1..Cam02Pixels.MAX_JPEG_BYTES)
    require(maxOwnedBytes in 1..Cam02Pixels.MAX_OWNED_BYTES)
    require(freshnessMs in 1..Cam02Pixels.FRESHNESS_MS)
    require(encodeTimeoutMs in 1..5_000 && readyTtlMs in 1..5_000 && consumeTimeoutMs in 1..5_000)
  }
}

/** Native/bridge metadata only. Never log or serialize the handle outside the private facade. */
data class ReadyCapture(
  val generation: Long, val frameId: String, val acquiredAtMs: Long,
  val emittedAtMs: Long, val expiresAtMs: Long, val image: ImageInfo,
  val jpegBytes: Int, val ownedPeakBytes: Int, val handle: String
) {
  override fun toString(): String = "ReadyCapture(generation=$generation, jpegBytes=$jpegBytes, handle=<redacted>)"
}
data class ConsumeReceipt(
  val generation: Long, val frameId: String, val acquiredAtMs: Long,
  val image: ImageInfo, val jpegBytes: Int, val consumed: Boolean = true
)
data class AdapterStatus(
  val phase: String, val generation: Long, val busy: Boolean,
  val ownedBytes: Int, val peakOwnedBytes: Int, val ownedFrames: Int,
  val lastError: String?, val limits: AdapterLimits
)
class CaptureFailure(val code: String) : RuntimeException(code)
enum class CancelReason { CANCELLED, BACKGROUND, SOURCE_CLOSED, DISPOSED }

/**
 * One approved capture per strictly increasing generation. No Android, storage or transport.
 *
 * Timers invalidate logically: a running encoder/consumer retains its input/buffer until it
 * cooperates or returns. cancel/dispose acknowledge physical release through drained, never by
 * blocking a UI thread or wiping memory underneath native code. Executors are host-owned.
 * Encoder/scope and consumer calls are synchronous; they must not retain references or start
 * asynchronous use. A trusted consumer may inspect only the callback-scoped read-only buffer;
 * this lifetime contract is not protection from malicious native code copying/retaining it.
 * All foreign callbacks, source closes, wipes and scheduler/executor calls occur outside lock.
 */
class NativeCaptureAdapter(
  private val clock: () -> Long,
  private val worker: Executor,
  private val deadlines: ScheduledExecutorService,
  private val encoder: AdapterEncoder,
  private val limits: AdapterLimits = AdapterLimits()
) {
  private enum class Phase { QUEUED, ENCODING, READY, CONSUMING, CLEANING }
  private class Transaction(
    val frame: AdapterFrame, val generation: Long, val frameId: String,
    val acquiredAtMs: Long, val encodeDeadline: Long,
    val done: (ReadyCapture?, String?) -> Unit
  ) {
    var phase = Phase.QUEUED
    var error: String? = null
    var timer: ScheduledFuture<*>? = null
    var frameOwned = true
    var jpeg: ByteArray? = null
    var jpegCount = 0
    var image: ImageInfo? = null
    var ready: ReadyCapture? = null
    var notified = false
    var notificationInFlight = false
    var cleanupComplete = false
    var ownedBytes = 0
    var peakBytes = 0
    val drained = ArrayList<() -> Unit>()
  }

  private val lock = Any()
  private val random = SecureRandom()
  private var generation = 0L
  private var armed = false
  private var spent = false
  private var disposed = false
  private var active: Transaction? = null
  private var ownedBytes = 0
  private var peakBytes = 0
  private var ownedFrames = 0
  private var lastError: String? = null

  fun start(generation: Long) {
    synchronized(lock) {
      if (disposed) throw CaptureFailure("E_CAPTURE_DISPOSED")
      if (active != null) throw CaptureFailure("E_CAPTURE_BUSY")
      if (generation <= this.generation || generation > MAX_SAFE_GENERATION) throw CaptureFailure("E_CAPTURE_GENERATION")
      this.generation = generation
      armed = true
      spent = false
      lastError = null
    }
  }

  /** Rejection leaves ownership with caller and invokes no completion callback. */
  fun capture(frame: AdapterFrame, done: (ReadyCapture?, String?) -> Unit): Boolean {
    val now = clock()
    val frameGeneration: Long
    val frameId: String
    val acquired: Long
    val width: Int
    val height: Int
    try {
      frameGeneration = frame.generation
      frameId = frame.frameId
      acquired = frame.acquiredAtMs
      width = frame.width
      height = frame.height
    } catch (_: Throwable) { return reject("E_CAPTURE_SOURCE_CLOSED") }
    val transaction = synchronized(lock) {
      val error = when {
        disposed -> "E_CAPTURE_DISPOSED"
        active != null -> "E_CAPTURE_BUSY"
        !armed || frameGeneration != generation || spent -> "E_CAPTURE_GENERATION"
        frameId.isEmpty() || acquired < 0 || acquired > now -> "E_CAPTURE_IDENTITY"
        now - acquired > limits.freshnessMs -> "E_CAPTURE_STALE_INPUT"
        width <= 0 || height <= 0 || width.toLong() * height > limits.maxPixels -> "E_CAPTURE_INPUT_CAP"
        else -> null
      }
      if (error != null) {
        lastError = error
        return false
      }
      Transaction(frame, frameGeneration, frameId, acquired, deadline(now, limits.encodeTimeoutMs), done).also {
        active = it
        spent = true
        ownedFrames++
      }
    }
    armTimer(transaction, Phase.QUEUED, minOf(deadline(acquired, limits.freshnessMs), transaction.encodeDeadline))
    try { worker.execute { encode(transaction) } }
    catch (_: Throwable) { fail(transaction, "E_CAPTURE_WORKER") }
    return true
  }

  /**
   * Claims a READY handle atomically. The sole winning caller invokes consumer synchronously,
   * outside the state lock. Read-only buffer capacity/limit equal the actual JPEG byte count.
   * Exceptions, cancellation and timeout all wipe only AFTER this callback has returned.
   */
  fun consume(generation: Long, handle: String, consumer: (ByteBuffer, ImageInfo) -> Unit): ConsumeReceipt {
    val now = clock()
    var expired: Transaction? = null
    val transaction = synchronized(lock) {
      if (disposed) throw CaptureFailure("E_CAPTURE_DISPOSED")
      if (generation != this.generation) throw CaptureFailure("E_CAPTURE_GENERATION")
      val current = active ?: throw CaptureFailure("E_CAPTURE_HANDLE")
      if (current.phase != Phase.READY || current.error != null || current.ready?.handle != handle) throw CaptureFailure("E_CAPTURE_HANDLE")
      if (now >= current.ready!!.expiresAtMs) {
        invalidateLocked(current, "E_CAPTURE_READY_TIMEOUT")
        current.phase = Phase.CLEANING
        expired = current
      } else current.phase = Phase.CONSUMING
      current
    }
    if (expired != null) {
      finish(transaction)
      throw CaptureFailure("E_CAPTURE_READY_TIMEOUT")
    }
    val until = deadline(now, limits.consumeTimeoutMs)
    armTimer(transaction, Phase.CONSUMING, until)
    var failure: String? = synchronized(lock) { transaction.error }
    try {
      if (failure == null) {
        val bytes = transaction.jpeg!!
        val view = ByteBuffer.wrap(bytes, 0, transaction.jpegCount).slice().asReadOnlyBuffer()
        try { consumer(view, transaction.image!!) }
        catch (_: Throwable) { failure = "E_CAPTURE_CONSUMER" }
      }
      val returnedAt = clock()
      synchronized(lock) {
        if (failure != null) invalidateLocked(transaction, failure!!)
        if (returnedAt >= until) invalidateLocked(transaction, "E_CAPTURE_CONSUME_TIMEOUT")
        failure = transaction.error
        transaction.phase = Phase.CLEANING
      }
      if (failure != null) throw CaptureFailure(failure!!)
      return ConsumeReceipt(transaction.generation, transaction.frameId, transaction.acquiredAtMs,
        transaction.image!!, transaction.jpegCount)
    } finally { finish(transaction) }
  }

  fun cancel(reason: CancelReason = CancelReason.CANCELLED, drained: () -> Unit = {}) {
    stop(reason, false, drained)
  }

  /** Terminal, including while an earlier operation is still cooperatively draining. */
  fun dispose(drained: () -> Unit = {}) { stop(CancelReason.DISPOSED, true, drained) }

  fun status(): AdapterStatus = synchronized(lock) {
    val current = active
    AdapterStatus(when {
      disposed -> "DISPOSED"
      current?.error != null || current?.phase == Phase.CLEANING -> "DRAINING"
      current != null -> if (current.phase == Phase.QUEUED) "ENCODING" else current.phase.name
      armed && !spent -> "ARMED"
      else -> "IDLE"
    }, generation, current != null, ownedBytes, peakBytes, ownedFrames, lastError, limits)
  }

  private fun reject(code: String): Boolean {
    synchronized(lock) { lastError = code }
    return false
  }

  private fun stop(reason: CancelReason, terminal: Boolean, drained: () -> Unit) {
    var cleanup: Transaction? = null
    val idle = synchronized(lock) {
      if (terminal) disposed = true
      armed = false
      val current = active
      if (current == null) true else {
        current.drained.add(drained)
        invalidateLocked(current, when (reason) {
          CancelReason.CANCELLED -> "E_CAPTURE_CANCELLED"
          CancelReason.BACKGROUND -> "E_CAPTURE_BACKGROUND"
          CancelReason.SOURCE_CLOSED -> "E_CAPTURE_SOURCE_CLOSED"
          CancelReason.DISPOSED -> "E_CAPTURE_DISPOSED"
        })
        if (current.phase == Phase.QUEUED || current.phase == Phase.READY) {
          current.phase = Phase.CLEANING
          cleanup = current
        }
        false
      }
    }
    if (idle) notifySafely(drained)
    cleanup?.let { finish(it) }
  }

  private inner class Scope(private val transaction: Transaction) : EncodingScope {
    private val scratch = ArrayList<ByteArray>()
    override fun checkActive() {
      val now = clock()
      synchronized(lock) {
        if (active !== transaction || transaction.phase != Phase.ENCODING) throw CaptureFailure("E_CAPTURE_CANCELLED")
        transaction.error?.let { throw CaptureFailure(it) }
        if (now >= transaction.encodeDeadline) throw CaptureFailure("E_CAPTURE_ENCODE_TIMEOUT")
      }
    }
    override fun allocateScratch(bytes: Int): ByteArray {
      checkActive()
      if (bytes <= 0) throw CaptureFailure("E_CAPTURE_OWNED_CAP")
      reserve(transaction, bytes)
      val allocation = try { ByteArray(bytes) }
      catch (error: Throwable) { releaseReservation(transaction, bytes); throw error }
      try { scratch.add(allocation) }
      catch (error: Throwable) {
        allocation.fill(0)
        releaseReservation(transaction, bytes)
        throw error
      }
      return allocation
    }
    fun release() {
      for (bytes in scratch) {
        bytes.fill(0)
        releaseReservation(transaction, bytes.size)
      }
      scratch.clear()
    }
  }

  private fun encode(transaction: Transaction) {
    val now = clock()
    var stale = false
    val claimed = synchronized(lock) {
      if (active !== transaction || transaction.phase != Phase.QUEUED) false
      else {
        if (now - transaction.acquiredAtMs > limits.freshnessMs || now >= transaction.encodeDeadline) {
          invalidateLocked(transaction, "E_CAPTURE_STALE_INPUT")
          transaction.phase = Phase.CLEANING
          stale = true
          false
        } else { transaction.phase = Phase.ENCODING; true }
      }
    }
    if (!claimed) { if (stale) finish(transaction); return }
    armTimer(transaction, Phase.ENCODING, transaction.encodeDeadline)
    val scope = Scope(transaction)
    var image: ImageInfo? = null
    var handle: String? = null
    try {
      scope.checkActive()
      reserve(transaction, limits.maxJpegBytes)
      transaction.jpeg = try { ByteArray(limits.maxJpegBytes) }
      catch (error: Throwable) { releaseReservation(transaction, limits.maxJpegBytes); throw error }
      val stream = Cam02Pixels.CappedStream(transaction.jpeg!!)
      image = encoder.encode(transaction.frame, stream, scope)
      scope.checkActive()
      if (image.width <= 0 || image.height <= 0 || image.width.toLong() * image.height > limits.maxPixels) throw CaptureFailure("E_CAPTURE_OUTPUT_CAP")
      if (Cam02Pixels.inspectExif(stream.backing, stream.count) || image.exifPresent) throw CaptureFailure("E_CAPTURE_EXIF")
      transaction.jpegCount = stream.count
      handle = newHandle()
    } catch (error: Throwable) {
      synchronized(lock) { invalidateLocked(transaction, safeEncodeError(error)) }
    } finally {
      scope.release()
      closeInput(transaction)
    }
    val emittedAt = clock()
    val publish = synchronized(lock) {
      if (emittedAt >= transaction.encodeDeadline) invalidateLocked(transaction, "E_CAPTURE_ENCODE_TIMEOUT")
      if (transaction.error != null) { transaction.phase = Phase.CLEANING; false }
      else {
        transaction.image = image!!
        transaction.phase = Phase.READY
        true
      }
    }
    if (!publish) { finish(transaction); return }
    // Randomness was produced during encoding, outside lock and inside its cleanup boundary.
    val ready = ReadyCapture(transaction.generation, transaction.frameId, transaction.acquiredAtMs,
      emittedAt, deadline(emittedAt, limits.readyTtlMs), image!!, transaction.jpegCount,
      transaction.peakBytes, handle!!)
    val installed = synchronized(lock) {
      if (active === transaction && transaction.phase == Phase.READY && transaction.error == null) {
        transaction.ready = ready
        true
      } else false
    }
    if (!installed) return // The READY cancellation owner performs physical cleanup.
    armTimer(transaction, Phase.READY, ready.expiresAtMs)
    val notify = synchronized(lock) {
      if (active === transaction && transaction.phase == Phase.READY && transaction.error == null && !transaction.notified) {
        transaction.notified = true
        transaction.notificationInFlight = true
        true
      } else false
    }
    if (notify) {
      try {
        val error = synchronized(lock) { transaction.error }
        transaction.done(if (error == null) ready else null, error)
      } catch (_: Throwable) { fail(transaction, "E_CAPTURE_CALLBACK") }
      finally {
        synchronized(lock) { transaction.notificationInFlight = false }
        completeCallbacks(transaction)
      }
    }
  }

  private fun reserve(transaction: Transaction, bytes: Int) {
    synchronized(lock) {
      transaction.error?.let { throw CaptureFailure(it) }
      if (active !== transaction || bytes > limits.maxOwnedBytes - ownedBytes) throw CaptureFailure("E_CAPTURE_OWNED_CAP")
      ownedBytes += bytes
      transaction.ownedBytes += bytes
      peakBytes = maxOf(peakBytes, ownedBytes)
      transaction.peakBytes = maxOf(transaction.peakBytes, transaction.ownedBytes)
    }
  }

  private fun releaseReservation(transaction: Transaction, bytes: Int) {
    synchronized(lock) { transaction.ownedBytes -= bytes; ownedBytes -= bytes }
  }

  private fun closeInput(transaction: Transaction) {
    val close = synchronized(lock) {
      if (transaction.frameOwned) { transaction.frameOwned = false; true } else false
    }
    if (!close) return
    try { transaction.frame.close() }
    catch (_: Throwable) { synchronized(lock) { invalidateLocked(transaction, "E_CAPTURE_SOURCE_CLOSED") } }
    finally { synchronized(lock) { ownedFrames-- } }
  }

  /** Caller has exclusive physical cleanup ownership (queued/ready claim or active call exit). */
  private fun finish(transaction: Transaction) {
    cancelTimer(transaction)
    closeInput(transaction)
    transaction.jpeg?.let {
      it.fill(0)
      transaction.jpeg = null
      releaseReservation(transaction, it.size)
    }
    synchronized(lock) { transaction.cleanupComplete = true }
    completeCallbacks(transaction)
  }

  /** Physical image cleanup may finish first; drain waits for every reserved notification. */
  private fun completeCallbacks(transaction: Transaction) {
    var completion: ((ReadyCapture?, String?) -> Unit)? = null
    var error: String? = null
    val drained = synchronized(lock) {
      if (active !== transaction || !transaction.cleanupComplete || transaction.notificationInFlight) return
      if (!transaction.notified) {
        transaction.notified = true
        transaction.notificationInFlight = true
        completion = transaction.done
        error = transaction.error ?: "E_CAPTURE_CANCELLED"
        null
      } else {
        transaction.ready = null
        active = null
        transaction.drained.toList().also { transaction.drained.clear() }
      }
    }
    completion?.let { callback ->
      try { notifySafely { callback(null, error) } }
      finally {
        synchronized(lock) { transaction.notificationInFlight = false }
        completeCallbacks(transaction)
      }
    }
    if (drained != null) for (callback in drained) notifySafely(callback)
  }

  private fun fail(transaction: Transaction, code: String, expected: Phase? = null) {
    val cleanup = synchronized(lock) {
      if (active !== transaction || (expected != null && transaction.phase != expected)) return
      invalidateLocked(transaction, code)
      if (transaction.phase == Phase.QUEUED || transaction.phase == Phase.READY) {
        transaction.phase = Phase.CLEANING
        true
      } else false
    }
    if (cleanup) finish(transaction)
  }

  private fun invalidateLocked(transaction: Transaction, code: String) {
    if (transaction.error == null) { transaction.error = code; lastError = code }
  }

  private fun armTimer(transaction: Transaction, phase: Phase, at: Long) {
    val old = synchronized(lock) {
      if (active !== transaction || transaction.phase != phase || transaction.error != null) return
      transaction.timer.also { transaction.timer = null }
    }
    old?.cancel(false)
    val future = try {
      deadlines.schedule({
        if (clock() < at) armTimer(transaction, phase, at)
        else fail(transaction, when (phase) {
          Phase.QUEUED -> "E_CAPTURE_STALE_INPUT"
          Phase.ENCODING -> "E_CAPTURE_ENCODE_TIMEOUT"
          Phase.READY -> "E_CAPTURE_READY_TIMEOUT"
          Phase.CONSUMING -> "E_CAPTURE_CONSUME_TIMEOUT"
          Phase.CLEANING -> "E_CAPTURE_CANCELLED"
        }, phase)
      }, maxOf(0, at - clock()), TimeUnit.MILLISECONDS)
    } catch (_: Throwable) { fail(transaction, "E_CAPTURE_SCHEDULER", phase); return }
    val keep = synchronized(lock) {
      if (active === transaction && transaction.phase == phase && transaction.error == null) {
        transaction.timer = future
        true
      } else false
    }
    if (!keep) future.cancel(false)
  }

  private fun cancelTimer(transaction: Transaction) {
    val timer = synchronized(lock) { transaction.timer.also { transaction.timer = null } }
    timer?.cancel(false)
  }

  private fun newHandle(): String {
    val bytes = ByteArray(32)
    random.nextBytes(bytes)
    val digits = "0123456789abcdef"
    val result = CharArray(bytes.size * 2)
    for (i in bytes.indices) {
      val value = bytes[i].toInt() and 255
      result[i * 2] = digits[value ushr 4]
      result[i * 2 + 1] = digits[value and 15]
    }
    bytes.fill(0)
    return String(result)
  }

  private fun safeEncodeError(error: Throwable): String = when (error) {
    is CaptureFailure -> if (error.code in SAFE_CAPTURE_ERRORS) error.code else "E_CAPTURE_ENCODER"
    is Cam02Pixels.Failure -> if (error.code in SAFE_PIXEL_ERRORS) error.code else "E_CAPTURE_ENCODER"
    else -> "E_CAPTURE_ENCODER"
  }

  private fun notifySafely(callback: () -> Unit) { try { callback() } catch (_: Throwable) { } }
  private fun deadline(now: Long, duration: Long): Long = if (now > Long.MAX_VALUE - duration) Long.MAX_VALUE else now + duration

  private companion object {
    const val MAX_SAFE_GENERATION = 9_007_199_254_740_991L
    val SAFE_CAPTURE_ERRORS = setOf("E_CAPTURE_CANCELLED", "E_CAPTURE_BACKGROUND", "E_CAPTURE_SOURCE_CLOSED",
      "E_CAPTURE_DISPOSED", "E_CAPTURE_ENCODE_TIMEOUT", "E_CAPTURE_OWNED_CAP", "E_CAPTURE_OUTPUT_CAP", "E_CAPTURE_EXIF")
    val SAFE_PIXEL_ERRORS = setOf("E_CAM02_PLANE", "E_CAM02_INPUT_CAP", "E_CAM02_CORNERS", "E_CAM02_CROP",
      "E_CAM02_PREVIEW_NOT_CONTAINED", "E_CAM02_NON_ORTHOGONAL", "E_CAM02_OUTPUT_CAP", "E_CAM02_JPEG_CAP", "E_CAM02_JPEG_FORMAT")
  }
}

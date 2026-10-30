package com.margelo.nitro.camera.facedetector

import android.graphics.BitmapFactory
import java.io.InputStream
import java.nio.ByteBuffer

/** Fixed trusted consumer: reads exact scoped JPEG bytes in memory, never materializes pixels. */
object NativeJpegValidationConsumer {
  fun validate(bytes: ByteBuffer, image: ImageInfo) {
    if (!bytes.isReadOnly || bytes.remaining() < 4) throw CaptureFailure("E_CAPTURE_CONSUMER")
    val view = bytes
    val start = view.position()
    val end = view.limit()
    fun unsigned(index: Int) = view.get(index).toInt() and 255
    if (unsigned(start) != 255 || unsigned(start + 1) != 216 ||
      unsigned(end - 2) != 255 || unsigned(end - 1) != 217) throw CaptureFailure("E_CAPTURE_CONSUMER")
    // Decode directly from the bounded view: no full-JPEG array extraction/copy, file, or loader.
    val stream = object : InputStream() {
      override fun read(): Int = if (view.hasRemaining()) view.get().toInt() and 255 else -1
      override fun read(target: ByteArray, offset: Int, length: Int): Int {
        if (offset < 0 || length < 0 || offset > target.size - length) throw IndexOutOfBoundsException()
        if (length == 0) return 0
        if (!view.hasRemaining()) return -1
        val count = minOf(length, view.remaining())
        view.get(target, offset, count)
        return count
      }
      override fun skip(count: Long): Long {
        val skipped = minOf(maxOf(count, 0L), view.remaining().toLong()).toInt()
        view.position(view.position() + skipped)
        return skipped.toLong()
      }
      override fun available() = view.remaining()
    }
    val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true; inScaled = false }
    BitmapFactory.decodeStream(stream, null, bounds)
    if (bounds.outWidth != image.width || bounds.outHeight != image.height || bounds.outMimeType != "image/jpeg") {
      throw CaptureFailure("E_CAPTURE_CONSUMER")
    }
  }
}

/** Shared native logical lifecycle boundary; it does not pretend to pause the Android OS. */
object CaptureLifecycleBoundary {
  fun background(adapter: NativeCaptureAdapter, drained: () -> Unit = {}) =
    adapter.cancel(CancelReason.BACKGROUND, drained)
}

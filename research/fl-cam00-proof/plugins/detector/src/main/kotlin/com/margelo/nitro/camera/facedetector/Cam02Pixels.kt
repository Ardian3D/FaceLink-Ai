package com.margelo.nitro.camera.facedetector

import java.io.OutputStream
import java.nio.ByteBuffer
import kotlin.math.abs
import kotlin.math.ceil
import kotlin.math.floor

/** Pure JVM pixel/format boundaries. No Android, disk, network, or whole-input copy. */
object Cam02Pixels {
  const val MAX_PIXELS = 1920 * 1080
  const val MAX_JPEG_BYTES = 4 * 1024 * 1024
  const val MAX_OWNED_BYTES = 16 * 1024 * 1024
  const val FRESHNESS_MS = 400L
  // Float Matrix roundtrips may cross an exact sensor edge by subpixel noise.
  private const val COVERAGE_EPSILON_PX = 0.05

  class Failure(val code: String) : RuntimeException(code)
  private fun requireSafe(ok: Boolean, code: String) { if (!ok) throw Failure(code) }

  data class Plane(val bytes: ByteBuffer, val rowStride: Int, val pixelStride: Int) {
    private val start = bytes.position()
    fun sample(x: Int, y: Int): Byte {
      requireSafe(x >= 0 && y >= 0 && rowStride > 0 && pixelStride > 0, "E_CAM02_PLANE")
      val offset = start.toLong() + y.toLong() * rowStride + x.toLong() * pixelStride
      requireSafe(offset >= start && offset < bytes.limit(), "E_CAM02_PLANE")
      return bytes.get(offset.toInt())
    }
  }

  data class Plan(
    val width: Int, val height: Int,
    val left: Int, val top: Int, val right: Int, val bottom: Int,
    val ux: Int, val uy: Int, val vx: Int, val vy: Int,
    val insetPixels: Double
  ) {
    val startX get() = if (ux + vx > 0) left else right - 1
    val startY get() = if (uy + vy > 0) top else bottom - 1
    fun sourceX(x: Int, y: Int) = startX + ux * x + vx * y
    fun sourceY(x: Int, y: Int) = startY + uy * x + vy * y
    val mirrored get() = ux * vy - uy * vx < 0
  }

  /** Ordered preview TL/TR/BR/BL, already mapped through sensorToBuffer. */
  fun plan(corners: DoubleArray, crop: IntArray, inputWidth: Int, inputHeight: Int): Plan {
    requireSafe(inputWidth > 0 && inputHeight > 0 && inputWidth.toLong() * inputHeight <= MAX_PIXELS, "E_CAM02_INPUT_CAP")
    requireSafe(corners.size == 8 && corners.all { it.isFinite() } && crop.size == 4, "E_CAM02_CORNERS")
    requireSafe(crop[0] >= 0 && crop[1] >= 0 && crop[2] <= inputWidth && crop[3] <= inputHeight && crop[0] < crop[2] && crop[1] < crop[3], "E_CAM02_CROP")
    val minX = (0..3).minOf { corners[it * 2] }
    val maxX = (0..3).maxOf { corners[it * 2] }
    val minY = (0..3).minOf { corners[it * 2 + 1] }
    val maxY = (0..3).maxOf { corners[it * 2 + 1] }
    requireSafe(minX >= crop[0] - COVERAGE_EPSILON_PX && minY >= crop[1] - COVERAGE_EPSILON_PX
      && maxX <= crop[2] + COVERAGE_EPSILON_PX && maxY <= crop[3] + COVERAGE_EPSILON_PX,
      "E_CAM02_PREVIEW_NOT_CONTAINED")
    fun axis(dx: Double, dy: Double): Pair<Int, Int> {
      requireSafe((abs(dx) < 0.05 && abs(dy) >= 2.0) || (abs(dy) < 0.05 && abs(dx) >= 2.0), "E_CAM02_NON_ORTHOGONAL")
      return if (abs(dx) < 0.05) 0 to if (dy > 0) 1 else -1 else (if (dx > 0) 1 else -1) to 0
    }
    val (ux, uy) = axis(corners[2] - corners[0], corners[3] - corners[1])
    val (vx, vy) = axis(corners[6] - corners[0], corners[7] - corners[1])
    requireSafe(ux * vx + uy * vy == 0 && abs(corners[4] - (corners[2] + corners[6] - corners[0])) < 0.05 && abs(corners[5] - (corners[3] + corners[7] - corners[1])) < 0.05, "E_CAM02_NON_ORTHOGONAL")
    // Inset at most two buffer pixels per edge, never expand beyond preview/crop.
    val left = (ceil(minX / 2.0) * 2).toInt()
    val top = (ceil(minY / 2.0) * 2).toInt()
    val right = (floor(maxX / 2.0) * 2).toInt()
    val bottom = (floor(maxY / 2.0) * 2).toInt()
    // Margin only affects comparison; sampling must remain exactly inside crop.
    requireSafe(left >= crop[0] && top >= crop[1] && right <= crop[2] && bottom <= crop[3],
      "E_CAM02_PREVIEW_NOT_CONTAINED")
    val width = if (ux != 0) right - left else bottom - top
    val height = if (vx != 0) right - left else bottom - top
    requireSafe(width >= 2 && height >= 2 && width.toLong() * height <= MAX_PIXELS, "E_CAM02_OUTPUT_CAP")
    return Plan(width, height, left, top, right, bottom, ux, uy, vx, vy,
      maxOf(left - minX, top - minY, maxX - right, maxY - bottom))
  }

  /** Samples directly into final upright/preview-mirrored NV21. */
  fun sampleNv21(plan: Plan, planes: Array<Plane>, target: ByteArray, checkCurrent: () -> Unit) {
    requireSafe(planes.size == 3 && target.size == plan.width * plan.height * 3 / 2, "E_CAM02_PLANE")
    var offset = 0
    for (y in 0 until plan.height) {
      checkCurrent()
      for (x in 0 until plan.width) target[offset++] = planes[0].sample(plan.sourceX(x, y), plan.sourceY(x, y))
    }
    for (y in 0 until plan.height step 2) {
      checkCurrent()
      for (x in 0 until plan.width step 2) {
        val sx = plan.sourceX(x, y) / 2
        val sy = plan.sourceY(x, y) / 2
        target[offset++] = planes[2].sample(sx, sy)
        target[offset++] = planes[1].sample(sx, sy)
      }
    }
  }

  class CappedStream(val backing: ByteArray) : OutputStream() {
    var count = 0
      private set
    override fun write(value: Int) {
      requireSafe(count < backing.size, "E_CAM02_JPEG_CAP")
      backing[count++] = value.toByte()
    }
    override fun write(bytes: ByteArray, offset: Int, length: Int) {
      requireSafe(offset >= 0 && length >= 0 && offset <= bytes.size - length, "E_CAM02_JPEG_FORMAT")
      requireSafe(length <= backing.size - count, "E_CAM02_JPEG_CAP")
      bytes.copyInto(backing, count, offset, offset + length)
      count += length
    }
  }

  /** Walk JPEG marker segments, not compressed bytes that may resemble APP1. */
  fun inspectExif(bytes: ByteArray, count: Int): Boolean {
    requireSafe(count in 4..bytes.size, "E_CAM02_JPEG_FORMAT")
    fun b(i: Int) = bytes[i].toInt() and 255
    requireSafe(b(0) == 255 && b(1) == 216 && b(count - 2) == 255 && b(count - 1) == 217, "E_CAM02_JPEG_FORMAT")
    var index = 2
    var exif = false
    var entropy = false
    var sawScan = false
    while (index < count) {
      if (entropy) {
        while (index < count && b(index) != 255) index++
      }
      requireSafe(index < count && b(index++) == 255, "E_CAM02_JPEG_FORMAT")
      while (index < count && b(index) == 255) index++
      requireSafe(index < count, "E_CAM02_JPEG_FORMAT")
      val marker = b(index++)
      if (entropy && (marker == 0 || marker in 208..215)) continue
      entropy = false
      if (marker == 217) {
        requireSafe(sawScan && index == count, "E_CAM02_JPEG_FORMAT")
        return exif
      }
      requireSafe(marker !in 208..216 && marker != 0 && index + 2 <= count, "E_CAM02_JPEG_FORMAT")
      val length = (b(index) shl 8) or b(index + 1)
      requireSafe(length >= 2 && length <= count - index, "E_CAM02_JPEG_FORMAT")
      if (marker == 225 && length >= 8 && b(index + 2) == 69 && b(index + 3) == 120 && b(index + 4) == 105 && b(index + 5) == 102 && b(index + 6) == 0 && b(index + 7) == 0) exif = true
      index += length
      if (marker == 218) { entropy = true; sawScan = true }
    }
    throw Failure("E_CAM02_JPEG_FORMAT")
  }
}

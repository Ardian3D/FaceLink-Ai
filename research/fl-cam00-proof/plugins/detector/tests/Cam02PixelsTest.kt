package com.margelo.nitro.camera.facedetector

import java.nio.ByteBuffer

/** Offline JVM regression executable. Exercises production helpers, not Android wiring. */
fun main() {
  var assertions = 0
  fun expect(value: Boolean) { check(value); assertions++ }
  fun failure(code: String, block: () -> Unit) {
    try { block(); error("Expected $code") }
    catch (actual: Cam02Pixels.Failure) { expect(actual.code == code) }
  }
  val crop = intArrayOf(0, 0, 4, 2)
  // Nonzero ByteBuffer position, padded Y rows, interleaved UV pixel stride.
  val y = ByteBuffer.wrap(byteArrayOf(99, 99, 0, 1, 2, 3, 99, 99, 4, 5, 6, 7)).apply { position(2) }
  val u = ByteBuffer.wrap(byteArrayOf(10, 99, 11))
  val v = ByteBuffer.wrap(byteArrayOf(20, 99, 21))
  val planes = arrayOf(Cam02Pixels.Plane(y, 6, 1), Cam02Pixels.Plane(u, 4, 2), Cam02Pixels.Plane(v, 4, 2))
  fun sampled(corners: DoubleArray, expected: ByteArray, mirrored: Boolean = false) {
    val plan = Cam02Pixels.plan(corners, crop, 4, 2)
    val bytes = ByteArray(plan.width * plan.height * 3 / 2)
    Cam02Pixels.sampleNv21(plan, planes, bytes) { }
    expect(bytes.contentEquals(expected))
    expect(plan.mirrored == mirrored)
  }
  sampled(doubleArrayOf(0.0, 0.0, 4.0, 0.0, 4.0, 2.0, 0.0, 2.0), byteArrayOf(0, 1, 2, 3, 4, 5, 6, 7, 20, 10, 21, 11))
  sampled(doubleArrayOf(4.0, 0.0, 0.0, 0.0, 0.0, 2.0, 4.0, 2.0), byteArrayOf(3, 2, 1, 0, 7, 6, 5, 4, 21, 11, 20, 10), true)
  sampled(doubleArrayOf(0.0, 2.0, 0.0, 0.0, 4.0, 0.0, 4.0, 2.0), byteArrayOf(4, 0, 5, 1, 6, 2, 7, 3, 20, 10, 21, 11))
  sampled(doubleArrayOf(4.0, 0.0, 4.0, 2.0, 0.0, 2.0, 0.0, 0.0), byteArrayOf(3, 7, 2, 6, 1, 5, 0, 4, 21, 11, 20, 10))
  sampled(doubleArrayOf(4.0, 2.0, 0.0, 2.0, 0.0, 0.0, 4.0, 0.0), byteArrayOf(7, 6, 5, 4, 3, 2, 1, 0, 21, 11, 20, 10))
  // Measured TECNO float-matrix roundtrip: bottom 960.0000610351562 for crop 960.
  val measured = Cam02Pixels.plan(
    doubleArrayOf(28.525840759277344, 0.0, 1251.4739990234375, 0.0,
      1251.4739990234375, 960.0000610351562, 28.525840759277344, 960.0000610351562),
    intArrayOf(0, 0, 1280, 960), 1280, 960)
  expect(measured.left == 30 && measured.right == 1250 && measured.top == 0 && measured.bottom == 960)
  sampled(doubleArrayOf(-0.00003, 0.0, 4.000061, 0.0, 4.000061, 2.000061, -0.00003, 2.000061),
    byteArrayOf(0, 1, 2, 3, 4, 5, 6, 7, 20, 10, 21, 11))
  failure("E_CAM02_PREVIEW_NOT_CONTAINED") {
    Cam02Pixels.plan(doubleArrayOf(0.0, 0.0, 4.051, 0.0, 4.051, 2.0, 0.0, 2.0), crop, 4, 2)
  }
  val oddCrop = Cam02Pixels.plan(
    doubleArrayOf(0.99997, 0.99997, 7.000061, 0.99997, 7.000061, 7.000061, 0.99997, 7.000061),
    intArrayOf(1, 1, 7, 7), 8, 8)
  expect(oddCrop.left == 2 && oddCrop.top == 2 && oddCrop.right == 6 && oddCrop.bottom == 6)
  failure("E_CAM02_PREVIEW_NOT_CONTAINED") { Cam02Pixels.plan(doubleArrayOf(-0.1, 0.0, 4.0, 0.0, 4.0, 2.0, -0.1, 2.0), crop, 4, 2) }
  failure("E_CAM02_NON_ORTHOGONAL") { Cam02Pixels.plan(doubleArrayOf(0.0, 0.0, 3.0, 1.0, 4.0, 2.0, 1.0, 1.0), crop, 4, 2) }
  failure("E_CAM02_INPUT_CAP") { Cam02Pixels.plan(DoubleArray(8), crop, Int.MAX_VALUE, Int.MAX_VALUE) }
  failure("E_CAM02_CORNERS") { Cam02Pixels.plan(DoubleArray(8) { Double.NaN }, crop, 4, 2) }
  failure("E_CAM02_PLANE") { Cam02Pixels.Plane(ByteBuffer.wrap(byteArrayOf(1)), Int.MAX_VALUE, 2).sample(2, 2) }
  val inset = Cam02Pixels.plan(doubleArrayOf(0.5, 0.5, 7.5, 0.5, 7.5, 7.5, 0.5, 7.5), intArrayOf(0, 0, 8, 8), 8, 8)
  expect(inset.left == 2 && inset.top == 2 && inset.right == 6 && inset.bottom == 6 && inset.insetPixels == 1.5)
  val stream = Cam02Pixels.CappedStream(ByteArray(3))
  stream.write(byteArrayOf(1, 2), 0, 2); stream.write(3)
  expect(stream.count == 3)
  failure("E_CAM02_JPEG_CAP") { stream.write(4) }
  expect(stream.count == 3)
  failure("E_CAM02_JPEG_CAP") { stream.write(byteArrayOf(5), 0, 1) }
  expect(stream.backing.contentEquals(byteArrayOf(1, 2, 3)))
  // Small structural JPEG markers are format-parser fixtures, not decoded fake images.
  val clean = byteArrayOf(-1, -40, -1, -32, 0, 2, -1, -38, 0, 2, 1, 2, -1, -39)
  expect(!Cam02Pixels.inspectExif(clean, clean.size))
  val exif = byteArrayOf(-1, -40, -1, -31, 0, 8, 69, 120, 105, 102, 0, 0, -1, -38, 0, 2, -1, -39)
  expect(Cam02Pixels.inspectExif(exif, exif.size))
  val lateExif = byteArrayOf(-1, -40, -1, -38, 0, 2, 1, -1, 0, -31, -1, -48, 2, -1, -31, 0, 8, 69, 120, 105, 102, 0, 0, -1, -39)
  expect(Cam02Pixels.inspectExif(lateExif, lateExif.size))
  val escapedMarker = byteArrayOf(-1, -40, -1, -38, 0, 2, 1, -1, 0, -31, 69, 120, 105, 102, 0, 0, -1, -39)
  expect(!Cam02Pixels.inspectExif(escapedMarker, escapedMarker.size))
  val capacity = ByteArray(100)
  clean.copyInto(capacity)
  expect(!Cam02Pixels.inspectExif(capacity, clean.size))
  failure("E_CAM02_JPEG_FORMAT") { Cam02Pixels.inspectExif(capacity, capacity.size) }
  failure("E_CAM02_JPEG_FORMAT") { Cam02Pixels.inspectExif(byteArrayOf(-1, -40, -1, -32, 0, 99, -1, -39), 8) }
  var checks = 0
  val plan = Cam02Pixels.plan(doubleArrayOf(0.0, 0.0, 4.0, 0.0, 4.0, 2.0, 0.0, 2.0), crop, 4, 2)
  failure("E_CAM02_CANCELLED") { Cam02Pixels.sampleNv21(plan, planes, ByteArray(12)) { if (++checks == 2) throw Cam02Pixels.Failure("E_CAM02_CANCELLED") } }
  expect(checks == 2)
  println("CAM02 pure Kotlin boundary assertions: $assertions")
}

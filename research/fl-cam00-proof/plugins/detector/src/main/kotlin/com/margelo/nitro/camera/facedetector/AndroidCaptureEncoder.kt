package com.margelo.nitro.camera.facedetector

import android.graphics.ImageFormat
import android.graphics.Rect
import android.graphics.YuvImage
import java.io.OutputStream

/** Plane views remain borrowed from the exact completed analyzer input until core closes it. */
interface AndroidYuvFrame : AdapterFrame {
  val plan: Cam02Pixels.Plan
  fun planes(): Array<Cam02Pixels.Plane>
}

/** The only production JPEG encoder. Scratch and output ownership belong to the core adapter. */
class AndroidCaptureEncoder : AdapterEncoder {
  override fun encode(frame: AdapterFrame, output: OutputStream, scope: EncodingScope): ImageInfo {
    val input = frame as? AndroidYuvFrame ?: throw Cam02Pixels.Failure("E_CAPTURE_INPUT")
    scope.checkActive()
    val plan = input.plan
    val nv21 = scope.allocateScratch(plan.width * plan.height * 3 / 2)
    Cam02Pixels.sampleNv21(plan, input.planes(), nv21) { scope.checkActive() }
    scope.checkActive()
    val compressed = YuvImage(nv21, ImageFormat.NV21, plan.width, plan.height, null)
      .compressToJpeg(Rect(0, 0, plan.width, plan.height), 90, output)
    scope.checkActive()
    if (!compressed) throw Cam02Pixels.Failure("E_CAPTURE_ENCODER")
    return ImageInfo(plan.width, plan.height, false)
  }
}

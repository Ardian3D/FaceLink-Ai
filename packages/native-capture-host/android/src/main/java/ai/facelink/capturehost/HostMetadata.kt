package ai.facelink.capturehost

import com.facebook.react.bridge.Arguments
import com.facebook.react.bridge.WritableMap
import com.margelo.nitro.camera.facedetector.AdapterLimits

/** Explicit allowlisted scalars only. Never serialize a core ReadyCapture, buffer, digest or handle. */
internal object HostMetadata {
  fun scalar(target: WritableMap, key: String, value: Any?) {
    when (value) {
      null -> target.putNull(key)
      is Boolean -> target.putBoolean(key, value)
      is Int -> target.putInt(key, value)
      is Long -> target.putDouble(key, value.toDouble())
      is Double -> if (value.isFinite()) target.putDouble(key, value) else target.putNull(key)
      is String -> target.putString(key, value)
      else -> throw IllegalArgumentException("E_CAPTURE_HOST_METADATA")
    }
  }

  fun report(report: HostReport): WritableMap = Arguments.createMap().apply {
    putString("suite", "FL-CAM03")
    putString("source", "synthetic_native_host")
    putBoolean("passed", report.passed)
    putString("status", if (report.passed) "PASSED" else "FAILED")
    putArray("cases", Arguments.createArray().apply {
      for (case in report.cases) pushMap(Arguments.createMap().apply {
        putString("name", case.name)
        putBoolean("passed", case.passed)
        putString("status", if (case.passed) "PASSED" else "FAILED")
        putMap("observations", Arguments.createMap().apply {
          for ((key, value) in case.observations) scalar(this, key, value)
        })
        putMap("assertions", Arguments.createMap().apply {
          for ((key, value) in case.assertions) putBoolean(key, value)
        })
      })
    })
  }

  fun limits(): WritableMap = Arguments.createMap().apply {
    val limits = AdapterLimits()
    putInt("maxPixels", limits.maxPixels)
    putInt("maxJpegBytes", limits.maxJpegBytes)
    putInt("maxOwnedBytes", limits.maxOwnedBytes)
    putDouble("freshnessMs", limits.freshnessMs.toDouble())
    putDouble("encodeTimeoutMs", limits.encodeTimeoutMs.toDouble())
    putDouble("readyTtlMs", limits.readyTtlMs.toDouble())
    putDouble("consumeTimeoutMs", limits.consumeTimeoutMs.toDouble())
  }
}

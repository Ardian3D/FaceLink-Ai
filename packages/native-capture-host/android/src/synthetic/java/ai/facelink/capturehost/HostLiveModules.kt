package ai.facelink.capturehost

import com.facebook.react.bridge.NativeModule
import com.facebook.react.bridge.ReactApplicationContext

/**
 * Synthetic-only build: the FL-ENR02B live bridge is not compiled because the installed
 * `react-native-vision-camera-face-detector` does not carry the patched CAM02 source set.
 * JavaScript receives `E_CAPTURE_LIVE_UNAVAILABLE` when it asks for `FaceLinkCaptureLive`.
 */
internal fun hostLiveModules(@Suppress("UNUSED_PARAMETER") context: ReactApplicationContext): List<NativeModule> =
  emptyList()

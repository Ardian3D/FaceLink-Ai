package ai.facelink.capturehost

import com.facebook.react.bridge.NativeModule
import com.facebook.react.bridge.ReactApplicationContext

/**
 * Live build: the patched CAM02 detector provides `Cam02FrameStore`, so the FL-ENR02B bridge is
 * registered next to the untouched synthetic FL-CAM03 module.
 */
internal fun hostLiveModules(context: ReactApplicationContext): List<NativeModule> =
  listOf(FaceLinkCaptureLiveModule(context))

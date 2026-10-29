package ai.facelink.capturehost

import com.facebook.react.ReactPackage
import com.facebook.react.bridge.NativeModule
import com.facebook.react.bridge.ReactApplicationContext
import com.facebook.react.uimanager.ViewManager

/**
 * The single autolinked package of this library.
 *
 * React Native autolinking instantiates exactly one package instance per npm dependency, so the
 * live FL-ENR02B module is contributed from here rather than from a second package class.
 * `hostLiveModules` is compiled from `src/live/java` only when the CAM02 detector source set is
 * present; synthetic-only builds compile the `src/synthetic/java` no-op variant instead.
 */
class FaceLinkCaptureHostPackage : ReactPackage {
  override fun createNativeModules(context: ReactApplicationContext): List<NativeModule> =
    listOf<NativeModule>(FaceLinkCaptureHostModule(context)) + hostLiveModules(context)

  override fun createViewManagers(context: ReactApplicationContext): List<ViewManager<*, *>> = emptyList()
}

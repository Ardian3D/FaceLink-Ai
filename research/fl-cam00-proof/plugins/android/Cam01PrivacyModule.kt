package ai.facelink.cam00proof

import android.os.Build
import android.system.Os
import android.system.OsConstants
import android.view.WindowManager
import com.facebook.react.bridge.Arguments
import com.facebook.react.bridge.Promise
import com.facebook.react.bridge.ReactApplicationContext
import com.facebook.react.bridge.ReactContextBaseJavaModule
import com.facebook.react.bridge.ReactMethod
import java.io.File
import java.util.ArrayDeque

class Cam01PrivacyModule(context: ReactApplicationContext) : ReactContextBaseJavaModule(context) {
  override fun getName() = "Cam01Privacy"

  @ReactMethod
  fun setScanningSecure(@Suppress("UNUSED_PARAMETER") active: Boolean, promise: Promise) {
    val activity = reactApplicationContext.currentActivity
    if (activity == null) {
      promise.reject("CAM01_NO_ACTIVITY", "Privacy protection requires an attached activity.")
      return
    }
    activity.runOnUiThread {
      try {
        // The isolated proof intentionally remains secure even after scanning stops.
        activity.window.addFlags(WindowManager.LayoutParams.FLAG_SECURE)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
          activity.setRecentsScreenshotEnabled(false)
        }
        promise.resolve(null)
      } catch (_: Exception) {
        promise.reject("CAM01_SECURE_UNAVAILABLE", "Cannot apply activity privacy protection.")
      }
    }
  }

  @ReactMethod
  fun getStorageInventory(promise: Promise) {
    try {
      val context = reactApplicationContext
      val dataDir = File(context.applicationInfo.dataDir)
      val roots = arrayOf(
        "files" to context.filesDir,
        "cache" to context.cacheDir,
        "codeCache" to context.codeCacheDir,
        "noBackup" to context.noBackupFilesDir,
        "databases" to File(dataDir, "databases"),
        "sharedPrefs" to File(dataDir, "shared_prefs")
      )
      val files = Arguments.createArray()
      val header = ByteArray(12)
      var totalBytes = 0L
      for ((label, directory) in roots) {
        // SDK metadata directories may not exist yet; only ENOENT is optional.
        val rootMetadata = try {
          Os.lstat(directory.path)
        } catch (error: android.system.ErrnoException) {
          if (error.errno == OsConstants.ENOENT) continue
          throw error
        }
        check(OsConstants.S_ISDIR(rootMetadata.st_mode)) { "Inventory root is not a directory" }
        val root = directory.canonicalFile
        val pending = ArrayDeque<Pair<File, String>>()
        pending.add(root to label)
        while (pending.isNotEmpty()) {
          val (entry, relativePath) = pending.removeLast()
          // lstat never follows a symbolic link; no external directories are traversed.
          val metadata = Os.lstat(entry.path)
          check(!OsConstants.S_ISLNK(metadata.st_mode)) { "Symbolic link outside inventory contract" }
          if (OsConstants.S_ISDIR(metadata.st_mode)) {
            val children = entry.listFiles() ?: error("Cannot enumerate private directory")
            for (child in children) {
              pending.add(child to "$relativePath/${child.name}")
            }
          } else if (OsConstants.S_ISREG(metadata.st_mode)) {
            files.pushMap(Arguments.createMap().apply {
              putString("path", relativePath)
              putDouble("bytes", metadata.st_size.toDouble())
              putDouble("modifiedAt", metadata.st_mtime.toDouble() * 1000)
              val type = imageType(entry, header)
              if (type == null) putNull("imageType") else putString("imageType", type)
            })
            totalBytes += metadata.st_size
          } else {
            error("Unsupported private storage entry")
          }
        }
      }
      promise.resolve(Arguments.createMap().apply {
        putArray("files", files)
        putDouble("totalBytes", totalBytes.toDouble())
      })
    } catch (_: Exception) {
      // Never return an empty/safe-looking partial inventory after an error or race.
      promise.reject("CAM01_INVENTORY_UNAVAILABLE", "Cannot enumerate all requested private storage metadata.")
    }
  }

  private fun imageType(file: File, header: ByteArray): String? {
    // Read only the first 12 signature bytes; never decode pixels or return content.
    val count = java.io.FileInputStream(file).use { stream ->
      var length = 0
      while (length < header.size) {
        val read = stream.read(header, length, header.size - length)
        if (read < 0) break
        length += read
      }
      length
    }
    fun byte(index: Int) = header[index].toInt() and 0xff
    return when {
      count >= 8 && byte(0) == 0x89 && byte(1) == 0x50 && byte(2) == 0x4e &&
        byte(3) == 0x47 && byte(4) == 0x0d && byte(5) == 0x0a &&
        byte(6) == 0x1a && byte(7) == 0x0a -> "png"
      count >= 3 && byte(0) == 0xff && byte(1) == 0xd8 && byte(2) == 0xff -> "jpeg"
      count >= 12 && byte(0) == 0x52 && byte(1) == 0x49 && byte(2) == 0x46 &&
        byte(3) == 0x46 && byte(8) == 0x57 && byte(9) == 0x45 &&
        byte(10) == 0x42 && byte(11) == 0x50 -> "webp"
      else -> null
    }
  }
}

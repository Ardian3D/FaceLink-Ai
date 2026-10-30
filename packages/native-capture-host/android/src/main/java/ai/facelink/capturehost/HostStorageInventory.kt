package ai.facelink.capturehost

import android.content.Context
import android.os.Build
import android.os.ParcelFileDescriptor
import android.os.SystemClock
import android.system.Os
import android.system.OsConstants
import com.facebook.react.bridge.Arguments
import com.facebook.react.bridge.WritableMap
import java.io.File
import java.nio.file.Files
import java.util.Locale

/** Stat-only by default. Preferences, databases, auth/session stores and videos are NEVER opened. */
internal class HostStorageInventory(private val context: Context) {
  private val entries = Arguments.createArray()
  private val warnings = linkedSetOf<String>()
  private var truncated = false
  private var signatureBytesRead = 0
  private var signatureFiles = 0
  private var visited = 0
  private val until = SystemClock.elapsedRealtime() + MAX_DURATION_MS

  fun collect(): WritableMap {
    val sandbox = File(context.applicationInfo.dataDir).canonicalFile
    for (root in ROOTS) {
      if (exhausted()) break
      val directory = File(sandbox, root)
      try {
        // Never follow a substituted/symlink root into another storage area.
        val stat = Os.lstat(directory.path)
        if (!OsConstants.S_ISDIR(stat.st_mode)) {
          warnings.add("E_INVENTORY_ROOT_TYPE_$root")
          continue
        }
        visit(root, directory, directory, ".", 0)
      } catch (_: Throwable) {
        warnings.add("E_INVENTORY_ROOT_UNAVAILABLE_$root")
      }
    }
    return Arguments.createMap().apply {
      putString("scope", "app_private_metadata_only")
      putDouble("capturedAtMs", SystemClock.elapsedRealtime().toDouble())
      putInt("maxEntries", MAX_ENTRIES)
      putInt("maxDepth", MAX_DEPTH)
      putInt("maxSignatureBytes", SIGNATURE_BYTES)
      putBoolean("truncated", truncated)
      putInt("signatureBytesRead", signatureBytesRead)
      putArray("entries", entries)
      putArray("warnings", Arguments.createArray().apply { for (warning in warnings.take(16)) pushString(warning) })
    }
  }

  private fun exhausted(): Boolean {
    if (entries.size() >= MAX_ENTRIES || visited >= MAX_VISITED || SystemClock.elapsedRealtime() >= until) {
      truncated = true
      return true
    }
    return false
  }

  private fun visit(root: String, base: File, file: File, relative: String, depth: Int) {
    if (exhausted()) return
    visited++
    if (relative.length > MAX_NAME_LENGTH || relative.any { it.code < 32 || it.code == 127 }) {
      truncated = true
      warnings.add("E_INVENTORY_NAME_OMITTED")
      return
    }
    val stat = try { Os.lstat(file.path) } catch (_: Throwable) {
      warnings.add("E_INVENTORY_STAT_$root")
      return
    }
    val kind = when {
      OsConstants.S_ISLNK(stat.st_mode) -> "symlink"
      OsConstants.S_ISDIR(stat.st_mode) -> "directory"
      OsConstants.S_ISREG(stat.st_mode) -> "file"
      else -> "other"
    }
    val signature = if (kind == "file" && safeImageCandidate(root, relative)) {
      readSignature(base, file, stat.st_dev, stat.st_ino, stat.st_size)
    } else null
    entries.pushMap(Arguments.createMap().apply {
      putString("root", root)
      putString("name", relative)
      putString("kind", kind)
      putDouble("bytes", stat.st_size.toDouble())
      putDouble("modifiedAtMs", stat.st_mtime.toDouble() * 1000)
      if (signature == null) putNull("signature") else putString("signature", signature)
    })
    if (kind != "directory") return
    if (depth >= MAX_DEPTH || ((root == "databases" || root == "shared_prefs") && depth >= 1)) {
      truncated = true
      return
    }
    if (Build.VERSION.SDK_INT < 26) {
      // Do not fall back to unbounded File.listFiles() allocation on older Android versions.
      truncated = true
      warnings.add("E_INVENTORY_DIRECTORY_API")
      return
    }
    try {
      // Streaming iteration: bounded work/output, no unbounded array of sandbox filenames.
      Files.newDirectoryStream(file.toPath()).use { children ->
        for (child in children) {
          if (exhausted()) break
          val name = child.fileName.toString()
          val nested = if (relative == ".") name else "$relative/$name"
          val entry = child.toFile()
          if (!within(base, entry)) {
            // Symlinks are still statted and reported, never followed/traversed/read.
            val link = try { OsConstants.S_ISLNK(Os.lstat(entry.path).st_mode) } catch (_: Throwable) { false }
            if (!link) { warnings.add("E_INVENTORY_OUTSIDE_ROOT"); continue }
          }
          visit(root, base, entry, nested, depth + 1)
        }
      }
    } catch (_: Throwable) {
      warnings.add("E_INVENTORY_DIRECTORY_$root")
    }
  }

  private fun within(base: File, file: File): Boolean = try {
    val root = base.absolutePath
    val path = file.canonicalPath
    path == root || path.startsWith(root + File.separator)
  } catch (_: Throwable) { false }

  private fun safeImageCandidate(root: String, relative: String): Boolean {
    if (root != "cache" || signatureFiles >= MAX_SIGNATURE_FILES) return false
    val name = relative.lowercase(Locale.ROOT)
    // Explicit image-cache directories only; arbitrary cache/auth files are stat-only, even if renamed .jpg.
    val parts = name.split('/')
    if (parts.dropLast(1).none { it in IMAGE_CACHE_DIRECTORIES }) return false
    if (SENSITIVE_NAMES.any { name.contains(it) }) return false
    return IMAGE_EXTENSIONS.any { name.endsWith(it) }
  }

  private fun readSignature(base: File, file: File, device: Long, inode: Long, size: Long): String? {
    if (!within(base, file)) return null
    signatureFiles++
    val header = ByteArray(SIGNATURE_BYTES)
    var count = 0
    try {
      val descriptor = Os.open(file.path, OsConstants.O_RDONLY or OsConstants.O_CLOEXEC or OsConstants.O_NOFOLLOW, 0)
      try {
        val opened = Os.fstat(descriptor)
        // Reject a raced replacement before any content read; no following symlink payloads.
        if (!OsConstants.S_ISREG(opened.st_mode) || opened.st_dev != device || opened.st_ino != inode || opened.st_size != size) return null
        // Pin and inspect the opened descriptor's actual location as well as inode identity.
        // An ancestor-directory swap must not redirect this read into preferences/databases.
        ParcelFileDescriptor.dup(descriptor).use { pinned ->
          val location = Os.readlink("/proc/self/fd/${pinned.fd}")
          if (!location.startsWith(base.absolutePath + File.separator) ||
            SENSITIVE_NAMES.any { location.lowercase(Locale.ROOT).contains(it) }) return null
          count = Os.read(pinned.fileDescriptor, header, 0, header.size)
          if (count < 0) count = 0
          signatureBytesRead += count
        }
      } finally { Os.close(descriptor) }
      fun b(index: Int) = header[index].toInt() and 255
      return when {
        count >= 3 && b(0) == 255 && b(1) == 216 && b(2) == 255 -> "jpeg"
        count >= 8 && b(0) == 137 && b(1) == 80 && b(2) == 78 && b(3) == 71 && b(4) == 13 && b(5) == 10 && b(6) == 26 && b(7) == 10 -> "png"
        count >= 6 && b(0) == 71 && b(1) == 73 && b(2) == 70 && b(3) == 56 && (b(4) == 55 || b(4) == 57) && b(5) == 97 -> "gif"
        count >= 12 && b(0) == 82 && b(1) == 73 && b(2) == 70 && b(3) == 70 && b(8) == 87 && b(9) == 69 && b(10) == 66 && b(11) == 80 -> "webp"
        else -> null
      }
    } catch (_: Throwable) {
      warnings.add("E_INVENTORY_IMAGE_SIGNATURE")
      return null
    } finally { header.fill(0) }
  }

  private companion object {
    const val MAX_ENTRIES = 256
    const val MAX_VISITED = 512
    const val MAX_DEPTH = 4
    const val MAX_NAME_LENGTH = 240
    const val MAX_DURATION_MS = 1_500L
    const val SIGNATURE_BYTES = 16
    const val MAX_SIGNATURE_FILES = 32
    val ROOTS = listOf("files", "cache", "code_cache", "no_backup", "databases", "shared_prefs")
    val IMAGE_CACHE_DIRECTORIES = setOf("image_cache", "small_image_cache", "image-cache", "expo-image", "expo-image-cache", "fresco", "coil_image_cache", "image_manager_disk_cache")
    val IMAGE_EXTENSIONS = listOf(".jpg", ".jpeg", ".png", ".gif", ".webp")
    val SENSITIVE_NAMES = listOf("secure", "secret", "token", "credential", "session", "cookie", "auth", "keystore", "preferences", "database")
  }
}

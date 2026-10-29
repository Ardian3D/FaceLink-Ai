const fs = require('node:fs/promises');
const path = require('node:path');
const os = require('node:os');
const { applyCam02 } = require('./detector/patch-detector.cjs');
const {
  AndroidConfig,
  withAndroidManifest,
  withAppBuildGradle,
  withDangerousMod,
  withGradleProperties,
  withMainActivity,
  withMainApplication,
} = require('expo/config-plugins');

const PACKAGE = 'ai.facelink.cam00proof';
const CMAKE_VERSION = '3.31.6';
const RELEASE_MANIFEST = `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools">
  <!-- Variant-specific: debug retains Metro networking and its dev manifest. -->
  <uses-permission android:name="android.permission.INTERNET" tools:node="remove" />
  <uses-permission android:name="android.permission.ACCESS_NETWORK_STATE" tools:node="remove" />
  <uses-permission android:name="android.permission.ACCESS_LOCAL_NETWORK" tools:node="remove" />
  <uses-permission android:name="android.permission.VIBRATE" tools:node="remove" />
  <uses-permission android:name="android.permission.SYSTEM_ALERT_WINDOW" tools:node="remove" />
  <uses-permission android:name="android.permission.READ_EXTERNAL_STORAGE" tools:node="remove" />
  <uses-permission android:name="android.permission.WRITE_EXTERNAL_STORAGE" tools:node="remove" />
  <uses-permission android:name="android.permission.MANAGE_EXTERNAL_STORAGE" tools:node="remove" />
  <uses-permission android:name="android.permission.READ_MEDIA_IMAGES" tools:node="remove" />
  <uses-permission android:name="android.permission.READ_MEDIA_VIDEO" tools:node="remove" />
  <uses-permission android:name="android.permission.READ_MEDIA_AUDIO" tools:node="remove" />
  <application android:usesCleartextTraffic="false" android:allowBackup="false" android:fullBackupContent="false" android:dataExtractionRules="@xml/cam01_data_extraction_rules" tools:replace="android:usesCleartextTraffic,android:allowBackup,android:fullBackupContent,android:dataExtractionRules" />
</manifest>
`;
// ML Kit discovers these classes by manifest strings; R8 cannot infer constructors.
const MLKIT_REGISTRAR_RULES = `# CAM01 ML Kit manifest-discovered registrars
-keep class com.google.mlkit.common.internal.CommonComponentRegistrar { public <init>(); }
-keep class com.google.mlkit.vision.common.internal.VisionCommonRegistrar { public <init>(); }
-keep class com.google.mlkit.vision.face.internal.FaceRegistrar { public <init>(); }
`;
const BACKUP_DOMAINS = ['root', 'file', 'database', 'sharedpref', 'external', 'device_root', 'device_file', 'device_database', 'device_sharedpref'];
const exclusions = BACKUP_DOMAINS.map(domain => `    <exclude domain="${domain}" path="." />`).join('\n');
const DATA_EXTRACTION_RULES = `<?xml version="1.0" encoding="utf-8"?>
<data-extraction-rules>
  <cloud-backup>
${exclusions}
  </cloud-backup>
  <device-transfer>
${exclusions}
  </device-transfer>
</data-extraction-rules>
`;
const GRADLE_POLICY = `// CAM01 privacy policy begin
android {
    buildTypes {
        release {
            debuggable false
            jniDebuggable false
            ndk {
                abiFilters.clear()
                abiFilters.add("arm64-v8a")
            }
        }
    }
}
// Only debug skips the ordinary Expo export:embed/Hermes release bundle tasks.
react { debuggableVariants = ["debug"] }
// CAM01 privacy policy end`;

function insertOnce(contents, anchor, addition, marker) {
  if (contents.includes(marker)) return contents;
  if (contents.split(anchor).length !== 2) throw new Error(`CAM01: unsupported native template at ${marker}`);
  return contents.replace(anchor, `${anchor}${addition}`);
}

function readProperty(contents, key) {
  const line = contents.split(/\r?\n/).find(value => value.startsWith(`${key}=`));
  return line?.slice(key.length + 1).replace(/\\([\\:= ])/g, '$1');
}

async function configureLocalCmake(androidRoot) {
  const file = path.join(androidRoot, 'local.properties');
  let contents;
  try {
    contents = await fs.readFile(file, 'utf8');
  } catch (error) {
    if (error.code !== 'ENOENT') throw error;
    contents = '';
  }
  // Preserve the existing local override when available; never commit SDK/user paths.
  const existing = readProperty(contents, 'cmake.dir');
  const sdk = readProperty(contents, 'sdk.dir') || process.env.ANDROID_HOME || process.env.ANDROID_SDK_ROOT ||
    (process.platform === 'win32' && process.env.LOCALAPPDATA
      ? path.join(process.env.LOCALAPPDATA, 'Android', 'Sdk')
      : path.join(os.homedir(), process.platform === 'darwin' ? 'Library/Android/sdk' : 'Android/Sdk'));
  const cmake = existing && path.basename(existing) === CMAKE_VERSION ? existing : path.join(sdk, 'cmake', CMAKE_VERSION);
  try {
    await fs.access(path.join(cmake, 'bin', process.platform === 'win32' ? 'cmake.exe' : 'cmake'));
  } catch {
    throw new Error(`CAM01: install Android SDK CMake ${CMAKE_VERSION}, or set a matching cmake.dir in ignored android/local.properties.`);
  }
  const escaped = cmake.replaceAll('\\', '/').replaceAll(':', '\\:');
  const retained = contents.split(/\r?\n/).filter(line => !line.startsWith('cmake.dir=')).join('\n').trimEnd();
  await fs.writeFile(file, `${retained}${retained ? '\n' : ''}cmake.dir=${escaped}\n`);
}

module.exports = function withCam01Privacy(config) {
  if (config.android?.package !== PACKAGE) throw new Error('CAM01: plugin is restricted to the isolated prototype package.');
  config = withAndroidManifest(config, mod => {
    const app = AndroidConfig.Manifest.getMainApplicationOrThrow(mod.modResults);
    Object.assign(app.$, {
      'android:allowBackup': 'false',
      'android:fullBackupContent': 'false',
      'android:dataExtractionRules': '@xml/cam01_data_extraction_rules',
      'android:usesCleartextTraffic': 'false',
    });
    const activity = AndroidConfig.Manifest.getMainActivityOrThrow(mod.modResults);
    Object.assign(activity.$, { 'android:screenOrientation': 'portrait', 'android:resizeableActivity': 'false' });
    // Android 16 large-screen orientation opt-out, scoped only to this activity/app.
    app.property = (app.property || []).filter(item => item.$['android:name'] !== 'android.window.PROPERTY_COMPAT_ALLOW_RESTRICTED_RESIZABILITY');
    app.property.push({ $: { 'android:name': 'android.window.PROPERTY_COMPAT_ALLOW_RESTRICTED_RESIZABILITY', 'android:value': 'true' } });
    return mod;
  });
  config = withMainApplication(config, mod => {
    if (mod.modResults.language !== 'kt') throw new Error('CAM01 requires the Expo Kotlin application template.');
    mod.modResults.contents = insertOnce(mod.modResults.contents, 'PackageList(this).packages.apply {', '\n          add(Cam01PrivacyPackage())', 'add(Cam01PrivacyPackage())');
    return mod;
  });
  config = withMainActivity(config, mod => {
    if (mod.modResults.language !== 'kt') throw new Error('CAM01 requires the Expo Kotlin activity template.');
    const anchor = '    super.onCreate(null)';
    if (!mod.modResults.contents.includes('// CAM01 activity privacy')) {
      if (mod.modResults.contents.split(anchor).length !== 2) throw new Error('CAM01: unsupported Activity onCreate template.');
      mod.modResults.contents = mod.modResults.contents.replace(anchor, `    // CAM01 activity privacy: secure before React/camera content, including recents.
    window.addFlags(android.view.WindowManager.LayoutParams.FLAG_SECURE)
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
      setRecentsScreenshotEnabled(false)
    }
${anchor}`);
    }
    return mod;
  });
  config = withAppBuildGradle(config, mod => {
    if (mod.modResults.language !== 'groovy') throw new Error('CAM01 requires the Expo Groovy Gradle template.');
    const start = mod.modResults.contents.indexOf('// CAM01 privacy policy begin');
    if (start >= 0) {
      const endMarker = '// CAM01 privacy policy end';
      const end = mod.modResults.contents.indexOf(endMarker, start);
      if (end < 0) throw new Error('CAM01: incomplete generated Gradle privacy policy.');
      mod.modResults.contents = mod.modResults.contents.slice(0, start) + GRADLE_POLICY + mod.modResults.contents.slice(end + endMarker.length);
    } else {
      mod.modResults.contents += `\n${GRADLE_POLICY}\n`;
    }
    return mod;
  });
  config = withGradleProperties(config, mod => {
    for (const [key, value] of Object.entries({ reactNativeArchitectures: 'arm64-v8a', EX_DEV_CLIENT_NETWORK_INSPECTOR: 'false' })) {
      mod.modResults = mod.modResults.filter(item => item.type !== 'property' || item.key !== key);
      mod.modResults.push({ type: 'property', key, value });
    }
    return mod;
  });
  return withDangerousMod(config, ['android', async mod => {
    const androidRoot = mod.modRequest.platformProjectRoot;
    const javaRoot = path.join(androidRoot, 'app/src/main/java', ...PACKAGE.split('.'));
    await fs.mkdir(javaRoot, { recursive: true });
    await fs.rm(path.join(javaRoot, 'Cam02CaptureModule.kt'), { force: true });
    for (const name of ['Cam01PrivacyModule.kt', 'Cam01PrivacyPackage.kt', 'FaceLinkCaptureModule.kt']) {
      await fs.copyFile(path.join(__dirname, 'android', name), path.join(javaRoot, name));
    }
    const releaseRoot = path.join(androidRoot, 'app/src/release');
    const xmlRoot = path.join(androidRoot, 'app/src/main/res/xml');
    await fs.mkdir(releaseRoot, { recursive: true });
    await fs.mkdir(xmlRoot, { recursive: true });
    await fs.writeFile(path.join(releaseRoot, 'AndroidManifest.xml'), RELEASE_MANIFEST);
    await fs.writeFile(path.join(xmlRoot, 'cam01_data_extraction_rules.xml'), DATA_EXTRACTION_RULES);
    const proguard = path.join(androidRoot, 'app/proguard-rules.pro');
    const rules = await fs.readFile(proguard, 'utf8');
    if (!rules.includes('# CAM01 ML Kit manifest-discovered registrars')) {
      await fs.writeFile(proguard, `${rules}\n${MLKIT_REGISTRAR_RULES}`);
    }
    await configureLocalCmake(androidRoot);
    await applyCam02(mod.modRequest.projectRoot);
    return mod;
  }]);
};

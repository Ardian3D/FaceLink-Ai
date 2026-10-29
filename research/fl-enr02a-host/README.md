# FL-ENR02A — Staging KYC Enrollment UI

Status: **synthetic-only staging UI and disposable backend integration**. Not a
production enrollment feature, not human capture, and not a claim of face
recognition accuracy.

## Isolation

- Source: actual `apps/mobile`, copied by `prepare-enr02a.cjs`; main APK and main
  Android package are never touched.
- Staging package: `ai.facelink.enr02host`; scheme `facelinkenr02`; label
  `FaceLink ENR02A Staging`.
- Main dashboard shows an enrollment entry only when
  `EXPO_PUBLIC_FACELINK_ENR_STAGING=1`. The enrollment route exists only in the
  generated staging tree; a main build has no such route.
- Default synthetic staging blocks camera, storage/media and overlay permissions. No camera module is added unless the explicit mannequin build is prepared.
- `MANNEQUIN_PREVIEW_ONLY` is a separate explicit staging build: CAMERA permission, VisionCamera/detector/host dependencies, live scanner template and FLAG_SECURE are added only to the generated staging tree. Main APK remains without route, camera permission, or camera dependencies.
- SecureStore code is unchanged. Separate Android package means separate app
  storage/Keystore namespace; main session data is not read or cleared.
- Backend: `cmd/enrollment-staging-serve` uses a fresh temp directory, SQLite
  database, disposable synthetic migrations, in-memory keys, synthetic accounts
  and a non-model processor. `cmd/server` is unchanged and enrollment remains
  OFF in the main backend.
- No Product Owner database, real account, SMTP/DNS, migration, camera, gallery,
  file capture, or human face data is used.

## FL-ENR06 — pinned positive fixture for Android acceptance

The main app and the default staging build still use the small procedural
pattern. For an explicitly authorized device acceptance run, preparation can
bake **one existing synthetic JPEG that already passed the pinned real
YuNet/SFace pipeline in FL-ENR04-R2** into the generated staging tree:

```powershell
$env:EXPO_PUBLIC_FACELINK_API_BASE_URL = "https://127.0.0.1:8091"
$env:EXPO_PUBLIC_FACELINK_ENR_STAGING = "1"
$env:EXPO_PUBLIC_FACELINK_ADMIN_ENR_STAGING = "1"
$env:EXPO_PUBLIC_FACELINK_CAPTURE_MODE = "synthetic_flow"
$env:FACELINK_ENR06_FIXTURE = "1"
node research/fl-enr02a-host/prepare-enr02a.cjs
```

- `FACELINK_ENR06_FIXTURE=1` is a preparation-only flag; it is not written to
  the app environment and does not enable a route at runtime.
- It requires `synthetic_flow` plus the staging Admin route, so no camera build
  and no CAMERA permission are involved.
- The JPEG must be the regular file at
  `research/fl-enr02a-host/evidence-local/enr04-r1/fixtures-v1-verified/procedural-mannequin-heightfield-v1-01.jpg`
  with SHA-256 `cf572a3ef381f4b52087c9dac062d0af4da6ca352b1e55742674a1c308e6cabe`.
  Missing file, symbolic link, non-regular file, size outside bounds, or hash
  mismatch aborts preparation.
- The verified bytes are embedded only in the generated, Git-ignored
  `stage/src/features/enrollment/fixture.ts`; provenance (kind, path, hash,
  bytes, camera false) is recorded in `enr02a-source-manifest.json`. No image is
  added to tracked source, a gallery, or an external service.
- This preparation and the build are **offline**. They do not authorize ADB,
  device installation, camera, or a human-face run; those need a new explicit
  Product Owner approval for each device session.

The Student pending panel now derives its retention countdown from the
authenticated `server_now` response, not the device clock; a missing or invalid
server time shows no invented deadline.

FL-ENR06-R1 subsequently exercised one Android Student/Admin chain using two
authorized packages while keeping the Student session alive. The positive
chain passed through real Python/AADv2 ACTIVE and withdrawal/cleanup; overall
acceptance remains PARTIAL because a native dev-inspector crash was observed.
The complete device gates, audited APKs, local ports, final submission trace,
test tooling and cleanup limits are recorded in
[`FL_ENR06_R1_TECHNICAL_REPORT.md`](../../docs/FL_ENR06_R1_TECHNICAL_REPORT.md).
Do not reuse an earlier ADB/install approval or overwrite old evidence.

## Start the disposable backend

From `backend`:

```powershell
go run ./cmd/enrollment-staging-serve -addr 127.0.0.1:8091
```

The command prints startup JSON containing the HTTPS base URL, synthetic student
and admin email addresses, disposable root, and an `otp_file` path. TLS uses the
repository's development certificate (SAN includes `127.0.0.1`) and the staging
debug app trusts the development CA. The OTP code is written only into the
disposable root; it is never printed or logged. The backend worker processes
approved synthetic submissions automatically.

For the automated cross-stack test only, `-insecure-http -test-tokens` is
available on loopback. It emits fixture access tokens in startup JSON and must
not be used for a device/manual run. This is a test harness, not an app login
bypass.

In test-token mode, stdin EOF gracefully stops the real Go server (including
when launched through `go run` on Windows). The E2E runner closes stdin and
awaits exit on success or readiness failure. Normal/manual runs stop with
Ctrl+C. Initialization errors return through cleanup before the command exits.

## Prepare and run the staging app

From the repository root:

```powershell
$env:EXPO_PUBLIC_FACELINK_API_BASE_URL = "https://127.0.0.1:8091"
node research/fl-enr02a-host/prepare-enr02a.cjs
cd research/fl-enr02a-host/stage
npm install --offline --ignore-scripts --no-audit --no-fund
npm start
```

For the camera-source build (not permission to run it):

```powershell
$env:EXPO_PUBLIC_FACELINK_API_BASE_URL = "https://127.0.0.1:8091"
$env:EXPO_PUBLIC_FACELINK_CAPTURE_MODE = "mannequin_preview_only"
node research/fl-enr02a-host/prepare-enr02a.cjs
cd research/fl-enr02a-host/stage
npm install --offline --ignore-scripts --no-audit --no-fund
node ../apply-detector-live.cjs
```

`apply-detector-live.cjs` patches the installed detector 2.1.0 with the CAM00
CAM02 source set. It must run after install and before the Android build; it is
idempotent. A mannequin build sets CAMERA and FLAG_SECURE only in staging.

For FL-ENR02B-R2 metadata diagnostics, set
`EXPO_PUBLIC_FACELINK_CAPTURE_TRACE=1` before preparation. Optional
`EXPO_PUBLIC_FACELINK_CAPTURE_PROBES=1` exercises wrong-generation consume before
the valid consume, then duplicate consume after it; it never logs the private
consume capability. Both flags default to `0`, are effective only in mannequin
staging, and tracing additionally requires a debug JavaScript runtime.
The generated trace module is not imported by the main app.

`ENR02B_R2` records contain whitelisted numeric geometry, safe frame/generation
identifiers, guidance/dwell metadata and capture/consume/drain observations.
Limits per module instance: one geometry record/second, at most 90 geometry and
64 transition records; movement/reset records are capped at 16 to preserve
terminal evidence capacity. No JPEG, base64, raw pixels, paths, OTP, session
token or opaque capture token is emitted. A `RELEASED` observation means native
status reported `busy=false`, `heldFrames=0`, `ownedBytes=0`; it is not a claim
of total process memory zeroization or zero telemetry. Capture/consume/release
counters are local diagnostic observations, not process-wide native counters.
Re-prepare without the flags to disable diagnostics. No permission to use ADB
or start a camera is implied by these flags.

`detectorBoundsSensor` uses CameraX sensor-active-array coordinates;
`analysisImage` uses the rotation-applied ML Kit dimensions, and `buffer` uses
the native analysis buffer dimensions. These are different coordinate spaces:
do not divide sensor bounds by ML Kit dimensions. Compare
`previewBoxNormalized` against `roiNormalized` in the same preview space.
`roundTripError` measures inverse-transform consistency only; matching forward
and inverse transforms do not independently prove absolute scale accuracy.

For a physical Android device, reverse the loopback port before opening the app:

```powershell
adb reverse tcp:8091 tcp:8091
```

Log in through the existing email-OTP screen with
`student.synthetic@campus.example`. Read the 8-digit code from the `otp_file`
path printed by the server. Do not use a real account or production database.
Navigation: student dashboard → `Persetujuan biometrik` → consent → synthetic
guided scan → waiting/rejected/expired/active. Default mode never opens a camera.
Mannequin source mode requires a new device permission gate before any camera run.

No ADB command, install, camera start, or device test is part of the
offline preparation script. A camera/mannequin run requires explicit new permission.

For Admin review, prepare with `EXPO_PUBLIC_FACELINK_ADMIN_ENR_STAGING=1`
and `EXPO_PUBLIC_FACELINK_CAPTURE_MODE=synthetic_flow`, then run the disposable
backend with `-admin-fixtures`. Log in normally as `admin.synthetic@campus.example`.
The five-second poll refreshes detail views only; queue pages remain loaded
until explicit refresh or foreground-resume refresh. Approval references accept
uppercase UUID input and are canonicalized to lowercase before confirmation.
Android acceptance requires new explicit device permission before any ADB;
this mode never opens a camera. Native bitmap-cache zeroization is not guaranteed.

## Checks

Main app:

```powershell
cd apps/mobile
npm run typecheck
npm run lint
npm test
```

Staging copies the same tests plus the generated route:

```powershell
cd research/fl-enr02a-host/stage
npm run typecheck
npm run lint
npm test
# opt-in real Go server + real mobile client:
$env:FACELINK_ENR_E2E = "1"
npx vitest run tests/enrollmentE2E.test.ts
```

`app.json`, Android application ID, scheme, permissions, `.env` and route are
generated; `stage/` is ignored by Git. Rerun the prepare script after changing
the actual mobile source. The script refuses a nonempty stage without its
ownership sentinel and preserves build artifacts.

## Privacy and limits

- The synthetic JPEG is a generated test pattern, held only in memory and sent
  once to the documented submission endpoint. It is not rendered, logged,
  stored, or placed in the gallery.
- No JPEG/pixel/base64/token is logged by the feature, transport, or backend
  staging command.
- The UI never derives ACTIVE or consent from local choice: an ACTIVE label is
  shown only for a fresh backend `ACTIVE` + `template_ready=true` + consent
  ACTIVE snapshot. Stale snapshots lose that label.
- Withdrawal stops local scanning/upload and displays the server-confirmed
  withdrawal; an already-authorized in-flight response cannot be recalled.
- No production processor, real YuNet/SFace model, human capture, PAD/liveness,
  accuracy claim, or production migration is part of this task.

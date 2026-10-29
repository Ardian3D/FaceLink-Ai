import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  AppState, NativeEventEmitter, NativeModules, PixelRatio, Pressable, SafeAreaView, ScrollView,
  StyleSheet, Text, useWindowDimensions, View, type LayoutChangeEvent,
} from 'react-native';
import { StatusBar } from 'expo-status-bar';
import {
  Camera, useCameraDevice, useCameraPermission,
  type CameraDevice, type CameraOutput, type CameraRef,
} from 'react-native-vision-camera';
import {
  createFaceDetectorOutput, useImageFaceDetector,
} from 'react-native-vision-camera-face-detector';
import {
  assessObservation, DWELL_MS, initialMachine, mapBoundsToView, MAX_GAP_MS, previewOval,
  reduceMachine, stableFixtureEvents, validSize,
  type Candidate, type MachineEvent, type Reason, type Rect, type Size,
} from './guidanceModel';
import { CaptureController } from './captureController';
import { CaptureAdapter, safeCaptureCode, type NativeCapturePort, type VerificationReport } from './nativeCaptureAdapter';

type Inventory = {
  files: { path: string; bytes: number; modifiedAt?: number; imageType?: 'png' | 'jpeg' | 'webp' | null }[];
  totalBytes: number;
};
type PrivacyBridge = {
  setScanningSecure(active: boolean): Promise<void>;
  getStorageInventory(): Promise<Inventory>;
};
const privacyBridge = NativeModules.Cam01Privacy as PrivacyBridge | undefined;
type FrameMetadata = {
  generation: number;
  frameId: string;
  acquiredAtMs: number;
  emittedAtMs: number;
  faces: {
    bounds: Rect;
    frameWidth: number;
    frameHeight: number;
    yawAngle: number;
    pitchAngle: number;
    rollAngle: number;
  }[];
};
const captureBridge = NativeModules.FaceLinkCapture as NativeCapturePort | undefined;

// Optional local mannequin file. Static detection never feeds live guidance.
const DUMMY_FILE_URI = 'file:///data/user/0/ai.facelink.cam00proof/files/dummy/mannequin.jpg';
const UI_INTERVAL_MS = 200;
const INVENTORY_PAGE_SIZE = 8;
const now = () => performance.now();

const messages: Record<Reason, string> = {
  OFF: 'Kamera mati. Mulai secara eksplisit.',
  WAIT_PREVIEW: 'Menyiapkan izin, proteksi layar, dan preview…',
  NO_FACE: 'Tidak ada wajah. Arahkan hanya ke mannequin / dummy.',
  MULTIPLE_FACES: 'Lebih dari satu wajah. Sisakan satu dummy saja.',
  INVALID_GEOMETRY: 'Koordinat belum valid; dwell dibatalkan.',
  OUTSIDE_OVAL: 'Seluruh kotak wajah harus berada di dalam oval.',
  TOO_FAR: 'Dekatkan dummy ke kamera.',
  YAW: 'Hadapkan dummy lurus ke kamera.',
  PITCH: 'Ratakan posisi dagu dummy.',
  ROLL: 'Sejajarkan kepala dummy.',
  HOLD_STILL: 'Tahan posisi dummy — mengukur stabilitas kontinu.',
  MOVEMENT: 'Posisi / pose bergerak; ulangi tahan posisi.',
  CALLBACK_GAP: 'Aliran deteksi terputus; dwell direset.',
  CLOCK_RESET: 'Waktu tidak kontinu; dwell direset.',
  LAYOUT_CHANGED: 'Ukuran preview berubah; dwell direset.',
  READY: 'Dwell terpenuhi; menunggu satu observasi valid berikutnya.',
  SIMULATED_ONLY: 'Persetujuan logis SIMULATED_CAPTURE; bukan bukti JPEG. Lihat status native terpisah.',
  OPERATOR_STOP: 'Dibatalkan operator. Mulai lagi secara eksplisit.',
  BACKGROUND: 'Dibatalkan saat background. Tidak restart otomatis.',
  PERMISSION_DENIED: 'Izin kamera tidak tersedia. Beri izin lalu coba lagi.',
  PRIVACY_UNAVAILABLE: 'Proteksi native tidak tersedia / gagal. Kamera tetap mati.',
  CAMERA_ERROR: 'Kamera tidak tersedia / gagal. Kamera mati; coba mulai lagi.',
  DETECTOR_ERROR: 'Detektor gagal. Kamera mati; coba mulai lagi.',
  CAPTURE_UNAVAILABLE: 'Bridge capture native tidak tersedia / gagal disiapkan. Kamera mati.',
  CAPTURE_ERROR: 'Encode / pemilihan frame dummy gagal. Tidak ada JPEG sukses yang diasumsikan.',
  PORTRAIT_REQUIRED: 'Gunakan orientasi portrait. Kamera tetap mati.',
  PREVIEW_STOPPED: 'Preview berhenti; mulai ulang secara eksplisit.',
};

type LiveCameraProps = {
  generation: number;
  device: CameraDevice;
  cameraRef: React.RefObject<CameraRef | null>;
  onReady(generation: number): void;
  onStopped(generation: number): void;
  onFailure(generation: number, reason: 'CAMERA_ERROR' | 'DETECTOR_ERROR'): void;
};

function LiveCamera(props: LiveCameraProps) {
  const callbacks = useRef(props);
  callbacks.current = props;
  const generation = useRef(props.generation).current;
  // Plugin 2.1.0's hook rest-options object invalidates its memo every render.
  // One factory output and output-array per explicitly started generation;
  // callbacks retain that generation even after native async completion/unmount.
  const output = useMemo(() => createFaceDetectorOutput({
    cameraFacing: 'front', mirrorMode: 'on', autoMode: true,
    outputResolution: 'preview', performanceMode: 'fast',
    runLandmarks: false, runContours: false, runClassifications: false,
    minFaceSize: 0.15, trackingEnabled: false,
    // Only tagged CaptureFrame events drive guidance; this plugin callback does not.
    onFacesDetected: () => {},
    onError: () => callbacks.current.onFailure(generation, 'DETECTOR_ERROR'),
  }), [generation]);
  const outputs = useMemo<CameraOutput[]>(() => [output], [output]);
  return (
    <Camera
      ref={props.cameraRef}
      style={StyleSheet.absoluteFill}
      device={props.device}
      isActive
      outputs={outputs}
      mirrorMode="on"
      orientationSource="interface"
      resizeMode="cover"
      implementationMode="compatible"
      onPreviewStarted={() => callbacks.current.onReady(generation)}
      onPreviewStopped={() => callbacks.current.onStopped(generation)}
      onError={() => callbacks.current.onFailure(generation, 'CAMERA_ERROR')}
    />
  );
}

export default function App() {
  const dimensions = useWindowDimensions();
  const portrait = dimensions.height >= dimensions.width;
  const device = useCameraDevice('front');
  const { hasPermission, canRequestPermission, requestPermission } = useCameraPermission();
  const [machine, setMachine] = useState(initialMachine);
  const machineRef = useRef(machine);
  const [session, setSession] = useState<number | null>(null);
  const activeGeneration = useRef<number | null>(null);
  const generation = useRef(0);
  const [starting, setStarting] = useState(false);
  const pendingStart = useRef(false);
  const mounted = useRef(true);
  const foreground = useRef(AppState.currentState === 'active');
  const camera = useRef<CameraRef>(null);
  const nativeListening = useRef(false);
  const [captureController] = useState(() => new CaptureController());
  const [captureAdapter] = useState(() => captureBridge ? new CaptureAdapter(captureBridge) : null);
  const [capture, setCapture] = useState(captureController.state);
  const [nativeStatus, setNativeStatus] = useState<Record<string, unknown> | null>(null);
  const [nativeDetail, setNativeDetail] = useState('Belum ada sesi native / JPEG.');
  const trace = useRef({ cycle: 0, startedAt: 0, seq: 0, captureTriggerCount: 0 });
  const [viewSize, setViewSize] = useState<Size>({ width: 0, height: 0 });
  const sizeRef = useRef(viewSize);
  const [boxes, setBoxes] = useState<Rect[]>([]);
  const [shownPose, setShownPose] = useState<Candidate | null>(null);
  const overlayRef = useRef<{ boxes: Rect[]; candidate: Candidate | null }>({ boxes: [], candidate: null });
  const [callbackCount, setCallbackCount] = useState(0);
  const callbackCounter = useRef(0);
  const [callbackRate, setCallbackRate] = useState(0);
  const rateWindow = useRef({ at: now(), count: 0 });
  const lastUi = useRef(-Infinity);
  const lastFrameId = useRef<string | null>(null);
  const [inventory, setInventory] = useState<Inventory | null>(null);
  const [inventoryStatus, setInventoryStatus] = useState('Belum diaudit; inventaris bukan bukti nol penulisan SDK.');
  const [inventoryPage, setInventoryPage] = useState(0);
  const [auditing, setAuditing] = useState(false);
  const auditBusy = useRef(false);
  const [imageTest, setImageTest] = useState('Belum diuji; hasil file terpisah dari live.');
  const [imageTesting, setImageTesting] = useState(false);
  const imageBusy = useRef(false);
  const verificationBusy = useRef(false);
  const [verifying, setVerifying] = useState(false);
  const [verification, setVerification] = useState<VerificationReport | null>(null);
  const [verificationDetail, setVerificationDetail] = useState('Belum dijalankan: fixture YUV sintetis → codec Android nyata, tanpa kamera.');
  const imageOptions = useMemo(() => ({
    performanceMode: 'accurate' as const, runLandmarks: false,
    runContours: false, runClassifications: false, minFaceSize: 0.1, trackingEnabled: false,
  }), []);
  const imageDetector = useImageFaceDetector(imageOptions);

  const publish = useCallback((force = false) => {
    if (!mounted.current) return;
    const at = now();
    if (!force && at - lastUi.current < UI_INTERVAL_MS) return;
    lastUi.current = at;
    const current = machineRef.current;
    setMachine(current);
    setBoxes(overlayRef.current.boxes);
    setShownPose(overlayRef.current.candidate);
    setCallbackCount(callbackCounter.current);
    const elapsed = at - rateWindow.current.at;
    if (elapsed >= 1000) {
      setCallbackRate(Math.round(((callbackCounter.current - rateWindow.current.count) * 1000) / elapsed));
      rateWindow.current = { at, count: callbackCounter.current };
    }
  }, []);

  const apply = useCallback((input: MachineEvent, force = false, frame?: FrameMetadata | { frameId: string }, nativeCode?: string) => {
    const previous = machineRef.current;
    const next = reduceMachine(previous, input);
    machineRef.current = next;
    if (input.type === 'START' && next !== previous) {
      trace.current = { cycle: input.generation, startedAt: input.at, seq: 0, captureTriggerCount: 0 };
      lastFrameId.current = null;
    }
    trace.current.seq += 1;
    if (frame) lastFrameId.current = frame.frameId;
    const transitioned = next.phase !== previous.phase || next.reason !== previous.reason
      || next.source !== previous.source || next.generation !== previous.generation;
    if (next !== previous && next.phase === 'SIMULATED_CAPTURE' && previous.phase !== next.phase) {
      trace.current.captureTriggerCount += 1;
    }
    // Trace is deliberately outside publish(): the 5 Hz UI and former 1 s log
    // throttle could coalesce CAPTURE_READY -> SIMULATED_CAPTURE entirely.
    if (transitioned) console.log(`ENR01A_EVENT ${JSON.stringify({
      cycle: trace.current.cycle, generation: next.generation, seq: trace.current.seq,
      event: input.type, frameId: frame?.frameId ?? lastFrameId.current,
      state: next.phase, reason: next.reason, source: next.source,
      clock: next.source === 'synthetic_fixture' ? 'virtual' : 'monotonic_js',
      count: next.faceCount, stableMs: next.stableMs, samples: next.samples,
      elapsedMs: Math.max(0, input.at - trace.current.startedAt),
      captureTriggerCount: trace.current.captureTriggerCount,
      nativeAcquireToEmitMs: frame && 'emittedAtMs' in frame ? frame.emittedAtMs - frame.acquiredAtMs : null,
      nativeCode: nativeCode ?? null,
    })}`);
    if (next.reason === 'CALLBACK_GAP') overlayRef.current = { boxes: [], candidate: null };
    publish(force || transitioned);
    return next;
  }, [publish]);

  const refreshNativeStatus = useCallback(async (token: number) => {
    if (!captureBridge) return;
    try {
      const result = await captureBridge.getStatus();
      if (mounted.current && foreground.current && generation.current === token) setNativeStatus(result);
    } catch {
      if (mounted.current && foreground.current && generation.current === token) {
        setNativeStatus(null);
        setNativeDetail('E_CAM02_STATUS — metadata native tidak tersedia.');
      }
    }
  }, []);

  const cancelNative = useCallback(async () => {
    if (!captureAdapter) return;
    const token = generation.current;
    try {
      await captureAdapter.cancel();
    } catch (error: unknown) {
      // Camera is unmounted and the JS epoch is already invalidated.
      if (mounted.current && foreground.current && generation.current === token) {
        setNativeDetail(`${safeCaptureCode(error)} — pembatalan native gagal; kamera tetap mati.`);
      }
      throw error;
    }
  }, [captureAdapter]);

  const cancel = useCallback((reason: Reason) => {
    activeGeneration.current = null;
    pendingStart.current = false;
    generation.current += 1;
    captureController.invalidate();
    overlayRef.current = { boxes: [], candidate: null };
    if (mounted.current) {
      setSession(null);
      setStarting(false);
      setCapture(captureController.state);
      setNativeStatus(null);
      setNativeDetail('CANCELLED — hasil / epoch sebelumnya tidak berlaku.');
    }
    apply({ type: 'CANCEL', generation: generation.current, at: now(), reason }, true);
    const token = generation.current;
    void cancelNative().then(() => refreshNativeStatus(token), () => {});
  }, [apply, cancelNative, captureController, refreshNativeStatus]);

  const fail = useCallback((token: number, reason: Reason, code?: string) => {
    if (token !== generation.current || !mounted.current || !foreground.current) return;
    if (activeGeneration.current !== token && !pendingStart.current) return;
    activeGeneration.current = null;
    pendingStart.current = false;
    captureController.fail(token);
    overlayRef.current = { boxes: [], candidate: null };
    setSession(null);
    setStarting(false);
    setCapture(captureController.state);
    setNativeStatus(null);
    setNativeDetail(code ?? 'ERROR — tidak ada JPEG sukses yang diasumsikan.');
    apply({ type: 'ERROR', generation: token, at: now(), reason }, true, undefined,
      code?.match(/^E_(?:CAPTURE|ENR01A|CAM02)_[A-Z0-9_]{1,60}(?=$| )/)?.[0]);
    void cancelNative().then(() => refreshNativeStatus(token), () => {});
  }, [apply, cancelNative, captureController, refreshNativeStatus]);

  useEffect(() => {
    mounted.current = true;
    const listener = AppState.addEventListener('change', (state) => {
      foreground.current = state === 'active';
      if (state !== 'active') cancel('BACKGROUND');
    });
    return () => {
      mounted.current = false;
      generation.current += 1;
      activeGeneration.current = null;
      pendingStart.current = false;
      captureController.invalidate();
      void cancelNative().catch(() => {});
      listener.remove();
    };
  }, [cancel, cancelNative, captureController]);

  useEffect(() => {
    if (!portrait && (activeGeneration.current !== null || pendingStart.current)) cancel('PORTRAIT_REQUIRED');
  }, [portrait, cancel]);

  useEffect(() => {
    if (session !== null && !hasPermission) fail(session, 'PERMISSION_DENIED');
  }, [session, hasPermission, fail]);

  useEffect(() => {
    if (session === null) return;
    const timer = setInterval(() => {
      if (activeGeneration.current !== session || !foreground.current) return;
      apply({ type: 'TICK', generation: session, at: now() });
    }, UI_INTERVAL_MS);
    return () => clearInterval(timer);
  }, [session, apply]);

  const onLayout = (layout: LayoutChangeEvent) => {
    const { width, height } = layout.nativeEvent.layout;
    if (width === sizeRef.current.width && height === sizeRef.current.height) return;
    const next = { width, height };
    sizeRef.current = next;
    setViewSize(next);
    overlayRef.current = { boxes: [], candidate: null };
    if (activeGeneration.current !== null) {
      apply({ type: 'RESET', generation: activeGeneration.current, at: now(), reason: 'LAYOUT_CHANGED' }, true);
    } else {
      publish(true);
    }
  };

  const decideFrame = useCallback(async (frame: FrameMetadata, approved: boolean, corners: number[] | null) => {
    if (!captureAdapter) return; // Native candidates also have an independent deadline.
    const token = frame.generation;
    const current = () => mounted.current && foreground.current
      && generation.current === token && activeGeneration.current === token;
    const captureFrame = approved && current();
    try {
      const lease = await captureAdapter.decide(token, frame.frameId, captureFrame, captureFrame ? corners : null);
      if (!captureFrame || !current()) return;
      if (!lease || !captureController.ready(lease.metadata, foreground.current)) {
        fail(token, 'CAPTURE_ERROR', 'E_CAPTURE_RESULT — hasil tidak cocok dengan frame / epoch yang dipilih.');
        return;
      }
      const result = lease.metadata; // Explicit whitelist; never contains the opaque handle.
      console.log(`ENR01A_EVENT ${JSON.stringify({
        event: 'NATIVE_CAPTURE_READY', generation: result.generation, frameId: result.frameId,
        nativeStatus: 'READY', source: machineRef.current.source,
      })}`);
      setCapture(captureController.state);
      if (!captureController.consuming(token, frame.frameId)) {
        fail(token, 'CAPTURE_ERROR', 'E_CAPTURE_STATE');
        return;
      }
      setCapture(captureController.state);
      setNativeDetail('CONSUMING — native consumer membaca JPEG tepat sekali; kamera belum ditutup.');
      const receipt = await lease.consumeOnce();
      if (!current()) return;
      if (!captureController.complete(receipt, foreground.current)) {
        fail(token, 'CAPTURE_ERROR', 'E_CAPTURE_RECEIPT');
        return;
      }
      const logical = machineRef.current;
      trace.current.seq += 1;
      console.log(`ENR01A_EVENT ${JSON.stringify({
        cycle: trace.current.cycle, generation: receipt.generation, seq: trace.current.seq,
        event: 'NATIVE_CONSUMED', frameId: receipt.frameId,
        state: logical.phase, reason: logical.reason, source: logical.source,
        clock: 'monotonic_js', nativeStatus: 'CONSUMED',
        count: logical.faceCount, stableMs: logical.stableMs, samples: logical.samples,
        elapsedMs: Math.max(0, now() - trace.current.startedAt),
        captureTriggerCount: trace.current.captureTriggerCount,
        nativeTriggerCount: captureController.state.triggerCount,
        jpegBytes: receipt.jpegBytes, width: receipt.width, height: receipt.height,
        encodeMs: result.encodeMs, selectedAgeMs: result.selectedAgeMs,
        peakOwnedBytes: result.peakOwnedBytes, exif: receipt.exif,
      })}`);
      // Closing the source revokes outstanding handles: consume must finish first.
      activeGeneration.current = null;
      pendingStart.current = false;
      setCapture(captureController.state);
      setNativeDetail('CONSUMED — native consumer selesai; buffer adapter dilepas sebelum kamera ditutup. Bukan enrollment.');
      setSession(null);
      setStarting(false);
      void refreshNativeStatus(token);
    } catch (error: unknown) {
      // A late rejection of an older nonterminal candidate cannot cancel the selected frame.
      if (current() && (captureFrame || captureController.state.status === 'ARMED')) {
        fail(token, 'CAPTURE_ERROR', safeCaptureCode(error));
      }
    }
  }, [captureAdapter, captureController, fail, refreshNativeStatus]);

  const onFrame = useCallback((input: FrameMetadata | undefined) => {
    // Missing identity cannot be safely acknowledged; the native 400 ms
    // ownership timeout releases it. Duplicate/old IDs were already decided.
    if (!input || !Number.isInteger(input.generation) || typeof input.frameId !== 'string'
      || !captureController.claimFrame(input.frameId)) return;
    const token = input.generation;
    let approved = false;
    let corners: number[] | null = null;
    let failed: 'DETECTOR_ERROR' | 'CAPTURE_ERROR' | null = null;
    try {
      const preview = camera.current;
      if (token !== activeGeneration.current || token !== generation.current || !mounted.current
        || !foreground.current || !machineRef.current.ready || !preview) return;
      if (!Array.isArray(input.faces) || !Number.isFinite(input.acquiredAtMs)
        || !Number.isFinite(input.emittedAtMs) || input.emittedAtMs < input.acquiredAtMs
        || input.emittedAtMs - input.acquiredAtMs > MAX_GAP_MS) {
        failed = 'DETECTOR_ERROR';
        return;
      }
      const faces = input.faces;
      callbackCounter.current += 1;
      const at = now(); // Same monotonic JS callback clock / dwell thresholds as CAM01.
      const view = sizeRef.current;
      let candidate: Candidate | null = null;
      let frame = { width: 0, height: 0 };
      const nextBoxes: Rect[] = [];
      // Every observation affects guidance. Only multi-face overlay work is throttled.
      const limit = faces.length === 1 ? 1 : at - lastUi.current >= UI_INTERVAL_MS ? Math.min(faces.length, 3) : 0;
      for (let index = 0; index < limit; index += 1) {
        const face = faces[index]!;
        const dimensions = { width: face.frameWidth, height: face.frameHeight };
        if (!validSize(dimensions) || !validSize(view)) continue;
        // Native autoMode metadata and the CameraRef converter share opaque
        // camera/sensor coordinates; do not substitute frame-size ratios.
        const box = mapBoundsToView(face.bounds, (point) => preview.convertCameraPointToViewPoint(point));
        if (!box) continue;
        nextBoxes.push(box);
        if (faces.length === 1) {
          frame = dimensions;
          const pose = { yaw: face.yawAngle, pitch: face.pitchAngle, roll: face.rollAngle };
          if (Object.values(pose).every(Number.isFinite)) candidate = { box, view, ...pose };
        }
      }
      overlayRef.current = {
        boxes: faces.length > 1 && nextBoxes.length === 0 ? overlayRef.current.boxes : nextBoxes,
        candidate,
      };
      const previous = machineRef.current;
      const next = apply({ type: 'FRAME', generation: token, at, count: faces.length,
        guidance: assessObservation(faces.length, candidate, frame) }, false, input);
      if (next.phase !== 'SIMULATED_CAPTURE' || previous.phase === 'SIMULATED_CAPTURE'
        || captureController.state.status !== 'ARMED') return;
      failed = 'CAPTURE_ERROR';
      corners = [
        { x: 0, y: 0 }, { x: view.width, y: 0 },
        { x: view.width, y: view.height }, { x: 0, y: view.height },
      ].flatMap((point) => {
        const sensor = preview.convertViewPointToCameraPoint(point);
        return [sensor.x, sensor.y];
      });
      if (!corners.every(Number.isFinite)) return;
      failed = null;
      if (!captureController.approve(next, input.frameId)) return;
      approved = true;
      setCapture(captureController.state);
      setNativeDetail('ENCODING — persetujuan logis live; menunggu JPEG native frame yang sama.');
    } catch {
      failed = failed ?? 'DETECTOR_ERROR';
    } finally {
      // Even an early return / invalid geometry makes one explicit rejection.
      void decideFrame(input, approved, corners);
      if (failed) fail(token, failed);
    }
  }, [apply, captureController, decideFrame, fail]);

  useEffect(() => {
    if (!captureBridge) return;
    try {
      const emitter = new NativeEventEmitter(captureBridge);
      const listener = emitter.addListener('CaptureFrame', (payload) => onFrame(payload as FrameMetadata));
      nativeListening.current = true;
      return () => {
        nativeListening.current = false;
        listener.remove();
      };
    } catch {
      nativeListening.current = false;
      setNativeDetail('E_CAM02_LISTENER — aliran metadata native tidak tersedia.');
    }
  }, [onFrame]);

  const onReady = useCallback((token: number) => {
    if (token !== activeGeneration.current || !foreground.current) return;
    apply({ type: 'READY', generation: token, at: now() }, true);
  }, [apply]);
  const onStopped = useCallback((token: number) => {
    if (token === activeGeneration.current) cancel('PREVIEW_STOPPED');
  }, [cancel]);

  const startCamera = async () => {
    if (!mounted.current || !foreground.current || pendingStart.current || verificationBusy.current
      || activeGeneration.current !== null || auditBusy.current || imageBusy.current) return;
    const token = ++generation.current;
    pendingStart.current = true;
    captureController.arm(token);
    setCapture(captureController.state);
    setNativeStatus(null);
    setNativeDetail('Menyiapkan epoch capture native; kamera belum dipasang.');
    setStarting(true);
    overlayRef.current = { boxes: [], candidate: null };
    callbackCounter.current = 0;
    rateWindow.current = { at: now(), count: 0 };
    setCallbackRate(0);
    apply({ type: 'START', generation: token, at: now(), source: 'live' }, true);
    const current = () => mounted.current && generation.current === token && foreground.current && pendingStart.current;
    if (!foreground.current || !portrait || !validSize(sizeRef.current)) {
      fail(token, 'PORTRAIT_REQUIRED');
      return;
    }
    if (!device) { fail(token, 'CAMERA_ERROR'); return; }
    if (!privacyBridge || typeof privacyBridge.setScanningSecure !== 'function') {
      fail(token, 'PRIVACY_UNAVAILABLE');
      return;
    }
    if (!captureAdapter || !captureBridge || !nativeListening.current || typeof captureBridge.start !== 'function'
      || typeof captureBridge.decide !== 'function' || typeof captureBridge.consume !== 'function'
      || typeof captureBridge.cancel !== 'function' || typeof captureBridge.getStatus !== 'function') {
      fail(token, 'CAPTURE_UNAVAILABLE'); return;
    }
    try {
      // Fail closed. Camera is not even mounted until native protection resolves.
      await privacyBridge.setScanningSecure(true);
    } catch {
      if (current()) fail(token, 'PRIVACY_UNAVAILABLE');
      return;
    }
    if (!current()) return;
    let granted = hasPermission;
    try {
      if (!granted && canRequestPermission) granted = await requestPermission();
    } catch {
      if (current()) fail(token, 'PERMISSION_DENIED');
      return;
    }
    if (!current()) return;
    if (!granted) { fail(token, 'PERMISSION_DENIED'); return; }
    try {
      // Explicit restart first drains old native ownership, then arms the new generation.
      // A late start result must never mount a cancelled/background generation.
      if (!current()) return;
      await cancelNative();
      if (!current()) return;
      await captureAdapter.start(token);
    } catch (error: unknown) {
      if (current()) fail(token, 'CAPTURE_UNAVAILABLE', safeCaptureCode(error));
      return;
    }
    if (!current()) return;
    setNativeDetail('ARMED — CaptureFrame live bertag; belum ada persetujuan / JPEG.');
    pendingStart.current = false;
    activeGeneration.current = token;
    setStarting(false);
    setSession(token);
    void refreshNativeStatus(token);
  };

  const runEventFixture = () => {
    if (pendingStart.current || activeGeneration.current !== null || auditBusy.current || imageBusy.current
      || verificationBusy.current || !foreground.current || !portrait || !validSize(sizeRef.current)) return;
    const token = ++generation.current;
    captureController.clear();
    setCapture(captureController.state);
    setNativeStatus(null);
    setNativeDetail('SYNTHETIC_ONLY — event waktu virtual; tidak memanggil start / decide / encode native.');
    const events = stableFixtureEvents(token, now(), sizeRef.current);
    const sample = events.find((input) => input.type === 'FRAME');
    if (sample?.type === 'FRAME') {
      overlayRef.current = { boxes: sample.guidance.candidate ? [sample.guidance.candidate.box] : [], candidate: sample.guidance.candidate };
    }
    // Virtual time remains explicit. All reducer transitions are traced, not
    // just the final coalesced React render. No native capture method is called.
    events.forEach((input, index) => apply(input, false,
      input.type === 'FRAME' ? { frameId: `fixture:${token}:${index}` } : undefined));
    publish(true);
  };

  const auditStorage = async () => {
    if (pendingStart.current || activeGeneration.current !== null || auditBusy.current || imageBusy.current || verificationBusy.current) return;
    if (!privacyBridge || typeof privacyBridge.getStorageInventory !== 'function') {
      setInventory(null);
      setInventoryStatus('AUDIT UNAVAILABLE — bridge native tidak tersedia. Tidak ada hasil aman yang diasumsikan.');
      return;
    }
    auditBusy.current = true;
    setAuditing(true);
    setInventory(null);
    setInventoryStatus('Membaca metadata sandbox dengan kamera mati…');
    const token = generation.current;
    try {
      const result = await privacyBridge.getStorageInventory();
      await refreshNativeStatus(token);
      if (!mounted.current || !foreground.current || generation.current !== token) return;
      setInventory(result);
      setInventoryPage(0);
      setInventoryStatus('Inventaris metadata: files/cache/codeCache/noBackup/databases/sharedPrefs. Bukan forensic proof / jaminan nol write SDK.');
    } catch {
      if (mounted.current) setInventoryStatus('AUDIT UNAVAILABLE — pembacaan metadata native gagal.');
    } finally {
      auditBusy.current = false;
      if (mounted.current) setAuditing(false);
    }
  };

  const runNativeVerification = async () => {
    if (!mounted.current || !foreground.current || pendingStart.current || activeGeneration.current !== null
      || auditBusy.current || imageBusy.current || verificationBusy.current) return;
    if (!captureBridge || !privacyBridge || typeof captureBridge.runSyntheticVerification !== 'function') {
      setVerificationDetail('BLOCKED — native verification / secure-screen bridge tidak tersedia.');
      return;
    }
    const token = ++generation.current;
    verificationBusy.current = true;
    setVerifying(true);
    setVerification(null);
    setVerificationDetail('RUNNING — fixture YUV sintetis, codec Android nyata; tidak membuka kamera.');
    const current = () => mounted.current && foreground.current && generation.current === token;
    try {
      await cancelNative();
      if (!current()) return;
      await privacyBridge.setScanningSecure(true);
      if (!current()) return;
      const report = await captureBridge.runSyntheticVerification();
      if (!current()) return;
      setVerification(report);
      setVerificationDetail(`${report.passed ? 'PASSED' : 'FAILED'} — ${report.source}; bukan deteksi live atau capture manusia.`);
      console.log(`ENR01A_SUITE ${JSON.stringify({
        suite: report.suite, source: report.source, passed: report.passed, caseCount: report.cases.length,
      })}`);
      for (const entry of report.cases) console.log(`ENR01A_CASE ${JSON.stringify({
        name: entry.name, passed: entry.passed, observations: entry.observations, assertions: entry.assertions,
      })}`);
      await refreshNativeStatus(token);
    } catch (error) {
      if (current()) setVerificationDetail(`FAILED — ${safeCaptureCode(error)}; tidak ada PASS yang diasumsikan.`);
    } finally {
      verificationBusy.current = false;
      if (mounted.current) setVerifying(false);
    }
  };

  const testStaticFile = async () => {
    if (pendingStart.current || activeGeneration.current !== null || imageBusy.current || auditBusy.current || verificationBusy.current) return;
    imageBusy.current = true;
    setImageTesting(true);
    setImageTest('STATIC FILE ONLY — mendeteksi mannequin lokal…');
    const at = now();
    try {
      const faces = await imageDetector.detectFaces(DUMMY_FILE_URI);
      if (mounted.current) setImageTest(`STATIC FILE ONLY: ${faces.length} wajah · ${Math.round(now() - at)} ms. Tidak masuk state machine live.`);
    } catch {
      if (mounted.current) setImageTest('STATIC FILE ONLY: file dummy tidak tersedia / deteksi gagal. Bukan hasil live.');
    } finally {
      imageBusy.current = false;
      if (mounted.current) setImageTesting(false);
    }
  };

  const oval = previewOval(viewSize);
  const stopped = session === null && !starting;
  const busy = starting || auditing || imageTesting || verifying;
  const progress = Math.min(1, machine.stableMs / DWELL_MS);
  const displayBox = boxes[0];
  const previewHeight = Math.min(Math.max(dimensions.width * 1.20, 260), dimensions.height * 0.52);
  const fixture = machine.source === 'synthetic_fixture';
  const pageFiles = inventory?.files.slice(inventoryPage * INVENTORY_PAGE_SIZE, (inventoryPage + 1) * INVENTORY_PAGE_SIZE) ?? [];
  const sourceLabel = fixture ? 'SYNTHETIC EVENT FIXTURE · waktu virtual · BUKAN LIVE'
    : session !== null ? 'LIVE CAMERA · mannequin / objek nonpersonal saja'
      : machine.source === 'live' ? `LIVE STOPPED · ${capture.status === 'CONSUMED' ? 'DUMMY JPEG · CONSUMED ONCE' : 'TIDAK ADA CONSUMED'}` : 'KAMERA OFF · belum ada observasi live';
  const diagnostic = [
    `source=${fixture ? 'SYNTHETIC_FIXTURE_VIRTUAL_TIME' : machine.source === 'live' ? 'LIVE' : 'OFF'}`,
    `state=${machine.phase}`, `reason=${machine.reason}`, `faces=${machine.faceCount}`,
    `stableMs=${Math.round(machine.stableMs)}`, `samples=${machine.samples}`,
    `cycle=${trace.current.cycle}`, `generation=${machine.generation}`,
    `frameId=${lastFrameId.current ?? 'none'}`, `logicalTriggerCount=${trace.current.captureTriggerCount}`,
    `nativeStatus=${capture.status}`, `nativeTriggerCount=${capture.triggerCount}`,
    `viewDp=${viewSize.width.toFixed(1)},${viewSize.height.toFixed(1)}`,
    oval ? `ovalDp=${(oval.cx - oval.rx).toFixed(1)},${(oval.cy - oval.ry).toFixed(1)},${(oval.rx * 2).toFixed(1)},${(oval.ry * 2).toFixed(1)}` : 'ovalDp=unmeasured',
    displayBox ? `boxDp=${displayBox.x.toFixed(1)},${displayBox.y.toFixed(1)},${displayBox.width.toFixed(1)},${displayBox.height.toFixed(1)}` : 'boxDp=none',
  ].join(' · ');

  return (
    <SafeAreaView style={styles.screen}>
      <StatusBar style="light" />
      <ScrollView contentContainerStyle={styles.content}>
        <Text style={styles.kicker}>FL-ENR01A · ISOLATED PROTOTYPE · PORTRAIT</Text>
        <Text style={styles.title}>Guided dummy → native consume once</Text>
        <Text style={styles.small}>Mannequin / objek nonpersonal saja. Persetujuan logis, READY, dan CONSUMED dibedakan. Tidak ada backend, enrollment, atau klaim recognition / liveness / PAD. Jangan arahkan ke manusia. Cache SDK diaudit terpisah.</Text>
        {!portrait && <Text style={styles.error}>PORTRAIT_REQUIRED — aktivitas kamera hanya portrait.</Text>}
        <View testID="cam02-preview" style={[styles.preview, { height: previewHeight }]} onLayout={onLayout}>
          {session !== null && device && portrait ? (
            <LiveCamera key={session} generation={session} device={device}
              cameraRef={camera} onReady={onReady} onStopped={onStopped} onFailure={fail} />
          ) : (
            <View style={styles.placeholder}>
              <Text style={styles.placeholderTitle}>{fixture ? 'Fixture koordinat sintetis' : 'Kamera mati'}</Text>
              <Text style={styles.placeholderBody}>{fixture ? 'Bukan hasil detektor / preview kamera.' : 'Mulai kamera secara eksplisit. Resume tidak mengaktifkan kamera.'}</Text>
            </View>
          )}
          <View pointerEvents="none" style={StyleSheet.absoluteFill}>
            {oval && <View style={[styles.oval, {
              left: oval.cx - oval.rx, top: oval.cy - oval.rx,
              width: oval.rx * 2, height: oval.rx * 2,
              borderRadius: oval.rx, transform: [{ scaleY: oval.ry / oval.rx }],
              borderColor: machine.phase === 'STABLE' || machine.phase === 'CAPTURE_READY' ? '#53e49d' : '#f5f9ff',
            }]} />}
            {boxes.map((box, index) => <View key={index} style={[styles.faceBox, {
              left: box.x, top: box.y, width: box.width, height: box.height,
              borderColor: fixture ? '#e9b65d' : '#52d9ff',
            }]} />)}
          </View>
          <View pointerEvents="none" style={styles.sourceBadge}><Text testID="cam02-source" style={styles.sourceText}>{sourceLabel}</Text></View>
        </View>
        <View style={styles.card}>
          <Text testID="cam02-state" style={styles.state}>{machine.phase} · {machine.reason}</Text>
          <Text style={styles.guidance}>{messages[machine.reason]}</Text>
          <Text testID="cam02-face-count" style={styles.small}>{fixture ? 'SUMBER SINTETIS' : 'SUMBER LIVE / OFF'} · face {machine.faceCount} · observasi dwell {machine.samples}</Text>
          <View style={styles.progressTrack}><View style={[styles.progressFill, { width: `${progress * 100}%` }]} /></View>
          <Text testID="cam02-stability" style={styles.small}>{machine.stableMs.toFixed(0)} / {DWELL_MS} ms kontinu · gap maks {MAX_GAP_MS} ms · satu frame live terpilih hanya setelah persetujuan terminal.</Text>
          <Text testID="cam02-diagnostics" accessible accessibilityLabel={diagnostic} style={styles.mono}>{diagnostic}</Text>
        </View>
        <View style={styles.controls}>
          {!stopped ? <Pressable testID="cam02-stop" accessibilityRole="button" style={styles.button} onPress={() => cancel('OPERATOR_STOP')}><Text style={styles.buttonText}>Hentikan / batalkan</Text></Pressable>
            : <Pressable testID="cam02-start" accessibilityRole="button" style={[styles.button, styles.primary, (busy || !portrait) && styles.disabled]} disabled={busy || !portrait} onPress={startCamera}><Text style={styles.buttonText}>{hasPermission ? 'Mulai kamera' : canRequestPermission ? 'Mulai + minta izin' : 'Coba lagi (izin di Settings)'}</Text></Pressable>}
          <Pressable testID="cam02-event-fixture" accessibilityRole="button" style={[styles.button, (!stopped || busy || !portrait) && styles.disabled]} disabled={!stopped || busy || !portrait} onPress={runEventFixture}><Text style={styles.buttonText}>Uji event sintetis (bukan live)</Text></Pressable>
          <Pressable testID="cam02-audit" accessibilityRole="button" style={[styles.button, (!stopped || busy) && styles.disabled]} disabled={!stopped || busy} onPress={auditStorage}><Text style={styles.buttonText}>{auditing ? 'Audit…' : 'Audit metadata (kamera mati)'}</Text></Pressable>
          <Pressable testID="cam02-static-file" accessibilityRole="button" style={[styles.button, (!stopped || busy) && styles.disabled]} disabled={!stopped || busy} onPress={testStaticFile}><Text style={styles.buttonText}>{imageTesting ? 'Menguji file…' : 'Uji file dummy (terpisah)'}</Text></Pressable>
          <Pressable testID="enr01a-native-suite" accessibilityRole="button" style={[styles.button, (!stopped || busy) && styles.disabled]} disabled={!stopped || busy} onPress={runNativeVerification}><Text style={styles.buttonText}>{verifying ? 'Codec suite berjalan…' : 'Uji adapter native (sintetis)'}</Text></Pressable>
        </View>
        <View style={styles.card}>
          <Text style={styles.label}>Persetujuan logis ≠ READY ≠ CONSUMED</Text>
          <Text testID="cam02-native-state" accessible style={styles.state}>{fixture ? 'SYNTHETIC_ONLY · native capture tidak dipanggil' : capture.status}</Text>
          <Text testID="cam02-native-detail" accessible style={styles.small}>{nativeDetail}</Text>
          <Text style={styles.mono}>logical trigger {trace.current.captureTriggerCount} · native trigger {capture.triggerCount} · generation {capture.generation ?? 'none'} · selected frame {capture.frameId ?? 'none'}</Text>
          {capture.result && <Text testID="cam02-jpeg-metadata" accessible style={styles.mono}>
            {`${capture.status} · ${capture.result.width} × ${capture.result.height} · ${capture.result.jpegBytes} B · encode ${capture.result.encodeMs.toFixed(1)} ms · selectedAge ${capture.result.selectedAgeMs.toFixed(1)} ms · peakOwned ${capture.result.peakOwnedBytes} B · EXIF ${String(capture.result.exif)} · frame ${capture.result.frameId}`}
          </Text>}
          <Text style={styles.small}>Ke JS hanya metadata dan handle opaque privat facade; tidak ada JPEG / base64 / path. Native consumer membaca sekali sebelum kamera ditutup. Cancel / background mencabut handle; pelepasan codec aktif bersifat cooperative.</Text>
        </View>
        <View style={styles.card}>
          <Text style={styles.label}>Adapter verification — sintetis, bukan kamera live</Text>
          <Text testID="enr01a-verification-status" accessible style={styles.small}>{verificationDetail}</Text>
          {verification && <Text testID="enr01a-verification-report" accessible style={styles.mono}>{JSON.stringify(verification)}</Text>}
        </View>
        <View style={styles.card}>
          <Text style={styles.label}>Diagnostik lokal — tidak dilog sebagai biometrik</Text>
          <Text style={styles.mono}>preview {session !== null ? machine.ready ? 'READY' : 'WAIT / ENCODING' : 'STOPPED'} · {callbackRate} event/s · {callbackCount} CaptureFrame diterima</Text>
          <Text style={styles.mono}>view dp {viewSize.width.toFixed(1)} × {viewSize.height.toFixed(1)} · density {PixelRatio.get().toFixed(2)} px/dp</Text>
          <Text style={styles.mono}>{oval ? `oval dp x ${(oval.cx - oval.rx).toFixed(1)} y ${(oval.cy - oval.ry).toFixed(1)} w ${(oval.rx * 2).toFixed(1)} h ${(oval.ry * 2).toFixed(1)}` : 'oval belum terukur'}</Text>
          <Text style={styles.mono}>{displayBox ? `box dp x ${displayBox.x.toFixed(1)} y ${displayBox.y.toFixed(1)} w ${displayBox.width.toFixed(1)} h ${displayBox.height.toFixed(1)}` : 'box —'}</Text>
          <Text style={styles.mono}>{shownPose ? `pose yaw ${shownPose.yaw.toFixed(1)} pitch ${shownPose.pitch.toFixed(1)} roll ${shownPose.roll.toFixed(1)} deg` : 'pose —'}</Text>
          <Text style={styles.small}>CaptureFrame native → CameraRef sensor / preview dp, mirror ON, cover. Empat sudut preview dikonversi kembali ke sensor saat approval; native menolak crop yang tidak didukung.</Text>
          <Text style={styles.small}>Proteksi native: {privacyBridge?.setScanningSecure ? 'FLAG_SECURE semua layar (termasuk setelah stop); recent-app snapshot diblok native.' : 'UNAVAILABLE — kamera tidak akan diaktifkan.'}</Text>
          <Text style={styles.small}>{imageTest}</Text>
        </View>
        <View style={styles.card}>
          <Text style={styles.label}>Sandbox metadata — kamera harus berhenti</Text>
          <Text testID="cam02-audit-status" accessible style={styles.small}>{inventoryStatus}</Text>
          <Text testID="cam02-native-audit" accessible style={styles.mono}>{nativeStatus ? `Snapshot native metadata: ${JSON.stringify(nativeStatus)}` : 'Snapshot native belum tersedia. Audit membaca getStatus + inventaris sandbox; bukan forensic proof.'}</Text>
          {inventory && <>
            <Text testID="cam02-audit-summary" accessible style={styles.mono}>{inventory.files.length} file · {inventory.totalBytes} byte · halaman {inventoryPage + 1}/{Math.max(1, Math.ceil(inventory.files.length / INVENTORY_PAGE_SIZE))}</Text>
            {pageFiles.map((file, index) => <Text key={file.path} testID={`cam02-audit-file-${index}`} accessible style={styles.mono}>{file.path} · {file.bytes} B · modifiedAt {file.modifiedAt ?? 'unknown'} · signature {file.imageType ?? 'not-image / unknown'}</Text>)}
            <View style={styles.controls}>
              <Pressable style={[styles.button, inventoryPage === 0 && styles.disabled]} disabled={inventoryPage === 0} onPress={() => setInventoryPage((page) => page - 1)}><Text style={styles.buttonText}>Sebelumnya</Text></Pressable>
              <Pressable style={[styles.button, (inventoryPage + 1) * INVENTORY_PAGE_SIZE >= inventory.files.length && styles.disabled]} disabled={(inventoryPage + 1) * INVENTORY_PAGE_SIZE >= inventory.files.length} onPress={() => setInventoryPage((page) => page + 1)}><Text style={styles.buttonText}>Berikutnya</Text></Pressable>
            </View>
          </>}
          <Text style={styles.small}>Hanya metadata sandbox termasuk databases/sharedPrefs; tidak mengembalikan isi file, piksel, nilai preference / record database, atau storage eksternal. Tidak menghapus file. Bukan packet capture atau bukti forensik.</Text>
        </View>
        <Text style={styles.attribution}>Fixture file opsional: “CosSchlHead2on112010”, CAMiller62, CC BY-SA 3.0, Wikimedia Commons; mannequin bukan manusia. Event sintetis tidak memakai foto / encode native. Kecocokan JPEG dengan preview (mirror / crop), privacy, dan pelepasan resource tetap memerlukan bukti perangkat.</Text>
      </ScrollView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  screen: { flex: 1, backgroundColor: '#080f16' },
  content: { padding: 14, gap: 12, paddingBottom: 30 },
  kicker: { color: '#8ddfc3', fontSize: 11, fontWeight: '700' },
  title: { color: '#fff', fontSize: 21, fontWeight: '700' },
  small: { color: '#bbc8d6', fontSize: 12, lineHeight: 17 },
  preview: { width: '100%', position: 'relative', overflow: 'hidden', borderRadius: 16, backgroundColor: '#12202b' },
  placeholder: { flex: 1, alignItems: 'center', justifyContent: 'center', padding: 32 },
  placeholderTitle: { color: '#fff', fontSize: 20, textAlign: 'center', fontWeight: '600' },
  placeholderBody: { color: '#b5c4d5', fontSize: 13, textAlign: 'center', marginTop: 8 },
  oval: { position: 'absolute', borderWidth: 2, borderStyle: 'dashed' },
  faceBox: { position: 'absolute', borderWidth: 2 },
  sourceBadge: { position: 'absolute', top: 8, left: 8, right: 8, backgroundColor: '#07101ddd', padding: 6, borderRadius: 5 },
  sourceText: { color: '#fff', fontSize: 10, fontWeight: '700', textAlign: 'center' },
  card: { padding: 12, borderRadius: 12, backgroundColor: '#14212d', gap: 7 },
  state: { color: '#89e8c4', fontSize: 12, fontWeight: '700' },
  guidance: { color: '#fff', fontSize: 16, fontWeight: '600', lineHeight: 22 },
  progressTrack: { height: 6, backgroundColor: '#314350', overflow: 'hidden', borderRadius: 3 },
  progressFill: { height: 6, backgroundColor: '#53e49d' },
  controls: { flexDirection: 'row', flexWrap: 'wrap', gap: 8 },
  button: { backgroundColor: '#2c4052', paddingVertical: 12, paddingHorizontal: 13, borderRadius: 9, minHeight: 44, justifyContent: 'center' },
  primary: { backgroundColor: '#23664d' },
  buttonText: { color: '#fff', fontSize: 12, fontWeight: '600' },
  disabled: { opacity: 0.40 },
  label: { color: '#e2edf8', fontSize: 13, fontWeight: '700' },
  mono: { color: '#c8d8e8', fontSize: 11, fontFamily: 'monospace', lineHeight: 17 },
  error: { color: '#ffaaaa', fontSize: 13 },
  attribution: { color: '#8395a7', fontSize: 10, lineHeight: 15 },
});

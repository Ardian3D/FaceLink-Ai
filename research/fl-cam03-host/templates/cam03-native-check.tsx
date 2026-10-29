import React, { useCallback, useRef, useState } from 'react';
import { AppState, Button, ScrollView, StyleSheet, Text, View } from 'react-native';
import { useFocusEffect } from 'expo-router';
import {
  cancel, getStatus, getStorageInventory, runSyntheticVerification,
  type HostReport, type HostStatus, type Inventory,
} from 'facelink-native-capture-host';
import { getPlaybackMetadata, type PlaybackMetadata } from '@/cam03/playback-metadata';
import { isAvailableAsync } from 'expo-secure-store';
import { useSession } from '@/features/auth/SessionProvider';
import { konfigurasiApi } from '@/features/api/config';

/** Staging deep link only: facelinkcam03:///cam03-native-check. Never starts a camera. */
export default function Cam03NativeCheck() {
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('Idle — explicit Run required. Synthetic native fixtures only.');
  const [report, setReport] = useState<HostReport | null>(null);
  const [status, setStatus] = useState<HostStatus | null>(null);
  const [inventory, setInventory] = useState<Inventory | null>(null);
  const [playback, setPlayback] = useState<PlaybackMetadata | null>(null);
  const [secureStoreAvailable, setSecureStoreAvailable] = useState<boolean | null>(null);
  const { status: sessionStatus } = useSession();
  const apiProtocol = konfigurasiApi.ok ? new URL(konfigurasiApi.baseUrl).protocol : null;
  const focused = useRef(false);
  const generation = useRef(0);
  const pending = useRef(false);

  const stop = useCallback(async () => {
    const ticket = ++generation.current;
    if (focused.current) { setBusy(true); setReport(null); setMessage('Cancelling; waiting for native work to drain…'); }
    pending.current = true;
    try {
      await cancel();
      const nativeStatus = await getStatus();
      if (focused.current && ticket === generation.current) {
        setStatus(nativeStatus);
        setMessage('Cancelled; native drain acknowledged.');
      }
    } catch {
      if (focused.current && ticket === generation.current) setMessage('Native cancellation unavailable; do not start another run until native status is idle.');
    } finally {
      if (ticket === generation.current) {
        pending.current = false;
        if (focused.current) setBusy(false);
      }
    }
  }, []);

  useFocusEffect(useCallback(() => {
    focused.current = true;
    setBusy(pending.current);
    const subscription = AppState.addEventListener('change', (state) => {
      if (state !== 'active') void stop();
    });
    return () => {
      focused.current = false;
      subscription.remove();
      // Blur AND unmount invalidate late success; native lifecycle separately enforces foreground.
      void stop();
    };
  }, [stop]));

  const run = async () => {
    if (pending.current || !focused.current || AppState.currentState !== 'active') return;
    pending.current = true;
    const ticket = ++generation.current;
    setBusy(true); setReport(null); setInventory(null); setMessage('Running synthetic native verification…');
    try {
      const result = await runSyntheticVerification();
      const nativeStatus = await getStatus();
      if (focused.current && ticket === generation.current && AppState.currentState === 'active') {
        setReport(result); setStatus(nativeStatus); setMessage(`Synthetic verification: ${result.status}`);
      }
    } catch {
      if (focused.current && ticket === generation.current) setMessage('Verification did not complete. Refresh native status before retrying.');
    } finally {
      if (ticket === generation.current) {
        pending.current = false;
        if (focused.current) setBusy(false);
      }
    }
  };

  const metadata = async () => {
    if (pending.current || !focused.current) return;
    pending.current = true;
    const ticket = ++generation.current;
    setBusy(true);
    try {
      const nativeStatus = await getStatus();
      const files = await getStorageInventory();
      if (focused.current && ticket === generation.current) { setStatus(nativeStatus); setInventory(files); }
    } catch {
      if (focused.current && ticket === generation.current) setMessage('Native metadata unavailable in this build.');
    } finally {
      if (ticket === generation.current) { pending.current = false; if (focused.current) setBusy(false); }
    }
  };

  return (
    <ScrollView contentContainerStyle={styles.page}>
      <Text style={styles.title}>FL-CAM03 · actual FaceLink host</Text>
      <Text style={styles.text}>Native capture verification uses synthetic fixtures only. This route exposes metadata, not images, uploads, OTP requests or sessions. Existing login video playback is observed separately through scalar SDK events; the normal host retains real HTTPS/auth dependencies.</Text>
      <Text accessibilityLiveRegion="polite" style={styles.text}>{message}</Text>
      <View style={styles.actions}>
        <Button title="Run synthetic verification" onPress={() => void run()} disabled={busy} />
        <Button title="Cancel and drain" onPress={() => void stop()} />
        <Button title="Refresh status / storage metadata" onPress={() => void metadata()} disabled={busy} />
        <Button title="Read actual host playback metadata" onPress={() => setPlayback(getPlaybackMetadata())} />
        <Button title="Check SecureStore availability only" onPress={() => {
          const ticket = generation.current;
          void isAvailableAsync().then((available) => {
            if (focused.current && ticket === generation.current && AppState.currentState === 'active') setSecureStoreAvailable(available);
          }).catch(() => {
            if (focused.current && ticket === generation.current && AppState.currentState === 'active') {
              setSecureStoreAvailable(null);
              setMessage('SecureStore availability check failed; availability is unknown.');
            }
          });
        }} />
      </View>
      <Text selectable style={styles.metadata}>{JSON.stringify({
        host: { secureStoreAvailable, sessionStatus, apiConfigurationValid: konfigurasiApi.ok, apiProtocol, apiUsesHttps: apiProtocol === 'https:' },
      }, null, 2)}</Text>
      {status && <Text selectable style={styles.metadata}>{JSON.stringify({ status }, null, 2)}</Text>}
      {report && <Text selectable style={styles.metadata}>{JSON.stringify({ report }, null, 2)}</Text>}
      {inventory && <Text selectable style={styles.metadata}>{JSON.stringify({ inventory }, null, 2)}</Text>}
      <Text style={styles.text}>Playback metadata observes the existing host players only. Ready/playing/progress and overlap do not prove rendered appearance. No screenshot or video frames are collected.</Text>
      {playback && <Text selectable style={styles.metadata}>{JSON.stringify({ playback }, null, 2)}</Text>}
    </ScrollView>
  );
}
const styles = StyleSheet.create({
  page: { padding: 24, paddingTop: 48, gap: 20, backgroundColor: '#10151d', flexGrow: 1 },
  title: { color: '#ffffff', fontSize: 21, fontWeight: '700' },
  text: { color: '#e0e6ed', fontSize: 14 },
  actions: { gap: 14 },
  metadata: { color: '#b9d6ef', fontFamily: 'monospace', fontSize: 11 },
});

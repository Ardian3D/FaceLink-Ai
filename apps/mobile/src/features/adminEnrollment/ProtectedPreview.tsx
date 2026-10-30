import React, { useEffect, useState } from "react";
import { ActivityIndicator, Image, Platform, Pressable, StyleSheet, View } from "react-native";
import { AppText } from "@/components/AppText";
import type { EnrollmentError } from "@/features/enrollment/model";
import { spacing, useTheme } from "@/theme";
import { adminApi } from "./api";

/** In-memory renderer only. This is not a claim of OS/decoder cache zeroization. */
export function ProtectedPreview({ submissionId, getToken, onClose, onDenied }: {
  submissionId: string;
  getToken: () => Promise<string | null>;
  onClose: () => void;
  onDenied: (error: EnrollmentError) => void;
}) {
  const { colors } = useTheme();
  const [uri, setUri] = useState<string | null>(null);
  const [error, setError] = useState(false);
  useEffect(() => {
    const abort = new AbortController();
    let objectUrl: string | null = null;
    let live = true;
    const load = async () => {
      const token = await getToken();
      if (!live) return;
      if (!token) { setError(true); onDenied({ kind: "sessionEnded" }); return; }
      const result = await adminApi.photo(token, submissionId, abort.signal);
      if (!live) { if (result.ok) result.data.fill(0); return; }
      if (!result.ok) { setError(true); onDenied(result.error); return; }
      const bytes = result.data;
      try {
        if (Platform.OS === "web") {
          // Copy into a Blob owned by this mount; revoke it on every close.
          objectUrl = URL.createObjectURL(new Blob([bytes.slice().buffer], { type: "image/jpeg" }));
          setUri(objectUrl);
        } else {
          // No filesystem/cache/gallery APIs. Native renderer still owns decoded memory.
          let binary = "";
          for (let i = 0; i < bytes.length; i += 1) binary += String.fromCharCode(bytes[i]!);
          setUri(`data:image/jpeg;base64,${btoa(binary)}`);
        }
      } finally { bytes.fill(0); }
    };
    void load().catch(() => { if (live) { setError(true); onDenied({ kind: "unknown" }); } });
    return () => { live = false; abort.abort(); if (objectUrl) URL.revokeObjectURL(objectUrl); };
  }, [submissionId, getToken, onDenied]);
  return <View style={styles.wrap}>
    {error ? <AppText accessibilityRole="alert">Preview tidak tersedia. Tidak ada fallback URL publik; tutup lalu muat status terbaru.</AppText> : uri ?
      <Image accessibilityLabel="Pola JPEG fixture sintetis, bukan wajah manusia" source={{ uri, cache: "reload" }} resizeMode="contain"
        style={styles.image} onError={() => { setUri(null); setError(true); }} /> : <ActivityIndicator color={colors.textPrimary} accessibilityLabel="Memuat preview terotorisasi" />}
    <Pressable accessibilityRole="button" accessibilityLabel="Tutup preview" onPress={onClose}
      style={[styles.close, { borderColor: colors.border }]}><AppText weight="semibold">Tutup preview</AppText></Pressable>
    <AppText variant="caption">Sintetis saja. Aplikasi tidak menulis file; pelepasan memori/cache renderer belum dibuktikan pada Android.</AppText>
  </View>;
}
const styles = StyleSheet.create({ wrap: { gap: spacing.md }, image: { height: 240, width: "100%" }, close: { minHeight: 48, minWidth: 48, padding: spacing.md, borderWidth: 1, borderRadius: 12, justifyContent: "center", alignItems: "center" } });

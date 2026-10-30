import React from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { radius, spacing, touch, useTheme } from "@/theme";

/**
 * Main-build stub for the staging-only live scanner.
 *
 * No camera module, permission call, vision-camera import, or capture bridge is
 * available here. The staging host replaces this file with the generated
 * template from `research/fl-enr02a-host/templates/nativeScanner.tsx`; keeping
 * the same props lets the enrollment screen compile and refuse to scan instead
 * of pretending a camera exists.
 */
export type NativeScannerProps = {
  onCancel(): void;
  onConsumed(meta: {
    width: number;
    height: number;
    jpegBytes: number;
    frameId: string;
    generation: number;
  }): void;
  onError(code: string): void;
};

export function NativeScanner({ onCancel }: NativeScannerProps) {
  const { colors } = useTheme();
  return (
    <View style={styles.screen}>
      <View
        accessible
        accessibilityRole="alert"
        accessibilityLabel="Pemindai kamera tidak tersedia pada build ini. Tidak ada kamera yang dinyalakan."
        style={[
          styles.card,
          { backgroundColor: colors.surface, borderColor: colors.border },
        ]}
      >
        <Text
          style={[styles.title, { color: colors.textPrimary }]}
          accessibilityRole="header"
        >
          Pemindai kamera tidak tersedia
        </Text>
        <Text style={[styles.body, { color: colors.textMuted }]}>
          Build aplikasi ini tidak menyertakan pemindai kamera native. Tidak ada
          kamera, izin, atau bingkai gambar yang diaktifkan dari layar ini.
        </Text>
        <Text style={[styles.body, { color: colors.textMuted }]}>
          Pemindaian hanya tersedia pada build staging terpisah yang disiapkan
          secara eksplisit untuk pengujian.
        </Text>
      </View>
      <Pressable
        accessibilityRole="button"
        accessibilityLabel="Kembali dari pemindai yang tidak tersedia"
        accessibilityState={{ disabled: false }}
        onPress={onCancel}
        style={[
          styles.button,
          { borderColor: colors.border, backgroundColor: colors.surfaceMuted },
        ]}
      >
        <Text style={[styles.buttonText, { color: colors.textPrimary }]}>Kembali</Text>
      </Pressable>
    </View>
  );
}

export default NativeScanner;

const styles = StyleSheet.create({
  screen: {
    flex: 1,
    padding: spacing.xl,
    gap: spacing.md,
    justifyContent: "center",
  },
  card: {
    padding: spacing.lg,
    borderRadius: radius.lg,
    borderWidth: StyleSheet.hairlineWidth,
    gap: spacing.sm,
  },
  title: { fontSize: 18, fontWeight: "600" },
  body: { fontSize: 13, lineHeight: 19 },
  button: {
    minHeight: touch.minHeight,
    minWidth: touch.minWidth,
    paddingVertical: spacing.sm,
    paddingHorizontal: spacing.lg,
    borderRadius: radius.md,
    alignItems: "center",
    justifyContent: "center",
    borderWidth: StyleSheet.hairlineWidth,
  },
  buttonText: { fontSize: 14, fontWeight: "600" },
});

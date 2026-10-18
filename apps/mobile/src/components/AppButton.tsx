import React from "react";
import { ActivityIndicator, Pressable, StyleSheet, View } from "react-native";

import { AppText } from "./AppText";
import { radius, spacing, touch, typography, useTheme } from "@/theme";

type Variant = "primary" | "secondary";

/**
 * Tombol utama aplikasi.
 *
 * Target sentuh memenuhi `touch.minHeight` (48) dan area sentuh diperluas
 * `hitSlop` supaya nyaman dipakai di HP. Keadaan `loading` menonaktifkan
 * tombol agar satu aksi tidak terkirim dua kali.
 */
export function AppButton({
  label,
  onPress,
  variant = "primary",
  loading = false,
  disabled = false,
  icon,
}: {
  label: string;
  onPress: () => void;
  variant?: Variant;
  loading?: boolean;
  disabled?: boolean;
  icon?: React.ReactNode;
}) {
  const { colors } = useTheme();
  const nonaktif = disabled || loading;
  const utama = variant === "primary";

  return (
    <Pressable
      accessibilityRole="button"
      accessibilityState={{ disabled: nonaktif, busy: loading }}
      accessibilityLabel={label}
      hitSlop={touch.hitSlop}
      disabled={nonaktif}
      onPress={onPress}
      style={({ pressed }) => [
        styles.base,
        utama
          ? { backgroundColor: colors.accent }
          : {
              backgroundColor: colors.surface,
              borderWidth: StyleSheet.hairlineWidth,
              borderColor: colors.border,
            },
        pressed && !nonaktif && styles.pressed,
        nonaktif && styles.disabled,
      ]}
    >
      <View style={styles.isi}>
        {loading ? (
          <ActivityIndicator size="small" color={utama ? colors.onAccent : colors.textPrimary} />
        ) : (
          icon
        )}
        <AppText
          variant="bodyLarge"
          weight="semibold"
          color={utama ? colors.onAccent : colors.textPrimary}
        >
          {label}
        </AppText>
      </View>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  base: {
    minHeight: touch.minHeight,
    minWidth: touch.minWidth,
    borderRadius: radius.lg,
    paddingHorizontal: spacing.xl,
    paddingVertical: spacing.md,
    justifyContent: "center",
    alignItems: "center",
  },
  pressed: {
    opacity: 0.85,
  },
  disabled: {
    opacity: 0.5,
  },
  isi: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.sm,
    minHeight: typography.lineHeight.bodyLarge,
  },
});

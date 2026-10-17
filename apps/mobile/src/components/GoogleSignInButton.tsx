import React from "react";
import { ActivityIndicator, Pressable, StyleSheet, View } from "react-native";

import { AppText } from "./AppText";
import { GoogleGIcon } from "./GoogleGIcon";
import { colors, layout, radius, spacing, touch, typography } from "@/theme";

/**
 * Tombol "Continue with Google".
 *
 * Bentuknya mengikuti mockup pemilik: pil putih penuh lebar, logo Google
 * berwarna di kiri, teks gelap tebal, dan sudut membulat penuh. Tombol putih
 * dipakai supaya kontras terhadap latar video yang gelap, sesuai anjuran merek
 * Google untuk tombol masuk.
 *
 * Target sentuh memenuhi `touch.minHeight`; saat `loading` tombol dinonaktifkan
 * agar satu aksi tidak terkirim dua kali.
 */
export function GoogleSignInButton({
  label = "Continue with Google",
  onPress,
  loading = false,
  disabled = false,
}: {
  label?: string;
  onPress: () => void;
  loading?: boolean;
  disabled?: boolean;
}) {
  const nonaktif = disabled || loading;

  return (
    <Pressable
      accessibilityRole="button"
      accessibilityState={{ disabled: nonaktif, busy: loading }}
      accessibilityLabel={label}
      hitSlop={touch.hitSlop}
      disabled={nonaktif}
      onPress={onPress}
      style={({ pressed }) => [
        styles.tombol,
        pressed && !nonaktif && styles.ditekan,
        nonaktif && styles.nonaktif,
      ]}
    >
      <View style={styles.isi}>
        {loading ? (
          <ActivityIndicator size="small" color={colors.onButtonLight} />
        ) : (
          <GoogleGIcon size={22} />
        )}
        <AppText
          variant="bodyLarge"
          weight="bold"
          color={colors.onButtonLight}
          style={styles.teks}
        >
          {label}
        </AppText>
      </View>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  tombol: {
    minHeight: layout.buttonHeight,
    borderRadius: radius.pill,
    backgroundColor: colors.buttonLight,
    justifyContent: "center",
    paddingHorizontal: spacing.xl,
  },
  ditekan: {
    backgroundColor: colors.buttonLightPressed,
  },
  nonaktif: {
    opacity: 0.6,
  },
  isi: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    gap: spacing.md,
    minHeight: typography.lineHeight.bodyLarge,
  },
  teks: {
    // Tidak memakai letterSpacing agar teks tetap mengikuti font sistem.
    textAlignVertical: "center",
  },
});

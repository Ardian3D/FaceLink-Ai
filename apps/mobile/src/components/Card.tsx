import React from "react";
import { StyleSheet, View, type ViewStyle } from "react-native";

import { colors, elevation, radius, spacing } from "@/theme";

/** Permukaan kartu yang dipakai berulang di seluruh dashboard. */
export function Card({
  children,
  style,
  muted = false,
}: {
  children: React.ReactNode;
  style?: ViewStyle;
  muted?: boolean;
}) {
  return (
    <View style={[styles.card, muted && styles.muted, style]}>
      {children}
    </View>
  );
}

const styles = StyleSheet.create({
  card: {
    backgroundColor: colors.surface,
    borderRadius: radius.lg,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: colors.border,
    padding: spacing.lg,
    gap: spacing.md,
    ...elevation.card,
  },
  muted: {
    backgroundColor: colors.surfaceMuted,
  },
});

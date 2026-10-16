import React from "react";
import { StyleSheet, View } from "react-native";
import { Ionicons } from "@expo/vector-icons";

import { AppText } from "./AppText";
import { colors, radius, spacing, typography } from "@/theme";

export type StatusTone = "success" | "warning" | "error" | "info" | "neutral" | "demo";

/**
 * Lencana status.
 *
 * Pedoman UX §3 melarang menyampaikan status lewat warna saja, sehingga setiap
 * lencana selalu memuat ikon **dan** teks.
 */
export function StatusBadge({
  label,
  tone = "neutral",
  icon,
}: {
  label: string;
  tone?: StatusTone;
  icon?: keyof typeof Ionicons.glyphMap;
}) {
  const warna = tone === "neutral" ? colors.textSecondary : colors[tone];
  const ikon = icon ?? ikonBawaan[tone];

  return (
    <View style={[styles.badge, { borderColor: warna }]}>
      <Ionicons name={ikon} size={typography.size.body} color={warna} />
      <AppText variant="caption" weight="medium" color={warna}>
        {label}
      </AppText>
    </View>
  );
}

const ikonBawaan: Record<StatusTone, keyof typeof Ionicons.glyphMap> = {
  success: "checkmark-circle",
  warning: "alert-circle",
  error: "close-circle",
  info: "information-circle",
  neutral: "ellipse-outline",
  demo: "flask",
};

const styles = StyleSheet.create({
  badge: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.xs,
    paddingHorizontal: spacing.sm,
    paddingVertical: spacing.xs,
    borderRadius: radius.pill,
    borderWidth: StyleSheet.hairlineWidth,
    backgroundColor: colors.surfaceMuted,
    alignSelf: "flex-start",
  },
});

import React from "react";
import { StyleSheet, View } from "react-native";

import { AppText } from "./AppText";
import { colors, radius, spacing } from "@/theme";

/**
 * Tile metrik untuk ringkasan dashboard. Nilai besar dengan label kecil di
 * bawahnya; `hint` dipakai untuk keterangan tambahan yang tidak mendesak.
 */
export function StatTile({
  label,
  value,
  hint,
  icon,
  tone = colors.textPrimary,
}: {
  label: string;
  value: string;
  hint?: string;
  icon?: React.ReactNode;
  tone?: string;
}) {
  return (
    <View style={styles.tile}>
      {icon}
      <AppText variant="heading" weight="bold" color={tone}>
        {value}
      </AppText>
      <AppText variant="caption" color={colors.textSecondary}>
        {label}
      </AppText>
      {hint ? (
        <AppText variant="caption" color={colors.textMuted}>
          {hint}
        </AppText>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  tile: {
    flexGrow: 1,
    flexBasis: 0,
    minWidth: 120,
    backgroundColor: colors.surface,
    borderRadius: radius.md,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: colors.border,
    padding: spacing.lg,
    gap: spacing.xs,
  },
});

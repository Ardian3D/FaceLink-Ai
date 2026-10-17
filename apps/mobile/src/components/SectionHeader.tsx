import React from "react";
import { StyleSheet, View } from "react-native";

import { AppText } from "./AppText";
import { colors, spacing } from "@/theme";

/** Judul bagian dengan aksi opsional di sisi kanan. */
export function SectionHeader({
  title,
  subtitle,
  action,
}: {
  title: string;
  subtitle?: string;
  action?: React.ReactNode;
}) {
  return (
    <View style={styles.wrap}>
      <View style={styles.teks}>
        <AppText variant="title" weight="semibold">
          {title}
        </AppText>
        {subtitle ? (
          <AppText variant="caption" color={colors.textMuted}>
            {subtitle}
          </AppText>
        ) : null}
      </View>
      {action}
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    gap: spacing.md,
  },
  teks: {
    flex: 1,
    gap: 2,
  },
});

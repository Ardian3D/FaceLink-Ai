import React from "react";
import { StyleSheet, View } from "react-native";
import { Ionicons } from "@expo/vector-icons";

import { AppText } from "./AppText";
import { radius, spacing, useTheme } from "@/theme";

/**
 * Penanda mode demo.
 *
 * Wajib tampil pada setiap layar yang menampilkan data contoh, supaya hasil
 * preview tidak pernah tertukar dengan data produksi (FL-UI01 §11).
 */
export function DemoBanner({ keterangan }: { keterangan: string }) {
  const { colors } = useTheme();

  return (
    <View
      style={[
        styles.banner,
        {
          backgroundColor: colors.surfaceMuted,
          borderColor: colors.demo,
        },
      ]}
    >
      <Ionicons name="flask" size={16} color={colors.demo} />
      <AppText variant="caption" weight="medium" color={colors.demo} style={styles.teks}>
        DEMO — {keterangan}
      </AppText>
    </View>
  );
}

const styles = StyleSheet.create({
  banner: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.sm,
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: radius.md,
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.sm,
  },
  teks: {
    flex: 1,
  },
});

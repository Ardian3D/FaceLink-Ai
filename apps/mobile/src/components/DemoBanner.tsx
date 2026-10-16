import React from "react";
import { StyleSheet, View } from "react-native";
import { Ionicons } from "@expo/vector-icons";

import { AppText } from "./AppText";
import { colors, radius, spacing } from "@/theme";

/**
 * Penanda mode demo.
 *
 * Wajib tampil pada setiap layar yang menampilkan data contoh, supaya hasil
 * preview tidak pernah tertukar dengan data produksi (FL-UI01 §11).
 */
export function DemoBanner({ keterangan }: { keterangan: string }) {
  return (
    <View style={styles.banner}>
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
    backgroundColor: colors.surfaceMuted,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: colors.demo,
    borderRadius: radius.md,
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.sm,
  },
  teks: {
    flex: 1,
  },
});

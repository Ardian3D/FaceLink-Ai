import React from "react";
import { Image, StyleSheet, View } from "react-native";

import { AppText } from "./AppText";
import { colors, layout, spacing, typography } from "@/theme";

/**
 * Logo FaceLink AI: mark resmi + wordmark.
 *
 * Mark diambil dari `assets/branding/logo-mark.png`, turunan dari logo resmi
 * pemilik project (`design-inputs/logo/logo-white-noBg.png`) yang latarnya
 * sudah dihapus, sehingga mark menyatu dengan latar gelap aplikasi.
 *
 * Wordmark digambar sebagai teks, bukan gambar, supaya tajam pada semua
 * kerapatan layar: "FaceLink" putih dan "Ai" merah brand — susunan yang sama
 * dengan logo resmi. Font brand belum diidentifikasi pemilik, jadi yang dipakai
 * adalah font sistem dengan bobot dan jarak antarhuruf yang mendekati; ini
 * disebut sebagai keterbatasan, bukan dianggap sama persis.
 */
export function BrandLogo({
  size = layout.logoSize,
  showWordmark = true,
}: {
  size?: number;
  showWordmark?: boolean;
}) {
  return (
    <View style={styles.wrap} accessible accessibilityLabel="FaceLink Ai">
      <Image
        source={require("../../assets/branding/logo-mark.png")}
        style={{ width: size, height: size }}
        resizeMode="contain"
      />
      {showWordmark ? (
        <View style={styles.wordmark}>
          <AppText variant="wordmark" weight="heavy" style={styles.huruf}>
            FaceLink
          </AppText>
          <AppText variant="wordmark" weight="heavy" color={colors.accent} style={styles.huruf}>
            Ai
          </AppText>
        </View>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: {
    alignItems: "center",
    gap: spacing.lg,
  },
  wordmark: {
    flexDirection: "row",
    alignItems: "flex-end",
    gap: spacing.sm,
  },
  huruf: {
    letterSpacing: typography.tracking.wordmark,
  },
});

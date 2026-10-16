import React from "react";
import { Image, StyleSheet, View } from "react-native";

import { AppText } from "./AppText";
import { colors, layout, spacing } from "@/theme";

/**
 * Logo FaceLink AI.
 *
 * PLACEHOLDER: aset logo resmi belum tersedia, sehingga yang ditampilkan adalah
 * gambar sementara dari `assets/branding/`. Berkas itu **bukan** logo brand dan
 * tidak boleh dianggap final. Saat aset resmi diberikan, cukup ganti
 * `assets/branding/logo-mark.png` — proporsi di sini sudah dijaga (rasio 1:1,
 * `resizeMode: contain`) sehingga tidak ada penyesuaian layout yang diperlukan.
 */
export function BrandLogo({
  size = layout.logoSize,
  showWordmark = true,
}: {
  size?: number;
  showWordmark?: boolean;
}) {
  return (
    <View style={styles.wrap}>
      <Image
        source={require("../../assets/branding/logo-mark.png")}
        style={{ width: size, height: size }}
        resizeMode="contain"
        accessibilityLabel="Logo FaceLink AI (placeholder)"
      />
      {showWordmark ? (
        <View style={styles.teks}>
          <AppText variant="heading" weight="bold">
            FaceLink AI
          </AppText>
          <AppText variant="caption" color={colors.textMuted}>
            Absensi wajah dan IoT
          </AppText>
        </View>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: {
    alignItems: "center",
    gap: spacing.md,
  },
  teks: {
    alignItems: "center",
    gap: spacing.xs,
  },
});

import React from "react";
import { Image, StyleSheet, View } from "react-native";

import { AppText } from "./AppText";
import { layout, typography, useTheme } from "@/theme";

/**
 * Logo FaceLink AI: mark resmi + wordmark.
 *
 * Mark diambil dari dua turunan logo resmi pemilik project
 * (`design-inputs/logo/logo-white-noBg.png`), keduanya berlatar transparan:
 *
 * - `logo-mark.png`      — mark putih, untuk latar gelap.
 * - `logo-mark-dark.png` — mark hitam, untuk latar terang.
 *
 * Mark putih tidak terlihat di atas latar putih, jadi pemilihan berkas
 * mengikuti tema aktif — bukan sekadar hiasan, melainkan syarat agar logo tetap
 * terbaca pada kedua tema.
 *
 * Wordmark digambar sebagai teks, bukan gambar, supaya tajam pada semua
 * kerapatan layar: "FaceLink" putih dan "Ai" merah brand — susunan yang sama
 * dengan logo resmi. Jenis hurufnya Poppins ExtraBold, dipilih pemilik project;
 * mark tetap memakai berkas gambar resmi, jadi hanya wordmark yang bergantung
 * pada font. Bila nanti pemilik memberikan wordmark sebagai gambar, ganti blok
 * `wordmark` di bawah dengan `Image` dan hapus ketergantungan font ini.
 */
export function BrandLogo({
  size = layout.logoSize,
  showWordmark = true,
}: {
  size?: number;
  showWordmark?: boolean;
}) {
  const { colors, mode } = useTheme();
  // Wordmark mengikuti ukuran mark, bukan ukuran tetap. Dengan begitu layar
  // yang memakai mark kecil (layar pemeriksaan peran) tidak menampilkan teks
  // raksasa, dan proporsi mark : wordmark tetap sama di semua layar.
  // Rasio 0,393 diambil dari layar masuk: mark 112 px dengan wordmark 44 px.
  const ukuranTeks = Math.round(size * 0.393);
  const spasiHuruf = Math.round((typography.tracking.wordmark * ukuranTeks) / 44);

  return (
    <View
      style={[styles.wrap, { gap: Math.max(8, Math.round(size * 0.14)) }]}
      accessible
      accessibilityLabel="FaceLink Ai"
    >
      <Image
        source={
          mode === "dark"
            ? require("../../assets/branding/logo-mark.png")
            : require("../../assets/branding/logo-mark-dark.png")
        }
        style={{ width: size, height: size }}
        resizeMode="contain"
      />
      {showWordmark ? (
        <View style={[styles.wordmark, { gap: Math.max(4, Math.round(size * 0.07)) }]}>
          <AppText
            weight="heavy"
            style={[styles.huruf, { fontSize: ukuranTeks, lineHeight: Math.round(ukuranTeks * 1.18), letterSpacing: spasiHuruf }]}
          >
            FaceLink
          </AppText>
          <AppText
            weight="heavy"
            color={colors.accent}
            style={[styles.huruf, { fontSize: ukuranTeks, lineHeight: Math.round(ukuranTeks * 1.18), letterSpacing: spasiHuruf }]}
          >
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
  },
  wordmark: {
    flexDirection: "row",
    alignItems: "flex-end",
  },
  huruf: {
    // Ukuran dan jarak antarhuruf dihitung dari `size` di dalam komponen.
  },
});

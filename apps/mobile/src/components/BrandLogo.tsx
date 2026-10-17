import React from "react";
import { Image, StyleSheet, View } from "react-native";

import { AppText } from "./AppText";
import { colors, layout, typography } from "@/theme";

/**
 * Logo FaceLink AI: mark resmi + wordmark.
 *
 * Mark diambil dari `assets/branding/logo-mark.png`, turunan dari logo resmi
 * pemilik project (`design-inputs/logo/logo-white-noBg.png`) yang latarnya
 * sudah dihapus, sehingga mark menyatu dengan latar gelap aplikasi.
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
        source={require("../../assets/branding/logo-mark.png")}
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

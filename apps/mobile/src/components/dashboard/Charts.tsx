import React from "react";
import { StyleSheet, View } from "react-native";

import { AppText } from "@/components/AppText";
import { useTheme , radius, spacing, typography } from "@/theme";

/**
 * Grafik batang kehadiran mingguan.
 *
 * Digambar dengan `View`, bukan library grafik: datanya kecil, bentuknya
 * sederhana, dan satu dependensi lebih sedikit berarti satu risiko versi lebih
 * sedikit. Batang memakai lebar tetap dan tinggi proporsional; hari ini
 * ditandai aksen penuh, hari lain memakai isian redup.
 *
 * Angka tidak ditulis di atas setiap batang — label hari di bawah sudah cukup
 * untuk membaca tren, dan angka pastinya tersedia di ringkasan.
 */
export function BarChart({
  data,
  activeId,
  tinggi = 96,
}: {
  data: { id: string; label: string; value: number }[];
  activeId?: string;
  tinggi?: number;
}) {
  const { colors } = useTheme();
  const maks = Math.max(1, ...data.map((d) => d.value));

  return (
    <View style={styles.wrap} accessibilityRole="image" accessibilityLabel="Grafik kehadiran mingguan">
      <View style={[styles.area, { height: tinggi }]}>
        {data.map((d) => {
          const aktif = d.id === activeId;
          const h = Math.max(6, Math.round((d.value / maks) * tinggi));
          return (
            <View key={d.id} style={styles.kolom}>
              <View
                style={[
                  styles.batang,
                  {
                    height: h,
                    backgroundColor: aktif ? colors.accent : colors.chartTrack,
                  },
                ]}
              />
            </View>
          );
        })}
      </View>
      <View style={styles.label}>
        {data.map((d) => {
          const aktif = d.id === activeId;
          return (
            <View key={d.id} style={styles.kolom}>
              <AppText
                variant="micro"
                weight={aktif ? "bold" : "medium"}
                color={aktif ? colors.accent : colors.textMuted}
              >
                {d.label}
              </AppText>
            </View>
          );
        })}
      </View>
    </View>
  );
}

/** Grafik garis tipis, dipakai untuk tren jumlah kehadiran. */
export function Sparkline({
  data,
  tinggi = 44,
}: {
  data: number[];
  tinggi?: number;
}) {
  const { colors } = useTheme();
  const maks = Math.max(1, ...data);
  // Setiap segmen digambar sebagai batang sangat tipis dengan ujung membulat;
  // pada ukuran ini hasilnya terbaca sebagai garis tren tanpa perlu SVG path.
  return (
    <View style={[styles.spark, { height: tinggi }]} accessibilityElementsHidden>
      {data.map((v, i) => {
        const h = Math.max(4, Math.round((v / maks) * tinggi));
        return (
          <View
            key={i}
            style={[
              styles.sparkSegmen,
              { height: h, backgroundColor: i === data.length - 1 ? colors.accent : colors.chartTrack },
            ]}
          />
        );
      })}
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: {
    gap: spacing.sm,
  },
  area: {
    flexDirection: "row",
    alignItems: "flex-end",
    gap: spacing.sm,
  },
  kolom: {
    flex: 1,
    alignItems: "center",
  },
  batang: {
    width: "100%",
    maxWidth: 34,
    borderRadius: radius.sm,
  },
  label: {
    flexDirection: "row",
    gap: spacing.sm,
  },
  spark: {
    flexDirection: "row",
    alignItems: "flex-end",
    gap: 3,
  },
  sparkSegmen: {
    flex: 1,
    borderRadius: typography.size.micro / 2,
  },
});

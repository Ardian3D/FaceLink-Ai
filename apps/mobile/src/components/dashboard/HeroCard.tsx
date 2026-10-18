import React from "react";
import { Pressable, StyleSheet, View } from "react-native";
import { Ionicons } from "@expo/vector-icons";

import { AppText } from "@/components/AppText";
import { useTheme , layout, radius, spacing, touch, typography } from "@/theme";

/**
 * Kartu sorotan besar di puncak dashboard.
 *
 * Mengikuti referensi: satu kartu besar berwarna aksen dengan sudut sangat
 * membulat, label kecil di atas, angka besar, dan bilah kemajuan di bawah.
 * Warnanya selalu aksen merek (merah) dengan teks putih — bukan warna acak per
 * role — supaya identitas FaceLink tetap terbaca.
 *
 * `onPress` opsional: bila tidak ada aksi nyata di baliknya, kartu tidak dibuat
 * seolah-olah bisa ditekan.
 */
export function HeroCard({
  label,
  value,
  meta,
  progress,
  progressLabel,
  actionLabel,
  onPress,
}: {
  label: string;
  value: string;
  meta?: string;
  /** 0..1. Bila diisi, bilah kemajuan ditampilkan. */
  progress?: number;
  progressLabel?: string;
  actionLabel?: string;
  onPress?: () => void;
}) {
  const { colors } = useTheme();
  const persen = progress === undefined ? undefined : Math.max(0, Math.min(1, progress));

  const isi = (
    <View style={[styles.kartu, { backgroundColor: colors.accent }]}>
      <View style={styles.kepala}>
        <View style={[styles.pil, { backgroundColor: "rgba(255,255,255,0.18)" }]}>
          <AppText variant="micro" weight="semibold" color={colors.textOnAccent} style={styles.pilTeks}>
            {label}
          </AppText>
        </View>
        {actionLabel ? (
          <View style={[styles.aksi, { backgroundColor: "rgba(0,0,0,0.22)" }]}>
            <Ionicons name="arrow-forward" size={16} color={colors.textOnAccent} />
          </View>
        ) : null}
      </View>

      <View style={styles.tengah}>
        <AppText variant="hero" weight="bold" color={colors.textOnAccent}>
          {value}
        </AppText>
        {meta ? (
          <AppText variant="caption" color="rgba(255,255,255,0.85)">
            {meta}
          </AppText>
        ) : null}
      </View>

      {persen !== undefined ? (
        <View style={styles.kemajuan}>
          <View style={[styles.trek, { backgroundColor: "rgba(255,255,255,0.25)" }]}>
            <View style={[styles.isiTrek, { width: `${persen * 100}%` }]} />
          </View>
          {progressLabel ? (
            <View style={styles.kemajuanTeks}>
              <AppText variant="micro" weight="medium" color="rgba(255,255,255,0.9)">
                {progressLabel}
              </AppText>
              <AppText variant="micro" weight="bold" color={colors.textOnAccent}>
                {Math.round(persen * 100)}%
              </AppText>
            </View>
          ) : null}
        </View>
      ) : null}
    </View>
  );

  if (!onPress) return isi;

  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={`${label}: ${value}`}
      onPress={onPress}
      hitSlop={touch.hitSlop}
      style={({ pressed }) => [pressed && styles.ditekan]}
    >
      {isi}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  kartu: {
    minHeight: layout.heroCardHeight,
    borderRadius: radius.xxl,
    padding: spacing.xl,
    gap: spacing.lg,
    justifyContent: "space-between",
  },
  kepala: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
  },
  pil: {
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.xs,
    borderRadius: radius.pill,
  },
  pilTeks: {
    letterSpacing: typography.tracking.caps,
  },
  aksi: {
    width: 32,
    height: 32,
    borderRadius: radius.pill,
    alignItems: "center",
    justifyContent: "center",
  },
  tengah: {
    gap: spacing.xs,
  },
  kemajuan: {
    gap: spacing.sm,
  },
  trek: {
    height: 10,
    borderRadius: radius.pill,
    overflow: "hidden",
  },
  isiTrek: {
    height: "100%",
    borderRadius: radius.pill,
    backgroundColor: "#FFFFFF",
  },
  kemajuanTeks: {
    flexDirection: "row",
    justifyContent: "space-between",
  },
  ditekan: {
    opacity: 0.9,
  },
});

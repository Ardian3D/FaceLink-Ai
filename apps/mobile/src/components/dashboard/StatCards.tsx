import React from "react";
import { Pressable, StyleSheet, View } from "react-native";
import { Ionicons } from "@expo/vector-icons";

import { AppText } from "@/components/AppText";
import { useTheme , radius, spacing, touch } from "@/theme";

/**
 * Tile ringkasan berpasangan (angka besar + label kecil + ikon).
 *
 * Dua tile berdampingan seperti pada referensi: kiri untuk capaian hari ini,
 * kanan untuk hal yang perlu perhatian. Latar tile berbeda tipis dari kartu
 * utama supaya tetap terpisah tanpa garis tebal.
 */
export function MiniStat({
  label,
  value,
  hint,
  icon,
  tone = "neutral",
}: {
  label: string;
  value: string;
  hint?: string;
  icon: keyof typeof Ionicons.glyphMap;
  tone?: "neutral" | "accent" | "warning";
}) {
  const { colors } = useTheme();
  const warna =
    tone === "accent" ? colors.accent : tone === "warning" ? colors.warning : colors.textPrimary;

  return (
    <View style={[styles.tile, { backgroundColor: colors.surface, borderColor: colors.border }]}>
      <View style={styles.kepala}>
        <Ionicons name={icon} size={16} color={tone === "neutral" ? colors.textMuted : warna} />
        <AppText variant="micro" color={colors.textMuted} numberOfLines={1} style={styles.label}>
          {label}
        </AppText>
      </View>
      <AppText variant="heading" weight="bold" color={warna}>
        {value}
      </AppText>
      {hint ? (
        <AppText variant="micro" color={colors.textMuted} numberOfLines={1}>
          {hint}
        </AppText>
      ) : null}
    </View>
  );
}

/**
 * Baris aksi cepat berbentuk pil dengan ikon di dalam lingkaran.
 * Dipakai untuk pintasan yang benar-benar menuju sesuatu.
 */
export function QuickAction({
  icon,
  label,
  onPress,
}: {
  icon: keyof typeof Ionicons.glyphMap;
  label: string;
  onPress?: () => void;
}) {
  const { colors } = useTheme();

  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={label}
      onPress={onPress}
      disabled={!onPress}
      hitSlop={touch.hitSlop}
      style={({ pressed }) => [
        styles.aksi,
        { backgroundColor: colors.surface, borderColor: colors.border },
        pressed && { opacity: 0.8 },
        !onPress && { opacity: 0.55 },
      ]}
    >
      <View style={[styles.lingkaran, { backgroundColor: colors.accentGlow }]}>
        <Ionicons name={icon} size={18} color={colors.accent} />
      </View>
      <AppText variant="caption" weight="medium" numberOfLines={2} style={styles.aksiTeks}>
        {label}
      </AppText>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  tile: {
    flex: 1,
    minWidth: 140,
    borderRadius: radius.xl,
    borderWidth: StyleSheet.hairlineWidth,
    padding: spacing.lg,
    gap: spacing.xs,
  },
  kepala: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.xs,
  },
  label: {
    flex: 1,
  },
  aksi: {
    flexGrow: 1,
    flexBasis: 100,
    alignItems: "center",
    gap: spacing.sm,
    paddingVertical: spacing.lg,
    paddingHorizontal: spacing.sm,
    borderRadius: radius.xl,
    borderWidth: StyleSheet.hairlineWidth,
  },
  lingkaran: {
    width: 40,
    height: 40,
    borderRadius: radius.pill,
    alignItems: "center",
    justifyContent: "center",
  },
  aksiTeks: {
    textAlign: "center",
  },
});

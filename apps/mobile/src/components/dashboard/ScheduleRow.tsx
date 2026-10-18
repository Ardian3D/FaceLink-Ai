import React from "react";
import { StyleSheet, View } from "react-native";
import { Ionicons } from "@expo/vector-icons";

import { AppText } from "@/components/AppText";
import { useTheme , radius, spacing, touch } from "@/theme";

/**
 * Baris jadwal/sesi bergaya kartu.
 *
 * Susunannya mengikuti referensi: jam di kiri sebagai kolom sempit, isi di
 * tengah, dan penanda status di kanan. Tinggi minimum mengikuti target sentuh
 * agar tetap nyaman bila baris ini nanti bisa ditekan.
 */
export function ScheduleRow({
  time,
  title,
  subtitle,
  badge,
  highlight = false,
}: {
  time: string;
  title: string;
  subtitle?: string;
  badge?: string;
  /** Baris yang sedang berjalan ditandai aksen, bukan hanya teks tebal. */
  highlight?: boolean;
}) {
  const { colors } = useTheme();

  return (
    <View
      style={[
        styles.baris,
        {
          backgroundColor: highlight ? colors.accentGlow : colors.surface,
          borderColor: highlight ? colors.accent : colors.border,
        },
      ]}
    >
      <View style={[styles.jam, { borderRightColor: colors.border }]}>
        <AppText variant="caption" weight="semibold" color={highlight ? colors.accent : colors.textSecondary}>
          {time}
        </AppText>
      </View>

      <View style={styles.isi}>
        <AppText variant="body" weight="semibold" numberOfLines={1}>
          {title}
        </AppText>
        {subtitle ? (
          <AppText variant="caption" color={colors.textMuted} numberOfLines={1}>
            {subtitle}
          </AppText>
        ) : null}
      </View>

      {badge ? (
        <View style={[styles.badge, { borderColor: highlight ? colors.accent : colors.border }]}>
          <Ionicons
            name={highlight ? "radio-button-on" : "ellipse-outline"}
            size={12}
            color={highlight ? colors.accent : colors.textMuted}
          />
          <AppText
            variant="micro"
            weight="medium"
            color={highlight ? colors.accent : colors.textMuted}
          >
            {badge}
          </AppText>
        </View>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  baris: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.md,
    minHeight: touch.minHeight,
    paddingVertical: spacing.md,
    paddingHorizontal: spacing.lg,
    borderRadius: radius.lg,
    borderWidth: StyleSheet.hairlineWidth,
  },
  jam: {
    minWidth: 52,
    paddingRight: spacing.md,
    borderRightWidth: StyleSheet.hairlineWidth,
  },
  isi: {
    flex: 1,
    gap: 2,
  },
  badge: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.xs,
    paddingHorizontal: spacing.sm,
    paddingVertical: 3,
    borderRadius: radius.pill,
    borderWidth: StyleSheet.hairlineWidth,
  },
});

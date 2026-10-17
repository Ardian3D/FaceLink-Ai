import React from "react";
import { Pressable, StyleSheet, View } from "react-native";
import { Ionicons } from "@expo/vector-icons";

import { AppText } from "./AppText";
import { colors, radius, spacing, touch } from "@/theme";

/**
 * Baris daftar serbaguna: ikon, judul, keterangan, dan tanda panah opsional.
 * Dipakai untuk aktivitas terakhir, daftar kelas, dan pintasan pengelolaan.
 */
export function ListRow({
  title,
  subtitle,
  icon,
  trailing,
  onPress,
  tone = colors.textPrimary,
}: {
  title: string;
  subtitle?: string;
  icon?: keyof typeof Ionicons.glyphMap;
  trailing?: React.ReactNode;
  onPress?: () => void;
  tone?: string;
}) {
  const isi = (
    <View style={styles.row}>
      {icon ? (
        <View style={styles.ikon}>
          <Ionicons name={icon} size={18} color={colors.textSecondary} />
        </View>
      ) : null}
      <View style={styles.teks}>
        <AppText variant="body" weight="medium" color={tone}>
          {title}
        </AppText>
        {subtitle ? (
          <AppText variant="caption" color={colors.textMuted}>
            {subtitle}
          </AppText>
        ) : null}
      </View>
      {trailing}
      {onPress ? <Ionicons name="chevron-forward" size={18} color={colors.textMuted} /> : null}
    </View>
  );

  if (!onPress) return isi;

  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={title}
      onPress={onPress}
      hitSlop={touch.hitSlop}
      style={({ pressed }) => [styles.pressable, pressed && styles.pressed]}
    >
      {isi}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  pressable: {
    borderRadius: radius.md,
  },
  pressed: {
    backgroundColor: colors.surfaceMuted,
  },
  row: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.md,
    minHeight: touch.minHeight,
    paddingVertical: spacing.sm,
  },
  ikon: {
    width: 32,
    height: 32,
    borderRadius: radius.pill,
    backgroundColor: colors.surfaceMuted,
    alignItems: "center",
    justifyContent: "center",
  },
  teks: {
    flex: 1,
    gap: 2,
  },
});

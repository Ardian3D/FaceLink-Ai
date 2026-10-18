import React from "react";
import { StyleSheet, View } from "react-native";

import { AppText } from "@/components/AppText";
import { useTheme , spacing } from "@/theme";

/**
 * Judul bagian dashboard.
 *
 * Judul dan aksi berada pada satu baris, dengan aksi berbentuk teks kecil di
 * kanan ("Lihat semua"), mengikuti referensi dashboard. Bila tidak ada aksi
 * nyata, `action` tidak diisi dan tidak ada elemen kosong yang ikut dirender.
 */
export function SectionTitle({
  title,
  action,
  onAction,
}: {
  title: string;
  action?: string;
  onAction?: () => void;
}) {
  const { colors } = useTheme();

  return (
    <View style={styles.baris}>
      <AppText variant="title" weight="semibold">
        {title}
      </AppText>
      {action ? (
        <AppText
          variant="caption"
          weight="medium"
          color={onAction ? colors.accent : colors.textMuted}
          onPress={onAction}
          accessibilityRole={onAction ? "button" : undefined}
          suppressHighlighting
        >
          {action}
        </AppText>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  baris: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    gap: spacing.md,
  },
});

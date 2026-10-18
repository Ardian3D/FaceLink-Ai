import React from "react";
import { ScrollView, Pressable, StyleSheet } from "react-native";

import { AppText } from "@/components/AppText";
import { useTheme , radius, spacing, touch } from "@/theme";

/**
 * Pil tab horisontal ("Semua", "Hari ini", "Mendatang").
 *
 * Dipakai sebagai penyaring tampilan, bukan navigasi halaman: jumlah tab
 * sengaja sedikit supaya bisa digeser dengan satu jempol tanpa kehilangan
 * konteks. Tab aktif ditandai isian penuh **dan** bobot teks, bukan warna saja.
 */
export function PillTabs({
  items,
  activeId,
  onChange,
}: {
  items: { id: string; label: string }[];
  activeId: string;
  onChange: (id: string) => void;
}) {
  const { colors } = useTheme();

  return (
    <ScrollView
      horizontal
      showsHorizontalScrollIndicator={false}
      contentContainerStyle={styles.baris}
    >
      {items.map((t) => {
        const aktif = t.id === activeId;
        return (
          <Pressable
            key={t.id}
            accessibilityRole="tab"
            accessibilityState={{ selected: aktif }}
            accessibilityLabel={t.label}
            onPress={() => onChange(t.id)}
            hitSlop={touch.hitSlop}
            style={({ pressed }) => [
              styles.pil,
              {
                backgroundColor: aktif ? colors.accent : colors.surface,
                borderColor: aktif ? colors.accent : colors.border,
              },
              pressed && { opacity: 0.85 },
            ]}
          >
            <AppText
              variant="caption"
              weight={aktif ? "semibold" : "medium"}
              color={aktif ? colors.textOnAccent : colors.textSecondary}
            >
              {t.label}
            </AppText>
          </Pressable>
        );
      })}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  baris: {
    flexDirection: "row",
    gap: spacing.sm,
    paddingRight: spacing.lg,
  },
  pil: {
    paddingHorizontal: spacing.lg,
    paddingVertical: spacing.sm,
    borderRadius: radius.pill,
    borderWidth: StyleSheet.hairlineWidth,
    minHeight: 36,
    justifyContent: "center",
  },
});

import React from "react";
import { Pressable, StyleSheet, View } from "react-native";
import { Ionicons } from "@expo/vector-icons";

import { AppText } from "@/components/AppText";
import { BrandLogo } from "@/components/BrandLogo";
import { useSession } from "@/features/auth/SessionProvider";
import { useTheme , layout, radius, spacing, touch } from "@/theme";

/**
 * Topbar dashboard.
 *
 * Sesuai permintaan pemilik: hanya logo FaceLink di kiri dan profil pengguna di
 * kanan. Tidak ada judul di tengah, tidak ada deretan ikon — supaya isi
 * dashboard mendapat ruang dan tidak ada dua navigasi yang bersaing.
 *
 * Profil di kanan membuka lembar pengaturan (tema dan keluar). Pada integrasi
 * nanti lembar yang sama menjadi tempat menu akun.
 */
export function TopBar({ onOpenSettings }: { onOpenSettings: () => void }) {
  const { colors } = useTheme();
  const { session } = useSession();

  const inisial = (session?.user.displayName ?? "?")
    .split(" ")
    .filter(Boolean)
    .slice(0, 2)
    .map((k) => k[0]?.toUpperCase() ?? "")
    .join("");

  return (
    <View style={[styles.bar, { borderBottomColor: colors.border }]}>
      <View style={styles.kiri}>
        <BrandLogo size={layout.logoSizeBar} showWordmark={false} />
        <AppText variant="bodyLarge" weight="bold">
          FaceLink
        </AppText>
        <AppText variant="bodyLarge" weight="bold" color={colors.accent}>
          Ai
        </AppText>
      </View>

      <Pressable
        accessibilityRole="button"
        accessibilityLabel={`Profil ${session?.user.displayName ?? "pengguna"}, buka pengaturan`}
        onPress={onOpenSettings}
        hitSlop={touch.hitSlop}
        style={({ pressed }) => [
          styles.profil,
          { backgroundColor: colors.surfaceMuted, borderColor: colors.border },
          pressed && { opacity: 0.75 },
        ]}
      >
        <AppText variant="body" weight="semibold">
          {inisial || "?"}
        </AppText>
        <Ionicons name="settings-outline" size={16} color={colors.textSecondary} />
      </Pressable>
    </View>
  );
}

const styles = StyleSheet.create({
  bar: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    gap: spacing.md,
    paddingVertical: spacing.sm,
    borderBottomWidth: StyleSheet.hairlineWidth,
  },
  kiri: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.xs,
  },
  profil: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.sm,
    minHeight: 36,
    paddingHorizontal: spacing.md,
    borderRadius: radius.pill,
    borderWidth: StyleSheet.hairlineWidth,
  },
});

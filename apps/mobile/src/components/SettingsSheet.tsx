import React from "react";
import { Modal, Pressable, StyleSheet, View } from "react-native";
import { Ionicons } from "@expo/vector-icons";

import { AppText } from "@/components/AppText";
import { useTheme, type ThemeMode } from "@/theme";
import { radius, spacing, touch } from "@/theme";

/**
 * Lembar pengaturan dari topbar.
 *
 * Isinya sengaja hanya dua hal yang benar-benar berfungsi sekarang: pilihan
 * tema dan keluar. Menu akun lain belum ada layarnya, jadi tidak ditampilkan —
 * lebih baik satu lembar pendek yang jujur daripada daftar menu yang tidak
 * menuju ke mana-mana.
 *
 * Pilihan tema memakai dua tombol bersebelahan, bukan satu sakelar: pengguna
 * melihat kedua hasilnya sekaligus dan tahu mana yang sedang aktif.
 */
export function SettingsSheet({
  visible,
  onClose,
  onSignOut,
  onOpenDiagnostics,
}: {
  visible: boolean;
  onClose: () => void;
  onSignOut: () => void;
  /** Hanya diberikan pada development build; membuka layar diagnostik. */
  onOpenDiagnostics: () => void;
}) {
  const { colors, mode, setMode } = useTheme();

  const pilihan: {
    nilai: ThemeMode;
    label: string;
    ikon: keyof typeof Ionicons.glyphMap;
  }[] = [
    { nilai: "dark", label: "Gelap", ikon: "moon" },
    { nilai: "light", label: "Terang", ikon: "sunny" },
  ];

  return (
    <Modal
      visible={visible}
      transparent
      animationType="fade"
      onRequestClose={onClose}
    >
      <Pressable
        style={styles.latar}
        onPress={onClose}
        accessibilityLabel="Tutup pengaturan"
      >
        <Pressable
          style={[
            styles.lembar,
            {
              backgroundColor: colors.surfaceElevated,
              borderColor: colors.border,
            },
          ]}
          onPress={() => {}}
        >
          <View style={styles.judul}>
            <AppText variant="title" weight="semibold">
              Pengaturan
            </AppText>
            <Pressable
              onPress={onClose}
              accessibilityRole="button"
              accessibilityLabel="Tutup"
              hitSlop={touch.hitSlop}
            >
              <Ionicons name="close" size={22} color={colors.textSecondary} />
            </Pressable>
          </View>

          <View style={styles.bagian}>
            <AppText variant="caption" weight="medium" color={colors.textMuted}>
              TEMA TAMPILAN
            </AppText>
            <View style={styles.pilihanBaris}>
              {pilihan.map((p) => {
                const aktif = mode === p.nilai;
                return (
                  <Pressable
                    key={p.nilai}
                    accessibilityRole="radio"
                    accessibilityState={{ selected: aktif }}
                    accessibilityLabel={`Tema ${p.label}`}
                    onPress={() => setMode(p.nilai)}
                    style={({ pressed }) => [
                      styles.pilihan,
                      {
                        borderColor: aktif ? colors.accent : colors.border,
                        backgroundColor: aktif
                          ? colors.accentGlow
                          : colors.surfaceMuted,
                      },
                      pressed && { opacity: 0.8 },
                    ]}
                  >
                    <Ionicons
                      name={p.ikon}
                      size={20}
                      color={aktif ? colors.accent : colors.textSecondary}
                    />
                    <AppText
                      variant="body"
                      weight={aktif ? "semibold" : "regular"}
                      color={aktif ? colors.accent : colors.textSecondary}
                    >
                      {p.label}
                    </AppText>
                    {aktif ? (
                      <Ionicons
                        name="checkmark-circle"
                        size={18}
                        color={colors.accent}
                      />
                    ) : null}
                  </Pressable>
                );
              })}
            </View>
            <AppText variant="caption" color={colors.textMuted}>
              Pilihan disimpan di perangkat ini dan berlaku pada semua layar.
            </AppText>
          </View>

          {/* Diagnostik build development. Tidak ada pemilih role di sini:
              satu akun hanya memiliki satu role utama (D-037), dan role hanya
              ditetapkan backend. */}
          {__DEV__ ? (
            <Pressable
              accessibilityRole="button"
              accessibilityLabel="Buka diagnostik build"
              onPress={onOpenDiagnostics}
              style={({ pressed }) => [
                styles.keluar,
                { borderColor: colors.border },
                pressed && { backgroundColor: colors.surfaceMuted },
              ]}
            >
              <Ionicons
                name="construct-outline"
                size={20}
                color={colors.textSecondary}
              />
              <AppText variant="bodyLarge" weight="medium">
                Diagnostik (dev)
              </AppText>
            </Pressable>
          ) : null}

          <Pressable
            accessibilityRole="button"
            accessibilityLabel="Keluar dari akun"
            onPress={onSignOut}
            style={({ pressed }) => [
              styles.keluar,
              { borderColor: colors.border },
              pressed && { backgroundColor: colors.surfaceMuted },
            ]}
          >
            <Ionicons name="log-out-outline" size={20} color={colors.error} />
            <AppText variant="bodyLarge" weight="medium" color={colors.error}>
              Keluar
            </AppText>
          </Pressable>
        </Pressable>
      </Pressable>
    </Modal>
  );
}

const styles = StyleSheet.create({
  latar: {
    flex: 1,
    backgroundColor: "rgba(0,0,0,0.55)",
    justifyContent: "flex-end",
    padding: spacing.lg,
  },
  lembar: {
    borderRadius: radius.xxl,
    borderWidth: StyleSheet.hairlineWidth,
    padding: spacing.xl,
    gap: spacing.xl,
  },
  judul: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
  },
  bagian: {
    gap: spacing.sm,
  },
  pilihanBaris: {
    flexDirection: "row",
    gap: spacing.sm,
  },
  pilihan: {
    flex: 1,
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    gap: spacing.sm,
    minHeight: touch.minHeight,
    borderRadius: radius.lg,
    borderWidth: 1.5,
  },
  keluar: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    gap: spacing.sm,
    minHeight: touch.minHeight,
    borderRadius: radius.lg,
    borderWidth: StyleSheet.hairlineWidth,
  },
});

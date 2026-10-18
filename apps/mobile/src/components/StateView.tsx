import React from "react";
import { ActivityIndicator, StyleSheet, View } from "react-native";
import { Ionicons } from "@expo/vector-icons";

import { AppText } from "./AppText";
import { radius, spacing, useTheme } from "@/theme";

/**
 * Keadaan UI yang harus punya tampilan jelas (pedoman UX §3).
 *
 * `loading`, `empty`, dan `error` dibedakan strukturnya — bukan hanya teksnya —
 * sehingga pengguna tidak pernah menebak apakah data kosong karena belum ada
 * atau karena gagal dimuat.
 */
export function StateView({
  tone,
  title,
  description,
  action,
}: {
  tone: "loading" | "empty" | "error";
  title: string;
  description?: string;
  action?: React.ReactNode;
}) {
  const { colors } = useTheme();

  return (
    <View
      style={[
        styles.wrap,
        { backgroundColor: colors.surface, borderColor: colors.border },
      ]}
    >
      {tone === "loading" ? (
        <ActivityIndicator color={colors.accent} />
      ) : (
        <Ionicons
          name={tone === "empty" ? "file-tray-outline" : "alert-circle-outline"}
          size={28}
          color={tone === "empty" ? colors.textMuted : colors.error}
        />
      )}
      <AppText variant="bodyLarge" weight="semibold" center>
        {title}
      </AppText>
      {description ? (
        <AppText variant="caption" color={colors.textMuted} center>
          {description}
        </AppText>
      ) : null}
      {action}
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: {
    alignItems: "center",
    justifyContent: "center",
    gap: spacing.sm,
    paddingVertical: spacing.xl,
    paddingHorizontal: spacing.lg,
    borderRadius: radius.lg,
    borderWidth: StyleSheet.hairlineWidth,
  },
});

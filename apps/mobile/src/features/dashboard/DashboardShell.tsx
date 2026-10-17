import React, { useEffect } from "react";
import { useRouter } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { StyleSheet, View } from "react-native";

import { AppText } from "@/components/AppText";
import { AppButton } from "@/components/AppButton";
import { DemoBanner } from "@/components/DemoBanner";
import { Screen } from "@/components/Screen";
import { StatusBadge } from "@/components/StatusBadge";
import { useSession } from "@/features/auth/SessionProvider";
import { DEMO_MODE } from "@/features/auth/demoMode";
import { roleHome } from "@/features/auth/roleRoutes";
import { colors, spacing } from "@/theme";
import type { Role } from "@/types";

/**
 * Kerangka beranda untuk satu role.
 *
 * Menjaga agar layar role hanya dapat dibuka oleh role yang sesuai: bila sesi
 * ternyata milik role lain, pengguna diarahkan ke berandanya sendiri. Satu akun
 * hanya punya satu role utama (D-037), sehingga tidak ada perpindahan role di
 * dalam sesi.
 */
export function DashboardShell({
  role,
  title,
  subtitle,
  children,
}: {
  role: Role;
  title: string;
  subtitle: string;
  children: React.ReactNode;
}) {
  const router = useRouter();
  const { session, signOut } = useSession();
  const sesiRole = session?.user.role;

  useEffect(() => {
    if (sesiRole && sesiRole !== role) router.replace(roleHome[sesiRole]);
  }, [role, router, sesiRole]);

  if (!session) return null;

  return (
    <Screen scroll>
      <View style={styles.header}>
        <View style={styles.headerTeks}>
          <AppText variant="caption" color={colors.textMuted}>
            {subtitle}
          </AppText>
          <AppText variant="display" weight="bold">
            {title}
          </AppText>
        </View>
        <View style={styles.headerAksi}>
          <StatusBadge label={session.user.displayName} tone="info" icon="person-circle-outline" />
        </View>
      </View>

      {DEMO_MODE ? (
        <DemoBanner keterangan={`beranda ${role} memakai data contoh`} />
      ) : null}

      {children}

      <View style={styles.footer}>
        {DEMO_MODE ? (
          <AppButton
            label="Ganti role (dev)"
            variant="secondary"
            onPress={() => router.push("/dev-preview")}
            icon={<Ionicons name="swap-horizontal" size={18} color={colors.textPrimary} />}
          />
        ) : null}
        <AppButton
          label="Keluar"
          variant="secondary"
          onPress={() => {
            signOut();
            router.replace("/");
          }}
          icon={<Ionicons name="log-out-outline" size={18} color={colors.textPrimary} />}
        />
      </View>
    </Screen>
  );
}

const styles = StyleSheet.create({
  header: {
    flexDirection: "row",
    alignItems: "flex-end",
    justifyContent: "space-between",
    gap: spacing.md,
  },
  headerTeks: {
    flex: 1,
    gap: spacing.xs,
  },
  headerAksi: {
    alignItems: "flex-end",
  },
  footer: {
    gap: spacing.sm,
    paddingTop: spacing.lg,
  },
});

import React, { useEffect, useState } from "react";
import { useRouter } from "expo-router";
import { StyleSheet, View } from "react-native";

import { AppText } from "@/components/AppText";
import { Screen } from "@/components/Screen";
import { SettingsSheet } from "@/components/SettingsSheet";
import { TopBar } from "@/components/TopBar";
import { useSession } from "@/features/auth/SessionProvider";
import { roleHome } from "@/features/auth/roleRoutes";
import { spacing, useTheme } from "@/theme";
import type { Role } from "@/types";

/**
 * Kerangka beranda untuk satu role.
 *
 * Susunannya mengikuti referensi dashboard: topbar tipis (logo kiri, profil
 * kanan), sapaan besar, lalu isi yang disusun per bagian. Topbar sengaja hanya
 * memuat dua hal itu — pengaturan (termasuk tema) dibuka dari profil, bukan
 * dari deretan ikon di header.
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
  const { colors } = useTheme();
  const { session, signOut } = useSession();
  const [pengaturan, setPengaturan] = useState(false);
  const sesiRole = session?.user.role;

  useEffect(() => {
    if (sesiRole && sesiRole !== role) router.replace(roleHome[sesiRole]);
  }, [role, router, sesiRole]);

  if (!session) return null;

  return (
    <Screen scroll>
      <TopBar onOpenSettings={() => setPengaturan(true)} />

      <View style={styles.sapaan}>
        <AppText variant="caption" color={colors.textMuted}>
          {subtitle}
        </AppText>
        <AppText variant="display" weight="bold">
          {title}
        </AppText>
      </View>

      {children}

      <SettingsSheet
        visible={pengaturan}
        onClose={() => setPengaturan(false)}
        onSignOut={() => {
          setPengaturan(false);
          signOut();
          router.replace("/");
        }}
      />
    </Screen>
  );
}

const styles = StyleSheet.create({
  sapaan: {
    gap: spacing.xs,
    paddingTop: spacing.lg,
  },
});

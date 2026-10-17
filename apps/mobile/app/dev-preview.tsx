import React from "react";
import { useRouter } from "expo-router";
import { StyleSheet, View } from "react-native";

import { AppButton } from "@/components/AppButton";
import { AppText } from "@/components/AppText";
import { DemoBanner } from "@/components/DemoBanner";
import { Screen } from "@/components/Screen";
import { StateView } from "@/components/StateView";
import { useSession } from "@/features/auth/SessionProvider";
import { DEMO_MODE } from "@/features/auth/demoMode";
import { roleHome } from "@/features/auth/roleRoutes";
import { colors, spacing } from "@/theme";
import { ROLES, roleLabel, type Role } from "@/types";

/**
 * Pemilih role untuk preview (FL-UI01 §11).
 *
 * Batas keamanan: layar ini **hanya** ada pada development build (`DEMO_MODE`,
 * yaitu `__DEV__`). Pada build produksi ia menampilkan penjelasan dan tidak
 * menawarkan tombol apa pun, sehingga tidak bisa dipakai menaikkan hak akses.
 * Role yang dipilih di sini juga tidak pernah dikirim ke backend sebagai klaim;
 * pada integrasi nanti role tetap berasal dari sesi yang divalidasi Go Backend.
 */
export default function DevRolePreview() {
  const router = useRouter();
  const { previewAs } = useSession();

  if (!DEMO_MODE) {
    return (
      <Screen center>
        <StateView
          tone="error"
          title="Tidak tersedia pada build ini"
          description="Pemilih role hanya aktif pada development build."
        />
      </Screen>
    );
  }

  const pilih = (role: Role) => {
    previewAs(role);
    router.replace(roleHome[role]);
  };

  return (
    <Screen scroll>
      <DemoBanner keterangan="hanya development build" />
      <View style={styles.header}>
        <AppText variant="heading" weight="bold">
          Preview sebagai
        </AppText>
        <AppText variant="caption" color={colors.textMuted}>
          Pilih role untuk melihat berandanya. Tidak ada autentikasi yang dijalankan dan tidak ada
          token yang diterbitkan.
        </AppText>
      </View>

      <View style={styles.daftar}>
        {ROLES.map((role) => (
          <AppButton
            key={role}
            label={`Preview sebagai ${roleLabel[role]}`}
            variant="secondary"
            onPress={() => pilih(role)}
          />
        ))}
      </View>

      <AppButton label="Kembali" variant="secondary" onPress={() => router.back()} />
    </Screen>
  );
}

const styles = StyleSheet.create({
  header: {
    gap: spacing.sm,
  },
  daftar: {
    gap: spacing.sm,
  },
});

import React from "react";
import { useRouter } from "expo-router";
import { StyleSheet, View } from "react-native";

import { AppButton } from "@/components/AppButton";
import { AppText } from "@/components/AppText";
import { Screen } from "@/components/Screen";
import { StateView } from "@/components/StateView";
import { ConnectionDiagnostics } from "@/features/api/ConnectionDiagnostics";
import { useSession } from "@/features/auth/SessionProvider";
import { DEMO_MODE } from "@/features/auth/demoMode";
import { roleHome } from "@/features/auth/roleRoutes";
import { spacing, useTheme } from "@/theme";
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
  const { colors } = useTheme();
  const { previewAs, markUnregistered } = useSession();

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

      {/* Keadaan akun ditolak juga perlu dapat dipreview tanpa backend. Yang
            ditampilkan bukan galat teknis, melainkan layar penjelasan yang
            sama dengan jalur produksi; tidak ada sesi yang dibuat. */}
      <View style={styles.header}>
        <AppText variant="bodyLarge" weight="semibold">
          Akun ditolak
        </AppText>
        <AppText variant="caption" color={colors.textMuted}>
          Memperlihatkan layar yang muncul bila backend menjawab bahwa akun belum
          terdaftar atau sedang ditangguhkan. Tidak ada sesi yang dibuat.
        </AppText>
      </View>

      <View style={styles.daftar}>
        <AppButton
          label="Preview akun belum terdaftar"
          variant="secondary"
          onPress={() => {
            markUnregistered("notGranted", "belum.terdaftar@example.test");
            router.replace("/not-registered");
          }}
        />
        <AppButton
          label="Preview akun ditangguhkan"
          variant="secondary"
          onPress={() => {
            markUnregistered("suspended", "ditangguhkan@example.test");
            router.replace("/not-registered");
          }}
        />
      </View>

      {/* Diagnostik koneksi Go Backend (FL-NET01). Ditempatkan di layar preview
          development, bukan di dashboard mana pun, dan hanya dirender di sini:
          pada build produksi `DEMO_MODE` sudah memaksa layar ini menampilkan
          "tidak tersedia" sebelum blok ini tercapai. */}
      <View style={styles.header}>
        <AppText variant="bodyLarge" weight="semibold">
          Koneksi Go Backend
        </AppText>
        <AppText variant="caption" color={colors.textMuted}>
          Dua permintaan nyata ke `healthz` dan `readyz`. Tidak ada login, token,
          atau data peserta yang dikirim.
        </AppText>
      </View>
      <ConnectionDiagnostics />

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

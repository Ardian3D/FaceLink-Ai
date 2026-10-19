import React from "react";
import { useRouter } from "expo-router";
import { StyleSheet, View } from "react-native";
import { AppButton } from "@/components/AppButton";
import { AppText } from "@/components/AppText";
import { Screen } from "@/components/Screen";
import { ConnectionDiagnostics } from "@/features/api/ConnectionDiagnostics";
import { useSession } from "@/features/auth/SessionProvider";
import { konfigurasiApi } from "@/features/api/config";
import { konfigurasiGoogle } from "@/features/auth/googleConfig";
import { radius, spacing, useTheme } from "@/theme";

/**
 * Diagnostik build development.
 *
 * Sejak FL-UI02 berkas ini **tidak lagi** memilih role: satu akun hanya punya
 * satu role utama (D-037), dan role hanya ditetapkan backend. Yang tersisa
 * adalah hal-hal yang memang hanya berguna saat mengembangkan di perangkat:
 *
 *  - konfigurasi alamat backend yang benar-benar dipakai bundel ini;
 *  - konfigurasi Web OAuth client ID untuk Google Sign-In;
 *  - dua permintaan nyata ke `healthz` dan `readyz`.
 *
 * Layar ini tidak menampilkan token, bukan karena belum sempat, tetapi karena
 * token memang tidak pernah melewatinya: ia hidup di penyimpanan kredensial
 * platform dan hanya dibaca lapisan transport.
 *
 * Batas keamanan: seluruh isi layar hanya ada pada development build
 * (`__DEV__`). Pada build produksi ia menampilkan penjelasan dan tidak
 * menawarkan apa pun, sehingga tidak dapat dipakai sebagai alat bantu masuk.
 */
export default function DevDiagnostics() {
  const router = useRouter();
  const { colors } = useTheme();
  const { status, session } = useSession();

  if (!__DEV__) {
    return (
      <Screen center>
        <AppText variant="bodyLarge" weight="semibold" center>
          Tidak tersedia pada build ini
        </AppText>
        <AppText variant="caption" color={colors.textMuted} center>
          Diagnostik hanya aktif pada development build.
        </AppText>
      </Screen>
    );
  }

  return (
    <Screen scroll>
      <View style={styles.header}>
        <AppText variant="heading" weight="bold">
          Diagnostik
        </AppText>
        <AppText variant="caption" color={colors.textMuted}>
          Keadaan nyata build ini. Tidak ada token atau data peserta yang
          ditampilkan.
        </AppText>
      </View>

      <View
        style={[
          styles.kotak,
          { backgroundColor: colors.surface, borderColor: colors.border },
        ]}
      >
        <AppText variant="caption" color={colors.textMuted}>
          Alamat backend
        </AppText>
        <AppText variant="body" weight="semibold">
          {konfigurasiApi.ok ? konfigurasiApi.baseUrl : "tidak sah"}
        </AppText>
        <AppText variant="caption" color={colors.textMuted}>
          {konfigurasiApi.ok
            ? konfigurasiApi.sumber === "environment"
              ? "dari EXPO_PUBLIC_FACELINK_API_BASE_URL"
              : "nilai bawaan development (loopback)"
            : konfigurasiApi.alasan}
        </AppText>
      </View>

      <View
        style={[
          styles.kotak,
          { backgroundColor: colors.surface, borderColor: colors.border },
        ]}
      >
        <AppText variant="caption" color={colors.textMuted}>
          Web OAuth client ID (Google Sign-In)
        </AppText>
        <AppText variant="body" weight="semibold">
          {konfigurasiGoogle.ok ? konfigurasiGoogle.webClientId : "belum disetel"}
        </AppText>
        {!konfigurasiGoogle.ok ? (
          <AppText variant="caption" color={colors.textMuted}>
            {konfigurasiGoogle.alasan}
          </AppText>
        ) : null}
      </View>

      <View
        style={[
          styles.kotak,
          { backgroundColor: colors.surface, borderColor: colors.border },
        ]}
      >
        <AppText variant="caption" color={colors.textMuted}>
          Sesi saat ini
        </AppText>
        <AppText variant="body" weight="semibold">
          {session === null ? status : `${session.user.role} · ${session.user.displayName}`}
        </AppText>
      </View>

      {/* Diagnostik koneksi Go Backend (FL-NET01): dua permintaan sungguhan.
          Ditempatkan di layar development, bukan di dashboard mana pun. */}
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
  kotak: {
    gap: spacing.xs,
    padding: spacing.md,
    borderRadius: radius.md,
    borderWidth: StyleSheet.hairlineWidth,
  },
});

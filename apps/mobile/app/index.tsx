import React from "react";
import { useRouter } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { ImageBackground, StyleSheet, View } from "react-native";

import { AppButton } from "@/components/AppButton";
import { AppText } from "@/components/AppText";
import { BrandLogo } from "@/components/BrandLogo";
import { DemoBanner } from "@/components/DemoBanner";
import { Screen } from "@/components/Screen";
import { StateView } from "@/components/StateView";
import { useSession } from "@/features/auth/SessionProvider";
import { DEMO_MODE } from "@/features/auth/demoMode";
import { colors, radius, spacing } from "@/theme";

/**
 * Layar masuk (FL-UI01 §7).
 *
 * Tombol "Continue with Google" belum memanggil Google: pada development build
 * ia memulai alur demo lokal, dan pada build produksi ia berhenti dengan pesan
 * bahwa integrasi belum tersedia — tanpa pernah memberi sesi. Tidak ada tombol
 * "Skip" dan tidak ada jalur masuk lain.
 */
export default function LoginScreen() {
  const router = useRouter();
  const { status, error, startSignIn } = useSession();

  const mengecek = status === "checking";

  const mulai = () => {
    startSignIn();
    if (DEMO_MODE) router.push("/role-checking");
  };

  return (
    <Screen scroll contentStyle={styles.isi}>
      {/* Ruang background visual demo. Saat pemilik memberikan foto orang
          sedang memindai wajah di perangkat FaceLink, gambar itu dipasang di
          lapisan ini tanpa mengubah layout di bawahnya. */}
      <View style={styles.latarVisual} pointerEvents="none">
        <ImageBackground
          source={require("../assets/branding/logo-mark.png")}
          style={styles.latarGambar}
          imageStyle={styles.latarGambarIsi}
          resizeMode="contain"
        />
      </View>

      <View style={styles.header}>
        {DEMO_MODE ? <DemoBanner keterangan="UI preview, belum terhubung backend" /> : null}
        <BrandLogo size={96} />
      </View>

      <View style={styles.aksi}>
        <AppButton
          label="Continue with Google"
          onPress={mulai}
          loading={mengecek}
          icon={<Ionicons name="logo-google" size={20} color={colors.onAccent} />}
        />
        <AppText variant="caption" color={colors.textMuted} center>
          Masuk memerlukan internet. Role akun ditentukan backend FaceLink AI, bukan oleh aplikasi.
        </AppText>

        {status === "failed" && error ? (
          <StateView
            tone="error"
            title={
              error.kind === "unknown" && error.message === "notImplemented"
                ? "Belum tersedia pada build ini"
                : "Gagal memulai"
            }
            description={
              error.kind === "unknown" && error.message === "notImplemented"
                ? "Masuk dengan Google belum diintegrasikan. Jalur demo hanya aktif pada development build."
                : "Layanan identitas tidak dapat dihubungi saat ini. Coba lagi nanti."
            }
          />
        ) : null}
      </View>
    </Screen>
  );
}

const styles = StyleSheet.create({
  isi: {
    justifyContent: "space-between",
  },
  latarVisual: {
    position: "absolute",
    top: 0,
    right: 0,
    bottom: 0,
    left: 0,
    alignItems: "center",
    justifyContent: "center",
    opacity: 0.07,
  },
  latarGambar: {
    width: "120%",
    height: "70%",
  },
  latarGambarIsi: {
    borderRadius: radius.pill,
  },
  header: {
    gap: spacing.xl,
    paddingTop: spacing.xxl,
    alignItems: "center",
  },
  aksi: {
    gap: spacing.md,
    paddingBottom: spacing.lg,
  },
});

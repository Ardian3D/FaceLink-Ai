import React from "react";
import { useRouter } from "expo-router";
import { StyleSheet, View } from "react-native";
import { LinearGradient } from "expo-linear-gradient";

import { AppText } from "@/components/AppText";
import { BrandLogo } from "@/components/BrandLogo";
import { DemoBanner } from "@/components/DemoBanner";
import { GoogleSignInButton } from "@/components/GoogleSignInButton";
import { Screen } from "@/components/Screen";
import { StateView } from "@/components/StateView";
import { VideoBackground } from "@/components/VideoBackground";
import { useSession } from "@/features/auth/SessionProvider";
import { DEMO_MODE } from "@/features/auth/demoMode";
import { colors, layout, spacing } from "@/theme";

/**
 * Layar masuk (FL-UI01 §7), mengikuti mockup "FaceLink AI Sign-in Page".
 *
 * Seluruh layar ditutup latar bergerak — klip video pemilik project bila
 * tersedia di mesin ini, atau latar animasi kode bila tidak (lihat
 * `assets/video/BACA-SAYA.md`) — lalu diberi lapisan gelap agar logo, wordmark,
 * dan tombol tetap terbaca. Lapisan itu makin pekat ke bawah, jadi bagian atas
 * video tetap terlihat.
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
  const belumTersedia = error?.kind === "unknown" && error.message === "notImplemented";

  const mulai = () => {
    startSignIn();
    if (DEMO_MODE) router.push("/role-checking");
  };

  return (
    <View style={styles.layar}>
      <VideoBackground />
      <LinearGradient
        colors={[
          colors.overlayLight,
          colors.overlayMedium,
          colors.overlayStrong,
          colors.overlayStrong,
        ]}
        locations={[0, 0.28, 0.55, 1]}
        style={StyleSheet.absoluteFill}
        pointerEvents="none"
      />

      <Screen transparent scroll contentStyle={styles.isi}>
        <View style={styles.atas}>
          {DEMO_MODE ? <DemoBanner keterangan="UI preview, belum terhubung backend" /> : null}
        </View>

        {/* Dua ruang kosong dengan rasio berbeda menempatkan blok identitas di
            bawah titik tengah layar, seperti pada mockup. Rasio dipakai, bukan
            `justifyContent: space-between`, supaya posisinya tetap sama pada
            layar pendek maupun tinggi. */}
        <View style={styles.ruangAtas} />

        <View style={styles.tengah}>
          <BrandLogo size={layout.logoSize} />

          <View style={styles.aksi}>
            <GoogleSignInButton onPress={mulai} loading={mengecek} />
            <AppText variant="caption" color={colors.textSecondary} center>
              Masuk memerlukan internet. Role akun ditentukan backend FaceLink AI, bukan oleh
              aplikasi.
            </AppText>

            {error ? (
              <StateView
                tone="error"
                title={belumTersedia ? "Belum tersedia pada build ini" : "Gagal memulai"}
                description={
                  belumTersedia
                    ? "Masuk dengan Google belum diintegrasikan. Jalur demo hanya aktif pada development build."
                    : "Layanan identitas tidak dapat dihubungi saat ini. Coba lagi nanti."
                }
              />
            ) : null}
          </View>
        </View>

        <View style={styles.ruangBawah} />

        <View style={styles.bawah}>
          {/* Mockup menampilkan tautan kebijakan di dasar layar. Dokumen publiknya
              belum ada (docs/ baru memuat dokumen internal), jadi keduanya
              ditampilkan sebagai label — bukan tautan yang bisa ditekan dan tidak
              menuju ke mana-mana. Beri tahu pemilik project bila dokumennya sudah
              tersedia supaya baris ini berubah menjadi tautan sungguhan. */}
          <View style={styles.footer}>
            <AppText variant="caption" color={colors.textMuted}>
              Privacy policy
            </AppText>
            <View style={styles.pemisah} />
            <AppText variant="caption" color={colors.textMuted}>
              Terms of service
            </AppText>
          </View>
          <AppText variant="caption" color={colors.textMuted} center>
            Tautan kebijakan belum aktif pada preview.
          </AppText>
        </View>
      </Screen>
    </View>
  );
}

const styles = StyleSheet.create({
  layar: {
    flex: 1,
    backgroundColor: colors.background,
  },
  isi: {
    justifyContent: "flex-start",
    gap: spacing.lg,
    paddingTop: spacing.sm,
    paddingBottom: spacing.xl,
  },
  atas: {
    minHeight: spacing.xl,
  },
  /**
   * Ruang kosong di atas dan di bawah blok identitas. Perbandingan 7:1
   * menaruh mark di sekitar 55% tinggi layar, wordmark di ~66%, tombol di
   * ~79%, dan label kebijakan di ~89% — posisi yang diukur dari mockup
   * pemilik. Karena memakai `flex`, proporsinya bertahan pada layar pendek
   * maupun tinggi.
   */
  ruangAtas: {
    flex: 1,
    minHeight: spacing.xxl,
  },
  ruangBawah: {
    height: spacing.md,
  },
  tengah: {
    alignItems: "center",
    gap: spacing.xxxl,
  },
  aksi: {
    width: "100%",
    gap: spacing.md,
  },
  bawah: {
    gap: spacing.sm,
  },
  footer: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    gap: spacing.md,
  },
  pemisah: {
    width: StyleSheet.hairlineWidth,
    height: spacing.lg,
    backgroundColor: colors.border,
  },
});

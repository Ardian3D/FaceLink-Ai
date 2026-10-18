import React from "react";
import { Redirect, useRouter } from "expo-router";
import { StyleSheet, View } from "react-native";
import { Ionicons } from "@expo/vector-icons";

import { AppButton } from "@/components/AppButton";
import { AppText } from "@/components/AppText";
import { BrandLogo } from "@/components/BrandLogo";
import { DemoBanner } from "@/components/DemoBanner";
import { Screen } from "@/components/Screen";
import { UnregisteredIllustration } from "@/components/UnregisteredIllustration";
import { DEMO_MODE } from "@/features/auth/demoMode";
import { useSession } from "@/features/auth/SessionProvider";
import { layout, radius, spacing, useTheme } from "@/theme";

/**
 * Layar untuk akun yang identitas Google-nya sah tetapi **belum terdaftar**
 * di FaceLink AI.
 *
 * Ini keadaan yang disengaja, bukan galat teknis: role hanya ditetapkan admin
 * (D-037), jadi pengguna yang belum didaftarkan tidak boleh mendapat sesi,
 * beranda kosong, atau jalan masuk alternatif. Layar ini menjelaskan hal itu
 * dengan tenang dan memberi satu langkah yang bisa dilakukan pengguna.
 *
 * Bentuknya mengikuti referensi empty state: ilustrasi di tengah, judul tebal
 * huruf besar, keterangan redup, lalu satu tombol. Tidak ada tombol "coba lagi"
 * yang menyamar sebagai solusi — mengulang login tidak akan mengubah apa pun
 * sampai admin mendaftarkan akunnya.
 */
export default function NotRegisteredScreen() {
  const router = useRouter();
  const { colors } = useTheme();
  const { status, error, signOut } = useSession();

  // Layar ini hanya bermakna bila sesi memang berakhir dengan penolakan akun.
  // Dibuka langsung lewat tautan/riwayat, ia tidak boleh menampilkan pesan
  // "belum terdaftar" untuk orang yang sebenarnya hanya belum masuk.
  if (status !== "unregistered") return <Redirect href="/" />;

  // Email hanya ditampilkan bila memang diketahui; tanpa email, pesan tetap
  // utuh dan tidak ada baris kosong.
  const email = error?.kind === "notGranted" || error?.kind === "suspended" ? error.email : undefined;
  const ditangguhkan = error?.kind === "suspended";

  return (
    <Screen scroll contentStyle={styles.isi}>
      {DEMO_MODE ? <DemoBanner keterangan="layar ini dapat dipreview tanpa backend" /> : null}

      <View style={styles.atas}>
        <BrandLogo size={layout.logoSizeCompact} showWordmark />
      </View>

      <View style={styles.tengah}>
        <UnregisteredIllustration />

        <View style={styles.teks}>
          <AppText variant="heading" weight="bold" center style={styles.judul}>
            {ditangguhkan ? "AKUN ANDA DITANGGUHKAN" : "ANDA BELUM TERDAFTAR"}
          </AppText>
          <AppText variant="body" color={colors.textMuted} center>
            {ditangguhkan
              ? "Akun Anda terdaftar tetapi sedang ditangguhkan. Hubungi admin FaceLink AI untuk memulihkannya."
              : "Wah, akun Anda belum terdaftar nih. Hubungi admin lebih dahulu untuk mendaftarkan Anda, lalu masuk kembali."}
          </AppText>
        </View>

        {email ? (
          <View
            style={[
              styles.kotak,
              { backgroundColor: colors.surface, borderColor: colors.border },
            ]}
          >
            <Ionicons name="mail-outline" size={16} color={colors.textMuted} />
            <AppText variant="caption" color={colors.textSecondary} style={styles.kotakTeks}>
              {email}
            </AppText>
          </View>
        ) : null}

        <View style={styles.langkah}>
          <AppText variant="caption" weight="semibold" color={colors.textSecondary}>
            Yang bisa Anda lakukan
          </AppText>
          {(ditangguhkan
            ? [
                "Hubungi admin FaceLink AI dan sebutkan akun Anda.",
                "Tunggu admin memulihkan status akun Anda.",
                "Masuk kembali dengan akun Google yang sama.",
              ]
            : [
                email
                  ? "Hubungi admin FaceLink AI dan sebutkan email di atas."
                  : "Hubungi admin FaceLink AI dan sebutkan akun Google yang Anda pakai.",
                "Tunggu admin mendaftarkan akun dan menetapkan role Anda.",
                "Masuk kembali dengan akun Google yang sama.",
              ]
          ).map((t, i) => (
            <View key={t} style={styles.langkahItem}>
              <View style={[styles.nomor, { backgroundColor: colors.surfaceMuted }]}>
                <AppText variant="micro" weight="bold">
                  {i + 1}
                </AppText>
              </View>
              <AppText variant="caption" color={colors.textMuted} style={styles.langkahTeks}>
                {t}
              </AppText>
            </View>
          ))}
        </View>
      </View>

      <View style={styles.bawah}>
        <AppButton
          label="Kembali ke layar masuk"
          onPress={() => {
            signOut();
            router.replace("/");
          }}
        />
        <AppText variant="caption" color={colors.textMuted} center>
          Tidak ada data yang disimpan dari percobaan masuk ini.
        </AppText>
      </View>
    </Screen>
  );
}

const styles = StyleSheet.create({
  isi: {
    justifyContent: "space-between",
    gap: spacing.xl,
    paddingTop: spacing.lg,
    paddingBottom: spacing.lg,
  },
  atas: {
    alignItems: "center",
  },
  tengah: {
    alignItems: "center",
    gap: spacing.xl,
  },
  teks: {
    gap: spacing.sm,
  },
  judul: {
    letterSpacing: 0.5,
  },
  kotak: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.sm,
    borderRadius: radius.md,
    borderWidth: StyleSheet.hairlineWidth,
    paddingHorizontal: spacing.lg,
    paddingVertical: spacing.md,
  },
  kotakTeks: {
    flexShrink: 1,
  },
  langkah: {
    alignSelf: "stretch",
    gap: spacing.sm,
  },
  langkahItem: {
    flexDirection: "row",
    alignItems: "flex-start",
    gap: spacing.md,
  },
  nomor: {
    width: 22,
    height: 22,
    borderRadius: radius.pill,
    alignItems: "center",
    justifyContent: "center",
    marginTop: 1,
  },
  langkahTeks: {
    flex: 1,
  },
  bawah: {
    gap: spacing.sm,
  },
});

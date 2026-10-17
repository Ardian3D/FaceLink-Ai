import React, { useEffect, useRef } from "react";
import { useRouter } from "expo-router";
import { StyleSheet, View } from "react-native";

import { AppText } from "@/components/AppText";
import { DemoBanner } from "@/components/DemoBanner";
import { ScanIndicator } from "@/components/ScanIndicator";
import { Screen } from "@/components/Screen";
import { useSession } from "@/features/auth/SessionProvider";
import { DEMO_MODE } from "@/features/auth/demoMode";
import { roleHome } from "@/features/auth/roleRoutes";
import { colors, spacing } from "@/theme";

/**
 * Layar Role Checking (FL-UI01 §8).
 *
 * Layar ini memisahkan "sedang masuk" dari "sudah masuk" sehingga pengguna
 * tidak pernah melihat dashboard sebelum identitas dan role dipastikan.
 *
 * Pada FL-UI01 tidak ada panggilan jaringan: identitas dan role berasal dari
 * sesi demo. Penundaan singkat di bawah ini murni supaya animasi pemindaian
 * terlihat pada preview — ia **bukan** mekanisme autentikasi dan **bukan** bukti
 * verifikasi apa pun. Pada integrasi nanti, blok ini diganti pemeriksaan sesi
 * ke Go Backend dan penundaan buatan itu dihapus.
 */
const DURASI_PREVIEW_MS = 1600;

export default function RoleCheckingScreen() {
  const router = useRouter();
  const { status, session } = useSession();
  const sudahDiarahkan = useRef(false);

  useEffect(() => {
    if (!DEMO_MODE) {
      // Build produksi tidak pernah menampilkan layar demo.
      router.replace("/");
      return;
    }
    if (status !== "checking") return;

    const timer = setTimeout(() => {
      if (sudahDiarahkan.current) return;
      sudahDiarahkan.current = true;
      // Mode preview: pemilik memilih role yang ingin dilihat. Pada integrasi
      // nanti role datang dari respons backend dan layar ini langsung menuju
      // beranda role tersebut.
      if (session) {
        router.replace(roleHome[session.user.role]);
      } else {
        router.replace("/dev-preview");
      }
    }, DURASI_PREVIEW_MS);

    return () => clearTimeout(timer);
  }, [router, session, status]);

  return (
    <Screen>
      <View style={styles.banner}>
        {DEMO_MODE ? <DemoBanner keterangan="role ditentukan data contoh" /> : null}
      </View>
      <View style={styles.tengah}>
        <ScanIndicator />
        <AppText variant="bodyLarge" weight="medium" center>
          Sedang mengecek peran anda…
        </AppText>
        <AppText variant="caption" color={colors.textMuted} center>
          Mohon tunggu sebentar, jangan tutup aplikasi.
        </AppText>
      </View>
    </Screen>
  );
}

const styles = StyleSheet.create({
  banner: {
    paddingTop: spacing.sm,
  },
  tengah: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
    gap: spacing.lg,
  },
});

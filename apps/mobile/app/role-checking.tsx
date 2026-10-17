import React, { useEffect, useRef, useState } from "react";
import { useRouter } from "expo-router";
import { StyleSheet, View } from "react-native";

import { AppText } from "@/components/AppText";
import { BrandLogo } from "@/components/BrandLogo";
import { LoadingHud } from "@/components/LoadingHud";
import { Screen } from "@/components/Screen";
import { StepTracker, type Langkah, type LangkahStatus } from "@/components/StepTracker";
import { useSession } from "@/features/auth/SessionProvider";
import { DEMO_MODE } from "@/features/auth/demoMode";
import { roleHome } from "@/features/auth/roleRoutes";
import { layout, spacing } from "@/theme";

/**
 * Layar pemeriksaan peran (FL-UI01 §8), mengikuti mockup
 * "FaceLink Ai Loading Page".
 *
 * Layar ini memisahkan "sedang masuk" dari "sudah masuk" sehingga pengguna
 * tidak pernah melihat beranda sebelum identitas dan role dipastikan.
 *
 * Urutan langkah di layar mengikuti proses yang sebenarnya, bukan hiasan:
 *
 *   1. **Verifikasi akun** — identitas dari Google sudah diterima.
 *   2. **Mengecek peran** — backend menentukan role akun.
 *   3. **Menyiapkan aplikasi** — beranda role tersebut disiapkan.
 *
 * Pada FL-UI01 tidak ada panggilan jaringan: identitas dan role berasal dari
 * sesi demo, dan durasi tiap langkah hanya untuk preview. Ia **bukan**
 * mekanisme autentikasi dan **bukan** bukti verifikasi apa pun. Pada integrasi
 * nanti, blok ini diganti pemeriksaan sesi ke Go Backend dan penundaan buatan
 * itu dihapus; urutan langkahnya tetap sama.
 */

/** Lama tiap langkah pada preview. Total ~2,4 detik. */
const DURASI_LANGKAH_MS = 800;

const LANGKAH_AWAL: Langkah[] = [
  { label: "Verifikasi akun", ikon: "person-outline", status: "berjalan" },
  { label: "Mengecek peran", ikon: "layers-outline", status: "menunggu" },
  { label: "Menyiapkan aplikasi", ikon: "checkmark", status: "menunggu" },
];

export default function RoleCheckingScreen() {
  const router = useRouter();
  const { status, session } = useSession();
  const sudahDiarahkan = useRef(false);

  const [langkah, setLangkah] = useState<Langkah[]>(LANGKAH_AWAL);

  useEffect(() => {
    if (!DEMO_MODE) {
      // Build produksi tidak pernah menampilkan layar demo.
      router.replace("/");
      return;
    }
    if (status !== "checking" || !session) return;

    const timers = LANGKAH_AWAL.map((_, i) =>
      setTimeout(() => {
        setLangkah((lama) =>
          lama.map((l, j): Langkah => {
            const baru: LangkahStatus =
              j < i + 1 ? "selesai" : j === i + 1 ? "berjalan" : "menunggu";
            return { ...l, status: baru };
          })
        );
      }, DURASI_LANGKAH_MS * (i + 1))
    );

    // Setelah langkah terakhir selesai, arahkan ke beranda role. Mode preview:
    // pemilik memilih role yang ingin dilihat. Pada integrasi nanti role datang
    // dari respons backend dan layar ini langsung menuju beranda role tersebut.
    timers.push(
      setTimeout(
        () => {
          if (sudahDiarahkan.current) return;
          sudahDiarahkan.current = true;
          // Role sudah ditetapkan saat sesi dibuat; layar ini hanya menampilkan
          // prosesnya, lalu menuju beranda role tersebut. Tidak ada layar
          // pemilih role di jalur ini.
          router.replace(roleHome[session.user.role]);
        },
        DURASI_LANGKAH_MS * (LANGKAH_AWAL.length + 1)
      )
    );

    return () => timers.forEach(clearTimeout);
  }, [router, session, status]);

  return (
    <Screen contentStyle={styles.isi}>
      <View style={styles.atas}>
        <BrandLogo size={layout.logoSizeCompact} />
      </View>

      <View style={styles.tengah}>
        <LoadingHud />
        <AppText variant="title" weight="semibold" center>
          Sedang mengecek peran anda…
        </AppText>
      </View>

      <View style={styles.bawah}>
        <StepTracker langkah={langkah} />
      </View>
    </Screen>
  );
}

const styles = StyleSheet.create({
  isi: {
    justifyContent: "space-between",
    paddingTop: spacing.lg,
    paddingBottom: spacing.xxl,
  },
  atas: {
    alignItems: "center",
  },
  tengah: {
    alignItems: "center",
    gap: spacing.xl,
  },
  bawah: {
    alignItems: "center",
  },
});

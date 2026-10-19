import React, { useEffect, useMemo, useRef } from "react";
import { useRouter } from "expo-router";
import { StyleSheet, View, useWindowDimensions } from "react-native";
import { AppText } from "@/components/AppText";
import { BrandLogo } from "@/components/BrandLogo";
import { LoadingHud } from "@/components/LoadingHud";
import { Screen } from "@/components/Screen";
import {
  StepTracker,
  type Langkah,
  type LangkahStatus,
} from "@/components/StepTracker";
import { useSession, type TahapMasuk } from "@/features/auth/SessionProvider";
import { roleHome } from "@/features/auth/roleRoutes";
import { BrandThemeScope, layout, spacing } from "@/theme";

/**
 * Layar pemeriksaan peran (FL-UI02), mengikuti mockup
 * "FaceLink Ai Loading Page".
 *
 * Layar ini memisahkan "sedang masuk" dari "sudah masuk" sehingga pengguna
 * tidak pernah melihat beranda sebelum identitas dan role dipastikan.
 *
 * Urutan langkah **mengikuti proses yang benar-benar berjalan**, bukan hiasan:
 *
 *   1. **Verifikasi akun** — Google ID token diterima dan ditukar ke backend.
 *   2. **Mengecek peran** — backend membaca role dari database (bukan dari token).
 *   3. **Menyiapkan aplikasi** — sesi tersimpan dan beranda role disiapkan.
 *
 * Tidak ada penundaan buatan: status tiap langkah berasal dari `tahap` pada
 * penyedia sesi, dan layar langsung berpindah begitu sesi sah. Bila proses
 * berhenti (ditolak, jaringan mati, dibatalkan), layar ini menyerahkan kembali
 * ke layar masuk atau layar penjelasan — ia tidak pernah berputar selamanya.
 */
const LANGKAH: Langkah[] = [
  { label: "Verifikasi akun", ikon: "person-outline", status: "menunggu" },
  { label: "Mengecek peran", ikon: "layers-outline", status: "menunggu" },
  { label: "Menyiapkan aplikasi", ikon: "checkmark", status: "menunggu" },
];

/**
 * Memetakan tahap autentikasi ke status ketiga langkah.
 *
 * Pemetaan ditulis eksplisit, bukan diturunkan dari urutan indeks, karena satu
 * permintaan backend menutup **dua** langkah sekaligus: `POST /auth/google`
 * memverifikasi ID token **dan** membaca role dari database dalam satu jawaban.
 * Karena itu pada tahap `menukar` langkah 1 dan 2 sama-sama berjalan — bukan
 * langkah 1 sudah selesai sebelum verifikasinya benar-benar terjadi.
 *
 * Pemetaan ini tidak pernah memundurkan langkah: tahap yang lebih jauh selalu
 * menandai langkah sebelumnya selesai.
 */
function langkahDari(tahap: TahapMasuk): Langkah[] {
  const status: Record<TahapMasuk, LangkahStatus[]> = {
    menganggur: ["menunggu", "menunggu", "menunggu"],
    google: ["berjalan", "menunggu", "menunggu"],
    menukar: ["berjalan", "berjalan", "menunggu"],
    menyiapkan: ["selesai", "selesai", "berjalan"],
    selesai: ["selesai", "selesai", "selesai"],
  };
  return LANGKAH.map((l, i) => ({ ...l, status: status[tahap][i] ?? "menunggu" }));
}

export default function RoleCheckingScreen() {
  // Layar pemeriksaan peran selalu gelap: cincin HUD dirancang untuk latar
  // hitam. Isinya dipisah ke komponen anak supaya `useTheme()` di dalamnya
  // membaca palet yang dikunci itu.
  return (
    <BrandThemeScope>
      <PemeriksaanPeran />
    </BrandThemeScope>
  );
}

function PemeriksaanPeran() {
  const router = useRouter();
  const { status, session, tahap } = useSession();
  const { width } = useWindowDimensions();
  // Cincin mengikuti lebar layar: pada HP sempit ia mengecil, pada layar lebar
  // berhenti di ukuran mockup. Tanpa ini, cincin 300 px melebihi lebar konten
  // pada layar 320 px dan terpotong di kedua sisi.
  const lebarHud = Math.min(layout.loadingHudSize, width - layout.screenPadding * 2);
  const sudahDiarahkan = useRef(false);
  // Diturunkan langsung dari tahap, bukan disalin ke state lewat efek: salinan
  // hanya menambah satu render dan dapat tertinggal satu tahap di belakang.
  const langkah = useMemo(() => langkahDari(tahap), [tahap]);

  useEffect(() => {
    // Penolakan akun bukan kegagalan teknis: pengguna dibawa ke penjelasan,
    // bukan dikembalikan ke layar masuk tanpa alasan.
    if (status === "unregistered") {
      router.replace("/not-registered");
      return;
    }
    // Layar ini hanya bermakna selama proses masuk berjalan. Dibuka langsung
    // lewat tautan/riwayat, atau setelah proses berakhir dengan kegagalan, ia
    // tidak boleh menampilkan loading palsu.
    if (status !== "checking" && status !== "signedIn") {
      router.replace("/");
      return;
    }
    if (status === "signedIn" && session !== null && !sudahDiarahkan.current) {
      sudahDiarahkan.current = true;
      router.replace(roleHome[session.user.role]);
    }
  }, [router, session, status]);

  return (
    <Screen contentStyle={styles.isi}>
      <View style={styles.atas}>
        <BrandLogo size={layout.logoSizeCompact} />
      </View>
      <View style={styles.tengah}>
        <LoadingHud size={lebarHud} />
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

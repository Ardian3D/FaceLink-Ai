import React, { useState } from "react";
import { Alert, StyleSheet, View } from "react-native";
import { Ionicons } from "@expo/vector-icons";

import { AppText } from "@/components/AppText";
import { BarChart } from "@/components/dashboard/Charts";
import { HeroCard } from "@/components/dashboard/HeroCard";
import { PillTabs } from "@/components/dashboard/PillTabs";
import { ScheduleRow } from "@/components/dashboard/ScheduleRow";
import { SectionTitle } from "@/components/dashboard/SectionTitle";
import { MiniStat, QuickAction } from "@/components/dashboard/StatCards";
import { DashboardShell } from "@/features/dashboard/DashboardShell";
import { useSession } from "@/features/auth/SessionProvider";
import { useTheme , radius, spacing } from "@/theme";
import { jadwalMahasiswa, riwayatMahasiswa, trenMingguan } from "@/features/dashboard/mockData";

/**
 * Beranda mahasiswa.
 *
 * Hanya memuat data milik sendiri: jadwal yang diikuti, kehadiran pribadi, dan
 * status persetujuan biometrik. Tidak ada daftar peserta lain dan tidak ada
 * data wajah siapa pun — batas itu terlihat dari bentuk layarnya, bukan hanya
 * dari data contohnya.
 */
export default function MahasiswaDashboard() {
  const { colors } = useTheme();
  const { session } = useSession();
  const [filter, setFilter] = useState("hari-ini");

  const jadwal = filter === "riwayat" ? [] : jadwalMahasiswa;
  const hadir = 9;
  const total = 10;

  return (
    <DashboardShell role="mahasiswa" title="Beranda pribadi" subtitle="Data milik Anda sendiri">
      <HeroCard
        label="KEHADIRAN SEMESTER INI"
        value={`${hadir} / ${total}`}
        meta={`Halo, ${session?.user.displayName ?? "Mahasiswa"}`}
        progress={hadir / total}
        progressLabel="Pertemuan dihadiri"
      />

      <View style={styles.dua}>
        <MiniStat
          label="Sesi berikutnya"
          value="10:00"
          hint="IF-202 · Basis Data"
          icon="time-outline"
        />
        <MiniStat
          label="Persetujuan"
          value="Aktif"
          hint="biometrik"
          icon="shield-checkmark-outline"
        />
      </View>

      <View style={styles.seksi}>
        <SectionTitle title="Jadwal saya" action="Hari ini" />
        <PillTabs
          items={[
            { id: "hari-ini", label: "Hari ini" },
            { id: "riwayat", label: "Riwayat" },
          ]}
          activeId={filter}
          onChange={setFilter}
        />
        <View style={styles.daftar}>
          {(filter === "riwayat" ? riwayatMahasiswa : jadwal).map((j) => (
            <ScheduleRow
              key={j.id}
              time={j.jam}
              title={j.judul}
              subtitle={j.keterangan}
              badge={j.status}
              highlight={j.berjalan}
            />
          ))}
        </View>
      </View>

      <View style={styles.seksi}>
        <SectionTitle title="Kehadiran mingguan" action="Saya" />
        <View style={[styles.kartu, { backgroundColor: colors.surface, borderColor: colors.border }]}>
          <BarChart data={trenMingguan} activeId="jum" />
        </View>
      </View>

      <View style={styles.seksi}>
        <SectionTitle title="Aksi cepat" />
        <View style={styles.aksiGrid}>
          <QuickAction
            icon="qr-code-outline"
            label="Tunjukkan identitas"
            onPress={() => Alert.alert("Belum tersedia", "Kartu identitas digital memerlukan integrasi backend.")}
          />
          <QuickAction
            icon="shield-outline"
            label="Persetujuan biometrik"
            onPress={() => Alert.alert("Belum tersedia", "Pengaturan persetujuan memerlukan integrasi backend.")}
          />
        </View>
      </View>

      <View style={[styles.catatan, { borderColor: colors.border, backgroundColor: colors.surface }]}>
        <Ionicons name="lock-closed-outline" size={16} color={colors.textMuted} />
        <AppText variant="caption" color={colors.textMuted} style={styles.catatanTeks}>
          Data wajah dan riwayat Anda tidak pernah ditampilkan ke peserta lain, dan absensi manual
          selalu tersedia bila pemindaian gagal.
        </AppText>
      </View>
    </DashboardShell>
  );
}

const styles = StyleSheet.create({
  dua: {
    flexDirection: "row",
    gap: spacing.md,
  },
  seksi: {
    gap: spacing.md,
  },
  daftar: {
    gap: spacing.sm,
  },
  kartu: {
    borderRadius: radius.xl,
    borderWidth: StyleSheet.hairlineWidth,
    padding: spacing.lg,
  },
  aksiGrid: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: spacing.md,
  },
  catatan: {
    flexDirection: "row",
    gap: spacing.sm,
    borderRadius: radius.lg,
    borderWidth: StyleSheet.hairlineWidth,
    padding: spacing.lg,
  },
  catatanTeks: {
    flex: 1,
  },
});

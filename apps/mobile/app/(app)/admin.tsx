import React, { useState } from "react";
import { Alert, StyleSheet, View } from "react-native";
import { useRouter } from "expo-router";
import { adminEnrollmentStagingAktif } from "@/features/adminEnrollment/staging";

import { AppText } from "@/components/AppText";
import { BarChart, Sparkline } from "@/components/dashboard/Charts";
import { HeroCard } from "@/components/dashboard/HeroCard";
import { PillTabs } from "@/components/dashboard/PillTabs";
import { ScheduleRow } from "@/components/dashboard/ScheduleRow";
import { SectionTitle } from "@/components/dashboard/SectionTitle";
import { MiniStat, QuickAction } from "@/components/dashboard/StatCards";
import { DashboardShell } from "@/features/dashboard/DashboardShell";
import { useTheme , radius, spacing } from "@/theme";
import {
  jadwalAdmin,
  kelasAktifDemo,
  pintasanAdmin,
  perangkatDemo,
  trenMingguan,
} from "@/features/dashboard/mockData";

/**
 * Beranda admin.
 *
 * Isi dipilih menurut pekerjaan admin sehari-hari: seberapa banyak yang hadir,
 * sesi mana yang sedang berjalan, dan terminal mana yang perlu disentuh. Kartu
 * sorotan di puncak menjawab pertanyaan pertama tanpa perlu menggulir.
 *
 * Seluruh angka berasal dari mock development dan ditandai DEMO. Pintasan
 * pengelolaan memberi tahu bahwa layarnya belum ada, bukan berpura-pura membuka
 * halaman kosong.
 */
export default function AdminDashboard() {
  const { colors } = useTheme();
  const router = useRouter();
  const [filter, setFilter] = useState("semua");

  // Angka mengikuti ringkasan yang sama dengan data contoh: 48 peserta
  // terdaftar (bukan jumlah baris kelas), 42 di antaranya sudah tercatat.
  const total = 48;
  const hadir = 42;
  const perangkatOnline = perangkatDemo.filter((d) => d.online).length;
  const sesiBerjalan = jadwalAdmin.find((j) => j.berjalan);

  const jadwal =
    filter === "berjalan"
      ? jadwalAdmin.filter((j) => j.berjalan)
      : filter === "terjadwal"
        ? jadwalAdmin.filter((j) => !j.berjalan)
        : jadwalAdmin;

  const bukaPintasan = (id: string) => {
    const p = pintasanAdmin.find((x) => x.id === id);
    Alert.alert(
      p ? p.title : "Pengelolaan",
      "Layar ini belum dibuat pada FL-UI01. Preview saat ini fokus pada fondasi dan tata visual."
    );
  };

  return (
    <DashboardShell role="admin" title="Ringkasan hari ini" subtitle="Beranda sistem">
      <HeroCard
        label="KEHADIRAN HARI INI"
        value={`${hadir} / ${total}`}
        meta={sesiBerjalan ? `Sesi berjalan · ${sesiBerjalan.judul.split(" · ")[0]}` : "Belum ada sesi"}
        progress={hadir / total}
        progressLabel="Peserta tercatat"
      />

      <View style={styles.dua}>
        <MiniStat
          label="Perangkat"
          value={`${perangkatOnline}/${perangkatDemo.length}`}
          hint="terminal online"
          icon="hardware-chip-outline"
          tone={perangkatOnline === perangkatDemo.length ? "neutral" : "warning"}
        />
        <MiniStat
          label="Perlu tindakan"
          value="1"
          hint="wajah ambigu"
          icon="alert-circle-outline"
          tone="accent"
        />
      </View>

      <View style={styles.seksi}>
        <SectionTitle title="Tren kehadiran" action="7 hari" />
        <View style={[styles.kartu, { backgroundColor: colors.surface, borderColor: colors.border }]}>
          <BarChart data={trenMingguan} activeId="jum" />
        </View>
      </View>

      <View style={styles.seksi}>
        <SectionTitle title="Sesi hari ini" action="Jadwal" />
        <PillTabs
          items={[
            { id: "semua", label: "Semua" },
            { id: "berjalan", label: "Berjalan" },
            { id: "terjadwal", label: "Terjadwal" },
          ]}
          activeId={filter}
          onChange={setFilter}
        />
        <View style={styles.daftar}>
          {jadwal.map((j) => (
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
        <SectionTitle title="Ringkasan kelas" action="Semua kelas" />
        <View style={[styles.kartu, { backgroundColor: colors.surface, borderColor: colors.border }]}>
          <View style={styles.kartuKepala}>
            <AppText variant="caption" color={colors.textMuted}>
              Peserta terdaftar
            </AppText>
            <AppText variant="bodyLarge" weight="bold">
              {total}
            </AppText>
          </View>
          <Sparkline data={[12, 18, 15, 24, 20, 26, 22]} />
          <View style={styles.legenda}>
            {kelasAktifDemo.map((k) => (
              <View key={k.id} style={styles.legendaItem}>
                <View
                  style={[
                    styles.titik,
                    { backgroundColor: k.status === "berjalan" ? colors.accent : colors.chartTrack },
                  ]}
                />
                <AppText variant="micro" color={colors.textMuted} numberOfLines={1}>
                  {k.code} · {k.peserta}
                </AppText>
              </View>
            ))}
          </View>
        </View>
      </View>

      <View style={styles.seksi}>
        <SectionTitle title="Pengelolaan" action="Belum aktif" />
        <View style={styles.aksiGrid}>
          {pintasanAdmin.slice(0, 4).map((p) => (
            <QuickAction
              key={p.id}
              icon={p.icon as never}
              label={p.title}
              onPress={() => bukaPintasan(p.id)}
            />
          ))}
          {adminEnrollmentStagingAktif ? (
            <QuickAction icon="scan-outline" label="Face Enrollment · Pengajuan"
              onPress={() => router.push("/admin-enrollment")} />
          ) : null}
        </View>
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
    gap: spacing.lg,
  },
  kartuKepala: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
  },
  legenda: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: spacing.md,
  },
  legendaItem: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.xs,
  },
  titik: {
    width: 8,
    height: 8,
    borderRadius: 4,
  },
  aksiGrid: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: spacing.md,
  },
});

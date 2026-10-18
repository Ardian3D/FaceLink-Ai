import React, { useState } from "react";
import { Alert, StyleSheet, View } from "react-native";

import { AppText } from "@/components/AppText";
import { BarChart } from "@/components/dashboard/Charts";
import { HeroCard } from "@/components/dashboard/HeroCard";
import { PillTabs } from "@/components/dashboard/PillTabs";
import { ScheduleRow } from "@/components/dashboard/ScheduleRow";
import { SectionTitle } from "@/components/dashboard/SectionTitle";
import { MiniStat, QuickAction } from "@/components/dashboard/StatCards";
import { DashboardShell } from "@/features/dashboard/DashboardShell";
import { useTheme , radius, spacing } from "@/theme";
import { jadwalDosen, perluDiperiksa, trenMingguan } from "@/features/dashboard/mockData";

/**
 * Beranda dosen.
 *
 * Fokusnya satu pertanyaan: sesi mana yang sedang berjalan dan berapa yang
 * sudah tercatat. Kartu sorotan memakai angka sesi berjalan, bukan angka
 * semester, karena itulah yang perlu diputuskan dosen sekarang.
 *
 * Dosen hanya melihat kelas yang ditugaskan kepadanya (docs/AUTHORIZATION.md):
 * tidak ada daftar seluruh kelas di layar ini.
 */
export default function DosenDashboard() {
  const { colors } = useTheme();
  const [filter, setFilter] = useState("hari-ini");

  const sesi = jadwalDosen.find((j) => j.berjalan);
  const hadir = 12;
  const totalPeserta = 24;

  const jadwal = filter === "berjalan" ? jadwalDosen.filter((j) => j.berjalan) : jadwalDosen;

  return (
    <DashboardShell role="dosen" title="Kelas saya" subtitle="Hanya kelas yang ditugaskan">
      <HeroCard
        label="SESI BERJALAN"
        value={`${hadir} / ${totalPeserta}`}
        meta={sesi ? `${sesi.judul} · ${sesi.keterangan}` : "Tidak ada sesi berjalan"}
        progress={hadir / totalPeserta}
        progressLabel="Peserta tercatat"
      />

      <View style={styles.dua}>
        <MiniStat
          label="Kelas hari ini"
          value="2"
          hint="sesuai penugasan"
          icon="school-outline"
        />
        <MiniStat
          label="Perlu diperiksa"
          value="1"
          hint="hasil ambigu"
          icon="alert-circle-outline"
          tone="accent"
        />
      </View>

      <View style={styles.seksi}>
        <SectionTitle title="Jadwal saya" action="Hari ini" />
        <PillTabs
          items={[
            { id: "hari-ini", label: "Hari ini" },
            { id: "berjalan", label: "Berjalan" },
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
        <SectionTitle title="Perlu tindakan" action="Buka sesi" onAction={() => Alert.alert("Belum tersedia", "Pemeriksaan manual memerlukan integrasi backend.")} />
        <View style={styles.daftar}>
          {perluDiperiksa.map((p) => (
            <ScheduleRow key={p.id} time={p.jam} title={p.judul} subtitle={p.keterangan} badge={p.status} highlight />
          ))}
        </View>
        <AppText variant="caption" color={colors.textMuted}>
          Wajah dengan keyakinan rendah tidak pernah dicatat otomatis. Dosen memutuskan
          kehadirannya secara manual.
        </AppText>
      </View>

      <View style={styles.seksi}>
        <SectionTitle title="Kehadiran mingguan" action="Kelas saya" />
        <View style={[styles.kartu, { backgroundColor: colors.surface, borderColor: colors.border }]}>
          <BarChart data={trenMingguan} activeId="jum" />
        </View>
      </View>

      <View style={styles.seksi}>
        <SectionTitle title="Aksi cepat" />
        <View style={styles.aksiGrid}>
          <QuickAction
            icon="play-circle-outline"
            label="Mulai sesi"
            onPress={() => Alert.alert("Belum tersedia", "Menjalankan sesi absensi memerlukan backend.")}
          />
          <QuickAction
            icon="list-outline"
            label="Rekap kelas"
            onPress={() => Alert.alert("Belum tersedia", "Rekap memerlukan data absensi dari backend.")}
          />
          <QuickAction
            icon="checkmark-done-outline"
            label="Absensi manual"
            onPress={() => Alert.alert("Belum tersedia", "Absensi manual memerlukan integrasi backend.")}
          />
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
  },
  aksiGrid: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: spacing.md,
  },
});

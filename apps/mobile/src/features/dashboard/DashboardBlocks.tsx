import React from "react";
import { StyleSheet, View } from "react-native";
import { Ionicons } from "@expo/vector-icons";

import { Aktivitas, KelasAktif, PerangkatStatus, RingkasanMetric } from "@/features/dashboard/mockData";
import { AppText } from "@/components/AppText";
import { Card } from "@/components/Card";
import { ListRow } from "@/components/ListRow";
import { SectionHeader } from "@/components/SectionHeader";
import { StatTile } from "@/components/StatTile";
import { StatusBadge } from "@/components/StatusBadge";
import { colors, spacing } from "@/theme";

/**
 * Blok isi dashboard yang dipakai bersama tiga role.
 *
 * Semuanya menerima data sebagai props — tidak ada pemanggilan backend dan
 * tidak ada state di dalamnya — sehingga penggantian mock dengan data nyata
 * hanya menyentuh layar pemanggilnya.
 */

export function MetricsGrid({ metrics }: { metrics: RingkasanMetric[] }) {
  return (
    <View style={styles.grid}>
      {metrics.map((m) => (
        <StatTile key={m.id} label={m.label} value={m.value} hint={m.hint} />
      ))}
    </View>
  );
}

export function ClassList({ classes }: { classes: KelasAktif[] }) {
  const tone = {
    berjalan: "success",
    terjadwal: "info",
    selesai: "neutral",
  } as const;

  return (
    <View style={styles.stack}>
      {classes.map((k) => (
        <Card key={k.id} style={styles.cardTight}>
          <View style={styles.barisKelas}>
            <View style={styles.infoKelas}>
              <AppText variant="bodyLarge" weight="semibold">
                {k.code}
              </AppText>
              <AppText variant="caption" color={colors.textMuted}>
                {k.title}
              </AppText>
            </View>
            <StatusBadge label={k.status} tone={tone[k.status]} />
          </View>
          <View style={styles.metaKelas}>
            <AppText variant="caption" color={colors.textSecondary}>
              {k.dosen}
            </AppText>
            <AppText variant="caption" color={colors.textMuted}>
              {k.peserta} peserta
            </AppText>
          </View>
        </Card>
      ))}
    </View>
  );
}

export function ActivityList({ items }: { items: Aktivitas[] }) {
  return (
    <Card style={styles.cardTight}>
      {items.map((a, i) => (
        <View key={a.id}>
          {i > 0 ? <View style={styles.pemisah} /> : null}
          <ListRow title={a.title} subtitle={a.time} icon={a.icon} />
        </View>
      ))}
    </Card>
  );
}

export function DeviceList({ devices }: { devices: PerangkatStatus[] }) {
  return (
    <View style={styles.stack}>
      {devices.map((d) => (
        <Card key={d.id} style={styles.cardTight}>
          <View style={styles.barisKelas}>
            <View style={styles.infoKelas}>
              <AppText variant="body" weight="medium">
                {d.name}
              </AppText>
              <AppText variant="caption" color={colors.textMuted}>
                {d.lokasi} · terakhir {d.lastSeen}
              </AppText>
            </View>
            <StatusBadge
              label={d.online ? "Online" : "Offline"}
              tone={d.online ? "success" : "error"}
              icon={d.online ? "wifi" : "cloud-offline"}
            />
          </View>
        </Card>
      ))}
    </View>
  );
}

export function ShortcutGrid({
  items,
  onPress,
}: {
  items: { id: string; title: string; subtitle: string; icon: string }[];
  onPress: (id: string) => void;
}) {
  return (
    <View style={styles.grid}>
      {items.map((p) => (
        <Card key={p.id} style={styles.pintasan}>
          <Ionicons name={p.icon as never} size={20} color={colors.accent} />
          <AppText variant="body" weight="semibold">
            {p.title}
          </AppText>
          <AppText variant="caption" color={colors.textMuted}>
            {p.subtitle}
          </AppText>
          <AppText
            variant="caption"
            weight="medium"
            color={colors.accent}
            onPress={() => onPress(p.id)}
            accessibilityRole="button"
          >
            Buka
          </AppText>
        </Card>
      ))}
    </View>
  );
}

export function DashboardSection({
  title,
  subtitle,
  children,
}: {
  title: string;
  subtitle?: string;
  children: React.ReactNode;
}) {
  return (
    <View style={styles.seksi}>
      <SectionHeader title={title} subtitle={subtitle} />
      {children}
    </View>
  );
}

const styles = StyleSheet.create({
  grid: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: spacing.md,
  },
  stack: {
    gap: spacing.md,
  },
  seksi: {
    gap: spacing.md,
  },
  cardTight: {
    padding: spacing.md,
  },
  barisKelas: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    gap: spacing.md,
  },
  infoKelas: {
    flex: 1,
    gap: 2,
  },
  metaKelas: {
    flexDirection: "row",
    justifyContent: "space-between",
    gap: spacing.md,
  },
  pemisah: {
    height: StyleSheet.hairlineWidth,
    backgroundColor: colors.border,
  },
  pintasan: {
    flexGrow: 1,
    flexBasis: 160,
    minWidth: 150,
    gap: spacing.xs,
  },
});

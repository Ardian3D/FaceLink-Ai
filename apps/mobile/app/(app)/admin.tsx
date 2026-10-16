import React from "react";
import { Alert } from "react-native";

import { DashboardShell } from "@/features/dashboard/DashboardShell";
import {
  ActivityList,
  ClassList,
  DashboardSection,
  DeviceList,
  MetricsGrid,
  ShortcutGrid,
} from "@/features/dashboard/DashboardBlocks";
import {
  aktivitasDemo,
  kelasAktifDemo,
  perangkatDemo,
  pintasanAdmin,
  ringkasanAdmin,
} from "@/features/dashboard/mockData";

/**
 * Beranda admin (FL-UI01 §9).
 *
 * Preview visual: seluruh angka dan daftar berasal dari mock development dan
 * ditandai DEMO pada banner. Layar pengelolaan (Akun & Role, Kelas, Peserta,
 * Perangkat, Sesi, Audit) belum dibuat pada fase ini — pintasannya sengaja
 * memberi tahu bahwa bagian itu belum tersedia, bukan berpura-pura membuka
 * halaman kosong.
 */
export default function AdminDashboard() {
  return (
    <DashboardShell role="admin" title="Beranda sistem" subtitle="Ringkasan hari ini">
      <DashboardSection title="Ringkasan">
        <MetricsGrid metrics={ringkasanAdmin} />
      </DashboardSection>

      <DashboardSection title="Kelas aktif" subtitle="Kelas dan sesi yang berjalan hari ini">
        <ClassList classes={kelasAktifDemo} />
      </DashboardSection>

      <DashboardSection title="Status perangkat" subtitle="Terminal FaceLink">
        <DeviceList devices={perangkatDemo} />
      </DashboardSection>

      <DashboardSection title="Aktivitas terakhir">
        <ActivityList items={aktivitasDemo} />
      </DashboardSection>

      <DashboardSection title="Pengelolaan" subtitle="Belum tersedia pada preview ini">
        <ShortcutGrid
          items={pintasanAdmin}
          onPress={(id) => {
            const p = pintasanAdmin.find((x) => x.id === id);
            Alert.alert(
              p ? p.title : "Pengelolaan",
              "Layar ini belum dibuat pada FL-UI01. Preview saat ini fokus pada fondasi dan tata visual."
            );
          }}
        />
      </DashboardSection>
    </DashboardShell>
  );
}

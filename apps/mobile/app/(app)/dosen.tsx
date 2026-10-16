import React from "react";

import { StateView } from "@/components/StateView";
import {
  ClassList,
  DashboardSection,
  MetricsGrid,
} from "@/features/dashboard/DashboardBlocks";
import { DashboardShell } from "@/features/dashboard/DashboardShell";
import { kelasAktifDemo, ringkasanDosen } from "@/features/dashboard/mockData";

/**
 * Beranda dosen (FL-UI01 §10).
 *
 * Kerangka awal: ringkasan dan daftar kelas yang ditugaskan. Layar Sesi
 * Absensi, Pemeriksaan Manual, dan Rekap belum dibuat pada fase ini; empty state
 * dipakai apa adanya supaya struktur keadaannya sudah terbukti sejak sekarang.
 */
export default function DosenDashboard() {
  return (
    <DashboardShell role="dosen" title="Kelas saya" subtitle="Hanya kelas yang ditugaskan">
      <DashboardSection title="Ringkasan">
        <MetricsGrid metrics={ringkasanDosen} />
      </DashboardSection>

      <DashboardSection title="Kelas yang ditugaskan" subtitle="Kelas di luar penugasan tidak ditampilkan">
        <ClassList classes={kelasAktifDemo.slice(0, 2)} />
      </DashboardSection>

      <DashboardSection title="Sesi absensi" subtitle="Belum tersedia pada preview ini">
        <StateView
          tone="empty"
          title="Belum ada sesi berjalan"
          description="Menjalankan sesi absensi akan tersedia setelah integrasi backend. Layar ini hanya menampilkan kerangka keadaannya."
        />
      </DashboardSection>
    </DashboardShell>
  );
}

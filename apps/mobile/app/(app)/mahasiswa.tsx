import React from "react";

import { StateView } from "@/components/StateView";
import { DashboardSection, MetricsGrid } from "@/features/dashboard/DashboardBlocks";
import { DashboardShell } from "@/features/dashboard/DashboardShell";
import { ringkasanMahasiswa } from "@/features/dashboard/mockData";

/**
 * Beranda mahasiswa (FL-UI01 §10).
 *
 * Kerangka awal: hanya ringkasan milik sendiri. Tidak ada daftar peserta lain
 * dan tidak ada data biometrik siapa pun — batas itu sudah tercermin pada
 * bentuk layarnya, bukan hanya pada data contoh.
 */
export default function MahasiswaDashboard() {
  return (
    <DashboardShell role="mahasiswa" title="Beranda pribadi" subtitle="Data milik Anda sendiri">
      <DashboardSection title="Ringkasan">
        <MetricsGrid metrics={ringkasanMahasiswa} />
      </DashboardSection>

      <DashboardSection title="Jadwal saya" subtitle="Sesi absensi kelas yang diikuti">
        <StateView
          tone="empty"
          title="Belum ada jadwal"
          description="Jadwal sesi absensi akan tampil setelah backend terhubung. Layar ini menampilkan kerangka keadaannya."
        />
      </DashboardSection>

      <DashboardSection title="Kehadiran saya" subtitle="Hanya data Anda sendiri">
        <StateView
          tone="empty"
          title="Belum ada riwayat"
          description="Riwayat kehadiran pribadi akan tampil di sini. Data peserta lain tidak pernah ditampilkan."
        />
      </DashboardSection>
    </DashboardShell>
  );
}

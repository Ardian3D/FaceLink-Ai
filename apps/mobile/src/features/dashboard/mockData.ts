import type { Role } from "@/types";

/**
 * Data contoh dashboard (FL-UI01).
 *
 * SELURUH nilai di berkas ini adalah **mock untuk preview**, bukan hasil
 * pemanggilan backend dan bukan data absensi produksi. Tidak ada nama peserta
 * nyata, tidak ada foto, dan tidak ada embedding. Saat integrasi backend
 * dikerjakan, berkas ini diganti pemanggilan API dan layar tetap sama.
 */

export type RingkasanMetric = {
  id: string;
  label: string;
  value: string;
  hint?: string;
};

export type KelasAktif = {
  id: string;
  code: string;
  title: string;
  dosen: string;
  peserta: number;
  status: "berjalan" | "terjadwal" | "selesai";
};

export type Aktivitas = {
  id: string;
  title: string;
  time: string;
  icon: "checkmark-circle" | "person-add" | "alert-circle" | "log-in" | "cloud-offline";
};

export type PerangkatStatus = {
  id: string;
  name: string;
  lokasi: string;
  online: boolean;
  lastSeen: string;
};

/** Satu baris jadwal/sesi untuk komponen `ScheduleRow`. */
export type JadwalItem = {
  id: string;
  jam: string;
  judul: string;
  keterangan?: string;
  status?: string;
  berjalan?: boolean;
};

/** Satu batang grafik mingguan. */
export type BatangMingguan = { id: string; label: string; value: number };

export const ringkasanAdmin: RingkasanMetric[] = [
  { id: "hadir", label: "Hadir hari ini", value: "42", hint: "dari 48 peserta terjadwal" },
  { id: "mahasiswa", label: "Total mahasiswa", value: "48", hint: "6 kelas demo" },
  { id: "dosen", label: "Total dosen", value: "5", hint: "3 kelas aktif" },
  { id: "perangkat", label: "Perangkat online", value: "1/2", hint: "1 terminal belum terhubung" },
];

export const kelasAktifDemo: KelasAktif[] = [
  {
    id: "demo-class-if101",
    code: "IF-101",
    title: "Pengantar Informatika",
    dosen: "Dosen Demo",
    peserta: 24,
    status: "berjalan",
  },
  {
    id: "demo-class-if202",
    code: "IF-202",
    title: "Basis Data",
    dosen: "Dosen Demo Dua",
    peserta: 18,
    status: "terjadwal",
  },
  {
    id: "demo-class-if303",
    code: "IF-303",
    title: "Jaringan Komputer",
    dosen: "Dosen Demo Tiga",
    peserta: 22,
    status: "selesai",
  },
];

export const aktivitasDemo: Aktivitas[] = [
  { id: "a1", title: "Sesi IF-101 dibuka", time: "08:02", icon: "log-in" },
  { id: "a2", title: "12 kehadiran tercatat", time: "08:15", icon: "checkmark-circle" },
  { id: "a3", title: "1 wajah perlu diperiksa petugas", time: "08:21", icon: "alert-circle" },
  { id: "a4", title: "Akun mahasiswa baru diundang", time: "07:40", icon: "person-add" },
];

export const perangkatDemo: PerangkatStatus[] = [
  { id: "d1", name: "Terminal Kelas IF-101", lokasi: "Lab 1", online: true, lastSeen: "baru saja" },
  { id: "d2", name: "Terminal Cadangan", lokasi: "Lab 2", online: false, lastSeen: "2 jam lalu" },
];

export const ringkasanDosen: RingkasanMetric[] = [
  { id: "kelas", label: "Kelas ditugaskan", value: "2", hint: "hari ini" },
  { id: "hadir", label: "Hadir sesi berjalan", value: "12", hint: "dari 24 peserta" },
  { id: "perlu", label: "Perlu tindakan", value: "1", hint: "hasil ambigu" },
];

export const ringkasanMahasiswa: RingkasanMetric[] = [
  { id: "hadir", label: "Kehadiran saya", value: "9/10", hint: "semester ini" },
  { id: "berikut", label: "Sesi berikutnya", value: "10:00", hint: "IF-101" },
  { id: "konsen", label: "Persetujuan", value: "Aktif", hint: "biometrik" },
];

/** Kehadiran tujuh hari terakhir — admin dan dosen melihat tren ini. */
export const trenMingguan: BatangMingguan[] = [
  { id: "sen", label: "Sen", value: 38 },
  { id: "sel", label: "Sel", value: 42 },
  { id: "rab", label: "Rab", value: 35 },
  { id: "kam", label: "Kam", value: 44 },
  { id: "jum", label: "Jum", value: 40 },
  { id: "sab", label: "Sab", value: 22 },
  { id: "min", label: "Min", value: 0 },
];

/** Jadwal sesi hari ini untuk admin. */
export const jadwalAdmin: JadwalItem[] = [
  {
    id: "j1",
    jam: "08:00",
    judul: "IF-101 · Pengantar Informatika",
    keterangan: "Dosen Demo · Lab 1",
    status: "Berjalan",
    berjalan: true,
  },
  {
    id: "j2",
    jam: "10:00",
    judul: "IF-202 · Basis Data",
    keterangan: "Dosen Demo Dua · Lab 2",
    status: "Terjadwal",
  },
  {
    id: "j3",
    jam: "13:00",
    judul: "IF-303 · Jaringan Komputer",
    keterangan: "Dosen Demo Tiga · Lab 1",
    status: "Terjadwal",
  },
];

/** Sesi milik dosen hari ini. */
export const jadwalDosen: JadwalItem[] = [
  {
    id: "jd1",
    jam: "08:00",
    judul: "IF-101 · Pengantar Informatika",
    keterangan: "24 peserta · Lab 1",
    status: "Berjalan",
    berjalan: true,
  },
  {
    id: "jd2",
    jam: "10:00",
    judul: "IF-202 · Basis Data",
    keterangan: "18 peserta · Lab 2",
    status: "Terjadwal",
  },
];

/** Jadwal kuliah mahasiswa hari ini. */
export const jadwalMahasiswa: JadwalItem[] = [
  {
    id: "jm1",
    jam: "08:00",
    judul: "IF-101 · Pengantar Informatika",
    keterangan: "Dosen Demo · Lab 1",
    status: "Hadir",
    berjalan: true,
  },
  {
    id: "jm2",
    jam: "10:00",
    judul: "IF-202 · Basis Data",
    keterangan: "Dosen Demo Dua · Lab 2",
    status: "Terjadwal",
  },
];

/** Riwayat kehadiran pribadi mahasiswa. */
export const riwayatMahasiswa: JadwalItem[] = [
  { id: "r1", jam: "12 Okt", judul: "IF-101 · Pengantar Informatika", keterangan: "08:02 · tepat waktu", status: "Hadir" },
  { id: "r2", jam: "10 Okt", judul: "IF-202 · Basis Data", keterangan: "10:05 · tepat waktu", status: "Hadir" },
  { id: "r3", jam: "05 Okt", judul: "IF-101 · Pengantar Informatika", keterangan: "Tidak ada rekaman", status: "Tidak hadir" },
];

/** Peserta yang perlu diperiksa dosen setelah pencocokan ambigu. */
export const perluDiperiksa: JadwalItem[] = [
  {
    id: "p1",
    jam: "08:21",
    judul: "Satu wajah tidak dapat dipastikan",
    keterangan: "Keyakinan rendah · perlu keputusan manual",
    status: "Periksa",
  },
];

/** Pintasan pengelolaan untuk admin. Belum ada layarnya pada FL-UI01. */
export const pintasanAdmin: { id: string; title: string; subtitle: string; icon: string }[] = [
  { id: "akun", title: "Akun & Role", subtitle: "Undang akun, ubah role dan status", icon: "people-outline" },
  { id: "kelas", title: "Kelas & Penugasan", subtitle: "Kelola kelas dan dosen pengampu", icon: "school-outline" },
  { id: "peserta", title: "Peserta & Persetujuan", subtitle: "Data peserta dan konsen biometrik", icon: "id-card-outline" },
  { id: "perangkat", title: "Perangkat", subtitle: "Terminal FaceLink dan token", icon: "hardware-chip-outline" },
  { id: "sesi", title: "Sesi & Rekap", subtitle: "Pantau sesi absensi dan ekspor", icon: "list-outline" },
  { id: "audit", title: "Audit", subtitle: "Telusuri perubahan dan insiden", icon: "shield-checkmark-outline" },
];

/** Label role untuk judul beranda. */
export const judulBeranda: Record<Role, string> = {
  admin: "Beranda sistem",
  dosen: "Kelas saya",
  mahasiswa: "Beranda pribadi",
};

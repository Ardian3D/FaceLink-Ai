import type { Role } from "@/types";

/**
 * Rute beranda per role.
 *
 * Navigasi ditentukan backend (`role` dari respons `POST /auth/google` atau
 * `GET /me`), bukan dari pilihan pengguna. Satu akun hanya memiliki satu role
 * utama (D-037), sehingga aplikasi tidak pernah berpindah role di dalam sesi
 * yang sama.
 */
export const roleHome = {
  admin: "/admin",
  dosen: "/dosen",
  mahasiswa: "/mahasiswa",
} as const satisfies Record<Role, string>;

export type RoleHome = (typeof roleHome)[Role];

/**
 * Rute yang harus dibuka untuk sebuah layar role.
 *
 * Mengembalikan `null` bila layar itu memang milik role sesi. Selain itu
 * mengembalikan beranda role yang benar, sehingga layar `/admin` yang dibuka
 * dengan sesi mahasiswa langsung berpindah alih-alih menampilkan data admin.
 *
 * Dipisah dari komponennya supaya aturan ini dapat diuji tanpa rendering React
 * — aturan otorisasi navigasi tidak boleh hanya terbukti lewat mata.
 */
export function ruteAlihRole(sesi: Role | null, layar: Role): RoleHome | null {
  // Tanpa sesi, penjaga di `app/(app)/_layout.tsx` yang mengarahkan ke layar
  // masuk; fungsi ini tidak boleh menebak role.
  if (sesi === null) return null;
  if (sesi === layar) return null;
  return roleHome[sesi];
}

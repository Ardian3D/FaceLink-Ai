import type { Role } from "@/types";

/**
 * Rute beranda per role.
 *
 * Navigasi ditentukan backend pada integrasi nanti (`role` dari respons
 * `POST /auth/google`), bukan dari pilihan pengguna. Satu akun hanya memiliki
 * satu role utama (D-037), sehingga aplikasi tidak pernah berpindah role di
 * dalam sesi yang sama.
 */
export const roleHome = {
  admin: "/admin",
  dosen: "/dosen",
  mahasiswa: "/mahasiswa",
} as const satisfies Record<Role, string>;

export type RoleHome = (typeof roleHome)[Role];

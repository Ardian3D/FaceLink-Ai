/**
 * Aturan navigasi antar-role.
 *
 * Satu sesi hanya boleh melihat beranda rolenya sendiri (D-037). Kalau aturan
 * ini salah, data admin dapat tampil di layar mahasiswa — kesalahan yang tidak
 * akan tertangkap oleh typecheck karena semuanya bertipe `Role`.
 */
import { describe, expect, it } from "vitest";
import { roleHome, ruteAlihRole } from "@/features/auth/roleRoutes";

describe("ruteAlihRole", () => {
  it("tidak mengalihkan saat role sesi cocok dengan layar", () => {
    expect(ruteAlihRole("admin", "admin")).toBeNull();
    expect(ruteAlihRole("dosen", "dosen")).toBeNull();
    expect(ruteAlihRole("mahasiswa", "mahasiswa")).toBeNull();
  });

  it("mengalihkan setiap kombinasi lintas-role ke beranda pemilik sesi", () => {
    const roles = ["admin", "dosen", "mahasiswa"] as const;
    for (const sesi of roles) {
      for (const layar of roles) {
        if (sesi === layar) continue;
        expect(ruteAlihRole(sesi, layar)).toBe(roleHome[sesi]);
      }
    }
  });

  it("tidak menebak role saat belum ada sesi", () => {
    // Penjaga `(app)/_layout.tsx` yang mengarahkan ke layar masuk; di sini
    // tidak boleh ada tebakan role yang dapat membuka dashboard.
    expect(ruteAlihRole(null, "admin")).toBeNull();
  });
});

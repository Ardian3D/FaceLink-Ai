import type { CurrentUser, Role } from "@/types";

/**
 * Data mock akun untuk preview (FL-UI01).
 *
 * Seluruh nilai di berkas ini adalah **contoh demo**, bukan data peserta nyata
 * dan bukan data produksi. Tidak ada satu pun nilai yang boleh dipakai sebagai
 * identitas sungguhan.
 *
 * Dipetakan per role supaya call site cukup menulis `akunDemo[role]`.
 */
export const akunDemo: Record<Role, CurrentUser> = {
  admin: {
    id: "demo-user-admin",
    email: "admin.demo@example.test",
    displayName: "Admin Demo",
    role: "admin",
    status: "active",
    personId: null,
    classIds: [],
  },
  dosen: {
    id: "demo-user-dosen",
    email: "dosen.demo@example.test",
    displayName: "Dosen Demo",
    role: "dosen",
    status: "active",
    personId: null,
    classIds: ["demo-class-if101", "demo-class-if202"],
  },
  mahasiswa: {
    id: "demo-user-mahasiswa",
    email: "mahasiswa.demo@example.test",
    displayName: "Mahasiswa Demo",
    role: "mahasiswa",
    status: "active",
    personId: "demo-person-001",
    classIds: ["demo-class-if101"],
  },
};

/**
 * Role yang dipakai jalur demo saat "masuk" tanpa backend.
 *
 * Pada integrasi nanti nilai ini **tidak dipakai**: role datang dari respons
 * `POST /auth/google` dan hanya backend yang menetapkannya
 * (docs/AUTHORIZATION.md §5). Di sini ia hanya menggantikan jawaban backend
 * supaya alur masuk → loading → beranda role dapat dipreview apa adanya,
 * tanpa layar pemilih role di tengah jalan.
 *
 * Mengubah nilai ini mengubah beranda mana yang muncul setelah loading.
 * Pemilik project tetap dapat berpindah role lewat tombol "Ganti role (dev)"
 * di beranda.
 */
export const ROLE_DEMO: Role = "admin";

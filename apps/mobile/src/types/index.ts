/**
 * Tipe bersama FaceLink AI.
 *
 * Role dan status akun di sini adalah cerminan kontrak backend (lihat
 * `docs/API_CONTRACT.md` §5.1 dan `docs/AUTHORIZATION.md` §4). Frontend tidak
 * pernah menetapkan role sendiri: pada integrasi nanti nilai `role` berasal dari
 * respons `POST /auth/google` yang sudah diverifikasi Go Backend.
 */

/** Tiga role manusia yang memakai aplikasi ini. `device` bukan role manusia. */
export type Role = "admin" | "dosen" | "mahasiswa";

/** Status akun pada tabel `users`. */
export type AccountStatus = "invited" | "active" | "suspended";

export const ROLES: readonly Role[] = ["admin", "dosen", "mahasiswa"];

/** Label bahasa Indonesia untuk ditampilkan ke pengguna. */
export const roleLabel: Record<Role, string> = {
  admin: "Admin",
  dosen: "Dosen",
  mahasiswa: "Mahasiswa",
};

/**
 * Identitas pemanggil sebagaimana diterima dari `GET /me`
 * (`docs/API_CONTRACT.md` §5.1). Pada FL-UI01 nilainya berasal dari mock
 * development, bukan dari backend.
 */
export type CurrentUser = {
  id: string;
  email: string;
  displayName: string;
  role: Role;
  status: AccountStatus;
  /** `null` berarti akun belum tertaut baris peserta. */
  personId: string | null;
  classIds: string[];
};

/**
 * Sesi aplikasi. Pada FL-UI01 token tidak pernah diterbitkan maupun disimpan:
 * antarmuka ini disiapkan supaya integrasi backend nanti hanya mengganti
 * penyedianya, bukan seluruh layar.
 */
export type Session = {
  user: CurrentUser;
  /** Menandai bahwa sesi ini berasal dari mock development, bukan backend. */
  demo: true;
};

/** Status akun di lapisan UI saat menunggu backend. */
export type SessionError =
  /**
   * Identitas Google sah, tetapi akun itu **belum terdaftar** di FaceLink AI.
   * Backend menjawab `403 IDENTITY_NOT_LINKED` (`docs/AUTHORIZATION.md` §6.1):
   * satu pesan generik untuk semua sebab, dengan alasan rinci hanya di audit
   * (`auth.login.denied` + `reason` ber-kode `no_account`/`not_linked`).
   * Aplikasi tidak pernah menerima alasan itu, jadi layar tidak membedakan
   * "belum pernah diundang" dari "belum ditautkan".
   *
   * Role hanya ditetapkan admin (D-037), sehingga pengguna seperti ini harus
   * menghubungi admin lebih dahulu.
   */
  | { kind: "notGranted"; email?: string }
  /**
   * Akun terdaftar tetapi sedang ditangguhkan (`users.status = suspended`).
   * Pada kontrak, keadaan ini juga tampil sebagai `403 IDENTITY_NOT_LINKED`;
   * pemisahan di UI hanya bertujuan agar pesan dan langkahnya tepat, bukan
   * untuk membocorkan status akun ke pihak yang belum terbukti memilikinya.
   */
  | { kind: "suspended"; email?: string }
  | { kind: "network" }
  | { kind: "tls" }
  | { kind: "unknown"; message: string };

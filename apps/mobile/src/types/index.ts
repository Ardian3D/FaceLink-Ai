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
 * Identitas pemanggil sebagaimana diterima dari `POST /auth/google` dan
 * `GET /me` (`docs/API_CONTRACT.md` §5.1). Seluruh nilai berasal dari
 * backend; aplikasi tidak pernah menetapkan role sendiri.
 */
export type CurrentUser = {
  id: string;
  /** Kosong bila backend tidak mengirimkannya (`omitempty` pada kontrak). */
  email: string;
  displayName: string;
  role: Role;
  status: AccountStatus;
  /** `null` berarti akun belum tertaut baris peserta. */
  personId: string | null;
  classIds: string[];
};

/**
 * Sesi aplikasi di lapisan UI.
 *
 * Token **tidak** ada di sini: ia hidup di `tokenStore` (penyimpanan
 * kredensial platform) dan hanya dibaca oleh lapisan transport. Dengan begitu
 * tidak ada layar yang dapat tanpa sengaja menampilkan atau mencatat token.
 */
export type Session = {
  user: CurrentUser;
};

/**
 * Sebab kegagalan yang dapat dibedakan pengguna.
 *
 * Setiap keadaan di bawah ini benar-benar dihasilkan kode; tidak ada cabang
 * yang hanya ada untuk preview. `notGranted` adalah satu-satunya bentuk
 * penolakan akun, sesuai kontrak: backend memakai jawaban generik yang sama
 * untuk "belum diundang", "belum ditautkan", dan "ditangguhkan" agar
 * tidak membocorkan keberadaan akun. Aplikasi karena itu **tidak** membedakan
 * akun ditangguhkan dari akun yang belum terdaftar.
 */
export type SessionError =
  /**
   * Identitas Google sah, tetapi akun itu belum diberi akses ke FaceLink AI
   * (`403 IDENTITY_NOT_LINKED`). Role hanya ditetapkan admin (D-037), jadi
   * pengguna seperti ini harus menghubungi admin lebih dahulu.
   */
  | { kind: "notGranted"; email?: string }
  /** Backend tidak menjawab: alamat salah, backend mati, atau `adb reverse` belum dipasang. */
  | { kind: "network" }
  /** Sertifikat TLS backend tidak tepercaya pada perangkat ini. */
  | { kind: "tls" }
  /** Konfigurasi alamat backend tidak sah; bukan masalah jaringan. */
  | { kind: "config"; message: string }
  /**
   * Penyimpanan kredensial perangkat gagal (mis. Keystore terkunci) atau tidak
   * tersedia pada platform ini (pratinjau web). Sesi tidak boleh dianggap sah
   * bila tokennya tidak dapat disimpan: ia akan hilang tanpa penjelasan.
   */
  | { kind: "storage"; message: string }
  /** Backend menjawab `503`: layanan autentikasi atau Google belum dikonfigurasi di server. */
  | { kind: "unavailable"; message: string }
  /** Bentuk respons atau kode galat yang belum dikenal aplikasi ini. */
  | { kind: "unknown"; message: string }
  /** Sesi tidak lagi berlaku di server (dicabut, akun ditangguhkan, atau token tidak dikenal). */
  | { kind: "sessionEnded" }
  /**
   * Google ID token ditolak backend (): tanda tangan,
   * audience, waktu, atau nonce tidak cocok. Berbeda dari `sessionEnded`: tidak
   * ada sesi yang berakhir, dan percobaan ulang dengan nonce baru dapat berhasil
   * (mis. jam perangkat yang menyimpang).
   */
  | { kind: "identityRejected" }
  /** Pengguna menutup lembar akun Google. Bukan kegagalan; layar kembali tenang. */
  | { kind: "cancelled" }
  /** Google Play Services tidak tersedia atau perlu diperbarui. */
  | { kind: "playServices" }
  /** Platform ini tidak mendukung Credential Manager (mis. pratinjau web). */
  | { kind: "unsupported" };

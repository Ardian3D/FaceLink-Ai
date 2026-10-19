/**
 * Penjembatanan Google Sign-In di perangkat (Android Credential Manager).
 *
 * **Mengapa pustaka ini:** backend menuntut nonce yang dikirim **ke Google** dan
 * kembali di klaim `nonce` ID token (`docs/API_CONTRACT.md` §5.1). Credential
 * Manager menyediakan `GetGoogleIdOption.setNonce` untuk itu, dan
 * `react-native-nitro-google-signin` (MIT) meneruskan nilai tersebut apa adanya.
 * Jalur lama `@react-native-google-signin/google-signin` hanya menyediakan nonce
 * kustom pada edisi berbayar, sehingga tidak dipakai — aturan project melarang
 * ketergantungan berbayar.
 *
 * **Nilai nonce yang dikirim harus nilai yang diterbitkan backend**, bukan
 * hash-nya: backend membandingkan klaim `nonce` dengan nilai yang ia simpan
 * (sebagai hash) memakai perbandingan waktu tetap, jadi hash ganda akan gagal.
 *
 * **Yang tidak dilakukan modul ini:** memverifikasi ID token (backend yang
 * memverifikasi), menyimpan token, dan menentukan role. Modul ini hanya
 * mengembalikan ID token beserta email yang ditampilkan Google.
 *
 * **Batas platform:** modul native ini tidak tersedia di Expo Go maupun web.
 * Karena itu `require` dilakukan malas (lazy) di dalam fungsi, bukan pada
 * tingkat modul: bundel web tetap dapat memuat layar masuk dan menampilkan
 * alasan yang benar, bukan gagal saat impor.
 *
 * **Dapat diuji tanpa perangkat:** seluruh keputusan (urutan panggilan,
 * penerjemahan galat, penerusan nonce) berada di fungsi murni yang menerima
 * modul dan konfigurasi sebagai parameter; hanya `masukDenganGoogle()` yang
 * menyentuh `require`. Lihat `tests/googleSignIn.test.ts`.
 */
import { Platform } from "react-native";
import type { SessionError } from "@/types";
import { konfigurasiGoogle, type HasilKonfigurasiGoogle } from "./googleConfig";

/** Hasil satu percobaan masuk lewat Google. */
export type HasilGoogle =
  | { ok: true; idToken: string; email?: string }
  | { ok: false; error: SessionError };

/** Satu jawaban Credential Manager. */
type JawabanGoogle = {
  type: "success" | "noSavedCredentialFound" | "cancelled";
  data: { idToken: string; user: { email: string | null } } | null;
};

/**
 * Sebagian permukaan pustaka yang dipakai. Ditulis sendiri, bukan diimpor dari
 * paketnya, supaya modul ini tidak menarik kode native pada bundel web.
 *
 * Diekspor agar keputusan yang bergantung padanya dapat diuji dengan modul
 * tiruan — tanpa perlu menambal `require` pada lingkungan pengujian ESM.
 */
export type ModulGoogle = {
  GoogleOneTapSignIn: {
    configure(params: {
      webClientId: string;
      nonce: string;
      offlineAccess?: boolean;
      autoSelectOnSignIn?: boolean;
    }): void;
    checkPlayServices(): Promise<void>;
    signIn(): Promise<JawabanGoogle>;
    createAccount(): Promise<JawabanGoogle>;
    signOut(): Promise<void>;
  };
  isErrorWithCode(error: unknown): error is { code: string; message: string };
  statusCodes: Record<string, string>;
};

function muatModul(): ModulGoogle | null {
  // `Platform.OS === "web"` tidak pernah punya modul native ini. Memanggil
  // `require` di sini membuat Metro tetap membundel paketnya (impor statis akan
  // sama), tetapi eksekusi `createHybridObject` hanya terjadi pada perangkat.
  if (Platform.OS === "web") return null;
  try {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    return require("react-native-nitro-google-signin") as ModulGoogle;
  } catch {
    // Modul native tidak tertaut (mis. Expo Go). Dilaporkan sebagai
    // "tidak didukung", bukan sebagai kegagalan jaringan.
    return null;
  }
}

/** Menerjemahkan galat pustaka menjadi keadaan UI yang dapat ditampilkan. */
function galatPustaka(modul: ModulGoogle, galat: unknown): SessionError {
  if (modul.isErrorWithCode(galat)) {
    if (galat.code === modul.statusCodes.PLAY_SERVICES_NOT_AVAILABLE) {
      return { kind: "playServices" };
    }
    if (galat.code === modul.statusCodes.SIGN_IN_CANCELLED) {
      return { kind: "cancelled" };
    }
    return { kind: "unknown", message: galat.code };
  }
  return { kind: "unknown", message: galat instanceof Error ? galat.message : String(galat) };
}

/**
 * Alasan platform/konfigurasi tidak memungkinkan masuk Google, atau `null`.
 *
 * Murni: seluruh masukan diberikan lewat parameter, sehingga setiap kombinasi
 * dapat diuji tanpa menjalankan aplikasi di perangkat.
 */
export function hambatanMasuk(
  platform: string,
  konfigurasi: HasilKonfigurasiGoogle,
  modulTersedia: boolean
): SessionError | null {
  if (platform === "web") return { kind: "unsupported" };
  if (!konfigurasi.ok) return { kind: "config", message: konfigurasi.alasan };
  if (!modulTersedia) return { kind: "unsupported" };
  return null;
}

/**
 * Alasan masuk Google tidak dapat dijalankan pada build ini, atau `null` bila
 * dapat. Dipanggil **sebelum** meminta nonce: platform tanpa Credential Manager
 * tidak akan pernah menyelesaikan alur, dan meminta nonce lebih dahulu hanya
 * membuat backend menerbitkan nonce yang tidak terpakai sambil menampilkan galat
 * jaringan yang menyesatkan.
 */
export function periksaDukungan(): SessionError | null {
  return hambatanMasuk(Platform.OS, konfigurasiGoogle, muatModul() !== null);
}

/**
 * Menjalankan satu percobaan masuk memakai modul yang sudah dimuat.
 *
 * Murni terhadap `require`: modul dan konfigurasi diberikan pemanggil, sehingga
 * urutan panggilan dan penerjemahan galat dapat diuji di Node.
 *
 * `nonce` **wajib** berasal dari `POST /auth/nonce`. Satu nonce hanya untuk satu
 * percobaan: bila pengguna membatalkan lembar akun lalu mencoba lagi, pemanggil
 * harus meminta nonce baru, karena Credential Manager dapat memakai ulang
 * permintaan sebelumnya.
 *
 * Urutan percobaan mengikuti anjuran pustaka: `checkPlayServices` → `signIn`
 * (akun yang sudah pernah memberi izin) → `createAccount` (semua akun di
 * perangkat). `presentExplicitSignIn` tidak dipakai karena aplikasi memakai
 * tombol sendiri, bukan tombol bawaan Google.
 */
export async function jalankanMasukGoogle(
  modul: ModulGoogle,
  konfigurasi: HasilKonfigurasiGoogle,
  nonce: string
): Promise<HasilGoogle> {
  if (!konfigurasi.ok) {
    return { ok: false, error: { kind: "config", message: konfigurasi.alasan } };
  }
  const google = modul.GoogleOneTapSignIn;
  try {
    google.configure({
      webClientId: konfigurasi.webClientId,
      // Nilai mentah dari backend. Jangan di-hash di sini: backend
      // membandingkan klaim `nonce` dengan nilai yang ia terbitkan.
      nonce,
      // Backend tidak memakai refresh token Google; sesi FaceLink AI berdiri
      // sendiri (D-033), jadi tidak ada server auth code yang diminta.
      offlineAccess: false,
      autoSelectOnSignIn: false,
    });
    await google.checkPlayServices();
    let jawaban = await google.signIn();
    if (jawaban.type === "noSavedCredentialFound") {
      jawaban = await google.createAccount();
    }
    if (jawaban.type === "cancelled") {
      return { ok: false, error: { kind: "cancelled" } };
    }
    if (jawaban.type !== "success" || jawaban.data === null) {
      return { ok: false, error: { kind: "unknown", message: "google_tanpa_id_token" } };
    }
    const { idToken, user } = jawaban.data;
    if (typeof idToken !== "string" || idToken === "") {
      return { ok: false, error: { kind: "unknown", message: "google_tanpa_id_token" } };
    }
    return user.email === null || user.email === undefined
      ? { ok: true, idToken }
      : { ok: true, idToken, email: user.email };
  } catch (galat) {
    return { ok: false, error: galatPustaka(modul, galat) };
  }
}

/** Menjalankan alur Google di perangkat dan mengembalikan ID token. */
export async function masukDenganGoogle(nonce: string): Promise<HasilGoogle> {
  const penghalang = periksaDukungan();
  if (penghalang !== null) return { ok: false, error: penghalang };
  const modul = muatModul();
  if (modul === null) return { ok: false, error: { kind: "unsupported" } };
  return await jalankanMasukGoogle(modul, konfigurasiGoogle, nonce);
}

/**
 * Membersihkan keadaan Google di perangkat.
 *
 * Dipanggil saat keluar agar lembar akun berikutnya menampilkan pilihan akun
 * lagi, bukan langsung memilih akun yang sama. Kegagalan diabaikan: keadaan
 * Google lokal bukan sesi FaceLink AI, dan keluarnya aplikasi tidak boleh
 * bergantung padanya.
 */
export async function lupakanGoogle(): Promise<void> {
  const modul = muatModul();
  if (modul === null) return;
  try {
    await modul.GoogleOneTapSignIn.signOut();
  } catch {
    // Sengaja diabaikan: lihat catatan fungsi.
  }
}

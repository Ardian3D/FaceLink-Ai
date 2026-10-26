/**
 * Inti alur autentikasi FaceLink AI — **tanpa React, tanpa modul native**.
 *
 * Modul ini memegang seluruh keputusan yang sensitif (urutan nonce → Google →
 * tukar token, penyegaran token tunggal, pemulihan sesi, keluar) dan menerima
 * semua ketergantungannya lewat parameter. `SessionProvider` hanya menjadi
 * pembungkus tipis yang meneruskan hasilnya ke React.
 *
 * Pemisahan ini disengaja: alur yang sama harus dapat diuji tanpa perangkat
 * Android, tanpa JVM, dan tanpa jaringan. Yang diuji adalah perilakunya — bukan
 * tiruan yang selalu berhasil.
 *
 * **Aturan yang ditegakkan di sini:**
 *
 *  1. Sesi hanya terbit setelah backend menukar ID token (`POST /auth/google`).
 *     Role tidak pernah ditetapkan di sini; ia hanya disalin dari jawaban.
 *  2. Token hanya ditulis lewat `PenyimpananSesi` (SecureStore pada produksi).
 *     ID token Google tidak pernah disimpan.
 *  3. Penyegaran token bersifat **satu penerbangan**: pemanggil yang datang
 *     saat penyegaran berjalan menunggu hasil yang sama. Ini bukan optimasi,
 *     melainkan syarat keamanan — backend mencabut *seluruh* sesi pengguna bila
 *     token penyegar yang sudah dirotasi dipakai ulang.
 *  4. Kegagalan jaringan **tidak** menghapus token. Hanya penolakan tegas dari
 *     backend (sesi dicabut, akun ditolak) yang membuang sesi lokal.
 *  5. Akun tanpa akses (`notGranted`) tidak pernah menghasilkan sesi, dan sisa
 *     tokennya dibuang.
 */
import type { CurrentUser, SessionError } from "@/types";
import type { HasilAuth, SesiBackend } from "./authApi";
import type { HasilGoogle } from "./googleSignIn";

export type AuthStatus =
  | "memulihkan"
  | "signedOut"
  | "checking"
  | "signedIn"
  | "unregistered"
  | "failed";

/** Tahap proses masuk, dipakai layar pemeriksaan peran untuk kemajuan nyata. */
export type TahapMasuk = "menganggur" | "google" | "menukar" | "menyiapkan" | "selesai";

export type AuthState = {
  status: AuthStatus;
  session: { user: CurrentUser } | null;
  error: SessionError | null;
  tahap: TahapMasuk;
};

/** Bentuk sesi yang disimpan di perangkat. Sama dengan `SesiTersimpan`. */
export type SesiPerangkat = {
  accessToken: string;
  refreshToken: string;
  expiresAtMs: number;
};

/** Penyimpanan kredensial. Implementasi produksi: `expo-secure-store`. */
export type PenyimpananSesi = {
  muat: () => Promise<SesiPerangkat | null>;
  simpan: (sesi: SesiPerangkat) => Promise<void>;
  hapus: () => Promise<void>;
  /** `true` bila token akses masih berlaku minimal selama margin. */
  masihBerlaku: (sesi: SesiPerangkat) => boolean;
};

/** Lima panggilan backend yang membentuk sesi. */
export type ApiAuth = {
  mintaNonce: () => Promise<HasilAuth<string>>;
  tukarIdToken: (
    idToken: string,
    nonce: string,
    emailGoogle?: string
  ) => Promise<HasilAuth<SesiBackend>>;
  segarkanSesi: (
    refreshToken: string
  ) => Promise<HasilAuth<{ accessToken: string; refreshToken: string; expiresAtMs: number }>>;
  ambilSaya: (accessToken: string) => Promise<HasilAuth<CurrentUser>>;
  keluarSesi: (accessToken: string) => Promise<void>;
};

/** Google Sign-In di perangkat. */
export type GoogleAuth = {
  periksaDukungan: () => SessionError | null;
  masukDenganGoogle: (nonce: string) => Promise<HasilGoogle>;
  lupakanGoogle: () => Promise<void>;
};

const AWAL: AuthState = {
  status: "memulihkan",
  session: null,
  error: null,
  tahap: "menganggur",
};

/**
 * Kegagalan yang **tidak** membuktikan sesi tidak sah.
 *
 * Backend yang tidak terjangkau atau menjawab 503 tidak boleh membuat pengguna
 * kehilangan sesi yang masih berlaku: memaksa login ulang hanya karena Wi-Fi
 * putus adalah cara termudah membuat orang mematikan pengaman.
 */
function sementara(error: SessionError): boolean {
  return (
    error.kind === "network" ||
    error.kind === "tls" ||
    error.kind === "unavailable" ||
    error.kind === "config"
  );
}

export class AuthFlow {
  private state: AuthState = AWAL;
  private sesi: SesiPerangkat | null = null;
  /** Penyegaran yang sedang berjalan, dibagi ke semua pemanggil. */
  private segarkanSedang: Promise<string | null> | null = null;
  private sedangMasuk = false;
  private sedangPulihkan = false;

  constructor(
    private readonly api: ApiAuth,
    private readonly google: GoogleAuth,
    private readonly store: PenyimpananSesi,
    private readonly ubahState: (state: AuthState) => void
  ) {}

  /** Keadaan terakhir. Dipakai pembungkus React untuk render pertama. */
  keadaan(): AuthState {
    return this.state;
  }

  private set(sebagian: Partial<AuthState>): void {
    this.state = { ...this.state, ...sebagian };
    this.ubahState(this.state);
  }

  /**
   * Menukar ID token menjadi sesi, lalu menyimpannya.
   *
   * Penyimpanan dilakukan **sebelum** status diubah menjadi `signedIn`: kalau
   * penulisan gagal, pengguna tidak boleh melihat dashboard yang sesinya akan
   * hilang saat aplikasi dibuka ulang.
   */
  private async terapkan(sesi: SesiBackend): Promise<void> {
    const baru: SesiPerangkat = {
      accessToken: sesi.accessToken,
      refreshToken: sesi.refreshToken,
      expiresAtMs: sesi.expiresAtMs,
    };
    try {
      await this.store.simpan(baru);
    } catch (galat) {
      this.sesi = null;
      this.set({
        status: "failed",
        session: null,
        error: {
          kind: "storage",
          message: galat instanceof Error ? galat.message : String(galat),
        },
        tahap: "menganggur",
      });
      return;
    }
    this.sesi = baru;
    this.set({
      status: "signedIn",
      session: { user: sesi.user },
      error: null,
      tahap: "selesai",
    });
  }

  /** Membuang sesi lokal tanpa menghubungi server. */
  private async buang(alasan: SessionError): Promise<void> {
    this.sesi = null;
    await this.store.hapus();
    this.set({ status: "signedOut", session: null, error: alasan, tahap: "menganggur" });
  }

  /**
   * Token akses yang layak dipakai, disegarkan bila mendekati kedaluwarsa.
   *
   * **Satu penerbangan:** pemanggil kedua dan seterusnya menunggu janji yang
   * sama. Tanpa ini, dua permintaan yang bersamaan akan memakai token penyegar
   * yang sama; backend menganggap pemakaian ulang itu indikasi token bocor dan
   * mencabut seluruh sesi pengguna (`docs/API_CONTRACT.md` §5.1, rotasi).
   */
  async tokenAkses(): Promise<string | null> {
    const sesi = this.sesi;
    if (sesi === null) return null;
    if (this.store.masihBerlaku(sesi)) return sesi.accessToken;
    if (this.segarkanSedang !== null) return this.segarkanSedang;
    const jalan = this.lakukanSegarkan(sesi);
    this.segarkanSedang = jalan;
    try {
      return await jalan;
    } finally {
      // Hanya pemilik penerbangan yang membersihkan penandanya.
      if (this.segarkanSedang === jalan) this.segarkanSedang = null;
    }
  }

  private async lakukanSegarkan(sesi: SesiPerangkat): Promise<string | null> {
    const hasil = await this.api.segarkanSesi(sesi.refreshToken);
    if (!hasil.ok) {
      if (sementara(hasil.error)) {
        // Token dipertahankan: sesi belum terbukti tidak sah.
        return null;
      }
      await this.buang({ kind: "sessionEnded" });
      return null;
    }
    // Penulisan gagal tidak boleh membuat token di memori berbeda dengan yang
    // tersimpan di perangkat: pakai token baru hanya setelah tersimpan.
    try {
      await this.store.simpan(hasil.data);
    } catch (galat) {
      this.sesi = null;
      this.set({
        status: "failed",
        session: null,
        error: {
          kind: "storage",
          message: galat instanceof Error ? galat.message : String(galat),
        },
        tahap: "menganggur",
      });
      return null;
    }
    this.sesi = hasil.data;
    return hasil.data.accessToken;
  }

  /**
   * Memulihkan sesi tersimpan, lalu memverifikasinya ke backend.
   *
   * Token lokal tidak pernah dipercaya sebagai bukti: role dan status dibaca
   * ulang lewat `GET /me`.
   */
  async pulihkan(): Promise<void> {
    if (this.sedangPulihkan) return;
    this.sedangPulihkan = true;
    try {
      const tersimpan = await this.store.muat();
      if (tersimpan === null) {
        this.sesi = null;
        this.set({ status: "signedOut", session: null, error: null, tahap: "menganggur" });
        return;
      }
      this.sesi = tersimpan;
      this.set({ status: "memulihkan", error: null });
      const token = await this.tokenAkses();
      if (token === null) {
        // `buang()` sudah mengubah status bila backend menolak. Kalau tidak,
        // penyebabnya sementara dan token tetap tersimpan.
        if (this.state.status === "memulihkan") {
          this.set({
            status: "signedOut",
            session: null,
            error: { kind: "network" },
            tahap: "menganggur",
          });
        }
        return;
      }
      const saya = await this.api.ambilSaya(token);
      if (!saya.ok) {
        if (saya.error.kind === "notGranted") {
          // Akun ini kehilangan akses: sisa token tidak boleh tinggal di
          // perangkat, meskipun backend tetap akan menolaknya.
          this.sesi = null;
          await this.store.hapus();
          this.set({
            status: "unregistered",
            session: null,
            error: saya.error,
            tahap: "menganggur",
          });
          return;
        }
        if (sementara(saya.error)) {
          this.set({
            status: "signedOut",
            session: null,
            error: saya.error,
            tahap: "menganggur",
          });
          return;
        }
        if (saya.error.kind === "sessionEnded") {
          await this.buang({ kind: "sessionEnded" });
          return;
        }
        // Backend menjawab tetapi bentuknya tidak dikenal. Token belum terbukti
        // salah, jadi jangan dibuang; pengguna dapat mencoba lagi.
        this.set({
          status: "signedOut",
          session: null,
          error: saya.error,
          tahap: "menganggur",
        });
        return;
      }
      this.set({
        status: "signedIn",
        session: { user: saya.data },
        error: null,
        tahap: "selesai",
      });
    } finally {
      this.sedangPulihkan = false;
    }
  }

  /**
   * Alur masuk: periksa dukungan → nonce → Google → tukar → simpan.
   *
   * Pemanggilan ganda diabaikan. Tanpa penjaga ini, menekan tombol dua kali
   * meminta dua nonce dan membuka dua lembar akun; nonce pertama hangus dan
   * pengguna melihat kegagalan yang tidak ia sebabkan.
   */
  async masuk(): Promise<void> {
    if (this.sedangMasuk) return;
    const penghalang = this.google.periksaDukungan();
    if (penghalang !== null) {
      this.set({
        status: "failed",
        session: null,
        error: penghalang,
        tahap: "menganggur",
      });
      return;
    }
    this.sedangMasuk = true;
    try {
      this.set({ status: "checking", session: null, error: null, tahap: "google" });
      const nonce = await this.api.mintaNonce();
      if (!nonce.ok) {
        this.set({
          status: "failed",
          session: null,
          error: nonce.error,
          tahap: "menganggur",
        });
        return;
      }
      const google = await this.google.masukDenganGoogle(nonce.data);
      if (!google.ok) {
        // Pembatalan bukan kegagalan: layar kembali tenang tanpa pesan.
        const batal = google.error.kind === "cancelled";
        this.set({
          status: batal ? "signedOut" : "failed",
          session: null,
          error: batal ? null : google.error,
          tahap: "menganggur",
        });
        return;
      }
      this.set({ tahap: "menukar" });
      const tukar = await this.api.tukarIdToken(google.idToken, nonce.data, google.email);
      if (!tukar.ok) {
        if (tukar.error.kind === "notGranted") {
          this.set({
            status: "unregistered",
            session: null,
            error: tukar.error,
            tahap: "menganggur",
          });
          return;
        }
        this.set({
          status: "failed",
          session: null,
          error: tukar.error,
          tahap: "menganggur",
        });
        return;
      }
      this.set({ tahap: "menyiapkan" });
      await this.terapkan(tukar.data);
    } finally {
      this.sedangMasuk = false;
    }
  }

  /**
   * Keluar: bersihkan lokal lebih dahulu, lalu cabut di server.
   *
   * Urutan itu disengaja. Bila pencabutan server gagal (jaringan mati), token
   * di perangkat sudah tidak ada — yang paling mendesak adalah memastikan
   * perangkat berhenti memegang kredensial.
   */
  async keluar(): Promise<void> {
    const sesi = this.sesi;
    this.sesi = null;
    // `try` di sini bukan formalitas: `hapus` menyentuh Keystore dan
    // `keluarSesi` menyentuh jaringan. Kegagalan keduanya tidak boleh membuat
    // pengguna tetap terlihat masuk — keadaan lokal adalah yang paling mendesak.
    try {
      await this.store.hapus();
    } catch {
      // Penyimpanan yang menolak menghapus tetap membuat token di memori
      // hilang; sisa di perangkat akan dibersihkan pada peluncuran berikutnya
      // lewat pemulihan yang ditolak backend.
    }
    if (sesi !== null) {
      try {
        await this.api.keluarSesi(sesi.accessToken);
      } catch {
        // Sesi di server akan kedaluwarsa sendiri (akses 60 menit). Yang harus
        // dipastikan di sini adalah perangkat berhenti memegang kredensial.
      }
    }
    await this.google.lupakanGoogle();
    this.set({ status: "signedOut", session: null, error: null, tahap: "menganggur" });
  }
}

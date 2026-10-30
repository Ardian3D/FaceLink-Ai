import React, { createContext, useContext, useEffect, useMemo, useState } from "react";
import type { Session, SessionError } from "@/types";
import { ambilSaya, keluarSesi, mintaNonce, segarkanSesi, tukarIdToken, verifikasiKodeEmail, type HasilEmail } from "./authApi";
import { AuthFlow, type AuthState, type AuthStatus, type TahapMasuk } from "./authFlow";
import { lupakanGoogle, masukDenganGoogle, periksaDukungan } from "./googleSignIn";
import {
  aksesMasihBerlaku,
  hapusSesiTersimpan,
  muatSesiTersimpan,
  simpanSesi,
} from "./tokenStore";

/**
 * Penyedia sesi FaceLink AI.
 *
 * Berkas ini **hanya** menjembatani `AuthFlow` ke React: seluruh keputusan
 * autentikasi berada di `authFlow.ts` supaya dapat diuji tanpa perangkat
 * Android. Yang ada di sini adalah pembuatan satu instance, langganan perubahan
 * keadaan, dan pemulihan sesi sekali saat aplikasi dibuka.
 *
 * Alur yang dijalankan (lihat `authFlow.ts` untuk aturan lengkapnya):
 *
 *  1. `POST /auth/nonce` — nonce sekali pakai.
 *  2. Google Sign-In di perangkat dengan nonce itu.
 *  3. `POST /auth/google` — backend memverifikasi ID token dan menetapkan role.
 *  4. Token disimpan di penyimpanan kredensial platform; ID token dibuang.
 *
 * Role tidak pernah ditetapkan aplikasi. Token tidak pernah ada di state React
 * maupun di log; `Session` hanya memuat `user`.
 */
export type { AuthStatus, TahapMasuk };

export type SessionContextValue = AuthState & {
  /** Titik masuk tombol "Continue with Google". */
  startSignIn: () => Promise<void>;
  signInEmail: (challengeId: string, code: string, signal: AbortSignal) => Promise<HasilEmail<void>>;
  /** Mengulang pemulihan sesi tersimpan tanpa mengulang Google Sign-In. */
  ulangiPemulihan: () => Promise<void>;
  /** Token akses untuk modul fitur yang tidak memegang AuthFlow; tidak masuk state React. */
  getAccessToken: () => Promise<string | null>;
  /** Menutup sesi: hapus token lokal, cabut di server, lupakan akun Google. */
  signOut: () => Promise<void>;
};

const SessionContext = createContext<SessionContextValue | null>(null);

export function SessionProvider({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<AuthState>({
    status: "memulihkan",
    session: null,
    error: null,
    tahap: "menganggur",
  });

  // Instance dibuat sekali lewat penginisialisasi `useState`, bukan ref yang
  // dibaca saat render. Membaca ref pada render melanggar aturan React dan
  // menghasilkan instance berbeda antara render pertama dan StrictMode.
  // Pembuatan ini murni (tanpa I/O, tanpa efek samping), jadi aman bila
  // penginisialisasi dijalankan dua kali.
  const [alur] = useState(
    () =>
      new AuthFlow(
        { mintaNonce, tukarIdToken, segarkanSesi, ambilSaya, keluarSesi, verifikasiKodeEmail },
        { periksaDukungan, masukDenganGoogle, lupakanGoogle },
        {
          muat: muatSesiTersimpan,
          simpan: simpanSesi,
          hapus: hapusSesiTersimpan,
          masihBerlaku: aksesMasihBerlaku,
        },
        setState
      )
  );

  useEffect(() => {
    // Pemulihan hanya sekali per pemasangan. `AuthFlow` sendiri menolak
    // pemulihan bersamaan, sehingga efek ganda pada StrictMode aman.
    void alur.pulihkan();
  }, [alur]);

  const value = useMemo<SessionContextValue>(
    () => ({
      ...state,
      startSignIn: () => alur.masuk(),
      signInEmail: (challengeId, code, signal) => alur.masukEmail(challengeId, code, signal),
      ulangiPemulihan: () => alur.pulihkan(),
      getAccessToken: () => alur.tokenAkses(),
      signOut: () => alur.keluar(),
    }),
    [alur, state]
  );

  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

/** Mengambil keadaan sesi. Melempar bila dipakai di luar `SessionProvider`. */
export function useSession(): SessionContextValue {
  const ctx = useContext(SessionContext);
  if (!ctx) throw new Error("useSession harus dipakai di dalam SessionProvider");
  return ctx;
}

export type { Session, SessionError };

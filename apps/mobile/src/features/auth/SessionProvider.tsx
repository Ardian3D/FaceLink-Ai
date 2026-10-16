import React, { createContext, useCallback, useContext, useMemo, useState } from "react";

import type { Role, Session, SessionError } from "@/types";
import { akunDemo } from "./demoAccounts";
import { DEMO_MODE } from "./demoMode";

/**
 * Status autentikasi di lapisan UI.
 *
 * `checking` adalah keadaan yang dipakai layar Role Checking: aplikasi sedang
 * memastikan identitas dan role. Pada FL-UI01 keadaan itu dipenuhi dari mock
 * development; pada integrasi nanti ia menjadi permintaan sesi ke Go Backend.
 */
export type AuthStatus = "signedOut" | "checking" | "signedIn" | "failed";

export type AuthState = {
  status: AuthStatus;
  session: Session | null;
  error: SessionError | null;
};

export type SessionContextValue = AuthState & {
  /**
   * Titik masuk tombol "Continue with Google".
   *
   * FL-UI01 belum mengintegrasikan OAuth. Pada development build fungsi ini
   * menjalankan alur demo lokal; pada build produksi ia berhenti dengan pesan
   * bahwa masuk dengan Google belum tersedia, tanpa pernah memberi sesi. Tidak
   * ada jalur yang memberi akses tanpa verifikasi backend.
   */
  startSignIn: () => void;
  /**
   * Menetapkan role yang sedang dipreview. Hanya berpengaruh pada development
   * build; pada build produksi panggilan ini tidak mengubah keadaan.
   */
  previewAs: (role: Role) => void;
  /** Menutup sesi demo dan kembali ke layar masuk. */
  signOut: () => void;
};

const SessionContext = createContext<SessionContextValue | null>(null);

export function SessionProvider({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<AuthState>({ status: "signedOut", session: null, error: null });

  const startSignIn = useCallback(() => {
    if (!DEMO_MODE) {
      // Tidak ada fallback yang memberi sesi: integrasi Google Auth adalah
      // tugas tersendiri dan harus lewat Go Backend.
      setState({ status: "failed", session: null, error: { kind: "unknown", message: "notImplemented" } });
      return;
    }
    setState({ status: "checking", session: null, error: null });
  }, []);

  const previewAs = useCallback((role: Role) => {
    if (!DEMO_MODE) return;
    setState({ status: "signedIn", session: { user: akunDemo[role], demo: true }, error: null });
  }, []);

  const signOut = useCallback(() => {
    setState({ status: "signedOut", session: null, error: null });
  }, []);

  const value = useMemo<SessionContextValue>(
    () => ({ ...state, startSignIn, previewAs, signOut }),
    [state, startSignIn, previewAs, signOut]
  );

  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

/** Mengambil keadaan sesi. Melempar bila dipakai di luar `SessionProvider`. */
export function useSession(): SessionContextValue {
  const ctx = useContext(SessionContext);
  if (!ctx) throw new Error("useSession harus dipakai di dalam SessionProvider");
  return ctx;
}

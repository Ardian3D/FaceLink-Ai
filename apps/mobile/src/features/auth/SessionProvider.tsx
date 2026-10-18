import React, { createContext, useCallback, useContext, useMemo, useState } from "react";

import type { Role, Session, SessionError } from "@/types";
import { ROLE_DEMO, akunDemo } from "./demoAccounts";
import { DEMO_MODE } from "./demoMode";

/**
 * Status autentikasi di lapisan UI.
 *
 * `checking` adalah keadaan yang dipakai layar Role Checking: aplikasi sedang
 * memastikan identitas dan role. Pada FL-UI01 keadaan itu dipenuhi dari mock
 * development; pada integrasi nanti ia menjadi permintaan sesi ke Go Backend.
 *
 * `unregistered` adalah keadaan tersendiri, bukan `failed`: identitas Google
 * sudah sah, tetapi akunnya belum ada di database FaceLink AI. Pengguna seperti
 * ini harus melihat penjelasan yang menenangkan dan satu langkah yang jelas —
 * bukan pesan galat teknis yang membuatnya mencoba login berulang kali.
 */
export type AuthStatus = "signedOut" | "checking" | "signedIn" | "failed" | "unregistered";

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
  /**
   * Menandai bahwa akun yang mencoba masuk ditolak backend.
   *
   * Dipanggil oleh klien API saat `POST /auth/google` menjawab
   * `403 IDENTITY_NOT_LINKED` (`docs/AUTHORIZATION.md` §6.1). Kontrak memakai
   * satu pesan generik untuk semua sebab — "belum diundang", "belum ditautkan",
   * dan "ditangguhkan" — sehingga pemanggil menentukan `alasan` dari audit-side
   * knowledge bila tersedia; tanpa itu, `notGranted` adalah pilihan aman.
   *
   * `email` berasal dari klaim Google milik pengguna itu sendiri, bukan dari
   * jawaban backend, jadi menampilkannya tidak membocorkan data akun lain.
   * Saat ini hanya dipakai oleh jalur demo untuk memperlihatkan layarnya.
   */
  markUnregistered: (alasan?: "notGranted" | "suspended", email?: string) => void;
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
    // Mode demo: sesi langsung diisi role contoh, meniru jawaban backend yang
    // menetapkan role. Layar pemeriksaan peran hanya menampilkan prosesnya;
    // ia tidak pernah menaikkan hak akses, dan pada build produksi blok ini
    // tidak dijalankan sama sekali.
    setState({
      status: "checking",
      session: { user: akunDemo[ROLE_DEMO], demo: true },
      error: null,
    });
  }, []);

  const previewAs = useCallback((role: Role) => {
    if (!DEMO_MODE) return;
    setState({ status: "signedIn", session: { user: akunDemo[role], demo: true }, error: null });
  }, []);

  const markUnregistered = useCallback(
    (alasan: "notGranted" | "suspended" = "notGranted", email?: string) => {
      // Tidak ada sesi yang dibuat di sini. Statusnya sengaja bukan
      // "signedIn": seluruh penjaga navigasi akan menolak, sehingga tidak ada
      // beranda yang bisa terbuka lewat keadaan ini.
      setState({
        status: "unregistered",
        session: null,
        error: alasan === "suspended" ? { kind: "suspended", email } : { kind: "notGranted", email },
      });
    },
    []
  );

  const signOut = useCallback(() => {
    setState({ status: "signedOut", session: null, error: null });
  }, []);

  const value = useMemo<SessionContextValue>(
    () => ({ ...state, startSignIn, previewAs, markUnregistered, signOut }),
    [state, startSignIn, previewAs, markUnregistered, signOut]
  );

  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

/** Mengambil keadaan sesi. Melempar bila dipakai di luar `SessionProvider`. */
export function useSession(): SessionContextValue {
  const ctx = useContext(SessionContext);
  if (!ctx) throw new Error("useSession harus dipakai di dalam SessionProvider");
  return ctx;
}

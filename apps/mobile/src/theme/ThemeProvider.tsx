import AsyncStorage from "@react-native-async-storage/async-storage";
import React, { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { useColorScheme } from "react-native";

import { dark, light, type Palette } from "./tokens";

/**
 * Tema terang/gelap untuk **area setelah masuk**.
 *
 * Layar masuk dan layar pemeriksaan peran sengaja selalu gelap: keduanya adalah
 * muka merek dengan video dan cincin pemindaian yang dirancang untuk latar
 * hitam. Karena itu komponen di sana memakai `palet.dark` langsung, bukan
 * tema aktif.
 *
 * Pilihan pengguna disimpan di perangkat (AsyncStorage) supaya tidak berubah
 * setiap aplikasi dibuka. Selama penyimpanan belum terbaca, aplikasi memakai
 * tema gelap — tema utama sesuai logo — lalu menyesuaikan bila pilihan
 * tersimpan ternyata terang. Dengan begitu tidak ada kedipan tema pada
 * pembukaan pertama.
 *
 * Preferensi sistem hanya dipakai sebagai nilai awal bila pengguna **belum
 * pernah** memilih. Setelah pengguna memilih lewat Settings, pilihannya menang
 * atas pengaturan sistem.
 */

export type ThemeMode = "dark" | "light";

const KUNCI_SIMPAN = "facelink.theme";

type ThemeContextValue = {
  /** Tema yang sedang dipakai. */
  mode: ThemeMode;
  /** Palet warna aktif. Seluruh komponen memakai objek ini. */
  colors: Palette;
  /** `true` bila pilihan sudah terbaca dari penyimpanan. */
  siap: boolean;
  /** Mengubah tema dan menyimpannya. */
  setMode: (mode: ThemeMode) => void;
  /** Membalik tema; dipakai tombol pintas di Settings. */
  toggle: () => void;
};

const ThemeContext = createContext<ThemeContextValue | null>(null);

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const sistem = useColorScheme();
  const [mode, setModeState] = useState<ThemeMode>("dark");
  const [siap, setSiap] = useState(false);

  useEffect(() => {
    let aktif = true;
    AsyncStorage.getItem(KUNCI_SIMPAN)
      .then((tersimpan) => {
        if (!aktif) return;
        if (tersimpan === "light" || tersimpan === "dark") {
          setModeState(tersimpan);
        } else if (sistem === "light") {
          // Belum pernah memilih: ikuti sistem sekali, lalu pengguna bebas
          // mengubahnya di Settings.
          setModeState("light");
        }
      })
      .catch(() => {
        // Penyimpanan tidak tersedia (mis. web dengan storage diblokir).
        // Tema gelap tetap dipakai; ini bukan alasan menghentikan aplikasi.
      })
      .finally(() => {
        if (aktif) setSiap(true);
      });
    return () => {
      aktif = false;
    };
  }, [sistem]);

  const setMode = useCallback((baru: ThemeMode) => {
    setModeState(baru);
    AsyncStorage.setItem(KUNCI_SIMPAN, baru).catch(() => {
      // Gagal menyimpan hanya berarti pilihan tidak bertahan setelah aplikasi
      // ditutup; tema yang sedang tampil tidak terpengaruh.
    });
  }, []);

  const toggle = useCallback(() => {
    setModeState((lama) => {
      const baru: ThemeMode = lama === "dark" ? "light" : "dark";
      AsyncStorage.setItem(KUNCI_SIMPAN, baru).catch(() => {});
      return baru;
    });
  }, []);

  const value = useMemo<ThemeContextValue>(
    () => ({
      mode,
      colors: mode === "dark" ? dark : light,
      siap,
      setMode,
      toggle,
    }),
    [mode, siap, setMode, toggle]
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

/** Mengambil tema aktif. Melempar bila dipakai di luar `ThemeProvider`. */
export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error("useTheme harus dipakai di dalam ThemeProvider");
  return ctx;
}

/**
 * Membatasi tema pada satu cabang tampilan.
 *
 * Dipakai layar masuk dan layar pemeriksaan peran: keduanya selalu gelap
 * karena dirancang untuk latar hitam (video dan cincin pemindaian), sedangkan
 * tema pengguna hanya berlaku setelah masuk. Semua komponen di dalamnya tetap
 * memakai `useTheme()` seperti biasa — mereka tidak perlu tahu bahwa temanya
 * dikunci.
 */
export function BrandThemeScope({ children }: { children: React.ReactNode }) {
  const nilai = useMemo<ThemeContextValue>(
    () => ({ mode: "dark", colors: dark, siap: true, setMode: () => {}, toggle: () => {} }),
    []
  );
  return <ThemeContext.Provider value={nilai}>{children}</ThemeContext.Provider>;
}

/**
 * Palet untuk komponen yang tidak boleh memakai hook (mis. `StyleSheet.create`
 * di tingkat modul). Komponen yang tampil di layar tetap harus memakai
 * `useTheme()` supaya ikut berubah saat tema diganti.
 */
export const palet: Record<ThemeMode, Palette> = { dark, light };

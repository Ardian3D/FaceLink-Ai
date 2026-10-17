import React from "react";
import { Stack } from "expo-router";
import { StatusBar } from "expo-status-bar";
import * as SplashScreen from "expo-splash-screen";
import { useFonts } from "expo-font";
import { SafeAreaProvider } from "react-native-safe-area-context";
// Impor per ketebalan, bukan dari akar paket: akar paket me-`require` seluruh
// 18 berkas (termasuk italic dan black), dan Metro ikut membundel semuanya.
import { Poppins_400Regular } from "@expo-google-fonts/poppins/400Regular";
import { Poppins_500Medium } from "@expo-google-fonts/poppins/500Medium";
import { Poppins_600SemiBold } from "@expo-google-fonts/poppins/600SemiBold";
import { Poppins_700Bold } from "@expo-google-fonts/poppins/700Bold";
import { Poppins_800ExtraBold } from "@expo-google-fonts/poppins/800ExtraBold";

import { SessionProvider } from "@/features/auth/SessionProvider";
import { colors } from "@/theme";

/**
 * Layout akar.
 *
 * Seluruh layar memakai header bawaan yang dimatikan: setiap layar menyusun
 * judulnya sendiri melalui komponen tema, sehingga tidak ada dua sistem judul
 * yang bersaing dan status bar Android tetap terkendali.
 *
 * Font Poppins dimuat di sini, sekali, sebelum layar pertama tampil. Splash
 * ditahan selama pemuatan supaya teks tidak sempat berkedip dengan font
 * sistem. Bila berkas font gagal dimuat (mis. aset hilang pada bundel rusak),
 * aplikasi tetap berjalan dengan font sistem — teks lebih penting daripada
 * jenis hurufnya.
 */
SplashScreen.preventAutoHideAsync().catch(() => {
  // Tidak berbahaya: splash mungkin sudah disembunyikan atau tidak tersedia
  // pada platform ini. Kegagalan menahannya tidak boleh menghentikan aplikasi.
});

export default function RootLayout() {
  const [fontSiap, galatFont] = useFonts({
    Poppins_400Regular,
    Poppins_500Medium,
    Poppins_600SemiBold,
    Poppins_700Bold,
    Poppins_800ExtraBold,
  });

  const siap = fontSiap || galatFont !== null;

  React.useEffect(() => {
    if (siap) SplashScreen.hideAsync().catch(() => {});
  }, [siap]);

  if (!siap) return null;

  return (
    <SafeAreaProvider>
      <SessionProvider>
        <StatusBar style="light" />
        <Stack
          screenOptions={{
            headerShown: false,
            contentStyle: { backgroundColor: colors.background },
            animation: "fade",
          }}
        />
      </SessionProvider>
    </SafeAreaProvider>
  );
}

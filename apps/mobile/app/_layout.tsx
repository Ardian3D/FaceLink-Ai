import React from "react";
import { Stack } from "expo-router";
import { StatusBar } from "expo-status-bar";
import { SafeAreaProvider } from "react-native-safe-area-context";

import { SessionProvider } from "@/features/auth/SessionProvider";
import { colors } from "@/theme";

/**
 * Layout akar.
 *
 * Seluruh layar memakai header bawaan yang dimatikan: setiap layar menyusun
 * judulnya sendiri melalui komponen tema, sehingga tidak ada dua sistem judul
 * yang bersaing dan status bar Android tetap terkendali.
 */
export default function RootLayout() {
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

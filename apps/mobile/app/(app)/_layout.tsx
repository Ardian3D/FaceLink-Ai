import React, { useEffect } from "react";
import { Redirect, Stack, useRouter } from "expo-router";

import { useSession } from "@/features/auth/SessionProvider";
import { colors } from "@/theme";

/**
 * Kerangka area terautentikasi.
 *
 * Penjaga di sini berlaku untuk seluruh layar dashboard: tanpa sesi, pengguna
 * dikembalikan ke layar masuk. Ini penjagaan **navigasi**, bukan pengendalian
 * akses — otorisasi sebenarnya tetap milik Go Backend pada setiap permintaan
 * (docs/AUTHORIZATION.md §5), dan role frontend tidak pernah dipercaya.
 */
export default function AppLayout() {
  const { status, session } = useSession();
  const router = useRouter();

  useEffect(() => {
    if (status === "signedOut") router.replace("/");
  }, [router, status]);

  if (status !== "signedIn" || !session) return <Redirect href="/" />;

  return (
    <Stack
      screenOptions={{
        headerShown: false,
        contentStyle: { backgroundColor: colors.background },
        animation: "fade",
      }}
    />
  );
}

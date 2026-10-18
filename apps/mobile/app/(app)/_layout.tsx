import React, { useEffect } from "react";
import { Redirect, Stack, useRouter } from "expo-router";

import { useSession } from "@/features/auth/SessionProvider";
import { useTheme } from "@/theme";

/**
 * Kerangka area terautentikasi.
 *
 * Penjaga di sini berlaku untuk seluruh layar dashboard: tanpa sesi, pengguna
 * dikembalikan ke layar masuk. Ini penjagaan **navigasi**, bukan pengendalian
 * akses — otorisasi sebenarnya tetap milik Go Backend pada setiap permintaan
 * (docs/AUTHORIZATION.md §5), dan role frontend tidak pernah dipercaya.
 *
 * Status `checking` **tidak** dianggap tanpa sesi: pada keadaan itulah layar
 * pemeriksaan peran sedang berjalan. Kalau penjaga ini mengalihkan pada
 * `checking`, alur masuk → loading → beranda akan terlempar kembali ke layar
 * masuk sebelum role sempat ditetapkan.
 */
export default function AppLayout() {
  const { colors } = useTheme();
  const { status, session } = useSession();
  const router = useRouter();

  useEffect(() => {
    if (status === "signedOut") router.replace("/");
  }, [router, status]);

  // Hanya keadaan yang benar-benar berakhir tanpa sesi yang dialihkan.
  if (status === "signedOut" || status === "failed") return <Redirect href="/" />;
  if (!session) return <Redirect href="/" />;

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

import React from "react";
import { Redirect, Stack } from "expo-router";
import { useSession } from "@/features/auth/SessionProvider";
import { useTheme } from "@/theme";
import { AdminReviewProvider } from "@/features/adminEnrollment/ReviewSession";
import { adminEnrollmentStagingAktif } from "@/features/adminEnrollment/staging";

/**
 * Kerangka area terautentikasi.
 *
 * Penjaga di sini berlaku untuk seluruh layar dashboard: tanpa sesi, pengguna
 * dikembalikan ke layar masuk. Ini penjagaan **navigasi**, bukan pengendalian
 * akses — otorisasi sebenarnya tetap milik Go Backend pada setiap permintaan
 * (docs/AUTHORIZATION.md §5), dan role frontend tidak pernah dipercaya.
 *
 * Aturannya satu: **tanpa objek sesi, tidak ada dashboard** — termasuk selama
 * `memulihkan` dan `checking`, ketika sesi belum diverifikasi. Redirect itu
 * tidak membuat pengguna kehilangan progres: layar masuk memutar status
 * pemulihan sendiri, dan layar pemeriksaan peran tetap berjalan sebagai rute
 * tersendiri sampai sesi sah. Dulu `checking` dikecualikan, tetapi pengecualian
 * itu membiarkan rute dashboard dipasang dengan `session === null`.
 *
 * Penolakan akun ditangani lebih dahulu karena statusnya tidak pernah membawa
 * sesi dan penjelasannya ada di layar tersendiri.
 */
export default function AppLayout() {
  const { colors } = useTheme();
  const { status, session } = useSession();

  if (status === "unregistered") return <Redirect href="/not-registered" />;
  if (session === null) return <Redirect href="/" />;

  const content = (
    <Stack
      screenOptions={{
        headerShown: false,
        contentStyle: { backgroundColor: colors.background },
        animation: "fade",
      }}
    />
  );
  return adminEnrollmentStagingAktif && session.user.role === "admin"
    ? <AdminReviewProvider key={`${session.user.id}:${session.user.role}`}>{content}</AdminReviewProvider>
    : content;
}

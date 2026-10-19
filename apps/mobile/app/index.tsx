import React, { useEffect, useRef } from "react";
import { Redirect, useRouter } from "expo-router";
import { StyleSheet, View } from "react-native";
import { LinearGradient } from "expo-linear-gradient";
import { AppButton } from "@/components/AppButton";
import { AppText } from "@/components/AppText";
import { BrandLogo } from "@/components/BrandLogo";
import { GoogleSignInButton } from "@/components/GoogleSignInButton";
import { Screen } from "@/components/Screen";
import { StateView } from "@/components/StateView";
import { VideoBackground } from "@/components/VideoBackground";
import { useSession } from "@/features/auth/SessionProvider";
import { roleHome } from "@/features/auth/roleRoutes";
import type { SessionError } from "@/types";
import { BrandThemeScope, layout, spacing, useTheme } from "@/theme";

/**
 * Layar masuk (FL-UI02), mengikuti mockup "FaceLink AI Sign-in Page".
 *
 * Seluruh layar ditutup latar bergerak — klip video pemilik project bila
 * tersedia di mesin ini, atau latar animasi kode bila tidak (lihat
 * `assets/video/BACA-SAYA.md`) — lalu diberi lapisan gelap agar logo, wordmark,
 * dan tombol tetap terbaca. Lapisan itu makin pekat ke bawah, jadi bagian atas
 * video tetap terlihat.
 *
 * Satu tombol, satu jalur: "Continue with Google" meminta nonce ke backend,
 * menjalankan Google Sign-In, lalu menukar ID token menjadi sesi FaceLink AI.
 * Tidak ada tombol "Skip", tidak ada masuk tanpa verifikasi, dan tidak ada
 * jalur yang menentukan role sendiri — role datang dari backend.
 *
 * Setiap kegagalan yang mungkin terjadi punya kalimat dan tindakan sendiri.
 * Menampilkan "gagal" untuk semua sebab akan menyuruh pengguna memperbaiki hal
 * yang salah: sertifikat bukan Wi-Fi, dan konfigurasi build bukan masalah
 * jaringan.
 */
export default function LoginScreen() {
  // Layar masuk selalu gelap: ia dirancang untuk latar video hitam, bukan
  // mengikuti tema dashboard. Isinya dipisah ke komponen anak supaya
  // `useTheme()` di dalamnya membaca palet yang dikunci itu.
  return (
    <BrandThemeScope>
      <LayarMasuk />
    </BrandThemeScope>
  );
}

function LayarMasuk() {
  const { colors } = useTheme();
  const router = useRouter();
  const { status, error, session, tahap, startSignIn, ulangiPemulihan } = useSession();
  const mengecek = status === "checking";
  const memulihkan = status === "memulihkan";
  // Efek di bawah bergantung pada `tahap`, yang berubah dua kali selama proses
  // masuk. Tanpa penjaga ini setiap perubahan menambah satu layar pemeriksaan
  // peran ke tumpukan navigasi, dan tombol kembali akan menelusuri riwayat
  // loading yang sudah selesai.
  const sudahNavigasi = useRef(false);

  // Layar pemeriksaan peran baru dibuka setelah lembar akun Google tertutup:
  // selama pengguna masih memilih akun, yang berjalan baru satu langkah dan
  // menampilkan tiga langkah di belakang lembar sistem hanya menyesatkan.
  // Navigasi ada di efek, bukan di handler tombol, karena `startSignIn` sendiri
  // tidak memegang router dan efek ini juga menangani percobaan ulang.
  useEffect(() => {
    if (
      status === "checking" &&
      (tahap === "menukar" || tahap === "menyiapkan") &&
      !sudahNavigasi.current
    ) {
      sudahNavigasi.current = true;
      router.push("/role-checking");
    }
    // Proses baru (atau percobaan ulang setelah gagal) perlu izin navigasi lagi.
    if (status === "signedOut" || status === "failed") sudahNavigasi.current = false;
  }, [router, status, tahap]);

  // Sesi yang sudah sah tidak perlu melihat layar masuk lagi.
  if (status === "signedIn" && session !== null) {
    return <Redirect href={roleHome[session.user.role]} />;
  }

  // Akun yang ditolak mendapat penjelasan tersendiri, bukan kotak galat di
  // layar masuk: pengguna perlu membaca langkah yang harus ditempuh admin.
  if (status === "unregistered") return <Redirect href="/not-registered" />;

  const pesan = pesanGalat(error);

  return (
    <View style={[styles.layar, { backgroundColor: colors.background }]}>
      <VideoBackground />
      <LinearGradient
        colors={[
          colors.overlayLight,
          colors.overlayMedium,
          colors.overlayStrong,
          colors.overlayStrong,
        ]}
        locations={[0, 0.28, 0.55, 1]}
        style={StyleSheet.absoluteFill}
        pointerEvents="none"
      />
      <Screen transparent scroll contentStyle={styles.isi}>
        {/* Dua ruang kosong dengan rasio berbeda menempatkan blok identitas di
              bagian bawah layar seperti pada mockup. Rasio dipakai, bukan
              `justifyContent: space-between`, supaya posisinya tetap sama pada
              layar pendek maupun tinggi. */}
        <View style={styles.ruangAtas} />
        <View style={styles.tengah}>
          <BrandLogo size={layout.logoSize} />
          <View style={styles.aksi}>
            {memulihkan ? (
              <StateView
                tone="loading"
                title="Memeriksa sesi tersimpan…"
                description="Sesi tidak pernah dipercaya begitu saja; backend memeriksa ulang sebelum beranda dibuka."
              />
            ) : (
              <>
                <GoogleSignInButton
                  onPress={() => {
                    void startSignIn();
                  }}
                  loading={mengecek}
                />
                <AppText variant="caption" color={colors.textSecondary} center>
                  Masuk memerlukan internet. Role akun ditentukan backend FaceLink
                  AI, bukan oleh aplikasi.
                </AppText>
              </>
            )}
            {pesan !== null && !mengecek && !memulihkan ? (
              <StateView
                tone="error"
                title={pesan.judul}
                description={pesan.keterangan}
                action={
                  pesan.bisaUlangi ? (
                    <AppButton
                      label="Coba lagi"
                      variant="secondary"
                      onPress={() => {
                        // Bila sesi tersimpan gagal diverifikasi karena
                        // jaringan, mengulang pemulihan tidak memaksa pengguna
                        // memilih akun Google lagi.
                        void ulangiPemulihan();
                      }}
                    />
                  ) : undefined
                }
              />
            ) : null}
          </View>
        </View>
        <View style={styles.ruangBawah} />
        <View style={styles.bawah}>
          {/* Mockup menampilkan tautan kebijakan di dasar layar. Dokumen publiknya
                belum ada (docs/ baru memuat dokumen internal), jadi keduanya
                ditampilkan sebagai label — bukan tautan yang bisa ditekan dan tidak
                menuju ke mana-mana. Beri tahu pemilik project bila dokumennya sudah
                tersedia supaya baris ini berubah menjadi tautan sungguhan. */}
          <View style={styles.footer}>
            <AppText variant="caption" color={colors.textMuted}>
              Privacy policy
            </AppText>
            <View style={[styles.pemisah, { backgroundColor: colors.border }]} />
            <AppText variant="caption" color={colors.textMuted}>
              Terms of service
            </AppText>
          </View>
        </View>
      </Screen>
    </View>
  );
}

/**
 * Kalimat dan tindakan untuk setiap sebab kegagalan yang benar-benar dapat
 * terjadi. `null` berarti tidak ada yang perlu ditampilkan (mis. pengguna
 * membatalkan lembar akun Google).
 */
function pesanGalat(
  error: SessionError | null
): { judul: string; keterangan: string; bisaUlangi: boolean } | null {
  if (error === null) return null;
  switch (error.kind) {
    case "cancelled":
      return null;
    case "network":
      return {
        judul: "Backend tidak terjangkau",
        keterangan:
          "Masuk memerlukan koneksi ke server FaceLink AI. Pada build development, pastikan `adb reverse tcp:8080 tcp:8080` sudah dipasang dan backend berjalan.",
        bisaUlangi: true,
      };
    case "tls":
      return {
        judul: "Sertifikat server tidak tepercaya",
        keterangan:
          "Server menjawab, tetapi sertifikatnya tidak dipercaya perangkat ini. Ini bukan masalah jaringan; hubungi admin agar sertifikat yang benar dipasang.",
        bisaUlangi: false,
      };
    case "config":
      return {
        judul: "Aplikasi belum dikonfigurasi",
        keterangan: error.message,
        bisaUlangi: false,
      };
    case "storage":
      return {
        judul: "Sesi tidak dapat disimpan",
        keterangan:
          "Perangkat gagal menyimpan token sesi di penyimpanan kredensial, atau platform ini tidak menyediakannya. Masuk di perangkat Android, atau buka kunci layar lalu coba lagi.",
        bisaUlangi: true,
      };
    case "unavailable":
      return {
        judul: "Layanan masuk belum siap",
        keterangan:
          "Backend belum dikonfigurasi untuk Google Sign-In. Hubungi admin; percobaan ulang tidak akan berhasil sampai konfigurasi server diperbaiki.",
        bisaUlangi: false,
      };
    case "playServices":
      return {
        judul: "Google Play Services bermasalah",
        keterangan:
          "Perbarui Google Play Services di perangkat ini, lalu coba lagi.",
        bisaUlangi: true,
      };
    case "unsupported":
      return {
        judul: "Masuk Google tidak tersedia di sini",
        keterangan:
          "Masuk dengan Google memerlukan development build atau APK di perangkat Android. Pratinjau web dan Expo Go tidak memuat modul native yang dibutuhkannya.",
        bisaUlangi: false,
      };
    case "sessionEnded":
      return {
        judul: "Sesi berakhir",
        keterangan:
          "Sesi Anda tidak berlaku lagi. Masuk kembali dengan Google untuk melanjutkan.",
        bisaUlangi: false,
      };
    case "identityRejected":
      return {
        judul: "Google menolak memverifikasi sesi ini",
        keterangan:
          "Backend tidak menerima ID token Google (tanda tangan, audience, waktu, atau nonce tidak cocok). Periksa apakah jam perangkat benar dan client ID pada build ini sama dengan yang dikonfigurasi backend, lalu coba lagi.",
        bisaUlangi: true,
      };
    case "notGranted":
      return {
        judul: "Akun belum diberi akses",
        keterangan: "Akun ini belum terdaftar di FaceLink AI. Hubungi admin.",
        bisaUlangi: false,
      };
    case "unknown":
      return {
        judul: "Masuk gagal",
        keterangan: `Backend menjawab dengan keadaan yang belum dikenal aplikasi ini (${error.message}).`,
        bisaUlangi: true,
      };
    default: {
      // Pemeriksaan kelengkapan: penambahan keadaan baru pada `SessionError`
      // memaksa fungsi ini diperbarui, bukan menampilkan pesan kosong.
      const belum: never = error;
      return {
        judul: "Masuk gagal",
        keterangan: String(belum),
        bisaUlangi: false,
      };
    }
  }
}

const styles = StyleSheet.create({
  layar: {
    flex: 1,
  },
  isi: {
    justifyContent: "flex-start",
    gap: spacing.lg,
    paddingTop: spacing.sm,
    paddingBottom: spacing.xl,
  },
  /**
   * Ruang kosong di atas dan di bawah blok identitas. Perbandingan 5:1
   * menaruh mark sekitar 51% tinggi layar, wordmark ~62%, dan tombol ~75% —
   * sedikit lebih tinggi daripada mockup, sesuai permintaan pemilik. Karena
   * memakai `flex`, proporsinya bertahan pada layar pendek maupun tinggi.
   */
  ruangAtas: {
    flex: 5,
    minHeight: spacing.xxl,
  },
  ruangBawah: {
    flex: 1,
    minHeight: spacing.xl,
  },
  tengah: {
    alignItems: "center",
    gap: spacing.xxxl,
  },
  aksi: {
    width: "100%",
    gap: spacing.md,
  },
  bawah: {
    gap: spacing.sm,
  },
  footer: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    gap: spacing.md,
  },
  pemisah: {
    width: StyleSheet.hairlineWidth,
    height: spacing.lg,
  },
});

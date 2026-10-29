import React, { useCallback, useEffect, useState } from "react";
import { Redirect, useFocusEffect, useRouter } from "expo-router";
import {
  AppState,
  BackHandler,
  KeyboardAvoidingView,
  Platform,
  Pressable,
  StyleSheet,
  TextInput,
  View,
} from "react-native";
import { LinearGradient } from "expo-linear-gradient";
import { Ionicons } from "@expo/vector-icons";
import { AppButton } from "@/components/AppButton";
import { AppText } from "@/components/AppText";
import { BrandLogo } from "@/components/BrandLogo";
import { MailIcon } from "@/components/MailIcon";
import { Screen } from "@/components/Screen";
import { VideoBackground } from "@/components/VideoBackground";
import { mintaKodeEmail } from "@/features/auth/authApi";
import { EmailOtpFlow, pesanEmail } from "@/features/auth/emailOtpFlow";
import { useSession } from "@/features/auth/SessionProvider";
import { roleHome } from "@/features/auth/roleRoutes";
import { BrandThemeScope, layout, radius, spacing, touch, typography, useTheme } from "@/theme";

export default function EmailLoginScreen() {
  return (
    <BrandThemeScope>
      <LayarEmail />
    </BrandThemeScope>
  );
}

function LayarEmail() {
  const { colors } = useTheme();
  const router = useRouter();
  const { signInEmail, status, session } = useSession();
  const [flow] = useState(() => new EmailOtpFlow(mintaKodeEmail, signInEmail));
  const [state, setState] = useState(flow.keadaan());
  const [email, setEmail] = useState("");
  const [code, setCode] = useState("");
  const [now, setNow] = useState(Date.now);

  useEffect(() => flow.subscribe(setState), [flow]);
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 1000);
    const subscription = AppState.addEventListener("change", (next) => {
      if (next === "active") setNow(Date.now());
    });
    return () => {
      clearInterval(timer);
      subscription.remove();
    };
  }, []);

  const kembali = useCallback(() => {
    setCode("");
    if (flow.keadaan().challenge) flow.back();
    else {
      flow.cancel();
      setEmail("");
      if (router.canGoBack()) router.back();
      else router.replace("/");
    }
  }, [flow, router]);

  useFocusEffect(
    useCallback(() => {
      const back = BackHandler.addEventListener("hardwareBackPress", () => {
        kembali();
        return true;
      });
      return () => {
        back.remove();
        flow.cancel();
        setCode("");
        setEmail("");
      };
    }, [flow, kembali])
  );

  if (status === "signedIn" && session) return <Redirect href={roleHome[session.user.role]} />;

  const otp = state.challenge !== null;
  const retry = Math.max(0, Math.ceil((state.retryAt - now) / 1000));
  const verifyRetry = Math.max(0, Math.ceil((state.verifyAt - now) / 1000));
  const expires = Math.max(0, Math.ceil((state.expiresAt - now) / 1000));
  const busy = state.busy !== null;
  const emailValid =
    email.trim().length <= 254 && /^[^\s@<>]+@[^\s@<>]+\.[^\s@<>]+$/.test(email.trim());
  const kirim = () => {
    setCode("");
    void flow.minta(otp ? state.email : email);
  };
  const verifikasi = () => {
    const nilai = code;
    setCode("");
    void flow.verifikasi(nilai);
  };
  const inputStyle = [
    styles.input,
    {
      color: colors.textPrimary,
      backgroundColor: colors.surface,
      borderColor: colors.border,
    },
  ];

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
      <KeyboardAvoidingView
        style={styles.layar}
        behavior={Platform.OS === "ios" ? "padding" : "height"}
      >
        <Screen transparent scroll contentStyle={styles.isi}>
          <View style={styles.header}>
            <Pressable
              onPress={kembali}
              accessibilityRole="button"
              accessibilityLabel="Kembali"
              hitSlop={touch.hitSlop}
              style={({ pressed }) => [
                styles.tombolKembali,
                { backgroundColor: colors.surfaceMuted, borderColor: colors.border },
                pressed && styles.ditekan,
              ]}
            >
              <Ionicons name="chevron-back" size={20} color={colors.textPrimary} />
            </Pressable>

            <View style={styles.merek}>
              <BrandLogo size={layout.logoSizeBar} showWordmark={false} />
              <AppText variant="bodyLarge" weight="bold">
                FaceLink
              </AppText>
              <AppText variant="bodyLarge" weight="bold" color={colors.accent}>
                Ai
              </AppText>
            </View>

            <AppText variant="micro" weight="semibold" color={colors.textMuted} style={styles.langkah}>
              {otp ? "LANGKAH 2/2" : "LANGKAH 1/2"}
            </AppText>
          </View>

          {/* Mendorong blok formulir ke bawah; header tetap di atas. */}
          <View style={styles.pendorong} />

          <View style={styles.pembuka}>
            <View
              style={[
                styles.ikon,
                { backgroundColor: colors.surface, borderColor: colors.border },
              ]}
            >
              {otp ? (
                <Ionicons name="shield-checkmark-outline" size={30} color={colors.accent} />
              ) : (
                <MailIcon size={30} color={colors.accent} />
              )}
            </View>
            <AppText variant="heading" weight="bold" center>
              {otp ? "Periksa email Anda" : "Masuk dengan FaceLink"}
            </AppText>
            <AppText center color={colors.textSecondary} style={styles.deskripsi}>
              {otp
                ? "Kami mengirim kode 8 digit jika akun ini memenuhi syarat. Periksa kotak masuk dan folder spam."
                : "Masukkan email yang telah diundang admin. Role dan akses akun ditentukan server FaceLink AI."}
            </AppText>
          </View>

          <View style={styles.form}>
            {otp ? (
              <>
                <View
                  style={[
                    styles.emailChip,
                    { backgroundColor: colors.surface, borderColor: colors.border },
                  ]}
                >
                  <Ionicons name="mail-outline" size={16} color={colors.textMuted} />
                  <AppText
                    variant="caption"
                    color={colors.textSecondary}
                    numberOfLines={1}
                    style={styles.emailTeks}
                  >
                    {state.email}
                  </AppText>
                  <Pressable
                    onPress={kembali}
                    accessibilityRole="button"
                    accessibilityLabel="Ubah email"
                    hitSlop={touch.hitSlop}
                    style={({ pressed }) => [styles.ubah, pressed && styles.ditekan]}
                  >
                    <AppText variant="caption" weight="semibold" color={colors.accent}>
                      Ubah
                    </AppText>
                  </Pressable>
                </View>

                <View style={styles.labelBaris}>
                  <AppText weight="semibold">Kode verifikasi</AppText>
                  <AppText
                    variant="caption"
                    color={expires > 0 ? colors.textSecondary : colors.error}
                  >
                    {expires > 0
                      ? `Berlaku ${Math.floor(expires / 60)}:${String(expires % 60).padStart(2, "0")}`
                      : "Kode kedaluwarsa"}
                  </AppText>
                </View>

                <TextInput
                  key="otp"
                  value={code}
                  onChangeText={(value) => setCode(value.replace(/\D/g, "").slice(0, 8))}
                  accessibilityLabel="Kode verifikasi 8 digit"
                  placeholder="00000000"
                  placeholderTextColor={colors.textMuted}
                  keyboardType="number-pad"
                  textContentType="oneTimeCode"
                  autoComplete="one-time-code"
                  importantForAutofill="yes"
                  autoCorrect={false}
                  autoFocus
                  maxLength={8}
                  editable={!busy}
                  selectionColor={colors.accent}
                  style={[inputStyle, styles.kode]}
                  onSubmitEditing={() => {
                    if (!busy && code.length === 8 && expires > 0 && verifyRetry === 0) {
                      verifikasi();
                    }
                  }}
                />

                <AppButton
                  label={
                    verifyRetry > 0
                      ? `Coba verifikasi dalam ${verifyRetry} dtk`
                      : "Verifikasi dan masuk"
                  }
                  onPress={verifikasi}
                  loading={state.busy === "verify"}
                  disabled={busy || code.length !== 8 || expires === 0 || verifyRetry > 0}
                />
                <AppButton
                  label={retry > 0 ? `Kirim ulang dalam ${retry} dtk` : "Kirim ulang kode"}
                  variant="secondary"
                  onPress={kirim}
                  loading={state.busy === "request"}
                  disabled={busy || retry > 0}
                />
                <AppText variant="caption" center color={colors.textMuted}>
                  Kode hanya dapat dipakai sekali. Email dapat terlambat tiba; kirim ulang
                  setelah hitung mundur selesai.
                </AppText>
              </>
            ) : (
              <>
                <AppText weight="semibold">Alamat email</AppText>
                <TextInput
                  key="email"
                  value={email}
                  onChangeText={setEmail}
                  accessibilityLabel="Alamat email"
                  placeholder="nama@facelink.online"
                  placeholderTextColor={colors.textMuted}
                  keyboardType="email-address"
                  textContentType="emailAddress"
                  autoComplete="email"
                  autoCapitalize="none"
                  autoCorrect={false}
                  autoFocus
                  maxLength={254}
                  editable={!busy}
                  returnKeyType="send"
                  selectionColor={colors.accent}
                  style={inputStyle}
                  onSubmitEditing={() => {
                    if (emailValid && !busy && retry === 0) kirim();
                  }}
                />
                <AppText variant="caption" color={colors.textMuted}>
                  Gunakan alamat yang sudah didaftarkan admin, misalnya
                  nama@facelink.online.
                </AppText>
                <AppButton
                  label={retry > 0 ? `Coba lagi dalam ${retry} dtk` : "Kirim kode verifikasi"}
                  onPress={kirim}
                  loading={state.busy === "request"}
                  disabled={!emailValid || busy || retry > 0 || status === "memulihkan"}
                />
              </>
            )}

            {state.error ? (
              <View
                accessibilityLiveRegion="polite"
                style={[
                  styles.galat,
                  { backgroundColor: colors.surface, borderColor: colors.border },
                ]}
              >
                <Ionicons name="alert-circle-outline" size={18} color={colors.error} />
                <AppText color={colors.error} style={styles.galatTeks}>
                  {pesanEmail(state.error)}
                </AppText>
              </View>
            ) : null}
          </View>
        </Screen>
      </KeyboardAvoidingView>
    </View>
  );
}

const styles = StyleSheet.create({
  layar: { flex: 1 },
  isi: {
    paddingTop: spacing.sm,
    paddingBottom: spacing.xxl,
    gap: spacing.xl,
  },
  header: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.md,
  },
  tombolKembali: {
    width: 40,
    height: 40,
    borderRadius: radius.pill,
    borderWidth: StyleSheet.hairlineWidth,
    alignItems: "center",
    justifyContent: "center",
  },
  merek: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.xs,
    flex: 1,
  },
  langkah: {
    letterSpacing: typography.tracking.caps,
  },
  pendorong: {
    flex: 1,
    minHeight: spacing.xl,
  },
  pembuka: {
    alignItems: "center",
    gap: spacing.md,
  },
  ikon: {
    width: 64,
    height: 64,
    borderRadius: radius.xl,
    borderWidth: StyleSheet.hairlineWidth,
    alignItems: "center",
    justifyContent: "center",
  },
  deskripsi: {
    maxWidth: 420,
  },
  form: {
    gap: spacing.lg,
    width: "100%",
  },
  emailChip: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.sm,
    minHeight: touch.minHeight,
    borderRadius: radius.md,
    borderWidth: StyleSheet.hairlineWidth,
    paddingHorizontal: spacing.lg,
  },
  emailTeks: {
    flex: 1,
  },
  ubah: {
    minHeight: touch.minHeight,
    justifyContent: "center",
    paddingLeft: spacing.sm,
  },
  labelBaris: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    gap: spacing.md,
  },
  input: {
    minHeight: touch.minHeight,
    borderWidth: 1,
    borderRadius: radius.lg,
    paddingHorizontal: spacing.lg,
    paddingVertical: spacing.md,
    fontFamily: typography.family.regular,
    fontSize: typography.size.bodyLarge,
  },
  kode: {
    fontFamily: typography.family.semibold,
    fontSize: typography.size.heading,
    letterSpacing: 6,
    textAlign: "center",
  },
  galat: {
    flexDirection: "row",
    alignItems: "flex-start",
    gap: spacing.sm,
    borderRadius: radius.md,
    borderWidth: StyleSheet.hairlineWidth,
    paddingHorizontal: spacing.lg,
    paddingVertical: spacing.md,
  },
  galatTeks: {
    flex: 1,
  },
  ditekan: {
    opacity: 0.7,
  },
});

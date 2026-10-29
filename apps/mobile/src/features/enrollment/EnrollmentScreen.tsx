import React, { useCallback, useEffect, useRef, useState } from "react";
import {
  AccessibilityInfo,
  ActivityIndicator,
  Alert,
  AppState,
  BackHandler,
  Pressable,
  StyleSheet,
  View,
} from "react-native";
import { Redirect, useFocusEffect, useRouter } from "expo-router";
import { Ionicons } from "@expo/vector-icons";

import { AppButton } from "@/components/AppButton";
import { AppText } from "@/components/AppText";
import { Screen } from "@/components/Screen";
import { StateView } from "@/components/StateView";
import { StepTracker } from "@/components/StepTracker";
import { useSession } from "@/features/auth/SessionProvider";
import { ruteAlihRole } from "@/features/auth/roleRoutes";
import { radius, spacing, touch, useTheme } from "@/theme";
import {
  beriConsent,
  bacaConsent,
  bacaStatus,
  kirimFotoSintetis,
  tarikConsent,
} from "./api";
import { EnrollmentFlow, detikTersisa, type EnrollmentState } from "./flow";
import { captureMode, captureModeLabel } from "./captureMode";
import { NativeScanner } from "./nativeScanner";
import {
  labelEnrollment,
  labelKonsen,
  labelStatusTerakhir,
  pesanActiveTidakTerpercaya,
  pesanEnrollment,
  ringkasanStatusAksesibel,
  type ConsentView,
  type EnrollmentError,
  type EnrollmentView,
} from "./model";
import type { SyntheticCaptureState } from "./syntheticCapture";

const apiNyata = {
  readConsent: bacaConsent,
  readStatus: bacaStatus,
  grantConsent: beriConsent,
  withdrawConsent: tarikConsent,
  submitJPEG: kirimFotoSintetis,
};

function alasanAman(view: EnrollmentView | null): string {
  switch (view?.reasonCode) {
    case "IDENTITY_MISMATCH":
      return "Identitas pada pengajuan tidak cocok menurut pemeriksaan kampus.";
    case "EVIDENCE_INSUFFICIENT":
      return "Bukti yang tersedia belum cukup untuk verifikasi.";
    case "UNSUITABLE_SUBMISSION":
      return "Pengajuan tidak layak diproses. Ajukan ulang setelah membaca panduan.";
    case "PHOTO_RETENTION_EXPIRED":
      return "Batas simpan foto 72 jam telah lewat. Foto sudah dihapus.";
    case "NO_FACE":
      return "Tidak ada wajah yang dapat diproses pada bingkai sintetis.";
    case "MULTIPLE_FACES":
      return "Bingkai berisi lebih dari satu wajah dan tidak diproses.";
    case "INVALID_EMBEDDING":
    case "EXTRACTION_FAILED":
      return "Pemrosesan teknis gagal; tidak ada template yang dibuat.";
    case "REVIEWER_INELIGIBLE":
      return "Peninjau tidak lagi berwenang. Pengajuan tidak dapat diaktifkan.";
    case "BIOMETRIC_UNAVAILABLE":
      return "Layanan kunci sintetis tidak tersedia. Data foto tidak dihapus otomatis.";
    case "CONSENT_WITHDRAWN":
      return "Persetujuan telah ditarik.";
    default:
      return "Pengajuan tidak selesai. Tidak ada status aktif yang diklaim.";
  }
}

function formatSisa(detik: number | null): string {
  if (detik === null) return "—";
  const jam = Math.floor(detik / 3600);
  const menit = Math.floor((detik % 3600) / 60);
  const sisa = detik % 60;
  return `${jam}j ${String(menit).padStart(2, "0")}m ${String(sisa).padStart(2, "0")}d`;
}

function StatusBadges({
  consent,
  enrollment,
  trusted,
}: {
  consent: ConsentView | null;
  enrollment: EnrollmentView | null;
  trusted: boolean;
}) {
  const { colors } = useTheme();
  const consentTone =
    consent?.state === "ACTIVE"
      ? colors.success
      : consent?.state === "WITHDRAWN"
        ? colors.warning
        : colors.textMuted;
  const enrollmentLabel =
    enrollment?.effectiveStatus === "ACTIVE" && !trusted
      ? "Aktif (belum terverifikasi)"
      : labelEnrollment(enrollment);
  return (
    <View style={styles.badges}>
      <View
        accessible
        accessibilityRole="text"
        accessibilityLabel={`Persetujuan: ${labelKonsen(consent?.state ?? null)}`}
        style={[styles.badge, { borderColor: colors.border, backgroundColor: colors.surface }]}
      >
        <View style={[styles.dot, { backgroundColor: consentTone }]} />
        <AppText variant="caption" color={colors.textPrimary} weight="semibold">
          {labelKonsen(consent?.state ?? null)}
        </AppText>
      </View>
      <View
        accessible
        accessibilityRole="text"
        accessibilityLabel={`Status pendaftaran: ${enrollmentLabel}`}
        style={[styles.badge, { borderColor: colors.border, backgroundColor: colors.surface }]}
      >
        <Ionicons name="person-outline" size={12} color={colors.textMuted} />
        <AppText variant="caption" color={colors.textMuted} weight="semibold">
          {enrollmentLabel}
        </AppText>
      </View>
    </View>
  );
}

function Header({ onBack }: { onBack: () => void }) {
  const { colors } = useTheme();
  return (
    <View style={styles.header}>
      <Pressable
        onPress={onBack}
        accessibilityRole="button"
        accessibilityLabel="Kembali"
        hitSlop={touch.hitSlop}
        style={({ pressed }) => [
          styles.back,
          { borderColor: colors.border, backgroundColor: colors.surface },
          pressed && { opacity: 0.7 },
        ]}
      >
        <Ionicons name="chevron-back" size={20} color={colors.textPrimary} />
      </Pressable>
      <View style={styles.headerText}>
        <AppText variant="micro" weight="semibold" color={colors.textMuted}>
          STAGING · {captureModeLabel}
        </AppText>
        <AppText variant="title" weight="bold" accessibilityRole="header">
          Verifikasi wajah
        </AppText>
      </View>
      <View
        style={[styles.stageMark, { backgroundColor: colors.accentGlow }]}
        accessible={false}
        accessibilityElementsHidden
        importantForAccessibility="no-hide-descendants"
      >
        <Ionicons name="flask-outline" size={18} color={colors.accent} />
      </View>
    </View>
  );
}

function ErrorBanner({
  error,
  onRetry,
}: {
  error: EnrollmentError;
  onRetry: () => void;
}) {
  const { colors } = useTheme();
  return (
    <View style={[styles.error, { borderColor: colors.error, backgroundColor: colors.accentSurface }]}>
      <Ionicons name="alert-circle-outline" size={18} color={colors.error} />
      <AppText variant="caption" color={colors.textPrimary} style={styles.errorText}>
        {pesanEnrollment(error)}
      </AppText>
      <Pressable
        onPress={onRetry}
        accessibilityRole="button"
        accessibilityLabel="Coba lagi"
        hitSlop={touch.hitSlop}
        style={styles.retry}
      >
        <AppText variant="caption" weight="semibold" color={colors.accent}>
          Coba lagi
        </AppText>
      </Pressable>
    </View>
  );
}

function IntroPanel({ flow, state }: { flow: EnrollmentFlow; state: EnrollmentState }) {
  const { colors } = useTheme();
  const trusted = flow.trustedActive();
  const mannequin = captureMode === "MANNEQUIN_PREVIEW_ONLY";
  return (
    <View style={styles.stack}>
      <View style={[styles.card, { backgroundColor: colors.surface, borderColor: colors.border }]}>
        <View style={[styles.heroIcon, { backgroundColor: colors.accentGlow }]}>
          <Ionicons name="scan-outline" size={30} color={colors.accent} />
        </View>
        <AppText variant="heading" weight="bold" center accessibilityRole="header">
          Kenali wajah Anda sekali
        </AppText>
        <AppText variant="body" color={colors.textMuted} center>
          {mannequin
            ? "Uji preview kamera depan dengan mannequin saja. Capture diproses lokal oleh modul native; tidak dikirim untuk review Admin dan tidak mengaktifkan pendaftaran. Absensi manual tetap tersedia."
            : "Verifikasi identitas kampus dengan satu bingkai. Admin akan meninjau pengajuan sebelum status dapat menjadi aktif. Absensi manual tetap tersedia kapan saja."}
        </AppText>
      </View>

      <View style={styles.poin}>
        {[
          ["shield-checkmark-outline", "Persetujuan eksplisit", "Anda dapat menarik persetujuan kapan saja."],
          mannequin
            ? ["time-outline", "Tanpa simpan atau upload", "Hasil capture hanya dikonsumsi modul native, bukan pengajuan enrollment."]
            : ["time-outline", "Batas simpan 72 jam", "Foto pending dihapus atau kedaluwarsa."],
          mannequin
            ? ["lock-closed-outline", "Preview kamera lokal", "Kamera depan hanya menyala setelah tindakan eksplisit. Gunakan mannequin, bukan wajah manusia."]
            : ["lock-closed-outline", "Tidak ada kamera nyata", "Alur ini memakai bingkai sintetis untuk staging."],
        ].map(([ikon, judul, isi]) => (
          <View key={judul} style={styles.poinBaris}>
            <Ionicons name={ikon as keyof typeof Ionicons.glyphMap} size={18} color={colors.accent} />
            <View style={styles.poinTeks}>
              <AppText variant="body" weight="semibold">
                {judul}
              </AppText>
              <AppText variant="caption" color={colors.textMuted}>
                {isi}
              </AppText>
            </View>
          </View>
        ))}
      </View>

      <AppButton
        label={state.consent?.state === "ACTIVE" ? "Lanjutkan ke pemindaian" : "Verifikasi sekarang"}
        onPress={() => (state.consent?.state === "ACTIVE" ? flow.siapkan() : flow.masukConsent())}
      />
      <AppText variant="caption" color={colors.textMuted} center>
        {state.stale
          ? `Status terakhir yang diverifikasi: ${labelEnrollment(state.enrollment)}. Status saat ini belum dapat diverifikasi; jangan anggap wajah terdaftar sampai pemeriksaan berhasil.`
          : `Status terakhir: ${labelStatusTerakhir(
              state.enrollment,
              state.stale,
              trusted
            )}. Status ini berasal dari server, bukan dari pilihan di layar.`}
      </AppText>
    </View>
  );
}

function ConsentPanel({
  flow,
  consent,
  busy,
}: {
  flow: EnrollmentFlow;
  consent: ConsentView;
  busy: boolean;
}) {
  const { colors } = useTheme();
  const [setuju, setSetuju] = useState(false);
  return (
    <View style={styles.stack}>
      <View style={styles.langkah}>
        <StepTracker
          langkah={[
            { label: "Consent", ikon: "document-text-outline", status: "berjalan" },
            { label: "Scan", ikon: "scan-outline", status: "menunggu" },
            { label: "Review", ikon: "people-outline", status: "menunggu" },
            { label: "Selesai", ikon: "checkmark-circle-outline", status: "menunggu" },
          ]}
        />
      </View>
      <View style={[styles.card, { backgroundColor: colors.surface, borderColor: colors.border }]}>
        <View style={styles.cardJudul}>
          <Ionicons name="document-text-outline" size={20} color={colors.accent} />
          <AppText variant="title" weight="semibold" accessibilityRole="header">
            Informasi & persetujuan
          </AppText>
        </View>
        <AppText variant="caption" color={colors.textMuted}>
          Versi pemberitahuan: {consent.notice.version}
        </AppText>
        <View style={[styles.noticeBox, { borderColor: colors.border }]}>
          <AppText variant="body" color={colors.textSecondary}>
            {consent.notice.text}
          </AppText>
        </View>
        <Pressable
          onPress={() => setSetuju((nilai) => !nilai)}
          accessibilityRole="checkbox"
          accessibilityState={{ checked: setuju }}
          accessibilityLabel="Saya telah membaca dan menyetujui pemrosesan satu foto sintetis"
          accessibilityHint="Centang untuk mengaktifkan tombol Setujui dan lanjutkan"
          hitSlop={touch.hitSlop}
          style={styles.checkboxBaris}
        >
          <View
            style={[
              styles.checkbox,
              { borderColor: setuju ? colors.accent : colors.border },
              setuju && { backgroundColor: colors.accent },
            ]}
          >
            {setuju ? <Ionicons name="checkmark" size={14} color={colors.onAccent} /> : null}
          </View>
          <AppText variant="body" style={styles.checkboxTeks}>
            Saya telah membaca pemberitahuan ini dan menyetujui pemrosesan satu bingkai sintetis
            untuk verifikasi identitas kampus.
          </AppText>
        </Pressable>
        <AppButton
          label="Setujui & lanjutkan"
          onPress={() => void flow.beriConsent()}
          loading={busy}
          disabled={!setuju}
        />
        <AppButton label="Kembali" variant="secondary" onPress={() => flow.batalConsent()} />
      </View>
      <AppText variant="caption" color={colors.textMuted} center>
        Persetujuan ini disimpan oleh server dengan versi dan digest pemberitahuan. Persetujuan
        bukan tanda tangan kriptografis dan bukan bukti identitas.
      </AppText>
    </View>
  );
}

function PreparePanel({ flow, mode }: { flow: EnrollmentFlow; mode: typeof captureMode }) {
  const { colors } = useTheme();
  const mannequin = mode === "MANNEQUIN_PREVIEW_ONLY";
  return (
    <View style={styles.stack}>
      <View style={styles.langkah}>
        <StepTracker
          langkah={[
            { label: "Consent", ikon: "checkmark", status: "selesai" },
            { label: "Scan", ikon: "scan-outline", status: "berjalan" },
            { label: "Review", ikon: "people-outline", status: "menunggu" },
            { label: "Selesai", ikon: "checkmark-circle-outline", status: "menunggu" },
          ]}
        />
      </View>
      <View style={[styles.card, { backgroundColor: colors.surface, borderColor: colors.border }]}>
        <View style={[styles.heroIcon, { backgroundColor: colors.accentGlow }]}>
          <Ionicons name="sunny-outline" size={28} color={colors.accent} />
        </View>
        <AppText variant="title" weight="semibold" center accessibilityRole="header">
          Persiapkan pemindaian
        </AppText>
        <AppText variant="body" color={colors.textMuted} center>
          {mannequin
            ? "Mode mannequin preview. Kamera hanya dapat dinyalakan setelah Anda menekan tombol di layar berikutnya. Panduan wajah aktif, tetapi hasil capture tidak diunggah, tidak disimpan, dan tidak membuat enrollment."
            : "Hadapkan wajah ke kamera dalam posisi portrait. Pastikan pencahayaan cukup dan wajah berada di dalam oval. Di staging ini, kamera tidak dibuka dan bingkai dihasilkan secara sintetis."}
        </AppText>
        <View style={styles.panduanGrid}>
          {[
            ["scan-outline", "Satu wajah", "Tidak ada orang lain di bingkai."],
            ["glasses-outline", "Tanpa aksesori", "Lepas masker atau penutup wajah."],
            ["phone-portrait-outline", "Portrait", "Jangan memutar perangkat."],
            ["hand-left-outline", "Tenang", "Tahan posisi hingga selesai."],
          ].map(([ikon, judul, isi]) => (
            <View key={judul} style={[styles.panduan, { borderColor: colors.border }]}>
              <Ionicons name={ikon as keyof typeof Ionicons.glyphMap} size={18} color={colors.accent} />
              <AppText variant="caption" weight="semibold">
                {judul}
              </AppText>
              <AppText variant="micro" color={colors.textMuted}>
                {isi}
              </AppText>
            </View>
          ))}
        </View>
        <AppButton
          label={mannequin ? "Lanjut ke kamera" : "Mulai pemindaian sintetis"}
          onPress={() => flow.mulaiScan()}
        />
        <AppButton label="Batal" variant="secondary" onPress={() => flow.batalConsent()} />
      </View>
    </View>
  );
}

function ScanPanel({
  flow,
  capture,
}: {
  flow: EnrollmentFlow;
  capture: SyntheticCaptureState | null;
}) {
  const { colors } = useTheme();
  const progress = capture?.progress ?? 0;
  const menuju = Math.round(progress * 100);
  return (
    <View style={styles.stack}>
      <View style={styles.langkah}>
        <StepTracker
          langkah={[
            { label: "Consent", ikon: "checkmark", status: "selesai" },
            { label: "Scan", ikon: "scan-outline", status: "berjalan" },
            { label: "Review", ikon: "people-outline", status: "menunggu" },
            { label: "Selesai", ikon: "checkmark-circle-outline", status: "menunggu" },
          ]}
        />
      </View>
      <View style={[styles.scanCard, { backgroundColor: colors.surface, borderColor: colors.border }]}>
        <View style={styles.scanTop}>
          <View style={[styles.syntheticBadge, { backgroundColor: colors.accentGlow }]}>
            <Ionicons name="flask-outline" size={13} color={colors.accent} />
            <AppText variant="micro" weight="semibold" color={colors.accent}>
              SIMULASI · TANPA KAMERA
            </AppText>
          </View>
          <AppText variant="caption" color={colors.textMuted}>
            {menuju}%
          </AppText>
        </View>
        <View
          style={[styles.oval, { borderColor: capture?.phase === "holding" ? colors.accent : colors.accentDim }]}
          accessibilityRole="image"
          accessibilityLabel="Oval panduan wajah sintetis"
        >
          <View style={[styles.ovalInner, { borderColor: colors.border }]} />
          <View style={[styles.scanLine, { top: `${18 + progress * 64}%`, backgroundColor: colors.accent }]} />
          <Ionicons name="person-outline" size={72} color={colors.hudLine} />
        </View>
        <AppText
          variant="bodyLarge"
          weight="semibold"
          center
          accessibilityLiveRegion="polite"
        >
          {capture?.instruction ?? "Menyiapkan pemindaian…"}
        </AppText>
        <View
          style={[styles.progressTrack, { backgroundColor: colors.chartTrack }]}
          accessibilityRole="progressbar"
          accessibilityLabel="Kemajuan pemindaian sintetis"
          accessibilityValue={{ min: 0, max: 100, now: menuju }}
        >
          <View style={[styles.progressFill, { width: `${menuju}%`, backgroundColor: colors.accent }]} />
        </View>
        <AppText variant="caption" color={colors.textMuted} center>
          {capture?.accessibility ?? "Panduan pemindaian sintetis."}
        </AppText>
        <AppButton label="Batalkan pemindaian" variant="secondary" onPress={() => flow.batalScan()} />
      </View>
      <AppText variant="caption" color={colors.textMuted} center>
        Tidak ada permission kamera yang diminta dan tidak ada gambar wajah manusia yang dibuat.
        Bingkai sintetis tidak ditampilkan atau disimpan.
      </AppText>
    </View>
  );
}

function UploadingPanel() {
  const { colors } = useTheme();
  return (
    <View
      accessible
      accessibilityRole="progressbar"
      accessibilityLabel="Mengirim bingkai sintetis. Tunggu hingga selesai."
      accessibilityState={{ busy: true }}
      style={[styles.card, { backgroundColor: colors.surface, borderColor: colors.border }]}
    >
      <ActivityIndicator color={colors.accent} />
      <AppText variant="title" weight="semibold" center accessibilityRole="header">
        Mengirim bingkai sintetis
      </AppText>
      <AppText variant="body" color={colors.textMuted} center>
        Foto kanonik dienkripsi oleh backend setelah diterima. Jangan menutup aplikasi sebelum
        proses selesai.
      </AppText>
    </View>
  );
}

function PendingPanel({
  flow,
  state,
  now,
}: {
  flow: EnrollmentFlow;
  state: EnrollmentState;
  now: number | null;
}) {
  const { colors } = useTheme();
  const sisa =
    now === null ? null : detikTersisa(state.enrollment?.photoExpiresAt ?? null, now);
  return (
    <View style={styles.stack}>
      <View style={styles.langkah}>
        <StepTracker
          langkah={[
            { label: "Consent", ikon: "checkmark", status: "selesai" },
            { label: "Scan", ikon: "checkmark", status: "selesai" },
            { label: "Review", ikon: "people-outline", status: "berjalan" },
            { label: "Selesai", ikon: "checkmark-circle-outline", status: "menunggu" },
          ]}
        />
      </View>
      <View style={[styles.card, { backgroundColor: colors.surface, borderColor: colors.border }]}>
        <View style={[styles.heroIcon, { backgroundColor: colors.accentGlow }]}>
          <Ionicons name="people-outline" size={28} color={colors.accent} />
        </View>
        <AppText variant="title" weight="semibold" center accessibilityRole="header">
          Menunggu verifikasi Admin
        </AppText>
        <AppText variant="body" color={colors.textMuted} center>
          Pengajuan sudah diterima. Status belum aktif sampai Admin meninjau identitas dan backend
          menyatakan template siap.
        </AppText>
        <View style={[styles.infoRow, { borderColor: colors.border }]}>
          <AppText variant="caption" color={colors.textMuted}>
            Sisa masa simpan foto
          </AppText>
          <AppText variant="body" weight="semibold">
            {formatSisa(sisa)}
          </AppText>
        </View>
        <AppText variant="caption" color={colors.textMuted} center>
          {state.stale
            ? "Status terakhir: Menunggu verifikasi Admin. Status saat ini belum dapat diverifikasi; jangan menganggap pengajuan aktif."
            : "Status ini berasal dari backend dan akan diperbarui berkala."}
        </AppText>
        <AppButton label="Periksa status" variant="secondary" onPress={() => void flow.refresh()} loading={state.busy} />
        {state.consent?.state === "ACTIVE" ? (
          <AppButton label="Tarik persetujuan" variant="secondary" onPress={() => konfirmasiTarik(flow)} />
        ) : null}
      </View>
    </View>
  );
}

function ActivePanel({ flow, state }: { flow: EnrollmentFlow; state: EnrollmentState }) {
  const { colors } = useTheme();
  const terpercaya = flow.trustedActive();
  return (
    <View style={styles.stack}>
      <View style={styles.langkah}>
        <StepTracker
          langkah={[
            { label: "Consent", ikon: "checkmark", status: "selesai" },
            { label: "Scan", ikon: "checkmark", status: "selesai" },
            { label: "Review", ikon: "checkmark", status: "selesai" },
            { label: "Selesai", ikon: "checkmark-circle-outline", status: terpercaya ? "selesai" : "berjalan" },
          ]}
        />
      </View>
      <View
        style={[
          styles.card,
          {
            backgroundColor: terpercaya ? colors.surface : colors.accentSurface,
            borderColor: terpercaya ? colors.border : colors.warning,
          },
        ]}
      >
        <View
          style={[
            styles.heroIcon,
            { backgroundColor: terpercaya ? colors.accentGlow : colors.surfaceMuted },
          ]}
        >
          <Ionicons
            name={terpercaya ? "checkmark-circle-outline" : "alert-circle-outline"}
            size={30}
            color={terpercaya ? colors.success : colors.warning}
          />
        </View>
        <AppText variant="title" weight="semibold" center accessibilityRole="header">
          {terpercaya ? "Wajah terdaftar" : "Status aktif belum terverifikasi"}
        </AppText>
        <AppText variant="body" color={colors.textMuted} center>
          {terpercaya
            ? "Backend menyatakan template siap dan persetujuan masih aktif. Status ini bukan jaminan akurasi pengenalan."
            : pesanActiveTidakTerpercaya(state.enrollment, state.consent, state.stale)}
        </AppText>
        <AppButton label="Periksa status ulang" variant="secondary" onPress={() => void flow.refresh()} loading={state.busy} />
        {state.consent?.state === "ACTIVE" ? (
          <AppButton label="Tarik persetujuan" variant="secondary" onPress={() => konfirmasiTarik(flow)} />
        ) : null}
      </View>
      <AppText variant="caption" color={colors.textMuted} center>
        “Wajah terdaftar” hanya ditampilkan dari effective_status ACTIVE, template_ready, consent
        ACTIVE, dan status yang baru dimuat.
      </AppText>
    </View>
  );
}

function ResultPanel({ flow, state }: { flow: EnrollmentFlow; state: EnrollmentState }) {
  const { colors } = useTheme();
  const view = state.enrollment;
  const phase = state.phase;
  const ikon =
    phase === "withdrawn"
      ? "log-out-outline"
      : phase === "expired"
        ? "time-outline"
        : phase === "failed"
          ? "warning-outline"
          : "close-circle-outline";
  const judul =
    phase === "withdrawn"
      ? "Persetujuan ditarik"
      : phase === "expired"
        ? "Pengajuan kedaluwarsa"
        : phase === "failed"
          ? "Pemrosesan tidak berhasil"
          : "Pengajuan ditolak";
  const bisaScan = flow.allowed("SUBMIT") && state.consent?.state === "ACTIVE";
  const bisaGrant = !bisaScan && flow.allowed("GRANT_CONSENT");
  const labelAksi = bisaScan
    ? "Mulai pemindaian"
    : bisaGrant
      ? "Mulai dengan persetujuan baru"
      : "Muat ulang status";
  const aksi = () => {
    if (bisaScan) flow.siapkan();
    else if (bisaGrant) flow.masukConsent();
    else void flow.load();
  };
  return (
    <View style={styles.stack}>
      <View style={[styles.card, { backgroundColor: colors.surface, borderColor: colors.border }]}>
        <View style={[styles.heroIcon, { backgroundColor: colors.accentGlow }]}>
          <Ionicons name={ikon as keyof typeof Ionicons.glyphMap} size={30} color={colors.accent} />
        </View>
        <AppText variant="title" weight="semibold" center accessibilityRole="header">
          {judul}
        </AppText>
        <AppText variant="body" color={colors.textMuted} center>
          {phase === "withdrawn"
            ? "Persetujuan telah ditarik. Pemindaian dihentikan dan data yang tersimpan diproses untuk dihapus; status aktif tidak ditampilkan."
            : alasanAman(view)}
        </AppText>
        {view?.reasonCode ? (
          <AppText variant="micro" color={colors.textMuted} center>
            Kode aman: {view.reasonCode}
          </AppText>
        ) : null}
        <AppButton label={labelAksi} onPress={aksi} />
        {state.consent?.state === "ACTIVE" ? (
          <AppButton label="Tarik persetujuan" variant="secondary" onPress={() => konfirmasiTarik(flow)} />
        ) : null}
      </View>
    </View>
  );
}

function konfirmasiTarik(flow: EnrollmentFlow): void {
  Alert.alert(
    "Tarik persetujuan?",
    "Pemindaian dan pengajuan akan dihentikan. Template dan foto yang tersimpan akan diproses untuk dihapus.",
    [
      { text: "Batal", style: "cancel" },
      { text: "Tarik", style: "destructive", onPress: () => void flow.tarik() },
    ]
  );
}

export default function EnrollmentScreen() {
  const { colors } = useTheme();
  const router = useRouter();
  const { session, getAccessToken } = useSession();
  const [flow] = useState(
    () => new EnrollmentFlow(apiNyata, async () => null)
  );
  const [state, setState] = useState<EnrollmentState>(flow.keadaan());
  const [now, setNow] = useState(Date.now);

  useEffect(() => {
    flow.setTokenProvider(getAccessToken);
    const berhenti = flow.subscribe(setState);
    void flow.load();
    return () => {
      berhenti();
      flow.dispose();
    };
  }, [flow, getAccessToken]);

  useEffect(() => {
    if (state.phase !== "pending" && state.phase !== "active") return;
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, [state.phase]);

  useEffect(() => {
    if (captureMode !== "SYNTHETIC_FLOW" || state.phase !== "scan") return;
    const timer = setInterval(() => flow.tickScan(), 100);
    return () => clearInterval(timer);
  }, [flow, state.phase]);

  useEffect(() => {
    if (state.phase !== "pending" && state.phase !== "processing") return;
    const timer = setInterval(() => void flow.refresh(false), 5000);
    return () => clearInterval(timer);
  }, [flow, state.phase]);

  const pengumumanTerakhir = useRef<string | null>(null);
  useEffect(() => {
    if (state.phase === "loading") return;
    const ringkas = ringkasanStatusAksesibel(
      state.phase,
      state.enrollment,
      state.consent,
      flow.trustedActive()
    );
    if (pengumumanTerakhir.current === ringkas) return;
    pengumumanTerakhir.current = ringkas;
    AccessibilityInfo.announceForAccessibility(ringkas);
  }, [flow, now, state.consent, state.enrollment, state.phase, state.stale]);

  useFocusEffect(
    useCallback(() => {
      const appState = AppState.addEventListener("change", (next) => {
        if (next === "active") {
          setNow(Date.now());
          void flow.refresh(false);
        } else {
          flow.suspend();
        }
      });
      const back = BackHandler.addEventListener("hardwareBackPress", () => {
        if (flow.keadaan().phase === "scan") {
          flow.batalScan();
          return true;
        }
        if (router.canGoBack()) router.back();
        else router.replace("/mahasiswa");
        return true;
      });
      return () => {
        appState.remove();
        back.remove();
        flow.suspend();
      };
    }, [flow, router])
  );

  const alih = ruteAlihRole(session?.user.role ?? null, "mahasiswa");
  if (alih !== null) return <Redirect href={alih} />;
  if (!session) return null;

  const onBack = () => {
    if (state.phase === "scan") flow.batalScan();
    else if (router.canGoBack()) router.back();
    else router.replace("/mahasiswa");
  };

  let isi: React.ReactNode;
  if (state.phase === "loading" && state.consent === null && state.enrollment === null) {
    isi = (
      <StateView
        tone="loading"
        title="Memuat status verifikasi"
        description="Membaca persetujuan dan status dari backend sintetis."
      />
    );
  } else if (state.phase === "error") {
    isi = (
      <StateView
        tone="error"
        title="Status tidak dapat dimuat"
        description={state.error ? pesanEnrollment(state.error) : "Coba lagi setelah koneksi pulih."}
        action={<AppButton label="Coba lagi" onPress={() => void flow.load()} />}
      />
    );
  } else if (state.phase === "consent" && state.consent !== null) {
    isi = <ConsentPanel key={state.consent.notice.digest} flow={flow} consent={state.consent} busy={state.busy} />;
  } else if (state.phase === "scan") {
    isi =
      captureMode === "MANNEQUIN_PREVIEW_ONLY" ? (
        <NativeScanner
          onCancel={() => flow.batalScan()}
          onConsumed={() => {}}
          onError={() => {}}
        />
      ) : (
        <ScanPanel flow={flow} capture={state.capture} />
      );
  } else if (state.phase === "uploading") {
    isi = <UploadingPanel />;
  } else if (state.phase === "pending" || state.phase === "processing") {
    isi = <PendingPanel flow={flow} state={state} now={flow.serverNow()} />;
  } else if (state.phase === "active") {
    isi = <ActivePanel flow={flow} state={state} />;
  } else if (
    state.phase === "rejected" ||
    state.phase === "expired" ||
    state.phase === "failed" ||
    state.phase === "withdrawn"
  ) {
    isi = <ResultPanel flow={flow} state={state} />;
  } else if (state.phase === "prepare") {
    isi = <PreparePanel flow={flow} mode={captureMode} />;
  } else {
    isi = <IntroPanel flow={flow} state={state} />;
  }

  return (
    <Screen scroll>
      <Header onBack={onBack} />
      <StatusBadges
        consent={state.consent}
        enrollment={state.enrollment}
        trusted={flow.trustedActive()}
      />
      {state.error && state.phase !== "error" && state.phase !== "loading" ? (
        <ErrorBanner
          error={state.error}
          onRetry={() => {
            if (state.phase === "intro" || state.phase === "prepare") void flow.load();
            else void flow.refresh();
          }}
        />
      ) : null}
      {state.stale && state.phase !== "scan" && state.phase !== "uploading" ? (
        <View
          accessible
          accessibilityRole="text"
          accessibilityLabel={`Status terakhir yang diverifikasi: ${labelEnrollment(
            state.enrollment
          )}. Status saat ini belum dapat diverifikasi. Status aktif lama tidak dianggap terdaftar.`}
          style={[styles.stale, { borderColor: colors.warning }]}
        >
          <Ionicons name="cloud-offline-outline" size={16} color={colors.warning} />
          <AppText variant="caption" color={colors.textMuted} style={styles.errorText}>
            Status terakhir yang diverifikasi: {labelEnrollment(state.enrollment)}. Status saat ini
            belum dapat diverifikasi; status aktif lama tidak dianggap terdaftar.
          </AppText>
        </View>
      ) : null}
      {isi}
    </Screen>
  );
}

const styles = StyleSheet.create({
  header: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.md,
  },
  back: {
    width: touch.minWidth,
    height: touch.minHeight,
    borderRadius: radius.pill,
    borderWidth: StyleSheet.hairlineWidth,
    alignItems: "center",
    justifyContent: "center",
  },
  headerText: {
    flex: 1,
    gap: 2,
  },
  stageMark: {
    width: 40,
    height: 40,
    borderRadius: radius.pill,
    alignItems: "center",
    justifyContent: "center",
  },
  badges: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: spacing.sm,
  },
  badge: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.xs,
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: radius.pill,
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.xs,
  },
  dot: {
    width: 7,
    height: 7,
    borderRadius: radius.pill,
  },
  stack: {
    gap: spacing.lg,
  },
  card: {
    gap: spacing.md,
    padding: spacing.xl,
    borderRadius: radius.xl,
    borderWidth: StyleSheet.hairlineWidth,
    alignItems: "stretch",
  },
  cardJudul: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.sm,
  },
  heroIcon: {
    width: 64,
    height: 64,
    borderRadius: radius.pill,
    alignItems: "center",
    justifyContent: "center",
    alignSelf: "center",
  },
  poin: {
    gap: spacing.lg,
  },
  poinBaris: {
    flexDirection: "row",
    gap: spacing.md,
    alignItems: "flex-start",
  },
  poinTeks: {
    flex: 1,
    gap: 2,
  },
  langkah: {
    paddingVertical: spacing.sm,
  },
  noticeBox: {
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: radius.lg,
    padding: spacing.lg,
  },
  checkboxBaris: {
    flexDirection: "row",
    alignItems: "flex-start",
    gap: spacing.md,
  },
  checkbox: {
    width: 22,
    height: 22,
    borderRadius: 6,
    borderWidth: 1.5,
    alignItems: "center",
    justifyContent: "center",
    marginTop: 1,
  },
  checkboxTeks: {
    flex: 1,
  },
  panduanGrid: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: spacing.sm,
  },
  panduan: {
    width: "48%",
    gap: spacing.xs,
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: radius.lg,
    padding: spacing.md,
  },
  scanCard: {
    gap: spacing.lg,
    padding: spacing.xl,
    borderRadius: radius.xxl,
    borderWidth: StyleSheet.hairlineWidth,
    alignItems: "stretch",
  },
  scanTop: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: spacing.sm,
    alignItems: "center",
    justifyContent: "space-between",
  },
  syntheticBadge: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.xs,
    paddingHorizontal: spacing.sm,
    paddingVertical: spacing.xs,
    borderRadius: radius.pill,
  },
  oval: {
    width: 220,
    height: 280,
    alignSelf: "center",
    borderRadius: 120,
    borderWidth: 2,
    alignItems: "center",
    justifyContent: "center",
    overflow: "hidden",
  },
  ovalInner: {
    position: "absolute",
    width: 196,
    height: 254,
    borderRadius: 110,
    borderWidth: StyleSheet.hairlineWidth,
  },
  scanLine: {
    position: "absolute",
    left: 18,
    right: 18,
    height: 2,
  },
  progressTrack: {
    height: 6,
    borderRadius: radius.pill,
    overflow: "hidden",
  },
  progressFill: {
    height: "100%",
    borderRadius: radius.pill,
  },
  infoRow: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: spacing.sm,
    justifyContent: "space-between",
    alignItems: "center",
    borderTopWidth: StyleSheet.hairlineWidth,
    paddingTop: spacing.md,
  },
  error: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.sm,
    padding: spacing.md,
    borderRadius: radius.lg,
    borderWidth: StyleSheet.hairlineWidth,
  },
  errorText: {
    flex: 1,
  },
  retry: {
    minHeight: touch.minHeight,
    minWidth: touch.minWidth,
    justifyContent: "center",
    alignItems: "center",
  },
  stale: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.sm,
    padding: spacing.md,
    borderRadius: radius.lg,
    borderWidth: StyleSheet.hairlineWidth,
  },
});

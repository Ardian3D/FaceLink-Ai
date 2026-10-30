import React, { useCallback, useEffect, useRef, useState } from "react";
import { AccessibilityInfo, ActivityIndicator, AppState, Modal, Pressable, ScrollView, StyleSheet, TextInput, View } from "react-native";
import { Redirect, useFocusEffect, useRouter } from "expo-router";
import { AppText } from "@/components/AppText";
import { Screen } from "@/components/Screen";
import { StateView } from "@/components/StateView";
import { useSession } from "@/features/auth/SessionProvider";
import { roleHome } from "@/features/auth/roleRoutes";
import { labelKonsen } from "@/features/enrollment/model";
import { radius, spacing, useTheme } from "@/theme";
import { AdminReviewFlow, canApprove, canReject, canRetry } from "./flow";
import { useAdminReviewSession } from "./ReviewSession";
import type { EnrollmentError } from "@/features/enrollment/model";
import type { AdminDetail, Decision } from "./model";
import { ProtectedPreview } from "./ProtectedPreview";
import { adminEnrollmentStagingAktif } from "./staging";

const statuses = ["PENDING_APPROVAL", "APPROVED_PROCESSING", "ACTIVE", "REJECTED", "FAILED_PROCESSING", "EXPIRED", "REVOKED"] as const;
const labels: Record<string, string> = {
  NOT_ENROLLED: "Belum terdaftar", PENDING_APPROVAL: "Menunggu persetujuan",
  APPROVED_PROCESSING: "Disetujui · sedang diproses", ACTIVE: "Aktif",
  REJECTED: "Ditolak", FAILED_PROCESSING: "Pemrosesan gagal", EXPIRED: "Kedaluwarsa", REVOKED: "Dicabut",
};
const reasonLabels = {
  IDENTITY_MISMATCH: "Identitas tidak sesuai",
  EVIDENCE_INSUFFICIENT: "Bukti belum memadai",
  UNSUITABLE_SUBMISSION: "Pengajuan tidak sesuai",
} as const;
const uuid4 = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

/** Local contrast fix: existing dark-red brand token, not a global palette change. */
function Action({ label, onPress, disabled = false, primary = false, busy = false }: {
  label: string; onPress: () => void; disabled?: boolean; primary?: boolean; busy?: boolean;
}) {
  const { colors } = useTheme();
  return <Pressable accessibilityRole="button" accessibilityLabel={label}
    accessibilityState={{ disabled: disabled || busy, busy }} disabled={disabled || busy} onPress={onPress}
    style={({ pressed }) => [styles.button, { backgroundColor: primary ? colors.accentPressed : colors.surfaceElevated,
      borderColor: colors.border, opacity: disabled || busy ? 0.55 : pressed ? 0.8 : 1 }]}>
    {busy ? <ActivityIndicator color={primary ? colors.onAccent : colors.textPrimary} /> : null}
    <AppText weight="semibold" center color={primary ? colors.onAccent : colors.textPrimary}>{label}</AppText>
  </Pressable>;
}
function Card({ children }: { children: React.ReactNode }) {
  const { colors } = useTheme();
  return <View style={[styles.card, { backgroundColor: colors.surface, borderColor: colors.border }]}>{children}</View>;
}
function Field({ label, value }: { label: string; value: string | null | undefined }) {
  const { colors } = useTheme();
  return <View style={styles.field}><AppText variant="caption" color={colors.textMuted}>{label}</AppText>
    <AppText selectable>{value || "Tidak tersedia"}</AppText></View>;
}
function Status({ view, trusted }: { view: AdminDetail; trusted: boolean }) {
  const ready = view.effectiveStatus !== "ACTIVE" || (view.templateReady && view.consentState === "ACTIVE");
  return <AppText weight="semibold" accessibilityLiveRegion="polite">
    {trusted && ready ? labels[view.effectiveStatus] ?? "Status tidak diketahui" : `Status terakhir: ${labels[view.effectiveStatus] ?? view.effectiveStatus} · belum diverifikasi ulang`}
  </AppText>;
}
function errorText(kind: string, code?: string): string {
  switch (kind) {
    case "network": case "tls": return "Backend tidak dapat dihubungi. Data sebelumnya bukan status terkini; preview ditutup.";
    case "sessionEnded": return "Sesi telah berakhir. Masuk kembali untuk melanjutkan review.";
    case "forbidden": return "Izin Admin tidak tersedia. Data review telah ditutup.";
    case "notFound": return "Pengajuan tidak ditemukan atau berada di luar cakupan Anda.";
    case "precondition": case "conflict": return "Data telah berubah atau keputusan sudah dibuat. Muat ulang sebelum mengambil tindakan.";
    case "photoUnavailable": return "Foto tidak tersedia, telah dihapus, atau melewati batas simpan.";
    case "rateLimited": return "Terlalu banyak permintaan. Tunggu sebentar sebelum memuat ulang.";
    case "validation": return "Periksa konfirmasi, metode, referensi kampus UUIDv4, atau alasan penolakan.";
    case "unavailable": return "Layanan pemrosesan belum tersedia. Tidak ada status sukses yang diasumsikan.";
    default: return code === "BIOMETRIC_UNAVAILABLE" ? "Artefak belum dapat diproses. Backend tetap menjadi sumber status." : "Status belum dapat diverifikasi. Muat ulang; jangan mengulang keputusan berbeda.";
  }
}
function deadlineText(deadline: string | null, now: number) {
  if (!deadline || !Number.isFinite(now)) return "Waktu server belum tersedia";
  const remaining = Date.parse(deadline) - now;
  if (!Number.isFinite(remaining)) return "Deadline tidak valid";
  if (remaining <= 0) return "Batas waktu telah lewat";
  const seconds = Math.ceil(remaining / 1000);
  return `${Math.floor(seconds / 3600)} jam ${Math.floor(seconds % 3600 / 60)} menit ${seconds % 60} detik tersisa`;
}

export default function AdminEnrollmentScreen() {
  const { session, status } = useSession();
  if (!adminEnrollmentStagingAktif) return <Redirect href="/" />;
  if (!session || status !== "signedIn") return <Redirect href="/" />;
  if (session.user.role !== "admin") return <Redirect href={roleHome[session.user.role]} />;
  return <ReviewScreen key={session.user.id} />;
}

function ReviewScreen() {
  const router = useRouter();
  const { colors } = useTheme();
  const { getAccessToken, signOut } = useSession();
  const flow = useRef<AdminReviewFlow | null>(null);
  const { owner, state } = useAdminReviewSession();
  const [search, setSearch] = useState("");
  const [foreground, setForeground] = useState(AppState.currentState === "active");
  const [confirmation, setConfirmation] = useState<Decision | null>(null);
  const [preview, setPreview] = useState<{ id: string; revision: number } | null>(null);
  const [now, setNow] = useState(NaN);
  const previewDenied = useCallback((error: EnrollmentError) => {
    if (error.kind === "forbidden" || error.kind === "sessionEnded") {
      setPreview(null);
      flow.current?.invalidate(error);
    }
  }, []);

  useFocusEffect(useCallback(() => {
    const controller = owner.acquire();
    flow.current = controller;
    void controller.refresh();
    const timer = setInterval(() => { controller.tick(); setNow(controller.now()); }, 1000);
    const polling = setInterval(() => { if (AppState.currentState === "active" && controller.snapshot().screen === "detail") void controller.refresh(); }, 5000);
    const listener = AppState.addEventListener("change", (next) => {
      setForeground(next === "active");
      setPreview(null);
      setConfirmation(null);
      if (next === "active") void controller.refresh();
      else controller.suspend();
    });
    return () => { clearInterval(timer); clearInterval(polling); listener.remove(); controller.suspend(); flow.current = null; setPreview(null); setConfirmation(null); };
  }, [owner]));
  const view = state?.detail ?? null;
  const trusted = Boolean(state && !state.stale && !state.denied && !state.error && !state.uncertain && foreground);
  // Polling the same revision must not unmount an open preview; any error,
  // settled staleness, selection/revision change, or background clears consent.
  const validPreview = Boolean(preview && view && preview.id === view.submissionId && preview.revision === view.revision
    && foreground && !state?.error && !state?.denied && !state?.uncertain && (state?.busy === "detail" || !state?.stale)
    && view.photoAvailable && view.photoExpiresAt && Date.parse(view.photoExpiresAt) > now);
  // Drop remembered preview intent with its invalidating snapshot, before recovery
  // can make it visible again. The conditional converges to preview === null.
  if (preview && (!view || preview.id !== view.submissionId || preview.revision !== view.revision || !view.photoAvailable || state?.error || state?.denied || state?.uncertain || !foreground || (state?.stale && state?.busy === null))) setPreview(null);
  const message = state?.error ? errorText(state.error.kind, state.error.code) : null;
  useEffect(() => { if (message) AccessibilityInfo.announceForAccessibility(message); }, [message]);
  const confirmationAllowed = view && state && trusted && (confirmation?.kind === "approve" ? canApprove(view, now, state.stale)
    : confirmation?.kind === "reject" ? canReject(view, now, state.stale) : confirmation?.kind === "retry" && canRetry(view, now, state.stale));
  const visibleItems = state?.items.filter((item) => `${item.student.displayName} ${item.student.externalCode} ${item.student.classes.map((c) => `${c.code} ${c.title}`).join(" ")}`.toLocaleLowerCase().includes(search.trim().toLocaleLowerCase())) ?? [];

  return <Screen scroll>
    <Action label={state?.screen === "detail" ? "Kembali ke antrean" : "Kembali ke Admin"} onPress={() => {
      setPreview(null); setConfirmation(null);
      if (state?.screen === "detail") flow.current?.back(); else router.back();
    }} />
    <AppText variant="micro" color={colors.textMuted}>ADMIN · STAGING · FIXTURE SINTETIS</AppText>
    <AppText variant="heading" accessibilityRole="header">Face Enrollment</AppText>
    <AppText color={colors.textSecondary}>Pengajuan sintetis saja. Bukan verifikasi identitas manusia atau izin enrollment production.</AppText>
    {state?.denied ? <Card><AppText accessibilityRole="alert">{message ?? "Sesi atau izin review tidak tersedia."}</AppText>
      <Action label="Keluar dan masuk kembali" onPress={() => { flow.current?.invalidate(); void signOut().then(() => router.replace("/")); }} /></Card> : <>
      <Action label="Muat ulang status server" busy={state?.busy !== null && state?.busy !== undefined} onPress={() => { setPreview(null); setConfirmation(null); void flow.current?.refresh(); }} />
      {message ? <Card><AppText accessibilityRole="alert">{message}</AppText></Card> : null}
      {state?.uncertain ? <Card><AppText accessibilityRole="alert">Hasil keputusan belum diketahui. Jangan mengirim keputusan baru atau berlawanan. Periksa status atau ulangi operasi yang sama.</AppText>
        <Action label="Periksa ulang operasi yang sama" busy={state.busy === "decision"} onPress={() => { setPreview(null); void flow.current?.replay(); }} /></Card> : null}
      {!state ? <StateView tone="loading" title="Memuat antrean" /> : state.screen === "queue" ? <>
        <Card><AppText variant="bodyLarge" weight="bold">{state.pendingCount} pengajuan menunggu</AppText>
          <AppText variant="caption">Jumlah dari server; pencarian hanya pada halaman yang sudah dimuat.</AppText>
          <AppText weight="semibold">Filter status</AppText>
          <View style={styles.wrap}>{["", ...statuses].map((filter) => <Pressable key={filter || "all"} accessibilityRole="button"
            accessibilityState={{ selected: state.filter === filter, disabled: state.busy !== null }} disabled={state.busy !== null}
            onPress={() => { setPreview(null); void flow.current?.load(filter); }}
            style={[styles.filter, { backgroundColor: state.filter === filter ? colors.surfaceElevated : colors.background, borderColor: state.filter === filter ? colors.textPrimary : colors.border }]}>
            <AppText variant="caption">{labels[filter] ?? "Semua status"}</AppText></Pressable>)}</View>
          <AppText>Cari nama, NIM, atau kelas fixture</AppText>
          <TextInput accessibilityLabel="Cari nama NIM atau kelas fixture" value={search} onChangeText={setSearch} maxLength={120}
            placeholder="Cari pada halaman dimuat" placeholderTextColor={colors.textMuted}
            style={[styles.input, { color: colors.textPrimary, borderColor: colors.border }]} />
        </Card>
        {state.busy === "queue" && state.items.length === 0 ? <StateView tone="loading" title="Mengambil pengajuan dari backend" /> : visibleItems.length === 0 ?
          <StateView tone="empty" title="Tidak ada pengajuan pada tampilan ini" description="Ubah filter atau pencarian, lalu muat ulang bila perlu." /> : visibleItems.map((item) => <Card key={item.id}>
            <AppText variant="bodyLarge" weight="semibold">{item.student.displayName}</AppText>
            <Field label="NIM fixture" value={item.student.externalCode} />
            <Field label="Kelas / roster" value={item.student.classes.map((c) => `${c.code} · ${c.title}`).join("; ")} />
            <Field label="Status dari server" value={labels[item.status] ?? item.status} />
            <Field label="Diajukan (UTC)" value={item.submittedAt} />
            <Field label="Batas simpan foto (UTC)" value={item.photoExpiresAt} />
            <AppText variant="caption">{deadlineText(item.photoExpiresAt, now)}</AppText>
            <Action label={`Buka pengajuan ${item.student.externalCode}`} disabled={state.busy !== null} onPress={() => { setPreview(null); void flow.current?.open(item.id); }} />
          </Card>)}
        {state.hasMore ? <Action label="Muat halaman berikutnya" busy={state.busy === "queue"} onPress={() => void flow.current?.load(state.filter, true)} /> : null}
      </> : !view ? <StateView tone={state.busy === "detail" ? "loading" : "error"} title="Detail belum tersedia" description="Pilih ulang atau muat status dari server." /> : <>
        <Card><AppText variant="title" accessibilityRole="header">{view.student.displayName}</AppText>
          <Status view={view} trusted={trusted} />
          <Field label="Submission ID" value={view.submissionId} />
          <Field label="NIM fixture" value={view.student.externalCode} />
          <Field label="Person ID" value={view.student.personId} />
          <Field label="Akun mahasiswa" value={view.student.userId} />
          <Field label="Binding akun → person" value={view.identityBinding.accountUserId ? `${view.identityBinding.accountUserId} → ${view.identityBinding.personId}` : `Tidak tertaut (person ${view.identityBinding.personId})`} />
          <Field label="Status akun / role / person" value={view.identityBinding.accountUserId ? `${view.identityBinding.accountStatus} / ${view.identityBinding.accountRole} / ${view.identityBinding.personStatus}` : `Akun tidak tertaut / — / ${view.identityBinding.personStatus}`} />
          <Field label="Kelas / roster aktif" value={view.identityBinding.classes.map((c) => `${c.code} · ${c.title}`).join("; ")} />
          <Field label="Referensi identitas fixture" value={view.identityBinding.identityReference} />
          <Field label="Identitas nonbiometrik diverifikasi (UTC)" value={view.identityBinding.identityVerifiedAt} />
          <Field label="Kelayakan binding dari server" value={view.identityBinding.eligible ? "Memenuhi syarat" : "Tidak memenuhi syarat"} />
          <Field label="Status verifikasi identitas" value={view.identityState} />
          <Field label="Consent" value={labelKonsen(view.consent.state)} />
          <Field label="Versi notice sintetis" value={view.consent.notice.version} />
          <Field label="Consent diberikan (UTC)" value={view.consent.grantedAt} />
          <Field label="Consent ditarik (UTC)" value={view.consent.withdrawnAt} />
          <Field label="Waktu submission (UTC)" value={view.submittedAt} />
          <Field label="Batas foto dari backend (UTC)" value={view.photoExpiresAt} />
          <AppText variant="caption" accessibilityLabel="Sisa waktu berdasarkan jam server">{deadlineText(view.photoExpiresAt, now)}</AppText>
          <Field label="Integrity artefak (metadata)" value={view.artifactIntegrity === "NOT_CHECKED" ? "Belum diperiksa; dekripsi/hash diperiksa saat preview" : "Tidak tersedia"} />
          <Field label="Penghapusan artefak" value={view.deletionState} />
          <Field label="Template sintetis siap" value={trusted && view.templateReady ? "Ya · dikonfirmasi backend" : "Belum dikonfirmasi"} />
          <Field label="Diperiksa (UTC)" value={view.reviewedAt} />
          <Field label="Alasan hasil / processing" value={view.reasonCode} />
          <Field label="Versi detail / respons keputusan" value={`${view.revision} / ${state.lastResponseStatus ?? "belum ada keputusan pada layar ini"}`} />
          {view.review ? <>
            <Field label="Admin reviewer" value={view.review.reviewerAdminId} />
            <Field label="Keputusan review" value={view.review.decision} />
            <Field label="Waktu keputusan (UTC)" value={view.review.at} />
            <Field label="Metode pemeriksaan sintetis" value={view.review.identityCheckMethod} />
            <Field label="Referensi attestation sintetis" value={view.review.campusCheckRef} />
          </> : <AppText>Belum ada keputusan Admin pada pengajuan ini.</AppText>}
          <AppText variant="caption">Identitas email/OTP bukan bukti kampus. Review sintetis mencatat attestation pada transaksi Approve, bukan hasil pemeriksaan fisik nyata. Detail menyediakan waktu review, status, dan kode alasan; bukan akses log audit penuh.</AppText>
        </Card>
        <Card><AppText variant="bodyLarge" accessibilityRole="header">Preview terotorisasi</AppText>
          <AppText>JPEG pola sintetis. Tidak ada URL publik, token pada URL, atau penyimpanan gambar oleh aplikasi.</AppText>
          {validPreview ? <ProtectedPreview key={`${view.submissionId}:${view.revision}`} submissionId={view.submissionId!} getToken={getAccessToken}
            onClose={() => setPreview(null)} onDenied={previewDenied} /> :
            <Action label="Buka preview fixture sintetis" disabled={!trusted || !view.photoAvailable || !view.photoExpiresAt || Date.parse(view.photoExpiresAt) <= now}
              onPress={() => setPreview({ id: view.submissionId!, revision: view.revision })} />}
          {!view.photoAvailable ? <AppText>Foto tidak tersedia. Keputusan tidak menghidupkan kembali artefak yang dihapus.</AppText> : null}
        </Card>
        <DecisionForm key={`${view.submissionId}:${view.revision}`} view={view} now={now} stale={!trusted} busy={state.busy !== null}
          onConfirm={(decision) => { setPreview(null); setConfirmation(decision); }} />
        <Card>
          <AppText weight="semibold" accessibilityRole="header">Riwayat / audit terkait (maksimal 50)</AppText>
          {view.history.length === 0 ? <AppText>Belum ada peristiwa terkait yang tersedia.</AppText> : view.history.map((event) =>
            <View key={event.auditId} style={styles.field}>
              <AppText selectable>{event.action} · {event.time}</AppText>
              <AppText variant="caption" selectable>Actor: {event.actorId ?? "Sistem"}{event.reasonCode ? ` · ${event.reasonCode}` : ""}</AppText>
            </View>)}
        </Card>
      </>}
    </>}
    <Modal visible={confirmation !== null} transparent animationType="fade" onRequestClose={() => setConfirmation(null)}>
      <View style={[styles.modalWrap, { backgroundColor: colors.overlayStrong }]}>
        <ScrollView contentContainerStyle={styles.modalContent} keyboardShouldPersistTaps="handled">
          <View accessibilityViewIsModal style={[styles.card, { backgroundColor: colors.surface, borderColor: colors.border }]}>
            <AppText variant="title" accessibilityRole="header">Konfirmasi {confirmation?.kind === "approve" ? "Approve" : confirmation?.kind === "reject" ? "Reject" : "Retry"}</AppText>
            <AppText>{view?.student.displayName} · {view?.student.externalCode}</AppText>
            <AppText>{confirmation?.kind === "approve" ? "Anda menyatakan binding, NIM, roster dan pemeriksaan kampus FIXTURE telah diperiksa. HTTP 202 berarti diproses, bukan ACTIVE. Tidak ada verifikasi manusia."
              : confirmation?.kind === "reject" ? `Alasan: ${reasonLabels[confirmation.reasonCode]}. Backend menangani penolakan dan penghapusan sesuai kebijakan.`
                : "Ulangi pemrosesan sintetis sesuai kondisi dan batas waktu backend. Tidak ada status sukses sebelum server mengonfirmasikannya."}</AppText>
            {!confirmationAllowed ? <AppText accessibilityRole="alert">Status berubah, kedaluwarsa, atau belum dapat diverifikasi. Tutup lalu muat ulang.</AppText> : null}
            <Action label="Batal, jangan kirim keputusan" onPress={() => setConfirmation(null)} />
            <Action label="Konfirmasi dan kirim ke backend" primary disabled={!confirmationAllowed || state?.busy !== null} onPress={() => {
              const decision = confirmation; setConfirmation(null); setPreview(null); if (decision) void flow.current?.decide(decision);
            }} />
          </View>
        </ScrollView>
      </View>
    </Modal>
  </Screen>;
}

function DecisionForm({ view, now, stale, busy, onConfirm }: {
  view: AdminDetail; now: number; stale: boolean; busy: boolean; onConfirm: (decision: Decision) => void;
}) {
  const { colors } = useTheme();
  const [checked, setChecked] = useState(false);
  const [reference, setReference] = useState("");
  const [reason, setReason] = useState<keyof typeof reasonLabels>("EVIDENCE_INSUFFICIENT");
  return <Card><AppText variant="bodyLarge" accessibilityRole="header">Verifikasi dan keputusan</AppText>
    <AppText>Periksa binding satu akun–person, NIM fixture, roster, status aktif, dan bukti kampus sintetis. Attestation dicatat atomik bersama Approve; bukan endpoint verifikasi terpisah.</AppText>
    <AppText>Metode: CAMPUS_AUTHORIZED_CHECK · simulasi pemeriksaan kampus berwenang.</AppText>
    <AppText>Referensi pemeriksaan kampus sintetis (UUIDv4)</AppText>
    <TextInput accessibilityLabel="Referensi pemeriksaan kampus sintetis UUIDv4" value={reference} onChangeText={(text) => { setReference(text); setChecked(false); }}
      editable={!busy} maxLength={36} autoCapitalize="none" autoCorrect={false} placeholder="UUIDv4 referensi fixture" placeholderTextColor={colors.textMuted}
      style={[styles.input, { color: colors.textPrimary, borderColor: colors.border }]} />
    {view.identityBinding.identityReference && uuid4.test(view.identityBinding.identityReference) ?
      <Action label="Gunakan referensi identitas fixture di atas" disabled={busy} onPress={() => { setReference(view.identityBinding.identityReference!); setChecked(false); }} /> : null}
    <Pressable accessibilityRole="checkbox" accessibilityState={{ checked, disabled: busy }} disabled={busy} onPress={() => setChecked(!checked)}
      style={[styles.checkbox, { borderColor: colors.border }]}>
      <AppText>{checked ? "☑" : "☐"} Saya telah memeriksa binding, NIM, roster dan referensi pemeriksaan FIXTURE sintetis. Ini bukan verifikasi identitas manusia.</AppText>
    </Pressable>
    <Action label="Approve · tinjau konfirmasi" primary disabled={busy || !checked || !uuid4.test(reference) || !canApprove(view, now, stale)}
      onPress={() => onConfirm({ kind: "approve", identityCheckMethod: "CAMPUS_AUTHORIZED_CHECK", campusCheckRef: reference.toLowerCase(), confirmed: true })} />
    <AppText weight="semibold">Alasan penolakan terstruktur</AppText>
    {(Object.keys(reasonLabels) as (keyof typeof reasonLabels)[]).map((r) => <Pressable key={r} accessibilityRole="radio"
      accessibilityState={{ checked: reason === r, disabled: busy }} disabled={busy} onPress={() => setReason(r)}
      style={[styles.filter, { borderColor: reason === r ? colors.textPrimary : colors.border }]}><AppText>{reasonLabels[r]}</AppText></Pressable>)}
    <Action label="Reject · tinjau konfirmasi" disabled={busy || !canReject(view, now, stale)} onPress={() => onConfirm({ kind: "reject", reasonCode: reason, confirmed: true })} />
    <Action label="Retry processing · tinjau konfirmasi" disabled={busy || !canRetry(view, now, stale)} onPress={() => onConfirm({ kind: "retry", confirmed: true })} />
    <AppText variant="caption">Approve/Reject/Retry diperiksa kembali di Go: sesi, role, consent, binding, deadline, versi dan idempotensi. Tombol nonaktif bukan pengganti RBAC.</AppText>
  </Card>;
}
const styles = StyleSheet.create({
  card: { borderWidth: 1, borderRadius: radius.lg, padding: spacing.lg, gap: spacing.md },
  button: { borderWidth: 1, borderRadius: radius.md, minHeight: 48, minWidth: 48, padding: spacing.md, justifyContent: "center", alignItems: "center", gap: spacing.sm },
  field: { gap: spacing.xs },
  input: { minHeight: 48, borderWidth: 1, borderRadius: radius.md, padding: spacing.md, fontSize: 16 },
  wrap: { flexDirection: "row", flexWrap: "wrap", gap: spacing.sm },
  filter: { borderWidth: 1, borderRadius: radius.md, minHeight: 48, minWidth: 48, padding: spacing.md, justifyContent: "center" },
  checkbox: { borderWidth: 1, borderRadius: radius.md, minHeight: 48, padding: spacing.md },
  modalWrap: { flex: 1, justifyContent: "center", padding: spacing.xl },
  modalContent: { flexGrow: 1, justifyContent: "center", width: "100%", maxWidth: 560, alignSelf: "center" },
});

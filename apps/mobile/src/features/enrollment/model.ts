import type { EnrollmentPhase } from "./flow";

/** Contract types for the synthetic FL-ENR01B enrollment endpoints. */

export type ConsentState = "NOT_GIVEN" | "ACTIVE" | "WITHDRAWN";

export type EnrollmentEffectiveStatus =
  | "NOT_ENROLLED"
  | "PENDING_APPROVAL"
  | "APPROVED_PROCESSING"
  | "ACTIVE"
  | "REJECTED"
  | "FAILED_PROCESSING"
  | "EXPIRED"
  | "REVOKED";

export type ConsentNotice = {
  version: string;
  digest: string;
  text: string;
};

export type ConsentView = {
  state: ConsentState;
  recordId: string | null;
  notice: ConsentNotice;
  grantedAt: string | null;
  withdrawnAt: string | null;
  revision: number;
};

export type EnrollmentView = {
  effectiveStatus: EnrollmentEffectiveStatus;
  serverNow: string | null;
  submissionStatus: string | null;
  submissionId: string | null;
  consentState: ConsentState;
  identityState: string;
  templateReady: boolean;
  submittedAt: string | null;
  reviewedAt: string | null;
  processingDeadline: string | null;
  reasonCode: string | null;
  deletionState: string;
  revision: number;
  allowedActions: string[];
  photoAvailable: boolean;
  photoExpiresAt: string | null;
  syntheticOnly: boolean;
};

export type ConsentEnvelope = {
  consent: ConsentView;
  etag: string;
};

export type EnrollmentErrorKind =
  | "config"
  | "network"
  | "tls"
  | "cancelled"
  | "sessionEnded"
  | "forbidden"
  | "notFound"
  | "conflict"
  | "photoUnavailable"
  | "precondition"
  | "validation"
  | "rateLimited"
  | "unavailable"
  | "unknown";

export type EnrollmentError = {
  kind: EnrollmentErrorKind;
  /** Stable server code, allowed for display mapping but not raw message. */
  code?: string;
  retryAfter?: number;
};

export type HasilEnrollment<T> =
  | { ok: true; data: T }
  | { ok: false; error: EnrollmentError };

/** True only for a currently verified ACTIVE status with a ready template. */
export function statusTerpercaya(
  view: EnrollmentView | null,
  stale: boolean,
  consent: ConsentView | null
): boolean {
  return (
    !stale &&
    view !== null &&
    view.effectiveStatus === "ACTIVE" &&
    view.templateReady &&
    view.consentState === "ACTIVE" &&
    consent?.state === "ACTIVE"
  );
}

/** Human labels that never assert enrollment/consent beyond backend values. */
export function labelKonsen(state: ConsentState | null): string {
  switch (state) {
    case "ACTIVE":
      return "Persetujuan aktif";
    case "WITHDRAWN":
      return "Persetujuan ditarik";
    case "NOT_GIVEN":
      return "Persetujuan belum diberikan";
    default:
      return "Status persetujuan belum diketahui";
  }
}

export function labelEnrollment(view: EnrollmentView | null): string {
  if (view === null) return "Status pendaftaran belum diketahui";
  switch (view.effectiveStatus) {
    case "NOT_ENROLLED":
      return "Belum terdaftar";
    case "PENDING_APPROVAL":
      return "Menunggu verifikasi Admin";
    case "APPROVED_PROCESSING":
      return "Sedang diproses";
    case "ACTIVE":
      return "Aktif";
    case "REJECTED":
      return "Ditolak";
    case "FAILED_PROCESSING":
      return "Pemrosesan gagal";
    case "EXPIRED":
      return "Kedaluwarsa";
    case "REVOKED":
      return "Dicabut";
  }
}

/** Last known backend label; a stale ACTIVE is explicitly not current. */
export function labelStatusTerakhir(
  view: EnrollmentView | null,
  stale: boolean,
  trusted: boolean
): string {
  if (view === null) return "Belum ada status dari server";
  if (view.effectiveStatus === "ACTIVE" && (stale || !trusted)) {
    return "Aktif (belum terverifikasi saat ini)";
  }
  return labelEnrollment(view);
}

/** Why an ACTIVE snapshot is not currently trustworthy, without claiming it is current. */
export function pesanActiveTidakTerpercaya(
  view: EnrollmentView | null,
  consent: ConsentView | null,
  stale: boolean
): string {
  if (stale) {
    return "Status terakhir yang terverifikasi: Aktif. Status saat ini belum dapat diverifikasi. Muat ulang status sebelum menganggap wajah terdaftar.";
  }
  if (view?.templateReady !== true) {
    return "Snapshot ACTIVE terakhir belum memiliki template siap. Status ini belum dapat dipakai sebagai terdaftar.";
  }
  if (view.consentState !== "ACTIVE" || consent?.state !== "ACTIVE") {
    return "Snapshot ACTIVE terakhir tidak lagi didukung persetujuan aktif. Status ini belum dapat dipakai sebagai terdaftar.";
  }
  return "Snapshot ACTIVE sudah melewati batas kesegaran. Status saat ini belum dapat diverifikasi; muat ulang status.";
}

/** One screen-reader announcement per meaningful phase/trust change. */
export function ringkasanStatusAksesibel(
  phase: EnrollmentPhase,
  view: EnrollmentView | null,
  consent: ConsentView | null,
  trusted: boolean
): string {
  switch (phase) {
    case "loading":
      return "Memuat status verifikasi.";
    case "error":
      return "Status verifikasi tidak dapat dimuat. Coba lagi setelah koneksi pulih.";
    case "intro":
      return `Verifikasi wajah. ${labelKonsen(consent?.state ?? null)}. ${labelEnrollment(view)}.`;
    case "consent":
      return "Informasi persetujuan. Baca pemberitahuan, centang persetujuan, lalu lanjutkan.";
    case "prepare":
      return "Persetujuan aktif. Siap memulai pemindaian sintetis.";
    case "scan":
      return "Pemindaian sintetis berjalan.";
    case "uploading":
      return "Mengirim bingkai sintetis. Tunggu hingga selesai.";
    case "pending":
      return "Menunggu verifikasi Admin. Status belum aktif.";
    case "processing":
      return "Pengajuan sedang diproses. Status belum aktif.";
    case "active":
      return trusted
        ? "Wajah terdaftar. Status berasal dari backend dan snapshot yang masih segar."
        : `Status terakhir: ${labelEnrollment(view)}. Status saat ini belum dapat diverifikasi.`;
    case "rejected":
      return "Pengajuan ditolak. Status tidak aktif.";
    case "expired":
      return "Pengajuan kedaluwarsa. Foto telah dihapus.";
    case "failed":
      return "Pemrosesan gagal. Tidak ada template aktif yang diklaim.";
    case "withdrawn":
      return "Persetujuan ditarik. Status aktif tidak ditampilkan.";
  }
}

/** Safe copy for every error; raw backend messages are never rendered. */
export function pesanEnrollment(error: EnrollmentError): string {
  switch (error.kind) {
    case "config":
      return "Alamat backend staging belum valid. Periksa konfigurasi build.";
    case "network":
      return "Server staging tidak terjangkau. Periksa koneksi lalu coba lagi.";
    case "tls":
      return "Sertifikat server staging tidak tepercaya pada perangkat ini.";
    case "cancelled":
      return "Tindakan dibatalkan.";
    case "sessionEnded":
      return "Sesi berakhir. Masuk kembali untuk melanjutkan.";
    case "forbidden":
      return "Akun ini tidak memiliki akses ke alur verifikasi wajah.";
    case "notFound":
      return "Data verifikasi tidak ditemukan.";
    case "photoUnavailable":
      return "Foto sintetis sudah tidak tersedia atau kedaluwarsa.";
    case "precondition":
      return "Status berubah di server. Muat ulang sebelum mencoba lagi.";
    case "validation":
      return "Permintaan tidak memenuhi kontrak atau data yang diizinkan.";
    case "rateLimited":
      return "Terlalu banyak percobaan. Tunggu sebentar sebelum mencoba lagi.";
    case "unavailable":
      return "Layanan sintetis belum tersedia. Coba lagi nanti.";
    case "conflict":
      switch (error.code) {
        case "CONSENT_VERSION_MISMATCH":
          return "Versi pemberitahuan berubah. Baca ulang dan setujui versi terbaru.";
        case "CONSENT_RENEWAL_REQUIRED":
          return "Persetujuan baru diperlukan sebelum mengirim foto lagi.";
        case "ENROLLMENT_EXISTS":
          return "Sudah ada pengajuan aktif. Muat ulang status.";
        case "DELETION_INCOMPLETE":
          return "Penghapusan data sebelumnya belum selesai.";
        case "INVALID_TRANSITION":
          return "Status berubah. Muat ulang status terbaru.";
        default:
          return "Permintaan bertentangan dengan status saat ini.";
      }
    default:
      return "Verifikasi belum dapat diselesaikan. Coba lagi atau hubungi Admin.";
  }
}

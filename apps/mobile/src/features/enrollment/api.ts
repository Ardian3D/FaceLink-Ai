/**
 * Client for the actual FL-ENR01B enrollment endpoints.
 *
 * The module only calls the documented student routes. Admin review is not
 * exposed here; staging tests use an isolated admin fixture directly. It never
 * logs tokens, notice text, JPEG bytes, base64, response bodies, or server
 * error messages. A non-synthetic backend is rejected before data is used.
 */
import { konfigurasiApi } from "@/features/api/config";
import {
  adalahObjek,
  bacaJson,
  mintaHttp,
  type JawabanHttp,
} from "@/features/api/http";
import type {
  ConsentEnvelope,
  ConsentState,
  ConsentView,
  EnrollmentError,
  EnrollmentView,
  HasilEnrollment,
} from "./model";

const TIMEOUT_ENROLLMENT_MS = 20_000;

function alamatBackend(): HasilEnrollment<string> {
  if (!konfigurasiApi.ok) {
    return { ok: false, error: { kind: "config" } };
  }
  return { ok: true, data: konfigurasiApi.baseUrl };
}

function galatTransport(
  jawaban: Extract<JawabanHttp, { jenis: "jaringan" | "timeout" }>
): EnrollmentError {
  if (jawaban.jenis === "timeout") return { kind: "network" };
  if (/certificate|ssl|tls|trust anchor|self.signed/i.test(jawaban.pesan)) {
    return { kind: "tls" };
  }
  return { kind: "network" };
}

function bacaGalat(teks: string): { code: string } | null {
  const isi = bacaJson(teks);
  if (!adalahObjek(isi) || !adalahObjek(isi.error)) return null;
  return typeof isi.error.code === "string" ? { code: isi.error.code } : null;
}

function retryDetik(nilai: string | null | undefined): number | undefined {
  if (nilai === null || nilai === undefined) return undefined;
  if (/^\d+$/.test(nilai.trim())) {
    const detik = Number(nilai);
    return Number.isFinite(detik) ? Math.min(300, Math.max(1, detik)) : undefined;
  }
  return undefined;
}

function galatHttp(jawaban: Extract<JawabanHttp, { jenis: "jawaban" }>): EnrollmentError {
  const kode = bacaGalat(jawaban.teks)?.code;
  switch (jawaban.status) {
    case 401:
      return { kind: "sessionEnded" };
    case 403:
      return { kind: "forbidden", code: kode };
    case 404:
      return { kind: "notFound", code: kode };
    case 409:
      return { kind: "conflict", code: kode };
    case 410:
      return { kind: "photoUnavailable", code: kode };
    case 412:
      return { kind: "precondition", code: kode };
    case 413:
      return { kind: "validation", code: kode };
    case 415:
      return { kind: "validation", code: kode };
    case 422:
      return { kind: "validation", code: kode };
    case 428:
      return { kind: "precondition", code: kode };
    case 429:
      return {
        kind: "rateLimited",
        code: kode,
        retryAfter: retryDetik(jawaban.retryAfter),
      };
    case 503:
      return { kind: "unavailable", code: kode };
    default:
      return { kind: "unknown", code: kode };
  }
}

function bacaData(teks: string): unknown {
  const isi = bacaJson(teks);
  if (!adalahObjek(isi)) return null;
  return isi.data ?? null;
}

function parseNotice(nilai: unknown): ConsentView["notice"] | null {
  if (!adalahObjek(nilai)) return null;
  const { version, digest, text } = nilai;
  if (
    typeof version !== "string" ||
    version === "" ||
    typeof digest !== "string" ||
    digest === "" ||
    typeof text !== "string" ||
    text === ""
  ) {
    return null;
  }
  return { version, digest, text };
}

function parseConsent(nilai: unknown): ConsentView | null {
  if (!adalahObjek(nilai)) return null;
  const { state, record_id, granted_at, withdrawn_at, revision } = nilai;
  const notice = parseNotice(nilai.notice);
  if (
    state !== "NOT_GIVEN" &&
    state !== "ACTIVE" &&
    state !== "WITHDRAWN"
  ) {
    return null;
  }
  if (
    notice === null ||
    typeof revision !== "number" ||
    !Number.isSafeInteger(revision) ||
    revision < 0 ||
    (record_id !== undefined && record_id !== null && typeof record_id !== "string") ||
    (granted_at !== undefined && granted_at !== null && typeof granted_at !== "string") ||
    (withdrawn_at !== undefined && withdrawn_at !== null && typeof withdrawn_at !== "string")
  ) {
    return null;
  }
  return {
    state: state as ConsentState,
    recordId: typeof record_id === "string" ? record_id : null,
    notice,
    grantedAt: typeof granted_at === "string" ? granted_at : null,
    withdrawnAt: typeof withdrawn_at === "string" ? withdrawn_at : null,
    revision,
  };
}

function parseEnrollment(nilai: unknown): EnrollmentView | null {
  if (!adalahObjek(nilai)) return null;
  const statuses: Record<string, true> = {
    NOT_ENROLLED: true,
    PENDING_APPROVAL: true,
    APPROVED_PROCESSING: true,
    ACTIVE: true,
    REJECTED: true,
    FAILED_PROCESSING: true,
    EXPIRED: true,
    REVOKED: true,
  };
  const consents: Record<string, true> = {
    NOT_GIVEN: true,
    ACTIVE: true,
    WITHDRAWN: true,
  };
  const {
    effective_status,
    consent_state,
    identity_state,
    template_ready,
    revision,
    allowed_actions,
    photo_available,
    synthetic_only,
  } = nilai;
  if (
    typeof effective_status !== "string" ||
    statuses[effective_status] !== true ||
    typeof consent_state !== "string" ||
    consents[consent_state] !== true ||
    typeof identity_state !== "string" ||
    typeof template_ready !== "boolean" ||
    typeof revision !== "number" ||
    !Number.isSafeInteger(revision) ||
    revision < 0 ||
    !Array.isArray(allowed_actions) ||
    allowed_actions.some((item) => typeof item !== "string") ||
    typeof photo_available !== "boolean" ||
    synthetic_only !== true
  ) {
    return null;
  }
  const stringAtauNull = (kunci: string): string | null =>
    typeof nilai[kunci] === "string" ? (nilai[kunci] as string) : null;
  return {
    effectiveStatus: effective_status as EnrollmentView["effectiveStatus"],
    serverNow: stringAtauNull("server_now"),
    submissionStatus: stringAtauNull("submission_status"),
    submissionId: stringAtauNull("submission_id"),
    consentState: consent_state as ConsentState,
    identityState: identity_state,
    templateReady: template_ready,
    submittedAt: stringAtauNull("submitted_at"),
    reviewedAt: stringAtauNull("reviewed_at"),
    processingDeadline: stringAtauNull("processing_deadline"),
    reasonCode: stringAtauNull("reason_code"),
    deletionState: stringAtauNull("deletion_state") ?? "NONE",
    revision,
    allowedActions: allowed_actions as string[],
    photoAvailable: photo_available,
    photoExpiresAt: stringAtauNull("photo_expires_at"),
    syntheticOnly: true,
  };
}

function parseReceiptConsent(teks: string): ConsentView | null {
  const data = bacaData(teks);
  if (!adalahObjek(data)) return null;
  return parseConsent(data.current);
}

function parseReceiptEnrollment(teks: string): EnrollmentView | null {
  const data = bacaData(teks);
  if (!adalahObjek(data)) return null;
  return parseEnrollment(data.current);
}

export async function bacaConsent(
  token: string,
  signal?: AbortSignal
): Promise<HasilEnrollment<ConsentEnvelope>> {
  const alamat = alamatBackend();
  if (!alamat.ok) return alamat;
  if (signal?.aborted) return { ok: false, error: { kind: "cancelled" } };
  const jawaban = await mintaHttp("/api/v1/me/consent", {
    baseUrl: alamat.data,
    token,
    timeoutMs: TIMEOUT_ENROLLMENT_MS,
    signal,
  });
  if (signal?.aborted) return { ok: false, error: { kind: "cancelled" } };
  if (jawaban.jenis !== "jawaban") return { ok: false, error: galatTransport(jawaban) };
  if (jawaban.status !== 200) return { ok: false, error: galatHttp(jawaban) };
  const consent = parseConsent(bacaData(jawaban.teks));
  if (consent === null || jawaban.etag === null || jawaban.etag === undefined || jawaban.etag === "") {
    return { ok: false, error: { kind: "unknown", code: "BENTUK_CONSENT" } };
  }
  return { ok: true, data: { consent, etag: jawaban.etag } };
}

export async function bacaStatus(
  token: string,
  signal?: AbortSignal
): Promise<HasilEnrollment<EnrollmentView>> {
  const alamat = alamatBackend();
  if (!alamat.ok) return alamat;
  if (signal?.aborted) return { ok: false, error: { kind: "cancelled" } };
  const jawaban = await mintaHttp("/api/v1/me/face-enrollment", {
    baseUrl: alamat.data,
    token,
    timeoutMs: TIMEOUT_ENROLLMENT_MS,
    signal,
  });
  if (signal?.aborted) return { ok: false, error: { kind: "cancelled" } };
  if (jawaban.jenis !== "jawaban") return { ok: false, error: galatTransport(jawaban) };
  if (jawaban.status !== 200) return { ok: false, error: galatHttp(jawaban) };
  const view = parseEnrollment(bacaData(jawaban.teks));
  if (view === null) {
    return { ok: false, error: { kind: "unknown", code: "BENTUK_STATUS" } };
  }
  return { ok: true, data: view };
}

export async function beriConsent(
  token: string,
  noticeVersion: string,
  ifMatch: string,
  replacesConsentId = "",
  signal?: AbortSignal
): Promise<HasilEnrollment<ConsentView>> {
  const alamat = alamatBackend();
  if (!alamat.ok) return alamat;
  if (ifMatch === "") return { ok: false, error: { kind: "precondition" } };
  const jawaban = await mintaHttp("/api/v1/me/consent", {
    baseUrl: alamat.data,
    method: "POST",
    token,
    body: {
      notice_version: noticeVersion,
      accepted: true,
      ...(replacesConsentId === "" ? {} : { replaces_consent_id: replacesConsentId }),
    },
    headers: { "If-Match": ifMatch, "Idempotency-Key": kunciIdempotensi() },
    timeoutMs: TIMEOUT_ENROLLMENT_MS,
    signal,
  });
  if (signal?.aborted) return { ok: false, error: { kind: "cancelled" } };
  if (jawaban.jenis !== "jawaban") return { ok: false, error: galatTransport(jawaban) };
  if (jawaban.status !== 201 && jawaban.status !== 200) {
    return { ok: false, error: galatHttp(jawaban) };
  }
  const consent = parseReceiptConsent(jawaban.teks);
  if (consent === null) {
    return { ok: false, error: { kind: "unknown", code: "BENTUK_GRANT" } };
  }
  return { ok: true, data: consent };
}

export async function tarikConsent(
  token: string,
  signal?: AbortSignal
): Promise<HasilEnrollment<EnrollmentView>> {
  const alamat = alamatBackend();
  if (!alamat.ok) return alamat;
  const jawaban = await mintaHttp("/api/v1/me/consent/withdraw", {
    baseUrl: alamat.data,
    method: "POST",
    token,
    body: { scope: "ALL_FACE_ENROLLMENT", confirmed: true },
    headers: { "Idempotency-Key": kunciIdempotensi() },
    timeoutMs: TIMEOUT_ENROLLMENT_MS,
    signal,
  });
  if (signal?.aborted) return { ok: false, error: { kind: "cancelled" } };
  if (jawaban.jenis !== "jawaban") return { ok: false, error: galatTransport(jawaban) };
  if (jawaban.status !== 200) return { ok: false, error: galatHttp(jawaban) };
  const view = parseReceiptEnrollment(jawaban.teks);
  if (view === null) {
    return { ok: false, error: { kind: "unknown", code: "BENTUK_WITHDRAW" } };
  }
  return { ok: true, data: view };
}

export async function kirimFotoSintetis(
  token: string,
  consentId: string,
  jpeg: Uint8Array,
  signal?: AbortSignal
): Promise<HasilEnrollment<EnrollmentView>> {
  const alamat = alamatBackend();
  if (!alamat.ok) return alamat;
  if (consentId === "" || jpeg.length === 0) {
    return { ok: false, error: { kind: "validation" } };
  }
  const jawaban = await mintaHttp("/api/v1/me/face-enrollment-submissions", {
    baseUrl: alamat.data,
    method: "POST",
    token,
    bodyBytes: jpeg,
    contentType: "image/jpeg",
    headers: {
      "X-Consent-Id": consentId,
      "Idempotency-Key": kunciIdempotensi(),
    },
    timeoutMs: TIMEOUT_ENROLLMENT_MS,
    signal,
  });
  if (signal?.aborted) return { ok: false, error: { kind: "cancelled" } };
  if (jawaban.jenis !== "jawaban") return { ok: false, error: galatTransport(jawaban) };
  if (jawaban.status !== 201 && jawaban.status !== 200) {
    return { ok: false, error: galatHttp(jawaban) };
  }
  const view = parseReceiptEnrollment(jawaban.teks);
  if (view === null) {
    return { ok: false, error: { kind: "unknown", code: "BENTUK_SUBMIT" } };
  }
  return { ok: true, data: view };
}

/** UUIDv4-shaped idempotency key; never a secret and never logged. */
export function kunciIdempotensi(): string {
  const bytes = new Uint8Array(16);
  const crypto = globalThis.crypto;
  if (crypto && typeof crypto.getRandomValues === "function") {
    crypto.getRandomValues(bytes);
  } else {
    for (let i = 0; i < bytes.length; i++) {
      bytes[i] = Math.floor(Math.random() * 256);
    }
  }
  bytes[6] = (bytes[6]! & 0x0f) | 0x40;
  bytes[8] = (bytes[8]! & 0x3f) | 0x80;
  const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

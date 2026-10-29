import { adalahObjek } from "@/features/api/http";
import type { ConsentView, EnrollmentError, EnrollmentView, HasilEnrollment } from "../enrollment/model";
import { labelEnrollment } from "../enrollment/model";

export type ClassRef = { id: string; code: string; title: string };
export type AdminStudent = { userId: string; personId: string; displayName: string; externalCode: string; classes: ClassRef[] };
export type IdentityBinding = {
  accountUserId: string; personId: string; externalCode: string;
  accountStatus: string; accountRole: string; personStatus: string;
  identityReference: string | null; identityVerifiedAt: string | null;
  eligible: boolean; classes: ClassRef[];
};
export type AdminReview = {
  reviewerAdminId: string; decision: "APPROVED" | "REJECTED"; at: string;
  identityCheckMethod: "CAMPUS_AUTHORIZED_CHECK" | null;
  campusCheckRef: string | null;
};
export type AdminHistoryItem = { auditId: string; action: string; actorId: string | null; time: string; reasonCode: string | null };
export type AdminDetail = EnrollmentView & {
  student: AdminStudent; consent: ConsentView; identityBinding: IdentityBinding;
  artifactIntegrity: "NOT_CHECKED" | "UNAVAILABLE"; serverNow: string;
  review: AdminReview | null; history: AdminHistoryItem[];
};
export type QueueItem = { id: string; student: AdminStudent; status: EnrollmentView["effectiveStatus"]; submittedAt: string; photoExpiresAt: string | null };
export type QueuePage = { items: QueueItem[]; nextCursor: string | null; hasMore: boolean; pendingCount: number; serverNow: string };
export type DetailEnvelope = { view: AdminDetail; etag: string };
export type DecisionResult = DetailEnvelope & { httpStatus: number };
export type Decision =
  | { kind: "approve"; identityCheckMethod: "CAMPUS_AUTHORIZED_CHECK"; campusCheckRef: string; confirmed: true }
  | { kind: "reject"; reasonCode: "IDENTITY_MISMATCH" | "EVIDENCE_INSUFFICIENT" | "UNSUITABLE_SUBMISSION"; confirmed: true }
  | { kind: "retry"; confirmed: true };
export type Operation = { idempotencyKey: string; ifMatch: string; decision: Decision };
export type AdminApi = {
  list(token: string, status: string, cursor: string | null, signal?: AbortSignal): Promise<HasilEnrollment<QueuePage>>;
  detail(token: string, id: string, signal?: AbortSignal): Promise<HasilEnrollment<DetailEnvelope>>;
  decide(token: string, id: string, operation: Operation, signal?: AbortSignal): Promise<HasilEnrollment<DecisionResult>>;
  photo(token: string, id: string, signal?: AbortSignal): Promise<HasilEnrollment<Uint8Array>>;
};
export type AdminReviewState = {
  screen: "queue" | "detail"; selectedId: string | null; items: QueueItem[];
  pendingCount: number; nextCursor: string | null; hasMore: boolean; filter: string;
  detail: AdminDetail | null; etag: string | null; busy: "queue" | "detail" | "decision" | null;
  error: EnrollmentError | null; denied: boolean; stale: boolean; uncertain: boolean; lastResponseStatus: number | null;
};

export const UUID_V4 = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
export const STATUSES = ["NOT_ENROLLED", "PENDING_APPROVAL", "APPROVED_PROCESSING", "ACTIVE", "REJECTED", "FAILED_PROCESSING", "EXPIRED", "REVOKED"] as const;
const text = (v: unknown): v is string => typeof v === "string" && v.trim().length > 0;
const natural = (v: unknown): v is number => typeof v === "number" && Number.isSafeInteger(v) && v >= 0;
export const timestamp = (v: unknown): v is string => typeof v === "string" && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/.test(v) && Number.isFinite(Date.parse(v));
const optionalTime = (v: unknown) => v === undefined || v === null || timestamp(v);
const optionalText = (v: unknown) => v === undefined || v === null || text(v);
const nullable = (v: unknown): string | null => typeof v === "string" ? v : null;
export function strongEtag(etag: unknown, revision?: number): etag is string {
  return typeof etag === "string" && /^"(?:0|[1-9]\d*)"$/.test(etag) && Number.isSafeInteger(Number(etag.slice(1, -1))) && (revision === undefined || etag === `"${revision}"`);
}
function classes(v: unknown): ClassRef[] | null {
  if (!Array.isArray(v)) return null;
  const result: ClassRef[] = [];
  for (const item of v) {
    if (!adalahObjek(item) || typeof item.id !== "string" || !UUID_V4.test(item.id) || !text(item.code) || !text(item.title) || result.some(c => c.id === item.id)) return null;
    result.push({ id: item.id, code: item.code, title: item.title });
  }
  return result;
}
function student(v: unknown): AdminStudent | null {
  if (!adalahObjek(v)) return null;
  const roster = classes(v.classes);
  // An unlinked person is reported with an empty account id, never a fabricated one.
  if (roster === null || typeof v.user_id !== "string" || (v.user_id !== "" && !UUID_V4.test(v.user_id)) || typeof v.person_id !== "string" || !UUID_V4.test(v.person_id) || !text(v.display_name) || !text(v.external_code)) return null;
  return { userId: v.user_id, personId: v.person_id, displayName: v.display_name, externalCode: v.external_code, classes: roster };
}
function consent(v: unknown): ConsentView | null {
  if (!adalahObjek(v) || !["NOT_GIVEN", "ACTIVE", "WITHDRAWN"].includes(String(v.state)) || !natural(v.revision) || !adalahObjek(v.notice) || v.notice.version !== "synthetic-v1" || typeof v.notice.digest !== "string" || !/^[0-9a-f]{64}$/.test(v.notice.digest) || !text(v.notice.text) || !optionalTime(v.granted_at) || !optionalTime(v.withdrawn_at) || !(v.record_id === undefined || v.record_id === null || (typeof v.record_id === "string" && UUID_V4.test(v.record_id)))) return null;
  if (v.state === "ACTIVE" && (!text(v.record_id) || !timestamp(v.granted_at) || v.withdrawn_at != null)) return null;
  return { state: v.state as ConsentView["state"], recordId: nullable(v.record_id), notice: { version: v.notice.version, digest: v.notice.digest, text: v.notice.text }, grantedAt: nullable(v.granted_at), withdrawnAt: nullable(v.withdrawn_at), revision: v.revision };
}
function reviewHistory(value: Record<string, unknown>): { review: AdminReview | null; history: AdminHistoryItem[] } | null {
  let review: AdminReview | null = null;
  if (value.review !== undefined && value.review !== null) {
    const r = value.review;
    if (!adalahObjek(r) || typeof r.reviewer_admin_id !== "string" || !UUID_V4.test(r.reviewer_admin_id) || (r.decision !== "APPROVED" && r.decision !== "REJECTED") || !timestamp(r.at)) return null;
    const method = r.identity_check_method == null ? null : r.identity_check_method;
    const ref = r.campus_check_ref == null ? null : r.campus_check_ref;
    if (method !== null && method !== "CAMPUS_AUTHORIZED_CHECK") return null;
    if (ref !== null && (typeof ref !== "string" || !UUID_V4.test(ref))) return null;
    if (r.decision === "APPROVED" && (method === null || ref === null)) return null;
    review = { reviewerAdminId: r.reviewer_admin_id, decision: r.decision, at: r.at, identityCheckMethod: method, campusCheckRef: ref as string | null };
  }
  const history: AdminHistoryItem[] = [];
  if (value.history !== undefined) {
    if (!Array.isArray(value.history) || value.history.length > 50) return null;
    for (const row of value.history) {
      if (!adalahObjek(row) || typeof row.audit_id !== "string" || !UUID_V4.test(row.audit_id) || typeof row.action !== "string" || row.action.length > 80 || !/^[a-z]+(?:[._][a-z]+)*$/.test(row.action) || !timestamp(row.time) || history.some(item => item.auditId === row.audit_id)) return null;
      if (row.actor_id != null && (typeof row.actor_id !== "string" || !UUID_V4.test(row.actor_id))) return null;
      if (row.reason_code != null && (typeof row.reason_code !== "string" || !/^[A-Z][A-Z0-9_]{0,79}$/.test(row.reason_code))) return null;
      history.push({ auditId: row.audit_id, action: row.action, actorId: nullable(row.actor_id), time: row.time, reasonCode: nullable(row.reason_code) });
    }
  }
  return { review, history };
}
export function parseAdminDetail(v: unknown, targetId: string): AdminDetail | null {
  if (!adalahObjek(v) || !UUID_V4.test(targetId) || v.submission_id !== targetId || v.synthetic_only !== true || !timestamp(v.server_now) || !STATUSES.includes(v.effective_status as typeof STATUSES[number]) || !natural(v.revision) || typeof v.template_ready !== "boolean" || typeof v.photo_available !== "boolean" || !text(v.identity_state) || !text(v.deletion_state) || !Array.isArray(v.allowed_actions) || !v.allowed_actions.every(text)) return null;
  const subject = student(v.student), permission = consent(v.consent), b = v.identity_binding;
  if (!subject || !permission || permission.state !== v.consent_state || !adalahObjek(b) || b.account_user_id !== subject.userId || b.person_id !== subject.personId || b.external_code !== subject.externalCode || typeof b.eligible !== "boolean" || !(b.identity_reference === "" || optionalText(b.identity_reference)) || !(b.identity_verified_at === "" || optionalTime(b.identity_verified_at))) return null;
  const linked = b.account_user_id !== "";
  if (linked && (!text(b.account_status) || !text(b.account_role) || !text(b.person_status))) return null;
  if (!linked && (b.account_status !== "" || b.account_role !== "")) return null;
  const roster = classes(b.classes);
  if (!roster || JSON.stringify(roster) !== JSON.stringify(subject.classes) || (v.artifact_integrity !== "NOT_CHECKED" && v.artifact_integrity !== "UNAVAILABLE")) return null;
  for (const key of ["submitted_at", "reviewed_at", "processing_deadline", "photo_expires_at"]) if (!optionalTime(v[key])) return null;
  if (!timestamp(v.submitted_at) || !optionalText(v.submission_status) || !optionalText(v.reason_code) || (v.photo_available && !timestamp(v.photo_expires_at))) return null;
  if (b.eligible && (!linked || b.account_status !== "active" || b.account_role !== "mahasiswa" || b.person_status !== "active" || !text(b.identity_reference) || !timestamp(b.identity_verified_at) || roster.length === 0)) return null;
  const audit = reviewHistory(v);
  if (audit === null) return null;
  return {
    effectiveStatus: v.effective_status as AdminDetail["effectiveStatus"], submissionStatus: nullable(v.submission_status), submissionId: targetId,
    consentState: permission.state, identityState: v.identity_state, templateReady: v.template_ready,
    submittedAt: v.submitted_at, reviewedAt: nullable(v.reviewed_at), processingDeadline: nullable(v.processing_deadline), reasonCode: nullable(v.reason_code), deletionState: v.deletion_state,
    revision: v.revision, allowedActions: [...v.allowed_actions] as string[], photoAvailable: v.photo_available, photoExpiresAt: nullable(v.photo_expires_at), syntheticOnly: true,
    student: subject, consent: permission, identityBinding: { accountUserId: b.account_user_id as string, personId: b.person_id as string, externalCode: b.external_code as string, accountStatus: b.account_status as string, accountRole: b.account_role as string, personStatus: b.person_status as string, identityReference: b.identity_reference === "" ? null : nullable(b.identity_reference), identityVerifiedAt: b.identity_verified_at === "" ? null : nullable(b.identity_verified_at), eligible: b.eligible, classes: roster }, artifactIntegrity: v.artifact_integrity, serverNow: v.server_now,
    review: audit.review, history: audit.history,
  };
}
/** The list wire envelope deliberately places pagination beside data. */
export function parseQueuePage(v: unknown): QueuePage | null {
  if (!adalahObjek(v) || !Array.isArray(v.data) || !adalahObjek(v.page) || !adalahObjek(v.summary) || !timestamp(v.server_now) || typeof v.page.has_more !== "boolean" || !natural(v.summary.pending_count) || !optionalText(v.page.next_cursor)) return null;
  const items: QueueItem[] = [];
  for (const row of v.data) {
    if (!adalahObjek(row) || typeof row.id !== "string" || !UUID_V4.test(row.id) || !STATUSES.includes(row.status as typeof STATUSES[number]) || row.status === "NOT_ENROLLED" || !timestamp(row.submitted_at) || !optionalTime(row.photo_expires_at) || items.some(item => item.id === row.id)) return null;
    const subject = student(row.student);
    if (!subject) return null;
    items.push({ id: row.id, student: subject, status: row.status as QueueItem["status"], submittedAt: row.submitted_at, photoExpiresAt: nullable(row.photo_expires_at) });
  }
  const nextCursor = nullable(v.page.next_cursor);
  if (v.page.has_more && !nextCursor) return null;
  return { items, nextCursor, hasMore: v.page.has_more, pendingCount: v.summary.pending_count, serverNow: v.server_now };
}
export function validDecision(d: Decision): boolean {
  if (d.confirmed !== true) return false;
  if (d.kind === "retry") return true;
  if (d.kind === "approve") return d.identityCheckMethod === "CAMPUS_AUTHORIZED_CHECK" && UUID_V4.test(d.campusCheckRef);
  return d.kind === "reject" && ["IDENTITY_MISMATCH", "EVIDENCE_INSUFFICIENT", "UNSUITABLE_SUBMISSION"].includes(d.reasonCode);
}
function reviewable(v: AdminDetail | null, now: number, stale: boolean): v is AdminDetail {
  return !!v && !stale && Number.isFinite(now) && v.syntheticOnly && v.consentState === "ACTIVE" && v.consent.state === "ACTIVE" && v.identityBinding.eligible && v.photoAvailable && v.photoExpiresAt !== null && Date.parse(v.photoExpiresAt) > now;
}
export function canApprove(v: AdminDetail | null, now: number, stale: boolean): boolean { return reviewable(v, now, stale) && v.effectiveStatus === "PENDING_APPROVAL" && v.allowedActions.includes("APPROVE"); }
export function canReject(v: AdminDetail | null, now: number, stale: boolean): boolean { return reviewable(v, now, stale) && v.effectiveStatus === "PENDING_APPROVAL" && v.allowedActions.includes("REJECT"); }
export function canRetry(v: AdminDetail | null, now: number, stale: boolean): boolean { return reviewable(v, now, stale) && v.effectiveStatus === "FAILED_PROCESSING" && v.allowedActions.includes("RETRY_PROCESSING"); }
export function statusLabel(v: AdminDetail | null, trusted = true): string {
  if (v?.effectiveStatus === "ACTIVE" && (!trusted || !v.templateReady || v.consentState !== "ACTIVE" || v.consent.state !== "ACTIVE")) return "Aktif (belum terverifikasi saat ini)";
  return labelEnrollment(v);
}

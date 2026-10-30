import type { AdminDetail, Decision, DetailEnvelope } from "@/features/adminEnrollment/model";
export const ID = "22222222-2222-4222-8222-222222222222";
export const OTHER_ID = "33333333-3333-4333-8333-333333333333";
export const USER_ID = "11111111-1111-4111-8111-111111111111";
export const PERSON_ID = "44444444-4444-4444-8444-444444444444";
export const CLASS_ID = "55555555-5555-4555-8555-555555555555";
export const SERVER_NOW = "2026-10-26T10:00:00.000Z";
export const APPROVE: Decision = { kind: "approve", identityCheckMethod: "CAMPUS_AUTHORIZED_CHECK", campusCheckRef: OTHER_ID, confirmed: true };
export function detail(patch: Partial<AdminDetail> = {}): AdminDetail {
  const classes = [{ id: CLASS_ID, code: "SYN101", title: "Synthetic class" }];
  return {
    effectiveStatus: "PENDING_APPROVAL", submissionStatus: "PENDING_APPROVAL", submissionId: ID, consentState: "ACTIVE", identityState: "UNREVIEWED", templateReady: false,
    submittedAt: SERVER_NOW, reviewedAt: null, processingDeadline: null, reasonCode: null, deletionState: "NONE", revision: 1, allowedActions: ["APPROVE", "REJECT"], photoAvailable: true, photoExpiresAt: "2026-10-29T10:00:00.000Z", syntheticOnly: true,
    student: { userId: USER_ID, personId: PERSON_ID, displayName: "Synthetic student", externalCode: "SYN-001", classes },
    consent: { state: "ACTIVE", recordId: OTHER_ID, notice: { version: "synthetic-v1", digest: "b".repeat(64), text: "Synthetic notice" }, grantedAt: SERVER_NOW, withdrawnAt: null, revision: 1 },
    identityBinding: { accountUserId: USER_ID, personId: PERSON_ID, externalCode: "SYN-001", accountStatus: "active", accountRole: "mahasiswa", personStatus: "active", identityReference: "synthetic-roster", identityVerifiedAt: SERVER_NOW, eligible: true, classes }, artifactIntegrity: "NOT_CHECKED", serverNow: SERVER_NOW,
    review: null, history: [],
    ...patch,
  };
}
export function envelope(patch: Partial<AdminDetail> = {}): DetailEnvelope {
  const view = detail(patch);
  return { view, etag: `"${view.revision}"` };
}
export function wireDetail(patch: Record<string, unknown> = {}): Record<string, unknown> {
  const d = detail();
  return {
    effective_status: d.effectiveStatus, submission_status: d.submissionStatus, submission_id: d.submissionId, consent_state: d.consentState, identity_state: d.identityState, template_ready: d.templateReady,
    submitted_at: d.submittedAt, deletion_state: d.deletionState, revision: d.revision, allowed_actions: d.allowedActions, photo_available: true, photo_expires_at: d.photoExpiresAt, synthetic_only: true,
    student: { user_id: USER_ID, person_id: PERSON_ID, display_name: d.student.displayName, external_code: d.student.externalCode, classes: d.student.classes },
    consent: { state: "ACTIVE", record_id: OTHER_ID, notice: d.consent.notice, granted_at: SERVER_NOW, revision: 1 },
    identity_binding: { account_user_id: USER_ID, person_id: PERSON_ID, external_code: d.student.externalCode, account_status: "active", account_role: "mahasiswa", person_status: "active", identity_reference: "synthetic-roster", identity_verified_at: SERVER_NOW, eligible: true, classes: d.student.classes },
    artifact_integrity: "NOT_CHECKED", server_now: SERVER_NOW, ...patch,
  };
}
export function deferred<T>() {
  return Promise.withResolvers<T>();
}

import { afterEach, describe, expect, it, vi } from "vitest";
import { parseAdminDetail, parseQueuePage } from "@/features/adminEnrollment/model";
import { APPROVE, ID, OTHER_ID, SERVER_NOW, USER_ID, wireDetail } from "./adminEnrollmentFixtures";

async function loadApi() {
  vi.resetModules();
  vi.stubEnv("EXPO_PUBLIC_FACELINK_API_BASE_URL", "https://admin-stage.invalid");
  vi.stubGlobal("__DEV__", false);
  return import("@/features/adminEnrollment/api");
}
function json(data: unknown, status = 200, etag = '"1"') {
  return new Response(JSON.stringify(data), { status, headers: { "Content-Type": "application/json", ETag: etag } });
}
const operation = { idempotencyKey: OTHER_ID, ifMatch: '"1"', decision: APPROVE };
const jpeg = new Uint8Array([0xff, 0xd8, 0xff, 0xd9]);
function photoResponse(bytes: Uint8Array = jpeg, headers: Record<string, string> = {}) {
  return new Response(bytes as unknown as BodyInit, { status: 200, headers: { "Content-Type": "image/jpeg", "Cache-Control": "no-store", "Content-Length": String(bytes.length), ...headers } });
}
afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
  vi.restoreAllMocks();
});

describe("strict synthetic admin DTO parsing", () => {
  it("maps camelCase identities and consent without retaining raw response fields", () => {
    const parsed = parseAdminDetail(wireDetail({ photo_base64: "sensitive" }), ID);
    expect(parsed).toMatchObject({ submissionId: ID, student: { userId: USER_ID, classes: [{ code: "SYN101" }] }, identityBinding: { eligible: true, accountRole: "mahasiswa" }, serverNow: SERVER_NOW, consent: { state: "ACTIVE" }, artifactIntegrity: "NOT_CHECKED" });
    expect(JSON.stringify(parsed)).not.toContain("sensitive");
    expect(parsed).not.toHaveProperty("identity_binding");
  });
  it("limits reviewer/audit DTOs to validated camelCase fields without exposing metadata", () => {
    const review = { reviewer_admin_id: USER_ID, decision: "APPROVED", at: SERVER_NOW, identity_check_method: "CAMPUS_AUTHORIZED_CHECK", campus_check_ref: OTHER_ID };
    const event = { audit_id: ID, action: "enrollment.approved", actor_id: USER_ID, time: SERVER_NOW, reason_code: "SYNTHETIC_ONLY", metadata: { photo: "private-bytes" } };
    expect(parseAdminDetail(wireDetail(), ID)).toMatchObject({ review: null, history: [] });
    const parsed = parseAdminDetail(wireDetail({ review, history: [event] }), ID);
    expect(parsed).toMatchObject({ review: { reviewerAdminId: USER_ID, decision: "APPROVED", campusCheckRef: OTHER_ID }, history: [{ auditId: ID, action: "enrollment.approved", actorId: USER_ID, time: SERVER_NOW, reasonCode: "SYNTHETIC_ONLY" }] });
    expect(JSON.stringify(parsed)).not.toContain("private-bytes");
    expect(parseAdminDetail(wireDetail({ review: { ...review, reviewer_admin_id: "bad" } }), ID)).toBeNull();
    expect(parseAdminDetail(wireDetail({ review: { ...review, at: "bad" } }), ID)).toBeNull();
    expect(parseAdminDetail(wireDetail({ history: Array.from({ length: 51 }, () => event) }), ID)).toBeNull();
    expect(parseAdminDetail(wireDetail({ history: [event, event] }), ID)).toBeNull();
    expect(parseAdminDetail(wireDetail({ history: [{ ...event, action: "<script>" }] }), ID)).toBeNull();
    expect(parseAdminDetail(wireDetail({ history: [{ ...event, reason_code: "private body text" }] }), ID)).toBeNull();
  });
  it.each([
    { synthetic_only: false }, { synthetic_only: undefined }, { server_now: "not-a-time" },
    { submission_id: OTHER_ID }, { effective_status: "NEEDS_IDENTITY_VERIFICATION" },
    { photo_expires_at: "yesterday" }, { revision: 1.5 }, { allowed_actions: [123] },
    { artifact_integrity: "VERIFIED" }, { consent_state: "WITHDRAWN" },
  ])("rejects malformed or non-synthetic detail: %j", patch => {
    expect(parseAdminDetail(wireDetail(patch), ID)).toBeNull();
  });
  it("opens an unlinked submission without inventing an account identity", () => {
    const raw = wireDetail();
    raw.student = { ...(raw.student as Record<string, unknown>), user_id: "" };
    raw.identity_binding = { ...(raw.identity_binding as Record<string, unknown>), account_user_id: "", account_status: "", account_role: "", identity_reference: "", identity_verified_at: "", eligible: false };
    raw.photo_available = false;
    const parsed = parseAdminDetail(raw, ID);
    expect(parsed).toMatchObject({ student: { userId: "" }, identityBinding: { accountUserId: "", accountStatus: "", eligible: false, identityReference: null } });
    expect(parsed?.photoAvailable).toBe(false);
  });
  it("rejects identity linkage, eligible role and roster mismatches", () => {
    const raw = wireDetail();
    const binding = raw.identity_binding as Record<string, unknown>;
    for (const patch of [{ account_user_id: OTHER_ID }, { person_id: OTHER_ID }, { external_code: "wrong" }, { account_role: "admin" }, { classes: [] }, { identity_verified_at: null }]) {
      expect(parseAdminDetail({ ...raw, identity_binding: { ...binding, ...patch } }, ID)).toBeNull();
    }
  });
  it("parses the actual list envelope and refuses cursor/status/duplicate corruption", () => {
    const raw = wireDetail();
    const item = { id: ID, student: raw.student, status: "PENDING_APPROVAL", submitted_at: SERVER_NOW, photo_expires_at: raw.photo_expires_at };
    const list = { data: [item], page: { next_cursor: "opaque+cursor", has_more: true }, summary: { pending_count: 1 }, server_now: SERVER_NOW };
    expect(parseQueuePage(list)).toMatchObject({ items: [{ id: ID, student: { userId: USER_ID } }], nextCursor: "opaque+cursor", pendingCount: 1, serverNow: SERVER_NOW });
    expect(parseQueuePage({ ...list, page: { has_more: true } })).toBeNull();
    expect(parseQueuePage({ ...list, data: [item, item] })).toBeNull();
    expect(parseQueuePage({ ...list, data: [{ ...item, status: "invented" }] })).toBeNull();
  });
});

describe("authenticated admin API", () => {
  it("encodes queue filters/cursors and rejects a response for the wrong filter", async () => {
    const row = { id: ID, student: wireDetail().student, status: "ACTIVE", submitted_at: SERVER_NOW };
    const fetcher = vi.fn(async () => json({ data: [row], page: { has_more: false }, summary: { pending_count: 0 }, server_now: SERVER_NOW }));
    vi.stubGlobal("fetch", fetcher);
    const { adminApi } = await loadApi();
    expect(await adminApi.list("secret", "ACTIVE", "opaque+/=")).toMatchObject({ ok: true, data: { items: [{ id: ID, status: "ACTIVE" }], hasMore: false } });
    const [url] = fetcher.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe("https://admin-stage.invalid/api/v1/face-enrollment-submissions?status=ACTIVE&cursor=opaque%2B%2F%3D");
    expect(await adminApi.list("secret", "PENDING_APPROVAL", null)).toMatchObject({ ok: false, error: { kind: "validation" } });
  });

  it("omits the empty status query for the all-status filter", async () => {
    const fetcher = vi.fn(async () => json({ data: [], page: { has_more: false }, summary: { pending_count: 0 }, server_now: SERVER_NOW }));
    vi.stubGlobal("fetch", fetcher);
    const { adminApi } = await loadApi();
    expect(await adminApi.list("secret", "", null)).toMatchObject({ ok: true, data: { items: [] } });
    expect(await adminApi.list("secret", "", "opaque+/=")).toMatchObject({ ok: true });
    expect((fetcher.mock.calls[0] as unknown as [string])[0]).toBe("https://admin-stage.invalid/api/v1/face-enrollment-submissions");
    expect((fetcher.mock.calls[1] as unknown as [string])[0]).toBe("https://admin-stage.invalid/api/v1/face-enrollment-submissions?cursor=opaque%2B%2F%3D");
  });

  it("validates detail target and revision-bound strong ETag", async () => {
    const fetcher = vi.fn().mockResolvedValueOnce(json({ data: wireDetail() })).mockResolvedValueOnce(json({ data: wireDetail() }, 200, 'W/"1"')).mockResolvedValueOnce(json({ data: wireDetail() }, 200, '"2"')).mockResolvedValueOnce(json({ data: wireDetail({ submission_id: OTHER_ID }) }));
    vi.stubGlobal("fetch", fetcher);
    const { adminApi } = await loadApi();
    expect(await adminApi.detail("secret", ID)).toMatchObject({ ok: true, data: { etag: '"1"', view: { submissionId: ID } } });
    for (let i = 0; i < 3; i++) expect(await adminApi.detail("secret", ID)).toMatchObject({ ok: false, error: { kind: "validation" } });
    const [url, options] = fetcher.mock.calls[0] as [string, RequestInit];
    expect(url).not.toContain("secret");
    expect(new Headers(options.headers).get("Authorization")).toBe("Bearer secret");
    expect(options.body).toBeUndefined();
  });

  it("sends exactly the agreed approve operation and preserves 202 processing", async () => {
    const fetcher = vi.fn(async () => json({ data: { operation_id: USER_ID, resource_id: ID, applied_revision: 2, current: wireDetail({ revision: 2, effective_status: "APPROVED_PROCESSING", template_ready: false }) } }, 202, '"2"'));
    vi.stubGlobal("fetch", fetcher);
    const { adminApi } = await loadApi();
    expect(await adminApi.decide("secret", ID, operation)).toMatchObject({ ok: true, data: { httpStatus: 202, view: { effectiveStatus: "APPROVED_PROCESSING", templateReady: false } } });
    const [url, options] = fetcher.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe(`https://admin-stage.invalid/api/v1/face-enrollment-submissions/${ID}/approve`);
    expect(new Headers(options.headers).get("If-Match")).toBe(operation.ifMatch);
    expect(new Headers(options.headers).get("Idempotency-Key")).toBe(operation.idempotencyKey);
    expect(JSON.parse(String(options.body))).toEqual({ identity_check_method: "CAMPUS_AUTHORIZED_CHECK", attestation_version: "synthetic-v1", campus_check_ref: OTHER_ID, confirmed: true });
  });

  it("sends actual reject and retry wire contracts without inventing fields", async () => {
    const receipt = (effectiveStatus: string) => ({ data: { operation_id: USER_ID, resource_id: ID, applied_revision: 2, current: wireDetail({ revision: 2, effective_status: effectiveStatus }) } });
    const fetcher = vi.fn().mockResolvedValueOnce(json(receipt("REJECTED"), 200, '"2"')).mockResolvedValueOnce(json(receipt("APPROVED_PROCESSING"), 202, '"2"'));
    vi.stubGlobal("fetch", fetcher);
    const { adminApi } = await loadApi();
    expect(await adminApi.decide("secret", ID, { ...operation, decision: { kind: "reject", reasonCode: "EVIDENCE_INSUFFICIENT", confirmed: true } })).toMatchObject({ ok: true, data: { httpStatus: 200 } });
    expect(await adminApi.decide("secret", ID, { ...operation, decision: { kind: "retry", confirmed: true } })).toMatchObject({ ok: true, data: { httpStatus: 202 } });
    const reject = fetcher.mock.calls[0] as [string, RequestInit], retry = fetcher.mock.calls[1] as [string, RequestInit];
    expect(reject[0]).toContain("/reject");
    expect(JSON.parse(String(reject[1].body))).toEqual({ reason_code: "EVIDENCE_INSUFFICIENT" });
    expect(retry[0]).toContain("/retry-processing");
    expect(JSON.parse(String(retry[1].body))).toEqual({ confirmed: true });
  });

  it("rejects mismatched receipts, fake 202 ACTIVE, invalid operation and missing consent", async () => {
    const receipt = { operation_id: USER_ID, resource_id: ID, applied_revision: 2, current: wireDetail({ revision: 2 }) };
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(json({ data: { ...receipt, resource_id: OTHER_ID } }, 200, '"2"')).mockResolvedValueOnce(json({ data: { ...receipt, current: wireDetail({ revision: 2, effective_status: "ACTIVE", template_ready: true }) } }, 202, '"2"')).mockResolvedValueOnce(json({ data: { ...receipt, current: wireDetail({ revision: 2, consent: undefined }) } }, 200, '"2"')));
    const { adminApi } = await loadApi();
    for (let i = 0; i < 3; i++) expect(await adminApi.decide("secret", ID, operation)).toMatchObject({ ok: false });
    expect(await adminApi.decide("secret", ID, { ...operation, ifMatch: 'W/"1"' })).toMatchObject({ ok: false, error: { kind: "validation" } });
  });

  it("retains actual replay status and CURRENT outcome, not the original approve intent", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => json({ data: { operation_id: USER_ID, resource_id: ID, applied_revision: 2, current: wireDetail({ revision: 5, effective_status: "REVOKED", template_ready: false, photo_available: false, consent_state: "WITHDRAWN", consent: { state: "WITHDRAWN", record_id: OTHER_ID, notice: (wireDetail().consent as Record<string, unknown>).notice, granted_at: SERVER_NOW, withdrawn_at: SERVER_NOW, revision: 2 } }) } }, 200, '"5"')));
    const { adminApi } = await loadApi();
    expect(await adminApi.decide("secret", ID, operation)).toMatchObject({ ok: true, data: { httpStatus: 200, view: { revision: 5, effectiveStatus: "REVOKED", consentState: "WITHDRAWN" } } });
  });

  it.each([[401, "sessionEnded"], [403, "forbidden"], [412, "precondition"], [410, "photoUnavailable"], [503, "unavailable"]] as const)("maps %i safely", async (status, kind) => {
    vi.stubGlobal("fetch", vi.fn(async () => json({ error: { code: "SAFE_CODE", message: "secret body detail" } }, status)));
    const { adminApi } = await loadApi();
    const result = await adminApi.detail("secret", ID);
    expect(result).toMatchObject({ ok: false, error: { kind, code: "SAFE_CODE" } });
    expect(JSON.stringify(result)).not.toContain("secret body detail");
  });

  it("bounds timeout and returns an ambiguous network outcome without generating another operation", async () => {
    vi.useFakeTimers();
    const fetcher = vi.fn((_url: string, options: RequestInit) => {
      const { promise, reject } = Promise.withResolvers<Response>();
      options.signal?.addEventListener("abort", () => reject(new Error("aborted")));
      return promise;
    });
    vi.stubGlobal("fetch", fetcher);
    const { adminApi } = await loadApi();
    const result = adminApi.decide("secret", ID, operation);
    await vi.advanceTimersByTimeAsync(20_000);
    expect(await result).toEqual({ ok: false, error: { kind: "network" } });
    expect(fetcher).toHaveBeenCalledTimes(1);
  });
});

describe("sensitive binary preview transport", () => {
  it("fetches authenticated header-only JPEG with no-store and never a token URL", async () => {
    const fetcher = vi.fn(async () => photoResponse());
    vi.stubGlobal("fetch", fetcher);
    const { adminApi } = await loadApi();
    expect(await adminApi.photo("private-token", ID)).toEqual({ ok: true, data: jpeg });
    const [url, options] = fetcher.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe(`https://admin-stage.invalid/api/v1/face-enrollment-submissions/${ID}/photo`);
    expect(url).not.toContain("private-token");
    expect(options).toMatchObject({ cache: "no-store", credentials: "omit", redirect: "error" });
    expect(new Headers(options.headers).get("Authorization")).toBe("Bearer private-token");
    expect(new Headers(options.headers).get("Cache-Control")).toBe("no-store");
  });

  it.each([{ "Content-Type": "text/html" }, { "Cache-Control": "private, max-age=30" }, { "Content-Length": "1048577" }, { "Content-Length": "2" }] as Record<string, string>[])("rejects unsafe preview headers before reading bytes: %j", async headers => {
    const response = photoResponse(jpeg, headers);
    const read = vi.spyOn(response.body!, "getReader");
    vi.stubGlobal("fetch", vi.fn(async () => response));
    const { adminApi } = await loadApi();
    expect(await adminApi.photo("secret", ID)).toMatchObject({ ok: false, error: { kind: "validation" } });
    expect(read).not.toHaveBeenCalled();
  });

  it("enforces streaming byte limit even without Content-Length", async () => {
    const response = photoResponse(new Uint8Array(1024 * 1024 + 1));
    response.headers.delete("Content-Length");
    vi.stubGlobal("fetch", vi.fn(async () => response));
    const { adminApi } = await loadApi();
    expect(await adminApi.photo("secret", ID)).toMatchObject({ ok: false, error: { kind: "validation" } });
  });

  it("rejects non-JPEG bytes and a lying length", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(photoResponse(new Uint8Array([1, 2, 3, 4]))).mockResolvedValueOnce(photoResponse(jpeg, { "Content-Length": "100" })));
    const { adminApi } = await loadApi();
    expect(await adminApi.photo("secret", ID)).toMatchObject({ ok: false });
    expect(await adminApi.photo("secret", ID)).toMatchObject({ ok: false });
  });

  it("checks abort after a body chunk instead of returning sensitive late bytes", async () => {
    const controller = new AbortController();
    const response = photoResponse();
    vi.spyOn(response.body!, "getReader").mockReturnValue({ read: async () => { controller.abort(); return { done: false, value: jpeg }; }, cancel: async () => {}, releaseLock: () => {}, closed: Promise.resolve() } as ReadableStreamDefaultReader<Uint8Array<ArrayBuffer>>);
    vi.stubGlobal("fetch", vi.fn(async () => response));
    const { adminApi } = await loadApi();
    expect(await adminApi.photo("secret", ID, controller.signal)).toEqual({ ok: false, error: { kind: "cancelled" } });
  });

  it("does not read or expose an unauthorized binary error body", async () => {
    const response = json({ error: { message: "private server body" } }, 403);
    const read = vi.spyOn(response, "text");
    vi.stubGlobal("fetch", vi.fn(async () => response));
    const { adminApi } = await loadApi();
    expect(await adminApi.photo("secret", ID)).toEqual({ ok: false, error: { kind: "forbidden" } });
    expect(read).not.toHaveBeenCalled();
  });
});

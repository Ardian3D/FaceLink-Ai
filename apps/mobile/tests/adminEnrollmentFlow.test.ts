import { describe, expect, it, vi } from "vitest";
import { AdminReviewFlow, canApprove, canRetry } from "@/features/adminEnrollment/flow";
import type { AdminApi, DecisionResult, DetailEnvelope, QueuePage } from "@/features/adminEnrollment/model";
import type { HasilEnrollment } from "@/features/enrollment/model";
import { statusTerpercaya } from "@/features/enrollment/model";
import { APPROVE, ID, OTHER_ID, SERVER_NOW, deferred, detail, envelope } from "./adminEnrollmentFixtures";
vi.hoisted(() => {
  vi.stubGlobal("__DEV__", false);
  vi.stubEnv("EXPO_PUBLIC_FACELINK_API_BASE_URL", "https://synthetic.invalid");
});

function setup() {
  let time = 1000;
  let token: string | null = "admin-session";
  const api = {
    list: vi.fn<AdminApi["list"]>().mockResolvedValue({ ok: true, data: { items: [], pendingCount: 0, hasMore: false, nextCursor: null, serverNow: SERVER_NOW } }),
    detail: vi.fn<AdminApi["detail"]>().mockResolvedValue({ ok: true, data: envelope() }),
    decide: vi.fn<AdminApi["decide"]>().mockResolvedValue({ ok: true, data: { ...envelope({ effectiveStatus: "APPROVED_PROCESSING", revision: 2, allowedActions: [] }), httpStatus: 202 } }),
    photo: vi.fn<AdminApi["photo"]>(),
  };
  const changed = vi.fn();
  const flow = new AdminReviewFlow(api, async () => token, changed, () => time);
  return { flow, api, changed, setTime: (value: number) => { time = value; }, setToken: (value: string | null) => { token = value; } };
}

describe("AdminReviewFlow backend authority and fencing", () => {
  it("uses server clock rather than the device epoch and fails closed at expiry without inventing EXPIRED", async () => {
    const s = setup();
    s.api.detail.mockResolvedValue({ ok: true, data: envelope({ photoExpiresAt: "2026-10-26T10:00:02.000Z" }) });
    await s.flow.open(ID);
    expect(s.flow.now()).toBe(Date.parse(SERVER_NOW));
    expect(canApprove(s.flow.snapshot().detail, s.flow.now(), s.flow.snapshot().stale)).toBe(true);
    s.setTime(3000);
    s.flow.tick();
    expect(s.flow.snapshot()).toMatchObject({ stale: true, detail: { effectiveStatus: "PENDING_APPROVAL" } });
    await s.flow.decide(APPROVE);
    expect(s.api.decide).not.toHaveBeenCalled();
  });

  it("marks snapshots stale after 30 seconds, local clock rollback, and backend rollback", async () => {
    const s = setup();
    await s.flow.open(ID);
    s.setTime(31_000);
    s.flow.tick();
    expect(s.flow.snapshot().stale).toBe(true);
    await s.flow.refresh();
    expect(s.flow.snapshot().stale).toBe(false);
    s.setTime(30_999);
    s.flow.tick();
    expect(s.flow.snapshot().stale).toBe(true);
    expect(Number.isNaN(s.flow.now())).toBe(true);
    const other = setup();
    await other.flow.open(ID);
    other.api.detail.mockResolvedValue({ ok: true, data: envelope({ serverNow: "2026-10-26T09:59:59.000Z" }) });
    await other.flow.refresh();
    expect(other.flow.snapshot()).toMatchObject({ stale: true, error: { kind: "precondition" } });
  });

  it("never claims ACTIVE from 202 and requires ready template plus active consent for an ACTIVE label", async () => {
    const s = setup();
    await s.flow.open(ID);
    await s.flow.decide(APPROVE);
    expect(s.flow.snapshot()).toMatchObject({ detail: { effectiveStatus: "APPROVED_PROCESSING", templateReady: false }, lastResponseStatus: 202, uncertain: false, busy: null });
    const active = detail({ effectiveStatus: "ACTIVE", templateReady: true });
    expect(statusTerpercaya(active, false, active.consent)).toBe(true);
    expect(statusTerpercaya({ ...active, templateReady: false }, false, active.consent)).toBe(false);
    expect(statusTerpercaya(active, true, active.consent)).toBe(false);
    expect(statusTerpercaya(active, false, { ...active.consent, state: "WITHDRAWN" })).toBe(false);
  });

  it("preflights decisions and stops when another reviewer changed the ETag", async () => {
    const s = setup();
    await s.flow.open(ID);
    s.api.detail.mockResolvedValue({ ok: true, data: envelope({ revision: 2, effectiveStatus: "REJECTED", allowedActions: [] }) });
    await s.flow.decide(APPROVE);
    expect(s.api.decide).not.toHaveBeenCalled();
    expect(s.flow.snapshot()).toMatchObject({ stale: true, detail: { effectiveStatus: "REJECTED" }, error: { kind: "precondition" } });
  });

  it("preflights current consent and identity eligibility rather than trusting previously allowed actions", async () => {
    for (const view of [
      detail({ consentState: "WITHDRAWN", consent: { ...detail().consent, state: "WITHDRAWN" } }),
      detail({ identityBinding: { ...detail().identityBinding, eligible: false } }),
    ]) {
      const s = setup();
      await s.flow.open(ID);
      s.api.detail.mockResolvedValueOnce({ ok: true, data: { view, etag: '"1"' } });
      await s.flow.decide(APPROVE);
      expect(s.api.decide).not.toHaveBeenCalled();
      expect(s.flow.snapshot()).toMatchObject({ stale: true, error: { kind: "precondition" } });
    }
  });

  it("does not accept wrong-target or older revision detail", async () => {
    const s = setup();
    s.api.detail.mockResolvedValue({ ok: true, data: envelope({ submissionId: OTHER_ID }) });
    await s.flow.open(ID);
    expect(s.flow.snapshot()).toMatchObject({ detail: null, stale: true, error: { kind: "precondition" } });
    s.api.detail.mockResolvedValue({ ok: true, data: envelope({ revision: 3 }) });
    await s.flow.refresh();
    s.api.detail.mockResolvedValue({ ok: true, data: envelope({ revision: 2 }) });
    await s.flow.refresh();
    expect(s.flow.snapshot()).toMatchObject({ detail: { revision: 3 }, stale: true });
    s.flow.suspend();
    await s.flow.refresh();
    expect(s.flow.snapshot()).toMatchObject({ selectedId: ID, detail: null, stale: true, error: { kind: "precondition" } });
    s.api.detail.mockResolvedValue({ ok: true, data: envelope({ revision: 4 }) });
    await s.flow.refresh();
    expect(s.flow.snapshot()).toMatchObject({ detail: { revision: 4 }, stale: false });
  });

  it("fences overlapping selections and ignores a late previous response", async () => {
    const s = setup(), first = deferred<HasilEnrollment<DetailEnvelope>>();
    s.api.detail.mockImplementationOnce(() => first.promise).mockResolvedValueOnce({ ok: true, data: envelope({ submissionId: OTHER_ID }) });
    const opening = s.flow.open(ID);
    // Start the first transport before changing selection.
    await vi.waitFor(() => expect(s.api.detail).toHaveBeenCalledTimes(1));
    await s.flow.open(OTHER_ID);
    first.resolve({ ok: true, data: envelope() });
    await opening;
    expect(s.flow.snapshot()).toMatchObject({ selectedId: OTHER_ID, detail: { submissionId: OTHER_ID } });
    expect(s.api.detail.mock.calls[0]?.[2]?.aborted).toBe(true);
  });

  it("blocks concurrent decisions during preflight and preserves original confirmed intent", async () => {
    const s = setup(), preflight = deferred<HasilEnrollment<DetailEnvelope>>();
    await s.flow.open(ID);
    s.api.detail.mockImplementationOnce(() => preflight.promise);
    const intent = { ...APPROVE };
    const deciding = s.flow.decide(intent);
    await vi.waitFor(() => expect(s.api.detail).toHaveBeenCalledTimes(2));
    if (intent.kind === "approve") intent.campusCheckRef = ID;
    await s.flow.decide({ kind: "reject", reasonCode: "IDENTITY_MISMATCH", confirmed: true });
    preflight.resolve({ ok: true, data: envelope() });
    await deciding;
    expect(s.api.decide).toHaveBeenCalledTimes(1);
    expect(s.api.decide.mock.calls[0]?.[2].decision).toEqual(APPROVE);
  });

  it("keeps exact operation after timeout, blocks opposite intent, and replays with original key, body and CAS", async () => {
    const s = setup();
    await s.flow.open(ID);
    s.api.decide.mockResolvedValueOnce({ ok: false, error: { kind: "network" } });
    await s.flow.decide(APPROVE);
    expect(s.flow.snapshot()).toMatchObject({ uncertain: true, stale: true, lastResponseStatus: null });
    const original = s.api.decide.mock.calls[0]?.[2];
    await s.flow.decide({ kind: "reject", reasonCode: "IDENTITY_MISMATCH", confirmed: true });
    expect(s.api.decide).toHaveBeenCalledTimes(1);
    s.api.decide.mockResolvedValueOnce({ ok: true, data: { ...envelope({ revision: 4, effectiveStatus: "REVOKED", allowedActions: [] }), httpStatus: 200 } });
    await s.flow.replay();
    expect(s.api.decide.mock.calls[1]?.[2]).toEqual(original);
    expect(s.flow.snapshot()).toMatchObject({ uncertain: false, detail: { effectiveStatus: "REVOKED", revision: 4 }, lastResponseStatus: 200 });
    expect(s.api.detail).toHaveBeenCalledTimes(2);
  });

  it("clears a server-rejected validation so the admin can correct after refresh", async () => {
    const s = setup();
    await s.flow.open(ID);
    s.api.decide.mockResolvedValueOnce({ ok: false, error: { kind: "validation" } });
    await s.flow.decide(APPROVE);
    expect(s.flow.snapshot()).toMatchObject({ uncertain: false, stale: true, error: { kind: "validation" } });
    await s.flow.refresh();
    await s.flow.decide({ kind: "reject", reasonCode: "EVIDENCE_INSUFFICIENT", confirmed: true });
    expect(s.api.decide).toHaveBeenCalledTimes(2);
  });

  it("fences a late mutation after selection change while retaining replay-only uncertainty", async () => {
    const s = setup(), mutation = deferred<HasilEnrollment<DecisionResult>>();
    await s.flow.open(ID);
    s.api.decide.mockImplementationOnce(() => mutation.promise);
    const deciding = s.flow.decide(APPROVE);
    await vi.waitFor(() => expect(s.api.decide).toHaveBeenCalledTimes(1));
    s.flow.back();
    mutation.resolve({ ok: true, data: { ...envelope({ revision: 2, effectiveStatus: "APPROVED_PROCESSING" }), httpStatus: 202 } });
    await deciding;
    expect(s.flow.snapshot()).toMatchObject({ screen: "queue", detail: null, uncertain: true, lastResponseStatus: null });
    await s.flow.open(ID);
    await s.flow.decide({ kind: "reject", reasonCode: "IDENTITY_MISMATCH", confirmed: true });
    expect(s.api.decide).toHaveBeenCalledTimes(1);
    await s.flow.replay();
    expect(s.api.decide).toHaveBeenCalledTimes(2);
  });

  it("suspends mid-decision, fences its late receipt and preserves exact replay across resume polling", async () => {
    const s = setup(), mutation = deferred<HasilEnrollment<DecisionResult>>();
    await s.flow.open(ID);
    s.api.decide.mockImplementationOnce(() => mutation.promise);
    const deciding = s.flow.decide(APPROVE);
    await vi.waitFor(() => expect(s.api.decide).toHaveBeenCalledTimes(1));
    const original = s.api.decide.mock.calls[0]?.[2];
    s.flow.suspend();
    expect(s.flow.snapshot()).toMatchObject({ screen: "detail", selectedId: ID, items: [], detail: null, etag: null, busy: null, stale: true, uncertain: true });
    mutation.resolve({ ok: true, data: { ...envelope({ revision: 2, effectiveStatus: "APPROVED_PROCESSING" }), httpStatus: 202 } });
    await deciding;
    expect(s.flow.snapshot()).toMatchObject({ detail: null, uncertain: true, lastResponseStatus: null });
    await s.flow.refresh();
    await s.flow.refresh();
    expect(s.flow.snapshot()).toMatchObject({ detail: { submissionId: ID }, uncertain: true });
    await s.flow.decide({ kind: "reject", reasonCode: "IDENTITY_MISMATCH", confirmed: true });
    expect(s.api.decide).toHaveBeenCalledTimes(1);
    await s.flow.replay();
    expect(s.api.decide.mock.calls[1]?.[2]).toEqual(original);
    expect(s.flow.snapshot()).toMatchObject({ uncertain: false, lastResponseStatus: 202 });
  });

  it.each(["sessionEnded", "forbidden"] as const)("clears sensitive queue/detail on %s and denies further loads", async kind => {
    const s = setup();
    await s.flow.open(ID);
    s.api.detail.mockResolvedValueOnce({ ok: false, error: { kind } });
    await s.flow.refresh();
    expect(s.flow.snapshot()).toMatchObject({ denied: true, items: [], detail: null, selectedId: null, etag: null, uncertain: false });
    const calls = s.api.detail.mock.calls.length;
    await s.flow.open(ID);
    expect(s.api.detail).toHaveBeenCalledTimes(calls);
  });

  it("discards in-flight detail when the session loses its credential", async () => {
    const s = setup(), response = deferred<HasilEnrollment<DetailEnvelope>>();
    s.api.detail.mockImplementationOnce(() => response.promise);
    const opening = s.flow.open(ID);
    await vi.waitFor(() => expect(s.api.detail).toHaveBeenCalledTimes(1));
    s.setToken(null);
    response.resolve({ ok: true, data: envelope() });
    await opening;
    expect(s.flow.snapshot()).toMatchObject({ denied: true, detail: null, items: [], error: { kind: "sessionEnded" } });
  });

  it("accepts a current-generation detail across normal bearer rotation", async () => {
    const s = setup(), response = deferred<HasilEnrollment<DetailEnvelope>>();
    s.api.detail.mockImplementationOnce(() => response.promise);
    const opening = s.flow.open(ID);
    await vi.waitFor(() => expect(s.api.detail).toHaveBeenCalledTimes(1));
    s.setToken("rotated-read-token");
    response.resolve({ ok: true, data: envelope() });
    await opening;
    expect(s.flow.snapshot()).toMatchObject({ denied: false, detail: { submissionId: ID }, stale: false });
  });

  it("uses the current credential after preflight rotation and accepts a receipt across further rotation", async () => {
    const s = setup(), preflight = deferred<HasilEnrollment<DetailEnvelope>>(), receipt = deferred<HasilEnrollment<DecisionResult>>();
    await s.flow.open(ID);
    s.api.detail.mockImplementationOnce(() => preflight.promise);
    s.api.decide.mockImplementationOnce(() => receipt.promise);
    const deciding = s.flow.decide(APPROVE);
    await vi.waitFor(() => expect(s.api.detail).toHaveBeenCalledTimes(2));
    s.setToken("rotated-before-send");
    preflight.resolve({ ok: true, data: envelope() });
    await vi.waitFor(() => expect(s.api.decide).toHaveBeenCalledTimes(1));
    expect(s.api.decide.mock.calls[0]?.[0]).toBe("rotated-before-send");
    s.setToken("rotated-during-send");
    receipt.resolve({ ok: true, data: { ...envelope({ revision: 2, effectiveStatus: "APPROVED_PROCESSING" }), httpStatus: 202 } });
    await deciding;
    expect(s.flow.snapshot()).toMatchObject({ denied: false, uncertain: false, lastResponseStatus: 202, detail: { revision: 2 } });
  });

  it("replays the identical uncertain operation under a rotated bearer and accepts a later rotation", async () => {
    const s = setup(), receipt = deferred<HasilEnrollment<DecisionResult>>();
    await s.flow.open(ID);
    s.api.decide.mockResolvedValueOnce({ ok: false, error: { kind: "network" } });
    await s.flow.decide(APPROVE);
    const original = s.api.decide.mock.calls[0]?.[2];
    s.setToken("rotated-replay-token");
    s.api.decide.mockImplementationOnce(() => receipt.promise);
    const replaying = s.flow.replay();
    await vi.waitFor(() => expect(s.api.decide).toHaveBeenCalledTimes(2));
    expect(s.api.decide.mock.calls[1]?.[0]).toBe("rotated-replay-token");
    expect(s.api.decide.mock.calls[1]?.[2]).toEqual(original);
    s.setToken("rotated-during-replay");
    receipt.resolve({ ok: true, data: { ...envelope({ revision: 3, effectiveStatus: "ACTIVE", templateReady: true }), httpStatus: 200 } });
    await replaying;
    expect(s.flow.snapshot()).toMatchObject({ denied: false, uncertain: false, lastResponseStatus: 200, detail: { revision: 3, effectiveStatus: "ACTIVE" } });
  });

  it("does not let periodic refresh abort or duplicate outstanding detail or queue reads", async () => {
    const s = setup(), response = deferred<HasilEnrollment<DetailEnvelope>>();
    s.api.detail.mockImplementationOnce(() => response.promise);
    const opening = s.flow.open(ID);
    await vi.waitFor(() => expect(s.api.detail).toHaveBeenCalledTimes(1));
    await s.flow.refresh();
    await s.flow.refresh();
    expect(s.api.detail).toHaveBeenCalledTimes(1);
    expect(s.api.detail.mock.calls[0]?.[2]?.aborted).toBe(false);
    response.resolve({ ok: true, data: envelope() });
    await opening;
    s.flow.back();
    const queue = deferred<HasilEnrollment<QueuePage>>();
    s.api.list.mockImplementationOnce(() => queue.promise);
    const loading = s.flow.load();
    await vi.waitFor(() => expect(s.api.list).toHaveBeenCalledTimes(1));
    await s.flow.refresh();
    expect(s.api.list).toHaveBeenCalledTimes(1);
    expect(s.api.list.mock.calls[0]?.[3]?.aborted).toBe(false);
    queue.resolve({ ok: true, data: { items: [], nextCursor: null, hasMore: false, pendingCount: 0, serverNow: SERVER_NOW } });
    await loading;
    expect(s.flow.snapshot()).toMatchObject({ busy: null, denied: false });
  });

  it("fences late responses after invalidate and dispose", async () => {
    for (const end of ["invalidate", "dispose"] as const) {
      const s = setup(), response = deferred<HasilEnrollment<DetailEnvelope>>();
      s.api.detail.mockImplementationOnce(() => response.promise);
      const opening = s.flow.open(ID);
      await vi.waitFor(() => expect(s.api.detail).toHaveBeenCalledTimes(1));
      s.flow[end]();
      const changes = s.changed.mock.calls.length;
      response.resolve({ ok: true, data: envelope() });
      await opening;
      expect(s.flow.snapshot().detail).toBeNull();
      expect(s.changed).toHaveBeenCalledTimes(changes);
    }
  });

  it("keeps failed queue loads authoritative and merges pagination without duplicates", async () => {
    const s = setup();
    const row = { id: ID, student: detail().student, status: "PENDING_APPROVAL" as const, submittedAt: SERVER_NOW, photoExpiresAt: detail().photoExpiresAt };
    const first: QueuePage = { items: [row], nextCursor: "cursor", hasMore: true, pendingCount: 2, serverNow: SERVER_NOW };
    s.api.list.mockResolvedValueOnce({ ok: true, data: first }).mockResolvedValueOnce({ ok: true, data: { ...first, items: [row, { ...row, id: OTHER_ID }], nextCursor: null, hasMore: false } });
    await s.flow.load();
    await s.flow.load("PENDING_APPROVAL", true);
    expect(s.flow.snapshot().items.map(item => item.id)).toEqual([ID, OTHER_ID]);
    s.api.list.mockResolvedValueOnce({ ok: false, error: { kind: "network" } });
    await s.flow.load();
    expect(s.flow.snapshot()).toMatchObject({ items: [], stale: true, error: { kind: "network" } });
  });

  it("allows retry only from the explicit backend action and valid photo deadline", () => {
    const now = Date.parse(SERVER_NOW), failed = detail({ effectiveStatus: "FAILED_PROCESSING", allowedActions: ["RETRY_PROCESSING"] });
    expect(canRetry(failed, now, false)).toBe(true);
    expect(canRetry({ ...failed, allowedActions: [] }, now, false)).toBe(false);
    expect(canRetry(failed, now, true)).toBe(false);
    expect(canRetry({ ...failed, photoExpiresAt: SERVER_NOW }, now, false)).toBe(false);
  });
});

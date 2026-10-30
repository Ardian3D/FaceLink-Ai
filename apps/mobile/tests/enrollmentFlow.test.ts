import { describe, expect, it, vi } from "vitest";
import { EnrollmentFlow, detikTersisa, type EnrollmentApi } from "@/features/enrollment/flow";
import type { ConsentView, EnrollmentView, HasilEnrollment } from "@/features/enrollment/model";
import {
  labelStatusTerakhir,
  pesanActiveTidakTerpercaya,
  ringkasanStatusAksesibel,
} from "@/features/enrollment/model";
import { syntheticJPEG } from "@/features/enrollment/fixture";
import { SyntheticCapture } from "@/features/enrollment/syntheticCapture";

const notice = {
  version: "synthetic-v1",
  digest: "a".repeat(64),
  text: "Pemberitahuan sintetis untuk pengujian.",
};

function consent(state: ConsentView["state"], revision = 0): ConsentView {
  return {
    state,
    recordId: state === "ACTIVE" ? "11111111-1111-4111-8111-111111111111" : null,
    notice,
    grantedAt: state === "ACTIVE" ? "2026-10-26T10:00:00.000Z" : null,
    withdrawnAt: state === "WITHDRAWN" ? "2026-10-26T10:01:00.000Z" : null,
    revision,
  };
}

function enrollment(
  effectiveStatus: EnrollmentView["effectiveStatus"],
  overrides: Partial<EnrollmentView> = {}
): EnrollmentView {
  return {
    effectiveStatus,
    serverNow: null,
    submissionStatus: effectiveStatus === "NOT_ENROLLED" ? null : effectiveStatus,
    submissionId:
      effectiveStatus === "NOT_ENROLLED" ? null : "22222222-2222-4222-8222-222222222222",
    consentState: "ACTIVE",
    identityState: "UNREVIEWED",
    templateReady: false,
    submittedAt: "2026-10-26T10:00:00.000Z",
    reviewedAt: null,
    processingDeadline: null,
    reasonCode: null,
    deletionState: "NONE",
    revision: 1,
    allowedActions: ["WITHDRAW"],
    photoAvailable: true,
    photoExpiresAt: "2026-10-29T10:00:00.000Z",
    syntheticOnly: true,
    ...overrides,
  };
}

function apiFixture() {
  let currentConsent = consent("NOT_GIVEN");
  let submitServerNow: string | null = null;
  let currentStatus = enrollment("NOT_ENROLLED", {
    consentState: "NOT_GIVEN",
    allowedActions: ["GRANT_CONSENT"],
    photoExpiresAt: null,
    submittedAt: null,
    photoAvailable: false,
    templateReady: false,
  });
  const readStatus = vi.fn(
    async (): Promise<HasilEnrollment<EnrollmentView>> => ({ ok: true, data: currentStatus })
  );
  const api = {
    readConsent: vi.fn(async () => ({
      ok: true as const,
      data: { consent: currentConsent, etag: `"${currentConsent.revision}"` },
    })),
    readStatus,
    grantConsent: vi.fn(async () => {
      currentConsent = consent("ACTIVE", 1);
      currentStatus = {
        ...currentStatus,
        consentState: "ACTIVE",
        allowedActions: ["SUBMIT", "WITHDRAW"],
      };
      return { ok: true as const, data: currentConsent };
    }),
    withdrawConsent: vi.fn(async () => {
      currentConsent = consent("WITHDRAWN", 2);
      currentStatus = enrollment("REVOKED", {
        consentState: "WITHDRAWN",
        templateReady: false,
        allowedActions: ["GRANT_CONSENT"],
        photoAvailable: false,
      });
      return { ok: true as const, data: currentStatus };
    }),
    submitJPEG: vi.fn(async (_token: string, _consent: string, jpeg: Uint8Array) => {
      expect(jpeg[0]).toBe(0xff);
      expect(jpeg[1]).toBe(0xd8);
      currentStatus = enrollment("PENDING_APPROVAL", { serverNow: submitServerNow });
      return { ok: true as const, data: currentStatus };
    }),
  } satisfies EnrollmentApi;
  return {
    api,
    readStatus,
    setStatus(next: EnrollmentView) {
      currentStatus = next;
    },
    setConsent(next: ConsentView) {
      currentConsent = next;
    },
    setSubmitServerNow(next: string | null) {
      submitServerNow = next;
    },
  };
}

describe("enrollment flow keeps consent and enrollment separate", () => {
  it("loads NOT_GIVEN and requires explicit consent before scanning", async () => {
    const { api } = apiFixture();
    const flow = new EnrollmentFlow(api, async () => "token", () => 0);
    await flow.load();
    expect(flow.keadaan().phase).toBe("intro");
    expect(flow.keadaan().consent?.state).toBe("NOT_GIVEN");
    expect(flow.allowed("SUBMIT")).toBe(false);

    flow.masukConsent();
    expect(flow.keadaan().phase).toBe("consent");
    await flow.beriConsent();
    expect(api.grantConsent).toHaveBeenCalledWith(
      "token",
      "synthetic-v1",
      '"0"',
      "",
      expect.any(AbortSignal)
    );
    expect(flow.keadaan().consent?.state).toBe("ACTIVE");
    expect(flow.keadaan().phase).toBe("prepare");
  });

  it("runs synthetic guidance once, submits once, and zeroes the fixture reference", async () => {
    const harness = apiFixture();
    let now = 0;
    const flow = new EnrollmentFlow(harness.api, async () => "token", () => now);
    await flow.load();
    await flow.beriConsent();
    flow.mulaiScan();
    expect(flow.keadaan().phase).toBe("scan");

    now = 1_000;
    flow.tickScan();
    expect(flow.keadaan().capture?.phase).toBe("aligning");
    now = 2_500;
    flow.tickScan();
    await vi.waitFor(() => expect(harness.api.submitJPEG).toHaveBeenCalledTimes(1));
    expect(flow.keadaan().phase).toBe("pending");
    expect(harness.api.submitJPEG).toHaveBeenCalledTimes(1);
    const submitted = harness.api.submitJPEG.mock.calls[0]?.[2];
    expect(submitted).toBeInstanceOf(Uint8Array);
    expect(submitted?.every((byte) => byte === 0)).toBe(true);
  });

  it("does not treat ACTIVE as registered when the snapshot is stale", async () => {
    const harness = apiFixture();
    const flow = new EnrollmentFlow(harness.api, async () => "token", () => 0);
    await flow.load();
    harness.setConsent(consent("ACTIVE", 1));
    harness.setStatus(enrollment("ACTIVE", { templateReady: true }));
    await flow.load();
    expect(flow.trustedActive()).toBe(true);
    flow.suspend();
    expect(flow.keadaan().stale).toBe(true);
    expect(flow.trustedActive()).toBe(false);
  });

  it("expires ACTIVE trust when the snapshot ages out", async () => {
    const harness = apiFixture();
    let now = 0;
    const flow = new EnrollmentFlow(harness.api, async () => "token", () => now);
    harness.setConsent(consent("ACTIVE", 1));
    harness.setStatus(enrollment("ACTIVE", { templateReady: true }));
    await flow.load();
    expect(flow.trustedActive()).toBe(true);
    now = 30_001;
    expect(flow.trustedActive()).toBe(false);
  });

  it("stops the workflow on withdrawal and never keeps ACTIVE trust", async () => {
    const harness = apiFixture();
    const flow = new EnrollmentFlow(harness.api, async () => "token", () => 0);
    await flow.load();
    harness.setConsent(consent("ACTIVE", 1));
    harness.setStatus(enrollment("ACTIVE", { templateReady: true }));
    await flow.load();
    await flow.tarik();
    expect(harness.api.withdrawConsent).toHaveBeenCalledTimes(1);
    expect(flow.keadaan().phase).toBe("withdrawn");
    expect(flow.keadaan().consent?.state).toBe("WITHDRAWN");
    expect(flow.trustedActive()).toBe(false);
  });

  it.each([
    ["REJECTED", "rejected"],
    ["EXPIRED", "expired"],
    ["FAILED_PROCESSING", "failed"],
  ] as const)("maps terminal %s without claiming registration", async (status, phase) => {
    const harness = apiFixture();
    const flow = new EnrollmentFlow(harness.api, async () => "token", () => 0);
    harness.setConsent(consent("ACTIVE", 1));
    harness.setStatus(enrollment(status, { templateReady: false }));
    await flow.load();
    expect(flow.keadaan().phase).toBe(phase);
    expect(flow.trustedActive()).toBe(false);
  });

  it("replaces the active prior consent after a terminal submission", async () => {
    const harness = apiFixture();
    const flow = new EnrollmentFlow(harness.api, async () => "token", () => 0);
    const prior = consent("ACTIVE", 3);
    harness.setConsent(prior);
    harness.setStatus(enrollment("REJECTED"));
    await flow.load();
    flow.masukConsent();
    await flow.beriConsent();
    expect(harness.api.grantConsent).toHaveBeenCalledWith(
      "token",
      prior.notice.version,
      '"3"',
      prior.recordId,
      expect.any(AbortSignal)
    );
    expect(flow.allowed("SUBMIT")).toBe(true);
    expect(flow.keadaan().phase).toBe("rejected");
    flow.siapkan();
    expect(flow.keadaan().phase).toBe("prepare");
    flow.mulaiScan();
    expect(flow.keadaan().phase).toBe("scan");
  });

  it("does not upload after background cancellation while acquiring a token", async () => {
    const harness = apiFixture();
    let now = 0;
    const flow = new EnrollmentFlow(harness.api, async () => "token", () => now);
    await flow.load();
    await flow.beriConsent();
    const pending = Promise.withResolvers<string | null>();
    flow.setTokenProvider(() => pending.promise);
    flow.mulaiScan();
    now = 2_500;
    flow.tickScan();
    expect(flow.keadaan().phase).toBe("uploading");
    flow.suspend();
    pending.resolve("token");
    await pending.promise;
    expect(harness.api.submitJPEG).not.toHaveBeenCalled();
    expect(flow.keadaan().busy).toBe(false);
    expect(flow.keadaan().phase).toBe("prepare");
  });

  it("leaves uploading with a recoverable session error when token acquisition fails", async () => {
    const harness = apiFixture();
    let now = 0;
    const flow = new EnrollmentFlow(harness.api, async () => "token", () => now);
    await flow.load();
    await flow.beriConsent();
    flow.setTokenProvider(async () => null);
    flow.mulaiScan();
    now = 2_500;
    flow.tickScan();
    await vi.waitFor(() => expect(flow.keadaan().error?.kind).toBe("sessionEnded"));
    expect(flow.keadaan().phase).toBe("prepare");
    expect(flow.keadaan().busy).toBe(false);
    expect(harness.api.submitJPEG).not.toHaveBeenCalled();
  });

  it("reloads both snapshots on resume after an interrupted initial load", async () => {
    const harness = apiFixture();
    const pending = Promise.withResolvers<string | null>();
    const flow = new EnrollmentFlow(harness.api, () => pending.promise, () => 0);
    const loading = flow.load();
    flow.suspend();
    pending.resolve("token");
    await loading;
    expect(harness.api.readConsent).not.toHaveBeenCalled();
    flow.setTokenProvider(async () => "token");
    await flow.refresh(false);
    expect(flow.keadaan().consent?.state).toBe("NOT_GIVEN");
    expect(flow.keadaan().phase).toBe("intro");
    expect(flow.keadaan().busy).toBe(false);
  });

  it("invalidates ACTIVE trust after a failed full status reload", async () => {
    const harness = apiFixture();
    const flow = new EnrollmentFlow(harness.api, async () => "token", () => 0);
    harness.setConsent(consent("ACTIVE", 1));
    harness.setStatus(enrollment("ACTIVE", { templateReady: true }));
    await flow.load();
    expect(flow.trustedActive()).toBe(true);
    harness.readStatus.mockResolvedValueOnce({ ok: false, error: { kind: "network" } });
    await flow.load();
    expect(flow.keadaan().stale).toBe(true);
    expect(flow.trustedActive()).toBe(false);
  });

  it.each(["load", "refresh", "background"] as const)(
    "invalidates cached ACTIVE when token acquisition fails during %s",
    async operation => {
      const harness = apiFixture();
      let token: string | null = "token";
      const flow = new EnrollmentFlow(harness.api, async () => token, () => 0);
      harness.setConsent(consent("ACTIVE", 1));
      harness.setStatus(enrollment("ACTIVE", { templateReady: true }));
      await flow.load();
      expect(flow.trustedActive()).toBe(true);
      const reads = harness.readStatus.mock.calls.length;
      token = null;
      if (operation === "load") await flow.load();
      else await flow.refresh(operation !== "background");
      expect(harness.readStatus.mock.calls.length).toBe(reads);
      expect(flow.keadaan().stale).toBe(true);
      expect(flow.trustedActive()).toBe(false);
      token = "restored-token";
      await flow.load();
      expect(flow.trustedActive()).toBe(true);
    }
  );

  it("reconciles a committed withdrawal whose response was lost", async () => {
    const harness = apiFixture();
    const pending = Promise.withResolvers<HasilEnrollment<EnrollmentView>>();
    const api: EnrollmentApi = {
      ...harness.api,
      withdrawConsent: () => pending.promise,
    };
    const flow = new EnrollmentFlow(api, async () => "token", () => 0);
    harness.setConsent(consent("ACTIVE", 1));
    harness.setStatus(enrollment("ACTIVE", { templateReady: true }));
    await flow.load();
    const withdrawing = flow.tarik();
    expect(flow.trustedActive()).toBe(false);
    harness.setConsent(consent("WITHDRAWN", 2));
    harness.setStatus(enrollment("REVOKED", { consentState: "WITHDRAWN" }));
    pending.resolve({ ok: false, error: { kind: "network" } });
    await withdrawing;
    expect(flow.keadaan().phase).toBe("withdrawn");
    expect(flow.keadaan().consent?.state).toBe("WITHDRAWN");
    expect(flow.trustedActive()).toBe(false);
  });

  it("maps a background status failure to stale without asserting ACTIVE", async () => {
    const harness = apiFixture();
    const flow = new EnrollmentFlow(harness.api, async () => "token", () => 0);
    await flow.load();
    harness.setConsent(consent("ACTIVE", 1));
    harness.setStatus(enrollment("ACTIVE", { templateReady: true }));
    await flow.load();
    harness.readStatus.mockResolvedValueOnce({
      ok: false,
      error: { kind: "network" },
    });
    await flow.refresh(false);
    expect(flow.keadaan().stale).toBe(true);
    expect(flow.keadaan().error).toBeNull();
    expect(flow.trustedActive()).toBe(false);
  });
});

describe("synthetic capture harness", () => {
  it("follows alignment, hold, capture, and exposes the fixture once", () => {
    const capture = new SyntheticCapture();
    capture.start(0);
    expect(capture.snapshot(100).phase).toBe("aligning");
    expect(capture.snapshot(1_500).phase).toBe("holding");
    expect(capture.snapshot(2_050).phase).toBe("capturing");
    expect(capture.snapshot(2_400).phase).toBe("ready");
    const jpeg = capture.consume(2_400);
    expect(jpeg).not.toBeNull();
    expect(Array.from(jpeg!.subarray(0, 2))).toEqual([0xff, 0xd8]);
    expect(capture.consume(2_401)).toBeNull();
  });

  it("cancellation is terminal and never exposes a late frame", () => {
    const capture = new SyntheticCapture();
    capture.start(0);
    capture.cancel();
    expect(capture.snapshot(10_000).phase).toBe("cancelled");
    expect(capture.consume(10_000)).toBeNull();
  });

  it("fixture decodes to a bounded JPEG pattern", () => {
    const jpeg = syntheticJPEG();
    expect(jpeg.byteLength).toBeGreaterThan(1_000);
    expect(jpeg.byteLength).toBeLessThan(1_048_576);
    expect(Array.from(jpeg.subarray(jpeg.byteLength - 2))).toEqual([0xff, 0xd9]);
  });
});

describe("accessibility status copy", () => {
  const active = consent("ACTIVE", 1);
  const view = enrollment("ACTIVE", { templateReady: true });

  it("never announces a stale ACTIVE snapshot as registered", () => {
    expect(ringkasanStatusAksesibel("active", view, active, true)).toContain("Wajah terdaftar");
    const stale = ringkasanStatusAksesibel("active", view, active, false);
    expect(stale).toContain("belum dapat diverifikasi");
    expect(stale).not.toContain("Wajah terdaftar");
  });

  it("distinguishes the last verified status from the unverifiable current status", () => {
    expect(labelStatusTerakhir(view, true, false)).toBe("Aktif (belum terverifikasi saat ini)");
    const copy = pesanActiveTidakTerpercaya(view, active, true);
    expect(copy).toContain("Status terakhir yang terverifikasi: Aktif");
    expect(copy).toContain("Status saat ini belum dapat diverifikasi");
  });

  it("does not blame a missing template when the snapshot is merely stale", () => {
    const copy = pesanActiveTidakTerpercaya(view, active, true);
    expect(copy).not.toContain("template belum siap");
  });

  it("reports a missing template when the backend snapshot itself is incomplete", () => {
    const pendingTemplate = enrollment("ACTIVE", { templateReady: false });
    expect(pesanActiveTidakTerpercaya(pendingTemplate, active, false)).toContain(
      "template siap"
    );
  });
});

describe("TTL formatting", () => {
  it("never invents a deadline and clamps elapsed values at zero", () => {
    expect(detikTersisa(null, 0)).toBeNull();
    expect(detikTersisa("tidak-valid", 0)).toBeNull();
    expect(detikTersisa("2026-10-26T10:00:00.000Z", Date.parse("2026-10-26T10:00:10.000Z"))).toBe(0);
    expect(detikTersisa("2026-10-26T10:00:00.000Z", Date.parse("2026-10-26T09:59:30.000Z"))).toBe(30);
  });

  it("shows the pending deadline against backend time, not a skewed device clock", async () => {
    const harness = apiFixture();
    let now = Date.parse("2030-01-01T00:00:00.000Z");
    const flow = new EnrollmentFlow(harness.api, async () => "token", () => now);
    harness.setConsent(consent("ACTIVE", 1));
    harness.setStatus(
      enrollment("PENDING_APPROVAL", {
        serverNow: "2026-10-26T09:59:00.000Z",
        photoExpiresAt: "2026-10-26T10:00:00.000Z",
      })
    );
    await flow.load();
    const deadline = flow.keadaan().enrollment?.photoExpiresAt ?? null;
    expect(flow.serverNow()).toBe(Date.parse("2026-10-26T09:59:00.000Z"));
    expect(detikTersisa(deadline, flow.serverNow()!)).toBe(60);
    now += 30_000;
    expect(detikTersisa(deadline, flow.serverNow()!)).toBe(30);
    // A device clock rollback must not lengthen the server-anchored window.
    now -= 120_000;
    expect(detikTersisa(deadline, flow.serverNow()!)).toBe(60);
    flow.dispose();
  });

  it("does not claim a server deadline when the status response omits server time", async () => {
    const harness = apiFixture();
    const flow = new EnrollmentFlow(harness.api, async () => "token", () => 0);
    harness.setConsent(consent("ACTIVE", 1));
    harness.setStatus(enrollment("PENDING_APPROVAL"));
    await flow.load();
    expect(flow.serverNow()).toBeNull();
    flow.dispose();
  });

  it("anchors the pending deadline from the submit receipt", async () => {
    const harness = apiFixture();
    let now = 0;
    harness.setSubmitServerNow("2026-10-26T09:59:00.000Z");
    const flow = new EnrollmentFlow(harness.api, async () => "token", () => now);
    await flow.load();
    await flow.beriConsent();
    flow.mulaiScan();
    now = 2_400;
    flow.tickScan();
    await vi.waitFor(() => expect(flow.keadaan().phase).toBe("pending"));
    expect(flow.serverNow()).toBe(Date.parse("2026-10-26T09:59:00.000Z"));
    flow.dispose();
  });
});

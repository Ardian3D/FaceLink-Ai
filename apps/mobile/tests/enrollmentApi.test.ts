import { afterEach, describe, expect, it, vi } from "vitest";

const notice = {
  version: "synthetic-v1",
  digest: "b".repeat(64),
  text: "Pemberitahuan sintetis.",
};
const consent = {
  state: "ACTIVE",
  record_id: "11111111-1111-4111-8111-111111111111",
  notice,
  granted_at: "2026-10-26T10:00:00.000Z",
  revision: 1,
};
const status = {
  effective_status: "PENDING_APPROVAL",
  submission_status: "PENDING_APPROVAL",
  submission_id: "22222222-2222-4222-8222-222222222222",
  consent_state: "ACTIVE",
  identity_state: "UNREVIEWED",
  template_ready: false,
  submitted_at: "2026-10-26T10:00:00.000Z",
  revision: 1,
  allowed_actions: ["WITHDRAW"],
  photo_available: true,
  photo_expires_at: "2026-10-29T10:00:00.000Z",
  synthetic_only: true,
};

async function loadApi() {
  vi.resetModules();
  vi.stubEnv("EXPO_PUBLIC_FACELINK_API_BASE_URL", "https://staging.enr02.invalid");
  vi.stubGlobal("__DEV__", false);
  return import("@/features/enrollment/api");
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
  vi.restoreAllMocks();
});

describe("actual enrollment API contract", () => {
  it("reads consent with its ETag and never uses a body on the GET", async () => {
    const fetchFixture = vi.fn(async () =>
      new Response(JSON.stringify({ data: consent }), {
        status: 200,
        headers: { "Content-Type": "application/json", ETag: '"1"' },
      })
    );
    vi.stubGlobal("fetch", fetchFixture);
    const api = await loadApi();
    const result = await api.bacaConsent("access-token");
    expect(result).toMatchObject({
      ok: true,
      data: { etag: '"1"', consent: { state: "ACTIVE", revision: 1 } },
    });
    const [url, init] = fetchFixture.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe("https://staging.enr02.invalid/api/v1/me/consent");
    expect(init.method).toBe("GET");
    expect(init.body).toBeUndefined();
    expect(new Headers(init.headers).get("Authorization")).toBe("Bearer access-token");
  });

  it("sends strong If-Match and a UUID idempotency key on consent grant", async () => {
    const fetchFixture = vi.fn(async () =>
      new Response(
        JSON.stringify({
          data: {
            operation_id: "op",
            resource_id: consent.record_id,
            applied_revision: 1,
            current: consent,
          },
        }),
        { status: 201, headers: { "Content-Type": "application/json" } }
      )
    );
    vi.stubGlobal("fetch", fetchFixture);
    const api = await loadApi();
    const result = await api.beriConsent(
      "access-token",
      "synthetic-v1",
      '"0"',
      consent.record_id
    );
    expect(result.ok).toBe(true);
    const [, init] = fetchFixture.mock.calls[0] as unknown as [string, RequestInit];
    const headers = new Headers(init.headers);
    expect(headers.get("If-Match")).toBe('"0"');
    expect(headers.get("Idempotency-Key")).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/
    );
    expect(JSON.parse(String(init.body))).toEqual({
      notice_version: "synthetic-v1",
      accepted: true,
      replaces_consent_id: consent.record_id,
    });
  });

  it("uploads JPEG bytes with the documented content type and consent header", async () => {
    const fetchFixture = vi.fn(async () =>
      new Response(
        JSON.stringify({
          data: {
            operation_id: "op",
            resource_id: status.submission_id,
            applied_revision: 1,
            current: status,
          },
        }),
        { status: 201, headers: { "Content-Type": "application/json" } }
      )
    );
    vi.stubGlobal("fetch", fetchFixture);
    const api = await loadApi();
    const jpeg = new Uint8Array([0xff, 0xd8, 0xff, 0xd9]);
    const result = await api.kirimFotoSintetis("access-token", consent.record_id, jpeg);
    expect(result.ok).toBe(true);
    const [url, init] = fetchFixture.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe("https://staging.enr02.invalid/api/v1/me/face-enrollment-submissions");
    const headers = new Headers(init.headers);
    expect(headers.get("Content-Type")).toBe("image/jpeg");
    expect(headers.get("X-Consent-Id")).toBe(consent.record_id);
    expect(headers.get("Idempotency-Key")).toBeTruthy();
    expect(init.body).toBe(jpeg);
  });

  it("rejects a status payload that is not explicitly synthetic", async () => {
    const fetchFixture = vi.fn(async () =>
      new Response(JSON.stringify({ data: { ...status, synthetic_only: false } }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      })
    );
    vi.stubGlobal("fetch", fetchFixture);
    const api = await loadApi();
    const result = await api.bacaStatus("access-token");
    expect(result).toMatchObject({ ok: false, error: { kind: "unknown", code: "BENTUK_STATUS" } });
  });

  it("maps a consent version conflict without rendering the raw server message", async () => {
    const fetchFixture = vi.fn(async () =>
      new Response(
        JSON.stringify({
          error: { code: "CONSENT_VERSION_MISMATCH", message: "raw internal detail" },
        }),
        { status: 409, headers: { "Content-Type": "application/json" } }
      )
    );
    vi.stubGlobal("fetch", fetchFixture);
    const api = await loadApi();
    const result = await api.beriConsent("access-token", "old", '"0"');
    expect(result).toMatchObject({
      ok: false,
      error: { kind: "conflict", code: "CONSENT_VERSION_MISMATCH" },
    });
    expect(JSON.stringify(result)).not.toContain("raw internal detail");
  });

  it("maps network failure to a safe kind and never throws", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => Promise.reject(new Error("socket closed"))));
    const api = await loadApi();
    const result = await api.bacaStatus("access-token");
    expect(result).toEqual({ ok: false, error: { kind: "network" } });
  });
});

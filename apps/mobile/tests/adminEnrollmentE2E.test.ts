import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";
import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import readline from "node:readline";
import type { AdminApi, Decision, Operation } from "@/features/adminEnrollment/model";

const enabled = process.env.FACELINK_ADMIN_ENR_E2E === "1";
type Fixture = { user_id: string; email: string; person_id?: string; submission_id?: string; identity_reference?: string };
type Startup = { base_url: string; root: string; otp_file: string; fixtures: Record<string, Fixture> };
function root(): string {
  let dir = import.meta.dirname;
  for (let i = 0; i < 8; i++, dir = path.dirname(dir)) if (fs.existsSync(path.join(dir, "backend/go.mod"))) return dir;
  throw new Error("Repository root not found");
}
/** Real TLS trust is supplied by NODE_EXTRA_CA_CERTS, never disabled. No UI auth bypass. */
describe.skipIf(!enabled)("FL-ENR03 actual HTTPS synthetic Admin integration", () => {
  let child: ChildProcessWithoutNullStreams;
  let startup: Startup;
  let api: AdminApi;
  let admin: string;
  let reviewer: string;
  let lines: readline.Interface;
  const controlWaiters: ((data: Record<string, unknown>) => void)[] = [];
  async function control(command: string, fields: Record<string, unknown> = {}) {
    const reply = Promise.withResolvers<Record<string, unknown>>();
    controlWaiters.push(reply.resolve);
    child.stdin.write(`${JSON.stringify({ command, ...fields })}\n`);
    const ack = await reply.promise;
    expect(ack.ok).toBe(true);
    return ack;
  }
  async function login(name: string) {
    const fixture = startup.fixtures[name];
    if (!fixture) throw new Error("Missing synthetic fixture");
    const requested = await fetch(`${startup.base_url}/api/v1/auth/email/request`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ email: fixture.email }) });
    expect(requested.status).toBe(202);
    const challenge = await requested.json();
    // The outbox file is published before the delivery transaction commits.
    // Wait for this exact challenge's committed state before the only verify.
    await control("otp.wait", { challenge_id: challenge.data.challenge_id });
    const code = fs.readFileSync(startup.otp_file, "utf8").trim();
    const verified = await fetch(`${startup.base_url}/api/v1/auth/email/verify`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ challenge_id: challenge.data.challenge_id, code }) });
    expect(verified.status).toBe(200);
    const session = await verified.json();
    if (typeof session.data.access_token !== "string") throw new Error("Missing session credential");
    return session.data.access_token as string;
  }
  const id = (name: string) => startup.fixtures[name]!.submission_id!;
  async function detail(name: string, token = admin) {
    const result = await api.detail(token, id(name));
    if (!result.ok) throw new Error(`Detail failed: ${result.error.kind}/${result.error.code ?? ""}`);
    return result.data;
  }
  async function operation(name: string, decision: Decision): Promise<Operation> {
    return { decision, idempotencyKey: crypto.randomUUID(), ifMatch: (await detail(name)).etag };
  }
  beforeAll(async () => {
    child = spawn("go", ["run", "./cmd/enrollment-staging-serve", "-admin-fixtures", "-addr", "127.0.0.1:0"], { cwd: path.join(root(), "backend"), windowsHide: true, stdio: ["pipe", "pipe", "pipe"] });
    const ready = Promise.withResolvers<Startup>();
    const timer = setTimeout(() => ready.reject(new Error("HTTPS fixture startup deadline")), 90_000);
    child.once("error", ready.reject);
    child.once("exit", () => ready.reject(new Error("HTTPS fixture exited before ready")));
    child.stderr.on("data", chunk => console.error(`[enr03 stderr] ${String(chunk).trim()}`));
    lines = readline.createInterface({ input: child.stdout });
    lines.on("line", line => {
      try {
        const value = JSON.parse(line);
        // Startup also carries fixture_control:"stdin-jsonl"; only ack objects are boolean true.
        if (value.mode === "FL-ENR03-synthetic-staging") ready.resolve(value);
        else if (value.fixture_control === true) controlWaiters.shift()?.(value);
      } catch { ready.reject(new Error("Invalid fixture control JSON")); }
    });
    try { startup = await ready.promise; } finally { clearTimeout(timer); }
    expect(startup.base_url.startsWith("https://127.0.0.1:")).toBe(true);
    vi.stubEnv("EXPO_PUBLIC_FACELINK_API_BASE_URL", startup.base_url);
    vi.stubGlobal("__DEV__", false);
    api = (await import("@/features/adminEnrollment/api")).adminApi;
    admin = await login("admin");
    reviewer = await login("reviewer2");
  }, 100_000);
  afterAll(async () => {
    if (child && child.exitCode === null) {
      const exited = Promise.withResolvers<void>();
      child.once("exit", exited.resolve);
      child.stdin.end();
      const timer = setTimeout(() => child.kill(), 10_000);
      try { await exited.promise; } finally { clearTimeout(timer); }
    }
    lines?.close();
    if (startup) expect(fs.existsSync(startup.root)).toBe(false);
    vi.unstubAllEnvs(); vi.unstubAllGlobals();
  }, 15_000);

  it("reads identity/roster/preview and commits approve202 → worker ACTIVE → withdrawal", async () => {
    const queue = await api.list(admin, "PENDING_APPROVAL", null);
    expect(queue).toMatchObject({ ok: true, data: { pendingCount: 4 } });
    const all = await api.list(admin, "", null);
    expect(all).toMatchObject({ ok: true });
    if (all.ok) expect(all.data.items.length).toBe(5);
    const before = await detail("pending-approve");
    expect(before.view.identityBinding).toMatchObject({ eligible: true, accountRole: "mahasiswa", accountStatus: "active" });
    expect(before.view.student.classes[0]?.code).toBe("SYN-ENR02A");
    const preview = await api.photo(admin, id("pending-approve"));
    expect(preview.ok).toBe(true);
    if (preview.ok) { expect([...preview.data.slice(0, 2)]).toEqual([255, 216]); preview.data.fill(0); }
    const op = await operation("pending-approve", { kind: "approve", identityCheckMethod: "CAMPUS_AUTHORIZED_CHECK", campusCheckRef: startup.fixtures["pending-approve"]!.identity_reference!, confirmed: true });
    const approved = await api.decide(admin, id("pending-approve"), op);
    expect(approved).toMatchObject({ ok: true, data: { httpStatus: 202, view: { effectiveStatus: "APPROVED_PROCESSING", templateReady: false } } });
    await control("worker.step");
    const active = await detail("pending-approve");
    expect(active.view).toMatchObject({ effectiveStatus: "ACTIVE", templateReady: true, photoAvailable: false, identityState: "ADMIN_ATTESTED" });
    expect(active.view.review?.reviewerAdminId).toBe(startup.fixtures.admin!.user_id);
    expect(active.view.history.some(event => event.action === "enrollment.approved")).toBe(true);
    const student = await login("pending-approve");
    const withdraw = await fetch(`${startup.base_url}/api/v1/me/consent/withdraw`, { method: "POST", headers: { Authorization: `Bearer ${student}`, "Content-Type": "application/json", "Idempotency-Key": crypto.randomUUID() }, body: JSON.stringify({ scope: "ALL_FACE_ENROLLMENT", confirmed: true }) });
    expect(withdraw.status).toBe(200);
    expect((await detail("pending-approve")).view).toMatchObject({ effectiveStatus: "REVOKED", templateReady: false, photoAvailable: false });
    expect(await api.photo(admin, id("pending-approve"))).toMatchObject({ ok: false, error: { kind: "photoUnavailable" } });
  }, 30_000);

  it("serializes concurrent reviewers; rejects stale opposite decisions and replays exact operation", async () => {
    const target = "pending-conflict";
    const op = await operation(target, { kind: "reject", reasonCode: "EVIDENCE_INSUFFICIENT", confirmed: true });
    const opposite: Operation = { ...op, idempotencyKey: crypto.randomUUID(), decision: { kind: "approve", confirmed: true, identityCheckMethod: "CAMPUS_AUTHORIZED_CHECK", campusCheckRef: startup.fixtures[target]!.identity_reference! } };
    const results = await Promise.all([api.decide(admin, id(target), op), api.decide(reviewer, id(target), opposite)]);
    expect(results.filter(r => r.ok)).toHaveLength(1);
    expect(results.filter(r => !r.ok)).toMatchObject([{ ok: false, error: { kind: "precondition" } }]);
    const winner = results[0]!.ok ? { token: admin, operation: op } : { token: reviewer, operation: opposite };
    expect(await api.decide(winner.token, id(target), winner.operation)).toMatchObject({ ok: true, data: { httpStatus: 200 } });
  });

  it("keeps failed/retry processing backend-controlled and purges rejected fixtures", async () => {
    const failed = await detail("failed-processing");
    expect(failed.view).toMatchObject({ effectiveStatus: "FAILED_PROCESSING", photoAvailable: true, reasonCode: "BIOMETRIC_UNAVAILABLE" });
    const retried = await api.decide(admin, id("failed-processing"), await operation("failed-processing", { kind: "retry", confirmed: true }));
    expect(retried).toMatchObject({ ok: true, data: { httpStatus: 202, view: { effectiveStatus: "APPROVED_PROCESSING", templateReady: false } } });
    // Process queued jobs until this real retry is active (another concurrent winner may precede it).
    for (let n = 0; n < 3; n++) { if ((await detail("failed-processing")).view.effectiveStatus === "ACTIVE") break; await control("worker.step"); }
    expect((await detail("failed-processing")).view).toMatchObject({ effectiveStatus: "ACTIVE", templateReady: true });
    const rejected = await api.decide(admin, id("pending-reject"), await operation("pending-reject", { kind: "reject", reasonCode: "IDENTITY_MISMATCH", confirmed: true }));
    expect(rejected).toMatchObject({ ok: true, data: { view: { effectiveStatus: "REJECTED", photoAvailable: false } } });
  });

  it("denies lecturer/student/cross-user preview and invalidated Admin before revealing data", async () => {
    const lecturer = await login("lecturer");
    const student = await login("pending-expiry");
    for (const token of [lecturer, student]) {
      expect(await api.list(token, "PENDING_APPROVAL", null)).toMatchObject({ ok: false, error: { kind: "forbidden" } });
      expect(await api.detail(token, id("pending-expiry"))).toMatchObject({ ok: false, error: { kind: "forbidden" } });
      expect(await api.photo(token, id("pending-expiry"))).toMatchObject({ ok: false, error: { kind: "forbidden" } });
    }
    await control("account.role", { fixture: "reviewer2", role: "dosen" });
    expect(await api.detail(reviewer, id("pending-expiry"))).toMatchObject({ ok: false, error: { kind: "forbidden" } });
    await control("account.status", { fixture: "reviewer2", status: "suspended" });
    expect(await api.photo(reviewer, id("pending-expiry"))).toMatchObject({ ok: false, error: { kind: "sessionEnded" } });
  });

  it("expires the original72h photo deadline without extending sessions or accepting late approval", async () => {
    const op = await operation("pending-expiry", { kind: "approve", confirmed: true, identityCheckMethod: "CAMPUS_AUTHORIZED_CHECK", campusCheckRef: startup.fixtures["pending-expiry"]!.identity_reference! });
    await control("clock.advance", { seconds: 72 * 3600 });
    expect(await api.detail(admin, id("pending-expiry"))).toMatchObject({ ok: false, error: { kind: "sessionEnded" } });
    admin = await login("admin");
    expect((await detail("pending-expiry")).view).toMatchObject({ effectiveStatus: "EXPIRED", photoAvailable: false, templateReady: false });
    expect(await api.decide(admin, id("pending-expiry"), op)).toMatchObject({ ok: false });
    expect(await api.photo(admin, id("pending-expiry"))).toMatchObject({ ok: false, error: { kind: "photoUnavailable" } });
  });
});

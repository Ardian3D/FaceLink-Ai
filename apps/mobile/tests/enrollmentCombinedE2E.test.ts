import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";
import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { createHash } from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import readline from "node:readline";
import type { AdminApi } from "@/features/adminEnrollment/model";
import type { AdminReviewFlow } from "@/features/adminEnrollment/flow";
import type { HasilEnrollment } from "@/features/enrollment/model";
import type { EnrollmentApi, EnrollmentFlow } from "@/features/enrollment/flow";

const enabled = process.env.FACELINK_ENR05_E2E === "1";
const FIXTURE_SHA256 = "cf572a3ef381f4b52087c9dac062d0af4da6ca352b1e55742674a1c308e6cabe";
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const HASH = /^[0-9a-f]{64}$/;
const MANIFEST = {
  SyntheticOnly: true,
  ModelID: "opencv-sface", ModelVersion: "2021dec",
  ModelChecksum: "0ba9fbfa01b5270c96627c4ef784da859931e02f04419c829e83484087c34e79",
  DetectorID: "opencv-yunet", DetectorVersion: "2023mar",
  DetectorChecksum: "8f2383e4dd3cfbb4553ea8718107fc0423210dc964f9f4280604804ed2552fa4",
  PipelineRevision: "enr04-synthetic-native-v2",
};
type Fixture = { user_id: string; email: string; person_id?: string; identity_reference?: string };
type Startup = {
  mode: string; base_url: string; root: string; otp_file: string; trace_id: string;
  schema: number; production: boolean; human_capture: boolean;
  fixtures: Record<string, Fixture>; processor: typeof MANIFEST;
};
type Audit = { action: string; result: string; request_id: string };
type Snapshot = {
  schema: number; journal_mode: string; submission_id: string; status: string; reason_code: string;
  photo_rows: number; template_rows: number; active_rows: number; ciphertext_files: number;
  deletion_incomplete: boolean; photo_expires_at: string; canonical_sha256: string; processed_sha256: string;
  processing_calls: number; face_count: number; embedding_dim: number; embedding_finite: boolean;
  manifest_verified: boolean; template_envelope_version: number; template_binding_valid: boolean;
  grant_identity_preserved: boolean; audit: Audit[]; trace: Record<string, unknown>[];
};
type Ack = { fixture_control: true; command: string; ok: boolean; trace_id: string; data?: Snapshot };
type Waiter = { command: string; resolve: (ack: Ack) => void; reject: (error: Error) => void; timer: NodeJS.Timeout };

function repoRoot(): string {
  let directory = import.meta.dirname;
  for (let i = 0; i < 8; i++, directory = path.dirname(directory)) {
    if (fs.existsSync(path.join(directory, "backend/go.mod"))) return directory;
  }
  throw new Error("Repository root not found");
}
function value<T>(result: HasilEnrollment<T>, operation: string): T {
  if (!result.ok) throw new Error(`${operation} failed (${result.error.kind})`);
  return result.data;
}
const sha256 = (bytes: Uint8Array) => createHash("sha256").update(bytes).digest("hex");

/**
 * Real authApi + Student/Admin clients and controllers, trusted HTTPS, actual Go
 * store/worker and pinned Python IPC. No response mock, startup session, camera,
 * SecureStore, production configuration or deterministic positive processor.
 * Android rendering/lifecycle instrumentation is deliberately NOT_RUN.
 */
describe.skipIf(!enabled)("FL-ENR05 real synthetic Student → Admin → Python enrollment", () => {
  let child: ChildProcessWithoutNullStreams | undefined;
  let lines: readline.Interface | undefined;
  let startup: Startup | undefined;
  let adminApi: AdminApi;
  let studentApi: EnrollmentApi;
  let studentFlow: EnrollmentFlow | undefined;
  let adminFlow: AdminReviewFlow | undefined;
  // Private memory only: reuse normal OTP sessions without triggering cooldown.
  let studentToken: string;
  let adminToken: string;
  let lossTarget: string | null = null;
  let dropNextApprovalResponse = false;
  const approvalAttempts: { body: string; ifMatch: string | null; idempotencyKey: string | null }[] = [];
  const responseLoss = {
    injection: "throw after actual Go HTTPS approval response; not physical network disconnect or Android",
    server_status: 0, replay_status: 0,
    uncertain_after_loss: false, no_active_after_loss: false,
    exact_body_replay: false, exact_cas_replay: false, exact_idempotency_replay: false,
    approval_audit_delta: 0, replay_approval_audit_delta: 0, cleanup_zero: false,
  };
  const waiters: Waiter[] = [];
  const requests: { method: string; route: string; status: number; request_id: string }[] = [];
  const consumer: { stage: string; phase: string; trusted_active?: boolean; http_status?: number }[] = [];
  const snapshots: { stage: string; data: Snapshot }[] = [];
  let positivePassed = false;
  let responseLossPassed = false;

  function ready(): Startup {
    if (!startup) throw new Error("Integration fixture unavailable");
    return startup;
  }
  function rejectControls(): void {
    for (const waiter of waiters.splice(0)) {
      clearTimeout(waiter.timer);
      waiter.reject(new Error("Integration control channel closed"));
    }
  }
  async function control(command: string, fields: Record<string, unknown> = {}): Promise<Ack> {
    const current = ready();
    const reply = Promise.withResolvers<Ack>();
    const waiter: Waiter = {
      command, resolve: reply.resolve, reject: reply.reject,
      timer: setTimeout(() => {
        const index = waiters.indexOf(waiter);
        if (index >= 0) waiters.splice(index, 1);
        reply.reject(new Error("Integration control acknowledgement deadline"));
      }, 30_000),
    };
    waiters.push(waiter);
    child!.stdin.write(`${JSON.stringify({ command, ...fields })}\n`, error => {
      if (error) rejectControls();
    });
    const ack = await reply.promise;
    expect(ack.command).toBe(command);
    expect(ack.ok).toBe(true);
    expect(ack.trace_id).toBe(current.trace_id);
    return ack;
  }
  async function snapshot(stage: string, submissionId: string): Promise<Snapshot> {
    const ack = await control("snapshot", { submission_id: submissionId });
    if (!ack.data) throw new Error("Missing integration snapshot");
    expect(ack.data.submission_id).toBe(submissionId);
    expect(ack.data.schema).toBe(8);
    expect(ack.data.journal_mode.toLowerCase()).toBe("wal");
    snapshots.push({ stage, data: ack.data });
    return ack.data;
  }
  async function login(name: "student" | "admin"): Promise<string> {
    const current = ready();
    const fixture = current.fixtures[name];
    if (!fixture) throw new Error("Missing synthetic login fixture");
    const auth = await import("@/features/auth/authApi");
    const requested = await auth.mintaKodeEmail(fixture.email);
    if (!requested.ok) throw new Error(`Synthetic OTP request failed (${requested.error.kind})`);
    // Outbox publication precedes DB delivery commit; wait for this challenge
    // before reading its code and making the single verification request.
    await control("otp.wait", { challenge_id: requested.data.challengeId });
    const code = fs.readFileSync(current.otp_file, "utf8").trim();
    if (!/^\d{8}$/.test(code)) throw new Error("Invalid synthetic OTP delivery");
    const verified = await auth.verifikasiKodeEmail(requested.data.challengeId, code);
    if (!verified.ok) throw new Error(`Synthetic OTP verification failed (${verified.error.kind})`);
    // Keep credentials in memory only; neither session nor account data enters evidence.
    return verified.data.accessToken;
  }
  function observeStudent(stage: string): void {
    const flow = studentFlow!;
    consumer.push({ stage, phase: flow.keadaan().phase, trusted_active: flow.trustedActive() });
  }

  beforeAll(async () => {
    const root = repoRoot();
    const ca = path.join(root, "backend/dev/certs/dev-ca.pem");
    if (process.env.NODE_TLS_REJECT_UNAUTHORIZED === "0") throw new Error("TLS verification bypass forbidden");
    if (!process.env.NODE_EXTRA_CA_CERTS || fs.realpathSync(process.env.NODE_EXTRA_CA_CERTS) !== fs.realpathSync(ca)) {
      throw new Error("Start Node with NODE_EXTRA_CA_CERTS pointing to the repository development CA");
    }
    const engine = path.join(root, "services/face-engine");
    const python = path.join(engine, ".venv", process.platform === "win32" ? "Scripts/python.exe" : "bin/python");
    const models = path.join(engine, "models");
    for (const file of [python, path.join(models, "face_detection_yunet_2023mar.onnx"), path.join(models, "face_recognition_sface_2021dec.onnx")]) {
      if (!fs.existsSync(file)) throw new Error("Pinned local CV prerequisite missing");
    }
    child = spawn("go", ["run", "./cmd/enrollment-staging-serve", "-integration-fixtures", "-addr", "127.0.0.1:0", "-cv-python", python, "-cv-engine-dir", engine, "-cv-model-dir", models], {
      cwd: path.join(root, "backend"), windowsHide: true, stdio: ["pipe", "pipe", "pipe"],
    });
    const pending = Promise.withResolvers<Startup>();
    const timer = setTimeout(() => pending.reject(new Error("Integration HTTPS startup deadline")), 90_000);
    child.once("error", () => pending.reject(new Error("Integration process startup failed")));
    child.once("exit", () => { pending.reject(new Error("Integration process exited")); rejectControls(); });
    // Drain, but never print raw child errors/paths or payloads into test output.
    child.stderr.on("data", () => {});
    lines = readline.createInterface({ input: child.stdout });
    lines.on("line", line => {
      try {
        const parsed = JSON.parse(line);
        if (parsed.mode === "FL-ENR05-synthetic-integration") { startup = parsed as Startup; pending.resolve(startup); }
        else if (parsed.fixture_control === true) {
          const waiter = waiters.shift();
          if (!waiter) return;
          clearTimeout(waiter.timer);
          if (parsed.command !== waiter.command) waiter.reject(new Error("Unexpected integration control reply"));
          else waiter.resolve(parsed as Ack);
        }
      } catch { pending.reject(new Error("Invalid integration control JSON")); rejectControls(); }
    });
    try { startup = await pending.promise; } finally { clearTimeout(timer); }
    expect(/^https:\/\/127\.0\.0\.1:\d+$/.test(startup.base_url)).toBe(true);
    expect(UUID.test(startup.trace_id)).toBe(true);
    expect(startup.schema).toBe(8);
    expect(startup.production).toBe(false);
    expect(startup.human_capture).toBe(false);
    expect(startup.processor).toEqual(MANIFEST);
    // All responses come from real verified HTTPS. The separate response-loss
    // case keeps only approval intent in private memory, then throws AFTER the
    // real server response; no fabricated HTTP success or physical-network claim.
    const realFetch = globalThis.fetch.bind(globalThis);
    vi.stubGlobal("fetch", async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
      const url = new URL(input instanceof Request ? input.url : String(input));
      if (url.origin !== ready().base_url) throw new Error("Integration request escaped disposable HTTPS origin");
      const headers = new Headers(init?.headers ?? (input instanceof Request ? input.headers : undefined));
      headers.set("X-Request-ID", ready().trace_id);
      const method = init?.method ?? (input instanceof Request ? input.method : "GET");
      const approval = method === "POST" && url.pathname === lossTarget;
      if (approval) {
        const body = init?.body ?? (input instanceof Request ? await input.clone().text() : undefined);
        if (typeof body !== "string") throw new Error("Missing real approval request body");
        approvalAttempts.push({ body, ifMatch: headers.get("If-Match"), idempotencyKey: headers.get("Idempotency-Key") });
      }
      const response = await realFetch(input, { ...init, headers });
      requests.push({ method: init?.method ?? (input instanceof Request ? input.method : "GET"), route: url.pathname.replace(/[0-9a-f]{8}-[0-9a-f-]{27}/gi, ":id"), status: response.status, request_id: ready().trace_id });
      if (approval && dropNextApprovalResponse) {
        dropNextApprovalResponse = false;
        responseLoss.server_status = response.status;
        // Consume and discard the genuine receipt; the client receives only a
        // transport error, not the server's committed 202 response.
        await response.arrayBuffer();
        throw new Error("Synthetic post-response loss injection");
      }
      if (approval) responseLoss.replay_status = response.status;
      return response;
    });
    vi.stubEnv("EXPO_PUBLIC_FACELINK_API_BASE_URL", startup.base_url);
    vi.stubGlobal("__DEV__", false);
    const student = await import("@/features/enrollment/api");
    studentApi = { readConsent: student.bacaConsent, readStatus: student.bacaStatus, grantConsent: student.beriConsent, withdrawConsent: student.tarikConsent, submitJPEG: student.kirimFotoSintetis };
    adminApi = (await import("@/features/adminEnrollment/api")).adminApi;
  }, 100_000);

  afterAll(async () => {
    studentFlow?.dispose(); adminFlow?.dispose();
    rejectControls();
    let forced = false;
    if (child?.pid !== undefined && child.exitCode === null && child.signalCode === null) {
      const exited = Promise.withResolvers<void>();
      child.once("exit", () => exited.resolve());
      child.stdin.end();
      const timer = setTimeout(() => { forced = true; child!.kill(); }, 15_000);
      try { await exited.promise; } finally { clearTimeout(timer); }
    }
    lines?.close();
    vi.unstubAllEnvs(); vi.unstubAllGlobals();
    if (startup) {
      const rootRemoved = !fs.existsSync(startup.root);
      // Only a fixed whitelist is persisted. No tokens/OTP/accounts/JPEG/vector,
      // raw AAD, raw processor responses, filesystem paths or child stderr.
      const safeSnapshots = snapshots.map(({ stage, data: s }) => ({
        stage, schema: s.schema, journal_mode: s.journal_mode, submission_id: s.submission_id,
        status: s.status, photo_rows: s.photo_rows, template_rows: s.template_rows, active_rows: s.active_rows,
        ciphertext_files: s.ciphertext_files, deletion_incomplete: s.deletion_incomplete,
        canonical_sha256: HASH.test(s.canonical_sha256) ? s.canonical_sha256 : null,
        processed_sha256: HASH.test(s.processed_sha256) ? s.processed_sha256 : null,
        processing_calls: s.processing_calls, face_count: s.face_count, embedding_dim: s.embedding_dim,
        embedding_finite: s.embedding_finite, manifest_verified: s.manifest_verified,
        template_envelope_version: s.template_envelope_version, template_binding_valid: s.template_binding_valid,
        grant_identity_preserved: s.grant_identity_preserved,
        audit: s.audit.map(a => ({ action: /^[a-z]+(?:[._][a-z]+)*$/.test(a.action) ? a.action : "redacted", result: /^[a-zA-Z_]{0,40}$/.test(a.result) ? a.result : "redacted", request_id: a.request_id === startup!.trace_id ? a.request_id : "" })),
        trace: s.trace.map(t => ({
          stage: ["snapshot", "python.process"].includes(String(t.stage)) ? t.stage : "redacted",
          trace_id: t.trace_id === startup!.trace_id ? t.trace_id : null,
          submission_id: typeof t.submission_id === "string" && UUID.test(t.submission_id) ? t.submission_id : null,
          canonical_sha256: typeof t.canonical_sha256 === "string" && HASH.test(t.canonical_sha256) ? t.canonical_sha256 : null,
          processed_sha256: typeof t.processed_sha256 === "string" && HASH.test(t.processed_sha256) ? t.processed_sha256 : null,
          processing_calls: typeof t.processing_calls === "number" ? t.processing_calls : null,
          face_count: typeof t.face_count === "number" ? t.face_count : null,
          embedding_dim: typeof t.embedding_dim === "number" ? t.embedding_dim : null,
          embedding_finite: typeof t.embedding_finite === "boolean" ? t.embedding_finite : null,
          manifest_verified: typeof t.manifest_verified === "boolean" ? t.manifest_verified : null,
        })),
      }));
      const evidence = path.join(repoRoot(), "research/fl-enr02a-host/evidence-local/enr05");
      fs.mkdirSync(evidence, { recursive: true });
      fs.writeFileSync(path.join(evidence, `combined-${startup.trace_id}.json`), JSON.stringify({
        contract: "FL-ENR05", trace_id: startup.trace_id, result: positivePassed && responseLossPassed && rootRemoved && !forced ? "PASS" : "FAIL",
        synthetic_only: true, production: false, human_capture: false, android_ui: "NOT_RUN",
        fixture_sha256: FIXTURE_SHA256, processor: MANIFEST, https_verification: "Node development CA; no TLS bypass",
        correlation: "HTTP audit uses X-Request-ID. Worker audit retains its actual blank request_id. The harness associates actual Go Process input SHA, validated Python IPC result and pinned manifest with the same submission UUID and trace_id; Python IPC keeps its independent private nonce/request UUID. No worker/Python protocol request ID is invented.",
        cases: { continuous_real_python: positivePassed ? "PASS" : "FAIL", approval_response_loss: { ...responseLoss, result: responseLossPassed ? "PASS" : "FAIL" } },
        requests, consumer, snapshots: safeSnapshots, cleanup: { child_eof: !forced, root_removed: rootRemoved },
      }, null, 2), { flag: "wx" });
      expect(forced).toBe(false);
      expect(rootRemoved).toBe(true);
    }
  }, 25_000);

  it("runs normal OTP → consent → exact R2 artifact → protected review → 202 → real Python ACTIVE → withdrawal/cleanup", async () => {
    const current = ready();
    studentToken = await login("student");
    // Load the configured modules only after beforeAll installs the disposable
    // API origin; static imports would capture configuration before that boundary.
    const { EnrollmentFlow } = await import("@/features/enrollment/flow");
    const { AdminReviewFlow } = await import("@/features/adminEnrollment/flow");
    const { statusLabel } = await import("@/features/adminEnrollment/model");
    let credentialAvailable = true;
    studentFlow = new EnrollmentFlow(studentApi, async () => credentialAvailable ? studentToken : null);
    await studentFlow.load();
    expect(studentFlow.keadaan().error).toBeNull();
    expect(studentFlow.keadaan().phase).toBe("intro");
    expect(studentFlow.keadaan().consent?.state).toBe("NOT_GIVEN");
    expect(studentFlow.keadaan().enrollment?.effectiveStatus).toBe("NOT_ENROLLED");
    expect(studentFlow.trustedActive()).toBe(false);
    studentFlow.masukConsent();
    await studentFlow.beriConsent();
    expect(studentFlow.keadaan().error).toBeNull();
    expect(studentFlow.keadaan().consent?.state).toBe("ACTIVE");
    expect(studentFlow.keadaan().phase).toBe("prepare");
    const grantId = studentFlow.keadaan().consent?.recordId;
    if (!grantId) throw new Error("Missing actual consent grant");
    const fixture = path.join(repoRoot(), "research/fl-enr02a-host/evidence-local/enr04-r1/fixtures-v1-verified/procedural-mannequin-heightfield-v1-01.jpg");
    const jpeg = fs.readFileSync(fixture);
    let submitted;
    try {
      expect(sha256(jpeg)).toBe(FIXTURE_SHA256);
      submitted = value(await studentApi.submitJPEG(studentToken, grantId, jpeg), "R2 upload");
    } finally { jpeg.fill(0); }
    expect(submitted.effectiveStatus).toBe("PENDING_APPROVAL");
    expect(submitted.templateReady).toBe(false);
    const submissionId = submitted.submissionId;
    if (!submissionId || !UUID.test(submissionId)) throw new Error("Missing actual submission ID");
    await studentFlow.refresh();
    expect(studentFlow.keadaan().phase).toBe("pending");
    expect(studentFlow.trustedActive()).toBe(false);
    observeStudent("pending");
    const pending = await snapshot("pending", submissionId);
    expect(pending.status).toBe("PENDING_APPROVAL");
    expect(pending.photo_rows).toBe(1);
    expect(pending.template_rows).toBe(0);
    expect(pending.active_rows).toBe(0);
    expect(pending.processing_calls).toBe(0);
    expect(HASH.test(pending.canonical_sha256)).toBe(true);

    adminToken = await login("admin");
    adminFlow = new AdminReviewFlow(adminApi, async () => adminToken, () => {});
    await adminFlow.load();
    expect(adminFlow.snapshot().error).toBeNull();
    expect(adminFlow.snapshot().pendingCount).toBe(1);
    expect(adminFlow.snapshot().items.map(item => item.id)).toEqual([submissionId]);
    await adminFlow.open(submissionId);
    expect(adminFlow.snapshot().error).toBeNull();
    const detail = adminFlow.snapshot().detail;
    expect(detail?.effectiveStatus).toBe("PENDING_APPROVAL");
    expect(detail?.student.userId === current.fixtures.student!.user_id).toBe(true);
    expect(detail?.identityBinding.eligible).toBe(true);
    expect(detail?.student.classes[0]?.code).toBe("SYN-ENR02A");
    expect(detail?.consent.recordId === grantId).toBe(true);
    // The reviewed bytes are the canonical artifact, not necessarily the source
    // JPEG bytes (the server canonicalizes uploads). Never persist this preview.
    const preview = value(await adminApi.photo(adminToken, submissionId), "Protected preview");
    try { expect(sha256(preview)).toBe(pending.canonical_sha256); }
    finally { preview.fill(0); }
    const identityReference = current.fixtures.student!.identity_reference;
    if (!identityReference || !UUID.test(identityReference)) throw new Error("Missing synthetic UUID identity reference");
    expect(detail?.identityBinding.identityReference === identityReference).toBe(true);
    await adminFlow.decide({ kind: "approve", identityCheckMethod: "CAMPUS_AUTHORIZED_CHECK", campusCheckRef: identityReference, confirmed: true });
    expect(adminFlow.snapshot().error).toBeNull();
    expect(adminFlow.snapshot().lastResponseStatus).toBe(202);
    expect(adminFlow.snapshot().detail?.effectiveStatus).toBe("APPROVED_PROCESSING");
    expect(adminFlow.snapshot().detail?.templateReady).toBe(false);
    expect(statusLabel(adminFlow.snapshot().detail)).not.toBe("Aktif");
    consumer.push({ stage: "approve_receipt", phase: "APPROVED_PROCESSING", http_status: adminFlow.snapshot().lastResponseStatus! });
    await studentFlow.refresh();
    expect(studentFlow.keadaan().phase).toBe("processing");
    expect(studentFlow.trustedActive()).toBe(false);
    observeStudent("before_worker");
    const processing = await snapshot("before_worker", submissionId);
    expect(processing.status).toBe("APPROVED_PROCESSING");
    expect(processing.active_rows).toBe(0);
    expect(processing.template_rows).toBe(0);
    expect(processing.processing_calls).toBe(0);

    await control("worker.step");
    // Activation commits logical purge first; physical ciphertext removal is
    // completed by the existing durable reconciliation path, not inference.
    await control("reconcile");
    const active = await snapshot("after_real_python", submissionId);
    expect(active.status).toBe("ACTIVE");
    expect(active.processing_calls).toBe(1);
    expect(active.canonical_sha256).toBe(pending.canonical_sha256);
    expect(active.processed_sha256).toBe(pending.canonical_sha256);
    expect(active.face_count).toBe(1);
    expect(active.embedding_dim).toBe(128);
    expect(active.embedding_finite).toBe(true);
    expect(active.manifest_verified).toBe(true);
    expect(active.template_envelope_version).toBe(2);
    expect(active.template_binding_valid).toBe(true);
    expect(active.template_rows).toBe(1);
    expect(active.active_rows).toBe(1);
    expect(active.photo_rows).toBe(0);
    expect(active.ciphertext_files).toBe(0);
    expect(active.trace.some(t => t.stage === "python.process" && t.trace_id === current.trace_id && t.submission_id === submissionId && t.processed_sha256 === pending.canonical_sha256)).toBe(true);
    await studentFlow.refresh();
    expect(studentFlow.keadaan().phase).toBe("active");
    expect(studentFlow.keadaan().enrollment?.templateReady).toBe(true);
    expect(studentFlow.trustedActive()).toBe(true);
    observeStudent("active");
    await adminFlow.refresh();
    expect(adminFlow.snapshot().detail?.effectiveStatus).toBe("ACTIVE");
    expect(adminFlow.snapshot().detail?.templateReady).toBe(true);
    expect(adminFlow.snapshot().detail?.review?.reviewerAdminId === current.fixtures.admin!.user_id).toBe(true);
    expect(await adminApi.photo(adminToken, submissionId)).toMatchObject({ ok: false, error: { kind: "photoUnavailable" } });
    // Missing consumer edge in the isolated suites: a real persisted ACTIVE
    // snapshot loses trust on background and regains it only after actual reads.
    studentFlow.suspend();
    expect(studentFlow.trustedActive()).toBe(false);
    observeStudent("active_background");
    adminFlow.suspend();
    expect(adminFlow.snapshot().detail).toBeNull();
    await Promise.all([studentFlow.load(), adminFlow.refresh()]);
    expect(studentFlow.trustedActive()).toBe(true);
    expect(adminFlow.snapshot().stale).toBe(false);
    expect(adminFlow.snapshot().detail?.effectiveStatus).toBe("ACTIVE");
    observeStudent("active_resumed");

    // Credential unavailability is a provider gate, not a fake server response:
    // retain the actual persisted ACTIVE snapshot but immediately remove trust.
    credentialAvailable = false;
    await studentFlow.refresh();
    expect(studentFlow.keadaan().error?.kind).toBe("sessionEnded");
    expect(studentFlow.keadaan().stale).toBe(true);
    expect(studentFlow.keadaan().enrollment?.effectiveStatus).toBe("ACTIVE");
    expect(studentFlow.trustedActive()).toBe(false);
    observeStudent("active_credential_unavailable");
    credentialAvailable = true;
    await studentFlow.load();
    expect(studentFlow.keadaan().error).toBeNull();
    expect(studentFlow.keadaan().stale).toBe(false);
    expect(studentFlow.keadaan().enrollment?.effectiveStatus).toBe("ACTIVE");
    expect(studentFlow.trustedActive()).toBe(true);
    observeStudent("active_credential_restored_real_https");

    await studentFlow.tarik();
    expect(studentFlow.keadaan().error).toBeNull();
    expect(studentFlow.keadaan().phase).toBe("withdrawn");
    expect(studentFlow.keadaan().consent?.state).toBe("WITHDRAWN");
    expect(studentFlow.keadaan().consent?.recordId === grantId).toBe(true);
    expect(studentFlow.keadaan().enrollment?.effectiveStatus).toBe("REVOKED");
    expect(studentFlow.trustedActive()).toBe(false);
    observeStudent("withdrawn");
    await adminFlow.refresh();
    expect(adminFlow.snapshot().detail?.effectiveStatus).toBe("REVOKED");
    expect(adminFlow.snapshot().detail?.templateReady).toBe(false);
    expect(adminFlow.snapshot().detail?.photoAvailable).toBe(false);
    expect(await adminApi.photo(adminToken, submissionId)).toMatchObject({ ok: false, error: { kind: "photoUnavailable" } });
    await control("reconcile");
    const revoked = await snapshot("withdrawn_cleanup", submissionId);
    expect(revoked.status).toBe("REVOKED");
    expect(revoked.photo_rows).toBe(0);
    expect(revoked.template_rows).toBe(0);
    expect(revoked.active_rows).toBe(0);
    expect(revoked.ciphertext_files).toBe(0);
    expect(revoked.deletion_incomplete).toBe(false);
    expect(revoked.grant_identity_preserved).toBe(true);
    expect(revoked.processing_calls).toBe(1);
    expect(revoked.processed_sha256).toBe(pending.canonical_sha256);
    for (const action of ["consent.granted", "enrollment.submitted", "enrollment.queue_read", "enrollment.review_opened", "photo.preview_authorized", "enrollment.approved", "consent.withdrawn", "enrollment.revoked"]) {
      expect(revoked.audit.some(a => a.action === action && a.request_id === current.trace_id)).toBe(true);
    }
    for (const action of ["processing.started", "enrollment.activated"]) {
      expect(revoked.audit.some(a => a.action === action && a.request_id === "")).toBe(true);
    }
    positivePassed = true;
  }, 90_000);

  it("injects response loss after real HTTPS approval, replays exact intent, and withdraws without activation (not Android/network disconnect)", async () => {
    expect(positivePassed).toBe(true);
    expect(studentToken).toBeTruthy();
    expect(adminToken).toBeTruthy();
    studentFlow?.dispose();
    adminFlow?.dispose();
    // Exercise the same post-environment module-loading boundary as the positive
    // case; these modules import the API configuration used by the actual client.
    const { EnrollmentFlow } = await import("@/features/enrollment/flow");
    const { AdminReviewFlow } = await import("@/features/adminEnrollment/flow");
    const { statusLabel } = await import("@/features/adminEnrollment/model");
    studentFlow = new EnrollmentFlow(studentApi, async () => studentToken);
    adminFlow = new AdminReviewFlow(adminApi, async () => adminToken, () => {});
    await studentFlow.load();
    expect(studentFlow.keadaan().error).toBeNull();
    expect(studentFlow.keadaan().consent?.state).toBe("WITHDRAWN");
    const previousGrant = studentFlow.keadaan().consent?.recordId;
    const previousSubmission = studentFlow.keadaan().enrollment?.submissionId;
    studentFlow.masukConsent();
    await studentFlow.beriConsent();
    expect(studentFlow.keadaan().error).toBeNull();
    expect(studentFlow.keadaan().consent?.state).toBe("ACTIVE");
    const grantId = studentFlow.keadaan().consent?.recordId;
    if (!grantId || grantId === previousGrant) throw new Error("Missing distinct second consent grant");
    const jpeg = fs.readFileSync(path.join(repoRoot(), "research/fl-enr02a-host/evidence-local/enr04-r1/fixtures-v1-verified/procedural-mannequin-heightfield-v1-01.jpg"));
    let submitted;
    try {
      expect(sha256(jpeg)).toBe(FIXTURE_SHA256);
      submitted = value(await studentApi.submitJPEG(studentToken, grantId, jpeg), "Second R2 upload");
    } finally { jpeg.fill(0); }
    const submissionId = submitted.submissionId;
    if (!submissionId || !UUID.test(submissionId) || submissionId === previousSubmission) throw new Error("Missing distinct second submission");
    expect(submitted.effectiveStatus).toBe("PENDING_APPROVAL");
    await studentFlow.refresh();
    expect(studentFlow.keadaan().phase).toBe("pending");
    expect(studentFlow.trustedActive()).toBe(false);
    const before = await snapshot("response_loss_pending", submissionId);
    expect(before.status).toBe("PENDING_APPROVAL");
    expect(before.photo_rows).toBe(1);
    expect(before.active_rows).toBe(0);
    expect(before.processing_calls).toBe(0);
    const approvalCount = (data: Snapshot) => data.audit.filter(a => a.action === "enrollment.approved" && a.result === "success").length;
    const baselineApprovals = approvalCount(before);
    await adminFlow.load();
    expect(adminFlow.snapshot().items.map(item => item.id)).toEqual([submissionId]);
    await adminFlow.open(submissionId);
    expect(adminFlow.snapshot().error).toBeNull();
    expect(adminFlow.snapshot().detail?.consent.recordId).toBe(grantId);
    const identityReference = ready().fixtures.student!.identity_reference;
    if (!identityReference || !UUID.test(identityReference)) throw new Error("Missing synthetic identity reference");
    lossTarget = `/api/v1/face-enrollment-submissions/${submissionId}/approve`;
    dropNextApprovalResponse = true;
    await adminFlow.decide({ kind: "approve", identityCheckMethod: "CAMPUS_AUTHORIZED_CHECK", campusCheckRef: identityReference, confirmed: true });
    expect(dropNextApprovalResponse).toBe(false);
    expect(responseLoss.server_status).toBe(202);
    expect(approvalAttempts).toHaveLength(1);
    expect(adminFlow.snapshot().error?.kind).toBe("network");
    expect(adminFlow.snapshot().uncertain).toBe(true);
    expect(adminFlow.snapshot().stale).toBe(true);
    expect(adminFlow.snapshot().lastResponseStatus).toBeNull();
    expect(adminFlow.snapshot().detail?.effectiveStatus).not.toBe("ACTIVE");
    expect(statusLabel(adminFlow.snapshot().detail)).not.toBe("Aktif");
    responseLoss.uncertain_after_loss = adminFlow.snapshot().uncertain;
    consumer.push({ stage: "approval_response_loss", phase: adminFlow.snapshot().detail!.effectiveStatus });
    await studentFlow.refresh();
    expect(studentFlow.keadaan().phase).toBe("processing");
    expect(studentFlow.trustedActive()).toBe(false);
    observeStudent("response_loss_before_replay");
    const committed = await snapshot("response_loss_server_committed", submissionId);
    expect(committed.status).toBe("APPROVED_PROCESSING");
    expect(committed.photo_rows).toBe(1);
    expect(committed.active_rows).toBe(0);
    expect(committed.template_rows).toBe(0);
    expect(committed.processing_calls).toBe(0);
    responseLoss.no_active_after_loss = !studentFlow.trustedActive() && committed.active_rows === 0 && committed.template_rows === 0;
    responseLoss.approval_audit_delta = approvalCount(committed) - baselineApprovals;
    expect(responseLoss.approval_audit_delta).toBe(1);
    // No new decision or fresh CAS: replay the controller's frozen operation
    // through the actual Admin API and real Go idempotency receipt.
    await adminFlow.replay();
    expect(approvalAttempts).toHaveLength(2);
    const original = approvalAttempts[0]!;
    const replay = approvalAttempts[1]!;
    expect(original.idempotencyKey).toMatch(UUID);
    expect(original.ifMatch).toMatch(/^"(?:0|[1-9]\d*)"$/);
    responseLoss.exact_body_replay = replay.body === original.body;
    responseLoss.exact_cas_replay = replay.ifMatch === original.ifMatch;
    responseLoss.exact_idempotency_replay = replay.idempotencyKey === original.idempotencyKey;
    expect(responseLoss.exact_body_replay).toBe(true);
    expect(responseLoss.exact_cas_replay).toBe(true);
    expect(responseLoss.exact_idempotency_replay).toBe(true);
    expect(responseLoss.replay_status).toBe(200);
    expect(adminFlow.snapshot().lastResponseStatus).toBe(200);
    expect(adminFlow.snapshot().uncertain).toBe(false);
    expect(adminFlow.snapshot().error).toBeNull();
    expect(adminFlow.snapshot().detail?.effectiveStatus).toBe("APPROVED_PROCESSING");
    expect(adminFlow.snapshot().detail?.templateReady).toBe(false);
    expect(adminFlow.snapshot().detail?.history.filter(a => a.action === "enrollment.approved")).toHaveLength(1);
    const replayed = await snapshot("response_loss_exact_replay", submissionId);
    expect(replayed.status).toBe("APPROVED_PROCESSING");
    expect(replayed.active_rows).toBe(0);
    expect(replayed.template_rows).toBe(0);
    expect(replayed.processing_calls).toBe(0);
    responseLoss.replay_approval_audit_delta = approvalCount(replayed) - approvalCount(committed);
    expect(responseLoss.replay_approval_audit_delta).toBe(0);
    lossTarget = null;
    // The worker remains paused: withdrawal must delete the queued artifact
    // without ever creating a biometric template or an ACTIVE enrollment.
    await studentFlow.tarik();
    expect(studentFlow.keadaan().error).toBeNull();
    expect(studentFlow.keadaan().phase).toBe("withdrawn");
    expect(studentFlow.keadaan().consent?.recordId).toBe(grantId);
    expect(studentFlow.trustedActive()).toBe(false);
    await control("reconcile");
    await adminFlow.refresh();
    expect(adminFlow.snapshot().detail?.effectiveStatus).toBe("REVOKED");
    expect(adminFlow.snapshot().detail?.templateReady).toBe(false);
    expect(adminFlow.snapshot().detail?.photoAvailable).toBe(false);
    expect(await adminApi.photo(adminToken, submissionId)).toMatchObject({ ok: false, error: { kind: "photoUnavailable" } });
    const cleaned = await snapshot("response_loss_withdrawn_cleanup", submissionId);
    expect(cleaned.status).toBe("REVOKED");
    expect(cleaned.photo_rows).toBe(0);
    expect(cleaned.template_rows).toBe(0);
    expect(cleaned.active_rows).toBe(0);
    expect(cleaned.ciphertext_files).toBe(0);
    expect(cleaned.processing_calls).toBe(0);
    expect(cleaned.deletion_incomplete).toBe(false);
    expect(cleaned.grant_identity_preserved).toBe(true);
    expect(approvalCount(cleaned) - baselineApprovals).toBe(1);
    responseLoss.cleanup_zero = true;
    observeStudent("response_loss_withdrawn");
    // Intent fields are never serialized; persist only the comparison booleans.
    approvalAttempts.length = 0;
    responseLossPassed = true;
  }, 45_000);
});

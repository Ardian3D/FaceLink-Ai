import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";
import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import readline from "node:readline";

/**
 * Opt-in cross-stack E2E: the actual Go disposable server + mobile enrollment
 * client, verified HTTPS and normal email OTP. TLS trust must be supplied with
 * NODE_EXTRA_CA_CERTS; certificate or hostname verification is never disabled.
 * The deterministic processor here is NOT evidence of real-model detection.
 */
const enabled = process.env.FACELINK_ENR_E2E === "1";

type Startup = {
  base_url: string;
  root: string;
  otp_file: string;
  student_email: string;
  admin_email: string;
  person_id: string;
  class_id: string;
};
type StagingServer = {
  child: ChildProcessWithoutNullStreams;
  startup: Startup;
  lines: readline.Interface;
  control: (command: string, fields?: Record<string, unknown>) => Promise<void>;
};

function repoRoot(): string {
  let directory = path.resolve(import.meta.dirname);
  for (let i = 0; i < 8; i++) {
    if (fs.existsSync(path.join(directory, "backend", "go.mod"))) return directory;
    directory = path.dirname(directory);
  }
  throw new Error("backend/go.mod tidak ditemukan dari direktori tes");
}

async function startServer(): Promise<StagingServer> {
  const child = spawn(
    "go",
    [
      "run",
      "./cmd/enrollment-staging-serve",
      // Default staging uses verified HTTPS and exports no session credentials.
      "-addr",
      "127.0.0.1:0",
    ],
    {
      cwd: path.join(repoRoot(), "backend"),
      windowsHide: true,
      stdio: ["pipe", "pipe", "pipe"],
    }
  );
  const lines = readline.createInterface({ input: child.stdout });
  const controlWaiters: { command: string; resolve: () => void; reject: (error: Error) => void; timer: NodeJS.Timeout }[] = [];
  const rejectControls = () => {
    for (const waiter of controlWaiters.splice(0)) {
      clearTimeout(waiter.timer);
      waiter.reject(new Error("Staging control channel closed"));
    }
  };
  child.once("exit", rejectControls);
  lines.on("line", line => {
    try {
      const ack = JSON.parse(line);
      if (ack.fixture_control !== true) return;
      const waiter = controlWaiters.shift();
      if (!waiter) return;
      clearTimeout(waiter.timer);
      if (ack.command !== waiter.command || ack.ok !== true) waiter.reject(new Error("Staging control acknowledgement failed"));
      else waiter.resolve();
    } catch { rejectControls(); }
  });
  const control = async (command: string, fields: Record<string, unknown> = {}) => {
    const reply = Promise.withResolvers<void>();
    // This bounds an actual separate Go process; fake timers cannot drive it.
    const waiter = {
      command, resolve: reply.resolve, reject: reply.reject,
      timer: setTimeout(() => {
        const index = controlWaiters.indexOf(waiter);
        if (index >= 0) controlWaiters.splice(index, 1);
        reply.reject(new Error("Staging control acknowledgement deadline"));
      }, 10_000),
    };
    controlWaiters.push(waiter);
    child.stdin.write(`${JSON.stringify({ command, ...fields })}\n`, error => { if (error) rejectControls(); });
    await reply.promise;
  };
  const { promise, resolve, reject } = Promise.withResolvers<Startup>();
  // A separate Go process cannot be advanced by fake timers; this is the
  // readiness deadline only, not a fixed settle delay.
  const timer = setTimeout(() => reject(new Error("HTTPS staging server tidak siap")), 90_000);
  child.once("error", (error) => {
    clearTimeout(timer);
    reject(error);
  });
  child.once("exit", (code) => {
    clearTimeout(timer);
    reject(new Error(`staging server berhenti sebelum siap (${code})`));
  });
  lines.once("line", (line) => {
    try {
      const parsed = JSON.parse(line) as Startup;
      if (
        typeof parsed.base_url !== "string" ||
        !parsed.base_url.startsWith("https://127.0.0.1:") ||
        typeof parsed.otp_file !== "string" ||
        typeof parsed.student_email !== "string" ||
        typeof parsed.admin_email !== "string" ||
        typeof parsed.root !== "string"
      ) {
        throw new Error("startup JSON tidak lengkap");
      }
      clearTimeout(timer);
      resolve(parsed);
    } catch (error) {
      clearTimeout(timer);
      reject(error);
    }
  });
  try {
    const startup = await promise;
    return { child, startup, lines, control };
  } catch (error) {
    await stopServer(child);
    lines.close();
    throw error;
  } finally {
    clearTimeout(timer);
  }
}

async function stopServer(child: ChildProcessWithoutNullStreams, root?: string): Promise<void> {
  if (child.pid !== undefined && child.exitCode === null && child.signalCode === null) {
    const exited = Promise.withResolvers<void>();
    child.once("exit", () => exited.resolve());
    child.stdin.end();
    const timeout = setTimeout(() => child.kill(), 10_000);
    try {
      await exited.promise;
    } finally {
      clearTimeout(timeout);
    }
  }
  if (root !== undefined) expect(fs.existsSync(root)).toBe(false);
}

function delay(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms));
}


async function login(server: StagingServer, email: string): Promise<string> {
  const { startup } = server;
  const requested = await fetch(`${startup.base_url}/api/v1/auth/email/request`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email }),
  });
  if (requested.status !== 202) throw new Error("Synthetic OTP request failed");
  const challenge = await requested.json();
  // File publication alone does not prove the delivery transaction committed.
  await server.control("otp.wait", { challenge_id: challenge.data.challenge_id });
  const code = fs.readFileSync(startup.otp_file, "utf8").trim();
  const verified = await fetch(`${startup.base_url}/api/v1/auth/email/verify`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ challenge_id: challenge.data.challenge_id, code }),
  });
  if (verified.status !== 200) throw new Error("Synthetic OTP verification failed");
  const session = await verified.json();
  if (typeof session.data.access_token !== "string") throw new Error("Missing session credential");
  return session.data.access_token as string;
}

async function adminRequest(
  startup: Startup,
  adminToken: string,
  method: "GET" | "POST",
  route: string,
  body?: unknown,
  ifMatch?: string
): Promise<{ status: number; body: unknown; etag: string | null }> {
  const headers: Record<string, string> = {
    Authorization: `Bearer ${adminToken}`,
    Accept: "application/json",
  };
  if (body !== undefined) headers["Content-Type"] = "application/json";
  if (ifMatch) headers["If-Match"] = ifMatch;
  if (method === "POST") headers["Idempotency-Key"] = crypto.randomUUID();
  const response = await fetch(`${startup.base_url}${route}`, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const text = await response.text();
  let parsed: unknown = null;
  try {
    parsed = text === "" ? null : JSON.parse(text);
  } catch {
    parsed = null;
  }
  return { status: response.status, body: parsed, etag: response.headers.get("ETag") };
}

describe.skipIf(!enabled)("FL-ENR02A synthetic backend E2E", () => {
  let server: StagingServer | null = null;
  let studentToken: string;
  let adminToken: string;

  beforeAll(async () => {
    if (process.env.NODE_TLS_REJECT_UNAUTHORIZED === "0") {
      throw new Error("TLS verification bypass is forbidden in this integration test");
    }
    server = await startServer();
    vi.stubEnv("EXPO_PUBLIC_FACELINK_API_BASE_URL", server.startup.base_url);
    vi.stubGlobal("__DEV__", false);
    studentToken = await login(server, server.startup.student_email);
    adminToken = await login(server, server.startup.admin_email);
  }, 100_000);

  afterAll(async () => {
    if (server) {
      await stopServer(server.child, server.startup.root);
      server.lines.close();
    }
    vi.unstubAllEnvs();
    vi.unstubAllGlobals();
  }, 15_000);

  it("runs normal OTP → consent → synthetic JPEG → approve → ACTIVE → withdrawal over verified HTTPS", async () => {
    if (!server) throw new Error("staging server tidak tersedia");
    const api = await import("@/features/enrollment/api");
    const { syntheticJPEG } = await import("@/features/enrollment/fixture");
    const token = studentToken;

    const initial = await api.bacaConsent(token);
    expect(initial.ok).toBe(true);
    if (!initial.ok) throw new Error("consent awal gagal");
    expect(initial.data.consent.state).toBe("NOT_GIVEN");

    const granted = await api.beriConsent(
      token,
      initial.data.consent.notice.version,
      initial.data.etag
    );
    expect(granted.ok).toBe(true);
    if (!granted.ok) throw new Error("grant gagal");
    expect(granted.data.state).toBe("ACTIVE");

    const submitted = await api.kirimFotoSintetis(
      token,
      granted.data.recordId ?? "",
      syntheticJPEG()
    );
    expect(submitted.ok).toBe(true);
    if (!submitted.ok) throw new Error(`submit gagal: ${submitted.error.code ?? submitted.error.kind}`);
    expect(submitted.data.effectiveStatus).toBe("PENDING_APPROVAL");
    const submissionID = submitted.data.submissionId;
    expect(submissionID).toBeTruthy();
    if (!submissionID) throw new Error("submission id kosong");

    const detail = await adminRequest(
      server.startup,
      adminToken,
      "GET",
      `/api/v1/face-enrollment-submissions/${submissionID}`
    );
    expect(detail.status).toBe(200);
    expect(detail.etag).toBeTruthy();
    const approved = await adminRequest(
      server.startup,
      adminToken,
      "POST",
      `/api/v1/face-enrollment-submissions/${submissionID}/approve`,
      {
        identity_check_method: "CAMPUS_AUTHORIZED_CHECK",
        attestation_version: "synthetic-v1",
        confirmed: true,
        campus_check_ref: crypto.randomUUID(),
      },
      detail.etag ?? undefined
    );
    expect(approved.status).toBe(202);

    let active = false;
    for (let attempt = 0; attempt < 50; attempt++) {
      const status = await api.bacaStatus(token);
      expect(status.ok).toBe(true);
      if (status.ok && status.data.effectiveStatus === "ACTIVE" && status.data.templateReady) {
        active = true;
        break;
      }
      // The Go worker runs in a separate process; fake timers cannot advance it.
      // Poll a bounded condition and stop immediately when ACTIVE is observed.
      await delay(200);
    }
    expect(active).toBe(true);

    const withdrawn = await api.tarikConsent(token);
    expect(withdrawn.ok).toBe(true);
    if (!withdrawn.ok) throw new Error("withdraw gagal");
    expect(withdrawn.data.effectiveStatus).toBe("REVOKED");
    const after = await api.bacaStatus(token);
    expect(after.ok).toBe(true);
    if (!after.ok) throw new Error("status setelah withdraw gagal");
    expect(after.data.effectiveStatus).toBe("REVOKED");
    expect(after.data.templateReady).toBe(false);
  }, 30_000);
});

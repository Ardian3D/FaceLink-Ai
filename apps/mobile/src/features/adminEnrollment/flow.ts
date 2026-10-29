import { kunciIdempotensi } from "../enrollment/api";
import type { EnrollmentError, HasilEnrollment } from "../enrollment/model";
import { canApprove, canReject, canRetry, strongEtag, timestamp, UUID_V4, validDecision, type AdminApi, type AdminReviewState, type Decision, type DetailEnvelope, type Operation } from "./model";
export { canApprove, canReject, canRetry, statusLabel } from "./model";
export type { AdminReviewState } from "./model";

const TRUST_MS = 30_000;
function initial(): AdminReviewState {
  return { screen: "queue", selectedId: null, items: [], pendingCount: 0, nextCursor: null, hasMore: false, filter: "PENDING_APPROVAL", detail: null, etag: null, busy: null, error: null, denied: false, stale: true, uncertain: false, lastResponseStatus: null };
}
type RequestContext = { generation: number; controller: AbortController };
type Pending = { id: string; operation: Operation; sent: boolean };

/** Memory-only review state. The backend, not these guards, authorizes and commits. */
export class AdminReviewFlow {
  private state = initial();
  private generation = 0;
  private controller: AbortController | null = null;
  private disposed = false;
  private pending: Pending | null = null;
  private observation: { server: number; local: number } | null = null;
  private latestLocal: number | null = null;
  private rollback = false;
  private readonly observed = new Map<string, { revision: number; userId: string; personId: string }>();

  constructor(private readonly api: AdminApi, private readonly getToken: () => Promise<string | null>, private readonly onChange: (state: AdminReviewState) => void, private readonly clock: () => number = Date.now) {}

  snapshot(): AdminReviewState { return this.state; }
  private set(patch: Partial<AdminReviewState>): void {
    if (this.disposed) return;
    this.state = { ...this.state, ...patch };
    this.onChange(this.state);
  }
  private fence(keepUncertain = true): void {
    ++this.generation;
    this.controller?.abort();
    this.controller = null;
    if (this.pending !== null && !this.pending.sent) this.pending = null;
    if (keepUncertain && this.pending?.sent) this.set({ uncertain: true });
  }
  private begin(busy: AdminReviewState["busy"]): RequestContext {
    this.fence();
    const controller = new AbortController();
    this.controller = controller;
    this.set({ busy, error: null });
    return { generation: this.generation, controller };
  }
  private current(ctx: RequestContext): boolean { return !this.disposed && this.generation === ctx.generation && !ctx.controller.signal.aborted; }
  private async token(ctx: RequestContext): Promise<string | null> {
    let token: string | null = null;
    try { token = await this.getToken(); } catch { /* Fail closed without exposing provider errors. */ }
    if (!this.current(ctx)) return null;
    if (!token) {
      this.invalidate({ kind: "sessionEnded" });
      return null;
    }
    return token;
  }
  private async accept(ctx: RequestContext): Promise<boolean> {
    if (!this.current(ctx)) return false;
    const active = await this.token(ctx);
    return this.current(ctx) && active !== null;
  }
  private failure(error: EnrollmentError, ambiguous = false): void {
    if (error.kind === "sessionEnded" || error.kind === "forbidden") { this.invalidate(error); return; }
    this.set({ error, busy: null, stale: true, uncertain: ambiguous || this.state.uncertain });
  }
  private async safely<T>(call: () => Promise<HasilEnrollment<T>>): Promise<HasilEnrollment<T>> {
    try { return await call(); } catch { return { ok: false, error: { kind: "network" } }; }
  }
  now(): number {
    const local = this.clock();
    if (!Number.isFinite(local) || (this.latestLocal !== null && local < this.latestLocal)) this.rollback = true;
    this.latestLocal = local;
    if (this.rollback || this.observation === null) return Number.NaN;
    return this.observation.server + (local - this.observation.local);
  }
  private observe(serverNow: string): boolean {
    const local = this.clock(), server = Date.parse(serverNow);
    this.now();
    if (this.rollback || !timestamp(serverNow) || !Number.isFinite(local) || (this.observation !== null && server < this.observation.server)) return false;
    this.observation = { server, local };
    return true;
  }
  tick(): void {
    const now = this.now(), view = this.state.detail;
    const old = !Number.isFinite(now) || this.observation === null || now - this.observation.server >= TRUST_MS;
    const expired = view !== null && ((view.photoAvailable && view.photoExpiresAt !== null && Date.parse(view.photoExpiresAt) <= now) || (view.effectiveStatus === "APPROVED_PROCESSING" && view.processingDeadline !== null && Date.parse(view.processingDeadline) <= now));
    if ((old || expired) && !this.state.stale) this.set({ stale: true });
  }
  private apply(envelope: DetailEnvelope, id: string): boolean {
    const view = envelope.view, previous = this.observed.get(id);
    if (view.submissionId !== id || !view.syntheticOnly || !strongEtag(envelope.etag, view.revision) || (previous !== undefined && (view.revision < previous.revision || view.student.userId !== previous.userId || view.student.personId !== previous.personId)) || !this.observe(view.serverNow)) {
      this.failure({ kind: "precondition" });
      return false;
    }
    this.observed.set(id, { revision: view.revision, userId: view.student.userId, personId: view.student.personId });
    this.set({ detail: view, etag: envelope.etag, stale: false, busy: this.state.busy === "decision" ? "decision" : null, error: null });
    this.tick();
    return true;
  }
  async load(status = "PENDING_APPROVAL", append = false): Promise<void> {
    if (this.disposed || this.state.denied || this.state.busy === "decision") return;
    if (append && (this.state.filter !== status || !this.state.hasMore || !this.state.nextCursor)) return;
    const cursor = append ? this.state.nextCursor : null;
    const ctx = this.begin("queue");
    this.set({ screen: "queue", selectedId: null, detail: null, etag: null, stale: true, filter: status, ...(append ? {} : { items: [], nextCursor: null, hasMore: false, pendingCount: 0 }), lastResponseStatus: null });
    const token = await this.token(ctx);
    if (!token) return;
    const result = await this.safely(() => this.api.list(token, status, cursor, ctx.controller.signal));
    if (!await this.accept(ctx)) return;
    if (!result.ok) { this.failure(result.error); return; }
    if (!this.observe(result.data.serverNow)) { this.failure({ kind: "precondition" }); return; }
    const items = append ? [...this.state.items] : [];
    for (const item of result.data.items) {
      const index = items.findIndex(row => row.id === item.id);
      if (index >= 0) items[index] = item; else items.push(item);
    }
    this.set({ items, pendingCount: result.data.pendingCount, nextCursor: result.data.nextCursor, hasMore: result.data.hasMore, busy: null, stale: false });
  }
  async open(id: string): Promise<void> {
    if (this.disposed || this.state.denied) return;
    const ctx = this.begin("detail");
    this.set({ screen: "detail", selectedId: id, detail: null, etag: null, stale: true, lastResponseStatus: null });
    if (!UUID_V4.test(id)) { this.failure({ kind: "validation" }); return; }
    await this.readDetail(ctx, id);
  }
  private async readDetail(ctx: RequestContext, id: string): Promise<void> {
    const token = await this.token(ctx);
    if (!token) return;
    const result = await this.safely(() => this.api.detail(token, id, ctx.controller.signal));
    if (!await this.accept(ctx)) return;
    if (!result.ok) { this.failure(result.error); return; }
    this.apply(result.data, id);
  }
  back(): void {
    if (this.disposed) return;
    this.fence();
    this.set({ screen: "queue", selectedId: null, detail: null, etag: null, stale: true, busy: null, error: null, lastResponseStatus: null });
  }
  async refresh(): Promise<void> {
    if (this.disposed || this.state.denied || this.state.busy !== null) return;
    if (this.state.screen === "queue" || this.state.selectedId === null) { await this.load(this.state.filter); return; }
    const id = this.state.selectedId, ctx = this.begin("detail");
    this.set({ stale: true });
    await this.readDetail(ctx, id);
  }
  async decide(decision: Decision): Promise<void> {
    if (this.disposed || this.state.denied || this.state.busy !== null || this.state.uncertain || this.pending !== null) return;
    // Freeze the confirmed intent before token acquisition and preflight await.
    decision = { ...decision };
    this.tick();
    const id = this.state.selectedId, snapshot = this.state.detail, etag = this.state.etag;
    const guard = decision.kind === "approve" ? canApprove : decision.kind === "reject" ? canReject : canRetry;
    if (!id || !snapshot || !etag || !validDecision(decision) || !guard(snapshot, this.now(), this.state.stale)) { this.failure({ kind: "precondition" }); return; }
    const ctx = this.begin("decision"), token = await this.token(ctx);
    if (!token) return;
    const fresh = await this.safely(() => this.api.detail(token, id, ctx.controller.signal));
    if (!await this.accept(ctx)) return;
    if (!fresh.ok) { this.failure(fresh.error); return; }
    if (!this.apply(fresh.data, id)) return;
    if (fresh.data.etag !== etag || fresh.data.view.revision !== snapshot.revision || !guard(fresh.data.view, this.now(), this.state.stale)) { this.failure({ kind: "precondition" }); return; }
    this.pending = { id, operation: { idempotencyKey: kunciIdempotensi(), ifMatch: fresh.data.etag, decision: { ...decision } }, sent: false };
    await this.send(ctx, this.pending);
  }
  async replay(): Promise<void> {
    if (this.disposed || this.state.denied || this.state.busy !== null || !this.state.uncertain || !this.pending) return;
    const pending = this.pending, ctx = this.begin("decision");
    this.set({ screen: "detail", selectedId: pending.id, detail: null, etag: null, stale: true });
    await this.send(ctx, pending);
  }
  private async send(ctx: RequestContext, pending: Pending): Promise<void> {
    this.set({ busy: "decision" });
    // The provider owns the authenticated user/role lifetime; bearer rotation is not logout.
    const token = await this.token(ctx);
    if (!token) return;
    pending.sent = true;
    const result = await this.safely(() => this.api.decide(token, pending.id, { ...pending.operation, decision: { ...pending.operation.decision } }, ctx.controller.signal));
    if (!await this.accept(ctx)) return;
    if (!result.ok) {
      // Transport, timeout, cancellation, malformed receipt and 5xx cannot prove non-commit.
      // Validation/400/422 means the server rejected the request; only transport,
      // timeout, cancellation, malformed receipt, or 5xx leave commit unknown.
      const ambiguous = ["network", "tls", "cancelled", "unknown", "unavailable"].includes(result.error.kind);
      if (!ambiguous) this.pending = null;
      this.failure(result.error, ambiguous);
      if (!ambiguous && !this.state.denied) this.set({ uncertain: false });
      return;
    }
    if (!this.apply(result.data, pending.id)) { this.set({ uncertain: true }); return; }
    this.pending = null;
    this.set({ uncertain: false, busy: null, lastResponseStatus: result.data.httpStatus });
  }
  /** Backgrounding drops visible sensitive data but cannot erase an ambiguous mutation. */
  suspend(): void {
    if (this.disposed) return;
    this.fence(false);
    this.set({
      items: [], pendingCount: 0, nextCursor: null, hasMore: false,
      detail: null, etag: null, busy: null, stale: true, error: null,
      uncertain: this.state.uncertain || this.pending?.sent === true,
      lastResponseStatus: null,
    });
  }
  invalidate(error?: EnrollmentError): void {
    if (this.disposed) return;
    this.fence(false);
    this.pending = null;
    this.observation = null;
    this.latestLocal = null;
    this.rollback = false;
    this.observed.clear();
    this.set({ ...initial(), filter: this.state.filter, denied: error?.kind === "sessionEnded" || error?.kind === "forbidden", error: error ?? null });
  }
  dispose(): void {
    if (this.disposed) return;
    this.disposed = true;
    this.fence();
    this.pending = null;
    this.observation = null;
    this.observed.clear();
    this.state = initial();
  }
}

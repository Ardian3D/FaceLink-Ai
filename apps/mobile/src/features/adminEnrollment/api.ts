import { konfigurasiApi } from "@/features/api/config";
import { adalahObjek, bacaJson, mintaHttp, type JawabanHttp } from "@/features/api/http";
import type { EnrollmentError, HasilEnrollment } from "../enrollment/model";
import { parseAdminDetail, parseQueuePage, strongEtag, UUID_V4, STATUSES, validDecision, type AdminApi, type Decision, type Operation } from "./model";

const TIMEOUT_MS = 20_000;
export const MAX_PREVIEW_BYTES = 1024 * 1024;
const ROOT = "/api/v1/face-enrollment-submissions";
const invalid = (): { ok: false; error: EnrollmentError } => ({ ok: false, error: { kind: "validation" } });
function httpError(status: number, code?: string, retryAfter?: string | null): EnrollmentError {
  const kinds: Record<number, EnrollmentError["kind"]> = { 400: "validation", 401: "sessionEnded", 403: "forbidden", 404: "notFound", 409: "conflict", 410: "photoUnavailable", 412: "precondition", 413: "validation", 415: "validation", 422: "validation", 428: "precondition", 429: "rateLimited", 503: "unavailable" };
  const error: EnrollmentError = { kind: kinds[status] ?? "unknown" };
  if (code && /^[A-Z][A-Z0-9_]{0,79}$/.test(code)) error.code = code;
  if (status === 429 && retryAfter && /^\d+$/.test(retryAfter)) error.retryAfter = Math.min(300, Math.max(1, Number(retryAfter)));
  return error;
}
function transportError(answer: Exclude<JawabanHttp, { jenis: "jawaban" }>): EnrollmentError {
  return { kind: answer.jenis !== "timeout" && /certificate|ssl|tls|trust anchor|self.signed/i.test(answer.pesan) ? "tls" : "network" };
}
function bodyFor(d: Decision): unknown {
  if (d.kind === "approve") return { identity_check_method: d.identityCheckMethod, attestation_version: "synthetic-v1", confirmed: true, campus_check_ref: d.campusCheckRef };
  if (d.kind === "reject") return { reason_code: d.reasonCode };
  return { confirmed: true };
}
async function request(token: string, path: string, signal?: AbortSignal, operation?: Operation): Promise<HasilEnrollment<Extract<JawabanHttp, { jenis: "jawaban" }>>> {
  if (!konfigurasiApi.ok) return { ok: false, error: { kind: "config" } };
  if (!token) return { ok: false, error: { kind: "sessionEnded" } };
  if (signal?.aborted) return { ok: false, error: { kind: "cancelled" } };
  const answer = await mintaHttp(path, {
    baseUrl: konfigurasiApi.baseUrl, token, signal, timeoutMs: TIMEOUT_MS,
    method: operation ? "POST" : "GET", body: operation ? bodyFor(operation.decision) : undefined,
    headers: { "Cache-Control": "no-store", ...(operation ? { "If-Match": operation.ifMatch, "Idempotency-Key": operation.idempotencyKey } : {}) },
  });
  if (signal?.aborted) return { ok: false, error: { kind: "cancelled" } };
  if (answer.jenis !== "jawaban") return { ok: false, error: transportError(answer) };
  if (answer.status < 200 || answer.status >= 300) {
    const payload = bacaJson(answer.teks);
    const code = adalahObjek(payload) && adalahObjek(payload.error) && typeof payload.error.code === "string" ? payload.error.code : undefined;
    return { ok: false, error: httpError(answer.status, code, answer.retryAfter) };
  }
  return { ok: true, data: answer };
}

export const adminApi: AdminApi = {
  async list(token, status, cursor, signal) {
    if ((status !== "" && (!STATUSES.includes(status as typeof STATUSES[number]) || status === "NOT_ENROLLED")) || (cursor !== null && (cursor.length > 4096 || /[\r\n]/.test(cursor)))) return invalid();
    // An empty filter means "all"; sending status= would be a rejected empty query value.
    const query = `${status ? `?status=${encodeURIComponent(status)}` : ""}${cursor ? `${status ? "&" : "?"}cursor=${encodeURIComponent(cursor)}` : ""}`;
    const result = await request(token, ROOT + query, signal);
    if (!result.ok) return result;
    const page = result.data.status === 200 ? parseQueuePage(bacaJson(result.data.teks)) : null;
    if (!page || (status !== "" && page.items.some(item => item.status !== status))) return invalid();
    return { ok: true, data: page };
  },
  async detail(token, id, signal) {
    if (!UUID_V4.test(id)) return invalid();
    const result = await request(token, `${ROOT}/${id}`, signal);
    if (!result.ok) return result;
    const payload = bacaJson(result.data.teks);
    const view = result.data.status === 200 && adalahObjek(payload) ? parseAdminDetail(payload.data, id) : null;
    if (!view || !strongEtag(result.data.etag, view.revision)) return invalid();
    return { ok: true, data: { view, etag: result.data.etag } };
  },
  async decide(token, id, operation, signal) {
    if (!UUID_V4.test(id) || !UUID_V4.test(operation.idempotencyKey) || !strongEtag(operation.ifMatch) || !validDecision(operation.decision)) return invalid();
    const suffix = operation.decision.kind === "retry" ? "retry-processing" : operation.decision.kind;
    const result = await request(token, `${ROOT}/${id}/${suffix}`, signal, operation);
    if (!result.ok) return result;
    const payload = bacaJson(result.data.teks);
    const receipt = adalahObjek(payload) && adalahObjek(payload.data) ? payload.data : null;
    const view = receipt ? parseAdminDetail(receipt.current, id) : null;
    // Replay is 200 with the CURRENT snapshot, which can differ from the original decision.
    // A 2xx with an unusable receipt cannot prove non-commit; keep it ambiguous.
    if (![200, 202].includes(result.data.status) || !receipt || receipt.resource_id !== id || typeof receipt.operation_id !== "string" || !UUID_V4.test(receipt.operation_id) || typeof receipt.applied_revision !== "number" || !Number.isSafeInteger(receipt.applied_revision) || !view || receipt.applied_revision <= Number(operation.ifMatch.slice(1, -1)) || receipt.applied_revision > view.revision || !strongEtag(result.data.etag, view.revision)) return { ok: false, error: { kind: "unknown", code: "MALFORMED_RECEIPT" } };
    if (result.data.status === 202 && (operation.decision.kind === "reject" || view.effectiveStatus !== "APPROVED_PROCESSING" || view.templateReady)) return { ok: false, error: { kind: "unknown", code: "MALFORMED_RECEIPT" } };
    return { ok: true, data: { view, etag: result.data.etag, httpStatus: result.data.status } };
  },
  async photo(token, id, signal) {
    if (!konfigurasiApi.ok) return { ok: false, error: { kind: "config" } };
    if (!UUID_V4.test(id)) return invalid();
    if (!token) return { ok: false, error: { kind: "sessionEnded" } };
    if (signal?.aborted) return { ok: false, error: { kind: "cancelled" } };
    const controller = new AbortController();
    const abort = () => controller.abort();
    signal?.addEventListener("abort", abort);
    const timer = setTimeout(abort, TIMEOUT_MS);
    let reader: ReadableStreamDefaultReader<Uint8Array> | undefined;
    try {
      const response = await fetch(`${konfigurasiApi.baseUrl}${ROOT}/${id}/photo`, {
        method: "GET", headers: { Authorization: `Bearer ${token}`, Accept: "image/jpeg", "Cache-Control": "no-store" }, cache: "no-store", credentials: "omit", redirect: "error", signal: controller.signal,
      });
      if (controller.signal.aborted) return { ok: false, error: { kind: signal?.aborted ? "cancelled" : "network" } };
      if (response.status !== 200) return { ok: false, error: httpError(response.status, undefined, response.headers.get("Retry-After")) };
      const length = response.headers.get("Content-Length");
      if (response.headers.get("Content-Type")?.trim().toLowerCase() !== "image/jpeg" || !/(?:^|,)\s*no-store\s*(?:,|$)/i.test(response.headers.get("Cache-Control") ?? "") || (length !== null && (!/^\d+$/.test(length) || Number(length) < 4 || Number(length) > MAX_PREVIEW_BYTES))) return invalid();
      let bytes: Uint8Array;
      if (response.body && typeof response.body.getReader === "function") {
        reader = response.body.getReader();
        const chunks: Uint8Array[] = [];
        let size = 0;
        while (true) {
          const part = await reader.read();
          if (controller.signal.aborted) return { ok: false, error: { kind: signal?.aborted ? "cancelled" : "network" } };
          if (part.done) break;
          size += part.value.byteLength;
          if (size > MAX_PREVIEW_BYTES) return invalid();
          chunks.push(part.value);
        }
        bytes = new Uint8Array(size);
        let offset = 0;
        for (const chunk of chunks) { bytes.set(chunk, offset); offset += chunk.length; }
      } else {
        // Native fetch lacks streaming. A bounded declared length is mandatory there.
        if (length === null) return invalid();
        bytes = new Uint8Array(await response.arrayBuffer());
      }
      if (controller.signal.aborted) return { ok: false, error: { kind: signal?.aborted ? "cancelled" : "network" } };
      if (bytes.length < 4 || bytes.length > MAX_PREVIEW_BYTES || (length !== null && bytes.length !== Number(length)) || bytes[0] !== 0xff || bytes[1] !== 0xd8 || bytes[bytes.length - 2] !== 0xff || bytes[bytes.length - 1] !== 0xd9) return invalid();
      return { ok: true, data: bytes };
    } catch (error) {
      return { ok: false, error: { kind: signal?.aborted ? "cancelled" : error instanceof Error && /certificate|ssl|tls|trust anchor|self.signed/i.test(error.message) ? "tls" : "network" } };
    } finally {
      controller.abort();
      if (reader) { void reader.cancel().catch(() => {}); }
      clearTimeout(timer);
      signal?.removeEventListener("abort", abort);
    }
  },
};

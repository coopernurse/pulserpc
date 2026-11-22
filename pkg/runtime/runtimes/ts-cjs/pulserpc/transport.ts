/**
 * Transport abstraction for JSON-RPC requests.
 *
 * Provides abstract Transport base class and concrete implementations:
 * - HttpTransport: HTTP-based transport using fetch
 * - InProcTransport: In-process transport for testing
 *
 * Resource/security invariants (D6/D13/R17/R19/R29/R30):
 * - requests time out after 30 s via AbortController;
 * - response bodies are capped at 10 MiB;
 * - custom headers are supported and a trailing `/` on the base URL is
 *   normalized away.
 */

import type { JsonRpcRequest, JsonRpcResponse } from "./types";
import { RPCError } from "./rpc";

/** Default HTTP request timeout (D6/R19): 30 seconds. */
export const HTTP_TIMEOUT_MS = 30_000;
/** Maximum accepted response body (D5/R17): 10 MiB. */
export const MAX_RESPONSE_BYTES = 10 * 1024 * 1024;

/**
 * UTF-8 byte length without assuming a Node-only global.
 */
function byteLength(text: string): number {
  if (typeof Buffer !== "undefined") {
    return Buffer.byteLength(text, "utf8");
  }
  return new TextEncoder().encode(text).length;
}

/** Decode previously collected UTF-8 chunks without assuming a Node-only global. */
function decodeUtf8(chunks: Uint8Array[], total: number): string {
  if (total === 0) {
    return "";
  }
  const merged = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    merged.set(chunk, offset);
    offset += chunk.byteLength;
  }
  if (typeof Buffer !== "undefined") {
    return Buffer.from(merged).toString("utf8");
  }
  return new TextDecoder().decode(merged);
}

/**
 * Release a response body that is not going to be read.
 *
 * Any throw path that skips the body (non-OK status, declared-length over the
 * cap) must cancel the stream first so the socket is not kept alive (R17).
 */
async function discardResponseBody(response: Response): Promise<void> {
  try {
    await response.body?.cancel();
  } catch {
    // The body may already be consumed or locked; nothing left to release.
  }
}

/**
 * Read a response body while enforcing a hard byte cap (D5/R17).
 *
 * Reading via `response.text()` first would buffer an arbitrarily large body
 * before the cap could be applied, so the stream is consumed chunk by chunk and
 * cancelled as soon as the running total exceeds `maxBytes`. If no readable
 * `body` is present (204 responses or test doubles) the cap is still checked
 * after `text()`.
 */
async function readBodyWithCap(response: Response, maxBytes: number): Promise<string> {
  const body = response.body as ReadableStream<Uint8Array> | null | undefined;
  if (!body || typeof (body as any).getReader !== "function") {
    const text = await response.text();
    const actual = byteLength(text);
    if (actual > maxBytes) {
      throw new RPCError(
        -32603,
        `Response body too large: ${actual} bytes exceeds limit of ${maxBytes}`
      );
    }
    return text;
  }

  const reader = body.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;
  let overflow = false;
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) {
        break;
      }
      if (!value || value.byteLength === 0) {
        continue;
      }
      total += value.byteLength;
      if (total > maxBytes) {
        overflow = true;
        break;
      }
      chunks.push(value);
    }
  } finally {
    if (overflow) {
      // Stop pulling bytes and release the underlying stream as soon as the
      // cap is crossed instead of draining a hostile body into memory (R17).
      try {
        await reader.cancel();
      } catch {
        // Stream already closed by the peer.
      }
    }
    try {
      reader.releaseLock();
    } catch {
      // Reader already released.
    }
  }
  if (overflow) {
    throw new RPCError(
      -32603,
      `Response body too large: ${total} bytes exceeds limit of ${maxBytes}`
    );
  }
  return decodeUtf8(chunks, total);
}

/**
 * Normalize a base URL by dropping a single trailing slash (D13/R30). An
 * endpoint path such as `/api/rpc` is otherwise used verbatim.
 */
export function normalizeBaseUrl(baseUrl: string): string {
  if (baseUrl.length > 1 && baseUrl.endsWith("/")) {
    return baseUrl.slice(0, -1);
  }
  return baseUrl;
}

/**
 * Abstract transport base class
 */
export abstract class Transport {
  abstract request(_req: JsonRpcRequest): Promise<JsonRpcResponse>;

  close(): void {
    // Default implementation does nothing
  }
}

/**
 * HTTP transport implementation using fetch
 */
export class HttpTransport extends Transport {
  baseUrl: string;
  headers: Record<string, string>;
  fetchFn: typeof fetch;
  /** Per-request timeout in milliseconds (D6). */
  timeoutMs: number;
  /** Maximum response body size in bytes (D5/R17). */
  maxResponseBytes: number;

  constructor(
    baseUrl: string,
    headers: Record<string, string> = {},
    fetchFn: typeof fetch = fetch,
    timeoutMs: number = HTTP_TIMEOUT_MS
  ) {
    super();
    this.baseUrl = normalizeBaseUrl(baseUrl);
    this.headers = {
      "Content-Type": "application/json",
      ...headers,
    };
    this.fetchFn = fetchFn;
    this.timeoutMs = timeoutMs;
    this.maxResponseBytes = MAX_RESPONSE_BYTES;
  }

  async request(req: JsonRpcRequest): Promise<JsonRpcResponse> {
    const url = this.baseUrl;
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), this.timeoutMs);

    try {
      const response = await this.fetchFn(url, {
        method: "POST",
        headers: this.headers,
        body: JSON.stringify(req),
        signal: controller.signal,
      });

      if (!response.ok) {
        await discardResponseBody(response);
        throw new RPCError(-32603, `HTTP ${response.status}: ${response.statusText}`, { status: response.status });
      }

      const declared = Number(response.headers?.get?.("content-length"));
      if (Number.isFinite(declared) && declared > this.maxResponseBytes) {
        await discardResponseBody(response);
        throw new RPCError(
          -32603,
          `Response body too large: ${declared} bytes exceeds limit of ${this.maxResponseBytes}`
        );
      }

      const text = await readBodyWithCap(response, this.maxResponseBytes);
      if (!text) {
        // A notification (no `id`) is answered with 204/No Content. There is no
        // response document to parse, so return a neutral envelope rather than
        // reporting a transport failure (R11 / M4).
        if (
          response.status === 204 &&
          !Object.prototype.hasOwnProperty.call(req, "id")
        ) {
          return { jsonrpc: "2.0", id: req.id ?? null };
        }
        throw new RPCError(-32603, "Empty response body");
      }

      try {
        return JSON.parse(text);
      } catch (e: any) {
        throw new RPCError(-32603, `Invalid JSON response: ${e.message}`);
      }
    } catch (e: any) {
      if (controller.signal.aborted && !(e instanceof RPCError)) {
        throw new RPCError(-32603, `Request timed out after ${this.timeoutMs} ms`);
      }
      throw e;
    } finally {
      clearTimeout(timer);
    }
  }
}

/**
 * In-process transport for testing.
 *
 * This transport calls the server directly without HTTP.
 */
export class InProcTransport extends Transport {
  serverCallFn: (req: JsonRpcRequest) => JsonRpcResponse | null | undefined;

  constructor(serverCallFn: (req: JsonRpcRequest) => JsonRpcResponse | null | undefined) {
    super();
    this.serverCallFn = serverCallFn;
  }

  async request(req: JsonRpcRequest): Promise<JsonRpcResponse> {
    const response = this.serverCallFn(req);
    if (!response) {
      // Emit a well-formed error envelope: a bare `{jsonrpc, id}` response has
      // neither `result` nor `error` and is rejected by client validation.
      // (Notifications are unaffected - Client ignores their response.)
      return {
        jsonrpc: "2.0",
        id: req.id ?? null,
        error: { code: -32603, message: "No response from server" },
      };
    }
    return response;
  }
}

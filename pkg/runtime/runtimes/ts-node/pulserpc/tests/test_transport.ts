/**
 * Tests for HttpTransport URL handling, limits and error behavior
 *
 * Covers D6/R17/R19 (30 s timeout, 10 MiB response cap), D13/R29/R30
 * (custom headers, trailing-slash normalization) and R8 (standard codes).
 */

import { strict as assert } from "assert";
import {
  HttpTransport,
  JsonRpcRequest,
  HTTP_TIMEOUT_MS,
  MAX_RESPONSE_BYTES,
} from "../transport.js";
import { RPCError } from "../rpc.js";

/**
 * Fake response body stream. Counts reads and records cancellation so tests can
 * assert the transport stops pulling bytes at the cap (R17).
 */
function fakeBody(
  chunks: Uint8Array[],
  onCancel?: () => void,
  readCount?: { reads: number }
): ReadableStream<Uint8Array> {
  let index = 0;
  return {
    getReader() {
      return {
        read: async () => {
          if (readCount) {
            readCount.reads += 1;
          }
          if (index >= chunks.length) {
            return { done: true, value: undefined };
          }
          const value = chunks[index++];
          return { done: false, value };
        },
        cancel: async () => {
          onCancel?.();
        },
        releaseLock: () => {
          // no-op
        },
      };
    },
    cancel: async () => {
      onCancel?.();
    },
  } as unknown as ReadableStream<Uint8Array>;
}

/** Build a 200 response whose body streams `body` in two chunks. */
function chunkedResponse(
  body: string,
  extra: Record<string, unknown> = {},
  onCancel?: () => void
): Response {
  const bytes = new TextEncoder().encode(body);
  const mid = Math.ceil(bytes.length / 2);
  const chunks = bytes.length === 0 ? [] : [bytes.slice(0, mid), bytes.slice(mid)];
  return {
    ok: true,
    status: 200,
    statusText: "OK",
    headers: { get: () => null },
    body: fakeBody(chunks, onCancel),
    text: async () => body,
    ...extra,
  } as unknown as Response;
}

function recordingFetch(): { urls: string[]; fn: typeof fetch } {
  const urls: string[] = [];
  const fn = (async (input: any) => {
    urls.push(String(input));
    return chunkedResponse(JSON.stringify({ jsonrpc: "2.0", result: "ok", id: 1 }));
  }) as unknown as typeof fetch;
  return { urls, fn };
}

const req: JsonRpcRequest = { jsonrpc: "2.0", method: "Foo.bar", id: 1 };

async function testBaseUrlTrailingSlashNormalized() {
  // D13/R30: a trailing "/" is normalized away; other URLs are verbatim.
  const cases: Array<[string, string]> = [
    ["http://localhost:8080", "http://localhost:8080"],
    ["http://localhost:8080/", "http://localhost:8080"],
    ["http://localhost:8080/api/rpc", "http://localhost:8080/api/rpc"],
  ];
  for (const [url, expected] of cases) {
    const { urls, fn } = recordingFetch();
    const transport = new HttpTransport(url, {}, fn);
    await transport.request(req);
    assert.strictEqual(
      urls[0],
      expected,
      `expected fetch to be called with "${expected}", got "${urls[0]}"`
    );
  }
  console.log("✓ testBaseUrlTrailingSlashNormalized");
}

async function testCustomHeadersAreSent() {
  // R29: caller-supplied headers must reach fetch alongside Content-Type.
  let init: any = null;
  const fn = (async (_input: any, options: any) => {
    init = options;
    return chunkedResponse(JSON.stringify({ jsonrpc: "2.0", result: "ok", id: 1 }));
  }) as unknown as typeof fetch;

  const transport = new HttpTransport("http://localhost:8080", { "X-Auth": "token-1" }, fn);
  await transport.request(req);

  assert.strictEqual(init.method, "POST");
  assert.strictEqual(init.headers["Content-Type"], "application/json");
  assert.strictEqual(init.headers["X-Auth"], "token-1");
  assert.ok(init.signal, "requests must carry an AbortSignal (R19)");
  console.log("✓ testCustomHeadersAreSent");
}

async function testDefaultLimits() {
  assert.strictEqual(HTTP_TIMEOUT_MS, 30_000, "D6 requires a 30 s timeout");
  assert.strictEqual(MAX_RESPONSE_BYTES, 10 * 1024 * 1024, "D5 requires a 10 MiB cap");
  const transport = new HttpTransport("http://localhost:8080", {}, recordingFetch().fn);
  assert.strictEqual(transport.timeoutMs, 30_000);
  assert.strictEqual(transport.maxResponseBytes, 10 * 1024 * 1024);
  console.log("✓ testDefaultLimits");
}

async function testRequestTimeout() {
  // R19: an AbortController-driven timeout surfaces as a standard RPCError.
  const hangingFetch = ((_input: any, options: any) =>
    new Promise((_resolve, reject) => {
      options.signal.addEventListener("abort", () => {
        const err: any = new Error("aborted");
        err.name = "AbortError";
        reject(err);
      });
    })) as unknown as typeof fetch;

  const transport = new HttpTransport("http://localhost:8080", {}, hangingFetch, 10);
  try {
    await transport.request(req);
    assert.fail("Expected timeout RPCError");
  } catch (e: any) {
    assert.ok(e instanceof RPCError, `expected RPCError, got ${e?.constructor?.name}`);
    assert.strictEqual(e.code, -32603);
    assert.match(e.message, /timed out after 10 ms/);
  }
  console.log("✓ testRequestTimeout");
}

async function testResponseBodyCap() {
  // R17: reject a body larger than maxResponseBytes, whether declared or read.
  // Both paths must release the body instead of leaving it unread.
  const cases: Array<[string, (onCancel: () => void) => any]> = [
    [
      "content-length",
      (onCancel) => ({
        ok: true,
        status: 200,
        statusText: "OK",
        headers: { get: (name: string) => (name.toLowerCase() === "content-length" ? "17" : null) },
        body: fakeBody([new TextEncoder().encode("small")], onCancel),
        text: async () => "small",
      }),
    ],
    [
      "actual bytes",
      (onCancel) => chunkedResponse("0123456789abcdefg", {}, onCancel),
    ],
  ];

  for (const [label, makeResponse] of cases) {
    let cancelled = false;
    const response = makeResponse(() => {
      cancelled = true;
    });
    const transport = new HttpTransport(
      "http://localhost:8080",
      {},
      (async () => response) as unknown as typeof fetch
    );
    transport.maxResponseBytes = 16;
    try {
      await transport.request(req);
      assert.fail(`Expected response-cap RPCError for ${label}`);
    } catch (e: any) {
      assert.ok(e instanceof RPCError, `expected RPCError for ${label}`);
      assert.strictEqual(e.code, -32603, `${label} must use a standard code`);
      assert.match(e.message, /Response body too large/, `${label} message: ${e.message}`);
    }
    assert.ok(cancelled, `${label}: the response body must be cancelled (R17)`);
  }
  console.log("✓ testResponseBodyCap");
}

async function testOversizeBodyStopsReading() {
  // R17: the cap must bound memory. Reading must stop at the chunk that crosses
  // the limit instead of draining the whole body first.
  const readCount = { reads: 0 };
  let cancelled = false;
  const chunk = new Uint8Array(8).fill(65); // 8 bytes per read
  const response = {
    ok: true,
    status: 200,
    statusText: "OK",
    headers: { get: () => null },
    body: fakeBody([chunk, chunk, chunk, chunk, chunk], () => {
      cancelled = true;
    }, readCount),
  } as unknown as Response;

  const transport = new HttpTransport(
    "http://localhost:8080",
    {},
    (async () => response) as unknown as typeof fetch
  );
  transport.maxResponseBytes = 10;

  try {
    await transport.request(req);
    assert.fail("Expected response-cap RPCError");
  } catch (e: any) {
    assert.ok(e instanceof RPCError);
    assert.strictEqual(e.code, -32603);
    assert.match(e.message, /Response body too large/);
  }
  assert.strictEqual(
    readCount.reads,
    2,
    `reader must stop at the chunk crossing the cap, read ${readCount.reads} chunks`
  );
  assert.ok(cancelled, "the stream must be cancelled when the cap is crossed");
  console.log("✓ testOversizeBodyStopsReading");
}

async function testNonOkResponseCancelsBody() {
  // R17: a throw that skips the body must still release the stream.
  let cancelled = false;
  const failingFetch = (async () => ({
    ok: false,
    status: 503,
    statusText: "Service Unavailable",
    headers: { get: () => null },
    body: fakeBody([new TextEncoder().encode("{}")], () => {
      cancelled = true;
    }),
    text: async () => "{}",
  })) as unknown as typeof fetch;
  const transport = new HttpTransport("http://localhost:8080", {}, failingFetch);
  try {
    await transport.request(req);
    assert.fail("Expected RPCError to be thrown");
  } catch (e: any) {
    assert.ok(e instanceof RPCError, `Expected RPCError, got ${e?.constructor?.name}`);
    // R8: transport failures use a standard JSON-RPC code, never -32000.
    assert.strictEqual(e.code, -32603);
    assert.ok(e.message.includes("503"), `Expected message to include status code, got: ${e.message}`);
    assert.deepStrictEqual(e.data, { status: 503 });
  }
  assert.ok(cancelled, "non-OK responses must cancel the unread body");
  console.log("✓ testNonOkResponseCancelsBody");
}

async function testHttpErrorWrapsRPCError() {
  const failingFetch = (async () => ({
    ok: false,
    status: 503,
    statusText: "Service Unavailable",
    headers: { get: () => null },
    body: fakeBody([new TextEncoder().encode("{}")]),
    text: async () => "{}",
  })) as unknown as typeof fetch;
  const transport = new HttpTransport("http://localhost:8080", {}, failingFetch);
  try {
    await transport.request(req);
    assert.fail("Expected RPCError to be thrown");
  } catch (e: any) {
    assert.ok(e instanceof RPCError, `Expected RPCError, got ${e?.constructor?.name}`);
    // R8: transport failures use a standard JSON-RPC code, never -32000.
    assert.strictEqual(e.code, -32603);
    assert.ok(e.message.includes("503"), `Expected message to include status code, got: ${e.message}`);
    assert.deepStrictEqual(e.data, { status: 503 });
  }
  console.log("✓ testHttpErrorWrapsRPCError");
}

async function testMalformedResponsesUseStandardCode() {
  const cases: Array<[string, typeof fetch]> = [
    [
      "empty body",
      (async () => ({
        ok: true,
        status: 200,
        statusText: "OK",
        text: async () => "",
      })) as unknown as typeof fetch,
    ],
    [
      "invalid JSON",
      (async () => ({
        ok: true,
        status: 200,
        statusText: "OK",
        text: async () => "{not json",
      })) as unknown as typeof fetch,
    ],
  ];
  for (const [label, fetchFn] of cases) {
    const transport = new HttpTransport("http://localhost:8080", {}, fetchFn);
    try {
      await transport.request(req);
      assert.fail(`Expected RPCError for ${label}`);
    } catch (e: any) {
      assert.ok(e instanceof RPCError, `Expected RPCError for ${label}`);
      assert.strictEqual(e.code, -32603, `${label} must use a standard error code`);
    }
  }
  console.log("✓ testMalformedResponsesUseStandardCode");
}

async function testNotificationEmptyBodyIsNotAnError() {
  const notificationFetch = (async () => ({
    ok: true,
    status: 204,
    statusText: "No Content",
    text: async () => "",
  })) as unknown as typeof fetch;
  const transport = new HttpTransport("http://localhost:8080", {}, notificationFetch);
  const response = await transport.request({
    jsonrpc: "2.0",
    method: "Foo.notify",
  });
  assert.strictEqual(response.jsonrpc, "2.0");
  console.log("✓ testNotificationEmptyBodyIsNotAnError");
}

(async () => {
  await testBaseUrlTrailingSlashNormalized();
  await testCustomHeadersAreSent();
  await testDefaultLimits();
  await testRequestTimeout();
  await testResponseBodyCap();
  await testOversizeBodyStopsReading();
  await testNonOkResponseCancelsBody();
  await testHttpErrorWrapsRPCError();
  await testMalformedResponsesUseStandardCode();
  await testNotificationEmptyBodyIsNotAnError();
  console.log("\nAll transport tests passed!");
})().catch((e) => {
  console.error(e);
  process.exit(1);
});

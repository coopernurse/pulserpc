/**
 * Tests for HttpTransport (CommonJS variant)
 *
 * Covers D6/R17/R19 (30 s timeout, 10 MiB response cap), D13/R29/R30
 * (custom headers, trailing-slash normalization) and R8 (standard codes).
 */

const { strict: assert } = require("assert");
const {
  HttpTransport,
  InProcTransport,
  HTTP_TIMEOUT_MS,
  MAX_RESPONSE_BYTES,
} = require("../transport");
const { RPCError } = require("../rpc");

// Fake response body stream. Counts reads and records cancellation so tests can
// assert the transport stops pulling bytes at the cap (R17).
function fakeBody(chunks, onCancel, readCount) {
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
          return { done: false, value: chunks[index++] };
        },
        cancel: async () => {
          if (onCancel) onCancel();
        },
        releaseLock: () => {},
      };
    },
    cancel: async () => {
      if (onCancel) onCancel();
    },
  };
}

// Build a 200 response whose body streams `body` in two chunks.
function chunkedResponse(body, extra = {}, onCancel) {
  const bytes = Buffer.from(body, "utf8");
  const mid = Math.ceil(bytes.length / 2);
  const chunks = bytes.length === 0 ? [] : [bytes.subarray(0, mid), bytes.subarray(mid)];
  return {
    ok: true,
    status: 200,
    statusText: "OK",
    headers: { get: () => null },
    body: fakeBody(chunks, onCancel),
    text: async () => body,
    ...extra,
  };
}

// Builds a fake fetch that records the URL it was called with and returns a
// minimal, valid JSON-RPC response.
function recordingFetch() {
  const urls = [];
  const fn = async (input) => {
    urls.push(String(input));
    return chunkedResponse(JSON.stringify({ jsonrpc: "2.0", result: "ok", id: 1 }));
  };
  return { urls, fn };
}

const req = { jsonrpc: "2.0", method: "Foo.bar", id: 1 };

async function testBaseUrlTrailingSlashNormalized() {
  // D13/R30: one trailing "/" is normalized away; other URLs are verbatim.
  const cases = [
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
  let init = null;
  const fn = async (_input, options) => {
    init = options;
    return chunkedResponse(JSON.stringify({ jsonrpc: "2.0", result: "ok", id: 1 }));
  };

  const transport = new HttpTransport("http://localhost:8080", { "X-Auth": "token-1" }, fn);
  await transport.request(req);

  assert.strictEqual(init.method, "POST");
  assert.strictEqual(init.headers["Content-Type"], "application/json");
  assert.strictEqual(init.headers["X-Auth"], "token-1");
  assert.ok(init.signal, "requests must carry an AbortSignal (R19)");
  console.log("✓ testCustomHeadersAreSent");
}

function testDefaultLimits() {
  assert.strictEqual(HTTP_TIMEOUT_MS, 30000, "D6 requires a 30 s timeout");
  assert.strictEqual(MAX_RESPONSE_BYTES, 10 * 1024 * 1024, "D5 requires a 10 MiB cap");
  const transport = new HttpTransport("http://localhost:8080", {}, recordingFetch().fn);
  assert.strictEqual(transport.timeoutMs, 30000);
  assert.strictEqual(transport.maxResponseBytes, 10 * 1024 * 1024);
  console.log("✓ testDefaultLimits");
}

async function testRequestTimeout() {
  // R19: an AbortController-driven timeout surfaces as a standard RPCError.
  const hangingFetch = (_input, options) =>
    new Promise((_resolve, reject) => {
      options.signal.addEventListener("abort", () => {
        const err = new Error("aborted");
        err.name = "AbortError";
        reject(err);
      });
    });

  const transport = new HttpTransport("http://localhost:8080", {}, hangingFetch, 10);
  let caught = null;
  try {
    await transport.request(req);
  } catch (e) {
    caught = e;
  }
  assert.ok(caught instanceof RPCError, `expected RPCError, got ${caught}`);
  assert.strictEqual(caught.code, -32603);
  assert.match(caught.message, /timed out after 10 ms/);
  console.log("✓ testRequestTimeout");
}

async function testResponseBodyCap() {
  // R17: reject a body larger than maxResponseBytes, whether declared or read.
  // Both paths must release the body instead of leaving it unread.
  const cases = [
    [
      "content-length",
      (onCancel) => ({
        ok: true,
        status: 200,
        statusText: "OK",
        headers: { get: (name) => (name.toLowerCase() === "content-length" ? "17" : null) },
        body: fakeBody([Buffer.from("small", "utf8")], onCancel),
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
    const transport = new HttpTransport("http://localhost:8080", {}, async () => response);
    transport.maxResponseBytes = 16;
    let caught = null;
    try {
      await transport.request(req);
    } catch (e) {
      caught = e;
    }
    assert.ok(caught instanceof RPCError, `${label}: expected RPCError, got ${caught}`);
    assert.strictEqual(caught.code, -32603, `${label}: expected -32603, got ${caught.code}`);
    assert.match(caught.message, /Response body too large/, `${label}: ${caught.message}`);
    assert.ok(cancelled, `${label}: the response body must be cancelled (R17)`);
  }
  console.log("✓ testResponseBodyCap");
}

async function testOversizeBodyStopsReading() {
  // R17: the cap must bound memory. Reading must stop at the chunk that crosses
  // the limit instead of draining the whole body first.
  const readCount = { reads: 0 };
  let cancelled = false;
  const chunk = Buffer.alloc(8, 65); // 8 bytes per read
  const response = {
    ok: true,
    status: 200,
    statusText: "OK",
    headers: { get: () => null },
    body: fakeBody([chunk, chunk, chunk, chunk, chunk], () => {
      cancelled = true;
    }, readCount),
  };

  const transport = new HttpTransport("http://localhost:8080", {}, async () => response);
  transport.maxResponseBytes = 10;

  let caught = null;
  try {
    await transport.request(req);
  } catch (e) {
    caught = e;
  }
  assert.ok(caught instanceof RPCError, `expected RPCError, got ${caught}`);
  assert.strictEqual(caught.code, -32603);
  assert.match(caught.message, /Response body too large/);
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
  const failingFetch = async () => ({
    ok: false,
    status: 503,
    statusText: "Service Unavailable",
    headers: { get: () => null },
    body: fakeBody([Buffer.from("{}", "utf8")], () => {
      cancelled = true;
    }),
    text: async () => "{}",
  });
  const transport = new HttpTransport("http://localhost:8080", {}, failingFetch);
  let caught = null;
  try {
    await transport.request(req);
  } catch (e) {
    caught = e;
  }
  assert.ok(caught instanceof RPCError, `expected RPCError, got ${caught}`);
  assert.strictEqual(caught.code, -32603);
  assert.match(caught.message, /HTTP 503/);
  assert.ok(cancelled, "non-OK responses must cancel the unread body");
  console.log("✓ testNonOkResponseCancelsBody");
}

async function testTransportErrorsUseStandardCodes() {
  // R8: transport/parse failures must use documented JSON-RPC codes.
  const cases = [
    {
      name: "non-OK response",
      fetchFn: async () => ({ ok: false, status: 500, statusText: "Server Error", text: async () => "" }),
      message: /HTTP 500/,
    },
    {
      name: "empty body",
      fetchFn: async () => ({ ok: true, status: 200, statusText: "OK", text: async () => "" }),
      message: /Empty response body/,
    },
    {
      name: "invalid JSON",
      fetchFn: async () => ({ ok: true, status: 200, statusText: "OK", text: async () => "not json" }),
      message: /Invalid JSON response/,
    },
  ];

  for (const { name, fetchFn, message } of cases) {
    const transport = new HttpTransport("http://localhost:8080", {}, fetchFn);
    let caught = null;
    try {
      await transport.request(req);
    } catch (e) {
      caught = e;
    }
    assert.ok(caught instanceof RPCError, `${name}: expected RPCError, got ${caught}`);
    assert.strictEqual(caught.code, -32603, `${name}: expected standard code -32603, got ${caught.code}`);
    assert.ok(message.test(caught.message), `${name}: unexpected message ${caught.message}`);
  }
  console.log("✓ testTransportErrorsUseStandardCodes");
}

async function testNotificationEmptyBodyIsNotAnError() {
  const notificationFetch = async () => ({
    ok: true,
    status: 204,
    statusText: "No Content",
    text: async () => "",
  });
  const transport = new HttpTransport("http://localhost:8080", {}, notificationFetch);
  const response = await transport.request({ jsonrpc: "2.0", method: "Foo.notify" });
  assert.strictEqual(response.jsonrpc, "2.0");
  console.log("✓ testNotificationEmptyBodyIsNotAnError");
}

async function testInProcFallbackIsWellFormedError() {
  // When the server function returns nothing, InProcTransport must emit a
  // well-formed error response (result XOR error) rather than a bare envelope.
  const transport = new InProcTransport(() => undefined);
  const resp = await transport.request({ jsonrpc: "2.0", method: "Svc.f", id: 7 });
  assert.strictEqual(resp.jsonrpc, "2.0");
  assert.strictEqual(resp.id, 7);
  assert.strictEqual(Object.prototype.hasOwnProperty.call(resp, "result"), false);
  assert.ok(resp.error, "fallback must carry an error member");
  assert.strictEqual(resp.error.code, -32603);
  assert.match(resp.error.message, /No response from server/);
  console.log("✓ testInProcFallbackIsWellFormedError");
}

testBaseUrlTrailingSlashNormalized()
  .then(testCustomHeadersAreSent)
  .then(() => testDefaultLimits())
  .then(testRequestTimeout)
  .then(testResponseBodyCap)
  .then(testOversizeBodyStopsReading)
  .then(testNonOkResponseCancelsBody)
  .then(testTransportErrorsUseStandardCodes)
  .then(testNotificationEmptyBodyIsNotAnError)
  .then(testInProcFallbackIsWellFormedError)
  .then(() => console.log("\nAll transport tests passed!"))
  .catch((e) => {
    console.error(e);
    process.exit(1);
  });

/**
 * Tests for HttpTransport URL handling and error behavior
 *
 * Verifies that HttpTransport uses the endpoint URL verbatim and wraps
 * transport-level errors in RPCError.
 */

import { strict as assert } from "assert";
import { HttpTransport, JsonRpcRequest } from "../transport.js";
import { RPCError } from "../rpc.js";

function recordingFetch(): { urls: string[]; fn: typeof fetch } {
  const urls: string[] = [];
  const fn = (async (input: any) => {
    urls.push(String(input));
    const body = JSON.stringify({ jsonrpc: "2.0", result: "ok", id: 1 });
    return {
      ok: true,
      status: 200,
      statusText: "OK",
      text: async () => body,
    } as Response;
  }) as unknown as typeof fetch;
  return { urls, fn };
}

const req: JsonRpcRequest = { jsonrpc: "2.0", method: "Foo.bar", id: 1 };

async function testUrlUsedVerbatim() {
  const cases = [
    "http://localhost:8080",
    "http://localhost:8080/",
    "http://localhost:8080/api/rpc",
  ];
  for (const url of cases) {
    const { urls, fn } = recordingFetch();
    const transport = new HttpTransport(url, {}, fn);
    await transport.request(req);
    assert.strictEqual(
      urls[0],
      url,
      `expected fetch to be called with "${url}", got "${urls[0]}"`
    );
  }
  console.log("✓ testUrlUsedVerbatim");
}

async function testHttpErrorWrapsRPCError() {
  const failingFetch = (async () => ({
    ok: false,
    status: 503,
    statusText: "Service Unavailable",
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
  await testUrlUsedVerbatim();
  await testHttpErrorWrapsRPCError();
  await testMalformedResponsesUseStandardCode();
  await testNotificationEmptyBodyIsNotAnError();
  console.log("\nAll transport tests passed!");
})().catch((e) => {
  console.error(e);
  process.exit(1);
});

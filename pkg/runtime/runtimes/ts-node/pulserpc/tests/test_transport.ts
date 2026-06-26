/**
 * Tests for HttpTransport URL handling
 *
 * Pins the guarantee that HttpTransport uses the endpoint URL verbatim:
 * it must neither strip a trailing slash nor append one before calling fetch.
 */

import { strict as assert } from "assert";
import { HttpTransport, JsonRpcRequest } from "../transport.js";

// Builds a fake fetch that records the URL it was called with and returns a
// minimal, valid JSON-RPC response.
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

testUrlUsedVerbatim()
  .then(() => console.log("\nAll transport tests passed!"))
  .catch((e) => {
    console.error(e);
    process.exit(1);
  });

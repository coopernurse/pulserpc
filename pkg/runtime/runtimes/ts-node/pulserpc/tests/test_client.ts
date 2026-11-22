/**
 * Unit tests for Client: JSON-RPC response-envelope validation, null-result
 * validation, request validation and contract verification (ts-node-03,
 * ts-node-04, ts-node-05, ts-node-07, ts-node-12).
 */

import { strict as assert } from "assert";
import { Client } from "../client.js";
import { FailFastAuditor } from "../contract.js";
import { RPCError } from "../rpc.js";
import { Transport, JsonRpcRequest, JsonRpcResponse } from "../transport.js";
import { Severity } from "../types.js";

function serverIDL(): Record<string, any> {
  return {
    interfaces: [
      {
        name: "Svc",
        methods: [
          {
            name: "req",
            parameters: [{ name: "s", type: { builtIn: "string" } }],
            returnType: { builtIn: "string" },
          },
          {
            name: "opt",
            parameters: [{ name: "s", type: { builtIn: "string" } }],
            returnType: { builtIn: "string" },
            returnOptional: true,
          },
        ],
      },
    ],
    structs: [{ name: "T", fields: [{ name: "shared", type: { builtIn: "string" } }] }],
    enums: [],
  };
}

/** Transport that answers pulserpc-idl with the server IDL and templated calls. */
class StubTransport extends Transport {
  template: any = null;

  constructor(private idl: Record<string, any>) {
    super();
  }

  async request(req: JsonRpcRequest): Promise<JsonRpcResponse> {
    if (req.method === "pulserpc-idl") {
      return { jsonrpc: "2.0", result: this.idl, id: req.id };
    }
    if (typeof this.template === "function") {
      return this.template(req);
    }
    if (this.template) {
      return { ...this.template, id: req.id };
    }
    return { jsonrpc: "2.0", result: "ok", id: req.id };
  }
}

async function createClient(
  validateResponse: boolean,
  validateRequest: boolean = false,
  options?: any
): Promise<{ client: Client; transport: StubTransport }> {
  const transport = new StubTransport(serverIDL());
  const client = await Client.create(transport, validateRequest, validateResponse, options);
  return { client, transport };
}

async function expectCallToThrow(client: Client, template: any, pattern: RegExp): Promise<void> {
  (client.transport as StubTransport).template = template;
  try {
    await client.call("Svc.req", ["x"]);
    assert.fail(`expected client.call to throw matching ${pattern}`);
  } catch (e: any) {
    assert.ok(pattern.test(e.message), `unexpected error: ${e.message}`);
  }
}

async function testEnvelopeValidation() {
  const { client } = await createClient(false);

  await expectCallToThrow(client, { jsonrpc: "1.0", result: "ok" }, /jsonrpc/);
  await expectCallToThrow(
    client,
    (req: JsonRpcRequest) => ({ jsonrpc: "2.0", result: "ok", id: Number(req.id) + 100 }),
    /expected id/
  );
  await expectCallToThrow(
    client,
    { jsonrpc: "2.0", result: "ok", error: { code: -32603, message: "x" } },
    /both 'result' and 'error'/
  );
  await expectCallToThrow(client, { jsonrpc: "2.0" }, /missing 'result' and 'error'/);
  console.log("✓ testEnvelopeValidation");
}

async function testNumericIdComparedByValue() {
  // M24/D2: numeric ids are compared by value. An echoed id that is
  // numerically equal to the request id must be accepted, regardless of how
  // the number was produced on the wire.
  const { client } = await createClient(false);
  (client.transport as StubTransport).template = (req: JsonRpcRequest) => ({
    jsonrpc: "2.0",
    result: "ok",
    id: parseFloat(String(req.id)),
  });
  assert.strictEqual(await client.call("Svc.req", ["x"]), "ok");
  console.log("✓ testNumericIdComparedByValue");
}

async function testErrorCodeZeroIsPreserved() {
  const { client } = await createClient(false);
  (client.transport as StubTransport).template = {
    jsonrpc: "2.0",
    error: { code: 0, message: "zero is a valid code" },
  };
  try {
    await client.call("Svc.req", ["x"]);
    assert.fail("expected RPCError");
  } catch (e: any) {
    assert.ok(e instanceof RPCError);
    assert.strictEqual(e.code, 0, "?? must not remap code 0 to -32603");
  }
  console.log("✓ testErrorCodeZeroIsPreserved");
}

async function testNullResultValidation() {
  const { client } = await createClient(true);
  (client.transport as StubTransport).template = { jsonrpc: "2.0", result: null };

  // A required return may not be null when response validation is enabled.
  try {
    await client.call("Svc.req", ["x"]);
    assert.fail("expected required-return null to be rejected");
  } catch (e: any) {
    assert.match(e.message, /Response validation failed/);
  }

  // An optional return may be null.
  assert.strictEqual(await client.call("Svc.opt", ["x"]), null);
  console.log("✓ testNullResultValidation");
}

async function testRequestValidation() {
  const { client } = await createClient(false, true);
  try {
    await client.call("Svc.req", [123]);
    assert.fail("expected request validation to fail");
  } catch (e: any) {
    assert.match(e.message, /Request validation failed/);
  }
  console.log("✓ testRequestValidation");
}

async function testConcurrentCallsUseUniqueIds() {
  // R20: the id counter increments synchronously before the transport await,
  // so concurrent calls cannot share an id.
  const { client } = await createClient(false);
  const ids: Array<string | number | null | undefined> = [];
  (client.transport as StubTransport).template = async (req: JsonRpcRequest) => {
    ids.push(req.id);
    // Yield so every call is genuinely in flight together.
    await new Promise((resolve) => setTimeout(resolve, 0));
    return { jsonrpc: "2.0" as const, result: req.id as any, id: req.id };
  };

  const count = 100;
  const results = await Promise.all(
    Array.from({ length: count }, () => client.call("Svc.req", ["x"]))
  );

  assert.strictEqual(ids.length, count);
  assert.strictEqual(new Set(ids).size, count, "every concurrent call must use a unique id");
  assert.strictEqual(new Set(results).size, count, "every response id must be distinct");
  console.log("✓ testConcurrentCallsUseUniqueIds");
}

async function testHostileMetaFromServerIsRejected() {
  // R28/S5: meta comes from the server's pulserpc-idl response and must not be
  // able to pollute Object.prototype through the wire.
  const hostile = JSON.parse(
    '{"interfaces":[],"structs":[],"enums":[],' +
      '"meta":{"ok":"v","__proto__":{"polluted":true},' +
      '"constructor":{"polluted":true},"prototype":{"polluted":true}}}'
  );
  const transport = new StubTransport(hostile);
  const client = await Client.create(transport);

  const meta = (client.contract as any).meta;
  assert.strictEqual(meta.ok, "v", "benign meta keys must survive");
  assert.strictEqual(meta.polluted, undefined);
  assert.strictEqual(Object.getPrototypeOf(meta), Object.prototype);
  for (const key of ["__proto__", "constructor", "prototype"]) {
    assert.strictEqual(Object.prototype.hasOwnProperty.call(meta, key), false);
  }
  assert.strictEqual(({} as any).polluted, undefined, "Object.prototype must stay clean");
  console.log("✓ testHostileMetaFromServerIsRejected");
}

async function testMalformedErrorEnvelopeFallsBack() {
  // M25: a non-numeric code or non-string message must not be trusted.
  const { client } = await createClient(false);
  (client.transport as StubTransport).template = {
    jsonrpc: "2.0",
    error: { code: "not-a-number", message: 42 },
  };
  try {
    await client.call("Svc.req", ["x"]);
    assert.fail("expected RPCError");
  } catch (e: any) {
    assert.ok(e instanceof RPCError);
    assert.strictEqual(e.code, -32603);
    assert.strictEqual(e.message, "Unknown error");
  }
  console.log("✓ testMalformedErrorEnvelopeFallsBack");
}

async function testVerifyCompatibilityUsesSeverityEnum() {
  const { client } = await createClient(false, false, { auditor: new FailFastAuditor() });

  // Canonical contract (core-12): a field the server removed from the client's
  // IDL is ClientHasMore -> Info, so compatibility stays true.
  const removedFieldIDL = {
    interfaces: [],
    structs: [
      {
        name: "T",
        fields: [
          { name: "shared", type: { builtIn: "string" } },
          { name: "extra", type: { builtIn: "string" } },
        ],
      },
    ],
    enums: [],
  };
  client.setLocalIDL(JSON.stringify(removedFieldIDL));
  const compatible = await client.verifyCompatibility();
  assert.strictEqual(compatible.compatible, true);
  assert.strictEqual(compatible.deltas[0].severity, Severity.Info);

  // A required field added by the server (ClientHasLess) is the Error case.
  const addedRequiredIDL = {
    interfaces: [],
    structs: [
      {
        name: "T",
        fields: [
          { name: "shared", type: { builtIn: "string" } },
          { name: "requiredNew", type: { builtIn: "string" }, optional: false },
        ],
      },
    ],
    enums: [],
  };
  // Swap the local/server roles by making the IDL the client's be the smaller one.
  const swappedTransport = new StubTransport(addedRequiredIDL);
  const swapped = await Client.create(swappedTransport, false, false, { auditor: new FailFastAuditor() });
  swapped.setLocalIDL(JSON.stringify({ interfaces: [], structs: [{ name: "T", fields: [{ name: "shared", type: { builtIn: "string" } }] }], enums: [] }));
  try {
    await swapped.verifyCompatibility();
    assert.fail("FailFastAuditor must throw for an Error delta");
  } catch (e: any) {
    assert.match(e.message, /compatibility verification failed/);
  }
  // Verify the severity decision itself on a client without the auditor.
  const plainTransport = new StubTransport(addedRequiredIDL);
  const plain = await Client.create(plainTransport);
  plain.setLocalIDL(JSON.stringify({ interfaces: [], structs: [{ name: "T", fields: [{ name: "shared", type: { builtIn: "string" } }] }], enums: [] }));
  const result = await plain.verifyCompatibility();
  assert.strictEqual(result.compatible, false);
  assert.strictEqual(result.deltas[0].severity, Severity.Error);
  console.log("✓ testVerifyCompatibilityUsesSeverityEnum");
}

(async () => {
  await testEnvelopeValidation();
  await testNumericIdComparedByValue();
  await testErrorCodeZeroIsPreserved();
  await testNullResultValidation();
  await testRequestValidation();
  await testConcurrentCallsUseUniqueIds();
  await testHostileMetaFromServerIsRejected();
  await testMalformedErrorEnvelopeFallsBack();
  await testVerifyCompatibilityUsesSeverityEnum();
  console.log("\nAll client tests passed!");
})().catch((e) => {
  console.error(e);
  process.exit(1);
});

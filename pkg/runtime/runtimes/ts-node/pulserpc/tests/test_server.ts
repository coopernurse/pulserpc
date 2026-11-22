/**
 * Unit tests for the transport-independent Server: response validation,
 * notification silence, id echo, parameter handling, internal-error redaction
 * and JSON-RPC envelope shapes (ts-node-01, ts-node-02, ts-node-03, ts-node-06;
 * R15/R24, M18-M28).
 */

import { strict as assert } from "assert";
import { Server } from "../server.js";
import { Contract } from "../contract.js";
import { RPCError } from "../rpc.js";
import { JsonRpcRequest, JsonRpcResponse } from "../transport.js";

function idl(): Record<string, any> {
  return {
    interfaces: [
      {
        name: "Svc",
        methods: [
          {
            name: "greet",
            parameters: [{ name: "name", type: { builtIn: "string" } }],
            returnType: { builtIn: "string" },
          },
          {
            name: "pair",
            parameters: [
              { name: "a", type: { builtIn: "int" } },
              { name: "b", type: { builtIn: "string" } },
            ],
            returnType: { builtIn: "string" },
          },
          {
            name: "wrong",
            parameters: [],
            returnType: { builtIn: "string" },
          },
          {
            name: "nullable",
            parameters: [],
            returnType: { builtIn: "string" },
            returnOptional: true,
          },
          { name: "boom", parameters: [], returnType: { builtIn: "string" } },
          { name: "declared", parameters: [], returnType: { builtIn: "string" } },
          {
            name: "opt",
            parameters: [
              { name: "a", type: { builtIn: "int" } },
              { name: "b", type: { builtIn: "string" }, optional: true },
            ],
            returnType: { builtIn: "int" },
          },
        ],
      },
    ],
    structs: [],
    enums: [],
  };
}

function makeServer(validateResponses: boolean): Server {
  const server = new Server({
    contract: new Contract(idl()),
    validateRequests: true,
    validateResponses,
  });
  server.addHandler("Svc", {
    greet(_ctx: any, name: string) { return `hi ${name}`; },
    pair(_ctx: any, a: number, b: string) { return `${a}:${b}`; },
    wrong(_ctx: any) { return 123; }, // wrong type for a string return
    nullable(_ctx: any) { return null; },
    boom(_ctx: any) { throw new Error("secret-token-123"); },
    declared(_ctx: any) { throw new RPCError(1002, "NotFound", { detail: "missing" }); },
    opt(_ctx: any, a: number, b?: string) { return b === undefined ? a : Number(b); },
  });
  return server;
}

function request(method: string, params?: any, id?: string | number): JsonRpcRequest {
  const req: JsonRpcRequest = { jsonrpc: "2.0", method };
  if (params !== undefined) req.params = params;
  if (id !== undefined) req.id = id;
  return req;
}

async function withSilentConsoleError<T>(fn: () => Promise<T>): Promise<{ result: T; logged: string }> {
  const originalError = console.error;
  const lines: string[] = [];
  console.error = (...args: any[]) => { lines.push(args.map(String).join(" ")); };
  try {
    const result = await fn();
    return { result, logged: lines.join("\n") };
  } finally {
    console.error = originalError;
  }
}

async function testResponseValidation() {
  const validating = makeServer(true);
  const rejecting = await withSilentConsoleError(() =>
    validating.call(request("Svc.wrong", undefined, 1))
  );
  assert.ok(rejecting.result !== null);
  assert.strictEqual(rejecting.result!.error?.code, -32603);
  assert.strictEqual("result" in rejecting.result!, false);
  // R15/D4: no detail leaks, but the failure is logged server-side.
  assert.strictEqual(rejecting.result!.error?.message, "Internal error");
  assert.strictEqual((rejecting.result!.error as any).data, undefined);
  assert.match(rejecting.logged, /Response validation failed/);

  const permissive = makeServer(false);
  const passing = await permissive.call(request("Svc.wrong", undefined, 1));
  assert.ok(passing !== null);
  assert.strictEqual(passing!.result, 123, "validateResponses:false must return the result unchanged");
  console.log("✓ testResponseValidation");
}

async function testNotificationsProduceNoResponse() {
  const server = makeServer(true);
  // No `id` member means notification, even though params are present.
  assert.strictEqual(await server.call(request("Svc.greet", ["x"])), null);

  // A failing notification is also silent.
  await withSilentConsoleError(async () => {
    assert.strictEqual(await server.call(request("Svc.boom")), null);
  });

  // M21: id:null is also a notification, and the handler still runs.
  assert.strictEqual(
    await server.call({ jsonrpc: "2.0", method: "Svc.greet", params: ["x"], id: null }),
    null
  );

  // id: 0 is a valid id and must be echoed, not treated as absent.
  const echoed = await server.call(request("Svc.greet", ["x"], 0));
  assert.ok(echoed !== null);
  assert.strictEqual(echoed!.id, 0);
  assert.strictEqual(echoed!.result, "hi x");
  console.log("✓ testNotificationsProduceNoResponse");
}

async function testRequestValidationAndErrorData() {
  const server = makeServer(true);
  const bad = await server.call(request("Svc.greet", [123], 7));
  assert.ok(bad !== null);
  assert.strictEqual(bad!.error?.code, -32602);
  assert.strictEqual(bad!.id, 7);
  // -32602 validation errors are user-facing and keep their data.
  assert.strictEqual((bad!.error as any).data.valid, false);
  assert.deepStrictEqual((bad!.error as any).data.invalidFields, ["param[0]"]);

  const unknown = await server.call(request("Svc.nope", [], 8));
  assert.strictEqual(unknown!.error?.code, -32601);
  console.log("✓ testRequestValidationAndErrorData");
}

async function testTrailingOptionalParams() {
  const server = makeServer(true);
  const positional = await server.call(request("Svc.opt", [5], 1));
  assert.strictEqual(positional!.result, 5);

  const named = await server.call(request("Svc.opt", { a: 5 }, 2));
  assert.strictEqual(named!.result, 5);

  const both = await server.call(request("Svc.opt", [5, "9"], 3));
  assert.strictEqual(both!.result, 9);

  const missingRequired = await server.call(request("Svc.opt", [], 4));
  assert.strictEqual(missingRequired!.error?.code, -32602);
  const extra = await server.call(request("Svc.opt", [5, "9", 10], 5));
  assert.strictEqual(extra!.error?.code, -32602);
  console.log("✓ testTrailingOptionalParams");
}

async function testObjectFormParams() {
  const server = makeServer(true);

  // M20: object-form params map by IDL name, not object key order.
  const reordered = await server.call(request("Svc.pair", { b: "two", a: 1 }, 1));
  assert.strictEqual(reordered!.result, "1:two");

  // Unknown keys are rejected explicitly instead of being silently dropped.
  const unknownKey = await server.call(request("Svc.pair", { a: 1, b: "two", extra: 3 }, 2));
  assert.strictEqual(unknownKey!.error?.code, -32602);
  assert.match(String((unknownKey!.error as any).data), /Unknown parameter 'extra'/);

  // Primitive params are rejected explicitly.
  const primitive = await server.call(request("Svc.pair", 5 as any, 3));
  assert.strictEqual(primitive!.error?.code, -32602);
  assert.match(String((primitive!.error as any).data), /object or array/);
  console.log("✓ testObjectFormParams");
}

async function testHandlerErrorRedaction() {
  const server = makeServer(true);
  const { result, logged } = await withSilentConsoleError(() =>
    server.call(request("Svc.boom", undefined, 11))
  );
  assert.ok(result !== null);
  assert.strictEqual(result!.error?.code, -32603);
  assert.strictEqual(result!.id, 11);
  // R15/D4: fixed message, no data, no exception text on the wire.
  assert.strictEqual(result!.error?.message, "Internal error");
  assert.strictEqual((result!.error as any).data, undefined);
  assert.strictEqual(JSON.stringify(result).includes("secret-token"), false);
  // The exception detail is still available server-side.
  assert.match(logged, /secret-token-123/);
  console.log("✓ testHandlerErrorRedaction");
}

async function testDeclaredRpcErrorIsPreserved() {
  const server = makeServer(true);
  const response = await server.call(request("Svc.declared", undefined, 12));
  assert.strictEqual(response!.error?.code, 1002);
  assert.strictEqual(response!.error?.message, "NotFound");
  assert.deepStrictEqual((response!.error as any).data, { detail: "missing" });
  console.log("✓ testDeclaredRpcErrorIsPreserved");
}

async function testIdTypes() {
  const server = makeServer(true);
  // M24/D2: string and numeric ids are echoed by value.
  const stringId = await server.call(request("Svc.greet", ["a"], "abc"));
  assert.strictEqual(stringId!.id, "abc");
  const numberId = await server.call(request("Svc.greet", ["a"], 42));
  assert.strictEqual(numberId!.id, 42);

  // R24: boolean/object/array ids are rejected with -32600 and id:null.
  for (const badId of [true, { x: 1 }, [1]] as any[]) {
    const response = await server.call({
      jsonrpc: "2.0",
      method: "Svc.greet",
      params: ["a"],
      id: badId,
    } as any);
    assert.ok(response !== null, "an invalid id must still receive an error response");
    assert.strictEqual(response!.error?.code, -32600, `id ${JSON.stringify(badId)}`);
    assert.strictEqual(response!.id, null);
  }
  console.log("✓ testIdTypes");
}

async function testJsonRpcVersionEnforced() {
  // M27: requests must declare jsonrpc "2.0"; responses always do.
  const server = makeServer(false);
  const bad = await server.call({
    jsonrpc: "1.0",
    method: "Svc.greet",
    params: ["a"],
    id: 9,
  } as any);
  assert.ok(bad !== null);
  assert.strictEqual(bad!.jsonrpc, "2.0");
  assert.strictEqual(bad!.error?.code, -32600);
  console.log("✓ testJsonRpcVersionEnforced");
}

async function testNonObjectRequestEntry() {
  // M19: the generated batch adapter delegates every entry (including
  // primitives) to Server.call, which must return a per-entry -32600.
  const server = makeServer(true);
  for (const entry of [5, "x", null, []] as any[]) {
    const response = await server.call(entry);
    assert.ok(response !== null, `entry ${JSON.stringify(entry)} must produce a response`);
    assert.strictEqual(response!.error?.code, -32600, `entry ${JSON.stringify(entry)}`);
    assert.strictEqual(response!.id, null);
  }
  console.log("✓ testNonObjectRequestEntry");
}

async function testNullResultIsExplicit() {
  const server = makeServer(true);
  const response = await server.call(request("Svc.nullable", undefined, 3));
  assert.ok(response !== null);
  assert.strictEqual(response!.result, null);
  assert.ok(
    JSON.stringify(response).includes('"result":null'),
    "M23: a null result must be serialized explicitly"
  );
  console.log("✓ testNullResultIsExplicit");
}

async function testParamCountRuleWithValidationDisabled() {
  // Even with request validation off, positional params are mapped against
  // the IDL signature via Contract.checkParamCount (single owner of the rule),
  // so a bad count still fails fast instead of being silently mis-mapped.
  const server = new Server({
    contract: new Contract(idl()),
    validateRequests: false,
    validateResponses: false,
  });
  server.addHandler("Svc", {
    opt(_ctx: any, a: number, b?: string) { return b === undefined ? a : Number(b); },
  });

  const tooFew = await server.call(request("Svc.opt", [], 1));
  assert.strictEqual(tooFew!.error?.code, -32602);
  assert.ok((tooFew!.error as any).data.includes("expects 1-2 param(s)"));

  const tooMany = await server.call(request("Svc.opt", [5, "9", 10], 2));
  assert.strictEqual(tooMany!.error?.code, -32602);
  assert.ok((tooMany!.error as any).data.includes("expects 1-2 param(s)"));

  const ok = await server.call(request("Svc.opt", [5], 3));
  assert.strictEqual(ok!.result, 5, "valid trailing-optional form must still work");
  console.log("✓ testParamCountRuleWithValidationDisabled");
}

(async () => {
  await testResponseValidation();
  await testNotificationsProduceNoResponse();
  await testRequestValidationAndErrorData();
  await testTrailingOptionalParams();
  await testObjectFormParams();
  await testHandlerErrorRedaction();
  await testDeclaredRpcErrorIsPreserved();
  await testIdTypes();
  await testJsonRpcVersionEnforced();
  await testNonObjectRequestEntry();
  await testNullResultIsExplicit();
  await testParamCountRuleWithValidationDisabled();
  console.log("\nAll server tests passed!");
})().catch((e) => {
  console.error(e);
  process.exit(1);
});

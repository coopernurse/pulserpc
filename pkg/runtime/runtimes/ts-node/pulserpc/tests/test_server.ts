/**
 * Unit tests for the transport-independent Server: response validation,
 * notification silence, id echo and trailing-optional parameters
 * (ts-node-01, ts-node-02, ts-node-03, ts-node-06).
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
            name: "wrong",
            parameters: [],
            returnType: { builtIn: "string" },
          },
          { name: "boom", parameters: [], returnType: { builtIn: "string" } },
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
    wrong(_ctx: any) { return 123; }, // wrong type for a string return
    boom(_ctx: any) { throw new Error("boom"); },
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

async function testResponseValidation() {
  const validating = makeServer(true);
  const rejecting = await validating.call(request("Svc.wrong", undefined, 1));
  assert.ok(rejecting !== null);
  assert.strictEqual(rejecting!.error?.code, -32603, "wrong-typed result must become an error response");
  assert.strictEqual("result" in rejecting!, false);

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
  const originalError = console.error;
  console.error = () => {};
  try {
    assert.strictEqual(await server.call(request("Svc.boom")), null);
  } finally {
    console.error = originalError;
  }

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

async function testHandlerErrorMapping() {
  const server = makeServer(true);
  const originalError = console.error;
  console.error = () => {};
  let response: JsonRpcResponse | null;
  try {
    response = await server.call(request("Svc.boom", undefined, 11));
  } finally {
    console.error = originalError;
  }
  assert.strictEqual(response!.error?.code, -32603);
  assert.strictEqual(response!.id, 11);
  assert.ok(new RPCError(-32603, "x") instanceof RPCError);
  console.log("✓ testHandlerErrorMapping");
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
  await testHandlerErrorMapping();
  await testParamCountRuleWithValidationDisabled();
  console.log("\nAll server tests passed!");
})().catch((e) => {
  console.error(e);
  process.exit(1);
});

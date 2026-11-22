/**
 * Tests for Server class (CommonJS variant)
 *
 * Covers response validation (R10), notification handling (R11/M4/M21),
 * internal-error redaction (R15/D4), id typing (R24), parameter handling
 * (M20) and the error/result envelope shapes.
 */

const { strict: assert } = require("assert");
const { Server } = require("../server");
const { Contract } = require("../contract");
const { RPCError } = require("../rpc");

function makeContract() {
  return new Contract({
    interfaces: [
      {
        name: "Svc",
        methods: [
          { name: "good", parameters: [], returnType: { builtIn: "int" } },
          { name: "bad", parameters: [], returnType: { builtIn: "int" } },
          { name: "boom", parameters: [], returnType: { builtIn: "int" } },
          { name: "declared", parameters: [], returnType: { builtIn: "int" } },
          { name: "nullable", parameters: [], returnType: { builtIn: "int" }, returnOptional: true },
          {
            name: "pair",
            parameters: [
              { name: "a", type: { builtIn: "int" } },
              { name: "b", type: { builtIn: "string" } },
            ],
            returnType: { builtIn: "string" },
          },
          {
            name: "greet",
            parameters: [
              { name: "name", type: { builtIn: "string" } },
              { name: "times", type: { builtIn: "int" }, optional: true },
            ],
            returnType: { builtIn: "string" },
          },
        ],
      },
    ],
    structs: [],
    enums: [],
    errors: [],
  });
}

function handlers(overrides = {}) {
  return {
    good: async () => 42,
    bad: async () => "not-an-int",
    boom: async () => {
      throw new Error("secret-token-123");
    },
    declared: async () => {
      throw new RPCError(1002, "NotFound", { detail: "missing" });
    },
    nullable: async () => null,
    pair: async (_ctx, a, b) => `${a}:${b}`,
    greet: async (_ctx, name, times) => `${name}:${times}`,
    ...overrides,
  };
}

async function withSilentConsoleError(fn) {
  const originalError = console.error;
  const lines = [];
  console.error = (...args) => { lines.push(args.map(String).join(" ")); };
  try {
    const result = await fn();
    return { result, logged: lines.join("\n") };
  } finally {
    console.error = originalError;
  }
}

async function testValidateResponsesEnabledRejectsWrongType() {
  const server = new Server({ contract: makeContract(), validateResponses: true });
  server.addHandler("Svc", handlers());

  const ok = await server.call({ jsonrpc: "2.0", method: "Svc.good", params: [], id: 1 });
  assert.deepStrictEqual(ok, { jsonrpc: "2.0", result: 42, id: 1 });

  const { result: resp, logged } = await withSilentConsoleError(() =>
    server.call({ jsonrpc: "2.0", method: "Svc.bad", params: [], id: 2 })
  );
  assert.ok(resp.error, `expected an error response, got ${JSON.stringify(resp)}`);
  assert.strictEqual(resp.error.code, -32603);
  assert.strictEqual(resp.error.message, "Internal error");
  assert.strictEqual(resp.error.data, undefined);
  assert.strictEqual(resp.result, undefined);
  assert.strictEqual(resp.id, 2);
  assert.match(logged, /Response validation failed/);
  console.log("✓ testValidateResponsesEnabledRejectsWrongType");
}

async function testValidateResponsesDisabledReturnsWrongType() {
  const server = new Server({ contract: makeContract(), validateResponses: false });
  server.addHandler("Svc", handlers());

  const resp = await server.call({ jsonrpc: "2.0", method: "Svc.bad", params: [], id: 3 });
  assert.strictEqual(resp.error, undefined);
  assert.strictEqual(resp.result, "not-an-int");
  console.log("✓ testValidateResponsesDisabledReturnsWrongType");
}

async function testNotificationReturnsNull() {
  const server = new Server({ contract: makeContract(), validateRequests: false });
  let called = 0;
  server.addHandler("Svc", handlers({
    good: async () => {
      called++;
      return 42;
    },
  }));

  const resp = await server.call({ jsonrpc: "2.0", method: "Svc.good", params: [] });
  assert.strictEqual(resp, null, "notifications must produce no response");
  assert.strictEqual(called, 1, "notification handler must still run");
  console.log("✓ testNotificationReturnsNull");
}

async function testIdNullIsNotification() {
  // M21: id:null is treated as a notification, and the handler still runs.
  const server = new Server({ contract: makeContract(), validateRequests: false });
  let called = 0;
  server.addHandler("Svc", handlers({
    good: async () => {
      called++;
      return 42;
    },
  }));

  const resp = await server.call({ jsonrpc: "2.0", method: "Svc.good", params: [], id: null });
  assert.strictEqual(resp, null);
  assert.strictEqual(called, 1);
  console.log("✓ testIdNullIsNotification");
}

async function testThrowingNotificationReturnsNull() {
  const server = new Server({ contract: makeContract(), validateRequests: false });
  server.addHandler("Svc", handlers());

  const resp = await withSilentConsoleError(() =>
    server.call({ jsonrpc: "2.0", method: "Svc.boom", params: [] })
  );
  assert.strictEqual(resp.result, null, "a failed notification must not emit an id:null error");
  console.log("✓ testThrowingNotificationReturnsNull");
}

async function testNamedParamsWithOmittedOptional() {
  const server = new Server({ contract: makeContract(), validateRequests: true });
  server.addHandler("Svc", handlers());

  const resp = await server.call({ jsonrpc: "2.0", method: "Svc.greet", params: { name: "Ann" }, id: 7 });
  assert.strictEqual(resp.error, undefined, JSON.stringify(resp));
  assert.strictEqual(resp.result, "Ann:undefined");
  console.log("✓ testNamedParamsWithOmittedOptional");
}

async function testObjectFormParams() {
  const server = new Server({ contract: makeContract(), validateRequests: true });
  server.addHandler("Svc", handlers());

  // M20: map by IDL name, not object key order.
  const reordered = await server.call({ jsonrpc: "2.0", method: "Svc.pair", params: { b: "two", a: 1 }, id: 1 });
  assert.strictEqual(reordered.result, "1:two");

  const unknown = await server.call({ jsonrpc: "2.0", method: "Svc.pair", params: { a: 1, b: "two", extra: 3 }, id: 2 });
  assert.strictEqual(unknown.error.code, -32602);
  assert.match(String(unknown.error.data), /Unknown parameter 'extra'/);

  const primitive = await server.call({ jsonrpc: "2.0", method: "Svc.pair", params: 5, id: 3 });
  assert.strictEqual(primitive.error.code, -32602);
  assert.match(String(primitive.error.data), /object or array/);
  console.log("✓ testObjectFormParams");
}

async function testHandlerErrorRedaction() {
  const server = new Server({ contract: makeContract(), validateRequests: false });
  server.addHandler("Svc", handlers());

  const { result: resp, logged } = await withSilentConsoleError(() =>
    server.call({ jsonrpc: "2.0", method: "Svc.boom", params: [], id: 11 })
  );
  assert.strictEqual(resp.error.code, -32603);
  assert.strictEqual(resp.error.message, "Internal error");
  assert.strictEqual(resp.error.data, undefined);
  assert.strictEqual(JSON.stringify(resp).includes("secret-token"), false);
  assert.match(logged, /secret-token-123/);
  console.log("✓ testHandlerErrorRedaction");
}

async function testDeclaredRpcErrorIsPreserved() {
  const server = new Server({ contract: makeContract(), validateRequests: false });
  server.addHandler("Svc", handlers());
  const resp = await server.call({ jsonrpc: "2.0", method: "Svc.declared", params: [], id: 12 });
  assert.strictEqual(resp.error.code, 1002);
  assert.strictEqual(resp.error.message, "NotFound");
  assert.deepStrictEqual(resp.error.data, { detail: "missing" });
  console.log("✓ testDeclaredRpcErrorIsPreserved");
}

async function testIdTypes() {
  const server = new Server({ contract: makeContract(), validateRequests: false });
  server.addHandler("Svc", handlers());

  const stringId = await server.call({ jsonrpc: "2.0", method: "Svc.good", params: [], id: "abc" });
  assert.strictEqual(stringId.id, "abc");
  const numberId = await server.call({ jsonrpc: "2.0", method: "Svc.good", params: [], id: 42 });
  assert.strictEqual(numberId.id, 42);

  for (const badId of [true, { x: 1 }, [1]]) {
    const resp = await server.call({ jsonrpc: "2.0", method: "Svc.good", params: [], id: badId });
    assert.ok(resp, "an invalid id must still receive an error response");
    assert.strictEqual(resp.error.code, -32600, `id ${JSON.stringify(badId)}`);
    assert.strictEqual(resp.id, null);
  }
  console.log("✓ testIdTypes");
}

async function testJsonRpcVersionEnforced() {
  // M27: requests must declare jsonrpc "2.0"; responses always do.
  const server = new Server({ contract: makeContract(), validateRequests: false });
  const resp = await server.call({ jsonrpc: "1.0", method: "Svc.good", params: [], id: 9 });
  assert.strictEqual(resp.jsonrpc, "2.0");
  assert.strictEqual(resp.error.code, -32600);
  console.log("✓ testJsonRpcVersionEnforced");
}

async function testNonObjectRequestEntry() {
  const server = new Server({ contract: makeContract(), validateRequests: false });
  for (const entry of [5, "x", null, []]) {
    const resp = await server.call(entry);
    assert.ok(resp, `entry ${JSON.stringify(entry)} must produce a response`);
    assert.strictEqual(resp.error.code, -32600, `entry ${JSON.stringify(entry)}`);
    assert.strictEqual(resp.id, null);
  }
  console.log("✓ testNonObjectRequestEntry");
}

async function testNullResultIsExplicit() {
  const server = new Server({ contract: makeContract(), validateRequests: false });
  server.addHandler("Svc", handlers());
  const resp = await server.call({ jsonrpc: "2.0", method: "Svc.nullable", params: [], id: 3 });
  assert.strictEqual(resp.result, null);
  assert.ok(JSON.stringify(resp).includes('"result":null'), "M23: null result must be explicit");
  console.log("✓ testNullResultIsExplicit");
}

async function testIdZeroIsEchoed() {
  const server = new Server({ contract: makeContract(), validateRequests: false });
  server.addHandler("Svc", handlers());

  const resp = await server.call({ jsonrpc: "2.0", method: "Svc.good", params: [], id: 0 });
  assert.strictEqual(resp.id, 0);
  assert.strictEqual(resp.result, 42);
  console.log("✓ testIdZeroIsEchoed");
}

async function main() {
  await testValidateResponsesEnabledRejectsWrongType();
  await testValidateResponsesDisabledReturnsWrongType();
  await testNotificationReturnsNull();
  await testIdNullIsNotification();
  await testThrowingNotificationReturnsNull();
  await testNamedParamsWithOmittedOptional();
  await testObjectFormParams();
  await testHandlerErrorRedaction();
  await testDeclaredRpcErrorIsPreserved();
  await testIdTypes();
  await testJsonRpcVersionEnforced();
  await testNonObjectRequestEntry();
  await testNullResultIsExplicit();
  await testIdZeroIsEchoed();
  console.log("\nAll server tests passed!");
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});

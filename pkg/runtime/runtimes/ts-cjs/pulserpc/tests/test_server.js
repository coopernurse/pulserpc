/**
 * Tests for Server class (CommonJS variant)
 *
 * Covers response validation (R10) and notification handling (R11/M4).
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

async function testValidateResponsesEnabledRejectsWrongType() {
  const server = new Server({ contract: makeContract(), validateResponses: true });
  server.addHandler("Svc", {
    good: async () => 42,
    bad: async () => "not-an-int",
    boom: async () => 0,
  });

  const ok = await server.call({ jsonrpc: "2.0", method: "Svc.good", params: [], id: 1 });
  assert.deepStrictEqual(ok, { jsonrpc: "2.0", result: 42, id: 1 });

  const resp = await server.call({ jsonrpc: "2.0", method: "Svc.bad", params: [], id: 2 });
  assert.ok(resp.error, `expected an error response, got ${JSON.stringify(resp)}`);
  assert.strictEqual(resp.error.code, -32603);
  assert.strictEqual(resp.result, undefined);
  assert.strictEqual(resp.id, 2);
  console.log("✓ testValidateResponsesEnabledRejectsWrongType");
}

async function testValidateResponsesDisabledReturnsWrongType() {
  const server = new Server({ contract: makeContract(), validateResponses: false });
  server.addHandler("Svc", {
    good: async () => 42,
    bad: async () => "not-an-int",
    boom: async () => 0,
  });

  const resp = await server.call({ jsonrpc: "2.0", method: "Svc.bad", params: [], id: 3 });
  assert.strictEqual(resp.error, undefined);
  assert.strictEqual(resp.result, "not-an-int");
  console.log("✓ testValidateResponsesDisabledReturnsWrongType");
}

async function testNotificationReturnsNull() {
  const server = new Server({ contract: makeContract(), validateRequests: false });
  let called = 0;
  server.addHandler("Svc", {
    good: async () => {
      called++;
      return 42;
    },
    bad: async () => "wrong",
    boom: async () => 0,
  });

  const resp = await server.call({ jsonrpc: "2.0", method: "Svc.good", params: [] });
  assert.strictEqual(resp, null, "notifications must produce no response");
  assert.strictEqual(called, 1, "notification handler must still run");
  console.log("✓ testNotificationReturnsNull");
}

async function testThrowingNotificationReturnsNull() {
  const server = new Server({ contract: makeContract(), validateRequests: false });
  server.addHandler("Svc", {
    good: async () => 42,
    bad: async () => "wrong",
    boom: async () => {
      throw new RPCError(1234, "kaboom");
    },
  });

  const resp = await server.call({ jsonrpc: "2.0", method: "Svc.boom", params: [] });
  assert.strictEqual(resp, null, "a failed notification must not emit an id:null error");
  console.log("✓ testThrowingNotificationReturnsNull");
}

async function testNamedParamsWithOmittedOptional() {
  const server = new Server({ contract: makeContract(), validateRequests: true });
  server.addHandler("Svc", {
    good: async () => 42,
    bad: async () => "not-an-int",
    boom: async () => 0,
    greet: async (_ctx, name, times) => `${name}:${times}`,
  });

  const resp = await server.call({ jsonrpc: "2.0", method: "Svc.greet", params: { name: "Ann" }, id: 7 });
  assert.strictEqual(resp.error, undefined, JSON.stringify(resp));
  assert.strictEqual(resp.result, "Ann:undefined");
  console.log("✓ testNamedParamsWithOmittedOptional");
}

async function testIdZeroIsEchoed() {
  const server = new Server({ contract: makeContract(), validateRequests: false });
  server.addHandler("Svc", {
    good: async () => 42,
    bad: async () => "wrong",
    boom: async () => 0,
  });

  const resp = await server.call({ jsonrpc: "2.0", method: "Svc.good", params: [], id: 0 });
  assert.strictEqual(resp.id, 0);
  assert.strictEqual(resp.result, 42);
  console.log("✓ testIdZeroIsEchoed");
}

async function main() {
  await testValidateResponsesEnabledRejectsWrongType();
  await testValidateResponsesDisabledReturnsWrongType();
  await testNotificationReturnsNull();
  await testThrowingNotificationReturnsNull();
  await testNamedParamsWithOmittedOptional();
  await testIdZeroIsEchoed();
  console.log("\nAll server tests passed!");
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});

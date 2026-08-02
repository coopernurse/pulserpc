import { strict as assert } from "assert";
import { Server } from "../server.js";
import { Contract } from "../contract.js";
import { RPCError } from "../rpc.js";

function makeEmptyContract(): Contract {
  return new Contract({
    interfaces: [],
    structs: [],
    enums: [],
    errors: [],
  });
}

function makeMathContract(): Contract {
  return new Contract({
    interfaces: [
      {
        name: "Math",
        methods: [
          {
            name: "add",
            parameters: [
              { name: "a", type: { builtIn: "int" } },
              { name: "b", type: { builtIn: "int" } },
            ],
            returnType: { builtIn: "int" },
          },
          {
            name: "echo",
            parameters: [
              { name: "msg", type: { builtIn: "string" } },
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

async function testServerConstruction() {
  const contract = makeEmptyContract();
  const server = new Server({ contract });
  assert.ok(server instanceof Server);
  assert.ok(server.handlers instanceof Map);
  assert.strictEqual(server.validateRequests, true);
  assert.strictEqual(server.validateResponses, true);
  console.log("✓ testServerConstruction");
}

async function testServerConstructionDefaultOptions() {
  const contract = makeEmptyContract();
  const server = new Server({ contract, validateRequests: false, validateResponses: false });
  assert.strictEqual(server.validateRequests, false);
  assert.strictEqual(server.validateResponses, false);
  console.log("✓ testServerConstructionDefaultOptions");
}

async function testServerAddHandler() {
  const contract = makeEmptyContract();
  const server = new Server({ contract });
  server.addHandler("Test", {
    ping: async (_ctx: any) => "pong",
  });
  assert.ok(server.handlers.has("Test"));
  const handler = server.handlers.get("Test")!;
  assert.strictEqual(typeof handler.ping, "function");
  console.log("✓ testServerAddHandler");
}

async function testServerPulseIdlIntrospection() {
  const contract = makeMathContract();
  const server = new Server({ contract });
  const req = { jsonrpc: "2.0", method: "pulserpc-idl", id: "1" };
  const resp = await server.call(req, {});
  assert.ok(resp);
  assert.strictEqual(resp!.id, "1");
  assert.ok(resp!.result);
  assert.ok(resp!.result.interfaces);
  assert.strictEqual(resp!.result.interfaces[0].name, "Math");
  console.log("✓ testServerPulseIdlIntrospection");
}

async function testServerHandlerInvocationWithNamedParams() {
  const contract = makeMathContract();
  const server = new Server({ contract, validateRequests: false });
  server.addHandler("Math", {
    add: async (_ctx: any, a: number, b: number) => a + b,
  });

  const req = {
    jsonrpc: "2.0",
    method: "Math.add",
    params: { a: 2, b: 3 },
    id: "1",
  };
  const resp = await server.call(req, {});
  assert.ok(resp);
  assert.strictEqual(resp!.result, 5);
  console.log("✓ testServerHandlerInvocationWithNamedParams");
}

async function testServerHandlerInvocationWithPositionalParams() {
  const contract = makeMathContract();
  const server = new Server({ contract, validateRequests: false });
  server.addHandler("Math", {
    add: async (_ctx: any, a: number, b: number) => a + b,
  });

  const req = {
    jsonrpc: "2.0",
    method: "Math.add",
    params: [2, 3],
    id: "1",
  };
  const resp = await server.call(req, {});
  assert.ok(resp);
  assert.strictEqual(resp!.result, 5);
  console.log("✓ testServerHandlerInvocationWithPositionalParams");
}

async function testServerHandlerInvocationNoParams() {
  const contract = new Contract({
    interfaces: [
      {
        name: "Clock",
        methods: [
          {
            name: "now",
            parameters: [],
            returnType: { builtIn: "int" },
          },
        ],
      },
    ],
    structs: [],
    enums: [],
    errors: [],
  });
  const server = new Server({ contract, validateRequests: false });
  server.addHandler("Clock", {
    now: async () => 42,
  });

  const req = { jsonrpc: "2.0", method: "Clock.now", id: "1" };
  const resp = await server.call(req, {});
  assert.ok(resp);
  assert.strictEqual(resp!.result, 42);
  console.log("✓ testServerHandlerInvocationNoParams");
}

async function testServerHandlerCtxForwarding() {
  const contract = makeEmptyContract();
  const server = new Server({ contract, validateRequests: false });
  server.addHandler("Svc", {
    info: async (ctx: any) => ctx.token,
  });

  const req = {
    jsonrpc: "2.0",
    method: "Svc.info",
    id: "1",
  };
  const resp = await server.call(req, { token: "secret" });
  assert.ok(resp);
  assert.strictEqual(resp!.result, "secret");
  console.log("✓ testServerHandlerCtxForwarding");
}

async function testServerRequestValidationNamedParams() {
  const contract = makeMathContract();
  const server = new Server({ contract, validateRequests: true });
  server.addHandler("Math", {
    add: async (_ctx: any, a: number, b: number) => a + b,
  });

  const req = {
    jsonrpc: "2.0",
    method: "Math.add",
    params: { a: "not_a_number", b: 3 },
    id: "1",
  };
  const resp = await server.call(req, {});
  assert.ok(resp);
  assert.ok(resp!.error);
  assert.strictEqual(resp!.error!.code, -32602);
  console.log("✓ testServerRequestValidationNamedParams");
}

async function testServerRequestValidationPositionalParams() {
  const contract = makeMathContract();
  const server = new Server({ contract, validateRequests: true });
  server.addHandler("Math", {
    add: async (_ctx: any, a: number, b: number) => a + b,
  });

  const req = {
    jsonrpc: "2.0",
    method: "Math.add",
    params: ["not_a_number", 3],
    id: "1",
  };
  const resp = await server.call(req, {});
  assert.ok(resp);
  assert.ok(resp!.error);
  assert.strictEqual(resp!.error!.code, -32602);
  console.log("✓ testServerRequestValidationPositionalParams");
}

async function testServerRequestValidationMissingParams() {
  const contract = makeMathContract();
  const server = new Server({ contract, validateRequests: true });
  server.addHandler("Math", {
    add: async (_ctx: any, a: number, b: number) => a + b,
  });

  const req = {
    jsonrpc: "2.0",
    method: "Math.add",
    params: [2],
    id: "1",
  };
  const resp = await server.call(req, {});
  assert.ok(resp);
  assert.ok(resp!.error);
  assert.strictEqual(resp!.error!.code, -32602);
  assert.ok(resp!.error!.message.toLowerCase().includes("expects 2"));
  console.log("✓ testServerRequestValidationMissingParams");
}

async function testServerRequestValidationDisabled() {
  const contract = makeMathContract();
  const server = new Server({ contract, validateRequests: false });
  server.addHandler("Math", {
    add: async (_ctx: any, a: number, b: number) => a + b,
  });

  const req = {
    jsonrpc: "2.0",
    method: "Math.add",
    params: { a: "bad", b: "also_bad" },
    id: "1",
  };
  const resp = await server.call(req, {});
  assert.ok(resp);
  assert.strictEqual(resp!.result, "badalso_bad");
  console.log("✓ testServerRequestValidationDisabled");
}

async function testServerOptionalParamOmitted() {
  const contract = makeMathContract();
  const server = new Server({ contract, validateRequests: false });
  server.addHandler("Math", {
    echo: async (_ctx: any, msg: string, times?: number) => {
      const n = times ?? 1;
      return Array(n).fill(msg).join("");
    },
  });

  const req = {
    jsonrpc: "2.0",
    method: "Math.echo",
    params: ["hello"],
    id: "1",
  };
  const resp = await server.call(req, {});
  assert.ok(resp);
  assert.strictEqual(resp!.result, "hello");
  console.log("✓ testServerOptionalParamOmitted");
}

async function testServerErrorHandlingRpcError() {
  const contract = makeEmptyContract();
  const server = new Server({ contract, validateRequests: false });
  server.addHandler("Svc", {
    fail: async () => {
      throw new RPCError(42, "custom error", { detail: "xyz" });
    },
  });

  const req = { jsonrpc: "2.0", method: "Svc.fail", id: "1" };
  const resp = await server.call(req, {});
  assert.ok(resp);
  assert.ok(resp!.error);
  assert.strictEqual(resp!.error!.code, 42);
  assert.ok(resp!.error!.message.includes("custom error"));
  assert.deepStrictEqual(resp!.error!.data, { detail: "xyz" });
  console.log("✓ testServerErrorHandlingRpcError");
}

async function testServerErrorHandlingUnexpectedError() {
  const contract = makeEmptyContract();
  const server = new Server({ contract, validateRequests: false });
  server.addHandler("Svc", {
    fail: async () => {
      throw new Error("something broke");
    },
  });

  const req = { jsonrpc: "2.0", method: "Svc.fail", id: "1" };
  const resp = await server.call(req, {});
  assert.ok(resp);
  assert.ok(resp!.error);
  assert.strictEqual(resp!.error!.code, -32603);
  assert.ok(resp!.error!.message.includes("Internal error"));
  console.log("✓ testServerErrorHandlingUnexpectedError");
}

async function testServerNotificationNoResponse() {
  const contract = makeEmptyContract();
  const server = new Server({ contract, validateRequests: false });
  let notified = false;
  server.addHandler("Svc", {
    notify: async () => {
      notified = true;
    },
  });

  const req = { jsonrpc: "2.0", method: "Svc.notify" };
  const resp = await server.call(req, {});
  assert.strictEqual(resp, null);
  assert.strictEqual(notified, true);
  console.log("✓ testServerNotificationNoResponse");
}

async function testServerNotificationHandlerErrorIsSwallowed() {
  const contract = makeEmptyContract();
  const server = new Server({ contract, validateRequests: false });
  server.addHandler("Svc", {
    fail: async () => {
      throw new Error("notification error");
    },
  });

  const req = { jsonrpc: "2.0", method: "Svc.fail" };
  const resp = await server.call(req, {});
  assert.strictEqual(resp, null);
  console.log("✓ testServerNotificationHandlerErrorIsSwallowed");
}

async function testServerInvalidMethodFormat() {
  const contract = makeEmptyContract();
  const server = new Server({ contract });
  const req = { jsonrpc: "2.0", method: "badFormat", id: "1" };
  const resp = await server.call(req, {});
  assert.ok(resp);
  assert.ok(resp!.error);
  assert.strictEqual(resp!.error!.code, -32601);
  console.log("✓ testServerInvalidMethodFormat");
}

async function testServerUnknownInterface() {
  const contract = makeEmptyContract();
  const server = new Server({ contract });
  const req = { jsonrpc: "2.0", method: "Unknown.foo", id: "1" };
  const resp = await server.call(req, {});
  assert.ok(resp);
  assert.ok(resp!.error);
  assert.strictEqual(resp!.error!.code, -32601);
  console.log("✓ testServerUnknownInterface");
}

async function testServerUnknownMethod() {
  const contract = makeMathContract();
  const server = new Server({ contract });
  server.addHandler("Math", {
    add: async (_ctx: any, a: number, b: number) => a + b,
  });

  const req = { jsonrpc: "2.0", method: "Math.unknown", id: "1" };
  const resp = await server.call(req, {});
  assert.ok(resp);
  assert.ok(resp!.error);
  assert.strictEqual(resp!.error!.code, -32601);
  console.log("✓ testServerUnknownMethod");
}

async function testServerInvalidJsonrpcVersion() {
  const contract = makeEmptyContract();
  const server = new Server({ contract });
  const req = { jsonrpc: "1.0", method: "Svc.foo", id: "1" };
  const resp = await server.call(req, {});
  assert.ok(resp);
  assert.ok(resp!.error);
  assert.strictEqual(resp!.error!.code, -32600);
  console.log("✓ testServerInvalidJsonrpcVersion");
}

async function testServerInvalidRequestNotObject() {
  const contract = makeEmptyContract();
  const server = new Server({ contract });
  const resp = await server.call("not_an_object", {});
  assert.ok(resp);
  assert.ok(resp!.error);
  assert.strictEqual(resp!.error!.code, -32600);
  console.log("✓ testServerInvalidRequestNotObject");
}

async function testServerInvalidRequestNull() {
  const contract = makeEmptyContract();
  const server = new Server({ contract });
  const resp = await server.call(null, {});
  assert.ok(resp);
  assert.ok(resp!.error);
  assert.strictEqual(resp!.error!.code, -32600);
  console.log("✓ testServerInvalidRequestNull");
}

async function testServerInvalidRequestNoMethod() {
  const contract = makeEmptyContract();
  const server = new Server({ contract });
  const req = { jsonrpc: "2.0", id: "1" };
  const resp = await server.call(req, {});
  assert.ok(resp);
  assert.ok(resp!.error);
  assert.strictEqual(resp!.error!.code, -32600);
  console.log("✓ testServerInvalidRequestNoMethod");
}

async function testServerInvalidRequestMethodNotString() {
  const contract = makeEmptyContract();
  const server = new Server({ contract });
  const req = { jsonrpc: "2.0", method: 123, id: "1" };
  const resp = await server.call(req, {});
  assert.ok(resp);
  assert.ok(resp!.error);
  assert.strictEqual(resp!.error!.code, -32600);
  console.log("✓ testServerInvalidRequestMethodNotString");
}

async function main() {
  await testServerConstruction();
  await testServerConstructionDefaultOptions();
  await testServerAddHandler();
  await testServerPulseIdlIntrospection();
  await testServerHandlerInvocationWithNamedParams();
  await testServerHandlerInvocationWithPositionalParams();
  await testServerHandlerInvocationNoParams();
  await testServerHandlerCtxForwarding();
  await testServerRequestValidationNamedParams();
  await testServerRequestValidationPositionalParams();
  await testServerRequestValidationMissingParams();
  await testServerRequestValidationDisabled();
  await testServerOptionalParamOmitted();
  await testServerErrorHandlingRpcError();
  await testServerErrorHandlingUnexpectedError();
  await testServerNotificationNoResponse();
  await testServerNotificationHandlerErrorIsSwallowed();
  await testServerInvalidMethodFormat();
  await testServerUnknownInterface();
  await testServerUnknownMethod();
  await testServerInvalidJsonrpcVersion();
  await testServerInvalidRequestNotObject();
  await testServerInvalidRequestNull();
  await testServerInvalidRequestNoMethod();
  await testServerInvalidRequestMethodNotString();
  console.log("\nAll server tests passed!");
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});

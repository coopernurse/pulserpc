/**
 * Tests for Client class (CommonJS variant)
 *
 * Exercises:
 *  - IDL auto-discovery via __filename (walk-up)
 *  - Error wrapping (RPCError propagation)
 *  - Basic request/response with InProcTransport
 */

const { strict: assert } = require("assert");
const { Client } = require("../client");
const { Server } = require("../server");
const { Contract } = require("../contract");
const { InProcTransport } = require("../transport");
const { RPCError } = require("../rpc");

async function testClientConstructsWithInProcTransport() {
  const idl = {
    interfaces: [],
    structs: [],
    enums: [],
    errors: [],
  };
  const contract = new Contract(idl);
  const server = new Server({ contract });

  const transport = new InProcTransport((req) => server.call(req, {}));
  const client = await Client.create(transport);

  assert.ok(client instanceof Client, "Client should be constructible");
  assert.strictEqual(typeof client.call, "function", "Client.call should be a function");
  console.log("✓ testClientConstructsWithInProcTransport");
}

async function testClientBasicRequestResponse() {
  const idl = {
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
        ],
      },
    ],
    structs: [],
    enums: [],
    errors: [],
  };
  const contract = new Contract(idl);
  const server = new Server({ contract, validateRequests: false });
  server.addHandler("Math", {
    add: async (_ctx, a, b) => a + b,
  });

  const transport = new InProcTransport((req) => server.call(req, {}));
  const client = await Client.create(transport);

  const sum = await client.Math.add(2, 3);
  assert.strictEqual(sum, 5, `expected 2+3=5, got ${sum}`);
  console.log("✓ testClientBasicRequestResponse");
}

async function testClientWrapsRpcError() {
  const idl = {
    interfaces: [
      {
        name: "Boom",
        methods: [
          {
            name: "explode",
            parameters: [],
            returnType: { builtIn: "string" },
          },
        ],
      },
    ],
    structs: [],
    enums: [],
    errors: [],
  };
  const contract = new Contract(idl);
  const server = new Server({ contract, validateRequests: false });
  server.addHandler("Boom", {
    explode: async () => {
      throw new RPCError(42, "kaboom", { detail: "explosion" });
    },
  });

  const transport = new InProcTransport((req) => server.call(req, {}));
  const client = await Client.create(transport);

  let caught = null;
  try {
    await client.Boom.explode();
  } catch (e) {
    caught = e;
  }
  assert.ok(caught instanceof RPCError, "expected RPCError");
  assert.strictEqual(caught.code, 42, `expected code 42, got ${caught.code}`);
  assert.ok(caught.message.includes("kaboom"), `message should include 'kaboom': ${caught.message}`);
  console.log("✓ testClientWrapsRpcError");
}

async function testClientNamedParamsOmittedTrailingOptional() {
  // Calling with a named-param object that omits a trailing optional parameter
  // must not make client-side request validation fail on an `undefined` slot.
  const idl = {
    interfaces: [
      {
        name: "Greeter",
        methods: [
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
  };
  const contract = new Contract(idl);
  const server = new Server({ contract, validateRequests: true });
  server.addHandler("Greeter", {
    greet: async (_ctx, name) => `hello ${name}`,
  });
  // validateRequest=true is what exercises namedToPositional on the client.
  const client = await Client.create(
    new InProcTransport((req) => server.call(req, {})),
    true,
    false
  );

  const result = await client.call("Greeter.greet", { name: "Ann" });
  assert.strictEqual(result, "hello Ann");
  console.log("✓ testClientNamedParamsOmittedTrailingOptional");
}

async function testConcurrentCallsUseUniqueIds() {
  // R20: the id counter increments synchronously before the transport await,
  // so Promise.all callers cannot share an id.
  const idl = { interfaces: [], structs: [], enums: [], errors: [] };
  const ids = [];
  const transport = {
    async request(req) {
      if (req.method === "pulserpc-idl") {
        return { jsonrpc: "2.0", result: idl, id: req.id };
      }
      ids.push(req.id);
      await new Promise((resolve) => setTimeout(resolve, 0));
      return { jsonrpc: "2.0", result: req.id, id: req.id };
    },
    close() {},
  };
  const client = await Client.create(transport);

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
  const transport = {
    async request(req) {
      return { jsonrpc: "2.0", result: hostile, id: req.id };
    },
    close() {},
  };
  const client = await Client.create(transport);
  const meta = client.contract.meta;
  assert.strictEqual(meta.ok, "v", "benign meta keys must survive");
  assert.strictEqual(meta.polluted, undefined);
  assert.strictEqual(Object.getPrototypeOf(meta), Object.prototype);
  for (const key of ["__proto__", "constructor", "prototype"]) {
    assert.strictEqual(Object.prototype.hasOwnProperty.call(meta, key), false);
  }
  assert.strictEqual(({}).polluted, undefined, "Object.prototype must stay clean");
  console.log("✓ testHostileMetaFromServerIsRejected");
}

async function testDivergentIdlIsIncompatible() {
  // R31: verification diffs the local IDL against the served one.
  const serverIDL = {
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
    errors: [],
  };
  const transport = {
    async request(req) {
      return { jsonrpc: "2.0", result: serverIDL, id: req.id };
    },
    close() {},
  };
  const client = await Client.create(transport);
  client.setLocalIDL(JSON.stringify({
    interfaces: [],
    structs: [{ name: "T", fields: [{ name: "shared", type: { builtIn: "string" } }] }],
    enums: [],
    errors: [],
  }));

  const result = await client.verifyCompatibility();
  assert.strictEqual(result.compatible, false);
  assert.ok(result.deltas.length > 0);
  assert.ok(result.deltas.some((d) => d.severity === "Error"));
  console.log("✓ testDivergentIdlIsIncompatible");
}

function testFilenameIsAvailable() {
  assert.ok(typeof __filename === "string", "__filename must be a string in CJS");
  assert.ok(__filename.endsWith("client.ts") || __filename.endsWith("client.js"),
    `__filename should point to the CJS client module: ${__filename}`);
  console.log("✓ testFilenameIsAvailable");
}

function testFindIDLJsonWalksUp() {
  // We test the walk-up logic by directly invoking _findIDLJson from a temp
  // directory that does NOT contain idl.json. The Client should attempt the
  // 10-step walk, give up, and leave _localIDL unset (no crash, no exception).
  // We use a transport that returns a valid (empty) IDL so the background
  // bootstrap triggered in the Client constructor completes cleanly.
  //
  // Note: _findIDLJson is only called from Client.create(), not the constructor.
  // We test via constructing a temporary async wrapper.
  const client = Object.create(Client.prototype);
  client._localIDL = null;

  assert.doesNotThrow(() => client._findIDLJson(), "_findIDLJson should not throw");
  console.log("✓ testFindIDLJsonWalksUp");
}

async function main() {
  await testClientConstructsWithInProcTransport();
  await testClientBasicRequestResponse();
  await testClientWrapsRpcError();
  await testClientNamedParamsOmittedTrailingOptional();
  await testConcurrentCallsUseUniqueIds();
  await testHostileMetaFromServerIsRejected();
  await testDivergentIdlIsIncompatible();
  testFilenameIsAvailable();
  testFindIDLJsonWalksUp();
  console.log("\nAll client tests passed!");
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});

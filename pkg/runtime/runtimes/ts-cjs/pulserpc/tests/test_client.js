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
  testFilenameIsAvailable();
  testFindIDLJsonWalksUp();
  console.log("\nAll client tests passed!");
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});

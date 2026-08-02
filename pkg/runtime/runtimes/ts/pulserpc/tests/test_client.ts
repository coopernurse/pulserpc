import { strict as assert } from "assert";
import { Client } from "../client.js";
import { Server } from "../server.js";
import { Contract } from "../contract.js";
import { InProcTransport } from "../transport.js";
import { RPCError } from "../rpc.js";

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
    add: async (_ctx: any, a: any, b: any) => a + b,
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

  let caught: any = null;
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

async function testClientNotify() {
  const idl = {
    interfaces: [
      {
        name: "Logger",
        methods: [
          {
            name: "log",
            parameters: [
              { name: "msg", type: { builtIn: "string" } },
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
  let logged = "";
  const server = new Server({ contract, validateRequests: false });
  server.addHandler("Logger", {
    log: async (_ctx: any, msg: string) => {
      logged = msg;
      return "ok";
    },
  });

  const transport = new InProcTransport((req) => server.call(req, {}));
  const client = await Client.create(transport);
  const result = await client.notify("Logger.log", { msg: "hello" });
  assert.strictEqual(result, null);
  assert.strictEqual(logged, "hello");
  console.log("✓ testClientNotify");
}

async function testClientValidationEnabled() {
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
    add: async (_ctx: any, a: number, b: number) => a + b,
  });

  const transport = new InProcTransport((req) => server.call(req, {}));
  const client = await Client.create(transport, true);

  const sum = await client.Math.add(2, 3);
  assert.strictEqual(sum, 5);

  let errMsg = "";
  try {
    await client.Math.add("bad", 3);
  } catch (e: any) {
    errMsg = e.message;
  }
  assert.ok(errMsg.includes("Request validation failed"));

  console.log("✓ testClientValidationEnabled");
}

async function testClientLocalIdlBootstrap() {
  const idl = {
    interfaces: [
      {
        name: "Svc",
        methods: [
          {
            name: "ping",
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
  server.addHandler("Svc", {
    ping: async () => "pong",
  });

  const transport = new InProcTransport((req) => server.call(req, {}));
  const client = await Client.create(transport, false, false, {
    localIDL: JSON.parse(JSON.stringify(idl)),
  });

  assert.ok(client.transport);
  assert.strictEqual(await client.Svc.ping(), "pong");
  console.log("✓ testClientLocalIdlBootstrap");
}

async function testClientVerifyCompatibility() {
  const idl = {
    interfaces: [
      {
        name: "Svc",
        methods: [
          {
            name: "ping",
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
  server.addHandler("Svc", {
    ping: async () => "pong",
  });

  const transport = new InProcTransport((req) => server.call(req, {}));
  const client = await Client.create(transport, false, false, {
    localIDL: JSON.parse(JSON.stringify(idl)),
  });

  const result = await client.verifyCompatibility();
  assert.strictEqual(result.compatible, true);
  assert.strictEqual(result.deltas.length, 0);
  console.log("✓ testClientVerifyCompatibility");
}

async function testClientVerifyCompatibilityIncompatible() {
  const serverIdl = {
    interfaces: [
      {
        name: "Svc",
        methods: [
          {
            name: "ping",
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
  const clientIdl = {
    interfaces: [
      {
        name: "Svc",
        methods: [
          {
            name: "extraMethod",
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
  const contract = new Contract(serverIdl);
  const server = new Server({ contract, validateRequests: false });
  server.addHandler("Svc", {
    ping: async () => "pong",
  });

  const transport = new InProcTransport((req) => server.call(req, {}));
  const client = await Client.create(transport, false, false, {
    localIDL: JSON.parse(JSON.stringify(clientIdl)),
  });

  const result = await client.verifyCompatibility();
  assert.strictEqual(result.compatible, false);
  assert.ok(result.deltas.length > 0);
  console.log("✓ testClientVerifyCompatibilityIncompatible");
}

async function testClientSetLocalIdl() {
  const idl = {
    interfaces: [
      {
        name: "Svc",
        methods: [
          {
            name: "ping",
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
  server.addHandler("Svc", {
    ping: async () => "pong",
  });

  const transport = new InProcTransport((req) => server.call(req, {}));
  const client = await Client.create(transport);

  client.setLocalIDL(JSON.stringify(idl));
  const result = await client.verifyCompatibility();
  assert.strictEqual(result.compatible, true);
  console.log("✓ testClientSetLocalIdl");
}

async function main() {
  await testClientConstructsWithInProcTransport();
  await testClientBasicRequestResponse();
  await testClientWrapsRpcError();
  await testClientNotify();
  await testClientValidationEnabled();
  await testClientLocalIdlBootstrap();
  await testClientVerifyCompatibility();
  await testClientVerifyCompatibilityIncompatible();
  await testClientSetLocalIdl();
  console.log("\nAll client tests passed!");
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});

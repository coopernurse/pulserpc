/**
 * Tests for client response validation and response-envelope checks
 * (CommonJS variant). Covers M5 and M14.
 */

const { strict: assert } = require("assert");
const { Client } = require("../client");
const { Server } = require("../server");
const { Contract } = require("../contract");
const { InProcTransport } = require("../transport");
const { RPCError } = require("../rpc");

const idl = {
  interfaces: [
    {
      name: "Svc",
      methods: [
        { name: "requiredNull", parameters: [], returnType: { builtIn: "int" } },
        { name: "optionalNull", parameters: [], returnType: { builtIn: "int" }, returnOptional: true },
      ],
    },
  ],
  structs: [],
  enums: [],
  errors: [],
};

// Transport that answers the IDL bootstrap normally and delegates all other
// requests to `responder`, so malformed envelopes can be injected precisely.
function injectingTransport(responder) {
  return {
    async request(req) {
      if (req.method === "pulserpc-idl") {
        return { jsonrpc: "2.0", result: idl, id: req.id };
      }
      return responder(req);
    },
    close() {},
  };
}

async function testRequiredNullResultFailsValidation() {
  // Server-side validation is off so the bad result reaches the client.
  const server = new Server({ contract: new Contract(idl), validateRequests: false, validateResponses: false });
  server.addHandler("Svc", {
    requiredNull: async () => null,
    optionalNull: async () => null,
  });
  const client = await Client.create(new InProcTransport((req) => server.call(req, {})), false, true);

  await assert.rejects(
    () => client.Svc.requiredNull(),
    (e) => e instanceof Error && /Response validation failed/.test(e.message),
    "a null result for a required return must fail client response validation"
  );
  console.log("✓ testRequiredNullResultFailsValidation");
}

async function testOptionalNullResultSucceeds() {
  const server = new Server({ contract: new Contract(idl), validateRequests: false, validateResponses: false });
  server.addHandler("Svc", {
    requiredNull: async () => null,
    optionalNull: async () => null,
  });
  const client = await Client.create(new InProcTransport((req) => server.call(req, {})), false, true);

  assert.strictEqual(await client.Svc.optionalNull(), null);
  console.log("✓ testOptionalNullResultSucceeds");
}

async function testEnvelopeMismatchedId() {
  const client = await Client.create(
    injectingTransport((req) => ({ jsonrpc: "2.0", result: 1, id: Number(req.id) + 100 })),
    false,
    false
  );
  await assert.rejects(() => client.call("Svc.requiredNull", []), /mismatched id/);
  console.log("✓ testEnvelopeMismatchedId");
}

async function testEnvelopeWrongJsonrpc() {
  const client = await Client.create(
    injectingTransport((req) => ({ jsonrpc: "1.0", result: 1, id: req.id })),
    false,
    false
  );
  await assert.rejects(() => client.call("Svc.requiredNull", []), /invalid jsonrpc version/);
  console.log("✓ testEnvelopeWrongJsonrpc");
}

async function testEnvelopeResultAndErrorRejected() {
  const client = await Client.create(
    injectingTransport((req) => ({
      jsonrpc: "2.0",
      result: 1,
      error: { code: -32603, message: "boom" },
      id: req.id,
    })),
    false,
    false
  );
  await assert.rejects(() => client.call("Svc.requiredNull", []), /both result and error/);
  console.log("✓ testEnvelopeResultAndErrorRejected");
}

async function testErrorCodeZeroPreserved() {
  const client = await Client.create(
    injectingTransport((req) => ({ jsonrpc: "2.0", error: { code: 0, message: "zero" }, id: req.id })),
    false,
    false
  );
  let caught = null;
  try {
    await client.call("Svc.requiredNull", []);
  } catch (e) {
    caught = e;
  }
  assert.ok(caught instanceof RPCError);
  assert.strictEqual(caught.code, 0, "code 0 must not be remapped to -32603");
  console.log("✓ testErrorCodeZeroPreserved");
}

async function testEnvelopeNullErrorRejected() {
  // A falsy `error` member is still an error response; it must not fall
  // through to a silent `undefined` result.
  const client = await Client.create(
    injectingTransport((req) => ({ jsonrpc: "2.0", error: null, id: req.id })),
    false,
    false
  );
  await assert.rejects(
    () => client.call("Svc.requiredNull", []),
    (e) => e instanceof RPCError && e.code === -32603 && /Unknown error/.test(e.message),
    "error:null must be rejected as a malformed error envelope"
  );
  console.log("✓ testEnvelopeNullErrorRejected");
}

async function testEnvelopeMalformedErrorFallsBack() {
  // A malformed error object gets the standard internal-error code/message
  // rather than being treated as a successful result.
  const client = await Client.create(
    injectingTransport((req) => ({
      jsonrpc: "2.0",
      error: { code: "not-a-number", message: 42 },
      id: req.id,
    })),
    false,
    false
  );
  await assert.rejects(
    () => client.call("Svc.requiredNull", []),
    (e) => e instanceof RPCError && e.code === -32603 && e.message === "Unknown error",
    "a malformed error object must fall back to -32603/Unknown error"
  );
  console.log("✓ testEnvelopeMalformedErrorFallsBack");
}

async function testInProcFallbackIsValidatedAsError() {
  // The InProcTransport fallback envelope must be well-formed so the client
  // turns it into an RPCError instead of returning `undefined` silently.
  const fallback = await new InProcTransport(() => undefined).request({
    jsonrpc: "2.0",
    method: "Svc.requiredNull",
    id: 1,
  });
  assert.strictEqual(fallback.error.code, -32603);
  const client = await Client.create(
    injectingTransport((req) => ({ ...fallback, id: req.id })),
    false,
    false
  );
  await assert.rejects(
    () => client.call("Svc.requiredNull", []),
    (e) => e instanceof RPCError && /No response from server/.test(e.message),
    "the InProcTransport fallback must surface as a well-formed RPCError"
  );
  console.log("✓ testInProcFallbackIsValidatedAsError");
}

async function main() {
  await testRequiredNullResultFailsValidation();
  await testOptionalNullResultSucceeds();
  await testEnvelopeMismatchedId();
  await testEnvelopeWrongJsonrpc();
  await testEnvelopeResultAndErrorRejected();
  await testErrorCodeZeroPreserved();
  await testEnvelopeNullErrorRejected();
  await testEnvelopeMalformedErrorFallsBack();
  await testInProcFallbackIsValidatedAsError();
  console.log("\nAll client validation tests passed!");
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});

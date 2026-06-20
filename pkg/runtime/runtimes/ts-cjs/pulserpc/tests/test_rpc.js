/**
 * Tests for RPC error handling (CommonJS variant)
 */

const { strict: assert } = require("assert");
const { RPCError } = require("../rpc");

function testRPCErrorCreation() {
  const error = new RPCError(-32603, "Internal error", {
    detail: "Something went wrong",
  });
  assert.strictEqual(error.code, -32603);
  assert(error.message.includes("Internal error"), `Expected message to include "Internal error", got: ${error.message}`);
  assert.deepStrictEqual(error.data, { detail: "Something went wrong" });
  console.log("✓ testRPCErrorCreation");
}

function testRPCErrorWithoutData() {
  const error = new RPCError(-32600, "Invalid Request");
  assert.strictEqual(error.code, -32600);
  assert(error.message.includes("Invalid Request"), `Expected message to include "Invalid Request", got: ${error.message}`);
  assert.strictEqual(error.data, undefined);
  console.log("✓ testRPCErrorWithoutData");
}

function testRPCErrorStringRepresentation() {
  const error = new RPCError(-32601, "Method not found");
  const errorStr = error.toString();
  assert(errorStr.includes("RPCError"), `Expected string to include "RPCError", got: ${errorStr}`);
  assert(errorStr.includes("-32601"), `Expected string to include "-32601", got: ${errorStr}`);
  assert(errorStr.includes("Method not found"), `Expected string to include "Method not found", got: ${errorStr}`);
  console.log("✓ testRPCErrorStringRepresentation");
}

testRPCErrorCreation();
testRPCErrorWithoutData();
testRPCErrorStringRepresentation();
console.log("\nAll RPC tests passed!");

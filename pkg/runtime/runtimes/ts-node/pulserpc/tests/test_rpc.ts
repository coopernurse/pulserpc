/**
 * Tests for RPC error handling
 */

import { strict as assert } from "assert";
import { RPCError } from "../rpc.js";

function testRPCErrorCreation() {
  const error = new RPCError(-32603, "Internal error", {
    detail: "Something went wrong",
  });
  assert.strictEqual(error.code, -32603);
  // The error message includes the formatted string from super()
  assert(error.message.includes("Internal error"), `Expected message to include "Internal error", got: ${error.message}`);
  assert.deepStrictEqual(error.data, { detail: "Something went wrong" });
  console.log("✓ testRPCErrorCreation");
}

function testRPCErrorWithoutData() {
  const error = new RPCError(-32600, "Invalid Request");
  assert.strictEqual(error.code, -32600);
  // The error message includes the formatted string from super()
  assert(error.message.includes("Invalid Request"), `Expected message to include "Invalid Request", got: ${error.message}`);
  assert.strictEqual(error.data, undefined);
  console.log("✓ testRPCErrorWithoutData");
}

function testRPCErrorStringRepresentation() {
  const error = new RPCError(-32601, "Method not found");
  // Error.toString() returns the error message (which includes "RPCError")
  const errorStr = error.toString();
  // The error message should be "RPCError -32601: Method not found"
  assert(errorStr.includes("RPCError"), `Expected string to include "RPCError", got: ${errorStr}`);
  assert(errorStr.includes("-32601"), `Expected string to include "-32601", got: ${errorStr}`);
  assert(errorStr.includes("Method not found"), `Expected string to include "Method not found", got: ${errorStr}`);
  console.log("✓ testRPCErrorStringRepresentation");
}

// Run tests
testRPCErrorCreation();
testRPCErrorWithoutData();
testRPCErrorStringRepresentation();
console.log("\nAll RPC tests passed!");

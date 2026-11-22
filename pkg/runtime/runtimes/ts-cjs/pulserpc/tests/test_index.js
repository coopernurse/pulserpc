/**
 * Index-level export smoke test (CommonJS variant), covering R1: every
 * scalar/aggregate validator must be importable from the package root.
 */

const { strict: assert } = require("assert");
const {
  validateType,
  validateString,
  validateInt,
  validateFloat,
  validateBool,
  validateArray,
  validateMap,
  validateEnum,
  validateStruct,
  findStruct,
  findEnum,
  getStructFields,
  extractChecksum,
  RPCError,
  Contract,
  Server,
  Client,
  HttpTransport,
} = require("../index");

function main() {
  for (const [name, fn] of Object.entries({
    validateType,
    validateString,
    validateInt,
    validateFloat,
    validateBool,
    validateArray,
    validateMap,
    validateEnum,
    validateStruct,
    findStruct,
    findEnum,
    getStructFields,
    extractChecksum,
  })) {
    assert.strictEqual(typeof fn, "function", `${name} must be exported from index`);
  }
  for (const [name, ctor] of Object.entries({ RPCError, Contract, Server, Client, HttpTransport })) {
    assert.strictEqual(typeof ctor, "function", `${name} must be exported from index`);
  }

  // Actually call validators through the index, not just check their presence.
  assert.deepStrictEqual(validateInt(42), []);
  assert.strictEqual(validateInt(Infinity).length, 1);
  assert.deepStrictEqual(validateString("ok"), []);
  assert.deepStrictEqual(validateEnum("RED", "Color", ["RED"]), []);

  console.log("✓ testIndexExportsValidators");
  console.log("\nAll index tests passed!");
}

main();

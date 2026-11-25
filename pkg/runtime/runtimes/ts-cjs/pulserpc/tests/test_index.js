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
  INT64_SAFE_MIN,
  INT64_SAFE_MAX,
  INT32_MIN,
  INT32_MAX,
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

  // D1/R25: IDL int is signed 64-bit on the wire. JavaScript cannot represent
  // the full int64 range exactly, so the safe-integer bounds are exported and
  // wider values are rejected by validation.
  assert.strictEqual(INT64_SAFE_MIN, -(2 ** 53) + 1, "INT64_SAFE_MIN must be exported from index");
  assert.strictEqual(INT64_SAFE_MAX, 2 ** 53 - 1, "INT64_SAFE_MAX must be exported from index");
  assert.strictEqual(INT32_MIN, -2147483648, "deprecated INT32_MIN alias must stay exported");
  assert.strictEqual(INT32_MAX, 2147483647, "deprecated INT32_MAX alias must stay exported");
  assert.strictEqual(validateInt(INT64_SAFE_MIN).length, 0);
  assert.strictEqual(validateInt(INT64_SAFE_MAX).length, 0);
  assert.strictEqual(validateInt(2 ** 31).length, 0);
  assert.strictEqual(validateInt(INT64_SAFE_MAX + 1).length, 1);

  // Actually call validators through the index, not just check their presence.
  assert.deepStrictEqual(validateInt(42), []);
  assert.strictEqual(validateInt(Infinity).length, 1);
  assert.deepStrictEqual(validateString("ok"), []);
  assert.deepStrictEqual(validateEnum("RED", "Color", ["RED"]), []);

  console.log("✓ testIndexExportsValidators");
  console.log("\nAll index tests passed!");
}

main();

/**
 * Tests for validation functions
 */

import { strict as assert } from "assert";
import {
  validateString,
  validateInt,
  validateFloat,
  validateBool,
  validateArray,
  validateMap,
  validateEnum,
  validateStruct,
  validateType,
} from "../validation.js";
import { StructMap, EnumMap } from "../types.js";

function testValidateStringSuccess() {
  validateString("hello");
  validateString("");
  console.log("✓ testValidateStringSuccess");
}

function testValidateStringFailure() {
  assert.throws(() => validateString(123), /Expected string/);
  assert.throws(() => validateString(null), /Expected string/);
  console.log("✓ testValidateStringFailure");
}

function testValidateIntSuccess() {
  validateInt(0);
  validateInt(42);
  validateInt(-100);
  validateInt(5.0); // Should pass - no fractional component
  console.log("✓ testValidateIntSuccess");
}

function testValidateIntFailure() {
  assert.throws(() => validateInt("123"), /Expected number for int/);
  assert.throws(() => validateInt(3.14), /Expected integer.*fractional component/);
  assert.throws(() => validateInt(5.1), /Expected integer.*fractional component/);
  console.log("✓ testValidateIntFailure");
}

function testValidateFloatSuccess() {
  validateFloat(3.14);
  validateFloat(42); // int is acceptable
  validateFloat(-1.5);
  console.log("✓ testValidateFloatSuccess");
}

function testValidateFloatFailure() {
  assert.throws(() => validateFloat("3.14"), /Expected number for float/);
  assert.throws(() => validateFloat(null), /Expected number for float/);
  console.log("✓ testValidateFloatFailure");
}

function testValidateBoolSuccess() {
  validateBool(true);
  validateBool(false);
  console.log("✓ testValidateBoolSuccess");
}

function testValidateBoolFailure() {
  assert.throws(() => validateBool(1), /Expected boolean/);
  assert.throws(() => validateBool("true"), /Expected boolean/);
  console.log("✓ testValidateBoolFailure");
}

function testValidateArraySuccess() {
  const elementValidator = (v: any) => validateString(v);
  validateArray(["a", "b", "c"], elementValidator);
  validateArray([], elementValidator);
  console.log("✓ testValidateArraySuccess");
}

function testValidateArrayWrongType() {
  const elementValidator = (v: any) => validateString(v);
  assert.throws(() => validateArray("not a list", elementValidator), /Expected array/);
  assert.throws(() => validateArray({}, elementValidator), /Expected array/);
  console.log("✓ testValidateArrayWrongType");
}

function testValidateArrayElementValidationFails() {
  const elementValidator = (v: any) => validateString(v);
  assert.throws(
    () => validateArray(["a", 123, "c"], elementValidator),
    /Array element at index 1/
  );
  console.log("✓ testValidateArrayElementValidationFails");
}

function testValidateMapSuccess() {
  const valueValidator = (v: any) => validateInt(v);
  validateMap({ a: 1, b: 2 }, valueValidator);
  validateMap({}, valueValidator);
  console.log("✓ testValidateMapSuccess");
}

function testValidateMapWrongType() {
  const valueValidator = (v: any) => validateInt(v);
  assert.throws(() => validateMap("not a dict", valueValidator), /Expected object for map/);
  assert.throws(() => validateMap([], valueValidator), /Expected object for map/);
  console.log("✓ testValidateMapWrongType");
}

function testValidateMapNonStringKey() {
  const valueValidator = (v: any) => validateInt(v);
  // In JavaScript/TypeScript, object keys are always strings or symbols
  // So this test might not be directly applicable, but we can test that numeric keys are coerced
  const obj: any = {};
  obj[123] = 1;
  // This should actually pass because JS coerces keys to strings
  // But if someone uses Map, that's different - we're using plain objects
  validateMap(obj, valueValidator); // This will work because keys become strings
  console.log("✓ testValidateMapNonStringKey");
}

function testValidateMapValueValidationFails() {
  const valueValidator = (v: any) => validateInt(v);
  assert.throws(
    () => validateMap({ a: "not an int" }, valueValidator),
    /Map value for key 'a'/
  );
  console.log("✓ testValidateMapValueValidationFails");
}

function testValidateEnumSuccess() {
  validateEnum("kindle", "Platform", ["kindle", "nook"]);
  validateEnum("nook", "Platform", ["kindle", "nook"]);
  console.log("✓ testValidateEnumSuccess");
}

function testValidateEnumWrongType() {
  assert.throws(
    () => validateEnum(123, "Platform", ["kindle", "nook"]),
    /Expected string for enum/
  );
  console.log("✓ testValidateEnumWrongType");
}

function testValidateEnumInvalidValue() {
  assert.throws(
    () => validateEnum("invalid", "Platform", ["kindle", "nook"]),
    /Invalid value for enum/
  );
  console.log("✓ testValidateEnumInvalidValue");
}

function testValidateStructSuccess() {
  const allStructs: StructMap = {
    User: {
      fields: [
        { name: "id", type: { builtIn: "string" }, optional: false },
        { name: "name", type: { builtIn: "string" }, optional: false },
      ],
    },
  };
  const allEnums: EnumMap = {};
  const structDef = allStructs["User"];

  validateStruct(
    { id: "123", name: "Alice" },
    "User",
    structDef,
    allStructs,
    allEnums
  );
  console.log("✓ testValidateStructSuccess");
}

function testValidateStructMissingRequiredField() {
  const allStructs: StructMap = {
    User: {
      fields: [{ name: "id", type: { builtIn: "string" }, optional: false }],
    },
  };
  const allEnums: EnumMap = {};
  const structDef = allStructs["User"];

  assert.throws(
    () => validateStruct({}, "User", structDef, allStructs, allEnums),
    /Missing required field/
  );
  console.log("✓ testValidateStructMissingRequiredField");
}

function testValidateStructOptionalField() {
  const allStructs: StructMap = {
    User: {
      fields: [
        { name: "id", type: { builtIn: "string" }, optional: false },
        { name: "email", type: { builtIn: "string" }, optional: true },
      ],
    },
  };
  const allEnums: EnumMap = {};
  const structDef = allStructs["User"];

  // Should work without optional field
  validateStruct({ id: "123" }, "User", structDef, allStructs, allEnums);

  // Should work with optional field
  validateStruct(
    { id: "123", email: "alice@example.com" },
    "User",
    structDef,
    allStructs,
    allEnums
  );
  console.log("✓ testValidateStructOptionalField");
}

function testValidateStructWithExtends() {
  const allStructs: StructMap = {
    Base: {
      fields: [{ name: "id", type: { builtIn: "string" }, optional: false }],
    },
    User: {
      extends: "Base",
      fields: [{ name: "name", type: { builtIn: "string" }, optional: false }],
    },
  };
  const allEnums: EnumMap = {};
  const structDef = allStructs["User"];

  // Should validate both parent and child fields
  validateStruct(
    { id: "123", name: "Alice" },
    "User",
    structDef,
    allStructs,
    allEnums
  );

  // Should fail if parent field missing
  assert.throws(
    () => validateStruct({ name: "Alice" }, "User", structDef, allStructs, allEnums),
    /Missing required field/
  );
  console.log("✓ testValidateStructWithExtends");
}

function testValidateTypeString() {
  const allStructs: StructMap = {};
  const allEnums: EnumMap = {};
  validateType("hello", { builtIn: "string" }, allStructs, allEnums);
  console.log("✓ testValidateTypeString");
}

function testValidateTypeOptionalNone() {
  const allStructs: StructMap = {};
  const allEnums: EnumMap = {};
  // null is valid for optional fields
  validateType(null, { builtIn: "string" }, allStructs, allEnums, true);
  // undefined is NOT valid for optional fields (per spec: only null is valid)
  assert.throws(
    () => validateType(undefined, { builtIn: "string" }, allStructs, allEnums, true),
    /cannot be undefined/
  );
  // null is NOT valid for non-optional fields
  assert.throws(
    () => validateType(null, { builtIn: "string" }, allStructs, allEnums, false),
    /cannot be null/
  );
  console.log("✓ testValidateTypeOptionalNone");
}

function testValidateTypeArray() {
  const allStructs: StructMap = {};
  const allEnums: EnumMap = {};
  const typeDef = { array: { builtIn: "string" } };
  validateType(["a", "b"], typeDef, allStructs, allEnums);

  assert.throws(
    () => validateType(["a", 123], typeDef, allStructs, allEnums),
    (err: any) => err instanceof Error
  );
  console.log("✓ testValidateTypeArray");
}

function testValidateTypeMap() {
  const allStructs: StructMap = {};
  const allEnums: EnumMap = {};
  const typeDef = { mapValue: { builtIn: "int" } };
  validateType({ a: 1, b: 2 }, typeDef, allStructs, allEnums);

  assert.throws(
    () => validateType({ a: "not int" }, typeDef, allStructs, allEnums),
    (err: any) => err instanceof Error
  );
  console.log("✓ testValidateTypeMap");
}

// Run all tests
testValidateStringSuccess();
testValidateStringFailure();
testValidateIntSuccess();
testValidateIntFailure();
testValidateFloatSuccess();
testValidateFloatFailure();
testValidateBoolSuccess();
testValidateBoolFailure();
testValidateArraySuccess();
testValidateArrayWrongType();
testValidateArrayElementValidationFails();
testValidateMapSuccess();
testValidateMapWrongType();
testValidateMapNonStringKey();
testValidateMapValueValidationFails();
testValidateEnumSuccess();
testValidateEnumWrongType();
testValidateEnumInvalidValue();
testValidateStructSuccess();
testValidateStructMissingRequiredField();
testValidateStructOptionalField();
testValidateStructWithExtends();
testValidateTypeString();
testValidateTypeOptionalNone();
testValidateTypeArray();
testValidateTypeMap();
console.log("\nAll validation tests passed!");

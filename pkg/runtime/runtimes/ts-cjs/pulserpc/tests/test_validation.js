/**
 * Tests for validation functions (CommonJS variant)
 */

const { strict: assert } = require("assert");
const {
  validateString,
  validateInt,
  validateFloat,
  validateBool,
  validateArray,
  validateMap,
  validateEnum,
  validateStruct,
  validateType,
} = require("../validation");

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
  validateInt(5.0);
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
  validateFloat(42);
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
  const elementValidator = (v) => validateString(v);
  validateArray(["a", "b", "c"], elementValidator);
  validateArray([], elementValidator);
  console.log("✓ testValidateArraySuccess");
}

function testValidateArrayWrongType() {
  const elementValidator = (v) => validateString(v);
  assert.throws(() => validateArray("not a list", elementValidator), /Expected array/);
  assert.throws(() => validateArray({}, elementValidator), /Expected array/);
  console.log("✓ testValidateArrayWrongType");
}

function testValidateArrayElementValidationFails() {
  const elementValidator = (v) => validateString(v);
  assert.throws(
    () => validateArray(["a", 123, "c"], elementValidator),
    /Array element at index 1/
  );
  console.log("✓ testValidateArrayElementValidationFails");
}

function testValidateMapSuccess() {
  const valueValidator = (v) => validateInt(v);
  validateMap({ a: 1, b: 2 }, valueValidator);
  validateMap({}, valueValidator);
  console.log("✓ testValidateMapSuccess");
}

function testValidateMapWrongType() {
  const valueValidator = (v) => validateInt(v);
  assert.throws(() => validateMap("not a dict", valueValidator), /Expected object for map/);
  assert.throws(() => validateMap([], valueValidator), /Expected object for map/);
  console.log("✓ testValidateMapWrongType");
}

function testValidateMapNonStringKey() {
  const valueValidator = (v) => validateInt(v);
  const obj = {};
  obj[123] = 1;
  validateMap(obj, valueValidator);
  console.log("✓ testValidateMapNonStringKey");
}

function testValidateMapValueValidationFails() {
  const valueValidator = (v) => validateInt(v);
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
  const allStructs = {
    User: {
      fields: [
        { name: "id", type: { builtIn: "string" }, optional: false },
        { name: "name", type: { builtIn: "string" }, optional: false },
      ],
    },
  };
  const allEnums = {};
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
  const allStructs = {
    User: {
      fields: [{ name: "id", type: { builtIn: "string" }, optional: false }],
    },
  };
  const allEnums = {};
  const structDef = allStructs["User"];

  assert.throws(
    () => validateStruct({}, "User", structDef, allStructs, allEnums),
    /Missing required field/
  );
  console.log("✓ testValidateStructMissingRequiredField");
}

function testValidateStructOptionalField() {
  const allStructs = {
    User: {
      fields: [
        { name: "id", type: { builtIn: "string" }, optional: false },
        { name: "email", type: { builtIn: "string" }, optional: true },
      ],
    },
  };
  const allEnums = {};
  const structDef = allStructs["User"];

  validateStruct({ id: "123" }, "User", structDef, allStructs, allEnums);

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
  const allStructs = {
    Base: {
      fields: [{ name: "id", type: { builtIn: "string" }, optional: false }],
    },
    User: {
      extends: "Base",
      fields: [{ name: "name", type: { builtIn: "string" }, optional: false }],
    },
  };
  const allEnums = {};
  const structDef = allStructs["User"];

  validateStruct(
    { id: "123", name: "Alice" },
    "User",
    structDef,
    allStructs,
    allEnums
  );

  assert.throws(
    () => validateStruct({ name: "Alice" }, "User", structDef, allStructs, allEnums),
    /Missing required field/
  );
  console.log("✓ testValidateStructWithExtends");
}

function testValidateTypeString() {
  const allStructs = {};
  const allEnums = {};
  validateType("hello", { builtIn: "string" }, allStructs, allEnums);
  console.log("✓ testValidateTypeString");
}

function testValidateTypeOptionalNone() {
  const allStructs = {};
  const allEnums = {};
  validateType(null, { builtIn: "string" }, allStructs, allEnums, true);
  assert.throws(
    () => validateType(undefined, { builtIn: "string" }, allStructs, allEnums, true),
    /cannot be undefined/
  );
  assert.throws(
    () => validateType(null, { builtIn: "string" }, allStructs, allEnums, false),
    /cannot be null/
  );
  console.log("✓ testValidateTypeOptionalNone");
}

function testValidateTypeArray() {
  const allStructs = {};
  const allEnums = {};
  const typeDef = { array: { builtIn: "string" } };
  validateType(["a", "b"], typeDef, allStructs, allEnums);

  assert.throws(
    () => validateType(["a", 123], typeDef, allStructs, allEnums),
    (err) => err instanceof Error
  );
  console.log("✓ testValidateTypeArray");
}

function testValidateTypeMap() {
  const allStructs = {};
  const allEnums = {};
  const typeDef = { mapValue: { builtIn: "int" } };
  validateType({ a: 1, b: 2 }, typeDef, allStructs, allEnums);

  assert.throws(
    () => validateType({ a: "not int" }, typeDef, allStructs, allEnums),
    (err) => err instanceof Error
  );
  console.log("✓ testValidateTypeMap");
}

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

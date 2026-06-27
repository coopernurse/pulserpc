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
  assert.deepEqual(validateString("hello"), []);
  assert.deepEqual(validateString(""), []);
  console.log("✓ testValidateStringSuccess");
}

function testValidateStringFailure() {
  const errs = validateString(123);
  assert.equal(errs.length, 1);
  assert.ok(errs[0].message.includes("Expected string"));
  assert.equal(errs[0].path, "");

  const errs2 = validateString(null);
  assert.equal(errs2.length, 1);
  assert.ok(errs2[0].message.includes("Expected string"));
  console.log("✓ testValidateStringFailure");
}

function testValidateIntSuccess() {
  assert.deepEqual(validateInt(0), []);
  assert.deepEqual(validateInt(42), []);
  assert.deepEqual(validateInt(-100), []);
  assert.deepEqual(validateInt(5.0), []);
  console.log("✓ testValidateIntSuccess");
}

function testValidateIntFailure() {
  const errs = validateInt("123");
  assert.equal(errs.length, 1);
  assert.ok(errs[0].message.includes("Expected number for int"));

  const errs2 = validateInt(3.14);
  assert.equal(errs2.length, 1);
  assert.ok(errs2[0].message.includes("Expected integer"));

  assert.equal(validateInt(5.1).length, 1);
  console.log("✓ testValidateIntFailure");
}

function testValidateFloatSuccess() {
  assert.deepEqual(validateFloat(3.14), []);
  assert.deepEqual(validateFloat(42), []);
  assert.deepEqual(validateFloat(-1.5), []);
  console.log("✓ testValidateFloatSuccess");
}

function testValidateFloatFailure() {
  const errs = validateFloat("3.14");
  assert.equal(errs.length, 1);
  assert.ok(errs[0].message.includes("Expected number for float"));

  assert.equal(validateFloat(null).length, 1);
  console.log("✓ testValidateFloatFailure");
}

function testValidateBoolSuccess() {
  assert.deepEqual(validateBool(true), []);
  assert.deepEqual(validateBool(false), []);
  console.log("✓ testValidateBoolSuccess");
}

function testValidateBoolFailure() {
  assert.equal(validateBool(1).length, 1);
  assert.equal(validateBool("true").length, 1);
  console.log("✓ testValidateBoolFailure");
}

function testValidateArraySuccess() {
  const elementValidator = (v, p) => validateString(v, p);
  assert.deepEqual(validateArray(["a", "b", "c"], elementValidator), []);
  assert.deepEqual(validateArray([], elementValidator), []);
  console.log("✓ testValidateArraySuccess");
}

function testValidateArrayWrongType() {
  const elementValidator = (v, p) => validateString(v, p);
  const errs = validateArray("not a list", elementValidator);
  assert.equal(errs.length, 1);
  assert.ok(errs[0].message.includes("Expected array"));
  console.log("✓ testValidateArrayWrongType");
}

function testValidateArrayElementValidationFails() {
  const elementValidator = (v, p) => validateString(v, p);
  const errs = validateArray(["a", 123, "c"], elementValidator);
  assert.equal(errs.length, 1);
  assert.ok(errs[0].path.includes("[1]"));
  assert.ok(errs[0].message.includes("Expected string"));
  console.log("✓ testValidateArrayElementValidationFails");
}

function testValidateArrayPathTracking() {
  const elementValidator = (v, p) => validateString(v, p);
  const errs = validateArray(["a", "b", 42], elementValidator, ".items");
  assert.equal(errs.length, 1);
  assert.equal(errs[0].path, ".items[2]");
  console.log("✓ testValidateArrayPathTracking");
}

function testValidateMapSuccess() {
  const valueValidator = (v, p) => validateInt(v, p);
  assert.deepEqual(validateMap({ a: 1, b: 2 }, valueValidator), []);
  assert.deepEqual(validateMap({}, valueValidator), []);
  console.log("✓ testValidateMapSuccess");
}

function testValidateMapWrongType() {
  const valueValidator = (v, p) => validateInt(v, p);
  const errs1 = validateMap("not a dict", valueValidator);
  assert.equal(errs1.length, 1);
  assert.ok(errs1[0].message.includes("Expected object for map"));

  const errs2 = validateMap([], valueValidator);
  assert.equal(errs2.length, 1);
  console.log("✓ testValidateMapWrongType");
}

function testValidateMapValueValidationFails() {
  const valueValidator = (v, p) => validateInt(v, p);
  const errs = validateMap({ a: "not an int" }, valueValidator);
  assert.equal(errs.length, 1);
  assert.ok(errs[0].path.includes("[a]"));
  console.log("✓ testValidateMapValueValidationFails");
}

function testValidateEnumSuccess() {
  assert.deepEqual(validateEnum("kindle", "Platform", ["kindle", "nook"]), []);
  assert.deepEqual(validateEnum("nook", "Platform", ["kindle", "nook"]), []);
  console.log("✓ testValidateEnumSuccess");
}

function testValidateEnumWrongType() {
  const errs = validateEnum(123, "Platform", ["kindle", "nook"]);
  assert.equal(errs.length, 1);
  assert.ok(errs[0].message.includes("Expected string for enum"));
  console.log("✓ testValidateEnumWrongType");
}

function testValidateEnumInvalidValue() {
  const errs = validateEnum("invalid", "Platform", ["kindle", "nook"]);
  assert.equal(errs.length, 1);
  assert.ok(errs[0].message.includes("Invalid value for enum"));
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

  const errs = validateStruct(
    { id: "123", name: "Alice" },
    "User",
    allStructs["User"],
    allStructs,
    allEnums
  );
  assert.deepEqual(errs, []);
  console.log("✓ testValidateStructSuccess");
}

function testValidateStructMissingRequiredField() {
  const allStructs = {
    User: {
      fields: [{ name: "id", type: { builtIn: "string" }, optional: false }],
    },
  };
  const allEnums = {};

  const errs = validateStruct({}, "User", allStructs["User"], allStructs, allEnums);
  assert.equal(errs.length, 1);
  assert.ok(errs[0].message.includes("Missing required field"));
  assert.equal(errs[0].path, ".id");
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

  assert.deepEqual(validateStruct({ id: "123" }, "User", allStructs["User"], allStructs, allEnums), []);
  assert.deepEqual(validateStruct({ id: "123", email: "alice@example.com" }, "User", allStructs["User"], allStructs, allEnums), []);
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

  assert.deepEqual(
    validateStruct({ id: "123", name: "Alice" }, "User", allStructs["User"], allStructs, allEnums),
    []
  );

  const errs = validateStruct({ name: "Alice" }, "User", allStructs["User"], allStructs, allEnums);
  assert.equal(errs.length, 1);
  assert.ok(errs[0].message.includes("Missing required field"));
  assert.equal(errs[0].path, ".id");
  console.log("✓ testValidateStructWithExtends");
}

function testValidateStructCollectsMultipleErrors() {
  const allStructs = {
    Person: {
      fields: [
        { name: "username", type: { builtIn: "string" } },
        { name: "age", type: { builtIn: "int" } },
        { name: "email", type: { builtIn: "string" } },
      ],
    },
  };
  const allEnums = {};

  const errs = validateStruct(
    { username: "alice", age: "not-a-number", email: 42 },
    "Person",
    allStructs["Person"],
    allStructs,
    allEnums
  );
  assert.equal(errs.length, 2);
  assert.equal(errs[0].path, ".age");
  assert.equal(errs[1].path, ".email");
  console.log("✓ testValidateStructCollectsMultipleErrors");
}

function testValidateTypeString() {
  const allStructs = {};
  const allEnums = {};
  assert.deepEqual(validateType("hello", { builtIn: "string" }, allStructs, allEnums), []);
  console.log("✓ testValidateTypeString");
}

function testValidateTypeOptionalNone() {
  const allStructs = {};
  const allEnums = {};

  assert.deepEqual(validateType(null, { builtIn: "string" }, allStructs, allEnums, true), []);

  const errs1 = validateType(undefined, { builtIn: "string" }, allStructs, allEnums, true);
  assert.equal(errs1.length, 1);
  assert.ok(errs1[0].message.includes("cannot be undefined"));

  const errs2 = validateType(null, { builtIn: "string" }, allStructs, allEnums, false);
  assert.equal(errs2.length, 1);
  assert.ok(errs2[0].message.includes("cannot be null"));
  console.log("✓ testValidateTypeOptionalNone");
}

function testValidateTypeArray() {
  const allStructs = {};
  const allEnums = {};
  const typeDef = { array: { builtIn: "string" } };

  assert.deepEqual(validateType(["a", "b"], typeDef, allStructs, allEnums), []);

  const errs = validateType(["a", 123], typeDef, allStructs, allEnums);
  assert.equal(errs.length, 1);
  assert.ok(errs[0].path.includes("[1]"));
  console.log("✓ testValidateTypeArray");
}

function testValidateTypeMap() {
  const allStructs = {};
  const allEnums = {};
  const typeDef = { mapValue: { builtIn: "int" } };

  assert.deepEqual(validateType({ a: 1, b: 2 }, typeDef, allStructs, allEnums), []);

  const errs = validateType({ a: "not int" }, typeDef, allStructs, allEnums);
  assert.equal(errs.length, 1);
  assert.ok(errs[0].path.includes("[a]"));
  console.log("✓ testValidateTypeMap");
}

function testValidateNestedStructInArray() {
  const allStructs = {
    Child: {
      fields: [
        { name: "name", type: { builtIn: "string" } },
        { name: "age", type: { builtIn: "int" } },
      ],
    },
    Person: {
      fields: [
        { name: "name", type: { builtIn: "string" } },
        { name: "children", type: { array: { userDefined: "Child" } } },
      ],
    },
  };
  const allEnums = {};

  assert.deepEqual(
    validateType(
      { name: "Alice", children: [{ name: "Bob", age: 10 }, { name: "Charlie", age: 12 }] },
      { userDefined: "Person" },
      allStructs,
      allEnums
    ),
    []
  );

  const errs = validateType(
    { name: "Alice", children: [{ name: "Bob", age: 10 }, { name: "Charlie", age: "twelve" }] },
    { userDefined: "Person" },
    allStructs,
    allEnums
  );
  assert.equal(errs.length, 1);
  assert.equal(errs[0].path, ".children[1].age");
  console.log("✓ testValidateNestedStructInArray");
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
testValidateArrayPathTracking();
testValidateMapSuccess();
testValidateMapWrongType();
testValidateMapValueValidationFails();
testValidateEnumSuccess();
testValidateEnumWrongType();
testValidateEnumInvalidValue();
testValidateStructSuccess();
testValidateStructMissingRequiredField();
testValidateStructOptionalField();
testValidateStructWithExtends();
testValidateStructCollectsMultipleErrors();
testValidateTypeString();
testValidateTypeOptionalNone();
testValidateTypeArray();
testValidateTypeMap();
testValidateNestedStructInArray();
console.log("\nAll validation tests passed!");

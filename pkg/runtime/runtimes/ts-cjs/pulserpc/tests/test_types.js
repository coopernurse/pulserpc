/**
 * Tests for type helper functions (CommonJS variant)
 */

const { strict: assert } = require("assert");
const {
  findStruct,
  findEnum,
  getStructFields,
} = require("../types");

function testFindStruct() {
  const allStructs = {
    User: { fields: [] },
    Book: { fields: [] },
  };
  assert.deepStrictEqual(findStruct("User", allStructs), { fields: [] });
  assert.deepStrictEqual(findStruct("Book", allStructs), { fields: [] });
  assert.strictEqual(findStruct("NotFound", allStructs), undefined);
  console.log("✓ testFindStruct");
}

function testFindEnum() {
  const allEnums = {
    Platform: { values: [] },
  };
  assert.deepStrictEqual(findEnum("Platform", allEnums), { values: [] });
  assert.strictEqual(findEnum("NotFound", allEnums), undefined);
  console.log("✓ testFindEnum");
}

function testGetStructFieldsSimple() {
  const allStructs = {
    User: {
      fields: [
        { name: "id", type: { builtIn: "string" } },
        { name: "name", type: { builtIn: "string" } },
      ],
    },
  };
  const fields = getStructFields("User", allStructs);
  assert.strictEqual(fields.length, 2);
  assert.strictEqual(fields[0].name, "id");
  assert.strictEqual(fields[1].name, "name");
  console.log("✓ testGetStructFieldsSimple");
}

function testGetStructFieldsWithExtends() {
  const allStructs = {
    Base: {
      fields: [{ name: "id", type: { builtIn: "string" } }],
    },
    User: {
      extends: "Base",
      fields: [{ name: "name", type: { builtIn: "string" } }],
    },
  };
  const fields = getStructFields("User", allStructs);
  assert.strictEqual(fields.length, 2);
  assert.strictEqual(fields[0].name, "id");
  assert.strictEqual(fields[1].name, "name");
  console.log("✓ testGetStructFieldsWithExtends");
}

function testGetStructFieldsOverrideParent() {
  const allStructs = {
    Base: {
      fields: [{ name: "id", type: { builtIn: "string" } }],
    },
    User: {
      extends: "Base",
      fields: [
        { name: "id", type: { builtIn: "int" } },
        { name: "name", type: { builtIn: "string" } },
      ],
    },
  };
  const fields = getStructFields("User", allStructs);
  assert.strictEqual(fields.length, 2);
  assert.strictEqual(fields[0].type.builtIn, "int");
  assert.strictEqual(fields[1].name, "name");
  console.log("✓ testGetStructFieldsOverrideParent");
}

testFindStruct();
testFindEnum();
testGetStructFieldsSimple();
testGetStructFieldsWithExtends();
testGetStructFieldsOverrideParent();
console.log("\nAll type tests passed!");

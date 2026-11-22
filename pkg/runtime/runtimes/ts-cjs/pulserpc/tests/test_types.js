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

function testFindQualifiedTypesByBaseName() {
  // Generated idl.json keeps imported names qualified while local references
  // are unqualified; lookups must resolve both forms.
  const allStructs = {
    "inc.Response": { fields: [{ name: "status", type: { userDefined: "Status" } }] },
  };
  const allEnums = { "inc.Status": { values: [{ name: "ok" }] } };
  assert.deepStrictEqual(findStruct("Response", allStructs), allStructs["inc.Response"]);
  assert.deepStrictEqual(findStruct("inc.Response", allStructs), allStructs["inc.Response"]);
  assert.deepStrictEqual(findEnum("Status", allEnums), allEnums["inc.Status"]);
  assert.strictEqual(findStruct("Missing", allStructs), undefined);
  assert.strictEqual(findEnum("Missing", allEnums), undefined);
  console.log("✓ testFindQualifiedTypesByBaseName");
}

function testFindByTypeNameCachesIndex() {
  // The base-name index is built lazily once per types map. A Proxy that
  // counts key enumerations proves repeated lookups never re-scan the map.
  const defs = {
    "inc.Response": { id: "response" },
    "inc.Status": { id: "status" },
  };
  let ownKeysCalls = 0;
  const watched = new Proxy(defs, {
    ownKeys(target) {
      ownKeysCalls++;
      return Reflect.ownKeys(target);
    },
  });

  assert.strictEqual(findStruct("Response", watched).id, "response");
  const afterFirst = ownKeysCalls;
  assert.ok(afterFirst >= 1, "the first base-name lookup must build the index");

  for (let i = 0; i < 50; i++) {
    assert.strictEqual(findStruct("Response", watched).id, "response");
    assert.strictEqual(findStruct("inc.Response", watched).id, "response");
    assert.strictEqual(findStruct("Nope", watched), undefined);
  }
  assert.strictEqual(
    ownKeysCalls,
    afterFirst,
    "lookups after the first must use the cached index, not re-enumerate the map"
  );
  console.log("✓ testFindByTypeNameCachesIndex");
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
testFindQualifiedTypesByBaseName();
testFindByTypeNameCachesIndex();
testGetStructFieldsSimple();
testGetStructFieldsWithExtends();
testGetStructFieldsOverrideParent();
console.log("\nAll type tests passed!");

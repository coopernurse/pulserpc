/**
 * Tests for type helper functions
 */

import { strict as assert } from "assert";
import {
  findStruct,
  findEnum,
  getStructFields,
  StructMap,
  EnumMap,
} from "../types.js";

function testFindStruct() {
  const allStructs: StructMap = {
    User: { fields: [] },
    Book: { fields: [] },
  };
  assert.deepStrictEqual(findStruct("User", allStructs), { fields: [] });
  assert.deepStrictEqual(findStruct("Book", allStructs), { fields: [] });
  assert.strictEqual(findStruct("NotFound", allStructs), undefined);
  console.log("✓ testFindStruct");
}

function testFindEnum() {
  const allEnums: EnumMap = {
    Platform: { values: [] },
  };
  assert.deepStrictEqual(findEnum("Platform", allEnums), { values: [] });
  assert.strictEqual(findEnum("NotFound", allEnums), undefined);
  console.log("✓ testFindEnum");
}

function testGetStructFieldsSimple() {
  const allStructs: StructMap = {
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
  const allStructs: StructMap = {
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
  assert.strictEqual(fields[0].name, "id"); // Parent field first
  assert.strictEqual(fields[1].name, "name"); // Child field second
  console.log("✓ testGetStructFieldsWithExtends");
}

function testGetStructFieldsOverrideParent() {
  const allStructs: StructMap = {
    Base: {
      fields: [{ name: "id", type: { builtIn: "string" } }],
    },
    User: {
      extends: "Base",
      fields: [
        { name: "id", type: { builtIn: "int" } }, // Override parent
        { name: "name", type: { builtIn: "string" } },
      ],
    },
  };
  const fields = getStructFields("User", allStructs);
  assert.strictEqual(fields.length, 2);
  // Child field should override parent
  assert.strictEqual(fields[0].type.builtIn, "int");
  assert.strictEqual(fields[1].name, "name");
  console.log("✓ testGetStructFieldsOverrideParent");
}

function testFindQualifiedTypesByBaseName() {
  // Generated idl.json keeps imported names qualified while local references
  // are unqualified; lookups must resolve both forms (invariant R6).
  const allStructs: StructMap = {
    "inc.Response": { fields: [{ name: "status", type: { userDefined: "Status" } }] },
  };
  const allEnums: EnumMap = {
    "inc.Status": { values: [{ name: "ok" }] },
  };
  assert.deepStrictEqual(findStruct("Response", allStructs), allStructs["inc.Response"]);
  assert.deepStrictEqual(findStruct("inc.Response", allStructs), allStructs["inc.Response"]);
  assert.deepStrictEqual(findEnum("Status", allEnums), allEnums["inc.Status"]);
  assert.strictEqual(findStruct("Missing", allStructs), undefined);
  assert.strictEqual(findEnum("Missing", allEnums), undefined);
  console.log("testFindQualifiedTypesByBaseName");
}

function testAmbiguousBaseNameIsDeterministic() {
  // Ambiguity resolves to the first key in sorted order, matching Go, Java
  // and Python (invariant R6).
  const allStructs: StructMap = {
    "b.Status": { fields: [] },
    "a.Status": { fields: [] },
  };
  assert.strictEqual(findStruct("x.Status", allStructs), allStructs["a.Status"]);
  console.log("testAmbiguousBaseNameIsDeterministic");
}

// Run tests
testFindStruct();
testFindEnum();
testFindQualifiedTypesByBaseName();
testAmbiguousBaseNameIsDeterministic();
testGetStructFieldsSimple();
testGetStructFieldsWithExtends();
testGetStructFieldsOverrideParent();
console.log("\nAll type tests passed!");

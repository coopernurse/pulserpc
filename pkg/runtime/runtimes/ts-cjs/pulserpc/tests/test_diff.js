/**
 * IDL diff / contract-verification tests (CommonJS variant).
 *
 * Port of ts-node's 14 diff scenarios (R13) plus a regression for the
 * severity contradiction resolved by the core audit (core-12): a removed
 * required field is ClientHasMore/Info, an added required field is
 * ClientHasLess/Error.
 */

const { strict: assert } = require("assert");
const { diffIDL, classifySeverity } = require("../diff");
const { EntityType, ChangeType, Direction, Severity } = require("../types");
const { FailFastAuditor } = require("../contract");
const { Client } = require("../client");

function testIdenticalIDLs() {
  const idl = {
    interfaces: [
      {
        name: "TestService",
        methods: [{ name: "getData", parameters: [], returnType: { builtIn: "string" } }],
      },
    ],
    structs: [{ name: "TestStruct", fields: [{ name: "field1", type: { builtIn: "string" }, optional: false }] }],
    enums: [{ name: "TestEnum", values: [{ name: "Value1" }, { name: "Value2" }] }],
    errors: [{ name: "TestError", code: 100 }],
  };
  assert.strictEqual(diffIDL(idl, idl).length, 0);
  console.log("✓ testIdenticalIDLs");
}

function testAddedOptionalField() {
  const clientIDL = {
    interfaces: [],
    structs: [{ name: "TestStruct", fields: [{ name: "field1", type: { builtIn: "string" }, optional: false }] }],
    enums: [],
    errors: [],
  };
  const serverIDL = {
    interfaces: [],
    structs: [
      {
        name: "TestStruct",
        fields: [
          { name: "field1", type: { builtIn: "string" }, optional: false },
          { name: "field2", type: { builtIn: "string" }, optional: true },
        ],
      },
    ],
    enums: [],
    errors: [],
  };
  const deltas = diffIDL(clientIDL, serverIDL);
  assert.strictEqual(deltas.length, 1);
  assert.strictEqual(deltas[0].entityType, EntityType.Field);
  assert.strictEqual(deltas[0].changeType, ChangeType.Added);
  assert.strictEqual(deltas[0].direction, Direction.ClientHasLess);
  assert.strictEqual(deltas[0].severity, Severity.Info);
  console.log("✓ testAddedOptionalField");
}

function testAddedRequiredField() {
  const clientIDL = {
    interfaces: [],
    structs: [{ name: "TestStruct", fields: [{ name: "field1", type: { builtIn: "string" }, optional: false }] }],
    enums: [],
    errors: [],
  };
  const serverIDL = {
    interfaces: [],
    structs: [
      {
        name: "TestStruct",
        fields: [
          { name: "field1", type: { builtIn: "string" }, optional: false },
          { name: "field2", type: { builtIn: "string" }, optional: false },
        ],
      },
    ],
    enums: [],
    errors: [],
  };
  const deltas = diffIDL(clientIDL, serverIDL);
  assert.strictEqual(deltas.length, 1);
  assert.strictEqual(deltas[0].severity, Severity.Error);
  console.log("✓ testAddedRequiredField");
}

function testRemovedField() {
  const clientIDL = {
    interfaces: [],
    structs: [
      {
        name: "TestStruct",
        fields: [
          { name: "field1", type: { builtIn: "string" }, optional: false },
          { name: "field2", type: { builtIn: "string" }, optional: true },
        ],
      },
    ],
    enums: [],
    errors: [],
  };
  const serverIDL = {
    interfaces: [],
    structs: [{ name: "TestStruct", fields: [{ name: "field1", type: { builtIn: "string" }, optional: false }] }],
    enums: [],
    errors: [],
  };
  const deltas = diffIDL(clientIDL, serverIDL);
  assert.strictEqual(deltas.length, 1);
  assert.strictEqual(deltas[0].entityType, EntityType.Field);
  assert.strictEqual(deltas[0].changeType, ChangeType.Removed);
  assert.strictEqual(deltas[0].direction, Direction.ClientHasMore);
  assert.strictEqual(deltas[0].severity, Severity.Info);
  console.log("✓ testRemovedField");
}

function testRemovedRequiredField() {
  const clientIDL = {
    interfaces: [],
    structs: [
      {
        name: "TestStruct",
        fields: [
          { name: "field1", type: { builtIn: "string" }, optional: false },
          { name: "field2", type: { builtIn: "string" }, optional: false },
        ],
      },
    ],
    enums: [],
    errors: [],
  };
  const serverIDL = {
    interfaces: [],
    structs: [{ name: "TestStruct", fields: [{ name: "field1", type: { builtIn: "string" }, optional: false }] }],
    enums: [],
    errors: [],
  };
  const deltas = diffIDL(clientIDL, serverIDL);
  assert.strictEqual(deltas.length, 1);
  assert.strictEqual(deltas[0].memberName, "field2");
  assert.strictEqual(deltas[0].severity, Severity.Info);
  assert.strictEqual(
    classifySeverity(EntityType.Field, ChangeType.Removed, Direction.ClientHasMore, "required"),
    Severity.Info
  );
  console.log("✓ testRemovedRequiredField");
}

function testFieldMadeOptional() {
  const clientIDL = {
    interfaces: [],
    structs: [{ name: "TestStruct", fields: [{ name: "field1", type: { builtIn: "string" }, optional: false }] }],
    enums: [],
    errors: [],
  };
  const serverIDL = {
    interfaces: [],
    structs: [{ name: "TestStruct", fields: [{ name: "field1", type: { builtIn: "string" }, optional: true }] }],
    enums: [],
    errors: [],
  };
  const deltas = diffIDL(clientIDL, serverIDL);
  assert.strictEqual(deltas.length, 1);
  assert.strictEqual(deltas[0].entityType, EntityType.Field);
  assert.strictEqual(deltas[0].changeType, ChangeType.Modified);
  assert.strictEqual(deltas[0].direction, Direction.ClientHasLess);
  assert.strictEqual(deltas[0].severity, Severity.Info);
  assert.ok(deltas[0].description.includes("required to optional"));
  console.log("✓ testFieldMadeOptional");
}

function testFieldMadeRequired() {
  const clientIDL = {
    interfaces: [],
    structs: [{ name: "TestStruct", fields: [{ name: "field1", type: { builtIn: "string" }, optional: true }] }],
    enums: [],
    errors: [],
  };
  const serverIDL = {
    interfaces: [],
    structs: [{ name: "TestStruct", fields: [{ name: "field1", type: { builtIn: "string" }, optional: false }] }],
    enums: [],
    errors: [],
  };
  const deltas = diffIDL(clientIDL, serverIDL);
  assert.strictEqual(deltas.length, 1);
  assert.strictEqual(deltas[0].severity, Severity.Warning);
  assert.ok(deltas[0].description.includes("optional to required"));
  console.log("✓ testFieldMadeRequired");
}

function testStructRemovedFromServer() {
  const clientIDL = { interfaces: [], structs: [{ name: "TestStruct", fields: [] }], enums: [], errors: [] };
  const serverIDL = { interfaces: [], structs: [], enums: [], errors: [] };
  const deltas = diffIDL(clientIDL, serverIDL);
  assert.strictEqual(deltas.length, 1);
  assert.strictEqual(deltas[0].entityType, EntityType.Struct);
  assert.strictEqual(deltas[0].changeType, ChangeType.Removed);
  assert.strictEqual(deltas[0].direction, Direction.ClientHasMore);
  assert.strictEqual(deltas[0].severity, Severity.Error);
  console.log("✓ testStructRemovedFromServer");
}

function testInterfaceAddedToServer() {
  const clientIDL = { interfaces: [], structs: [], enums: [], errors: [] };
  const serverIDL = { interfaces: [{ name: "TestService", methods: [] }], structs: [], enums: [], errors: [] };
  const deltas = diffIDL(clientIDL, serverIDL);
  assert.strictEqual(deltas.length, 1);
  assert.strictEqual(deltas[0].entityType, EntityType.Interface);
  assert.strictEqual(deltas[0].changeType, ChangeType.Added);
  assert.strictEqual(deltas[0].direction, Direction.ClientHasLess);
  assert.strictEqual(deltas[0].severity, Severity.Info);
  console.log("✓ testInterfaceAddedToServer");
}

function testClassifySeverityRemovedStruct() {
  assert.strictEqual(classifySeverity(EntityType.Struct, ChangeType.Removed, Direction.ClientHasMore), Severity.Error);
  console.log("✓ testClassifySeverityRemovedStruct");
}

function testClassifySeverityAddedStruct() {
  assert.strictEqual(classifySeverity(EntityType.Struct, ChangeType.Added, Direction.ClientHasLess), Severity.Info);
  console.log("✓ testClassifySeverityAddedStruct");
}

function testClassifySeverityModifiedFieldMismatch() {
  assert.strictEqual(classifySeverity(EntityType.Field, ChangeType.Modified, Direction.Mismatch), Severity.Error);
  console.log("✓ testClassifySeverityModifiedFieldMismatch");
}

function testClassifySeverityAddedRequiredField() {
  assert.strictEqual(
    classifySeverity(EntityType.Field, ChangeType.Added, Direction.ClientHasLess, "required"),
    Severity.Error
  );
  console.log("✓ testClassifySeverityAddedRequiredField");
}

function testClassifySeverityAddedOptionalField() {
  assert.strictEqual(
    classifySeverity(EntityType.Field, ChangeType.Added, Direction.ClientHasLess, "optional"),
    Severity.Info
  );
  console.log("✓ testClassifySeverityAddedOptionalField");
}

function testClassifySeverityMadeRequired() {
  assert.strictEqual(
    classifySeverity(EntityType.Field, ChangeType.Modified, Direction.ClientHasLess, "made_required"),
    Severity.Warning
  );
  console.log("✓ testClassifySeverityMadeRequired");
}

function testClassifySeverityMadeOptional() {
  assert.strictEqual(
    classifySeverity(EntityType.Field, ChangeType.Modified, Direction.ClientHasLess, "made_optional"),
    Severity.Info
  );
  console.log("✓ testClassifySeverityMadeOptional");
}

function clientWithIDLs(clientIDL, serverIDL) {
  const client = Object.create(Client.prototype);
  client.contract = { idlParsed: serverIDL };
  client._localIDL = clientIDL;
  client._auditor = undefined;
  return client;
}

async function testRemovedRequiredFieldCompatibility() {
  const clientIDL = {
    interfaces: [],
    structs: [
      {
        name: "T",
        fields: [
          { name: "requiredField", type: { builtIn: "string" }, optional: false },
          { name: "optionalField", type: { builtIn: "string" }, optional: true },
        ],
      },
    ],
    enums: [],
    errors: [],
  };
  const serverIDL = { interfaces: [], structs: [{ name: "T", fields: [] }], enums: [], errors: [] };
  const result = await clientWithIDLs(clientIDL, serverIDL).verifyCompatibility();
  assert.strictEqual(result.compatible, true, "removed fields are Info per the resolved spec scenario");
  assert.strictEqual(result.deltas.length, 2);
  assert.ok(result.deltas.every((d) => d.severity === Severity.Info));
  assert.doesNotThrow(() => new FailFastAuditor().audit(result));
  console.log("✓ testRemovedRequiredFieldCompatibility");
}

async function testAddedRequiredFieldCompatibilityFails() {
  const clientIDL = {
    interfaces: [],
    structs: [{ name: "T", fields: [{ name: "a", type: { builtIn: "string" }, optional: false }] }],
    enums: [],
    errors: [],
  };
  const serverIDL = {
    interfaces: [],
    structs: [
      {
        name: "T",
        fields: [
          { name: "a", type: { builtIn: "string" }, optional: false },
          { name: "b", type: { builtIn: "string" }, optional: false },
        ],
      },
    ],
    enums: [],
    errors: [],
  };
  const result = await clientWithIDLs(clientIDL, serverIDL).verifyCompatibility();
  assert.strictEqual(result.compatible, false);
  assert.ok(result.deltas.some((d) => d.severity === Severity.Error));
  assert.throws(() => new FailFastAuditor().audit(result), /Contract compatibility verification failed/);
  console.log("✓ testAddedRequiredFieldCompatibilityFails");
}

async function main() {
  testIdenticalIDLs();
  testAddedOptionalField();
  testAddedRequiredField();
  testRemovedField();
  testRemovedRequiredField();
  testFieldMadeOptional();
  testFieldMadeRequired();
  testStructRemovedFromServer();
  testInterfaceAddedToServer();
  testClassifySeverityRemovedStruct();
  testClassifySeverityAddedStruct();
  testClassifySeverityModifiedFieldMismatch();
  testClassifySeverityAddedRequiredField();
  testClassifySeverityAddedOptionalField();
  testClassifySeverityMadeRequired();
  testClassifySeverityMadeOptional();
  await testRemovedRequiredFieldCompatibility();
  await testAddedRequiredFieldCompatibilityFails();
  console.log("\nAll diff tests passed!");
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});

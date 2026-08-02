import { strict as assert } from "assert";
import {
  validateStruct, validateType,
} from "../validation.js";
import {
  findStruct, findEnum, getStructFields,
  StructMap, EnumMap, extractChecksum,
} from "../types.js";
import { RPCError } from "../rpc.js";
import { Contract } from "../contract.js";
import { diffIDL, classifySeverity } from "../diff.js";
import {
  EntityType, ChangeType, Direction, Severity,
} from "../types.js";

function test_shared_validateType_covers_all_builtins() {
  const s: StructMap = {};
  const e: EnumMap = {};
  assert.deepStrictEqual(validateType("hello", { builtIn: "string" }, s, e), []);
  assert.deepStrictEqual(validateType(42, { builtIn: "int" }, s, e), []);
  assert.strictEqual(validateType(3.14, { builtIn: "int" }, s, e).length, 1);
  assert.deepStrictEqual(validateType(3.14, { builtIn: "float" }, s, e), []);
  assert.deepStrictEqual(validateType(true, { builtIn: "bool" }, s, e), []);
  assert.strictEqual(validateType("true", { builtIn: "bool" }, s, e).length, 1);
  console.log("  ✓ validateType covers all builtins");
}

function test_shared_validateType_array() {
  const s: StructMap = {};
  const e: EnumMap = {};
  const arrType = { array: { builtIn: "string" } };
  assert.deepStrictEqual(validateType(["a", "b"], arrType, s, e), []);
  const errs = validateType(["a", 123], arrType, s, e);
  assert.strictEqual(errs.length, 1);
  assert.ok(errs[0].path.includes("[1]"));
  console.log("  ✓ validateType array");
}

function test_shared_validateType_map() {
  const s: StructMap = {};
  const e: EnumMap = {};
  const mapType = { mapValue: { builtIn: "int" } };
  assert.deepStrictEqual(validateType({ a: 1, b: 2 }, mapType, s, e), []);
  const errs = validateType({ a: "x" }, mapType, s, e);
  assert.strictEqual(errs.length, 1);
  console.log("  ✓ validateType map");
}

function test_shared_validateType_optional_null() {
  const s: StructMap = {};
  const e: EnumMap = {};
  assert.deepStrictEqual(validateType(null, { builtIn: "string" }, s, e, true), []);
  const errs = validateType(null, { builtIn: "string" }, s, e, false);
  assert.strictEqual(errs.length, 1);
  const errs2 = validateType(undefined, { builtIn: "string" }, s, e, true);
  assert.strictEqual(errs2.length, 1);
  console.log("  ✓ validateType optional/null");
}

function test_shared_validateStruct_extends() {
  const allStructs: StructMap = {
    Base: { fields: [{ name: "id", type: { builtIn: "string" }, optional: false }] },
    Child: {
      extends: "Base",
      fields: [{ name: "name", type: { builtIn: "string" }, optional: false }],
    },
  };
  const e: EnumMap = {};
  assert.deepStrictEqual(
    validateStruct({ id: "123", name: "Alice" }, "Child", allStructs, e),
    []
  );
  const errs = validateStruct({ name: "Alice" }, "Child", allStructs, e);
  assert.strictEqual(errs.length, 1);
  assert.strictEqual(errs[0].path, ".id");
  console.log("  ✓ validateStruct with extends");
}

function test_shared_validateStruct_override() {
  const allStructs: StructMap = {
    Base: { fields: [{ name: "id", type: { builtIn: "string" }, optional: false }] },
    Child: {
      extends: "Base",
      fields: [{ name: "id", type: { builtIn: "int" }, optional: false }],
    },
  };
  const e: EnumMap = {};
  assert.deepStrictEqual(
    validateStruct({ id: 1 }, "Child", allStructs, e),
    []
  );
  const errs = validateStruct({ id: "x" }, "Child", allStructs, e);
  assert.strictEqual(errs.length, 1);
  assert.strictEqual(errs[0].path, ".id");
  console.log("  ✓ validateStruct override parent");
}

function test_shared_getStructFields() {
  const allStructs: StructMap = {
    Base: { fields: [{ name: "id", type: { builtIn: "string" }, optional: false }] },
    Child: {
      extends: "Base",
      fields: [{ name: "name", type: { builtIn: "string" }, optional: false }],
    },
  };
  const fields = getStructFields("Child", allStructs);
  assert.strictEqual(fields.length, 2);
  assert.strictEqual(fields[0].name, "id");
  assert.strictEqual(fields[1].name, "name");
  console.log("  ✓ getStructFields with extends");
}

function test_shared_getStructFields_override() {
  const allStructs: StructMap = {
    Base: { fields: [{ name: "id", type: { builtIn: "string" }, optional: false }] },
    Child: {
      extends: "Base",
      fields: [{ name: "id", type: { builtIn: "int" }, optional: false }],
    },
  };
  const fields = getStructFields("Child", allStructs);
  assert.strictEqual(fields.length, 1);
  assert.strictEqual(fields[0].type.builtIn, "int");
  console.log("  ✓ getStructFields with override");
}

function test_shared_findStruct_enum() {
  const s: StructMap = { User: { fields: [] } };
  const e: EnumMap = { Status: { values: [{ name: "ok" }] } };
  assert.ok(findStruct("User", s));
  assert.strictEqual(findStruct("Unknown", s), undefined);
  assert.ok(findEnum("Status", e));
  assert.strictEqual(findEnum("Unknown", e), undefined);
  console.log("  ✓ findStruct / findEnum");
}

function test_shared_extractChecksum() {
  assert.strictEqual(extractChecksum({ checksum: "abc123" }), "abc123");
  assert.strictEqual(extractChecksum({}), "");
  assert.strictEqual(extractChecksum(null), "");
  assert.strictEqual(extractChecksum("not an object"), "");
  console.log("  ✓ extractChecksum");
}

function test_shared_RPCError() {
  const err = new RPCError(-32603, "Internal error", { detail: "boom" });
  assert.strictEqual(err.code, -32603);
  assert.ok(err.message.includes("Internal error"));
  assert.deepStrictEqual(err.data, { detail: "boom" });
  assert.ok(err.toString().includes("RPCError"));
  assert.ok(err.toString().includes("-32603"));
  console.log("  ✓ RPCError");
}

function test_shared_diffIDL_identical() {
  const idl = {
    interfaces: [{ name: "Svc", methods: [{ name: "f", parameters: [], returnType: { builtIn: "string" } }] }],
    structs: [{ name: "T", fields: [{ name: "x", type: { builtIn: "string" }, optional: false }] }],
    enums: [{ name: "E", values: [{ name: "v1" }] }],
    errors: [{ name: "Err", code: 100 }],
  };
  assert.strictEqual(diffIDL(idl, idl).length, 0);
  console.log("  ✓ diffIDL identical");
}

function test_shared_diffIDL_interface_added() {
  const c = { interfaces: [], structs: [], enums: [], errors: [] };
  const s = { interfaces: [{ name: "Svc", methods: [] }], structs: [], enums: [], errors: [] };
  const deltas = diffIDL(c, s);
  assert.strictEqual(deltas.length, 1);
  assert.strictEqual(deltas[0].entityType, EntityType.Interface);
  assert.strictEqual(deltas[0].changeType, ChangeType.Added);
  console.log("  ✓ diffIDL interface added");
}

function test_shared_diffIDL_method_removed() {
  const c = { interfaces: [{ name: "Svc", methods: [{ name: "f", parameters: [] }] }], structs: [], enums: [], errors: [] };
  const s = { interfaces: [{ name: "Svc", methods: [] }], structs: [], enums: [], errors: [] };
  const deltas = diffIDL(c, s);
  assert.strictEqual(deltas.length, 1);
  assert.strictEqual(deltas[0].entityType, EntityType.Method);
  assert.strictEqual(deltas[0].changeType, ChangeType.Removed);
  assert.strictEqual(deltas[0].severity, Severity.Error);
  console.log("  ✓ diffIDL method removed");
}

function test_shared_classifySeverity() {
  assert.strictEqual(classifySeverity(EntityType.Struct, ChangeType.Removed, Direction.ClientHasMore), Severity.Error);
  assert.strictEqual(classifySeverity(EntityType.Struct, ChangeType.Added, Direction.ClientHasLess), Severity.Info);
  assert.strictEqual(classifySeverity(EntityType.Field, ChangeType.Modified, Direction.Mismatch), Severity.Error);
  assert.strictEqual(classifySeverity(EntityType.Field, ChangeType.Added, Direction.ClientHasLess, "required"), Severity.Error);
  assert.strictEqual(classifySeverity(EntityType.Field, ChangeType.Added, Direction.ClientHasLess, "optional"), Severity.Info);
  assert.strictEqual(classifySeverity(EntityType.Field, ChangeType.Modified, Direction.ClientHasLess, "made_required"), Severity.Warning);
  assert.strictEqual(classifySeverity(EntityType.Field, ChangeType.Modified, Direction.ClientHasLess, "made_optional"), Severity.Info);
  console.log("  ✓ classifySeverity");
}

function test_shared_contract_pulse_format() {
  const idl = {
    interfaces: [{ name: "Svc", methods: [{ name: "hello", parameters: [], returnType: { builtIn: "string" } }] }],
    structs: [{ name: "Person", fields: [{ name: "name", type: { builtIn: "string" }, optional: false }] }],
    enums: [{ name: "Color", values: [{ name: "red" }, { name: "blue" }] }],
  };
  const contract = new Contract(idl);
  assert.ok(contract.hasInterface("Svc"));
  const iface = contract.getInterface("Svc");
  assert.ok(iface);
  assert.ok(iface!.getFunction("hello"));
  console.log("  ✓ Contract (PulseRPC format)");
}

function test_shared_contract_barrister_format() {
  const idl = [
    { type: "interface", name: "Svc", functions: [{ name: "ping", params: [], returns: { type: "string" } }] },
    { type: "struct", name: "Person", fields: [{ name: "name", type: "string", optional: false }] },
  ];
  const contract = new Contract(idl);
  assert.ok(contract.hasInterface("Svc"));
  console.log("  ✓ Contract (Barrister format)");
}

function run(label: string, fn: () => void) {
  try {
    fn();
  } catch (e: any) {
    console.error(`FAILED: ${label} — ${e.message}`);
    process.exit(1);
  }
}

run("test_shared_validateType_covers_all_builtins", test_shared_validateType_covers_all_builtins);
run("test_shared_validateType_array", test_shared_validateType_array);
run("test_shared_validateType_map", test_shared_validateType_map);
run("test_shared_validateType_optional_null", test_shared_validateType_optional_null);
run("test_shared_validateStruct_extends", test_shared_validateStruct_extends);
run("test_shared_validateStruct_override", test_shared_validateStruct_override);
run("test_shared_getStructFields", test_shared_getStructFields);
run("test_shared_getStructFields_override", test_shared_getStructFields_override);
run("test_shared_findStruct_enum", test_shared_findStruct_enum);
run("test_shared_extractChecksum", test_shared_extractChecksum);
run("test_shared_RPCError", test_shared_RPCError);
run("test_shared_diffIDL_identical", test_shared_diffIDL_identical);
run("test_shared_diffIDL_interface_added", test_shared_diffIDL_interface_added);
run("test_shared_diffIDL_method_removed", test_shared_diffIDL_method_removed);
run("test_shared_classifySeverity", test_shared_classifySeverity);
run("test_shared_contract_pulse_format", test_shared_contract_pulse_format);
run("test_shared_contract_barrister_format", test_shared_contract_barrister_format);
console.log("\nAll shared tests passed!");

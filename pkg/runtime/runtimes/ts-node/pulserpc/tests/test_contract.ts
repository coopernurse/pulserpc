/**
 * Unit tests for Contract, its validation helpers, IDL metadata and the
 * contract-verification auditors (ts-node-03).
 */

import { strict as assert } from "assert";
import {
  Contract,
  NoOpAuditor,
  LoggingAuditor,
  FailFastAuditor,
  VerificationResult,
} from "../contract.js";
import { RPCError } from "../rpc.js";
import {
  EntityType, ChangeType, Direction, Severity, ContractDelta,
} from "../types.js";

function sampleIDL(): Record<string, any> {
  return {
    interfaces: [
      {
        name: "Svc",
        methods: [
          {
            name: "greet",
            parameters: [{ name: "name", type: { builtIn: "string" } }],
            returnType: { builtIn: "string" },
          },
          {
            name: "opt",
            parameters: [
              { name: "a", type: { builtIn: "int" } },
              { name: "b", type: { builtIn: "string" }, optional: true },
            ],
            returnType: { builtIn: "string" },
          },
          { name: "voidFn", parameters: [] },
          {
            name: "optReturn",
            parameters: [],
            returnType: { builtIn: "string" },
            returnOptional: true,
          },
        ],
      },
    ],
    structs: [
      { name: "User", fields: [{ name: "email", type: { builtIn: "string" } }] },
    ],
    enums: [],
  };
}

function testValidate() {
  const contract = new Contract(sampleIDL());
  assert.deepStrictEqual(contract.validate("User", { email: "a@b.c" }), { valid: true });
  const bad = contract.validate("User", {});
  assert.strictEqual(bad.valid, false);
  assert.ok(bad.error?.includes("email"));
  assert.deepStrictEqual(bad.invalidFields, [".email"]);
  console.log("✓ testValidate");
}

function testValidateRequestTrailingOptional() {
  const contract = new Contract(sampleIDL());
  // M9: a short array is accepted when the omitted trailing params are optional.
  assert.doesNotThrow(() => contract.validateRequest("Svc", "opt", [1]));
  assert.doesNotThrow(() => contract.validateRequest("Svc", "opt", [1, "x"]));

  // Missing required params and extra params are still invalid.
  assert.throws(
    () => contract.validateRequest("Svc", "opt", []),
    (e: any) => e instanceof RPCError && e.code === -32602
  );
  assert.throws(
    () => contract.validateRequest("Svc", "opt", [1, "x", 3]),
    (e: any) => e instanceof RPCError && e.code === -32602
  );
  // An absent optional value at a named position is accepted too.
  assert.doesNotThrow(() => contract.validateRequest("Svc", "opt", [1, undefined]));
  console.log("✓ testValidateRequestTrailingOptional");
}

function testValidateResponseNullability() {
  const contract = new Contract(sampleIDL());
  assert.doesNotThrow(() => contract.validateResponse("Svc", "greet", "hi"));
  assert.throws(
    () => contract.validateResponse("Svc", "greet", 123),
    (e: any) => e instanceof RPCError && e.code === -32603
  );
  // M14: null is a real value and must be checked against returnOptional.
  assert.throws(
    () => contract.validateResponse("Svc", "greet", null),
    (e: any) => e instanceof RPCError && e.code === -32603
  );
  assert.doesNotThrow(() => contract.validateResponse("Svc", "optReturn", null));
  // R14: void returns accept null only, not undefined.
  assert.doesNotThrow(() => contract.validateResponse("Svc", "voidFn", null));
  assert.throws(
    () => contract.validateResponse("Svc", "voidFn", undefined),
    (e: any) => e instanceof RPCError && e.code === -32603
  );
  console.log("✓ testValidateResponseNullability");
}

function testValidationErrorDataShape() {
  const contract = new Contract(sampleIDL());
  try {
    contract.validateRequest("Svc", "greet", [123]);
    assert.fail("expected validateRequest to throw");
  } catch (e: any) {
    assert.ok(e instanceof RPCError);
    assert.strictEqual(e.code, -32602);
    // M13: TS exposes a structured ValidationResult (validation.md:245-247).
    assert.strictEqual(e.data.valid, false);
    assert.strictEqual(typeof e.data.error, "string");
    assert.deepStrictEqual(e.data.invalidFields, ["param[0]"]);
  }
  console.log("✓ testValidationErrorDataShape");
}

function testMetaParsing() {
  // R12: Barrister-format `meta` items.
  const barrister = new Contract([
    { type: "meta", namespace: "x", version: 2 },
    { type: "interface", name: "Svc", methods: [] },
  ]);
  assert.strictEqual(barrister.meta.namespace, "x");
  assert.strictEqual(barrister.meta.version, 2);
  assert.strictEqual("type" in barrister.meta, false);

  // The dict form accepts both a `meta` object and a list of meta items.
  const dictObject = new Contract({ meta: { namespace: "y" }, interfaces: [] });
  assert.strictEqual(dictObject.meta.namespace, "y");
  const dictList = new Contract({
    meta: [{ type: "meta", namespace: "z" }],
    interfaces: [],
  });
  assert.strictEqual(dictList.meta.namespace, "z");
  console.log("✓ testMetaParsing");
}

function delta(severity: Severity): ContractDelta {
  return {
    entityType: EntityType.Field,
    entityName: "T",
    memberName: "f",
    changeType: ChangeType.Added,
    direction: Direction.ClientHasLess,
    severity,
    description: "test delta",
  };
}

function result(severity: Severity, compatible: boolean): VerificationResult {
  return {
    compatible,
    serverChecksum: "s",
    clientChecksum: "c",
    deltas: [delta(severity)],
    timestamp: new Date(),
  };
}

function testAuditors() {
  const noOp = new NoOpAuditor();
  assert.strictEqual(noOp.name(), "NoOp");
  assert.doesNotThrow(() => noOp.audit(result(Severity.Error, false)));

  const failFast = new FailFastAuditor();
  assert.strictEqual(failFast.name(), "FailFast");
  assert.throws(() => failFast.audit(result(Severity.Error, false)), /compatibility verification failed/);
  assert.doesNotThrow(() => failFast.audit(result(Severity.Info, true)));

  const logging = new LoggingAuditor();
  assert.strictEqual(logging.name(), "Logging");
  const original = { error: console.error, warn: console.warn, info: console.info };
  const seen: string[] = [];
  console.error = (...args: any[]) => { seen.push(args.join(" ")); };
  console.warn = (...args: any[]) => { seen.push(args.join(" ")); };
  console.info = (...args: any[]) => { seen.push(args.join(" ")); };
  try {
    logging.audit(result(Severity.Error, false));
    logging.audit(result(Severity.Info, true));
  } finally {
    console.error = original.error;
    console.warn = original.warn;
    console.info = original.info;
  }
  assert.ok(seen.some((line) => line.includes("test delta")));
  console.log("✓ testAuditors");
}

// All tests are synchronous; a shared runner keeps failures explicit.
function run(label: string, fn: () => void): void {
  try {
    fn();
  } catch (e: any) {
    console.error(`FAILED: ${label} — ${e.message}`);
    process.exit(1);
  }
}

run("testValidate", testValidate);
run("testValidateRequestTrailingOptional", testValidateRequestTrailingOptional);
run("testValidateResponseNullability", testValidateResponseNullability);
run("testValidationErrorDataShape", testValidationErrorDataShape);
run("testMetaParsing", testMetaParsing);
run("testAuditors", testAuditors);
console.log("\nAll contract tests passed!");

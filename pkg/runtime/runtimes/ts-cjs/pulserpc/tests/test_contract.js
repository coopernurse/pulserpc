const { strict: assert } = require("assert");
const { Contract, InterfaceImpl, NoOpAuditor, LoggingAuditor, FailFastAuditor } = require("../contract");

function testContractConstructsWithEmptyIDL() {
  const c = new Contract({ interfaces: [], structs: [], enums: [], errors: [] });
  assert.strictEqual(c.interfaces.size, 0);
  assert.strictEqual(Object.keys(c.structs).length, 0);
  assert.strictEqual(Object.keys(c.enums).length, 0);
  console.log("✓ testContractConstructsWithEmptyIDL");
}

function testContractParsesInterfaces() {
  const idl = {
    interfaces: [
      {
        name: "Math",
        methods: [
          { name: "add", parameters: [], returnType: { builtIn: "int" } },
          { name: "sub", parameters: [], returnType: { builtIn: "int" } },
        ],
      },
    ],
    structs: [],
    enums: [],
    errors: [],
  };
  const c = new Contract(idl);
  assert.strictEqual(c.interfaces.size, 1);
  assert.ok(c.hasInterface("Math"));
  assert.ok(!c.hasInterface("Missing"));
  const iface = c.getInterface("Math");
  assert.ok(iface instanceof InterfaceImpl);
  assert.strictEqual(iface.name, "Math");
  assert.ok(iface.getFunction("add"));
  assert.ok(iface.getFunction("sub"));
  assert.strictEqual(iface.getFunction("nope"), undefined);
  console.log("✓ testContractParsesInterfaces");
}

function testContractParsesStructs() {
  const idl = {
    interfaces: [],
    structs: [
      { name: "User", fields: [{ name: "id", type: { builtIn: "int" } }] },
    ],
    enums: [],
    errors: [],
  };
  const c = new Contract(idl);
  assert.ok(c.structs["User"]);
  assert.strictEqual(c.structs["User"].fields.length, 1);
  assert.strictEqual(Object.keys(c.structs).length, 1);
  console.log("✓ testContractParsesStructs");
}

function testContractParsesEnums() {
  const idl = {
    interfaces: [],
    structs: [],
    enums: [{ name: "Color", values: ["RED", "GREEN", "BLUE"] }],
    errors: [],
  };
  const c = new Contract(idl);
  assert.ok(c.enums["Color"]);
  assert.strictEqual(c.enums["Color"].values.length, 3);
  console.log("✓ testContractParsesEnums");
}

function testContractValidateRequestPasses() {
  const idl = {
    interfaces: [
      {
        name: "Calc",
        methods: [
          {
            name: "add",
            parameters: [
              { name: "a", type: { builtIn: "int" } },
              { name: "b", type: { builtIn: "int" } },
            ],
            returnType: { builtIn: "int" },
          },
        ],
      },
    ],
    structs: [],
    enums: [],
    errors: [],
  };
  const c = new Contract(idl);
  assert.doesNotThrow(() => c.validateRequest("Calc", "add", [1, 2]));
  console.log("✓ testContractValidateRequestPasses");
}

function testContractValidateRequestFailsOnWrongParamCount() {
  const idl = {
    interfaces: [
      {
        name: "Calc",
        methods: [
          {
            name: "add",
            parameters: [
              { name: "a", type: { builtIn: "int" } },
              { name: "b", type: { builtIn: "int" } },
            ],
            returnType: { builtIn: "int" },
          },
        ],
      },
    ],
    structs: [],
    enums: [],
    errors: [],
  };
  const c = new Contract(idl);
  assert.throws(() => c.validateRequest("Calc", "add", [1]), /expects 2 param/);
  assert.throws(() => c.validateRequest("Calc", "add", [1, 2, 3]), /expects 2 param/);
  console.log("✓ testContractValidateRequestFailsOnWrongParamCount");
}

function testContractValidateRequestFailsOnUnknownInterface() {
  const c = new Contract({ interfaces: [], structs: [], enums: [], errors: [] });
  assert.throws(() => c.validateRequest("Ghost", "ping", []), /Unknown interface/);
  console.log("✓ testContractValidateRequestFailsOnUnknownInterface");
}

function testContractValidateRequestFailsOnUnknownFunction() {
  const idl = {
    interfaces: [{ name: "Svc", methods: [] }],
    structs: [],
    enums: [],
    errors: [],
  };
  const c = new Contract(idl);
  assert.throws(() => c.validateRequest("Svc", "missing", []), /Unknown function/);
  console.log("✓ testContractValidateRequestFailsOnUnknownFunction");
}

function testContractValidateResponsePasses() {
  const idl = {
    interfaces: [
      {
        name: "Calc",
        methods: [
          { name: "getAnswer", parameters: [], returnType: { builtIn: "int" } },
        ],
      },
    ],
    structs: [],
    enums: [],
    errors: [],
  };
  const c = new Contract(idl);
  assert.doesNotThrow(() => c.validateResponse("Calc", "getAnswer", 42));
  console.log("✓ testContractValidateResponsePasses");
}

function testContractValidateResponseFailsOnWrongType() {
  const idl = {
    interfaces: [
      {
        name: "Calc",
        methods: [
          { name: "getAnswer", parameters: [], returnType: { builtIn: "int" } },
        ],
      },
    ],
    structs: [],
    enums: [],
    errors: [],
  };
  const c = new Contract(idl);
  assert.throws(() => c.validateResponse("Calc", "getAnswer", "wrong"), /invalid response/);
  console.log("✓ testContractValidateResponseFailsOnWrongType");
}

function testNoOpAuditor() {
  const a = new NoOpAuditor();
  assert.doesNotThrow(() => a.audit({ compatible: true, deltas: [] }));
  assert.strictEqual(a.name(), "NoOp");
  console.log("✓ testNoOpAuditor");
}

function testFailFastAuditorThrowsOnIncompatible() {
  const a = new FailFastAuditor();
  assert.throws(() => a.audit({ compatible: false, deltas: [{ severity: "Error", entityType: "test", description: "boom" }] }), /Contract compatibility/);
  assert.doesNotThrow(() => a.audit({ compatible: true, deltas: [] }));
  console.log("✓ testFailFastAuditorThrowsOnIncompatible");
}

function main() {
  testContractConstructsWithEmptyIDL();
  testContractParsesInterfaces();
  testContractParsesStructs();
  testContractParsesEnums();
  testContractValidateRequestPasses();
  testContractValidateRequestFailsOnWrongParamCount();
  testContractValidateRequestFailsOnUnknownInterface();
  testContractValidateRequestFailsOnUnknownFunction();
  testContractValidateResponsePasses();
  testContractValidateResponseFailsOnWrongType();
  testNoOpAuditor();
  testFailFastAuditorThrowsOnIncompatible();
  console.log("\nAll contract tests passed!");
}

main();

const { strict: assert } = require("assert");
const { Contract, InterfaceImpl, NoOpAuditor, LoggingAuditor, FailFastAuditor } = require("../contract");
const { Severity } = require("../types");

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

function testContractParsesMeta() {
  // R12: metadata must survive both IDL JSON shapes.
  const objectForm = new Contract({
    interfaces: [],
    structs: [],
    enums: [],
    errors: [],
    meta: [{ key: "k", value: "v" }, { type: "meta", author: "me" }],
  });
  assert.strictEqual(objectForm.meta.k, "v");
  assert.strictEqual(objectForm.meta.author, "me");

  const objectMapForm = new Contract({
    interfaces: [],
    structs: [],
    enums: [],
    errors: [],
    meta: { version: "1", type: "meta" },
  });
  assert.strictEqual(objectMapForm.meta.version, "1");
  assert.strictEqual(objectMapForm.meta.type, undefined);

  const arrayForm = new Contract([
    { type: "meta", key: "k", value: "v" },
    { type: "meta", owner: "team" },
    { type: "interface", name: "Svc", methods: [] },
  ]);
  assert.strictEqual(arrayForm.meta.k, "v");
  assert.strictEqual(arrayForm.meta.owner, "team");
  assert.ok(arrayForm.hasInterface("Svc"));
  console.log("✓ testContractParsesMeta");
}

function testMetaPrototypePollution() {
  // A server's pulserpc-idl response is remote-controlled: a `__proto__` key
  // must not change the prototype of the metadata object (security finding).
  const listItem = JSON.parse(
    '{"type":"meta","name":"ok","__proto__":{"polluted":true},' +
    '"constructor":{"polluted":true},"prototype":{"polluted":true}}'
  );
  const fromList = new Contract([listItem]);
  assert.strictEqual(Object.getPrototypeOf(fromList.meta), Object.prototype);
  assert.strictEqual(fromList.meta.polluted, undefined);
  assert.strictEqual(fromList.meta.name, "ok", "benign keys must still be copied");
  for (const key of ["__proto__", "constructor", "prototype"]) {
    assert.strictEqual(Object.prototype.hasOwnProperty.call(fromList.meta, key), false);
  }

  const dictMeta = JSON.parse('{"name":"ok","__proto__":{"polluted":true}}');
  const fromDict = new Contract({ meta: dictMeta, interfaces: [] });
  assert.strictEqual(Object.getPrototypeOf(fromDict.meta), Object.prototype);
  assert.strictEqual(fromDict.meta.polluted, undefined);
  assert.strictEqual(fromDict.meta.name, "ok");
  assert.strictEqual(Object.prototype.hasOwnProperty.call(fromDict.meta, "__proto__"), false);
  assert.strictEqual({}.polluted, undefined, "Object.prototype must stay clean");

  // The { key, value } item form must reject unsafe keys too.
  const kvForm = new Contract({
    meta: [
      { key: "__proto__", value: { polluted: true } },
      { key: "safe", value: 1 },
    ],
  });
  assert.strictEqual(Object.getPrototypeOf(kvForm.meta), Object.prototype);
  assert.strictEqual(kvForm.meta.safe, 1);
  assert.strictEqual(Object.prototype.hasOwnProperty.call(kvForm.meta, "__proto__"), false);
  console.log("✓ testMetaPrototypePollution");
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

function testContractValidateRequestTrailingOptionalParams() {
  const idl = {
    interfaces: [
      {
        name: "Calc",
        methods: [
          {
            name: "greet",
            parameters: [
              { name: "name", type: { builtIn: "string" } },
              { name: "times", type: { builtIn: "int" }, optional: true },
            ],
            returnType: { builtIn: "string" },
          },
        ],
      },
    ],
    structs: [],
    enums: [],
    errors: [],
  };
  const c = new Contract(idl);
  assert.doesNotThrow(() => c.validateRequest("Calc", "greet", ["Ann"]));
  assert.doesNotThrow(() => c.validateRequest("Calc", "greet", ["Ann", 2]));
  assert.throws(() => c.validateRequest("Calc", "greet", []), /expects 2 param/);
  assert.throws(() => c.validateRequest("Calc", "greet", ["Ann", 2, 3]), /expects 2 param/);
  console.log("✓ testContractValidateRequestTrailingOptionalParams");
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
  assert.throws(() => a.audit({ compatible: false, deltas: [{ severity: Severity.Error, entityType: "test", description: "boom" }] }), /Contract compatibility/);
  assert.doesNotThrow(() => a.audit({ compatible: true, deltas: [] }));
  console.log("✓ testFailFastAuditorThrowsOnIncompatible");
}

function testLoggingAuditorDispatchesBySeverity() {
  const a = new LoggingAuditor();
  const calls = { error: [], warn: [], info: [] };
  const originals = {
    error: console.error,
    warn: console.warn,
    info: console.info,
  };

  try {
    console.error = (...args) => calls.error.push(args.join(" "));
    console.warn = (...args) => calls.warn.push(args.join(" "));
    console.info = (...args) => calls.info.push(args.join(" "));

    const delta = (severity, description) => ({
      severity,
      entityType: "Field",
      description,
    });
    a.audit({
      compatible: false,
      deltas: [
        delta(Severity.Error, "error-delta"),
        delta(Severity.Warning, "warning-delta"),
        delta(Severity.Info, "info-delta"),
      ],
    });
    assert.strictEqual(calls.error.length, 2, "incompatible header + Error delta");
    assert.ok(calls.error.some((m) => m.includes("error-delta")));
    assert.strictEqual(calls.warn.length, 1);
    assert.ok(calls.warn[0].includes("warning-delta"));
    assert.strictEqual(calls.info.length, 1);
    assert.ok(calls.info[0].includes("info-delta"));

    // A compatible, zero-delta result logs exactly the success message.
    calls.error.length = 0;
    calls.warn.length = 0;
    calls.info.length = 0;
    a.audit({ compatible: true, deltas: [] });
    assert.strictEqual(calls.error.length, 0);
    assert.strictEqual(calls.warn.length, 0);
    assert.strictEqual(calls.info.length, 1);
    assert.ok(calls.info[0].includes("identical"), calls.info[0]);
  } finally {
    console.error = originals.error;
    console.warn = originals.warn;
    console.info = originals.info;
  }
  console.log("✓ testLoggingAuditorDispatchesBySeverity");
}

function main() {
  testContractConstructsWithEmptyIDL();
  testContractParsesInterfaces();
  testContractParsesStructs();
  testContractParsesEnums();
  testContractParsesMeta();
  testMetaPrototypePollution();
  testContractValidateRequestPasses();
  testContractValidateRequestFailsOnWrongParamCount();
  testContractValidateRequestTrailingOptionalParams();
  testContractValidateRequestFailsOnUnknownInterface();
  testContractValidateRequestFailsOnUnknownFunction();
  testContractValidateResponsePasses();
  testContractValidateResponseFailsOnWrongType();
  testNoOpAuditor();
  testFailFastAuditorThrowsOnIncompatible();
  testLoggingAuditorDispatchesBySeverity();
  console.log("\nAll contract tests passed!");
}

main();

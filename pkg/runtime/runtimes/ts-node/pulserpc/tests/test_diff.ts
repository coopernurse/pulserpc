/**
 * Tests for diff.ts contract verification functionality
 */

import { diffIDL, classifySeverity } from "./diff.js";
import { EntityType, ChangeType, Direction, Severity } from "./types.js";

describe("diffIDL", () => {
  describe("identical IDLs", () => {
    it("produces no deltas for identical IDLs", () => {
      const idl = {
        interfaces: [
          {
            name: "TestService",
            methods: [
              { name: "getData", parameters: [], returnType: { builtIn: "string" } },
            ],
          },
        ],
        structs: [
          { name: "TestStruct", fields: [{ name: "field1", type: { builtIn: "string" }, optional: false }] },
        ],
        enums: [{ name: "TestEnum", values: [{ name: "Value1" }, { name: "Value2" }] }],
        errors: [{ name: "TestError", code: 100 }],
      };
      const deltas = diffIDL(idl, idl);
      expect(deltas).toHaveLength(0);
    });
  });

  describe("field changes", () => {
    it("detects added optional field with Info severity", () => {
      const clientIDL = {
        interfaces: [],
        structs: [
          {
            name: "TestStruct",
            fields: [{ name: "field1", type: { builtIn: "string" }, optional: false }],
          },
        ],
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
      expect(deltas).toHaveLength(1);
      expect(deltas[0].entityType).toBe(EntityType.Field);
      expect(deltas[0].changeType).toBe(ChangeType.Added);
      expect(deltas[0].direction).toBe(Direction.ClientHasLess);
      expect(deltas[0].severity).toBe(Severity.Info);
    });

    it("detects added required field with Error severity", () => {
      const clientIDL = {
        interfaces: [],
        structs: [
          {
            name: "TestStruct",
            fields: [{ name: "field1", type: { builtIn: "string" }, optional: false }],
          },
        ],
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
      expect(deltas).toHaveLength(1);
      expect(deltas[0].severity).toBe(Severity.Error);
    });

    it("detects removed field with Info severity", () => {
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
        structs: [
          {
            name: "TestStruct",
            fields: [{ name: "field1", type: { builtIn: "string" }, optional: false }],
          },
        ],
        enums: [],
        errors: [],
      };
      const deltas = diffIDL(clientIDL, serverIDL);
      expect(deltas).toHaveLength(1);
      expect(deltas[0].entityType).toBe(EntityType.Field);
      expect(deltas[0].changeType).toBe(ChangeType.Removed);
      expect(deltas[0].direction).toBe(Direction.ClientHasMore);
      expect(deltas[0].severity).toBe(Severity.Info);
    });

    it("detects field made optional with Info severity", () => {
      const clientIDL = {
        interfaces: [],
        structs: [
          {
            name: "TestStruct",
            fields: [{ name: "field1", type: { builtIn: "string" }, optional: false }],
          },
        ],
        enums: [],
        errors: [],
      };
      const serverIDL = {
        interfaces: [],
        structs: [
          {
            name: "TestStruct",
            fields: [{ name: "field1", type: { builtIn: "string" }, optional: true }],
          },
        ],
        enums: [],
        errors: [],
      };
      const deltas = diffIDL(clientIDL, serverIDL);
      expect(deltas).toHaveLength(1);
      expect(deltas[0].entityType).toBe(EntityType.Field);
      expect(deltas[0].changeType).toBe(ChangeType.Modified);
      expect(deltas[0].direction).toBe(Direction.ClientHasLess);
      expect(deltas[0].severity).toBe(Severity.Info);
      expect(deltas[0].description).toContain("required to optional");
    });

    it("detects field made required with Warning severity", () => {
      const clientIDL = {
        interfaces: [],
        structs: [
          {
            name: "TestStruct",
            fields: [{ name: "field1", type: { builtIn: "string" }, optional: true }],
          },
        ],
        enums: [],
        errors: [],
      };
      const serverIDL = {
        interfaces: [],
        structs: [
          {
            name: "TestStruct",
            fields: [{ name: "field1", type: { builtIn: "string" }, optional: false }],
          },
        ],
        enums: [],
        errors: [],
      };
      const deltas = diffIDL(clientIDL, serverIDL);
      expect(deltas).toHaveLength(1);
      expect(deltas[0].severity).toBe(Severity.Warning);
      expect(deltas[0].description).toContain("optional to required");
    });
  });

  describe("struct changes", () => {
    it("detects struct removed from server with Error severity", () => {
      const clientIDL = {
        interfaces: [],
        structs: [{ name: "TestStruct", fields: [] }],
        enums: [],
        errors: [],
      };
      const serverIDL = {
        interfaces: [],
        structs: [],
        enums: [],
        errors: [],
      };
      const deltas = diffIDL(clientIDL, serverIDL);
      expect(deltas).toHaveLength(1);
      expect(deltas[0].entityType).toBe(EntityType.Struct);
      expect(deltas[0].changeType).toBe(ChangeType.Removed);
      expect(deltas[0].direction).toBe(Direction.ClientHasMore);
      expect(deltas[0].severity).toBe(Severity.Error);
    });
  });

  describe("interface changes", () => {
    it("detects interface added to server with Info severity", () => {
      const clientIDL = {
        interfaces: [],
        structs: [],
        enums: [],
        errors: [],
      };
      const serverIDL = {
        interfaces: [{ name: "TestService", methods: [] }],
        structs: [],
        enums: [],
        errors: [],
      };
      const deltas = diffIDL(clientIDL, serverIDL);
      expect(deltas).toHaveLength(1);
      expect(deltas[0].entityType).toBe(EntityType.Interface);
      expect(deltas[0].changeType).toBe(ChangeType.Added);
      expect(deltas[0].direction).toBe(Direction.ClientHasLess);
      expect(deltas[0].severity).toBe(Severity.Info);
    });
  });
});

describe("classifySeverity", () => {
  it("returns Error for removed struct", () => {
    expect(classifySeverity(EntityType.Struct, ChangeType.Removed, Direction.ClientHasMore)).toBe(
      Severity.Error
    );
  });

  it("returns Info for added struct", () => {
    expect(classifySeverity(EntityType.Struct, ChangeType.Added, Direction.ClientHasLess)).toBe(
      Severity.Info
    );
  });

  it("returns Error for modified field with Mismatch direction", () => {
    expect(classifySeverity(EntityType.Field, ChangeType.Modified, Direction.Mismatch)).toBe(
      Severity.Error
    );
  });

  it("returns Error for added required field", () => {
    expect(classifySeverity(EntityType.Field, ChangeType.Added, Direction.ClientHasLess, "required")).toBe(
      Severity.Error
    );
  });

  it("returns Info for added optional field", () => {
    expect(classifySeverity(EntityType.Field, ChangeType.Added, Direction.ClientHasLess, "optional")).toBe(
      Severity.Info
    );
  });

  it("returns Warning for made_required", () => {
    expect(classifySeverity(EntityType.Field, ChangeType.Modified, Direction.ClientHasLess, "made_required")).toBe(
      Severity.Warning
    );
  });

  it("returns Info for made_optional", () => {
    expect(classifySeverity(EntityType.Field, ChangeType.Modified, Direction.ClientHasLess, "made_optional")).toBe(
      Severity.Info
    );
  });
});
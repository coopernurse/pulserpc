/**
 * Type definitions used across the runtime.
 */

export interface JsonRpcRequest {
  jsonrpc: "2.0";
  method: string;
  params?: any;
  id?: string | number | null;
}

export interface JsonRpcError {
  code: number;
  message: string;
  data?: any;
}

export interface JsonRpcResponse {
  jsonrpc: "2.0";
  result?: any;
  error?: JsonRpcError;
  id?: string | number | null;
}

export interface FieldDef {
  name: string;
  type: any;
  optional?: boolean;
}

export interface StructDef {
  extends?: string;
  fields: FieldDef[];
}

export interface EnumDef {
  values: Array<{ name: string }>;
}

export type StructMap = { [key: string]: StructDef };
export type EnumMap = { [key: string]: EnumDef };

export const EntityType = {
  Interface: "Interface",
  Method: "Method",
  Struct: "Struct",
  Field: "Field",
  Enum: "Enum",
  Error: "Error",
} as const;
export type EntityType = (typeof EntityType)[keyof typeof EntityType];

export const ChangeType = {
  Added: "Added",
  Removed: "Removed",
  Modified: "Modified",
} as const;
export type ChangeType = (typeof ChangeType)[keyof typeof ChangeType];

export const Direction = {
  ClientHasMore: "ClientHasMore",
  ClientHasLess: "ClientHasLess",
  Mismatch: "Mismatch",
} as const;
export type Direction = (typeof Direction)[keyof typeof Direction];

export const Severity = {
  Error: "Error",
  Warning: "Warning",
  Info: "Info",
} as const;
export type Severity = (typeof Severity)[keyof typeof Severity];

export interface ContractDelta {
  entityType: EntityType;
  entityName: string;
  memberName: string;
  changeType: ChangeType;
  direction: Direction;
  severity: Severity;
  description: string;
}

export interface VerificationResult {
  compatible: boolean;
  serverChecksum: string;
  clientChecksum: string;
  deltas: ContractDelta[];
  timestamp: Date;
}

function baseTypeName(name: string): string {
  return name.substring(name.lastIndexOf(".") + 1);
}

interface BaseNameIndex<T> {
  /** First definition for each base name, in the map's key order. */
  byBase: Map<string, T>;
}

/**
 * Cache of base-name indexes, keyed by the types map object itself. Type maps
 * are built once by the Contract parser and then treated as immutable, so the
 * map's identity doubles as the "did the map change" sentinel: a different map
 * object gets a freshly built index. Weak keys avoid retaining discarded
 * contracts.
 */
const baseNameIndexCache = new WeakMap<object, BaseNameIndex<any>>();

function getBaseNameIndex<T>(types: { [key: string]: T }): BaseNameIndex<T> {
  let index = baseNameIndexCache.get(types) as BaseNameIndex<T> | undefined;
  if (index === undefined) {
    // Built lazily, once per map. Object.keys preserves the same order as
    // Object.entries, so resolution stays deterministic: the first definition
    // with a matching base name wins.
    const byBase = new Map<string, T>();
    for (const key of Object.keys(types)) {
      const base = baseTypeName(key);
      if (!byBase.has(base)) {
        byBase.set(base, types[key]);
      }
    }
    index = { byBase };
    baseNameIndexCache.set(types, index);
  }
  return index;
}

function findByTypeName<T>(name: string, types: { [key: string]: T }): T | undefined {
  // Prefer an exact match. Generated idl.json files store imported types under
  // their qualified names ("inc.Status") while type references inside the
  // entry-point namespace are unqualified ("Status"), so fall back to the last
  // name component — the same suffix resolution other runtimes use.
  if (types[name]) {
    return types[name];
  }
  // O(1) after the first lookup: no per-call Object.entries scan/allocation.
  return getBaseNameIndex(types).byBase.get(baseTypeName(name));
}

export function findStruct(structName: string, allStructs: StructMap): StructDef | undefined {
  return findByTypeName(structName, allStructs);
}

export function findEnum(enumName: string, allEnums: EnumMap): EnumDef | undefined {
  return findByTypeName(enumName, allEnums);
}

export function getStructFields(structName: string, allStructs: StructMap): FieldDef[] {
  const structDef = findStruct(structName, allStructs);
  if (!structDef) {
    return [];
  }

  const fields: FieldDef[] = [];

  if (structDef.extends) {
    const parentFields = getStructFields(structDef.extends, allStructs);
    fields.push(...parentFields);
  }

  const fieldNames = new Set(fields.map((f) => f.name));
  for (const field of structDef.fields) {
    if (!fieldNames.has(field.name)) {
      fields.push(field);
      fieldNames.add(field.name);
    } else {
      const index = fields.findIndex((f) => f.name === field.name);
      if (index !== -1) {
        fields[index] = field;
      }
    }
  }

  return fields;
}

export function extractChecksum(idl: any): string {
  if (idl && typeof idl === "object" && "checksum" in idl) {
    return String(idl.checksum);
  }
  return "";
}

export interface ValidationError {
  path: string;
  message: string;
}

export interface ValidationResult {
  valid: boolean;
  error?: string;
  invalidFields?: string[];
}

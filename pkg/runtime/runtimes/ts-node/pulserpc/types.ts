/**
 * Helper functions for working with type definitions
 */

export enum EntityType {
  Interface = "Interface",
  Method = "Method",
  Struct = "Struct",
  Field = "Field",
  Enum = "Enum",
  Error = "Error",
}

export enum ChangeType {
  Added = "Added",
  Removed = "Removed",
  Modified = "Modified",
}

export enum Direction {
  ClientHasMore = "ClientHasMore",
  ClientHasLess = "ClientHasLess",
  Mismatch = "Mismatch",
}

export enum Severity {
  Error = "Error",
  Warning = "Warning",
  Info = "Info",
}

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

export interface TypeDef {
  builtIn?: string;
  array?: TypeDef;
  mapValue?: TypeDef;
  userDefined?: string;
}

export interface FieldDef {
  name: string;
  type: TypeDef;
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

function baseTypeName(name: string): string {
  return name.substring(name.lastIndexOf(".") + 1);
}

interface BaseNameIndex<T> {
  /** First definition for each base name, in sorted key order. */
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
    // Built lazily, once per map. Sorted keys make ambiguous base names
    // deterministic and match the Go/Java/Python runtimes (invariant R6).
    const byBase = new Map<string, T>();
    for (const key of Object.keys(types).sort()) {
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

  // Get parent fields first
  if (structDef.extends) {
    const parentFields = getStructFields(structDef.extends, allStructs);
    fields.push(...parentFields);
  }

  // Add child fields (override parent if name conflict)
  const fieldNames = new Set(fields.map((f) => f.name));
  for (const field of structDef.fields) {
    if (!fieldNames.has(field.name)) {
      fields.push(field);
      fieldNames.add(field.name);
    } else {
      // Override parent field
      const index = fields.findIndex((f) => f.name === field.name);
      if (index !== -1) {
        fields[index] = field;
      }
    }
  }

  return fields;
}

export function extractChecksum(idl: any): string {
  if (idl && typeof idl === 'object' && 'checksum' in idl) {
    return String(idl.checksum);
  }
  return '';
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

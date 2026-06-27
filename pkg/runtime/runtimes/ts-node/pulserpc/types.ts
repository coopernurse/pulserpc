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

export function findStruct(structName: string, allStructs: StructMap): StructDef | undefined {
  return allStructs[structName];
}

export function findEnum(enumName: string, allEnums: EnumMap): EnumDef | undefined {
  return allEnums[enumName];
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

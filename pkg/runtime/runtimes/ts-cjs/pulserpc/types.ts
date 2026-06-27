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

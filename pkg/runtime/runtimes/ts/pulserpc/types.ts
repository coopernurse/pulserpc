/**
 * Helper functions for working with type definitions
 */

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

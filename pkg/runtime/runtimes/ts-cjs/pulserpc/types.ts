/**
 * Helper functions for working with type definitions
 */

const EntityType = {
  Interface: "Interface",
  Method: "Method",
  Struct: "Struct",
  Field: "Field",
  Enum: "Enum",
  Error: "Error",
};

const ChangeType = {
  Added: "Added",
  Removed: "Removed",
  Modified: "Modified",
};

const Direction = {
  ClientHasMore: "ClientHasMore",
  ClientHasLess: "ClientHasLess",
  Mismatch: "Mismatch",
};

const Severity = {
  Error: "Error",
  Warning: "Warning",
  Info: "Info",
};

function findStruct(structName, allStructs) {
  return allStructs[structName];
}

function findEnum(enumName, allEnums) {
  return allEnums[enumName];
}

function getStructFields(structName, allStructs) {
  const structDef = findStruct(structName, allStructs);
  if (!structDef) {
    return [];
  }

  const fields = [];

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

function extractChecksum(idl) {
  if (idl && typeof idl === 'object' && 'checksum' in idl) {
    return String(idl.checksum);
  }
  return '';
}

module.exports = {
  EntityType,
  ChangeType,
  Direction,
  Severity,
  findStruct,
  findEnum,
  getStructFields,
  extractChecksum,
};

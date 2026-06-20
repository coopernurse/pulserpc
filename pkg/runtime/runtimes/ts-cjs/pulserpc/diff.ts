/**
 * IDL diff functionality for contract verification
 */

const { EntityType, ChangeType, Direction, Severity } = require("./types");

function diffIDL(clientIDL, serverIDL) {
  const deltas = [];

  const clientInterfaces = extractInterfaces(clientIDL);
  const serverInterfaces = extractInterfaces(serverIDL);
  deltas.push(...diffInterfaces(clientInterfaces, serverInterfaces));

  const clientStructs = extractStructs(clientIDL);
  const serverStructs = extractStructs(serverIDL);
  deltas.push(...diffStructs(clientStructs, serverStructs));

  const clientEnums = extractEnums(clientIDL);
  const serverEnums = extractEnums(serverIDL);
  deltas.push(...diffEnums(clientEnums, serverEnums));

  const clientErrors = extractErrors(clientIDL);
  const serverErrors = extractErrors(serverIDL);
  deltas.push(...diffErrors(clientErrors, serverErrors));

  return deltas;
}

function extractInterfaces(idl) {
  const result = {};
  for (const ifaceData of idl.interfaces || []) {
    const name = ifaceData.name;
    if (name) {
      result[name] = ifaceData;
    }
  }
  return result;
}

function extractStructs(idl) {
  const result = {};
  for (const structData of idl.structs || []) {
    const name = structData.name;
    if (name) {
      result[name] = structData;
    }
  }
  return result;
}

function extractEnums(idl) {
  const result = {};
  for (const enumData of idl.enums || []) {
    const name = enumData.name;
    if (name) {
      result[name] = enumData;
    }
  }
  return result;
}

function extractErrors(idl) {
  const result = {};
  for (const errData of idl.errors || []) {
    const name = errData.name;
    if (name) {
      result[name] = errData;
    }
  }
  return result;
}

function diffInterfaces(client, server) {
  const deltas = [];

  for (const name of Object.keys(client)) {
    if (name in server) {
      deltas.push(...diffInterfaceMethods(name, client[name], server[name]));
    } else {
      deltas.push({
        entityType: EntityType.Interface,
        entityName: name,
        memberName: "",
        changeType: ChangeType.Removed,
        direction: Direction.ClientHasMore,
        severity: Severity.Error,
        description: `Interface '${name}' exists in client but not in server`,
      });
    }
  }

  for (const name of Object.keys(server)) {
    if (!(name in client)) {
      deltas.push({
        entityType: EntityType.Interface,
        entityName: name,
        memberName: "",
        changeType: ChangeType.Added,
        direction: Direction.ClientHasLess,
        severity: Severity.Info,
        description: `Interface '${name}' exists in server but not in client`,
      });
    }
  }

  return deltas;
}

function diffInterfaceMethods(ifaceName, clientIface, serverIface) {
  const deltas = [];
  const clientMethods = extractMethods(clientIface);
  const serverMethods = extractMethods(serverIface);

  for (const name of Object.keys(clientMethods)) {
    if (name in serverMethods) {
      if (!methodsEqual(clientMethods[name], serverMethods[name])) {
        deltas.push({
          entityType: EntityType.Method,
          entityName: ifaceName,
          memberName: name,
          changeType: ChangeType.Modified,
          direction: Direction.Mismatch,
          severity: Severity.Error,
          description: `Method '${name}' in interface '${ifaceName}' has mismatched signatures`,
        });
      }
    } else {
      deltas.push({
        entityType: EntityType.Method,
        entityName: ifaceName,
        memberName: name,
        changeType: ChangeType.Removed,
        direction: Direction.ClientHasMore,
        severity: Severity.Error,
        description: `Method '${name}' in interface '${ifaceName}' exists in client but not in server`,
      });
    }
  }

  for (const name of Object.keys(serverMethods)) {
    if (!(name in clientMethods)) {
      deltas.push({
        entityType: EntityType.Method,
        entityName: ifaceName,
        memberName: name,
        changeType: ChangeType.Added,
        direction: Direction.ClientHasLess,
        severity: Severity.Warning,
        description: `Method '${name}' in interface '${ifaceName}' exists in server but not in client`,
      });
    }
  }

  return deltas;
}

function extractMethods(iface) {
  const result = {};
  for (const method of iface.methods || []) {
    const name = method.name;
    if (name) {
      result[name] = method;
    }
  }
  return result;
}

function methodsEqual(a, b) {
  if (!mapsEqual(a.parameters, b.parameters)) {
    return false;
  }
  if (!mapsEqual(a.returnType, b.returnType)) {
    return false;
  }
  return true;
}

function mapsEqual(a, b) {
  if (a === null && b === null) return true;
  if (a === null || b === null) return false;
  if (typeof a === "object" && typeof b === "object") {
    if (Array.isArray(a) !== Array.isArray(b)) return false;
    if (Array.isArray(a)) {
      if (a.length !== b.length) return false;
      for (let i = 0; i < a.length; i++) {
        if (!mapsEqual(a[i], b[i])) return false;
      }
      return true;
    }
    const keysA = Object.keys(a);
    const keysB = Object.keys(b);
    if (keysA.length !== keysB.length) return false;
    for (const key of keysA) {
      if (!(key in b) || !mapsEqual(a[key], b[key])) return false;
    }
    return true;
  }
  return a === b;
}

function diffStructs(client, server) {
  const deltas = [];

  for (const name of Object.keys(client)) {
    if (name in server) {
      deltas.push(...diffStructFields(name, client[name], server[name]));
    } else {
      deltas.push({
        entityType: EntityType.Struct,
        entityName: name,
        memberName: "",
        changeType: ChangeType.Removed,
        direction: Direction.ClientHasMore,
        severity: Severity.Error,
        description: `Struct '${name}' exists in client but not in server`,
      });
    }
  }

  for (const name of Object.keys(server)) {
    if (!(name in client)) {
      deltas.push({
        entityType: EntityType.Struct,
        entityName: name,
        memberName: "",
        changeType: ChangeType.Added,
        direction: Direction.ClientHasLess,
        severity: Severity.Info,
        description: `Struct '${name}' exists in server but not in client`,
      });
    }
  }

  return deltas;
}

function diffStructFields(structName, clientStruct, serverStruct) {
  const deltas = [];
  const clientFields = extractFields(clientStruct);
  const serverFields = extractFields(serverStruct);

  for (const name of Object.keys(clientFields)) {
    if (name in serverFields) {
      const [typeChanged, optionalityChanged, wasRequired, isRequired] = fieldsEqualDetailed(
        clientFields[name],
        serverFields[name]
      );
      if (typeChanged) {
        deltas.push({
          entityType: EntityType.Field,
          entityName: structName,
          memberName: name,
          changeType: ChangeType.Modified,
          direction: Direction.Mismatch,
          severity: Severity.Error,
          description: `Field '${name}' in struct '${structName}' has changed type`,
        });
      } else if (optionalityChanged) {
        if (wasRequired && !isRequired) {
          deltas.push({
            entityType: EntityType.Field,
            entityName: structName,
            memberName: name,
            changeType: ChangeType.Modified,
            direction: Direction.ClientHasLess,
            severity: Severity.Info,
            description: `Field '${name}' in struct '${structName}' optionality changed from required to optional`,
          });
        } else if (!wasRequired && isRequired) {
          deltas.push({
            entityType: EntityType.Field,
            entityName: structName,
            memberName: name,
            changeType: ChangeType.Modified,
            direction: Direction.ClientHasLess,
            severity: Severity.Warning,
            description: `Field '${name}' in struct '${structName}' optionality changed from optional to required`,
          });
        }
      }
    } else {
      deltas.push({
        entityType: EntityType.Field,
        entityName: structName,
        memberName: name,
        changeType: ChangeType.Removed,
        direction: Direction.ClientHasMore,
        severity: Severity.Info,
        description: `Field '${name}' in struct '${structName}' exists in client but not in server`,
      });
    }
  }

  for (const name of Object.keys(serverFields)) {
    if (!(name in clientFields)) {
      const isRequired = isFieldRequired(serverFields[name]);
      const severity = classifySeverity(
        EntityType.Field,
        ChangeType.Added,
        Direction.ClientHasLess,
        isRequired ? "required" : "optional"
      );
      deltas.push({
        entityType: EntityType.Field,
        entityName: structName,
        memberName: name,
        changeType: ChangeType.Added,
        direction: Direction.ClientHasLess,
        severity,
        description: `Field '${name}' in struct '${structName}' exists in server but not in client`,
      });
    }
  }

  return deltas;
}

function extractFields(structData) {
  const result = {};
  for (const field of structData.fields || []) {
    const name = field.name;
    if (name) {
      result[name] = field;
    }
  }
  return result;
}

function fieldsEqualDetailed(a, b) {
  const typeChanged = !mapsEqual(a.type, b.type);
  const aOptional = getFieldOptional(a);
  const bOptional = getFieldOptional(b);
  const wasRequired = !aOptional;
  const isRequired = !bOptional;
  const optionalityChanged = aOptional !== bOptional;
  return [typeChanged, optionalityChanged, wasRequired, isRequired];
}

function getFieldOptional(field) {
  return field.optional === true;
}

function isFieldRequired(field) {
  return field.optional !== true;
}

function diffEnums(client, server) {
  const deltas = [];

  for (const name of Object.keys(client)) {
    if (name in server) {
      deltas.push(...diffEnumValues(name, client[name], server[name]));
    } else {
      deltas.push({
        entityType: EntityType.Enum,
        entityName: name,
        memberName: "",
        changeType: ChangeType.Removed,
        direction: Direction.ClientHasMore,
        severity: Severity.Warning,
        description: `Enum '${name}' exists in client but not in server`,
      });
    }
  }

  for (const name of Object.keys(server)) {
    if (!(name in client)) {
      deltas.push({
        entityType: EntityType.Enum,
        entityName: name,
        memberName: "",
        changeType: ChangeType.Added,
        direction: Direction.ClientHasLess,
        severity: Severity.Warning,
        description: `Enum '${name}' exists in server but not in client`,
      });
    }
  }

  return deltas;
}

function diffEnumValues(enumName, clientEnum, serverEnum) {
  const deltas = [];
  const clientValues = extractEnumValues(clientEnum);
  const serverValues = extractEnumValues(serverEnum);

  for (const name of Object.keys(clientValues)) {
    if (!(name in serverValues)) {
      deltas.push({
        entityType: EntityType.Enum,
        entityName: enumName,
        memberName: name,
        changeType: ChangeType.Removed,
        direction: Direction.ClientHasMore,
        severity: Severity.Warning,
        description: `Enum value '${name}' in enum '${enumName}' exists in client but not in server`,
      });
    }
  }

  for (const name of Object.keys(serverValues)) {
    if (!(name in clientValues)) {
      deltas.push({
        entityType: EntityType.Enum,
        entityName: enumName,
        memberName: name,
        changeType: ChangeType.Added,
        direction: Direction.ClientHasLess,
        severity: Severity.Warning,
        description: `Enum value '${name}' in enum '${enumName}' exists in server but not in client`,
      });
    }
  }

  return deltas;
}

function extractEnumValues(enumData) {
  const result = {};
  for (const value of enumData.values || []) {
    const name = value.name;
    if (name) {
      result[name] = true;
    }
  }
  return result;
}

function diffErrors(client, server) {
  const deltas = [];

  for (const name of Object.keys(client)) {
    if (!(name in server)) {
      deltas.push({
        entityType: EntityType.Error,
        entityName: name,
        memberName: "",
        changeType: ChangeType.Removed,
        direction: Direction.ClientHasMore,
        severity: Severity.Info,
        description: `Error '${name}' exists in client but not in server`,
      });
    }
  }

  for (const name of Object.keys(server)) {
    if (!(name in client)) {
      deltas.push({
        entityType: EntityType.Error,
        entityName: name,
        memberName: "",
        changeType: ChangeType.Added,
        direction: Direction.ClientHasLess,
        severity: Severity.Info,
        description: `Error '${name}' exists in server but not in client`,
      });
    }
  }

  return deltas;
}

function classifySeverity(entityType, changeType, direction, extra = "") {
  if (entityType === EntityType.Struct) {
    if (changeType === ChangeType.Removed && direction === Direction.ClientHasMore) {
      return Severity.Error;
    }
    if (changeType === ChangeType.Added && direction === Direction.ClientHasLess) {
      return Severity.Info;
    }
  } else if (entityType === EntityType.Field) {
    if (changeType === ChangeType.Modified && direction === Direction.Mismatch) {
      return Severity.Error;
    }
    if (changeType === ChangeType.Removed && direction === Direction.ClientHasMore) {
      return Severity.Info;
    }
    if (changeType === ChangeType.Added && direction === Direction.ClientHasLess) {
      return extra === "required" ? Severity.Error : Severity.Info;
    }
    if (changeType === ChangeType.Modified && direction === Direction.ClientHasLess) {
      if (extra === "made_required") return Severity.Warning;
      if (extra === "made_optional") return Severity.Info;
      return Severity.Info;
    }
  } else if (entityType === EntityType.Method) {
    if (changeType === ChangeType.Removed && direction === Direction.ClientHasMore) {
      return Severity.Error;
    }
    if (changeType === ChangeType.Added && direction === Direction.ClientHasLess) {
      return Severity.Warning;
    }
    if (changeType === ChangeType.Modified && direction === Direction.Mismatch) {
      return Severity.Error;
    }
  } else if (entityType === EntityType.Enum) {
    if (changeType === ChangeType.Removed && direction === Direction.ClientHasMore) {
      return Severity.Warning;
    }
    if (changeType === ChangeType.Added && direction === Direction.ClientHasLess) {
      return Severity.Warning;
    }
  } else if (entityType === EntityType.Error) {
    if (changeType === ChangeType.Removed && direction === Direction.ClientHasMore) {
      return Severity.Info;
    }
    if (changeType === ChangeType.Added && direction === Direction.ClientHasLess) {
      return Severity.Info;
    }
  } else if (entityType === EntityType.Interface) {
    if (changeType === ChangeType.Removed && direction === Direction.ClientHasMore) {
      return Severity.Error;
    }
    if (changeType === ChangeType.Added && direction === Direction.ClientHasLess) {
      return Severity.Info;
    }
  }

  return Severity.Info;
}

module.exports = {
  diffIDL,
  classifySeverity,
  extractChecksum,
};

function extractChecksum(idl) {
  if (idl && typeof idl === 'object' && 'checksum' in idl) {
    return String(idl.checksum);
  }
  return '';
}

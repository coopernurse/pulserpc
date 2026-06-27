/**
 * Validation functions for PulseRPC types
 */

import { findStruct, findEnum, getStructFields, TypeDef, StructMap, EnumMap, ValidationError } from "./types.js";

function joinPath(parent: string, child: string): string {
  return parent ? `${parent}.${child}` : `.${child}`;
}

function arrayIndexPath(parent: string, index: number | string): string {
  return `${parent}[${index}]`;
}

function makeError(path: string, message: string): ValidationError {
  return { path, message };
}

export function validateString(value: any, path: string = ""): ValidationError[] {
  if (typeof value !== "string") {
    return [makeError(path, `Expected string, got ${typeof value}`)];
  }
  return [];
}

export function validateInt(value: any, path: string = ""): ValidationError[] {
  if (typeof value !== "number") {
    return [makeError(path, `Expected number for int, got ${typeof value}`)];
  }
  if (Number.isNaN(value)) {
    return [makeError(path, `Expected integer, got NaN`)];
  }
  if (!Number.isFinite(value)) {
    return [makeError(path, `Expected integer, got infinity`)];
  }
  if (Number.isInteger(value)) {
    return [];
  }
  if (value === Math.floor(value)) {
    return [];
  }
  return [makeError(path, `Expected integer, got number with fractional component: ${value}`)];
}

export function validateFloat(value: any, path: string = ""): ValidationError[] {
  if (typeof value !== "number") {
    return [makeError(path, `Expected number for float, got ${typeof value}`)];
  }
  return [];
}

export function validateBool(value: any, path: string = ""): ValidationError[] {
  if (typeof value !== "boolean") {
    return [makeError(path, `Expected boolean, got ${typeof value}`)];
  }
  return [];
}

export function validateArray(
  value: any,
  elementValidator: (_v: any, _path: string) => ValidationError[],
  path: string = ""
): ValidationError[] {
  if (!Array.isArray(value)) {
    return [makeError(path, `Expected array, got ${typeof value}`)];
  }
  const errors: ValidationError[] = [];
  for (let i = 0; i < value.length; i++) {
    const elementPath = arrayIndexPath(path, i);
    errors.push(...elementValidator(value[i], elementPath));
  }
  return errors;
}

export function validateMap(
  value: any,
  valueValidator: (_v: any, _path: string) => ValidationError[],
  path: string = ""
): ValidationError[] {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return [makeError(path, `Expected object for map, got ${typeof value}`)];
  }
  const errors: ValidationError[] = [];
  for (const [key, val] of Object.entries(value)) {
    if (typeof key !== "string") {
      errors.push(makeError(path, `Map key must be string, got ${typeof key}`));
      continue;
    }
    const keyPath = arrayIndexPath(path, key);
    errors.push(...valueValidator(val, keyPath));
  }
  return errors;
}

export function validateEnum(
  value: any,
  enumName: string,
  allowedValues: string[],
  path: string = ""
): ValidationError[] {
  if (typeof value !== "string") {
    return [makeError(path, `Expected string for enum ${enumName}, got ${typeof value}`)];
  }
  if (!allowedValues.includes(value)) {
    return [makeError(path, `Invalid value for enum ${enumName}: '${value}'. Allowed values: ${allowedValues.join(", ")}`)];
  }
  return [];
}

export function validateStruct(
  value: any,
  structName: string,
  structDef: any,
  allStructs: StructMap,
  allEnums: EnumMap,
  path: string = ""
): ValidationError[] {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return [makeError(path, `Expected object for struct ${structName}, got ${typeof value}`)];
  }

  const fields = getStructFields(structName, allStructs);
  const errors: ValidationError[] = [];

  for (const field of fields) {
    const fieldName = field.name;
    const fieldType = field.type;
    const isOptional = field.optional || false;
    const fieldPath = joinPath(path, fieldName);

    if (!(fieldName in value)) {
      if (!isOptional) {
        errors.push(makeError(fieldPath, `Missing required field '${fieldName}' in struct ${structName}`));
      }
    } else {
      const fieldValue = value[fieldName];
      if (fieldValue === null) {
        if (!isOptional) {
          errors.push(makeError(fieldPath, `Field '${fieldName}' in struct ${structName} cannot be null`));
        }
      } else if (fieldValue === undefined) {
        if (!isOptional) {
          errors.push(makeError(fieldPath, `Field '${fieldName}' in struct ${structName} cannot be undefined`));
        }
      } else {
        errors.push(...validateType(fieldValue, fieldType, allStructs, allEnums, isOptional, fieldPath));
      }
    }
  }

  return errors;
}

export function validateType(
  value: any,
  typeDef: TypeDef,
  allStructs: StructMap,
  allEnums: EnumMap,
  isOptional: boolean = false,
  path: string = ""
): ValidationError[] {
  if (value === null) {
    if (isOptional) {
      return [];
    } else {
      return [makeError(path, "Value cannot be null for non-optional type")];
    }
  }

  if (value === undefined) {
    return [makeError(path, "Value cannot be undefined")];
  }

  if (typeDef.builtIn === "string") {
    return validateString(value, path);
  } else if (typeDef.builtIn === "int") {
    return validateInt(value, path);
  } else if (typeDef.builtIn === "float") {
    return validateFloat(value, path);
  } else if (typeDef.builtIn === "bool") {
    return validateBool(value, path);
  } else if (typeDef.array) {
    const elementType = typeDef.array;
    const elementValidator = (v: any, p: string) =>
      validateType(v, elementType, allStructs, allEnums, false, p);
    return validateArray(value, elementValidator, path);
  } else if (typeDef.mapValue) {
    const valueType = typeDef.mapValue;
    const valueValidator = (v: any, p: string) =>
      validateType(v, valueType, allStructs, allEnums, false, p);
    return validateMap(value, valueValidator, path);
  } else if (typeDef.userDefined) {
    const userType = typeDef.userDefined;
    const structDef = findStruct(userType, allStructs);
    if (structDef) {
      return validateStruct(value, userType, structDef, allStructs, allEnums, path);
    } else {
      const enumDef = findEnum(userType, allEnums);
      if (enumDef) {
        const allowedValues = enumDef.values.map((v: any) => v.name);
        return validateEnum(value, userType, allowedValues, path);
      } else {
        return [makeError(path, `Unknown user-defined type: ${userType}`)];
      }
    }
  } else {
    return [makeError(path, `Invalid type definition: ${JSON.stringify(typeDef)}`)];
  }
}

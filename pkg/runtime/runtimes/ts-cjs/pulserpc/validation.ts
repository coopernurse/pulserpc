/**
 * Validation functions for PulseRPC types.
 */

import { findStruct, findEnum, getStructFields, StructMap, EnumMap, EnumDef, ValidationError } from "./types";

function joinPath(parent: string, child: string): string {
  return parent ? `${parent}.${child}` : `.${child}`;
}

function arrayIndexPath(parent: string, index: number | string): string {
  return `${parent}[${index}]`;
}

function makeError(path: string, message: string): ValidationError {
  return { path, message };
}

interface EnumValueCache {
  /** Names in declaration order; kept for the standalone validator's signature. */
  allowedValues: string[];
  /** O(1) membership check, replacing allowedValues.includes(value). */
  allowedSet: Set<string>;
  /** Error text joined once per enum definition instead of once per failure. */
  allowedText: string;
}

/**
 * Derived enum metadata is cached per definition object. IDL definitions are
 * built once by the Contract parser and then treated as immutable; WeakMap
 * keys avoid retaining discarded contracts.
 */
const enumValueCacheByDef = new WeakMap<object, EnumValueCache>();

function getEnumValueCache(enumDef: EnumDef): EnumValueCache {
  let cache = enumValueCacheByDef.get(enumDef);
  if (cache === undefined) {
    const allowedValues = enumDef.values.map((v: any) => v.name);
    cache = {
      allowedValues,
      allowedSet: new Set(allowedValues),
      allowedText: allowedValues.join(", "),
    };
    enumValueCacheByDef.set(enumDef, cache);
  }
  return cache;
}

export function validateString(value: any, path: string = ""): ValidationError[] {
  if (typeof value !== "string") {
    return [makeError(path, `Expected string, got ${typeof value}`)];
  }
  return [];
}

/**
 * IDL `int` is a signed 64-bit integer on the wire (D1/R25). TypeScript
 * numbers are IEEE-754 doubles and cannot represent the full int64 range
 * exactly, so the validator accepts only integers in JavaScript's safe
 * integer range [-(2^53 - 1), 2^53 - 1] and rejects wider values instead of
 * silently rounding them. This is a TypeScript/JavaScript runtime limitation,
 * not a change to the wire type.
 */
export const INT64_SAFE_MIN = Number.MIN_SAFE_INTEGER;
export const INT64_SAFE_MAX = Number.MAX_SAFE_INTEGER;

/** @deprecated IDL `int` is signed 64-bit; use INT64_SAFE_MIN. */
export const INT32_MIN = -2147483648;
/** @deprecated IDL `int` is signed 64-bit; use INT64_SAFE_MAX. */
export const INT32_MAX = 2147483647;

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
  if (!Number.isInteger(value)) {
    return [makeError(path, `Expected integer, got number with fractional component: ${value}`)];
  }
  if (!Number.isSafeInteger(value)) {
    return [makeError(
      path,
      `Expected signed 64-bit integer within JavaScript's safe integer range ` +
        `[${INT64_SAFE_MIN}, ${INT64_SAFE_MAX}], got ${value}: JavaScript numbers ` +
        `cannot represent the full int64 range exactly`,
    )];
  }
  return [];
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
  elementValidator: (v: any, p: string) => ValidationError[],
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
  valueValidator: (v: any, p: string) => ValidationError[],
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
  path: string = "",
  // Internal fast path used by validateType; standalone callers omit it.
  prepared?: EnumValueCache
): ValidationError[] {
  if (typeof value !== "string") {
    return [makeError(path, `Expected string for enum ${enumName}, got ${typeof value}`)];
  }
  const allowed = prepared ? prepared.allowedSet.has(value) : allowedValues.includes(value);
  if (!allowed) {
    const allowedText = prepared ? prepared.allowedText : allowedValues.join(", ");
    return [makeError(path, `Invalid value for enum ${enumName}: '${value}'. Allowed values: ${allowedText}`)];
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
  typeDef: any,
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
        const cache = getEnumValueCache(enumDef);
        return validateEnum(value, userType, cache.allowedValues, path, cache);
      } else {
        return [makeError(path, `Unknown user-defined type: ${userType}`)];
      }
    }
  } else {
    return [makeError(path, `Invalid type definition: ${JSON.stringify(typeDef)}`)];
  }
}

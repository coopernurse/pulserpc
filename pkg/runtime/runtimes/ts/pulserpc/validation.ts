/**
 * Validation functions for PulseRPC types
 */

import { findStruct, findEnum, getStructFields, TypeDef, StructMap, EnumMap, StructDef } from "./types";

export function validateString(value: any): void {
  if (typeof value !== "string") {
    throw new TypeError(`Expected string, got ${typeof value}`);
  }
}

export function validateInt(value: any): void {
  // Validate that value is a number with no fractional component
  // Values like 5.0 should pass (effectively an integer), but 5.1 should fail
  if (typeof value !== "number") {
    throw new TypeError(`Expected number for int, got ${typeof value}`);
  }
  if (Number.isInteger(value)) {
    return;
  }
  // Accept whole-number floats (e.g., 5.0, -3.0)
  if (value === Math.floor(value)) {
    return;
  }
  throw new TypeError(`Expected integer, got number with fractional component: ${value}`);
}

export function validateFloat(value: any): void {
  if (typeof value !== "number") {
    throw new TypeError(`Expected number for float, got ${typeof value}`);
  }
}

export function validateBool(value: any): void {
  if (typeof value !== "boolean") {
    throw new TypeError(`Expected boolean, got ${typeof value}`);
  }
}

export function validateArray(
  value: any,
  elementValidator: (v: any) => void
): void {
  if (!Array.isArray(value)) {
    throw new TypeError(`Expected array, got ${typeof value}`);
  }
  for (let i = 0; i < value.length; i++) {
    try {
      elementValidator(value[i]);
    } catch (e: any) {
      throw new Error(
        `Array element at index ${i} validation failed: ${e.message}`
      );
    }
  }
}

export function validateMap(
  value: any,
  valueValidator: (v: any) => void
): void {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw new TypeError(`Expected object for map, got ${typeof value}`);
  }
  for (const [key, val] of Object.entries(value)) {
    if (typeof key !== "string") {
      throw new TypeError(`Map key must be string, got ${typeof key}`);
    }
    try {
      valueValidator(val);
    } catch (e: any) {
      throw new Error(
        `Map value for key '${key}' validation failed: ${e.message}`
      );
    }
  }
}

export function validateEnum(
  value: any,
  enumName: string,
  allowedValues: string[]
): void {
  if (typeof value !== "string") {
    throw new TypeError(
      `Expected string for enum ${enumName}, got ${typeof value}`
    );
  }
  if (!allowedValues.includes(value)) {
    throw new Error(
      `Invalid value for enum ${enumName}: '${value}'. Allowed values: ${allowedValues.join(", ")}`
    );
  }
}

export function validateStruct(
  value: any,
  structName: string,
  structDef: any,
  allStructs: StructMap,
  allEnums: EnumMap
): void {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw new TypeError(
      `Expected object for struct ${structName}, got ${typeof value}`
    );
  }

  // Get all fields including parent fields
  const fields = getStructFields(structName, allStructs);

  // Check required fields
  for (const field of fields) {
    const fieldName = field.name;
    const fieldType = field.type;
    const isOptional = field.optional || false;

    if (!(fieldName in value)) {
      if (!isOptional) {
        throw new Error(
          `Missing required field '${fieldName}' in struct ${structName}`
        );
      }
    } else {
      // Field is present, validate it
      const fieldValue = value[fieldName];
      if (fieldValue === null) {
        if (!isOptional) {
          throw new Error(
            `Field '${fieldName}' in struct ${structName} cannot be null`
          );
        }
      } else if (fieldValue === undefined) {
        if (!isOptional) {
          throw new Error(
            `Field '${fieldName}' in struct ${structName} cannot be undefined`
          );
        }
      } else {
        // Create validator for this field type
        try {
          validateType(fieldValue, fieldType, allStructs, allEnums, isOptional);
        } catch (e: any) {
          throw new Error(
            `Field '${fieldName}' in struct ${structName} validation failed: ${e.message}`
          );
        }
      }
    }
  }
}

export function validateType(
  value: any,
  typeDef: TypeDef,
  allStructs: StructMap,
  allEnums: EnumMap,
  isOptional: boolean = false
): void {
  // Handle optional types - only null is valid, not undefined
  if (value === null) {
    if (isOptional) {
      return;
    } else {
      throw new Error("Value cannot be null for non-optional type");
    }
  }

  // undefined is never valid (not even for optional fields)
  if (value === undefined) {
    throw new Error("Value cannot be undefined");
  }

  // Built-in types
  if (typeDef.builtIn === "string") {
    validateString(value);
  } else if (typeDef.builtIn === "int") {
    validateInt(value);
  } else if (typeDef.builtIn === "float") {
    validateFloat(value);
  } else if (typeDef.builtIn === "bool") {
    validateBool(value);
  }
  // Array types
  else if (typeDef.array) {
    const elementType = typeDef.array;
    const elementValidator = (v: any) =>
      validateType(v, elementType, allStructs, allEnums, false);
    validateArray(value, elementValidator);
  }
  // Map types
  else if (typeDef.mapValue) {
    const valueType = typeDef.mapValue;
    const valueValidator = (v: any) =>
      validateType(v, valueType, allStructs, allEnums, false);
    validateMap(value, valueValidator);
  }
  // User-defined types
  else if (typeDef.userDefined) {
    const userType = typeDef.userDefined;
    // Check if it's a struct
    const structDef = findStruct(userType, allStructs);
    if (structDef) {
      validateStruct(value, userType, structDef, allStructs, allEnums);
    }
    // Check if it's an enum
    else {
      const enumDef = findEnum(userType, allEnums);
      if (enumDef) {
        const allowedValues = enumDef.values.map((v) => v.name);
        validateEnum(value, userType, allowedValues);
      } else {
        throw new Error(`Unknown user-defined type: ${userType}`);
      }
    }
  } else {
    throw new Error(`Invalid type definition: ${JSON.stringify(typeDef)}`);
  }
}


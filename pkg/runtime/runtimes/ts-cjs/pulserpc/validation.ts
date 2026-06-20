/**
 * Validation functions for PulseRPC types
 */

const { findStruct, findEnum, getStructFields } = require("./types");

function validateString(value) {
  if (typeof value !== "string") {
    throw new TypeError(`Expected string, got ${typeof value}`);
  }
}

function validateInt(value) {
  if (typeof value !== "number") {
    throw new TypeError(`Expected number for int, got ${typeof value}`);
  }
  if (Number.isInteger(value)) {
    return;
  }
  if (value === Math.floor(value)) {
    return;
  }
  throw new TypeError(`Expected integer, got number with fractional component: ${value}`);
}

function validateFloat(value) {
  if (typeof value !== "number") {
    throw new TypeError(`Expected number for float, got ${typeof value}`);
  }
}

function validateBool(value) {
  if (typeof value !== "boolean") {
    throw new TypeError(`Expected boolean, got ${typeof value}`);
  }
}

function validateArray(value, elementValidator) {
  if (!Array.isArray(value)) {
    throw new TypeError(`Expected array, got ${typeof value}`);
  }
  for (let i = 0; i < value.length; i++) {
    try {
      elementValidator(value[i]);
    } catch (e) {
      throw new Error(
        `Array element at index ${i} validation failed: ${e.message}`
      );
    }
  }
}

function validateMap(value, valueValidator) {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw new TypeError(`Expected object for map, got ${typeof value}`);
  }
  for (const [key, val] of Object.entries(value)) {
    if (typeof key !== "string") {
      throw new TypeError(`Map key must be string, got ${typeof key}`);
    }
    try {
      valueValidator(val);
    } catch (e) {
      throw new Error(
        `Map value for key '${key}' validation failed: ${e.message}`
      );
    }
  }
}

function validateEnum(value, enumName, allowedValues) {
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

function validateStruct(value, structName, structDef, allStructs, allEnums) {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw new TypeError(
      `Expected object for struct ${structName}, got ${typeof value}`
    );
  }

  const fields = getStructFields(structName, allStructs);

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
        try {
          validateType(fieldValue, fieldType, allStructs, allEnums, isOptional);
        } catch (e) {
          throw new Error(
            `Field '${fieldName}' in struct ${structName} validation failed: ${e.message}`
          );
        }
      }
    }
  }
}

function validateType(value, typeDef, allStructs, allEnums, isOptional = false) {
  if (value === null) {
    if (isOptional) {
      return;
    } else {
      throw new Error("Value cannot be null for non-optional type");
    }
  }

  if (value === undefined) {
    throw new Error("Value cannot be undefined");
  }

  if (typeDef.builtIn === "string") {
    validateString(value);
  } else if (typeDef.builtIn === "int") {
    validateInt(value);
  } else if (typeDef.builtIn === "float") {
    validateFloat(value);
  } else if (typeDef.builtIn === "bool") {
    validateBool(value);
  } else if (typeDef.array) {
    const elementType = typeDef.array;
    const elementValidator = (v) =>
      validateType(v, elementType, allStructs, allEnums, false);
    validateArray(value, elementValidator);
  } else if (typeDef.mapValue) {
    const valueType = typeDef.mapValue;
    const valueValidator = (v) =>
      validateType(v, valueType, allStructs, allEnums, false);
    validateMap(value, valueValidator);
  } else if (typeDef.userDefined) {
    const userType = typeDef.userDefined;
    const structDef = findStruct(userType, allStructs);
    if (structDef) {
      validateStruct(value, userType, structDef, allStructs, allEnums);
    } else {
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

module.exports = {
  validateString,
  validateInt,
  validateFloat,
  validateBool,
  validateArray,
  validateMap,
  validateEnum,
  validateStruct,
  validateType,
};

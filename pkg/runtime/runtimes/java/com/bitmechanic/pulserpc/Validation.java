package com.bitmechanic.pulserpc;

import java.util.Map;
import java.util.List;
import java.util.ArrayList;
import java.util.HashMap;
import java.lang.reflect.Array;

/**
 * Validation functions for PulseRPC types
 */
public class Validation {

    /**
     * Validate that value is a string
     */
    public static void validateString(Object value) {
        if (!(value instanceof String)) {
            throw new IllegalArgumentException("Expected string, got " + getTypeName(value));
        }
    }

    /**
     * Validate that value is an int
     */
    public static void validateInt(Object value) {
        if (!(value instanceof Integer)) {
            throw new IllegalArgumentException("Expected int, got " + getTypeName(value));
        }
    }

    /**
     * Validate that value is a float or int
     */
    public static void validateFloat(Object value) {
        if (!(value instanceof Float) && !(value instanceof Double) && !(value instanceof Integer)) {
            throw new IllegalArgumentException("Expected float, got " + getTypeName(value));
        }
    }

    /**
     * Validate that value is a bool
     */
    public static void validateBool(Object value) {
        if (!(value instanceof Boolean)) {
            throw new IllegalArgumentException("Expected bool, got " + getTypeName(value));
        }
    }

    /**
     * Validate that value is an array and each element passes validation
     */
    public static void validateArray(Object value, ValidationCallback elementValidator) {
        if (value == null || value instanceof String || value instanceof Map) {
            throw new IllegalArgumentException("Expected array, got " + getTypeName(value));
        }

        if (!(value instanceof List)) {
            throw new IllegalArgumentException("Expected array, got " + getTypeName(value));
        }

        List<?> list = (List<?>) value;
        for (int i = 0; i < list.size(); i++) {
            try {
                elementValidator.validate(list.get(i));
            } catch (Exception e) {
                throw new IllegalArgumentException("Array element at index " + i + " validation failed: " + e.getMessage(), e);
            }
        }
    }

    /**
     * Validate that value is a map (Map) with string keys and validated values
     */
    public static void validateMap(Object value, ValidationCallback valueValidator) {
        if (!(value instanceof Map)) {
            throw new IllegalArgumentException("Expected map, got " + getTypeName(value));
        }

        Map<?, ?> map = (Map<?, ?>) value;
        for (Map.Entry<?, ?> entry : map.entrySet()) {
            try {
                valueValidator.validate(entry.getValue());
            } catch (Exception e) {
                throw new IllegalArgumentException("Map value for key '" + entry.getKey() + "' validation failed: " + e.getMessage(), e);
            }
        }
    }

    /**
     * Validate that value is a string and matches one of the allowed enum values
     */
    public static void validateEnum(Object value, String enumName, List<String> allowedValues) {
        if (!(value instanceof String)) {
            throw new IllegalArgumentException("Expected string for enum " + enumName + ", got " + getTypeName(value));
        }

        String strValue = (String) value;
        if (!allowedValues.contains(strValue)) {
            throw new IllegalArgumentException("Invalid value for enum " + enumName + ": '" + strValue + "'. Allowed values: " + allowedValues);
        }
    }

    /**
     * Validate that value is a Map matching the struct definition
     */
    public static void validateStruct(Object value, String structName, Map<String, Object> structDef,
                                    Map<String, Map<String, Object>> allStructs,
                                    Map<String, Map<String, Object>> allEnums) {
        if (!(value instanceof Map)) {
            throw new IllegalArgumentException("Expected map for struct " + structName + ", got " + getTypeName(value));
        }

        Map<?, ?> dict = (Map<?, ?>) value;

        // Get all fields including parent fields
        List<Map<String, Object>> fields = Types.getStructFields(structName, allStructs);

        // Check required fields
        for (Map<String, Object> field : fields) {
            String fieldName = (String) field.get("name");
            Map<String, Object> fieldType = (Map<String, Object>) field.get("type");
            boolean isOptional = field.containsKey("optional") && (Boolean) field.get("optional");

            if (!dict.containsKey(fieldName)) {
                if (!isOptional) {
                    throw new IllegalArgumentException("Missing required field '" + fieldName + "' in struct " + structName);
                }
            } else {
                Object fieldValue = dict.get(fieldName);
                if (fieldValue == null) {
                    if (!isOptional) {
                        throw new IllegalArgumentException("Field '" + fieldName + "' in struct " + structName + " cannot be null");
                    }
                } else {
                    try {
                        validateType(fieldValue, fieldType, allStructs, allEnums, false);
                    } catch (Exception e) {
                        throw new IllegalArgumentException("Field '" + fieldName + "' in struct " + structName + " validation failed: " + e.getMessage(), e);
                    }
                }
            }
        }
    }

    /**
     * Validate a value against a type definition
     */
    public static void validateType(Object value, Map<String, Object> typeDef,
                                  Map<String, Map<String, Object>> allStructs,
                                  Map<String, Map<String, Object>> allEnums,
                                  boolean isOptional) {
        // Handle optional types
        if (value == null) {
            if (!isOptional) {
                throw new IllegalArgumentException("Value cannot be null for non-optional type");
            }
            return;
        }

        // Built-in types
        if (typeDef.containsKey("builtIn")) {
            String builtIn = (String) typeDef.get("builtIn");
            switch (builtIn) {
                case "string":
                    validateString(value);
                    break;
                case "int":
                    validateInt(value);
                    break;
                case "float":
                    validateFloat(value);
                    break;
                case "bool":
                    validateBool(value);
                    break;
                default:
                    throw new IllegalArgumentException("Unknown built-in type: " + builtIn);
            }
        }
        // Array types
        else if (typeDef.containsKey("array")) {
            Map<String, Object> elementType = (Map<String, Object>) typeDef.get("array");
            validateArray(value, elem -> validateType(elem, elementType, allStructs, allEnums, false));
        }
        // Map types
        else if (typeDef.containsKey("mapValue")) {
            Map<String, Object> valueType = (Map<String, Object>) typeDef.get("mapValue");
            validateMap(value, val -> validateType(val, valueType, allStructs, allEnums, false));
        }
        // User-defined types
        else if (typeDef.containsKey("userDefined")) {
            String userType = (String) typeDef.get("userDefined");

            // Check if it's a struct
            Map<String, Object> structDef = Types.findStruct(userType, allStructs);
            if (structDef != null) {
                validateStruct(value, userType, structDef, allStructs, allEnums);
            }
            // Check if it's an enum
            else {
                Map<String, Object> enumDef = Types.findEnum(userType, allEnums);
                if (enumDef != null) {
                    if (enumDef.containsKey("values")) {
                        List<?> enumValues = (List<?>) enumDef.get("values");
                        List<String> allowedValues = new ArrayList<>();
                        for (Object enumValue : enumValues) {
                            if (enumValue instanceof Map) {
                                Map<String, Object> valueMap = (Map<String, Object>) enumValue;
                                String name = (String) valueMap.get("name");
                                if (name != null) {
                                    allowedValues.add(name);
                                }
                            }
                        }
                        validateEnum(value, userType, allowedValues);
                    } else {
                        throw new IllegalArgumentException("Invalid enum definition for " + userType + ": missing values");
                    }
                } else {
                    throw new IllegalArgumentException("Unknown user-defined type: " + userType);
                }
            }
        } else {
            throw new IllegalArgumentException("Invalid type definition");
        }
    }

    /**
     * Functional interface for validation callbacks
     */
    @FunctionalInterface
    public interface ValidationCallback {
        void validate(Object value);
    }

    /**
     * Helper method to get a readable type name for error messages
     */
    private static String getTypeName(Object value) {
        if (value == null) {
            return "null";
        }
        return value.getClass().getSimpleName();
    }
}

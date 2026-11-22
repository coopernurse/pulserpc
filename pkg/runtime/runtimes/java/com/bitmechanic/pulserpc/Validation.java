package com.bitmechanic.pulserpc;

import java.util.Map;
import java.util.List;
import java.util.ArrayList;
import java.util.HashMap;
import java.lang.reflect.Array;

/**
 * Validation functions for PulseRPC types.
 * Mirrors the Python validation module: each method returns a list of
 * ValidationError (empty == valid) instead of throwing on the first failure,
 * so callers can accumulate all errors and report them together.
 */
public class Validation {

    public static List<ValidationError> validateString(Object value, String path) {
        List<ValidationError> errs = new ArrayList<>();
        if (!(value instanceof String)) {
            errs.add(new ValidationError(path, "Expected string, got " + getTypeName(value)));
        }
        return errs;
    }

    public static List<ValidationError> validateInt(Object value, String path) {
        List<ValidationError> errs = new ArrayList<>();
        if (value instanceof Integer) {
            return checkIntRange(((Integer) value).longValue(), path, errs);
        }
        if (value instanceof Double) {
            double d = (Double) value;
            if (d == Math.floor(d) && !Double.isInfinite(d) && !Double.isNaN(d)
                    && d >= Integer.MIN_VALUE && d <= Integer.MAX_VALUE) {
                return checkIntRange((long) d, path, errs);
            }
        }
        if (value instanceof Long) {
            long l = (Long) value;
            if (l >= Integer.MIN_VALUE && l <= Integer.MAX_VALUE) {
                return checkIntRange(l, path, errs);
            }
        }
        if (value instanceof Boolean) {
            errs.add(new ValidationError(path, "Expected int, got bool"));
            return errs;
        }
        if (value instanceof Float) {
            // treat like double
            double d = ((Float) value).doubleValue();
            if (d == Math.floor(d) && !Double.isInfinite(d) && !Double.isNaN(d)
                    && d >= Integer.MIN_VALUE && d <= Integer.MAX_VALUE) {
                return checkIntRange((long) d, path, errs);
            }
        }
        errs.add(new ValidationError(path, "Expected int, got " + getTypeName(value)));
        return errs;
    }

    private static List<ValidationError> checkIntRange(long v, String path, List<ValidationError> errs) {
        if (v < Integer.MIN_VALUE || v > Integer.MAX_VALUE) {
            errs.add(new ValidationError(path, "Expected int in signed 32-bit range [" + Integer.MIN_VALUE + ", " + Integer.MAX_VALUE + "], got " + v));
        }
        return errs;
    }

    public static List<ValidationError> validateFloat(Object value, String path) {
        List<ValidationError> errs = new ArrayList<>();
        if (value instanceof Boolean) {
            errs.add(new ValidationError(path, "Expected float, got bool"));
            return errs;
        }
        if (!(value instanceof Number)) {
            errs.add(new ValidationError(path, "Expected float, got " + getTypeName(value)));
        }
        return errs;
    }

    public static List<ValidationError> validateBool(Object value, String path) {
        List<ValidationError> errs = new ArrayList<>();
        if (!(value instanceof Boolean)) {
            errs.add(new ValidationError(path, "Expected bool, got " + getTypeName(value)));
        }
        return errs;
    }

    public static List<ValidationError> validateArray(Object value, ValidationCallback elementValidator, String path) {
        List<ValidationError> errs = new ArrayList<>();
        if (value == null || value instanceof String || value instanceof Map) {
            errs.add(new ValidationError(path, "Expected array, got " + getTypeName(value)));
            return errs;
        }
        if (!(value instanceof List)) {
            errs.add(new ValidationError(path, "Expected array, got " + getTypeName(value)));
            return errs;
        }
        List<?> list = (List<?>) value;
        for (int i = 0; i < list.size(); i++) {
            String elemPath = path + "[" + i + "]";
            List<ValidationError> e = elementValidator.validate(list.get(i), elemPath);
            errs.addAll(e);
        }
        return errs;
    }

    public static List<ValidationError> validateMap(Object value, ValidationCallback valueValidator, String path) {
        List<ValidationError> errs = new ArrayList<>();
        if (!(value instanceof Map)) {
            errs.add(new ValidationError(path, "Expected map, got " + getTypeName(value)));
            return errs;
        }
        Map<?, ?> map = (Map<?, ?>) value;
        for (Map.Entry<?, ?> entry : map.entrySet()) {
            if (!(entry.getKey() instanceof String)) {
                errs.add(new ValidationError(path, "Map key must be string, got " + getTypeName(entry.getKey())));
                continue;
            }
            String keyPath = path + "[" + entry.getKey() + "]";
            List<ValidationError> e = valueValidator.validate(entry.getValue(), keyPath);
            errs.addAll(e);
        }
        return errs;
    }

    public static List<ValidationError> validateEnum(Object value, String enumName, List<String> allowedValues, String path) {
        List<ValidationError> errs = new ArrayList<>();
        if (!(value instanceof String)) {
            errs.add(new ValidationError(path, "Expected string for enum " + enumName + ", got " + getTypeName(value)));
            return errs;
        }
        String strValue = (String) value;
        if (!allowedValues.contains(strValue)) {
            errs.add(new ValidationError(path, "Invalid value for enum " + enumName + ": '" + strValue + "'. Allowed values: " + allowedValues));
        }
        return errs;
    }

    public static List<ValidationError> validateStruct(Object value, String structName, Map<String, Object> structDef,
                                    Map<String, Map<String, Object>> allStructs,
                                    Map<String, Map<String, Object>> allEnums, String path) {
        List<ValidationError> errs = new ArrayList<>();
        if (!(value instanceof Map)) {
            errs.add(new ValidationError(path, "Expected map for struct " + structName + ", got " + getTypeName(value)));
            return errs;
        }

        Map<?, ?> dict = (Map<?, ?>) value;
        List<Map<String, Object>> fields = Types.getStructFields(structName, allStructs);

        for (Map<String, Object> field : fields) {
            String fieldName = (String) field.get("name");
            Map<String, Object> fieldType = (Map<String, Object>) field.get("type");
            boolean isOptional = field.containsKey("optional") && Boolean.TRUE.equals(field.get("optional"));
            String fieldPath = joinPath(path, fieldName);

            if (!dict.containsKey(fieldName)) {
                if (!isOptional) {
                    errs.add(new ValidationError(fieldPath, "Missing required field '" + fieldName + "' in struct " + structName));
                }
            } else {
                Object fieldValue = dict.get(fieldName);
                if (fieldValue == null) {
                    if (!isOptional) {
                        errs.add(new ValidationError(fieldPath, "Field '" + fieldName + "' in struct " + structName + " cannot be null"));
                    }
                } else {
                    List<ValidationError> sub = validateType(fieldValue, fieldType, allStructs, allEnums, false, fieldPath);
                    errs.addAll(sub);
                }
            }
        }
        return errs;
    }

    private static String joinPath(String parent, String child) {
        if (parent == null || parent.isEmpty()) {
            return "." + child;
        }
        return parent + "." + child;
    }

    public static List<ValidationError> validateType(Object value, Map<String, Object> typeDef,
                                  Map<String, Map<String, Object>> allStructs,
                                  Map<String, Map<String, Object>> allEnums,
                                  boolean isOptional, String path) {
        List<ValidationError> errs = new ArrayList<>();
        if (value == null) {
            if (!isOptional) {
                errs.add(new ValidationError(path, "Value cannot be null for non-optional type"));
            }
            return errs;
        }

        if (typeDef.containsKey("builtIn")) {
            String builtIn = (String) typeDef.get("builtIn");
            switch (builtIn) {
                case "string":
                    return validateString(value, path);
                case "int":
                    return validateInt(value, path);
                case "float":
                    return validateFloat(value, path);
                case "bool":
                    return validateBool(value, path);
                default:
                    errs.add(new ValidationError(path, "Unknown built-in type: " + builtIn));
                    return errs;
            }
        } else if (typeDef.containsKey("array")) {
            Map<String, Object> elementType = (Map<String, Object>) typeDef.get("array");
            ValidationCallback cb = (v, p) -> validateType(v, elementType, allStructs, allEnums, false, p);
            return validateArray(value, cb, path);
        } else if (typeDef.containsKey("mapValue")) {
            Map<String, Object> valueType = (Map<String, Object>) typeDef.get("mapValue");
            ValidationCallback cb = (v, p) -> validateType(v, valueType, allStructs, allEnums, false, p);
            return validateMap(value, cb, path);
        } else if (typeDef.containsKey("userDefined")) {
            String userType = (String) typeDef.get("userDefined");
            Map<String, Object> structDef = Types.findStruct(userType, allStructs);
            if (structDef != null) {
                return validateStruct(value, userType, structDef, allStructs, allEnums, path);
            }
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
                    return validateEnum(value, userType, allowedValues, path);
                } else {
                    errs.add(new ValidationError(path, "Invalid enum definition for " + userType + ": missing values"));
                    return errs;
                }
            } else {
                errs.add(new ValidationError(path, "Unknown user-defined type: " + userType));
                return errs;
            }
        } else {
            errs.add(new ValidationError(path, "Invalid type definition"));
            return errs;
        }
    }

    @FunctionalInterface
    public interface ValidationCallback {
        List<ValidationError> validate(Object value, String path);
    }

    // ---- Backward-compatible throwing overloads (delegate to the accumulating
    //      implementations above). Kept so existing tests and callers that
    //      expect IllegalArgumentException continue to compile and pass.
    public static void validateString(Object value) {
        List<ValidationError> e = validateString(value, "");
        if (!e.isEmpty()) { ValidationError _er = e.get(0); String _m = _er.path != null && !_er.path.isEmpty() ? _er.path + ": " + _er.message : _er.message; throw new IllegalArgumentException(_m); }
    }
    public static void validateInt(Object value) {
        List<ValidationError> e = validateInt(value, "");
        if (!e.isEmpty()) { ValidationError _er = e.get(0); String _m = _er.path != null && !_er.path.isEmpty() ? _er.path + ": " + _er.message : _er.message; throw new IllegalArgumentException(_m); }
    }
    public static void validateFloat(Object value) {
        List<ValidationError> e = validateFloat(value, "");
        if (!e.isEmpty()) { ValidationError _er = e.get(0); String _m = _er.path != null && !_er.path.isEmpty() ? _er.path + ": " + _er.message : _er.message; throw new IllegalArgumentException(_m); }
    }
    public static void validateBool(Object value) {
        List<ValidationError> e = validateBool(value, "");
        if (!e.isEmpty()) { ValidationError _er = e.get(0); String _m = _er.path != null && !_er.path.isEmpty() ? _er.path + ": " + _er.message : _er.message; throw new IllegalArgumentException(_m); }
    }
    public static void validateEnum(Object value, String enumName, List<String> allowedValues) {
        List<ValidationError> e = validateEnum(value, enumName, allowedValues, "");
        if (!e.isEmpty()) { ValidationError _er = e.get(0); String _m = _er.path != null && !_er.path.isEmpty() ? _er.path + ": " + _er.message : _er.message; throw new IllegalArgumentException(_m); }
    }
    public static void validateArray(Object value, ValidationCallback elementValidator) {
        ValidationCallback cb = (v, path) -> elementValidator.validate(v, path);
        List<ValidationError> e = validateArray(value, cb, "");
        if (!e.isEmpty()) { ValidationError _er = e.get(0); String _m = _er.path != null && !_er.path.isEmpty() ? _er.path + ": " + _er.message : _er.message; throw new IllegalArgumentException(_m); }
    }
    public static void validateMap(Object value, ValidationCallback valueValidator) {
        ValidationCallback cb = (v, path) -> valueValidator.validate(v, path);
        List<ValidationError> e = validateMap(value, cb, "");
        if (!e.isEmpty()) { ValidationError _er = e.get(0); String _m = _er.path != null && !_er.path.isEmpty() ? _er.path + ": " + _er.message : _er.message; throw new IllegalArgumentException(_m); }
    }
    public static void validateStruct(Object value, String structName, Map<String, Object> structDef,
            Map<String, Map<String, Object>> allStructs, Map<String, Map<String, Object>> allEnums) {
        List<ValidationError> e = validateStruct(value, structName, structDef, allStructs, allEnums, "");
        if (!e.isEmpty()) { ValidationError _er = e.get(0); String _m = _er.path != null && !_er.path.isEmpty() ? _er.path + ": " + _er.message : _er.message; throw new IllegalArgumentException(_m); }
    }
    public static void validateType(Object value, Map<String, Object> typeDef,
            Map<String, Map<String, Object>> allStructs, Map<String, Map<String, Object>> allEnums,
            boolean isOptional) {
        List<ValidationError> e = validateType(value, typeDef, allStructs, allEnums, isOptional, "");
        if (!e.isEmpty()) { ValidationError _er = e.get(0); String _m = _er.path != null && !_er.path.isEmpty() ? _er.path + ": " + _er.message : _er.message; throw new IllegalArgumentException(_m); }
    }

    private static String getTypeName(Object value) {
        if (value == null) {
            return "null";
        }
        return value.getClass().getSimpleName();
    }
}
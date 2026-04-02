package com.bitmechanic.pulserpc;

import java.util.Map;
import java.util.List;
import java.util.ArrayList;
import java.util.HashMap;

/**
 * Helper methods for PulseRPC type operations
 */
public class Types {

    /**
     * Find a struct definition by name
     */
    public static Map<String, Object> findStruct(String name, Map<String, Map<String, Object>> allStructs) {
        return allStructs.get(name);
    }

    /**
     * Find an enum definition by name
     */
    public static Map<String, Object> findEnum(String name, Map<String, Map<String, Object>> allEnums) {
        return allEnums.get(name);
    }

    /**
     * Get all fields for a struct, including inherited fields from parent structs
     */
    public static List<Map<String, Object>> getStructFields(String structName, Map<String, Map<String, Object>> allStructs) {
        List<Map<String, Object>> allFields = new ArrayList<>();
        String currentStruct = structName;

        // Walk up the inheritance chain
        while (currentStruct != null) {
            Map<String, Object> structDef = allStructs.get(currentStruct);
            if (structDef == null) {
                break;
            }

            // Add fields from this struct (if any)
            if (structDef.containsKey("fields")) {
                List<?> fields = (List<?>) structDef.get("fields");
                for (Object fieldObj : fields) {
                    if (fieldObj instanceof Map) {
                        @SuppressWarnings("unchecked")
                        Map<String, Object> field = (Map<String, Object>) fieldObj;
                        allFields.add(0, field); // Add at beginning to maintain inheritance order
                    }
                }
            }

            // Move to parent struct
            currentStruct = (String) structDef.get("extends");
        }

        return allFields;
    }

    /**
     * Get all struct definitions as a map suitable for validation
     */
    public static Map<String, Map<String, Object>> getAllStructs(List<Map<String, Object>> structs) {
        Map<String, Map<String, Object>> result = new HashMap<>();
        for (Map<String, Object> struct : structs) {
            String name = (String) struct.get("name");
            if (name != null) {
                result.put(name, struct);
            }
        }
        return result;
    }

    /**
     * Get all enum definitions as a map suitable for validation
     */
    public static Map<String, Map<String, Object>> getAllEnums(List<Map<String, Object>> enums) {
        Map<String, Map<String, Object>> result = new HashMap<>();
        for (Map<String, Object> enumDef : enums) {
            String name = (String) enumDef.get("name");
            if (name != null) {
                result.put(name, enumDef);
            }
        }
        return result;
    }
}

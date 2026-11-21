package com.bitmechanic.pulserpc;

import java.util.Map;
import java.util.List;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.Collections;
import java.util.WeakHashMap;

/**
 * Helper methods for PulseRPC type operations
 */
public class Types {

    /**
     * Sorted key lists cached by the definitions map. Definition maps are
     * created once per contract but looked up many times during validation, so
     * rebuilding and sorting the key list on every lookup was pure overhead.
     * Weak keys avoid pinning contracts in memory, and the size guard keeps a
     * mutated map correct (resolution depends on keys, not values).
     */
    private static final Map<Map<String, ?>, List<String>> SORTED_KEYS =
        Collections.synchronizedMap(new WeakHashMap<>());

    /**
     * Return the deterministically sorted keys for a definitions map, computing
     * and caching them on first use. Package-private so Contract can warm the
     * cache at construction and share the same resolution order.
     */
    static List<String> sortedKeys(Map<String, ?> definitions) {
        List<String> cached = SORTED_KEYS.get(definitions);
        if (cached == null || cached.size() != definitions.size()) {
            List<String> keys = new ArrayList<>(definitions.keySet());
            Collections.sort(keys);
            SORTED_KEYS.put(definitions, keys);
            cached = keys;
        }
        return cached;
    }

    /**
     * Find a struct definition by name. Resolves exact names first, then a
     * qualified name for an unqualified request ("Response" -&gt; "inc.Response"),
     * then a base-name match for a qualified request ("inc.Response" -&gt;
     * "Response"). Ties are broken deterministically by sorting the keys.
     */
    public static Map<String, Object> findStruct(String name, Map<String, Map<String, Object>> allStructs) {
        return resolve(name, allStructs);
    }

    /**
     * Find an enum definition by name (same resolution rules as findStruct).
     */
    public static Map<String, Object> findEnum(String name, Map<String, Map<String, Object>> allEnums) {
        return resolve(name, allEnums);
    }

    private static Map<String, Object> resolve(String name, Map<String, Map<String, Object>> definitions) {
        if (name == null || definitions == null) {
            return null;
        }
        Map<String, Object> exact = definitions.get(name);
        if (exact != null) {
            return exact;
        }

        // Cached, deterministically sorted keys (same tie-breaking as before).
        List<String> keys = sortedKeys(definitions);

        // Requested name is unqualified but the definition is qualified.
        for (String key : keys) {
            if (key.endsWith("." + name)) {
                return definitions.get(key);
            }
        }

        // Requested name is qualified but the definition is not (or has a
        // different namespace): compare base names.
        String base = name.substring(name.lastIndexOf('.') + 1);
        for (String key : keys) {
            if (key.substring(key.lastIndexOf('.') + 1).equals(base)) {
                return definitions.get(key);
            }
        }
        return null;
    }

    /**
     * Get all fields for a struct, including inherited fields from parent
     * structs. Parent fields come first; a child redeclaration of the same field
     * name overrides the parent definition (java-06).
     */
    public static List<Map<String, Object>> getStructFields(String structName, Map<String, Map<String, Object>> allStructs) {
        // Collect the inheritance chain from child to parent.
        List<Map<String, Object>> chain = new ArrayList<>();
        List<String> visited = new ArrayList<>();
        String currentStruct = structName;
        while (currentStruct != null && !visited.contains(currentStruct)) {
            visited.add(currentStruct);
            Map<String, Object> structDef = findStruct(currentStruct, allStructs);
            if (structDef == null) {
                break;
            }
            chain.add(structDef);
            currentStruct = (String) structDef.get("extends");
        }

        // Walk parent -> child so child fields replace parent fields of the
        // same name. LinkedHashMap keeps the parent's insertion position.
        Map<String, Map<String, Object>> byName = new LinkedHashMap<>();
        for (int i = chain.size() - 1; i >= 0; i--) {
            Map<String, Object> structDef = chain.get(i);
            if (structDef.containsKey("fields")) {
                List<?> fields = (List<?>) structDef.get("fields");
                for (Object fieldObj : fields) {
                    if (fieldObj instanceof Map) {
                        @SuppressWarnings("unchecked")
                        Map<String, Object> field = (Map<String, Object>) fieldObj;
                        Object fieldName = field.get("name");
                        if (fieldName != null) {
                            byName.put(fieldName.toString(), field);
                        }
                    }
                }
            }
        }
        return new ArrayList<>(byName.values());
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

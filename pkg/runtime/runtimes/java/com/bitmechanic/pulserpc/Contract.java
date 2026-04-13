package com.bitmechanic.pulserpc;

import java.util.Map;
import java.util.List;
import java.util.ArrayList;
import java.util.HashMap;

/**
 * Represents a parsed IDL contract
 */
public class Contract {
    public Object idlParsed;
    public Map<String, Interface> interfaces = new HashMap<>();
    public Map<String, Map<String, Object>> structs = new HashMap<>();
    public Map<String, Map<String, Object>> enums = new HashMap<>();
    public Map<String, Object> meta = new HashMap<>();

    /**
     * Creates a new Contract from parsed IDL data.
     * Supports both PulseRPC format (dict with interfaces, structs, enums keys)
     * and Barrister format (list of items with type field)
     */
    public Contract(Object idlParsed) {
        this.idlParsed = idlParsed;

        // Handle both barrister format (list) and PulseRPC format (dict)
        if (idlParsed instanceof Map) {
            // PulseRPC format - dict with interfaces, structs, enums keys
            @SuppressWarnings("unchecked")
            Map<String, Object> dict = (Map<String, Object>) idlParsed;

            // Parse interfaces
            if (dict.containsKey("interfaces") && dict.get("interfaces") instanceof List) {
                List<?> interfacesList = (List<?>) dict.get("interfaces");
                for (Object ifaceData : interfacesList) {
                    if (ifaceData instanceof Map) {
                        @SuppressWarnings("unchecked")
                        Map<String, Object> ifaceDict = (Map<String, Object>) ifaceData;
                        Interface iface = new Interface();
                        iface.name = (String) ifaceDict.get("name");

                        if (ifaceDict.containsKey("methods") && ifaceDict.get("methods") instanceof List) {
                            List<?> methodsList = (List<?>) ifaceDict.get("methods");
                            for (Object method : methodsList) {
                                if (method instanceof Map) {
                                    @SuppressWarnings("unchecked")
                                    Map<String, Object> methodDict = (Map<String, Object>) method;
                                    String funcName = (String) methodDict.get("name");
                                    iface.functions.put(funcName, new FunctionDef(methodDict));
                                }
                            }
                        }
                        interfaces.put(iface.name, iface);
                    }
                }
            }

            // Parse structs
            if (dict.containsKey("structs") && dict.get("structs") instanceof List) {
                List<?> structsList = (List<?>) dict.get("structs");
                for (Object structData : structsList) {
                    if (structData instanceof Map) {
                        @SuppressWarnings("unchecked")
                        Map<String, Object> structDict = (Map<String, Object>) structData;
                        String name = (String) structDict.get("name");
                        structs.put(name, structDict);
                    }
                }
            }

            // Parse enums
            if (dict.containsKey("enums") && dict.get("enums") instanceof List) {
                List<?> enumsList = (List<?>) dict.get("enums");
                for (Object enumData : enumsList) {
                    if (enumData instanceof Map) {
                        @SuppressWarnings("unchecked")
                        Map<String, Object> enumDict = (Map<String, Object>) enumData;
                        String name = (String) enumDict.get("name");
                        enums.put(name, enumDict);
                    }
                }
            }
        } else if (idlParsed instanceof List) {
            // Barrister format - list of items with type field
            @SuppressWarnings("unchecked")
            List<?> list = (List<?>) idlParsed;
            for (Object item : list) {
                if (item instanceof Map) {
                    @SuppressWarnings("unchecked")
                    Map<String, Object> itemDict = (Map<String, Object>) item;
                    String itemType = (String) itemDict.get("type");

                    if ("struct".equals(itemType)) {
                        String name = (String) itemDict.get("name");
                        structs.put(name, itemDict);
                    } else if ("enum".equals(itemType)) {
                        String name = (String) itemDict.get("name");
                        enums.put(name, itemDict);
                    } else if ("interface".equals(itemType)) {
                        Interface iface = new Interface();
                        iface.name = (String) itemDict.get("name");

                        if (itemDict.containsKey("methods") && itemDict.get("methods") instanceof List) {
                            List<?> methodsList = (List<?>) itemDict.get("methods");
                            for (Object method : methodsList) {
                                if (method instanceof Map) {
                                    @SuppressWarnings("unchecked")
                                    Map<String, Object> methodDict = (Map<String, Object>) method;
                                    String funcName = (String) methodDict.get("name");
                                    iface.functions.put(funcName, new FunctionDef(methodDict));
                                }
                            }
                        }
                        interfaces.put(iface.name, iface);
                    } else if ("meta".equals(itemType)) {
                        // Copy metadata
                        for (Map.Entry<String, Object> entry : itemDict.entrySet()) {
                            if (!"type".equals(entry.getKey())) {
                                meta.put(entry.getKey(), entry.getValue());
                            }
                        }
                    }
                }
            }
        }
    }

    /**
     * Check if an interface exists in the contract
     */
    public boolean hasInterface(String ifaceName) {
        return interfaces.containsKey(ifaceName);
    }

    /**
     * Get an interface by name
     */
    public Interface getInterface(String ifaceName) {
        return interfaces.get(ifaceName);
    }

    public Object getIdlParsed() {
        return idlParsed;
    }

    /**
     * Validate request parameters against the IDL
     */
    public void validateRequest(String ifaceName, String funcName, List<Object> params) {
        Interface iface = getInterface(ifaceName);
        if (iface == null) {
            throw new IllegalArgumentException("Unknown interface: '" + ifaceName + "'");
        }

        FunctionDef fn = iface.getFunction(funcName);
        if (fn == null) {
            throw new IllegalArgumentException(ifaceName + ": Unknown function: '" + funcName + "'");
        }

        // Get parameter definitions
        List<?> paramDefs = fn.containsKey("parameters") && fn.get("parameters") instanceof List
            ? (List<?>) fn.get("parameters")
            : new ArrayList<>();

        // Check parameter count
        if (params.size() != paramDefs.size()) {
            throw new IllegalArgumentException("Function '" + ifaceName + "." + funcName + "' expects " +
                paramDefs.size() + " param(s), " + params.size() + " given");
        }

        // Validate each parameter
        for (int i = 0; i < params.size(); i++) {
            Object paramValue = params.get(i);
            @SuppressWarnings("unchecked")
            Map<String, Object> paramDef = paramDefs.get(i) instanceof Map ? (Map<String, Object>) paramDefs.get(i) : null;
            if (paramDef == null) continue;

            String paramName = (String) paramDef.get("name");
            @SuppressWarnings("unchecked")
            Map<String, Object> paramType = paramDef.get("type") instanceof Map ? (Map<String, Object>) paramDef.get("type") : new HashMap<>();
            boolean isOptional = paramDef.containsKey("optional") && (Boolean) paramDef.get("optional");

            try {
                Validation.validateType(paramValue, paramType, structs, enums, isOptional);
            } catch (Exception e) {
                throw new IllegalArgumentException("Function '" + ifaceName + "." + funcName +
                    "' invalid param '" + paramName + "': " + e.getMessage(), e);
            }
        }
    }

    /**
     * Validate response result against the IDL
     */
    public void validateResponse(String ifaceName, String funcName, Object result) {
        Interface iface = getInterface(ifaceName);
        if (iface == null) {
            throw new IllegalArgumentException("Unknown interface: '" + ifaceName + "'");
        }

        FunctionDef fn = iface.getFunction(funcName);
        if (fn == null) {
            throw new IllegalArgumentException(ifaceName + ": Unknown function: '" + funcName + "'");
        }

        // Check if function has a return type
        if (!fn.containsKey("returnType") || !(fn.get("returnType") instanceof Map)) {
            // Function returns void/null
            if (result != null) {
                throw new IllegalArgumentException("Function '" + ifaceName + "." + funcName +
                    "' invalid response: expected null, got " + result);
            }
            return;
        }

        @SuppressWarnings("unchecked")
        Map<String, Object> returnType = (Map<String, Object>) fn.get("returnType");
        boolean isOptional = fn.containsKey("returnOptional") && (Boolean) fn.get("returnOptional");

        try {
            Validation.validateType(result, returnType, structs, enums, isOptional);
        } catch (Exception e) {
            throw new IllegalArgumentException("Function '" + ifaceName + "." + funcName +
                "' invalid response: " + e.getMessage(), e);
        }
    }
}
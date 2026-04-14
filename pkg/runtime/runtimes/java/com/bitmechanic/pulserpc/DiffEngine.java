package com.bitmechanic.pulserpc;

import java.util.*;

public class DiffEngine {
    public static List<ContractDelta> diffIDL(Object clientIDL, Object serverIDL) {
        List<ContractDelta> deltas = new ArrayList<>();

        Map<String, Map<String, Object>> clientInterfaces = extractInterfaces(clientIDL);
        Map<String, Map<String, Object>> serverInterfaces = extractInterfaces(serverIDL);
        deltas.addAll(diffInterfaces(clientInterfaces, serverInterfaces));

        Map<String, Map<String, Object>> clientStructs = extractStructs(clientIDL);
        Map<String, Map<String, Object>> serverStructs = extractStructs(serverIDL);
        deltas.addAll(diffStructs(clientStructs, serverStructs));

        Map<String, Map<String, Object>> clientEnums = extractEnums(clientIDL);
        Map<String, Map<String, Object>> serverEnums = extractEnums(serverIDL);
        deltas.addAll(diffEnums(clientEnums, serverEnums));

        Map<String, Map<String, Object>> clientErrors = extractErrors(clientIDL);
        Map<String, Map<String, Object>> serverErrors = extractErrors(serverIDL);
        deltas.addAll(diffErrors(clientErrors, serverErrors));

        return deltas;
    }

    private static Map<String, Map<String, Object>> extractInterfaces(Object idl) {
        Map<String, Map<String, Object>> result = new HashMap<>();
        if (idl instanceof Map) {
            Map dict = (Map) idl;
            Object interfacesObj = dict.get("interfaces");
            if (interfacesObj instanceof List) {
                List interfaces = (List) interfacesObj;
                for (Object ifaceData : interfaces) {
                    if (ifaceData instanceof Map) {
                        Map ifaceDict = (Map) ifaceData;
                        Object nameObj = ifaceDict.get("name");
                        if (nameObj instanceof String) {
                            String name = (String) nameObj;
                            result.put(name, ifaceDict);
                        }
                    }
                }
            }
        }
        return result;
    }

    private static Map<String, Map<String, Object>> extractStructs(Object idl) {
        Map<String, Map<String, Object>> result = new HashMap<>();
        if (idl instanceof Map) {
            Map dict = (Map) idl;
            Object structsObj = dict.get("structs");
            if (structsObj instanceof List) {
                List structs = (List) structsObj;
                for (Object structData : structs) {
                    if (structData instanceof Map) {
                        Map structDict = (Map) structData;
                        Object nameObj = structDict.get("name");
                        if (nameObj instanceof String) {
                            String name = (String) nameObj;
                            result.put(name, structDict);
                        }
                    }
                }
            }
        }
        return result;
    }

    private static Map<String, Map<String, Object>> extractEnums(Object idl) {
        Map<String, Map<String, Object>> result = new HashMap<>();
        if (idl instanceof Map) {
            Map dict = (Map) idl;
            Object enumsObj = dict.get("enums");
            if (enumsObj instanceof List) {
                List enums = (List) enumsObj;
                for (Object enumData : enums) {
                    if (enumData instanceof Map) {
                        Map enumDict = (Map) enumData;
                        Object nameObj = enumDict.get("name");
                        if (nameObj instanceof String) {
                            String name = (String) nameObj;
                            result.put(name, enumDict);
                        }
                    }
                }
            }
        }
        return result;
    }

    private static Map<String, Map<String, Object>> extractErrors(Object idl) {
        Map<String, Map<String, Object>> result = new HashMap<>();
        if (idl instanceof Map) {
            Map dict = (Map) idl;
            Object errorsObj = dict.get("errors");
            if (errorsObj instanceof List) {
                List errors = (List) errorsObj;
                for (Object errData : errors) {
                    if (errData instanceof Map) {
                        Map errDict = (Map) errData;
                        Object nameObj = errDict.get("name");
                        if (nameObj instanceof String) {
                            String name = (String) nameObj;
                            result.put(name, errDict);
                        }
                    }
                }
            }
        }
        return result;
    }

    private static List<ContractDelta> diffInterfaces(Map<String, Map<String, Object>> client, Map<String, Map<String, Object>> server) {
        List<ContractDelta> deltas = new ArrayList<>();

        for (Map.Entry<String, Map<String, Object>> entry : client.entrySet()) {
            String name = entry.getKey();
            Map<String, Object> clientIface = entry.getValue();
            if (server.containsKey(name)) {
                deltas.addAll(diffInterfaceMethods(name, clientIface, server.get(name)));
            } else {
                deltas.add(new ContractDelta(
                    ContractDelta.EntityType.Interface,
                    name,
                    "",
                    ContractDelta.ChangeType.Removed,
                    ContractDelta.Direction.ClientHasMore,
                    classifySeverity(ContractDelta.EntityType.Interface, ContractDelta.ChangeType.Removed, ContractDelta.Direction.ClientHasMore),
                    String.format("Interface '%s' exists in client but not in server", name)
                ));
            }
        }

        for (String name : server.keySet()) {
            if (!client.containsKey(name)) {
                deltas.add(new ContractDelta(
                    ContractDelta.EntityType.Interface,
                    name,
                    "",
                    ContractDelta.ChangeType.Added,
                    ContractDelta.Direction.ClientHasLess,
                    classifySeverity(ContractDelta.EntityType.Interface, ContractDelta.ChangeType.Added, ContractDelta.Direction.ClientHasLess),
                    String.format("Interface '%s' exists in server but not in client", name)
                ));
            }
        }

        return deltas;
    }

    private static List<ContractDelta> diffInterfaceMethods(String ifaceName, Map<String, Object> clientIface, Map<String, Object> serverIface) {
        List<ContractDelta> deltas = new ArrayList<>();
        Map<String, Map<String, Object>> clientMethods = extractMethods(clientIface);
        Map<String, Map<String, Object>> serverMethods = extractMethods(serverIface);

        for (Map.Entry<String, Map<String, Object>> entry : clientMethods.entrySet()) {
            String name = entry.getKey();
            Map<String, Object> clientMethod = entry.getValue();
            if (serverMethods.containsKey(name)) {
                if (!methodsEqual(clientMethod, serverMethods.get(name))) {
                    deltas.add(new ContractDelta(
                        ContractDelta.EntityType.Method,
                        ifaceName,
                        name,
                        ContractDelta.ChangeType.Modified,
                        ContractDelta.Direction.Mismatch,
                        classifySeverity(ContractDelta.EntityType.Method, ContractDelta.ChangeType.Modified, ContractDelta.Direction.Mismatch),
                        String.format("Method '%s' in interface '%s' has mismatched signatures", name, ifaceName)
                    ));
                }
            } else {
                deltas.add(new ContractDelta(
                    ContractDelta.EntityType.Method,
                    ifaceName,
                    name,
                    ContractDelta.ChangeType.Removed,
                    ContractDelta.Direction.ClientHasMore,
                    classifySeverity(ContractDelta.EntityType.Method, ContractDelta.ChangeType.Removed, ContractDelta.Direction.ClientHasMore),
                    String.format("Method '%s' in interface '%s' exists in client but not in server", name, ifaceName)
                ));
            }
        }

        for (String name : serverMethods.keySet()) {
            if (!clientMethods.containsKey(name)) {
                deltas.add(new ContractDelta(
                    ContractDelta.EntityType.Method,
                    ifaceName,
                    name,
                    ContractDelta.ChangeType.Added,
                    ContractDelta.Direction.ClientHasLess,
                    classifySeverity(ContractDelta.EntityType.Method, ContractDelta.ChangeType.Added, ContractDelta.Direction.ClientHasLess),
                    String.format("Method '%s' in interface '%s' exists in server but not in client", name, ifaceName)
                ));
            }
        }

        return deltas;
    }

    private static Map<String, Map<String, Object>> extractMethods(Map<String, Object> iface) {
        Map<String, Map<String, Object>> result = new HashMap<>();
        Object methodsObj = iface.get("methods");
        if (methodsObj instanceof List) {
            List methods = (List) methodsObj;
            for (Object method : methods) {
                if (method instanceof Map) {
                    Map methodDict = (Map) method;
                    Object nameObj = methodDict.get("name");
                    if (nameObj instanceof String) {
                        String name = (String) nameObj;
                        result.put(name, methodDict);
                    }
                }
            }
        }
        return result;
    }

    private static boolean methodsEqual(Object a, Object b) {
        if (!(a instanceof Map) || !(b instanceof Map)) {
            return a.equals(b);
        }

        Map aMap = (Map) a;
        Map bMap = (Map) b;

        Object aParams = aMap.get("parameters");
        Object bParams = bMap.get("parameters");
        if (!mapsEqual(aParams, bParams)) {
            return false;
        }

        Object aReturn = aMap.get("returnType");
        Object bReturn = bMap.get("returnType");
        if (!mapsEqual(aReturn, bReturn)) {
            return false;
        }

        return true;
    }

    private static boolean mapsEqual(Object a, Object b) {
        if (a == null && b == null) return true;
        if (a == null || b == null) return false;

        if (!(a instanceof Map) || !(b instanceof Map)) {
            return interfaceEqual(a, b);
        }

        Map aMap = (Map) a;
        Map bMap = (Map) b;

        if (aMap.size() != bMap.size()) return false;

        for (Object key : aMap.keySet()) {
            if (!bMap.containsKey(key) || !mapsEqual(aMap.get(key), bMap.get(key))) {
                return false;
            }
        }
        return true;
    }

    private static boolean interfaceEqual(Object a, Object b) {
        if (a instanceof List && b instanceof List) {
            List aList = (List) a;
            List bList = (List) b;
            if (aList.size() != bList.size()) return false;
            for (int i = 0; i < aList.size(); i++) {
                if (!interfaceEqual(aList.get(i), bList.get(i))) return false;
            }
            return true;
        }
        if (a instanceof Map && b instanceof Map) {
            return mapsEqual(a, b);
        }
        return a.equals(b);
    }

    private static List<ContractDelta> diffStructs(Map<String, Map<String, Object>> client, Map<String, Map<String, Object>> server) {
        List<ContractDelta> deltas = new ArrayList<>();

        for (Map.Entry<String, Map<String, Object>> entry : client.entrySet()) {
            String name = entry.getKey();
            Map<String, Object> clientStruct = entry.getValue();
            if (server.containsKey(name)) {
                deltas.addAll(diffStructFields(name, clientStruct, server.get(name)));
            } else {
                deltas.add(new ContractDelta(
                    ContractDelta.EntityType.Struct,
                    name,
                    "",
                    ContractDelta.ChangeType.Removed,
                    ContractDelta.Direction.ClientHasMore,
                    classifySeverity(ContractDelta.EntityType.Struct, ContractDelta.ChangeType.Removed, ContractDelta.Direction.ClientHasMore),
                    String.format("Struct '%s' exists in client but not in server", name)
                ));
            }
        }

        for (String name : server.keySet()) {
            if (!client.containsKey(name)) {
                deltas.add(new ContractDelta(
                    ContractDelta.EntityType.Struct,
                    name,
                    "",
                    ContractDelta.ChangeType.Added,
                    ContractDelta.Direction.ClientHasLess,
                    classifySeverity(ContractDelta.EntityType.Struct, ContractDelta.ChangeType.Added, ContractDelta.Direction.ClientHasLess),
                    String.format("Struct '%s' exists in server but not in client", name)
                ));
            }
        }

        return deltas;
    }

    private static List<ContractDelta> diffStructFields(String structName, Map<String, Object> clientStruct, Map<String, Object> serverStruct) {
        List<ContractDelta> deltas = new ArrayList<>();
        Map<String, Map<String, Object>> clientFields = extractFields(clientStruct);
        Map<String, Map<String, Object>> serverFields = extractFields(serverStruct);

        for (Map.Entry<String, Map<String, Object>> entry : clientFields.entrySet()) {
            String name = entry.getKey();
            Map<String, Object> clientField = entry.getValue();
            if (serverFields.containsKey(name)) {
                boolean[] typeChanged = new boolean[1];
                boolean[] optionalityChanged = new boolean[1];
                boolean[] wasRequired = new boolean[1];
                boolean[] isRequired = new boolean[1];
                fieldsEqualDetailed(clientField, serverFields.get(name), typeChanged, optionalityChanged, wasRequired, isRequired);
                if (typeChanged[0]) {
                    deltas.add(new ContractDelta(
                        ContractDelta.EntityType.Field,
                        structName,
                        name,
                        ContractDelta.ChangeType.Modified,
                        ContractDelta.Direction.Mismatch,
                        classifySeverity(ContractDelta.EntityType.Field, ContractDelta.ChangeType.Modified, ContractDelta.Direction.Mismatch),
                        String.format("Field '%s' in struct '%s' has changed type", name, structName)
                    ));
                } else if (optionalityChanged[0]) {
                    if (wasRequired[0] && !isRequired[0]) {
                        deltas.add(new ContractDelta(
                            ContractDelta.EntityType.Field,
                            structName,
                            name,
                            ContractDelta.ChangeType.Modified,
                            ContractDelta.Direction.ClientHasLess,
                            classifySeverity(ContractDelta.EntityType.Field, ContractDelta.ChangeType.Modified, ContractDelta.Direction.ClientHasLess, "made_optional"),
                            String.format("Field '%s' in struct '%s' optionality changed from required to optional", name, structName)
                        ));
                    } else if (!wasRequired[0] && isRequired[0]) {
                        deltas.add(new ContractDelta(
                            ContractDelta.EntityType.Field,
                            structName,
                            name,
                            ContractDelta.ChangeType.Modified,
                            ContractDelta.Direction.ClientHasLess,
                            classifySeverity(ContractDelta.EntityType.Field, ContractDelta.ChangeType.Modified, ContractDelta.Direction.ClientHasLess, "made_required"),
                            String.format("Field '%s' in struct '%s' optionality changed from optional to required", name, structName)
                        ));
                    }
                }
            } else {
                deltas.add(new ContractDelta(
                    ContractDelta.EntityType.Field,
                    structName,
                    name,
                    ContractDelta.ChangeType.Removed,
                    ContractDelta.Direction.ClientHasMore,
                    classifySeverity(ContractDelta.EntityType.Field, ContractDelta.ChangeType.Removed, ContractDelta.Direction.ClientHasMore),
                    String.format("Field '%s' in struct '%s' exists in client but not in server", name, structName)
                ));
            }
        }

        for (Map.Entry<String, Map<String, Object>> entry : serverFields.entrySet()) {
            String name = entry.getKey();
            if (!clientFields.containsKey(name)) {
                boolean isRequired = false;
                Map<String, Object> field = entry.getValue();
                Object opt = field.get("optional");
                if (opt instanceof Boolean && !((Boolean) opt)) {
                    isRequired = true;
                }
                String extra = isRequired ? "required" : "optional";
                deltas.add(new ContractDelta(
                    ContractDelta.EntityType.Field,
                    structName,
                    name,
                    ContractDelta.ChangeType.Added,
                    ContractDelta.Direction.ClientHasLess,
                    classifySeverity(ContractDelta.EntityType.Field, ContractDelta.ChangeType.Added, ContractDelta.Direction.ClientHasLess, extra),
                    String.format("Field '%s' in struct '%s' exists in server but not in client", name, structName)
                ));
            }
        }

        return deltas;
    }

    private static Map<String, Map<String, Object>> extractFields(Map<String, Object> structData) {
        Map<String, Map<String, Object>> result = new HashMap<>();
        Object fieldsObj = structData.get("fields");
        if (fieldsObj instanceof List) {
            List fields = (List) fieldsObj;
            for (Object field : fields) {
                if (field instanceof Map) {
                    Map fieldMap = (Map) field;
                    Object nameObj = fieldMap.get("name");
                    if (nameObj instanceof String) {
                        String name = (String) nameObj;
                        result.put(name, fieldMap);
                    }
                }
            }
        }
        return result;
    }

    private static boolean fieldsEqual(Map<String, Object> a, Map<String, Object> b) {
        boolean[] typeChanged = new boolean[1];
        fieldsEqualDetailed(a, b, typeChanged, new boolean[1], new boolean[1], new boolean[1]);
        return !typeChanged[0];
    }

    private static void fieldsEqualDetailed(Object a, Object b, boolean[] typeChanged, boolean[] optionalityChanged, boolean[] wasRequired, boolean[] isRequired) {
        if (a == null && b == null) return;
        if (!(a instanceof Map) || !(b instanceof Map)) {
            typeChanged[0] = !Objects.equals(a, b);
            return;
        }

        Map aMap = (Map) a;
        Map bMap = (Map) b;

        Object aType = aMap.get("type");
        Object bType = bMap.get("type");
        typeChanged[0] = !mapsEqual(aType, bType);

        boolean aOptional = getFieldOptional(aMap);
        boolean bOptional = getFieldOptional(bMap);
        wasRequired[0] = !aOptional;
        isRequired[0] = !bOptional;
        optionalityChanged[0] = aOptional != bOptional;
    }

    private static boolean getFieldOptional(Map<String, Object> fieldMap) {
        Object opt = fieldMap.get("optional");
        if (opt instanceof Boolean) {
            return (Boolean) opt;
        }
        return false;
    }

    private static List<ContractDelta> diffEnums(Map<String, Map<String, Object>> client, Map<String, Map<String, Object>> server) {
        List<ContractDelta> deltas = new ArrayList<>();

        for (Map.Entry<String, Map<String, Object>> entry : client.entrySet()) {
            String name = entry.getKey();
            Map<String, Object> clientEnum = entry.getValue();
            if (server.containsKey(name)) {
                deltas.addAll(diffEnumValues(name, clientEnum, server.get(name)));
            } else {
                deltas.add(new ContractDelta(
                    ContractDelta.EntityType.Enum,
                    name,
                    "",
                    ContractDelta.ChangeType.Removed,
                    ContractDelta.Direction.ClientHasMore,
                    classifySeverity(ContractDelta.EntityType.Enum, ContractDelta.ChangeType.Removed, ContractDelta.Direction.ClientHasMore),
                    String.format("Enum '%s' exists in client but not in server", name)
                ));
            }
        }

        for (String name : server.keySet()) {
            if (!client.containsKey(name)) {
                deltas.add(new ContractDelta(
                    ContractDelta.EntityType.Enum,
                    name,
                    "",
                    ContractDelta.ChangeType.Added,
                    ContractDelta.Direction.ClientHasLess,
                    classifySeverity(ContractDelta.EntityType.Enum, ContractDelta.ChangeType.Added, ContractDelta.Direction.ClientHasLess),
                    String.format("Enum '%s' exists in server but not in client", name)
                ));
            }
        }

        return deltas;
    }

    private static List<ContractDelta> diffEnumValues(String enumName, Map<String, Object> clientEnum, Map<String, Object> serverEnum) {
        List<ContractDelta> deltas = new ArrayList<>();
        Set<String> clientValues = extractEnumValues(clientEnum);
        Set<String> serverValues = extractEnumValues(serverEnum);

        for (String name : clientValues) {
            if (!serverValues.contains(name)) {
                deltas.add(new ContractDelta(
                    ContractDelta.EntityType.Enum,
                    enumName,
                    name,
                    ContractDelta.ChangeType.Removed,
                    ContractDelta.Direction.ClientHasMore,
                    classifySeverity(ContractDelta.EntityType.Enum, ContractDelta.ChangeType.Removed, ContractDelta.Direction.ClientHasMore),
                    String.format("Enum value '%s' in enum '%s' exists in client but not in server", name, enumName)
                ));
            }
        }

        for (String name : serverValues) {
            if (!clientValues.contains(name)) {
                deltas.add(new ContractDelta(
                    ContractDelta.EntityType.Enum,
                    enumName,
                    name,
                    ContractDelta.ChangeType.Added,
                    ContractDelta.Direction.ClientHasLess,
                    classifySeverity(ContractDelta.EntityType.Enum, ContractDelta.ChangeType.Added, ContractDelta.Direction.ClientHasLess),
                    String.format("Enum value '%s' in enum '%s' exists in server but not in client", name, enumName)
                ));
            }
        }

        return deltas;
    }

    private static Set<String> extractEnumValues(Map<String, Object> enumData) {
        Set<String> result = new HashSet<>();
        Object valuesObj = enumData.get("values");
        if (valuesObj instanceof List) {
            List values = (List) valuesObj;
            for (Object value : values) {
                if (value instanceof Map) {
                    Map valueMap = (Map) value;
                    Object nameObj = valueMap.get("name");
                    if (nameObj instanceof String) {
                        String name = (String) nameObj;
                        result.add(name);
                    }
                }
            }
        }
        return result;
    }

    private static List<ContractDelta> diffErrors(Map<String, Map<String, Object>> client, Map<String, Map<String, Object>> server) {
        List<ContractDelta> deltas = new ArrayList<>();

        for (String name : client.keySet()) {
            if (!server.containsKey(name)) {
                deltas.add(new ContractDelta(
                    ContractDelta.EntityType.Error,
                    name,
                    "",
                    ContractDelta.ChangeType.Removed,
                    ContractDelta.Direction.ClientHasMore,
                    classifySeverity(ContractDelta.EntityType.Error, ContractDelta.ChangeType.Removed, ContractDelta.Direction.ClientHasMore),
                    String.format("Error '%s' exists in client but not in server", name)
                ));
            }
        }

        for (String name : server.keySet()) {
            if (!client.containsKey(name)) {
                deltas.add(new ContractDelta(
                    ContractDelta.EntityType.Error,
                    name,
                    "",
                    ContractDelta.ChangeType.Added,
                    ContractDelta.Direction.ClientHasLess,
                    classifySeverity(ContractDelta.EntityType.Error, ContractDelta.ChangeType.Added, ContractDelta.Direction.ClientHasLess),
                    String.format("Error '%s' exists in server but not in client", name)
                ));
            }
        }

        return deltas;
    }

    public static ContractDelta.Severity classifySeverity(ContractDelta.EntityType entityType, ContractDelta.ChangeType changeType, ContractDelta.Direction direction, String... extra) {
        switch (entityType) {
            case Struct:
                if (changeType == ContractDelta.ChangeType.Removed && direction == ContractDelta.Direction.ClientHasMore)
                    return ContractDelta.Severity.Error;
                if (changeType == ContractDelta.ChangeType.Added && direction == ContractDelta.Direction.ClientHasLess)
                    return ContractDelta.Severity.Info;
                break;

            case Field:
                if (changeType == ContractDelta.ChangeType.Modified && direction == ContractDelta.Direction.Mismatch)
                    return ContractDelta.Severity.Error;
                if (changeType == ContractDelta.ChangeType.Removed && direction == ContractDelta.Direction.ClientHasMore)
                    return ContractDelta.Severity.Info;
                if (changeType == ContractDelta.ChangeType.Added && direction == ContractDelta.Direction.ClientHasLess) {
                    if (extra.length > 0 && "required".equals(extra[0]))
                        return ContractDelta.Severity.Error;
                    return ContractDelta.Severity.Info;
                }
                if (changeType == ContractDelta.ChangeType.Modified && direction == ContractDelta.Direction.ClientHasLess) {
                    if (extra.length > 0 && "made_required".equals(extra[0]))
                        return ContractDelta.Severity.Warning;
                    if (extra.length > 0 && "made_optional".equals(extra[0]))
                        return ContractDelta.Severity.Info;
                    return ContractDelta.Severity.Info;
                }
                break;

            case Method:
                if (changeType == ContractDelta.ChangeType.Removed && direction == ContractDelta.Direction.ClientHasMore)
                    return ContractDelta.Severity.Error;
                if (changeType == ContractDelta.ChangeType.Added && direction == ContractDelta.Direction.ClientHasLess)
                    return ContractDelta.Severity.Warning;
                if (changeType == ContractDelta.ChangeType.Modified && direction == ContractDelta.Direction.Mismatch)
                    return ContractDelta.Severity.Error;
                break;

            case Enum:
                if (changeType == ContractDelta.ChangeType.Removed && direction == ContractDelta.Direction.ClientHasMore)
                    return ContractDelta.Severity.Warning;
                if (changeType == ContractDelta.ChangeType.Added && direction == ContractDelta.Direction.ClientHasLess)
                    return ContractDelta.Severity.Warning;
                break;

            case Error:
                if (changeType == ContractDelta.ChangeType.Removed && direction == ContractDelta.Direction.ClientHasMore)
                    return ContractDelta.Severity.Info;
                if (changeType == ContractDelta.ChangeType.Added && direction == ContractDelta.Direction.ClientHasLess)
                    return ContractDelta.Severity.Info;
                break;

            case Interface:
                if (changeType == ContractDelta.ChangeType.Removed && direction == ContractDelta.Direction.ClientHasMore)
                    return ContractDelta.Severity.Error;
                if (changeType == ContractDelta.ChangeType.Added && direction == ContractDelta.Direction.ClientHasLess)
                    return ContractDelta.Severity.Info;
                break;
        }

        return ContractDelta.Severity.Info;
    }

}
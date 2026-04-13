using System;
using System.Collections.Generic;
using System.Linq;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;

namespace PulseRPC
{
    public static class DiffEngine
    {
        public static List<ContractDelta> DiffIDL(object clientIDL, object serverIDL)
        {
            var deltas = new List<ContractDelta>();

            var clientInterfaces = ExtractInterfaces(clientIDL);
            var serverInterfaces = ExtractInterfaces(serverIDL);
            deltas.AddRange(DiffInterfaces(clientInterfaces, serverInterfaces));

            var clientStructs = ExtractStructs(clientIDL);
            var serverStructs = ExtractStructs(serverIDL);
            deltas.AddRange(DiffStructs(clientStructs, serverStructs));

            var clientEnums = ExtractEnums(clientIDL);
            var serverEnums = ExtractEnums(serverIDL);
            deltas.AddRange(DiffEnums(clientEnums, serverEnums));

            var clientErrors = ExtractErrors(clientIDL);
            var serverErrors = ExtractErrors(serverIDL);
            deltas.AddRange(DiffErrors(clientErrors, serverErrors));

            return deltas;
        }

        private static Dictionary<string, Dictionary<string, object>> ExtractInterfaces(object idl)
        {
            var result = new Dictionary<string, Dictionary<string, object>>();
            if (idl is Dictionary<string, object> dict)
            {
                if (dict.TryGetValue("interfaces", out var interfacesObj))
                {
                    foreach (var ifaceData in EnumerateArray(interfacesObj))
                    {
                        if (ifaceData is Dictionary<string, object> ifaceDict)
                        {
                            if (ifaceDict.TryGetValue("name", out var nameObj) && nameObj is string name)
                            {
                                result[name] = ifaceDict;
                            }
                        }
                    }
                }
            }
            return result;
        }

        private static IEnumerable<object> EnumerateArray(object arrayObj)
        {
            if (arrayObj is List<object> list)
                return list;
            if (arrayObj is System.Text.Json.JsonElement jsonElement && jsonElement.ValueKind == System.Text.Json.JsonValueKind.Array)
            {
                var items = new List<object>();
                foreach (var item in jsonElement.EnumerateArray())
                {
                    items.Add(ConvertJsonElement(item));
                }
                return items;
            }
            return Enumerable.Empty<object>();
        }

        private static object ConvertJsonElement(System.Text.Json.JsonElement element)
        {
            switch (element.ValueKind)
            {
                case System.Text.Json.JsonValueKind.Object:
                    var dict = new Dictionary<string, object>();
                    foreach (var prop in element.EnumerateObject())
                    {
                        dict[prop.Name] = ConvertJsonElement(prop.Value);
                    }
                    return dict;
                case System.Text.Json.JsonValueKind.Array:
                    var list = new List<object>();
                    foreach (var item in element.EnumerateArray())
                    {
                        list.Add(ConvertJsonElement(item));
                    }
                    return list;
                case System.Text.Json.JsonValueKind.String:
                    return element.GetString() ?? "";
                case System.Text.Json.JsonValueKind.Number:
                    return element.GetDouble();
                case System.Text.Json.JsonValueKind.True:
                    return true;
                case System.Text.Json.JsonValueKind.False:
                    return false;
                default:
                    return null;
            }
        }

        private static Dictionary<string, Dictionary<string, object>> ExtractStructs(object idl)
        {
            var result = new Dictionary<string, Dictionary<string, object>>();
            if (idl is Dictionary<string, object> dict)
            {
                if (dict.TryGetValue("structs", out var structsObj))
                {
                    foreach (var structData in EnumerateArray(structsObj))
                    {
                        if (structData is Dictionary<string, object> structDict)
                        {
                            if (structDict.TryGetValue("name", out var nameObj) && nameObj is string name)
                            {
                                result[name] = structDict;
                            }
                        }
                    }
                }
            }
            return result;
        }

        private static Dictionary<string, Dictionary<string, object>> ExtractEnums(object idl)
        {
            var result = new Dictionary<string, Dictionary<string, object>>();
            if (idl is Dictionary<string, object> dict)
            {
                if (dict.TryGetValue("enums", out var enumsObj))
                {
                    foreach (var enumData in EnumerateArray(enumsObj))
                    {
                        if (enumData is Dictionary<string, object> enumDict)
                        {
                            if (enumDict.TryGetValue("name", out var nameObj) && nameObj is string name)
                            {
                                result[name] = enumDict;
                            }
                        }
                    }
                }
            }
            return result;
        }

        private static Dictionary<string, Dictionary<string, object>> ExtractErrors(object idl)
        {
            var result = new Dictionary<string, Dictionary<string, object>>();
            if (idl is Dictionary<string, object> dict)
            {
                if (dict.TryGetValue("errors", out var errorsObj))
                {
                    foreach (var errData in EnumerateArray(errorsObj))
                    {
                        if (errData is Dictionary<string, object> errDict)
                        {
                            if (errDict.TryGetValue("name", out var nameObj) && nameObj is string name)
                            {
                                result[name] = errDict;
                            }
                        }
                    }
                }
            }
            return result;
        }

        private static List<ContractDelta> DiffInterfaces(Dictionary<string, Dictionary<string, object>> client, Dictionary<string, Dictionary<string, object>> server)
        {
            var deltas = new List<ContractDelta>();

            foreach (var kvp in client)
            {
                var name = kvp.Key;
                var clientIface = kvp.Value;
                if (server.TryGetValue(name, out var serverIface))
                {
                    deltas.AddRange(DiffInterfaceMethods(name, clientIface, serverIface));
                }
                else
                {
                    deltas.Add(new ContractDelta(
                        EntityType.Interface,
                        name,
                        string.Empty,
                        ChangeType.Removed,
                        Direction.ClientHasMore,
                        ClassifySeverity(EntityType.Interface, ChangeType.Removed, Direction.ClientHasMore),
                        $"Interface '{name}' exists in client but not in server"
                    ));
                }
            }

            foreach (var name in server.Keys)
            {
                if (!client.ContainsKey(name))
                {
                    deltas.Add(new ContractDelta(
                        EntityType.Interface,
                        name,
                        string.Empty,
                        ChangeType.Added,
                        Direction.ClientHasLess,
                        ClassifySeverity(EntityType.Interface, ChangeType.Added, Direction.ClientHasLess),
                        $"Interface '{name}' exists in server but not in client"
                    ));
                }
            }

            return deltas;
        }

        private static List<ContractDelta> DiffInterfaceMethods(string ifaceName, Dictionary<string, object> clientIface, Dictionary<string, object> serverIface)
        {
            var deltas = new List<ContractDelta>();
            var clientMethods = ExtractMethods(clientIface);
            var serverMethods = ExtractMethods(serverIface);

            foreach (var kvp in clientMethods)
            {
                var name = kvp.Key;
                var clientMethod = kvp.Value;
                if (serverMethods.TryGetValue(name, out var serverMethod))
                {
                    if (!MethodsEqual(clientMethod, serverMethod))
                    {
                        deltas.Add(new ContractDelta(
                            EntityType.Method,
                            ifaceName,
                            name,
                            ChangeType.Modified,
                            Direction.Mismatch,
                            ClassifySeverity(EntityType.Method, ChangeType.Modified, Direction.Mismatch),
                            $"Method '{name}' in interface '{ifaceName}' has mismatched signatures"
                        ));
                    }
                }
                else
                {
                    deltas.Add(new ContractDelta(
                        EntityType.Method,
                        ifaceName,
                        name,
                        ChangeType.Removed,
                        Direction.ClientHasMore,
                        ClassifySeverity(EntityType.Method, ChangeType.Removed, Direction.ClientHasMore),
                        $"Method '{name}' in interface '{ifaceName}' exists in client but not in server"
                    ));
                }
            }

            foreach (var name in serverMethods.Keys)
            {
                if (!clientMethods.ContainsKey(name))
                {
                    deltas.Add(new ContractDelta(
                        EntityType.Method,
                        ifaceName,
                        name,
                        ChangeType.Added,
                        Direction.ClientHasLess,
                        ClassifySeverity(EntityType.Method, ChangeType.Added, Direction.ClientHasLess),
                        $"Method '{name}' in interface '{ifaceName}' exists in server but not in client"
                    ));
                }
            }

            return deltas;
        }

        private static Dictionary<string, Dictionary<string, object>> ExtractMethods(Dictionary<string, object> iface)
        {
            var result = new Dictionary<string, Dictionary<string, object>>();
            if (iface.TryGetValue("methods", out var methodsObj))
            {
                foreach (var method in EnumerateArray(methodsObj))
                {
                    if (method is Dictionary<string, object> methodDict)
                    {
                        if (methodDict.TryGetValue("name", out var nameObj) && nameObj is string name)
                        {
                            result[name] = methodDict;
                        }
                    }
                }
            }
            return result;
        }

        private static bool MethodsEqual(object a, object b)
        {
            if (a is not Dictionary<string, object> aMap || b is not Dictionary<string, object> bMap)
            {
                return a == b;
            }

            aMap.TryGetValue("parameters", out var aParams);
            bMap.TryGetValue("parameters", out var bParams);
            if (!MapsEqual(aParams, bParams))
            {
                return false;
            }

            aMap.TryGetValue("returnType", out var aReturn);
            bMap.TryGetValue("returnType", out var bReturn);
            if (!MapsEqual(aReturn, bReturn))
            {
                return false;
            }

            return true;
        }

        private static bool MapsEqual(object? a, object? b)
        {
            if (a == null && b == null) return true;
            if (a == null || b == null) return false;

            if (a is not Dictionary<string, object> aMap || b is not Dictionary<string, object> bMap)
            {
                return InterfaceEqual(a, b);
            }

            if (aMap.Count != bMap.Count) return false;

            foreach (var kvp in aMap)
            {
                if (!bMap.TryGetValue(kvp.Key, out var bValue) || !MapsEqual(kvp.Value, bValue))
                {
                    return false;
                }
            }

            return true;
        }

        private static bool InterfaceEqual(object a, object b)
        {
            switch (a)
            {
                case List<object> aList:
                    if (b is not List<object> bList) return false;
                    if (aList.Count != bList.Count) return false;
                    for (int i = 0; i < aList.Count; i++)
                    {
                        if (!InterfaceEqual(aList[i], bList[i])) return false;
                    }
                    return true;
                case Dictionary<string, object> aDict:
                    return MapsEqual(aDict, b);
                default:
                    return a.Equals(b);
            }
        }

        private static List<ContractDelta> DiffStructs(Dictionary<string, Dictionary<string, object>> client, Dictionary<string, Dictionary<string, object>> server)
        {
            var deltas = new List<ContractDelta>();

            foreach (var kvp in client)
            {
                var name = kvp.Key;
                var clientStruct = kvp.Value;
                if (server.TryGetValue(name, out var serverStruct))
                {
                    deltas.AddRange(DiffStructFields(name, clientStruct, serverStruct));
                }
                else
                {
                    deltas.Add(new ContractDelta(
                        EntityType.Struct,
                        name,
                        string.Empty,
                        ChangeType.Removed,
                        Direction.ClientHasMore,
                        ClassifySeverity(EntityType.Struct, ChangeType.Removed, Direction.ClientHasMore),
                        $"Struct '{name}' exists in client but not in server"
                    ));
                }
            }

            foreach (var name in server.Keys)
            {
                if (!client.ContainsKey(name))
                {
                    deltas.Add(new ContractDelta(
                        EntityType.Struct,
                        name,
                        string.Empty,
                        ChangeType.Added,
                        Direction.ClientHasLess,
                        ClassifySeverity(EntityType.Struct, ChangeType.Added, Direction.ClientHasLess),
                        $"Struct '{name}' exists in server but not in client"
                    ));
                }
            }

            return deltas;
        }

        private static List<ContractDelta> DiffStructFields(string structName, Dictionary<string, object> clientStruct, Dictionary<string, object> serverStruct)
        {
            var deltas = new List<ContractDelta>();
            var clientFields = ExtractFields(clientStruct);
            var serverFields = ExtractFields(serverStruct);

            foreach (var kvp in clientFields)
            {
                var name = kvp.Key;
                var clientField = kvp.Value;
                if (serverFields.TryGetValue(name, out var serverField))
                {
                    var (typeChanged, optionalityChanged, wasRequired, isRequired) = FieldsEqualDetailed(clientField, serverField);
                    if (typeChanged)
                    {
                        deltas.Add(new ContractDelta(
                            EntityType.Field,
                            structName,
                            name,
                            ChangeType.Modified,
                            Direction.Mismatch,
                            ClassifySeverity(EntityType.Field, ChangeType.Modified, Direction.Mismatch),
                            $"Field '{name}' in struct '{structName}' has changed type"
                        ));
                    }
                    else if (optionalityChanged)
                    {
                        if (wasRequired && !isRequired)
                        {
                            deltas.Add(new ContractDelta(
                                EntityType.Field,
                                structName,
                                name,
                                ChangeType.Modified,
                                Direction.ClientHasLess,
                                ClassifySeverity(EntityType.Field, ChangeType.Modified, Direction.ClientHasLess, "made_optional"),
                                $"Field '{name}' in struct '{structName}' optionality changed from required to optional"
                            ));
                        }
                        else if (!wasRequired && isRequired)
                        {
                            deltas.Add(new ContractDelta(
                                EntityType.Field,
                                structName,
                                name,
                                ChangeType.Modified,
                                Direction.ClientHasLess,
                                ClassifySeverity(EntityType.Field, ChangeType.Modified, Direction.ClientHasLess, "made_required"),
                                $"Field '{name}' in struct '{structName}' optionality changed from optional to required"
                            ));
                        }
                    }
                }
                else
                {
                    deltas.Add(new ContractDelta(
                        EntityType.Field,
                        structName,
                        name,
                        ChangeType.Removed,
                        Direction.ClientHasMore,
                        ClassifySeverity(EntityType.Field, ChangeType.Removed, Direction.ClientHasMore),
                        $"Field '{name}' in struct '{structName}' exists in client but not in server"
                    ));
                }
            }

            foreach (var kvp in serverFields)
            {
                var name = kvp.Key;
                if (!clientFields.ContainsKey(name))
                {
                    var isRequired = false;
                    if (kvp.Value is Dictionary<string, object> optDict)
                    {
                        if (optDict.TryGetValue("optional", out var optObj) && optObj is bool opt && !opt)
                        {
                            isRequired = true;
                        }
                    }
                    var extra = isRequired ? "required" : "optional";
                    deltas.Add(new ContractDelta(
                        EntityType.Field,
                        structName,
                        name,
                        ChangeType.Added,
                        Direction.ClientHasLess,
                        ClassifySeverity(EntityType.Field, ChangeType.Added, Direction.ClientHasLess, extra),
                        $"Field '{name}' in struct '{structName}' exists in server but not in client"
                    ));
                }
            }

            return deltas;
        }

        private static Dictionary<string, Dictionary<string, object>> ExtractFields(Dictionary<string, object> structData)
        {
            var result = new Dictionary<string, Dictionary<string, object>>();
            if (structData.TryGetValue("fields", out var fieldsObj))
            {
                foreach (var field in EnumerateArray(fieldsObj))
                {
                    if (field is Dictionary<string, object> fieldMap)
                    {
                        if (fieldMap.TryGetValue("name", out var nameObj) && nameObj is string name)
                        {
                            result[name] = fieldMap;
                        }
                    }
                }
            }
            return result;
        }

        private static bool FieldsEqual(object? a, object? b)
        {
            var (typeChanged, _, _, _) = FieldsEqualDetailed(a, b);
            return !typeChanged;
        }

        private static (bool typeChanged, bool optionalityChanged, bool wasRequired, bool isRequired) FieldsEqualDetailed(object? a, object? b)
        {
            if (a == null && b == null) return (false, false, false, false);

            if (a is not Dictionary<string, object> aMap || b is not Dictionary<string, object> bMap)
            {
                return (a != b, false, false, false);
            }

            aMap.TryGetValue("type", out var aType);
            bMap.TryGetValue("type", out var bType);
            var typeChanged = !MapsEqual(aType, bType);

            var aOptional = GetFieldOptional(aMap);
            var bOptional = GetFieldOptional(bMap);
            var wasRequired = !aOptional;
            var isRequired = !bOptional;
            var optionalityChanged = aOptional != bOptional;

            return (typeChanged, optionalityChanged, wasRequired, isRequired);
        }

        private static bool GetFieldOptional(Dictionary<string, object> fieldMap)
        {
            if (fieldMap.TryGetValue("optional", out var optObj) && optObj is bool opt)
            {
                return opt;
            }
            return false;
        }

        private static List<ContractDelta> DiffEnums(Dictionary<string, Dictionary<string, object>> client, Dictionary<string, Dictionary<string, object>> server)
        {
            var deltas = new List<ContractDelta>();

            foreach (var kvp in client)
            {
                var name = kvp.Key;
                var clientEnum = kvp.Value;
                if (server.TryGetValue(name, out var serverEnum))
                {
                    deltas.AddRange(DiffEnumValues(name, clientEnum, serverEnum));
                }
                else
                {
                    deltas.Add(new ContractDelta(
                        EntityType.Enum,
                        name,
                        string.Empty,
                        ChangeType.Removed,
                        Direction.ClientHasMore,
                        ClassifySeverity(EntityType.Enum, ChangeType.Removed, Direction.ClientHasMore),
                        $"Enum '{name}' exists in client but not in server"
                    ));
                }
            }

            foreach (var name in server.Keys)
            {
                if (!client.ContainsKey(name))
                {
                    deltas.Add(new ContractDelta(
                        EntityType.Enum,
                        name,
                        string.Empty,
                        ChangeType.Added,
                        Direction.ClientHasLess,
                        ClassifySeverity(EntityType.Enum, ChangeType.Added, Direction.ClientHasLess),
                        $"Enum '{name}' exists in server but not in client"
                    ));
                }
            }

            return deltas;
        }

        private static List<ContractDelta> DiffEnumValues(string enumName, Dictionary<string, object> clientEnum, Dictionary<string, object> serverEnum)
        {
            var deltas = new List<ContractDelta>();
            var clientValues = ExtractEnumValues(clientEnum);
            var serverValues = ExtractEnumValues(serverEnum);

            foreach (var name in clientValues.Keys)
            {
                if (!serverValues.ContainsKey(name))
                {
                    deltas.Add(new ContractDelta(
                        EntityType.Enum,
                        enumName,
                        name,
                        ChangeType.Removed,
                        Direction.ClientHasMore,
                        ClassifySeverity(EntityType.Enum, ChangeType.Removed, Direction.ClientHasMore),
                        $"Enum value '{name}' in enum '{enumName}' exists in client but not in server"
                    ));
                }
            }

            foreach (var name in serverValues.Keys)
            {
                if (!clientValues.ContainsKey(name))
                {
                    deltas.Add(new ContractDelta(
                        EntityType.Enum,
                        enumName,
                        name,
                        ChangeType.Added,
                        Direction.ClientHasLess,
                        ClassifySeverity(EntityType.Enum, ChangeType.Added, Direction.ClientHasLess),
                        $"Enum value '{name}' in enum '{enumName}' exists in server but not in client"
                    ));
                }
            }

            return deltas;
        }

        private static Dictionary<string, bool> ExtractEnumValues(Dictionary<string, object> enumData)
        {
            var result = new Dictionary<string, bool>();
            if (enumData.TryGetValue("values", out var valuesObj))
            {
                foreach (var value in EnumerateArray(valuesObj))
                {
                    if (value is Dictionary<string, object> valueMap)
                    {
                        if (valueMap.TryGetValue("name", out var nameObj) && nameObj is string name)
                        {
                            result[name] = true;
                        }
                    }
                }
            }
            return result;
        }

        private static List<ContractDelta> DiffErrors(Dictionary<string, Dictionary<string, object>> client, Dictionary<string, Dictionary<string, object>> server)
        {
            var deltas = new List<ContractDelta>();

            foreach (var name in client.Keys)
            {
                if (!server.ContainsKey(name))
                {
                    deltas.Add(new ContractDelta(
                        EntityType.Error,
                        name,
                        string.Empty,
                        ChangeType.Removed,
                        Direction.ClientHasMore,
                        ClassifySeverity(EntityType.Error, ChangeType.Removed, Direction.ClientHasMore),
                        $"Error '{name}' exists in client but not in server"
                    ));
                }
            }

            foreach (var name in server.Keys)
            {
                if (!client.ContainsKey(name))
                {
                    deltas.Add(new ContractDelta(
                        EntityType.Error,
                        name,
                        string.Empty,
                        ChangeType.Added,
                        Direction.ClientHasLess,
                        ClassifySeverity(EntityType.Error, ChangeType.Added, Direction.ClientHasLess),
                        $"Error '{name}' exists in server but not in client"
                    ));
                }
            }

            return deltas;
        }

        public static Severity ClassifySeverity(EntityType entityType, ChangeType changeType, Direction direction, params string[] extra)
        {
            switch (entityType)
            {
                case EntityType.Struct:
                    if (changeType == ChangeType.Removed && direction == Direction.ClientHasMore)
                        return Severity.Error;
                    if (changeType == ChangeType.Added && direction == Direction.ClientHasLess)
                        return Severity.Info;
                    break;

                case EntityType.Field:
                    if (changeType == ChangeType.Modified && direction == Direction.Mismatch)
                        return Severity.Error;
                    if (changeType == ChangeType.Removed && direction == Direction.ClientHasMore)
                        return Severity.Info;
                    if (changeType == ChangeType.Added && direction == Direction.ClientHasLess)
                    {
                        if (extra.Length > 0 && extra[0] == "required")
                            return Severity.Error;
                        return SeverityInfo;
                    }
                    if (changeType == ChangeType.Modified && direction == Direction.ClientHasLess)
                    {
                        if (extra.Length > 0 && extra[0] == "made_required")
                            return Severity.Warning;
                        if (extra.Length > 0 && extra[0] == "made_optional")
                            return Severity.Info;
                        return Severity.Info;
                    }
                    break;

                case EntityType.Method:
                    if (changeType == ChangeType.Removed && direction == Direction.ClientHasMore)
                        return Severity.Error;
                    if (changeType == ChangeType.Added && direction == Direction.ClientHasLess)
                        return Severity.Warning;
                    if (changeType == ChangeType.Modified && direction == Direction.Mismatch)
                        return Severity.Error;
                    break;

                case EntityType.Enum:
                    if (changeType == ChangeType.Removed && direction == Direction.ClientHasMore)
                        return Severity.Warning;
                    if (changeType == ChangeType.Added && direction == Direction.ClientHasLess)
                        return Severity.Warning;
                    break;

                case EntityType.Error:
                    if (changeType == ChangeType.Removed && direction == Direction.ClientHasMore)
                        return Severity.Info;
                    if (changeType == ChangeType.Added && direction == Direction.ClientHasLess)
                        return Severity.Info;
                    break;

                case EntityType.Interface:
                    if (changeType == ChangeType.Removed && direction == Direction.ClientHasMore)
                        return Severity.Error;
                    if (changeType == ChangeType.Added && direction == Direction.ClientHasLess)
                        return Severity.Info;
                    break;
            }

            return Severity.Info;
        }

        private static Severity SeverityInfo => Severity.Info;

        public static string ComputeChecksum(object idl)
        {
            var json = JsonSerializer.Serialize(idl);
            var bytes = SHA256.HashData(Encoding.UTF8.GetBytes(json));
            return Convert.ToHexString(bytes).ToLowerInvariant();
        }
    }
}
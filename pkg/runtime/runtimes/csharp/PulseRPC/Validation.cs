using System;
using System.Collections;
using System.Collections.Generic;
using System.Linq;
using System.Runtime.CompilerServices;

namespace PulseRPC
{
    /// <summary>
    /// Validation functions for PulseRPC types.
    /// Accumulating variants return List&lt;ValidationError&gt; (empty == valid).
    /// </summary>
    public static class Validation
    {
        private static string TypeName(object? value) => value == null ? "null" : value.GetType().Name;

        /// <summary>
        /// Precomputed enum value names: a set for O(1) membership plus the exact
        /// bracketed text used in error messages. Entries are held weakly on the
        /// definition dictionary so contracts are not pinned in memory.
        /// </summary>
        private sealed class EnumInfo
        {
            public readonly HashSet<string> Allowed;
            public readonly string Display;

            public EnumInfo(HashSet<string> allowed, string display)
            {
                Allowed = allowed;
                Display = display;
            }
        }

        private static readonly ConditionalWeakTable<Dictionary<string, object>, EnumInfo> EnumCache = new();

        private static EnumInfo GetEnumInfo(Dictionary<string, object> enumDef)
        {
            return EnumCache.GetValue(enumDef, static def =>
            {
                var allowed = def.TryGetValue("values", out var valuesObj) && valuesObj is IList values
                    ? values.Cast<Dictionary<string, object>>()
                        .Select(v => v.TryGetValue("name", out var n) ? n?.ToString() : null)
                        .Where(n => n != null).Cast<string>().ToList()
                    : new List<string>();
                return new EnumInfo(new HashSet<string>(allowed, StringComparer.Ordinal),
                    $"[{string.Join(", ", allowed)}]");
            });
        }

        private static string JoinPath(string parent, string child)
        {
            if (string.IsNullOrEmpty(parent)) return string.IsNullOrEmpty(child) ? "" : "." + child;
            if (string.IsNullOrEmpty(child)) return parent;
            return parent + "." + child;
        }

        public static List<ValidationError> ValidateString(object? value, string path)
        {
            var errs = new List<ValidationError>();
            if (value is not string)
                errs.Add(new ValidationError(path, $"Expected string, got {TypeName(value)}"));
            return errs;
        }

        public static List<ValidationError> ValidateInt(object? value, string path)
        {
            var errs = new List<ValidationError>();
            if (value is int) return errs;
            if (value is long l && l >= IntMin && l <= IntMax) return errs;
            if (value is double d)
            {
                if (double.IsNaN(d) || double.IsInfinity(d)) { errs.Add(new ValidationError(path, $"Expected int, got non-finite float {d}")); return errs; }
                if (d != Math.Floor(d)) { errs.Add(new ValidationError(path, $"Expected int, got float with fractional part: {d}")); return errs; }
                if (d < IntMin || d > IntMax) { errs.Add(new ValidationError(path, $"Expected int in signed 32-bit range, got {d}")); return errs; }
                return errs;
            }
            if (value is float f)
            {
                double df = f;
                if (double.IsNaN(df) || double.IsInfinity(df)) { errs.Add(new ValidationError(path, $"Expected int, got non-finite float {df}")); return errs; }
                if (df != Math.Floor(df)) { errs.Add(new ValidationError(path, $"Expected int, got float with fractional part: {df}")); return errs; }
                if (df < IntMin || df > IntMax) { errs.Add(new ValidationError(path, $"Expected int in signed 32-bit range, got {df}")); return errs; }
                return errs;
            }
            if (value is bool)
            {
                errs.Add(new ValidationError(path, "Expected int, got bool"));
                return errs;
            }
            errs.Add(new ValidationError(path, $"Expected int, got {TypeName(value)}"));
            return errs;
        }

        public const long IntMin = -2147483648L;
        public const long IntMax = 2147483647L;

        public static List<ValidationError> ValidateFloat(object? value, string path)
        {
            var errs = new List<ValidationError>();
            if (value is bool)
            {
                errs.Add(new ValidationError(path, "Expected float, got bool"));
                return errs;
            }
            if (value is double d)
            {
                if (double.IsNaN(d) || double.IsInfinity(d))
                {
                    errs.Add(new ValidationError(path, "Expected a finite float value"));
                    return errs;
                }
                return errs;
            }
            if (value is float f)
            {
                if (float.IsNaN(f) || float.IsInfinity(f))
                {
                    errs.Add(new ValidationError(path, "Expected a finite float value"));
                    return errs;
                }
                return errs;
            }
            if (value is int or long)
            {
                return errs;
            }
            errs.Add(new ValidationError(path, $"Expected float, got {TypeName(value)}"));
            return errs;
        }

        public static List<ValidationError> ValidateBool(object? value, string path)
        {
            var errs = new List<ValidationError>();
            if (value is not bool)
                errs.Add(new ValidationError(path, $"Expected bool, got {TypeName(value)}"));
            return errs;
        }

        public static List<ValidationError> ValidateArray(object? value, Func<object?, string, List<ValidationError>> elementValidator, string path)
        {
            var errs = new List<ValidationError>();
            if (value is string || value is IDictionary || value == null)
            {
                // null handled as error unless... match original: null array invalid
                if (value == null) { errs.Add(new ValidationError(path, $"Expected array, got null")); return errs; }
                if (value is string || value is IDictionary) { errs.Add(new ValidationError(path, $"Expected array, got {TypeName(value)}")); return errs; }
            }
            if (value is not IEnumerable enumerable)
            {
                errs.Add(new ValidationError(path, $"Expected array, got {TypeName(value)}"));
                return errs;
            }
            int i = 0;
            foreach (var elem in enumerable)
            {
                string elemPath = $"{path}[{i}]";
                var sub = elementValidator(elem, elemPath);
                if (sub != null) errs.AddRange(sub);
                i++;
            }
            return errs;
        }

        public static List<ValidationError> ValidateMap(object? value, Func<object?, string, List<ValidationError>> valueValidator, string path)
        {
            var errs = new List<ValidationError>();
            if (value is not IDictionary dict)
            {
                errs.Add(new ValidationError(path, $"Expected map, got {TypeName(value)}"));
                return errs;
            }
            foreach (DictionaryEntry entry in dict)
            {
                if (entry.Key is not string key)
                {
                    errs.Add(new ValidationError(path, $"Map key must be string, got {TypeName(entry.Key)}"));
                    continue;
                }
                string keyPath = $"{path}[{key}]";
                var sub = valueValidator(entry.Value, keyPath);
                if (sub != null) errs.AddRange(sub);
            }
            return errs;
        }

        public static List<ValidationError> ValidateEnum(object? value, string enumName, List<string> allowedValues, string path)
        {
            var errs = new List<ValidationError>();
            if (value is not string s)
            {
                errs.Add(new ValidationError(path, $"Expected string for enum {enumName}, got {TypeName(value)}"));
                return errs;
            }
            if (!allowedValues.Contains(s))
            {
                errs.Add(new ValidationError(path, $"Invalid value for enum {enumName}: '{s}'. Allowed values: [{string.Join(", ", allowedValues)}]"));
            }
            return errs;
        }

        /// <summary>Enum-definition path using the cached set and cached error text.</summary>
        private static List<ValidationError> ValidateEnum(object? value, string enumName, EnumInfo info, string path)
        {
            var errs = new List<ValidationError>();
            if (value is not string s)
            {
                errs.Add(new ValidationError(path, $"Expected string for enum {enumName}, got {TypeName(value)}"));
                return errs;
            }
            if (!info.Allowed.Contains(s))
            {
                errs.Add(new ValidationError(path, $"Invalid value for enum {enumName}: '{s}'. Allowed values: {info.Display}"));
            }
            return errs;
        }

        public static List<ValidationError> ValidateStruct(object? value, string structName, Dictionary<string, object> structDef,
            Dictionary<string, Dictionary<string, object>> allStructs,
            Dictionary<string, Dictionary<string, object>> allEnums, string path,
            Func<string, List<Dictionary<string, object>>>? fieldResolver = null)
        {
            var errs = new List<ValidationError>();
            if (value is not IDictionary dict)
            {
                errs.Add(new ValidationError(path, $"Expected map for struct {structName}, got {TypeName(value)}"));
                return errs;
            }
            var fields = fieldResolver != null
                ? fieldResolver(structName)
                : Types.GetStructFields(structName, allStructs);
            foreach (var field in fields)
            {
                string fieldName = (string)field["name"];
                var fieldType = (Dictionary<string, object>)field["type"];
                bool isOptional = field.ContainsKey("optional") && (bool)field["optional"];
                string fieldPath = string.IsNullOrEmpty(path) ? "." + fieldName : path + "." + fieldName;

                if (!dict.Contains(fieldName))
                {
                    if (!isOptional)
                        errs.Add(new ValidationError(fieldPath, $"Missing required field '{fieldName}' in struct {structName}"));
                }
                else
                {
                    var fieldValue = dict[fieldName];
                    if (fieldValue == null)
                    {
                        if (!isOptional)
                            errs.Add(new ValidationError(fieldPath, $"Field '{fieldName}' in struct {structName} cannot be null"));
                    }
                    else
                    {
                        var sub = ValidateType(fieldValue, fieldType, allStructs, allEnums, false, fieldPath, fieldResolver);
                        errs.AddRange(sub);
                    }
                }
            }
            return errs;
        }

        public static List<ValidationError> ValidateType(object? value, Dictionary<string, object> typeDef,
            Dictionary<string, Dictionary<string, object>> allStructs,
            Dictionary<string, Dictionary<string, object>> allEnums,
            bool isOptional, string path,
            Func<string, List<Dictionary<string, object>>>? fieldResolver = null)
        {
            var errs = new List<ValidationError>();
            if (value == null)
            {
                if (!isOptional)
                    errs.Add(new ValidationError(path, "Value cannot be null for non-optional type"));
                return errs;
            }
            if (typeDef.ContainsKey("builtIn"))
            {
                string builtIn = (string)typeDef["builtIn"];
                switch (builtIn)
                {
                    case "string": return ValidateString(value, path);
                    case "int": return ValidateInt(value, path);
                    case "float": return ValidateFloat(value, path);
                    case "bool": return ValidateBool(value, path);
                    default: errs.Add(new ValidationError(path, $"Unknown built-in type: {builtIn}")); return errs;
                }
            }
            if (typeDef.ContainsKey("array"))
            {
                var elementType = (Dictionary<string, object>)typeDef["array"];
                Func<object?, string, List<ValidationError>> cb = (v, p) => ValidateType(v, elementType, allStructs, allEnums, false, p, fieldResolver);
                return ValidateArray(value, cb, path);
            }
            if (typeDef.ContainsKey("mapValue"))
            {
                var valueType = (Dictionary<string, object>)typeDef["mapValue"];
                Func<object?, string, List<ValidationError>> cb = (v, p) => ValidateType(v, valueType, allStructs, allEnums, false, p, fieldResolver);
                return ValidateMap(value, cb, path);
            }
            if (typeDef.ContainsKey("userDefined"))
            {
                string userType = (string)typeDef["userDefined"];
                var structDef = Types.FindStruct(userType, allStructs);
                if (structDef != null)
                {
                    return ValidateStruct(value, userType, structDef, allStructs, allEnums, path, fieldResolver);
                }
                var enumDef = Types.FindEnum(userType, allEnums);
                if (enumDef != null)
                {
                    if (enumDef.TryGetValue("values", out var valuesObj) && valuesObj is IList)
                    {
                        return ValidateEnum(value, userType, GetEnumInfo(enumDef), path);
                    }
                    errs.Add(new ValidationError(path, $"Invalid enum definition for {userType}: missing values"));
                    return errs;
                }
                errs.Add(new ValidationError(path, $"Unknown user-defined type: {userType}"));
                return errs;
            }
            errs.Add(new ValidationError(path, "Invalid type definition"));
            return errs;
        }

        // ---- Backward-compatible throwing overloads ----
        public static void ValidateString(object? value)
        {
            var e = ValidateString(value, "");
            if (e.Count > 0) throw new ArgumentException(e[0].Message);
        }
        public static void ValidateInt(object? value)
        {
            var e = ValidateInt(value, "");
            if (e.Count > 0) throw new ArgumentException(e[0].Message);
        }
        public static void ValidateFloat(object? value)
        {
            var e = ValidateFloat(value, "");
            if (e.Count > 0) throw new ArgumentException(e[0].Message);
        }
        public static void ValidateBool(object? value)
        {
            var e = ValidateBool(value, "");
            if (e.Count > 0) throw new ArgumentException(e[0].Message);
        }
        public static void ValidateEnum(object? value, string enumName, List<string> allowedValues)
        {
            var e = ValidateEnum(value, enumName, allowedValues, "");
            if (e.Count > 0) throw new ArgumentException(e[0].Message);
        }
        public static void ValidateArray(object? value, Action<object?> elementValidator)
        {
            Func<object?, string, List<ValidationError>> cb = (v, p) =>
            {
                try { elementValidator(v); return new List<ValidationError>(); }
                catch (Exception e) { return new List<ValidationError> { new ValidationError(p, e.Message) }; }
            };
            var e = ValidateArray(value, cb, "");
            if (e.Count > 0) throw new ArgumentException(e[0].Message);
        }
        public static void ValidateMap(object? value, Action<object?> valueValidator)
        {
            Func<object?, string, List<ValidationError>> cb = (v, p) =>
            {
                try { valueValidator(v); return new List<ValidationError>(); }
                catch (Exception e) { return new List<ValidationError> { new ValidationError(p, e.Message) }; }
            };
            var e = ValidateMap(value, cb, "");
            if (e.Count > 0) throw new ArgumentException(e[0].Message);
        }
        public static void ValidateStruct(object? value, string structName, Dictionary<string, object> structDef,
            Dictionary<string, Dictionary<string, object>> allStructs,
            Dictionary<string, Dictionary<string, object>> allEnums)
        {
            var e = ValidateStruct(value, structName, structDef, allStructs, allEnums, "");
            if (e.Count > 0) throw new ArgumentException(e[0].Message);
        }
        public static void ValidateType(object? value, Dictionary<string, object> typeDef,
            Dictionary<string, Dictionary<string, object>> allStructs,
            Dictionary<string, Dictionary<string, object>> allEnums,
            bool isOptional)
        {
            var e = ValidateType(value, typeDef, allStructs, allEnums, isOptional, "");
            if (e.Count > 0) throw new ArgumentException(e[0].Message);
        }

        public static void ValidateType(object? value, Dictionary<string, object> typeDef,
            Dictionary<string, Dictionary<string, object>> allStructs,
            Dictionary<string, Dictionary<string, object>> allEnums)
        {
            var e = ValidateType(value, typeDef, allStructs, allEnums, false, "");
            if (e.Count > 0) throw new ArgumentException(e[0].Message);
        }
    }
}

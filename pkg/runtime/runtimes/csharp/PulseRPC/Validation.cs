using System;
using System.Collections;
using System.Collections.Generic;
using System.Linq;

namespace PulseRPC
{
    /// <summary>
    /// Validation functions for PulseRPC types
    /// </summary>
    public static class Validation
    {
        /// <summary>
        /// Validate that value is a string
        /// </summary>
        public static void ValidateString(object? value)
        {
            if (value is not string)
            {
                throw new ArgumentException($"Expected string, got {value?.GetType().Name ?? "null"}");
            }
        }

        /// <summary>
        /// Validate that value is an int
        /// </summary>
        public static void ValidateInt(object? value)
        {
            if (value is int)
            {
                return;
            }
            // Accept whole-number floats/doubles
            if (value is double d && d == Math.Floor(d))
            {
                return;
            }
            if (value is long l && l == Math.Floor((double)l))
            {
                return;
            }
            throw new ArgumentException($"Expected int, got {value?.GetType().Name ?? "null"}");
        }

        /// <summary>
        /// Validate that value is a float or int
        /// </summary>
        public static void ValidateFloat(object? value)
        {
            if (value is not float && value is not double && value is not int)
            {
                throw new ArgumentException($"Expected float, got {value?.GetType().Name ?? "null"}");
            }
        }

        /// <summary>
        /// Validate that value is a bool
        /// </summary>
        public static void ValidateBool(object? value)
        {
            if (value is not bool)
            {
                throw new ArgumentException($"Expected bool, got {value?.GetType().Name ?? "null"}");
            }
        }

        /// <summary>
        /// Validate that value is an array and each element passes validation
        /// </summary>
        public static void ValidateArray(object? value, Action<object?> elementValidator)
        {
            // Check if it's an array or list (but not a string or Dictionary, which also implement IEnumerable)
            if (value == null || value is string || value is System.Collections.IDictionary || value is not System.Collections.IEnumerable enumerable)
            {
                throw new ArgumentException($"Expected array, got {value?.GetType().Name ?? "null"}");
            }

            int index = 0;
            foreach (var elem in enumerable)
            {
                try
                {
                    elementValidator(elem);
                }
                catch (Exception e)
                {
                    throw new ArgumentException($"Array element at index {index} validation failed: {e.Message}", e);
                }
                index++;
            }
        }

        /// <summary>
        /// Validate that value is a map (Dictionary) with string keys and validated values
        /// </summary>
        public static void ValidateMap(object? value, Action<object?> valueValidator)
        {
            if (value is not Dictionary<string, object?> dict)
            {
                throw new ArgumentException($"Expected dictionary, got {value?.GetType().Name ?? "null"}");
            }

            foreach (var kvp in dict)
            {
                try
                {
                    valueValidator(kvp.Value);
                }
                catch (Exception e)
                {
                    throw new ArgumentException($"Map value for key '{kvp.Key}' validation failed: {e.Message}", e);
                }
            }
        }

        /// <summary>
        /// Validate that value is a string and matches one of the allowed enum values
        /// </summary>
        public static void ValidateEnum(object? value, string enumName, List<string> allowedValues)
        {
            if (value is not string strValue)
            {
                throw new ArgumentException($"Expected string for enum {enumName}, got {value?.GetType().Name ?? "null"}");
            }

            if (!allowedValues.Contains(strValue))
            {
                throw new ArgumentException($"Invalid value for enum {enumName}: '{strValue}'. Allowed values: [{string.Join(", ", allowedValues.Select(v => $"'{v}'"))}]");
            }
        }

        /// <summary>
        /// Validate that value is a Dictionary matching the struct definition
        /// </summary>
        public static void ValidateStruct(
            object? value,
            string structName,
            Dictionary<string, object> structDef,
            Dictionary<string, Dictionary<string, object>> allStructs,
            Dictionary<string, Dictionary<string, object>> allEnums)
        {
            if (value is not Dictionary<string, object?> dict)
            {
                throw new ArgumentException($"Expected dictionary for struct {structName}, got {value?.GetType().Name ?? "null"}");
            }

            // Get all fields including parent fields
            var fields = Types.GetStructFields(structName, allStructs);

            // Check required fields
            foreach (var field in fields)
            {
                var fieldName = field["name"].ToString() ?? "";
                var fieldType = field["type"];
                var isOptional = field.TryGetValue("optional", out var optionalObj) && optionalObj is bool opt && opt;

                if (!dict.ContainsKey(fieldName))
                {
                    if (!isOptional)
                    {
                        throw new ArgumentException($"Missing required field '{fieldName}' in struct {structName}");
                    }
                }
                else
                {
                    var fieldValue = dict[fieldName];
                    if (fieldValue == null)
                    {
                        if (!isOptional)
                        {
                            throw new ArgumentException($"Field '{fieldName}' in struct {structName} cannot be null");
                        }
                    }
                    else
                    {
                        try
                        {
                            if (fieldType is Dictionary<string, object> typeDict)
                            {
                                ValidateType(fieldValue, typeDict, allStructs, allEnums, false);
                            }
                            else
                            {
                                // Field type should always be a Dictionary, but handle edge cases
                                throw new ArgumentException($"Invalid field type definition for field '{fieldName}' in struct {structName}");
                            }
                        }
                        catch (Exception e)
                        {
                            throw new ArgumentException($"Field '{fieldName}' in struct {structName} validation failed: {e.Message}", e);
                        }
                    }
                }
            }
        }

        /// <summary>
        /// Validate a value against a type definition
        /// </summary>
        public static void ValidateType(
            object? value,
            Dictionary<string, object> typeDef,
            Dictionary<string, Dictionary<string, object>> allStructs,
            Dictionary<string, Dictionary<string, object>> allEnums,
            bool isOptional = false)
        {
            // Handle optional types
            if (value == null)
            {
                if (isOptional)
                {
                    return;
                }
                else
                {
                    throw new ArgumentException("Value cannot be null for non-optional type");
                }
            }

            // Built-in types
            if (typeDef.TryGetValue("builtIn", out var builtInObj) && builtInObj is string builtIn)
            {
                switch (builtIn)
                {
                    case "string":
                        ValidateString(value);
                        break;
                    case "int":
                        ValidateInt(value);
                        break;
                    case "float":
                        ValidateFloat(value);
                        break;
                    case "bool":
                        ValidateBool(value);
                        break;
                    default:
                        throw new ArgumentException($"Unknown built-in type: {builtIn}");
                }
            }
            // Array types
            else if (typeDef.TryGetValue("array", out var arrayObj))
            {
                if (arrayObj is Dictionary<string, object> elementType)
                {
                    ValidateArray(value, elem => ValidateType(elem, elementType, allStructs, allEnums, false));
                }
                else
                {
                    throw new ArgumentException("Invalid array type definition");
                }
            }
            // Map types
            else if (typeDef.TryGetValue("mapValue", out var mapValueObj))
            {
                if (mapValueObj is Dictionary<string, object> valueType)
                {
                    ValidateMap(value, val => ValidateType(val, valueType, allStructs, allEnums, false));
                }
                else
                {
                    throw new ArgumentException("Invalid map type definition");
                }
            }
            // User-defined types
            else if (typeDef.TryGetValue("userDefined", out var userDefinedObj) && userDefinedObj is string userType)
            {
                // Check if it's a struct
                var structDef = Types.FindStruct(userType, allStructs);
                if (structDef != null)
                {
                    ValidateStruct(value, userType, structDef, allStructs, allEnums);
                }
                // Check if it's an enum
                else
                {
                    var enumDef = Types.FindEnum(userType, allEnums);
                    if (enumDef != null)
                    {
                        if (enumDef.TryGetValue("values", out var valuesObj) && valuesObj is System.Collections.IList enumValues)
                        {
                            var allowedValues = enumValues
                                .OfType<Dictionary<string, object>>()
                                .Select(v => v.TryGetValue("name", out var nameObj) ? nameObj.ToString() ?? "" : "")
                                .Where(name => !string.IsNullOrEmpty(name))
                                .ToList();
                            ValidateEnum(value, userType, allowedValues);
                        }
                        else
                        {
                            throw new ArgumentException($"Invalid enum definition for {userType}: missing values");
                        }
                    }
                    else
                    {
                        throw new ArgumentException($"Unknown user-defined type: {userType}");
                    }
                }
            }
            else
            {
                throw new ArgumentException($"Invalid type definition: {System.Text.Json.JsonSerializer.Serialize(typeDef)}");
            }
        }
    }
}


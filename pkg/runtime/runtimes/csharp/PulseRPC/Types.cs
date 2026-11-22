using System;
using System.Collections.Generic;
using System.Linq;

namespace PulseRPC
{
    /// <summary>
    /// Helper functions for working with type definitions
    /// </summary>
    public static class Types
    {
        /// <summary>
        /// Find a struct definition by name
        /// </summary>
        public static Dictionary<string, object>? FindStruct(string structName, Dictionary<string, Dictionary<string, object>> allStructs)
        {
            // Exact match first.
            if (allStructs.TryGetValue(structName, out var structDef))
            {
                return structDef;
            }

            // A field may reference a struct by a qualified name such as
            // "inc.Address" while the struct is registered as "Address". Mirror
            // FindEnum's qualified-name fallback. When two namespaces register the
            // same base name, resolve deterministically to the first full name in
            // ordinal order so resolution is stable (rather than arbitrary).
            if (structName.Contains("."))
            {
                var baseName = structName.Substring(structName.LastIndexOf('.') + 1);
                if (allStructs.TryGetValue(baseName, out structDef))
                {
                    return structDef;
                }

                var candidate = allStructs.Keys
                    .Where(k => k.EndsWith("." + baseName, StringComparison.Ordinal))
                    .OrderBy(k => k, StringComparer.Ordinal)
                    .FirstOrDefault();
                if (candidate != null)
                {
                    return allStructs[candidate];
                }
            }

            return null;
        }

        /// <summary>
        /// Find an enum definition by name
        /// </summary>
        public static Dictionary<string, object>? FindEnum(string enumName, Dictionary<string, Dictionary<string, object>> allEnums)
        {
            // Try qualified name first
            if (allEnums.TryGetValue(enumName, out var enumDef))
                return enumDef;
            
            // Try unqualified name (extract base name from qualified name)
            if (enumName.Contains("."))
            {
                var baseName = enumName.Substring(enumName.LastIndexOf('.') + 1);
                if (allEnums.TryGetValue(baseName, out enumDef))
                    return enumDef;
            }
            
            return null;
        }

        /// <summary>
        /// Recursively resolve struct extends to return all fields (parent + child)
        /// </summary>
        public static List<Dictionary<string, object>> GetStructFields(string structName, Dictionary<string, Dictionary<string, object>> allStructs)
        {
            var structDef = FindStruct(structName, allStructs);
            if (structDef == null)
            {
                return new List<Dictionary<string, object>>();
            }

            var fields = new List<Dictionary<string, object>>();

            // Get parent fields first
            if (structDef.TryGetValue("extends", out var extendsObj) && extendsObj is string parentName)
            {
                var parentFields = GetStructFields(parentName, allStructs);
                fields.AddRange(parentFields);
            }

            // Add child fields (override parent if name conflict)
            var fieldNames = new HashSet<string>(fields.Select(f => f["name"].ToString() ?? ""));
            if (structDef.TryGetValue("fields", out var fieldsObj) && fieldsObj is System.Collections.IEnumerable structFieldsEnumerable)
            {
                foreach (var fieldObj in structFieldsEnumerable)
                {
                    if (fieldObj is Dictionary<string, object> field)
                    {
                        var fieldName = field["name"].ToString() ?? "";
                        if (!fieldNames.Contains(fieldName))
                        {
                            fields.Add(field);
                            fieldNames.Add(fieldName);
                        }
                        else
                        {
                            // Override parent field
                            for (int i = 0; i < fields.Count; i++)
                            {
                                if (fields[i]["name"].ToString() == fieldName)
                                {
                                    fields[i] = field;
                                    break;
                                }
                            }
                        }
                    }
                }
            }

            return fields;
        }
    }
}


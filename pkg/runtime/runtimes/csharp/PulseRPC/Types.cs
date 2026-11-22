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
            return ResolveQualifiedName(structName, allStructs);
        }

        /// <summary>
        /// Find an enum definition by name
        /// </summary>
        public static Dictionary<string, object>? FindEnum(string enumName, Dictionary<string, Dictionary<string, object>> allEnums)
        {
            return ResolveQualifiedName(enumName, allEnums);
        }

        /// <summary>
        /// Shared qualified-name resolution (D16): exact key, then exact base
        /// name, then first sorted qualified key sharing the base name.
        /// </summary>
        private static Dictionary<string, object>? ResolveQualifiedName(string name, Dictionary<string, Dictionary<string, object>> all)
        {
            if (all.TryGetValue(name, out var def))
            {
                return def;
            }

            if (name.Contains("."))
            {
                var baseName = name.Substring(name.LastIndexOf('.') + 1);
                if (all.TryGetValue(baseName, out def))
                {
                    return def;
                }

                var candidate = all.Keys
                    .Where(k => k.EndsWith("." + baseName, StringComparison.Ordinal))
                    .OrderBy(k => k, StringComparer.Ordinal)
                    .FirstOrDefault();
                if (candidate != null && all.TryGetValue(candidate, out def))
                {
                    return def;
                }
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


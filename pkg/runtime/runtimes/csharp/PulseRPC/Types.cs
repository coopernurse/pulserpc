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
            // Name -> index keeps inherited-field overrides O(1) instead of
            // scanning the accumulated field list for every child field.
            var fieldIndex = new Dictionary<string, int>(StringComparer.Ordinal);

            // Get parent fields first
            if (structDef.TryGetValue("extends", out var extendsObj) && extendsObj is string parentName)
            {
                foreach (var parentField in GetStructFields(parentName, allStructs))
                {
                    AddOrReplace(fields, fieldIndex, parentField);
                }
            }

            // Add child fields (override parent if name conflict)
            if (structDef.TryGetValue("fields", out var fieldsObj) && fieldsObj is System.Collections.IEnumerable structFieldsEnumerable)
            {
                foreach (var fieldObj in structFieldsEnumerable)
                {
                    if (fieldObj is Dictionary<string, object> field)
                    {
                        AddOrReplace(fields, fieldIndex, field);
                    }
                }
            }

            return fields;
        }

        /// <summary>
        /// Adds a field, or replaces an existing field with the same name in
        /// O(1) using the maintained name -> index map.
        /// </summary>
        private static void AddOrReplace(List<Dictionary<string, object>> fields,
            Dictionary<string, int> fieldIndex, Dictionary<string, object> field)
        {
            var fieldName = field.TryGetValue("name", out var nameObj) ? nameObj?.ToString() ?? "" : "";
            if (fieldIndex.TryGetValue(fieldName, out var index))
            {
                fields[index] = field;
                return;
            }
            fieldIndex[fieldName] = fields.Count;
            fields.Add(field);
        }
    }
}


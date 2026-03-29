using System;
using System.Collections.Generic;
using System.Linq;

namespace PulseRPC
{
    /// <summary>
    /// Represents a function/method definition from the IDL
    /// </summary>
    public class FunctionDef : Dictionary<string, object>
    {
    }

    /// <summary>
    /// Represents an interface from the IDL
    /// </summary>
    public class Interface
    {
        public string Name { get; set; } = "";
        public Dictionary<string, FunctionDef> Functions { get; set; } = new Dictionary<string, FunctionDef>();

        /// <summary>
        /// Get a function definition by name
        /// </summary>
        public FunctionDef? GetFunction(string funcName)
        {
            return Functions.TryGetValue(funcName, out var fn) ? fn : null;
        }
    }

    /// <summary>
    /// Represents a parsed IDL contract
    /// </summary>
    public class Contract
    {
        public object IdlParsed { get; set; } = null!;
        public Dictionary<string, Interface> Interfaces { get; set; } = new Dictionary<string, Interface>();
        public Dictionary<string, Dictionary<string, object>> Structs { get; set; } = new Dictionary<string, Dictionary<string, object>>();
        public Dictionary<string, Dictionary<string, object>> Enums { get; set; } = new Dictionary<string, Dictionary<string, object>>();
        public Dictionary<string, object> Meta { get; set; } = new Dictionary<string, object>();

        /// <summary>
        /// Creates a new Contract from parsed IDL data.
        /// Supports both PulseRPC format (dict with interfaces, structs, enums keys)
        /// and Barrister format (list of items with type field)
        /// </summary>
        public Contract(object idlParsed)
        {
            IdlParsed = idlParsed;

            // Handle both barrister format (list) and PulseRPC format (dict)
            if (idlParsed is Dictionary<string, object> dict)
            {
                // PulseRPC format - dict with interfaces, structs, enums keys

                // Parse interfaces
                if (dict.TryGetValue("interfaces", out var interfacesObj) && interfacesObj is System.Collections.IList interfacesList)
                {
                    foreach (var ifaceData in interfacesList)
                    {
                        if (ifaceData is Dictionary<string, object> ifaceDict && ifaceDict.TryGetValue("name", out var nameObj))
                        {
                            var iface = new Interface
                            {
                                Name = nameObj?.ToString() ?? ""
                            };

                            if (ifaceDict.TryGetValue("methods", out var methodsObj) && methodsObj is System.Collections.IList methodsList)
                            {
                                foreach (var method in methodsList)
                                {
                                    if (method is Dictionary<string, object> methodDict && methodDict.TryGetValue("name", out var funcNameObj))
                                    {
                                        var funcName = funcNameObj?.ToString() ?? "";
                                        var funcDef = new FunctionDef();
foreach (var kvp in methodDict)
{
    funcDef[kvp.Key] = kvp.Value;
}
iface.Functions[funcName] = funcDef;
                                    }
                                }
                            }
                            Interfaces[iface.Name] = iface;
                        }
                    }
                }

                // Parse structs
                if (dict.TryGetValue("structs", out var structsObj) && structsObj is System.Collections.IList structsList)
                {
                    foreach (var structData in structsList)
                    {
                        if (structData is Dictionary<string, object> structDict && structDict.TryGetValue("name", out var structNameObj))
                        {
                            var name = structNameObj?.ToString() ?? "";
                            Structs[name] = structDict;
                        }
                    }
                }

                // Parse enums
                if (dict.TryGetValue("enums", out var enumsObj) && enumsObj is System.Collections.IList enumsList)
                {
                    foreach (var enumData in enumsList)
                    {
                        if (enumData is Dictionary<string, object> enumDict && enumDict.TryGetValue("name", out var enumNameObj))
                        {
                            var name = enumNameObj?.ToString() ?? "";
                            Enums[name] = enumDict;
                        }
                    }
                }
            }
            else if (idlParsed is System.Collections.IList list)
            {
                // Barrister format - list of items with type field
                foreach (var item in list)
                {
                    if (item is Dictionary<string, object> itemDict && itemDict.TryGetValue("type", out var typeObj))
                    {
                        var itemType = typeObj?.ToString();
                        switch (itemType)
                        {
                            case "struct":
                                if (itemDict.TryGetValue("name", out var structNameObj))
                                {
                                    var name = structNameObj?.ToString() ?? "";
                                    Structs[name] = itemDict;
                                }
                                break;
                            case "enum":
                                if (itemDict.TryGetValue("name", out var enumNameObj))
                                {
                                    var name = enumNameObj?.ToString() ?? "";
                                    Enums[name] = itemDict;
                                }
                                break;
                            case "interface":
                                var iface = new Interface();
                                if (itemDict.TryGetValue("name", out var ifaceNameObj))
                                {
                                    iface.Name = ifaceNameObj?.ToString() ?? "";
                                }
                                if (itemDict.TryGetValue("methods", out var methodsObj) && methodsObj is System.Collections.IList methodsList)
                                {
                                    foreach (var method in methodsList)
                                    {
                                        if (method is Dictionary<string, object> methodDict && methodDict.TryGetValue("name", out var funcNameObj))
                                        {
                                            var funcName = funcNameObj?.ToString() ?? "";
                                            var funcDef = new FunctionDef();
foreach (var kvp in methodDict)
{
    funcDef[kvp.Key] = kvp.Value;
}
iface.Functions[funcName] = funcDef;
                                        }
                                    }
                                }
                                Interfaces[iface.Name] = iface;
                                break;
                            case "meta":
                                // Copy metadata
                                foreach (var kvp in itemDict)
                                {
                                    if (kvp.Key != "type")
                                    {
                                        Meta[kvp.Key] = kvp.Value;
                                    }
                                }
                                break;
                        }
                    }
                }
            }
        }

        /// <summary>
        /// Check if an interface exists in the contract
        /// </summary>
        public bool HasInterface(string ifaceName)
        {
            return Interfaces.ContainsKey(ifaceName);
        }

        /// <summary>
        /// Get an interface by name
        /// </summary>
        public Interface? GetInterface(string ifaceName)
        {
            return Interfaces.TryGetValue(ifaceName, out var iface) ? iface : null;
        }

        /// <summary>
        /// Validate request parameters against the IDL
        /// </summary>
        public void ValidateRequest(string ifaceName, string funcName, List<object?> @params)
        {
            var iface = GetInterface(ifaceName);
            if (iface == null)
            {
                throw new ArgumentException($"Unknown interface: '{ifaceName}'");
            }

            var fn = iface.GetFunction(funcName);
            if (fn == null)
            {
                throw new ArgumentException($"{ifaceName}: Unknown function: '{funcName}'");
            }

            // Get parameter definitions
            var paramDefs = fn.TryGetValue("parameters", out var paramDefsObj) && paramDefsObj is System.Collections.IList paramList
                ? paramList
                : new System.Collections.ArrayList();

            // Check parameter count
            if (@params.Count != paramDefs.Count)
            {
                throw new ArgumentException($"Function '{ifaceName}.{funcName}' expects {paramDefs.Count} param(s), {@params.Count} given");
            }

            // Validate each parameter
            for (int i = 0; i < @params.Count; i++)
            {
                var paramValue = @params[i];
                var paramDef = paramDefs[i] as Dictionary<string, object>;
                if (paramDef == null) continue;

                var paramName = paramDef.TryGetValue("name", out var pn) ? pn?.ToString() ?? "" : "";
                var paramType = paramDef.TryGetValue("type", out var pt) && pt is Dictionary<string, object> ptDict
                    ? ptDict
                    : new Dictionary<string, object>();

                var isOptional = paramDef.TryGetValue("optional", out var optObj) && optObj is bool opt && opt;

                try
                {
                    Validation.ValidateType(paramValue, paramType, Structs, Enums, isOptional);
                }
                catch (Exception e)
                {
                    throw new ArgumentException($"Function '{ifaceName}.{funcName}' invalid param '{paramName}': {e.Message}", e);
                }
            }
        }

        /// <summary>
        /// Validate response result against the IDL
        /// </summary>
        public void ValidateResponse(string ifaceName, string funcName, object? result)
        {
            var iface = GetInterface(ifaceName);
            if (iface == null)
            {
                throw new ArgumentException($"Unknown interface: '{ifaceName}'");
            }

            var fn = iface.GetFunction(funcName);
            if (fn == null)
            {
                throw new ArgumentException($"{ifaceName}: Unknown function: '{funcName}'");
            }

            // Check if function has a return type
            if (!fn.TryGetValue("returnType", out var returnTypeObj) || returnTypeObj is not Dictionary<string, object> returnType)
            {
                // Function returns void/None
                if (result != null)
                {
                    throw new ArgumentException($"Function '{ifaceName}.{funcName}' invalid response: expected null, got {result}");
                }
                return;
            }

            // Validate return type
            var isOptional = fn.TryGetValue("returnOptional", out var retOptObj) && retOptObj is bool retOpt && retOpt;

            try
            {
                Validation.ValidateType(result, returnType, Structs, Enums, isOptional);
            }
            catch (Exception e)
            {
                throw new ArgumentException($"Function '{ifaceName}.{funcName}' invalid response: {e.Message}", e);
            }
        }
    }
}

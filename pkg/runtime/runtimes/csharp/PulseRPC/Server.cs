using System;
using System.Collections.Generic;
using System.Linq;
using System.Reflection;

namespace PulseRPC
{
    /// <summary>
    /// JSON-RPC 2.0 server with handler registration and optional validation
    /// </summary>
    public class Server
    {
        private readonly Contract _contract;
        private readonly Dictionary<string, object> _handlers = new Dictionary<string, object>();
        private readonly bool _validateRequests;
        private readonly bool _validateResponses;

        /// <summary>
        /// Optional server-side sink for internal error details. The wire never
        /// carries exception text or stack traces (D4/R15); callers that want
        /// diagnostics supply a logger here.
        /// </summary>
        public Action<string, Exception>? InternalErrorLogger { get; set; }

        /// <summary>
        /// Creates a new Server instance
        /// </summary>
        public Server(Contract contract, bool validateRequests = true, bool validateResponses = true)
        {
            _contract = contract;
            _validateRequests = validateRequests;
            _validateResponses = validateResponses;
        }

        private void LogInternalError(string context, Exception ex)
        {
            if (InternalErrorLogger != null)
            {
                InternalErrorLogger(context, ex);
                return;
            }

            // Default diagnostics deliberately omit the exception message and
            // stack trace: handler text can contain secrets and must never be
            // written anywhere implicitly. Supply InternalErrorLogger to get
            // the full detail (R15/D4).
            Console.Error.WriteLine($"[PulseRPC] {context}: {ex.GetType().FullName}");
        }

        /// <summary>
        /// Registers a handler instance for an interface
        /// </summary>
        public void AddHandler(string ifaceName, object handler)
        {
            _handlers[ifaceName] = handler;
        }

        /// <summary>
        /// Processes a single JSON-RPC request and returns a JSON-RPC response.
        /// Returns null for notifications (requests without 'id').
        /// </summary>
        public Dictionary<string, object>? Call(Dictionary<string, object> request)
        {
            // Parse the id first: a notification (no id) must never receive a
            // response, including an error response (M4/R11). An id is only
            // allowed to be a string, a number, or null (D2/R24); booleans,
            // objects, and arrays are invalid requests.
            var reqId = request == null ? null : (request.TryGetValue("id", out var idVal) ? idVal : null);
            if (reqId is bool || reqId is System.Collections.IDictionary || reqId is System.Collections.IList)
            {
                return ErrorResponse(null, -32600, "Invalid Request", "id must be a string, number, or null");
            }
            var isNotification = reqId == null;

            Dictionary<string, object>? Fail(int code, string message, object? data = null)
                => isNotification ? null : ErrorResponse(reqId, code, message, data);

            // Validate request format
            if (request == null)
            {
                return Fail(-32600, "Invalid Request", "Request must be an object");
            }

            // Check JSON-RPC version
            if (!request.TryGetValue("jsonrpc", out var jsonrpc) || jsonrpc?.ToString() != "2.0")
            {
                return Fail(-32600, "Invalid Request", "jsonrpc version must be '2.0'");
            }

            // Check for method
            if (!request.TryGetValue("method", out var methodObj) || methodObj is not string method || string.IsNullOrEmpty(method))
            {
                return Fail(-32600, "Invalid Request", "Method must be a string");
            }

            // Handle pulserpc-idl request
            if (method == "pulserpc-idl")
            {
                return isNotification ? null : SuccessResponse(reqId, _contract.IdlParsed);
            }

            // Parse method name (e.g., "UserService.getUser")
            if (!ParseMethodName(method, out var ifaceName, out var funcName))
            {
                return Fail(-32601, "Method not found", $"Invalid method name format: {method}");
            }

            // Look up handler
            if (!_handlers.TryGetValue(ifaceName, out var handler))
            {
                return Fail(-32601, "Method not found", $"Unknown interface: {ifaceName}");
            }

            // Get function from handler
            var handlerType = handler.GetType();
            var methodInfo = handlerType.GetMethod(funcName, BindingFlags.Public | BindingFlags.Instance | BindingFlags.IgnoreCase);
            if (methodInfo == null)
            {
                // Try with first letter uppercased (Go/C# convention)
                var capitalizedName = char.ToUpper(funcName[0]) + funcName.Substring(1);
                methodInfo = handlerType.GetMethod(capitalizedName, BindingFlags.Public | BindingFlags.Instance | BindingFlags.IgnoreCase);
            }

            if (methodInfo == null)
            {
                return Fail(-32601, "Method not found", $"Unknown method: {method}");
            }

            // Get and normalize params (positional arrays, by-name objects, or a single scalar)
            request.TryGetValue("params", out var paramsObj);
            List<object?> positionalParams;
            try
            {
                positionalParams = NormalizeParams(ifaceName, funcName, paramsObj);
            }
            catch (ArgumentException ex)
            {
                return Fail(-32602, "Invalid params", ex.Message);
            }

            // Validate request using positional params - including count-0 requests (M9)
            if (_validateRequests)
            {
                try
                {
                    _contract.ValidateRequest(ifaceName, funcName, positionalParams);
                }
                catch (ArgumentException ex)
                {
                    return Fail(-32602, "Invalid params", ex.Message);
                }
            }

            // Convert positional params to named params using IDL signature
            var namedParams = PositionalToNamedParams(ifaceName, funcName, positionalParams);

            // Invoke handler method
            object? result;
            try
            {
                result = InvokeHandler(handler, methodInfo, namedParams);
            }
            catch (TargetInvocationException ex)
            {
                // Unwrap exception
                var innerEx = ex.InnerException ?? ex;
                if (innerEx is RPCError rpcErr)
                {
                    return Fail(rpcErr.Code, rpcErr.Message, rpcErr.Data);
                }
                LogInternalError($"Exception invoking {ifaceName}.{funcName}", innerEx);
                return Fail(-32603, "Internal error");
            }
            catch (RPCError rpcErr)
            {
                return Fail(rpcErr.Code, rpcErr.Message, rpcErr.Data);
            }
            catch (ArgumentException ex)
            {
                return Fail(-32602, "Invalid params", ex.Message);
            }
            catch (Exception ex)
            {
                LogInternalError($"Exception invoking {ifaceName}.{funcName}", ex);
                return Fail(-32603, "Internal error");
            }

            // Validate response if validation is enabled - null results for a
            // required return type must be rejected too (M14).
            if (_validateResponses)
            {
                try
                {
                    _contract.ValidateResponse(ifaceName, funcName, result);
                }
                catch (ArgumentException ex)
                {
                    LogInternalError($"Response validation failed for {ifaceName}.{funcName}", ex);
                    return Fail(-32603, "Internal error");
                }
            }

            // Don't respond to notifications
            if (isNotification)
            {
                return null;
            }

            return SuccessResponse(reqId, result);
        }

        /// <summary>
        /// Gets the IDL parameter definitions for a method.
        /// </summary>
        private List<Dictionary<string, object>> GetParamDefs(string ifaceName, string funcName)
        {
            var fn = _contract.GetInterface(ifaceName)?.GetFunction(funcName);
            if (fn == null || !fn.TryGetValue("parameters", out var pdObj) || pdObj is not System.Collections.IList pdList)
            {
                return new List<Dictionary<string, object>>();
            }
            return pdList.OfType<Dictionary<string, object>>().ToList();
        }

        /// <summary>
        /// Normalizes JSON-RPC params into a positional list using the IDL
        /// parameter names. By-name objects are reordered and unknown names are
        /// rejected so they cannot be silently dropped (M8).
        /// </summary>
        private List<object?> NormalizeParams(string ifaceName, string funcName, object? paramsObj)
        {
            if (paramsObj == null)
            {
                return new List<object?>();
            }

            if (paramsObj is System.Collections.IList list)
            {
                var result = new List<object?>();
                foreach (var p in list)
                {
                    result.Add(p);
                }
                return result;
            }

            if (paramsObj is System.Collections.IDictionary dict)
            {
                var paramDefs = GetParamDefs(ifaceName, funcName);
                var knownNames = new HashSet<string>(StringComparer.OrdinalIgnoreCase);
                foreach (var pd in paramDefs)
                {
                    if (pd.TryGetValue("name", out var nameObj) && nameObj != null)
                    {
                        knownNames.Add(nameObj.ToString()!);
                    }
                }

                foreach (var key in dict.Keys)
                {
                    var keyName = key?.ToString() ?? "";
                    if (!knownNames.Contains(keyName))
                    {
                        throw new ArgumentException($"Unknown parameter '{keyName}' for {ifaceName}.{funcName}");
                    }
                }

                var result = new List<object?>();
                foreach (var pd in paramDefs)
                {
                    var name = pd.TryGetValue("name", out var nameObj) ? nameObj?.ToString() ?? "" : "";
                    object? value = null;
                    if (!string.IsNullOrEmpty(name) && dict.Contains(name))
                    {
                        value = dict[name];
                    }
                    else
                    {
                        foreach (var key in dict.Keys)
                        {
                            if (string.Equals(key?.ToString(), name, StringComparison.OrdinalIgnoreCase))
                            {
                                value = dict[key!];
                                break;
                            }
                        }
                    }
                    result.Add(value);
                }
                return result;
            }

            // A single scalar param value.
            return new List<object?> { paramsObj };
        }

        /// <summary>
        /// Creates a JSON-RPC error response. Internal (not private) so the
        /// envelope shape can be asserted by tests.
        /// </summary>
        internal Dictionary<string, object> ErrorResponse(object? reqId, int code, string message, object? data)
        {
            var errObj = new Dictionary<string, object>
            {
                ["code"] = code,
                ["message"] = message
            };
            if (data != null)
            {
                errObj["data"] = data;
            }

            var response = new Dictionary<string, object>
            {
                ["jsonrpc"] = "2.0",
                ["error"] = errObj,
                // JSON-RPC 2.0 requires an id member; it is null when the
                // request id is unknown or invalid (M22/M25).
                ["id"] = reqId!
            };
            return response;
        }

        /// <summary>
        /// Creates a JSON-RPC success response. The id member is always present
        /// (null only when the request id is unknown), matching ErrorResponse and
        /// the D3 envelope shape. Internal (not private) for envelope tests.
        /// </summary>
        internal Dictionary<string, object> SuccessResponse(object? reqId, object? result)
        {
            var response = new Dictionary<string, object>
            {
                ["jsonrpc"] = "2.0",
                ["result"] = result!,
                ["id"] = reqId!
            };
            return response;
        }

        /// <summary>
        /// Parses "Interface.method" format
        /// </summary>
        private bool ParseMethodName(string method, out string ifaceName, out string funcName)
        {
            var lastDot = method.LastIndexOf('.');
            if (lastDot < 0)
            {
                ifaceName = "";
                funcName = "";
                return false;
            }
            ifaceName = method.Substring(0, lastDot);
            funcName = method.Substring(lastDot + 1);
            return true;
        }

        /// <summary>
        /// Converts positional params to named params using IDL signature
        /// </summary>
        private Dictionary<string, object?> PositionalToNamedParams(string ifaceName, string funcName,
            List<object?> positionalParams)
        {
            var result = new Dictionary<string, object?>();

            if (_contract == null)
            {
                for (int i = 0; i < positionalParams.Count; i++)
                {
                    result[$"p{i}"] = positionalParams[i];
                }
                return result;
            }

            var iface = _contract.GetInterface(ifaceName);
            if (iface == null)
            {
                for (int i = 0; i < positionalParams.Count; i++)
                {
                    result[$"p{i}"] = positionalParams[i];
                }
                return result;
            }

            var fn = iface.GetFunction(funcName);
            if (fn == null)
            {
                for (int i = 0; i < positionalParams.Count; i++)
                {
                    result[$"p{i}"] = positionalParams[i];
                }
                return result;
            }

            // Get parameter definitions
            var paramDefs = fn.TryGetValue("parameters", out var pdObj) && pdObj is System.Collections.IList pdList
                ? pdList.Cast<object>().ToList()
                : new List<object>();

            for (int i = 0; i < positionalParams.Count; i++)
            {
                if (i < paramDefs.Count && paramDefs[i] is Dictionary<string, object> paramDef)
                {
                    var paramName = paramDef.TryGetValue("name", out var pn) ? pn?.ToString() ?? $"p{i}" : $"p{i}";
                    result[paramName] = positionalParams[i];
                }
                else
                {
                    result[$"p{i}"] = positionalParams[i];
                }
            }

            return result;
        }

        /// <summary>
        /// Invokes a handler method with the given named parameters
        /// </summary>
        private object? InvokeHandler(object handler, MethodInfo methodInfo, Dictionary<string, object?> namedParams)
        {
            var parameters = methodInfo.GetParameters();
            var args = new List<object?>();

            for (int i = 0; i < parameters.Length; i++)
            {
                var paramName = parameters[i].Name ?? $"p{i}";

                // Try to find the parameter by various names
                object? arg = null;
                if (namedParams.TryGetValue(paramName, out arg))
                {
                    // Convert if necessary
                    if (arg != null && parameters[i].ParameterType.IsAssignableFrom(arg.GetType()))
                    {
                        args.Add(arg);
                        continue;
                    }
                }

                // Try lowercase first letter (Go style)
                if (paramName.Length > 0)
                {
                    var lowerName = char.ToLower(paramName[0]) + paramName.Substring(1);
                    if (namedParams.TryGetValue(lowerName, out arg))
                    {
                        args.Add(arg);
                        continue;
                    }
                }

                // Use default value
                args.Add(parameters[i].DefaultValue);
            }

            var result = methodInfo.Invoke(handler, args.ToArray());

            // Check if method returns an error
            if (methodInfo.ReturnType is Type rt && rt.IsGenericType)
            {
                // For methods returning (result, error) tuples
            }

            return result;
        }
    }
}

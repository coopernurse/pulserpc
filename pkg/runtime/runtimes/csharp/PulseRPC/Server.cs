using System;
using System.Collections;
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
        // Per interface: case-insensitive name -> method plus its cached parameters.
        private readonly Dictionary<string, Dictionary<string, (MethodInfo Method, ParameterInfo[] Parameters)>>
            _handlerMethods = new Dictionary<string, Dictionary<string, (MethodInfo Method, ParameterInfo[] Parameters)>>();
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

            // Resolve reflection once at registration. The case-insensitive map
            // covers both the wire name and the capitalized C# alias, and caches
            // ParameterInfo[] so invocation never re-queries it.
            var methods = new Dictionary<string, (MethodInfo Method, ParameterInfo[] Parameters)>(
                StringComparer.OrdinalIgnoreCase);
            foreach (var m in handler.GetType().GetMethods(BindingFlags.Public | BindingFlags.Instance))
            {
                if (!methods.ContainsKey(m.Name))
                {
                    methods[m.Name] = (m, m.GetParameters());
                }
            }
            _handlerMethods[ifaceName] = methods;
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

            // Check for method. An invalid method makes the request envelope
            // invalid, so the id cannot be echoed back: reply with a null id.
            if (!request.TryGetValue("method", out var methodObj) || methodObj is not string method || string.IsNullOrEmpty(method))
            {
                return isNotification ? null : ErrorResponse(null, -32600, "Invalid Request", "Method must be a string");
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

            // Get function from handler using the cache built at AddHandler
            if (!_handlerMethods.TryGetValue(ifaceName, out var handlerMethods)
                || !handlerMethods.TryGetValue(funcName, out var resolvedMethod))
            {
                return Fail(-32601, "Method not found", $"Unknown method: {method}");
            }

            // Resolve the IDL entries once and share them with normalization,
            // validation and parameter naming.
            var iface = _contract.GetInterface(ifaceName);
            var fn = iface?.GetFunction(funcName);
            IList paramDefs = fn != null && fn.TryGetValue("parameters", out var pdObj) && pdObj is IList pdList
                ? pdList
                : new ArrayList();

            // Get and normalize params (positional arrays, by-name objects, or a single scalar)
            request.TryGetValue("params", out var paramsObj);
            List<object?> positionalParams;
            try
            {
                positionalParams = NormalizeParams(ifaceName, funcName, paramDefs, paramsObj);
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
                    _contract.ValidateRequest(ifaceName, funcName, positionalParams, iface, fn, paramDefs);
                }
                catch (RPCError rpcErr)
                {
                    return Fail(rpcErr.Code, rpcErr.Message, rpcErr.Data);
                }
                catch (ArgumentException ex)
                {
                    return Fail(-32602, "Invalid params", ex.Message);
                }
            }

            // Convert positional params to named params using IDL signature
            var namedParams = PositionalToNamedParams(paramDefs, positionalParams);

            // Invoke handler method
            object? result;
            try
            {
                result = InvokeHandler(handler, resolvedMethod, namedParams);
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
                    _contract.ValidateResponse(ifaceName, funcName, result, iface, fn);
                }
                catch (Exception ex)
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
        /// Normalizes JSON-RPC params into a positional list using the IDL
        /// parameter names. By-name objects are reordered and unknown names are
        /// rejected so they cannot be silently dropped (M8).
        /// </summary>
        private List<object?> NormalizeParams(string ifaceName, string funcName, IList paramDefs, object? paramsObj)
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
                // One case-insensitive lookup serves both unknown-key rejection
                // and value resolution, replacing the old O(paramCount * keyCount)
                // scan. First occurrence wins for names differing only by case,
                // matching the previous loop's break-on-first-match behavior.
                var byName = new Dictionary<string, object?>(StringComparer.OrdinalIgnoreCase);
                foreach (var key in dict.Keys)
                {
                    var keyName = key?.ToString() ?? "";
                    if (!byName.ContainsKey(keyName))
                    {
                        byName[keyName] = dict[key!];
                    }
                }

                var knownNames = new HashSet<string>(StringComparer.OrdinalIgnoreCase);
                foreach (var pd in paramDefs)
                {
                    if (pd is Dictionary<string, object> pdDict
                        && pdDict.TryGetValue("name", out var nameObj) && nameObj != null)
                    {
                        knownNames.Add(nameObj.ToString()!);
                    }
                }

                foreach (var key in byName.Keys)
                {
                    if (!knownNames.Contains(key))
                    {
                        throw new ArgumentException($"Unknown parameter '{key}' for {ifaceName}.{funcName}");
                    }
                }

                var result = new List<object?>();
                foreach (var pd in paramDefs)
                {
                    if (pd is not Dictionary<string, object> paramDef)
                    {
                        result.Add(null);
                        continue;
                    }
                    var name = paramDef.TryGetValue("name", out var nameObj) ? nameObj?.ToString() ?? "" : "";
                    object? value = null;
                    // Exact match still takes precedence over case-insensitive.
                    if (!string.IsNullOrEmpty(name) && dict.Contains(name))
                    {
                        value = dict[name];
                    }
                    else if (!string.IsNullOrEmpty(name) && byName.TryGetValue(name, out var matched))
                    {
                        value = matched;
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
        private Dictionary<string, object?> PositionalToNamedParams(IList paramDefs,
            List<object?> positionalParams)
        {
            var result = new Dictionary<string, object?>();

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
        private object? InvokeHandler(object handler,
            (MethodInfo Method, ParameterInfo[] Parameters) resolved,
            Dictionary<string, object?> namedParams)
        {
            var parameters = resolved.Parameters;
            var args = new object?[parameters.Length];

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
                        args[i] = arg;
                        continue;
                    }
                }

                // Try lowercase first letter (Go style)
                if (paramName.Length > 0)
                {
                    var lowerName = char.ToLower(paramName[0]) + paramName.Substring(1);
                    if (namedParams.TryGetValue(lowerName, out arg))
                    {
                        args[i] = arg;
                        continue;
                    }
                }

                // Use default value
                args[i] = parameters[i].DefaultValue;
            }

            var result = resolved.Method.Invoke(handler, args);

            // Check if method returns an error
            if (resolved.Method.ReturnType is Type rt && rt.IsGenericType)
            {
                // For methods returning (result, error) tuples
            }

            return result;
        }
    }
}

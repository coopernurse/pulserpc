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
        /// Creates a new Server instance
        /// </summary>
        public Server(Contract contract, bool validateRequests = true, bool validateResponses = true)
        {
            _contract = contract;
            _validateRequests = validateRequests;
            _validateResponses = validateResponses;
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
            // Validate request format
            if (request == null)
            {
                return ErrorResponse(null, -32600, "Invalid Request", "Request must be an object");
            }

            // Check JSON-RPC version
            if (!request.TryGetValue("jsonrpc", out var jsonrpc) || jsonrpc?.ToString() != "2.0")
            {
                return ErrorResponse(request.TryGetValue("id", out var id) ? id : null, -32600, "Invalid Request",
                    "jsonrpc version must be '2.0'");
            }

            // Check for method
            if (!request.TryGetValue("method", out var methodObj) || methodObj is not string method || string.IsNullOrEmpty(method))
            {
                return ErrorResponse(request.TryGetValue("id", out var id) ? id : null, -32600, "Invalid Request",
                    "Method must be a string");
            }

            // Handle pulserpc-idl request
            if (method == "pulserpc-idl")
            {
                return SuccessResponse(request.TryGetValue("id", out var reqIdForIdl) ? reqIdForIdl : null, _contract.IdlParsed);
            }

            // Check for notification (no 'id' means no response expected)
            var reqId = request.TryGetValue("id", out var idVal) ? idVal : null;
            var isNotification = reqId == null;

            // Parse method name (e.g., "UserService.getUser")
            if (!ParseMethodName(method, out var ifaceName, out var funcName))
            {
                return ErrorResponse(reqId, -32601, "Method not found", $"Invalid method name format: {method}");
            }

            // Look up handler
            if (!_handlers.TryGetValue(ifaceName, out var handler))
            {
                return ErrorResponse(reqId, -32601, "Method not found", $"Unknown interface: {ifaceName}");
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
                return ErrorResponse(reqId, -32601, "Method not found", $"Unknown method: {method}");
            }

            // Get params
            request.TryGetValue("params", out var paramsObj);

            // Convert params to list if it's a list
            var positionalParams = new List<object?>();
            if (paramsObj is System.Collections.IList paramsList)
            {
                foreach (var p in paramsList)
                {
                    positionalParams.Add(p);
                }
            }
            else if (paramsObj == null)
            {
                positionalParams = new List<object?>();
            }

            // Validate request using positional params
            if (_validateRequests && positionalParams.Count > 0)
            {
                try
                {
                    _contract.ValidateRequest(ifaceName, funcName, positionalParams);
                }
                catch (ArgumentException ex)
                {
                    return ErrorResponse(reqId, -32602, "Invalid params", ex.Message);
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
                    return ErrorResponse(reqId, rpcErr.Code, rpcErr.Message, rpcErr.Data);
                }
                return ErrorResponse(reqId, -32603, "Internal error", innerEx.Message);
            }
            catch (RPCError rpcErr)
            {
                return ErrorResponse(reqId, rpcErr.Code, rpcErr.Message, rpcErr.Data);
            }
            catch (ArgumentException ex)
            {
                return ErrorResponse(reqId, -32602, "Invalid params", ex.Message);
            }
            catch (Exception ex)
            {
                return ErrorResponse(reqId, -32603, "Internal error", ex.Message);
            }

            // Validate response if validation is enabled
            if (_validateResponses && result != null)
            {
                try
                {
                    _contract.ValidateResponse(ifaceName, funcName, result);
                }
                catch (ArgumentException ex)
                {
                    return ErrorResponse(reqId, -32603, "Internal error",
                        $"Response validation failed: {ex.Message}");
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
        /// Creates a JSON-RPC error response
        /// </summary>
        private Dictionary<string, object> ErrorResponse(object? reqId, int code, string message, object? data)
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
                ["error"] = errObj
            };
            if (reqId != null)
            {
                response["id"] = reqId;
            }
            return response;
        }

        /// <summary>
        /// Creates a JSON-RPC success response
        /// </summary>
        private Dictionary<string, object> SuccessResponse(object? reqId, object? result)
        {
            var response = new Dictionary<string, object>
            {
                ["jsonrpc"] = "2.0",
                ["result"] = result!
            };
            if (reqId != null)
            {
                response["id"] = reqId;
            }
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

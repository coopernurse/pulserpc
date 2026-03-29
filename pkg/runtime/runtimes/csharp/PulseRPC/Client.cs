using System;
using System.Collections.Generic;
using System.Dynamic;
using System.Linq;

namespace PulseRPC
{
    /// <summary>
    /// Dynamic proxy for an interface that provides callable methods
    /// </summary>
    public class InterfaceClientProxy : DynamicObject
    {
        private readonly Client _client;
        private readonly Interface _iface;
        private readonly string _ifaceName;
        private readonly Dictionary<string, Func<object?, object?>> _methods = new Dictionary<string, Func<object?, object?>>();

        public InterfaceClientProxy(Client client, Interface iface)
        {
            _client = client;
            _iface = iface;
            _ifaceName = iface.Name;

            // Create method callables for each function
            foreach (var funcName in iface.Functions.Keys)
            {
                var capturedFuncName = funcName;
                _methods[funcName] = (parms) => _client.Call(_ifaceName + "." + capturedFuncName, parms);
            }
        }

        /// <summary>
        /// Calls a method on the interface
        /// </summary>
        public object? Call(string funcName, params object[] args)
        {
            if (_methods.TryGetValue(funcName, out var method))
            {
                if (args == null || args.Length == 0)
                {
                    return method(null);
                }
                else if (args.Length == 1)
                {
                    return method(args[0]);
                }
                else
                {
                    return method(args);
                }
            }

            throw new InvalidOperationException($"Method not found: {funcName}");
        }

        /// <summary>
        /// Gets a method by name for direct access
        /// </summary>
        public Func<object?, object?>? GetMethod(string funcName)
        {
            return _methods.TryGetValue(funcName, out var method) ? method : null;
        }

        /// <summary>
        /// Lists all available method names
        /// </summary>
        public IEnumerable<string> ListMethods() => _methods.Keys;

        // DynamicObject implementation for dynamic access
        public override bool TryGetMember(GetMemberBinder binder, out object? result)
        {
            if (_methods.TryGetValue(binder.Name, out var method))
            {
                result = method;
                return true;
            }
            result = null;
            return false;
        }

        public override bool TryInvokeMember(InvokeMemberBinder binder, object?[]? args, out object? result)
        {
            if (_methods.TryGetValue(binder.Name, out var method))
            {
                if (args == null || args.Length == 0)
                {
                    result = method(null);
                    return true;
                }
                else if (args.Length == 1)
                {
                    result = method(args[0]);
                    return true;
                }
                else
                {
                    result = method(args);
                    return true;
                }
            }
            result = null;
            return false;
        }
    }

    /// <summary>
    /// JSON-RPC 2.0 client with automatic interface discovery
    /// </summary>
    public class Client
    {
        private readonly Transport _transport;
        private readonly bool _validateRequest;
        private readonly bool _validateResponse;
        private int _requestID;
        private readonly Dictionary<string, InterfaceClientProxy> _ifaces = new Dictionary<string, InterfaceClientProxy>();

        public Contract? Contract { get; private set; }

        /// <summary>
        /// Creates a new Client and bootstraps by fetching IDL from server
        /// </summary>
        public Client(Transport transport, bool validateRequest = false, bool validateResponse = false)
        {
            _transport = transport;
            _validateRequest = validateRequest;
            _validateResponse = validateResponse;

            // Bootstrap: fetch IDL from server
            Bootstrap();
        }

        /// <summary>
        /// Gets an interface proxy by name
        /// </summary>
        public InterfaceClientProxy? GetInterface(string ifaceName)
        {
            return _ifaces.TryGetValue(ifaceName, out var proxy) ? proxy : null;
        }

        /// <summary>
        /// Gets an interface proxy using dynamic access
        /// </summary>
        public dynamic Interfaces => new InterfaceProxyAccessor(this);

        /// <summary>
        /// Makes a JSON-RPC call
        /// </summary>
        public object? Call(string method, object? @params = null)
        {
            // Validate request if enabled
            if (_validateRequest && Contract != null)
            {
                var parts = ParseMethodName(method);
                if (parts != null)
                {
                    var paramList = ConvertParamsToList(parts.Value.ifaceName, parts.Value.funcName, @params);
                    if (paramList != null)
                    {
                        try
                        {
                            Contract.ValidateRequest(parts.Value.ifaceName, parts.Value.funcName, paramList);
                        }
                        catch (ArgumentException ex)
                        {
                            throw new Exception($"Request validation failed: {ex.Message}", ex);
                        }
                    }
                }
            }

            // Generate request ID
            _requestID++;
            var reqID = _requestID;

            // Build request
            var request = new Dictionary<string, object?>
            {
                ["jsonrpc"] = "2.0",
                ["method"] = method,
                ["id"] = reqID
            };

            if (@params != null)
            {
                request["params"] = @params;
            }

            // Send request via transport
            var response = _transport.Request(method, @params);

            // Check for error response
            if (response.TryGetValue("error", out var errorObj) && errorObj is Dictionary<string, object> errDict)
            {
                var code = errDict.TryGetValue("code", out var codeObj) ? Convert.ToInt32(codeObj) : -32603;
                var message = errDict.TryGetValue("message", out var msgObj) ? msgObj?.ToString() ?? "Unknown error" : "Unknown error";
                var data = errDict.TryGetValue("data", out var dataObj) ? dataObj : null;
                throw new RPCError(code, message!, data);
            }

            // Get result
            var result = response.TryGetValue("result", out var resultObj) ? resultObj : null;

            // Validate response if enabled
            if (_validateResponse && Contract != null && result != null)
            {
                var parts = ParseMethodName(method);
                if (parts != null)
                {
                    try
                    {
                        Contract.ValidateResponse(parts.Value.ifaceName, parts.Value.funcName, result);
                    }
                    catch (ArgumentException ex)
                    {
                        throw new Exception($"Response validation failed: {ex.Message}", ex);
                    }
                }
            }

            return result;
        }

        /// <summary>
        /// Sends a notification (no response expected)
        /// </summary>
        public void Notify(string method, object? @params = null)
        {
            // Ignore errors for notifications
            try
            {
                Call(method, @params);
            }
            catch
            {
                // Ignore
            }
        }

        /// <summary>
        /// Bootstrap: fetch IDL from server and create interface proxies
        /// </summary>
        private void Bootstrap()
        {
            // Make request to get IDL
            var response = _transport.Request("pulserpc-idl", null);

            // Check for error
            if (response.TryGetValue("error", out var errorObj) && errorObj is Dictionary<string, object> errDict)
            {
                throw new Exception($"Failed to fetch IDL: {errDict["message"]}");
            }

            // Get IDL result
            if (!response.TryGetValue("result", out var idlJSON) || idlJSON == null)
            {
                throw new Exception("Server returned empty IDL");
            }

            // Create contract
            Contract = new Contract(idlJSON);

            // Create interface proxies
            foreach (var kvp in Contract.Interfaces)
            {
                var proxy = new InterfaceClientProxy(this, kvp.Value);
                _ifaces[kvp.Key] = proxy;
            }
        }

        /// <summary>
        /// Parses "Interface.method" format
        /// </summary>
        private (string ifaceName, string funcName)? ParseMethodName(string method)
        {
            var lastDot = method.LastIndexOf('.');
            if (lastDot < 0)
            {
                return null;
            }
            return (method.Substring(0, lastDot), method.Substring(lastDot + 1));
        }

        /// <summary>
        /// Converts params to positional list for validation
        /// </summary>
        private List<object?>? ConvertParamsToList(string ifaceName, string funcName, object? @params)
        {
            if (Contract == null) return null;

            var iface = Contract.GetInterface(ifaceName);
            if (iface == null) return null;

            var fn = iface.GetFunction(funcName);
            if (fn == null) return null;

            var paramDefs = fn.TryGetValue("parameters", out var pdObj) && pdObj is System.Collections.IList pdList
                ? pdList.Cast<object>().ToList()
                : new List<object>();

            var result = new List<object?>();

            if (@params is System.Collections.IList paramList)
            {
                foreach (var p in paramList)
                {
                    result.Add(p);
                }
            }
            else if (@params is Dictionary<string, object> namedParams)
            {
                foreach (var paramDef in paramDefs)
                {
                    if (paramDef is Dictionary<string, object> pd)
                    {
                        var name = pd.TryGetValue("name", out var pn) ? pn?.ToString() ?? "" : "";
                        result.Add(namedParams.TryGetValue(name, out var v) ? v : null);
                    }
                    else
                    {
                        result.Add(null);
                    }
                }
            }
            else if (@params != null)
            {
                result.Add(@params);
            }

            return result;
        }
    }

    /// <summary>
    /// Helper class for dynamic interface access
    /// </summary>
    public class InterfaceProxyAccessor
    {
        private readonly Client _client;

        public InterfaceProxyAccessor(Client client)
        {
            _client = client;
        }

        public InterfaceClientProxy? Get(string ifaceName)
        {
            return _client.GetInterface(ifaceName);
        }
    }
}

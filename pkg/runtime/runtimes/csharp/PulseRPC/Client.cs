using System;
using System.Collections.Generic;
using System.Dynamic;
using System.Linq;
using System.Text.Json;
using System.Threading;
using System.Threading.Tasks;

namespace PulseRPC
{
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

            foreach (var funcName in iface.Functions.Keys)
            {
                var capturedFuncName = funcName;
                _methods[funcName] = (parms) => _client.Call(_ifaceName + "." + capturedFuncName, parms);
            }
        }

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

        public Func<object?, object?>? GetMethod(string funcName)
        {
            return _methods.TryGetValue(funcName, out var method) ? method : null;
        }

        public IEnumerable<string> ListMethods() => _methods.Keys;

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

    public class Client
    {
        private readonly ITransport _transport;
        private readonly bool _validateRequest;
        private readonly bool _validateResponse;
        private int _requestID;
        private readonly Dictionary<string, InterfaceClientProxy> _ifaces = new Dictionary<string, InterfaceClientProxy>();
        private IContractAuditor? _auditor;
        private bool _verifyOnBootstrap;
        private object? _localIDL;

        public Contract? Contract { get; private set; }

        public Client(ITransport transport, bool validateRequest = false, bool validateResponse = false)
        {
            _transport = transport;
            _validateRequest = validateRequest;
            _validateResponse = validateResponse;

            Bootstrap();
        }

        private Client(ITransport transport, bool validateRequest, bool validateResponse, ClientOptions options)
        {
            _transport = transport;
            _validateRequest = validateRequest;
            _validateResponse = validateResponse;
            _auditor = options.GetAuditor();
            _verifyOnBootstrap = options.GetVerifyOnBootstrap();

            Bootstrap();

            if (_verifyOnBootstrap)
            {
                // GetAwaiter().GetResult() surfaces the auditor's original exception
                // instead of wrapping it in an AggregateException.
                VerifyCompatibility(CancellationToken.None).GetAwaiter().GetResult();
            }
        }

        public static Client Create(ITransport transport, bool validateRequest = false, bool validateResponse = false, ClientOptions? options = null)
        {
            if (options != null)
            {
                return new Client(transport, validateRequest, validateResponse, options);
            }
            return new Client(transport, validateRequest, validateResponse);
        }

        public static ClientOptions Options => new ClientOptions();

        public InterfaceClientProxy? GetInterface(string ifaceName)
        {
            return _ifaces.TryGetValue(ifaceName, out var proxy) ? proxy : null;
        }

        public dynamic Interfaces => new InterfaceProxyAccessor(this);

        public object? Call(string method, object? @params = null)
        {
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

            var reqID = NextRequestId();
            var responseRaw = _transport.CallAsync(method, @params, reqID).GetAwaiter().GetResult();
            var response = NormalizeResponse(responseRaw)
                ?? throw new InvalidOperationException($"Call({method}): transport returned an empty response");

            ValidateEnvelope(response, reqID, $"Call({method})");

            if (response.TryGetValue("error", out var errorObj))
            {
                throw ParseRpcError(errorObj);
            }

            var result = response["result"];

            if (_validateResponse && Contract != null)
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

        public void Notify(string method, object? @params = null)
        {
            try
            {
                Call(method, @params);
            }
            catch
            {
            }
        }

        private void Bootstrap()
        {
            var reqID = NextRequestId();
            var responseRaw = _transport.CallAsync("pulserpc-idl", null, reqID).GetAwaiter().GetResult();
            var response = NormalizeResponse(responseRaw)
                ?? throw new Exception("Server returned empty IDL response");

            ValidateEnvelope(response, reqID, "Bootstrap");

            if (response.TryGetValue("error", out var errorObj))
            {
                var err = ParseRpcError(errorObj);
                throw new Exception($"Failed to fetch IDL: {err.Message}");
            }

            if (!response.TryGetValue("result", out var idlJSON) || idlJSON == null)
            {
                throw new Exception("Server returned empty IDL");
            }

            Contract = new Contract(idlJSON);

            foreach (var kvp in Contract.Interfaces)
            {
                var proxy = new InterfaceClientProxy(this, kvp.Value);
                _ifaces[kvp.Key] = proxy;
            }
        }

        private (string ifaceName, string funcName)? ParseMethodName(string method)
        {
            var lastDot = method.LastIndexOf('.');
            if (lastDot < 0)
            {
                return null;
            }
            return (method.Substring(0, lastDot), method.Substring(lastDot + 1));
        }

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

        public async Task<VerificationResult> VerifyCompatibility(CancellationToken cancellationToken = default)
        {
            object? clientIDL;
            if (_localIDL != null)
            {
                clientIDL = _localIDL;
            }
            else if (Contract != null)
            {
                clientIDL = Contract.GetIDLParsed();
            }
            else
            {
                clientIDL = null;
            }

            // Fetch the running server's contract on every verification instead of
            // diffing the bootstrap contract against itself (M15).
            object? serverIDL = null;
            string? fetchError = null;
            try
            {
                var reqID = NextRequestId();
                var responseRaw = await _transport.CallAsync("pulserpc-idl", null, reqID);
                var response = NormalizeResponse(responseRaw);
                if (response == null)
                {
                    fetchError = "empty response from pulserpc-idl";
                }
                else if (response.TryGetValue("error", out var errObj))
                {
                    fetchError = "server returned error: " + errObj;
                }
                else if (!response.TryGetValue("result", out var idlResult) || idlResult == null)
                {
                    fetchError = "server IDL is empty";
                }
                else
                {
                    serverIDL = idlResult;
                }
            }
            catch (Exception ex)
            {
                fetchError = "failed to fetch server IDL: " + ex.Message;
            }

            if (clientIDL == null)
            {
                fetchError ??= "client IDL is empty";
            }

            if (fetchError != null || serverIDL == null)
            {
                var failed = new VerificationResult
                {
                    Compatible = false,
                    ClientChecksum = ExtractChecksum(clientIDL),
                    Deltas = Array.Empty<ContractDelta>(),
                    Error = fetchError ?? "server IDL is empty",
                    Timestamp = DateTime.UtcNow
                };
                _auditor?.Audit(failed);
                return failed;
            }

            var deltas = DiffEngine.DiffIDL(clientIDL!, serverIDL);

            var compatible = true;
            foreach (var delta in deltas)
            {
                if (delta.Severity == Severity.Error)
                {
                    compatible = false;
                    break;
                }
            }

            var result = new VerificationResult
            {
                Compatible = compatible,
                ServerChecksum = ExtractChecksum(serverIDL),
                ClientChecksum = ExtractChecksum(clientIDL),
                Deltas = deltas,
                Timestamp = DateTime.UtcNow
            };

            _auditor?.Audit(result);

            return result;
        }

        private object NextRequestId()
        {
            _requestID++;
            return _requestID;
        }

        /// <summary>
        /// Normalizes transport responses ( dictionaries or JsonElement) into a
        /// single dictionary shape with plain CLR values.
        /// </summary>
        private static Dictionary<string, object?>? NormalizeResponse(object? response)
        {
            if (response is Dictionary<string, object?> dict)
            {
                return dict;
            }
            if (response is Dictionary<string, object> dict2)
            {
                return dict2.ToDictionary(kvp => kvp.Key, kvp => (object?)kvp.Value);
            }
            if (response is JsonElement element && element.ValueKind == JsonValueKind.Object)
            {
                var result = new Dictionary<string, object?>();
                foreach (var prop in element.EnumerateObject())
                {
                    result[prop.Name] = ConvertJsonElement(prop.Value);
                }
                return result;
            }
            return null;
        }

        private static object? ConvertJsonElement(JsonElement element)
        {
            switch (element.ValueKind)
            {
                case JsonValueKind.Object:
                    var dict = new Dictionary<string, object?>();
                    foreach (var prop in element.EnumerateObject())
                    {
                        dict[prop.Name] = ConvertJsonElement(prop.Value);
                    }
                    return dict;
                case JsonValueKind.Array:
                    var list = new List<object?>();
                    foreach (var item in element.EnumerateArray())
                    {
                        list.Add(ConvertJsonElement(item));
                    }
                    return list;
                case JsonValueKind.String:
                    return element.GetString();
                case JsonValueKind.Number:
                    if (element.TryGetInt64(out var longValue))
                    {
                        return longValue;
                    }
                    return element.GetDouble();
                case JsonValueKind.True:
                    return true;
                case JsonValueKind.False:
                    return false;
                default:
                    return null;
            }
        }

        private static RPCError ParseRpcError(object? errorObj)
        {
            var err = NormalizeResponse(errorObj)
                ?? throw new InvalidOperationException("Server error object is malformed");
            var code = err.TryGetValue("code", out var codeObj) ? Convert.ToInt32(codeObj) : -32603;
            var message = err.TryGetValue("message", out var msgObj)
                ? msgObj?.ToString() ?? "Unknown error"
                : "Unknown error";
            err.TryGetValue("data", out var data);
            return new RPCError(code, message!, data);
        }

        private static void ValidateEnvelope(Dictionary<string, object?> response, object expectedId, string operation)
        {
            if (!response.TryGetValue("jsonrpc", out var jsonrpc) || jsonrpc?.ToString() != "2.0")
            {
                throw new InvalidOperationException($"{operation}: response is missing a valid jsonrpc member");
            }
            if (!response.TryGetValue("id", out var idObj))
            {
                throw new InvalidOperationException($"{operation}: response is missing an id member");
            }
            if (!string.Equals(idObj?.ToString(), expectedId?.ToString(), StringComparison.Ordinal))
            {
                throw new InvalidOperationException(
                    $"{operation}: response id '{idObj}' does not match request id '{expectedId}'");
            }
            var hasResult = response.ContainsKey("result");
            var hasError = response.ContainsKey("error");
            if (hasResult == hasError)
            {
                throw new InvalidOperationException(
                    $"{operation}: response must contain exactly one of 'result' or 'error'");
            }
        }

        private static string ExtractChecksum(object? idl)
        {
            if (idl is Dictionary<string, object> dict)
            {
                if (dict.TryGetValue("checksum", out var checksumObj))
                {
                    if (checksumObj is string checksum)
                    {
                        return checksum;
                    }
                    // SetLocalIDL deserializes values as JsonElement.
                    if (checksumObj is JsonElement value && value.ValueKind == JsonValueKind.String)
                    {
                        return value.GetString() ?? string.Empty;
                    }
                }
            }
            else if (idl is JsonElement element && element.ValueKind == JsonValueKind.Object)
            {
                if (element.TryGetProperty("checksum", out var checksumProp) && checksumProp.ValueKind == JsonValueKind.String)
                {
                    return checksumProp.GetString() ?? string.Empty;
                }
            }
            return string.Empty;
        }

        public void SetLocalIDL(string idlJson)
        {
            // Deserialize to a dictionary (not a bare JsonElement) so DiffEngine
            // can extract interfaces/structs from the root object.
            _localIDL = JsonSerializer.Deserialize<Dictionary<string, object>>(idlJson);
        }

        public class ClientOptions
        {
            private IContractAuditor? _auditor;
            private bool _verifyOnBootstrap;

            public ClientOptions WithAuditor(IContractAuditor auditor)
            {
                _auditor = auditor;
                return this;
            }

            public ClientOptions VerifyOnBootstrap()
            {
                _verifyOnBootstrap = true;
                return this;
            }

            internal IContractAuditor? GetAuditor() => _auditor;
            internal bool GetVerifyOnBootstrap() => _verifyOnBootstrap;
        }
    }

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
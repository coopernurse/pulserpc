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
        private readonly Transport _transport;
        private readonly bool _validateRequest;
        private readonly bool _validateResponse;
        private int _requestID;
        private readonly Dictionary<string, InterfaceClientProxy> _ifaces = new Dictionary<string, InterfaceClientProxy>();
        private IContractAuditor? _auditor;
        private bool _verifyOnBootstrap;
        private object? _localIDL;

        public Contract? Contract { get; private set; }

        public Client(Transport transport, bool validateRequest = false, bool validateResponse = false)
        {
            _transport = transport;
            _validateRequest = validateRequest;
            _validateResponse = validateResponse;

            Bootstrap();
        }

        private Client(Transport transport, bool validateRequest, bool validateResponse, ClientOptions options)
        {
            _transport = transport;
            _validateRequest = validateRequest;
            _validateResponse = validateResponse;
            _auditor = options.GetAuditor();
            _verifyOnBootstrap = options.GetVerifyOnBootstrap();

            Bootstrap();

            if (_verifyOnBootstrap)
            {
                VerifyCompatibility(CancellationToken.None).Wait();
            }
        }

        public static Client Create(Transport transport, bool validateRequest = false, bool validateResponse = false, ClientOptions? options = null)
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

            _requestID++;
            var reqID = _requestID;

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

            var response = _transport.Request(method, @params);

            if (response.TryGetValue("error", out var errorObj) && errorObj is Dictionary<string, object> errDict)
            {
                var code = errDict.TryGetValue("code", out var codeObj) ? Convert.ToInt32(codeObj) : -32603;
                var message = errDict.TryGetValue("message", out var msgObj) ? msgObj?.ToString() ?? "Unknown error" : "Unknown error";
                var data = errDict.TryGetValue("data", out var dataObj) ? dataObj : null;
                throw new RPCError(code, message!, data);
            }

            var result = response.TryGetValue("result", out var resultObj) ? resultObj : null;

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
            var response = _transport.Request("pulserpc-idl", null);

            if (response.TryGetValue("error", out var errorObj) && errorObj is Dictionary<string, object> errDict)
            {
                throw new Exception($"Failed to fetch IDL: {errDict["message"]}");
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

            object? serverIDL = Contract?.GetIDLParsed();

            var deltas = new List<ContractDelta>();
            if (clientIDL != null && serverIDL != null)
            {
                deltas = DiffEngine.DiffIDL(clientIDL, serverIDL);
            }

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
                ServerChecksum = serverIDL != null ? DiffEngine.ComputeChecksum(serverIDL) : string.Empty,
                ClientChecksum = clientIDL != null ? DiffEngine.ComputeChecksum(clientIDL) : string.Empty,
                Deltas = deltas,
                Timestamp = DateTime.UtcNow
            };

            if (_auditor != null)
            {
                _auditor.Audit(result);
            }

            return result;
        }

        public void SetLocalIDL(string idlJson)
        {
            _localIDL = JsonSerializer.Deserialize<object>(idlJson);
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
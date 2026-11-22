using System;
using System.Collections.Generic;
using System.Net.Http;
using System.Text;
using System.Threading.Tasks;

namespace PulseRPC
{
    /// <summary>
    /// The single transport abstraction shared by the runtime client and the
    /// generated clients. The id-aware overload lets the caller own the
    /// JSON-RPC request id so the response id can be matched on return.
    /// </summary>
    public interface ITransport
    {
        /// <summary>
        /// Sends a JSON-RPC request using a transport-generated id (the API used
        /// by generated clients).
        /// </summary>
        Task<Dictionary<string, object?>> CallAsync(string method, object[] parameters);

        /// <summary>
        /// Sends a JSON-RPC request with a caller-supplied id. Implementations
        /// must put that id in the request so the response can be matched.
        /// </summary>
        Task<Dictionary<string, object?>> CallAsync(string method, object? @params, object id)
            => CallAsync(method, @params as object[] ?? Array.Empty<object>());
    }

    /// <summary>
    /// Legacy synchronous transport base. It now implements <see cref="ITransport"/>
    /// so there is a single abstraction; the synchronous <see cref="Request"/> is
    /// kept for backwards compatibility.
    /// </summary>
    public abstract class Transport : ITransport
    {
        /// <summary>
        /// Sends a JSON-RPC request and returns the response
        /// </summary>
        public abstract Dictionary<string, object> Request(string method, object? @params);

        /// <summary>
        /// Id-aware variant of <see cref="Request"/>. Legacy transports that cannot
        /// carry the id keep delegating to the two-argument overload.
        /// </summary>
        public virtual Dictionary<string, object> Request(string method, object? @params, object id)
            => Request(method, @params);

        public Task<Dictionary<string, object?>> CallAsync(string method, object[] parameters)
            => CallAsync(method, parameters, Guid.NewGuid().ToString());

        public Task<Dictionary<string, object?>> CallAsync(string method, object? @params, object id)
        {
            var response = Request(method, @params, id);
            return Task.FromResult<Dictionary<string, object?>>(response!);
        }
    }

    /// <summary>
    /// HTTP transport implementation
    /// </summary>
    public class HttpTransport : ITransport
    {
        private static readonly System.Text.Json.JsonSerializerOptions _jsonOptions = new System.Text.Json.JsonSerializerOptions
        {
            PropertyNamingPolicy = System.Text.Json.JsonNamingPolicy.CamelCase
        };

        static HttpTransport()
        {
            _jsonOptions.Converters.Add(new System.Text.Json.Serialization.JsonStringEnumConverter());
        }

        private readonly HttpClient _httpClient;
        private readonly string _baseUrl;

        public HttpTransport(string baseUrl, Dictionary<string, string>? headers = null)
        {
            _baseUrl = baseUrl.TrimEnd('/');
            _httpClient = new HttpClient();
            if (headers != null)
            {
                foreach (var header in headers)
                {
                    _httpClient.DefaultRequestHeaders.Add(header.Key, header.Value);
                }
            }
        }

        public Task<Dictionary<string, object?>> CallAsync(string method, object[] parameters)
            => CallAsync(method, parameters, Guid.NewGuid().ToString());

        public async Task<Dictionary<string, object?>> CallAsync(string method, object? @params, object id)
        {
            var request = new Dictionary<string, object?>
            {
                { "jsonrpc", "2.0" },
                { "method", method },
                { "id", id }
            };
            if (@params != null)
            {
                request["params"] = @params;
            }

            var json = System.Text.Json.JsonSerializer.Serialize(request, _jsonOptions);
            var content = new StringContent(json, Encoding.UTF8, "application/json");

            var response = await _httpClient.PostAsync(_baseUrl, content);
            response.EnsureSuccessStatusCode();

            var responseJson = await response.Content.ReadAsStringAsync();
            var responseDict = System.Text.Json.JsonSerializer.Deserialize<Dictionary<string, object?>>(responseJson);

            // Validate the JSON-RPC 2.0 envelope before interpreting it so a
            // malformed success or error response is never silently accepted
            // (invariant M5). Client.Call validates again for other transports.
            if (responseDict != null)
            {
                Client.ValidateEnvelope(responseDict, id, "HttpTransport");
            }

            if (responseDict != null && responseDict.TryGetValue("error", out var errorObj) && errorObj != null)
            {
                var code = -32603;
                var message = "Unknown error";
                object? data = null;
                if (errorObj is System.Text.Json.JsonElement errorElem)
                {
                    if (errorElem.TryGetProperty("code", out var codeProp)) code = codeProp.GetInt32();
                    if (errorElem.TryGetProperty("message", out var msgProp)) message = msgProp.GetString() ?? "Unknown error";
                    if (errorElem.TryGetProperty("data", out var dataProp)) data = dataProp;
                }
                else if (errorObj is Dictionary<string, object?> errorDict)
                {
                    if (errorDict.TryGetValue("code", out var codeObj)) code = Convert.ToInt32(codeObj);
                    if (errorDict.TryGetValue("message", out var msgObj)) message = msgObj?.ToString() ?? "Unknown error";
                    if (errorDict.TryGetValue("data", out var dataObj)) data = dataObj;
                }
                throw new RPCError(code, message, data);
            }

            return responseDict ?? new Dictionary<string, object?>();
        }
    }

    /// <summary>
    /// HTTP transport implementation using the legacy synchronous base class
    /// </summary>
    public class HttpTransportSync : Transport
    {
        private readonly HttpTransport _innerTransport;

        public HttpTransportSync(string baseUrl, Dictionary<string, string>? headers = null)
        {
            _innerTransport = new HttpTransport(baseUrl, headers);
        }

        public override Dictionary<string, object> Request(string method, object? @params)
        {
            var task = _innerTransport.CallAsync(method, @params as object[] ?? Array.Empty<object>());
            return task.GetAwaiter().GetResult()!;
        }

        public override Dictionary<string, object> Request(string method, object? @params, object id)
        {
            // Preserve the caller-supplied id so the runtime client can match it.
            var task = _innerTransport.CallAsync(method, @params, id);
            return task.GetAwaiter().GetResult()!;
        }
    }

    /// <summary>
    /// In-process transport for testing (directly calls Server)
    /// </summary>
    public class InProcTransport : ITransport
    {
        private readonly Server _server;

        public InProcTransport(Server server)
        {
            _server = server;
        }

        public Task<Dictionary<string, object?>> CallAsync(string method, object[] parameters)
            => CallAsync(method, parameters, Guid.NewGuid().ToString());

        public Task<Dictionary<string, object?>> CallAsync(string method, object? @params, object id)
        {
            var request = new Dictionary<string, object>
            {
                ["jsonrpc"] = "2.0",
                ["method"] = method,
                ["id"] = id
            };

            if (@params != null)
            {
                request["params"] = @params;
            }

            var response = _server.Call(request);
            if (response == null)
            {
                throw new Exception("Empty response (notification?)");
            }

            return Task.FromResult<Dictionary<string, object?>>(response!);
        }
    }
}

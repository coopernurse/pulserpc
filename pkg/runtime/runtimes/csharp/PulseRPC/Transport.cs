using System;
using System.Collections.Generic;
using System.Net.Http;
using System.Text;
using System.Threading.Tasks;

namespace PulseRPC
{
    /// <summary>
    /// Interface for RPC transports
    /// </summary>
    public interface ITransport
    {
        /// <summary>
        /// Sends a JSON-RPC request asynchronously and returns the response
        /// </summary>
        Task<Dictionary<string, object?>> CallAsync(string method, object[] parameters);
    }

    /// <summary>
    /// Abstract base class for RPC transports (legacy support)
    /// </summary>
    public abstract class Transport
    {
        /// <summary>
        /// Sends a JSON-RPC request and returns the response
        /// </summary>
        public abstract Dictionary<string, object> Request(string method, object? @params);
    }

    /// <summary>
    /// HTTP transport implementation using ITransport interface
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

        public async Task<Dictionary<string, object?>> CallAsync(string method, object[] parameters)
        {
            var requestId = Guid.NewGuid().ToString();
            var request = new Dictionary<string, object?>
            {
                { "jsonrpc", "2.0" },
                { "method", method },
                { "params", parameters },
                { "id", requestId }
            };

            var json = System.Text.Json.JsonSerializer.Serialize(request, _jsonOptions);
            var content = new StringContent(json, Encoding.UTF8, "application/json");

            var response = await _httpClient.PostAsync(_baseUrl, content);
            response.EnsureSuccessStatusCode();

            var responseJson = await response.Content.ReadAsStringAsync();
            var responseDict = System.Text.Json.JsonSerializer.Deserialize<Dictionary<string, object?>>(responseJson);

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
    /// HTTP transport implementation using abstract Transport base class (legacy sync support)
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
            return task.GetAwaiter().GetResult();
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
        {
            var request = new Dictionary<string, object>
            {
                ["jsonrpc"] = "2.0",
                ["method"] = method,
                ["id"] = 1
            };

            if (parameters != null && parameters.Length > 0)
            {
                request["params"] = parameters;
            }

            var response = _server.Call(request);
            if (response == null)
            {
                throw new Exception("Empty response (notification?)");
            }

            return Task.FromResult<Dictionary<string, object?>>(response);
        }
    }
}
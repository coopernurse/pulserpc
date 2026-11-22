using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text.Json;
using System.Threading.Tasks;
using Xunit;
using PulseRPC;

namespace PulseRPC.Tests
{
    public class ThrowingHandler
    {
        public string ping() => throw new InvalidOperationException("secret-detail-do-not-leak");
        public int add(int a, int b) => a + b;
        public int? nullReturn() => 0;
    }

    public class OptionalEchoHandler
    {
        public string? echo(string? s) => s;
    }

    public enum WireColor
    {
        red,
        green
    }

    public class WireColorDto
    {
        public WireColor Color { get; set; }
    }

    public class InvariantServerTests
    {
        private static Server CreateServer(bool validateResponses = true)
        {
            var server = new Server(new Contract(TestIDL.Build()), validateResponses: validateResponses);
            server.AddHandler("Calc", new CalcHandler());
            return server;
        }

        private static Dictionary<string, object> Request(
            string method, object? id = null, bool notification = false, object? @params = null,
            bool includeParams = false)
        {
            var request = new Dictionary<string, object>
            {
                ["jsonrpc"] = "2.0",
                ["method"] = method
            };
            if (!notification)
            {
                request["id"] = id ?? 1;
            }
            if (includeParams)
            {
                request["params"] = @params!;
            }
            return request;
        }

        // R15/D4: handler exceptions are logged server-side and the wire only
        // sees the fixed "Internal error" message with no data.
        [Fact]
        public void Call_HandlerThrows_DoesNotLeakExceptionOrStackTrace()
        {
            var server = new Server(new Contract(TestIDL.Build()));
            server.AddHandler("Calc", new ThrowingHandler());
            var logged = new List<(string Context, Exception Error)>();
            server.InternalErrorLogger = (context, error) => logged.Add((context, error));

            var response = server.Call(Request("Calc.ping"));

            Assert.NotNull(response);
            var err = Assert.IsType<Dictionary<string, object>>(response!["error"]);
            Assert.Equal(-32603, Convert.ToInt32(err["code"]));
            Assert.Equal("Internal error", err["message"]);
            Assert.False(err.ContainsKey("data"), "error.data must be omitted for internal errors");
            Assert.DoesNotContain("secret-detail-do-not-leak", JsonSerializer.Serialize(response));
            Assert.Single(logged);
            Assert.Contains("secret-detail-do-not-leak", logged[0].Error.Message);
        }

        [Fact]
        public void Call_ResponseValidationFailure_IsRedacted()
        {
            var server = new Server(new Contract(TestIDL.Build()));
            server.AddHandler("Calc", new CalcHandler());
            var logged = new List<Exception>();
            server.InternalErrorLogger = (_, error) => logged.Add(error);

            var response = server.Call(Request("Calc.nullReturn"));

            var err = Assert.IsType<Dictionary<string, object>>(response!["error"]);
            Assert.Equal(-32603, Convert.ToInt32(err["code"]));
            Assert.Equal("Internal error", err["message"]);
            Assert.False(err.ContainsKey("data"));
            Assert.Single(logged);
        }

        // M22: error.data is omitted when there is no data, not serialized as null.
        [Fact]
        public void Call_ErrorWithoutData_OmitsDataMember()
        {
            var response = CreateServer().Call(Request("Calc.nullReturn"));
            var err = Assert.IsType<Dictionary<string, object>>(response!["error"]);
            Assert.False(err.ContainsKey("data"));
            Assert.Equal(-32603, Convert.ToInt32(err["code"]));
            Assert.IsType<string>(err["message"]);
        }

        // M23: a null result is an explicit "result": null member.
        [Fact]
        public void Call_NullResult_SerializesExplicitNull()
        {
            var contract = new Contract(new Dictionary<string, object>
            {
                ["interfaces"] = new List<object>
                {
                    new Dictionary<string, object>
                    {
                        ["name"] = "E",
                        ["methods"] = new List<object>
                        {
                            new Dictionary<string, object>
                            {
                                ["name"] = "echo",
                                ["parameters"] = new List<object>
                                {
                                    new Dictionary<string, object>
                                    {
                                        ["name"] = "s",
                                        ["type"] = new Dictionary<string, object> { ["builtIn"] = "string" },
                                        ["optional"] = true
                                    }
                                },
                                ["returnType"] = new Dictionary<string, object> { ["builtIn"] = "string" },
                                ["returnOptional"] = true
                            }
                        }
                    }
                }
            });
            var eServer = new Server(contract);
            eServer.AddHandler("E", new OptionalEchoHandler());

            var response = eServer.Call(Request("E.echo", @params: new object?[] { null }, includeParams: true));

            Assert.NotNull(response);
            Assert.True(response!.ContainsKey("result"));
            Assert.Null(response["result"]);
        }

        // M21: id null is a notification and receives no response.
        [Fact]
        public void Call_NullId_IsNotification()
        {
            var response = CreateServer().Call(Request("Calc.ping", notification: true));
            Assert.Null(response);

            var explicitNull = new Dictionary<string, object>
            {
                ["jsonrpc"] = "2.0",
                ["method"] = "Calc.ping",
                ["id"] = null!
            };
            Assert.Null(CreateServer().Call(explicitNull));
        }

        // R24/D2: boolean, object and array ids are invalid requests with a
        // null response id.
        [Fact]
        public void Call_InvalidIdTypes_ReturnInvalidRequest()
        {
            var invalidIds = new object[]
            {
                true,
                new Dictionary<string, object> { ["x"] = 1 },
                new List<object> { 1, 2 }
            };

            foreach (var badId in invalidIds)
            {
                var request = new Dictionary<string, object>
                {
                    ["jsonrpc"] = "2.0",
                    ["method"] = "Calc.ping",
                    ["id"] = badId
                };
                var response = CreateServer().Call(request);
                Assert.NotNull(response);
                var err = Assert.IsType<Dictionary<string, object>>(response!["error"]);
                Assert.Equal(-32600, Convert.ToInt32(err["code"]));
                Assert.True(response.ContainsKey("id"));
                Assert.Null(response["id"]);
            }
        }

        // M24: numeric ids are echoed back with the same value.
        [Fact]
        public void Call_NumericId_IsEchoed()
        {
            var response = CreateServer().Call(Request("Calc.ping", id: 42L));
            Assert.NotNull(response);
            Assert.Equal(42L, response!["id"]);
        }

        // M20: object-form params are normalized (already covered more fully in
        // ServerTests) and reject unknown names explicitly.
        [Fact]
        public void Call_ObjectFormParams_AreNormalized()
        {
            var named = new Dictionary<string, object> { ["b"] = 4, ["a"] = 6 };
            var response = CreateServer().Call(Request("Calc.add", @params: named, includeParams: true));
            Assert.Equal(10, Convert.ToInt32(response!["result"]));
        }
    }

    public class InvariantClientTests
    {
        private sealed class ConcurrentTransport : ITransport
        {
            public ConcurrentDictionary<object, bool> Ids { get; } = new ConcurrentDictionary<object, bool>();

            public Task<Dictionary<string, object?>> CallAsync(string method, object[] parameters)
                => CallAsync(method, parameters, Guid.NewGuid().ToString());

            public Task<Dictionary<string, object?>> CallAsync(string method, object? @params, object id)
            {
                if (method == "pulserpc-idl")
                {
                    return Task.FromResult(FakeTransport.Result(id, TestIDL.Build()));
                }
                Ids.TryAdd(id, true);
                return Task.FromResult(FakeTransport.Result(id, 8));
            }
        }

        // R20: concurrent client calls use unique ids.
        [Fact]
        public void ConcurrentCalls_UseUniqueIds()
        {
            var transport = new ConcurrentTransport();
            var client = new Client(transport);

            Parallel.For(0, 200, _ => client.Call("Calc.add", new object[] { 1, 2 }));

            Assert.Equal(200, transport.Ids.Count);
            Assert.All(transport.Ids.Keys, id => Assert.IsType<string>(id));
        }

        // C11/R27: transport JSON uses declared enum names, not ordinals.
        [Fact]
        public void TransportJsonOptions_SerializeEnumsAsStrings()
        {
            var json = JsonSerializer.Serialize(new WireColorDto { Color = WireColor.green }, HttpTransport.JsonOptions);
            Assert.Contains("\"color\":\"green\"", json);
            Assert.DoesNotContain(":1", json);
        }
    }

    public class InvariantValidationTests
    {
        // R25/D1: IDL int is signed 32-bit.
        [Fact]
        public void ValidateInt_EnforcesSigned32BitRange()
        {
            Validation.ValidateInt(int.MaxValue);
            Validation.ValidateInt(int.MinValue);
            Validation.ValidateInt(2147483647L);
            Validation.ValidateInt(-2147483648L);
            Validation.ValidateInt(2147483647.0);
            Validation.ValidateInt(-2147483648.0);

            Assert.Throws<ArgumentException>(() => Validation.ValidateInt(2147483648L));
            Assert.Throws<ArgumentException>(() => Validation.ValidateInt(-2147483649L));
            Assert.Throws<ArgumentException>(() => Validation.ValidateInt(2147483648.0));
            Assert.Throws<ArgumentException>(() => Validation.ValidateInt(-2147483649.0));
            Assert.Throws<ArgumentException>(() => Validation.ValidateInt(true));
        }
    }

    public class InvariantTransportTests
    {
        // R19/D6: the HTTP client applies a 30 second timeout.
        [Fact]
        public void HttpTransport_UsesThirtySecondTimeout()
        {
            var transport = new HttpTransport("http://example.test");
            try
            {
                Assert.Equal(TimeSpan.FromSeconds(30), transport.HttpClient.Timeout);
            }
            finally
            {
                transport.HttpClient.Dispose();
            }
        }

        // R30: a trailing slash on the base URL is trimmed.
        [Fact]
        public void HttpTransport_TrimsTrailingSlash()
        {
            var transport = new HttpTransport("http://example.test/api/");
            try
            {
                Assert.Equal("http://example.test/api", transport.BaseUrl);
            }
            finally
            {
                transport.HttpClient.Dispose();
            }
        }

        // R29: custom headers are attached to every request.
        [Fact]
        public void HttpTransport_AppliesCustomHeaders()
        {
            var transport = new HttpTransport(
                "http://example.test",
                new Dictionary<string, string> { ["X-Api-Key"] = "secret", ["X-Tenant"] = "acme" });
            try
            {
                Assert.True(transport.HttpClient.DefaultRequestHeaders.Contains("X-Api-Key"));
                Assert.Equal("secret", transport.HttpClient.DefaultRequestHeaders.GetValues("X-Api-Key").Single());
                Assert.True(transport.HttpClient.DefaultRequestHeaders.Contains("X-Tenant"));
            }
            finally
            {
                transport.HttpClient.Dispose();
            }
        }

        // R17: response bodies are capped at 10 MiB.
        [Fact]
        public async Task HttpTransport_ResponseCap_RejectsOversizedBody()
        {
            Assert.Equal(10L * 1024 * 1024, HttpTransport.MaxResponseBytes);

            using var atLimit = new MemoryStream(new byte[1024]);
            var accepted = await HttpTransport.ReadLimitedAsync(atLimit, 1024);
            Assert.NotNull(accepted);
            Assert.Equal(1024, accepted!.Length);

            using var overLimit = new MemoryStream(new byte[1025]);
            Assert.Null(await HttpTransport.ReadLimitedAsync(overLimit, 1024));
        }
    }
}

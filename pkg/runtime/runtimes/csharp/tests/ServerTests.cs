using System;
using System.Collections.Generic;
using Xunit;
using PulseRPC;

namespace PulseRPC.Tests
{
    public class ServerTests
    {
        private static Server CreateServer()
        {
            var server = new Server(new Contract(TestIDL.Build()));
            server.AddHandler("Calc", new CalcHandler());
            return server;
        }

        private static Dictionary<string, object> Request(
            string method, object? id = null, bool notification = false, object? @params = null,
            bool includeParams = false, string jsonrpc = "2.0")
        {
            var request = new Dictionary<string, object>
            {
                ["jsonrpc"] = jsonrpc,
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

        private static int ErrorCode(Dictionary<string, object>? response)
        {
            Assert.NotNull(response);
            var err = Assert.IsType<Dictionary<string, object>>(response!["error"]);
            return Convert.ToInt32(err["code"]);
        }

        private static string ErrorData(Dictionary<string, object>? response)
        {
            var err = Assert.IsType<Dictionary<string, object>>(response!["error"]);
            return err.TryGetValue("data", out var data)
                ? data?.ToString() ?? string.Empty
                : err["message"].ToString() ?? string.Empty;
        }

        // csharp-02: exact parameter count is enforced even at zero params.
        [Fact]
        public void Call_NoParamsForTwoRequiredParams_ReturnsInvalidParams()
        {
            var response = CreateServer().Call(Request("Calc.add"));
            Assert.Equal(-32602, ErrorCode(response));
            Assert.Contains("expects 2 param(s)", ErrorData(response));
        }

        [Fact]
        public void Call_ExplicitNullParamsForTwoRequiredParams_ReturnsInvalidParams()
        {
            var response = CreateServer().Call(Request("Calc.add", @params: null, includeParams: true));
            Assert.Equal(-32602, ErrorCode(response));
        }

        [Fact]
        public void Call_NoParamsForZeroParamMethod_ReturnsNormalResult()
        {
            var response = CreateServer().Call(Request("Calc.ping"));
            Assert.NotNull(response);
            Assert.Equal("pong", response!["result"]);
        }

        // csharp-03: notifications never receive a response, not even errors.
        [Fact]
        public void Call_UnknownMethodNotification_ReturnsNull()
        {
            var response = CreateServer().Call(Request("Calc.nope", notification: true));
            Assert.Null(response);
        }

        [Fact]
        public void Call_JsonRpcIdlNotification_ReturnsNull()
        {
            var response = CreateServer().Call(Request("pulserpc-idl", notification: true));
            Assert.Null(response);
        }

        [Fact]
        public void Call_SuccessfulNotification_ReturnsNull()
        {
            var response = CreateServer().Call(Request("Calc.ping", notification: true));
            Assert.Null(response);
        }

        [Fact]
        public void Call_InvalidJsonRpcNotification_ReturnsNull()
        {
            var response = CreateServer().Call(Request("Calc.ping", notification: true, jsonrpc: "1.0"));
            Assert.Null(response);
        }

        [Fact]
        public void Call_ErrorForRequestWithId_IncludesId()
        {
            var response = CreateServer().Call(Request("Calc.nope", id: 7));
            Assert.NotNull(response);
            Assert.Equal(7, response!["id"]);
        }

        // csharp-04: a null result for a required return type is an error. The
        // wire message is generic; validation details stay server-side (R15).
        [Fact]
        public void Call_NullResultForRequiredReturn_ReturnsError()
        {
            var response = CreateServer().Call(Request("Calc.nullReturn"));
            Assert.Equal(-32603, ErrorCode(response));
            var err = Assert.IsType<Dictionary<string, object>>(response!["error"]);
            Assert.Equal("Internal error", err["message"]);
            Assert.False(err.ContainsKey("data"));
        }

        // csharp-05: by-name params are normalized using the IDL parameter names.
        [Fact]
        public void Call_NamedParams_MatchPositionalResult()
        {
            var named = new Dictionary<string, object> { ["a"] = 5, ["b"] = 3 };
            var namedResponse = CreateServer().Call(Request("Calc.add", @params: named, includeParams: true));
            var positionalResponse = CreateServer().Call(Request("Calc.add", @params: new object[] { 5, 3 }, includeParams: true));

            Assert.NotNull(namedResponse);
            Assert.Equal(positionalResponse!["result"], namedResponse!["result"]);
            Assert.Equal(8, Convert.ToInt32(namedResponse["result"]));
        }

        [Fact]
        public void Call_NamedParamsOutOfOrder_AreMappedByName()
        {
            var named = new Dictionary<string, object> { ["b"] = 3, ["a"] = 5 };
            var response = CreateServer().Call(Request("Calc.add", @params: named, includeParams: true));
            Assert.Equal(8, Convert.ToInt32(response!["result"]));
        }

        [Fact]
        public void Call_UnknownParamName_ReturnsInvalidParams()
        {
            var named = new Dictionary<string, object> { ["a"] = 5, ["c"] = 3 };
            var response = CreateServer().Call(Request("Calc.add", @params: named, includeParams: true));
            Assert.Equal(-32602, ErrorCode(response));
            Assert.Contains("Unknown parameter 'c'", ErrorData(response));
        }

        // csharp-06: an empty method is an invalid request in the runtime server.
        [Fact]
        public void Call_EmptyMethod_ReturnsInvalidRequest()
        {
            var response = CreateServer().Call(Request(""));
            Assert.Equal(-32600, ErrorCode(response));
        }
    }
}

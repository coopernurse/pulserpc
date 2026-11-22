using System;
using System.Collections.Generic;
using System.Linq;
using Xunit;
using PulseRPC;

namespace PulseRPC.Tests
{
    public class ClientTests
    {
        private static FakeTransport DefaultTransport(
            object idl, Func<string, object?, object, Dictionary<string, object?>>? methodHandler = null)
            => new FakeTransport((method, @params, id) =>
            {
                if (method == "pulserpc-idl")
                {
                    return FakeTransport.Result(id, idl);
                }
                return methodHandler != null
                    ? methodHandler(method, @params, id)
                    : FakeTransport.Result(id, null);
            });

        // csharp-01: verification compares the local IDL against the running server.
        [Fact]
        public async Task VerifyCompatibility_DivergentLocalIDL_ReturnsIncompatibleWithErrorDelta()
        {
            var serverIdl = TestIDL.Build(addressHasExtraRequiredField: true, checksum: "server-sum");
            var transport = DefaultTransport(serverIdl);
            var client = new Client(transport);

            client.SetLocalIDL(TestIDL.BuildJson(addressHasExtraRequiredField: false, checksum: "client-sum"));

            var result = await client.VerifyCompatibility();

            Assert.False(result.Compatible);
            Assert.Contains(result.Deltas, d => d.Severity == Severity.Error);
            Assert.Equal("server-sum", result.ServerChecksum);
            Assert.Equal("client-sum", result.ClientChecksum);
            Assert.True(transport.Calls.Count(c => c.Method == "pulserpc-idl") >= 2,
                "VerifyCompatibility must fetch the server IDL over the transport");
        }

        [Fact]
        public async Task VerifyCompatibility_IdenticalIDLs_ReturnsCompatible()
        {
            var idl = TestIDL.Build(checksum: "same-sum");
            var transport = DefaultTransport(idl);
            var client = new Client(transport);

            var result = await client.VerifyCompatibility();

            Assert.True(result.Compatible);
            Assert.Empty(result.Deltas);
            Assert.Equal("same-sum", result.ServerChecksum);
        }

        [Fact]
        public async Task VerifyCompatibility_ServerChangedAfterBootstrap_IsDetectedOnSecondCall()
        {
            object currentIdl = TestIDL.Build(checksum: "v1");
            // The pulserpc-idl response is sourced from a mutable document.
            var dynamicTransport = new FakeTransport((method, @params, id) =>
                method == "pulserpc-idl"
                    ? FakeTransport.Result(id, currentIdl)
                    : FakeTransport.Result(id, null));

            var client = new Client(dynamicTransport);
            var first = await client.VerifyCompatibility();
            Assert.True(first.Compatible);
            Assert.Equal("v1", first.ServerChecksum);

            currentIdl = TestIDL.Build(addressHasExtraRequiredField: true, checksum: "v2");
            var second = await client.VerifyCompatibility();

            Assert.False(second.Compatible);
            Assert.Equal("v2", second.ServerChecksum);
            Assert.Contains(second.Deltas, d => d.Severity == Severity.Error);
        }

        [Fact]
        public async Task VerifyCompatibility_FailedFetch_ReturnsIncompatibleWithError()
        {
            var calls = 0;
            var transport = new FakeTransport((method, @params, id) =>
            {
                if (method != "pulserpc-idl") return FakeTransport.Result(id, null);
                calls++;
                if (calls == 1) return FakeTransport.Result(id, TestIDL.Build());
                throw new Exception("connection lost");
            });

            var client = new Client(transport);
            var result = await client.VerifyCompatibility();

            Assert.False(result.Compatible);
            Assert.Contains("failed to fetch server IDL", result.Error);
        }

        // csharp-08: VerifyOnBootstrap is honored by the runtime client.
        [Fact]
        public void VerifyOnBootstrap_WithFailFastAuditor_ThrowsOnDivergentServer()
        {
            var calls = 0;
            var transport = new FakeTransport((method, @params, id) =>
            {
                calls++;
                var idl = calls == 1
                    ? TestIDL.Build(checksum: "v1")
                    : TestIDL.Build(addressHasExtraRequiredField: true, checksum: "v2");
                return FakeTransport.Result(id, idl);
            });

            var options = Client.Options
                .WithAuditor(new FailFastAuditor())
                .VerifyOnBootstrap();

            Assert.Throws<InvalidOperationException>(() =>
                Client.Create(transport, options: options));
        }

        // csharp-09: the client uses ITransport and matches the response id.
        [Fact]
        public void Call_UsesITransportWithCallerOwnedId()
        {
            object? sentId = null;
            var transport = new FakeTransport((method, @params, id) =>
                method == "pulserpc-idl"
                    ? FakeTransport.Result(id, TestIDL.Build())
                    : FakeTransport.Result(id, 8));

            var client = new Client(transport);
            var result = client.Call("Calc.add", new object[] { 5, 3 });

            Assert.Equal(8, Convert.ToInt32(result));
            var call = transport.Calls.Last(c => c.Method == "Calc.add");
            sentId = call.Id;
            Assert.NotNull(sentId);
            Assert.IsType<int>(sentId);
        }

        [Fact]
        public void Call_MismatchedResponseId_Throws()
        {
            var transport = new FakeTransport((method, @params, id) =>
                method == "pulserpc-idl"
                    ? FakeTransport.Result(id, TestIDL.Build())
                    : FakeTransport.Result(99999, 8));

            var client = new Client(transport);

            var ex = Assert.Throws<InvalidOperationException>(() => client.Call("Calc.add", new object[] { 5, 3 }));
            Assert.Contains("does not match request id", ex.Message);
        }

        // csharp-12: the response envelope must be well-formed.
        [Fact]
        public void Call_ResponseWithBothErrorAndResult_Throws()
        {
            var transport = new FakeTransport((method, @params, id) =>
            {
                if (method == "pulserpc-idl") return FakeTransport.Result(id, TestIDL.Build());
                var response = FakeTransport.Result(id, 8);
                response["error"] = new Dictionary<string, object?>
                {
                    ["code"] = -32603,
                    ["message"] = "boom"
                };
                return response;
            });

            var client = new Client(transport);
            var ex = Assert.Throws<InvalidOperationException>(() => client.Call("Calc.add", new object[] { 5, 3 }));
            Assert.Contains("exactly one", ex.Message);
        }

        [Fact]
        public void Call_MissingJsonRpcInResponse_Throws()
        {
            var transport = new FakeTransport((method, @params, id) =>
            {
                if (method == "pulserpc-idl") return FakeTransport.Result(id, TestIDL.Build());
                return new Dictionary<string, object?> { ["id"] = id, ["result"] = 8 };
            });

            var client = new Client(transport);
            Assert.Throws<InvalidOperationException>(() => client.Call("Calc.add", new object[] { 5, 3 }));
        }

        [Fact]
        public void Call_ErrorResponse_ThrowsRPCError()
        {
            var transport = new FakeTransport((method, @params, id) =>
                method == "pulserpc-idl"
                    ? FakeTransport.Result(id, TestIDL.Build())
                    : FakeTransport.Error(id, -32602, "bad params"));

            var client = new Client(transport);
            var ex = Assert.Throws<RPCError>(() => client.Call("Calc.add", new object[] { 5, 3 }));
            Assert.Equal(-32602, ex.Code);
        }

        // csharp-04: client response validation must reject null required results.
        [Fact]
        public void Call_ValidateResponse_NullForRequiredReturn_Throws()
        {
            var transport = new FakeTransport((method, @params, id) =>
                method == "pulserpc-idl"
                    ? FakeTransport.Result(id, TestIDL.Build())
                    : FakeTransport.Result(id, null));

            var client = new Client(transport, validateResponse: true);

            var ex = Assert.Throws<Exception>(() => client.Call("Calc.nullReturn"));
            Assert.Contains("Response validation failed", ex.Message);
        }
    }
}

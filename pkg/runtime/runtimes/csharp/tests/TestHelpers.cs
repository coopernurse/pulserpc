using System;
using System.Collections.Generic;
using System.Text.Json;
using System.Threading.Tasks;
using PulseRPC;

namespace PulseRPC.Tests
{
    /// <summary>
    /// Builds the in-memory IDL shape understood by Contract and DiffEngine.
    /// </summary>
    public static class TestIDL
    {
        public static Dictionary<string, object> Build(bool addressHasExtraRequiredField = false, string checksum = "base-checksum")
        {
            var addressFields = new List<object>
            {
                new Dictionary<string, object>
                {
                    ["name"] = "city",
                    ["type"] = BuiltIn("string"),
                    ["optional"] = false
                }
            };
            if (addressHasExtraRequiredField)
            {
                addressFields.Add(new Dictionary<string, object>
                {
                    ["name"] = "zip",
                    ["type"] = BuiltIn("int"),
                    ["optional"] = false
                });
            }

            return new Dictionary<string, object>
            {
                ["checksum"] = checksum,
                ["interfaces"] = new List<object>
                {
                    new Dictionary<string, object>
                    {
                        ["name"] = "Calc",
                        ["methods"] = new List<object>
                        {
                            Method("add", new List<object> { Param("a", "int"), Param("b", "int") }, BuiltIn("int")),
                            Method("ping", new List<object>(), BuiltIn("string")),
                            Method("nullReturn", new List<object>(), BuiltIn("int"))
                        }
                    }
                },
                ["structs"] = new List<object>
                {
                    new Dictionary<string, object> { ["name"] = "Address", ["fields"] = addressFields }
                },
                ["enums"] = new List<object>
                {
                    new Dictionary<string, object>
                    {
                        ["name"] = "Color",
                        ["values"] = new List<object>
                        {
                            new Dictionary<string, object> { ["name"] = "red" },
                            new Dictionary<string, object> { ["name"] = "green" }
                        }
                    }
                }
            };
        }

        public static string BuildJson(bool addressHasExtraRequiredField = false, string checksum = "base-checksum")
            => JsonSerializer.Serialize(Build(addressHasExtraRequiredField, checksum));

        public static Dictionary<string, object> BuiltIn(string name)
            => new Dictionary<string, object> { ["builtIn"] = name };

        private static Dictionary<string, object> Param(string name, string type)
            => new Dictionary<string, object> { ["name"] = name, ["type"] = BuiltIn(type), ["optional"] = false };

        private static Dictionary<string, object> Method(string name, List<object> parameters, Dictionary<string, object> returnType)
            => new Dictionary<string, object>
            {
                ["name"] = name,
                ["parameters"] = parameters,
                ["returnType"] = returnType
            };
    }

    /// <summary>
    /// Handler used by Server unit tests.
    /// </summary>
    public class CalcHandler
    {
        public int add(int a, int b) => a + b;
        public string ping() => "pong";
        public int? nullReturn() => null;
    }

    /// <summary>
    /// Records calls and returns scripted responses, so Client id/envelope
    /// behavior can be asserted without a network.
    /// </summary>
    public class FakeTransport : ITransport
    {
        private readonly Func<string, object?, object, Dictionary<string, object?>> _handler;
        public List<(string Method, object? Params, object Id)> Calls { get; } = new List<(string, object?, object)>();

        public FakeTransport(Func<string, object?, object, Dictionary<string, object?>> handler)
        {
            _handler = handler;
        }

        public Task<Dictionary<string, object?>> CallAsync(string method, object[] parameters)
            => CallAsync(method, parameters, Guid.NewGuid().ToString());

        public Task<Dictionary<string, object?>> CallAsync(string method, object? @params, object id)
        {
            Calls.Add((method, @params, id));
            return Task.FromResult(_handler(method, @params, id));
        }

        public static Dictionary<string, object?> Result(object id, object? result)
            => new Dictionary<string, object?>
            {
                ["jsonrpc"] = "2.0",
                ["id"] = id,
                ["result"] = result
            };

        public static Dictionary<string, object?> Error(object id, int code, string message)
            => new Dictionary<string, object?>
            {
                ["jsonrpc"] = "2.0",
                ["id"] = id,
                ["error"] = new Dictionary<string, object?>
                {
                    ["code"] = code,
                    ["message"] = message
                }
            };
    }
}

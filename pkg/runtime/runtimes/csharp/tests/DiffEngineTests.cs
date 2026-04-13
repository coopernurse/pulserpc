using System;
using System.Collections.Generic;
using System.Text.Json;
using Xunit;
using PulseRPC;

namespace PulseRPC.Tests
{
    public class DiffEngineTests
    {
        [Fact]
        public void DiffIDL_IdenticalIDLs_ReturnsNoDeltas()
        {
            var idl = @"{
                ""interfaces"": [{
                    ""name"": ""TestService"",
                    ""methods"": [{
                        ""name"": ""testMethod"",
                        ""parameters"": [{""name"": ""arg1"", ""type"": ""string""}],
                        ""returnType"": {""type"": ""string""}
                    }]
                }],
                ""structs"": [{
                    ""name"": ""TestStruct"",
                    ""fields"": [{""name"": ""field1"", ""type"": ""string""}]
                }]
            }";

            var clientIDL = JsonSerializer.Deserialize<Dictionary<string, object>>(idl);
            var serverIDL = JsonSerializer.Deserialize<Dictionary<string, object>>(idl);

            var deltas = DiffEngine.DiffIDL(clientIDL, serverIDL);

            Assert.Empty(deltas);
        }

        [Fact]
        public void DiffIDL_AddedOptionalField_ReturnsInfoSeverity()
        {
            var clientIDL = JsonSerializer.Deserialize<Dictionary<string, object>>(@"{
                ""interfaces"": [],
                ""structs"": [{
                    ""name"": ""TestStruct"",
                    ""fields"": [
                        {""name"": ""existingField"", ""type"": ""string"", ""optional"": false}
                    ]
                }]
            }");

            var serverIDL = JsonSerializer.Deserialize<Dictionary<string, object>>(@"{
                ""interfaces"": [],
                ""structs"": [{
                    ""name"": ""TestStruct"",
                    ""fields"": [
                        {""name"": ""existingField"", ""type"": ""string"", ""optional"": false},
                        {""name"": ""newField"", ""type"": ""int"", ""optional"": true}
                    ]
                }]
            }");

            var deltas = DiffEngine.DiffIDL(clientIDL, serverIDL);

            Assert.Single(deltas);
            Assert.Equal(Severity.Info, deltas[0].Severity);
            Assert.Equal(ChangeType.Added, deltas[0].ChangeType);
            Assert.Equal(Direction.ClientHasLess, deltas[0].Direction);
        }

        [Fact]
        public void DiffIDL_AddedRequiredField_ReturnsErrorSeverity()
        {
            var clientIDL = JsonSerializer.Deserialize<Dictionary<string, object>>(@"{
                ""interfaces"": [],
                ""structs"": [{
                    ""name"": ""TestStruct"",
                    ""fields"": [
                        {""name"": ""existingField"", ""type"": ""string"", ""optional"": false}
                    ]
                }]
            }");

            var serverIDL = JsonSerializer.Deserialize<Dictionary<string, object>>(@"{
                ""interfaces"": [],
                ""structs"": [{
                    ""name"": ""TestStruct"",
                    ""fields"": [
                        {""name"": ""existingField"", ""type"": ""string"", ""optional"": false},
                        {""name"": ""newField"", ""type"": ""int"", ""optional"": false}
                    ]
                }]
            }");

            var deltas = DiffEngine.DiffIDL(clientIDL, serverIDL);

            Assert.Single(deltas);
            Assert.Equal(Severity.Error, deltas[0].Severity);
        }

        [Fact]
        public void DiffIDL_RemovedField_ReturnsInfoSeverity()
        {
            var clientIDL = JsonSerializer.Deserialize<Dictionary<string, object>>(@"{
                ""interfaces"": [],
                ""structs"": [{
                    ""name"": ""TestStruct"",
                    ""fields"": [
                        {""name"": ""existingField"", ""type"": ""string"", ""optional"": false},
                        {""name"": ""oldField"", ""type"": ""int"", ""optional"": false}
                    ]
                }]
            }");

            var serverIDL = JsonSerializer.Deserialize<Dictionary<string, object>>(@"{
                ""interfaces"": [],
                ""structs"": [{
                    ""name"": ""TestStruct"",
                    ""fields"": [
                        {""name"": ""existingField"", ""type"": ""string"", ""optional"": false}
                    ]
                }]
            }");

            var deltas = DiffEngine.DiffIDL(clientIDL, serverIDL);

            Assert.Single(deltas);
            Assert.Equal(Severity.Info, deltas[0].Severity);
            Assert.Equal(ChangeType.Removed, deltas[0].ChangeType);
            Assert.Equal(Direction.ClientHasMore, deltas[0].Direction);
        }

        [Fact]
        public void DiffIDL_FieldMadeOptional_ReturnsInfoSeverity()
        {
            var clientIDL = JsonSerializer.Deserialize<Dictionary<string, object>>(@"{
                ""interfaces"": [],
                ""structs"": [{
                    ""name"": ""TestStruct"",
                    ""fields"": [
                        {""name"": ""status"", ""type"": ""string"", ""optional"": false}
                    ]
                }]
            }");

            var serverIDL = JsonSerializer.Deserialize<Dictionary<string, object>>(@"{
                ""interfaces"": [],
                ""structs"": [{
                    ""name"": ""TestStruct"",
                    ""fields"": [
                        {""name"": ""status"", ""type"": ""string"", ""optional"": true}
                    ]
                }]
            }");

            var deltas = DiffEngine.DiffIDL(clientIDL, serverIDL);

            Assert.Single(deltas);
            Assert.Equal(Severity.Info, deltas[0].Severity);
            Assert.Equal(ChangeType.Modified, deltas[0].ChangeType);
        }

        [Fact]
        public void DiffIDL_FieldMadeRequired_ReturnsWarningSeverity()
        {
            var clientIDL = JsonSerializer.Deserialize<Dictionary<string, object>>(@"{
                ""interfaces"": [],
                ""structs"": [{
                    ""name"": ""TestStruct"",
                    ""fields"": [
                        {""name"": ""status"", ""type"": ""string"", ""optional"": true}
                    ]
                }]
            }");

            var serverIDL = JsonSerializer.Deserialize<Dictionary<string, object>>(@"{
                ""interfaces"": [],
                ""structs"": [{
                    ""name"": ""TestStruct"",
                    ""fields"": [
                        {""name"": ""status"", ""type"": ""string"", ""optional"": false}
                    ]
                }]
            }");

            var deltas = DiffEngine.DiffIDL(clientIDL, serverIDL);

            Assert.Single(deltas);
            Assert.Equal(Severity.Warning, deltas[0].Severity);
            Assert.Equal(ChangeType.Modified, deltas[0].ChangeType);
        }

        [Fact]
        public void ComputeChecksum_SameIDL_ReturnsSameChecksum()
        {
            var idl = JsonSerializer.Deserialize<Dictionary<string, object>>(@"{""test"": ""data""}");

            var checksum1 = DiffEngine.ComputeChecksum(idl);
            var checksum2 = DiffEngine.ComputeChecksum(idl);

            Assert.Equal(checksum1, checksum2);
        }

        [Fact]
        public void ComputeChecksum_DifferentIDL_ReturnsDifferentChecksum()
        {
            var idl1 = JsonSerializer.Deserialize<Dictionary<string, object>>(@"{""test"": ""data1""}");
            var idl2 = JsonSerializer.Deserialize<Dictionary<string, object>>(@"{""test"": ""data2""}");

            var checksum1 = DiffEngine.ComputeChecksum(idl1);
            var checksum2 = DiffEngine.ComputeChecksum(idl2);

            Assert.NotEqual(checksum1, checksum2);
        }

        [Fact]
        public void DiffIDL_StructRemovedFromServer_ReturnsErrorSeverity()
        {
            var clientIDL = JsonSerializer.Deserialize<Dictionary<string, object>>(@"{
                ""structs"": [{
                    ""name"": ""TestStruct"",
                    ""fields"": [{""name"": ""field1"", ""type"": ""string""}]
                }]
            }");

            var serverIDL = JsonSerializer.Deserialize<Dictionary<string, object>>(@"{
                ""structs"": []
            }");

            var deltas = DiffEngine.DiffIDL(clientIDL, serverIDL);

            Assert.Single(deltas);
            Assert.Equal(Severity.Error, deltas[0].Severity);
            Assert.Equal(EntityType.Struct, deltas[0].EntityType);
        }

        [Fact]
        public void DiffIDL_InterfaceAddedToServer_ReturnsInfoSeverity()
        {
            var clientIDL = JsonSerializer.Deserialize<Dictionary<string, object>>(@"{
                ""interfaces"": []
            }");

            var serverIDL = JsonSerializer.Deserialize<Dictionary<string, object>>(@"{
                ""interfaces"": [{
                    ""name"": ""NewService"",
                    ""methods"": []
                }]
            }");

            var deltas = DiffEngine.DiffIDL(clientIDL, serverIDL);

            Assert.Single(deltas);
            Assert.Equal(Severity.Info, deltas[0].Severity);
            Assert.Equal(EntityType.Interface, deltas[0].EntityType);
        }
    }
}
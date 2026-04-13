using System;
using System.Collections.Generic;
using Xunit;
using PulseRPC;

namespace PulseRPC.Tests
{
    public class AuditorTests
    {
        [Fact]
        public void NoOpAuditor_Name_ReturnsNoOp()
        {
            var auditor = new NoOpAuditor();
            Assert.Equal("NoOp", auditor.Name);
        }

        [Fact]
        public void NoOpAuditor_AuditCompatible_DoesNotThrow()
        {
            var auditor = new NoOpAuditor();
            var result = new VerificationResult
            {
                Compatible = true,
                ServerChecksum = "abc123",
                ClientChecksum = "def456",
                Deltas = new List<ContractDelta>(),
                Timestamp = DateTime.UtcNow
            };

            var exception = Record.Exception(() => auditor.Audit(result));
            Assert.Null(exception);
        }

        [Fact]
        public void NoOpAuditor_AuditIncompatible_DoesNotThrow()
        {
            var auditor = new NoOpAuditor();
            var result = new VerificationResult
            {
                Compatible = false,
                ServerChecksum = "abc123",
                ClientChecksum = "def456",
                Deltas = new List<ContractDelta>
                {
                    new ContractDelta(
                        EntityType.Struct,
                        "TestStruct",
                        "",
                        ChangeType.Removed,
                        Direction.ClientHasMore,
                        Severity.Error,
                        "Struct 'TestStruct' removed")
                },
                Timestamp = DateTime.UtcNow
            };

            var exception = Record.Exception(() => auditor.Audit(result));
            Assert.Null(exception);
        }

        [Fact]
        public void LoggingAuditor_Name_ReturnsLogging()
        {
            var auditor = new LoggingAuditor();
            Assert.Equal("Logging", auditor.Name);
        }

        [Fact]
        public void LoggingAuditor_AuditCompatible_DoesNotThrow()
        {
            var auditor = new LoggingAuditor();
            var result = new VerificationResult
            {
                Compatible = true,
                ServerChecksum = "abc123",
                ClientChecksum = "def456",
                Deltas = new List<ContractDelta>(),
                Timestamp = DateTime.UtcNow
            };

            var exception = Record.Exception(() => auditor.Audit(result));
            Assert.Null(exception);
        }

        [Fact]
        public void FailFastAuditor_Name_ReturnsFailFast()
        {
            var auditor = new FailFastAuditor();
            Assert.Equal("FailFast", auditor.Name);
        }

        [Fact]
        public void FailFastAuditor_AuditCompatible_DoesNotThrow()
        {
            var auditor = new FailFastAuditor();
            var result = new VerificationResult
            {
                Compatible = true,
                ServerChecksum = "abc123",
                ClientChecksum = "def456",
                Deltas = new List<ContractDelta>(),
                Timestamp = DateTime.UtcNow
            };

            var exception = Record.Exception(() => auditor.Audit(result));
            Assert.Null(exception);
        }

        [Fact]
        public void FailFastAuditor_AuditIncompatibleWithErrorDelta_ThrowsException()
        {
            var auditor = new FailFastAuditor();
            var result = new VerificationResult
            {
                Compatible = false,
                ServerChecksum = "abc123",
                ClientChecksum = "def456",
                Deltas = new List<ContractDelta>
                {
                    new ContractDelta(
                        EntityType.Struct,
                        "TestStruct",
                        "",
                        ChangeType.Removed,
                        Direction.ClientHasMore,
                        Severity.Error,
                        "Struct 'TestStruct' removed")
                },
                Timestamp = DateTime.UtcNow
            };

            Assert.Throws<InvalidOperationException>(() => auditor.Audit(result));
        }

        [Fact]
        public void FailFastAuditor_AuditWithWarningOnly_DoesNotThrow()
        {
            var auditor = new FailFastAuditor();
            var result = new VerificationResult
            {
                Compatible = true,
                ServerChecksum = "abc123",
                ClientChecksum = "def456",
                Deltas = new List<ContractDelta>
                {
                    new ContractDelta(
                        EntityType.Enum,
                        "TestEnum",
                        "",
                        ChangeType.Added,
                        Direction.ClientHasLess,
                        Severity.Warning,
                        "Enum value added")
                },
                Timestamp = DateTime.UtcNow
            };

            var exception = Record.Exception(() => auditor.Audit(result));
            Assert.Null(exception);
        }

        [Fact]
        public void FailFastAuditor_AuditWithInfoOnly_DoesNotThrow()
        {
            var auditor = new FailFastAuditor();
            var result = new VerificationResult
            {
                Compatible = true,
                ServerChecksum = "abc123",
                ClientChecksum = "def456",
                Deltas = new List<ContractDelta>
                {
                    new ContractDelta(
                        EntityType.Field,
                        "TestStruct",
                        "newField",
                        ChangeType.Added,
                        Direction.ClientHasLess,
                        Severity.Info,
                        "Field 'newField' added")
                },
                Timestamp = DateTime.UtcNow
            };

            var exception = Record.Exception(() => auditor.Audit(result));
            Assert.Null(exception);
        }
    }
}
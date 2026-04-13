using System;
using Xunit;
using PulseRPC;

namespace PulseRPC.Tests
{
    public class SeverityTests
    {
        [Theory]
        [InlineData(EntityType.Struct, ChangeType.Removed, Direction.ClientHasMore, Severity.Error)]
        [InlineData(EntityType.Struct, ChangeType.Added, Direction.ClientHasLess, Severity.Info)]
        [InlineData(EntityType.Struct, ChangeType.Modified, Direction.Mismatch, Severity.Info)]
        public void ClassifySeverity_StructChanges_ReturnsExpectedSeverity(
            EntityType entityType, ChangeType changeType, Direction direction, Severity expected)
        {
            var result = DiffEngine.ClassifySeverity(entityType, changeType, direction);
            Assert.Equal(expected, result);
        }

        [Fact]
        public void ClassifySeverity_FieldModified_ReturnsError()
        {
            var result = DiffEngine.ClassifySeverity(EntityType.Field, ChangeType.Modified, Direction.Mismatch);
            Assert.Equal(Severity.Error, result);
        }

        [Fact]
        public void ClassifySeverity_FieldRemoved_ReturnsInfo()
        {
            var result = DiffEngine.ClassifySeverity(EntityType.Field, ChangeType.Removed, Direction.ClientHasMore);
            Assert.Equal(Severity.Info, result);
        }

        [Fact]
        public void ClassifySeverity_FieldAddedRequired_ReturnsError()
        {
            var result = DiffEngine.ClassifySeverity(EntityType.Field, ChangeType.Added, Direction.ClientHasLess, "required");
            Assert.Equal(Severity.Error, result);
        }

        [Fact]
        public void ClassifySeverity_FieldAddedOptional_ReturnsInfo()
        {
            var result = DiffEngine.ClassifySeverity(EntityType.Field, ChangeType.Added, Direction.ClientHasLess, "optional");
            Assert.Equal(Severity.Info, result);
        }

        [Theory]
        [InlineData(ChangeType.Modified, Direction.Mismatch, Severity.Error)]
        [InlineData(ChangeType.Removed, Direction.ClientHasMore, Severity.Error)]
        [InlineData(ChangeType.Added, Direction.ClientHasLess, Severity.Warning)]
        public void ClassifySeverity_MethodChanges_ReturnsExpectedSeverity(
            ChangeType changeType, Direction direction, Severity expected)
        {
            var result = DiffEngine.ClassifySeverity(EntityType.Method, changeType, direction);
            Assert.Equal(expected, result);
        }

        [Theory]
        [InlineData(ChangeType.Removed, Direction.ClientHasMore, Severity.Warning)]
        [InlineData(ChangeType.Added, Direction.ClientHasLess, Severity.Warning)]
        public void ClassifySeverity_EnumChanges_ReturnsExpectedSeverity(
            ChangeType changeType, Direction direction, Severity expected)
        {
            var result = DiffEngine.ClassifySeverity(EntityType.Enum, changeType, direction);
            Assert.Equal(expected, result);
        }

        [Theory]
        [InlineData(ChangeType.Removed, Direction.ClientHasMore, Severity.Info)]
        [InlineData(ChangeType.Added, Direction.ClientHasLess, Severity.Info)]
        public void ClassifySeverity_ErrorChanges_ReturnsExpectedSeverity(
            ChangeType changeType, Direction direction, Severity expected)
        {
            var result = DiffEngine.ClassifySeverity(EntityType.Error, changeType, direction);
            Assert.Equal(expected, result);
        }

        [Theory]
        [InlineData(ChangeType.Removed, Direction.ClientHasMore, Severity.Error)]
        [InlineData(ChangeType.Added, Direction.ClientHasLess, Severity.Info)]
        public void ClassifySeverity_InterfaceChanges_ReturnsExpectedSeverity(
            ChangeType changeType, Direction direction, Severity expected)
        {
            var result = DiffEngine.ClassifySeverity(EntityType.Interface, changeType, direction);
            Assert.Equal(expected, result);
        }

        [Fact]
        public void ClassifySeverity_FieldMadeOptional_ReturnsInfoSeverity()
        {
            var result = DiffEngine.ClassifySeverity(
                EntityType.Field, ChangeType.Modified, Direction.ClientHasLess, "made_optional");
            Assert.Equal(Severity.Info, result);
        }

        [Fact]
        public void ClassifySeverity_FieldMadeRequired_ReturnsWarningSeverity()
        {
            var result = DiffEngine.ClassifySeverity(
                EntityType.Field, ChangeType.Modified, Direction.ClientHasLess, "made_required");
            Assert.Equal(Severity.Warning, result);
        }

        [Fact]
        public void ClassifySeverity_UnknownCombination_ReturnsInfoSeverity()
        {
            var result = DiffEngine.ClassifySeverity(EntityType.Interface, ChangeType.Modified, Direction.Mismatch);
            Assert.Equal(Severity.Info, result);
        }
    }
}
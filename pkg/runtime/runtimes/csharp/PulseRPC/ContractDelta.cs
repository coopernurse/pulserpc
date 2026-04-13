using System;
using System.Collections.Generic;

namespace PulseRPC
{
    public enum EntityType
    {
        Interface,
        Method,
        Struct,
        Field,
        Enum,
        Error
    }

    public enum ChangeType
    {
        Added,
        Removed,
        Modified
    }

    public enum Direction
    {
        ClientHasMore,
        ClientHasLess,
        Mismatch
    }

    public enum Severity
    {
        Error,
        Warning,
        Info
    }

    public readonly record struct ContractDelta(
        EntityType EntityType,
        string EntityName,
        string MemberName,
        ChangeType ChangeType,
        Direction Direction,
        Severity Severity,
        string Description
    );
}
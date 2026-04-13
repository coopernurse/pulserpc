package com.bitmechanic.pulserpc;

public class ContractDelta {
    public enum EntityType {
        Interface,
        Method,
        Struct,
        Field,
        Enum,
        Error
    }

    public enum ChangeType {
        Added,
        Removed,
        Modified
    }

    public enum Direction {
        ClientHasMore,
        ClientHasLess,
        Mismatch
    }

    public enum Severity {
        Error,
        Warning,
        Info
    }

    private final EntityType entityType;
    private final String entityName;
    private final String memberName;
    private final ChangeType changeType;
    private final Direction direction;
    private final Severity severity;
    private final String description;

    public ContractDelta(EntityType entityType, String entityName, String memberName,
                         ChangeType changeType, Direction direction, Severity severity,
                         String description) {
        this.entityType = entityType;
        this.entityName = entityName;
        this.memberName = memberName;
        this.changeType = changeType;
        this.direction = direction;
        this.severity = severity;
        this.description = description;
    }

    public EntityType getEntityType() { return entityType; }
    public String getEntityName() { return entityName; }
    public String getMemberName() { return memberName; }
    public ChangeType getChangeType() { return changeType; }
    public Direction getDirection() { return direction; }
    public Severity getSeverity() { return severity; }
    public String getDescription() { return description; }
}
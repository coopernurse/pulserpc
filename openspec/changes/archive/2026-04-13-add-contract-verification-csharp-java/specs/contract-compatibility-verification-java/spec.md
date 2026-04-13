## ADDED Requirements

### Requirement: Java ContractAuditor Interface

The system SHALL provide a `ContractAuditor` interface that Java clients can implement to handle verification results:

```java
public interface ContractAuditor {
    void audit(Object result);
    String name();
    
    static ContractAuditor noOp() { ... }
    static ContractAuditor logging() { ... }
    static ContractAuditor failFast() { ... }
}
```

### Requirement: Java Built-in Auditors

The system SHALL provide three built-in auditor implementations via static factory methods:

**noOp()**: Returns a `ContractAuditor` that performs no action, allowing users to inspect results directly.

**logging()**: Returns a `ContractAuditor` that logs deltas at appropriate levels using `System.err` for Error/Warning and `System.out` for Info.

**failFast()**: Returns a `ContractAuditor` that throws `RuntimeException` if any Error-level deltas are found.

### Requirement: Java VerificationResult Class

The system SHALL provide a `VerificationResult` class containing:
- `compatible`: Boolean indicating if any Error-level deltas exist
- `serverChecksum`: SHA-256 checksum from server's IDL
- `clientChecksum`: SHA-256 checksum from client's IDL
- `deltas`: List of all deltas found
- `timestamp`: long epoch millis when verification was performed

### Requirement: Java ContractDelta Class

The system SHALL provide a `ContractDelta` class containing:
- `entityType`: EntityType enum (Interface, Method, Struct, Field, Enum, Error)
- `entityName`: Name of the containing entity
- `memberName`: Name of the specific member (empty for top-level)
- `changeType`: ChangeType enum (Added, Removed, Modified)
- `direction`: Direction enum (ClientHasMore, ClientHasLess, Mismatch)
- `severity`: Severity enum (Error, Warning, Info)
- `description`: Human-readable explanation string

### Requirement: Java verifyCompatibility Method

The Java `Client` class SHALL provide a `verifyCompatibility() -> VerificationResult` method that:
1. Parses the embedded local IDL JSON
2. Retrieves the server's IDL (fetched at bootstrap)
3. Computes the diff
4. Invokes the configured auditor
5. Returns the verification result

### Requirement: Java Auditor Configuration

The Java `Client` class SHALL accept auditor configuration via builder methods `withAuditor(ContractAuditor)` and `verifyOnBootstrap()`.

### Requirement: Java Client Embeds IDL JSON

The Java client code generator SHALL embed `IDL_JSON` constant containing the client's IDL definition, enabling runtime comparison with server's IDL.

#### Scenario: Verify compatibility with no deltas

- **WHEN** client calls `verifyCompatibility()` and both client and server have identical IDL
- **THEN** result is returned with `compatible=true` and empty `deltas`

#### Scenario: Verify compatibility with non-breaking changes

- **WHEN** client calls `verifyCompatibility()` and server has only added optional fields
- **THEN** result is returned with `compatible=true` and `deltas` containing Info-level entries

#### Scenario: Verify compatibility with breaking changes

- **WHEN** client calls `verifyCompatibility()` and server has removed a required field
- **THEN** result is returned with `compatible=false` and `deltas` containing Error-level entry

#### Scenario: Audit with custom auditor implementation

- **WHEN** client configures with a custom auditor that sends metrics to a monitoring service
- **THEN** after any verification call, the custom auditor receives the result

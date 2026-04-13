## ADDED Requirements

### Requirement: C# ContractAuditor Interface

The system SHALL provide a `IContractAuditor` interface that C# clients can implement to handle verification results:

```csharp
public interface IContractAuditor
{
    void Audit(VerificationResult result);
    string Name { get; }
}
```

### Requirement: C# Built-in Auditors

The system SHALL provide three built-in auditor implementations:

**NoOpAuditor**: Implements `IContractAuditor` and performs no action, allowing users to inspect results directly.

**LoggingAuditor**: Implements `IContractAuditor` and logs deltas at appropriate levels:
- `Console.Error.WriteLine` for Error severity
- `Console.Error.WriteLine` for Warning severity  
- `Console.WriteLine` for Info severity

**FailFastAuditor**: Implements `IContractAuditor` and throws `InvalidOperationException` if any Error-level deltas are found.

### Requirement: C# VerificationResult Structure

The system SHALL provide a `VerificationResult` class containing:
- `Compatible`: Boolean indicating if any Error-level deltas exist
- `ServerChecksum`: SHA-256 checksum from server's IDL
- `ClientChecksum`: SHA-256 checksum from client's IDL
- `Deltas`: ReadOnlyList of all deltas found
- `Timestamp`: DateTime when verification was performed

### Requirement: C# ContractDelta Structure

The system SHALL provide a `ContractDelta` record containing:
- `EntityType`: EntityType enum (Interface, Method, Struct, Field, Enum, Error)
- `EntityName`: Name of the containing entity
- `MemberName`: Name of the specific member (empty for top-level)
- `ChangeType`: ChangeType enum (Added, Removed, Modified)
- `Direction`: Direction enum (ClientHasMore, ClientHasLess, Mismatch)
- `Severity`: Severity enum (Error, Warning, Info)
- `Description`: Human-readable explanation string

### Requirement: C# VerifyCompatibility Method

The C# `Client` class SHALL provide a `VerifyCompatibility(CancellationToken) -> Task<VerificationResult>` method that:
1. Parses the embedded local IDL JSON
2. Retrieves the server's IDL (fetched at bootstrap)
3. Computes the diff
4. Invokes the configured auditor
5. Returns the verification result

### Requirement: C# Auditor Configuration

The C# `Client` class SHALL accept auditor configuration via `ClientOptions.WithAuditor(IContractAuditor)` and `ClientOptions.VerifyOnBootstrap()`.

### Requirement: C# Client Embeds IDL JSON

The C# client code generator SHALL embed `IDL_JSON` constant containing the client's IDL definition, enabling runtime comparison with server's IDL.

#### Scenario: Verify compatibility with no deltas

- **WHEN** client calls `VerifyCompatibility()` and both client and server have identical IDL
- **THEN** result is returned with `Compatible=true` and empty `Deltas`

#### Scenario: Verify compatibility with non-breaking changes

- **WHEN** client calls `VerifyCompatibility()` and server has only added optional fields
- **THEN** result is returned with `Compatible=true` and `Deltas` containing Info-level entries

#### Scenario: Verify compatibility with breaking changes

- **WHEN** client calls `VerifyCompatibility()` and server has removed a required field
- **THEN** result is returned with `Compatible=false` and `Deltas` containing Error-level entry

#### Scenario: Audit with custom auditor implementation

- **WHEN** client configures with a custom auditor that sends metrics to a monitoring service
- **THEN** after any verification call, the custom auditor receives the result

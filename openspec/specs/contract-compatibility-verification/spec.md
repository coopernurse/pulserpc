# Contract Compatibility Verification

## ADDED Requirements

### Requirement: Structural Diff Detection

The system SHALL compute a directional structural diff between client and server IDL definitions, identifying differences in interfaces, methods, structs, fields, enums, enum values, and error codes.

### Requirement: Direction Classification

The diff SHALL classify each delta by direction:
- **ClientHasMore**: Client has entity server doesn't know (client too new)
- **ClientHasLess**: Server has entity client doesn't know (client too old)
- **Mismatch**: Both have but incompatible (type mismatch, signature change)

### Requirement: Severity Classification

The diff SHALL classify each delta by severity based on runtime impact:

| Delta Type | Direction | Severity |
|------------|----------|----------|
| Struct removed | ClientHasMore | Error |
| Struct added | ClientHasLess | Info |
| Field type changed | Mismatch | Error |
| Field removed | ClientHasMore | Info |
| Required field added | ClientHasLess | Error |
| Optional field added | ClientHasLess | Info |
| Field made optional | ClientHasLess | Info |
| Field made required | ClientHasLess | Warning |
| Method removed | ClientHasMore | Error |
| Method added | ClientHasLess | Warning |
| Method signature changed | Mismatch | Error |
| Enum value removed | ClientHasMore | Warning |
| Enum value added | ClientHasLess | Warning |
| Error removed | ClientHasMore | Info |
| Error added | ClientHasLess | Info |

### Requirement: Verification Result Structure

The system SHALL return a `VerificationResult` containing:
- `Compatible`: Boolean indicating if any Error-level deltas exist
- `ServerChecksum`: SHA-256 checksum from server's IDL
- `ClientChecksum`: SHA-256 checksum from client's IDL
- `Deltas`: List of all deltas found (may be empty)
- `Timestamp`: When verification was performed

Each delta SHALL contain:
- `EntityType`: What kind of entity (Interface, Method, Struct, Field, Enum, etc.)
- `EntityName`: Name of the containing entity
- `MemberName`: Name of the specific member (empty for top-level)
- `ChangeType`: Added, Removed, Modified
- `Direction`: ClientHasMore, ClientHasLess, Mismatch
- `Severity`: Error, Warning, Info
- `Description`: Human-readable explanation

### Requirement: Auditor Interface

The system SHALL provide a `ContractAuditor` interface:

```go
type ContractAuditor interface {
    Audit(ctx context.Context, result *VerificationResult)
    Name() string
}
```

Auditors are invoked after every verification call, regardless of whether deltas were found.

### Requirement: Built-in NoOp Auditor

The system SHALL provide a `NoOpAuditor` that implements `ContractAuditor` and performs no action. This allows users to inspect results directly without automatic handling.

### Requirement: Built-in Logging Auditor

The system SHALL provide a `LoggingAuditor` that implements `ContractAuditor` and logs:
- Error-level for `SeverityError` deltas
- Warning-level for `SeverityWarning` deltas
- Info-level for `SeverityInfo` deltas

### Requirement: Built-in FailFast Auditor

The system SHALL provide a `FailFastAuditor` that implements `ContractAuditor` and panics if any `SeverityError` deltas are found.

### Requirement: Client Embeds Local IDL

The client code generator SHALL embed `IDL_JSON` constant containing the client's IDL definition, enabling runtime comparison with server's IDL.

### Requirement: VerifyCompatibility Method

The Go client SHALL provide a `VerifyCompatibility(ctx context.Context) *VerificationResult` method that:
1. Parses the embedded local IDL
2. Retrieves the server's IDL (fetched at bootstrap)
3. Computes the diff
4. Invokes the configured auditor
5. Returns the verification result

### Requirement: Auditor Configuration Option

The client SHALL accept `WithAuditor(auditor ContractAuditor)` as a dial option to configure automatic auditing.

### Requirement: VerifyOnBootstrap Option

The client SHALL accept `VerifyOnBootstrap()` as a dial option that triggers automatic verification during client initialization.

#### Scenario: Compatibility verification with no deltas

- **WHEN** client calls `VerifyCompatibility()` and both client and server have identical IDL
- **THEN** result is returned with `Compatible=true` and empty `Deltas`

#### Scenario: Compatibility verification with non-breaking changes

- **WHEN** client calls `VerifyCompatibility()` and server has only added optional fields
- **THEN** result is returned with `Compatible=true` and `Deltas` containing Info-level entries

#### Scenario: Compatibility verification with breaking changes

- **WHEN** client calls `VerifyCompatibility()` and server has removed a required field
- **THEN** result is returned with `Compatible=false` and `Deltas` containing Error-level entry

#### Scenario: Auditor is called on every verification

- **WHEN** client calls `VerifyCompatibility()` multiple times with no changes
- **THEN** auditor is called each time, allowing logging of "still compatible" status

#### Scenario: Audit with custom auditor implementation

- **WHEN** client dials with a custom auditor that sends metrics to a monitoring service
- **THEN** after any verification call, the custom auditor receives the result
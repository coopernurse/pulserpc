# ts-contract-verification Specification

## Purpose
TBD - created by archiving change python-ts-contract-verification. Update Purpose after archive.
## Requirements
### Requirement: DiffIDL Function

The TypeScript runtime SHALL provide a `diffIDL(clientIDL: any, serverIDL: any): ContractDelta[]` function that computes a directional structural diff between client and server IDL definitions, identifying differences in interfaces, methods, structs, fields, enums, enum values, and error codes.

#### Scenario: Identical IDLs produces no deltas
- **WHEN** `diffIDL()` is called with identical client and server IDL
- **THEN** an empty array is returned

#### Scenario: Detects added optional field
- **WHEN** server has an optional field that client doesn't have
- **THEN** a delta with `changeType: 'Added'`, `direction: 'ClientHasLess'`, `severity: 'Info'` is returned

#### Scenario: Detects added required field
- **WHEN** server has a required field that client doesn't have
- **THEN** a delta with `changeType: 'Added'`, `direction: 'ClientHasLess'`, `severity: 'Error'` is returned

#### Scenario: Detects removed field
- **WHEN** client has a field that server doesn't have
- **THEN** a delta with `changeType: 'Removed'`, `direction: 'ClientHasMore'`, `severity: 'Info'` is returned

#### Scenario: Detects field type change
- **WHEN** a field exists in both client and server but with different types
- **THEN** a delta with `changeType: 'Modified'`, `direction: 'Mismatch'`, `severity: 'Error'` is returned

### Requirement: ContractDelta Interface

The TypeScript runtime SHALL provide a `ContractDelta` interface with the following properties:
- `entityType`: EntityType - What kind of entity (Interface, Method, Struct, Field, Enum, etc.)
- `entityName`: string - Name of the containing entity
- `memberName`: string - Name of the specific member (empty for top-level)
- `changeType`: ChangeType - Added, Removed, Modified
- `direction`: Direction - ClientHasMore, ClientHasLess, Mismatch
- `severity`: Severity - Error, Warning, Info
- `description`: string - Human-readable explanation

### Requirement: VerificationResult Interface

The TypeScript runtime SHALL provide a `VerificationResult` interface with the following properties:
- `compatible`: boolean - Boolean indicating if any Error-level deltas exist
- `serverChecksum`: string - Checksum from server's IDL
- `clientChecksum`: string - Checksum from client's IDL
- `deltas`: ContractDelta[] - List of all deltas found
- `timestamp`: Date - When verification was performed

### Requirement: ContractAuditor Interface

The TypeScript runtime SHALL provide an `ContractAuditor` interface:

```typescript
interface ContractAuditor {
  audit(result: VerificationResult): void;
  name(): string;
}
```

Auditors are invoked after every verification call, regardless of whether deltas were found.

### Requirement: Built-in NoOp Auditor

The TypeScript runtime SHALL provide a `NoOpAuditor` class that implements `ContractAuditor` and performs no action. This allows users to inspect results directly without automatic handling.

### Requirement: Built-in Logging Auditor

The TypeScript runtime SHALL provide a `LoggingAuditor` class that implements `ContractAuditor` and logs:
- Error-level for Error severity deltas
- Warning-level for Warning severity deltas
- Info-level for Info severity deltas

### Requirement: Built-in FailFast Auditor

The TypeScript runtime SHALL provide a `FailFastAuditor` class that implements `ContractAuditor` and throws an error if any Error severity deltas are found.

### Requirement: Client Loads Local IDL from idl.json

The TypeScript client SHALL load local IDL from `idl.json` file located by walking up from the client module's directory, enabling runtime comparison with server's IDL without requiring explicit `setLocalIDL()`.

#### Scenario: Local IDL loaded automatically
- **WHEN** TypeScript client is instantiated
- **THEN** it attempts to load IDL from `idl.json` relative to the client module
- **IF** `idl.json` is not found, verification uses server IDL only (no local comparison)

### Requirement: VerifyCompatibility Async Method

The TypeScript client SHALL provide a `verifyCompatibility(): Promise<VerificationResult>` method that:
1. Uses the local IDL (from `idl.json` or `setLocalIDL()`)
2. Retrieves the server's IDL (fetched at bootstrap via `pulserpc-idl`)
3. Computes the diff
4. Invokes the configured auditor
5. Returns the verification result

#### Scenario: Compatibility verification with no deltas
- **WHEN** client calls `verifyCompatibility()` and both client and server have identical IDL
- **THEN** result is returned with `compatible: true` and empty `deltas`

#### Scenario: Compatibility verification with non-breaking changes
- **WHEN** client calls `verifyCompatibility()` and server has only added optional fields
- **THEN** result is returned with `compatible: true` and `deltas` containing Info-level entries

#### Scenario: Compatibility verification with breaking changes
- **WHEN** client calls `verifyCompatibility()` and the server has added a required field the client does not send (a `Required field added` / `ClientHasLess` delta)
- **THEN** result is returned with `compatible: false` and `deltas` containing an Error-level entry

#### Scenario: Compatibility verification with a removed field
- **WHEN** client calls `verifyCompatibility()` and the server has removed a required field the client still has (a `Field removed` / `ClientHasMore` delta)
- **THEN** result is returned with `compatible: true` and `deltas` containing an Info-level entry

#### Scenario: Auditor is called on every verification
- **WHEN** client calls `verifyCompatibility()` multiple times with no changes
- **THEN** auditor is called each time, allowing logging of "still compatible" status

### Requirement: Client Constructor Options

The TypeScript client constructor SHALL accept an options object with the following properties:
- `auditor`: `ContractAuditor` instance for automatic auditing (optional)
- `verifyOnBootstrap`: boolean to trigger automatic verification during client initialization (optional, default false)

#### Scenario: Client with auditor option
- **WHEN** `const client = await Client.create(transport, { auditor: new LoggingAuditor() })` is called
- **THEN** verification results are passed to the auditor after each call

#### Scenario: Client with verifyOnBootstrap option
- **WHEN** `const client = await Client.create(transport, { verifyOnBootstrap: true })` is called
- **THEN** `verifyCompatibility()` is called automatically after bootstrap

### Requirement: SetLocalIDL Method

The TypeScript client SHALL provide a `setLocalIDL(idlJson: string)` method to override the local IDL for verification. This allows users to provide their own IDL (e.g., loaded from a file) instead of relying on `idl.json`.

#### Scenario: Set local IDL explicitly
- **WHEN** `client.setLocalIDL('{"interfaces": [...]}')` is called with valid IDL JSON
- **THEN** subsequent `verifyCompatibility()` calls use the provided IDL


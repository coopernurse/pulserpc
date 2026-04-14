# python-contract-verification Specification

## Purpose
TBD - created by archiving change python-ts-contract-verification. Update Purpose after archive.
## Requirements
### Requirement: DiffIDL Function

The Python runtime SHALL provide a `diff_idl(client_idl, server_idl)` function that computes a directional structural diff between client and server IDL definitions, identifying differences in interfaces, methods, structs, fields, enums, enum values, and error codes.

#### Scenario: Identical IDLs produces no deltas
- **WHEN** `diff_idl()` is called with identical client and server IDL
- **THEN** an empty list is returned

#### Scenario: Detects added optional field
- **WHEN** server has an optional field that client doesn't have
- **THEN** a delta with `ChangeType.Added`, `Direction.ClientHasLess`, `Severity.Info` is returned

#### Scenario: Detects added required field
- **WHEN** server has a required field that client doesn't have
- **THEN** a delta with `ChangeType.Added`, `Direction.ClientHasLess`, `Severity.Error` is returned

#### Scenario: Detects removed field
- **WHEN** client has a field that server doesn't have
- **THEN** a delta with `ChangeType.Removed`, `Direction.ClientHasMore`, `Severity.Info` is returned

#### Scenario: Detects field type change
- **WHEN** a field exists in both client and server but with different types
- **THEN** a delta with `ChangeType.Modified`, `Direction.Mismatch`, `Severity.Error` is returned

### Requirement: ContractDelta Dataclass

The Python runtime SHALL provide a `ContractDelta` class with the following attributes:
- `entity_type`: str - What kind of entity (Interface, Method, Struct, Field, Enum, etc.)
- `entity_name`: str - Name of the containing entity
- `member_name`: str - Name of the specific member (empty for top-level)
- `change_type`: str - Added, Removed, Modified
- `direction`: str - ClientHasMore, ClientHasLess, Mismatch
- `severity`: str - Error, Warning, Info
- `description`: str - Human-readable explanation

### Requirement: VerificationResult Dataclass

The Python runtime SHALL provide a `VerificationResult` class with the following attributes:
- `compatible`: bool - Boolean indicating if any Error-level deltas exist
- `server_checksum`: str - Checksum from server's IDL
- `client_checksum`: str - Checksum from client's IDL
- `deltas`: List[ContractDelta] - List of all deltas found
- `timestamp`: datetime - When verification was performed

### Requirement: ContractAuditor Abstract Base Class

The Python runtime SHALL provide a `ContractAuditor` abstract base class:

```python
from abc import ABC, abstractmethod

class ContractAuditor(ABC):
    @abstractmethod
    def audit(self, result: VerificationResult) -> None:
        pass
    
    @abstractmethod
    def name(self) -> str:
        pass
```

Auditors are invoked after every verification call, regardless of whether deltas were found.

### Requirement: Built-in NoOp Auditor

The Python runtime SHALL provide a `NoOpAuditor` class that implements `ContractAuditor` and performs no action. This allows users to inspect results directly without automatic handling.

### Requirement: Built-in Logging Auditor

The Python runtime SHALL provide a `LoggingAuditor` class that implements `ContractAuditor` and logs:
- Error-level for Error severity deltas
- Warning-level for Warning severity deltas
- Info-level for Info severity deltas

### Requirement: Built-in FailFast Auditor

The Python runtime SHALL provide a `FailFastAuditor` class that implements `ContractAuditor` and raises an exception if any Error severity deltas are found.

### Requirement: Client Loads Local IDL from idl.json

The Python client SHALL load local IDL from `idl.json` file located by walking up from the client module's directory, enabling runtime comparison with server's IDL without requiring explicit `set_local_idl()`.

#### Scenario: Local IDL loaded automatically
- **WHEN** Python client is instantiated
- **THEN** it attempts to load IDL from `idl.json` relative to the client module
- **IF** `idl.json` is not found, verification uses server IDL only (no local comparison)

### Requirement: VerifyCompatibility Method

The Python client SHALL provide a `verify_compatibility()` method that:
1. Uses the local IDL (from `idl.json` or `set_local_idl()`)
2. Retrieves the server's IDL (fetched at bootstrap via `pulserpc-idl`)
3. Computes the diff
4. Invokes the configured auditor
5. Returns the verification result

#### Scenario: Compatibility verification with no deltas
- **WHEN** client calls `verify_compatibility()` and both client and server have identical IDL
- **THEN** result is returned with `compatible=True` and empty `deltas`

#### Scenario: Compatibility verification with non-breaking changes
- **WHEN** client calls `verify_compatibility()` and server has only added optional fields
- **THEN** result is returned with `compatible=True` and `deltas` containing Info-level entries

#### Scenario: Compatibility verification with breaking changes
- **WHEN** client calls `verify_compatibility()` and server has removed a required field
- **THEN** result is returned with `compatible=False` and `deltas` containing Error-level entry

#### Scenario: Auditor is called on every verification
- **WHEN** client calls `verify_compatibility()` multiple times with no changes
- **THEN** auditor is called each time, allowing logging of "still compatible" status

### Requirement: Client Options Dict

The Python client SHALL accept an options dictionary with the following keys:
- `auditor`: `ContractAuditor` instance for automatic auditing (optional)
- `verify_on_bootstrap`: bool to trigger automatic verification during client initialization (optional, default False)

#### Scenario: Client with auditor option
- **WHEN** `Client(transport, options={"auditor": LoggingAuditor()})` is instantiated
- **THEN** verification results are passed to the auditor after each call

#### Scenario: Client with verify_on_bootstrap option
- **WHEN** `Client(transport, options={"verify_on_bootstrap": True})` is instantiated
- **THEN** `verify_compatibility()` is called automatically after bootstrap

### Requirement: SetLocalIDL Method

The Python client SHALL provide a `set_local_idl(idl_json: str)` method to override the local IDL for verification. This allows users to provide their own IDL (e.g., loaded from a file) instead of relying on `idl.json`.

#### Scenario: Set local IDL explicitly
- **WHEN** `client.set_local_idl('{"interfaces": [...]}')` is called with valid IDL JSON
- **THEN** subsequent `verify_compatibility()` calls use the provided IDL


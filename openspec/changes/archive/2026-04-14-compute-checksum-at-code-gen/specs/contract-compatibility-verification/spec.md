## MODIFIED Requirements

### Requirement: Verification Result Structure

**FROM:**
```
The system SHALL return a `VerificationResult` containing:
- `Compatible`: Boolean indicating if any Error-level deltas exist
- `ServerChecksum`: SHA-256 checksum from server's IDL
- `ClientChecksum`: SHA-256 checksum from client's IDL
- `Deltas`: List of all deltas found (may be empty)
- `Timestamp`: When verification was performed
```

**TO:**
```
The system SHALL return a `VerificationResult` containing:
- `Compatible`: Boolean indicating if any Error-level deltas exist
- `ServerChecksum`: Checksum from server's IDL (computed at code generation)
- `ClientChecksum`: Checksum from client's IDL (computed at code generation)
- `Deltas`: List of all deltas found (may be empty)
- `Timestamp`: When verification was performed
```

**Rationale for change**: Checksums are now computed once at code generation time and stored in idl.json, rather than computed at runtime. This eliminates cross-language checksum divergence risk.

#### Scenario: Verification result includes code-generation checksums
- **WHEN** client calls `VerifyCompatibility()`
- **THEN** `ServerChecksum` and `ClientChecksum` are extracted from stored checksum fields
- **AND** checksums match values computed at code generation time

### Requirement: Client Embeds Local IDL

**FROM:**
```
The client code generator SHALL embed `IDL_JSON` constant containing the client's IDL definition, enabling runtime comparison with server's IDL.
```

**TO:**
```
The client code generator SHALL embed the client's IDL definition, which includes a top-level `checksum` field. The checksum enables runtime comparison with server's IDL.
```

**Rationale for change**: Clarify that the embedded IDL includes the checksum field, not just the IDL data.

#### Scenario: Embedded IDL includes checksum field
- **WHEN** client code is generated
- **THEN** the embedded IDL constant includes the `checksum` field at top level
- **AND** the checksum enables runtime comparison without recomputation

## ADDED Requirements

### Requirement: IDL JSON Includes Checksum Field

The code generator SHALL write idl.json with a top-level `checksum` field containing the SHA-256 checksum of the IDL data. The idl.json format SHALL be:

```json
{
  "interfaces": [...],
  "structs": [...],
  "enums": [...],
  "errors": [...],
  "checksum": "<sha256-hex>"
}
```

#### Scenario: Generated idl.json includes checksum
- **WHEN** code generator produces idl.json
- **THEN** the file contains a `checksum` field with a valid SHA-256 hex string

#### Scenario: Existing idl.json without checksum is backward compatible
- **WHEN** a runtime reads an idl.json without `checksum` field
- **THEN** verification proceeds with empty checksums (compatible mode)

### Requirement: Server Returns IDL With Checksum

The server's `pulserpc-idl` RPC endpoint SHALL return the server's IDL including the `checksum` field. The response format SHALL be:

```json
{
  "result": {
    "interfaces": [...],
    "structs": [...],
    "enums": [...],
    "errors": [...],
    "checksum": "<sha256-hex>"
  }
}
```

#### Scenario: Client receives server IDL with checksum
- **WHEN** client calls `pulserpc-idl` RPC
- **THEN** the response includes the `checksum` field from the server's idl.json

### Requirement: Client Reads Checksum Without Computation

The client SHALL extract `ClientChecksum` from the locally embedded/generated IDL data, and `ServerChecksum` from the server's `pulserpc-idl` response. No runtime checksum computation SHALL be required.

#### Scenario: Verification uses stored checksums
- **WHEN** client calls `VerifyCompatibility()`
- **THEN** `ClientChecksum` is read from embedded IDL's `checksum` field
- **AND** `ServerChecksum` is read from server response's `checksum` field
- **AND** no SHA-256 computation occurs at runtime

**Note:** Python and TypeScript runtime support for contract compatibility verification is deferred to a future change. The checksum extraction utilities (`extract_checksum` in Python, `extractChecksum` in TypeScript) have been added to the types modules, but the full `VerifyCompatibility` implementation is not yet complete for these runtimes.

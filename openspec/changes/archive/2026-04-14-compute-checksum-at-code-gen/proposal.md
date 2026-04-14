# Proposal: Compute Checksum at Code-Gen Time

## Summary

Currently, each runtime (Go, C#, Java) re-implements SHA-256 checksum computation to populate `VerificationResult`. This is:
- **Redundant**: SHA-256 is computed multiple times across languages
- **Risk of divergence**: Different JSON serialization could produce different checksums
- **Unnecessary complexity**: The checksum represents the IDL at code-gen time, not runtime

This change moves checksum computation to code generation:
- Go generator computes SHA-256 when writing idl.json
- idl.json gains a top-level `checksum` field
- Runtimes simply read the checksum from idl.json (or server response) - no computation needed

## Motivation

When implementing contract compatibility verification for C# and Java, each runtime re-implements `ComputeChecksum()`. This creates risk:

1. **Cross-language divergence**: Java uses `MessageDigest.getInstance("SHA-256")`, C# uses `SHA256.HashData()`, Go uses `sha256.Sum256()`. While all should produce the same result, JSON serialization can differ between languages.

2. **Unnecessary code**: Checksum represents the IDL as it existed at code-gen time. Computing it at runtime is conceptually wrong - the IDL doesn't change after code-gen.

3. **Python/TypeScript burden**: These runtimes don't yet have verification. If they need to implement SHA-256, that's another language to potentially diverge.

## Proposed Solution

### 1. Add `checksum` to idl.json

```json
{
  "idl": { ... },
  "checksum": "abc123..."
}
```

The Go generator already computes this checksum. We just need to:
- Store it in idl.json instead of discarding it
- Include it in the JSON output alongside the IDL data

### 2. Server returns checksum in pulserpc-idl response

```go
// Server's handler for pulserpc-idl
{
  "result": {
    "idl": { ... },
    "checksum": "xyz789..."
  }
}
```

### 3. Runtimes read checksum, don't compute

**Client (verification):**
- `ClientChecksum` = read from local idl.json
- `ServerChecksum` = from server's pulserpc-idl response (or read from server's idl.json if available)
- No SHA-256 implementation needed

**VerificationResult** stays the same structure, just sourcing changes.

## Scope

### Changes Required

1. **Go generator** (`pkg/generator/go_client_server.go`):
   - Compute checksum during idl.json write
   - Add `checksum` field to output JSON

2. **Go runtime** (`pkg/runtime/runtimes/go/pulserpc/`):
   - Server: include checksum in pulserpc-idl response
   - Client: read checksum from server response (not ComputeChecksum)
   - Remove or deprecate `ComputeChecksum()` function (kept for backward compat with tests)

3. **C# runtime** (`pkg/runtime/runtimes/csharp/PulseRPC/`):
   - Remove `DiffEngine.ComputeChecksum()`
   - Client: read checksum from embedded IDL at code-gen time (already embedded)
   - Keep `VerificationResult.ServerChecksum` from server response

4. **Java runtime** (`pkg/runtime/runtimes/java/com/bitmechanic/pulserpc/`):
   - Remove `DiffEngine.computeChecksum()`
   - Client: read checksum from embedded IDL at code-gen time
   - Keep `VerificationResult.serverChecksum` from server response

5. **Python runtime** (`pkg/runtime/runtimes/python/`):
   - Implement verification reading checksum from local idl.json
   - No SHA-256 implementation needed

6. **TypeScript runtime** (`pkg/runtime/runtimes/typescript/`):
   - Implement verification reading checksum from embedded IDL
   - No SHA-256 implementation needed

### Non-Goals

- Not changing the severity classification matrix
- Not changing the diff algorithm
- Not modifying the Go generator's Go-side checksum computation (it's the source of truth)

## Alternatives Considered

### 1. Keep computing at runtime (status quo)

Rejected because:
- Introduces cross-language checksum divergence risk
- Unnecessary code duplication
- Wrong conceptual model (computing something that was determined at code-gen)

### 2. Compute checksum in all languages and verify they match

Rejected because:
- Adds complexity (cross-language test infrastructure)
- Doesn't solve the root problem (divergence risk)
- Still requires SHA-256 in Python/TypeScript

### 3. Use a shared library for checksum computation

Rejected because:
- Would require significant refactoring
- Overkill for a single hash function
- Go is already the code generator - it's natural to compute there

## Impact

- **Go generator**: Minor change (store computed checksum)
- **Go runtime**: Small change (include checksum in response)
- **C# runtime**: Remove code, change checksum source
- **Java runtime**: Remove code, change checksum source
- **Python/TypeScript**: Simpler implementation (read only, no SHA-256)

This change also provides a template for how Python/TypeScript should implement verification - read checksum from idl.json, don't compute it.

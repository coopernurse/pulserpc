## Context

Contract compatibility verification was implemented across Go, C#, and Java runtimes. Each runtime computes SHA-256 checksums at runtime using language-specific JSON serialization. This creates cross-language divergence risk and is conceptually wrong (the checksum represents IDL at code-gen time, not runtime).

The Go generator already computes checksums during code generation, but they're not being stored. Instead, each runtime re-computes them.

## Goals / Non-Goals

**Goals:**
- Move checksum computation to code generation time
- Store checksum in idl.json alongside IDL data
- Eliminate SHA-256 implementations from Python and TypeScript runtimes
- Maintain backward compatibility with existing idl.json format

**Non-Goals:**
- Not changing the diff algorithm or severity classification
- Not changing the VerificationResult structure
- Not eliminating all runtime SHA-256 (Go's generator still needs it)

## Decisions

### Decision: Add `checksum` as top-level field in idl.json

**Chosen:** Store checksum alongside IDL data, not wrapped inside

```json
// idl.json (new format)
{
  "interfaces": [...],
  "structs": [...],
  "enums": [...],
  "checksum": "abc123..."
}
```

**Rationale:** Backward compatible. Existing code that parses idl.json and extracts `interfaces`/`structs`/`enums` continues to work. Extra field (`checksum`) is ignored by existing parsers.

**Alternatives Considered:**
- Wrap in object `{ "idl": {...}, "checksum": "..." }`: Breaks backward compat, requires all parsers to change
- Separate file `idl_checksum.json`: Requires two files, more complex

### Decision: Server returns checksum in pulserpc-idl response

**Chosen:** Server's `s.contract.idlParsed` already contains checksum (since it's in idl.json). Response naturally includes it.

```json
// Response to pulserpc-idl
{
  "result": {
    "interfaces": [...],
    "structs": [...],
    "enums": [...],
    "checksum": "xyz789..."
  }
}
```

**Rationale:** Minimal change. Server doesn't need to know about checksum specifically - it just returns the contract data it already has.

### Decision: Client reads checksum from server response

**Chosen:** Client extracts `checksum` from `pulserpc-idl` response for `ServerChecksum`.

**Rationale:** No computation needed. Server computed checksum at code-gen and stored in idl.json.

### Decision: Client's `ClientChecksum` from embedded/generated IDL

**Chosen:** Generated clients embed the full idl.json (or at least the checksum).

For Go/C#/Java: Generated client already has IDL embedded. The embedded IDL contains the checksum field, so client reads it from there.

**Rationale:** The embedded IDL represents what the client was generated from. Its checksum is the client's version identifier.

## Risks / Trade-offs

[Risk] **Existing idl.json files lack checksum** → Mitigation: Existing files work fine (checksum field just missing). Client reads from server response. Or regenerate idl.json files with new generator.

[Risk] **C#/Java still compute checksums** → Mitigation: After this change, they read from embedded IDL instead. Can remove ComputeChecksum later.

[Risk] **Python/TypeScript embedded IDL** → Mitigation: These runtimes need to embed idl.json or its checksum. Design choice deferred to their implementation.

## Implementation Summary

### 1. Go Generator (`pkg/generator/go_client_server.go`)

Modify `writeIDLJSONGo()`:

```go
func writeIDLJSONGo(idl *parser.IDL, outputDir string, fs *flag.FlagSet) error {
    // Compute checksum of the IDL
    idlBytes, _ := json.Marshal(idl)
    hash := sha256.Sum256(idlBytes)
    checksum := fmt.Sprintf("%x", hash)

    // Create output with IDL + checksum
    output := map[string]interface{}{
        "interfaces": idl.Interfaces,
        "structs":    idl.Structs,
        "enums":      idl.Enums,
        "errors":     idl.Errors,
        "checksum":   checksum,
    }

    idlJSON, _ := json.MarshalIndent(output, "", "  ")
    // ... write to file
}
```

### 2. Go Runtime Server (`pkg/runtime/runtimes/go/pulserpc/server.go`)

No change needed! The `s.contract.idlParsed` already contains checksum since idl.json now includes it.

### 3. Go Runtime Client (`pkg/runtime/runtimes/go/pulserpc/client.go`)

Modify `VerifyCompatibility()`:

```go
// Old: ComputeChecksum(clientIDL), ComputeChecksum(serverIDL)
// New: Read from data

func (c *Client) VerifyCompatibility(ctx context.Context) *VerificationResult {
    // Get client IDL and its checksum
    var clientIDL interface{}
    var clientChecksum string
    if c.localIDL != nil {
        clientIDL = c.localIDL
    } else {
        clientIDL = c.contract.idlParsed
    }
    // Extract checksum from client IDL data
    if dict, ok := clientIDL.(map[string]interface{}); ok {
        clientChecksum, _ = dict["checksum"].(string)
    }

    // Get server IDL and its checksum
    serverIDL := c.contract.idlParsed
    var serverChecksum string
    if dict, ok := serverIDL.(map[string]interface{}); ok {
        serverChecksum, _ = dict["checksum"].(string)
    }

    deltas := DiffIDL(clientIDL, serverIDL)
    // ... rest unchanged
}
```

### 4. C# Runtime

Remove `DiffEngine.ComputeChecksum()`. Read checksum from embedded `IDL_JSON`:

```csharp
// In generated client's constructor or SetLocalIDL:
var idlObj = JsonSerializer.Deserialize<object>(IDL_JSON);
if (idlObj is JsonElement element && element.TryGetProperty("checksum", out var checksumProp))
{
    _clientChecksum = checksumProp.GetString();
}
```

### 5. Java Runtime

Remove `DiffEngine.computeChecksum()`. Read checksum from embedded `IDL_JSON`:

```java
// In generated client's constructor:
Object idlDoc = jsonParser.fromJson(IDL_JSON, Object.class);
if (idlDoc instanceof Map) {
    String checksum = (String) ((Map) idlDoc).get("checksum");
}
```

### 6. Python Runtime

Read checksum from local `idl.json`:

```python
with open('idl.json', 'r') as f:
    idl_data = json.load(f)
client_checksum = idl_data.get('checksum', '')
```

### 7. TypeScript Runtime

Read checksum from embedded IDL or load from file.

## Migration Plan

1. **Phase 1**: Go generator adds checksum to idl.json
   - Existing idl.json files continue to work (no checksum = use empty or compute at runtime)
   - Regenerate files to get new format

2. **Phase 2**: Go runtime reads checksum instead of computing
   - Client extracts from `idlParsed` or server response
   - Server unchanged (already returns idlParsed)

3. **Phase 3**: C# and Java remove ComputeChecksum
   - Read from embedded IDL instead
   - Verification continues to work

4. **Phase 4**: Python and TypeScript implement verification
   - Read checksum from idl.json (Python) or embedded IDL (TypeScript)
   - No SHA-256 implementation needed

## Open Questions

1. **Backward compat for existing idl.json**: Should we require checksum or allow missing? Recommendation: Allow missing, client falls back to empty checksum or computes.

2. **Python embedded IDL**: Does Python currently embed IDL anywhere, or always load from file? If embed, where?

3. **TypeScript embedded IDL**: Same question - does TypeScript embed or load from file?

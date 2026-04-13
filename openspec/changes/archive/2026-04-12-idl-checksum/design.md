## Context

The parser generates `idl.json` files that are consumed by client code generators. Clients currently have no way to verify whether their cached IDL matches the server's current definition. Adding a checksum allows clients to detect drift without comparing the entire IDL content.

## Goals / Non-Goals

**Goals:**
- Compute a deterministic checksum from structural IDL elements
- Exclude whitespace and comments (non-structural)
- Ensure checksum is stable regardless of element ordering in the source file
- Use namespace-qualified names for type references to avoid ambiguity
- Include error codes (which affect runtime behavior) but exclude error messages

**Non-Goals:**
- Cryptographic security (checksum is for contract matching, not tamper detection)
- Tracking changes between versions (that's a version control problem)
- Computing checksums for individual elements (only the full IDL checksum)

## Decisions

### 1. What elements are included in the checksum

| Element | Include | Rationale |
|---------|---------|-----------|
| `rootNamespace` | ✅ Yes | Namespace qualifications depend on it |
| Interface names (FQN) | ✅ Yes | Part of API surface |
| Method names | ✅ Yes | API surface |
| Parameter names | ✅ Yes | Generated code uses them |
| Parameter types (FQN) | ✅ Yes | Core structural element |
| Return type (FQN) | ✅ Yes | Core structural element |
| `returnOptional` | ✅ Yes | Affects generated signatures |
| `raises` list | ✅ Yes | Affects error handling |
| Struct names (FQN) | ✅ Yes | Part of type system |
| `extends` (FQN) | ✅ Yes | Inheritance is structural |
| Struct fields (name, type, optional) | ✅ Yes | Core type definition |
| Enum names (FQN) | ✅ Yes | Part of type system |
| Enum values (names only) | ✅ Yes | Values affect wire protocol |
| Error names (FQN) | ✅ Yes | Part of API |
| Error codes | ✅ Yes | Affects runtime behavior |
| Error messages | ❌ No | Documentation only |
| Comments | ❌ No | Already excluded by lexer |
| Whitespace | ❌ No | Already excluded by lexer |

### 2. Normalization of ordering

Since the checksum must be invariant to element ordering:

```
For each namespace:
  Sort interfaces alphabetically by FQN
  For each interface:
    Sort methods alphabetically by name
    For each method:
      Sort parameters alphabetically by name
      Sort raises alphabetically by name

  Sort structs alphabetically by FQN
  For each struct:
    Sort fields alphabetically by name
    (Inherited fields are NOT flattened - extends chain is recorded separately)

  Sort enums alphabetically by FQN
  For each enum:
    Sort values alphabetically by name

Sort errors by FQN
```

### 3. Hash algorithm

SHA-256, encoded as base64url (URL-safe base64 without padding). Output is 43 characters.

```go
hash := sha256.Sum256([]byte(canonicalForm))
checksum := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(hash[:])
```

### 4. Where to compute the checksum

In `pkg/parser/` - a new `checksum.go` file. The parser already has the complete parsed structure (`*IDL`), so computing the checksum there avoids circular dependencies with generators.

The `ParseIDL` function returns `*IDL` which includes all structural elements. A new `ComputeChecksum(idl *IDL) (string, error)` function will:

1. Traverse the IDL in normalized order
2. Build a canonical string representation
3. Hash and encode

### 5. Integration with generators

The checksum is added to `idl.json` as a top-level field:

```json
{
  "rootNamespace": "checkout",
  "checksum": "Lm3HTz...",
  "interfaces": [...],
  "structs": [...],
  "enums": [...],
  "errors": [...]
}
```

Generators call `parser.ComputeChecksum(idl)` after parsing and include it when marshaling to JSON. No changes to generator-specific IDL serialization are needed beyond adding the field to the output.

## Risks / Trade-offs

[Risk] **Cycle detection**: Structs with circular extends (A extends B, B extends A) should be rejected by the validator before checksum computation.
→ Mitigation: Validator already exists; ensure it catches this case.

[Risk] **Type resolution**: References like `BaseResponse` in `book.pulse` must be resolved to fully-qualified `book.BaseResponse` before checksum.
→ Mitigation: Parser maintains namespace context; implement a resolution step before canonicalization.

[Risk] **Forward references**: A struct may reference another struct defined later in the file.
→ Mitigation: Namespace resolution happens post-parse, so order doesn't matter.

[Risk] **Map key type is hardcoded**: The grammar only allows `map[string]ValueType`. If expanded later, historical checksums would be computed differently.
→ Mitigation: Document this constraint. Not a concern for current grammar.

## Migration Plan

1. Add `ComputeChecksum` function to `pkg/parser/checksum.go`
2. Update all generators to compute and include checksum in idl.json output
3. Regenerate all example idl.json files
4. No migration of existing deployments needed - checksum is additive

## Open Questions

- Should the checksum algorithm version be embedded? (e.g., "v1:sha256:...") - Not needed initially, but could help future-proof
- Should we expose a CLI command to compute checksum of a .pulse file directly? - Nice to have, defer to future change

## Context

The PulseRPC Python runtimes (Python2 and Python3) currently:
1. Receive positional parameters in JSON-RPC requests (as array)
2. Convert positional params → named params via `_positional_to_named_params()`
3. Invoke handler with `func(**params)` (named, via kwargs unpacking)

This conversion layer is unnecessary because:
- All PulseRPC methods use positional parameters (codegen enforces this)
- All other runtimes (Go, Java, TypeScript) invoke handler methods positionally
- Python can directly invoke with `func(*params)` (positional, via args unpacking)

Current flow:
```
JSON-RPC params [a, b, c]
        │
        ▼
_positional_to_named_params()
        │
        ▼
{name1: a, name2: b, name3: c}
        │
        ▼
func(**params)
```

Simplified flow:
```
JSON-RPC params [a, b, c]
        │
        ▼
func(*params)
```

## Goals / Non-Goals

**Goals:**
- Simplify Python2 and Python3 runtime handler invocation
- Remove unnecessary named-parameter conversion layer
- Align Python runtime invocation with other runtimes (Go, Java, TypeScript)

**Non-Goals:**
- No behavioral change - handlers receive identical arguments
- No API change - JSON-RPC format unchanged
- No changes to Go, Java, or TypeScript runtimes (they already do it right)
- No changes to codegen - parameter ordering remains positional

## Decisions

### Decision: Use `func(*params)` instead of `func(**params)`

**Choice**: Direct positional invocation with `func(*params)`

**Rationale**:
- Matches how Go, Java, and TypeScript invoke handlers (positionally)
- Removes two conversion methods per runtime
- No loss of functionality - codegen ensures positional parameter ordering
- Simpler code path for debugging

**Alternatives considered**:
- `func(**params)`: Current approach, requires positional→named→positional conversion
- Keep as-is: Maintains unnecessary complexity

### Decision: Keep validation against positional params

**Choice**: Validate using positional params directly from JSON-RPC array

**Rationale**:
- Contract validation already works with positional params
- No need to convert for validation alone

### Decision: Remove `_positional_to_named_params` and `_named_to_positional_params`

**Choice**: Remove both methods from Python2 and Python3 runtimes

**Rationale**:
- These become dead code after simplification
- `_named_to_positional_params` is only used for validating dict-style params, which aren't part of PulseRPC protocol

## Risks / Trade-offs

[Risk] Developer might try to use named params in JSON-RPC request
→ Mitigation: PulseRPC spec requires positional params only; this is enforced by codegen

[Risk] Python2 reaches end-of-life
→ Mitigation: Python2 runtime is maintained for legacy systems; simplification still applies

## Migration Plan

1. Update Python2 runtime first (simpler codebase)
2. Update Python3 runtime (mirrors Python2 structure)
3. Run existing tests to verify no behavioral change
4. No migration needed for users - JSON-RPC protocol unchanged

## Open Questions

None - the simplification is straightforward given the analysis showing all PulseRPC methods use positional parameters.

## Context

The TypeScript generator in `pkg/generator/ts_client_server.go` has inconsistent comment handling. The Go, C#, and Python generators all correctly handle multi-line comments by splitting on `\n` before emitting the comment prefix. The TypeScript generator skips this split for field comments and enum value comments.

## Goals / Non-Goals

**Goals:**
- Make TypeScript comment generation consistent with other language generators
- Fix the 4 affected locations in ts_client_server.go

**Non-Goals:**
- No changes to the parser (already correct)
- No changes to other language generators (already correct)
- No new functionality

## Decisions

### Fix Pattern

**Decision**: Use the same pattern as Go/C# generators: split the comment string and iterate.

**Current (buggy):**
```go
fmt.Fprintf(&sb, "  // %s\n", fieldComment)  // fieldComment may contain \n
```

**Fixed:**
```go
lines := strings.Split(strings.TrimSpace(field.Comment), "\n")
for _, line := range lines {
    fmt.Fprintf(&sb, "  // %s\n", line)
}
```

**Alternatives considered:**
- Helper function - not needed, pattern is simple enough
- Template approach - overkill for this fix

### Test Approach

**Decision**: Add test cases to existing `ts_client_server_test.go` that verify multi-line comment output.

## Risks / Trade-offs

- **Low risk**: This is a straightforward bug fix with matching pattern across 3 other generators
- **No trade-offs**: Fixes incorrect output, no behavior changes

## Locations to Fix

| Line | Context | Type |
|------|---------|------|
| 701 | Enum value comments | non-namespace |
| 742 | Field comments | non-namespace |
| 803 | Enum value comments | namespace |
| 840 | Field comments | namespace |

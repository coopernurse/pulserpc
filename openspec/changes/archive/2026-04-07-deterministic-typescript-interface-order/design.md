## Context

TypeScript code generation in `pkg/generator/ts_client_server.go` iterates over slices of enums, structs, and methods without sorting. The iteration order depends on the internal order from the parser, which may vary between runs even with identical input.

## Goals / Non-Goals

**Goals:**
- Alphabetically sort enum declarations by name
- Alphabetically sort struct/interface declarations by name
- Alphabetically sort method declarations by name within interfaces
- Produce deterministic output across regeneration cycles

**Non-Goals:**
- Changing the API surface or behavior of the generated code
- Modifying the parsing logic

## Decisions

1. **Use `sort.Strings` for simple name sorting**
   - Enums and struct names are simple identifiers
   - Use standard library `sort.Strings` for consistent alphabetical ordering

2. **Sort methods in `generateServerTs` and `generateServerTsForNamespace`**
   - Methods in interface stubs need sorting
   - Apply same sorting approach to client-side generation

3. **Sort struct fields within each struct?**
   - No - field ordering may be semantically significant (e.g., sequential field numbering)
   - Only sort at the declaration level (enums, structs, methods)

## Risks / Trade-offs

- **Risk**: Changing output order could break existing code that depends on enum value ordering
  - **Mitigation**: Enum values within an enum are already ordered by their declaration order; only enum declarations themselves are sorted
- **Trade-off**: Small performance cost for sorting (negligible for typical IDL sizes)
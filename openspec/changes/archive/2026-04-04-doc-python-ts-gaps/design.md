## Context

The Python and TypeScript code generators have undocumented features discovered through source code analysis. This design outlines how to structure the documentation additions to maximize discoverability without duplicating information.

**Current documentation state:**
- CLI reference documents basic flags but misses `--use-pydantic`
- Python reference documents structs as dicts but not Pydantic model generation
- TypeScript reference documents basic usage but misses multi-namespace output structure
- Neither generator documents `idl.json` artifact or test file generation

## Goals / Non-Goals

**Goals:**
- Document all undocumented CLI flags (`--use-pydantic`, `-generate-test-files`)
- Document multi-namespace output structure for both Python and TypeScript
- Document runtime APIs (`InProcTransport`, `Client.validate_*`, `Client.notify()`, `Client.ready()`)
- Document `idl.json` artifact and its runtime role
- Add examples showing multi-namespace code generation

**Non-Goals:**
- Changing any code - this is documentation only
- Adding new features to generators
- Modifying existing documented behavior
- Creating runtime implementation guides

## Decisions

### Decision 1: Where to document `--use-pydantic`

**Choice:** Add to CLI reference and create new "Pydantic Models" section in Python reference

**Rationale:** The `--use-pydantic` flag is a CLI option, so it belongs in CLI reference. But explaining Pydantic model usage requires a dedicated section with examples, which fits best in Python reference alongside the existing "Validation" section.

### Decision 2: Multi-namespace documentation structure

**Choice:** Add dedicated "Multi-Namespace Projects" section to both Python and TypeScript quickstarts/references

**Rationale:** Multi-namespace output is a significant feature with different output structures. Grouping all multi-namespace info in one section (vs scattering across reference pages) makes it easier to find.

**Python structure:**
```
docs/languages/python/quickstart.md - Add multi-namespace example
docs/languages/python/reference.md - Add "Multi-Namespace Projects" section
```

**TypeScript structure:**
```
docs/languages/typescript/quickstart.md - Add multi-namespace note
docs/languages/typescript/reference.md - Add "Multi-Namespace Projects" section
```

### Decision 3: Runtime API documentation

**Choice:** Add "Runtime Reference" section to each language reference covering undocumented APIs

**Rationale:** `InProcTransport`, `validate_request/response`, and `notify()` are runtime APIs, not code generation. Grouping them together in a "Runtime" section keeps reference organized.

**Python runtime APIs to document:**
- `InProcTransport` class
- `Client(transport, validate_request=False, validate_response=False)`
- `client.notify(method, params)`
- Auto-discovery via `pulserpc-idl` RPC

**TypeScript runtime APIs to document:**
- `Client.ready()` async initialization
- `Contract` class with `validateRequest`/`validateResponse`
- IDL loading from `idl.json`

### Decision 4: Test file generation documentation

**Choice:** Add section in CLI reference under "Generate Test Files" example, expanded with details

**Rationale:** Test file generation is triggered by a CLI flag, so it belongs near other flag documentation. The current example is minimal - expand it to show what files are created and how to use them.

## Risks / Trade-offs

| Risk | Mitigation |
|------|------------|
| Documentation diverges from implementation | Keep implementation analysis notes in design.md for future reference |
| Multi-namespace docs become stale if output structure changes | Add warning box noting generator version dependency |
| Pydantic documentation assumes Python 3.8+ features | Note Pydantic v1 vs v2 compatibility in the section |

## Open Questions

1. **Pydantic version:** Should documentation cover Pydantic v1, v2, or both? Currently code generates Pydantic v1 style (`BaseModel` with `class Config`).

2. **Test file customization:** Should we document how to customize generated test files, or just that they exist?

3. **Cross-language consistency:** Ensure Python and TypeScript multi-namespace sections mirror each other's structure for predictability.

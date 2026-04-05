## Why

The Python and TypeScript code generators have undocumented features that prevent users from fully utilizing the generated code. Analysis of the generator implementations revealed several CLI flags, runtime APIs, and output structures that are not mentioned in any documentation, leading to discoverability issues and incomplete understanding of capabilities.

## What Changes

This change adds comprehensive documentation for undocumented features in the Python and TypeScript generators:

**Python Generator:**
- Document `--use-pydantic` flag for generating Pydantic validation models
- Document `idl.json` file generation and its role in runtime contract loading
- Document `InProcTransport` for in-process RPC (useful for testing)
- Document `Client.validate_request` and `Client.validate_response` flags
- Document `Client.notify()` for fire-and-forget RPC calls
- Document multi-namespace output structure and `__init__.py` generation
- Document test file generation (`test_server.py`, `test_client.py`)

**TypeScript Generator:**
- Document multi-namespace output structure (per-namespace directories)
- Document `index.ts` re-export files
- Document `idl.json` placement requirement (must be at root)
- Document cross-namespace import patterns
- Document `Client.ready()` method for async initialization
- Document test file generation (`test_server.ts`, `test_client.ts`)

**CLI Reference:**
- Add `--use-pydantic` flag documentation
- Clarify `-package` flag behavior for both Python and TypeScript

## Capabilities

### New Capabilities

- `python-pydantic-models`: Document the `--use-pydantic` flag that generates Pydantic `BaseModel` classes for struct and enum types, enabling runtime validation using Pydantic
- `python-multi-namespace`: Document Python multi-namespace output including directory structure, `__init__.py` generation, and import patterns
- `ts-multi-namespace`: Document TypeScript multi-namespace output including per-namespace directories, `index.ts` re-exports, and cross-namespace imports
- `python-runtime-discoverability`: Document Python runtime features: `InProcTransport`, `Client.validate_request/response`, `Client.notify()`, and auto-discovery via `pulserpc-idl`
- `ts-runtime-discoverability`: Document TypeScript runtime features: `Client.ready()` for async initialization and `Contract` class for IDL loading
- `idl-json-artifacts`: Document `idl.json` generation, purpose, and deployment requirements for both Python and TypeScript runtimes
- `test-file-generation`: Document `-generate-test-files` flag behavior for generating integration test scaffolding

### Modified Capabilities

<!-- No existing spec-level changes -->

## Impact

**Documentation files:**
- `docs/get-started/cli-reference.md` - Add `--use-pydantic` flag, clarify `-package` behavior
- `docs/languages/python/quickstart.md` - Add multi-namespace example
- `docs/languages/python/reference.md` - Add InProcTransport, validate flags, notify(), Pydantic models section
- `docs/languages/typescript/quickstart.md` - Add multi-namespace output details
- `docs/languages/typescript/reference.md` - Add Client.ready(), multi-namespace patterns

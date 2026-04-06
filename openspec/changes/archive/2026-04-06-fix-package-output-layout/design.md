## Context

The TypeScript and Python generators currently treat `-package` as a dot-separated path that creates nested directories (e.g., `-package com.example.generated` creates `com/example/generated/`). This is inconsistent with the intended semantics from `docs/MULTI_NAMESPACE_CODE_GEN_SPEC.md`.

The spec defines `-dir` as the base output directory and `-package` as the base module/package path for imports. For TypeScript, the package should create a single directory level under `-dir`, and for imports, it's only used as a module path prefix (though we're keeping relative imports).

Additionally:
- `idl.json` is currently written to `-dir` root, but should be per-namespace to support multiple `.pulse` files in one project
- Playground mode sets a default `-package com.example.generated` for all generators, but only Java needs this

## Goals / Non-Goals

**Goals:**
- Fix `-package` to create `{dir}/{package}/` directory structure (single level, not split by dots)
- Place `idl.json` inside the namespace subdirectory
- Remove inappropriate default `-package` in playground mode for non-Java generators
- Ensure Python and TypeScript generators have consistent behavior

**Non-Goals:**
- Change import string format (keeping relative imports)
- Modify Java generator (already has correct package handling)
- Modify Go or C# generators (may need similar fixes, but out of scope for this change)

## Decisions

### Decision 1: Package Directory Structure

**Choice:** `-package` creates a single directory level, not split by dots.

**Rationale:** 
- The `-dir` flag defines the project root for generated code
- The `-package` flag defines the package/namespace prefix for RPC-related code
- Splitting `com.example.generated` into nested directories forces users into deep nesting they may not want
- Simpler structure: `{dir}/{package}/{namespace}/`

**Example:**
```
-dir foo -package myapp example.pulse
│
foo/
└── myapp/                      <- package creates this
    ├── pulserpc/               <- runtime
    └── example/                <- namespace
        ├── idl.json
        ├── types.ts
        ├── server.ts
        └── client.ts
```

**Alternative Considered:** Split by dots for TypeScript (creating nested dirs).
- Rejected: Creates deep nesting, doesn't match common TS project layouts, inconsistent with Python

### Decision 2: `idl.json` Placement

**Choice:** `idl.json` goes inside the namespace directory.

**Rationale:**
- Each `.pulse` file produces its own `idl.json`
- Supports multiple `.pulse` files with different entry points in one project
- Avoids ambiguity about which IDL to load - look in the namespace directory for that entry point

**Example:**
```
foo/myapp/example/idl.json     <- for example.pulse
foo/myapp/common/idl.json     <- for common.pulse
```

**Alternative Considered:** Single `idl.json` at package root.
- Rejected: Would overwrite when running pulserpc multiple times; unclear which IDL to load for runtime

### Decision 3: Playground Defaults

**Choice:** Only Java gets a default `-package` value (`com.example.generated`).

**Rationale:**
- Java conventionally requires package names
- TypeScript, Python, C#, Go projects often don't use package prefixes
-Generated code should work without configuration in the simplest case

### Decision 4: Import Strings

**Choice:** Keep relative imports (`../pulserpc`, `../common`).

**Rationale:**
- Works without any `tsconfig.json` configuration
- Most portable approach
- `-package` affects directory structure only, not import strings

## Risks / Trade-offs

**Breaking Change:** Existing users with `-package com.foo.bar` will get `com.foo.bar/` instead of `com/foo/bar/`.
- **Mitigation:** Document the change; it's a clearer model that matches user expectations

**`idl.json` Visibility:** Runtime must know where to find `idl.json` for each entry point.
- **Mitigation:** Document that `idl.json` is per-namespace; runtime loads from the namespace directory

**Test Coverage:** Integration tests and quickstarts assume old structure.
- **Mitigation:** Update all tests and quickstarts to use new structure before merging
## Context

The Python runtime libraries (`pkg/runtime/runtimes/python/` for Python 3 and `pkg/runtime/runtimes/python2/` for Python 2.7) each contain a `types.py` file with helper functions like `find_struct`, `find_enum`, and `get_struct_fields`. Python 2.7 has a stdlib module also named `types` (containing type constructors like `IntType`, `StringType`, etc.). When code does `from types import ...` in Python 2.7, import path resolution can inconsistently select the stdlib `types` instead of `pulserpc.types` depending on import order and sys.path configuration.

Python 3 does not have this stdlib conflict (it uses `types` differently), but for consistency and to prevent future issues, the same rename should be applied.

The embedded runtime files are copied to generated project output directories when users run `pulserpc -plugin python-client-server`.

## Goals / Non-Goals

**Goals:**
- Rename `types.py` to `rpctypes.py` in all Python runtime source directories
- Update all internal imports to use `rpctypes` instead of `types`
- Ensure Python 2.7 quickstart test passes after changes
- Ensure Python 3 quickstart test passes after changes

**Non-Goals:**
- No spec-level behavior changes (pure refactoring)
- No API changes to public interfaces
- No changes to generated namespace files (`checkout/rpctypes.py` etc.) - those are namespace-specific, not runtime

## Decisions

### Rename to `rpctypes.py` (not `pulserpc_types`)

**Decision:** Use `rpctypes.py` as the new filename.

**Rationale:** The `rpc` prefix aligns with other modules in the package (`rpc.py`, `rpcerror.py` if it existed). `rpctypes` is distinct from stdlib `types` while being concise.

**Alternatives considered:**
- `pulserpc_types.py` - Too long, redundant with package name
- `type_helpers.py` - Doesn't match the package naming convention

### Update Go test assertions

**Decision:** Update `pkg/generator/python_client_server_test.go` line 697 and `pkg/generator/python_namespace_paths_test.go` references to use `rpctypes.py` instead of `types.py`.

**Rationale:** Tests verify that correct files are present in generated output. Since the embedded runtime now provides `rpctypes.py`, tests must check for the renamed file.

## Risks / Trade-offs

**[Risk]** Go tests might miss other assertions about `types.py`

→ **Mitigation:** Searched codebase for `types.py` references and updated all found assertions.

**[Risk]** Some user code might import from `pulserpc.types` directly

→ **Mitigation:** This is unlikely - the public API is through `pulserpc/__init__.py` which re-exports the helper functions. Users importing `pulserpc.types` directly is a non-standard usage.

## Migration Plan

1. Rename `types.py` → `rpctypes.py` in source runtime directories
2. Update all `from types import` statements to `from rpctypes import` (Python 2) or `from .rpctypes import` (Python 3 relative)
3. Update Go test assertions
4. Verify with `make test-runtime-python2`, `make test-quickstart-python2`, `make test-quickstart-python`
5. Rebuild CLI binary to embed updated runtime files

**Rollback:** Revert file renames and import changes to restore `types.py`.

## Open Questions

- (none)

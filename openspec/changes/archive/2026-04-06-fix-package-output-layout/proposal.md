## Why

The `-package` flag for TypeScript and Python generators doesn't match the intended semantics from the multi-namespace code generation spec. Currently, `-package com.example.generated` creates deep nested directories`com/example/generated/` which is confusing and inconsistent with how `-dir` and `-package` should interact.Additionally, `idl.json` is placed at the root output directory, but should be per-namespace to support multiple `.pulse` files in a single project. Finally, the playground (UI mode) inappropriately sets a default `-package` value for all generators, when only Java requires it.

## What Changes

- **TypeScript generator**: `-package` creates `{dir}/{package}/` directory structure with the package as a single directory level (not split by dots)
- **Python generator**: Same behavior as TypeScript for consistency
- **TypeScript/Python `idl.json` placement**: Move from root directory to inside the namespace subdirectory
- **UI mode defaults**: Remove default `-package com.example.generated` for TypeScript/Python/C#/Go; keep only for Java
- **Cross-namespace imports**: Update to use correct relative paths based on new structure
- **Import strings**: Keep relative imports (no breaking change to import format)

## Capabilities

### New Capabilities

- `package-output-layout`: Defines how `-dir` and `-package` flags control output directory structure and import paths across all generators

### Modified Capabilities

- `typescript-multi-namespace` (if exists): Update to reflect correct package handling
- `python-multi-namespace` (if exists): Update to reflect correct package handling

## Impact

**Affected Code:**
- `pkg/generator/ts_namespace_paths.go` - `ResolveRuntimeDir()`, `ResolveNamespaceDir()`
- `pkg/generator/ts_client_server.go` - file output paths, idl.json placement
- `pkg/generator/python_namespace_paths.go` - same functions for Python
- `pkg/generator/python_client_server.go` - file output paths, idl.json placement
- `pkg/playground/manager.go` - remove default package for non-Java generators
- `pkg/generator/ts_client_server_test.go` - update tests for new structure
- `pkg/generator/python_client_server_test.go` - update tests for new structure

**Breaking Changes:**
- Users who rely on the current nested directory structure will need to update their import paths
- Projects with existing generated code will need to regenerate

**Systems:**
- CI/CD pipelines that reference generated file paths
- Quickstart examples that demonstrate generated code
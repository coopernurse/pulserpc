## Why

The Python generator currently treats `-package` values with dots as a single directory name (e.g., `myapp.rpc` becomes `myapp.rpc/`), which is not idiomatic Python. Python packages should use dot-separated names that translate to nested directories, allowing imports like `import myapp.rpc.pulserpc`.

## What Changes

- **Modify Python generator** to split `-package` values by dots and create nested directory structure
- **Update unit tests** in `python_namespace_paths_test.go` to expect nested directories
- **Update Python quickstart integration test** to verify correct nested structure
- **Update Python quickstart documentation** to show correct package structure
- **Update `package-output-layout` spec** to reflect the corrected behavior

## Capabilities

### New Capabilities
- (none)

### Modified Capabilities
- `package-output-layout`: Change Python generator behavior to split `-package` by dots into nested directories (e.g., `myapp.rpc` → `myapp/rpc/`)

## Impact

- Python generator output directory structure
- Unit tests in `pkg/generator/python_namespace_paths_test.go`
- Integration test `tests/integration/test_quickstart_python.sh`
- Documentation in `docs/languages/python/quickstart.md`

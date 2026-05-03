## Why

The Python 2.7 runtime library contains a file named `types.py` which conflicts with Python's built-in `types` module. When code does `from types import ...` in Python 2.7, it can sometimes resolve to the stdlib `types` module instead of the PulseRPC `pulserpc.types` module depending on import order and path resolution, causing subtle bugs. The Python 3 runtime has the same file which should be renamed for consistency and to prevent future conflicts.

## What Changes

- Rename `pkg/runtime/runtimes/python2/pulserpc/types.py` to `rpctypes.py`
- Rename `pkg/runtime/runtimes/python/pulserpc/types.py` to `rpctypes.py`
- Update all imports referencing `types` to use `rpctypes` in both runtimes
- Rename `examples/quickstart/python/pulserpc/types.py` to `rpctypes.py` and update its imports
- Update Go test file assertions that check for `types.py` in runtime files
- Verify Python 2.7 quickstart test passes after changes
- Verify Python 3 quickstart test passes after changes

## Capabilities

### New Capabilities

- `python-runtime-types-rename`: Internal refactoring with no new capability contracts - this is a pure rename with no behavioral changes

### Modified Capabilities

- (none - this is a pure refactor with no spec-level behavior changes)

## Impact

**Affected code:**
- `pkg/runtime/runtimes/python2/pulserpc/` - Python 2.7 runtime source
- `pkg/runtime/runtimes/python/pulserpc/` - Python 3 runtime source
- `examples/quickstart/python/pulserpc/` - Example quickstart runtime copy
- `pkg/generator/python_client_server_test.go` - Test assertions
- `pkg/generator/python_namespace_paths_test.go` - Test assertions

**No spec-level changes** - This is a pure refactoring that renames a file and updates imports without changing behavior.

**Tests to verify:**
- `make test-runtime-python2` - Python 2.7 runtime tests
- `make test-quickstart-python2` - Python 2.7 quickstart integration test
- `make test-quickstart-python` - Python 3 quickstart integration test

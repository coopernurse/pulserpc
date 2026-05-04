## Why

The TypeScript runtime already supports passing a `ctx` (context) parameter to JSON-RPC method handlers, allowing transport-level metadata (headers, auth tokens, request IDs) to be injected without polluting the core request payload. The Python runtimes (both Python 2 and 3) lack this mechanism, creating an inconsistency across language runtimes and preventing Python handlers from accessing transport-level context.

## What Changes

- **BREAKING**: `Server.call(self, req)` → `Server.call(self, req, ctx=None)` in both Python2 and Python3 runtimes
- **BREAKING**: All handler method signatures must accept `ctx` as the last positional parameter (default `None`)
- `func(*params)` → `func(*params, ctx)` when invoking handlers in `Server.call()`
- Update Python generator to include `ctx=None` parameter in generated interface stubs
- Update Python generator to include `ctx=None` in generated test server implementations
- Update Python runtime tests to cover `ctx` passing
- Update Python generator tests to verify `ctx` appears in generated output
- Update quickstart examples (both code and comments) to demonstrate `ctx` usage

## Capabilities

### New Capabilities
- `python-ctx-support`: Pass transport-level context to Python JSON-RPC method handlers via `ctx` parameter

### Modified Capabilities

## Impact

- **Python runtime files**: `server.py` in both `python/` and `python2/` runtime directories
- **Python generator**: `python_client_server.go` - stub generation, test server generation
- **Python tests**: Runtime tests, generator tests
- **Quickstart examples**: `examples/quickstart/python/` - server implementations, documentation comments
- **Breaking change**: All existing Python handler implementations will need to add `ctx=None` parameter

## Why

PulseRPC needs to support a client running Python 2.7 who cannot migrate to Python 3. Python 2 reached EOL in January 2020, but legacy systems persist. Since the existing Python 3 runtime relies on Python 3-only features (f-strings, dataclasses, typing module, abc.ABC), we need a separate runtime and generator target for Python 2.

## What Changes

- **New Python 2 generator target**: Add `python2` as a target in the code generator, alongside existing `python` (Python 3)
- **New Python 2 runtime library**: Create `runtimes/python2/pulserpc/` with Python 2-compatible implementations
- **Minimal code generation for Py2**: Py2 target generates only `idl.json` - no code-generated types or stubs. Users write plain Python 2 against the runtime validation API
- **Py2 runtime reuses Barrister patterns**: The predecessor project Barrister was written in Python 2; its runtime.py provides a proven pattern for transport, server dispatch, and validation that ports directly to PulseRPC
- **Generator flag**: Add `--python-version=2.7` flag (default `3`) to select between Py2 and Py3 runtimes
- **No quickstart for Py2**: Skip the quickstart guide generation for Python 2

## Capabilities

### New Capabilities

- `python2-runtime`: A Python 2.7-compatible runtime library providing validation, server dispatch, and HTTP client transport. Reuses the validation logic from the Python 3 runtime, adapted to Python 2 syntax. Includes:
  - `validation.py`: Type validators for built-in types, arrays, structs, enums
  - `contract.py`: IDL parsing and request/response validation
  - `server.py`: JSON-RPC 2.0 request dispatch with handler registration
  - `client.py`: HTTP transport client with proxy objects
  - `transport.py`: HttpTransport (urllib2) and InProcTransport
  - `rpc.py`: RpcException and JSON-RPC error codes
  - `types.py`: Struct/enum lookup helpers

- `python2-generator`: Generator plugin changes to produce Py2-compatible output:
  - When `--python-version=2.7`: generates only `idl.json` and copies Py2 runtime
  - When `--python-version=3` (default): existing behavior with dataclasses and type stubs

- `python2-generator-tests`: Generator tests verifying Py2 output conforms to basic requirements (IDL generation, runtime file copy, validation correctness)

- `python2-runtime-tests`: Python 2 runtime tests validating input/output validation across all type scenarios (structs, enums, arrays, optionals, nested types)

## Impact

- **New directory**: `pkg/runtime/runtimes/python2/pulserpc/` - Python 2 runtime library
- **Modified file**: `pkg/generator/python_client_server.go` - add Python version flag and conditional generation
- **Modified file**: `pkg/runtime/embed.go` - embed Python 2 runtime files
- **Generator tests**: New tests in `pkg/generator/python_client_server_test.go` for Py2 target
- **Runtime tests**: New Python 2 tests in `runtimes/python2/pulserpc/test_validation.py`

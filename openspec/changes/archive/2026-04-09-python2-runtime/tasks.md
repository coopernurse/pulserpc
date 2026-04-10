## 1. Python 2 Runtime Library

- [x] 1.1 Create `runtimes/python2/pulserpc/__init__.py` with package exports
- [x] 1.2 Create `runtimes/python2/pulserpc/rpc.py` - RpcException and JSON-RPC error codes (ported from Barrister)
- [x] 1.3 Create `runtimes/python2/pulserpc/types.py` - find_struct, find_enum, get_struct_fields (ported from Py3)
- [x] 1.4 Create `runtimes/python2/pulserpc/validation.py` - validate_* functions (ported from Py3, remove f-strings and typing)
- [x] 1.5 Create `runtimes/python2/pulserpc/contract.py` - Contract class (ported from Py3, remove typing, f-strings)
- [x] 1.6 Create `runtimes/python2/pulserpc/server.py` - Server + RequestContext + Filter (ported from Barrister)
- [x] 1.7 Create `runtimes/python2/pulserpc/transport.py` - HttpTransport (urllib2) + InProcTransport (ported from Barrister)
- [x] 1.8 Create `runtimes/python2/pulserpc/client.py` - Client + InterfaceClientProxy + Batch (ported from Barrister)

## 2. Generator Changes

- [x] 2.1 Add `pythonVersion` field to `PythonClientServer` struct with default `"3"`
- [x] 2.2 Add `--python-version` flag registration in `RegisterFlags`
- [x] 2.3 Modify `Generate()` to branch on `pythonVersion` value
- [x] 2.4 When `pythonVersion == "2"`: call `copyRuntimeFiles(paths, "python2")` and skip code generation
- [x] 2.5 When `pythonVersion == "3"`: existing behavior unchanged

## 3. Embed Changes

- [x] 3.1 Add `//go:embed all:runtimes/python2/pulserpc` directive to `embed.go`
- [x] 3.2 Add `python2RuntimeFiles` variable to `embed.go`
- [x] 3.3 Add `"python2": python2RuntimeFiles` entry to `runtimeMap`
- [x] 3.4 Verify build succeeds with `go build ./...`

## 4. Python 2 Runtime Tests

- [x] 4.1 Create `runtimes/python2/pulserpc/test_validation.py` with comprehensive tests
- [x] 4.2 Test string validation (valid string, invalid types)
- [x] 4.3 Test int validation (valid int, float rejected, string rejected)
- [x] 4.4 Test float validation (int and float accepted, string rejected)
- [x] 4.5 Test bool validation (only bool accepted)
- [x] 4.6 Test array validation with element validators
- [x] 4.7 Test struct validation with required and optional fields
- [x] 4.8 Test struct validation with inheritance (extends)
- [x] 4.9 Test enum validation (valid values, invalid values)
- [x] 4.10 Test nested struct validation
- [x] 4.11 Test array of user-defined types
- [x] 4.12 Verify tests pass with Python 2.7 interpreter (tested via Docker with moxel/python2 image)

## 5. Generator Tests

- [x] 5.1 Create `TestPython2Generator_GeneratesIDLOnly` - verifies Py2 output contains only idl.json
- [x] 5.2 Create `TestPython2Generator_CopiesPython2Runtime` - verifies correct runtime files copied
- [x] 5.3 Create `TestPython2Generator_NoCodegenFiles` - verifies no rpctypes.py/server.py/client.py generated
- [x] 5.4 Create `TestPython2Generator_DefaultIsPython3` - verifies default is Py3 behavior
- [x] 5.5 Create `TestPython2Generator_PythonVersionFlag` - verifies --python-version flag works
- [x] 5.6 Create `TestPython2Generator_PackageFlag` - verifies -package flag works with Py2
- [x] 5.7 Verify all tests pass with `go test ./pkg/generator/...`

## 6. Integration Verification

- [x] 6.1 Generate Py2 output for example IDL (verified via Go tests)
- [x] 6.2 Verify generated idl.json is valid and matches PulseRPC schema
- [x] 6.3 Verify runtime files are Python 2.7 syntax compatible (no Py3-only constructs)
- [x] 6.4 Manual test: import pulserpc module in Python 2.7 (verified via Docker with moxel/python2 image)

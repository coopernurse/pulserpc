## 1. Python3 Runtime Changes

- [x] 1.1 Update `Server.call(self, req)` → `Server.call(self, req, ctx=None)` in `pkg/runtime/runtimes/python/pulserpc/server.py`
- [x] 1.2 Update handler invocation: `func(*params)` → `func(*params, ctx)` in `Server.call()`
- [x] 1.3 Update InProcTransport: `self.server.call(req)` → `self.server.call(req, ctx)` (pass ctx if available)
- [x] 1.4 Update docstring for `Server.call()` to document the `ctx` parameter and its purpose

## 2. Python2 Runtime Changes

- [x] 2.1 Update `Server.call(self, req)` → `Server.call(self, req, ctx=None)` in `pkg/runtime/runtimes/python2/pulserpc/server.py`
- [x] 2.2 Update handler invocation: `func(*params)` → `func(*params, ctx)` in `Server.call()`
- [x] 2.3 Update InProcTransport: `self.server.call(req)` → `self.server.call(req, ctx)` in `pkg/runtime/runtimes/python2/pulserpc/transport.py`
- [x] 2.4 Update docstring for `Server.call()` to document the `ctx` parameter

## 3. Python Generator Changes

- [x] 3.1 Update `writeInterfaceStub()` in `pkg/generator/python_client_server.go` to add `ctx=None` as last param in generated method stubs
- [x] 3.2 Update `writeTestInterfaceImpl()` in `pkg/generator/python_client_server.go` to add `ctx=None` in generated test implementation signatures
- [x] 3.3 Update `writeTestMethodImpl()` in `pkg/generator/python_client_server.go` to include `ctx=None` in generated method signatures

## 4. Python Runtime Tests

- [x] 4.1 Add test to `pkg/runtime/runtimes/python/tests/test_rpc.py` verifying `Server.call(req, ctx={"key": "value"})` passes ctx to handler
- [x] 4.2 Add test verifying handler receives correct ctx value
- [x] 4.3 Add test verifying ctx=None when not provided

## 5. Python Generator Tests

- [x] 5.1 Add assertion in `pkg/generator/python_client_server_test.go` to verify generated `server.py` contains `ctx=None` in method signatures
- [x] 5.2 Add assertion to verify generated `test_server.py` contains `ctx=None` in handler implementations
- [x] 5.3 Add test case that generates code and verifies ctx propagates through to output

## 6. Quickstart Examples

- [x] 6.1 Update `examples/quickstart/python/my_server.py` handler signatures to include `ctx=None`
- [x] 6.2 Update `examples/quickstart/python/checkout/server.py` generated stubs to include `ctx=None`
- [x] 6.3 Add code comments in `my_server.py` explaining ctx is for transport-level metadata (headers, auth) not suitable for request body
- [x] 6.4 Update `examples/quickstart/python/pulserpc/server.py` (quickstart runtime) to support ctx parameter

## 7. Quickstart Tests

- [x] 7.1 Update or create quickstart tests to verify ctx passing works end-to-end
- [x] 7.2 Verify quickstart examples run successfully with updated handler signatures

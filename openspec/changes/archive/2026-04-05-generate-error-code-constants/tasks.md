## 1. Add error constant generation to Python code generator

- [x] 1.1 Modify `generateTypesPyForNamespace()` in `pkg/generator/python_client_server.go` to generate `ErrJsonRpc` class with standard JSON-RPC error codes (-32700, -32600, -32601, -32602, -32603)
- [x] 1.2 Add generation of `Err` class containing namespace error constants from `idl.Errors`
- [x] 1.3 Verify generated `types.py` includes error classes when errors are present in IDL

## 2. Update checkout.pulse with errors block

- [x] 2.1 Add `errors {}` block to `examples/quickstart/checkout.pulse` with CartNotFound (1001), CartEmpty (1002), PaymentFailed (1003), OutOfStock (1004), InvalidAddress (1005)
- [x] 2.2 Remove the error code comment block (lines 80-85) from `checkout.pulse`

## 3. Update my_server.py to use error constants

- [x] 3.1 Import `Err` and `ErrJsonRpc` from `checkout` package in `my_server.py`
- [x] 3.2 Replace `RPCError(1001, ...)` with `RPCError(Err.CartNotFound, ...)`
- [x] 3.3 Replace `RPCError(1002, ...)` with `RPCError(Err.CartEmpty, ...)`
- [x] 3.4 Replace `RPCError(1004, ...)` with `RPCError(Err.OutOfStock, ...)`
- [x] 3.5 Replace `RPCError(1005, ...)` with `RPCError(Err.InvalidAddress, ...)`
- [x] 3.6 Replace `RPCError(-32602, ...)` with `RPCError(ErrJsonRpc.InvalidParams, ...)`

## 4. Regenerate quickstart code and verify

- [x] 4.1 Regenerate Python code for the quickstart using `pulse generate` command
- [x] 4.2 Run `make quality` to verify no linting issues
- [x] 4.3 Run `make test-quickstarts` to verify quickstart works correctly

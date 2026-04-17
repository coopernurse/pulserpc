## 1. Python2 Runtime Simplification

- [x] 1.1 Change `func(**params)` to `func(*params)` in `server.py:97`
- [x] 1.2 Remove `_positional_to_named_params` method
- [x] 1.3 Remove `_named_to_positional_params` method
- [x] 1.4 Simplify `call()` method to pass positional params directly
- [x] 1.5 Remove validation for named params dict (lines 88-94)
- [x] 1.6 Run Python2 runtime tests to verify behavior unchanged

## 2. Python3 Runtime Simplification

- [x] 2.1 Change `func(**params)` to `func(*params)` in `server.py:140`
- [x] 2.2 Remove `_positional_to_named_params` method
- [x] 2.3 Remove `_named_to_positional_params` method
- [x] 2.4 Simplify `call()` method to pass positional params directly
- [x] 2.5 Remove validation for named params dict (lines 148-158)
- [x] 2.6 Run Python3 runtime tests to verify behavior unchanged

## 3. Verification

- [x] 3.1 Run full test suite for Python2 runtime
- [x] 3.2 Run full test suite for Python3 runtime
- [x] 3.3 Verify JSON-RPC request/response format unchanged

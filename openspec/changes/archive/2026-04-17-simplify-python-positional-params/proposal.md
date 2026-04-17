## Why

The Python2 and Python3 runtimes in PulseRPC convert positional JSON-RPC parameters to named parameters, then invoke handlers with `func(**params)`. This indirection is unnecessary since all PulseRPC methods use positional parameters (codegen enforces this), and all other runtimes (Go, Java, TypeScript) invoke handler methods positionally via reflection or spread operators.

## What Changes

- **Simplify invocation in Python2 runtime**: Change `func(**params)` to `func(*params)`
- **Simplify invocation in Python3 runtime**: Change `func(**params)` to `func(*params)`
- **Remove positional-to-named conversion**: Methods `_positional_to_named_params` and `_named_to_positional_params` become unnecessary
- **Simplify validation path**: Validate positional params directly without conversion

## Capabilities

### New Capabilities
None - this is a refactoring/simplification with no new capabilities.

### Modified Capabilities
None - existing method invocation behavior remains the same; parameters are still passed correctly, just without the unnecessary named-parameter layer.

## Impact

- **Python2 runtime** (`pkg/runtime/runtimes/python2/pulserpc/server.py`): Remove `_positional_to_named_params`, `_named_to_positional_params`, simplify `call()` method
- **Python3 runtime** (`pkg/runtime/runtimes/python/pulserpc/server.py`): Remove `_positional_to_named_params`, `_named_to_positional_params`, simplify `call()` method
- **No behavioral change**: Handler functions receive the same positional arguments in the same order
- **No API change**: JSON-RPC request/response format unchanged
- **No breaking change**: Public interfaces unchanged
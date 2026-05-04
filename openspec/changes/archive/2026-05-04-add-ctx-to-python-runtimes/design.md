## Context

The TypeScript runtime (`pkg/runtime/runtimes/ts/pulserpc/server.ts`) already implements `ctx` passing: `Server.call(req, ctx?)` passes `ctx` as the last argument to all handler methods. This allows transport-level metadata (auth tokens, request IDs, tracing headers) to reach handlers without polluting the JSON-RPC params.

The Python runtimes (`python/` and `python2/`) currently don't support this:
- `Server.call(self, req)` takes only the request
- `func(*params)` invokes handlers without `ctx`
- Generated interface stubs and test implementations lack `ctx` in signatures

The change spans 6 areas: python2 runtime, python3 runtime, python runtime tests, python generator tests, python quickstart docs, and python quickstart tests.

## Goals / Non-Goals

**Goals:**
- Add `ctx` parameter support to Python2 and Python3 `Server.call()` methods
- Pass `ctx` as the last positional argument to all JSON-RPC method handlers
- Update generated interface stubs to include `ctx=None` parameter
- Update all tests and quickstart examples to reflect the new signature
- Document that `ctx` is for transport-level metadata (headers, auth) not suitable for the request body

**Non-Goals:**
- Adding `ctx` support to the TypeScript client (it's server-side only)
- Modifying the wire protocol (JSON-RPC remains unchanged)
- Adding specific transport-level features (auth, tracing) — `ctx` is an open mechanism

## Decisions

**1. `ctx` default value: `None`**

Decision: Use `ctx=None` as the default in all handler signatures.

Rationale: Breaking change is acceptable (user confirmed), but `None` default keeps the function signature flexible. TypeScript always passes `ctx` (even if `undefined`), but Python convention favors explicit defaults for readability and gradual migration.

Alternatives considered:
- No default (required): Would be consistent with TS but less Pythonic
- Context object always created: Unnecessary overhead when not needed

**2. `Server.call(req, ctx=None)` signature**

Decision: Add `ctx` as an optional second parameter to `Server.call()`.

Rationale: Mirrors TypeScript's `call(req, ctx?)`. The `ctx` value flows directly to handler methods. Transport implementations (HttpTransport, InProcTransport) can optionally pass `ctx` when calling `server.call()`.

Alternatives considered:
- Include `ctx` in the JSON-RPC request dict: Pollutes the request, requires serialization
- Use thread-local storage: Over-engineered for this use case

**3. Handler invocation: `func(*params, ctx)`**

Decision: Always pass `ctx` as the last positional argument to handlers.

Rationale: Consistent with TypeScript behavior. Since `ctx` defaults to `None`, handlers that haven't been updated will break (breaking change, as agreed).

Implementation note: Change `func(*params)` to `func(*params, ctx)` in both runtimes.

**4. Generated stubs: `ctx=None` in method signatures**

Decision: Generated interface stubs will include `ctx=None` as the last parameter.

For Python3 (`server.py`):
```python
@abc.abstractmethod
def method_name(self, param1, param2, ctx=None):
    pass
```

For Python2: Same pattern (without type hints).

Alternatives considered:
- Omit from stubs, document separately: Would cause confusion when implementing
- Make it required: Breaks the gradual adoption pattern

## Risks / Trade-offs

- **[Risk] Breaking change for existing Python handlers** → Handlers need to add `ctx=None` parameter. Migration: add `ctx=None` to all method signatures.
- **[Risk] Python2 syntax limitations** → Python2 doesn't have `unicode` type issues with `ctx=None`. No special handling needed.
- **[Trade-off] Always passing `ctx` even when not used** → Minimal overhead; clarity and consistency outweigh the cost.

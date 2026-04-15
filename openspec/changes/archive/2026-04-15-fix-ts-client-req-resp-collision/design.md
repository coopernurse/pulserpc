## Context

The TypeScript code generator's `writeClientMethodTs()` function in `pkg/generator/ts_client_server.go` hardcodes variable names `req` and `resp` for the internal request object and response variable. These names can collide with method parameter names when an IDL defines parameters named `req` or `resp`.

## Goals / Non-Goals

**Goals:**
- Prevent name collisions in generated TypeScript client code when IDL uses `req` or `resp` as parameter names

**Non-Goals:**
- No changes to the runtime TypeScript library (client.ts is generated, not part of runtime)
- No changes to server-side code generation

## Decisions

### Decision: Use `_req` and `_resp` as prefixed internal variable names

**Choice**: Prefix internal variables with underscore (`_req`, `_resp`) rather than using fully different names.

**Rationale**:
- Minimal change: only adds an underscore prefix
- Underscore prefix is a TypeScript convention for "internal" variables that aren't part of the public API
- No collision with any valid TypeScript identifier since `-` is not allowed in identifiers
- Alternative options considered:
  - `requestObj`/`responseObj`: More verbose, less idiomatic
  - `rpcReq`/`rpcResp`: Another reasonable option, but `_req`/`_resp` is simpler

## Risks / Trade-offs

- **Regeneration required**: Existing generated clients with `req`/`resp` parameters will break and need regeneration. This is unavoidable but necessary to fix the bug.

## Why

The TypeScript code generator in `pkg/generator/ts_client_server.go` hardcodes `const req` and `const resp` as internal variable names when generating client methods. When an IDL defines methods with parameters named `req` or `resp`, the generated code produces name collisions and invalid TypeScript.

## What Changes

- Change generated internal variable names from `req`/`resp` to `_req`/`_resp` in `writeClientMethodTs()` function
- **BREAKING**: Existing generated clients with methods using `req` or `resp` as parameter names will need to be regenerated

## Capabilities

### New Capabilities

<!-- None - pure bug fix -->

### Modified Capabilities

<!-- None - no spec-level behavior changes -->

## Impact

- **Affected Code**: `pkg/generator/ts_client_server.go:1032,1047-1058`
- **Generated Output**: All TypeScript client files (client.ts) with methods whose parameters are named `req` or `resp`
- **Breaking**: Generated client code that previously worked will now need regeneration if it had `req`/`resp` as method parameters

## Why

Multi-line IDL comments are not rendered correctly in TypeScript output. When a field or enum value has a multi-line comment in the IDL, only the first line gets the `//` prefix - subsequent lines lose the comment marker, producing invalid TypeScript syntax.

## What Changes

- Fix TypeScript generator to properly split multi-line comments before emitting `//` prefix (4 locations)
- Add test coverage for multi-line comment rendering in TypeScript generator
- This is a bug fix with no behavioral changes to the API or IDL schema

## Capabilities

This is a bug fix - no new capabilities are introduced and no existing capabilities are modified.

## Impact

- **Affected Code**: `pkg/generator/ts_client_server.go` (4 locations: lines 701, 742, 803, 840)
- **Affected Languages**: TypeScript only (Go, C#, Python generators already handle this correctly)
- **Breaking Changes**: None
- **Test Coverage**: No existing tests for multi-line comment rendering in TS generator

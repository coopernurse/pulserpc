## Why

Generated Go structs currently add `omitempty` only to optional fields. A
downstream service that wants every zero-valued field omitted from serialized
requests and responses, including required scalar and string fields, has to
edit generated code by hand or wrap every struct with a custom marshaler.

## What Changes

- Add a Go generator flag `-omit-empty` (default `false`).
- When `true`, every generated struct field gets `,omitempty` in its JSON tag;
  optional fields already have it.
- Keep the current tags when the flag is unset: optional fields use
  `omitempty`, required fields do not.
- Document that this is a compile-time generator option, not a runtime
  `ServerConfig` field, because struct tags are part of the generated Go source.
- Add generator tests for both tag modes and document the zero-value caveat for
  required fields.

## Capabilities

### New Capabilities

- `go-omit-empty`: Go struct generation can opt into `omitempty` on every field.

### Modified Capabilities

- None.

## Impact

**Affected code:**
- `pkg/generator/go_client_server.go` — flag registration, flag plumbing, tag
  emission.
- `pkg/generator/go_*_test.go` — default and enabled tag coverage.

**Documentation:**
- `docs/languages/go/reference.md`
- `openspec/changes/add-go-omit-empty/`

**Compatibility:**
- Default output is unchanged.
- `-omit-empty=true` changes generated JSON tags and is a source-level change
  only for regenerated code. It can omit legitimate zero values from requests;
  callers that rely on explicit zero values must leave it disabled.

# Doc vs Implementation Audit: Go runtime

## Summary

Audited the Go runtime documentation against the implementation in
`pkg/runtime/runtimes/go/pulserpc/` and the generator in
`pkg/generator/go_client_server.go`. Most limits, transport behavior,
notification handling, batch handling, and contract-verification severity
tables match the code. The deltas below are concentrated in type mapping
(pointer vs. value semantics), the documented validation-error `data` shape,
the documented `int` width in the cross-language types guide, the
`VerifyOnBootstrap` client option, and the list of shared runtime files
created by generation.

## Deltas

### go-doc-01 — Validation error `data` is structured, not a plain string
- Severity: medium
- Doc claim: `docs/idl-guide/validation.md:240-247` shows `data` as a plain
  string and states "`data` is a plain string in the Go, Java and C#
  runtimes".
- Code reality: Go emits a structured object.
  `pkg/runtime/runtimes/go/pulserpc/contract.go:253-255` returns
  `NewRPCErrorWithData(-32602, ..., vr.ToMap())`;
  `pkg/runtime/runtimes/go/pulserpc/validation_result.go:47-55` serializes
  `{"valid":..., "error":..., "invalid_fields":[...]}`.
- Recommendation: change doc
- Rationale: The Go runtime converged to structured validation data (M13);
  the guide still describes the old string shape.

### go-doc-02 — `int` is documented as 64-bit in the cross-language types guide
- Severity: medium
- Doc claim: `docs/idl-guide/types.md:15` says `int` is "64-bit integers"
  and maps Go to `int64`.
- Code reality: generated Go uses `int`
  (`pkg/generator/go_client_server.go:508`), and the runtime enforces signed
  32-bit (`pkg/runtime/runtimes/go/pulserpc/validation.go:17-47`). The
  Go-specific reference already documents the 32-bit rule.
- Recommendation: change doc
- Rationale: `types.md` is out of sync with both the Go generator and the
  Go reference; update the Go column to `int` (signed 32-bit on the wire).

### go-doc-03 — Struct type mapping implies pointer types for non-optional structs
- Severity: medium
- Doc claim: `docs/languages/go/reference.md:22` maps `Struct -> *Struct`, and
  examples use pointer slices such as
  `docs/languages/go/reference.md:53` (`Items: []*checkout.CartItem{}`) and
  `docs/languages/go/reference.md:270`.
- Code reality: non-optional struct fields and array elements are value types;
  only optional structs become pointers.
  `pkg/generator/go_client_server.go:490-495` returns `typeName` unless
  `optional`, in which case it returns `*typeName`. Generated output confirms
  `Items []Bar` and `Req Bar` for required fields.
- Recommendation: change doc
- Rationale: The quickstart already uses value slices
  (`Items: []checkout.CartItem{}` and dereferences `Cart: *cart`), so the
  reference should be aligned with generated code.

### go-doc-04 — `VerifyOnBootstrap()` is documented but not generated
- Severity: high
- Doc claim: `docs/languages/go/quickstart.md:170-173` shows
  `checkout.VerifyOnBootstrap()` as a client option.
- Code reality: the generated client declares a `verifyOnBootstrap` field
  (`pkg/generator/go_client_server.go:1317`) but `writeClientOptions`
  (`pkg/generator/go_client_server.go:1140-1174`) defines only
  `WithAuditor`, not `VerifyOnBootstrap`; no option or setter is emitted.
  The runtime client (`pkg/runtime/runtimes/go/pulserpc/client.go:319-323`)
  does implement it, but generated interface clients do not.
- Recommendation: change code
- Rationale: The openspec contract-verification spec requires this option, and
  the documented example will not compile against generated code. Either
  implement the option in the generator or remove the documented example.

### go-doc-05 — Shared runtime file list is incomplete
- Severity: low
- Doc claim: `docs/languages/go/quickstart.md:68` and `:220-223` say
  generation creates only `rpc.go`, `types.go`, and `validation.go`.
- Code reality: `copyRuntimeFiles`
  (`pkg/generator/go_client_server.go:401-414`) copies every `.go` file from
  `pkg/runtime/runtimes/go/pulserpc/`, including `auditor.go`,
  `contract.go`, `diff.go`, `server.go`, `transport.go`, `client.go`, etc.
- Recommendation: change doc
- Rationale: The generated project contains the full runtime package, not just
  three files.

## Docs that are accurate

- `docs/languages/go/reference.md` int 32-bit wire rule and validation
  rejection list.
- Request/response body, batch, timeout, and transport limits table in
  `docs/languages/go/reference.md` (1 MiB request, 10 MiB response, 100
  batch entries, 30 s client timeout).
- Non-POST 405, handler panic `-32603` with no `data`, boolean/object/array
  id rejection, notification handling, and notification-only batch 204
  behavior.
- `NewHTTPTransport` trailing-slash removal and custom headers.
- IDL JSON top-level keys and embedded `IDL_JSON`.
- Byte-deterministic generation statement.
- Contract verification severity table and auditor behavior in
  `docs/languages/go/quickstart.md`.
- Server concurrency guidance and optional-field `omitempty` behavior.

## Recommended doc edits (concrete)

1. `docs/idl-guide/validation.md:245-247`: change the Go/Java/C# sentence so
   Go reports structured data (`valid`, `error`, `invalid_fields`), or split
   Go out of the plain-string sentence.
2. `docs/idl-guide/types.md:15`: change the Go column from `int64` to `int`
   and describe `int` as signed 32-bit on the wire.
3. `docs/languages/go/reference.md:22`: change `Struct -> *Struct` to note
   that required structs are value types and only optional structs are
   pointers; update the `Items: []*checkout.CartItem{}` examples at `:53`
   and `:270` to `[]checkout.CartItem{}`.
4. `docs/languages/go/quickstart.md:68,220-223`: replace the three-file list
   with "all files in `pkg/pulserpc/`" (or enumerate the runtime package).
5. `docs/languages/go/quickstart.md:170-173`: remove or guard the
   `VerifyOnBootstrap()` example until the generator implements it.

## Cases where code may be wrong (disagreements with user's bias)

- Generated Go clients do not implement `VerifyOnBootstrap()` despite the
  openspec contract-verification spec and quickstart documenting it
  (`pkg/generator/go_client_server.go:1140-1174`,
  `pkg/generator/go_client_server.go:1317`). Recommend implementing the option
  rather than only editing docs.
- `Contract.FindStruct`/`FindEnum` doc comments say an ambiguous base name
  resolves to `nil`, while `typeindex.go` deterministically returns the first
  sorted key. This is an internal comment/code mismatch; user-facing
  behavior follows the guide (first sorted key).
}
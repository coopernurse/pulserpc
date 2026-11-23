# Wave 6 — Go identifier audit

Scope: find OTHER instances of the bug class just fixed — qualified IDL names
(containing `.`, e.g. `alpha.Alpha` for an interface declared in an imported
`.pulse` file) being interpolated into Go identifiers, plus adjacent name/copying
hazards.

Already fixed before this audit (do NOT re-report): `goInterfaceIdent()` uses
`GetBaseName`; `copyRuntimeFiles` filters `*_test.go`. JSON-RPC method strings
must KEEP the qualified name (`alpha.Alpha.Ping`).

## Findings table

| # | Candidate | Evidence file:line | Status | Severity | Recommended fix | Resolution |
|---|-----------|--------------------|--------|----------|-----------------|------------|
| 1 | Two imported namespaces declare interface `Alpha`; base-name collapse emits duplicate `type Alpha`/`AlphaClient`/ctors in one package | `pkg/generator/go_client_server.go:883,1345,1437,1717,1743,1755,1943-1965` (`goInterfaceIdent` → `GetBaseName`) | CONFIRMED | High | Disambiguate colliding base names with a deterministic namespace prefix; keep method strings qualified. | FIXED in `0bbee13` (`goInterfaceIdent` now namespace-prefixes only colliding interfaces) |
| 2 | Namespace name is a Go keyword (`namespace type`): emitted verbatim as `package type` and as an import qualifier | `pkg/generator/go_client_server.go:699,245-246,547-549`; namespace grammar `pkg/parser/parser.go:81` (`@Ident`, accepts keywords) | CONFIRMED | Medium | Reject Go-reserved namespace names in `Generate` (dots/hyphens impossible via `@Ident`). | FIXED in `0bbee13` (clear `reserved word` error) |
| 3 | Method parameter named `type` (etc.): raw `param.Name` emitted as a Go identifier | `pkg/generator/go_client_server.go:895,1445,1477,1763`; validator regex `pkg/parser/validator.go:19` accepts keywords | CONFIRMED | Medium | Sanitize Go parameter identifiers (append `_` on keyword). | FIXED in `0bbee13` (`goParamIdent`) |
| 4 | Upstream `*_test.go` shipped by an older generation survives the new filter: the idempotency check skips copying when all current files exist | `pkg/generator/go_client_server.go:405-414,424-435` | CONFIRMED | Medium | In the early-return path, delete the known upstream test filenames from the runtime dir before returning. | FIXED in `0bbee13` (evicts only known upstream test filenames) |
| 5 | Two namespaces declare struct `User`; with `-generate-test-files` the test server/client dot-import both and emit bare `User` → redeclaration + unused import | `pkg/generator/go_client_server.go:1695-1703,1762,2120,1884-1886` | CONFIRMED | Medium | Qualify test-data type names with the namespace instead of dot-importing ambiguous namespaces (larger generated-test refactor). | NOT FIXED — recommended; needs coordinated change to generated test data and imports (see reason below) |
| 6 | Enum values `foo_bar` and `fooBar` pass IDL uniqueness but collapse to the same Go const `ColorFooBar` | `pkg/generator/go_client_server.go:730-738`; validator `pkg/parser/validator.go:99-111` checks raw names only | CONFIRMED | Low | Disambiguate generated const names deterministically (`_2`, `_3`, …). | FIXED in `0bbee13` |

No UNVERIFIED findings remain. Item 5 is CONFIRMED but deliberately not fixed:
the generated test server and client rely on dot-imports plus bare type names,
so making ambiguous base names work requires qualifying every generated test
literal (`alpha.User`, `beta.User`) and dropping conflicting dot-imports — a
cross-cutting change to `generateTestServerGo`, `generateTestClientGo`,
`writeTestMethodImplGo` and `generateTestParamValueGo` that is not a small,
safe patch. It does not affect normal (non-`-generate-test-files`) generation.

## Repro evidence (before fixes)

Built with temporary `.pulse` files + `parser.ParseIDL` + `GoClientServer.Generate`
+ `go build ./...` / `go test ./...` in a fresh generated module. Exact output:

1. `alpha.Alpha` + `beta.Alpha` → `alpha\client.go:577:6: AlphaClient redeclared in this block`, `alpha\server.go:29:6: Alpha redeclared in this block`.
2. `namespace type` → `type\client.go:6:9: expected 'IDENT', found 'type'`.
3. `Ping(type string)` → `kw\client.go:338:26: syntax error: unexpected keyword type, expected )`.
4. After regenerating over an output dir containing upstream `severity_spec_test.go`: file still present; `go test ./...` → `severity_spec_test.go:18: read spec: open ...\openspec\specs\contract-compatibility-verification\spec.md: The system cannot find the path specified.`
5. `alpha.User` + `beta.User` with `-generate-test-files` → `cmd\test_server\main.go:13:2: User redeclared in this block`, `".../beta" imported and not used`.
6. Enum values `foo_bar` + `fooBar` → `en\en.go:12:2: ColorFooBar redeclared in this block`.

Permanent regression tests for items 1–4 and 6 live in
`pkg/generator/go_identifiers_wave6_test.go`.

## Ruled out / by design

- Qualified type names in field/param/return types already use `GetBaseName` plus a namespace qualifier (`getGoTypeWithNamespace`, `getGoStructOrEnumTypeName`) — no raw dotted Go identifier survives.
- Struct/enum/field/method names go through `GetBaseName` / `snakeToCamelCase`; no `.` can remain.
- Namespace values cannot contain dots or hyphens (lexer `Ident` rule); uppercase is legal Go and needs no change.
- Collapsing duplicate base names in `findNamespaceForType` is asserted deterministic by `go_invariants_test.go:49-88` (sorted-first); it is not a compile error in normal generation.
- Embedded Go runtime has only top-level `.go` files (no nested directories, no non-Go files); once upstream tests are omitted, a fresh-module `go build ./...` and `go test ./...` pass.

## Verification

`go test ./pkg/generator -count=1` → PASS (15.2s) after fixes, plus the five
regression tests in `pkg/generator/go_identifiers_wave6_test.go` (each builds
or tests the generated module) and the pre-fix repro builds above.

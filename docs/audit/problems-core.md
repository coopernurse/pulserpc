# Core (parser / generator / docs) — audit findings

## Summary
- Invariants checked: 17 catalog invariants (G11, G13, G15, G17, G18, G20, G21, G22, G23, G25, M5, M6, M7, M13, M15, M16, M17) plus 2 newly discovered shared-layer defects
- Confirmed violations: 16 (includes test-only and doc/spec defects; 1 catalog item downgraded, see G21)
- Test gaps: 9 (non-asserting cycle/duplicate/negative-code tests, vacuous checksum regression test, no generator self-diff test, no adapter compliance tests)
- Go toolchain is not installed in this environment, so empirical CLI reproduction was not possible; all findings below are verified by direct source reading with exact file:line.

## Problems

### core-01 — Negative JSON-RPC error codes are documented but impossible to parse
- Severity: high
- Invariant: G11
- Current behavior: `IntLiteral` is `[0-9]+` only (`pkg/parser/parser.go:20`), and `ErrorDecl.Code` is `parser:"@IntLiteral"` (`pkg/parser/parser.go:155`). There is no sign token, so `-32700 ParseError "..."` is a parse error.
- Expected behavior: Docs advertise negative codes: "Error codes can be any integer. JSON-RPC reserves certain ranges: -32700 to -32000..." (`docs/idl-guide/syntax.md:63-66`) and list the standard negative codes (`docs/idl-guide/errors.md:111-121`).
- Test status: `TestJSONRPCReservedErrorCodes` (`pkg/parser/parser_test.go:2136-2159`) logs "Negative error codes not supported (expected)" at `:2149` and `return`s — it never fails, so the doc/impl divergence is permanently green.
- Suggested fix: Either extend the lexer/AST to accept an optional leading `-` (and add `strconv.Atoi` parsing) so documented reserved codes can be declared, or remove the negative-code claims from both docs and make the test assert the rejection explicitly.
- Acceptance criteria:
  - [ ] `errors { -32700 ParseError "..." }` parses and yields `Code == -32700`, or documentation explicitly forbids negative codes.
  - [ ] `TestJSONRPCReservedErrorCodes` uses `t.Fatal`/`t.Error` instead of `t.Logf` + `return`.

### core-02 — Error-code uniqueness is global across merged IDL, docs say per-file
- Severity: medium
- Invariant: G15
- Current behavior: A single `errorCodes map[int]lexer.Position` is built over **all** `idl.Errors`, including imported errors merged at `pkg/parser/parser.go:638-692`, and any collision is rejected (`pkg/parser/validator.go:101-115`).
- Expected behavior: `docs/idl-guide/errors.md:178` states "Unique codes: Each error code must be unique within a file". Implementation is strictly stronger than the documented contract.
- Test status: `TestDuplicateErrorCodes` (`pkg/parser/parser_test.go:2063-2073`) only covers two declarations in the same file; no test covers the cross-file case (neither rejection nor acceptance).
- Suggested fix: Decide the intended scope. If global uniqueness is intended, update the doc to "unique across the merged IDL"; if per-file is intended, scope the registry by namespace/source file.
- Acceptance criteria:
  - [ ] Documentation and validator agree on the scope of error-code uniqueness.
  - [ ] A test covers two imported files that declare the same code.

### core-03 — `extends` accepts built-ins and any registered type, docs require a struct
- Severity: medium
- Invariant: G17
- Current behavior: Validation only checks `exists := typeRegistry[s.Extends]` **or** `builtInTypes[s.Extends]` (`pkg/parser/validator.go:160-170`). Therefore `struct X extends string {}` and `struct X extends SomeEnum {}` both pass.
- Expected behavior: `CLAUDE.md:95` says "`extends` keyword requires validating parent type exists **and is a struct**"; `docs/idl-guide/syntax.md:138-151` says "Extend existing structs".
- Test status: `TestInvalidExtendsNonExistentType` (`pkg/parser/parser_test.go:731-736`) only covers a missing parent; no test covers extending a built-in or an enum.
- Suggested fix: Check `typeNames[s.Extends] == "struct"` (using the registry already populated at `validator.go:43,64-98`) and drop the `builtInTypes` escape hatch.
- Acceptance criteria:
  - [ ] `struct X extends string {}` and extending an enum/interface/errors both produce a validation error.
  - [ ] Extending a valid struct still passes.

### core-04 — Cycle detection uses raw names and its tests never exercise it
- Severity: high
- Invariant: G18
- Current behavior: `detectCycles` keys `structMap` by `s.Name` (`pkg/parser/validator.go:327-330`), but field refs come from `getReferencedTypes` returning raw `t.UserDefined` (`validator.go:307-322`, used at `:394-397`) and `s.Extends` is the raw string (`:385-389`). Imported structs are renamed to `Namespace.Type` at `pkg/parser/parser.go:650`, while their field refs are deliberately left unqualified (`parser.go:651-653`), so a cycle spanning an import is invisible to `structMap[refType]`. Separately, when a cycle is found the recursion stack is never unwound before early `return true` (`:388-389`, `:403-404` vs cleanup at `:410-413`), producing false positives for later DFS roots.
- Expected behavior: "No transitive cycles via `extends` or required fields; optional fields break cycles" (catalog G18) must hold across namespaces, not just within a raw-name bucket.
- Test status: All negative cycle tests are non-asserting — `TestCycleDetectionSelfReferenceWithoutOptional` (`parser_test.go:509-529`), `TestCycleDetectionIndirectCycle` (`:540-561`), `TestCycleDetectionThroughExtends` (`:574-590`), `TestCycleDetectionInArray` (`:592-606`), `TestCycleDetectionInMap` (`:617-631`) only call `t.Log` on success. Worse, they call `ValidateIDL` on input with **no namespace**, so `validator.go:29-38` returns early and `detectCycles` (`:177`) is never reached; `TestCycleDetectionInMapWithOptional` (`:633-649`) even `return`s on any error.
- Suggested fix: Resolve every reference with `resolveTypeName(ref, typeRegistry, s.Namespace)` before `structMap` lookup; always clean `recursionStack`/`visited` via `defer`; rewrite cycle tests to use `assertValidationError(t, input, "circular")` with a namespace prepended.
- Acceptance criteria:
  - [ ] A two-file cycle (`a.A.required -> b.B`, `b.B.required -> a.A`) is rejected.
  - [ ] Each cycle test fails when no "circular type reference" error is produced; optional-field variants still validate.

### core-05 — Checksum is 43 chars; spec requires 44 padded
- Severity: medium
- Invariant: G20
- Current behavior: `ComputeChecksum` uses `base64.URLEncoding.WithPadding(base64.NoPadding)` on a 32-byte SHA-256 (`pkg/parser/checksum.go:58-59`), yielding 43 characters. `checksum_test.go:34-38` asserts 43.
- Expected behavior: The approved spec requires "a non-empty string of 44 base64url characters" (`openspec/changes/archive/2026-04-12-idl-checksum/specs/idl-checksum/spec.md:11-13`; proposal `openspec/changes/archive/2026-04-12-idl-checksum/proposal.md:8`) — i.e. padded base64.
- Test status: `checksum_test.go:34-38` actively enforces the wrong (43) length, so the spec scenario has no passing assertion anywhere.
- Suggested fix: Choose one canonical format. To satisfy the archived spec, switch to `base64.URLEncoding` (padded, 44 chars incl. `=`) and update the two length assertions (`checksum_test.go:36, :322`) plus all runtime checksum parsers; otherwise amend the spec and proposal to 43 unpadded chars.
- Acceptance criteria:
  - [ ] `len(ComputeChecksum(idl)) == 44` and the value is padded base64url, or the spec/proposal are updated to 43.
  - [ ] Generator and runtime `extractChecksum` implementations tolerate the chosen format.

### core-06 — `TestChecksumBookPulse` computes a checksum of an empty string
- Severity: medium
- Invariant: G22
- Current behavior: `TestChecksumBookPulse` calls `ParseIDL("book.pulse", "")` (`pkg/parser/checksum_test.go:311`) — it never reads a file despite the comment and function name. The only assertion is `len(checksum) == 43` (`:322-324`); the "expected checksum" is merely `t.Logf`'d (`:327-328`).
- Expected behavior: A regression test for a real `.pulse` fixture should parse actual file content and pin the resulting digest.
- Test status: none (vacuous test).
- Suggested fix: Read `book.pulse` from a known fixture path via `os.ReadFile`, fail on read error, and compare against a committed golden checksum constant.
- Acceptance criteria:
  - [ ] The test reads a non-empty `.pulse` fixture.
  - [ ] A golden checksum is asserted with `t.Errorf`, not logged.

### core-07 — No uniqueness checks for enum values, field names, parameter names, or method names
- Severity: medium
- Invariant: G23
- Current behavior: The first pass of `ValidateIDL` registers only interfaces/structs/enums/errors and their codes (`pkg/parser/validator.go:41-128`). The second pass calls `validateIdentifierName` for methods, params and fields (`:131-174`) but never records their names in a set; enum values are stored verbatim by the parser (`pkg/parser/parser.go:139-144, 589-599`) and never checked at all. Duplicate names therefore validate. This also makes the checksum ambiguous: `writeEnum` sorts and emits duplicate `value:` lines (`checksum.go:171-183`).
- Expected behavior: Duplicate members within a scope should be a validation error (parallel to duplicate type names at `validator.go:52-98`).
- Test status: none for enum values/fields/params/methods (only `TestDuplicateTypeNames` at `parser_test.go:712-729`, which itself only `t.Log`s at `:727`).
- Suggested fix: Add per-enum, per-struct, per-interface and per-method `map[string]bool` sets in `ValidateIDL` and emit a `duplicate <kind> name: X` `ValidationError` with the position.
- Acceptance criteria:
  - [ ] `enum E { a a }`, duplicate struct fields, duplicate method params, and duplicate methods in one interface each fail validation.
  - [ ] `TestDuplicateTypeNames` is converted to a real assertion.

### core-08 — Imported IDL files without a namespace are silently accepted
- Severity: medium
- Invariant: G25
- Current behavior: `parseIDLWithImports` never calls `ValidateIDL` on an imported file (`pkg/parser/parser.go:426`). If no namespace can be extracted from the imported types (`:431-462`), the import's declarations are appended as-is with empty namespaces (`:685-691`), and the merged root still validates as long as the **root** has a namespace (`pkg/parser/validator.go:26-39`).
- Expected behavior: "All IDL files **must** declare a namespace" (`CLAUDE.md:62`; `docs/idl-guide/syntax.md:14`), and errors specifically "must be in a namespace" (`docs/idl-guide/errors.md:177`).
- Test status: `TestErrorsBlockWithoutNamespace` (`pkg/parser/parser_test.go:2095-2112`) covers only a root file, not an import; the survey-cited import test (`parser_test.go:1258,1322,1379,1517`) does not assert missing-namespace rejection.
- Suggested fix: In `parseIDLWithImports`, fail when an imported file yields no namespace (or when its `IDL.RootNamespace` is empty) before merging, with a file-path-bearing error.
- Acceptance criteria:
  - [ ] Importing a `.pulse` file with declarations but no namespace is a parse/validation error naming that file.
  - [ ] Empty imported files (no declarations) remain acceptable.

### core-09 — Generated `VerifyCompatibility` diffs the embedded IDL against itself
- Severity: critical
- Invariant: M15
- Current behavior:
  - Go: `clientIDL` is `c.contract.GetIDLParsed()` (`pkg/generator/go_client_server.go:1316-1321`) and `serverIDL` is `c.contract.GetIDLParsed()` from the **same** contract (`:1322-1325`), then `pulserpc.DiffIDL(clientIDL, serverIDL)` (`:1334`). The client constructor only loads local `idl.json` (`:1198-1201`); no server IDL is ever fetched.
  - Java: both sides are `jsonParser.fromJson(IDL_JSON, Object.class)` (`pkg/generator/java_client_server.go:1287-1288`), diffed at `:1289`.
  - C#: `clientIDL` is the embedded `IdlHolder.GetIDLJson()` (or a caller-supplied `_localIDL`, `csharp_client_server.go:1532-1540`) and the "server" side is again `IdlHolder.GetIDLJson()` (`:1541-1542`).
  The result is always 0 deltas and `Compatible=true`; checksums are read from the same object (`go:1326-1332`, `java:1299-1300`, `cs:1555-1556`).
- Expected behavior: The spec requires the method to "Parse the embedded local IDL", "Retrieve the server's IDL (fetched at bootstrap)", then "Compute the diff" (`openspec/specs/contract-compatibility-verification/spec.md:90-95`). The breaking-change scenario requires `Compatible=false` when the server's schema diverges (`:115-118`).
- Test status: No generator test exercises `VerifyCompatibility` against a differing server IDL; the Go runtime unit test (`pkg/runtime/runtimes/go/pulserpc/integration_test.go:83-84,165-166`) tests `DiffIDL` directly, not the generated client path.
- Suggested fix: Generate a real fetch of the server IDL (RPC `pulserpc-idl`, already produced by Java at `java_client_server.go:1032-1035` and C# at `csharp_client_server.go:1022-1025`) into a `serverIDL` field at bootstrap, and diff that against the local IDL; add an end-to-end test with divergent client/server contracts.
- Acceptance criteria:
  - [ ] A generated client connected to a server with a removed required field gets `Compatible=false` and an Error delta.
  - [ ] `ServerChecksum != ClientChecksum` is possible in a generated client test.

### core-10 — Guide promises full JSON-RPC 2.0 compliance; generated adapters omit required behaviors
- Severity: high
- Invariant: M5, M6, M7
- Current behavior:
  - Parse error `-32700`: present in generated Go (`go_client_server.go:920,926,1396`), Python (`python_client_server.go:993-1000`), C# (`csharp_client_server.go:814`) and TS (`ts_client_server.go:2140`), but **absent entirely** from `java_client_server.go` (no `-32700` occurrence).
  - Batch requests: only Go (`go_client_server.go:930-951`) and C# (`csharp_client_server.go:820`) handle arrays; no batch branch in TS/Python/Java generators.
  - Response envelope/error handling: generated clients read only `response["result"]` and never inspect `response["error"]` (Go `go_client_server.go:1279-1293`; Java `java_client_server.go:574`; C# `csharp_client_server.go:1646` "Missing result in response"), so a server error surfaces as a bogus "missing result" / internal error and result+error XOR is unchecked. No generated adapter validates incoming `id` type/range.
- Expected behavior: "Both server and client must fully comply with JSON-RPC 2.0 specification", including batch requests, notifications, standard error codes and the result-or-error response shape (`docs/RUNTIME_IMPLEMENTATION_GUIDE.md:808-815`, response format at `:812`); checklist also requires "Server handles batch requests" (`:1212`).
- Test status: none in the generator suite; `tests/integration/test_http_api.sh:35-73` only checks `"error"`/`"result"` presence via substring `grep`, so envelope violations and batch behavior are never asserted.
- Suggested fix: Add `-32700` and batch handling to the Java/TS/Python generated adapters, add `error`-before-`result` handling to all generated clients, and add a conformance test that sends malformed JSON, batches, and dual `result`+`error` responses to every generated server.
- Acceptance criteria:
  - [ ] Every generated server returns `-32700` for malformed JSON and handles batches (or the guide is amended to mark these optional).
  - [ ] Every generated client surfaces the server's `error` object rather than "missing result".

### core-11 — Integration harness prints WARN and counts PASS on wrong error codes; runtime matrix is incomplete
- Severity: high
- Invariant: M17
- Current behavior: `test_validation_error` prints `⚠ WARN ... wrong code` at `tests/integration/test_http_api.sh:128-136`, then unconditionally prints `✓ PASS`, increments `TESTS_PASSED` and returns 0 at `:140-142`. The multi-runtime driver loops only `python ts csharp` (`tests/integration/test_all_runtimes.sh:70`) even though `java` is declared with a port at `:25`; `go`, `python2` and `ts-cjs` are absent entirely. `test_all_runtimes.sh` is not wired into the root `Makefile` (`Makefile:141` `test-runtimes` runs per-runtime `test-integration` targets only).
- Expected behavior: An integration suite must fail when the observed JSON-RPC error code differs from the expected one, and the advertised "all runtimes" harness should cover all seven runtimes named in `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:14-15`.
- Test status: `tests/integration/test_http_api.sh:75-143` is the harness itself; its WARN branch means code mismatches are silently green in every caller (`test_all_runtimes.sh:104`, `test_generator*.sh`, `test_generator.sh:169`).
- Suggested fix: Change the mismatch branch at `:128-136` to increment `TESTS_FAILED`, record the description, and `return 1`; add `java`, `go`, `python2`, `ts-cjs` entries to `RUNTIME_PORTS` and the loop, and invoke the harness from a Makefile target.
- Acceptance criteria:
  - [ ] Injecting a wrong error code makes `test_http_api.sh` exit non-zero.
  - [ ] `make test-all-runtimes` runs the HTTP suite against all advertised servers.

### core-12 — Severity spec scenario contradicts its own table and the implementation
- Severity: medium
- Invariant: M16
- Current behavior: The scenario "server has removed a required field" expects `Compatible=false` with an Error-level delta (`openspec/specs/contract-compatibility-verification/spec.md:115-118`), but the same spec's classification table says "Field removed | ClientHasMore | Info" (`:25`). The Go runtime implements Info (`pkg/runtime/runtimes/go/pulserpc/severity.go:17-18`) and the test asserts it (`pkg/runtime/runtimes/go/pulserpc/integration_test.go:165-166`).
- Expected behavior: The spec must give a single, unambiguous expectation. Given the direction model (`:9-14`), "server removed a field the client still has" is ClientHasMore → Info; the Error case is "server **added** a required field the client doesn't send" (`:26`, implemented at `severity.go:20-23`).
- Test status: `integration_test.go:165-166` locks in Info, so the spec scenario can never be satisfied by the current code.
- Suggested fix: Reword the scenario at `spec.md:117` to the ClientHasLess/required case (e.g. "server added a required field"), leaving the Info classification intact, or change the table + `ClassifySeverity` if removal really should be breaking.
- Acceptance criteria:
  - [ ] The scenario's stated severity matches the table row and `ClassifySeverity` output.
  - [ ] A spec-driven test for the scenario passes.

### core-13 — Docs advertise a validation error `data` shape that no shared code emits
- Severity: low
- Invariant: M13
- Current behavior: `docs/idl-guide/validation.md:229-246` documents `data: {"field": "email", "message": "Required field 'email' is missing"}`. Neither the generators (no `"field"` literal in any `*_client_server.go`) nor the shared docs define a producer; generated errors pass through opaque strings (e.g. `sendError(..., data string)` at `go_client_server.go:1418-1424`).
- Expected behavior: Documented wire contracts must match what generated/runtime code actually emits; the survey reports runtimes use strings or `ValidationResult` objects instead.
- Test status: none — no test validates the shape of validation-error `data`.
- Suggested fix: Either define and emit a canonical `{field, message}` payload in the shared validation error path, or update `validation.md` (and the per-runtime references) to show the actual emitted shape.
- Acceptance criteria:
  - [ ] The documented example in `validation.md` is byte-for-byte reproducible from at least one runtime.
  - [ ] A conformance test asserts the `data` shape on a missing-required-field error.

### core-14 — Go generator never sets `idl.Checksum`, so embedded `IDL_JSON` has no checksum
- Severity: low
- Invariant: G21 (verified with a correction)
- Current behavior: The catalog claim "Go generator never calls ComputeChecksum" is a **false positive** — `writeIDLJSONGo` computes it (`pkg/generator/go_client_server.go:55-58`) and writes it into `idl.json` via a local map (`:60-66`). However, unlike Java (`:189-193`), Python (`:269-273`), TS (`:1246-1250`) and C# (`:148-152`), Go does not assign `idl.Checksum`. Consequently the `IDL_JSON` constant, which marshals the bare `*parser.IDL` (`go_client_server.go:793-798`), omits the `checksum` key entirely.
- Expected behavior: Spec requires a top-level `checksum` in the serialized IDL (`openspec/changes/archive/2026-04-12-idl-checksum/specs/idl-checksum/spec.md:3-13`); every other generator embeds it.
- Test status: Generator tests assert only that `idl.json` exists, not that both serialized forms contain `checksum`.
- Suggested fix: Set `idl.Checksum = checksum` in `writeIDLJSONGo` (then build the map from `idl` or keep the explicit key) so the embedded `IDL_JSON` also carries the digest.
- Acceptance criteria:
  - [ ] Generated Go `server.go`'s `IDL_JSON` constant contains a `"checksum"` key.
  - [ ] A generator test asserts `idl.json` and `IDL_JSON` have equal checksums.

### core-15 — Generated Go test server double-encodes the `/idl.json` payload
- Severity: medium
- Invariant: NEW
- Current behavior: `IDL_JSON` is a Go string constant holding JSON text (`pkg/generator/go_client_server.go:795-797`), but the generated `/idl.json` handler does `json.Marshal(IDL_JSON)` (`:1409`), which yields a quoted JSON *string* rather than the IDL object. The generated Java and C# servers instead parse `IDL_JSON` into an object before returning it (`java_client_server.go:1032-1035`, `csharp_client_server.go:1022-1025`).
- Expected behavior: `/idl.json` (and the `pulserpc-idl` RPC) should serve a JSON object with top-level `checksum`, interfaces, structs, etc. (`idl-checksum/spec.md:5-9`).
- Test status: No test fetches `/idl.json` and parses it as an object.
- Suggested fix: Emit `w.Write([]byte(IDL_JSON))` (or unmarshal then `json.NewEncoder(w).Encode(obj)`) in the generated handler at `go_client_server.go:1407-1410`.
- Acceptance criteria:
  - [ ] `curl /idl.json` on a generated Go test server returns a JSON object whose `.checksum` is a string.
  - [ ] A generator test unmarshals the endpoint body into `map[string]interface{}` and checks `interfaces`/`checksum`.

### core-16 — Aborted cycle DFS leaves `recursionStack` polluted (false positives)
- Severity: low
- Invariant: NEW
- Current behavior: In `detectCycles`, a detected cycle returns `true` through `:388-389` and `:403-404` before the cleanup at `:410-413`, so nodes remain in `recursionStack` and are not marked `visited`. A subsequent DFS root (`validator.go:417-420`) that merely references one of those nodes is misclassified as a cycle.
- Expected behavior: DFS state must be restored on every return path; only genuine cycles should be reported.
- Test status: none (see core-04's non-asserting cycle tests).
- Suggested fix: Use `defer func(){ delete(recursionStack, structName); visited[structName] = true }()` immediately after pushing, and make the cycle branch record the error without early-aborting the whole traversal.
- Acceptance criteria:
  - [ ] A schema with one real cycle and one separate acyclic chain reachable from another root yields exactly one cycle error.
  - [ ] All cycle tests use real assertions (per core-04).

## Verified-correct invariants
- G1 — Comments are `//` only; lexer has a single `Comment` rule (`pkg/parser/parser.go:16`).
- G3 — Identifiers `[a-zA-Z][a-zA-Z0-9_]*` in lexer (`parser.go:35`) and validator regex (`validator.go:19`).
- G4 — Namespace is a single `@Ident` (`parser.go:78-81`).
- G5 — Top-level elements `@@*` with empty bodies allowed (`parser.go:48-61,87,117,149`).
- G6 — Exactly four built-ins in grammar (`parser.go:163`) and registry (`validator.go:12-17`).
- G7 — `ArrayType.ElementType` is recursive (`parser.go:169-175,191-196`).
- G8 — Map grammar is `@Map '[' @String ']'` (`parser.go:177-183`).
- G9 — Optional token is exactly `\[optional\]` (`parser.go:18`); field/method capture it (`parser.go:96,136`).
- G10 — Method grammar fixes order return → optional → raises (`parser.go:91-98`).
- G12 — Exactly one namespace enforced in `parseIDLWithImports` (`parser.go:394-403`) and `ValidateIDL` (`validator.go:26-39`); empty files exempt (`:28`, test `parser_test.go:655-659`).
- G13 — Type names are registered across all four declaration kinds with duplicate detection (`validator.go:41-128`) — implementation correct despite the non-asserting test at `parser_test.go:712-729`.
- G14 — User-defined refs resolved against the declaring namespace (`validator.go:222-233,285-305`).
- G16 — Every `raises()` name is resolved and checked (`validator.go:145-156,262-283`).
- G19 — Import cycles, duplicate namespaces, path dedup and missing-file fatals all present (`parser.go:349-381,420-429,464-470`).
- G20 (implementation half) — Checksum canonicalization sorts interfaces/structs/enums/errors/methods/params/fields/values and excludes comments/whitespace (`pkg/parser/checksum.go:11-59,102-187`); only the 43-vs-44 spec mismatch (core-05) remains.
- G21 (corrected) — Go *does* compute and persist the checksum for `idl.json` (`go_client_server.go:55-66`); only the embedded `IDL_JSON` is missing it (core-14).
- G24 — Preceding comments require direct adjacency with no blank line (`parser.go:212-252`; enum values `parser.go:254-344`).
- M1 (partial) — Generated Java validates `jsonrpc == "2.0"` with `-32600` (`java_client_server.go:1016-1024`).
- M3 — Method names use `Interface.method` in generated clients (`go_client_server.go:1263`).

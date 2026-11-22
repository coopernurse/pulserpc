# Additional PulseRPC Invariants

Companion to [invariants.md](invariants.md), discovered by reviewing the code at
`f2de19b`. The original catalog is a pre-fix survey; several of its `[V]`
markers are now resolved (see "Status updates" below). This file adds invariants
that were not documented at all.

**Markers:** **[H]** holds in the current code · **[V]** violated · **[D]**
divergent across runtimes · **[U]** undocumented (no statement in the runtime
guide or catalog; may or may not be tested).

Line references are from `f2de19b`.

---

## Status updates to the original catalog

These are no longer `[V]`; the catalog text is stale.

- G11 — negative error codes now parse (`parser.go:20,155`).
- G17 — `extends` must name a struct (validator `validateExtends`-style check).
- G20 — checksum is 43-char unpadded base64url; spec amended to 43.
- G21/G22 — Go sets `idl.Checksum`; `TestChecksumBookPulse` reads the file.
- G23 — duplicate enum values/fields/params/methods are rejected.
- G25 — imported files with declarations must declare a namespace.
- R1 — ts-cjs exports the full validation/type surface.
- R2/R5/R6 — scalar sets, Java overrides, and type resolution aligned.
- R11/M4 — generated Java/C# servers suppress notification errors.
- M2/M5/M6/M9/M10/M14/M15/M17 — the audit fixes landed.

---

## Part 4 — New grammar / parser invariants

- **G26 [H][U] No string escapes.** `StringLiteral` is `"[^"]*"`
  (`parser.go:19`). Backslashes are literal and `\"` is a lexer error. This
  constrains error messages and import paths. Verified: an escaped quote is a
  lexer error.
- **G27 [H][U] Method return type is mandatory.** `MethodDef.ReturnType` is a
  required `@@"` (`parser.go:95`), and there is no `void` keyword in the lexer.
  `void` exists only as an internal AST `Type.BuiltIn` value.
- **G28 [H][U] Parameters are positional and order-significant.**
  `ParameterDef` captures source order (`parser.go:105-110`). Runtimes and
  generated clients dispatch positionally.
- **G29 [H][V][U] Imports are preprocessed by regex, not the grammar.**
  Extraction uses `(?m)^\s*import\s+"([^"]+)"` while removal requires the line
  to end after the closing quote (`parser.go:369,385`). An import with a
  trailing comment is resolved but not removed, so parsing fails. Verified.
- **G30 [H][U] Import de-duplication is by literal path string.** Two strings
  that resolve to the same file (`inc.pulse` and `./inc.pulse`) are parsed
  twice and rejected as a duplicate namespace (`parser.go:371-380,475-480`).
  Verified.
- **G31 [H][U] Import paths resolve relative to the importing file's
  directory; absolute paths are accepted** (`parser.go:414-418`).
- **G32 [H][V][U] The root namespace is not compared with imported
  namespaces.** `namespaceMap` only tracks imports (`parser.go:411,475-480`),
  so a root file and an import may declare the same namespace. Verified.
- **G33 [H][U] Import prefixing is asymmetric.** Imported declarations are
  renamed `Namespace.Type`; their field/parameter/`extends` references are left
  unqualified. Root declarations are not prefixed (`parser.go:650-680`).
- **G34 [H][U] The checksum excludes error message text and comments** but
  includes field/return optionality, enum values, raises, and resolved type
  names (`checksum.go:147-187`). Verified: changing an error message does not
  change the checksum.
- **G35 [H][V][U] The checksum is insensitive to parameter order.** `writeMethod`
  sorts parameters by name and emits no positional index
  (`checksum.go:119-127`), so `m(x int, y string)` and `m(y string, x int)`
  hash identically even though they are different APIs. Verified.
- **G36 [H][U] Error codes are decimal, optionally negative.** `IntLiteral` is
  `-?[0-9]+` (`parser.go:20,155`); no `+`, underscores, or hex.
- **G37 [H][U] At most one namespace per file; the declaration may appear
  anywhere among top-level elements** (`parser.go:394-403`).
- **G38 [H][U] Root declarations are not namespace-prefixed; only imported
  declarations are.** See G33.
- **G39 [H][U] Empty files may omit a namespace; non-empty files (root or
  imported) may not** (`validator.go:28-39`; `parser.go:431-439`).

---

## Part 5 — New generator / output invariants

- **C1 [V][U] Generation is byte-deterministic.** Generating the same IDL twice
  must produce identical bytes. Java currently varies import order
  (`java_client_server.go:376,463,932,1582,1749,1833`). Verified by generating
  twice and comparing hashes.
- **C2 [V][U] All generators emit the same top-level `idl.json` schema.**
  Go builds its own map without `rootNamespace`
  (`go_client_server.go:64-70`); Python/Java/C# include `rootNamespace`. Verified.
- **C3 [D][U] IDL metadata placement is uniform.** Go writes `<dir>/idl.json` at
  the root; Python writes `<dir>/<namespace>/idl.json`; Java/TS/C# embed
  `IDL_JSON` and write no root file. Verified.
- **C4 [H][U] The checksum in `idl.json` and in the embedded `IDL_JSON` agree**
  (`go_client_server.go:61-70`; `go_generator_audit_test.go:33-70`).
- **C5 [H][U] `IDL_JSON` is valid JSON object text served as an object**, not a
  double-encoded string (`go_client_server.go:1515-1520`).
- **C6 [H][U] TS config generation refuses to overwrite.** `-ts-gen-package-json`
  and `-ts-gen-tsconfig` error if the file exists
  (`ts_client_server.go:799-805,883-889`).
- **C7 [H][U] TS module-style precedence:** `-ts-module` > tsconfig
  `compilerOptions.module` > package.json `type` > `esm-node`; `-ts-no-detect`
  disables detection (`ts_client_server.go:62-74,231-240`).
- **C8 [D][U] Generated layout is stable and documented per runtime** (namespace
  dirs, runtime copy, test files, `src/main/java`, `src/test/java`, etc.).
- **C9 [H][U] Go JSON tags distinguish required and optional fields.** Required
  fields have no `omitempty`; optional fields do (`go_client_server.go:743-747`).
- **C10 [V][U] Optional built-in fields are nullable in statically typed
  generated structs.** Java maps optional `int`/`bool`/`float` to primitives
  (`getJavaType`); C# maps optional `int` to `int` and `string` to `string`
  (only `bool`/`double` become nullable, `csharp_client_server.go:368-371`).
  Verified by generation: `int A`, `string B`, `bool? C`, `double? D`.
- **C11 [H][U] Enums serialize as declared string names, not ordinals**
  (C# uses `JsonStringEnumConverter`, `csharp_client_server.go:540`; Java enums
  serialize by name; Go/TS/Python use strings).
- **C12 [H][U] Generator flag defaults:** `-json-lib jackson`,
  `-inline-runtime true`, `-package` required for Java and effectively for Go
  without a `go.mod` (`go_client_server.go:35-43`,
  `java_client_server.go:35-38`).
- **C13 [H][U] Generated code compiles under the selected module system**
  (ts-node ESM, ts-cjs CJS with `moduleDetection: force` +
  `ignoreDeprecations: 5.0`, Java `pom.xml`, C# `csproj`).

---

## Part 6 — New runtime / wire invariants

- **R15 [V][U] Internal errors never leak implementation details.** Handler
  exceptions must be logged server-side, not put in `error.message`/`error.data`.
  Violations: Go runtime `err.Error()` data (`server.go:150`), Java
  `e.getMessage()` (`java_client_server.go:1012,1116,1205,1228,1237`), C#
  `e.Message` + `e.StackTrace` (`csharp_client_server.go:1309`), Python
  `Handler exception: {e}` (`server.py:223`), Python2 (`server.py:124`), TS
  (`ts-node/server.ts:202`; `ts-cjs/server.ts:174`).
- **R16 [D][U] Request body size is bounded.** Only the generated Python server
  caps it (`MAX_BODY_SIZE = 1 MiB`, `python_client_server.go:991`); generated
  Go (`io.ReadAll(r.Body)`), Java (`readAllBytes`), and TS are unbounded
  (C# relies on Kestrel's default).
- **R17 [D][U] Response body size is bounded.** Only the generated Go client
  caps it (`maxResponseSize = 10 MiB`, `go_client_server.go:1112`).
- **R18 [D][U] Batch request count is bounded.** Only the Python runtime
  (`DEFAULT_MAX_BATCH_SIZE = 100`, `server.py:18`); Go/Java/C# adapters are
  unbounded.
- **R19 [D][U] HTTP client timeouts are bounded and consistent.** Generated Go
  uses `&http.Client{}` (no timeout, `go_client_server.go:1130`), Java uses
  10s connect/30s request (`HTTPTransport.java:23,35`), Python 30s
  (`transport.py:33`), C# the `HttpClient` default, TS no timeout.
- **R20 [D][U] Concurrent client calls use unique ids.** Generated Go uses
  `atomic.AddUint64`; the Go runtime client (`c.requestID++`), Java runtime
  (`requestID++`), Python (`_request_id += 1`), and Python2 are not atomic;
  C# uses GUIDs.
- **R21 [V][U] Runtime caches are thread-safe.** Python's global resolution
  index is an `OrderedDict` cache with no lock (`rpctypes.py`
  `_index_cache`/`_get_resolution_index`); concurrent validation can race.
  Go/Java/TS/C# use atomic/`WeakHashMap`/`WeakMap`/read-only maps.
- **R22 [D][U] Generated servers accept only POST with a consistent
  status/shape.** Go/C#/TS return 405, Java returns HTTP 200 with a JSON-RPC
  `-32600`, Python returns 501 (see the respective generators).
- **R23 [H][U] Notifications produce no response body.** Fixed for all
  generated servers; the status is 204 in Go/Python/C#/TS and 200 with an empty
  body in Java.
- **R24 [V][U] JSON-RPC `id` is a string or number.** No server validates the
  type; servers echo booleans/objects as the response id (TS types it as
  `string | number | null`, `types.ts:9,22`, but does not enforce it).
- **R25 [D][U] IDL `int` has a uniform, documented range.** Java and C# generate
  32-bit `int`; Go uses 64-bit `int`; TypeScript numbers are safe to 2^53;
  Python is arbitrary precision. Values beyond 2^31−1 differ.
- **R26 [H][U] IDL `float` is IEEE-754 double.** Go `float64`, Java/C# `double`,
  TS `number`, Python `float`.
- **R27 [H][U] Enums are wire strings** (see C11).
- **R28 [H][U] Untrusted `meta` keys cannot pollute prototypes.** TS rejects
  `__proto__`/`constructor`/`prototype` (`contract.ts` `UNSAFE_META_KEYS`);
  Go/Java/Python use maps.
- **R29 [D][U] Transports support custom headers.** Go/Python/Python2/C# do;
  the Java `HTTPTransport` constructor takes only a URL and parser
  (`HTTPTransport.java:19`).
- **R30 [D][U] Base-URL trailing slashes are normalized.** Java and C# trim;
  Go/Python/TS use the URL as given.
- **R31 [H][U] Contract verification diffs against a fetched server IDL.**
  Generated Go/Java/C# fetch `pulserpc-idl`; the TS/Python runtime clients
  fetch it at bootstrap (`ts-node/client.ts:142,220`; `python/client.py:104,329`).
- **R32 [H][U] `idl.json` carries declaration comments; the checksum excludes
  them** (see G34). Verified in the Java/Python embedded IDL.

---

## Part 7 — New message-validation invariants

- **M18 [H][U] An empty batch returns a single `-32600` error object**, not an
  array (Go/Java/Python/C#; TS fixed).
- **M19 [D][U] Non-object batch entries produce a per-entry `-32600`** and the
  overall response remains an array. Go/Java/C# handle this explicitly; the TS
  adapter delegates to `Server.call`.
- **M20 [D][U] Object-form (named) params are accepted or rejected explicitly.**
  Go normalizes them (`server.go namedParamsToPositional`); Java/C# runtime
  servers still silently drop object-form params.
- **M21 [H][U] `id: null` is treated as a notification** consistently across
  servers.
- **M22 [H][U] `error.data` is omitted when nil**, not serialized as `null`
  (Java/C#/Python error envelopes).
- **M23 [H][U] A null result is serialized as an explicit `"result": null`**
  (Gson now uses `serializeNulls`; all runtimes).
- **M24 [H][U] Response-id comparison is number-type tolerant.** Go uses
  `idsEqual`; Java uses `idsMatch`; TS `===`; Python `==`; C# stringifies.
- **M25 [H][U] Error objects have a numeric `code` and string `message`;**
  clients validate this (Go/Java/TS/Python/Python2; C# validates in `Client`).
- **M26 [H][U] Malformed JSON returns `-32700` in every generated server.**
- **M27 [H][U] `jsonrpc` is `"2.0"` on every request and response;** clients
  validate the response version.
- **M28 [H][U] Notifications inside a batch are dropped; an all-notification
  batch returns no body.**

---

## Part 8 — New security / limit invariants

- **S1 [V][U] Playground file download cannot escape the session directory.**
  The handler passes the raw URL suffix (`webui/server.go:296-330`) and
  `Manager.GetFile` does `filepath.Join(session.Dir, filename)` with no
  containment check (`playground/files.go:21`). Depends on `ServeMux` path
  cleaning; the library API itself is unsafe.
- **S2 [H][U] `embed.FS` paths use forward slashes on every OS**
  (`runtime/embed.go`, `webui/server.go`).
- **S3 [H][U] ZIP entry names use forward slashes and are session-relative**
  (`playground/files.go:70-72`).
- **S4 [H][U] Generated paths/imports use the correct separator on every OS**
  (`filepath` for filesystem, `path`/`ToSlash` for embed/zip/import specifiers).
- **S5 [H][U] Untrusted `meta` keys cannot mutate prototypes** (see R28).
- **S6 [V][U] Internal error details are redacted from clients** (see R15).
- **S7 [D][U] Body/response/batch limits prevent request amplification** (see
  R16-R18).
- **S8 [D][U] Non-POST requests are rejected consistently** (see R22).

---

## Part 9 — Integration-test coverage assessment

Filled in by the coverage subagents; see
[integration-coverage.md](integration-coverage.md).

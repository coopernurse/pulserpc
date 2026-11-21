# PulseRPC Invariant Catalog

Result of a three-axis survey (grammar/parser, language runtimes, client/server
message validation). This is the reference for the invariant audit.

**Markers:** **[C]** covered by a test · **[G]** test gap · **[V]** apparent
violation or divergence.

**Enforcement split:** `ParseIDL()` does syntax + imports
(`pkg/parser/parser.go:348`); `ValidateIDL()` does all semantics
(`pkg/parser/validator.go:23`). Most "valid" unit tests cheat by prepending
`namespace test` (`parser_test.go:12-31`).

**Canonical runtime contract:** `docs/RUNTIME_IMPLEMENTATION_GUIDE.md`.
Seven runtimes: go, java, python, python2, ts-node, ts-cjs, csharp.

---

## Part 1 — Grammar / IDL invariants

### Syntax

- **G1** Comments are `//` only; no block comments. `parser.go:16`. [C] `parser_test.go:224,759`
- **G2** Closed keyword set; word-boundary lexer keeps `strings`/`mapstring` as identifiers. `parser.go:15-38`. [G] no keyword-as-identifier test
- **G3** Identifiers are `[a-zA-Z][a-zA-Z0-9_]*`. `parser.go:35`, `validator.go:19,244`. [C] `parser_test.go:306`
- **G4** Namespace is a single ident (no dotted names). `parser.go:78-81`. [G]
- **G5** Top-level elements in any order/count; empty bodies allowed. `parser.go:48-61`. [C] `parser_test.go:261,670`
- **G6** Exactly four scalars: `string,int,float,bool`. `parser.go:163`, `validator.go:12`. [C] `parser_test.go:855`
- **G7** Arrays `[]T` nest arbitrarily. `parser.go:169-206`. [C] `parser_test.go:474`
- **G8** Maps are always `map[string]T`. `parser.go:177-183`. [C] `parser_test.go:432`
- **G9** Optional syntax is exactly `[optional]`. `parser.go:18`. [C] `parser_test.go:738`
- **G10** Method order: return type → `[optional]` → `raises(...)`. `parser.go:91-98`. [G] no negative-order test
- **G11** Error decl is `<int> <name> "<msg>"`; codes non-negative only. `parser.go:153-158`, `:20`. [V] docs advertise negative JSON-RPC codes (`docs/idl-guide/syntax.md:63`); test merely logs it (`parser_test.go:2136`)

### Declarations / semantics

- **G12** Non-empty file must declare exactly one namespace; empty file exempt. `validator.go:26-39`, `parser.go:394-403`. [C] `parser_test.go:2095,1357,655`
- **G13** Type names unique across all declaration kinds. `validator.go:41-98`. [C] `parser_test.go:712` ⚠ test is non-asserting
- **G14** All type refs must resolve; unqualified resolved against declaring namespace. `validator.go:187-241,291`. [C] `parser_test.go:359,1427`
- **G15** Error codes globally unique across merged IDL. `validator.go:101-115`. [C] `parser_test.go:2063`. [V] docs say "unique within a file" (`errors.md:178`) — implementation is stricter
- **G16** Every `raises()` name must resolve. `validator.go:145-156,262`. [C] `parser_test.go:2086`
- **G17** `extends` target must exist or be a built-in. `validator.go:160-170`. [V] docs/CLAUDE.md:95 say parent "must be a struct", but `struct X extends string {}` validates
- **G18** No transitive cycles via `extends` or required fields; optional fields break cycles. `validator.go:324-422`. [G] several cycle tests only `t.Log` warnings (`parser_test.go:509-649`); cycle detection uses raw names so qualified imported refs may evade it
- **G19** Imports: no cycles, no duplicate namespaces, dedup by path, missing file is fatal. `parser.go:349-470,371-380,420`. [C] `parser_test.go:1258,1322,1517,1379`
- **G20** Checksum is order-invariant, 43-char base64url SHA-256 over canonical AST, comments/whitespace excluded. `checksum.go:11-59`. [C] `checksum_test.go:57-544`. [V] spec says 44 padded chars (`spec.md:11`)
- **G21** Go generator does **not** set `idl.Checksum` (all others do). `go_client_server.go:54-79` vs `ts:1250, py:273, java:193, cs:152`. [V]
- **G22** `TestChecksumBookPulse` parses an empty string, not the file. `checksum_test.go:309-329`. [V] test gap
- **G23** No uniqueness checks for enum values, field names, param names, method names. [V] `enum E { a a }` validates. [G]
- **G24** Comments attached only when directly above with no blank line. `parser.go:208-344`. [C] `parser_test.go:1702-1978`
- **G25** Imported files without namespace are accepted (ValidateIDL not run per import). `parser.go:685-691`. [V] vs CLAUDE.md:62

---

## Part 2 — Language runtime invariants

- **R1** Every runtime exposes `validate_type`, `findStruct`/`findEnum`, `getStructFields`. [C] across all 7. [V] ts-cjs `index.ts:19` exports only `validateType`; ts-node exports all
- **R2** Scalar acceptance sets should be identical. **[V]** Go int has 1e-9 tolerance (`validation.go:18-29`); Java/C# float reject `Long`; Python int rejects bool but float accepts it; Python2 int accepts bool; TS float accepts NaN/Infinity; **ts-cjs int accepts `Infinity`** (`ts-cjs/validation.ts:33`) while ts-node rejects it. [G] no test covers the ts-node/ts-cjs split
- **R3** Aggregation model: fail-fast vs accumulating. **[V]** Python3 + both TS accumulate `ValidationError[]` with paths; Go/Java/Python2/C# fail fast. Python2 raises plain `ValueError` with no code/data. [C] multi-error tests in Python3/TS only
- **R4** Missing required field = error; missing optional = OK; null required = error. All 7 conform. [C] except **Java has no struct-validation test at all**
- **R5** Struct `extends`: parent fields resolved first, child overrides on conflict. **[V] Java does not dedupe child overrides** (`Types.java:41-50`) and its lookups are exact-only. [C] others have override tests; Java missing
- **R6** Qualified type-name resolution. **[V]** Go suffix-scan > Python/Python2 suffix-scan inside `validate_type` only > C# enum-base-name only > Java/TS exact-only. [G] no dedicated namespace test anywhere
- **R7** `RPCError` carries `code`, `message`, optional `data`. All 7 conform. [C] except Python2 has no RPC test
- **R8** No named error-code constants; literal `-32600..-32603` everywhere. TS transports uniquely use nonstandard `-32000` (`transport.ts:80,85,91`). [V]
- **R9** Transport APIs differ across all 7 (method+params vs request dict vs typed objects; sync/async; int vs GUID request IDs; Go HTTP hardcodes `id:1` at `transport.go:40,104`). [V]
- **R10** Server response validation. **[V] dead in both TS runtimes** (`validateResponses` set but never called: `ts-node/server.ts:15,31,36` vs `:163-172`), and generated Java never validates. Go/Java/Python/Python2/C# do. [G] no server-response unit tests
- **R11** Notification handling. **[V] both TS servers always return `id:null` response** (`ts-node/server.ts:168-172`); generated Java always emits `id`. Other 5 suppress correctly. [G] no tests
- **R12** `meta` items parsed by Go/Java/Python/Python2/C# but **not TS**. [V]
- **R13** Contract verification (diff/auditors/checksum). **[V] Python2 has none**; Go has no `extractChecksum`; ts-cjs has no diff test. Test files for Go/Java/Python exist but are excluded from their runtime Makefiles. [V]
- **R14** Void-return: TS accepts both `null` and `undefined`; others only null/nil/None. [V]

---

## Part 3 — Client/server message validation

- **M1** Incoming `jsonrpc` must be `"2.0"` → `-32600`. `server.go:41` etc. [C] partial (`test_http_api.sh:222` Python/TS/C# only). [V] generated C# checks presence extra; generated Java message differs
- **M2** `method` present, non-empty string. **[V] generated Java casts without check → NPE risk** (`java_client_server.go:1029`). [G]
- **M3** Method name `Interface.method` → `-32601`. [C] `test_http_api.sh:235`
- **M4** Notifications produce no response. **[V] TS runtime + generated Java fail** (see R11). [G] no tests
- **M5** `id` type/range and response-envelope validation (`jsonrpc`, id match, result XOR error). **[V] not enforced anywhere.** Clients let `error` silently win over `result` (`client.go:154-181`, `client.ts:322-344`). Contradicts guide `:810-815`. [G]
- **M6** Parse error `-32700` exists only in generated Go/Python/C#/TS HTTP adapters; **generated Java emits none**. [G]
- **M7** Batch requests only in generated Go/C# adapters. Guide says required. [V][G]
- **M8** `params` shape inconsistent: TS rejects non-object/array; Python requires list only; generated C# coerces to empty list; Go/Java/C# runtimes silently drop object-form params. [V][G]
- **M9** Exact parameter count. **[V] Go/Java/C# skip validation at zero params** (`server.go:95`, `Server.java:97`, `Server.cs:117`); TS/Python always validate. TS "trailing optional" logic is unreachable. [C] partial
- **M10** Per-param type validation against IDL. [C] except generated Java relies on JSON deserialization only — **no IDL validation** (`java_client_server.go:1129`). [V]
- **M11** Enum membership, nested/recursive validation, int-fractional rejection. [C] mostly; [G] Go doesn't test fractional rejection (`validation_test.go:19-32`)
- **M12** Extra/unknown fields never rejected in any language (intentional per docs). [C-ish] documented as permissive
- **M13** Validation error `data` shape. **[V] documented as `{field,message}` (`validation.md:235`) but no runtime produces it** — Go/Java/C# use strings, TS/Python use `ValidationResult`. [G]
- **M14** Server/client response validation. **[V]** `result != nil` guards mean null results for required returns skip validation everywhere (`server.go:121`, `client.go:172`, etc.). Generated Go client flags are inert (`go_client_server.go:1182-1211`). All clients default validation off except Go runtime explicit.
- **M15** Generated-client contract verification compares embedded IDL **against itself** in Go/Java/C# (`go_client_server.go:1316-1325`) → always compatible, zero deltas. [V][G]
- **M16** Severity spec contradiction: spec scenario says removed required field → Error/incompatible (`spec.md:115`), code classifies as Info (`severity.go:17-18`) and test asserts Info. [V]
- **M17** Integration tests print wrong error codes as WARN, not failure (`test_http_api.sh:128-142`), and only exercise Python/TS/C# servers. [V][G]

---

## Audit plan

Score each invariant on two axes: **test coverage** (C/G) and **conformance**
(uniform / divergent / violated). Triage by severity. See
`docs/audit/problems-<runtime>.md` for per-runtime audit results.

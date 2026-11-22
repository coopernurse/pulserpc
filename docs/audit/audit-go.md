# Audit: Go runtime

## Scope & methodology

Scope: Go runtime library `pkg/runtime/runtimes/go/pulserpc/` (server, client,
transport, validation, contract, diff, types, type index, rpc, severity,
auditor) and the Go generator `pkg/generator/go_client_server.go`, plus the
embedded IDL JSON / generated HTTP adapter. Parser and grammar invariants
(`G*`) are included only where they affect generated Go output or the runtime.

Methodology: re-verified every prior finding in `docs/audit/problems-go.md`
against the current source (treated as a stale baseline), ran
`go test ./pkg/runtime/runtimes/go/... -count=1` (both packages pass) and
`go test ./pkg/generator/ -run Go`, and ran small probe tests for the
conversion path. No source code was modified by this audit.

## Summary (counts by severity)

- Confirmed issues: 1 (high)
- Test gaps (confirmed missing coverage): 6
- Invariants verified as holding: 60+ (see below)

## Confirmed issues

### go-01 — Handler struct parameters are not converted from request maps

- Severity: high
- Invariant: M10 (per-param type validation / handler invocation), R4 (struct validation), and the generated-server contract in `docs/RUNTIME_IMPLEMENTATION_GUIDE.md` (server invokes the handler with typed parameters).
- Evidence:
  - `pkg/runtime/runtimes/go/pulserpc/server.go:540-601` (`convertMapToStruct`): field lookup tries only the Go field name (`ToRepeat`) and its lowercased first letter (`toRepeat`); it never consults the generated `json:"to_repeat"` tag, so any snake_case IDL field whose Go name is not a single capitalized word fails to match.
  - `pkg/runtime/runtimes/go/pulserpc/server.go:517-537` (slice branch of `convertValueToType`): elements are only copied when already assignable/convertible; a `[]interface{}` of maps is never converted into `[]SomeStruct`, leaving zero-valued structs.
  - `pkg/runtime/runtimes/go/pulserpc/server.go:504-550` (`convertValueToType`): there is no branch for `map[string]interface{}` → `map[string]T`, and no handling for pointer targets (optional structs).
- Probe results (removed after verification):
  - `convertMapToStruct({"to_repeat":"hi","count":3})` → `{ToRepeat:"", Count:3}` (multi-word field dropped).
  - `convertMapToStruct({"items":[{name:"x"}]})` → `Items:[{Name:""}]` (nested struct in slice dropped).
  - `convertMapToStruct({"m":{"a":{"name":"x"}}})` → `M:map[]` (map of structs dropped).
  - An optional struct parameter resolves to a nil pointer even when a value is supplied.
- Current behavior: request validation succeeds (params are maps), but the handler receives zero values for multi-word fields and for any struct contained in a slice/map, and nil for optional struct params.
- Expected: the handler receives the values present in the request, including nested collections and optional structs, matching the generated Go struct's JSON tags.
- Suggested fix: resolve each field key from its `json` struct tag (falling back to the lowercased field name); make the slice branch recurse through `convertValueToType`; add a map branch that converts keys/values; allocate pointer targets before converting.
- Test status: no unit test exercises struct-typed handler parameters through `Server.Call`; `server_call_test.go` only uses `int`/`string` params.
- Acceptance criteria:
  - [ ] `Server.Call` with a struct param whose field is `to_repeat` invokes the handler with that field populated.
  - [ ] Arrays and maps of structs, and optional struct params, are populated.
  - [ ] Existing `int`/`string` param tests still pass.

## Test gaps (confirmed missing coverage)

1. **Fractional `int` rejection is untested** (M11/R2): `validation.go:21-47` rejects fractional floats, but `tests/validation_test.go:19-32` only checks `123`, `123.0`, and `"123"`. No positive/negative fractional case.
2. **Struct-typed handler params are untested** (M10): no `Server.Call` test passes a struct (or slice/map of structs) through to a handler. This is exactly what hid go-01.
3. **Optional struct params are untested** (R4/M14): no test supplies a non-nil value for an optional `*Struct` parameter.
4. **HTTP integration does not assert JSON-RPC error codes** (M17): `tests/integration/test_http_api.sh:103-142` downgrades a code mismatch to `WARN` and still counts the case as passed; the Go generator integration therefore never asserts `-32600`/`-32601`/`-32602`.
5. **Batch requests are untested at HTTP level** (M7/M28): no integration case sends an array request, empty batch, or notification-only batch.
6. **`ValidateFloat` NaN/Infinity and wider numeric inputs are untested** (R2): no test pins whether non-finite doubles or `int64`/`float32` inputs are accepted.

## Verified as holding (with evidence)

- **R1** — `ValidateType` (`validation.go:202`), `FindStruct` (`types.go:98`), `FindEnum` (`types.go:107`), `GetStructFields` (`types.go:115`) are exported. `tests/types_test.go`.
- **R2** — scalar acceptance: `ValidateInt` enforces signed 32-bit and rejects bool/fractional/NaN/Inf/out-of-range (`validation.go:17-47`); `ValidateFloat` accepts `float64`/`int` (`validation.go:49-57`); `ValidateBool`/`ValidateString` are strict (`validation.go:59-65`, `validation.go:9-15`).
- **R3** — fail-fast: first validation error is returned (`validation.go:136,141,150`; `contract.go:226,249`).
- **R4** — missing required field → error, missing optional → OK, null required → error (`validation.go:158-199`; `server_call_test.go:134-163`).
- **R5** — parent fields first, child overrides by name (`types.go:122-169`; `tests/types_test.go:59-94`).
- **R6** — D16 resolution: exact key, exact base-name key, then first sorted base-name key (`typeindex.go:61-78`; `typeindex_test.go:26-59`).
- **R7** — `RPCError` carries `code`, `message`, `data` (`rpc.go:5-10`; `tests/rpc_test.go`).
- **R8** — standard literal codes `-32600..-32603` (`server.go:79-179`).
- **R9** — runtime client uses atomic IDs (`client.go:138`) and `RequestWithID` (`transport.go:23-25,148-213`); generated transport uses `atomic.AddUint64` (`go_client_server.go:1218`).
- **R10** — response validation runs when enabled, including null results (`server.go:169-176`; `server_call_test.go:268-288`).
- **R11/M4** — notifications produce no response, including errors (`server.go:61-195`; `server_call_test.go:77-99`).
- **R12** — Barrister `meta` items parsed (`contract.go:117-124`).
- **R13** — contract verification tests are now selected by `make test` (`Makefile:10-12` runs `./pkg/runtime/runtimes/go/...`).
- **R14** — void return rejects non-nil (`contract.go:270-288`; `server_call_test.go:278-288`).
- **R15/D4/S6** — internal errors are opaque: fixed message, no data, logged by type only (`server.go:12-16,63-71,159-164`; `server_call_test.go:176-266`).
- **R16/D5** — request body capped at 1 MiB (`go_client_server.go:956,970`).
- **R17** — response body capped at 10 MiB (`transport.go:107-124`; `go_client_server.go:1193,1259-1265`).
- **R18** — batch count capped at 100 (`go_client_server.go:957,994`).
- **R19/D6** — 30-second HTTP timeout, 10-second connect (`transport.go:139`; `go_client_server.go:1212`).
- **R20** — atomic request IDs in runtime and generated clients (`client.go:138`; `go_client_server.go:1218`; `client_concurrency_test.go`).
- **R21** — contract-owned, read-only type indexes (`contract.go:27-31,129-132`; `typeindex.go`).
- **R22/D3** — non-POST → 405 with JSON-RPC error body (`go_client_server.go:962-967`).
- **R23** — notifications → 204 / no body (`go_client_server.go:1012-1014,1035-1038`).
- **R24/D2** — id type validation and null-id notifications (`server.go:54-59,61`; `server_call_test.go:332-365`).
- **R25** — int is signed 32-bit on the wire (`validation.go:17-47`).
- **R26** — float is `float64` (`validation.go:49-57`; generated `Ratio float64`).
- **R27** — enums are wire strings (`go_client_server.go:714`).
- **R28/S5** — Go uses maps, not prototype chains (N/A by construction).
- **R29/R30/D13** — custom headers and trailing-slash normalization (`transport.go:129-131,137,176-178`; `go_client_server.go:1210,1249-1251`).
- **R31** — verification fetches `pulserpc-idl` and treats a failed fetch as incompatible (`go_client_server.go:1483-1517`).
- **R32** — `idl.json` carries comments and excludes them from the checksum (`idl_json.go:38-45`; `checksum.go`).
- **M1** — `jsonrpc != "2.0"` → `-32600` (`server.go:79-81`).
- **M2** — missing/empty/non-string `method` → `-32600` (`server.go:84-87`).
- **M3** — `Interface.method` parse and lookup → `-32601` (`server.go:95-115`).
- **M5** — response-envelope validation (version, id match, result XOR error, error shape) (`transport.go:64-99`; `transport_envelope_test.go`).
- **M6** — generated Go adapter emits `-32700` on read/JSON errors (`go_client_server.go:978,984`).
- **M7** — batch requests supported in the generated adapter (`go_client_server.go:988-1045`).
- **M8** — array and object params accepted, others rejected (`server.go:123-137`; `server_call_test.go:101-132`).
- **M9** — exact parameter count enforced (`contract.go:225-229`; `server.go:372-375`).
- **M10** — per-param IDL type validation wired (`contract.go:231-253`; `server.go:139-144`), apart from go-01's conversion gap.
- **M11** — enum membership and recursive array/map/struct validation (`validation.go:102-116,246-299`).
- **M12** — unknown/extra fields intentionally ignored (`validation.go:158-199`).
- **M13** — validation `data` is the documented plain string for Go (`validation_data_shape_test.go`).
- **M14** — response validation runs for null results and generated client flags gate validation (`server.go:169-176`; `client.go:175-182`; `go_client_server.go:1425-1455`).
- **M15** — generated client fetches the server IDL and compares against it (`go_client_server.go:1483-1536`).
- **M16** — removed field classified as Info per the amended spec (`severity.go:17-18`; `severity_spec_test.go`).
- **M18** — empty batch → single `-32600` (`go_client_server.go:990-992`).
- **M19** — non-object batch entries → per-entry `-32600`, array shape retained (`go_client_server.go:999-1005`).
- **M20** — object-form params normalized by IDL name (`server.go:308-351`).
- **M21** — `id: null` is a notification (`server.go:61`; `server_call_test.go:355-365`).
- **M22** — `error.data` omitted when nil (`server.go:199-206`; `server_call_test.go:367-394`).
- **M23** — null result serialized as explicit `"result": null` (`server.go:233-242`; `server_call_test.go:396-421`).
- **M24** — number-type-tolerant id comparison (`transport.go:30-59`; `server_call_test.go:423-439`).
- **M25** — error objects have numeric code and string message (`server_call_test.go:367-394`).
- **M26** — malformed JSON → `-32700` (`go_client_server.go:978,984`).
- **M27** — `jsonrpc: "2.0"` on requests and responses (`server.go:79-81,199-214`).
- **M28** — notifications in a batch are dropped; all-notification batch → no body (`go_client_server.go:1007-1014`).
- **C1/D12** — byte-deterministic generation (`go_invariants_test.go:223-255`).
- **C2/D8** — uniform IDL JSON schema including `rootNamespace` (`idl_json.go:38-45`; `go_invariants_test.go:90-147`).
- **C4** — `idl.json` and embedded `IDL_JSON` agree (`go_invariants_test.go:90-147`).
- **C5** — `IDL_JSON` is an object, not a double-encoded string (`go_invariants_test.go:119-125`).
- **C9** — required fields have no `omitempty`, optional fields do (`go_client_server.go:763-767`; `go_invariants_test.go:257-297`).
- **C11** — enums serialize as declared string names (`go_client_server.go:714,718`).
- **C12** — flag defaults (`go_client_server.go:31-45`).
- **C13** — generated code compiles (generator unit tests + `test_generator_go.sh`).
- **S1** — playground file access contained (`resolveSessionFile`; `TestGetFileRejectsTraversal`).
- **S2** — `embed.FS` paths use forward slashes (`runtime/embed.go`).
- **S3** — ZIP entry names use forward slashes (`playground/files.go:70-72`).
- **S4** — generated paths use correct separators (`go_client_server.go` uses `filepath`/`path`).
- **S6** — internal error details redacted (see R15).
- **S7** — body/response/batch limits enforced (see R16-R18).
- **S8** — non-POST rejected consistently (see R22).
- **G20** — 43-character unpadded base64url checksum (`checksum.go`; `go_invariants_test.go:141-146`).
- **G21** — Go generator sets `idl.Checksum` (`idl_json.go:15-19`).
- **G29-G32** — trailing import comments, dedup by resolved path, root/import namespace collision (`parser.go`).
- **G34** — checksum excludes error message text and comments (`checksum.go:147-187`).
- **G35** — checksum includes parameter position (`checksum.go`).

## Unconfirmed hypotheses

- **ValidateFloat accepts NaN and Infinity** (`validation.go:49-57` has no finiteness check). The catalog lists TS float accepting NaN/Infinity as a divergence, but it is not settled whether the Go runtime should reject them; the guide defines `float` only as IEEE-754 double. Left as a hypothesis, not an issue.
- **Wider numeric inputs to validation functions** (`ValidateInt`/`ValidateFloat` do not accept `int64`, `float32`, or `json.Number` in all paths). JSON-decoded values are `float64`/`int`, so this is not observed in normal use; whether direct callers need these is unconfirmed.
- **Runtime client verification does not re-fetch** (`client.go:335-378` uses `c.contract.idlParsed` as the server IDL). This is consistent with bootstrap having already fetched it, but a runtime-only user cannot force a fresh fetch; whether that is required is unconfirmed.
- **Generated client defaults validation off** (`go_client_server.go:1324-1325`). This matches the runtime client defaults, but whether generated clients should default validation on is a product decision, not a confirmed invariant violation.

## Notes

- The prior `docs/audit/problems-go.md` is a stale baseline: all of its `go-01`..`go-15` items that were code defects (notification suppression, distinct IDs, object-form params, null-arg panics, inert validation flags, null-result validation, self-comparison verification, severity classification, internal-error redaction) are resolved in the current code and are covered by the tests listed above. The remaining prior items (WARN-only integration codes, untested batches) are recorded here as test gaps.
- `go test ./pkg/runtime/runtimes/go/... -count=1` and `go test ./pkg/generator/ -run Go` both pass on this audit.
- The Go integration test (`tests/integration/test_generator_go.sh`) was not executed in this audit because the web UI build is unavailable in this environment; generator behavior was verified through the unit tests in `pkg/generator/*_test.go`.
- go-01 is the only confirmed code defect found; it is high severity because it silently zeroes struct fields in every generated Go server that uses struct parameters.
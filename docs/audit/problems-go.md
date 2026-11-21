# Go — audit findings

## Summary
- Invariants checked: 32 (G21, R1–R14, M1–M17)
- Confirmed violations: 12 (go-01…go-10, go-12, go-13)
- Test gaps: 3 (go-11, go-14, go-15)

Note: `make test` in `pkg/runtime/runtimes/go/Makefile:8-15` runs only
`cd tests && go test -v` (Docker path at `:18-22` does the same). Root
`Makefile:41-49` runs only `./pkg/runtime/` (embed tests). Therefore every
test in `pkg/runtime/runtimes/go/pulserpc/*_test.go` is dead unless invoked
manually, and "tested" below means "reachable from a make target" only when
stated.

## Problems

### go-01 — Error replies are sent for notifications
- Severity: high
- Invariant: R11 / M4
- Current behavior: `Server.Call` computes `isNotification` at `pkg/runtime/runtimes/go/pulserpc/server.go:56-58`, but every error return at `server.go:42,48,63,69,79,97,104,115,117,123` happens *before* the suppression check at `server.go:128-130`. The `pulserpc-idl` special case at `server.go:51-54` also replies without an `id`. The generated HTTP adapter writes any non-nil response with HTTP 200 (`pkg/generator/go_client_server.go:954-963`), so a failed notification gets a JSON body.
- Expected behavior: JSON-RPC 2.0 notifications must never receive a response, including errors (`docs/RUNTIME_IMPLEMENTATION_GUIDE.md:810-815`); the catalog itself claims Go suppresses correctly (R11/M4).
- Test status: none. `tests/integration/test_http_api.sh:155-243` has no notification case; grep across `tests/integration/*.sh` finds no notification test.
- Suggested fix: move the `isNotification` determination to the top of `Server.Call` and return `nil` from every error path (and from the `pulserpc-idl` path) when `reqID == nil`.
- Acceptance criteria:
  - [ ] A request with no `id` and an unknown method returns `nil` from `Server.Call` and the generated HTTP handler returns 204.
  - [ ] A batch containing only failing notifications returns 204 / empty body.

### go-02 — Go client cannot emit notifications (always sends an id)
- Severity: medium
- Invariant: M4 / R9
- Current behavior: `Client.Notify` (`pkg/runtime/runtimes/go/pulserpc/client.go:184-188`) just calls `Client.Call`, which allocates `c.requestID` at `client.go:134-136`; both HTTP and in-proc transports hardcode `"id": 1` (`transport.go:34-41`, `transport.go:100-105`). The generated `HTTPTransport.Call` likewise always sets `"id"` (`pkg/generator/go_client_server.go:1117-1124`).
- Expected behavior: a notification is a request without `id`, and must be sent without one (`docs/RUNTIME_IMPLEMENTATION_GUIDE.md:810-815`).
- Test status: none. `pkg/runtime/runtimes/go/tests/rpc_test.go:9-38` only tests `RPCError`.
- Suggested fix: add a `Request` path that omits `id` when the caller requests a notification, and have `Notify` use it; thread an explicit id through the generated transport instead of hardcoding one.
- Acceptance criteria:
  - [ ] `Client.Notify` produces a JSON-RPC object with no `id` key (assert via a capturing `Transport`).
  - [ ] Generated `HTTPTransport` has an id-less call path used by `Notify`.

### go-03 — Request IDs are dead/duplicated and never validated on responses
- Severity: high
- Invariant: R9 / M5
- Current behavior: `Client.Call` builds an `id` from `c.requestID` into a local `req` map at `pkg/runtime/runtimes/go/pulserpc/client.go:134-146`, then discards it and calls `c.transport.Request(method, params)` at `client.go:149`. Both transports use constant `id: 1` (`transport.go:40`, `transport.go:104`). The generated transport uses `fmt.Sprintf("%d", len(method)+len(params))` as a string id (`pkg/generator/go_client_server.go:1118-1123`) — colliding for any two calls with equal lengths. No client checks `response["id"]` against the sent id (`client.go:154-181`; grep confirms no read of `response["id"]`).
- Expected behavior: response envelope must carry the matching request id, and result XOR error (`docs/RUNTIME_IMPLEMENTATION_GUIDE.md:810-812`; catalog M5).
- Test status: none. No unit test covers `Client.Call`/transport; `tests/integration/test_generator_go.sh:121-134` only checks process exit codes.
- Suggested fix: pass the generated id into the transport, use it in the serialized request, and reject responses whose `id` does not match or that contain both `result` and `error`.
- Acceptance criteria:
  - [ ] Two sequential `Client.Call`s send distinct ids (asserted on a capturing transport).
  - [ ] A response with a mismatched id or with both `result` and `error` returns an error.

### go-04 — Object-form `params` are ignored / validation skipped
- Severity: medium
- Invariant: M8
- Current behavior: `Server.Call` only accepts `[]interface{}` params; a JSON object leaves `positionalParams` nil (`pkg/runtime/runtimes/go/pulserpc/server.go:83-92`). Validation at `server.go:95-99` is then skipped because `len(positionalParams) > 0` is false, and with no contract the handler is invoked with zero values (`server.go:233-242`, `server.go:344-347`).
- Expected behavior: JSON-RPC 2.0 permits structured (object) params by name (`docs/RUNTIME_IMPLEMENTATION_GUIDE.md:811`); per-param IDL validation must not be silently bypassed.
- Test status: none. `tests/integration/test_http_api.sh:41-46` only ever emits array params (or omits them).
- Suggested fix: build named params from object-form `params` and run `ValidateRequest` in both forms; reject unknown param names.
- Acceptance criteria:
  - [ ] A request with `params: {"a":1,"b":2}` for `A.add` invokes the handler with both arguments.
  - [ ] A request with object-form params that omit a required parameter returns `-32602`.

### go-05 — Null argument panics the server via reflection (new finding)
- Severity: high
- Invariant: M10 / R4 (new)
- Current behavior: `ValidateRequest` passes `isOptional=true` through to `ValidateType` (`pkg/runtime/runtimes/go/pulserpc/contract.go:207-212`), and `ValidateType` explicitly accepts nil for optional types (`validation.go:169-173`). In `invokeHandler`, a present param whose value is nil is passed straight to `s.convertValueToType(reflect.ValueOf(v), paramType)` (`server.go:325-327`), which calls `value.Type()` on the zero `reflect.Value` (`server.go:374-378`) → `panic: reflect: call of reflect.Value.Type on zero Value`. A `net/http` handler therefore dies instead of returning JSON-RPC.
- Expected behavior: an optional parameter set to `null` is valid input and must be delivered to the handler as a zero value/nil; no handler invocation may panic on request data.
- Test status: none. `pkg/runtime/runtimes/go/tests/validation_test.go:205-208` tests optional nil only at the `ValidateType` level, never through `Server.Call`.
- Suggested fix: in `invokeHandler`, when `v == nil`, set `argValue = reflect.Zero(paramType)` and mark found; also recover panics in `Server.Call` and convert them to `-32603`.
- Acceptance criteria:
  - [ ] `Server.Call` with an optional param explicitly `null` invokes the handler and returns a normal result (no panic).
  - [ ] With `validateRequests=false`, a `null` param for a required non-pointer type does not panic.

### go-06 — `ValidateInt` accepts near-integers asymmetrically; fractional rejection untested
- Severity: medium
- Invariant: R2 / M11
- Current behavior: `pkg/runtime/runtimes/go/pulserpc/validation.go:18-29` accepts a `float64` when `math.Abs(v-math.Floor(v)) < 1e-9`. Because `math.Floor` rounds toward `-∞`, `1.0000000001` is accepted while `-1.0000000001` is rejected (`Floor(-1.0000000001) = -2`, difference ≈ 1). The catalog flags this 1e-9 tolerance as the Go divergence in R2.
- Expected behavior: scalar acceptance should match sibling runtimes — reject floats with a fractional part (`invariants.md:57`; M11 at `:85`).
- Test status: `pkg/runtime/runtimes/go/tests/validation_test.go:19-32` tests only `123`, `123.0`, `"123"`; it never passes a fractional float, so the tolerance and the asymmetry are invisible.
- Suggested fix: drop the tolerance (or apply a symmetric `math.Round` check consistently for both signs) and add positive and negative fractional cases.
- Acceptance criteria:
  - [ ] `ValidateInt(1.5)` and `ValidateInt(-1.5)` both return errors.
  - [ ] Positive and negative near-integer floats behave symmetrically and match the documented rule.

### go-07 — Qualified type-name resolution is ambiguous and untested
- Severity: low
- Invariant: R6
- Current behavior: `FindStruct` strips everything before the last `.` and looks up only the bare base name (`types.go:97-117`); `FindEnum` falls back to a suffix scan over the whole map (`types.go:135-140`). With two namespaces defining the same base name, e.g. `a.Status` and `b.Status`, `FindEnum("x.Status")` returns whichever key Go map iteration yields first — nondeterministic.
- Expected behavior: deterministic, namespace-aware resolution (R6 at `invariants.md:61`).
- Test status: `pkg/runtime/runtimes/go/tests/types_test.go:9-30` tests only `FindStruct("TestStruct")` (no qualified case); `:47-51` tests `FindEnum("ns.TestEnum")` only with a single candidate. No ambiguity/duplicate-base-name test.
- Suggested fix: resolve exact qualified keys only, or define an explicit precedence rule and return an error on ambiguity; add deterministic multi-namespace tests.
- Acceptance criteria:
  - [ ] Given `a.Status` and `b.Status`, resolving a third namespace's `x.Status` is either rejected or deterministic across 100 iterations.
  - [ ] `FindStruct` resolves a qualified name against a qualified map key.

### go-08 — Generated client validation flags are inert
- Severity: high
- Invariant: M14
- Current behavior: the generator emits `validateRequests`/`validateResponses` fields and setters (`pkg/generator/go_client_server.go:1182-1183,1192-1193,1205-1211`) and `WithValidateRequests`/`WithValidateResponses` options (`:1061-1072`), but the generated method bodies (`:1230-1311`) and generated transport (`:1096-1165`) never read them and never call `Contract.ValidateRequest`/`ValidateResponse`.
- Expected behavior: flags must actually enable IDL validation (`docs/RUNTIME_IMPLEMENTATION_GUIDE.md:820`).
- Test status: none. The generated test client (`go_client_server.go:1593-1609`) constructs clients with no options, and only `add`/`sqrt`/`echo` have value assertions (`:1657-1682`).
- Suggested fix: emit validation calls in each generated method before the transport call and on the extracted result, gated on the two flags.
- Acceptance criteria:
  - [ ] With `WithValidateRequests(true)`, a generated client rejects a wrong-typed argument locally.
  - [ ] With `WithValidateResponses(true)`, a generated client rejects a wrong-typed result locally.

### go-09 — `result != nil` guards skip response validation for null results
- Severity: medium
- Invariant: M14 / R10
- Current behavior: `pkg/runtime/runtimes/go/pulserpc/server.go:120-125` and `client.go:171-179` gate `ValidateResponse` on `result != nil`. `Contract.ValidateResponse` would reject nil for a non-optional return (`contract.go:233-262`, via `validation.go:169-173`), but is never called. The generated server does enable response validation (`go_client_server.go:872-873`).
- Expected behavior: a null result for a required return must fail validation; validation should run for all non-void returns (M14 at `invariants.md:88`).
- Test status: none. No test constructs a handler returning nil for a required return.
- Suggested fix: remove the `result != nil` guards and let `ValidateResponse` decide, passing the function's return-type optionality.
- Acceptance criteria:
  - [ ] A server handler returning `nil` for a required return produces `-32603` when validation is enabled.
  - [ ] A client receiving `result: null` for a required return returns a validation error.

### go-10 — Generated `VerifyCompatibility` compares the IDL against itself
- Severity: high
- Invariant: M15
- Current behavior: `pkg/generator/go_client_server.go:1315-1325` sets `clientIDL` from `c.contract.GetIDLParsed()` whenever `localIDL == nil`, and always sets `serverIDL` from the same `c.contract.GetIDLParsed()`. No `SetLocalIDL` setter is generated for the `localIDL` field (`:1181`, grep finds no other reference), so both sides are the identical object → `DiffIDL` returns zero deltas and `Compatible=true` unconditionally. Additionally the generated `VerificationResult` (`:1342-1347`) omits the spec-required `Timestamp` (`openspec/specs/contract-compatibility-verification/spec.md:38-45`), which the runtime client does set (`client.go:351-357`).
- Expected behavior: compare a client-supplied local IDL against the server's IDL fetched over RPC (M15 at `invariants.md:89`).
- Test status: none that runs. `pkg/runtime/runtimes/go/pulserpc/integration_test.go:9-318` calls `DiffIDL` directly, never `VerifyCompatibility`, and is excluded from `make test` (see go-11).
- Suggested fix: generate a `SetLocalIDL` setter and fetch the server IDL via `pulserpc-idl` during bootstrap; populate `Timestamp: time.Now()` (and import `time`).
- Acceptance criteria:
  - [ ] A generated client with a deliberately divergent local IDL reports incompatible with non-zero deltas.
  - [ ] The returned `VerificationResult.Timestamp` is non-zero.

### go-11 — Contract-verification and auditor tests never run; no Go generator unit tests
- Severity: high (test gap)
- Invariant: R13
- Current behavior: `pkg/runtime/runtimes/go/Makefile:8-22` runs only `cd tests && go test -v`; that directory contains only `validation_test.go`, `types_test.go`, `rpc_test.go`. The contract/diff/auditor/verification tests in `pkg/runtime/runtimes/go/pulserpc/auditor_test.go:9-139` and `integration_test.go:9-318` are never selected by any make target. Root `Makefile:41-49` tests only `./pkg/runtime/` (embed tests), and `test-runtime-go` (`:136-138`) delegates to the Go Makefile. There is also no `*_test.go` for `pkg/generator/go_client_server.go`, while sibling generators have e.g. `pkg/generator/ts_client_server_test.go` and `python_client_server_test.go`.
- Expected behavior: R13 (`invariants.md:68`) requires contract verification to be covered by tests; generated-output behavior needs generator unit tests.
- Test status: test files exist but are excluded (exact lines above); no generator-level test exists.
- Suggested fix: extend the Go runtime Makefile `test`/`test-docker` to also run the `pulserpc` package tests, and add `go_client_server_test.go` alongside the sibling generator tests.
- Acceptance criteria:
  - [ ] `make -C pkg/runtime/runtimes/go test` executes `TestFailFastAuditor_PanicsOnIncompatible` and `TestVerifyCompatibility_*`.
  - [ ] A `pkg/generator/go_client_server_test.go` exists and asserts generated checksum/validation/verification code.

### go-12 — Validation error `data` is a plain string, not `{field,message}`
- Severity: medium
- Invariant: M13
- Current behavior: all producers use `fmt.Errorf` strings, e.g. `validation.go:136,141,150-151,213` and `contract.go:190,213`; the server forwards `err.Error()` as `data` at `server.go:97,104,123`. No error object with `field`/`message` keys is built anywhere in the runtime.
- Expected behavior: documented error `data` shape is `{"field":"email","message":"..."}` (`docs/idl-guide/validation.md:233-245`).
- Test status: `tests/integration/test_http_api.sh:75-143` checks only presence of `"error"` and the code (which is itself WARN-only, see go-14); no assertion on `data`.
- Suggested fix: introduce a structured validation error type that carries field path and message, and emit it as `data` from `Server.Call`.
- Acceptance criteria:
  - [ ] A missing required field yields `error.data.field` and `error.data.message`.
  - [ ] A unit test asserts the shape for both param and struct-field failures.

### go-13 — Breaking "field removed" delta is classified Info (spec contradiction)
- Severity: medium
- Invariant: M16
- Current behavior: `pkg/runtime/runtimes/go/pulserpc/severity.go:17-18` returns `SeverityInfo` for `EntityField`/`ChangeRemoved`/`DirectionClientHasMore`, so `VerifyCompatibility` reports `Compatible=true` after a required field is removed. The test `pkg/runtime/runtimes/go/pulserpc/integration_test.go:133-172` asserts this Info result (but does not run, go-11).
- Expected behavior: the scenario at `openspec/specs/contract-compatibility-verification/spec.md:115-118` requires `Compatible=false` and an Error-level entry when the server removes a required field; the spec's own table at `:25` says Info, so the spec is internally contradictory and needs resolution.
- Test status: `integration_test.go:165-167` asserts the current (Info) behavior and is excluded from `make test`.
- Suggested fix: resolve the spec conflict; if the scenario is authoritative, distinguish removal of a *required* field (Error) from an optional one (Info) using the field's `optional` flag and update `ClassifySeverity` and the test.
- Acceptance criteria:
  - [ ] Removing a required field yields `SeverityError` and `Compatible=false`.
  - [ ] Removing an optional field still yields `SeverityInfo`.

### go-14 — Integration suite WARNs on wrong JSON-RPC error codes instead of failing
- Severity: medium (test gap)
- Invariant: M17 / M1 / M9
- Current behavior: in `tests/integration/test_http_api.sh:103-138`, a mismatched error code only prints `⚠ WARN` and then falls through to `✓ PASS` / `TESTS_PASSED++` (`:140-142`). The Go generator integration script invokes this suite at `tests/integration/test_generator_go.sh:136-154`, so Go's `-32600/-32601/-32602` behavior is never actually asserted, even when `make test-generator-go` passes.
- Expected behavior: M17 (`invariants.md:91`) flags this as a violation; error codes are part of the JSON-RPC contract (`docs/RUNTIME_IMPLEMENTATION_GUIDE.md:815`).
- Test status: the WARN-only helper at `test_http_api.sh:75-143` is used for all validation cases at `:208-241`.
- Suggested fix: increment `TESTS_FAILED` and add to `FAILED_TESTS` on a code mismatch (remove the WARN-only branch), then re-run Go generator integration.
- Acceptance criteria:
  - [ ] Injecting a wrong code makes `test_http_api.sh` exit 1.
  - [ ] Go server responses for the cases at `test_http_api.sh:211-241` assert `-32601`/`-32602` exactly.

### go-15 — Batch requests are implemented but never tested
- Severity: low (test gap)
- Invariant: M7 / R11
- Current behavior: the generated Go HTTP adapter implements batching at `pkg/generator/go_client_server.go:930-952` (including empty-batch `-32600` and 204 for all-notification batches), and the guide requires it (`docs/RUNTIME_IMPLEMENTATION_GUIDE.md:813`). No test sends an array request: `tests/integration/test_http_api.sh:155-243` uses only single objects, and grep finds no `batch` assertion in `tests/integration/*.sh`.
- Expected behavior: M7 (`invariants.md:81`) marks batch support as required-but-untested.
- Test status: none.
- Suggested fix: add cases to `test_http_api.sh` for a mixed batch, an empty batch (`-32600`), and a notifications-only batch (204/no body).
- Acceptance criteria:
  - [ ] A mixed batch returns an array with one response per non-notification request.
  - [ ] An empty batch returns `-32600` and a notifications-only batch returns no body.

## Verified-correct invariants
- G21 — catalog false positive: generator computes checksum at `pkg/generator/go_client_server.go:55` and writes `"checksum"` into idl.json at `:65`.
- R1 — `ValidateType` (`validation.go:161`), `FindStruct` (`types.go:97`), `FindEnum` (`types.go:121`), `GetStructFields` (`types.go:146`) all exported.
- R3 — Go is fail-fast: first validation error returned, e.g. `validation.go:136,141,150`.
- R4 — missing/null required fields error, optional skip (`validation.go:133-154`); asserted at `tests/validation_test.go:163-177`.
- R5 — parent fields resolved before child, child overrides by name (`types.go:154-188`); asserted at `tests/types_test.go:59-94`.
- R7 — `RPCError` carries code/message/data (`rpc.go:5-10`); asserted at `tests/rpc_test.go:9-38`.
- R8 — Go uses literal standard codes (`server.go:42,48,63,97,117`); same as catalog, no named constants (cross-runtime divergence, not a Go-only defect).
- R10 — runtime server does validate responses when enabled (`server.go:120-125`), generated server enables it (`go_client_server.go:872-873`); null-result hole is go-09.
- R12 — Go parses barrister-format `meta` items (`contract.go:112-118`), matching catalog.
- R14 — void return rejects non-nil results (`contract.go:233-242`).
- M1 — `jsonrpc != "2.0"` → `-32600` (`server.go:41-43`).
- M2 — missing/empty/non-string `method` → `-32600` (`server.go:45-49`); Java NPE issue is not Go.
- M3 — `Interface.method` parse and lookup → `-32601` (`server.go:60-79`).
- M6 — generated Go HTTP adapter emits `-32700` on read/JSON errors (`go_client_server.go:920,926`, test server `:1396`).
- M9 — exact param count enforced for contracted methods at `server.go:250-253` regardless of `validateRequests`; the catalog's "skip at zero params" at `:95` is mitigated for Go (count check still runs), though type validation is skipped.
- M10 — per-param IDL type validation exists (`contract.go:170-219`), wired at `server.go:95-99`; null-argument hole is go-05.
- M11 — enum membership (`validation.go:84-98`) and recursive array/map/struct validation (`validation.go:192-213`) exist; fractional gap is go-06.
- M12 — unknown/extra fields are intentionally ignored (`validation.go:117-155` iterates only known fields), as documented.

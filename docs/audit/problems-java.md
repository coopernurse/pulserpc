# Java — audit findings

## Summary
- Invariants checked: 31 (R1–R14, M1–M17 from `docs/audit/invariants.md:54-91`)
- Confirmed violations: 17 (17 problems documented below, including 13 with explicit test gaps)
- Test gaps: 13 (called out per problem)
- Files read: Java runtime source + 8 test files, `pkg/generator/java_client_server.go`,
  `pkg/runtime/runtimes/java/Makefile`, root `Makefile`, `tests/integration/*`,
  `docs/RUNTIME_IMPLEMENTATION_GUIDE.md`, `docs/idl-guide/validation.md`,
  `openspec/specs/contract-compatibility-verification/spec.md`.

## Problems

### java-01 — Generated server error paths throw NPE: `Map.of(..., null)` (NEW)
- Severity: critical
- Invariant: M1 / M2 / M4 / R11 (JSON-RPC error responses must be well-formed)
- Current behavior: `pkg/generator/java_client_server.go:997-1012` builds the error
  response with `Map.of("jsonrpc","2.0","error",Map.of(...),"id", null)`. Java's
  `Map.of` rejects null values and throws `NullPointerException` on every call.
  `sendError` is invoked at `:975` (non-POST) and `:992` (any exception, including
  malformed JSON), so those paths return an empty/500 response instead of a JSON-RPC error.
  The same defect affects every error return in `handleJsonRpcRequest` that uses
  `Map.of(..., "id", id)` — e.g. `:1020-1027`, `:1055-1062`, `:1070-1076`,
  `:1110-1117`, `:1120-1127`, `:1139-1146`, `:1161-1169`, `:1178-1185`,
  `:1189-1197`, `:1202-1209` — whenever `id` is absent (notification).
- Expected behavior: `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:810-815` requires valid
  JSON-RPC error envelopes for invalid requests/parse errors. The success path already
  uses `new HashMap<>()` + `put` (`:1150-1155`) precisely because it permits null ids.
- Test status: `tests/integration/test_http_api.sh:222-241` exercises invalid
  requests, but every request carries `"id":1` so the `Map.of(..., id)` path does not
  throw; non-POST and malformed-JSON requests are never sent. No test covers a
  notification that also produces an error. `test_generator_java.sh:184` is the only
  Java invocation of that script.
- Suggested fix: replace all `Map.of(...)` error constructions with a `HashMap` (or a
  helper that omits the `id` key when null), and add a `sendError` unit test.
- Acceptance criteria:
  - [ ] Non-POST request returns HTTP 200 with a JSON-RPC `-32600` error object.
  - [ ] A notification with an invalid/missing `method` does not throw and produces no 500.
  - [ ] Malformed JSON produces a JSON-RPC error (see java-03) rather than an empty 500.

### java-02 — Generated Java server always responds to notifications
- Severity: high
- Invariant: R11 / M4
- Current behavior: `pkg/generator/java_client_server.go:1150-1155` always puts an
  `"id"` key (null when absent) into the success response and returns it; `:983-990`
  unconditionally serializes and sends a response. There is no `isNotification` check
  anywhere in the generated server. Every error return at `:1020-1209` also emits an
  `"id"` key. The hand-written runtime does it correctly:
  `pkg/runtime/runtimes/java/com/bitmechanic/pulserpc/Server.java:61-63,134-137`.
- Expected behavior: `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:814` "Notifications:
  Requests without `id` field (no response sent)"; checklist `:1213`.
- Test status: none. `tests/integration/test_http_api.sh` never sends a request
  without `id`; no Java unit test constructs a `Server`/generated handler.
- Suggested fix: detect absence of `id` in `handleJsonRpcRequest`, return `null`, and
  skip `sendResponseHeaders`/body write in `handleRequest`.
- Acceptance criteria:
  - [ ] POST with `{"jsonrpc":"2.0","method":"A.say_hi"}` (no id) yields an empty HTTP body.
  - [ ] The handler still executes the method for notifications.

### java-03 — Generated Java server emits no JSON-RPC parse error (-32700)
- Severity: high
- Invariant: M6
- Current behavior: `pkg/generator/java_client_server.go:978-981` parses the body with
  `jsonParser.fromJson(requestBody, Map.class)`; a parse failure is swallowed by the
  generic catch at `:991-993` which calls `sendError(exchange, -32603, ...)`
  (which itself NPEs — java-01). No `-32700` is produced anywhere in the file.
- Expected behavior: `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:815` lists -32700 among
  the standard codes; catalog `invariants.md:80` notes generated Go/Python/C#/TS have it.
- Test status: none — `tests/integration/test_http_api.sh:1-266` sends only valid JSON.
- Suggested fix: catch parse exceptions separately and return
  `{"jsonrpc":"2.0","error":{"code":-32700,...},"id":null}` using a null-safe map.
- Acceptance criteria:
  - [ ] Posting `not json` returns HTTP 200 with error code -32700.
  - [ ] The response envelope survives the sendError path (java-01 fixed).

### java-04 — Generated Java server/client skip IDL type validation entirely
- Severity: high
- Invariant: M10 / R10 / M14
- Current behavior: `pkg/generator/java_client_server.go:1129-1149` deserializes params
  solely via reflection generic types (`jsonParser.fromJson(paramJson, paramTypes[i])`);
  no `Validation.validateType`, `Contract.validateRequest`, enum-membership, required-field,
  or optional-field check is performed. The success path at `:1149-1155` does no response
  validation. Generated client `call()` at `:1329-1338` returns `result` with no
  validation. By contrast the hand-written runtime validates requests at
  `Server.java:96-103` and responses at `:124-132`; client at `Client.java:65-77,105-114`.
- Expected behavior: `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:820` "Runtime validation:
  Still required even with static types", checklist `:1208-1209`.
- Test status: `tests/integration/test_http_api.sh:210-241` only checks that *some*
  `-32602` appears; Jackson deserialization quietly produces that for a few wrong-type
  cases, so missing-field/optional/enum IDL rules are never exercised. No Java unit test
  calls `Contract.validateRequest`/`validateResponse`.
- Suggested fix: construct a `Contract` from `IDL_JSON` in the generated server/client
  and call `validateRequest`/`validateResponse` before/after invocation.
- Acceptance criteria:
  - [ ] A struct param missing a required field is rejected with -32602.
  - [ ] A response whose type violates the IDL return type yields an error, not a raw result.

### java-05 — Generated client verifies the embedded IDL against itself
- Severity: high
- Invariant: M15 / R13
- Current behavior: `pkg/generator/java_client_server.go:1286-1308`:
  `Object clientIDL = jsonParser.fromJson(IDL_JSON, Object.class);`
  `Object serverIDL = jsonParser.fromJson(IDL_JSON, Object.class);` (`:1287-1288`) —
  both sides are the same embedded constant, so `DiffEngine.diffIDL` (`:1289`) always
  returns zero deltas and `compatible=true`. The runtime client has the same default:
  `Client.java:197-212` uses `contract.getIdlParsed()` (the server IDL) for both sides
  unless `setLocalIDL` (`:248-251`) was called.
- Expected behavior: `openspec/specs/contract-compatibility-verification/spec.md:105-118`
  scenarios require real deltas (added optional field → Info; removed required field →
  Error/incompatible).
- Test status: `tests/SeverityTest.java`, `tests/DiffEngineTest.java`,
  `tests/AuditorTest.java` exist and are correct in isolation, but are **excluded** from
  the Java Makefile test target (`pkg/runtime/runtimes/java/Makefile:7`). No test calls
  `verifyCompatibility()` against a divergent server.
- Suggested fix: fetch the server contract via the `pulserpc-idl` RPC (or use the
  already-bootstrapped `contract`) and diff it against the client's own local IDL constant.
- Acceptance criteria:
  - [ ] `verifyCompatibility()` returns `compatible=false` when the server removes a required field.
  - [ ] A divergence produces at least one non-Info delta in the result.
  - [ ] The diff/auditor test suites run under `make test-runtime-java`.

### java-06 — `Types.getStructFields` does not implement child-override dedup
- Severity: medium
- Invariant: R5
- Current behavior: `pkg/runtime/runtimes/java/com/bitmechanic/pulserpc/Types.java:35-55`
  walks the inheritance chain and inserts each parent/child field with
  `allFields.add(0, field)` (`:48`) with no name-based merge. When a child redeclares a
  parent field both definitions remain in the list, so the first (parent) one wins during
  `Validation.validateStruct` (`Validation.java:135-158`). Sibling runtimes do it
  correctly: `pkg/runtime/runtimes/python/pulserpc/rpctypes.py:79-90`,
  `pkg/runtime/runtimes/ts-node/pulserpc/types.ts:97-105`.
- Expected behavior: `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:879-883` "Field override:
  Handle field name conflicts (child overrides parent)".
- Test status: `tests/TypesTest.java:51-82` tests only two distinct fields; it contains
  no override case, unlike `python/tests/test_types.py:59` and
  `ts-node/pulserpc/tests/test_shared.ts:131`.
- Suggested fix: build a name→field map while walking parent→child, overwriting on
  conflict, then emit values in parent-first order.
- Acceptance criteria:
  - [ ] `getStructFields` returns exactly one field for a name redeclared by the child.
  - [ ] The returned definition is the child's (assert its type/optional flag).

### java-07 — Type lookups are exact-name only; qualified names not resolved
- Severity: medium
- Invariant: R6
- Current behavior: `Types.java:16-18` `findStruct` and `:23-25` `findEnum` are plain
  `map.get(name)`. `Validation.java:208-218` uses those exact lookups for `userDefined`
  types. No suffix scan or base-name fallback exists, unlike Go / Python / Python2 / C#.
- Expected behavior: `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:856-860` "Qualified names:
  Handle qualified type names (e.g. `inc.Response`)". Generated namespaces do emit
  qualified names (`pkg/generator/java_client_server.go:804,821` put `s.Name`/`e.Name`
  which the parser prefixes with `inc.` at `pkg/parser/parser.go:650,660`).
- Test status: `tests/TypesTest.java:20,42` uses only exact simple names; no namespace
  or qualified-name test exists in any Java test.
- Suggested fix: in `findStruct`/`findEnum`, try exact match, then suffix match on
  `.` + requested name, then base-name match; make lookups deterministic on ties.
- Acceptance criteria:
  - [ ] `findStruct("Response", map)` resolves a map keyed `"inc.Response"`.
  - [ ] `Validation.validateType` validates a field whose `userDefined` is qualified
        against an unqualified map key (and vice versa).

### java-08 — Divergent scalar acceptance: `validateFloat` rejects `Long`; no float test
- Severity: medium
- Invariant: R2 / M11
- Current behavior: `Validation.java:50-54` accepts only `Float`, `Double`, `Integer`
  and rejects `Long`; `:37-43` `validateInt` *accepts* `Long`. Other runtimes accept
  integer values wherever a float is expected (Go, Python, TS). No test exercises
  `validateFloat` at all.
- Expected behavior: catalog `invariants.md:57` requires identical scalar sets;
  `docs/idl-guide/validation.md:8` promises automatic validation of IDL scalars.
- Test status: `tests/ValidationTest.java:30-42` covers only `validateInt`; there is no
  `testValidateFloat`, no whole-number-double, no `Long`, no NaN/Infinity case.
- Suggested fix: accept any `Number` (explicitly `Long`) in `validateFloat`, then add
  a `testValidateFloat` covering Integer/Long/Double/Float and rejection of strings/bool.
- Acceptance criteria:
  - [ ] `Validation.validateFloat(5L)` succeeds and `validateFloat(5)` succeeds.
  - [ ] `Validation.validateFloat("5")` throws `IllegalArgumentException`.

### java-09 — Runtime server skips validation at zero params and drops object-form params
- Severity: medium
- Invariant: M9 / M8
- Current behavior: `Server.java:97` guards validation with
  `if (validateRequests && !positionalParams.isEmpty())`, so a method declared with
  parameters but invoked with none never reaches `Contract.validateRequest`
  (`Contract.java:166-169` would otherwise reject the count). Separately `:86-94`
  converts params only when they are a `List` or null; an object/Map-form `params`
  is silently discarded (becomes an empty list), unlike TS which rejects it.
- Expected behavior: `invariants.md:83,82` — exact parameter count always checked;
  params shape must be validated consistently.
- Test status: no Java `Server` unit test exists (`tests/` contains no ServerTest).
  `test_http_api.sh:211,241` sends non-empty arrays only.
- Suggested fix: always call `validateRequest` when `validateRequests` is true; reject
  non-array/non-null `params` with `-32602` instead of dropping them.
- Acceptance criteria:
  - [ ] Calling a one-arg method with `params: []` returns -32602.
  - [ ] Calling with `params: {"a":1}` returns -32602 rather than invoking the handler.

### java-10 — Null-result guard bypasses response validation
- Severity: medium
- Invariant: M14 / R10
- Current behavior: `Server.java:125` `if (validateResponses && result != null)` and
  `Client.java:105` `if (validateResponse && contract != null && result != null)`.
  `Contract.validateResponse` correctly rejects null for non-optional returns
  (`Contract.java:206-225`), but the guard means a handler returning null for a
  required return type is never checked. Generated server never validates at all (java-04).
- Expected behavior: `invariants.md:88` flags this `result != nil` guard as a violation;
  `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:889` "Validate return values match IDL definition".
- Test status: none — no test enables `validateResponses` and returns null from a
  non-optional method.
- Suggested fix: remove the `result != null` guard and let `validateResponse` decide
  based on `returnOptional`.
- Acceptance criteria:
  - [ ] A handler returning null for a non-optional return type produces an error when validation is on.
  - [ ] A null return for an `[optional]` return type still succeeds.

### java-11 — Validation error `data` is a string, not the documented `{field,message}`
- Severity: medium
- Invariant: M13
- Current behavior: `Validation.java:19,44,52,62,82,100,110,115,142,148,154,171,193,236,240`
  throw `IllegalArgumentException` with plain-text messages; `Contract.java:186-188,223-225`
  wraps them into another string; `Server.java:100-101` passes that string as `data`
  via `errorResponse` (`:145-159`). No runtime produces a structured object.
- Expected behavior: `docs/idl-guide/validation.md:239-243` documents
  `"data": {"field": "email", "message": "Required field 'email' is missing"}`.
- Test status: `tests/ValidationTest.java:14-118` only asserts `getMessage().contains(...)`,
  never the shape of error data.
- Suggested fix: introduce a structured validation error carrying field path + message,
  and emit it as `data` from server/client error paths.
- Acceptance criteria:
  - [ ] Missing required field error has `data` with `field` and `message` keys.
  - [ ] Existing message text remains human-readable (backwards compatibility).

### java-12 — Contract/diff/auditor tests exist but never run
- Severity: medium
- Invariant: R13
- Current behavior: `pkg/runtime/runtimes/java/Makefile:7`
  `test: test-validation test-types test-rpc test-json` — omits `SeverityTest.java`,
  `DiffEngineTest.java`, `AuditorTest.java` (all present in `tests/`). Root `Makefile:130-133`
  calls this Makefile, so `make test`/`make test-runtimes` never execute those suites.
  `tests/integration/test_java_compile.sh` is also not referenced by any root Makefile
  target (grep for `test_java_compile` in root `Makefile` returns nothing).
- Expected behavior: catalog `invariants.md:68` marks this [V]; guide `:1216` "Runtime tests written".
- Test status: the tests themselves are asserting and correct; the defect is wiring.
- Suggested fix: add `test-severity`, `test-diff`, `test-audit` targets to the Java
  Makefile and include them in `test`; add a root `test-java-compile` target (or fold
  into `test-generator-java`).
- Acceptance criteria:
  - [ ] `make -C pkg/runtime/runtimes/java test` runs all seven Java test classes.
  - [ ] `make test-runtime-java` fails if any diff/auditor test fails.

### java-13 — Integration harness excludes Java and treats wrong error codes as success
- Severity: medium
- Invariant: M17 / M3 / M1
- Current behavior: `tests/integration/test_all_runtimes.sh:25` declares Java port 9003
  but `:70` loops `for runtime in python ts csharp; do` — Java is skipped. The script is
  not wired into the root `Makefile` (no matching target). Separately
  `tests/integration/test_http_api.sh:102-142` prints `⚠ WARN` for an unexpected error
  code but still executes `✓ PASS` and returns 0. Java is covered only indirectly by
  `test_generator_java.sh:184`.
- Expected behavior: catalog `invariants.md:91` [V][G]; guide `:854` "Test notifications".
- Test status: the harness itself is the gap — no test asserts error codes for Java,
  and no notification/parse-error request exists in the shared suite.
- Suggested fix: add `java` to the loop and wire a root target; change code mismatch in
  `test_validation_error` to increment `TESTS_FAILED` and return 1; add cases for
  notification and malformed JSON.
- Acceptance criteria:
  - [ ] `test_all_runtimes.sh` includes Java and is reachable from the root Makefile.
  - [ ] A wrong error code causes the integration suite to exit non-zero.

### java-14 — Clients do not validate the JSON-RPC response envelope
- Severity: medium
- Invariant: M5
- Current behavior: generated client `pkg/generator/java_client_server.go:1329-1338`
  checks only `jsonResponse.containsKey("error")`, then returns `result`; it never
  checks `jsonrpc == "2.0"`, that the response `id` matches the request id (`:1317`
  hardcodes `"id", 1`), or that exactly one of `result`/`error` is present. Runtime
  `Client.java:95-116` similarly relies solely on `Response.hasError()` (`:50-52`).
- Expected behavior: `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:810-815` mandates full
  JSON-RPC 2.0 response format; catalog `invariants.md:79` marks this [V][G].
- Test status: none — no Java test constructs a malformed response.
- Suggested fix: add a `validateEnvelope(jsonResponse, expectedId)` helper in the
  runtime and call it from both generated and runtime clients before reading result/error.
- Acceptance criteria:
  - [ ] A response with `id != request id` is rejected.
  - [ ] A response containing both `result` and `error` (or neither) is rejected.
  - [ ] A response with `jsonrpc != "2.0"` is rejected.

### java-15 — Generated Java server has no batch request support
- Severity: low
- Invariant: M7
- Current behavior: `pkg/generator/java_client_server.go:978-983` deserializes the body
  to a single `Map<String,Object>`; a JSON array causes `fromJson(..., Map.class)` to
  throw → generic catch (`:991-993`) → NPE via java-01. No branch handles `List`.
- Expected behavior: `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:813` "Batch requests: Array
  of requests"; checklist `:1212`.
- Test status: none — `tests/integration/test_http_api.sh` sends only single objects.
- Suggested fix: parse to `Object`, detect `List`, dispatch each element through
  `handleJsonRpcRequest`, collect non-null responses, and suppress an entirely-empty
  batch response.
- Acceptance criteria:
  - [ ] A two-element batch returns a two-element JSON array of responses.
  - [ ] A batch containing only notifications produces an empty body.
  - [ ] An empty batch returns -32600.

### java-16 — Severity classification contradicts the breaking-change spec
- Severity: low
- Invariant: M16
- Current behavior: `DiffEngine.java:599-600` classifies
  `(Field, Removed, ClientHasMore)` as `Info`, and `tests/SeverityTest.java:29-36`
  asserts `Info`. `DiffEngineTest.java:48-58` likewise asserts Info for a removed field.
- Expected behavior: `openspec/specs/contract-compatibility-verification/spec.md:115-118`
  says removing a required field yields `Compatible=false` and an **Error**-level delta.
- Test status: asserting tests exist but are excluded from the Makefile (java-12).
- Suggested fix: decide spec vs. implementation once; if spec wins, change classifySeverity
  to Error and update SeverityTest/DiffEngineTest expectations accordingly.
- Acceptance criteria:
  - [ ] Removing a required field produces an Error delta and `compatible=false`.
  - [ ] Severity tests run under `make test-runtime-java` and match the spec.

### java-17 — No struct/`validateType` unit tests in the runtime (R4 test gap)
- Severity: low
- Invariant: R4 / M11
- Current behavior: `tests/ValidationTest.java:6-120` tests string/int/bool/array/map/enum
  only; there is no `validateStruct` or `validateType` test. `tests/TypesTest.java:6-83`
  tests only the three helper functions. The code itself
  (`Validation.java:122-159,164-242`) implements missing-required/null-required/optional
  handling correctly, so this is a coverage gap, not a code defect.
- Expected behavior: catalog `invariants.md:59` "[C] except Java has no struct-validation
  test at all"; guide `:843-847` "Test all validation functions ... edge cases".
- Test status: none.
- Suggested fix: add a `testValidateStruct` covering required missing, null required,
  optional missing, nested struct, enum field, and array/map field cases.
- Acceptance criteria:
  - [ ] Missing required field throws; missing optional field does not.
  - [ ] Nested struct and enum field violations are reported with field paths.

## Verified-correct invariants
- R1 — `Types.java:16-25` exposes `findStruct`/`findEnum`/`getStructFields`; `Validation.java:164` exposes `validateType`.
- R3 — Java fails fast with `IllegalArgumentException` (`Validation.java:19,44` etc.), consistent with the survey.
- R4 (code) — required/optional/null rules implemented in `Validation.java:135-158`; only the test is missing (java-17).
- R7 — `RPCError.java:8-53` carries code/message/data, asserted in `tests/RPCTest.java:7-35`.
- R8 — Java uses literal -32600..-32603 codes, consistent with the other runtimes (no named constants anywhere).
- R12 — `Contract.java:115-121` copies `meta` from Barrister-format IDL; PulseRPC IDL JSON has no `meta` key (`pkg/parser/idl.go:8-15`), so no divergence.
- R14 — `Contract.java:207-213` requires null for void returns (only TS accepts undefined), consistent with the survey.
- M3 — runtime `Server.java:66-83` and generated server `java_client_server.go:1052-1077` both return -32601 for unknown interface/method/format; covered by `test_http_api.sh:235-237`.
- M11 — enum membership and nested array/map validation implemented in `Validation.java:68-117,196-205` with tests at `tests/ValidationTest.java:59-119`.
- M12 — extra fields are ignored in `Validation.validateStruct` (`Validation.java:135-158`), matching documented permissive behavior.
- M16 (code matches test) — Java internal behavior is self-consistent; it only conflicts with the spec (java-16).

# Core Plan — parser, checksum, playground, shared harness

Worktree: `pulserpc-fix-core` · Branch: `fix-invariants-core`
Owns: `pkg/parser/**`, `pkg/playground/**`, `pkg/webui/**`,
`pkg/runtime/embed.go`, `tests/integration/**`, `scripts/test-servers.sh`,
`Makefile`, `docs/RUNTIME_IMPLEMENTATION_GUIDE.md`, `docs/audit/**`,
`docs/plans/**`.

Read [README.md](README.md) for the canonical decisions D1–D16.

## P0 — violations and gaps

### 1. G35 — checksum must include parameter position
- `pkg/parser/checksum.go` `writeMethod`: emit `param:<index>:<name>:<type>`
  instead of name-sorted `param:<name>:<type>`. Keep sorting for determinism,
  but include the original index so reordering changes the hash.
- Update any pinned checksums in `pkg/parser/checksum_test.go`,
  `checksum_spec_test.go`, `openspec/**` examples, and runtime tests that pin
  a digest.
- Add a test: `m(x int, y string)` and `m(y string, x int)` have different
  checksums; declaration order of interfaces/structs/fields is still invariant.

### 2. G29 — allow trailing comments on imports
- `pkg/parser/parser.go`: the removal regex must remove an import line with
  trailing whitespace and/or a `//` comment, matching the extraction regex.
- Add tests: `import "inc.pulse" // note`, trailing spaces, leading whitespace,
  plain import, no-comment.

### 3. G30 — deduplicate imports by resolved path
- Resolve each import (`filepath.Join(baseDir, path)` / absolute), `filepath.Clean`,
  and deduplicate on the resolved absolute path while preserving the first
  literal for error messages.
- Add a test: `import "inc.pulse"` + `import "./inc.pulse"` parse once and do
  not raise a duplicate-namespace error.

### 4. G32 — reject root/import namespace collisions
- After collecting imports, reject an imported namespace equal to
  `RootNamespace` with a clear error naming both files.
- Add a test.

### 5. S1 — playground path containment
- `pkg/playground/files.go` `GetFile`: resolve `filepath.Join(session.Dir, filename)`,
  `filepath.Clean` it, and reject absolute paths or a cleaned path that is not
  inside `session.Dir` (use `filepath.Rel` and reject `..`). Apply the same
  helper to any other session-relative read/write.
- Add tests for `../`, `..\\`, nested `a/../../b`, absolute paths, and a valid
  nested file. Add a web-handler test if practical.

### 6. Strengthen the integration harness (M18–M28, R22–R24, R23)
Extend `tests/integration/test_http_api.sh` (and add helpers) so each runtime is
probed for:
- response `jsonrpc == "2.0"`, echoed `id`, HTTP 200, `Content-Type: application/json`;
- malformed JSON → `-32700`;
- empty batch `[]` → a single `-32600` object (not an array);
- a batch with a non-object entry → per-entry `-32600`;
- a notification (no id, valid and invalid method) → no response body;
- a batch mixing a request and a notification → only the request response;
- `{"id":null}` → treated as a notification;
- object-form `params` → an explicit result or an explicit `-32602` (not a
  silent drop);
- extra struct/param fields → accepted;
- a boolean/object `id` → `-32600`;
- non-POST (GET/PUT) → HTTP 405 with a JSON-RPC `-32600` body;
- `error.message` is a string and `error.data` is absent for `-32603`.
Use a small JSON extractor (e.g. `python3 -c` or `jq`) rather than substring
greps where possible.

Add `tests/integration/test_limits.sh`:
- request body > 1 MiB → `-32600` (or the documented status);
- batch > 100 → `-32600` "Batch exceeds maximum size";
- (response cap) a large result is rejected by the client where the runtime
  supports a configurable large-result test.

### 7. Wire the harness and orphaned scripts
- `scripts/test-servers.sh`: add `python2` and `ts-cjs` entries (pinned images
  and commands) so `test_all_runtimes.sh` actually runs all seven.
- `Makefile`: add targets (or extend `test-integration`/`quality-full`) for
  `test_numeric_types.sh`, `test_enum_case.sh`, `test_raises_propagation.sh`,
  `test_static_client_ts.sh`, `test_java_compile.sh`, `test_errors.sh`, and the
  OpenAPI scripts.
- Fix the dead Java `test-integration` target: point it at
  `tests/integration/test_generator_java.sh` (or remove it and document the
  replacement).
- Keep `bash -n` clean on every script.

### 8. Document the canonical decisions
- Add/expand `docs/RUNTIME_IMPLEMENTATION_GUIDE.md` sections for D1–D16:
  int range, id types, non-POST, internal errors, limits, timeout, optional
  nullability, IDL JSON schema, checksum, imports, string literals,
  determinism, headers/trailing slash, playground containment, verification,
  resolution order.
- Update `docs/audit/invariants.md` with a short "resolved" note per stale
  `[V]`, and update `docs/audit/integration-coverage.md` after the harness
  changes.

## P1 — tests and cross-generator checks

- G26/G27/G28/G31/G33–G39: add focused parser tests and document behavior
  (no escapes, mandatory return type, positional params, absolute imports,
  prefixing asymmetry, decimal/negative codes, single namespace, empty files).
- C1: add a cross-generator determinism check (generate twice per runtime into
  two dirs and byte-compare). This will pass once the Java/TS plans land; do not
  wire it into `make test` until the final merge.
- C2/C4/C5: add a check that `idl.json` (where emitted) and the served
  `pulserpc-idl` result have `rootNamespace`, `checksum`, and the five data
  keys, and that the served value is an object (not a JSON string).
- S2/S3/S4: add tests for embed forward slashes, ZIP forward slashes, and
  namespace-path separators (mostly already present; fill gaps).
- R23/R31: assert notification bodies and a `VerifyCompatibility` call against
  a divergent IDL in the harness where a runtime client exposes it.

## Acceptance

- `go test ./pkg/parser ./pkg/generator ./pkg/openapi ./pkg/playground ./pkg/runtime ./tests/integration -count=1`
- `go vet ./pkg/parser ./pkg/generator ./pkg/playground`
- `bash -n` on every changed integration script.
- New tests fail before the fix and pass after.
- No edits to runtime generator/runtime files owned by another plan.

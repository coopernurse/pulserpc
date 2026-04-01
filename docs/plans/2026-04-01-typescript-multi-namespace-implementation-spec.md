# TypeScript Multi-Namespace Generator Implementation Spec

**Date:** 2026-04-01
**Source:** `docs/MULTI_NAMESPACE_CODE_GEN_SPEC.md` (Section 3: TypeScript/JavaScript Generator Specification)
**Scope:** TypeScript generator output layout and import behavior for namespace-per-directory output
**Goal:** Implement Section 3 in small, agent-sized steps with recurring verification gates.

## Background and Current State

The TypeScript generator (`pkg/generator/ts_client_server.go`) currently:

- Has a `-package` flag, but uses it only as a **prefix for abstract class names** (`applyPackagePrefix`), not for module paths or directory structure.
- Calls `GroupTypesByNamespace` but discards the result — all output lands in a single flat directory.
- Emits `types.ts`, `server.ts`, `client.ts`, and `idl.json` in the root `-dir`.
- Copies runtime files to `-dir/pulserpc/`.

### Target Behavior (from spec)

```
lib/rpc/
├── pulserpc/           # Runtime module (unchanged location)
│   ├── index.ts
│   ├── rpc.ts
│   ├── client.ts
│   └── ...
├── user/               # One subdirectory per namespace
│   ├── index.ts        # Re-exports from types, server, client
│   ├── types.ts        # User-defined types for this namespace
│   ├── server.ts       # Abstract class stubs
│   └── client.ts       # Client class
├── book/
│   ├── index.ts
│   ├── types.ts
│   ├── server.ts
│   └── client.ts
└── common/
    ├── index.ts
    ├── types.ts
    ├── server.ts
    └── client.ts
```

**Import rules:**
- Runtime: relative path `'../pulserpc'` (from inside a namespace directory)
- Cross-namespace: relative path `'../{namespace}'`
- When only one namespace is present and no `-dir` flag is given, behavior is unchanged (backwards compatible).

---

## 1. Redefine `-package` Flag Semantics and Add Namespace Path Helpers

**Context:** The current `-package` flag prepends a string to class names (see `applyPackagePrefix`, line 158 of `ts_client_server.go`). Under the new spec, `-package` becomes the base module path (e.g., `@myapp/lib/rpc`). The class-name-prefix behavior must be preserved under a separate mechanism or removed without breaking existing tests.

**Tasks:**

1. Audit all uses of `packagePrefix` / `applyPackagePrefix` in `ts_client_server.go`. Determine whether the class-name-prefix behavior is exercised by any existing test or quickstart. Document findings in a code comment.
2. Add a new helper function `tsNamespaceOutputDir(outputDir, namespace string) string` in `ts_client_server.go` (or a new `ts_namespace_paths.go` file) that returns `filepath.Join(outputDir, namespace)`. This is the directory for all files belonging to one namespace.
3. Add a helper `tsRuntimeImportPath(fromNamespaceDir bool) string` that returns `"./pulserpc"` when the output is in the root (no namespace subdir) and `"../pulserpc"` when inside a namespace subdirectory.
4. Add a helper `tsCrossNamespaceImportPath(fromNamespace, toNamespace string) string` that returns `"../{toNamespace}"` when inside a namespace subdirectory.
5. Add unit tests for all new helper functions in a new file `pkg/generator/ts_namespace_paths_test.go` covering:
   - Single-namespace (no subdir): runtime path is `"./pulserpc"`.
   - Namespace subdir: runtime path is `"../pulserpc"`.
   - Cross-namespace import from `"book"` to `"common"` is `"../common"`.
   - `tsNamespaceOutputDir("lib/rpc", "book")` returns `"lib/rpc/book"`.

**Acceptance tests:**
- `go test ./pkg/generator/... -run TestTsNamespace` passes.
- No existing tests regress (the flag audit comment should explain the plan for the class-name-prefix behavior).

**Make targets to run:**
```
make quality
```

---

## 2. Implement Namespace Output Directory Creation

**Context:** When the generator detects multiple namespaces (or a single namespace with `-dir` set), it must create per-namespace subdirectories under `-dir`.

**Tasks:**

1. In the `Generate` function of `ts_client_server.go`, after calling `GroupTypesByNamespace`, check whether multi-namespace mode should activate. Multi-namespace mode is active when `len(namespaceMap) > 1` OR when `outputDir != ""` and `len(namespaceMap) == 1` with a non-empty namespace value.
2. Add a helper `ensureTsNamespaceDirs(outputDir string, namespaces []string) error` that calls `os.MkdirAll` for each `filepath.Join(outputDir, ns)`.
3. Call `ensureTsNamespaceDirs` from `Generate` when multi-namespace mode is active.
4. Add unit tests in `ts_namespace_paths_test.go` (or a new `ts_generate_dirs_test.go`) that:
   - Verify `ensureTsNamespaceDirs` creates the expected directories in a temp dir.
   - Verify it is idempotent (calling twice does not error).
   - Verify a single-namespace project with `outputDir=""` does **not** create a subdirectory.

**Acceptance tests:**
- `go test ./pkg/generator/... -run TestTsEnsureDirs` passes.
- Existing integration tests are not broken (single-namespace flat output still works).

**Make targets to run:**
```
make quality
```

---

## 3. Split Namespace Types, Server, and Client into Subdirectories

**Context:** Currently all types, server stubs, and client code land in a single flat directory. This step splits them into per-namespace subdirectories, each containing `types.ts`, `server.ts`, and `client.ts`.

**Tasks:**

1. Refactor `generateTypesTs` to accept the target output directory as a parameter instead of always using `outputDir`. When multi-namespace mode is active, pass `filepath.Join(outputDir, namespace)` as the target. Emit only the types that belong to that namespace.
2. Refactor `generateServerTs` similarly: emit only server stubs for that namespace, writing to the namespace subdirectory.
3. Refactor `generateClientTs` similarly: emit the client for that namespace in the namespace subdirectory.
4. In `Generate`, loop over `namespaceMap` entries and call the three refactored generators for each namespace, passing the namespace-scoped output directory.
5. Keep the original flat-file behavior for the single-namespace-no-`-dir` case (backwards compatibility).
6. Add golden / structural tests in `pkg/generator/ts_client_server_test.go` (new file) that:
   - Use a temp output directory.
   - Generate from a two-namespace IDL fixture (or use `examples/conform.pulse` if it has multiple namespaces; otherwise create a minimal inline fixture with `common` and `book` namespaces).
   - Assert `lib/rpc/book/types.ts` exists.
   - Assert `lib/rpc/book/server.ts` exists.
   - Assert `lib/rpc/book/client.ts` exists.
   - Assert `lib/rpc/common/types.ts` exists.
   - Assert `lib/rpc/pulserpc/` exists (runtime unchanged).

**Acceptance tests:**
- `go test ./pkg/generator/... -run TestTsMultiNamespace` passes.
- `go test ./pkg/generator/... -run TestTs` passes (no regressions).

**Make targets to run:**
```
make quality
```

---

## 4. Verification Gate A (Verify Steps 1–3)

**Context:** Validate the foundation: flag semantics clarification, path helpers, directory creation, and per-namespace file splitting.

**Tasks:**

1. Review the code changes from Steps 1–3 for spec alignment:
   - Confirm `tsNamespaceOutputDir`, `tsRuntimeImportPath`, and `tsCrossNamespaceImportPath` helpers exist with unit tests.
   - Confirm `ensureTsNamespaceDirs` is called during generation when appropriate.
   - Confirm that `generateTypesTs`, `generateServerTs`, and `generateClientTs` now emit into namespace subdirectories in multi-namespace mode.
   - Confirm backwards-compatible single-namespace flat output still works.
2. Run quality and TypeScript integration gates:
   ```
   make quality
   make test-quickstart-ts
   make test-generator-ts
   ```
3. Fix any regressions from these runs, then rerun failed targets until all are green.

**Exit criteria:**
- All three targets pass.
- No open regressions from Steps 1–3.

---

## 5. Generate Namespace `index.ts` Re-export Files

**Context:** Each namespace subdirectory must contain an `index.ts` that re-exports from `types.ts`, `server.ts`, and `client.ts` so consumers can import from `'../book'` instead of `'../book/types'`.

**Tasks:**

1. Add a function `generateNamespaceIndexTs(outputDir, namespace string) error` that writes `index.ts` to the namespace subdirectory with re-export statements:
   ```typescript
   export * from './types';
   export * from './server';
   export * from './client';
   ```
2. Call `generateNamespaceIndexTs` for each namespace when in multi-namespace mode from the `Generate` function.
3. Ensure the generated `index.ts` is syntactically valid TypeScript (no duplicate exports if `client.ts` re-exports something from `types.ts` — use `export * from` which TypeScript handles without ambiguity errors in most cases; note any exceptions).
4. Add tests in the existing test file that assert:
   - `lib/rpc/book/index.ts` exists after generation.
   - The file contains `export * from './types'`.
   - The file contains `export * from './server'`.
   - The file contains `export * from './client'`.

**Acceptance tests:**
- `go test ./pkg/generator/... -run TestTsNamespaceIndex` passes.
- The quickstart still compiles (`make test-quickstart-ts`).

**Make targets to run:**
```
make quality
make test-quickstart-ts
```

---

## 6. Fix Import Paths Inside Namespace-Scoped Files

**Context:** Files now live inside a namespace subdirectory (e.g., `book/types.ts`). Any import that references the runtime or another namespace must use a relative path that accounts for the extra directory level: `'../pulserpc'` instead of `'./pulserpc'`, and `'../common'` instead of `'./common'`.

**Tasks:**

1. Update `generateTypesTs` to use `tsRuntimeImportPath(inNamespaceSubdir)` when constructing the import for `RPCError` or any runtime symbol. Pass a boolean `inNamespaceSubdir` (true when multi-namespace mode is active).
2. Update `generateServerTs` similarly for any runtime import it emits (e.g., `import { ... } from './pulserpc/rpc'` must become `import { ... } from '../pulserpc/rpc'`).
3. Update `generateClientTs` similarly.
4. For cross-namespace type references (a type in `book` that references a type defined in `common`), update the import to use `tsCrossNamespaceImportPath("book", "common")` → `'../common'`.
5. Ensure that when multi-namespace mode is **not** active (flat single-namespace output), all imports remain unchanged (e.g., `'./pulserpc'`).
6. Add tests that assert the generated import strings:
   - `book/types.ts` contains `from '../pulserpc'` (not `from './pulserpc'`).
   - `book/types.ts` contains `from '../common'` when `book` includes `common`.
   - Single-namespace flat output still contains `from './pulserpc'`.

**Acceptance tests:**
- `go test ./pkg/generator/... -run TestTsImportPaths` passes.

**Make targets to run:**
```
make quality
```

---

## 7. Place `idl.json` in the Root Output Directory and Update Runtime Copy

**Context:** The `idl.json` file is consumed by the runtime `Contract` class. In multi-namespace mode it should remain in the root `-dir` (not per-namespace), and the runtime copy should remain at `{dir}/pulserpc/`. Verify both behaviors and adjust if needed.

**Tasks:**

1. Confirm (by reading `writeIDLJSONTs`) that `idl.json` is always written to `outputDir` directly (not a namespace subdir). If it is, add an explicit comment explaining this is intentional. If it is not, fix it.
2. Confirm `copyRuntimeFiles` writes to `filepath.Join(outputDir, "pulserpc")`. If not, fix it.
3. Verify that `pulserpc/index.ts` still correctly re-exports all runtime symbols so that `import { ... } from '../pulserpc'` works from inside a namespace subdirectory.
4. Add a test that asserts:
   - `idl.json` is at `lib/rpc/idl.json` (root), not inside any namespace subdir.
   - `lib/rpc/pulserpc/index.ts` exists after generation.
5. If the test client or server files (`test_server.ts`, `test_client.ts`) are generated, verify their import paths are also updated to use `'../pulserpc'` when in multi-namespace mode (or note that they remain in the root and use `'./pulserpc'`).

**Acceptance tests:**
- `go test ./pkg/generator/... -run TestTsIdlAndRuntime` passes.
- `make test-quickstart-ts` passes.

**Make targets to run:**
```
make quality
make test-quickstart-ts
```

---

## 8. Verification Gate B (Verify Steps 5–7)

**Context:** Validate `index.ts` re-exports, corrected import paths, and correct placement of `idl.json` and runtime files.

**Tasks:**

1. Review the code changes from Steps 5–7 for spec alignment:
   - Confirm `index.ts` is generated for each namespace with correct re-exports.
   - Confirm runtime imports inside namespace subdirs use `'../pulserpc'`.
   - Confirm cross-namespace imports use `'../{namespace}'`.
   - Confirm `idl.json` is at root `-dir` and runtime at `{dir}/pulserpc/`.
2. Run quality and TypeScript integration gates:
   ```
   make quality
   make test-quickstart-ts
   make test-generator-ts
   ```
3. Fix any regressions from these runs, then rerun failed targets until all are green.

**Exit criteria:**
- All three targets pass.
- Generated import paths and file placement are validated by tests.

---

## 9. Add Multi-File End-to-End TypeScript Coverage

**Context:** Add a test that mirrors the canonical three-file example from `MULTI_NAMESPACE_CODE_GEN_SPEC.md` (`common.pulse`, `book.pulse`, `user.pulse`) and verifies the full output tree and import strings.

**Tasks:**

1. Create a minimal multi-namespace IDL test fixture (inline in the test, or as files under `tests/fixtures/multi-namespace/`) with three namespaces: `common`, `book`, and `user`, where `book` and `user` include `common`.
2. Add a Go test in `pkg/generator/ts_client_server_test.go` that:
   - Generates from all three IDL inputs with `-dir lib/rpc`.
   - Asserts the following files exist:
     - `lib/rpc/pulserpc/index.ts`
     - `lib/rpc/common/types.ts`, `lib/rpc/common/server.ts`, `lib/rpc/common/client.ts`, `lib/rpc/common/index.ts`
     - `lib/rpc/book/types.ts`, `lib/rpc/book/server.ts`, `lib/rpc/book/client.ts`, `lib/rpc/book/index.ts`
     - `lib/rpc/user/types.ts`, `lib/rpc/user/server.ts`, `lib/rpc/user/client.ts`, `lib/rpc/user/index.ts`
   - Asserts that `lib/rpc/book/types.ts` contains `from '../common'`.
   - Asserts that `lib/rpc/user/types.ts` contains `from '../common'`.
   - Asserts that `lib/rpc/book/types.ts` contains `from '../pulserpc'`.
3. Optionally, add a shell integration test under `tests/integration/` that compiles the multi-namespace output with `tsc` and verifies zero compile errors.

**Acceptance tests:**
- `go test ./pkg/generator/... -run TestTsMultiFileEndToEnd` passes.
- (Optional) Shell integration test exits 0.

**Make targets to run:**
```
make quality
make test-generator-ts
```

---

## 10. Backward Compatibility and Deprecation of Class-Name Prefix Behavior

**Context:** The existing `-package` flag currently acts as a class-name prefix. This must not silently break users who relied on that behavior. The new spec repurposes `-package` as a base module path. This step ensures a clean migration.

**Tasks:**

1. Based on the audit from Step 1, decide one of the following (document the decision in a code comment):
   - **Option A:** The class-name prefix behavior was not load-bearing (no test uses it meaningfully); remove `applyPackagePrefix` and update the flag description.
   - **Option B:** Add a separate `-name-prefix` flag to replace the old class-name prefix behavior; emit a deprecation warning if `-package` is used in a way that looks like the old behavior.
2. Update the `-package` flag's usage string to: `"Base module path for generated imports (e.g., @myapp/lib/rpc)"`.
3. Verify that existing integration tests (`test-quickstart-ts`, `test-generator-ts`) do not pass `-package` in a way that would break under the new semantics. If they do, update the test scripts to use the new flag.
4. Add a test that verifies the new `-package` flag does **not** affect class names (or documents that it no longer does).

**Acceptance tests:**
- `go test ./pkg/generator/... -run TestTsPackageFlag` passes.
- `make test-quickstart-ts` passes without modification to the quickstart example code.

**Make targets to run:**
```
make quality
make test-quickstart-ts
```

---

## 11. Stabilize Test Reliability and Fixtures

**Context:** Ensure all new TypeScript generator tests are deterministic, do not rely on path separators or temp-dir assumptions, and share reusable helpers.

**Tasks:**

1. Review all new tests added in Steps 1–10. Remove any brittle assertions such as hardcoded `/tmp/` paths, OS-specific path separators (use `filepath.Join`), or ordering-dependent output.
2. Add a shared test helper `withTempOutputDir(t *testing.T, fn func(dir string))` in `ts_client_server_test.go` that creates and cleans up a temp directory, following the pattern from the Python generator tests.
3. Add a shared helper `assertTsFileExists(t *testing.T, dir, relPath string)` and `assertTsFileContains(t *testing.T, dir, relPath, substr string)` to reduce boilerplate.
4. Confirm all new tests pass consistently when run multiple times (`go test -count=3 ./pkg/generator/... -run TestTs`).
5. Confirm new tests pass in isolation (`go test -run TestTs ./pkg/generator/...`) and as part of the full suite.

**Acceptance tests:**
- `go test -count=3 ./pkg/generator/... -run TestTs` exits 0 all three times.
- No test uses hardcoded platform-specific paths.

**Make targets to run:**
```
make quality
```

---

## 12. Final Verification Gate (Verify Steps 9–11 and Full TypeScript Scope)

**Context:** Full validation that all TypeScript multi-namespace deliverables from Section 3 of the spec are complete and no regressions remain.

**Tasks:**

1. Review all Step 9–11 deliverables and confirm overall Section 3 compliance:
   - Multi-namespace output tree matches the spec layout exactly.
   - Import paths (`'../pulserpc'`, `'../{namespace}'`) are generated correctly.
   - `index.ts` re-exports are present for each namespace.
   - `idl.json` and runtime files are at the correct root locations.
   - `-package` flag semantics are updated and documented.
   - Backwards-compatible single-namespace flat output still works.
   - All tests are deterministic and use shared helpers.
2. Run the full relevant target set:
   ```
   make quality
   make test-runtime-ts
   make test-quickstart-ts
   make test-generator-ts
   ```
3. Fix any regressions, rerun impacted targets, and do not close until all are green.

**Exit criteria:**
- All four targets pass.
- No known regressions remain in TypeScript code generation behavior.
- Section 3 of `docs/MULTI_NAMESPACE_CODE_GEN_SPEC.md` is fully implemented.

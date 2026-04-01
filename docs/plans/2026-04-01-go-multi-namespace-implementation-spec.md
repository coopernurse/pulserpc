# Go Multi-Namespace Generator Implementation Spec

**Date:** 2026-04-01
**Source:** `docs/MULTI_NAMESPACE_CODE_GEN_SPEC.md` (Section 1: Go Generator Specification)
**Scope:** Go generator and Go runtime packaging behavior for namespace-per-directory output
**Goal:** Implement Section 1 in small, agent-sized steps with recurring verification gates.

## 1. Add Go `-package` CLI Surface and Deprecate `-go-module`

Tasks:
1. Add `-package` flag to Go generator configuration, accepting base import path (e.g., `github.com/myapp/lib/rpc`).
2. Add deprecation warning for `-go-module` flag, suggesting `-package` instead.
3. Ensure the Go plugin receives the `-package` value without changing other language plugins.
4. Add/update focused unit tests for flag parsing and plugin config propagation.

Deliverable:
- Go generator can read a `-package` value for use as base import path.
- `-go-module` still works but emits deprecation warning.

Acceptance Tests:
- Verify `-package github.com/myapp/lib/rpc` is parsed correctly.
- Verify `-go-module github.com/myapp/lib/rpc` still works but shows deprecation warning.
- Verify `-package` and `-go-module` cannot be used together.

Make targets to run after step:
```
make build
make test-generator-go
```

## 2. Introduce Namespace Output Layout Helpers

Tasks:
1. Add helper(s) that resolve Go output paths as `{dir}/{namespace}/{namespace}.go` plus `{dir}/pulserpc/`.
2. Normalize namespace directory creation for include-driven and single-file generation.
3. Add path-construction tests that cover nested output dirs and multiple namespaces.
4. Ensure runtime always goes to `{dir}/pulserpc/` regardless of namespace location.

Deliverable:
- Deterministic path logic for generated `.go` files and runtime files.
- Runtime placement is always under `-dir` as `pulserpc/` subdirectory.

Acceptance Tests:
- Verify `lib/rpc/user/user.go` path construction for namespace "user" and dir "lib/rpc".
- Verify `lib/rpc/pulserpc/rpc.go` path for runtime when dir is "lib/rpc".
- Verify path construction for nested dirs like `src/github.com/myapp/rpc`.

Make targets to run after step:
```
make build
make test-generator-go
```

## 3. Generate Namespace Package Scaffolding

Tasks:
1. Ensure each namespace output directory contains a `package {namespace}` declaration.
2. Ensure generated files are named `{namespace}.go` (e.g., `user.go` for namespace `user`).
3. Add tests that assert required file structure exists for one namespace and two-namespace scenarios.
4. Verify package declaration matches directory name.

Deliverable:
- Go namespace directories contain properly named `.go` files with correct package declarations.

Acceptance Tests:
- Single namespace "book" with `-dir lib/rpc` produces `lib/rpc/book/book.go` with `package book`.
- Two namespaces "book" and "common" produce separate directories with correct package declarations.
- Package name in generated file matches the directory name (not the filename).

Make targets to run after step:
```
make build
make test-generator-go
```

## 4. Verification Gate A (Verify Steps 1-3)

Tasks:
1. Verify implementation quality and spec alignment for Steps 1-3 (CLI flag, path helpers, package scaffolding).
2. Run quality and Go integration gates:
   - `make quality`
   - `make test-generator-go`
3. Fix regressions from these runs, then rerun failed targets until green.

Exit criteria:
- All targets pass and no open regressions remain from Steps 1-3.
- CLI flag behavior matches spec (package flag works, go-module shows deprecation).
- Path construction produces correct `lib/rpc/{namespace}/{namespace}.go` layout.
- Runtime always goes to `lib/rpc/pulserpc/`.

## 5. Split Go Outputs by Namespace into Subdirectories

Tasks:
1. Refactor Go generation to emit namespace-local `{namespace}.go` files into `{dir}/{namespace}/` subdirectories.
2. Keep existing single-namespace behavior functionally equivalent when only one namespace is present.
3. Ensure runtime files are NOT placed inside namespace directories (always in `pulserpc/`).
4. Add golden/snapshot or structural tests for generated file placement.

Deliverable:
- Namespace folders contain their own generated artifacts under `{dir}/{namespace}/{namespace}.go`.
- Runtime files are isolated in `{dir}/pulserpc/` and never duplicated in namespace dirs.

Acceptance Tests:
- Verify `pulserpc` runtime files exist only in `lib/rpc/pulserpc/` and NOT in `lib/rpc/book/pulserpc/`.
- Verify `lib/rpc/book/book.go` exists when generating for namespace "book".
- Verify `lib/rpc/user/user.go` exists when generating for namespace "user".

Make targets to run after step:
```
make build
make test-generator-go
```

## 6. Implement Cross-Namespace Imports

Tasks:
1. Generate imports using the configured `-package` value for cross-namespace references (e.g., `import "github.com/myapp/lib/rpc/common"`).
2. Ensure include-based type references resolve to the correct namespace package.
3. Add tests for `book` -> `common` and `user` -> `common` import strings.
4. Verify imports are only added when the namespace is actually used (not all possible imports).

Deliverable:
- Cross-namespace imports are stable, spec-compliant, and only included when needed.

Acceptance Tests:
- Given `book.pulse` includes `common.pulse`, when generating with `-package github.com/myapp/lib/rpc`, then `book/book.go` imports `"github.com/myapp/lib/rpc/common"`.
- Given `user.pulse` includes `common.pulse`, when generating with `-package github.com/myapp/lib/rpc`, then `user/user.go` imports `"github.com/myapp/lib/rpc/common"`.
- Book that does NOT include common should NOT import the common package.

Make targets to run after step:
```
make build
make test-generator-go
```

## 7. Implement Runtime Imports Through Base Package

Tasks:
1. Generate runtime imports via base package path (e.g., `import "github.com/myapp/lib/rpc/pulserpc"`).
2. Preserve backwards-compatible behavior when `-package` is omitted (use existing import logic).
3. Add tests for both configured and empty-package modes.
4. Ensure runtime imports are always present in files that need them.

Deliverable:
- Runtime imports match Section 1 rules and remain backwards compatible.

Acceptance Tests:
- When `-package github.com/myapp/lib/rpc` is set, generated files import `"github.com/myapp/lib/rpc/pulserpc"`.
- When `-package` is omitted, existing single-namespace import behavior is preserved.
- Generated files that need runtime types (e.g., RPCError) include the pulserpc import.

Make targets to run after step:
```
make build
make test-generator-go
```

## 8. Verification Gate B (Verify Steps 5-7)

Tasks:
1. Verify implementation quality and spec alignment for Steps 5-7 (output split, namespace imports, runtime imports).
2. Run quality and Go integration gates:
   - `make quality`
   - `make test-generator-go`
3. Fix regressions from these runs, then rerun failed targets until green.

Exit criteria:
- All targets pass and generated imports/paths are validated by tests.
- Namespace subdirectory layout is enforced.
- Cross-namespace imports use full `-package` qualified paths.
- Runtime imports use the configured base package.

## 9. Add Multi-File End-to-End Go Coverage

Tasks:
1. Add tests for the `common.pulse`, `book.pulse`, `user.pulse` multi-file flow from the spec.
2. Assert output tree includes `pulserpc/`, `common/`, `book/`, and `user/` under the selected `-dir`.
3. Assert generated `book` and `user` packages import from `common` via configured base package.
4. Verify generated code compiles (where applicable) and passes basic structure validation.

Deliverable:
- Multi-file namespace generation is enforced in tests with real IDL file scenarios.

Acceptance Tests:
- Generate `common.pulse` first, then `book.pulse` (includes common), then `user.pulse` (includes common).
- Verify final structure: `lib/rpc/{pulserpc,common,book,user}` directories exist.
- Verify `lib/rpc/book/book.go` imports `"github.com/myapp/lib/rpc/common"`.
- Verify `lib/rpc/user/user.go` imports `"github.com/myapp/lib/rpc/common"`.
- Verify `lib/rpc/book/book.go` and `lib/rpc/user/user.go` both import `"github.com/myapp/lib/rpc/pulserpc"`.

Make targets to run after step:
```
make build
make test-generator-go
make test-quickstart-go
```

## 10. Add Migration and Compatibility Notes

Tasks:
1. Add/update user-facing docs for Go `-package` usage and expected output layout.
2. Document backward compatibility expectations when `-package` is not set.
3. Add deprecation notice for `-go-module` flag.
4. Add a short troubleshooting section for import path mistakes.

Deliverable:
- Developers can adopt new behavior without ambiguity.
- Migration path from old `-go-module` to new `-package` is clear.

Acceptance Tests:
- Verify deprecation warning is shown when using `-go-module`.
- Verify docs mention `-package` as the preferred flag.

Make targets to run after step:
```
make build
make quality
```

## 11. Stabilize Test Reliability and Fixtures

Tasks:
1. Remove brittle assertions in updated Go generator tests (path separator, ordering, temp-dir assumptions).
2. Consolidate reusable fixtures/helpers for namespace layout and import assertions.
3. Ensure tests are deterministic locally and in CI.
4. Verify test cleanup properly removes generated files.

Deliverable:
- Reliable Go generator test suite for multi-namespace behavior.

Acceptance Tests:
- All Go generator tests pass consistently across runs.
- Temp directories are cleaned up after tests.
- Path assertions use cross-platform compatible path joining.

Make targets to run after step:
```
make build
make test-generator-go
```

## 12. Final Verification Gate (Verify Steps 9-11 and Full Go Scope)

Tasks:
1. Verify all Step 9-11 deliverables and overall Section 1 compliance.
2. Run full relevant targets for this language and quality:
   - `make quality`
   - `make test-generator-go`
   - `make test-quickstart-go`
   - `make test-runtime-go` (if exists)
3. Fix any regressions, rerun impacted targets, and do not close until all are green.

Exit criteria:
- All relevant quality/runtime/quickstart/generator targets pass.
- No known regressions remain in Go code generation behavior.
- Generated code matches the directory layout and import path requirements from Section 1 of the spec.

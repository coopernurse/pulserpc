# C# Multi-Namespace Generator Implementation Spec

**Date:** 2026-04-01
**Source:** `docs/MULTI_NAMESPACE_CODE_GEN_SPEC.md` (Section 5: C# Generator Specification)
**Scope:** C# generator and runtime packaging behavior for namespace-per-directory output
**Goal:** Implement Section 5 in small, agent-sized steps with recurring verification gates.

## 1. Add C# `-package` CLI Surface

Tasks:
1. Add a `-package` option to generator configuration plumbing (default empty, backwards-compatible).
2. Ensure the C# plugin receives this value without changing other language plugins.
3. Add/update focused unit tests for flag parsing and plugin config propagation.

Deliverable:
- C# generator can read a base namespace value (e.g., `MyApp.Lib.Rpc`).
- "make quality" passes

## 2. Introduce Namespace Output Layout Helpers

Tasks:
1. Add helper(s) that resolve C# output paths as `{dir}/{Namespace}/` plus `{dir}/PulseRPC/`.
2. Normalize namespace-to-PascalCase conversion (e.g., `user_account` → `UserAccount`).
3. Add path-construction tests that cover nested output dirs and multiple namespaces.

Deliverable:
- Deterministic path logic for `Types.cs`, `Server.cs`, `Client.cs`, and runtime files.
- "make quality" passes

## 3. Generate Namespace Directory Scaffolding

Tasks:
1. Ensure namespace output directory is created (e.g., `lib/rpc/Book/`).
2. Ensure regeneration is idempotent and does not corrupt existing generated files.
3. Add tests that assert required directories exist for one namespace and two-namespace scenarios.

Deliverable:
- C# namespace directories are created with proper PascalCase names.
- "make quality" passes

## 4. Verification Gate A (Verify Steps 1-3)

Tasks:
1. Ask an agent to verify implementation quality and spec alignment for Steps 1-3 (CLI flag, path helpers, namespace scaffolding).
2. Run quality and C# integration gates:
   - `make quality`
   - `make test-quickstart-csharp` (or equivalent)
   - `make test-generator-csharp` (or equivalent)
3. Fix regressions from these runs, then rerun failed targets until green.

Exit criteria:
- All relevant targets pass and no open regressions remain from Steps 1-3.

## 5. Split C# Outputs by Namespace

Tasks:
1. Refactor C# generation to emit namespace-local `Types.cs`, `Server.cs`, and `Client.cs` instead of flat files.
2. Keep existing single-namespace behavior functionally equivalent when only one namespace is present.
3. Add golden/snapshot or structural tests for generated file placement.

Deliverable:
- Namespace folders contain their own generated artifacts.
- "make quality" passes

## 6. Implement Cross-Namespace Imports

Tasks:
1. Generate `using` statements using the configured base namespace for cross-namespace references (e.g., `using MyApp.Lib.Rpc.Common;`).
2. Ensure include-driven type references resolve to the correct namespace.
3. Add tests for `book` -> `common` and `user` -> `common` import strings.

Deliverable:
- Cross-namespace imports are stable and spec-compliant.
- "make quality" passes

## 7. Implement Runtime Imports Through Base Namespace

Tasks:
1. Generate runtime imports via base namespace path (e.g., `using MyApp.Lib.Rpc.PulseRPC;`).
2. Preserve backwards-compatible behavior when `-package` is omitted (use `using PulseRPC;`).
3. Add tests for both configured and empty-package modes.

Deliverable:
- Runtime imports match Section 5 rules and remain backwards compatible.
- "make quality" passes

## 8. Verification Gate B (Verify Steps 5-7)

Tasks:
1. Ask an agent to verify implementation quality and spec alignment for Steps 5-7 (output split, namespace imports, runtime imports).
2. Run quality and C# integration gates:
   - `make quality`
   - `make test-quickstart-csharp`
   - `make test-generator-csharp`
3. Fix regressions from these runs, then rerun failed targets until green.

Exit criteria:
- All relevant targets pass and generated imports/paths are validated by tests.

## 9. Add Multi-File End-to-End C# Coverage

Tasks:
1. Add tests for the `common.pulse`, `book.pulse`, `user.pulse` multi-file flow from the spec.
2. Assert output tree includes `PulseRPC/`, `Common/`, `Book/`, and `User/` under the selected `-dir`.
3. Assert generated `Book` and `User` namespaces import from `Common` via configured base namespace.

Deliverable:
- Multi-file namespace generation is enforced in tests.
- "make quality" passes

## 10. Implement PascalCase Namespace Conversion

Tasks:
1. Ensure namespace conversion handles underscores and lowercase (e.g., `user_account` → `UserAccount`).
2. Apply PascalCase conversion to both directory names and namespace declarations.
3. Add tests verifying `user_account` produces `UserAccount/` directory and `namespace UserAccount`.

Deliverable:
- Namespace conversion is consistent with C# conventions.
- "make quality" passes

## 11. Add Migration and Compatibility Notes

Tasks:
1. Add/update user-facing docs for C# `-package` usage and expected output layout.
2. Document backward compatibility expectations when `-package` is not set.
3. Add a short troubleshooting section for namespace/using statement mistakes.

Deliverable:
- Developers can adopt new behavior without ambiguity.
- "make quality" passes

## 12. Verification Gate C (Verify Steps 9-11)

Tasks:
1. Ask an agent to verify implementation quality and spec alignment for Steps 9-11 (multi-file tests, PascalCase conversion, docs).
2. Run quality and C# integration gates:
   - `make quality`
   - `make test-quickstart-csharp`
   - `make test-generator-csharp`
3. Fix regressions from these runs, then rerun failed targets until green.

Exit criteria:
- All relevant targets pass and no open regressions remain from Steps 9-11.

## 13. Stabilize Test Reliability and Fixtures

Tasks:
1. Remove brittle assertions in updated C# generator tests (path separator, ordering, temp-dir assumptions).
2. Consolidate reusable fixtures/helpers for namespace layout and using statement assertions.
3. Ensure tests are deterministic locally and in CI.

Deliverable:
- Reliable C# generator test suite for multi-namespace behavior.
- "make quality" passes

## 14. Final Verification Gate (Verify Steps 10-12 and Full C# Scope)

Tasks:
1. Ask an agent to verify all Step 10-12 deliverables and overall Section 5 compliance.
2. Run full relevant targets for this language and quality:
   - `make quality`
   - `make test-quickstart-csharp`
   - `make test-generator-csharp`
3. Fix any regressions, rerun impacted targets, and do not close until all are green.

Exit criteria:
- All relevant quality/quickstart/generator targets pass.
- No known regressions remain in C# code generation behavior.
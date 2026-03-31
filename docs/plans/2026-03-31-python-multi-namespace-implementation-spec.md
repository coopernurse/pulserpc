# Python Multi-Namespace Generator Implementation Spec

**Date:** 2026-03-31
**Source:** `docs/MULTI_NAMESPACE_CODE_GEN_SPEC.md` (Section 2: Python Generator Specification)
**Scope:** Python generator and Python runtime packaging behavior for namespace-per-directory output
**Goal:** Implement Section 2 in small, agent-sized steps with recurring verification gates.

## 1. Add Python `-package` CLI Surface

Tasks:
1. Add a Python package-base option to generator configuration plumbing (default empty, backwards-compatible).
2. Ensure the Python plugin receives this value without changing other language plugins.
3. Add/update focused unit tests for flag parsing and plugin config propagation.

Deliverable:
- Python generator can read a base package value (e.g., `myapp.lib.rpc`).

## 2. Introduce Namespace Output Layout Helpers

Tasks:
1. Add helper(s) that resolve Python output paths as `{dir}/{namespace}/` plus `{dir}/pulserpc/`.
2. Normalize namespace directory creation for include-driven and single-file generation.
3. Add path-construction tests that cover nested output dirs and multiple namespaces.

Deliverable:
- Deterministic path logic for `types.py`, `server.py`, `client.py`, and runtime files.

## 3. Generate Namespace Package Scaffolding

Tasks:
1. Generate `__init__.py` in each namespace output directory.
2. Ensure regeneration is idempotent and does not corrupt existing generated files.
3. Add tests that assert required files exist for one namespace and two-namespace scenarios.

Deliverable:
- Python namespace directories are importable packages.

## 4. Verification Gate A (Verify Steps 1-3)

Tasks:
1. Ask an agent to verify implementation quality and spec alignment for Steps 1-3 (CLI flag, path helpers, namespace scaffolding).
2. Run quality and Python integration gates:
   - `make quality`
   - `make test-quickstart-python`
   - `make test-generator-python`
3. Fix regressions from these runs, then rerun failed targets until green.

Exit criteria:
- All three targets pass and no open regressions remain from Steps 1-3.

## 5. Split Python Outputs by Namespace

Tasks:
1. Refactor Python generation to emit namespace-local `types.py`, `server.py`, and `client.py` instead of flat files.
2. Keep existing single-namespace behavior functionally equivalent when only one namespace is present.
3. Add golden/snapshot or structural tests for generated file placement.

Deliverable:
- Namespace folders contain their own generated artifacts.

## 6. Implement Cross-Namespace Imports

Tasks:
1. Generate imports using the configured base package for cross-namespace references (e.g., `from myapp.lib.rpc.common import ...`).
2. Ensure include-based type references resolve to the correct namespace package.
3. Add tests for `book` -> `common` and `user` -> `common` import strings.

Deliverable:
- Cross-namespace imports are stable and spec-compliant.

## 7. Implement Runtime Imports Through Base Package

Tasks:
1. Generate runtime imports via base package path (e.g., `from myapp.lib.rpc.pulserpc import ...`).
2. Preserve backwards-compatible behavior when `-package` is omitted.
3. Add tests for both configured and empty-package modes.

Deliverable:
- Runtime imports match Section 2 rules and remain backwards compatible.

## 8. Verification Gate B (Verify Steps 5-7)

Tasks:
1. Ask an agent to verify implementation quality and spec alignment for Steps 5-7 (output split, namespace imports, runtime imports).
2. Run quality and Python integration gates:
   - `make quality`
   - `make test-quickstart-python`
   - `make test-generator-python`
3. Fix regressions from these runs, then rerun failed targets until green.

Exit criteria:
- All three targets pass and generated imports/paths are validated by tests.

## 9. Add Multi-File End-to-End Python Coverage

Tasks:
1. Add tests for the `common.pulse`, `book.pulse`, `user.pulse` multi-file flow from the spec.
2. Assert output tree includes `pulserpc/`, `common/`, `book/`, and `user/` under the selected `-dir`.
3. Assert generated `book` and `user` modules import from `common` via configured base package.

Deliverable:
- Multi-file namespace generation is enforced in tests.

## 10. Add Migration and Compatibility Notes

Tasks:
1. Add/update user-facing docs for Python `-package` usage and expected output layout.
2. Document backward compatibility expectations when `-package` is not set.
3. Add a short troubleshooting section for import path mistakes.

Deliverable:
- Developers can adopt new behavior without ambiguity.

## 11. Stabilize Test Reliability and Fixtures

Tasks:
1. Remove brittle assertions in updated Python generator tests (path separator, ordering, temp-dir assumptions).
2. Consolidate reusable fixtures/helpers for namespace layout and import assertions.
3. Ensure tests are deterministic locally and in CI.

Deliverable:
- Reliable Python generator test suite for multi-namespace behavior.

## 12. Final Verification Gate (Verify Steps 9-11 and Full Python Scope)

Tasks:
1. Ask an agent to verify all Step 9-11 deliverables and overall Section 2 compliance.
2. Run full relevant targets for this language and quality:
   - `make quality`
   - `make test-runtime-python`
   - `make test-quickstart-python`
   - `make test-generator-python`
3. Fix any regressions, rerun impacted targets, and do not close until all are green.

Exit criteria:
- All relevant quality/runtime/quickstart/generator targets pass.
- No known regressions remain in Python code generation behavior.

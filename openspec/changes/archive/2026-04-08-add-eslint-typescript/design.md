## Context

The pulserpc TypeScript runtime (`pkg/runtime/runtimes/ts/pulserpc/`) and generated TypeScript output in `examples/quickstart/typescript/` currently have no linting. Generated code and runtime code should be validated for quality issues before use.

## Goals / Non-Goals

**Goals:**
- Add ESLint configuration for TypeScript files in the pulserpc runtime
- Add ESLint configuration for generated TypeScript output in quickstart-ts example
- Integrate linting into the test/validation workflow
- Iterate on lint rules until all issues pass

**Non-Goals:**
- Linting other language runtimes (Go, Python, Java)
- Changing existing eslint configuration in pkg/webui
- Creating a shared eslint config across all TypeScript packages at this time

## Decisions

### 1. ESLint Configuration for TypeScript Runtime

**Decision:** Add `eslint.config.js` to `pkg/runtime/runtimes/ts/` and include TypeScript support via `@typescript-eslint/parser`.

**Rationale:** The runtime package is standalone and should have its own lint config. Using `typescript-eslint` parser enables proper TypeScript parsing and rules.

**Alternative:** Could add to the root `pulserpc` directory, but keeping it with the runtime package is more self-contained.

### 2. ESLint Configuration for quickstart-ts Example

**Decision:** Add `eslint.config.js` to `examples/quickstart/typescript/`.

**Rationale:** The example serves as a test case for generated output. Linting it validates that the generator produces lint-clean code.

### 3. Linting Integration

**Decision:** Run `eslint` via npm script or directly in the validation step.

**Rationale:** Simple approach that can be run locally and in CI. The user mentioned either `generator-ts` test or `quickstart-ts` would be sufficient - we'll use quickstart-ts as the validation point.

## Risks / Trade-offs

- **Generated code may have lint issues** → Iterate on eslint rules and/or generator to fix issues
- **Strict rules may block development** → Start with reasonable defaults, disable problematic rules if needed
- **Multiple eslint configs** → Each package is independent for now to avoid coupling
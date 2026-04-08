## Why

Generated TypeScript code and the pulserpc runtime need to be validated for code quality. Currently there is no linting on the TypeScript output, which could allow malformed or problematic code to be generated and used in production.

## What Changes

- Add ESLint configuration for TypeScript files in the pulserpc runtime
- Add ESLint configuration to validate generated TypeScript output in quickstart-ts example
- Run linting as part of testing/validation workflow
- Iterate on lint rules until all issues are resolved

## Capabilities

### New Capabilities

- `typescript-eslint`: Add ESLint linting for TypeScript files in the pulserpc runtime and generated quickstart example

### Modified Capabilities

- None

## Impact

- `pkg/runtime/runtimes/ts/pulserpc/` - Runtime TypeScript files will be linted
- `examples/quickstart/typescript/` - Generated output will be linted
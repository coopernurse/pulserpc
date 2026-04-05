## ADDED Requirements

### Requirement: Multi-namespace directory structure

When the IDL contains multiple namespaces and the `-package` flag is provided, the TypeScript generator SHALL create a directory structure where each namespace has its own subdirectory containing `types.ts`, `server.ts`, `client.ts`, and `index.ts`.

#### Scenario: Multi-namespace output with package flag
- **WHEN** IDL has namespaces `common` and `orders`
- **AND** user runs with `-package @mycompany/api`
- **THEN** output contains `common/` and `orders/` directories
- **AND** each contains `types.ts`, `server.ts`, `client.ts`, `index.ts`

### Requirement: index.ts re-exports

Each namespace directory SHALL contain an `index.ts` that re-exports from `types.ts`, `server.ts`, and `client.ts`.

#### Scenario: index.ts contents
- **WHEN** generating multi-namespace TypeScript
- **THEN** `common/index.ts` contains `export * from './types'; export * from './server'; export * from './client';`

### Requirement: idl.json at root

The `idl.json` file SHALL be placed at the root output directory, NOT inside any namespace subdirectory, regardless of multi-namespace mode.

#### Scenario: idl.json placement
- **WHEN** generating multi-namespace TypeScript
- **THEN** `idl.json` is at `{outputDir}/idl.json`
- **AND** it is NOT at `{outputDir}/{namespace}/idl.json`

### Requirement: Cross-namespace imports

When in multi-namespace mode, types from other namespaces SHALL be imported using relative paths (`../{namespace}/types`).

#### Scenario: Import from another namespace
- **WHEN** `orders/server.ts` references a type from `common`
- **THEN** it contains `import * as common from '../common/types';`

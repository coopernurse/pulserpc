## ADDED Requirements

### Requirement: ESLint validates TypeScript runtime code
The pulserpc TypeScript runtime SHALL pass ESLint validation with no errors.

#### Scenario: Runtime code passes linting
- **WHEN** ESLint is run on `pkg/runtime/runtimes/ts/pulserpc/`
- **THEN** no lint errors are reported

### Requirement: ESLint validates generated TypeScript output
The generated TypeScript code in quickstart-ts example SHALL pass ESLint validation with no errors.

#### Scenario: Generated code passes linting
- **WHEN** ESLint is run on `examples/quickstart/typescript/`
- **THEN** no lint errors are reported

### Requirement: ESLint configuration is self-contained
Each TypeScript package SHALL have its own `eslint.config.js` configuration file.

#### Scenario: Runtime has eslint config
- **WHEN** examining `pkg/runtime/runtimes/ts/`
- **THEN** an `eslint.config.js` file exists

#### Scenario: Quickstart has eslint config
- **WHEN** examining `examples/quickstart/typescript/`
- **THEN** an `eslint.config.js` file exists
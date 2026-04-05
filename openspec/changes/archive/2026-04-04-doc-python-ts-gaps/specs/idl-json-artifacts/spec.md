## ADDED Requirements

### Requirement: idl.json file generation

The Python and TypeScript generators SHALL write an `idl.json` file containing the parsed IDL metadata to the output directory.

#### Scenario: Python generates idl.json
- **WHEN** `python-client-server` plugin generates code
- **THEN** `idl.json` is written to the output directory root

#### Scenario: TypeScript generates idl.json
- **WHEN** `ts-client-server` plugin generates code
- **THEN** `idl.json` is written to the output directory root (not in namespace subdir)

### Requirement: idl.json deployment requirement

Documentation SHALL clarify that `idl.json` must be deployed alongside generated code as it is required at runtime.

#### Scenario: Runtime dependency
- **WHEN** deploying generated Python or TypeScript code
- **THEN** `idl.json` must be accessible from the deployed application
- **AND** Python `Client` fetches it via `pulserpc-idl` RPC if not present locally
- **AND** TypeScript `Contract` loads it via `JSON.parse(readFileSync('idl.json'))`

### Requirement: idl.json contents

The `idl.json` file SHALL contain valid JSON matching the PulseRPC IDL JSON schema with interfaces, structs, and enums arrays.

#### Scenario: idl.json structure
- **WHEN** `idl.json` is generated for IDL with interfaces, structs, and enums
- **THEN** it contains `{ "interfaces": [...], "structs": [...], "enums": [...] }`

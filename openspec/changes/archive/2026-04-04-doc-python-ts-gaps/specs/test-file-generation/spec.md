## ADDED Requirements

### Requirement: Test file generation flag

The CLI SHALL support a `-generate-test-files` flag that causes test server and client implementations to be generated alongside normal output files.

#### Scenario: Flag generates test files
- **WHEN** user runs with `-generate-test-files`
- **THEN** `test_server.py` and `test_client.py` are created (Python)
- **AND** `test_server.ts` and `test_client.ts` are created (TypeScript)

### Requirement: Test server implementation

Generated test servers SHALL provide concrete implementations of all interface methods with sensible default return values for testing.

#### Scenario: Python test server structure
- **WHEN** Python test server is generated
- **THEN** it imports generated server classes and provides implementations
- **AND** it creates a `TestRPCHandler` extending `BaseHTTPRequestHandler`
- **AND** it listens on port 8080

#### Scenario: TypeScript test server structure
- **WHEN** TypeScript test server is generated
- **THEN** it imports generated server classes and provides implementations
- **AND** it creates an HTTP server listening on port 8080

### Requirement: Test client scaffolding

Generated test clients SHALL provide ready-to-run tests that validate basic RPC functionality.

#### Scenario: Python test client structure
- **WHEN** Python test client is generated
- **THEN** it contains `wait_for_server()` function
- **AND** a `main()` function that calls each interface method
- **AND** assertions that validate responses

#### Scenario: TypeScript test client structure
- **WHEN** TypeScript test client is generated
- **THEN** it contains `waitForServer()` async function
- **AND** a `main()` async function that calls each interface method
- **AND** assertions that validate responses

### Requirement: Test file naming

Test files SHALL be named consistently: `test_server.{py,ts}` and `test_client.{py,ts}` and placed in the same directory as generated output files.

#### Scenario: Test file location
- **WHEN** `-dir ./output -generate-test-files` is used
- **THEN** `test_server.py` and `test_client.py` are at `./output/`
- **AND** they reference `idl.json` at `./output/idl.json`

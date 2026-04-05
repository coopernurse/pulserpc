## ADDED Requirements

### Requirement: Client.ready() async initialization

The TypeScript `Client` SHALL provide a `ready()` method that returns a Promise, ensuring the client has fetched IDL and created interface proxies before use.

#### Scenario: Client ready method
- **WHEN** `Client` is instantiated with `HttpTransport`
- **THEN** user SHOULD call `await client.ready()` before making RPC calls
- **AND** interface proxies are available after ready resolves

#### Scenario: Client ready before calls
- **WHEN** `await client.ready()` completes successfully
- **THEN** `client.CatalogService.listProducts()` can be called

### Requirement: Contract class for IDL loading

The TypeScript runtime SHALL provide a `Contract` class that parses IDL JSON and provides `validateRequest` and `validateResponse` methods.

#### Scenario: Contract instantiation
- **WHEN** `const contract = new Contract(idlData)` is called with IDL JSON
- **THEN** `contract.interfaces` contains all interface definitions
- **AND** `contract.structs` and `contract.enums` are populated

#### Scenario: Validate request
- **WHEN** `contract.validateRequest(ifaceName, funcName, params)` is called
- **THEN** it validates parameter count and types against IDL
- **AND** throws error if validation fails

### Requirement: Server validate options

The TypeScript `Server` constructor SHALL accept `validateRequests` and `validateResponses` options.

#### Scenario: Server with validation
- **WHEN** `new Server({ contract, validateRequests: true, validateResponses: true })` is called
- **THEN** incoming requests are validated against IDL
- **AND** outgoing responses are validated before sending

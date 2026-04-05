## ADDED Requirements

### Requirement: InProcTransport availability

The Python runtime SHALL export `InProcTransport` class for in-process RPC calls without network overhead.

#### Scenario: InProcTransport import
- **WHEN** user imports from pulserpc runtime
- **THEN** `from pulserpc import InProcTransport` succeeds

#### Scenario: InProcTransport usage
- **WHEN** `InProcTransport` is passed to `Client`
- **THEN** RPC calls are handled in-process without HTTP

### Requirement: Client validation flags

The `Client` constructor SHALL accept `validate_request` and `validate_response` boolean flags to enable validation against IDL definitions.

#### Scenario: Client with validation
- **WHEN** `Client(transport, validate_request=True, validate_response=True)` is instantiated
- **THEN** requests are validated before sending
- **AND** responses are validated after receiving

### Requirement: Notify method for fire-and-forget

The Python `Client` SHALL provide a `notify(method, params)` method for sending JSON-RPC notifications that do not expect a response.

#### Scenario: Send notification
- **WHEN** `client.notify("Service.logEvent", {"event": "login"})` is called
- **THEN** a JSON-RPC notification is sent without waiting for response
- **AND** no RPCError is raised regardless of server behavior

### Requirement: Auto-discovery via pulserpc-idl

The Python `Client` SHALL automatically fetch IDL from the server using the `pulserpc-idl` RPC method on initialization.

#### Scenario: Client bootstrap
- **WHEN** `Client(transport)` is instantiated
- **THEN** it makes a `pulserpc-idl` request to get IDL JSON
- **AND** creates interface proxies for each interface

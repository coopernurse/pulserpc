## ADDED Requirements

### Requirement: Python 2.7 compatible runtime library

The `runtimes/python2/pulserpc/` directory SHALL contain a Python 2.7-compatible runtime library with no Python 3 syntax.

#### Scenario: Runtime files are Python 2 compatible
- **WHEN** the runtime files are examined
- **THEN** no f-strings are present
- **AND** no `@dataclass` decorators are present
- **AND** no `typing` module imports are present
- **AND** no `abc.ABC` base class syntax is present
- **AND** `urllib2` is used instead of `urllib.request`

### Requirement: Validation supports PulseRPC IDL types

The runtime SHALL validate all PulseRPC types according to the IDL schema:

#### Scenario: String validation
- **WHEN** `validate_string("hello")` is called
- **THEN** it returns without error
- **WHEN** `validate_string(123)` is called
- **THEN** it raises `TypeError`

#### Scenario: Integer validation
- **WHEN** `validate_int(42)` is called
- **THEN** it returns without error
- **WHEN** `validate_int(3.14)` is called
- **THEN** it raises `TypeError`
- **WHEN** `validate_int("42")` is called
- **THEN** it raises `TypeError`

#### Scenario: Float validation
- **WHEN** `validate_float(3.14)` is called
- **THEN** it returns without error
- **WHEN** `validate_float(42)` is called
- **THEN** it returns without error (int is allowed for float)
- **WHEN** `validate_float("3.14")` is called
- **THEN** it raises `TypeError`

#### Scenario: Boolean validation
- **WHEN** `validate_bool(True)` is called
- **THEN** it returns without error
- **WHEN** `validate_bool(1)` is called
- **THEN** it raises `TypeError`

#### Scenario: Array validation with element type
- **WHEN** `validate_array([1, 2, 3], element_validator=validate_int)` is called
- **THEN** it returns without error
- **WHEN** `validate_array([1, "two", 3], element_validator=validate_int)` is called
- **THEN** it raises `ValueError` at index 1

#### Scenario: Struct validation with required fields
- **WHEN** a struct with fields `{"name": string, "age": int}` is validated with `{"name": "Alice", "age": 30}`
- **THEN** it returns without error
- **WHEN** the struct is validated with `{"name": "Alice"}` (missing required field)
- **THEN** it raises `ValueError` about missing required field
- **WHEN** the struct is validated with `{"name": "Alice", "age": "thirty"}`
- **THEN** it raises `ValueError` about type mismatch

#### Scenario: Struct validation with optional fields
- **WHEN** a struct with fields `{"name": string, "nickname": string (optional)}` is validated with `{"name": "Alice"}`
- **THEN** it returns without error
- **WHEN** the struct is validated with `{"name": "Alice", "nickname": null}`
- **THEN** it returns without error
- **WHEN** the struct is validated with `{"name": "Alice", "nickname": "Ali"}`
- **THEN** it returns without error

#### Scenario: Enum validation
- **WHEN** an enum with values `["pending", "paid", "shipped"]` is validated with `"paid"`
- **THEN** it returns without error
- **WHEN** the enum is validated with `"unknown"`
- **THEN** it raises `ValueError` about invalid enum value

#### Scenario: Nested struct validation
- **WHEN** a struct A contains a field of type struct B, and struct B is validated with correct structure
- **THEN** it returns without error
- **WHEN** struct A is validated with malformed struct B inside
- **THEN** it raises `ValueError` describing the nested validation failure

#### Scenario: Array of user-defined types
- **WHEN** an array field is declared as `array<User>` and validated with `[{...}, {...}]`
- **THEN** each element is validated against the User struct
- **WHEN** one element is invalid
- **THEN** validation fails at that element's index

### Requirement: Contract validates requests and responses

The `Contract` class SHALL validate JSON-RPC requests and responses against the IDL.

#### Scenario: Valid request validation
- **WHEN** `contract.validate_request("ServiceName", "methodName", [{"param": "value"}])` is called
- **AND** the IDL defines the method with matching parameters
- **THEN** it returns without error

#### Scenario: Invalid request - wrong parameter count
- **WHEN** `contract.validate_request("ServiceName", "methodName", [param1, param2])` is called
- **AND** the IDL expects 3 parameters
- **THEN** it raises `ValueError`

#### Scenario: Valid response validation
- **WHEN** `contract.validate_response("ServiceName", "methodName", result)` is called
- **AND** the result matches the return type in the IDL
- **THEN** it returns without error

#### Scenario: Invalid response - type mismatch
- **WHEN** `contract.validate_response("ServiceName", "methodName", result)` is called
- **AND** the result type does not match the IDL
- **THEN** it raises `ValueError`

### Requirement: Server dispatches JSON-RPC requests

The `Server` class SHALL dispatch JSON-RPC 2.0 requests to registered handlers.

#### Scenario: Handler registration
- **WHEN** `server.add_handler("ServiceName", handler_instance)` is called
- **AND** the handler has a method matching an interface method
- **THEN** the handler is registered for that interface

#### Scenario: Single request dispatch
- **WHEN** `server.call({"jsonrpc": "2.0", "method": "Service.method", "params": {...}, "id": 1})` is called
- **AND** a handler is registered for "Service"
- **THEN** the handler method is invoked with params
- **AND** a JSON-RPC response is returned

#### Scenario: Named params dispatch
- **WHEN** a request with named params `{"method": "Service.method", "params": {"arg1": "value"}}` is dispatched
- **THEN** the handler method is called with `**params` (keyword arguments)

#### Scenario: Positional params dispatch
- **WHEN** a request with positional params `{"method": "Service.method", "params": ["value"]}` is dispatched
- **THEN** the params are converted to named params using the IDL signature
- **AND** the handler method is called with keyword arguments

#### Scenario: Request validation enabled
- **WHEN** `Server(contract, validate_requests=True)` is configured
- **AND** a request with invalid params is received
- **THEN** an error response is returned with code -32602

#### Scenario: Response validation enabled
- **WHEN** `Server(contract, validate_responses=True)` is configured
- **AND** a handler returns a value that doesn't match the IDL return type
- **THEN** an error response is returned with code -32603

#### Scenario: InProcTransport invokes server directly
- **WHEN** `InProcTransport(server).request(request)` is called
- **THEN** it calls `server.call(request)` directly

### Requirement: HttpTransport makes HTTP requests

The `HttpTransport` class SHALL make HTTP requests using urllib2.

#### Scenario: HTTP POST request
- **WHEN** `HttpTransport(url).request(request)` is called
- **THEN** a POST request is sent to the URL with JSON-encoded body
- **AND** the Content-Type header is set to "application/json"
- **AND** the response is parsed as JSON and returned

#### Scenario: HTTP error handling
- **WHEN** the HTTP server returns an error status
- **THEN** `URLError` or `HTTPError` is raised

### Requirement: Client fetches IDL and creates proxies

The `Client` class SHALL connect to a server, fetch the IDL, and create interface proxies.

#### Scenario: Client initialization
- **WHEN** `Client(transport)` is created
- **THEN** it fetches the IDL via `pulserpc-idl` method
- **AND** creates proxy objects for each interface
- **AND** proxy objects are accessible as attributes (e.g., `client.MyService`)

#### Scenario: Proxy method call
- **WHEN** `client.MyService.getUser(id=123)` is called
- **THEN** a JSON-RPC request is sent with method "MyService.getUser"
- **AND** params are `{"id": 123}`
- **AND** the result is returned

### Requirement: Batch requests supported

The `Batch` class SHALL allow multiple requests to be batched.

#### Scenario: Batch creation
- **WHEN** `client.start_batch()` is called
- **THEN** a `Batch` object is returned with interface proxies attached

#### Scenario: Batch add and send
- **WHEN** requests are added to a batch via proxy calls
- **AND** `batch.send()` is called
- **THEN** all requests are sent in a single HTTP request
- **AND** results are returned in order

### Requirement: Filter hooks for pre/post processing

The `Filter` class SHALL provide pre and post hook points.

#### Scenario: Pre-filter can set error
- **WHEN** a filter's `pre(context)` sets `context.error`
- **THEN** the request handler is not invoked
- **AND** the error response is returned

#### Scenario: Post-filter can inspect response
- **WHEN** a handler returns successfully
- **THEN** all filters' `post(context)` methods are called
- **AND** filters can inspect `context.response`

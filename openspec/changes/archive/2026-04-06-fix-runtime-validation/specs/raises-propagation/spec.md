## ADDED Requirements

### Requirement: Server methods with raises() clause must declare error types

When a method is defined with a `raises(ErrorType)` clause in the IDL, the generated server stub SHALL include the error type in its signature or documentation so the implementor knows which errors to raise.

#### Scenario: Method with raises() generates error documentation
- **WHEN** the IDL defines a method `A.divide(a int, b int) float raises(ValidationError)`
- **AND** code is generated for Python runtime
- **THEN** the generated server code SHALL include `raises ValidationError` in the method signature or docstring

### Requirement: Server implementation catching raises() errors must return RPCError

When a server implementation raises an error type that is listed in the method's `raises()` clause, the runtime SHALL catch the exception and return a JSON-RPC error response with the error code and message.

#### Scenario: Server raises ValidationError for method with raises clause
- **WHEN** the IDL defines `A.divide(a int, b int) float raises(ValidationError)`
- **AND** the Python server implementation raises `ValidationError("division by zero")`
- **THEN** the runtime SHALL return a JSON-RPC error response with `{"code": <error-code>, "message": "division by zero", "data": null}`

#### Scenario: Server raises non-listed error type
- **WHEN** the IDL defines `A.divide(a int, b int) float raises(ValidationError)`
- **AND** the Python server implementation raises `ValueError("unexpected value")` which is NOT in the raises clause
- **THEN** the runtime MAY return a generic internal error or the error as-is (implementation-defined)

### Requirement: All runtimes must have RPCError class

Each runtime SHALL provide an `RPCError` class or equivalent error representation for JSON-RPC error responses.

#### Scenario: Go has RPCError
- **WHEN** the Go runtime receives an error condition
- **THEN** it SHALL be able to return `pulserpc.RPCError` with `Code`, `Message`, and `Data` fields

#### Scenario: Python has RPCError
- **WHEN** the Python runtime receives an error condition
- **THEN** it SHALL be able to return `RPCError` with `code`, `message`, and `data` attributes

#### Scenario: TypeScript has RPCError
- **WHEN** the TypeScript runtime receives an error condition
- **THEN** it SHALL be able to return `RPCError` with `code`, `message`, and `data` properties

#### Scenario: C# has RPCError
- **WHEN** the C# runtime receives an error condition
- **THEN** it SHALL be able to return `RPCError` with `Code`, `Message`, and `Data` properties

#### Scenario: Java has RPCError
- **WHEN** the Java runtime receives an error condition
- **THEN** it SHALL be able to return `RPCError` with `code`, `message`, and `data` fields

### Requirement: RPCError must be returned for raises() errors

When a method with a `raises()` clause is called and the implementation raises the specified error, the runtime SHALL catch the error and return an RPCError to the client.

#### Scenario: Go server returns RPCError for raises clause
- **WHEN** a Go server method has `raises(ValidationError)` in its IDL definition
- **AND** the implementation returns an error that matches one of the declared error types
- **THEN** the Go runtime SHALL return a JSON-RPC error response with the error code and message

#### Scenario: Python server returns RPCError for raises clause
- **WHEN** a Python server method has `raises(ValidationError)` in its IDL definition
- **AND** the implementation raises an exception that is an instance of the declared error type
- **THEN** the Python runtime SHALL return a JSON-RPC error response with the error code and message

#### Scenario: TypeScript server returns RPCError for raises clause
- **WHEN** a TypeScript server method has `raises(ValidationError)` in its IDL definition
- **AND** the implementation throws an error that is an instance of the declared error type
- **THEN** the TypeScript runtime SHALL return a JSON-RPC error response with the error code and message

#### Scenario: C# server returns RPCError for raises clause
- **WHEN** a C# server method has `raises(ValidationError)` in its IDL definition
- **AND** the implementation throws an exception that matches one of the declared error types
- **THEN** the C# runtime SHALL return a JSON-RPC error response with the error code and message

#### Scenario: Java server returns RPCError for raises clause
- **WHEN** a Java server method has `raises(ValidationError)` in its IDL definition
- **AND** the implementation throws an exception that is an instance of the declared error type
- **THEN** the Java runtime SHALL return a JSON-RPC error response with the error code and message

### Requirement: Enum values are case-sensitive across all runtimes

When validating an enum field, the runtime SHALL enforce case-sensitive matching of enum values.

#### Scenario: Lowercase enum value accepted
- **WHEN** the IDL defines an enum `MathOp` with values `add`, `multiply`
- **AND** the JSON value is `"add"`
- **THEN** validation SHALL pass

#### Scenario: Wrong case enum value rejected
- **WHEN** the IDL defines an enum `MathOp` with values `add`, `multiply`
- **AND** the JSON value is `"Add"` or `"ADD"`
- **THEN** validation SHALL fail with an error indicating the value is not a valid enum value
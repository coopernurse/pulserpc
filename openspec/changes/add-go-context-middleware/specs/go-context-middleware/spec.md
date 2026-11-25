## ADDED Requirements

### Requirement: Generated Go handlers receive context first and always return an error

Every generated Go interface method SHALL take `ctx context.Context` as its
first parameter, before all IDL-declared parameters, and SHALL expose an error
result: `(T, error)` when the IDL declares a return type, and `error` when it
does not. Generated client methods and generated test handlers SHALL use the
same signature.

#### Scenario: Generated interface includes context and error
- **WHEN** the Go generator emits an interface for `interface Svc { Ping(msg string) string }`
- **THEN** the generated interface contains `Ping(ctx context.Context, msg string) (string, error)`
- **AND** the generated client contains `func (c *SvcClient) Ping(ctx context.Context, msg string) (string, error)`

#### Scenario: Void method returns an error
- **WHEN** the IDL declares `Notify(msg string)` with no return type
- **THEN** the generated interface contains `Notify(ctx context.Context, msg string) error`

#### Scenario: Generated test server includes context and error
- **WHEN** code is generated with `-generate-test-files`
- **THEN** every generated handler implementation contains `ctx context.Context` as its first parameter and the declared error result
- **AND** the generated test client passes a context to every call

#### Scenario: Context is not a wire parameter
- **WHEN** a request reaches the runtime with IDL parameters `[a, b]`
- **THEN** the handler is invoked as `handler.Method(ctx, a, b)`
- **AND** the IDL arity check and validation still count only `a` and `b`

### Requirement: Registration rejects handlers without context or error return

`Server.AddHandler` and the generated `Register` SHALL return an error when a
handler method that implements a registered interface method does not accept a
leading `context.Context` or does not return the declared error result. The
runtime SHALL NOT invoke a handler method without supplying a non-nil context.

#### Scenario: Valid handler registers
- **WHEN** `AddHandler("A", &AImpl{})` is called and every exported method starts with `context.Context` and returns the declared error shape
- **THEN** registration succeeds and returns nil

#### Scenario: Legacy handler is rejected at registration
- **WHEN** `AddHandler("A", &LegacyAImpl{})` is called and `Add` does not take a context
- **THEN** registration returns an error naming the offending method
- **AND** the handler is not stored

#### Scenario: Value-only handler is rejected at registration
- **WHEN** `AddHandler("A", &ValueOnlyAImpl{})` is called and `ListProducts` returns only a value
- **THEN** registration returns an error naming the offending method
- **AND** the handler is not stored

#### Scenario: Called handler always receives context
- **WHEN** a request is processed through `Server.Call(request)`
- **THEN** the handler receives `context.Background()`
- **WHEN** a request is processed through `Server.CallWithContext(ctx, request)`
- **THEN** the handler receives `ctx`

### Requirement: Context flows through generated clients and transports

Generated Go client methods SHALL take `ctx context.Context` first and pass it
to the transport. The generated `Transport.Call` and `NotifyTransport.Notify`
SHALL take a context, and the generated HTTP transport SHALL use
`http.NewRequestWithContext`.

#### Scenario: Client context reaches the HTTP request
- **WHEN** a caller invokes `client.Ping(ctx, "hi")`
- **THEN** the generated HTTP transport sends the request with `ctx`
- **AND** cancelling `ctx` aborts the in-flight request

#### Scenario: Notification carries context
- **WHEN** a caller invokes `client.Notify(ctx, "Svc.ping", params)`
- **THEN** the transport sends the notification with `ctx`

#### Scenario: Cancelled first call cannot skip bootstrap verification
- **WHEN** the first client call has a cancelled context and `VerifyOnBootstrap` is enabled
- **THEN** the one-shot verification still runs with the caller's values but without its cancellation

### Requirement: HTTP middleware hook

The generated server SHALL expose an HTTP middleware hook that receives the
concrete `http.ResponseWriter` and `*http.Request`, and returns a context and a
boolean continue flag. The adapter SHALL install its request body cap before
invoking the hook. Returning `false` SHALL stop processing without the adapter
writing a second response.

#### Scenario: Header-derived context reaches the handler
- **WHEN** HTTP middleware stores a header value with `context.WithValue`
- **THEN** the RPC middleware and the handler receive a context containing that value

#### Scenario: HTTP middleware aborts authentication
- **WHEN** HTTP middleware writes a 401 response and returns `false`
- **THEN** the JSON-RPC body is not parsed
- **AND** no handler is invoked

#### Scenario: Nil context falls back to request context
- **WHEN** HTTP middleware returns `nil, true`
- **THEN** the adapter uses `r.Context()`

#### Scenario: Middleware body reads are bounded
- **WHEN** HTTP middleware reads `r.Body`
- **THEN** the read is subject to the adapter's 1 MiB request cap

#### Scenario: Derived context reaches the next HTTP hook
- **WHEN** the first HTTP middleware returns a derived context
- **THEN** the next hook receives `r.WithContext` containing that context

### Requirement: RPC middleware hook

`Server` SHALL expose a `UseRPC` hook invoked once per JSON-RPC request, after
envelope validation and before `pulserpc-idl` handling, parameter validation,
and handler dispatch. It SHALL receive and return a context, request, and
response.

#### Scenario: Middleware can rewrite context and request
- **WHEN** RPC middleware returns a derived context and a request with a normalized parameter
- **THEN** the handler receives the derived context and the normalized parameter

#### Scenario: Middleware short-circuits dispatch
- **WHEN** RPC middleware returns a non-nil response
- **THEN** the handler is not invoked
- **AND** the returned response is sent as the JSON-RPC response

#### Scenario: Notification suppression wins
- **WHEN** RPC middleware returns a response for a notification
- **THEN** no response is written

#### Scenario: Middleware panic is contained
- **WHEN** RPC middleware panics
- **THEN** the server returns `-32603` with `Internal error`
- **AND** the panic value is not sent to the client

#### Scenario: Batch invokes middleware per member
- **WHEN** an HTTP batch contains three JSON-RPC requests
- **THEN** the HTTP middleware runs once
- **AND** the RPC middleware runs three times
- **AND** each member receives a context derived from the HTTP-derived context

### Requirement: Go wire behavior is unchanged

Adding context and middleware SHALL NOT change any JSON-RPC envelope, error
code, validation result, or serialized response.

#### Scenario: Golden responses are unchanged
- **WHEN** the golden response suite runs for Go
- **THEN** every captured response is byte-identical to the pre-change baseline

#### Scenario: Conformance is unchanged
- **WHEN** the multi-runtime conformance suite runs against the generated Go server
- **THEN** all existing request/response expectations pass

### Requirement: Go documentation demonstrates context and middleware

The Go reference, quickstart includes, and reference implementation under
`examples/quickstart/go` SHALL show `ctx` as the first handler parameter and
demonstrate both middleware hooks, including a header-to-context authentication
example.

#### Scenario: Quickstart server builds with context
- **WHEN** the Go quickstart server and client are built from the updated examples
- **THEN** both compile and the quickstart test passes

#### Scenario: Reference documents registration errors
- **WHEN** a reader follows the Go reference's server section
- **THEN** they see `Register` returning an error and `ctx` on every handler

## Context

Today the Go generated server is a thin wrapper over `pulserpc.Server`:

- `PulseRPCServer.handleRequest` reads and parses the HTTP body, then calls
  `s.rpcServer.Call(reqMap)`.
- `Server.Call` resolves the handler method by reflection and builds the
  argument list from IDL parameter metadata. There is no context parameter and
  no hook between the HTTP request and handler dispatch.
- Generated clients call `Transport.Call(method, params)`, and the generated
  `HTTPTransport` uses `http.NewRequest`, so a caller context cannot cancel or
  bound a request.
- `AddHandler` accepts `interface{}` and performs no validation, so the
  generated interface stub is advisory only.

TypeScript and Python already pass a context as the first handler argument.
TypeScript's generated client methods do **not** take a context, and no runtime
has middleware. This change therefore adds two capabilities to Go rather than
mirroring an existing implementation.

## Goals / Non-Goals

**Goals:**

- Make `context.Context` the first parameter of every generated Go handler and
  client method.
- Let a caller context flow through the generated transport to the server.
- Provide two middleware layers: HTTP-level (headers/auth, abort before parse)
  and RPC-level (per JSON-RPC request, context/request/response transformation).
- Fail fast when a registered handler does not accept a leading context.
- Keep the JSON-RPC wire format and all golden/conformance responses identical.

**Non-Goals:**

- Adding client-side middleware or interceptors.
- Changing the JSON-RPC envelope, validation rules, or error codes.
- Mapping `context.Canceled`/`context.DeadlineExceeded` to a new JSON-RPC error
  code. A handler that returns one is treated like any other undeclared error
  (`-32603` / `Internal error`) unless it returns an `RPCError`.
- Changing other language runtimes.

## Decisions

### Handler context and error return are required

Generated interfaces always expose an error return, matching the generated
client and the runtime's error handling:

```go
type CatalogService interface {
    ListProducts(ctx context.Context) ([]*Product, error)
    GetProduct(ctx context.Context, productId string) (*Product, error)
    DeleteProduct(ctx context.Context, productId string) error
}
```

- A method with a declared IDL return type returns `(T, error)`.
- A method with no declared return type returns `error`.
- Optional return types keep their existing pointer/optional representation and
  return `nil, nil` when absent.

This corrects an existing generator inconsistency: `writeInterfaceStubGo`
emitted only the value type while `writeClientMethodGo` and
`writeTestMethodImplGo` emitted `(T, error)`, so an implementation following the
documentation could not satisfy the generated interface.

`Server.AddHandler` and the generated `Register` return `error` and validate the
full handler shape for every method that implements a method of the registered
interface: a leading `context.Context` and the declared error result
(`(T, error)` for value-returning methods, `error` for void methods). Methods
unrelated to the interface are ignored. Registration is deliberately the
validation point rather than call time: a misconfigured server fails at startup
instead of returning `-32603` on the first request. Changing the return type
does not break existing statement-form calls such as
`server.Register("A", &AImpl{})`; Go permits discarding all results of a call
used as a statement.

**Alternatives considered:**

- *Accept both ctx and legacy handlers.* Rejected: the user wants ctx required,
  and silently skipping it would make cancellation unreliable.
- *Allow value-only handlers.* Rejected: the generated interface would again be
  unsatisfiable, and handlers could not report errors the way the runtime
  already supports.
- *Panic at registration.* Rejected: a library should not panic for a
  recoverable configuration error.
- *Reject per call with `-32603`.* Rejected: hides a startup misconfiguration
  until production traffic.

### Context-aware dispatch keeps the wire contract

`Server.CallWithContext(ctx, request)` is the primary entry point.
`Server.Call(request)` remains and delegates with `context.Background()`.
`invokeHandler` detects the leading `context.Context`, supplies it, and offsets
IDL parameter metadata by one. Validation remains IDL-only: `ctx` is never a
wire parameter, never counted in arity, and never validated.

The generated adapter calls `CallWithContext(r.Context(), reqMap)`, so client
cancellation and HTTP server timeouts reach handlers.

### Client and transport take a context

```go
type Transport interface {
    Call(ctx context.Context, method string, params []interface{}) (map[string]interface{}, error)
}

type NotifyTransport interface {
    Notify(ctx context.Context, method string, params []interface{}) error
}
```

Generated client methods take `ctx context.Context` first and pass it to
`Transport.Call`. `HTTPTransport` uses `http.NewRequestWithContext`.

`ensureBootstrapVerification` receives the caller's context because its IDL
fetch is a real RPC; the first caller to trigger verification controls that
request.

**Alternative considered:** adding `CallWithContext` as an optional transport
capability while keeping `Call`. Rejected: the generated transport is generated
code, so there is no long-lived third-party transport contract to preserve, and
a single signature avoids a permanently split API.

### Two middleware hooks

HTTP middleware runs once per HTTP request, before the method check, body read,
and JSON parsing:

```go
// In the generated server package.
type HTTPMiddleware func(w http.ResponseWriter, r *http.Request) (context.Context, bool)
```

- It can read headers, derive a context, and write an HTTP response.
- Returning `false` aborts: the adapter writes nothing further.
- Returning a nil context falls back to `r.Context()`.

RPC middleware runs once per JSON-RPC request (per batch member):

```go
type Middleware func(
    ctx context.Context,
    request map[string]interface{},
    response map[string]interface{},
) (context.Context, map[string]interface{}, map[string]interface{})
```

- `response` is `nil` on entry. A non-nil returned response short-circuits
  dispatch and is sent as-is (after notification suppression).
- The returned context is used for handler invocation and response validation.
- The returned request is used for dispatch, so middleware can normalize or
  redact parameters.
- It runs after envelope validation (`id`, `jsonrpc`, `method`) so it can trust
  the method/id, and before `pulserpc-idl` handling and parameter validation so
  it can protect the IDL endpoint and reject unauthenticated callers before
  validation detail is produced.
- A panic in middleware is recovered and mapped to `-32603`, logged by type,
  matching handler panic behavior.

**Why two hooks:** header/auth policy belongs to the concrete HTTP transport;
JSON-RPC concerns (method-level authorization, request/response rewriting) belong
one layer down. Combining them forces the RPC layer to know about HTTP types or
forces the HTTP layer to duplicate JSON-RPC envelope handling.

**Batch semantics:** the HTTP hook runs once. The RPC hook runs once per member,
each with a child context derived from the HTTP-derived context, so one member's
cancellation does not cancel its siblings unless the parent is cancelled.

### Middleware ordering and failure behavior

1. HTTP middleware
2. method/POST/body-size checks
3. JSON parse and envelope validation
4. RPC middleware
5. `pulserpc-idl` or parameter validation
6. handler
7. response validation
8. serialization

A middleware returned response is never re-validated or re-encoded through the
handler path; it is treated as the final JSON-RPC response. For notifications it
is discarded to preserve the "notifications never receive a response" rule.

## Risks / Trade-offs

- **[Breaking generated API]** Every generated Go handler and client call site
  must add `ctx`. Mitigation: document in the reference and release notes; the
  generator tests build a generated module so stale signatures fail in CI.
- **[Registration return value]** `AddHandler`/`Register` now return `error`.
  Mitigation: statement-form calls remain valid; docs show checking it.
- **[Context values are an escape hatch]** Middleware can attach arbitrary
  values to `ctx`. Mitigation: document typed-key helpers in examples; do not
  introduce an untyped header map.
- **[Middleware can mutate requests/responses]** A buggy middleware can break
  the wire contract. Mitigation: middleware only runs before dispatch and the
  returned response is passed through unchanged; tests cover short-circuit,
  notification, panic, and batch behavior.
- **[Local tooling]** Windows Application Control has blocked some locally built
  Go test binaries in this environment. Final verification relies on CI, as in
  previous changes; local checks that can run must not be reported as passing
  when they did not run.

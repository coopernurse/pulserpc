## Why

The Go generator and runtime do not support `context.Context` in interface handlers, so a handler cannot honor cancellation or a deadline for downstream calls. The generated client also has no way to carry a caller context through the transport. Every other generated server runtime (Python 3, Python 2, TypeScript) already receives a per-request context as the first handler argument.

There is also no supported per-request hook for transport-level concerns. Implementations that need authentication, header extraction, or request rejection must either hand-write the HTTP adapter or wrap the generated server externally, which is not possible with the generated `ServeForever` path.

## What Changes

- Add `ctx context.Context` as the first parameter of every generated Go interface method, client method, and generated test handler.
- Make generated interface methods return the Go-idiomatic shape: `(T, error)` when the IDL declares a return type and `error` when it does not. Today the interface omits `error` while the client and generated test handler include it, so the interface cannot be satisfied by the documented implementations.
- Require handlers registered through `AddHandler`/`Register` to accept a leading `context.Context` and return the declared error shape; registration returns an error otherwise.
- Add `context.Context` to the generated `Transport`/`NotifyTransport` contracts and use `http.NewRequestWithContext` in the generated `HTTPTransport`.
- Add runtime `Server.CallWithContext(ctx, request)` and keep `Server.Call(request)` as a background-context convenience.
- Add an RPC middleware hook (`UseRPC`) invoked once per JSON-RPC request (per batch member), after envelope validation and before parameter validation/dispatch. It can derive the context, rewrite the request, or return a non-nil response to short-circuit dispatch.
- Add an HTTP middleware hook (`UseHTTP`) on the generated server that receives the concrete `http.ResponseWriter` and `*http.Request`, so it can read headers, abort with an HTTP response, and return the context used for the JSON-RPC call.
- Update Go documentation, quickstart includes, and reference implementations to show `ctx` and both middleware hooks.
- Record the breaking generated-API change in the release notes and ship it as `v0.4.0`.

## Capabilities

### New Capabilities

- `go-context-middleware`: Go handler context propagation plus HTTP- and RPC-level middleware hooks.

### Modified Capabilities

- None.

## Impact

**Affected code:**
- `pkg/runtime/runtimes/go/pulserpc/server.go` — context-aware dispatch, middleware, registration validation.
- `pkg/runtime/runtimes/go/pulserpc/client.go` and `transport.go` — context on calls.
- `pkg/generator/go_client_server.go` — interface, client, transport, test server/client, generated HTTP adapter.
- `pkg/generator/go_*_test.go` — signature expectations and new generator coverage.
- `examples/quickstart/go/{server,client}.go` — ctx-aware reference implementation.
- `examples/cross-ns` generated builds — must still compile.

**Documentation:**
- `docs/languages/go/reference.md`
- `docs/languages/go/quickstart.md`
- `docs/_includes/quickstart/go-server.md`
- `docs/_includes/quickstart/go-client.md`

**Compatibility:**
- Generated Go interfaces and client methods gain a leading `ctx` parameter, and interface methods now return `(T, error)` / `error`; downstream handler/client implementations must be updated.
- `AddHandler` and generated `Register` return `error`; existing statement-form calls still compile.
- The JSON-RPC wire format is unchanged; golden responses and conformance fixtures must not change.

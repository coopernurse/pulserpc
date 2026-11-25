## 1. Runtime: context-aware dispatch

- [x] 1.1 Add `CallWithContext(ctx context.Context, request map[string]interface{}) map[string]interface{}` to `pkg/runtime/runtimes/go/pulserpc/server.go`; make `Call` delegate with `context.Background()`.
- [x] 1.2 Change `invokeHandler` to detect a leading `context.Context`, pass the supplied context, and offset IDL parameter metadata by one; leave validation and arity IDL-only.
- [x] 1.3 Change `Server.AddHandler` to `AddHandler(ifaceName string, handler interface{}) error`; validate every handler method that implements a registered interface method: leading `context.Context` and the declared error shape (`(T, error)` for value-returning methods, `error` for void methods), rejecting with a method-naming error. Ignore unrelated methods.
- [x] 1.4 Add a defensive call-time check that returns `-32603` if a handler method lacks a context (registration should make this unreachable).

## 2. Runtime: RPC middleware

- [x] 2.1 Add `type Middleware func(ctx context.Context, request map[string]interface{}, response map[string]interface{}) (context.Context, map[string]interface{}, map[string]interface{})` and `Server.UseRPC`.
- [x] 2.2 Invoke the hook once per JSON-RPC request after envelope validation and before `pulserpc-idl`/parameter validation/dispatch; apply returned context and request.
- [x] 2.3 Short-circuit on a non-nil returned response; discard it for notifications; recover middleware panics as `-32603` and log only the type.
- [x] 2.4 Add tests covering context values, deadlines, request rewriting, abort, notification suppression, panic containment, `Call` vs `CallWithContext`, and per-member batch invocation.

## 3. Runtime: client and transport context

- [x] 3.1 Update `pkg/runtime/runtimes/go/pulserpc/client.go` and `transport.go` so `Transport.Request`/`RequestWithID`, `Client.Call`, and `Notify` accept a context and use `http.NewRequestWithContext`; keep the request-id and envelope logic unchanged.
- [x] 3.2 Update runtime tests and any in-repo callers for the new signatures.
- [x] 3.3 Add a test that a cancelled context aborts an HTTP call and a test that a deadline reaches the server.

## 4. Generator: signatures and generated server

- [x] 4.1 Add `ctx context.Context` as parameter zero and an `error` result in `writeInterfaceStubGo` (`(T, error)` for value-returning methods, `error` for void methods); apply the same signature to `writeClientMethodGo` and generated test handlers/clients.
- [x] 4.2 Change generated `Transport.Call` and `NotifyTransport.Notify` to take `ctx context.Context`; update `HTTPTransport` to `http.NewRequestWithContext`.
- [x] 4.3 Pass the caller context through `ensureBootstrapVerification` and generated client calls.
- [x] 4.4 Generate `PulseRPCServer.Register(...) error`, `UseRPC(pulserpc.Middleware)`, `UseHTTP(HTTPMiddleware)`, and `CallWithContext`.
- [x] 4.5 Generate `type HTTPMiddleware func(w http.ResponseWriter, r *http.Request) (context.Context, bool)` and run it at the top of `handleRequest`, before method/body/parse checks; use the returned context for single and batch calls.
- [x] 4.6 Ensure generated imports include `context` everywhere it is now referenced and no longer includes unused imports.

## 5. Generator: tests and fixtures

- [x] 5.1 Add `pkg/generator/go_context_test.go` asserting `ctx context.Context` appears first and `error` appears in every interface/client/test signature, and that a generated module builds.
- [x] 5.2 Update exact-signature assertions in `go_identifiers_wave6_test.go`, `go_import_interface_regression_test.go`, and any other Go generator tests.
- [x] 5.3 Extend the cross-namespace fixture test to build the generated Go output with the new signatures.
- [x] 5.4 Run `gofmt -l` and `go vet` on changed Go files.
- [x] 5.5 Add runtime tests that a value-only handler and a no-context handler are both rejected at registration.

## 6. Documentation and reference implementations

- [x] 6.1 Update `docs/languages/go/reference.md`: server and client examples with `ctx`, registration error handling, a "Context and Middleware" section with `UseHTTP` header/auth and `UseRPC` examples, and a migration note.
- [x] 6.2 Update `docs/languages/go/quickstart.md` and `docs/_includes/quickstart/go-server.md` / `go-client.md`.
- [x] 6.3 Update `examples/quickstart/go/server.go` and `client.go`; include a header-derived context and an RPC middleware example that does not change wire behavior.
- [x] 6.4 Check `docs/RUNTIME_IMPLEMENTATION_GUIDE.md` and any other generated-Go snippets for stale signatures.
- [ ] 6.5 Run the docs link check.

## 7. Integration and verification

- [x] 7.1 Run `go test ./pkg/generator ./pkg/runtime/...` with `-race` where supported.
- [ ] 7.2 Run `tests/integration/test_quickstart_go.sh` (Docker) and the Go generator integration script.
- [ ] 7.3 Run the multi-runtime conformance and golden suites; confirm no wire changes.
- [x] 7.4 Confirm `go test ./...` and `go build ./...` in a generated module outside the checkout.
- [ ] 7.5 Push and require green `CI`, `CI Conformance`, and `Quickstart guides` checks.

## 8. Release

- [ ] 8.1 Add a migration/breaking-change note for generated Go signatures and `Register`/`AddHandler` returning `error`.
- [ ] 8.2 Bump `cmd/pulse/version.go` to `v0.4.0`, tag, push, and verify release assets and workflow results.

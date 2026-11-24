package pulserpc

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"testing"
	"time"
)

// contextTestIDL declares the methods used by the context/middleware tests.
const contextTestIDL = `{
	"interfaces": [{
		"name": "Ctx",
		"methods": [
			{"name":"ping","parameters":[],"returnType":{"builtIn":"string"}},
			{"name":"echo","parameters":[{"name":"name","type":{"builtIn":"string"}}],"returnType":{"builtIn":"string"}},
			{"name":"notify","parameters":[],"returnType":{"builtIn":"void"}}
		]
	}]
}`

type ctxKey string

const (
	callerKey     ctxKey = "caller"
	middlewareKey ctxKey = "middleware"
)

func newContextTestServer(t *testing.T) *Server {
	t.Helper()
	var idl interface{}
	if err := json.Unmarshal([]byte(contextTestIDL), &idl); err != nil {
		t.Fatalf("unmarshal IDL: %v", err)
	}
	return NewServer(NewContract(idl), true, true)
}

// recordingHandler records the context it receives and reports whether the
// context can be cancelled (Done() != nil).
type recordingHandler struct {
	mu    sync.Mutex
	ctxs  []context.Context
	calls int
}

func (h *recordingHandler) record(ctx context.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.calls++
	h.ctxs = append(h.ctxs, ctx)
}

func (h *recordingHandler) lastContext() context.Context {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.ctxs) == 0 {
		return nil
	}
	return h.ctxs[len(h.ctxs)-1]
}

func (h *recordingHandler) callCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.calls
}

func (h *recordingHandler) Ping(ctx context.Context) (string, error) {
	h.record(ctx)
	if v, ok := ctx.Value(callerKey).(string); ok {
		return v, nil
	}
	if _, ok := ctx.Value(middlewareKey).(string); ok {
		return "from-middleware", nil
	}
	if ctx.Done() == nil {
		return "background", nil
	}
	return "derived", nil
}

func (h *recordingHandler) Echo(ctx context.Context, name string) (string, error) {
	h.record(ctx)
	return name, nil
}

func (h *recordingHandler) Notify(ctx context.Context) error {
	h.record(ctx)
	return nil
}

func pingRequest(id interface{}) map[string]interface{} {
	request := map[string]interface{}{"jsonrpc": "2.0", "method": "Ctx.ping"}
	if id != nil {
		request["id"] = id
	}
	return request
}

// A value placed on the caller context reaches the handler through
// CallWithContext.
func TestCallWithContextPassesCallerContextToHandler(t *testing.T) {
	server := newContextTestServer(t)
	handler := &recordingHandler{}
	if err := server.AddHandler("Ctx", handler); err != nil {
		t.Fatalf("AddHandler: %v", err)
	}

	ctx := context.WithValue(context.Background(), callerKey, "from-caller")
	response := server.CallWithContext(ctx, pingRequest(1))
	if response == nil {
		t.Fatal("expected a response")
	}
	if result, _ := response["result"].(string); result != "from-caller" {
		t.Errorf("result = %q, want from-caller", result)
	}
	if got := handler.lastContext(); got == nil || got.Value(callerKey) != "from-caller" {
		t.Errorf("handler context = %#v, want the caller value", got)
	}
}

// Call is the background-context convenience form.
func TestCallUsesBackgroundContext(t *testing.T) {
	server := newContextTestServer(t)
	handler := &recordingHandler{}
	if err := server.AddHandler("Ctx", handler); err != nil {
		t.Fatalf("AddHandler: %v", err)
	}

	response := server.Call(pingRequest(1))
	if result, _ := response["result"].(string); result != "background" {
		t.Errorf("result = %q, want background", result)
	}
}

// Deadlines and cancellation reach the handler rather than being ignored.
func TestHandlerObservesDeadlineAndCancellation(t *testing.T) {
	server := newContextTestServer(t)
	handler := &deadlineHandler{}
	if err := server.AddHandler("Ctx", handler); err != nil {
		t.Fatalf("AddHandler: %v", err)
	}

	deadlineCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	response := server.CallWithContext(deadlineCtx, pingRequest(1))
	if result, _ := response["result"].(string); result != context.DeadlineExceeded.Error() {
		t.Errorf("deadline result = %q, want %q", result, context.DeadlineExceeded.Error())
	}

	cancelledCtx, cancelNow := context.WithCancel(context.Background())
	cancelNow()
	response = server.CallWithContext(cancelledCtx, pingRequest(2))
	if result, _ := response["result"].(string); result != context.Canceled.Error() {
		t.Errorf("cancel result = %q, want %q", result, context.Canceled.Error())
	}
}

type deadlineHandler struct{}

func (deadlineHandler) Ping(ctx context.Context) (string, error) {
	select {
	case <-ctx.Done():
		return ctx.Err().Error(), nil
	case <-time.After(2 * time.Second):
		return "not-cancelled", nil
	}
}

func (deadlineHandler) Echo(ctx context.Context, name string) (string, error) { return name, nil }
func (deadlineHandler) Notify(ctx context.Context) error                      { return nil }

// The runtime client/transport path forwards a caller deadline to the server
// handler rather than dropping it at the transport boundary.
func TestInProcTransportPropagatesDeadlineToHandler(t *testing.T) {
	server := newContextTestServer(t)
	if err := server.AddHandler("Ctx", deadlineHandler{}); err != nil {
		t.Fatalf("AddHandler: %v", err)
	}
	transport := NewInProcTransport(server)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	response, err := transport.RequestWithID(ctx, "Ctx.ping", []interface{}{}, 1)
	if err != nil {
		t.Fatalf("RequestWithID: %v", err)
	}
	if result, _ := response["result"].(string); result != context.DeadlineExceeded.Error() {
		t.Errorf("result = %q, want %q", result, context.DeadlineExceeded.Error())
	}
}

// An RPC middleware can derive the context used for dispatch.
func TestRPCMiddlewareDerivesContext(t *testing.T) {
	server := newContextTestServer(t)
	handler := &recordingHandler{}
	if err := server.AddHandler("Ctx", handler); err != nil {
		t.Fatalf("AddHandler: %v", err)
	}
	server.UseRPC(func(ctx context.Context, request map[string]interface{}, response map[string]interface{}) (context.Context, map[string]interface{}, map[string]interface{}) {
		return context.WithValue(ctx, middlewareKey, "from-middleware"), request, nil
	})

	response := server.Call(pingRequest(1))
	if response == nil {
		t.Fatal("expected a response")
	}
	if got := handler.lastContext(); got == nil || got.Value(middlewareKey) != "from-middleware" {
		t.Errorf("handler context = %#v, want the middleware value", got)
	}
}

// An RPC middleware can rewrite the request used for dispatch.
func TestRPCMiddlewareRewritesRequest(t *testing.T) {
	server := newContextTestServer(t)
	handler := &recordingHandler{}
	if err := server.AddHandler("Ctx", handler); err != nil {
		t.Fatalf("AddHandler: %v", err)
	}
	server.UseRPC(func(ctx context.Context, request map[string]interface{}, response map[string]interface{}) (context.Context, map[string]interface{}, map[string]interface{}) {
		rewritten := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "Ctx.echo",
			"params":  []interface{}{"normalized"},
			"id":      request["id"],
		}
		return ctx, rewritten, nil
	})

	response := server.Call(pingRequest(1))
	if result, _ := response["result"].(string); result != "normalized" {
		t.Errorf("result = %q, want normalized", result)
	}
}

// A non-nil middleware response short-circuits dispatch.
func TestRPCMiddlewareShortCircuitsDispatch(t *testing.T) {
	server := newContextTestServer(t)
	handler := &recordingHandler{}
	if err := server.AddHandler("Ctx", handler); err != nil {
		t.Fatalf("AddHandler: %v", err)
	}
	server.UseRPC(func(ctx context.Context, request map[string]interface{}, response map[string]interface{}) (context.Context, map[string]interface{}, map[string]interface{}) {
		return ctx, request, map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      request["id"],
			"error":   map[string]interface{}{"code": -32001, "message": "blocked"},
		}
	})

	response := server.Call(pingRequest(1))
	if response == nil {
		t.Fatal("expected a response")
	}
	if code := errorCode(t, response); code != -32001 {
		t.Errorf("code = %d, want -32001", code)
	}
	if handler.callCount() != 0 {
		t.Errorf("handler was invoked %d times, want 0", handler.callCount())
	}
}

// Notification silence wins over a middleware response.
func TestRPCMiddlewareResponseDiscardedForNotification(t *testing.T) {
	server := newContextTestServer(t)
	handler := &recordingHandler{}
	if err := server.AddHandler("Ctx", handler); err != nil {
		t.Fatalf("AddHandler: %v", err)
	}
	server.UseRPC(func(ctx context.Context, request map[string]interface{}, response map[string]interface{}) (context.Context, map[string]interface{}, map[string]interface{}) {
		return ctx, request, map[string]interface{}{
			"jsonrpc": "2.0",
			"result":  "should be discarded",
		}
	})

	if response := server.Call(pingRequest(nil)); response != nil {
		t.Errorf("notification received a response: %#v", response)
	}
}

// A middleware panic becomes -32603 without reflecting the panic value.
func TestRPCMiddlewarePanicIsContained(t *testing.T) {
	server := newContextTestServer(t)
	handler := &recordingHandler{}
	if err := server.AddHandler("Ctx", handler); err != nil {
		t.Fatalf("AddHandler: %v", err)
	}
	server.UseRPC(func(ctx context.Context, request map[string]interface{}, response map[string]interface{}) (context.Context, map[string]interface{}, map[string]interface{}) {
		panic("secret-middleware-detail")
	})

	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previous) })

	response := server.Call(pingRequest(1))
	if response == nil {
		t.Fatal("expected a response")
	}
	if code := errorCode(t, response); code != -32603 {
		t.Errorf("code = %d, want -32603", code)
	}
	errObj := response["error"].(map[string]interface{})
	if message, _ := errObj["message"].(string); message != internalErrorMessage {
		t.Errorf("message = %q, want %q", message, internalErrorMessage)
	}
	if _, hasData := errObj["data"]; hasData {
		t.Errorf("error carries data: %#v", errObj)
	}
	if strings.Contains(logs.String(), "secret-middleware-detail") {
		t.Errorf("log leaked the middleware panic value: %s", logs.String())
	}
	if handler.callCount() != 0 {
		t.Errorf("handler was invoked %d times, want 0", handler.callCount())
	}
}

// Call also runs middleware, with the background context.
func TestRPCMiddlewareRunsForCallWithBackgroundContext(t *testing.T) {
	server := newContextTestServer(t)
	handler := &recordingHandler{}
	if err := server.AddHandler("Ctx", handler); err != nil {
		t.Fatalf("AddHandler: %v", err)
	}
	var sawBackground bool
	server.UseRPC(func(ctx context.Context, request map[string]interface{}, response map[string]interface{}) (context.Context, map[string]interface{}, map[string]interface{}) {
		sawBackground = ctx.Done() == nil
		return ctx, request, nil
	})

	server.Call(pingRequest(1))
	if !sawBackground {
		t.Error("middleware did not receive a background context from Call")
	}
}

// Middleware runs once per request and each member sees its own envelope.
func TestRPCMiddlewareRunsPerBatchMember(t *testing.T) {
	server := newContextTestServer(t)
	handler := &recordingHandler{}
	if err := server.AddHandler("Ctx", handler); err != nil {
		t.Fatalf("AddHandler: %v", err)
	}
	var seen []interface{}
	server.UseRPC(func(ctx context.Context, request map[string]interface{}, response map[string]interface{}) (context.Context, map[string]interface{}, map[string]interface{}) {
		seen = append(seen, request["id"])
		return ctx, request, nil
	})

	batch := []map[string]interface{}{pingRequest(1), pingRequest(2), pingRequest(3)}
	for _, request := range batch {
		server.Call(request)
	}
	if len(seen) != len(batch) {
		t.Fatalf("middleware ran %d times, want %d", len(seen), len(batch))
	}
	if seen[0] != 1 || seen[1] != 2 || seen[2] != 3 {
		t.Errorf("middleware saw ids %#v, want 1,2,3", seen)
	}
	if handler.callCount() != len(batch) {
		t.Errorf("handler ran %d times, want %d", handler.callCount(), len(batch))
	}
}

// registrationIDL is a one-method contract for registration checks.
const registrationIDL = `{
	"interfaces": [{
		"name": "R",
		"methods": [{"name":"ping","parameters":[],"returnType":{"builtIn":"string"}}]
	}]
}`

func newRegistrationServer(t *testing.T) *Server {
	t.Helper()
	var idl interface{}
	if err := json.Unmarshal([]byte(registrationIDL), &idl); err != nil {
		t.Fatalf("unmarshal IDL: %v", err)
	}
	return NewServer(NewContract(idl), false, false)
}

type validRegistrationHandler struct{}

func (validRegistrationHandler) Ping(ctx context.Context) (string, error) { return "pong", nil }

type noContextHandler struct{}

func (noContextHandler) Ping() (string, error) { return "pong", nil }

type valueOnlyHandler struct{}

func (valueOnlyHandler) Ping(ctx context.Context) string { return "pong" }

type missingMethodHandler struct{}

func (missingMethodHandler) Pong(ctx context.Context) (string, error) { return "pong", nil }

type extraMethodHandler struct{}

func (extraMethodHandler) Ping(ctx context.Context) (string, error) { return "pong", nil }
func (extraMethodHandler) Extra() string                            { return "unrelated" }

func TestAddHandlerValidatesContextAndErrorShape(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		server := newRegistrationServer(t)
		if err := server.AddHandler("R", validRegistrationHandler{}); err != nil {
			t.Fatalf("valid handler rejected: %v", err)
		}
		if _, ok := server.handlers["R"]; !ok {
			t.Error("valid handler was not stored")
		}
	})

	t.Run("no context", func(t *testing.T) {
		server := newRegistrationServer(t)
		err := server.AddHandler("R", noContextHandler{})
		if err == nil {
			t.Fatal("handler without a context was accepted")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "ping") {
			t.Errorf("error %q does not name the offending method", err)
		}
		if _, ok := server.handlers["R"]; ok {
			t.Error("rejected handler was stored")
		}
	})

	t.Run("value only", func(t *testing.T) {
		server := newRegistrationServer(t)
		err := server.AddHandler("R", valueOnlyHandler{})
		if err == nil {
			t.Fatal("value-only handler was accepted")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "ping") {
			t.Errorf("error %q does not name the offending method", err)
		}
		if _, ok := server.handlers["R"]; ok {
			t.Error("rejected handler was stored")
		}
	})

	t.Run("missing method", func(t *testing.T) {
		server := newRegistrationServer(t)
		err := server.AddHandler("R", missingMethodHandler{})
		if err == nil {
			t.Fatal("handler missing an interface method was accepted")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "ping") {
			t.Errorf("error %q does not name the missing method", err)
		}
	})

	t.Run("unrelated methods ignored", func(t *testing.T) {
		server := newRegistrationServer(t)
		if err := server.AddHandler("R", extraMethodHandler{}); err != nil {
			t.Errorf("unrelated exported method caused registration to fail: %v", err)
		}
	})
}

// Without a contract, every exported method is still checked for the
// context/error convention.
func TestAddHandlerWithoutContractValidatesAllExportedMethods(t *testing.T) {
	server := NewServer(nil, false, false)
	if err := server.AddHandler("R", validRegistrationHandler{}); err != nil {
		t.Fatalf("valid handler rejected without a contract: %v", err)
	}
	if err := server.AddHandler("R", noContextHandler{}); err == nil {
		t.Fatal("handler without a context was accepted without a contract")
	}
}

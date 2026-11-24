package pulserpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"testing"
)

// testContractIDL mirrors the shape emitted by the generator for a small IDL.
const testContractIDL = `{
	"interfaces": [{
		"name": "A",
		"methods": [
			{"name":"add","parameters":[
				{"name":"a","type":{"builtIn":"int"}},
				{"name":"b","type":{"builtIn":"int"}}
			],"returnType":{"builtIn":"int"}},
			{"name":"echo","parameters":[
				{"name":"value","type":{"builtIn":"string"},"optional":true}
			],"returnType":{"builtIn":"string"}},
			{"name":"null_return","parameters":[],"returnType":{"builtIn":"string"}},
			{"name":"panics","parameters":[],"returnType":{"builtIn":"string"}},
			{"name":"fails","parameters":[],"returnType":{"builtIn":"string"}},
			{"name":"leaks","parameters":[],"returnType":{"builtIn":"string"}},
			{"name":"bad_enum","parameters":[],"returnType":{"userDefined":"E"}},
			{"name":"nothing","parameters":[],"returnType":{"builtIn":"void"}}
		]
	}],
	"enums": [{"name":"E","values":[{"name":"ok"}]}]
}`

type testHandler struct{}

func (testHandler) Add(ctx context.Context, a, b int) (int, error)         { return a + b, nil }
func (testHandler) Echo(ctx context.Context, value string) (string, error) { return value, nil }
func (testHandler) NullReturn(ctx context.Context) (interface{}, error)    { return nil, nil }
func (testHandler) Panics(ctx context.Context) (string, error)             { panic("handler exploded") }
func (testHandler) Fails(ctx context.Context) (string, error) {
	return "", errors.New("secret-token-abc123")
}
func (testHandler) Leaks(ctx context.Context) (string, error) {
	return "", secretError{"secret-token-abc123"}
}
func (testHandler) BadEnum(ctx context.Context) (string, error) { return "secret-token-abc123", nil }
func (testHandler) Nothing(ctx context.Context) error           { return nil }

// secretError is a handler error whose text carries a sentinel value. Only its
// type may appear in logs.
type secretError struct{ msg string }

func (e secretError) Error() string { return e.msg }

func newTestServer(t *testing.T, validateRequests, validateResponses bool) *Server {
	t.Helper()
	var idl interface{}
	if err := json.Unmarshal([]byte(testContractIDL), &idl); err != nil {
		t.Fatalf("unmarshal IDL: %v", err)
	}
	server := NewServer(NewContract(idl), validateRequests, validateResponses)
	if err := server.AddHandler("A", testHandler{}); err != nil {
		t.Fatalf("AddHandler: %v", err)
	}
	return server
}

func errorCode(t *testing.T, response map[string]interface{}) int {
	t.Helper()
	errObj, ok := response["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected an error response, got %#v", response)
	}
	code, ok := errObj["code"].(int)
	if !ok {
		t.Fatalf("error code is not an int: %#v", errObj["code"])
	}
	return code
}

// go-01: a notification must never receive a response, including an error one.
func TestNotificationErrorsAreSuppressed(t *testing.T) {
	server := newTestServer(t, true, true)
	cases := []map[string]interface{}{
		{"jsonrpc": "2.0", "method": "A.unknown", "params": []interface{}{}},
		{"jsonrpc": "1.0", "method": "A.add", "params": []interface{}{1, 2}},
		{"jsonrpc": "2.0", "method": "A.add", "params": []interface{}{"wrong", 2}},
		{"jsonrpc": "2.0", "method": "A.null_return", "params": []interface{}{}},
		{"jsonrpc": "2.0", "method": "A.panics", "params": []interface{}{}},
	}
	for _, request := range cases {
		if response := server.Call(request); response != nil {
			t.Errorf("notification %v received a response: %#v", request["method"], response)
		}
	}
}

// A request missing jsonrpc is invalid: reply with -32600 and a null id, never
// echoing the caller-supplied id. Notifications stay silent via respondError.
func TestInvalidJSONRPCVersionUsesNullID(t *testing.T) {
	server := newTestServer(t, true, true)
	response := server.Call(map[string]interface{}{
		"method": "A.add",
		"params": []interface{}{5, 3},
		"id":     1.0,
	})
	if response == nil {
		t.Fatal("expected a response for a request with an id")
	}
	if code := errorCode(t, response); code != -32600 {
		t.Errorf("code = %d, want -32600", code)
	}
	if responseID, exists := response["id"]; !exists || responseID != nil {
		t.Errorf("response id = %#v (exists=%t), want explicit null", responseID, exists)
	}

	// The same invalid request without an id is a notification: no response.
	if response := server.Call(map[string]interface{}{
		"method": "A.add",
		"params": []interface{}{5, 3},
	}); response != nil {
		t.Errorf("notification received a response: %#v", response)
	}
}

// go-01: a pulserpc-idl notification must also be silent.
func TestIDLNotificationIsSuppressed(t *testing.T) {
	server := newTestServer(t, true, true)
	if response := server.Call(map[string]interface{}{"jsonrpc": "2.0", "method": "pulserpc-idl"}); response != nil {
		t.Errorf("pulserpc-idl notification received a response: %#v", response)
	}
}

// go-04: object-form params are mapped by IDL parameter name and validated.
func TestObjectFormParams(t *testing.T) {
	server := newTestServer(t, true, true)
	response := server.Call(map[string]interface{}{
		"jsonrpc": "2.0", "method": "A.add",
		"params": map[string]interface{}{"a": 5, "b": 3}, "id": 1,
	})
	if response == nil {
		t.Fatal("expected a response for a request with an id")
	}
	if result, _ := response["result"].(int); result != 8 {
		t.Errorf("result = %v, want 8 (%#v)", result, response)
	}

	// Omitting a required parameter must be rejected, not silently defaulted.
	response = server.Call(map[string]interface{}{
		"jsonrpc": "2.0", "method": "A.add",
		"params": map[string]interface{}{"a": 5}, "id": 2,
	})
	if code := errorCode(t, response); code != -32602 {
		t.Errorf("missing required param code = %d, want -32602", code)
	}

	// Unknown names are rejected rather than dropped.
	response = server.Call(map[string]interface{}{
		"jsonrpc": "2.0", "method": "A.add",
		"params": map[string]interface{}{"a": 5, "b": 3, "c": 9}, "id": 3,
	})
	if code := errorCode(t, response); code != -32602 {
		t.Errorf("unknown param name code = %d, want -32602", code)
	}
}

// go-05: an explicit null for an optional parameter is valid input.
func TestNullOptionalParamDoesNotPanic(t *testing.T) {
	server := newTestServer(t, true, true)
	response := server.Call(map[string]interface{}{
		"jsonrpc": "2.0", "method": "A.echo",
		"params": map[string]interface{}{"value": nil}, "id": 1,
	})
	if response == nil {
		t.Fatal("expected a response")
	}
	if result, _ := response["result"].(string); result != "" {
		t.Errorf("result = %q, want zero value", result)
	}
}

// go-05: null for a required non-pointer type must not panic when request
// validation is disabled.
func TestNullRequiredParamWithoutValidationDoesNotPanic(t *testing.T) {
	server := newTestServer(t, false, false)
	response := server.Call(map[string]interface{}{
		"jsonrpc": "2.0", "method": "A.add",
		"params": []interface{}{nil, 2}, "id": 1,
	})
	if response == nil {
		t.Fatal("expected a response")
	}
	if result, _ := response["result"].(int); result != 2 {
		t.Errorf("result = %v, want 2 (%#v)", result, response)
	}
}

// go-05: panics from handler code become JSON-RPC internal errors.
func TestHandlerPanicBecomesInternalError(t *testing.T) {
	server := newTestServer(t, true, true)
	response := server.Call(map[string]interface{}{
		"jsonrpc": "2.0", "method": "A.panics", "params": []interface{}{}, "id": 1,
	})
	if code := errorCode(t, response); code != -32603 {
		t.Errorf("panic code = %d, want -32603", code)
	}
}

// audit-go-1: the recover() path must return a fixed generic message and must
// never reflect the raw panic value into error.data, which the generated HTTP
// adapter serializes straight to clients.
func TestHandlerPanicDetailIsNotReturnedToClient(t *testing.T) {
	server := newTestServer(t, true, true)
	response := server.Call(map[string]interface{}{
		"jsonrpc": "2.0", "method": "A.panics", "params": []interface{}{}, "id": 7,
	})
	if code := errorCode(t, response); code != -32603 {
		t.Errorf("panic code = %d, want -32603", code)
	}

	errObj := response["error"].(map[string]interface{})
	if message, _ := errObj["message"].(string); message != internalErrorMessage {
		t.Errorf("error message = %q, want the fixed %q", message, internalErrorMessage)
	}
	if data, exists := errObj["data"]; exists {
		t.Errorf("error data = %#v, want no data member (D4/R15)", data)
	}

	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal error response: %v", err)
	}
	if strings.Contains(string(encoded), "handler exploded") {
		t.Errorf("panic detail leaked into the serialized error response: %s", encoded)
	}
}

// go-03: unexpected handler and response-validation errors are logged by type
// only. Their text may contain secrets or PII and must never be persisted to
// logs, matching the panic path. Declared RPCError messages are unaffected.
func TestUnexpectedErrorsAreLoggedWithoutDetail(t *testing.T) {
	server := newTestServer(t, true, true)

	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previous) })

	handlerErr := server.Call(map[string]interface{}{
		"jsonrpc": "2.0", "method": "A.leaks", "params": []interface{}{}, "id": 1,
	})
	if code := errorCode(t, handlerErr); code != -32603 {
		t.Fatalf("handler error code = %d, want -32603", code)
	}

	validationErr := server.Call(map[string]interface{}{
		"jsonrpc": "2.0", "method": "A.bad_enum", "params": []interface{}{}, "id": 2,
	})
	if code := errorCode(t, validationErr); code != -32603 {
		t.Fatalf("response validation code = %d, want -32603", code)
	}

	logged := logs.String()
	if strings.Contains(logged, "secret-token-abc123") {
		t.Errorf("log output leaked the error text: %s", logged)
	}
	if !strings.Contains(logged, "pulserpc.secretError") {
		t.Errorf("handler error was not logged by type (%%T): %s", logged)
	}
	if !strings.Contains(logged, "response validation failed") {
		t.Errorf("response validation failure was not logged: %s", logged)
	}
}

// R15/D4: a handler error must not leak into the client-visible message or
// data; the raw error is only logged server-side.
func TestHandlerErrorDetailIsNotReturnedToClient(t *testing.T) {
	server := newTestServer(t, true, true)
	response := server.Call(map[string]interface{}{
		"jsonrpc": "2.0", "method": "A.fails", "params": []interface{}{}, "id": 9,
	})
	if code := errorCode(t, response); code != -32603 {
		t.Errorf("handler error code = %d, want -32603", code)
	}
	errObj := response["error"].(map[string]interface{})
	if message, _ := errObj["message"].(string); message != internalErrorMessage {
		t.Errorf("error message = %q, want %q", message, internalErrorMessage)
	}
	if data, exists := errObj["data"]; exists {
		t.Errorf("error data = %#v, want no data member", data)
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal error response: %v", err)
	}
	if strings.Contains(string(encoded), "secret-token") {
		t.Errorf("handler error leaked into the response: %s", encoded)
	}
}

// go-09: nil results are validated; a required return must reject null.
func TestNilResultIsValidated(t *testing.T) {
	server := newTestServer(t, true, true)
	response := server.Call(map[string]interface{}{
		"jsonrpc": "2.0", "method": "A.null_return", "params": []interface{}{}, "id": 1,
	})
	if code := errorCode(t, response); code != -32603 {
		t.Errorf("null required result code = %d, want -32603", code)
	}

	// A void return with a nil result remains valid.
	response = server.Call(map[string]interface{}{
		"jsonrpc": "2.0", "method": "A.nothing", "params": []interface{}{}, "id": 2,
	})
	if response == nil {
		t.Fatal("expected a response for void method")
	}
	if _, hasErr := response["error"]; hasErr {
		t.Errorf("void method returned an error: %#v", response)
	}
}

// go-15: batch semantics delegated to Server.Call. The generated HTTP adapter
// writes 204 when no member produces a response.
func processBatch(server *Server, requests []interface{}) []interface{} {
	var responses []interface{}
	for _, raw := range requests {
		request, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if response := server.Call(request); response != nil {
			responses = append(responses, response)
		}
	}
	return responses
}

func TestBatchRequests(t *testing.T) {
	server := newTestServer(t, true, true)

	mixed := processBatch(server, []interface{}{
		map[string]interface{}{"jsonrpc": "2.0", "method": "A.add", "params": []interface{}{2, 3}, "id": 1},
		map[string]interface{}{"jsonrpc": "2.0", "method": "A.unknown", "params": []interface{}{}},
		map[string]interface{}{"jsonrpc": "2.0", "method": "A.echo", "params": []interface{}{"x"}, "id": 2},
	})
	if len(mixed) != 2 {
		t.Errorf("mixed batch produced %d responses, want 2: %#v", len(mixed), mixed)
	}

	onlyNotifications := processBatch(server, []interface{}{
		map[string]interface{}{"jsonrpc": "2.0", "method": "A.unknown", "params": []interface{}{}},
		map[string]interface{}{"jsonrpc": "2.0", "method": "A.add", "params": []interface{}{"bad", 1}},
	})
	if len(onlyNotifications) != 0 {
		t.Errorf("notification-only batch produced responses: %#v", onlyNotifications)
	}

	empty := processBatch(server, []interface{}{})
	if len(empty) != 0 {
		t.Errorf("empty batch produced responses: %#v", empty)
	}
}

// R24/D2: boolean, object, and array ids are rejected with -32600 and are not
// echoed back as the response id.
func TestInvalidRequestIDTypesAreRejected(t *testing.T) {
	server := newTestServer(t, true, true)
	for _, id := range []interface{}{
		true,
		false,
		map[string]interface{}{"nested": 1},
		[]interface{}{1, 2},
	} {
		response := server.Call(map[string]interface{}{
			"jsonrpc": "2.0", "method": "A.add",
			"params": []interface{}{1, 2}, "id": id,
		})
		if code := errorCode(t, response); code != -32600 {
			t.Errorf("id %#v: code = %d, want -32600", id, code)
		}
		if responseID, exists := response["id"]; !exists || responseID != nil {
			t.Errorf("id %#v: response id = %#v, want explicit null", id, responseID)
		}
	}
}

// M21: an explicit id:null is a notification and receives no response.
func TestNullIDIsNotification(t *testing.T) {
	server := newTestServer(t, true, true)
	response := server.Call(map[string]interface{}{
		"jsonrpc": "2.0", "method": "A.add",
		"params": []interface{}{1, 2}, "id": nil,
	})
	if response != nil {
		t.Fatalf("id:null request received a response: %#v", response)
	}
}

// M22/M25/M27: error objects omit data when nil, carry a numeric code and
// string message, and every response declares jsonrpc "2.0".
func TestErrorResponseShape(t *testing.T) {
	server := newTestServer(t, true, true)
	response := server.Call(map[string]interface{}{
		"jsonrpc": "2.0", "method": "A.fails",
		"params": []interface{}{}, "id": 4,
	})
	if response["jsonrpc"] != "2.0" {
		t.Errorf("jsonrpc = %#v, want 2.0", response["jsonrpc"])
	}
	if response["id"] != 4 {
		t.Errorf("id = %#v, want 4", response["id"])
	}
	errObj, ok := response["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("error is not an object: %#v", response)
	}
	if _, ok := errObj["code"].(int); !ok {
		t.Errorf("error.code = %#v, want a numeric value", errObj["code"])
	}
	if _, ok := errObj["message"].(string); !ok {
		t.Errorf("error.message = %#v, want a string", errObj["message"])
	}
	if _, exists := errObj["data"]; exists {
		t.Errorf("error.data = %#v, want it omitted when nil", errObj["data"])
	}
}

// M23: a null result for an optional return is serialized as an explicit
// "result": null member, not omitted.
func TestNullResultIsExplicit(t *testing.T) {
	server := newTestServer(t, true, true)
	response := server.Call(map[string]interface{}{
		"jsonrpc": "2.0", "method": "A.null_return",
		"params": []interface{}{}, "id": 3,
	})
	// A.null_return is required, so validation rejects it. Disable response
	// validation to exercise the success envelope itself.
	server = newTestServer(t, false, false)
	response = server.Call(map[string]interface{}{
		"jsonrpc": "2.0", "method": "A.null_return",
		"params": []interface{}{}, "id": 3,
	})
	if response == nil {
		t.Fatal("expected a response")
	}
	result, exists := response["result"]
	if !exists {
		t.Fatalf("response omits result: %#v", response)
	}
	if result != nil {
		t.Errorf("result = %#v, want nil", result)
	}
}

// M24: numeric response ids are echoed exactly and comparisons tolerate
// integer/float representations.
func TestNumericIDIsEchoed(t *testing.T) {
	server := newTestServer(t, true, true)
	for _, id := range []interface{}{7, 7.0, "abc"} {
		response := server.Call(map[string]interface{}{
			"jsonrpc": "2.0", "method": "A.add",
			"params": []interface{}{1, 2}, "id": id,
		})
		if response["id"] != id {
			t.Errorf("id %#v echoed as %#v", id, response["id"])
		}
	}
	if !idsEqual(7, 7.0) || !idsEqual("x", "x") || idsEqual(1, 2) {
		t.Error("idsEqual does not compare numeric ids by value")
	}
}

// C5: the pulserpc-idl method returns the IDL as a JSON object (not a
// double-encoded string) and follows the normal response envelope.
func TestIDLMethodReturnsObject(t *testing.T) {
	server := newTestServer(t, true, true)
	response := server.Call(map[string]interface{}{
		"jsonrpc": "2.0", "method": "pulserpc-idl", "params": []interface{}{}, "id": 1,
	})
	if response == nil {
		t.Fatal("expected a response")
	}
	if response["jsonrpc"] != "2.0" || response["id"] != 1 {
		t.Errorf("unexpected envelope: %#v", response)
	}
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("pulserpc-idl result is not an object: %#v", response["result"])
	}
	if _, ok := result["interfaces"].([]interface{}); !ok {
		t.Errorf("IDL object has no interfaces array: %#v", result)
	}
}

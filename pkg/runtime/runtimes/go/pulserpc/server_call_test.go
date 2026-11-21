package pulserpc

import (
	"encoding/json"
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
			{"name":"nothing","parameters":[],"returnType":{"builtIn":"void"}}
		]
	}]
}`

type testHandler struct{}

func (testHandler) Add(a, b int) int         { return a + b }
func (testHandler) Echo(value string) string { return value }
func (testHandler) NullReturn() interface{}  { return nil }
func (testHandler) Panics() string           { panic("handler exploded") }
func (testHandler) Nothing() error           { return nil }

func newTestServer(t *testing.T, validateRequests, validateResponses bool) *Server {
	t.Helper()
	var idl interface{}
	if err := json.Unmarshal([]byte(testContractIDL), &idl); err != nil {
		t.Fatalf("unmarshal IDL: %v", err)
	}
	server := NewServer(NewContract(idl), validateRequests, validateResponses)
	server.AddHandler("A", testHandler{})
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

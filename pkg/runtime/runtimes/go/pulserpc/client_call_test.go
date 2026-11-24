package pulserpc

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

type capturingTransport struct {
	requests []map[string]interface{}
	respond  func(request map[string]interface{}) map[string]interface{}
}

func (t *capturingTransport) Request(ctx context.Context, method string, params interface{}) (map[string]interface{}, error) {
	return t.RequestWithID(ctx, method, params, 1)
}

func (t *capturingTransport) RequestWithID(ctx context.Context, method string, params interface{}, id interface{}) (map[string]interface{}, error) {
	request := map[string]interface{}{"jsonrpc": "2.0", "method": method}
	if params != nil {
		request["params"] = params
	}
	if id != nil {
		request["id"] = id
	}
	t.requests = append(t.requests, request)
	if t.respond != nil {
		return t.respond(request), nil
	}
	response := map[string]interface{}{"jsonrpc": "2.0", "result": nil}
	if id != nil {
		response["id"] = id
	}
	return response, nil
}

func testClientContract(t *testing.T) *Contract {
	t.Helper()
	var idl interface{}
	if err := json.Unmarshal([]byte(testContractIDL), &idl); err != nil {
		t.Fatalf("unmarshal IDL: %v", err)
	}
	return NewContract(idl)
}

// go-02: a notification is serialized without an id key.
func TestNotifyOmitsID(t *testing.T) {
	transport := &capturingTransport{}
	client := &Client{transport: transport}

	client.Notify(context.Background(), "A.echo", []interface{}{"hello"})

	if len(transport.requests) != 1 {
		t.Fatalf("got %d requests, want 1", len(transport.requests))
	}
	request := transport.requests[0]
	if _, hasID := request["id"]; hasID {
		t.Errorf("notification request contains an id: %#v", request)
	}
	if request["method"] != "A.echo" {
		t.Errorf("method = %v, want A.echo", request["method"])
	}
}

// go-03: each Call uses a distinct id and it is the id sent to the transport.
func TestSequentialCallsUseDistinctIDs(t *testing.T) {
	transport := &capturingTransport{}
	client := &Client{transport: transport}

	for i := 0; i < 2; i++ {
		if _, err := client.Call(context.Background(), "A.add", []interface{}{1, 2}); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}

	first, ok1 := transport.requests[0]["id"].(uint64)
	second, ok2 := transport.requests[1]["id"].(uint64)
	if !ok1 || !ok2 {
		t.Fatalf("request ids are not uint64: %#v", transport.requests)
	}
	if first == second {
		t.Errorf("sequential calls reused id %d", first)
	}
}

// go-03: a response whose id does not match (or which carries both result and
// error) must be rejected.
func TestClientRejectsMalformedEnvelope(t *testing.T) {
	cases := []struct {
		name     string
		response map[string]interface{}
	}{
		{"mismatched id", map[string]interface{}{"jsonrpc": "2.0", "id": 99, "result": 3}},
		{"missing id", map[string]interface{}{"jsonrpc": "2.0", "result": 3}},
		{"result and error", map[string]interface{}{
			"jsonrpc": "2.0", "id": 1, "result": 3,
			"error": map[string]interface{}{"code": -32602, "message": "bad"},
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			transport := &capturingTransport{
				respond: func(map[string]interface{}) map[string]interface{} { return tc.response },
			}
			client := &Client{transport: transport}
			if _, err := client.Call(context.Background(), "A.add", []interface{}{1, 2}); err == nil {
				t.Errorf("expected an error for %s", tc.name)
			}
		})
	}
}

// go-09: a null result for a required return is rejected on the client too.
func TestClientRejectsNullResultForRequiredReturn(t *testing.T) {
	transport := &capturingTransport{
		respond: func(map[string]interface{}) map[string]interface{} {
			return map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": nil}
		},
	}
	client := &Client{transport: transport, contract: testClientContract(t), validateResponse: true}

	if _, err := client.Call(context.Background(), "A.null_return", []interface{}{}); err == nil {
		t.Fatal("expected a response validation error for a null required result")
	} else if fmt.Sprint(err) == "" {
		t.Fatal("empty validation error")
	}
}

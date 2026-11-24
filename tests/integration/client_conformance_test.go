package integration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/coopernurse/pulserpc/pkg/runtime/runtimes/go/pulserpc"
)

// Canonical client-envelope cases (M5/M14/M24/M27/D2), applied uniformly to
// every runtime in its own client test suite:
//
//  1. response jsonrpc != "2.0" is rejected
//  2. response id that does not match the request is rejected
//  3. a response carrying both result and error is rejected
//  4. a response carrying neither result nor error is rejected
//  5. an error object with a non-numeric code is rejected
//  6. an error object with a non-string message is rejected
//  7. numeric ids are compared by value (1 vs 1.0)
//  8. null result for a required return is rejected when response validation is on
//  9. null result for an optional return is accepted when response validation is on
//
// This file exercises the Go client against scripted envelopes; the other
// runtimes cover the same list in their own client test suites.

const clientConformanceIDL = `{
	"rootNamespace": "conform",
	"checksum": "client-conformance",
	"interfaces": [{
		"name": "A",
		"methods": [
			{"name":"add","parameters":[
				{"name":"a","type":{"builtIn":"int"}},
				{"name":"b","type":{"builtIn":"int"}}
			],"returnType":{"builtIn":"int"}},
			{"name":"required","parameters":[],"returnType":{"builtIn":"string"}},
			{"name":"optional","parameters":[],"returnType":{"builtIn":"string"},"returnOptional":true}
		]
	}],
	"structs": [], "enums": [], "errors": []
}`

type scriptedTransport struct {
	respond func(method string, id interface{}) map[string]interface{}
}

func (t *scriptedTransport) Request(ctx context.Context, method string, params interface{}) (map[string]interface{}, error) {
	return t.RequestWithID(ctx, method, params, 1)
}

func (t *scriptedTransport) RequestWithID(ctx context.Context, method string, params interface{}, id interface{}) (map[string]interface{}, error) {
	if method == "pulserpc-idl" {
		var idl interface{}
		if err := json.Unmarshal([]byte(clientConformanceIDL), &idl); err != nil {
			return nil, err
		}
		return map[string]interface{}{"jsonrpc": "2.0", "result": idl, "id": id}, nil
	}
	return t.respond(method, id), nil
}

func TestClientConformance(t *testing.T) {
	cases := []struct {
		name    string
		method  string
		params  interface{}
		build   func(id interface{}) map[string]interface{}
		wantErr bool
	}{
		{
			name: "wrong jsonrpc version", method: "A.add", params: []interface{}{1, 2}, wantErr: true,
			build: func(id interface{}) map[string]interface{} {
				return map[string]interface{}{"jsonrpc": "1.0", "result": 3, "id": id}
			},
		},
		{
			name: "mismatched id", method: "A.add", params: []interface{}{1, 2}, wantErr: true,
			build: func(id interface{}) map[string]interface{} {
				return map[string]interface{}{"jsonrpc": "2.0", "result": 3, "id": uint64(999)}
			},
		},
		{
			name: "both result and error", method: "A.add", params: []interface{}{1, 2}, wantErr: true,
			build: func(id interface{}) map[string]interface{} {
				return map[string]interface{}{"jsonrpc": "2.0", "result": 3,
					"error": map[string]interface{}{"code": -32602, "message": "bad"}, "id": id}
			},
		},
		{
			name: "neither result nor error", method: "A.add", params: []interface{}{1, 2}, wantErr: true,
			build: func(id interface{}) map[string]interface{} {
				return map[string]interface{}{"jsonrpc": "2.0", "id": id}
			},
		},
		{
			name: "error code not numeric", method: "A.add", params: []interface{}{1, 2}, wantErr: true,
			build: func(id interface{}) map[string]interface{} {
				return map[string]interface{}{"jsonrpc": "2.0",
					"error": map[string]interface{}{"code": "oops", "message": "bad"}, "id": id}
			},
		},
		{
			name: "error message not string", method: "A.add", params: []interface{}{1, 2}, wantErr: true,
			build: func(id interface{}) map[string]interface{} {
				return map[string]interface{}{"jsonrpc": "2.0",
					"error": map[string]interface{}{"code": -32602, "message": 42}, "id": id}
			},
		},
		{
			name: "numeric id compared by value", method: "A.add", params: []interface{}{1, 2}, wantErr: false,
			build: func(id interface{}) map[string]interface{} {
				var echoed interface{} = id
				if n, ok := id.(uint64); ok {
					echoed = float64(n)
				}
				return map[string]interface{}{"jsonrpc": "2.0", "result": 3, "id": echoed}
			},
		},
		{
			name: "null required result", method: "A.required", wantErr: true,
			build: func(id interface{}) map[string]interface{} {
				return map[string]interface{}{"jsonrpc": "2.0", "result": nil, "id": id}
			},
		},
		{
			name: "null optional result", method: "A.optional", wantErr: false,
			build: func(id interface{}) map[string]interface{} {
				return map[string]interface{}{"jsonrpc": "2.0", "result": nil, "id": id}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			transport := &scriptedTransport{respond: func(method string, id interface{}) map[string]interface{} {
				return tc.build(id)
			}}
			client, err := pulserpc.NewClient(transport, false, true)
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			_, err = client.Call(context.Background(), tc.method, tc.params)
			if tc.wantErr && err == nil {
				t.Errorf("expected the malformed envelope to be rejected")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

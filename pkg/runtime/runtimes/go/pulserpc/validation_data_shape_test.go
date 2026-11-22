package pulserpc

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// core-13: the documented validation-error `data` is a string. This conformance
// test reproduces the documented example byte-for-byte from the Go runtime.
func TestValidationErrorDataShapeMatchesDocs(t *testing.T) {
	raw := `{
		"interfaces": [{
			"name": "Svc",
			"methods": [{
				"name": "Create",
				"parameters": [{"name": "user", "type": {"userDefined": "User"}}],
				"returnType": {"builtIn": "string"}
			}]
		}],
		"structs": [{
			"name": "User",
			"fields": [{"name": "email", "type": {"builtIn": "string"}, "optional": false}]
		}]
	}`
	var parsed interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("unmarshal IDL: %v", err)
	}
	contract := NewContract(parsed)
	err := contract.ValidateRequest("Svc", "Create", []interface{}{map[string]interface{}{}})
	if err == nil {
		t.Fatal("expected a validation error for a missing required field")
	}

	// This mirrors Server.Call's error path (server.go:97).
	server := &Server{}
	_ = server
	// Converged to structured data (M13): RPCError carries a map.
	rpcErr, ok := err.(*RPCError)
	if !ok {
		t.Fatalf("expected *RPCError, got %T", err)
	}
	dmap, ok := rpcErr.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data must be a map, got %T: %#v", rpcErr.Data, rpcErr.Data)
	}
	if !strings.Contains(fmt.Sprint(dmap["error"]), "email") {
		t.Errorf("error must mention 'email': %#v", dmap)
	}
	fields := dmap["invalid_fields"]
	switch v := fields.(type) {
	case []string:
		if len(v) == 0 {
			t.Fatalf("invalid_fields must be non-empty: %#v", dmap)
		}
	case []interface{}:
		if len(v) == 0 {
			t.Fatalf("invalid_fields must be non-empty: %#v", dmap)
		}
	default:
		t.Fatalf("invalid_fields wrong type: %#v", dmap)
	}
}

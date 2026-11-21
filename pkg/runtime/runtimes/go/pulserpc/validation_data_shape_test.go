package pulserpc

import (
	"encoding/json"
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
	response := server.errorResponse(1, -32602, "Invalid params", err.Error())
	errObj, ok := response["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("error is not an object: %#v", response["error"])
	}
	data, ok := errObj["data"].(string)
	if !ok {
		t.Fatalf("data must be a string, got %T: %#v", errObj["data"], errObj["data"])
	}
	const documented = "function 'Svc.Create' invalid param 'user': missing required field 'email' in struct User"
	if data != documented {
		t.Errorf("data = %q, want documented %q", data, documented)
	}
	if !strings.Contains(data, "email") {
		t.Error("data must mention the offending field")
	}
}

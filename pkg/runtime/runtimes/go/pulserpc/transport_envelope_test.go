package pulserpc

import "testing"

// TestValidateEnvelope covers the M5 response-envelope contract: the version
// must be 2.0, the id must match, exactly one of result/error must be present,
// and an error member must be a well-formed object.
func TestValidateEnvelope(t *testing.T) {
	validError := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1.0,
		"error": map[string]interface{}{
			"code":    float64(-32602),
			"message": "Invalid params",
		},
	}
	validResult := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1.0,
		"result":  "ok",
	}

	valid := []struct {
		name     string
		response map[string]interface{}
	}{
		{"result", validResult},
		{"error", validError},
		{"null result", map[string]interface{}{"jsonrpc": "2.0", "id": 1.0, "result": nil}},
	}
	for _, tt := range valid {
		if err := validateEnvelope(tt.response, 1); err != nil {
			t.Errorf("%s: valid envelope rejected: %v", tt.name, err)
		}
	}

	invalid := []struct {
		name     string
		response map[string]interface{}
	}{
		{"nil response", nil},
		{"missing jsonrpc", map[string]interface{}{"id": 1.0, "result": "ok"}},
		{"wrong jsonrpc", map[string]interface{}{"jsonrpc": "1.0", "id": 1.0, "result": "ok"}},
		{"missing id", map[string]interface{}{"jsonrpc": "2.0", "result": "ok"}},
		{"id mismatch", map[string]interface{}{"jsonrpc": "2.0", "id": 2.0, "result": "ok"}},
		{"both result and error", map[string]interface{}{
			"jsonrpc": "2.0", "id": 1.0, "result": "ok",
			"error": map[string]interface{}{"code": float64(-1), "message": "x"},
		}},
		{"neither result nor error", map[string]interface{}{"jsonrpc": "2.0", "id": 1.0}},
		{"null error", map[string]interface{}{"jsonrpc": "2.0", "id": 1.0, "error": nil}},
		{"non-object error", map[string]interface{}{"jsonrpc": "2.0", "id": 1.0, "error": "boom"}},
		{"error without code", map[string]interface{}{
			"jsonrpc": "2.0", "id": 1.0,
			"error": map[string]interface{}{"message": "boom"},
		}},
		{"error with string code", map[string]interface{}{
			"jsonrpc": "2.0", "id": 1.0,
			"error": map[string]interface{}{"code": "boom", "message": "boom"},
		}},
		{"error without message", map[string]interface{}{
			"jsonrpc": "2.0", "id": 1.0,
			"error": map[string]interface{}{"code": float64(-32603)},
		}},
	}
	for _, tt := range invalid {
		if err := validateEnvelope(tt.response, 1); err == nil {
			t.Errorf("%s: malformed envelope accepted", tt.name)
		}
	}
}

// TestValidateEnvelopeExported verifies generated transports use the same
// contract as the runtime client.
func TestValidateEnvelopeExported(t *testing.T) {
	ok := map[string]interface{}{"jsonrpc": "2.0", "id": 1.0, "result": "ok"}
	if err := ValidateResponseEnvelope(ok, 1); err != nil {
		t.Fatalf("exported validator rejected a valid envelope: %v", err)
	}
	bad := map[string]interface{}{"jsonrpc": "1.0", "id": 1.0, "result": "ok"}
	if err := ValidateResponseEnvelope(bad, 1); err == nil {
		t.Fatal("exported validator accepted a wrong jsonrpc version")
	}
}

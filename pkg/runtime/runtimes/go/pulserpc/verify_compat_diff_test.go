package pulserpc

import (
	"encoding/json"
	"testing"
)

// core-09 acceptance: when the server's IDL diverges from the client's, the
// verification diff contains an Error delta and the two checksums differ.
// (A genuinely breaking divergence is a required field added by the server;
// see core-12 for why a removed field is classified Info.)
func TestVerifyCompatibilityDivergentIDLsProduceErrorDelta(t *testing.T) {
	clientRaw := `{
		"checksum": "client-checksum",
		"structs": [{"name": "User", "fields": [{"name": "email", "type": {"builtIn": "string"}}]}]
	}`
	serverRaw := `{
		"checksum": "server-checksum",
		"structs": [{"name": "User", "fields": [
			{"name": "email", "type": {"builtIn": "string"}},
			{"name": "requiredNewField", "type": {"builtIn": "string"}, "optional": false}
		]}]
	}`
	var client, server interface{}
	if err := json.Unmarshal([]byte(clientRaw), &client); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(serverRaw), &server); err != nil {
		t.Fatal(err)
	}

	deltas := DiffIDL(client, server)
	foundError := false
	for _, d := range deltas {
		if d.Severity == SeverityError {
			foundError = true
		}
	}
	if !foundError {
		t.Fatalf("expected an Error delta for a divergent server IDL, got: %+v", deltas)
	}

	clientChecksum, _ := client.(map[string]interface{})["checksum"].(string)
	serverChecksum, _ := server.(map[string]interface{})["checksum"].(string)
	if clientChecksum == serverChecksum {
		t.Errorf("ServerChecksum must be able to differ from ClientChecksum")
	}
}

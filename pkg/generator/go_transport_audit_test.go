package generator

import (
	"strings"
	"testing"
)

// The generated Go HTTP transport must enforce the same response-envelope
// contract as the runtime client (invariant M5) rather than its own partial
// id/result checks.
func TestGeneratedGoTransportValidatesEnvelope(t *testing.T) {
	var sb strings.Builder
	writeHTTPTransportGo(&sb)
	code := sb.String()

	if !strings.Contains(code, "pulserpc.ValidateResponseEnvelope(response, id)") {
		t.Errorf("generated Go transport does not call pulserpc.ValidateResponseEnvelope:\n%s", code)
	}
	for _, stale := range []string{
		"response id %v does not match request id",
		"invalid response: both result and error are present",
	} {
		if strings.Contains(code, stale) {
			t.Errorf("generated Go transport still contains stale inline envelope check %q", stale)
		}
	}
}

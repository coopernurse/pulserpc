package generator

import (
	"strings"
	"testing"
)

// ts-node-11: the generated TypeScript test-server HTTP adapter is the only
// TS HTTP binding exercised by the integration suite, so it must dispatch JSON-RPC
// batch requests (arrays) and suppress notification responses.
func TestGeneratedTsTestServerHandlesBatchRequests(t *testing.T) {
	idl := parseTestIDL(t, "namespace test\ninterface Svc { Ping() string }\n")
	code := generateTestServerTs(idl, nil, nil, nil, "", nil, "test", "")

	markers := []string{
		"if (Array.isArray(data)) {",
		"const resp = await this.rpcServer.call(item);",
		"if (resp !== null && resp !== undefined) {",
		"responses.push(resp);",
		"res.end(JSON.stringify(responses));",
		// An all-notification batch produces no body.
		"if (responses.length === 0) {",
		"res.writeHead(204);",
		// Single-object requests must still work.
		"const response = await this.rpcServer.call(data);",
	}
	for _, marker := range markers {
		if !strings.Contains(code, marker) {
			t.Errorf("generated TS test server is missing %q\n%s", marker, code)
		}
	}

	// The batch branch must come before the single-request branch.
	batch := strings.Index(code, "if (Array.isArray(data)) {")
	single := strings.Index(code, "const response = await this.rpcServer.call(data);")
	if batch < 0 || single < 0 || batch > single {
		t.Errorf("batch dispatch (%d) must precede single dispatch (%d)", batch, single)
	}
}

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
		"Array.isArray(data)",
		"this.rpcServer.call(item)",
		"item !== null && item !== undefined",
		"res.writeHead(204)",
		"res.end(payload)",
	}
	for _, marker := range markers {
		if !strings.Contains(code, marker) {
			t.Errorf("generated TS test server is missing %q\n%s", marker, code)
		}
	}

	// A batch must be detected before the single-request response is emitted.
	batch := strings.Index(code, "Array.isArray(data)")
	emit := strings.Index(code, "res.end(payload)")
	if batch < 0 || emit < 0 || batch > emit {
		t.Errorf("batch detection (%d) must precede response emission (%d)", batch, emit)
	}
}

// ts-cjs-11: the generated CJS tsconfig must be type-checkable by the pinned
// TypeScript 5 compiler used by the integration harness.
func TestGeneratedTsCjsTSConfigSupportsTypecheck(t *testing.T) {
	withTempOutputDir(t, func(outputDir string) {
		idl := parseTestIDL(t, "namespace test\ninterface Svc { Ping() string }\n")
		tsRunGenWithFlags(t, idl, outputDir, map[string]string{
			"ts-module":       "cjs",
			"ts-gen-tsconfig": "true",
		})

		cfg := tsReadJSONFile(t, outputDir, "tsconfig.json")
		opts, ok := cfg["compilerOptions"].(map[string]interface{})
		if !ok {
			t.Fatalf("compilerOptions missing or not an object: %v", cfg["compilerOptions"])
		}
		if opts["moduleDetection"] != "force" {
			t.Errorf("moduleDetection = %v, want force", opts["moduleDetection"])
		}
		if opts["ignoreDeprecations"] != "5.0" {
			t.Errorf("ignoreDeprecations = %v, want 5.0", opts["ignoreDeprecations"])
		}
	})
}

// ts-cjs-14: the generated CommonJS test server's HTTP adapter must accept
// batch requests (arrays), drop notification responses, and return 204 when
// nothing is left to send.
func TestGeneratedTsCjsServerHandlesBatches(t *testing.T) {
	idl := parseTestIDL(t, "namespace test\ninterface Svc { Ping() string }\n")
	code := generateTestServerTs(idl, nil, nil, nil, "test", nil, "test", "cjs")

	for _, want := range []string{
		"Array.isArray(data)",
		"Promise.all(data.map((item) => this.rpcServer.call(item)))",
		"filter((item) => item !== null && item !== undefined)",
		"Array.isArray(response) && response.length === 0",
		"res.writeHead(204)",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated CJS test server missing batch handling %q", want)
		}
	}

	// An empty batch must produce a JSON-RPC error object, not a 204.
	for _, want := range []string{
		"data.length === 0",
		"Invalid Request - empty batch",
		"code: -32600",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated CJS test server missing empty-batch handling %q", want)
		}
	}
}

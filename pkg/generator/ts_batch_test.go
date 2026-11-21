package generator

import (
	"strings"
	"testing"
)

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
}

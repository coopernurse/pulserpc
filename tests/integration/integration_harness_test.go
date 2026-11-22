package integration

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// core-11: a wrong JSON-RPC error code must fail the HTTP harness, not print a
// WARN and keep going.
func TestHTTPAPIHarnessFailsOnWrongErrorCode(t *testing.T) {
	// Prefer Git Bash; on Windows "bash" on PATH may be the WSL stub.
	bash := ""
	for _, candidate := range []string{`C:\Program Files\Git\bin\bash.exe`, `C:\Program Files\Git\usr\bin\bash.exe`} {
		if _, err := os.Stat(candidate); err == nil {
			bash = candidate
			break
		}
	}
	if bash == "" {
		var err error
		bash, err = exec.LookPath("bash")
		if err != nil {
			t.Skip("bash not available on PATH")
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		if req.Method == "pulserpc-idl" {
			fmt.Fprint(w, `{"jsonrpc":"2.0","result":{},"id":1}`)
			return
		}
		// The first validation test calls A.add with a single param. Return a
		// deliberately wrong error code so the harness must fail.
		if req.Method == "A.add" && strings.TrimSpace(string(req.Params)) == "[5]" {
			fmt.Fprint(w, `{"jsonrpc":"2.0","error":{"code":-99999,"message":"wrong code"},"id":1}`)
			return
		}
		fmt.Fprint(w, `{"jsonrpc":"2.0","result":0,"id":1}`)
	}))
	defer server.Close()

	script := filepath.Join("test_http_api.sh")
	cmd := exec.Command(bash, script, server.URL)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected test_http_api.sh to exit non-zero on a wrong error code; output:\n%s", out)
	}
	if !strings.Contains(string(out), "FAIL") {
		t.Errorf("expected a FAIL message in harness output, got:\n%s", out)
	}
}

// Windows ships `python3` as a Microsoft Store stub that is on PATH but exits
// non-zero. Both shell harnesses must probe functionality before using a
// candidate interpreter rather than trusting `command -v`.
func TestHarnessPythonDetectionIsFunctional(t *testing.T) {
	for _, script := range []string{"test_limits.sh", "test_http_api.sh"} {
		data, err := os.ReadFile(script)
		if err != nil {
			t.Fatalf("read %s: %v", script, err)
		}
		text := string(data)
		if !strings.Contains(text, `"$candidate" -c 'import json'`) {
			t.Errorf("%s does not probe that a python candidate can run", script)
		}
		if strings.Contains(text, "elif command -v python >/dev/null 2>&1; then") {
			t.Errorf("%s still selects a python interpreter by presence only", script)
		}
	}
}

// The missing-jsonrpc probe must assert the full error envelope (HTTP 200,
// jsonrpc 2.0, id null, code -32600) instead of grepping for `"error"`.
func TestHTTPAPIMissingJSONRPCProbeIsStrict(t *testing.T) {
	bash := findBash(t)
	const description = "Request without jsonrpc field returns -32600"

	cases := []struct {
		mode     string
		wantPass bool
	}{
		{mode: "compliant", wantPass: true},
		{mode: "id-echo", wantPass: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.mode, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					Method  string          `json:"method"`
					Params  json.RawMessage `json:"params"`
					JSONRPC string          `json:"jsonrpc"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				w.Header().Set("Content-Type", "application/json")
				if req.Method == "pulserpc-idl" {
					fmt.Fprint(w, `{"jsonrpc":"2.0","result":{},"id":1}`)
					return
				}
				if req.JSONRPC == "" && req.Method == "A.add" {
					switch tc.mode {
					case "compliant":
						fmt.Fprint(w, `{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request"},"id":null}`)
					case "id-echo":
						fmt.Fprint(w, `{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request"},"id":1}`)
					default:
						fmt.Fprint(w, `{"jsonrpc":"2.0","result":0,"id":1}`)
					}
					return
				}
				fmt.Fprint(w, `{"jsonrpc":"2.0","result":0,"id":1}`)
			}))
			defer server.Close()

			cmd := exec.Command(bash, filepath.Join("test_http_api.sh"), server.URL)
			out, err := cmd.CombinedOutput()
			if err != nil && tc.wantPass {
				// Other probes legitimately fail against the stub server; only
				// the targeted probe line is under test here.
				t.Logf("harness exited non-zero: %v", err)
			}

			var probeLine string
			for _, line := range strings.Split(string(out), "\n") {
				if strings.Contains(line, description) {
					probeLine = line
					break
				}
			}
			if probeLine == "" {
				t.Fatalf("harness never ran the missing-jsonrpc probe; output:\n%s", out)
			}
			passed := strings.Contains(probeLine, "PASS")
			if passed != tc.wantPass {
				t.Errorf("missing-jsonrpc probe passed=%v, want %v; line: %s", passed, tc.wantPass, probeLine)
			}
		})
	}
}

// D5: test_limits.sh must pass against a server that enforces the documented
// body and batch caps, regardless of where the runtime puts the reason text
// (error.message vs error.data) or how it words the batch limit.
func TestLimitsHarnessAgainstFakeServer(t *testing.T) {
	bash := findBash(t)

	cases := []struct {
		name string
		// dataStyle puts the reason in error.data and uses the Python runtime's
		// "Batch exceeds maximum size of 100" wording.
		dataStyle bool
		// uncapped models a runtime that has not implemented the caps yet.
		uncapped       bool
		wantOutput     []string
		unwantedOutput []string
	}{
		{
			name:           "documented message",
			wantOutput:     []string{"All limit tests passed"},
			unwantedOutput: []string{"✗ FAIL", "SKIP"},
		},
		{
			name:           "python-style data and prefixed batch wording",
			dataStyle:      true,
			wantOutput:     []string{"All limit tests passed"},
			unwantedOutput: []string{"✗ FAIL", "SKIP"},
		},
		{
			name:           "uncapped runtimes skip instead of failing",
			uncapped:       true,
			wantOutput:     []string{"SKIP", "All limit tests passed"},
			unwantedOutput: []string{"✗ FAIL"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(limitHarnessServer(tc.dataStyle, tc.uncapped))
			defer server.Close()

			cmd := exec.Command(bash, filepath.Join("test_limits.sh"), server.URL)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("test_limits.sh failed: %v\n%s", err, out)
			}
			for _, want := range tc.wantOutput {
				if !strings.Contains(string(out), want) {
					t.Errorf("expected output to contain %q, got:\n%s", want, out)
				}
			}
			for _, unwanted := range tc.unwantedOutput {
				if strings.Contains(string(out), unwanted) {
					t.Errorf("did not expect output to contain %q, got:\n%s", unwanted, out)
				}
			}
		})
	}
}

func limitHarnessServer(dataStyle, uncapped bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if !uncapped && r.ContentLength > 1024*1024 {
			if dataStyle {
				fmt.Fprint(w, `{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"Request body too large"},"id":null}`)
			} else {
				fmt.Fprint(w, `{"jsonrpc":"2.0","error":{"code":-32600,"message":"Request body too large"},"id":null}`)
			}
			return
		}

		var body json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			fmt.Fprint(w, `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error"},"id":null}`)
			return
		}

		var batch []json.RawMessage
		if err := json.Unmarshal(body, &batch); err == nil {
			if uncapped {
				fmt.Fprint(w, `[{"jsonrpc":"2.0","result":0,"id":1}]`)
				return
			}
			if len(batch) > 100 {
				if dataStyle {
					fmt.Fprint(w, `{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"Batch exceeds maximum size of 100"},"id":null}`)
				} else {
					fmt.Fprint(w, `{"jsonrpc":"2.0","error":{"code":-32600,"message":"Batch exceeds maximum size"},"id":null}`)
				}
				return
			}
		}

		fmt.Fprint(w, `{"jsonrpc":"2.0","result":{},"id":1}`)
	})
}

func findBash(t *testing.T) string {
	t.Helper()
	for _, candidate := range []string{`C:\Program Files\Git\bin\bash.exe`, `C:\Program Files\Git\usr\bin\bash.exe`} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available on PATH")
	}
	return bash
}

// core-11: the advertised runtime matrix must cover all seven runtimes and be
// reachable from `make test-all-runtimes`.
func TestAllRuntimesMatrixIsComplete(t *testing.T) {
	data, err := os.ReadFile("test_all_runtimes.sh")
	if err != nil {
		t.Fatalf("read test_all_runtimes.sh: %v", err)
	}
	script := string(data)
	for _, runtime := range []string{"python", "ts", "csharp", "java", "go", "python2", "ts-cjs"} {
		if !strings.Contains(script, `["`+runtime+`"]=`) {
			t.Errorf("RUNTIME_PORTS is missing %q", runtime)
		}
		if !strings.Contains(script, "for runtime in ") || !strings.Contains(script, runtime) {
			t.Errorf("runtime loop is missing %q", runtime)
		}
	}

	mk, err := os.ReadFile(filepath.Join("..", "..", "Makefile"))
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	if !strings.Contains(string(mk), "test-all-runtimes:") || !strings.Contains(string(mk), "test_all_runtimes.sh") {
		t.Errorf("Makefile does not wire up test-all-runtimes")
	}
}

// The runtime matrix must be real, not nominal: test-servers.sh has to launch
// python2 and ts-cjs as well.
func TestTestServersConfiguresAllRuntimes(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "test-servers.sh"))
	if err != nil {
		t.Fatalf("read test-servers.sh: %v", err)
	}
	script := string(data)
	for _, runtime := range []string{"python", "ts", "csharp", "java", "go", "python2", "ts-cjs"} {
		if !strings.Contains(script, runtime+":") {
			t.Errorf("RUNTIMES is missing %q", runtime)
		}
	}
}

// Orphaned harness scripts must be reachable from the root Makefile, and the
// dead Java runtime target must have a documented replacement.
func TestMakefileWiresIntegrationScripts(t *testing.T) {
	mk, err := os.ReadFile(filepath.Join("..", "..", "Makefile"))
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	text := string(mk)

	for _, script := range []string{
		"test_numeric_types.sh",
		"test_enum_case.sh",
		"test_raises_propagation.sh",
		"test_static_client_ts.sh",
		"test_java_compile.sh",
		"test_errors.sh",
	} {
		if !strings.Contains(text, script) {
			t.Errorf("Makefile does not reference %s", script)
		}
	}

	if !strings.Contains(text, "test-integration-java:") {
		t.Error("Makefile is missing the Java integration replacement target")
	}
	if !strings.Contains(text, "test-integration-scripts:") {
		t.Error("Makefile is missing test-integration-scripts")
	}
}

// The cross-generator suite is gated behind PULSERPC_CROSS_GENERATOR=1, so it
// is unreachable unless a non-default make target opts in.
func TestCrossGeneratorMakeTargetIsOptIn(t *testing.T) {
	mk, err := os.ReadFile(filepath.Join("..", "..", "Makefile"))
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	text := string(mk)

	if !strings.Contains(text, "test-cross-generator:") {
		t.Error("Makefile is missing the test-cross-generator target")
	}
	if !strings.Contains(text, "PULSERPC_CROSS_GENERATOR=1") {
		t.Error("test-cross-generator must set PULSERPC_CROSS_GENERATOR=1")
	}
	// The default/CI paths must not set the gate implicitly.
	for _, target := range []string{"test:", "quality-full:", "test-integration:", "test-all-runtimes:"} {
		idx := strings.Index(text, "\n"+target)
		if idx < 0 {
			continue
		}
		end := strings.Index(text[idx+1:], "\n\n")
		if end < 0 {
			end = len(text) - idx - 1
		}
		block := text[idx+1 : idx+1+end]
		if strings.Contains(block, "PULSERPC_CROSS_GENERATOR=1") {
			t.Errorf("%s must not enable the cross-generator gate", target)
		}
	}
}

// Every cross-generator variant must only set flags its generator registers.
// This runs even when the gated suite is skipped, so a renamed flag surfaces as
// a clear failure instead of a panic inside fs.Lookup.
func TestCrossGeneratorVariantFlagsAreRegistered(t *testing.T) {
	for _, variant := range crossGeneratorVariants() {
		variant := variant
		t.Run(variant.name, func(t *testing.T) {
			plugin := variant.new()
			fs := flag.NewFlagSet(variant.name, flag.ContinueOnError)
			fs.String("dir", t.TempDir(), "Output directory")
			fs.String("base-dir", "", "Base directory")
			plugin.RegisterFlags(fs)
			if variant.configure != nil {
				variant.configure(t, fs)
			}
		})
	}
}

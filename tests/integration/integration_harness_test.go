package integration

import (
	"encoding/json"
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

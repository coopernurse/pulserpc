package generator

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGoGeneratedServerPreservesExactOptionalIntegers(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}
	idl := parseTestIDL(t, `namespace exact

struct Payload {
    requiredValue int
    optionalValue int [optional]
    optionalFloat float [optional]
}

interface Probe {
    Echo(req Payload) Payload
}
`)
	dir := t.TempDir()
	flags := flag.NewFlagSet("exact", flag.ContinueOnError)
	flags.String("dir", filepath.Join(dir, "rpc"), "")
	flags.String("package", "example.com/exactprobe/rpc", "")
	if err := NewGoClientServer().Generate(idl, flags); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for name, contents := range map[string]string{
		"go.mod":        "module example.com/exactprobe\n\ngo 1.25.4\n",
		"probe_test.go": exactIntGeneratedProbe,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "test", ".")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated server test failed: %v\n%s", err, output)
	}
}

const exactIntGeneratedProbe = `package exactprobe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/exactprobe/rpc/exact"
)

type echo struct { calls int }

func (e *echo) Echo(ctx context.Context, payload exact.Payload) (exact.Payload, error) {
	e.calls++
	return payload, nil
}

func TestNumericParams(t *testing.T) {
	server := exact.NewPulseRPCServer("", 0)
	handler := &echo{}
	if err := server.Register("Probe", handler); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		params string
		want string
		calls int
	}{
		{"{\"requiredValue\":9007199254740993e0,\"optionalValue\":9223372036854775807.0,\"optionalFloat\":1.000000000000000000001}", "\"result\":{\"requiredValue\":9007199254740993,\"optionalValue\":9223372036854775807,\"optionalFloat\":1}", 1},
		{"{\"requiredValue\":1e0,\"optionalValue\":-92233720368547758080e-1}", "\"result\":{\"requiredValue\":1,\"optionalValue\":-9223372036854775808}", 2},
		{"{\"requiredValue\":9007199254740992.5}", "\"code\":-32602", 2},
		{"{\"requiredValue\":1,\"optionalValue\":9223372036854775808.0}", "\"code\":-32602", 2},
	}
	for _, test := range tests {
		body := "{\"jsonrpc\":\"2.0\",\"id\":9223372036854775807,\"method\":\"Probe.Echo\",\"params\":[" + test.params + "]}"
		w := httptest.NewRecorder()
		server.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
		if w.Code != 200 || handler.calls != test.calls ||
			!strings.Contains(w.Body.String(), "\"id\":9223372036854775807") ||
			!strings.Contains(w.Body.String(), test.want) {
			t.Errorf("%s returned %d %s, handler calls %d", test.params, w.Code, w.Body.String(), handler.calls)
		}
	}
}
`

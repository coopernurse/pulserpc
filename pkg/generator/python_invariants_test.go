package generator

import (
	"flag"
	"path/filepath"
	"strings"
	"testing"
)

// R17/R19/D6: the generated Python client tree copies the runtime transport,
// which must enforce a 10 MiB response cap and a 30 s default request timeout.
func TestPythonGeneratedTransportHardening(t *testing.T) {
	dir := newPythonTestTempDir(t, "pulserpc-python-invariants-")
	idl := parseTestIDL(t, "namespace test\ninterface Svc { Ping() string }\n")

	p := NewPythonClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", "", "output dir")
	p.RegisterFlags(fs)
	if err := fs.Set("dir", dir); err != nil {
		t.Fatalf("set dir: %v", err)
	}
	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	tree := readTree(t, dir)
	var transport string
	for rel, data := range tree {
		if filepath.Base(rel) == "transport.py" {
			transport = string(data)
			break
		}
	}
	if transport == "" {
		t.Fatal("generated client tree has no pulserpc/transport.py")
	}

	for _, want := range []string{
		"DEFAULT_TIMEOUT = 30",
		"MAX_RESPONSE_BODY_SIZE = 10 * 1024 * 1024",
		"self.max_response_size + 1",
		"timeout=self.timeout",
		"Response body exceeds maximum size of",
	} {
		if !strings.Contains(transport, want) {
			t.Errorf("generated Python transport missing %q", want)
		}
	}
}

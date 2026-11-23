package generator

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coopernurse/pulserpc/pkg/parser"
	"github.com/coopernurse/pulserpc/pkg/runtime"
)

// An interface declared in an imported .pulse file carries a qualified IDL
// name ("alpha.Alpha"). That qualifier is part of the JSON-RPC method name,
// but interpolating it into Go identifiers produced `type alpha.AlphaClient`,
// which does not parse. Identifiers must use the base name while method
// strings stay fully qualified.
func TestGoImportedInterfaceIdentifiersAndMethodName(t *testing.T) {
	dir := t.TempDir()
	alphaPath := filepath.Join(dir, "alpha.pulse")
	rootPath := filepath.Join(dir, "root.pulse")
	if err := os.WriteFile(alphaPath, []byte("namespace alpha\n\ninterface Alpha {\n  Ping() string\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rootPath, []byte("namespace gamma\n\nimport \"alpha.pulse\"\n\ninterface Gamma {\n  Status() string\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	idl, err := parser.ParseIDL(rootPath, string(src))
	if err != nil {
		t.Fatalf("ParseIDL: %v", err)
	}

	outDir := filepath.Join(dir, "out")
	p := NewGoClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", "", "output dir")
	p.RegisterFlags(fs)
	for name, value := range map[string]string{
		"dir":     outDir,
		"package": "example.com/importtest/rpc",
	} {
		if err := fs.Set(name, value); err != nil {
			t.Fatalf("set %s: %v", name, err)
		}
	}
	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	client, err := os.ReadFile(filepath.Join(outDir, "alpha", "client.go"))
	if err != nil {
		t.Fatal(err)
	}
	server, err := os.ReadFile(filepath.Join(outDir, "alpha", "server.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"type AlphaClient struct",
		"func NewAlphaClient(",
		"func (c *AlphaClient) Ping(",
		`methodName := "alpha.Alpha.Ping"`,
		`ValidateRequest("alpha.Alpha", "Ping", params)`,
		`ValidateResponse("alpha.Alpha", "Ping", result)`,
	} {
		if !strings.Contains(string(client), want) {
			t.Errorf("client.go missing %q:\n%s", want, client)
		}
	}
	for _, want := range []string{"type Alpha interface", "type Gamma interface"} {
		if !strings.Contains(string(server), want) {
			t.Errorf("server.go missing %q:\n%s", want, server)
		}
	}
	for _, bad := range []string{
		"type alpha.AlphaClient", "func Newalpha.AlphaClient",
		"func (c *alpha.AlphaClient)", "type alpha.Alpha interface",
	} {
		if strings.Contains(string(client), bad) || strings.Contains(string(server), bad) {
			t.Errorf("generated Go still contains invalid qualified identifier %q", bad)
		}
	}

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH; compile check skipped")
	}
	if err := os.WriteFile(filepath.Join(outDir, "go.mod"), []byte("module example.com/importtest/rpc\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = outDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated Go does not build: %v\n%s", err, output)
	}
}

// Generated Go packages must not ship the runtime's own tests: they are
// upstream-only, and the spec-consistency test reads a Markdown file from the
// PulseRPC checkout. Keeping it upstream but omitting it from generated output
// lets `go test ./...` pass in a fresh module outside the checkout.
func TestGoGeneratedRuntimeOmitsUpstreamTests(t *testing.T) {
	embedded, err := runtime.GetRuntimeFiles("go")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := embedded["severity_spec_test.go"]; !ok {
		t.Fatal("upstream Go runtime lost severity_spec_test.go; the spec check must stay upstream")
	}

	dir := t.TempDir()
	idlPath := filepath.Join(dir, "alpha.pulse")
	idlSrc := "namespace alpha\n\ninterface Alpha {\n  Ping() string\n}\n"
	if err := os.WriteFile(idlPath, []byte(idlSrc), 0644); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(idlPath)
	if err != nil {
		t.Fatal(err)
	}
	idl, err := parser.ParseIDL(idlPath, string(src))
	if err != nil {
		t.Fatalf("ParseIDL: %v", err)
	}

	outDir := filepath.Join(dir, "out")
	p := NewGoClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", "", "output dir")
	p.RegisterFlags(fs)
	for name, value := range map[string]string{
		"dir":     outDir,
		"package": "example.com/runtimetest/rpc",
	} {
		if err := fs.Set(name, value); err != nil {
			t.Fatalf("set %s: %v", name, err)
		}
	}
	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	runtimeDir := filepath.Join(outDir, "pulserpc")
	entries, err := os.ReadDir(runtimeDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), "_test.go") {
			t.Errorf("generated runtime ships upstream test %s", entry.Name())
		}
	}
	for _, name := range []string{"validation.go", "severity.go", "server.go", "types.go"} {
		if _, err := os.Stat(filepath.Join(runtimeDir, name)); err != nil {
			t.Errorf("generated runtime is missing production file %s: %v", name, err)
		}
	}

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH; go test check skipped")
	}
	if err := os.WriteFile(filepath.Join(outDir, "go.mod"), []byte("module example.com/runtimetest/rpc\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = outDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go test ./... failed in generated module: %v\n%s", err, output)
	}
}

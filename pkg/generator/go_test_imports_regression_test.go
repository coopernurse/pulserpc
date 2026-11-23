package generator

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coopernurse/pulserpc/pkg/parser"
)

// A minimal generated Go test program must not import stdlib packages it does
// not use. The test server used to emit math/strings unconditionally, which
// broke "go build" for fixtures without sqrt/repeat methods.
func TestGoTestFilesOmitUnusedStdlibImports(t *testing.T) {
	dir := t.TempDir()
	idlPath := filepath.Join(dir, "minimal.pulse")
	idlSrc := "namespace minimal\n\ninterface Greeter {\n  get(a int) int\n  ping() string\n}\n"
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
	// generate-test-files is registered by the CLI, not by the generator.
	fs.Bool("generate-test-files", false, "generate test files")
	p.RegisterFlags(fs)
	for name, value := range map[string]string{
		"dir":                 outDir,
		"package":             "example.com/mintest/rpc",
		"generate-test-files": "true",
	} {
		if err := fs.Set(name, value); err != nil {
			t.Fatalf("set %s: %v", name, err)
		}
	}
	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	for _, rel := range []string{
		filepath.Join("cmd", "test_server", "main.go"),
		filepath.Join("cmd", "test_client", "main.go"),
	} {
		content, err := os.ReadFile(filepath.Join(outDir, rel))
		if err != nil {
			t.Fatal(err)
		}
		for _, unused := range []string{"\t\"math\"\n", "\t\"strings\"\n"} {
			if strings.Contains(string(content), unused) {
				t.Errorf("%s imports a package it does not use (%q):\n%s", rel, strings.TrimSpace(unused), content)
			}
		}
	}

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH; compile check skipped")
	}
	modPath := filepath.Join(outDir, "go.mod")
	if err := os.WriteFile(modPath, []byte("module example.com/mintest/rpc\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = outDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated Go does not build: %v\n%s", err, output)
	}
}

// A conformance-style fixture (e.g. examples/conform.pulse with sqrt/repeat)
// emits math.Sqrt / strings.ToUpper in the test-server method bodies, so those
// imports must still be present.
func TestGoTestFilesKeepUsedStdlibImports(t *testing.T) {
	idlPath := filepath.Join(t.TempDir(), "fixture.pulse")
	idlSrc := "namespace fixture\n\ninterface A {\n  sqrt(a float) float\n  repeat(req1 RepeatRequest) RepeatResponse\n}\n"
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

	structMap := map[string]*parser.Struct{}
	enumMap := map[string]*parser.Enum{}
	code := generateTestServerGo(idl, structMap, enumMap,
		"example.com/fixture/rpc/fixture", "example.com/fixture/rpc",
		GroupTypesByNamespace(idl), "fixture", "example.com/fixture/rpc/pulserpc")
	for _, used := range []string{"\t\"math\"\n", "\t\"strings\"\n"} {
		if !strings.Contains(code, used) {
			t.Errorf("conformance-style fixture missing needed import %q:\n%s", strings.TrimSpace(used), code)
		}
	}
}

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

// A shared/imported namespace may have any name. Struct fields that reference
// its types must be emitted with the namespace qualifier and the package must
// be imported, or the generated Go does not compile (reported against v0.3.6
// and still reproducible on main before this fix).
func TestGoSharedNamespaceStructFieldIsQualified(t *testing.T) {
	dir := t.TempDir()
	commonPath := filepath.Join(dir, "common.pulse")
	appPath := filepath.Join(dir, "app.pulse")
	if err := os.WriteFile(commonPath, []byte("namespace common\n\nstruct Thing {\n  id int\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(appPath, []byte("namespace app\n\nimport \"common.pulse\"\n\nstruct Holder {\n  thing common.Thing\n}\n\ninterface A {\n  get(h Holder) Holder\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(appPath)
	if err != nil {
		t.Fatal(err)
	}
	idl, err := parser.ParseIDL(appPath, string(src))
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
		"package": "example.com/sharedtest/rpc",
	} {
		if err := fs.Set(name, value); err != nil {
			t.Fatalf("set %s: %v", name, err)
		}
	}
	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	typesPath := filepath.Join(outDir, "app", "app.go")
	content, err := os.ReadFile(typesPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "Thing common.Thing") {
		t.Fatalf("generated struct field is not namespace-qualified:\n%s", content)
	}
	if !strings.Contains(string(content), `example.com/sharedtest/rpc/common`) {
		t.Fatalf("generated types file does not import the shared namespace:\n%s", content)
	}

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH; compile check skipped")
	}
	modPath := filepath.Join(outDir, "go.mod")
	if err := os.WriteFile(modPath, []byte("module example.com/sharedtest/rpc\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "./app/...", "./common/...", "./pulserpc/...")
	cmd.Dir = outDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated Go does not build: %v\n%s", err, output)
	}
}

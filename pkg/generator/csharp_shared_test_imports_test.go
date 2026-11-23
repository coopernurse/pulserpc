package generator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coopernurse/pulserpc/pkg/parser"
)

// A struct field whose type comes from a shared namespace used to make the
// generated test files uncompilable: the field value fell back to a string
// literal and the raw lowercase IDL namespace was emitted as a C# namespace.
func TestCSharpSharedStructInGeneratedTestFiles(t *testing.T) {
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
	runCSharpGenerateWithPackage(t, idl, outDir, "MyApp.Lib.Rpc")

	for _, name := range []string{"TestServer.cs", "TestClient.cs"} {
		data, err := os.ReadFile(filepath.Join(outDir, name))
		if err != nil {
			t.Fatal(err)
		}
		code := string(data)
		if strings.Contains(code, "Thing = \"test\"") {
			t.Errorf("%s: shared struct field still emitted as a string literal", name)
		}
		if strings.Contains(code, "new common.Thing") {
			t.Errorf("%s: shared struct emitted with raw lowercase IDL namespace", name)
		}
		if !strings.Contains(code, "new Thing") {
			t.Errorf("%s: shared struct not emitted by base name:\n%s", name, code)
		}
		if !strings.Contains(code, "using MyApp.Lib.Rpc.Common;") {
			t.Errorf("%s: generated test file does not import the shared namespace:\n%s", name, code)
		}
	}

	if _, err := exec.LookPath("dotnet"); err != nil {
		t.Skip("dotnet not on PATH; generated-project build skipped")
	}
	for _, project := range []string{"TestServer.csproj", "TestClient.csproj"} {
		cmd := exec.Command("dotnet", "build", project, "--nologo")
		cmd.Dir = outDir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("generated %s does not build: %v\n%s", project, err, output)
		}
	}
}

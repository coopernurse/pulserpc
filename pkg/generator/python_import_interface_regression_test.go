package generator

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCrossNamespaceFixturePythonIdentifiers guards the Python half of the
// imported-interface fix: generated identifiers use the base name while the
// qualified IDL name stays in wire strings, IDL metadata and handler
// registration. The committed fixture's imported interface is
// common.CommonOps, which used to emit "class common.CommonOps(abc.ABC):".
func TestCrossNamespaceFixturePythonIdentifiers(t *testing.T) {
	idl := parseCrossNamespaceFixture(t)
	outDir := t.TempDir()

	p := NewPythonClientServer()
	fs := flag.NewFlagSet("python", flag.ContinueOnError)
	fs.String("dir", outDir, "")
	fs.Bool("generate-test-files", false, "")
	p.RegisterFlags(fs)
	for _, setting := range [][2]string{
		{"dir", outDir},
		{"use-pydantic", "true"},
		{"generate-test-files", "true"},
	} {
		if err := fs.Set(setting[0], setting[1]); err != nil {
			t.Fatalf("set -%s: %v", setting[0], err)
		}
	}
	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate Python: %v", err)
	}

	// Base-name identifiers in every generated code site.
	requireContains(t, "common/server.py", readCrossNSFile(t, filepath.Join(outDir, "common", "server.py")),
		"class CommonOps(abc.ABC):",
		"    def Ping(self, ctx):",
		"    def Echo(self, ctx, item):",
	)
	requireContains(t, "common/client.py", readCrossNSFile(t, filepath.Join(outDir, "common", "client.py")),
		"# result = client.CommonOps.Ping(",
	)
	requireContains(t, "common/test_server.py", readCrossNSFile(t, filepath.Join(outDir, "common", "test_server.py")),
		"class CommonOpsImpl(CommonOps):",
		`rpc_server.add_handler("common.CommonOps", CommonOpsImpl())`,
	)
	requireContains(t, "common/test_client.py", readCrossNSFile(t, filepath.Join(outDir, "common", "test_client.py")),
		"result = client.CommonOps.Ping()",
		"result = client.CommonOps.Echo(",
	)
	requireContains(t, "models.py", readCrossNSFile(t, filepath.Join(outDir, "models.py")),
		"class Mode(BaseModel):",
		"class Shared(BaseModel):",
	)

	// Qualified names survive where they belong: wire strings, IDL metadata
	// and the server's handler registration key.
	requireContains(t, "common/test_client.py", readCrossNSFile(t, filepath.Join(outDir, "common", "test_client.py")),
		"# Test common.CommonOps.Ping",
		`error_msg = "common.CommonOps.Ping failed: {}"`,
	)
	requireContains(t, "app/idl.json", readCrossNSFile(t, filepath.Join(outDir, "app", "idl.json")),
		`"name": "common.CommonOps"`,
		`"name": "Ping"`,
		`"name": "Echo"`,
	)

	// No raw dotted identifier may remain in any generated Python file.
	badIdentifiers := []string{
		"class common.CommonOps",
		"common.CommonOpsImpl",
		"client.common.CommonOps",
		"class common.Mode",
		"class common.Shared",
	}
	var pyFiles []string
	if err := filepath.Walk(outDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".py") {
			pyFiles = append(pyFiles, path)
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			content := string(data)
			rel, _ := filepath.Rel(outDir, path)
			for _, bad := range badIdentifiers {
				if strings.Contains(content, bad) {
					t.Errorf("%s still contains invalid dotted identifier %q", rel, bad)
				}
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("walk generated Python files: %v", err)
	}
	if len(pyFiles) == 0 {
		t.Fatal("no Python files were generated")
	}

	// Syntax-check every generated file. A missing interpreter is an
	// environment skip, not a known-gap skip.
	pythonExe, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not on PATH; py_compile check skipped")
	}
	for _, path := range pyFiles {
		out, err := exec.Command(pythonExe, "-m", "py_compile", path).CombinedOutput()
		if err != nil {
			t.Errorf("python3 -m py_compile %s failed: %v\n%s", path, err, out)
		}
	}
}

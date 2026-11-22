package generator

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/coopernurse/pulserpc/pkg/parser"
)

// fromImportLine matches a Python `from <module> import <members>` statement.
var fromImportLine = regexp.MustCompile(`^\s*from\s+\S+\s+import\s+(.+?)\s*$`)

// qualifiedImportMember matches a dotted name such as `inc.Response` when it
// appears as a member of a from-import (rather than as the module path).
var qualifiedImportMember = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)+$`)

// TestPythonNamespacedImportsAreValidMembers is a regression test for the
// multi-runtime conformance build. Parsing examples/conform.pulse merges the
// imported `inc` namespace into the root IDL, so the types of that namespace
// carry qualified names (inc.Response, inc.Status, inc.MathOp). Those names
// must be unqualified when emitted as members of a Python from-import:
//
//	from inc.rpctypes import Response, Status, MathOp   // valid
//	from inc.rpctypes import inc.Response, ...          // SyntaxError
func TestPythonNamespacedImportsAreValidMembers(t *testing.T) {
	idlPath := filepath.Join("..", "..", "examples", "conform.pulse")
	src, err := os.ReadFile(idlPath)
	if err != nil {
		t.Fatalf("read fixture %s: %v", idlPath, err)
	}
	idl, err := parser.ParseIDL(idlPath, string(src))
	if err != nil {
		t.Fatalf("ParseIDL(%s): %v", idlPath, err)
	}

	// The fixture must actually exercise more than one namespace, otherwise
	// the assertions below would be vacuous.
	namespaceMap := GroupTypesByNamespace(idl)
	if len(namespaceMap) < 2 || namespaceMap["inc"] == nil {
		t.Fatalf("fixture should provide the imported inc namespace; got namespaces %v", CollectNamespaces(namespaceMap))
	}

	tmpDir := newPythonTestTempDir(t, "pulserpc-ns-imports-")
	p := NewPythonClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", "", "output dir")
	fs.Bool("generate-test-files", false, "generate test files")
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("set dir flag: %v", err)
	}
	if err := fs.Set("generate-test-files", "true"); err != nil {
		t.Fatalf("set generate-test-files flag: %v", err)
	}
	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// No generated .py file may contain a dotted name in the member list of a
	// `from ... import ...` statement. The module path itself stays qualified.
	pyFiles := make([]string, 0)
	err = filepath.WalkDir(tmpDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".py") {
			return nil
		}
		pyFiles = append(pyFiles, path)
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(content), "\n") {
			match := fromImportLine.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			for _, member := range strings.Split(match[1], ",") {
				member = strings.TrimSpace(member)
				if qualifiedImportMember.MatchString(member) {
					t.Errorf("%s:%d: qualified name %q in from-import member list: %s",
						path, i+1, member, strings.TrimSpace(line))
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk generated output: %v", err)
	}
	if len(pyFiles) == 0 {
		t.Fatal("no Python files were generated")
	}

	// The inc namespace types must be imported by base name, with the module
	// path still namespace-qualified (test_server.py) or relative (server.py).
	incTestServer := filepath.Join(tmpDir, "inc", "test_server.py")
	assertFileContains(t, incTestServer, "from inc.rpctypes import Response")
	for _, name := range []string{"Response", "Status", "MathOp"} {
		assertFileContains(t, incTestServer, name)
	}
	assertFileContains(t, filepath.Join(tmpDir, "inc", "server.py"), "from .rpctypes import Response")

	// Syntax-check the generated stubs with Python 3 when an interpreter is
	// available. This is what failed in the conformance job before the fix.
	pythonExe, err := exec.LookPath("python3")
	if err != nil {
		pythonExe, err = exec.LookPath("python")
	}
	if err != nil {
		t.Skip("no Python interpreter on PATH; py_compile check skipped")
	}
	for _, path := range []string{
		filepath.Join(tmpDir, "inc", "test_server.py"),
		filepath.Join(tmpDir, "inc", "server.py"),
		filepath.Join(tmpDir, "conform", "test_server.py"),
		filepath.Join(tmpDir, "conform", "server.py"),
	} {
		out, err := exec.Command(pythonExe, "-m", "py_compile", path).CombinedOutput()
		if err != nil {
			t.Errorf("python3 -m py_compile %s failed: %v\n%s", path, err, out)
		}
	}
}

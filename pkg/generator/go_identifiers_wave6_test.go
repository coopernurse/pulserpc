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

// wave6GenerateRaw writes the given .pulse files, parses and validates the
// root, generates Go, adds a go.mod and returns (output dir, Generate error).
func wave6GenerateRaw(t *testing.T, files map[string]string, root string, testFiles bool) (string, error) {
	t.Helper()
	dir := t.TempDir()
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0644); err != nil {
			t.Fatal(err)
		}
	}
	rootPath := filepath.Join(dir, root)
	src, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	idl, err := parser.ParseIDL(rootPath, string(src))
	if err != nil {
		t.Fatalf("ParseIDL: %v", err)
	}
	if err := parser.ValidateIDL(idl); err != nil {
		t.Fatalf("ValidateIDL rejected input: %v", err)
	}
	out := filepath.Join(dir, "out")
	fs := flag.NewFlagSet("wave6", flag.ContinueOnError)
	fs.String("dir", "", "")
	fs.Bool("generate-test-files", false, "")
	p := NewGoClientServer()
	p.RegisterFlags(fs)
	for n, v := range map[string]string{"dir": out, "package": "example.com/wave6/rpc"} {
		if err := fs.Set(n, v); err != nil {
			t.Fatal(err)
		}
	}
	if testFiles {
		if err := fs.Set("generate-test-files", "true"); err != nil {
			t.Fatal(err)
		}
	}
	if err := p.Generate(idl, fs); err != nil {
		return out, err
	}
	if err := os.WriteFile(filepath.Join(out, "go.mod"), []byte("module example.com/wave6/rpc\n\ngo 1.25.4\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return out, nil
}

func wave6Generate(t *testing.T, files map[string]string, root string, testFiles bool) string {
	t.Helper()
	out, err := wave6GenerateRaw(t, files, root, testFiles)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return out
}

func wave6Go(t *testing.T, out string, args ...string) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = out
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go %v failed: %v\n%s", args, err, b)
	}
}

func wave6Read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func wave6Want(t *testing.T, code string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(code, want) {
			t.Errorf("generated Go missing %q", want)
		}
	}
}

// Base names of interfaces from different namespaces must not collide in the
// single generated package, while JSON-RPC method strings keep the qualifier.
func TestGoImportedInterfaceBaseNameCollision(t *testing.T) {
	out := wave6Generate(t, map[string]string{
		"alpha.pulse": "namespace alpha\n\ninterface Alpha {\n  Ping() string\n}\n",
		"beta.pulse":  "namespace beta\n\ninterface Alpha {\n  Pong() string\n}\n",
		"root.pulse":  "namespace gamma\n\nimport \"alpha.pulse\"\nimport \"beta.pulse\"\n\ninterface Gamma {\n  Status() string\n}\n",
	}, "root.pulse", false)

	server := wave6Read(t, filepath.Join(out, "alpha", "server.go"))
	client := wave6Read(t, filepath.Join(out, "alpha", "client.go"))
	wave6Want(t, server,
		"type alphaAlpha interface", "type betaAlpha interface", "type Gamma interface")
	wave6Want(t, client,
		"type alphaAlphaClient struct", "type betaAlphaClient struct",
		`methodName := "alpha.Alpha.Ping"`, `methodName := "beta.Alpha.Pong"`,
		`ValidateRequest("alpha.Alpha", "Ping", params)`,
	)
	wave6Go(t, out, "build", "./...")
}

// A namespace becomes a Go package name; reserved words must be rejected with
// a clear error rather than emitting unparseable `package type`.
func TestGoRejectsGoKeywordNamespace(t *testing.T) {
	_, err := wave6GenerateRaw(t, map[string]string{
		"root.pulse": "namespace type\n\ninterface Svc {\n  Ping() string\n}\n",
	}, "root.pulse", false)
	if err == nil || !strings.Contains(err.Error(), "reserved word") {
		t.Fatalf("want reserved-word error for namespace type, got %v", err)
	}
}

// IDL parameter names can be Go keywords; emitted Go identifiers are suffixed.
func TestGoKeywordParameterNames(t *testing.T) {
	out := wave6Generate(t, map[string]string{
		"root.pulse": "namespace kw\n\ninterface Svc {\n  Ping(type string) string\n}\n",
	}, "root.pulse", false)
	server := wave6Read(t, filepath.Join(out, "kw", "server.go"))
	client := wave6Read(t, filepath.Join(out, "kw", "client.go"))
	wave6Want(t, server, "Ping(ctx context.Context, type_ string) (string, error)")
	wave6Want(t, client, "Ping(ctx context.Context, type_ string) (string, error)", "\t\ttype_,\n")
	wave6Go(t, out, "build", "./...")
}

// An idempotent re-generation must evict upstream *_test.go files left by an
// older generator, otherwise `go test ./...` fails in a fresh module.
func TestGoRegenerationRemovesStaleUpstreamTests(t *testing.T) {
	files := map[string]string{
		"root.pulse": "namespace alpha\n\ninterface Alpha {\n  Ping() string\n}\n",
	}
	out := wave6Generate(t, files, "root.pulse", false)
	embedded, err := runtime.GetRuntimeFiles("go")
	if err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(out, "pulserpc", "transport_test.go")
	if err := os.WriteFile(stale, embedded["transport_test.go"], 0644); err != nil {
		t.Fatal(err)
	}
	// Regenerate into the same output directory (the idempotent path).
	rootPath := filepath.Join(filepath.Dir(out), "root.pulse")
	idl, err := parser.ParseIDL(rootPath, files["root.pulse"])
	if err != nil {
		t.Fatal(err)
	}
	fs := flag.NewFlagSet("wave6b", flag.ContinueOnError)
	fs.String("dir", "", "")
	p := NewGoClientServer()
	p.RegisterFlags(fs)
	fs.Set("dir", out)
	fs.Set("package", "example.com/wave6/rpc")
	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	if _, err := os.Stat(stale); err == nil {
		t.Fatal("stale upstream _test.go survived regeneration")
	}
	wave6Go(t, out, "test", "./...")
}

// Distinct IDL enum values can collapse under snakeToCamelCase; generated Go
// const identifiers must stay unique.
func TestGoEnumValueCamelCaseCollision(t *testing.T) {
	out := wave6Generate(t, map[string]string{
		"root.pulse": "namespace en\n\nenum Color {\n  foo_bar\n  fooBar\n}\n\ninterface Svc {\n  Ping() string\n}\n",
	}, "root.pulse", false)
	code := wave6Read(t, filepath.Join(out, "en", "en.go"))
	wave6Want(t, code, `ColorFooBar Color = "foo_bar"`, `ColorFooBar_2 Color = "fooBar"`)
	wave6Go(t, out, "build", "./...")
}

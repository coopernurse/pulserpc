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

// TestJavaRepeatStatusTypeFromIDL guards the repeat() test-body status enum.
// The enum may live in an imported namespace and need not be named "Status";
// hardcoding a namespace-qualified name emitted a reference to a package that
// does not exist (com.example.rpc.inc) for otherwise valid IDLs.
func TestJavaRepeatStatusTypeFromIDL(t *testing.T) {
	cases := []struct {
		name     string
		enumName string
		wantType string
	}{
		{name: "enum named Status in common", enumName: "Status", wantType: "com.example.rpc.common.Status.ok"},
		{name: "enum named Code in common", enumName: "Code", wantType: "com.example.rpc.common.Code.ok"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			outDir := filepath.Join(root, "out")
			if err := os.Mkdir(outDir, 0755); err != nil {
				t.Fatalf("create output dir: %v", err)
			}

			commonIDL := "namespace common\n\nenum " + tc.enumName + " {\n    ok\n}\n"
			appIDL := `namespace app

import "common.pulse"

struct RepeatRequest {
    to_repeat string
    count int
    force_uppercase bool
}

struct RepeatResponse {
    status common.` + tc.enumName + `
    count int
    items []string
}

interface A {
    repeat(req1 RepeatRequest) RepeatResponse
}
`
			for name, src := range map[string]string{"common.pulse": commonIDL, "app.pulse": appIDL} {
				if err := os.WriteFile(filepath.Join(root, name), []byte(src), 0644); err != nil {
					t.Fatalf("write fixture %s: %v", name, err)
				}
			}

			idl, err := parser.ParseIDL(filepath.Join(root, "app.pulse"), appIDL)
			if err != nil {
				t.Fatalf("parse fixture: %v", err)
			}

			p := NewJavaClientServer()
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			fs.String("dir", "", "output dir")
			fs.Bool("generate-test-files", false, "generate test files")
			p.RegisterFlags(fs)
			if err := fs.Set("dir", outDir); err != nil {
				t.Fatalf("set dir: %v", err)
			}
			if err := fs.Set("package", "com.example.rpc"); err != nil {
				t.Fatalf("set package: %v", err)
			}
			if err := fs.Set("generate-test-files", "true"); err != nil {
				t.Fatalf("set generate-test-files: %v", err)
			}
			if err := p.Generate(idl, fs); err != nil {
				t.Fatalf("Generate: %v", err)
			}

			implPath := filepath.Join(outDir, "src", "main", "java", "com", "example", "rpc", "app", "AImpl.java")
			impl, err := os.ReadFile(implPath)
			if err != nil {
				t.Fatalf("read generated impl: %v", err)
			}
			got := string(impl)
			if !strings.Contains(got, tc.wantType) {
				t.Errorf("expected generated status type %q, got:\n%s", tc.wantType, got)
			}
			if strings.Contains(got, "inc.Status") {
				t.Errorf("generated impl still references hardcoded inc.Status:\n%s", got)
			}

			mvn, err := exec.LookPath("mvn")
			if err != nil {
				t.Skip("mvn not available; skipping generated-source compile check")
			}
			cmd := exec.Command(mvn, "-q", "-DskipTests", "compile")
			cmd.Dir = outDir
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("generated Java did not compile: %v\n%s", err, output)
			}
		})
	}
}

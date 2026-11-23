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

// An interface declared in an imported .pulse file carries a qualified IDL name
// ("alpha.Alpha"). The qualifier is not a valid Java identifier, so every
// generated identifier uses the base name, while the JSON-RPC method string and
// handler registration key keep the qualified name, matching the Go generator.
func TestJavaImportedInterfaceIdentifiersAndMethodName(t *testing.T) {
	dir := t.TempDir()
	alphaPath := filepath.Join(dir, "alpha.pulse")
	rootPath := filepath.Join(dir, "root.pulse")
	if err := os.WriteFile(alphaPath, []byte("namespace alpha\n\ninterface Alpha {\n  Ping() string\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rootPath, []byte("namespace gamma\n\nimport \"alpha.pulse\"\n\ninterface Gamma {\n  Echo() string\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	outDir := generateJavaProject(t, rootPath, "com.example.rpc")

	assertJavaContains(t, filepath.Join(outDir, "src", "main", "java", "com", "example", "rpc", "alpha", "Alpha.java"),
		"public interface Alpha {")
	clientPath := filepath.Join(outDir, "src", "main", "java", "com", "example", "rpc", "alpha", "AlphaClient.java")
	assertJavaContains(t, clientPath,
		"public class AlphaClient implements Alpha {",
		`String method = "alpha.Alpha.Ping";`)
	assertJavaNotContains(t, clientPath,
		"public class alpha.Alpha", "new alpha.AlphaClient(", "interface alpha.Alpha")

	assertJavaContains(t, filepath.Join(outDir, "src", "test", "java", "com", "example", "rpc", "TestServer.java"),
		`server.register("alpha.Alpha", new com.example.rpc.alpha.AlphaImpl());`)
	assertJavaContains(t, filepath.Join(outDir, "src", "test", "java", "com", "example", "rpc", "TestClient.java"),
		"com.example.rpc.alpha.AlphaClient alphaClient = new com.example.rpc.alpha.AlphaClient(")

	// Upstream Java runtime tests live outside the embedded production tree and
	// must not be copied into a generated project.
	err := filepath.WalkDir(outDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), "Test.java") {
			t.Errorf("generated project ships upstream test %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	mvnCompile(t, outDir)
}

// Two namespaces may declare interfaces with the same base name. Java packages
// keep the files apart, but generated single-type imports, local variables and
// handler registration keys must not collapse the two into one.
func TestJavaSameBaseNameAcrossNamespaces(t *testing.T) {
	dir := t.TempDir()
	alphaPath := filepath.Join(dir, "alpha.pulse")
	rootPath := filepath.Join(dir, "root.pulse")
	if err := os.WriteFile(alphaPath, []byte("namespace alpha\n\ninterface Thing {\n  Ping() string\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rootPath, []byte("namespace gamma\n\nimport \"alpha.pulse\"\n\ninterface Thing {\n  Echo() string\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	outDir := generateJavaProject(t, rootPath, "com.example.rpc")

	assertJavaContains(t, filepath.Join(outDir, "src", "main", "java", "com", "example", "rpc", "alpha", "ThingClient.java"),
		`String method = "alpha.Thing.Ping";`)
	// Root-namespace declarations are not prefixed by the parser, so the local
	// interface keeps the simple name while the imported one is qualified.
	assertJavaContains(t, filepath.Join(outDir, "src", "main", "java", "com", "example", "rpc", "gamma", "ThingClient.java"),
		`String method = "Thing.Echo";`)

	serverPath := filepath.Join(outDir, "src", "main", "java", "com", "example", "rpc", "Server.java")
	assertJavaNotContains(t, serverPath,
		"import com.example.rpc.alpha.Thing;", "import com.example.rpc.gamma.Thing;")

	testServerPath := filepath.Join(outDir, "src", "test", "java", "com", "example", "rpc", "TestServer.java")
	assertJavaContains(t, testServerPath,
		`server.register("alpha.Thing", new com.example.rpc.alpha.ThingImpl());`,
		`server.register("Thing", new com.example.rpc.gamma.ThingImpl());`)

	testClientPath := filepath.Join(outDir, "src", "test", "java", "com", "example", "rpc", "TestClient.java")
	assertJavaContains(t, testClientPath,
		"com.example.rpc.gamma.ThingClient thingClient = new com.example.rpc.gamma.ThingClient(",
		"com.example.rpc.alpha.ThingClient alphathingClient = new com.example.rpc.alpha.ThingClient(")

	mvnCompile(t, outDir)
	mvnTest(t, outDir)
}

func generateJavaProject(t *testing.T, rootPath string, basePackage string) string {
	t.Helper()
	src, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	idl, err := parser.ParseIDL(rootPath, string(src))
	if err != nil {
		t.Fatalf("ParseIDL: %v", err)
	}
	outDir := filepath.Join(filepath.Dir(rootPath), "out")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatal(err)
	}
	p := NewJavaClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", "", "output dir")
	fs.Bool("generate-test-files", false, "generate test files")
	p.RegisterFlags(fs)
	for name, value := range map[string]string{
		"dir":                 outDir,
		"package":             basePackage,
		"generate-test-files": "true",
	} {
		if err := fs.Set(name, value); err != nil {
			t.Fatalf("set %s: %v", name, err)
		}
	}
	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return outDir
}

func mvnCompile(t *testing.T, dir string) {
	t.Helper()
	runMaven(t, dir, "-q", "-DskipTests", "compile")
}

func mvnTest(t *testing.T, dir string) {
	t.Helper()
	runMaven(t, dir, "-q", "test")
}

func runMaven(t *testing.T, dir string, args ...string) {
	t.Helper()
	mvn, err := exec.LookPath("mvn")
	if err != nil {
		t.Skip("mvn not on PATH; generated Maven project build skipped")
	}
	cmd := exec.Command(mvn, args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("mvn %v failed in generated project: %v\n%s", args, err, output)
	}
}

func assertJavaContains(t *testing.T, path string, want ...string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range want {
		if !strings.Contains(string(data), s) {
			t.Errorf("%s missing %q:\n%s", path, s, data)
		}
	}
}

func assertJavaNotContains(t *testing.T, path string, unwanted ...string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range unwanted {
		if strings.Contains(string(data), s) {
			t.Errorf("%s still contains %q:\n%s", path, s, data)
		}
	}
}

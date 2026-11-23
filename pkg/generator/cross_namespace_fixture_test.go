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

// Parse the committed fixture rather than an inline copy: the example is the
// shared reproducer for imported-interface generation across all runtimes.
func parseCrossNamespaceFixture(t *testing.T) *parser.IDL {
	t.Helper()
	path := filepath.Join("..", "..", "examples", "cross-ns", "app.pulse")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cross-ns fixture: %v", err)
	}
	idl, err := parser.ParseIDL(path, string(src))
	if err != nil {
		t.Fatalf("ParseIDL cross-ns fixture: %v", err)
	}
	return idl
}

func readCrossNSFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func requireContains(t *testing.T, label, content string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(content, want) {
			t.Errorf("%s missing %q", label, want)
		}
	}
}

// Go identifiers must use the imported interface's base name while JSON-RPC
// method strings and IDL validation keep the qualified name (commit aa56289).
// The committed fixture guards that fix against a real two-namespace example.
func TestCrossNamespaceFixtureGo(t *testing.T) {
	idl := parseCrossNamespaceFixture(t)
	outDir := t.TempDir()
	fs := flag.NewFlagSet("go", flag.ContinueOnError)
	fs.String("dir", outDir, "")
	fs.String("package", "example.com/crossns/rpc", "")
	if err := NewGoClientServer().Generate(idl, fs); err != nil {
		t.Fatalf("Generate Go: %v", err)
	}

	client := readCrossNSFile(t, filepath.Join(outDir, "app", "client.go"))
	server := readCrossNSFile(t, filepath.Join(outDir, "app", "server.go"))
	requireContains(t, "app/client.go", client,
		"type CommonOpsClient struct",
		"func NewCommonOpsClient(",
		"func (c *CommonOpsClient) Ping(",
		"func (c *CommonOpsClient) Echo(",
		`methodName := "common.CommonOps.Ping"`,
		`methodName := "common.CommonOps.Echo"`,
		`ValidateRequest("common.CommonOps", "Ping", params)`,
		`ValidateResponse("common.CommonOps", "Ping", result)`,
	)
	requireContains(t, "app/server.go", server,
		"type AppOps interface", "type CommonOps interface")
	for _, bad := range []string{
		"type common.CommonOpsClient",
		"Newcommon.CommonOpsClient",
		"type common.CommonOps interface",
	} {
		if strings.Contains(client, bad) || strings.Contains(server, bad) {
			t.Errorf("generated Go still contains invalid qualified identifier %q", bad)
		}
	}

	// The same fixture also carries the shared struct/enum, inheritance from
	// the imported namespace, an array field and an optional field.
	requireContains(t, "common/common.go", readCrossNSFile(t, filepath.Join(outDir, "common", "common.go")),
		"type Mode string", "type Shared struct", "Tags []string", "Note *string")
	requireContains(t, "app/app.go", readCrossNSFile(t, filepath.Join(outDir, "app", "app.go")),
		"type Derived struct", "common.Shared")

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH; generated-Go build skipped")
	}
	if err := os.WriteFile(filepath.Join(outDir, "go.mod"),
		[]byte("module example.com/crossns/rpc\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = outDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated Go does not build: %v\n%s", err, output)
	}
}

// Java and C# must use base-name Go-style identifiers while keeping the
// namespace-qualified name in JSON-RPC method strings and IDL metadata. Both
// generated projects must compile outside the PulseRPC checkout.
func TestCrossNamespaceFixtureJavaAndCSharp(t *testing.T) {
	idl := parseCrossNamespaceFixture(t)

	t.Run("java", func(t *testing.T) {
		outDir := t.TempDir()
		fs := flag.NewFlagSet("java", flag.ContinueOnError)
		fs.String("dir", outDir, "")
		fs.String("package", "com.example.rpc", "")
		fs.String("json-lib", "jackson", "")
		fs.Bool("generate-test-files", true, "")
		if err := NewJavaClientServer().Generate(idl, fs); err != nil {
			t.Fatalf("Generate Java: %v", err)
		}
		client := readCrossNSFile(t, filepath.Join(outDir, "src", "main", "java", "com", "example", "rpc", "Client.java"))
		requireContains(t, "Client.java", client,
			`\"name\": \"common.CommonOps\"`,
			`\"name\": \"Ping\"`,
			`\"name\": \"Echo\"`)
		// Per-interface clients live in the interface's own package; the wire
		// method string is emitted there, not in the aggregate Client.java.
		commonClient := readCrossNSFile(t, filepath.Join(outDir, "src", "main", "java", "com", "example", "rpc", "common", "CommonOpsClient.java"))
		requireContains(t, "common/CommonOpsClient.java", commonClient,
			`String method = "common.CommonOps.Ping";`,
			`String method = "common.CommonOps.Echo";`)
		for _, bad := range []string{`String method = "CommonOps.Ping"`, `String method = "CommonOps.Echo"`} {
			if strings.Contains(commonClient, bad) {
				t.Errorf("Java client uses unqualified method string %q for the imported interface", bad)
			}
		}

		mvn, err := exec.LookPath("mvn")
		if err != nil {
			t.Skip("mvn not on PATH; generated-Java compile skipped")
		}
		cmd := exec.Command(mvn, "-q", "-DskipTests", "compile")
		cmd.Dir = outDir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("generated Java does not compile: %v\n%s", err, output)
		}
	})

	t.Run("csharp", func(t *testing.T) {
		outDir := t.TempDir()
		runCSharpGenerateWithPackage(t, idl, outDir, "MyApp.Lib.Rpc")
		client := readCrossNSFile(t, filepath.Join(outDir, "Client.cs"))
		requireContains(t, "Client.cs", client,
			`""name"": ""common.CommonOps""`,
			`""name"": ""Ping""`,
			`""name"": ""Echo""`,
			"public class CommonOpsClient : ICommonOps")
		if strings.Contains(client, "public class common.CommonOpsClient") {
			t.Error("C# client class still uses the raw namespace qualifier in its identifier")
		}

		t.Run("build", func(t *testing.T) {
			dotnet, err := exec.LookPath("dotnet")
			if err != nil {
				t.Skip("dotnet not on PATH; generated-C# build skipped")
			}
			projects, err := filepath.Glob(filepath.Join(outDir, "*.csproj"))
			if err != nil {
				t.Fatal(err)
			}
			for _, project := range projects {
				cmd := exec.Command(dotnet, "build", project, "--nologo")
				cmd.Dir = outDir
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Errorf("generated %s does not build: %v\n%s", filepath.Base(project), err, output)
				}
			}
		})
	})
}

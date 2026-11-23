package generator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coopernurse/pulserpc/pkg/parser"
)

// An interface declared in an imported .pulse file carries a qualified IDL
// name ("alpha.Alpha"). That qualifier is part of the JSON-RPC method name,
// but interpolating it into C# identifiers produced `public interface
// Ialpha.Alpha` and `class alpha.AlphaClient`, which do not parse. Identifiers
// must use the base name while method strings stay fully qualified.
func TestCSharpImportedInterfaceIdentifiersAndMethodName(t *testing.T) {
	dir := t.TempDir()
	alphaPath := filepath.Join(dir, "alpha.pulse")
	rootPath := filepath.Join(dir, "gamma.pulse")
	alphaSrc := "namespace alpha\n\nstruct AlphaData {\n  id string\n}\n\ninterface Alpha {\n  Ping() string\n}\n"
	if err := os.WriteFile(alphaPath, []byte(alphaSrc), 0644); err != nil {
		t.Fatal(err)
	}
	rootSrc := "namespace gamma\n\nimport \"alpha.pulse\"\n\ninterface Gamma {\n  Status() string\n}\n"
	if err := os.WriteFile(rootPath, []byte(rootSrc), 0644); err != nil {
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
	runCSharpGenerateWithPackage(t, idl, outDir, "MyApp.Lib.Rpc")

	// Identifier sites must use the base name.
	for name, wants := range map[string][]string{
		"Contract.cs":   {"public interface IAlpha", "public interface IGamma"},
		"Server.cs":     {"public void RegisterAlpha(IAlpha implementation)", "this.Register(\"alpha.Alpha\", implementation)"},
		"Client.cs":     {"public class AlphaClient : IAlpha", "var method = \"alpha.Alpha.Ping\";"},
		"TestServer.cs": {"public class AlphaImpl : IAlpha", "server.RegisterAlpha(new AlphaImpl());"},
		"TestClient.cs": {"var alphaClient = new AlphaClient(transport);", "await alphaClient.PingAsync()"},
	} {
		data, err := os.ReadFile(filepath.Join(outDir, name))
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range wants {
			if !strings.Contains(string(data), want) {
				t.Errorf("%s missing %q:\n%s", name, want, data)
			}
		}
		for _, bad := range []string{
			"Ialpha.Alpha", "alpha.AlphaClient", "alpha.AlphaImpl",
			"Registeralpha.Alpha", "var alpha.alphaClient", "class alpha.Alpha",
		} {
			if strings.Contains(string(data), bad) {
				t.Errorf("%s still contains invalid qualified identifier %q", name, bad)
			}
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

// Generated C# output must not ship the runtime's own test files: they are
// upstream-only and a generated project must build outside the checkout.
func TestCSharpGeneratedRuntimeOmitsUpstreamTests(t *testing.T) {
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
	runCSharpGenerateWithPackage(t, idl, outDir, "MyApp.Lib.Rpc")

	runtimeDir := filepath.Join(outDir, "MyApp", "Lib", "Rpc", "PulseRPC")
	entries, err := os.ReadDir(runtimeDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, "Tests.cs") || strings.HasSuffix(name, "Test.cs") || name == "TestHelpers.cs" {
			t.Errorf("generated runtime ships upstream test %s", name)
		}
	}
	for _, name := range []string{"Client.cs", "Server.cs", "Types.cs", "Transport.cs", "Validation.cs"} {
		if _, err := os.Stat(filepath.Join(runtimeDir, name)); err != nil {
			t.Errorf("generated runtime is missing production file %s: %v", name, err)
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

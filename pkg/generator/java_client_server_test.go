package generator

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/coopernurse/pulserpc/pkg/parser"
)

func TestJavaGeneratorBasicFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulserpc-java-gen-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Build a minimal IDL with one namespace, a struct and an enum
	idl := &parser.IDL{
		Structs: []*parser.Struct{
			{
				Name:      "inc.Req",
				Namespace: "inc",
				Fields:    []*parser.Field{{Name: "msg", Type: &parser.Type{BuiltIn: "string"}}},
			},
		},
		Enums: []*parser.Enum{
			{
				Name:      "inc.Status",
				Namespace: "inc",
				Values:    []*parser.EnumValue{{Name: "ok"}},
			},
		},
		Interfaces: []*parser.Interface{
			{
				Name:      "A",
				Namespace: "",
			},
		},
	}

	p := NewJavaClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	// ensure dir flag exists
	fs.String("dir", "", "output dir")
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}
	if err := fs.Set("package", "com.example"); err != nil {
		t.Fatalf("failed to set base-package flag: %v", err)
	}

	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Check namespace Idl file
	nsPath := filepath.Join(tmpDir, "src", "main", "java", "com", "example", "inc", "incIdl.java")
	if _, err := os.Stat(nsPath); err != nil {
		t.Fatalf("expected namespace idl file at %s, missing: %v", nsPath, err)
	}

	// Check Server.java and Client.java in base package
	serverPath := filepath.Join(tmpDir, "src", "main", "java", "com", "example", "Server.java")
	if _, err := os.Stat(serverPath); err != nil {
		t.Fatalf("expected Server.java at %s, missing: %v", serverPath, err)
	}
	clientPath := filepath.Join(tmpDir, "src", "main", "java", "com", "example", "Client.java")
	if _, err := os.Stat(clientPath); err != nil {
		t.Fatalf("expected Client.java at %s, missing: %v", clientPath, err)
	}
}

func TestJavaGeneratorTestFilesWithFlag(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulserpc-java-gen-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Build a minimal IDL with an interface
	idl := &parser.IDL{
		Interfaces: []*parser.Interface{
			{
				Name:      "A",
				Namespace: "",
				Methods: []*parser.Method{
					{
						Name:       "add",
						Parameters: []*parser.Parameter{{Name: "a", Type: &parser.Type{BuiltIn: "int"}}, {Name: "b", Type: &parser.Type{BuiltIn: "int"}}},
						ReturnType: &parser.Type{BuiltIn: "int"},
					},
				},
			},
		},
	}

	p := NewJavaClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", "", "output dir")
	fs.Bool("generate-test-files", false, "generate test files")
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}
	if err := fs.Set("package", "com.example"); err != nil {
		t.Fatalf("failed to set base-package flag: %v", err)
	}
	if err := fs.Set("generate-test-files", "true"); err != nil {
		t.Fatalf("failed to set generate-test-files flag: %v", err)
	}

	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Check that test files are generated when flag is true
	testServerPath := filepath.Join(tmpDir, "src", "test", "java", "com", "example", "TestServer.java")
	if _, err := os.Stat(testServerPath); err != nil {
		t.Fatalf("expected TestServer.java at %s, missing: %v", testServerPath, err)
	}
	testClientPath := filepath.Join(tmpDir, "src", "test", "java", "com", "example", "TestClient.java")
	if _, err := os.Stat(testClientPath); err != nil {
		t.Fatalf("expected TestClient.java at %s, missing: %v", testClientPath, err)
	}
}

func TestJavaGeneratorTestFilesDisabled(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulserpc-java-gen-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Build a minimal IDL with an interface
	idl := &parser.IDL{
		Interfaces: []*parser.Interface{
			{
				Name:      "A",
				Namespace: "",
				Methods: []*parser.Method{
					{
						Name:       "add",
						Parameters: []*parser.Parameter{{Name: "a", Type: &parser.Type{BuiltIn: "int"}}, {Name: "b", Type: &parser.Type{BuiltIn: "int"}}},
						ReturnType: &parser.Type{BuiltIn: "int"},
					},
				},
			},
		},
	}

	p := NewJavaClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", "", "output dir")
	fs.Bool("generate-test-files", false, "generate test files")
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}
	if err := fs.Set("package", "com.example"); err != nil {
		t.Fatalf("failed to set base-package flag: %v", err)
	}
	// Explicitly disable test file generation
	if err := fs.Set("generate-test-files", "false"); err != nil {
		t.Fatalf("failed to set generate-test-files flag: %v", err)
	}

	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Check that test files are NOT generated when generate-test-files is false
	testServerPath := filepath.Join(tmpDir, "src", "test", "java", "com", "example", "TestServer.java")
	if _, err := os.Stat(testServerPath); err == nil {
		t.Fatalf("TestServer.java should NOT be generated when -generate-test-files=false")
	}
	testClientPath := filepath.Join(tmpDir, "src", "test", "java", "com", "example", "TestClient.java")
	if _, err := os.Stat(testClientPath); err == nil {
		t.Fatalf("TestClient.java should NOT be generated when -generate-test-files=false")
	}
}

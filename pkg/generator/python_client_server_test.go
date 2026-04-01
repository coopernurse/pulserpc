package generator

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/coopernurse/pulserpc/pkg/parser"
)

func TestPythonGeneratorBasicFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulserpc-python-gen-")
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

	p := NewPythonClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", "", "output dir")
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}

	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Check server.py
	serverPath := filepath.Join(tmpDir, "server.py")
	if _, err := os.Stat(serverPath); err != nil {
		t.Fatalf("expected server.py at %s, missing: %v", serverPath, err)
	}

	// Check client.py
	clientPath := filepath.Join(tmpDir, "client.py")
	if _, err := os.Stat(clientPath); err != nil {
		t.Fatalf("expected client.py at %s, missing: %v", clientPath, err)
	}

	// Check idl.json
	idlPath := filepath.Join(tmpDir, "idl.json")
	if _, err := os.Stat(idlPath); err != nil {
		t.Fatalf("expected idl.json at %s, missing: %v", idlPath, err)
	}

	// Check pulserpc runtime directory
	runtimePath := filepath.Join(tmpDir, "pulserpc")
	if _, err := os.Stat(runtimePath); err != nil {
		t.Fatalf("expected pulserpc runtime dir at %s, missing: %v", runtimePath, err)
	}
}

func TestPythonGeneratorTestFilesWithFlag(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulserpc-python-gen-")
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

	p := NewPythonClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", "", "output dir")
	fs.Bool("generate-test-files", false, "generate test files")
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}
	if err := fs.Set("generate-test-files", "true"); err != nil {
		t.Fatalf("failed to set generate-test-files flag: %v", err)
	}

	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Check that test files are generated when flag is true
	testServerPath := filepath.Join(tmpDir, "test_server.py")
	if _, err := os.Stat(testServerPath); err != nil {
		t.Fatalf("expected test_server.py at %s, missing: %v", testServerPath, err)
	}
	testClientPath := filepath.Join(tmpDir, "test_client.py")
	if _, err := os.Stat(testClientPath); err != nil {
		t.Fatalf("expected test_client.py at %s, missing: %v", testClientPath, err)
	}
}

func TestPythonGeneratorTestFilesDisabled(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulserpc-python-gen-")
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

	p := NewPythonClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", "", "output dir")
	fs.Bool("generate-test-files", false, "generate test files")
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}
	// Explicitly disable test file generation
	if err := fs.Set("generate-test-files", "false"); err != nil {
		t.Fatalf("failed to set generate-test-files flag: %v", err)
	}

	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Check that test files are NOT generated when generate-test-files is false
	testServerPath := filepath.Join(tmpDir, "test_server.py")
	if _, err := os.Stat(testServerPath); err == nil {
		t.Fatalf("test_server.py should NOT be generated when -generate-test-files=false")
	}
	testClientPath := filepath.Join(tmpDir, "test_client.py")
	if _, err := os.Stat(testClientPath); err == nil {
		t.Fatalf("test_client.py should NOT be generated when -generate-test-files=false")
	}
}

func TestPythonGeneratorPackageFlagParsing(t *testing.T) {
	tests := []struct {
		name          string
		packageValue  string
		expectPackage string
	}{
		{
			name:          "empty package",
			packageValue:  "",
			expectPackage: "",
		},
		{
			name:          "simple package",
			packageValue:  "myapp",
			expectPackage: "myapp",
		},
		{
			name:          "nested package",
			packageValue:  "myapp.lib.rpc",
			expectPackage: "myapp.lib.rpc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPythonClientServer()
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			fs.String("dir", "", "output dir")
			p.RegisterFlags(fs)

			tmpDir, err := os.MkdirTemp("", "pulserpc-python-pkg-")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer func() { _ = os.RemoveAll(tmpDir) }()

			if err := fs.Set("dir", tmpDir); err != nil {
				t.Fatalf("failed to set dir flag: %v", err)
			}
			if tt.packageValue != "" {
				if err := fs.Set("package", tt.packageValue); err != nil {
					t.Fatalf("failed to set package flag: %v", err)
				}
			}

			idl := &parser.IDL{
				Interfaces: []*parser.Interface{
					{
						Name:      "A",
						Namespace: "",
					},
				},
			}

			if err := p.Generate(idl, fs); err != nil {
				t.Fatalf("Generate failed: %v", err)
			}

			if p.packageBase != tt.expectPackage {
				t.Errorf("expected packageBase=%q, got %q", tt.expectPackage, p.packageBase)
			}
		})
	}
}

func TestPythonGeneratorPackageFlagDoesNotConflict(t *testing.T) {
	// Verify that registering Python plugin flags doesn't break when
	// another plugin (like TS) has already registered the -package flag
	fs := flag.NewFlagSet("test", flag.ContinueOnError)

	// Simulate TS plugin registering first
	if fs.Lookup("package") == nil {
		fs.String("package", "", "Package prefix for generated types and classes")
	}

	// Now register Python plugin flags - should not panic or error
	p := NewPythonClientServer()
	p.RegisterFlags(fs)

	// Verify the flag exists and can be set
	if err := fs.Set("package", "myapp.lib.rpc"); err != nil {
		t.Fatalf("failed to set package flag: %v", err)
	}

	// Verify the value was set
	pkgFlag := fs.Lookup("package")
	if pkgFlag == nil {
		t.Fatal("package flag should exist")
	}
	if pkgFlag.Value.String() != "myapp.lib.rpc" {
		t.Errorf("expected package flag value %q, got %q", "myapp.lib.rpc", pkgFlag.Value.String())
	}
}

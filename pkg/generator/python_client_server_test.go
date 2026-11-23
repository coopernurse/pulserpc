package generator

import (
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coopernurse/pulserpc/pkg/parser"
)

func TestPythonGeneratorBasicFiles(t *testing.T) {
	tmpDir := newPythonTestTempDir(t, "pulserpc-python-gen-")

	// Build a minimal IDL with single namespace (empty) - all types in same namespace
	idl := &parser.IDL{
		Structs: []*parser.Struct{
			{
				Name:      "Req",
				Namespace: "",
				Fields:    []*parser.Field{{Name: "msg", Type: &parser.Type{BuiltIn: "string"}}},
			},
		},
		Enums: []*parser.Enum{
			{
				Name:      "Status",
				Namespace: "",
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

	// Check server.py in default namespace (root dir for single-namespace)
	assertFilesExist(t, tmpDir, "server.py", "client.py", "rpctypes.py", "idl.json")
	assertDirExists(t, filepath.Join(tmpDir, "pulserpc"))
}

func TestPythonGeneratorTestFilesWithFlag(t *testing.T) {
	tmpDir := newPythonTestTempDir(t, "pulserpc-python-gen-")

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

	// Check that test files are generated in the default namespace (root dir)
	assertFilesExist(t, tmpDir, "test_server.py", "test_client.py")
}

func TestPythonGeneratorTestFilesDisabled(t *testing.T) {
	tmpDir := newPythonTestTempDir(t, "pulserpc-python-gen-")

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
	if _, err := os.Stat(filepath.Join(tmpDir, "test_server.py")); err == nil {
		t.Fatalf("test_server.py should NOT be generated when -generate-test-files=false")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "test_client.py")); err == nil {
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

			tmpDir := newPythonTestTempDir(t, "pulserpc-python-pkg-")

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

func TestPythonGeneratorInitPySingleNamespace(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulserpc-python-initpy-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	idl := &parser.IDL{
		Structs: []*parser.Struct{
			{Name: "Foo", Namespace: "", Fields: []*parser.Field{{Name: "msg", Type: &parser.Type{BuiltIn: "string"}}}},
		},
		Interfaces: []*parser.Interface{
			{Name: "A", Namespace: ""},
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

	assertFileExists(t, filepath.Join(tmpDir, "__init__.py"))
	assertFileContains(t, filepath.Join(tmpDir, "__init__.py"), "Generated by pulserpc")
	assertFileContains(t, filepath.Join(tmpDir, "__init__.py"), "Python package")
}

func TestPythonGeneratorInitPyTwoNamespaces(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulserpc-python-initpy2-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	idl := &parser.IDL{
		Structs: []*parser.Struct{
			{Name: "common.Response", Namespace: "common", Fields: []*parser.Field{{Name: "status", Type: &parser.Type{BuiltIn: "string"}}}},
			{Name: "book.Book", Namespace: "book", Fields: []*parser.Field{{Name: "title", Type: &parser.Type{BuiltIn: "string"}}}},
		},
		Interfaces: []*parser.Interface{
			{Name: "book.BookService", Namespace: "book"},
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

	for _, path := range []string{filepath.Join(tmpDir, "common", "__init__.py"), filepath.Join(tmpDir, "book", "__init__.py")} {
		assertFileExists(t, path)
		assertFileContains(t, path, "Generated by pulserpc")
	}
}

func TestPythonGeneratorInitPyIdempotent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulserpc-python-idemp-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	idl := &parser.IDL{
		Structs: []*parser.Struct{
			{Name: "Foo", Namespace: "", Fields: []*parser.Field{{Name: "msg", Type: &parser.Type{BuiltIn: "string"}}}},
		},
		Interfaces: []*parser.Interface{
			{Name: "A", Namespace: ""},
		},
	}

	p := NewPythonClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", "", "output dir")
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}

	// Generate twice
	for i := 0; i < 2; i++ {
		if err := p.Generate(idl, fs); err != nil {
			t.Fatalf("Generate iteration %d failed: %v", i, err)
		}
	}

	initPath := filepath.Join(tmpDir, "__init__.py")
	assertFileExists(t, initPath)
	content, err := os.ReadFile(initPath)
	if err != nil {
		t.Fatalf("failed to read __init__.py: %v", err)
	}
	if strings.Count(string(content), "Generated by pulserpc") != 1 {
		t.Errorf("__init__.py should have exactly one generator marker, got %d", strings.Count(string(content), "Generated by pulserpc"))
	}

	// Verify other generated files are still intact
	for _, filename := range []string{"rpctypes.py", "server.py", "client.py", "idl.json"} {
		path := filepath.Join(tmpDir, filename)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s at %s after regeneration, missing: %v", filename, path, err)
		}
	}
}

func TestPythonGeneratorInitPyWithPackageBase(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulserpc-python-pkginit-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	idl := &parser.IDL{
		Structs: []*parser.Struct{
			{Name: "Foo", Namespace: "", Fields: []*parser.Field{{Name: "msg", Type: &parser.Type{BuiltIn: "string"}}}},
		},
		Interfaces: []*parser.Interface{
			{Name: "A", Namespace: ""},
		},
	}

	p := NewPythonClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", "", "output dir")
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}
	if err := fs.Set("package", "myapp.lib.rpc"); err != nil {
		t.Fatalf("failed to set package flag: %v", err)
	}

	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	initPath := filepath.Join(tmpDir, "myapp", "lib", "rpc", "__init__.py")
	assertFileExists(t, initPath)
	assertFileContains(t, initPath, "myapp.lib.rpc.pulserpc")
}

func TestPythonGeneratorNamespaceSplitFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulserpc-ns-split-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Multi-namespace IDL: common has types, book has interface
	idl := &parser.IDL{
		Structs: []*parser.Struct{
			{Name: "common.Response", Namespace: "common", Fields: []*parser.Field{{Name: "status", Type: &parser.Type{BuiltIn: "string"}}}},
			{Name: "book.Book", Namespace: "book", Fields: []*parser.Field{{Name: "title", Type: &parser.Type{BuiltIn: "string"}}}},
		},
		Interfaces: []*parser.Interface{
			{Name: "book.BookService", Namespace: "book", Methods: []*parser.Method{
				{Name: "getBook", Parameters: []*parser.Parameter{{Name: "id", Type: &parser.Type{BuiltIn: "string"}}}, ReturnType: &parser.Type{UserDefined: "book.Book"}},
			}},
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

	for _, ns := range []string{"common", "book"} {
		assertNamespaceFilesExist(t, tmpDir, ns, "rpctypes.py", "server.py", "client.py", "test_server.py", "test_client.py", "__init__.py")
	}

	// Verify book/server.py contains the BookService interface
	bookServerPath := filepath.Join(tmpDir, "book", "server.py")
	content, err := os.ReadFile(bookServerPath)
	if err != nil {
		t.Fatalf("failed to read book/server.py: %v", err)
	}
	if !strings.Contains(string(content), "BookService") {
		t.Errorf("book/server.py should contain BookService class, got:\n%s", string(content))
	}

	// Verify common/server.py does NOT contain BookService (only types, no interfaces)
	commonServerPath := filepath.Join(tmpDir, "common", "server.py")
	content, err = os.ReadFile(commonServerPath)
	if err != nil {
		t.Fatalf("failed to read common/server.py: %v", err)
	}
	if strings.Contains(string(content), "BookService") {
		t.Errorf("common/server.py should NOT contain BookService class")
	}
}

func TestPythonGeneratorSingleNamespaceEquivalence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulserpc-single-ns-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Single namespace IDL - all types in empty namespace
	idl := &parser.IDL{
		Structs: []*parser.Struct{
			{Name: "User", Namespace: "", Fields: []*parser.Field{
				{Name: "id", Type: &parser.Type{BuiltIn: "string"}},
				{Name: "name", Type: &parser.Type{BuiltIn: "string"}},
			}},
		},
		Interfaces: []*parser.Interface{
			{Name: "UserService", Namespace: "", Methods: []*parser.Method{
				{Name: "getUser", Parameters: []*parser.Parameter{{Name: "id", Type: &parser.Type{BuiltIn: "string"}}}, ReturnType: &parser.Type{UserDefined: "User"}},
			}},
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

	// For single-namespace, files should be at root level (backwards compatible)
	for _, filename := range []string{"rpctypes.py", "server.py", "client.py", "test_server.py", "test_client.py"} {
		path := filepath.Join(tmpDir, filename)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %s at root %s, missing: %v", filename, path, err)
		}
	}

	// Verify server.py contains the UserService interface
	serverPath := filepath.Join(tmpDir, "server.py")
	content, err := os.ReadFile(serverPath)
	if err != nil {
		t.Fatalf("failed to read server.py: %v", err)
	}
	if !strings.Contains(string(content), "class UserService") {
		t.Errorf("server.py should contain UserService class")
	}

	// Verify no extra namespace directories were created
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read output dir: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != "pulserpc" {
			t.Errorf("unexpected directory %s in single-namespace output", entry.Name())
		}
	}
}

func TestPythonGeneratorTypesPyContent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulserpc-typespy-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	idl := &parser.IDL{
		Structs: []*parser.Struct{
			{
				Name:      "common.Response",
				Namespace: "common",
				Fields: []*parser.Field{
					{Name: "status", Type: &parser.Type{BuiltIn: "string"}},
					{Name: "code", Type: &parser.Type{BuiltIn: "int"}},
				},
			},
			{
				Name:      "book.Book",
				Namespace: "book",
				Fields: []*parser.Field{
					{Name: "title", Type: &parser.Type{BuiltIn: "string"}},
					{Name: "author", Type: &parser.Type{BuiltIn: "string"}, Optional: true},
				},
			},
		},
		Enums: []*parser.Enum{
			{
				Name:      "common.Status",
				Namespace: "common",
				Values:    []*parser.EnumValue{{Name: "active"}, {Name: "inactive"}},
			},
		},
		Interfaces: []*parser.Interface{
			{Name: "book.BookService", Namespace: "book", Methods: []*parser.Method{
				{Name: "getBook", Parameters: []*parser.Parameter{{Name: "id", Type: &parser.Type{BuiltIn: "string"}}}, ReturnType: &parser.Type{UserDefined: "book.Book"}},
			}},
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

	// Verify rpctypes.py in common namespace contains struct and enum
	commonTypesPath := filepath.Join(tmpDir, "common", "rpctypes.py")
	content, err := os.ReadFile(commonTypesPath)
	if err != nil {
		t.Fatalf("failed to read common/rpctypes.py: %v", err)
	}
	commonTypesContent := string(content)
	if !strings.Contains(commonTypesContent, "class Response:") {
		t.Errorf("common/rpctypes.py should contain Response dataclass")
	}
	if !strings.Contains(commonTypesContent, "class Status:") {
		t.Errorf("common/rpctypes.py should contain Status enum")
	}
	if !strings.Contains(commonTypesContent, "@dataclass") {
		t.Errorf("common/rpctypes.py should use @dataclass decorator")
	}

	// Verify rpctypes.py in book namespace contains struct
	bookTypesPath := filepath.Join(tmpDir, "book", "rpctypes.py")
	content, err = os.ReadFile(bookTypesPath)
	if err != nil {
		t.Fatalf("failed to read book/rpctypes.py: %v", err)
	}
	bookTypesContent := string(content)
	if !strings.Contains(bookTypesContent, "class Book:") {
		t.Errorf("book/rpctypes.py should contain Book dataclass")
	}

	// Verify server.py imports types from local rpctypes.py
	bookServerPath := filepath.Join(tmpDir, "book", "server.py")
	content, err = os.ReadFile(bookServerPath)
	if err != nil {
		t.Fatalf("failed to read book/server.py: %v", err)
	}
	if !strings.Contains(string(content), "from .rpctypes import") {
		t.Errorf("book/server.py should import from .rpctypes")
	}
}

func TestPythonGeneratorFilePlacementStructure(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulserpc-filestruct-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Multi-namespace IDL matching the spec's common/book/user example
	idl := &parser.IDL{
		Structs: []*parser.Struct{
			{Name: "common.Response", Namespace: "common", Fields: []*parser.Field{
				{Name: "status", Type: &parser.Type{BuiltIn: "string"}},
			}},
			{Name: "book.Book", Namespace: "book", Fields: []*parser.Field{
				{Name: "title", Type: &parser.Type{BuiltIn: "string"}},
				{Name: "address", Type: &parser.Type{UserDefined: "common.Response"}},
			}},
			{Name: "user.User", Namespace: "user", Fields: []*parser.Field{
				{Name: "name", Type: &parser.Type{BuiltIn: "string"}},
				{Name: "address", Type: &parser.Type{UserDefined: "common.Response"}},
			}},
		},
		Interfaces: []*parser.Interface{
			{Name: "book.BookService", Namespace: "book"},
			{Name: "user.UserService", Namespace: "user"},
		},
	}

	p := NewPythonClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", "", "output dir")
	fs.String("package", "", "base package")
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}
	if err := fs.Set("package", "myapp.lib.rpc"); err != nil {
		t.Fatalf("failed to set package flag: %v", err)
	}

	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Assert output tree includes myapp/lib/rpc/pulserpc/, myapp/lib/rpc/common/, etc.
	expectedDirs := []string{
		filepath.Join("myapp", "lib", "rpc", "pulserpc"),
		filepath.Join("myapp", "lib", "rpc", "common"),
		filepath.Join("myapp", "lib", "rpc", "book"),
		filepath.Join("myapp", "lib", "rpc", "user"),
	}
	for _, dir := range expectedDirs {
		dirPath := filepath.Join(tmpDir, dir)
		info, err := os.Stat(dirPath)
		if err != nil {
			t.Errorf("expected directory %s at %s, missing: %v", dir, dirPath, err)
		} else if !info.IsDir() {
			t.Errorf("expected %s to be a directory", dirPath)
		}
	}

	// Each namespace dir should have rpctypes.py, server.py, client.py, __init__.py
	for _, ns := range []string{"common", "book", "user"} {
		for _, filename := range []string{"rpctypes.py", "server.py", "client.py", "__init__.py"} {
			path := filepath.Join(tmpDir, "myapp", "lib", "rpc", ns, filename)
			if _, err := os.Stat(path); err != nil {
				t.Errorf("expected %s in %s namespace at %s, missing: %v", filename, ns, path, err)
			}
		}
	}

	bookTypesPath := filepath.Join(tmpDir, "myapp", "lib", "rpc", "book", "rpctypes.py")
	bookTypesContent, err := os.ReadFile(bookTypesPath)
	if err != nil {
		t.Fatalf("failed to read book/rpctypes.py: %v", err)
	}
	if !strings.Contains(string(bookTypesContent), "from myapp.lib.rpc.common import Response") {
		t.Errorf("book/rpctypes.py should import Response from myapp.lib.rpc.common, got:\n%s", string(bookTypesContent))
	}

	userTypesPath := filepath.Join(tmpDir, "myapp", "lib", "rpc", "user", "rpctypes.py")
	userTypesContent, err := os.ReadFile(userTypesPath)
	if err != nil {
		t.Fatalf("failed to read user/rpctypes.py: %v", err)
	}
	if !strings.Contains(string(userTypesContent), "from myapp.lib.rpc.common import Response") {
		t.Errorf("user/rpctypes.py should import Response from myapp.lib.rpc.common, got:\n%s", string(userTypesContent))
	}

	// pulserpc runtime dir should have its files
	runtimeFiles := []string{"__init__.py", "rpc.py", "server.py", "client.py", "transport.py", "rpctypes.py"}
	for _, filename := range runtimeFiles {
		path := filepath.Join(tmpDir, "myapp", "lib", "rpc", "pulserpc", filename)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected runtime file %s at %s, missing: %v", filename, path, err)
		}
	}
}

func TestPythonGeneratorCrossNamespaceImports(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulserpc-cross-ns-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Multi-namespace IDL: book and user both reference common.Address
	idl := &parser.IDL{
		Structs: []*parser.Struct{
			{Name: "common.Address", Namespace: "common", Fields: []*parser.Field{
				{Name: "street", Type: &parser.Type{BuiltIn: "string"}},
				{Name: "city", Type: &parser.Type{BuiltIn: "string"}},
			}},
			{Name: "book.Book", Namespace: "book", Fields: []*parser.Field{
				{Name: "title", Type: &parser.Type{BuiltIn: "string"}},
				{Name: "address", Type: &parser.Type{UserDefined: "common.Address"}},
			}},
			{Name: "user.User", Namespace: "user", Fields: []*parser.Field{
				{Name: "name", Type: &parser.Type{BuiltIn: "string"}},
				{Name: "address", Type: &parser.Type{UserDefined: "common.Address"}},
			}},
		},
		Interfaces: []*parser.Interface{
			{Name: "book.BookService", Namespace: "book"},
			{Name: "user.UserService", Namespace: "user"},
		},
	}

	p := NewPythonClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", "", "output dir")
	fs.String("package", "", "base package")
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}
	if err := fs.Set("package", "myapp.lib.rpc"); err != nil {
		t.Fatalf("failed to set package flag: %v", err)
	}

	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify book/rpctypes.py imports from myapp.lib.rpc.common
	bookTypesPath := filepath.Join(tmpDir, "myapp", "lib", "rpc", "book", "rpctypes.py")
	content, err := os.ReadFile(bookTypesPath)
	if err != nil {
		t.Fatalf("failed to read book/rpctypes.py: %v", err)
	}
	bookTypesContent := string(content)
	if !strings.Contains(bookTypesContent, "from myapp.lib.rpc.common import Address") {
		t.Errorf("book/rpctypes.py should contain 'from myapp.lib.rpc.common import Address', got:\n%s", bookTypesContent)
	}

	// Verify user/rpctypes.py imports from myapp.lib.rpc.common
	userTypesPath := filepath.Join(tmpDir, "myapp", "lib", "rpc", "user", "rpctypes.py")
	content, err = os.ReadFile(userTypesPath)
	if err != nil {
		t.Fatalf("failed to read user/rpctypes.py: %v", err)
	}
	userTypesContent := string(content)
	if !strings.Contains(userTypesContent, "from myapp.lib.rpc.common import Address") {
		t.Errorf("user/rpctypes.py should contain 'from myapp.lib.rpc.common import Address', got:\n%s", userTypesContent)
	}

	// Verify common/rpctypes.py does NOT have cross-namespace imports (it has none)
	commonTypesPath := filepath.Join(tmpDir, "myapp", "lib", "rpc", "common", "rpctypes.py")
	content, err = os.ReadFile(commonTypesPath)
	if err != nil {
		t.Fatalf("failed to read common/rpctypes.py: %v", err)
	}
	commonTypesContent := string(content)
	if strings.Contains(commonTypesContent, "Cross-namespace imports") {
		t.Errorf("common/rpctypes.py should NOT have cross-namespace imports section, got:\n%s", commonTypesContent)
	}
}

func TestPythonGeneratorCrossNamespaceImportsNoPackage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulserpc-cross-ns-nopkg-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Multi-namespace IDL: book references common.Address
	idl := &parser.IDL{
		Structs: []*parser.Struct{
			{Name: "common.Address", Namespace: "common", Fields: []*parser.Field{
				{Name: "street", Type: &parser.Type{BuiltIn: "string"}},
			}},
			{Name: "book.Book", Namespace: "book", Fields: []*parser.Field{
				{Name: "title", Type: &parser.Type{BuiltIn: "string"}},
				{Name: "address", Type: &parser.Type{UserDefined: "common.Address"}},
			}},
		},
		Interfaces: []*parser.Interface{
			{Name: "book.BookService", Namespace: "book"},
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

	// Without -package, cross-namespace imports should use bare namespace (e.g., "from common import Address")
	bookTypesPath := filepath.Join(tmpDir, "book", "rpctypes.py")
	content, err := os.ReadFile(bookTypesPath)
	if err != nil {
		t.Fatalf("failed to read book/rpctypes.py: %v", err)
	}
	bookTypesContent := string(content)
	if !strings.Contains(bookTypesContent, "from common import Address") {
		t.Errorf("book/rpctypes.py should contain 'from common import Address' (no package), got:\n%s", bookTypesContent)
	}
}

func TestPythonGeneratorCrossNamespaceImportsNestedTypes(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulserpc-cross-ns-nested-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Test with nested types: arrays and maps of cross-namespace types
	idl := &parser.IDL{
		Structs: []*parser.Struct{
			{Name: "common.Item", Namespace: "common", Fields: []*parser.Field{
				{Name: "name", Type: &parser.Type{BuiltIn: "string"}},
			}},
			{Name: "book.Book", Namespace: "book", Fields: []*parser.Field{
				{Name: "title", Type: &parser.Type{BuiltIn: "string"}},
				{Name: "items", Type: &parser.Type{Array: &parser.Type{UserDefined: "common.Item"}}},
			}},
		},
		Interfaces: []*parser.Interface{
			{Name: "book.BookService", Namespace: "book"},
		},
	}

	p := NewPythonClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", "", "output dir")
	fs.String("package", "", "base package")
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}
	if err := fs.Set("package", "myapp.lib.rpc"); err != nil {
		t.Fatalf("failed to set package flag: %v", err)
	}

	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify book/rpctypes.py imports Item from common even though it's inside an array
	bookTypesPath := filepath.Join(tmpDir, "myapp", "lib", "rpc", "book", "rpctypes.py")
	content, err := os.ReadFile(bookTypesPath)
	if err != nil {
		t.Fatalf("failed to read book/rpctypes.py: %v", err)
	}
	bookTypesContent := string(content)
	if !strings.Contains(bookTypesContent, "from myapp.lib.rpc.common import Item") {
		t.Errorf("book/rpctypes.py should contain 'from myapp.lib.rpc.common import Item' for array element type, got:\n%s", bookTypesContent)
	}
}

func TestPythonGeneratorCrossNamespaceImportsPydantic(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulserpc-cross-ns-pydantic-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	idl := &parser.IDL{
		Structs: []*parser.Struct{
			{Name: "common.Address", Namespace: "common", Fields: []*parser.Field{
				{Name: "street", Type: &parser.Type{BuiltIn: "string"}},
			}},
			{Name: "book.Book", Namespace: "book", Fields: []*parser.Field{
				{Name: "title", Type: &parser.Type{BuiltIn: "string"}},
				{Name: "address", Type: &parser.Type{UserDefined: "common.Address"}},
			}},
		},
		Interfaces: []*parser.Interface{
			{Name: "book.BookService", Namespace: "book"},
		},
	}

	p := NewPythonClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", "", "output dir")
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}
	if err := fs.Set("package", "myapp.lib.rpc"); err != nil {
		t.Fatalf("failed to set package flag: %v", err)
	}
	if err := fs.Set("use-pydantic", "true"); err != nil {
		t.Fatalf("failed to set use-pydantic flag: %v", err)
	}

	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify models.py has cross-namespace imports
	modelsPath := filepath.Join(tmpDir, "models.py")
	content, err := os.ReadFile(modelsPath)
	if err != nil {
		t.Fatalf("failed to read models.py: %v", err)
	}
	modelsContent := string(content)
	if !strings.Contains(modelsContent, "from myapp.lib.rpc.common import Address") {
		t.Errorf("models.py should contain 'from myapp.lib.rpc.common import Address', got:\n%s", modelsContent)
	}
}

func TestCollectCrossNamespaceRefs(t *testing.T) {
	tests := []struct {
		name          string
		nsTypes       *NamespaceTypes
		currentNS     string
		expectRefs    int
		expectTargets []string
	}{
		{
			name: "no cross-namespace refs",
			nsTypes: &NamespaceTypes{
				Structs: []*parser.Struct{
					{Name: "Foo", Namespace: "book", Fields: []*parser.Field{
						{Name: "name", Type: &parser.Type{BuiltIn: "string"}},
					}},
				},
			},
			currentNS:     "book",
			expectRefs:    0,
			expectTargets: nil,
		},
		{
			name: "single cross-namespace ref",
			nsTypes: &NamespaceTypes{
				Structs: []*parser.Struct{
					{Name: "Book", Namespace: "book", Fields: []*parser.Field{
						{Name: "title", Type: &parser.Type{BuiltIn: "string"}},
						{Name: "addr", Type: &parser.Type{UserDefined: "common.Address"}},
					}},
				},
			},
			currentNS:     "book",
			expectRefs:    1,
			expectTargets: []string{"common"},
		},
		{
			name: "duplicate refs deduplicated",
			nsTypes: &NamespaceTypes{
				Structs: []*parser.Struct{
					{Name: "Book", Namespace: "book", Fields: []*parser.Field{
						{Name: "addr1", Type: &parser.Type{UserDefined: "common.Address"}},
						{Name: "addr2", Type: &parser.Type{UserDefined: "common.Address"}},
					}},
				},
			},
			currentNS:     "book",
			expectRefs:    1,
			expectTargets: []string{"common"},
		},
		{
			name: "array of cross-namespace type",
			nsTypes: &NamespaceTypes{
				Structs: []*parser.Struct{
					{Name: "Book", Namespace: "book", Fields: []*parser.Field{
						{Name: "addrs", Type: &parser.Type{Array: &parser.Type{UserDefined: "common.Address"}}},
					}},
				},
			},
			currentNS:     "book",
			expectRefs:    1,
			expectTargets: []string{"common"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs := collectCrossNamespaceRefs(tt.nsTypes, tt.currentNS)
			if len(refs) != tt.expectRefs {
				t.Errorf("expected %d refs, got %d", tt.expectRefs, len(refs))
			}
			if tt.expectTargets != nil {
				found := make(map[string]bool)
				for _, ref := range refs {
					found[ref.TargetNS] = true
				}
				for _, target := range tt.expectTargets {
					if !found[target] {
						t.Errorf("expected target namespace %q not found", target)
					}
				}
			}
		})
	}
}

func TestBuildCrossNamespaceImports(t *testing.T) {
	tests := []struct {
		name        string
		refs        []crossNamespaceRef
		packageBase string
		expect      string
	}{
		{
			name:        "empty refs",
			refs:        nil,
			packageBase: "",
			expect:      "",
		},
		{
			name: "with package base",
			refs: []crossNamespaceRef{
				{TargetNS: "common", BaseName: "Address"},
			},
			packageBase: "myapp.lib.rpc",
			expect:      "from myapp.lib.rpc.common import Address\n\n",
		},
		{
			name: "without package base",
			refs: []crossNamespaceRef{
				{TargetNS: "common", BaseName: "Address"},
			},
			packageBase: "",
			expect:      "from common import Address\n\n",
		},
		{
			name: "multiple types from same namespace",
			refs: []crossNamespaceRef{
				{TargetNS: "common", BaseName: "Address"},
				{TargetNS: "common", BaseName: "Status"},
			},
			packageBase: "myapp.lib.rpc",
			expect:      "from myapp.lib.rpc.common import Address, Status\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildCrossNamespaceImports(tt.refs, tt.packageBase)
			if result != tt.expect {
				t.Errorf("expected %q, got %q", tt.expect, result)
			}
		})
	}
}

func TestGenerateTestServerPyForNamespaceRuntimeImport(t *testing.T) {
	baseNs := &NamespaceTypes{Interfaces: []*parser.Interface{{Name: "A"}}}
	configured := generateTestServerPyForNamespace(baseNs, nil, nil, "", "myapp.lib.rpc")
	if !strings.Contains(configured, "from myapp.lib.rpc.pulserpc import Server, Contract, RPCError") {
		t.Fatalf("expected configured runtime import, got:\n%s", configured)
	}

	backwardsCompatible := generateTestServerPyForNamespace(baseNs, nil, nil, "", "")
	if !strings.Contains(backwardsCompatible, "from pulserpc import Server, Contract, RPCError") {
		t.Fatalf("expected default runtime import, got:\n%s", backwardsCompatible)
	}
}

func TestGenerateTestServerPyCapsRequestBody(t *testing.T) {
	baseNs := &NamespaceTypes{Interfaces: []*parser.Interface{{Name: "A"}}}
	code := generateTestServerPyForNamespace(baseNs, nil, nil, "", "")

	for _, want := range []string{
		"MAX_BODY_SIZE = 1024 * 1024",
		"if content_length > self.MAX_BODY_SIZE:",
		"\"Invalid Request\", \"Request body too large\"",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated test_server.py should contain %q, got:\n%s", want, code)
		}
	}
}

func TestGenerateTestClientPyForNamespaceRuntimeImport(t *testing.T) {
	baseNs := &NamespaceTypes{Interfaces: []*parser.Interface{{Name: "A"}}}
	configured := generateTestClientPyForNamespace(baseNs, nil, nil, "", "myapp.lib.rpc")
	if !strings.Contains(configured, "from myapp.lib.rpc.pulserpc import HttpTransport, Client") {
		t.Fatalf("expected configured runtime import, got:\n%s", configured)
	}

	backwardsCompatible := generateTestClientPyForNamespace(baseNs, nil, nil, "", "")
	if !strings.Contains(backwardsCompatible, "from pulserpc import HttpTransport, Client") {
		t.Fatalf("expected default runtime import, got:\n%s", backwardsCompatible)
	}
}

func TestPython2Generator_GeneratesIDLOnly(t *testing.T) {
	tmpDir := newPythonTestTempDir(t, "pulserpc-python2-gen-")

	idl := &parser.IDL{
		RootNamespace: "testns",
		Interfaces: []*parser.Interface{
			{
				Name:      "A",
				Namespace: "testns",
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
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}
	if err := fs.Set("python-version", "2"); err != nil {
		t.Fatalf("failed to set python-version flag: %v", err)
	}

	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	assertFileExists(t, filepath.Join(tmpDir, "testns", "idl.json"))
	assertDirExists(t, filepath.Join(tmpDir, "pulserpc"))
}

func TestPython2Generator_CopiesPython2Runtime(t *testing.T) {
	tmpDir := newPythonTestTempDir(t, "pulserpc-python2-runtime-")

	idl := &parser.IDL{
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
	if err := fs.Set("python-version", "2"); err != nil {
		t.Fatalf("failed to set python-version flag: %v", err)
	}

	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	pulserpcDir := filepath.Join(tmpDir, "pulserpc")
	assertDirExists(t, pulserpcDir)

	expectedFiles := []string{"__init__.py", "rpc.py", "rpctypes.py", "validation.py", "contract.py", "server.py", "transport.py", "client.py"}
	assertFilesExist(t, pulserpcDir, expectedFiles...)
}

func TestPython2Generator_NoCodegenFiles(t *testing.T) {
	tmpDir := newPythonTestTempDir(t, "pulserpc-python2-nocode-")

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
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}
	if err := fs.Set("python-version", "2"); err != nil {
		t.Fatalf("failed to set python-version flag: %v", err)
	}

	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "rpctypes.py")); err == nil {
		t.Fatalf("rpctypes.py should NOT be generated for Python 2")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "server.py")); err == nil {
		t.Fatalf("server.py should NOT be generated for Python 2")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "client.py")); err == nil {
		t.Fatalf("client.py should NOT be generated for Python 2")
	}
}

func TestPython2Generator_DefaultIsPython3(t *testing.T) {
	tmpDir := newPythonTestTempDir(t, "pulserpc-python3-default-")

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
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}

	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	assertFilesExist(t, tmpDir, "server.py", "client.py", "rpctypes.py", "idl.json")
}

func TestPython2Generator_PythonVersionFlag(t *testing.T) {
	tmpDirPy2 := newPythonTestTempDir(t, "pulserpc-py2-flag-")
	tmpDirPy3 := newPythonTestTempDir(t, "pulserpc-py3-flag-")

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

	p2 := NewPythonClientServer()
	fs2 := flag.NewFlagSet("test", flag.ContinueOnError)
	fs2.String("dir", "", "output dir")
	p2.RegisterFlags(fs2)
	if err := fs2.Set("dir", tmpDirPy2); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}
	if err := fs2.Set("python-version", "2"); err != nil {
		t.Fatalf("failed to set python-version flag: %v", err)
	}
	if err := p2.Generate(idl, fs2); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	p3 := NewPythonClientServer()
	fs3 := flag.NewFlagSet("test", flag.ContinueOnError)
	fs3.String("dir", "", "output dir")
	p3.RegisterFlags(fs3)
	if err := fs3.Set("dir", tmpDirPy3); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}
	if err := fs3.Set("python-version", "3"); err != nil {
		t.Fatalf("failed to set python-version flag: %v", err)
	}
	if err := p3.Generate(idl, fs3); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpDirPy2, "rpctypes.py")); err == nil {
		t.Fatalf("Python 2 should NOT generate rpctypes.py")
	}
	if _, err := os.Stat(filepath.Join(tmpDirPy3, "rpctypes.py")); err != nil {
		t.Fatalf("Python 3 should generate rpctypes.py")
	}
}

func TestPython2Generator_PackageFlag(t *testing.T) {
	tmpDir := newPythonTestTempDir(t, "pulserpc-py2-pkg-")

	idl := &parser.IDL{
		RootNamespace: "checkout",
		Interfaces: []*parser.Interface{
			{
				Name:      "A",
				Namespace: "checkout",
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
	fs.String("package", "", "base package")
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}
	if err := fs.Set("python-version", "2"); err != nil {
		t.Fatalf("failed to set python-version flag: %v", err)
	}
	if err := fs.Set("package", "myapp.lib.rpc"); err != nil {
		t.Fatalf("failed to set package flag: %v", err)
	}

	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	assertDirExists(t, filepath.Join(tmpDir, "myapp", "lib", "rpc", "pulserpc"))
	assertFileExists(t, filepath.Join(tmpDir, "myapp", "lib", "rpc", "checkout", "idl.json"))
}

func TestPythonGeneratorCtxInServerPy(t *testing.T) {
	tmpDir := newPythonTestTempDir(t, "pulserpc-ctx-server-")

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
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}

	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	serverPath := filepath.Join(tmpDir, "server.py")
	content, err := os.ReadFile(serverPath)
	if err != nil {
		t.Fatalf("failed to read server.py: %v", err)
	}
	serverContent := string(content)

	// Verify abstract method stub has ctx first
	if !strings.Contains(serverContent, "def add(self, ctx, a, b):") {
		t.Errorf("server.py should contain 'def add(self, ctx, a, b):', got:\n%s", serverContent)
	}
}

func TestPythonGeneratorCtxInTestServerPy(t *testing.T) {
	tmpDir := newPythonTestTempDir(t, "pulserpc-ctx-testserver-")

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

	testServerPath := filepath.Join(tmpDir, "test_server.py")
	content, err := os.ReadFile(testServerPath)
	if err != nil {
		t.Fatalf("failed to read test_server.py: %v", err)
	}
	testServerContent := string(content)

	// Verify test implementation has ctx first
	if !strings.Contains(testServerContent, "def add(self, ctx, a, b):") {
		t.Errorf("test_server.py should contain 'def add(self, ctx, a, b):', got:\n%s", testServerContent)
	}
}

func TestPythonGeneratorCtxPropagatesToOutput(t *testing.T) {
	tmpDir := newPythonTestTempDir(t, "pulserpc-ctx-propagate-")

	// Create a more complex IDL with multiple methods
	idl := &parser.IDL{
		Structs: []*parser.Struct{
			{
				Name:      "User",
				Namespace: "",
				Fields:    []*parser.Field{{Name: "id", Type: &parser.Type{BuiltIn: "string"}}},
			},
		},
		Interfaces: []*parser.Interface{
			{
				Name:      "UserService",
				Namespace: "",
				Methods: []*parser.Method{
					{
						Name:       "getUser",
						Parameters: []*parser.Parameter{{Name: "id", Type: &parser.Type{BuiltIn: "string"}}},
						ReturnType: &parser.Type{UserDefined: "User"},
					},
					{
						Name:       "createUser",
						Parameters: []*parser.Parameter{{Name: "user", Type: &parser.Type{UserDefined: "User"}}},
						ReturnType: &parser.Type{BuiltIn: "bool"},
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

	// Check server.py (interface stubs)
	serverPath := filepath.Join(tmpDir, "server.py")
	serverContent, err := os.ReadFile(serverPath)
	if err != nil {
		t.Fatalf("failed to read server.py: %v", err)
	}
	serverStr := string(serverContent)

	// Verify all methods have ctx first
	if !strings.Contains(serverStr, "def getUser(self, ctx, id):") {
		t.Errorf("server.py should contain getUser with ctx first")
	}
	if !strings.Contains(serverStr, "def createUser(self, ctx, user):") {
		t.Errorf("server.py should contain createUser with ctx first")
	}

	// Check test_server.py (test implementations)
	testServerPath := filepath.Join(tmpDir, "test_server.py")
	testServerContent, err := os.ReadFile(testServerPath)
	if err != nil {
		t.Fatalf("failed to read test_server.py: %v", err)
	}
	testServerStr := string(testServerContent)

	if !strings.Contains(testServerStr, "def getUser(self, ctx, id):") {
		t.Errorf("test_server.py should contain getUser with ctx first")
	}
	if !strings.Contains(testServerStr, "def createUser(self, ctx, user):") {
		t.Errorf("test_server.py should contain createUser with ctx first")
	}
}

// --- C1/D12: byte-deterministic output ---------------------------------------

func TestBuildCrossNamespaceImportsIsSorted(t *testing.T) {
	refs := []crossNamespaceRef{
		{TargetNS: "zeta", BaseName: "Zed"},
		{TargetNS: "alpha", BaseName: "Beta"},
		{TargetNS: "zeta", BaseName: "Alpha"},
		{TargetNS: "alpha", BaseName: "Alpha"},
	}
	got := buildCrossNamespaceImports(refs, "")
	want := "from alpha import Alpha, Beta\nfrom zeta import Alpha, Zed\n\n"
	if got != want {
		t.Fatalf("buildCrossNamespaceImports must be sorted:\nwant %q\ngot  %q", want, got)
	}
}

func TestGenerateModelsPyIsDeterministic(t *testing.T) {
	namespaceMap := map[string]*NamespaceTypes{
		"zeta": {Structs: []*parser.Struct{{
			Name:   "zeta.Zed",
			Fields: []*parser.Field{{Name: "id", Type: &parser.Type{BuiltIn: "string"}}},
		}}},
		"alpha": {Structs: []*parser.Struct{{
			Name:   "alpha.Able",
			Fields: []*parser.Field{{Name: "id", Type: &parser.Type{BuiltIn: "string"}}},
		}}},
	}

	first := generateModelsPy(&parser.IDL{}, namespaceMap, "")
	for i := 0; i < 20; i++ {
		if again := generateModelsPy(&parser.IDL{}, namespaceMap, ""); again != first {
			t.Fatalf("generateModelsPy output is not deterministic:\nfirst:\n%s\nagain:\n%s", first, again)
		}
	}

	alpha := strings.Index(first, "# alpha")
	zeta := strings.Index(first, "# zeta")
	if alpha == -1 || zeta == -1 {
		t.Fatalf("expected both namespaces in output, got:\n%s", first)
	}
	if alpha > zeta {
		t.Fatalf("namespace model blocks must be sorted, got:\n%s", first)
	}
}

// --- R22/D3: non-POST requests return HTTP 405 with a JSON-RPC body ----------

func TestGenerateTestServerPyNonPostReturns405(t *testing.T) {
	baseNs := &NamespaceTypes{Interfaces: []*parser.Interface{{Name: "A"}}}
	code := generateTestServerPyForNamespace(baseNs, nil, nil, "", "")

	for _, method := range []string{"do_GET", "do_PUT", "do_DELETE", "do_PATCH"} {
		if !strings.Contains(code, "def "+method+"(self):") {
			t.Errorf("generated test_server.py should define %s, got:\n%s", method, code)
		}
	}
	if !strings.Contains(code, "def do_HEAD(self):") {
		t.Errorf("generated test_server.py should define do_HEAD, got:\n%s", code)
	}
	for _, want := range []string{
		"_send_json_response(405, payload",
		"Invalid Request - only POST allowed",
		"'id': None",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated test_server.py should contain %q, got:\n%s", want, code)
		}
	}
}

// --- M26: malformed JSON returns -32700 --------------------------------------

func TestGenerateTestServerPyMalformedJSONReturnsParseError(t *testing.T) {
	baseNs := &NamespaceTypes{Interfaces: []*parser.Interface{{Name: "A"}}}
	code := generateTestServerPyForNamespace(baseNs, nil, nil, "", "")

	for _, want := range []string{
		"-32700",
		"\"Parse error\"",
		"Invalid JSON",
		// json.loads raises plain ValueError (not JSONDecodeError) for huge
		// integers, and RecursionError for deeply nested input; the parse
		// handler must catch both or the connection is dropped (M26/S3).
		"except (ValueError, RecursionError):",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated test_server.py should contain %q, got:\n%s", want, code)
		}
	}
	if strings.Contains(code, "except json.JSONDecodeError") {
		t.Errorf("generated test_server.py must not catch only JSONDecodeError")
	}
}

// runGeneratedParseErrorDriver generates a Python 3 test server for a single
// namespace and exercises its do_POST with bodies whose decoding raises
// ValueError rather than json.JSONDecodeError (invalid UTF-8 and, on Python
// >=3.11, a huge integer).  Before the M26 fix these escaped do_POST and dropped
// the connection instead of returning -32700.
func TestGeneratedTestServerReturnsParseErrorForValueErrors(t *testing.T) {
	pythonExe, err := exec.LookPath("python")
	if err != nil {
		pythonExe, err = exec.LookPath("python3")
	}
	if err != nil {
		t.Skip("no python interpreter on PATH; generated-server execution is covered by tests/integration")
	}

	tmpDir := newPythonTestTempDir(t, "pulserpc-parseerror-")
	idl := &parser.IDL{
		RootNamespace: "demo",
		Interfaces: []*parser.Interface{{
			Name:      "A",
			Namespace: "demo",
			Methods: []*parser.Method{{
				Name:       "m",
				ReturnType: &parser.Type{BuiltIn: "string"},
			}},
		}},
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

	driver := `import io
import json
import sys

from demo.test_server import TestRPCHandler


class _WFile(object):
    def __init__(self):
        self.data = b""

    def write(self, chunk):
        self.data += chunk
        return len(chunk)


def dispatch(body):
    handler = TestRPCHandler.__new__(TestRPCHandler)
    handler.rfile = io.BytesIO(body)
    handler.wfile = _WFile()
    handler.headers = {"Content-Length": str(len(body))}
    handler._headers_buffer = []
    handler.request_version = "HTTP/1.0"
    handler.requestline = "POST / HTTP/1.0"
    handler.command = "POST"
    handler.client_address = ("127.0.0.1", 0)
    handler.server = None
    handler.close_connection = True
    handler.do_POST()
    head, _, payload = handler.wfile.data.partition(b"\r\n\r\n")
    status = int(head.split(b" ")[1])
    return status, json.loads(payload.decode("utf-8"))


def check(label, body):
    status, response = dispatch(body)
    assert status == 200, (label, status, response)
    assert response["error"]["code"] == -32700, (label, response)
    assert response["id"] is None, (label, response)


# Invalid UTF-8 raises UnicodeDecodeError, a ValueError but not JSONDecodeError.
check("invalid utf-8", b'{"jsonrpc": "2.0", "id": 1, "v": "\xff"}')

# A huge integer raises ValueError from json.loads on Python >= 3.11.
if sys.version_info >= (3, 11):
    huge = b'{"jsonrpc": "2.0", "method": "A.m", "id": ' + b"9" * 6000 + b"}"
    check("huge integer", huge)

print("OK")
`
	driverPath := filepath.Join(tmpDir, "parse_error_driver.py")
	if err := os.WriteFile(driverPath, []byte(driver), 0644); err != nil {
		t.Fatalf("failed to write driver: %v", err)
	}

	cmd := exec.Command(pythonExe, "parse_error_driver.py")
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated test server did not return -32700: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "OK") {
		t.Fatalf("driver did not complete: %s", out)
	}
}

func TestGenerateTestServerPyRequestBodyCapIsOneMiB(t *testing.T) {
	baseNs := &NamespaceTypes{Interfaces: []*parser.Interface{{Name: "A"}}}
	code := generateTestServerPyForNamespace(baseNs, nil, nil, "", "")

	for _, want := range []string{
		"MAX_BODY_SIZE = 1024 * 1024",
		"if content_length > self.MAX_BODY_SIZE:",
		"Request body too large",
		"json.loads(body.decode('utf-8'))",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated test_server.py should contain %q", want)
		}
	}
}

// --- C4/C5: idl.json/rootNamespace/checksum and object results ---------------

func TestPythonGeneratorIDLJSONCarriesRootNamespaceAndChecksum(t *testing.T) {
	tmpDir := newPythonTestTempDir(t, "pulserpc-idljson-")

	idl := &parser.IDL{
		RootNamespace: "mapdemo",
		Interfaces: []*parser.Interface{
			{Name: "mapdemo.A", Namespace: "mapdemo"},
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

	raw, err := os.ReadFile(filepath.Join(tmpDir, "mapdemo", "idl.json"))
	if err != nil {
		t.Fatalf("failed to read idl.json: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("idl.json is not valid JSON: %v", err)
	}
	if parsed["rootNamespace"] != "mapdemo" {
		t.Errorf("idl.json rootNamespace = %v, want mapdemo", parsed["rootNamespace"])
	}
	checksum, _ := parsed["checksum"].(string)
	if len(checksum) != 43 {
		t.Errorf("idl.json checksum = %q, want 43-character base64url SHA-256", checksum)
	}
	if _, ok := parsed["interfaces"].([]interface{}); !ok {
		t.Errorf("idl.json must carry interfaces, got %T", parsed["interfaces"])
	}
}

// C1: with several namespaces and no recorded root namespace, the fallback
// entry point must be stable (lexicographically first), not map-iteration order.
func TestPythonGeneratorIDLJSONPlacementIsDeterministic(t *testing.T) {
	idl := &parser.IDL{
		Interfaces: []*parser.Interface{
			{Name: "Zeta", Namespace: "zeta"},
			{Name: "Alpha", Namespace: "alpha"},
		},
	}

	for run := 0; run < 5; run++ {
		tmpDir := newPythonTestTempDir(t, "pulserpc-idljson-multins-")

		p := NewPythonClientServer()
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.String("dir", "", "output dir")
		p.RegisterFlags(fs)
		if err := fs.Set("dir", tmpDir); err != nil {
			t.Fatalf("failed to set dir flag: %v", err)
		}
		if err := p.Generate(idl, fs); err != nil {
			t.Fatalf("run %d: Generate failed: %v", run, err)
		}

		if _, err := os.Stat(filepath.Join(tmpDir, "alpha", "idl.json")); err != nil {
			t.Fatalf("run %d: idl.json must be written to the lexicographically first namespace: %v", run, err)
		}
		if _, err := os.Stat(filepath.Join(tmpDir, "zeta", "idl.json")); err == nil {
			t.Fatalf("run %d: idl.json must not also be written to zeta", run)
		}
	}
}

// --- C11/R26/R27: enum wire strings and float is double ----------------------

func TestPythonGeneratorEnumsAreStringsAndFloatIsDouble(t *testing.T) {
	tmpDir := newPythonTestTempDir(t, "pulserpc-enumfloat-")

	idl := &parser.IDL{
		Structs: []*parser.Struct{{
			Name: "Reading",
			Fields: []*parser.Field{
				{Name: "rate", Type: &parser.Type{BuiltIn: "float"}},
				{Name: "status", Type: &parser.Type{UserDefined: "Status"}},
			},
		}},
		Enums: []*parser.Enum{{
			Name:   "Status",
			Values: []*parser.EnumValue{{Name: "ok"}, {Name: "bad"}},
		}},
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

	content, err := os.ReadFile(filepath.Join(tmpDir, "rpctypes.py"))
	if err != nil {
		t.Fatalf("failed to read rpctypes.py: %v", err)
	}
	types := string(content)
	for _, want := range []string{
		"rate: float",
		"class Status:",
		`ok = "ok"`,
		`bad = "bad"`,
	} {
		if !strings.Contains(types, want) {
			t.Errorf("rpctypes.py should contain %q, got:\n%s", want, types)
		}
	}
}

// --- R16/R18: Python 2 carve-outs --------------------------------------------

func TestPython2GeneratorHasNoGeneratedServerForHttpLimits(t *testing.T) {
	// Python 2 generates metadata + runtime only.  There is therefore no
	// generated HTTP adapter to which the 1 MiB body cap / 405 handling can be
	// added; the cap is exposed as pulserpc.server.DEFAULT_MAX_BODY_SIZE and
	// documented in docs/languages/python/python2.md instead.
	tmpDir := newPythonTestTempDir(t, "pulserpc-python2-limits-")

	idl := &parser.IDL{
		RootNamespace: "demo",
		Interfaces:    []*parser.Interface{{Name: "demo.A", Namespace: "demo"}},
	}

	p := NewPythonClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", "", "output dir")
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}
	if err := fs.Set("python-version", "2"); err != nil {
		t.Fatalf("failed to set python-version flag: %v", err)
	}
	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	for _, name := range []string{"server.py", "client.py", "test_server.py", "test_client.py"} {
		if _, err := os.Stat(filepath.Join(tmpDir, "demo", name)); err == nil {
			t.Errorf("Python 2 must not generate %s", name)
		}
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "pulserpc", "server.py")); err != nil {
		t.Errorf("Python 2 must copy the runtime server.py: %v", err)
	}
}

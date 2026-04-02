package generator

import (
	"flag"
	"os"
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
	assertFilesExist(t, tmpDir, "server.py", "client.py", "types.py", "idl.json")
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
	for _, filename := range []string{"types.py", "server.py", "client.py", "idl.json"} {
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
		assertNamespaceFilesExist(t, tmpDir, ns, "types.py", "server.py", "client.py", "test_server.py", "test_client.py", "__init__.py")
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
	for _, filename := range []string{"types.py", "server.py", "client.py", "test_server.py", "test_client.py"} {
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

	// Verify types.py in common namespace contains struct and enum
	commonTypesPath := filepath.Join(tmpDir, "common", "types.py")
	content, err := os.ReadFile(commonTypesPath)
	if err != nil {
		t.Fatalf("failed to read common/types.py: %v", err)
	}
	commonTypesContent := string(content)
	if !strings.Contains(commonTypesContent, "class Response:") {
		t.Errorf("common/types.py should contain Response dataclass")
	}
	if !strings.Contains(commonTypesContent, "class Status:") {
		t.Errorf("common/types.py should contain Status enum")
	}
	if !strings.Contains(commonTypesContent, "@dataclass") {
		t.Errorf("common/types.py should use @dataclass decorator")
	}

	// Verify types.py in book namespace contains struct
	bookTypesPath := filepath.Join(tmpDir, "book", "types.py")
	content, err = os.ReadFile(bookTypesPath)
	if err != nil {
		t.Fatalf("failed to read book/types.py: %v", err)
	}
	bookTypesContent := string(content)
	if !strings.Contains(bookTypesContent, "class Book:") {
		t.Errorf("book/types.py should contain Book dataclass")
	}

	// Verify server.py imports types from local types.py
	bookServerPath := filepath.Join(tmpDir, "book", "server.py")
	content, err = os.ReadFile(bookServerPath)
	if err != nil {
		t.Fatalf("failed to read book/server.py: %v", err)
	}
	if !strings.Contains(string(content), "from .types import") {
		t.Errorf("book/server.py should import from .types")
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

	// Each namespace dir should have types.py, server.py, client.py, __init__.py
	for _, ns := range []string{"common", "book", "user"} {
		for _, filename := range []string{"types.py", "server.py", "client.py", "__init__.py"} {
			path := filepath.Join(tmpDir, "myapp", "lib", "rpc", ns, filename)
			if _, err := os.Stat(path); err != nil {
				t.Errorf("expected %s in %s namespace at %s, missing: %v", filename, ns, path, err)
			}
		}
	}

	bookTypesPath := filepath.Join(tmpDir, "myapp", "lib", "rpc", "book", "types.py")
	bookTypesContent, err := os.ReadFile(bookTypesPath)
	if err != nil {
		t.Fatalf("failed to read book/types.py: %v", err)
	}
	if !strings.Contains(string(bookTypesContent), "from myapp.lib.rpc.common import Response") {
		t.Errorf("book/types.py should import Response from myapp.lib.rpc.common, got:\n%s", string(bookTypesContent))
	}

	userTypesPath := filepath.Join(tmpDir, "myapp", "lib", "rpc", "user", "types.py")
	userTypesContent, err := os.ReadFile(userTypesPath)
	if err != nil {
		t.Fatalf("failed to read user/types.py: %v", err)
	}
	if !strings.Contains(string(userTypesContent), "from myapp.lib.rpc.common import Response") {
		t.Errorf("user/types.py should import Response from myapp.lib.rpc.common, got:\n%s", string(userTypesContent))
	}

	// pulserpc runtime dir should have its files
	runtimeFiles := []string{"__init__.py", "rpc.py", "server.py", "client.py", "transport.py", "types.py"}
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

	// Verify book/types.py imports from myapp.lib.rpc.common
	bookTypesPath := filepath.Join(tmpDir, "myapp", "lib", "rpc", "book", "types.py")
	content, err := os.ReadFile(bookTypesPath)
	if err != nil {
		t.Fatalf("failed to read book/types.py: %v", err)
	}
	bookTypesContent := string(content)
	if !strings.Contains(bookTypesContent, "from myapp.lib.rpc.common import Address") {
		t.Errorf("book/types.py should contain 'from myapp.lib.rpc.common import Address', got:\n%s", bookTypesContent)
	}

	// Verify user/types.py imports from myapp.lib.rpc.common
	userTypesPath := filepath.Join(tmpDir, "myapp", "lib", "rpc", "user", "types.py")
	content, err = os.ReadFile(userTypesPath)
	if err != nil {
		t.Fatalf("failed to read user/types.py: %v", err)
	}
	userTypesContent := string(content)
	if !strings.Contains(userTypesContent, "from myapp.lib.rpc.common import Address") {
		t.Errorf("user/types.py should contain 'from myapp.lib.rpc.common import Address', got:\n%s", userTypesContent)
	}

	// Verify common/types.py does NOT have cross-namespace imports (it has none)
	commonTypesPath := filepath.Join(tmpDir, "myapp", "lib", "rpc", "common", "types.py")
	content, err = os.ReadFile(commonTypesPath)
	if err != nil {
		t.Fatalf("failed to read common/types.py: %v", err)
	}
	commonTypesContent := string(content)
	if strings.Contains(commonTypesContent, "Cross-namespace imports") {
		t.Errorf("common/types.py should NOT have cross-namespace imports section, got:\n%s", commonTypesContent)
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
	bookTypesPath := filepath.Join(tmpDir, "book", "types.py")
	content, err := os.ReadFile(bookTypesPath)
	if err != nil {
		t.Fatalf("failed to read book/types.py: %v", err)
	}
	bookTypesContent := string(content)
	if !strings.Contains(bookTypesContent, "from common import Address") {
		t.Errorf("book/types.py should contain 'from common import Address' (no package), got:\n%s", bookTypesContent)
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

	// Verify book/types.py imports Item from common even though it's inside an array
	bookTypesPath := filepath.Join(tmpDir, "myapp", "lib", "rpc", "book", "types.py")
	content, err := os.ReadFile(bookTypesPath)
	if err != nil {
		t.Fatalf("failed to read book/types.py: %v", err)
	}
	bookTypesContent := string(content)
	if !strings.Contains(bookTypesContent, "from myapp.lib.rpc.common import Item") {
		t.Errorf("book/types.py should contain 'from myapp.lib.rpc.common import Item' for array element type, got:\n%s", bookTypesContent)
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

package generator

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coopernurse/pulserpc/pkg/parser"
)

func TestJavaPackageToPathHelper(t *testing.T) {
	tests := []struct {
		name     string
		pkg      string
		expected string
	}{
		{
			name:     "simple package",
			pkg:      "com.example",
			expected: "com" + string(filepath.Separator) + "example",
		},
		{
			name:     "nested package",
			pkg:      "com.myapp.rpc",
			expected: "com" + string(filepath.Separator) + "myapp" + string(filepath.Separator) + "rpc",
		},
		{
			name:     "deep package",
			pkg:      "org.company.services.internal",
			expected: "org" + string(filepath.Separator) + "company" + string(filepath.Separator) + "services" + string(filepath.Separator) + "internal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := packageToPath(tt.pkg)
			if result != tt.expected {
				t.Errorf("packageToPath(%q) = %q, want %q", tt.pkg, result, tt.expected)
			}
		})
	}
}

func TestJavaQualifiedPackageAssertions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulserpc-java-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	idl := &parser.IDL{
		Structs: []*parser.Struct{
			{
				Name:      "user.User",
				Namespace: "user",
				Fields:    []*parser.Field{{Name: "username", Type: &parser.Type{BuiltIn: "string"}}},
			},
		},
	}

	p := NewJavaClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", "src/main/java", "output dir")
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}
	if err := fs.Set("package", "com.myapp.rpc"); err != nil {
		t.Fatalf("failed to set package flag: %v", err)
	}

	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	basePath := filepath.Join(tmpDir, "src", "main", "java")

	userDir := filepath.Join(basePath, packageToPath("com.myapp.rpc"), "user")
	if _, err := os.Stat(userDir); err != nil {
		t.Fatalf("expected user namespace directory at %s, missing: %v", userDir, err)
	}

	userPath := filepath.Join(userDir, "User.java")
	if _, err := os.Stat(userPath); err != nil {
		t.Fatalf("expected User.java at %s, missing: %v", userPath, err)
	}

	content, err := os.ReadFile(userPath)
	if err != nil {
		t.Fatalf("failed to read User.java: %v", err)
	}

	expectedPackage := "package com.myapp.rpc.user;"
	if !strings.Contains(string(content), expectedPackage) {
		t.Errorf("User.java should contain %q but got:\n%s", expectedPackage, string(content))
	}

	expectedDirPath := filepath.Join(basePath, "com", "myapp", "rpc", "user")
	if userDir != expectedDirPath {
		t.Errorf("directory path %q != expected %q", userDir, expectedDirPath)
	}
}

func TestJavaRuntimeLocationValidation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulserpc-java-runtime-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	idl := &parser.IDL{
		Structs: []*parser.Struct{
			{
				Name:      "book.Book",
				Namespace: "book",
				Fields:    []*parser.Field{{Name: "title", Type: &parser.Type{BuiltIn: "string"}}},
			},
		},
	}

	p := NewJavaClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", "src/main/java", "output dir")
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}
	if err := fs.Set("package", "com.example.prod"); err != nil {
		t.Fatalf("failed to set package flag: %v", err)
	}

	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	runtimeDir := filepath.Join(tmpDir, "src/main/java/com/bitmechanic/pulserpc")
	if _, err := os.Stat(runtimeDir); err != nil {
		t.Fatalf("runtime should be at %s, missing: %v", runtimeDir, err)
	}

	bookDir := filepath.Join(tmpDir, "src", "main", "java", packageToPath("com.example.prod"), "book")
	bookRuntimePath := filepath.Join(bookDir, "RPCError.java")
	if _, err := os.Stat(bookRuntimePath); err == nil {
		t.Fatalf("runtime should NOT be in namespace directory at %s", bookRuntimePath)
	}
}

func TestJavaCrossNamespaceImportVerification(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulserpc-java-import-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	idl := &parser.IDL{
		Structs: []*parser.Struct{
			{
				Name:      "common.Address",
				Namespace: "common",
				Fields:    []*parser.Field{{Name: "street", Type: &parser.Type{BuiltIn: "string"}}},
			},
			{
				Name:      "user.User",
				Namespace: "user",
				Fields: []*parser.Field{
					{Name: "name", Type: &parser.Type{BuiltIn: "string"}},
					{Name: "address", Type: &parser.Type{UserDefined: "common.Address"}},
				},
			},
		},
	}

	p := NewJavaClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", "src/main/java", "output dir")
	p.RegisterFlags(fs)
	if err := fs.Set("dir", tmpDir); err != nil {
		t.Fatalf("failed to set dir flag: %v", err)
	}
	if err := fs.Set("package", "org.test.app"); err != nil {
		t.Fatalf("failed to set package flag: %v", err)
	}

	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	userPath := filepath.Join(tmpDir, "src", "main", "java", packageToPath("org.test.app"), "user", "User.java")
	content, err := os.ReadFile(userPath)
	if err != nil {
		t.Fatalf("failed to read User.java: %v", err)
	}

	expectedImport := "import org.test.app.common.Address;"
	if !strings.Contains(string(content), expectedImport) {
		t.Errorf("User.java should contain %q but got:\n%s", expectedImport, string(content))
	}
}

func packageToPath(pkg string) string {
	return strings.ReplaceAll(pkg, ".", string(filepath.Separator))
}

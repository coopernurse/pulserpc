package generator

import (
	"flag"
	"fmt"
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

	runtimeDir := filepath.Join(tmpDir, "pulserpc")
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

func TestJavaTestsAreDeterministic(t *testing.T) {
	for iteration := 0; iteration < 3; iteration++ {
		t.Run(fmt.Sprintf("iteration_%d", iteration), func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "pulserpc-java-det-")
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
					{
						Name:      "user.User",
						Namespace: "user",
						Fields: []*parser.Field{
							{Name: "name", Type: &parser.Type{BuiltIn: "string"}},
							{Name: "book", Type: &parser.Type{UserDefined: "book.Book"}},
						},
					},
				},
				Interfaces: []*parser.Interface{
					{
						Name:      "book.Library",
						Namespace: "book",
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
			if err := fs.Set("package", "com.test.ns"); err != nil {
				t.Fatalf("failed to set package flag: %v", err)
			}

			if err := p.Generate(idl, fs); err != nil {
				t.Fatalf("Generate failed: %v", err)
			}

			basePath := filepath.Join(tmpDir, "src", "main", "java")

			bookDir := filepath.Join(basePath, packageToPath("com.test.ns"), "book")
			if _, err := os.Stat(bookDir); err != nil {
				t.Fatalf("expected book namespace directory at %s, missing: %v", bookDir, err)
			}

			userDir := filepath.Join(basePath, packageToPath("com.test.ns"), "user")
			if _, err := os.Stat(userDir); err != nil {
				t.Fatalf("expected user namespace directory at %s, missing: %v", userDir, err)
			}

			bookPath := filepath.Join(bookDir, "Book.java")
			content, err := os.ReadFile(bookPath)
			if err != nil {
				t.Fatalf("failed to read Book.java: %v", err)
			}
			if !strings.Contains(string(content), "package com.test.ns.book;") {
				t.Fatalf("Book.java should contain 'package com.test.ns.book;' but got:\n%s", string(content))
			}

			userPath := filepath.Join(userDir, "User.java")
			userContent, err := os.ReadFile(userPath)
			if err != nil {
				t.Fatalf("failed to read User.java: %v", err)
			}
			if !strings.Contains(string(userContent), "import com.test.ns.book.Book;") {
				t.Fatalf("User.java should contain 'import com.test.ns.book.Book;' but got:\n%s", string(userContent))
			}

			runtimeDir := filepath.Join(tmpDir, "pulserpc")
			if _, err := os.Stat(runtimeDir); err != nil {
				t.Fatalf("runtime should be at %s, missing: %v", runtimeDir, err)
			}
		})
	}
}

package generator

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coopernurse/pulserpc/pkg/parser"
)

func TestCSharpNamespacePaths_ResolveNamespaceDir(t *testing.T) {
	tests := []struct {
		name        string
		baseDir     string
		packageBase string
		namespace   string
		expected    string
	}{
		{
			name:        "empty namespace",
			baseDir:     "/output",
			packageBase: "",
			namespace:   "",
			expected:    "/output",
		},
		{
			name:        "single namespace",
			baseDir:     "/output",
			packageBase: "",
			namespace:   "common",
			expected:    "/output/Common",
		},
		{
			name:        "nested namespace",
			baseDir:     "/output",
			packageBase: "",
			namespace:   "myapp.lib",
			expected:    "/output/Myapp.lib",
		},
		{
			name:        "underscore namespace",
			baseDir:     "/output",
			packageBase: "",
			namespace:   "user_account",
			expected:    "/output/UserAccount",
		},
		{
			name:        "with package base",
			baseDir:     "/output",
			packageBase: "MyApp.Lib.Rpc",
			namespace:   "book",
			expected:    "/output/Book",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := NewCSharpNamespacePaths(tt.baseDir, tt.packageBase)
			result := paths.ResolveNamespaceDir(tt.namespace)
			if result != tt.expected {
				t.Errorf("ResolveNamespaceDir() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCSharpNamespacePaths_ResolveRuntimeDir(t *testing.T) {
	tests := []struct {
		name        string
		baseDir     string
		packageBase string
		expected    string
	}{
		{
			name:        "no package base",
			baseDir:     "/output",
			packageBase: "",
			expected:    "/output/PulseRPC",
		},
		{
			name:        "with package base",
			baseDir:     "/output",
			packageBase: "MyApp.Lib.Rpc",
			expected:    "/output/MyApp/Lib/Rpc/PulseRPC",
		},
		{
			name:        "simple package base",
			baseDir:     "/output",
			packageBase: "MyApp",
			expected:    "/output/MyApp/PulseRPC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := NewCSharpNamespacePaths(tt.baseDir, tt.packageBase)
			result := paths.ResolveRuntimeDir()
			if result != tt.expected {
				t.Errorf("ResolveRuntimeDir() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCSharpNamespacePaths_ResolveOutputPath(t *testing.T) {
	paths := NewCSharpNamespacePaths("/output", "")

	result := paths.ResolveOutputPath("common", "Types.cs")
	expected := "/output/Common/Types.cs"
	if result != expected {
		t.Errorf("ResolveOutputPath() = %v, want %v", result, expected)
	}
}

func TestCSharpNamespacePaths_EnsureNamespaceDir(t *testing.T) {
	tmpDir := t.TempDir()
	paths := NewCSharpNamespacePaths(tmpDir, "")

	dir, err := paths.EnsureNamespaceDir("book")
	if err != nil {
		t.Fatalf("EnsureNamespaceDir() error = %v", err)
	}

	expected := filepath.Join(tmpDir, "Book")
	if dir != expected {
		t.Errorf("EnsureNamespaceDir() = %v, want %v", dir, expected)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Failed to stat directory: %v", err)
	}
	if !info.IsDir() {
		t.Error("Expected directory to exist")
	}
}

func TestCSharpNamespacePaths_EnsureRuntimeDir(t *testing.T) {
	tmpDir := t.TempDir()
	paths := NewCSharpNamespacePaths(tmpDir, "MyApp.Lib.Rpc")

	dir, err := paths.EnsureRuntimeDir()
	if err != nil {
		t.Fatalf("EnsureRuntimeDir() error = %v", err)
	}

	expected := filepath.Join(tmpDir, "MyApp/Lib/Rpc/PulseRPC")
	if dir != expected {
		t.Errorf("EnsureRuntimeDir() = %v, want %v", dir, expected)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Failed to stat directory: %v", err)
	}
	if !info.IsDir() {
		t.Error("Expected directory to exist")
	}
}

func TestNamespaceToPascalCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"common", "Common"},
		{"user_account", "UserAccount"},
		{"my_app_lib", "MyAppLib"},
		{"MyApp", "MyApp"},
		{"myapp", "Myapp"},
		{"my_app", "MyApp"},
		{"a_b_c", "ABC"},
		{"", ""},
	}

	for _, tt := range tests {
		result := NamespaceToPascalCase(tt.input)
		if result != tt.expected {
			t.Errorf("NamespaceToPascalCase(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestCollectSortedNamespaces(t *testing.T) {
	namespaceMap := map[string]*NamespaceTypes{
		"user":   {},
		"common": {},
		"book":   {},
		"admin":  {},
		"":       {},
	}

	result := CollectSortedNamespaces(namespaceMap)

	expected := []string{"admin", "book", "common", "user"}
	if len(result) != len(expected) {
		t.Fatalf("CollectSortedNamespaces() returned %d items, want %d", len(result), len(expected))
	}

	for i, ns := range expected {
		if result[i] != ns {
			t.Errorf("CollectSortedNamespaces()[%d] = %v, want %v", i, result[i], ns)
		}
	}
}

func TestCSharpNamespacePaths_GetRuntimeImport(t *testing.T) {
	tests := []struct {
		name        string
		packageBase string
		expected    string
	}{
		{
			name:        "no package base",
			packageBase: "",
			expected:    "PulseRPC",
		},
		{
			name:        "with package base",
			packageBase: "MyApp.Lib.Rpc",
			expected:    "MyApp.Lib.Rpc.PulseRPC",
		},
		{
			name:        "simple package base",
			packageBase: "MyApp",
			expected:    "MyApp.PulseRPC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := NewCSharpNamespacePaths("/output", tt.packageBase)
			result := paths.GetRuntimeImport()
			if result != tt.expected {
				t.Errorf("GetRuntimeImport() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCSharpNamespacePaths_GetNamespaceImportPrefix(t *testing.T) {
	tests := []struct {
		name        string
		packageBase string
		expected    string
	}{
		{
			name:        "no package base",
			packageBase: "",
			expected:    "",
		},
		{
			name:        "with package base",
			packageBase: "MyApp.Lib.Rpc",
			expected:    "MyApp.Lib.Rpc.",
		},
		{
			name:        "simple package base",
			packageBase: "MyApp",
			expected:    "MyApp.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := NewCSharpNamespacePaths("/output", tt.packageBase)
			result := paths.GetNamespaceImportPrefix()
			if result != tt.expected {
				t.Errorf("GetNamespaceImportPrefix() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCSharpCrossNamespaceImports(t *testing.T) {
	paths := NewCSharpNamespacePaths("/output", "MyApp.Lib.Rpc")

	allNamespaces := []string{"common", "book", "user"}

	tests := []struct {
		name      string
		currentNS string
	}{
		{
			name:      "book imports common and user",
			currentNS: "book",
		},
		{
			name:      "user imports common and book",
			currentNS: "user",
		},
		{
			name:      "common imports book and user",
			currentNS: "common",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtimeImport := paths.GetRuntimeImport()
			prefix := paths.GetNamespaceImportPrefix()

			var otherNS []string
			for _, ns := range allNamespaces {
				if ns != tt.currentNS {
					otherNS = append(otherNS, prefix+ns)
				}
			}

			if runtimeImport != "MyApp.Lib.Rpc.PulseRPC" {
				t.Errorf("runtime import = %v, want MyApp.Lib.Rpc.PulseRPC", runtimeImport)
			}

			if len(otherNS) != 2 {
				t.Fatalf("expected 2 other namespaces, got %d", len(otherNS))
			}

			otherNSMap := make(map[string]bool)
			for _, ns := range otherNS {
				otherNSMap[ns] = true
			}

			expectedOthers := map[string]bool{
				"MyApp.Lib.Rpc.common": true,
				"MyApp.Lib.Rpc.user":   true,
				"MyApp.Lib.Rpc.book":   true,
			}
			delete(expectedOthers, prefix+tt.currentNS)

			for expectedNS := range expectedOthers {
				if !otherNSMap[expectedNS] {
					t.Errorf("missing import: %v", expectedNS)
				}
			}
		})
	}
}

func TestCSharpCrossNamespaceImportsNoPackage(t *testing.T) {
	paths := NewCSharpNamespacePaths("/output", "")

	allNamespaces := []string{"common", "book", "user"}

	tests := []struct {
		name      string
		currentNS string
	}{
		{
			name:      "book imports common and user without package",
			currentNS: "book",
		},
		{
			name:      "user imports common and book without package",
			currentNS: "user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtimeImport := paths.GetRuntimeImport()
			prefix := paths.GetNamespaceImportPrefix()

			var otherNS []string
			for _, ns := range allNamespaces {
				if ns != tt.currentNS {
					otherNS = append(otherNS, prefix+ns)
				}
			}

			if runtimeImport != "PulseRPC" {
				t.Errorf("runtime import = %v, want PulseRPC", runtimeImport)
			}

			if len(otherNS) != 2 {
				t.Fatalf("expected 2 other namespaces, got %d", len(otherNS))
			}

			otherNSMap := make(map[string]bool)
			for _, ns := range otherNS {
				otherNSMap[ns] = true
			}

			expectedOthers := map[string]bool{
				"common": true,
				"user":   true,
				"book":   true,
			}
			delete(expectedOthers, prefix+tt.currentNS)

			for expectedNS := range expectedOthers {
				if !otherNSMap[expectedNS] {
					t.Errorf("missing import: %v", expectedNS)
				}
			}
		})
	}
}

func TestCSharpNamespacePaths_EnsureAllNamespaceDirs(t *testing.T) {
	tmpDir := t.TempDir()
	paths := NewCSharpNamespacePaths(tmpDir, "MyApp.Lib.Rpc")

	namespaceMap := map[string]*NamespaceTypes{
		"common": {},
		"book":   {},
		"user":   {},
	}

	dirs, err := paths.EnsureAllNamespaceDirs(namespaceMap)
	if err != nil {
		t.Fatalf("EnsureAllNamespaceDirs() error = %v", err)
	}

	expectedDirs := map[string]string{
		"common": filepath.Join(tmpDir, "Common"),
		"book":   filepath.Join(tmpDir, "Book"),
		"user":   filepath.Join(tmpDir, "User"),
	}

	for ns, expectedPath := range expectedDirs {
		if dirs[ns] != expectedPath {
			t.Errorf("EnsureAllNamespaceDirs()[%q] = %v, want %v", ns, dirs[ns], expectedPath)
		}

		info, err := os.Stat(expectedPath)
		if err != nil {
			t.Errorf("Failed to stat directory for namespace %s: %v", ns, err)
		}
		if !info.IsDir() {
			t.Errorf("Expected directory to exist for namespace %s", ns)
		}
	}

	runtimeDir := filepath.Join(tmpDir, "MyApp/Lib/Rpc/PulseRPC")
	info, err := os.Stat(runtimeDir)
	if err != nil {
		t.Errorf("Failed to stat runtime directory: %v", err)
	}
	if !info.IsDir() {
		t.Error("Expected runtime directory to exist")
	}
}

func buildCSharpMultiNamespaceIDL() *parser.IDL {
	return &parser.IDL{
		Structs: []*parser.Struct{
			{
				Name:      "Address",
				Namespace: "common",
				Fields: []*parser.Field{
					{Name: "street", Type: &parser.Type{BuiltIn: "string"}},
					{Name: "city", Type: &parser.Type{BuiltIn: "string"}},
				},
			},
			{
				Name:      "Book",
				Namespace: "book",
				Fields: []*parser.Field{
					{Name: "title", Type: &parser.Type{BuiltIn: "string"}},
					{Name: "address", Type: &parser.Type{UserDefined: "common.Address"}},
				},
			},
			{
				Name:      "User",
				Namespace: "user",
				Fields: []*parser.Field{
					{Name: "name", Type: &parser.Type{BuiltIn: "string"}},
					{Name: "address", Type: &parser.Type{UserDefined: "common.Address"}},
				},
			},
		},
		Interfaces: []*parser.Interface{
			{
				Name:      "BookService",
				Namespace: "book",
				Methods: []*parser.Method{
					{
						Name: "getBook",
						Parameters: []*parser.Parameter{
							{Name: "id", Type: &parser.Type{BuiltIn: "string"}},
						},
						ReturnType: &parser.Type{UserDefined: "Book"},
					},
				},
			},
			{
				Name:      "UserService",
				Namespace: "user",
				Methods: []*parser.Method{
					{
						Name: "getUser",
						Parameters: []*parser.Parameter{
							{Name: "id", Type: &parser.Type{BuiltIn: "string"}},
						},
						ReturnType: &parser.Type{UserDefined: "User"},
					},
				},
			},
		},
	}
}

func TestCSharpMultiFileEndToEnd(t *testing.T) {
	tmpDir := t.TempDir()

	idl := buildCSharpMultiNamespaceIDL()

	gen := NewCSharpClientServer()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("dir", tmpDir, "output dir")
	fs.String("package", "MyApp.Lib.Rpc", "base namespace")
	gen.RegisterFlags(fs)

	if err := gen.Generate(idl, fs); err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	assertCSharpFileExists(t, tmpDir, "Common/Types.cs")
	assertCSharpFileExists(t, tmpDir, "Common/Server.cs")
	assertCSharpFileExists(t, tmpDir, "Common/Client.cs")
	assertCSharpFileExists(t, tmpDir, "Book/Types.cs")
	assertCSharpFileExists(t, tmpDir, "Book/Server.cs")
	assertCSharpFileExists(t, tmpDir, "Book/Client.cs")
	assertCSharpFileExists(t, tmpDir, "User/Types.cs")
	assertCSharpFileExists(t, tmpDir, "User/Server.cs")
	assertCSharpFileExists(t, tmpDir, "User/Client.cs")

	assertCSharpFileExists(t, tmpDir, "MyApp/Lib/Rpc/PulseRPC/Client.cs")
	assertCSharpFileExists(t, tmpDir, "MyApp/Lib/Rpc/PulseRPC/Server.cs")
	assertCSharpFileExists(t, tmpDir, "MyApp/Lib/Rpc/PulseRPC/Types.cs")

	assertCSharpDirExists(t, tmpDir, "MyApp/Lib/Rpc/PulseRPC")

	assertCSharpFileContains(t, tmpDir, "Book/Types.cs", "using MyApp.Lib.Rpc.Common;")
	assertCSharpFileContains(t, tmpDir, "User/Types.cs", "using MyApp.Lib.Rpc.Common;")
	assertCSharpFileContains(t, tmpDir, "Book/Server.cs", "using MyApp.Lib.Rpc.PulseRPC;")
	assertCSharpFileContains(t, tmpDir, "User/Server.cs", "using MyApp.Lib.Rpc.PulseRPC;")
}

func assertCSharpFileExists(t *testing.T, dir, relPath string) {
	path := filepath.Join(dir, relPath)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected file to exist: %s", path)
	}
}

func assertCSharpDirExists(t *testing.T, dir, relPath string) {
	path := filepath.Join(dir, relPath)
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		t.Errorf("expected directory to exist: %s", path)
	} else if err != nil {
		t.Errorf("error checking directory %s: %v", path, err)
	} else if !info.IsDir() {
		t.Errorf("expected %s to be a directory", path)
	}
}

func assertCSharpFileContains(t *testing.T, dir, relPath, substr string) {
	path := filepath.Join(dir, relPath)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file %s: %v", path, err)
	}
	if !strings.Contains(string(content), substr) {
		maxLen := 2000
		if len(content) < maxLen {
			maxLen = len(content)
		}
		t.Logf("file content of %s:\n%s", relPath, content[:maxLen])
		t.Errorf("file %s does not contain %q", relPath, substr)
	}
}

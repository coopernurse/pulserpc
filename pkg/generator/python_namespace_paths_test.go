package generator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/coopernurse/pulserpc/pkg/parser"
)

func TestPythonNamespacePathsResolveNamespaceDir(t *testing.T) {
	tests := []struct {
		name        string
		baseDir     string
		namespace   string
		packageBase string
		expected    string
	}{
		{
			name:        "empty namespace returns base dir",
			baseDir:     "/output",
			namespace:   "",
			packageBase: "",
			expected:    "/output",
		},
		{
			name:        "named namespace returns base/namespace",
			baseDir:     "/output",
			namespace:   "common",
			packageBase: "",
			expected:    "/output/common",
		},
		{
			name:        "nested namespace returns base/namespace",
			baseDir:     "/output",
			namespace:   "inc",
			packageBase: "",
			expected:    "/output/inc",
		},
		{
			name:        "named namespace with simple package base",
			baseDir:     "/output",
			namespace:   "common",
			packageBase: "myapp",
			expected:    "/output/myapp/common",
		},
		{
			name:        "named namespace with nested package base",
			baseDir:     "/output",
			namespace:   "common",
			packageBase: "myapp.lib.rpc",
			expected:    "/output/myapp/lib/rpc/common",
		},
		{
			name:        "empty namespace with package base",
			baseDir:     "/output",
			namespace:   "",
			packageBase: "myapp.lib.rpc",
			expected:    "/output/myapp/lib/rpc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := NewPythonNamespacePaths(tt.baseDir, tt.packageBase)
			got := paths.ResolveNamespaceDir(tt.namespace)
			if got != tt.expected {
				t.Errorf("ResolveNamespaceDir(%q) = %q, want %q", tt.namespace, got, tt.expected)
			}
		})
	}
}

func TestPythonNamespacePathsResolveRuntimeDir(t *testing.T) {
	tests := []struct {
		name        string
		baseDir     string
		packageBase string
		expected    string
	}{
		{
			name:        "no package base uses pulserpc in base dir",
			baseDir:     "/output",
			packageBase: "",
			expected:    "/output/pulserpc",
		},
		{
			name:        "simple package base prepends package path",
			baseDir:     "/output",
			packageBase: "myapp",
			expected:    "/output/myapp/pulserpc",
		},
		{
			name:        "nested package base prepends full package path",
			baseDir:     "/output",
			packageBase: "myapp.lib.rpc",
			expected:    "/output/myapp/lib/rpc/pulserpc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := NewPythonNamespacePaths(tt.baseDir, tt.packageBase)
			got := paths.ResolveRuntimeDir()
			if got != tt.expected {
				t.Errorf("ResolveRuntimeDir() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestPythonNamespacePathsResolveOutputPath(t *testing.T) {
	tests := []struct {
		name        string
		baseDir     string
		packageBase string
		namespace   string
		filename    string
		expected    string
	}{
		{
			name:        "types.py in empty namespace",
			baseDir:     "/output",
			packageBase: "",
			namespace:   "",
			filename:    "types.py",
			expected:    "/output/types.py",
		},
		{
			name:        "types.py in common namespace",
			baseDir:     "/output",
			packageBase: "",
			namespace:   "common",
			filename:    "types.py",
			expected:    "/output/common/types.py",
		},
		{
			name:        "server.py in book namespace",
			baseDir:     "/output",
			packageBase: "",
			namespace:   "book",
			filename:    "server.py",
			expected:    "/output/book/server.py",
		},
		{
			name:        "types.py in common namespace with simple package",
			baseDir:     "/output",
			packageBase: "myapp",
			namespace:   "common",
			filename:    "types.py",
			expected:    "/output/myapp/common/types.py",
		},
		{
			name:        "types.py in book namespace with nested package",
			baseDir:     "/output",
			packageBase: "myapp.lib.rpc",
			namespace:   "book",
			filename:    "types.py",
			expected:    "/output/myapp/lib/rpc/book/types.py",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := NewPythonNamespacePaths(tt.baseDir, tt.packageBase)
			got := paths.ResolveOutputPath(tt.namespace, tt.filename)
			if got != tt.expected {
				t.Errorf("ResolveOutputPath(%q, %q) = %q, want %q", tt.namespace, tt.filename, got, tt.expected)
			}
		})
	}
}

func TestPythonNamespacePathsEnsureNamespaceDir(t *testing.T) {
	tmpDir := newPythonTestTempDir(t, "pulserpc-ns-paths-")

	tests := []struct {
		name        string
		namespace   string
		packageBase string
		expected    string
	}{
		{
			name:        "empty namespace without package",
			namespace:   "",
			packageBase: "",
			expected:    tmpDir,
		},
		{
			name:        "named namespace without package",
			namespace:   "common",
			packageBase: "",
			expected:    filepath.Join(tmpDir, "common"),
		},
		{
			name:        "nested namespace without package",
			namespace:   "inc",
			packageBase: "",
			expected:    filepath.Join(tmpDir, "inc"),
		},
		{
			name:        "named namespace with simple package",
			namespace:   "common",
			packageBase: "myapp",
			expected:    filepath.Join(tmpDir, "myapp", "common"),
		},
		{
			name:        "named namespace with nested package",
			namespace:   "book",
			packageBase: "myapp.lib.rpc",
			expected:    filepath.Join(tmpDir, "myapp", "lib", "rpc", "book"),
		},
		{
			name:        "empty namespace with package",
			namespace:   "",
			packageBase: "myapp.lib.rpc",
			expected:    filepath.Join(tmpDir, "myapp", "lib", "rpc"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := NewPythonNamespacePaths(tmpDir, tt.packageBase)
			dir, err := paths.EnsureNamespaceDir(tt.namespace)
			if err != nil {
				t.Fatalf("EnsureNamespaceDir(%q) failed: %v", tt.namespace, err)
			}

			assertDirExists(t, dir)
			assertPathEqual(t, dir, tt.expected)
		})
	}
}

func TestPythonNamespacePathsEnsureRuntimeDir(t *testing.T) {
	tmpDir := newPythonTestTempDir(t, "pulserpc-runtime-dir-")

	tests := []struct {
		name        string
		packageBase string
		expected    string
	}{
		{
			name:        "no package base",
			packageBase: "",
			expected:    filepath.Join(tmpDir, "pulserpc"),
		},
		{
			name:        "with package base",
			packageBase: "myapp.lib.rpc",
			expected:    filepath.Join(tmpDir, "myapp", "lib", "rpc", "pulserpc"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := NewPythonNamespacePaths(tmpDir, tt.packageBase)
			dir, err := paths.EnsureRuntimeDir()
			if err != nil {
				t.Fatalf("EnsureRuntimeDir() failed: %v", err)
			}

			assertDirExists(t, dir)
			assertPathEqual(t, dir, tt.expected)
		})
	}
}

func TestCollectNamespaces(t *testing.T) {
	tests := []struct {
		name         string
		namespaceMap map[string]*NamespaceTypes
		expected     []string
	}{
		{
			name: "single empty namespace",
			namespaceMap: map[string]*NamespaceTypes{
				"": {
					Structs: []*parser.Struct{{Name: "Foo"}},
				},
			},
			expected: []string{""},
		},
		{
			name: "multiple namespaces excludes empty",
			namespaceMap: map[string]*NamespaceTypes{
				"": {
					Structs: []*parser.Struct{{Name: "Foo"}},
				},
				"common": {
					Structs: []*parser.Struct{{Name: "Bar"}},
				},
				"book": {
					Structs: []*parser.Struct{{Name: "Baz"}},
				},
			},
			expected: []string{"common", "book"},
		},
		{
			name: "only named namespaces",
			namespaceMap: map[string]*NamespaceTypes{
				"user": {
					Interfaces: []*parser.Interface{{Name: "UserService"}},
				},
				"common": {
					Enums: []*parser.Enum{{Name: "Status"}},
				},
			},
			expected: []string{"user", "common"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CollectNamespaces(tt.namespaceMap)

			// Convert to map for order-independent comparison
			gotMap := make(map[string]bool)
			for _, ns := range got {
				gotMap[ns] = true
			}

			expectedMap := make(map[string]bool)
			for _, ns := range tt.expected {
				expectedMap[ns] = true
			}

			if len(gotMap) != len(expectedMap) {
				t.Errorf("CollectNamespaces() = %v, want %v", got, tt.expected)
			}

			for ns := range expectedMap {
				if !gotMap[ns] {
					t.Errorf("CollectNamespaces() missing namespace %q", ns)
				}
			}
		})
	}
}

func TestPythonNamespacePathsEnsureAllNamespaceDirs(t *testing.T) {
	tmpDir := newPythonTestTempDir(t, "pulserpc-all-ns-dirs-")

	namespaceMap := map[string]*NamespaceTypes{
		"": {
			Interfaces: []*parser.Interface{{Name: "A"}},
		},
		"common": {
			Structs: []*parser.Struct{{Name: "Foo"}},
		},
		"book": {
			Structs: []*parser.Struct{{Name: "Bar"}},
		},
	}

	paths := NewPythonNamespacePaths(tmpDir, "")
	dirs, err := paths.EnsureAllNamespaceDirs(namespaceMap)
	if err != nil {
		t.Fatalf("EnsureAllNamespaceDirs() failed: %v", err)
	}

	// Verify all namespace directories were created
	expectedDirs := map[string]string{
		"":       tmpDir,
		"common": filepath.Join(tmpDir, "common"),
		"book":   filepath.Join(tmpDir, "book"),
	}

	for ns, expectedDir := range expectedDirs {
		if gotDir, ok := dirs[ns]; !ok {
			t.Errorf("missing directory for namespace %q", ns)
		} else {
			assertPathEqual(t, gotDir, expectedDir)
		}
		assertDirExists(t, expectedDir)
	}

	// Verify runtime directory was created
	runtimeDir := paths.ResolveRuntimeDir()
	assertDirExists(t, runtimeDir)
}

func TestPythonNamespacePathsWithNestedOutputDirs(t *testing.T) {
	tmpDir := newPythonTestTempDir(t, "pulserpc-nested-")

	// Test with nested output directory
	nestedDir := filepath.Join(tmpDir, "gen", "python", "v1")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}

	paths := NewPythonNamespacePaths(nestedDir, "myapp.lib.rpc")

	// Verify namespace dir resolution
	got := paths.ResolveNamespaceDir("common")
	expected := filepath.Join(nestedDir, "myapp", "lib", "rpc", "common")
	assertPathEqual(t, got, expected)

	// Verify runtime dir resolution
	got = paths.ResolveRuntimeDir()
	expected = filepath.Join(nestedDir, "myapp", "lib", "rpc", "pulserpc")
	assertPathEqual(t, got, expected)

	// Ensure directories can be created
	if _, err := paths.EnsureNamespaceDir("common"); err != nil {
		t.Fatalf("EnsureNamespaceDir(\"common\") failed: %v", err)
	}

	if _, err := paths.EnsureRuntimeDir(); err != nil {
		t.Fatalf("EnsureRuntimeDir() failed: %v", err)
	}

	// Verify they exist
	if _, err := os.Stat(filepath.Join(nestedDir, "myapp", "lib", "rpc", "common")); err != nil {
		t.Fatalf("common directory not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(nestedDir, "myapp", "lib", "rpc", "pulserpc")); err != nil {
		t.Fatalf("runtime directory not created: %v", err)
	}
}

func TestPythonNamespacePathsMultipleNamespaces(t *testing.T) {
	tmpDir := newPythonTestTempDir(t, "pulserpc-multi-ns-")

	// Simulate multi-namespace IDL
	namespaceMap := map[string]*NamespaceTypes{
		"common": {
			Structs: []*parser.Struct{{Name: "common.Response"}},
			Enums:   []*parser.Enum{{Name: "common.Status"}},
		},
		"book": {
			Structs:    []*parser.Struct{{Name: "book.Book"}},
			Interfaces: []*parser.Interface{{Name: "book.BookService"}},
		},
		"user": {
			Structs:    []*parser.Struct{{Name: "user.User"}},
			Interfaces: []*parser.Interface{{Name: "user.UserService"}},
		},
	}

	paths := NewPythonNamespacePaths(tmpDir, "myapp.rpc")

	// Create all namespace directories
	dirs, err := paths.EnsureAllNamespaceDirs(namespaceMap)
	if err != nil {
		t.Fatalf("EnsureAllNamespaceDirs() failed: %v", err)
	}

	// Verify each namespace has expected paths
	expectedPaths := map[string]string{
		"common": filepath.Join(tmpDir, "myapp", "rpc", "common"),
		"book":   filepath.Join(tmpDir, "myapp", "rpc", "book"),
		"user":   filepath.Join(tmpDir, "myapp", "rpc", "user"),
	}

	for ns, expectedPath := range expectedPaths {
		gotPath, ok := dirs[ns]
		if !ok {
			t.Errorf("missing dir for namespace %q", ns)
			continue
		} else {
			assertPathEqual(t, gotPath, expectedPath)
		}

		// Verify output paths for standard files
		for _, filename := range []string{"types.py", "server.py", "client.py"} {
			outputPath := paths.ResolveOutputPath(ns, filename)
			expectedOutput := filepath.Join(expectedPath, filename)
			assertPathEqual(t, outputPath, expectedOutput)
		}
	}

	// Verify runtime path
	runtimeDir := paths.ResolveRuntimeDir()
	expectedRuntime := filepath.Join(tmpDir, "myapp", "rpc", "pulserpc")
	assertPathEqual(t, runtimeDir, expectedRuntime)
}

package generator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTsNamespaceOutputDir(t *testing.T) {
	tests := []struct {
		name      string
		outputDir string
		namespace string
		expected  string
	}{
		{
			name:      "simple namespace",
			outputDir: "lib/rpc",
			namespace: "book",
			expected:  filepath.Join("lib", "rpc", "book"),
		},
		{
			name:      "common namespace",
			outputDir: "lib/rpc",
			namespace: "common",
			expected:  filepath.Join("lib", "rpc", "common"),
		},
		{
			name:      "empty output dir",
			outputDir: "",
			namespace: "user",
			expected:  "user",
		},
		{
			name:      "empty namespace",
			outputDir: "lib/rpc",
			namespace: "",
			expected:  filepath.Join("lib", "rpc"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tsNamespaceOutputDir(tt.outputDir, tt.namespace)
			if got != tt.expected {
				t.Errorf("tsNamespaceOutputDir(%q, %q) = %q, want %q", tt.outputDir, tt.namespace, got, tt.expected)
			}
		})
	}
}

func TestTsRuntimeImportPath(t *testing.T) {
	tests := []struct {
		name              string
		inNamespaceSubdir bool
		expected          string
	}{
		{
			name:              "single-namespace flat output",
			inNamespaceSubdir: false,
			expected:          "./pulserpc",
		},
		{
			name:              "namespace subdirectory",
			inNamespaceSubdir: true,
			expected:          "../pulserpc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tsRuntimeImportPath(tt.inNamespaceSubdir)
			if got != tt.expected {
				t.Errorf("tsRuntimeImportPath(%v) = %q, want %q", tt.inNamespaceSubdir, got, tt.expected)
			}
		})
	}
}

func TestTsCrossNamespaceImportPath(t *testing.T) {
	tests := []struct {
		name          string
		fromNamespace string
		toNamespace   string
		expected      string
	}{
		{
			name:          "book to common",
			fromNamespace: "book",
			toNamespace:   "common",
			expected:      "../common",
		},
		{
			name:          "user to common",
			fromNamespace: "user",
			toNamespace:   "common",
			expected:      "../common",
		},
		{
			name:          "common to book",
			fromNamespace: "common",
			toNamespace:   "book",
			expected:      "../book",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tsCrossNamespaceImportPath(tt.fromNamespace, tt.toNamespace)
			if got != tt.expected {
				t.Errorf("tsCrossNamespaceImportPath(%q, %q) = %q, want %q", tt.fromNamespace, tt.toNamespace, got, tt.expected)
			}
		})
	}
}

func TestTsEnsureNamespaceDirs(t *testing.T) {
	t.Run("creates expected directories", func(t *testing.T) {
		tmpDir := t.TempDir()
		namespaces := []string{"book", "common", "user"}

		err := ensureTsNamespaceDirs(tmpDir, namespaces)
		if err != nil {
			t.Fatalf("ensureTsNamespaceDirs() failed: %v", err)
		}

		for _, ns := range namespaces {
			dir := filepath.Join(tmpDir, ns)
			info, err := os.Stat(dir)
			if err != nil {
				t.Errorf("directory %q should exist: %v", dir, err)
				continue
			}
			if !info.IsDir() {
				t.Errorf("expected %q to be a directory", dir)
			}
		}
	})

	t.Run("is idempotent", func(t *testing.T) {
		tmpDir := t.TempDir()
		namespaces := []string{"book", "common"}

		// First call
		err := ensureTsNamespaceDirs(tmpDir, namespaces)
		if err != nil {
			t.Fatalf("first ensureTsNamespaceDirs() failed: %v", err)
		}

		// Second call should not error
		err = ensureTsNamespaceDirs(tmpDir, namespaces)
		if err != nil {
			t.Fatalf("second ensureTsNamespaceDirs() failed (should be idempotent): %v", err)
		}
	})

	t.Run("empty namespaces list does nothing", func(t *testing.T) {
		tmpDir := t.TempDir()

		err := ensureTsNamespaceDirs(tmpDir, []string{})
		if err != nil {
			t.Fatalf("ensureTsNamespaceDirs() with empty list failed: %v", err)
		}
	})

	t.Run("single namespace with output dir", func(t *testing.T) {
		tmpDir := t.TempDir()
		namespaces := []string{"book"}

		err := ensureTsNamespaceDirs(tmpDir, namespaces)
		if err != nil {
			t.Fatalf("ensureTsNamespaceDirs() failed: %v", err)
		}

		dir := filepath.Join(tmpDir, "book")
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("directory %q should exist: %v", dir, err)
		}
	})
}

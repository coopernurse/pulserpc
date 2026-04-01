package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newPythonTestTempDir(t *testing.T, prefix string) string {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", prefix)
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	t.Cleanup(func() {
		_ = os.RemoveAll(tmpDir)
	})

	return tmpDir
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected file %s, missing: %v", path, err)
	}
	if info.IsDir() {
		t.Fatalf("expected file %s, found directory", path)
	}
}

func assertDirExists(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected directory %s, missing: %v", path, err)
	}
	if !info.IsDir() {
		t.Fatalf("expected directory %s, found file", path)
	}
}

func assertFileContains(t *testing.T, path string, want string) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	if !containsString(string(content), want) {
		t.Fatalf("expected %s to contain %q, got:\n%s", path, want, string(content))
	}
}

func assertFilesExist(t *testing.T, dir string, files ...string) {
	t.Helper()

	for _, file := range files {
		assertFileExists(t, filepath.Join(dir, file))
	}
}

func assertPathEqual(t *testing.T, got, want string) {
	t.Helper()

	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func assertNamespaceFilesExist(t *testing.T, baseDir string, namespace string, files ...string) {
	t.Helper()

	nsDir := baseDir
	if namespace != "" {
		nsDir = filepath.Join(baseDir, namespace)
	}
	assertFilesExist(t, nsDir, files...)
}

func containsString(s, substr string) bool {
	return strings.Contains(s, substr)
}

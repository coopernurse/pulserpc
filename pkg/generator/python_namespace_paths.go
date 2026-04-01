package generator

import (
	"os"
	"path/filepath"
)

// PythonNamespacePaths holds the resolved output paths for Python multi-namespace generation
type PythonNamespacePaths struct {
	// BaseDir is the root output directory (from -dir flag)
	BaseDir string
	// PackageBase is the Python package prefix (from -package flag), e.g., "myapp.lib.rpc"
	PackageBase string
}

// ResolveNamespaceDir returns the output directory path for a given namespace.
// For the empty/default namespace, returns BaseDir directly.
// For named namespaces, returns {BaseDir}/{namespace}/
func (p PythonNamespacePaths) ResolveNamespaceDir(namespace string) string {
	if namespace == "" {
		return p.BaseDir
	}
	return filepath.Join(p.BaseDir, namespace)
}

// ResolveRuntimeDir returns the output directory path for the pulserpc runtime.
// When PackageBase is set, returns {BaseDir}/{PackageBase}/pulserpc/
// When PackageBase is empty, returns {BaseDir}/pulserpc/
func (p PythonNamespacePaths) ResolveRuntimeDir() string {
	base := p.BaseDir
	if p.PackageBase != "" {
		pkgParts := splitByDot(p.PackageBase)
		base = filepath.Join(append([]string{p.BaseDir}, pkgParts...)...)
	}
	return filepath.Join(base, "pulserpc")
}

// ResolveOutputPath returns the full path for a file within a namespace directory.
// For example: ResolveOutputPath("common", "types.py") -> {BaseDir}/common/types.py
func (p PythonNamespacePaths) ResolveOutputPath(namespace, filename string) string {
	dir := p.ResolveNamespaceDir(namespace)
	return filepath.Join(dir, filename)
}

// EnsureNamespaceDir creates the directory for a namespace if it doesn't exist.
// Returns the resolved directory path.
func (p PythonNamespacePaths) EnsureNamespaceDir(namespace string) (string, error) {
	dir := p.ResolveNamespaceDir(namespace)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// EnsureRuntimeDir creates the runtime directory if it doesn't exist.
// Returns the resolved directory path.
func (p PythonNamespacePaths) EnsureRuntimeDir() (string, error) {
	dir := p.ResolveRuntimeDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// CollectNamespaces returns a sorted list of all namespaces present in the IDL.
// The empty namespace (for types without a namespace) is excluded from the list
// unless it's the only namespace present.
func CollectNamespaces(namespaceMap map[string]*NamespaceTypes) []string {
	namespaces := make([]string, 0)
	hasEmptyNamespace := false

	for ns := range namespaceMap {
		if ns == "" {
			hasEmptyNamespace = true
			continue
		}
		namespaces = append(namespaces, ns)
	}

	// If no named namespaces exist, include the empty namespace for single-namespace mode
	if len(namespaces) == 0 && hasEmptyNamespace {
		namespaces = append(namespaces, "")
	}

	return namespaces
}

// NewPythonNamespacePaths creates a new PythonNamespacePaths instance
func NewPythonNamespacePaths(baseDir, packageBase string) PythonNamespacePaths {
	return PythonNamespacePaths{
		BaseDir:     baseDir,
		PackageBase: packageBase,
	}
}

// EnsureAllNamespaceDirs creates directories for all namespaces in the IDL.
// This handles both include-driven (multiple namespaces) and single-file generation.
// Returns a map of namespace -> directory path.
func (p PythonNamespacePaths) EnsureAllNamespaceDirs(namespaceMap map[string]*NamespaceTypes) (map[string]string, error) {
	dirs := make(map[string]string)

	// Ensure runtime directory
	if _, err := p.EnsureRuntimeDir(); err != nil {
		return nil, err
	}

	// Ensure directory for each namespace
	for ns := range namespaceMap {
		dir, err := p.EnsureNamespaceDir(ns)
		if err != nil {
			return nil, err
		}
		dirs[ns] = dir
	}

	return dirs, nil
}

// splitByDot splits a string by dots (used for Python package paths)
func splitByDot(s string) []string {
	result := make([]string, 0)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			if i > start {
				result = append(result, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}

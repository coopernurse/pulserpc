package generator

import (
	"os"
	"path/filepath"
)

// TSNamespacePaths holds the resolved output paths for TypeScript multi-namespace generation
type TSNamespacePaths struct {
	BaseDir     string
	PackageBase string
}

// NewTSNamespacePaths creates a new TSNamespacePaths instance
func NewTSNamespacePaths(baseDir, packageBase string) TSNamespacePaths {
	return TSNamespacePaths{
		BaseDir:     baseDir,
		PackageBase: packageBase,
	}
}

// ResolveNamespaceDir returns the output directory path for a given namespace.
// When PackageBase is set, namespaces are placed under {BaseDir}/{PackageBase}/{namespace}/
// For the empty/default namespace with PackageBase, returns {BaseDir}/{PackageBase}/
// For named namespaces without PackageBase, returns {BaseDir}/{namespace}/
func (p TSNamespacePaths) ResolveNamespaceDir(namespace string) string {
	base := p.BaseDir
	if p.PackageBase != "" {
		base = filepath.Join(p.BaseDir, p.PackageBase)
	}
	if namespace == "" {
		return base
	}
	return filepath.Join(base, namespace)
}

// ResolveRuntimeDir returns the output directory path for the pulserpc runtime.
// When PackageBase is set, returns {BaseDir}/{PackageBase}/pulserpc/
// When PackageBase is empty, returns {BaseDir}/pulserpc/
func (p TSNamespacePaths) ResolveRuntimeDir() string {
	base := p.BaseDir
	if p.PackageBase != "" {
		base = filepath.Join(p.BaseDir, p.PackageBase)
	}
	return filepath.Join(base, "pulserpc")
}

// EnsureNamespaceDir creates the output directory for a namespace if it doesn't exist.
func (p TSNamespacePaths) EnsureNamespaceDir(namespace string) error {
	dir := p.ResolveNamespaceDir(namespace)
	return os.MkdirAll(dir, 0755)
}

// EnsureRuntimeDir creates the runtime directory if it doesn't exist.
func (p TSNamespacePaths) EnsureRuntimeDir() error {
	dir := p.ResolveRuntimeDir()
	return os.MkdirAll(dir, 0755)
}

// tsNamespaceOutputDir returns the directory path for a given namespace's generated files.
// Deprecated: Use TSNamespacePaths.ResolveNamespaceDir instead.
func tsNamespaceOutputDir(outputDir, namespace string) string {
	return filepath.Join(outputDir, namespace)
}

// tsRuntimeImportPath returns the relative import path to the pulserpc runtime module.
// When inNamespaceSubdir is false (single-namespace flat output), returns "./pulserpc".
// When inNamespaceSubdir is true (files inside a namespace subdirectory), returns "../pulserpc".
func tsRuntimeImportPath(inNamespaceSubdir bool) string {
	if inNamespaceSubdir {
		return "../pulserpc"
	}
	return "./pulserpc"
}

// tsCrossNamespaceImportPath returns the relative import path from one namespace to another.
// Both fromNamespace and toNamespace are namespace names (e.g., "book", "common").
// Returns "../{toNamespace}/types.js" for use inside a namespace subdirectory.
func tsCrossNamespaceImportPath(_ string, toNamespace string) string {
	return "../" + toNamespace + "/types.js"
}

// ensureTsNamespaceDirs creates the output directories for all given namespaces.
// For each namespace, it calls os.MkdirAll on filepath.Join(outputDir, namespace).
// This function is idempotent - calling it multiple times is safe.
func ensureTsNamespaceDirs(outputDir string, namespaces []string) error {
	for _, ns := range namespaces {
		dir := tsNamespaceOutputDir(outputDir, ns)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

// isMultiNamespaceMode determines whether multi-namespace output mode should be activated.
// Multi-namespace mode is active only when there are multiple namespaces (len(namespaceMap) > 1).
// When multi-namespace mode is active, generated files go into per-namespace subdirectories.
// When inactive, all files go into the root output directory (backwards compatible).
func isMultiNamespaceMode(_ string, namespaceMap map[string]*NamespaceTypes) bool {
	return len(namespaceMap) > 1
}

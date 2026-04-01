package generator

import (
	"os"
	"path/filepath"
)

// tsNamespaceOutputDir returns the directory path for a given namespace's generated files.
// For example: tsNamespaceOutputDir("lib/rpc", "book") returns "lib/rpc/book".
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
// Returns "../{toNamespace}" for use inside a namespace subdirectory.
func tsCrossNamespaceImportPath(_ string, toNamespace string) string {
	return "../" + toNamespace
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

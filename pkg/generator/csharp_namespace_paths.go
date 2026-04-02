package generator

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type CSharpNamespacePaths struct {
	BaseDir     string
	PackageBase string
}

func NewCSharpNamespacePaths(baseDir, packageBase string) CSharpNamespacePaths {
	return CSharpNamespacePaths{
		BaseDir:     baseDir,
		PackageBase: packageBase,
	}
}

func (p CSharpNamespacePaths) ResolveNamespaceDir(namespace string) string {
	base := p.BaseDir
	if p.PackageBase != "" {
		parts := splitByDot(p.PackageBase)
		base = filepath.Join(append([]string{p.BaseDir}, parts...)...)
	}
	if namespace == "" {
		return base
	}
	return filepath.Join(base, NamespaceToPascalCase(namespace))
}

func (p CSharpNamespacePaths) ResolveRuntimeDir() string {
	base := p.BaseDir
	if p.PackageBase != "" {
		parts := splitByDot(p.PackageBase)
		base = filepath.Join(append([]string{p.BaseDir}, parts...)...)
	}
	return filepath.Join(base, "PulseRPC")
}

func (p CSharpNamespacePaths) ResolveOutputPath(namespace, filename string) string {
	dir := p.ResolveNamespaceDir(namespace)
	return filepath.Join(dir, filename)
}

func (p CSharpNamespacePaths) EnsureNamespaceDir(namespace string) (string, error) {
	dir := p.ResolveNamespaceDir(namespace)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func (p CSharpNamespacePaths) EnsureRuntimeDir() (string, error) {
	dir := p.ResolveRuntimeDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func (p CSharpNamespacePaths) EnsureAllNamespaceDirs(namespaceMap map[string]*NamespaceTypes) (map[string]string, error) {
	dirs := make(map[string]string)

	if _, err := p.EnsureRuntimeDir(); err != nil {
		return nil, err
	}

	for ns := range namespaceMap {
		dir, err := p.EnsureNamespaceDir(ns)
		if err != nil {
			return nil, err
		}
		dirs[ns] = dir
	}

	return dirs, nil
}

func NamespaceToPascalCase(namespace string) string {
	parts := strings.Split(namespace, "_")
	result := ""
	for _, part := range parts {
		if len(part) > 0 {
			result += strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return result
}

func CollectSortedNamespaces(namespaceMap map[string]*NamespaceTypes) []string {
	namespaces := make([]string, 0)
	hasEmptyNamespace := false

	for ns := range namespaceMap {
		if ns == "" {
			hasEmptyNamespace = true
			continue
		}
		namespaces = append(namespaces, ns)
	}

	sort.Strings(namespaces)

	if len(namespaces) == 0 && hasEmptyNamespace {
		namespaces = append(namespaces, "")
	}

	return namespaces
}

func (p CSharpNamespacePaths) GetRuntimeImport() string {
	if p.PackageBase != "" {
		return p.PackageBase + ".PulseRPC"
	}
	return "PulseRPC"
}

func (p CSharpNamespacePaths) GetNamespaceImportPrefix() string {
	if p.PackageBase != "" {
		return p.PackageBase + "."
	}
	return ""
}

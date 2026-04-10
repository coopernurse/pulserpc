package runtime

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Embed all Python runtime files
// The embed path is relative to this file's location in pkg/runtime/
// Note: Go's embed doesn't support ".." paths, so runtime files are located in
// pkg/runtime/runtimes/ to enable embedding. This allows the binary to be
// self-contained without requiring the source tree at runtime.
//
//go:embed all:runtimes/python/pulserpc
var pythonRuntimeFiles embed.FS

// Embed all TypeScript runtime files
//
//go:embed all:runtimes/ts/pulserpc
var tsRuntimeFiles embed.FS

// Embed all C# runtime files
//
//go:embed all:runtimes/csharp/PulseRPC
var csharpRuntimeFiles embed.FS

// Embed all Java runtime files
//
//go:embed all:runtimes/java/com/bitmechanic/pulserpc
var javaRuntimeFiles embed.FS

// Embed all Go runtime files
//
//go:embed all:runtimes/go/pulserpc
var goRuntimeFiles embed.FS

// Embed all Python 2 runtime files
//
//go:embed all:runtimes/python2/pulserpc
var python2RuntimeFiles embed.FS

// runtimeMap maps language names to their embedded file systems
var runtimeMap = map[string]embed.FS{
	"python":  pythonRuntimeFiles,
	"python2": python2RuntimeFiles,
	"ts":      tsRuntimeFiles,
	"csharp":  csharpRuntimeFiles,
	"java":    javaRuntimeFiles,
	"go":      goRuntimeFiles,
}

// ListRuntimes returns a list of all available embedded runtimes
func ListRuntimes() []string {
	runtimes := make([]string, 0, len(runtimeMap))
	for lang := range runtimeMap {
		runtimes = append(runtimes, lang)
	}
	return runtimes
}

// GetRuntimeFiles returns a map of filename -> file contents for the specified language runtime
func GetRuntimeFiles(lang string) (map[string][]byte, error) {
	fs, ok := runtimeMap[lang]
	if !ok {
		return nil, fmt.Errorf("runtime for language %q not found (available: %v)", lang, ListRuntimes())
	}

	files := make(map[string][]byte)

	// The embed path includes the directory structure, so we need to walk it
	// For Python, files are at: runtimes/python/pulserpc/*.py
	basePath := getRuntimeEmbedPath(lang)

	entries, err := fs.ReadDir(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded runtime directory for %s: %w", lang, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Filter files by language-specific extension
		if (lang == "python" || lang == "python2") && !strings.HasSuffix(entry.Name(), ".py") {
			continue
		}
		if lang == "ts" && !strings.HasSuffix(entry.Name(), ".ts") {
			continue
		}
		if lang == "csharp" && !strings.HasSuffix(entry.Name(), ".cs") {
			continue
		}
		if lang == "java" && !strings.HasSuffix(entry.Name(), ".java") {
			continue
		}
		if lang == "go" && !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}

		filePath := filepath.Join(basePath, entry.Name())
		data, err := fs.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded runtime file %s: %w", entry.Name(), err)
		}

		// Extract just the filename (not the full path) for the map key
		files[entry.Name()] = data
	}

	return files, nil
}

// CopyRuntimeFiles copies all runtime files for the specified language to the output directory
// The files are copied to outputDir/{runtimePackageName}/ where runtimePackageName is typically
// "pulserpc" for most languages, "PulseRPC" for C#, "com/bitmechanic/pulserpc" for Java
// If silent is true, no file paths are printed
func CopyRuntimeFiles(lang string, outputDir string, silent bool) error {
	return CopyRuntimeFilesToPackage(lang, outputDir, getRuntimePackageName(lang), silent)
}

// CopyRuntimeFilesToPackage copies all runtime files for the specified language to the output directory
// using the specified package name (relative to outputDir).
// If packageName is empty, files are copied directly into outputDir.
// If silent is true, no file paths are printed
func CopyRuntimeFilesToPackage(lang string, outputDir string, packageName string, silent bool) error {
	files, err := GetRuntimeFiles(lang)
	if err != nil {
		return err
	}

	runtimeDir := outputDir
	if packageName != "" {
		runtimeDir = filepath.Join(outputDir, packageName)
	}

	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		return fmt.Errorf("failed to create runtime directory: %w", err)
	}

	// Copy all files
	for filename, data := range files {
		dstPath := filepath.Join(runtimeDir, filename)
		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write runtime file %s: %w", dstPath, err)
		}
		// Print file path unless silent mode
		if !silent {
			fmt.Println(dstPath)
		}
	}

	return nil
}

// getRuntimePackageName returns the package/module name for the runtime library
// This is the directory name where runtime files are placed in the output
func getRuntimePackageName(lang string) string {
	switch lang {
	case "go", "python", "python2", "ts":
		return "pulserpc"
	case "java":
		return "com/bitmechanic/pulserpc"
	case "csharp":
		return "PulseRPC"
	default:
		return "pulserpc"
	}
}

// getRuntimeEmbedPath returns the embed filesystem path for the runtime library
// This is the path used in //go:embed directives and must match the actual directory structure
func getRuntimeEmbedPath(lang string) string {
	switch lang {
	case "go", "python", "python2", "ts":
		return fmt.Sprintf("runtimes/%s/pulserpc", lang)
	case "java":
		return "runtimes/java/com/bitmechanic/pulserpc"
	case "csharp":
		return "runtimes/csharp/PulseRPC"
	default:
		return fmt.Sprintf("runtimes/%s/pulserpc", lang)
	}
}

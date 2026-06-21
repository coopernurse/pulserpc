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

// Embed all TypeScript runtime files (Node ESM variant)
//
//go:embed all:runtimes/ts-node/pulserpc
var tsNodeRuntimeFiles embed.FS

// Embed all TypeScript runtime files (CommonJS variant)
//
//go:embed all:runtimes/ts-cjs/pulserpc
var tsCjsRuntimeFiles embed.FS

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

// runtimeMap maps language names to their embedded file systems.
// "ts" and "ts-node" both point at the Node ESM tree; "ts-cjs" points at the
// CommonJS tree. Style selection happens in GetRuntimeFilesForStyle.
var runtimeMap = map[string]embed.FS{
	"python":  pythonRuntimeFiles,
	"python2": python2RuntimeFiles,
	"ts":      tsNodeRuntimeFiles,
	"ts-node": tsNodeRuntimeFiles,
	"ts-cjs":  tsCjsRuntimeFiles,
	"csharp":  csharpRuntimeFiles,
	"java":    javaRuntimeFiles,
	"go":      goRuntimeFiles,
}

// ListRuntimes returns a list of all available embedded runtime languages.
// Deprecated aliases (e.g. "ts" in addition to "ts-node") are excluded
// from the list so callers see only canonical names.
func ListRuntimes() []string {
	seen := make(map[string]bool)
	var runtimes []string
	for lang := range runtimeMap {
		if seen[lang] {
			continue
		}
		// Skip deprecated aliases. "ts-node" and "ts-cjs" are the
		// canonical TS runtime names; "ts" is a backward-compat alias
		// for "ts-node".
		if lang == "ts" {
			continue
		}
		seen[lang] = true
		runtimes = append(runtimes, lang)
	}
	return runtimes
}

// GetRuntimeFiles returns a map of filename -> file contents for the specified language runtime.
// It is equivalent to GetRuntimeFilesForStyle(lang, "") and is kept for backwards
// compatibility with callers that don't care about TypeScript module style.
func GetRuntimeFiles(lang string) (map[string][]byte, error) {
	return GetRuntimeFilesForStyle(lang, "")
}

// GetRuntimeFilesForStyle returns runtime files for a given language and module style.
// For non-TypeScript languages, style must be empty.
//
// For lang == "ts", style selects between the runtime trees:
//   - "" (default), "esm-node", "node", "esm" → ts-node (Node ESM) tree
//   - "esm-bundler", "bundler"               → ts-node tree; caller must apply
//                                              the bundler post-process transform
//                                              to strip the ".js" import suffix
//   - "cjs", "commonjs"                       → ts-cjs (CommonJS) tree
//
// Unknown styles produce an error.
func GetRuntimeFilesForStyle(lang, style string) (map[string][]byte, error) {
	effectiveLang := lang
	if lang == "ts" {
		switch style {
		case "", "esm-node", "node", "esm":
			effectiveLang = "ts-node"
		case "esm-bundler", "bundler":
			effectiveLang = "ts-node"
		case "cjs", "commonjs":
			effectiveLang = "ts-cjs"
		default:
			return nil, fmt.Errorf("invalid module style %q for language %q (valid: esm-node, esm-bundler, cjs; aliases: esm, node, bundler, commonjs)", style, lang)
		}
	} else if style != "" {
		return nil, fmt.Errorf("module style %q is not supported for language %q", style, lang)
	}

	fs, ok := runtimeMap[effectiveLang]
	if !ok {
		return nil, fmt.Errorf("runtime for language %q not found (available: %v)", effectiveLang, ListRuntimes())
	}

	files := make(map[string][]byte)

	// The embed path includes the directory structure, so we need to walk it
	// For Python, files are at: runtimes/python/pulserpc/*.py
	basePath := getRuntimeEmbedPath(effectiveLang)

	entries, err := fs.ReadDir(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded runtime directory for %s: %w", effectiveLang, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Filter files by language-specific extension
		if (effectiveLang == "python" || effectiveLang == "python2") && !strings.HasSuffix(entry.Name(), ".py") {
			continue
		}
		if (effectiveLang == "ts" || effectiveLang == "ts-node" || effectiveLang == "ts-cjs") && !strings.HasSuffix(entry.Name(), ".ts") {
			continue
		}
		if effectiveLang == "csharp" && !strings.HasSuffix(entry.Name(), ".cs") {
			continue
		}
		if effectiveLang == "java" && !strings.HasSuffix(entry.Name(), ".java") {
			continue
		}
		if effectiveLang == "go" && !strings.HasSuffix(entry.Name(), ".go") {
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
		if lang == "python2" && IsPythonTestFile(filename) {
			continue
		}
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

// IsPythonTestFile returns true if the filename is a Python test file
func IsPythonTestFile(filename string) bool {
	return strings.HasPrefix(filename, "test_") || strings.HasSuffix(filename, "_test.py")
}

// getRuntimePackageName returns the package/module name for the runtime library
// This is the directory name where runtime files are placed in the output
func getRuntimePackageName(lang string) string {
	switch lang {
	case "go", "python", "python2", "ts", "ts-node", "ts-cjs":
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
// This is the path used in //go:embed directives and must match the actual directory structure.
// "ts" and "ts-node" share the ts-node tree; "ts-cjs" lives in its own tree (wired up in step 3).
func getRuntimeEmbedPath(lang string) string {
	switch lang {
	case "go", "python", "python2":
		return fmt.Sprintf("runtimes/%s/pulserpc", lang)
	case "ts", "ts-node":
		return "runtimes/ts-node/pulserpc"
	case "ts-cjs":
		return "runtimes/ts-cjs/pulserpc"
	case "java":
		return "runtimes/java/com/bitmechanic/pulserpc"
	case "csharp":
		return "runtimes/csharp/PulseRPC"
	default:
		return fmt.Sprintf("runtimes/%s/pulserpc", lang)
	}
}

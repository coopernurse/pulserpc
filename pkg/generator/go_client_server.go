package generator

import (
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/coopernurse/pulserpc/pkg/parser"
	"github.com/coopernurse/pulserpc/pkg/runtime"
)

// GoClientServer is a plugin that generates Go HTTP server and client code from IDL
type GoClientServer struct {
}

// NewGoClientServer creates a new GoClientServer plugin instance
func NewGoClientServer() *GoClientServer {
	return &GoClientServer{}
}

// Name returns the plugin identifier
func (p *GoClientServer) Name() string {
	return "go-client-server"
}

// RegisterFlags registers CLI flags for this plugin
func (p *GoClientServer) RegisterFlags(fs *flag.FlagSet) {
	// Register -package flag for base import path (e.g., github.com/myapp/lib/rpc)
	// Only register if not already registered by another plugin (e.g., TypeScript)
	if fs.Lookup("package") == nil {
		fs.String("package", "", "Base import path for generated code (e.g., github.com/myapp/lib/rpc)")
	}
	// Register go-module flag for manual override (deprecated, use -package instead)
	if fs.Lookup("go-module") == nil {
		fs.String("go-module", "", "[DEPRECATED: use -package instead] Override Go module path for pulserpc imports")
	}
	// Register inline-runtime flag (default true for backward compat)
	if fs.Lookup("inline-runtime") == nil {
		fs.Bool("inline-runtime", true, "Place runtime files inline with generated code (for playground/testing)")
	}
	// Struct-tag flags. omit-empty adds omitempty to every field; the default
	// keeps optional-only tags. preserve-empty-optional-arrays emits omitzero on
	// optional slices so an empty non-nil array survives serialization.
	if fs.Lookup("omit-empty") == nil {
		fs.Bool("omit-empty", false, "Add omitempty to every generated Go struct field (default: optional fields only)")
	}
	if fs.Lookup("preserve-empty-optional-arrays") == nil {
		fs.Bool("preserve-empty-optional-arrays", false, "Use omitzero on optional Go slice fields to emit non-nil empty arrays (Go 1.24+)")
	}
}

// writeIDLJSONGo writes the IDL metadata as JSON to idl.json and returns the
// exact bytes that were written so the caller can embed them in server.go.
//
// IDL Placement Note:
// This function intentionally writes idl.json directly to outputDir (the root -dir),
// NOT to a namespace subdirectory. The idl.json file is consumed by the runtime
// Contract class at startup, and it must be accessible as {root}/idl.json regardless
// of which namespace mode is active.
func writeIDLJSONGo(idl *parser.IDL, outputDir string, fs *flag.FlagSet) ([]byte, error) {
	idlJSON, err := MarshalIDLJSON(idl)
	if err != nil {
		return nil, err
	}

	idlPath := filepath.Join(outputDir, "idl.json")
	if err := os.WriteFile(idlPath, idlJSON, 0644); err != nil {
		return nil, fmt.Errorf("failed to write idl.json: %w", err)
	}
	PrintFileCreated(idlPath, fs)
	return idlJSON, nil
}

// detectRuntimeImportPath determines the fully qualified import path for the pulserpc runtime
// It returns (packageImportPath, runtimeImportPath, error)
// - packageImportPath: the base import path for namespace packages (e.g., github.com/myapp/lib/rpc)
// - runtimeImportPath: the import path for the runtime (e.g., github.com/myapp/lib/rpc/pulserpc)
func (p *GoClientServer) detectRuntimeImportPath(outputDir string, fs *flag.FlagSet) (string, string, error) {
	// Check if -package flag is set (preferred)
	packageFlag := fs.Lookup("package")
	if packageFlag != nil && packageFlag.Value.String() != "" {
		basePath := packageFlag.Value.String()
		runtimePath := path.Join(basePath, "pulserpc")
		return basePath, runtimePath, nil
	}

	// Check if -go-module flag is set (deprecated, for backward compat)
	goModuleFlag := fs.Lookup("go-module")
	if goModuleFlag != nil && goModuleFlag.Value.String() != "" {
		// User provided override - use it for both paths
		overridePath := goModuleFlag.Value.String()
		// For the new multi-namespace layout, runtime is at {dir}/pulserpc/
		// The package import path is {go-module}/{dir}/{namespace}
		// The runtime import path is {go-module}/{dir}/pulserpc
		if outputDir != "" && outputDir != "." {
			runtimePath := path.Join(overridePath, outputDir, "pulserpc")
			packagePath := path.Join(overridePath, outputDir)
			return packagePath, runtimePath, nil
		}
		// For override, runtime is at overridePath/pulserpc
		runtimePath := path.Join(overridePath, "pulserpc")
		// And generated package is at overridePath (or with relative path added)
		return overridePath, runtimePath, nil
	}

	// Auto-detect from go.mod
	if outputDir == "" {
		outputDir = "."
	}

	moduleRoot, modulePath, err := findGoMod(outputDir)
	if err != nil {
		return "", "", fmt.Errorf("unable to find go.mod file to determine module path\n\n"+
			"To fix this:\n"+
			"  1. Initialize a Go module: go mod init <module-name>\n"+
			"  2. Or use -package flag: pulserpc -package github.com/myapp/lib/rpc -dir %s <idl-file>\n\n"+
			"Searched from: %s", outputDir, outputDir)
	}

	runtimeImportPath, err := calculateImportPath(moduleRoot, modulePath, outputDir)
	if err != nil {
		return "", "", fmt.Errorf("failed to calculate runtime import path: %w", err)
	}

	// Calculate the import path for the generated package itself
	// If outputDir is the module root, it's just modulePath
	// If outputDir is a subdirectory, we need to include that
	absRoot, err := filepath.Abs(moduleRoot)
	if err != nil {
		return "", "", fmt.Errorf("failed to get absolute path for module root: %w", err)
	}

	absOutput, err := filepath.Abs(outputDir)
	if err != nil {
		return "", "", fmt.Errorf("failed to get absolute path for output dir: %w", err)
	}

	var packageImportPath string
	if absRoot == absOutput {
		// Output dir is module root, package is imported via module path
		packageImportPath = modulePath
	} else {
		// Output dir is a subdirectory, include relative path from module root
		relPath, err := filepath.Rel(absRoot, absOutput)
		if err != nil {
			return "", "", fmt.Errorf("failed to calculate relative path: %w", err)
		}
		packageImportPath = path.Join(modulePath, relPath)
		// Clean path separators to use forward slashes (Go import paths)
		packageImportPath = strings.ReplaceAll(packageImportPath, "\\", "/")
	}

	return packageImportPath, runtimeImportPath, nil
}

// Generate generates Go HTTP server and client code from the parsed IDL
func (p *GoClientServer) Generate(idl *parser.IDL, fs *flag.FlagSet) error {
	// Check silent flag
	silentFlag := fs.Lookup("silent")
	isSilent := func() bool {
		return silentFlag != nil && silentFlag.Value.String() == "true"
	}

	jsonOptions := goJSONOptions{}
	if f := fs.Lookup("omit-empty"); f != nil {
		jsonOptions.omitEmpty = f.Value.String() == "true"
	}
	if f := fs.Lookup("preserve-empty-optional-arrays"); f != nil {
		jsonOptions.preserveEmptyOptionalArrays = f.Value.String() == "true"
	}

	// Access the -dir flag value
	dirFlag := fs.Lookup("dir")
	outputDir := ""
	if dirFlag != nil && dirFlag.Value.String() != "" {
		outputDir = dirFlag.Value.String()
	}

	// Detect module path for fully qualified imports
	packageImportPath, runtimeImportPath, err := p.detectRuntimeImportPath(outputDir, fs)
	if err != nil {
		return fmt.Errorf("failed to detect module path: %w", err)
	}

	// Build type registries
	structMap := make(map[string]*parser.Struct)
	enumMap := make(map[string]*parser.Enum)
	interfaceMap := make(map[string]*parser.Interface)

	for _, s := range idl.Structs {
		structMap[s.Name] = s
	}
	for _, e := range idl.Enums {
		enumMap[e.Name] = e
	}
	for _, i := range idl.Interfaces {
		interfaceMap[i.Name] = i
	}

	// Group types by namespace
	namespaceMap := GroupTypesByNamespace(idl)

	// D12: iterate namespaces in sorted order so generation is byte-deterministic.
	namespaceNames := CollectSortedNamespaces(namespaceMap)

	// A namespace becomes both a directory name and a Go package name. The IDL
	// lexer accepts any [a-zA-Z][a-zA-Z0-9_]*, which includes Go keywords;
	// emitting `package type` would produce unparseable Go. Dots and hyphens
	// cannot occur here (the grammar has no such tokens), so reserved words are
	// the only unsafe namespace shape.
	for _, ns := range namespaceNames {
		if ns != "" && goKeywords[ns] {
			return fmt.Errorf("namespace %q is a Go reserved word and cannot be used as a generated package name", ns)
		}
	}

	// Get the primary namespace for package name
	// Prefer the namespace that has interfaces (the main IDL file)
	// over namespaces that only have imported types
	primaryNs := ""

	// First, try to find a namespace with interfaces
	for _, ns := range namespaceNames {
		if ns != "" && len(namespaceMap[ns].Interfaces) > 0 {
			primaryNs = ns
			break
		}
	}

	// If no namespace has interfaces, pick the first non-empty one
	if primaryNs == "" {
		for _, ns := range namespaceNames {
			if ns != "" {
				primaryNs = ns
				break
			}
		}
	}

	if primaryNs == "" {
		primaryNs = "generated"
	}

	// Copy runtime library files to {dir}/pulserpc/
	if err := p.copyRuntimeFiles(outputDir, isSilent()); err != nil {
		return fmt.Errorf("failed to copy runtime files: %w", err)
	}

	// Write idl.json to output directory and reuse the exact same bytes for the
	// embedded IDL_JSON constant instead of marshalling the IDL twice.
	idlJSON, err := writeIDLJSONGo(idl, outputDir, fs)
	if err != nil {
		return fmt.Errorf("failed to write idl.json: %w", err)
	}

	// Generate namespace subdirectories and files in sorted order.
	for _, namespace := range namespaceNames {
		types := namespaceMap[namespace]
		if namespace == "" {
			continue // Skip types without namespace (shouldn't happen with required namespaces)
		}
		// Create namespace subdirectory
		namespaceDir := filepath.Join(outputDir, namespace)
		if err := os.MkdirAll(namespaceDir, 0755); err != nil {
			return fmt.Errorf("failed to create namespace directory: %w", err)
		}

		namespaceCode := generateNamespaceGo(namespace, types, structMap, enumMap, packageImportPath, jsonOptions)
		namespacePath := filepath.Join(namespaceDir, namespace+".go")
		if err := os.WriteFile(namespacePath, []byte(namespaceCode), 0644); err != nil {
			return fmt.Errorf("failed to write %s/%s.go: %w", namespace, namespace, err)
		}
		PrintFileCreated(namespacePath, fs)
	}

	// Generate server.go in the primary namespace directory
	serverCode := generateServerGo(idl, idlJSON, structMap, enumMap, primaryNs, namespaceMap, runtimeImportPath, packageImportPath)
	serverDir := filepath.Join(outputDir, primaryNs)
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		return fmt.Errorf("failed to create server directory: %w", err)
	}
	serverPath := filepath.Join(serverDir, "server.go")
	if err := os.WriteFile(serverPath, []byte(serverCode), 0644); err != nil {
		return fmt.Errorf("failed to write server.go: %w", err)
	}
	PrintFileCreated(serverPath, fs)

	// Generate client.go in the primary namespace directory
	clientCode := generateClientGo(idl, structMap, enumMap, primaryNs, namespaceMap, runtimeImportPath, packageImportPath)
	clientDir := filepath.Join(outputDir, primaryNs)
	if err := os.MkdirAll(clientDir, 0755); err != nil {
		return fmt.Errorf("failed to create client directory: %w", err)
	}
	clientPath := filepath.Join(clientDir, "client.go")
	if err := os.WriteFile(clientPath, []byte(clientCode), 0644); err != nil {
		return fmt.Errorf("failed to write client.go: %w", err)
	}
	PrintFileCreated(clientPath, fs)

	// Check if generate-test-files flag is set
	generateTestFilesFlag := fs.Lookup("generate-test-files")
	generateTestServer := generateTestFilesFlag != nil && generateTestFilesFlag.Value.String() == "true"

	// Generate test server and client if flag is set
	if generateTestServer {
		// The import path for the generated package includes the primary namespace
		primaryImportPath := path.Join(packageImportPath, primaryNs)
		// Generate cmd/test_server/main.go
		testServerCode := generateTestServerGo(idl, structMap, enumMap, primaryImportPath, packageImportPath, namespaceMap, primaryNs, runtimeImportPath)
		testServerDir := filepath.Join(outputDir, "cmd", "test_server")
		if err := os.MkdirAll(testServerDir, 0755); err != nil {
			return fmt.Errorf("failed to create test_server directory: %w", err)
		}
		testServerPath := filepath.Join(testServerDir, "main.go")
		if err := os.WriteFile(testServerPath, []byte(testServerCode), 0644); err != nil {
			return fmt.Errorf("failed to write test_server/main.go: %w", err)
		}
		PrintFileCreated(testServerPath, fs)

		// Generate cmd/test_client/main.go
		testClientCode := generateTestClientGo(idl, structMap, enumMap, primaryImportPath, primaryNs, packageImportPath)
		testClientDir := filepath.Join(outputDir, "cmd", "test_client")
		if err := os.MkdirAll(testClientDir, 0755); err != nil {
			return fmt.Errorf("failed to create test_client directory: %w", err)
		}
		testClientPath := filepath.Join(testClientDir, "main.go")
		if err := os.WriteFile(testClientPath, []byte(testClientCode), 0644); err != nil {
			return fmt.Errorf("failed to write test_client/main.go: %w", err)
		}
		PrintFileCreated(testClientPath, fs)
	}

	return nil
}

// findGoMod searches upward from startDir to find go.mod
// Returns (moduleRoot, modulePath, error)
func findGoMod(startDir string) (string, string, error) {
	dir := startDir
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			// Found go.mod
			modulePath, err := parseModulePath(goModPath)
			if err != nil {
				return "", "", fmt.Errorf("failed to parse go.mod at %s: %w", goModPath, err)
			}
			return dir, modulePath, nil
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding go.mod
			return "", "", fmt.Errorf("unable to find go.mod file starting from %s", startDir)
		}
		dir = parent
	}
}

// parseModulePath reads go.mod and extracts module path
func parseModulePath(goModPath string) (string, error) {
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return "", fmt.Errorf("failed to read go.mod: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1], nil
			}
		}
	}

	return "", fmt.Errorf("no module declaration found in go.mod")
}

// calculateImportPath constructs full import path
// Combines module path with relative path to runtime
func calculateImportPath(moduleRoot, modulePath, outputDir string) (string, error) {
	// Get absolute paths
	absRoot, err := filepath.Abs(moduleRoot)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for module root: %w", err)
	}

	absOutput, err := filepath.Abs(outputDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for output dir: %w", err)
	}

	// Find relative path from module root to output dir
	relPath, err := filepath.Rel(absRoot, absOutput)
	if err != nil {
		return "", fmt.Errorf("failed to calculate relative path: %w", err)
	}

	// The runtime is in pulserpc/ relative to output dir's parent
	// Output dir might be like "pkg/checkout", runtime is in "pkg/pulserpc"
	// So from module root, runtime is at "pkg/pulserpc"
	// Or if output dir is module root (relPath == "."), runtime is at "pulserpc"
	var runtimeRelPath string
	if relPath == "." {
		runtimeRelPath = "pulserpc"
	} else {
		runtimeRelPath = filepath.Join(filepath.Dir(relPath), "pulserpc")
	}

	// Construct full import path
	fullImportPath := path.Join(modulePath, runtimeRelPath)
	// Clean path separators to use forward slashes (Go import paths)
	fullImportPath = strings.ReplaceAll(fullImportPath, "\\", "/")

	return fullImportPath, nil
}

// copyRuntimeFiles copies the Go runtime library files to {dir}/pulserpc/
// Uses embedded runtime files from the binary
// Runtime is always placed at {dir}/pulserpc/ regardless of inline-runtime flag
func (p *GoClientServer) copyRuntimeFiles(outputDir string, silent bool) error {
	files, err := runtime.GetRuntimeFiles("go")
	if err != nil {
		return err
	}
	// The embedded runtime tree contains upstream's own *_test.go files. They
	// are not part of the generated package and some of them read files from
	// the PulseRPC source checkout (e.g. the spec-consistency test), so
	// shipping them makes `go test ./...` fail in a fresh generated module.
	// Keep those tests upstream; generated output gets production files only.
	// Remember their names so an idempotent re-generation can evict copies left
	// behind by an older generator.
	upstreamTests := make(map[string]bool)
	for filename := range files {
		if strings.HasSuffix(filename, "_test.go") {
			upstreamTests[filename] = true
			delete(files, filename)
		}
	}

	// Runtime is always at {dir}/pulserpc/
	var runtimeDir string
	if outputDir == "" || outputDir == "." {
		runtimeDir = "pulserpc"
	} else {
		runtimeDir = filepath.Join(outputDir, "pulserpc")
	}

	// Check if runtime files already exist (idempotent for multiple generations)
	if _, err := os.Stat(runtimeDir); err == nil {
		// Directory exists, check if it has the expected files
		for filename := range files {
			dstPath := filepath.Join(runtimeDir, filename)
			if _, err := os.Stat(dstPath); os.IsNotExist(err) {
				// File missing, need to copy
				goto copyFiles
			}
		}
		// All current files exist, but an older generator may have shipped
		// upstream tests into this directory. Evict exactly those known files;
		// never touch other files the user may have added.
		for filename := range upstreamTests {
			stalePath := filepath.Join(runtimeDir, filename)
			if err := os.Remove(stalePath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to remove stale runtime test %s: %w", stalePath, err)
			}
		}
		return nil
	}

copyFiles:
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		return fmt.Errorf("failed to create runtime directory: %w", err)
	}

	for filename, data := range files {
		content := string(data)
		// Keep package name as "pulserpc" (don't rename to packageName)

		dstPath := filepath.Join(runtimeDir, filename)
		if err := os.WriteFile(dstPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write runtime file %s: %w", dstPath, err)
		}
		// Print file path unless silent mode
		if !silent {
			fmt.Println(dstPath)
		}
	}

	return nil
}

// goKeywords are Go reserved words. IDL identifiers and namespaces may use
// these (the IDL lexer has no such restriction), so generated Go identifiers
// must be sanitized before they are emitted.
var goKeywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
	"func": true, "go": true, "goto": true, "if": true, "import": true,
	"interface": true, "map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true, "var": true,
}

// goParamIdent returns a Go identifier for an IDL parameter name. Only keyword
// collisions need renaming; field and method names already go through
// snakeToCamelCase, which cannot produce a keyword.
func goParamIdent(name string) string {
	if goKeywords[name] {
		return name + "_"
	}
	return name
}

// snakeToCamelCase converts snake_case to CamelCase
// Example: "to_repeat" -> "ToRepeat"
func snakeToCamelCase(s string) string {
	parts := strings.Split(s, "_")
	result := ""
	for _, part := range parts {
		if len(part) > 0 {
			result += strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return result
}

// goMethodReturnsValue reports whether an IDL method declares a value return.
// A nil ReturnType and the legacy Type{BuiltIn: "void"} representation (used by
// the runtime contract and the OpenAPI importer) both mean the generated Go
// method returns only error. The runtime's shape validation uses the same rule.
func goMethodReturnsValue(method *parser.Method) bool {
	if method == nil || method.ReturnType == nil {
		return false
	}
	if method.ReturnType.IsBuiltIn() && method.ReturnType.BuiltIn == "void" {
		return false
	}
	return true
}

// mapTypeToGoType maps an IDL type to a Go type string
func mapTypeToGoType(t *parser.Type, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, optional bool) string {
	if t.IsBuiltIn() {
		var goType string
		switch t.BuiltIn {
		case "string":
			goType = "string"
		case "int":
			goType = "int64"
		case "float":
			goType = "float64"
		case "bool":
			goType = "bool"
		default:
			goType = "interface{}"
		}
		if optional {
			return "*" + goType
		}
		return goType
	} else if t.IsArray() {
		elementType := mapTypeToGoType(t.Array, structMap, enumMap, false)
		return "[]" + elementType
	} else if t.IsMap() {
		valueType := mapTypeToGoType(t.MapValue, structMap, enumMap, false)
		return "map[string]" + valueType
	} else if t.IsUserDefined() {
		typeName := getGoStructOrEnumTypeName(t.UserDefined, structMap, enumMap)
		if optional {
			return "*" + typeName
		}
		return typeName
	}
	return "interface{}"
}

// getGoTypeWithNamespace maps an IDL type to a Go type string with namespace prefix for cross-namespace types
// It also tracks which namespaces need to be imported via the nsImports map
func getGoTypeWithNamespace(t *parser.Type, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, optional bool, currentNamespace string, nsImports map[string]string, packageImportPath string) string {
	if t.IsBuiltIn() {
		var goType string
		switch t.BuiltIn {
		case "string":
			goType = "string"
		case "int":
			goType = "int64"
		case "float":
			goType = "float64"
		case "bool":
			goType = "bool"
		default:
			goType = "interface{}"
		}
		if optional {
			return "*" + goType
		}
		return goType
	} else if t.IsArray() {
		elementType := getGoTypeWithNamespace(t.Array, structMap, enumMap, false, currentNamespace, nsImports, packageImportPath)
		return "[]" + elementType
	} else if t.IsMap() {
		valueType := getGoTypeWithNamespace(t.MapValue, structMap, enumMap, false, currentNamespace, nsImports, packageImportPath)
		return "map[string]" + valueType
	} else if t.IsUserDefined() {
		typeNamespace := findNamespaceForType(t.UserDefined, structMap, enumMap)
		baseName := GetBaseName(t.UserDefined)

		// If type is from a different namespace, add import and prefix
		if typeNamespace != "" && typeNamespace != currentNamespace {
			// Add to imports if not already there
			if _, exists := nsImports[typeNamespace]; !exists {
				nsImports[typeNamespace] = packageImportPath + "/" + typeNamespace
			}
			if optional {
				return "*" + typeNamespace + "." + baseName
			}
			return typeNamespace + "." + baseName
		}

		// Same namespace - just return base name
		typeName := getGoStructOrEnumTypeName(t.UserDefined, structMap, enumMap)
		if optional {
			return "*" + typeName
		}
		return typeName
	}
	return "interface{}"
}

// getGoStructOrEnumTypeName returns the Go type name for a user-defined type
func getGoStructOrEnumTypeName(typeName string, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) string {
	baseName := GetBaseName(typeName)

	// Check if it's a struct
	if _, ok := structMap[baseName]; ok {
		return baseName
	}
	if _, ok := structMap[typeName]; ok {
		return baseName
	}

	// Check if it's an enum
	if _, ok := enumMap[baseName]; ok {
		return baseName
	}
	if _, ok := enumMap[typeName]; ok {
		return baseName
	}

	// Fallback: return base name
	return baseName
}

// getReferencedNamespaces returns all namespaces referenced by the given types
func getReferencedNamespaces(types *NamespaceTypes, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, currentNamespace string) map[string]bool {
	referenced := make(map[string]bool)

	for _, s := range types.Structs {
		// Check extends
		if s.Extends != "" {
			if ns := findNamespaceForType(s.Extends, structMap, enumMap); ns != "" && ns != currentNamespace {
				referenced[ns] = true
			}
		}
		// Check field types
		for _, field := range s.Fields {
			findReferencedNamespacesInType(field.Type, structMap, enumMap, currentNamespace, referenced)
		}
	}

	return referenced
}

// findReferencedNamespacesInType recursively finds namespaces referenced by a type
func findReferencedNamespacesInType(t *parser.Type, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, currentNamespace string, referenced map[string]bool) {
	if t.IsUserDefined() {
		if ns := findNamespaceForType(t.UserDefined, structMap, enumMap); ns != "" && ns != currentNamespace {
			referenced[ns] = true
		}
	} else if t.IsArray() {
		findReferencedNamespacesInType(t.Array, structMap, enumMap, currentNamespace, referenced)
	} else if t.IsMap() {
		findReferencedNamespacesInType(t.MapValue, structMap, enumMap, currentNamespace, referenced)
	}
}

// findNamespaceForType finds which namespace a type belongs to.
//
// D16: resolution order is exact key, exact base-name key, then the
// lexicographically smallest key sharing the base name. Map iteration order is
// never used, so generation is deterministic when duplicate base names exist.
// The scan is a single O(N) pass that remembers only the smallest matching key:
// the previous implementation allocated and sorted the whole key set on every
// miss, which is wasteful because only the first match is ever used.
func findNamespaceForType(typeName string, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) string {
	// Check if type is already qualified
	if strings.Contains(typeName, ".") {
		parts := strings.Split(typeName, ".")
		return parts[0]
	}

	// Struct keys take precedence over enum keys. The structMap keys are full
	// type names with namespace.
	if ns, ok := namespaceForMatchingKey(typeName, structMap); ok {
		return ns
	}
	if ns, ok := namespaceForMatchingKey(typeName, enumMap); ok {
		return ns
	}

	return ""
}

// namespaceForMatchingKey returns the namespace of the lexicographically
// smallest qualified key in defs whose base name equals typeName. The second
// result reports whether a matching qualified key exists. Unqualified keys are
// skipped, matching the previous sorted-scan behavior.
//
// This is the single generic replacement for the former per-type
// sortedTypeKeys/sortedEnumKeys helpers: it keeps deterministic ordering
// without allocating a slice or sorting a map-derived key set, and it does not
// allocate at all (no strings.Split) so lookups stay cheap on the miss path.
func namespaceForMatchingKey[T any](typeName string, defs map[string]T) (string, bool) {
	smallest := ""
	found := false
	for key := range defs {
		dot := strings.LastIndexByte(key, '.')
		if dot < 0 || key[dot+1:] != typeName {
			continue
		}
		if found && key >= smallest {
			continue
		}
		smallest = key
		found = true
	}
	if !found {
		return "", false
	}
	return smallest[:strings.IndexByte(smallest, '.')], true
}

type goJSONOptions struct {
	omitEmpty                   bool
	preserveEmptyOptionalArrays bool
}

// generateNamespaceGo generates a Go file for a single namespace.
func generateNamespaceGo(namespace string, types *NamespaceTypes, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, packageImportPath string, jsonOptions goJSONOptions) string {
	// Find referenced namespaces for cross-namespace imports
	referencedNamespaces := getReferencedNamespaces(types, structMap, enumMap, namespace)

	// Track the imports actually required while emitting types. Seeding from
	// getReferencedNamespaces keeps the import block complete even when a type
	// reference is discovered by a path other than the struct generator.
	nsImports := make(map[string]string, len(referencedNamespaces))
	for refNs := range referencedNamespaces {
		nsImports[refNs] = packageImportPath + "/" + refNs
	}

	// Generate enum types first (they may be referenced by structs), then the
	// struct types. This populates nsImports before the import block is written.
	var body strings.Builder
	generateEnumTypesGo(&body, types.Enums)
	body.WriteString("\n")
	generateStructTypesGo(&body, namespace, types.Structs, structMap, enumMap, nsImports, packageImportPath, jsonOptions)
	body.WriteString("\n")

	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString(fmt.Sprintf("package %s\n\n", namespace))

	// Namespace files don't need the pulserpc runtime import; only the
	// shared packages referenced by types are imported.
	sb.WriteString("import (\n")

	// Add imports for referenced namespaces in sorted order (D12).
	refNamespaces := make([]string, 0, len(nsImports))
	for refNs := range nsImports {
		refNamespaces = append(refNamespaces, refNs)
	}
	sort.Strings(refNamespaces)
	for _, refNs := range refNamespaces {
		sb.WriteString(fmt.Sprintf("	\"%s\"\n", nsImports[refNs]))
	}

	sb.WriteString(")\n\n")
	sb.WriteString(body.String())

	return sb.String()
}

// generateEnumTypesGo generates Go enum types for all enums in the namespace
func generateEnumTypesGo(sb *strings.Builder, enums []*parser.Enum) {
	for _, e := range enums {
		if e.Comment != "" {
			lines := strings.Split(strings.TrimSpace(e.Comment), "\n")
			for _, line := range lines {
				fmt.Fprintf(sb, "// %s\n", line)
			}
		}
		enumName := GetBaseName(e.Name)
		fmt.Fprintf(sb, "type %s string\n\n", enumName)
		fmt.Fprintf(sb, "const (\n")
		// Distinct IDL values can collapse to the same Go identifier after
		// snakeToCamelCase (e.g. foo_bar and fooBar). Keep every emitted const
		// name unique with a deterministic numeric suffix; the wire value is
		// still the original IDL value.
		usedConstNames := make(map[string]bool)
		for _, val := range e.Values {
			constName := enumName + snakeToCamelCase(val.Name)
			if usedConstNames[constName] {
				for n := 2; ; n++ {
					candidate := fmt.Sprintf("%s_%d", constName, n)
					if !usedConstNames[candidate] {
						constName = candidate
						break
					}
				}
			}
			usedConstNames[constName] = true
			fmt.Fprintf(sb, "	%s %s = \"%s\"\n", constName, enumName, val.Name)
		}
		sb.WriteString(")\n\n")
	}
}

// generateStructTypesGo generates Go struct types for all structs in the namespace
func generateStructTypesGo(sb *strings.Builder, namespace string, structs []*parser.Struct, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, nsImports map[string]string, packageImportPath string, jsonOptions goJSONOptions) {
	for _, s := range structs {
		if s.Comment != "" {
			lines := strings.Split(strings.TrimSpace(s.Comment), "\n")
			for _, line := range lines {
				fmt.Fprintf(sb, "// %s\n", line)
			}
		}

		structName := GetBaseName(s.Name)
		fmt.Fprintf(sb, "type %s struct {\n", structName)

		// Handle inheritance via embedding
		if s.Extends != "" {
			parentNs := findNamespaceForType(s.Extends, structMap, enumMap)
			parent := &parser.Type{UserDefined: s.Extends}
			parentName := getGoTypeWithNamespace(parent, structMap, enumMap, false, namespace, nsImports, packageImportPath)
			// Ensure the parent's namespace is registered for import even when
			// the helper could not qualify it (e.g. unqualified type name).
			if parentNs != "" && parentNs != namespace {
				if _, exists := nsImports[parentNs]; !exists {
					nsImports[parentNs] = packageImportPath + "/" + parentNs
				}
			}
			fmt.Fprintf(sb, "	%s\n", parentName)
		}

		// Generate fields
		for _, field := range s.Fields {
			if field.Comment != "" {
				lines := strings.Split(strings.TrimSpace(field.Comment), "\n")
				for _, line := range lines {
					fmt.Fprintf(sb, "	// %s\n", line)
				}
			}

			// JSON tag (IDL uses snake_case, Go uses CamelCase)
			fieldName := snakeToCamelCase(field.Name)
			goType := getGoTypeWithNamespace(field.Type, structMap, enumMap, field.Optional, namespace, nsImports, packageImportPath)
			jsonTag := field.Name
			switch {
			case field.Optional && field.Type.Array != nil && jsonOptions.preserveEmptyOptionalArrays:
				jsonTag += ",omitzero"
			case field.Optional || jsonOptions.omitEmpty:
				jsonTag += ",omitempty"
			}
			fmt.Fprintf(sb, "	%s %s `json:\"%s\"`\n", fieldName, goType, jsonTag)
		}

		sb.WriteString("}\n\n")
	}
}

// generateServerGo generates the server.go file with HTTP server and interface stubs.
// idlJSON is the already-marshalled IDL metadata (the exact bytes written to
// idl.json); it is embedded verbatim so the IDL is serialized only once.
func generateServerGo(idl *parser.IDL, idlJSON []byte, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, primaryNs string, namespaceMap map[string]*NamespaceTypes, runtimeImportPath string, packageImportPath string) string {
	_ = namespaceMap // reserved for future use
	var sb strings.Builder

	sb.WriteString("//go:build !client_only\n")
	sb.WriteString("// +build !client_only\n\n")
	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString(fmt.Sprintf("package %s\n\n", primaryNs))

	// Track namespace imports for cross-namespace type references
	nsImports := make(map[string]string)

	// Pre-scan all interfaces to populate nsImports before writing imports
	for _, iface := range idl.Interfaces {
		for _, method := range iface.Methods {
			for _, param := range method.Parameters {
				getGoTypeWithNamespace(param.Type, structMap, enumMap, false, primaryNs, nsImports, packageImportPath)
			}
			if goMethodReturnsValue(method) {
				getGoTypeWithNamespace(method.ReturnType, structMap, enumMap, method.ReturnOptional, primaryNs, nsImports, packageImportPath)
			}
		}
	}

	// Write imports
	sb.WriteString("import (\n")
	sb.WriteString("	\"bytes\"\n")
	sb.WriteString("	\"context\"\n")
	sb.WriteString("	\"encoding/json\"\n")
	sb.WriteString("	\"errors\"\n")
	sb.WriteString("	\"fmt\"\n")
	sb.WriteString("	\"io\"\n")
	sb.WriteString("	\"net/http\"\n")
	sb.WriteString("	\"time\"\n")
	sb.WriteString("	\"unicode/utf8\"\n")

	// Import the shared pulserpc runtime
	sb.WriteString(fmt.Sprintf("	\"%s\"\n", runtimeImportPath))

	// Add cross-namespace imports in sorted order (D12).
	importPaths := make([]string, 0, len(nsImports))
	for _, importPath := range nsImports {
		importPaths = append(importPaths, importPath)
	}
	sort.Strings(importPaths)
	for _, importPath := range importPaths {
		sb.WriteString(fmt.Sprintf("	\"%s\"\n", importPath))
	}

	sb.WriteString(")\n\n")

	// Embed IDL JSON directly in server.go for pulserpc-idl RPC method. These
	// are the exact bytes written to idl.json (D8), so both forms always agree.
	if len(idlJSON) > 0 {
		sb.WriteString("// IDL_JSON contains the IDL definition used to generate this code\n")
		sb.WriteString("const IDL_JSON = ")
		sb.WriteString(escapeGoString(string(idlJSON)))
		sb.WriteString("\n\n")
	}

	// Generate interface stubs
	for _, iface := range idl.Interfaces {
		writeInterfaceStubGo(&sb, iface, structMap, enumMap, primaryNs, nsImports, packageImportPath, idl)
	}

	// Generate PulseRPCServer
	writePulseRPCServerGo(&sb)

	return sb.String()
}

// goInterfaceIdent returns the Go identifier for an IDL interface. Interfaces
// declared in an imported .pulse file carry a namespace qualifier (e.g.
// "alpha.Alpha"); that qualifier is part of the JSON-RPC method name but is not
// a valid Go identifier. Identifier sites use the base name, while method
// strings keep the qualified name so "alpha.Alpha.Ping" stays intact.
func goInterfaceIdent(iface *parser.Interface, idl ...*parser.IDL) string {
	base := GetBaseName(iface.Name)
	if base == "" || len(idl) == 0 || idl[0] == nil {
		return base
	}
	// One generated package hosts interfaces from every namespace. Two
	// imported namespaces can declare the same base name (alpha.Alpha and
	// beta.Alpha); collapsing both to "Alpha" emits duplicate declarations.
	// Disambiguate only the colliding names so all other generated APIs are
	// unchanged.
	collides := false
	for _, other := range idl[0].Interfaces {
		if other != iface && GetBaseName(other.Name) == base {
			collides = true
			break
		}
	}
	if !collides {
		return base
	}
	ns := GetNamespaceFromType(iface.Name, iface.Namespace)
	if ns == "" {
		return base
	}
	return ns + strings.ToUpper(base[:1]) + base[1:]
}

// writeInterfaceStubGo generates a Go interface for an IDL interface
func writeInterfaceStubGo(sb *strings.Builder, iface *parser.Interface, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, namespace string, nsImports map[string]string, packageImportPath string, idl ...*parser.IDL) {
	if iface.Comment != "" {
		lines := strings.Split(strings.TrimSpace(iface.Comment), "\n")
		for _, line := range lines {
			fmt.Fprintf(sb, "// %s\n", line)
		}
	}
	fmt.Fprintf(sb, "type %s interface {\n", goInterfaceIdent(iface, idl...))

	for _, method := range iface.Methods {
		methodName := snakeToCamelCase(method.Name)
		fmt.Fprintf(sb, "	%s(ctx context.Context", methodName)

		// Parameters
		for _, param := range method.Parameters {
			sb.WriteString(", ")
			paramType := getGoTypeWithNamespace(param.Type, structMap, enumMap, false, namespace, nsImports, packageImportPath)
			fmt.Fprintf(sb, "%s %s", goParamIdent(param.Name), paramType)
		}
		sb.WriteString(") ")

		// Return type: always expose the Go-idiomatic error result.
		if goMethodReturnsValue(method) {
			returnType := getGoTypeWithNamespace(method.ReturnType, structMap, enumMap, method.ReturnOptional, namespace, nsImports, packageImportPath)
			fmt.Fprintf(sb, "(%s, error)\n", returnType)
		} else {
			sb.WriteString("error\n")
		}
	}

	sb.WriteString("}\n\n")
}

// writePulseRPCServerGo generates the PulseRPCServer struct and methods
// Refactored to use pulserpc.Server instead of inline implementation
func writePulseRPCServerGo(sb *strings.Builder) {
	sb.WriteString("// HTTPMiddleware runs at the very top of handleRequest, before the method\n")
	sb.WriteString("// and body checks. It can read headers, derive the context used for the\n")
	sb.WriteString("// JSON-RPC call, or write an HTTP response and return false to stop\n")
	sb.WriteString("// processing. Returning a nil context falls back to r.Context().\n")
	sb.WriteString("type HTTPMiddleware func(w http.ResponseWriter, r *http.Request) (context.Context, bool)\n\n")

	sb.WriteString("// PulseRPCServer is an HTTP server for JSON-RPC 2.0 requests\n")
	sb.WriteString("// This implementation uses the pulserpc.Server runtime class\n")
	sb.WriteString("type PulseRPCServer struct {\n")
	sb.WriteString("	host           string\n")
	sb.WriteString("	port           int\n")
	sb.WriteString("	rpcServer      *pulserpc.Server\n")
	sb.WriteString("	server         *http.Server\n")
	sb.WriteString("	httpMiddleware []HTTPMiddleware\n")
	sb.WriteString("}\n\n")

	sb.WriteString("// NewPulseRPCServer creates a new PulseRPCServer with the default policy:\n")
	sb.WriteString("// batches allowed, notifications allowed, both params forms accepted, and\n")
	sb.WriteString("// the legacy 1 MiB body cap.\n")
	sb.WriteString("func NewPulseRPCServer(host string, port int) *PulseRPCServer {\n")
	sb.WriteString("	return NewPulseRPCServerWithConfig(host, port, pulserpc.DefaultServerConfig())\n")
	sb.WriteString("}\n\n")

	sb.WriteString("// NewPulseRPCServerWithConfig creates a new PulseRPCServer with an explicit\n")
	sb.WriteString("// server policy. Start from pulserpc.DefaultServerConfig() and override the\n")
	sb.WriteString("// fields you need; the ServerConfig zero value disables batching.\n")
	sb.WriteString("func NewPulseRPCServerWithConfig(host string, port int, config pulserpc.ServerConfig) *PulseRPCServer {\n")
	sb.WriteString("	// Parse the embedded IDL JSON\n")
	sb.WriteString("	var idlData interface{}\n")
	sb.WriteString("	if err := json.Unmarshal([]byte(IDL_JSON), &idlData); err != nil {\n")
	sb.WriteString("		panic(fmt.Sprintf(\"failed to parse IDL JSON: %v\", err))\n")
	sb.WriteString("	}\n")
	sb.WriteString("	\n")
	sb.WriteString("	// Create Contract from IDL\n")
	sb.WriteString("	contract := pulserpc.NewContract(idlData)\n")
	sb.WriteString("	\n")
	sb.WriteString("	// Create the runtime Server with validation enabled\n")
	sb.WriteString("	rpcServer := pulserpc.NewServerWithConfig(contract, true, true, config)\n")
	sb.WriteString("	\n")
	sb.WriteString("	return &PulseRPCServer{\n")
	sb.WriteString("		host:     host,\n")
	sb.WriteString("		port:     port,\n")
	sb.WriteString("		rpcServer: rpcServer,\n")
	sb.WriteString("	}\n")
	sb.WriteString("}\n\n")

	sb.WriteString("// Register registers an interface implementation. It returns an error when\n")
	sb.WriteString("// a handler method does not accept a leading context.Context or does not\n")
	sb.WriteString("// return the declared error shape.\n")
	sb.WriteString("func (s *PulseRPCServer) Register(interfaceName string, implementation interface{}) error {\n")
	sb.WriteString("	return s.rpcServer.AddHandler(interfaceName, implementation)\n")
	sb.WriteString("}\n\n")

	sb.WriteString("// UseHTTP registers an HTTP middleware hook. Hooks run in registration order\n")
	sb.WriteString("// at the top of handleRequest, before the method and body checks.\n")
	sb.WriteString("func (s *PulseRPCServer) UseHTTP(mw HTTPMiddleware) {\n")
	sb.WriteString("	if mw != nil {\n")
	sb.WriteString("		s.httpMiddleware = append(s.httpMiddleware, mw)\n")
	sb.WriteString("	}\n")
	sb.WriteString("}\n\n")

	sb.WriteString("// UseRPC registers an RPC middleware hook. Hooks run once per JSON-RPC\n")
	sb.WriteString("// request (per batch member), after envelope validation and before\n")
	sb.WriteString("// parameter validation and dispatch.\n")
	sb.WriteString("func (s *PulseRPCServer) UseRPC(mw pulserpc.Middleware) {\n")
	sb.WriteString("	s.rpcServer.UseRPC(mw)\n")
	sb.WriteString("}\n\n")

	sb.WriteString("// Call processes a JSON-RPC request with a background context and returns a response\n")
	sb.WriteString("func (s *PulseRPCServer) Call(request map[string]interface{}) map[string]interface{} {\n")
	sb.WriteString("	return s.rpcServer.Call(request)\n")
	sb.WriteString("}\n\n")

	sb.WriteString("// CallWithContext processes a JSON-RPC request with the supplied context\n")
	sb.WriteString("func (s *PulseRPCServer) CallWithContext(ctx context.Context, request map[string]interface{}) map[string]interface{} {\n")
	sb.WriteString("	return s.rpcServer.CallWithContext(ctx, request)\n")
	sb.WriteString("}\n\n")

	sb.WriteString("// ServeForever starts the HTTP server and serves forever\n")
	sb.WriteString("func (s *PulseRPCServer) ServeForever() error {\n")
	sb.WriteString("	mux := http.NewServeMux()\n")
	sb.WriteString("	mux.HandleFunc(\"/\", s.handleRequest)\n")
	sb.WriteString("	addr := fmt.Sprintf(\"%s:%d\", s.host, s.port)\n")
	sb.WriteString("	s.server = &http.Server{\n")
	sb.WriteString("		Addr:    addr,\n")
	sb.WriteString("		Handler: mux,\n")
	sb.WriteString("		// D6/R19: bound slow, idle, and header-flooding clients.\n")
	sb.WriteString("		ReadHeaderTimeout: 10 * time.Second,\n")
	sb.WriteString("		ReadTimeout:       30 * time.Second,\n")
	sb.WriteString("		WriteTimeout:      30 * time.Second,\n")
	sb.WriteString("		IdleTimeout:       60 * time.Second,\n")
	sb.WriteString("		MaxHeaderBytes:    1 << 20, // 1 MiB\n")
	sb.WriteString("	}\n")
	sb.WriteString("	fmt.Printf(\"PulseRPC server listening on http://%s\\n\", addr)\n")
	sb.WriteString("	return s.server.ListenAndServe()\n")
	sb.WriteString("}\n\n")

	sb.WriteString("// ServeHTTP exposes the JSON-RPC adapter as an http.Handler.\n")
	sb.WriteString("func (s *PulseRPCServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {\n")
	sb.WriteString("	s.handleRequest(w, r)\n")
	sb.WriteString("}\n\n")

	// Generate handleRequest method that uses pulserpc.Server
	writeServerHandleRequestGoRefactored(sb)
}

// writeServerHandleRequestGoRefactored generates the handleRequest method using pulserpc.Server
func writeServerHandleRequestGoRefactored(sb *strings.Builder) {
	sb.WriteString("// D5: request and batch limits enforced by the generated adapter.\n")
	sb.WriteString("const maxRequestBodySize = 1 << 20 // 1 MiB\n")
	sb.WriteString("const maxBatchSize = 100\n")
	sb.WriteString("const maxBatchResponseSize = 10 * 1024 * 1024 // 10 MiB\n\n")

	sb.WriteString("// handleRequest handles HTTP requests by delegating to pulserpc.Server.CallWithContext()\n")
	sb.WriteString("func (s *PulseRPCServer) handleRequest(w http.ResponseWriter, r *http.Request) {\n")
	sb.WriteString("	config := s.rpcServer.Config()\n\n")
	sb.WriteString("	// Bound the body before middleware so a hook that inspects the payload\n")
	sb.WriteString("	// cannot buffer an unbounded request. The method check still runs after\n")
	sb.WriteString("	// middleware so authentication can reject before parsing.\n")
	sb.WriteString("	bodyLimit := int64(maxRequestBodySize)\n")
	sb.WriteString("	if config.MaxRequestBodyBytes > 0 {\n")
	sb.WriteString("		bodyLimit = config.MaxRequestBodyBytes\n")
	sb.WriteString("	}\n")
	sb.WriteString("	r.Body = http.MaxBytesReader(w, r.Body, bodyLimit)\n\n")
	sb.WriteString("	// HTTP middleware runs first, before the method check and body read. A\n")
	sb.WriteString("	// middleware that writes a response and returns false owns the response;\n")
	sb.WriteString("	// the adapter writes nothing further.\n")
	sb.WriteString("	ctx := r.Context()\n")
	sb.WriteString("	for _, mw := range s.httpMiddleware {\n")
	sb.WriteString("		nextCtx, cont := mw(w, r.WithContext(ctx))\n")
	sb.WriteString("		if nextCtx != nil {\n")
	sb.WriteString("			ctx = nextCtx\n")
	sb.WriteString("		}\n")
	sb.WriteString("		if !cont {\n")
	sb.WriteString("			return\n")
	sb.WriteString("		}\n")
	sb.WriteString("	}\n\n")

	sb.WriteString("	if r.Method != http.MethodPost {\n")
	sb.WriteString("		// D3: non-POST requests return HTTP 405 with a JSON-RPC error body.\n")
	sb.WriteString("		w.Header().Set(\"Content-Type\", \"application/json\")\n")
	sb.WriteString("		w.WriteHeader(http.StatusMethodNotAllowed)\n")
	sb.WriteString("		json.NewEncoder(w).Encode(errorResponse(nil, -32600, \"Invalid Request - only POST allowed\", nil))\n")
	sb.WriteString("		return\n")
	sb.WriteString("	}\n\n")

	sb.WriteString("	body, err := io.ReadAll(r.Body)\n")
	sb.WriteString("	if err != nil {\n")
	sb.WriteString("		var maxBytesErr *http.MaxBytesError\n")
	sb.WriteString("		if errors.As(err, &maxBytesErr) {\n")
	sb.WriteString("			if config.MaxRequestBodyBytes > 0 {\n")
	sb.WriteString("				s.sendErrorResponse(w, nil, -32700, \"Parse error\", nil)\n")
	sb.WriteString("			} else {\n")
	sb.WriteString("				s.sendErrorResponse(w, nil, -32600, \"Request body too large\", nil)\n")
	sb.WriteString("			}\n")
	sb.WriteString("			return\n")
	sb.WriteString("		}\n")
	sb.WriteString("		s.sendErrorResponse(w, nil, -32700, \"Parse error\", nil)\n")
	sb.WriteString("		return\n")
	sb.WriteString("	}\n\n")

	sb.WriteString("	if config.RejectInvalidUTF8 && !utf8.Valid(body) {\n")
	sb.WriteString("		s.sendErrorResponse(w, nil, -32700, \"Parse error\", nil)\n")
	sb.WriteString("		return\n")
	sb.WriteString("	}\n\n")

	sb.WriteString("	// UseNumber keeps numeric ids exact: a float64 round-trip corrupts\n")
	sb.WriteString("	// integers above 2^53. A token after the value (trailing data) is a\n")
	sb.WriteString("	// parse error. Reading one token avoids materializing a second value.\n")
	sb.WriteString("	var requestData interface{}\n")
	sb.WriteString("	decoder := json.NewDecoder(bytes.NewReader(body))\n")
	sb.WriteString("	decoder.UseNumber()\n")
	sb.WriteString("	if err := decoder.Decode(&requestData); err != nil {\n")
	sb.WriteString("		s.sendErrorResponse(w, nil, -32700, \"Parse error\", nil)\n")
	sb.WriteString("		return\n")
	sb.WriteString("	}\n")
	sb.WriteString("	if _, err := decoder.Token(); err != io.EOF {\n")
	sb.WriteString("		s.sendErrorResponse(w, nil, -32700, \"Parse error\", nil)\n")
	sb.WriteString("		return\n")
	sb.WriteString("	}\n\n")
	sb.WriteString("	// Reject a disallowed batch before walking its members.\n")
	sb.WriteString("	if _, isBatch := requestData.([]interface{}); isBatch && !config.AllowBatch {\n")
	sb.WriteString("		s.sendErrorResponse(w, nil, -32600, \"Invalid Request\", nil)\n")
	sb.WriteString("		return\n")
	sb.WriteString("	}\n\n")
	sb.WriteString("	// Keep params and other envelope values in their v0.4.0 float64 form for\n")
	sb.WriteString("	// middleware and handlers, but leave the request id as json.Number so it\n")
	sb.WriteString("	// can be echoed without losing precision.\n")
	sb.WriteString("	pulserpc.NormalizeRequestNumbers(requestData)\n\n")

	sb.WriteString("	// Handle batch requests\n")
	sb.WriteString("	if requests, ok := requestData.([]interface{}); ok {\n")
	sb.WriteString("		if len(requests) == 0 {\n")
	sb.WriteString("			s.sendErrorResponse(w, nil, -32600, \"Invalid Request\", nil)\n")
	sb.WriteString("			return\n")
	sb.WriteString("		}\n")
	sb.WriteString("		if len(requests) > maxBatchSize {\n")
	sb.WriteString("			s.sendErrorResponse(w, nil, -32600, \"Batch exceeds maximum size\", nil)\n")
	sb.WriteString("			return\n")
	sb.WriteString("		}\n")
	sb.WriteString("		var responses []interface{}\n")
	sb.WriteString("		for _, req := range requests {\n")
	sb.WriteString("			reqMap, ok := req.(map[string]interface{})\n")
	sb.WriteString("			if !ok {\n")
	sb.WriteString("				// M19: non-object entries get a per-entry -32600 and the\n")
	sb.WriteString("				// overall response keeps the batch array shape.\n")
	sb.WriteString("				responses = append(responses, errorResponse(nil, -32600, \"Invalid Request\", nil))\n")
	sb.WriteString("				continue\n")
	sb.WriteString("			}\n")
	sb.WriteString("			// Each batch member gets a child of the HTTP-derived context so\n")
	sb.WriteString("			// one member cannot cancel its siblings.\n")
	sb.WriteString("			memberCtx, cancel := context.WithCancel(ctx)\n")
	sb.WriteString("			resp := s.rpcServer.CallWithContext(memberCtx, reqMap)\n")
	sb.WriteString("			cancel()\n")
	sb.WriteString("			if resp != nil {\n")
	sb.WriteString("				responses = append(responses, resp)\n")
	sb.WriteString("			}\n")
	sb.WriteString("		}\n")
	sb.WriteString("		if len(responses) == 0 {\n")
	sb.WriteString("			w.WriteHeader(http.StatusNoContent)\n")
	sb.WriteString("			return\n")
	sb.WriteString("		}\n")
	sb.WriteString("		// Batch responses concatenate up to maxBatchSize member results, so\n")
	sb.WriteString("		// the serialized array needs its own size bound.\n")
	sb.WriteString("		payload, err := json.Marshal(responses)\n")
	sb.WriteString("		if err != nil {\n")
	sb.WriteString("			s.sendErrorResponse(w, nil, -32603, \"Internal error\", nil)\n")
	sb.WriteString("			return\n")
	sb.WriteString("		}\n")
	sb.WriteString("		if len(payload) > maxBatchResponseSize {\n")
	sb.WriteString("			s.sendErrorResponse(w, nil, -32600, \"Batch response too large\", nil)\n")
	sb.WriteString("			return\n")
	sb.WriteString("		}\n")
	sb.WriteString("		w.Header().Set(\"Content-Type\", \"application/json\")\n")
	sb.WriteString("		w.Write(payload)\n")
	sb.WriteString("		return\n")
	sb.WriteString("	}\n\n")

	sb.WriteString("	// Handle single request using pulserpc.Server.CallWithContext()\n")
	sb.WriteString("	if reqMap, ok := requestData.(map[string]interface{}); ok {\n")
	sb.WriteString("		response := s.rpcServer.CallWithContext(ctx, reqMap)\n")
	sb.WriteString("		if response == nil {\n")
	sb.WriteString("			// Notification - no response expected\n")
	sb.WriteString("			w.WriteHeader(http.StatusNoContent)\n")
	sb.WriteString("			return\n")
	sb.WriteString("		}\n")
	sb.WriteString("		w.Header().Set(\"Content-Type\", \"application/json\")\n")
	sb.WriteString("		json.NewEncoder(w).Encode(response)\n")
	sb.WriteString("	} else {\n")
	sb.WriteString("		s.sendErrorResponse(w, nil, -32600, \"Invalid Request\", nil)\n")
	sb.WriteString("	}\n")
	sb.WriteString("}\n\n")

	sb.WriteString("// errorResponse builds a JSON-RPC error object. An unknown id is emitted\n")
	sb.WriteString("// as null per the JSON-RPC 2.0 specification.\n")
	sb.WriteString("func errorResponse(requestID interface{}, code int, message string, data interface{}) map[string]interface{} {\n")
	sb.WriteString("	errObj := map[string]interface{}{\n")
	sb.WriteString("		\"code\":    code,\n")
	sb.WriteString("		\"message\": message,\n")
	sb.WriteString("	}\n")
	sb.WriteString("	if data != nil {\n")
	sb.WriteString("		errObj[\"data\"] = data\n")
	sb.WriteString("	}\n")
	sb.WriteString("	return map[string]interface{}{\n")
	sb.WriteString("		\"jsonrpc\": \"2.0\",\n")
	sb.WriteString("		\"error\":   errObj,\n")
	sb.WriteString("		\"id\":      requestID,\n")
	sb.WriteString("	}\n")
	sb.WriteString("}\n\n")

	sb.WriteString("// sendErrorResponse sends a JSON-RPC error response\n")
	sb.WriteString("func (s *PulseRPCServer) sendErrorResponse(w http.ResponseWriter, requestID interface{}, code int, message string, data interface{}) {\n")
	sb.WriteString("	w.Header().Set(\"Content-Type\", \"application/json\")\n")
	sb.WriteString("	json.NewEncoder(w).Encode(errorResponse(requestID, code, message, data))\n")
	sb.WriteString("}\n")
}

// generateClientGo generates the client.go file with transport abstraction and client classes
func generateClientGo(idl *parser.IDL, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, primaryNs string, namespaceMap map[string]*NamespaceTypes, runtimeImportPath string, packageImportPath string) string {
	_ = namespaceMap // reserved for future use
	var sb strings.Builder

	sb.WriteString("//go:build !server_only\n")
	sb.WriteString("// +build !server_only\n\n")
	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString(fmt.Sprintf("package %s\n\n", primaryNs))

	// Track namespace imports for cross-namespace type references
	nsImports := make(map[string]string)

	// Pre-scan all interfaces to populate nsImports before writing imports
	for _, iface := range idl.Interfaces {
		for _, method := range iface.Methods {
			for _, param := range method.Parameters {
				getGoTypeWithNamespace(param.Type, structMap, enumMap, false, primaryNs, nsImports, packageImportPath)
			}
			if goMethodReturnsValue(method) {
				getGoTypeWithNamespace(method.ReturnType, structMap, enumMap, method.ReturnOptional, primaryNs, nsImports, packageImportPath)
			}
		}
	}

	sb.WriteString("import (\n")
	sb.WriteString("	\"bytes\"\n")
	sb.WriteString("	\"context\"\n")
	sb.WriteString("	\"encoding/json\"\n")
	sb.WriteString("	\"fmt\"\n")
	sb.WriteString("	\"io\"\n")
	sb.WriteString("	\"net/http\"\n")
	sb.WriteString("	\"strings\"\n")
	sb.WriteString("	\"sync\"\n")
	sb.WriteString("	\"sync/atomic\"\n")
	sb.WriteString("	\"time\"\n")

	// Import the shared pulserpc runtime
	sb.WriteString(fmt.Sprintf("	\"%s\"\n", runtimeImportPath))

	// Add cross-namespace imports in sorted order (D12).
	importPaths := make([]string, 0, len(nsImports))
	for _, importPath := range nsImports {
		importPaths = append(importPaths, importPath)
	}
	sort.Strings(importPaths)
	for _, importPath := range importPaths {
		sb.WriteString(fmt.Sprintf("	\"%s\"\n", importPath))
	}

	sb.WriteString(")\n\n")

	// Generate Transport interface
	writeTransportInterfaceGo(&sb)

	// Generate HTTPTransport
	writeHTTPTransportGo(&sb)

	// Generate shared client options interface and helper functions
	writeClientOptions(&sb)

	// Generate client classes for each interface
	for _, iface := range idl.Interfaces {
		writeInterfaceClientGo(&sb, iface, structMap, enumMap, primaryNs, nsImports, packageImportPath, idl)
	}

	return sb.String()
}

// writeClientOptions generates the shared ClientOption interface and helper functions
func writeClientOptions(sb *strings.Builder) {
	sb.WriteString("// ClientOption is a functional option for configuring a client\n")
	sb.WriteString("type ClientOption func(ClientConfigurator)\n\n")
	sb.WriteString("// ClientConfigurator is implemented by all generated client types\n")
	sb.WriteString("type ClientConfigurator interface {\n")
	sb.WriteString("	SetValidateRequests(bool)\n")
	sb.WriteString("	SetValidateResponses(bool)\n")
	sb.WriteString("	SetContract(*pulserpc.Contract)\n")
	sb.WriteString("	SetAuditor(pulserpc.ContractAuditor)\n")
	sb.WriteString("	SetVerifyOnBootstrap(bool)\n")
	sb.WriteString("}\n\n")
	sb.WriteString("// WithValidateRequests enables request validation\n")
	sb.WriteString("func WithValidateRequests(v bool) ClientOption {\n")
	sb.WriteString("	return func(c ClientConfigurator) {\n")
	sb.WriteString("		c.SetValidateRequests(v)\n")
	sb.WriteString("	}\n")
	sb.WriteString("}\n\n")
	sb.WriteString("// WithValidateResponses enables response validation\n")
	sb.WriteString("func WithValidateResponses(v bool) ClientOption {\n")
	sb.WriteString("	return func(c ClientConfigurator) {\n")
	sb.WriteString("		c.SetValidateResponses(v)\n")
	sb.WriteString("	}\n")
	sb.WriteString("}\n\n")
	sb.WriteString("// WithContract sets a custom contract (e.g., loaded from a different path)\n")
	sb.WriteString("func WithContract(contract *pulserpc.Contract) ClientOption {\n")
	sb.WriteString("	return func(c ClientConfigurator) {\n")
	sb.WriteString("		c.SetContract(contract)\n")
	sb.WriteString("	}\n")
	sb.WriteString("}\n\n")
	sb.WriteString("// WithAuditor sets a contract auditor for compatibility verification\n")
	sb.WriteString("func WithAuditor(auditor pulserpc.ContractAuditor) ClientOption {\n")
	sb.WriteString("	return func(c ClientConfigurator) {\n")
	sb.WriteString("		c.SetAuditor(auditor)\n")
	sb.WriteString("	}\n")
	sb.WriteString("}\n\n")
	sb.WriteString("// VerifyOnBootstrap verifies the contract before the first RPC call.\n")
	sb.WriteString("func VerifyOnBootstrap() ClientOption {\n")
	sb.WriteString("	return func(c ClientConfigurator) {\n")
	sb.WriteString("		c.SetVerifyOnBootstrap(true)\n")
	sb.WriteString("	}\n")
	sb.WriteString("}\n\n")
}

// writeTransportInterfaceGo generates the Transport interface
func writeTransportInterfaceGo(sb *strings.Builder) {
	sb.WriteString("// Transport is an interface for making JSON-RPC 2.0 calls\n")
	sb.WriteString("type Transport interface {\n")
	sb.WriteString("	Call(ctx context.Context, method string, params []interface{}) (map[string]interface{}, error)\n")
	sb.WriteString("}\n\n")
	sb.WriteString("// NotifyTransport is an optional capability for sending notifications.\n")
	sb.WriteString("// A notification is a request without an id and receives no response.\n")
	sb.WriteString("type NotifyTransport interface {\n")
	sb.WriteString("	Notify(ctx context.Context, method string, params []interface{}) error\n")
	sb.WriteString("}\n\n")
}

// writeHTTPTransportGo generates the HTTPTransport struct
func writeHTTPTransportGo(sb *strings.Builder) {
	// Bound response bodies so a malicious or misbehaving server cannot make the
	// client allocate an unbounded amount of memory.
	sb.WriteString("// maxResponseSize is the maximum number of bytes read from an HTTP response body.\n")
	sb.WriteString("const maxResponseSize = 10 * 1024 * 1024 // 10 MiB\n\n")

	sb.WriteString("// HTTPTransport implements Transport using HTTP\n")
	sb.WriteString("type HTTPTransport struct {\n")
	sb.WriteString("	baseURL   string\n")
	sb.WriteString("	headers   map[string]string\n")
	sb.WriteString("	client    *http.Client\n")
	sb.WriteString("	requestID uint64\n")
	sb.WriteString("}\n\n")

	sb.WriteString("// NewHTTPTransport creates a new HTTPTransport\n")
	sb.WriteString("func NewHTTPTransport(baseURL string, headers map[string]string) *HTTPTransport {\n")
	sb.WriteString("	if headers == nil {\n")
	sb.WriteString("		headers = make(map[string]string)\n")
	sb.WriteString("	}\n")
	sb.WriteString("	return &HTTPTransport{\n")
	sb.WriteString("		baseURL: strings.TrimSuffix(baseURL, \"/\"),\n")
	sb.WriteString("		headers: headers,\n")
	sb.WriteString("		client:  &http.Client{Timeout: 30 * time.Second},\n")
	sb.WriteString("	}\n")
	sb.WriteString("}\n\n")

	sb.WriteString("// Call performs a JSON-RPC 2.0 call over HTTP with a unique id.\n")
	sb.WriteString("func (t *HTTPTransport) Call(ctx context.Context, method string, params []interface{}) (map[string]interface{}, error) {\n")
	sb.WriteString("	return t.call(ctx, method, params, fmt.Sprintf(\"%d\", atomic.AddUint64(&t.requestID, 1)))\n")
	sb.WriteString("}\n\n")

	sb.WriteString("// Notify sends a notification over HTTP: no id member, no response expected.\n")
	sb.WriteString("func (t *HTTPTransport) Notify(ctx context.Context, method string, params []interface{}) error {\n")
	sb.WriteString("	_, err := t.call(ctx, method, params, nil)\n")
	sb.WriteString("	return err\n")
	sb.WriteString("}\n\n")

	sb.WriteString("// call builds and sends one JSON-RPC request. A nil id omits the id member.\n")
	sb.WriteString("func (t *HTTPTransport) call(ctx context.Context, method string, params []interface{}, id interface{}) (map[string]interface{}, error) {\n")
	sb.WriteString("	if ctx == nil {\n")
	sb.WriteString("		ctx = context.Background()\n")
	sb.WriteString("	}\n")
	sb.WriteString("	request := map[string]interface{}{\n")
	sb.WriteString("		\"jsonrpc\": \"2.0\",\n")
	sb.WriteString("		\"method\":  method,\n")
	sb.WriteString("		\"params\":  params,\n")
	sb.WriteString("	}\n")
	sb.WriteString("	if id != nil {\n")
	sb.WriteString("		request[\"id\"] = id\n")
	sb.WriteString("	}\n\n")

	sb.WriteString("	jsonData, err := json.Marshal(request)\n")
	sb.WriteString("	if err != nil {\n")
	sb.WriteString("		return nil, fmt.Errorf(\"failed to marshal request: %w\", err)\n")
	sb.WriteString("	}\n\n")

	sb.WriteString("	req, err := http.NewRequestWithContext(ctx, \"POST\", t.baseURL, bytes.NewBuffer(jsonData))\n")
	sb.WriteString("	if err != nil {\n")
	sb.WriteString("		return nil, fmt.Errorf(\"failed to create request: %w\", err)\n")
	sb.WriteString("	}\n\n")

	sb.WriteString("	req.Header.Set(\"Content-Type\", \"application/json\")\n")
	sb.WriteString("	for k, v := range t.headers {\n")
	sb.WriteString("		req.Header.Set(k, v)\n")
	sb.WriteString("	}\n\n")

	sb.WriteString("	resp, err := t.client.Do(req)\n")
	sb.WriteString("	if err != nil {\n")
	sb.WriteString("		return nil, fmt.Errorf(\"HTTP request failed: %w\", err)\n")
	sb.WriteString("	}\n")
	sb.WriteString("	defer resp.Body.Close()\n\n")

	sb.WriteString("	// Read one byte past the limit so oversized bodies are detectable.\n")
	sb.WriteString("	jsonDataResp, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))\n")
	sb.WriteString("	if err != nil {\n")
	sb.WriteString("		return nil, fmt.Errorf(\"failed to read response: %w\", err)\n")
	sb.WriteString("	}\n")
	sb.WriteString("	if len(jsonDataResp) > maxResponseSize {\n")
	sb.WriteString("		return nil, fmt.Errorf(\"failed to decode response: response exceeds %d bytes\", maxResponseSize)\n")
	sb.WriteString("	}\n")
	sb.WriteString("	if len(jsonDataResp) == 0 {\n")
	sb.WriteString("		if id == nil {\n")
	sb.WriteString("			return map[string]interface{}{}, nil\n")
	sb.WriteString("		}\n")
	sb.WriteString("		return nil, fmt.Errorf(\"empty response\")\n")
	sb.WriteString("	}\n\n")

	sb.WriteString("	// DecodeResponse keeps the response id exact (json.Number) and normalizes\n")
	sb.WriteString("	// other numbers to float64, preserving the v0.4.0 client behavior.\n")
	sb.WriteString("	response, err := pulserpc.DecodeResponse(jsonDataResp)\n")
	sb.WriteString("	if err != nil {\n")
	sb.WriteString("		return nil, fmt.Errorf(\"failed to decode response: %w\", err)\n")
	sb.WriteString("	}\n\n")

	sb.WriteString("	if err := pulserpc.ValidateResponseEnvelope(response, id); err != nil {\n")
	sb.WriteString("		return nil, err\n")
	sb.WriteString("	}\n\n")

	sb.WriteString("	if errObj, ok := response[\"error\"].(map[string]interface{}); ok {\n")
	sb.WriteString("		code, _ := errObj[\"code\"].(float64)\n")
	sb.WriteString("		message, _ := errObj[\"message\"].(string)\n")
	sb.WriteString("		data := errObj[\"data\"]\n")
	sb.WriteString("		return nil, &pulserpc.RPCError{\n")
	sb.WriteString("			Code:    int(code),\n")
	sb.WriteString("			Message: message,\n")
	sb.WriteString("			Data:    data,\n")
	sb.WriteString("		}\n")
	sb.WriteString("	}\n\n")

	sb.WriteString("	return response, nil\n")
	sb.WriteString("}\n\n")
}

// writeInterfaceClientGo generates a client struct for an interface
func writeInterfaceClientGo(sb *strings.Builder, iface *parser.Interface, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, namespace string, nsImports map[string]string, packageImportPath string, idl ...*parser.IDL) {
	if iface.Comment != "" {
		lines := strings.Split(strings.TrimSpace(iface.Comment), "\n")
		for _, line := range lines {
			fmt.Fprintf(sb, "// %s\n", line)
		}
	}

	clientName := goInterfaceIdent(iface, idl...) + "Client"
	fmt.Fprintf(sb, "// %s is a client for the %s interface\n", clientName, iface.Name)
	fmt.Fprintf(sb, "type %s struct {\n", clientName)
	sb.WriteString("	transport         Transport\n")
	sb.WriteString("	contract         *pulserpc.Contract\n")
	sb.WriteString("	localIDL         interface{}\n")
	sb.WriteString("	serverIDL        interface{}\n")
	sb.WriteString("	validateRequests  bool\n")
	sb.WriteString("	validateResponses bool\n")
	sb.WriteString("	auditor          pulserpc.ContractAuditor\n")
	sb.WriteString("	verifyOnBootstrap bool\n")
	sb.WriteString("	verifyOnce       sync.Once\n")
	sb.WriteString("}\n\n")

	fmt.Fprintf(sb, "// New%s creates a new %s\n", clientName, clientName)
	fmt.Fprintf(sb, "func New%s(transport Transport, opts ...ClientOption) *%s {\n", clientName, clientName)
	sb.WriteString("	c := &" + clientName + "{\n")
	sb.WriteString("		transport:         transport,\n")
	sb.WriteString("		validateRequests:  false,\n")
	sb.WriteString("		validateResponses: false,\n")
	sb.WriteString("	}\n")
	sb.WriteString("	for _, opt := range opts {\n")
	sb.WriteString("		opt(c)\n")
	sb.WriteString("	}\n")
	sb.WriteString("	// Load contract from idl.json for compatibility verification\n")
	sb.WriteString("	if c.contract == nil {\n")
	sb.WriteString("		c.contract, _ = pulserpc.LoadContractFromFile(\"idl.json\")\n")
	sb.WriteString("	}\n")
	sb.WriteString("	return c\n")
	sb.WriteString("}\n\n")

	fmt.Fprintf(sb, "func (c *%s) SetValidateRequests(v bool) {\n", clientName)
	sb.WriteString("	c.validateRequests = v\n")
	sb.WriteString("}\n\n")

	fmt.Fprintf(sb, "func (c *%s) SetValidateResponses(v bool) {\n", clientName)
	sb.WriteString("	c.validateResponses = v\n")
	sb.WriteString("}\n\n")

	fmt.Fprintf(sb, "func (c *%s) SetContract(contract *pulserpc.Contract) {\n", clientName)
	sb.WriteString("	c.contract = contract\n")
	sb.WriteString("}\n\n")

	sb.WriteString("// SetServerIDL injects the server's IDL (fetched at bootstrap via the pulserpc-idl RPC).\n")
	fmt.Fprintf(sb, "func (c *%s) SetServerIDL(idl interface{}) {\n", clientName)
	sb.WriteString("	c.serverIDL = idl\n")
	sb.WriteString("}\n\n")

	fmt.Fprintf(sb, "func (c *%s) SetAuditor(auditor pulserpc.ContractAuditor) {\n", clientName)
	sb.WriteString("	c.auditor = auditor\n")
	sb.WriteString("}\n\n")

	fmt.Fprintf(sb, "func (c *%s) SetVerifyOnBootstrap(v bool) {\n", clientName)
	sb.WriteString("	c.verifyOnBootstrap = v\n")
	sb.WriteString("}\n\n")

	// Generate VerifyCompatibility method
	writeVerifyCompatibilityMethod(sb, clientName)

	// Verification runs once, before the first RPC, when VerifyOnBootstrap() is
	// set. The generated client has no network bootstrap in its constructor, so
	// the first call is the earliest safe point to fetch the server IDL.
	fmt.Fprintf(sb, "// ensureBootstrapVerification verifies the contract once before the first RPC\n")
	fmt.Fprintf(sb, "// call when VerifyOnBootstrap() was supplied. Values from the caller context\n")
	fmt.Fprintf(sb, "// are preserved, but its cancellation is not: a cancelled first call must not\n")
	fmt.Fprintf(sb, "// permanently consume the one-shot integrity check.\n")
	fmt.Fprintf(sb, "func (c *%s) ensureBootstrapVerification(ctx context.Context) {\n", clientName)
	sb.WriteString("	if !c.verifyOnBootstrap {\n")
	sb.WriteString("		return\n")
	sb.WriteString("	}\n")
	sb.WriteString("	if ctx == nil {\n")
	sb.WriteString("		ctx = context.Background()\n")
	sb.WriteString("	}\n")
	sb.WriteString("	c.verifyOnce.Do(func() {\n")
	sb.WriteString("		c.VerifyCompatibility(context.WithoutCancel(ctx))\n")
	sb.WriteString("	})\n")
	sb.WriteString("}\n\n")

	// Notify sends a request without an id through transports that support it.
	fmt.Fprintf(sb, "// Notify sends a JSON-RPC notification to %s.\n", iface.Name)
	fmt.Fprintf(sb, "func (c *%s) Notify(ctx context.Context, method string, params []interface{}) error {\n", clientName)
	sb.WriteString("	if transport, ok := c.transport.(NotifyTransport); ok {\n")
	sb.WriteString("		return transport.Notify(ctx, method, params)\n")
	sb.WriteString("	}\n")
	sb.WriteString("	return fmt.Errorf(\"transport does not support notifications\")\n")
	sb.WriteString("}\n\n")

	// Generate methods
	for _, method := range iface.Methods {
		writeClientMethodGo(sb, iface, method, structMap, enumMap, namespace, nsImports, packageImportPath, idl...)
	}
}

// writeClientMethodGo generates a method implementation for a Go client struct
func writeClientMethodGo(sb *strings.Builder, iface *parser.Interface, method *parser.Method, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, namespace string, nsImports map[string]string, packageImportPath string, idl ...*parser.IDL) {
	methodName := snakeToCamelCase(method.Name)
	fmt.Fprintf(sb, "// %s calls %s.%s\n", methodName, iface.Name, method.Name)
	fmt.Fprintf(sb, "func (c *%sClient) %s(ctx context.Context", goInterfaceIdent(iface, idl...), methodName)

	// Parameters
	for _, param := range method.Parameters {
		sb.WriteString(", ")
		paramType := getGoTypeWithNamespace(param.Type, structMap, enumMap, false, namespace, nsImports, packageImportPath)
		fmt.Fprintf(sb, "%s %s", goParamIdent(param.Name), paramType)
	}
	sb.WriteString(") ")

	// Return type
	if goMethodReturnsValue(method) {
		returnType := getGoTypeWithNamespace(method.ReturnType, structMap, enumMap, method.ReturnOptional, namespace, nsImports, packageImportPath)
		fmt.Fprintf(sb, "(%s, error)", returnType)
	} else {
		sb.WriteString("error")
	}
	sb.WriteString(" {\n")
	sb.WriteString("\tif ctx == nil {\n\t\tctx = context.Background()\n\t}\n")

	goReturnType := ""
	if goMethodReturnsValue(method) {
		goReturnType = getGoTypeWithNamespace(method.ReturnType, structMap, enumMap, method.ReturnOptional, namespace, nsImports, packageImportPath)
	}

	// A small helper keeps the generated error returns consistent for methods
	// that do and do not declare a return value.
	writeErrorReturn := func(errExpr string) {
		if !goMethodReturnsValue(method) {
			fmt.Fprintf(sb, "\t\treturn %s\n", errExpr)
			return
		}
		fmt.Fprintf(sb, "\t\tvar zero %s\n", goReturnType)
		fmt.Fprintf(sb, "\t\treturn zero, %s\n", errExpr)
	}

	// Build params array
	sb.WriteString("\tparams := []interface{}{\n")
	for _, param := range method.Parameters {
		fmt.Fprintf(sb, "\t\t%s,\n", goParamIdent(param.Name))
	}
	sb.WriteString("\t}\n\n")

	// Verify the contract once before the first RPC when VerifyOnBootstrap() is set.
	sb.WriteString("\tc.ensureBootstrapVerification(ctx)\n\n")

	// The generated validation flags must actually gate IDL validation.
	fmt.Fprintf(sb, "\tif c.validateRequests && c.contract != nil {\n")
	fmt.Fprintf(sb, "\t\tif err := c.contract.ValidateRequest(%q, %q, params); err != nil {\n", iface.Name, method.Name)
	writeErrorReturn("fmt.Errorf(\"request validation failed: %w\", err)")
	sb.WriteString("\t\t}\n\t}\n\n")

	// Call transport
	fmt.Fprintf(sb, "\tmethodName := \"%s.%s\"\n", iface.Name, method.Name)
	sb.WriteString("\tresponse, err := c.transport.Call(ctx, methodName, params)\n")
	sb.WriteString("\tif err != nil {\n")
	writeErrorReturn("err")
	sb.WriteString("\t}\n\n")

	// Extract result
	if goMethodReturnsValue(method) {
		sb.WriteString("\tresult, ok := response[\"result\"]\n")
		sb.WriteString("\tif !ok {\n")
		if method.ReturnOptional {
			fmt.Fprintf(sb, "\t\tvar zero %s\n", goReturnType)
			sb.WriteString("\t\treturn zero, nil\n")
		} else {
			fmt.Fprintf(sb, "\t\tvar zero %s\n", goReturnType)
			sb.WriteString("\t\treturn zero, fmt.Errorf(\"missing result in response\")\n")
		}
		sb.WriteString("\t}\n\n")

		// Validate the raw result before decoding it. Null results are passed
		// through so a required return can be rejected.
		fmt.Fprintf(sb, "\tif c.validateResponses && c.contract != nil {\n")
		fmt.Fprintf(sb, "\t\tif err := c.contract.ValidateResponse(%q, %q, result); err != nil {\n", iface.Name, method.Name)
		writeErrorReturn("fmt.Errorf(\"response validation failed: %w\", err)")
		sb.WriteString("\t\t}\n\t}\n\n")

		fmt.Fprintf(sb, "\tvar typedResult %s\n", goReturnType)
		sb.WriteString("\tresultJSON, _ := json.Marshal(result)\n")
		sb.WriteString("\tif err := json.Unmarshal(resultJSON, &typedResult); err != nil {\n")
		fmt.Fprintf(sb, "\t\tvar zero %s\n", goReturnType)
		sb.WriteString("\t\treturn zero, fmt.Errorf(\"failed to unmarshal result: %w\", err)\n")
		sb.WriteString("\t}\n")
		sb.WriteString("\treturn typedResult, nil\n")
	} else {
		sb.WriteString("\treturn nil\n")
	}
	sb.WriteString("}\n\n")
}

// writeVerifyCompatibilityMethod generates the VerifyCompatibility method for the client
func writeVerifyCompatibilityMethod(sb *strings.Builder, clientName string) {
	fmt.Fprintf(sb, "func (c *%s) VerifyCompatibility(ctx context.Context) *pulserpc.VerificationResult {\n", clientName)
	sb.WriteString("	var clientIDL interface{}\n")
	sb.WriteString("	if c.localIDL != nil {\n")
	sb.WriteString("		clientIDL = c.localIDL\n")
	sb.WriteString("	} else if c.contract != nil {\n")
	sb.WriteString("		clientIDL = c.contract.GetIDLParsed()\n")
	sb.WriteString("	}\n\n")
	sb.WriteString("	var clientChecksum string\n")
	sb.WriteString("	if dict, ok := clientIDL.(map[string]interface{}); ok {\n")
	sb.WriteString("		clientChecksum, _ = dict[\"checksum\"].(string)\n")
	sb.WriteString("	}\n\n")
	sb.WriteString("	// The server IDL is fetched from the remote server via the pulserpc-idl RPC.\n")
	sb.WriteString("	// Never compare the embedded local IDL against itself. A failed fetch\n")
	sb.WriteString("	// cannot be treated as success: DiffIDL(clientIDL, nil) produces only\n")
	sb.WriteString("	// Info-level \"removed\" deltas, which would look compatible.\n")
	sb.WriteString("	var fetchErr error\n")
	sb.WriteString("	if c.serverIDL == nil && c.transport != nil {\n")
	sb.WriteString("		resp, err := c.transport.Call(ctx, \"pulserpc-idl\", []interface{}{})\n")
	sb.WriteString("		switch {\n")
	sb.WriteString("		case err != nil:\n")
	sb.WriteString("			fetchErr = err\n")
	sb.WriteString("		case resp == nil:\n")
	sb.WriteString("			fetchErr = fmt.Errorf(\"empty response\")\n")
	sb.WriteString("		default:\n")
	sb.WriteString("			if errObj, hasError := resp[\"error\"]; hasError {\n")
	sb.WriteString("				fetchErr = fmt.Errorf(\"server returned error: %v\", errObj)\n")
	sb.WriteString("			} else {\n")
	sb.WriteString("				c.serverIDL = resp[\"result\"]\n")
	sb.WriteString("			}\n")
	sb.WriteString("		}\n")
	sb.WriteString("	}\n")
	sb.WriteString("	if c.serverIDL == nil {\n")
	sb.WriteString("		if fetchErr == nil {\n")
	sb.WriteString("			fetchErr = fmt.Errorf(\"server IDL is empty\")\n")
	sb.WriteString("		}\n")
	sb.WriteString("		result := &pulserpc.VerificationResult{\n")
	sb.WriteString("			Compatible: false,\n")
	sb.WriteString("			ClientChecksum: clientChecksum,\n")
	sb.WriteString("			Error: \"failed to fetch server IDL via pulserpc-idl: \" + fetchErr.Error(),\n")
	sb.WriteString("			Timestamp: time.Now(),\n")
	sb.WriteString("		}\n")
	sb.WriteString("		if c.auditor != nil {\n")
	sb.WriteString("			c.auditor.Audit(ctx, result)\n")
	sb.WriteString("		}\n")
	sb.WriteString("		return result\n")
	sb.WriteString("	}\n")
	sb.WriteString("	serverIDL := c.serverIDL\n\n")
	sb.WriteString("	var serverChecksum string\n")
	sb.WriteString("	if dict, ok := serverIDL.(map[string]interface{}); ok {\n")
	sb.WriteString("		serverChecksum, _ = dict[\"checksum\"].(string)\n")
	sb.WriteString("	}\n\n")
	sb.WriteString("	deltas := pulserpc.DiffIDL(clientIDL, serverIDL)\n\n")
	sb.WriteString("	compatible := true\n")
	sb.WriteString("	for _, delta := range deltas {\n")
	sb.WriteString("		if delta.Severity == pulserpc.SeverityError {\n")
	sb.WriteString("			compatible = false\n")
	sb.WriteString("			break\n")
	sb.WriteString("		}\n")
	sb.WriteString("	}\n\n")
	sb.WriteString("	result := &pulserpc.VerificationResult{\n")
	sb.WriteString("		Compatible:     compatible,\n")
	sb.WriteString("		ServerChecksum: serverChecksum,\n")
	sb.WriteString("		ClientChecksum: clientChecksum,\n")
	sb.WriteString("		Deltas:         deltas,\n")
	sb.WriteString("		Timestamp:      time.Now(),\n")
	sb.WriteString("	}\n\n")
	sb.WriteString("	if c.auditor != nil {\n")
	sb.WriteString("		c.auditor.Audit(ctx, result)\n")
	sb.WriteString("	}\n\n")
	sb.WriteString("	return result\n")
	sb.WriteString("}\n")
}

// referencedTestServerNamespaces returns the sorted non-primary namespaces
// referenced by interface signatures. These must be dot-imported by the
// generated test server because its method implementations use bare type names.
func referencedTestServerNamespaces(idl *parser.IDL, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, primaryNs string, namespaceMap map[string]*NamespaceTypes) []string {
	refs := make(map[string]bool)
	for _, iface := range idl.Interfaces {
		for _, method := range iface.Methods {
			for _, param := range method.Parameters {
				findReferencedNamespacesInType(param.Type, structMap, enumMap, primaryNs, refs)
			}
			if goMethodReturnsValue(method) {
				findReferencedNamespacesInType(method.ReturnType, structMap, enumMap, primaryNs, refs)
			}
		}
	}
	// Test data literals also construct structs directly, so a shared type
	// reached only through a struct field (or an inherited parent) must be
	// dot-imported as well.
	for _, s := range idl.Structs {
		for _, field := range s.Fields {
			findReferencedNamespacesInType(field.Type, structMap, enumMap, primaryNs, refs)
		}
		if s.Extends != "" {
			if parentNs := findNamespaceForType(s.Extends, structMap, enumMap); parentNs != "" && parentNs != primaryNs {
				refs[parentNs] = true
			}
		}
	}

	names := make([]string, 0, len(refs))
	for ns := range refs {
		// Only import namespaces that actually exist in this IDL. When the
		// caller does not supply a namespace map, trust the resolved names.
		if namespaceMap != nil {
			if _, ok := namespaceMap[ns]; !ok {
				continue
			}
		}
		names = append(names, ns)
	}
	sort.Strings(names)
	return names
}

// generateTestServerGo generates test_server.go with concrete implementations of all interfaces
func generateTestServerGo(idl *parser.IDL, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, primaryImportPath string, packageImportPath string, namespaceMap map[string]*NamespaceTypes, primaryNs string, runtimeImportPath ...string) string {
	var sb strings.Builder

	// The divide handler returns a declared RPCError, which lives in the
	// generated pulserpc runtime package. Older callers omit the explicit
	// runtime path, so fall back to the conventional sibling package.
	runtimePath := path.Join(packageImportPath, "pulserpc")
	if len(runtimeImportPath) > 0 && runtimeImportPath[0] != "" {
		runtimePath = runtimeImportPath[0]
	}
	hasDivide := false
	for _, iface := range idl.Interfaces {
		for _, method := range iface.Methods {
			if strings.ToLower(method.Name) == "divide" {
				hasDivide = true
			}
		}
	}

	// Render the interface method bodies first so the import block can be
	// derived from what the emitted code actually references (e.g. math.Sqrt
	// or strings.ToUpper), rather than guessed from method names. A minimal
	// IDL without such methods must not import math/strings, otherwise
	// "go build" fails with "imported and not used".
	var implSB strings.Builder
	for _, iface := range idl.Interfaces {
		writeTestInterfaceImplGo(&implSB, iface, structMap, enumMap, idl)
	}
	implBody := implSB.String()
	usesMath := strings.Contains(implBody, "math.")
	usesStrings := strings.Contains(implBody, "strings.")
	// Context is only referenced by generated handler methods. An interface
	// with no methods emits no signatures, so importing context would make
	// go build fail with "imported and not used".
	hasInterfaceMethods := false
	for _, iface := range idl.Interfaces {
		if len(iface.Methods) > 0 {
			hasInterfaceMethods = true
			break
		}
	}

	sb.WriteString("// Generated by pulserpc - do not edit\n")
	sb.WriteString("// Test server implementation for integration testing\n\n")
	sb.WriteString("package main\n\n")
	sb.WriteString("import (\n")
	if hasInterfaceMethods {
		sb.WriteString("	\"context\"\n")
	}
	sb.WriteString("	\"fmt\"\n")
	if usesMath {
		sb.WriteString("	\"math\"\n")
	}
	sb.WriteString("	\"net/http\"\n")
	if usesStrings {
		sb.WriteString("	\"strings\"\n")
	}
	sb.WriteString(")\n\n")
	fmt.Fprintf(&sb, "import (\n")
	fmt.Fprintf(&sb, "\t. \"%s\"\n", primaryImportPath)
	// Interface implementations use bare type names, so every non-primary
	// namespace referenced by a signature must be dot-imported. The namespace
	// name comes from the IDL (not a hardcoded "inc"), so any shared namespace
	// works.
	for _, refNs := range referencedTestServerNamespaces(idl, structMap, enumMap, primaryNs, namespaceMap) {
		fmt.Fprintf(&sb, "\t. \"%s\"\n", path.Join(packageImportPath, refNs))
	}
	if hasDivide {
		fmt.Fprintf(&sb, "\t\"%s\"\n", runtimePath)
	}
	sb.WriteString(")\n\n")

	// Emit the interface implementations rendered above.
	sb.WriteString(implBody)

	// Generate main function
	sb.WriteString("func main() {\n")
	sb.WriteString("	port := 8080\n")
	sb.WriteString("	server := NewPulseRPCServer(\"0.0.0.0\", port)\n\n")
	for _, iface := range idl.Interfaces {
		implName := goInterfaceIdent(iface, idl) + "Impl"
		fmt.Fprintf(&sb, "\tif err := server.Register(\"%s\", &%s{}); err != nil {\n", iface.Name, implName)
		fmt.Fprintf(&sb, "\t\tpanic(err)\n\t}\n")
	}
	sb.WriteString("\n")
	sb.WriteString("\tmux := http.NewServeMux()\n")
	sb.WriteString("\t// Reuse the generated adapter so limits, batches, and error shapes are\n")
	sb.WriteString("\t// exercised by the integration test server too.\n")
	sb.WriteString("\tmux.Handle(\"/\", server)\n\n")
	sb.WriteString("\tmux.HandleFunc(\"/idl.json\", func(w http.ResponseWriter, r *http.Request) {\n")
	sb.WriteString("\t\tw.Header().Set(\"Content-Type\", \"application/json\")\n")
	sb.WriteString("\t\t// IDL_JSON is already JSON text - marshal it would produce a quoted\n")
	sb.WriteString("\t\t// JSON string instead of an IDL object.\n")
	sb.WriteString("\t\tw.Write([]byte(IDL_JSON))\n")
	sb.WriteString("\t})\n\n")
	sb.WriteString("\taddr := fmt.Sprintf(\"0.0.0.0:%d\", port)\n")
	sb.WriteString("\tfmt.Printf(\"Test server listening on http://%s\\n\", addr)\n")
	sb.WriteString("\tif err := http.ListenAndServe(addr, mux); err != nil {\n")
	sb.WriteString("\t\tfmt.Printf(\"Server error: %v\\n\", err)\n")
	sb.WriteString("\t}\n")
	sb.WriteString("}\n\n")

	return sb.String()
}

// writeTestInterfaceImplGo generates a test implementation struct for an interface
func writeTestInterfaceImplGo(sb *strings.Builder, iface *parser.Interface, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, idl ...*parser.IDL) {
	implName := goInterfaceIdent(iface, idl...) + "Impl"
	fmt.Fprintf(sb, "type %s struct{}\n\n", implName)

	// Generate method implementations
	for _, method := range iface.Methods {
		writeTestMethodImplGo(sb, iface, method, structMap, enumMap, idl...)
	}
}

// writeTestMethodImplGo generates a test method implementation
func writeTestMethodImplGo(sb *strings.Builder, iface *parser.Interface, method *parser.Method, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, idl ...*parser.IDL) {
	methodName := snakeToCamelCase(method.Name)
	fmt.Fprintf(sb, "func (i *%sImpl) %s(ctx context.Context", goInterfaceIdent(iface, idl...), methodName)

	// Parameters
	for _, param := range method.Parameters {
		sb.WriteString(", ")
		paramType := mapTypeToGoType(param.Type, structMap, enumMap, false)
		fmt.Fprintf(sb, "%s %s", goParamIdent(param.Name), paramType)
	}
	sb.WriteString(") ")

	// Return type
	if goMethodReturnsValue(method) {
		returnType := mapTypeToGoType(method.ReturnType, structMap, enumMap, method.ReturnOptional)
		fmt.Fprintf(sb, "(%s, error)", returnType)
	} else {
		sb.WriteString("error")
	}
	sb.WriteString(" {\n")

	// Special handling for known test cases
	if iface.Name == "B" && method.Name == "echo" {
		sb.WriteString("	if s == \"return-null\" {\n")
		sb.WriteString("		return nil, nil\n")
		sb.WriteString("	}\n")
		sb.WriteString("	return &s, nil\n")
		sb.WriteString("}\n\n")
		return
	}

	// Generate based on method name patterns
	methodNameLower := strings.ToLower(method.Name)
	switch methodNameLower {
	case "add":
		sb.WriteString("	return a + b, nil\n")
	case "sqrt":
		sb.WriteString("	return math.Sqrt(a), nil\n")
	case "calc":
		sb.WriteString("	if len(nums) == 0 {\n")
		sb.WriteString("		return 0.0, nil\n")
		sb.WriteString("	}\n")
		sb.WriteString("	if operation == \"add\" {\n")
		sb.WriteString("		sum := 0.0\n")
		sb.WriteString("		for _, num := range nums {\n")
		sb.WriteString("			sum += num\n")
		sb.WriteString("		}\n")
		sb.WriteString("		return sum, nil\n")
		sb.WriteString("	} else if operation == \"multiply\" {\n")
		sb.WriteString("		product := 1.0\n")
		sb.WriteString("		for _, num := range nums {\n")
		sb.WriteString("			product *= num\n")
		sb.WriteString("		}\n")
		sb.WriteString("		return product, nil\n")
		sb.WriteString("	}\n")
		sb.WriteString("	return 0.0, nil\n")
	case "divide":
		sb.WriteString("	if b == 0 {\n")
		sb.WriteString("		return 0.0, &pulserpc.RPCError{Code: 1001, Message: \"Validation failed\"}\n")
		sb.WriteString("	}\n")
		sb.WriteString("	return float64(a) / float64(b), nil\n")
	case "repeat":
		sb.WriteString("	text := req1.ToRepeat\n")
		sb.WriteString("	count := req1.Count\n")
		sb.WriteString("\tif count < 0 { panic(\"PULSERPC_SECRET_TOKEN_7F3A\") }\n")
		sb.WriteString("	if req1.ForceUppercase {\n")
		sb.WriteString("		text = strings.ToUpper(text)\n")
		sb.WriteString("	}\n")
		sb.WriteString("	items := make([]string, count)\n")
		sb.WriteString("	for i := int64(0); i < count; i++ {\n")
		sb.WriteString("		items[i] = text\n")
		sb.WriteString("	}\n")
		sb.WriteString("	return RepeatResponse{\n")
		sb.WriteString("		Response: Response{Status: StatusOk},\n")
		sb.WriteString("		Count:  count,\n")
		sb.WriteString("		Items:  items,\n")
		sb.WriteString("	}, nil\n")
	case "say_hi":
		sb.WriteString("	return HiResponse{Hi: \"hi\"}, nil\n")
	case "repeat_num":
		sb.WriteString("	result := make([]int64, count)\n")
		sb.WriteString("	for i := range result {\n")
		sb.WriteString("		result[i] = num\n")
		sb.WriteString("	}\n")
		sb.WriteString("	return result, nil\n")
	case "putperson":
		sb.WriteString("	return p.PersonId, nil\n")
	case "echooptional":
		// echo the whole Person back so optional built-ins round-trip
		sb.WriteString("	return p, nil\n")
	case "pick":
		// return the enum argument unchanged so its wire string round-trips
		sb.WriteString("	return op, nil\n")
	default:
		// Default implementation
		if goMethodReturnsValue(method) {
			returnType := mapTypeToGoType(method.ReturnType, structMap, enumMap, method.ReturnOptional)
			sb.WriteString("	var zero ")
			sb.WriteString(returnType)
			sb.WriteString("\n")
			sb.WriteString("	return zero, nil\n")
		} else {
			sb.WriteString("	return nil\n")
		}
	}
	sb.WriteString("}\n\n")
}

// generateTestClientGo generates test_client.go test program
func generateTestClientGo(idl *parser.IDL, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, primaryImportPath string, primaryNs string, packageImportPath string) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n")
	sb.WriteString("// Test client for integration testing\n\n")
	sb.WriteString("package main\n\n")
	sb.WriteString("import (\n")
	sb.WriteString("	\"bytes\"\n")
	sb.WriteString("	\"context\"\n")
	sb.WriteString("	\"fmt\"\n")
	sb.WriteString("	\"net/http\"\n")
	sb.WriteString("	\"os\"\n")
	sb.WriteString("	\"time\"\n")
	// Import the generated package (primaryNs)
	fmt.Fprintf(&sb, "	. \"%s\"\n", primaryImportPath)
	// Test data literals use bare type names, so dot-import every other
	// namespace referenced by the interface signatures (e.g. a shared
	// namespace named "common", not a hardcoded "inc").
	for _, refNs := range referencedTestServerNamespaces(idl, structMap, enumMap, primaryNs, nil) {
		fmt.Fprintf(&sb, "	. \"%s\"\n", path.Join(packageImportPath, refNs))
	}
	sb.WriteString(")\n\n")

	sb.WriteString("// divergentIDLTransport is a stub transport that reports a server IDL\n")
	sb.WriteString("// different from this client's embedded IDL (R31).\n")
	sb.WriteString("type divergentIDLTransport struct{}\n\n")
	sb.WriteString("func (divergentIDLTransport) Call(ctx context.Context, method string, params []interface{}) (map[string]interface{}, error) {\n")
	sb.WriteString("	if method != \"pulserpc-idl\" {\n")
	sb.WriteString("		return nil, fmt.Errorf(\"unexpected method %s\", method)\n")
	sb.WriteString("	}\n")
	sb.WriteString("	return map[string]interface{}{\n")
	sb.WriteString("		\"jsonrpc\": \"2.0\",\n")
	sb.WriteString("		\"id\":      1,\n")
	sb.WriteString("		\"result\": map[string]interface{}{\n")
	sb.WriteString("			\"rootNamespace\": \"divergent\",\n")
	sb.WriteString("			\"checksum\":      \"divergent-checksum\",\n")
	sb.WriteString("			\"interfaces\":    []interface{}{},\n")
	sb.WriteString("			\"structs\":       []interface{}{},\n")
	sb.WriteString("			\"enums\":         []interface{}{},\n")
	sb.WriteString("			\"errors\":        []interface{}{},\n")
	sb.WriteString("		},\n")
	sb.WriteString("	}, nil\n")
	sb.WriteString("}\n\n")

	sb.WriteString("func waitForServer(url string, timeout time.Duration) bool {\n")
	sb.WriteString("	start := time.Now()\n")
	sb.WriteString("	for time.Since(start) < timeout {\n")
	sb.WriteString("		resp, err := http.Post(url, \"application/json\", bytes.NewReader([]byte(\"{\\\"jsonrpc\\\":\\\"2.0\\\",\\\"method\\\":\\\"pulserpc-idl\\\",\\\"id\\\":1}\")))\n")
	sb.WriteString("		if err == nil && resp.StatusCode == 200 {\n")
	sb.WriteString("			resp.Body.Close()\n")
	sb.WriteString("			return true\n")
	sb.WriteString("		}\n")
	sb.WriteString("		if resp != nil {\n")
	sb.WriteString("			resp.Body.Close()\n")
	sb.WriteString("		}\n")
	sb.WriteString("		time.Sleep(500 * time.Millisecond)\n")
	sb.WriteString("	}\n")
	sb.WriteString("	return false\n")
	sb.WriteString("}\n\n")

	sb.WriteString("func main() {\n")
	sb.WriteString("	serverURL := \"http://localhost:8080\"\n")
	sb.WriteString("	if len(os.Args) > 1 {\n")
	sb.WriteString("		serverURL = os.Args[1]\n")
	sb.WriteString("	}\n\n")

	sb.WriteString("	fmt.Println(\"Waiting for server to be ready...\")\n")
	sb.WriteString("	if !waitForServer(serverURL, 10*time.Second) {\n")
	sb.WriteString("		fmt.Fprintf(os.Stderr, \"ERROR: Server did not become ready in time\\n\")\n")
	sb.WriteString("		os.Exit(1)\n")
	sb.WriteString("	}\n\n")

	sb.WriteString("	fmt.Println(\"Server is ready. Running tests...\")\n")
	sb.WriteString("	fmt.Println()\n\n")

	fmt.Fprintf(&sb, "	transport := NewHTTPTransport(serverURL, nil)\n")
	for _, iface := range idl.Interfaces {
		clientName := goInterfaceIdent(iface, idl) + "Client"
		clientVar := strings.ToLower(goInterfaceIdent(iface, idl)) + "Client"
		fmt.Fprintf(&sb, "	%s := New%s(transport)\n", clientVar, clientName)
		if len(iface.Methods) == 0 {
			// No test calls will reference the client; keep it used so a
			// methodless interface still builds.
			fmt.Fprintf(&sb, "	_ = %s\n", clientVar)
		}
	}
	sb.WriteString("\n")

	sb.WriteString("	errors := []string{}\n\n")

	// R31: exercise VerifyCompatibility with a stub transport that serves a
	// divergent IDL. This runs only when the generated test files are used.
	if len(idl.Interfaces) > 0 {
		first := idl.Interfaces[0]
		fmt.Fprintf(&sb, "	divergentClient := New%sClient(divergentIDLTransport{})\n", goInterfaceIdent(first, idl))
		sb.WriteString("	if result := divergentClient.VerifyCompatibility(context.Background()); result.Compatible {\n")
		sb.WriteString("		errors = append(errors, \"VerifyCompatibility accepted a divergent server IDL\")\n")
		sb.WriteString("	} else if result.ServerChecksum != \"divergent-checksum\" {\n")
		sb.WriteString("		errors = append(errors, \"VerifyCompatibility did not use the fetched server checksum\")\n")
		sb.WriteString("	}\n\n")
	}

	// Generate test cases for each method
	for _, iface := range idl.Interfaces {
		clientVar := strings.ToLower(goInterfaceIdent(iface, idl)) + "Client"
		for _, method := range iface.Methods {
			writeTestClientCallGo(&sb, iface, method, clientVar, structMap, enumMap)
		}
	}

	sb.WriteString("	fmt.Println()\n")
	sb.WriteString("	if len(errors) > 0 {\n")
	sb.WriteString("		fmt.Fprintf(os.Stderr, \"FAILED: %d test(s) failed:\\n\", len(errors))\n")
	sb.WriteString("		for _, err := range errors {\n")
	sb.WriteString("			fmt.Fprintf(os.Stderr, \"  - %s\\n\", err)\n")
	sb.WriteString("		}\n")
	sb.WriteString("		os.Exit(1)\n")
	sb.WriteString("	} else {\n")
	sb.WriteString("		fmt.Println(\"SUCCESS: All tests passed!\")\n")
	sb.WriteString("		os.Exit(0)\n")
	sb.WriteString("	}\n")
	sb.WriteString("}\n")

	return sb.String()
}

// writeTestClientCallGo generates a test call for a method
func writeTestClientCallGo(sb *strings.Builder, iface *parser.Interface, method *parser.Method, clientVar string, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	testName := fmt.Sprintf("%s.%s", iface.Name, method.Name)
	fmt.Fprintf(sb, "	// Test %s\n", testName)
	sb.WriteString("	func() {\n")
	sb.WriteString("		defer func() {\n")
	sb.WriteString("			if r := recover(); r != nil {\n")
	fmt.Fprintf(sb, "				errors = append(errors, fmt.Sprintf(\"%s failed: %%v\", r))\n", testName)
	sb.WriteString("			}\n")
	sb.WriteString("		}()\n")

	// Generate test parameters
	params := make([]string, 0)
	for _, param := range method.Parameters {
		paramValue := generateTestParamValueGo(param.Type, param.Name, structMap, enumMap)
		params = append(params, paramValue)
	}

	// Generate method call, always passing a context as the first argument.
	methodName := snakeToCamelCase(method.Name)
	callArgs := append([]string{"context.Background()"}, params...)
	if goMethodReturnsValue(method) {
		fmt.Fprintf(sb, "		result, err := %s.%s(%s)\n", clientVar, methodName, strings.Join(callArgs, ", "))
	} else {
		fmt.Fprintf(sb, "		err := %s.%s(%s)\n", clientVar, methodName, strings.Join(callArgs, ", "))
	}
	sb.WriteString("		if err != nil {\n")
	fmt.Fprintf(sb, "			errors = append(errors, fmt.Sprintf(\"%s failed: %%v\", err))\n", testName)
	sb.WriteString("			return\n")
	sb.WriteString("		}\n")

	if !goMethodReturnsValue(method) {
		// Void methods return only error; there is no result to assert.
		fmt.Fprintf(sb, "		fmt.Printf(\"✓ %s passed\\n\")\n", testName)
		sb.WriteString("	}()\n\n")
		return
	}

	// Generate assertions
	methodNameLower := strings.ToLower(method.Name)
	if iface.Name == "B" && method.Name == "echo" {
		sb.WriteString("		if result == nil || *result != \"test\" {\n")
		fmt.Fprintf(sb, "			errors = append(errors, fmt.Sprintf(\"%s: expected 'test', got %%v\", result))\n", testName)
		sb.WriteString("			return\n")
		sb.WriteString("		}\n")
		sb.WriteString("		// Test null return\n")
		fmt.Fprintf(sb, "		resultNull, _ := %s.Echo(context.Background(), \"return-null\")\n", clientVar)
		sb.WriteString("		if resultNull != nil {\n")
		fmt.Fprintf(sb, "			errors = append(errors, fmt.Sprintf(\"%s (null): expected nil, got %%v\", resultNull))\n", testName)
		sb.WriteString("			return\n")
		sb.WriteString("		}\n")
	} else if methodNameLower == "add" {
		sb.WriteString("		if result != 5 {\n")
		fmt.Fprintf(sb, "			errors = append(errors, fmt.Sprintf(\"%s: expected 5, got %%v\", result))\n", testName)
		sb.WriteString("			return\n")
		sb.WriteString("		}\n")
	} else if methodNameLower == "sqrt" {
		sb.WriteString("		if result < 1.99 || result > 2.01 {\n")
		fmt.Fprintf(sb, "			errors = append(errors, fmt.Sprintf(\"%s: expected ~2.0, got %%v\", result))\n", testName)
		sb.WriteString("			return\n")
		sb.WriteString("		}\n")
	} else {
		sb.WriteString("		_ = result // Use result to avoid unused variable\n")
	}

	fmt.Fprintf(sb, "		fmt.Printf(\"✓ %s passed\\n\")\n", testName)
	sb.WriteString("	}()\n\n")
}

// generateTestParamValueGo generates a test parameter value
func generateTestParamValueGo(t *parser.Type, paramName string, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) string {
	if t.IsBuiltIn() {
		switch t.BuiltIn {
		case "string":
			if paramName == "s" {
				return "\"test\""
			}
			return "\"test\""
		case "int":
			switch paramName {
			case "a", "num":
				return "2"
			case "b":
				return "3"
			case "count":
				return "2"
			default:
				return "1"
			}
		case "float":
			if paramName == "a" {
				return "4.0"
			}
			return "1.0"
		case "bool":
			return "true"
		default:
			return "nil"
		}
	} else if t.IsArray() {
		if t.Array.IsBuiltIn() && t.Array.BuiltIn == "float" {
			return "[]float64{1.0, 2.0, 3.0}"
		}
		return "[]interface{}{}"
	} else if t.IsMap() {
		return "map[string]interface{}{}"
	} else if t.IsUserDefined() {
		// Check if it's a struct
		if structMap[t.UserDefined] != nil {
			s := structMap[t.UserDefined]
			// Build struct literal
			fields := []string{}
			for _, field := range s.Fields {
				if field.Optional && field.Name == "email" {
					// Special case: set email to nil for putPerson test
					fields = append(fields, fmt.Sprintf("%s: nil", snakeToCamelCase(field.Name)))
				} else if !field.Optional {
					fieldValue := generateTestParamValueGo(field.Type, field.Name, structMap, enumMap)
					fields = append(fields, fmt.Sprintf("%s: %s", snakeToCamelCase(field.Name), fieldValue))
				}
			}
			// Handle inheritance
			if s.Extends != "" {
				baseName := GetBaseName(s.Extends)
				if baseStruct := structMap[baseName]; baseStruct != nil {
					for _, field := range baseStruct.Fields {
						if !field.Optional {
							fieldValue := generateTestParamValueGo(field.Type, field.Name, structMap, enumMap)
							fields = append(fields, fmt.Sprintf("%s: %s", snakeToCamelCase(field.Name), fieldValue))
						}
					}
				}
			}
			// Special handling for RepeatRequest
			if t.UserDefined == "RepeatRequest" || GetBaseName(t.UserDefined) == "RepeatRequest" {
				return "RepeatRequest{ToRepeat: \"hello\", Count: 3, ForceUppercase: false}"
			}
			// Special handling for Person
			if t.UserDefined == "Person" || GetBaseName(t.UserDefined) == "Person" {
				return "Person{PersonId: \"person123\", FirstName: \"John\", LastName: \"Doe\", Email: nil}"
			}
			structName := GetBaseName(t.UserDefined)
			return structName + "{" + strings.Join(fields, ", ") + "}"
		} else if enumMap[t.UserDefined] != nil {
			e := enumMap[t.UserDefined]
			if len(e.Values) > 0 {
				// Special case for MathOp
				if strings.Contains(t.UserDefined, "MathOp") {
					return "\"add\""
				}
				enumName := GetBaseName(t.UserDefined)
				valName := e.Values[0].Name
				return fmt.Sprintf("%s%s", enumName, snakeToCamelCase(valName))
			}
			return "nil"
		}
		return "nil"
	}
	return "nil"
}

// escapeGoString escapes a string for use as a Go string literal
// Escapes backslashes, double quotes, newlines, and other special characters
func escapeGoString(s string) string {
	var sb strings.Builder
	sb.WriteString("\"") // Start of Go string
	for _, r := range s {
		switch r {
		case '\\':
			sb.WriteString("\\\\") // Escape backslashes
		case '"':
			sb.WriteString("\\\"") // Escape double quotes
		case '\n':
			sb.WriteString("\\n") // Escape newlines
		case '\r':
			sb.WriteString("\\r") // Escape carriage returns
		case '\t':
			sb.WriteString("\\t") // Escape tabs
		default:
			sb.WriteRune(r)
		}
	}
	sb.WriteString("\"") // End of Go string
	return sb.String()
}

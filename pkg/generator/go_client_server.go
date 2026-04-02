package generator

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
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
}

// writeIDLJSONGo writes the IDL metadata as JSON to idl.json
//
// IDL Placement Note:
// This function intentionally writes idl.json directly to outputDir (the root -dir),
// NOT to a namespace subdirectory. The idl.json file is consumed by the runtime
// Contract class at startup, and it must be accessible as {root}/idl.json regardless
// of which namespace mode is active.
func writeIDLJSONGo(idl *parser.IDL, outputDir string, fs *flag.FlagSet) error {
	idlJSON, err := json.MarshalIndent(idl, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal IDL to JSON: %w", err)
	}

	idlPath := filepath.Join(outputDir, "idl.json")
	if err := os.WriteFile(idlPath, idlJSON, 0644); err != nil {
		return fmt.Errorf("failed to write idl.json: %w", err)
	}
	PrintFileCreated(idlPath, fs)
	return nil
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

	// Get the primary namespace for package name
	// Prefer the namespace that has interfaces (the main IDL file)
	// over namespaces that only have imported types
	primaryNs := ""

	// First, try to find a namespace with interfaces
	for ns, types := range namespaceMap {
		if ns != "" && len(types.Interfaces) > 0 {
			primaryNs = ns
			break
		}
	}

	// If no namespace has interfaces, pick the first non-empty one
	if primaryNs == "" {
		for ns := range namespaceMap {
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

	// Write idl.json to output directory
	if err := writeIDLJSONGo(idl, outputDir, fs); err != nil {
		return fmt.Errorf("failed to write idl.json: %w", err)
	}

	// Generate namespace subdirectories and files
	for namespace, types := range namespaceMap {
		if namespace == "" {
			continue // Skip types without namespace (shouldn't happen with required namespaces)
		}
		// Create namespace subdirectory
		namespaceDir := filepath.Join(outputDir, namespace)
		if err := os.MkdirAll(namespaceDir, 0755); err != nil {
			return fmt.Errorf("failed to create namespace directory: %w", err)
		}

		namespaceCode := generateNamespaceGo(namespace, types, structMap, enumMap, packageImportPath)
		namespacePath := filepath.Join(namespaceDir, namespace+".go")
		if err := os.WriteFile(namespacePath, []byte(namespaceCode), 0644); err != nil {
			return fmt.Errorf("failed to write %s/%s.go: %w", namespace, namespace, err)
		}
		PrintFileCreated(namespacePath, fs)
	}

	// Generate server.go in the primary namespace directory
	serverCode := generateServerGo(idl, structMap, enumMap, primaryNs, namespaceMap, runtimeImportPath, packageImportPath)
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
		testServerCode := generateTestServerGo(idl, structMap, enumMap, primaryImportPath, packageImportPath, namespaceMap, primaryNs)
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
		testClientCode := generateTestClientGo(idl, structMap, enumMap, primaryImportPath)
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
		// All files exist, skip copying
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

// mapTypeToGoType maps an IDL type to a Go type string
func mapTypeToGoType(t *parser.Type, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, optional bool) string {
	if t.IsBuiltIn() {
		var goType string
		switch t.BuiltIn {
		case "string":
			goType = "string"
		case "int":
			goType = "int"
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
			goType = "int"
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

// findNamespaceForType finds which namespace a type belongs to
func findNamespaceForType(typeName string, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) string {
	// Check if type is already qualified
	if strings.Contains(typeName, ".") {
		parts := strings.Split(typeName, ".")
		return parts[0]
	}

	// Check struct map for namespace info
	// The structMap keys are full type names with namespace
	for fullName := range structMap {
		if GetBaseName(fullName) == typeName {
			if strings.Contains(fullName, ".") {
				parts := strings.Split(fullName, ".")
				return parts[0]
			}
		}
	}

	// Check enum map
	for fullName := range enumMap {
		if GetBaseName(fullName) == typeName {
			if strings.Contains(fullName, ".") {
				parts := strings.Split(fullName, ".")
				return parts[0]
			}
		}
	}

	return ""
}

// findTestServerImports finds all namespace imports needed for test server code
func findTestServerImports(t *parser.Type, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, currentNamespace string, nsImports map[string]bool) {
	if t.IsUserDefined() {
		if ns := findNamespaceForType(t.UserDefined, structMap, enumMap); ns != "" && ns != currentNamespace {
			nsImports[ns] = true
		}
	} else if t.IsArray() {
		findTestServerImports(t.Array, structMap, enumMap, currentNamespace, nsImports)
	} else if t.IsMap() {
		findTestServerImports(t.MapValue, structMap, enumMap, currentNamespace, nsImports)
	}
}

// generateNamespaceGo generates a Go file for a single namespace
func generateNamespaceGo(namespace string, types *NamespaceTypes, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, packageImportPath string) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString(fmt.Sprintf("package %s\n\n", namespace))

	// Find referenced namespaces for cross-namespace imports
	referencedNamespaces := getReferencedNamespaces(types, structMap, enumMap, namespace)

	// Build import section - only include runtime import if needed
	// Since ALL_STRUCTS/ALL_ENUMS were removed, namespace files don't need pulserpc import
	sb.WriteString("import (\n")

	// Add imports for referenced namespaces
	for refNs := range referencedNamespaces {
		importPath := packageImportPath + "/" + refNs
		sb.WriteString(fmt.Sprintf("	\"%s\"\n", importPath))
	}

	sb.WriteString(")\n\n")

	// Generate enum types first (they may be referenced by structs)
	generateEnumTypesGo(&sb, types.Enums)
	sb.WriteString("\n")

	// Generate struct types
	generateStructTypesGo(&sb, namespace, types.Structs, structMap, enumMap)
	sb.WriteString("\n")

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
		for i, val := range e.Values {
			if i == 0 {
				fmt.Fprintf(sb, "	%s%s %s = \"%s\"\n", enumName, snakeToCamelCase(val.Name), enumName, val.Name)
			} else {
				fmt.Fprintf(sb, "	%s%s %s = \"%s\"\n", enumName, snakeToCamelCase(val.Name), enumName, val.Name)
			}
		}
		sb.WriteString(")\n\n")
	}
}

// generateStructTypesGo generates Go struct types for all structs in the namespace
func generateStructTypesGo(sb *strings.Builder, namespace string, structs []*parser.Struct, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
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
			parentName := getGoStructOrEnumTypeName(s.Extends, structMap, enumMap)
			// Check if the parent is from a different namespace
			parentNs := findNamespaceForType(s.Extends, structMap, enumMap)
			if parentNs != "" && parentNs != namespace {
				parentName = parentNs + "." + GetBaseName(s.Extends)
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
			goType := mapTypeToGoType(field.Type, structMap, enumMap, field.Optional)
			jsonTag := field.Name
			if field.Optional {
				jsonTag += ",omitempty"
			}
			fmt.Fprintf(sb, "	%s %s `json:\"%s\"`\n", fieldName, goType, jsonTag)
		}

		sb.WriteString("}\n\n")
	}
}

// generateServerGo generates the server.go file with HTTP server and interface stubs
func generateServerGo(idl *parser.IDL, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, primaryNs string, namespaceMap map[string]*NamespaceTypes, runtimeImportPath string, packageImportPath string) string {
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
			if method.ReturnType != nil {
				getGoTypeWithNamespace(method.ReturnType, structMap, enumMap, method.ReturnOptional, primaryNs, nsImports, packageImportPath)
			}
		}
	}

	// Write imports
	sb.WriteString("import (\n")
	sb.WriteString("	\"encoding/json\"\n")
	sb.WriteString("	\"fmt\"\n")
	sb.WriteString("	\"io\"\n")
	sb.WriteString("	\"net/http\"\n")

	// Import the shared pulserpc runtime
	sb.WriteString(fmt.Sprintf("	\"%s\"\n", runtimeImportPath))

	// Add cross-namespace imports
	for _, importPath := range nsImports {
		sb.WriteString(fmt.Sprintf("	\"%s\"\n", importPath))
	}

	sb.WriteString(")\n\n")

	// Embed IDL JSON directly in server.go for pulserpc-idl RPC method
	idlJSON, err := json.MarshalIndent(idl, "", "  ")
	if err == nil {
		sb.WriteString("// IDL_JSON contains the IDL definition used to generate this code\n")
		sb.WriteString("const IDL_JSON = ")
		sb.WriteString(escapeGoString(string(idlJSON)))
		sb.WriteString("\n\n")
	}

	// Generate interface stubs
	for _, iface := range idl.Interfaces {
		writeInterfaceStubGo(&sb, iface, structMap, enumMap, primaryNs, nsImports, packageImportPath)
	}

	// Generate PulseRPCServer
	writePulseRPCServerGo(&sb)

	return sb.String()
}

// writeInterfaceStubGo generates a Go interface for an IDL interface
func writeInterfaceStubGo(sb *strings.Builder, iface *parser.Interface, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, namespace string, nsImports map[string]string, packageImportPath string) {
	if iface.Comment != "" {
		lines := strings.Split(strings.TrimSpace(iface.Comment), "\n")
		for _, line := range lines {
			fmt.Fprintf(sb, "// %s\n", line)
		}
	}
	fmt.Fprintf(sb, "type %s interface {\n", iface.Name)

	for _, method := range iface.Methods {
		methodName := snakeToCamelCase(method.Name)
		fmt.Fprintf(sb, "	%s(", methodName)

		// Parameters
		for i, param := range method.Parameters {
			if i > 0 {
				sb.WriteString(", ")
			}
			paramType := getGoTypeWithNamespace(param.Type, structMap, enumMap, false, namespace, nsImports, packageImportPath)
			fmt.Fprintf(sb, "%s %s", param.Name, paramType)
		}
		sb.WriteString(") ")

		// Return type
		if method.ReturnType != nil {
			returnType := getGoTypeWithNamespace(method.ReturnType, structMap, enumMap, method.ReturnOptional, namespace, nsImports, packageImportPath)
			sb.WriteString(returnType)
		} else {
			sb.WriteString("error")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("}\n\n")
}

// writePulseRPCServerGo generates the PulseRPCServer struct and methods
// Refactored to use pulserpc.Server instead of inline implementation
func writePulseRPCServerGo(sb *strings.Builder) {
	sb.WriteString("// PulseRPCServer is an HTTP server for JSON-RPC 2.0 requests\n")
	sb.WriteString("// This implementation uses the pulserpc.Server runtime class\n")
	sb.WriteString("type PulseRPCServer struct {\n")
	sb.WriteString("	host     string\n")
	sb.WriteString("	port     int\n")
	sb.WriteString("	rpcServer *pulserpc.Server\n")
	sb.WriteString("	server   *http.Server\n")
	sb.WriteString("}\n\n")

	sb.WriteString("// NewPulseRPCServer creates a new PulseRPCServer\n")
	sb.WriteString("func NewPulseRPCServer(host string, port int) *PulseRPCServer {\n")
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
	sb.WriteString("	rpcServer := pulserpc.NewServer(contract, true, true)\n")
	sb.WriteString("	\n")
	sb.WriteString("	return &PulseRPCServer{\n")
	sb.WriteString("		host:     host,\n")
	sb.WriteString("		port:     port,\n")
	sb.WriteString("		rpcServer: rpcServer,\n")
	sb.WriteString("	}\n")
	sb.WriteString("}\n\n")

	sb.WriteString("// Register registers an interface implementation\n")
	sb.WriteString("func (s *PulseRPCServer) Register(interfaceName string, implementation interface{}) {\n")
	sb.WriteString("	s.rpcServer.AddHandler(interfaceName, implementation)\n")
	sb.WriteString("}\n\n")

	sb.WriteString("// ServeForever starts the HTTP server and serves forever\n")
	sb.WriteString("func (s *PulseRPCServer) ServeForever() error {\n")
	sb.WriteString("	mux := http.NewServeMux()\n")
	sb.WriteString("	mux.HandleFunc(\"/\", s.handleRequest)\n")
	sb.WriteString("	addr := fmt.Sprintf(\"%s:%d\", s.host, s.port)\n")
	sb.WriteString("	s.server = &http.Server{\n")
	sb.WriteString("		Addr:    addr,\n")
	sb.WriteString("		Handler: mux,\n")
	sb.WriteString("	}\n")
	sb.WriteString("	fmt.Printf(\"PulseRPC server listening on http://%s\\n\", addr)\n")
	sb.WriteString("	return s.server.ListenAndServe()\n")
	sb.WriteString("}\n\n")

	// Generate handleRequest method that uses pulserpc.Server
	writeServerHandleRequestGoRefactored(sb)
}

// writeServerHandleRequestGoRefactored generates the handleRequest method using pulserpc.Server
func writeServerHandleRequestGoRefactored(sb *strings.Builder) {
	sb.WriteString("// handleRequest handles HTTP requests by delegating to pulserpc.Server.Call()\n")
	sb.WriteString("func (s *PulseRPCServer) handleRequest(w http.ResponseWriter, r *http.Request) {\n")
	sb.WriteString("	if r.Method != http.MethodPost {\n")
	sb.WriteString("		http.Error(w, \"Method Not Allowed\", http.StatusMethodNotAllowed)\n")
	sb.WriteString("		return\n")
	sb.WriteString("	}\n\n")

	sb.WriteString("	body, err := io.ReadAll(r.Body)\n")
	sb.WriteString("	if err != nil {\n")
	sb.WriteString("		s.sendErrorResponse(w, nil, -32700, \"Parse error\", fmt.Sprintf(\"Failed to read body: %v\", err))\n")
	sb.WriteString("		return\n")
	sb.WriteString("	}\n\n")

	sb.WriteString("	var requestData interface{}\n")
	sb.WriteString("	if err := json.Unmarshal(body, &requestData); err != nil {\n")
	sb.WriteString("		s.sendErrorResponse(w, nil, -32700, \"Parse error\", fmt.Sprintf(\"Invalid JSON: %v\", err))\n")
	sb.WriteString("		return\n")
	sb.WriteString("	}\n\n")

	sb.WriteString("	// Handle batch requests\n")
	sb.WriteString("	if requests, ok := requestData.([]interface{}); ok {\n")
	sb.WriteString("		if len(requests) == 0 {\n")
	sb.WriteString("			s.sendErrorResponse(w, nil, -32600, \"Invalid Request\", \"Empty batch array\")\n")
	sb.WriteString("			return\n")
	sb.WriteString("		}\n")
	sb.WriteString("		var responses []interface{}\n")
	sb.WriteString("		for _, req := range requests {\n")
	sb.WriteString("			if reqMap, ok := req.(map[string]interface{}); ok {\n")
	sb.WriteString("				resp := s.rpcServer.Call(reqMap)\n")
	sb.WriteString("				if resp != nil {\n")
	sb.WriteString("					responses = append(responses, resp)\n")
	sb.WriteString("				}\n")
	sb.WriteString("			}\n")
	sb.WriteString("		}\n")
	sb.WriteString("		if len(responses) == 0 {\n")
	sb.WriteString("			w.WriteHeader(http.StatusNoContent)\n")
	sb.WriteString("			return\n")
	sb.WriteString("		}\n")
	sb.WriteString("		w.Header().Set(\"Content-Type\", \"application/json\")\n")
	sb.WriteString("		json.NewEncoder(w).Encode(responses)\n")
	sb.WriteString("		return\n")
	sb.WriteString("	}\n\n")

	sb.WriteString("	// Handle single request using pulserpc.Server.Call()\n")
	sb.WriteString("	if reqMap, ok := requestData.(map[string]interface{}); ok {\n")
	sb.WriteString("		response := s.rpcServer.Call(reqMap)\n")
	sb.WriteString("		if response == nil {\n")
	sb.WriteString("			// Notification - no response expected\n")
	sb.WriteString("			w.WriteHeader(http.StatusNoContent)\n")
	sb.WriteString("			return\n")
	sb.WriteString("		}\n")
	sb.WriteString("		w.Header().Set(\"Content-Type\", \"application/json\")\n")
	sb.WriteString("		json.NewEncoder(w).Encode(response)\n")
	sb.WriteString("	} else {\n")
	sb.WriteString("		s.sendErrorResponse(w, nil, -32600, \"Invalid Request\", \"Request must be an object or array\")\n")
	sb.WriteString("	}\n")
	sb.WriteString("}\n\n")

	sb.WriteString("// sendErrorResponse sends a JSON-RPC error response\n")
	sb.WriteString("func (s *PulseRPCServer) sendErrorResponse(w http.ResponseWriter, requestID interface{}, code int, message string, data interface{}) {\n")
	sb.WriteString("	errObj := map[string]interface{}{\n")
	sb.WriteString("		\"code\":    code,\n")
	sb.WriteString("		\"message\": message,\n")
	sb.WriteString("	}\n")
	sb.WriteString("	if data != nil {\n")
	sb.WriteString("		errObj[\"data\"] = data\n")
	sb.WriteString("	}\n")
	sb.WriteString("	response := map[string]interface{}{\n")
	sb.WriteString("		\"jsonrpc\": \"2.0\",\n")
	sb.WriteString("		\"error\":   errObj,\n")
	sb.WriteString("	}\n")
	sb.WriteString("	if requestID != nil {\n")
	sb.WriteString("		response[\"id\"] = requestID\n")
	sb.WriteString("	}\n")
	sb.WriteString("	w.Header().Set(\"Content-Type\", \"application/json\")\n")
	sb.WriteString("	json.NewEncoder(w).Encode(response)\n")
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
			if method.ReturnType != nil {
				getGoTypeWithNamespace(method.ReturnType, structMap, enumMap, method.ReturnOptional, primaryNs, nsImports, packageImportPath)
			}
		}
	}

	sb.WriteString("import (\n")
	sb.WriteString("	\"bytes\"\n")
	sb.WriteString("	\"encoding/json\"\n")
	sb.WriteString("	\"fmt\"\n")
	sb.WriteString("	\"net/http\"\n")
	sb.WriteString("	\"strings\"\n")

	// Import the shared pulserpc runtime
	sb.WriteString(fmt.Sprintf("	\"%s\"\n", runtimeImportPath))

	// Add cross-namespace imports
	for _, importPath := range nsImports {
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
		writeInterfaceClientGo(&sb, iface, structMap, enumMap, primaryNs, nsImports, packageImportPath)
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
}

// writeTransportInterfaceGo generates the Transport interface
func writeTransportInterfaceGo(sb *strings.Builder) {
	sb.WriteString("// Transport is an interface for making JSON-RPC 2.0 calls\n")
	sb.WriteString("type Transport interface {\n")
	sb.WriteString("	Call(method string, params []interface{}) (map[string]interface{}, error)\n")
	sb.WriteString("}\n\n")
}

// writeHTTPTransportGo generates the HTTPTransport struct
func writeHTTPTransportGo(sb *strings.Builder) {
	sb.WriteString("// HTTPTransport implements Transport using HTTP\n")
	sb.WriteString("type HTTPTransport struct {\n")
	sb.WriteString("	baseURL string\n")
	sb.WriteString("	headers map[string]string\n")
	sb.WriteString("	client  *http.Client\n")
	sb.WriteString("}\n\n")

	sb.WriteString("// NewHTTPTransport creates a new HTTPTransport\n")
	sb.WriteString("func NewHTTPTransport(baseURL string, headers map[string]string) *HTTPTransport {\n")
	sb.WriteString("	if headers == nil {\n")
	sb.WriteString("		headers = make(map[string]string)\n")
	sb.WriteString("	}\n")
	sb.WriteString("	return &HTTPTransport{\n")
	sb.WriteString("		baseURL: strings.TrimSuffix(baseURL, \"/\"),\n")
	sb.WriteString("		headers: headers,\n")
	sb.WriteString("		client:  &http.Client{},\n")
	sb.WriteString("	}\n")
	sb.WriteString("}\n\n")

	sb.WriteString("// Call performs a JSON-RPC 2.0 call over HTTP\n")
	sb.WriteString("func (t *HTTPTransport) Call(method string, params []interface{}) (map[string]interface{}, error) {\n")
	sb.WriteString("	requestID := fmt.Sprintf(\"%d\", len(method)+len(params))\n")
	sb.WriteString("	request := map[string]interface{}{\n")
	sb.WriteString("		\"jsonrpc\": \"2.0\",\n")
	sb.WriteString("		\"method\":  method,\n")
	sb.WriteString("		\"params\":  params,\n")
	sb.WriteString("		\"id\":      requestID,\n")
	sb.WriteString("	}\n\n")

	sb.WriteString("	jsonData, err := json.Marshal(request)\n")
	sb.WriteString("	if err != nil {\n")
	sb.WriteString("		return nil, fmt.Errorf(\"failed to marshal request: %w\", err)\n")
	sb.WriteString("	}\n\n")

	sb.WriteString("	req, err := http.NewRequest(\"POST\", t.baseURL, bytes.NewBuffer(jsonData))\n")
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

	sb.WriteString("	var response map[string]interface{}\n")
	sb.WriteString("	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {\n")
	sb.WriteString("		return nil, fmt.Errorf(\"failed to decode response: %w\", err)\n")
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
func writeInterfaceClientGo(sb *strings.Builder, iface *parser.Interface, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, namespace string, nsImports map[string]string, packageImportPath string) {
	if iface.Comment != "" {
		lines := strings.Split(strings.TrimSpace(iface.Comment), "\n")
		for _, line := range lines {
			fmt.Fprintf(sb, "// %s\n", line)
		}
	}

	clientName := iface.Name + "Client"
	fmt.Fprintf(sb, "// %s is a client for the %s interface\n", clientName, iface.Name)
	fmt.Fprintf(sb, "type %s struct {\n", clientName)
	sb.WriteString("	transport         Transport\n")
	sb.WriteString("	contract         *pulserpc.Contract\n")
	sb.WriteString("	validateRequests  bool\n")
	sb.WriteString("	validateResponses bool\n")
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

	// Generate methods
	for _, method := range iface.Methods {
		writeClientMethodGo(sb, iface, method, structMap, enumMap, namespace, nsImports, packageImportPath)
	}
}

// writeClientMethodGo generates a method implementation for a client struct
func writeClientMethodGo(sb *strings.Builder, iface *parser.Interface, method *parser.Method, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, namespace string, nsImports map[string]string, packageImportPath string) {
	methodName := snakeToCamelCase(method.Name)
	fmt.Fprintf(sb, "// %s calls %s.%s\n", methodName, iface.Name, method.Name)
	fmt.Fprintf(sb, "func (c *%sClient) %s(", iface.Name, methodName)

	// Parameters
	for i, param := range method.Parameters {
		if i > 0 {
			sb.WriteString(", ")
		}
		paramType := getGoTypeWithNamespace(param.Type, structMap, enumMap, false, namespace, nsImports, packageImportPath)
		fmt.Fprintf(sb, "%s %s", param.Name, paramType)
	}
	sb.WriteString(") ")

	// Return type
	if method.ReturnType != nil {
		returnType := getGoTypeWithNamespace(method.ReturnType, structMap, enumMap, method.ReturnOptional, namespace, nsImports, packageImportPath)
		fmt.Fprintf(sb, "(%s, error)", returnType)
	} else {
		sb.WriteString("error")
	}
	sb.WriteString(" {\n")

	// Build params array
	sb.WriteString("	params := []interface{}{\n")
	for _, param := range method.Parameters {
		fmt.Fprintf(sb, "		%s,\n", param.Name)
	}
	sb.WriteString("	}\n\n")

	// Validate parameters if enabled
	sb.WriteString("	if c.validateRequests && c.contract != nil {\n")
	sb.WriteString("		if err := c.contract.ValidateRequest(\"" + iface.Name + "\", \"" + method.Name + "\", params); err != nil {\n")
	if method.ReturnType != nil {
		returnType := getGoTypeWithNamespace(method.ReturnType, structMap, enumMap, method.ReturnOptional, namespace, nsImports, packageImportPath)
		sb.WriteString("			var zero ")
		sb.WriteString(returnType)
		sb.WriteString("\n")
		sb.WriteString("			return zero, fmt.Errorf(\"request validation failed: %w\", err)\n")
	} else {
		sb.WriteString("			return fmt.Errorf(\"request validation failed: %w\", err)\n")
	}
	sb.WriteString("		}\n")
	sb.WriteString("	}\n\n")

	// Call transport
	fmt.Fprintf(sb, "	methodName := \"%s.%s\"\n", iface.Name, method.Name)
	sb.WriteString("	response, err := c.transport.Call(methodName, params)\n")
	sb.WriteString("	if err != nil {\n")
	if method.ReturnType != nil {
		sb.WriteString("		var zero ")
		returnType := getGoTypeWithNamespace(method.ReturnType, structMap, enumMap, method.ReturnOptional, namespace, nsImports, packageImportPath)
		sb.WriteString(returnType)
		sb.WriteString("\n")
		sb.WriteString("		return zero, err\n")
	} else {
		sb.WriteString("		return err\n")
	}
	sb.WriteString("	}\n\n")

	// Extract and validate result
	if method.ReturnType != nil {
		sb.WriteString("	result, ok := response[\"result\"]\n")
		sb.WriteString("	if !ok {\n")
		if method.ReturnOptional {
			sb.WriteString("		var zero ")
			returnType := getGoTypeWithNamespace(method.ReturnType, structMap, enumMap, method.ReturnOptional, namespace, nsImports, packageImportPath)
			sb.WriteString(returnType)
			sb.WriteString("\n")
			sb.WriteString("		return zero, nil\n")
		} else {
			sb.WriteString("		var zero ")
			returnType := getGoTypeWithNamespace(method.ReturnType, structMap, enumMap, method.ReturnOptional, namespace, nsImports, packageImportPath)
			sb.WriteString(returnType)
			sb.WriteString("\n")
			sb.WriteString("		return zero, fmt.Errorf(\"missing result in response\")\n")
		}
		sb.WriteString("	}\n\n")

		sb.WriteString("	if c.validateResponses && c.contract != nil {\n")
		sb.WriteString("		if err := c.contract.ValidateResponse(\"" + iface.Name + "\", \"" + method.Name + "\", result); err != nil {\n")
		sb.WriteString("			var zero ")
		goReturnType := getGoTypeWithNamespace(method.ReturnType, structMap, enumMap, method.ReturnOptional, namespace, nsImports, packageImportPath)
		sb.WriteString(goReturnType)
		sb.WriteString("\n")
		sb.WriteString("			return zero, fmt.Errorf(\"response validation failed: %w\", err)\n")
		sb.WriteString("		}\n")
		sb.WriteString("	}\n\n")

		// Deserialize result to typed value
		goReturnType = getGoTypeWithNamespace(method.ReturnType, structMap, enumMap, method.ReturnOptional, namespace, nsImports, packageImportPath)
		sb.WriteString("	var typedResult ")
		sb.WriteString(goReturnType)
		sb.WriteString("\n")
		sb.WriteString("	resultJSON, _ := json.Marshal(result)\n")
		sb.WriteString("	if err := json.Unmarshal(resultJSON, &typedResult); err != nil {\n")
		sb.WriteString("		var zero ")
		sb.WriteString(goReturnType)
		sb.WriteString("\n")
		sb.WriteString("		return zero, fmt.Errorf(\"failed to unmarshal result: %w\", err)\n")
		sb.WriteString("	}\n")
		sb.WriteString("	return typedResult, nil\n")
	} else {
		sb.WriteString("	return nil\n")
	}
	sb.WriteString("}\n\n")
}

// generateTestServerGo generates test_server.go with concrete implementations
func generateTestServerGo(idl *parser.IDL, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, primaryImportPath string, packageImportPath string, namespaceMap map[string]*NamespaceTypes, primaryNs string) string {
	_ = namespaceMap // reserved for future use
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n")
	sb.WriteString("// Test server implementation for integration testing\n\n")
	sb.WriteString("package main\n\n")

	// Determine which imports are needed
	needsMath := false
	needsStrings := false
	for _, iface := range idl.Interfaces {
		for _, method := range iface.Methods {
			methodNameLower := strings.ToLower(method.Name)
			if methodNameLower == "sqrt" {
				needsMath = true
			}
			if methodNameLower == "repeat" {
				needsStrings = true
			}
		}
	}

	// Find cross-namespace imports needed by method signatures
	nsImports := make(map[string]bool)
	for _, iface := range idl.Interfaces {
		for _, method := range iface.Methods {
			for _, param := range method.Parameters {
				findTestServerImports(param.Type, structMap, enumMap, primaryNs, nsImports)
			}
			if method.ReturnType != nil {
				findTestServerImports(method.ReturnType, structMap, enumMap, primaryNs, nsImports)
			}
		}
	}

	sb.WriteString("import (\n")
	if needsMath {
		sb.WriteString("	\"math\"\n")
	}
	if needsStrings {
		sb.WriteString("	\"strings\"\n")
	}
	// Import the generated package using full import path
	fmt.Fprintf(&sb, "	. \"%s\"\n", primaryImportPath)
	// Add cross-namespace imports
	for ns := range nsImports {
		importPath := packageImportPath + "/" + ns
		fmt.Fprintf(&sb, "	. \"%s\"\n", importPath)
	}
	sb.WriteString(")\n\n")

	// Generate implementation structs for each interface
	for _, iface := range idl.Interfaces {
		writeTestInterfaceImplGo(&sb, iface, structMap, enumMap)
	}

	// Generate main function
	sb.WriteString("func main() {\n")
	fmt.Fprintf(&sb, "	server := NewPulseRPCServer(\"0.0.0.0\", 8080)\n")
	for _, iface := range idl.Interfaces {
		implName := iface.Name + "Impl"
		fmt.Fprintf(&sb, "	server.Register(\"%s\", &%s{})\n", iface.Name, implName)
	}
	sb.WriteString("	if err := server.ServeForever(); err != nil {\n")
	sb.WriteString("		panic(err)\n")
	sb.WriteString("	}\n")
	sb.WriteString("}\n")

	return sb.String()
}

// writeTestInterfaceImplGo generates a test implementation struct for an interface
func writeTestInterfaceImplGo(sb *strings.Builder, iface *parser.Interface, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	implName := iface.Name + "Impl"
	fmt.Fprintf(sb, "type %s struct{}\n\n", implName)

	// Generate method implementations
	for _, method := range iface.Methods {
		writeTestMethodImplGo(sb, iface, method, structMap, enumMap)
	}
}

// writeTestMethodImplGo generates a test method implementation
func writeTestMethodImplGo(sb *strings.Builder, iface *parser.Interface, method *parser.Method, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	methodName := snakeToCamelCase(method.Name)
	fmt.Fprintf(sb, "func (i *%sImpl) %s(", iface.Name, methodName)

	// Parameters
	for i, param := range method.Parameters {
		if i > 0 {
			sb.WriteString(", ")
		}
		paramType := mapTypeToGoType(param.Type, structMap, enumMap, false)
		fmt.Fprintf(sb, "%s %s", param.Name, paramType)
	}
	sb.WriteString(") ")

	// Return type
	if method.ReturnType != nil {
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
	case "repeat":
		sb.WriteString("	text := req1.ToRepeat\n")
		sb.WriteString("	count := req1.Count\n")
		sb.WriteString("	if req1.ForceUppercase {\n")
		sb.WriteString("		text = strings.ToUpper(text)\n")
		sb.WriteString("	}\n")
		sb.WriteString("	items := make([]string, count)\n")
		sb.WriteString("	for i := 0; i < count; i++ {\n")
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
		sb.WriteString("	result := make([]int, count)\n")
		sb.WriteString("	for i := 0; i < count; i++ {\n")
		sb.WriteString("		result[i] = num\n")
		sb.WriteString("	}\n")
		sb.WriteString("	return result, nil\n")
	case "putperson":
		sb.WriteString("	return p.PersonId, nil\n")
	default:
		// Default implementation
		if method.ReturnType != nil {
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
func generateTestClientGo(idl *parser.IDL, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, primaryNs string) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n")
	sb.WriteString("// Test client for integration testing\n\n")
	sb.WriteString("package main\n\n")
	sb.WriteString("import (\n")
	sb.WriteString("	\"bytes\"\n")
	sb.WriteString("	\"fmt\"\n")
	sb.WriteString("	\"net/http\"\n")
	sb.WriteString("	\"os\"\n")
	sb.WriteString("	\"time\"\n")
	// Import the generated package (primaryNs)
	fmt.Fprintf(&sb, "	. \"%s\"\n", primaryNs)
	sb.WriteString(")\n\n")

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
		clientName := iface.Name + "Client"
		clientVar := strings.ToLower(iface.Name) + "Client"
		fmt.Fprintf(&sb, "	%s := New%s(transport)\n", clientVar, clientName)
	}
	sb.WriteString("\n")

	sb.WriteString("	errors := []string{}\n\n")

	// Generate test cases for each method
	for _, iface := range idl.Interfaces {
		clientVar := strings.ToLower(iface.Name) + "Client"
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

	// Generate method call
	methodName := snakeToCamelCase(method.Name)
	if len(params) > 0 {
		fmt.Fprintf(sb, "		result, err := %s.%s(%s)\n", clientVar, methodName, strings.Join(params, ", "))
	} else {
		fmt.Fprintf(sb, "		result, err := %s.%s()\n", clientVar, methodName)
	}
	sb.WriteString("		if err != nil {\n")
	fmt.Fprintf(sb, "			errors = append(errors, fmt.Sprintf(\"%s failed: %%v\", err))\n", testName)
	sb.WriteString("			return\n")
	sb.WriteString("		}\n")

	// Generate assertions
	methodNameLower := strings.ToLower(method.Name)
	if iface.Name == "B" && method.Name == "echo" {
		sb.WriteString("		if result == nil || *result != \"test\" {\n")
		fmt.Fprintf(sb, "			errors = append(errors, fmt.Sprintf(\"%s: expected 'test', got %%v\", result))\n", testName)
		sb.WriteString("			return\n")
		sb.WriteString("		}\n")
		sb.WriteString("		// Test null return\n")
		fmt.Fprintf(sb, "		resultNull, _ := %s.Echo(\"return-null\")\n", clientVar)
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

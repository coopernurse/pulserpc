package generator

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/coopernurse/pulserpc/pkg/parser"
	"github.com/coopernurse/pulserpc/pkg/runtime"
)

// TSClientServer is a plugin that generates TypeScript HTTP server and client code from IDL
type TSClientServer struct {
	packageBase string
}

// NewTSClientServer creates a new TSClientServer plugin instance
func NewTSClientServer() *TSClientServer {
	return &TSClientServer{}
}

// Name returns the plugin identifier
func (p *TSClientServer) Name() string {
	return "ts-client-server"
}

// RegisterFlags registers CLI flags for this plugin
func (p *TSClientServer) RegisterFlags(fs *flag.FlagSet) {
	if fs.Lookup("package") == nil {
		fs.String("package", "", "Base module path for generated imports (e.g., @myapp/lib/rpc)")
	}
}

// Generate generates TypeScript HTTP server and client code from the parsed IDL
func (p *TSClientServer) Generate(idl *parser.IDL, fs *flag.FlagSet) error {
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

	// Get package prefix flag
	packageFlag := fs.Lookup("package")
	packagePrefix := ""
	if packageFlag != nil && packageFlag.Value.String() != "" {
		packagePrefix = packageFlag.Value.String()
	}
	p.packageBase = packagePrefix

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

	// Initialize path helpers with package base
	paths := NewTSNamespacePaths(outputDir, p.packageBase)

	// Ensure runtime directory exists
	if err := paths.EnsureRuntimeDir(); err != nil {
		return fmt.Errorf("failed to create runtime directory: %w", err)
	}

	// Copy runtime library files to the resolved runtime directory
	if err := p.copyRuntimeFiles(paths, isSilent()); err != nil {
		return fmt.Errorf("failed to copy runtime files: %w", err)
	}

	// Determine if multi-namespace mode is active
	multiNsMode := isMultiNamespaceMode(outputDir, namespaceMap)

	// Create per-namespace subdirectories when in multi-namespace mode
	if multiNsMode {
		for ns := range namespaceMap {
			if err := paths.EnsureNamespaceDir(ns); err != nil {
				return fmt.Errorf("failed to create namespace directory: %w", err)
			}
		}
	}

	// Write IDL JSON file
	if err := writeIDLJSONTs(idl, outputDir, fs); err != nil {
		return fmt.Errorf("failed to write idl.json: %w", err)
	}

	// Generate per-namespace files when in multi-namespace mode,
	// or flat files for backwards-compatible single-namespace output.
	if multiNsMode {
		// Multi-namespace mode: generate types.ts, server.ts, client.ts per namespace
		for ns, nsTypes := range namespaceMap {
			nsDir := paths.ResolveNamespaceDir(ns)

			// Build namespace-scoped maps for type resolution
			nsStructMap := make(map[string]*parser.Struct)
			for _, s := range nsTypes.Structs {
				nsStructMap[s.Name] = s
			}
			nsEnumMap := make(map[string]*parser.Enum)
			for _, e := range nsTypes.Enums {
				nsEnumMap[e.Name] = e
			}
			nsInterfaceMap := make(map[string]*parser.Interface)
			for _, i := range nsTypes.Interfaces {
				nsInterfaceMap[i.Name] = i
			}

			// Generate types.ts for this namespace
			typesCode := generateTypesTsForNamespace(nsTypes, ns, nsStructMap, nsEnumMap, true, namespaceMap)
			typesPath := filepath.Join(nsDir, "types.ts")
			if err := os.WriteFile(typesPath, []byte(typesCode), 0644); err != nil {
				return fmt.Errorf("failed to write %s/types.ts: %w", ns, err)
			}
			PrintFileCreated(typesPath, fs)

			// Generate server.ts for this namespace
			serverCode := generateServerTsForNamespace(nsTypes, nsStructMap, nsEnumMap, nsInterfaceMap, packagePrefix, true, namespaceMap)
			serverPath := filepath.Join(nsDir, "server.ts")
			if err := os.WriteFile(serverPath, []byte(serverCode), 0644); err != nil {
				return fmt.Errorf("failed to write %s/server.ts: %w", ns, err)
			}
			PrintFileCreated(serverPath, fs)

			// Generate client.ts for this namespace
			clientCode := generateClientTsForNamespace(nsTypes, nsStructMap, nsEnumMap, packagePrefix, true, namespaceMap)
			clientPath := filepath.Join(nsDir, "client.ts")
			if err := os.WriteFile(clientPath, []byte(clientCode), 0644); err != nil {
				return fmt.Errorf("failed to write %s/client.ts: %w", ns, err)
			}
			PrintFileCreated(clientPath, fs)

			// Generate index.ts for this namespace (re-exports from types, server, client)
			if err := generateNamespaceIndexTs(paths, ns); err != nil {
				return fmt.Errorf("failed to write %s/index.ts: %w", ns, err)
			}
			indexPath := filepath.Join(nsDir, "index.ts")
			PrintFileCreated(indexPath, fs)
		}
	} else {
		// Backwards-compatible flat output: generate single types.ts, server.ts, client.ts
		typesCode := generateTypesTs(idl, structMap, enumMap)
		typesPath := filepath.Join(outputDir, "types.ts")
		if err := os.WriteFile(typesPath, []byte(typesCode), 0644); err != nil {
			return fmt.Errorf("failed to write types.ts: %w", err)
		}
		PrintFileCreated(typesPath, fs)

		serverCode := generateServerTs(idl, structMap, enumMap, interfaceMap, packagePrefix, namespaceMap)
		serverPath := filepath.Join(outputDir, "server.ts")
		if err := os.WriteFile(serverPath, []byte(serverCode), 0644); err != nil {
			return fmt.Errorf("failed to write server.ts: %w", err)
		}
		PrintFileCreated(serverPath, fs)

		clientCode := generateClientTs(idl, structMap, enumMap, packagePrefix, namespaceMap)
		clientPath := filepath.Join(outputDir, "client.ts")
		if err := os.WriteFile(clientPath, []byte(clientCode), 0644); err != nil {
			return fmt.Errorf("failed to write client.ts: %w", err)
		}
		PrintFileCreated(clientPath, fs)
	}

	// Check if generate-test-files flag is set
	generateTestFilesFlag := fs.Lookup("generate-test-files")
	generateTestServer := generateTestFilesFlag != nil && generateTestFilesFlag.Value.String() == "true"

	// Generate test server and client if flag is set
	if generateTestServer {
		// Generate test_server.ts
		testServerCode := generateTestServerTs(idl, structMap, enumMap, interfaceMap, packagePrefix, namespaceMap)
		testServerPath := filepath.Join(outputDir, "test_server.ts")
		if err := os.WriteFile(testServerPath, []byte(testServerCode), 0644); err != nil {
			return fmt.Errorf("failed to write test_server.ts: %w", err)
		}
		PrintFileCreated(testServerPath, fs)

		// Generate test_client.ts
		testClientCode := generateTestClientTs(idl, structMap, enumMap, interfaceMap, packagePrefix, namespaceMap)
		testClientPath := filepath.Join(outputDir, "test_client.ts")
		if err := os.WriteFile(testClientPath, []byte(testClientCode), 0644); err != nil {
			return fmt.Errorf("failed to write test_client.ts: %w", err)
		}
		PrintFileCreated(testClientPath, fs)
	}

	return nil
}

// copyRuntimeFiles copies the TypeScript runtime library files to the output directory
// Uses embedded runtime files from the binary
func (p *TSClientServer) copyRuntimeFiles(paths TSNamespacePaths, silent bool) error {
	runtimeDir := paths.ResolveRuntimeDir()
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		return fmt.Errorf("failed to create runtime directory: %w", err)
	}

	files, err := runtime.GetRuntimeFiles("ts")
	if err != nil {
		return err
	}

	for filename, data := range files {
		dstPath := filepath.Join(runtimeDir, filename)
		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write runtime file %s: %w", dstPath, err)
		}
		if !silent {
			fmt.Println(dstPath)
		}
	}

	return nil
}

// writeIDLJSONTs writes the IDL metadata as JSON to idl.json
//
// IDL Placement Note (2026-04-01):
// This function intentionally writes idl.json directly to outputDir (the root -dir),
// NOT to a namespace subdirectory. The idl.json file is consumed by the runtime
// Contract class at startup, and it must be accessible as {root}/idl.json regardless
// of which namespace mode is active. This mirrors the behavior of the original
// single-namespace generator and ensures the runtime can locate the contract.
func writeIDLJSONTs(idl *parser.IDL, outputDir string, fs *flag.FlagSet) error {
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

// collectCrossNamespaceImports identifies which external namespaces are referenced
// by types in the given namespace's structs and enums. Returns a map of namespace
// names that need to be imported.
func collectCrossNamespaceImports(nsTypes *NamespaceTypes, currentNs string, allNamespaceMap map[string]*NamespaceTypes) map[string]string {
	imports := make(map[string]string)

	localTypes := make(map[string]bool)
	for _, s := range nsTypes.Structs {
		localTypes[GetBaseName(s.Name)] = true
	}
	for _, e := range nsTypes.Enums {
		localTypes[GetBaseName(e.Name)] = true
	}

	for _, s := range nsTypes.Structs {
		// Check extends clause
		if s.Extends != "" {
			typeName := GetBaseName(s.Extends)
			if !localTypes[typeName] {
				for ns, nsTypes := range allNamespaceMap {
					if ns == currentNs {
						continue
					}
					for _, otherStruct := range nsTypes.Structs {
						if GetBaseName(otherStruct.Name) == typeName {
							imports[ns] = ns + "Types"
							break
						}
					}
				}
			}
		}

		// Check struct fields
		for _, field := range s.Fields {
			if field.Type != nil && field.Type.IsUserDefined() {
				typeName := GetBaseName(field.Type.UserDefined)
				if !localTypes[typeName] {
					for ns, nsTypes := range allNamespaceMap {
						if ns == currentNs {
							continue
						}
						for _, otherStruct := range nsTypes.Structs {
							if GetBaseName(otherStruct.Name) == typeName {
								imports[ns] = ns + "Types"
								break
							}
						}
						if _, ok := imports[ns]; !ok {
							for _, otherEnum := range nsTypes.Enums {
								if GetBaseName(otherEnum.Name) == typeName {
									imports[ns] = ns + "Types"
									break
								}
							}
						}
					}
				}
			}
		}
	}

	return imports
}

// getExtendsTypeForNamespace returns the proper TypeScript type reference for an extends clause.
// When the parent type is from another namespace, it returns "namespace.TypeName" format.
func getExtendsTypeForNamespace(extendsType string, _ map[string]*parser.Struct, _ map[string]*parser.Enum, currentNs string, _ map[string]*NamespaceTypes) string {
	baseName := GetBaseName(extendsType)

	// Check if it's a local type (no namespace or same namespace)
	if !strings.Contains(extendsType, ".") {
		return baseName
	}

	// Check if the extends type belongs to this namespace
	nsPrefix := GetNamespaceFromType(extendsType, "")
	if nsPrefix == currentNs {
		return baseName
	}

	// It's from another namespace - use namespace.TypeName format
	if strings.Contains(extendsType, ".") {
		parts := strings.Split(extendsType, ".")
		if len(parts) >= 2 {
			return parts[len(parts)-2] + "." + baseName
		}
	}

	return baseName
}

// getTypeScriptTypeForNamespace converts a parser.Type to a TypeScript type string,
// with support for cross-namespace type references in multi-namespace mode.
// If useTypesPrefix is true, user-defined types are prefixed with "types." (for use in server.ts).
// When inNamespaceSubdir is true and the type belongs to another namespace, it prefixes with that namespace.
func getTypeScriptTypeForNamespace(t *parser.Type, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, useTypesPrefix bool, inNamespaceSubdir bool, allNamespaceMap map[string]*NamespaceTypes) string {
	if t == nil {
		return "void"
	}
	if t.IsBuiltIn() {
		switch t.BuiltIn {
		case "string":
			return "string"
		case "int", "float":
			return "number"
		case "bool":
			return "boolean"
		default:
			return "any"
		}
	}
	if t.IsArray() {
		return getTypeScriptTypeForNamespace(t.Array, structMap, enumMap, useTypesPrefix, inNamespaceSubdir, allNamespaceMap) + "[]"
	}
	if t.IsMap() {
		return "Record<string, " + getTypeScriptTypeForNamespace(t.MapValue, structMap, enumMap, useTypesPrefix, inNamespaceSubdir, allNamespaceMap) + ">"
	}
	if t.IsUserDefined() {
		typeName := t.UserDefined
		// Check if it's a struct
		if structMap[typeName] != nil {
			if useTypesPrefix {
				return "types." + GetBaseName(typeName)
			}
			return typeName
		}
		// Check if it's an enum
		if enumMap[typeName] != nil {
			if useTypesPrefix {
				return "types." + GetBaseName(typeName)
			}
			return typeName
		}
		// Try to find by base name (for namespaced types like "inc.Response")
		for key := range structMap {
			if strings.HasSuffix(key, "."+typeName) {
				if useTypesPrefix {
					return "types." + key
				}
				return key
			}
		}
		for key := range enumMap {
			if strings.HasSuffix(key, "."+typeName) {
				if useTypesPrefix {
					return "types." + GetBaseName(key)
				}
				return GetBaseName(key)
			}
		}

		// In multi-namespace mode, check if this type belongs to another namespace
		if inNamespaceSubdir && allNamespaceMap != nil {
			baseTypeName := GetBaseName(typeName)
			for ns, nsTypes := range allNamespaceMap {
				for _, s := range nsTypes.Structs {
					if GetBaseName(s.Name) == baseTypeName {
						return ns + "." + baseTypeName
					}
				}
				for _, e := range nsTypes.Enums {
					if GetBaseName(e.Name) == baseTypeName {
						return ns + "." + baseTypeName
					}
				}
			}
		}

		if useTypesPrefix {
			return "types." + GetBaseName(typeName)
		}
		return GetBaseName(typeName)
	}
	return "any"
}

// Step 10 of the multi-namespace implementation spec:
// The -package flag no longer affects class names. It now only serves as the base module
// path for generated imports (e.g., @myapp/lib/rpc). The previous class-name-prefix behavior
// has been removed as no existing test or quickstart used it with a non-empty value.

// generateServerTs generates the server.ts file with abstract interface classes only
func generateServerTs(idl *parser.IDL, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, _ map[string]*parser.Interface, _ string, _ map[string]*NamespaceTypes) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString("// Abstract service classes\n")
	sb.WriteString("// Implement these classes to create your service\n\n")
	sb.WriteString("import { RPCError } from './pulserpc/rpc';\n")
	sb.WriteString("import * as types from './types';\n\n")

	// Generate interface stub abstract classes
	for _, iface := range idl.Interfaces {
		writeInterfaceStubTs(&sb, iface, structMap, enumMap)
	}

	return sb.String()
}

// writeInterfaceStubTs generates an abstract class for an interface
func writeInterfaceStubTs(sb *strings.Builder, iface *parser.Interface, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	if iface.Comment != "" {
		lines := strings.Split(strings.TrimSpace(iface.Comment), "\n")
		for _, line := range lines {
			fmt.Fprintf(sb, "// %s\n", line)
		}
	}
	fmt.Fprintf(sb, "export abstract class %s {\n", iface.Name)

	for _, method := range iface.Methods {
		fmt.Fprintf(sb, "  abstract %s(", method.Name)
		for i, param := range method.Parameters {
			if i > 0 {
				sb.WriteString(", ")
			}
			tsType := getTypeScriptType(param.Type, structMap, enumMap, true)
			fmt.Fprintf(sb, "%s: %s", param.Name, tsType)
		}
		returnType := getTypeScriptType(method.ReturnType, structMap, enumMap, true)
		if method.ReturnOptional {
			returnType = returnType + " | null"
		}
		sb.WriteString("): " + returnType + ";\n")
	}
	sb.WriteString("}\n\n")
}

// writeInterfaceStubTsForNamespace generates an abstract class for an interface
// with support for cross-namespace type references.
func writeInterfaceStubTsForNamespace(sb *strings.Builder, iface *parser.Interface, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, _ string, allNamespaceMap map[string]*NamespaceTypes) {
	if iface.Comment != "" {
		lines := strings.Split(strings.TrimSpace(iface.Comment), "\n")
		for _, line := range lines {
			fmt.Fprintf(sb, "// %s\n", line)
		}
	}
	fmt.Fprintf(sb, "export abstract class %s {\n", iface.Name)

	for _, method := range iface.Methods {
		fmt.Fprintf(sb, "  abstract %s(", method.Name)
		for i, param := range method.Parameters {
			if i > 0 {
				sb.WriteString(", ")
			}
			tsType := getTypeScriptTypeForNamespace(param.Type, structMap, enumMap, true, true, allNamespaceMap)
			fmt.Fprintf(sb, "%s: %s", param.Name, tsType)
		}
		returnType := getTypeScriptTypeForNamespace(method.ReturnType, structMap, enumMap, true, true, allNamespaceMap)
		if method.ReturnOptional {
			returnType = returnType + " | null"
		}
		sb.WriteString("): " + returnType + ";\n")
	}
	sb.WriteString("}\n\n")
}

// getTypeScriptType converts a parser.Type to a TypeScript type string
// If useTypesPrefix is true, user-defined types are prefixed with "types." (for use in server.ts)
func getTypeScriptType(t *parser.Type, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, useTypesPrefix bool) string {
	if t == nil {
		return "void"
	}
	if t.IsBuiltIn() {
		switch t.BuiltIn {
		case "string":
			return "string"
		case "int", "float":
			return "number"
		case "bool":
			return "boolean"
		default:
			return "any"
		}
	}
	if t.IsArray() {
		return getTypeScriptType(t.Array, structMap, enumMap, useTypesPrefix) + "[]"
	}
	if t.IsMap() {
		return "Record<string, " + getTypeScriptType(t.MapValue, structMap, enumMap, useTypesPrefix) + ">"
	}
	if t.IsUserDefined() {
		typeName := t.UserDefined
		// Check if it's a struct
		if structMap[typeName] != nil {
			if useTypesPrefix {
				return "types." + GetBaseName(typeName)
			}
			return typeName
		}
		// Check if it's an enum
		if enumMap[typeName] != nil {
			if useTypesPrefix {
				return "types." + GetBaseName(typeName)
			}
			return typeName
		}
		// Try to find by base name (for namespaced types like "inc.Response")
		for key := range structMap {
			if strings.HasSuffix(key, "."+typeName) {
				if useTypesPrefix {
					return "types." + key
				}
				return key
			}
		}
		for key := range enumMap {
			if strings.HasSuffix(key, "."+typeName) {
				if useTypesPrefix {
					return "types." + GetBaseName(key)
				}
				return GetBaseName(key)
			}
		}
		if useTypesPrefix {
			return "types." + GetBaseName(typeName)
		}
		return typeName
	}
	return "any"
}

// generateTypesTs generates a types.ts file with TypeScript interfaces for structs and enums
func generateTypesTs(idl *parser.IDL, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString("// TypeScript interfaces and enums for all IDL types\n\n")

	// Generate enums first (they may be used by structs)
	for _, enum := range idl.Enums {
		comment := strings.TrimSpace(enum.Comment)
		if comment != "" {
			lines := strings.Split(comment, "\n")
			for _, line := range lines {
				fmt.Fprintf(&sb, "// %s\n", line)
			}
		}
		fmt.Fprintf(&sb, "export enum %s {\n", GetBaseName(enum.Name))
		for i, val := range enum.Values {
			valComment := strings.TrimSpace(val.Comment)
			if valComment != "" {
				fmt.Fprintf(&sb, "  // %s\n", valComment)
			}
			fmt.Fprintf(&sb, "  %s = \"%s\"", val.Name, val.Name)
			if i < len(enum.Values)-1 {
				sb.WriteString(",\n")
			} else {
				sb.WriteString("\n")
			}
		}
		sb.WriteString("}\n\n")
	}

	// Generate structs (with extends support)
	for _, structDef := range idl.Structs {
		comment := strings.TrimSpace(structDef.Comment)
		if comment != "" {
			lines := strings.Split(comment, "\n")
			for _, line := range lines {
				fmt.Fprintf(&sb, "// %s\n", line)
			}
		}
		fmt.Fprintf(&sb, "export interface %s", structDef.Name)
		if structDef.Extends != "" {
			// Handle extends - use just the base name if it's namespaced
			baseName := structDef.Extends
			if strings.Contains(baseName, ".") {
				parts := strings.Split(baseName, ".")
				baseName = parts[len(parts)-1]
			}
			sb.WriteString(" extends " + baseName)
		}
		sb.WriteString(" {\n")
		for _, field := range structDef.Fields {
			fieldComment := strings.TrimSpace(field.Comment)
			if fieldComment != "" {
				fmt.Fprintf(&sb, "  // %s\n", fieldComment)
			}
			tsType := getTypeScriptType(field.Type, structMap, enumMap, false)
			optionalMarker := ""
			if field.Optional {
				optionalMarker = "?"
			}
			fmt.Fprintf(&sb, "  %s%s: %s;\n", field.Name, optionalMarker, tsType)
		}
		sb.WriteString("}\n\n")
	}

	return sb.String()
}

// generateTypesTsForNamespace generates a types.ts file with TypeScript interfaces for structs and enums
// belonging to a single namespace. Used in multi-namespace mode.
// When inNamespaceSubdir is true, cross-namespace type references use '../{namespace}' imports.
func generateTypesTsForNamespace(nsTypes *NamespaceTypes, currentNs string, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, inNamespaceSubdir bool, allNamespaceMap map[string]*NamespaceTypes) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString("// TypeScript interfaces and enums for all IDL types\n\n")

	// Collect cross-namespace imports needed by types in this namespace
	crossNsImports := collectCrossNamespaceImports(nsTypes, currentNs, allNamespaceMap)

	// Write cross-namespace imports if any
	if inNamespaceSubdir && len(crossNsImports) > 0 {
		importedNs := make([]string, 0, len(crossNsImports))
		for ns := range crossNsImports {
			importedNs = append(importedNs, ns)
		}
		// Sort for deterministic output
		for i := 0; i < len(importedNs); i++ {
			for j := i + 1; j < len(importedNs); j++ {
				if importedNs[i] > importedNs[j] {
					importedNs[i], importedNs[j] = importedNs[j], importedNs[i]
				}
			}
		}
		for _, ns := range importedNs {
			importPath := tsCrossNamespaceImportPath("", ns)
			fmt.Fprintf(&sb, "import * as %s from '%s';\n", ns, importPath)
		}
		sb.WriteString("\n")
	}

	// Generate enums first (they may be used by structs)
	for _, enum := range nsTypes.Enums {
		comment := strings.TrimSpace(enum.Comment)
		if comment != "" {
			lines := strings.Split(comment, "\n")
			for _, line := range lines {
				fmt.Fprintf(&sb, "// %s\n", line)
			}
		}
		fmt.Fprintf(&sb, "export enum %s {\n", GetBaseName(enum.Name))
		for i, val := range enum.Values {
			valComment := strings.TrimSpace(val.Comment)
			if valComment != "" {
				fmt.Fprintf(&sb, "  // %s\n", valComment)
			}
			fmt.Fprintf(&sb, "  %s = \"%s\"", val.Name, val.Name)
			if i < len(enum.Values)-1 {
				sb.WriteString(",\n")
			} else {
				sb.WriteString("\n")
			}
		}
		sb.WriteString("}\n\n")
	}

	// Generate structs (with extends support)
	for _, structDef := range nsTypes.Structs {
		comment := strings.TrimSpace(structDef.Comment)
		if comment != "" {
			lines := strings.Split(comment, "\n")
			for _, line := range lines {
				fmt.Fprintf(&sb, "// %s\n", line)
			}
		}
		baseName := GetBaseName(structDef.Name)
		fmt.Fprintf(&sb, "export interface %s", baseName)
		if structDef.Extends != "" {
			extendsRef := getExtendsTypeForNamespace(structDef.Extends, structMap, enumMap, currentNs, allNamespaceMap)
			sb.WriteString(" extends " + extendsRef)
		}
		sb.WriteString(" {\n")
		for _, field := range structDef.Fields {
			fieldComment := strings.TrimSpace(field.Comment)
			if fieldComment != "" {
				fmt.Fprintf(&sb, "  // %s\n", fieldComment)
			}
			tsType := getTypeScriptTypeForNamespace(field.Type, structMap, enumMap, false, inNamespaceSubdir, allNamespaceMap)
			optionalMarker := ""
			if field.Optional {
				optionalMarker = "?"
			}
			fmt.Fprintf(&sb, "  %s%s: %s;\n", field.Name, optionalMarker, tsType)
		}
		sb.WriteString("}\n\n")
	}

	return sb.String()
}

// generateServerTsForNamespace generates the server.ts file with abstract interface classes
// for a single namespace. Used in multi-namespace mode.
func generateServerTsForNamespace(nsTypes *NamespaceTypes, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, _ map[string]*parser.Interface, _ string, inNamespaceSubdir bool, allNamespaceMap map[string]*NamespaceTypes) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString("// Abstract service classes\n")
	sb.WriteString("// Implement these classes to create your service\n\n")
	runtimeImport := tsRuntimeImportPath(inNamespaceSubdir)
	sb.WriteString(fmt.Sprintf("import { RPCError } from '%s/rpc';\n", runtimeImport))
	sb.WriteString("import * as types from './types';\n\n")

	currentNs := ""
	for ns := range allNamespaceMap {
		if allNamespaceMap[ns] == nsTypes {
			currentNs = ns
			break
		}
	}

	crossNsImports := collectCrossNamespaceImports(nsTypes, currentNs, allNamespaceMap)
	for crossNs := range crossNsImports {
		sb.WriteString(fmt.Sprintf("import * as %s from '../%s/types';\n", crossNs, crossNs))
	}
	if len(crossNsImports) > 0 {
		sb.WriteString("\n")
	}

	for _, iface := range nsTypes.Interfaces {
		writeInterfaceStubTsForNamespace(&sb, iface, structMap, enumMap, currentNs, allNamespaceMap)
	}

	return sb.String()
}

// generateClientTsForNamespace generates the client.ts file with static typed client classes
// for a single namespace. Used in multi-namespace mode.
func generateClientTsForNamespace(nsTypes *NamespaceTypes, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, _ string, inNamespaceSubdir bool, allNamespaceMap map[string]*NamespaceTypes) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString("// Static typed client stubs for each interface\n")
	sb.WriteString("// Use these for compile-time type safety\n\n")

	runtimeImport := tsRuntimeImportPath(inNamespaceSubdir)
	fmt.Fprintf(&sb, "import { Transport, HttpTransport } from '%s/transport';\n", runtimeImport)
	fmt.Fprintf(&sb, "import { RPCError } from '%s/rpc';\n", runtimeImport)
	sb.WriteString("import * as types from './types';\n\n")

	currentNs := ""
	for ns := range allNamespaceMap {
		if allNamespaceMap[ns] == nsTypes {
			currentNs = ns
			break
		}
	}

	crossNsImports := collectCrossNamespaceImports(nsTypes, currentNs, allNamespaceMap)
	for crossNs := range crossNsImports {
		sb.WriteString(fmt.Sprintf("import * as %s from '../%s/types';\n", crossNs, crossNs))
	}
	if len(crossNsImports) > 0 {
		sb.WriteString("\n")
	}

	sb.WriteString("export { Transport, HttpTransport };\n\n")

	for _, iface := range nsTypes.Interfaces {
		writeInterfaceClientTs(&sb, iface, structMap, enumMap, currentNs, allNamespaceMap)
	}

	return sb.String()
}

// generateClientTs generates the client.ts file with static typed client classes
func generateClientTs(idl *parser.IDL, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, _ string, _ map[string]*NamespaceTypes) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString("// Static typed client stubs for each interface\n")
	sb.WriteString("// Use these for compile-time type safety\n\n")

	sb.WriteString("import { Transport, HttpTransport } from './pulserpc/transport';\n")
	sb.WriteString("import { RPCError } from './pulserpc/rpc';\n")
	sb.WriteString("import * as types from './types';\n\n")

	sb.WriteString("export { Transport, HttpTransport };\n\n")

	for _, iface := range idl.Interfaces {
		writeInterfaceClientTs(&sb, iface, structMap, enumMap, "", nil)
	}

	return sb.String()
}

// writeClientMethodTs generates a typed method for a TypeScript client class
func writeClientMethodTs(sb *strings.Builder, iface *parser.Interface, method *parser.Method, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, ns string, allNamespaceMap map[string]*NamespaceTypes) {
	methodName := method.Name
	fmt.Fprintf(sb, "  async %s(", methodName)

	// Parameters
	for i, param := range method.Parameters {
		if i > 0 {
			sb.WriteString(", ")
		}
		tsType := getTypeScriptTypeForNamespace(param.Type, structMap, enumMap, true, ns != "", allNamespaceMap)
		fmt.Fprintf(sb, "%s: %s", param.Name, tsType)
	}
	sb.WriteString(")")

	// Return type
	if method.ReturnType != nil {
		returnType := getTypeScriptTypeForNamespace(method.ReturnType, structMap, enumMap, true, ns != "", allNamespaceMap)
		if method.ReturnOptional {
			fmt.Fprintf(sb, ": Promise<%s | null> {\n", returnType)
		} else {
			fmt.Fprintf(sb, ": Promise<%s> {\n", returnType)
		}
	} else {
		sb.WriteString(": Promise<void> {\n")
	}

	// Build request
	fmt.Fprintf(sb, "    const req = {\n")
	sb.WriteString("      jsonrpc: \"2.0\" as const,\n")
	fmt.Fprintf(sb, "      method: \"%s.%s\",\n", iface.Name, method.Name)
	if len(method.Parameters) > 0 {
		sb.WriteString("      params: [")
		for i, param := range method.Parameters {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(param.Name)
		}
		sb.WriteString("],\n")
	}
	sb.WriteString("    };\n")

	sb.WriteString("    const resp = await this.transport.request(req as any);\n")
	sb.WriteString("    if (resp.error) {\n")
	sb.WriteString("      throw new RPCError(resp.error.code, resp.error.message, resp.error.data);\n")
	sb.WriteString("    }\n")

	if method.ReturnType != nil {
		if method.ReturnOptional {
			sb.WriteString("    return resp.result as ")
			returnType := getTypeScriptTypeForNamespace(method.ReturnType, structMap, enumMap, true, ns != "", allNamespaceMap)
			fmt.Fprintf(sb, "%s | null;\n", returnType)
		} else {
			sb.WriteString("    return resp.result as ")
			returnType := getTypeScriptTypeForNamespace(method.ReturnType, structMap, enumMap, true, ns != "", allNamespaceMap)
			fmt.Fprintf(sb, "%s;\n", returnType)
		}
	} else {
		sb.WriteString("    return;\n")
	}

	sb.WriteString("  }\n\n")
}

// writeInterfaceClientTs generates a client class for a TypeScript interface
func writeInterfaceClientTs(sb *strings.Builder, iface *parser.Interface, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, ns string, allNamespaceMap map[string]*NamespaceTypes) {
	if iface.Comment != "" {
		lines := strings.Split(strings.TrimSpace(iface.Comment), "\n")
		for _, line := range lines {
			fmt.Fprintf(sb, "// %s\n", line)
		}
	}

	clientName := iface.Name + "Client"
	fmt.Fprintf(sb, "export class %s {\n", clientName)
	sb.WriteString("  constructor(private transport: Transport) {}\n\n")

	// Generate methods
	for _, method := range iface.Methods {
		writeClientMethodTs(sb, iface, method, structMap, enumMap, ns, allNamespaceMap)
	}

	sb.WriteString("}\n\n")
}

// generateNamespaceIndexTs writes an index.ts file to the namespace subdirectory
// that re-exports from types.ts, server.ts, and client.ts.
func generateNamespaceIndexTs(paths TSNamespacePaths, namespace string) error {
	nsDir := paths.ResolveNamespaceDir(namespace)
	indexContent := "export * from './types';\nexport * from './server';\nexport * from './client';\n"
	indexPath := filepath.Join(nsDir, "index.ts")
	if err := os.WriteFile(indexPath, []byte(indexContent), 0644); err != nil {
		return fmt.Errorf("failed to write %s/index.ts: %w", namespace, err)
	}
	return nil
}

// generateTestServerTs generates test_server.ts with concrete implementations of all interfaces
func generateTestServerTs(idl *parser.IDL, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, _ map[string]*parser.Interface, _ string, _ map[string]*NamespaceTypes) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n")
	sb.WriteString("// Test server implementation for integration testing\n\n")
	sb.WriteString("import { readFileSync } from 'fs';\n")
	sb.WriteString("import { Server, Contract } from './pulserpc';\n")

	for _, iface := range idl.Interfaces {
		ns := GetNamespaceFromType(iface.Name, iface.Namespace)
		if ns != "" {
			fmt.Fprintf(&sb, "import { %s } from './%s/server';\n", iface.Name, ns)
		} else {
			fmt.Fprintf(&sb, "import { %s } from './server';\n", iface.Name)
		}
	}
	sb.WriteString("\n")

	// Generate implementation classes for each interface
	for _, iface := range idl.Interfaces {
		writeTestInterfaceImplTs(&sb, iface, structMap, enumMap)
	}

	// Generate main entry point
	sb.WriteString("// Load IDL and create Contract\n")
	sb.WriteString("const idlData = JSON.parse(readFileSync('idl.json', 'utf-8'));\n")
	sb.WriteString("const contract = new Contract(idlData);\n\n")
	sb.WriteString("// Create Server instance\n")
	sb.WriteString("const rpcServer = new Server({ contract, validateRequests: true, validateResponses: true });\n")
	for _, iface := range idl.Interfaces {
		fmt.Fprintf(&sb, "rpcServer.addHandler(\"%s\", new %sImpl());\n", iface.Name, iface.Name)
	}

	// Generate HTTP server handler
	sb.WriteString("\n")
	sb.WriteString("// HTTP server\n")
	sb.WriteString("import * as http from 'http';\n\n")
	sb.WriteString("class TestRPCHandler {\n")
	sb.WriteString("  private rpcServer: Server;\n\n")
	sb.WriteString("  constructor(rpcServer: Server) {\n")
	sb.WriteString("    this.rpcServer = rpcServer;\n")
	sb.WriteString("  }\n\n")
	sb.WriteString("  handle(req: http.IncomingMessage, res: http.ServerResponse): void {\n")
	sb.WriteString("    let body = '';\n")
	sb.WriteString("    req.on('data', (chunk) => { body += chunk.toString(); });\n")
	sb.WriteString("    req.on('end', () => {\n")
	sb.WriteString("      try {\n")
	sb.WriteString("        const data = JSON.parse(body);\n")
	sb.WriteString("        const response = this.rpcServer.call(data);\n")
	sb.WriteString("        if (response === null || response === undefined) {\n")
	sb.WriteString("          res.writeHead(204);\n")
	sb.WriteString("          res.end();\n")
	sb.WriteString("        } else {\n")
	sb.WriteString("          res.writeHead(200, { 'Content-Type': 'application/json' });\n")
	sb.WriteString("          res.end(JSON.stringify(response));\n")
	sb.WriteString("        }\n")
	sb.WriteString("      } catch (err: any) {\n")
	sb.WriteString("        const errorResponse = {\n")
	sb.WriteString("          jsonrpc: '2.0',\n")
	sb.WriteString("          error: { code: -32700, message: `Parse error: ${err.message}` },\n")
	sb.WriteString("          id: null,\n")
	sb.WriteString("        };\n")
	sb.WriteString("        res.writeHead(200, { 'Content-Type': 'application/json' });\n")
	sb.WriteString("        res.end(JSON.stringify(errorResponse));\n")
	sb.WriteString("      }\n")
	sb.WriteString("    });\n")
	sb.WriteString("  }\n")
	sb.WriteString("}\n\n")
	sb.WriteString("const handler = new TestRPCHandler(rpcServer);\n")
	sb.WriteString("const httpServer = http.createServer((req, res) => {\n")
	sb.WriteString("  if (req.method === 'POST') {\n")
	sb.WriteString("    handler.handle(req, res);\n")
	sb.WriteString("  } else {\n")
	sb.WriteString("    res.writeHead(405, { 'Content-Type': 'application/json' });\n")
	sb.WriteString("    res.end(JSON.stringify({ error: 'Method Not Allowed' }));\n")
	sb.WriteString("  }\n")
	sb.WriteString("});\n\n")
	sb.WriteString("httpServer.listen(8080, '0.0.0.0', () => {\n")
	sb.WriteString("  console.log('PulseRPC test server listening on http://0.0.0.0:8080');\n")
	sb.WriteString("});\n")

	return sb.String()
}

// writeTestInterfaceImplTs generates a test implementation class for an interface
func writeTestInterfaceImplTs(sb *strings.Builder, iface *parser.Interface, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	fmt.Fprintf(sb, "class %sImpl extends %s {\n", iface.Name, iface.Name)
	fmt.Fprintf(sb, "  // Test implementation of %s interface\n\n", iface.Name)

	// Generate method implementations
	for _, method := range iface.Methods {
		writeTestMethodImplTs(sb, iface, method, structMap, enumMap)
	}
	sb.WriteString("}\n\n")
}

// writeTestMethodImplTs generates a test implementation for a method
func writeTestMethodImplTs(sb *strings.Builder, iface *parser.Interface, method *parser.Method, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	// Method signature
	fmt.Fprintf(sb, "  %s(", method.Name)
	for i, param := range method.Parameters {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(sb, "%s: any", param.Name)
	}
	sb.WriteString("): any {\n")

	// Special handling for known test cases
	if iface.Name == "B" && method.Name == "echo" {
		sb.WriteString("    // Handle optional return: return null if s === 'return-null'\n")
		sb.WriteString("    if (s === 'return-null') {\n")
		sb.WriteString("      return null;\n")
		sb.WriteString("    }\n")
		sb.WriteString("    return s;\n")
		sb.WriteString("  }\n\n")
		return
	}

	// Generate based on method name patterns
	methodNameLower := strings.ToLower(method.Name)
	switch methodNameLower {
	case "add":
		sb.WriteString("    // returns a+b\n")
		sb.WriteString("    return a + b;\n")
	case "sqrt":
		sb.WriteString("    // returns the square root of a\n")
		sb.WriteString("    return globalThis.Math.sqrt(a);\n")
	case "calc":
		sb.WriteString("    // performs the given operation against all the values in nums and returns the result\n")
		sb.WriteString("    if (!nums || nums.length === 0) {\n")
		sb.WriteString("      return 0.0;\n")
		sb.WriteString("    }\n")
		sb.WriteString("    if (operation === 'add') {\n")
		sb.WriteString("      return nums.reduce((sum, num) => sum + num, 0.0);\n")
		sb.WriteString("    } else if (operation === 'multiply') {\n")
		sb.WriteString("      return nums.reduce((prod, num) => prod * num, 1.0);\n")
		sb.WriteString("    } else {\n")
		sb.WriteString("      return 0.0;\n")
		sb.WriteString("    }\n")
	case "repeat":
		sb.WriteString("    // Echos the req1.to_repeat string as a list, optionally forcing to_repeat to upper case\n")
		sb.WriteString("    // RepeatResponse.items should be a list of strings whose length is equal to req1.count\n")
		sb.WriteString("    const text = req1.to_repeat || '';\n")
		sb.WriteString("    const count = req1.count || 0;\n")
		sb.WriteString("    const forceUppercase = req1.force_uppercase || false;\n")
		sb.WriteString("    const finalText = forceUppercase ? text.toUpperCase() : text;\n")
		sb.WriteString("    const items: string[] = [];\n")
		sb.WriteString("    for (let i = 0; i < count; i++) {\n")
		sb.WriteString("      items.push(finalText);\n")
		sb.WriteString("    }\n")
		sb.WriteString("    return {\n")
		sb.WriteString("      status: 'ok',\n")
		sb.WriteString("      count: count,\n")
		sb.WriteString("      items: items,\n")
		sb.WriteString("    };\n")
	case "say_hi":
		sb.WriteString("    // returns a result with: hi='hi' and status='ok'\n")
		sb.WriteString("    return {\n")
		sb.WriteString("      hi: 'hi',\n")
		sb.WriteString("    };\n")
	case "repeat_num":
		sb.WriteString("    // returns num as an array repeated 'count' number of times\n")
		sb.WriteString("    const result: number[] = [];\n")
		sb.WriteString("    for (let i = 0; i < count; i++) {\n")
		sb.WriteString("      result.push(num);\n")
		sb.WriteString("    }\n")
		sb.WriteString("    return result;\n")
	case "putperson":
		sb.WriteString("    // simply returns p.personId\n")
		sb.WriteString("    // we use this to test the '[optional]' enforcement, as we invoke it with a null email\n")
		sb.WriteString("    return p.personId;\n")
	default:
		// Default implementation: return appropriate type based on return type
		writeDefaultTestReturnTs(sb, method.ReturnType, structMap, enumMap)
	}
	sb.WriteString("  }\n\n")
}

// writeDefaultTestReturnTs generates a default return value for a type
func writeDefaultTestReturnTs(sb *strings.Builder, returnType *parser.Type, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	if returnType.IsBuiltIn() {
		switch returnType.BuiltIn {
		case "string":
			sb.WriteString("    return '';\n")
		case "int":
			sb.WriteString("    return 0;\n")
		case "float":
			sb.WriteString("    return 0.0;\n")
		case "bool":
			sb.WriteString("    return false;\n")
		default:
			sb.WriteString("    return null;\n")
		}
	} else if returnType.IsArray() {
		sb.WriteString("    return [];\n")
	} else if returnType.IsMap() {
		sb.WriteString("    return {};\n")
	} else if returnType.IsUserDefined() {
		// Check if it's a struct
		if structMap[returnType.UserDefined] != nil {
			s := structMap[returnType.UserDefined]
			sb.WriteString("    return {\n")
			// Handle inheritance - get all fields including parent
			for _, field := range s.Fields {
				if field.Optional {
					continue // Skip optional fields in default return
				}
				fmt.Fprintf(sb, "      %s: ", field.Name)
				writeDefaultTestValueTs(sb, field.Type, structMap, enumMap)
				sb.WriteString(",\n")
			}
			// If extends, add parent fields
			if s.Extends != "" {
				baseName := s.Extends
				// First try looking up with full name (including namespace)
				baseStruct := structMap[baseName]
				// If not found and has a namespace prefix, try with just the base name
				if baseStruct == nil && strings.Contains(baseName, ".") {
					parts := strings.Split(baseName, ".")
					baseName = parts[len(parts)-1]
					baseStruct = structMap[baseName]
				}
				// If we found the parent struct, add its fields
				if baseStruct != nil {
					for _, field := range baseStruct.Fields {
						if field.Optional {
							continue
						}
						fmt.Fprintf(sb, "      %s: ", field.Name)
						writeDefaultTestValueTs(sb, field.Type, structMap, enumMap)
						sb.WriteString(",\n")
					}
				}
			}
			sb.WriteString("    };\n")
		} else if enumMap[returnType.UserDefined] != nil {
			// Return first enum value
			e := enumMap[returnType.UserDefined]
			if len(e.Values) > 0 {
				fmt.Fprintf(sb, "    return '%s';\n", e.Values[0].Name)
			} else {
				sb.WriteString("    return null;\n")
			}
		} else {
			sb.WriteString("    return null;\n")
		}
	} else {
		sb.WriteString("    return null;\n")
	}
}

// writeDefaultTestValueTs generates a default value for a type (used in structs)
func writeDefaultTestValueTs(sb *strings.Builder, t *parser.Type, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	if t.IsBuiltIn() {
		switch t.BuiltIn {
		case "string":
			sb.WriteString("''")
		case "int":
			sb.WriteString("0")
		case "float":
			sb.WriteString("0.0")
		case "bool":
			sb.WriteString("false")
		default:
			sb.WriteString("null")
		}
	} else if t.IsArray() {
		sb.WriteString("[]")
	} else if t.IsMap() {
		sb.WriteString("{}")
	} else if t.IsUserDefined() {
		if structMap[t.UserDefined] != nil {
			sb.WriteString("{}")
		} else {
			// Try to find enum
			e := enumMap[t.UserDefined]
			// If not found with exact name, try to find by base name
			// (e.g., 'Status' might be registered as 'inc.Status')
			if e == nil {
				for enumKey, enumVal := range enumMap {
					if strings.HasSuffix(enumKey, "."+t.UserDefined) || enumKey == t.UserDefined {
						e = enumVal
						break
					}
				}
			}
			if e != nil {
				if len(e.Values) > 0 {
					fmt.Fprintf(sb, "'%s'", e.Values[0].Name)
				} else {
					sb.WriteString("null")
				}
			} else {
				sb.WriteString("null")
			}
		}
	} else {
		sb.WriteString("null")
	}
}

// generateTestClientTs generates test_client.ts that exercises all client methods
func generateTestClientTs(idl *parser.IDL, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, _ map[string]*parser.Interface, _ string, _ map[string]*NamespaceTypes) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n")
	sb.WriteString("// Test client for integration testing\n\n")
	sb.WriteString("import { HttpTransport, Client } from './pulserpc';\n\n")

	// Generate wait for server function
	sb.WriteString("async function waitForServer(url: string, timeout: number = 10000): Promise<boolean> {\n")
	sb.WriteString("  const startTime = Date.now();\n")
	sb.WriteString("  let retryDelay = 200;\n\n")
	sb.WriteString("  while (Date.now() - startTime < timeout) {\n")
	sb.WriteString("    try {\n")
	sb.WriteString("      const controller = new AbortController();\n")
	sb.WriteString("      const timeoutId = setTimeout(() => controller.abort(), 2000);\n")
	sb.WriteString("      const response = await fetch(url, {\n")
	sb.WriteString("        method: 'POST',\n")
	sb.WriteString("        headers: { 'Content-Type': 'application/json' },\n")
	sb.WriteString("        body: '{\"jsonrpc\":\"2.0\",\"method\":\"pulserpc-idl\",\"id\":1}',\n")
	sb.WriteString("        signal: controller.signal,\n")
	sb.WriteString("      });\n")
	sb.WriteString("      clearTimeout(timeoutId);\n")
	sb.WriteString("      if (response.ok) {\n")
	sb.WriteString("        return true;\n")
	sb.WriteString("      }\n")
	sb.WriteString("    } catch (err: any) {\n")
	sb.WriteString("      // Connection error - server not ready yet\n")
	sb.WriteString("    }\n")
	sb.WriteString("    await new Promise(resolve => setTimeout(resolve, retryDelay));\n")
	sb.WriteString("    retryDelay = Math.min(retryDelay * 1.5, 1000);\n")
	sb.WriteString("  }\n")
	sb.WriteString("  return false;\n")
	sb.WriteString("}\n\n")

	// Generate main test function
	sb.WriteString("async function main() {\n")
	sb.WriteString("  const serverUrl = 'http://localhost:8080';\n\n")
	sb.WriteString("  // Wait for server to be ready\n")
	sb.WriteString("  if (!(await waitForServer(serverUrl, 10000))) {\n")
	sb.WriteString("    console.error('ERROR: Server did not become ready in time');\n")
	sb.WriteString("    process.exit(1);\n")
	sb.WriteString("  }\n\n")
	sb.WriteString("  console.log('Server is ready. Running tests...');\n")
	sb.WriteString("  console.log();\n\n")

	sb.WriteString("  // Create client - interfaces are auto-discovered\n")
	sb.WriteString("  const transport = new HttpTransport(serverUrl);\n")
	sb.WriteString("  const client = new Client(transport);\n")
	sb.WriteString("  await client.ready();\n\n")
	sb.WriteString("  const errors: string[] = [];\n\n")

	// Generate test cases for each method using dynamic proxy pattern
	for _, iface := range idl.Interfaces {
		for _, method := range iface.Methods {
			writeTestClientCallTs(&sb, iface, method, "client", structMap, enumMap)
		}
	}

	sb.WriteString("  // Report results\n")
	sb.WriteString("  console.log();\n")
	sb.WriteString("  if (errors.length > 0) {\n")
	sb.WriteString("    console.error(`FAILED: ${errors.length} test(s) failed:`);\n")
	sb.WriteString("    for (const error of errors) {\n")
	sb.WriteString("      console.error(`  - ${error}`);\n")
	sb.WriteString("    }\n")
	sb.WriteString("    process.exit(1);\n")
	sb.WriteString("  } else {\n")
	sb.WriteString("    console.log('SUCCESS: All tests passed!');\n")
	sb.WriteString("    process.exit(0);\n")
	sb.WriteString("  }\n")
	sb.WriteString("}\n\n")

	sb.WriteString("main().catch((err) => {\n")
	sb.WriteString("  console.error('Fatal error:', err);\n")
	sb.WriteString("  process.exit(1);\n")
	sb.WriteString("});\n")

	return sb.String()
}

// writeTestClientCallTs generates a test call for a method
func writeTestClientCallTs(sb *strings.Builder, iface *parser.Interface, method *parser.Method, clientVar string, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	testName := fmt.Sprintf("%s.%s", iface.Name, method.Name)
	fmt.Fprintf(sb, "  // Test %s\n", testName)
	sb.WriteString("  try {\n")

	// Generate test parameters based on method signature
	params := make([]string, 0)
	for _, param := range method.Parameters {
		paramValue := generateTestParamValueTs(param.Type, param.Name, structMap, enumMap)
		params = append(params, paramValue)
	}

	// Generate method call using dynamic proxy pattern: client.InterfaceName.methodName()
	if len(params) > 0 {
		fmt.Fprintf(sb, "    const result = await %s.%s.%s(%s);\n", clientVar, iface.Name, method.Name, strings.Join(params, ", "))
	} else {
		fmt.Fprintf(sb, "    const result = await %s.%s.%s();\n", clientVar, iface.Name, method.Name)
	}

	// Generate assertions based on method
	methodNameLower := strings.ToLower(method.Name)
	if iface.Name == "B" && method.Name == "echo" {
		sb.WriteString("    // Test normal return\n")
		sb.WriteString("    if (result !== 'test') {\n")
		sb.WriteString("      throw new Error(`Expected 'test', got ${result}`);\n")
		sb.WriteString("    }\n")
		sb.WriteString("    // Test null return\n")
		fmt.Fprintf(sb, "    const resultNull = await %s.%s.echo('return-null');\n", clientVar, iface.Name)
		sb.WriteString("    if (resultNull !== null) {\n")
		sb.WriteString("      throw new Error(`Expected null, got ${resultNull}`);\n")
		sb.WriteString("    }\n")
	} else if methodNameLower == "add" {
		sb.WriteString("    if (result !== 5) {\n")
		sb.WriteString("      throw new Error(`Expected 5, got ${result}`);\n")
		sb.WriteString("    }\n")
	} else if methodNameLower == "sqrt" {
		sb.WriteString("    if (globalThis.Math.abs(result - 2.0) >= 0.001) {\n")
		sb.WriteString("      throw new Error(`Expected ~2.0, got ${result}`);\n")
		sb.WriteString("    }\n")
	} else if methodNameLower == "calc" {
		sb.WriteString("    if (typeof result !== 'number') {\n")
		sb.WriteString("      throw new Error(`Expected number, got ${typeof result}`);\n")
		sb.WriteString("    }\n")
	} else if methodNameLower == "repeat" {
		sb.WriteString("    if (typeof result !== 'object' || !result) {\n")
		sb.WriteString("      throw new Error(`Expected object, got ${typeof result}`);\n")
		sb.WriteString("    }\n")
		sb.WriteString("    if (!('items' in result)) {\n")
		sb.WriteString("      throw new Error(\"Result missing 'items' field\");\n")
		sb.WriteString("    }\n")
		sb.WriteString("    if (result.items.length !== 3) {\n")
		sb.WriteString("      throw new Error(`Expected 3 items, got ${result.items.length}`);\n")
		sb.WriteString("    }\n")
	} else if methodNameLower == "say_hi" {
		sb.WriteString("    if (typeof result !== 'object' || !result) {\n")
		sb.WriteString("      throw new Error(`Expected object, got ${typeof result}`);\n")
		sb.WriteString("    }\n")
		sb.WriteString("    if (result.hi !== 'hi') {\n")
		sb.WriteString("      throw new Error(`Expected hi='hi', got ${JSON.stringify(result)}`);\n")
		sb.WriteString("    }\n")
	} else if methodNameLower == "repeat_num" {
		sb.WriteString("    if (!Array.isArray(result)) {\n")
		sb.WriteString("      throw new Error(`Expected array, got ${typeof result}`);\n")
		sb.WriteString("    }\n")
		sb.WriteString("    if (result.length !== 2) {\n")
		sb.WriteString("      throw new Error(`Expected 2 items, got ${result.length}`);\n")
		sb.WriteString("    }\n")
	} else if methodNameLower == "putperson" {
		sb.WriteString("    if (typeof result !== 'string') {\n")
		sb.WriteString("      throw new Error(`Expected string, got ${typeof result}`);\n")
		sb.WriteString("    }\n")
		sb.WriteString("    if (result !== 'person123') {\n")
		sb.WriteString("      throw new Error(`Expected 'person123', got ${result}`);\n")
		sb.WriteString("    }\n")
	} else {
		// Generic assertion - just check that we got a result
		sb.WriteString("    if (result === null || result === undefined) {\n")
		sb.WriteString("      throw new Error('Expected non-null result');\n")
		sb.WriteString("    }\n")
	}

	fmt.Fprintf(sb, "    console.log('✓ %s passed');\n", testName)
	sb.WriteString("  } catch (err: any) {\n")
	fmt.Fprintf(sb, "    const errorMsg = `%s failed: ${err.message || err}`;\n", testName)
	sb.WriteString("    errors.push(errorMsg);\n")
	fmt.Fprintf(sb, "    console.error(`✗ ${errorMsg}`);\n")
	sb.WriteString("  }\n")
	sb.WriteString("\n")
}

// generateTestParamValueTs generates a test parameter value for a type
func generateTestParamValueTs(t *parser.Type, paramName string, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) string {
	if t.IsBuiltIn() {
		switch t.BuiltIn {
		case "string":
			if paramName == "s" {
				return "'test'"
			}
			return "'test'"
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
			return "null"
		}
	} else if t.IsArray() {
		if t.Array.IsBuiltIn() && t.Array.BuiltIn == "float" {
			return "[1.0, 2.0, 3.0]"
		}
		return "[]"
	} else if t.IsMap() {
		return "{}"
	} else if t.IsUserDefined() {
		// Check if it's a struct
		if structMap[t.UserDefined] != nil {
			s := structMap[t.UserDefined]
			// Build struct object
			fields := []string{}
			for _, field := range s.Fields {
				if field.Optional && field.Name == "email" {
					// Special case: set email to null for putPerson test
					fields = append(fields, fmt.Sprintf("%s: null", field.Name))
				} else if !field.Optional {
					fieldValue := generateTestParamValueTs(field.Type, field.Name, structMap, enumMap)
					fields = append(fields, fmt.Sprintf("%s: %s", field.Name, fieldValue))
				}
			}
			// Handle inheritance
			if s.Extends != "" {
				baseName := s.Extends
				// First try looking up with full name (including namespace)
				baseStruct := structMap[baseName]
				// If not found and has a namespace prefix, try with just the base name
				if baseStruct == nil && strings.Contains(baseName, ".") {
					parts := strings.Split(baseName, ".")
					baseName = parts[len(parts)-1]
					baseStruct = structMap[baseName]
				}
				// If we found the parent struct, add its fields
				if baseStruct != nil {
					for _, field := range baseStruct.Fields {
						if !field.Optional {
							fieldValue := generateTestParamValueTs(field.Type, field.Name, structMap, enumMap)
							fields = append(fields, fmt.Sprintf("%s: %s", field.Name, fieldValue))
						}
					}
				}
			}
			// Special handling for RepeatRequest
			if t.UserDefined == "RepeatRequest" {
				return "{ to_repeat: 'hello', count: 3, force_uppercase: false }"
			}
			// Special handling for Person
			if t.UserDefined == "Person" {
				return "{ personId: 'person123', firstName: 'John', lastName: 'Doe', email: null }"
			}
			return "{ " + strings.Join(fields, ", ") + " }"
		} else if enumMap[t.UserDefined] != nil {
			e := enumMap[t.UserDefined]
			if len(e.Values) > 0 {
				// Special case for MathOp
				if t.UserDefined == "inc.MathOp" || strings.HasSuffix(t.UserDefined, "MathOp") {
					return "'add'"
				}
				return fmt.Sprintf("'%s'", e.Values[0].Name)
			}
			return "null"
		}
		return "null"
	}
	return "null"
}

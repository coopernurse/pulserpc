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

// JavaClientServer is a plugin that generates Java HTTP server and client code from IDL
type JavaClientServer struct {
}

// NewJavaClientServer creates a new JavaClientServer plugin instance
func NewJavaClientServer() *JavaClientServer {
	return &JavaClientServer{}
}

// Name returns the plugin identifier
func (p *JavaClientServer) Name() string {
	return "java-client-server"
}

// RegisterFlags registers CLI flags for this plugin
func (p *JavaClientServer) RegisterFlags(fs *flag.FlagSet) {
	// Register -package flag for base Java package (e.g., com.example.server)
	// Only register if not already registered by another plugin (e.g., Go, Python, TS)
	if fs.Lookup("package") == nil {
		fs.String("package", "", "Base package name for generated Java classes (required, e.g., com.example.server)")
	}
	// Register json-lib flag for choosing between Jackson and GSON
	fs.String("json-lib", "jackson", "JSON library to use: 'jackson' or 'gson'")
}

// Generate generates Java HTTP server and client code from the parsed IDL
func (p *JavaClientServer) Generate(idl *parser.IDL, fs *flag.FlagSet) error {
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

	// Get package flag (required)
	packageFlag := fs.Lookup("package")
	basePackage := ""
	if packageFlag != nil {
		basePackage = packageFlag.Value.String()
	}
	if basePackage == "" {
		return fmt.Errorf("package flag is required for Java code generation")
	}

	// Get json-lib flag
	jsonLibFlag := fs.Lookup("json-lib")
	jsonLib := "jackson" // default
	if jsonLibFlag != nil && jsonLibFlag.Value.String() != "" {
		jsonLib = jsonLibFlag.Value.String()
	}
	if jsonLib != "jackson" && jsonLib != "gson" {
		return fmt.Errorf("invalid json-lib value: %s (must be 'jackson' or 'gson')", jsonLib)
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

	// Generate pom.xml for Maven builds
	pomCode := generatePomXml(jsonLib)
	pomPath := filepath.Join(outputDir, "pom.xml")
	if err := os.WriteFile(pomPath, []byte(pomCode), 0644); err != nil {
		return fmt.Errorf("failed to write pom.xml: %w", err)
	}
	PrintFileCreated(pomPath, fs)

	// Copy runtime library files with selective copying based on json-lib
	// Runtime goes to {dir}/pulserpc/ regardless of -package flag
	if err := p.copyRuntimeFiles(outputDir, jsonLib, isSilent()); err != nil {
		return fmt.Errorf("failed to copy runtime files: %w", err)
	}

	// Group types by namespace
	namespaceMap := GroupTypesByNamespace(idl)

	// Generate separate files for each type with proper package structure
	for namespace, types := range namespaceMap {
		if namespace == "" {
			continue // Skip types without namespace (shouldn't happen with required namespaces)
		}

		// Use namespace as-is (preserve case for Java package names)
		fullPackage := basePackage
		if namespace != "" {
			fullPackage = basePackage + "." + namespace
		}
		packageDir := filepath.Join(outputDir, "src/main/java", strings.ReplaceAll(fullPackage, ".", string(filepath.Separator)))

		// Generate enum files
		for _, enum := range types.Enums {
			enumCode := generateEnumFile(enum, fullPackage)
			enumName := GetBaseName(enum.Name)
			enumPath := filepath.Join(packageDir, enumName+".java")
			if err := os.MkdirAll(filepath.Dir(enumPath), 0755); err != nil {
				return fmt.Errorf("failed to create package directory: %w", err)
			}
			if err := os.WriteFile(enumPath, []byte(enumCode), 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", enumPath, err)
			}
			PrintFileCreated(enumPath, fs)
		}

		// Generate struct files (need to handle inheritance)
		for _, structDef := range types.Structs {
			structCode := generateStructFile(structDef, fullPackage, structMap, enumMap, jsonLib, basePackage)
			structName := GetBaseName(structDef.Name)
			structPath := filepath.Join(packageDir, structName+".java")
			if err := os.MkdirAll(filepath.Dir(structPath), 0755); err != nil {
				return fmt.Errorf("failed to create package directory: %w", err)
			}
			if err := os.WriteFile(structPath, []byte(structCode), 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", structPath, err)
			}
			PrintFileCreated(structPath, fs)
		}

		// Generate interface files
		for _, iface := range types.Interfaces {
			interfaceCode := generateInterfaceFile(iface, fullPackage, structMap, enumMap, basePackage)
			interfaceName := GetBaseName(iface.Name)
			interfacePath := filepath.Join(packageDir, interfaceName+".java")
			if err := os.MkdirAll(filepath.Dir(interfacePath), 0755); err != nil {
				return fmt.Errorf("failed to create package directory: %w", err)
			}
			if err := os.WriteFile(interfacePath, []byte(interfaceCode), 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", interfacePath, err)
			}
			PrintFileCreated(interfacePath, fs)
		}

		// Generate client files for each interface
		for _, iface := range types.Interfaces {
			clientCode := generateInterfaceClient(iface, fullPackage, enumMap, jsonLib, basePackage, structMap)
			interfaceName := GetBaseName(iface.Name)
			clientPath := filepath.Join(packageDir, interfaceName+"Client.java")
			if err := os.MkdirAll(filepath.Dir(clientPath), 0755); err != nil {
				return fmt.Errorf("failed to create package directory: %w", err)
			}
			if err := os.WriteFile(clientPath, []byte(clientCode), 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", clientPath, err)
			}
			PrintFileCreated(clientPath, fs)
		}

		// Generate namespace aggregate (IDL maps + types) into a single file
		nsIdlCode := generateNamespaceJava(namespace, types, enumMap, jsonLib, fullPackage)
		nsIdlPath := filepath.Join(packageDir, namespace+"Idl.java")
		if err := os.MkdirAll(filepath.Dir(nsIdlPath), 0755); err != nil {
			return fmt.Errorf("failed to create package directory: %w", err)
		}
		if err := os.WriteFile(nsIdlPath, []byte(nsIdlCode), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", nsIdlPath, err)
		}
		PrintFileCreated(nsIdlPath, fs)
	}

	// Marshal IDL to JSON for embedding in server code
	idlJSON, err := json.MarshalIndent(idl, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal IDL to JSON: %w", err)
	}

	// Generate Server.java with embedded IDL
	serverCodePkg := generateServerJava(idl, structMap, namespaceMap, basePackage, basePackage, string(idlJSON))
	// Server and Client belong in the base package
	basePackageDir := filepath.Join(outputDir, "src/main/java", strings.ReplaceAll(basePackage, ".", string(filepath.Separator)))
	if err := os.MkdirAll(basePackageDir, 0755); err != nil {
		return fmt.Errorf("failed to create base package directory: %w", err)
	}
	serverPath := filepath.Join(basePackageDir, "Server.java")
	if err := os.WriteFile(serverPath, []byte(serverCodePkg), 0644); err != nil {
		return fmt.Errorf("failed to write Server.java: %w", err)
	}
	PrintFileCreated(serverPath, fs)

	// Generate Client.java
	clientCodePkg := generateClientJava(idl, namespaceMap, basePackage, basePackage)
	clientPath := filepath.Join(basePackageDir, "Client.java")
	if err := os.WriteFile(clientPath, []byte(clientCodePkg), 0644); err != nil {
		return fmt.Errorf("failed to write Client.java: %w", err)
	}
	PrintFileCreated(clientPath, fs)

	// Check if generate-test-files flag is set
	generateTestFilesFlag := fs.Lookup("generate-test-files")
	generateTestServer := generateTestFilesFlag != nil && generateTestFilesFlag.Value.String() == "true"

	// Generate test server and client if flag is set
	if generateTestServer {
		// Generate separate implementation files for each interface
		for _, iface := range idl.Interfaces {
			ifaceNamespace := GetNamespaceFromType(iface.Name, iface.Namespace)
			ifacePackage := basePackage
			if ifaceNamespace != "" {
				ifacePackage = basePackage + "." + ifaceNamespace
			}
			implCode := generateTestInterfaceImplFile(iface, ifacePackage, structMap, enumMap, jsonLib, basePackage)
			implName := GetBaseName(iface.Name) + "Impl"
			implPath := filepath.Join(dirFlag.Value.String(), "src/main/java", strings.ReplaceAll(ifacePackage, ".", string(filepath.Separator)), implName+".java")
			if err := os.MkdirAll(filepath.Dir(implPath), 0755); err != nil {
				return fmt.Errorf("failed to create package directory: %w", err)
			}
			if err := os.WriteFile(implPath, []byte(implCode), 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", implPath, err)
			}
			PrintFileCreated(implPath, fs)
		}

		// Generate TestServer.java in base package
		testServerCode := generateTestServerJava(idl, jsonLib, basePackage, namespaceMap)
		testServerDir := filepath.Join(dirFlag.Value.String(), "src/test/java", strings.ReplaceAll(basePackage, ".", string(filepath.Separator)))
		if err := os.MkdirAll(testServerDir, 0755); err != nil {
			return fmt.Errorf("failed to create test java directory: %w", err)
		}
		testServerPath := filepath.Join(testServerDir, "TestServer.java")
		if err := os.WriteFile(testServerPath, []byte(testServerCode), 0644); err != nil {
			return fmt.Errorf("failed to write TestServer.java: %w", err)
		}
		PrintFileCreated(testServerPath, fs)

		// Generate TestClient.java in base package
		testClientCode := generateTestClientJava(idl, structMap, enumMap, jsonLib, basePackage, namespaceMap)
		testClientPath := filepath.Join(testServerDir, "TestClient.java")
		if err := os.WriteFile(testClientPath, []byte(testClientCode), 0644); err != nil {
			return fmt.Errorf("failed to write TestClient.java: %w", err)
		}
		PrintFileCreated(testClientPath, fs)
	}

	return nil
}

// copyRuntimeFiles copies the Java runtime library files to the output directory
// Selectively copies files based on json-lib flag
func (p *JavaClientServer) copyRuntimeFiles(outputDir string, jsonLib string, silent bool) error {
	// Runtime always goes to {dir}/pulserpc/ regardless of package flag
	runtimeBaseDir := filepath.Join(outputDir, getRuntimePackageDirName())

	// Get runtime files
	files, err := runtime.GetRuntimeFiles("java")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(runtimeBaseDir, 0755); err != nil {
		return fmt.Errorf("failed to create runtime directory: %w", err)
	}

	// Copy all files
	for filename, data := range files {
		dstPath := filepath.Join(runtimeBaseDir, filename)
		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write runtime file %s: %w", dstPath, err)
		}
		if !silent {
			fmt.Println(dstPath)
		}
	}

	// Remove the JSON parser implementation we don't want (keep only selected jsonLib)
	switch jsonLib {
	case "jackson":
		// remove Gson implementation if present
		_ = os.Remove(filepath.Join(runtimeBaseDir, "GsonJsonParser.java"))
	case "gson":
		// remove Jackson implementation if present
		_ = os.Remove(filepath.Join(runtimeBaseDir, "JacksonJsonParser.java"))
	}

	return nil
}

// getRuntimePackageDirName returns the directory name used for runtime files.
// Mirrors runtime.getRuntimePackageName but kept local to avoid import cycles.
func getRuntimePackageDirName() string {
	return "src/main/java/com/bitmechanic/pulserpc"
}

// generateEnumFile generates a Java enum file
func generateEnumFile(enum *parser.Enum, packageName string) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString(fmt.Sprintf("package %s;\n\n", packageName))

	enumName := GetBaseName(enum.Name)
	sb.WriteString(fmt.Sprintf("public enum %s {\n", enumName))
	for i, value := range enum.Values {
		fmt.Fprintf(&sb, "    %s", value.Name)
		if i < len(enum.Values)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("}\n")

	return sb.String()
}

// generateStructFile generates a Java struct file
func generateStructFile(structDef *parser.Struct, packageName string, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, jsonLib string, basePackage string) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString(fmt.Sprintf("package %s;\n\n", packageName))

	// Add imports based on json-lib
	switch jsonLib {
	case "jackson":
		sb.WriteString("import com.fasterxml.jackson.annotation.JsonProperty;\n")
	case "gson":
		sb.WriteString("import com.google.gson.annotations.SerializedName;\n")
	}

	// Add imports for types from other packages
	imports := make(map[string]bool)
	_ = structMap
	className := GetBaseName(structDef.Name)

	// Check if struct extends another struct
	if structDef.Extends != "" {
		parentName := GetBaseName(structDef.Extends)
		parentNamespace := GetNamespaceFromType(structDef.Extends, "")
		if parentNamespace != "" {
			parentPackage := basePackage + "." + parentNamespace
			if parentPackage != packageName {
				imports[parentPackage+"."+parentName] = true
			}
		}
	}

	// Check field types for imports
	for _, field := range structDef.Fields {
		addTypeImports(field.Type, basePackage, packageName, imports, structMap, enumMap)
	}

	// Write imports
	for imp := range imports {
		sb.WriteString(fmt.Sprintf("import %s;\n", imp))
	}
	if len(imports) > 0 {
		sb.WriteString("\n")
	}

	// Generate class declaration
	if structDef.Extends != "" {
		parentName := GetBaseName(structDef.Extends)
		parentNamespace := GetNamespaceFromType(structDef.Extends, "")
		if parentNamespace != "" {
			parentPackage := basePackage + "." + parentNamespace
			if parentPackage != packageName {
				// Use fully qualified name
				fmt.Fprintf(&sb, "public class %s extends %s.%s {\n", className, parentPackage, parentName)
			} else {
				fmt.Fprintf(&sb, "public class %s extends %s {\n", className, parentName)
			}
		} else {
			fmt.Fprintf(&sb, "public class %s extends %s {\n", className, parentName)
		}
	} else {
		fmt.Fprintf(&sb, "public class %s {\n", className)
	}

	// Generate fields
	for _, field := range structDef.Fields {
		fieldType := getJavaTypeWithPackage(field.Type, enumMap, basePackage, packageName, structMap)
		fieldName := toCamelCase(field.Name)

		// Add JSON annotation based on library
		switch jsonLib {
		case "jackson":
			fmt.Fprintf(&sb, "    @JsonProperty(\"%s\")\n", field.Name)
		case "gson":
			fmt.Fprintf(&sb, "    @SerializedName(\"%s\")\n", field.Name)
		}

		fmt.Fprintf(&sb, "    private %s %s;\n\n", fieldType, fieldName)
	}

	// Generate getters and setters
	for _, field := range structDef.Fields {
		fieldType := getJavaTypeWithPackage(field.Type, enumMap, basePackage, packageName, structMap)
		fieldName := toCamelCase(field.Name)
		capitalizedName := capitalizeFirst(fieldName)

		// Getter
		fmt.Fprintf(&sb, "    public %s get%s() {\n", fieldType, capitalizedName)
		fmt.Fprintf(&sb, "        return %s;\n", fieldName)
		sb.WriteString("    }\n\n")

		// Setter
		fmt.Fprintf(&sb, "    public void set%s(%s %s) {\n", capitalizedName, fieldType, fieldName)
		fmt.Fprintf(&sb, "        this.%s = %s;\n", fieldName, fieldName)
		sb.WriteString("    }\n\n")
	}

	sb.WriteString("}\n")

	return sb.String()
}

// generateInterfaceFile generates a Java interface file
func generateInterfaceFile(iface *parser.Interface, packageName string, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, basePackage string) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString(fmt.Sprintf("package %s;\n\n", packageName))

	// Add imports for types from other packages
	imports := make(map[string]bool)
	_ = structMap
	interfaceName := GetBaseName(iface.Name)

	// Check method parameter and return types for imports
	for _, method := range iface.Methods {
		if method.ReturnType != nil {
			addTypeImports(method.ReturnType, basePackage, packageName, imports, structMap, enumMap)
		}
		for _, param := range method.Parameters {
			addTypeImports(param.Type, basePackage, packageName, imports, structMap, enumMap)
		}
	}

	// Write imports
	for imp := range imports {
		sb.WriteString(fmt.Sprintf("import %s;\n", imp))
	}
	if len(imports) > 0 {
		sb.WriteString("\n")
	}

	// Generate interface declaration
	fmt.Fprintf(&sb, "public interface %s {\n", interfaceName)

	// Generate methods
	for _, method := range iface.Methods {
		returnType := "void"
		if method.ReturnType != nil {
			returnType = getJavaTypeWithPackage(method.ReturnType, enumMap, basePackage, packageName, structMap)
		}

		fmt.Fprintf(&sb, "    public %s %s(", returnType, method.Name)

		// Parameters
		for i, param := range method.Parameters {
			if i > 0 {
				sb.WriteString(", ")
			}
			paramType := getJavaTypeWithPackage(param.Type, enumMap, basePackage, packageName, structMap)
			fmt.Fprintf(&sb, "%s %s", paramType, param.Name)
		}
		sb.WriteString(");\n\n")
	}

	sb.WriteString("}\n")

	return sb.String()
}

// generateInterfaceClient generates a client class for an interface
func generateInterfaceClient(iface *parser.Interface, packageName string, enumMap map[string]*parser.Enum, jsonLib string, basePackage string, structMap map[string]*parser.Struct) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString(fmt.Sprintf("package %s;\n\n", packageName))

	// Add imports
	sb.WriteString("import com.bitmechanic.pulserpc.*;\n")
	if jsonLib == "jackson" {
		sb.WriteString("import com.fasterxml.jackson.core.type.TypeReference;\n")
	}

	interfaceName := GetBaseName(iface.Name)
	clientName := interfaceName + "Client"

	// Generate class declaration - interface is in same package, so no qualification needed
	sb.WriteString("public class ")
	sb.WriteString(clientName)
	sb.WriteString(" implements ")
	sb.WriteString(interfaceName)
	sb.WriteString(" {\n")
	sb.WriteString("    private final Transport transport;\n")
	sb.WriteString("    private final JsonParser jsonParser;\n\n")

	// Constructor
	sb.WriteString("    public ")
	sb.WriteString(clientName)
	sb.WriteString("(Transport transport, JsonParser jsonParser) {\n")
	sb.WriteString("        this.transport = transport;\n")
	sb.WriteString("        this.jsonParser = jsonParser;\n")
	sb.WriteString("    }\n\n")

	// Generate methods
	for _, method := range iface.Methods {
		returnType := "void"
		if method.ReturnType != nil {
			returnType = getJavaTypeWithPackage(method.ReturnType, enumMap, basePackage, packageName, structMap)
		}

		fmt.Fprintf(&sb, "    @Override\n")
		fmt.Fprintf(&sb, "    public %s %s(", returnType, method.Name)

		// Parameters
		for i, param := range method.Parameters {
			if i > 0 {
				sb.WriteString(", ")
			}
			paramType := getJavaTypeWithPackage(param.Type, enumMap, basePackage, packageName, structMap)
			fmt.Fprintf(&sb, "%s %s", paramType, param.Name)
		}
		sb.WriteString(") {\n")

		// Method implementation
		sb.WriteString("        try {\n")
		fmt.Fprintf(&sb, "            String method = \"%s.%s\";\n", interfaceName, method.Name)

		// Build parameters array
		sb.WriteString("            Object[] params = new Object[] { ")
		for i, param := range method.Parameters {
			if i > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(&sb, "%s", param.Name)
		}
		sb.WriteString(" };\n\n")

		// Create request and call transport
		sb.WriteString("            Request rpcRequest = new Request(method, params, java.util.UUID.randomUUID().toString());\n")
		sb.WriteString("            Response response = transport.call(rpcRequest);\n\n")

		// Handle return value
		if method.ReturnType != nil {
			sb.WriteString("            if (response.getResult() == null) {\n")
			if method.ReturnOptional {
				sb.WriteString("                return null;\n")
			} else {
				sb.WriteString("                throw new RPCError(-32603, \"Internal error\", \"Missing result in response\");\n")
			}
			sb.WriteString("            }\n\n")

			// Deserialize result
			sb.WriteString("            String resultJson = jsonParser.toJson(response.getResult());\n")
			if jsonLib == "jackson" {
				// Jackson: Use TypeReference wrapped in a Type
				sb.WriteString("            com.fasterxml.jackson.core.type.TypeReference<")
				writeJavaType(&sb, method.ReturnType, enumMap, basePackage, packageName, structMap)
				sb.WriteString("> typeRef = new com.fasterxml.jackson.core.type.TypeReference<")
				writeJavaType(&sb, method.ReturnType, enumMap, basePackage, packageName, structMap)
				sb.WriteString(">() {};\n")
				sb.WriteString("            return jsonParser.fromJson(resultJson, typeRef.getType());\n")
			} else {
				// Gson uses TypeToken
				sb.WriteString("            java.lang.reflect.Type type = new com.google.gson.reflect.TypeToken<")
				writeJavaType(&sb, method.ReturnType, enumMap, basePackage, packageName, structMap)
				sb.WriteString(">(){}.getType();\n")
				sb.WriteString("            return jsonParser.fromJson(resultJson, type);\n")
			}
		}

		sb.WriteString("        } catch (Exception e) {\n")
		sb.WriteString("            if (e instanceof RPCError) {\n")
		sb.WriteString("                throw (RPCError) e;\n")
		sb.WriteString("            }\n")
		sb.WriteString("            throw new RPCError(-32603, \"Internal error\", e.getMessage());\n")
		sb.WriteString("        }\n")
		sb.WriteString("    }\n\n")
	}

	sb.WriteString("}\n")

	return sb.String()
}

// Helper functions for type handling

// findTypeNamespace looks up a type by base name in structMap and enumMap
// to find which namespace it belongs to. This handles cross-namespace references
// where types are stored with simple names (e.g., "Category" instead of "common.Category").
func findTypeNamespace(typeName string, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) string {
	baseName := GetBaseName(typeName)

	// First check structMap by qualified name
	for fullName, s := range structMap {
		if GetBaseName(fullName) == baseName {
			return GetNamespaceFromType(fullName, s.Namespace)
		}
	}

	// Then check enumMap by qualified name
	for fullName, e := range enumMap {
		if GetBaseName(fullName) == baseName {
			return GetNamespaceFromType(fullName, e.Namespace)
		}
	}

	// Fall back to GetNamespaceFromType which may extract from qualified name
	return GetNamespaceFromType(typeName, "")
}

// addTypeImports adds necessary imports for a type
func addTypeImports(t *parser.Type, basePackage string, currentPackage string, imports map[string]bool, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	if t.IsUserDefined() {
		typeNamespace := findTypeNamespace(t.UserDefined, structMap, enumMap)
		typeName := GetBaseName(t.UserDefined)
		if typeNamespace != "" {
			typePackage := basePackage + "." + typeNamespace
			if typePackage != currentPackage {
				imports[typePackage+"."+typeName] = true
			}
		}
	} else if t.IsArray() {
		addTypeImports(t.Array, basePackage, currentPackage, imports, structMap, enumMap)
	} else if t.IsMap() {
		addTypeImports(t.MapValue, basePackage, currentPackage, imports, structMap, enumMap)
	}
}

// getJavaTypeWithPackage returns Java type name with package qualification if needed
// For primitives in generics, this uses boxed types (Integer, Double, Boolean)
func getJavaTypeWithPackage(t *parser.Type, enumMap map[string]*parser.Enum, basePackage string, currentPackage string, structMap map[string]*parser.Struct) string {
	if t.IsBuiltIn() {
		return getJavaType(t, enumMap)
	} else if t.IsArray() {
		elementType := getJavaTypeWithPackageForGeneric(t.Array, basePackage, currentPackage, structMap)
		return fmt.Sprintf("java.util.List<%s>", elementType)
	} else if t.IsMap() {
		valueType := getJavaTypeWithPackageForGeneric(t.MapValue, basePackage, currentPackage, structMap)
		return fmt.Sprintf("java.util.Map<String, %s>", valueType)
	} else if t.IsUserDefined() {
		typeName := GetBaseName(t.UserDefined)
		typeNamespace := findTypeNamespace(t.UserDefined, structMap, enumMap)
		if typeNamespace != "" {
			typePackage := basePackage + "." + typeNamespace
			if typePackage != currentPackage {
				return typePackage + "." + typeName
			}
		}
		return typeName
	}
	return "Object"
}

// getJavaTypeWithPackageForGeneric returns Java type name for use in generics (uses boxed types for primitives)
func getJavaTypeWithPackageForGeneric(t *parser.Type, basePackage string, currentPackage string, structMap map[string]*parser.Struct) string {
	if t.IsBuiltIn() {
		switch t.BuiltIn {
		case "string":
			return "String"
		case "int":
			return "Integer"
		case "float":
			return "Double"
		case "bool":
			return "Boolean"
		}
		return "Object"
	} else if t.IsArray() {
		elementType := getJavaTypeWithPackageForGeneric(t.Array, basePackage, currentPackage, structMap)
		return fmt.Sprintf("java.util.List<%s>", elementType)
	} else if t.IsMap() {
		valueType := getJavaTypeWithPackageForGeneric(t.MapValue, basePackage, currentPackage, structMap)
		return fmt.Sprintf("java.util.Map<String, %s>", valueType)
	} else if t.IsUserDefined() {
		typeName := GetBaseName(t.UserDefined)
		typeNamespace := findTypeNamespace(t.UserDefined, structMap, nil)
		if typeNamespace != "" {
			typePackage := basePackage + "." + typeNamespace
			if typePackage != currentPackage {
				return typePackage + "." + typeName
			}
		}
		return typeName
	}
	return "Object"
}

// writeJavaType writes Java type for use in generics (uses boxed types for primitives)
func writeJavaType(sb *strings.Builder, t *parser.Type, enumMap map[string]*parser.Enum, basePackage string, currentPackage string, structMap map[string]*parser.Struct) {
	if t.IsBuiltIn() {
		// Use boxed types for primitives in generics
		switch t.BuiltIn {
		case "string":
			sb.WriteString("String")
		case "int":
			sb.WriteString("Integer")
		case "float":
			sb.WriteString("Double")
		case "bool":
			sb.WriteString("Boolean")
		default:
			sb.WriteString("Object")
		}
	} else if t.IsArray() {
		sb.WriteString("java.util.List<")
		writeJavaType(sb, t.Array, enumMap, basePackage, currentPackage, structMap)
		sb.WriteString(">")
	} else if t.IsMap() {
		sb.WriteString("java.util.Map<String, ")
		writeJavaType(sb, t.MapValue, enumMap, basePackage, currentPackage, structMap)
		sb.WriteString(">")
	} else if t.IsUserDefined() {
		sb.WriteString(getJavaTypeWithPackage(t, enumMap, basePackage, currentPackage, structMap))
	} else {
		sb.WriteString("Object")
	}
}

// writeTypeReference writes TypeReference for Jackson
func writeTypeReference(sb *strings.Builder, t *parser.Type, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, basePackage string, currentPackage string) {
	sb.WriteString("new com.fasterxml.jackson.core.type.TypeReference<")
	writeJavaType(sb, t, enumMap, basePackage, currentPackage, structMap)
	sb.WriteString(">() {}")
}

// generateNamespaceJava generates a Java file for a single namespace
func generateNamespaceJava(namespace string, types *NamespaceTypes, enumMap map[string]*parser.Enum, jsonLib string, packageName string) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")

	// Package declaration
	sb.WriteString(fmt.Sprintf("package %s;\n\n", packageName))

	// Add imports based on json-lib
	switch jsonLib {
	case "jackson":
		sb.WriteString("import com.fasterxml.jackson.annotation.JsonProperty;\n")
	case "gson":
		sb.WriteString("import com.google.gson.annotations.SerializedName;\n")
	}
	sb.WriteString("import java.util.Map;\n")
	sb.WriteString("import java.util.List;\n\n")

	// Generate IDL-specific type definitions for this namespace
	sb.WriteString(fmt.Sprintf("// IDL-specific type definitions for namespace: %s\n", namespace))
	sb.WriteString("public final class " + namespace + "Idl {\n")
	sb.WriteString("    public static final java.util.Map<String, java.util.Map<String, Object>> ALL_STRUCTS;\n")
	sb.WriteString("    public static final java.util.Map<String, java.util.Map<String, Object>> ALL_ENUMS;\n\n")

	sb.WriteString("    static {\n")
	sb.WriteString("        java.util.Map<String, java.util.Map<String, Object>> structs = new java.util.HashMap<>();\n")
	sb.WriteString("        java.util.Map<String, java.util.Map<String, Object>> enums = new java.util.HashMap<>();\n\n")

	// Populate structs
	for _, s := range types.Structs {
		sb.WriteString("        {\n")
		sb.WriteString("            java.util.Map<String, Object> def = new java.util.HashMap<>();\n")
		if s.Extends != "" {
			sb.WriteString(fmt.Sprintf("            def.put(\"extends\", \"%s\");\n", s.Extends))
		}
		sb.WriteString("            java.util.List<java.util.Map<String, Object>> fields = new java.util.ArrayList<>();\n")
		for _, field := range s.Fields {
			sb.WriteString("            {\n")
			sb.WriteString("                java.util.Map<String, Object> f = new java.util.HashMap<>();\n")
			sb.WriteString(fmt.Sprintf("                f.put(\"name\", \"%s\");\n", field.Name))
			sb.WriteString("                java.util.Map<String, Object> typeDef = new java.util.HashMap<>();\n")
			// write type dict as simple map form
			writeTypeDictJava(&sb, field.Type)
			sb.WriteString("                f.put(\"type\", typeDef);\n")
			if field.Optional {
				sb.WriteString("                f.put(\"optional\", true);\n")
			}
			sb.WriteString("                fields.add(f);\n")
			sb.WriteString("            }\n")
		}
		sb.WriteString("            def.put(\"fields\", fields);\n")
		sb.WriteString(fmt.Sprintf("            structs.put(\"%s\", def);\n", s.Name))
		sb.WriteString("        }\n")
	}

	// Populate enums
	for _, e := range types.Enums {
		sb.WriteString("        {\n")
		sb.WriteString("            java.util.Map<String, Object> ed = new java.util.HashMap<>();\n")
		sb.WriteString("            java.util.List<java.util.Map<String, Object>> values = new java.util.ArrayList<>();\n")
		for _, v := range e.Values {
			sb.WriteString("            {\n")
			sb.WriteString("                java.util.Map<String, Object> ev = new java.util.HashMap<>();\n")
			sb.WriteString(fmt.Sprintf("                ev.put(\"name\", \"%s\");\n", v.Name))
			sb.WriteString("                values.add(ev);\n")
			sb.WriteString("            }\n")
		}
		sb.WriteString("            ed.put(\"values\", values);\n")
		sb.WriteString(fmt.Sprintf("            enums.put(\"%s\", ed);\n", e.Name))
		sb.WriteString("        }\n")
	}

	sb.WriteString("        ALL_STRUCTS = java.util.Collections.unmodifiableMap(structs);\n")
	sb.WriteString("        ALL_ENUMS = java.util.Collections.unmodifiableMap(enums);\n")
	sb.WriteString("    }\n")
	sb.WriteString("}\n")

	return sb.String()
}

// generateEnumTypesJava generates Java enum types
func generateEnumTypesJava(sb *strings.Builder, enums []*parser.Enum) {
	for _, enum := range enums {
		simpleName := getSimpleName(enum.Name)
		fmt.Fprintf(sb, "enum %s {\n", simpleName)
		for i, value := range enum.Values {
			fmt.Fprintf(sb, "    %s", value.Name)
			if i < len(enum.Values)-1 {
				sb.WriteString(",")
			}
			sb.WriteString("\n")
		}
		sb.WriteString("}\n\n")
	}
}

// generateStructClassesJava generates Java struct classes
func generateStructClassesJava(sb *strings.Builder, structs []*parser.Struct, enumMap map[string]*parser.Enum, jsonLib string) {
	for _, structDef := range structs {
		generateStructClassJava(sb, structDef, enumMap, jsonLib)
		sb.WriteString("\n")
	}
}

// generateStructClassJava generates a single Java struct class
func generateStructClassJava(sb *strings.Builder, structDef *parser.Struct, enumMap map[string]*parser.Enum, jsonLib string) {
	className := getSimpleName(structDef.Name)
	extendsName := ""
	if structDef.Extends != "" {
		extendsName = getSimpleName(structDef.Extends)
		fmt.Fprintf(sb, "class %s extends %s {\n", className, extendsName)
	} else {
		fmt.Fprintf(sb, "class %s {\n", className)
	}

	// Generate fields
	for _, field := range structDef.Fields {
		fieldType := getJavaType(field.Type, enumMap)
		fieldName := toCamelCase(field.Name)

		// Add JSON annotation based on library
		switch jsonLib {
		case "jackson":
			fmt.Fprintf(sb, "    @JsonProperty(\"%s\")\n", field.Name)
		case "gson":
			fmt.Fprintf(sb, "    @SerializedName(\"%s\")\n", field.Name)
		}

		fmt.Fprintf(sb, "    private %s %s;\n\n", fieldType, fieldName)
	}

	// Generate getters and setters
	for _, field := range structDef.Fields {
		fieldType := getJavaType(field.Type, enumMap)
		fieldName := toCamelCase(field.Name)
		capitalizedName := capitalizeFirst(fieldName)

		// Getter
		fmt.Fprintf(sb, "    public %s get%s() {\n", fieldType, capitalizedName)
		fmt.Fprintf(sb, "        return %s;\n", fieldName)
		sb.WriteString("    }\n\n")

		// Setter
		fmt.Fprintf(sb, "    public void set%s(%s %s) {\n", capitalizedName, fieldType, fieldName)
		fmt.Fprintf(sb, "        this.%s = %s;\n", fieldName, fieldName)
		sb.WriteString("    }\n\n")
	}

	sb.WriteString("}\n")
}

// generateServerJava generates the Server.java file
func generateServerJava(idl *parser.IDL, _ map[string]*parser.Struct, namespaceMap map[string]*NamespaceTypes, basePackage string, packageDecl string, idlJson string) string {
	_ = namespaceMap
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	if packageDecl != "" {
		sb.WriteString(fmt.Sprintf("package %s;\n\n", packageDecl))
	}
	sb.WriteString("import com.bitmechanic.pulserpc.*;\n")
	sb.WriteString("import com.sun.net.httpserver.HttpServer;\n")
	sb.WriteString("import com.sun.net.httpserver.HttpExchange;\n")
	sb.WriteString("import java.io.*;\n")
	sb.WriteString("import java.net.*;\n")
	sb.WriteString("import java.util.*;\n")
	sb.WriteString("import java.lang.reflect.*;\n\n")

	// Add imports for interfaces
	imports := make(map[string]bool)
	for _, iface := range idl.Interfaces {
		ifaceNamespace := GetNamespaceFromType(iface.Name, iface.Namespace)
		if ifaceNamespace != "" {
			ifacePackage := basePackage + "." + ifaceNamespace
			ifaceName := GetBaseName(iface.Name)
			imports[ifacePackage+"."+ifaceName] = true
		}
	}
	for imp := range imports {
		sb.WriteString(fmt.Sprintf("import %s;\n", imp))
	}
	if len(imports) > 0 {
		sb.WriteString("\n")
	}

	sb.WriteString("public class Server {\n")
	sb.WriteString("    // Embedded IDL JSON for pulserpc-idl RPC method\n")
	sb.WriteString("    private static final String IDL_JSON = ")
	sb.WriteString(escapeJavaString(idlJson))
	sb.WriteString(";\n\n")
	sb.WriteString("    private final HttpServer server;\n")
	sb.WriteString("    private final JsonParser jsonParser;\n")
	sb.WriteString("    private final Map<String, Object> interfaceHandlers;\n\n")

	// Constructor
	sb.WriteString("    public Server(int port, JsonParser jsonParser) throws IOException {\n")
	sb.WriteString("        this.jsonParser = jsonParser;\n")
	sb.WriteString("        this.server = HttpServer.create(new InetSocketAddress(port), 0);\n")
	sb.WriteString("        this.server.createContext(\"/\", this::handleRequest);\n")
	sb.WriteString("        this.interfaceHandlers = new HashMap<>();\n")
	sb.WriteString("    }\n\n")

	// Register interface implementation
	sb.WriteString("    public void register(String interfaceName, Object implementation) {\n")
	sb.WriteString("        interfaceHandlers.put(interfaceName, implementation);\n")
	sb.WriteString("    }\n\n")

	// Start method
	sb.WriteString("    public void start() {\n")
	sb.WriteString("        server.start();\n")
	sb.WriteString("        System.out.println(\"Server started on port \" + server.getAddress().getPort());\n")
	sb.WriteString("    }\n\n")

	// Stop method
	sb.WriteString("    public void stop() {\n")
	sb.WriteString("        server.stop(0);\n")
	sb.WriteString("    }\n\n")

	// Handle request method
	sb.WriteString("    private void handleRequest(HttpExchange exchange) throws IOException {\n")
	sb.WriteString("        try {\n")
	sb.WriteString("            if (!\"POST\".equals(exchange.getRequestMethod())) {\n")
	sb.WriteString("                sendError(exchange, -32600, \"Invalid Request - only POST allowed\");\n")
	sb.WriteString("                return;\n")
	sb.WriteString("            }\n\n")
	sb.WriteString("            // Read request body\n")
	sb.WriteString("            String requestBody = new String(exchange.getRequestBody().readAllBytes());\n\n")
	sb.WriteString("            // Parse JSON-RPC request\n")
	sb.WriteString("            Map<String, Object> request = jsonParser.fromJson(requestBody, Map.class);\n\n")
	sb.WriteString("            // Handle the request\n")
	sb.WriteString("            Map<String, Object> response = handleJsonRpcRequest(request);\n\n")
	sb.WriteString("            // Send response\n")
	sb.WriteString("            String responseBody = jsonParser.toJson(response);\n")
	sb.WriteString("            exchange.getResponseHeaders().set(\"Content-Type\", \"application/json\");\n")
	sb.WriteString("            exchange.sendResponseHeaders(200, responseBody.getBytes().length);\n")
	sb.WriteString("            try (OutputStream os = exchange.getResponseBody()) {\n")
	sb.WriteString("                os.write(responseBody.getBytes());\n")
	sb.WriteString("            }\n")
	sb.WriteString("        } catch (Exception e) {\n")
	sb.WriteString("            sendError(exchange, -32603, \"Internal error: \" + e.getMessage());\n")
	sb.WriteString("        }\n")
	sb.WriteString("    }\n\n")

	// Error response helper
	sb.WriteString("    private void sendError(HttpExchange exchange, int code, String message) throws IOException {\n")
	sb.WriteString("        Map<String, Object> error = Map.of(\n")
	sb.WriteString("            \"jsonrpc\", \"2.0\",\n")
	sb.WriteString("            \"error\", Map.of(\n")
	sb.WriteString("                \"code\", code,\n")
	sb.WriteString("                \"message\", message\n")
	sb.WriteString("            ),\n")
	sb.WriteString("            \"id\", null\n")
	sb.WriteString("        );\n")
	sb.WriteString("        String errorBody = jsonParser.toJson(error);\n")
	sb.WriteString("        exchange.getResponseHeaders().set(\"Content-Type\", \"application/json\");\n")
	sb.WriteString("        exchange.sendResponseHeaders(200, errorBody.getBytes().length);\n")
	sb.WriteString("        try (OutputStream os = exchange.getResponseBody()) {\n")
	sb.WriteString("            os.write(errorBody.getBytes());\n")
	sb.WriteString("        }\n")
	sb.WriteString("    }\n\n")

	// Handle JSON-RPC request
	sb.WriteString("    private Map<String, Object> handleJsonRpcRequest(Map<String, Object> request) {\n")
	sb.WriteString("        // Validate jsonrpc field\n")
	sb.WriteString("        Object jsonrpc = request.get(\"jsonrpc\");\n")
	sb.WriteString("        if (jsonrpc == null || !\"2.0\".equals(jsonrpc)) {\n")
	sb.WriteString("            Object id = request.get(\"id\");\n")
	sb.WriteString("            return Map.of(\n")
	sb.WriteString("                \"jsonrpc\", \"2.0\",\n")
	sb.WriteString("                \"error\", Map.of(\n")
	sb.WriteString("                    \"code\", -32600,\n")
	sb.WriteString("                    \"message\", \"Invalid Request - jsonrpc must be '2.0'\"\n")
	sb.WriteString("                ),\n")
	sb.WriteString("                \"id\", id\n")
	sb.WriteString("            );\n")
	sb.WriteString("        }\n\n")
	sb.WriteString("        String method = (String) request.get(\"method\");\n")
	sb.WriteString("        Object id = request.get(\"id\");\n")
	sb.WriteString("        Object params = request.get(\"params\");\n\n")
	sb.WriteString("        if (\"pulserpc-idl\".equals(method)) {\n")
	sb.WriteString("            // Return IDL definition - use embedded constant\n")
	sb.WriteString("            try {\n")
	sb.WriteString("                Object idlDoc = jsonParser.fromJson(IDL_JSON, Object.class);\n")
	sb.WriteString("                return Map.of(\n")
	sb.WriteString("                    \"jsonrpc\", \"2.0\",\n")
	sb.WriteString("                    \"result\", idlDoc,\n")
	sb.WriteString("                    \"id\", id\n")
	sb.WriteString("                );\n")
	sb.WriteString("            } catch (Exception e) {\n")
	sb.WriteString("                return Map.of(\n")
	sb.WriteString("                    \"jsonrpc\", \"2.0\",\n")
	sb.WriteString("                    \"error\", Map.of(\n")
	sb.WriteString("                        \"code\", -32603,\n")
	sb.WriteString("                        \"message\", \"Failed to load IDL: \" + e.getMessage()\n")
	sb.WriteString("                    ),\n")
	sb.WriteString("                    \"id\", id\n")
	sb.WriteString("                );\n")
	sb.WriteString("            }\n")
	sb.WriteString("        }\n\n")
	sb.WriteString("        // Parse method name: interface.method\n")
	sb.WriteString("        String[] parts = method.split(\"\\\\.\", 2);\n")
	sb.WriteString("        if (parts.length != 2) {\n")
	sb.WriteString("            return Map.of(\n")
	sb.WriteString("                \"jsonrpc\", \"2.0\",\n")
	sb.WriteString("                \"error\", Map.of(\n")
	sb.WriteString("                    \"code\", -32601,\n")
	sb.WriteString("                    \"message\", \"Invalid method format: \" + method\n")
	sb.WriteString("                ),\n")
	sb.WriteString("                \"id\", id\n")
	sb.WriteString("            );\n")
	sb.WriteString("        }\n\n")
	sb.WriteString("        String interfaceName = parts[0];\n")
	sb.WriteString("        String methodName = parts[1];\n\n")
	sb.WriteString("        // Find interface handler\n")
	sb.WriteString("        Object handler = interfaceHandlers.get(interfaceName);\n")
	sb.WriteString("        if (handler == null) {\n")
	sb.WriteString("            return Map.of(\n")
	sb.WriteString("                \"jsonrpc\", \"2.0\",\n")
	sb.WriteString("                \"error\", Map.of(\n")
	sb.WriteString("                    \"code\", -32601,\n")
	sb.WriteString("                    \"message\", \"Interface not found: \" + interfaceName\n")
	sb.WriteString("                ),\n")
	sb.WriteString("                \"id\", id\n")
	sb.WriteString("            );\n")
	sb.WriteString("        }\n\n")
	sb.WriteString("                // Invoke method using reflection\n")
	sb.WriteString("        try {\n")
	sb.WriteString("            // Handle null params (methods with no parameters)\n")
	sb.WriteString("            List<?> paramList;\n")
	sb.WriteString("            if (params == null) {\n")
	sb.WriteString("                paramList = new ArrayList<>();\n")
	sb.WriteString("            } else if (params instanceof List) {\n")
	sb.WriteString("                paramList = (List<?>) params;\n")
	sb.WriteString("            } else {\n")
	sb.WriteString("                return Map.of(\n")
	sb.WriteString("                    \"jsonrpc\", \"2.0\",\n")
	sb.WriteString("                    \"error\", Map.of(\n")
	sb.WriteString("                        \"code\", -32602,\n")
	sb.WriteString("                        \"message\", \"Invalid params: must be an array\"\n")
	sb.WriteString("                    ),\n")
	sb.WriteString("                    \"id\", id\n")
	sb.WriteString("                );\n")
	sb.WriteString("            }\n\n")
	sb.WriteString("            Class<?> handlerClass = handler.getClass();\n")
	sb.WriteString("            Method[] methods = handlerClass.getMethods();\n")
	sb.WriteString("            Method targetMethod = null;\n")
	sb.WriteString("            boolean methodNameFound = false;\n")
	sb.WriteString("            for (Method m : methods) {\n")
	sb.WriteString("                if (m.getName().equals(methodName)) {\n")
	sb.WriteString("                    methodNameFound = true;\n")
	sb.WriteString("                    if (m.getParameterCount() == paramList.size()) {\n")
	sb.WriteString("                        targetMethod = m;\n")
	sb.WriteString("                        break;\n")
	sb.WriteString("                    }\n")
	sb.WriteString("                }\n")
	sb.WriteString("            }\n\n")
	sb.WriteString("            if (!methodNameFound) {\n")
	sb.WriteString("                return Map.of(\n")
	sb.WriteString("                    \"jsonrpc\", \"2.0\",\n")
	sb.WriteString("                    \"error\", Map.of(\n")
	sb.WriteString("                        \"code\", -32601,\n")
	sb.WriteString("                        \"message\", \"Method not found: \" + method\n")
	sb.WriteString("                    ),\n")
	sb.WriteString("                    \"id\", id\n")
	sb.WriteString("                );\n")
	sb.WriteString("            }\n\n")
	sb.WriteString("            if (targetMethod == null) {\n")
	sb.WriteString("                return Map.of(\n")
	sb.WriteString("                    \"jsonrpc\", \"2.0\",\n")
	sb.WriteString("                    \"error\", Map.of(\n")
	sb.WriteString("                        \"code\", -32602,\n")
	sb.WriteString("                        \"message\", \"Invalid params: parameter count mismatch for \" + method\n")
	sb.WriteString("                    ),\n")
	sb.WriteString("                    \"id\", id\n")
	sb.WriteString("                );\n")
	sb.WriteString("            }\n\n")
	sb.WriteString("            // Deserialize parameters using generic types\n")
	sb.WriteString("            java.lang.reflect.Type[] paramTypes = targetMethod.getGenericParameterTypes();\n")
	sb.WriteString("            Object[] deserializedParams = new Object[paramList.size()];\n")
	sb.WriteString("            try {\n")
	sb.WriteString("                for (int i = 0; i < paramList.size(); i++) {\n")
	sb.WriteString("                    String paramJson = jsonParser.toJson(paramList.get(i));\n")
	sb.WriteString("                    deserializedParams[i] = jsonParser.fromJson(paramJson, paramTypes[i]);\n")
	sb.WriteString("                }\n")
	sb.WriteString("            } catch (Exception deserEx) {\n")
	sb.WriteString("                // Deserialization errors should return -32602 (Invalid params)\n")
	sb.WriteString("                return Map.of(\n")
	sb.WriteString("                    \"jsonrpc\", \"2.0\",\n")
	sb.WriteString("                    \"error\", Map.of(\n")
	sb.WriteString("                        \"code\", -32602,\n")
	sb.WriteString("                        \"message\", \"Invalid params: \" + deserEx.getMessage()\n")
	sb.WriteString("                    ),\n")
	sb.WriteString("                    \"id\", id\n")
	sb.WriteString("                );\n")
	sb.WriteString("            }\n\n")
	sb.WriteString("            // Invoke method\n")
	sb.WriteString("            Object result = targetMethod.invoke(handler, deserializedParams);\n\n")
	sb.WriteString("            // Return response (use HashMap to allow null result values)\n")
	sb.WriteString("            Map<String, Object> response = new HashMap<>();\n")
	sb.WriteString("            response.put(\"jsonrpc\", \"2.0\");\n")
	sb.WriteString("            response.put(\"result\", result);\n")
	sb.WriteString("            response.put(\"id\", id);\n")
	sb.WriteString("            return response;\n")
	sb.WriteString("        } catch (java.lang.reflect.InvocationTargetException ite) {\n")
	sb.WriteString("            // Unwrap InvocationTargetException to get the actual exception\n")
	sb.WriteString("            Throwable cause = ite.getCause();\n")
	sb.WriteString("            if (cause instanceof RPCError) {\n")
	sb.WriteString("                RPCError rpcErr = (RPCError) cause;\n")
	sb.WriteString("                return Map.of(\n")
	sb.WriteString("                    \"jsonrpc\", \"2.0\",\n")
	sb.WriteString("                    \"error\", Map.of(\n")
	sb.WriteString("                        \"code\", rpcErr.getCode(),\n")
	sb.WriteString("                        \"message\", rpcErr.getMessage(),\n")
	sb.WriteString("                        \"data\", rpcErr.getData()\n")
	sb.WriteString("                    ),\n")
	sb.WriteString("                    \"id\", id\n")
	sb.WriteString("                );\n")
	sb.WriteString("            } else {\n")
	sb.WriteString("                // Print stack trace for unexpected exceptions\n")
	sb.WriteString("                System.err.println(\"Exception in method \" + method + \":\");\n")
	sb.WriteString("                if (cause != null) {\n")
	sb.WriteString("                    cause.printStackTrace();\n")
	sb.WriteString("                } else {\n")
	sb.WriteString("                    ite.printStackTrace();\n")
	sb.WriteString("                }\n")
	sb.WriteString("                return Map.of(\n")
	sb.WriteString("                    \"jsonrpc\", \"2.0\",\n")
	sb.WriteString("                    \"error\", Map.of(\n")
	sb.WriteString("                        \"code\", -32603,\n")
	sb.WriteString("                        \"message\", \"Internal error: \" + (cause != null ? cause.getMessage() : ite.getMessage())\n")
	sb.WriteString("                    ),\n")
	sb.WriteString("                    \"id\", id\n")
	sb.WriteString("                );\n")
	sb.WriteString("            }\n")
	sb.WriteString("        } catch (RPCError rpcErr) {\n")
	sb.WriteString("            // RPCError is expected and can be thrown by implementations\n")
	sb.WriteString("            return Map.of(\n")
	sb.WriteString("                \"jsonrpc\", \"2.0\",\n")
	sb.WriteString("                \"error\", Map.of(\n")
	sb.WriteString("                    \"code\", rpcErr.getCode(),\n")
	sb.WriteString("                    \"message\", rpcErr.getMessage(),\n")
	sb.WriteString("                    \"data\", rpcErr.getData()\n")
	sb.WriteString("                ),\n")
	sb.WriteString("                \"id\", id\n")
	sb.WriteString("            );\n")
	sb.WriteString("        } catch (Exception e) {\n")
	sb.WriteString("            // Print stack trace for unexpected exceptions\n")
	sb.WriteString("            System.err.println(\"Exception in method \" + method + \":\");\n")
	sb.WriteString("            e.printStackTrace();\n")
	sb.WriteString("            return Map.of(\n")
	sb.WriteString("                \"jsonrpc\", \"2.0\",\n")
	sb.WriteString("                \"error\", Map.of(\n")
	sb.WriteString("                    \"code\", -32603,\n")
	sb.WriteString("                    \"message\", \"Internal error: \" + e.getMessage()\n")
	sb.WriteString("                ),\n")
	sb.WriteString("                \"id\", id\n")
	sb.WriteString("            );\n")
	sb.WriteString("        }\n")
	sb.WriteString("    }\n")

	sb.WriteString("}\n")

	return sb.String()
}

// generateClientJava generates the Client.java file
func generateClientJava(_ *parser.IDL, namespaceMap map[string]*NamespaceTypes, basePackage string, packageDecl string) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	if packageDecl != "" {
		sb.WriteString(fmt.Sprintf("package %s;\n\n", packageDecl))
	}
	sb.WriteString("import com.bitmechanic.pulserpc.*;\n")
	sb.WriteString("import java.io.*;\n")
	sb.WriteString("import java.net.*;\n")
	sb.WriteString("import java.net.http.*;\n")
	sb.WriteString("import java.net.http.HttpRequest.*;\n")
	sb.WriteString("import java.net.http.HttpResponse.*;\n")
	sb.WriteString("import java.util.*;\n")
	sb.WriteString("import java.util.concurrent.*;\n\n")

	sb.WriteString("public class Client {\n")
	sb.WriteString("    private final HttpClient httpClient;\n")
	sb.WriteString("    private final String baseUrl;\n")
	sb.WriteString("    private final JsonParser jsonParser;\n")
	sb.WriteString("    private final Map<String, Map<String, Object>> allStructs;\n")
	sb.WriteString("    private final Map<String, Map<String, Object>> allEnums;\n\n")

	// Constructor
	sb.WriteString("    public Client(String baseUrl, JsonParser jsonParser) {\n")
	sb.WriteString("        this.httpClient = HttpClient.newHttpClient();\n")
	sb.WriteString("        this.baseUrl = baseUrl;\n")
	sb.WriteString("        this.jsonParser = jsonParser;\n")
	sb.WriteString("        this.allStructs = new HashMap<>();\n")
	sb.WriteString("        this.allEnums = new HashMap<>();\n\n")

	// Collect all structs and enums from namespace IDL classes
	for namespace := range namespaceMap {
		if namespace != "" {
			nsPackage := basePackage + "." + namespace
			sb.WriteString(fmt.Sprintf("        this.allStructs.putAll(%s.%sIdl.ALL_STRUCTS);\n", nsPackage, namespace))
			sb.WriteString(fmt.Sprintf("        this.allEnums.putAll(%s.%sIdl.ALL_ENUMS);\n", nsPackage, namespace))
		}
	}

	sb.WriteString("    }\n\n")

	// Call method
	sb.WriteString("    @SuppressWarnings(\"unchecked\")\n")
	sb.WriteString("    public Map<String, Object> call(String method, Map<String, Object> params) throws Exception {\n")
	sb.WriteString("        Map<String, Object> request = Map.of(\n")
	sb.WriteString("            \"jsonrpc\", \"2.0\",\n")
	sb.WriteString("            \"method\", method,\n")
	sb.WriteString("            \"params\", params,\n")
	sb.WriteString("            \"id\", 1\n")
	sb.WriteString("        );\n\n")
	sb.WriteString("        String requestBody = jsonParser.toJson(request);\n\n")
	sb.WriteString("        HttpRequest httpRequest = HttpRequest.newBuilder()\n")
	sb.WriteString("            .uri(URI.create(baseUrl))\n")
	sb.WriteString("            .header(\"Content-Type\", \"application/json\")\n")
	sb.WriteString("            .POST(HttpRequest.BodyPublishers.ofString(requestBody))\n")
	sb.WriteString("            .build();\n\n")
	sb.WriteString("        HttpResponse<String> response = httpClient.send(httpRequest, HttpResponse.BodyHandlers.ofString());\n\n")
	sb.WriteString("        if (response.statusCode() != 200) {\n")
	sb.WriteString("            throw new RuntimeException(\"HTTP error: \" + response.statusCode());\n")
	sb.WriteString("        }\n\n")
	sb.WriteString("        Map<String, Object> jsonResponse = jsonParser.fromJson(response.body(), Map.class);\n\n")
	sb.WriteString("        if (jsonResponse.containsKey(\"error\")) {\n")
	sb.WriteString("            Map<String, Object> error = (Map<String, Object>) jsonResponse.get(\"error\");\n")
	sb.WriteString("            throw new RPCError(\n")
	sb.WriteString("                ((Number) error.get(\"code\")).intValue(),\n")
	sb.WriteString("                (String) error.get(\"message\"),\n")
	sb.WriteString("                error.get(\"data\")\n")
	sb.WriteString("            );\n")
	sb.WriteString("        }\n\n")
	sb.WriteString("        return (Map<String, Object>) jsonResponse.get(\"result\");\n")
	sb.WriteString("    }\n")

	sb.WriteString("}\n")

	return sb.String()
}

// Helper functions

// getSimpleName extracts the simple name from a fully qualified name
// e.g., "inc.Status" -> "Status", "Response" -> "Response"
func getSimpleName(qualifiedName string) string {
	if idx := strings.LastIndex(qualifiedName, "."); idx >= 0 {
		return qualifiedName[idx+1:]
	}
	return qualifiedName
}

func getJavaType(typeDef *parser.Type, enumMap map[string]*parser.Enum) string {
	if typeDef.IsBuiltIn() {
		switch typeDef.BuiltIn {
		case "string":
			return "String"
		case "int":
			return "int"
		case "float":
			return "double"
		case "bool":
			return "boolean"
		}
	} else if typeDef.IsArray() {
		elementType := getJavaType(typeDef.Array, enumMap)
		return fmt.Sprintf("List<%s>", elementType)
	} else if typeDef.IsMap() {
		valueType := getJavaType(typeDef.MapValue, enumMap)
		return fmt.Sprintf("Map<String, %s>", valueType)
	} else if typeDef.IsUserDefined() {
		// Lookup uses fully qualified name, but return simple name
		if enumMap[typeDef.UserDefined] != nil {
			return getSimpleName(typeDef.UserDefined)
		}
		return getSimpleName(typeDef.UserDefined)
	}
	return "Object"
}

// getBoxedJavaType returns the boxed Java type for primitives when used in generics
func getBoxedJavaType(typeDef *parser.Type, enumMap map[string]*parser.Enum) string {
	if typeDef.IsBuiltIn() {
		switch typeDef.BuiltIn {
		case "string":
			return "String"
		case "int":
			return "Integer"
		case "float":
			return "Double"
		case "bool":
			return "Boolean"
		}
	} else if typeDef.IsArray() {
		elementType := getBoxedJavaType(typeDef.Array, enumMap)
		return fmt.Sprintf("java.util.List<%s>", elementType)
	} else if typeDef.IsMap() {
		valueType := getBoxedJavaType(typeDef.MapValue, enumMap)
		return fmt.Sprintf("java.util.Map<String, %s>", valueType)
	} else if typeDef.IsUserDefined() {
		// Lookup uses fully qualified name, but return simple name
		if enumMap[typeDef.UserDefined] != nil {
			return getSimpleName(typeDef.UserDefined)
		}
		return getSimpleName(typeDef.UserDefined)
	}
	return "Object"
}

func writeTypeDictJava(sb *strings.Builder, typeDef *parser.Type) {
	// Emit Java statements that populate a variable named `typeDef` in scope.
	if typeDef.IsBuiltIn() {
		fmt.Fprintf(sb, "                typeDef.put(\"builtIn\", \"%s\");\n", typeDef.BuiltIn)
	} else if typeDef.IsArray() {
		sb.WriteString("                java.util.Map<String, Object> inner = new java.util.HashMap<>();\n")
		writeTypeDictJava(sb, typeDef.Array)
		sb.WriteString("                typeDef.put(\"array\", inner);\n")
	} else if typeDef.IsMap() {
		sb.WriteString("                java.util.Map<String, Object> inner = new java.util.HashMap<>();\n")
		writeTypeDictJava(sb, typeDef.MapValue)
		sb.WriteString("                typeDef.put(\"mapValue\", inner);\n")
	} else if typeDef.IsUserDefined() {
		fmt.Fprintf(sb, "                typeDef.put(\"userDefined\", \"%s\");\n", typeDef.UserDefined)
	}
}

func toCamelCase(s string) string {
	if s == "" {
		return s
	}
	// Simple camelCase conversion - first letter lowercase
	return strings.ToLower(s[:1]) + s[1:]
}

func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	// Capitalize first letter
	return strings.ToUpper(s[:1]) + s[1:]
}

// getGetterName generates the getter method name for a field
// This matches how struct generation creates getters: "get" + capitalizeFirst(fieldName)
func getGetterName(fieldName string) string {
	return "get" + capitalizeFirst(fieldName)
}

// generateTestInterfaceImplFile generates a separate implementation file for an interface
func generateTestInterfaceImplFile(iface *parser.Interface, packageName string, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, jsonLib string, basePackage string) string {
	_ = jsonLib
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString(fmt.Sprintf("package %s;\n\n", packageName))

	// Add imports
	imports := make(map[string]bool)
	interfaceName := GetBaseName(iface.Name)
	interfaceNamespace := GetNamespaceFromType(iface.Name, iface.Namespace)
	interfacePackage := basePackage
	if interfaceNamespace != "" {
		interfacePackage = basePackage + "." + interfaceNamespace
	}
	if interfacePackage != packageName {
		imports[interfacePackage+"."+interfaceName] = true
	}

	// Add imports for method types
	for _, method := range iface.Methods {
		if method.ReturnType != nil {
			addTypeImports(method.ReturnType, basePackage, packageName, imports, structMap, enumMap)
		}
		for _, param := range method.Parameters {
			addTypeImports(param.Type, basePackage, packageName, imports, structMap, enumMap)
		}
	}

	for imp := range imports {
		sb.WriteString(fmt.Sprintf("import %s;\n", imp))
	}
	if len(imports) > 0 {
		sb.WriteString("\n")
	}

	implName := interfaceName + "Impl"
	if interfacePackage != packageName {
		sb.WriteString(fmt.Sprintf("public class %s implements %s.%s {\n", implName, interfacePackage, interfaceName))
	} else {
		sb.WriteString(fmt.Sprintf("public class %s implements %s {\n", implName, interfaceName))
	}

	// Generate method implementations
	for _, method := range iface.Methods {
		returnType := "void"
		if method.ReturnType != nil {
			returnType = getJavaTypeWithPackage(method.ReturnType, enumMap, basePackage, packageName, structMap)
		}

		fmt.Fprintf(&sb, "    @Override\n")
		fmt.Fprintf(&sb, "    public %s %s(", returnType, method.Name)

		// Parameters
		for i, param := range method.Parameters {
			if i > 0 {
				sb.WriteString(", ")
			}
			paramType := getJavaTypeWithPackage(param.Type, enumMap, basePackage, packageName, structMap)
			fmt.Fprintf(&sb, "%s %s", paramType, param.Name)
		}
		sb.WriteString(") {\n")

		// Generate implementation based on method name (similar to C# version)
		writeTestMethodBody(&sb, iface, method, structMap, enumMap, basePackage, packageName)

		sb.WriteString("    }\n\n")
	}

	sb.WriteString("}\n")

	return sb.String()
}

// writeTestMethodBody generates the body of a test method implementation
func writeTestMethodBody(sb *strings.Builder, iface *parser.Interface, method *parser.Method, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, basePackage string, packageName string) {
	_ = structMap
	interfaceName := iface.Name
	methodName := method.Name

	// Handle specific methods based on IDL comments (similar to C# implementation)
	switch interfaceName {
	case "A":
		switch methodName {
		case "add":
			sb.WriteString("        return a + b;\n")
			return
		case "calc":
			sb.WriteString("        double result = 0.0;\n")
			sb.WriteString("        if (nums.size() > 0) {\n")
			sb.WriteString("            result = nums.get(0);\n")
			sb.WriteString("            for (int i = 1; i < nums.size(); i++) {\n")
			sb.WriteString("                if (operation == ")
			opEnumType := getJavaTypeWithPackage(method.Parameters[1].Type, enumMap, basePackage, packageName, structMap)
			sb.WriteString(opEnumType)
			sb.WriteString(".add) {\n")
			sb.WriteString("                    result += nums.get(i);\n")
			sb.WriteString("                } else if (operation == ")
			sb.WriteString(opEnumType)
			sb.WriteString(".multiply) {\n")
			sb.WriteString("                    result *= nums.get(i);\n")
			sb.WriteString("                }\n")
			sb.WriteString("            }\n")
			sb.WriteString("        }\n")
			sb.WriteString("        return result;\n")
			return
		case "sqrt":
			sb.WriteString("        return Math.sqrt(a);\n")
			return
		case "repeat":
			respType := getJavaTypeWithPackage(method.ReturnType, enumMap, basePackage, packageName, structMap)
			sb.WriteString("        String toRepeat = req1.getTo_repeat();\n")
			sb.WriteString("        if (req1.getForce_uppercase()) toRepeat = toRepeat.toUpperCase();\n")
			sb.WriteString("        java.util.List<String> items = new java.util.ArrayList<>();\n")
			sb.WriteString("        for (int i = 0; i < req1.getCount(); i++) {\n")
			sb.WriteString("            items.add(toRepeat);\n")
			sb.WriteString("        }\n")
			fmt.Fprintf(sb, "        %s response = new %s();\n", respType, respType)
			sb.WriteString("        response.setStatus(")
			statusEnumType := getJavaTypeWithPackage(&parser.Type{UserDefined: "inc.Status"}, enumMap, basePackage, packageName, structMap)
			sb.WriteString(statusEnumType)
			sb.WriteString(".ok);\n")
			sb.WriteString("        response.setCount(req1.getCount());\n")
			sb.WriteString("        response.setItems(items);\n")
			sb.WriteString("        return response;\n")
			return
		case "say_hi":
			respType := getJavaTypeWithPackage(method.ReturnType, enumMap, basePackage, packageName, structMap)
			fmt.Fprintf(sb, "        %s response = new %s();\n", respType, respType)
			sb.WriteString("        response.setHi(\"hi\");\n")
			sb.WriteString("        return response;\n")
			return
		case "repeat_num":
			sb.WriteString("        java.util.List<Integer> result = new java.util.ArrayList<>();\n")
			sb.WriteString("        for (int i = 0; i < count; i++) {\n")
			sb.WriteString("            result.add(num);\n")
			sb.WriteString("        }\n")
			sb.WriteString("        return result;\n")
			return
		case "putPerson":
			sb.WriteString("        return p.getPersonId();\n")
			return
		}
	case "B":
		switch methodName {
		case "echo":
			sb.WriteString("        if (\"return-null\".equals(s)) return null;\n")
			sb.WriteString("        return s;\n")
			return
		}
	}

	// Default implementation
	if method.ReturnType != nil {
		if method.ReturnType.IsBuiltIn() {
			switch method.ReturnType.BuiltIn {
			case "string":
				sb.WriteString("        return \"test\";\n")
			case "int":
				sb.WriteString("        return 42;\n")
			case "float":
				sb.WriteString("        return 3.14;\n")
			case "bool":
				sb.WriteString("        return true;\n")
			default:
				sb.WriteString("        return null;\n")
			}
		} else if method.ReturnType.IsArray() {
			elementType := getJavaTypeWithPackage(method.ReturnType.Array, enumMap, basePackage, packageName, structMap)
			fmt.Fprintf(sb, "        return new java.util.ArrayList<%s>();\n", elementType)
		} else {
			sb.WriteString("        return null;\n")
		}
	}
}

// generateTestServerJava generates TestServer.java
func generateTestServerJava(idl *parser.IDL, jsonLib string, basePackage string, namespaceMap map[string]*NamespaceTypes) string {
	_ = namespaceMap
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString(fmt.Sprintf("package %s;\n\n", basePackage))
	sb.WriteString("import com.bitmechanic.pulserpc.*;\n")

	// Add imports for interface implementations
	imports := make(map[string]bool)
	for _, iface := range idl.Interfaces {
		ifaceNamespace := GetNamespaceFromType(iface.Name, iface.Namespace)
		ifacePackage := basePackage
		if ifaceNamespace != "" {
			ifacePackage = basePackage + "." + ifaceNamespace
		}
		implName := GetBaseName(iface.Name) + "Impl"
		imports[ifacePackage+"."+implName] = true
	}
	for imp := range imports {
		sb.WriteString(fmt.Sprintf("import %s;\n", imp))
	}
	if len(imports) > 0 {
		sb.WriteString("\n")
	}

	sb.WriteString("public class TestServer extends Server {\n")
	sb.WriteString("    public TestServer(int port, JsonParser jsonParser) throws Exception {\n")
	sb.WriteString("        super(port, jsonParser);\n")
	sb.WriteString("    }\n\n")

	sb.WriteString("    public static void main(String[] args) {\n")
	sb.WriteString("        try {\n")
	switch jsonLib {
	case "jackson":
		sb.WriteString("            JsonParser jsonParser = new JacksonJsonParser();\n")
	default:
		sb.WriteString("            JsonParser jsonParser = new GsonJsonParser();\n")
	}
	sb.WriteString("            TestServer server = new TestServer(8080, jsonParser);\n\n")

	// Register interface implementations
	for _, iface := range idl.Interfaces {
		ifaceNamespace := GetNamespaceFromType(iface.Name, iface.Namespace)
		ifacePackage := basePackage
		if ifaceNamespace != "" {
			ifacePackage = basePackage + "." + ifaceNamespace
		}
		implName := GetBaseName(iface.Name) + "Impl"
		interfaceName := GetBaseName(iface.Name)
		fmt.Fprintf(&sb, "            server.register(\"%s\", new %s.%s());\n", interfaceName, ifacePackage, implName)
	}

	sb.WriteString("            server.start();\n")
	sb.WriteString("            System.out.println(\"Test server started on port 8080\");\n")
	sb.WriteString("            // Keep server running indefinitely\n")
	sb.WriteString("            Object lock = new Object();\n")
	sb.WriteString("            synchronized (lock) {\n")
	sb.WriteString("                lock.wait();\n")
	sb.WriteString("            }\n")
	sb.WriteString("        } catch (Exception e) {\n")
	sb.WriteString("            System.err.println(\"Fatal error: \" + e.getMessage());\n")
	sb.WriteString("            e.printStackTrace();\n")
	sb.WriteString("            System.exit(1);\n")
	sb.WriteString("        }\n")
	sb.WriteString("    }\n")
	sb.WriteString("}\n")

	return sb.String()
}

// generateTestClientJava generates TestClient.java
func generateTestClientJava(idl *parser.IDL, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, jsonLib string, basePackage string, namespaceMap map[string]*NamespaceTypes) string {
	_ = namespaceMap
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString(fmt.Sprintf("package %s;\n\n", basePackage))
	sb.WriteString("import com.bitmechanic.pulserpc.*;\n")

	// Add imports for client classes
	imports := make(map[string]bool)
	for _, s := range idl.Structs {
		typeNamespace := GetNamespaceFromType(s.Name, s.Namespace)
		structPackage := basePackage
		if typeNamespace != "" {
			structPackage = basePackage + "." + typeNamespace
		}
		if structPackage != basePackage {
			imports[structPackage+"."+GetBaseName(s.Name)] = true
		}
	}
	for _, iface := range idl.Interfaces {
		ifaceNamespace := GetNamespaceFromType(iface.Name, iface.Namespace)
		ifacePackage := basePackage
		if ifaceNamespace != "" {
			ifacePackage = basePackage + "." + ifaceNamespace
		}
		clientName := GetBaseName(iface.Name) + "Client"
		imports[ifacePackage+"."+clientName] = true
	}
	for imp := range imports {
		sb.WriteString(fmt.Sprintf("import %s;\n", imp))
	}
	if len(imports) > 0 {
		sb.WriteString("\n")
	}

	sb.WriteString("public class TestClient {\n")
	sb.WriteString("    public static void main(String[] args) throws Exception {\n")
	switch jsonLib {
	case "jackson":
		sb.WriteString("        JsonParser jsonParser = new JacksonJsonParser();\n")
	default:
		sb.WriteString("        JsonParser jsonParser = new GsonJsonParser();\n")
	}
	sb.WriteString("        String baseUrl = args.length > 0 ? args[0] : \"http://localhost:8080\";\n")
	sb.WriteString("        Transport transport = new HTTPTransport(baseUrl, jsonParser);\n\n")

	// Create client instances and make test calls
	for _, iface := range idl.Interfaces {
		ifaceNamespace := GetNamespaceFromType(iface.Name, iface.Namespace)
		ifacePackage := basePackage
		if ifaceNamespace != "" {
			ifacePackage = basePackage + "." + ifaceNamespace
		}
		clientName := GetBaseName(iface.Name) + "Client"
		clientVar := strings.ToLower(GetBaseName(iface.Name)) + "Client"
		fmt.Fprintf(&sb, "        %s %s = new %s.%s(transport, jsonParser);\n", ifacePackage+"."+clientName, clientVar, ifacePackage, clientName)

		// Generate test calls for each method
		for _, method := range iface.Methods {
			sb.WriteString("        try {\n")
			fmt.Fprintf(&sb, "            ")
			if method.ReturnType != nil {
				sb.WriteString("var result = ")
			}
			fmt.Fprintf(&sb, "%s.%s(", clientVar, method.Name)
			// Generate test parameters
			for i, param := range method.Parameters {
				if i > 0 {
					sb.WriteString(", ")
				}
				writeTestParamValue(&sb, param, structMap, enumMap, basePackage, ifacePackage, jsonLib)
			}

			sb.WriteString(");\n")
			fmt.Fprintf(&sb, "            System.out.println(\"✓ %s.%s passed\");\n", GetBaseName(iface.Name), method.Name)
			sb.WriteString("        } catch (Exception e) {\n")
			fmt.Fprintf(&sb, "            System.err.println(\"✗ %s.%s failed: \" + e.getMessage());\n", GetBaseName(iface.Name), method.Name)
			sb.WriteString("        }\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("        System.out.println(\"Test client completed\");\n")
	sb.WriteString("    }\n")
	sb.WriteString("}\n")

	return sb.String()
}

// writeTestParamValue generates a test parameter value
func writeTestParamValue(sb *strings.Builder, param *parser.Parameter, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, basePackage string, currentPackage string, _ string) {
	_ = structMap
	if param.Type.IsBuiltIn() {
		switch param.Type.BuiltIn {
		case "string":
			fmt.Fprintf(sb, "\"test%s\"", param.Name)
		case "int":
			sb.WriteString("42")
		case "float":
			sb.WriteString("3.14")
		case "bool":
			sb.WriteString("true")
		default:
			sb.WriteString("null")
		}
	} else if param.Type.IsArray() {
		switch param.Type.Array.BuiltIn {
		case "float":
			sb.WriteString("java.util.Arrays.asList(1.1, 2.2, 3.3)")
		case "int":
			sb.WriteString("java.util.Arrays.asList(1, 2, 3)")
		default:
			elementType := getJavaTypeWithPackage(param.Type.Array, enumMap, basePackage, currentPackage, structMap)
			fmt.Fprintf(sb, "java.util.Arrays.asList(/* %s values */)", elementType)
		}
	} else if param.Type.IsUserDefined() {
		typeName := GetBaseName(param.Type.UserDefined)
		typeNamespace := GetNamespaceFromType(param.Type.UserDefined, "")
		fullTypeName := typeName
		if typeNamespace != "" {
			fullTypeName = basePackage + "." + typeNamespace + "." + typeName
		}

		if _, isEnum := enumMap[param.Type.UserDefined]; isEnum {
			// Find first enum value
			enum := enumMap[param.Type.UserDefined]
			if len(enum.Values) > 0 {
				fmt.Fprintf(sb, "%s.%s", fullTypeName, enum.Values[0].Name)
			} else {
				sb.WriteString("null")
			}
		} else {
			// It's a struct
			fmt.Fprintf(sb, "new %s()", fullTypeName)
		}
	} else {
		sb.WriteString("null")
	}
}

// generatePomXml generates pom.xml for Maven builds
func generatePomXml(jsonLib string) string {
	var sb strings.Builder

	sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	sb.WriteString("<project xmlns=\"http://maven.apache.org/POM/4.0.0\"\n")
	sb.WriteString("         xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\"\n")
	sb.WriteString("         xsi:schemaLocation=\"http://maven.apache.org/POM/4.0.0\n")
	sb.WriteString("                             http://maven.apache.org/xsd/maven-4.0.0.xsd\">\n")
	sb.WriteString("    <modelVersion>4.0.0</modelVersion>\n\n")
	sb.WriteString("    <groupId>com.example</groupId>\n")
	sb.WriteString("    <artifactId>pulserpc-test</artifactId>\n")
	sb.WriteString("    <version>1.0.0</version>\n")
	sb.WriteString("    <packaging>jar</packaging>\n\n")
	sb.WriteString("    <properties>\n")
	sb.WriteString("        <maven.compiler.source>11</maven.compiler.source>\n")
	sb.WriteString("        <maven.compiler.target>11</maven.compiler.target>\n")
	sb.WriteString("        <project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>\n")
	sb.WriteString("    </properties>\n\n")
	sb.WriteString("    <dependencies>\n")

	switch jsonLib {
	case "jackson":
		sb.WriteString("        <dependency>\n")
		sb.WriteString("            <groupId>com.fasterxml.jackson.core</groupId>\n")
		sb.WriteString("            <artifactId>jackson-databind</artifactId>\n")
		sb.WriteString("            <version>2.15.2</version>\n")
		sb.WriteString("        </dependency>\n")
	case "gson":
		sb.WriteString("        <dependency>\n")
		sb.WriteString("            <groupId>com.google.code.gson</groupId>\n")
		sb.WriteString("            <artifactId>gson</artifactId>\n")
		sb.WriteString("            <version>2.10.1</version>\n")
		sb.WriteString("        </dependency>\n")
	}

	sb.WriteString("        <dependency>\n")
	sb.WriteString("            <groupId>junit</groupId>\n")
	sb.WriteString("            <artifactId>junit</artifactId>\n")
	sb.WriteString("            <version>4.13.2</version>\n")
	sb.WriteString("            <scope>test</scope>\n")
	sb.WriteString("        </dependency>\n")
	sb.WriteString("    </dependencies>\n\n")
	sb.WriteString("    <build>\n")
	sb.WriteString("        <plugins>\n")
	sb.WriteString("            <plugin>\n")
	sb.WriteString("                <groupId>org.apache.maven.plugins</groupId>\n")
	sb.WriteString("                <artifactId>maven-compiler-plugin</artifactId>\n")
	sb.WriteString("                <version>3.11.0</version>\n")
	sb.WriteString("                <configuration>\n")
	sb.WriteString("                    <source>11</source>\n")
	sb.WriteString("                    <target>11</target>\n")
	sb.WriteString("                </configuration>\n")
	sb.WriteString("            </plugin>\n")
	sb.WriteString("            <plugin>\n")
	sb.WriteString("                <groupId>org.codehaus.mojo</groupId>\n")
	sb.WriteString("                <artifactId>exec-maven-plugin</artifactId>\n")
	sb.WriteString("                <version>3.1.0</version>\n")
	sb.WriteString("            </plugin>\n")
	sb.WriteString("        </plugins>\n")
	sb.WriteString("    </build>\n")
	sb.WriteString("</project>\n")

	return sb.String()
}

// escapeJavaString escapes a string for use as a Java string literal
// Escapes backslashes, double quotes, and newlines
func escapeJavaString(s string) string {
	var sb strings.Builder
	sb.WriteString("\"") // Start of Java string
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
	sb.WriteString("\"") // End of Java string
	return sb.String()
}

// Keep references to helper functions that are intentionally retained
// to avoid removing them while satisfying the `unused` linter.
var _ = []interface{}{
	writeTypeReference,
	generateNamespaceJava,
	generateEnumTypesJava,
	generateStructClassesJava,
	generateStructClassJava,
	generateClientJava,
	getBoxedJavaType,
	writeTypeDictJava,
	getGetterName,
}

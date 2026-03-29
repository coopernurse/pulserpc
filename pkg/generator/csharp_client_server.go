package generator

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/coopernurse/pulserpc/pkg/parser"
	"github.com/coopernurse/pulserpc/pkg/runtime"
)

// CSharpClientServer is a plugin that generates C# HTTP server and client code from IDL
type CSharpClientServer struct {
}

// NewCSharpClientServer creates a new CSharpClientServer plugin instance
func NewCSharpClientServer() *CSharpClientServer {
	return &CSharpClientServer{}
}

// Name returns the plugin identifier
func (p *CSharpClientServer) Name() string {
	return "csharp-client-server"
}

// RegisterFlags registers CLI flags for this plugin
func (p *CSharpClientServer) RegisterFlags(fs *flag.FlagSet) {
	// No plugin-specific flags
}

// Generate generates C# HTTP server and client code from the parsed IDL
func (p *CSharpClientServer) Generate(idl *parser.IDL, fs *flag.FlagSet) error {
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

	// Copy runtime library files
	if err := p.copyRuntimeFiles(outputDir, isSilent()); err != nil {
		return fmt.Errorf("failed to copy runtime files: %w", err)
	}

	// Group types by namespace
	namespaceMap := GroupTypesByNamespace(idl)

	// Get sorted list of all namespaces
	namespaces := make([]string, 0, len(namespaceMap))
	for ns := range namespaceMap {
		if ns != "" {
			namespaces = append(namespaces, ns)
		}
	}
	sort.Strings(namespaces)

	// Generate Contract.cs (shared interfaces and IdlData)
	contractCode := generateContractCs(idl, structMap, enumMap, namespaceMap)
	contractPath := filepath.Join(outputDir, "Contract.cs")
	if err := os.WriteFile(contractPath, []byte(contractCode), 0644); err != nil {
		return fmt.Errorf("failed to write Contract.cs: %w", err)
	}
	PrintFileCreated(contractPath, fs)

	// Generate one file per namespace
	for namespace, types := range namespaceMap {
		if namespace == "" {
			continue // Skip types without namespace (shouldn't happen with required namespaces)
		}
		namespaceCode := generateNamespaceCs(namespace, namespaces, types, structMap, enumMap)
		namespacePath := filepath.Join(outputDir, snakeToPascalCase(namespace)+".cs")
		if err := os.WriteFile(namespacePath, []byte(namespaceCode), 0644); err != nil {
			return fmt.Errorf("failed to write %s.cs: %w", namespace, err)
		}
		PrintFileCreated(namespacePath, fs)
	}

	// Marshal IDL JSON for embedding in Server.cs
	jsonData, err := json.MarshalIndent(idl, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal IDL to JSON: %w", err)
	}

	// Generate Server.cs
	serverCode := generateServerCs(idl, namespaceMap, structMap, enumMap, string(jsonData))
	serverPath := filepath.Join(outputDir, "Server.cs")
	if err := os.WriteFile(serverPath, []byte(serverCode), 0644); err != nil {
		return fmt.Errorf("failed to write Server.cs: %w", err)
	}
	PrintFileCreated(serverPath, fs)

	// Generate Client.cs
	clientCode := generateClientCs(idl, structMap, enumMap, namespaceMap)
	clientPath := filepath.Join(outputDir, "Client.cs")
	if err := os.WriteFile(clientPath, []byte(clientCode), 0644); err != nil {
		return fmt.Errorf("failed to write Client.cs: %w", err)
	}
	PrintFileCreated(clientPath, fs)

	// Check if generate-test-files flag is set
	generateTestFilesFlag := fs.Lookup("generate-test-files")
	generateTestServer := generateTestFilesFlag != nil && generateTestFilesFlag.Value.String() == "true"
	if generateTestServer {
		// Generate TestServer.cs
		testServerCode := generateTestServerCs(idl, namespaces, structMap, enumMap)
		testServerPath := filepath.Join(outputDir, "TestServer.cs")
		if err := os.WriteFile(testServerPath, []byte(testServerCode), 0644); err != nil {
			return fmt.Errorf("failed to write TestServer.cs: %w", err)
		}
		PrintFileCreated(testServerPath, fs)

		// Generate TestClient.cs
		testClientCode := generateTestClientCs(idl, namespaces, structMap, enumMap)
		testClientPath := filepath.Join(outputDir, "TestClient.cs")
		if err := os.WriteFile(testClientPath, []byte(testClientCode), 0644); err != nil {
			return fmt.Errorf("failed to write TestClient.cs: %w", err)
		}
		PrintFileCreated(testClientPath, fs)

		// Generate TestServer.csproj
		testServerProjCode := generateTestServerCsproj()
		testServerProjPath := filepath.Join(outputDir, "TestServer.csproj")
		if err := os.WriteFile(testServerProjPath, []byte(testServerProjCode), 0644); err != nil {
			return fmt.Errorf("failed to write TestServer.csproj: %w", err)
		}
		PrintFileCreated(testServerProjPath, fs)

		// Generate TestClient.csproj
		testClientProjCode := generateTestClientCsproj()
		testClientProjPath := filepath.Join(outputDir, "TestClient.csproj")
		if err := os.WriteFile(testClientProjPath, []byte(testClientProjCode), 0644); err != nil {
			return fmt.Errorf("failed to write TestClient.csproj: %w", err)
		}
		PrintFileCreated(testClientProjPath, fs)
	}

	return nil
}

// copyRuntimeFiles copies the C# runtime library files to the output directory
// Uses embedded runtime files from the binary
func (p *CSharpClientServer) copyRuntimeFiles(outputDir string, silent bool) error {
	return runtime.CopyRuntimeFiles("csharp", outputDir, silent)
}

// generateNamespaceCs generates a C# file for a single namespace
func generateNamespaceCs(namespace string, allNamespaces []string, types *NamespaceTypes, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString("using System.Collections.Generic;\n")
	sb.WriteString("using System.Text.Json.Serialization;\n")
	sb.WriteString("using PulseRPC;\n")

	for _, ns := range allNamespaces {
		if ns != namespace {
			sb.WriteString(fmt.Sprintf("using %s;\n", ns))
		}
	}
	sb.WriteString("\n")

	sb.WriteString(fmt.Sprintf("namespace %s\n", namespace))
	sb.WriteString("{\n")

	// Generate enum types first (they may be referenced by structs)
	generateEnumTypesCs(&sb, types.Enums, "    ")
	sb.WriteString("\n")

	// Generate struct classes
	generateStructClassesCs(&sb, types.Structs, structMap, enumMap, "    ")
	sb.WriteString("\n")

	// Generate IDL-specific type definitions for this namespace
	sb.WriteString(fmt.Sprintf("    // IDL-specific type definitions for namespace: %s\n", namespace))
	sb.WriteString(fmt.Sprintf("    public static class %sIdl\n", namespace))
	sb.WriteString("    {\n")
	sb.WriteString("        public static readonly Dictionary<string, Dictionary<string, object>> ALL_STRUCTS = new Dictionary<string, Dictionary<string, object>>\n")
	sb.WriteString("        {\n")
	for _, s := range types.Structs {
		sb.WriteString(fmt.Sprintf("            { \"%s\", new Dictionary<string, object>\n", s.Name))
		sb.WriteString("            {\n")
		if s.Extends != "" {
			sb.WriteString(fmt.Sprintf("                { \"extends\", \"%s\" },\n", s.Extends))
		}
		sb.WriteString("                { \"fields\", new List<Dictionary<string, object>>\n")
		sb.WriteString("                {\n")
		for _, field := range s.Fields {
			sb.WriteString("                    new Dictionary<string, object>\n")
			sb.WriteString("                    {\n")
			sb.WriteString(fmt.Sprintf("                        { \"name\", \"%s\" },\n", field.Name))
			sb.WriteString("                        { \"type\", ")
			writeTypeDictCs(&sb, field.Type, structMap, enumMap)
			sb.WriteString(" },\n")
			if field.Optional {
				sb.WriteString("                        { \"optional\", true },\n")
			}
			sb.WriteString("                    },\n")
		}
		sb.WriteString("                }},\n")
		sb.WriteString("            }},\n")
	}
	sb.WriteString("        };\n\n")

	sb.WriteString("        public static readonly Dictionary<string, Dictionary<string, object>> ALL_ENUMS = new Dictionary<string, Dictionary<string, object>>\n")
	sb.WriteString("        {\n")
	for _, e := range types.Enums {
		sb.WriteString(fmt.Sprintf("            { \"%s\", new Dictionary<string, object>\n", e.Name))
		sb.WriteString("            {\n")
		sb.WriteString("                { \"values\", new List<Dictionary<string, object>>\n")
		sb.WriteString("                {\n")
		for _, val := range e.Values {
			sb.WriteString("                    new Dictionary<string, object>\n")
			sb.WriteString("                    {\n")
			sb.WriteString(fmt.Sprintf("                        { \"name\", \"%s\" },\n", val.Name))
			sb.WriteString("                    },\n")
		}
		sb.WriteString("                }},\n")
		sb.WriteString("            }},\n")
	}
	sb.WriteString("        };\n")
	sb.WriteString("    }\n")
	sb.WriteString("}\n")

	return sb.String()
}

// writeTypeDictCs writes a type definition as a C# Dictionary initializer
// structMap and enumMap are used to resolve user-defined types to their fully qualified names
func writeTypeDictCs(sb *strings.Builder, t *parser.Type, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	sb.WriteString("new Dictionary<string, object> { ")
	if t.IsBuiltIn() {
		fmt.Fprintf(sb, "{ \"builtIn\", \"%s\" }", t.BuiltIn)
	} else if t.IsArray() {
		sb.WriteString("{ \"array\", ")
		writeTypeDictCs(sb, t.Array, structMap, enumMap)
		sb.WriteString(" }")
	} else if t.IsMap() {
		sb.WriteString("{ \"mapValue\", ")
		writeTypeDictCs(sb, t.MapValue, structMap, enumMap)
		sb.WriteString(" }")
	} else if t.IsUserDefined() {
		// Use fully qualified name for user-defined types to match ALL_STRUCTS/ALL_ENUMS keys
		typeName := t.UserDefined
		// Check if it's an enum and get its fully qualified name
		if enumDef, ok := enumMap[typeName]; ok {
			typeName = enumDef.Name
		} else if structDef, ok := structMap[typeName]; ok {
			typeName = structDef.Name
		} else {
			// Try to find by unqualified name (handle case where field type is "Status" but enum is "inc.Status")
			for _, enumDef := range enumMap {
				if GetBaseName(enumDef.Name) == typeName || enumDef.Name == typeName {
					typeName = enumDef.Name
					break
				}
			}
			if typeName == t.UserDefined {
				// If still not found in enums, try structs
				for _, structDef := range structMap {
					if GetBaseName(structDef.Name) == typeName || structDef.Name == typeName {
						typeName = structDef.Name
						break
					}
				}
			}
		}
		fmt.Fprintf(sb, "{ \"userDefined\", \"%s\" }", typeName)
	}
	sb.WriteString(" }")
}

// mapTypeToCsType maps an IDL type to a C# type string
// structMap and enumMap are used to resolve user-defined types
func mapTypeToCsType(t *parser.Type, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, optional bool) string {
	if t.IsBuiltIn() {
		csType := ""
		switch t.BuiltIn {
		case "string":
			csType = "string"
		case "int":
			csType = "int"
		case "float":
			csType = "double"
		case "bool":
			csType = "bool"
		default:
			csType = "object"
		}
		if optional {
			if csType == "long" || csType == "bool" || csType == "double" {
				return csType + "?"
			}
			return csType
		}
		return csType
	} else if t.IsArray() {
		elementType := mapTypeToCsType(t.Array, structMap, enumMap, false)
		return "List<" + elementType + ">"
	} else if t.IsMap() {
		valueType := mapTypeToCsType(t.MapValue, structMap, enumMap, false)
		return "Dictionary<string, " + valueType + ">"
	} else if t.IsUserDefined() {
		typeName := getStructOrEnumTypeName(t.UserDefined, structMap, enumMap)
		if optional {
			return typeName + "?"
		}
		return typeName
	}
	return "object"
}

// getStructClassName returns the C# class name for a struct
// Handles qualified names (e.g., "inc.Response" -> "Response")
func getStructClassName(structName string, structMap map[string]*parser.Struct) string {
	baseName := GetBaseName(structName)
	// Check if it exists in structMap (unqualified or qualified)
	if _, ok := structMap[baseName]; ok {
		return baseName
	}
	// If not found with base name, try the qualified name
	if _, ok := structMap[structName]; ok {
		return GetBaseName(structName)
	}
	// Fallback: just return the base name
	return baseName
}

// getEnumTypeName returns the C# enum name for an enum
// Handles qualified names (e.g., "inc.MathOp" -> "MathOp")
func getEnumTypeName(enumName string, enumMap map[string]*parser.Enum) string {
	baseName := GetBaseName(enumName)
	// Check if it exists in enumMap (unqualified or qualified)
	if _, ok := enumMap[baseName]; ok {
		return baseName
	}
	// If not found with base name, try the qualified name
	if _, ok := enumMap[enumName]; ok {
		return GetBaseName(enumName)
	}
	// Fallback: just return the base name
	return baseName
}

// getStructOrEnumTypeName returns the C# type name for a user-defined type
// First checks if it's a struct, then checks if it's an enum
func getStructOrEnumTypeName(typeName string, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) string {
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

// snakeToPascalCase converts snake_case to PascalCase
// Example: "to_repeat" -> "ToRepeat"
func snakeToPascalCase(s string) string {
	parts := strings.Split(s, "_")
	result := ""
	for _, part := range parts {
		if len(part) > 0 {
			result += strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return result
}

// generateEnumTypesCs generates C# enum types for all enums in the namespace
func generateEnumTypesCs(sb *strings.Builder, enums []*parser.Enum, prefix string) {
	for _, e := range enums {
		if e.Comment != "" {
			lines := strings.Split(strings.TrimSpace(e.Comment), "\n")
			for _, line := range lines {
				fmt.Fprintf(sb, "%s// %s\n", prefix, line)
			}
		}
		// Use base name only (remove namespace prefix if present)
		enumName := GetBaseName(e.Name)
		fmt.Fprintf(sb, "%spublic enum %s\n", prefix, enumName)
		sb.WriteString(prefix + "{\n")
		for i, val := range e.Values {
			if i > 0 {
				sb.WriteString(",\n")
			}
			// C# enum values - use the IDL name directly (may be lowercase)
			// System.Text.Json will serialize these as strings if JsonStringEnumConverter is used
			fmt.Fprintf(sb, "%s    %s", prefix, val.Name)
		}
		if len(e.Values) > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(prefix + "}\n\n")
	}
}

// generateStructClassesCs generates C# classes for all structs in the namespace
func generateStructClassesCs(sb *strings.Builder, structs []*parser.Struct, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, prefix string) {
	for _, s := range structs {
		if s.Comment != "" {
			lines := strings.Split(strings.TrimSpace(s.Comment), "\n")
			for _, line := range lines {
				fmt.Fprintf(sb, "%s// %s\n", prefix, line)
			}
		}

		// Use base name only (remove namespace prefix if present)
		structName := GetBaseName(s.Name)
		fmt.Fprintf(sb, "%spublic class %s", prefix, structName)

		// Handle inheritance
		if s.Extends != "" {
			parentName := getStructClassName(s.Extends, structMap)
			fmt.Fprintf(sb, " : %s", parentName)
		}

		sb.WriteString("\n" + prefix + "{\n")

		// Generate default constructor
		fmt.Fprintf(sb, "%s    public %s() { }\n\n", prefix, structName)

		// Generate properties for each field
		for _, field := range s.Fields {
			if field.Comment != "" {
				lines := strings.Split(strings.TrimSpace(field.Comment), "\n")
				for _, line := range lines {
					fmt.Fprintf(sb, "%s    // %s\n", prefix, line)
				}
			}

			// JSON property name attribute (IDL uses snake_case, C# uses PascalCase)
			fmt.Fprintf(sb, "%s    [JsonPropertyName(\"%s\")]\n", prefix, field.Name)

			// Property type
			csType := mapTypeToCsType(field.Type, structMap, enumMap, field.Optional)

			// Property name in PascalCase
			propName := snakeToPascalCase(field.Name)

			// Generate property
			sb.WriteString(prefix + "    public ")
			fmt.Fprintf(sb, "%s %s { get; set; }\n\n", csType, propName)
		}

		sb.WriteString(prefix + "}\n\n")
	}
}

func generateContractCs(idl *parser.IDL, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, namespaceMap map[string]*NamespaceTypes) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString("using System.Collections.Generic;\n")
	sb.WriteString("using PulseRPC;\n\n")

	// Import from namespace files
	namespaces := make([]string, 0, len(namespaceMap))
	for ns := range namespaceMap {
		if ns != "" {
			namespaces = append(namespaces, ns)
		}
	}
	sort.Strings(namespaces)

	for _, ns := range namespaces {
		sb.WriteString(fmt.Sprintf("using %s;\n", ns))
	}
	sb.WriteString("\n")

	sb.WriteString("namespace PulseRPC\n")
	sb.WriteString("{\n")

	// Merge ALL_STRUCTS and ALL_ENUMS from all namespaces
	sb.WriteString("    public static class IdlData\n")
	sb.WriteString("    {\n")
	sb.WriteString("        public static Dictionary<string, Dictionary<string, object>> ALL_STRUCTS = new Dictionary<string, Dictionary<string, object>>();\n")
	sb.WriteString("        public static Dictionary<string, Dictionary<string, object>> ALL_ENUMS = new Dictionary<string, Dictionary<string, object>>();\n")
	sb.WriteString("        \n")
	sb.WriteString("        static IdlData()\n")
	sb.WriteString("        {\n")
	for _, ns := range namespaces {
		sb.WriteString(fmt.Sprintf("            foreach (var kvp in %s.%sIdl.ALL_STRUCTS) ALL_STRUCTS[kvp.Key] = kvp.Value;\n", ns, ns))
		sb.WriteString(fmt.Sprintf("            foreach (var kvp in %s.%sIdl.ALL_ENUMS) ALL_ENUMS[kvp.Key] = kvp.Value;\n", ns, ns))
	}
	sb.WriteString("        }\n")
	sb.WriteString("    }\n\n")

	// Generate interface definitions
	for _, iface := range idl.Interfaces {
		writeInterfaceStubCs(&sb, iface, structMap, enumMap)
	}

	sb.WriteString("}\n")

	return sb.String()
}

// generateServerCs generates the Server.cs file with HTTP server and interface stubs
// This is a large function - implementing step by step
func generateServerCs(idl *parser.IDL, namespaceMap map[string]*NamespaceTypes, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, idlJson string) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString("using System;\n")
	sb.WriteString("using System.Collections.Generic;\n")
	sb.WriteString("using System.Linq;\n")
	sb.WriteString("using System.Net;\n")
	sb.WriteString("using System.Text.Json;\n")
	sb.WriteString("using System.Text.Json.Serialization;\n")
	sb.WriteString("using System.Threading.Tasks;\n")
	sb.WriteString("using Microsoft.AspNetCore.Builder;\n")
	sb.WriteString("using Microsoft.AspNetCore.Http;\n")
	sb.WriteString("using Microsoft.Extensions.Logging;\n")
	sb.WriteString("using Microsoft.Extensions.DependencyInjection;\n")
	sb.WriteString("using PulseRPC;\n\n")

	// Import from namespace files
	namespaces := make([]string, 0, len(namespaceMap))
	for ns := range namespaceMap {
		if ns != "" {
			namespaces = append(namespaces, ns)
		}
	}
	sort.Strings(namespaces)

	for _, ns := range namespaces {
		// Namespace files define static classes like "checkoutIdl" in the "PulseRPC" namespace
		sb.WriteString(fmt.Sprintf("using static %s.%sIdl;\n", ns, ns))
		sb.WriteString(fmt.Sprintf("using %s;\n", ns))
	}
	sb.WriteString("\n")

	sb.WriteString("namespace PulseRPC\n")
	sb.WriteString("{\n")

	// Generate PulseRPCServer class
	writePulseRPCServerCs(&sb, idl, structMap, enumMap, idlJson)

	sb.WriteString("}\n")

	return sb.String()
}

// escapeCSharpVerbatimString escapes a string for use as a C# verbatim string literal
func escapeCSharpVerbatimString(s string) string {
	var sb strings.Builder
	sb.WriteString(`@"`) // Start of C# verbatim string
	for _, r := range s {
		switch r {
		case '"':
			sb.WriteString(`""`) // Escape double quotes in verbatim strings
		case '\r':
			// Skip carriage returns in verbatim strings
		default:
			sb.WriteRune(r)
		}
	}
	sb.WriteString(`"`) // End of C# verbatim string
	return sb.String()
}

// writeInterfaceStubCs generates an interface for an IDL interface
func writeInterfaceStubCs(sb *strings.Builder, iface *parser.Interface, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	if iface.Comment != "" {
		lines := strings.Split(strings.TrimSpace(iface.Comment), "\n")
		for _, line := range lines {
			fmt.Fprintf(sb, "// %s\n", line)
		}
	}
	fmt.Fprintf(sb, "public interface I%s\n", iface.Name)
	sb.WriteString("{\n")

	for _, method := range iface.Methods {
		// Return type
		returnType := "object"
		if method.ReturnType != nil {
			returnType = mapTypeToCsType(method.ReturnType, structMap, enumMap, method.ReturnOptional)
		}
		fmt.Fprintf(sb, "    %s %s(", returnType, method.Name)

		// Parameters
		for i, param := range method.Parameters {
			if i > 0 {
				sb.WriteString(", ")
			}
			paramType := mapTypeToCsType(param.Type, structMap, enumMap, false)
			fmt.Fprintf(sb, "%s %s", paramType, param.Name)
		}
		sb.WriteString(");\n")
	}
	sb.WriteString("}\n\n")
}

// writePulseRPCServerCs generates the PulseRPCServer class
func writePulseRPCServerCs(sb *strings.Builder, idl *parser.IDL, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, idlJson string) {
	sb.WriteString("public class PulseRPCServer\n")
	sb.WriteString("{\n")
	sb.WriteString("    private static readonly string _idlJson = ")
	sb.WriteString(escapeCSharpVerbatimString(idlJson))
	sb.WriteString(";\n\n")
	sb.WriteString("    private Dictionary<string, object> _handlers = new Dictionary<string, object>();\n")
	sb.WriteString("    private WebApplication? _app;\n")
	sb.WriteString("    private ILogger<PulseRPCServer>? _logger;\n\n")

	sb.WriteString("    public PulseRPCServer(ILogger<PulseRPCServer>? logger = null)\n")
	sb.WriteString("    {\n")
	sb.WriteString("        _logger = logger;\n")
	sb.WriteString("    }\n\n")
	sb.WriteString("    public void Register<T>(string interfaceName, T implementation) where T : class\n")
	sb.WriteString("    {\n")
	sb.WriteString("        _handlers[interfaceName] = implementation!;\n")
	sb.WriteString("        _logger?.LogInformation(\"Registered handler for interface: {InterfaceName}\", interfaceName);\n")
	sb.WriteString("    }\n\n")

	// Generate typed Register methods for each interface
	for _, iface := range idl.Interfaces {
		fmt.Fprintf(sb, "    public void Register%s(I%s implementation)\n", iface.Name, iface.Name)
		sb.WriteString("    {\n")
		fmt.Fprintf(sb, "        this.Register(\"%s\", implementation);\n", iface.Name)
		sb.WriteString("    }\n\n")
	}

	sb.WriteString("    public async Task RunAsync(string host = \"localhost\", int port = 8080)\n")
	sb.WriteString("    {\n")
	sb.WriteString("        var builder = WebApplication.CreateBuilder(new WebApplicationOptions\n")
	sb.WriteString("        {\n")
	sb.WriteString("            WebRootPath = null,\n")
	sb.WriteString("            Args = new[] { $\"--urls=http://{host}:{port}\" }\n")
	sb.WriteString("        });\n")
	sb.WriteString("        _app = builder.Build();\n")
	sb.WriteString("        // Get logger from app services if not already set\n")
	sb.WriteString("        if (_logger == null)\n")
	sb.WriteString("        {\n")
	sb.WriteString("            _logger = _app.Services.GetService<ILogger<PulseRPCServer>>();\n")
	sb.WriteString("        }\n\n")
	sb.WriteString("        _app.MapPost(\"/\", async (HttpContext context) =>\n")
	sb.WriteString("        {\n")
	sb.WriteString("            await HandleRequest(context);\n")
	sb.WriteString("        });\n\n")
	sb.WriteString("        Console.WriteLine($\"PulseRPC server listening on http://{host}:{port}\");\n")
	sb.WriteString("        await _app.RunAsync();\n")
	sb.WriteString("    }\n\n")

	sb.WriteString("    private async Task HandleRequest(HttpContext context)\n")
	sb.WriteString("    {\n")
	sb.WriteString("        if (context.Request.Method != \"POST\")\n")
	sb.WriteString("        {\n")
	sb.WriteString("            context.Response.StatusCode = 405;\n")
	sb.WriteString("            await context.Response.WriteAsJsonAsync(new { error = \"Method Not Allowed\" });\n")
	sb.WriteString("            return;\n")
	sb.WriteString("        }\n\n")
	sb.WriteString("        JsonElement requestJson;\n")
	sb.WriteString("        try\n")
	sb.WriteString("        {\n")
	sb.WriteString("            requestJson = await JsonSerializer.DeserializeAsync<JsonElement>(context.Request.Body);\n")
	sb.WriteString("        }\n")
	sb.WriteString("        catch (Exception e)\n")
	sb.WriteString("        {\n")
	sb.WriteString("            await WriteErrorResponse(context, null, -32700, \"Parse error\", $\"Invalid JSON: {e.Message}\");\n")
	sb.WriteString("            return;\n")
	sb.WriteString("        }\n\n")
	sb.WriteString("        object? response;\n")
	sb.WriteString("        if (requestJson.ValueKind == JsonValueKind.Array)\n")
	sb.WriteString("        {\n")
	sb.WriteString("            // Batch request\n")
	sb.WriteString("            var responses = new List<object?>();\n")
	sb.WriteString("            foreach (var req in requestJson.EnumerateArray())\n")
	sb.WriteString("            {\n")
	sb.WriteString("                var reqDict = ConvertJsonElementToDict(req);\n")
	sb.WriteString("                var resp = await HandleSingleRequest(reqDict);\n")
	sb.WriteString("                if (resp != null) responses.Add(resp);\n")
	sb.WriteString("            }\n")
	sb.WriteString("            if (responses.Count == 0)\n")
	sb.WriteString("            {\n")
	sb.WriteString("                context.Response.StatusCode = 204;\n")
	sb.WriteString("            }\n")
	sb.WriteString("            else\n")
	sb.WriteString("            {\n")
	sb.WriteString("                await context.Response.WriteAsJsonAsync(responses);\n")
	sb.WriteString("            }\n")
	sb.WriteString("        }\n")
	sb.WriteString("        else\n")
	sb.WriteString("        {\n")
	sb.WriteString("            var reqDict = ConvertJsonElementToDict(requestJson);\n")
	sb.WriteString("            response = await HandleSingleRequest(reqDict);\n")
	sb.WriteString("            if (response == null)\n")
	sb.WriteString("            {\n")
	sb.WriteString("                context.Response.StatusCode = 204;\n")
	sb.WriteString("            }\n")
	sb.WriteString("            else\n")
	sb.WriteString("            {\n")
	sb.WriteString("                await context.Response.WriteAsJsonAsync(response);\n")
	sb.WriteString("            }\n")
	sb.WriteString("        }\n")
	sb.WriteString("    }\n\n")
	sb.WriteString("    private Dictionary<string, object?> ConvertJsonElementToDict(JsonElement element)\n")
	sb.WriteString("    {\n")
	sb.WriteString("        var dict = new Dictionary<string, object?>();\n")
	sb.WriteString("        foreach (var prop in element.EnumerateObject())\n")
	sb.WriteString("        {\n")
	sb.WriteString("            dict[prop.Name] = ConvertJsonElementValue(prop.Value);\n")
	sb.WriteString("        }\n")
	sb.WriteString("        return dict;\n")
	sb.WriteString("    }\n\n")
	sb.WriteString("    private string? ExtractStringValue(object? value)\n")
	sb.WriteString("    {\n")
	sb.WriteString("        if (value is string str)\n")
	sb.WriteString("            return str;\n")
	sb.WriteString("        if (value is JsonElement jsonElement && jsonElement.ValueKind == JsonValueKind.String)\n")
	sb.WriteString("            return jsonElement.GetString();\n")
	sb.WriteString("        return null;\n")
	sb.WriteString("    }\n\n")
	sb.WriteString("    private object? ConvertJsonElementValue(JsonElement element)\n")
	sb.WriteString("    {\n")
	sb.WriteString("        switch (element.ValueKind)\n")
	sb.WriteString("        {\n")
	sb.WriteString("            case JsonValueKind.String:\n")
	sb.WriteString("                return element.GetString();\n")
	sb.WriteString("            case JsonValueKind.Number:\n")
	sb.WriteString("                if (element.TryGetInt32(out var intVal))\n")
	sb.WriteString("                    return intVal;\n")
	sb.WriteString("                if (element.TryGetInt64(out var longVal))\n")
	sb.WriteString("                    return longVal;\n")
	sb.WriteString("                return element.GetDouble();\n")
	sb.WriteString("            case JsonValueKind.True:\n")
	sb.WriteString("                return true;\n")
	sb.WriteString("            case JsonValueKind.False:\n")
	sb.WriteString("                return false;\n")
	sb.WriteString("            case JsonValueKind.Null:\n")
	sb.WriteString("                return null;\n")
	sb.WriteString("            case JsonValueKind.Array:\n")
	sb.WriteString("                var list = new List<object?>();\n")
	sb.WriteString("                foreach (var item in element.EnumerateArray())\n")
	sb.WriteString("                {\n")
	sb.WriteString("                    list.Add(ConvertJsonElementValue(item));\n")
	sb.WriteString("                }\n")
	sb.WriteString("                return list;\n")
	sb.WriteString("            case JsonValueKind.Object:\n")
	sb.WriteString("                return ConvertJsonElementToDict(element);\n")
	sb.WriteString("            default:\n")
	sb.WriteString("                return element;\n")
	sb.WriteString("        }\n")
	sb.WriteString("    }\n\n")
	sb.WriteString("    private void ConvertEnumIntsToStrings(Dictionary<string, object?> dict, string structName, Dictionary<string, Dictionary<string, object>> allStructs, Dictionary<string, Dictionary<string, object>> allEnums)\n")
	sb.WriteString("    {\n")
	sb.WriteString("        var structDef = Types.FindStruct(structName, allStructs);\n")
	sb.WriteString("        foreach (var kvp in dict.ToList())\n")
	sb.WriteString("        {\n")
	sb.WriteString("            var key = kvp.Key;\n")
	sb.WriteString("            var value = kvp.Value;\n")
	sb.WriteString("            if (value is Dictionary<string, object?> nestedDict)\n")
	sb.WriteString("            {\n")
	sb.WriteString("                // Determine the nested struct name\n")
	sb.WriteString("                string nestedStructName = null;\n")
	sb.WriteString("                if (structDef != null && structDef.TryGetValue(\"fields\", out var fieldsObj) && fieldsObj is System.Collections.IList fields)\n")
	sb.WriteString("                {\n")
	sb.WriteString("                    foreach (Dictionary<string, object> field in fields)\n")
	sb.WriteString("                    {\n")
	sb.WriteString("                        if (field.TryGetValue(\"name\", out var nameObj) && nameObj?.ToString() == key)\n")
	sb.WriteString("                        {\n")
	sb.WriteString("                            if (field.TryGetValue(\"type\", out var typeObj) && typeObj is Dictionary<string, object> typeDict && typeDict.TryGetValue(\"userDefined\", out var userTypeObj) && userTypeObj is string userType)\n")
	sb.WriteString("                            {\n")
	sb.WriteString("                                nestedStructName = Types.FindStruct(userType, allStructs) != null ? userType : null;\n")
	sb.WriteString("                            }\n")
	sb.WriteString("                            break;\n")
	sb.WriteString("                        }\n")
	sb.WriteString("                    }\n")
	sb.WriteString("                }\n")
	sb.WriteString("                if (nestedStructName != null)\n")
	sb.WriteString("                {\n")
	sb.WriteString("                    ConvertEnumIntsToStrings(nestedDict, nestedStructName, allStructs, allEnums);\n")
	sb.WriteString("                }\n")
	sb.WriteString("                else\n")
	sb.WriteString("                {\n")
	sb.WriteString("                    ConvertEnumIntsToStrings(nestedDict, null, allStructs, allEnums);\n")
	sb.WriteString("                }\n")
	sb.WriteString("            }\n")
	sb.WriteString("            else if (value is System.Collections.IList list)\n")
	sb.WriteString("            {\n")
	sb.WriteString("                for (int i = 0; i < list.Count; i++)\n")
	sb.WriteString("                {\n")
	sb.WriteString("                    if (list[i] is Dictionary<string, object?> listDict)\n")
	sb.WriteString("                    {\n")
	sb.WriteString("                        ConvertEnumIntsToStrings(listDict, structName, allStructs, allEnums);\n")
	sb.WriteString("                    }\n")
	sb.WriteString("                }\n")
	sb.WriteString("            }\n")
	sb.WriteString("            else if (value is int intVal && structDef != null)\n")
	sb.WriteString("            {\n")
	sb.WriteString("                // Check if this field is an enum type before converting\n")
	sb.WriteString("                string enumTypeName = null;\n")
	sb.WriteString("                if (structDef.TryGetValue(\"fields\", out var fieldsObj) && fieldsObj is System.Collections.IList fields)\n")
	sb.WriteString("                {\n")
	sb.WriteString("                    foreach (Dictionary<string, object> field in fields)\n")
	sb.WriteString("                    {\n")
	sb.WriteString("                        if (field.TryGetValue(\"name\", out var nameObj) && nameObj?.ToString() == key)\n")
	sb.WriteString("                        {\n")
	sb.WriteString("                            if (field.TryGetValue(\"type\", out var typeObj) && typeObj is Dictionary<string, object> typeDict && typeDict.TryGetValue(\"userDefined\", out var userTypeObj) && userTypeObj is string userType)\n")
	sb.WriteString("                            {\n")
	sb.WriteString("                                if (Types.FindEnum(userType, allEnums) != null)\n")
	sb.WriteString("                                {\n")
	sb.WriteString("                                    enumTypeName = userType;\n")
	sb.WriteString("                                }\n")
	sb.WriteString("                            }\n")
	sb.WriteString("                            break;\n")
	sb.WriteString("                        }\n")
	sb.WriteString("                    }\n")
	sb.WriteString("                }\n")
	sb.WriteString("                if (enumTypeName != null && allEnums.TryGetValue(enumTypeName, out var enumDef))\n")
	sb.WriteString("                {\n")
	sb.WriteString("                    if (enumDef.TryGetValue(\"values\", out var valuesObj) && valuesObj is System.Collections.IList enumValues && intVal >= 0 && intVal < enumValues.Count)\n")
	sb.WriteString("                    {\n")
	sb.WriteString("                        var enumValue = enumValues[intVal];\n")
	sb.WriteString("                        if (enumValue is Dictionary<string, object> enumValueDict && enumValueDict.TryGetValue(\"name\", out var nameObj))\n")
	sb.WriteString("                        {\n")
	sb.WriteString("                            dict[key] = nameObj?.ToString();\n")
	sb.WriteString("                        }\n")
	sb.WriteString("                        else if (enumValue is string enumName)\n")
	sb.WriteString("                        {\n")
	sb.WriteString("                            dict[key] = enumName;\n")
	sb.WriteString("                        }\n")
	sb.WriteString("                    }\n")
	sb.WriteString("                }\n")
	sb.WriteString("            }\n")
	sb.WriteString("        }\n")
	sb.WriteString("    }\n\n")

	// HandleSingleRequest method
	writeHandleSingleRequestCs(sb, idl, structMap, enumMap)

	sb.WriteString("}\n")
}

// writeHandleSingleRequestCs generates the HandleSingleRequest method
func writeHandleSingleRequestCs(sb *strings.Builder, idl *parser.IDL, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	sb.WriteString("    private async Task<Dictionary<string, object?>?> HandleSingleRequest(Dictionary<string, object?> requestJson)\n")
	sb.WriteString("    {\n")
	sb.WriteString("        // Validate JSON-RPC 2.0 structure\n")
	sb.WriteString("        if (!requestJson.TryGetValue(\"jsonrpc\", out var jsonrpcObj))\n")
	sb.WriteString("        {\n")
	sb.WriteString("            _logger?.LogWarning(\"Missing jsonrpc field\");\n")
	sb.WriteString("            return ErrorResponse(null, -32600, \"Invalid Request\", \"jsonrpc field is required\");\n")
	sb.WriteString("        }\n")
	sb.WriteString("        var jsonrpc = ExtractStringValue(jsonrpcObj);\n")
	sb.WriteString("        if (jsonrpc != \"2.0\")\n")
	sb.WriteString("        {\n")
	sb.WriteString("            _logger?.LogWarning(\"Invalid JSON-RPC version: {JsonRpc}\", jsonrpc ?? \"null\");\n")
	sb.WriteString("            return ErrorResponse(null, -32600, \"Invalid Request\", \"jsonrpc must be '2.0'\");\n")
	sb.WriteString("        }\n\n")
	sb.WriteString("        if (!requestJson.TryGetValue(\"method\", out var methodObj))\n")
	sb.WriteString("        {\n")
	sb.WriteString("            _logger?.LogWarning(\"Missing method field\");\n")
	sb.WriteString("            return ErrorResponse(null, -32600, \"Invalid Request\", \"method field is required\");\n")
	sb.WriteString("        }\n")
	sb.WriteString("        var method = ExtractStringValue(methodObj);\n")
	sb.WriteString("        if (method == null)\n")
	sb.WriteString("        {\n")
	sb.WriteString("            _logger?.LogWarning(\"Invalid method in request: {Method}\", methodObj?.ToString() ?? \"null\");\n")
	sb.WriteString("            return ErrorResponse(null, -32600, \"Invalid Request\", \"method must be a string\");\n")
	sb.WriteString("        }\n\n")
	sb.WriteString("        requestJson.TryGetValue(\"params\", out var paramsObj);\n")
	sb.WriteString("        requestJson.TryGetValue(\"id\", out var requestId);\n")
	sb.WriteString("        bool isNotification = !requestJson.ContainsKey(\"id\");\n")
	sb.WriteString("        _logger?.LogInformation(\"Received request: method={Method}, id={RequestId}, isNotification={IsNotification}\", method, requestId, isNotification);\n")
	sb.WriteString("        _logger?.LogDebug(\"Request params: {Params}\", paramsObj != null ? JsonSerializer.Serialize(paramsObj) : \"null\");\n\n")

	sb.WriteString("        // Special case: pulserpc-idl method\n")
	sb.WriteString("        if (method == \"pulserpc-idl\")\n")
	sb.WriteString("        {\n")
	sb.WriteString("            _logger?.LogDebug(\"Handling pulserpc-idl request\");\n")
	sb.WriteString("            try\n")
	sb.WriteString("            {\n")
	sb.WriteString("                var idlDoc = JsonSerializer.Deserialize<object>(_idlJson);\n")
	sb.WriteString("                if (isNotification) return null;\n")
	sb.WriteString("                return new Dictionary<string, object?>\n")
	sb.WriteString("                {\n")
	sb.WriteString("                    { \"jsonrpc\", \"2.0\" },\n")
	sb.WriteString("                    { \"result\", idlDoc },\n")
	sb.WriteString("                    { \"id\", requestId }\n")
	sb.WriteString("                };\n")
	sb.WriteString("            }\n")
	sb.WriteString("            catch (Exception e)\n")
	sb.WriteString("            {\n")
	sb.WriteString("                _logger?.LogError(e, \"Failed to deserialize IDL JSON\");\n")
	sb.WriteString("                return ErrorResponse(requestId, -32603, \"Internal error\", $\"Failed to deserialize IDL JSON: {e.Message}\");\n")
	sb.WriteString("            }\n")
	sb.WriteString("        }\n\n")

	sb.WriteString("        // Parse method name: interface.method\n")
	sb.WriteString("        var parts = method.Split('.', 2);\n")
	sb.WriteString("        if (parts.Length != 2)\n")
	sb.WriteString("        {\n")
	sb.WriteString("            _logger?.LogWarning(\"Invalid method format: {Method}\", method);\n")
	sb.WriteString("            return ErrorResponse(requestId, -32601, \"Method not found\", $\"Invalid method format: {method}\");\n")
	sb.WriteString("        }\n\n")
	sb.WriteString("        var interfaceName = parts[0];\n")
	sb.WriteString("        var methodName = parts[1];\n")
	sb.WriteString("        _logger?.LogDebug(\"Parsed method: interface={InterfaceName}, method={MethodName}\", interfaceName, methodName);\n\n")

	sb.WriteString("        // Find handler\n")
	sb.WriteString("        if (!_handlers.TryGetValue(interfaceName, out var handler))\n")
	sb.WriteString("        {\n")
	sb.WriteString("            _logger?.LogWarning(\"Interface not registered: {InterfaceName}\", interfaceName);\n")
	sb.WriteString("            return ErrorResponse(requestId, -32601, \"Method not found\", $\"Interface '{interfaceName}' not registered\");\n")
	sb.WriteString("        }\n\n")

	// Method lookup and invocation
	writeMethodLookupAndInvokeCs(sb, idl, structMap, enumMap)

	sb.WriteString("    }\n\n")

	sb.WriteString("    private Dictionary<string, object?> ErrorResponse(object? requestId, int code, string message, object? data = null)\n")
	sb.WriteString("    {\n")
	sb.WriteString("        var error = new Dictionary<string, object?> { { \"code\", code }, { \"message\", message } };\n")
	sb.WriteString("        if (data != null) error[\"data\"] = data;\n")
	sb.WriteString("        return new Dictionary<string, object?>\n")
	sb.WriteString("        {\n")
	sb.WriteString("            { \"jsonrpc\", \"2.0\" },\n")
	sb.WriteString("            { \"error\", error },\n")
	sb.WriteString("            { \"id\", requestId }\n")
	sb.WriteString("        };\n")
	sb.WriteString("    }\n\n")

	sb.WriteString("    private async Task WriteErrorResponse(HttpContext context, object? requestId, int code, string message, object? data = null)\n")
	sb.WriteString("    {\n")
	sb.WriteString("        await context.Response.WriteAsJsonAsync(ErrorResponse(requestId, code, message, data));\n")
	sb.WriteString("    }\n")
}

// writeDeserializeParamCs writes C# code to deserialize a parameter value to its typed object
// writeMethodLookupAndInvokeCs generates method lookup and invocation code
func writeMethodLookupAndInvokeCs(sb *strings.Builder, idl *parser.IDL, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	sb.WriteString("        // Find method definition\n")
	sb.WriteString("        Dictionary<string, object>? methodDef = null;\n\n")

	for i, iface := range idl.Interfaces {
		if i == 0 {
			fmt.Fprintf(sb, "        if (interfaceName == \"%s\")\n", iface.Name)
		} else {
			fmt.Fprintf(sb, "        else if (interfaceName == \"%s\")\n", iface.Name)
		}
		sb.WriteString("        {\n")
		sb.WriteString("            var interfaceMethods = new Dictionary<string, Dictionary<string, object>>\n")
		sb.WriteString("            {\n")
		for _, method := range iface.Methods {
			fmt.Fprintf(sb, "                { \"%s\", new Dictionary<string, object>\n", method.Name)
			sb.WriteString("                {\n")
			sb.WriteString("                    { \"parameters\", new List<Dictionary<string, object>>\n")
			sb.WriteString("                    {\n")
			for _, param := range method.Parameters {
				sb.WriteString("                        new Dictionary<string, object>\n")
				sb.WriteString("                        {\n")
				fmt.Fprintf(sb, "                            { \"name\", \"%s\" },\n", param.Name)
				sb.WriteString("                            { \"type\", ")
				writeTypeDictCs(sb, param.Type, structMap, enumMap)
				sb.WriteString(" },\n")
				sb.WriteString("                        },\n")
			}
			sb.WriteString("                    }},\n")
			sb.WriteString("                    { \"returnType\", ")
			writeTypeDictCs(sb, method.ReturnType, structMap, enumMap)
			sb.WriteString(" },\n")
			sb.WriteString("                    { \"returnOptional\", ")
			if method.ReturnOptional {
				sb.WriteString("true")
			} else {
				sb.WriteString("false")
			}
			sb.WriteString(" },\n")
			sb.WriteString("                }},\n")
		}
		sb.WriteString("            };\n")
		sb.WriteString("            methodDef = interfaceMethods.TryGetValue(methodName, out var def) ? def : null;\n")
		sb.WriteString("        }\n")
	}
	sb.WriteString("\n")

	sb.WriteString("        if (methodDef == null)\n")
	sb.WriteString("        {\n")
	sb.WriteString("            _logger?.LogWarning(\"Method not found: {InterfaceName}.{MethodName}\", interfaceName, methodName);\n")
	sb.WriteString("            return ErrorResponse(requestId, -32601, \"Method not found\", $\"Method '{methodName}' not found in interface '{interfaceName}'\");\n")
	sb.WriteString("        }\n\n")

	sb.WriteString("        // Validate params\n")
	sb.WriteString("        var paramsList = paramsObj as System.Collections.IList ?? new List<object>();\n")
	sb.WriteString("        var expectedParams = (methodDef[\"parameters\"] as System.Collections.IList) ?? new List<object>();\n")
	sb.WriteString("        _logger?.LogDebug(\"Validating params: expected={ExpectedCount}, got={ActualCount}\", expectedParams.Count, paramsList.Count);\n")
	sb.WriteString("        if (paramsList.Count != expectedParams.Count)\n")
	sb.WriteString("        {\n")
	sb.WriteString("            _logger?.LogWarning(\"Parameter count mismatch: expected={ExpectedCount}, got={ActualCount}\", expectedParams.Count, paramsList.Count);\n")
	sb.WriteString("            return ErrorResponse(requestId, -32602, \"Invalid params\", $\"Expected {expectedParams.Count} parameters, got {paramsList.Count}\");\n")
	sb.WriteString("        }\n\n")

	sb.WriteString("        // Validate each param\n")
	sb.WriteString("        for (int i = 0; i < paramsList.Count; i++)\n")
	sb.WriteString("        {\n")
	sb.WriteString("            var paramValue = paramsList[i];\n")
	sb.WriteString("            var paramDef = (expectedParams[i] as Dictionary<string, object>)!;\n")
	sb.WriteString("            var paramName = paramDef.TryGetValue(\"name\", out var name) ? name?.ToString() : $\"parameter {i}\";\n")
	sb.WriteString("            _logger?.LogDebug(\"Validating parameter {Index} ({ParamName})\", i, paramName);\n")
	sb.WriteString("            try\n")
	sb.WriteString("            {\n")
	sb.WriteString("                var typeDef = (Dictionary<string, object>)paramDef[\"type\"];\n")
	sb.WriteString("                // Convert enum objects/values to strings for validation\n")
	sb.WriteString("                object? valueToValidate = paramValue;\n")
	sb.WriteString("                if (typeDef.TryGetValue(\"userDefined\", out var userTypeObj) && userTypeObj is string userType)\n")
	sb.WriteString("                {\n")
	sb.WriteString("                    var enumDef = Types.FindEnum(userType, IdlData.ALL_ENUMS);\n")
	sb.WriteString("                    if (enumDef != null && paramValue != null)\n")
	sb.WriteString("                    {\n")
	sb.WriteString("                        if (paramValue is System.Text.Json.JsonElement jsonElem)\n")
	sb.WriteString("                        {\n")
	sb.WriteString("                            // Handle JsonElement enum values (could be string or number)\n")
	sb.WriteString("                            if (jsonElem.ValueKind == System.Text.Json.JsonValueKind.String)\n")
	sb.WriteString("                            {\n")
	sb.WriteString("                                valueToValidate = jsonElem.GetString();\n")
	sb.WriteString("                            }\n")
	sb.WriteString("                            else if (jsonElem.ValueKind == System.Text.Json.JsonValueKind.Number && jsonElem.TryGetInt32(out var enumInt))\n")
	sb.WriteString("                            {\n")
	sb.WriteString("                                // Convert integer enum value to string by looking up in enum definition\n")
	sb.WriteString("                                if (enumDef.TryGetValue(\"values\", out var valuesObj) && valuesObj is System.Collections.IList enumValues && enumInt >= 0 && enumInt < enumValues.Count)\n")
	sb.WriteString("                                {\n")
	sb.WriteString("                                    var enumValue = enumValues[enumInt];\n")
	sb.WriteString("                                    if (enumValue is Dictionary<string, object> enumValueDict && enumValueDict.TryGetValue(\"name\", out var nameObj))\n")
	sb.WriteString("                                    {\n")
	sb.WriteString("                                        valueToValidate = nameObj?.ToString();\n")
	sb.WriteString("                                    }\n")
	sb.WriteString("                                    else if (enumValue is string enumName)\n")
	sb.WriteString("                                    {\n")
	sb.WriteString("                                        valueToValidate = enumName;\n")
	sb.WriteString("                                    }\n")
	sb.WriteString("                                    else\n")
	sb.WriteString("                                    {\n")
	sb.WriteString("                                        // Fallback: use the integer as string\n")
	sb.WriteString("                                        valueToValidate = enumInt.ToString();\n")
	sb.WriteString("                                    }\n")
	sb.WriteString("                                }\n")
	sb.WriteString("                                else\n")
	sb.WriteString("                                {\n")
	sb.WriteString("                                    // Enum definition structure doesn't match expected format, use integer as string\n")
	sb.WriteString("                                    valueToValidate = enumInt.ToString();\n")
	sb.WriteString("                                }\n")
	sb.WriteString("                            }\n")
	sb.WriteString("                        }\n")
	sb.WriteString("                        else if (paramValue is int enumIntVal)\n")
	sb.WriteString("                        {\n")
	sb.WriteString("                            // Convert integer enum value to string by looking up in enum definition\n")
	sb.WriteString("                            if (enumDef.TryGetValue(\"values\", out var valuesObj) && valuesObj is System.Collections.IList enumValues && enumIntVal >= 0 && enumIntVal < enumValues.Count)\n")
	sb.WriteString("                            {\n")
	sb.WriteString("                                var enumValue = enumValues[enumIntVal];\n")
	sb.WriteString("                                if (enumValue is Dictionary<string, object> enumValueDict && enumValueDict.TryGetValue(\"name\", out var nameObj))\n")
	sb.WriteString("                                {\n")
	sb.WriteString("                                    valueToValidate = nameObj?.ToString();\n")
	sb.WriteString("                                }\n")
	sb.WriteString("                                else if (enumValue is string enumName)\n")
	sb.WriteString("                                {\n")
	sb.WriteString("                                    valueToValidate = enumName;\n")
	sb.WriteString("                                }\n")
	sb.WriteString("                                else\n")
	sb.WriteString("                                {\n")
	sb.WriteString("                                    // Fallback: use the integer as string\n")
	sb.WriteString("                                    valueToValidate = enumIntVal.ToString();\n")
	sb.WriteString("                                }\n")
	sb.WriteString("                            }\n")
	sb.WriteString("                            else\n")
	sb.WriteString("                            {\n")
	sb.WriteString("                                // Enum definition structure doesn't match expected format, use integer as string\n")
	sb.WriteString("                                valueToValidate = enumIntVal.ToString();\n")
	sb.WriteString("                            }\n")
	sb.WriteString("                        }\n")
	sb.WriteString("                        else if (!(paramValue is string))\n")
	sb.WriteString("                        {\n")
	sb.WriteString("                            // Convert enum object to string representation\n")
	sb.WriteString("                            valueToValidate = paramValue.ToString();\n")
	sb.WriteString("                        }\n")
	sb.WriteString("                    }\n")
	sb.WriteString("                }\n")
	sb.WriteString("                Validation.ValidateType(valueToValidate, typeDef, IdlData.ALL_STRUCTS, IdlData.ALL_ENUMS, false);\n")
	sb.WriteString("            }\n")
	sb.WriteString("            catch (Exception e)\n")
	sb.WriteString("            {\n")
	sb.WriteString("                _logger?.LogError(e, \"Parameter validation failed: parameter {Index} ({ParamName})\", i, paramName);\n")
	sb.WriteString("                return ErrorResponse(requestId, -32602, \"Invalid params\", $\"Parameter {i} ({paramName}) validation failed: {e.Message}\");\n")
	sb.WriteString("            }\n")
	sb.WriteString("        }\n\n")

	sb.WriteString("        // Invoke handler using reflection\n")
	sb.WriteString("        var jsonOptions = new JsonSerializerOptions\n")
	sb.WriteString("        {\n")
	sb.WriteString("            PropertyNameCaseInsensitive = true\n")
	sb.WriteString("        };\n")
	sb.WriteString("        jsonOptions.Converters.Add(new JsonStringEnumConverter());\n")
	sb.WriteString("        object? result;\n")
	sb.WriteString("        try\n")
	sb.WriteString("        {\n")
	sb.WriteString("            _logger?.LogDebug(\"Invoking method {InterfaceName}.{MethodName}\", interfaceName, methodName);\n")
	sb.WriteString("            var handlerType = handler.GetType();\n")
	sb.WriteString("            var methodInfo = handlerType.GetMethod(methodName);\n")
	sb.WriteString("            if (methodInfo == null)\n")
	sb.WriteString("            {\n")
	sb.WriteString("                _logger?.LogError(\"Method not found via reflection: {InterfaceName}.{MethodName}\", interfaceName, methodName);\n")
	sb.WriteString("                return ErrorResponse(requestId, -32601, \"Method not found\", $\"Method '{methodName}' not found on interface '{interfaceName}'\");\n")
	sb.WriteString("            }\n")
	sb.WriteString("            // Deserialize parameters to expected types using method parameter types\n")
	sb.WriteString("            var paramInfos = methodInfo.GetParameters();\n")
	sb.WriteString("            var deserializedParams = new object[paramsList.Count];\n")
	sb.WriteString("            for (int i = 0; i < paramsList.Count; i++)\n")
	sb.WriteString("            {\n")
	sb.WriteString("                var paramValue = paramsList[i];\n")
	sb.WriteString("                var paramType = paramInfos[i].ParameterType;\n")
	sb.WriteString("                _logger?.LogDebug(\"Deserializing parameter {Index} to type {ParamType}\", i, paramType.Name);\n")
	sb.WriteString("                string paramJson;\n")
	sb.WriteString("                if (paramValue is System.Text.Json.JsonElement jsonElement)\n")
	sb.WriteString("                {\n")
	sb.WriteString("                    paramJson = jsonElement.GetRawText();\n")
	sb.WriteString("                }\n")
	sb.WriteString("                else\n")
	sb.WriteString("                {\n")
	sb.WriteString("                    paramJson = JsonSerializer.Serialize(paramValue);\n")
	sb.WriteString("                }\n")
	sb.WriteString("                deserializedParams[i] = JsonSerializer.Deserialize(paramJson, paramType, jsonOptions);\n")
	sb.WriteString("            }\n")
	sb.WriteString("            _logger?.LogDebug(\"Calling method {InterfaceName}.{MethodName} with {ParamCount} parameters\", interfaceName, methodName, deserializedParams.Length);\n")
	sb.WriteString("            result = methodInfo.Invoke(handler, deserializedParams);\n")
	sb.WriteString("            if (result is Task task)\n")
	sb.WriteString("            {\n")
	sb.WriteString("                await task;\n")
	sb.WriteString("                var resultProperty = task.GetType().GetProperty(\"Result\");\n")
	sb.WriteString("                result = resultProperty?.GetValue(task);\n")
	sb.WriteString("            }\n")
	sb.WriteString("            _logger?.LogDebug(\"Method {InterfaceName}.{MethodName} completed successfully\", interfaceName, methodName);\n")
	sb.WriteString("        }\n")
	sb.WriteString("        catch (RPCError rpcErr)\n")
	sb.WriteString("        {\n")
	sb.WriteString("            _logger?.LogWarning(\"RPCError from {InterfaceName}.{MethodName}: {Code} - {Message}\", interfaceName, methodName, rpcErr.Code, rpcErr.Message);\n")
	sb.WriteString("            return ErrorResponse(requestId, rpcErr.Code, rpcErr.Message, rpcErr.Data);\n")
	sb.WriteString("        }\n")
	sb.WriteString("        catch (Exception e)\n")
	sb.WriteString("        {\n")
	sb.WriteString("            _logger?.LogError(e, \"Exception invoking {InterfaceName}.{MethodName}: {Message}\", interfaceName, methodName, e.Message);\n")
	sb.WriteString("            return ErrorResponse(requestId, -32603, \"Internal error\", $\"Exception: {e.Message}\\nStackTrace: {e.StackTrace}\");\n")
	sb.WriteString("        }\n\n")

	sb.WriteString("        // Validate response\n")
	sb.WriteString("        if (methodDef.TryGetValue(\"returnType\", out var returnTypeObj) && returnTypeObj is Dictionary<string, object> returnType)\n")
	sb.WriteString("        {\n")
	sb.WriteString("            _logger?.LogDebug(\"Validating response for {InterfaceName}.{MethodName}\", interfaceName, methodName);\n")
	sb.WriteString("            try\n")
	sb.WriteString("            {\n")
	sb.WriteString("                var returnOptional = methodDef.TryGetValue(\"returnOptional\", out var opt) && opt is bool optBool && optBool;\n")
	sb.WriteString("                // Convert struct objects to dictionaries and enum objects to strings for validation\n")
	sb.WriteString("                object? valueToValidate = result;\n")
	sb.WriteString("                if (returnType.TryGetValue(\"userDefined\", out var returnUserTypeObj) && returnUserTypeObj is string returnUserType)\n")
	sb.WriteString("                {\n")
	sb.WriteString("                    var structDef = Types.FindStruct(returnUserType, IdlData.ALL_STRUCTS);\n")
	sb.WriteString("                    if (structDef != null && result != null && !(result is Dictionary<string, object?>))\n")
	sb.WriteString("                    {\n")
	sb.WriteString("                        // Serialize struct object to JSON, then convert JsonElement to dictionary with proper type conversion\n")
	sb.WriteString("                        var structResultJson = JsonSerializer.Serialize(result, jsonOptions);\n")
	sb.WriteString("                        var structJsonElement = JsonSerializer.Deserialize<JsonElement>(structResultJson);\n")
	sb.WriteString("                        valueToValidate = ConvertJsonElementToDict(structJsonElement);\n")
	sb.WriteString("                        // Convert enum integers to strings for validation\n")
	sb.WriteString("                        if (valueToValidate is Dictionary<string, object?> structDict)\n")
	sb.WriteString("                        {\n")
	sb.WriteString("                            ConvertEnumIntsToStrings(structDict, returnUserType, IdlData.ALL_STRUCTS, IdlData.ALL_ENUMS);\n")
	sb.WriteString("                        }\n")
	sb.WriteString("                    }\n")
	sb.WriteString("                    else\n")
	sb.WriteString("                    {\n")
	sb.WriteString("                        var enumDef = Types.FindEnum(returnUserType, IdlData.ALL_ENUMS);\n")
	sb.WriteString("                        if (enumDef != null && result != null && !(result is string) && !(result is System.Text.Json.JsonElement))\n")
	sb.WriteString("                        {\n")
	sb.WriteString("                            // Convert enum object to string representation\n")
	sb.WriteString("                            valueToValidate = result.ToString();\n")
	sb.WriteString("                        }\n")
	sb.WriteString("                    }\n")
	sb.WriteString("                }\n")
	sb.WriteString("                // Handle arrays of structs or enums - convert elements to dictionaries/strings for validation\n")
	sb.WriteString("                else if (returnType.TryGetValue(\"array\", out var arrayObj) && arrayObj is Dictionary<string, object> elementType)\n")
	sb.WriteString("                {\n")
	sb.WriteString("                    if (result != null && elementType.TryGetValue(\"userDefined\", out var elementUserTypeObj) && elementUserTypeObj is string elementUserType)\n")
	sb.WriteString("                    {\n")
	sb.WriteString("                        var structDef = Types.FindStruct(elementUserType, IdlData.ALL_STRUCTS);\n")
	sb.WriteString("                        if (structDef != null && result is System.Collections.IList resultEnum)\n")
	sb.WriteString("                        {\n")
	sb.WriteString("                            // Convert each struct object in the array to a dictionary for validation\n")
	sb.WriteString("                            var convertedList = new List<object?>();\n")
	sb.WriteString("                            foreach (var item in resultEnum)\n")
	sb.WriteString("                            {\n")
	sb.WriteString("                                if (item != null && !(item is Dictionary<string, object?>))\n")
	sb.WriteString("                                {\n")
	sb.WriteString("                                    var itemJson = JsonSerializer.Serialize(item, jsonOptions);\n")
	sb.WriteString("                                    var itemJsonElement = JsonSerializer.Deserialize<JsonElement>(itemJson);\n")
	sb.WriteString("                                    var itemDict = ConvertJsonElementToDict(itemJsonElement);\n")
	sb.WriteString("                                    if (itemDict is Dictionary<string, object?> dict)\n")
	sb.WriteString("                                    {\n")
	sb.WriteString("                                        ConvertEnumIntsToStrings(dict, elementUserType, IdlData.ALL_STRUCTS, IdlData.ALL_ENUMS);\n")
	sb.WriteString("                                    }\n")
	sb.WriteString("                                    convertedList.Add(itemDict);\n")
	sb.WriteString("                                }\n")
	sb.WriteString("                                else\n")
	sb.WriteString("                                {\n")
	sb.WriteString("                                    convertedList.Add(item);\n")
	sb.WriteString("                                }\n")
	sb.WriteString("                            }\n")
	sb.WriteString("                            valueToValidate = convertedList;\n")
	sb.WriteString("                        }\n")
	sb.WriteString("                        else\n")
	sb.WriteString("                        {\n")
	sb.WriteString("                            var enumDef = Types.FindEnum(elementUserType, IdlData.ALL_ENUMS);\n")
	sb.WriteString("                            if (enumDef != null && result is System.Collections.IList enumList)\n")
	sb.WriteString("                            {\n")
	sb.WriteString("                                // Convert each enum object in the array to a string for validation\n")
	sb.WriteString("                                var convertedEnumList = new List<object?>();\n")
	sb.WriteString("                                foreach (var item in enumList)\n")
	sb.WriteString("                                {\n")
	sb.WriteString("                                    if (item != null && !(item is string) && !(item is System.Text.Json.JsonElement))\n")
	sb.WriteString("                                    {\n")
	sb.WriteString("                                        convertedEnumList.Add(item.ToString());\n")
	sb.WriteString("                                    }\n")
	sb.WriteString("                                    else\n")
	sb.WriteString("                                    {\n")
	sb.WriteString("                                        convertedEnumList.Add(item);\n")
	sb.WriteString("                                    }\n")
	sb.WriteString("                                }\n")
	sb.WriteString("                                valueToValidate = convertedEnumList;\n")
	sb.WriteString("                            }\n")
	sb.WriteString("                        }\n")
	sb.WriteString("                    }\n")
	sb.WriteString("                }\n")
	sb.WriteString("                Validation.ValidateType(valueToValidate, returnType, IdlData.ALL_STRUCTS, IdlData.ALL_ENUMS, returnOptional);\n")
	sb.WriteString("                _logger?.LogDebug(\"Response validation passed for {InterfaceName}.{MethodName}\", interfaceName, methodName);\n")
	sb.WriteString("            }\n")
	sb.WriteString("            catch (Exception e)\n")
	sb.WriteString("            {\n")
	sb.WriteString("                _logger?.LogError(e, \"Response validation failed for {InterfaceName}.{MethodName}\", interfaceName, methodName);\n")
	sb.WriteString("                return ErrorResponse(requestId, -32603, \"Internal error\", $\"Response validation failed: {e.Message}\");\n")
	sb.WriteString("            }\n")
	sb.WriteString("        }\n\n")

	sb.WriteString("        // Return success response\n")
	sb.WriteString("        if (isNotification) return null;\n")
	sb.WriteString("        // Serialize result to JSON for proper response\n")
	sb.WriteString("        var resultJson = JsonSerializer.Serialize(result, jsonOptions);\n")
	sb.WriteString("        return new Dictionary<string, object?>\n")
	sb.WriteString("        {\n")
	sb.WriteString("            { \"jsonrpc\", \"2.0\" },\n")
	sb.WriteString("            { \"result\", JsonSerializer.Deserialize<object>(resultJson, jsonOptions) },\n")
	sb.WriteString("            { \"id\", requestId }\n")
	sb.WriteString("        };\n")
}

// writeParameterDeserializationCs writes C# code to determine the Type for parameter deserialization
// GetCSharpType converts a type definition to a C# Type object for deserialization
func GetCSharpType(typeDef map[string]interface{}, allStructs map[string]*parser.Struct, allEnums map[string]*parser.Enum) string {
	if builtIn, ok := typeDef["builtIn"].(string); ok {
		switch builtIn {
		case "string":
			return "typeof(string)"
		case "int":
			return "typeof(int)"
		case "float":
			return "typeof(double)"
		case "bool":
			return "typeof(bool)"
		default:
			return "typeof(object)"
		}
	} else if _, ok := typeDef["array"]; ok {
		return "typeof(object[])" // Will be handled as List<T> in actual code
	} else if _, ok := typeDef["mapValue"]; ok {
		return "typeof(Dictionary<string, object>)"
	} else if userDefined, ok := typeDef["userDefined"].(string); ok {
		// For user-defined types, return the type name directly
		return userDefined
	}
	return "typeof(object)"
}

// generateClientCs generates the Client.cs file with transport abstraction and client classes
// Note: ITransport and HttpTransport are provided by the runtime Transport.cs, not generated here
func generateClientCs(idl *parser.IDL, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, namespaceMap map[string]*NamespaceTypes) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString("using System;\n")
	sb.WriteString("using System.Collections.Generic;\n")
	sb.WriteString("using System.Linq;\n")
	sb.WriteString("using System.Net.Http;\n")
	sb.WriteString("using System.Text.Json;\n")
	sb.WriteString("using System.Text.Json.Serialization;\n")
	sb.WriteString("using System.Threading.Tasks;\n")
	sb.WriteString("using PulseRPC;\n\n")

	// Import from namespace files
	namespaces := make([]string, 0, len(namespaceMap))
	for ns := range namespaceMap {
		if ns != "" {
			namespaces = append(namespaces, ns)
		}
	}
	sort.Strings(namespaces)

	for _, ns := range namespaces {
		// Namespace files define static classes like "checkoutIdl" in the "PulseRPC" namespace
		sb.WriteString(fmt.Sprintf("using static %s.%sIdl;\n", ns, ns))
		sb.WriteString(fmt.Sprintf("using %s;\n", ns))
	}
	sb.WriteString("\n")

	sb.WriteString("namespace PulseRPC\n")
	sb.WriteString("{\n")

	// Generate client classes for each interface
	for _, iface := range idl.Interfaces {
		writeInterfaceClientCs(&sb, iface, structMap, enumMap)
	}

	sb.WriteString("}\n")

	return sb.String()
}


// writeInterfaceClientCs generates a client class for an interface that implements the interface
func writeInterfaceClientCs(sb *strings.Builder, iface *parser.Interface, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	clientClassName := iface.Name + "Client"
	fmt.Fprintf(sb, "public class %s : I%s\n", clientClassName, iface.Name)
	sb.WriteString("{\n")
	sb.WriteString("    private readonly ITransport _transport;\n\n")
	sb.WriteString("    public " + clientClassName + "(ITransport transport)\n")
	sb.WriteString("    {\n")
	sb.WriteString("        _transport = transport;\n")
	sb.WriteString("    }\n\n")

	// Generate methods for each interface method
	for _, method := range iface.Methods {
		writeClientMethodImplCs(sb, iface, method, structMap, enumMap)
		sb.WriteString("\n")
	}

	sb.WriteString("}\n\n")
}

// writeClientMethodImplCs generates a synchronous method implementation for a client class
func writeClientMethodImplCs(sb *strings.Builder, iface *parser.Interface, method *parser.Method, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	// Return type
	var returnTypeStr string
	if method.ReturnType != nil {
		returnTypeStr = mapTypeToCsType(method.ReturnType, structMap, enumMap, method.ReturnOptional)
	} else {
		returnTypeStr = "object?"
	}

	// Generate synchronous method that implements the interface
	fmt.Fprintf(sb, "    public %s %s(", returnTypeStr, method.Name)

	// Parameters
	for i, param := range method.Parameters {
		if i > 0 {
			sb.WriteString(", ")
		}
		paramType := mapTypeToCsType(param.Type, structMap, enumMap, false)
		sb.WriteString(paramType)
		sb.WriteString(" ")
		fmt.Fprintf(sb, "%s", param.Name)
	}
	sb.WriteString(")\n")
	sb.WriteString("    {\n")
	sb.WriteString("        var task = ")
	fmt.Fprintf(sb, "%sAsync(", method.Name)
	for i, param := range method.Parameters {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(sb, "%s", param.Name)
	}
	sb.WriteString(");\n")
	sb.WriteString("        return task.GetAwaiter().GetResult();\n")
	sb.WriteString("    }\n")

	// Generate async version as well for convenience
	sb.WriteString("\n")
	fmt.Fprintf(sb, "    public async Task<%s> %sAsync(", returnTypeStr, method.Name)

	// Parameters for async
	for i, param := range method.Parameters {
		if i > 0 {
			sb.WriteString(", ")
		}
		paramType := mapTypeToCsType(param.Type, structMap, enumMap, false)
		sb.WriteString(paramType)
		sb.WriteString(" ")
		fmt.Fprintf(sb, "%s", param.Name)
	}
	sb.WriteString(")\n")
	sb.WriteString("    {\n")

	// Create parameters array for transport
	fmt.Fprintf(sb, "        var method = \"%s.%s\";\n", iface.Name, method.Name)
	sb.WriteString("        var parameters = new object[] { ")
	for i, param := range method.Parameters {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(sb, "%s", param.Name)
	}
	sb.WriteString(" };\n\n")

	sb.WriteString("        var response = await _transport.CallAsync(method, parameters);\n")
	sb.WriteString("        if (!response.TryGetValue(\"result\", out var result)) {\n")
	if method.ReturnOptional {
		sb.WriteString("            return default;\n")
	} else {
		sb.WriteString("            throw new RPCError(-32603, \"Internal error\", \"Missing result in response\");\n")
	}
	sb.WriteString("        }\n\n")

	// Deserialize response to typed object
	if method.ReturnType != nil {
		sb.WriteString("        // Deserialize to return type\n")
		sb.WriteString("        string resultJsonStr;\n")
		sb.WriteString("        if (result is System.Text.Json.JsonElement jsonElement)\n")
		sb.WriteString("        {\n")
		sb.WriteString("            resultJsonStr = jsonElement.GetRawText();\n")
		sb.WriteString("        }\n")
		sb.WriteString("        else\n")
		sb.WriteString("        {\n")
		sb.WriteString("            resultJsonStr = JsonSerializer.Serialize(result);\n")
		sb.WriteString("        }\n")

		returnTypeStr = mapTypeToCsType(method.ReturnType, structMap, enumMap, method.ReturnOptional)
		sb.WriteString("        var clientJsonOptions = new JsonSerializerOptions\n")
		sb.WriteString("        {\n")
		sb.WriteString("            PropertyNameCaseInsensitive = true\n")
		sb.WriteString("        };\n")
		sb.WriteString("        clientJsonOptions.Converters.Add(new JsonStringEnumConverter());\n")
		sb.WriteString("        return JsonSerializer.Deserialize<")
		fmt.Fprintf(sb, "%s", returnTypeStr)
		sb.WriteString(">(resultJsonStr, clientJsonOptions);\n")
	} else {
		sb.WriteString("        return result;\n")
	}
	sb.WriteString("    }\n")
}

// generateTestServerCs generates TestServer.cs with concrete implementations of all interfaces
func generateTestServerCs(idl *parser.IDL, allNamespaces []string, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n")
	sb.WriteString("// Test server implementation for integration testing\n\n")
	sb.WriteString("using System;\n")
	sb.WriteString("using System.Collections.Generic;\n")
	sb.WriteString("using System.Linq;\n")
	sb.WriteString("using System.Threading.Tasks;\n")
	sb.WriteString("using Microsoft.Extensions.Logging;\n")
	sb.WriteString("using Microsoft.Extensions.DependencyInjection;\n")
	sb.WriteString("using PulseRPC;\n")

	for _, ns := range allNamespaces {
		sb.WriteString(fmt.Sprintf("using %s;\n", ns))
	}
	sb.WriteString("\n")

	// Generate implementation classes for each interface
	for _, iface := range idl.Interfaces {
		writeTestInterfaceImplCs(&sb, iface, structMap, enumMap)
	}

	// Generate main entry point
	sb.WriteString("public class Program\n")
	sb.WriteString("{\n")
	sb.WriteString("    public static async Task Main(string[] args)\n")
	sb.WriteString("    {\n")
	sb.WriteString("        // Configure logging\n")
	sb.WriteString("        using var loggerFactory = LoggerFactory.Create(builder =>\n")
	sb.WriteString("        {\n")
	sb.WriteString("            builder.AddConsole().SetMinimumLevel(LogLevel.Debug);\n")
	sb.WriteString("        });\n")
	sb.WriteString("        var logger = loggerFactory.CreateLogger<PulseRPCServer>();\n\n")
	sb.WriteString("        var server = new PulseRPCServer(logger);\n")
	for _, iface := range idl.Interfaces {
		implName := iface.Name + "Impl"
		fmt.Fprintf(&sb, "        server.Register%s(new %s());\n", iface.Name, implName)
	}
	sb.WriteString("        await server.RunAsync(\"0.0.0.0\", 8080);\n")
	sb.WriteString("    }\n")
	sb.WriteString("}\n")

	return sb.String()
}

// generateTestClientCs generates TestClient.cs test program
func generateTestClientCs(idl *parser.IDL, allNamespaces []string, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n")
	sb.WriteString("// Test client program for integration testing\n\n")
	sb.WriteString("using System;\n")
	sb.WriteString("using System.Collections.Generic;\n")
	sb.WriteString("using System.Threading.Tasks;\n")
	sb.WriteString("using PulseRPC;\n")

	for _, ns := range allNamespaces {
		sb.WriteString(fmt.Sprintf("using %s;\n", ns))
	}
	sb.WriteString("\n")
	sb.WriteString("public class Program\n")
	sb.WriteString("{\n")
	sb.WriteString("    public static async Task Main(string[] args)\n")
	sb.WriteString("    {\n")
	sb.WriteString("        var baseUrl = args.Length > 0 ? args[0] : \"http://localhost:8080\";\n")
	sb.WriteString("        var transport = new HttpTransport(baseUrl);\n")
	sb.WriteString("        var errors = new List<string>();\n\n")

	// Generate test calls for all interfaces and methods
	for _, iface := range idl.Interfaces {
		fmt.Fprintf(&sb, "        var %sClient = new %sClient(transport);\n", strings.ToLower(iface.Name), iface.Name)
		sb.WriteString("\n")
		for _, method := range iface.Methods {
			writeTestClientMethodCallCs(&sb, iface, method, structMap, enumMap)
		}
	}

	sb.WriteString("        if (errors.Count > 0)\n")
	sb.WriteString("        {\n")
	sb.WriteString("            Console.WriteLine($\"FAILED: {errors.Count} test(s) failed\");\n")
	sb.WriteString("            foreach (var error in errors)\n")
	sb.WriteString("            {\n")
	sb.WriteString("                Console.WriteLine($\"  {error}\");\n")
	sb.WriteString("            }\n")
	sb.WriteString("            Environment.Exit(1);\n")
	sb.WriteString("        }\n")
	sb.WriteString("        else\n")
	sb.WriteString("        {\n")
	sb.WriteString("            Console.WriteLine(\"SUCCESS: All tests passed!\");\n")
	sb.WriteString("            Environment.Exit(0);\n")
	sb.WriteString("        }\n")
	sb.WriteString("    }\n")
	sb.WriteString("}\n")

	return sb.String()
}

// generateTestServerCsproj generates TestServer.csproj project file
// Note: .NET SDK automatically includes all .cs files in the project directory,
// so we exclude Client.cs and TestClient.cs to avoid duplicate class definitions.
func generateTestServerCsproj() string {
	var sb strings.Builder

	sb.WriteString("<Project Sdk=\"Microsoft.NET.Sdk\">\n\n")
	sb.WriteString("  <PropertyGroup>\n")
	sb.WriteString("    <TargetFramework>net8.0</TargetFramework>\n")
	sb.WriteString("    <ImplicitUsings>enable</ImplicitUsings>\n")
	sb.WriteString("    <Nullable>enable</Nullable>\n")
	sb.WriteString("    <LangVersion>latest</LangVersion>\n")
	sb.WriteString("    <OutputType>Exe</OutputType>\n")
	sb.WriteString("  </PropertyGroup>\n\n")

	sb.WriteString("  <ItemGroup>\n")
	sb.WriteString("    <FrameworkReference Include=\"Microsoft.AspNetCore.App\" />\n")
	sb.WriteString("  </ItemGroup>\n\n")

	sb.WriteString("  <ItemGroup>\n")
	sb.WriteString("    <Compile Remove=\"Client.cs\" />\n")
	sb.WriteString("    <Compile Remove=\"TestClient.cs\" />\n")
	sb.WriteString("  </ItemGroup>\n\n")

	sb.WriteString("</Project>\n")

	return sb.String()
}

// generateTestClientCsproj generates TestClient.csproj project file
// Note: .NET SDK automatically includes all .cs files in the project directory,
// so we exclude Server.cs and TestServer.cs to avoid duplicate class definitions.
func generateTestClientCsproj() string {
	var sb strings.Builder

	sb.WriteString("<Project Sdk=\"Microsoft.NET.Sdk\">\n\n")
	sb.WriteString("  <PropertyGroup>\n")
	sb.WriteString("    <TargetFramework>net8.0</TargetFramework>\n")
	sb.WriteString("    <ImplicitUsings>enable</ImplicitUsings>\n")
	sb.WriteString("    <Nullable>enable</Nullable>\n")
	sb.WriteString("    <LangVersion>latest</LangVersion>\n")
	sb.WriteString("    <OutputType>Exe</OutputType>\n")
	sb.WriteString("  </PropertyGroup>\n\n")

	sb.WriteString("  <ItemGroup>\n")
	sb.WriteString("    <FrameworkReference Include=\"Microsoft.AspNetCore.App\" />\n")
	sb.WriteString("  </ItemGroup>\n\n")

	sb.WriteString("  <ItemGroup>\n")
	sb.WriteString("    <Compile Remove=\"Server.cs\" />\n")
	sb.WriteString("    <Compile Remove=\"TestServer.cs\" />\n")
	sb.WriteString("  </ItemGroup>\n\n")

	sb.WriteString("</Project>\n")

	return sb.String()
}

// writeTestInterfaceImplCs generates a concrete implementation class for an interface
func writeTestInterfaceImplCs(sb *strings.Builder, iface *parser.Interface, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	implName := iface.Name + "Impl"
	fmt.Fprintf(sb, "public class %s : I%s\n", implName, iface.Name)
	sb.WriteString("{\n")

	for _, method := range iface.Methods {
		writeTestMethodImplCs(sb, iface, method, structMap, enumMap)
	}

	sb.WriteString("}\n\n")
}

// writeTestMethodImplCs generates a concrete method implementation
func writeTestMethodImplCs(sb *strings.Builder, iface *parser.Interface, method *parser.Method, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	// Return type
	returnType := "object"
	if method.ReturnType != nil {
		returnType = mapTypeToCsType(method.ReturnType, structMap, enumMap, method.ReturnOptional)
	}
	fmt.Fprintf(sb, "    public %s ", returnType)

	fmt.Fprintf(sb, "%s(", method.Name)

	// Parameters
	for i, param := range method.Parameters {
		if i > 0 {
			sb.WriteString(", ")
		}
		paramType := mapTypeToCsType(param.Type, structMap, enumMap, false)
		fmt.Fprintf(sb, "%s %s", paramType, param.Name)
	}
	sb.WriteString(")\n")
	sb.WriteString("    {\n")

	// Implement based on method name and IDL comments
	writeMethodImplementationCs(sb, iface, method, structMap, enumMap)

	sb.WriteString("    }\n\n")
}

// writeMethodImplementationCs generates the actual method implementation body
func writeMethodImplementationCs(sb *strings.Builder, iface *parser.Interface, method *parser.Method, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	methodName := method.Name
	interfaceName := iface.Name

	// Handle specific methods based on IDL comments
	switch interfaceName {
	case "A":
		switch methodName {
		case "add":
			// returns a+b
			sb.WriteString("        return a + b;\n")
			return
		case "calc":
			// performs the given operation against all the values in nums and returns the result
			sb.WriteString("        double result = 0.0;\n")
			sb.WriteString("        if (nums.Count > 0)\n")
			sb.WriteString("        {\n")
			sb.WriteString("            result = nums[0];\n")
			sb.WriteString("            for (int i = 1; i < nums.Count; i++)\n")
			sb.WriteString("            {\n")
			sb.WriteString("                if (operation == MathOp.add)\n")
			sb.WriteString("                    result += nums[i];\n")
			sb.WriteString("                else if (operation == MathOp.multiply)\n")
			sb.WriteString("                    result *= nums[i];\n")
			sb.WriteString("            }\n")
			sb.WriteString("        }\n")
			sb.WriteString("        return result;\n")
			return
		case "sqrt":
			// returns the square root of a
			sb.WriteString("        return Math.Sqrt(a);\n")
			return
		case "repeat":
			// Echos the req1.to_repeat string as a list, optionally forcing to_repeat to upper case
			// RepeatResponse.items should be a list of strings whose length is equal to req1.count
			sb.WriteString("        var toRepeat = req1.ToRepeat;\n")
			sb.WriteString("        if (req1.ForceUppercase) toRepeat = toRepeat.ToUpper();\n")
			sb.WriteString("        var items = new List<string>();\n")
			sb.WriteString("        for (int i = 0; i < req1.Count; i++)\n")
			sb.WriteString("        {\n")
			sb.WriteString("            items.Add(toRepeat);\n")
			sb.WriteString("        }\n")
			sb.WriteString("        return new RepeatResponse\n")
			sb.WriteString("        {\n")
			sb.WriteString("            Status = Status.ok,\n")
			sb.WriteString("            Count = req1.Count,\n")
			sb.WriteString("            Items = items\n")
			sb.WriteString("        };\n")
			return
		case "say_hi":
			// returns a result with: hi="hi" (HiResponse only has hi field, not status)
			sb.WriteString("        return new HiResponse\n")
			sb.WriteString("        {\n")
			sb.WriteString("            Hi = \"hi\"\n")
			sb.WriteString("        };\n")
			return
		case "repeat_num":
			// returns num as an array repeated 'count' number of times
			sb.WriteString("        var result = new List<int>();\n")
			sb.WriteString("        for (int i = 0; i < count; i++)\n")
			sb.WriteString("        {\n")
			sb.WriteString("            result.Add((int)num);\n")
			sb.WriteString("        }\n")
			sb.WriteString("        return result;\n")
			return
		case "putPerson":
			// simply returns p.personId
			sb.WriteString("        return p.PersonId;\n")
			return
		}
	case "B":
		switch methodName {
		case "echo":
			// simply returns s, if s == "return-null" then you should return a null
			sb.WriteString("        if (s == \"return-null\") return null;\n")
			sb.WriteString("        return s;\n")
			return
		}
	}

	// Default implementation for methods not specifically handled
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
			elementType := mapTypeToCsType(method.ReturnType.Array, structMap, enumMap, false)
			fmt.Fprintf(sb, "        return new List<%s>();\n", elementType)
		} else if method.ReturnType.IsUserDefined() {
			// For user-defined types, check if it's an enum or struct
			typeName := method.ReturnType.UserDefined
			if enumDef, ok := enumMap[typeName]; ok {
				// Return the first enum value
				if len(enumDef.Values) > 0 {
					fmt.Fprintf(sb, "        return %s.%s;\n", typeName, enumDef.Values[0].Name)
				} else {
					sb.WriteString("        return null;\n")
				}
			} else if structDef, ok := structMap[typeName]; ok {
				// Generate a struct instance with minimal valid values for required fields
				if method.ReturnOptional {
					sb.WriteString("        return null;\n")
				} else {
					writeMinimalStructInstanceCs(sb, typeName, structDef, structMap, enumMap)
				}
			} else {
				sb.WriteString("        return null;\n")
			}
		} else {
			sb.WriteString("        return null;\n")
		}
	} else {
		sb.WriteString("        return null;\n")
	}
}

// writeMinimalStructInstanceCs generates a minimal valid struct instance with all required fields
func writeMinimalStructInstanceCs(sb *strings.Builder, typeName string, structDef *parser.Struct, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	fmt.Fprintf(sb, "        return new %s\n", typeName)
	sb.WriteString("        {\n")

	for _, field := range structDef.Fields {
		// Only include required fields
		if field.Optional {
			continue
		}

		fieldName := field.Name
		// Convert to PascalCase for C#
		csFieldName := snakeToPascalCase(fieldName)

		// Generate appropriate default value based on field type
		if field.Type.IsBuiltIn() {
			switch field.Type.BuiltIn {
			case "string":
				fmt.Fprintf(sb, "            %s = \"test\",\n", csFieldName)
			case "int":
				fmt.Fprintf(sb, "            %s = 42,\n", csFieldName)
			case "float":
				fmt.Fprintf(sb, "            %s = 3.14,\n", csFieldName)
			case "bool":
				fmt.Fprintf(sb, "            %s = true,\n", csFieldName)
			default:
				fmt.Fprintf(sb, "            %s = null,\n", csFieldName)
			}
		} else if field.Type.IsArray() {
			elementType := mapTypeToCsType(field.Type.Array, structMap, enumMap, false)
			fmt.Fprintf(sb, "            %s = new List<%s>(),\n", csFieldName, elementType)
		} else if field.Type.IsUserDefined() {
			userType := field.Type.UserDefined
			// Check if it's an enum
			if enumDef, ok := enumMap[userType]; ok {
				// Use first enum value
				if len(enumDef.Values) > 0 {
					fmt.Fprintf(sb, "            %s = %s.%s,\n", csFieldName, userType, enumDef.Values[0].Name)
				} else {
					fmt.Fprintf(sb, "            %s = default,\n", csFieldName)
				}
			} else if nestedStruct, ok := structMap[userType]; ok {
				// Recursively generate nested struct (only go one level deep for simplicity)
				fmt.Fprintf(sb, "            %s = new %s\n", csFieldName, userType)
				sb.WriteString("            {\n")
				for _, nestedField := range nestedStruct.Fields {
					if nestedField.Optional {
						continue
					}
					nestedCsFieldName := snakeToPascalCase(nestedField.Name)
					if nestedField.Type.IsBuiltIn() {
						switch nestedField.Type.BuiltIn {
						case "string":
							fmt.Fprintf(sb, "                %s = \"test\",\n", nestedCsFieldName)
						case "int":
							fmt.Fprintf(sb, "                %s = 42,\n", nestedCsFieldName)
						case "float":
							fmt.Fprintf(sb, "                %s = 3.14,\n", nestedCsFieldName)
						case "bool":
							fmt.Fprintf(sb, "                %s = true,\n", nestedCsFieldName)
						default:
							fmt.Fprintf(sb, "                %s = default,\n", nestedCsFieldName)
						}
					} else if nestedField.Type.IsUserDefined() {
						if nestedEnum, ok := enumMap[nestedField.Type.UserDefined]; ok {
							if len(nestedEnum.Values) > 0 {
								fmt.Fprintf(sb, "                %s = %s.%s,\n", nestedCsFieldName, nestedField.Type.UserDefined, nestedEnum.Values[0].Name)
							} else {
								fmt.Fprintf(sb, "                %s = default,\n", nestedCsFieldName)
							}
						} else {
							fmt.Fprintf(sb, "                %s = null,\n", nestedCsFieldName)
						}
					} else {
						fmt.Fprintf(sb, "                %s = default,\n", nestedCsFieldName)
					}
				}
				sb.WriteString("            },\n")
			} else {
				fmt.Fprintf(sb, "            %s = null,\n", csFieldName)
			}
		} else {
			fmt.Fprintf(sb, "            %s = null,\n", csFieldName)
		}
	}

	sb.WriteString("        };\n")
}

// writeTestClientMethodCallCs generates a test method call
func writeTestClientMethodCallCs(sb *strings.Builder, iface *parser.Interface, method *parser.Method, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	fmt.Fprintf(sb, "        try\n")
	sb.WriteString("        {\n")
	fmt.Fprintf(sb, "            var result = await %sClient.%sAsync(", strings.ToLower(iface.Name), method.Name)

	// Generate test parameter values
	for i, param := range method.Parameters {
		if i > 0 {
			sb.WriteString(", ")
		}
		writeTestParamValueCs(sb, param, structMap, enumMap)
	}
	sb.WriteString(");\n")
	fmt.Fprintf(sb, "            Console.WriteLine($\"✓ %s.%s passed\");\n", iface.Name, method.Name)
	sb.WriteString("        }\n")
	sb.WriteString("        catch (Exception e)\n")
	sb.WriteString("        {\n")
	fmt.Fprintf(sb, "            errors.Add($\"%s.%s failed: {e.Message}\");\n", iface.Name, method.Name)
	sb.WriteString("        }\n\n")
}

// writeTestParamValueCs generates C# code for a test parameter value
func writeTestParamValueCs(sb *strings.Builder, param *parser.Parameter, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
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
		elementType := mapTypeToCsType(param.Type.Array, structMap, enumMap, false)
		fmt.Fprintf(sb, "new List<%s> { ", elementType)
		// Generate a single element for the array
		writeTestFieldValueCs(sb, param.Type.Array, structMap, enumMap)
		sb.WriteString(" }")
	} else if param.Type.IsUserDefined() {
		// Check if it's an enum or struct
		typeName := param.Type.UserDefined
		// First try with qualified name (for imported types like inc.MathOp)
		// Try to find enum or struct
		unqualifiedName := typeName
		if strings.Contains(typeName, ".") {
			parts := strings.Split(typeName, ".")
			unqualifiedName = parts[len(parts)-1]
		}

		enumDef, isEnum := enumMap[unqualifiedName]
		if !isEnum {
			enumDef, isEnum = enumMap[typeName]
		}

		if isEnum {
			// It's an enum - use the first enum value (enum name is the type name)
			if len(enumDef.Values) > 0 {
				enumTypeName := getEnumTypeName(unqualifiedName, enumMap)
				fmt.Fprintf(sb, "%s.%s", enumTypeName, enumDef.Values[0].Name)
			} else {
				sb.WriteString("default")
			}
		} else if structDef, ok := structMap[unqualifiedName]; ok {
			// It's a struct - create instance of generated class
			writeStructInstanceCs(sb, structDef, structMap, enumMap)
		} else {
			// Unknown type - default to string
			sb.WriteString("\"test\"")
		}
	} else {
		sb.WriteString("null")
	}
}

// writeStructInstanceCs generates C# code to create an instance of a generated struct class
func writeStructInstanceCs(sb *strings.Builder, structDef *parser.Struct, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	className := getStructClassName(structDef.Name, structMap)
	fmt.Fprintf(sb, "new %s\n", className)
	sb.WriteString("        {\n")
	fieldCount := 0
	for _, field := range structDef.Fields {
		// For optional fields, sometimes omit them, sometimes set to null
		// For Person.email specifically, set to null to test optional enforcement
		if field.Optional && field.Name == "email" {
			if fieldCount > 0 {
				sb.WriteString(",\n")
			}
			propName := snakeToPascalCase(field.Name)
			fmt.Fprintf(sb, "            %s = null", propName)
			fieldCount++
		} else if !field.Optional {
			if fieldCount > 0 {
				sb.WriteString(",\n")
			}
			propName := snakeToPascalCase(field.Name)
			fmt.Fprintf(sb, "            %s = ", propName)
			writeTestFieldValueCs(sb, field.Type, structMap, enumMap)
			fieldCount++
		}
		// Skip optional fields that aren't email (they can be omitted)
	}
	sb.WriteString("\n        }")
}

// writeTestFieldValueCs generates C# code for a field value in a struct
func writeTestFieldValueCs(sb *strings.Builder, fieldType *parser.Type, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	if fieldType.IsBuiltIn() {
		switch fieldType.BuiltIn {
		case "string":
			sb.WriteString("\"test\"")
		case "int":
			sb.WriteString("42")
		case "float":
			sb.WriteString("3.14")
		case "bool":
			sb.WriteString("true")
		default:
			sb.WriteString("null")
		}
	} else if fieldType.IsArray() {
		elementTypeStr := mapTypeToCsType(fieldType.Array, structMap, enumMap, false)
		fmt.Fprintf(sb, "new List<%s> { ", elementTypeStr)
		// Generate a single element for the array
		writeTestFieldValueCs(sb, fieldType.Array, structMap, enumMap)
		sb.WriteString(" }")
	} else if fieldType.IsUserDefined() {
		typeName := fieldType.UserDefined
		// First try with qualified name (for imported types like inc.MathOp)
		// Try to find enum or struct
		unqualifiedName := typeName
		if strings.Contains(typeName, ".") {
			parts := strings.Split(typeName, ".")
			unqualifiedName = parts[len(parts)-1]
		}

		enumDef, isEnum := enumMap[unqualifiedName]
		if !isEnum {
			enumDef, isEnum = enumMap[typeName]
		}

		if isEnum {
			// It's an enum - use the first enum value (enum name is the type name)
			if len(enumDef.Values) > 0 {
				enumTypeName := getEnumTypeName(unqualifiedName, enumMap)
				fmt.Fprintf(sb, "%s.%s", enumTypeName, enumDef.Values[0].Name)
			} else {
				sb.WriteString("default")
			}
		} else if structDef, ok := structMap[unqualifiedName]; ok {
			// It's a struct - create instance of generated class
			writeStructInstanceCs(sb, structDef, structMap, enumMap)
		} else {
			// Unknown type - default to string
			sb.WriteString("\"test\"")
		}
	} else {
		sb.WriteString("null")
	}
}

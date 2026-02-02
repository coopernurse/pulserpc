package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/coopernurse/pulserpc/pkg/generator"
	"github.com/coopernurse/pulserpc/pkg/parser"
	"github.com/coopernurse/pulserpc/pkg/webui"
)

func main() {
	// Register all available plugins
	registerPlugins()

	// Define global flags
	var validate = flag.Bool("validate", false, "Validate the IDL after parsing")
	var toJSON = flag.String("to-json", "", "Write parsed IDL as JSON to the specified file")
	var fromJSON = flag.String("from-json", "", "Read JSON file and generate IDL text on STDOUT")
	var pluginName = flag.String("plugin", "", "Code generation plugin to use (e.g., python-client-server)")
	var uiMode = flag.Bool("ui", false, "Start the embedded web UI server")
	var uiPort = flag.Int("ui-port", 8080, "Port for the web UI server (default: 8080)")
	_ = flag.String("dir", "", "Output directory for generated code") // Available to plugins via FlagSet
	_ = flag.Bool("silent", false, "Suppress file generation output")
	_ = flag.Bool("generate-test-files", false, "Generate test files (test_server.*, test_client.*)")

	// Register flags for all plugins
	allPlugins := getAllPlugins()
	for _, plugin := range allPlugins {
		plugin.RegisterFlags(flag.CommandLine)
	}

	flag.Parse()

	// Handle UI server mode - must be checked early
	if *uiMode {
		server := webui.NewServer(*uiPort)
		if err := server.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to start web UI server: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Check for mutual exclusivity
	if *toJSON != "" && *fromJSON != "" {
		fmt.Fprintf(os.Stderr, "error: -to-json and -from-json cannot be used together\n")
		os.Exit(1)
	}

	if *pluginName != "" && (*toJSON != "" || *fromJSON != "") {
		fmt.Fprintf(os.Stderr, "error: -plugin cannot be used with -to-json or -from-json\n")
		os.Exit(1)
	}

	// Handle JSON input mode
	if *fromJSON != "" {
		handleJSONInput(*fromJSON)
		return
	}

	// Handle normal IDL parsing or JSON output mode
	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "error: filename required\n")
		os.Exit(1)
	}

	filename := args[0]

	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "error: file does not exist: %s\n", filename)
		os.Exit(1)
	}

	// Read file content
	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to read file %s: %v\n", filename, err)
		os.Exit(1)
	}

	// Parse IDL
	idl, err := parser.ParseIDL(filename, string(content))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Validate if flag is set
	if *validate {
		if err := parser.ValidateIDL(idl); err != nil {
			fmt.Fprintf(os.Stderr, "error: validation failed: %v\n", err)
			os.Exit(1)
		}
	}

	// Handle plugin generation mode
	if *pluginName != "" {
		handlePluginGeneration(*pluginName, idl)
		return
	}

	// Handle JSON output mode
	if *toJSON != "" {
		handleJSONOutput(idl, *toJSON)
		return
	}

	// Pretty print to STDOUT
	prettyPrintIDL(idl)
}

func handleJSONInput(jsonFile string) {
	// Read JSON file
	content, err := os.ReadFile(jsonFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to read JSON file %s: %v\n", jsonFile, err)
		os.Exit(1)
	}

	// Unmarshal JSON
	var idl parser.IDL
	if err := json.Unmarshal(content, &idl); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to parse JSON: %v\n", err)
		os.Exit(1)
	}

	// Always validate JSON input
	if err := parser.ValidateIDL(&idl); err != nil {
		fmt.Fprintf(os.Stderr, "error: validation failed: %v\n", err)
		os.Exit(1)
	}

	// Generate IDL text on STDOUT
	fmt.Print(generateIDLText(&idl))
}

func handleJSONOutput(idl *parser.IDL, outputFile string) {
	// Marshal to JSON with indentation
	jsonData, err := json.MarshalIndent(idl, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to marshal IDL to JSON: %v\n", err)
		os.Exit(1)
	}

	// Write to file
	if err := os.WriteFile(outputFile, jsonData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to write JSON file %s: %v\n", outputFile, err)
		os.Exit(1)
	}
}

func prettyPrintIDL(idl *parser.IDL) {
	if len(idl.Interfaces) > 0 {
		fmt.Println("Interfaces:")
		for _, iface := range idl.Interfaces {
			fmt.Printf("  %s:\n", iface.Name)
			if iface.Namespace != "" {
				fmt.Printf("    Namespace: %s\n", iface.Namespace)
			}
			fmt.Println("    Methods:")
			for _, method := range iface.Methods {
				fmt.Printf("      %s(", method.Name)
				for i, param := range method.Parameters {
					if i > 0 {
						fmt.Print(", ")
					}
					fmt.Printf("%s %s", param.Name, param.Type.String())
				}
				fmt.Printf(") %s\n", method.ReturnType.String())
			}
		}
	}

	if len(idl.Structs) > 0 {
		fmt.Println("Structs:")
		for _, s := range idl.Structs {
			if s.Extends != "" {
				fmt.Printf("  %s extends %s:\n", s.Name, s.Extends)
			} else {
				fmt.Printf("  %s:\n", s.Name)
			}
			if s.Namespace != "" {
				fmt.Printf("    Namespace: %s\n", s.Namespace)
			}
			fmt.Println("    Fields:")
			for _, field := range s.Fields {
				optional := ""
				if field.Optional {
					optional = " [optional]"
				}
				fmt.Printf("      %s: %s%s\n", field.Name, field.Type.String(), optional)
			}
		}
	}

	if len(idl.Enums) > 0 {
		fmt.Println("Enums:")
		for _, enum := range idl.Enums {
			fmt.Printf("  %s:\n", enum.Name)
			if enum.Namespace != "" {
				fmt.Printf("    Namespace: %s\n", enum.Namespace)
			}
			fmt.Print("    Values: ")
			for i, value := range enum.Values {
				if i > 0 {
					fmt.Print(", ")
				}
				fmt.Print(value.Name)
			}
			fmt.Println()
		}
	}
}

// generateIDLText converts a parsed IDL structure back to IDL text format
func generateIDLText(idl *parser.IDL) string {
	var sb strings.Builder

	// Group elements by namespace
	namespaceInterfaces := make(map[string][]*parser.Interface)
	namespaceStructs := make(map[string][]*parser.Struct)
	namespaceEnums := make(map[string][]*parser.Enum)

	// Collect all elements by namespace
	for _, iface := range idl.Interfaces {
		ns := iface.Namespace
		namespaceInterfaces[ns] = append(namespaceInterfaces[ns], iface)
	}

	for _, s := range idl.Structs {
		ns := s.Namespace
		namespaceStructs[ns] = append(namespaceStructs[ns], s)
	}

	for _, enum := range idl.Enums {
		ns := enum.Namespace
		namespaceEnums[ns] = append(namespaceEnums[ns], enum)
	}

	// Collect all unique namespaces (excluding empty string)
	allNamespaces := make(map[string]bool)
	for ns := range namespaceInterfaces {
		if ns != "" {
			allNamespaces[ns] = true
		}
	}
	for ns := range namespaceStructs {
		if ns != "" {
			allNamespaces[ns] = true
		}
	}
	for ns := range namespaceEnums {
		if ns != "" {
			allNamespaces[ns] = true
		}
	}

	// Output elements without namespace first (no namespace declaration)
	if ifaces, ok := namespaceInterfaces[""]; ok {
		for _, iface := range ifaces {
			writeInterface(&sb, iface)
		}
	}
	if structs, ok := namespaceStructs[""]; ok {
		for _, s := range structs {
			writeStruct(&sb, s)
		}
	}
	if enums, ok := namespaceEnums[""]; ok {
		for _, enum := range enums {
			writeEnum(&sb, enum)
		}
	}

	// Output elements with namespaces, grouped by namespace
	for ns := range allNamespaces {
		// Output namespace declaration
		fmt.Fprintf(&sb, "namespace %s\n\n", ns)

		// Output all interfaces in this namespace
		if ifaces, ok := namespaceInterfaces[ns]; ok {
			for _, iface := range ifaces {
				writeInterface(&sb, iface)
			}
		}

		// Output all structs in this namespace
		if structs, ok := namespaceStructs[ns]; ok {
			for _, s := range structs {
				writeStruct(&sb, s)
			}
		}

		// Output all enums in this namespace
		if enums, ok := namespaceEnums[ns]; ok {
			for _, enum := range enums {
				writeEnum(&sb, enum)
			}
		}
	}

	return sb.String()
}

func writeInterface(sb *strings.Builder, iface *parser.Interface) {
	if iface.Comment != "" {
		writeComment(sb, iface.Comment)
	}
	fmt.Fprintf(sb, "interface %s {\n", iface.Name)
	for _, method := range iface.Methods {
		fmt.Fprintf(sb, "  %s(", method.Name)
		for i, param := range method.Parameters {
			if i > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(sb, "%s %s", param.Name, param.Type.String())
		}
		fmt.Fprintf(sb, ") %s\n", method.ReturnType.String())
	}
	sb.WriteString("}\n\n")
}

func writeStruct(sb *strings.Builder, s *parser.Struct) {
	if s.Comment != "" {
		writeComment(sb, s.Comment)
	}
	if s.Extends != "" {
		fmt.Fprintf(sb, "struct %s extends %s {\n", s.Name, s.Extends)
	} else {
		fmt.Fprintf(sb, "struct %s {\n", s.Name)
	}
	for _, field := range s.Fields {
		if field.Comment != "" {
			writeComment(sb, field.Comment)
		}
		optional := ""
		if field.Optional {
			optional = " [optional]"
		}
		fmt.Fprintf(sb, "  %s %s%s\n", field.Name, field.Type.String(), optional)
	}
	sb.WriteString("}\n\n")
}

func writeEnum(sb *strings.Builder, enum *parser.Enum) {
	if enum.Comment != "" {
		writeComment(sb, enum.Comment)
	}
	fmt.Fprintf(sb, "enum %s {\n", enum.Name)
	for _, value := range enum.Values {
		if value.Comment != "" {
			writeComment(sb, value.Comment)
		}
		fmt.Fprintf(sb, "  %s\n", value.Name)
	}
	sb.WriteString("}\n\n")
}

func writeComment(sb *strings.Builder, comment string) {
	lines := strings.Split(comment, "\n")
	for _, line := range lines {
		fmt.Fprintf(sb, "// %s\n", line)
	}
}

// registerPlugins registers all available code generation plugins
func registerPlugins() {
	generator.Register(generator.NewPythonClientServer())
	generator.Register(generator.NewTSClientServer())
	generator.Register(generator.NewCSharpClientServer())
	generator.Register(generator.NewJavaClientServer())
	generator.Register(generator.NewGoClientServer())
	// Add more plugins here as they are implemented
}

// getAllPlugins returns a slice of all registered plugins
func getAllPlugins() []generator.Plugin {
	pluginNames := generator.List()
	plugins := make([]generator.Plugin, 0, len(pluginNames))
	for _, name := range pluginNames {
		if plugin, ok := generator.Get(name); ok {
			plugins = append(plugins, plugin)
		}
	}
	return plugins
}

// handlePluginGeneration routes IDL to the specified plugin for code generation
func handlePluginGeneration(pluginName string, idl *parser.IDL) {
	plugin, ok := generator.Get(pluginName)
	if !ok {
		fmt.Fprintf(os.Stderr, "error: unknown plugin %q\n", pluginName)
		fmt.Fprintf(os.Stderr, "available plugins: %v\n", generator.List())
		os.Exit(1)
	}

	// Pass the global FlagSet so the plugin can access all parsed flag values
	if err := plugin.Generate(idl, flag.CommandLine); err != nil {
		fmt.Fprintf(os.Stderr, "error: plugin %q failed: %v\n", pluginName, err)
		os.Exit(1)
	}
}

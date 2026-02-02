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

// TSClientServer is a plugin that generates TypeScript HTTP server and client code from IDL
type TSClientServer struct {
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
	fs.String("package", "", "Package prefix for generated types and classes (for namespace isolation)")
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

	// Generate one file per namespace
	for namespace, types := range namespaceMap {
		if namespace == "" {
			continue // Skip types without namespace (shouldn't happen with required namespaces)
		}
		namespaceCode := generateNamespaceTs(namespace, types)
		namespacePath := filepath.Join(outputDir, namespace+".ts")
		if err := os.WriteFile(namespacePath, []byte(namespaceCode), 0644); err != nil {
			return fmt.Errorf("failed to write %s.ts: %w", namespace, err)
		}
		PrintFileCreated(namespacePath, fs)
	}

	// Marshal IDL to JSON for embedding in server code
	idlJSON, err := json.MarshalIndent(idl, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal IDL to JSON: %w", err)
	}

	// Generate server.ts with embedded IDL
	serverCode := generateServerTs(idl, structMap, enumMap, interfaceMap, packagePrefix, namespaceMap, string(idlJSON))
	serverPath := filepath.Join(outputDir, "server.ts")
	if err := os.WriteFile(serverPath, []byte(serverCode), 0644); err != nil {
		return fmt.Errorf("failed to write server.ts: %w", err)
	}
	PrintFileCreated(serverPath, fs)

	// Generate client.ts
	clientCode := generateClientTs(idl, structMap, enumMap, interfaceMap, packagePrefix, namespaceMap)
	clientPath := filepath.Join(outputDir, "client.ts")
	if err := os.WriteFile(clientPath, []byte(clientCode), 0644); err != nil {
		return fmt.Errorf("failed to write client.ts: %w", err)
	}
	PrintFileCreated(clientPath, fs)

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
func (p *TSClientServer) copyRuntimeFiles(outputDir string, silent bool) error {
	return runtime.CopyRuntimeFiles("ts", outputDir, silent)
}

// generateNamespaceTs generates a TypeScript file for a single namespace
func generateNamespaceTs(namespace string, types *NamespaceTypes) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString("// Type definitions (TypeScript types, erased at runtime)\n")
	sb.WriteString("interface TypeDef {\n")
	sb.WriteString("  builtIn?: string;\n")
	sb.WriteString("  array?: TypeDef;\n")
	sb.WriteString("  mapValue?: TypeDef;\n")
	sb.WriteString("  userDefined?: string;\n")
	sb.WriteString("}\n")
	sb.WriteString("interface StructDef {\n")
	sb.WriteString("  extends?: string;\n")
	sb.WriteString("  fields: Array<{ name: string; type: TypeDef; optional?: boolean }>;\n")
	sb.WriteString("}\n")
	sb.WriteString("interface EnumDef {\n")
	sb.WriteString("  values: Array<{ name: string }>;\n")
	sb.WriteString("}\n")
	sb.WriteString("type StructMap = { [key: string]: StructDef };\n")
	sb.WriteString("type EnumMap = { [key: string]: EnumDef };\n\n")

	// Generate IDL-specific type definitions for this namespace
	sb.WriteString(fmt.Sprintf("// IDL-specific type definitions for namespace: %s\n", namespace))
	sb.WriteString("const ALL_STRUCTS: StructMap = {\n")
	for _, s := range types.Structs {
		sb.WriteString(fmt.Sprintf("  '%s': {\n", s.Name))
		if s.Extends != "" {
			sb.WriteString(fmt.Sprintf("    extends: '%s',\n", s.Extends))
		}
		sb.WriteString("    fields: [\n")
		for _, field := range s.Fields {
			sb.WriteString("      {\n")
			sb.WriteString(fmt.Sprintf("        name: '%s',\n", field.Name))
			sb.WriteString("        type: ")
			writeTypeDictTs(&sb, field.Type)
			sb.WriteString(",\n")
			if field.Optional {
				sb.WriteString("        optional: true,\n")
			}
			sb.WriteString("      },\n")
		}
		sb.WriteString("    ],\n")
		sb.WriteString("  },\n")
	}
	sb.WriteString("};\n\n")

	sb.WriteString("const ALL_ENUMS: EnumMap = {\n")
	for _, e := range types.Enums {
		sb.WriteString(fmt.Sprintf("  '%s': {\n", e.Name))
		sb.WriteString("    values: [\n")
		for _, val := range e.Values {
			sb.WriteString(fmt.Sprintf("      { name: '%s' },\n", val.Name))
		}
		sb.WriteString("    ],\n")
		sb.WriteString("  },\n")
	}
	sb.WriteString("};\n\n")
	sb.WriteString("// Export for CommonJS compatibility\n")
	sb.WriteString("export { ALL_STRUCTS, ALL_ENUMS };\n")

	return sb.String()
}

// writeTypeDictTs writes a type definition as a TypeScript object
func writeTypeDictTs(sb *strings.Builder, t *parser.Type) {
	sb.WriteString("{")
	if t.IsBuiltIn() {
		fmt.Fprintf(sb, "builtIn: '%s'", t.BuiltIn)
	} else if t.IsArray() {
		sb.WriteString("array: ")
		writeTypeDictTs(sb, t.Array)
	} else if t.IsMap() {
		sb.WriteString("mapValue: ")
		writeTypeDictTs(sb, t.MapValue)
	} else if t.IsUserDefined() {
		fmt.Fprintf(sb, "userDefined: '%s'", t.UserDefined)
	}
	sb.WriteString("}")
}

// applyPackagePrefix applies package prefix to a name if provided
func applyPackagePrefix(name, prefix string) string {
	if prefix != "" {
		return prefix + name
	}
	return name
}

// generateServerTs generates the server.ts file with HTTP server and interface stubs
func generateServerTs(idl *parser.IDL, _ map[string]*parser.Struct, _ map[string]*parser.Enum, _ map[string]*parser.Interface, packagePrefix string, namespaceMap map[string]*NamespaceTypes, idlJson string) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString("/// <reference types=\"node\" />\n\n")
	sb.WriteString("import * as http from 'http';\n")
	sb.WriteString("import { RPCError } from './pulserpc/rpc';\n")
	sb.WriteString("import { validateType } from './pulserpc/validation';\n\n")
	sb.WriteString("// Embedded IDL JSON for pulserpc-idl RPC method\n")
	sb.WriteString("const IDL_JSON: string = ")
	sb.WriteString(escapeTypeScriptTemplateLiteral(idlJson))
	sb.WriteString(";\n\n")

	// Import from namespace files
	namespaces := make([]string, 0, len(namespaceMap))
	for ns := range namespaceMap {
		if ns != "" {
			namespaces = append(namespaces, ns)
		}
	}
	// Sort namespaces for consistent output
	sort.Strings(namespaces)
	for _, ns := range namespaces {
		importPath := "./" + ns
		sb.WriteString(fmt.Sprintf("import { ALL_STRUCTS as %s_STRUCTS, ALL_ENUMS as %s_ENUMS } from '%s';\n", strings.ToUpper(ns), strings.ToUpper(ns), importPath))
	}
	sb.WriteString("\n")
	sb.WriteString("// Inline type definitions\n")
	sb.WriteString("interface TypeDef {\n")
	sb.WriteString("  builtIn?: string;\n")
	sb.WriteString("  array?: TypeDef;\n")
	sb.WriteString("  mapValue?: TypeDef;\n")
	sb.WriteString("  userDefined?: string;\n")
	sb.WriteString("}\n")
	sb.WriteString("interface StructDef {\n")
	sb.WriteString("  extends?: string;\n")
	sb.WriteString("  fields: Array<{ name: string; type: TypeDef; optional?: boolean }>;\n")
	sb.WriteString("}\n")
	sb.WriteString("interface EnumDef {\n")
	sb.WriteString("  values: Array<{ name: string }>;\n")
	sb.WriteString("}\n")
	sb.WriteString("type StructMap = { [key: string]: StructDef };\n")
	sb.WriteString("type EnumMap = { [key: string]: EnumDef };\n\n")
	sb.WriteString("// Merge ALL_STRUCTS and ALL_ENUMS from all namespaces\n")
	sb.WriteString("const ALL_STRUCTS: StructMap = {\n")
	// Merge structs from all namespaces
	for _, ns := range namespaces {
		sb.WriteString(fmt.Sprintf("  ...%s_STRUCTS,\n", strings.ToUpper(ns)))
	}
	sb.WriteString("};\n\n")

	sb.WriteString("const ALL_ENUMS: EnumMap = {\n")
	// Merge enums from all namespaces
	for _, ns := range namespaces {
		sb.WriteString(fmt.Sprintf("  ...%s_ENUMS,\n", strings.ToUpper(ns)))
	}
	sb.WriteString("};\n\n")

	// Generate interface stub abstract classes
	for _, iface := range idl.Interfaces {
		writeInterfaceStubTs(&sb, iface, packagePrefix)
	}

	// Generate PulseRPCServer class
	serverClassName := applyPackagePrefix("PulseRPCServer", packagePrefix)
	fmt.Fprintf(&sb, "export class %s {\n", serverClassName)
	sb.WriteString("  private host: string;\n")
	sb.WriteString("  private port: number;\n")
	sb.WriteString("  private handlers: Map<string, any>;\n")
	sb.WriteString("  private server: http.Server | null;\n\n")

	sb.WriteString("  constructor(host: string = 'localhost', port: number = 8080) {\n")
	sb.WriteString("    this.host = host;\n")
	sb.WriteString("    this.port = port;\n")
	sb.WriteString("    this.handlers = new Map();\n")
	sb.WriteString("    this.server = null;\n")
	sb.WriteString("  }\n\n")

	sb.WriteString("  register(interfaceName: string, instance: any): void {\n")
	sb.WriteString("    this.handlers.set(interfaceName, instance);\n")
	sb.WriteString("  }\n\n")

	// Generate handleRequest method
	writeServerHandleRequestTs(&sb, idl.Interfaces)

	// Generate serveForever and shutdown methods
	sb.WriteString("  serveForever(): void {\n")
	sb.WriteString("    this.server = http.createServer((req, res) => {\n")
	sb.WriteString("      if (req.method !== 'POST') {\n")
	sb.WriteString("        res.writeHead(405, { 'Content-Type': 'application/json' });\n")
	sb.WriteString("        res.end(JSON.stringify({ error: 'Method Not Allowed' }));\n")
	sb.WriteString("        return;\n")
	sb.WriteString("      }\n\n")
	sb.WriteString("      let body = '';\n")
	sb.WriteString("      req.on('data', (chunk) => { body += chunk.toString(); });\n")
	sb.WriteString("      req.on('end', () => {\n")
	sb.WriteString("        try {\n")
	sb.WriteString("          const data = JSON.parse(body);\n\n")
	sb.WriteString("          // Handle batch requests\n")
	sb.WriteString("          if (Array.isArray(data)) {\n")
	sb.WriteString("            if (data.length === 0) {\n")
	sb.WriteString("              res.writeHead(400, { 'Content-Type': 'application/json' });\n")
	sb.WriteString("              res.end(JSON.stringify({ error: 'Empty batch array' }));\n")
	sb.WriteString("              return;\n")
	sb.WriteString("            }\n")
	sb.WriteString("            const responses: any[] = [];\n")
	sb.WriteString("            for (const req of data) {\n")
	sb.WriteString("              const response = this.handleRequest(req);\n")
	sb.WriteString("              if (response !== null && response !== undefined) {\n")
	sb.WriteString("                responses.push(response);\n")
	sb.WriteString("              }\n")
	sb.WriteString("            }\n")
	sb.WriteString("            if (responses.length === 0) {\n")
	sb.WriteString("              res.writeHead(204);\n")
	sb.WriteString("              res.end();\n")
	sb.WriteString("            } else {\n")
	sb.WriteString("              res.writeHead(200, { 'Content-Type': 'application/json' });\n")
	sb.WriteString("              res.end(JSON.stringify(responses));\n")
	sb.WriteString("            }\n")
	sb.WriteString("          } else {\n")
	sb.WriteString("            const response = this.handleRequest(data);\n")
	sb.WriteString("            if (response === null || response === undefined) {\n")
	sb.WriteString("              res.writeHead(204);\n")
	sb.WriteString("              res.end();\n")
	sb.WriteString("            } else {\n")
	sb.WriteString("              res.writeHead(200, { 'Content-Type': 'application/json' });\n")
	sb.WriteString("              res.end(JSON.stringify(response));\n")
	sb.WriteString("            }\n")
	sb.WriteString("          }\n")
	sb.WriteString("        } catch (err: any) {\n")
	sb.WriteString("          const errorResponse = this.errorResponse(null, -32700, 'Parse error', err.message);\n")
	sb.WriteString("          res.writeHead(200, { 'Content-Type': 'application/json' });\n")
	sb.WriteString("          res.end(JSON.stringify(errorResponse));\n")
	sb.WriteString("        }\n")
	sb.WriteString("      });\n")
	sb.WriteString("    });\n\n")
	sb.WriteString("    this.server.listen(this.port, this.host, () => {\n")
	sb.WriteString(fmt.Sprintf("      console.log(`%s server listening on http://${this.host}:${this.port}`);\n", serverClassName))
	sb.WriteString("    });\n")
	sb.WriteString("  }\n\n")

	sb.WriteString("  shutdown(): void {\n")
	sb.WriteString("    if (this.server) {\n")
	sb.WriteString("      this.server.close();\n")
	sb.WriteString("      this.server = null;\n")
	sb.WriteString("    }\n")
	sb.WriteString("  }\n")
	sb.WriteString("}\n")

	return sb.String()
}

// writeInterfaceStubTs generates an abstract class for an interface
func writeInterfaceStubTs(sb *strings.Builder, iface *parser.Interface, packagePrefix string) {
	if iface.Comment != "" {
		lines := strings.Split(strings.TrimSpace(iface.Comment), "\n")
		for _, line := range lines {
			fmt.Fprintf(sb, "// %s\n", line)
		}
	}
	className := applyPackagePrefix(iface.Name, packagePrefix)
	fmt.Fprintf(sb, "export abstract class %s {\n", className)

	for _, method := range iface.Methods {
		fmt.Fprintf(sb, "  abstract %s(", method.Name)
		for i, param := range method.Parameters {
			if i > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(sb, "%s: any", param.Name)
		}
		sb.WriteString("): any;\n")
	}
	sb.WriteString("}\n\n")
}

// writeServerHandleRequestTs generates the handleRequest method for the server
func writeServerHandleRequestTs(sb *strings.Builder, interfaces []*parser.Interface) {
	sb.WriteString("  handleRequest(requestJson: any): any {\n")
	sb.WriteString("    // Validate JSON-RPC 2.0 structure\n")
	sb.WriteString("    if (typeof requestJson !== 'object' || requestJson === null || Array.isArray(requestJson)) {\n")
	sb.WriteString("      return this.errorResponse(null, -32600, 'Invalid Request', 'Request must be an object');\n")
	sb.WriteString("    }\n\n")

	sb.WriteString("    const jsonrpc = requestJson.jsonrpc;\n")
	sb.WriteString("    if (jsonrpc !== '2.0') {\n")
	sb.WriteString("      return this.errorResponse(null, -32600, 'Invalid Request', \"jsonrpc must be '2.0'\");\n")
	sb.WriteString("    }\n\n")

	sb.WriteString("    const method = requestJson.method;\n")
	sb.WriteString("    if (typeof method !== 'string') {\n")
	sb.WriteString("      return this.errorResponse(null, -32600, 'Invalid Request', 'method must be a string');\n")
	sb.WriteString("    }\n\n")

	sb.WriteString("    let params = requestJson.params;\n")
	sb.WriteString("    const requestId = requestJson.id;\n")
	sb.WriteString("    const isNotification = !('id' in requestJson);\n\n")

	// Handle pulserpc-idl method
	sb.WriteString("    // Special case: pulserpc-idl method returns the IDL JSON document\n")
	sb.WriteString("    if (method === 'pulserpc-idl') {\n")
	sb.WriteString("      try {\n")
	sb.WriteString("        const idlDoc = JSON.parse(IDL_JSON);\n\n")
	sb.WriteString("        if (isNotification) {\n")
	sb.WriteString("          return null;\n")
	sb.WriteString("        }\n")
	sb.WriteString("        return {\n")
	sb.WriteString("          jsonrpc: '2.0',\n")
	sb.WriteString("          result: idlDoc,\n")
	sb.WriteString("          id: requestId,\n")
	sb.WriteString("        };\n")
	sb.WriteString("      } catch (err: any) {\n")
	sb.WriteString("        return this.errorResponse(requestId, -32603, 'Internal error', `Failed to parse IDL JSON: ${err.message}`);\n")
	sb.WriteString("      }\n")
	sb.WriteString("    }\n\n")

	// Parse method name
	sb.WriteString("    // Parse method name: interface.method\n")
	sb.WriteString("    const parts = method.split('.', 2);\n")
	sb.WriteString("    if (parts.length !== 2) {\n")
	sb.WriteString("      return this.errorResponse(requestId, -32601, 'Method not found', `Invalid method format: ${method}`);\n")
	sb.WriteString("    }\n\n")

	sb.WriteString("    const interfaceName = parts[0];\n")
	sb.WriteString("    const methodName = parts[1];\n\n")

	sb.WriteString("    // Find handler\n")
	sb.WriteString("    const handler = this.handlers.get(interfaceName);\n")
	sb.WriteString("    if (!handler) {\n")
	sb.WriteString("      return this.errorResponse(requestId, -32601, 'Method not found', `Interface '${interfaceName}' not registered`);\n")
	sb.WriteString("    }\n\n")

	sb.WriteString("    // Find method on handler\n")
	sb.WriteString("    if (typeof handler[methodName] !== 'function') {\n")
	sb.WriteString("      return this.errorResponse(requestId, -32601, 'Method not found', `Method '${methodName}' not found on interface '${interfaceName}'`);\n")
	sb.WriteString("    }\n\n")

	sb.WriteString("    const methodFunc = handler[methodName];\n\n")

	// Find method definition
	sb.WriteString("    // Find interface and method definition\n")
	sb.WriteString("    let methodDef: any = null;\n\n")
	writeInterfaceMethodLookupTs(sb, interfaces)
	sb.WriteString("    if (!methodDef) {\n")
	sb.WriteString("      return this.errorResponse(requestId, -32601, 'Method not found', `Method '${methodName}' not found in interface '${interfaceName}'`);\n")
	sb.WriteString("    }\n\n")

	// Validate params
	sb.WriteString("    // Validate params\n")
	sb.WriteString("    if (params === null || params === undefined) {\n")
	sb.WriteString("      params = [];\n")
	sb.WriteString("    }\n")
	sb.WriteString("    if (!Array.isArray(params)) {\n")
	sb.WriteString("      return this.errorResponse(requestId, -32602, 'Invalid params', 'params must be an array');\n")
	sb.WriteString("    }\n\n")

	sb.WriteString("    // Validate param count\n")
	sb.WriteString("    const expectedParams = methodDef.parameters || [];\n")
	sb.WriteString("    if (params.length !== expectedParams.length) {\n")
	sb.WriteString("      return this.errorResponse(requestId, -32602, 'Invalid params', `Expected ${expectedParams.length} parameters, got ${params.length}`);\n")
	sb.WriteString("    }\n\n")

	sb.WriteString("    // Validate each param\n")
	sb.WriteString("    for (let i = 0; i < params.length; i++) {\n")
	sb.WriteString("      try {\n")
	sb.WriteString("        validateType(params[i], expectedParams[i].type, ALL_STRUCTS, ALL_ENUMS, false);\n")
	sb.WriteString("      } catch (err: any) {\n")
	sb.WriteString("        return this.errorResponse(requestId, -32602, 'Invalid params', `Parameter ${i} (${expectedParams[i].name}) validation failed: ${err.message}`);\n")
	sb.WriteString("      }\n")
	sb.WriteString("    }\n\n")

	// Invoke handler
	sb.WriteString("    // Invoke handler\n")
	sb.WriteString("    let result: any;\n")
	sb.WriteString("    try {\n")
	sb.WriteString("      result = methodFunc.apply(handler, params);\n")
	sb.WriteString("    } catch (err: any) {\n")
	sb.WriteString("      if (err instanceof RPCError) {\n")
	sb.WriteString("        return this.errorResponse(requestId, err.code, err.message, err.data);\n")
	sb.WriteString("      }\n")
	sb.WriteString("      return this.errorResponse(requestId, -32603, 'Internal error', err.message || String(err));\n")
	sb.WriteString("    }\n\n")

	// Validate response
	sb.WriteString("    // Validate response\n")
	sb.WriteString("    const returnType = methodDef.returnType;\n")
	sb.WriteString("    const returnOptional = methodDef.returnOptional || false;\n")
	sb.WriteString("    if (returnType) {\n")
	sb.WriteString("      try {\n")
	sb.WriteString("        validateType(result, returnType, ALL_STRUCTS, ALL_ENUMS, returnOptional);\n")
	sb.WriteString("      } catch (err: any) {\n")
	sb.WriteString("        return this.errorResponse(requestId, -32603, 'Internal error', `Response validation failed: ${err.message}`);\n")
	sb.WriteString("      }\n")
	sb.WriteString("    }\n\n")

	sb.WriteString("    // Return success response\n")
	sb.WriteString("    if (isNotification) {\n")
	sb.WriteString("      return null;\n")
	sb.WriteString("    }\n")
	sb.WriteString("    return {\n")
	sb.WriteString("      jsonrpc: '2.0',\n")
	sb.WriteString("      result: result,\n")
	sb.WriteString("      id: requestId,\n")
	sb.WriteString("    };\n")
	sb.WriteString("  }\n\n")

	// errorResponse helper
	sb.WriteString("  private errorResponse(requestId: any, code: number, message: string, data?: any): any {\n")
	sb.WriteString("    const error: any = { code, message };\n")
	sb.WriteString("    if (data !== null && data !== undefined) {\n")
	sb.WriteString("      error.data = data;\n")
	sb.WriteString("    }\n")
	sb.WriteString("    return {\n")
	sb.WriteString("      jsonrpc: '2.0',\n")
	sb.WriteString("      error: error,\n")
	sb.WriteString("      id: requestId,\n")
	sb.WriteString("    };\n")
	sb.WriteString("  }\n\n")
}

// writeInterfaceMethodLookupTs generates code to find method definitions
func writeInterfaceMethodLookupTs(sb *strings.Builder, interfaces []*parser.Interface) {
	for i, iface := range interfaces {
		if i == 0 {
			fmt.Fprintf(sb, "    if (interfaceName === '%s') {\n", iface.Name)
		} else {
			fmt.Fprintf(sb, "    } else if (interfaceName === '%s') {\n", iface.Name)
		}
		sb.WriteString("      const interfaceMethods: any = {\n")
		for _, method := range iface.Methods {
			fmt.Fprintf(sb, "        '%s': {\n", method.Name)
			sb.WriteString("          parameters: [\n")
			for _, param := range method.Parameters {
				sb.WriteString("            {\n")
				fmt.Fprintf(sb, "              name: '%s',\n", param.Name)
				sb.WriteString("              type: ")
				writeTypeDictTs(sb, param.Type)
				sb.WriteString(",\n")
				sb.WriteString("            },\n")
			}
			sb.WriteString("          ],\n")
			sb.WriteString("          returnType: ")
			writeTypeDictTs(sb, method.ReturnType)
			sb.WriteString(",\n")
			if method.ReturnOptional {
				sb.WriteString("          returnOptional: true,\n")
			} else {
				sb.WriteString("          returnOptional: false,\n")
			}
			sb.WriteString("        },\n")
		}
		sb.WriteString("      };\n")
		sb.WriteString("      methodDef = interfaceMethods[methodName];\n")
	}
	sb.WriteString("    }\n\n")
}

// generateClientTs generates the client.ts file with transport abstraction and client classes
func generateClientTs(idl *parser.IDL, _ map[string]*parser.Struct, _ map[string]*parser.Enum, _ map[string]*parser.Interface, packagePrefix string, namespaceMap map[string]*NamespaceTypes) string {
	var sb strings.Builder

	sb.WriteString("// Generated by pulserpc - do not edit\n\n")
	sb.WriteString("/// <reference types=\"node\" />\n\n")
	sb.WriteString("import * as crypto from 'crypto';\n")
	sb.WriteString("import { RPCError } from './pulserpc/rpc';\n")

	// Import from namespace files
	namespaces := make([]string, 0, len(namespaceMap))
	for ns := range namespaceMap {
		if ns != "" {
			namespaces = append(namespaces, ns)
		}
	}
	// Sort namespaces for consistent output
	sort.Strings(namespaces)
	for _, ns := range namespaces {
		importPath := "./" + ns
		sb.WriteString(fmt.Sprintf("import { ALL_STRUCTS as %s_STRUCTS, ALL_ENUMS as %s_ENUMS } from '%s';\n", strings.ToUpper(ns), strings.ToUpper(ns), importPath))
	}
	sb.WriteString("\n")
	sb.WriteString("import { validateType } from './pulserpc/validation';\n\n")
	sb.WriteString("// Inline type definitions\n")
	sb.WriteString("interface TypeDef {\n")
	sb.WriteString("  builtIn?: string;\n")
	sb.WriteString("  array?: TypeDef;\n")
	sb.WriteString("  mapValue?: TypeDef;\n")
	sb.WriteString("  userDefined?: string;\n")
	sb.WriteString("}\n")
	sb.WriteString("interface StructDef {\n")
	sb.WriteString("  extends?: string;\n")
	sb.WriteString("  fields: Array<{ name: string; type: TypeDef; optional?: boolean }>;\n")
	sb.WriteString("}\n")
	sb.WriteString("interface EnumDef {\n")
	sb.WriteString("  values: Array<{ name: string }>;\n")
	sb.WriteString("}\n")
	sb.WriteString("type StructMap = { [key: string]: StructDef };\n")
	sb.WriteString("type EnumMap = { [key: string]: EnumDef };\n\n")
	sb.WriteString("// Merge ALL_STRUCTS and ALL_ENUMS from all namespaces\n")
	sb.WriteString("const ALL_STRUCTS: StructMap = {\n")
	// Merge structs from all namespaces
	for _, ns := range namespaces {
		sb.WriteString(fmt.Sprintf("  ...%s_STRUCTS,\n", strings.ToUpper(ns)))
	}
	sb.WriteString("};\n\n")

	sb.WriteString("const ALL_ENUMS: EnumMap = {\n")
	// Merge enums from all namespaces
	for _, ns := range namespaces {
		sb.WriteString(fmt.Sprintf("  ...%s_ENUMS,\n", strings.ToUpper(ns)))
	}
	sb.WriteString("};\n\n")

	// Generate Transport abstract class
	writeTransportAbstractTs(&sb, packagePrefix)

	// Generate HTTPTransport
	writeHTTPTransportTs(&sb, packagePrefix)

	// Generate client classes for each interface
	for _, iface := range idl.Interfaces {
		writeInterfaceClientTs(&sb, iface, idl.Interfaces, packagePrefix)
	}

	return sb.String()
}

// writeTransportAbstractTs generates the Transport abstract class
func writeTransportAbstractTs(sb *strings.Builder, packagePrefix string) {
	className := applyPackagePrefix("Transport", packagePrefix)
	fmt.Fprintf(sb, "export abstract class %s {\n", className)
	sb.WriteString("  /**\n")
	sb.WriteString("   * Perform a JSON-RPC 2.0 call and return the response.\n")
	sb.WriteString("   * @param method The method name in format 'interface.method'\n")
	sb.WriteString("   * @param params List of parameters to pass to the method\n")
	sb.WriteString("   * @returns Promise that resolves to the JSON-RPC 2.0 response\n")
	sb.WriteString("   * @throws RPCError If the JSON-RPC call returns an error\n")
	sb.WriteString("   */\n")
	sb.WriteString("  abstract call(method: string, params: any[]): Promise<any>;\n")
	sb.WriteString("}\n\n")
}

// writeHTTPTransportTs generates the HTTPTransport class
func writeHTTPTransportTs(sb *strings.Builder, packagePrefix string) {
	transportClassName := applyPackagePrefix("Transport", packagePrefix)
	className := applyPackagePrefix("HTTPTransport", packagePrefix)
	fmt.Fprintf(sb, "export class %s extends %s {\n", className, transportClassName)
	sb.WriteString("  private baseUrl: string;\n")
	sb.WriteString("  private headers: Record<string, string>;\n\n")

	sb.WriteString("  constructor(baseUrl: string, headers?: Record<string, string>) {\n")
	sb.WriteString("    super();\n")
	sb.WriteString("    this.baseUrl = baseUrl.replace(/\\/$/, '');\n")
	sb.WriteString("    this.headers = headers ? { ...headers } : {};\n")
	sb.WriteString("  }\n\n")

	sb.WriteString("  async call(method: string, params: any[]): Promise<any> {\n")
	sb.WriteString("    // Generate request ID\n")
	sb.WriteString("    const requestId = crypto.randomUUID();\n\n")

	sb.WriteString("    // Build JSON-RPC 2.0 request\n")
	sb.WriteString("    const requestData = {\n")
	sb.WriteString("      jsonrpc: '2.0',\n")
	sb.WriteString("      method: method,\n")
	sb.WriteString("      params: params,\n")
	sb.WriteString("      id: requestId,\n")
	sb.WriteString("    };\n\n")

	sb.WriteString("    // Prepare fetch options\n")
	sb.WriteString("    const headers: Record<string, string> = {\n")
	sb.WriteString("      'Content-Type': 'application/json',\n")
	sb.WriteString("      ...this.headers,\n")
	sb.WriteString("    };\n\n")

	sb.WriteString("    try {\n")
	sb.WriteString("      // Send request using native fetch (Node.js 18+)\n")
	sb.WriteString("      const response = await fetch(this.baseUrl, {\n")
	sb.WriteString("        method: 'POST',\n")
	sb.WriteString("        headers: headers,\n")
	sb.WriteString("        body: JSON.stringify(requestData),\n")
	sb.WriteString("      });\n\n")

	sb.WriteString("      const responseBody = await response.text();\n")
	sb.WriteString("      let responseData: any;\n")
	sb.WriteString("      try {\n")
	sb.WriteString("        responseData = JSON.parse(responseBody);\n")
	sb.WriteString("      } catch (err) {\n")
	sb.WriteString("        throw new RPCError(-32700, 'Parse error', `Invalid JSON response: ${err}`);\n")
	sb.WriteString("      }\n\n")

	sb.WriteString("      // Check for JSON-RPC error\n")
	sb.WriteString("      if (responseData.error) {\n")
	sb.WriteString("        const error = responseData.error;\n")
	sb.WriteString("        const code = error.code || -32603;\n")
	sb.WriteString("        const message = error.message || 'Internal error';\n")
	sb.WriteString("        const data = error.data;\n")
	sb.WriteString("        throw new RPCError(code, message, data);\n")
	sb.WriteString("      }\n\n")

	sb.WriteString("      // Return response\n")
	sb.WriteString("      return responseData;\n")
	sb.WriteString("    } catch (err: any) {\n")
	sb.WriteString("      if (err instanceof RPCError) {\n")
	sb.WriteString("        throw err;\n")
	sb.WriteString("      }\n")
	sb.WriteString("      throw new RPCError(-32603, `Network error: ${err.message || String(err)}`, undefined);\n")
	sb.WriteString("    }\n")
	sb.WriteString("  }\n")
	sb.WriteString("}\n\n")
}

// writeInterfaceClientTs generates a client class for an interface
func writeInterfaceClientTs(sb *strings.Builder, iface *parser.Interface, _ []*parser.Interface, packagePrefix string) {
	if iface.Comment != "" {
		lines := strings.Split(strings.TrimSpace(iface.Comment), "\n")
		for _, line := range lines {
			fmt.Fprintf(sb, "// %s\n", line)
		}
	}

	transportClassName := applyPackagePrefix("Transport", packagePrefix)
	clientClassName := applyPackagePrefix(iface.Name+"Client", packagePrefix)
	fmt.Fprintf(sb, "export class %s {\n", clientClassName)
	sb.WriteString("  private transport: " + transportClassName + ";\n")
	sb.WriteString("  private methodDefs: any;\n\n")

	fmt.Fprintf(sb, "  constructor(transport: %s) {\n", transportClassName)
	sb.WriteString("    this.transport = transport;\n")
	sb.WriteString("    // Method definitions for validation\n")
	sb.WriteString("    this.methodDefs = {\n")
	for _, method := range iface.Methods {
		fmt.Fprintf(sb, "      '%s': {\n", method.Name)
		sb.WriteString("        parameters: [\n")
		for _, param := range method.Parameters {
			sb.WriteString("          {\n")
			fmt.Fprintf(sb, "            name: '%s',\n", param.Name)
			sb.WriteString("            type: ")
			writeTypeDictTs(sb, param.Type)
			sb.WriteString(",\n")
			sb.WriteString("          },\n")
		}
		sb.WriteString("        ],\n")
		sb.WriteString("        returnType: ")
		writeTypeDictTs(sb, method.ReturnType)
		sb.WriteString(",\n")
		if method.ReturnOptional {
			sb.WriteString("        returnOptional: true,\n")
		} else {
			sb.WriteString("        returnOptional: false,\n")
		}
		sb.WriteString("      },\n")
	}
	sb.WriteString("    };\n")
	sb.WriteString("  }\n\n")

	// Generate methods
	for _, method := range iface.Methods {
		writeClientMethodTs(sb, iface, method)
	}
	sb.WriteString("}\n\n")
}

// writeClientMethodTs generates a method implementation for a client class
func writeClientMethodTs(sb *strings.Builder, iface *parser.Interface, method *parser.Method) {
	// Method signature
	fmt.Fprintf(sb, "  async %s(", method.Name)
	for i, param := range method.Parameters {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(sb, "%s: any", param.Name)
	}
	sb.WriteString("): Promise<any> {\n")

	// Get method definition
	fmt.Fprintf(sb, "    const methodDef = this.methodDefs['%s'];\n", method.Name)
	sb.WriteString("    const params: any[] = [\n")
	for _, param := range method.Parameters {
		fmt.Fprintf(sb, "      %s,\n", param.Name)
	}
	sb.WriteString("    ];\n\n")

	// Validate parameters
	sb.WriteString("    // Validate parameters\n")
	sb.WriteString("    const expectedParams = methodDef.parameters || [];\n")
	sb.WriteString("    for (let i = 0; i < params.length; i++) {\n")
	sb.WriteString("      try {\n")
	sb.WriteString("        validateType(params[i], expectedParams[i].type, ALL_STRUCTS, ALL_ENUMS, false);\n")
	sb.WriteString("      } catch (err: any) {\n")
	sb.WriteString("        throw new Error(`Parameter ${i} (${expectedParams[i].name}) validation failed: ${err.message}`);\n")
	sb.WriteString("      }\n")
	sb.WriteString("    }\n\n")

	// Call transport
	fmt.Fprintf(sb, "    // Call transport\n")
	fmt.Fprintf(sb, "    const methodName = '%s.%s';\n", iface.Name, method.Name)
	sb.WriteString("    const response = await this.transport.call(methodName, params);\n\n")

	// Extract result
	sb.WriteString("    // Extract result from JSON-RPC response\n")
	sb.WriteString("    if (response.error) {\n")
	sb.WriteString("      const error = response.error;\n")
	sb.WriteString("      const code = error.code || -32603;\n")
	sb.WriteString("      const message = error.message || 'Internal error';\n")
	sb.WriteString("      const data = error.data;\n")
	sb.WriteString("      throw new RPCError(code, message, data);\n")
	sb.WriteString("    }\n\n")
	sb.WriteString("    const result = response.result;\n\n")

	// Validate result
	sb.WriteString("    // Validate result\n")
	sb.WriteString("    const returnType = methodDef.returnType;\n")
	sb.WriteString("    const returnOptional = methodDef.returnOptional || false;\n")
	sb.WriteString("    if (returnType) {\n")
	sb.WriteString("      try {\n")
	sb.WriteString("        validateType(result, returnType, ALL_STRUCTS, ALL_ENUMS, returnOptional);\n")
	sb.WriteString("      } catch (err: any) {\n")
	sb.WriteString("        throw new Error(`Response validation failed: ${err.message}`);\n")
	sb.WriteString("      }\n")
	sb.WriteString("    }\n\n")

	// Return result
	sb.WriteString("    return result;\n")
	sb.WriteString("  }\n\n")
}

// generateTestServerTs generates test_server.ts with concrete implementations of all interfaces
func generateTestServerTs(idl *parser.IDL, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, _ map[string]*parser.Interface, packagePrefix string, _ map[string]*NamespaceTypes) string {
	var sb strings.Builder

	sb.WriteString("// Generated by barrister - do not edit\n")
	sb.WriteString("// Test server implementation for integration testing\n\n")
	serverClassName := applyPackagePrefix("PulseRPCServer", packagePrefix)
	fmt.Fprintf(&sb, "import { %s", serverClassName)
	for _, iface := range idl.Interfaces {
		className := applyPackagePrefix(iface.Name, packagePrefix)
		fmt.Fprintf(&sb, ", %s", className)
	}
	fmt.Fprintf(&sb, " } from './server';\n")
	sb.WriteString("\n")

	// Generate implementation classes for each interface
	for _, iface := range idl.Interfaces {
		writeTestInterfaceImplTs(&sb, iface, structMap, enumMap, packagePrefix)
	}

	// Generate main entry point
	sb.WriteString("// Main entry point\n")
	fmt.Fprintf(&sb, "const server = new %s('0.0.0.0', 8080);\n", serverClassName)
	for _, iface := range idl.Interfaces {
		implName := applyPackagePrefix(iface.Name+"Impl", packagePrefix)
		fmt.Fprintf(&sb, "server.register('%s', new %s());\n", iface.Name, implName)
	}
	sb.WriteString("server.serveForever();\n")

	return sb.String()
}

// writeTestInterfaceImplTs generates a test implementation class for an interface
func writeTestInterfaceImplTs(sb *strings.Builder, iface *parser.Interface, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, packagePrefix string) {
	baseClassName := applyPackagePrefix(iface.Name, packagePrefix)
	implName := applyPackagePrefix(iface.Name+"Impl", packagePrefix)
	fmt.Fprintf(sb, "class %s extends %s {\n", implName, baseClassName)
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
				if strings.Contains(baseName, ".") {
					parts := strings.Split(baseName, ".")
					baseName = parts[len(parts)-1]
				}
				if baseStruct := structMap[baseName]; baseStruct != nil {
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
		} else if enumMap[t.UserDefined] != nil {
			e := enumMap[t.UserDefined]
			if len(e.Values) > 0 {
				fmt.Fprintf(sb, "'%s'", e.Values[0].Name)
			} else {
				sb.WriteString("null")
			}
		} else {
			sb.WriteString("null")
		}
	} else {
		sb.WriteString("null")
	}
}

// generateTestClientTs generates test_client.ts that exercises all client methods
func generateTestClientTs(idl *parser.IDL, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, _ map[string]*parser.Interface, packagePrefix string, _ map[string]*NamespaceTypes) string {
	var sb strings.Builder

	sb.WriteString("// Generated by barrister - do not edit\n")
	sb.WriteString("// Test client for integration testing\n\n")
	sb.WriteString("/// <reference types=\"node\" />\n\n")
	transportClassName := applyPackagePrefix("HTTPTransport", packagePrefix)
	fmt.Fprintf(&sb, "import { %s", transportClassName)
	for _, iface := range idl.Interfaces {
		clientName := applyPackagePrefix(iface.Name+"Client", packagePrefix)
		fmt.Fprintf(&sb, ", %s", clientName)
	}
	fmt.Fprintf(&sb, " } from './client';\n")
	sb.WriteString("import * as http from 'http';\n\n")

	// Generate wait for server function
	sb.WriteString("async function waitForServer(url: string, timeout: number = 10000): Promise<boolean> {\n")
	sb.WriteString("  const startTime = Date.now();\n")
	sb.WriteString("  let retryDelay = 200; // Start with 200ms delay, will increase exponentially\n")
	sb.WriteString("\n")
	sb.WriteString("  while (Date.now() - startTime < timeout) {\n")
	sb.WriteString("    try {\n")
	sb.WriteString("      const controller = new AbortController();\n")
	sb.WriteString("      const timeoutId = setTimeout(() => controller.abort(), 2000);\n")
	sb.WriteString("\n")
	sb.WriteString("      const response = await fetch(url, {\n")
	sb.WriteString("        method: 'POST',\n")
	sb.WriteString("        headers: { 'Content-Type': 'application/json' },\n")
	sb.WriteString("        body: '{\"jsonrpc\":\"2.0\",\"method\":\"pulserpc-idl\",\"id\":1}',\n")
	sb.WriteString("        signal: controller.signal,\n")
	sb.WriteString("      });\n")
	sb.WriteString("\n")
	sb.WriteString("      clearTimeout(timeoutId);\n")
	sb.WriteString("\n")
	sb.WriteString("      // Accept 2xx status codes as success\n")
	sb.WriteString("      if (response.ok) {\n")
	sb.WriteString("        return true;\n")
	sb.WriteString("      } else {\n")
	sb.WriteString("        // Server responded but with error status - fail fast\n")
	sb.WriteString("        console.error(`Server responded with status ${response.status}`);\n")
	sb.WriteString("        return false;\n")
	sb.WriteString("      }\n")
	sb.WriteString("    } catch (err: any) {\n")
	sb.WriteString("      // Check if this is a network/connection error (retry) or abort (stop)\n")
	sb.WriteString("      if (err.name === 'AbortError') {\n")
	sb.WriteString("        console.error('Request timed out, server may not be ready');\n")
	sb.WriteString("      } else if (err.code === 'ECONNREFUSED' || err.code === 'ENOTFOUND') {\n")
	sb.WriteString("        // Connection refused or host not found - server not ready yet\n")
	sb.WriteString("      } else {\n")
	sb.WriteString("        // Other errors - log and continue trying\n")
	sb.WriteString("        console.error(`Connection error: ${err.message}`);\n")
	sb.WriteString("      }\n")
	sb.WriteString("    }\n")
	sb.WriteString("\n")
	sb.WriteString("    // Wait before retrying with exponential backoff\n")
	sb.WriteString("    await new Promise(resolve => setTimeout(resolve, retryDelay));\n")
	sb.WriteString("    retryDelay = Math.min(retryDelay * 1.5, 1000); // Cap at 1 second\n")
	sb.WriteString("  }\n")
	sb.WriteString("\n")
	sb.WriteString("  return false;\n")
	sb.WriteString("}\n\n")

	// Generate main test function
	sb.WriteString("async function main() {\n")
	sb.WriteString("  const serverUrl = 'http://localhost:8080';\n\n")
	sb.WriteString("  // Wait for server to be ready (shell script already waited, but do a final check)\n")
	sb.WriteString("  if (!(await waitForServer(serverUrl, 10000))) {\n")
	sb.WriteString("    console.error('ERROR: Server did not become ready in time');\n")
	sb.WriteString("    process.exit(1);\n")
	sb.WriteString("  }\n\n")
	sb.WriteString("  console.log('Server is ready. Running tests...');\n")
	sb.WriteString("  console.log();\n\n")

	sb.WriteString("  // Create transport and clients\n")
	sb.WriteString(fmt.Sprintf("  const transport = new %s(serverUrl);\n", transportClassName))
	for _, iface := range idl.Interfaces {
		clientName := applyPackagePrefix(iface.Name+"Client", packagePrefix)
		clientVar := strings.ToLower(iface.Name) + "Client"
		fmt.Fprintf(&sb, "  const %s = new %s(transport);\n", clientVar, clientName)
	}
	sb.WriteString("\n")
	sb.WriteString("  const errors: string[] = [];\n\n")

	// Generate test cases for each method
	for _, iface := range idl.Interfaces {
		clientVar := strings.ToLower(iface.Name) + "Client"
		for _, method := range iface.Methods {
			writeTestClientCallTs(&sb, iface, method, clientVar, structMap, enumMap)
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

	// Generate method call
	if len(params) > 0 {
		fmt.Fprintf(sb, "    const result = await %s.%s(%s);\n", clientVar, method.Name, strings.Join(params, ", "))
	} else {
		fmt.Fprintf(sb, "    const result = await %s.%s();\n", clientVar, method.Name)
	}

	// Generate assertions based on method
	methodNameLower := strings.ToLower(method.Name)
	if iface.Name == "B" && method.Name == "echo" {
		sb.WriteString("    // Test normal return\n")
		sb.WriteString("    if (result !== 'test') {\n")
		sb.WriteString("      throw new Error(`Expected 'test', got ${result}`);\n")
		sb.WriteString("    }\n")
		sb.WriteString("    // Test null return\n")
		fmt.Fprintf(sb, "    const resultNull = await %s.echo('return-null');\n", clientVar)
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
				if strings.Contains(baseName, ".") {
					parts := strings.Split(baseName, ".")
					baseName = parts[len(parts)-1]
				}
				if baseStruct := structMap[baseName]; baseStruct != nil {
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

// escapeTypeScriptTemplateLiteral escapes a string for use as a TypeScript template literal
func escapeTypeScriptTemplateLiteral(s string) string {
	var sb strings.Builder
	sb.WriteString("`") // Start of TypeScript template literal
	for _, r := range s {
		switch r {
		case '`':
			sb.WriteString("\\`") // Escape backticks
		case '$':
			sb.WriteString("\\$") // Escape dollar signs (to avoid ${ interpretation)
		case '\\':
			sb.WriteString("\\\\") // Escape backslashes
		default:
			sb.WriteRune(r)
		}
	}
	sb.WriteString("`") // End of TypeScript template literal
	return sb.String()
}

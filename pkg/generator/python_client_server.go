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

// PythonClientServer is a plugin that generates Python HTTP server and client code from IDL
type PythonClientServer struct {
	usePydantic bool
}

// NewPythonClientServer creates a new PythonClientServer plugin instance
func NewPythonClientServer() *PythonClientServer {
	return &PythonClientServer{}
}

// Name returns the plugin identifier
func (p *PythonClientServer) Name() string {
	return "python-client-server"
}

// RegisterFlags registers CLI flags for this plugin
func (p *PythonClientServer) RegisterFlags(fs *flag.FlagSet) {
	fs.BoolVar(&p.usePydantic, "use-pydantic", false, "Generate Pydantic models for types")
}

// Generate generates Python HTTP server and client code from the parsed IDL
func (p *PythonClientServer) Generate(idl *parser.IDL, fs *flag.FlagSet) error {
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

	// Write IDL JSON file
	if err := writeIDLJSON(idl, outputDir, fs); err != nil {
		return fmt.Errorf("failed to write idl.json: %w", err)
	}

	// Group types by namespace
	namespaceMap := GroupTypesByNamespace(idl)

	// Generate one file per namespace
	for namespace, types := range namespaceMap {
		if namespace == "" {
			continue // Skip types without namespace (shouldn't happen with required namespaces)
		}
		namespaceCode := generateNamespacePy(namespace, types)
		namespacePath := filepath.Join(outputDir, namespace+".py")
		if err := os.WriteFile(namespacePath, []byte(namespaceCode), 0644); err != nil {
			return fmt.Errorf("failed to write %s.py: %w", namespace, err)
		}
		PrintFileCreated(namespacePath, fs)
	}

	// Marshal IDL to JSON for embedding in server code
	idlJSON, err := json.MarshalIndent(idl, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal IDL to JSON: %w", err)
	}

	// Generate server.py with embedded IDL
	serverCode := generateServerPy(idl, structMap, enumMap, interfaceMap, namespaceMap, outputDir, string(idlJSON))
	serverPath := filepath.Join(outputDir, "server.py")
	if err := os.WriteFile(serverPath, []byte(serverCode), 0644); err != nil {
		return fmt.Errorf("failed to write server.py: %w", err)
	}
	PrintFileCreated(serverPath, fs)

	// Generate client.py
	clientCode := generateClientPy(idl, structMap, enumMap, interfaceMap, namespaceMap, outputDir)
	clientPath := filepath.Join(outputDir, "client.py")
	if err := os.WriteFile(clientPath, []byte(clientCode), 0644); err != nil {
		return fmt.Errorf("failed to write client.py: %w", err)
	}
	PrintFileCreated(clientPath, fs)

	// Generate Pydantic models if --use-pydantic flag is set
	if p.usePydantic {
		modelsCode := generateModelsPy(idl, namespaceMap)
		modelsPath := filepath.Join(outputDir, "models.py")
		if err := os.WriteFile(modelsPath, []byte(modelsCode), 0644); err != nil {
			return fmt.Errorf("failed to write models.py: %w", err)
		}
		PrintFileCreated(modelsPath, fs)
	}

	// Check if generate-test-files flag is set
	generateTestFilesFlag := fs.Lookup("generate-test-files")
	generateTestServer := generateTestFilesFlag != nil && generateTestFilesFlag.Value.String() == "true"

	// Generate test server and client if flag is set
	if generateTestServer {
		// Generate test_server.py
		testServerCode := generateTestServerPy(idl, structMap, enumMap, interfaceMap, namespaceMap, outputDir)
		testServerPath := filepath.Join(outputDir, "test_server.py")
		if err := os.WriteFile(testServerPath, []byte(testServerCode), 0644); err != nil {
			return fmt.Errorf("failed to write test_server.py: %w", err)
		}
		PrintFileCreated(testServerPath, fs)

		// Generate test_client.py
		testClientCode := generateTestClientPy(idl, structMap, enumMap, interfaceMap, namespaceMap, outputDir)
		testClientPath := filepath.Join(outputDir, "test_client.py")
		if err := os.WriteFile(testClientPath, []byte(testClientCode), 0644); err != nil {
			return fmt.Errorf("failed to write test_client.py: %w", err)
		}
		PrintFileCreated(testClientPath, fs)
	}

	return nil
}

// copyRuntimeFiles copies the Python runtime library files to the output directory
// Uses embedded runtime files from the binary
func (p *PythonClientServer) copyRuntimeFiles(outputDir string, silent bool) error {
	return runtime.CopyRuntimeFiles("python", outputDir, silent)
}

// writeIDLJSON writes the IDL metadata as JSON to idl.json
func writeIDLJSON(idl *parser.IDL, outputDir string, fs *flag.FlagSet) error {
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

// generateNamespacePy generates a Python file for a single namespace
func generateNamespacePy(namespace string, types *NamespaceTypes) string {
	var sb strings.Builder

	sb.WriteString("# Generated by pulserpc - do not edit\n\n")
	sb.WriteString("from pulserpc import (\n")
	sb.WriteString("    RPCError,\n")
	sb.WriteString("    validate_type,\n")
	sb.WriteString("    validate_struct,\n")
	sb.WriteString("    validate_enum,\n")
	sb.WriteString("    find_struct,\n")
	sb.WriteString("    find_enum,\n")
	sb.WriteString("    get_struct_fields,\n")
	sb.WriteString(")\n\n")

	// Generate IDL-specific type definitions for this namespace
	sb.WriteString(fmt.Sprintf("# IDL-specific type definitions for namespace: %s\n", namespace))
	sb.WriteString("ALL_STRUCTS = {\n")
	for _, s := range types.Structs {
		sb.WriteString(fmt.Sprintf("    '%s': {\n", s.Name))
		if s.Extends != "" {
			sb.WriteString(fmt.Sprintf("        'extends': '%s',\n", s.Extends))
		}
		sb.WriteString("        'fields': [\n")
		for _, field := range s.Fields {
			sb.WriteString("            {\n")
			sb.WriteString(fmt.Sprintf("                'name': '%s',\n", field.Name))
			sb.WriteString("                'type': ")
			writeTypeDictWithNamespace(&sb, field.Type, namespace, types)
			sb.WriteString(",\n")
			if field.Optional {
				sb.WriteString("                'optional': True,\n")
			}
			sb.WriteString("            },\n")
		}
		sb.WriteString("        ],\n")
		sb.WriteString("    },\n")
	}
	sb.WriteString("}\n\n")

	sb.WriteString("ALL_ENUMS = {\n")
	for _, e := range types.Enums {
		sb.WriteString(fmt.Sprintf("    '%s': {\n", e.Name))
		sb.WriteString("        'values': [\n")
		for _, val := range e.Values {
			sb.WriteString(fmt.Sprintf("            {'name': '%s'},\n", val.Name))
		}
		sb.WriteString("        ],\n")
		sb.WriteString("    },\n")
	}
	sb.WriteString("}\n")

	return sb.String()
}

// writeTypeDictWithNamespace writes a type definition as a Python dict, with namespace context
// It qualifies user-defined types from the same namespace with the namespace prefix
func writeTypeDictWithNamespace(sb *strings.Builder, t *parser.Type, currentNamespace string, types *NamespaceTypes) {
	sb.WriteString("{")
	if t.IsBuiltIn() {
		fmt.Fprintf(sb, "'builtIn': '%s'", t.BuiltIn)
	} else if t.IsArray() {
		sb.WriteString("'array': ")
		writeTypeDictWithNamespace(sb, t.Array, currentNamespace, types)
	} else if t.IsMap() {
		sb.WriteString("'mapValue': ")
		writeTypeDictWithNamespace(sb, t.MapValue, currentNamespace, types)
	} else if t.IsUserDefined() {
		// Determine if the type is from the current namespace
		typeName := t.UserDefined
		if currentNamespace != "" && types != nil && !strings.Contains(typeName, ".") {
			// Check if this type is defined in the current namespace
			isFromCurrentNamespace := false
			for _, s := range types.Structs {
				if GetBaseName(s.Name) == typeName {
					isFromCurrentNamespace = true
					break
				}
			}
			if !isFromCurrentNamespace {
				for _, e := range types.Enums {
					if GetBaseName(e.Name) == typeName {
						isFromCurrentNamespace = true
						break
					}
				}
			}
			// If it's from the current namespace, qualify it
			if isFromCurrentNamespace {
				typeName = currentNamespace + "." + typeName
			}
		}
		fmt.Fprintf(sb, "'userDefined': '%s'", typeName)
	}
	sb.WriteString("}")
}

// generateServerPy generates the server.py file with HTTP server and interface stubs
func generateServerPy(idl *parser.IDL, _ map[string]*parser.Struct, _ map[string]*parser.Enum, _ map[string]*parser.Interface, namespaceMap map[string]*NamespaceTypes, _outputDir string, _ string) string {
	var sb strings.Builder

	sb.WriteString("# Generated by pulserpc - do not edit\n\n")
	sb.WriteString("import abc\n")
	sb.WriteString("import json\n")
	sb.WriteString("from http.server import HTTPServer, BaseHTTPRequestHandler\n")
	sb.WriteString("from typing import Any, Dict, Optional\n\n")
	sb.WriteString("from pulserpc import Server, RPCError\n")
	sb.WriteString("\n")

	// Import from namespace modules
	namespaces := make([]string, 0, len(namespaceMap))
	for ns := range namespaceMap {
		if ns != "" {
			namespaces = append(namespaces, ns)
		}
	}
	// Sort namespaces for consistent output
	sort.Strings(namespaces)

	// All files are in the same directory (outputDir), so use direct imports
	for _, ns := range namespaces {
		sb.WriteString(fmt.Sprintf("from %s import ALL_STRUCTS as %s_STRUCTS, ALL_ENUMS as %s_ENUMS\n", ns, strings.ToUpper(ns), strings.ToUpper(ns)))
	}
	sb.WriteString("\n")

	// Merge ALL_STRUCTS and ALL_ENUMS from all namespaces
	sb.WriteString("# Merge ALL_STRUCTS and ALL_ENUMS from all namespaces\n")
	sb.WriteString("ALL_STRUCTS = {}\n")
	for _, ns := range namespaces {
		sb.WriteString(fmt.Sprintf("ALL_STRUCTS.update(%s_STRUCTS)\n", strings.ToUpper(ns)))
	}
	sb.WriteString("\n")
	sb.WriteString("ALL_ENUMS = {}\n")
	for _, ns := range namespaces {
		sb.WriteString(fmt.Sprintf("ALL_ENUMS.update(%s_ENUMS)\n", strings.ToUpper(ns)))
	}
	sb.WriteString("\n")

	// Load IDL metadata from idl.json
	sb.WriteString("# Load IDL metadata from idl.json\n")
	sb.WriteString("with open('idl.json', 'r') as f:\n")
	sb.WriteString("    IDL_DATA = json.load(f)\n")
	sb.WriteString("\n")

	// Create Server instance
	sb.WriteString("# Create Server instance with validation enabled\n")
	sb.WriteString("server = Server(validate_requests=True, validate_responses=True)\n")
	sb.WriteString("server.load_idl(IDL_DATA, ALL_STRUCTS, ALL_ENUMS)\n")
	sb.WriteString("\n")

	// Generate interface stub classes
	for _, iface := range idl.Interfaces {
		writeInterfaceStub(&sb, iface)
	}

	// Generate PulseRPCServer class - now just a wrapper around Server
	sb.WriteString("class PulseRPCServer:\n")
	sb.WriteString("    \"\"\"HTTP server for JSON-RPC 2.0 requests using Python's built-in http.server\"\"\"\n\n")
	sb.WriteString("    def __init__(self, host: str = 'localhost', port: int = 8080):\n")
	sb.WriteString("        self.host = host\n")
	sb.WriteString("        self.port = port\n")
	sb.WriteString("        self._http_server: Optional[HTTPServer] = None\n\n")

	sb.WriteString("    def register(self, interface_name: str, instance: Any) -> None:\n")
	sb.WriteString("        \"\"\"Register an interface implementation instance\"\"\"\n")
	sb.WriteString("        server.add_handler(interface_name, instance)\n\n")

	sb.WriteString("    def _create_handler_class(self):\n")
	sb.WriteString("        class PulseRPCHandler(BaseHTTPRequestHandler):\n")
	sb.WriteString("            def do_POST(self):\n")
	sb.WriteString("                # Read request body\n")
	sb.WriteString("                content_length = int(self.headers.get('Content-Length', 0))\n")
	sb.WriteString("                if content_length == 0:\n")
	sb.WriteString("                    self._send_error_response(None, -32700, \"Parse error\", \"Empty request body\")\n")
	sb.WriteString("                    return\n\n")
	sb.WriteString("                body = self.rfile.read(content_length)\n")
	sb.WriteString("                \n")
	sb.WriteString("                # Parse JSON request\n")
	sb.WriteString("                try:\n")
	sb.WriteString("                    req = json.loads(body.decode('utf-8'))\n")
	sb.WriteString("                except json.JSONDecodeError as e:\n")
	sb.WriteString("                    self._send_error_response(None, -32700, \"Parse error\", f\"Invalid JSON: {e}\")\n")
	sb.WriteString("                    return\n\n")
	sb.WriteString("                # Handle batch requests\n")
	sb.WriteString("                if isinstance(req, list):\n")
	sb.WriteString("                    if len(req) == 0:\n")
	sb.WriteString("                        self._send_error_response(None, -32600, \"Invalid Request\", \"Empty batch array\")\n")
	sb.WriteString("                        return\n")
	sb.WriteString("                    responses = []\n")
	sb.WriteString("                    for r in req:\n")
	sb.WriteString("                        response = server.call(r)\n")
	sb.WriteString("                        if response is not None:\n")
	sb.WriteString("                            responses.append(response)\n")
	sb.WriteString("                    if len(responses) == 0:\n")
	sb.WriteString("                        self._send_response(204, b'')\n")
	sb.WriteString("                    else:\n")
	sb.WriteString("                        self._send_json_response(200, responses)\n")
	sb.WriteString("                else:\n")
	sb.WriteString("                    # Single request\n")
	sb.WriteString("                    response = server.call(req)\n")
	sb.WriteString("                    if response is None:\n")
	sb.WriteString("                        self._send_response(204, b'')\n")
	sb.WriteString("                    else:\n")
	sb.WriteString("                        self._send_json_response(200, response)\n\n")

	sb.WriteString("            def _send_json_response(self, status: int, data: Any) -> None:\n")
	sb.WriteString("                \"\"\"Send a JSON response\"\"\"\n")
	sb.WriteString("                response_body = json.dumps(data).encode('utf-8')\n")
	sb.WriteString("                self.send_response(status)\n")
	sb.WriteString("                self.send_header('Content-Type', 'application/json')\n")
	sb.WriteString("                self.send_header('Content-Length', str(len(response_body)))\n")
	sb.WriteString("                self.end_headers()\n")
	sb.WriteString("                self.wfile.write(response_body)\n\n")

	sb.WriteString("            def _send_response(self, status: int, body: bytes) -> None:\n")
	sb.WriteString("                \"\"\"Send a response with raw body\"\"\"\n")
	sb.WriteString("                self.send_response(status)\n")
	sb.WriteString("                if len(body) > 0:\n")
	sb.WriteString("                    self.send_header('Content-Length', str(len(body)))\n")
	sb.WriteString("                self.end_headers()\n")
	sb.WriteString("                if len(body) > 0:\n")
	sb.WriteString("                    self.wfile.write(body)\n\n")

	sb.WriteString("            def _send_error_response(self, request_id: Any, code: int, message: str, data: Any = None) -> None:\n")
	sb.WriteString("                \"\"\"Send a JSON-RPC 2.0 error response\"\"\"\n")
	sb.WriteString("                error = {'code': code, 'message': message}\n")
	sb.WriteString("                if data is not None:\n")
	sb.WriteString("                    error['data'] = data\n")
	sb.WriteString("                response = {'jsonrpc': '2.0', 'error': error, 'id': request_id}\n")
	sb.WriteString("                self._send_json_response(200, response)\n\n")

	sb.WriteString("            def log_message(self, format: str, *args: Any) -> None:\n")
	sb.WriteString("                \"\"\"Override to customize logging if needed\"\"\"\n")
	sb.WriteString("                # Suppress default logging\n")
	sb.WriteString("                pass\n\n")

	sb.WriteString("        return PulseRPCHandler\n\n")

	sb.WriteString("    def serve_forever(self) -> None:\n")
	sb.WriteString("        \"\"\"Start the HTTP server and serve forever\"\"\"\n")
	sb.WriteString("        handler_class = self._create_handler_class()\n")
	sb.WriteString("        self._http_server = HTTPServer((self.host, self.port), handler_class)\n")
	sb.WriteString("        print(f\"PulseRPC server listening on http://{self.host}:{self.port}\")\n")
	sb.WriteString("        self._http_server.serve_forever()\n\n")

	sb.WriteString("    def shutdown(self) -> None:\n")
	sb.WriteString("        \"\"\"Shutdown the HTTP server\"\"\"\n")
	sb.WriteString("        if self._http_server:\n")
	sb.WriteString("            self._http_server.shutdown()\n\n")

	sb.WriteString("def main():\n")
	sb.WriteString("    \"\"\"Main entry point for running the server\"\"\"\n")
	sb.WriteString("    httpd = HTTPServer(('localhost', 8080), PulseRPCServer()._create_handler_class())\n")
	sb.WriteString("    print(\"PulseRPC server listening on http://localhost:8080\")\n")
	sb.WriteString("    httpd.serve_forever()\n\n")

	sb.WriteString("if __name__ == '__main__':\n")
	sb.WriteString("    main()\n")

	return sb.String()
}

func generateClientPy(idl *parser.IDL, _ map[string]*parser.Struct, _ map[string]*parser.Enum, _ map[string]*parser.Interface, namespaceMap map[string]*NamespaceTypes, _outputDir string) string {
	var sb strings.Builder

	sb.WriteString("# Generated by pulserpc - do not edit\n\n")
	sb.WriteString("from typing import Dict, Any\n\n")
	sb.WriteString("from pulserpc import Client, HttpTransport, RPCError\n")
	sb.WriteString("\n")

	// Import from namespace modules
	namespaces := make([]string, 0, len(namespaceMap))
	for ns := range namespaceMap {
		if ns != "" {
			namespaces = append(namespaces, ns)
		}
	}
	// Sort namespaces for consistent output
	sort.Strings(namespaces)

	// All files are in the same directory (outputDir), so use direct imports
	for _, ns := range namespaces {
		sb.WriteString(fmt.Sprintf("from %s import ALL_STRUCTS as %s_STRUCTS, ALL_ENUMS as %s_ENUMS\n", ns, strings.ToUpper(ns), strings.ToUpper(ns)))
	}
	sb.WriteString("\n")

	// Merge ALL_STRUCTS and ALL_ENUMS from all namespaces
	sb.WriteString("# Merge ALL_STRUCTS and ALL_ENUMS from all namespaces\n")
	sb.WriteString("ALL_STRUCTS = {}\n")
	for _, ns := range namespaces {
		sb.WriteString(fmt.Sprintf("ALL_STRUCTS.update(%s_STRUCTS)\n", strings.ToUpper(ns)))
	}
	sb.WriteString("\n")
	sb.WriteString("ALL_ENUMS = {}\n")
	for _, ns := range namespaces {
		sb.WriteString(fmt.Sprintf("ALL_ENUMS.update(%s_ENUMS)\n", strings.ToUpper(ns)))
	}
	sb.WriteString("\n")

	// Generate client classes for each interface
	for _, iface := range idl.Interfaces {
		writeInterfaceClient(&sb, iface, idl.Interfaces)
	}

	// Generate convenience client class that provides access to all interfaces
	sb.WriteString("class PulseRPCClient:\n")
	sb.WriteString("    \"\"\"Convenience client that provides access to all interfaces\"\"\"\n\n")
	sb.WriteString("    def __init__(self, url: str = \"http://localhost:8080\"):\n")
	sb.WriteString("        \"\"\"Initialize client with server URL\n\n")
	sb.WriteString("        Args:\n")
	sb.WriteString("            url: Server URL (default: http://localhost:8080)\n")
	sb.WriteString("        \"\"\"\n")
	sb.WriteString("        transport = HttpTransport(url)\n")
	sb.WriteString("        self._client = Client(transport)\n")
	sb.WriteString("        self._interfaces: Dict[str, Any] = {\n")
	for _, iface := range idl.Interfaces {
		fmt.Fprintf(&sb, "            '%s': %sClient(self._client),\n", iface.Name, iface.Name)
	}
	sb.WriteString("        }\n\n")

	for _, iface := range idl.Interfaces {
		fmt.Fprintf(&sb, "    @property\n")
		fmt.Fprintf(&sb, "    def %s(self) -> '%sClient':\n", iface.Name, iface.Name)
		fmt.Fprintf(&sb, "        \"\"\"Get client for %s interface\"\"\"\n", iface.Name)
		fmt.Fprintf(&sb, "        return self._interfaces['%s']\n\n", iface.Name)
	}

	return sb.String()
}


// writeInterfaceClient generates a client class for an interface
func writeInterfaceClient(sb *strings.Builder, iface *parser.Interface, _ []*parser.Interface) {
	// Write interface comment if present
	if iface.Comment != "" {
		lines := strings.Split(strings.TrimSpace(iface.Comment), "\n")
		for _, line := range lines {
			fmt.Fprintf(sb, "# %s\n", line)
		}
	}

	clientClassName := iface.Name + "Client"
	fmt.Fprintf(sb, "class %s:\n", clientClassName)
	if iface.Comment != "" {
		fmt.Fprintf(sb, "    \"\"\"Client for %s interface.\n\n", iface.Name)
		fmt.Fprintf(sb, "    %s\n", escapePythonDocstring(strings.TrimSpace(iface.Comment)))
		sb.WriteString("    \"\"\"\n\n")
	} else {
		fmt.Fprintf(sb, "    \"\"\"Client for %s interface.\"\"\"\n\n", iface.Name)
	}

	sb.WriteString("    def __init__(self, client: Client):\n")
	sb.WriteString("        \"\"\"Initialize client with a Client instance.\n\n")
	sb.WriteString("        Args:\n")
	sb.WriteString("            client: Client instance to use for RPC calls\n")
	sb.WriteString("        \"\"\"\n")
	sb.WriteString("        self._client = client\n")
	sb.WriteString("        self._iface_name = '")
	sb.WriteString(iface.Name)
	sb.WriteString("'\n\n")

	// Generate methods
	for _, method := range iface.Methods {
		writeClientMethod(sb, iface, method)
	}
	sb.WriteString("\n")
}

// writeClientMethod generates a method implementation for a client class
func writeClientMethod(sb *strings.Builder, iface *parser.Interface, method *parser.Method) {
	// Method signature
	fmt.Fprintf(sb, "    def %s(self", method.Name)
	for _, param := range method.Parameters {
		fmt.Fprintf(sb, ", %s", param.Name)
	}
	sb.WriteString("):\n")

	// Method docstring
	if len(method.Parameters) > 0 {
		sb.WriteString("        \"\"\"Call ")
		fmt.Fprintf(sb, "%s.%s", iface.Name, method.Name)
		sb.WriteString(".\n\n")
		sb.WriteString("        Args:\n")
		for _, param := range method.Parameters {
			fmt.Fprintf(sb, "            %s: Parameter %s\n", param.Name, param.Name)
		}
		sb.WriteString("\n        Returns:\n")
		sb.WriteString("            The method return value\n\n")
		sb.WriteString("        Raises:\n")
		sb.WriteString("            RPCError: If the RPC call fails\n")
		sb.WriteString("        \"\"\"\n")
	} else {
		sb.WriteString("        \"\"\"Call ")
		fmt.Fprintf(sb, "%s.%s", iface.Name, method.Name)
		sb.WriteString(".\n\n")
		sb.WriteString("        Returns:\n")
		sb.WriteString("            The method return value\n\n")
		sb.WriteString("        Raises:\n")
		sb.WriteString("            RPCError: If the RPC call fails\n")
		sb.WriteString("        \"\"\"\n")
	}

	// Build params dict
	if len(method.Parameters) > 0 {
		sb.WriteString("        params = {\n")
		for _, param := range method.Parameters {
			fmt.Fprintf(sb, "            '%s': %s,\n", param.Name, param.Name)
		}
		sb.WriteString("        }\n\n")
	} else {
		sb.WriteString("        params = {}\n\n")
	}

	// Call client
	fmt.Fprintf(sb, "        # Call %s.%s\n", iface.Name, method.Name)
	fmt.Fprintf(sb, "        method_name = '%s.%s'\n", iface.Name, method.Name)
	sb.WriteString("        return self._client.call(method_name, params)\n\n")
}

// generateModelsPy generates Pydantic models for structs and enums
func generateModelsPy(_ *parser.IDL, namespaceMap map[string]*NamespaceTypes) string {
	var sb strings.Builder

	sb.WriteString("# Generated by pulserpc - do not edit\n\n")
	sb.WriteString("from pydantic import BaseModel, Field\n")
	sb.WriteString("from typing import Optional, List, Dict\n\n")

	// Generate models for each namespace
	for namespace, types := range namespaceMap {
		if namespace == "" {
			continue
		}

		sb.WriteString(fmt.Sprintf("# %s\n", namespace))
		sb.WriteString(fmt.Sprintf("class %sBaseModel(BaseModel):\n", namespace))
		sb.WriteString("    \"\"\"Base model for all types in this namespace\"\"\"\n\n")

		// Generate models for enums
		for _, enum := range types.Enums {
			writePydanticEnumModel(&sb, enum)
		}

		// Generate models for structs
		for _, structDef := range types.Structs {
			writePydanticStructModel(&sb, structDef)
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// writePydanticEnumModel generates a Pydantic model for an enum
func writePydanticEnumModel(sb *strings.Builder, enum *parser.Enum) {
	// Write comment if present
	if enum.Comment != "" {
		lines := strings.Split(strings.TrimSpace(enum.Comment), "\n")
		for _, line := range lines {
			fmt.Fprintf(sb, "# %s\n", line)
		}
	}

	fmt.Fprintf(sb, "class %s(BaseModel):\n", enum.Name)
	sb.WriteString("    \"\"\"Pydantic model for enum\n\n")
	sb.WriteString("    Values:\n")
	for _, value := range enum.Values {
		fmt.Fprintf(sb, "        %s: %s\n", value.Name, value.Name)
		if value.Comment != "" {
			fmt.Fprintf(sb, "            %s\n", strings.TrimSpace(value.Comment))
		}
	}
	sb.WriteString("    \"\"\"\n\n")

	// For enums, we use a string field with constraints
	sb.WriteString("    value: str\n\n")

	sb.WriteString("    class Config:\n")
	sb.WriteString("        json_schema_extra = {\n")
	sb.WriteString("            'examples': [\n")
	for i, value := range enum.Values {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(sb, "{'value': '%s'}", value.Name)
	}
	sb.WriteString("            ]\n")
	sb.WriteString("        }\n\n")
}

// writePydanticStructModel generates a Pydantic model for a struct
func writePydanticStructModel(sb *strings.Builder, structDef *parser.Struct) {
	// Write comment if present
	if structDef.Comment != "" {
		lines := strings.Split(strings.TrimSpace(structDef.Comment), "\n")
		for _, line := range lines {
			fmt.Fprintf(sb, "# %s\n", line)
		}
	}

	fmt.Fprintf(sb, "class %s(BaseModel):\n", structDef.Name)
	if structDef.Comment != "" {
		fmt.Fprintf(sb, "    \"\"\"%s\"\"\"\n", escapePythonDocstring(strings.TrimSpace(structDef.Comment)))
	} else {
		fmt.Fprintf(sb, "    \"\"\"Pydantic model for %s\"\"\"\n", structDef.Name)
	}
	sb.WriteString("\n")

	// Generate fields
	for _, field := range structDef.Fields {
		writePydanticField(sb, field)
	}

	sb.WriteString("\n")

	// Config class
	sb.WriteString("    class Config:\n")
	sb.WriteString("        json_encoders = {\n")
	sb.WriteString("            # Add custom encoders if needed\n")
	sb.WriteString("        }\n")
	sb.WriteString("        json_schema_extra = {\n")
	sb.WriteString("            'example': {\n")
	for i, field := range structDef.Fields {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(sb, "'%s': ", field.Name)
		writePydanticExampleValue(sb, field.Type)
	}
	sb.WriteString("\n")
	sb.WriteString("            }\n")
	sb.WriteString("        }\n\n")
}

// writePydanticField generates a Pydantic field
func writePydanticField(sb *strings.Builder, field *parser.Field) {
	// Write field comment if present
	if field.Comment != "" {
		lines := strings.Split(strings.TrimSpace(field.Comment), "\n")
		for _, line := range lines {
			fmt.Fprintf(sb, "    # %s\n", line)
		}
	}

	// Field name and type annotation
	fmt.Fprintf(sb, "    %s: ", field.Name)
	writePydanticTypeAnnotation(sb, field.Type, field.Optional)

	// Add Field() with description if comment exists
	if field.Comment != "" {
		fmt.Fprintf(sb, " = Field(description=\"%s\")", escapePythonDocstring(field.Comment))
	} else if !field.Optional {
		// Add ... for required fields
		sb.WriteString(" = ...")
	}

	sb.WriteString("\n")
}

// writePydanticTypeAnnotation writes a Pydantic type annotation
func writePydanticTypeAnnotation(sb *strings.Builder, t *parser.Type, optional bool) {
	if t.IsBuiltIn() {
		switch t.BuiltIn {
		case "string":
			if optional {
				sb.WriteString("Optional[str]")
			} else {
				sb.WriteString("str")
			}
		case "int":
			if optional {
				sb.WriteString("Optional[int]")
			} else {
				sb.WriteString("int")
			}
		case "float":
			if optional {
				sb.WriteString("Optional[float]")
			} else {
				sb.WriteString("float")
			}
		case "bool":
			if optional {
				sb.WriteString("Optional[bool]")
			} else {
				sb.WriteString("bool")
			}
		default:
			sb.WriteString("Any")
		}
	} else if t.IsArray() {
		sb.WriteString("List[")
		writePydanticTypeAnnotation(sb, t.Array, false)
		sb.WriteString("]")
		if optional {
			sb.WriteString(" = []")
		}
	} else if t.IsMap() {
		sb.WriteString("Dict[str, ")
		writePydanticTypeAnnotation(sb, t.MapValue, false)
		sb.WriteString("]")
		if optional {
			sb.WriteString(" = {}")
		}
	} else if t.IsUserDefined() {
		// User-defined type - just use the name
		typeName := t.UserDefined
		// Handle namespace qualified names
		if strings.Contains(typeName, ".") {
			parts := strings.Split(typeName, ".")
			typeName = parts[len(parts)-1]
		}
		if optional {
			fmt.Fprintf(sb, "Optional['%s']", typeName)
		} else {
			fmt.Fprintf(sb, "'%s'", typeName)
		}
	}
}

// writePydanticExampleValue writes an example value for a type
func writePydanticExampleValue(sb *strings.Builder, t *parser.Type) {
	if t.IsBuiltIn() {
		switch t.BuiltIn {
		case "string":
			sb.WriteString("'string'")
		case "int":
			sb.WriteString("0")
		case "float":
			sb.WriteString("0.0")
		case "bool":
			sb.WriteString("True")
		default:
			sb.WriteString("None")
		}
	} else if t.IsArray() {
		sb.WriteString("[")
		writePydanticExampleValue(sb, t.Array)
		sb.WriteString("]")
	} else if t.IsMap() {
		sb.WriteString("{}")
	} else if t.IsUserDefined() {
		sb.WriteString("{}")
	}
}

// writeInterfaceStub writes an abstract base class for an interface
func writeInterfaceStub(sb *strings.Builder, iface *parser.Interface) {
	if iface.Comment != "" {
		lines := strings.Split(strings.TrimSpace(iface.Comment), "\n")
		for _, line := range lines {
			fmt.Fprintf(sb, "# %s\n", line)
		}
	}
	fmt.Fprintf(sb, "class %s(abc.ABC):\n", iface.Name)
	if iface.Comment != "" {
		fmt.Fprintf(sb, "    \"\"\"%s\"\"\"\n", escapePythonDocstring(strings.TrimSpace(iface.Comment)))
	}
	sb.WriteString("\n")

	for _, method := range iface.Methods {
		sb.WriteString("    @abc.abstractmethod\n")
		fmt.Fprintf(sb, "    def %s(self", method.Name)
		for _, param := range method.Parameters {
			fmt.Fprintf(sb, ", %s", param.Name)
		}
		sb.WriteString("):\n")
		sb.WriteString("        pass\n\n")
	}
	sb.WriteString("\n")
}


// generateTestServerPy generates test_server.py with concrete implementations of all interfaces
func generateTestServerPy(idl *parser.IDL, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, _ map[string]*parser.Interface, _ map[string]*NamespaceTypes, _ string) string {
	var sb strings.Builder

	sb.WriteString("# Generated by pulserpc - do not edit\n")
	sb.WriteString("# Test server implementation for integration testing\n\n")
	sb.WriteString("import math\n")
	sb.WriteString("from server import PulseRPCServer\n")

	// Import interface stubs
	for _, iface := range idl.Interfaces {
		fmt.Fprintf(&sb, "from server import %s\n", iface.Name)
	}
	sb.WriteString("\n")

	// Generate implementation classes for each interface
	for _, iface := range idl.Interfaces {
		writeTestInterfaceImpl(&sb, iface, structMap, enumMap)
	}

	// Generate main entry point
	sb.WriteString("if __name__ == \"__main__\":\n")
	sb.WriteString("    server = PulseRPCServer(host=\"0.0.0.0\", port=8080)\n")
	for _, iface := range idl.Interfaces {
		implName := iface.Name + "Impl"
		fmt.Fprintf(&sb, "    server.register(\"%s\", %s())\n", iface.Name, implName)
	}
	sb.WriteString("    server.serve_forever()\n")

	return sb.String()
}

// writeTestInterfaceImpl generates a test implementation class for an interface
func writeTestInterfaceImpl(sb *strings.Builder, iface *parser.Interface, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	implName := iface.Name + "Impl"
	fmt.Fprintf(sb, "class %s(%s):\n", implName, iface.Name)
	sb.WriteString("    \"\"\"Test implementation of ")
	fmt.Fprintf(sb, "%s", iface.Name)
	sb.WriteString(" interface\"\"\"\n\n")

	sb.WriteString("    def __init__(self):\n")
	sb.WriteString("        pass\n\n")

	// Generate method implementations
	for _, method := range iface.Methods {
		writeTestMethodImpl(sb, iface, method, structMap, enumMap)
	}
	sb.WriteString("\n")
}

// writeTestMethodImpl generates a test implementation for a method
func writeTestMethodImpl(sb *strings.Builder, iface *parser.Interface, method *parser.Method, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	// Method signature
	fmt.Fprintf(sb, "    def %s(self", method.Name)
	for _, param := range method.Parameters {
		fmt.Fprintf(sb, ", %s", param.Name)
	}
	sb.WriteString("):\n")

	// Special handling for known test cases
	if iface.Name == "B" && method.Name == "echo" {
		sb.WriteString("        # Handle optional return: return None if s == \"return-null\"\n")
		sb.WriteString("        if s == \"return-null\":\n")
		sb.WriteString("            return None\n")
		sb.WriteString("        return s\n\n")
		return
	}

	// Generate based on method name patterns
	methodNameLower := strings.ToLower(method.Name)
	switch methodNameLower {
	case "add":
		sb.WriteString("        # returns a+b\n")
		sb.WriteString("        return a + b\n\n")
	case "sqrt":
		sb.WriteString("        # returns the square root of a\n")
		sb.WriteString("        return math.sqrt(a)\n\n")
	case "calc":
		sb.WriteString("        # performs the given operation against all the values in nums and returns the result\n")
		sb.WriteString("        if not nums:\n")
		sb.WriteString("            return 0.0\n")
		sb.WriteString("        if operation == \"add\":\n")
		sb.WriteString("            return sum(nums)\n")
		sb.WriteString("        elif operation == \"multiply\":\n")
		sb.WriteString("            result = 1.0\n")
		sb.WriteString("            for num in nums:\n")
		sb.WriteString("                result *= num\n")
		sb.WriteString("            return result\n")
		sb.WriteString("        else:\n")
		sb.WriteString("            return 0.0\n\n")
	case "repeat":
		sb.WriteString("        # Echos the req1.to_repeat string as a list, optionally forcing to_repeat to upper case\n")
		sb.WriteString("        # RepeatResponse.items should be a list of strings whose length is equal to req1.count\n")
		sb.WriteString("        text = req1.get('to_repeat', '')\n")
		sb.WriteString("        count = req1.get('count', 0)\n")
		sb.WriteString("        force_uppercase = req1.get('force_uppercase', False)\n")
		sb.WriteString("        \n")
		sb.WriteString("        if force_uppercase:\n")
		sb.WriteString("            text = text.upper()\n")
		sb.WriteString("        \n")
		sb.WriteString("        items = [text] * count\n")
		sb.WriteString("        \n")
		sb.WriteString("        return {\n")
		sb.WriteString("            'status': 'ok',\n")
		sb.WriteString("            'count': count,\n")
		sb.WriteString("            'items': items\n")
		sb.WriteString("        }\n\n")
	case "say_hi":
		sb.WriteString("        # returns a result with: hi=\"hi\" and status=\"ok\"\n")
		sb.WriteString("        return {\n")
		sb.WriteString("            'hi': 'hi'\n")
		sb.WriteString("        }\n\n")
	case "repeat_num":
		sb.WriteString("        # returns num as an array repeated 'count' number of times\n")
		sb.WriteString("        return [num] * count\n\n")
	case "putperson":
		sb.WriteString("        # simply returns p.personId\n")
		sb.WriteString("        # we use this to test the '[optional]' enforcement, as we invoke it with a null email\n")
		sb.WriteString("        if isinstance(p, dict):\n")
		sb.WriteString("            return p.get('personId', '')\n")
		sb.WriteString("        return getattr(p, 'personId', '')\n\n")
	default:
		// Default implementation: return appropriate type based on return type
		// If return type is optional, return None
		if method.ReturnOptional {
			sb.WriteString("        # Optional return type - return None\n")
			sb.WriteString("        return None\n\n")
		} else {
			writeDefaultTestReturn(sb, method.ReturnType, structMap, enumMap)
		}
	}
}

// writeDefaultTestReturn generates a default return value for a type
func writeDefaultTestReturn(sb *strings.Builder, returnType *parser.Type, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	if returnType.IsBuiltIn() {
		switch returnType.BuiltIn {
		case "string":
			sb.WriteString("        return \"\"\n\n")
		case "int":
			sb.WriteString("        return 0\n\n")
		case "float":
			sb.WriteString("        return 0.0\n\n")
		case "bool":
			sb.WriteString("        return False\n\n")
		default:
			sb.WriteString("        return None\n\n")
		}
	} else if returnType.IsArray() {
		sb.WriteString("        return []\n\n")
	} else if returnType.IsMap() {
		sb.WriteString("        return {}\n\n")
	} else if returnType.IsUserDefined() {
		// Check if it's a struct
		if structMap[returnType.UserDefined] != nil {
			s := structMap[returnType.UserDefined]
			sb.WriteString("        return {\n")
			// Handle inheritance - get all fields including parent
			// For now, just use the struct's direct fields
			for _, field := range s.Fields {
				if field.Optional {
					continue // Skip optional fields in default return
				}
				fmt.Fprintf(sb, "            '%s': ", field.Name)
				writeDefaultTestValue(sb, field.Type, structMap, enumMap)
				sb.WriteString(",\n")
			}
			// If extends, add parent fields
			if s.Extends != "" {
				// Extract base struct name (handle qualified names)
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
						fmt.Fprintf(sb, "            '%s': ", field.Name)
						writeDefaultTestValue(sb, field.Type, structMap, enumMap)
						sb.WriteString(",\n")
					}
				}
			}
			sb.WriteString("        }\n\n")
		} else if enumMap[returnType.UserDefined] != nil {
			// Return first enum value
			e := enumMap[returnType.UserDefined]
			if len(e.Values) > 0 {
				fmt.Fprintf(sb, "        return \"%s\"\n\n", e.Values[0].Name)
			} else {
				sb.WriteString("        return None\n\n")
			}
		} else {
			sb.WriteString("        return None\n\n")
		}
	} else {
		sb.WriteString("        return None\n\n")
	}
}

// writeDefaultTestValue generates a default value for a type (used in structs)
func writeDefaultTestValue(sb *strings.Builder, t *parser.Type, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	if t.IsBuiltIn() {
		switch t.BuiltIn {
		case "string":
			sb.WriteString("\"\"")
		case "int":
			sb.WriteString("0")
		case "float":
			sb.WriteString("0.0")
		case "bool":
			sb.WriteString("False")
		default:
			sb.WriteString("None")
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
				fmt.Fprintf(sb, "\"%s\"", e.Values[0].Name)
			} else {
				sb.WriteString("None")
			}
		} else {
			sb.WriteString("None")
		}
	} else {
		sb.WriteString("None")
	}
}

// generateTestClientPy generates test_client.py that exercises all client methods
func generateTestClientPy(idl *parser.IDL, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, _ map[string]*parser.Interface, _ map[string]*NamespaceTypes, _ string) string {
	var sb strings.Builder

	sb.WriteString("# Generated by pulserpc - do not edit\n")
	sb.WriteString("# Test client for integration testing\n\n")
	sb.WriteString("import sys\n")
	sb.WriteString("import time\n")
	sb.WriteString("import urllib.request\n")
	sb.WriteString("from pulserpc import HttpTransport, Client\n")
	sb.WriteString("\n")

	// Generate client imports
	for _, iface := range idl.Interfaces {
		clientName := iface.Name + "Client"
		fmt.Fprintf(&sb, "from client import %s\n", clientName)
	}
	sb.WriteString("\n")

	// Generate main test function
	sb.WriteString("def wait_for_server(url: str, timeout: int = 10) -> bool:\n")
	sb.WriteString("    \"\"\"Wait for server to be ready\"\"\"\n")
	sb.WriteString("    import urllib.error\n")
	sb.WriteString("    start_time = time.time()\n")
	sb.WriteString("    while time.time() - start_time < timeout:\n")
	sb.WriteString("        try:\n")
	sb.WriteString("            req = urllib.request.Request(url, method='POST')\n")
	sb.WriteString("            req.add_header('Content-Type', 'application/json')\n")
	sb.WriteString("            urllib.request.urlopen(req, data=b'{}', timeout=1)\n")
	sb.WriteString("            return True\n")
	sb.WriteString("        except (urllib.error.URLError, urllib.error.HTTPError):\n")
	sb.WriteString("            time.sleep(0.5)\n")
	sb.WriteString("    return False\n\n")

	sb.WriteString("def main():\n")
	sb.WriteString("    server_url = \"http://localhost:8080\"\n")
	sb.WriteString("    \n")
	sb.WriteString("    # Wait for server to be ready\n")
	sb.WriteString("    print(\"Waiting for server to be ready...\")\n")
	sb.WriteString("    if not wait_for_server(server_url, timeout=10):\n")
	sb.WriteString("        print(\"ERROR: Server did not become ready in time\")\n")
	sb.WriteString("        sys.exit(1)\n")
	sb.WriteString("    \n")
	sb.WriteString("    print(\"Server is ready. Running tests...\")\n")
	sb.WriteString("    print()\n")
	sb.WriteString("    \n")
	sb.WriteString("    # Create transport and clients\n")
	sb.WriteString("    transport = HttpTransport(server_url)\n")
	sb.WriteString("    client = Client(transport)\n")
	for _, iface := range idl.Interfaces {
		clientName := iface.Name + "Client"
		clientVar := strings.ToLower(iface.Name) + "_client"
		fmt.Fprintf(&sb, "    %s = %s(client)\n", clientVar, clientName)
	}
	sb.WriteString("    \n")
	sb.WriteString("    errors = []\n")
	sb.WriteString("    \n")

	// Generate test cases for each method
	for _, iface := range idl.Interfaces {
		clientVar := strings.ToLower(iface.Name) + "_client"
		for _, method := range iface.Methods {
			writeTestClientCall(&sb, iface, method, clientVar, structMap, enumMap)
		}
	}

	sb.WriteString("    # Report results\n")
	sb.WriteString("    print()\n")
	sb.WriteString("    if errors:\n")
	sb.WriteString("        print(f\"FAILED: {len(errors)} test(s) failed:\")\n")
	sb.WriteString("        for error in errors:\n")
	sb.WriteString("            print(f\"  - {error}\")\n")
	sb.WriteString("        sys.exit(1)\n")
	sb.WriteString("    else:\n")
	sb.WriteString("        print(\"SUCCESS: All tests passed!\")\n")
	sb.WriteString("        sys.exit(0)\n\n")

	sb.WriteString("if __name__ == \"__main__\":\n")
	sb.WriteString("    main()\n")

	return sb.String()
}

// writeTestClientCall generates a test call for a method
func writeTestClientCall(sb *strings.Builder, iface *parser.Interface, method *parser.Method, clientVar string, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) {
	testName := fmt.Sprintf("%s.%s", iface.Name, method.Name)
	fmt.Fprintf(sb, "    # Test %s\n", testName)
	sb.WriteString("    try:\n")

	// Generate test parameters based on method signature
	params := make([]string, 0)
	for _, param := range method.Parameters {
		paramValue := generateTestParamValue(param.Type, param.Name, structMap, enumMap)
		params = append(params, paramValue)
	}

	// Generate method call
	if len(params) > 0 {
		fmt.Fprintf(sb, "        result = %s.%s(%s)\n", clientVar, method.Name, strings.Join(params, ", "))
	} else {
		fmt.Fprintf(sb, "        result = %s.%s()\n", clientVar, method.Name)
	}

	// Generate assertions based on method
	methodNameLower := strings.ToLower(method.Name)
	if iface.Name == "B" && method.Name == "echo" {
		sb.WriteString("        # Test normal return\n")
		sb.WriteString("        assert result == \"test\", f\"Expected 'test', got {result}\"\n")
		sb.WriteString("        # Test null return\n")
		fmt.Fprintf(sb, "        result_null = %s.echo(\"return-null\")\n", clientVar)
		sb.WriteString("        assert result_null is None, f\"Expected None, got {result_null}\"\n")
	} else if methodNameLower == "add" {
		sb.WriteString("        assert result == 5, f\"Expected 5, got {result}\"\n")
	} else if methodNameLower == "sqrt" {
		sb.WriteString("        assert abs(result - 2.0) < 0.001, f\"Expected ~2.0, got {result}\"\n")
	} else if methodNameLower == "calc" {
		sb.WriteString("        assert isinstance(result, float), f\"Expected float, got {type(result)}\"\n")
	} else if methodNameLower == "repeat" {
		sb.WriteString("        assert isinstance(result, dict), f\"Expected dict, got {type(result)}\"\n")
		sb.WriteString("        assert 'items' in result, \"Result missing 'items' field\"\n")
		sb.WriteString("        assert len(result['items']) == 3, f\"Expected 3 items, got {len(result['items'])}\"\n")
	} else if methodNameLower == "say_hi" {
		sb.WriteString("        assert isinstance(result, dict), f\"Expected dict, got {type(result)}\"\n")
		sb.WriteString("        assert result.get('hi') == 'hi', f\"Expected hi='hi', got {result}\"\n")
	} else if methodNameLower == "repeat_num" {
		sb.WriteString("        assert isinstance(result, list), f\"Expected list, got {type(result)}\"\n")
		sb.WriteString("        assert len(result) == 2, f\"Expected 2 items, got {len(result)}\"\n")
	} else if methodNameLower == "putperson" {
		sb.WriteString("        assert isinstance(result, str), f\"Expected str, got {type(result)}\"\n")
		sb.WriteString("        assert result == \"person123\", f\"Expected 'person123', got {result}\"\n")
	} else if methodNameLower == "finditem" {
		// findItem has optional return type - it can return None
		sb.WriteString("        # findItem has optional return type\n")
		sb.WriteString("        assert result is None, f\"Expected None (optional return), got {result}\"\n")
	} else {
		// Generic assertion - just check that we got a result
		sb.WriteString("        assert result is not None, \"Expected non-None result\"\n")
	}

	fmt.Fprintf(sb, "        print(\"✓ %s passed\")\n", testName)
	sb.WriteString("    except Exception as e:\n")
	fmt.Fprintf(sb, "        error_msg = \"%s failed: {}\".format(str(e))\n", testName)
	sb.WriteString("        errors.append(error_msg)\n")
	sb.WriteString("        print(f\"✗ {error_msg}\")\n")
	sb.WriteString("    \n")
}

// generateTestParamValue generates a test parameter value for a type
func generateTestParamValue(t *parser.Type, paramName string, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum) string {
	if t.IsBuiltIn() {
		switch t.BuiltIn {
		case "string":
			// Special case for echo method
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
			return "True"
		default:
			return "None"
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
			// Build struct dict
			fields := []string{}
			for _, field := range s.Fields {
				if field.Optional && field.Name == "email" {
					// Special case: set email to None for putPerson test
					fields = append(fields, fmt.Sprintf("'%s': None", field.Name))
				} else if !field.Optional {
					fieldValue := generateTestParamValue(field.Type, field.Name, structMap, enumMap)
					fields = append(fields, fmt.Sprintf("'%s': %s", field.Name, fieldValue))
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
							fieldValue := generateTestParamValue(field.Type, field.Name, structMap, enumMap)
							fields = append(fields, fmt.Sprintf("'%s': %s", field.Name, fieldValue))
						}
					}
				}
			}
			// Special handling for RepeatRequest
			if t.UserDefined == "RepeatRequest" {
				return "{'to_repeat': 'hello', 'count': 3, 'force_uppercase': False}"
			}
			// Special handling for Person
			if t.UserDefined == "Person" {
				return "{'personId': 'person123', 'firstName': 'John', 'lastName': 'Doe', 'email': None}"
			}
			return "{" + strings.Join(fields, ", ") + "}"
		} else if enumMap[t.UserDefined] != nil {
			e := enumMap[t.UserDefined]
			if len(e.Values) > 0 {
				// Special case for MathOp
				if t.UserDefined == "inc.MathOp" || strings.HasSuffix(t.UserDefined, "MathOp") {
					return "\"add\""
				}
				return fmt.Sprintf("\"%s\"", e.Values[0].Name)
			}
			return "None"
		}
		return "None"
	}
	return "None"
}


// escapePythonDocstring escapes a string for use inside a Python triple-double-quoted docstring
// Escapes """ to \""" to avoid ending the docstring prematurely
func escapePythonDocstring(s string) string {
	// Replace """ with \"""
	return strings.ReplaceAll(s, `"""`, `\"""`)
}

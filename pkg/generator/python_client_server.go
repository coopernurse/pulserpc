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

	// Group types by namespace (used by Pydantic models generation)
	namespaceMap := GroupTypesByNamespace(idl)

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

// generateServerPy generates the server.py file with abstract service classes only
func generateServerPy(idl *parser.IDL, _ map[string]*parser.Struct, _ map[string]*parser.Enum, _ map[string]*parser.Interface, _ map[string]*NamespaceTypes, _outputDir string, _ string) string {
	var sb strings.Builder

	sb.WriteString("# Generated by pulserpc - do not edit\n\n")
	sb.WriteString("# Abstract service classes\n")
	sb.WriteString("# Implement these classes to create your service\n\n")
	sb.WriteString("import abc\n")
	sb.WriteString("from pulserpc import RPCError\n")
	sb.WriteString("\n")

	// Generate interface stub classes
	for _, iface := range idl.Interfaces {
		writeInterfaceStub(&sb, iface)
	}

	return sb.String()
}

func generateClientPy(idl *parser.IDL, _ map[string]*parser.Struct, _ map[string]*parser.Enum, _ map[string]*parser.Interface, _ map[string]*NamespaceTypes, _outputDir string) string {
	var sb strings.Builder

	sb.WriteString("# Generated by pulserpc - do not edit\n\n")
	sb.WriteString("# This file contains example code showing how to use the PulseRPC client\n")
	sb.WriteString("# The client automatically discovers interfaces from the server\n\n")
	sb.WriteString("from pulserpc import HttpTransport, Client\n\n")
	sb.WriteString("# Example: Create a client and call a method\n")
	sb.WriteString("# \n")
	sb.WriteString("# transport = HttpTransport(\"http://localhost:8080\")\n")
	sb.WriteString("# client = Client(transport)\n")

	// Generate example calls for each interface
	for _, iface := range idl.Interfaces {
		if len(iface.Methods) > 0 {
			method := iface.Methods[0]
			sb.WriteString("#\n")
			sb.WriteString(fmt.Sprintf("# result = client.%s.%s(", iface.Name, method.Name))
			if len(method.Parameters) > 0 {
				params := make([]string, 0, len(method.Parameters))
				for _, param := range method.Parameters {
					params = append(params, fmt.Sprintf("%s=...", param.Name))
				}
				sb.WriteString(strings.Join(params, ", "))
			}
			sb.WriteString(")\n")
		}
	}
	sb.WriteString("#\n")
	sb.WriteString("# The client automatically fetches the IDL from the server and\n")
	sb.WriteString("# makes interfaces available as attributes (e.g., client.CatalogService)\n")

	return sb.String()
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
	sb.WriteString("import json\n")
	sb.WriteString("import math\n")
	sb.WriteString("from http.server import HTTPServer, BaseHTTPRequestHandler\n")
	sb.WriteString("from typing import Any\n")
	sb.WriteString("from pulserpc import Server, Contract, RPCError\n\n")
	sb.WriteString("from server import *\n")

	// Generate implementation classes for each interface
	for _, iface := range idl.Interfaces {
		writeTestInterfaceImpl(&sb, iface, structMap, enumMap)
	}

	// Generate JSON-RPC handler
	sb.WriteString("\n")
	sb.WriteString("class TestRPCHandler(BaseHTTPRequestHandler):\n")
	sb.WriteString("    def do_POST(self):\n")
	sb.WriteString("        # Read request body\n")
	sb.WriteString("        content_length = int(self.headers.get('Content-Length', 0))\n")
	sb.WriteString("        if content_length == 0:\n")
	sb.WriteString("            self._send_error_response(None, -32700, \"Parse error\", \"Empty request body\")\n")
	sb.WriteString("            return\n\n")
	sb.WriteString("        body = self.rfile.read(content_length)\n\n")
	sb.WriteString("        # Parse JSON request\n")
	sb.WriteString("        try:\n")
	sb.WriteString("            req = json.loads(body.decode('utf-8'))\n")
	sb.WriteString("        except json.JSONDecodeError as e:\n")
	sb.WriteString("            self._send_error_response(None, -32700, \"Parse error\", f\"Invalid JSON: {e}\")\n")
	sb.WriteString("            return\n\n")
	sb.WriteString("        # Handle request\n")
	sb.WriteString("        response = rpc_server.call(req)\n")
	sb.WriteString("        if response is None:\n")
	sb.WriteString("            self._send_response(204, b'')\n")
	sb.WriteString("        else:\n")
	sb.WriteString("            self._send_json_response(200, response)\n\n")
	sb.WriteString("    def _send_json_response(self, status: int, data: Any) -> None:\n")
	sb.WriteString("        \"\"\"Send a JSON response\"\"\"\n")
	sb.WriteString("        response_body = json.dumps(data).encode('utf-8')\n")
	sb.WriteString("        self.send_response(status)\n")
	sb.WriteString("        self.send_header('Content-Type', 'application/json')\n")
	sb.WriteString("        self.send_header('Content-Length', str(len(response_body)))\n")
	sb.WriteString("        self.end_headers()\n")
	sb.WriteString("        self.wfile.write(response_body)\n\n")
	sb.WriteString("    def _send_response(self, status: int, body: bytes) -> None:\n")
	sb.WriteString("        \"\"\"Send a response with raw body\"\"\"\n")
	sb.WriteString("        self.send_response(status)\n")
	sb.WriteString("        if len(body) > 0:\n")
	sb.WriteString("            self.send_header('Content-Length', str(len(body)))\n")
	sb.WriteString("        self.end_headers()\n")
	sb.WriteString("        if len(body) > 0:\n")
	sb.WriteString("            self.wfile.write(body)\n\n")
	sb.WriteString("    def _send_error_response(self, request_id: Any, code: int, message: str, data: Any = None) -> None:\n")
	sb.WriteString("        \"\"\"Send a JSON-RPC 2.0 error response\"\"\"\n")
	sb.WriteString("        error = {'code': code, 'message': message}\n")
	sb.WriteString("        if data is not None:\n")
	sb.WriteString("            error['data'] = data\n")
	sb.WriteString("        response = {'jsonrpc': '2.0', 'error': error, 'id': request_id}\n")
	sb.WriteString("        self._send_json_response(200, response)\n\n")
	sb.WriteString("    def log_message(self, format: str, *args: Any) -> None:\n")
	sb.WriteString("        \"\"\"Suppress default logging\"\"\"\n")
	sb.WriteString("        pass\n\n")

	// Generate main entry point
	sb.WriteString("if __name__ == \"__main__\":\n")
	sb.WriteString("    # Load IDL and create Contract\n")
	sb.WriteString("    with open('idl.json', 'r') as f:\n")
	sb.WriteString("        idl_data = json.load(f)\n")
	sb.WriteString("    contract = Contract(idl_data)\n\n")
	sb.WriteString("    # Create Server instance\n")
	sb.WriteString("    rpc_server = Server(contract)\n")
	for _, iface := range idl.Interfaces {
		implName := iface.Name + "Impl"
		fmt.Fprintf(&sb, "    rpc_server.add_handler(\"%s\", %s())\n", iface.Name, implName)
	}
	sb.WriteString("\n")
	sb.WriteString("    # Start HTTP server\n")
	sb.WriteString("    http_server = HTTPServer(('0.0.0.0', 8080), TestRPCHandler)\n")
	sb.WriteString("    print('PulseRPC test server listening on http://0.0.0.0:8080')\n")
	sb.WriteString("    http_server.serve_forever()\n")

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
		// Extract base name to handle qualified names (e.g., "inc.Response" -> "Response")
		baseTypeName := GetBaseName(returnType.UserDefined)
		if structMap[baseTypeName] != nil {
			s := structMap[baseTypeName]
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
		} else if enumMap[baseTypeName] != nil {
			// Return first enum value
			e := enumMap[baseTypeName]
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
		// Extract base name to handle qualified names (e.g., "inc.Response" -> "Response")
		baseTypeName := GetBaseName(t.UserDefined)
		if structMap[baseTypeName] != nil {
			sb.WriteString("{}")
		} else if enumMap[baseTypeName] != nil {
			e := enumMap[baseTypeName]
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
	sb.WriteString("    # Create client - interfaces are auto-discovered\n")
	sb.WriteString("    transport = HttpTransport(server_url)\n")
	sb.WriteString("    client = Client(transport)\n")
	sb.WriteString("    \n")
	sb.WriteString("    errors = []\n")
	sb.WriteString("    \n")

	// Generate test cases for each method
	for _, iface := range idl.Interfaces {
		for _, method := range iface.Methods {
			writeTestClientCall(&sb, iface, method, "client", structMap, enumMap)
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

	// Generate method call - use interface name to access the interface proxy
	if len(params) > 0 {
		fmt.Fprintf(sb, "        result = %s.%s.%s(%s)\n", clientVar, iface.Name, method.Name, strings.Join(params, ", "))
	} else {
		fmt.Fprintf(sb, "        result = %s.%s.%s()\n", clientVar, iface.Name, method.Name)
	}

	// Generate assertions based on method
	methodNameLower := strings.ToLower(method.Name)
	if iface.Name == "B" && method.Name == "echo" {
		sb.WriteString("        # Test normal return\n")
		sb.WriteString("        assert result == \"test\", f\"Expected 'test', got {result}\"\n")
		sb.WriteString("        # Test null return\n")
		fmt.Fprintf(sb, "        result_null = %s.%s.echo(\"return-null\")\n", clientVar, iface.Name)
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
		// Check if it's a struct (use full qualified name for lookup)
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
			// Handle inheritance (use full qualified name for lookup)
			if s.Extends != "" {
				if baseStruct := structMap[s.Extends]; baseStruct != nil {
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

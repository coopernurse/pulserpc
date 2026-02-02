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
	// Only register base-dir if it hasn't been registered by another plugin
	if fs.Lookup("base-dir") == nil {
		fs.String("base-dir", "", "Base directory for namespace packages/modules (defaults to -dir if not specified)")
	}
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

	// Get base-dir flag (defaults to outputDir if not specified)
	baseDirFlag := fs.Lookup("base-dir")
	baseDir := outputDir
	if baseDirFlag != nil && baseDirFlag.Value.String() != "" {
		baseDir = baseDirFlag.Value.String()
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
		namespaceCode := generateNamespacePy(namespace, types)
		namespacePath := filepath.Join(baseDir, namespace+".py")
		if err := os.WriteFile(namespacePath, []byte(namespaceCode), 0644); err != nil {
			return fmt.Errorf("failed to write %s.py: %w", namespace, err)
		}
		PrintFileCreated(namespacePath, fs)
	}

	// Generate server.py
	serverCode := generateServerPy(idl, structMap, enumMap, interfaceMap, namespaceMap, baseDir, outputDir)
	serverPath := filepath.Join(outputDir, "server.py")
	if err := os.WriteFile(serverPath, []byte(serverCode), 0644); err != nil {
		return fmt.Errorf("failed to write server.py: %w", err)
	}
	PrintFileCreated(serverPath, fs)

	// Generate client.py
	clientCode := generateClientPy(idl, structMap, enumMap, interfaceMap, namespaceMap, baseDir, outputDir)
	clientPath := filepath.Join(outputDir, "client.py")
	if err := os.WriteFile(clientPath, []byte(clientCode), 0644); err != nil {
		return fmt.Errorf("failed to write client.py: %w", err)
	}
	PrintFileCreated(clientPath, fs)

	// Write IDL JSON document for pulserpc-idl RPC method
	jsonData, err := json.MarshalIndent(idl, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal IDL to JSON: %w", err)
	}
	jsonPath := filepath.Join(outputDir, "idl.json")
	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write idl.json: %w", err)
	}
	PrintFileCreated(jsonPath, fs)

	// Check if generate-test-files flag is set
	generateTestFilesFlag := fs.Lookup("generate-test-files")
	generateTestServer := generateTestFilesFlag != nil && generateTestFilesFlag.Value.String() == "true"

	// Generate test server and client if flag is set
	if generateTestServer {
		// Generate test_server.py
		testServerCode := generateTestServerPy(idl, structMap, enumMap, interfaceMap, namespaceMap, baseDir, outputDir)
		testServerPath := filepath.Join(outputDir, "test_server.py")
		if err := os.WriteFile(testServerPath, []byte(testServerCode), 0644); err != nil {
			return fmt.Errorf("failed to write test_server.py: %w", err)
		}
		PrintFileCreated(testServerPath, fs)

		// Generate test_client.py
		testClientCode := generateTestClientPy(idl, structMap, enumMap, interfaceMap, namespaceMap, baseDir, outputDir)
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
			writeTypeDict(&sb, field.Type)
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

// writeTypeDict writes a type definition as a Python dict
func writeTypeDict(sb *strings.Builder, t *parser.Type) {
	sb.WriteString("{")
	if t.IsBuiltIn() {
		fmt.Fprintf(sb, "'builtIn': '%s'", t.BuiltIn)
	} else if t.IsArray() {
		sb.WriteString("'array': ")
		writeTypeDict(sb, t.Array)
	} else if t.IsMap() {
		sb.WriteString("'mapValue': ")
		writeTypeDict(sb, t.MapValue)
	} else if t.IsUserDefined() {
		fmt.Fprintf(sb, "'userDefined': '%s'", t.UserDefined)
	}
	sb.WriteString("}")
}

// generateServerPy generates the server.py file with HTTP server and interface stubs
func generateServerPy(idl *parser.IDL, _ map[string]*parser.Struct, _ map[string]*parser.Enum, _ map[string]*parser.Interface, namespaceMap map[string]*NamespaceTypes, baseDir string, outputDir string) string {
	var sb strings.Builder

	sb.WriteString("# Generated by pulserpc - do not edit\n\n")
	sb.WriteString("import abc\n")
	sb.WriteString("import json\n")
	sb.WriteString("import os\n")
	sb.WriteString("import sys\n")
	sb.WriteString("from http.server import HTTPServer, BaseHTTPRequestHandler\n")
	sb.WriteString("from typing import Any, Dict, List, Optional\n")
	sb.WriteString("from pathlib import Path\n\n")
	sb.WriteString("from pulserpc import RPCError, validate_type\n")

	// Import from namespace modules
	namespaces := make([]string, 0, len(namespaceMap))
	for ns := range namespaceMap {
		if ns != "" {
			namespaces = append(namespaces, ns)
		}
	}
	// Sort namespaces for consistent output
	sort.Strings(namespaces)

	// Calculate relative path from outputDir to baseDir for imports
	if baseDir != outputDir {
		relPath, err := filepath.Rel(outputDir, baseDir)
		if err == nil && relPath != "." {
			// Use relative import path
			for _, ns := range namespaces {
				importPath := filepath.ToSlash(filepath.Join(relPath, ns))
				sb.WriteString(fmt.Sprintf("from %s import ALL_STRUCTS as %s_STRUCTS, ALL_ENUMS as %s_ENUMS\n", strings.ReplaceAll(importPath, "/", "."), strings.ToUpper(ns), strings.ToUpper(ns)))
			}
		} else {
			// Fallback: add to sys.path
			sb.WriteString(fmt.Sprintf("sys.path.insert(0, str(Path(__file__).parent / '%s'))\n", filepath.Base(baseDir)))
			for _, ns := range namespaces {
				sb.WriteString(fmt.Sprintf("from %s import ALL_STRUCTS as %s_STRUCTS, ALL_ENUMS as %s_ENUMS\n", ns, strings.ToUpper(ns), strings.ToUpper(ns)))
			}
		}
	} else {
		// Same directory - direct imports
		for _, ns := range namespaces {
			sb.WriteString(fmt.Sprintf("from %s import ALL_STRUCTS as %s_STRUCTS, ALL_ENUMS as %s_ENUMS\n", ns, strings.ToUpper(ns), strings.ToUpper(ns)))
		}
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

	// Generate interface stub classes
	for _, iface := range idl.Interfaces {
		writeInterfaceStub(&sb, iface)
	}

	// Generate PulseRPCServer class
	sb.WriteString("class PulseRPCServer:\n")
	sb.WriteString("    \"\"\"HTTP server for JSON-RPC 2.0 requests using Python's built-in http.server\"\"\"\n\n")
	sb.WriteString("    def __init__(self, host: str = 'localhost', port: int = 8080):\n")
	sb.WriteString("        self.host = host\n")
	sb.WriteString("        self.port = port\n")
	sb.WriteString("        self.handlers: Dict[str, Any] = {}\n")
	sb.WriteString("        self._server: Optional[HTTPServer] = None\n\n")

	sb.WriteString("    def register(self, interface_name: str, instance: Any) -> None:\n")
	sb.WriteString("        \"\"\"Register an interface implementation instance\"\"\"\n")
	sb.WriteString("        self.handlers[interface_name] = instance\n\n")

	// Generate handler class
	sb.WriteString("    def _create_handler_class(self):\n")
	sb.WriteString("        handlers = self.handlers\n")
	sb.WriteString("        server_instance = self\n\n")
	sb.WriteString("        class PulseRPCHandler(BaseHTTPRequestHandler):\n")
	sb.WriteString("            def do_POST(self):\n")
	sb.WriteString("                # Read request body\n")
	sb.WriteString("                content_length = int(self.headers.get('Content-Length', 0))\n")
	sb.WriteString("                if content_length == 0:\n")
	sb.WriteString("                    self._send_error_response(None, -32700, \"Parse error\", \"Empty request body\")\n")
	sb.WriteString("                    return\n\n")
	sb.WriteString("                body = self.rfile.read(content_length)\n")
	sb.WriteString("                \n")
	sb.WriteString("                try:\n")
	sb.WriteString("                    data = json.loads(body.decode('utf-8'))\n")
	sb.WriteString("                except (json.JSONDecodeError, UnicodeDecodeError) as e:\n")
	sb.WriteString("                    self._send_error_response(None, -32700, \"Parse error\", f\"Invalid JSON: {e}\")\n")
	sb.WriteString("                    return\n\n")
	sb.WriteString("                # Handle batch requests\n")
	sb.WriteString("                if isinstance(data, list):\n")
	sb.WriteString("                    if len(data) == 0:\n")
	sb.WriteString("                        self._send_error_response(None, -32600, \"Invalid Request\", \"Empty batch array\")\n")
	sb.WriteString("                        return\n")
	sb.WriteString("                    responses = []\n")
	sb.WriteString("                    for req in data:\n")
	sb.WriteString("                        response = server_instance.handle_request(req)\n")
	sb.WriteString("                        if response is not None:\n")
	sb.WriteString("                            responses.append(response)\n")
	sb.WriteString("                    if len(responses) == 0:\n")
	sb.WriteString("                        self._send_response(204, b'')\n")
	sb.WriteString("                    else:\n")
	sb.WriteString("                        self._send_json_response(200, responses)\n")
	sb.WriteString("                else:\n")
	sb.WriteString("                    response = server_instance.handle_request(data)\n")
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
	sb.WriteString("                # Suppress default logging, or customize as needed\n")
	sb.WriteString("                pass\n\n")

	sb.WriteString("        return PulseRPCHandler\n\n")

	sb.WriteString("    def handle_request(self, request_json: Dict[str, Any]) -> Optional[Dict[str, Any]]:\n")
	sb.WriteString("        \"\"\"Handle a single JSON-RPC 2.0 request\"\"\"\n")
	sb.WriteString("        # Validate JSON-RPC 2.0 structure\n")
	sb.WriteString("        if not isinstance(request_json, dict):\n")
	sb.WriteString("            return self._error_response(None, -32600, \"Invalid Request\", \"Request must be an object\")\n")
	sb.WriteString("        \n")
	sb.WriteString("        jsonrpc = request_json.get('jsonrpc')\n")
	sb.WriteString("        if jsonrpc != '2.0':\n")
	sb.WriteString("            return self._error_response(None, -32600, \"Invalid Request\", \"jsonrpc must be '2.0'\")\n")
	sb.WriteString("        \n")
	sb.WriteString("        method = request_json.get('method')\n")
	sb.WriteString("        if not isinstance(method, str):\n")
	sb.WriteString("            return self._error_response(None, -32600, \"Invalid Request\", \"method must be a string\")\n")
	sb.WriteString("        \n")
	sb.WriteString("        params = request_json.get('params')\n")
	sb.WriteString("        request_id = request_json.get('id')\n")
	sb.WriteString("        is_notification = 'id' not in request_json\n")
	sb.WriteString("        \n")
	sb.WriteString("        # Special case: pulserpc-idl method returns the IDL JSON document\n")
	sb.WriteString("        if method == \"pulserpc-idl\":\n")
	sb.WriteString("            try:\n")
	sb.WriteString("                # Get the directory where server.py is located\n")
	sb.WriteString("                server_dir = os.path.dirname(os.path.abspath(__file__))\n")
	sb.WriteString("                idl_json_path = os.path.join(server_dir, \"idl.json\")\n")
	sb.WriteString("                \n")
	sb.WriteString("                with open(idl_json_path, 'r', encoding='utf-8') as f:\n")
	sb.WriteString("                    idl_doc = json.load(f)\n")
	sb.WriteString("                \n")
	sb.WriteString("                # Return success response\n")
	sb.WriteString("                if is_notification:\n")
	sb.WriteString("                    return None\n")
	sb.WriteString("                return {\n")
	sb.WriteString("                    'jsonrpc': '2.0',\n")
	sb.WriteString("                    'result': idl_doc,\n")
	sb.WriteString("                    'id': request_id\n")
	sb.WriteString("                }\n")
	sb.WriteString("            except FileNotFoundError:\n")
	sb.WriteString("                return self._error_response(request_id, -32603, \"Internal error\", \"IDL JSON file not found\")\n")
	sb.WriteString("            except json.JSONDecodeError as e:\n")
	sb.WriteString("                return self._error_response(request_id, -32603, \"Internal error\", f\"Failed to parse IDL JSON: {e}\")\n")
	sb.WriteString("            except Exception as e:\n")
	sb.WriteString("                return self._error_response(request_id, -32603, \"Internal error\", f\"Failed to load IDL JSON: {e}\")\n")
	sb.WriteString("        \n")
	sb.WriteString("        # Parse method name: interface.method\n")
	sb.WriteString("        parts = method.split('.', 1)\n")
	sb.WriteString("        if len(parts) != 2:\n")
	sb.WriteString("            return self._error_response(request_id, -32601, \"Method not found\", f\"Invalid method format: {method}\")\n")
	sb.WriteString("        \n")
	sb.WriteString("        interface_name, method_name = parts\n")
	sb.WriteString("        \n")
	sb.WriteString("        # Find handler\n")
	sb.WriteString("        handler = self.handlers.get(interface_name)\n")
	sb.WriteString("        if handler is None:\n")
	sb.WriteString("            return self._error_response(request_id, -32601, \"Method not found\", f\"Interface '{interface_name}' not registered\")\n")
	sb.WriteString("        \n")
	sb.WriteString("        # Find method on handler\n")
	sb.WriteString("        if not hasattr(handler, method_name):\n")
	sb.WriteString("            return self._error_response(request_id, -32601, \"Method not found\", f\"Method '{method_name}' not found on interface '{interface_name}'\")\n")
	sb.WriteString("        \n")
	sb.WriteString("        method_func = getattr(handler, method_name)\n")
	sb.WriteString("        \n")
	sb.WriteString("        # Find interface and method definition\n")
	writeInterfaceMethodLookup(&sb, idl.Interfaces)
	sb.WriteString("        \n")
	sb.WriteString("        if method_def is None:\n")
	sb.WriteString("            return self._error_response(request_id, -32601, \"Method not found\", f\"Method '{method_name}' not found in interface '{interface_name}'\")\n")
	sb.WriteString("        \n")
	sb.WriteString("        # Validate params\n")
	sb.WriteString("        if params is None:\n")
	sb.WriteString("            params = []\n")
	sb.WriteString("        if not isinstance(params, list):\n")
	sb.WriteString("            return self._error_response(request_id, -32602, \"Invalid params\", \"params must be an array\")\n")
	sb.WriteString("        \n")
	sb.WriteString("        # Validate param count\n")
	sb.WriteString("        expected_params = method_def.get('parameters', [])\n")
	sb.WriteString("        if len(params) != len(expected_params):\n")
	sb.WriteString("            return self._error_response(request_id, -32602, \"Invalid params\", f\"Expected {len(expected_params)} parameters, got {len(params)}\")\n")
	sb.WriteString("        \n")
	sb.WriteString("        # Validate each param\n")
	sb.WriteString("        for i, (param_value, param_def) in enumerate(zip(params, expected_params)):\n")
	sb.WriteString("            try:\n")
	sb.WriteString("                validate_type(param_value, param_def['type'], ALL_STRUCTS, ALL_ENUMS, False)\n")
	sb.WriteString("            except Exception as e:\n")
	sb.WriteString("                return self._error_response(request_id, -32602, \"Invalid params\", f\"Parameter {i} ({param_def['name']}) validation failed: {e}\")\n")
	sb.WriteString("        \n")
	sb.WriteString("        # Invoke handler\n")
	sb.WriteString("        try:\n")
	sb.WriteString("            result = method_func(*params)\n")
	sb.WriteString("        except RPCError as e:\n")
	sb.WriteString("            return self._error_response(request_id, e.code, e.message, e.data)\n")
	sb.WriteString("        except Exception as e:\n")
	sb.WriteString("            return self._error_response(request_id, -32603, \"Internal error\", str(e))\n")
	sb.WriteString("        \n")
	sb.WriteString("        # Validate response\n")
	sb.WriteString("        return_type = method_def.get('returnType')\n")
	sb.WriteString("        return_optional = method_def.get('returnOptional', False)\n")
	sb.WriteString("        if return_type:\n")
	sb.WriteString("            try:\n")
	sb.WriteString("                validate_type(result, return_type, ALL_STRUCTS, ALL_ENUMS, return_optional)\n")
	sb.WriteString("            except Exception as e:\n")
	sb.WriteString("                return self._error_response(request_id, -32603, \"Internal error\", f\"Response validation failed: {e}\")\n")
	sb.WriteString("        \n")
	sb.WriteString("        # Return success response\n")
	sb.WriteString("        if is_notification:\n")
	sb.WriteString("            return None\n")
	sb.WriteString("        return {\n")
	sb.WriteString("            'jsonrpc': '2.0',\n")
	sb.WriteString("            'result': result,\n")
	sb.WriteString("            'id': request_id\n")
	sb.WriteString("        }\n\n")

	sb.WriteString("    def _error_response(self, request_id: Any, code: int, message: str, data: Any = None) -> Dict[str, Any]:\n")
	sb.WriteString("        \"\"\"Create a JSON-RPC 2.0 error response\"\"\"\n")
	sb.WriteString("        error = {\n")
	sb.WriteString("            'code': code,\n")
	sb.WriteString("            'message': message\n")
	sb.WriteString("        }\n")
	sb.WriteString("        if data is not None:\n")
	sb.WriteString("            error['data'] = data\n")
	sb.WriteString("        return {\n")
	sb.WriteString("            'jsonrpc': '2.0',\n")
	sb.WriteString("            'error': error,\n")
	sb.WriteString("            'id': request_id\n")
	sb.WriteString("        }\n\n")

	sb.WriteString("    def serve_forever(self) -> None:\n")
	sb.WriteString("        \"\"\"Start the HTTP server and serve forever\"\"\"\n")
	sb.WriteString("        handler_class = self._create_handler_class()\n")
	sb.WriteString("        self._server = HTTPServer((self.host, self.port), handler_class)\n")
	sb.WriteString("        print(f\"PulseRPC server listening on http://{self.host}:{self.port}\")\n")
	sb.WriteString("        self._server.serve_forever()\n\n")

	sb.WriteString("    def shutdown(self) -> None:\n")
	sb.WriteString("        \"\"\"Shutdown the HTTP server\"\"\"\n")
	sb.WriteString("        if self._server:\n")
	sb.WriteString("            self._server.shutdown()\n")

	return sb.String()
}

// generateClientPy generates the client.py file with transport abstraction and client classes
func generateClientPy(idl *parser.IDL, _ map[string]*parser.Struct, _ map[string]*parser.Enum, _ map[string]*parser.Interface, namespaceMap map[string]*NamespaceTypes, baseDir string, outputDir string) string {
	var sb strings.Builder

	sb.WriteString("# Generated by pulserpc - do not edit\n\n")
	sb.WriteString("from abc import ABC, abstractmethod\n")
	sb.WriteString("from typing import Dict, Any, Optional, List\n")
	sb.WriteString("import json\n")
	sb.WriteString("import sys\n")
	sb.WriteString("import urllib.request\n")
	sb.WriteString("import urllib.error\n")
	sb.WriteString("import uuid\n")
	sb.WriteString("from pathlib import Path\n\n")
	sb.WriteString("from pulserpc import RPCError, validate_type\n")

	// Import from namespace modules
	namespaces := make([]string, 0, len(namespaceMap))
	for ns := range namespaceMap {
		if ns != "" {
			namespaces = append(namespaces, ns)
		}
	}
	// Sort namespaces for consistent output
	sort.Strings(namespaces)

	// Calculate relative path from outputDir to baseDir for imports
	if baseDir != outputDir {
		relPath, err := filepath.Rel(outputDir, baseDir)
		if err == nil && relPath != "." {
			// Use relative import path
			for _, ns := range namespaces {
				importPath := filepath.ToSlash(filepath.Join(relPath, ns))
				sb.WriteString(fmt.Sprintf("from %s import ALL_STRUCTS as %s_STRUCTS, ALL_ENUMS as %s_ENUMS\n", strings.ReplaceAll(importPath, "/", "."), strings.ToUpper(ns), strings.ToUpper(ns)))
			}
		} else {
			// Fallback: add to sys.path
			sb.WriteString(fmt.Sprintf("sys.path.insert(0, str(Path(__file__).parent / '%s'))\n", filepath.Base(baseDir)))
			for _, ns := range namespaces {
				sb.WriteString(fmt.Sprintf("from %s import ALL_STRUCTS as %s_STRUCTS, ALL_ENUMS as %s_ENUMS\n", ns, strings.ToUpper(ns), strings.ToUpper(ns)))
			}
		}
	} else {
		// Same directory - direct imports
		for _, ns := range namespaces {
			sb.WriteString(fmt.Sprintf("from %s import ALL_STRUCTS as %s_STRUCTS, ALL_ENUMS as %s_ENUMS\n", ns, strings.ToUpper(ns), strings.ToUpper(ns)))
		}
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

	// Generate Transport ABC
	writeTransportABC(&sb)

	// Generate HTTPTransport
	writeHTTPTransport(&sb)

	// Generate client classes for each interface
	for _, iface := range idl.Interfaces {
		writeInterfaceClient(&sb, iface, idl.Interfaces)
	}

	return sb.String()
}

// writeTransportABC generates the Transport abstract base class
func writeTransportABC(sb *strings.Builder) {
	sb.WriteString("class Transport(ABC):\n")
	sb.WriteString("    \"\"\"Abstract base class for transport implementations.\n")
	sb.WriteString("    \n")
	sb.WriteString("    Transports handle the roundtrip of sending requests to the server\n")
	sb.WriteString("    and decoding responses. Different transports can use different\n")
	sb.WriteString("    protocols (HTTP, ZeroMQ, etc.) and serialization formats (JSON, MessagePack, etc.).\n")
	sb.WriteString("    \"\"\"\n\n")
	sb.WriteString("    @abstractmethod\n")
	sb.WriteString("    def call(self, method: str, params: list) -> dict:\n")
	sb.WriteString("        \"\"\"Perform a JSON-RPC 2.0 call and return the response.\n")
	sb.WriteString("        \n")
	sb.WriteString("        Args:\n")
	sb.WriteString("            method: The method name in format 'interface.method'\n")
	sb.WriteString("            params: List of parameters to pass to the method\n")
	sb.WriteString("        \n")
	sb.WriteString("        Returns:\n")
	sb.WriteString("            dict: The JSON-RPC 2.0 response dictionary\n")
	sb.WriteString("        \n")
	sb.WriteString("        Raises:\n")
	sb.WriteString("            RPCError: If the JSON-RPC call returns an error\n")
	sb.WriteString("            Exception: For transport-level errors (network, etc.)\n")
	sb.WriteString("        \"\"\"\n")
	sb.WriteString("        pass\n\n\n")
}

// writeHTTPTransport generates the HTTPTransport class
func writeHTTPTransport(sb *strings.Builder) {
	sb.WriteString("class HTTPTransport(Transport):\n")
	sb.WriteString("    \"\"\"HTTP transport implementation using JSON-RPC 2.0 over HTTP.\n")
	sb.WriteString("    \n")
	sb.WriteString("    Uses Python's standard library urllib.request for HTTP requests.\n")
	sb.WriteString("    Supports configurable headers for authentication and other purposes.\n")
	sb.WriteString("    \"\"\"\n\n")
	sb.WriteString("    def __init__(self, base_url: str, headers: Optional[Dict[str, str]] = None):\n")
	sb.WriteString("        \"\"\"Initialize HTTP transport.\n")
	sb.WriteString("        \n")
	sb.WriteString("        Args:\n")
	sb.WriteString("            base_url: Base URL of the server (e.g., 'http://localhost:8080')\n")
	sb.WriteString("            headers: Optional dictionary of HTTP headers to include with each request\n")
	sb.WriteString("        \"\"\"\n")
	sb.WriteString("        self.base_url = base_url.rstrip('/')\n")
	sb.WriteString("        self.headers = headers.copy() if headers else {}\n\n")
	sb.WriteString("    def call(self, method: str, params: list) -> dict:\n")
	sb.WriteString("        \"\"\"Perform a JSON-RPC 2.0 call over HTTP.\n")
	sb.WriteString("        \n")
	sb.WriteString("        Args:\n")
	sb.WriteString("            method: The method name in format 'interface.method'\n")
	sb.WriteString("            params: List of parameters to pass to the method\n")
	sb.WriteString("        \n")
	sb.WriteString("        Returns:\n")
	sb.WriteString("            dict: The JSON-RPC 2.0 response dictionary\n")
	sb.WriteString("        \n")
	sb.WriteString("        Raises:\n")
	sb.WriteString("            RPCError: If the JSON-RPC call returns an error\n")
	sb.WriteString("            urllib.error.HTTPError: For HTTP errors\n")
	sb.WriteString("            urllib.error.URLError: For network errors\n")
	sb.WriteString("        \"\"\"\n")
	sb.WriteString("        # Generate request ID\n")
	sb.WriteString("        request_id = str(uuid.uuid4())\n\n")
	sb.WriteString("        # Build JSON-RPC 2.0 request\n")
	sb.WriteString("        request_data = {\n")
	sb.WriteString("            'jsonrpc': '2.0',\n")
	sb.WriteString("            'method': method,\n")
	sb.WriteString("            'params': params,\n")
	sb.WriteString("            'id': request_id\n")
	sb.WriteString("        }\n\n")
	sb.WriteString("        # Serialize to JSON\n")
	sb.WriteString("        json_data = json.dumps(request_data).encode('utf-8')\n\n")
	sb.WriteString("        # Prepare request\n")
	sb.WriteString("        req = urllib.request.Request(self.base_url, data=json_data, method='POST')\n")
	sb.WriteString("        req.add_header('Content-Type', 'application/json')\n")
	sb.WriteString("        req.add_header('Content-Length', str(len(json_data)))\n\n")
	sb.WriteString("        # Add custom headers\n")
	sb.WriteString("        for key, value in self.headers.items():\n")
	sb.WriteString("            req.add_header(key, value)\n\n")
	sb.WriteString("        try:\n")
	sb.WriteString("            # Send request\n")
	sb.WriteString("            with urllib.request.urlopen(req) as response:\n")
	sb.WriteString("                response_body = response.read().decode('utf-8')\n")
	sb.WriteString("                response_data = json.loads(response_body)\n\n")
	sb.WriteString("                # Check for JSON-RPC error\n")
	sb.WriteString("                if 'error' in response_data:\n")
	sb.WriteString("                    error = response_data['error']\n")
	sb.WriteString("                    code = error.get('code', -32603)\n")
	sb.WriteString("                    message = error.get('message', 'Internal error')\n")
	sb.WriteString("                    data = error.get('data')\n")
	sb.WriteString("                    raise RPCError(code, message, data)\n\n")
	sb.WriteString("                # Return response\n")
	sb.WriteString("                return response_data\n\n")
	sb.WriteString("        except urllib.error.HTTPError as e:\n")
	sb.WriteString("            # Try to parse error response as JSON-RPC\n")
	sb.WriteString("            try:\n")
	sb.WriteString("                error_body = e.read().decode('utf-8')\n")
	sb.WriteString("                error_data = json.loads(error_body)\n")
	sb.WriteString("                if 'error' in error_data:\n")
	sb.WriteString("                    error = error_data['error']\n")
	sb.WriteString("                    code = error.get('code', -32603)\n")
	sb.WriteString("                    message = error.get('message', 'Internal error')\n")
	sb.WriteString("                    data = error.get('data')\n")
	sb.WriteString("                    raise RPCError(code, message, data)\n")
	sb.WriteString("            except (json.JSONDecodeError, UnicodeDecodeError):\n")
	sb.WriteString("                pass\n")
	sb.WriteString("            # If not JSON-RPC error, raise HTTP error\n")
	sb.WriteString("            raise RPCError(-32603, f\"HTTP error: {e.code} {e.reason}\", None)\n")
	sb.WriteString("        except urllib.error.URLError as e:\n")
	sb.WriteString("            raise RPCError(-32603, f\"Network error: {e.reason}\", None)\n\n\n")
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
		fmt.Fprintf(sb, "    %s\n", strings.TrimSpace(iface.Comment))
		sb.WriteString("    \"\"\"\n\n")
	} else {
		fmt.Fprintf(sb, "    \"\"\"Client for %s interface.\"\"\"\n\n", iface.Name)
	}

	sb.WriteString("    def __init__(self, transport: Transport):\n")
	sb.WriteString("        \"\"\"Initialize client with a transport.\n\n")
	sb.WriteString("        Args:\n")
	sb.WriteString("            transport: Transport instance to use for RPC calls\n")
	sb.WriteString("        \"\"\"\n")
	sb.WriteString("        self.transport = transport\n\n")

	// Generate method lookup for this interface
	sb.WriteString("        # Method definitions for validation\n")
	sb.WriteString("        self._method_defs = {\n")
	for _, method := range iface.Methods {
		fmt.Fprintf(sb, "            '%s': {\n", method.Name)
		sb.WriteString("                'parameters': [\n")
		for _, param := range method.Parameters {
			sb.WriteString("                    {\n")
			fmt.Fprintf(sb, "                        'name': '%s',\n", param.Name)
			sb.WriteString("                        'type': ")
			writeTypeDict(sb, param.Type)
			sb.WriteString(",\n")
			sb.WriteString("                    },\n")
		}
		sb.WriteString("                ],\n")
		sb.WriteString("                'returnType': ")
		writeTypeDict(sb, method.ReturnType)
		sb.WriteString(",\n")
		if method.ReturnOptional {
			sb.WriteString("                'returnOptional': True,\n")
		} else {
			sb.WriteString("                'returnOptional': False,\n")
		}
		sb.WriteString("            },\n")
	}
	sb.WriteString("        }\n\n")

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

	// Get method definition
	fmt.Fprintf(sb, "        method_def = self._method_defs['%s']\n", method.Name)
	sb.WriteString("        params = [\n")
	for _, param := range method.Parameters {
		fmt.Fprintf(sb, "            %s,\n", param.Name)
	}
	sb.WriteString("        ]\n\n")

	// Validate parameters
	sb.WriteString("        # Validate parameters\n")
	sb.WriteString("        expected_params = method_def.get('parameters', [])\n")
	sb.WriteString("        for i, (param_value, param_def) in enumerate(zip(params, expected_params)):\n")
	sb.WriteString("            try:\n")
	sb.WriteString("                validate_type(param_value, param_def['type'], ALL_STRUCTS, ALL_ENUMS, False)\n")
	sb.WriteString("            except Exception as e:\n")
	sb.WriteString("                raise ValueError(f\"Parameter {i} ({param_def['name']}) validation failed: {e}\")\n\n")

	// Call transport
	fmt.Fprintf(sb, "        # Call transport\n")
	fmt.Fprintf(sb, "        method_name = '%s.%s'\n", iface.Name, method.Name)
	sb.WriteString("        response = self.transport.call(method_name, params)\n\n")

	// Extract result
	sb.WriteString("        # Extract result from JSON-RPC response\n")
	sb.WriteString("        if 'error' in response:\n")
	sb.WriteString("            error = response['error']\n")
	sb.WriteString("            code = error.get('code', -32603)\n")
	sb.WriteString("            message = error.get('message', 'Internal error')\n")
	sb.WriteString("            data = error.get('data')\n")
	sb.WriteString("            raise RPCError(code, message, data)\n\n")
	sb.WriteString("        result = response.get('result')\n\n")

	// Validate result
	sb.WriteString("        # Validate result\n")
	sb.WriteString("        return_type = method_def.get('returnType')\n")
	sb.WriteString("        return_optional = method_def.get('returnOptional', False)\n")
	sb.WriteString("        if return_type:\n")
	sb.WriteString("            try:\n")
	sb.WriteString("                validate_type(result, return_type, ALL_STRUCTS, ALL_ENUMS, return_optional)\n")
	sb.WriteString("            except Exception as e:\n")
	sb.WriteString("                raise ValueError(f\"Response validation failed: {e}\")\n\n")

	// Return result
	sb.WriteString("        return result\n\n")
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
		fmt.Fprintf(sb, "    \"\"\"%s\"\"\"\n", strings.TrimSpace(iface.Comment))
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

// writeInterfaceMethodLookup generates code to find method definitions
func writeInterfaceMethodLookup(sb *strings.Builder, interfaces []*parser.Interface) {
	sb.WriteString("        method_def = None\n")
	sb.WriteString("        \n")
	sb.WriteString("        # Interface method lookup\n")
	for i, iface := range interfaces {
		if i == 0 {
			fmt.Fprintf(sb, "        if interface_name == '%s':\n", iface.Name)
		} else {
			fmt.Fprintf(sb, "        elif interface_name == '%s':\n", iface.Name)
		}
		sb.WriteString("            interface_methods = {\n")
		for _, method := range iface.Methods {
			fmt.Fprintf(sb, "                '%s': {\n", method.Name)
			sb.WriteString("                    'parameters': [\n")
			for _, param := range method.Parameters {
				sb.WriteString("                        {\n")
				fmt.Fprintf(sb, "                            'name': '%s',\n", param.Name)
				sb.WriteString("                            'type': ")
				writeTypeDict(sb, param.Type)
				sb.WriteString(",\n")
				sb.WriteString("                        },\n")
			}
			sb.WriteString("                    ],\n")
			sb.WriteString("                    'returnType': ")
			writeTypeDict(sb, method.ReturnType)
			sb.WriteString(",\n")
			if method.ReturnOptional {
				sb.WriteString("                    'returnOptional': True,\n")
			} else {
				sb.WriteString("                    'returnOptional': False,\n")
			}
			sb.WriteString("                },\n")
		}
		sb.WriteString("            }\n")
		sb.WriteString("            method_def = interface_methods.get(method_name)\n")
	}
	sb.WriteString("\n")
}

// generateTestServerPy generates test_server.py with concrete implementations of all interfaces
func generateTestServerPy(idl *parser.IDL, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, _ map[string]*parser.Interface, _ map[string]*NamespaceTypes, _ string, _ string) string {
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
		writeDefaultTestReturn(sb, method.ReturnType, structMap, enumMap)
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
func generateTestClientPy(idl *parser.IDL, structMap map[string]*parser.Struct, enumMap map[string]*parser.Enum, _ map[string]*parser.Interface, _ map[string]*NamespaceTypes, _ string, _ string) string {
	var sb strings.Builder

	sb.WriteString("# Generated by pulserpc - do not edit\n")
	sb.WriteString("# Test client for integration testing\n\n")
	sb.WriteString("import sys\n")
	sb.WriteString("import time\n")
	sb.WriteString("import urllib.request\n")
	sb.WriteString("from client import HTTPTransport\n")
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
	sb.WriteString("    transport = HTTPTransport(server_url)\n")
	for _, iface := range idl.Interfaces {
		clientName := iface.Name + "Client"
		clientVar := strings.ToLower(iface.Name) + "_client"
		fmt.Fprintf(&sb, "    %s = %s(transport)\n", clientVar, clientName)
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

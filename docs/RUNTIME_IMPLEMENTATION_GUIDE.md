# PulseRPC Runtime Implementation Guide

This document describes how to implement a new language runtime for PulseRPC. It is based on the Python implementation and provides a comprehensive guide for creating runtimes for other languages.

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Separation of Concerns](#separation-of-concerns)
3. [Plugin Requirements](#plugin-requirements)
4. [Runtime Library Requirements](#runtime-library-requirements)
5. [Generated Code Requirements](#generated-code-requirements)
6. [Build System Integration](#build-system-integration)
7. [Additional Considerations](#additional-considerations)

## Architecture Overview

The PulseRPC code generation system consists of two main components:

1. **Code Generator Plugin** (Go): Generates language-specific code from IDL
2. **Runtime Library** (Target Language): Provides validation, RPC handling, and type utilities

```
┌─────────────────┐
│   IDL File      │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Go Parser      │
└────────┬────────┘
         │
         ▼
┌─────────────────┐      ┌──────────────────┐
│  Plugin (Go)    │─────▶│  Generated Code   │
│                 │      │  (Target Lang)    │
└────────┬────────┘      └────────┬──────────┘
         │                        │
         │                        │ imports
         │                        ▼
         │              ┌──────────────────┐
         └──────────────▶│  Runtime Library │
                        │  (Target Lang)    │
                        └───────────────────┘
```

## Separation of Concerns

### Runtime Library vs Generated Code

**Runtime Library** (`pkg/runtime/runtimes/{lang}/pulserpc/`):
- **Purpose**: Reusable library code that is copied into the output directory
- **Contents**:
  - Type validation functions (struct, enum, built-in types, arrays, maps)
  - RPC error handling (exception/error classes)
  - Type helper utilities (finding structs/enums, resolving inheritance)
  - **No IDL-specific code** - works with type definitions passed as data structures

**Generated Code** (created by plugin):
- **Purpose**: IDL-specific code that uses the runtime library
- **Contents**:
  - `idl.{ext}` - IDL-specific type definitions (structs, enums) as data structures
  - `server.{ext}` - HTTP server with interface stubs and request handling
  - `client.{ext}` - Client classes with transport abstraction
  - **IDL JSON embedded** - The IDL is embedded directly in `server.{ext}` for the `pulserpc-idl` RPC method

### Runtime Directory Structure

For a language `{lang}`, the runtime should be organized as:

```
pkg/runtime/runtimes/{lang}/
├── pulserpc/              # Runtime library package/module
│   ├── __init__.{ext}       # Package exports (if applicable)
│   ├── rpc.{ext}            # RPC error handling
│   ├── contract.{ext}       # Contract class for IDL parsing and validation
│   ├── validation.{ext}     # Type validation functions
│   ├── types.{ext}          # Type helper functions
│   ├── transport.{ext}      # Transport abstraction
│   ├── client.{ext}         # Transport-independent Client
│   └── server.{ext}         # Transport-independent Server
├── tests/                   # Unit tests for runtime
│   ├── test_validation.{ext}
│   ├── test_types.{ext}
│   └── test_rpc.{ext}
├── Makefile                 # Build/test targets
└── README.md                # Runtime-specific documentation
```

**Reference Architecture (Python Implementation):**

The runtime uses a modular architecture that separates transport logic from RPC handling:

- **`pulserpc/Contract`**: Encapsulates IDL parsing and request/response validation
- **`pulserpc/Server`**: Transport-independent server class that processes JSON-RPC requests
- **`pulserpc/Client`**: Transport-independent client class that auto-discovers interfaces
- **`pulserpc/Transport`**: Abstract transport interface
- **`pulserpc/HttpTransport`**: HTTP transport implementation
- **`pulserpc/InProcTransport`**: In-process transport for testing

Generated code (`server.py`, `client.py`) creates thin wrappers around these core classes:
- Generated `server.py` creates a module-level `Server` instance and provides `PulseRPCServer` HTTP wrapper
- Generated `client.py` uses the `Client` class with auto-discovery

This design allows the runtime to support multiple transports (HTTP, WebSocket, etc.) without code duplication.

### Runtime Library Components

#### 1. RPC Error Handling (`rpc.{ext}`)

Must provide an exception/error class for JSON-RPC errors:

- **Class name**: `RPCError` (or language-appropriate equivalent)
- **Properties**:
  - `code` (int): JSON-RPC error code
  - `message` (string): Error message
  - `data` (any): Optional error data
- **Usage**: Thrown/returned when JSON-RPC calls fail

**Example (Python)**:
```python
class RPCError(Exception):
    def __init__(self, code: int, message: str, data: Any = None):
        self.code = code
        self.message = message
        self.data = data
```

#### 2. Type Validation (`validation.{ext}`)

Must provide validation functions for all PulseRPC types:

- **Built-in types**: `string`, `int`, `float`, `bool`
- **Arrays**: `[]Type` - validate array structure and element types
- **Maps**: `map[string]Type` - validate map structure, string keys, value types
- **Enums**: Validate string value matches enum definition
- **Structs**: Validate dict/object structure, required fields, optional fields, inheritance
- **Main function**: `validate_type(value, type_def, all_structs, all_enums, is_optional)`

**Key Requirements**:
- Must handle optional types (None/null values)
- Must handle struct inheritance (`extends`)
- Must provide clear error messages indicating what failed and where
- Must validate nested types recursively

**Type Definition Format**:
Type definitions are passed as dictionaries/objects with the following structure:
```json
{
  "builtIn": "string" | "int" | "float" | "bool",
  "array": <type_def>,
  "mapValue": <type_def>,
  "userDefined": "TypeName"
}
```

#### 3. Contract Class (`contract.{ext}`)

The Contract class encapsulates IDL parsing and request/response validation. This is a core architectural pattern that provides:

- **Single point of IDL parsing**: Parses the IDL JSON once and stores parsed interface/struct/enum definitions
- **Centralized validation**: `validate_request()` and `validate_response()` methods use the parsed IDL
- **Interface metadata**: `get_interface(name)` and `has_interface(name)` helpers for runtime lookup

**Class structure**:
```python
class Interface:
    """Represents an interface from the IDL"""
    name: str
    functions: Dict[str, FunctionDef]  # maps function name to metadata

class Contract:
    """Represents a parsed IDL contract"""
    interfaces: Dict[str, Interface]
    structs: Dict[str, StructDef]
    enums: Dict[str, EnumDef]

    def __init__(self, idl_data):
        """Initialize from IDL JSON (supports both PulseRPC dict and Barrister list formats)"""

    def validate_request(self, iface_name: str, func_name: str, params: List[Any]) -> None:
        """Validate request parameters against IDL signature"""

    def validate_response(self, iface_name: str, func_name: str, result: Any) -> None:
        """Validate response against IDL return type"""

    def get_interface(self, iface_name: str) -> Optional[Interface]
    def has_interface(self, iface_name: str) -> bool
```

**Python Example**:
```python
from pulserpc import Contract

# Parse IDL and create contract
contract = Contract(idl_data)

# Validate a request
contract.validate_request("UserService", "getUser", [{"userId": "123"}])

# Validate a response
contract.validate_response("UserService", "getUser", {"name": "John", "email": "john@example.com"})
```

#### 4. Type Helpers (`types.{ext}`)

Must provide utility functions for working with type definitions:

- `find_struct(name, all_structs)` - Find struct definition by name
- `find_enum(name, all_enums)` - Find enum definition by name
- `get_struct_fields(name, all_structs)` - Get all fields including parent fields (handles `extends`)

**Struct Definition Format**:
```json
{
  "extends": "ParentStruct",  // optional
  "fields": [
    {
      "name": "fieldName",
      "type": <type_def>,
      "optional": true/false
    }
  ]
}
```

**Enum Definition Format**:
```json
{
  "values": [
    {"name": "VALUE1"},
    {"name": "VALUE2"}
  ]
}
```

## Plugin Requirements

### Plugin Interface

All plugins must implement the `generator.Plugin` interface:

```go
type Plugin interface {
    Name() string                    // e.g., "python-client-server"
    RegisterFlags(fs *flag.FlagSet)  // Register CLI flags
    Generate(idl *parser.IDL, fs *flag.FlagSet) error
}
```

### Plugin Registration

Plugins are registered in `cmd/pulserpc/pulserpc.go`:

```go
func registerPlugins() {
    generator.Register(generator.NewPythonClientServer())
    // Add new plugins here
}
```

### Plugin Implementation Steps

1. **Create plugin file**: `pkg/generator/{lang}_client_server.go`
2. **Implement Plugin interface**:
   - `Name()`: Return plugin identifier (e.g., "java-client-server")
   - `RegisterFlags()`: Register any language-specific flags
   - `Generate()`: Main code generation logic
3. **Register plugin**: Add to `registerPlugins()` in `cmd/pulserpc/pulserpc.go`

### Generate() Method Responsibilities

The `Generate()` method must:

1. **Access output directory**: Read `-dir` flag from FlagSet
2. **Build type registries**: Create maps of structs, enums, interfaces for efficient lookup
3. **Copy runtime files**: Use `runtime.CopyRuntimeFiles(lang, outputDir)` to copy embedded runtime files to output directory
4. **Generate IDL-specific file**: Create `idl.{ext}` with type definitions
5. **Generate server file**: Create `server.{ext}` with HTTP server, interface stubs, and **embedded IDL JSON** for `pulserpc-idl` RPC method
6. **Generate client file**: Create `client.{ext}` with client classes and transport

### Runtime File Copying

Runtime files are embedded directly into the pulserpc binary using Go's `embed` package. This allows the binary to be self-contained and work without requiring the source tree at runtime.

Plugins should use the `runtime` package to copy embedded runtime files:

```go
import "github.com/coopernurse/pulserpc/pkg/runtime"

func (p *PythonClientServer) copyRuntimeFiles(outputDir string) error {
    return runtime.CopyRuntimeFiles("python", outputDir)
}
```

The `runtime.CopyRuntimeFiles()` function:
- Extracts embedded runtime files from the binary
- Copies them to `outputDir/{runtimePackageName}/` (e.g., `outputDir/pulserpc/` for Python)
- Handles directory creation and file permissions automatically

**Adding a New Runtime**:

To add runtime files for a new language, you must:

1. **Add embed directive**: In `pkg/runtime/embed.go`, add a new embed variable:
   ```go
   //go:embed all:runtimes/java/pulserpc
   var javaRuntimeFiles embed.FS
   ```

3. **Register in runtimeMap**: Add the new runtime to the `runtimeMap`:
   ```go
   var runtimeMap = map[string]embed.FS{
       "python": pythonRuntimeFiles,
       "java": javaRuntimeFiles,  // Add new runtime here
   }
   ```

4. **Update file filtering** (if needed): In `GetRuntimeFiles()`, add language-specific file filtering if your language has different file extensions:
   ```go
   if lang == "java" && !strings.HasSuffix(entry.Name(), ".java") {
       continue
   }
   ```

**Note**: Go's `embed` directive doesn't support `..` paths, so runtime files must be located in `pkg/runtime/runtimes/` to enable embedding.

## Runtime Library Requirements

### Standard Library Preference

- **Use standard library whenever possible** - avoid third-party dependencies
- If third-party libraries are necessary, document them clearly and minimize the set
- Consider the impact on users who must install dependencies

### Language-Specific Considerations

- **Package/module structure**: Follow language conventions
- **Naming conventions**: Follow language style guides
- **Error handling**: Use language-appropriate mechanisms (exceptions, errors, etc.)
- **Type system**: Leverage language type system where possible, but runtime validation is still required

## Generated Code Requirements

### 1. IDL-Specific File (`idl.{ext}`)

**Purpose**: Define IDL-specific type definitions as data structures

**Contents**:
- **For static languages** (C#, TypeScript, Java, Go):
  - Generated classes for all structs (with proper inheritance)
  - Generated enums for all IDL enums
  - `ALL_STRUCTS` - Dictionary/map of struct definitions (for runtime validation)
  - `ALL_ENUMS` - Dictionary/map of enum definitions (for runtime validation)
  - Imports from runtime library
  
- **For dynamic languages** (Python, JavaScript):
  - `ALL_STRUCTS` - Dictionary/map of struct definitions
  - `ALL_ENUMS` - Dictionary/map of enum definitions
  - Imports from runtime library

**Format**: Type definitions match the format expected by runtime validation functions. Static languages generate both static types (for user code) and dictionary types (for validation).

**Example structure**:
```python
from pulserpc import validate_type, validate_struct, validate_enum, ...

ALL_STRUCTS = {
    'User': {
        'extends': 'Base',  # optional
        'fields': [
            {
                'name': 'id',
                'type': {'builtIn': 'string'},
                'optional': False
            }
        ]
    }
}

ALL_ENUMS = {
    'Platform': {
        'values': [
            {'name': 'kindle'},
            {'name': 'nook'}
        ]
    }
}
```

### 2. Server File (`server.{ext}`)

**Purpose**: HTTP server that handles JSON-RPC 2.0 requests

**Requirements**:

1. **Interface Stubs**:
   - Generate abstract base class/interface for each IDL interface
   - Each method should be abstract/must implement
   - Include method signatures matching IDL
   - **For static languages**: Use generated struct/enum types in method signatures
     - Example (C#): `public abstract RepeatResponse repeat(RepeatRequest req1);`
     - Example (Java): `public abstract RepeatResponse repeat(RepeatRequest req1);`
   - **For dynamic languages**: May use generic types (e.g., `object`, `dict`, `Any`)
     - Example (Python): `def repeat(self, req1: dict) -> dict:`

2. **Transport-Independent Server Class**:
   - **Constructor**: Takes a `Contract` instance and validation flags
     ```python
     server = Server(contract, validate_requests=True, validate_responses=True)
     ```
   - **Registration**: Easy way to register interface implementations
     ```python
     server.add_handler("InterfaceName", implementation_instance)
     ```
   - **`call(req)` Method**: Process a single JSON-RPC request dict
     - Returns JSON-RPC response dict or `None` for notifications
     - Works with any transport layer (HTTP, WebSocket, etc.)
   - **Request Processing**:
     - Validate JSON-RPC 2.0 structure (jsonrpc, method, params, id)
     - Handle `pulserpc-idl` method (returns embedded IDL JSON)
     - Parse `Interface.method` to find handler
     - Normalize params (list to dict using IDL signature)
     - Validate request against Contract (if enabled)
     - Invoke handler method
     - Validate response against Contract (if enabled)
   - **Error Handling**:
     - Return JSON-RPC 2.0 error responses
     - Handle `RPCError` exceptions from handlers
     - Handle validation errors
     - Handle internal errors

3. **HTTP Adapter (Generated)**:
   - HTTP binding is separate from the core Server
   - Takes a `Server` instance and feeds it parsed JSON-RPC dicts
   - Example: `PulseRPCServer` wraps `Server` and implements HTTP request handling

**Python Architecture (Reference Implementation)**:
```python
# In pulserpc/server.py (runtime library):
class Server:
    def __init__(self, contract: Contract, validate_requests=True, validate_responses=True):
        self.handlers: Dict[str, Any] = {}
        self.contract = contract
        self.validate_requests = validate_requests
        self.validate_responses = validate_responses

    def add_handler(self, iface_name: str, handler: Any) -> None:
        self.handlers[iface_name] = handler

    def call(self, req: Dict[str, Any]) -> Optional[Dict[str, Any]]:
        # Validates JSON-RPC format
        # Handles pulserpc-idl method
        # Parses Interface.method
        # Looks up handler
        # Normalizes params
        # Validates request (if enabled)
        # Invokes handler.method(**params)
        # Validates response (if enabled)
        # Returns JSON-RPC response dict or None
        pass

# Generated server.py:
from pulserpc import Server, Contract, RPCError

# Create Contract from embedded IDL
contract = Contract(idl_data)

# Create module-level Server instance
rpc_server = Server(contract, validate_requests=True, validate_responses=True)

# Abstract interface stubs (generated)
class CatalogService:
    def listProducts(self) -> list: pass
    def getProduct(self, productId: str) -> dict: pass

# HTTP adapter (generated)
class PulseRPCServer:
    def __init__(self, host='localhost', port=8080):
        self.host = host
        self.port = port

    def register(self, interface_name, instance):
        rpc_server.add_handler(interface_name, instance)

    def serve_forever(self):
        # HTTP server that parses JSON and calls rpc_server.call(req)
        pass
```

**Key Design Principle**: The `Server` class is transport-independent. It processes JSON-RPC dicts, not HTTP requests. This allows the same Server to work with HTTP, WebSocket, or any other transport by feeding it parsed JSON-RPC requests.

3. **Server Lifecycle**:
   - `serve_forever()` or equivalent - start server
   - `shutdown()` or equivalent - stop server
   - Configurable host and port

**Legacy/Monolithic Architecture (pre-refactor)**:
```python
class PulseRPCServer:
    def __init__(self, host='localhost', port=8080):
        self.handlers = {}

    def register(self, interface_name, instance):
        self.handlers[interface_name] = instance

    def handle_request(self, request_json):
        # Validate JSON-RPC structure
        # Handle pulserpc-idl
        # Route to handler
        # Validate params
        # Call handler method
        # Validate response
        # Return JSON-RPC response

    def serve_forever(self):
        # Start HTTP server
```

### 3. Client File (`client.{ext}`)

**Purpose**: Client classes for making RPC calls with automatic interface discovery

**Requirements**:

1. **Transport Abstraction**:
   - Abstract base class/interface for transports
   - `call(method, params)` method that returns JSON-RPC response
   - Allows pluggable transports (HTTP, ZeroMQ, etc.)

2. **HTTP Transport** (default implementation):
   - Uses standard HTTP library for the language
   - Configurable base URL
   - **Configurable headers**: Must allow setting HTTP headers (for auth, etc.)
   - Handles JSON-RPC 2.0 request/response
   - Handles errors (HTTP errors, JSON-RPC errors)
   - Generates unique request IDs

3. **Client Class with Auto-Discovery**:
   - Single `Client` class that takes a `Transport` in constructor
   - On construction, fetches IDL from server via `pulserpc-idl` RPC method
   - Creates a `Contract` instance from the IDL
   - Creates dynamic interface proxies (e.g., `client.UserService.getUser(...)`)
   - **Parameter validation**: Validate parameters before sending (using Contract)
   - **Response validation**: Validate response before returning (using Contract)
   - Raise `RPCError` on JSON-RPC errors

**Python Architecture (Reference Implementation):**
```python
# In pulserpc/client.py (runtime library):
class InterfaceClientProxy:
    """Dynamic proxy for an interface that provides callable methods"""
    def __init__(self, client: 'Client', iface):
        self._client = client
        self._iface = iface
        for func_name in iface.functions.keys():
            setattr(self, func_name, self._create_methodcaller(func_name))

class Client:
    def __init__(self, transport: Transport, validate_request=False, validate_response=False):
        self.transport = transport
        self._bootstrap()  # Fetches IDL and creates Contract

    def _bootstrap(self):
        # Fetch IDL via pulserpc-idl
        resp = self.transport.request({'method': 'pulserpc-idl', 'id': 'bootstrap'})
        self.contract = Contract(resp['result'])
        # Create interface proxies
        for iface_name, iface in self.contract.interfaces.items():
            setattr(self, iface_name, InterfaceClientProxy(self, iface))

    def call(self, method: str, params=None) -> Any:
        # Build JSON-RPC request and send via transport
        # Handle response, raise RPCError on error
        pass
```

**Usage Example**:
```python
from pulserpc import Client, HttpTransport

transport = HttpTransport("http://localhost:8080")
client = Client(transport)  # Auto-discovers interfaces

# Use dynamic interface proxies - no need for generated client classes
products = client.CatalogService.listProducts()
user = client.UserService.getUser({"userId": "123"})
```

**For languages without dynamic proxies**, generated code may create static interface client classes that delegate to the `Client`:
```python
# Generated client.py:
from pulserpc import Client, HttpTransport

# Create client instance
transport = HttpTransport("http://localhost:8080")
_client = Client(transport)

# Generated interface client (delegates to Client):
class CatalogServiceClient:
    def listProducts(self):
        return _client.CatalogService.listProducts()
```

**Transport Interface**:
```python
# In pulserpc/transport.py (runtime library):
class Transport(ABC):
    @abstractmethod
    def request(self, req: dict) -> dict:
        pass

class HttpTransport(Transport):
    def __init__(self, base_url: str, headers: Optional[Dict[str, str]] = None):
        self.base_url = base_url
        self.headers = headers or {}

    def request(self, req: dict) -> dict:
        # Send HTTP POST request
        # Return JSON-RPC response
        pass

class InProcTransport(Transport):
    """For testing - bypasses network"""
    def __init__(self, server_call_fn):
        self.server_call_fn = server_call_fn
```

**Legacy/Monolithic Architecture (pre-refactor):**
```python
class Transport(ABC):
    @abstractmethod
    def call(self, method: str, params: list) -> dict:
        pass

class HTTPTransport(Transport):
    def __init__(self, base_url: str, headers: Optional[Dict[str, str]] = None):
        self.base_url = base_url
        self.headers = headers or {}
    
    def call(self, method: str, params: list) -> dict:
        # Build JSON-RPC request
        # Add headers
        # Send HTTP POST
        # Parse response
        # Handle errors

class BookServiceClient:
    def __init__(self, transport: Transport):
        self.transport = transport
    
    def getBook(self, bookId: str):
        # Validate params
        # Call transport
        # Validate response
        # Return result
```

### 4. Embedded IDL JSON

**Purpose**: The IDL JSON is embedded directly in the server file as a constant for the `pulserpc-idl` RPC method

**Format**: Language-appropriate string constant containing JSON-serialized `parser.IDL` structure

**Implementation Details**:
- **Go**: Raw string literal (backticks) - no escaping needed
- **C#:** Verbatim string `@"..."` - escape `"` as `""`
- **Java**: Text block `"""..."""` - escape `"""` as `\"""\"`
- **Python**: Triple-quoted string `'''...'''` - escape `'` as `\'` and `\` as `\\`
- **TypeScript**: Template literal `` `...` `` - escape `` ` `` as `` \` ``, `$` as `\$`, and `\` as `\\`

**Usage**: Server uses the embedded constant when handling `pulserpc-idl` requests

### Static vs Dynamic Type Generation

**Important**: The code generation approach differs significantly between static and dynamic languages.

**Static Languages** (C#, TypeScript, Java, Go):

- **Must generate actual classes** for all IDL structs
  - Use native class syntax with proper inheritance (`extends` maps to class inheritance)
  - Include JSON serialization attributes/annotations for field name mapping
  - Handle optional fields as nullable types
  
- **Must generate native enums** for all IDL enums
  - C#: `enum`, TypeScript: `enum`, Java: `enum`, Go: constants
  
- **Interface stub methods** must use these generated types in method signatures
  - Example (C#): `public abstract RepeatResponse repeat(RepeatRequest req1);`
  - Example (Java): `public abstract RepeatResponse repeat(RepeatRequest req1);`
  
- **Client methods** must use these generated types for parameters and return values
  - Type-safe method signatures with proper return types
  - JSON serialization/deserialization at the boundary (client ↔ transport)
  
- **Test server implementations** must use these generated types
  - Create instances of generated struct classes
  - Use enum values directly (not strings)
  
- **Dual type definitions**: Static languages must generate BOTH:
  1. Static type definitions (classes, enums) for use in user code
  2. Dictionary-based type definitions (ALL_STRUCTS, ALL_ENUMS) for runtime validation
  
- **JSON serialization**: Must work with generated types (may require attributes/annotations)
  - Field name mapping: IDL uses snake_case, languages may use different conventions
  - Use appropriate JSON library attributes (e.g., `[JsonPropertyName]` in C#, `@JsonProperty` in Java)

**Dynamic Languages** (Python, JavaScript):

- May use dictionary/map types for structs (Python `dict`, JavaScript `object`)
- May use string-based enums
- Runtime validation still required (uses ALL_STRUCTS/ALL_ENUMS dictionaries)
- Type safety provided by runtime validation, not compile-time types

**Example Comparison**:

**C# (Static)**:
```csharp
// Generated class
public class RepeatRequest {
    [JsonPropertyName("to_repeat")]
    public string ToRepeat { get; set; }
    
    [JsonPropertyName("count")]
    public int Count { get; set; }
}

// Interface stub uses typed parameters
public abstract RepeatResponse repeat(RepeatRequest req1);
```

**Python (Dynamic)**:
```python
# No generated class, uses dict
# Interface stub uses object/Any
def repeat(self, req1: dict) -> dict:
    pass
```

## Build System Integration

### Makefile Structure

The root `Makefile` should include targets for testing each runtime:

```makefile
# Test {lang} runtime
test-runtime-{lang}:
	@echo "Testing {lang} runtime..."
	@cd pkg/runtime/runtimes/{lang} && $(MAKE) test

# Test all runtimes
test-runtimes: test-runtime-python test-runtime-{lang}
	@echo "All runtime tests passed"
```

### Runtime-Specific Makefile

Each runtime should have its own `Makefile` in `pkg/runtime/runtimes/{lang}/`:

**Required Targets**:
- `test` - Run tests (should use Docker if available)
- `test-docker` - Run tests in Docker container
- `clean` - Clean build artifacts

**Docker Testing Pattern**:

```makefile
# Variables
{LANG}_IMAGE={lang}:{version}  # e.g., openjdk:17-slim, node:18-slim
DOCKER_AVAILABLE := $(shell command -v docker >/dev/null 2>&1 && echo "yes" || echo "no")

# Test using local {lang} if available, otherwise Docker
test:
ifeq ($(DOCKER_AVAILABLE),yes)
	@echo "Using Docker for testing..."
	@$(MAKE) test-docker
else
	@echo "Using local {lang} for testing..."
	@{lang-specific test command}
endif

# Test using Docker
test-docker:
	@echo "Testing {lang} runtime in Docker..."
	@docker run --rm -v $(PWD):/workspace -w /workspace \
		$({LANG}_IMAGE) \
		{lang-specific test command}
```

**Benefits**:
- No assumption that user has the language installed
- Consistent test environment
- Easy CI/CD integration
- Works on any platform with Docker

### Docker Image Selection

Choose appropriate official Docker images:
- **Python**: `python:3.11-slim` or similar
- **Java**: `openjdk:17-slim` or `eclipse-temurin:17-jdk`
- **Node.js**: `node:18-slim` or `node:20-slim`
- **Go**: `golang:1.21-alpine`
- **Ruby**: `ruby:3.2-slim`
- **Rust**: `rust:1.75-slim`

## Additional Considerations

### 1. JSON-RPC 2.0 Compliance

Both server and client must fully comply with JSON-RPC 2.0 specification:
- Request format: `{jsonrpc: "2.0", method: "...", params: [...], id: "..."}`
- Response format: `{jsonrpc: "2.0", result: ..., id: "..."}` or `{jsonrpc: "2.0", error: {...}, id: "..."}`
- Batch requests: Array of requests
- Notifications: Requests without `id` field (no response sent)
- Error codes: Standard JSON-RPC error codes (-32700, -32600, -32601, -32602, -32603)

### 2. Type System Integration

- **Static typing**: If language supports static typing, use it where possible
- **Runtime validation**: Still required even with static types (defense in depth)
- **Type definitions**: May need to generate type definitions for static type checkers (e.g., TypeScript `.d.ts`, Java generics)

### 3. Error Messages

- **Clear validation errors**: Indicate what failed, where, and why
- **Context**: Include field names, parameter indices, type names
- **User-friendly**: Errors should help users fix their code

### 4. Performance Considerations

- **Validation**: Can be expensive for large nested structures - consider performance
- **Caching**: Consider caching type definitions, compiled validators
- **Lazy validation**: Consider making validation optional in production

### 5. Documentation

- **Runtime README**: Document installation, usage, API
- **Generated code comments**: Include helpful comments in generated code
- **Examples**: Provide example usage in runtime README

### 6. Testing

**Runtime Tests**:
- Test all validation functions
- Test error handling
- Test type helpers
- Test edge cases (null, empty arrays, inheritance, etc.)

**Integration Tests** (optional but recommended):
- Test full server/client interaction
- Test with real IDL files
- Test error scenarios
- Test batch requests
- Test notifications

### 7. Namespace Handling

- **IDL namespaces**: May need to map to language namespaces/packages
- **Qualified names**: Handle qualified type names (e.g., `inc.Response`)
- **Import statements**: Generate appropriate import/using statements

### 8. Comments

- **IDL comments**: Preserve and include in generated code where appropriate
- **Generated code comments**: Mark generated code clearly ("Generated by pulserpc - do not edit")

### 9. Code Style

- **Consistent formatting**: Use language formatters (gofmt, black, prettier, etc.)
- **Naming conventions**: Follow language conventions
- **File organization**: Follow language project structure conventions

### 10. Optional Fields

- **Struct fields**: Must handle optional fields correctly
- **Validation**: Optional fields can be missing or null
- **Serialization**: Ensure optional fields are handled correctly in JSON

### 11. Struct Inheritance

- **Extends**: Must handle struct inheritance (`struct Child extends Parent`)
- **Field resolution**: Get all fields including parent fields
- **Field override**: Handle field name conflicts (child overrides parent)

### 12. Method Return Types

- **Void returns**: Handle methods that return void/null
- **Optional returns**: Handle optional return types (if supported by IDL)
- **Validation**: Validate return values match IDL definition

### 13. Request ID Generation

- **Unique IDs**: Generate unique request IDs for each RPC call
- **Type**: Can be string, number, or null (per JSON-RPC 2.0)
- **UUID**: Consider using UUIDs for string IDs

### 14. HTTP Headers

- **Content-Type**: Must set `application/json`
- **Content-Length**: Should set for proper HTTP compliance
- **Custom headers**: Allow users to set custom headers (auth, etc.)

### 15. Logging

- **Server logging**: Consider logging requests/responses (optional, configurable)
- **Error logging**: Log errors appropriately
- **Debug mode**: Consider debug mode for verbose logging

### 16. Concurrency

- **Thread safety**: Consider thread safety if language/runtime requires it
- **Async support**: Consider async/await support if language supports it
- **Connection pooling**: For HTTP clients, consider connection pooling

### 17. Security

- **Input validation**: Always validate input (defense in depth)
- **Error messages**: Don't leak sensitive information in error messages
- **HTTP security**: Consider security headers, HTTPS support

### 18. Backward Compatibility

- **Runtime changes**: Consider impact on existing generated code
- **Versioning**: Consider versioning runtime library
- **Breaking changes**: Document breaking changes clearly

### 19. Integration Testing

To verify that generated client and server code can interoperate correctly, each generator plugin should support automated integration testing.

#### Test Generation Flag

Plugins should check for the `-test-server` flag in the `Generate()` method:

```go
testServerFlag := fs.Lookup("test-server")
generateTestServer := false
if testServerFlag != nil && testServerFlag.Value.String() == "true" {
    generateTestServer = true
}
```

When this flag is set, the plugin should generate two additional files:

1. **`test_server.{ext}`** - Concrete implementations of all interface stubs
2. **`test_client.{ext}`** - Test program that exercises all client methods

#### Test Server Generation (`test_server.{ext}`)

The test server must:

- **Implement all interface methods**: Create concrete implementation classes for each interface
- **Follow IDL comments**: Where methods have comments describing behavior, implement accordingly
- **Handle all type cases**: Built-ins, structs, arrays, maps, enums, optional fields and returns
- **Return appropriate types**: Match the IDL return types exactly
- **Handle special cases**: For example, `B.echo` should return `None`/`null` when input is `"return-null"`

**Example structure**:
```python
class AImpl:
    def add(self, a: int, b: int) -> int:
        return a + b
    
    def sqrt(self, a: float) -> float:
        return math.sqrt(a)
    # ... other methods

if __name__ == "__main__":
    server = PulseRPCServer(host="0.0.0.0", port=8080)
    server.register("A", AImpl())
    server.serve_forever()
```

#### Test Client Generation (`test_client.{ext}`)

The test client must:

- **Exercise all interface methods**: Call every method on every interface
- **Validate responses**: Assert that responses match expected values
- **Handle optional returns**: Test both null and non-null cases where applicable
- **Report test results**: Print pass/fail for each test and exit with appropriate code
- **Wait for server**: Include logic to wait for server to be ready before running tests

**Example structure** (Python):
```python
from pulserpc import HttpTransport
from client import AClient

def main():
    transport = HttpTransport("http://localhost:8080")
    client = AClient(transport)

    errors = []

    try:
        result = client.add(2, 3)
        assert result == 5
        print("✓ A.add passed")
    except Exception as e:
        errors.append(f"A.add failed: {e}")

    if errors:
        print(f"FAILED: {len(errors)} test(s) failed")
        sys.exit(1)
    else:
        print("SUCCESS: All tests passed!")
        sys.exit(0)
```

**Example structure** (other languages):
```python
def main():
    transport = HTTPTransport("http://localhost:8080")
    client = AClient(transport)

    errors = []

    try:
        result = client.add(2, 3)
        assert result == 5
        print("✓ A.add passed")
    except Exception as e:
        errors.append(f"A.add failed: {e}")

    if errors:
        print(f"FAILED: {len(errors)} test(s) failed")
        sys.exit(1)
    else:
        print("SUCCESS: All tests passed!")
        sys.exit(0)
```

#### Docker Test Harness

A test harness script (`tests/integration/test_generator.sh`) should:

1. **Build the pulserpc binary** (if needed)
2. **Generate code** from `examples/conform.pulse` with `-test-server` flag
3. **Start the test server** in background
4. **Wait for server to be ready** (poll or timeout)
5. **Run the test client** program
6. **Capture results** and exit codes
7. **Clean up** server process and temporary files

The script should use Docker to ensure a consistent test environment:

```bash
docker run --rm \
    -v $(pwd):/workspace \
    -w /workspace \
    python:3.11-slim \
    /bin/bash -c "bash tests/integration/test_generator.sh"
```

#### Makefile Integration

Each runtime should add a `test-integration` target to its Makefile:

```makefile
test-integration:
	@echo "Testing {lang} generator integration..."
	@cd ../.. && docker run --rm \
		-v $$(pwd):/workspace \
		-w /workspace \
		$({LANG}_IMAGE) \
		/bin/bash -c "bash tests/integration/test_generator.sh"
```

The root Makefile should include generator test targets:

```makefile
test-generator-{lang}:
	@echo "Testing {lang} generator integration..."
	@cd pkg/runtime/runtimes/{lang} && $(MAKE) test-integration

test-generators: test-generator-python test-generator-{lang}
	@echo "All generator tests passed"
```

#### Test IDL

The `examples/conform.pulse` file is designed to exercise all IDL features:

- All built-in types (string, int, float, bool)
- Arrays and maps
- Structs and inheritance (`extends`)
- Enums (including namespaced enums)
- Optional fields and optional returns
- Multiple interfaces
- Namespaces

This IDL should be used for all integration tests to ensure comprehensive coverage.

### 20. Test Server Management

To facilitate testing with the web UI, a centralized script manages test servers for all runtimes in Docker containers.

#### Script Location

The test server management script is located at `scripts/test-servers.sh` and provides three commands:

- **`start`**: Starts all test server containers
- **`stop`**: Stops and removes all test server containers
- **`status`**: Shows the status of running containers

#### Usage

```bash
# Start all test servers
./scripts/test-servers.sh start

# Check status
./scripts/test-servers.sh status

# Stop all servers
./scripts/test-servers.sh stop
```

#### Adding a New Runtime

When implementing a new runtime, you must add it to the `RUNTIMES` array in `scripts/test-servers.sh`:

```bash
RUNTIMES=(
    "python:python-client-server:python:3.11-slim:9000:python3 test_server.py"
    "ts:ts-client-server:node:18-slim:9001:ts-node --project tsconfig.json test_server.ts"
    "java:java-client-server:openjdk:17-slim:9002:java -cp . TestServer"  # New runtime
)
```

The format is: `name:plugin:image:port:start_command`

- **name**: Short identifier for the runtime (e.g., `python`, `ts`, `java`)
- **plugin**: Plugin name used with the `-plugin` flag (e.g., `python-client-server`)
- **image**: Docker image to use (e.g., `python:3.11-slim`, `node:18-slim`)
- **port**: Host port to map (starting at 9000, increment for each runtime)
- **start_command**: Command to run the test server inside the container

#### Port Assignment

Ports are assigned starting at 9000 and incrementing for each runtime:
- Python: 9000
- TypeScript: 9001
- Java (if added): 9002
- etc.

#### Container Naming

Containers are named using the pattern `pulserpc-test-{name}`:
- `pulserpc-test-python`
- `pulserpc-test-ts`
- `pulserpc-test-java` (if added)

This naming convention allows the script to easily find and manage all test server containers.

#### Container Configuration

Each container:
- Runs in detached mode (`-d`)
- Maps host port to container port 8080 (e.g., `-p 9000:8080`)
- Mounts the generated code directory as `/workspace`
- Uses the appropriate Docker image for the runtime
- Runs the test server command in the workspace directory

#### Health Checks

The script performs health checks by calling the `pulserpc-idl` RPC method on each server. Servers must respond to this method within 30 seconds to be considered ready.

#### Makefile Integration

The root `Makefile` includes convenience targets:

```makefile
start-test-servers:
	@./scripts/test-servers.sh start

stop-test-servers:
	@./scripts/test-servers.sh stop

status-test-servers:
	@./scripts/test-servers.sh status
```

These can be used as:
```bash
make start-test-servers
make stop-test-servers
make status-test-servers
```

## Implementation Checklist

When implementing a new runtime, ensure:

- [ ] Plugin implements `generator.Plugin` interface
- [ ] Plugin registered in `registerPlugins()`
- [ ] Runtime library structure created in `pkg/runtime/runtimes/{lang}/`
- [ ] Embed directive added in `pkg/runtime/embed.go` for new language
- [ ] New runtime added to `runtimeMap` in `pkg/runtime/embed.go`
- [ ] Plugin uses `runtime.CopyRuntimeFiles()` to copy embedded runtime files
- [ ] `idl.{ext}` generated with type definitions
- [ ] `server.{ext}` generated with HTTP server and **embedded IDL JSON**
- [ ] `client.{ext}` generated with transport abstraction
- [ ] Runtime validation functions implemented
- [ ] RPC error class implemented
- [ ] Type helper functions implemented
- [ ] Interface stubs generated
- [ ] Server validates requests and responses
- [ ] Client validates parameters and responses
- [ ] HTTP transport supports custom headers
- [ ] Server handles `pulserpc-idl` method
- [ ] Server handles batch requests
- [ ] Server handles notifications
- [ ] Makefile targets for testing
- [ ] Docker testing setup
- [ ] Runtime tests written
- [ ] Test server generation implemented (`-test-server` flag)
- [ ] Test client generation implemented
- [ ] Integration test harness works
- [ ] Runtime added to `scripts/test-servers.sh` RUNTIMES array
- [ ] Documentation written
- [ ] Examples provided

## Example: Java Runtime (Hypothetical)

To illustrate the concepts, here's how a Java runtime might be structured:

**Runtime Structure**:
```
pkg/runtime/runtimes/java/
├── pulserpc/
│   ├── RPCError.java
│   ├── Validation.java
│   └── Types.java
├── tests/
│   ├── ValidationTest.java
│   └── TypesTest.java
└── Makefile
```

**Generated Files**:
- `Idl.java` - Contains `ALL_STRUCTS` and `ALL_ENUMS` as static maps
- `Server.java` - HTTP server using `HttpServer` or Servlet with **embedded IDL JSON**
- `Client.java` - Client classes with `Transport` interface

**Server Integration**:
- Could use `com.sun.net.httpserver.HttpServer` (standard library)
- Or generate Servlet for integration with Servlet containers
- Interface stubs as abstract classes or interfaces

**Client Transport**:
- `Transport` interface
- `HTTPTransport` using `java.net.http.HttpClient` (Java 11+)
- Support for custom headers via `HttpRequest.Builder`


### Java Runtime Package Conventions

**Java runtime** follows idiomatic Java package naming with full package qualification:

#### Generated Code Structure
When generating with `-dir src/main/java -package com.myapp.rpc`:
- **Runtime package**: `pulserpc` → `{dir}/pulserpc/`
  - Files: `src/main/java/pulserpc/RPCError.java`, etc.
  - Package declaration: `package pulserpc;`
  
- **Namespace packages**: `{package}.{namespace}` → `{dir}/{package-dirs}/{namespace}/`
  - Example: namespace `user` → `src/main/java/com/myapp/rpc/user/`
  - Package declaration: `package com.myapp.rpc.user;`
  - Cross-namespace imports: `import com.myapp.rpc.common.MyType;`

#### Key Java-Specific Rules:
1. **Package declarations are fully-qualified** using the `-package` prefix
2. **Directory structure mirrors packages** (Maven/Gradle convention)
3. **Cross-namespace imports are fully-qualified** with `{package}.{namespace}`
4. **Runtime stays in simple `pulserpc` package** (not under `-package` hierarchy)
5. **Build tools work seamlessly** - just add `{dir}` as a source directory

#### Java Runtime Structure:
```
pkg/runtime/runtimes/java/
├── pulserpc/                          # Runtime library
│   ├── RPCError.java                  # package pulserpc;
│   ├── Validation.java
│   └── Types.java
├── tests/
│   ├── ValidationTest.java
│   └── TypesTest.java
└── Makefile
```

#### Generated Files:
- `{package}/{namespace}/Idl.java` - Contains `ALL_STRUCTS` and `ALL_ENUMS` as static maps, package `{package}.{namespace}`
- `{package}/{namespace}/Server.java` - HTTP server with **embedded IDL JSON**, package `{package}.{namespace}`
- `{package}/{namespace}/Client.java` - Client classes, package `{package}.{namespace}`
- `{package}/{namespace}/{Type}.java` - Generated struct/enum classes, package `{package}.{namespace}`

#### Java Build Integration:
```xml
<!-- Maven: add generated source directory -->
<build>
  <sourceDirectory>src/main/java</sourceDirectory>
  <!-- Generated code is already in proper package structure -->
</build>

<!-- Gradle: add generated source directory -->
sourceSets {
  main {
    java {
      srcDir 'src/main/java'  # Packages auto-resolve
    }
  }
}
```

This guide should provide a comprehensive foundation for implementing new language runtimes. Refer to the Python implementation as a reference, and adapt the patterns to your target language's conventions and capabilities.


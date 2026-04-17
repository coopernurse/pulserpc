## ADDED Requirements

### Requirement: Python 2 server documentation page

The documentation SHALL provide a mini-quickstart for implementing PulseRPC servers in Python 2.7, enabling users on legacy Python 2 systems to understand code generation, runtime architecture, and web server integration.

#### Scenario: User generates Python 2 code

- **WHEN** user runs `pulserpc -plugin python-client-server -python-version 2 -dir ./output checkout.pulse`
- **THEN** the CLI generates `output/idl.json` and `output/pulserpc/*.py` runtime files
- **AND** no Python stub classes are generated (Python 2 generates only metadata + runtime)

#### Scenario: User implements a handler

- **WHEN** user creates a class with methods matching interface methods
- **AND** registers it with `server.add_handler("InterfaceName", MyHandler())`
- **THEN** the Server dispatches JSON-RPC calls to the handler methods

#### Scenario: User integrates with BaseHTTPServer

- **WHEN** user implements a `BaseHTTPRequestHandler.do_POST` that extracts the JSON-RPC request and calls `server.call(request_dict)`
- **THEN** the server processes the request and returns a JSON-RPC response

#### Scenario: Server validates requests and responses

- **WHEN** a request comes in with wrong parameter types
- **THEN** the Server returns a JSON-RPC error response with code -32602
- **AND** does not call the handler method

#### Scenario: Documentation example runs in Docker

- **WHEN** the quickstart test script runs in Docker with `moxel/python2` image
- **THEN** it generates Python 2 code, starts the server, runs a client request, and verifies the response

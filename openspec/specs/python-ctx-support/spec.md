## Requirements

### Requirement: Server.call accepts ctx parameter
The Python `Server.call()` method SHALL accept an optional `ctx` parameter as the second argument (default `None`). The `ctx` parameter SHALL be passed as the last positional argument to all JSON-RPC method handler invocations.

#### Scenario: Server.call with ctx passes ctx to handler
- **WHEN** `Server.call(req, ctx={"auth": "token123"})` is called
- **THEN** the handler method is invoked with `func(*params, ctx)` where `ctx={"auth": "token123"}`

#### Scenario: Server.call without ctx passes None to handler
- **WHEN** `Server.call(req)` is called (no ctx argument)
- **THEN** the handler method is invoked with `func(*params, ctx)` where `ctx=None`

#### Scenario: Handler can access ctx for transport metadata
- **WHEN** a handler method receives `ctx` as the last parameter
- **THEN** the handler can read transport-level metadata (headers, auth tokens) from `ctx` without polluting the request payload

### Requirement: Generated interface stubs include ctx parameter
The Python generator SHALL include `ctx=None` as the last parameter in all generated interface stub methods (both Python 2 and Python 3 output).

#### Scenario: Python3 generated stub has ctx=None
- **WHEN** the generator creates a `server.py` for Python 3
- **THEN** each abstract method signature includes `ctx=None` as the last parameter (e.g., `def method(self, param1, param2, ctx=None):`)

#### Scenario: Python2 generated stub has ctx=None
- **WHEN** the generator creates a `server.py` for Python 2 (if applicable)
- **THEN** each abstract method signature includes `ctx=None` as the last parameter

### Requirement: Generated test server implementations include ctx parameter
The Python generator SHALL include `ctx=None` as the last parameter in all generated test server method implementations.

#### Scenario: Test server handler accepts ctx
- **WHEN** the generator creates a `test_server.py`
- **THEN** each handler implementation method includes `ctx=None` as the last parameter

### Requirement: Quickstart examples demonstrate ctx usage
The quickstart Python examples SHALL include `ctx=None` in handler signatures and comments explaining that `ctx` can be used for transport-level values not suitable for the request body.

#### Scenario: Quickstart server handlers document ctx
- **WHEN** a user reads the quickstart server example
- **THEN** they see `ctx=None` in handler signatures and a comment explaining its purpose

#### Scenario: Quickstart documentation mentions ctx
- **WHEN** a user reads quickstart documentation/comments
- **THEN** they find an explanation that `ctx` is for transport-level metadata (headers, auth) not suitable for the request body

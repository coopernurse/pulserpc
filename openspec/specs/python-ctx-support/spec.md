## Requirements

### Requirement: Server.call accepts ctx parameter
The Python `Server.call()` method SHALL accept an optional `ctx` parameter as the second argument (default `None`). The `ctx` value SHALL be passed as the **first** positional argument to every JSON-RPC method handler invocation.

#### Scenario: Server.call with ctx passes ctx to handler
- **WHEN** `Server.call(req, ctx={"auth": "token123"})` is called
- **THEN** the handler method is invoked with `func(ctx, *params)` where `ctx={"auth": "token123"}`

#### Scenario: Server.call without ctx passes None to handler
- **WHEN** `Server.call(req)` is called (no ctx argument)
- **THEN** the handler method is invoked with `func(ctx, *params)` where `ctx=None`

#### Scenario: Handler can access ctx for transport metadata
- **WHEN** a handler method receives `ctx` as the first parameter after `self`
- **THEN** the handler can read transport-level metadata (headers, auth tokens) from `ctx` without polluting the request payload

### Requirement: Generated interface stubs include ctx parameter
The Python generator SHALL include `ctx` as the **first** parameter after `self` in all generated Python 3 abstract method signatures. It has no default value, so implementations must accept it.

#### Scenario: Python3 generated stub has ctx first
- **WHEN** the generator creates a `server.py` for Python 3
- **THEN** each abstract method signature is `def method(self, ctx, param1, param2):`

### Requirement: Generated test server implementations include ctx parameter
The Python generator SHALL include `ctx` as the first parameter after `self` in all generated test server method implementations.

#### Scenario: Test server handler accepts ctx
- **WHEN** the generator creates a `test_server.py`
- **THEN** each handler implementation method is `def method(self, ctx, ...):`

### Requirement: Quickstart examples demonstrate ctx usage
The quickstart Python examples SHALL show `ctx` as the first handler parameter and explain that it carries transport-level values not suitable for the request body.

#### Scenario: Quickstart server handlers document ctx
- **WHEN** a user reads the quickstart server example
- **THEN** they see `ctx` as the first parameter and a comment explaining its purpose

#### Scenario: Quickstart documentation mentions ctx
- **WHEN** a user reads quickstart documentation/comments
- **THEN** they find an explanation that `ctx` is for transport-level metadata (headers, auth) not suitable for the request body

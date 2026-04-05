## ADDED Requirements

### Requirement: Multi-namespace directory structure

When the IDL contains multiple namespaces and the `-package` flag is provided, the Python generator SHALL create a directory structure where each namespace has its own subdirectory.

#### Scenario: Multi-namespace output with package flag
- **WHEN** IDL has namespaces `common` and `orders`
- **AND** user runs with `-package myapp.lib.rpc`
- **THEN** output contains `myapp/lib/rpc/common/` and `myapp/lib/rpc/orders/` directories
- **AND** each contains `types.py`, `server.py`, `client.py`

### Requirement: __init__.py generation

The Python generator SHALL create `__init__.py` files in each namespace directory to make them importable Python packages.

#### Scenario: __init__.py with package base
- **WHEN** generating with `-package myapp.lib.rpc`
- **THEN** `myapp/lib/rpc/__init__.py` imports `RPCError, Server, Client, Contract, HttpTransport, InProcTransport` from `myapp.lib.rpc.pulserpc`
- **AND** `myapp/lib/rpc/common/__init__.py` exists but contains minimal content

### Requirement: Cross-namespace import paths

When generating multi-namespace output, types from other namespaces SHALL be imported using the proper relative Python import paths.

#### Scenario: Import from another namespace
- **WHEN** `common/types.py` defines a type used by `orders/types.py`
- **THEN** `orders/types.py` contains `from myapp.lib.rpc.common import TypeName`

### Requirement: Runtime import paths

The `-package` flag SHALL affect runtime imports so that the pulserpc runtime is importable from the correct package path.

#### Scenario: Runtime import with package base
- **WHEN** generating with `-package myapp.lib.rpc`
- **THEN** all generated files import from `myapp.lib.rpc.pulserpc`
- **AND** NOT from bare `pulserpc`

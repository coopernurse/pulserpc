## MODIFIED Requirements

### Requirement: Package flag creates nested directory structure for Python

The `-package` flag for the Python generator SHALL split the package value by dots and create a nested directory hierarchy. For example, `-package myapp.rpc` creates `myapp/rpc/` under the `-dir` path.

#### Scenario: Python generator with package flag containing dots
- **WHEN** user runs `pulserpc -plugin python-client-server -dir output -package myapp.rpc book.pulse`
- **THEN** runtime files are written to `output/myapp/rpc/pulserpc/`
- **AND** namespace files are written to `output/myapp/rpc/book/`
- **AND** `__init__.py` files are written to `output/myapp/`, `output/myapp/rpc/`, and `output/myapp/rpc/book/`

#### Scenario: Python generator with deeply nested package flag
- **WHEN** user runs `pulserpc -plugin python-client-server -dir output -package myapp.lib.rpc book.pulse`
- **THEN** runtime files are written to `output/myapp/lib/rpc/pulserpc/`
- **AND** namespace files are written to `output/myapp/lib/rpc/book/`

#### Scenario: Python cross-namespace imports with package
- **WHEN** namespace `book` references types from namespace `common`
- **AND** user specified `-package myapp.rpc`
- **THEN** generated code uses `from myapp.rpc.common import ...`
- **AND** `__init__.py` files enable the package imports

#### Scenario: Python imports remain relative within packages
- **WHEN** namespace `book` references runtime types like `RPCError`
- **AND** user specified `-package myapp.rpc`
- **THEN** generated code uses `from myapp.rpc.pulserpc import ...`

## ADDED Requirements

### Requirement: Package flag creates single directory level for most generators

The `-package` flag SHALL create a single directory level under `-dir`, not a nested hierarchy based on dot separators.

For a command like `pulserpc -dir foo -package myapp example.pulse`, the output directory structure SHALL be:
- Runtime files: `foo/myapp/pulserpc/`
- Namespace files: `foo/myapp/example/`
- IDL file: `foo/myapp/example/idl.json`

This applies to TypeScript, Go, Java, and C# generators.

#### Scenario: TypeScript generator with package flag
- **WHEN** user runs `pulserpc -plugin ts-client-server -dir output -package myapp book.pulse`
- **THEN** runtime files are written to `output/myapp/pulserpc/`
- **AND** namespace files are written to `output/myapp/book/`
- **AND** idl.json is written to `output/myapp/book/idl.json`

#### Scenario: Go generator with package flag
- **WHEN** user runs `pulserpc -plugin go-client-server -dir output -package myapp book.pulse`
- **THEN** runtime files are written to `output/myapp/pulserpc/`
- **AND** namespace files are written to `output/myapp/book/`

### Requirement: Package flag creates nested directory structure for Python

The Python generator SHALL split the `-package` flag value by dots to create a nested directory hierarchy, following Python package conventions.

For a command like `pulserpc -dir foo -package myapp example.pulse`, the output directory structure SHALL be:
- Runtime files: `foo/myapp/pulserpc/`
- Namespace files: `foo/myapp/example/`
- `__init__.py` files: `foo/myapp/__init__.py` and `foo/myapp/example/__init__.py`

For a dotted package like `pulserpc -dir foo -package myapp.lib.rpc example.pulse`:
- Runtime files: `foo/myapp/lib/rpc/pulserpc/`
- Namespace files: `foo/myapp/lib/rpc/example/`
- `__init__.py` files: `foo/myapp/__init__.py`, `foo/myapp/lib/__init__.py`, `foo/myapp/lib/rpc/__init__.py`, and `foo/myapp/lib/rpc/example/__init__.py`

#### Scenario: Python generator with package flag creates nested directories
- **WHEN** user runs `pulserpc -plugin python-client-server -dir output -package myapp book.pulse`
- **THEN** runtime files are written to `output/myapp/pulserpc/`
- **AND** namespace files are written to `output/myapp/book/`
- **AND** `__init__.py` files are written to `output/myapp/` and `output/myapp/book/`

#### Scenario: Python generator with dotted package flag splits by dots
- **WHEN** user runs `pulserpc -plugin python-client-server -dir output -package myapp.lib.rpc book.pulse`
- **THEN** runtime files are written to `output/myapp/lib/rpc/pulserpc/`
- **AND** namespace files are written to `output/myapp/lib/rpc/book/`
- **AND** `__init__.py` files are written to `output/myapp/`, `output/myapp/lib/`, `output/myapp/lib/rpc/`, and `output/myapp/lib/rpc/book/`

#### Scenario: No package flag specified
- **WHEN** user runs `pulserpc -plugin ts-client-server -dir output book.pulse` without `-package`
- **THEN** runtime files are written to `output/pulserpc/`
- **AND** namespace files are written to `output/book/`

### Requirement: IDL file placement per namespace

The `idl.json` file SHALL be placed inside the namespace subdirectory, not at the package root or `-dir` root.

#### Scenario: Single namespace IDL file
- **WHEN** user generates code from `book.pulse` which defines namespace `book`
- **THEN** `idl.json` is written to `{package-dir}/book/idl.json`

#### Scenario: Multiple namespaces from one file
- **WHEN** user generates code from `app.pulse` which defines namespaces `app`, `common`, and `user`
- **THEN** `idl.json` is written to `{package-dir}/app/idl.json` (primary namespace)
- **AND** `common/` and `user/` directories contain only types.ts, server.ts, client.ts

#### Scenario: Multiple pulse files in one project
- **WHEN** user runs pulserpc twice: once for `book.pulse` and once for `payment.pulse`
- **AND** both use the same `-dir` and `-package`
- **THEN** `book/idl.json` and `payment/idl.json` coexist without overwriting

### Requirement: Playground mode omits default package for non-Java generators

The playground (UI mode) SHALL NOT set a default `-package` value for TypeScript, Python, C#, or Go generators. Only Java SHALL have a default `-package` of `com.example.generated`.

#### Scenario: TypeScript playground generation
- **WHEN** user generates TypeScript code via the web UI
- **THEN** no `-package` flag is passed to the generator
- **AND** files are output to `{session-dir}/{namespace}/`

#### Scenario: Java playground generation
- **WHEN** user generates Java code via the web UI
- **THEN** `-package com.example.generated` is passed to the generator
- **AND** files are output to `{session-dir}/src/main/java/com/example/generated/{namespace}/`

### Requirement: Imports remain relative within packages

Generated import statements SHALL use relative paths regardless of the `-package` flag value.

#### Scenario: TypeScript cross-namespace imports
- **WHEN** namespace `book` references types from namespace `common`
- **THEN** generated code uses `import { X } from '../common/types'`
- **AND** NOT `import { X } from 'myapp/common/types'`

#### Scenario: TypeScript runtime imports
- **WHEN** namespace `book` references runtime types like `RPCError`
- **THEN** generated code uses `import { RPCError } from '../pulserpc/rpc'`
- **AND** NOT `import { RPCError } from 'myapp/pulserpc/rpc'`

#### Scenario: Python cross-namespace imports with package
- **WHEN** namespace `book` references types from namespace `common`
- **AND** user specified `-package myapp.rpc`
- **THEN** generated code uses `from myapp.rpc.common import ...`
- **AND** `__init__.py` files enable the package imports

### Requirement: Backward compatible single namespace behavior

When no `-package` is specified and there is only one namespace, the generator SHALL output files to the `-dir` root (not in a subdirectory).

#### Scenario: Single namespace without package flag
- **WHEN** user runs `pulserpc -plugin ts-client-server -dir output book.pulse`
- **AND** `book.pulse` defines only namespace `book`
- **THEN** runtime files are written to `output/pulserpc/`
- **AND** namespace files are written to `output/` directly (types.ts, server.ts, client.ts)
- **AND** idl.json is written to `output/idl.json`

#### Scenario: Multiple namespaces without package flag
- **WHEN** user runs `pulserpc -plugin ts-client-server -dir output app.pulse`
- **AND** `app.pulse` defines multiple namespaces
- **THEN** runtime files are written to `output/pulserpc/`
- **AND** each namespace is written to `output/<namespace>/`
- **AND** idl.json for primary namespace is at `output/<primary>/idl.json`
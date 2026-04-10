## ADDED Requirements

### Requirement: Python version flag selects runtime target

The `--python-version` flag SHALL control which runtime is used for code generation.

#### Scenario: Default python version is 3
- **WHEN** the generator is invoked without `--python-version`
- **THEN** the Python 3 runtime is used
- **AND** Python 3 files are generated

#### Scenario: Python 2.7 target
- **WHEN** the generator is invoked with `--python-version=2.7`
- **THEN** the Python 2 runtime is used
- **AND** Python 2 compatible files are generated

#### Scenario: Python 3 explicit
- **WHEN** the generator is invoked with `--python-version=3`
- **THEN** the Python 3 runtime is used

### Requirement: Python 2.7 output is minimal

When targeting Python 2.7, the generator SHALL produce minimal output.

#### Scenario: Py2 generates only idl.json and runtime
- **WHEN** the generator is invoked with `--python-version=2.7`
- **THEN** only `idl.json` is generated (no rpctypes.py, server.py, client.py)
- **AND** the Python 2 runtime files are copied to the output directory

#### Scenario: Py3 generates full output
- **WHEN** the generator is invoked without `--python-version` (or with `--python-version=3`)
- **THEN** `idl.json` is generated
- **AND** `rpctypes.py` is generated with dataclass definitions
- **AND** `server.py` is generated with abstract base classes
- **AND** `client.py` is generated with example code
- **AND** the Python 3 runtime files are copied to the output directory

### Requirement: Runtime files embedded and copied correctly

The generator SHALL embed Python 2 runtime files and copy them to output.

#### Scenario: Py2 runtime files embedded
- **WHEN** the Go binary is built
- **THEN** `runtimes/python2/pulserpc/*.py` files are embedded
- **AND** accessible via `runtime.GetRuntimeFiles("python2")`

#### Scenario: Py2 runtime copied to output
- **WHEN** the generator is invoked with `--python-version=2.7`
- **THEN** Python 2 runtime files are copied to `{outputDir}/pulserpc/`
- **AND** file permissions are set to 0644

### Requirement: IDL JSON format is unchanged

The generated `idl.json` SHALL be identical regardless of Python version target.

#### Scenario: IDL JSON matches PulseRPC schema
- **WHEN** `idl.json` is generated for Py2
- **THEN** it conforms to the same schema as Py3 output
- **AND** contains `rootNamespace`, `interfaces`, `structs`, `enums`, `errors`

### Requirement: Namespace handling is consistent

Multi-namespace IDL generation SHALL work identically for Py2 and Py3.

#### Scenario: Multi-namespace Py2 output
- **WHEN** generating from an IDL with namespaces `common` and `book`
- **AND** targeting Python 2.7
- **THEN** `idl.json` is written to the appropriate namespace directory
- **AND** runtime is copied once to `pulserpc/`
- **AND** no namespace-specific code files are generated

### Requirement: Package flag works with Py2

The `-package` flag SHALL work correctly with Python 2.7 target.

#### Scenario: Py2 with package flag
- **WHEN** generating with `--python-version=2.7 -package myapp`
- **THEN** runtime is copied to `{outputDir}/myapp/pulserpc/`
- **AND** `idl.json` is written to `{outputDir}/myapp/{namespace}/`
- **AND** `__init__.py` files are NOT generated (Py2 runtime uses no packages)

#### Scenario: Py2 with dotted package flag
- **WHEN** generating with `--python-version=2.7 -package myapp.lib`
- **THEN** runtime is copied to `{outputDir}/myapp/lib/pulserpc/`
- **AND** `idl.json` is written to `{outputDir}/myapp/lib/{namespace}/`

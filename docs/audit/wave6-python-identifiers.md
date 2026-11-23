# Wave 6 Audit: Python Imported-Interface Identifiers

Status: **CONFIRMED** — concrete repro generated from the committed fixture;
fix + regression test in progress.

## Scope

The parser assigns an imported interface declared in another `.pulse` file the
qualified IDL name `common.CommonOps` (fixture `examples/cross-ns/app.pulse`
imports `common.pulse`, in which `interface CommonOps` is declared in
`namespace common`). Go/C#/Java were fixed in earlier waves to use base-name
identifiers (`CommonOps`) in generated code while retaining the qualified name
in JSON-RPC wire strings (`common.CommonOps.Ping`). This audit confirms and
fixes the same class of bug in the Python generator.

## Reproduction method

Local Windows execution of a freshly built Go `.exe` is blocked by Windows
Application Control (`An Application Control policy has blocked this file`),
including `go test`. Workaround used: `GOOS=linux GOARCH=amd64 CGO_ENABLED=0
go build -o ./pulse-repro-linux ./cmd/pulse`, executed under WSL 2 through
`/mnt/host/c/...`, writing generated files to a Windows-visible temp directory;
`python3 -m py_compile` then runs natively on those files. This is a real
generator run, not static guesswork.

Commands:

```
pulse-repro-linux -plugin python-client-server -dir <out> examples/cross-ns/app.pulse
pulse-repro-linux -plugin python-client-server -generate-test-files -use-pydantic -dir <out2> examples/cross-ns/app.pulse
python3 -m py_compile <each generated .py>
```

## Findings

| Candidate | file:line | CONFIRMED / UNVERIFIED | Severity | Fix |
| --- | --- | --- | --- | --- |
| Abstract server class uses qualified name: `class common.CommonOps(abc.ABC):` (pre-fix `common/server.py:13`) | `pkg/generator/python_client_server.go:951` | **CONFIRMED** — generated file; `python3 -m py_compile` → `SyntaxError: invalid syntax` at that line | High (invalid Python syntax) | `pythonInterfaceIdent(iface)` = `GetBaseName(iface.Name)` |
| Client example comment uses qualified attribute path: `# result = client.common.CommonOps.Ping()` | `pkg/generator/python_client_server.go:367` | **CONFIRMED** — generated `common/client.py:13` | Medium (invalid attribute path in example) | Base-name identifier; qualified name retained only in wire strings |
| Test impl class/base uses qualified name: `class common.CommonOpsImpl(common.CommonOps):` | `pkg/generator/python_client_server.go:1118,1132-1133` | **CONFIRMED** — generated `common/test_server.py:12`; py_compile SyntaxError | High (invalid Python syntax) | Base-name identifiers for class name and base class |
| Registration key `add_handler("common.CommonOps", ...)` | `pkg/generator/python_client_server.go:1119` | **CONFIRMED correct** — runtime `server.py:257` splits wire method `common.CommonOps.Ping` with `rsplit('.', 1)` and looks up `iface_name == "common.CommonOps"` | Must keep qualified | Keep `iface.Name` as the registration key |
| Test client calls use qualified attribute path: `client.common.CommonOps.Ping()` / `.Echo()` | `pkg/generator/python_client_server.go:1452,1454,1463` | **CONFIRMED** — generated `common/test_client.py:43,53` | Medium (invalid attribute path) | Base-name identifiers |
| Pydantic model class names use qualified struct/enum names: `class common.Mode(BaseModel):`, `class common.Shared(BaseModel):` | `pkg/generator/python_client_server.go:760,797` | **CONFIRMED** — generated `models.py:31,47`; py_compile SyntaxError | High (invalid Python syntax with `-use-pydantic`) | `GetBaseName(...)` at class-name sites |
| Other qualified names in strings/comments: `# Test common.CommonOps.Ping`, `print("✓ common.CommonOps.Ping passed")`, docstrings | `pkg/generator/python_client_server.go:1134-1136,1439-1440,1493,1495` | **CONFIRMED correct** — these are wire method strings / IDL metadata, not identifiers | Keep | No change |
| `python2` generated code uses qualified identifiers | `pkg/generator/python_client_server.go:126-129` | **CONFIRMED not applicable** — plugin returns after writing `idl.json` + runtime files when `-python-version 2`; no `.py` code with interface identifiers is emitted (matches fixture audit note that qualified name appears in `idl.json` only). `python2` executable absent locally, but no code path exists to verify. | None | No fix |
| No regression test for imported-interface identifiers in Python | `pkg/generator` | **CONFIRMED** — no existing test generates the cross-ns fixture for Python | Medium | Add test using committed `examples/cross-ns/app.pulse` fixture |

## Verification Log

- Before fix, `python3 -m py_compile` failed on `models.py`, `common/server.py`,
  and `common/test_server.py` with `SyntaxError` on the qualified class names.
- Before fix, `common/client.py` and `common/test_client.py` contained the
  invalid dotted attribute path `client.common.CommonOps.*`.
- After fix, regenerating with `-generate-test-files -use-pydantic` produced
  base-name identifiers everywhere (`class CommonOps(abc.ABC):`,
  `class CommonOpsImpl(CommonOps):`, `client.CommonOps.Ping()`,
  `class Mode(BaseModel):`, `class Shared(BaseModel):`); `python3 -m py_compile`
  passed on **all 23 generated `.py` files**. The qualified registration key
  `add_handler("common.CommonOps", CommonOpsImpl())` and the qualified
  `"common.CommonOps.Ping"` wire strings survive.
- New test `TestCrossNamespaceFixturePythonIdentifiers` (in
  `pkg/generator/python_import_interface_regression_test.go`) **PASSED**. Native
  Windows `go test` is blocked by application control, so the package tests were
  compiled for Linux (`go test -c`, exit 0) and executed under WSL 2 from the
  `pkg/generator` directory with a `python3` shim that maps paths to the native
  Windows Python 3.11.9. The new test ran with a real `python3 -m py_compile`
  over every generated file. `go vet ./pkg/generator` and `gofmt -l` are clean.
- Full package test binary under WSL: all Python-related tests pass; unrelated
  failures are pre-existing/environmental (no `go`, no `JAVA_HOME`, no `bash`
  for `tsx` inside WSL, plus the already-documented Java unqualified
  `"CommonOps.Ping"` method-string bug). None touch the Python generator.
- python2: regenerated with `-python-version 2 -generate-test-files
  -use-pydantic`. Only `app/idl.json` (qualified `common.CommonOps`) and the
  copied python2 runtime files are emitted; no namespace `server.py` /
  `client.py` / `models.py` / test files exist, so there are no generated
  interface identifiers to fix. No python2 interpreter is installed locally;
  this is an environment limitation only, not a known code defect.

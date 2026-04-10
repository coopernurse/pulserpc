## Context

PulseRPC's existing Python 3 runtime uses Python 3-only features throughout:
- `f-strings` (string formatting)
- `@dataclass` decorator and `dataclasses` module
- `typing` module (Any, Dict, List, Optional, etc.)
- `abc.ABC` (abstract base class syntax)
- `urllib.request` (Python 3 stdlib)

A client needs to integrate with PulseRPC from a Python 2.7 system. Python 2 reached EOL January 2020 but legacy systems persist. The predecessor project Barrister was written in Python 2 and its runtime.py provides a proven pattern for JSON-RPC transport, server dispatch, and validation.

## Goals / Non-Goals

**Goals:**
- Provide a Python 2.7-compatible runtime library under `runtimes/python2/pulserpc/`
- Enable code generation for Py2 targets via `--python-version=2.7` flag
- Py2 output is minimal: only `idl.json` + runtime files. No code-generated types
- Reuse existing validation logic from Python 3 runtime (adapted to Py2 syntax)
- Transport, server, and client patterns borrowed from Barrister runtime.py
- Comprehensive tests for both generator and runtime

**Non-Goals:**
- Py2 quickstart documentation or examples
- One runtime supporting both Py2 and Py3 (separate runtimes by design)
- Migrating existing Py3 clients to Py2
- Supporting Python 2.6 or earlier

## Decisions

### Decision 1: Parallel Runtimes, Not Unified Runtime

**Choice**: Separate `runtimes/python/` (Py3) and `runtimes/python2/` (Py2) directories.

**Rationale**: Python 2 and 3 have incompatible syntax (f-strings, class syntax). A unified runtime would require constant conditional logic. Separate directories allow each to use its natural syntax without compromise.

**Alternatives Considered**:
- Single runtime with `six` compatibility layer: adds dependency, complicates both runtimes
- Single runtime targeting Py3, with Py2 using Barrister directly: loses PulseRPC-specific validation

### Decision 2: Minimal Code Generation for Py2

**Choice**: Py2 target generates only `idl.json` + copies runtime. No `rpctypes.py`, no `server.py`, no `client.py` stubs.

**Rationale**: The Py2 user writes plain Python 2 against the runtime validation API. They have their own patterns, frameworks, and HTTP handling. We provide validation, not scaffolding.

**Alternatives Considered**:
- Generate simple classes without dataclasses: possible, but Py2 users may not want generated classes
- Generate everything Py3 does, adapted to Py2 syntax: significant complexity for marginal benefit

### Decision 3: Reuse Py3 Validation Logic

**Choice**: Port `validation.py` and `types.py` from Py3 runtime, removing f-strings and typing annotations.

**Rationale**: The validation logic (type checking, struct validation, array validation, enum validation) is the same. Only syntax differs. This avoids rewriting complex validation logic.

**Alternatives Considered**:
- Reuse Barrister validation: Barrister uses positional params, PulseRPC uses named params - too different
- Write new validation from scratch: error-prone, ignores lessons learned from Py3 implementation

### Decision 4: Reuse Barrister Transport and Server

**Choice**: Port `HttpTransport`, `Server`, `Client`, `RequestContext`, `Filter` from Barrister runtime.py.

**Rationale**: Barrister's patterns are proven, Py2-compatible, and match PulseRPC's JSON-RPC 2.0 approach. The port is mostly line-by-line with `urllib2` instead of `urllib.request`.

**Alternatives Considered**:
- Adapt Py3 transport/server to Py2: Py3 version has f-strings, typing, abc.ABC everywhere - too much to strip
- Write new transport/server: unnecessary when Barrister pattern is proven

### Decision 5: Generator Flag `--python-version`

**Choice**: Add `--python-version=2.7` flag (default `3`) to select runtime.

**Rationale**: Follows existing flag pattern in the codebase. Users explicitly opt into Py2, avoiding accidental Py2 output.

## Decisions Detail

### Runtime File Structure

```
runtimes/python2/pulserpc/
├── __init__.py          # Package exports
├── rpc.py               # RpcException, error codes (from Barrister)
├── contract.py          # Contract class (adapted from Py3)
├── validation.py        # validate_* functions (adapted from Py3)
├── types.py             # find_struct, find_enum (from Py3)
├── server.py            # Server + RequestContext + Filter (from Barrister)
├── client.py            # Client + InterfaceClientProxy + Batch (from Barrister)
└── transport.py         # HttpTransport (urllib2) + InProcTransport (from Barrister)
```

### Generator Changes

In `python_client_server.go`:
1. Add `pythonVersion` field (default `"3"`)
2. Add `--python-version` flag registration
3. In `Generate()`:
   - If `pythonVersion == "2.7"`: call `copyRuntimeFiles(paths, "python2")` and generate only `idl.json`
   - If `pythonVersion == "3"`: existing behavior

### Embed Changes

In `embed.go`:
1. Add `//go:embed all:runtimes/python2/pulserpc` directive
2. Add `python2RuntimeFiles` variable
3. Add `"python2": python2RuntimeFiles` to `runtimeMap`

### Validation.py Porting Rules

| Py3 Pattern | Py2 Replacement |
|-------------|-----------------|
| `f"Expected {x}"` | `"Expected %s" % x` |
| `isinstance(x, str)` | `isinstance(x, basestring)` (add `from past.builtins import basestring` or use `isinstance(x, (str, unicode))`) |
| `isinstance(x, (int, float))` | `isinstance(x, (int, long, float))` |
| `typing` imports | Remove all |
| Type annotations `x: int` | Remove all |

### Contract.py Adaptations

The existing Py3 `contract.py` already handles both Barrister (list-based) and PulseRPC (dict-based) IDL formats. Minor adaptation needed:
- Remove `typing` imports
- Replace f-strings with `%` formatting
- Ensure `validate_request` bridges named ↔ positional params (Py3 version already has this)

### Server.py Dispatch

Py3 `server.py` (lines 108-137) already handles both positional and named params. Port the param normalization logic to Py2.

## Risks / Trade-offs

| Risk | Mitigation |
|------|------------|
| Py2 validation edge cases differ from Py3 | Extensive test coverage in `test_validation.py` |
| urllib2 behavior differs from urllib.request | Test HTTP transport with actual server |
| Py2 string/bytes ambiguity | Explicit UTF-8 encoding throughout |
| Client may not use InProcTransport | Test both HttpTransport and InProcTransport |
| Generator flag defaults to Py3 | Document `--python-version` flag clearly |

## Open Questions

1. **simplejson fallback**: Barrister used `try: import json; except: import simplejson`. Should Py2 runtime include this fallback, or assume `json` is available?
2. **Error handling parity**: Py3 raises `TypeError`/`ValueError`, Barrister raises `RpcException`. Should Py2 runtime use `RpcException` for validation errors too?
3. **Batch requests**: Barrister supports batch requests. Py3 implementation does not. Include in Py2?

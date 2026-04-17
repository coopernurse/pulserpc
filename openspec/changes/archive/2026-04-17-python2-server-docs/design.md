## Context

The Python 2 runtime exists (embedded in the CLI binary) and is tested via `make test-runtime-python2` using Docker with the `moxel/python2` image. However, there is no documentation showing users how to use it.

Users on legacy Python 2 systems need to:
1. Know how to generate Python 2-targeted code
2. Understand what gets generated (no stubs, just idl.json + runtime)
3. Write handlers that integrate with their existing web server

The Python 3 quickstart uses `python3` command, generated stub classes, and Flask. Python 2 needs different examples using stdlib.

## Goals / Non-Goals

**Goals:**
- Document how to generate PulseRPC code for Python 2 target
- Explain the runtime architecture (Server, Contract, handlers)
- Show how to integrate with Python 2 stdlib `BaseHTTPServer`
- Provide a testable example that runs in Docker

**Non-Goals:**
- Client documentation (user mentioned no Python 2 client plans)
- IDL reference (covered elsewhere)
- Python 3 content (separate docs)
- Quickstart-style tutorial with all the explanatory depth of Python 3 quickstart

## Decisions

**1. Use `checkout.pulse` for the example**

Using the same IDL as Python 3 quickstart provides consistency. The example focuses on wiring, not domain complexity. Users can reference Python 3 quickstart for full domain understanding.

**2. Use stdlib `BaseHTTPServer` for HTTP integration**

Python 2 doesn't have Flask in common deployments. Using stdlib demonstrates that PulseRPC integrates with "any" web server - you just call `server.call(request_dict)` from your HTTP handler.

**3. Jekyll include for server code (same pattern as other quickstarts)**

Following the existing pattern from `docs/_includes/quickstart/python-server.md` ensures consistency and allows the example to be tested.

**4. Docker test similar to existing Python 2 runtime test**

The existing `make test-runtime-python2` tests validation only. The new test will test the full server+client flow similar to `test_quickstart_python.sh`.

## Risks / Trade-offs

| Risk | Mitigation |
|------|------------|
| Python 2 stdlib HTTP server is single-threaded | Note in docs that for production, use with a proper WSGI server or wrapper |
| `moxel/python2` image becomes unavailable | Low risk - image is stable, can be replaced if needed |
| Example drifts from reality | Test in Docker ensures example works |

## Open Questions

- Should we extract the server code to `examples/quickstart/python2/` like other languages? Or keep it inline in the include?

# Agent Instructions

## Project

PulseRPC is a JSON-RPC 2.0 system with an IDL (`.pulse` files), a Go parser and
code generator, and client/server runtimes for Go, Java, Python, TypeScript,
and C#.

- [CLAUDE.md](CLAUDE.md) — architecture, build/test commands, conventions, pitfalls
- [.github/copilot-instructions.md](.github/copilot-instructions.md) — implementation detail

## Quick commands

```bash
make build            # build binary + web UI → target/pulserpc
make test             # Go unit tests
make test-runtimes    # runtime library tests
make test-generators  # end-to-end generator tests (requires Docker)
make lint             # golangci-lint
```

## Conventions

- Commits follow Conventional Commits with a required kebab-case scope, e.g. `fix(ts): ...`
- Every non-empty IDL file must declare a namespace
- Web UI changes require `cd pkg/webui && npm run build` before `make build`
- Integration tests need Docker and use `examples/conform.pulse`

---
title: CLI Reference
parent: Get Started
nav_order: 2
layout: default
---

# PulseRPC CLI Reference

The `pulserpc` command-line interface supports multiple modes of operation for parsing IDL files, generating code, and running the web UI.

## Usage

```bash
pulserpc [flags] <idl-file>
```

## Global Flags

### Core Flags

| Flag | Type | Description |
|------|------|-------------|
| `-validate` | bool | Validate the IDL after parsing. Checks for semantic errors like undefined types, missing namespaces, etc. |
| `-to-json` | string | Write parsed IDL as JSON to the specified file. Useful for debugging or inspecting the parsed AST. |
| `-from-json` | string | Read a JSON file and generate IDL text on STDOUT. The reverse of `-to-json`. |
| `-plugin` | string | Code generation plugin to use (e.g., `python-client-server`, `go-client-server`). |
| `-ui` | bool | Start the embedded web UI server for interactive testing and code generation. |
| `-ui-port` | int | Port for the web UI server (default: `8080`). |

### Output Directory Flags

| Flag | Type | Description |
|------|------|-------------|
| `-dir` | string | Output directory for generated code and runtime files. Required when using `-plugin`. |
| `-silent` | bool | Suppress file generation output. Useful for scripting or when output is too verbose. |
| `-generate-test-files` | bool | Generate test files (`test_server.*`, `test_client.*`) along with regular output. |

## Code Generation Plugins

### Available Plugins

| Plugin | Description |
|--------|-------------|
| `python-client-server` | Generate Python client and server code |
| `go-client-server` | Generate Go client and server code |
| `java-client-server` | Generate Java client and server code |
| `ts-client-server` | Generate TypeScript client and server code |
| `csharp-client-server` | Generate C# client and server code |

### Plugin-Specific Flags

#### Python Plugin (`python-client-server`)

| Flag | Type | Description |
|------|------|-------------|
| `-base-dir` | string | Base directory for namespace packages/modules. Defaults to `-dir` if not specified. |

#### Go Plugin (`go-client-server`)

| Flag | Type | Description |
|------|------|-------------|
| `-base-dir` | string | Base directory for namespace packages. Defaults to `-dir` if not specified. |
| `-go-module` | string | Override Go module path for pulserpc imports. Auto-detected from `go.mod` if not specified. |
| `-inline-runtime` | bool | Place runtime files inline with generated code. Useful for playground/testing environments. |

#### Java Plugin (`java-client-server`)

| Flag | Type | Description |
|------|------|-------------|
| `-base-dir` | string | Base directory for namespace package directories. Defaults to `-dir` if not specified. |
| `-base-package` | string | Base package name for generated Java classes. Required (e.g., `com.example.server`). |
| `-json-lib` | string | JSON library to use: `jackson` (default) or `gson`. |

#### TypeScript Plugin (`ts-client-server`)

| Flag | Type | Description |
|------|------|-------------|
| `-package` | string | Package prefix for generated types and classes. Used for namespace isolation. |
| `-base-dir` | string | Base directory for namespace packages. Defaults to `-dir` if not specified. |

#### C# Plugin (`csharp-client-server`)

| Flag | Type | Description |
|------|------|-------------|
| `-base-dir` | string | Base directory for namespace directories. Defaults to `-dir` if not specified. |

## Common Usage Examples

### Validate an IDL File

Check your IDL file for syntax and semantic errors:

```bash
pulserpc -validate api/service.pulse
```

### Pretty-Print an IDL File

Display a human-readable summary of interfaces, structs, and enums:

```bash
pulserpc api/service.pulse
```

### Convert IDL to JSON

Export the parsed AST as JSON for inspection or debugging:

```bash
pulserpc -to-json service.json api/service.pulse
```

### Generate Python Code

Generate Python client and server code:

```bash
pulserpc -plugin python-client-server -dir ./output api/service.pulse
```

### Generate Java Code with Custom Package

Generate Java code with a specific base package:

```bash
pulserpc -plugin java-client-server -dir ./src/main/java \
  -base-package com.example.api api/service.pulse
```

### Generate Go Code with Inline Runtime

Generate Go code with runtime files embedded (useful for testing):

```bash
pulserpc -plugin go-client-server -dir ./output -inline-runtime api/service.pulse
```

### Generate TypeScript Code with Package Prefix

Generate TypeScript with a namespace prefix:

```bash
pulserpc -plugin ts-client-server -dir ./src -package '@mycompany/api' api/service.pulse
```

### Generate Test Files

Generate additional test files for integration testing:

```bash
pulserpc -plugin python-client-server -dir ./output -generate-test-files api/service.pulse
```

This creates `test_server.py` and `test_client.py` in addition to the normal output files.

### Silent Mode

Suppress file generation output:

```bash
pulserpc -plugin go-client-server -dir ./output -silent api/service.pulse
```

### Start the Web UI

Launch the interactive web UI on a custom port:

```bash
pulserpc -ui -ui-port 9090
```

Then open http://localhost:9090 in your browser.

## Using with Docker

When running PulseRPC via Docker, mount your working directory and pass flags as usual:

```bash
docker run --rm -v $(pwd):/work ghcr.io/coopernurse/pulserpc:latest \
  -plugin python-client-server -dir ./output api/service.pulse
```

Start the web UI with Docker:

```bash
docker run --rm -p 8080:8080 -v $(pwd):/work ghcr.io/coopernurse/pulserpc:latest -ui
```

## Flag Mutually Exclusivity

Some flag combinations are not allowed:

- `-to-json` and `-from-json` cannot be used together
- `-plugin` cannot be used with `-to-json` or `-from-json`
- `-ui` mode ignores other flags and starts the web UI immediately

## Getting Help

Display all available flags and their descriptions:

```bash
pulserpc -h
```

For plugin-specific flags, use the `-h` flag with a plugin:

```bash
pulserpc -plugin java-client-server -h
```

## Next Steps

- [Quickstart Overview](quickstart-overview.html) - Learn what you'll build
- [IDL Syntax](../idl-guide/syntax.html) - IDL language reference

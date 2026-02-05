---
title: OpenAPI Translation
parent: Get Started
nav_order: 4
layout: default
---

# OpenAPI Translation

PulseRPC provides bidirectional translation between OpenAPI specifications and Pulse IDL files. This enables you to:

- **Import existing REST APIs** - Convert OpenAPI/Swagger specs to Pulse IDL and generate type-safe JSON-RPC clients
- **Export Pulse services** - Generate OpenAPI specs from your Pulse IDL for REST tooling compatibility

## Installation

The OpenAPI translator is built into PulseRPC. Ensure you have PulseRPC installed:

```bash
# Install via Go
go install github.com/coopernurse/pulserpc/cmd/pulse@latest

# Or use Docker
docker pull ghcr.io/coopernurse/pulserpc:latest
```

## OpenAPI to Pulse IDL

Convert an existing OpenAPI specification to a Pulse IDL file:

```bash
pulserpc -openapi-to-pulse <openapi-spec.yaml> -output-dir ./output
```

### Example: Petstore API

```bash
pulserpc -openapi-to-pulse petstore.yaml -output-dir ./generated
```

This creates `petstore.pulse` with:

- **Namespace** derived from `info.title`
- **Structs** for each schema in `components/schemas`
- **Interfaces** grouped by operation tags
- **Methods** named with HTTP verb prefix (e.g., `getUsers`, `createUser`)
- **Enums** for string schemas with `enum` values

### Supported OpenAPI Versions

- **OpenAPI 3.0.x** - Full support
- **OpenAPI 3.1.x** - Full support
- **Swagger 2.0** - Not supported (use OpenAPI 3.x converter first)

## Pulse IDL to OpenAPI

Convert a Pulse IDL file to an OpenAPI specification:

```bash
pulserpc -pulse-to-openapi <service.pulse> -output-dir ./output
```

### Example: Exporting a Service

```bash
pulserpc -pulse-to-openapi checkout.pulse -output-dir ./specs
```

This generates `checkout.openapi.yaml` with:

- **OpenAPI 3.1 specification** by default
- **POST endpoints** for each interface method: `POST /{interface}/{method}`
- **Request schemas** in the request body
- **Response schemas** with proper status codes
- **Tags** corresponding to Pulse interfaces

## CLI Flags

### OpenAPI Translation Flags

| Flag | Type | Description |
|------|------|-------------|
| `-openapi-to-pulse` | string | Path to OpenAPI spec (YAML or JSON) to convert to Pulse IDL |
| `-pulse-to-openapi` | string | Path to Pulse IDL file to convert to OpenAPI spec |
| `-output-dir` | string | Output directory for generated files (default: `./generated`) |
| `-openapi-version` | string | Target OpenAPI version: `3.0` or `3.1` (default: `3.1`) |
| `-strict` | bool | Treat warnings as errors (non-zero exit code on warnings) |

### Output Files

| Input | Output File |
|-------|-------------|
| `petstore.yaml` | `petstore.yaml.pulse` |
| `api.yaml` | `api.yaml.pulse` |
| `service.pulse` | `service.openapi.yaml` |

Generated file names preserve the original filename with appropriate extension.

## Quick Start Examples

### Import an External API

Start using an external API with type safety:

```bash
# 1. Convert the OpenAPI spec
pulserpc -openapi-to-pulse github-api.yaml -output-dir ./src

# 2. Generate code for your language
pulserpc -plugin python-client-server -dir ./client src/github-api.yaml.pulse

# 3. Use the generated client with full type safety
```

### Document Your Pulse Service

Generate OpenAPI documentation for your Pulse service:

```bash
# 1. Generate OpenAPI spec
pulserpc -pulse-to-openapi myservice.pulse -output-dir ./docs

# 2. Use with OpenAPI tools
# - Swagger UI for interactive docs
# - Spectral for linting
# - OpenAPI Generator for additional client types
```

## Type Mapping

### OpenAPI to Pulse

| OpenAPI Type | Pulse Type |
|--------------|------------|
| `string` | `string` |
| `integer` (int32/int64) | `int` |
| `number` (float/double) | `float` |
| `boolean` | `bool` |
| `array` | `[]Type` |
| `object` with properties | `struct` |
| `object` with additionalProperties | `map[string]Type` |
| `enum` (strings) | `enum` |
| `allOf` | `extends` keyword |
| `nullable: true` | `[optional]` |

### Pulse to OpenAPI

| Pulse Type | OpenAPI Type |
|------------|--------------|
| `string` | `type: string` |
| `int` | `type: integer, format: int64` |
| `float` | `type: number, format: double` |
| `bool` | `type: boolean` |
| `[]Type` | `type: array, items: {...}` |
| `map[string]Type` | `type: object, additionalProperties: {...}` |
| `enum` | `type: string, enum: [...]` |
| `extends` | `allOf: [...]` |
| `[optional]` | Not in `required` array |

See the [OpenAPI Mapping Reference](../advanced/openapi-mapping-reference.html) for complete details.

## Known Limitations

### OpenAPI Features Not Supported

| Feature | Behavior |
|---------|----------|
| `oneOf` / `anyOf` | Warning issued; first type or `string` fallback used |
| Header parameters | Dropped with warning |
| Cookie parameters | Dropped with warning |
| Multiple response codes | Only 200/201/204 preserved; others dropped with warning |
| File uploads / `binary` format | Error; not supported |
| Security schemes | Dropped with warning |
| `discriminator` | Dropped; no Pulse equivalent |
| Default values | Dropped; no Pulse equivalent |
| Examples | Dropped |
| Path parameters | Become method parameters (named, not positional) |

### Pulse Features Not Mapped

| Feature | OpenAPI Behavior |
|---------|------------------|
| Method overloading | Each method becomes separate POST endpoint |
| Custom error codes | Not represented in OpenAPI |
| Multiple namespaces | Single OpenAPI spec with interface tags |

## Warnings and Error Handling

The translator emits warnings for unsupported features:

```bash
$ pulserpc -openapi-to-pulse api.yaml -output-dir ./output

Warning: oneOf encountered at #/components/schemas/User (using first type)
Warning: security scheme 'oauth2' dropped (security schemes not supported)
Warning: response code 404 dropped (only 2xx codes supported)
Generated Pulse IDL from api.yaml
Output file: ./output/api.yaml.pulse
```

Use the `-strict` flag to treat warnings as errors:

```bash
pulserpc -openapi-to-pulse api.yaml -output-dir ./output -strict
# Exit code: 1 if any warnings
```

## Docker Usage

```bash
# Convert OpenAPI to Pulse
docker run --rm -v $(pwd):/work ghcr.io/coopernurse/pulserpc:latest \
  -openapi-to-pulse api.yaml -output-dir ./output

# Convert Pulse to OpenAPI
docker run --rm -v $(pwd):/work ghcr.io/coopernurse/pulserpc:latest \
  -pulse-to-openapi service.pulse -output-dir ./specs
```

## Next Steps

- [OpenAPI Mapping Reference](../advanced/openapi-mapping-reference.html) - Complete type mapping details
- [IDL Syntax](../idl-guide/syntax.html) - Pulse IDL language reference
- [CLI Reference](cli-reference.html) - Full CLI documentation

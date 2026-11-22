# PulseRPC Java Runtime

This directory contains the Java runtime library for PulseRPC, providing validation, type utilities, and JSON parsing abstractions.

## Components

### Core Classes

- **RPCError.java**: Exception class for JSON-RPC 2.0 errors
- **Validation.java**: Static validation methods for all PulseRPC types
- **Types.java**: Helper methods for type operations and inheritance resolution
- **HttpLimits.java**: Canonical request/response/batch limits and HTTP timeouts

### JSON Parser Abstraction

- **JsonParser.java**: Interface for JSON parsing operations
- **JacksonJsonParser.java**: Jackson-based implementation
- **GsonJsonParser.java**: GSON-based implementation

## Dependencies

The runtime library itself has no external dependencies. Generated code requires either:

- **Jackson**: `com.fasterxml.jackson.core:jackson-databind:2.15.2`
- **GSON**: `com.google.code.gson:gson:2.10.1`

## Java Version

Requires Java 11+ for modern HTTP client and server APIs.

## Testing

Run tests with:

```bash
make test
```

Or individual test suites:

```bash
make test-validation
make test-types
make test-rpc
make test-json
```

## Integration Testing

For full integration tests including HTTP server/client:

```bash
make test-integration
```

This requires Maven and a Java 11+ runtime.

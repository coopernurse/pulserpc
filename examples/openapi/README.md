# OpenAPI Translation Examples

This directory contains example files demonstrating the OpenAPI ↔ Pulse IDL translation feature.

## Files

### petstore.yaml
A standard Petstore OpenAPI 3.0 specification. This is based on the canonical example from the OpenAPI specification.

### petstore.pulse
Expected Pulse IDL output when converting `petstore.yaml` to Pulse IDL.

To regenerate:
```bash
pulserpc -openapi-to-pulse petstore.yaml -output-dir .
```

### simple-api.pulse
A simple Pulse IDL file demonstrating basic IDL features for round-trip testing.

### simple-api.openapi.yaml
Expected OpenAPI 3.1 specification when converting `simple-api.pulse` to OpenAPI.

To regenerate:
```bash
pulserpc -pulse-to-openapi simple-api.pulse -output-dir .
```

## Usage

### Convert OpenAPI to Pulse IDL

```bash
# From this directory
pulserpc -openapi-to-pulse petstore.yaml -output-dir .

# Or specify full paths
pulserpc -openapi-to-pulse ./petstore.yaml -output-dir ./output
```

### Convert Pulse IDL to OpenAPI

```bash
# From this directory
pulserpc -pulse-to-openapi simple-api.pulse -output-dir .

# Or specify full paths
pulserpc -pulse-to-openapi ./simple-api.pulse -output-dir ./output
```

## Round-Trip Testing

To test round-trip conversion (OpenAPI → Pulse → OpenAPI):

```bash
# Step 1: Convert OpenAPI to Pulse
pulserpc -openapi-to-pulse petstore.yaml -output-dir ./roundtrip

# Step 2: Convert the generated Pulse back to OpenAPI
pulserpc -pulse-to-openapi ./roundtrip/petstore.yaml.pulse -output-dir ./roundtrip

# Compare the original and round-trip specs
diff petstore.yaml ./roundtrip/petstore.yaml.pulse.openapi.yaml
```

Note: Due to inherent differences between REST and RPC paradigms, round-trip conversion will not be 100% identical. The generated OpenAPI spec will use POST endpoints for all methods and wrap parameters in a request body.

## Known Differences

### OpenAPI → Pulse → OpenAPI

When converting from OpenAPI to Pulse and back:
- All HTTP methods (GET, POST, PUT, DELETE) become POST endpoints
- Path parameters become method parameters (no positional parameters in the URL)
- Query parameters become method parameters
- Header/cookie parameters are dropped with a warning
- Multiple response codes beyond 2xx are dropped
- Security schemes are dropped

### Pulse → OpenAPI → Pulse

When converting from Pulse to OpenAPI and back:
- POST endpoints may have method names like `postGetUser` (verb + original name)
- Request body parameters become individual method parameters
- The interface grouping may differ if tags weren't used in the original OpenAPI spec

## See Also

- [OpenAPI Translation Guide](../../docs/get-started/openapi-translation.md)
- [OpenAPI Mapping Reference](../../docs/advanced/openapi-mapping-reference.md)

## Why

A Go service sharing one generated server across multiple RPC methods can require different public codes for the same kind of invalid parameter. The current error mapper only receives the phase and error, so method-specific codes require request inspection in middleware.

## What Changes

Add a method-aware parameter error mapper to the Go server configuration. Preserve the existing mapper and JSON-RPC defaults.

## Capabilities

### Modified Capabilities

- `go-server-options`: Select parameter error mappings by qualified RPC method.

## Impact

The Go dispatcher and generated server adapters honor the option. The zero value preserves existing behavior and the IDL and other languages are unchanged.

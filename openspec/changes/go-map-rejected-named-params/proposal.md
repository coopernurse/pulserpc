## Why

A positional-only Go RPC server can need its existing method-specific errors for object-shaped parameters. The current positional-only policy always returns the standard protocol error.

## What Changes

Add an opt-in positional-only mode that lets parameter error mappers handle rejected named parameters.

## Capabilities

### Modified Capabilities

- `go-server-options`: Opt into mapping errors for rejected object parameters.

## Impact

Default and existing positional-only modes are unchanged. The new mode keeps the standard response when no mapper handles the error. This change MODIFIES a `go-server-options` requirement introduced by `add-go-server-options`, so archive that change first.

## Why

The Go runtime can round a decimal or exponent token before integer validation. Numeric tokens preserved as json.Number can also become nil when dispatch converts an optional field.

## What Changes

Parse mathematically integral JSON tokens exactly within the signed 64-bit range. Keep ordinary float values available to middleware and pass preserved numbers to optional numeric fields.

## Capabilities

### Modified Capabilities

- `go-server-options`: Keep wide and decimal/exponent numbers exact through request normalization and response decoding.

## Impact

The Go runtime, generated clients, and generated servers adopt the exact number view. Middleware must handle `json.Number` for values outside the safe-integer range. The IDL and other language runtimes are unchanged. This change MODIFIES a `go-server-options` requirement introduced by `add-go-server-options`, so archive that change first.

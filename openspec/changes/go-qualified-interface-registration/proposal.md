## Why

A Go server may need to retain an existing qualified wire name when the generated contract supplies the interface's name and namespace separately.

## What Changes

Resolve qualified interface names from the declared namespace during Go contract lookup and server registration.

## Capabilities

### Modified Capabilities

- `go-server-options`: Register a generated interface directly by its qualified IDL name.

## Impact

The Go server and contract validator accept an explicitly registered qualified name. Bare names and other language runtimes are unchanged.

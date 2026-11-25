## Why

Some Go request contracts use a default when a field is omitted and reject an explicit JSON null. Current optional IDL fields allow both cases.

## What Changes

Add an optional Go server validation callback for explicit null request struct fields. It selects fields by declared struct name and exact wire key.

## Capabilities

### Modified Capabilities

- `go-server-options`: Select optional request struct fields that cannot be null.

## Impact

Go servers can enable this policy without changing the IDL or response validation. Existing defaults and other runtimes are unaffected.

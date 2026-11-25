## Why

Go request decoding accepts native struct field aliases as well as JSON tags. An alias can avoid checks expressed in the IDL, including checks on optional fields.

## What Changes

Add an opt-in Go server policy that accepts exact IDL field names in request structs. Keep the existing unknown-field option and defaults.

## Capabilities

### Modified Capabilities

- `go-server-options`: Require canonical IDL field names in requests.

## Impact

The Go contract validator applies the policy to request structs only. Ordinary map keys, response validation and other language runtimes are unchanged.

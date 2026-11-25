## Why

The Go runtime can round a decimal or exponent token before integer validation. Numeric tokens preserved as json.Number can also become nil when dispatch converts an optional field.

## What Changes

Parse mathematically integral JSON tokens exactly within the signed 64-bit range. Keep ordinary float values available to middleware and pass preserved numbers to optional numeric fields.

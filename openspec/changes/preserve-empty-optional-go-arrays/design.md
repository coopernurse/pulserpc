## Decision

Tag opted-in optional Go slice fields with `omitzero`. This preserves existing Go field types and leaves IDL metadata unchanged. The flag takes precedence over `-omit-empty` for optional slices.

The Go validator already delegates unfamiliar JSON tags to `encoding/json`. Decode its serialized bytes with `UseNumber` and retain the runtime's small-number view so the fallback preserves wide signed integers.

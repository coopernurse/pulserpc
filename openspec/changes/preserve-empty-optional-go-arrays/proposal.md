## Why

Some optional arrays distinguish an absent field from an explicitly empty array. Go's generated `omitempty` tag omits both.

## What Changes

- Add an opt-in Go generator flag, `-preserve-empty-optional-arrays`.
- Preserve signed 64-bit integers when the Go response validator takes its JSON fallback.
- Document the Go 1.24 minimum for code generated with the flag. Default generation keeps its existing tags.

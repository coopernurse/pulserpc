## Context

The Go dispatcher resolves the method before checking and decoding its parameters.

## Goals / Non-Goals

- Allow one configured server to select method-specific errors after normal parameter checks.
- Keep envelope protocol errors and handler errors outside this option.

## Decisions

An optional `MapMethodError(method, phase, err)` receives the method used for dispatch. Returning nil falls through to `MapError`, then to the existing standard error. Existing callbacks retain their signature and precedence when the new option is unset.

## Context

Request contract validation has the original decoded keys and a resolved definition for each request struct.

## Goals / Non-Goals

Reject aliases and unknown keys, including in nested and inherited structs. Preserve existing behavior when the option is disabled.

## Decisions

Pass a request-only policy to the existing recursive validation. Compare against resolved IDL field definitions, and report unknown keys in the validation error phase. Ordinary maps still validate only their values.

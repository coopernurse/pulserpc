## Context

The runtime resolves the registered version of the method before applying its parameter-shape policy.

## Goals / Non-Goals

Allow existing decode error mappers to handle named-parameter rejection explicitly. Keep malformed JSON-RPC envelopes outside that policy.

## Decisions

`ParamsModePositionalOnlyMapped` uses the parameter decode error path for a named object and supplies the existing invalid-request error as the fallback.

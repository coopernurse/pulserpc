## Context

The contract validator already distinguishes a missing key from one with a null value before Go argument hydration.

## Goals / Non-Goals

Preserve omission while rejecting selected explicit null request fields. Existing response, client, and standalone contract validation semantics stay in place.

## Decisions

Only server request validation passes the optional policy callback. It receives the resolved struct's declared name and exact field key only for optional explicit null fields. Rejections use the ordinary validation phase and error mapping.

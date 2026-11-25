## Context

Generated Go servers validate registration using the contract and dispatch by registered method prefix.

## Goals / Non-Goals

Use the wire name directly without application middleware rewriting. Keep existing bare registrations and do not implicitly expose a second spelling.

## Decisions

Index qualified names when the IDL declares a short name and a namespace. Exact declared keys win. Dispatch still uses only the name passed to registration.

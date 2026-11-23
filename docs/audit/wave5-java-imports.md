# Wave 5 Java Imports Audit

## Goal
Determine whether generated Java code has a bug analogous to the Go `generateStructTypesGo` bug: a type from a shared/imported namespace referenced in a struct field or method signature but emitted without its package qualifier, or a wrong/unused import, causing compile failure.

Shared namespace used for reproduction: `common` (NOT `inc`).

## Case Matrix

| case | generated file:line | emitted type/import | expected | compile result | status |
|------|--------------------|--------------------|----------|----------------|--------|
| baseline generate (no -package) | TBD | TBD | TBD | TBD | TBD |
| generate with -package com.example.rpc | TBD | TBD | TBD | TBD | TBD |
| shared struct field type qualifier | TBD | TBD | TBD | TBD | TBD |
| interface method signature type | TBD | TBD | TBD | TBD | TBD |
| import present/used | TBD | TBD | TBD | TBD | TBD |
| hardcoded namespace (inc) check | TBD | TBD | TBD | TBD | TBD |

## Findings
TBD

## Residual
TBD

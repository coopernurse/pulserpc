# Wave 5 — C# cross-namespace import emission audit

Target: generated C# from `csharp-client-server` plugin with a shared namespace `common`.
Concern: is a type from an imported namespace referenced in a field/property or method
signature but emitted without its namespace qualifier (analogous to the Go
`generateStructTypesGo` bug), or are `using` directives wrong/unused, causing compile failure?

Go bug reference: `generateStructTypesGo` used a non-namespace-aware type mapper emitting
`Thing Thing` while importing the shared package -> `undefined: Thing` + unused import (fixed).
A separate C# `-package` namespace bug was just fixed (all generated sources now declare
`<base>.PulseRPC`).

## Case matrix

| case | generated file:line | emitted type/using | expected | compile result | status |
|------|---------------------|--------------------|----------|----------------|--------|
| C1 no `-package`: struct field cross-namespace type | TBD | TBD | `common.Thing` / `Common.Thing` / fully-qualified | TBD | PENDING |
| C2 with `-package MyApp.Lib.Rpc`: struct field cross-namespace type | TBD | TBD | qualified type | TBD | PENDING |
| C3 `using` directives emitted/used | TBD | TBD | correct + used | TBD | PENDING |
| C4 `inc` hardcoded anywhere | TBD | TBD | none | TBD | PENDING |
| C5 casing mismatch (dir/file ns vs type ns: `app` vs `App`) | TBD | TBD | consistent | TBD | PENDING |
| C6 `dotnet build` of generated output | TBD | TBD | success | TBD | PENDING |
| C7 type-mapping helper namespace-awareness | pkg/generator/csharp_client_server.go:TBD | TBD | namespace-aware for field types | TBD | PENDING |

## Findings

TBD

## Residual

TBD

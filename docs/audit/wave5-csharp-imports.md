# Wave 5 — C# cross-namespace import audit

Scope: does generated C# have the Go-style defect where a type from a shared
namespace is emitted unqualified / with a wrong using, so valid strict-validated
IDL produces code that does not compile? Report-only; no source changes.

## Fixture and method

Temp IDL (shared namespace deliberately named `common`, not `inc`):

```pulse
namespace common
struct Thing { id int }

namespace app  // in a separate file importing common.pulse
struct Holder { thing common.Thing }
interface A { get(h Holder) Holder }
```

Generated with `-plugin csharp-client-server`, both with `-package MyApp.Lib.Rpc`
and without, and with `-generate-test-files`. Compiled with `dotnet build`
(`Microsoft.NET.Sdk.Web`, net8.0, `OutputType=Library` / generated
`TestServer.csproj` + `TestClient.csproj`).

## Case matrix

| Case | Generated file:line | Emitted | Expected | Compile | Status |
|---|---|---|---|---|---|
| Struct field type, `-package` | `MyApp/Lib/Rpc/App/Types.cs:8,16` | `public Common.Thing Thing { get; set; }` with `using MyApp.Lib.Rpc.Common;` | qualified type + used using | `dotnet build` succeeded | **RULED OUT** |
| Struct field type, no `-package` | `Common/Types.cs` | `public Common.Thing Thing { get; set; }` | qualified | succeeded | **RULED OUT** |
| Generated test server, shared struct | `TestServer.cs:13,21` | `using MyApp.Lib.Rpc.common;` and `Thing = new common.Thing { ... }` | `MyApp.Lib.Rpc.Common.Thing` (C# is case-sensitive; `common` here holds no `Thing`) | `error CS0246: The type or namespace name 'common' could not be found` | **CONFIRMED BUG** |
| Generated test client, shared struct | `TestClient.cs:26` | `Thing = "test"` | `Thing = new Common.Thing { ... }` | `error CS0029: Cannot implicitly convert type 'string' to 'MyApp.Lib.Rpc.Common.Thing'` | **CONFIRMED BUG** |
| Control: local (non-shared) struct field | `TestClient.cs` (single-namespace fixture) | `Thing = new Thing { ... }` | same | succeeded | Confirms bug is shared-import-specific |

## Root cause (exact code)

- `pkg/generator/csharp_client_server.go:445` `mapTypeToCsTypeWithNamespace` is
  the namespace-aware mapper used by `generateStructClassesCs` (:585), which is
  why production model code is correct.
- Test-data emission is NOT namespace-aware: `writeTestFieldValueCs`
  (~:2504-2535) tries qualified lookup only for enums
  (`enumMap[typeName]`), but for structs uses only
  `structMap[unqualifiedName]`. `structMap` is keyed by qualified names such as
  `common.Thing`, so the lookup misses and the generator falls through to a
  string literal (`Thing = "test"`). The struct-instance writer
  (`writeStructInstanceCs`, used around :2443) similarly derives the type name
  from the raw IDL namespace, producing lowercase `common.Thing`.
- `inc` references at :337, :489, :505, :2427, :2513 are comments/examples, not
  hardcoded emission; no generated file contained a literal `inc` reference in
  this fixture.

## Quality note (not a failure)

`App.cs`/`Common.cs` declare lowercase namespaces (`MyApp.Lib.Rpc.app`,
`...common`) while `Types.cs`/`Client.cs`/`Server.cs` use PascalCase
(`...App`, `...Common`). These are separate C# namespaces; the code still
compiles because `Common.Thing` resolves from the enclosing `MyApp.Lib.Rpc`,
but the split is fragile and should be normalized separately.

## Findings

- The Go-style bug is **absent from C# production/model generation** — both
  `-package` and default modes emit qualified field types and compile.
- It **is present in generated test files** for shared struct fields: two hard
  compile errors, reproducible with a strict-validated two-namespace IDL.
- A local struct field compiles correctly, so the defect is specifically in
  qualified-type handling in test-data generation.

## Residual

- Only struct fields were exercised; shared enums and inherited (`extends`)
  parents from a shared namespace were not probed.
- The no-`-package` variant was compiled for production code only; its test
  files were not separately compiled (likely the same defect).
- Java is audited separately in `wave5-java-imports.md`.

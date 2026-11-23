# Wave 5 — Java cross-namespace import audit

Scope: does generated Java have the Go-style defect where a type from a shared
namespace is referenced in a field/signature without its package qualifier, or
with a wrong/unused import, so valid strict-validated IDL fails to compile?
Report-only; no source changes.

## Fixture and method

Temp IDL (shared namespace deliberately named `common`, not `inc`):

```pulse
namespace common
struct Thing { id int }

namespace app  // separate file importing common.pulse
struct Holder { thing common.Thing }
interface A { get(h Holder) Holder }
```

Generated with `-plugin java-client-server -package com.example.rpc`, with and
without `-generate-test-files`, and compiled with `mvn -q -DskipTests compile`
(generated `pom.xml`). Note: this plugin **requires** `-package` (it errors with
"package flag is required for Java code generation" when absent), so there is no
no-package case.

## Case matrix

| Case | Generated file:line | Emitted | Expected | Compile | Status |
|---|---|---|---|---|---|
| Struct field type | `com/example/rpc/app/Holder.java:6,10` | `import com.example.rpc.common.Thing;` and `private com.example.rpc.common.Thing thing;` (fully qualified both import and field; redundant but valid) | qualified type + import | `mvn compile` exit 0 | **RULED OUT** |
| Getter/setter | `Holder.java:12,16` | `public com.example.rpc.common.Thing getThing()` / `setThing(...)` | qualified | exit 0 | **RULED OUT** |
| Interface signature using Holder | generated `A.java` | `Holder` within same package | unqualified (same package) | exit 0 | **RULED OUT** |
| Generated test files | `TestServer.java` / `TestClient.java` | no literal `inc` references in emitted code (only doc comments mention `inc.Response`) | any shared namespace works | `mvn compile` exit 0 | **RULED OUT** |

## Why it is correct (exact code)

- `pkg/generator/java_client_server.go:643` `findTypeNamespace` resolves a
  type's namespace from `structMap`/`enumMap`.
- `:682` `addTypeImports` registers imports for referenced namespaces.
- `:701` `getJavaTypeWithPackage` and `:728`
  `getJavaTypeWithPackageOptional` map user-defined types to
  `<basePackage>.<namespace>.<BaseName>`, which is the package directory layout
  actually emitted (`:114-119`). This is in contrast to Go's old
  `mapTypeToGoType`, which dropped the namespace.

## Residual / latent risk

- `pkg/generator/java_client_server.go:1792` hardcodes
  `&parser.Type{UserDefined: "inc.Status"}` when generating the `repeat`
  method's test body. Our fixture has no method named `repeat`, so this path was
  not exercised. An IDL with a `repeat`-style method whose status enum lives in
  a namespace not named `inc` may generate a bad type reference. Unverified.
- `getJavaType` (:1576) is not namespace-aware (`getSimpleName` only), but it
  belongs to an alternate struct-class generation path that did not appear in
  the plugin's `Generate` call chain; confirm before changing.
- Shared enums and inherited (`extends`) parents from a shared namespace were
  not probed.

## Findings

- No Go-style import defect was found in Java's generated production or test
  code for a shared struct field; both compile successfully.
- One latent hardcoded `inc.Status` remains in a test-body helper for the
  `repeat` method, which should be probed separately.
- C# is audited separately in `wave5-csharp-imports.md`; it does have a
  confirmed analogous bug limited to generated test files.

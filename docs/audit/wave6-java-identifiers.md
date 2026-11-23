# Wave 6 — Java identifier / qualified-name audit

Scope: determine whether the Java generator (`pkg/generator/java_client_server.go`)
has the Go-style defect where a namespace-qualified IDL name (e.g. an imported
interface `alpha.Alpha`) is interpolated into a generated Java identifier
(class / interface / method / variable / file / package). Also check whether
upstream Java `*Test.java` files leak into generated output and whether
namespace-to-package conversion / base-name collisions are handled.

Prior work: `docs/audit/wave5-java-imports.md` (struct field imports OK;
`inc.Status` hardcode already fixed via `repeatStatusType`). This audit focuses
on identifiers, not type references.

## Findings

| Candidate | File:line evidence | Status | Severity | Fix |
|---|---|---|---|---|
| Qualified interface name → Java identifier (class/impl/client/file/package) | All identifier sites use `GetBaseName`: `java_client_server.go:124,138,152,166,236,324,356,445,501,1017,1710,1732,1901,1928,1929,1980,2004,2005`; `getSimpleName` at `:926,949` (alternate path, not in `Generate` chain) | **RULED OUT** | — | none needed |
| Qualified struct/enum name → Java identifier | same base-name sites above; generated files `alpha/Alpha.java`, `alpha/AlphaClient.java`, `alpha/Status.java`, `alpha/Payload.java`, `alpha/alphaIdl.java` | **RULED OUT** | — | none needed |
| JSON-RPC method string used base name (`Alpha.Ping`) instead of qualified IDL name (`alpha.Alpha.Ping` kept by Go) | `java_client_server.go:543` (`interfaceName` was `GetBaseName(iface.Name)` at `:501`); generated server split at first dot (`:1240`); register key (`:1930`) | **CONFIRMED** (wrong output / runtime collision) | Medium | Fixed: use `iface.Name` for method string + register key; split generated method at last dot |
| Upstream `*Test.java` copied into generated output | `runtime/embed.go:38` embeds only `runtimes/java/com/bitmechanic/pulserpc` (production dir); upstream tests live in `runtimes/java/tests/` and are not embedded. `copyRuntimeFiles` :273-309 writes only embedded files | **RULED OUT** | — | none needed |
| Namespace-to-package conversion / base-name collisions (two namespaces, same base interface name) | Generated `Server.java:14` imported both `com.example.rpc.alpha.Thing` and `com.example.rpc.gamma.Thing`; `java_client_server.go:1013-1021`, `:1893-1904`, `:1955-1975` emitted unused single-type imports; duplicate local `clientVar` at `:2005` | **CONFIRMED** (compiler error) | High | Fixed: drop unused single-type imports, dedupe client local variables, qualify handler keys |

The parser prefixes only *imported* declarations (`pkg/parser/parser.go:735-747`):
`iface.Name` becomes `alpha.Alpha`. Root-namespace declarations keep a simple
name (`Thing`), so the correct wire name is the raw `iface.Name` in both cases.

## Fixture and method

Two fixtures, generated with the plugin directly (the CLI needs a web UI
`dist/` build not present in this worktree) via a temporary harness calling
`parser.ParseIDL` + `NewJavaClientServer().Generate` with `-dir`, `-package
com.example.rpc`, `-generate-test-files=true`, into a pre-created output dir.
Maven runs in the generated project outside the checkout.

### Fixture A — imported interface identifiers

`alpha.pulse`: `namespace alpha` + `struct Payload { id int }`, `enum Status { ok failed }`, `interface Alpha { Ping() string }`.
`gamma.pulse`: `namespace gamma`, `import "alpha.pulse"`, `interface Gamma { Echo() string }`.

Before the fix: `mvn -q -DskipTests compile` exit 0 and `mvn -q test` exit 0, so
no Java syntax error from qualified names (all identifiers use base names:
`Alpha.java`, `AlphaClient.java`, `AlphaImpl.java`, `class AlphaClient implements
Alpha`, `clientVar alphaClient`). No upstream `*Test.java` in output; only
generated `TestServer.java` / `TestClient.java`. But
`AlphaClient.java:19` emitted `String method = "Alpha.Ping";` and
`TestServer.java` registered `"Alpha"` / `"Gamma"`, diverging from the embedded
IDL and the Go generator's `alpha.Alpha.Ping`. Java-only dispatch happened to
work because `Contract.findInterfaceName` falls back to base-name matching.

After the fix: `String method = "alpha.Alpha.Ping";` and
`server.register("alpha.Alpha", new ...AlphaImpl())`; both mvn commands still
exit 0.

### Fixture B — two namespaces, same base interface name

`alpha.pulse`: `namespace alpha` + `interface Thing { Ping() string }`.
`gamma.pulse`: `namespace gamma`, `import "alpha.pulse"`, `interface Thing { Echo() string }`.

Before the fix, generation succeeded but `mvn -DskipTests compile` failed:

```
[ERROR] /C:/Users/james/AppData/Local/Temp/opencode/wave6-java-collide-out/src/main/java/com/example/rpc/Server.java:[14,1] a
type with the same simple name is already defined by the single-type-import of com.example.rpc.alpha.Thing
[ERROR] BUILD FAILURE   (exit 1)
```

Also `TestServer.java` registered both handlers under `"Thing"`, and the two
clients emitted `"Thing.Ping"` / `"Thing.Echo"`, so one interface was
unreachable at runtime. After the fix: no duplicate interface imports,
distinct local variables (`thingClient` / `alphathingClient`), keys
`"Thing"` (root) and `"alpha.Thing"` (imported), method strings
`"Thing.Echo"` / `"alpha.Thing.Ping"`, and both `mvn -DskipTests compile` and
`mvn -q test` exit 0.

## Regression tests

`pkg/generator/java_import_identifiers_regression_test.go` builds the generated
Maven project with the exact two fixtures above (`mvn -q -DskipTests compile`,
plus `mvn -q test` for the collision case) and asserts the qualified method
strings, register keys, base-name identifiers, absence of upstream test files,
and absence of duplicate simple-name imports.

## Verification

- `go test ./pkg/generator -count=1` — pass (exit 0) after the fix.
- Fixture A `mvn -q -DskipTests compile` / `mvn -q test` — pass (exit 0).
- Fixture B `mvn -q -DskipTests compile` / `mvn -q test` — pass (exit 0).

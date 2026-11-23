# Wave 6: C# Identifier Audit (qualified IDL names + upstream test file leakage)

Branch: `audit-csharp-identifiers` (from main incl. 830ef90).

Audit question: is the C# generator vulnerable to the same class of bug fixed in Go at 830ef90 — a namespace-qualified IDL name such as `alpha.Alpha` being interpolated into a generated C# identifier, plus upstream test `.cs` files leaking into generated output?

## Reproduction (primary bug, before fix)

Temp dir `<tmp>\csaudit` (outside the checkout):

`alpha.pulse`:
```
namespace alpha

struct AlphaData { id string }

interface Alpha { Ping() string }
```

`gamma.pulse`:
```
namespace gamma

import "alpha.pulse"

interface Gamma { Status() string }
```

Command:
```
pulse -plugin csharp-client-server -package MyApp.Lib.Rpc -generate-test-files -dir <tmp>\csaudit\out3 <tmp>\csaudit\gamma.pulse
```

The parser gives the imported interface the qualified IDL name `alpha.Alpha` (confirmed in generated JSON: `"name": "alpha.Alpha"` at `Client.cs:41`, `Server.cs:45`).

Generated invalid identifiers (exact file:line in `<out3>`):
- `Contract.cs:31` — `public interface Ialpha.Alpha`
- `Server.cs:98` — `public void Registeralpha.Alpha(Ialpha.Alpha implementation)`
- `Client.cs:278,287,292,298,309` — `public class alpha.AlphaClient : Ialpha.Alpha`, constructors/fluent methods `alpha.AlphaClient(...)`
- `TestServer.cs:24` — `public class alpha.AlphaImpl : Ialpha.Alpha`
- `TestServer.cs:46` — `server.Registeralpha.Alpha(new alpha.AlphaImpl());`
- `TestClient.cs:32` — `var alpha.alphaClient = new alpha.AlphaClient(transport);`

Correct JSON-RPC method string (kept qualified): `Client.cs:403` — `var method = "alpha.Alpha.Ping";` (matches the IDL/Go canonical form; the Go regression test asserts exactly this string).

`dotnet build TestServer.csproj` (before fix, exact errors): `Contract.cs(31,24): error CS1514: { expected`, `TestServer.cs(24,19): error CS1514: { expected`, `TestServer.cs(24,20): error CS8803: Top-level statements must precede namespace and type declarations.`, `TestServer.cs(26,5): error CS0106: The modifier 'public' is not valid for this item` — 12 errors, FAILED.

`dotnet build TestClient.csproj` (before fix, exact errors): `Client.cs(278,19): error CS1514: { expected`, `TestClient.cs(32,18): error CS1003: Syntax error, ',' expected`, `Contract.cs(31,24): error CS1514: { expected` — FAILED.

## Findings

| # | Candidate | File:line evidence | Status | Severity | Fix |
|---|---|---|---|---|---|
| 1 | Qualified interface name interpolated into C# **identifier** (`I<Name>`, `<Name>Client`, `<Name>Impl`, `Register<Name>`, client variable) | `csharp_client_server.go:763,813,1690-1691,1999-2000,2036,2123-2124,2425`; generated `Contract.cs:31`, `Client.cs:278`, `TestServer.cs:24,46`, `TestClient.cs:32` | **CONFIRMED** (repro above) | High — any imported interface makes generated projects uncompilable | **FIXED**: added `csharpInterfaceIdent(iface) = GetBaseName(iface.Name)` and used it at every identifier site; `iface.Name` retained only for strings (JSON-RPC, dispatch, display) |
| 2 | Qualified interface name in JSON-RPC method string | `csharp_client_server.go:1871` → generated `Client.cs:403 var method = "alpha.Alpha.Ping";`; interface key at `Server.cs:100 this.Register("alpha.Alpha", implementation)` | **RULED OUT** (already correct) | — | None |
| 3 | Qualified struct/enum name in C# identifier | `csharp_client_server.go:566` (`GetBaseName(e.Name)`), `:595` (`GetBaseName(s.Name)`), helpers `:499-544` (`resolveTypeKey`, `qualifiedCsTypeName`, `getStructOrEnumTypeName`) | **RULED OUT** (already base-named) | — | None |
| 4 | Upstream test `.cs` files copied into generated output | `pkg/runtime/embed.go:33` embeds only `runtimes/csharp/PulseRPC` (sibling `runtimes/csharp/tests/` is not embedded); `embed.go:138-170` skips directories; generated `out3`/`outfixed` contain only production `.cs` under `MyApp/Lib/Rpc/PulseRPC/` (no `*Tests.cs`). Verified by `dotnet build` in `<tmp>` outside the checkout. | **RULED OUT** | — | None. (`CopyRuntimeFilesToPackage` at `embed.go:187-218` copies whatever `GetRuntimeFiles` returns; the embed boundary, not a C#-specific filter, keeps tests out — unlike Go, which needed an explicit `_test.go` filter.) |
| 5a | Same base interface name in two namespaces (`alpha.Shared`, `gamma.Shared`) | Fixture `col-alpha.pulse`/`col-gamma.pulse`; generated `Contract.cs:26,31` both `public interface IShared`; `dotnet build`: `Contract.cs(31,18): error CS0101: The namespace 'MyApp.Lib.Rpc.PulseRPC' already contains a definition for 'IShared'`, `Server.cs(85,17): error CS0111: Type 'PulseRPCServer' already defines a member called 'RegisterShared'`, `TestServer.cs(24,14): error CS0101: ... 'SharedImpl'` | **CONFIRMED** (pre-existing, not introduced by fix — before fix both declared `Ialpha.Shared`/`Igamma.Shared`, also uncompilable) | Medium — multi-namespace IDLs with a repeated interface name | Recommend, do not fix here: emit interface stubs/clients/impls into per-namespace files under `namespace <PascalNs>` (contract per namespace) instead of the single global `Contract.cs`/`Client.cs`/`Server.cs`, and disambiguate the `Register<Name>` typed overloads. Large change; would break hand-written quickstart projects that include only `<Ns>.cs`, `Types.cs`, `Contract.cs`, `Server.cs`, `Client.cs`. |
| 5b | Same base struct name in two namespaces (`alpha.Item`, `gamma.Item`) | Fixture `sc-alpha.pulse`/`sc-gamma.pulse`; `dotnet build`: `Contract.cs(28,5)/(33,5): error CS0104: 'Item' is an ambiguous reference between 'MyApp.Lib.Rpc.Alpha.Item' and 'MyApp.Lib.Rpc.Gamma.Item'`, `Client.cs(211,12)/(407,12): error CS0104`, `TestServer.cs(17,12): error CS0104` | **CONFIRMED** (pre-existing; struct names were already base-named) | Medium — multi-namespace IDLs with a repeated struct name | Recommend: in top-level files (`Contract.cs`, `Client.cs`, `Server.cs`, generated tests) fully qualify user-defined types via `qualifiedCsTypeName` using each type's registry namespace instead of emitting bare base names when multiple namespaces are loaded. Must preserve quickstart includes (only `<Ns>.cs` + `Types.cs`), so scope to generated top-level files first. |

## Fix implemented (finding 1)

`pkg/generator/csharp_client_server.go`:
- Added `csharpInterfaceIdent(iface) = GetBaseName(iface.Name)` (mirrors Go's `goInterfaceIdent`), documented that the qualifier stays in JSON-RPC strings.
- Applied at: interface stub (`I%s`), `Register%s` typed overload + `I%s` parameter, client class name/inheritance/constructors/fluent methods, test impl class + inheritance, test server registration/impl, test client variable declaration and calls.
- Left `iface.Name` for: `this.Register("alpha.Alpha", ...)`, interface dispatch comparisons (`interfaceName == "alpha.Alpha"`), `var method = "alpha.Alpha.Ping"`, the `interfaceName` switch, and test output messages.

After fix, regenerating `<outfixed>` produces `IAlpha`, `AlphaClient`, `AlphaImpl`, `RegisterAlpha`, `alphaClient`, while `"alpha.Alpha"` and `"alpha.Alpha.Ping"` remain intact. Both `dotnet build TestServer.csproj` and `dotnet build TestClient.csproj` succeed (warnings only, 0 errors).

## Regression tests

Added `pkg/generator/csharp_import_interface_identifiers_test.go`:
- `TestCSharpImportedInterfaceIdentifiersAndMethodName` — two-namespace fixture; asserts base-name identifiers, qualified JSON-RPC strings, absence of invalid qualified identifiers, and builds both generated projects with `dotnet build`.
- `TestCSharpGeneratedRuntimeOmitsUpstreamTests` — asserts no upstream `*Tests.cs`/`TestHelpers.cs` in generated runtime, production files present, and both generated projects build outside the checkout.

## Verification

- `go test ./pkg/generator -count=1` — PASS (18.0s).
- Regression tests alone — PASS (5.8s).
- `dotnet build` of generated `TestServer.csproj` / `TestClient.csproj` (two-namespace, with `-package`) — PASS after fix (FAILED before fix with CS1514/CS1003 etc.).
- Quickstart: generated `checkout.pulse` with the fixed binary into a temp `Shared/`, built the hand-written `examples/quickstart/csharp/TestServer` and `TestClient` projects — both PASS (0 errors). The fix does not change identifiers for unqualified single-namespace interfaces, so the quickstart is unaffected.
- `pkg/webui/dist/index.html` was created as a gitignored build stub to compile the CLI; it is not part of the change set.

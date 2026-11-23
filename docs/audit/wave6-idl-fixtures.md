# Wave 6 Audit: Committed IDL Fixtures

Status: **COMPLETE** — fixture and tests added; all verification commands pass.

## Scope

Assess whether the repository needs a more complex, committed multi-namespace
IDL fixture to catch cross-namespace generation bugs of the class fixed in
commit `830ef90` / `aa56289` (Go), and found in the Java / C# wave-6 audits.
The original repro was two temp files: `alpha.pulse` (namespace `alpha`,
`interface Alpha { Ping() string }`) and `root.pulse` (namespace `gamma`,
`import "alpha.pulse"`, `interface Gamma { Status() string }`). Go/C# emitted
invalid identifiers from the qualified name `alpha.Alpha`; Java emitted a
wrong RPC method string.

## Findings

### 1. Coverage of every committed `.pulse` fixture

| Fixture | Namespaces | Imports | Interfaces declared | Interface in *imported* namespace? | Inheritance | Optional | Arrays | Maps | Enums |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `examples/conform.pulse` | `conform` (`:13`) | `conform-inc.pulse` (`:15`) | `A` (`:53`), `B` (`:100`) — both in root | **No** | `RepeatResponse extends inc.Response` (`:26`) | `:47-50`, `:95` | `:28`, `:80` | No | imported `inc.MathOp` at `:59`, `:92` |
| `examples/conform-inc.pulse` | `inc` (`:1`) | none | none | **No** (it *is* the imported file) | none | no | no | no | `Status` (`:3`), `MathOp` (`:8`); `Response` (`:13`) |
| `examples/errors-service.pulse` | `example` (`:1`) | `errors.pulse` (`:3`) | `pets` (`:24`) — root only | **No** — imports only errors (`api.InvalidInput` etc., `:29`, `:32`) | no | no | `:26` | no | no |
| `examples/errors.pulse` | `api` (`:1`) | none | none | **No** | no | no | no | no | no (errors only, `:4-12`) |
| `examples/book.pulse` | `book` (`:1`) | none | `UserService` (`:8`), `BookService` (`:20`), `CronJobs` (`:37`) | **No** (single namespace) | `:71`, `:75` | no | `:23`, `:26`, `:100`, `:166` | no | `:47`, `:52`, `:132` |
| `examples/quickstart/checkout.pulse` | `checkout` (`:1`) | none | `CatalogService` (`:90`), `CartService` (`:98`), `OrderService` (`:109`) | **No** | no | `:28`, `:64`, `:77`, `:95`, `:103`, `:114` | `:39`, `:92` | no | `:5`, `:13` |
| `examples/openapi/simple-api.pulse` | `simple_api` (`:4`) | none | `UserService` (`:28`) | **No** | no | `:9` | `:39` | no | `:21` |
| `examples/openapi/petstore.pulse` | `petstore` (`:7`) | none | `pets` (`:29`) | **No** | no | `:13`, `:19` | `:31` | no | no |
| `pulserpc-vscode/sample.pulse` | `conform` (`:7`) | `conform-inc.pulse` (`:9`) | `A` (`:34`), `B` (`:69`) — root only | **No** | `:12` | `:31` | `:14`, `:58` | no | imported `inc.MathOp` at `:40` |

Only three committed fixtures import another file: `conform.pulse:15`,
`errors-service.pulse:3`, `pulserpc-vscode/sample.pulse:9`. In every case the
imported file contains only structs/enums/errors (`conform-inc.pulse:1-15`,
`errors.pulse:1-13`) — never an interface. Maps are unsupported by all
committed fixtures (`tests/integration/README.md:32`); the parser supports them
(`pkg/parser/checksum.go:212`, `pkg/parser/parser_test.go:180`) but that is
outside this audit's requested fixture scope.

### 2. No committed fixture exercises an interface in an imported namespace

Confirmed by reading all nine files above and by grepping the generator tests.
The coverage exists only as ad-hoc temp fixtures written into `t.TempDir()`:

- `pkg/generator/go_import_interface_regression_test.go:20-29` —
  `TestGoImportedInterfaceIdentifiersAndMethodName` writes `alpha.pulse` and
  `root.pulse`; assertions at `:64-88`, `go build` at `:90-100`.
- `pkg/generator/go_shared_import_test.go:18-27` —
  `TestGoSharedNamespaceStructFieldIsQualified`; imported *struct* only,
  interface `A` is in the root file.
- `pkg/generator/csharp_shared_test_imports_test.go:16-25` —
  `TestCSharpSharedStructInGeneratedTestFiles`; imported *struct* only.
- `pkg/generator/java_repeat_status_regression_test.go:14-56` —
  `TestJavaRepeatStatusTypeFromIDL`; imported *enum* only.
- `pkg/generator/java_audit_test.go:12-15`, `csharp_audit_test.go:11-14`,
  `go_generator_audit_test.go:35-38` use inline single-namespace strings, not
  imported interfaces.

The only cross-runtime consumer of a committed fixture is
`tests/integration/cross_generator_test.go:64-76` (`loadCrossCheckerIDL`), which
reads `examples/conform.pulse` and fans out to all 7 variants defined at
`:38-62`. Because `conform.pulse` has no imported interface, that fan-out never
exercises the bug class.

### 3. What the new fixture proved (generated with the committed files)

Generation of `examples/cross-ns/app.pulse` succeeded for all 7 variants
(`go`, `java`, `python`, `python2`, `ts-node`, `ts-cjs`, `csharp`) and each
generator's IDL metadata carries the qualified name `common.CommonOps`.
The remaining behavior is *not* uniformly correct, which is precisely why the
committed fixture is warranted:

- **Go** — correct: `type CommonOpsClient struct`, `func (c *CommonOpsClient)
  Ping(`, `methodName := "common.CommonOps.Ping"`, `type CommonOps interface`;
  generated module builds. This guards commit `aa56289`.
- **Java** — generated code compiles (`mvn -q -DskipTests compile`, exit 0),
  but `CommonOpsClient.java` emits `String method = "CommonOps.Ping"` —
  unqualified, while IDL metadata says `common.CommonOps`. This is the
  wrong-RPC-method-string class from the wave-6 Java audit, still present.
- **C#** — `Client.cs` has `public class common.CommonOpsClient`,
  `Contract.cs` has `public interface Icommon.CommonOps`, `Server.cs` has
  `public void Registercommon.CommonOps(...)`. `dotnet build` fails with
  `Client.cs(367,34): error CS1001: Identifier expected`,
  `Contract.cs(29,27): error CS1001`, and follow-on syntax errors.
- **Python** — `server.py` contains `class common.CommonOps(abc.ABC):` (invalid
  Python identifier) and `client.py` a `client.common.CommonOps.Ping()` comment.
- **TypeScript** (`ts-node` and `ts-cjs`) — `client.ts` contains
  `export class common.CommonOpsClient {` and `server.ts` contains
  `export abstract class common.CommonOps {`; method strings are correctly
  qualified (`method: "common.CommonOps.Ping"`).
- **python2** — qualified name appears in `idl.json` only; no further claim is
  made here.

Go-specific ad-hoc temp fixtures cannot reveal any of this because no other
runtime runs them, and the committed `conform.pulse` has no imported interface.

## Recommendation

**A committed fixture is warranted.** Add one minimal two-file example and wire
it into both the Go generator unit suite and the existing cross-generator CI
fan-out. Do not build a new conformance suite.

### Files added

- `examples/cross-ns/common.pulse` — namespace `common` (`:4`), `enum Mode`
  (`:6-9`), `struct Shared` (`:11-16`) with array `tags []string` (`:14`) and
  optional `note` (`:15`), and **`interface CommonOps` (`:21-24`) with methods
  `Ping` (`:22`) and `Echo` (`:23`) declared in the imported namespace**.
- `examples/cross-ns/app.pulse` — namespace `app` (`:2`), `import
  "common.pulse"` (`:4`), `struct Derived extends common.Shared` (`:7-9`), and
  `interface AppOps` (`:11-14`) whose methods take/return imported types.
- `pkg/generator/cross_namespace_fixture_test.go` —
  `TestCrossNamespaceFixtureGo` (`:51`) parses the committed file, generates Go,
  asserts base identifiers + qualified method strings, rejects qualified
  identifiers, and runs `go build ./...` when `go` is on PATH;
  `TestCrossNamespaceFixtureJavaAndCSharp` (`:111`) generates Java and C#,
  asserts the qualified interface and both method names in embedded IDL
  metadata, compiles Java when `mvn` exists, and runs a C# `dotnet build` check
  that is skipped only while the known qualified-identifier bug is present
  (it self-enables once the generator is fixed). Helpers at `:16`, `:30`, `:39`.
- `tests/integration/cross_generator_cross_namespace_test.go` —
  `TestCrossGeneratorCrossNamespaceFixture` (`:17`) generates from the same
  committed fixture for all 7 variants from `crossGeneratorVariants()` and
  asserts every generator emits at least one file containing `common.CommonOps`.

### Wiring plan (existing tests / CI jobs, no new workflow needed)

1. `make test` (`Makefile:43`) and `go test -race ./pkg/generator` (`.github/workflows/ci.yml:80`)
   pick up both new `pkg/generator` tests automatically.
2. `make test-cross-generator` (`Makefile:155`, run by the `variants` job at
   `.github/workflows/ci.yml:107-108`) uses `-run 'TestCrossGenerator'`, so the
   new `TestCrossGeneratorCrossNamespaceFixture` runs for all 7 variants.
3. The `go-unit` job's `go test ./tests/integration/...` (`:83`) and
   `go vet` (`:74`) also cover the new integration test.
4. Build/assert steps per runtime once the known bugs are fixed: Go `go build
   ./...` (already green); Java `mvn -q -DskipTests compile` (already green,
   then add the qualified `String method = "common.CommonOps.Ping"` assertion);
   C# `dotnet build` on each generated `.csproj` plus base-name identifier
   assertions; Python `python -m py_compile` on generated `server.py`/`client.py`;
   TypeScript `tsc --noEmit` on generated `client.ts`/`server.ts`; python2 at
   minimum the `idl.json` qualified-name assertion. Only Go/Java/C# are
   asserted today; Python/TS coverage is the next step enabled by this fixture.

## Verification

Run in the worktree with `GOTMPDIR=C:\Users\james\AppData\Local\Temp\opencode`:

- `go test ./pkg/generator -count=1` — **PASS** (17.3s).
- `PULSERPC_CROSS_GENERATOR=1 go test ./tests/integration -run 'TestCrossGenerator' -count=1` — **PASS** (1.6s), including the new all-7-variant test.
- `gofmt -l` on both new Go test files — no output (formatted).
- `go test ./pkg/generator -run TestCrossNamespaceFixture -v -count=1` —
  Go and Java subtests PASS; C# metadata PASS; C# build subtest **SKIP** with
  the explicit known-bug reason (dotnet build of the current output fails with
  CS1001, confirmed out of band).
- `mvn -q -DskipTests compile` on generated Java — exit 0.
- `dotnet build TestClient.csproj` / `TestServer.csproj` on generated C# —
  fails with CS1001, reproducing the imported-interface identifier bug.

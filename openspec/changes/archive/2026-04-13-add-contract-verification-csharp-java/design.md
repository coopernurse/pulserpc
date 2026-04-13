## Context

Contract compatibility verification was implemented for Go in `pkg/runtime/runtimes/go/pulserpc/`. The Go implementation provides:

- `ContractAuditor` interface with `Audit(ctx, result)` and `Name()` methods
- Built-in auditors: `NoOpAuditor`, `LoggingAuditor`, `FailFastAuditor`
- `DiffIDL()` function that compares embedded client IDL against server IDL
- Severity classification based on runtime impact
- `VerificationResult` with `Compatible`, checksums, deltas, and timestamp
- `VerifyCompatibility(ctx)` method on the `Client`

This design specifies implementing the same capability for C# and Java runtimes.

## Goals / Non-Goals

**Goals:**
- Mirror the Go implementation's capability in C# and Java
- Use idiomatic patterns for each language (async/await in C#, synchronous in Java)
- Allow custom auditor implementations
- Embed IDL_JSON in generated clients for runtime comparison
- Document the feature in each runtime's reference documentation

**Non-Goals:**
- Modifying the Go implementation (already complete)
- Adding cancellation support to Java (not needed)
- Implementing verification for Python or TypeScript runtimes
- Changing the severity classification matrix (already defined in spec)

## Decisions

### Decision: C# VerificationCompatibility is Async

**Chosen:** `Task<VerificationResult> VerifyCompatibility(CancellationToken cancellationToken = default)`

**Rationale:** C# has strong async-first idioms. Since `VerifyCompatibility` involves computation (parsing IDL, computing diff) and potentially I/O if needed, async is appropriate and expected by C# developers.

**Alternatives Considered:**
- Sync version only: Would be unfamiliar to C# developers who expect async I/O operations
- `IProgress<T>` callback: Adds complexity; auditor interface is sufficient for notification

### Decision: Java Verification is Synchronous

**Chosen:** `VerificationResult verifyCompatibility()` (no async variant)

**Rationale:** Java's client runtime is synchronous. Adding async would require significant refactoring of the existing client architecture. The computation is fast enough that blocking is acceptable.

**Alternatives Considered:**
- `CompletableFuture<VerificationResult>`: Adds complexity without clear benefit
- `Callable<VerificationResult>`: More ceremony than value for this use case

### Decision: C# Uses CancellationToken Instead of Context

**Chosen:** `VerifyCompatibility(CancellationToken cancellationToken = default)`

**Rationale:** .NET uses `CancellationToken` for cancellation patterns, not Go's `context.Context`. Using `CancellationToken` is idiomatic C#.

**Alternatives Considered:**
- `IAsyncDisposable` with context: Overcomplicated for this feature
- No cancellation support: Fine since this operation is typically fast

### Decision: Java Uses Static Factory Methods for Auditors

**Chosen:** `ContractAuditor.noOp()`, `ContractAuditor.logging()`, `ContractAuditor.failFast()`

**Rationale:** Java 8 friendly interface with default static methods. Mirrors how other Java libraries (e.g., Jackson) provide built-in implementations.

**Alternatives Considered:**
- Separate classes: More verbose, requires users to know about separate implementations
- Enum with values: Doesn't allow user-defined implementations cleanly

### Decision: Generated Clients Embed IDL_JSON

**Chosen:** Each generated client class contains `private static final String IDL_JSON = "...";`

**Rationale:** Enables offline verification without needing the original IDL file. Server IDL is fetched via `pulserpc-idl` RPC at bootstrap.

**Alternatives Considered:**
- Load from file at runtime: Requires IDL file to be present, complicates deployment
- Pass IDL to constructor: Users might forget, more error-prone

### Decision: DiffEngine is Language-Specific Port

**Chosen:** Implement `DiffIDL()` and helper functions in each language, ported from Go

**Rationale:** The diff algorithm works on generic IDL structure (maps, lists). Each language's implementation mirrors the Go logic but uses language-appropriate idioms.

**Alternatives Considered:**
- Share diff logic via common library: Would require significant refactoring
- Call Go from other languages: Not practical (different runtimes)

## Risks / Trade-offs

[Risk] **Porting errors in diff logic** → Mitigation: Port test cases from Go `auditor_test.go` to C# and Java

[Risk] **Different JSON parsing behavior across languages** → Mitigation: Use the same JSON library each runtime already uses (Jackson/Gson for Java, System.Text.Json for C#)

[Risk] **Checksum computation differences** → Mitigation: Use SHA-256 in all languages; verify with integration tests

## Open Questions

None at this time. Key decisions have been made based on Go implementation precedent and language idioms.

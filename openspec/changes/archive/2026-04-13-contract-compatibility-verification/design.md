## Context

PulseRPC provides a checksum-based mechanism to detect that client and server IDLs differ, but offers no insight into *what* changed or *how severe* the difference is. This brittleness problem - described by Nelson Minar regarding SOAP/WSDL - manifests when clients and servers evolve independently. Currently, any checksum mismatch triggers the same blunt "incompatible" response regardless of whether the difference is adding an optional field (backward compatible) or removing a required field (breaking).

The Go runtime client already fetches the server's IDL at bootstrap via the `pulserpc-idl` method. We need to extend this infrastructure to compare the server's IDL against the client's embedded IDL and classify the differences.

## Goals / Non-Goals

**Goals:**
- Provide directional structural diff between client and server IDLs
- Classify deltas by severity (Error, Warning, Info) based on runtime impact
- Expose verification results for programmatic inspection
- Support pluggable auditors for custom handling (logging, fail-fast, etc.)
- Opt-in verification at bootstrap or explicit verification after

**Non-Goals:**
- Semantic change detection (we only detect structural differences)
- Version number management or compatibility matrices
- Cross-namespace import version tracking (compare namespace-qualified names directly)
- Automatic migration or backward compatibility handling (just advisory)
- Multi-language runtimes (Go runtime first, others follow pattern)

## Decisions

### 1. Verification is explicit, not automatic

**Decision:** `VerifyCompatibility()` returns a result but does not fail on incompatibility. Users must inspect the result or configure an auditor.

**Rationale:** Automatic fail-on-incompatibility complicates deployment scenarios where brief checksum mismatches are expected (e.g., rolling deployments). Users should choose when to enforce strict compatibility.

**Alternatives considered:**
- Fail automatically on any delta: Too brittle for real-world deployment
- Configurable severity threshold (fail on Error but not Warning): Adds complexity; users can implement via custom auditor

### 2. Severity classification based on runtime impact

**Decision:** Deltas are classified by severity:

| Delta | Direction | Severity |
|-------|-----------|----------|
| Struct removed | ClientHasMore | Error |
| Struct added | ClientHasLess | Info |
| Field type changed | Mismatch | Error |
| Field removed | ClientHasMore | Info |
| Required field added | ClientHasLess | Error |
| Optional field added | ClientHasLess | Info |
| Required field made optional | ClientHasLess | Info |
| Optional field made required | ClientHasLess | Warning |
| Method removed | ClientHasMore | Error |
| Method added | ClientHasLess | Warning |
| Method signature changed | Mismatch | Error |
| Enum value removed | ClientHasMore | Warning |
| Enum value added | ClientHasLess | Warning |

**Rationale:** Direction tells us who has the extra thing:
- `ClientHasMore` = client sends server-ignorable extras → typically Info (server ignores)
- `ClientHasLess` = client missing server-required things → Error/Warning depending on required
- `TypeMismatch` = JSON-level incompatibility → always Error

**Alternatives considered:**
- Severity configurable per entity type: Over-engineering for first pass
- Runtime impact instead of severity: Equivalent but "severity" is more actionable

### 3. Auditor interface for pluggable handling

**Decision:**

```go
type ContractAuditor interface {
    Audit(ctx context.Context, result *VerificationResult)
    Name() string
}
```

**Rationale:** Separates verification logic from handling logic. Users can inspect results directly (NoOp), log at appropriate levels (LoggingAuditor), fail fast (FailFastAuditor), or implement custom (send to monitoring service, page oncall, etc.).

**Alternatives considered:**
- Return result with embedded action: Less flexible
- Callback function instead of interface: Interface is more testable and extensible

### 4. Built-in auditors only: NoOp, Logging, FailFast

**Decision:** Provide three built-in implementations.

**Rationale:** Cover the common cases: inspection (NoOp), production logging (Logging), development strictness (FailFast). Users can implement custom for specialized needs.

### 5. Client embeds IDL_JSON alongside server

**Decision:** Both client and server generated code embed `IDL_JSON` constant containing their respective IDL definitions.

**Rationale:** Server already embeds for `pulserpc-idl` RPC method. Extend to client for comparison. The embedded JSON is the source of truth for "what this side knows."

### 6. Verification operates on full IDL, no scoping

**Decision:** `VerifyCompatibility()` compares all namespaces, interfaces, structs, enums, and errors.

**Rationale:** Simpler first implementation. Users with large multi-namespace systems can filter results client-side if needed.

## Risks / Trade-offs

- **[Risk]** Cross-namespace import version mismatch: If client imports `common.pulse` v1 and server has `common.pulse` v2, we compare what we have but don't detect the import version difference. → **Mitigation:** Document this limitation; users should regenerate both sides together.

- **[Risk]** Verification adds latency to bootstrap: Fetching and parsing server IDL then computing diff takes time. → **Mitigation:** Opt-in via `VerifyOnBootstrap()`; explicit `VerifyCompatibility()` call is async anyway.

- **[Trade-off]** Granularity vs complexity: We don't track field-level optionality changes with fine granularity (was optional, now required vs was required, now optional). → **Mitigation:** Current classification handles both cases; severity differs appropriately.

## Open Questions

1. **Should `VerifyOnBootstrap()` call the auditor or just return the result?**
   - Recommendation: Yes, call auditor automatically when this option is set.

2. **What should happen if server doesn't support `pulserpc-idl`?**
   - Recommendation: Return error indicating server is too old for verification.

3. **Should we expose `ServerChecksum` and `ClientChecksum` separately?**
   - Recommendation: Yes, in `VerificationResult` for debugging/alerting purposes.

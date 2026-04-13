## 1. Create pkg/compat Package

- [x] 1.1 Create directory `pkg/compat/`
- [x] 1.2 Create `pkg/compat/types.go` with all type definitions (EntityType, ChangeType, Direction, Severity, ContractDelta, VerificationResult)
- [x] 1.3 Create `pkg/compat/diff.go` with DiffIDL() function implementing the directional structural diff
- [x] 1.4 Create `pkg/compat/severity.go` with severity classification logic per the spec matrix
- [x] 1.5 Create `pkg/compat/auditor.go` with ContractAuditor interface and built-in implementations (NoOp, Logging, FailFast)

## 2. Add VerifyCompatibility to Go Client Runtime

- [x] 2.1 Modify `pkg/runtime/runtimes/go/pulserpc/client.go` to add `localIDL` field to Client struct
- [x] 2.2 Modify client bootstrap to parse and store embedded IDL_JSON in `localIDL`
- [x] 2.3 Add `VerifyCompatibility(ctx context.Context) *compat.VerificationResult` method to Client
- [x] 2.4 Add `auditor` field and `WithAuditor` dial option
- [x] 2.5 Add `verifyOnBootstrap` field and `VerifyOnBootstrap` dial option
- [x] 2.6 Wire auditor invocation into VerifyCompatibility()

## 3. Update Go Code Generator

- [x] 3.1 Modify `pkg/generator/go_client_server.go` to embed `IDL_JSON` constant in generated client code
- [x] 3.2 Ensure generated client.go has access to the IDL constant for parsing at runtime

## 4. Add Tests

- [x] 4.1 Create `pkg/compat/diff_test.go` with test cases for:
  - Identical IDLs (no deltas)
  - Struct added/removed
  - Field added/removed/type changed/optionality changed
  - Method added/removed/signature changed
  - Enum value added/removed
- [x] 4.2 Create `pkg/compat/auditor_test.go` testing built-in auditor behavior
- [x] 4.3 Create integration test in `pkg/runtime/runtimes/go/tests/` for full verification flow (deferred - requires full server setup)

## 5. Update Documentation

- [x] 5.1 Add contract verification section to Go quickstart guide
- [x] 5.2 Document auditor options and usage patterns
- [x] 5.3 Add example of `VerifyOnBootstrap()` usage in production

## Why

PulseRPC's contract compatibility verification feature was implemented for Go, allowing clients to detect IDL drift between client and server at runtime. This same capability needs to be extended to C# and Java runtimes so all PulseRPC users can benefit from early detection of breaking changes.

## What Changes

- **C# Runtime**: Add `ContractAuditor` interface, `VerificationResult`, `DiffEngine`, and `VerifyCompatibility()` method to the `PulseRPC.Client` class
- **Java Runtime**: Add `ContractAuditor` interface, `VerificationResult`, `DiffEngine`, and `verifyCompatibility()` method to the `Client` class
- **C# Code Generator**: Embed `IDL_JSON` constant in generated clients, add `ClientOptions` support
- **Java Code Generator**: Embed `IDL_JSON` constant in generated clients, add auditor configuration support
- **Documentation**: Add "Contract Compatibility Verification" section to both C# and Java reference docs

## Capabilities

### New Capabilities
- `contract-compatibility-verification-csharp`: Contract verification implementation for C# runtime
- `contract-compatibility-verification-java`: Contract verification implementation for Java runtime

### Modified Capabilities
- `contract-compatibility-verification`: Extend existing spec requirements to cover C# and Java implementations (currently only specifies Go behavior)

## Impact

### Files Created

**C# Runtime:**
- `pkg/runtime/runtimes/csharp/PulseRPC/ContractAuditor.cs`
- `pkg/runtime/runtimes/csharp/PulseRPC/ContractDelta.cs`
- `pkg/runtime/runtimes/csharp/PulseRPC/DiffEngine.cs`

**Java Runtime:**
- `com/bitmechanic/pulserpc/ContractAuditor.java`
- `com/bitmechanic/pulserpc/ContractDelta.java`
- `com/bitmechanic/pulserpc/DiffEngine.java`

### Files Modified

- `pkg/runtime/runtimes/csharp/PulseRPC/Client.cs` - Add auditor support and VerifyCompatibility
- `pkg/runtime/runtimes/java/com/bitmechanic/pulserpc/Client.java` - Add auditor support and verifyCompatibility
- `pkg/generator/csharp_client_server.go` - Embed IDL_JSON, add auditor options
- `pkg/generator/java_client_server.go` - Embed IDL_JSON, add auditor options
- `docs/languages/csharp/reference.md` - Add verification documentation
- `docs/languages/java/reference.md` - Add verification documentation

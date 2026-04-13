## Why

Nelson Minar wrote in 2006 that "the moment you need to change anything, the type signature changes and all the clients that were built to your earlier protocol spec break." This brittleness problem afflicts IDL-based RPC systems when clients and servers evolve independently without a way to detect drift. PulseRPC's current checksum mechanism detects that a change occurred but provides no insight into *what* changed or *how severe* the difference is. We need a compatibility verification system that advises callers on structural differences and their impact.

## What Changes

- New `pkg/compat` package with directional structural diff engine
- New `VerifyCompatibility()` method on Go client runtime
- New `ContractAuditor` interface for pluggable handling of verification results
- Built-in auditors: `NoOpAuditor`, `LoggingAuditor`, `FailFastAuditor`
- Client code generation embeds `IDL_JSON` (client's local copy) alongside server's embedded IDL
- New client options: `WithAuditor(auditor)` and `VerifyOnBootstrap()`
- Quickstart documentation demonstrating verification patterns

## Capabilities

### New Capabilities

- `contract-compatibility-verification`: Detects and classifies structural differences between client and server IDLs at runtime. Reports directional deltas (client has more/less than server), severity classification (error/warning/info), and runtime impact assessment. Supports pluggable auditors for custom handling.

## Impact

- New Go package: `pkg/compat/` (diff engine, auditor interface, built-in implementations)
- Modified: Go client runtime (`pkg/runtime/runtimes/go/pulserpc/client.go`) - new fields and `VerifyCompatibility()` method
- Modified: Go code generator (`pkg/generator/go_client_server.go`) - embed `IDL_JSON` in client code
- Modified: Documentation - quickstart guide update

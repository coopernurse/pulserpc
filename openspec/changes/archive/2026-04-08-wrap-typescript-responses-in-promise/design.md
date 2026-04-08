## Context

The TypeScript generator (`pkg/generator/ts_client_server.go`) produces interface stubs in `server.ts` where method return types are synchronous (e.g., `echo(s: string): string`), but the client implementations in `client.ts` return async `Promise<T>` (e.g., `async echo(s: string): Promise<string>`).

This creates an inconsistency: implementations must cast the async return type to match the synchronous interface, or the interface must be duplicated.

## Goals / Non-Goals

**Goals:**
- Wrap interface method return types in `Promise<>` when a return type exists
- Ensure interface contracts match client implementations
- Maintain backward compatibility for void-returning methods

**Non-Goals:**
- Changing the runtime library architecture
- Modifying other language generators
- Adding new spec requirements

## Decisions

### Decision: Wrap return types in `Promise<>` for interface methods only where a return type exists

**Rationale:** The client methods already correctly return `Promise<T>`. The issue is that interface stubs (which represent the abstract service contract) declare synchronous signatures.

**Approach:** Modify `writeInterfaceStubTs` function (around line 535) to wrap the return type in `Promise<>` when:
1. A return type exists (is not nil/void)
2. The method is not marked as return-optional (or handle both cases)

**Alternatives considered:**
- Wrap ALL interface methods in Promise (even void) - rejected because void-returning async methods are `Promise<void>`, not `void`
- Change client methods to be sync - rejected because RPC calls are inherently async

### Decision: Update generator code in place

**Rationale:** This is a localized change to one function. No new files needed.

**Approach:** In `writeInterfaceStubTs`, after computing `returnType`, conditionally wrap with `Promise<>`:
```go
if returnType != "" && returnType != "void" {
    if method.ReturnOptional {
        returnType = "Promise<" + returnType + " | null>"
    } else {
        returnType = "Promise<" + returnType + ">"
    }
}
```

## Risks / Trade-offs

**[Risk]** Existing implementations may rely on synchronous interface signatures
→ **Mitigation:** This is a breaking change for TypeScript users implementing the interface. Users will need to update their implementations to return `Promise<T>`.

**[Risk]** TypeScript runtime may have assumptions about sync interfaces
→ **Mitigation:** Verify runtime type compatibility after generator change.

## Migration Plan

1. Update generator in `pkg/generator/ts_client_server.go`
2. Verify generated output matches expected `Promise<>` wrapping
3. Update quickstart tests if they implement the interface directly
4. Regenerate quickstart examples to verify

## Open Questions

- Should void-returning methods also return `Promise<void>` in interfaces? (Currently they are `void` which is inconsistent with async client)

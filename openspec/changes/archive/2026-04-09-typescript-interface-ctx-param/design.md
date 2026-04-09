## Context

The TypeScript generator produces abstract interface classes in `server.ts` where each method signature reflects only the IDL-defined parameters. Server implementations override these methods to provide business logic. The runtime `Server.call()` method invokes handlers with extracted params, but has no mechanism to pass additional context (e.g., HTTP headers for auth).

## Goals / Non-Goals

**Goals:**
- Allow server-side implementations to receive transport-specific metadata (HTTP headers, auth info) via a `ctx` parameter
- Require `ctx` parameter on all generated interface methods (no backward compatibility concern)
- Pass `ctx` through from `Server.call()` to the handler method

**Non-Goals:**
- Client-side changes (client stubs do not need ctx)
- Transport layer modifications
- Changes to other language generators

## Decisions

### Decision: `ctx` as a required parameter (not optional)

The proposal uses required parameter for cleaner semantics. All interface methods will have:
```typescript
abstract Create(req: CreateRequest, ctx: Record<string, any>): Promise<Response>
```

**Rationale**: Simpler mental model. No conditional checks for undefined. New implementations must handle ctx regardless.

### Decision: Pass `ctx` after request params in `Server.call()`

Handler methods receive args followed by ctx:
```typescript
const args = Object.values(params);
const result = await func(...args, ctx);
```

**Rationale**: Natural ordering - business params first, context second. Consistent with how middleware frameworks typically handle context.

### Decision: `ctx: Record<string, any>` type

Using `Record<string, any>` allows implementations to store any metadata without type constraints.

**Alternatives considered**:
- `ctx: { headers?: Record<string, string>, userId?: string, ... }` - Too rigid, requires schema updates as needs grow
- `ctx: Map<string, any>` - Less idiomatic TypeScript, no native JSON serialization

## Risks / Trade-offs

- **Existing implementations break**: Service classes implementing generated interfaces will fail to compile until `ctx` parameter is added. **No migration path provided** - this is an intentional breaking change for TypeScript users.
- **No runtime validation**: `ctx` is untyped - implementations must trust the caller provides correct shape. **Mitigation**: Documentation guides users on expected ctx contents.
- **Handler signature assumes positional params**: `Server.call()` spreads `Object.values(params)` then appends `ctx`. This works for current IDL semantics (positional params converted to named). If future IDL adds complex param structures, this pattern may need revisiting.

## Migration Plan

1. Update generator to emit `ctx` parameter in interface stubs
2. Update runtime `Server.call()` to accept and pass `ctx`
3. Update test server generator to include `ctx` in generated implementations
4. Regenerate any examples that test interface compatibility
5. Users must update their implementations to match new signatures
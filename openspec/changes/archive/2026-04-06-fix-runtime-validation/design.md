## Context

The PulseRPC runtime provides server and client libraries for 5 languages: Go, Python, TypeScript, C#, and Java. Each runtime has a validation module that enforces the types defined in the IDL against actual request/response data.

The validation logic currently differs across languages in three critical areas:

1. **Integer validation**: Go accepts float64 for int fields. Python, C#, and Java strictly require their native int type. TypeScript checks `Number.isInteger()`. This means JSON `5.0` passes in Go but may fail in C#/Java.

2. **Optional field handling**: TypeScript allows both `null` and `undefined` for optional fields. Go, Python, C#, and Java only consider `null` as a valid value for optional fields. A TypeScript client sending `undefined` for an optional field would work with a TS server but fail with other servers.

3. **Error propagation via `raises()`**: The IDL supports a `raises(ErrorType)` clause on methods to declare which errors a method can return. Only Go correctly catches server exceptions and returns them as `RPCError` to the client. Other languages have `RPCError` classes but don't use them in the server dispatch.

## Goals / Non-Goals

**Goals:**
- JSON number `5.0` passes int validation (maps to integer 5)
- JSON number `5.1` fails int validation (has fractional part)
- JSON number `-3.0` passes int validation (maps to -3)
- TypeScript optional fields only accept `null`, not `undefined`
- All 5 runtimes return `RPCError` to clients when a method's `raises()` clause is triggered
- Enums are case-sensitive across all languages

**Non-Goals:**
- Cross-language client-server testing (deferred)
- Changes to the IDL parser or generator output
- Modifying the wire protocol or transport layer
- Adding new error types or changing error code values

## Decisions

### Decision 1: Int validation should accept whole-number floats

**Choice**: When validating an `int` field, accept any JSON number that equals a whole number (value % 1 === 0).

**Rationale**: JSON has no integer type - all numbers are floating point. A Go server sending `{"count": 5.0}` is valid. The receiving runtime should normalize this to integer 5 if needed, not reject it.

**Implementation per language**:

| Language | Current | Change |
|----------|---------|--------|
| Go | Accepts float64 | Keep as-is |
| Python | `isinstance(value, int)` | Accept int OR float where `value == int(value)` |
| TypeScript | `Number.isInteger(value)` | Accept if `isInteger` OR `value === Math.floor(value)` |
| C# | `value is not int` | Accept int, long, double where `value == Math.Floor(value)` |
| Java | `!value instanceof Integer` | Accept Integer, Long, Double where `value.doubleValue() % 1 == 0` |

### Decision 2: TypeScript optional fields reject undefined

**Choice**: Change TypeScript validation to only accept `null` for optional fields, not `undefined`.

**Rationale**: Other 4 languages use `null` (or equivalent). TypeScript should be consistent.

**Implementation**: In `validation.ts`, change line 151 from:
```typescript
if (value === null || value === undefined)
```
to:
```typescript
if (value === null)
```

### Decision 3: RPCError creation and propagation for TS and C#

**Choice**: Create `RPCError` class in TypeScript and C# if they don't exist, and update all runtime servers to catch exceptions and return them as `RPCError` responses.

**Rationale**: The `raises()` clause declares what errors a method can return. The server should catch those errors and format them as JSON-RPC error responses.

**Implementation**:
- TypeScript: Create `rpc.ts` with `RPCError` class, update `server.ts` dispatch to catch and format errors
- C#: Create `RPCError.cs`, update `Server.cs` dispatch
- Python: Update `server.py` dispatch to catch and propagate errors (RPCError already exists)
- Java: Update `Server.java` dispatch to catch and propagate errors (RPCError already exists)
- Go: Already correct, no changes needed

### Decision 4: Enum case sensitivity

**Choice**: Enums are case-sensitive. `add` != `Add` != `ADD`. No changes needed - all languages already treat enums as case-sensitive strings.

## Risks / Trade-offs

**[Breaking Change] TypeScript optional field behavior**: Existing TS clients sending `undefined` for optional fields will start failing after this change. This is a breaking change but improves consistency.

**Migration**: Deploy validation updates first (all 5 runtimes should have consistent int validation), then deploy `raises()` propagation updates. The order matters because int validation is more fundamental.

## Open Questions

1. Should we add a `validate_float_strict` mode that rejects `5.0` for float fields? (Currently `5.0` would pass as a float since it's a valid number)
2. Do we need to update the Go validation to reject `5.1` for int even though Go accepts it? (Go currently accepts any float64 as int if it passes the validation - this is already consistent with the new approach)

## Migration Plan

1. **Phase 1 - Int validation fix** (all 5 runtimes):
   - Deploy Go, Python, TS, C#, Java validation updates simultaneously
   - This ensures cross-language compatibility before any client/server deployments

2. **Phase 2 - TypeScript undefined fix**:
   - Deploy TS runtime update
   - Existing TS clients using `undefined` for optionals will need to use `null`

3. **Phase 3 - raises() propagation** (Python, TS, C#, Java):
   - Deploy server updates to all 4 runtimes
   - RPCError will start appearing in error responses for methods with `raises()` clause

4. **Phase 4 - New integration tests**:
   - Deploy `test_numeric_types.sh` and `test_enum_case.sh`
   - Run against all 5 runtimes to verify consistency
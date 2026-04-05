## Context

The Python code generator (`pkg/generator/python_client_server.go`) currently generates `types.py` with dataclasses for structs and classes for enums. Error codes are documented in comments in the pulse file but are not parsed or generated into Python constants. The existing `examples/quickstart/python/checkout/errors.py` shows the desired pattern - a class `Err` with constants like `CartNotFound = 1001` - but this file is manually created, not generated.

## Goals / Non-Goals

**Goals:**
- Generate Python error constant classes from `errors {}` block in IDL
- Output should follow existing `Err`/`ErrJsonRpc` class pattern from `errors.py`
- Add `errors {}` block to `examples/quickstart/checkout.pulse` with proper IDL syntax
- Update `examples/quickstart/python/my_server.py` to use generated constants

**Non-Goals:**
- Not changing runtime behavior (RPCError class remains the same)
- Not adding validation that errors must be caught
- Not affecting code generation for other languages (Go, TypeScript, Java, C#)
- Not generating new tests - just updating the quickstart example

## Decisions

### 1. Where to add error generation

**Decision:** Add error constant generation to `generateTypesPyForNamespace()` function

**Rationale:** The `types.py` file is the natural place for type-related constants. Errors are part of the namespace's type system. The alternative of creating a separate `errors.py` would complicate the output structure and require additional `__init__.py` updates.

**Alternative considered:** Create a separate `errors.py` file per namespace, similar to how `types.py`, `server.py`, and `client.py` are separate. Rejected because:
- Errors are simpler than types/enums and fit naturally in `types.py`
- Reduces number of generated files
- Follows the existing manually-created `errors.py` pattern which is part of the namespace

### 2. Generated Python structure

**Decision:** Generate an `Err` class containing all error constants for the namespace, plus standard JSON-RPC errors in `ErrJsonRpc` class

**Rationale:** Matches the existing pattern in `examples/quickstart/python/checkout/errors.py`:
```python
class ErrJsonRpc:
    ParseError = -32700
    InvalidRequest = -32600
    ...

class Err:
    CartNotFound = 1001
    CartEmpty = 1002
    ...
```

### 3. IDL parser changes needed

**Decision:** The parser already has `ErrorDef` struct and `errors {}` block parsing. Only the Python generator needs modification.

**Rationale:** The `pkg/parser/parser.go` already handles `errors {}` syntax and populates `IDL.Errors`. The `ErrorDef` struct has `Name`, `Code`, `Message`, and `Comment` fields. The Python generator simply needs to iterate over these and generate Python code.

### 4. Checkout.pulse changes

**Decision:** Replace the comment block with an `errors {}` block:
```
errors {
    1001 CartNotFound "Cart doesn't exist"
    1002 CartEmpty "Cart has no items"
    1003 PaymentFailed "Payment method rejected"
    1004 OutOfStock "Insufficient inventory"
    1005 InvalidAddress "Shipping address validation failed"
}
```

**Rationale:** This uses the existing IDL grammar. The error codes and names match the existing comments.

## Risks / Trade-offs

1. **Manual edits to generated files** → If developers manually edit `types.py`, their changes will be lost on regeneration. Mitigation: Document that files are generated and should not be manually edited.

2. **Error codes in comments removed** → The old comments documented error codes. These should be removed once the `errors {}` block is added. The generated code will provide the authoritative source.

3. **my_server.py uses -32602 for validation errors** → The standard JSON-RPC error for invalid params (-32602) is used in `my_server.py` for "product not found". This should use `ErrJsonRpc.InvalidParams` constant for consistency.

## Open Questions

1. Should the `raises()` clause on methods be validated against declared errors? (Not in scope for this change, but worth noting)

## ADDED Requirements

### Requirement: Error constants generated in types.py

The Python code generator SHALL generate error constant classes in `types.py` when an `errors {}` block is present in the IDL.

#### Scenario: Generate Err class with namespace errors
- **WHEN** IDL contains `errors { 1001 CartNotFound "Cart not found" }` in namespace `checkout`
- **THEN** generated `checkout/types.py` SHALL contain a class `Err` with constant `CartNotFound = 1001`

#### Scenario: Generate ErrJsonRpc class with standard errors
- **WHEN** Python code is generated from any IDL
- **THEN** generated `types.py` SHALL contain a class `ErrJsonRpc` with standard JSON-RPC error codes:
  - `ParseError = -32700`
  - `InvalidRequest = -32600`
  - `MethodNotFound = -32601`
  - `InvalidParams = -32602`
  - `InternalError = -32603`

#### Scenario: Multiple errors in namespace
- **WHEN** IDL contains multiple error declarations in the same namespace
- **THEN** generated `Err` class SHALL contain all error constants with correct code values

### Requirement: Checkout.pulse uses errors block

The quickstart `checkout.pulse` SHALL use the `errors {}` IDL syntax to declare error codes instead of comments.

#### Scenario: Checkout.pulse has errors block
- **WHEN** the quickstart is loaded
- **THEN** `examples/quickstart/checkout.pulse` SHALL contain an `errors {}` block with CartNotFound, CartEmpty, PaymentFailed, OutOfStock, and InvalidAddress

#### Scenario: Error comments removed
- **WHEN** the errors block is added to checkout.pulse
- **THEN** the comment lines defining error codes SHALL be removed

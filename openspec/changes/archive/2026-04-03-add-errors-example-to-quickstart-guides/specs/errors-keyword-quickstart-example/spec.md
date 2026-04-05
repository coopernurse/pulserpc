## ADDED Requirements

### Requirement: Errors keyword example in quickstart IDL

The checkout.pulse IDL in quickstart guides SHALL use the formal `errors` keyword to declare error codes and `raises()` clauses on interface methods to specify which errors each method can raise.

#### Scenario: Error declarations in IDL
- **WHEN** user reviews the quickstart checkout.idl
- **THEN** they see an `errors {}` block defining CartNotFound, CartEmpty, PaymentFailed, OutOfStock, and InvalidAddress

#### Scenario: Raises clause on createOrder method
- **WHEN** user reviews the OrderService interface in checkout.idl
- **THEN** the createOrder method includes `raises(CartNotFound, CartEmpty, PaymentFailed, OutOfStock, InvalidAddress)`

### Requirement: Errors keyword section in language quickstarts

Each language quickstart (Python, Go, TypeScript, Java, C#) SHALL include a "Defining Service Errors" section that demonstrates the `errors` keyword and `raises()` clause syntax.

#### Scenario: Quickstart shows errors syntax
- **WHEN** user completes the "Define the Service" section of any language quickstart
- **THEN** they see a subsection showing how errors are declared and used with raises()

#### Scenario: Quickstart links to full errors documentation
- **WHEN** user views the "Defining Service Errors" section
- **THEN** they see a link to the full Error Handling guide at docs/idl-guide/errors.md

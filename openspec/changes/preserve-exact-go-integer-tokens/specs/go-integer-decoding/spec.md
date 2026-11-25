## ADDED Requirements

### Requirement: Decode exact JSON integer values in the Go runtime

The Go runtime SHALL accept plain, decimal, and exponent JSON number tokens at an IDL integer field when the represented value is a mathematical integer in signed 64-bit range. It SHALL preserve the value exactly for required and optional integer fields. A fractional or out-of-range token SHALL fail integer validation.

#### Scenario: Exact wide exponent input

- **WHEN** an integer field receives `9007199254740993e0`
- **THEN** its handler receives `9007199254740993`

#### Scenario: Fraction rounded by float64

- **WHEN** an integer field receives `9007199254740992.5`
- **THEN** the runtime rejects the argument before dispatch

#### Scenario: Optional numeric input

- **WHEN** an optional numeric field receives an accepted number that remains json.Number
- **THEN** the handler receives a pointer containing that number in the declared Go type

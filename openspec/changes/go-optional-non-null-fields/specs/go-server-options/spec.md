## ADDED Requirements

### Requirement: Optional non-null fields in Go requests

The Go server SHALL allow a request-validation policy to reject explicit null for selected optional IDL struct fields. It SHALL supply the declared struct name and the exact field key. It SHALL leave omission, responses, and direct contract validation unchanged.

#### Scenario: Selected optional field

- **WHEN** an optional request field selected by the policy is explicitly null
- **THEN** request validation reports an invalid argument

#### Scenario: Omitted field

- **WHEN** that field is omitted
- **THEN** the request proceeds through the remaining validation and dispatch

## ADDED Requirements

### Requirement: Method-aware parameter error mapping

The Go server SHALL offer an optional method-aware mapper for parameter decode and validation errors. It SHALL receive the qualified method, error phase, and underlying error. A nil mapping SHALL fall back to the existing mapper and response. The option SHALL NOT map envelope or handler errors.

#### Scenario: Method-specific parameter error

- **WHEN** validation fails for a registered method and its method-aware mapper returns an RPC error
- **THEN** the server returns that error with the request ID

#### Scenario: Existing mapper fallback

- **WHEN** the method-aware mapper returns nil
- **THEN** the server applies the existing parameter-error policy

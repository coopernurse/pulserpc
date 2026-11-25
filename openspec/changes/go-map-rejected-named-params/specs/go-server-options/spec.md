## ADDED Requirements

### Requirement: Optional mapping for rejected named parameters

The Go runtime SHALL provide an opt-in positional parameter mode that rejects object parameters through the parameter decode mappers. It SHALL fall back to the existing invalid-request error and data. Existing modes SHALL remain unchanged.

#### Scenario: Method-specific rejection

- **WHEN** a registered method receives named parameters in mapped positional mode
- **THEN** the runtime applies its decode error mappers without invoking the handler

#### Scenario: No mapping

- **WHEN** no mapper replaces the rejection
- **THEN** the runtime returns the existing invalid-request response

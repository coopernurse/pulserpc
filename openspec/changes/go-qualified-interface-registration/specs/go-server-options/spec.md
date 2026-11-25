## ADDED Requirements

### Requirement: Explicit qualified interface registration in Go

A Go server SHALL support registering an interface by its qualified IDL name when the namespace is declared. The contract SHALL prefer exact declared names over derived qualified names. The server SHALL dispatch only the registered spelling.

#### Scenario: Qualified registration

- **WHEN** a server registers the qualified name of a namespaced interface
- **THEN** it validates and dispatches requests under that name

#### Scenario: Unregistered bare alias

- **WHEN** a server registers only the qualified name and receives a call using the bare name
- **THEN** it returns method not found

package openapi

import (
	"errors"
)

// Parser handles loading and validating OpenAPI 3.x specifications.
// Phase 1: Stub implementation

// Parser represents an OpenAPI spec parser.
type Parser struct{}

// NewParser creates a new OpenAPI parser.
func NewParser() *Parser {
	return &Parser{}
}

// ParseFile loads and validates an OpenAPI 3.0/3.1 YAML or JSON file.
// It resolves $ref references (local and external) and extracts
// components/schemas into a normalized type map.
//
// Phase 2 will implement the actual parsing logic.
func (p *Parser) ParseFile(filename string) (*ParsedSpec, error) {
	return nil, errors.Join(ErrNotImplemented, errors.New("parser.ParseFile: not implemented yet - see Phase 2"))
}

// ParsedSpec represents a parsed OpenAPI specification.
type ParsedSpec struct {
	// TODO: Add fields in Phase 2:
	// - Version (3.0 or 3.1)
	// - Info (title, description, version)
	// - Paths (endpoints)
	// - Components (schemas, parameters, etc.)
	// - Security schemes
}

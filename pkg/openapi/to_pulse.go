package openapi

import (
	"errors"

	"github.com/coopernurse/pulserpc/pkg/parser"
)

// ToPulse converts OpenAPI specifications to Pulse IDL.
// Phase 1: Stub implementation

// Generator generates Pulse IDL from parsed OpenAPI specifications.
type ToPulseGenerator struct {
	// Parser is used to load OpenAPI specs
	Parser *Parser
}

// NewToPulseGenerator creates a new OpenAPI → Pulse generator.
func NewToPulseGenerator() *ToPulseGenerator {
	return &ToPulseGenerator{
		Parser: NewParser(),
	}
}

// Generate reads an OpenAPI spec file and generates Pulse IDL.
//
// Phase 3 will implement the actual generation logic including:
// - Namespace derivation from info.title
// - Struct generation from components/schemas
// - Enum generation from string enums
// - Interface/method generation from paths/operations
func (g *ToPulseGenerator) Generate(openapiFile string) (*parser.IDL, error) {
	return nil, errors.Join(ErrNotImplemented, errors.New("ToPulseGenerator.Generate: not implemented yet - see Phase 3"))
}

// GenerateToFile reads an OpenAPI spec file and writes Pulse IDL to a file.
func (g *ToPulseGenerator) GenerateToFile(openapiFile, outputFile string) error {
	return errors.Join(ErrNotImplemented, errors.New("ToPulseGenerator.GenerateToFile: not implemented yet - see Phase 3"))
}

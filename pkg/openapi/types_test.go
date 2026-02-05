package openapi

import (
	"testing"
)

// TestTypesNotImplemented verifies that stub functions return appropriate errors.
// Tests will be expanded in Phase 2+.
func TestTypesNotImplemented(t *testing.T) {
	// Phase 1: Verify the error exists
	if ErrNotImplemented == nil {
		t.Fatal("ErrNotImplemented should not be nil")
	}

	if ErrNotImplemented.Error() == "" {
		t.Error("ErrNotImplemented should have a non-empty message")
	}
}

// TODO: Add tests in Phase 2:
// - TestTypeMappingOpenAPIToPulse
// - TestTypeMappingPulseToOpenAPI
// - TestTranslationContext
// - TestWarningsCollection

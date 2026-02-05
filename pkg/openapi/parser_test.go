package openapi

import (
	"os"
	"testing"
)

// TestParserNotImplemented verifies that stub functions return appropriate errors.
// Tests will be expanded in Phase 2.
func TestParserNotImplemented(t *testing.T) {
	p := NewParser()
	if p == nil {
		t.Fatal("NewParser() should not return nil")
	}

	// Verify that ParseFile returns an error for now
	_, err := p.ParseFile("test.yaml")
	if err == nil {
		t.Error("ParseFile should return an error in Phase 1")
	}
}

// TestParserExists verifies testdata directory exists for Phase 2.
func TestParserExists(t *testing.T) {
	testdataDir := "testdata"
	info, err := os.Stat(testdataDir)
	if err != nil {
		t.Fatalf("testdata directory should exist: %v", err)
	}

	if !info.IsDir() {
		t.Fatal("testdata should be a directory")
	}
}

// TODO: Add tests in Phase 2:
// - TestParsePetstoreOpenAPI30
// - TestParsePetstoreOpenAPI31
// - TestParseLocalRefs
// - TestParseExternalRefs
// - TestCircularReferenceDetection
// - TestPrimitiveTypeMapping
// - TestArrayTypeMapping
// - TestObjectTypeMapping
// - TestEnumTypeMapping
// - TestAllOfMapping
// - TestOneOfWarning
// - TestAnyOfWarning
// - TestBinaryFormatError

package openapi

import (
	"testing"
)

// TestTypesNotImplemented verifies that stub functions return appropriate errors.
// Tests will be expanded in Phase 2+.
func TestTypesNotImplemented(t *testing.T) {
	// Phase 2: Verify the error exists
	if ErrNotImplemented == nil {
		t.Fatal("ErrNotImplemented should not be nil")
	}

	if ErrNotImplemented.Error() == "" {
		t.Error("ErrNotImplemented should have a non-empty message")
	}
}

// TestMapOpenAPITypeToPulse tests the OpenAPI to Pulse type mapping.
func TestMapOpenAPITypeToPulse(t *testing.T) {
	tests := []struct {
		name         string
		openAPIType  *OpenAPIType
		expectedName string
		expectedOpt  bool
	}{
		{
			name: "string",
			openAPIType: &OpenAPIType{
				Type: "string",
			},
			expectedName: "string",
			expectedOpt:  false,
		},
		{
			name: "integer",
			openAPIType: &OpenAPIType{
				Type: "integer",
			},
			expectedName: "int",
			expectedOpt:  false,
		},
		{
			name: "integer int32",
			openAPIType: &OpenAPIType{
				Type:   "integer",
				Format: "int32",
			},
			expectedName: "int",
			expectedOpt:  false,
		},
		{
			name: "integer int64",
			openAPIType: &OpenAPIType{
				Type:   "integer",
				Format: "int64",
			},
			expectedName: "int",
			expectedOpt:  false,
		},
		{
			name: "number",
			openAPIType: &OpenAPIType{
				Type: "number",
			},
			expectedName: "float",
			expectedOpt:  false,
		},
		{
			name: "number float",
			openAPIType: &OpenAPIType{
				Type:   "number",
				Format: "float",
			},
			expectedName: "float",
			expectedOpt:  false,
		},
		{
			name: "number double",
			openAPIType: &OpenAPIType{
				Type:   "number",
				Format: "double",
			},
			expectedName: "float",
			expectedOpt:  false,
		},
		{
			name: "boolean",
			openAPIType: &OpenAPIType{
				Type: "boolean",
			},
			expectedName: "bool",
			expectedOpt:  false,
		},
		{
			name: "nullable string",
			openAPIType: &OpenAPIType{
				Type:     "string",
				Nullable: true,
			},
			expectedName: "string",
			expectedOpt:  true,
		},
		{
			name: "nullable integer",
			openAPIType: &OpenAPIType{
				Type:     "integer",
				Nullable: true,
			},
			expectedName: "int",
			expectedOpt:  true,
		},
		{
			name:         "nil defaults to string",
			openAPIType:  nil,
			expectedName: "string",
			expectedOpt:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MapOpenAPITypeToPulse(tt.openAPIType)
			if result.Name != tt.expectedName {
				t.Errorf("expected type name %s, got %s", tt.expectedName, result.Name)
			}
			if result.IsOptional != tt.expectedOpt {
				t.Errorf("expected optional %v, got %v", tt.expectedOpt, result.IsOptional)
			}
		})
	}
}

// TestMakeOptional tests the optional type creation.
func TestMakeOptional(t *testing.T) {
	t.Run("make int optional", func(t *testing.T) {
		result := MakeOptional(TypeInt)
		if !result.IsOptional {
			t.Error("type should be optional")
		}
		if result.Name != "int" {
			t.Errorf("expected name int, got %s", result.Name)
		}
	})

	t.Run("make custom type optional", func(t *testing.T) {
		custom := MakeCustomType("User")
		result := MakeOptional(custom)
		if !result.IsOptional {
			t.Error("type should be optional")
		}
		if result.Name != "User" {
			t.Errorf("expected name User, got %s", result.Name)
		}
	})
}

// TestMakeArrayType tests the array type creation.
func TestMakeArrayType(t *testing.T) {
	t.Run("array of string", func(t *testing.T) {
		result := MakeArrayType(TypeString)
		if !result.IsArray {
			t.Error("type should be array")
		}
		if result.ArrayElementType == nil {
			t.Fatal("ArrayElementType should not be nil")
		}
		if result.ArrayElementType.Name != "string" {
			t.Errorf("expected element type string, got %s", result.ArrayElementType.Name)
		}
	})

	t.Run("array of custom type", func(t *testing.T) {
		custom := MakeCustomType("Pet")
		result := MakeArrayType(custom)
		if !result.IsArray {
			t.Error("type should be array")
		}
		if result.ArrayElementType.Name != "Pet" {
			t.Errorf("expected element type Pet, got %s", result.ArrayElementType.Name)
		}
	})
}

// TestMakeMapType tests the map type creation.
func TestMakeMapType(t *testing.T) {
	t.Run("map of string", func(t *testing.T) {
		result := MakeMapType(TypeString)
		if !result.IsMap {
			t.Error("type should be map")
		}
		if result.MapValueType == nil {
			t.Fatal("MapValueType should not be nil")
		}
		if result.MapValueType.Name != "string" {
			t.Errorf("expected value type string, got %s", result.MapValueType.Name)
		}
	})

	t.Run("map of int", func(t *testing.T) {
		result := MakeMapType(TypeInt)
		if !result.IsMap {
			t.Error("type should be map")
		}
		if result.MapValueType.Name != "int" {
			t.Errorf("expected value type int, got %s", result.MapValueType.Name)
		}
	})
}

// TestMakeCustomType tests the custom type creation.
func TestMakeCustomType(t *testing.T) {
	result := MakeCustomType("User")
	if !result.IsCustom {
		t.Error("type should be custom")
	}
	if result.Name != "User" {
		t.Errorf("expected name User, got %s", result.Name)
	}
}

// TestWarningString tests the warning string representation.
func TestWarningString(t *testing.T) {
	tests := []struct {
		name     string
		warning  Warning
		expected string
	}{
		{
			name: "warning with location",
			warning: Warning{
				Level:    "warning",
				Message:  "test message",
				Location: "file.yaml:10",
			},
			expected: "file.yaml:10: warning: test message",
		},
		{
			name: "warning without location",
			warning: Warning{
				Level:   "warning",
				Message: "test message",
			},
			expected: "warning: test message",
		},
		{
			name: "error with location",
			warning: Warning{
				Level:    "error",
				Message:  "test error",
				Location: "schema:5",
			},
			expected: "schema:5: error: test error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.warning.String()
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// TestWarningsEmpty tests that new warnings collection is empty.
func TestWarningsEmpty(t *testing.T) {
	w := NewWarnings()
	if w.Count() != 0 {
		t.Errorf("expected 0 warnings, got %d", w.Count())
	}
	if w.HasErrors() {
		t.Error("should not have errors")
	}
}

// TestSchemaInfoDefaults tests the SchemaInfo default values.
func TestSchemaInfoDefaults(t *testing.T) {
	// When creating a SchemaInfo directly, Required and Properties are nil
	// They are only initialized by the parser
	info := &SchemaInfo{
		Name: "TestSchema",
	}

	// These maps are not initialized when creating SchemaInfo directly
	if info.Required != nil {
		t.Error("Required map should be nil when created directly")
	}
	if info.Properties != nil {
		t.Error("Properties map should be nil when created directly")
	}
	if info.IsCircular {
		t.Error("IsCircular should be false by default")
	}
	if info.IsEnum {
		t.Error("IsEnum should be false by default")
	}
	if info.IsObject {
		t.Error("IsObject should be false by default")
	}
}

// TestPulseTypeDefaults tests the PulseType default values.
func TestPulseTypeDefaults(t *testing.T) {
	pt := PulseType{
		Name: "test",
	}

	if pt.IsOptional {
		t.Error("IsOptional should be false by default")
	}
	if pt.IsArray {
		t.Error("IsArray should be false by default")
	}
	if pt.IsMap {
		t.Error("IsMap should be false by default")
	}
	if pt.IsPrimitive {
		t.Error("IsPrimitive should be false by default")
	}
	if pt.IsCustom {
		t.Error("IsCustom should be false by default")
	}
}

// TestPrimitiveTypes tests that predefined primitive types are correct.
func TestPrimitiveTypes(t *testing.T) {
	tests := []struct {
		name   string
		pType  PulseType
		expect string
	}{
		{"TypeString", TypeString, "string"},
		{"TypeInt", TypeInt, "int"},
		{"TypeFloat", TypeFloat, "float"},
		{"TypeBool", TypeBool, "bool"},
		{"TypeVoid", TypeVoid, "void"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.pType.IsPrimitive {
				t.Errorf("%s should be primitive", tt.name)
			}
			if tt.pType.String() != tt.expect {
				t.Errorf("expected %s, got %s", tt.expect, tt.pType.String())
			}
		})
	}
}

// TestSanitizeComment tests comment sanitization.
func TestSanitizeComment(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal comment",
			input:    "This is a comment",
			expected: "This is a comment",
		},
		{
			name:     "comment with null bytes",
			input:    "This\x00has\x00nulls",
			expected: "Thishasnulls",
		},
		{
			name:     "comment with carriage returns",
			input:    "Line1\r\nLine2",
			expected: "Line1\nLine2",
		},
		{
			name:     "comment with leading/trailing spaces",
			input:    "  spaces  ",
			expected: "spaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeComment(tt.input)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// TestIsReservedOpenAPIWord tests the reserved word checker.
func TestIsReservedOpenAPIWord(t *testing.T) {
	tests := []struct {
		name     string
		word     string
		expected bool
	}{
		{"$ref is reserved", "$ref", true},
		{"schema is reserved", "schema", true},
		{"example is reserved", "example", true},
		{"content is reserved", "content", true},
		{"allowReserved is reserved", "allowReserved", true},
		{"normal word is not reserved", "myParam", false},
		{"empty string is not reserved", "", false},
		{"mixed case is not reserved", "Schema", false},
		{"x-custom is not reserved", "x-custom", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsReservedOpenAPIWord(tt.word)
			if result != tt.expected {
				t.Errorf("expected %v for '%s', got %v", tt.expected, tt.word, result)
			}
		})
	}
}

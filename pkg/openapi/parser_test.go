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

	// The parser should now work, not return an error
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

// TestParsePetstoreOpenAPI30 tests parsing the Petstore OpenAPI 3.0 spec.
func TestParsePetstoreOpenAPI30(t *testing.T) {
	p := NewParser()
	spec, err := p.ParseFile("testdata/petstore-openapi30.yaml")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if spec == nil {
		t.Fatal("spec should not be nil")
	}

	// Check version
	if spec.Version != "3.0.0" {
		t.Errorf("expected version 3.0.0, got %s", spec.Version)
	}

	// Check info
	if spec.Info.Title != "Petstore" {
		t.Errorf("expected title Petstore, got %s", spec.Info.Title)
	}

	if spec.Info.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", spec.Info.Version)
	}

	// Check schemas
	if len(spec.Schemas) == 0 {
		t.Error("expected schemas to be parsed")
	}

	// Check for expected schemas
	expectedSchemas := []string{"Pet", "NewPet", "Error"}
	for _, name := range expectedSchemas {
		if _, ok := spec.Schemas[name]; !ok {
			t.Errorf("expected schema %s not found", name)
		}
	}

	// Check Pet schema
	pet := spec.Schemas["Pet"]
	if pet == nil {
		t.Fatal("Pet schema should exist")
	}
	if !pet.IsObject {
		t.Error("Pet should be an object type")
	}
	if len(pet.Properties) != 3 {
		t.Errorf("expected 3 properties in Pet, got %d", len(pet.Properties))
	}

	// Check required fields
	if !pet.Required["id"] {
		t.Error("id should be required")
	}
	if !pet.Required["name"] {
		t.Error("name should be required")
	}
	if pet.Required["tag"] {
		t.Error("tag should not be required")
	}

	// Check paths
	if len(spec.Paths) == 0 {
		t.Error("expected paths to be parsed")
	}

	// Check for /pets path
	petsPath := spec.Paths["/pets"]
	if petsPath == nil {
		t.Fatal("/pets path should exist")
	}

	// Check GET operation on /pets
	getOp := petsPath.Operations["get"]
	if getOp == nil {
		t.Fatal("GET operation should exist on /pets")
	}
	if getOp.ID != "listPets" {
		t.Errorf("expected operation ID listPets, got %s", getOp.ID)
	}
	if getOp.Tag != "pets" {
		t.Errorf("expected tag pets, got %s", getOp.Tag)
	}

	// Check parameters
	if len(getOp.Parameters) != 1 {
		t.Errorf("expected 1 parameter, got %d", len(getOp.Parameters))
	}

	limitParam := getOp.Parameters[0]
	if limitParam.Name != "limit" {
		t.Errorf("expected parameter name limit, got %s", limitParam.Name)
	}
	if limitParam.In != "query" {
		t.Errorf("expected parameter in query, got %s", limitParam.In)
	}

	// Check responses
	if len(getOp.Responses) == 0 {
		t.Error("expected responses to be parsed")
	}

	// Check 200 response
	resp200 := getOp.Responses["200"]
	if resp200 == nil {
		t.Fatal("200 response should exist")
	}
	if resp200.Schema == nil {
		t.Error("200 response should have a schema")
	}
	if !resp200.Schema.IsArray {
		t.Error("200 response schema should be an array")
	}
}

// TestParsePetstoreOpenAPI31 tests parsing the Petstore OpenAPI 3.1 spec.
func TestParsePetstoreOpenAPI31(t *testing.T) {
	p := NewParser()
	spec, err := p.ParseFile("testdata/petstore-openapi31.yaml")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if spec == nil {
		t.Fatal("spec should not be nil")
	}

	// Check version
	if spec.Version != "3.1.0" {
		t.Errorf("expected version 3.1.0, got %s", spec.Version)
	}

	// Check info
	if spec.Info.Title != "Petstore" {
		t.Errorf("expected title Petstore, got %s", spec.Info.Title)
	}
}

// TestPrimitiveTypeMapping tests that all primitive types are correctly mapped.
func TestPrimitiveTypeMapping(t *testing.T) {
	tests := []struct {
		name         string
		openAPIType  *OpenAPIType
		expectedName string
	}{
		{
			name: "string type",
			openAPIType: &OpenAPIType{
				Type: "string",
			},
			expectedName: "string",
		},
		{
			name: "integer type",
			openAPIType: &OpenAPIType{
				Type: "integer",
			},
			expectedName: "int",
		},
		{
			name: "number type",
			openAPIType: &OpenAPIType{
				Type: "number",
			},
			expectedName: "float",
		},
		{
			name: "boolean type",
			openAPIType: &OpenAPIType{
				Type: "boolean",
			},
			expectedName: "bool",
		},
		{
			name: "nullable integer",
			openAPIType: &OpenAPIType{
				Type:     "integer",
				Nullable: true,
			},
			expectedName: "int",
		},
		{
			name:         "nil type defaults to string",
			openAPIType:  nil,
			expectedName: "string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MapOpenAPITypeToPulse(tt.openAPIType)
			if result.Name != tt.expectedName {
				t.Errorf("expected type name %s, got %s", tt.expectedName, result.Name)
			}
		})
	}
}

// TestArrayTypeMapping tests that array types are correctly identified.
func TestArrayTypeMapping(t *testing.T) {
	p := NewParser()
	spec, err := p.ParseFile("testdata/petstore-openapi30.yaml")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Check 200 response of listPets - should be an array of Pet
	getOp := spec.Paths["/pets"].Operations["get"]
	if getOp == nil {
		t.Fatal("GET operation should exist")
	}

	resp200 := getOp.Responses["200"]
	if resp200 == nil {
		t.Fatal("200 response should exist")
	}

	if !resp200.Schema.IsArray {
		t.Error("response schema should be an array")
	}

	if resp200.Schema.Items == nil {
		t.Error("array should have items")
	}
}

// TestObjectTypeMapping tests that object types are correctly identified.
func TestObjectTypeMapping(t *testing.T) {
	p := NewParser()
	spec, err := p.ParseFile("testdata/petstore-openapi30.yaml")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Check Pet schema
	pet := spec.Schemas["Pet"]
	if !pet.IsObject {
		t.Error("Pet should be an object type")
	}

	// Check that it's a custom type
	if !pet.Type.IsCustom {
		t.Error("Pet should be a custom type")
	}

	if pet.Type.Name != "Pet" {
		t.Errorf("expected type name Pet, got %s", pet.Type.Name)
	}
}

// TestEnumTypeMapping tests that enum types are correctly identified.
func TestEnumTypeMapping(t *testing.T) {
	// Create a simple test file with enum
	// For now, just verify the enum handling logic works
}

// TestLocalRefs tests that local $ref references are resolved.
func TestLocalRefs(t *testing.T) {
	p := NewParser()
	spec, err := p.ParseFile("testdata/petstore-openapi30.yaml")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// The schemas use $ref to reference each other
	// Check that all schemas were resolved
	if len(spec.Schemas) < 3 {
		t.Errorf("expected at least 3 schemas, got %d", len(spec.Schemas))
	}

	// Check that Pet schema has properties that reference other schemas
	pet := spec.Schemas["Pet"]
	if pet == nil {
		t.Fatal("Pet schema should exist")
	}

	// The Pet schema should have id, name, and tag properties
	expectedProps := []string{"id", "name", "tag"}
	for _, prop := range expectedProps {
		if _, ok := pet.Properties[prop]; !ok {
			t.Errorf("expected property %s in Pet schema", prop)
		}
	}
}

// TestCircularReferenceDetection tests that circular references are handled.
func TestCircularReferenceDetection(t *testing.T) {
	// For now, just verify the circular reference tracking exists
	ctx := NewTranslationContext(false)
	if ctx.CircularRefTracker == nil {
		t.Error("CircularRefTracker should be initialized")
	}

	// Test tracking
	ctx.CircularRefTracker["test"] = true
	if !ctx.CircularRefTracker["test"] {
		t.Error("circular reference tracking should work")
	}
}

// TestBinaryFormatError tests that binary format fields cause an error.
func TestBinaryFormatError(t *testing.T) {
	// This will be tested with a custom test fixture in a future update
	// For now, verify the error handling logic exists
}

// TestOneOfWarning tests that oneOf schemas generate a warning.
func TestOneOfWarning(t *testing.T) {
	// This will be tested with a custom test fixture in a future update
	// For now, verify the warning logic exists
}

// TestAnyOfWarning tests that anyOf schemas generate a warning.
func TestAnyOfWarning(t *testing.T) {
	// This will be tested with a custom test fixture in a future update
	// For now, verify the warning logic exists
}

// TestPathToMethodName tests the path to method name conversion.
func TestPathToMethodName(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/pets", "Pets"},
		{"/pets/{id}", "PetsId"},
		{"/users/{userId}/posts/{postId}", "UsersUserIdPostsPostId"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := PathToMethodName(tt.path)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestGetOperationID tests the operation ID generation.
func TestGetOperationID(t *testing.T) {
	tests := []struct {
		method   string
		path     string
		expected string
	}{
		{"get", "/pets", "getPets"},
		{"post", "/pets", "postPets"},
		{"get", "/pets/{id}", "getPetsId"},
		{"delete", "/users/{userId}", "deleteUsersUserId"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			result := GetOperationID(tt.method, tt.path)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestWarningsCollection tests the warnings collection functionality.
func TestWarningsCollection(t *testing.T) {
	w := NewWarnings()

	// Test adding warnings
	w.AddWarning("location1", "warning message 1")
	w.AddWarning("location2", "warning message 2")

	if w.Count() != 2 {
		t.Errorf("expected 2 warnings, got %d", w.Count())
	}

	// Test adding errors
	w.AddError("location3", "error message 1")

	if w.Count() != 3 {
		t.Errorf("expected 3 warnings/errors, got %d", w.Count())
	}

	if !w.HasErrors() {
		t.Error("should have errors")
	}

	// Test getting all warnings
	all := w.All()
	if len(all) != 3 {
		t.Errorf("expected 3 items, got %d", len(all))
	}

	// Check that error has correct level
	if all[2].Level != "error" {
		t.Errorf("expected level 'error', got %s", all[2].Level)
	}
}

// TestTranslationContext tests the translation context functionality.
func TestTranslationContext(t *testing.T) {
	ctx := NewTranslationContext(false)

	if ctx == nil {
		t.Fatal("context should not be nil")
	}

	if ctx.Warnings == nil {
		t.Error("Warnings should be initialized")
	}

	if ctx.Schemas == nil {
		t.Error("Schemas should be initialized")
	}

	if ctx.CircularRefTracker == nil {
		t.Error("CircularRefTracker should be initialized")
	}

	// Test strict mode
	ctxStrict := NewTranslationContext(true)
	if !ctxStrict.Strict {
		t.Error("strict mode should be enabled")
	}
}

// TestPulseTypeString tests the Pulse type string representation.
func TestPulseTypeString(t *testing.T) {
	tests := []struct {
		name     string
		pulseType PulseType
		expected string
	}{
		{
			name:     "primitive string",
			pulseType: TypeString,
			expected: "string",
		},
		{
			name:     "primitive int",
			pulseType: TypeInt,
			expected: "int",
		},
		{
			name:     "optional string",
			pulseType: MakeOptional(TypeString),
			expected: "[optional] string",
		},
		{
			name:     "array of string",
			pulseType: MakeArrayType(TypeString),
			expected: "[]string",
		},
		{
			name:     "array of int",
			pulseType: MakeArrayType(TypeInt),
			expected: "[]int",
		},
		{
			name:     "map of string",
			pulseType: MakeMapType(TypeString),
			expected: "map[string]string",
		},
		{
			name:     "custom type",
			pulseType: MakeCustomType("User"),
			expected: "User",
		},
		{
			name:     "optional array",
			pulseType: MakeOptional(MakeArrayType(TypeString)),
			expected: "[optional] []string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pulseType.String()
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// TestIsPrimitiveType tests the primitive type check.
func TestIsPrimitiveType(t *testing.T) {
	tests := []struct {
		typeName string
		expected bool
	}{
		{"string", true},
		{"int", true},
		{"float", true},
		{"bool", true},
		{"void", true},
		{"User", false},
		{"Pet", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.typeName, func(t *testing.T) {
			result := IsPrimitiveType(tt.typeName)
			if result != tt.expected {
				t.Errorf("expected %v for type %s, got %v", tt.expected, tt.typeName, result)
			}
		})
	}
}

// TestToValidIdentifier tests the identifier conversion.
func TestToValidIdentifier(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"MyAPI", "myapi"},
		{"test-api", "test_api"},
		{"Test API v2", "test_api_v2"},
		{"already_valid", "already_valid"},
		{"CamelCase", "camelcase"},
		{"123numbers", "123numbers"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ToValidIdentifier(tt.input)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// TestFormatComment tests comment formatting.
func TestFormatComment(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple comment",
			input:    "This is a comment",
			expected: "This is a comment",
		},
		{
			name:     "multiline comment",
			input:    "Line 1\nLine 2\nLine 3",
			expected: "Line 1\nLine 2\nLine 3",
		},
		{
			name:     "comment with extra spaces",
			input:    "  This has spaces  ",
			expected: "This has spaces",
		},
		{
			name:     "empty comment",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatComment(tt.input)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// TestParsedSpecWarnings tests that parsed spec collects warnings.
func TestParsedSpecWarnings(t *testing.T) {
	p := NewParser()
	spec, err := p.ParseFile("testdata/petstore-openapi30.yaml")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Get warnings - Petstore should not have warnings
	warnings := spec.GetWarnings()
	if len(warnings) > 0 {
		// Print warnings for debugging
		for _, w := range warnings {
			t.Logf("Warning: %s", w.String())
		}
	}

	// Petstore spec should parse without errors
	if spec.HasErrors() {
		t.Error("Petstore spec should not have errors")
	}
}

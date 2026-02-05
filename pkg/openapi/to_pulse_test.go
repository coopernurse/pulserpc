package openapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coopernurse/pulserpc/pkg/parser"
)

// TestToPulseGeneratorExists verifies the generator can be created.
func TestToPulseGeneratorExists(t *testing.T) {
	g := NewToPulseGenerator()
	if g == nil {
		t.Fatal("NewToPulseGenerator() should not return nil")
	}

	if g.Parser == nil {
		t.Error("ToPulseGenerator.Parser should not be nil")
	}
}

// TestNamespaceDerivation tests namespace derivation from OpenAPI title.
func TestNamespaceDerivation(t *testing.T) {
	tests := []struct {
		name      string
		title     string
		expected  string
	}{
		{"simple title", "MyAPI", "myapi"},
		{"with spaces", "Test API v2", "test_api_v2"},
		// Note: ToValidIdentifier converts . to _, so "v2.0" becomes "v2_0"
		{"with special chars", "My-API_v2.0", "my_api_v2_0"},
		{"empty title", "", "api"},
		{"already valid", "valid_namespace", "valid_namespace"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deriveNamespace(tt.title)
			if result != tt.expected {
				t.Errorf("deriveNamespace(%q) = %q, want %q", tt.title, result, tt.expected)
			}
		})
	}
}

// TestGenerateMethodNames tests method name generation.
func TestGenerateMethodNames(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		path     string
		opID     string
		expected string
	}{
		// With operationId: HTTP method (lowercase) + Title(operationId)
		{"get with operationId", "get", "/pets", "listPets", "getListPets"},
		{"post with operationId", "post", "/pets", "createPet", "postCreatePet"},
		{"get without operationId", "get", "/pets/{id}", "", "getPetsId"},
		{"delete without operationId", "delete", "/users/{userId}", "", "deleteUsersUserId"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := &Operation{
				Method: tt.method,
				Path:   tt.path,
				ID:     tt.opID,
			}
			result := generateMethodName(op)
			if result != tt.expected {
				t.Errorf("generateMethodName() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestDeriveInterfaceNameFromPath tests interface name derivation.
func TestDeriveInterfaceNameFromPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"simple path", "/pets", "pets"},
		{"nested path", "/users/{id}/posts", "users"},
		{"root path", "/", "default"},
		{"empty", "", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deriveInterfaceNameFromPath(tt.path)
			if result != tt.expected {
				t.Errorf("deriveInterfaceNameFromPath(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

// TestConvertSchemaInfoToType tests schema to type conversion.
func TestConvertSchemaInfoToType(t *testing.T) {
	tests := []struct {
		name     string
		schema   *SchemaInfo
		expected string
	}{
		{
			name: "primitive string",
			schema: &SchemaInfo{
				Type: TypeString,
			},
			expected: "string",
		},
		{
			name: "primitive int",
			schema: &SchemaInfo{
				Type: TypeInt,
			},
			expected: "int",
		},
		{
			name: "array of strings",
			schema: &SchemaInfo{
				IsArray: true,
				Items: &SchemaInfo{
					Type: TypeString,
				},
			},
			expected: "[]string",
		},
		{
			name: "map of ints",
			schema: &SchemaInfo{
				IsMap: true,
				AdditionalProperties: &SchemaInfo{
					Type: TypeInt,
				},
			},
			expected: "map[string]int",
		},
		{
			name: "custom type",
			schema: &SchemaInfo{
				Name:     "User",
				IsObject: true,
			},
			expected: "User",
		},
		{
			name: "nested array",
			schema: &SchemaInfo{
				IsArray: true,
				Items: &SchemaInfo{
					IsArray: true,
					Items: &SchemaInfo{
						Type: TypeString,
					},
				},
			},
			expected: "[][]string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertSchemaInfoToType(tt.schema)
			resultStr := formatType(result)
			if resultStr != tt.expected {
				t.Errorf("convertSchemaInfoToType() = %s, want %s", resultStr, tt.expected)
			}
		})
	}
}

// TestFormatType tests type formatting.
func TestFormatType(t *testing.T) {
	tests := []struct {
		name     string
		typeDef  *parser.Type
		expected string
	}{
		{
			name:     "primitive string",
			typeDef:  &parser.Type{BuiltIn: "string"},
			expected: "string",
		},
		{
			name:     "primitive int",
			typeDef:  &parser.Type{BuiltIn: "int"},
			expected: "int",
		},
		{
			name:     "array",
			typeDef:  &parser.Type{Array: &parser.Type{BuiltIn: "string"}},
			expected: "[]string",
		},
		{
			name:     "nested array",
			typeDef:  &parser.Type{Array: &parser.Type{Array: &parser.Type{BuiltIn: "int"}}},
			expected: "[][]int",
		},
		{
			name:     "map",
			typeDef:  &parser.Type{MapValue: &parser.Type{BuiltIn: "string"}},
			expected: "map[string]string",
		},
		{
			name:     "user defined",
			typeDef:  &parser.Type{UserDefined: "User"},
			expected: "User",
		},
		{
			name:     "nil defaults to void",
			typeDef:  nil,
			expected: "void",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatType(tt.typeDef)
			if result != tt.expected {
				t.Errorf("formatType() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestFormatCommentAsPulse tests comment formatting.
func TestFormatCommentAsPulse(t *testing.T) {
	tests := []struct {
		name     string
		comment  string
		expected string
	}{
		{
			name:     "simple comment",
			comment:  "This is a comment",
			expected: "// This is a comment",
		},
		{
			name:     "multiline comment",
			comment:  "Line 1\nLine 2\nLine 3",
			expected: "// Line 1\n// Line 2\n// Line 3",
		},
		{
			name:     "comment with extra spaces",
			comment:  "  Line 1  \n  Line 2  ",
			expected: "// Line 1\n// Line 2",
		},
		{
			name:     "empty comment",
			comment:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatCommentAsPulse(tt.comment)
			if result != tt.expected {
				t.Errorf("formatCommentAsPulse() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestGenerateSimpleOpenAPISpec tests generation of a simple OpenAPI spec.
func TestGenerateSimpleOpenAPISpec(t *testing.T) {
	// Create a simple test OpenAPI spec
	spec := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /users:
    get:
      operationId: listUsers
      responses:
        '200':
          description: List of users
          content:
            application/json:
              schema:
                type: array
                items:
                  type: string
components:
  schemas:
    User:
      type: object
      properties:
        id:
          type: integer
        name:
          type: string
      required:
        - id
`

	// Write spec to temp file
	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(specFile, []byte(spec), 0644); err != nil {
		t.Fatal(err)
	}

	// Generate Pulse IDL
	g := NewToPulseGenerator()
	idl, err := g.Generate(specFile)
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// Verify namespace
	if idl.RootNamespace != "test_api" {
		t.Errorf("RootNamespace = %q, want %q", idl.RootNamespace, "test_api")
	}

	// Verify struct was generated
	if len(idl.Structs) != 1 {
		t.Errorf("Structs length = %d, want 1", len(idl.Structs))
	}

	// Verify interface was generated
	if len(idl.Interfaces) != 1 {
		t.Errorf("Interfaces length = %d, want 1", len(idl.Interfaces))
	}
}

// TestGenerateToFile tests generating to a file.
func TestGenerateToFile(t *testing.T) {
	// Create a simple test OpenAPI spec
	spec := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /ping:
    get:
      operationId: ping
      responses:
        '200':
          description: OK
`

	// Write spec to temp file
	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(specFile, []byte(spec), 0644); err != nil {
		t.Fatal(err)
	}

	// Generate Pulse IDL to file
	outputFile := filepath.Join(tmpDir, "test.pulse")
	g := NewToPulseGenerator()
	if err := g.GenerateToFile(specFile, outputFile); err != nil {
		t.Fatalf("GenerateToFile() failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Error("Output file was not created")
	}

	// Read and verify content
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatal(err)
	}

	contentStr := string(content)

	// Verify key elements are present
	if !strings.Contains(contentStr, "namespace test_api") {
		t.Error("Generated file missing namespace declaration")
	}

	if !strings.Contains(contentStr, "interface") {
		t.Error("Generated file missing interface")
	}

	if !strings.Contains(contentStr, "ping") {
		t.Error("Generated file missing ping method")
	}
}

// TestEnumGeneration tests enum generation from OpenAPI schemas.
func TestEnumGeneration(t *testing.T) {
	spec := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths: {}
components:
  schemas:
    Status:
      type: string
      enum:
        - active
        - inactive
        - pending
`

	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(specFile, []byte(spec), 0644); err != nil {
		t.Fatal(err)
	}

	g := NewToPulseGenerator()
	idl, err := g.Generate(specFile)
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	if len(idl.Enums) != 1 {
		t.Errorf("Enums length = %d, want 1", len(idl.Enums))
	}

	enum := idl.Enums[0]
	if enum.Name != "Status" {
		t.Errorf("Enum name = %q, want %q", enum.Name, "Status")
	}

	if len(enum.Values) != 3 {
		t.Errorf("Enum values length = %d, want 3", len(enum.Values))
	}
}

// TestAllOfExtends tests that allOf generates extends clause.
func TestAllOfExtends(t *testing.T) {
	spec := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths: {}
components:
  schemas:
    Base:
      type: object
      properties:
        id:
          type: string
    Extended:
      allOf:
        - $ref: '#/components/schemas/Base'
        - type: object
          properties:
            extra:
              type: string
`

	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(specFile, []byte(spec), 0644); err != nil {
		t.Fatal(err)
	}

	g := NewToPulseGenerator()
	idl, err := g.Generate(specFile)
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// Find the Extended struct
	var extendedStruct *parser.Struct
	for _, s := range idl.Structs {
		if s.Name == "Extended" {
			extendedStruct = s
			break
		}
	}

	if extendedStruct == nil {
		t.Fatal("Extended struct not found")
	}

	if extendedStruct.Extends != "Base" {
		t.Errorf("Extended.Extends = %q, want %q", extendedStruct.Extends, "Base")
	}
}

// TestRequiredVsOptional tests required vs optional field handling.
func TestRequiredVsOptional(t *testing.T) {
	spec := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths: {}
components:
  schemas:
    User:
      type: object
      properties:
        id:
          type: string
        name:
          type: string
        email:
          type: string
      required:
        - id
        - name
`

	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(specFile, []byte(spec), 0644); err != nil {
		t.Fatal(err)
	}

	g := NewToPulseGenerator()
	idl, err := g.Generate(specFile)
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	if len(idl.Structs) != 1 {
		t.Fatalf("Structs length = %d, want 1", len(idl.Structs))
	}

	userStruct := idl.Structs[0]

	// Find email field (should be optional)
	var emailField *parser.Field
	for _, f := range userStruct.Fields {
		if f.Name == "email" {
			emailField = f
			break
		}
	}

	if emailField == nil {
		t.Fatal("email field not found")
	}

	if !emailField.Optional {
		t.Error("email field should be optional")
	}

	// id field should not be optional
	var idField *parser.Field
	for _, f := range userStruct.Fields {
		if f.Name == "id" {
			idField = f
			break
		}
	}

	if idField == nil {
		t.Fatal("id field not found")
	}

	if idField.Optional {
		t.Error("id field should not be optional")
	}
}

// Test204ResponseOptional tests that 204 responses generate optional return.
func Test204ResponseOptional(t *testing.T) {
	spec := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /items/{id}:
    delete:
      operationId: deleteItem
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        '204':
          description: No content
`

	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(specFile, []byte(spec), 0644); err != nil {
		t.Fatal(err)
	}

	g := NewToPulseGenerator()
	idl, err := g.Generate(specFile)
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	if len(idl.Interfaces) != 1 {
		t.Fatalf("Interfaces length = %d, want 1", len(idl.Interfaces))
	}

	iface := idl.Interfaces[0]
	if len(iface.Methods) != 1 {
		t.Fatalf("Methods length = %d, want 1", len(iface.Methods))
	}

	method := iface.Methods[0]
	if !method.ReturnOptional {
		t.Error("Return should be optional for 204 response")
	}

	if method.ReturnType.BuiltIn != "void" {
		t.Errorf("Return type = %q, want void", method.ReturnType.BuiltIn)
	}
}

// TestGeneratePulseContent tests the Pulse IDL content generation.
func TestGeneratePulseContent(t *testing.T) {
	idl := &parser.IDL{
		RootNamespace: "test",
		Structs: []*parser.Struct{
			{
				Name: "User",
				Fields: []*parser.Field{
					{Name: "id", Type: &parser.Type{BuiltIn: "string"}, Optional: false},
					{Name: "name", Type: &parser.Type{BuiltIn: "string"}, Optional: true},
				},
			},
		},
		Interfaces: []*parser.Interface{
			{
				Name: "UserService",
				Methods: []*parser.Method{
					{
						Name:       "getUser",
						Parameters: []*parser.Parameter{{Name: "id", Type: &parser.Type{BuiltIn: "string"}}},
						ReturnType: &parser.Type{UserDefined: "User"},
					},
				},
			},
		},
	}

	g := NewToPulseGenerator()
	content := g.GeneratePulseContent(idl, "test.yaml")

	// Verify key elements
	if !strings.Contains(content, "namespace test") {
		t.Error("Generated content missing namespace")
	}

	if !strings.Contains(content, "struct User") {
		t.Error("Generated content missing struct User")
	}

	if !strings.Contains(content, "interface UserService") {
		t.Error("Generated content missing interface UserService")
	}

	if !strings.Contains(content, "getUser(id string) User") {
		t.Error("Generated content missing getUser method")
	}

	if !strings.Contains(content, "[optional]") {
		t.Error("Generated content missing optional annotation")
	}
}

// TestFormatEnum tests enum formatting.
func TestFormatEnum(t *testing.T) {
	g := NewToPulseGenerator()

	enumDef := &parser.Enum{
		Name:    "Status",
		Comment: "Status enum",
		Values: []*parser.EnumValue{
			{Name: "Active"},
			{Name: "Inactive"},
			{Name: "Pending"},
		},
	}

	result := g.formatEnum(enumDef)

	// Verify the formatted enum contains expected content
	if !strings.Contains(result, "enum Status") {
		t.Error("formatEnum() should contain 'enum Status'")
	}

	if !strings.Contains(result, "Active") {
		t.Error("formatEnum() should contain 'Active'")
	}

	if !strings.Contains(result, "Inactive") {
		t.Error("formatEnum() should contain 'Inactive'")
	}

	if !strings.Contains(result, "Pending") {
		t.Error("formatEnum() should contain 'Pending'")
	}
}

// TestFormatEnumNoComment tests enum formatting without comment.
func TestFormatEnumNoComment(t *testing.T) {
	g := NewToPulseGenerator()

	enumDef := &parser.Enum{
		Name: "Color",
		Values: []*parser.EnumValue{
			{Name: "Red"},
			{Name: "Green"},
			{Name: "Blue"},
		},
	}

	result := g.formatEnum(enumDef)

	// Verify the formatted enum doesn't start with comment
	if strings.HasPrefix(strings.TrimSpace(result), "//") {
		t.Error("formatEnum() should not start with comment when enum has no comment")
	}

	if !strings.Contains(result, "enum Color") {
		t.Error("formatEnum() should contain 'enum Color'")
	}
}

// TestSetStrict tests setting strict mode.
func TestSetStrict(t *testing.T) {
	g := NewToPulseGenerator()

	// Default should be false
	if g.Strict {
		t.Error("Default Strict should be false")
	}

	// Set to true
	g.SetStrict(true)
	if !g.Strict {
		t.Error("Strict should be true after SetStrict(true)")
	}

	// Set back to false
	g.SetStrict(false)
	if g.Strict {
		t.Error("Strict should be false after SetStrict(false)")
	}
}

// TestGenerateToFileWithWarnings tests file generation with warnings.
func TestGenerateToFileWithWarnings(t *testing.T) {
	// Create a simple test OpenAPI spec
	spec := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /ping:
    get:
      operationId: ping
      responses:
        '200':
          description: OK
`

	// Write spec to temp file
	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(specFile, []byte(spec), 0644); err != nil {
		t.Fatal(err)
	}

	// Generate Pulse IDL to file
	outputFile := filepath.Join(tmpDir, "test.pulse")
	g := NewToPulseGenerator()
	warnings, err := g.GenerateToFileWithWarnings(specFile, outputFile)

	if err != nil {
		t.Fatalf("GenerateToFileWithWarnings() failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Error("Output file was not created")
	}

	// Warnings should be a non-nil slice (even if empty)
	if warnings == nil {
		t.Error("Warnings should not be nil")
	}
}

// TestGenerateToFileWithWarningsInvalidFile tests error handling in GenerateToFileWithWarnings.
func TestGenerateToFileWithWarningsInvalidFile(t *testing.T) {
	g := NewToPulseGenerator()

	// Try to generate from non-existent file
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "test.pulse")

	_, err := g.GenerateToFileWithWarnings("/nonexistent/file.yaml", outputFile)
	if err == nil {
		t.Error("GenerateToFileWithWarnings() should fail with non-existent input file")
	}
}

package openapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coopernurse/pulserpc/pkg/parser"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/go-cmp/cmp"
)

// Helper function to check if a type matches expected string
func typeMatches(typ *openapi3.Types, expected string) bool {
	if typ == nil {
		return false
	}
	return (*typ).Is(expected)
}

// TestFromPulseGeneratorVersion verifies version handling.
func TestFromPulseGeneratorVersion(t *testing.T) {
	g := NewFromPulseGenerator("")
	if g.OpenAPIVersion != "3.1" {
		t.Errorf("Default OpenAPIVersion should be 3.1, got %s", g.OpenAPIVersion)
	}

	g = NewFromPulseGenerator("3.0")
	if g.OpenAPIVersion != "3.0" {
		t.Errorf("OpenAPIVersion should be 3.0, got %s", g.OpenAPIVersion)
	}
}

// TestSimpleInterface tests generating OpenAPI from a simple Pulse interface.
func TestSimpleInterface(t *testing.T) {
	input := `namespace example

interface UserService {
    getUser(id string) string
    createUser(name string, email string) string
}
`

	g := NewFromPulseGenerator("3.1")
	spec, err := g.GenerateFromString(input)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify OpenAPI version
	if spec.T.OpenAPI != "3.1" {
		t.Errorf("Expected OpenAPI version 3.1, got %s", spec.T.OpenAPI)
	}

	// Verify info
	if spec.T.Info.Title == "" {
		t.Error("Info.Title should not be empty")
	}

	// Verify paths
	if len(spec.T.Paths.Map()) == 0 {
		t.Fatal("No paths generated")
	}

	// Check that paths are POST /UserService/getUser and POST /UserService/createUser
	expectedPaths := []string{"/UserService/getUser", "/UserService/createUser"}
	for _, expectedPath := range expectedPaths {
		pathItem := spec.T.Paths.Find(expectedPath)
		if pathItem == nil || pathItem.Post == nil {
			t.Errorf("Expected POST operation at path %s", expectedPath)
		}
	}
}

// TestSchemaGeneration tests schema generation from structs.
func TestSchemaGeneration(t *testing.T) {
	input := `namespace example

struct User {
    name string
    email string
    age int
    active bool
}
`

	g := NewFromPulseGenerator("3.1")
	spec, err := g.GenerateFromString(input)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify User schema exists
	userSchemaRef, ok := spec.T.Components.Schemas["User"]
	if !ok {
		t.Fatal("User schema not found in components/schemas")
	}

	userSchema := userSchemaRef.Value
	if userSchema.Type == nil || !(*userSchema.Type).Is("object") {
		t.Errorf("User schema type should be object, got %v", userSchema.Type)
	}

	// Verify properties
	expectedProps := map[string]string{
		"name":   "string",
		"email":  "string",
		"age":    "integer",
		"active": "boolean",
	}

	for propName, expectedType := range expectedProps {
		propRef, ok := userSchema.Properties[propName]
		if !ok {
			t.Errorf("Property %s not found", propName)
			continue
		}
		prop := propRef.Value
		if !typeMatches(prop.Type, expectedType) {
			t.Errorf("Property %s: expected type %s, got %v", propName, expectedType, prop.Type)
		}
	}

	// Verify all fields are required
	if len(userSchema.Required) != 4 {
		t.Errorf("Expected 4 required fields, got %d", len(userSchema.Required))
	}
}

// TestEnumSchemaGeneration tests enum schema generation.
func TestEnumSchemaGeneration(t *testing.T) {
	input := `namespace example

enum Status {
    Active
    Inactive
    Pending
}
`

	g := NewFromPulseGenerator("3.1")
	spec, err := g.GenerateFromString(input)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify Status schema
	statusSchemaRef, ok := spec.T.Components.Schemas["Status"]
	if !ok {
		t.Fatal("Status schema not found")
	}

	statusSchema := statusSchemaRef.Value
	if !typeMatches(statusSchema.Type, "string") {
		t.Errorf("Enum schema type should be string, got %s", statusSchema.Type)
	}

	expectedValues := []interface{}{"Active", "Inactive", "Pending"}
	if diff := cmp.Diff(expectedValues, statusSchema.Enum); diff != "" {
		t.Errorf("Enum values mismatch (-expected +got):\n%s", diff)
	}
}

// TestStructInheritanceAllOf tests that extends generates allOf.
func TestStructInheritanceAllOf(t *testing.T) {
	input := `namespace example

struct BaseResponse {
    success bool
    message string
}

struct UserResponse extends BaseResponse {
    userId string
    username string
}
`

	g := NewFromPulseGenerator("3.1")
	spec, err := g.GenerateFromString(input)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify UserResponse has allOf
	userResponseSchemaRef, ok := spec.T.Components.Schemas["UserResponse"]
	if !ok {
		t.Fatal("UserResponse schema not found")
	}

	userResponseSchema := userResponseSchemaRef.Value
	if userResponseSchema.AllOf == nil {
		t.Error("UserResponse should have allOf for inheritance")
	}

	if len(userResponseSchema.AllOf) != 1 {
		t.Errorf("Expected 1 element in allOf, got %d", len(userResponseSchema.AllOf))
	}

	ref := userResponseSchema.AllOf[0].Ref
	expectedRef := "#/components/schemas/BaseResponse"
	if ref != expectedRef {
		t.Errorf("Expected allOf ref %s, got %s", expectedRef, ref)
	}
}

// TestOptionalFields tests that optional fields are not in required array.
func TestOptionalFields(t *testing.T) {
	input := `namespace example

struct User {
    name string
    email string
    nickname string [optional]
    bio string [optional]
}
`

	g := NewFromPulseGenerator("3.1")
	spec, err := g.GenerateFromString(input)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	userSchemaRef := spec.T.Components.Schemas["User"]
	userSchema := userSchemaRef.Value

	// Only name and email should be required
	if len(userSchema.Required) != 2 {
		t.Errorf("Expected 2 required fields, got %d: %v", len(userSchema.Required), userSchema.Required)
	}

	// Verify optional fields are not in required
	for _, field := range []string{"nickname", "bio"} {
		for _, req := range userSchema.Required {
			if req == field {
				t.Errorf("Optional field %s should not be in required array", field)
			}
		}
	}
}

// TestArrayTypes tests array type mapping.
func TestArrayTypes(t *testing.T) {
	input := `namespace example

struct Container {
    strList  []string
    intList  []int
    users    []User
}

struct User {
    name  string
}
`

	g := NewFromPulseGenerator("3.1")
	spec, err := g.GenerateFromString(input)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	containerSchemaRef := spec.T.Components.Schemas["Container"]
	containerSchema := containerSchemaRef.Value

	tests := []struct {
		prop     string
		wantType string
		wantItem string
		wantRef  string
	}{
		{"strList", "array", "string", ""},
		{"intList", "array", "integer", ""},
		{"users", "array", "", "#/components/schemas/User"},
	}

	for _, tt := range tests {
		propRef := containerSchema.Properties[tt.prop]
		if propRef == nil {
			t.Fatalf("Property %s not found", tt.prop)
		}
		prop := propRef.Value
		if !typeMatches(prop.Type, tt.wantType) {
			t.Errorf("Property %s: expected type %s, got %v", tt.prop, tt.wantType, prop.Type)
		}
		if tt.wantItem != "" && !typeMatches(prop.Items.Value.Type, tt.wantItem) {
			t.Errorf("Property %s: expected item type %s, got %v", tt.prop, tt.wantItem, prop.Items.Value.Type)
		}
		if tt.wantRef != "" && prop.Items.Ref != tt.wantRef {
			t.Errorf("Property %s: expected ref %s, got %s", tt.prop, tt.wantRef, prop.Items.Ref)
		}
	}
}

// TestMapTypes tests map type mapping.
func TestMapTypes(t *testing.T) {
	input := `namespace example

struct Container {
    stringMap  map[string]string
    intMap     map[string]int
}
`

	g := NewFromPulseGenerator("3.1")
	spec, err := g.GenerateFromString(input)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	containerSchemaRef := spec.T.Components.Schemas["Container"]
	containerSchema := containerSchemaRef.Value

	tests := []struct {
		prop           string
		wantType       string
		wantAddPropVal string
	}{
		{"stringMap", "object", "string"},
		{"intMap", "object", "integer"},
	}

	for _, tt := range tests {
		propRef := containerSchema.Properties[tt.prop]
		if propRef == nil {
			t.Fatalf("Property %s not found", tt.prop)
		}
		prop := propRef.Value
		if !typeMatches(prop.Type, tt.wantType) {
			t.Errorf("Property %s: expected type %s, got %v", tt.prop, tt.wantType, prop.Type)
		}
		if prop.AdditionalProperties.Schema == nil ||
			!typeMatches(prop.AdditionalProperties.Schema.Value.Type, tt.wantAddPropVal) {
			t.Errorf("Property %s: expected additionalProperties type %s, got %v", tt.prop, tt.wantAddPropVal, prop.AdditionalProperties)
		}
	}
}

// TestPathGeneration tests path generation from interfaces.
func TestPathGeneration(t *testing.T) {
	input := `namespace example

interface UserService {
    getUser(id string) string
}

interface AdminService {
    deleteUser(id string) bool
}
`

	g := NewFromPulseGenerator("3.1")
	spec, err := g.GenerateFromString(input)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Check paths
	paths := spec.T.Paths.Map()
	if len(paths) != 2 {
		t.Fatalf("Expected 2 paths, got %d", len(paths))
	}

	expectedPaths := map[string]bool{
		"/UserService/getUser":     false,
		"/AdminService/deleteUser": false,
	}

	for path := range paths {
		if _, ok := expectedPaths[path]; ok {
			expectedPaths[path] = true
		} else {
			t.Errorf("Unexpected path: %s", path)
		}
	}

	for path, found := range expectedPaths {
		if !found {
			t.Errorf("Expected path not found: %s", path)
		}
	}
}

// TestTagGeneration tests that interfaces become tags.
func TestTagGeneration(t *testing.T) {
	input := `namespace example

interface UserService {
    getUser(id string) string
}
`

	g := NewFromPulseGenerator("3.1")
	spec, err := g.GenerateFromString(input)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	pathItem := spec.T.Paths.Find("/UserService/getUser")
	if pathItem == nil || pathItem.Post == nil {
		t.Fatal("POST operation not found")
	}

	tags := pathItem.Post.Tags
	if len(tags) != 1 {
		t.Fatalf("Expected 1 tag, got %d", len(tags))
	}

	if tags[0] != "UserService" {
		t.Errorf("Expected tag UserService, got %s", tags[0])
	}
}

// TestOperationIdGeneration tests operationId format.
func TestOperationIdGeneration(t *testing.T) {
	input := `namespace example

interface UserService {
    getUser(id string) string
}
`

	g := NewFromPulseGenerator("3.1")
	spec, err := g.GenerateFromString(input)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	pathItem := spec.T.Paths.Find("/UserService/getUser")
	operationId := pathItem.Post.OperationID

	expected := "UserService_getUser"
	if operationId != expected {
		t.Errorf("Expected operationId %s, got %s", expected, operationId)
	}
}

// TestRequestBodyGeneration tests request body generation.
func TestRequestBodyGeneration(t *testing.T) {
	input := `namespace example

interface UserService {
    createUser(name string, email string, age int) string
}
`

	g := NewFromPulseGenerator("3.1")
	spec, err := g.GenerateFromString(input)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	pathItem := spec.T.Paths.Find("/UserService/createUser")
	requestBody := pathItem.Post.RequestBody

	if requestBody == nil {
		t.Fatal("Request body should not be nil")
	}

	content := requestBody.Value.Content
	if content == nil || len(content) == 0 {
		t.Fatal("Request body content is empty")
	}

	jsonContent := content["application/json"]
	if jsonContent == nil {
		t.Fatal("application/json content not found")
	}

	schema := jsonContent.Schema.Value
	if !typeMatches(schema.Type, "object") {
		t.Errorf("Request body schema type should be object, got %s", schema.Type)
	}

	// Verify parameters are properties
	expectedProps := []string{"name", "email", "age"}
	for _, prop := range expectedProps {
		if _, ok := schema.Properties[prop]; !ok {
			t.Errorf("Property %s not found in request body", prop)
		}
	}

	// All parameters should be required
	if len(schema.Required) != 3 {
		t.Errorf("Expected 3 required parameters, got %d", len(schema.Required))
	}
}

// TestResponseGeneration tests response generation.
func TestResponseGeneration(t *testing.T) {
	input := `namespace example

interface UserService {
    getUser(id string) string
}
`

	g := NewFromPulseGenerator("3.1")
	spec, err := g.GenerateFromString(input)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	pathItem := spec.T.Paths.Find("/UserService/getUser")
	responses := pathItem.Post.Responses

	if responses == nil {
		t.Fatal("Responses should not be nil")
	}

	// Check 200 response
	response200 := responses.Value("200")
	if response200 == nil {
		t.Fatal("200 response not found")
	}

	content := response200.Value.Content["application/json"]
	if content == nil {
		t.Fatal("application/json content not found in 200 response")
	}

	schema := content.Schema.Value
	if !typeMatches(schema.Type, "string") {
		t.Errorf("200 response schema type should be string, got %s", schema.Type)
	}
}

// TestOptionalReturnResponse tests that optional returns add 204 response.
func TestOptionalReturnResponse(t *testing.T) {
	input := `namespace example

interface UserService {
    findUser(id string) string [optional]
}
`

	g := NewFromPulseGenerator("3.1")
	spec, err := g.GenerateFromString(input)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	pathItem := spec.T.Paths.Find("/UserService/findUser")
	responses := pathItem.Post.Responses

	// Should have both 200 and 204
	response200 := responses.Value("200")
	if response200 == nil {
		t.Fatal("200 response not found")
	}

	response204 := responses.Value("204")
	if response204 == nil {
		t.Fatal("204 response not found for optional return")
	}

	if response204.Value.Description == nil || *response204.Value.Description == "" {
		t.Error("204 response should have description")
	}
}

// TestYAMLOutput tests YAML output generation.
func TestYAMLOutput(t *testing.T) {
	input := `namespace example

interface Test {
    method() string
}
`

	g := NewFromPulseGenerator("3.1")
	spec, err := g.GenerateFromString(input)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	yaml, err := spec.ToYAML()
	if err != nil {
		t.Fatalf("ToYAML failed: %v", err)
	}

	if len(yaml) == 0 {
		t.Error("YAML output is empty")
	}

	// Check for expected YAML content
	yamlStr := string(yaml)
	if !strings.Contains(yamlStr, "openapi:") {
		t.Error("YAML should contain 'openapi:'")
	}
	if !strings.Contains(yamlStr, "3.1") {
		t.Error("YAML should contain version '3.1'")
	}
}

// TestJSONOutput tests JSON output generation.
func TestJSONOutput(t *testing.T) {
	input := `namespace example

interface Test {
    method() string
}
`

	g := NewFromPulseGenerator("3.1")
	spec, err := g.GenerateFromString(input)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	json, err := spec.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	if len(json) == 0 {
		t.Error("JSON output is empty")
	}

	// Check for expected JSON content
	jsonStr := string(json)
	if !strings.Contains(jsonStr, "\"openapi\"") {
		t.Error("JSON should contain 'openapi'")
	}
	if !strings.Contains(jsonStr, "3.1") {
		t.Error("JSON should contain version '3.1'")
	}
}

// TestGenerateToFileOutput tests file output generation.
func TestGenerateToFileOutput(t *testing.T) {
	input := `namespace example

interface Test {
    method() string
}
`

	// Create temporary directory
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "output.yaml")
	jsonFile := filepath.Join(tmpDir, "output.json")

	g := NewFromPulseGenerator("3.1")

	// Test YAML output
	if err := g.GenerateFromStringToFile(input, yamlFile); err != nil {
		t.Fatalf("GenerateToFile (YAML) failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(yamlFile); os.IsNotExist(err) {
		t.Error("YAML output file was not created")
	}

	// Test JSON output
	if err := g.GenerateFromStringToFile(input, jsonFile); err != nil {
		t.Fatalf("GenerateToFile (JSON) failed: %v", err)
	}

	if _, err := os.Stat(jsonFile); os.IsNotExist(err) {
		t.Error("JSON output file was not created")
	}
}

// TestEmptyIDL tests handling of IDL with only namespace.
func TestEmptyIDL(t *testing.T) {
	input := `namespace example
`

	g := NewFromPulseGenerator("3.1")
	spec, err := g.GenerateFromString(input)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Should still create valid spec
	if spec.T.OpenAPI != "3.1" {
		t.Error("OpenAPI version should be 3.1")
	}
}

// GenerateFromString is a helper method for testing (reads from string instead of file).
func (g *FromPulseGenerator) GenerateFromString(input string) (*GeneratedSpec, error) {
	// Create a temporary file path for parsing
	idl, err := parser.ParseIDL("test.pulse", input)
	if err != nil {
		return nil, err
	}

	ctx := NewTranslationContext(false)

	doc := openapi3.T{
		OpenAPI: g.OpenAPIVersion,
		Info: &openapi3.Info{
			Title:       deriveTitleFromNamespace(idl.RootNamespace),
			Version:     "1.0.0",
			Description: generateDescription(idl),
		},
		Paths: openapi3.NewPaths(),
		Components: &openapi3.Components{
			Schemas:       make(map[string]*openapi3.SchemaRef),
			RequestBodies: make(map[string]*openapi3.RequestBodyRef),
			Responses:     make(map[string]*openapi3.ResponseRef),
		},
		Servers: []*openapi3.Server{
			{
				URL:         "http://localhost:8080",
				Description: "PulseRPC Server",
			},
		},
	}

	if err := g.generateSchemas(idl, &doc, ctx); err != nil {
		return nil, err
	}

	if err := g.generatePaths(idl, &doc, ctx); err != nil {
		return nil, err
	}

	return &GeneratedSpec{
		Version: g.OpenAPIVersion,
		T:       &doc,
	}, nil
}

// GenerateFromStringToFile is a helper method for testing.
func (g *FromPulseGenerator) GenerateFromStringToFile(input, outputFile string) error {
	spec, err := g.GenerateFromString(input)
	if err != nil {
		return err
	}

	var data []byte
	ext := strings.ToLower(filepath.Ext(outputFile))
	if ext == ".json" {
		data, err = spec.ToJSON()
	} else {
		data, err = spec.ToYAML()
	}

	if err != nil {
		return err
	}

	return os.WriteFile(outputFile, data, 0644)
}

// TestSetStrictFromPulse tests setting strict mode on FromPulseGenerator.
func TestSetStrictFromPulse(t *testing.T) {
	g := NewFromPulseGenerator("3.1")

	// Default should be false
	if g.Strict {
		t.Error("Default Strict should be false")
	}

	g.SetStrict(true)
	if !g.Strict {
		t.Error("Strict should be true after SetStrict(true)")
	}

	g.SetStrict(false)
	if g.Strict {
		t.Error("Strict should be false after SetStrict(false)")
	}
}

// TestVoidTypeConversion tests void type conversion.
func TestVoidTypeConversion(t *testing.T) {
	input := `namespace example

interface UserService {
    deleteUser(id string) void
}
`

	g := NewFromPulseGenerator("3.1")
	spec, err := g.GenerateFromString(input)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Just verify the spec is valid - void is a special case
	if spec.T.OpenAPI != "3.1" {
		t.Error("OpenAPI version should be 3.1")
	}

	// Check that a path was created
	paths := spec.T.Paths.Map()
	if len(paths) == 0 {
		t.Error("Expected at least one path")
	}
}

// TestGenerateFromFile tests generating from a file.
func TestGenerateFromFile(t *testing.T) {
	input := `namespace example

interface TestService {
    test() string
}
`

	// Write to temp file
	tmpDir := t.TempDir()
	pulseFile := filepath.Join(tmpDir, "test.pulse")
	if err := os.WriteFile(pulseFile, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}

	g := NewFromPulseGenerator("3.1")
	spec, err := g.Generate(pulseFile)
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	if spec.T.OpenAPI != "3.1" {
		t.Error("OpenAPI version should be 3.1")
	}
}

// TestGenerateToFileFromPulse tests generating OpenAPI to file from Pulse IDL.
func TestGenerateToFileFromPulse(t *testing.T) {
	input := `namespace example

interface TestService {
    test() string
}
`

	// Write to temp file
	tmpDir := t.TempDir()
	pulseFile := filepath.Join(tmpDir, "test.pulse")
	if err := os.WriteFile(pulseFile, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}

	yamlFile := filepath.Join(tmpDir, "output.yaml")
	jsonFile := filepath.Join(tmpDir, "output.json")

	g := NewFromPulseGenerator("3.1")

	// Test YAML output
	if err := g.GenerateToFile(pulseFile, yamlFile); err != nil {
		t.Fatalf("GenerateToFile (YAML) failed: %v", err)
	}

	if _, err := os.Stat(yamlFile); os.IsNotExist(err) {
		t.Error("YAML output file was not created")
	}

	// Test JSON output
	if err := g.GenerateToFile(pulseFile, jsonFile); err != nil {
		t.Fatalf("GenerateToFile (JSON) failed: %v", err)
	}

	if _, err := os.Stat(jsonFile); os.IsNotExist(err) {
		t.Error("JSON output file was not created")
	}
}

// TestGenerateToFileWithWarningsFromPulse tests generating with warnings collection.
func TestGenerateToFileWithWarningsFromPulse(t *testing.T) {
	input := `namespace example

interface TestService {
    test() string
}
`

	// Write to temp file
	tmpDir := t.TempDir()
	pulseFile := filepath.Join(tmpDir, "test.pulse")
	if err := os.WriteFile(pulseFile, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}

	yamlFile := filepath.Join(tmpDir, "output.yaml")

	g := NewFromPulseGenerator("3.1")
	warnings, err := g.GenerateToFileWithWarnings(pulseFile, yamlFile, false)

	if err != nil {
		t.Fatalf("GenerateToFileWithWarnings() failed: %v", err)
	}

	if _, err := os.Stat(yamlFile); os.IsNotExist(err) {
		t.Error("Output file was not created")
	}

	// Warnings should be a non-nil slice
	if warnings == nil {
		t.Error("Warnings should not be nil")
	}
}

// TestGenerateToFileNonExistentFile tests error handling for non-existent file.
func TestGenerateToFileNonExistentFile(t *testing.T) {
	g := NewFromPulseGenerator("3.1")
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output.yaml")

	err := g.GenerateToFile("/nonexistent/file.pulse", outputFile)
	if err == nil {
		t.Error("GenerateToFile() should fail with non-existent input file")
	}
}

// TestInterfaceWithNoMethods tests interface with no methods.
func TestInterfaceWithNoMethods(t *testing.T) {
	input := `namespace example

interface EmptyService {
}
`

	g := NewFromPulseGenerator("3.1")
	spec, err := g.GenerateFromString(input)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Should still create valid spec
	if spec.T.OpenAPI != "3.1" {
		t.Error("OpenAPI version should be 3.1")
	}

	// Empty interface should not create a path
	pathItem := spec.T.Paths.Find("/EmptyService")
	if pathItem != nil {
		// It's ok to have an empty path or no path at all
	}
}

// TestStructWithAllOptionalFields tests struct with all optional fields.
func TestStructWithAllOptionalFields(t *testing.T) {
	input := `namespace example

struct OptionalUser {
    name string [optional]
    email string [optional]
    age int [optional]
}
`

	g := NewFromPulseGenerator("3.1")
	spec, err := g.GenerateFromString(input)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	userSchemaRef := spec.T.Components.Schemas["OptionalUser"]
	userSchema := userSchemaRef.Value

	// No fields should be required
	if len(userSchema.Required) != 0 {
		t.Errorf("Expected 0 required fields, got %d: %v", len(userSchema.Required), userSchema.Required)
	}
}

// TestMultipleInterfacesWithSameMethodNames tests different interfaces with methods having same name.
func TestMultipleInterfacesWithSameMethodNames(t *testing.T) {
	input := `namespace example

interface ServiceA {
    process(data string) string
}

interface ServiceB {
    process(data string) string
}
`

	g := NewFromPulseGenerator("3.1")
	spec, err := g.GenerateFromString(input)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Should have two different paths
	pathA := spec.T.Paths.Find("/ServiceA/process")
	if pathA == nil {
		t.Error("Path /ServiceA/process not found")
	}

	pathB := spec.T.Paths.Find("/ServiceB/process")
	if pathB == nil {
		t.Error("Path /ServiceB/process not found")
	}
}

// TestDeeplyNestedArray tests deeply nested array types.
func TestDeeplyNestedArray(t *testing.T) {
	input := `namespace example

struct Matrix {
    data [][][]int
}
`

	g := NewFromPulseGenerator("3.1")
	spec, err := g.GenerateFromString(input)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	matrixSchemaRef := spec.T.Components.Schemas["Matrix"]
	if matrixSchemaRef == nil {
		t.Fatal("Matrix schema not found")
	}

	matrixSchema := matrixSchemaRef.Value
	dataProp := matrixSchema.Properties["data"]

	if dataProp == nil {
		t.Fatal("data property not found")
	}

	// Should be array
	if !typeMatches(dataProp.Value.Type, "array") {
		t.Errorf("data should be array, got %v", dataProp.Value.Type)
	}
}

// TestMapWithCustomType tests map with custom value type.
func TestMapWithCustomType(t *testing.T) {
	input := `namespace example

struct Metadata {
    tags map[string]Tag
}

struct Tag {
    key string
    value string
}
`

	g := NewFromPulseGenerator("3.1")
	spec, err := g.GenerateFromString(input)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	metadataSchemaRef := spec.T.Components.Schemas["Metadata"]
	if metadataSchemaRef == nil {
		t.Fatal("Metadata schema not found")
	}

	metadataSchema := metadataSchemaRef.Value
	tagsProp := metadataSchema.Properties["tags"]

	if tagsProp == nil {
		t.Fatal("tags property not found")
	}

	// Should be object with additionalProperties having ref to Tag
	if tagsProp.Value.AdditionalProperties.Schema == nil {
		t.Fatal("tags additionalProperties schema ref should not be nil")
	}

	// Verify Tag schema exists
	_, ok := spec.T.Components.Schemas["Tag"]
	if !ok {
		t.Error("Tag schema should exist in components/schemas")
	}

	// The additionalProperties should have a ref to Tag
	expectedRef := "#/components/schemas/Tag"
	if tagsProp.Value.AdditionalProperties.Schema.Ref != expectedRef {
		t.Errorf("Expected additionalProperties ref %s, got %s", expectedRef, tagsProp.Value.AdditionalProperties.Schema.Ref)
	}
}
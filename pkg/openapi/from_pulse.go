package openapi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/coopernurse/pulserpc/pkg/parser"
	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

// Helper function to create a Type pointer from a string
func typePtr(t string) *openapi3.Types {
	result := openapi3.Types{t}
	return &result
}

// Helper function to create a string pointer (not currently used but kept for potential future use)
func stringPtr(s string) *string {
	return &s
}

// Helper function to create a SchemaRef from a parser.Type
func (g *FromPulseGenerator) createSchemaRef(t *parser.Type, doc *openapi3.T, ctx *TranslationContext) *openapi3.SchemaRef {
	schema := g.convertTypeToSchema(t, doc, ctx)

	// For user-defined types, create a reference instead
	if t != nil && t.IsUserDefined() {
		return openapi3.NewSchemaRef("#/components/schemas/"+t.UserDefined, nil)
	}

	return openapi3.NewSchemaRef("", schema)
}

// FromPulseGenerator generates OpenAPI specs from Pulse IDL.
type FromPulseGenerator struct {
	// OpenAPIVersion specifies the target OpenAPI version (3.0 or 3.1)
	OpenAPIVersion string
	// ctx is the translation context for collecting warnings
	ctx *TranslationContext
	// Strict mode treats warnings as errors
	Strict bool
}

// NewFromPulseGenerator creates a new Pulse → OpenAPI generator.
func NewFromPulseGenerator(version string) *FromPulseGenerator {
	if version == "" {
		version = "3.1" // Default to OpenAPI 3.1
	}
	return &FromPulseGenerator{
		OpenAPIVersion: version,
		ctx:            NewTranslationContext(false),
		Strict:         false,
	}
}

// SetStrict sets the strict mode flag.
func (g *FromPulseGenerator) SetStrict(strict bool) {
	g.Strict = strict
	g.ctx.Strict = strict
}

// GeneratedSpec represents a generated OpenAPI specification.
type GeneratedSpec struct {
	// Version is the OpenAPI version (3.0 or 3.1)
	Version string
	// T is the OpenAPI document
	T *openapi3.T
}

// ToJSON serializes the spec to JSON format.
func (s *GeneratedSpec) ToJSON() ([]byte, error) {
	return json.MarshalIndent(s.T, "", "  ")
}

// ToYAML serializes the spec to YAML format (preferred).
func (s *GeneratedSpec) ToYAML() ([]byte, error) {
	return yaml.Marshal(s.T)
}

// Generate reads a Pulse IDL file and generates an OpenAPI specification.
func (g *FromPulseGenerator) Generate(pulseFile string) (*GeneratedSpec, error) {
	// Read the Pulse IDL file
	content, err := os.ReadFile(pulseFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read Pulse file %s: %w", pulseFile, err)
	}

	// Parse the Pulse IDL
	idl, err := parser.ParseIDL(pulseFile, string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse Pulse IDL: %w", err)
	}

	// Reset context for this generation
	g.ctx = NewTranslationContext(g.Strict)

	// Create OpenAPI document
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

	// Check for empty interfaces
	if len(idl.Interfaces) == 0 {
		g.ctx.Warnings.AddWarning("interfaces", "no interfaces defined in Pulse IDL; generating schemas only")
	}

	// Generate schemas from structs and enums
	if err := g.generateSchemas(idl, &doc, g.ctx); err != nil {
		return nil, fmt.Errorf("failed to generate schemas: %w", err)
	}

	// Generate paths from interfaces and methods
	if err := g.generatePaths(idl, &doc, g.ctx); err != nil {
		return nil, fmt.Errorf("failed to generate paths: %w", err)
	}

	return &GeneratedSpec{
		Version: g.OpenAPIVersion,
		T:       &doc,
	}, nil
}

// GenerateToFile reads a Pulse IDL file and writes OpenAPI spec to a file.
func (g *FromPulseGenerator) GenerateToFile(pulseFile, outputFile string) error {
	spec, err := g.Generate(pulseFile)
	if err != nil {
		return err
	}

	// Determine output format from file extension
	var data []byte
	ext := strings.ToLower(filepath.Ext(outputFile))
	if ext == ".json" {
		data, err = spec.ToJSON()
	} else {
		// Default to YAML
		data, err = spec.ToYAML()
	}

	if err != nil {
		return fmt.Errorf("failed to serialize OpenAPI spec: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write output file %s: %w", outputFile, err)
	}

	return nil
}

// GenerateToFileWithWarnings reads a Pulse IDL file, writes OpenAPI spec to a file, and returns warnings.
func (g *FromPulseGenerator) GenerateToFileWithWarnings(pulseFile, outputFile string, strict bool) ([]Warning, error) {
	g.Strict = strict
	spec, err := g.Generate(pulseFile)
	if err != nil {
		return nil, err
	}

	// Determine output format from file extension
	var data []byte
	ext := strings.ToLower(filepath.Ext(outputFile))
	if ext == ".json" {
		data, err = spec.ToJSON()
	} else {
		// Default to YAML
		data, err = spec.ToYAML()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to serialize OpenAPI spec: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write output file %s: %w", outputFile, err)
	}

	// Return warnings
	return g.ctx.Warnings.All(), nil
}

// generateSchemas generates OpenAPI schemas from Pulse structs and enums.
func (g *FromPulseGenerator) generateSchemas(idl *parser.IDL, doc *openapi3.T, ctx *TranslationContext) error {
	// Generate schemas from all structs
	for _, s := range idl.Structs {
		if err := g.generateStructSchema(s, doc, ctx); err != nil {
			return fmt.Errorf("failed to generate schema for struct %s: %w", s.Name, err)
		}
	}

	// Generate schemas from all enums
	for _, e := range idl.Enums {
		if err := g.generateEnumSchema(e, doc, ctx); err != nil {
			return fmt.Errorf("failed to generate schema for enum %s: %w", e.Name, err)
		}
	}

	return nil
}

// generateStructSchema generates an OpenAPI schema from a Pulse struct.
func (g *FromPulseGenerator) generateStructSchema(s *parser.Struct, doc *openapi3.T, ctx *TranslationContext) error {
	schema := &openapi3.Schema{
		Type:        typePtr("object"),
		Description: s.Comment,
		Properties:  make(map[string]*openapi3.SchemaRef),
	}

	// Handle struct inheritance via allOf
	if s.Extends != "" {
		schema.AllOf = []*openapi3.SchemaRef{
			openapi3.NewSchemaRef("#/components/schemas/"+s.Extends, nil),
		}
	}

	// Track required fields
	required := make([]string, 0)

	// Process fields
	for _, f := range s.Fields {
		propSchemaRef := g.createSchemaRef(f.Type, doc, ctx)

		// Add field description from comment
		if propSchemaRef.Value != nil && f.Comment != "" {
			propSchemaRef.Value.Description = f.Comment
		}

		schema.Properties[f.Name] = propSchemaRef

		// Mark as required if not optional
		if !f.Optional {
			required = append(required, f.Name)
		}
	}

	schema.Required = required

	// Add to components/schemas
	doc.Components.Schemas[s.Name] = &openapi3.SchemaRef{
		Value: schema,
	}

	return nil
}

// generateEnumSchema generates an OpenAPI schema from a Pulse enum.
func (g *FromPulseGenerator) generateEnumSchema(e *parser.Enum, doc *openapi3.T, ctx *TranslationContext) error {
	enumValues := make([]interface{}, len(e.Values))
	for i, v := range e.Values {
		enumValues[i] = v.Name
	}

	schema := &openapi3.Schema{
		Type:        typePtr("string"),
		Description: e.Comment,
		Enum:        enumValues,
	}

	// Add to components/schemas
	doc.Components.Schemas[e.Name] = &openapi3.SchemaRef{
		Value: schema,
	}

	return nil
}

// generatePaths generates OpenAPI paths from Pulse interfaces and methods.
func (g *FromPulseGenerator) generatePaths(idl *parser.IDL, doc *openapi3.T, ctx *TranslationContext) error {
	// Process each interface
	for _, iface := range idl.Interfaces {
		// Each interface becomes a tag
		tag := iface.Name

		// Process each method in the interface
		for _, method := range iface.Methods {
			// Generate path: POST /{interface}/{method}
			path := fmt.Sprintf("/%s/%s", iface.Name, method.Name)

			// Warn about very long paths (>100 characters)
			if len(path) > 100 {
				ctx.Warnings.AddWarning(path, fmt.Sprintf("path is very long (%d characters); some OpenAPI tools may have issues", len(path)))
			}

			// Check for reserved parameter names
			for _, param := range method.Parameters {
				if IsReservedOpenAPIWord(param.Name) {
					ctx.Warnings.AddWarning(param.Name, fmt.Sprintf("parameter name '%s' conflicts with OpenAPI reserved word; may cause issues", param.Name))
				}
			}

			// Create operation
			operation := &openapi3.Operation{
				OperationID: fmt.Sprintf("%s_%s", iface.Name, method.Name),
				Tags:        []string{tag},
				Description: methodComment(method),
				RequestBody: g.generateRequestBody(method, doc, ctx),
				Responses:   g.generateResponses(method, doc, ctx),
			}

			// Add or update path
			pathItem := doc.Paths.Find(path)
			if pathItem == nil {
				pathItem = &openapi3.PathItem{}
			}
			pathItem.Post = operation

			doc.Paths.Set(path, pathItem)
		}
	}

	return nil
}

// generateRequestBody generates the request body for a method.
func (g *FromPulseGenerator) generateRequestBody(method *parser.Method, doc *openapi3.T, ctx *TranslationContext) *openapi3.RequestBodyRef {
	if len(method.Parameters) == 0 {
		return nil
	}

	// Create schema wrapping all parameters
	properties := make(map[string]*openapi3.SchemaRef)
	required := make([]string, 0)

	for _, param := range method.Parameters {
		properties[param.Name] = g.createSchemaRef(param.Type, doc, ctx)
		required = append(required, param.Name)
	}

	bodySchema := &openapi3.Schema{
		Type:       typePtr("object"),
		Properties: properties,
		Required:   required,
	}

	return &openapi3.RequestBodyRef{
		Value: &openapi3.RequestBody{
			Description: "Request body for " + method.Name,
			Required:    true,
			Content: map[string]*openapi3.MediaType{
				"application/json": {
					Schema: openapi3.NewSchemaRef("", bodySchema),
				},
			},
		},
	}
}

// generateResponses generates the responses for a method.
func (g *FromPulseGenerator) generateResponses(method *parser.Method, doc *openapi3.T, ctx *TranslationContext) *openapi3.Responses {
	responses := openapi3.NewResponses()

	// Generate 200 response with return type
	responseSchemaRef := g.createSchemaRef(method.ReturnType, doc, ctx)

	response200 := &openapi3.Response{
		Description: stringPtr("Successful response"),
		Content: map[string]*openapi3.MediaType{
			"application/json": {
				Schema: responseSchemaRef,
			},
		},
	}

	responses.Set("200", &openapi3.ResponseRef{
		Value: response200,
	})

	// If return type is optional, add 204 No Content response
	if method.ReturnOptional {
		response204 := &openapi3.Response{
			Description: stringPtr("No Content (optional value not present)"),
		}
		responses.Set("204", &openapi3.ResponseRef{
			Value: response204,
		})
	}

	return responses
}

// convertTypeToSchema converts a Pulse type to an OpenAPI schema.
// Note: For user-defined types, use createSchemaRef instead to get proper references.
func (g *FromPulseGenerator) convertTypeToSchema(t *parser.Type, doc *openapi3.T, ctx *TranslationContext) *openapi3.Schema {
	if t == nil {
		return &openapi3.Schema{
			Type: typePtr("string"),
		}
	}

	switch {
	case t.IsBuiltIn():
		// Map built-in types
		switch t.BuiltIn {
		case "string":
			return &openapi3.Schema{Type: typePtr("string")}
		case "int":
			return &openapi3.Schema{Type: typePtr("integer"), Format: "int64"}
		case "float":
			return &openapi3.Schema{Type: typePtr("number"), Format: "double"}
		case "bool":
			return &openapi3.Schema{Type: typePtr("boolean")}
		default:
			return &openapi3.Schema{Type: typePtr("string")}
		}

	case t.IsArray():
		// Array type: []Type -> OpenAPI array with items
		// For user-defined element types, use a $ref; for built-in types, inline the schema
		if t.Array != nil && t.Array.IsUserDefined() {
			return &openapi3.Schema{
				Type:  typePtr("array"),
				Items: openapi3.NewSchemaRef("#/components/schemas/"+t.Array.UserDefined, nil),
			}
		}
		itemSchema := g.convertTypeToSchema(t.Array, doc, ctx)
		return &openapi3.Schema{
			Type:  typePtr("array"),
			Items: openapi3.NewSchemaRef("", itemSchema),
		}

	case t.IsMap():
		// Map type: map[string]Type -> OpenAPI object with additionalProperties
		// For user-defined value types, use a $ref; for built-in types, inline the schema
		has := true
		if t.MapValue != nil && t.MapValue.IsUserDefined() {
			return &openapi3.Schema{
				Type: typePtr("object"),
				AdditionalProperties: openapi3.AdditionalProperties{
					Has:    &has,
					Schema: openapi3.NewSchemaRef("#/components/schemas/"+t.MapValue.UserDefined, nil),
				},
			}
		}
		valueSchema := g.convertTypeToSchema(t.MapValue, doc, ctx)
		return &openapi3.Schema{
			Type: typePtr("object"),
			AdditionalProperties: openapi3.AdditionalProperties{
				Has:    &has,
				Schema: openapi3.NewSchemaRef("", valueSchema),
			},
		}

	case t.IsUserDefined():
		// User-defined type should use createSchemaRef instead
		// Return a placeholder for now
		return &openapi3.Schema{Type: typePtr("object")}

	default:
		return &openapi3.Schema{Type: typePtr("string")}
	}
}

// deriveTitleFromNamespace derives an OpenAPI title from the namespace.
func deriveTitleFromNamespace(namespace string) string {
	if namespace == "" {
		return "PulseRPC API"
	}
	// Convert namespace to title case
	title := strings.ReplaceAll(namespace, "_", " ")
	title = strings.ReplaceAll(title, ".", " ")
	return strings.Title(title)
}

// generateDescription generates an overall API description from the IDL.
func generateDescription(idl *parser.IDL) string {
	var parts []string

	if idl.RootNamespace != "" {
		parts = append(parts, fmt.Sprintf("Namespace: %s", idl.RootNamespace))
	}

	if len(idl.Interfaces) > 0 {
		parts = append(parts, fmt.Sprintf("\nInterfaces: %d", len(idl.Interfaces)))
	}
	if len(idl.Structs) > 0 {
		parts = append(parts, fmt.Sprintf("Structs: %d", len(idl.Structs)))
	}
	if len(idl.Enums) > 0 {
		parts = append(parts, fmt.Sprintf("Enums: %d", len(idl.Enums)))
	}

	return strings.Join(parts, "\n")
}

// methodComment extracts or generates a method description.
func methodComment(method *parser.Method) string {
	// Future enhancement: extract from IDL comments
	return fmt.Sprintf("Method: %s", method.Name)
}

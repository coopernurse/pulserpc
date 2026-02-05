package openapi

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/coopernurse/pulserpc/pkg/parser"
)

// ToPulse converts OpenAPI specifications to Pulse IDL.
// Phase 3: Full implementation

// ToPulseGenerator generates Pulse IDL from parsed OpenAPI specifications.
type ToPulseGenerator struct {
	// Parser is used to load OpenAPI specs
	Parser *Parser
	// Strict mode treats warnings as errors
	Strict bool
}

// NewToPulseGenerator creates a new OpenAPI → Pulse generator.
func NewToPulseGenerator() *ToPulseGenerator {
	return &ToPulseGenerator{
		Parser: NewParser(),
		Strict: false,
	}
}

// SetStrict sets the strict mode flag.
func (g *ToPulseGenerator) SetStrict(strict bool) {
	g.Strict = strict
}

// Generate reads an OpenAPI spec file and generates Pulse IDL.
func (g *ToPulseGenerator) Generate(openapiFile string) (*parser.IDL, error) {
	// Parse the OpenAPI spec
	spec, err := g.Parser.ParseFile(openapiFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}

	// Check for errors in strict mode
	if g.Strict && spec.HasErrors() {
		return nil, fmt.Errorf("OpenAPI spec has errors (strict mode enabled)")
	}

	// Derive namespace from info.title
	namespace := deriveNamespace(spec.Info.Title)

	// Create the IDL structure
	idl := &parser.IDL{
		RootNamespace: namespace,
		Interfaces:    make([]*parser.Interface, 0),
		Structs:       make([]*parser.Struct, 0),
		Enums:         make([]*parser.Enum, 0),
	}

	// Generate structs from schemas
	for _, schema := range spec.Schemas {
		if schema.IsEnum {
			// Generate enum
			enum := g.generateEnum(schema, namespace)
			if enum != nil {
				idl.Enums = append(idl.Enums, enum)
			}
		} else if schema.IsObject || len(schema.AllOf) > 0 {
			// Generate struct
			structDef := g.generateStruct(schema, namespace, spec.Schemas)
			if structDef != nil {
				idl.Structs = append(idl.Structs, structDef)
			}
		}
	}

	// Generate interfaces and methods from paths/operations
	interfaces := g.generateInterfaces(spec, namespace)
	idl.Interfaces = append(idl.Interfaces, interfaces...)

	return idl, nil
}

// GenerateToFile reads an OpenAPI spec file and writes Pulse IDL to a file.
func (g *ToPulseGenerator) GenerateToFile(openapiFile, outputFile string) error {
	// Generate the IDL
	idl, err := g.Generate(openapiFile)
	if err != nil {
		return err
	}

	// Generate the Pulse IDL content
	content := g.GeneratePulseContent(idl, openapiFile)

	// Write to file
	if err := writePulseFile(outputFile, content); err != nil {
		return fmt.Errorf("failed to write Pulse IDL file: %w", err)
	}

	return nil
}

// deriveNamespace converts an OpenAPI title to a valid Pulse namespace.
// Uses lowercase and replaces non-alphanumeric characters with underscores.
func deriveNamespace(title string) string {
	if title == "" {
		return "api"
	}
	return ToValidIdentifier(title)
}

// generateEnum creates a Pulse enum from an OpenAPI schema.
func (g *ToPulseGenerator) generateEnum(schema *SchemaInfo, namespace string) *parser.Enum {
	if !schema.IsEnum || len(schema.Enum) == 0 {
		return nil
	}

	// Convert enum values to EnumValue structs
	enumValues := make([]*parser.EnumValue, 0, len(schema.Enum))
	for _, value := range schema.Enum {
		enumValues = append(enumValues, &parser.EnumValue{
			Name:    value,
			Comment: "",
		})
	}

	return &parser.Enum{
		Name:      schema.Name,
		Namespace: namespace,
		Comment:   FormatComment(schema.Description),
		Values:    enumValues,
	}
}

// generateStruct creates a Pulse struct from an OpenAPI schema.
func (g *ToPulseGenerator) generateStruct(schema *SchemaInfo, namespace string, allSchemas map[string]*SchemaInfo) *parser.Struct {
	if schema == nil {
		return nil
	}

	// Handle allOf composition
	var extends string
	if len(schema.AllOf) > 0 {
		// Find the first reference in allOf - this becomes the extends clause
		for _, allOfSchema := range schema.AllOf {
			if allOfSchema.RefName != "" {
				extends = allOfSchema.RefName
				break
			}
			// Check if it's a named schema
			if allOfSchema.IsObject && allOfSchema.Name != "" && allOfSchema.Name != schema.Name {
				extends = allOfSchema.Name
				break
			}
		}
	}

	// Generate fields from properties
	fields := make([]*parser.Field, 0, len(schema.Properties))
	for propName, propSchema := range schema.Properties {
		if propSchema == nil {
			continue
		}

		// Check if field is required
		isRequired := schema.Required[propName]

		// Convert the type
		fieldType := convertSchemaInfoToType(propSchema)

		fields = append(fields, &parser.Field{
			Name:     propName,
			Type:     fieldType,
			Optional: !isRequired,
			Comment:  FormatComment(propSchema.Description),
		})
	}

	return &parser.Struct{
		Name:      schema.Name,
		Namespace: namespace,
		Extends:   extends,
		Comment:   FormatComment(schema.Description),
		Fields:    fields,
	}
}

// convertSchemaInfoToType converts a SchemaInfo to a parser.Type.
func convertSchemaInfoToType(schema *SchemaInfo) *parser.Type {
	if schema == nil {
		return &parser.Type{BuiltIn: "string"}
	}

	// Handle array types
	if schema.IsArray && schema.Items != nil {
		return &parser.Type{
			Array: convertSchemaInfoToType(schema.Items),
		}
	}

	// Handle map types
	if schema.IsMap && schema.AdditionalProperties != nil {
		return &parser.Type{
			MapValue: convertSchemaInfoToType(schema.AdditionalProperties),
		}
	}

	// Handle user-defined types (structs, enums)
	if schema.IsObject || schema.IsEnum {
		return &parser.Type{
			UserDefined: schema.Name,
		}
	}

	// Handle primitive types
	if schema.Type.Name != "" {
		if schema.Type.IsPrimitive {
			return &parser.Type{BuiltIn: schema.Type.Name}
		}
		return &parser.Type{UserDefined: schema.Type.Name}
	}

	// Default to string
	return &parser.Type{BuiltIn: "string"}
}

// generateInterfaces creates Pulse interfaces from OpenAPI paths and operations.
func (g *ToPulseGenerator) generateInterfaces(spec *ParsedSpec, namespace string) []*parser.Interface {
	// Group operations by tag (or path prefix if no tags)
	operationGroups := make(map[string][]*Operation)

	for _, pathItem := range spec.Paths {
		for _, op := range pathItem.Operations {
			if op == nil {
				continue
			}

			// Use the first tag as the interface name, or use path prefix
			interfaceName := op.Tag
			if interfaceName == "" {
				// Derive interface name from path
				interfaceName = deriveInterfaceNameFromPath(op.Path)
			}

			// Sanitize interface name to be a valid identifier
			interfaceName = ToValidIdentifier(interfaceName)
			if interfaceName == "" {
				interfaceName = "default"
			}

			operationGroups[interfaceName] = append(operationGroups[interfaceName], op)
		}
	}

	// Generate interfaces from grouped operations
	interfaces := make([]*parser.Interface, 0, len(operationGroups))
	for interfaceName, operations := range operationGroups {
		iface := &parser.Interface{
			Name:      interfaceName,
			Namespace: namespace,
			Comment:   "",
			Methods:   make([]*parser.Method, 0),
		}

		// Generate methods from operations
		for _, op := range operations {
			method := g.generateMethod(op)
			if method != nil {
				iface.Methods = append(iface.Methods, method)
			}
		}

		interfaces = append(interfaces, iface)
	}

	return interfaces
}

// deriveInterfaceNameFromPath derives an interface name from a path.
func deriveInterfaceNameFromPath(path string) string {
	// Remove leading slash and get the first segment
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	return "default"
}

// generateMethod creates a Pulse method from an OpenAPI operation.
func (g *ToPulseGenerator) generateMethod(op *Operation) *parser.Method {
	if op == nil {
		return nil
	}

	// Generate method name: {httpMethod}{operationId} or {httpMethod}{pathToMethodName}
	methodName := generateMethodName(op)

	// Collect parameters
	parameters := make([]*parser.Parameter, 0)

	// Add path and query parameters
	for _, param := range op.Parameters {
		if param == nil || param.Schema == nil {
			continue
		}

		// Only include path and query parameters
		if param.In != "path" && param.In != "query" {
			continue
		}

		paramType := convertSchemaInfoToType(param.Schema)
		parameters = append(parameters, &parser.Parameter{
			Name: param.Name,
			Type: paramType,
		})
	}

	// Add request body as parameter
	if op.RequestBody != nil {
		bodyName := "body"
		if op.RequestBody.Name != "" {
			bodyName = op.RequestBody.Name
		}
		bodyType := convertSchemaInfoToType(op.RequestBody)
		parameters = append(parameters, &parser.Parameter{
			Name: bodyName,
			Type: bodyType,
		})
	}

	// Determine return type from response
	var returnType *parser.Type
	var returnOptional bool

	// Look for 200 or 201 response
	for _, code := range []string{"200", "201", "default"} {
		if resp, ok := op.Responses[code]; ok && resp != nil && resp.Schema != nil {
			returnType = convertSchemaInfoToType(resp.Schema)
			break
		}
	}

	// Check for 204 No Content (empty response)
	if _, has204 := op.Responses["204"]; has204 {
		returnOptional = true
	}

	// Default to void if no response schema
	if returnType == nil {
		returnType = &parser.Type{BuiltIn: "void"}
	}

	// Build method comment from summary and description
	comment := op.Summary
	if op.Description != "" {
		if comment != "" {
			comment += "\n" + op.Description
		} else {
			comment = op.Description
		}
	}

	return &parser.Method{
		Name:           methodName,
		Parameters:     parameters,
		ReturnType:     returnType,
		ReturnOptional: returnOptional,
	}
}

// generateMethodName generates a method name from an operation.
// Format: {httpMethod}{operationId} or {httpMethod}{pathToMethodName}
func generateMethodName(op *Operation) string {
	if op == nil {
		return "unknown"
	}

	// Use operationId if available
	if op.ID != "" {
		// Prefix with HTTP method
		return fmt.Sprintf("%s%s", strings.ToLower(op.Method), strings.Title(op.ID))
	}

	// Otherwise, derive from path
	pathName := PathToMethodName(op.Path)
	return fmt.Sprintf("%s%s", strings.ToLower(op.Method), pathName)
}

// GeneratePulseContent generates the Pulse IDL file content from an IDL structure.
func (g *ToPulseGenerator) GeneratePulseContent(idl *parser.IDL, sourceFile string) string {
	var sb strings.Builder

	// Add header comment with generation info
	sb.WriteString("// Generated from OpenAPI spec: ")
	sb.WriteString(sourceFile)
	sb.WriteString("\n// Generated at: ")
	sb.WriteString(time.Now().Format(time.RFC3339))
	sb.WriteString("\n// DO NOT EDIT - This file is auto-generated\n\n")

	// Add namespace declaration
	sb.WriteString("namespace ")
	sb.WriteString(idl.RootNamespace)
	sb.WriteString("\n\n")

	// Add structs
	for _, structDef := range idl.Structs {
		sb.WriteString(g.formatStruct(structDef))
		sb.WriteString("\n")
	}

	// Add enums
	for _, enumDef := range idl.Enums {
		sb.WriteString(g.formatEnum(enumDef))
		sb.WriteString("\n")
	}

	// Add interfaces
	for _, iface := range idl.Interfaces {
		sb.WriteString(g.formatInterface(iface))
		sb.WriteString("\n")
	}

	return sb.String()
}

// formatStruct formats a struct definition as Pulse IDL.
func (g *ToPulseGenerator) formatStruct(structDef *parser.Struct) string {
	var sb strings.Builder

	// Add comment if present
	if structDef.Comment != "" {
		sb.WriteString(formatCommentAsPulse(structDef.Comment))
		sb.WriteString("\n")
	}

	sb.WriteString("struct ")
	sb.WriteString(structDef.Name)

	// Add extends clause if present
	if structDef.Extends != "" {
		sb.WriteString(" extends ")
		sb.WriteString(structDef.Extends)
	}

	sb.WriteString(" {\n")

	// Add fields
	for _, field := range structDef.Fields {
		if field.Comment != "" {
			comment := formatCommentAsPulse(field.Comment)
			sb.WriteString("    ")
			sb.WriteString(comment)
			sb.WriteString("\n")
		}
		sb.WriteString("    ")
		sb.WriteString(field.Name)
		sb.WriteString("    ")
		sb.WriteString(formatType(field.Type))
		if field.Optional {
			sb.WriteString("    [optional]")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("}\n")
	return sb.String()
}

// formatEnum formats an enum definition as Pulse IDL.
func (g *ToPulseGenerator) formatEnum(enumDef *parser.Enum) string {
	var sb strings.Builder

	// Add comment if present
	if enumDef.Comment != "" {
		sb.WriteString(formatCommentAsPulse(enumDef.Comment))
		sb.WriteString("\n")
	}

	sb.WriteString("enum ")
	sb.WriteString(enumDef.Name)
	sb.WriteString(" {\n")

	// Add values
	for _, value := range enumDef.Values {
		sb.WriteString("    ")
		sb.WriteString(value.Name)
		sb.WriteString("\n")
	}

	sb.WriteString("}\n")
	return sb.String()
}

// formatInterface formats an interface definition as Pulse IDL.
func (g *ToPulseGenerator) formatInterface(iface *parser.Interface) string {
	var sb strings.Builder

	// Add comment if present
	if iface.Comment != "" {
		sb.WriteString(formatCommentAsPulse(iface.Comment))
		sb.WriteString("\n")
	}

	sb.WriteString("interface ")
	sb.WriteString(iface.Name)
	sb.WriteString(" {\n")

	// Add methods
	for _, method := range iface.Methods {
		// Method comments are handled in formatMethod
		sb.WriteString(g.formatMethod(method))
		sb.WriteString("\n")
	}

	sb.WriteString("}\n")
	return sb.String()
}

// formatMethod formats a method definition as Pulse IDL.
func (g *ToPulseGenerator) formatMethod(method *parser.Method) string {
	var sb strings.Builder

	sb.WriteString("  ")

	// Add parameters
	sb.WriteString(method.Name)
	sb.WriteString("(")
	for i, param := range method.Parameters {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(param.Name)
		sb.WriteString(" ")
		sb.WriteString(formatType(param.Type))
	}
	sb.WriteString(") ")

	// Add return type
	sb.WriteString(formatType(method.ReturnType))

	// Add optional return flag
	if method.ReturnOptional {
		sb.WriteString(" [optional]")
	}

	return sb.String()
}

// formatType formats a type as Pulse IDL string.
func formatType(t *parser.Type) string {
	if t == nil {
		return "void"
	}

	if t.IsBuiltIn() {
		return t.BuiltIn
	}

	if t.IsArray() {
		return "[]" + formatType(t.Array)
	}

	if t.IsMap() {
		return "map[string]" + formatType(t.MapValue)
	}

	if t.IsUserDefined() {
		return t.UserDefined
	}

	return "unknown"
}

// formatCommentAsPulse formats a comment string as a Pulse comment.
func formatCommentAsPulse(comment string) string {
	if comment == "" {
		return ""
	}

	// Format as multi-line comment
	lines := strings.Split(comment, "\n")
	var sb strings.Builder
	for _, line := range lines {
		sb.WriteString("// ")
		sb.WriteString(strings.TrimSpace(line))
		sb.WriteString("\n")
	}
	// Remove trailing newline
	result := sb.String()
	return strings.TrimSuffix(result, "\n")
}

// writePulseFile writes the Pulse IDL content to a file.
func writePulseFile(filename string, content string) error {
	// Create parent directories if needed
	dir := filename[:strings.LastIndex(filename, "/")]
	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Write to file
	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", filename, err)
	}

	return nil
}

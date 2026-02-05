package openapi

import (
	"errors"
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// Parser handles loading and validating OpenAPI 3.x specifications.
// Phase 2: Full implementation

// Parser represents an OpenAPI spec parser.
type Parser struct {
	// loader is used for loading OpenAPI specs
	loader *openapi3.Loader
}

// NewParser creates a new OpenAPI parser.
func NewParser() *Parser {
	return &Parser{
		loader: openapi3.NewLoader(),
	}
}

// ParseFile loads and validates an OpenAPI 3.0/3.1 YAML or JSON file.
// It resolves $ref references (local and external) and extracts
// components/schemas into a normalized type map.
func (p *Parser) ParseFile(filename string) (*ParsedSpec, error) {
	// Load the OpenAPI spec from file
	doc, err := p.loader.LoadFromFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to load OpenAPI spec: %w", err)
	}

	// Validate the spec
	if err := doc.Validate(p.loader.Context); err != nil {
		return nil, fmt.Errorf("OpenAPI spec validation failed: %w", err)
	}

	// Create a translation context
	ctx := NewTranslationContext(false)

	// Build the parsed spec
	spec := &ParsedSpec{
		Version: doc.OpenAPI,
		Info: Info{
			Title:       doc.Info.Title,
			Description: doc.Info.Description,
			Version:     doc.Info.Version,
		},
		Paths:     make(map[string]*PathItem),
		Schemas:   make(map[string]*SchemaInfo),
		Security:  make(map[string]*SecurityScheme),
		ctx:       ctx,
	}

	// Extract schemas from components
	if doc.Components != nil && doc.Components.Schemas != nil {
		for name, schemaRef := range doc.Components.Schemas {
			if schemaRef == nil || schemaRef.Value == nil {
				continue
			}
			schemaInfo, err := p.parseSchema(schemaRef.Value, name, ctx, []string{name})
			if err != nil {
				ctx.Warnings.AddError(name, err.Error())
				continue
			}
			spec.Schemas[name] = schemaInfo
			ctx.Schemas[name] = schemaInfo
		}
	}

	// Extract paths and operations
	for path, pathItem := range doc.Paths.Map() {
		if pathItem == nil {
			continue
		}

		parsedPath := &PathItem{
			Path:       path,
			Operations: make(map[string]*Operation),
		}

		// Process each HTTP method
		for method, operationRef := range map[string]*openapi3.Operation{
			"get":    pathItem.Get,
			"post":   pathItem.Post,
			"put":    pathItem.Put,
			"delete": pathItem.Delete,
			"patch":  pathItem.Patch,
			"head":   pathItem.Head,
			"options": pathItem.Options,
		} {
			if operationRef == nil {
				continue
			}

			// Parse the operation
			op := p.parseOperation(operationRef, method, path, ctx)
			parsedPath.Operations[method] = op
		}

		spec.Paths[path] = parsedPath
	}

	// Extract security schemes
	if doc.Components != nil && doc.Components.SecuritySchemes != nil {
		for name, securityRef := range doc.Components.SecuritySchemes {
			if securityRef == nil || securityRef.Value == nil {
				continue
			}
			spec.Security[name] = &SecurityScheme{
				Type:   securityRef.Value.Type,
				Scheme: securityRef.Value.Scheme,
			}
		}
	}

	return spec, nil
}

// parseSchema converts an OpenAPI schema to our internal SchemaInfo representation.
func (p *Parser) parseSchema(schema *openapi3.Schema, name string, ctx *TranslationContext, refPath []string) (*SchemaInfo, error) {
	if schema == nil {
		return nil, errors.New("schema is nil")
	}

	// Check for circular references
	refKey := strings.Join(refPath, "/")
	if ctx.CircularRefTracker[refKey] {
		// Circular reference detected
		return &SchemaInfo{
			Name:       name,
			Type:       MakeCustomType(name),
			IsCircular: true,
		}, nil
	}
	ctx.CircularRefTracker[refKey] = true
	defer delete(ctx.CircularRefTracker, refKey)

	info := &SchemaInfo{
		Name:        name,
		Description: schema.Description,
		Required:    make(map[string]bool),
		Properties:  make(map[string]*SchemaInfo),
	}

	// Check for enum
	if len(schema.Enum) > 0 {
		info.IsEnum = true
		info.Enum = make([]string, 0, len(schema.Enum))
		for _, enumVal := range schema.Enum {
			if strVal, ok := enumVal.(string); ok {
				info.Enum = append(info.Enum, strVal)
			}
		}
		// Enums are typically string-based
		info.Type = TypeString
		return info, nil
	}

	// Check for oneOf/anyOf (unsupported)
	if len(schema.OneOf) > 0 {
		ctx.Warnings.AddWarning(name, "oneOf is not supported, using first option or string fallback")
		// Try to use the first option
		if len(schema.OneOf) > 0 && schema.OneOf[0] != nil && schema.OneOf[0].Value != nil {
			return p.parseSchema(schema.OneOf[0].Value, name, ctx, refPath)
		}
		info.Type = TypeString
		return info, nil
	}

	if len(schema.AnyOf) > 0 {
		ctx.Warnings.AddWarning(name, "anyOf is not supported, using first option or string fallback")
		// Try to use the first option
		if len(schema.AnyOf) > 0 && schema.AnyOf[0] != nil && schema.AnyOf[0].Value != nil {
			return p.parseSchema(schema.AnyOf[0].Value, name, ctx, refPath)
		}
		info.Type = TypeString
		return info, nil
	}

	// Check for allOf (composition)
	if len(schema.AllOf) > 0 {
		info.AllOf = make([]*SchemaInfo, 0, len(schema.AllOf))
		for _, allOfRef := range schema.AllOf {
			if allOfRef == nil {
				continue
			}

			var nestedInfo *SchemaInfo
			var err error

			// Check if this is a $ref
			if allOfRef.Ref != "" {
				// Extract the ref name (e.g., "#/components/schemas/Base" -> "Base")
				refName := extractRefName(allOfRef.Ref)
				if allOfRef.Value != nil {
					// Parse the referenced schema
					nestedInfo, err = p.parseSchema(allOfRef.Value, refName, ctx, append(refPath, refName))
					if err != nil {
						return nil, err
					}
					// Store the ref name for extends generation
					nestedInfo.RefName = refName
				} else {
					// Create a placeholder SchemaInfo for the ref
					nestedInfo = &SchemaInfo{
						Name:     refName,
						RefName:  refName,
						IsObject: true,
						Type:     MakeCustomType(refName),
					}
				}
			} else if allOfRef.Value != nil {
				// Generate a synthetic name for inline allOf schemas
				nestedName := fmt.Sprintf("%s_allof_%d", name, len(info.AllOf))
				nestedInfo, err = p.parseSchema(allOfRef.Value, nestedName, ctx, append(refPath, nestedName))
				if err != nil {
					return nil, err
				}
			} else {
				continue
			}

			info.AllOf = append(info.AllOf, nestedInfo)
		}
		// allOf is handled specially during Pulse generation
		info.IsObject = true
		info.Type = MakeCustomType(name)
		return info, nil
	}

	// Handle array types
	if schema.Type != nil && schema.Type.Slice() != nil && len(schema.Type.Slice()) > 0 && schema.Type.Slice()[0] == "array" {
		info.IsArray = true
		if schema.Items != nil && schema.Items.Value != nil {
			itemsName := fmt.Sprintf("%s_item", name)
			items, err := p.parseSchema(schema.Items.Value, itemsName, ctx, append(refPath, itemsName))
			if err != nil {
				return nil, err
			}
			info.Items = items
			info.Type = MakeArrayType(items.Type)
		} else {
			info.Type = MakeArrayType(TypeString)
		}
		return info, nil
	}

	// Handle object types with additionalProperties (map)
	if schema.AdditionalProperties.Schema != nil && schema.AdditionalProperties.Schema.Value != nil {
		info.IsMap = true
		addPropsName := fmt.Sprintf("%s_value", name)
		addProps, err := p.parseSchema(schema.AdditionalProperties.Schema.Value, addPropsName, ctx, append(refPath, addPropsName))
		if err != nil {
			return nil, err
		}
		info.AdditionalProperties = addProps
		info.Type = MakeMapType(addProps.Type)
		return info, nil
	}

	// Handle object types with properties
	if len(schema.Properties) > 0 {
		info.IsObject = true
		info.Type = MakeCustomType(name)

		// Mark required fields
		for _, req := range schema.Required {
			info.Required[req] = true
		}

		// Parse properties
		for propName, propRef := range schema.Properties {
			if propRef == nil || propRef.Value == nil {
				continue
			}
			propName := propName
			propInfo, err := p.parseSchema(propRef.Value, propName, ctx, append(refPath, propName))
			if err != nil {
				ctx.Warnings.AddError(name, fmt.Sprintf("failed to parse property %s: %v", propName, err))
				continue
			}
			info.Properties[propName] = propInfo
		}
		return info, nil
	}

	// Check for binary format (not supported)
	if len(schema.Format) > 0 && (schema.Format == "binary" || schema.Format == "byte") {
		return nil, fmt.Errorf("binary format is not supported (field: %s)", name)
	}

	// Handle primitive types
	if schema.Type != nil && len(schema.Type.Slice()) > 0 {
		openapiType := &OpenAPIType{
			Type:     schema.Type.Slice()[0],
			Format:   schema.Format,
			Nullable: schema.Nullable,
		}
		info.Type = MapOpenAPITypeToPulse(openapiType)
		return info, nil
	}

	// If no type is specified, default to string
	info.Type = TypeString
	return info, nil
}

// parseOperation converts an OpenAPI operation to our internal Operation representation.
func (p *Parser) parseOperation(op *openapi3.Operation, method, path string, ctx *TranslationContext) *Operation {
	operation := &Operation{
		ID:          op.OperationID,
		Method:      method,
		Path:        path,
		Summary:     op.Summary,
		Description: op.Description,
		Tags:        op.Tags,
		Parameters:  make([]*Parameter, 0),
		RequestBody: nil,
		Responses:   make(map[string]*Response),
	}

	// Extract tags
	if len(op.Tags) > 0 {
		operation.Tag = op.Tags[0]
	}

	// Extract parameters
	for _, paramRef := range op.Parameters {
		if paramRef == nil || paramRef.Value == nil {
			continue
		}
		param := paramRef.Value

		// Skip header and cookie params with warning
		if param.In == "header" {
			ctx.Warnings.AddWarning(operation.ID, fmt.Sprintf("header parameter '%s' is not supported", param.Name))
			continue
		}
		if param.In == "cookie" {
			ctx.Warnings.AddWarning(operation.ID, fmt.Sprintf("cookie parameter '%s' is not supported", param.Name))
			continue
		}

		parsedParam := &Parameter{
			Name:        param.Name,
			In:          param.In,
			Description: param.Description,
			Required:    param.Required,
		}

		// Parse parameter schema
		if param.Schema != nil && param.Schema.Value != nil {
			paramSchema, err := p.parseSchema(param.Schema.Value, param.Name, ctx, []string{operation.ID, param.Name})
			if err != nil {
				ctx.Warnings.AddError(operation.ID, fmt.Sprintf("failed to parse parameter %s: %v", param.Name, err))
			} else {
				parsedParam.Schema = paramSchema
			}
		}

		operation.Parameters = append(operation.Parameters, parsedParam)
	}

	// Extract request body
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		reqBody := op.RequestBody.Value

		// Get JSON content
		content := reqBody.Content.Get("application/json")
		if content == nil {
			content = reqBody.Content.Get("application/x-www-form-urlencoded")
		}

		if content != nil && content.Schema != nil && content.Schema.Value != nil {
			bodyName := operation.ID
			if bodyName == "" {
				bodyName = fmt.Sprintf("%s%s", method, strings.ReplaceAll(path, "/", "_"))
			}
			bodyName = bodyName + "Body"

			bodySchema, err := p.parseSchema(content.Schema.Value, bodyName, ctx, []string{operation.ID, "body"})
			if err != nil {
				ctx.Warnings.AddError(operation.ID, fmt.Sprintf("failed to parse request body: %v", err))
			} else {
				operation.RequestBody = bodySchema
			}
		}
	}

	// Extract responses
	for code, responseRef := range op.Responses.Map() {
		if responseRef == nil || responseRef.Value == nil {
			continue
		}
		resp := responseRef.Value

		// Only keep 200, 201, and 204 responses
		if code != "200" && code != "201" && code != "204" && code != "default" {
			ctx.Warnings.AddWarning(operation.ID, fmt.Sprintf("response code %s is not supported (only 200, 201, 204, default are preserved)", code))
			continue
		}

		description := ""
		if resp.Description != nil {
			description = *resp.Description
		}
		parsedResp := &Response{
			Code:        code,
			Description: description,
		}

		// Get response schema from JSON content
		content := resp.Content.Get("application/json")
		if content != nil && content.Schema != nil && content.Schema.Value != nil {
			respName := operation.ID + "Response"
			respSchema, err := p.parseSchema(content.Schema.Value, respName, ctx, []string{operation.ID, "response", code})
			if err != nil {
				ctx.Warnings.AddError(operation.ID, fmt.Sprintf("failed to parse response %s: %v", code, err))
			} else {
				parsedResp.Schema = respSchema
			}
		}

		operation.Responses[code] = parsedResp
	}

	return operation
}

// ParsedSpec represents a parsed OpenAPI specification.
type ParsedSpec struct {
	// Version is the OpenAPI version (e.g., "3.0.0", "3.1.0")
	Version string
	// Info contains metadata about the API
	Info Info
	// Paths maps path strings to path items
	Paths map[string]*PathItem
	// Schemas contains all components/schemas
	Schemas map[string]*SchemaInfo
	// Security contains security schemes
	Security map[string]*SecurityScheme
	// ctx is the translation context
	ctx *TranslationContext
}

// GetWarnings returns all warnings encountered during parsing.
func (s *ParsedSpec) GetWarnings() []Warning {
	return s.ctx.Warnings.All()
}

// HasErrors returns true if there were any errors during parsing.
func (s *ParsedSpec) HasErrors() bool {
	return s.ctx.Warnings.HasErrors()
}

// Info contains metadata about the API.
type Info struct {
	Title       string
	Description string
	Version     string
}

// PathItem represents a path in the OpenAPI spec.
type PathItem struct {
	Path       string
	Operations map[string]*Operation
}

// Operation represents an operation in the OpenAPI spec.
type Operation struct {
	ID          string
	Method      string
	Path        string
	Tag         string
	Summary     string
	Description string
	Tags        []string
	Parameters  []*Parameter
	RequestBody *SchemaInfo
	Responses   map[string]*Response
}

// Parameter represents a parameter in an operation.
type Parameter struct {
	Name        string
	In          string // "path", "query", "header", "cookie"
	Description string
	Required    bool
	Schema      *SchemaInfo
}

// Response represents a response from an operation.
type Response struct {
	Code        string
	Description string
	Schema      *SchemaInfo
}

// SecurityScheme represents a security scheme.
type SecurityScheme struct {
	Type   string
	Scheme string
}

// GetOperationID generates a consistent operation ID from method and path.
func GetOperationID(method, path string) string {
	// Convert path to a valid identifier
	parts := strings.Split(strings.Trim(path, "/"), "/")
	var sb strings.Builder
	sb.WriteString(method)
	for _, part := range parts {
		if part != "" {
			// Remove path parameter braces {id} -> id
			part = strings.Trim(part, "{}")
			sb.WriteString(strings.Title(part))
		}
	}
	return sb.String()
}

// PathToMethodName converts a path to a valid method name.
func PathToMethodName(path string) string {
	// Remove path parameters and convert to PascalCase
	parts := strings.Split(path, "/")
	var result string
	for _, part := range parts {
		part = strings.Trim(part, "{}")
		if part == "" {
			continue
		}
		result += strings.Title(part)
	}
	return result
}

// extractRefName extracts the schema name from a $ref string.
// For example, "#/components/schemas/Base" -> "Base"
func extractRefName(ref string) string {
	// Handle local references like "#/components/schemas/Name"
	parts := strings.Split(ref, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ref
}

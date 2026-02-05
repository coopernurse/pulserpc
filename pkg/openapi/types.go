package openapi

import (
	"errors"
	"fmt"
	"strings"
)

// This file contains shared types and interfaces for OpenAPI translation.
// Phase 2: Type mapping and translation context implementation

var (
	// ErrNotImplemented is returned when a feature is not yet implemented.
	ErrNotImplemented = errors.New("not implemented yet")
)

// Warning represents a warning or error encountered during translation.
type Warning struct {
	// Level is either "warning" or "error"
	Level string
	// Message is the warning message
	Message string
	// Location is the source location (e.g., file path, schema name)
	Location string
}

// String returns a formatted representation of the warning.
func (w Warning) String() string {
	if w.Location != "" {
		return fmt.Sprintf("%s: %s: %s", w.Location, w.Level, w.Message)
	}
	return fmt.Sprintf("%s: %s", w.Level, w.Message)
}

// Warnings collects warnings during translation.
type Warnings struct {
	warnings []Warning
}

// NewWarnings creates a new Warnings collection.
func NewWarnings() *Warnings {
	return &Warnings{
		warnings: make([]Warning, 0),
	}
}

// AddWarning adds a warning to the collection.
func (w *Warnings) AddWarning(location, message string) {
	w.warnings = append(w.warnings, Warning{
		Level:    "warning",
		Message:  message,
		Location: location,
	})
}

// AddError adds an error to the collection (non-fatal).
func (w *Warnings) AddError(location, message string) {
	w.warnings = append(w.warnings, Warning{
		Level:    "error",
		Message:  message,
		Location: location,
	})
}

// HasErrors returns true if there are any error-level warnings.
func (w *Warnings) HasErrors() bool {
	for _, warning := range w.warnings {
		if warning.Level == "error" {
			return true
		}
	}
	return false
}

// All returns all warnings.
func (w *Warnings) All() []Warning {
	return w.warnings
}

// Count returns the number of warnings.
func (w *Warnings) Count() int {
	return len(w.warnings)
}

// TranslationContext holds state during the translation process.
type TranslationContext struct {
	// Warnings collects warnings during translation
	Warnings *Warnings
	// Strict mode treats warnings as errors
	Strict bool
	// Schemas is the map of parsed schemas from OpenAPI spec
	Schemas map[string]*SchemaInfo
	// CircularRefTracker tracks circular references
	CircularRefTracker map[string]bool
}

// NewTranslationContext creates a new translation context.
func NewTranslationContext(strict bool) *TranslationContext {
	return &TranslationContext{
		Warnings:          NewWarnings(),
		Strict:            strict,
		Schemas:           make(map[string]*SchemaInfo),
		CircularRefTracker: make(map[string]bool),
	}
}

// SchemaInfo represents a parsed schema from the OpenAPI spec.
type SchemaInfo struct {
	// Name is the schema name (from components/schemas or generated)
	Name string
	// Description is the schema description
	Description string
	// Type is the Pulse type
	Type PulseType
	// Required fields (for object schemas)
	Required map[string]bool
	// Properties map for object schemas
	Properties map[string]*SchemaInfo
	// IsArray indicates if this is an array type
	IsArray bool
	// Items type for array schemas
	Items *SchemaInfo
	// IsMap indicates if this is a map type
	IsMap bool
	// AdditionalProperties type for map schemas
	AdditionalProperties *SchemaInfo
	// Enum values for enum schemas
	Enum []string
	// AllOf schemas for composition
	AllOf []*SchemaInfo
	// RefName is the name of the referenced schema (for $ref)
	RefName string
	// IsCircular indicates if this schema has circular references
	IsCircular bool
	// IsEnum indicates if this is an enum type
	IsEnum bool
	// IsObject indicates if this is an object/struct type
	IsObject bool
}

// PulseType represents a Pulse type.
type PulseType struct {
	// Name is the type name (e.g., "string", "int", "User")
	Name string
	// IsOptional indicates if the type is optional ([optional])
	IsOptional bool
	// IsArray indicates if the type is an array
	IsArray bool
	// ArrayElementType is the element type for arrays
	ArrayElementType *PulseType
	// IsMap indicates if the type is a map
	IsMap bool
	// MapValueType is the value type for maps
	MapValueType *PulseType
	// IsPrimitive indicates if this is a primitive type
	IsPrimitive bool
	// IsCustom indicates if this is a custom/user-defined type
	IsCustom bool
}

// String returns the Pulse type as a string.
func (t PulseType) String() string {
	var sb strings.Builder

	if t.IsOptional {
		sb.WriteString("[optional] ")
	}

	if t.IsArray {
		sb.WriteString("[]")
		if t.ArrayElementType != nil {
			sb.WriteString(t.ArrayElementType.String())
		}
	} else if t.IsMap {
		sb.WriteString("map[string]")
		if t.MapValueType != nil {
			sb.WriteString(t.MapValueType.String())
		}
	} else {
		sb.WriteString(t.Name)
	}

	return sb.String()
}

// Primitive types mapping from OpenAPI to Pulse.
var (
	TypeString   = PulseType{Name: "string", IsPrimitive: true}
	TypeInt      = PulseType{Name: "int", IsPrimitive: true}
	TypeFloat    = PulseType{Name: "float", IsPrimitive: true}
	TypeBool     = PulseType{Name: "bool", IsPrimitive: true}
	TypeVoid     = PulseType{Name: "void", IsPrimitive: true}
)

// OpenAPIType represents the OpenAPI type format.
type OpenAPIType struct {
	// Type is the OpenAPI type (e.g., "string", "integer", "number", "boolean", "array", "object")
	Type string
	// Format is the OpenAPI format (e.g., "int32", "int64", "float", "double")
	Format string
	// Nullable indicates if the type is nullable (OpenAPI 3.1)
	Nullable bool
}

// MapOpenAPITypeToPulse maps an OpenAPI type to a Pulse type.
// This implements the type mapping from Phase 2, Task 2.
func MapOpenAPITypeToPulse(openapiType *OpenAPIType) PulseType {
	if openapiType == nil {
		return TypeString
	}

	baseType := TypeString // Default fallback

	switch openapiType.Type {
	case "string":
		baseType = TypeString
	case "integer":
		// int32 and int64 both map to Pulse int
		baseType = TypeInt
	case "number":
		// float and double both map to Pulse float
		baseType = TypeFloat
	case "boolean":
		baseType = TypeBool
	case "array":
		// Array types are handled separately in schema processing
		baseType = TypeString
	case "object":
		// Object types are handled separately in schema processing
		baseType = TypeString
	}

	// Handle nullable (OpenAPI 3.1 or 3.0 with nullable: true)
	if openapiType.Nullable {
		baseType.IsOptional = true
	}

	return baseType
}

// IsPrimitiveType returns true if the given type name is a primitive Pulse type.
func IsPrimitiveType(typeName string) bool {
	switch typeName {
	case "string", "int", "float", "bool", "void":
		return true
	default:
		return false
	}
}

// MakeOptional creates an optional version of the given type.
func MakeOptional(t PulseType) PulseType {
	t.IsOptional = true
	return t
}

// MakeArrayType creates an array type with the given element type.
func MakeArrayType(elementType PulseType) PulseType {
	return PulseType{
		Name:             "",
		IsArray:          true,
		ArrayElementType: &elementType,
	}
}

// MakeMapType creates a map type with the given value type.
func MakeMapType(valueType PulseType) PulseType {
	return PulseType{
		Name:         "",
		IsMap:        true,
		MapValueType: &valueType,
	}
}

// MakeCustomType creates a custom/user-defined type.
func MakeCustomType(typeName string) PulseType {
	return PulseType{
		Name:     typeName,
		IsCustom: true,
	}
}

// FormatComment converts a description string to a Pulse comment.
// Handles multi-line comments and ensures proper formatting.
func FormatComment(description string) string {
	if description == "" {
		return ""
	}

	// Split into lines and trim each
	lines := strings.Split(strings.TrimSpace(description), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}

	return strings.Join(lines, "\n")
}

// ToValidIdentifier converts a string to a valid Pulse identifier.
// Converts to lowercase and replaces non-alphanumeric characters with underscores.
func ToValidIdentifier(s string) string {
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			result.WriteRune(r)
		} else if r >= 'A' && r <= 'Z' {
			// Convert to lowercase
			result.WriteRune(r + 32)
		} else {
			// Replace with underscore
			result.WriteRune('_')
		}
	}
	return result.String()
}

// SanitizeComment removes characters that might break comment formatting.
func SanitizeComment(comment string) string {
	// Remove null characters and other problematic characters
	result := strings.ReplaceAll(comment, "\x00", "")
	result = strings.ReplaceAll(result, "\r", "")
	return strings.TrimSpace(result)
}

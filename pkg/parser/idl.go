package parser

import (
	"github.com/alecthomas/participle/v2/lexer"
)

// IDL represents the root structure containing all parsed IDL elements
type IDL struct {
	RootNamespace string       `json:"rootNamespace,omitempty"` // Namespace of the root file being parsed
	Checksum      string       `json:"checksum,omitempty"`      // SHA-256 checksum of structural elements
	Interfaces    []*Interface `json:"interfaces,omitempty"`
	Structs       []*Struct    `json:"structs,omitempty"`
	Enums         []*Enum      `json:"enums,omitempty"`
	Errors        []*ErrorDef  `json:"errors,omitempty"`
}

// Interface represents a service interface with methods
type Interface struct {
	Pos       lexer.Position `json:"-"`
	Name      string         `json:"name"`
	Namespace string         `json:"namespace,omitempty"`
	Comment   string         `json:"comment,omitempty"`
	Methods   []*Method      `json:"methods,omitempty"`
}

// Method represents an interface method with parameters and return type
type Method struct {
	Pos            lexer.Position `json:"-"`
	Name           string         `json:"name"`
	Parameters     []*Parameter   `json:"parameters,omitempty"`
	ReturnType     *Type          `json:"returnType"`
	ReturnOptional bool           `json:"returnOptional,omitempty"`
	Raises         []string       `json:"raises,omitempty"`
}

// Parameter represents a method parameter
type Parameter struct {
	Pos  lexer.Position `json:"-"`
	Name string         `json:"name"`
	Type *Type          `json:"type"`
}

// Struct represents a struct definition with fields and optional extends
type Struct struct {
	Pos       lexer.Position `json:"-"`
	Name      string         `json:"name"`
	Namespace string         `json:"namespace,omitempty"`
	Extends   string         `json:"extends,omitempty"` // Empty if no extends, can be qualified (e.g., "inc.Response")
	Comment   string         `json:"comment,omitempty"`
	Fields    []*Field       `json:"fields,omitempty"`
}

// Field represents a struct field with type, optional flag, and comments
type Field struct {
	Pos      lexer.Position `json:"-"`
	Name     string         `json:"name"`
	Type     *Type          `json:"type"`
	Optional bool           `json:"optional,omitempty"`
	Comment  string         `json:"comment,omitempty"`
}

// EnumValue represents a single enum value with optional comment
type EnumValue struct {
	Name    string `json:"name"`
	Comment string `json:"comment,omitempty"`
}

// Enum represents an enum definition with values
type Enum struct {
	Pos       lexer.Position `json:"-"`
	Name      string         `json:"name"`
	Namespace string         `json:"namespace,omitempty"`
	Comment   string         `json:"comment,omitempty"`
	Values    []*EnumValue   `json:"values,omitempty"`
}

// ErrorDef represents a declared error with code, name, and message
type ErrorDef struct {
	Pos       lexer.Position `json:"-"`
	Name      string         `json:"name"`                // e.g., "NotFound" or "errors.NotFound" if imported
	Namespace string         `json:"namespace,omitempty"` // Namespace where declared
	Code      int            `json:"code"`                // JSON-RPC error code
	Message   string         `json:"message"`             // Error message template
	Comment   string         `json:"comment,omitempty"`   // Documentation comment
}

// Type represents a type (built-in, array, map, or user-defined)
type Type struct {
	Pos lexer.Position `json:"-"`

	// For built-in types: string, int, float, bool
	BuiltIn string `json:"builtIn,omitempty"`

	// For arrays: []Type
	Array *Type `json:"array,omitempty"`

	// For maps: map[string]ValueType
	MapValue *Type `json:"mapValue,omitempty"`

	// For user-defined types (interfaces, structs, enums)
	UserDefined string `json:"userDefined,omitempty"`
}

// IsBuiltIn returns true if this is a built-in type
func (t *Type) IsBuiltIn() bool {
	return t.BuiltIn != ""
}

// IsArray returns true if this is an array type
func (t *Type) IsArray() bool {
	return t.Array != nil
}

// IsMap returns true if this is a map type
func (t *Type) IsMap() bool {
	return t.MapValue != nil
}

// IsUserDefined returns true if this is a user-defined type
func (t *Type) IsUserDefined() bool {
	return t.UserDefined != ""
}

// String returns a string representation of the type
func (t *Type) String() string {
	if t.IsBuiltIn() {
		return t.BuiltIn
	}
	if t.IsArray() {
		return "[]" + t.Array.String()
	}
	if t.IsMap() {
		return "map[string]" + t.MapValue.String()
	}
	if t.IsUserDefined() {
		return t.UserDefined
	}
	return "unknown"
}

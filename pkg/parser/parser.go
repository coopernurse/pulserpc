package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

var (
	idlLexer = lexer.MustSimple([]lexer.SimpleRule{
		{Name: "Comment", Pattern: `//[^\n]*`},
		{Name: "Whitespace", Pattern: `[ \t\r\n]+`},
		{Name: "Optional", Pattern: `\[optional\]`},
		{Name: "StringLiteral", Pattern: `"[^"]*"`},
		// Keywords must use word boundaries to avoid matching prefixes of identifiers
		// e.g., "strings" should be Ident, not String + "s"
		{Name: "Namespace", Pattern: `\bnamespace\b`},
		{Name: "Interface", Pattern: `\binterface\b`},
		{Name: "Struct", Pattern: `\bstruct\b`},
		{Name: "Enum", Pattern: `\benum\b`},
		{Name: "Extends", Pattern: `\bextends\b`},
		{Name: "Map", Pattern: `\bmap\b`},
		{Name: "String", Pattern: `\bstring\b`},
		{Name: "Float", Pattern: `\bfloat\b`},
		{Name: "Bool", Pattern: `\bbool\b`},
		{Name: "Int", Pattern: `\bint\b`},
		{Name: "Ident", Pattern: `[a-zA-Z][a-zA-Z0-9_]*`},
		{Name: "Dot", Pattern: `\.`},
		{Name: "Punct", Pattern: `[{}[\]();,]`},
	})

	parser = participle.MustBuild[IDLFile](
		participle.Lexer(idlLexer),
		participle.Elide("Whitespace", "Comment"),
		participle.UseLookahead(2),
	)
)

// IDLFile is the root grammar structure
type IDLFile struct {
	Pos      lexer.Position
	Elements []*IDLElement `parser:"@@*"`
}

// IDLElement represents any top-level IDL element
type IDLElement struct {
	Pos       lexer.Position
	Namespace *NamespaceDef `parser:"  'namespace' @@"`
	Interface *InterfaceDef `parser:"| 'interface' @@"`
	Struct    *StructDef    `parser:"| 'struct' @@"`
	Enum      *EnumDef      `parser:"| 'enum' @@"`
}

// ImportString is a custom type for parsing import paths
type ImportString string

// String returns the string value
func (i ImportString) String() string {
	return string(i)
}

// ImportDef represents an import statement (for backwards compatibility)
type ImportDef struct {
	Pos  lexer.Position
	Path string
}

// NamespaceDef represents a namespace declaration
type NamespaceDef struct {
	Pos  lexer.Position
	Name string `parser:"@Ident"`
}

// InterfaceDef represents an interface definition
type InterfaceDef struct {
	Pos     lexer.Position
	Name    string       `parser:"@Ident '{'"`
	Methods []*MethodDef `parser:"@@* '}'"`
}

// MethodDef represents a method definition
type MethodDef struct {
	Pos            lexer.Position
	Name           string          `parser:"@Ident '('"`
	Parameters     []*ParameterDef `parser:"( @@ (',' @@)* )? ')'"`
	ReturnType     *TypeExpr       `parser:"@@"`
	ReturnOptional bool            `parser:"( @Optional )?"`
}

// ParameterDef represents a parameter definition
type ParameterDef struct {
	Pos  lexer.Position
	Name string    `parser:"@Ident"`
	Type *TypeExpr `parser:"@@"`
}

// StructDef represents a struct definition
type StructDef struct {
	Pos     lexer.Position
	Name    string         `parser:"@Ident"`
	Extends *QualifiedName `parser:"( 'extends' @@ )?"`
	Fields  []*FieldDef    `parser:"'{' @@* '}'"`
}

// QualifiedName represents a qualified type name (e.g., "inc.Response" or "Response")
type QualifiedName struct {
	Pos   lexer.Position
	Parts []string `parser:"@Ident ( '.' @Ident )*"`
}

// String returns the qualified name as a string
func (q *QualifiedName) String() string {
	return strings.Join(q.Parts, ".")
}

// FieldDef represents a field definition
type FieldDef struct {
	Pos      lexer.Position
	Name     string    `parser:"@Ident"`
	Type     *TypeExpr `parser:"@@"`
	Optional bool      `parser:"( @Optional )?"`
}

// EnumDef represents an enum definition
type EnumDef struct {
	Pos    lexer.Position
	Name   string   `parser:"@Ident '{'"`
	Values []string `parser:"@Ident* '}'"`
}

// TypeExpr represents a type expression
type TypeExpr struct {
	Pos         lexer.Position
	BuiltIn     *string        `parser:"( @String | @Int | @Float | @Bool )"`
	Array       *ArrayType     `parser:"| @@"`
	MapType     *MapTypeExpr   `parser:"| @@"`
	UserDefined *QualifiedName `parser:"| @@"`
}

// ArrayType represents []Type - uses TextUnmarshaler for recursive parsing
type ArrayType struct {
	Pos          lexer.Position
	ArrayMarkers string `parser:"'[' ']'"`
	// Capture the element type - can be nested
	ElementType *TypeExpr `parser:"@@"`
}

// MapTypeExpr represents map[string]ValueType - matches map pattern, value type parsed in post-processing
type MapTypeExpr struct {
	Pos        lexer.Position
	MapPattern string `parser:"@Map '[' @String ']'"`
	// Capture the value type - can be nested
	ValueType *TypeExpr `parser:"@@"`
}

// parseNestedTypes post-processes the AST to parse nested types recursively
func parseNestedTypes(expr *TypeExpr) error {
	if expr == nil {
		return nil
	}

	if expr.Array != nil && expr.Array.ElementType != nil {
		// ElementType is already parsed by the grammar, just recursively parse nested types
		if err := parseNestedTypes(expr.Array.ElementType); err != nil {
			return err
		}
	}

	if expr.MapType != nil && expr.MapType.ValueType != nil {
		// ValueType is already parsed by the grammar, just recursively parse nested types
		if err := parseNestedTypes(expr.MapType.ValueType); err != nil {
			return err
		}
	}

	return nil
}

// extractPrecedingComments extracts comments directly above a given position in the input.
// It scans backwards from the position, collecting consecutive comment lines (no blank lines in between).
// Returns the concatenated comment text with `//` prefix removed and whitespace trimmed on each line,
// joined with newlines. Returns empty string if no comments found.
func extractPrecedingComments(input string, pos lexer.Position) string {
	if pos.Line <= 1 {
		return ""
	}

	lines := strings.Split(input, "\n")
	if len(lines) < pos.Line {
		return ""
	}

	// Start from the line before the target (pos.Line is 1-indexed)
	var commentLines []string
	targetLineIdx := pos.Line - 1

	// Scan backwards from the line before the target
	for i := targetLineIdx - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])

		// If we hit a blank line, stop
		if line == "" {
			break
		}

		// If it's a comment line, extract the content
		if strings.HasPrefix(line, "//") {
			// Remove "//" prefix and trim whitespace
			commentText := strings.TrimSpace(line[2:])
			// Prepend to maintain order (we're scanning backwards)
			commentLines = append([]string{commentText}, commentLines...)
		} else {
			// If it's not a comment and not blank, stop
			break
		}
	}

	if len(commentLines) == 0 {
		return ""
	}

	return strings.Join(commentLines, "\n")
}

// extractEnumValueComments extracts comments for all enum values in one pass.
// Returns a slice of comments, one for each value in the same order as values.
func extractEnumValueComments(input string, enumPos lexer.Position, values []string) []string {
	lines := strings.Split(input, "\n")
	if len(lines) < enumPos.Line {
		return make([]string, len(values))
	}

	// Find the enum body (starts after the opening brace)
	enumStartLineIdx := enumPos.Line - 1
	enumBodyStart := -1
	for i := enumStartLineIdx; i < len(lines); i++ {
		if strings.Contains(lines[i], "{") {
			enumBodyStart = i + 1
			break
		}
	}

	if enumBodyStart < 0 {
		return make([]string, len(values))
	}

	// Map to track which value index we're looking for
	valueIndex := 0
	comments := make([]string, len(values))

	// Scan through the enum body to find values
	for i := enumBodyStart; i < len(lines); i++ {
		line := lines[i]
		trimmedLine := strings.TrimSpace(line)

		// Skip comment-only lines (we'll collect them when we find the value)
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "//") {
			continue
		}

		// If we hit the closing brace, stop
		if strings.Contains(trimmedLine, "}") {
			break
		}

		// Remove any inline comments first
		if commentIdx := strings.Index(trimmedLine, "//"); commentIdx >= 0 {
			trimmedLine = strings.TrimSpace(trimmedLine[:commentIdx])
		}

		// Check if this line matches the current value we're looking for
		if valueIndex < len(values) {
			valueName := values[valueIndex]
			isValueLine := false
			if trimmedLine == valueName {
				isValueLine = true
			} else {
				// Check if valueName is the first word
				words := strings.Fields(trimmedLine)
				if len(words) > 0 && words[0] == valueName {
					isValueLine = true
				}
			}

			if isValueLine {
				// Extract comments directly above this line
				var commentLines []string
				for j := i - 1; j >= enumBodyStart; j-- {
					prevLine := strings.TrimSpace(lines[j])

					// Stop at blank lines
					if prevLine == "" {
						break
					}

					// If it's a comment line, collect it
					if strings.HasPrefix(prevLine, "//") {
						commentText := strings.TrimSpace(prevLine[2:])
						commentLines = append([]string{commentText}, commentLines...)
					} else {
						// If it's not a comment and not blank, stop
						break
					}
				}

				if len(commentLines) > 0 {
					comments[valueIndex] = strings.Join(commentLines, "\n")
				}
				valueIndex++
			}
		}
	}

	return comments
}

// ParseIDL parses an IDL file string and returns the parsed IDL structure
// filename is used for resolving relative imports
func ParseIDL(filename string, input string) (*IDL, error) {
	visited := make(map[string]bool)
	return parseIDLWithImports(filename, input, visited)
}

// parseIDLWithImports parses an IDL file and resolves imports recursively
func parseIDLWithImports(filename string, input string, visited map[string]bool) (*IDL, error) {
	// Normalize filename path
	absPath, err := filepath.Abs(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path for %s: %w", filename, err)
	}

	// Check for import cycles
	if visited[absPath] {
		return nil, fmt.Errorf("import cycle detected: file %s is already being processed", filename)
	}
	visited[absPath] = true
	defer delete(visited, absPath)

	// Pre-process: extract imports manually using regex
	importRegex := regexp.MustCompile(`(?m)^\s*import\s+"([^"]+)"`)
	importMatches := importRegex.FindAllStringSubmatch(input, -1)
	importSet := make(map[string]bool) // Deduplicate imports
	var imports []string
	for _, match := range importMatches {
		if len(match) > 1 {
			importPath := match[1]
			if !importSet[importPath] {
				importSet[importPath] = true
				imports = append(imports, importPath)
			}
		}
	}

	// Remove import lines from input for parsing
	// Match import statement with any whitespace
	removeImportRegex := regexp.MustCompile(`(?m)^\s*import\s+"[^"]+"\s*$`)
	filteredInput := removeImportRegex.ReplaceAllString(input, "")

	// Parse the file
	file, err := parser.ParseString(filename, filteredInput)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	// Extract namespace
	var namespace string
	for _, elem := range file.Elements {
		if elem.Namespace != nil {
			if namespace != "" {
				return nil, fmt.Errorf("multiple namespace declarations in file %s", filename)
			}
			namespace = elem.Namespace.Name
		}
	}

	// Resolve imports
	baseDir := filepath.Dir(absPath)
	importedIDLs := make([]struct {
		namespace string
		idl       *IDL
	}, 0)
	namespaceMap := make(map[string]string) // namespace -> file path

	for _, importPath := range imports {
		// Resolve import path relative to current file's directory
		resolvedPath := importPath
		if !filepath.IsAbs(importPath) {
			resolvedPath = filepath.Join(baseDir, importPath)
		}

		// Read and parse imported file
		importedContent, err := os.ReadFile(resolvedPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read import file %s (resolved from %s): %w", resolvedPath, importPath, err)
		}

		importedIDL, err := parseIDLWithImports(resolvedPath, string(importedContent), visited)
		if err != nil {
			return nil, fmt.Errorf("failed to parse imported file %s: %w", resolvedPath, err)
		}

		// Get namespace from imported file
		importedNamespace := ""
		for _, s := range importedIDL.Structs {
			if s.Namespace != "" {
				importedNamespace = s.Namespace
				break
			}
		}
		if importedNamespace == "" {
			for _, e := range importedIDL.Enums {
				if e.Namespace != "" {
					importedNamespace = e.Namespace
					break
				}
			}
		}
		if importedNamespace == "" {
			for _, i := range importedIDL.Interfaces {
				if i.Namespace != "" {
					importedNamespace = i.Namespace
					break
				}
			}
		}

		// Check for duplicate namespace
		if importedNamespace != "" {
			if existingFile, exists := namespaceMap[importedNamespace]; exists {
				return nil, fmt.Errorf("duplicate namespace %s: already used in %s, also used in %s", importedNamespace, existingFile, resolvedPath)
			}
			namespaceMap[importedNamespace] = resolvedPath
		}

		importedIDLs = append(importedIDLs, struct {
			namespace string
			idl       *IDL
		}{namespace: importedNamespace, idl: importedIDL})
	}

	// Post-process to parse nested types recursively
	var processTypeExpr = func(expr *TypeExpr) error {
		if expr == nil {
			return nil
		}
		return parseNestedTypes(expr)
	}

	for _, elem := range file.Elements {
		if elem.Interface != nil {
			for _, m := range elem.Interface.Methods {
				if err := processTypeExpr(m.ReturnType); err != nil {
					return nil, fmt.Errorf("error processing return type: %w", err)
				}
				for _, p := range m.Parameters {
					if err := processTypeExpr(p.Type); err != nil {
						return nil, fmt.Errorf("error processing parameter type: %w", err)
					}
				}
			}
		} else if elem.Struct != nil {
			for _, f := range elem.Struct.Fields {
				if err := processTypeExpr(f.Type); err != nil {
					return nil, fmt.Errorf("error processing field type: %w", err)
				}
			}
		}
	}

	idl := &IDL{
		RootNamespace: namespace,
		Interfaces:    make([]*Interface, 0),
		Structs:       make([]*Struct, 0),
		Enums:         make([]*Enum, 0),
	}

	// Process local elements
	for _, elem := range file.Elements {
		if elem.Interface != nil {
			// Extract interface comment
			interfaceComment := extractPrecedingComments(filteredInput, elem.Interface.Pos)
			iface := &Interface{
				Pos:       elem.Interface.Pos,
				Name:      elem.Interface.Name,
				Namespace: namespace,
				Comment:   interfaceComment,
				Methods:   make([]*Method, 0),
			}
			for _, m := range elem.Interface.Methods {
				method := &Method{
					Pos:            m.Pos,
					Name:           m.Name,
					Parameters:     make([]*Parameter, 0),
					ReturnType:     convertTypeExpr(m.ReturnType),
					ReturnOptional: m.ReturnOptional,
				}
				for _, p := range m.Parameters {
					method.Parameters = append(method.Parameters, &Parameter{
						Pos:  p.Pos,
						Name: p.Name,
						Type: convertTypeExpr(p.Type),
					})
				}
				iface.Methods = append(iface.Methods, method)
			}
			idl.Interfaces = append(idl.Interfaces, iface)
		} else if elem.Struct != nil {
			// Extract struct comment
			structComment := extractPrecedingComments(filteredInput, elem.Struct.Pos)
			s := &Struct{
				Pos:       elem.Struct.Pos,
				Name:      elem.Struct.Name,
				Namespace: namespace,
				Extends:   "",
				Comment:   structComment,
				Fields:    make([]*Field, 0),
			}
			if elem.Struct.Extends != nil {
				s.Extends = elem.Struct.Extends.String()
			}
			for _, f := range elem.Struct.Fields {
				// Extract field comment
				fieldComment := extractPrecedingComments(filteredInput, f.Pos)
				s.Fields = append(s.Fields, &Field{
					Pos:      f.Pos,
					Name:     f.Name,
					Type:     convertTypeExpr(f.Type),
					Optional: f.Optional,
					Comment:  fieldComment,
				})
			}
			idl.Structs = append(idl.Structs, s)
		} else if elem.Enum != nil {
			// Extract enum comment
			enumComment := extractPrecedingComments(filteredInput, elem.Enum.Pos)

			// Extract comments for all enum values in one pass
			valueComments := extractEnumValueComments(filteredInput, elem.Enum.Pos, elem.Enum.Values)

			// Convert enum values to EnumValue structs with comments
			enumValues := make([]*EnumValue, 0, len(elem.Enum.Values))
			for i, valueName := range elem.Enum.Values {
				comment := ""
				if i < len(valueComments) {
					comment = valueComments[i]
				}
				enumValues = append(enumValues, &EnumValue{
					Name:    valueName,
					Comment: comment,
				})
			}

			idl.Enums = append(idl.Enums, &Enum{
				Pos:       elem.Enum.Pos,
				Name:      elem.Enum.Name,
				Namespace: namespace,
				Comment:   enumComment,
				Values:    enumValues,
			})
		}
	}

	// Merge imported IDLs with namespace prefixes
	for _, imported := range importedIDLs {
		importedNamespace := imported.namespace
		importedIDL := imported.idl
		if importedNamespace != "" {
			// Build a map of unqualified to qualified names for this namespace
			typeMap := make(map[string]string)
			for _, s := range importedIDL.Structs {
				if s.Namespace == importedNamespace {
					typeMap[s.Name] = importedNamespace + "." + s.Name
				}
			}
			for _, e := range importedIDL.Enums {
				if e.Namespace == importedNamespace {
					typeMap[e.Name] = importedNamespace + "." + e.Name
				}
			}
			for _, i := range importedIDL.Interfaces {
				if i.Namespace == importedNamespace {
					typeMap[i.Name] = importedNamespace + "." + i.Name
				}
			}

			// Update type references within the same namespace to use qualified names
			updateTypeRefs := func(t *Type) {
				if t != nil && t.IsUserDefined() {
					if qualified, exists := typeMap[t.UserDefined]; exists {
						t.UserDefined = qualified
					}
				}
			}

			// Prefix types from the imported file with the imported namespace
			// Types from nested imports already have their namespace prefix
			for _, s := range importedIDL.Structs {
				// If the type's namespace matches the imported file's namespace, it's a local type - prefix it
				// If it has a different namespace, it's from a nested import - keep it as-is
				if s.Namespace == importedNamespace {
					// Local type from imported file - prefix it
					s.Name = importedNamespace + "." + s.Name
					// Update field type references
					for _, f := range s.Fields {
						updateTypeRefs(f.Type)
					}
					// Update extends reference
					if s.Extends != "" {
						if qualified, exists := typeMap[s.Extends]; exists {
							s.Extends = qualified
						}
					}
				}
				idl.Structs = append(idl.Structs, s)
			}
			for _, e := range importedIDL.Enums {
				if e.Namespace == importedNamespace {
					// Local type from imported file - prefix it
					e.Name = importedNamespace + "." + e.Name
				}
				idl.Enums = append(idl.Enums, e)
			}
			for _, i := range importedIDL.Interfaces {
				if i.Namespace == importedNamespace {
					// Local type from imported file - prefix it
					i.Name = importedNamespace + "." + i.Name
					// Update method parameter and return type references
					for _, m := range i.Methods {
						updateTypeRefs(m.ReturnType)
						for _, p := range m.Parameters {
							updateTypeRefs(p.Type)
						}
					}
				}
				idl.Interfaces = append(idl.Interfaces, i)
			}
		} else {
			// No namespace - add types as-is
			idl.Structs = append(idl.Structs, importedIDL.Structs...)
			idl.Enums = append(idl.Enums, importedIDL.Enums...)
			idl.Interfaces = append(idl.Interfaces, importedIDL.Interfaces...)
		}
	}

	return idl, nil
}

// convertTypeExpr converts a TypeExpr from the grammar to a Type in the IDL structure
func convertTypeExpr(expr *TypeExpr) *Type {
	if expr == nil {
		return nil
	}

	t := &Type{
		Pos: expr.Pos,
	}

	if expr.BuiltIn != nil {
		t.BuiltIn = *expr.BuiltIn
		return t
	}

	if expr.Array != nil {
		// ElementType should be parsed by post-processing
		if expr.Array.ElementType != nil {
			t.Array = convertTypeExpr(expr.Array.ElementType)
		}
		return t
	}

	if expr.MapType != nil {
		// ValueType should be parsed by now (post-processing)
		if expr.MapType.ValueType != nil {
			t.MapValue = convertTypeExpr(expr.MapType.ValueType)
		}
		return t
	}

	if expr.UserDefined != nil {
		t.UserDefined = expr.UserDefined.String()
		return t
	}

	return t
}

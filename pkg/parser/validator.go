package parser

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
)

var (
	builtInTypes = map[string]bool{
		"string": true,
		"int":    true,
		"float":  true,
		"bool":   true,
	}

	identifierRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)
)

// ValidateIDL validates the parsed IDL and returns any validation errors
func ValidateIDL(idl *IDL) error {
	errors := &ValidationErrors{Errors: make([]*ValidationError, 0)}

	// Validate that the root file has a namespace declaration
	// Exception: empty files (no types defined) are allowed without a namespace
	isEmpty := len(idl.Interfaces) == 0 && len(idl.Structs) == 0 && len(idl.Enums) == 0 && len(idl.Errors) == 0
	if idl.RootNamespace == "" && !isEmpty {
		errors.Add(&ValidationError{
			Line:   0,
			Column: 0,
			Msg:    "IDL file must declare a namespace at the top level",
		})
		// Return early if no namespace - other validations may not make sense
		if errors.HasErrors() {
			return errors
		}
	}

	// Build type registry and track positions for duplicate detection
	typeRegistry := make(map[string]lexer.Position)
	typeNames := make(map[string]string) // type name -> "interface", "struct", or "enum"

	// First pass: register all types and check for duplicates
	// For qualified names (namespace.Type), validate the base name part
	for _, iface := range idl.Interfaces {
		baseName := getBaseName(iface.Name)
		if !validateIdentifierName(baseName, errors, iface.Pos.Line, iface.Pos.Column) {
			continue
		}
		if existingPos, exists := typeRegistry[iface.Name]; exists {
			errors.Add(&ValidationError{
				Line:   iface.Pos.Line,
				Column: iface.Pos.Column,
				Msg:    fmt.Sprintf("duplicate type name: %s (previously defined as %s at %d:%d)", iface.Name, typeNames[iface.Name], existingPos.Line, existingPos.Column),
			})
		} else {
			typeRegistry[iface.Name] = iface.Pos
			typeNames[iface.Name] = "interface"
		}
	}

	// Register all structs
	for _, s := range idl.Structs {
		baseName := getBaseName(s.Name)
		if !validateIdentifierName(baseName, errors, s.Pos.Line, s.Pos.Column) {
			continue
		}
		if existingPos, exists := typeRegistry[s.Name]; exists {
			errors.Add(&ValidationError{
				Line:   s.Pos.Line,
				Column: s.Pos.Column,
				Msg:    fmt.Sprintf("duplicate type name: %s (previously defined as %s at %d:%d)", s.Name, typeNames[s.Name], existingPos.Line, existingPos.Column),
			})
		} else {
			typeRegistry[s.Name] = s.Pos
			typeNames[s.Name] = "struct"
		}
	}

	// Register all enums
	for _, enum := range idl.Enums {
		baseName := getBaseName(enum.Name)
		if !validateIdentifierName(baseName, errors, enum.Pos.Line, enum.Pos.Column) {
			continue
		}
		if existingPos, exists := typeRegistry[enum.Name]; exists {
			errors.Add(&ValidationError{
				Line:   enum.Pos.Line,
				Column: enum.Pos.Column,
				Msg:    fmt.Sprintf("duplicate type name: %s (previously defined as %s at %d:%d)", enum.Name, typeNames[enum.Name], existingPos.Line, existingPos.Column),
			})
		} else {
			typeRegistry[enum.Name] = enum.Pos
			typeNames[enum.Name] = "enum"
		}
	}

	// Register all errors and check for duplicate codes
	errorCodes := make(map[int]lexer.Position) // code -> position
	for _, err := range idl.Errors {
		baseName := getBaseName(err.Name)
		if !validateIdentifierName(baseName, errors, err.Pos.Line, err.Pos.Column) {
			continue
		}
		if existingPos, exists := errorCodes[err.Code]; exists {
			errors.Add(&ValidationError{
				Line:   err.Pos.Line,
				Column: err.Pos.Column,
				Msg:    fmt.Sprintf("duplicate error code: %d (previously defined at %d:%d)", err.Code, existingPos.Line, existingPos.Column),
			})
		} else {
			errorCodes[err.Code] = err.Pos
		}

		// Also check for duplicate error names in type registry
		if existingPos, exists := typeRegistry[err.Name]; exists {
			errors.Add(&ValidationError{
				Line:   err.Pos.Line,
				Column: err.Pos.Column,
				Msg:    fmt.Sprintf("duplicate type name: %s (previously defined as %s at %d:%d)", err.Name, typeNames[err.Name], existingPos.Line, existingPos.Column),
			})
		} else {
			typeRegistry[err.Name] = err.Pos
			typeNames[err.Name] = "error"
		}
	}

	// Second pass: validate everything now that all types are registered
	for _, iface := range idl.Interfaces {
		// Validate method names and types
		for _, method := range iface.Methods {
			if !validateIdentifierName(method.Name, errors, method.Pos.Line, method.Pos.Column) {
				continue
			}
			validateType(method.ReturnType, typeRegistry, errors, iface.Namespace)
			for _, param := range method.Parameters {
				if !validateIdentifierName(param.Name, errors, param.Pos.Line, param.Pos.Column) {
					continue
				}
				validateType(param.Type, typeRegistry, errors, iface.Namespace)
			}

			// Validate raises clauses reference existing errors
			for _, raisesName := range method.Raises {
				resolvedName := resolveErrorName(raisesName, typeRegistry, idl.RootNamespace)
				_, exists := typeRegistry[resolvedName]
				if !exists {
					errors.Add(&ValidationError{
						Line:   method.Pos.Line,
						Column: method.Pos.Column,
						Msg:    fmt.Sprintf("method %s raises unknown error: %s", method.Name, raisesName),
					})
				}
			}
		}
	}

	for _, s := range idl.Structs {
		if s.Extends != "" {
			_, exists := typeRegistry[s.Extends]
			if !exists && !builtInTypes[s.Extends] {
				errors.Add(&ValidationError{
					Line:   s.Pos.Line,
					Column: s.Pos.Column,
					Msg:    fmt.Sprintf("struct %s extends unknown type %s", s.Name, s.Extends),
				})
			}
		}
		for _, field := range s.Fields {
			validateType(field.Type, typeRegistry, errors, s.Namespace)
		}
	}

	// Third pass: cycle detection
	detectCycles(idl, errors)

	if errors.HasErrors() {
		return errors
	}

	return nil
}

// validateType validates that a type exists and is well-formed
func validateType(t *Type, typeRegistry map[string]lexer.Position, errors *ValidationErrors, sourceNamespace string) {
	if t == nil {
		errors.Add(&ValidationError{
			Line:   0,
			Column: 0,
			Msg:    "type is nil",
		})
		return
	}

	line := t.Pos.Line
	column := t.Pos.Column

	if t.IsBuiltIn() {
		if !builtInTypes[t.BuiltIn] {
			errors.Add(&ValidationError{
				Line:   line,
				Column: column,
				Msg:    fmt.Sprintf("unknown built-in type: %s", t.BuiltIn),
			})
		}
		return
	}

	if t.IsArray() {
		validateType(t.Array, typeRegistry, errors, sourceNamespace)
		return
	}

	if t.IsMap() {
		// Map keys are always string, so we just validate the value type
		validateType(t.MapValue, typeRegistry, errors, sourceNamespace)
		return
	}

	if t.IsUserDefined() {
		typeName := t.UserDefined
		// Resolve the type name based on the source namespace
		resolvedTypeName := resolveTypeName(typeName, typeRegistry, sourceNamespace)
		if _, exists := typeRegistry[resolvedTypeName]; !exists && !builtInTypes[resolvedTypeName] {
			errors.Add(&ValidationError{
				Line:   line,
				Column: column,
				Msg:    fmt.Sprintf("unknown type: %s", typeName),
			})
		}
		return
	}

	errors.Add(&ValidationError{
		Line:   line,
		Column: column,
		Msg:    "invalid type expression",
	})
}

// validateIdentifierName validates that an identifier matches the naming rules
func validateIdentifierName(name string, errors *ValidationErrors, line, column int) bool {
	if !identifierRegex.MatchString(name) {
		errors.Add(&ValidationError{
			Line:   line,
			Column: column,
			Msg:    fmt.Sprintf("invalid identifier: %s (must start with a letter, followed by letters, numbers, or underscores)", name),
		})
		return false
	}
	return true
}

// getBaseName extracts the base name from a qualified name (e.g., "inc.Response" -> "Response")
func getBaseName(name string) string {
	parts := strings.Split(name, ".")
	return parts[len(parts)-1]
}

// resolveErrorName resolves a potentially-unqualified error name to a fully-qualified name.
// If the name is already qualified (contains a dot), it's returned as-is.
// Otherwise, it attempts to resolve it by checking:
// 1. If the unqualified name exists in the type registry, prefix with currentNamespace
// 2. If currentNamespace.name exists, return that
// 3. Otherwise, return the original name (will fail validation)
// NOTE: Does NOT search across imported namespaces - imported errors must be fully qualified
func resolveErrorName(name string, typeRegistry map[string]lexer.Position, currentNamespace string) string {
	// If already qualified, return as-is
	if strings.Contains(name, ".") {
		return name
	}

	// Check if qualified version exists in current namespace
	qualified := currentNamespace + "." + name
	if _, exists := typeRegistry[qualified]; exists {
		return qualified
	}

	// Not found in current namespace - return as-is (will fail validation with clear error message)
	return name
}

// resolveTypeName resolves a potentially-unqualified type name to a fully-qualified name.
// If the name is already qualified (contains a dot), it's returned as-is.
// Otherwise, it attempts to resolve it by checking:
// 1. If the unqualified name exists in the current namespace, prefix with currentNamespace
// 2. Otherwise, return the original name (will fail validation)
// NOTE: Does NOT search across imported namespaces - imported types must be fully qualified
func resolveTypeName(name string, typeRegistry map[string]lexer.Position, currentNamespace string) string {
	// If already qualified, return as-is
	if strings.Contains(name, ".") {
		return name
	}

	// Check if qualified version exists in current namespace
	qualified := currentNamespace + "." + name
	if _, exists := typeRegistry[qualified]; exists {
		return qualified
	}

	// Not found in current namespace - return as-is (will fail validation with clear error message)
	return name
}

// getReferencedTypes extracts user-defined type names from a Type
func getReferencedTypes(t *Type) []string {
	if t == nil {
		return nil
	}
	if t.IsUserDefined() {
		return []string{t.UserDefined}
	}
	if t.IsArray() {
		return getReferencedTypes(t.Array)
	}
	if t.IsMap() {
		return getReferencedTypes(t.MapValue)
	}
	return nil
}

// detectCycles detects circular type references in structs
func detectCycles(idl *IDL, errors *ValidationErrors) {
	// Build a map of struct name to struct for quick lookup
	structMap := make(map[string]*Struct)
	for _, s := range idl.Structs {
		structMap[s.Name] = s
	}

	// Track visited nodes and recursion stack for cycle detection
	visited := make(map[string]bool)
	recursionStack := make(map[string]bool)

	// DFS function to detect cycles
	var dfs func(structName string, path []string) bool
	dfs = func(structName string, path []string) bool {
		// If we've seen this node in the current path, we have a cycle
		if recursionStack[structName] {
			// Build cycle path string
			cyclePath := ""
			cycleStart := -1
			for i, name := range path {
				if name == structName {
					cycleStart = i
					break
				}
			}
			if cycleStart >= 0 {
				for i := cycleStart; i < len(path); i++ {
					if cyclePath != "" {
						cyclePath += " -> "
					}
					cyclePath += path[i]
				}
				cyclePath += " -> " + structName
			} else {
				cyclePath = structName + " -> ... -> " + structName
			}

			s := structMap[structName]
			if s != nil {
				errors.Add(&ValidationError{
					Line:   s.Pos.Line,
					Column: s.Pos.Column,
					Msg:    fmt.Sprintf("circular type reference detected: %s", cyclePath),
				})
			}
			return true
		}

		// If we've already fully processed this node, skip it
		if visited[structName] {
			return false
		}

		// Mark as being processed in current path
		recursionStack[structName] = true
		path = append(path, structName)

		// Check extends relationship
		s := structMap[structName]
		if s != nil {
			if s.Extends != "" {
				if _, isStruct := structMap[s.Extends]; isStruct {
					if dfs(s.Extends, path) {
						return true
					}
				}
			}

			// Check all fields
			for _, field := range s.Fields {
				refTypes := getReferencedTypes(field.Type)
				for _, refType := range refTypes {
					if _, isStruct := structMap[refType]; isStruct {
						// Optional fields break cycles
						if field.Optional {
							continue
						}
						if dfs(refType, path) {
							return true
						}
					}
				}
			}
		}

		// Remove from recursion stack and mark as visited
		delete(recursionStack, structName)
		visited[structName] = true
		return false
	}

	// Run DFS on all structs
	for _, s := range idl.Structs {
		if !visited[s.Name] {
			dfs(s.Name, []string{})
		}
	}
}

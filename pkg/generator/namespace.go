package generator

import (
	"strings"

	"github.com/coopernurse/pulserpc/pkg/parser"
)

// NamespaceTypes groups all types (structs, enums, interfaces, errors) for a single namespace
type NamespaceTypes struct {
	Structs    []*parser.Struct
	Enums      []*parser.Enum
	Interfaces []*parser.Interface
	Errors     []*parser.ErrorDef
}

// GroupTypesByNamespace groups all types in the IDL by their namespace
func GroupTypesByNamespace(idl *parser.IDL) map[string]*NamespaceTypes {
	namespaceMap := make(map[string]*NamespaceTypes)

	// Helper to get or create NamespaceTypes for a namespace
	getOrCreate := func(ns string) *NamespaceTypes {
		if namespaceMap[ns] == nil {
			namespaceMap[ns] = &NamespaceTypes{
				Structs:    make([]*parser.Struct, 0),
				Enums:      make([]*parser.Enum, 0),
				Interfaces: make([]*parser.Interface, 0),
				Errors:     make([]*parser.ErrorDef, 0),
			}
		}
		return namespaceMap[ns]
	}

	// Group structs by namespace
	for _, s := range idl.Structs {
		ns := GetNamespaceFromType(s.Name, s.Namespace)
		nt := getOrCreate(ns)
		nt.Structs = append(nt.Structs, s)
	}

	// Group enums by namespace
	for _, e := range idl.Enums {
		ns := GetNamespaceFromType(e.Name, e.Namespace)
		nt := getOrCreate(ns)
		nt.Enums = append(nt.Enums, e)
	}

	// Group interfaces by namespace
	for _, i := range idl.Interfaces {
		ns := GetNamespaceFromType(i.Name, i.Namespace)
		nt := getOrCreate(ns)
		nt.Interfaces = append(nt.Interfaces, i)
	}

	// Group errors by namespace
	for _, errDef := range idl.Errors {
		ns := errDef.Namespace
		if ns == "" {
			ns = idl.RootNamespace
		}
		nt := getOrCreate(ns)
		nt.Errors = append(nt.Errors, errDef)
	}

	return namespaceMap
}

// GetNamespaceFromType extracts the namespace from a type name
// It first checks the type's Namespace field, then falls back to extracting from the qualified name
// Examples: "auth.User" -> "auth", "User" (with namespace="auth") -> "auth"
func GetNamespaceFromType(typeName string, namespaceField string) string {
	// If the type has a namespace field, use it
	if namespaceField != "" {
		return namespaceField
	}

	// Otherwise, try to extract from the qualified name (e.g., "auth.User" -> "auth")
	if strings.Contains(typeName, ".") {
		parts := strings.Split(typeName, ".")
		if len(parts) > 1 {
			return strings.Join(parts[:len(parts)-1], ".")
		}
	}

	// If no namespace found, return empty string (shouldn't happen with required namespaces)
	return ""
}

// GetBaseName extracts the base name from a qualified type name
// Examples: "auth.User" -> "User", "inc.Response" -> "Response"
func GetBaseName(typeName string) string {
	if strings.Contains(typeName, ".") {
		parts := strings.Split(typeName, ".")
		return parts[len(parts)-1]
	}
	return typeName
}

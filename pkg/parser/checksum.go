package parser

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
)

func ComputeChecksum(idl *IDL) (string, error) {
	typeRegistry := buildTypeRegistry(idl)

	var sb strings.Builder

	fmt.Fprintf(&sb, "namespace:%s\n", idl.RootNamespace)

	interfaces := make([]*Interface, len(idl.Interfaces))
	copy(interfaces, idl.Interfaces)
	sort.Slice(interfaces, func(i, j int) bool {
		return fqName(interfaces[i].Namespace, interfaces[i].Name) < fqName(interfaces[j].Namespace, interfaces[j].Name)
	})

	for _, iface := range interfaces {
		writeInterface(&sb, iface, typeRegistry)
	}

	structs := make([]*Struct, len(idl.Structs))
	copy(structs, idl.Structs)
	sort.Slice(structs, func(i, j int) bool {
		return fqName(structs[i].Namespace, structs[i].Name) < fqName(structs[j].Namespace, structs[j].Name)
	})

	for _, s := range structs {
		writeStruct(&sb, s, typeRegistry)
	}

	enums := make([]*Enum, len(idl.Enums))
	copy(enums, idl.Enums)
	sort.Slice(enums, func(i, j int) bool {
		return fqName(enums[i].Namespace, enums[i].Name) < fqName(enums[j].Namespace, enums[j].Name)
	})

	for _, e := range enums {
		writeEnum(&sb, e)
	}

	errors := make([]*ErrorDef, len(idl.Errors))
	copy(errors, idl.Errors)
	sort.Slice(errors, func(i, j int) bool {
		return errors[i].Name < errors[j].Name
	})

	for _, err := range errors {
		writeError(&sb, err)
	}

	hash := sha256.Sum256([]byte(sb.String()))
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(hash[:]), nil
}

func fqName(namespace, name string) string {
	if namespace == "" {
		return name
	}
	return namespace + "." + name
}

func buildTypeRegistry(idl *IDL) map[string]string {
	registry := make(map[string]string)

	for _, iface := range idl.Interfaces {
		fqn := fqName(iface.Namespace, iface.Name)
		registry[fqn] = "interface"
	}
	for _, s := range idl.Structs {
		fqn := fqName(s.Namespace, s.Name)
		registry[fqn] = "struct"
	}
	for _, e := range idl.Enums {
		fqn := fqName(e.Namespace, e.Name)
		registry[fqn] = "enum"
	}
	for _, err := range idl.Errors {
		registry[err.Name] = "error"
	}

	return registry
}

func resolveType(typeName string, typeRegistry map[string]string, sourceNamespace string) string {
	if strings.Contains(typeName, ".") {
		return typeName
	}
	qualified := sourceNamespace + "." + typeName
	if _, exists := typeRegistry[qualified]; exists {
		return qualified
	}
	return typeName
}

func writeInterface(sb *strings.Builder, iface *Interface, typeRegistry map[string]string) {
	fmt.Fprintf(sb, "interface:%s\n", fqName(iface.Namespace, iface.Name))

	methods := make([]*Method, len(iface.Methods))
	copy(methods, iface.Methods)
	sort.Slice(methods, func(i, j int) bool {
		return methods[i].Name < methods[j].Name
	})

	for _, m := range methods {
		writeMethod(sb, m, typeRegistry, iface.Namespace)
	}
}

func writeMethod(sb *strings.Builder, m *Method, typeRegistry map[string]string, sourceNamespace string) {
	fmt.Fprintf(sb, "  method:%s\n", m.Name)

	params := make([]*Parameter, len(m.Parameters))
	copy(params, m.Parameters)
	sort.Slice(params, func(i, j int) bool {
		return params[i].Name < params[j].Name
	})

	for _, p := range params {
		fmt.Fprintf(sb, "    param:%s:%s\n", p.Name, writeType(p.Type, typeRegistry, sourceNamespace))
	}

	fmt.Fprintf(sb, "    returns:%s\n", writeType(m.ReturnType, typeRegistry, sourceNamespace))

	if m.ReturnOptional {
		sb.WriteString("    returnOptional:true\n")
	}

	if len(m.Raises) > 0 {
		raises := make([]string, len(m.Raises))
		copy(raises, m.Raises)
		sort.Slice(raises, func(i, j int) bool {
			return raises[i] < raises[j]
		})
		for _, r := range raises {
			fmt.Fprintf(sb, "    raises:%s\n", r)
		}
	}
}

func writeStruct(sb *strings.Builder, s *Struct, typeRegistry map[string]string) {
	fmt.Fprintf(sb, "struct:%s\n", fqName(s.Namespace, s.Name))

	if s.Extends != "" {
		resolved := resolveType(s.Extends, typeRegistry, s.Namespace)
		fmt.Fprintf(sb, "  extends:%s\n", resolved)
	}

	fields := make([]*Field, len(s.Fields))
	copy(fields, s.Fields)
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Name < fields[j].Name
	})

	for _, f := range fields {
		typeStr := writeType(f.Type, typeRegistry, s.Namespace)
		if f.Optional {
			fmt.Fprintf(sb, "  field:%s:%s:optional\n", f.Name, typeStr)
		} else {
			fmt.Fprintf(sb, "  field:%s:%s\n", f.Name, typeStr)
		}
	}
}

func writeEnum(sb *strings.Builder, e *Enum) {
	fmt.Fprintf(sb, "enum:%s\n", fqName(e.Namespace, e.Name))

	values := make([]*EnumValue, len(e.Values))
	copy(values, e.Values)
	sort.Slice(values, func(i, j int) bool {
		return values[i].Name < values[j].Name
	})

	for _, v := range values {
		fmt.Fprintf(sb, "  value:%s\n", v.Name)
	}
}

func writeError(sb *strings.Builder, err *ErrorDef) {
	fmt.Fprintf(sb, "error:%s:%d\n", err.Name, err.Code)
}

func writeType(t *Type, typeRegistry map[string]string, sourceNamespace string) string {
	if t == nil {
		return ""
	}

	if t.BuiltIn != "" {
		return t.BuiltIn
	}

	if t.Array != nil {
		return "[]" + writeType(t.Array, typeRegistry, sourceNamespace)
	}

	if t.MapValue != nil {
		return "map[string]" + writeType(t.MapValue, typeRegistry, sourceNamespace)
	}

	if t.UserDefined != "" {
		return resolveType(t.UserDefined, typeRegistry, sourceNamespace)
	}

	return ""
}

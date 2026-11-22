package generator

import (
	"encoding/json"
	"fmt"

	"github.com/coopernurse/pulserpc/pkg/parser"
)

// MarshalIDLJSON returns the canonical D8 IDL JSON representation shared by
// every generator: rootNamespace, checksum, interfaces, structs, enums, errors.
// The six keys are always present (empty arrays for empty collections), and
// the checksum is computed and persisted on the IDL if it is empty.
func MarshalIDLJSON(idl *parser.IDL) ([]byte, error) {
	checksum, err := parser.ComputeChecksum(idl)
	if err != nil {
		return nil, fmt.Errorf("failed to compute checksum: %w", err)
	}
	idl.Checksum = checksum

	interfaces := idl.Interfaces
	if interfaces == nil {
		interfaces = []*parser.Interface{}
	}
	structs := idl.Structs
	if structs == nil {
		structs = []*parser.Struct{}
	}
	enums := idl.Enums
	if enums == nil {
		enums = []*parser.Enum{}
	}
	errors := idl.Errors
	if errors == nil {
		errors = []*parser.ErrorDef{}
	}

	return json.MarshalIndent(map[string]interface{}{
		"rootNamespace": idl.RootNamespace,
		"interfaces":    interfaces,
		"structs":       structs,
		"enums":         enums,
		"errors":        errors,
		"checksum":      idl.Checksum,
	}, "", "  ")
}

// marshalIDLMetadataGo is retained as an alias for the shared helper.
func marshalIDLMetadataGo(idl *parser.IDL) ([]byte, error) {
	return MarshalIDLJSON(idl)
}

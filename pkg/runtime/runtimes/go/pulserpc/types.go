package pulserpc

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

// EntityType represents the type of IDL entity
type EntityType string

const (
	EntityInterface EntityType = "Interface"
	EntityMethod    EntityType = "Method"
	EntityStruct    EntityType = "Struct"
	EntityField     EntityType = "Field"
	EntityEnum      EntityType = "Enum"
	EntityError     EntityType = "Error"
)

// ChangeType represents the type of change
type ChangeType string

const (
	ChangeAdded    ChangeType = "Added"
	ChangeRemoved  ChangeType = "Removed"
	ChangeModified ChangeType = "Modified"
)

// Direction represents the direction of a delta
type Direction string

const (
	DirectionClientHasMore Direction = "ClientHasMore"
	DirectionClientHasLess Direction = "ClientHasLess"
	DirectionMismatch      Direction = "Mismatch"
)

// Severity represents the severity of a delta
type Severity string

const (
	SeverityError   Severity = "Error"
	SeverityWarning Severity = "Warning"
	SeverityInfo    Severity = "Info"
)

// ContractDelta represents a single difference between client and server IDL
type ContractDelta struct {
	EntityType  EntityType
	EntityName  string
	MemberName  string
	ChangeType  ChangeType
	Direction   Direction
	Severity    Severity
	Description string
}

// VerificationResult contains the result of a contract compatibility verification
type VerificationResult struct {
	Compatible bool
	// Error is non-empty when verification could not be completed (for example,
	// when the server IDL could not be fetched). A non-empty Error means the
	// result is inconclusive and must never be read as a successful verification.
	Error          string
	ServerChecksum string
	ClientChecksum string
	Deltas         []ContractDelta
	Timestamp      time.Time
}

// Deprecated: ComputeChecksum is no longer used for verification.
// Checksums are now computed at code generation time and stored in idl.json.
// Clients read checksums from IDL data via the "checksum" field.
// This function is kept for backward compatibility with existing tests.
func ComputeChecksum(idl interface{}) string {
	data, err := json.Marshal(idl)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

// StructDef represents a struct definition
type StructDef map[string]interface{}

// EnumDef represents an enum definition
type EnumDef map[string]interface{}

// StructMap maps struct names to their definitions
type StructMap map[string]StructDef

// EnumMap maps enum names to their definitions
type EnumMap map[string]EnumDef

// FindStruct finds a struct definition by name.
//
// This package-level helper is kept for callers that only have a raw StructMap.
// Repeated or validation-time lookups should use Contract.FindStruct, which
// resolves in O(1) through an index built once at contract construction.
func FindStruct(structName string, allStructs StructMap) StructDef {
	return newDefinitionIndex(allStructs).resolve(structName)
}

// FindEnum finds an enum definition by name.
//
// See FindStruct: Contract.FindEnum is the O(1) path for repeated lookups.
func FindEnum(enumName string, allEnums EnumMap) EnumDef {
	return newDefinitionIndex(allEnums).resolve(enumName)
}

// GetStructFields recursively resolves struct extends to return all fields (parent + child)
func GetStructFields(structName string, allStructs StructMap) []map[string]interface{} {
	return getStructFields(structName, newDefinitionIndex(allStructs))
}

// getStructFields is the indexed implementation used by the public helper.
// Contract-time validation calls typeResolver.getStructFields instead so the
// resolved field list is reused across requests.
func getStructFields(structName string, structs *definitionIndex[StructDef]) []map[string]interface{} {
	return resolveStructFields(structName, structs, nil)
}

// resolveStructFields walks the `extends` chain and returns parent fields
// followed by child fields, with child definitions overriding parent fields of
// the same name. cache (when non-nil) memoizes every struct in the chain. The
// name index makes overrides O(1) instead of rescanning the accumulated list.
func resolveStructFields(structName string, structs *definitionIndex[StructDef], cache map[string][]map[string]interface{}) []map[string]interface{} {
	if fields, ok := cache[structName]; ok {
		return fields
	}

	structDef := structs.resolve(structName)
	if structDef == nil {
		fields := []map[string]interface{}{}
		if cache != nil {
			cache[structName] = fields
		}
		return fields
	}

	var fields []map[string]interface{}

	// Get parent fields first
	if extends, ok := structDef["extends"].(string); ok && extends != "" {
		fields = append(fields, resolveStructFields(extends, structs, cache)...)
	}

	// Track the first position of each field name so overrides replace the
	// same field the original linear scan would have found.
	fieldIndex := make(map[string]int, len(fields))
	for i, f := range fields {
		if name, ok := f["name"].(string); ok {
			if _, exists := fieldIndex[name]; !exists {
				fieldIndex[name] = i
			}
		}
	}

	// Add child fields (override parent if name conflict)
	if fieldsObj, ok := structDef["fields"]; ok {
		if fieldsList, ok := fieldsObj.([]interface{}); ok {
			for _, fieldObj := range fieldsList {
				if field, ok := fieldObj.(map[string]interface{}); ok {
					if name, ok := field["name"].(string); ok {
						if i, exists := fieldIndex[name]; exists {
							fields[i] = field
						} else {
							fieldIndex[name] = len(fields)
							fields = append(fields, field)
						}
					}
				}
			}
		}
	}

	if cache != nil {
		cache[structName] = fields
	}
	return fields
}

package pulserpc

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
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
	Compatible     bool
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

// FindStruct finds a struct definition by name
// Tries qualified name first, then unqualified name
func FindStruct(structName string, allStructs StructMap) StructDef {
	// Try qualified name first
	if structDef, ok := allStructs[structName]; ok {
		return structDef
	}

	// Try unqualified name (extract base name from qualified name)
	if idx := len(structName) - 1; idx >= 0 {
		for i := idx; i >= 0; i-- {
			if structName[i] == '.' {
				baseName := structName[i+1:]
				if structDef, ok := allStructs[baseName]; ok {
					return structDef
				}
				break
			}
		}
	}

	return nil
}

// FindEnum finds an enum definition by name
// Tries qualified name first, then unqualified name
func FindEnum(enumName string, allEnums EnumMap) EnumDef {
	// Try qualified name first
	if enumDef, ok := allEnums[enumName]; ok {
		return enumDef
	}

	// If enumName contains a dot, extract the base name and try that
	if strings.Contains(enumName, ".") {
		baseName := enumName[strings.LastIndex(enumName, ".")+1:]
		if enumDef, ok := allEnums[baseName]; ok {
			return enumDef
		}
	}

	// Try looking for the enum in allEnums with matching base name
	for key, enumDef := range allEnums {
		if strings.HasSuffix(key, "."+enumName) {
			return enumDef
		}
	}

	return nil
}

// GetStructFields recursively resolves struct extends to return all fields (parent + child)
func GetStructFields(structName string, allStructs StructMap) []map[string]interface{} {
	structDef := FindStruct(structName, allStructs)
	if structDef == nil {
		return []map[string]interface{}{}
	}

	var fields []map[string]interface{}

	// Get parent fields first
	if extends, ok := structDef["extends"].(string); ok && extends != "" {
		parentFields := GetStructFields(extends, allStructs)
		fields = append(fields, parentFields...)
	}

	// Track field names to handle overrides
	fieldNames := make(map[string]bool)
	for _, f := range fields {
		if name, ok := f["name"].(string); ok {
			fieldNames[name] = true
		}
	}

	// Add child fields (override parent if name conflict)
	if fieldsObj, ok := structDef["fields"]; ok {
		if fieldsList, ok := fieldsObj.([]interface{}); ok {
			for _, fieldObj := range fieldsList {
				if field, ok := fieldObj.(map[string]interface{}); ok {
					if name, ok := field["name"].(string); ok {
						if !fieldNames[name] {
							fields = append(fields, field)
							fieldNames[name] = true
						} else {
							// Override parent field
							for i, f := range fields {
								if fName, ok := f["name"].(string); ok && fName == name {
									fields[i] = field
									break
								}
							}
						}
					}
				}
			}
		}
	}

	return fields
}

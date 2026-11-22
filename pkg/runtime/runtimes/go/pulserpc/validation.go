package pulserpc

import (
	"fmt"
	"math"
	"reflect"
)

// ValidateString validates that value is a string
func ValidateString(value interface{}) error {
	if _, ok := value.(string); !ok {
		return fmt.Errorf("expected string, got %T", value)
	}
	return nil
}

// ValidateInt validates that value is an IDL int: a whole JSON number within
// the signed 32-bit range. D1/R25: Go's native int is wider, but the wire
// contract is [-2147483648, 2147483647] in every runtime. Fractional, NaN/Inf,
// bool, and out-of-range values are rejected.
func ValidateInt(value interface{}) error {
	switch v := value.(type) {
	case int:
		if int64(v) < math.MinInt32 || int64(v) > math.MaxInt32 {
			return fmt.Errorf("expected int in signed 32-bit range, got %d", v)
		}
		return nil
	case int64:
		if v < math.MinInt32 || v > math.MaxInt32 {
			return fmt.Errorf("expected int in signed 32-bit range, got %d", v)
		}
		return nil
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("expected int, got non-finite float %v", v)
		}
		if v != math.Trunc(v) {
			return fmt.Errorf("expected int, got float with fractional part: %v", v)
		}
		if v < math.MinInt32 || v > math.MaxInt32 {
			return fmt.Errorf("expected int in signed 32-bit range, got %v", v)
		}
		return nil
	default:
		return fmt.Errorf("expected int, got %T", value)
	}
}

// ValidateFloat validates that value is a float64 or int
func ValidateFloat(value interface{}) error {
	switch value.(type) {
	case float64, int:
		return nil
	default:
		return fmt.Errorf("expected float, got %T", value)
	}
}

// ValidateBool validates that value is a bool
func ValidateBool(value interface{}) error {
	if _, ok := value.(bool); !ok {
		return fmt.Errorf("expected bool, got %T", value)
	}
	return nil
}

// ValidateArray validates that value is an array and each element passes validation
func ValidateArray(value interface{}, elementValidator func(interface{}) error) error {
	// Check if it's a slice
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return fmt.Errorf("expected array, got %T", value)
	}

	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i).Interface()
		if err := elementValidator(elem); err != nil {
			return fmt.Errorf("array element at index %d validation failed: %w", i, err)
		}
	}

	return nil
}

// ValidateMap validates that value is a map (map[string]interface{}) with string keys and validated values
func ValidateMap(value interface{}, valueValidator func(interface{}) error) error {
	dict, ok := value.(map[string]interface{})
	if !ok {
		return fmt.Errorf("expected map, got %T", value)
	}

	for key, val := range dict {
		if err := valueValidator(val); err != nil {
			return fmt.Errorf("map value for key '%s' validation failed: %w", key, err)
		}
		_ = key // key is always string in Go maps
	}

	return nil
}

// ValidateEnum validates that value is a string and matches one of the allowed enum values
func ValidateEnum(value interface{}, enumName string, allowedValues []string) error {
	strValue, ok := value.(string)
	if !ok {
		return fmt.Errorf("expected string for enum %s, got %T", enumName, value)
	}

	for _, allowed := range allowedValues {
		if strValue == allowed {
			return nil
		}
	}

	return fmt.Errorf("invalid value for enum %s: '%s'. Allowed values: %v", enumName, strValue, allowedValues)
}

// typeResolver bundles the contract-owned struct/enum indexes so validation
// resolves user-defined types in O(1) without rebuilding an index per field.
type typeResolver struct {
	structs *definitionIndex[StructDef]
	enums   *definitionIndex[EnumDef]
}

func newTypeResolver(structs StructMap, enums EnumMap) *typeResolver {
	return &typeResolver{
		structs: newDefinitionIndex(structs),
		enums:   newDefinitionIndex(enums),
	}
}

// ValidateStruct validates that value is a map[string]interface{} matching the struct definition
func ValidateStruct(
	value interface{},
	structName string,
	structDef StructDef,
	allStructs StructMap,
	allEnums EnumMap,
) error {
	return validateStruct(value, structName, structDef, newTypeResolver(allStructs, allEnums))
}

// validateStruct is the indexed implementation shared with contract validation.
func validateStruct(
	value interface{},
	structName string,
	structDef StructDef,
	resolver *typeResolver,
) error {
	dict, ok := value.(map[string]interface{})
	if !ok {
		return fmt.Errorf("expected map for struct %s, got %T", structName, value)
	}

	// Get all fields including parent fields
	fields := getStructFields(structName, resolver.structs)

	// Check required fields
	for _, field := range fields {
		fieldName, ok := field["name"].(string)
		if !ok {
			continue
		}

		fieldType, ok := field["type"]
		if !ok {
			continue
		}

		isOptional := false
		if opt, ok := field["optional"].(bool); ok {
			isOptional = opt
		}

		fieldValue, exists := dict[fieldName]
		if !exists {
			if !isOptional {
				return fmt.Errorf("missing required field '%s' in struct %s", fieldName, structName)
			}
		} else {
			if fieldValue == nil {
				if !isOptional {
					return fmt.Errorf("field '%s' in struct %s cannot be nil", fieldName, structName)
				}
			} else {
				// Validate field value
				typeDef, ok := fieldType.(map[string]interface{})
				if !ok {
					return fmt.Errorf("invalid field type definition for field '%s' in struct %s", fieldName, structName)
				}

				if err := validateType(fieldValue, typeDef, resolver, false); err != nil {
					return fmt.Errorf("field '%s' in struct %s validation failed: %w", fieldName, structName, err)
				}
			}
		}
	}

	return nil
}

// ValidateType validates a value against a type definition
func ValidateType(
	value interface{},
	typeDef map[string]interface{},
	allStructs StructMap,
	allEnums EnumMap,
	isOptional bool,
) error {
	return validateType(value, typeDef, newTypeResolver(allStructs, allEnums), isOptional)
}

// validateType is the indexed implementation. The resolver is built once per
// contract (or once per standalone ValidateType call) and reused for nested
// array/map/struct validation, so user-defined lookups stay O(1) per field.
func validateType(
	value interface{},
	typeDef map[string]interface{},
	resolver *typeResolver,
	isOptional bool,
) error {
	// Handle optional types
	if value == nil {
		if isOptional {
			return nil
		}
		return fmt.Errorf("value cannot be nil for non-optional type")
	}

	// Built-in types
	if builtIn, ok := typeDef["builtIn"].(string); ok {
		switch builtIn {
		case "string":
			return ValidateString(value)
		case "int":
			return ValidateInt(value)
		case "float":
			return ValidateFloat(value)
		case "bool":
			return ValidateBool(value)
		default:
			return fmt.Errorf("unknown built-in type: %s", builtIn)
		}
	}

	// Array types
	if arrayObj, ok := typeDef["array"]; ok {
		elementType, ok := arrayObj.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid array type definition")
		}
		elementValidator := func(v interface{}) error {
			return validateType(v, elementType, resolver, false)
		}
		return ValidateArray(value, elementValidator)
	}

	// Map types
	if mapValueObj, ok := typeDef["mapValue"]; ok {
		valueType, ok := mapValueObj.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid map type definition")
		}
		valueValidator := func(v interface{}) error {
			return validateType(v, valueType, resolver, false)
		}
		return ValidateMap(value, valueValidator)
	}

	// User-defined types
	if userDefined, ok := typeDef["userDefined"].(string); ok {
		// Check if it's a struct
		structDef := resolver.structs.resolve(userDefined)
		if structDef != nil {
			return validateStruct(value, userDefined, structDef, resolver)
		}

		// Check if it's an enum
		enumDef := resolver.enums.resolve(userDefined)
		if enumDef != nil {
			// Extract allowed values from enum definition
			valuesObj, ok := enumDef["values"]
			if !ok {
				return fmt.Errorf("invalid enum definition for %s: missing values", userDefined)
			}

			valuesList, ok := valuesObj.([]interface{})
			if !ok {
				return fmt.Errorf("invalid enum definition for %s: values is not a list", userDefined)
			}

			allowedValues := make([]string, 0, len(valuesList))
			for _, valObj := range valuesList {
				if valMap, ok := valObj.(map[string]interface{}); ok {
					if name, ok := valMap["name"].(string); ok {
						allowedValues = append(allowedValues, name)
					}
				}
			}

			return ValidateEnum(value, userDefined, allowedValues)
		}

		return fmt.Errorf("unknown user-defined type: %s", userDefined)
	}

	return fmt.Errorf("invalid type definition: %v", typeDef)
}

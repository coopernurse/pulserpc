package pulserpc

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync"
)

// ValidationError carries a field path and a human-readable message.
type ValidationError struct {
	Path    string
	Message string
}

// Validation errors returned by the accumulating validators. Empty == valid.

func joinPath(parent, child string) string {
	if parent == "" {
		if child == "" {
			return ""
		}
		// top-level field gets a leading dot? Python: param[i].field -> ".field"
		// handled by caller; here just concatenate with dot if parent non-empty
		return child
	}
	if child == "" {
		return parent
	}
	return parent + "." + child
}

// --- accumulating validators (return []ValidationError) ---

func ValidateStringE(value interface{}, path string) []ValidationError {
	if _, ok := value.(string); !ok {
		return []ValidationError{{Path: path, Message: fmt.Sprintf("expected string, got %T", value)}}
	}
	return nil
}

func ValidateIntE(value interface{}, path string) []ValidationError {
	switch v := value.(type) {
	case int:
		return nil
	case int64:
		return nil
	case float64:
		return validateFloatAsIntE(v, path)
	case json.Number:
		if _, ok := exactJSONInt64(v); ok {
			return nil
		}
		return []ValidationError{{Path: path, Message: fmt.Sprintf("expected int in signed 64-bit range, got %s", v)}}
	default:
		return []ValidationError{{Path: path, Message: fmt.Sprintf("expected int, got %T", value)}}
	}
}

// float64 cannot represent math.MaxInt64 exactly: the nearest double is 2^63.
// Accept the representable bounds [-2^63, 2^63) so -2^63 (exact) is valid and
// 2^63 (which would land outside the int64 range) is rejected.
const (
	minInt64AsFloat        = -9223372036854775808.0 // -2^63
	maxInt64FloatExclusive = 9223372036854775808.0  // 2^63
	// maxExactFloatInteger is the largest integer magnitude a float64 can
	// represent exactly (2^53-1). Larger int64 values are kept as
	// json.Number so their exact JSON integer text survives a round trip;
	// float64 marshaling can otherwise rewrite even an exact value such as
	// MinInt64 as -9223372036854776000.
	maxExactFloatInteger = 9007199254740991
)

// intFitsFloatExactly reports whether i can round-trip through float64 and
// JSON as the same integer value and text.
func intFitsFloatExactly(i int64) bool {
	return i >= -maxExactFloatInteger && i <= maxExactFloatInteger
}

// isIntegerFormJSONNumber reports whether a json.Number has no fraction or
// exponent, i.e. it denotes an integer and must not be rounded through float64.
func isIntegerFormJSONNumber(n json.Number) bool {
	return !strings.ContainsAny(n.String(), ".eE")
}

// validateFloatAsIntE checks values already represented as floats.
func validateFloatAsIntE(v float64, path string) []ValidationError {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return []ValidationError{{Path: path, Message: fmt.Sprintf("expected int, got non-finite float %v", v)}}
	}
	if v != math.Trunc(v) {
		return []ValidationError{{Path: path, Message: fmt.Sprintf("expected int, got float with fractional part: %v", v)}}
	}
	if v < minInt64AsFloat || v >= maxInt64FloatExclusive {
		return []ValidationError{{Path: path, Message: fmt.Sprintf("expected int in signed 64-bit range, got %v", v)}}
	}
	return nil
}

func ValidateFloatE(value interface{}, path string) []ValidationError {
	switch v := value.(type) {
	case float64, int, int64:
		return nil
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return []ValidationError{{Path: path, Message: fmt.Sprintf("expected float, got non-numeric value %q", v.String())}}
		}
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return []ValidationError{{Path: path, Message: fmt.Sprintf("expected float, got non-finite value %v", f)}}
		}
		return nil
	default:
		return []ValidationError{{Path: path, Message: fmt.Sprintf("expected float, got %T", value)}}
	}
}

func ValidateBoolE(value interface{}, path string) []ValidationError {
	if _, ok := value.(bool); !ok {
		return []ValidationError{{Path: path, Message: fmt.Sprintf("expected bool, got %T", value)}}
	}
	return nil
}

func ValidateArrayE(value interface{}, elementValidator func(interface{}, string) []ValidationError, path string) []ValidationError {
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return []ValidationError{{Path: path, Message: fmt.Sprintf("expected array, got %T", value)}}
	}
	var errs []ValidationError
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i).Interface()
		elemPath := fmt.Sprintf("%s[%d]", path, i)
		if e := elementValidator(elem, elemPath); e != nil {
			errs = append(errs, e...)
		}
	}
	return errs
}

func ValidateMapE(value interface{}, valueValidator func(interface{}, string) []ValidationError, path string) []ValidationError {
	dict, ok := value.(map[string]interface{})
	if !ok {
		return []ValidationError{{Path: path, Message: fmt.Sprintf("expected map, got %T", value)}}
	}
	var errs []ValidationError
	for key, val := range dict {
		keyPath := fmt.Sprintf("%s[%s]", path, key)
		if e := valueValidator(val, keyPath); e != nil {
			errs = append(errs, e...)
		}
	}
	return errs
}

func ValidateEnumE(value interface{}, enumName string, allowedValues []string, path string) []ValidationError {
	// External callers pass a plain slice, so build the membership set here.
	// The ordered slice is retained for the (unchanged) error message.
	return validateEnumEWithSet(value, enumName, path, newEnumValueSetFromStrings(allowedValues))
}

// validateEnumEWithSet is the O(1) lookup variant used by contract validation,
// where the set is built once when the contract is constructed.
func validateEnumEWithSet(value interface{}, enumName string, path string, set *enumValueSet) []ValidationError {
	// Generated enums are named string types (e.g. `type MathOp string`).
	// A plain type assertion to string rejects those, so accept any value
	// whose underlying kind is string.
	rv := reflect.ValueOf(value)
	if !rv.IsValid() || rv.Kind() != reflect.String {
		return []ValidationError{{Path: path, Message: fmt.Sprintf("expected string for enum %s, got %T", enumName, value)}}
	}
	strValue := rv.String()
	if _, ok := set.members[strValue]; ok {
		return nil
	}
	return []ValidationError{{Path: path, Message: fmt.Sprintf("invalid value for enum %s: '%s'. Allowed values: %v", enumName, strValue, set.allowed)}}
}

// enumValueSet pairs the ordered enum names (for error text) with a set used
// for constant-time membership checks.
type enumValueSet struct {
	allowed []string
	members map[string]struct{}
}

func newEnumValueSet(valuesList []interface{}) *enumValueSet {
	set := &enumValueSet{members: make(map[string]struct{}, len(valuesList))}
	for _, valObj := range valuesList {
		if valMap, ok := valObj.(map[string]interface{}); ok {
			if name, ok := valMap["name"].(string); ok {
				set.allowed = append(set.allowed, name)
				set.members[name] = struct{}{}
			}
		}
	}
	return set
}

func newEnumValueSetFromStrings(values []string) *enumValueSet {
	set := &enumValueSet{allowed: values, members: make(map[string]struct{}, len(values))}
	for _, name := range values {
		set.members[name] = struct{}{}
	}
	return set
}

func enumSetFromDef(def EnumDef) *enumValueSet {
	if def == nil {
		return nil
	}
	valuesList, ok := def["values"].([]interface{})
	if !ok {
		return nil
	}
	return newEnumValueSet(valuesList)
}

type nullFieldPolicy func(structName, fieldName string) bool

func validateStructE(value interface{}, structName string, structDef StructDef, resolver *typeResolver, path string, rejectNull nullFieldPolicy) []ValidationError {
	dict, ok := value.(map[string]interface{})
	if !ok {
		return []ValidationError{{Path: path, Message: fmt.Sprintf("expected map for struct %s, got %T", structName, value)}}
	}

	fields := resolver.getStructFields(structName)
	var errs []ValidationError
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

		fieldPath := path
		if fieldPath == "" {
			fieldPath = "." + fieldName
		} else {
			fieldPath = fieldPath + "." + fieldName
		}

		fieldValue, exists := dict[fieldName]
		if !exists {
			if !isOptional {
				errs = append(errs, ValidationError{Path: fieldPath, Message: fmt.Sprintf("missing required field '%s' in struct %s", fieldName, structName)})
			}
			continue
		}
		if fieldValue == nil {
			if !isOptional || rejectNull != nil && rejectNull(declaredStructName(structDef, structName), fieldName) {
				errs = append(errs, ValidationError{Path: fieldPath, Message: fmt.Sprintf("field '%s' in struct %s cannot be nil", fieldName, structName)})
			}
			continue
		}
		typeDef, ok := fieldType.(map[string]interface{})
		if !ok {
			errs = append(errs, ValidationError{Path: fieldPath, Message: fmt.Sprintf("invalid field type definition for field '%s' in struct %s", fieldName, structName)})
			continue
		}
		sub := validateTypeE(fieldValue, typeDef, resolver, false, fieldPath, rejectNull)
		errs = append(errs, sub...)
	}
	return errs
}

func declaredStructName(def StructDef, fallback string) string {
	name, _ := def["name"].(string)
	if name == "" {
		return fallback
	}
	namespace, _ := def["namespace"].(string)
	if namespace != "" && !strings.Contains(name, ".") {
		return namespace + "." + name
	}
	return name
}

func validateTypeE(value interface{}, typeDef map[string]interface{}, resolver *typeResolver, isOptional bool, path string, rejectNull nullFieldPolicy) []ValidationError {
	if value == nil {
		if isOptional {
			return nil
		}
		return []ValidationError{{Path: path, Message: "value cannot be nil for non-optional type"}}
	}

	if builtIn, ok := typeDef["builtIn"].(string); ok {
		switch builtIn {
		case "string":
			return ValidateStringE(value, path)
		case "int":
			return ValidateIntE(value, path)
		case "float":
			return ValidateFloatE(value, path)
		case "bool":
			return ValidateBoolE(value, path)
		default:
			return []ValidationError{{Path: path, Message: fmt.Sprintf("unknown built-in type: %s", builtIn)}}
		}
	}

	if arrayObj, ok := typeDef["array"]; ok {
		elementType, ok := arrayObj.(map[string]interface{})
		if !ok {
			return []ValidationError{{Path: path, Message: "invalid array type definition"}}
		}
		elementValidator := func(v interface{}, p string) []ValidationError {
			return validateTypeE(v, elementType, resolver, false, p, rejectNull)
		}
		return ValidateArrayE(value, elementValidator, path)
	}

	if mapValueObj, ok := typeDef["mapValue"]; ok {
		valueType, ok := mapValueObj.(map[string]interface{})
		if !ok {
			return []ValidationError{{Path: path, Message: "invalid map type definition"}}
		}
		valueValidator := func(v interface{}, p string) []ValidationError {
			return validateTypeE(v, valueType, resolver, false, p, rejectNull)
		}
		return ValidateMapE(value, valueValidator, path)
	}

	if userDefined, ok := typeDef["userDefined"].(string); ok {
		structDef := resolver.structs.resolve(userDefined)
		if structDef != nil {
			return validateStructE(value, userDefined, structDef, resolver, path, rejectNull)
		}
		enumDef := resolver.enums.resolve(userDefined)
		if enumDef != nil {
			valuesObj, ok := enumDef["values"]
			if !ok {
				return []ValidationError{{Path: path, Message: fmt.Sprintf("invalid enum definition for %s: missing values", userDefined)}}
			}
			valuesList, ok := valuesObj.([]interface{})
			if !ok {
				return []ValidationError{{Path: path, Message: fmt.Sprintf("invalid enum definition for %s: values is not a list", userDefined)}}
			}
			// Shape checks stay here so malformed definitions keep their
			// original error messages; the membership index is cached.
			return validateEnumEWithSet(value, userDefined, path, resolver.enumSetFor(userDefined, valuesList))
		}
		return []ValidationError{{Path: path, Message: fmt.Sprintf("unknown user-defined type: %s", userDefined)}}
	}

	return []ValidationError{{Path: path, Message: fmt.Sprintf("invalid type definition: %v", typeDef)}}
}

// --- backward-compat wrappers returning error ---

func ValidateString(value interface{}) error {
	if e := ValidateStringE(value, ""); len(e) > 0 {
		return fmt.Errorf("%s", e[0].Message)
	}
	return nil
}
func ValidateInt(value interface{}) error {
	if e := ValidateIntE(value, ""); len(e) > 0 {
		return fmt.Errorf("%s", e[0].Message)
	}
	return nil
}
func ValidateFloat(value interface{}) error {
	if e := ValidateFloatE(value, ""); len(e) > 0 {
		return fmt.Errorf("%s", e[0].Message)
	}
	return nil
}
func ValidateBool(value interface{}) error {
	if e := ValidateBoolE(value, ""); len(e) > 0 {
		return fmt.Errorf("%s", e[0].Message)
	}
	return nil
}
func ValidateArray(value interface{}, elementValidator func(interface{}) error) error {
	wrapped := func(v interface{}, p string) []ValidationError {
		if err := elementValidator(v); err != nil {
			return []ValidationError{{Path: p, Message: err.Error()}}
		}
		return nil
	}
	if e := ValidateArrayE(value, wrapped, ""); len(e) > 0 {
		return fmt.Errorf("%s", e[0].Message)
	}
	return nil
}
func ValidateMap(value interface{}, valueValidator func(interface{}) error) error {
	wrapped := func(v interface{}, p string) []ValidationError {
		if err := valueValidator(v); err != nil {
			return []ValidationError{{Path: p, Message: err.Error()}}
		}
		return nil
	}
	if e := ValidateMapE(value, wrapped, ""); len(e) > 0 {
		return fmt.Errorf("%s", e[0].Message)
	}
	return nil
}
func ValidateEnum(value interface{}, enumName string, allowedValues []string) error {
	if e := ValidateEnumE(value, enumName, allowedValues, ""); len(e) > 0 {
		return fmt.Errorf("%s", e[0].Message)
	}
	return nil
}
func ValidateStruct(value interface{}, structName string, structDef StructDef, allStructs StructMap, allEnums EnumMap) error {
	if e := validateStructE(value, structName, structDef, newTypeResolver(allStructs, allEnums), "", nil); len(e) > 0 {
		return fmt.Errorf("%s", e[0].Message)
	}
	return nil
}
func ValidateType(value interface{}, typeDef map[string]interface{}, allStructs StructMap, allEnums EnumMap, isOptional bool) error {
	if e := validateTypeE(value, typeDef, newTypeResolver(allStructs, allEnums), isOptional, "", nil); len(e) > 0 {
		return fmt.Errorf("%s", e[0].Message)
	}
	return nil
}

// Keep exported ValidateStruct/ValidateType with resolver? The above is fine.

// ValidateStruct indexed (exported already exists via ValidateStruct). Add E variant exported? contract uses validateStructE.
// Export validateStructE as ValidateStructE? keep lowercase for package use; contract is same package.
// typeResolver bundles the contract-owned struct/enum indexes and the
// validation metadata derived from them. Metadata is built once at
// construction; the mutex only guards the lazily filled field cache used by
// ad-hoc resolvers (contracts pre-populate the common paths at construction).
type typeResolver struct {
	structs *definitionIndex[StructDef]
	enums   *definitionIndex[EnumDef]

	enumValues   map[string]*enumValueSet
	structFields map[string][]map[string]interface{}
	fieldsMu     sync.RWMutex
}

func newTypeResolver(structs StructMap, enums EnumMap) *typeResolver {
	r := &typeResolver{
		structs:    newDefinitionIndex(structs),
		enums:      newDefinitionIndex(enums),
		enumValues: make(map[string]*enumValueSet, len(enums)*2),
	}

	// Exact keys win, matching definitionIndex.resolve.
	for name, def := range enums {
		if set := enumSetFromDef(def); set != nil {
			r.enumValues[name] = set
		}
	}
	// Base names resolve to the first sorted key sharing that base name; the
	// index already records that deterministic winner.
	for base, entry := range r.enums.byBase {
		if _, isExact := enums[base]; isExact {
			continue
		}
		if _, exists := r.enumValues[base]; exists {
			continue
		}
		if set := enumSetFromDef(entry.def); set != nil {
			r.enumValues[base] = set
		}
	}

	return r
}

// enumSetFor returns the precomputed membership set for a user-defined enum
// name. The values list is only used as a fallback for resolvers built outside
// newTypeResolver; such sets are not cached, so the maps stay read-only after
// construction and validation is safe across goroutines.
func (r *typeResolver) enumSetFor(name string, valuesList []interface{}) *enumValueSet {
	if r != nil {
		if set := r.enumValues[name]; set != nil {
			return set
		}
	}
	return newEnumValueSet(valuesList)
}

// getStructFields returns the cached, fully resolved field list for a struct.
func (r *typeResolver) getStructFields(structName string) []map[string]interface{} {
	if r == nil {
		return getStructFields(structName, nil)
	}
	r.fieldsMu.RLock()
	fields, ok := r.structFields[structName]
	r.fieldsMu.RUnlock()
	if ok {
		return fields
	}

	scratch := make(map[string][]map[string]interface{})
	fields = resolveStructFields(structName, r.structs, scratch)

	r.fieldsMu.Lock()
	if r.structFields == nil {
		r.structFields = make(map[string][]map[string]interface{})
	}
	for name, resolved := range scratch {
		if _, exists := r.structFields[name]; !exists {
			r.structFields[name] = resolved
		}
	}
	if _, exists := r.structFields[structName]; !exists {
		r.structFields[structName] = fields
	}
	r.fieldsMu.Unlock()
	return fields
}

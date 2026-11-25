package pulserpc

import (
	"bytes"
	"encoding"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
)

// FunctionDef represents a function/method definition from the IDL
type FunctionDef map[string]interface{}

// Interface represents an interface from the IDL
type Interface struct {
	Name      string
	Functions map[string]FunctionDef
}

// Contract represents a parsed IDL contract
type Contract struct {
	idlParsed  interface{}
	Interfaces map[string]*Interface
	Structs    StructMap
	Enums      EnumMap
	Meta       map[string]interface{}

	// resolver owns the base-name indexes for Structs and Enums. It is built
	// once here and reused by all validation/FindStruct/FindEnum calls so type
	// resolution stays O(1) instead of scanning the whole map per lookup.
	resolver *typeResolver
}

// NewContract creates a new Contract from parsed IDL data
// Supports both PulseRPC format (dict with interfaces, structs, enums keys)
// and Barrister format (list of items with type field)
func NewContract(idlParsed interface{}) *Contract {
	c := &Contract{
		idlParsed:  idlParsed,
		Interfaces: make(map[string]*Interface),
		Structs:    make(StructMap),
		Enums:      make(EnumMap),
		Meta:       make(map[string]interface{}),
	}

	// Handle both barrister format (list) and PulseRPC format (dict)
	if dict, ok := idlParsed.(map[string]interface{}); ok {
		// PulseRPC format - dict with interfaces, structs, enums keys

		// Parse interfaces
		if interfaces, ok := dict["interfaces"].([]interface{}); ok {
			for _, ifaceData := range interfaces {
				if ifaceDict, ok := ifaceData.(map[string]interface{}); ok {
					iface := &Interface{
						Name:      ifaceDict["name"].(string),
						Functions: make(map[string]FunctionDef),
					}
					if methods, ok := ifaceDict["methods"].([]interface{}); ok {
						for _, method := range methods {
							if methodDict, ok := method.(map[string]interface{}); ok {
								funcName := methodDict["name"].(string)
								iface.Functions[funcName] = methodDict
							}
						}
					}
					c.Interfaces[iface.Name] = iface
				}
			}
		}

		// Parse structs
		if structs, ok := dict["structs"].([]interface{}); ok {
			for _, structData := range structs {
				if structDict, ok := structData.(map[string]interface{}); ok {
					name := structDict["name"].(string)
					c.Structs[name] = structDict
				}
			}
		}

		// Parse enums
		if enums, ok := dict["enums"].([]interface{}); ok {
			for _, enumData := range enums {
				if enumDict, ok := enumData.(map[string]interface{}); ok {
					name := enumDict["name"].(string)
					c.Enums[name] = enumDict
				}
			}
		}
	} else if list, ok := idlParsed.([]interface{}); ok {
		// Barrister format - list of items with type field
		for _, item := range list {
			if itemDict, ok := item.(map[string]interface{}); ok {
				itemType, _ := itemDict["type"].(string)
				switch itemType {
				case "struct":
					if name, ok := itemDict["name"].(string); ok {
						c.Structs[name] = itemDict
					}
				case "enum":
					if name, ok := itemDict["name"].(string); ok {
						c.Enums[name] = itemDict
					}
				case "interface":
					iface := &Interface{
						Name:      itemDict["name"].(string),
						Functions: make(map[string]FunctionDef),
					}
					if methods, ok := itemDict["methods"].([]interface{}); ok {
						for _, method := range methods {
							if methodDict, ok := method.(map[string]interface{}); ok {
								funcName := methodDict["name"].(string)
								iface.Functions[funcName] = methodDict
							}
						}
					}
					c.Interfaces[iface.Name] = iface
				case "meta":
					// Copy metadata
					for key, value := range itemDict {
						if key != "type" {
							c.Meta[key] = value
						}
					}
				}
			}
		}
	}

	// Build the type-resolution indexes once, when the contract is constructed.
	c.resolver = newTypeResolver(c.Structs, c.Enums)

	return c
}

// LoadContractFromFile loads and parses an IDL contract from a JSON file
func LoadContractFromFile(path string) (*Contract, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read idl.json: %w", err)
	}

	var idlParsed interface{}
	if err := json.Unmarshal(data, &idlParsed); err != nil {
		return nil, fmt.Errorf("failed to parse idl.json: %w", err)
	}

	return NewContract(idlParsed), nil
}

// HasInterface checks if an interface exists in the contract
func (c *Contract) HasInterface(ifaceName string) bool {
	_, ok := c.Interfaces[ifaceName]
	return ok
}

// GetInterface returns an interface by name, or nil if not found
func (c *Contract) GetInterface(ifaceName string) *Interface {
	iface, ok := c.Interfaces[ifaceName]
	if !ok {
		return nil
	}
	return iface
}

// GetFunction returns a function definition from an interface
func (i *Interface) GetFunction(funcName string) FunctionDef {
	if fn, ok := i.Functions[funcName]; ok {
		return fn
	}
	return nil
}

// GetIDLParsed returns the raw parsed IDL data
func (c *Contract) GetIDLParsed() interface{} {
	return c.idlParsed
}

// typeResolver returns the contract-owned resolver. Contracts created through
// NewContract always have one; the fallback keeps manually constructed values
// working without mutating shared state.
func (c *Contract) typeResolver() *typeResolver {
	if c != nil && c.resolver != nil {
		return c.resolver
	}
	var structs StructMap
	var enums EnumMap
	if c != nil {
		structs, enums = c.Structs, c.Enums
	}
	return newTypeResolver(structs, enums)
}

// FindStruct resolves a struct by possibly-qualified name in O(1) using the
// index built at contract construction. Exact keys win; an ambiguous base name
// resolves to the first key in sorted order, deterministically.
func (c *Contract) FindStruct(structName string) StructDef {
	return c.typeResolver().structs.resolve(structName)
}

// FindEnum resolves an enum by possibly-qualified name in O(1) using the index
// built at contract construction. Exact keys win; an ambiguous base name
// resolves to the first key in sorted order, deterministically.
func (c *Contract) FindEnum(enumName string) EnumDef {
	return c.typeResolver().enums.resolve(enumName)
}

// ValidateRequest validates request parameters against the IDL
func (c *Contract) ValidateRequest(ifaceName string, funcName string, params []interface{}) error {
	return c.validateRequest(ifaceName, funcName, params, nil)
}

func (c *Contract) validateRequest(ifaceName string, funcName string, params []interface{}, rejectNull nullFieldPolicy) error {
	iface := c.GetInterface(ifaceName)
	if iface == nil {
		return fmt.Errorf("unknown interface: '%s'", ifaceName)
	}

	fn := iface.GetFunction(funcName)
	if fn == nil {
		return fmt.Errorf("%s: unknown function: '%s'", ifaceName, funcName)
	}

	// Get parameter definitions
	paramDefs, ok := fn["parameters"].([]interface{})
	if !ok {
		paramDefs = []interface{}{}
	}

	// Check parameter count
	if len(params) != len(paramDefs) {
		return fmt.Errorf("function '%s.%s' expects %d param(s), %d given",
			ifaceName, funcName, len(paramDefs), len(params))
	}

	// Validate each parameter and accumulate errors
	var allErrors []ValidationError
	for i, paramValue := range params {
		paramDef, ok := paramDefs[i].(map[string]interface{})
		if !ok {
			continue
		}

		paramType, ok := paramDef["type"].(map[string]interface{})
		if !ok {
			continue
		}

		isOptional := false
		if opt, ok := paramDef["optional"].(bool); ok {
			isOptional = opt
		}

		errs := validateTypeE(paramValue, paramType, c.typeResolver(), isOptional, fmt.Sprintf("param[%d]", i), rejectNull)
		allErrors = append(allErrors, errs...)
	}

	if len(allErrors) > 0 {
		vr := NewValidationResult(allErrors)
		return NewRPCErrorWithData(-32602, fmt.Sprintf("function '%s.%s' invalid params", ifaceName, funcName), vr.ToMap())
	}

	return nil
}

// ValidateResponse validates response result against the IDL
func (c *Contract) ValidateResponse(ifaceName string, funcName string, result interface{}) error {
	iface := c.GetInterface(ifaceName)
	if iface == nil {
		return fmt.Errorf("unknown interface: '%s'", ifaceName)
	}

	fn := iface.GetFunction(funcName)
	if fn == nil {
		return fmt.Errorf("%s: unknown function: '%s'", ifaceName, funcName)
	}

	// Check if function has a return type
	returnType, ok := fn["returnType"].(map[string]interface{})
	if !ok {
		// Function returns void/None
		if result != nil {
			return fmt.Errorf("function '%s.%s' invalid response: expected nil, got %v",
				ifaceName, funcName, result)
		}
		return nil
	}
	// A void return is encoded as {"builtIn":"void"}. It must accept nil and
	// reject any result, just like a method with no returnType key.
	if builtIn, ok := returnType["builtIn"].(string); ok && builtIn == "void" {
		if result != nil {
			return fmt.Errorf("function '%s.%s' invalid response: expected nil, got %v",
				ifaceName, funcName, result)
		}
		return nil
	}

	// Convert result to map if it's a struct (for proper validation)
	result, err := convertToMap(result)
	if err != nil {
		return fmt.Errorf("function '%s.%s' invalid response: %w",
			ifaceName, funcName, err)
	}

	// Validate return type
	isOptional := false
	if opt, ok := fn["returnOptional"].(bool); ok {
		isOptional = opt
	}

	errs := validateTypeE(result, returnType, c.typeResolver(), isOptional, "", nil)
	if len(errs) > 0 {
		vr := NewValidationResult(errs)
		return NewRPCErrorWithData(-32603, fmt.Sprintf("function '%s.%s' invalid response", ifaceName, funcName), vr.ToMap())
	}

	return nil
}

// convertToMap converts a Go struct to map[string]interface{} for validation
// If the value is already a map or primitive, it returns it as-is
func convertToMap(value interface{}) (interface{}, error) {
	if value == nil {
		return nil, nil
	}

	// If it's already a map or primitive, return as-is. json.Number is a
	// numeric scalar even though its Go kind is string; validators accept it
	// and it must not be mistaken for a string response.
	switch value.(type) {
	case map[string]interface{}, string, float64, int, int64, bool, []interface{}, json.Number:
		return value, nil
	}

	rv := reflect.ValueOf(value)
	if needsJSONFallback(rv) {
		// Keep the original top-level round trip for custom marshalers,
		// including its "unmarshal into map" error behavior.
		return structViaJSON(value)
	}
	return convertValueToMap(rv)
}

// convertValueToMap normalizes a reflected value to the shape the validators
// expect: maps for structs, []interface{} for slices/arrays, and primitives for
// named scalar types such as generated enums.
func convertValueToMap(rv reflect.Value) (interface{}, error) {
	if !rv.IsValid() {
		return nil, nil
	}
	if needsJSONFallback(rv) {
		return jsonRoundTrip(rv.Interface())
	}

	switch rv.Kind() {
	case reflect.Interface:
		if rv.IsNil() {
			return nil, nil
		}
		return convertValueToMap(rv.Elem())
	case reflect.Pointer:
		if rv.IsNil() {
			return nil, nil
		}
		return convertValueToMap(rv.Elem())
	case reflect.Slice, reflect.Array:
		result := make([]interface{}, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			elem, err := convertValueToMap(rv.Index(i))
			if err != nil {
				return nil, err
			}
			result[i] = elem
		}
		return result, nil
	case reflect.String:
		return rv.String(), nil
	case reflect.Bool:
		return rv.Bool(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// Keep the JSON-shaped float64 view for values that round-trip
		// exactly. Larger int64 values stay int64: validation accepts int64,
		// and an inexact float would turn a valid value into an invalid one.
		i := rv.Int()
		if intFitsFloatExactly(i) {
			return float64(i), nil
		}
		return i, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return float64(rv.Uint()), nil
	case reflect.Float32, reflect.Float64:
		return rv.Float(), nil
	case reflect.Struct:
		if !canConvertStructByReflection(rv.Type(), make(map[reflect.Type]bool)) ||
			typeIsRecursive(rv.Type(), rv.Type(), make(map[reflect.Type]bool)) {
			return jsonRoundTrip(rv.Interface())
		}
		return convertStructToMap(rv)
	default:
		// Typed maps, channels, funcs, etc. went through JSON before; keep
		// that behavior (and its error messages) for these cases.
		return jsonRoundTrip(rv.Interface())
	}
}

var (
	jsonMarshalerType = reflect.TypeOf((*json.Marshaler)(nil)).Elem()
	textMarshalerType = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
)

func typeImplementsMarshaler(t reflect.Type) bool {
	ptr := reflect.PointerTo(t)
	return t.Implements(jsonMarshalerType) || t.Implements(textMarshalerType) ||
		ptr.Implements(jsonMarshalerType) || ptr.Implements(textMarshalerType)
}

// needsJSONFallback detects values whose JSON encoding is not reproducible by
// walking fields (custom marshalers, including pointer-receiver methods on
// addressable values).
func needsJSONFallback(rv reflect.Value) bool {
	if !rv.IsValid() {
		return false
	}
	t := rv.Type()
	if t.Implements(jsonMarshalerType) || t.Implements(textMarshalerType) {
		return true
	}
	return rv.CanAddr() && typeImplementsMarshaler(t)
}

// structViaJSON is the original top-level round trip, kept for custom
// marshalers and other cases reflection cannot safely reproduce.
func structViaJSON(value interface{}) (interface{}, error) {
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal struct to JSON: %w", err)
	}
	var result map[string]interface{}
	decoder := json.NewDecoder(bytes.NewReader(jsonBytes))
	decoder.UseNumber()
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON to map: %w", err)
	}
	normalizeJSONNumbers(result)
	return result, nil
}

// jsonRoundTrip is the narrow fallback for nested values, where the JSON shape
// may be a scalar or array rather than an object.
func jsonRoundTrip(value interface{}) (interface{}, error) {
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal struct to JSON: %w", err)
	}
	var result interface{}
	decoder := json.NewDecoder(bytes.NewReader(jsonBytes))
	decoder.UseNumber()
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON to map: %w", err)
	}
	return normalizeJSONNumbers(result), nil
}

// canConvertStructByReflection reports whether a struct type can be walked
// without changing the shape json.Marshal would produce. Unsupported field
// kinds, exotic tag options and unexported embedded structs take the fallback.
func canConvertStructByReflection(t reflect.Type, visited map[reflect.Type]bool) bool {
	if visited[t] {
		return true
	}
	visited[t] = true
	switch t.Kind() {
	case reflect.Chan, reflect.Func, reflect.Complex64, reflect.Complex128, reflect.UnsafePointer:
		return false
	case reflect.Pointer, reflect.Slice, reflect.Array:
		return canConvertStructByReflection(t.Elem(), visited)
	case reflect.Map:
		return canConvertStructByReflection(t.Key(), visited) && canConvertStructByReflection(t.Elem(), visited)
	case reflect.Struct:
		if typeImplementsMarshaler(t) {
			return true
		}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.PkgPath != "" && !f.Anonymous {
				continue
			}
			if f.Tag.Get("json") == "-" {
				continue
			}
			if _, _, _, ok := parseJSONTagForValidation(f.Tag.Get("json")); !ok {
				return false
			}
			// json.Marshal can promote exported fields of an unexported
			// embedded struct; reflect marks those fields read-only.
			if f.Anonymous && f.PkgPath != "" {
				return false
			}
			if !canConvertStructByReflection(f.Type, visited) {
				return false
			}
		}
		return true
	default:
		return true
	}
}

// typeIsRecursive reports whether target can be reached again from itself,
// in which case a JSON round trip keeps json.Marshal's cycle detection.
func typeIsRecursive(target, t reflect.Type, path map[reflect.Type]bool) bool {
	if t == target && len(path) > 0 {
		return true
	}
	if path[t] {
		return false
	}
	path[t] = true
	defer delete(path, t)
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
		return typeIsRecursive(target, t.Elem(), path)
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.PkgPath != "" && !f.Anonymous {
				continue
			}
			if typeIsRecursive(target, f.Type, path) {
				return true
			}
		}
	}
	return false
}

func parseJSONTagForValidation(tag string) (name string, omitEmpty bool, tagged bool, ok bool) {
	if tag == "" {
		return "", false, false, true
	}
	parts := strings.Split(tag, ",")
	name = parts[0]
	if name != "" && !validJSONTagName(name) {
		return "", false, false, false
	}
	for _, opt := range parts[1:] {
		if opt != "omitempty" {
			// e.g. ",string" or ",omitzero": defer to encoding/json.
			return "", false, false, false
		}
		omitEmpty = true
	}
	return name, omitEmpty, name != "", true
}

func validJSONTagName(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		switch {
		case strings.ContainsRune("!#$%&()*+-./:<=>?@[]^_{|}~ ", c):
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		default:
			return false
		}
	}
	return true
}

// jsonFieldCandidate is a struct field that json.Marshal would emit, together
// with the data needed to apply Go's embedded-field dominance rules.
type jsonFieldCandidate struct {
	name      string
	value     reflect.Value
	omitEmpty bool
	tagged    bool
	depth     int
	index     []int
}

// collectJSONFields walks embedded structs breadth-first, as encoding/json
// does, so promoted fields are flattened with the correct dominance.
func collectJSONFields(v reflect.Value) ([]jsonFieldCandidate, bool) {
	type pending struct {
		typ   reflect.Type
		val   reflect.Value
		index []int
		depth int
	}
	current := []pending{{typ: v.Type(), val: v}}
	visited := map[reflect.Type]bool{}
	var candidates []jsonFieldCandidate

	for len(current) > 0 {
		var next []pending
		nextCount := map[reflect.Type]bool{}
		for _, item := range current {
			if visited[item.typ] {
				continue
			}
			visited[item.typ] = true
			if !item.val.IsValid() { // nil embedded pointer: no promoted fields
				continue
			}
			for i := 0; i < item.typ.NumField(); i++ {
				sf := item.typ.Field(i)
				if sf.Anonymous {
					ft := sf.Type
					if ft.Kind() == reflect.Pointer {
						ft = ft.Elem()
					}
					if !sf.IsExported() {
						if ft.Kind() != reflect.Struct {
							continue
						}
						return nil, false
					}
				} else if !sf.IsExported() {
					continue
				}
				tag := sf.Tag.Get("json")
				if tag == "-" {
					continue
				}
				name, omitEmpty, tagged, ok := parseJSONTagForValidation(tag)
				if !ok {
					return nil, false
				}
				index := make([]int, len(item.index)+1)
				copy(index, item.index)
				index[len(item.index)] = i
				fieldVal := item.val.Field(i)

				ft := sf.Type
				if ft.Name() == "" && ft.Kind() == reflect.Pointer {
					ft = ft.Elem()
				}
				if name != "" || !sf.Anonymous || ft.Kind() != reflect.Struct {
					if name == "" {
						name = sf.Name
					}
					candidates = append(candidates, jsonFieldCandidate{
						name: name, value: fieldVal, omitEmpty: omitEmpty,
						tagged: tagged, depth: item.depth, index: index,
					})
					continue
				}

				// Untagged embedded struct: promote its fields into this one.
				if fieldVal.Kind() == reflect.Pointer {
					if fieldVal.IsNil() {
						continue
					}
					fieldVal = fieldVal.Elem()
				}
				if !nextCount[ft] {
					nextCount[ft] = true
					next = append(next, pending{typ: ft, val: fieldVal, index: index, depth: item.depth + 1})
				}
			}
		}
		current = next
	}
	return candidates, true
}

func selectJSONFields(candidates []jsonFieldCandidate) map[string]jsonFieldCandidate {
	groups := make(map[string][]jsonFieldCandidate)
	for _, c := range candidates {
		groups[c.name] = append(groups[c.name], c)
	}
	selected := make(map[string]jsonFieldCandidate, len(groups))
	for name, group := range groups {
		sort.SliceStable(group, func(i, j int) bool {
			a, b := group[i], group[j]
			if a.depth != b.depth {
				return a.depth < b.depth
			}
			if a.tagged != b.tagged {
				return a.tagged
			}
			return compareFieldIndices(a.index, b.index) < 0
		})
		// Two fields at the same depth with the same tagged status hide each
		// other, exactly as encoding/json's dominantField does.
		if len(group) > 1 && group[0].depth == group[1].depth && group[0].tagged == group[1].tagged {
			continue
		}
		selected[name] = group[0]
	}
	return selected
}

func compareFieldIndices(a, b []int) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	return len(a) - len(b)
}

func convertStructToMap(rv reflect.Value) (interface{}, error) {
	candidates, ok := collectJSONFields(rv)
	if !ok {
		return jsonRoundTrip(rv.Interface())
	}

	result := make(map[string]interface{})
	for name, field := range selectJSONFields(candidates) {
		if field.omitEmpty && isEmptyJSONValue(field.value) {
			continue
		}
		var converted interface{}
		var err error
		// Inside a struct, json.Marshal encodes []byte as base64; the
		// top-level slice short-circuit in convertValueToMap must not.
		if field.value.Kind() == reflect.Slice && field.value.Type().Elem().Kind() == reflect.Uint8 && !needsJSONFallback(field.value) {
			converted, err = jsonRoundTrip(field.value.Interface())
		} else {
			converted, err = convertValueToMap(field.value)
		}
		if err != nil {
			return nil, err
		}
		result[name] = converted
	}
	return result, nil
}

func isEmptyJSONValue(rv reflect.Value) bool {
	switch rv.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map, reflect.String:
		return rv.Len() == 0
	case reflect.Bool:
		return !rv.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return rv.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return rv.Float() == 0
	case reflect.Interface, reflect.Pointer:
		return rv.IsNil()
	default:
		return false // structs are never empty for json:",omitempty"
	}
}

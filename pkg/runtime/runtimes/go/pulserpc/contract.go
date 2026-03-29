package pulserpc

import (
	"encoding/json"
	"fmt"
	"reflect"
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

	return c
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

// ValidateRequest validates request parameters against the IDL
func (c *Contract) ValidateRequest(ifaceName string, funcName string, params []interface{}) error {
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

	// Validate each parameter
	for i, paramValue := range params {
		paramDef, ok := paramDefs[i].(map[string]interface{})
		if !ok {
			continue
		}

		paramName, _ := paramDef["name"].(string)
		paramType, ok := paramDef["type"].(map[string]interface{})
		if !ok {
			continue
		}

		isOptional := false
		if opt, ok := paramDef["optional"].(bool); ok {
			isOptional = opt
		}

		if err := ValidateType(paramValue, paramType, c.Structs, c.Enums, isOptional); err != nil {
			return fmt.Errorf("function '%s.%s' invalid param '%s': %w",
				ifaceName, funcName, paramName, err)
		}
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

	if err := ValidateType(result, returnType, c.Structs, c.Enums, isOptional); err != nil {
		return fmt.Errorf("function '%s.%s' invalid response: %w",
			ifaceName, funcName, err)
	}

	return nil
}

// convertToMap converts a Go struct to map[string]interface{} for validation
// If the value is already a map or primitive, it returns it as-is
func convertToMap(value interface{}) (interface{}, error) {
	if value == nil {
		return nil, nil
	}

	// If it's already a map or primitive, return as-is
	switch value.(type) {
	case map[string]interface{}, string, float64, int, bool, []interface{}:
		return value, nil
	}

	// For pointers, unwrap and try again
	if rv := reflect.ValueOf(value); rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil, nil
		}
		return convertToMap(rv.Elem().Interface())
	}

	// For slices/arrays, convert each element
	if rv := reflect.ValueOf(value); rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
		result := make([]interface{}, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			elem, err := convertToMap(rv.Index(i).Interface())
			if err != nil {
				return nil, err
			}
			result[i] = elem
		}
		return result, nil
	}

	// For structs, convert to map via JSON
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal struct to JSON: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON to map: %w", err)
	}

	return result, nil
}

package pulserpc

import (
	"fmt"
	"reflect"
	"strings"
)

// Server represents a JSON-RPC 2.0 server with handler registration and optional validation
type Server struct {
	contract           *Contract
	handlers           map[string]interface{}
	validateRequests   bool
	validateResponses  bool
}

// NewServer creates a new Server instance
func NewServer(contract *Contract, validateRequests bool, validateResponses bool) *Server {
	return &Server{
		contract:          contract,
		handlers:          make(map[string]interface{}),
		validateRequests:  validateRequests,
		validateResponses: validateResponses,
	}
}

// AddHandler registers a handler instance for an interface
func (s *Server) AddHandler(ifaceName string, handler interface{}) {
	s.handlers[ifaceName] = handler
}

// Call processes a single JSON-RPC request and returns a JSON-RPC response
// Returns nil for notifications (requests without 'id')
func (s *Server) Call(request map[string]interface{}) map[string]interface{} {
	// Validate request format
	if request == nil {
		return s.errorResponse(nil, -32600, "Invalid Request", "Request must be an object")
	}

	// Check JSON-RPC version
	if request["jsonrpc"] != "2.0" {
		return s.errorResponse(request["id"], -32600, "Invalid Request", "jsonrpc version must be '2.0'")
	}

	// Check for method
	method, ok := request["method"].(string)
	if !ok || method == "" {
		return s.errorResponse(request["id"], -32600, "Invalid Request", "Method must be a string")
	}

	// Handle pulserpc-idl request
	if method == "pulserpc-idl" {
		return s.successResponse(request["id"], s.contract.idlParsed)
	}

	// Check for notification (no 'id' means no response expected)
	reqID := request["id"]
	isNotification := reqID == nil

	// Parse method name (e.g., "UserService.getUser")
	ifaceName, funcName, err := parseMethodName(method)
	if err != nil {
		return s.errorResponse(reqID, -32601, "Method not found", err.Error())
	}

	// Look up handler
	handler, exists := s.handlers[ifaceName]
	if !exists {
		return s.errorResponse(reqID, -32601, "Method not found", fmt.Sprintf("Unknown interface: %s", ifaceName))
	}

	// Get function from handler
	handlerValue := reflect.ValueOf(handler)
	funcValue := handlerValue.MethodByName(funcName)
	if !funcValue.IsValid() {
		// Try finding method by name (case-insensitive for Go conventions)
		funcValue = findMethodByName(handlerValue, funcName)
		if !funcValue.IsValid() {
			return s.errorResponse(reqID, -32601, "Method not found", fmt.Sprintf("Unknown method: %s", method))
		}
	}

	// Get params
	params := request["params"]

	// Convert params to []interface{} if it's a list
	var positionalParams []interface{}
	if paramsList, ok := params.([]interface{}); ok {
		positionalParams = paramsList
	} else if params == nil {
		positionalParams = []interface{}{}
	}

	// Validate request using positional params
	if s.validateRequests && len(positionalParams) > 0 {
		if err := s.contract.ValidateRequest(ifaceName, funcName, positionalParams); err != nil {
			return s.errorResponse(reqID, -32602, "Invalid params", err.Error())
		}
	}

	// Convert positional params to named params using IDL signature
	namedParams, err := s.positionalToNamedParams(ifaceName, funcName, positionalParams)
	if err != nil {
		return s.errorResponse(reqID, -32602, "Invalid params", err.Error())
	}

	// Get function definition for type conversion
	fnDef := s.getFunctionDef(ifaceName, funcName)

	// Invoke handler method
	result, err := s.invokeHandler(handlerValue, funcName, namedParams, fnDef)
	if err != nil {
		// Check if it's an RPCError
		if rpcErr, ok := err.(*RPCError); ok {
			return s.errorResponse(reqID, rpcErr.Code, rpcErr.Message, rpcErr.Data)
		}
		return s.errorResponse(reqID, -32603, "Internal error", err.Error())
	}

	// Validate response if validation is enabled
	if s.validateResponses && result != nil {
		if err := s.contract.ValidateResponse(ifaceName, funcName, result); err != nil {
			return s.errorResponse(reqID, -32603, "Internal error", fmt.Sprintf("Response validation failed: %s", err.Error()))
		}
	}

	// Don't respond to notifications
	if isNotification {
		return nil
	}

	return s.successResponse(reqID, result)
}

// errorResponse creates a JSON-RPC error response
func (s *Server) errorResponse(reqID interface{}, code int, message string, data interface{}) map[string]interface{} {
	errObj := map[string]interface{}{
		"code":    code,
		"message": message,
	}
	if data != nil {
		errObj["data"] = data
	}

	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"error":   errObj,
	}
	if reqID != nil {
		response["id"] = reqID
	}
	return response
}

// successResponse creates a JSON-RPC success response
func (s *Server) successResponse(reqID interface{}, result interface{}) map[string]interface{} {
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"result":  result,
	}
	if reqID != nil {
		response["id"] = reqID
	}
	return response
}

// parseMethodName parses "Interface.method" format
func parseMethodName(method string) (ifaceName, funcName string, err error) {
	for i := len(method) - 1; i >= 0; i-- {
		if method[i] == '.' {
			return method[:i], method[i+1:], nil
		}
	}
	return "", "", fmt.Errorf("invalid method name format: %s", method)
}

// findMethodByName finds a method by name (handles snake_case to CamelCase conversion for Go conventions)
func findMethodByName(v reflect.Value, name string) reflect.Value {
	// Try exact match first
	method := v.MethodByName(name)
	if method.IsValid() {
		return method
	}

	// Try with first letter uppercased (Go convention)
	if len(name) > 0 {
		capitalized := string(rune(name[0]-'a'+'A')) + name[1:]
		method = v.MethodByName(capitalized)
		if method.IsValid() {
			return method
		}
	}

	// Try snake_case to CamelCase conversion (e.g., "say_hi" -> "SayHi")
	camelCase := snakeToCamelCase(name)
	if camelCase != name {
		method = v.MethodByName(camelCase)
		if method.IsValid() {
			return method
		}
	}

	return reflect.Value{}
}

// snakeToCamelCase converts snake_case to CamelCase
// Example: "to_repeat" -> "ToRepeat", "say_hi" -> "SayHi"
func snakeToCamelCase(s string) string {
	parts := strings.Split(s, "_")
	result := ""
	for _, part := range parts {
		if len(part) > 0 {
			result += strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return result
}

// getFunctionDef returns the function definition for a given interface and function name
func (s *Server) getFunctionDef(ifaceName, funcName string) FunctionDef {
	if s.contract == nil {
		return nil
	}
	iface := s.contract.GetInterface(ifaceName)
	if iface == nil {
		return nil
	}
	return iface.GetFunction(funcName)
}

// positionalToNamedParams converts positional params to named params using IDL signature
func (s *Server) positionalToNamedParams(ifaceName, funcName string, positionalParams []interface{}) (map[string]interface{}, error) {
	fn := s.getFunctionDef(ifaceName, funcName)

	result := make(map[string]interface{})
	if fn == nil {
		// Without contract info, return positional params as numbered keys
		for i, v := range positionalParams {
			result[fmt.Sprintf("p%d", i)] = v
		}
		return result, nil
	}

	// Get parameter definitions
	paramDefs, ok := fn["parameters"].([]interface{})
	if !ok {
		paramDefs = []interface{}{}
	}

	// Check parameter count
	if len(positionalParams) != len(paramDefs) {
		return nil, fmt.Errorf("parameter count mismatch: expected %d, got %d", len(paramDefs), len(positionalParams))
	}

	// Build named params
	for i, paramValue := range positionalParams {
		if i < len(paramDefs) {
			paramDef, ok := paramDefs[i].(map[string]interface{})
			if !ok {
				result[fmt.Sprintf("p%d", i)] = paramValue
				continue
			}
			paramName, _ := paramDef["name"].(string)
			if paramName == "" {
				result[fmt.Sprintf("p%d", i)] = paramValue
			} else {
				result[paramName] = paramValue
			}
		} else {
			result[fmt.Sprintf("p%d", i)] = paramValue
		}
	}

	return result, nil
}

// invokeHandler invokes a handler method with the given named parameters
// fnDef is the function definition from the contract, used to get param names and types
func (s *Server) invokeHandler(handlerValue reflect.Value, funcName string, params map[string]interface{}, fnDef FunctionDef) (interface{}, error) {
	// Find the method
	method := handlerValue.MethodByName(funcName)
	if !method.IsValid() {
		// Try capitalized
		if len(funcName) > 0 {
			capitalized := string(rune(funcName[0]-'a'+'A')) + funcName[1:]
			method = handlerValue.MethodByName(capitalized)
		}
	}

	if !method.IsValid() {
		// Try with snake_case to CamelCase conversion
		method = findMethodByName(handlerValue, funcName)
	}

	if !method.IsValid() {
		return nil, fmt.Errorf("method not found: %s", funcName)
	}

	// Get method type to determine number of parameters
	methodType := method.Type()
	numIn := methodType.NumIn()

	// Get parameter definitions from function def
	var paramDefs []interface{}
	if fnDef != nil {
		if pd, ok := fnDef["parameters"].([]interface{}); ok {
			paramDefs = pd
		}
	}

	// Build call arguments
	args := make([]reflect.Value, 0, numIn)

	for i := 0; i < numIn; i++ {
		paramType := methodType.In(i)

		// Try to get the parameter by name from paramDefs
		var argValue reflect.Value
		found := false

		if i < len(paramDefs) {
			if pd, ok := paramDefs[i].(map[string]interface{}); ok {
				paramName, _ := pd["name"].(string)
				if paramName != "" {
					if v, ok := params[paramName]; ok {
						argValue = s.convertValueToType(reflect.ValueOf(v), paramType)
						found = true
					}
				}
			}
		}

		if !found {
			// Try all keys in order (for numbered params)
			for _, v := range params {
				if v != nil {
					argValue = s.convertValueToType(reflect.ValueOf(v), paramType)
					found = true
					break
				}
			}
		}

		if !found {
			// Use zero value for the parameter type
			argValue = reflect.Zero(paramType)
		}

		args = append(args, argValue)
	}

	// Call the method
	result := method.Call(args)

	// Check if last result is an error
	if len(result) > 0 {
		if errVal := result[len(result)-1]; errVal.Type().Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			if !errVal.IsNil() {
				return nil, errVal.Interface().(error)
			}
		}
	}

	// Return first result if exists, otherwise nil
	if len(result) > 0 && result[0].IsValid() {
		return result[0].Interface(), nil
	}

	return nil, nil
}

// convertValueToType converts a reflect.Value to the target type
// Handles the common case of []interface{} being used for arrays and maps being used for structs
func (s *Server) convertValueToType(value reflect.Value, targetType reflect.Type) reflect.Value {
	// If already assignable, return as-is
	if value.Type().AssignableTo(targetType) {
		return value
	}

	// Handle []interface{} to typed slice conversion
	if value.Kind() == reflect.Slice && targetType.Kind() == reflect.Slice {
		elemType := targetType.Elem()
		// Check if this is []interface{} to concrete type like []float64
		if value.Type().Elem().Kind() == reflect.Interface {
			// Create new slice of the target element type
			newSlice := reflect.MakeSlice(targetType, value.Len(), value.Len())
			for i := 0; i < value.Len(); i++ {
				elem := value.Index(i)
				if elem.Kind() == reflect.Interface && !elem.IsNil() {
					elem = elem.Elem()
				}
				// Try to convert the element to the target element type
				if elem.Type().AssignableTo(elemType) {
					newSlice.Index(i).Set(elem)
				} else if elem.Type().ConvertibleTo(elemType) {
					newSlice.Index(i).Set(elem.Convert(elemType))
				}
			}
			return newSlice
		}
	}

	// Handle map[string]interface{} to struct conversion
	if value.Kind() == reflect.Map && targetType.Kind() == reflect.Struct {
		return s.convertMapToStruct(value, targetType)
	}

	// Try standard conversion
	if value.Type().ConvertibleTo(targetType) {
		return value.Convert(targetType)
	}

	return value
}

// convertMapToStruct converts a map[string]interface{} to a struct type
// Handles nested structs and embedded types
func (s *Server) convertMapToStruct(mapValue reflect.Value, structType reflect.Type) reflect.Value {
	if mapValue.Kind() != reflect.Map {
		return mapValue
	}

	// Create new struct instance
	result := reflect.New(structType)

	// Get the underlying struct
	elem := result.Elem()

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		fieldName := field.Name

		// Try to find the value in the map using the field name
		mapKey := reflect.ValueOf(fieldName)
		mapVal := mapValue.MapIndex(mapKey)

		if !mapVal.IsValid() {
			// Try with lowercase first letter (Go field naming)
			if len(fieldName) > 0 {
				lowerName := string(rune(fieldName[0]-'A'+'a')) + fieldName[1:]
				mapKey = reflect.ValueOf(lowerName)
				mapVal = mapValue.MapIndex(mapKey)
			}
		}

		if mapVal.IsValid() {
			// Get the actual value if it's an interface
			if mapVal.Kind() == reflect.Interface && !mapVal.IsNil() {
				mapVal = mapVal.Elem()
			}

			fieldType := field.Type
			convertedVal := s.convertValueToType(mapVal, fieldType)

			if convertedVal.Type().AssignableTo(fieldType) {
				elem.Field(i).Set(convertedVal)
			} else if convertedVal.Type().ConvertibleTo(fieldType) {
				elem.Field(i).Set(convertedVal.Convert(fieldType))
			}
		}
	}

	// Return the value, not the pointer
	return elem
}

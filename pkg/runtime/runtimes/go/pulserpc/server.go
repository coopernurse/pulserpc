package pulserpc

import (
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"sort"
	"strings"
)

// internalErrorMessage is the fixed, generic message returned to clients for
// unexpected internal failures. Raw panic/error values must never be reflected
// into a JSON-RPC error's data field: they can contain secrets or stack details
// and are serialized straight to the client by the generated HTTP adapter.
const internalErrorMessage = "Internal error"

// Server represents a JSON-RPC 2.0 server with handler registration and optional validation
type Server struct {
	contract          *Contract
	handlers          map[string]interface{}
	validateRequests  bool
	validateResponses bool
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
// Returns nil for notifications (requests without 'id').
//
// Notification status is determined once, up front, and honored by every
// return path: JSON-RPC notifications must never receive a response, not even
// an error one. Reflection panics caused by malformed request data are also
// contained here so an HTTP handler survives them.
func (s *Server) Call(request map[string]interface{}) (response map[string]interface{}) {
	var reqID interface{}
	if request != nil {
		reqID = request["id"]
	}

	// D2/R24: a request id must be a string, a number, or null. Booleans,
	// objects, and arrays are invalid; reply with -32600 and a null id rather
	// than echoing the malformed value back to the caller.
	if request != nil && !validRequestID(reqID) {
		return s.errorResponse(nil, -32600, "Invalid Request", "id must be a string, number, or null")
	}

	isNotification := request != nil && reqID == nil

	defer func() {
		if r := recover(); r != nil {
			// Log only the panic type for server-side diagnostics. Never format
			// the panic value itself: it may carry secrets or internal details.
			log.Printf("pulserpc: recovered panic of type %T while processing a request", r)
			response = s.respondError(isNotification, reqID, -32603,
				internalErrorMessage, nil)
		}
	}()

	// Validate request format
	if request == nil {
		return s.errorResponse(nil, -32600, "Invalid Request", "Request must be an object")
	}

	// Check JSON-RPC version
	if request["jsonrpc"] != "2.0" {
		return s.respondError(isNotification, reqID, -32600, "Invalid Request", "jsonrpc version must be '2.0'")
	}

	// Check for method
	method, ok := request["method"].(string)
	if !ok || method == "" {
		return s.respondError(isNotification, reqID, -32600, "Invalid Request", "Method must be a string")
	}

	// Handle pulserpc-idl request
	if method == "pulserpc-idl" {
		return s.respondSuccess(isNotification, reqID, s.contract.idlParsed)
	}

	// Parse method name (e.g., "UserService.getUser")
	ifaceName, funcName, err := parseMethodName(method)
	if err != nil {
		return s.respondError(isNotification, reqID, -32601, "Method not found", err.Error())
	}

	// Look up handler
	handler, exists := s.handlers[ifaceName]
	if !exists {
		return s.respondError(isNotification, reqID, -32601, "Method not found", fmt.Sprintf("Unknown interface: %s", ifaceName))
	}

	// Get function from handler
	handlerValue := reflect.ValueOf(handler)
	funcValue := handlerValue.MethodByName(funcName)
	if !funcValue.IsValid() {
		// Try finding method by name (case-insensitive for Go conventions)
		funcValue = findMethodByName(handlerValue, funcName)
		if !funcValue.IsValid() {
			return s.respondError(isNotification, reqID, -32601, "Method not found", fmt.Sprintf("Unknown method: %s", method))
		}
	}

	// Get function definition for param names and type conversion
	fnDef := s.getFunctionDef(ifaceName, funcName)

	// Params may be positional (array) or named (object). Both forms are
	// normalized to a positional list so validation and invocation share one
	// code path and IDL validation is never silently skipped.
	var positionalParams []interface{}
	switch p := request["params"].(type) {
	case []interface{}:
		positionalParams = p
	case map[string]interface{}:
		positionalParams, err = s.namedParamsToPositional(p, fnDef)
		if err != nil {
			return s.respondError(isNotification, reqID, -32602, "Invalid params", err.Error())
		}
	case nil:
		positionalParams = []interface{}{}
	default:
		return s.respondError(isNotification, reqID, -32602, "Invalid params",
			"params must be an array or an object")
	}

	// Validate request using positional params
	if s.validateRequests && s.contract != nil {
		if err := s.contract.ValidateRequest(ifaceName, funcName, positionalParams); err != nil {
			return s.respondError(isNotification, reqID, -32602, "Invalid params", err.Error())
		}
	}

	// Convert positional params to named params using IDL signature
	namedParams, err := s.positionalToNamedParams(ifaceName, funcName, positionalParams)
	if err != nil {
		return s.respondError(isNotification, reqID, -32602, "Invalid params", err.Error())
	}

	// Invoke handler method
	result, err := s.invokeHandler(handlerValue, funcName, namedParams, fnDef)
	if err != nil {
		// Check if it's an RPCError
		if rpcErr, ok := err.(*RPCError); ok {
			return s.respondError(isNotification, reqID, rpcErr.Code, rpcErr.Message, rpcErr.Data)
		}
		// D4/R15: never leak handler error text to the client. Log only the
		// error type server-side: the message itself may persist secrets or
		// PII straight to logs. Return a fixed generic message with no data
		// member.
		log.Printf("pulserpc: handler error for %s: type %T", method, err)
		return s.respondError(isNotification, reqID, -32603, internalErrorMessage, nil)
	}

	// Validate response if validation is enabled. Validation must also run for
	// nil results: a null result is invalid for a required return type.
	if s.validateResponses && s.contract != nil {
		if err := s.contract.ValidateResponse(ifaceName, funcName, result); err != nil {
			// Only the error type is logged: validation messages can include
			// rejected values, which may be secrets or PII.
			log.Printf("pulserpc: response validation failed for %s: type %T", method, err)
			return s.respondError(isNotification, reqID, -32603, internalErrorMessage, nil)
		}
	}

	return s.respondSuccess(isNotification, reqID, result)
}

// respondError suppresses error replies for notifications (no id requested).
func (s *Server) respondError(isNotification bool, reqID interface{}, code int, message string, data interface{}) map[string]interface{} {
	if isNotification {
		return nil
	}
	return s.errorResponse(reqID, code, message, data)
}

// respondSuccess suppresses success replies for notifications.
func (s *Server) respondSuccess(isNotification bool, reqID interface{}, result interface{}) map[string]interface{} {
	if isNotification {
		return nil
	}
	return s.successResponse(reqID, result)
}

// errorResponse creates a JSON-RPC error response. The id is always present so
// an unknown or invalid request id serializes as an explicit null (D2/D3).
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
		"id":      reqID,
	}
	return response
}

// validRequestID reports whether id is a legal JSON-RPC request id: string,
// number, or null. Booleans, objects, and arrays are rejected (R24/D2).
func validRequestID(id interface{}) bool {
	switch id.(type) {
	case nil, string:
		return true
	case bool, map[string]interface{}, []interface{}:
		return false
	case float64, float32, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64, json.Number:
		return true
	default:
		return false
	}
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

// namedParamsToPositional converts object-form (named) params to a positional
// list using the IDL parameter order. Unknown names are rejected so no value is
// silently dropped; omitted params become nil and are caught by validation
// when they are required.
func (s *Server) namedParamsToPositional(named map[string]interface{}, fnDef FunctionDef) ([]interface{}, error) {
	if fnDef == nil {
		// Without a contract there is no declared order; sort keys so the
		// result is at least deterministic.
		keys := make([]string, 0, len(named))
		for key := range named {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		positional := make([]interface{}, 0, len(keys))
		for _, key := range keys {
			positional = append(positional, named[key])
		}
		return positional, nil
	}

	paramDefs, _ := fnDef["parameters"].([]interface{})
	positional := make([]interface{}, len(paramDefs))
	seen := make(map[string]bool, len(named))
	for i, raw := range paramDefs {
		paramDef, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := paramDef["name"].(string)
		if name == "" {
			continue
		}
		if value, ok := named[name]; ok {
			positional[i] = value
			seen[name] = true
		}
	}
	for name := range named {
		if !seen[name] {
			return nil, fmt.Errorf("unknown parameter '%s'", name)
		}
	}
	return positional, nil
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
						argValue = valueToType(v, paramType, s)
						found = true
					}
				}
			}
		}

		if !found {
			// Try all keys in order (for numbered params)
			for _, v := range params {
				if v != nil {
					argValue = valueToType(v, paramType, s)
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

// valueToType converts a request value to the handler parameter type. A nil
// value is valid input for optional parameters and must become the type's zero
// value instead of reaching reflect.Value.Type() on a zero Value (which panics).
func valueToType(value interface{}, targetType reflect.Type, s *Server) reflect.Value {
	if value == nil {
		return reflect.Zero(targetType)
	}
	return s.convertValueToType(reflect.ValueOf(value), targetType)
}

// convertValueToType converts a reflect.Value to the target type
// Handles the common case of []interface{} being used for arrays and maps being used for structs
func (s *Server) convertValueToType(value reflect.Value, targetType reflect.Type) reflect.Value {
	if !value.IsValid() {
		return reflect.Zero(targetType)
	}

	// Unwrap interface values
	if value.Kind() == reflect.Interface {
		if value.IsNil() {
			return reflect.Zero(targetType)
		}
		value = value.Elem()
	}

	// If already assignable, return as-is
	if value.Type().AssignableTo(targetType) {
		return value
	}

	// Handle nil pointer inputs
	if value.Kind() == reflect.Pointer && value.IsNil() {
		return reflect.Zero(targetType)
	}

	// Handle pointer targets (optional structs etc.): allocate and convert.
	if targetType.Kind() == reflect.Pointer {
		elemType := targetType.Elem()
		if value.Kind() == reflect.Pointer && value.IsNil() {
			return reflect.Zero(targetType)
		}
		if value.Kind() == reflect.Map && value.IsNil() {
			return reflect.Zero(targetType)
		}
		if value.Kind() == reflect.Slice && value.IsNil() {
			return reflect.Zero(targetType)
		}
		converted := s.convertValueToType(value, elemType)
		if !converted.IsValid() {
			return reflect.Zero(targetType)
		}
		if !converted.Type().AssignableTo(elemType) && !converted.Type().ConvertibleTo(elemType) {
			return reflect.Zero(targetType)
		}
		ptr := reflect.New(elemType)
		if converted.Type().AssignableTo(elemType) {
			ptr.Elem().Set(converted)
		} else {
			ptr.Elem().Set(converted.Convert(elemType))
		}
		return ptr
	}

	// Handle slice targets
	if value.Kind() == reflect.Slice && targetType.Kind() == reflect.Slice {
		elemType := targetType.Elem()
		newSlice := reflect.MakeSlice(targetType, value.Len(), value.Len())
		for i := 0; i < value.Len(); i++ {
			elem := value.Index(i)
			converted := s.convertValueToType(elem, elemType)
			if !converted.IsValid() {
				continue
			}
			if converted.Type().AssignableTo(elemType) {
				newSlice.Index(i).Set(converted)
			} else if converted.Type().ConvertibleTo(elemType) {
				newSlice.Index(i).Set(converted.Convert(elemType))
			}
		}
		return newSlice
	}

	// Handle map[string]interface{} (or typed map) to struct conversion
	if value.Kind() == reflect.Map && targetType.Kind() == reflect.Struct {
		return s.convertMapToStruct(value, targetType)
	}

	// Handle map targets (map[string]T etc.)
	if value.Kind() == reflect.Map && targetType.Kind() == reflect.Map {
		keyType := targetType.Key()
		elemType := targetType.Elem()
		newMap := reflect.MakeMap(targetType)
		iter := value.MapRange()
		for iter.Next() {
			k := iter.Key()
			v := iter.Value()
			if k.Kind() == reflect.Interface && !k.IsNil() {
				k = k.Elem()
			}
			if v.Kind() == reflect.Interface && !v.IsNil() {
				v = v.Elem()
			}
			convertedK := k
			if !k.Type().AssignableTo(keyType) {
				if k.Type().ConvertibleTo(keyType) {
					convertedK = k.Convert(keyType)
				} else {
					convertedK = s.convertValueToType(k, keyType)
				}
			}
			convertedV := s.convertValueToType(v, elemType)
			if !convertedV.IsValid() {
				continue
			}
			if !convertedV.Type().AssignableTo(elemType) {
				if convertedV.Type().ConvertibleTo(elemType) {
					convertedV = convertedV.Convert(elemType)
				} else {
					continue
				}
			}
			if !convertedK.IsValid() || !convertedK.Type().AssignableTo(keyType) {
				if convertedK.Type().ConvertibleTo(keyType) {
					convertedK = convertedK.Convert(keyType)
				} else {
					continue
				}
			}
			newMap.SetMapIndex(convertedK, convertedV)
		}
		return newMap
	}

	// Try standard conversion
	if value.Type().ConvertibleTo(targetType) {
		return value.Convert(targetType)
	}

	return value
}

// convertMapToStruct converts a map[string]interface{} to a struct type
// Handles nested structs, slices, maps, embedded types, and json tags.
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
		if !elem.Field(i).CanSet() {
			continue
		}
		fieldType := field.Type

		// Build candidate keys for this field.
		candidates := []string{}
		if tag := field.Tag.Get("json"); tag != "" {
			name := strings.SplitN(tag, ",", 2)[0]
			if name != "" && name != "-" {
				candidates = append(candidates, name)
			}
		}
		candidates = append(candidates, field.Name)
		if len(field.Name) > 0 {
			candidates = append(candidates, string(rune(field.Name[0]-'A'+'a'))+field.Name[1:])
		}
		candidates = append(candidates, strings.ToLower(field.Name))

		var mapVal reflect.Value
		found := false
		for _, c := range candidates {
			if c == "" {
				continue
			}
			mv := mapValue.MapIndex(reflect.ValueOf(c))
			if mv.IsValid() {
				mapVal = mv
				found = true
				break
			}
		}

		// Embedded (anonymous) struct/pointer fields: if no direct key matched,
		// convert from the same map so promoted fields get populated.
		if !found && field.Anonymous {
			switch fieldType.Kind() {
			case reflect.Struct:
				converted := s.convertMapToStruct(mapValue, fieldType)
				elem.Field(i).Set(converted)
				continue
			case reflect.Pointer:
				if fieldType.Elem().Kind() == reflect.Struct {
					// Only flatten if the map has at least one matching promoted field.
					converted := s.convertMapToStruct(mapValue, fieldType.Elem())
					// If all fields zero, still allocate? Keep nil for optional with no data.
					allZero := true
					for j := 0; j < fieldType.Elem().NumField(); j++ {
						if !converted.Field(j).IsZero() {
							allZero = false
							break
						}
					}
					if !allZero {
						ptr := reflect.New(fieldType.Elem())
						ptr.Elem().Set(converted)
						elem.Field(i).Set(ptr)
					}
					continue
				}
			}
		}

		if !found {
			continue
		}

		// Unwrap interface
		if mapVal.Kind() == reflect.Interface {
			if mapVal.IsNil() {
				// null value
				elem.Field(i).Set(reflect.Zero(fieldType))
				continue
			}
			mapVal = mapVal.Elem()
		}

		convertedVal := s.convertValueToType(mapVal, fieldType)

		if convertedVal.Type().AssignableTo(fieldType) {
			elem.Field(i).Set(convertedVal)
		} else if convertedVal.Type().ConvertibleTo(fieldType) {
			elem.Field(i).Set(convertedVal.Convert(fieldType))
		}
	}

	// Return the value, not the pointer
	return elem
}

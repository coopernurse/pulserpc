package pulserpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// internalErrorMessage is the fixed, generic message returned to clients for
// unexpected internal failures. Raw panic/error values must never be reflected
// into a JSON-RPC error's data field: they can contain secrets or stack details
// and are serialized straight to the client by the generated HTTP adapter.
const internalErrorMessage = "Internal error"

// contextType is the interface type every generated handler method must accept
// as its first parameter.
var contextType = reflect.TypeOf((*context.Context)(nil)).Elem()

// errorType is the built-in error interface returned by every generated
// handler method.
var errorType = reflect.TypeOf((*error)(nil)).Elem()

// Middleware is an RPC middleware hook. It runs once per JSON-RPC request
// (per batch member), after envelope validation and before pulserpc-idl
// handling, parameter validation, and dispatch.
//
// The response argument is always nil on entry. Returning a non-nil response
// short-circuits dispatch and is sent as the final JSON-RPC response (it is
// discarded for notifications). Returning a non-nil context or request replaces
// the value used for dispatch.
type Middleware func(
	ctx context.Context,
	request map[string]interface{},
	response map[string]interface{},
) (context.Context, map[string]interface{}, map[string]interface{})

// Server represents a JSON-RPC 2.0 server with handler registration and optional validation
type Server struct {
	contract          *Contract
	handlers          map[string]interface{}
	middleware        []Middleware
	validateRequests  bool
	validateResponses bool
	config            ServerConfig
}

// NewServer creates a new Server instance with the default server policy.
func NewServer(contract *Contract, validateRequests bool, validateResponses bool) *Server {
	return NewServerWithConfig(contract, validateRequests, validateResponses, DefaultServerConfig())
}

// NewServerWithConfig creates a new Server instance with an explicit policy.
func NewServerWithConfig(contract *Contract, validateRequests bool, validateResponses bool, config ServerConfig) *Server {
	return &Server{
		contract:          contract,
		handlers:          make(map[string]interface{}),
		middleware:        nil,
		validateRequests:  validateRequests,
		validateResponses: validateResponses,
		config:            config,
	}
}

// Config returns a copy of the server policy.
func (s *Server) Config() ServerConfig {
	return s.config
}

// rejectNullID applies the RequireNonNullID policy. It returns a response only
// when the policy is enabled and reqID is nil.
func (s *Server) rejectNullID(reqID interface{}) map[string]interface{} {
	if s.config.RequireNonNullID && reqID == nil {
		return s.errorResponse(nil, -32600, "Invalid Request", "id must not be null")
	}
	return nil
}

// NormalizeRequestNumbers converts numbers inside a decoded request to
// float64, matching the pre-UseNumber middleware and handler view, while
// preserving the top-level request id as json.Number so it can be echoed
// exactly. Generated HTTP adapters call it after decoding the body. Batch
// arrays are walked member by member, so each member's id is preserved.
func NormalizeRequestNumbers(request interface{}) {
	switch r := request.(type) {
	case map[string]interface{}:
		for key, value := range r {
			if key == "id" {
				continue
			}
			r[key] = normalizeJSONNumbers(value)
		}
	case []interface{}:
		for _, item := range r {
			NormalizeRequestNumbers(item)
		}
	}
}

// paramError is an internal error that carries the ErrorPhase a params failure
// belongs to so the response path can consult ServerConfig.MapError.
type paramError struct {
	phase ErrorPhase
	err   error
}

func (e *paramError) Error() string { return e.err.Error() }

// paramErrorResponse tries the method-aware mapper before MapError. If neither
// maps the failure, pass through an RPCError or return -32602 with error data.
func (s *Server) paramErrorResponse(isNotification bool, reqID interface{}, method string, phase ErrorPhase, err error) map[string]interface{} {
	if s.config.MapMethodError != nil {
		if mapped := s.config.MapMethodError(method, phase, err); mapped != nil {
			return s.respondError(isNotification, reqID, mapped.Code, mapped.Message, mapped.Data)
		}
	}
	if s.config.MapError != nil {
		if mapped := s.config.MapError(phase, err); mapped != nil {
			return s.respondError(isNotification, reqID, mapped.Code, mapped.Message, mapped.Data)
		}
	}
	if rpcErr, ok := err.(*RPCError); ok {
		return s.respondError(isNotification, reqID, rpcErr.Code, rpcErr.Message, rpcErr.Data)
	}
	return s.respondError(isNotification, reqID, -32602, "Invalid params", err.Error())
}

// UseRPC registers an RPC middleware hook. Hooks run in registration order.
func (s *Server) UseRPC(mw Middleware) {
	if mw != nil {
		s.middleware = append(s.middleware, mw)
	}
}

// AddHandler registers a handler instance for an interface.
//
// Every handler method that implements a method of the registered interface
// must accept a leading context.Context and return the declared error shape:
// (T, error) for value-returning methods and error for void methods. Methods
// unrelated to the interface are ignored. A rejected handler is not stored.
// When the server has no contract (or the contract does not declare the
// interface), every exported handler method is validated instead.
func (s *Server) AddHandler(ifaceName string, handler interface{}) error {
	if handler == nil {
		return fmt.Errorf("handler for interface %s is nil", ifaceName)
	}
	if err := s.validateHandler(ifaceName, handler); err != nil {
		return err
	}
	s.handlers[ifaceName] = handler
	return nil
}

// handlerShape describes the declared return shape of an interface method.
type handlerShape int

const (
	// handlerShapeAny accepts either (T, error) or error. It is used when no
	// IDL contract is available to distinguish value-returning methods.
	handlerShapeAny handlerShape = iota
	handlerShapeValue
	handlerShapeVoid
)

// validateHandler checks a handler against the registered interface methods.
func (s *Server) validateHandler(ifaceName string, handler interface{}) error {
	handlerValue := reflect.ValueOf(handler)

	if s.contract == nil {
		// Without an IDL contract there is no declared shape to compare
		// against; still require the context/error convention on every
		// exported method so a misconfigured handler fails at startup.
		for i := 0; i < handlerValue.NumMethod(); i++ {
			name := handlerValue.Type().Method(i).Name
			// handlerValue.Method(i) is bound, so its type does not include
			// the receiver.
			if err := validateHandlerMethod(ifaceName, name, handlerValue.Method(i).Type(), handlerShapeAny); err != nil {
				return err
			}
		}
		return nil
	}

	iface := s.contract.GetInterface(ifaceName)
	if iface == nil {
		return fmt.Errorf("interface %s is not declared in the contract", ifaceName)
	}

	methodNames := make([]string, 0, len(iface.Functions))
	for name := range iface.Functions {
		methodNames = append(methodNames, name)
	}
	sort.Strings(methodNames)
	for _, name := range methodNames {
		methodType, ok := findHandlerMethodType(handlerValue, name)
		if !ok {
			return fmt.Errorf("handler for interface %s is missing method %s", ifaceName, name)
		}
		if err := validateHandlerMethod(ifaceName, name, methodType, shapeForFunction(iface.Functions[name])); err != nil {
			return err
		}
	}
	return nil
}

// shapeForFunction derives the declared return shape from IDL metadata. A
// missing return type or a void built-in means the method returns only error.
func shapeForFunction(fnDef FunctionDef) handlerShape {
	if fnDef == nil {
		return handlerShapeAny
	}
	returnType, ok := fnDef["returnType"].(map[string]interface{})
	if !ok {
		return handlerShapeVoid
	}
	if builtIn, ok := returnType["builtIn"].(string); ok && builtIn == "void" {
		return handlerShapeVoid
	}
	return handlerShapeValue
}

// findHandlerMethodType resolves the Go method for an IDL method name using the
// same name fallbacks as dispatch.
func findHandlerMethodType(handlerValue reflect.Value, name string) (reflect.Type, bool) {
	method := handlerValue.MethodByName(name)
	if !method.IsValid() {
		method = findMethodByName(handlerValue, name)
	}
	if !method.IsValid() {
		return nil, false
	}
	return method.Type(), true
}

// validateHandlerMethod enforces the leading-context and error-result contract.
func validateHandlerMethod(ifaceName, methodName string, methodType reflect.Type, shape handlerShape) error {
	if methodType.NumIn() == 0 || methodType.In(0) != contextType {
		return fmt.Errorf("handler method %s.%s must accept context.Context as its first parameter", ifaceName, methodName)
	}

	numOut := methodType.NumOut()
	if numOut == 0 || methodType.Out(numOut-1) != errorType {
		return fmt.Errorf("handler method %s.%s must return error as its last result", ifaceName, methodName)
	}

	switch shape {
	case handlerShapeVoid:
		if numOut != 1 {
			return fmt.Errorf("handler method %s.%s must return exactly error for a void method", ifaceName, methodName)
		}
	case handlerShapeValue:
		if numOut != 2 {
			return fmt.Errorf("handler method %s.%s must return (value, error)", ifaceName, methodName)
		}
	case handlerShapeAny:
		if numOut > 2 {
			return fmt.Errorf("handler method %s.%s must return error or (value, error)", ifaceName, methodName)
		}
	}
	return nil
}

// Call processes a single JSON-RPC request with a background context and
// returns a JSON-RPC response. It is the convenience form of CallWithContext.
// Returns nil for notifications (requests without 'id').
func (s *Server) Call(request map[string]interface{}) map[string]interface{} {
	return s.CallWithContext(context.Background(), request)
}

// CallWithContext processes a single JSON-RPC request with the supplied context
// and returns a JSON-RPC response. The context is passed to every handler
// method; a nil context is treated as context.Background().
//
// Returns nil for notifications (requests without 'id').
//
// Notification status is determined once, up front, and honored by every
// return path: JSON-RPC notifications must never receive a response, not even
// an error one. Reflection panics caused by malformed request data are also
// contained here so an HTTP handler survives them.
func (s *Server) CallWithContext(ctx context.Context, request map[string]interface{}) (response map[string]interface{}) {
	if ctx == nil {
		ctx = context.Background()
	}

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

	// A non-null id policy turns what would otherwise be a notification into an
	// invalid request. errorResponse is used directly (not respondError) so the
	// error is written even though the id is absent.
	if response := s.rejectNullID(reqID); response != nil {
		return response
	}

	// Check JSON-RPC version. A request with an invalid (or missing) version is
	// not a valid JSON-RPC 2.0 request, so its id cannot be trusted: reply with
	// a null id rather than echoing the caller-supplied one. respondError still
	// keeps notifications silent.
	if request["jsonrpc"] != "2.0" {
		return s.respondError(isNotification, nil, -32600, "Invalid Request", "jsonrpc version must be '2.0'")
	}

	// Check for method
	method, ok := request["method"].(string)
	if !ok || method == "" {
		return s.respondError(isNotification, nil, -32600, "Invalid Request", "Method must be a string")
	}

	// Run RPC middleware once per request, after envelope validation and
	// before pulserpc-idl handling, parameter validation, and dispatch. A
	// non-nil response short-circuits dispatch.
	ctx, request, shortCircuit := s.applyMiddleware(ctx, request)
	if shortCircuit != nil {
		if isNotification {
			return nil
		}
		return shortCircuit
	}

	// Middleware may have rewritten the request. Re-read the envelope so a
	// rewriting hook still controls dispatch and so the response id and
	// notification status describe the request that is actually dispatched.
	if request == nil {
		return s.errorResponse(nil, -32600, "Invalid Request", "Request must be an object")
	}
	reqID = request["id"]
	if !validRequestID(reqID) {
		return s.errorResponse(nil, -32600, "Invalid Request", "id must be a string, number, or null")
	}
	isNotification = reqID == nil
	if response := s.rejectNullID(reqID); response != nil {
		return response
	}
	if request["jsonrpc"] != "2.0" {
		return s.respondError(isNotification, nil, -32600, "Invalid Request", "jsonrpc version must be '2.0'")
	}
	method, ok = request["method"].(string)
	if !ok || method == "" {
		return s.respondError(isNotification, nil, -32600, "Invalid Request", "Method must be a string")
	}

	// Handle pulserpc-idl request. Without a contract there is no IDL to serve;
	// report an internal error instead of dereferencing a nil contract.
	if method == "pulserpc-idl" {
		if s.contract == nil {
			return s.respondError(isNotification, reqID, -32603, internalErrorMessage, nil)
		}
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
		// Named params are an envelope-level protocol shape decision, so they
		// are never routed through MapError.
		if s.config.ParamsMode == ParamsModePositionalOnly {
			return s.respondError(isNotification, reqID, -32600, "Invalid Request",
				"named params are not allowed")
		}
		positionalParams, err = s.namedParamsToPositional(p, fnDef)
		if err != nil {
			return s.paramErrorResponse(isNotification, reqID, method, ErrorPhaseDecodeParams, err)
		}
	case nil:
		positionalParams = []interface{}{}
	default:
		return s.paramErrorResponse(isNotification, reqID, method, ErrorPhaseDecodeParams,
			fmt.Errorf("params must be an array or an object"))
	}

	// A parameter-count mismatch is a decode failure, not a value-validation
	// failure. Check it before Contract.ValidateRequest so MapError receives
	// ErrorPhaseDecodeParams; the message matches the contract validator's.
	if fnDef != nil {
		paramDefs, _ := fnDef["parameters"].([]interface{})
		if len(positionalParams) != len(paramDefs) {
			return s.paramErrorResponse(isNotification, reqID, method, ErrorPhaseDecodeParams,
				fmt.Errorf("function '%s.%s' expects %d param(s), %d given",
					ifaceName, funcName, len(paramDefs), len(positionalParams)))
		}
	}

	// Validate request using positional params
	if s.validateRequests && s.contract != nil {
		if err := s.contract.ValidateRequest(ifaceName, funcName, positionalParams); err != nil {
			return s.paramErrorResponse(isNotification, reqID, method, ErrorPhaseValidateParams, err)
		}
	}

	// Convert positional params to named params using IDL signature
	namedParams, err := s.positionalToNamedParams(ifaceName, funcName, positionalParams)
	if err != nil {
		return s.paramErrorResponse(isNotification, reqID, method, ErrorPhaseDecodeParams, err)
	}

	// Invoke handler method with the reflect.Value resolved above.
	result, err := s.invokeHandler(ctx, handlerValue, funcName, funcValue, namedParams, fnDef)
	if err != nil {
		// Parameter decode failures (unknown request fields) carry their phase
		// so MapError can see them.
		if pe, ok := err.(*paramError); ok {
			return s.paramErrorResponse(isNotification, reqID, method, pe.phase, pe.err)
		}
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

// applyMiddleware runs the registered RPC hooks in order. A non-nil context or
// request returned by a hook replaces the value passed to the next hook. The
// first non-nil response short-circuits the chain.
func (s *Server) applyMiddleware(ctx context.Context, request map[string]interface{}) (context.Context, map[string]interface{}, map[string]interface{}) {
	for _, mw := range s.middleware {
		nextCtx, nextRequest, shortCircuit := mw(ctx, request, nil)
		if nextCtx != nil {
			ctx = nextCtx
		}
		if nextRequest != nil {
			request = nextRequest
		}
		if shortCircuit != nil {
			return ctx, request, shortCircuit
		}
	}
	return ctx, request, nil
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

// invokeHandler invokes a handler method with the given context and named
// parameters. fnDef is the function definition from the contract, used to get
// param names and types; resolved is the method value already found by Call;
// fallbacks only run when it is invalid.
//
// A leading context.Context parameter is supplied from ctx and is never part of
// the wire contract: IDL parameter metadata is offset by one when present, and
// arity/validation stay IDL-only. A handler method without a leading context is
// rejected defensively; registration should make this unreachable.
func (s *Server) invokeHandler(ctx context.Context, handlerValue reflect.Value, funcName string, resolved reflect.Value, params map[string]interface{}, fnDef FunctionDef) (interface{}, error) {
	method := resolved
	if !method.IsValid() {
		// Covers capitalized and snake_case variants.
		method = findMethodByName(handlerValue, funcName)
	}
	if !method.IsValid() {
		return nil, fmt.Errorf("method not found: %s", funcName)
	}

	// Get method type to determine number of parameters
	methodType := method.Type()
	numIn := methodType.NumIn()

	hasContext := numIn > 0 && methodType.In(0) == contextType
	if !hasContext {
		return nil, fmt.Errorf("handler method %s does not accept a leading context.Context", funcName)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// Get parameter definitions from function def
	var paramDefs []interface{}
	if fnDef != nil {
		if pd, ok := fnDef["parameters"].([]interface{}); ok {
			paramDefs = pd
		}
	}

	// Build call arguments. Index 0 is always the context; IDL parameters start
	// at index 1 and their metadata is offset accordingly.
	args := make([]reflect.Value, 0, numIn)
	args = append(args, reflect.ValueOf(ctx))
	paramOffset := 1

	for i := paramOffset; i < numIn; i++ {
		paramType := methodType.In(i)

		// Try to get the parameter by name from paramDefs
		var rawValue interface{}
		found := false

		defIndex := i - paramOffset
		if defIndex < len(paramDefs) {
			if pd, ok := paramDefs[defIndex].(map[string]interface{}); ok {
				paramName, _ := pd["name"].(string)
				if paramName != "" {
					if v, ok := params[paramName]; ok {
						rawValue = v
						found = true
					}
				}
			}
		}

		if !found {
			// Try all keys in order (for numbered params)
			for _, v := range params {
				if v != nil {
					rawValue = v
					found = true
					break
				}
			}
		}

		// Strict mode rejects keys that would be silently dropped by
		// convertMapToStruct. The check runs before conversion so the handler
		// never observes a partially populated struct. It is a decode failure:
		// the payload shape does not match the handler type.
		if found && rawValue != nil && s.config.RejectUnknownRequestFields {
			if err := checkUnknownRequestFields(rawValue, paramType, fmt.Sprintf("param[%d]", defIndex)); err != nil {
				return nil, &paramError{phase: ErrorPhaseDecodeParams, err: err}
			}
		}

		var argValue reflect.Value
		if found {
			argValue = valueToType(rawValue, paramType, s)
		} else {
			// Use zero value for the parameter type
			argValue = reflect.Zero(paramType)
		}

		args = append(args, argValue)
	}

	// Call the method
	result := method.Call(args)

	// Check if last result is an error
	if len(result) > 0 {
		if errVal := result[len(result)-1]; errVal.Type().Implements(errorType) {
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

	// json.Number has string kind. Special-case numeric targets before the
	// generic conversion paths so a decoder-read number never silently becomes
	// a string (or an unusable value) when the handler wants an int or float.
	if num, ok := value.Interface().(json.Number); ok {
		if converted, ok := convertJSONNumber(num, targetType); ok {
			return converted
		}
		// A JSON number must not fall through to the generic ConvertibleTo
		// path: json.Number has string kind, so a number could otherwise
		// silently populate a string field. Use the target's zero value.
		return reflect.Zero(targetType)
	}

	// Handle nil pointer inputs
	if value.Kind() == reflect.Pointer && value.IsNil() {
		return reflect.Zero(targetType)
	}

	// Handle pointer targets (optional structs etc.): allocate and convert.
	if targetType.Kind() == reflect.Pointer {
		elemType := targetType.Elem()
		if value.Kind() == reflect.Pointer {
			if value.IsNil() {
				return reflect.Zero(targetType)
			}
			value = value.Elem()
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
		if value.IsNil() {
			return reflect.Zero(targetType)
		}
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
		if value.IsNil() {
			return reflect.Zero(targetType)
		}
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

		// Build candidate keys for this field. json:"-" (and json:"-,...")
		// fields must never be populated from request maps.
		candidates, ok := structFieldKeyCandidates(field)
		if !ok {
			continue
		}

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

// convertJSONNumber converts a json.Number to a numeric target type. It reports
// false when the target is not numeric or the value does not fit, leaving the
// caller's existing fallback behavior intact.
func convertJSONNumber(num json.Number, targetType reflect.Type) (reflect.Value, bool) {
	switch targetType.Kind() {
	case reflect.Pointer:
		value, ok := convertJSONNumber(num, targetType.Elem())
		if !ok {
			return reflect.Value{}, false
		}
		ptr := reflect.New(targetType.Elem())
		ptr.Elem().Set(value)
		return ptr, true

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, ok := exactJSONInt64(num)
		if !ok {
			return reflect.Value{}, false
		}
		converted := reflect.New(targetType).Elem()
		if converted.OverflowInt(i) {
			return reflect.Value{}, false
		}
		converted.SetInt(i)
		return converted, true

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		u, err := strconv.ParseUint(num.String(), 10, 64)
		if err != nil {
			return reflect.Value{}, false
		}
		converted := reflect.New(targetType).Elem()
		if converted.OverflowUint(u) {
			return reflect.Value{}, false
		}
		converted.SetUint(u)
		return converted, true

	case reflect.Float32, reflect.Float64:
		f, err := num.Float64()
		if err != nil {
			return reflect.Value{}, false
		}
		converted := reflect.New(targetType).Elem()
		if converted.OverflowFloat(f) {
			return reflect.Value{}, false
		}
		converted.SetFloat(f)
		return converted, true
	}
	return reflect.Value{}, false
}

// checkUnknownRequestFields reports the first request-map key that
// convertMapToStruct would silently ignore when value is converted to
// targetType. It mirrors that function's key candidates and recursion so strict
// mode accepts exactly the keys lenient conversion would populate.
func checkUnknownRequestFields(value interface{}, targetType reflect.Type, path string) error {
	if value == nil || targetType == nil {
		return nil
	}
	for targetType.Kind() == reflect.Pointer {
		targetType = targetType.Elem()
	}
	if targetType == nil || targetType.Kind() == reflect.Interface {
		// An interface target has no key schema.
		return nil
	}

	switch targetType.Kind() {
	case reflect.Struct:
		dict, ok := value.(map[string]interface{})
		if !ok {
			return nil
		}
		return checkStructRequestFields(dict, targetType, path)
	case reflect.Slice, reflect.Array:
		items, ok := value.([]interface{})
		if !ok {
			return nil
		}
		for i, item := range items {
			if err := checkUnknownRequestFields(item, targetType.Elem(), fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	case reflect.Map:
		dict, ok := value.(map[string]interface{})
		if !ok {
			return nil
		}
		for key, item := range dict {
			if err := checkUnknownRequestFields(item, targetType.Elem(), path+"."+key); err != nil {
				return err
			}
		}
	}
	return nil
}

// structRequestFields caches the accepted key schema for a struct type. The
// schema is immutable after construction and strict checking is read-only, so a
// single cached map can be shared by concurrent requests.
var structRequestFields sync.Map // map[reflect.Type]map[string]reflect.Type

// requestFieldsFor returns the cached key schema for structType.
func requestFieldsFor(structType reflect.Type) map[string]reflect.Type {
	if cached, ok := structRequestFields.Load(structType); ok {
		return cached.(map[string]reflect.Type)
	}
	allowed := make(map[string]reflect.Type, structType.NumField())
	addStructRequestKeys(structType, allowed, make(map[reflect.Type]bool))
	actual, _ := structRequestFields.LoadOrStore(structType, allowed)
	return actual.(map[string]reflect.Type)
}

// checkStructRequestFields verifies every key in dict maps to a struct field.
func checkStructRequestFields(dict map[string]interface{}, structType reflect.Type, path string) error {
	allowed := requestFieldsFor(structType)

	var unknown []string
	for key := range dict {
		if _, ok := allowed[key]; !ok {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) > 0 {
		// Sort only when producing an error so the valid path allocates nothing.
		sort.Strings(unknown)
		return fmt.Errorf("%s: unknown field %q", path, unknown[0])
	}

	for key := range dict {
		if err := checkUnknownRequestFields(dict[key], allowed[key], path+"."+key); err != nil {
			return err
		}
	}
	return nil
}

// addStructRequestKeys records the keys convertMapToStruct accepts for a struct,
// including promoted fields from embedded structs. visited bounds recursive
// embedded types so a self-referential struct cannot overflow the stack.
func addStructRequestKeys(structType reflect.Type, allowed map[string]reflect.Type, visited map[reflect.Type]bool) {
	if visited[structType] {
		return
	}
	visited[structType] = true

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if field.PkgPath != "" {
			// Unexported fields are not settable; convertMapToStruct skips them.
			continue
		}

		candidates, ok := structFieldKeyCandidates(field)
		if !ok {
			continue
		}
		for _, candidate := range candidates {
			if _, exists := allowed[candidate]; !exists {
				allowed[candidate] = field.Type
			}
		}

		// convertMapToStruct flattens anonymous structs when the embedded
		// field's own name did not match, so their promoted keys are valid.
		if field.Anonymous {
			embedded := field.Type
			for embedded.Kind() == reflect.Pointer {
				embedded = embedded.Elem()
			}
			if embedded.Kind() == reflect.Struct {
				addStructRequestKeys(embedded, allowed, visited)
			}
		}
	}
}

// structFieldKeyCandidates returns the request-map keys convertMapToStruct and
// strict unknown-field checking accept for a struct field. ok is false for a
// field excluded with json:"-".
func structFieldKeyCandidates(field reflect.StructField) ([]string, bool) {
	jsonName := ""
	if tag := field.Tag.Get("json"); tag != "" {
		name := strings.SplitN(tag, ",", 2)[0]
		if name == "-" {
			return nil, false
		}
		jsonName = name
	}

	candidates := make([]string, 0, 4)
	if jsonName != "" {
		candidates = append(candidates, jsonName)
	}
	candidates = append(candidates, field.Name)
	if len(field.Name) > 0 {
		candidates = append(candidates, string(rune(field.Name[0]-'A'+'a'))+field.Name[1:])
	}
	candidates = append(candidates, strings.ToLower(field.Name))
	return candidates, true
}

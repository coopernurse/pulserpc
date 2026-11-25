package pulserpc

import "fmt"

// RPCError represents a JSON-RPC 2.0 error
type RPCError struct {
	Code    int
	Message string
	Data    interface{}
}

// Error implements the error interface
func (e *RPCError) Error() string {
	if e.Data != nil {
		return fmt.Sprintf("RPCError %d: %s (data: %v)", e.Code, e.Message, e.Data)
	}
	return fmt.Sprintf("RPCError %d: %s", e.Code, e.Message)
}

// NewRPCError creates a new RPCError with the given code and message
func NewRPCError(code int, message string) *RPCError {
	return &RPCError{
		Code:    code,
		Message: message,
	}
}

// NewRPCErrorWithData creates a new RPCError with the given code, message, and data
func NewRPCErrorWithData(code int, message string, data interface{}) *RPCError {
	return &RPCError{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

package main

import (
	"testing"

	"github.com/coopernurse/pulserpc/pkg/runtime/runtimes/go/pulserpc"
)

func TestRPCError(t *testing.T) {
	err := &pulserpc.RPCError{
		Code:    -32603,
		Message: "Internal error",
		Data:    nil,
	}

	if err.Error() == "" {
		t.Error("RPCError.Error() should return non-empty string")
	}

	if err.Code != -32603 {
		t.Errorf("Expected Code -32603, got %d", err.Code)
	}

	if err.Message != "Internal error" {
		t.Errorf("Expected Message 'Internal error', got '%s'", err.Message)
	}

	// Test with data
	errWithData := &pulserpc.RPCError{
		Code:    -32602,
		Message: "Invalid params",
		Data:    "test data",
	}

	if errWithData.Data != "test data" {
		t.Errorf("Expected Data 'test data', got '%v'", errWithData.Data)
	}
}
